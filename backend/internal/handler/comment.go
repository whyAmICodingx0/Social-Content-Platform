package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

type CommentHandler struct {
	Svc *service.CommentService
}

// ---------- GET /api/v1/posts/:id/comments（純公開，決策 #47） ----------
//
// 刻意不用 optional auth：回應完全不因使用者而異
// （草稿對所有人一律 404，且不含個人化欄位）。
// 前端用 author.username 比對即可算出能否編輯 / 刪除。

func (h *CommentHandler) List(c *gin.Context) {
	q := api.ParsePageQuery(c)

	comments, total, err := h.Svc.List(c.Request.Context(), c.Param("id"), q.Limit, q.Offset())
	if err != nil {
		h.fail(c, err)
		return
	}

	items := make([]gin.H, 0, len(comments))
	for _, cm := range comments {
		items = append(items, commentJSON(cm))
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

// ---------- POST /api/v1/posts/:id/comments ----------

type createCommentRequest struct {
	Content string `json:"content"`
}

func (h *CommentHandler) Create(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req createCommentRequest
	if !api.BindStrict(c, &req) {
		return
	}

	cm, err := h.Svc.Create(c.Request.Context(), u.ID, c.Param("id"), req.Content)
	if err != nil {
		h.fail(c, err)
		return
	}
	api.Created(c, commentJSON(cm))
}

// ---------- PATCH /api/v1/comments/:id ----------

type updateCommentRequest struct {
	Content string `json:"content"`
}

func (h *CommentHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req updateCommentRequest
	if !api.BindStrict(c, &req) {
		return
	}

	cm, err := h.Svc.Update(c.Request.Context(), u.ID, c.Param("id"), req.Content)
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, commentJSON(cm))
}

// ---------- DELETE /api/v1/comments/:id ----------

func (h *CommentHandler) Delete(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	if err := h.Svc.Delete(c.Request.Context(), u.ID, c.Param("id")); err != nil {
		h.fail(c, err)
		return
	}
	api.NoContent(c)
}

// ---------- 共用 ----------

func (h *CommentHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		// 草稿也走這條（決策 #43）
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "not found")
	case errors.Is(err, service.ErrForbidden):
		api.Fail(c, http.StatusForbidden, api.CodeForbidden,
			"you do not have permission to modify this comment")
	default:
		log.Printf("<handler>.<method>: unexpected error: %v", err)
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// commentJSON：Comment shape（見 C-0）。
// edited 由後端計算——時間比較有微秒精度問題，統一在一處判斷較安全。
func commentJSON(c *repository.Comment) gin.H {
	return gin.H{
		"id":      c.ID,
		"content": c.Content,
		"author": gin.H{
			"username":     c.AuthorUsername,
			"display_name": c.AuthorDisplayName,
			"avatar_url":   c.AuthorAvatarURL,
		},
		"created_at": c.CreatedAt.UTC(),
		"updated_at": c.UpdatedAt.UTC(),
		"edited":     c.UpdatedAt.After(c.CreatedAt),
	}
}
