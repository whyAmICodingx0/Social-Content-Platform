package store

import (
	"time"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
)

// TTL 合約(spec 4.4)
const (
	SessionTTL    = 7 * 24 * time.Hour // 604800 秒,絕對過期
	OAuthStateTTL = 10 * time.Minute
	PendingTTL    = 30 * time.Minute
)

// key 命名合約(spec 4.2 / 決策 #11)。
// 前綴集中在 config.RedisKeyPrefix("scp:"),這裡是唯一組 key 的地方。
func sessionKey(id string) string       { return config.RedisKeyPrefix + "session:" + id }
func oauthStateKey(state string) string { return config.RedisKeyPrefix + "oauth_state:" + state }
func pendingKey(token string) string    { return config.RedisKeyPrefix + "pending_signup:" + token }
