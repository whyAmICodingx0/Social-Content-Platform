package service

import (
	"context"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// maxCommentLen 決策 #46：1000 個 Unicode rune（不是 byte）
const maxCommentLen = 1000

type CommentService struct {
	Comments *repository.CommentRepository
	Notifier *Notifier // P4-1：留言通知
}

// List 列出某篇文章的留言。
// 先確認文章可留言——草稿一律 ErrNotFound（決策 #43），
// 對所有人一致，包括作者本人。
func (s *CommentService) List(ctx context.Context, postID string, limit, offset int) ([]*repository.Comment, int, error) {
	if _, err := s.Comments.FindCommentablePost(ctx, postID); err != nil {
		return nil, 0, err
	}
	return s.Comments.List(ctx, postID, limit, offset)
}

// Create 新增留言。
func (s *CommentService) Create(ctx context.Context, authorID, postID, rawContent string) (*repository.Comment, error) {
	content, err := validateCommentContent(rawContent)
	if err != nil {
		return nil, err
	}

	// FindCommentablePost 本來就回傳文章作者 id（決策 #89：不需要額外 SELECT）
	postAuthorID, err := s.Comments.FindCommentablePost(ctx, postID)
	if err != nil {
		return nil, err
	}

	cm, err := s.Comments.Create(ctx, postID, authorID, content)
	if err != nil {
		return nil, err
	}

	// ⚠️ entity_id 用 **comment_id 而非 post_id**（決策 #85）：
	// 同一人可在同一篇文章留多則留言，每則都該通知；
	// 用 post_id 會被去重索引擋掉，只有第一則會通知。
	if s.Notifier != nil {
		cid := cm.ID
		if nerr := s.Notifier.Notify(
			ctx, postAuthorID, authorID, repository.NotificationTypeComment, &cid,
		); nerr != nil {
			log.Printf("comment: notify failed (comment=%s): %v", cm.ID, nerr)
		}
	}

	return cm, nil
}

// Update 編輯留言。
// 【決策 #54】只有留言作者本人可以編輯——文章作者不行。
// 這與刪除權限刻意不對稱：移除不當內容是正當的，
// 修改別人的發言不是。
func (s *CommentService) Update(ctx context.Context, actorID, commentID, rawContent string) (*repository.Comment, error) {
	content, err := validateCommentContent(rawContent)
	if err != nil {
		return nil, err
	}

	c, _, err := s.Comments.GetForPermission(ctx, commentID)
	if err != nil {
		return nil, err
	}
	if c.AuthorID != actorID {
		return nil, ErrForbidden
	}
	return s.Comments.Update(ctx, commentID, content)
}

// Delete 刪除留言。
// 【決策 #54】留言作者 **或** 文章作者皆可刪。
func (s *CommentService) Delete(ctx context.Context, actorID, commentID string) error {
	c, target, err := s.Comments.GetForPermission(ctx, commentID)
	if err != nil {
		return err
	}
	if c.AuthorID != actorID && target.PostAuthorID != actorID {
		return ErrForbidden
	}
	return s.Comments.SoftDelete(ctx, commentID)
}

// validateCommentContent 決策 #46：
//  1. 先 trim，trim 後為空 → 400
//  2. 上限 1000 個 Unicode rune
//
// ⚠️ 必須用 utf8.RuneCountInString。
//
//	len(string) 與 Gin binding 的 max tag 算的都是 byte——
//	一個中文字 3 bytes，中文使用者會在 333 字就被擋下。
//	這與任務 G 的 bio 驗證是同一個坑。
func validateCommentContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)

	if content == "" {
		return "", &ValidationError{
			Field: "content", Message: "must not be empty",
		}
	}
	if utf8.RuneCountInString(content) > maxCommentLen {
		return "", &ValidationError{
			Field: "content", Message: "must be at most 1000 characters",
		}
	}
	return content, nil
}
