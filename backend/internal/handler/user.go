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
	Svc *service.UserService
}

// ---------- PATCH /api/v1/me（spec 6.1） ----------

// patchMeRequest 只定義「可以改」的欄位。
//
// ★ 決策 #24 的落實處：username 和 email 故意**不**出現在這個 struct，
//
//	所以 BindStrict 的 DisallowUnknownFields 會把它們當 unknown field 擋下並回 400。
//	一個機制同時處理「未知欄位」和「唯讀欄位」，不需要額外的檢查程式碼。
//
// 用 *string 而非 string：才能區分「沒送這個欄位」(nil) 和「送了空字串」("")。
type patchMeRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

func (h *UserHandler) PatchMe(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok { // Required middleware 保證不會發生；防禦性
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
			// session 有效但 user 消失：極罕見（並行刪帳號），視同未認證
			api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		default:
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
		}
		return
	}

	api.OK(c, meJSON(updated)) // 沿用任務 F 的 Me shape
}

// ---------- GET /api/v1/users/{username}（spec 6.2） ----------

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

	api.OK(c, publicUserJSON(u))
}

// publicUserJSON：User 公開 shape（spec 11.1）。
// ★ 刻意不含 email 與 updated_at——公開頁不該洩漏聯絡方式。
//
//	這是「對外資料模型 ≠ DB 欄位」的具體實踐：repository 撈了完整資料，
//	由 handler 決定對外露出哪些。
func publicUserJSON(u *repository.User) gin.H {
	return gin.H{
		"id":           u.ID,
		"username":     u.Username,
		"display_name": u.DisplayName,
		"bio":          u.Bio,
		"avatar_url":   u.AvatarURL,
		"created_at":   u.CreatedAt.UTC(),
	}
}
