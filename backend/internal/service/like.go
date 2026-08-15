package service

import (
	"context"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

type LikeService struct {
	Likes *repository.LikeRepository
}

// Like 讓文章處於「已按讚」狀態（PUT，冪等）。
func (s *LikeService) Like(ctx context.Context, userID, postID string) (*repository.LikeState, error) {
	// 決策 #43：草稿 / 已刪 / 作者已刪 → ErrNotFound（handler 轉 404）
	if err := s.Likes.FindLikeablePost(ctx, postID); err != nil {
		return nil, err
	}
	if err := s.Likes.Like(ctx, userID, postID); err != nil {
		return nil, err
	}
	return s.Likes.GetState(ctx, postID, userID)
}

// Unlike 讓文章處於「未按讚」狀態（DELETE，冪等）。
func (s *LikeService) Unlike(ctx context.Context, userID, postID string) (*repository.LikeState, error) {
	if err := s.Likes.FindLikeablePost(ctx, postID); err != nil {
		return nil, err
	}
	if err := s.Likes.Unlike(ctx, userID, postID); err != nil {
		return nil, err
	}
	return s.Likes.GetState(ctx, postID, userID)
}
