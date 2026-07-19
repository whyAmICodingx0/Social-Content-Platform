package store

import (
	"crypto/rand"
	"encoding/base64"
)

// NewToken 產生憑證 token(決策 #22 / spec 1.2):
// CSPRNG 32 bytes → base64url(無 padding),固定 43 字元。
// 用途:session ID、OAuth state、pending token。
// 注意這**不是** UUID——資源用 UUID、憑證用這個,兩者不得混用。
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
