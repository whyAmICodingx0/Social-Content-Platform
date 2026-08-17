package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Conversation 是兩人之間的對話。
//
// DB 以 (user_low_id, user_high_id) 保證唯一性，但這是儲存細節 ——
// 對外一律以 Other* 欄位表達「對方是誰」，前端不需要知道 low/high 的存在。
type Conversation struct {
	ID         string
	UserLowID  string
	UserHighID string
	CreatedAt  time.Time

	// 由 JOIN 帶出：相對於查詢者的「對方」
	OtherUserID      string
	OtherUsername    string
	OtherDisplayName *string
	OtherAvatarURL   *string
}

// Participants 回傳這個對話的兩位參與者 id（推送 WS 事件用）。
func (c *Conversation) Participants() []string {
	return []string{c.UserLowID, c.UserHighID}
}

type ConversationRepository struct {
	pool *pgxpool.Pool
}

func NewConversationRepository(pool *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{pool: pool}
}

// FindOrCreate 取得或建立兩人之間的對話。
//
// 呼叫端必須先把兩個 id 排序好（lowID < highID，見 service.orderUserIDs），
// 否則會違反 CHECK (user_low_id < user_high_id)。
//
// 回傳的第二個值代表「是否為本次新建」：新建 → 201，既有 → 200。
//
// 【為什麼分成兩個獨立的 statement】
// ON CONFLICT DO NOTHING 不會讓交易 abort，所以不需要決策 #19 那種
// 「交易外重查」的處理 —— 但也不能把 INSERT 與重查寫在同一個 CTE 裡：
// CTE 內的 SELECT 使用「語句開始時」的快照，看不到另一個交易剛 commit
// 的那一列，會回傳零筆。分成兩個 statement 後，第二個 SELECT 取得
// 新的快照（READ COMMITTED），必然看得到已 commit 的對話。
func (r *ConversationRepository) FindOrCreate(ctx context.Context, lowID, highID string) (*Conversation, bool, error) {
	const insertSQL = `
		INSERT INTO conversations (user_low_id, user_high_id)
		VALUES ($1, $2)
		ON CONFLICT (user_low_id, user_high_id) DO NOTHING
		RETURNING id, user_low_id, user_high_id, created_at`

	var c Conversation
	err := r.pool.QueryRow(ctx, insertSQL, lowID, highID).
		Scan(&c.ID, &c.UserLowID, &c.UserHighID, &c.CreatedAt)

	if err == nil {
		return &c, true, nil // 本次新建
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	// ErrNoRows 代表 DO NOTHING 跳過了 INSERT → 對話已存在，重新查詢。
	const selectSQL = `
		SELECT id, user_low_id, user_high_id, created_at
		FROM conversations
		WHERE user_low_id = $1 AND user_high_id = $2`

	err = r.pool.QueryRow(ctx, selectSQL, lowID, highID).
		Scan(&c.ID, &c.UserLowID, &c.UserHighID, &c.CreatedAt)
	if err != nil {
		// 理論上不會發生：DO NOTHING 只在對方「已 commit」時才跳過，
		// 若對方 rollback，我們的 INSERT 會取得鎖並成功。
		return nil, false, err
	}
	return &c, false, nil
}

// GetForUser 取得對話，並帶出「相對於 viewerID 的對方」資訊。
//
// 【決策 #73】非參與者一律回 ErrNotFound（handler 轉 404），不回 403 ——
// 延續不洩漏資源存在性的原則。
//
// 對方已被軟刪時同樣回 ErrNotFound（決策 #74 的「活著的使用者」語意）。
func (r *ConversationRepository) GetForUser(ctx context.Context, conversationID, viewerID string) (*Conversation, error) {
	// CASE 只出現在這裡（「對方是誰」），不會散落到各處 ——
	// 這正是把已讀狀態獨立成 conversation_reads 表所避免的那種擴散。
	const q = `
		SELECT c.id, c.user_low_id, c.user_high_id, c.created_at,
		       u.id, u.username, u.display_name, u.avatar_url
		FROM conversations c
		JOIN users u ON u.id = (
		    CASE WHEN c.user_low_id = $2 THEN c.user_high_id ELSE c.user_low_id END
		)
		WHERE c.id = $1
		  AND (c.user_low_id = $2 OR c.user_high_id = $2)
		  AND u.deleted_at IS NULL`

	var c Conversation
	err := r.pool.QueryRow(ctx, q, conversationID, viewerID).Scan(
		&c.ID, &c.UserLowID, &c.UserHighID, &c.CreatedAt,
		&c.OtherUserID, &c.OtherUsername, &c.OtherDisplayName, &c.OtherAvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ConversationListItem 是對話列表的一筆。
type ConversationListItem struct {
	Conversation

	// 最後一則訊息（列表一定有訊息，故必然非空）
	LastMessageID        string
	LastMessageContent   string
	LastMessageSenderID  string
	LastMessageCreatedAt time.Time

	UnreadCount int
}

// ListForUser 取得某人的對話列表（決策 #74）。
//
// 規則：
//   - 沒有任何訊息的對話不出現（LATERAL 結果為 NULL 就被 JOIN 濾掉）
//   - 對方已軟刪的對話排除
//   - 依最後一則訊息時間由新到舊，id 作 tie-break（決策 #27）
//   - offset 分頁（只有 messages 用 cursor）
//
// 【未讀數的 NULL 陷阱】從未讀過的對話沒有 conversation_reads 那一列，
// LEFT JOIN 出來是 NULL，而 `m.created_at > NULL` 的結果是 NULL 不是 true ——
// 未讀數會全部算成 0。必須用 COALESCE(r.last_read_at, '-infinity'::timestamptz)：
// 任何時間都大於 -infinity，所以「從未讀過」= 全部他人訊息皆未讀。
//
// 未讀數不計自己傳的訊息（sender_id <> viewer）。
func (r *ConversationRepository) ListForUser(
	ctx context.Context, viewerID string, limit, offset int,
) ([]*ConversationListItem, int, error) {
	// FROM 與 WHERE 拆成兩段：list 查詢需要在兩者之間插入未讀數的 LATERAL。
	// SQL 的 JOIN 必須全部出現在 WHERE 之前，合成一個常數就沒地方插了。
	const fromClause = `
		FROM conversations c
		JOIN users u ON u.id = (
		    CASE WHEN c.user_low_id = $1 THEN c.user_high_id ELSE c.user_low_id END
		) AND u.deleted_at IS NULL
		-- 取最後一則訊息。JOIN LATERAL（非 LEFT）→ 沒有訊息的對話直接被濾掉
		JOIN LATERAL (
		    SELECT m.id, m.content, m.sender_id, m.created_at
		    FROM messages m
		    WHERE m.conversation_id = c.id
		    ORDER BY m.created_at DESC, m.id DESC
		    LIMIT 1
		) lm ON true`

	// OR 加上括號：目前沒有其他 WHERE 條件所以不影響，
	// 但日後若加上 AND，沒括號會讓整個條件靜默失效。
	const whereClause = `
		WHERE (c.user_low_id = $1 OR c.user_high_id = $1)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) `+fromClause+whereClause, viewerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ConversationListItem{}, 0, nil
	}

	const listSQL = `
		SELECT c.id, c.user_low_id, c.user_high_id, c.created_at,
		       u.id, u.username, u.display_name, u.avatar_url,
		       lm.id, lm.content, lm.sender_id, lm.created_at,
		       COALESCE(uc.cnt, 0)
		` + fromClause + `
		-- 未讀數：他人傳的、且晚於我的已讀位置。
		-- COALESCE(..., '-infinity') 處理「從未讀過」的情況：
		-- 沒有 conversation_reads 那一列時 LEFT JOIN 出來是 NULL，
		-- 而 m2.created_at > NULL 的結果是 NULL 不是 true，會全部算成 0。
		LEFT JOIN LATERAL (
		    SELECT count(*) AS cnt
		    FROM messages m2
		    LEFT JOIN conversation_reads r
		           ON r.conversation_id = c.id AND r.user_id = $1
		    WHERE m2.conversation_id = c.id
		      AND m2.sender_id <> $1
		      AND m2.created_at > COALESCE(r.last_read_at, '-infinity'::timestamptz)
		) uc ON true
		` + whereClause + `
		ORDER BY lm.created_at DESC, c.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, listSQL, viewerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []*ConversationListItem{}
	for rows.Next() {
		var it ConversationListItem
		if err := rows.Scan(
			&it.ID, &it.UserLowID, &it.UserHighID, &it.CreatedAt,
			&it.OtherUserID, &it.OtherUsername, &it.OtherDisplayName, &it.OtherAvatarURL,
			&it.LastMessageID, &it.LastMessageContent,
			&it.LastMessageSenderID, &it.LastMessageCreatedAt,
			&it.UnreadCount,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, &it)
	}
	return items, total, rows.Err()
}
