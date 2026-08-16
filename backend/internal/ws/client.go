package ws

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// pongWait：多久沒收到 pong 就判定連線已死
	pongWait = 60 * time.Second

	// pingPeriod：送 ping 的間隔。
	// 必須明顯小於 pongWait，留出網路往返與抖動的餘裕。
	// 慣例是 pongWait 的九成。
	pingPeriod = pongWait * 9 / 10

	// writeWait：單次寫入的逾時
	writeWait = 10 * time.Second

	// maxMessageSize：客戶端不該送任何訊息（決策 #76），
	// 這純粹是防禦 —— 一行程式碼擋掉超大 frame 造成的 OOM
	maxMessageSize = 4096

	// sendBufferSize：每條連線的傳送佇列容量。
	// 有界是必須的：無界會讓慢速客戶端吃光記憶體
	sendBufferSize = 64
)

// errSessionRevoked 用來讓 pong handler 中止 ReadMessage 迴圈。
//
// 刻意不用 websocket.ErrCloseSent —— 那個 error 的語意是
// 「已送出 close frame 後仍嘗試寫入」，跟 session 撤銷無關，
// 出現在 log 裡會誤導未來的自己。
var errSessionRevoked = errors.New("ws: session revoked")

// SessionValidator 讓連線能定期重驗 session（決策 #70）。
// 抽象成 interface 是為了讓 ws package 不直接依賴 store/middleware。
type SessionValidator interface {
	// Validate 回傳 (該 session 是否仍有效, 檢查本身是否失敗)。
	//   (true,  nil)  → 有效
	//   (false, nil)  → 確定已失效 → 應關閉連線
	//   (_,     err)  → 無法確認（如 Redis 連不上）→ 不關閉
	Validate(sid string) (bool, error)
}

// Client 是一條真實的 WebSocket 連線。
//
// 併發模型（本檔案最重要的部分）：
//   - readPump 是唯一的讀取者
//   - writePump 是唯一的寫入者 —— gorilla 併發寫會 panic
//   - 其他地方只能透過 send channel 間接寫入
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	userID string
	sid    string

	send chan []byte

	// done 在 Close 時關閉，作為兩個 pump 的結束訊號。
	//
	// ⚠️ 刻意「不」關閉 send channel。
	//
	// send 有多個潛在寫入者（任何呼叫 Hub.SendToUser 的 goroutine），
	// 而 Close 是從讀取端呼叫的。Go 的規則是：對已關閉的 channel 寫入
	// 會直接 panic，且寫入端無法用 select 或 recover 避開。
	//
	// 實際會踩到的路徑：SendToUser 在 RLock 內複製完 targets、RUnlock
	// 之後，readPump 因網路錯誤觸發 Close()，接著 c.Send() 就撞上
	// 已關閉的 channel —— panic 發生在 HTTP handler 的 goroutine，
	// 那裡沒有 recover，整個程序掛掉。
	//
	// 通用規則：有多個 sender 時，receiver 不可關閉 channel。
	// send 交給 GC 回收即可。
	done chan struct{}

	// closeOnce 確保 Close 可安全重複呼叫 ——
	// readPump 與 writePump 結束時都會呼叫它
	closeOnce sync.Once

	validator SessionValidator
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, sid string, v SessionValidator) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		userID:    userID,
		sid:       sid,
		send:      make(chan []byte, sendBufferSize),
		done:      make(chan struct{}),
		validator: v,
	}
}

func (c *Client) UserID() string { return c.userID }
func (c *Client) SID() string    { return c.sid }

// Send 把訊息投遞到傳送佇列。
// 佇列已滿或連線已關閉時回傳 false —— Hub 會據此關閉這條連線。
//
// 用 select + default 而非直接送入：直接送入會在佇列滿時阻塞，
// 讓整個廣播被一個慢客戶端卡住。
//
// 先檢查 done 是快速路徑。即使在檢查通過之後才 Close，
// 訊息也只是留在 buffered channel 裡等 GC —— 不會 panic。
// 這正是「不關閉 send」換來的安全性。
func (c *Client) Send(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// Close 關閉連線。可安全重複呼叫（sync.Once 保證）。
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

// Run 啟動這條連線的讀寫迴圈。呼叫後會註冊到 Hub。
func (c *Client) Run() {
	c.hub.Register(c)
	go c.writePump()
	go c.readPump()
}

// readPump 是這條連線唯一的讀取者。
//
// 職責有三：
//  1. 偵測斷線（read 回傳錯誤時）
//  2. 處理 pong，延長 read deadline
//  3. 丟棄客戶端送來的任何訊息（決策 #76：WS 為單向推送）
func (c *Client) readPump() {
	// panic recovery：一條連線的 panic 不能拖垮整個服務
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ws: readPump panic (user=%s): %v", c.userID, r)
		}
		c.hub.Unregister(c)
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// 每次收到 pong：延長 deadline，並順便重驗 session（決策 #70）
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))

		if c.validator != nil {
			valid, err := c.validator.Validate(c.sid)
			switch {
			case err != nil:
				// 無法確認（Redis 連不上、逾時）→ 不關閉。
				//
				// 這與決策 #15（required auth 在 Redis 掛掉時 fail closed）
				// 看似衝突，但語意不同：那是「不知道你是誰就不給進門」，
				// 這裡是「已經在門內的人，不因為門房打瞌睡而被趕出去」。
				// 使用者已通過握手驗證，暫時無法確認不等於憑證失效。
				log.Printf("ws: session recheck unavailable (user=%s): %v", c.userID, err)
			case !valid:
				// 確定已失效（登出、過期、被撤銷）→ 讓 ReadMessage 回傳此 error
				return errSessionRevoked
			}
		}
		return nil
	})

	for {
		// 客戶端不該送任何訊息（決策 #76）。
		// 這裡讀取只是為了偵測斷線與觸發 pong handler ——
		// 讀到的內容一律丟棄，server 不解析任何客戶端 WS 訊息。
		if _, _, err := c.conn.ReadMessage(); err != nil {
			switch {
			case errors.Is(err, errSessionRevoked):
				log.Printf("ws: session invalid, closing (user=%s)", c.userID)
			case websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure):
				log.Printf("ws: unexpected close (user=%s): %v", c.userID, err)
			}
			return
		}
	}
}

// writePump 是這條連線唯一的寫入者。
//
// ⚠️ 這是整個 WebSocket 實作最關鍵的紀律：
// gorilla/websocket 不允許併發寫入，兩個 goroutine 同時寫會 panic。
// 因此所有寫入 —— 包含推送訊息與 ping frame —— 都必須發生在這裡。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("ws: writePump panic (user=%s): %v", c.userID, r)
		}
		ticker.Stop()
		c.hub.Unregister(c)
		c.Close()
	}()

	for {
		// 優先檢查是否已關閉。
		// select 在多個 case 同時就緒時是隨機挑選的，
		// 這個前置檢查讓「已關閉」的情況能確定性地立刻返回。
		select {
		case <-c.done:
			return
		default:
		}

		select {
		case <-c.done:
			// 連線已關閉（send 永遠不會被關閉，所以結束訊號是 done）
			return

		case payload := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}

		case <-ticker.C:
			// 主動送 ping（決策 #69）。
			// 【事實】瀏覽器的 WebSocket API 沒有暴露 ping 方法，
			// ping/pong 是協定層由瀏覽器自動處理 ——
			// 所以心跳必須由 server 發起，前端不需要任何程式碼。
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteControl(websocket.PingMessage, nil,
				time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

// NewUpgrader 建立 upgrader，並掛上 Origin 檢查。
//
// ⚠️ Origin 檢查是必做，不是加分項（決策 #64）。
// 【事實】WebSocket 握手雖然是 HTTP 請求，但不受 SameSite cookie 保護 ——
// 跨站發起的 WS 連線仍會帶上 cookie。若不驗 Origin，
// 任何網站都能用你使用者的身分建立連線（Cross-Site WebSocket Hijacking）。
//
// 現有的 CSRF middleware 只擋 POST/PATCH/DELETE，而握手是 GET，
// 會直接放行 —— 必須在 upgrade 前另外檢查。
func NewUpgrader(allowedOrigins []string) *websocket.Upgrader {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// 缺少 Origin 也拒絕（決策 #64）。
			// 非瀏覽器客戶端不在 Phase 3 的支援範圍內。
			if origin == "" {
				return false
			}
			_, ok := allowed[origin]
			return ok
		},
	}
}
