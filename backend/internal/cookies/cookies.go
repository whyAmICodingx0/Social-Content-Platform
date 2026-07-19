package cookies

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Cookie 名稱合約(spec 4.1)。集中定義,全專案只有這裡出現這些字串。
const (
	NameSID        = "sid"
	NamePendingSID = "pending_sid"
	NameOAuthState = "oauth_state"
)

// Max-Age 合約(spec 4.5,與各自的 Redis TTL 對齊)
const (
	sessionMaxAge = 604800 // 7 天
	pendingMaxAge = 1800   // 30 分鐘
	stateMaxAge   = 600    // 10 分鐘
)

// Manager 統一管三顆 cookie。Secure 由 config 注入(spec 4.6:dev=false、prod=true)。
type Manager struct {
	Secure bool
}

func (m *Manager) SetSession(c *gin.Context, sid string) {
	m.set(c, NameSID, sid, sessionMaxAge)
}
func (m *Manager) ClearSession(c *gin.Context) { m.clear(c, NameSID) }

func (m *Manager) SetPendingSignup(c *gin.Context, token string) {
	m.set(c, NamePendingSID, token, pendingMaxAge)
}
func (m *Manager) ClearPendingSignup(c *gin.Context) { m.clear(c, NamePendingSID) }

func (m *Manager) SetOAuthState(c *gin.Context, state string) {
	m.set(c, NameOAuthState, state, stateMaxAge)
}
func (m *Manager) ClearOAuthState(c *gin.Context) { m.clear(c, NameOAuthState) }

// 統一屬性(spec 4.5):HttpOnly、SameSite=Lax、Path=/、Secure 依環境。
// 【事實】直接用 net/http 的 SetCookie 而不是 gin 的 c.SetCookie,
// 因為後者無法直接指定 SameSite(要靠 c.SetSameSite 的隱式狀態,容易漏)。
func (m *Manager) set(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// 【事實】net/http 的規則:MaxAge < 0 會送出 Max-Age=0,即「立刻刪除」。
func (m *Manager) clear(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}
