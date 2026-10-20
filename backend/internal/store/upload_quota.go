package store

import (
	"context"
	"time"

	// ⚠️ 需確認：與 store 套件內其他檔案（session.go 等）使用相同的 import 路徑
	"github.com/redis/go-redis/v9"
)

// UploadQuotaStore 計算每人每日的上傳簽章次數（決策 #137）。
//
// 因為決策 #135 讓每個簽章最多產生一個檔案，
// 這裡數的簽章次數 = 實際上傳次數的上限。
type UploadQuotaStore struct {
	rdb *redis.Client
}

func NewUploadQuotaStore(rdb *redis.Client) *UploadQuotaStore {
	return &UploadQuotaStore{rdb: rdb}
}

// uploadQuotaKey 用 UTC 日期（決策 #22）。
// 台灣時間早上 8 點前，UTC 日期仍是前一天。
func uploadQuotaKey(userID string, now time.Time) string {
	return "scp:upload_quota:" + userID + ":" + now.UTC().Format("2006-01-02")
}

// Incr 將今日計數 +1 並回傳新值。
//
// INCR 與 EXPIRE 包在 MULTI/EXEC 交易裡：兩者要嘛都執行、要嘛都不執行，
// 不會因為兩個指令之間崩潰而留下永不過期的 key。
// EXPIRE 每次無條件重設 26 小時 —— key 本身帶日期，重設無害。
func (s *UploadQuotaStore) Incr(ctx context.Context, userID string, now time.Time) (int64, error) {
	key := uploadQuotaKey(userID, now)

	pipe := s.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 26*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
