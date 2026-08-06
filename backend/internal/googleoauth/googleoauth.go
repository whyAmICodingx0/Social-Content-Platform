package googleoauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// User 是我們從 Google 取回的最小欄位集（spec 5.2 第 2 步）。
type User struct {
	Sub           string `json:"sub"` // Google 的永久唯一 id —— 識別使用者的唯一依據
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type Client struct {
	cfg *oauth2.Config
}

func New(clientID, clientSecret, redirectURL string) *Client {
	return &Client{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}}
}

// AuthURL 組出 Google 授權頁網址（spec 5.1 第 4 步）。
// x/oauth2 會自動帶上 client_id、redirect_uri、scope、response_type=code、state。
func (c *Client) AuthURL(state string) string {
	return c.cfg.AuthCodeURL(state)
}

// FetchUser：用授權碼換 token，再呼叫 Google 的 OIDC userinfo 端點取得使用者資料。
//
// 【設計說明】另一條路是自己驗證 id_token（JWT）的簽章，可省一次 HTTP 呼叫，
// 但需要處理 JWKS 金鑰輪替，複雜不少。我們直接透過 HTTPS 向 Google 要 userinfo
// —— 資料直接來自 Google、走 TLS，一樣可信，是 MVP 的合理選擇（進階版本再換）。
func (c *Client) FetchUser(ctx context.Context, code string) (*User, error) {
	tok, err := c.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	httpClient := c.cfg.Client(ctx, tok) // 會自動帶 Authorization header
	resp, err := httpClient.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo status %d", resp.StatusCode)
	}

	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	if u.Sub == "" {
		return nil, fmt.Errorf("userinfo missing sub")
	}
	return &u, nil
}
