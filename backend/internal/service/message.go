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
