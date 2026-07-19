package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
)

// sessionValue 對應 spec 4.3:value 最小化,只放指向永久資料的 user_id。
// time.Time 的 JSON 序列化天然就是 RFC 3339,符合合約。
type sessionValue struct {
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionStore struct {
	rdb *redis.Client
}

func NewSessionStore(rdb *redis.Client) *SessionStore {
	return &SessionStore{rdb: rdb}
}

// Create 建立 session(spec 4.7):
// session ID 一律在這裡新產生(session fixation 防護,決策 #18),
// 呼叫端沒有任何辦法指定 ID。
func (s *SessionStore) Create(ctx context.Context, userID string) (string, error) {
	sid, err := NewToken()
	if err != nil {
		return "", err
	}
	val, err := json.Marshal(sessionValue{
		UserID:    userID,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, sessionKey(sid), val, SessionTTL).Err(); err != nil {
		return "", err
	}
	return sid, nil
}

// GetUserID 實作 middleware.SessionStore 介面。
// 錯誤對映(spec 4.8 / 4.12 的合約):
//
//	查無 key(無效或已過期)→ middleware.ErrSessionNotFound
//	Redis 連不上等基礎設施問題 → 包裝 middleware.ErrStoreUnavailable
//
// 用 %w 包裝讓上層的 errors.Is 判斷得到。
func (s *SessionStore) GetUserID(ctx context.Context, sessionID string) (string, error) {
	raw, err := s.rdb.Get(ctx, sessionKey(sessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", middleware.ErrSessionNotFound
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", middleware.ErrStoreUnavailable, err)
	}

	var v sessionValue
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		// value 損毀 → 視同無效 session(fail-safe:寧可要求重新登入)
		return "", middleware.ErrSessionNotFound
	}
	return v.UserID, nil
}

// Delete 刪除 session(登出用)。錯誤交由呼叫端決定
// (spec 4.10:登出是 best-effort,Redis 掛掉照樣清 cookie 回 204)。
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	return s.rdb.Del(ctx, sessionKey(sessionID)).Err()
}
