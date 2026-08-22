package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSlugTaken：同作者下 slug 已存在（23505 on posts_author_slug_key）。
// service 捕獲後換候選重試整個 transaction（spec 7.6）。
var ErrSlugTaken = errors.New("repository: slug taken")

// 計數聚合的共用 SQL 片段（決策 #50）。
//
// ⚠️ 必須用 LEFT JOIN LATERAL 各自聚合，不可同時 JOIN post_likes 與 comments：
//
//	那會產生笛卡爾乘積（3 個讚 × 4 則留言 = 12 列），兩邊的 count 都會變成 12。
//	COUNT(DISTINCT) 雖能算對，但那 12 列仍被實體化，只是把錯的算對。
//
// $likeParam 是 viewer 的 user id，未登入時必須是真正的 SQL NULL
// （決策 #49：不可用 uuid.Nil —— 那是合法 UUID，結果碰巧正確但語意錯誤）。
const countsJoin = `
	LEFT JOIN LATERAL (
		SELECT count(*) AS cnt FROM post_likes pl WHERE pl.post_id = p.id
	) lc ON true
	LEFT JOIN LATERAL (
		SELECT count(*) AS cnt FROM comments c
		WHERE c.post_id = p.id AND c.deleted_at IS NULL
	) cc ON true`

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

	// 以下由聚合查詢帶出（Phase 2）
	LikeCount    int
	CommentCount int
	LikedByMe    bool
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

// GetByAuthorAndSlug 讀單篇（spec 7.4），一併帶出作者資訊、標籤與計數。
// viewerID 為 nil 代表匿名 —— pgx 會送出真正的 SQL NULL（決策 #49）。
func (r *PostRepository) GetByAuthorAndSlug(ctx context.Context, username, slug string, viewerID *string) (*Post, error) {
	q := `
		SELECT p.id, p.author_id, p.title, p.slug, p.content_md, p.excerpt,
		       p.status, p.published_at, p.created_at, p.updated_at,
		       u.username, u.display_name, u.avatar_url,
		       COALESCE(lc.cnt, 0), COALESCE(cc.cnt, 0),
		       EXISTS (
		           SELECT 1 FROM post_likes
		           WHERE post_id = p.id AND user_id = $3
		       )
		FROM posts p
		JOIN users u ON u.id = p.author_id` + countsJoin + `
		WHERE lower(u.username) = lower($1)
		  AND p.slug = $2
		  AND p.deleted_at IS NULL
		  AND u.deleted_at IS NULL`

	var p Post
	err := r.pool.QueryRow(ctx, q, username, slug, viewerID).Scan(
		&p.ID, &p.AuthorID, &p.Title, &p.Slug, &p.ContentMD, &p.Excerpt,
		&p.Status, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.AuthorUsername, &p.AuthorDisplayName, &p.AuthorAvatarURL,
		&p.LikeCount, &p.CommentCount, &p.LikedByMe,
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

type ListParams struct {
	AuthorID         *string
	AuthorName       *string
	Status           *string
	OnlyPublished    bool
	Tag              *string
	OrderByPublished bool
	Asc              bool
	Limit            int
	Offset           int
	ViewerID         *string // nil = 匿名（決策 #49）
	// FeedFor 非 nil 時啟用 feed 篩選：
	// 只回「此人追蹤的作者 + 此人自己」的文章（決策 #45）
	FeedFor *string

	// 搜尋（P4-3）。兩者必須同時給或同時不給。
	// SearchPattern：已 escape 並加上 % 的 ILIKE pattern
	// SearchQuery：未 escape 的原始 query，供 word_similarity 排序
	SearchPattern *string
	SearchQuery   *string
}

// List 回傳文章清單與符合條件的總筆數。
func (r *PostRepository) List(ctx context.Context, p ListParams) ([]*Post, int, error) {
	where := []string{"p.deleted_at IS NULL", "u.deleted_at IS NULL"}
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}

	if p.AuthorID != nil {
		add("p.author_id = $%d", *p.AuthorID)
	}
	if p.AuthorName != nil {
		add("lower(u.username) = lower($%d)", *p.AuthorName)
	}
	if p.OnlyPublished {
		where = append(where, "p.status = 'published'")
	} else if p.Status != nil {
		add("p.status = $%d", *p.Status)
	}
	if p.Tag != nil {
		add(`EXISTS (
			SELECT 1 FROM post_tags pt
			JOIN tags t ON t.id = pt.tag_id
			WHERE pt.post_id = p.id AND t.slug = $%d
		)`, *p.Tag)
	}
	if p.FeedFor != nil {
		// Feed（pull 模型）：我追蹤的人 + 我自己。
		//
		// 用 IN (子查詢) 而非 JOIN follows：JOIN 的話「我自己的文章」
		// 需要額外 UNION 或 OR 才能包含，且可能產生重複列。
		// 子查詢語意清楚，也不影響筆數。
		add(`(
			p.author_id = $%[1]d
			OR p.author_id IN (
				SELECT f.followee_id FROM follows f WHERE f.follower_id = $%[1]d
			)
		)`, *p.FeedFor)
	}

	// 搜尋命中條件（P4-3）。
	//
	// ⚠️ 不可寫 COALESCE(p.excerpt, '')：
	//   (a) 不需要 —— false OR NULL = NULL → 排除，而 NULL excerpt
	//       本來就不該匹配任何東西，排除正是想要的語意
	//   (b) 更嚴重 —— GIN 索引建在裸欄位 excerpt 上，
	//       COALESCE 讓 planner 無法匹配，索引永遠用不到
	//
	// ⚠️ 這裡不能用 add()：pattern 要在兩個位置被引用
	//    （WHERE 的命中條件、ORDER BY 的「標題命中優先」），
	//    所以需要保留 searchIdx 供後面重複使用。
	searchIdx := 0
	if p.SearchPattern != nil {
		args = append(args, *p.SearchPattern)
		searchIdx = len(args)
		where = append(where, fmt.Sprintf(
			`(p.title ILIKE $%[1]d ESCAPE '\' OR p.excerpt ILIKE $%[1]d ESCAPE '\')`,
			searchIdx,
		))
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	// 先算總數。計數欄位不影響筆數，故 count 查詢不需要 LATERAL。
	//
	// ⚠️ SearchQuery（word_similarity 用）必須在這行之後才 append ——
	// count 查詢沒有 ORDER BY，多收一個沒用到的參數 pgx 會直接報錯。
	var total int
	countSQL := `SELECT count(*) FROM posts p JOIN users u ON u.id = p.author_id ` + whereSQL
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Post{}, 0, nil
	}

	// 排序。搜尋模式的排序與其他模式完全不同（相關性優先）。
	var orderSQL string
	if p.SearchPattern != nil {
		args = append(args, *p.SearchQuery)
		qIdx := len(args)
		// 標題命中優先（true > false）→ word_similarity → tie-break（決策 #27）。
		//
		// word_similarity 而非 similarity：後者被 haystack 長度稀釋，
		// 200 字的 excerpt 對短 query 的分數趨近 0。
		// 也不用 strict_word_similarity：它要求對齊詞邊界，中文沒有詞邊界會退化。
		//
		// ⚠️ 第一個參數是**未 escape** 的 query（見 service.SearchTerms 的說明）。
		orderSQL = fmt.Sprintf(
			`ORDER BY (p.title ILIKE $%d ESCAPE '\') DESC,
			          word_similarity($%d, p.title) DESC,
			          p.published_at DESC, p.id DESC`,
			searchIdx, qIdx,
		)
	} else {
		sortCol := "p.created_at"
		if p.OrderByPublished {
			sortCol = "p.published_at"
		}
		dir := "DESC"
		if p.Asc {
			dir = "ASC"
		}
		orderSQL = fmt.Sprintf("ORDER BY %s %s, p.id %s", sortCol, dir, dir)
	}

	// viewer 與分頁參數接在篩選參數之後
	args = append(args, p.ViewerID, p.Limit, p.Offset)
	viewerIdx := len(args) - 2
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	listSQL := fmt.Sprintf(`
		SELECT p.id, p.author_id, p.title, p.slug, p.excerpt, p.status,
		       p.published_at, p.created_at, p.updated_at,
		       u.username, u.display_name, u.avatar_url,
		       COALESCE(lc.cnt, 0), COALESCE(cc.cnt, 0),
		       EXISTS (
		           SELECT 1 FROM post_likes
		           WHERE post_id = p.id AND user_id = $%d
		       )
		FROM posts p
		JOIN users u ON u.id = p.author_id
		%s
		%s %s
		LIMIT $%d OFFSET $%d`,
		viewerIdx, countsJoin, whereSQL, orderSQL, limitIdx, offsetIdx)

	rows, err := r.pool.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	posts := []*Post{}
	ids := []string{}
	for rows.Next() {
		var po Post
		if err := rows.Scan(
			&po.ID, &po.AuthorID, &po.Title, &po.Slug, &po.Excerpt, &po.Status,
			&po.PublishedAt, &po.CreatedAt, &po.UpdatedAt,
			&po.AuthorUsername, &po.AuthorDisplayName, &po.AuthorAvatarURL,
			&po.LikeCount, &po.CommentCount, &po.LikedByMe,
		); err != nil {
			return nil, 0, err
		}
		po.Tags = []string{}
		posts = append(posts, &po)
		ids = append(ids, po.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// 避免 N+1：一次撈完所有標籤再於 Go 分組
	if err := r.attachTags(ctx, posts, ids); err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (r *PostRepository) attachTags(ctx context.Context, posts []*Post, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `
		SELECT pt.post_id, t.slug
		FROM post_tags pt
		JOIN tags t ON t.id = pt.tag_id
		WHERE pt.post_id = ANY($1)
		ORDER BY t.slug ASC`

	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byID := make(map[string]*Post, len(posts))
	for _, p := range posts {
		byID[p.ID] = p
	}
	for rows.Next() {
		var postID, slug string
		if err := rows.Scan(&postID, &slug); err != nil {
			return err
		}
		if p, ok := byID[postID]; ok {
			p.Tags = append(p.Tags, slug)
		}
	}
	return rows.Err()
}
