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

type FollowHandler struct {
	Svc *service.FollowService
}

// PUT /api/v1/users/:username/follow（決策 #44）
func (h *FollowHandler) Follow(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	st, err := h.Svc.Follow(c.Request.Context(), u.ID, c.Param("username"))
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, followStateJSON(st))
}

// DELETE /api/v1/users/:username/follow
// ⚠️ 回 200 + body（不是 204）：前端需要更新後的計數（決策 #44）
func (h *FollowHandler) Unfollow(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	st, err := h.Svc.Unfollow(c.Request.Context(), u.ID, c.Param("username"))
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, followStateJSON(st))
}

// GET /api/v1/users/:username/followers（選做，純公開）
func (h *FollowHandler) ListFollowers(c *gin.Context) {
	q := api.ParsePageQuery(c)
	users, total, err := h.Svc.ListFollowers(c.Request.Context(), c.Param("username"), q.Limit, q.Offset())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.respondUserList(c, q, users, total)
}

// GET /api/v1/users/:username/following（選做，純公開）
func (h *FollowHandler) ListFollowing(c *gin.Context) {
	q := api.ParsePageQuery(c)
	users, total, err := h.Svc.ListFollowing(c.Request.Context(), c.Param("username"), q.Limit, q.Offset())
	if err != nil {
		h.fail(c, err)
		return
	}
	h.respondUserList(c, q, users, total)
}

func (h *FollowHandler) respondUserList(c *gin.Context, q api.PageQuery, users []*repository.FollowUser, total int) {
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":           u.ID,
			"username":     u.Username,
			"display_name": u.DisplayName,
			"avatar_url":   u.AvatarURL,
			"bio":          u.Bio,
		})
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

func (h *FollowHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		// 追蹤自己走這條
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

func followStateJSON(st *repository.FollowState) gin.H {
	return gin.H{
		"follower_count": st.FollowerCount,
		"followed_by_me": st.FollowedByMe,
	}
}
