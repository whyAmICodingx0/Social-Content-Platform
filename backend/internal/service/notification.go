package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

const (
	maxMarkReadIDs = 100

	// notifyTimeout：通知寫入的逾時（決策 #89）。
	// 沿用決策 #70（pong 重驗 session）的同一模式與數值。
	notifyTimeout = 3 * time.Second
)

type NotificationService struct {
	Notifications *repository.NotificationRepository
}

// ---------- 讀取 ----------

func (s *NotificationService) List(
	ctx context.Context, recipientID string, limit, offset int,
) ([]*repository.Notification, int, error) {
	return s.Notifications.List(ctx, recipientID, limit, offset)
}

func (s *NotificationService) UnreadCount(ctx context.Context, recipientID string) (int, error) {
	return s.Notifications.UnreadCount(ctx, recipientID)
}

// MarkRead 標記指定幾筆為已讀。
//
// ⚠️ 分成兩句 SQL（更新、再查未讀數），不可用 data-modifying CTE：
// 【事實】WITH 裡的 data-modifying statement 與主查詢共用同一個 snapshot，
// 主查詢看不到 CTE 的寫入效果 → unread_count 會是「更新前」的數字。
// 這種 bug 手測時很難發現（數字確實有變小），要到前端接起來才會看出
// 「紅點永遠差幾格」。
//
// 兩句之間的 race 已被決策 #99 涵蓋（毫秒級，可接受），不需要交易。
func (s *NotificationService) MarkRead(
	ctx context.Context, recipientID string, rawIDs []string,
) (updated int, unread int, err error) {
	if len(rawIDs) == 0 {
		return 0, 0, &ValidationError{Field: "ids", Message: "must not be empty"}
	}
	if len(rawIDs) > maxMarkReadIDs {
		return 0, 0, &ValidationError{
			Field: "ids", Message: "at most 100 ids per request",
		}
	}

	// 先驗格式，不要讓非法字串進 SQL 觸發 uuid 語法錯誤
	ids := make([]string, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, perr := uuid.Parse(raw)
		if perr != nil {
			return 0, 0, &ValidationError{
				Field: "ids", Message: "contains an invalid UUID",
			}
		}
		ids = append(ids, id.String())
	}

	updated, err = s.Notifications.MarkRead(ctx, recipientID, ids)
	if err != nil {
		return 0, 0, err
	}

	unread, err = s.Notifications.UnreadCount(ctx, recipientID)
	if err != nil {
		return 0, 0, err
	}
	return updated, unread, nil
}

// MarkAllRead 全部標記已讀。
//
// 【簡化】不需要再查一次未讀數 —— 依定義就是 0
// （決策 #99 的毫秒級 race 範圍內），直接回 0 省一次查詢。
func (s *NotificationService) MarkAllRead(
	ctx context.Context, recipientID string,
) (updated int, unread int, err error) {
	updated, err = s.Notifications.MarkAllRead(ctx, recipientID)
	if err != nil {
		return 0, 0, err
	}
	return updated, 0, nil
}

// ---------- 產生 ----------

// Notifier 是通知的產生入口，供 like / comment / follow service 呼叫。
//
// 【決策 #89】通知是次要副作用：
//   - 主操作 commit 後才寫入
//   - 寫入失敗只記 log，不回滾主操作、不回 500
//   - ⚠️ **同步呼叫，不可 go notify(...)**（理由見 Notify 的註解）
type Notifier struct {
	Notifications *repository.NotificationRepository
	// OnCreated 在「確實新增了一筆通知」時被呼叫（決策 #88）。
	// 由 main.go 注入 WS 推送，讓 service 不依賴 ws package。
	OnCreated func(recipientID string, n *repository.Notification)
}

// Notify 產生一則通知。
//
// ⚠️ **必須同步呼叫，不可包在 goroutine 裡**，兩個理由：
//  1. 決策 #79 的同一個坑 —— goroutine 裡的 panic 會炸掉整個程序，
//     而 gin 的 recovery middleware 管不到別的 goroutine
//  2. 沒有 graceful shutdown 的等待機制，Render 重啟時飛在半空的 goroutine 直接消失
//
// 代價只是 response 多等一次單筆 INSERT（有索引，毫秒級）。
// 用 WithoutCancel 的整個意義就是「主操作已 commit、response 已注定成功」，
// 這時多等幾毫秒零風險。
//
// 回傳 error 供呼叫端記 log；呼叫端**不應**因此讓主操作失敗。
func (nf *Notifier) Notify(
	ctx context.Context, recipientID, actorID, notifType string, entityID *string,
) error {
	// 不通知自己（決策 #87）。應用層先擋，少一次 DB 往返；
	// DB 的 CHECK (recipient_id <> actor_id) 是最終防線。
	if recipientID == actorID {
		return nil
	}

	// ⚠️ 不可沿用 request context（決策 #89）。
	// 主操作 commit 後 client 可能已斷線（關頁面、Render 冷啟動逾時），
	// 此時 ctx 已被 cancel，INSERT 會直接失敗且靜默 ——
	// 變成「按讚成功但通知永久遺失」。
	// context.WithoutCancel 為 Go 1.21 引入，專案使用 1.24（Dockerfile）。
	nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), notifyTimeout)
	defer cancel()

	n, err := nf.Notifications.Create(nctx, repository.CreateNotificationParams{
		RecipientID: recipientID,
		ActorID:     actorID,
		Type:        notifType,
		EntityID:    entityID,
	})

	if errors.Is(err, repository.ErrNotificationDuplicate) {
		// 去重擋下 —— 沒有實際產生通知。
		// ⚠️ 這裡**不呼叫 OnCreated**（決策 #88）：
		// 若照推 WS，前端紅點會 +1 但列表無變化 → 幽靈紅點且清不掉。
		return nil
	}
	if err != nil {
		return err
	}

	// 取完整通知（含 actor 與 target）供 WS 推送使用
	if nf.OnCreated != nil {
		full, ferr := nf.Notifications.GetByID(nctx, n.ID, recipientID)
		if ferr != nil {
			// 通知已寫入，只是推送用的資料撈不到 —— 呼叫端記 log 即可，
			// 使用者下次重新整理或重連時會看到（決策 #90）
			return ferr
		}
		nf.OnCreated(recipientID, full)
	}
	return nil
}
