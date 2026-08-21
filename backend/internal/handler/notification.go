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

type NotificationHandler struct {
	Svc *service.NotificationService
}

// ---------- GET /api/v1/notifications ----------

func (h *NotificationHandler) List(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	q := api.ParsePageQuery(c)

	items, total, err := h.Svc.List(c.Request.Context(), u.ID, q.Limit, q.Offset())
	if err != nil {
		h.fail(c, err)
		return
	}

	out := make([]gin.H, 0, len(items))
	for _, n := range items {
		out = append(out, NotificationJSON(n))
	}
	api.OKList(c, out, api.NewPagination(q, total))
}

// ---------- GET /api/v1/notifications/unread-count ----------

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	n, err := h.Svc.UnreadCount(c.Request.Context(), u.ID)
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, gin.H{"unread_count": n})
}

// ---------- POST /api/v1/notifications/read ----------

type markReadNotificationsRequest struct {
	IDs []string `json:"ids"`
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req markReadNotificationsRequest
	if !api.BindStrict(c, &req) {
		return
	}

	updated, unread, err := h.Svc.MarkRead(c.Request.Context(), u.ID, req.IDs)
	if err != nil {
		h.fail(c, err)
		return
	}

	// 沿用決策 #44「寫入端點回傳計數與狀態」，前端少一次往返
	api.OK(c, gin.H{"updated": updated, "unread_count": unread})
}

// ---------- POST /api/v1/notifications/read-all ----------
//
// ⚠️ 無 body。兩個注意事項：
//  1. **不可呼叫 api.BindStrict** —— 它對空 body 會回「request body is empty」的 400
//  2. CSRF middleware 不會擋：它是 `if c.Request.ContentLength != 0` 才檢查 Content-Type
//     （同 PUT /posts/{id}/like 的既有慣例）
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	updated, unread, err := h.Svc.MarkAllRead(c.Request.Context(), u.ID)
	if err != nil {
		h.fail(c, err)
		return
	}
	api.OK(c, gin.H{"updated": updated, "unread_count": unread})
}

// ---------- 共用 ----------

func (h *NotificationHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "not found")
	default:
		// 決策 #80：default 的定義就是「非預期的錯誤」，不 log 等於沒有線索
		log.Printf("notification handler: unexpected error: %v", err)
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// NotificationJSON：Notification shape。
//
// ⚠️ 匯出（大寫開頭）是因為 main.go 的 WS 推送也要用同一份實作 ——
// 事件的 data 必須與 HTTP 回應完全相同，否則兩份序列化遲早漂移
// （同 P3-2 的 messageJSON 原則）。
//
// is_read 由 read_at 算出（決策 #86：不設 is_read 欄位）。
func NotificationJSON(n *repository.Notification) gin.H {
	out := gin.H{
		"id":         n.ID,
		"type":       n.Type,
		"is_read":    n.IsRead(),
		"created_at": n.CreatedAt.UTC(),
		"actor":      nil,
		"target":     nil,
	}

	// actor 為 nil 代表觸發者已軟刪 → 前端顯示「已刪除的使用者」（決策 #91）
	if n.ActorUsername != nil {
		out["actor"] = gin.H{
			"username":     *n.ActorUsername,
			"display_name": n.ActorDisplayName,
			"avatar_url":   n.ActorAvatarURL,
		}
	}

	// target 為 nil 代表：type=follow，或相關內容已被刪除
	// → 前端顯示「內容已不存在」且不給跳轉連結（決策 #91）
	//
	// ⚠️ target.type **恆為 "post"**（見 N-0 §0.3 的合約申報）。
	//    comment 類型的 title 與 url 指向的是**文章**；
	//    留言被軟刪時 target 也是 nil，即使文章還在 —— 這是預期行為。
	if n.TargetType != nil && n.TargetAuthorUsername != nil && n.TargetSlug != nil {
		out["target"] = gin.H{
			"type":  *n.TargetType,
			"title": n.TargetTitle,
			"url":   "/@" + *n.TargetAuthorUsername + "/" + *n.TargetSlug,
		}
	}

	return out
}
