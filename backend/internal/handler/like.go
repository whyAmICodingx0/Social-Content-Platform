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

type LikeHandler struct {
	Svc *service.LikeService
}

// PUT /api/v1/posts/:id/like（決策 #44）
func (h *LikeHandler) Like(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	st, err := h.Svc.Like(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, likeStateJSON(st))
}

// DELETE /api/v1/posts/:id/like
// ⚠️ 回 200 + body（不是 204）：前端需要更新後的計數（決策 #44）。
func (h *LikeHandler) Unlike(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	st, err := h.Svc.Unlike(c.Request.Context(), u.ID, c.Param("id"))
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, likeStateJSON(st))
}

func (h *LikeHandler) fail(c *gin.Context, err error) {
	if errors.Is(err, repository.ErrNotFound) {
		// 決策 #43：草稿也走這條，對作者本人同樣是 404
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "post not found")
		return
	}
	api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
		"Service temporarily unavailable")
}

func likeStateJSON(st *repository.LikeState) gin.H {
	return gin.H{
		"like_count":  st.LikeCount,
		"liked_by_me": st.LikedByMe,
	}
}
