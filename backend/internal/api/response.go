package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 成功回應(spec 2.1 / 2.3)
func OK(c *gin.Context, data any)      { c.JSON(http.StatusOK, gin.H{"data": data}) }
func Created(c *gin.Context, data any) { c.JSON(http.StatusCreated, gin.H{"data": data}) }
func NoContent(c *gin.Context)         { c.Status(http.StatusNoContent) }

// 列表回應(spec 2.2)
type Pagination struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
}

func OKList(c *gin.Context, data any, p Pagination) {
	c.JSON(http.StatusOK, gin.H{"data": data, "pagination": p})
}

// 錯誤回應(spec 3.1)。
// 用 AbortWithStatusJSON 而不是 JSON:在 middleware 裡呼叫時,
// Abort 會阻止請求繼續往後面的 handler 走。
func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"code": code, "message": message},
	})
}

func FailWithFields(c *gin.Context, status int, code, message string, fields map[string]string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{"code": code, "message": message, "details": gin.H{"fields": fields}},
	})
}
