package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/googleoauth"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

type AuthHandler struct {
	Google   *googleoauth.Client
	Svc      *service.AuthService
	Sessions *store.SessionStore
	States   *store.OAuthStateStore
	Pendings *store.PendingSignupStore
	Cookies  *cookies.Manager
	Cfg      *config.Config
}

// ---------- GET /api/v1/auth/google/login(spec 5.1) ----------

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	state, err := h.States.Create(c.Request.Context())
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	h.Cookies.SetOAuthState(c, state) // 把 state 綁到這個瀏覽器(決策 #14)
	c.Redirect(http.StatusFound, h.Google.AuthURL(state))
}

// ---------- GET /api/v1/auth/google/callback(spec 5.2) ----------

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	ctx := c.Request.Context()

	// 使用者在 Google 按了拒絕
	if c.Query("error") != "" {
		h.redirectError(c, "access_denied")
		return
	}

	// 三重 state 驗證(決策 #14):query 存在、等於 cookie、Redis 一次性存在
	qState := c.Query("state")
	cState, cerr := c.Cookie(cookies.NameOAuthState)
	h.Cookies.ClearOAuthState(c) // 不論成敗都清(一次性)
	if qState == "" || cerr != nil || cState == "" || qState != cState {
		api.Fail(c, http.StatusBadRequest, api.CodeValidationError, "invalid oauth state")
		return
	}
	ok, err := h.States.Consume(ctx, qState)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	if !ok {
		api.Fail(c, http.StatusBadRequest, api.CodeValidationError, "invalid oauth state")
		return
	}

	code := c.Query("code")
	if code == "" {
		api.Fail(c, http.StatusBadRequest, api.CodeValidationError, "missing code")
		return
	}

	gu, err := h.Google.FetchUser(ctx, code)
	if err != nil {
		h.redirectError(c, "google_exchange_failed")
		return
	}
	if !gu.EmailVerified { // spec 5.2 第 2 步:未驗證的 email 不可信任
		h.redirectError(c, "email_not_verified")
		return
	}

	u, err := h.Svc.LoginWithGoogle(ctx, gu)
	switch {
	case err == nil: // 老用戶
		sid, serr := h.Sessions.Create(ctx, u.ID)
		if serr != nil {
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
			return
		}
		h.Cookies.SetSession(c, sid)
		c.Redirect(http.StatusFound, h.Cfg.PostLoginURL)

	case errors.Is(err, service.ErrNoAccount): // 新用戶 → pending,此刻 DB 零寫入
		token, perr := h.Pendings.Create(ctx, store.PendingSignup{
			Provider:       "google",
			ProviderUserID: gu.Sub,
			Email:          gu.Email,
			Name:           gu.Name,
			Picture:        gu.Picture,
		})
		if perr != nil {
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
			return
		}
		h.Cookies.SetPendingSignup(c, token)
		c.Redirect(http.StatusFound, h.Cfg.OnboardingURL)

	case errors.Is(err, service.ErrAccountUnavailable):
		// spec 5.2:資料異常(依附錄 B-1 政策不應出現),log + 錯誤頁
		log.Printf("callback anomaly: %v", err)
		h.redirectError(c, "account_unavailable")

	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

func (h *AuthHandler) redirectError(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, h.Cfg.AuthErrorURL+"?error="+code)
}

// ---------- POST /api/v1/auth/signup(spec 5.3) ----------

type signupRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	ctx := c.Request.Context()

	// 已持有有效正式 session → 409(spec 5.3 步驟 1)
	if sid, err := c.Cookie(cookies.NameSID); err == nil && sid != "" {
		if _, serr := h.Sessions.GetUserID(ctx, sid); serr == nil {
			api.Fail(c, http.StatusConflict, api.CodeConflict, "already signed in")
			return
		} else if errors.Is(serr, middleware.ErrStoreUnavailable) {
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
			return
		}
		// 無效殘留的 sid → 忽略,繼續
	}

	// 步驟 1:讀 pending(先不刪——失敗收斂表的關鍵順序)
	token, err := c.Cookie(cookies.NamePendingSID)
	if err != nil || token == "" {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	pending, err := h.Pendings.Get(ctx, token)
	if errors.Is(err, store.ErrPendingNotFound) {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	// 步驟 2:嚴格解析 body
	var req signupRequest
	if !api.BindStrict(c, &req) {
		return
	}

	// 步驟 3-4:service(正規化管線 + 快路徑 + transaction + 收斂)
	u, isExisting, err := h.Svc.Signup(ctx, pending, req.Username, req.DisplayName)
	if err != nil {
		var vErr *service.ValidationError
		switch {
		case errors.As(err, &vErr):
			api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
				"Request validation failed", map[string]string{vErr.Field: vErr.Message})
		case errors.Is(err, repository.ErrUsernameTaken):
			// pending 保留:使用者換個名字直接重試
			api.Fail(c, http.StatusConflict, api.CodeUsernameTaken, "username already taken")
		case errors.Is(err, repository.ErrEmailTaken):
			api.Fail(c, http.StatusConflict, api.CodeEmailTaken,
				"an account with this email already exists; please sign in with your original Google account")
		case errors.Is(err, repository.ErrUnexpectedConflict),
			errors.Is(err, service.ErrAccountUnavailable):
			// 修訂 5.3:資料異常 / 未預期 constraint → log + 500(非暫時性故障,不用 503)
			log.Printf("signup anomaly: %v", err)
			api.Fail(c, http.StatusInternalServerError, api.CodeInternalError,
				"Internal server error")
		default:
			api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
				"Service temporarily unavailable")
		}
		return
	}

	// 步驟 4:建 session(失敗 → 503;帳號已建立,重走 Google 登入即收斂)
	sid, err := h.Sessions.Create(ctx, u.ID)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	// 步驟 5:清 pending(best-effort,失敗由 TTL 收拾)
	_ = h.Pendings.Delete(ctx, token)
	h.Cookies.ClearPendingSignup(c)

	// 步驟 6:設 cookie、回應
	h.Cookies.SetSession(c, sid)
	if isExisting {
		api.OK(c, meJSON(u)) // 視同已註冊 → 200
		return
	}
	api.Created(c, meJSON(u)) // 全新帳號 → 201
}

// ---------- POST /api/v1/auth/logout(spec 4.10 / 5.4,冪等) ----------

func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	if sid, err := c.Cookie(cookies.NameSID); err == nil && sid != "" {
		_ = h.Sessions.Delete(ctx, sid) // Redis 掛掉也不影響回應
	}
	if token, err := c.Cookie(cookies.NamePendingSID); err == nil && token != "" {
		_ = h.Pendings.Delete(ctx, token)
	}
	h.Cookies.ClearSession(c)
	h.Cookies.ClearPendingSignup(c)
	api.NoContent(c)
}

// ---------- GET /api/v1/me(spec 5.5,required auth) ----------

func (h *AuthHandler) Me(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok { // Required middleware 保證不會發生;防禦性
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	api.OK(c, meJSON(u))
}

// meJSON:Me shape(spec 11.1)。
// 【事實】.UTC() 確保序列化成 "2026-...Z"(RFC 3339 UTC,決策 #21)。
// *string 為 nil 時 JSON 自動輸出 null。
func meJSON(u *repository.User) gin.H {
	return gin.H{
		"id":           u.ID,
		"username":     u.Username,
		"display_name": u.DisplayName,
		"bio":          u.Bio,
		"avatar_url":   u.AvatarURL,
		"email":        u.Email,
		"created_at":   u.CreatedAt.UTC(),
		"updated_at":   u.UpdatedAt.UTC(),
	}
}
