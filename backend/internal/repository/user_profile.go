package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// UpdateProfileParams：nil 代表「這個欄位不更新」（PATCH 語意）。
//
// P5-2：從 user.go 搬到這個檔案，並新增 AvatarMediaID。
type UpdateProfileParams struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string

	// AvatarMediaID 非 nil 時，service 保證 AvatarURL 同時被設成該 media 的 url
	// （P5-0 §5.3 規則 1）。repository 不重複檢查這件事。
	AvatarMediaID *string
}

// UpdateProfile 部分更新個人檔案。
//
// display_name / bio / avatar_url 的寫法與 Task G-1 相同：
//   - display_name：空字串或沒給 = 不變（不允許清空）
//   - bio / avatar_url：沒給 = 不變；空字串 = 清成 NULL
//
// avatar_media_id 的四種結果（P5-0 §5.3 + 決策 #147）：
//
//	$5 有值                       → 設為 $5（使用者剛上傳的頭像）
//	$4 沒給                       → 不變
//	$4 與「目前的」avatar_url 相同 → 不變（決策 #147：表單重送同一個網址不是換頭像）
//	$4 是不同的網址或空字串       → NULL（頭像不再由本站託管）
//
// ★ UPDATE 的 SET 子句裡，等號右邊讀到的欄位一律是「更新前」的值。
// 所以 CASE 裡的 avatar_url 是舊值——即使同一句 SQL 也正在改 avatar_url。
//
// ★ $n::text / $5::uuid 的明確轉型：同一個參數出現在多個位置時，
// 讓 PostgreSQL 對它推導出同一個型別，避免 "inconsistent types deduced for parameter"。
func (r *UserRepository) UpdateProfile(ctx context.Context, id string, p UpdateProfileParams) (*User, error) {
	const q = `
		UPDATE users SET
			display_name    = COALESCE(NULLIF($2::text, ''), display_name),
			bio             = NULLIF(COALESCE($3::text, bio, ''), ''),
			avatar_url      = NULLIF(COALESCE($4::text, avatar_url, ''), ''),
			avatar_media_id = CASE
				WHEN $5::uuid IS NOT NULL THEN $5::uuid
				WHEN $4::text IS NULL THEN avatar_media_id
				WHEN NULLIF($4::text, '') IS NOT DISTINCT FROM avatar_url THEN avatar_media_id
				ELSE NULL
			END,
			updated_at      = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, username, email, display_name, avatar_url, bio,
		          created_at, updated_at`

	var u User
	err := r.pool.QueryRow(ctx, q, id, p.DisplayName, p.Bio, p.AvatarURL, p.AvatarMediaID).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Bio,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound // 使用者不存在或已軟刪
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
