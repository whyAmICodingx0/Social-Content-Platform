package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

const (
	maxDisplayNameLen = 50
	maxBioLen         = 500
	maxAvatarURLLen   = 500
)

type UserService struct {
	Users *repository.UserRepository
}

// UpdateProfileInput：nil = 該欄位未出現在 request body（不更新）。
type UpdateProfileInput struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, in UpdateProfileInput) (*repository.User, error) {
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

	if in.AvatarURL != nil {
		a := strings.TrimSpace(*in.AvatarURL)
		if utf8.RuneCountInString(a) > maxAvatarURLLen {
			return nil, &ValidationError{
				Field: "avatar_url", Message: "must be at most 500 characters",
			}
		}
		p.AvatarURL = &a
	}

	return s.Users.UpdateProfile(ctx, userID, p)
}

func (s *UserService) GetPublicProfile(ctx context.Context, username string) (*repository.User, error) {
	return s.Users.GetByUsername(ctx, username)
}
