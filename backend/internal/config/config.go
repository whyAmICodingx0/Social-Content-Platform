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
	RedisURL           string
	CookieSecure       bool     // spec 4.6:dev=false、prod=true
	FrontendOrigins    []string // spec 4.13:CSRF Origin 白名單
	GoogleClientID     string   // 任務 F 才會填
	GoogleClientSecret string
	GoogleRedirectURL  string
	PostLoginURL       string // callback 成功（老用戶）導向
	OnboardingURL      string // callback 新用戶導向
	AuthErrorURL       string // OAuth 失敗導向（會附上 ?error=xxx）

	// Cloudinary（P5-1）
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env 不存在也沒關係

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "dev"),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://app:devpassword@localhost:5432/social_dev?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		CookieSecure:       getEnv("COOKIE_SECURE", "false") == "true",
		FrontendOrigins:    splitAndTrim(getEnv("FRONTEND_ORIGINS", "http://localhost:5173")),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		PostLoginURL:       getEnv("POST_LOGIN_URL", "http://localhost:5173/"),
		OnboardingURL:      getEnv("ONBOARDING_URL", "http://localhost:5173/onboarding"),
		AuthErrorURL:       getEnv("AUTH_ERROR_URL", "http://localhost:5173/login"),

		CloudinaryCloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret: os.Getenv("CLOUDINARY_API_SECRET"),
	}

	// spec 4.6 的護欄:正式環境不允許不安全的 cookie 設定
	if cfg.AppEnv == "prod" && !cfg.CookieSecure {
		return nil, fmt.Errorf("COOKIE_SECURE must be true when APP_ENV=prod")
	}
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" || cfg.GoogleRedirectURL == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_REDIRECT_URL are required（見任務 F-1）")
	}
	// P5-1 護欄：正式環境缺任一 Cloudinary 憑證就拒絕啟動（同 COOKIE_SECURE 的模式）。
	// 本機若漏設，服務仍會啟動，媒體端點回 503。
	if cfg.AppEnv == "prod" &&
		(cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "") {
		return nil, fmt.Errorf("CLOUDINARY_CLOUD_NAME / CLOUDINARY_API_KEY / CLOUDINARY_API_SECRET are required when APP_ENV=prod")
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
