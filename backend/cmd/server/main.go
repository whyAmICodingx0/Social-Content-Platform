package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/handler"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

func main() {
	// 1. 設定
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. 資料庫連線池(惰性連線,DB 沒開伺服器也起得來,由 healthz 回報)
	pool, err := repository.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// 3. 組裝(wiring):repository → middleware / handler
	userRepo := repository.NewUserRepository(pool)
	auth := &middleware.Auth{
		Store: middleware.NoopSessionStore{}, // Redis 基建完成後換成真的
		Users: userRepo,
	}
	healthHandler := handler.NewHealthHandler(pool)

	// 4. Gin 引擎:panic 時也要回合約格式的 500(spec 3.2 INTERNAL_ERROR)
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

	// 任務 F 會把這個 placeholder 換成真正的 /me handler。
	// 現在掛著是為了驗收 Required middleware(沒有 Redis,永遠 401)。
	v1.GET("/me", auth.Required(), func(c *gin.Context) {})

	if cfg.IsDev() {
		dev := handler.DevHandler{Users: userRepo}
		v1.POST("/dev/echo", dev.Echo)
		v1.GET("/dev/users/:id", dev.GetUser)
		v1.GET("/dev/whoami", auth.Optional(), dev.WhoAmI)
	}

	// 6. 啟動
	log.Printf("listening on :%s (env=%s)", cfg.Port, cfg.AppEnv)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
