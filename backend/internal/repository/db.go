package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool 建立連線池。
// 【事實】pgxpool 是惰性連線:這裡不會真的連 DB,第一次查詢才連。
// 所以「DB 沒開」不會讓伺服器起不來,而是由 /healthz 的 Ping 回報
// —— 維持你原本 healthz 的行為(down → error、up → 恢復)。
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
