package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mmarkdown/mmark/v2/mast"
	"github.com/rs/zerolog"

	"github.com/debemdeboas/the-archive/internal/auth"
	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/logger"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/render"
	"github.com/debemdeboas/the-archive/internal/repository"
	"github.com/debemdeboas/the-archive/internal/repository/editor"
	"github.com/debemdeboas/the-archive/internal/routes"
	"github.com/debemdeboas/the-archive/internal/sse"
	"github.com/debemdeboas/the-archive/internal/theme"
	"github.com/debemdeboas/the-archive/internal/util"
)

// authStatusMiddleware adds authentication status to request context
func (app *Application) authStatusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, isAuthenticated := auth.UserIDFromContext(r.Context())
		ctx := model.WithAuthStatus(r.Context(), isAuthenticated)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

//go:embed static/* templates/*
var content embed.FS

type Application struct {
	log           zerolog.Logger
	db            db.DB
	postRepo      repository.PostRepository
	editorRepo    editor.Repository
	editorHandler *editor.Handler
	authProvider  auth.AuthProvider
	clients       *sse.SSEClients
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading .env file: %v\n", err)
	}

	if err := config.LoadConfig("config.yaml"); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config.yaml, using defaults: %v\n", err)
	}

	log := logger.New(config.AppConfig.Logging.Level)

	config.SetLogger(log)
	db.SetLogger(log)
	repository.SetLogger(log)
	auth.SetLogger(log)
	render.SetLogger(log)

	database := db.NewSQLite()
	if err := database.InitDB(); err != nil {
		log.Fatal().Err(err).Msg("Error initializing database")
	}

	postRepo := repository.NewDBPostRepository(database)
	postRepo.SetReloadTimeout(time.Duration(config.AppConfig.Posts.ReloadTimeout) * time.Second)

	editorRepo := editor.NewMemoryRepository()
	clients := sse.NewSSEClients()
	editorHandler := editor.NewHandler(editorRepo, clients, &content)

	authProvider, err := auth.NewEd25519AuthProvider(
		os.Getenv("ED25519_PUBKEY"),
		"Authorization",
		model.UserID("admin"),
	)
	if err != nil {
		log.Error().Err(err).Msg("Error starting ED25519 auth provider")
	}

	app := &Application{
		log:           log,
		db:            database,
		postRepo:      postRepo,
		editorRepo:    editorRepo,
		editorHandler: editorHandler,
		authProvider:  authProvider,
		clients:       clients,
	}

	static, _ := fs.Sub(content, config.StaticLocalDir)
	fs.WalkDir(static, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			cache.SetStaticHash(config.StaticURLPath+path, util.ContentHash([]byte(path)))
		}
		return nil
	})

	mux := http.NewServeMux()

	mux.HandleFunc(routes.RobotsPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(config.HCType, "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User-agent: *\nDisallow:"))
	})

	mux.HandleFunc(routes.RootPath, app.serveIndex)
	mux.Handle(config.StaticURLPath, http.StripPrefix(config.StaticURLPath, http.FileServer(http.FS(static))))

	// Serve uploaded images from filesystem
	mux.Handle("/static/uploads/", http.StripPrefix("/static/uploads/", http.FileServer(http.Dir("static/uploads/"))))

	mux.HandleFunc(config.PostsURLPath, app.servePost)
	mux.HandleFunc(routes.PartialsPost, app.servePartialsPost)

	if config.AppConfig.Theme.AllowSwitching {
		mux.HandleFunc(routes.ThemeToggle, app.serveThemePostToggle)
		mux.HandleFunc(routes.ThemeOppositeIcon, app.serveThemeOppositeIcon)
	}
	mux.HandleFunc(routes.SyntaxThemeSet, app.serveSyntaxThemePostSet)
	mux.HandleFunc(routes.SyntaxThemeGet, app.serveSyntaxThemeGetTheme)

	mux.HandleFunc(routes.SSEPath, app.eventsHandler)

	if config.AppConfig.Features.Editor.Enabled {
		// Editor routes (for editing existing posts) - protected by authentication
		if config.AppConfig.Features.Authentication.Enabled {
			mux.Handle(routes.EditPost, app.authProvider.WithHeaderAuthorization()(http.HandlerFunc(app.ServeEditPost)))
		} else {
			mux.Handle(routes.EditPost, http.HandlerFunc(app.ServeEditPost))
		}
		mux.HandleFunc(routes.APIPosts, app.handleAPIPosts)
		mux.HandleFunc(routes.APIImages, app.handleAPIImages)

		if config.AppConfig.Features.Editor.LivePreview {
			mux.Handle(
				routes.PartialsPostPreview,
				http.HandlerFunc(app.midWithPostSaving(app.serveNewPostPreview)),
			)
		}

		// Draft routes (for creating new posts)
		if config.AppConfig.Features.Editor.EnableDrafts {
			mux.HandleFunc(routes.NewPost, app.serveNewPost)
			mux.Handle(routes.NewPostEdit, http.HandlerFunc(app.editorHandler.ServeNewDraftEditor))

			if config.AppConfig.Features.Editor.LivePreview {
				mux.Handle(
					routes.PartialsDraftPreview,
					http.HandlerFunc(app.midWithDraftSaving(app.serveNewPostPreview)),
				)
			}
		}
	}

	if config.AppConfig.Features.Authentication.Enabled {
		auth.RegisterEd25519AuthRoutes(mux, app.authProvider.(*auth.Ed25519AuthProvider), &content)
	}

	go app.postRepo.Init()
	app.postRepo.SetReloadNotifier(app.handleReloadPost)

	securedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == routes.RobotsPath {
			mux.ServeHTTP(w, r)
		} else {
			secureHeaders(mux.ServeHTTP)(w, r)
		}
	})

	var finalHandler http.Handler
	if config.AppConfig.Features.Authentication.Enabled {
		finalHandler = app.authProvider.WithHeaderAuthorization()(app.authStatusMiddleware(securedMux))
	} else {
		finalHandler = app.authStatusMiddleware(securedMux)
	}

	log.Info().Msg("Server started on " + config.AppConfig.Server.Host + ":" + config.AppConfig.Server.Port)
	log.Info().Msg("Using static files from " + config.StaticLocalDir)
	log.Info().Msg("Using templates from " + config.TemplatesLocalDir)

	log.Fatal().Err(http.ListenAndServe(config.AppConfig.Server.Host+":"+config.AppConfig.Server.Port, loggingMiddleware(log)(cacheIt(finalHandler)))).Msg("Server closed")
}

func (app *Application) serveThemeOppositeIcon(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())
	currTheme := r.URL.Query().Get("theme")
	if currTheme == "" {
		l.Error().Msg("theme required")
		http.Error(w, "theme required", http.StatusBadRequest)
		return
	}
	w.Header().Set(config.HCType, config.CTypeHTML)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(theme.GetThemeIcon(currTheme)))
}

func (app *Application) servePartialsPost(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("post")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	post, err := app.postRepo.ReadPost(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	htmlContent, _ := render.RenderMarkdownCached(post.Markdown, post.MDContentHash, theme.GetSyntaxThemeFromRequest(r))
	title := post.Title
	w.Header().Set(config.HCType, config.CTypeHTML)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("<title>%s</title>\n%s", title, htmlContent)))
}

func (app *Application) serveNewPost(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: config.CookieDraftID, Value: "", Path: "/"})
	w.Header().Add(config.HHxRedirect, routes.NewPostEdit)
	http.Redirect(w, r, routes.NewPostEdit, http.StatusFound)
}

func loggingMiddleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			r = r.WithContext(log.WithContext(r.Context()))
			defer func() {
				log.Debug().
					Str("method", r.Method).
					Str("url", r.URL.RequestURI()).
					Str("user_agent", r.UserAgent()).
					Dur("elapsed_ms", time.Since(start)).
					Msg("incoming request")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func (app *Application) ServeEditPost(w http.ResponseWriter, r *http.Request) {
	usrID, err := app.authProvider.GetUserIDFromSession(r)
	if err != nil {
		if r.Header.Get("Hx-Request") == "" {
			http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.String()), http.StatusFound)
			return
		}
		w.Header().Add(config.HHxRedirect, "/auth/login?redirect="+url.QueryEscape(r.URL.String()))
		return
	}
	postID := strings.TrimPrefix(r.URL.Path, routes.EditPost)
	if postID == "" {
		http.NotFound(w, r)
		return
	}
	post, err := app.postRepo.ReadPost(postID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if usrID != post.Owner {
		l := zerolog.Ctx(r.Context())
		l.Warn().Str("user_id", string(usrID)).Str("post_id", postID).Msg("Unauthorized attempt to edit post")
		w.Header().Add(config.HHxRedirect, r.Header.Get("Referer"))
		return
	}
	app.editorHandler.ServeEditPostEditor(w, r, post)
}

func (app *Application) midWithDraftSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		draftID := r.FormValue("draft-id")
		if draftID == "" {
			next.ServeHTTP(w, r)
			return
		}
		content := r.FormValue("content")
		if err := app.editorRepo.SaveDraft(editor.DraftID(draftID), []byte(content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (app *Application) midWithPostSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := strings.TrimPrefix(r.URL.Path, routes.EditPost)
		if postID == "" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (app *Application) serveNewPostPreview(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if content == "" {
		content = "Start typing in the editor to see a preview here."
	}
	htmlContent, _ := render.RenderMarkdown([]byte(content), theme.GetSyntaxThemeFromRequest(r))
	w.Header().Set(config.HCType, config.CTypeHTML)
	w.WriteHeader(http.StatusOK)
	// Wrap the content in a div with the id "post-content" to match the target
	// for the hx-swap="morph" attribute.
	w.Write([]byte(fmt.Sprintf(`<div id="post-content">%s</div>`, htmlContent)))
}

func cacheIt(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hash, ok := cache.GetStaticHash(r.URL.Path); ok {
			w.Header().Set(config.HCacheControl, "public, max-age=3600")
			w.Header().Set(config.HETag, hash)
		} else if r.URL.Path == "/" {
			w.Header().Set(config.HCacheControl, "private, max-age=300, must-revalidate")
			w.Header().Set("Vary", "Cookie")
		} else {
			w.Header().Set(config.HCacheControl, "no-cache")
			w.Header().Set("Vary", "Cookie")
		}
		h.ServeHTTP(w, r)
	}
}

func secureHeaders(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		h(w, r)
	}
}

func (app *Application) serveIndex(w http.ResponseWriter, r *http.Request) {
	posts := app.postRepo.GetPostList()
	tmpl, err := template.ParseFS(content, config.TemplatesLocalDir+"/"+config.TemplateLayout, config.TemplatesLocalDir+"/"+config.TemplateIndex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		*model.PageData
		PostsPath string
		Posts     []model.Post
	}{
		PageData:  model.NewPageData(r),
		PostsPath: config.PostsURLPath,
		Posts:     posts,
	}
	w.Header().Set(config.HETag, util.ContentHash([]byte(data.Theme+data.SyntaxTheme)))
	err = tmpl.ExecuteTemplate(w, config.TemplateLayout, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *Application) servePost(w http.ResponseWriter, r *http.Request) {
	postID := strings.TrimPrefix(r.URL.Path, config.PostsURLPath)
	if postID == "" {
		http.NotFound(w, r)
		return
	}
	post, err := app.postRepo.ReadPost(postID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	syntaxTheme := theme.GetSyntaxThemeFromRequest(r)
	htmlContent, extra := render.RenderMarkdownCached(post.Markdown, post.MDContentHash, syntaxTheme)
	post.Path = postID
	post.Content = template.HTML(htmlContent)
	post.Info = extra.(*mast.TitleData)

	// Warm cache for adjacent posts
	go func() {
		prev, next := app.postRepo.GetAdjacentPosts(postID)
		if prev != nil {
			app.log.Debug().Str("prev_post_id", string(prev.ID)).Msg("Warming cache for previous post")
			go render.WarmCache(prev.Markdown, prev.MDContentHash, syntaxTheme)
		}
		if next != nil {
			app.log.Debug().Str("next_post_id", string(next.ID)).Msg("Warming cache for next post")
			go render.WarmCache(next.Markdown, next.MDContentHash, syntaxTheme)
		}
	}()
	data := struct {
		*model.PageData
		Post *model.Post
	}{
		PageData: model.NewPageData(r),
		Post:     post,
	}
	tmpl, err := template.ParseFS(content, config.TemplatesLocalDir+"/"+config.TemplateLayout, config.TemplatesLocalDir+"/"+config.TemplatePost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *Application) serveThemePostToggle(w http.ResponseWriter, r *http.Request) {
	currentTheme := theme.GetThemeFromRequest(r)
	newTheme := config.AppConfig.Theme.Default
	if currentTheme == config.DarkTheme {
		newTheme = config.LightTheme
	}
	http.SetCookie(w, &http.Cookie{Name: config.CookieTheme, Value: newTheme, Path: "/"})
	syntaxTheme := theme.GetDefaultSyntaxTheme(newTheme)
	if cookie, err := r.Cookie(config.CookieSyntaxTheme); err == nil {
		syntaxTheme = cookie.Value
	}
	w.Header().Set("Hx-Trigger", fmt.Sprintf(`{"themeChanged":{"value":"%s","syntaxTheme":"%s"}}`, newTheme, syntaxTheme))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(theme.GetThemeIcon(newTheme)))
}

func (app *Application) serveSyntaxThemePostSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	currTheme := r.FormValue("syntax-theme-select")
	if currTheme == "" {
		http.Error(w, "theme required", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: config.CookieSyntaxTheme, Value: currTheme, Path: "/", HttpOnly: true})
	themeStyle := []byte(theme.GenerateSyntaxCSS(currTheme))
	w.WriteHeader(http.StatusOK)
	w.Header().Set(config.HCType, config.CTypeCSS)
	w.Header().Set(config.HETag, util.ContentHash(themeStyle))
	w.Write(themeStyle)
}

func (app *Application) serveSyntaxThemeGetTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	currTheme := r.PathValue("theme")
	themeStyle := []byte(theme.GenerateSyntaxCSS(currTheme))
	w.WriteHeader(http.StatusOK)
	w.Header().Set(config.HCType, config.CTypeCSS)
	w.Header().Set(config.HETag, util.ContentHash(themeStyle))
	w.Write(themeStyle)
}

func (app *Application) eventsHandler(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())
	postID := r.URL.Query().Get("post")
	if postID == "" {
		http.Error(w, "Post parameter required", http.StatusBadRequest)
		return
	}
	w.Header().Set(config.HCType, "text/event-stream")
	w.Header().Set(config.HCacheControl, "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Del("X-Content-Type-Options")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "event: connected\ndata: SSE connection established\n\n")
	flusher.Flush()
	client := &sse.Client{
		Msg:    make(chan string),
		PostID: model.PostID(postID),
	}
	app.clients.Add(client)
	l.Debug().Str("remote_addr", r.RemoteAddr).Msg("New SSE client connected")
	defer func() {
		app.clients.Delete(client)
		l.Debug().Str("remote_addr", r.RemoteAddr).Msg("SSE client disconnected")
	}()
	notify := r.Context().Done()
	for {
		select {
		case msg := <-client.Msg:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func (app *Application) handleReloadPost(postID model.PostID) {
	go app.clients.Broadcast(postID, "reload")
}

func (app *Application) handleAPIPosts(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())
	usrID, err := app.authProvider.EnforceUserAndGetID(w, r)
	if err != nil {
		l.Error().Err(err).Str("method", r.Method).Str("path", r.URL.Path).Msg("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		draftID := r.PathValue("id")
		if _, err := app.editorRepo.GetDraft(editor.DraftID(draftID)); err != nil {
			http.Error(w, "Draft not found", http.StatusNotFound)
			return
		}
		content := r.FormValue("content")
		post := app.postRepo.NewPost()
		post.Markdown = []byte(content)
		post.Owner = usrID
		post.Path = string(post.ID)
		frontMatter := util.GetFrontMatter(post.Markdown)
		if frontMatter != nil && frontMatter.Title != "" {
			post.Title = frontMatter.Title
		} else {
			post.Title = "Untitled - " + post.CreatedDate.Format("2006-01-02")
		}
		if err := app.postRepo.SavePost(post); err != nil {
			l.Error().Err(err).Str("post_id", string(post.ID)).Str("user_id", string(usrID)).Msg("Failed to save post")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case http.MethodPut:
		postID := r.PathValue("id")
		content := r.FormValue("content")
		post, err := app.postRepo.ReadPost(postID)
		if post == nil {
			http.Error(w, "Post not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		post.Markdown = []byte(content)
		frontMatter := util.GetFrontMatter(post.Markdown)
		if frontMatter != nil && frontMatter.Title != "" && post.Title != frontMatter.Title {
			post.Title = frontMatter.Title
		}
		if err := app.postRepo.SetPostContent(post); err != nil {
			l.Error().Err(err).Str("post_id", string(post.ID)).Str("user_id", string(usrID)).Msg("Failed to set post content")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
	}
}

func (app *Application) handleAPIImages(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())

	// Require authentication for image uploads
	usrID, err := app.authProvider.EnforceUserAndGetID(w, r)
	if err != nil {
		l.Error().Err(err).Str("method", r.Method).Str("path", r.URL.Path).Msg("Unauthorized image upload attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		l.Error().Err(err).Msg("Failed to parse multipart form")
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		l.Error().Err(err).Msg("Failed to get image from form")
		http.Error(w, "No image file found", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		l.Warn().Str("content_type", contentType).Msg("Invalid file type for image upload")
		http.Error(w, "File must be an image", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		l.Error().Err(err).Msg("Failed to generate random filename")
		http.Error(w, "Failed to generate filename", http.StatusInternalServerError)
		return
	}

	filename := hex.EncodeToString(randomBytes)

	// Determine file extension from content type
	var ext string
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	default:
		ext = ".jpg" // default fallback
	}

	filename += ext

	// Create uploads directory if it doesn't exist
	uploadsDir := filepath.Join("static", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		l.Error().Err(err).Str("dir", uploadsDir).Msg("Failed to create uploads directory")
		http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
		return
	}

	// Create the file
	filePath := filepath.Join(uploadsDir, filename)
	dst, err := os.Create(filePath)
	if err != nil {
		l.Error().Err(err).Str("file_path", filePath).Msg("Failed to create file")
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy the uploaded file to the destination
	if _, err := io.Copy(dst, file); err != nil {
		l.Error().Err(err).Str("file_path", filePath).Msg("Failed to save file")
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Return the URL path to the uploaded image
	imageURL := "/static/uploads/" + filename
	l.Info().Str("user_id", string(usrID)).Str("image_url", imageURL).Str("content_type", contentType).Msg("Image uploaded successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"url": "%s"}`, imageURL)
}
