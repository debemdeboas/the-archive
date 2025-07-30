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
		draftId := DraftId(cookie.Value)
		draft, _ = h.repo.GetDraft(draftId)
	}

	if draft == nil {
		draft, err = h.repo.CreateDraft()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  config.CookieDraftID,
			Value: string(draft.Id),
			Path:  "/",
		})
	}

	saveUrl := "/api/posts/" + string(draft.Id)
	saveMethod := "POST"

	data := struct {
		*model.PageData
		*model.Post
		HxPostUrl    string
		HxSaveUrl    *string
		HxSaveMethod *string
	}{
		PageData:     model.NewPageData(r),
		Post:         &model.Post{Id: model.PostId(draft.Id), Markdown: draft.Content},
		HxPostUrl:    "/partials/draft/preview",
		HxSaveUrl:    &saveUrl,
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

	saveUrl := "/api/posts/" + string(post.Id)
	savePut := "PUT"

	data := struct {
		*model.PageData
		*model.Post
		HxPostUrl    string
		HxSaveUrl    *string
		HxSaveMethod *string
	}{
		PageData:     model.NewPageData(r),
		Post:         post,
		HxPostUrl:    "/partials/post/preview",
		HxSaveUrl:    &saveUrl,
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
