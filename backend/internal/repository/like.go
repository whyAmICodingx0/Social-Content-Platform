package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LikeState 是按讚操作後的結果，供 API 直接回傳（決策 #44：
// PUT / DELETE 都回 200 + 計數，前端不必再發一次請求）。
type LikeState struct {
	LikeCount int
	LikedByMe bool
}

type LikeRepository struct {
	pool *pgxpool.Pool
}

func NewLikeRepository(pool *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{pool: pool}
}

// FindLikeablePost 檢查文章是否可被按讚（決策 #43）。
// 條件：存在、未軟刪、作者未軟刪、且 status = 'published'。
//
// 草稿一律視為不存在——包括作者本人，所以這個查詢不需要 viewer 參數，
// 也少了一個分支。
func (r *LikeRepository) FindLikeablePost(ctx context.Context, postID string) error {
	const q = `
		SELECT 1
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.id = $1
		  AND p.status = 'published'
		  AND p.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var dummy int
	err := r.pool.QueryRow(ctx, q, postID).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Like 建立按讚。
// ON CONFLICT DO NOTHING：重複請求撞到複合主鍵時靜默略過，
// 這就是 PUT 冪等的實作基礎（你在 migration 驗收時親眼看過那個
// duplicate key 錯誤——這裡把它吃掉）。
func (r *LikeRepository) Like(ctx context.Context, userID, postID string) error {
	const q = `
		INSERT INTO post_likes (user_id, post_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, post_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, q, userID, postID)
	return err
}

// Unlike 移除按讚。沒按過時影響 0 列，不算錯誤（冪等）。
func (r *LikeRepository) Unlike(ctx context.Context, userID, postID string) error {
	const q = `DELETE FROM post_likes WHERE user_id = $1 AND post_id = $2`
	_, err := r.pool.Exec(ctx, q, userID, postID)
	return err
}

// GetState 取得某篇文章的讚數與「我是否按過」。
//
// 【決策 #56】讚數不排除已軟刪使用者的讚——MVP 簡化，
// 不為了一個數字多 JOIN 一次 users。
//
// 這裡的 userID 一定有值（兩個端點都是 required auth），
// 所以不需要處理 SQL NULL；那是 L-2 列表查詢才會遇到的問題。
func (r *LikeRepository) GetState(ctx context.Context, postID, userID string) (*LikeState, error) {
	const q = `
		SELECT
			(SELECT count(*) FROM post_likes WHERE post_id = $1),
			EXISTS (SELECT 1 FROM post_likes WHERE post_id = $1 AND user_id = $2)`

	var st LikeState
	if err := r.pool.QueryRow(ctx, q, postID, userID).Scan(&st.LikeCount, &st.LikedByMe); err != nil {
		return nil, err
	}
	return &st, nil
}