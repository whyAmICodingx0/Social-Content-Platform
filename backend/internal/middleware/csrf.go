package middleware

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
)

// CSRF 實作 spec 4.13 的第 2、3 層:
//
//	第 2 層:有 body 的請求,Content-Type 必須是 application/json → 否則 415
//	第 3 層:不安全方法若帶 Origin header,必須在白名單內 → 否則 403
//
// (第 1 層 SameSite=Lax 在 cookie 屬性上;第 4 層「GET 不變更狀態」是設計紀律。)
func CSRF(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(c *gin.Context) {
		// 安全方法(唯讀)直接放行
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		// 第 3 層:Origin 檢查。
		// 沒帶 Origin 的請求(curl / Postman 等非瀏覽器客戶端)放行:
		// CSRF 的威脅只存在於「瀏覽器自動帶 cookie」的情境。
		if origin := c.GetHeader("Origin"); origin != "" {
			if _, ok := allowed[origin]; !ok {
				api.Fail(c, http.StatusForbidden, api.CodeForbidden, "Origin not allowed")
				return
			}
		}

		// 第 2 層:Content-Type 檢查,只在真的有 body 時要求。
		// (所以 POST /auth/logout 這種無 body 的請求不受影響。)
		if c.Request.ContentLength != 0 {
			mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if err != nil || mediaType != "application/json" {
				api.Fail(c, http.StatusUnsupportedMediaType, api.CodeUnsupportedMediaType,
					"Content-Type must be application/json")
				return
			}
		}

		c.Next()
	}
}
