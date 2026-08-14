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
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/web"
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

	rdb, err := store.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
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

	postRepo := repository.NewPostRepository(pool)
	postSvc := &service.PostService{Posts: postRepo}
	postHandler := &handler.PostHandler{Svc: postSvc}

	tagRepo := repository.NewTagRepository(pool)
	tagSvc := &service.TagService{Tags: tagRepo}
	tagHandler := &handler.TagHandler{Svc: tagSvc}

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
	v1.POST("/posts", auth.Required(), postHandler.Create)
	v1.PATCH("/posts/:id", auth.Required(), postHandler.Update)
	v1.DELETE("/posts/:id", auth.Required(), postHandler.Delete)
	v1.GET("/users/:username/posts/:slug", auth.Optional(), postHandler.GetBySlug)
	v1.GET("/posts", postHandler.ListPublic)
	v1.GET("/me/posts", auth.Required(), postHandler.ListMine)
	v1.GET("/users/:username/posts", postHandler.ListByUser)
	v1.GET("/tags", tagHandler.List)

	if web.Available() {
		if err := web.Register(r); err != nil {
			log.Fatalf("web: %v", err)
		}
		log.Println("serving embedded frontend")
	} else {
		log.Println("no embedded frontend (dev mode — use vite dev server)")
	}

	log.Printf("listening on :%s (env=%s)", cfg.Port, cfg.AppEnv)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
