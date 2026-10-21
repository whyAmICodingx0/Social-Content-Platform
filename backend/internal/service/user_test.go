package service

import (
	"context"
	"errors"
	"testing"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

const (
	avTestMe    = "11111111-1111-4111-8111-111111111111"
	avTestOther = "22222222-2222-4222-8222-222222222222"
	avTestMedia = "33333333-3333-4333-8333-333333333333"
)

func avTestStr(v string) *string { return &v }

// 每個失敗案例只違反「一條」政策，其餘條件都合法。
// 這樣某一條檢查被拿掉時，對應的案例一定會失敗（不會被其他條件順便擋下而假通過）。
func TestCheckAvatarMedia(t *testing.T) {
	valid := func() *repository.Media {
		return &repository.Media{OwnerID: avTestMe, Purpose: "avatar", Status: "ready"}
	}

	cases := []struct {
		name    string
		mutate  func(m *repository.Media)
		wantErr bool
	}{
		{"valid", func(m *repository.Media) {}, false},
		{"other owner", func(m *repository.Media) { m.OwnerID = avTestOther }, true},
		{"purpose post_image", func(m *repository.Media) { m.Purpose = "post_image" }, true},
		{"purpose post_video", func(m *repository.Media) { m.Purpose = "post_video" }, true},
		{"status failed", func(m *repository.Media) { m.Status = "failed" }, true},
		{"status pending", func(m *repository.Media) { m.Status = "pending" }, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := valid()
			tc.mutate(m)
			err := checkAvatarMedia(m, avTestMe)

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			var vErr *ValidationError
			if !errors.As(err, &vErr) || vErr.Field != "avatar_media_id" {
				t.Fatalf("want ValidationError on avatar_media_id, got %v", err)
			}
		})
	}
}

// 這些輸入必須在「碰資料庫之前」就被擋下。
// UserService 的 Users 與 Media 都是 nil：只要程式往下走到資料庫，就會 nil pointer panic，
// 測試直接失敗——所以「檢查順序錯了」或「漏了檢查」都不可能假通過。
func TestUpdateProfile_RejectedBeforeDB(t *testing.T) {
	s := &UserService{}

	cases := []struct {
		name string
		in   UpdateProfileInput
	}{
		{"both given (rule 4)", UpdateProfileInput{
			AvatarURL: avTestStr("https://example.com/a.png"), AvatarMediaID: avTestStr(avTestMedia),
		}},
		{"both given, empty url (rule 4)", UpdateProfileInput{
			AvatarURL: avTestStr(""), AvatarMediaID: avTestStr(avTestMedia),
		}},
		{"media id is not a uuid", UpdateProfileInput{AvatarMediaID: avTestStr("abc")}},
		{"media id is empty", UpdateProfileInput{AvatarMediaID: avTestStr("")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.UpdateProfile(context.Background(), avTestMe, tc.in)
			var vErr *ValidationError
			if !errors.As(err, &vErr) || vErr.Field != "avatar_media_id" {
				t.Fatalf("want ValidationError on avatar_media_id, got %v", err)
			}
		})
	}
}
