package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// 兩個 sentinel error 對應 spec 4.8 / 4.12 的兩種失敗:
//
//	ErrSessionNotFound  → 憑證無效(required 回 401;optional 視同匿名)
//	ErrStoreUnavailable → 儲存後端掛掉(required 回 503;optional 降級匿名)
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrStoreUnavailable = errors.New("session store unavailable")
)

type SessionStore interface {
	GetUserID(ctx context.Context, sessionID string) (string, error)
}

const ctxUserKey = "auth.user"

type Auth struct {
	Store SessionStore
	Users *repository.UserRepository
}

var errNoCookie = errors.New("no session cookie")

// Required:受保護端點(spec 4.8)。
// 憑證問題一律 401 且不區分原因(spec 4.11 安全設計);
// 依賴服務掛掉 → 503 fail closed(spec 4.12)。
func (a *Auth) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := a.resolve(c)
		switch {
		case err == nil:
			c.Set(ctxUserKey, user)
			c.Next()
		case errors.Is(err, errNoCookie), errors.Is(err, ErrSessionNotFound):
			api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated,
				"Authentication required")
		default: // Redis 掛、DB 掛……一律 503,不放行
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
		}
	}
}

// Optional:公開但行為隨身分變化的端點(spec 4.8)。
// 任何失敗(含 Redis 掛掉)都視同匿名續行、絕不回 401 ——
// 降級為匿名只會讓權限變小,是 fail-safe 方向。
func (a *Auth) Optional() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, err := a.resolve(c); err == nil {
			c.Set(ctxUserKey, user)
		}
		c.Next()
	}
}

func (a *Auth) resolve(c *gin.Context) (*repository.User, error) {
	sid, err := c.Cookie(cookies.NameSID)
	if err != nil || sid == "" {
		return nil, errNoCookie
	}
	userID, err := a.Store.GetUserID(c.Request.Context(), sid)
	if err != nil {
		return nil, err
	}
	user, err := a.Users.GetByID(c.Request.Context(), userID)
	if errors.Is(err, repository.ErrNotFound) {
		// session 指向已刪除 / 不存在的 user → 視同憑證無效(spec 4.8 第 3 步)
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CurrentUser:handler 從 context 取出已驗證的使用者。
// 第二個回傳值 false = 匿名(optional auth 下是正常情況)。
func CurrentUser(c *gin.Context) (*repository.User, bool) {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return nil, false
	}
	u, ok := v.(*repository.User)
	return u, ok
}
