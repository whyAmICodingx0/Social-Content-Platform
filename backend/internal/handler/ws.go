package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/ws"
)

type WSHandler struct {
	Hub       *ws.Hub
	Upgrader  *websocket.Upgrader
	Validator ws.SessionValidator
}

// GET /api/v1/ws
//
// 兩道檢查（決策 #64）：
//  1. auth.Required() middleware 已驗過 sid session（路由層掛上）
//  2. Upgrader.CheckOrigin 驗 Origin allowlist（在 Upgrade 時執行）
func (h *WSHandler) Serve(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	// 用常數而非字面字串 "sid"：cookie 名稱散落兩處遲早會不一致
	sid, err := c.Cookie(cookies.NameSID)
	if err != nil || sid == "" {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	// Upgrade 會在失敗時自行寫入 HTTP 錯誤回應
	//（Origin 檢查不通過時是 403）
	conn, err := h.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws: upgrade failed (user=%s): %v", u.ID, err)
		return
	}

	// repository.User.ID 是 string，直接傳入即可
	ws.NewClient(h.Hub, conn, u.ID, sid, h.Validator).Run()
}

// GET /api/v1/dev/ws-stats（僅 dev 環境註冊，P3-1 驗收用）
func (h *WSHandler) Stats(c *gin.Context) {
	users, conns := h.Hub.Stats()
	api.OK(c, gin.H{"online_users": users, "connections": conns})
}
