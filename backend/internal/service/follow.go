package service

import (
	"context"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

type FollowService struct {
	Follows *repository.FollowRepository
	Users   *repository.UserRepository
}

// Follow 讓「我」處於追蹤某人的狀態（PUT，冪等）。
func (s *FollowService) Follow(ctx context.Context, actorID, targetUsername string) (*repository.FollowState, error) {
	target, err := s.Users.GetByUsername(ctx, targetUsername) // 內含 deleted_at IS NULL
	if err != nil {
		return nil, err // ErrNotFound → handler 轉 404
	}

	// 應用層擋下追蹤自己，給可讀的錯誤訊息。
	// DB 的 CHECK (follower_id <> followee_id) 是最終防線——
	// 兩層都要有：DB 保證正確性，應用層提供好訊息（同決策 #28 的原則）。
	if target.ID == actorID {
		return nil, &ValidationError{
			Field: "username", Message: "you cannot follow yourself",
		}
	}

	if err := s.Follows.Follow(ctx, actorID, target.ID); err != nil {
		return nil, err
	}
	return s.Follows.GetState(ctx, target.ID, &actorID)
}

// Unfollow 讓「我」處於未追蹤某人的狀態（DELETE，冪等）。
func (s *FollowService) Unfollow(ctx context.Context, actorID, targetUsername string) (*repository.FollowState, error) {
	target, err := s.Users.GetByUsername(ctx, targetUsername)
	if err != nil {
		return nil, err
	}
	// 取消追蹤自己不是錯誤，只是無事發生 —— 不需要特別擋

	if err := s.Follows.Unfollow(ctx, actorID, target.ID); err != nil {
		return nil, err
	}
	return s.Follows.GetState(ctx, target.ID, &actorID)
}

// ProfileExtras 是公開個人頁需要的追蹤相關資料。
type ProfileExtras struct {
	FollowerCount  int
	FollowingCount int
	FollowedByMe   bool
}

// GetProfileExtras 取得個人頁的追蹤數字與狀態。
// viewerID 為 nil（匿名）時 FollowedByMe 為 false（決策 #48）。
func (s *FollowService) GetProfileExtras(ctx context.Context, userID string, viewerID *string) (*ProfileExtras, error) {
	counts, err := s.Follows.GetCounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	st, err := s.Follows.GetState(ctx, userID, viewerID)
	if err != nil {
		return nil, err
	}

	return &ProfileExtras{
		FollowerCount:  counts.FollowerCount,
		FollowingCount: counts.FollowingCount,
		FollowedByMe:   st.FollowedByMe,
	}, nil
}

// ListFollowers / ListFollowing 供選做端點使用。
func (s *FollowService) ListFollowers(ctx context.Context, username string, limit, offset int) ([]*repository.FollowUser, int, error) {
	u, err := s.Users.GetByUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	return s.Follows.ListFollowers(ctx, u.ID, limit, offset)
}

func (s *FollowService) ListFollowing(ctx context.Context, username string, limit, offset int) ([]*repository.FollowUser, int, error) {
	u, err := s.Users.GetByUsername(ctx, username)
	if err != nil {
		return nil, 0, err
	}
	return s.Follows.ListFollowing(ctx, u.ID, limit, offset)
}
