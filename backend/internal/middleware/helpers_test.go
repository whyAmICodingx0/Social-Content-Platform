package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// csrf_test.go 與 auth_test.go 共用的小工具。

func init() {
	// 關掉 Gin 的 debug 輸出,測試結果比較好讀
	gin.SetMode(gin.TestMode)
}

// serve 把一個請求送進「受測的 middleware → 終點 handler」這條最小路由。
//
// 回傳的 reached 表示終點 handler 有沒有被執行:
// middleware 擋下請求時必須是 false。這是「真的擋住了」的直接證據——
// 只看狀態碼不夠,因為 Gin 的 middleware 如果只寫回應、沒有 Abort,
// 後面的 handler 照樣會執行(也就是請求其實被放行了)。
//
// endpoint 是終點 handler 額外要做的事(可為 nil),例如讀取 CurrentUser。
func serve(mw gin.HandlerFunc, req *http.Request, endpoint func(c *gin.Context)) (rec *httptest.ResponseRecorder, reached bool) {
	r := gin.New()
	r.Handle(req.Method, "/test", mw, func(c *gin.Context) {
		reached = true
		if endpoint != nil {
			endpoint(c)
		}
		c.Status(http.StatusOK)
	})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec, reached
}

// errorCode 取出錯誤回應的 error.code(spec 3.1 的格式)。
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是 JSON:%v(body = %q)", err, rec.Body.String())
	}
	return body.Error.Code
}
