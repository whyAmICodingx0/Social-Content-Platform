package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// 測試用的 helper 與常數加 pmTest 前綴，避免和同 package 其他測試檔撞名。
const (
	pmTestAuthor = "11111111-1111-4111-8111-111111111111"
	pmTestOther  = "22222222-2222-4222-8222-222222222222"
)

// pmTestID 產生第 n 個合法 UUID（n 從 1 開始），方便建立多筆
func pmTestID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func pmTestFieldErr(t *testing.T, err error, wantMsgPart string) {
	t.Helper()
	var vErr *ValidationError
	if !errors.As(err, &vErr) || vErr.Field != "media_ids" {
		t.Fatalf("want ValidationError on media_ids, got %v", err)
	}
	if !strings.Contains(vErr.Message, wantMsgPart) {
		t.Fatalf("want message containing %q, got %q", wantMsgPart, vErr.Message)
	}
}

func TestNormalizeMediaIDs(t *testing.T) {
	t.Run("nil gives empty slice", func(t *testing.T) {
		got, err := normalizeMediaIDs(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	t.Run("dedupe keeps first position, case-insensitive", func(t *testing.T) {
		a, b, c := pmTestID(1), pmTestID(2), pmTestID(3)
		got, err := normalizeMediaIDs([]string{b, a, " " + strings.ToUpper(b) + " ", c, a})
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{b, a, c}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		_, err := normalizeMediaIDs([]string{pmTestID(1), "abc"})
		pmTestFieldErr(t, err, "invalid id")
	})

	// 15 筆輸入、去重後剛好 10 筆 → 必須通過。
	// 若上限在去重「之前」檢查，這條會失敗——所以它能抓到檢查順序錯誤。
	t.Run("limit is checked after dedupe", func(t *testing.T) {
		raw := []string{}
		for i := 1; i <= 10; i++ {
			raw = append(raw, pmTestID(i))
		}
		raw = append(raw, pmTestID(1), pmTestID(2), pmTestID(3), pmTestID(4), pmTestID(5))
		got, err := normalizeMediaIDs(raw)
		if err != nil || len(got) != 10 {
			t.Fatalf("got %d ids, err %v", len(got), err)
		}
	})

	t.Run("11 distinct ids rejected", func(t *testing.T) {
		raw := []string{}
		for i := 1; i <= 11; i++ {
			raw = append(raw, pmTestID(i))
		}
		_, err := normalizeMediaIDs(raw)
		pmTestFieldErr(t, err, "at most 10")
	})
}

// 每個失敗案例只違反「一條」規則（同 TestCheckAvatarMedia 的設計），
// 拿掉任何一條檢查，對應的案例就會失敗。
func TestCheckPostMedia(t *testing.T) {
	media := func(n int, purpose string) *repository.Media {
		return &repository.Media{ID: pmTestID(n), OwnerID: pmTestAuthor, Purpose: purpose, Status: "ready"}
	}

	t.Run("9 images + 1 video ok", func(t *testing.T) {
		ids, rows := []string{}, []*repository.Media{}
		for i := 1; i <= 9; i++ {
			ids = append(ids, pmTestID(i))
			rows = append(rows, media(i, "post_image"))
		}
		ids = append(ids, pmTestID(10))
		rows = append(rows, media(10, "post_video"))
		if err := checkPostMedia(ids, rows, pmTestAuthor); err != nil {
			t.Fatalf("want nil, got %v", err)
		}
	})

	cases := []struct {
		name    string
		ids     []string
		rows    []*repository.Media
		wantMsg string
	}{
		{
			name:    "id not found",
			ids:     []string{pmTestID(1), pmTestID(2)},
			rows:    []*repository.Media{media(1, "post_image")},
			wantMsg: "cannot be attached",
		},
		{
			name:    "other owner",
			ids:     []string{pmTestID(1)},
			rows:    []*repository.Media{{ID: pmTestID(1), OwnerID: pmTestOther, Purpose: "post_image", Status: "ready"}},
			wantMsg: "cannot be attached",
		},
		{
			name:    "status failed",
			ids:     []string{pmTestID(1)},
			rows:    []*repository.Media{{ID: pmTestID(1), OwnerID: pmTestAuthor, Purpose: "post_image", Status: "failed"}},
			wantMsg: "cannot be attached",
		},
		{
			name:    "purpose avatar",
			ids:     []string{pmTestID(1)},
			rows:    []*repository.Media{media(1, "avatar")},
			wantMsg: "cannot be attached",
		},
		{
			name:    "two videos",
			ids:     []string{pmTestID(1), pmTestID(2)},
			rows:    []*repository.Media{media(1, "post_video"), media(2, "post_video")},
			wantMsg: "at most 1 video",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pmTestFieldErr(t, checkPostMedia(tc.ids, tc.rows, pmTestAuthor), tc.wantMsg)
		})
	}
}

// 這些輸入必須在「碰資料庫之前」就被擋下（Media repository 是 nil，碰到就 panic）
func TestResolvePostMedia_RejectedBeforeDB(t *testing.T) {
	s := &PostService{}

	if ids, err := s.resolvePostMedia(context.Background(), pmTestAuthor, nil); err != nil || len(ids) != 0 {
		t.Fatalf("empty input must not touch DB: %v %v", ids, err)
	}
	_, err := s.resolvePostMedia(context.Background(), pmTestAuthor, []string{"not-a-uuid"})
	pmTestFieldErr(t, err, "invalid id")
}

func TestMakeExcerpt(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{
			name: "image at start is removed",
			in:   "![my photo](https://res.cloudinary.com/x/image/upload/c_limit,w_1600,f_auto,q_auto/pi_abc)\n\nHello world",
			want: "Hello world",
		},
		{
			name: "image in the middle is removed",
			in:   "Before ![](https://example.com/a.jpg) after",
			want: "Before after",
		},
		{
			name: "link keeps its text",
			in:   "Read [the docs](https://example.com/docs) first",
			want: "Read the docs first",
		},
		{
			name: "existing behavior unchanged",
			in:   "# Title\n\nSome **bold** and `code`",
			want: "Title Some bold and code",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := makeExcerpt(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
