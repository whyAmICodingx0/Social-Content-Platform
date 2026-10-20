package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrMediaDuplicate：撞到 media_provider_asset_key（決策 #117 的冪等收斂用）
var ErrMediaDuplicate = errors.New("repository: media already exists")

type Media struct {
	ID               string
	OwnerID          string
	ProviderAssetID  string
	ProviderPublicID string
	Purpose          string
	Status           string
	URL              string
	ThumbnailURL     *string
	Format           string
	Bytes            int64
	Width            *int
	Height           *int
	Duration         *float64
	CreatedAt        time.Time
}

type MediaRepository struct {
	pool *pgxpool.Pool
}

func NewMediaRepository(pool *pgxpool.Pool) *MediaRepository {
	return &MediaRepository{pool: pool}
}

// duration 是 REAL（float4）。轉成 float8 再掃，避免 float4 → float64 的掃描相容性疑慮。
const mediaColumns = `
	id, owner_id, provider_asset_id, provider_public_id, purpose, status,
	url, thumbnail_url, format, bytes, width, height, duration::float8, created_at`

func scanMedia(row pgx.Row) (*Media, error) {
	var m Media
	err := row.Scan(
		&m.ID, &m.OwnerID, &m.ProviderAssetID, &m.ProviderPublicID, &m.Purpose, &m.Status,
		&m.URL, &m.ThumbnailURL, &m.Format, &m.Bytes, &m.Width, &m.Height, &m.Duration,
		&m.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 寫入一筆 media。
// 撞到 provider_asset_id 的 UNIQUE → ErrMediaDuplicate，由 service 收斂成 200。
// 單一 statement、無顯式交易，失敗後連線可繼續使用（同 P3-2 的訊息冪等）。
func (r *MediaRepository) Create(ctx context.Context, m *Media) (*Media, error) {
	const q = `
		INSERT INTO media (
			owner_id, provider_asset_id, provider_public_id, purpose, status,
			url, thumbnail_url, format, bytes, width, height, duration
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING ` + mediaColumns

	out, err := scanMedia(r.pool.QueryRow(ctx, q,
		m.OwnerID, m.ProviderAssetID, m.ProviderPublicID, m.Purpose, m.Status,
		m.URL, m.ThumbnailURL, m.Format, m.Bytes, m.Width, m.Height, m.Duration,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "media_provider_asset_key" {
			return nil, ErrMediaDuplicate
		}
		return nil, err
	}
	return out, nil
}

// GetByAssetID 以不可變的 asset_id 查詢（決策 #113）。
func (r *MediaRepository) GetByAssetID(ctx context.Context, assetID string) (*Media, error) {
	const q = `SELECT ` + mediaColumns + ` FROM media WHERE provider_asset_id = $1`
	return scanMedia(r.pool.QueryRow(ctx, q, assetID))
}
