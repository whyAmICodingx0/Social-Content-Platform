package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// maxMessageLen 決策 #75：2000 個 Unicode rune（不是 byte）
const maxMessageLen = 2000

type MessageService struct {
	Messages      *repository.MessageRepository
	Conversations *repository.ConversationRepository
}

// Send 送出訊息。
//
// 回傳 (訊息, 是否為既有訊息, error)：
//
//	isExisting=false → 新建 → handler 回 201 並推送 WS
//	isExisting=true  → 冪等重送 → handler 回 200，不重複推送
func (s *MessageService) Send(
	ctx context.Context, actorID, conversationID, rawClientID, rawContent string,
) (*repository.Message, *repository.Conversation, bool, error) {
	// 1. 權限：非參與者一律 ErrNotFound（決策 #73）
	conv, err := s.Conversations.GetForUser(ctx, conversationID, actorID)
	if err != nil {
		return nil, nil, false, err
	}

	// 2. client_message_id 必須是合法 UUID
	clientID, err := uuid.Parse(strings.TrimSpace(rawClientID))
	if err != nil {
		return nil, nil, false, &ValidationError{
			Field: "client_message_id", Message: "must be a valid UUID",
		}
	}

	// 3. 內容驗證
	content, err := validateMessageContent(rawContent)
	if err != nil {
		return nil, nil, false, err
	}

	// 4. 寫入
	m, err := s.Messages.Create(ctx, conv.ID, actorID, clientID.String(), content)
	if err == nil {
		return m, conv, false, nil
	}

	// 5. 冪等收斂（決策 #65）：撞到 UNIQUE(sender_id, client_message_id)
	//    → 這是同一則訊息的重送，查出既有那則回傳。
	//
	//    ⚠️ 這裡與決策 #19（signup）是同一個模式，但有一點不同：
	//    Create 沒有開啟顯式交易，單一 statement 失敗後連線即可繼續使用，
	//    不存在「aborted transaction」的問題。
	if errors.Is(err, repository.ErrDuplicateClientMessageID) {
		existing, ferr := s.Messages.FindByClientID(ctx, actorID, clientID.String())
		if ferr != nil {
			return nil, nil, false, ferr
		}
		return existing, conv, true, nil
	}

	return nil, nil, false, err
}

// validateMessageContent 決策 #75：
//  1. 先 trim，trim 後為空 → 400
//  2. 上限 2000 個 Unicode rune
//
// ⚠️ 必須用 utf8.RuneCountInString。len(string) 與 Gin binding 的
// max tag 算的都是 byte，一個中文字 3 bytes，中文使用者會在 666 字
// 就被擋下。這與決策 #46（留言）是同一個坑。
func validateMessageContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)

	if content == "" {
		return "", &ValidationError{Field: "content", Message: "must not be empty"}
	}
	if utf8.RuneCountInString(content) > maxMessageLen {
		return "", &ValidationError{
			Field: "content", Message: "must be at most 2000 characters",
		}
	}
	return content, nil
}

// 分頁參數（決策 #67）
const (
	defaultMessageLimit = 30
	maxMessageLimit     = 100
)

// ErrInvalidCursor：before / after 的錨點不存在或不屬於本對話。
var ErrInvalidCursor = errors.New("service: invalid cursor")

type ListMessagesInput struct {
	ConversationID string
	ViewerID       string
	Before         string
	After          string
	Limit          int
}

// ListMessages 取得歷史訊息（cursor 分頁）。
func (s *MessageService) ListMessages(
	ctx context.Context, in ListMessagesInput,
) ([]*repository.Message, bool, error) {
	// 1. 權限：非參與者一律 ErrNotFound（決策 #73）
	if _, err := s.Conversations.GetForUser(ctx, in.ConversationID, in.ViewerID); err != nil {
		return nil, false, err
	}

	// 2. before 與 after 互斥
	if in.Before != "" && in.After != "" {
		return nil, false, &ValidationError{
			Field: "cursor", Message: "before and after are mutually exclusive",
		}
	}

	// 3. limit 夾住（沿用決策 #26 的寬容處理：不合法就夾回範圍，不回 400）
	limit := in.Limit
	if limit <= 0 {
		limit = defaultMessageLimit
	}
	if limit > maxMessageLimit {
		limit = maxMessageLimit
	}

	// 4. cursor 必須是合法 UUID 且屬於本對話。
	//    先擋格式，避免把非法字串送進 SQL 觸發 uuid 語法錯誤。
	cur := repository.MessageCursor{Limit: limit}

	if in.Before != "" {
		if err := s.validateCursor(ctx, in.ConversationID, in.Before); err != nil {
			return nil, false, err
		}
		cur.Before = in.Before
	}
	if in.After != "" {
		if err := s.validateCursor(ctx, in.ConversationID, in.After); err != nil {
			return nil, false, err
		}
		cur.After = in.After
	}

	return s.Messages.ListMessages(ctx, in.ConversationID, cur)
}

func (s *MessageService) validateCursor(ctx context.Context, conversationID, messageID string) error {
	if _, err := uuid.Parse(messageID); err != nil {
		return ErrInvalidCursor
	}
	err := s.Messages.CursorExists(ctx, conversationID, messageID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidCursor
	}
	return err
}
