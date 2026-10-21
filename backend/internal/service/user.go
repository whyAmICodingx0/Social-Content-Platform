package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

const (
	maxDisplayNameLen = 50
	maxBioLen         = 500
	maxAvatarURLLen   = 500
)

type UserService struct {
	Users *repository.UserRepository
	Media *repository.MediaRepository // P5-2：驗證 avatar_media_id
}

// UpdateProfileInput：nil = 該欄位未出現在 request body（不更新）。
// JSON 的 null 也會變成 nil，所以 null 同樣等於「不更新」。
type UpdateProfileInput struct {
	DisplayName   *string
	Bio           *string
	AvatarURL     *string
	AvatarMediaID *string // P5-2（P5-0 §5.3）
}

// invalidAvatarMedia：不存在、不是本人的、purpose 不對、狀態不對——刻意回同一個錯誤，
// 不讓呼叫者藉由錯誤訊息探測別人的 media id 是否存在（同 ErrUploadNotFound 的理由）。
func invalidAvatarMedia() error {
	return &ValidationError{Field: "avatar_media_id", Message: "invalid avatar media"}
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*repository.User, error) {
	// 規則 4：同時給兩者 → 400。語意衝突，不猜使用者的意圖。
	// 放在最前面：這條不需要碰資料庫。
	if in.AvatarURL != nil && in.AvatarMediaID != nil {
		return nil, &ValidationError{
			Field: "avatar_media_id", Message: "cannot be combined with avatar_url",
		}
	}

	p := repository.UpdateProfileParams{}

	if in.DisplayName != nil {
		d := strings.TrimSpace(*in.DisplayName)
		if d != "" && utf8.RuneCountInString(d) > maxDisplayNameLen {
			return nil, &ValidationError{
				Field: "display_name", Message: "must be 1-50 characters",
			}
		}
		p.DisplayName = &d
	}

	if in.Bio != nil {
		b := strings.TrimSpace(*in.Bio)
		if utf8.RuneCountInString(b) > maxBioLen {
			return nil, &ValidationError{
				Field: "bio", Message: "must be at most 500 characters",
			}
		}
		p.Bio = &b
	}

	// 規則 2、3（外部網址 / 空字串）：avatar_media_id 怎麼變由 repository 的 SQL 決定（決策 #147）
	if in.AvatarURL != nil {
		a := strings.TrimSpace(*in.AvatarURL)
		if utf8.RuneCountInString(a) > maxAvatarURLLen {
			return nil, &ValidationError{
				Field: "avatar_url", Message: "must be at most 500 characters",
			}
		}
		p.AvatarURL = &a
	}

	// 規則 1：只給 avatar_media_id → 驗證後，avatar_url 跟著設成該 media 的 url。
	// 放在最後：前面的驗證都不需要資料庫，先擋掉便宜的錯誤。
	if in.AvatarMediaID != nil {
		m, err := s.resolveAvatarMedia(ctx, userID, *in.AvatarMediaID)
		if err != nil {
			return nil, err
		}
		p.AvatarMediaID = &m.ID
		p.AvatarURL = &m.URL
	}

	return s.Users.UpdateProfile(ctx, userID, p)
}

// resolveAvatarMedia：格式檢查 → 查 DB → 政策檢查。
func (s *UserService) resolveAvatarMedia(ctx context.Context, userID, raw string) (*repository.Media, error) {
	// 先驗 UUID 格式：不合法的字串若直接進 SQL，PostgreSQL 會回 22P02 → 變成 503（見 GetByID 的註解）
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, invalidAvatarMedia()
	}

	m, err := s.Media.GetByID(ctx, id.String())
	if errors.Is(err, repository.ErrNotFound) {
		return nil, invalidAvatarMedia()
	}
	if err != nil {
		return nil, err // DB 錯誤 → handler default 分支 → log + 503
	}

	if err := checkAvatarMedia(m, userID); err != nil {
		return nil, err
	}
	return m, nil
}

// checkAvatarMedia：P5-0 §5.3 的三項政策（存在性已由 GetByID 確認）。
// 獨立成純函式，單元測試不需要資料庫。
func checkAvatarMedia(m *repository.Media, userID string) error {
	if m.OwnerID != userID || m.Purpose != "avatar" || m.Status != "ready" {
		return invalidAvatarMedia()
	}
	return nil
}

func (s *UserService) GetPublicProfile(ctx context.Context, username string) (*repository.User, error) {
	return s.Users.GetByUsername(ctx, username)
}

// SearchUsers 搜尋使用者（決策 #101）。
func (s *UserService) SearchUsers(
	ctx context.Context, terms *SearchTerms, limit, offset int,
) ([]*repository.SearchUser, int, error) {
	return s.Users.Search(ctx, terms.Pattern, terms.Query, limit, offset)
}
