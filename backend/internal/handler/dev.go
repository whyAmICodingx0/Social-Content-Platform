package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

type DevHandler struct {
	Users    *repository.UserRepository
	Sessions *store.SessionStore
	States   *store.OAuthStateStore
	Cookies  *cookies.Manager
}

type echoRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// Echo:驗證 BindStrict 的行為。
func (h *DevHandler) Echo(c *gin.Context) {
	var req echoRequest
	if !api.BindStrict(c, &req) {
		return
	}
	api.OK(c, req)
}

type devLoginRequest struct {
	UserID string `json:"user_id"`
}

// Login:模擬「登入成功後建 session + 設 cookie」——
// 這正是任務 F 的 callback / signup 最後兩步會做的事。
// 任務 F 完成後刪除。
func (h *DevHandler) Login(c *gin.Context) {
	var req devLoginRequest
	if !api.BindStrict(c, &req) {
		return
	}
	u, err := h.Users.GetByID(c.Request.Context(), req.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		api.Fail(c, http.StatusInternalServerError, api.CodeInternalError, "unexpected error")
		return
	}
	sid, err := h.Sessions.Create(c.Request.Context(), u.ID)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	h.Cookies.SetSession(c, sid)
	api.OK(c, gin.H{"session_created_for": u.Username})
}

// StateDemo:演示 OAuth state 的一次性消費。
// 同一個 state 消費兩次:第一次 true、第二次必須 false。
func (h *DevHandler) StateDemo(c *gin.Context) {
	ctx := c.Request.Context()
	state, err := h.States.Create(ctx)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	first, err := h.States.Consume(ctx, state)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	second, err := h.States.Consume(ctx, state)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}
	api.OK(c, gin.H{"first_consume": first, "second_consume": second})
}

// GetUser:驗證 repository 的 deleted_at 封裝。
// 注意:id 亂打非 UUID 會回 500(dev 工具不做格式驗證;
// 正式端點在任務 H 會先驗格式)。
func (h *DevHandler) GetUser(c *gin.Context) {
	u, err := h.Users.GetByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, repository.ErrNotFound) {
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "user not found")
		return
	}
	if err != nil {
		api.Fail(c, http.StatusInternalServerError, api.CodeInternalError, "unexpected error")
		return
	}
	api.OK(c, gin.H{"id": u.ID, "username": u.Username, "email": u.Email})
}

// WhoAmI:驗證 optional auth(帶無效 sid 也不該 401)。
func (h *DevHandler) WhoAmI(c *gin.Context) {
	if u, ok := middleware.CurrentUser(c); ok {
		api.OK(c, gin.H{"authenticated": true, "username": u.Username})
		return
	}
	api.OK(c, gin.H{"authenticated": false})
}
