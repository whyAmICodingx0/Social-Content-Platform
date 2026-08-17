package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/ws"
)

type MessageHandler struct {
	Svc      *service.MessageService
	Notifier *ws.Notifier
}

// ---------- POST /api/v1/conversations/:id/messages ----------

type createMessageRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content"`
}

func (h *MessageHandler) Create(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req createMessageRequest
	if !api.BindStrict(c, &req) {
		return
	}

	msg, conv, isExisting, err := h.Svc.Send(
		c.Request.Context(), u.ID, c.Param("id"), req.ClientMessageID, req.Content)
	if err != nil {
		h.fail(c, err)
		return
	}

	payload := messageJSON(msg)

	if isExisting {
		// 冪等重送：這則訊息先前已推送過，不重複推。
		// 就算當時推送失敗也無妨 —— WS 不保證送達，
		// 前端重連時會用 ?after= 補齊（P3-3）。
		api.OK(c, payload)
		return
	}

	// 推給兩位參與者，包含發送者自己的其他分頁。
	// 推送失敗不影響 HTTP 回應：PostgreSQL 已經寫入，那才是權威。
	h.Notifier.Broadcast(conv.Participants(), ws.EventMessageCreated, payload)

	api.Created(c, payload)
}

// ---------- GET /api/v1/conversations/:id/messages ----------
//
// cursor（keyset）分頁，決策 #67。回應一律以 (created_at, id) 遞增排序。
func (h *MessageHandler) List(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit")) // 非數字得到 0，service 會套預設值

	msgs, hasMore, err := h.Svc.ListMessages(c.Request.Context(), service.ListMessagesInput{
		ConversationID: c.Param("id"),
		ViewerID:       u.ID,
		Before:         c.Query("before"),
		After:          c.Query("after"),
		Limit:          limit,
	})
	if err != nil {
		h.fail(c, err)
		return
	}

	items := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, messageJSON(m))
	}

	// cursor 分頁不回 total / page —— 那是 offset 分頁的概念
	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"has_more": hasMore,
	})
}

func (h *MessageHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, service.ErrInvalidCursor):
		api.Fail(c, http.StatusBadRequest, api.CodeInvalidCursor,
			"cursor does not belong to this conversation")
	case errors.Is(err, repository.ErrNotFound):
		// 對話不存在 / 非參與者 / 對方已軟刪，一律 404（決策 #73）
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "not found")
	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// messageJSON：Message shape。
//
// ⚠️ 這個函式同時服務兩個出口 —— HTTP 回應與 WebSocket 事件的 data。
// 兩者必須完全一致，所以只能有這一份實作。
//
// 回傳 client_message_id 是為了讓前端比對並替換樂觀顯示的 pending 訊息。
func messageJSON(m *repository.Message) gin.H {
	return gin.H{
		"id":                m.ID,
		"conversation_id":   m.ConversationID,
		"client_message_id": m.ClientMessageID,
		"content":           m.Content,
		"sender": gin.H{
			"username":     m.SenderUsername,
			"display_name": m.SenderDisplayName,
			"avatar_url":   m.SenderAvatarURL,
		},
		"created_at": m.CreatedAt.UTC(),
	}
}
