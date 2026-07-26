package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/madhava-poojari/dashboard-api/internal/auth"
	"github.com/madhava-poojari/dashboard-api/internal/config"
	"github.com/madhava-poojari/dashboard-api/internal/models"
	"github.com/madhava-poojari/dashboard-api/internal/store"
	"github.com/madhava-poojari/dashboard-api/internal/utils"
	"gorm.io/datatypes"
)

// BlogHandler handles blog-related HTTP endpoints.
type BlogHandler struct {
	store        *store.Store
	imageStorage *utils.R2Storage
	cfg          *config.Config
}

// NewBlogHandler creates a new BlogHandler.
func NewBlogHandler(s serviceStore, cfg *config.Config) *BlogHandler {
	return &BlogHandler{
		store:        s.Store,
		imageStorage: utils.NewR2Storage(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Endpoint, cfg.R2BucketName),
		cfg:          cfg,
	}
}

// ─── POST /blogs ────────────────────────────────────────────────────────────────

func (h *BlogHandler) CreateBlog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title      string          `json:"title"`
		Summary    string          `json:"summary"`
		Content    json.RawMessage `json:"content"`    // TipTap JSON object
		Visibility string          `json:"visibility"` // "public" | "internal"
		Status     string          `json:"status"`     // "draft" | "published"
		Tags       []string        `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err.Error())
		return
	}

	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	if req.Title == "" {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "title is required", nil, nil)
		return
	}

	visibility := models.BlogVisibilityPublic
	if req.Visibility == string(models.BlogVisibilityInternal) {
		visibility = models.BlogVisibilityInternal
	}

	status := models.BlogStatusDraft
	if req.Status == string(models.BlogStatusPublished) {
		status = models.BlogStatusPublished
	}

	blog := &models.Blog{
		Title:      req.Title,
		Summary:    req.Summary,
		Content:    datatypes.JSON(req.Content),
		AuthorID:   current.ID,
		Visibility: visibility,
		Status:     status,
	}

	if err := h.store.CreateBlog(ctx, blog, req.Tags); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to create blog", nil, err.Error())
		return
	}

	// Re-fetch with preloads
	created, _ := h.store.GetBlogByID(ctx, blog.ID)
	if created != nil {
		blog = created
	}

	utils.WriteJSONResponse(w, http.StatusCreated, true, "blog created", blog, nil)
}

// ─── GET /blogs ─────────────────────────────────────────────────────────────────

func (h *BlogHandler) ListBlogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	page := 1
	pageSize := 12
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 && ps <= 50 {
		pageSize = ps
	}

	blogs, total, err := h.store.ListBlogs(ctx, page, pageSize)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to list blogs", nil, err.Error())
		return
	}

	totalPages := (int(total) + pageSize - 1) / pageSize

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", map[string]interface{}{
		"blogs":       blogs,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	}, nil)
}

// ─── GET /blogs/my-drafts ───────────────────────────────────────────────────────

func (h *BlogHandler) ListMyDrafts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	drafts, err := h.store.ListDraftsByAuthor(ctx, current.ID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to list drafts", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", drafts, nil)
}

// ─── GET /blogs/tags ────────────────────────────────────────────────────────────

func (h *BlogHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	tags, err := h.store.ListBlogTags(ctx)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to list tags", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", tags, nil)
}

// ─── GET /blogs/{slug} ──────────────────────────────────────────────────────────

func (h *BlogHandler) GetBlog(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogBySlug(ctx, slug)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	// If it's a draft, only author or those with edit permission can view
	if blog.Status == models.BlogStatusDraft {
		if !h.store.CanEditBlog(ctx, current, blog) {
			utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, nil)
			return
		}
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", blog, nil)
}

// ─── PATCH /blogs/{id} ──────────────────────────────────────────────────────────

func (h *BlogHandler) UpdateBlog(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	var req struct {
		Title      *string          `json:"title"`
		Summary    *string          `json:"summary"`
		Content    *json.RawMessage `json:"content"`
		Visibility *string          `json:"visibility"`
		Status     *string          `json:"status"`
		Tags       *[]string        `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "invalid request body", nil, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Summary != nil {
		updates["summary"] = *req.Summary
	}
	if req.Content != nil {
		updates["content"] = datatypes.JSON(*req.Content)
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) == 0 && req.Tags == nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "no fields to update", nil, nil)
		return
	}

	if err := h.store.UpdateBlogFields(ctx, blogID, updates, req.Tags); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to update blog", nil, err.Error())
		return
	}

	// Re-fetch updated blog
	updated, _ := h.store.GetBlogByID(ctx, blogID)
	utils.WriteJSONResponse(w, http.StatusOK, true, "blog updated", updated, nil)
}

// ─── DELETE /blogs/{id} ─────────────────────────────────────────────────────────

func (h *BlogHandler) DeleteBlog(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	if err := h.store.DeleteBlogSoft(ctx, blogID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to delete blog", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "blog deleted", nil, nil)
}

// ─── POST /blogs/{id}/images ────────────────────────────────────────────────────

func (h *BlogHandler) UploadBlogImage(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// Max 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "file too large or invalid form", nil, err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing file field", nil, err.Error())
		return
	}
	defer file.Close()

	subDir := fmt.Sprintf("blogs/%s", blogID)
	urlSuffix, err := h.imageStorage.SaveFile(subDir, header.Filename, file)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to upload image", nil, err.Error())
		return
	}

	altText := r.FormValue("alt_text")

	img := &models.BlogImage{
		BlogID:    blogID,
		URLSuffix: urlSuffix,
		AltText:   altText,
	}

	if err := h.store.CreateBlogImage(ctx, img); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to save image record", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusCreated, true, "image uploaded", map[string]interface{}{
		"id":         img.ID,
		"url_suffix": urlSuffix,
	}, nil)
}

// ─── GET /blogs/{id}/images ─────────────────────────────────────────────────────

func (h *BlogHandler) ListBlogImages(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	images, err := h.store.ListBlogImages(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to list images", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "success", images, nil)
}

// ─── DELETE /blogs/{id}/images/{imageId} ────────────────────────────────────────

func (h *BlogHandler) DeleteBlogImage(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	imageID := chi.URLParam(r, "imageId")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	img, err := h.store.GetBlogImageByID(ctx, imageID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "image not found", nil, nil)
		return
	}

	// Delete from R2
	_ = h.imageStorage.DeleteFile(img.URLSuffix)

	// Delete from DB
	if err := h.store.DeleteBlogImage(ctx, imageID); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to delete image", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "image deleted", nil, nil)
}

// ─── POST /blogs/{id}/cover ─────────────────────────────────────────────────────

func (h *BlogHandler) UploadCoverImage(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	// Max 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "file too large or invalid form", nil, err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, false, "missing file field", nil, err.Error())
		return
	}
	defer file.Close()

	// Delete old cover if exists
	if blog.CoverImageURL != "" {
		_ = h.imageStorage.DeleteFile(blog.CoverImageURL)
	}

	subDir := fmt.Sprintf("blogs/%s/covers", blogID)
	urlSuffix, err := h.imageStorage.SaveFile(subDir, header.Filename, file)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to upload cover image", nil, err.Error())
		return
	}

	if err := h.store.UpdateBlogFields(ctx, blogID, map[string]interface{}{
		"cover_image_url": urlSuffix,
	}, nil); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to update blog", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "cover image uploaded", map[string]string{
		"url_suffix": urlSuffix,
	}, nil)
}

// ─── DELETE /blogs/{id}/cover ───────────────────────────────────────────────────

func (h *BlogHandler) DeleteCoverImage(w http.ResponseWriter, r *http.Request) {
	blogID := chi.URLParam(r, "id")
	ctx := r.Context()
	current := auth.GetUserFromCtx(ctx)
	if current == nil {
		utils.WriteJSONResponse(w, http.StatusUnauthorized, false, "unauthorized", nil, nil)
		return
	}

	blog, err := h.store.GetBlogByID(ctx, blogID)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusNotFound, false, "blog not found", nil, err.Error())
		return
	}

	if !h.store.CanEditBlog(ctx, current, blog) {
		utils.WriteJSONResponse(w, http.StatusForbidden, false, "forbidden", nil, nil)
		return
	}

	if blog.CoverImageURL != "" {
		_ = h.imageStorage.DeleteFile(blog.CoverImageURL)
	}

	if err := h.store.UpdateBlogFields(ctx, blogID, map[string]interface{}{
		"cover_image_url": "",
	}, nil); err != nil {
		utils.WriteJSONResponse(w, http.StatusInternalServerError, false, "failed to update blog", nil, err.Error())
		return
	}

	utils.WriteJSONResponse(w, http.StatusOK, true, "cover image removed", nil, nil)
}
