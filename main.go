package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
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
	"github.com/debemdeboas/the-archive/internal/sse"
	"github.com/debemdeboas/the-archive/internal/theme"
	"github.com/debemdeboas/the-archive/internal/util"
)

//go:embed static/* templates/*
var content embed.FS

type Application struct {
	log          zerolog.Logger
	db           db.Db
	postRepo     repository.PostRepository
	editorRepo   editor.Repository
	editorHandler *editor.Handler
	authProvider auth.AuthProvider
	clients      *sse.SSEClients
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

	database := db.NewSQLite()
	if err := database.InitDb(); err != nil {
		log.Fatal().Err(err).Msg("Error initializing database")
	}

	postRepo := repository.NewDbPostRepository(database)
	editorRepo := editor.NewMemoryRepository()
	clients := sse.NewSSEClients()
	editorHandler := editor.NewHandler(editorRepo, clients, &content)

	authProvider, err := auth.NewEd25519AuthProvider(
		os.Getenv("ED25519_PUBKEY"),
		"Authorization",
		model.UserId("admin"),
	)
	if err != nil {
		log.Error().Err(err).Msg("Error starting ED25519 auth provider")
	}

	app := &Application{
		log:          log,
		db:           database,
		postRepo:     postRepo,
		editorRepo:   editorRepo,
		editorHandler: editorHandler,
		authProvider: authProvider,
		clients:      clients,
	}

	static, _ := fs.Sub(content, config.StaticLocalDir)
	fs.WalkDir(static, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			cache.SetStaticHash(config.StaticUrlPath+path, util.ContentHash([]byte(path)))
		}
		return nil
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(config.HCType, "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User-agent: *\nDisallow:"))
	})

	mux.HandleFunc("/theme/opposite-icon", app.serveThemeOppositeIcon)
	mux.HandleFunc("/partials/post", app.servePartialsPost)

	mux.Handle(config.StaticUrlPath, http.StripPrefix(config.StaticUrlPath, http.FileServer(http.FS(static))))
	mux.HandleFunc(config.PostsUrlPath, app.servePost)
	if config.AppConfig.Features.Editor.Enabled {
		mux.HandleFunc("/new/post", app.serveNewPost)
	}

	if config.AppConfig.Theme.AllowSwitching {
		mux.HandleFunc("/theme/toggle", app.serveThemePostToggle)
	} else {
		mux.HandleFunc("/theme/toggle", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
	mux.HandleFunc("/syntax-theme/set", app.serveSyntaxThemePostSet)
	mux.HandleFunc("/syntax-theme/{theme}", app.serveSyntaxThemeGetTheme)
	mux.HandleFunc("/sse", app.eventsHandler)
	mux.HandleFunc("/", app.serveIndex)

	if config.AppConfig.Features.Editor.Enabled {
		if config.AppConfig.Features.Editor.LivePreview {
			mux.Handle(
				"/partials/post/preview",
				http.HandlerFunc(app.midWithPostSaving(app.serveNewPostPreview)),
			)
			mux.Handle(
				"/partials/draft/preview",
				http.HandlerFunc(app.midWithDraftSaving(app.serveNewPostPreview)),
			)
		} else {
			previewDisabledHandler := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
			mux.HandleFunc("/partials/post/preview", previewDisabledHandler)
			mux.HandleFunc("/partials/draft/preview", previewDisabledHandler)
		}
		mux.Handle("/new/post/edit", http.HandlerFunc(app.editorHandler.ServeNewDraftEditor))
		mux.Handle("/edit/post/", http.HandlerFunc(app.ServeEditPost))
	} else {
		editorDisabledHandler := func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }
		mux.HandleFunc("/new/post", editorDisabledHandler)
		mux.HandleFunc("/partials/post/preview", editorDisabledHandler)
		mux.HandleFunc("/partials/draft/preview", editorDisabledHandler)
		mux.HandleFunc("/new/post/edit", editorDisabledHandler)
		mux.HandleFunc("/edit/post/", editorDisabledHandler)
	}

	if config.AppConfig.Features.Editor.Enabled {
		mux.HandleFunc("/api/posts/{id}", app.handleApiPosts)
	} else {
		mux.HandleFunc("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	}

	if config.AppConfig.Features.Authentication.Enabled {
		auth.RegisterEd25519AuthRoutes(mux, app.authProvider.(*auth.Ed25519AuthProvider), &content)
	}

	go app.postRepo.Init()
	app.postRepo.SetReloadNotifier(app.handleReloadPost)

	securedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			mux.ServeHTTP(w, r)
		} else {
			secureHeaders(mux.ServeHTTP)(w, r)
		}
	})

	var finalHandler http.Handler
	if config.AppConfig.Features.Authentication.Enabled {
		finalHandler = app.authProvider.WithHeaderAuthorization()(securedMux)
	} else {
		finalHandler = securedMux
	}

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
	w.Header().Add(config.HHxRedirect, "/new/post/edit")
	http.Redirect(w, r, "/new/post/edit", http.StatusFound)
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
	usrId, err := app.authProvider.GetUserIdFromSession(r)
	if err != nil {
		if r.Header.Get("Hx-Request") == "" {
			http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.String()), http.StatusFound)
			return
		}
		w.Header().Add(config.HHxRedirect, "/auth/login?redirect="+url.QueryEscape(r.URL.String()))
		return
	}
	postID := strings.TrimPrefix(r.URL.Path, "/edit/post/")
	if postID == "" {
		http.NotFound(w, r)
		return
	}
	post, err := app.postRepo.ReadPost(postID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if usrId != post.Owner {
		l := zerolog.Ctx(r.Context())
		l.Warn().Str("user_id", string(usrId)).Str("post_id", postID).Msg("Unauthorized attempt to edit post")
		w.Header().Add(config.HHxRedirect, r.Header.Get("Referer"))
		return
	}
	app.editorHandler.ServeEditPostEditor(w, r, post)
}

func (app *Application) midWithDraftSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		draftId := r.FormValue("draft-id")
		if draftId == "" {
			next.ServeHTTP(w, r)
			return
		}
		content := r.FormValue("content")
		if err := app.editorRepo.SaveDraft(editor.DraftId(draftId), []byte(content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (app *Application) midWithPostSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID := strings.TrimPrefix(r.URL.Path, "/edit/post/")
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
	w.Write(htmlContent)
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
		PostsPath: config.PostsUrlPath,
		Posts:     posts,
	}
	w.Header().Set(config.HETag, util.ContentHash([]byte(data.Theme+data.SyntaxTheme)))
	err = tmpl.ExecuteTemplate(w, config.TemplateLayout, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *Application) servePost(w http.ResponseWriter, r *http.Request) {
	postID := strings.TrimPrefix(r.URL.Path, config.PostsUrlPath)
	if postID == "" {
		http.NotFound(w, r)
		return
	}
	post, err := app.postRepo.ReadPost(postID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	htmlContent, extra := render.RenderMarkdownCached(post.Markdown, post.MDContentHash, theme.GetSyntaxThemeFromRequest(r))
	post.Path = postID
	post.Content = template.HTML(htmlContent)
	post.Info = extra.(*mast.TitleData)
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
		PostId: model.PostId(postID),
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

func (app *Application) handleReloadPost(postID model.PostId) {
	go app.clients.Broadcast(postID, "reload")
}

func (app *Application) handleApiPosts(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())
	usrID, err := app.authProvider.EnforceUserAndGetId(w, r)
	if err != nil {
		l.Error().Err(err).Str("method", r.Method).Str("path", r.URL.Path).Msg("Unauthorized access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodPost:
		draftID := r.PathValue("id")
		if _, err := app.editorRepo.GetDraft(editor.DraftId(draftID)); err != nil {
			http.Error(w, "Draft not found", http.StatusNotFound)
			return
		}
		content := r.FormValue("content")
		post := app.postRepo.NewPost()
		post.Markdown = []byte(content)
		post.Owner = usrID
		post.Path = string(post.Id)
		frontMatter := util.GetFrontMatter(post.Markdown)
		if frontMatter != nil && frontMatter.Title != "" {
			post.Title = frontMatter.Title
		} else {
			post.Title = "Untitled - " + post.CreatedDate.Format("2006-01-02")
		}
		if err := app.postRepo.SavePost(post); err != nil {
			l.Error().Err(err).Str("post_id", string(post.Id)).Str("user_id", string(usrID)).Msg("Failed to save post")
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
			l.Error().Err(err).Str("post_id", string(post.Id)).Str("user_id", string(usrID)).Msg("Failed to set post content")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
	}
}
