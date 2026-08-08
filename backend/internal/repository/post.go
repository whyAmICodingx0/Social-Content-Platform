package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSlugTaken：同作者下 slug 已存在（23505 on posts_author_slug_key）。
// service 捕獲後換候選重試整個 transaction（spec 7.6）。
var ErrSlugTaken = errors.New("repository: slug taken")

type Post struct {
	ID          string
	AuthorID    string
	Title       string
	Slug        string
	ContentMD   string
	Excerpt     *string
	Status      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// 以下由 JOIN 帶出，非 posts 表欄位
	AuthorUsername    string
	AuthorDisplayName *string
	AuthorAvatarURL   *string
	Tags              []string
}

type PostRepository struct {
	pool *pgxpool.Pool
}

func NewPostRepository(pool *pgxpool.Pool) *PostRepository {
	return &PostRepository{pool: pool}
}

type CreatePostParams struct {
	AuthorID    string
	Title       string
	Slug        string
	ContentMD   string
	Excerpt     string
	Status      string
	PublishedAt *time.Time
	Tags        []string
}

// Create 建立文章 + 標籤，全部在同一個 transaction（決策 #28）。
// slug 衝突回 ErrSlugTaken，由 service 換候選後重試**整個** transaction
// —— 交易 abort 後不能在原交易內重來（spec 7.6 第 3 步）。
func (r *PostRepository) Create(ctx context.Context, p CreatePostParams) (*Post, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const insertPost = `
		INSERT INTO posts (author_id, title, slug, content_md, excerpt, status, published_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7)
		RETURNING id, author_id, title, slug, content_md, excerpt, status,
		          published_at, created_at, updated_at`

	var post Post
	err = tx.QueryRow(ctx, insertPost,
		p.AuthorID, p.Title, p.Slug, p.ContentMD, p.Excerpt, p.Status, p.PublishedAt,
	).Scan(
		&post.ID, &post.AuthorID, &post.Title, &post.Slug, &post.ContentMD,
		&post.Excerpt, &post.Status, &post.PublishedAt, &post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		return nil, mapPostViolation(err)
	}

	if err := replaceTags(ctx, tx, post.ID, p.Tags); err != nil {
		return nil, err
	}
	post.Tags = p.Tags

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &post, nil
}

type UpdatePostParams struct {
	Title       *string
	ContentMD   *string
	Excerpt     *string
	Status      *string
	PublishedAt *time.Time // 僅在首次發布時帶值
	Tags        []string   // nil = 不動；空 slice = 清空
	ReplaceTags bool
}

// Update 更新文章 + （可選）重建標籤，同一 transaction。
// 呼叫端必須先確認作者身分（權限判斷屬 service 職責）。
func (r *PostRepository) Update(ctx context.Context, postID string, p UpdatePostParams) (*Post, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// COALESCE 部分更新（同任務 G 的技巧）。
	// published_at 特別處理：只在原本為 NULL 時填入，語意為「首次發布時間」
	// （決策 #7：改回 draft 不清除、再次發布不更新）。
	// updated_at 由 Go 帶 now()（決策 #10）。
	const q = `
		UPDATE posts SET
			title        = COALESCE($2, title),
			content_md   = COALESCE($3, content_md),
			excerpt      = CASE WHEN $4::text IS NULL THEN excerpt ELSE NULLIF($4, '') END,
			status       = COALESCE($5, status),
			published_at = CASE
			                 WHEN published_at IS NOT NULL THEN published_at
			                 ELSE $6
			               END,
			updated_at   = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, author_id, title, slug, content_md, excerpt, status,
		          published_at, created_at, updated_at`

	var post Post
	err = tx.QueryRow(ctx, q,
		postID, p.Title, p.ContentMD, p.Excerpt, p.Status, p.PublishedAt,
	).Scan(
		&post.ID, &post.AuthorID, &post.Title, &post.Slug, &post.ContentMD,
		&post.Excerpt, &post.Status, &post.PublishedAt, &post.CreatedAt, &post.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, mapPostViolation(err)
	}

	if p.ReplaceTags {
		if err := replaceTags(ctx, tx, post.ID, p.Tags); err != nil {
			return nil, err
		}
		post.Tags = p.Tags
	} else {
		tags, err := loadTags(ctx, tx, post.ID)
		if err != nil {
			return nil, err
		}
		post.Tags = tags
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &post, nil
}

// SoftDelete 設 deleted_at（決策 #8）。找不到未刪的目標 → ErrNotFound。
func (r *PostRepository) SoftDelete(ctx context.Context, postID string) error {
	const q = `
		UPDATE posts SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, postID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID 供權限檢查用（只取必要欄位）。
func (r *PostRepository) GetByID(ctx context.Context, postID string) (*Post, error) {
	const q = `
		SELECT id, author_id, title, slug, content_md, excerpt, status,
		       published_at, created_at, updated_at
		FROM posts
		WHERE id = $1 AND deleted_at IS NULL`

	var p Post
	err := r.pool.QueryRow(ctx, q, postID).Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Slug, &p.ContentMD, &p.Excerpt,
		&p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByAuthorAndSlug 讀單篇（spec 7.4），一併帶出作者資訊與標籤。
// users 也要 deleted_at IS NULL：作者被軟刪 → 文章視為不存在。
func (r *PostRepository) GetByAuthorAndSlug(ctx context.Context, username, slug string) (*Post, error) {
	const q = `
		SELECT p.id, p.author_id, p.title, p.slug, p.content_md, p.excerpt,
		       p.status, p.published_at, p.created_at, p.updated_at,
		       u.username, u.display_name, u.avatar_url
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE lower(u.username) = lower($1)
		  AND p.slug = $2
		  AND p.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var p Post
	err := r.pool.QueryRow(ctx, q, username, slug).Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Slug, &p.ContentMD, &p.Excerpt,
		&p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.AuthorUsername, &p.AuthorDisplayName, &p.AuthorAvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	tags, err := loadTagsPool(ctx, r.pool, p.ID)
	if err != nil {
		return nil, err
	}
	p.Tags = tags
	return &p, nil
}

// ---------- 內部工具 ----------

// replaceTags：整組取代語意（spec 7.5）——先清空既有關聯，再 upsert 並重建。
// tags upsert 用 ON CONFLICT DO UPDATE ... RETURNING（決策 #28）：
// 【事實】DO NOTHING 在衝突時**不回傳任何列**，拿不到既有 tag 的 id，
// 所以必須用 DO UPDATE（即使更新的是同樣的值）才能 RETURNING id。
func replaceTags(ctx context.Context, tx pgx.Tx, postID string, tags []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, postID); err != nil {
		return err
	}
	for _, t := range tags {
		var tagID string
		const upsert = `
			INSERT INTO tags (name, slug) VALUES ($1, $1)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`
		if err := tx.QueryRow(ctx, upsert, t).Scan(&tagID); err != nil {
			return err
		}
		const link = `INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2)`
		if _, err := tx.Exec(ctx, link, postID, tagID); err != nil {
			return err
		}
	}
	return nil
}

const selectTags = `
	SELECT t.slug FROM tags t
	JOIN post_tags pt ON pt.tag_id = t.id
	WHERE pt.post_id = $1
	ORDER BY t.slug ASC`

func loadTags(ctx context.Context, tx pgx.Tx, postID string) ([]string, error) {
	rows, err := tx.Query(ctx, selectTags, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTags(rows)
}

func loadTagsPool(ctx context.Context, pool *pgxpool.Pool, postID string) ([]string, error) {
	rows, err := pool.Query(ctx, selectTags, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTags(rows)
}

func scanTags(rows pgx.Rows) ([]string, error) {
	tags := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		tags = append(tags, s)
	}
	return tags, rows.Err()
}

func mapPostViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == "posts_author_slug_key" {
		return ErrSlugTaken
	}
	return err
}
