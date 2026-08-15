package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FollowState 是追蹤操作後的結果，供 API 直接回傳（決策 #44）。
type FollowState struct {
	FollowerCount int
	FollowedByMe  bool
}

// FollowCounts 是個人頁需要的兩個數字。
type FollowCounts struct {
	FollowerCount  int // 有幾個人追蹤我
	FollowingCount int // 我追蹤幾個人
}

// FollowUser 是關注列表的項目（選做端點用）。
type FollowUser struct {
	ID          string
	Username    string
	DisplayName *string
	AvatarURL   *string
	Bio         *string
	CreatedAt   time.Time
}

type FollowRepository struct {
	pool *pgxpool.Pool
}

func NewFollowRepository(pool *pgxpool.Pool) *FollowRepository {
	return &FollowRepository{pool: pool}
}

// Follow 建立追蹤關係。
// ON CONFLICT DO NOTHING 讓重複請求無害（決策 #44 冪等）。
//
// ⚠️ 注意欄位順序：follower 是「按追蹤的人」，followee 是「被追蹤的人」。
func (r *FollowRepository) Follow(ctx context.Context, followerID, followeeID string) error {
	const q = `
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_id, followee_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, q, followerID, followeeID)
	return err
}

// Unfollow 移除追蹤關係。沒追蹤過也不算錯（冪等）。
func (r *FollowRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	const q = `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	_, err := r.pool.Exec(ctx, q, followerID, followeeID)
	return err
}

// GetState 取得某人的粉絲數與「我是否追蹤他」。
// viewerID 為 nil 時（匿名），followed_by_me 一律為 false（決策 #48、#49）。
func (r *FollowRepository) GetState(ctx context.Context, targetID string, viewerID *string) (*FollowState, error) {
	const q = `
		SELECT
			(SELECT count(*) FROM follows WHERE followee_id = $1),
			EXISTS (
				SELECT 1 FROM follows
				WHERE followee_id = $1 AND follower_id = $2
			)`

	var st FollowState
	if err := r.pool.QueryRow(ctx, q, targetID, viewerID).
		Scan(&st.FollowerCount, &st.FollowedByMe); err != nil {
		return nil, err
	}
	return &st, nil
}

// GetCounts 取得粉絲數與追蹤中數量。
//
// 【決策 #52】兩者都排除已軟刪的使用者——
// 個人頁顯示「追蹤 5 人」但其中 2 個帳號已刪除會很怪。
// 這與讚數的處理不同（決策 #56 為省 JOIN 而不排除），
// 因為這裡的數字語意就是「幾個活著的使用者」。
func (r *FollowRepository) GetCounts(ctx context.Context, userID string) (*FollowCounts, error) {
	const q = `
		SELECT
			(SELECT count(*) FROM follows f
			 JOIN users u ON u.id = f.follower_id AND u.deleted_at IS NULL
			 WHERE f.followee_id = $1),
			(SELECT count(*) FROM follows f
			 JOIN users u ON u.id = f.followee_id AND u.deleted_at IS NULL
			 WHERE f.follower_id = $1)`

	var c FollowCounts
	if err := r.pool.QueryRow(ctx, q, userID).
		Scan(&c.FollowerCount, &c.FollowingCount); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListFollowers 列出追蹤某人的使用者（選做端點）。
// WHERE followee_id = target → 取 follower_id 對應的使用者。
func (r *FollowRepository) ListFollowers(ctx context.Context, userID string, limit, offset int) ([]*FollowUser, int, error) {
	const countSQL = `
		SELECT count(*) FROM follows f
		JOIN users u ON u.id = f.follower_id AND u.deleted_at IS NULL
		WHERE f.followee_id = $1`

	const listSQL = `
		SELECT u.id, u.username, u.display_name, u.avatar_url, u.bio, f.created_at
		FROM follows f
		JOIN users u ON u.id = f.follower_id AND u.deleted_at IS NULL
		WHERE f.followee_id = $1
		ORDER BY f.created_at DESC, u.id DESC
		LIMIT $2 OFFSET $3`

	return r.listFollowUsers(ctx, countSQL, listSQL, userID, limit, offset)
}

// ListFollowing 列出某人追蹤的使用者（選做端點）。
// WHERE follower_id = target → 取 followee_id 對應的使用者。
func (r *FollowRepository) ListFollowing(ctx context.Context, userID string, limit, offset int) ([]*FollowUser, int, error) {
	const countSQL = `
		SELECT count(*) FROM follows f
		JOIN users u ON u.id = f.followee_id AND u.deleted_at IS NULL
		WHERE f.follower_id = $1`

	const listSQL = `
		SELECT u.id, u.username, u.display_name, u.avatar_url, u.bio, f.created_at
		FROM follows f
		JOIN users u ON u.id = f.followee_id AND u.deleted_at IS NULL
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC, u.id DESC
		LIMIT $2 OFFSET $3`

	return r.listFollowUsers(ctx, countSQL, listSQL, userID, limit, offset)
}

func (r *FollowRepository) listFollowUsers(
	ctx context.Context, countSQL, listSQL, userID string, limit, offset int,
) ([]*FollowUser, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*FollowUser{}, 0, nil
	}

	rows, err := r.pool.Query(ctx, listSQL, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []*FollowUser{}
	for rows.Next() {
		var u FollowUser
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName,
			&u.AvatarURL, &u.Bio, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}
	return users, total, rows.Err()
}
