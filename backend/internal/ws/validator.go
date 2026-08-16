package ws

import (
	"context"
	"errors"
	"time"

	// ErrSessionNotFound 定義在 middleware package（與 SessionStore 介面同處），
	// store.SessionStore.GetUserID 會回傳它
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
)

// sessionRecheckTimeout：重驗 session 的逾時。
//
// ⚠️ 這個 timeout 是必須的，不是可有可無的保險。
// 重驗跑在 readPump 的 goroutine 裡，而此時它「不在 ReadMessage 中」——
// read deadline 完全管不到它。若用 context.Background()，
// Redis（正式環境是 Upstash）一旦 hang 住，
// 那條連線的 goroutine 就永久洩漏，且不會有任何錯誤訊息。
const sessionRecheckTimeout = 3 * time.Second

// sessionStore 是 store.SessionStore 需要滿足的最小介面。
// 簽章與 store.SessionStore.GetUserID 一致。
type sessionStore interface {
	GetUserID(ctx context.Context, sessionID string) (string, error)
}

// RedisSessionValidator 用既有的 session store 實作 SessionValidator。
type RedisSessionValidator struct {
	Store sessionStore
}

// Validate 區分「確定失效」與「無法確認」（決策 #70）。
//
// store.SessionStore.GetUserID 的錯誤語意：
//   - redis.Nil（查無此 key）→ middleware.ErrSessionNotFound
//   - 其他錯誤（含 context 逾時）→ 包裝 middleware.ErrStoreUnavailable
//
// 因此逾時會落進 default 分支，回傳 (false, err) → 呼叫端不關閉連線。
func (v *RedisSessionValidator) Validate(sid string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionRecheckTimeout)
	defer cancel()

	_, err := v.Store.GetUserID(ctx, sid)

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, middleware.ErrSessionNotFound):
		// 確定失效：登出、過期、被撤銷
		return false, nil
	default:
		// 無法確認（Redis 連不上、逾時）→ 交由呼叫端決定不關閉
		return false, err
	}
}
