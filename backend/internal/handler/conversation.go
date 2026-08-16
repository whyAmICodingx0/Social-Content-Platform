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

type ConversationHandler struct {
	Svc *service.ConversationService
}

// ---------- POST /api/v1/conversations ----------

type createConversationRequest struct {
	Username string `json:"username"`
}

// find-or-create。新建 → 201；既有 → 200。兩者 body 相同。
//
// 前端每次開啟對話頁都會呼叫它（路由是 /messages/@username），
// 因此冪等是必要的，而非只是好習慣。
func (h *ConversationHandler) Create(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req createConversationRequest
	if !api.BindStrict(c, &req) {
		return
	}
	if req.Username == "" {
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed",
			map[string]string{"username": "must not be empty"})
		return
	}

	conv, created, err := h.Svc.FindOrCreate(c.Request.Context(), u.ID, req.Username)
	if err != nil {
		h.fail(c, err)
		return
	}

	if created {
		api.Created(c, conversationJSON(conv))
		return
	}
	api.OK(c, conversationJSON(conv))
}

func (h *ConversationHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, service.ErrCannotMessageSelf):
		api.Fail(c, http.StatusBadRequest, api.CodeCannotMessageSelf,
			"you cannot start a conversation with yourself")
	case errors.Is(err, repository.ErrNotFound):
		// 非參與者、對話不存在、對方已軟刪，一律 404（決策 #73）
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "not found")
	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// conversationJSON：Conversation shape。
//
// last_message 與 unread_count 在 P3-3 / P3-4 補上；
// 此處先不輸出，避免前端誤以為「永遠是 null / 0」。
func conversationJSON(c *repository.Conversation) gin.H {
	return gin.H{
		"id": c.ID,
		"other_user": gin.H{
			"username":     c.OtherUsername,
			"display_name": c.OtherDisplayName,
			"avatar_url":   c.OtherAvatarURL,
		},
		"created_at": c.CreatedAt.UTC(),
	}
}
