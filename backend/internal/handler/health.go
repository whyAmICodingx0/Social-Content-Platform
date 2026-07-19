package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

func NewHealthHandler(pool *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{pool: pool, rdb: rdb}
}

func (h *HealthHandler) Healthz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	dbStatus, redisStatus := "connected", "connected"
	if err := h.pool.Ping(ctx); err != nil {
		dbStatus = "unreachable"
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		redisStatus = "unreachable"
	}

	if dbStatus != "connected" || redisStatus != "connected" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "error", "db": dbStatus, "redis": redisStatus,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "db": dbStatus, "redis": redisStatus})
}
