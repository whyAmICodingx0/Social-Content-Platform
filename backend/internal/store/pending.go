package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrPendingNotFound:pending 不存在 / 已過期 / 已被消費。
// 任務 F 的 signup handler 會把它對映成 401 UNAUTHENTICATED(spec 5.3)。
var ErrPendingNotFound = errors.New("store: pending signup not found")

// PendingSignup 對應 spec 4.3 的 pending_signup value。
type PendingSignup struct {
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	Picture        string    `json:"picture"`
	CreatedAt      time.Time `json:"created_at"`
}

type PendingSignupStore struct {
	rdb *redis.Client
}

func NewPendingSignupStore(rdb *redis.Client) *PendingSignupStore {
	return &PendingSignupStore{rdb: rdb}
}

// Create 存入 pending 資料,回傳 token(spec 5.2 新用戶分支)。
func (s *PendingSignupStore) Create(ctx context.Context, p PendingSignup) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	p.CreatedAt = time.Now().UTC()
	val, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, pendingKey(token), val, PendingTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// Get 讀取 pending 資料(spec 5.3 第 1 步:先讀、不刪——
// 刪除要等 transaction 成功後才做,這是失敗收斂表的關鍵順序)。
func (s *PendingSignupStore) Get(ctx context.Context, token string) (*PendingSignup, error) {
	raw, err := s.rdb.Get(ctx, pendingKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrPendingNotFound
	}
	if err != nil {
		return nil, err
	}
	var p PendingSignup
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, ErrPendingNotFound
	}
	return &p, nil
}

// Delete 刪除 pending(spec 5.3 第 5 步,best-effort:
// 失敗不影響回應,殘留 key 由 TTL 收拾)。
func (s *PendingSignupStore) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, pendingKey(token)).Err()
}
