package ws

import (
	"encoding/json"
	"log"
)

// 事件型別（決策：envelope 的 type 欄位，為未來擴充預留）
const (
	EventMessageCreated      = "message.created"
	EventNotificationCreated = "notification.created" // P4-1
)

// Event 是 server → client 的訊息封包。
//
//	{ "type": "message.created", "data": { ... } }
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Notifier 把事件序列化後推給指定使用者。
//
// 【為什麼放在 handler 層呼叫而非 service】
// 事件的 data 必須與 HTTP 回應的形狀完全相同，否則前端會遇到
// 「同一筆資料兩種格式」。handler 已經有 messageJSON()，
// 讓它同時服務兩個出口，就不會有第二份序列化邏輯可以漂移。
type Notifier struct {
	Hub *Hub
}

// Broadcast 推送事件。
//
// WS 不保證送達（決策 #62）—— 序列化失敗只記 log，
// 絕不影響已經寫入 PostgreSQL 的結果。漏收的訊息由前端
// 重連後以 REST ?after= 補齊（P3-3）。
func (n *Notifier) Broadcast(userIDs []string, eventType string, data any) {
	if n == nil || n.Hub == nil {
		return
	}

	payload, err := json.Marshal(Event{Type: eventType, Data: data})
	if err != nil {
		log.Printf("ws: marshal event failed (type=%s): %v", eventType, err)
		return
	}

	n.Hub.SendToUsers(userIDs, payload)
}
