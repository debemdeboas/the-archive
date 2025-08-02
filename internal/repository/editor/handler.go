package editor

import (
	"embed"
	"net/http"
	"text/template"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/sse"
	"github.com/debemdeboas/the-archive/internal/util"
)

type Handler struct {
	repo    Repository
	clients *sse.SSEClients

	fs *embed.FS
}

func NewHandler(repo Repository, clients *sse.SSEClients, fs *embed.FS) *Handler {
	return &Handler{
		repo:    repo,
		clients: clients,
		fs:      fs,
	}
}

func (h *Handler) ServeNewDraftEditor(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(h.fs, config.TemplatesLocalDir+"/"+config.TemplateLayout, config.TemplatesLocalDir+"/"+config.TemplateEditor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var draft *Draft = nil
	if cookie, err := r.Cookie(config.CookieDraftID); err == nil {
		draftID := DraftID(cookie.Value)
		draft, _ = h.repo.GetDraft(draftID)
	}

	if draft == nil {
		draft, err = h.repo.CreateDraft()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  config.CookieDraftID,
			Value: string(draft.ID),
			Path:  "/",
		})
	}

	saveURL := "/api/posts/" + string(draft.ID)
	saveMethod := "POST"

	pageData := model.NewPageData(r)
	hxPostURL := ""
	if pageData.LivePreviewEnabled {
		hxPostURL = "/partials/draft/preview"
	}

	data := struct {
		*model.PageData
		*model.Post
		HxPostURL    string
		HxSaveURL    *string
		HxSaveMethod *string
	}{
		PageData:     pageData,
		Post:         &model.Post{ID: model.PostID(draft.ID), Markdown: draft.Content},
		HxPostURL:    hxPostURL,
		HxSaveURL:    &saveURL,
		HxSaveMethod: &saveMethod,
	}

	showToolbar := true
	data.IsEditorPage = &showToolbar
	data.ShowToolbar = &showToolbar

	w.Header().Set(config.HETag, util.ContentHash([]byte(data.Theme+data.SyntaxTheme)))
	err = tmpl.ExecuteTemplate(w, config.TemplateLayout, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) ServeEditPostEditor(w http.ResponseWriter, r *http.Request, post *model.Post) {
	tmpl, err := template.ParseFS(h.fs, config.TemplatesLocalDir+"/"+config.TemplateLayout, config.TemplatesLocalDir+"/"+config.TemplateEditor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	saveURL := "/api/posts/" + string(post.ID)
	savePut := "PUT"

	pageData := model.NewPageData(r)
	hxPostURL := ""
	if pageData.LivePreviewEnabled {
		hxPostURL = "/partials/post/preview"
	}

	data := struct {
		*model.PageData
		*model.Post
		HxPostURL    string
		HxSaveURL    *string
		HxSaveMethod *string
	}{
		PageData:     pageData,
		Post:         post,
		HxPostURL:    hxPostURL,
		HxSaveURL:    &saveURL,
		HxSaveMethod: &savePut,
	}

	showToolbar := true
	data.IsEditorPage = &showToolbar
	data.ShowToolbar = &showToolbar

	w.Header().Set(config.HETag, util.ContentHash([]byte(data.Theme+data.SyntaxTheme)))
	err = tmpl.ExecuteTemplate(w, config.TemplateLayout, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
