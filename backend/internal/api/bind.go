package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BindStrict:嚴格 JSON 綁定(spec 1.3 / 決策 #24)。
// 成功回 true;失敗回 false(此時 400 回應已經寫好,handler 直接 return 即可)。
//
// 用法:
//
//	var req myRequest
//	if !api.BindStrict(c, &req) { return }
func BindStrict(c *gin.Context, dst any) bool {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields() // 核心:出現 struct 沒定義的欄位就報錯

	if err := dec.Decode(dst); err != nil {
		FailWithFields(c, http.StatusBadRequest, CodeValidationError,
			"Request validation failed",
			map[string]string{"body": bindErrMessage(err)})
		return false
	}
	// body 後面不得再有第二段 JSON 或垃圾內容
	if dec.More() {
		Fail(c, http.StatusBadRequest, CodeValidationError,
			"Request body must contain a single JSON object")
		return false
	}
	return true
}

func bindErrMessage(err error) string {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case errors.Is(err, io.EOF):
		return "request body is empty"
	case errors.As(err, &syntaxErr):
		return fmt.Sprintf("malformed JSON at position %d", syntaxErr.Offset)
	case errors.As(err, &typeErr):
		return fmt.Sprintf("field %q has wrong type", typeErr.Field)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// 【事實】標準庫沒有為 unknown field 提供 typed error,
		// 用字串前綴判斷是目前公認的做法(醜但標準)。
		return fmt.Sprintf("unknown field %s",
			strings.TrimPrefix(err.Error(), "json: unknown field "))
	default:
		return "invalid JSON body"
	}
}
