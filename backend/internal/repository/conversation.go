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

// MarkRead 更新某人在某對話的已讀位置（決策 #71、#72）。
//
// 【只前進不後退】最後一行的 WHERE 是整個語句的重點。
// 使用者開兩個分頁時，A 滑到最新標記已讀、B 還停在舊位置也送出標記——
// 順序一亂，未讀數就會從 0 憑空長回。加上 WHERE 之後，
// 舊的標記無法覆蓋新的（DO UPDATE 的條件不成立就整筆跳過）。
//
// 【錨點驗證在 DB 層】SELECT ... FROM messages WHERE conversation_id = $1
// 同時把「這則訊息真的屬於這個對話」擋在資料庫層，
// 而非只信任應用層檢查。錨點不合法時 INSERT 的來源是空集合，
// 不會寫入任何列 —— 呼叫端據此回傳 ErrNotFound。
//
// 【為什麼存兩個欄位】last_read_message_id 給未來的「未讀分隔線」UI 用；
// last_read_at 讓未讀數查詢維持單純的 m.created_at > r.last_read_at
// （若只存 message_id，每個對話都要多一層 LATERAL 去解析錨點）。
// 兩者在同一句 SQL 內從同一列取值，結構上不可能漂移。
func (r *ConversationRepository) MarkRead(
	ctx context.Context, conversationID, userID, lastReadMessageID string,
) error {
	const q = `
		INSERT INTO conversation_reads
		       (conversation_id, user_id, last_read_message_id, last_read_at, updated_at)
		SELECT $1, $2, m.id, m.created_at, now()
		FROM   messages m
		WHERE  m.id = $3 AND m.conversation_id = $1
		ON CONFLICT (conversation_id, user_id) DO UPDATE
		SET    last_read_message_id = EXCLUDED.last_read_message_id,
		       last_read_at         = EXCLUDED.last_read_at,
		       updated_at           = now()
		WHERE  EXCLUDED.last_read_at > conversation_reads.last_read_at`

	tag, err := r.pool.Exec(ctx, q, conversationID, userID, lastReadMessageID)
	if err != nil {
		return err
	}

	// RowsAffected 為 0 有兩種可能：
	//   (a) 錨點訊息不存在或不屬於本對話 → 應回報錯誤
	//   (b) DO UPDATE 的 WHERE 不成立（送了較舊的位置）→ 正常，不是錯誤
	// 兩者無法從 tag 區分，所以由呼叫端另外驗證錨點（見 service）。
	_ = tag
	return nil
}

// UnreadCount 取得某人在某對話的未讀數。
//
// 【NULL 陷阱】從未讀過的對話沒有 conversation_reads 那一列，
// LEFT JOIN 出來是 NULL，而 `m.created_at > NULL` 的結果是 NULL 不是 true ——
// 未讀數會算成 0。COALESCE(..., '-infinity') 讓「從未讀過」
// 正確地等於「全部他人訊息皆未讀」。
//
// 不計自己傳的訊息（sender_id <> $2）。
func (r *ConversationRepository) UnreadCount(
	ctx context.Context, conversationID, userID string,
) (int, error) {
	const q = `
		SELECT count(*)
		FROM messages m
		LEFT JOIN conversation_reads r
		       ON r.conversation_id = m.conversation_id AND r.user_id = $2
		WHERE m.conversation_id = $1
		  AND m.sender_id <> $2
		  AND m.created_at > COALESCE(r.last_read_at, '-infinity'::timestamptz)`

	var n int
	err := r.pool.QueryRow(ctx, q, conversationID, userID).Scan(&n)
	return n, err
}

// TotalUnreadCount 取得某人所有對話的未讀總數（header 紅點用）。
//
// 條件與對話列表一致（決策 #74）：
//   - 對方已軟刪的對話不計入
//   - 沒有訊息的對話自然不會有未讀
func (r *ConversationRepository) TotalUnreadCount(ctx context.Context, userID string) (int, error) {
	const q = `
		SELECT count(*)
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		JOIN users other ON other.id = (
		    CASE WHEN c.user_low_id = $1 THEN c.user_high_id ELSE c.user_low_id END
		) AND other.deleted_at IS NULL
		LEFT JOIN conversation_reads r
		       ON r.conversation_id = c.id AND r.user_id = $1
		WHERE (c.user_low_id = $1 OR c.user_high_id = $1)
		  AND m.sender_id <> $1
		  AND m.created_at > COALESCE(r.last_read_at, '-infinity'::timestamptz)`

	var n int
	err := r.pool.QueryRow(ctx, q, userID).Scan(&n)
	return n, err
}
