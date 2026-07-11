package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://app:devpassword@localhost:5432/social_dev?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("無法建立資料庫連線池: %v", err)
	}
	defer pool.Close()
	db = pool

	r := gin.Default()

	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			c.JSON(500, gin.H{"status": "error", "db": "unreachable"})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "db": "connected"})
	})

	r.Run(":8080")
}
