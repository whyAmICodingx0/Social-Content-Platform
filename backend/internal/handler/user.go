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

type UserHandler struct {
	Svc     *service.UserService
	Follows *service.FollowService // P2-3：公開個人頁的追蹤資訊
}

// ---------- PATCH /api/v1/me ----------

// patchMeRequest 只定義可以改的欄位。
// username 與 email 故意不出現 → BindStrict 會當成 unknown field 擋下（決策 #24）。
type patchMeRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

func (h *UserHandler) PatchMe(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req patchMeRequest
	if !api.BindStrict(c, &req) {
		return
	}

	updated, err := h.Svc.UpdateProfile(c.Request.Context(), u.ID, service.UpdateProfileInput{
		DisplayName: req.DisplayName,
		Bio:         req.Bio,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		var vErr *service.ValidationError
		switch {
		case errors.As(err, &vErr):
			api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
				"Request validation failed", map[string]string{vErr.Field: vErr.Message})
		case errors.Is(err, repository.ErrNotFound):
			api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		default:
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
		}
		return
	}

	api.OK(c, meJSON(updated))
}

// ---------- GET /api/v1/users/:username（optional auth，決策 #47） ----------

func (h *UserHandler) GetPublicProfile(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
		return
	}

	u, err := h.Svc.GetPublicProfile(c.Request.Context(), username)
	if errors.Is(err, repository.ErrNotFound) {
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	// P2-3：追蹤數字與狀態。viewerID 為 nil 時 followed_by_me 為 false（決策 #48）
	extras, err := h.Follows.GetProfileExtras(c.Request.Context(), u.ID, viewerID(c))
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	api.OK(c, publicUserJSON(u, extras))
}

// publicUserJSON：User 公開 shape（spec 11.1 + P2-3 擴充）。
// 刻意不含 email 與 updated_at。
func publicUserJSON(u *repository.User, e *service.ProfileExtras) gin.H {
	return gin.H{
		"id":              u.ID,
		"username":        u.Username,
		"display_name":    u.DisplayName,
		"bio":             u.Bio,
		"avatar_url":      u.AvatarURL,
		"created_at":      u.CreatedAt.UTC(),
		"follower_count":  e.FollowerCount,
		"following_count": e.FollowingCount,
		"followed_by_me":  e.FollowedByMe,
	}
}
