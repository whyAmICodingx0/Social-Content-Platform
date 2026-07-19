package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// RedisKeyPrefix 對應決策 #11:前綴集中定義,不散落字串。
// 任務 E 還用不到,先放著給 Redis 基建用。
const RedisKeyPrefix = "scp:"

type Config struct {
	AppEnv             string // "dev" | "prod"
	Port               string
	DatabaseURL        string
	RedisAddr          string   // Redis 基建(下一步)才會用
	CookieSecure       bool     // spec 4.6:dev=false、prod=true
	FrontendOrigins    []string // spec 4.13:CSRF Origin 白名單
	GoogleClientID     string   // 任務 F 才會填
	GoogleClientSecret string
	GoogleRedirectURL  string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env 不存在也沒關係

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "dev"),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://app:devpassword@localhost:5432/social_dev?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		CookieSecure:       getEnv("COOKIE_SECURE", "false") == "true",
		FrontendOrigins:    splitAndTrim(getEnv("FRONTEND_ORIGINS", "http://localhost:5173")),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}

	// spec 4.6 的護欄:正式環境不允許不安全的 cookie 設定
	if cfg.AppEnv == "prod" && !cfg.CookieSecure {
		return nil, fmt.Errorf("COOKIE_SECURE must be true when APP_ENV=prod")
	}
	return cfg, nil
}

func (c *Config) IsDev() bool { return c.AppEnv == "dev" }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
