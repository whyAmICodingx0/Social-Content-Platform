package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

type PostHandler struct {
	Svc *service.PostService
}

type createPostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Status  *string  `json:"status"`
	Tags    []string `json:"tags"`
}

func (h *PostHandler) Create(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req createPostRequest
	if !api.BindStrict(c, &req) {
		return
	}

	post, err := h.Svc.Create(c.Request.Context(), u.ID, service.CreatePostInput{
		Title: req.Title, Content: req.Content, Status: req.Status, Tags: req.Tags,
	})
	if err != nil {
		h.failPost(c, err)
		return
	}

	post.AuthorUsername = u.Username
	post.AuthorDisplayName = u.DisplayName
	post.AuthorAvatarURL = u.AvatarURL

	api.Created(c, postDetailJSON(post))
}

func (h *PostHandler) GetBySlug(c *gin.Context) {
	viewerID := ""
	if u, ok := middleware.CurrentUser(c); ok {
		viewerID = u.ID
	}

	post, err := h.Svc.GetForReader(c.Request.Context(),
		c.Param("username"), c.Param("slug"), viewerID)
	if err != nil {
		h.failPost(c, err)
		return
	}
	api.OK(c, postDetailJSON(post))
}

type updatePostRequest struct {
	Title   *string  `json:"title"`
	Content *string  `json:"content"`
	Status  *string  `json:"status"`
	Tags    []string `json:"tags"`
}

func (h *PostHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req updatePostRequest
	if !api.BindStrict(c, &req) {
		return
	}

	post, err := h.Svc.Update(c.Request.Context(), u.ID, c.Param("id"), service.UpdatePostInput{
		Title:   req.Title,
		Content: req.Content,
		Status:  req.Status,
		Tags:    req.Tags,
		HasTags: req.Tags != nil,
	})
	if err != nil {
		h.failPost(c, err)
		return
	}

	post.AuthorUsername = u.Username
	post.AuthorDisplayName = u.DisplayName
	post.AuthorAvatarURL = u.AvatarURL

	api.OK(c, postDetailJSON(post))
}

func (h *PostHandler) Delete(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), u.ID, c.Param("id")); err != nil {
		h.failPost(c, err)
		return
	}
	api.NoContent(c)
}

func (h *PostHandler) failPost(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "post not found")
	case errors.Is(err, service.ErrForbidden):
		api.Fail(c, http.StatusForbidden, api.CodeForbidden, "you are not the author of this post")
	case errors.Is(err, service.ErrSlugExhausted):
		api.Fail(c, http.StatusConflict, api.CodeSlugConflict, "could not generate a unique slug")
	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

func postDetailJSON(p *repository.Post) gin.H {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	var publishedAt any
	if p.PublishedAt != nil {
		publishedAt = p.PublishedAt.UTC()
	}
	return gin.H{
		"id":      p.ID,
		"slug":    p.Slug,
		"title":   p.Title,
		"content": p.ContentMD,
		"excerpt": p.Excerpt,
		"status":  p.Status,
		"author": gin.H{
			"username":     p.AuthorUsername,
			"display_name": p.AuthorDisplayName,
			"avatar_url":   p.AuthorAvatarURL,
		},
		"tags":         tags,
		"created_at":   p.CreatedAt.UTC(),
		"updated_at":   p.UpdatedAt.UTC(),
		"published_at": publishedAt,
	}
}
