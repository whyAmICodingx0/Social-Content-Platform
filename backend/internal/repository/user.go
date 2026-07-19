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
