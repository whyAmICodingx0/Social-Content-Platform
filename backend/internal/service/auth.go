package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/googleoauth"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

var (
	// ErrNoAccount:這個 Google 身分還沒有帳號 → handler 走 pending signup 分支。
	ErrNoAccount = errors.New("service: no linked account")
	// ErrAccountUnavailable:oauth 綁定存在但 user 已被軟刪(spec 5.2 的資料異常)。
	ErrAccountUnavailable = errors.New("service: account unavailable")
)

// ValidationError:帶欄位資訊的驗證錯誤,
// handler 用 errors.As 取出後放進 details.fields(spec 3.1)。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s %s", e.Field, e.Message)
}

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

type AuthService struct {
	Auth  *repository.AuthRepository
	Users *repository.UserRepository
}

// LoginWithGoogle:spec 5.2 第 3 步的老用戶路徑。
func (s *AuthService) LoginWithGoogle(ctx context.Context, g *googleoauth.User) (*repository.User, error) {
	userID, err := s.Auth.FindUserIDByOAuth(ctx, "google", g.Sub)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrNoAccount
	}
	if err != nil {
		return nil, err
	}

	u, err := s.Users.GetByID(ctx, userID) // 內含 deleted_at IS NULL
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrAccountUnavailable
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// Signup:修訂版 spec 5.3。回傳 (user, isExisting, error):
// isExisting=true 代表走了「視同已註冊」收斂路徑(handler 回 200 而非 201)。
func (s *AuthService) Signup(ctx context.Context, p *store.PendingSignup, rawUsername, rawDisplayName string) (*repository.User, bool, error) {
	// 步驟 2:先驗證與正規化——格式錯誤一律 400,不受註冊狀態影響(錯誤優先級一致性)。
	username, err := normalizeUsername(rawUsername)
	if err != nil {
		return nil, false, err
	}
	displayName, err := resolveDisplayName(rawDisplayName, p.Name, username)
	if err != nil {
		return nil, false, err
	}

	// 步驟 3:建立前先查綁定(快路徑,處理絕大多數重複提交)。
	if u, found, err := s.findRegistered(ctx, p); err != nil {
		return nil, false, err
	} else if found {
		return u, true, nil
	}

	// 步驟 4:transaction 建 users + oauth_accounts。
	u, err := s.Auth.CreateUserWithOAuth(ctx, repository.CreateUserWithOAuthParams{
		Username:       username,
		Email:          p.Email,
		DisplayName:    displayName,
		AvatarURL:      p.Picture,
		Provider:       p.Provider,
		ProviderUserID: p.ProviderUserID,
		ProviderEmail:  p.Email,
	})
	if err == nil {
		return u, false, nil
	}

	// 修訂核心:任何 unique violation(rollback 已由 repository 的 defer 完成)
	// → 在交易外重查綁定,以 (provider, provider_user_id) 為錨點收斂。
	if errors.Is(err, repository.ErrUsernameTaken) ||
		errors.Is(err, repository.ErrEmailTaken) ||
		errors.Is(err, repository.ErrOAuthAccountExists) {

		if existing, found, qerr := s.findRegistered(ctx, p); qerr != nil {
			return nil, false, qerr
		} else if found {
			// 並行請求(或先前請求)已完成註冊 → 視同已註冊。
			return existing, true, nil
		}
		// 綁定仍不存在 → 這是「真的」撞名 / 撞 email,以原始 constraint 決定回應。
		return nil, false, err
	}

	// 含 ErrUnexpectedConflict 與其他錯誤,原樣上拋。
	return nil, false, err
}

// findRegistered:以 (provider, provider_user_id) 為錨點判斷此 Google 身分是否已註冊。
// 回傳 (user, found, error);「綁定存在但 user 已軟刪」依修訂 5.3 視為資料異常。
func (s *AuthService) findRegistered(ctx context.Context, p *store.PendingSignup) (*repository.User, bool, error) {
	userID, err := s.Auth.FindUserIDByOAuth(ctx, p.Provider, p.ProviderUserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	u, err := s.Users.GetByID(ctx, userID) // 內含 deleted_at IS NULL
	if errors.Is(err, repository.ErrNotFound) {
		return nil, false, ErrAccountUnavailable // 綁定在、人已刪 → 異常
	}
	if err != nil {
		return nil, false, err
	}
	return u, true, nil
}

// normalizeUsername:決策 #29 的正規化管線,順序即合約:
// trim → NFKC → lower → regex → 保留字 →(DB unique 是最終仲裁,不在這裡)
func normalizeUsername(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	u = norm.NFKC.String(u) // 全形→半形等;讓「ａｌｉｃｅ」被正確理解而非直接打回
	u = strings.ToLower(u)

	if !usernameRe.MatchString(u) {
		return "", &ValidationError{
			Field:   "username",
			Message: "must be 3-30 chars, lowercase letters, numbers or underscore",
		}
	}
	if config.IsReservedUsername(u) {
		return "", &ValidationError{Field: "username", Message: "this username is reserved"}
	}
	return u, nil
}

// resolveDisplayName:使用者給的 → Google 的 name → username,取第一個非空者。
func resolveDisplayName(raw, googleName, fallback string) (string, error) {
	d := strings.TrimSpace(raw)
	if d == "" {
		d = strings.TrimSpace(googleName)
	}
	if d == "" {
		d = fallback
	}
	if utf8.RuneCountInString(d) > 50 {
		return "", &ValidationError{Field: "display_name", Message: "must be 1-50 characters"}
	}
	return d, nil
}
