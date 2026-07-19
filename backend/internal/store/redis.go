package store

import (
	"github.com/redis/go-redis/v9"
)

// NewRedisClient 建立 Redis 連線。
// 【事實】和 pgxpool 一樣是惰性連線:這裡不會真的連上,
// 第一次執行指令才連。Redis 沒開時伺服器照樣起得來,
// 由 healthz 回報、由各 store 的錯誤處理承接(spec 4.12)。
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}
