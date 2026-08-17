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
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/ws"
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

	followRepo := repository.NewFollowRepository(pool)
	followSvc := &service.FollowService{Follows: followRepo, Users: userRepo}
	followHandler := &handler.FollowHandler{Svc: followSvc}

	hub := ws.NewHub()
	wsHandler := &handler.WSHandler{
		Hub:       hub,
		Upgrader:  ws.NewUpgrader(cfg.FrontendOrigins),
		Validator: &ws.RedisSessionValidator{Store: sessions},
	}

	conversationRepo := repository.NewConversationRepository(pool)
	messageRepo := repository.NewMessageRepository(pool)

	conversationSvc := &service.ConversationService{
		Conversations: conversationRepo,
		Messages:      messageRepo, // P3-4：驗證已讀錨點
		Users:         userRepo,
	}
	messageSvc := &service.MessageService{
		Messages:      messageRepo,
		Conversations: conversationRepo,
	}

	conversationHandler := &handler.ConversationHandler{Svc: conversationSvc}
	messageHandler := &handler.MessageHandler{
		Svc:      messageSvc,
		Notifier: &ws.Notifier{Hub: hub},
	}

	userSvc := &service.UserService{Users: userRepo}
	userHandler := &handler.UserHandler{Svc: userSvc, Follows: followSvc}

	postRepo := repository.NewPostRepository(pool)
	postSvc := &service.PostService{Posts: postRepo}
	postHandler := &handler.PostHandler{Svc: postSvc}

	likeRepo := repository.NewLikeRepository(pool)
	likeSvc := &service.LikeService{Likes: likeRepo}
	likeHandler := &handler.LikeHandler{Svc: likeSvc}

	commentRepo := repository.NewCommentRepository(pool)
	commentSvc := &service.CommentService{Comments: commentRepo}
	commentHandler := &handler.CommentHandler{Svc: commentSvc}

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
		Hub:      hub, // P3-1：登出時關閉同 sid 的 WS 連線（決策 #77）
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
	v1.PATCH("/me", auth.Required(), userHandler.PatchMe)                     // 新增
	v1.GET("/users/:username", auth.Optional(), userHandler.GetPublicProfile) // 新增（純公開）
	v1.POST("/posts", auth.Required(), postHandler.Create)
	v1.PATCH("/posts/:id", auth.Required(), postHandler.Update)
	v1.DELETE("/posts/:id", auth.Required(), postHandler.Delete)
	v1.GET("/users/:username/posts/:slug", auth.Optional(), postHandler.GetBySlug)
	v1.GET("/posts", auth.Optional(), postHandler.ListPublic)
	v1.GET("/me/posts", auth.Required(), postHandler.ListMine)
	v1.GET("/feed", auth.Required(), postHandler.Feed)
	v1.GET("/users/:username/posts", auth.Optional(), postHandler.ListByUser)
	v1.GET("/tags", tagHandler.List)
	v1.PUT("/posts/:id/like", auth.Required(), likeHandler.Like)
	v1.DELETE("/posts/:id/like", auth.Required(), likeHandler.Unlike)
	// Comments（P2-2）
	v1.GET("/posts/:id/comments", commentHandler.List) // 純公開
	v1.POST("/posts/:id/comments", auth.Required(), commentHandler.Create)
	v1.PATCH("/comments/:id", auth.Required(), commentHandler.Update)
	v1.DELETE("/comments/:id", auth.Required(), commentHandler.Delete)
	// Follows（P2-3）
	v1.PUT("/users/:username/follow", auth.Required(), followHandler.Follow)
	v1.DELETE("/users/:username/follow", auth.Required(), followHandler.Unfollow)
	v1.GET("/users/:username/followers", followHandler.ListFollowers)
	v1.GET("/users/:username/following", followHandler.ListFollowing)
	// WebSocket（P3-1）
	v1.GET("/ws", auth.Required(), wsHandler.Serve)
	// Messaging（P3-2）
	v1.POST("/conversations", auth.Required(), conversationHandler.Create)
	v1.POST("/conversations/:id/messages", auth.Required(), messageHandler.Create)
	v1.GET("/conversations", auth.Required(), conversationHandler.List)
	v1.GET("/conversations/:id/messages", auth.Required(), messageHandler.List)
	// ⚠️ /conversations/unread-count 必須在 /conversations/:id 之前註冊？
	//    不需要 —— 這兩條路徑長度相同但 Gin 會優先比對靜態片段，
	//    unread-count 不會被當成 :id。但目前沒有 GET /conversations/:id，
	//    所以無論如何都不衝突。
	v1.GET("/conversations/unread-count", auth.Required(), conversationHandler.UnreadCount)
	v1.POST("/conversations/:id/read", auth.Required(), conversationHandler.MarkRead)

	// Dev only（P3-1 驗收用，正式環境不註冊）
	if cfg.IsDev() {
		v1.GET("/dev/ws-stats", auth.Required(), wsHandler.Stats)
	}

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
