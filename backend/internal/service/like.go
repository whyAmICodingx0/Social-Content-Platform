package service

import (
	"context"
	"log"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

type LikeService struct {
	Likes    *repository.LikeRepository
	Notifier *Notifier // P4-1：按讚通知
}

// Like 讓文章處於「已按讚」狀態（PUT，冪等）。
func (s *LikeService) Like(ctx context.Context, userID, postID string) (*repository.LikeState, error) {
	// 決策 #43：草稿 / 已刪 / 作者已刪 → ErrNotFound（handler 轉 404）
	// authorID 供通知使用（決策 #89：不需要額外 SELECT）
	authorID, err := s.Likes.FindLikeablePost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if err := s.Likes.Like(ctx, userID, postID); err != nil {
		return nil, err
	}

	state, err := s.Likes.GetState(ctx, postID, userID)
	if err != nil {
		return nil, err
	}

	// 通知：主操作已完成，這是次要副作用（決策 #89）。
	// 同步呼叫、失敗只記 log、不影響回傳值。
	if s.Notifier != nil {
		pid := postID
		if nerr := s.Notifier.Notify(
			ctx, authorID, userID, repository.NotificationTypeLike, &pid,
		); nerr != nil {
			log.Printf("like: notify failed (post=%s): %v", postID, nerr)
		}
	}

	return state, nil
}

// Unlike 讓文章處於「未按讚」狀態（DELETE，冪等）。
func (s *LikeService) Unlike(ctx context.Context, userID, postID string) (*repository.LikeState, error) {
	if _, err := s.Likes.FindLikeablePost(ctx, postID); err != nil {
		return nil, err
	}
	if err := s.Likes.Unlike(ctx, userID, postID); err != nil {
		return nil, err
	}
	return s.Likes.GetState(ctx, postID, userID)
}
