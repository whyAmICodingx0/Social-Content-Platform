package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 唯一性衝突的 typed errors——上層用 errors.Is 分支,
// 不需要知道 SQLSTATE 或 constraint 名稱這些底層細節。
var (
	ErrUsernameTaken      = errors.New("repository: username taken")
	ErrEmailTaken         = errors.New("repository: email taken")
	ErrOAuthAccountExists = errors.New("repository: oauth account exists")
	ErrUnexpectedConflict = errors.New("repository: unexpected unique violation")
)

type AuthRepository struct {
	pool *pgxpool.Pool
}

func NewAuthRepository(pool *pgxpool.Pool) *AuthRepository {
	return &AuthRepository{pool: pool}
}

// FindUserIDByOAuth:登入的核心查詢(spec 5.2 第 3 步)。
// oauth_accounts 沒有 soft delete,這裡不需要 deleted_at 條件;
// 「使用者是否已刪」由後續的 UserRepository.GetByID 把關。
func (r *AuthRepository) FindUserIDByOAuth(ctx context.Context, provider, providerUserID string) (string, error) {
	const q = `
		SELECT user_id FROM oauth_accounts
		WHERE provider = $1 AND provider_user_id = $2`

	var userID string
	err := r.pool.QueryRow(ctx, q, provider, providerUserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

type CreateUserWithOAuthParams struct {
	Username       string
	Email          string
	DisplayName    string // 已由 service 算好(不會是空字串)
	AvatarURL      string // 可能為空 → SQL 用 NULLIF 轉 NULL
	Provider       string
	ProviderUserID string
	ProviderEmail  string
}

// CreateUserWithOAuth:spec 5.3 第 4 步——
// 同一個 transaction 建 users + oauth_accounts,任何一步失敗全部回滾,
// 資料庫永遠不會出現「有 user 沒綁定」或反過來的半成品。
func (r *AuthRepository) CreateUserWithOAuth(ctx context.Context, p CreateUserWithOAuthParams) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// 【事實】defer Rollback 是 pgx 的標準寫法:
	// Commit 成功後再 Rollback 會回傳「交易已結束」錯誤,忽略即可;
	// 中途任何 return,都保證交易被回滾。
	defer tx.Rollback(ctx)

	const insertUser = `
		INSERT INTO users (username, email, display_name, avatar_url)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, username, email, display_name, avatar_url, bio,
		          created_at, updated_at`

	var u User
	err = tx.QueryRow(ctx, insertUser,
		p.Username, p.Email, p.DisplayName, p.AvatarURL,
	).Scan(
		&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.AvatarURL, &u.Bio,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, mapUniqueViolation(err)
	}

	const insertOAuth = `
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, email)
		VALUES ($1, $2, $3, $4)`

	if _, err := tx.Exec(ctx, insertOAuth,
		u.ID, p.Provider, p.ProviderUserID, p.ProviderEmail,
	); err != nil {
		return nil, mapUniqueViolation(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &u, nil
}

// mapUniqueViolation:把 PostgreSQL 的 unique_violation(SQLSTATE 23505)
// 依 constraint 名稱翻成 typed error。
func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_username_key":
			return ErrUsernameTaken
		case "users_email_key":
			return ErrEmailTaken
		case "oauth_accounts_provider_uid_key":
			return ErrOAuthAccountExists
		default:
			// 依修訂 5.3:未預期 constraint → 上層記 log 回 500。
			// 保留 constraint 名稱供 log 使用。
			return fmt.Errorf("%w (constraint %s)", ErrUnexpectedConflict, pgErr.ConstraintName)
		}
	}
	return err
}
