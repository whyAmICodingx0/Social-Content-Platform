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
