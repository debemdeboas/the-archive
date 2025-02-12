package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/mmarkdown/mmark/v2/mast"

	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/render"
	"github.com/debemdeboas/the-archive/internal/repository"
	"github.com/debemdeboas/the-archive/internal/theme"
	"github.com/debemdeboas/the-archive/internal/util"
)

//go:embed static/* templates/*
var content embed.FS

type Client struct {
	msgChan chan string
	postId  string
}

var (
	clientsMu sync.Mutex
	clients   = make(map[*Client]bool)
)

var postRepository repository.PostRepository = repository.NewFSPostRepository(config.PostsLocalDir)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// Calculate the hash of static content to use as a cache buster
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

		mdContent, err := postRepository.ReadPost(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		htmlContent, _ := render.RenderMarkdown(mdContent, theme.GetSyntaxThemeFromRequest(r))

		w.Header().Set(config.HCType, config.CTypeHTML)
		w.WriteHeader(http.StatusOK)
		w.Write(htmlContent)
	})

	mux.Handle(config.StaticUrlPath, http.StripPrefix(config.StaticUrlPath, http.FileServer(http.FS(static))))
	mux.HandleFunc(config.PostsUrlPath, servePost)
	mux.HandleFunc("/new/post/edit", serveNewPostEditor)
	mux.HandleFunc("/partials/post/preview", serveNewPostPreview)
	mux.HandleFunc("/theme/toggle", serveThemePostToggle)
	mux.HandleFunc("/syntax-theme/set", serveSyntaxThemePostSet)
	mux.HandleFunc("/syntax-theme/{theme}", serveSyntaxThemeGetTheme)
	mux.HandleFunc("/sse", eventsHandler)
	mux.HandleFunc("/", serveIndex)

	postRepository.Init()
	postRepository.SetReloadNotifier(handleReloadPost)

	securedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" { // Ignore robots.txt
			mux.ServeHTTP(w, r)
		} else {
			secureHeaders(mux.ServeHTTP)(w, r)
		}
	})

	log.Fatal(http.ListenAndServe(config.ServerAddr+":"+config.ServerPort, cacheIt(securedMux)))
}

func serveNewPostEditor(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(content, config.TemplatesLocalDir+"/layout.html", config.TemplatesLocalDir+"/editor.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		*model.PageData
		*model.Post
	}{
		PageData: model.NewPageData(r),
		Post:     &model.Post{},
	}
	showToolbar := true
	data.IsEditorPage = &showToolbar
	data.ShowToolbar = &showToolbar

	w.Header().Set(config.HETag, util.ContentHash([]byte(data.Theme+data.SyntaxTheme)))

	err = tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func serveNewPostPreview(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Start typing in the editor to see a preview here.", http.StatusBadRequest)
		return
	}

	htmlContent, _ := render.RenderMarkdown([]byte(content), theme.GetSyntaxThemeFromRequest(r))

	w.Header().Set(config.HCType, config.CTypeHTML)
	w.WriteHeader(http.StatusOK)
	w.Write(htmlContent)
}

func cacheIt(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/partials/") {
			w.Header().Set(config.HCacheControl, "no-cache")
			h(w, r)
			return
		}

		w.Header().Set(config.HCacheControl, "public, max-age=3600")
		w.Header().Set("Vary", "Cookie")

		// Add etag header to response if it's a static file
		if hash, ok := cache.GetStaticHash(r.URL.Path); ok {
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

	tmpl, err := template.ParseFS(content, config.TemplatesLocalDir+"/layout.html", config.TemplatesLocalDir+"/index.html")
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

	err = tmpl.ExecuteTemplate(w, "layout.html", data)
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

	mdContent, err := postRepository.ReadPost(postId)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	htmlContent, extra := render.RenderMarkdown(mdContent, theme.GetSyntaxThemeFromRequest(r))

	var title string
	if info := extra.(*mast.TitleData); info != nil {
		if info.Title != "" {
			title = info.Title
		} else {
			title = strings.TrimSuffix(postId, filepath.Ext(postId))
		}
	}

	post := model.Post{
		Title:   title,
		Path:    postId,
		Content: template.HTML(htmlContent),
		Info:    extra.(*mast.TitleData),
	}

	data := struct {
		*model.PageData
		Post *model.Post
	}{
		PageData: model.NewPageData(r),
		Post:     &post,
	}

	tmpl, err := template.ParseFS(content, config.TemplatesLocalDir+"/layout.html", config.TemplatesLocalDir+"/post.html")
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

	client := &Client{
		msgChan: make(chan string),
		postId:  postId,
	}

	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	log.Printf("New SSE client connected")

	defer func() {
		clientsMu.Lock()
		delete(clients, client)
		close(client.msgChan)
		clientsMu.Unlock()
		log.Printf("SSE client disconnected")
	}()

	notify := r.Context().Done()
	for {
		select {
		case msg := <-client.msgChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func handleReloadPost(postId model.PostId) {
	clientsMu.Lock()
	for client := range clients {
		if client.postId == string(postId) {
			select {
			case client.msgChan <- "reload":
			default:
			}
		}
	}
	clientsMu.Unlock()
}
