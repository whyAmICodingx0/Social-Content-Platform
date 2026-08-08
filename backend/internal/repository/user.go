package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound:repository 層統一的「查無資料」錯誤。
// 上層用 errors.Is(err, repository.ErrNotFound) 判斷,
// 不需要知道底層是 pgx.ErrNoRows。
var ErrNotFound = errors.New("repository: not found")

// User 對應 users 資料表。
// 可為 NULL 的欄位用指標(*string):nil 就是資料庫的 NULL。
type User struct {
	ID          string
	Username    string
	Email       string
	DisplayName *string
	AvatarURL   *string
	Bio         *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// GetByID 只回傳「未刪除」的使用者。
// ★ 決策 #8 的落實處:deleted_at IS NULL 寫死在 repository 層,
//
//	上層(service / handler / middleware)永遠拿不到已刪資料,
//	不存在「忘記加條件」的可能。之後每個查 users / posts 的
//	repository 方法都必須遵守這個模式。
func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	const q = `
		SELECT id, username, email, display_name, avatar_url, bio,
		       created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	var u User
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Bio,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateProfileParams：nil 代表「這個欄位不更新」（PATCH 語意）。
type UpdateProfileParams struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

// UpdateProfile 部分更新個人檔案（spec 6.1）。
func (r *UserRepository) UpdateProfile(ctx context.Context, id string, p UpdateProfileParams) (*User, error) {
	const q = `
		UPDATE users SET
			display_name = COALESCE(NULLIF($2, ''), display_name),
			bio          = NULLIF(COALESCE($3, bio, ''), ''),
			avatar_url   = NULLIF(COALESCE($4, avatar_url, ''), ''),
			updated_at   = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, username, email, display_name, avatar_url, bio,
		          created_at, updated_at`

	var u User
	err := r.pool.QueryRow(ctx, q, id, p.DisplayName, p.Bio, p.AvatarURL).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Bio,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound // 使用者不存在或已軟刪
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByUsername 依 username 查未刪除的使用者（spec 6.2）。
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	const q = `
		SELECT id, username, email, display_name, avatar_url, bio,
		       created_at, updated_at
		FROM users
		WHERE lower(username) = lower($1) AND deleted_at IS NULL`

	var u User
	err := r.pool.QueryRow(ctx, q, username).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Bio,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
