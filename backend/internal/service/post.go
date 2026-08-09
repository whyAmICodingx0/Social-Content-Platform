package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
)

const (
	maxTitleLen   = 200
	maxExcerptLen = 200
)

var (
	ErrForbidden     = errors.New("service: forbidden")
	ErrSlugExhausted = errors.New("service: slug attempts exhausted")
)

type PostService struct {
	Posts *repository.PostRepository
}

type CreatePostInput struct {
	Title   string
	Content string
	Status  *string
	Tags    []string
}

func (s *PostService) Create(ctx context.Context, authorID string, in CreatePostInput) (*repository.Post, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		return nil, &ValidationError{Field: "title", Message: "must be 1-200 characters"}
	}
	if strings.TrimSpace(in.Content) == "" {
		return nil, &ValidationError{Field: "content", Message: "must not be empty"}
	}

	status := "draft"
	if in.Status != nil {
		status = *in.Status
	}
	if status != "draft" && status != "published" {
		return nil, &ValidationError{Field: "status", Message: `must be "draft" or "published"`}
	}

	tags, err := normalizeTags(in.Tags)
	if err != nil {
		return nil, err
	}

	var publishedAt *time.Time
	if status == "published" {
		now := time.Now().UTC()
		publishedAt = &now
	}

	base, err := makeBaseSlug(title)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt <= slugMaxAttempts; attempt++ {
		slug, err := candidateSlug(base, attempt)
		if err != nil {
			return nil, err
		}

		post, err := s.Posts.Create(ctx, repository.CreatePostParams{
			AuthorID:    authorID,
			Title:       title,
			Slug:        slug,
			ContentMD:   in.Content,
			Excerpt:     makeExcerpt(in.Content),
			Status:      status,
			PublishedAt: publishedAt,
			Tags:        tags,
		})
		if err == nil {
			return post, nil
		}
		if errors.Is(err, repository.ErrSlugTaken) {
			continue
		}
		return nil, err
	}
	return nil, ErrSlugExhausted
}

type UpdatePostInput struct {
	Title   *string
	Content *string
	Status  *string
	Tags    []string
	HasTags bool
}

func (s *PostService) Update(ctx context.Context, actorID, postID string, in UpdatePostInput) (*repository.Post, error) {
	existing, err := s.Posts.GetByID(ctx, postID)
	if err != nil {
		return nil, err
	}
	if existing.AuthorID != actorID {
		return nil, ErrForbidden
	}

	p := repository.UpdatePostParams{ReplaceTags: in.HasTags}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" || utf8.RuneCountInString(t) > maxTitleLen {
			return nil, &ValidationError{Field: "title", Message: "must be 1-200 characters"}
		}
		p.Title = &t
	}

	if in.Content != nil {
		c := *in.Content
		if strings.TrimSpace(c) == "" {
			return nil, &ValidationError{Field: "content", Message: "must not be empty"}
		}
		p.ContentMD = &c
		ex := makeExcerpt(c)
		p.Excerpt = &ex
	}

	if in.Status != nil {
		st := *in.Status
		if st != "draft" && st != "published" {
			return nil, &ValidationError{Field: "status", Message: `must be "draft" or "published"`}
		}
		p.Status = &st
		if st == "published" && existing.PublishedAt == nil {
			now := time.Now().UTC()
			p.PublishedAt = &now
		}
	}

	if in.HasTags {
		tags, err := normalizeTags(in.Tags)
		if err != nil {
			return nil, err
		}
		p.Tags = tags
	}

	return s.Posts.Update(ctx, postID, p)
}

func (s *PostService) Delete(ctx context.Context, actorID, postID string) error {
	existing, err := s.Posts.GetByID(ctx, postID)
	if err != nil {
		return err
	}
	if existing.AuthorID != actorID {
		return ErrForbidden
	}
	return s.Posts.SoftDelete(ctx, postID)
}

func (s *PostService) GetForReader(ctx context.Context, username, slug, viewerID string) (*repository.Post, error) {
	p, err := s.Posts.GetByAuthorAndSlug(ctx, username, slug)
	if err != nil {
		return nil, err
	}
	if p.Status == "draft" && p.AuthorID != viewerID {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func makeExcerpt(md string) string {
	replacer := strings.NewReplacer(
		"#", "", "*", "", "_", "", "`", "", ">", "",
		"[", "", "]", "", "\n", " ", "\r", " ",
	)
	s := replacer.Replace(md)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxExcerptLen {
		runes := []rune(s)
		s = string(runes[:maxExcerptLen])
	}
	return s
}

type ListPostsInput struct {
	AuthorID         *string
	AuthorName       *string
	Status           *string
	OnlyPublished    bool
	Tag              *string
	OrderByPublished bool
	Asc              bool
	Limit            int
	Offset           int
}

func (s *PostService) List(ctx context.Context, in ListPostsInput) ([]*repository.Post, int, error) {
	return s.Posts.List(ctx, repository.ListParams{
		AuthorID:         in.AuthorID,
		AuthorName:       in.AuthorName,
		Status:           in.Status,
		OnlyPublished:    in.OnlyPublished,
		Tag:              in.Tag,
		OrderByPublished: in.OrderByPublished,
		Asc:              in.Asc,
		Limit:            in.Limit,
		Offset:           in.Offset,
	})
}
