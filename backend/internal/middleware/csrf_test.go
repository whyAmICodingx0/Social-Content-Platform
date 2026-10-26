package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
)

// TestCSRF 驗證 spec 4.13 的第 2、3 層防護。
//
// 這些防護在正常使用時完全看不到:瀏覽器從我們自己的前端送出的請求,
// 永遠帶正確的 Origin 與 Content-Type,所以 403 / 415 那條路平常不會被走到。
// 哪天防護失效,網站照常運作、沒有人會發現——只能靠這裡的測試抓到。
func TestCSRF(t *testing.T) {
	const (
		ourSite   = "https://inkwell.example"
		devSite   = "http://localhost:5173"
		evilSite  = "https://evil.example"
		jsonType  = "application/json"
		jsonBody  = `{"title":"hi"}`
		plainType = "text/plain"
	)
	mw := CSRF([]string{ourSite, devSite})

	tests := []struct {
		name        string
		method      string
		origin      string // "" = 不帶 Origin header
		contentType string // "" = 不帶 Content-Type header
		body        string // "" = 沒有 body
		unknownLen  bool   // true = body 長度未知(chunked 傳輸,ContentLength = -1)
		wantStatus  int
		wantCode    string // 被擋下時預期的 error.code
	}{
		// ---- 安全方法(唯讀):一律放行,不檢查 Origin 與 Content-Type ----
		{name: "GET 來自其他網站也放行", method: http.MethodGet, origin: evilSite,
			contentType: plainType, body: "x", wantStatus: http.StatusOK},
		{name: "HEAD 來自其他網站也放行", method: http.MethodHead, origin: evilSite,
			wantStatus: http.StatusOK},
		{name: "OPTIONS 來自其他網站也放行", method: http.MethodOptions, origin: evilSite,
			wantStatus: http.StatusOK},

		// ---- 正常請求:放行 ----
		{name: "自己的前端送 JSON", method: http.MethodPost, origin: ourSite,
			contentType: jsonType, body: jsonBody, wantStatus: http.StatusOK},
		{name: "Content-Type 帶 charset 參數", method: http.MethodPost, origin: ourSite,
			contentType: "application/json; charset=utf-8", body: jsonBody, wantStatus: http.StatusOK},
		{name: "白名單的第二個來源(開發環境)", method: http.MethodPatch, origin: devSite,
			contentType: jsonType, body: jsonBody, wantStatus: http.StatusOK},
		{name: "沒帶 Origin(curl 等非瀏覽器客戶端)", method: http.MethodPost,
			contentType: jsonType, body: jsonBody, wantStatus: http.StatusOK},
		{name: "沒有 body 的 POST 不要求 Content-Type(例如登出)", method: http.MethodPost,
			origin: ourSite, wantStatus: http.StatusOK},
		{name: "沒有 body 的 DELETE", method: http.MethodDelete, origin: ourSite,
			wantStatus: http.StatusOK},

		// ---- 第 3 層:Origin 不在白名單 → 403 ----
		{name: "POST 來自其他網站", method: http.MethodPost, origin: evilSite,
			contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "PUT 來自其他網站", method: http.MethodPut, origin: evilSite,
			contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "PATCH 來自其他網站", method: http.MethodPatch, origin: evilSite,
			contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "沒有 body 的 DELETE 來自其他網站也要擋", method: http.MethodDelete, origin: evilSite,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "Origin: null(sandbox iframe、本機檔案會送出這個值)", method: http.MethodPost,
			origin: "null", contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "白名單網址後面接上攻擊者網域(只比對開頭會漏掉)", method: http.MethodPost,
			origin: ourSite + ".evil.example", contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},
		{name: "http 與 https 是不同來源", method: http.MethodPost,
			origin: "http://inkwell.example", contentType: jsonType, body: jsonBody,
			wantStatus: http.StatusForbidden, wantCode: api.CodeForbidden},

		// ---- 第 2 層:有 body 但不是 JSON → 415 ----
		// 前三種是 HTML <form> 不需要 JavaScript 就能跨站送出的格式,也就是 CSRF 攻擊最常用的。
		{name: "text/plain", method: http.MethodPost, origin: ourSite,
			contentType: plainType, body: jsonBody,
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
		{name: "表單格式 urlencoded", method: http.MethodPost, origin: ourSite,
			contentType: "application/x-www-form-urlencoded", body: "title=hi",
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
		{name: "表單格式 multipart", method: http.MethodPost, origin: ourSite,
			contentType: "multipart/form-data; boundary=x", body: "--x--",
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
		{name: "有 body 但沒帶 Content-Type", method: http.MethodPost, origin: ourSite,
			body:       jsonBody,
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
		{name: "body 長度未知(chunked)也要檢查", method: http.MethodPost, origin: ourSite,
			contentType: plainType, body: jsonBody, unknownLen: true,
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
		{name: "沒帶 Origin 也要檢查 Content-Type", method: http.MethodPost,
			contentType: plainType, body: jsonBody,
			wantStatus: http.StatusUnsupportedMediaType, wantCode: api.CodeUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, "/test", body)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.unknownLen {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}

			rec, reached := serve(mw, req, nil)

			if rec.Code != tt.wantStatus {
				t.Fatalf("狀態碼 = %d,預期 %d(body = %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			wantReached := tt.wantStatus == http.StatusOK
			if reached != wantReached {
				t.Fatalf("後面的 handler 是否被執行 = %v,預期 %v", reached, wantReached)
			}
			if tt.wantCode != "" {
				if got := errorCode(t, rec); got != tt.wantCode {
					t.Fatalf("error.code = %q,預期 %q", got, tt.wantCode)
				}
			}
		})
	}
}
