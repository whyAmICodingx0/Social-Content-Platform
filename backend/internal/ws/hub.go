// Package ws 提供 WebSocket 連線管理。
//
// 設計要點（決策 #62、#63、#76）：
//   - WebSocket 只負責 server → client 的單向推送；所有寫入走 HTTP
//   - Hub 存於 Go 程序記憶體，Phase 3 僅支援單一 instance
//   - 一個使用者可有多條連線（多分頁、多裝置）
package ws

import (
	"sync"
)

// Conn 是 Hub 眼中的一條連線。
//
// 抽象成 interface 而非直接用 *websocket.Conn，是為了讓 Hub
// 能在不開真實網路連線的情況下被測試（見 hub_test.go）。
// 這也讓 Hub 完全不依賴 gorilla —— 它只管「誰在線上」與「往誰送」。
type Conn interface {
	// UserID 回傳這條連線所屬的使用者。
	UserID() string
	// SID 回傳建立此連線時使用的 session id（供 CloseBySID 使用）。
	SID() string
	// Send 嘗試把訊息放進該連線的傳送佇列。
	// 佇列已滿時回傳 false —— 呼叫端應據此關閉這條慢速連線。
	Send(payload []byte) bool
	// Close 關閉底層連線。必須可安全重複呼叫。
	Close()
}

// Hub 管理所有在線連線。
//
// 併發模型：所有對 conns 的存取都由 mu 保護。
// 選擇 mutex 而非「單一 goroutine + channel」的理由：
// 操作都極短（map 讀寫），mutex 更直觀，也不會讓廣播被排隊阻塞。
type Hub struct {
	mu sync.RWMutex
	// user_id → 該使用者的所有連線
	conns map[string]map[Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[string]map[Conn]struct{}),
	}
}

// Register 加入一條連線。
func (h *Hub) Register(c Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uid := c.UserID()
	if h.conns[uid] == nil {
		h.conns[uid] = make(map[Conn]struct{})
	}
	h.conns[uid][c] = struct{}{}
}

// Unregister 移除一條連線。
// 必須可安全重複呼叫 —— readPump 與 writePump 都可能觸發它。
func (h *Hub) Unregister(c Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uid := c.UserID()
	set, ok := h.conns[uid]
	if !ok {
		return
	}
	delete(set, c)
	// 使用者已無連線 → 移除整個 entry，避免 map 無限成長
	if len(set) == 0 {
		delete(h.conns, uid)
	}
}

// SendToUser 把 payload 推給某使用者的所有連線。
//
// 【重要】此方法不直接寫入連線 —— 它只往各連線的 send channel 投遞，
// 由該連線唯一的 writePump goroutine 負責實際寫入。
// gorilla/websocket 併發寫會 panic，這是避免它的關鍵設計。
//
// 慢速連線（佇列已滿）會被關閉，避免記憶體與 goroutine 無限累積。
func (h *Hub) SendToUser(userID string, payload []byte) {
	// 先在鎖內複製一份連線清單，再於鎖外執行 Send/Close。
	// 理由：Close 可能觸發 Unregister（需要相同的鎖），
	// 在鎖內呼叫會造成 deadlock。
	h.mu.RLock()
	targets := make([]Conn, 0, len(h.conns[userID]))
	for c := range h.conns[userID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if !c.Send(payload) {
			// 佇列已滿 → 這是慢速或卡死的客戶端
			c.Close()
		}
	}
}

// SendToUsers 推給多個使用者（P3-2 傳訊息時，收件者與發送者自己）。
func (h *Hub) SendToUsers(userIDs []string, payload []byte) {
	for _, uid := range userIDs {
		h.SendToUser(uid, payload)
	}
}

// CloseBySID 關閉使用指定 session 建立的所有連線（決策 #77）。
//
// 定位是「加速撤銷」而非安全保障 —— 兜底是連線期間的 pong 重驗。
// 必須冪等：找不到任何連線時安靜返回，不影響 logout 的回應。
//
// 注意只關閉同 sid 的連線，不是該使用者的全部連線 ——
// 別把使用者其他裝置踢掉。
func (h *Hub) CloseBySID(sid string) {
	if sid == "" {
		return
	}

	h.mu.RLock()
	var targets []Conn
	for _, set := range h.conns {
		for c := range set {
			if c.SID() == sid {
				targets = append(targets, c)
			}
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.Close()
	}
}

// IsOnline 回報某使用者是否有任何連線（除錯與未來的在線狀態功能用）。
func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns[userID]) > 0
}

// Stats 回傳目前的連線統計（除錯端點用）。
func (h *Hub) Stats() (users int, connections int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users = len(h.conns)
	for _, set := range h.conns {
		connections += len(set)
	}
	return
}
