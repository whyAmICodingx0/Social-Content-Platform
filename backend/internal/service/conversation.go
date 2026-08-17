package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// ErrCannotMessageSelf：對自己開對話（決策：應用層先擋，不讓 DB CHECK 噴 500）
var ErrCannotMessageSelf = errors.New("service: cannot message self")

type ConversationService struct {
	Conversations *repository.ConversationRepository
	Messages      *repository.MessageRepository // P3-4：驗證已讀錨點
	Users         *repository.UserRepository
}

// FindOrCreate 取得或建立與 targetUsername 的對話。
// 第二個回傳值代表是否為本次新建（201 / 200）。
func (s *ConversationService) FindOrCreate(
	ctx context.Context, actorID, targetUsername string,
) (*repository.Conversation, bool, error) {
	target, err := s.Users.GetByUsername(ctx, targetUsername) // 內含 deleted_at IS NULL
	if err != nil {
		return nil, false, err // ErrNotFound → 404
	}

	// 應用層先擋，給可讀的錯誤訊息。
	// DB 的 CHECK (user_low_id < user_high_id) 是最終防線 ——
	// 相等會違反 <，所以它同時擋掉自我對話與順序錯誤。
	if target.ID == actorID {
		return nil, false, ErrCannotMessageSelf
	}

	low, high, err := orderUserIDs(actorID, target.ID)
	if err != nil {
		return nil, false, err
	}

	conv, created, err := s.Conversations.FindOrCreate(ctx, low, high)
	if err != nil {
		return nil, false, err
	}

	// FindOrCreate 只回傳對話本身，補上「對方是誰」給回應使用
	conv.OtherUserID = target.ID
	conv.OtherUsername = target.Username
	conv.OtherDisplayName = target.DisplayName
	conv.OtherAvatarURL = target.AvatarURL

	return conv, created, nil
}

// Get 取得對話（含對方資訊）。非參與者回 ErrNotFound（決策 #73）。
func (s *ConversationService) Get(
	ctx context.Context, conversationID, viewerID string,
) (*repository.Conversation, error) {
	return s.Conversations.GetForUser(ctx, conversationID, viewerID)
}

// orderUserIDs 把兩個 user id 排序成 (low, high)。
//
// ⚠️ 排序結果必須與 PostgreSQL 的 uuid 比較一致，
// 否則會違反 CHECK (user_low_id < user_high_id)。
//
// PostgreSQL 的 uuid 型別是按 16 bytes 的二進位順序比較，
// 所以這裡先 Parse 再用 bytes.Compare。
//
// 【為什麼不直接比字串】全小寫 canonical 形式的字串比較「碰巧」等價
// —— hex 字元 '0'-'9' 在 ASCII 與數值上都小於 'a'-'f'，且連字號位置固定。
// 但那依賴「所有來源都是小寫」這個假設，一旦某處回傳大寫（例如未來
// 換 driver、或從 API 直接收到使用者輸入的 id）就會靜默排錯，
// 而錯誤只會表現為偶發的 CHECK violation。用 bytes.Compare 沒有這個風險。
func orderUserIDs(a, b string) (low, high string, err error) {
	ua, err := uuid.Parse(a)
	if err != nil {
		return "", "", fmt.Errorf("orderUserIDs: parse %q: %w", a, err)
	}
	ub, err := uuid.Parse(b)
	if err != nil {
		return "", "", fmt.Errorf("orderUserIDs: parse %q: %w", b, err)
	}

	if bytes.Compare(ua[:], ub[:]) < 0 {
		return a, b, nil
	}
	return b, a, nil
}

// List 取得我的對話列表。
func (s *ConversationService) List(
	ctx context.Context, viewerID string, limit, offset int,
) ([]*repository.ConversationListItem, int, error) {
	return s.Conversations.ListForUser(ctx, viewerID, limit, offset)
}

// MarkRead 標記已讀（決策 #72：只前進不後退）。
// 回傳更新後的未讀數，讓前端不必再發一次請求。
func (s *ConversationService) MarkRead(
	ctx context.Context, viewerID, conversationID, lastReadMessageID string,
) (int, error) {
	// 1. 權限：非參與者一律 ErrNotFound（決策 #73）
	if _, err := s.Conversations.GetForUser(ctx, conversationID, viewerID); err != nil {
		return 0, err
	}

	// 2. 錨點必須是合法 UUID 且屬於本對話。
	//    MarkRead 的 SQL 在錨點不合法時只是「不寫入任何列」，
	//    無法從 RowsAffected 區分「錨點錯誤」與「送了較舊的位置」，
	//    所以在這裡先驗一次，讓前者能明確回報錯誤。
	if _, err := uuid.Parse(lastReadMessageID); err != nil {
		return 0, &ValidationError{
			Field: "last_read_message_id", Message: "must be a valid UUID",
		}
	}
	if err := s.Messages.CursorExists(ctx, conversationID, lastReadMessageID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, &ValidationError{
				Field:   "last_read_message_id",
				Message: "message does not belong to this conversation",
			}
		}
		return 0, err
	}

	// 3. upsert（送了較舊的位置時安靜跳過，不是錯誤）
	if err := s.Conversations.MarkRead(ctx, conversationID, viewerID, lastReadMessageID); err != nil {
		return 0, err
	}

	return s.Conversations.UnreadCount(ctx, conversationID, viewerID)
}

// TotalUnread 取得所有對話的未讀總數（header 紅點用）。
func (s *ConversationService) TotalUnread(ctx context.Context, viewerID string) (int, error) {
	return s.Conversations.TotalUnreadCount(ctx, viewerID)
}
