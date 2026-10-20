// Package cloudinary 是 Cloudinary 的最小客戶端：簽章、Admin API 反查、刪除。
//
// 刻意不用官方 Go SDK（決策 #146）：這裡只需要三個動作，
// 而簽章演算法本身是 P5 的學習重點 —— 自己寫約 15 行且完全透明。
package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase      = "https://api.cloudinary.com/v1_1"
	deliveryBase = "https://res.cloudinary.com"
)

// ErrNotFound：Admin API 回 404，資源不存在。
var ErrNotFound = errors.New("cloudinary: resource not found")

type Client struct {
	CloudName string
	APIKey    string
	apiSecret string // 小寫：不讓其他 package 有機會把它序列化出去
	http      *http.Client
}

func New(cloudName, apiKey, apiSecret string) *Client {
	return &Client{
		CloudName: cloudName,
		APIKey:    apiKey,
		apiSecret: apiSecret,
		// 決策 #127：Admin API 在回報端點的請求路徑上，必須有 timeout
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

// Configured 回報三個憑證是否齊全。
// 正式環境由 config 的護欄保證；本機若漏設，端點會回 503 而不是 panic。
func (c *Client) Configured() bool {
	return c.CloudName != "" && c.APIKey != "" && c.apiSecret != ""
}

// StringToSign 依 Cloudinary 規則組出待簽字串（不含 secret）：
// 參數依 key 的字母順序排序，以 k=v 形式、用 & 串接。
//
// ⚠️ file、cloud_name、resource_type、api_key 不參與簽章 —— 呼叫端不要傳入。
func StringToSign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}

// Sign 回傳 SHA-1 hex 簽章：sha1(StringToSign(params) + api_secret)。
func (c *Client) Sign(params map[string]string) string {
	return signWithSecret(params, c.apiSecret)
}

func signWithSecret(params map[string]string, secret string) string {
	sum := sha1.Sum([]byte(StringToSign(params) + secret))
	return hex.EncodeToString(sum[:])
}

// UploadURL 是瀏覽器直傳的目標網址。
func (c *Client) UploadURL(resourceType string) string {
	return fmt.Sprintf("%s/%s/%s/upload", apiBase, c.CloudName, resourceType)
}

// DeliveryURL 組出帶固定 transformation 的公開網址（決策 #119：前端不得自行組合）。
// ext 可為空字串（f_auto 時不需要副檔名）。
func (c *Client) DeliveryURL(resourceType, transformation, publicID, ext string) string {
	u := fmt.Sprintf("%s/%s/%s/upload/%s/%s",
		deliveryBase, c.CloudName, resourceType, transformation, publicID)
	if ext != "" {
		u += "." + ext
	}
	return u
}

// Resource 是 Admin API 回傳的資源資訊（只取我們需要的欄位）。
type Resource struct {
	AssetID      string `json:"asset_id"`
	PublicID     string `json:"public_id"`
	AssetFolder  string `json:"asset_folder"` // dynamic folder mode 才有（你的帳號已確認）
	ResourceType string `json:"resource_type"`
	Format       string `json:"format"`
	Bytes        int64  `json:"bytes"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	// ⚠️ 需確認：Admin API 的影片資源是否帶 duration（P5-4 實測）。
	//            缺值時 service 一律拒絕（fail closed，決策 #138）。
	Duration *float64 `json:"duration"`
}

// GetResource 以 public_id 反查資源（決策 #114）。
//
// 呼叫端必須先用正規表示式驗證 publicID 的格式 ——
// 不要讓任意字串進入 URL path。
func (c *Client) GetResource(ctx context.Context, resourceType, publicID string) (*Resource, error) {
	endpoint := fmt.Sprintf("%s/%s/resources/%s/upload/%s",
		apiBase, c.CloudName, resourceType, url.PathEscape(publicID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.APIKey, c.apiSecret)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: admin api: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("cloudinary: admin api status %d: %s", resp.StatusCode, body)
	}

	var r Resource
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("cloudinary: decode resource: %w", err)
	}
	return &r, nil
}

// Destroy 刪除資源（決策 #140）。走 Upload API，不佔 Admin API 的每小時額度。
// 「not found」視為成功 —— 目標狀態（不存在）已達成。
func (c *Client) Destroy(ctx context.Context, resourceType, publicID string) error {
	params := map[string]string{
		"invalidate": "true", // 一併清除 CDN 快取
		"public_id":  publicID,
		"timestamp":  strconv.FormatInt(time.Now().Unix(), 10),
	}

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("api_key", c.APIKey)
	form.Set("signature", c.Sign(params))

	endpoint := fmt.Sprintf("%s/%s/%s/destroy", apiBase, c.CloudName, resourceType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudinary: destroy: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("cloudinary: decode destroy (status %d): %w", resp.StatusCode, err)
	}
	if out.Result != "ok" && out.Result != "not found" {
		return fmt.Errorf("cloudinary: destroy result %q (status %d)", out.Result, resp.StatusCode)
	}
	return nil
}
