package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Comment struct {
	ID        string
	PostID    string
	AuthorID  string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time

	// 由 JOIN 帶出
	AuthorUsername    string
	AuthorDisplayName *string
	AuthorAvatarURL   *string
}

// CommentTarget 是留言所屬文章的必要資訊，供權限判斷使用。
type CommentTarget struct {
	PostID       string
	PostAuthorID string
}

type CommentRepository struct {
	pool *pgxpool.Pool
}

func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{pool: pool}
}

// FindCommentablePost 檢查文章是否可被留言（決策 #43）。
// 條件與按讚相同：存在、未軟刪、作者未軟刪、且已發布。
// 草稿一律視為不存在——包括作者本人，故不接收 viewer 參數。
// 回傳文章作者 id，供後續的刪除權限判斷使用。
func (r *CommentRepository) FindCommentablePost(ctx context.Context, postID string) (string, error) {
	const q = `
		SELECT p.author_id
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.id = $1
		  AND p.status = 'published'
		  AND p.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var authorID string
	err := r.pool.QueryRow(ctx, q, postID).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return authorID, nil
}

// List 取得某篇文章的留言（分頁）。
//
// 排序：created_at ASC, id ASC（留言正序閱讀較自然），
// 與 comments_post_idx 的索引方向一致。
//
// 【決策 #52】必須 JOIN users 並排除軟刪的留言者——
// 與讚數不同（那裡為了省一次 JOIN 而不排除），留言本來就要
// 顯示作者名字與頭像，順手就能排除。
func (r *CommentRepository) List(ctx context.Context, postID string, limit, offset int) ([]*Comment, int, error) {
	const countSQL = `
		SELECT count(*)
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.post_id = $1
		  AND c.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, postID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Comment{}, 0, nil
	}

	const listSQL = `
		SELECT c.id, c.post_id, c.author_id, c.content, c.created_at, c.updated_at,
		       u.username, u.display_name, u.avatar_url
		FROM comments c
		JOIN users u ON u.id = c.author_id
		WHERE c.post_id = $1
		  AND c.deleted_at IS NULL
		  AND u.deleted_at IS NULL
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, listSQL, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	comments := []*Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(
			&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
			&c.AuthorUsername, &c.AuthorDisplayName, &c.AuthorAvatarURL,
		); err != nil {
			return nil, 0, err
		}
		comments = append(comments, &c)
	}
	return comments, total, rows.Err()
}

// Create 新增留言，並回傳含作者資訊的完整結果。
// 用 CTE（WITH）一次完成 INSERT + JOIN users，省一次往返。
func (r *CommentRepository) Create(ctx context.Context, postID, authorID, content string) (*Comment, error) {
	const q = `
		WITH inserted AS (
			INSERT INTO comments (post_id, author_id, content)
			VALUES ($1, $2, $3)
			RETURNING id, post_id, author_id, content, created_at, updated_at
		)
		SELECT i.id, i.post_id, i.author_id, i.content, i.created_at, i.updated_at,
		       u.username, u.display_name, u.avatar_url
		FROM inserted i
		JOIN users u ON u.id = i.author_id`

	var c Comment
	err := r.pool.QueryRow(ctx, q, postID, authorID, content).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&c.AuthorUsername, &c.AuthorDisplayName, &c.AuthorAvatarURL,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetForPermission 取得留言與其所屬文章的作者，供權限判斷（決策 #54）。
// 回傳的 Comment 只含權限判斷需要的欄位。
func (r *CommentRepository) GetForPermission(ctx context.Context, commentID string) (*Comment, *CommentTarget, error) {
	const q = `
		SELECT c.id, c.post_id, c.author_id, c.content, c.created_at, c.updated_at,
		       p.author_id
		FROM comments c
		JOIN posts p ON p.id = c.post_id
		WHERE c.id = $1
		  AND c.deleted_at IS NULL
		  AND p.deleted_at IS NULL`

	var c Comment
	var t CommentTarget
	err := r.pool.QueryRow(ctx, q, commentID).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&t.PostAuthorID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	t.PostID = c.PostID
	return &c, &t, nil
}

// Update 更新留言內容。updated_at 由 Go 帶 now()（決策 #10）。
func (r *CommentRepository) Update(ctx context.Context, commentID, content string) (*Comment, error) {
	const q = `
		WITH updated AS (
			UPDATE comments
			SET content = $2, updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
			RETURNING id, post_id, author_id, content, created_at, updated_at
		)
		SELECT up.id, up.post_id, up.author_id, up.content, up.created_at, up.updated_at,
		       u.username, u.display_name, u.avatar_url
		FROM updated up
		JOIN users u ON u.id = up.author_id`

	var c Comment
	err := r.pool.QueryRow(ctx, q, commentID, content).Scan(
		&c.ID, &c.PostID, &c.AuthorID, &c.Content, &c.CreatedAt, &c.UpdatedAt,
		&c.AuthorUsername, &c.AuthorDisplayName, &c.AuthorAvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// SoftDelete 設 deleted_at（決策 #8、#55：不記錄 deleted_by）。
func (r *CommentRepository) SoftDelete(ctx context.Context, commentID string) error {
	const q = `
		UPDATE comments SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, commentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
