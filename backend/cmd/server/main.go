package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/handler"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

func main() {
	// 1. 設定
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. 基礎設施:PostgreSQL + Redis(皆惰性連線)
	pool, err := repository.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	rdb := store.NewRedisClient(cfg.RedisAddr)
	defer rdb.Close()

	// 3. 組裝(wiring)
	userRepo := repository.NewUserRepository(pool)
	sessions := store.NewSessionStore(rdb)
	states := store.NewOAuthStateStore(rdb)
	pendings := store.NewPendingSignupStore(rdb) // 任務 F 使用;先建好
	_ = pendings                                 // 暫時避開 unused 編譯錯誤,任務 F 移除這行

	cookieMgr := &cookies.Manager{Secure: cfg.CookieSecure}

	// ★ interface 替換時刻:NoopSessionStore → 真正的 Redis SessionStore。
	//   middleware 的程式碼零修改。
	auth := &middleware.Auth{Store: sessions, Users: userRepo}

	healthHandler := handler.NewHealthHandler(pool, rdb)

	// 4. Gin 引擎
	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.CustomRecovery(func(c *gin.Context, _ any) {
		api.Fail(c, http.StatusInternalServerError, api.CodeInternalError,
			"Internal server error")
	}))

	// 5. 路由
	r.GET("/healthz", healthHandler.Healthz)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.CSRF(cfg.FrontendOrigins))

	// 任務 F 會換成真正的 /me handler
	v1.GET("/me", auth.Required(), func(c *gin.Context) {})

	if cfg.IsDev() {
		dev := handler.DevHandler{
			Users:    userRepo,
			Sessions: sessions,
			States:   states,
			Cookies:  cookieMgr,
		}
		v1.POST("/dev/echo", dev.Echo)
		v1.GET("/dev/users/:id", dev.GetUser)
		v1.GET("/dev/whoami", auth.Optional(), dev.WhoAmI)
		v1.POST("/dev/login", dev.Login)          // 本步驟驗收用,任務 F 後刪
		v1.POST("/dev/state-demo", dev.StateDemo) // 本步驟驗收用,任務 F 後刪
	}

	// 6. 啟動
	log.Printf("listening on :%s (env=%s)", cfg.Port, cfg.AppEnv)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
