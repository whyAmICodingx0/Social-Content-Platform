package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 通知型別（決策 #84）
const (
	NotificationTypeLike    = "like"
	NotificationTypeComment = "comment"
	NotificationTypeFollow  = "follow"
)

// ErrNotificationDuplicate：撞到去重索引（決策 #85）。
//
// ⚠️ 呼叫端據此判斷「這次沒有實際產生通知」，
// 進而決定不推送 WS 事件（決策 #88）。
var ErrNotificationDuplicate = errors.New("repository: notification already exists")

type Notification struct {
	ID        string
	Type      string
	EntityID  *string
	ReadAt    *time.Time
	CreatedAt time.Time

	// actor：LEFT JOIN 帶出。全部為 nil 代表觸發者已軟刪
	ActorUsername    *string
	ActorDisplayName *string
	ActorAvatarURL   *string

	// target：LATERAL 帶出。TargetType 為 nil 代表 type=follow 或內容已不存在。
	// ⚠️ TargetType 恆為 "post"（見 N-0 §0.3 的合約申報）
	TargetType           *string
	TargetTitle          *string
	TargetAuthorUsername *string
	TargetSlug           *string
}

// IsRead 由 read_at 算出（決策 #86：不設 is_read 欄位）
func (n *Notification) IsRead() bool { return n.ReadAt != nil }

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

type CreateNotificationParams struct {
	RecipientID string
	ActorID     string
	Type        string
	EntityID    *string // follow 為 nil
}

// Create 寫入通知。
//
// ⚠️ 回傳 ErrNotificationDuplicate 代表「去重索引擋下、沒有實際插入」——
// 呼叫端必須據此**不推送 WS 事件**（決策 #88）。
//
// 「DB 操作沒回錯就代表成功」在這裡不成立：
// ON CONFLICT DO NOTHING 影響 0 列時，它是「成功地什麼都沒做」。
// 若照推 WS，前端紅點會 +1 但列表無變化 → 幽靈紅點，且清不掉
// （沒有那筆通知可以標為已讀）。
//
// 用**不指名**的 ON CONFLICT DO NOTHING（不寫欄位清單）：
// 此表只有一個 unique index，沒有歧義，且避開 NULLS NOT DISTINCT 的 inference 邊界。
func (r *NotificationRepository) Create(ctx context.Context, p CreateNotificationParams) (*Notification, error) {
	const q = `
		INSERT INTO notifications (recipient_id, actor_id, type, entity_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
		RETURNING id, type, entity_id, read_at, created_at`

	var n Notification
	err := r.pool.QueryRow(ctx, q, p.RecipientID, p.ActorID, p.Type, p.EntityID).
		Scan(&n.ID, &n.Type, &n.EntityID, &n.ReadAt, &n.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotificationDuplicate
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// notificationSelectSQL 是 List 與 GetByID 共用的查詢主體。
//
// 【決策 #91：不做任何過濾】
// actor 軟刪 → LEFT JOIN 得到 NULL → 前端顯示「已刪除的使用者」
// target 失效 → LATERAL 無列 → 前端顯示「內容已不存在」
//
// ⚠️ 必須用 LEFT JOIN 而非 INNER JOIN —— INNER 會讓舊通知整筆消失，
// 導致 unread-count 與列表中的未讀筆數漂移（同決策 #81 類型的 bug）。
//
// ⚠️ 兩個分支都回 'post'（見 N-0 §0.3）：target 指的是「點下去跳到哪」，
// comment 通知點下去就是那篇文章。「這是留言引發的」由 notification.type 表達。
const notificationSelectSQL = `
	SELECT n.id, n.type, n.entity_id, n.read_at, n.created_at,
	       a.username, a.display_name, a.avatar_url,
	       t.target_type, t.title, t.author_username, t.slug
	FROM notifications n
	LEFT JOIN users a ON a.id = n.actor_id AND a.deleted_at IS NULL
	LEFT JOIN LATERAL (
	    -- polymorphic target：兩個分支都帶 n.type 守衛，
	    -- 因此最多只有一個分支產生列；follow 兩邊都不符 → 全 NULL
	    SELECT 'post'::text AS target_type, p.title,
	           pu.username AS author_username, p.slug
	    FROM posts p
	    JOIN users pu ON pu.id = p.author_id AND pu.deleted_at IS NULL
	    WHERE n.type = 'like' AND p.id = n.entity_id AND p.deleted_at IS NULL
	    UNION ALL
	    SELECT 'post'::text, p.title, pu.username, p.slug
	    FROM comments c
	    JOIN posts p ON p.id = c.post_id AND p.deleted_at IS NULL
	    JOIN users pu ON pu.id = p.author_id AND pu.deleted_at IS NULL
	    WHERE n.type = 'comment' AND c.id = n.entity_id AND c.deleted_at IS NULL
	) t ON true`

func scanNotification(row pgx.Row) (*Notification, error) {
	var n Notification
	err := row.Scan(
		&n.ID, &n.Type, &n.EntityID, &n.ReadAt, &n.CreatedAt,
		&n.ActorUsername, &n.ActorDisplayName, &n.ActorAvatarURL,
		&n.TargetType, &n.TargetTitle, &n.TargetAuthorUsername, &n.TargetSlug,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// GetByID 取單筆完整通知（含 actor 與 target），供 WS 推送使用。
// 條件與 List 完全相同（決策 #91），只多一個 id 過濾。
func (r *NotificationRepository) GetByID(ctx context.Context, id, recipientID string) (*Notification, error) {
	q := notificationSelectSQL + `
		WHERE n.id = $2 AND n.recipient_id = $1`

	n, err := scanNotification(r.pool.QueryRow(ctx, q, recipientID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// List 取得某人的通知列表（決策 #91：不做任何過濾）。
func (r *NotificationRepository) List(
	ctx context.Context, recipientID string, limit, offset int,
) ([]*Notification, int, error) {
	const countSQL = `SELECT count(*) FROM notifications WHERE recipient_id = $1`

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, recipientID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Notification{}, 0, nil
	}

	q := notificationSelectSQL + `
		WHERE n.recipient_id = $1
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, recipientID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []*Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, total, rows.Err()
}

// UnreadCount 未讀數（決策 #91）。
//
// ⚠️ 這句刻意「不 JOIN 任何表」—— 這正是決策 #91 選擇「兩邊都不過濾」的
// 主要理由：結構上不可能與 List 的未讀筆數漂移。
// 若哪天有人在 List 加了過濾條件卻忘了這裡，紅點就會顯示清不掉的數字。
func (r *NotificationRepository) UnreadCount(ctx context.Context, recipientID string) (int, error) {
	const q = `
		SELECT count(*) FROM notifications
		WHERE recipient_id = $1 AND read_at IS NULL`

	var n int
	err := r.pool.QueryRow(ctx, q, recipientID).Scan(&n)
	return n, err
}

// MarkRead 標記指定幾筆為已讀。
//
// 三個要點：
//  1. recipient_id = $1 是 IDOR 防護 —— 不能標記別人的通知
//  2. 他人的 id 或不存在的 id 靜默忽略、不回 404（回 404 會洩漏通知存在）
//  3. 尾巴的 read_at IS NULL 已保證冪等，不需要 COALESCE
//
// ⚠️ 回傳的是「實際更新的列數」。呼叫端**必須另外查一次** UnreadCount，
// 不可用 data-modifying CTE 一次算完 —— 【事實】WITH 裡的
// data-modifying statement 與主查詢共用同一個 snapshot，
// 主查詢看不到 CTE 的寫入效果，會回傳「更新前」的未讀數。
//
// $2 用 []string + ::uuid[]：N-1(b) 已實測通過（方案 A）。
func (r *NotificationRepository) MarkRead(
	ctx context.Context, recipientID string, ids []string,
) (int, error) {
	const q = `
		UPDATE notifications
		SET read_at = now()
		WHERE recipient_id = $1 AND id = ANY($2::uuid[]) AND read_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, recipientID, ids)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// MarkAllRead 全部標記已讀。
func (r *NotificationRepository) MarkAllRead(ctx context.Context, recipientID string) (int, error) {
	const q = `
		UPDATE notifications SET read_at = now()
		WHERE recipient_id = $1 AND read_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, recipientID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
