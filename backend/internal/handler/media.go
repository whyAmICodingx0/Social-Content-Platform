package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

type MediaHandler struct {
	Svc *service.MediaService
}

// ---------- POST /api/v1/media/signature ----------

type signatureRequest struct {
	Purpose string `json:"purpose"`
}

func (h *MediaHandler) Signature(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req signatureRequest
	if !api.BindStrict(c, &req) {
		return
	}

	t, err := h.Svc.IssueTicket(c.Request.Context(), u.ID, req.Purpose)
	if err != nil {
		h.fail(c, err)
		return
	}

	api.OK(c, gin.H{
		"upload_url":    t.UploadURL,
		"resource_type": t.ResourceType,
		"max_bytes":     t.MaxBytes,
		"fields":        t.Fields,
	})
}

// ---------- POST /api/v1/media ----------

type reportMediaRequest struct {
	Purpose  string `json:"purpose"`
	AssetID  string `json:"asset_id"`
	PublicID string `json:"public_id"`
}

func (h *MediaHandler) Report(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req reportMediaRequest
	if !api.BindStrict(c, &req) {
		return
	}

	m, existing, err := h.Svc.Report(c.Request.Context(), u.ID, service.ReportInput{
		Purpose:  req.Purpose,
		AssetID:  req.AssetID,
		PublicID: req.PublicID,
	})
	if err != nil {
		h.fail(c, err)
		return
	}

	if existing {
		api.OK(c, mediaJSON(m)) // 冪等重送（決策 #117）
		return
	}
	api.Created(c, mediaJSON(m))
}

// ---------- 共用 ----------

func (h *MediaHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	var rej *service.UploadRejectedError

	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.As(err, &rej):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Upload rejected", map[string]string{"upload": rej.Reason})
	case errors.Is(err, service.ErrUploadNotFound):
		// 涵蓋「不存在」與「不屬於本人」，刻意不區分
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Upload not found", map[string]string{"upload": "upload not found"})
	case errors.Is(err, service.ErrQuotaExceeded):
		api.Fail(c, http.StatusTooManyRequests, api.CodeRateLimited,
			"daily upload limit reached")
	default:
		// 決策 #80：default 必須 log。ErrProviderUnavailable 也走這條（503）。
		log.Printf("media handler: unexpected error: %v", err)
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// mediaJSON：Media shape。
// 不輸出 status（P5 一律 ready，輸出會讓前端以為要處理其他狀態）、
// 不輸出 provider 的識別碼與 owner_id。
func mediaJSON(m *repository.Media) gin.H {
	return gin.H{
		"id":            m.ID,
		"purpose":       m.Purpose,
		"url":           m.URL,
		"thumbnail_url": m.ThumbnailURL,
		"width":         m.Width,
		"height":        m.Height,
		"bytes":         m.Bytes,
		"duration":      m.Duration,
		"created_at":    m.CreatedAt.UTC(),
	}
}
