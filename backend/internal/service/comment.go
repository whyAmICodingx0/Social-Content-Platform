package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

// maxCommentLen 決策 #46：1000 個 Unicode rune（不是 byte）
const maxCommentLen = 1000

type CommentService struct {
	Comments *repository.CommentRepository
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
	if _, err := s.Comments.FindCommentablePost(ctx, postID); err != nil {
		return nil, err
	}
	return s.Comments.Create(ctx, postID, authorID, content)
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
