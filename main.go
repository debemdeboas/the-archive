package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	md_html "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

var staticCache = make(map[string]string)

const (
	ServerAddr = "0.0.0.0"
	ServerPort = "12600"
)

const (
	LightTheme string = "light-theme"
	DarkTheme  string = "dark-theme"

	LightThemeIcon string = `<i class="fas fa-sun"></i>`
	DarkThemeIcon  string = `<i class="fas fa-moon"></i>`

	DefaultDarkSyntaxTheme  string = "native"
	DefaultLightSyntaxTheme string = "base16-snazzy"

	DefaultTheme string = DarkTheme
)

const (
	HCType        = "Content-Type"
	HETag         = "ETag"
	HCacheControl = "Cache-Control"

	CTypeCSS  = "text/css"
	CTypeHTML = "text/html"
)

const (
	HTTPErrMethodNotAllowed = "Method not allowed"
)

const (
	CookieTheme       = "theme"
	CookieSyntaxTheme = "syntax-theme"
)

const (
	//? These paths must match the paths in the embed directive

	StaticLocalDir = "static"
	StaticUrlPath  = "/" + StaticLocalDir + "/"

	PostsLocalDir = "posts"
	PostsUrlPath  = "/" + PostsLocalDir + "/"

	TemplatesLocalDir = "templates"
)

//? Check if the embed directive is correct!

//go:embed static/* templates/*
var content embed.FS

//? Did you forget to add a file to the embed directive?

var syntaxCSSCache = make(map[string]template.CSS)

type Post struct {
	Title   string
	Content template.HTML
	Path    string

	// Used for cache busting.
	// We cannot use the content hash because the content is already rendered.
	MDContentHash string

	ModifiedDate time.Time
}

type PageData struct {
	PageURL string

	Theme string

	SyntaxCSS    template.CSS
	SyntaxTheme  string
	SyntaxThemes []string
}

func newPageData(r *http.Request) *PageData {
	return &PageData{
		PageURL:      r.URL.Path,
		Theme:        getThemeFromRequest(r),
		SyntaxTheme:  getSyntaxThemeFromRequest(r),
		SyntaxThemes: getSyntaxThemes(),
		SyntaxCSS:    generateSyntaxCSS(getSyntaxThemeFromRequest(r)),
	}
}

func (pd *PageData) IsPost() bool {
	return strings.HasPrefix(pd.PageURL, PostsUrlPath)
}

type PostRepository interface {
	GetPosts() ([]Post, map[string]*Post, error)
	ReadPost(path any) ([]byte, error)
	ReloadPosts()
}

type FSPostRepository struct{}

func (r *FSPostRepository) GetPosts() ([]Post, map[string]*Post, error) {
	entries, err := os.ReadDir(PostsLocalDir)
	if err != nil {
		return nil, nil, err
	}

	var posts []Post
	postsMap := make(map[string]*Post)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")

			mdContent, err := r.ReadPost(name)
			if err != nil {
				return nil, nil, err
			}

			fileInfo, err := entry.Info()
			if err != nil {
				return nil, nil, err
			}

			post := Post{
				Title:         name,
				Path:          name,
				MDContentHash: contentHash(mdContent),
				ModifiedDate:  fileInfo.ModTime(),
			}

			posts = append(posts, post)
			postsMap[name] = &post
		}
	}

	slices.SortStableFunc(posts, func(a, b Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postsMap, nil
}

func (r *FSPostRepository) ReadPost(path any) ([]byte, error) {
	return os.ReadFile(filepath.Join(PostsLocalDir, path.(string)+".md"))
}

func (r *FSPostRepository) ReloadPosts() {
	for {
		posts, postMap, err := r.GetPosts()
		if err != nil {
			log.Println("Error reloading posts:", err)
		} else {
			postsMu.Lock()

			for _, post := range cachedPostsSorted {
				if newPost, ok := postMap[post.Path]; ok {
					if newPost.MDContentHash != post.MDContentHash {
						log.Printf("Reloading post: %s", post.Path)
						go handleReloadPost(post.Path)
					}
				}
			}

			cachedPostsSorted = posts
			cachedPosts = postMap
			postsMu.Unlock()
		}
		time.Sleep(1 * time.Second)
	}
}

var (
	postRepository    PostRepository   = &FSPostRepository{}
	cachedPosts       map[string]*Post = make(map[string]*Post)
	cachedPostsSorted []Post
	postsMu           sync.RWMutex
)

type Client struct {
	msgChan chan string
	postID  string
}

var (
	clientsMu sync.Mutex
	clients   = make(map[*Client]bool)

	webhookSecret = "8a0ddd79-4e57-401d-8a00-23ed34e9afc9" //todo read from env

	lastWebhookMu      sync.Mutex
	lastWebhookTrigger time.Time
	reloadInterval     = 10 * time.Second
)

func getThemeFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(CookieTheme); err == nil {
		return cookie.Value
	}
	return DefaultTheme
}

func getDefaultSyntaxTheme(theme string) string {
	return map[string]string{
		LightTheme: DefaultLightSyntaxTheme,
		DarkTheme:  DefaultDarkSyntaxTheme,
	}[theme]
}

func getSyntaxThemeFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(CookieSyntaxTheme); err == nil {
		return cookie.Value
	}
	return getDefaultSyntaxTheme(getThemeFromRequest(r))
}

func getSyntaxThemes() []string {
	return styles.Names()
}

func getFormatter() *html.Formatter {
	formatter := html.New(
		html.WithClasses(true),
		html.TabWidth(4),
		html.WithLineNumbers(true),
		html.WrapLongLines(true),
	)
	return formatter
}

func generateSyntaxCSS(theme string) template.CSS {
	if css, ok := syntaxCSSCache[theme]; ok {
		return css
	}

	var buf strings.Builder
	formatter := getFormatter()
	style := styles.Get(theme)

	bg := style.Get(chroma.Background)
	if !bg.Colour.IsSet() {
		// Calculate the color of highlighted text given the background color
		// for when the Chroma theme doesn't supply a default
		luminance := (0.299*float64(bg.Background.Red()) +
			0.587*float64(bg.Background.Green()) +
			0.114*float64(bg.Background.Blue())) / 255
		if luminance > 0.5 {
			buf.WriteString(".chroma { color: #181818; }\n")
		}
	}

	formatter.WriteCSS(&buf, style)
	css := template.CSS(buf.String())
	syntaxCSSCache[theme] = css
	return css
}

func getThemeIcon(theme string) string {
	if theme == LightTheme {
		return DarkThemeIcon
	}
	return LightThemeIcon
}

func addCacheHeaders(w http.ResponseWriter) {
	w.Header().Set(HCacheControl, "public, max-age=3600")
	w.Header().Set("Vary", "Cookie")
}

func contentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func cacheIt(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/theme/opposite-icon" {
			w.Header().Set(HCacheControl, "no-store, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		} else {
			addCacheHeaders(w)

			// Add etag header to response if it's a static file
			if hash, ok := staticCache[r.URL.Path]; ok {
				w.Header().Set(HETag, hash)
			}
		}
		h(w, r)
	}
}

func secureHeaders(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self' *.debem.dev;"+
				"script-src 'self' 'unsafe-inline' *.debem.dev unpkg.com cdn.jsdelivr.net *.cloudflareinsights.com;"+
				"style-src 'self' 'unsafe-inline' *.debem.dev cdnjs.cloudflare.com;"+
				"font-src 'self' cdnjs.cloudflare.com cdn.jsdelivr.net;"+
				"img-src *;"+
				"object-src 'none'",
		)

		h(w, r)
	}
}

func main() {
	mux := http.NewServeMux()

	// Calculate the hash of static content to use as a cache buster
	static, _ := fs.Sub(content, StaticLocalDir)

	fs.WalkDir(static, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			staticCache[StaticUrlPath+path] = contentHash([]byte(path))
		}
		return nil
	})

	mux.Handle(StaticUrlPath, http.StripPrefix(StaticUrlPath, http.FileServer(http.FS(static))))

	mux.HandleFunc(PostsUrlPath, servePost)

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(HCType, "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User-agent: *\nDisallow:"))
	})

	mux.HandleFunc("/theme/opposite-icon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(HCType, CTypeHTML)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(getThemeIcon(getThemeFromRequest(r))))
	})

	mux.HandleFunc("/theme/toggle", serveThemePostToggle)

	mux.HandleFunc("/syntax-theme/set", serveSyntaxThemePostSet)
	mux.HandleFunc("/syntax-theme/{theme}", serveSyntaxThemeGetTheme)

	mux.HandleFunc("/sse", eventsHandler)
	mux.HandleFunc("/webhook/reload", webhookReloadHandler)

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

		htmlContent := renderMarkdown(mdContent, getSyntaxThemeFromRequest(r))

		w.Header().Set(HCType, CTypeHTML)
		w.WriteHeader(http.StatusOK)
		w.Write(htmlContent)
	})

	mux.HandleFunc("/", serveIndex)

	// Load posts into cache
	var err error
	cachedPostsSorted, cachedPosts, err = postRepository.GetPosts()
	if err != nil {
		log.Fatal(err)
	}
	go postRepository.ReloadPosts()

	securedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secureHeaders(mux.ServeHTTP)(w, r)
	})

	log.Fatal(http.ListenAndServe(ServerAddr+":"+ServerPort, cacheIt(securedMux)))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	postsMu.Lock()
	posts := cachedPostsSorted
	postsMu.Unlock()

	tmpl, err := template.ParseFS(content, TemplatesLocalDir+"/layout.html", TemplatesLocalDir+"/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		*PageData
		PostsPath string
		Posts     []Post
	}{
		PageData:  newPageData(r),
		PostsPath: PostsUrlPath,
		Posts:     posts,
	}

	w.Header().Set(HETag, contentHash([]byte(data.Theme+data.SyntaxTheme)))

	err = tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func servePost(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, PostsUrlPath)
	if path == "" {
		http.NotFound(w, r)
		return
	}

	mdContent, err := postRepository.ReadPost(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	htmlContent := renderMarkdown(mdContent, getSyntaxThemeFromRequest(r))
	post := Post{
		Title:   strings.TrimSuffix(path, filepath.Ext(path)),
		Path:    path,
		Content: template.HTML(htmlContent),
	}

	data := struct {
		*PageData
		Post *Post
	}{
		PageData: newPageData(r),
		Post:     &post,
	}

	tmpl, err := template.ParseFS(content, TemplatesLocalDir+"/layout.html", TemplatesLocalDir+"/post.html")
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
	currentTheme := getThemeFromRequest(r)

	newTheme := DefaultTheme
	if currentTheme == DarkTheme {
		newTheme = LightTheme
	}

	http.SetCookie(w, &http.Cookie{
		Name:  CookieTheme,
		Value: newTheme,
		Path:  "/",
	})

	syntaxTheme := getDefaultSyntaxTheme(newTheme)
	if cookie, err := r.Cookie(CookieSyntaxTheme); err == nil {
		syntaxTheme = cookie.Value
	}

	w.Header().Set("Hx-Trigger", fmt.Sprintf(`{"themeChanged":{"value":"%s","syntaxTheme":"%s"}}`, newTheme, syntaxTheme))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(getThemeIcon(newTheme)))
}

func serveSyntaxThemePostSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	theme := r.FormValue("syntax-theme-select")
	if theme == "" {
		http.Error(w, "theme required", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieSyntaxTheme,
		Value:    theme,
		Path:     "/",
		HttpOnly: true,
	})

	themeStyle := []byte(generateSyntaxCSS(theme))
	w.WriteHeader(http.StatusOK)
	w.Header().Set(HCType, CTypeCSS)
	w.Header().Set(HETag, contentHash(themeStyle))
	w.Write(themeStyle)
}

func serveSyntaxThemeGetTheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	theme := r.PathValue("theme")

	themeStyle := []byte(generateSyntaxCSS(theme))
	w.WriteHeader(http.StatusOK)
	w.Header().Set(HCType, CTypeCSS)
	w.Header().Set(HETag, contentHash(themeStyle))
	w.Write(themeStyle)
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post")
	if postID == "" {
		http.Error(w, "Post parameter required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
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
		postID:  postID,
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

func webhookReloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Webhook-Secret") != webhookSecret {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	changedPostID := r.URL.Query().Get("post")
	if changedPostID == "" {
		http.Error(w, "Post parameter required", http.StatusBadRequest)
		return
	}

	lastWebhookMu.Lock()
	if time.Since(lastWebhookTrigger) < reloadInterval {
		lastWebhookMu.Unlock()
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}
	lastWebhookTrigger = time.Now()
	lastWebhookMu.Unlock()

	go handleReloadPost(changedPostID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Reload triggered"))
}

func handleReloadPost(postID string) {
	clientsMu.Lock()
	for client := range clients {
		if client.postID == postID {
			select {
			case client.msgChan <- "reload":
			default:
			}
		}
	}
	clientsMu.Unlock()
}

func highlightCode(code, language, highlightTheme string) string {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var buf strings.Builder
	style := styles.Get(highlightTheme)
	formatter := getFormatter()
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return code
	}

	return buf.String()
}

func renderMarkdown(md []byte, highlightTheme string) []byte {
	opts := md_html.RendererOptions{
		Flags:    md_html.CommonFlags | md_html.HrefTargetBlank | md_html.FootnoteReturnLinks,
		Comments: [][]byte{[]byte("//"), []byte("#")},
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			if code, ok := node.(*ast.CodeBlock); ok && entering {
				var lang string
				if info := code.Info; info != nil {
					lang = string(info)
				}
				highlighted := highlightCode(string(code.Literal), lang, highlightTheme)
				fmt.Fprintf(w, "<div class=\"highlight\">%s</div>", highlighted)
				return ast.GoToNext, true
			}

			return ast.GoToNext, false
		},
	}

	doc := parser.NewWithExtensions(parser.CommonExtensions | parser.AutoHeadingIDs |
		parser.Footnotes | parser.SuperSubscript | parser.Mmark).Parse(md)
	rendered := markdown.Render(doc, md_html.NewRenderer(opts))

	return rendered
}
