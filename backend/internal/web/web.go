package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// dist 目錄由 Docker 建置流程從 frontend/dist 複製進來（見 Dockerfile）。
// all: 前綴確保以底線或點開頭的檔案也被嵌入（Vite 產物可能有這類檔名）。
//
//go:embed all:dist
var distFS embed.FS

// Available 回報是否有真正的前端資源可提供。
// 本機開發時 dist/ 只有佔位檔，此時回 false，前端請走 Vite dev server（:5173）。
func Available() bool {
	entries, err := fs.ReadDir(distFS, "dist")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == "index.html" {
			return true
		}
	}
	return false
}

// Register 掛上靜態檔服務與 SPA fallback。
//
// 【重點：SPA history fallback】
// Vue Router 使用 HTML5 history 模式，網址如 /@alice/my-post 在前端是有效路由，
// 但伺服器上沒有這個檔案。若直接回 404，使用者一重新整理文章頁就壞掉。
// 因此：找不到對應檔案時一律回傳 index.html，交由前端路由決定顯示什麼。
func Register(r *gin.Engine) error {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path

		// API 路徑不做 fallback：不存在的端點應回 JSON 404，
		// 而不是回一份 HTML（否則前端的錯誤處理會解析失敗）。
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{"code": "NOT_FOUND", "message": "endpoint not found"},
			})
			return
		}

		// SPA fallback 只適用於「瀏覽器要看一個頁面」的請求。
		// 非 GET/HEAD（例如 POST 到不存在的路徑）回一份 HTML 是錯的語意，
		// 應明確回 405 Method Not Allowed。
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"error": gin.H{"code": "NOT_FOUND", "message": "method not allowed"},
			})
			return
		}

		// 檔案存在 → 直接提供（JS、CSS、favicon 等）
		if fileExists(sub, p) {
			// 帶 hash 的建置產物可長期快取；其餘交給預設行為
			if strings.HasPrefix(p, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// 檔案不存在 → 回 index.html（SPA fallback）
		// index.html 本身不可快取，否則使用者會拿到舊版前端
		c.Header("Cache-Control", "no-cache")
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

func fileExists(fsys fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(urlPath, "/")
	if name == "" {
		return false
	}
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	return err == nil && !info.IsDir()
}
