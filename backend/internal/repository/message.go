package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDuplicateClientMessageID：撞到 UNIQUE(sender_id, client_message_id)。
// service 捕獲後改查既有訊息並回 200（決策 #65 的冪等語意）。
var ErrDuplicateClientMessageID = errors.New("repository: duplicate client_message_id")

type Message struct {
	ID              string
	ConversationID  string
	SenderID        string
	ClientMessageID string
	Content         string
	CreatedAt       time.Time

	// 由 JOIN 帶出
	SenderUsername    string
	SenderDisplayName *string
	SenderAvatarURL   *string
}

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

// selectMessageColumns 是 Create 與 FindByClientID 共用的欄位清單，
// 確保兩條路徑回傳完全相同的形狀。
const selectMessageColumns = `
	m.id, m.conversation_id, m.sender_id, m.client_message_id,
	m.content, m.created_at,
	u.username, u.display_name, u.avatar_url`

func scanMessage(row pgx.Row) (*Message, error) {
	var m Message
	err := row.Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID,
		&m.Content, &m.CreatedAt,
		&m.SenderUsername, &m.SenderDisplayName, &m.SenderAvatarURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 寫入訊息並回傳含寄件者資訊的完整結果。
// 用 CTE 一次完成 INSERT + JOIN users，省一次往返。
func (r *MessageRepository) Create(
	ctx context.Context, conversationID, senderID, clientMessageID, content string,
) (*Message, error) {
	const q = `
		WITH inserted AS (
			INSERT INTO messages (conversation_id, sender_id, client_message_id, content)
			VALUES ($1, $2, $3, $4)
			RETURNING id, conversation_id, sender_id, client_message_id, content, created_at
		)
		SELECT m.id, m.conversation_id, m.sender_id, m.client_message_id,
		       m.content, m.created_at,
		       u.username, u.display_name, u.avatar_url
		FROM inserted m
		JOIN users u ON u.id = m.sender_id`

	m, err := scanMessage(r.pool.QueryRow(ctx, q, conversationID, senderID, clientMessageID, content))
	if err != nil {
		return nil, mapMessageViolation(err)
	}
	return m, nil
}

// FindByClientID 以 (sender_id, client_message_id) 查既有訊息。
// 冪等重送時走這條路徑。
func (r *MessageRepository) FindByClientID(
	ctx context.Context, senderID, clientMessageID string,
) (*Message, error) {
	const q = `
		SELECT ` + selectMessageColumns + `
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.sender_id = $1 AND m.client_message_id = $2`

	return scanMessage(r.pool.QueryRow(ctx, q, senderID, clientMessageID))
}

func mapMessageViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == "messages_client_id_key" {
		return ErrDuplicateClientMessageID
	}
	return err
}

// MessageCursor 描述一次歷史查詢的位置。
// Before 與 After 互斥（service 層已驗證），兩者皆空代表「取最新的」。
type MessageCursor struct {
	Before string // message_id：取比它更舊的
	After  string // message_id：取比它更新的
	Limit  int
}

// ListMessages 以 keyset（cursor）分頁取得歷史訊息（決策 #67）。
//
// 回傳的訊息一律以 (created_at, id) 遞增排序 —— 最舊在前，
// 也就是畫面由上而下的順序（決策 #68）。before 模式的查詢是 DESC，
// 因此在這裡反轉後才回傳，讓兩種模式的輸出形狀一致。
//
// hasMore 的語意依模式而異：
//
//	before → 還有更舊的
//	after  → 還有更新的（前端必須迴圈抓到 false）
func (r *MessageRepository) ListMessages(
	ctx context.Context, conversationID string, cur MessageCursor,
) (msgs []*Message, hasMore bool, err error) {
	// 多抓一筆用來判斷 has_more，回傳前丟掉
	limitPlusOne := cur.Limit + 1

	var q string
	var args []any
	reverse := false

	switch {
	case cur.After != "":
		// 往下補：比錨點更新的，自然是 ASC
		//
		// (created_at, id) 的複合比較：tie-break 完整存在，
		// 不會因為同一毫秒的多則訊息而漏掉或重複。
		// 子查詢同時驗證錨點屬於本對話（service 也驗過，這裡是 DB 層兜底）。
		q = `
			SELECT ` + selectMessageColumns + `
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			  AND (m.created_at, m.id) > (
			        SELECT created_at, id FROM messages
			        WHERE id = $2 AND conversation_id = $1
			      )
			ORDER BY m.created_at ASC, m.id ASC
			LIMIT $3`
		args = []any{conversationID, cur.After, limitPlusOne}

	case cur.Before != "":
		// 往上捲：比錨點更舊的，自然是 DESC，回傳前反轉
		q = `
			SELECT ` + selectMessageColumns + `
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			  AND (m.created_at, m.id) < (
			        SELECT created_at, id FROM messages
			        WHERE id = $2 AND conversation_id = $1
			      )
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $3`
		args = []any{conversationID, cur.Before, limitPlusOne}
		reverse = true

	default:
		// 首次載入：取最新的 limit 筆，同樣反轉成遞增
		q = `
			SELECT ` + selectMessageColumns + `
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $2`
		args = []any{conversationID, limitPlusOne}
		reverse = true
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := []*Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.ClientMessageID,
			&m.Content, &m.CreatedAt,
			&m.SenderUsername, &m.SenderDisplayName, &m.SenderAvatarURL,
		); err != nil {
			return nil, false, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	// 多抓的那一筆代表「還有更多」，丟掉它
	if len(out) > cur.Limit {
		hasMore = true
		out = out[:cur.Limit]
	}

	if reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	return out, hasMore, nil
}

// CursorExists 驗證錨點訊息存在且屬於指定對話。
// 不存在 → ErrNotFound（handler 轉 400 INVALID_CURSOR）。
func (r *MessageRepository) CursorExists(ctx context.Context, conversationID, messageID string) error {
	const q = `SELECT 1 FROM messages WHERE id = $1 AND conversation_id = $2`

	var dummy int
	err := r.pool.QueryRow(ctx, q, messageID, conversationID).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
