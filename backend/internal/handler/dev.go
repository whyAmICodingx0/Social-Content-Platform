package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

type DevHandler struct {
	Users *repository.UserRepository
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
