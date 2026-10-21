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

// GetByID 以 media.id 查詢（P5-2：驗證 avatar_media_id）。
//
// ⚠️ 呼叫端必須先確認 id 是合法的 UUID。否則 PostgreSQL 會回 22P02
// （invalid input syntax for type uuid），那會落到 handler 的 default 分支變成 503，
// 而不是「使用者輸入錯誤」該有的 400。
func (r *MediaRepository) GetByID(ctx context.Context, id string) (*Media, error) {
	const q = `SELECT ` + mediaColumns + ` FROM media WHERE id = $1`
	return scanMedia(r.pool.QueryRow(ctx, q, id))
}

// GetByIDs 一次查多筆（P5-3：驗證 media_ids）。找不到的 id 不會出現在結果裡，
// 由呼叫端比對數量與內容。
//
// ⚠️ 同 GetByID：每個 id 都必須先確認是合法的 UUID。
func (r *MediaRepository) GetByIDs(ctx context.Context, ids []string) ([]*Media, error) {
	const q = `SELECT ` + mediaColumns + ` FROM media WHERE id = ANY($1::uuid[])`

	rows, err := r.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*Media{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
