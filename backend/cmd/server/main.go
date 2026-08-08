package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/config"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/cookies"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/googleoauth"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/handler"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := repository.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	rdb := store.NewRedisClient(cfg.RedisAddr)
	defer rdb.Close()

	// wiring:repository → service → handler
	userRepo := repository.NewUserRepository(pool)
	authRepo := repository.NewAuthRepository(pool)

	sessions := store.NewSessionStore(rdb)
	states := store.NewOAuthStateStore(rdb)
	pendings := store.NewPendingSignupStore(rdb)

	cookieMgr := &cookies.Manager{Secure: cfg.CookieSecure}
	auth := &middleware.Auth{Store: sessions, Users: userRepo}

	userSvc := &service.UserService{Users: userRepo}
	userHandler := &handler.UserHandler{Svc: userSvc}

	authSvc := &service.AuthService{Auth: authRepo, Users: userRepo}
	authHandler := &handler.AuthHandler{
		Google:   googleoauth.New(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL),
		Svc:      authSvc,
		Sessions: sessions,
		States:   states,
		Pendings: pendings,
		Cookies:  cookieMgr,
		Cfg:      cfg,
	}
	healthHandler := handler.NewHealthHandler(pool, rdb)

	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.CustomRecovery(func(c *gin.Context, _ any) {
		api.Fail(c, http.StatusInternalServerError, api.CodeInternalError,
			"Internal server error")
	}))

	r.GET("/healthz", healthHandler.Healthz)

	v1 := r.Group("/api/v1")
	v1.Use(middleware.CSRF(cfg.FrontendOrigins))

	// Auth(spec 端點 1-5)
	v1.GET("/auth/google/login", authHandler.GoogleLogin)
	v1.GET("/auth/google/callback", authHandler.GoogleCallback)
	v1.POST("/auth/signup", authHandler.Signup)
	v1.POST("/auth/logout", authHandler.Logout)
	v1.GET("/me", auth.Required(), authHandler.Me)
	v1.PATCH("/me", auth.Required(), userHandler.PatchMe)    // 新增
	v1.GET("/users/:username", userHandler.GetPublicProfile) // 新增（純公開）

	if cfg.IsDev() {
		dev := handler.DevHandler{Users: userRepo}
		v1.GET("/dev/onboarding", dev.OnboardingPage)
		v1.GET("/dev/whoami", auth.Optional(), dev.WhoAmI)
		v1.POST("/dev/echo", dev.Echo)
	}

	log.Printf("listening on :%s (env=%s)", cfg.Port, cfg.AppEnv)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
