package store

import (
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient 從連線 URL 建立 client。
//
// 支援兩種格式:
//
//	redis://localhost:6379                    本機(無 TLS)
//	rediss://default:<password>@host:6379      Upstash(強制 TLS)
func NewRedisClient(redisURL string) (*redis.Client, error) {
	if !strings.Contains(redisURL, "://") {
		redisURL = "redis://" + redisURL
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return redis.NewClient(opt), nil
}
