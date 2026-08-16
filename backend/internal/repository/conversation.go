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
