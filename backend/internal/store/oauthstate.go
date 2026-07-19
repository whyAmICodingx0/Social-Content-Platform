package store

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type OAuthStateStore struct {
	rdb *redis.Client
}

func NewOAuthStateStore(rdb *redis.Client) *OAuthStateStore {
	return &OAuthStateStore{rdb: rdb}
}

// Create 產生 state 並存入(spec 5.1 第 1、2 步)。
func (s *OAuthStateStore) Create(ctx context.Context) (string, error) {
	state, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, oauthStateKey(state), "1", OAuthStateTTL).Err(); err != nil {
		return "", err
	}
	return state, nil
}

// Consume 一次性消費 state(spec 5.2 第 1 步的「Redis 一次性存在」)。
// 回傳 true = 存在且已刪除;false = 不存在(無效 / 已過期 / 已被用過)。
//
// 【事實 / 關鍵】用 GETDEL(Redis 6.2+)而不是先 GET 再 DEL:
// GETDEL 是單一原子操作。先 GET 再 DEL 有微小的 race window——
// 兩個帶同一 state 的請求可能同時通過 GET,一次性保證就破了。
// 和 slug 的教訓一樣:check-then-act 在併發下不可靠,要用原子操作。
func (s *OAuthStateStore) Consume(ctx context.Context, state string) (bool, error) {
	err := s.rdb.GetDel(ctx, oauthStateKey(state)).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err // Redis 掛掉:交由呼叫端回 503(spec 4.12)
	}
	return true, nil
}
