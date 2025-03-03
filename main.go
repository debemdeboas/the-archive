package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/mmarkdown/mmark/v2/mast"

	"github.com/debemdeboas/the-archive/internal/auth"
	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/db"
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

var Db db.Db = db.NewSQLite()

var clients = sse.NewSSEClients()

var dbPostRepository repository.PostRepository = repository.NewDbPostRepository(Db)
var postRepository repository.PostRepository = repository.NewDbPostRepository(Db)

var editorRepo editor.Repository = editor.NewMemoryRepository()
var editorHandler = editor.NewHandler(editorRepo, clients, &content)

var clerkAuthProvider auth.AuthProvider
var ed25519AuthProvider auth.AuthProvider

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	Db.InitDb()

	clerkAuthProvider = auth.NewClerkAuthProvider(os.Getenv("CLERK_API"))

	ed25519AuthProvider, err = auth.NewEd25519AuthProvider(
		os.Getenv("ED25519_PUBKEY"),
		"Authorization",
		model.UserId("admin"),
	)
	if err != nil {
		log.Println(err)
	}

	// Calculate the hash of static content
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

	mux.HandleFunc("/theme/opposite-icon", func(w http.ResponseWriter, r *http.Request) {
		currTheme := r.URL.Query().Get("theme")
		if currTheme == "" {
			http.Error(w, "theme required", http.StatusBadRequest)
			return
		}

		w.Header().Set(config.HCType, config.CTypeHTML)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(theme.GetThemeIcon(currTheme)))
	})

	mux.HandleFunc("/partials/post", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("post")
		if path == "" {
			http.NotFound(w, r)
			return
		}

		post, err := postRepository.ReadPost(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		htmlContent, _ := render.RenderMarkdown(post.Markdown, theme.GetSyntaxThemeFromRequest(r))

		title := post.Title

		w.Header().Set(config.HCType, config.CTypeHTML)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("<title>%s</title>\n%s", title, htmlContent)))
	})

	mux.Handle(config.StaticUrlPath, http.StripPrefix(config.StaticUrlPath, http.FileServer(http.FS(static))))
	mux.HandleFunc(config.PostsUrlPath, servePost)
	mux.HandleFunc("/new/post", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  config.CookieDraftId,
			Value: "",
			Path:  "/",
		})
		w.Header().Add(config.HHxRedirect, "/new/post/edit")
		http.Redirect(w, r, "/new/post/edit", http.StatusFound)
	})
	mux.HandleFunc("/theme/toggle", serveThemePostToggle)
	mux.HandleFunc("/syntax-theme/set", serveSyntaxThemePostSet)
	mux.HandleFunc("/syntax-theme/{theme}", serveSyntaxThemeGetTheme)
	mux.HandleFunc("/sse", eventsHandler)
	mux.HandleFunc("/", serveIndex)

	mux.Handle(
		"/partials/post/preview",
		http.HandlerFunc(midWithPostSaving(serveNewPostPreview)),
	)

	mux.Handle(
		"/partials/draft/preview",
		http.HandlerFunc(midWithDraftSaving(serveNewPostPreview)),
	)

	mux.Handle(
		"/new/post/edit",
		http.HandlerFunc(editorHandler.ServeNewDraftEditor),
	)

	mux.Handle(
		"/edit/post/",
		http.HandlerFunc(ServeEditPost),
	)

	mux.HandleFunc("/webhook/user", clerkAuthProvider.HandleWebhookUser)

	mux.HandleFunc("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		usrId, err := ed25519AuthProvider.EnforceUserAndGetId(w, r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodPost:
			draftId := r.PathValue("id")

			if _, err := editorRepo.GetDraft(editor.DraftId(draftId)); err != nil {
				http.Error(w, "Draft not found", http.StatusNotFound)
				return
			}

			content := r.FormValue("content")

			post := postRepository.NewPost()
			post.Markdown = []byte(content)
			post.Owner = usrId
			post.Path = string(post.Id)

			frontMatter := util.GetFrontMatter(post.Markdown)
			if frontMatter != nil && frontMatter.Title != "" {
				post.Title = frontMatter.Title
			} else {
				post.Title = "Untitled - " + post.CreatedDate.Format("2006-01-02")
			}

			if err := postRepository.SavePost(post); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case http.MethodPut:
			postId := r.PathValue("id")
			content := r.FormValue("content")

			post, err := postRepository.ReadPost(postId)
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

			if err := postRepository.SetPostContent(post); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	})

	auth.RegisterEd25519AuthRoutes(mux, ed25519AuthProvider.(*auth.Ed25519AuthProvider), &content)

	go postRepository.Init()
	postRepository.SetReloadNotifier(handleReloadPost)

	securedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" { // Ignore robots.txt
			mux.ServeHTTP(w, r)
		} else {
			secureHeaders(mux.ServeHTTP)(w, r)
		}
	})

	authMux := ed25519AuthProvider.WithHeaderAuthorization()(securedMux)
	authHandlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authMux.ServeHTTP(w, r)
	})

	log.Fatal(http.ListenAndServe(config.ServerAddr+":"+config.ServerPort, cacheIt(authHandlerFunc)))
}

func ServeEditPost(w http.ResponseWriter, r *http.Request) {
	usrId, err := ed25519AuthProvider.GetUserIdFromSession(r)
	if err != nil {
		// Verify if it's an Hx-Request and if not, use standard redirect
		if r.Header.Get("Hx-Request") == "" {
			http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(r.URL.String()), http.StatusFound)
			return
		}
		// Redirect to /auth/login if no userId (unauthorized)
		w.Header().Add(config.HHxRedirect, "/auth/login?redirect="+url.QueryEscape(r.URL.String()))
		return
	}

	postId := strings.TrimPrefix(r.URL.Path, "/edit/post/")
	if postId == "" {
		http.NotFound(w, r)
		return
	}

	post, err := postRepository.ReadPost(postId)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Check ownership
	if usrId != post.Owner {
		w.Header().Add(config.HHxRedirect, r.Header.Get("Referer"))
		return
	}

	editorHandler.ServeEditPostEditor(w, r, post)
}

func midWithDraftSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		draftId := r.FormValue("draft-id")
		if draftId == "" {
			next.ServeHTTP(w, r)
			return
		}

		content := r.FormValue("content")
		if err := editorRepo.SaveDraft(editor.DraftId(draftId), []byte(content)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func midWithPostSaving(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postId := strings.TrimPrefix(r.URL.Path, "/edit/post/")
		if postId == "" {
			http.NotFound(w, r)
			return
		}
		// The content is at r.FormValue("content")
		next.ServeHTTP(w, r)
	}
}

func serveNewPostPreview(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if content == "" {
		content = "Start typing in the editor to see a preview here."
	}

	htmlContent, _ := render.RenderMarkdown([]byte(content), theme.GetSyntaxThemeFromRequest(r))

	w.Header().Set(config.HCType, config.CTypeHTML)
	w.WriteHeader(http.StatusOK)
	w.Write(htmlContent)
}

func cacheIt(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(config.HCacheControl, "no-cache")
		w.Header().Set("Vary", "Cookie")

		// Add etag header to response if it's a static file
		if hash, ok := cache.GetStaticHash(r.URL.Path); ok {
			w.Header().Set(config.HCacheControl, "public, max-age=3600")
			w.Header().Set(config.HETag, hash)
		}

		h(w, r)
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

func serveIndex(w http.ResponseWriter, r *http.Request) {
	posts := postRepository.GetPostList()

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

func servePost(w http.ResponseWriter, r *http.Request) {
	postId := strings.TrimPrefix(r.URL.Path, config.PostsUrlPath)
	if postId == "" {
		http.NotFound(w, r)
		return
	}

	post, err := postRepository.ReadPost(postId)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	htmlContent, extra := render.RenderMarkdown(post.Markdown, theme.GetSyntaxThemeFromRequest(r))
	post.Path = postId
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

func serveThemePostToggle(w http.ResponseWriter, r *http.Request) {
	currentTheme := theme.GetThemeFromRequest(r)

	newTheme := config.DefaultTheme
	if currentTheme == config.DarkTheme {
		newTheme = config.LightTheme
	}

	http.SetCookie(w, &http.Cookie{
		Name:  config.CookieTheme,
		Value: newTheme,
		Path:  "/",
	})

	syntaxTheme := theme.GetDefaultSyntaxTheme(newTheme)
	if cookie, err := r.Cookie(config.CookieSyntaxTheme); err == nil {
		syntaxTheme = cookie.Value
	}

	w.Header().Set("Hx-Trigger", fmt.Sprintf(`{"themeChanged":{"value":"%s","syntaxTheme":"%s"}}`, newTheme, syntaxTheme))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(theme.GetThemeIcon(newTheme)))
}

func serveSyntaxThemePostSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	currTheme := r.FormValue("syntax-theme-select")
	if currTheme == "" {
		http.Error(w, "theme required", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     config.CookieSyntaxTheme,
		Value:    currTheme,
		Path:     "/",
		HttpOnly: true,
	})

	themeStyle := []byte(theme.GenerateSyntaxCSS(currTheme))
	w.WriteHeader(http.StatusOK)
	w.Header().Set(config.HCType, config.CTypeCSS)
	w.Header().Set(config.HETag, util.ContentHash(themeStyle))
	w.Write(themeStyle)
}

func serveSyntaxThemeGetTheme(w http.ResponseWriter, r *http.Request) {
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

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	postId := r.URL.Query().Get("post")
	if postId == "" {
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
		PostId: model.PostId(postId),
	}

	clients.Add(client)

	log.Printf("New SSE client connected")

	defer func() {
		clients.Delete(client)
		log.Printf("SSE client disconnected")
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

func handleReloadPost(postId model.PostId) {
	go clients.Broadcast(postId, "reload")
}
