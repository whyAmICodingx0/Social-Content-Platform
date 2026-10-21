package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// 每篇文章的媒體上限（P5-0 §5.4 / 決策 #122）：總數 10（含影片），影片最多 1。
const (
	maxPostMedia  = 10
	maxPostVideos = 1
)

// 不存在、不是本人的、狀態不對、purpose 不對——刻意回同一個訊息，
// 不讓呼叫者藉由錯誤訊息探測別人的 media id 是否存在（同 invalidAvatarMedia 的理由）。
func invalidPostMedia() error {
	return &ValidationError{Field: "media_ids", Message: "contains media that cannot be attached"}
}

// normalizeMediaIDs：trim → UUID 格式 → 去重（保留第一次出現的位置）→ 總數上限。
//
// 回傳的陣列順序就是 post_media.position（決策 #133）。
// 上限在「去重之後」才檢查：P5-0 §5.4 規定重複的 id 去重後計算、不回錯。
func normalizeMediaIDs(raw []string) ([]string, error) {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))

	for _, r := range raw {
		id, err := uuid.Parse(strings.TrimSpace(r))
		if err != nil {
			return nil, &ValidationError{Field: "media_ids", Message: "contains an invalid id"}
		}
		s := id.String() // 統一成小寫、帶連字號的標準格式，去重才不會被大小寫騙過
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}

	if len(out) > maxPostMedia {
		return nil, &ValidationError{Field: "media_ids", Message: "at most 10 media per post"}
	}
	return out, nil
}

// checkPostMedia：P5-0 §5.4 的政策檢查。純函式，單元測試不需要資料庫。
//
// rows 是 GetByIDs 的結果；找不到的 id 不會出現在 rows 裡，所以用 map 逐一比對，
// 而不是只比較 len(rows) == len(ids)。
func checkPostMedia(ids []string, rows []*repository.Media, authorID string) error {
	byID := make(map[string]*repository.Media, len(rows))
	for _, m := range rows {
		byID[m.ID] = m
	}

	videos := 0
	for _, id := range ids {
		m, ok := byID[id]
		if !ok || m.OwnerID != authorID || m.Status != "ready" {
			return invalidPostMedia()
		}
		switch m.Purpose {
		case "post_image":
		case "post_video":
			videos++
		default: // avatar 不能放進文章
			return invalidPostMedia()
		}
	}

	if videos > maxPostVideos {
		return &ValidationError{Field: "media_ids", Message: "at most 1 video per post"}
	}
	return nil
}

// resolvePostMedia：normalize → 查 DB → 政策檢查。回傳可以直接寫入的 id 清單。
//
// 驗證在交易「之外」做（決策 #159）：P5 的 media 列建立後就不會再被修改，
// 唯一的刪除路徑是使用者被刪除時的 CASCADE，所以驗證與寫入之間不存在可利用的空窗。
// 萬一真的發生，post_media 的 FK 會讓寫入失敗，不會寫進壞資料。
func (s *PostService) resolvePostMedia(ctx context.Context, authorID string, raw []string) ([]string, error) {
	ids, err := normalizeMediaIDs(raw)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return ids, nil // 不需要查資料庫
	}

	rows, err := s.Media.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err // DB 錯誤 → handler default 分支 → log + 503
	}
	if err := checkPostMedia(ids, rows, authorID); err != nil {
		return nil, err
	}
	return ids, nil
}
