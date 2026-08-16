package ws

import (
	"sync"
	"sync/atomic"
	"testing"
)

// fakeConn 是測試用的假連線 —— 不碰網路，只記錄行為。
// 這正是把 Conn 抽象成 interface 的價值：
// Hub 的併發正確性可以完全在記憶體中驗證。
//
// ⚠️ 但要知道它的盲點：fakeConn 用 mutex + slice 模擬佇列，
// 不是 channel。所以 Client 自身的 channel 生命週期問題
// （見 W-4 的 done channel）這裡測不出來。
type fakeConn struct {
	userID string
	sid    string

	mu       sync.Mutex
	received [][]byte
	closed   bool

	// full 為 true 時模擬「送信佇列已滿」的慢速客戶端
	full bool
}

func newFakeConn(userID, sid string) *fakeConn {
	return &fakeConn{userID: userID, sid: sid}
}

func (f *fakeConn) UserID() string { return f.userID }
func (f *fakeConn) SID() string    { return f.sid }

func (f *fakeConn) Send(payload []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.full || f.closed {
		return false
	}
	f.received = append(f.received, payload)
	return true
}

func (f *fakeConn) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

func (f *fakeConn) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.received)
}

func (f *fakeConn) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// --- 基本行為 ---

func TestHub_RegisterAndSend(t *testing.T) {
	h := NewHub()
	c := newFakeConn("user-1", "sid-1")

	h.Register(c)
	h.SendToUser("user-1", []byte("hello"))

	if got := c.count(); got != 1 {
		t.Fatalf("期望收到 1 則訊息，實際 %d", got)
	}
}

func TestHub_MultipleConnectionsPerUser(t *testing.T) {
	// 同一使用者開兩個分頁 —— 兩條連線都要收到
	h := NewHub()
	a := newFakeConn("user-1", "sid-1")
	b := newFakeConn("user-1", "sid-1")

	h.Register(a)
	h.Register(b)
	h.SendToUser("user-1", []byte("hello"))

	if a.count() != 1 || b.count() != 1 {
		t.Fatalf("兩條連線都應收到訊息，實際 a=%d b=%d", a.count(), b.count())
	}
}

func TestHub_UnregisterStopsDelivery(t *testing.T) {
	h := NewHub()
	c := newFakeConn("user-1", "sid-1")

	h.Register(c)
	h.Unregister(c)
	h.SendToUser("user-1", []byte("hello"))

	if c.count() != 0 {
		t.Fatal("已註銷的連線不該收到訊息")
	}
}

func TestHub_UnregisterIsIdempotent(t *testing.T) {
	// readPump 與 writePump 都可能觸發 Unregister，必須可重複呼叫
	h := NewHub()
	c := newFakeConn("user-1", "sid-1")

	h.Register(c)
	h.Unregister(c)
	h.Unregister(c) // 不應 panic

	if users, conns := h.Stats(); users != 0 || conns != 0 {
		t.Fatalf("註銷後應無殘留，實際 users=%d conns=%d", users, conns)
	}
}

func TestHub_SlowClientIsClosed(t *testing.T) {
	// 佇列已滿的連線應被關閉，避免記憶體與 goroutine 累積
	h := NewHub()
	slow := newFakeConn("user-1", "sid-1")
	slow.full = true

	h.Register(slow)
	h.SendToUser("user-1", []byte("hello"))

	if !slow.isClosed() {
		t.Fatal("慢速連線應被關閉")
	}
}

func TestHub_CloseBySID(t *testing.T) {
	h := NewHub()
	// 同一使用者的兩個裝置，不同 session
	desktop := newFakeConn("user-1", "sid-desktop")
	phone := newFakeConn("user-1", "sid-phone")

	h.Register(desktop)
	h.Register(phone)
	h.CloseBySID("sid-desktop")

	if !desktop.isClosed() {
		t.Fatal("同 sid 的連線應被關閉")
	}
	if phone.isClosed() {
		t.Fatal("其他裝置的連線不該被關閉")
	}
}

func TestHub_CloseBySIDNotFoundIsSafe(t *testing.T) {
	// logout 可能在 session 已失效時被呼叫，必須冪等
	h := NewHub()
	h.Register(newFakeConn("user-1", "sid-1"))

	h.CloseBySID("sid-does-not-exist") // 不應 panic
	h.CloseBySID("")                   // 空字串也要安全
}

func TestHub_SendToOfflineUserIsSafe(t *testing.T) {
	h := NewHub()
	h.SendToUser("nobody", []byte("hello")) // 不應 panic
}

// --- 併發測試（本檔案的重點，用 -race 執行） ---

func TestHub_ConcurrentRegisterUnregisterSend(t *testing.T) {
	h := NewHub()

	const (
		workers = 50
		rounds  = 100
		userID  = "user-1"
	)

	var wg sync.WaitGroup
	var sent int64

	// 一群 goroutine 不斷註冊與註銷
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c := newFakeConn(userID, "sid-x")
			for j := 0; j < rounds; j++ {
				h.Register(c)
				h.Unregister(c)
			}
		}(i)
	}

	// 同時另一群不斷推送
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				h.SendToUser(userID, []byte("x"))
				atomic.AddInt64(&sent, 1)
			}
		}()
	}

	// 同時查詢狀態
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				h.Stats()
				h.IsOnline(userID)
			}
		}()
	}

	wg.Wait()

	// 所有連線都已註銷，Hub 應為空 —— 驗證沒有 entry 洩漏
	if users, conns := h.Stats(); users != 0 || conns != 0 {
		t.Fatalf("併發操作後應無殘留，實際 users=%d conns=%d", users, conns)
	}
}

func TestHub_ConcurrentCloseBySID(t *testing.T) {
	// CloseBySID 會走訪整個 map，同時有人在註冊 —— 最容易出競態的組合
	h := NewHub()

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				c := newFakeConn("user-1", "sid-1")
				h.Register(c)
				h.Unregister(c)
			}
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				h.CloseBySID("sid-1")
			}
		}()
	}
	wg.Wait()
}
