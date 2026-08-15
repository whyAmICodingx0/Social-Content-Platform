package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/middleware"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

type PostHandler struct {
	Svc *service.PostService
}

// viewerID 取出目前使用者 id；匿名時回 nil。
// ⚠️ 決策 #49：回 nil 而非 uuid.Nil —— pgx 會送出真正的 SQL NULL。
func viewerID(c *gin.Context) *string {
	if u, ok := middleware.CurrentUser(c); ok {
		return &u.ID
	}
	return nil
}

// ---------- POST /api/v1/posts ----------

type createPostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Status  *string  `json:"status"`
	Tags    []string `json:"tags"`
}

func (h *PostHandler) Create(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req createPostRequest
	if !api.BindStrict(c, &req) {
		return
	}

	post, err := h.Svc.Create(c.Request.Context(), u.ID, service.CreatePostInput{
		Title: req.Title, Content: req.Content, Status: req.Status, Tags: req.Tags,
	})
	if err != nil {
		h.failPost(c, err)
		return
	}

	post.AuthorUsername = u.Username
	post.AuthorDisplayName = u.DisplayName
	post.AuthorAvatarURL = u.AvatarURL

	api.Created(c, postDetailJSON(post))
}

// ---------- GET /api/v1/users/:username/posts/:slug（optional auth） ----------

func (h *PostHandler) GetBySlug(c *gin.Context) {
	post, err := h.Svc.GetForReader(c.Request.Context(),
		c.Param("username"), c.Param("slug"), viewerID(c))
	if err != nil {
		h.failPost(c, err)
		return
	}
	api.OK(c, postDetailJSON(post))
}

// ---------- PATCH /api/v1/posts/:id ----------

type updatePostRequest struct {
	Title   *string  `json:"title"`
	Content *string  `json:"content"`
	Status  *string  `json:"status"`
	Tags    []string `json:"tags"`
}

func (h *PostHandler) Update(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}

	var req updatePostRequest
	if !api.BindStrict(c, &req) {
		return
	}

	post, err := h.Svc.Update(c.Request.Context(), u.ID, c.Param("id"), service.UpdatePostInput{
		Title:   req.Title,
		Content: req.Content,
		Status:  req.Status,
		Tags:    req.Tags,
		HasTags: req.Tags != nil,
	})
	if err != nil {
		h.failPost(c, err)
		return
	}

	post.AuthorUsername = u.Username
	post.AuthorDisplayName = u.DisplayName
	post.AuthorAvatarURL = u.AvatarURL

	api.OK(c, postDetailJSON(post))
}

// ---------- DELETE /api/v1/posts/:id ----------

func (h *PostHandler) Delete(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), u.ID, c.Param("id")); err != nil {
		h.failPost(c, err)
		return
	}
	api.NoContent(c)
}

// ---------- 列表 ----------

// GET /api/v1/posts（optional auth，決策 #47）
func (h *PostHandler) ListPublic(c *gin.Context) {
	q := api.ParsePageQuery(c)

	in := service.ListPostsInput{
		OnlyPublished:    true,
		OrderByPublished: true,
		Asc:              q.Asc(),
		Limit:            q.Limit,
		Offset:           q.Offset(),
		ViewerID:         viewerID(c),
	}
	if tag := c.Query("tag"); tag != "" {
		in.Tag = &tag
	}

	h.respondList(c, q, in)
}

// GET /api/v1/users/:username/posts（optional auth，決策 #47）
func (h *PostHandler) ListByUser(c *gin.Context) {
	q := api.ParsePageQuery(c)
	username := c.Param("username")

	h.respondList(c, q, service.ListPostsInput{
		AuthorName:       &username,
		OnlyPublished:    true,
		OrderByPublished: true,
		Asc:              q.Asc(),
		Limit:            q.Limit,
		Offset:           q.Offset(),
		ViewerID:         viewerID(c),
	})
}

// GET /api/v1/me/posts（required auth）
func (h *PostHandler) ListMine(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	q := api.ParsePageQuery(c)

	in := service.ListPostsInput{
		AuthorID:         &u.ID,
		OrderByPublished: false,
		Asc:              q.Asc(),
		Limit:            q.Limit,
		Offset:           q.Offset(),
		ViewerID:         &u.ID,
	}
	if st := c.Query("status"); st == "draft" || st == "published" {
		in.Status = &st
	}

	h.respondList(c, q, in)
}

// GET /api/v1/feed（required auth，決策 #45）
//
// 內容：我追蹤的人 + 我自己 的已發布文章。
// 空時回空陣列，不 fallback 全站文章——那會讓同一端點回傳語意
// 不同的資料，前端無法分辨。空狀態由前端呈現。
func (h *PostHandler) Feed(c *gin.Context) {
	u, ok := middleware.CurrentUser(c)
	if !ok {
		api.Fail(c, http.StatusUnauthorized, api.CodeUnauthenticated, "Authentication required")
		return
	}
	q := api.ParsePageQuery(c)

	h.respondList(c, q, service.ListPostsInput{
		FeedFor:          &u.ID,
		OnlyPublished:    true,
		OrderByPublished: true,
		Asc:              q.Asc(),
		Limit:            q.Limit,
		Offset:           q.Offset(),
		ViewerID:         &u.ID, // 自己的 feed，liked_by_me 一定是自己的視角
	})
}

func (h *PostHandler) respondList(c *gin.Context, q api.PageQuery, in service.ListPostsInput) {
	posts, total, err := h.Svc.List(c.Request.Context(), in)
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	items := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		items = append(items, postSummaryJSON(p))
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

// ---------- 共用 ----------

func (h *PostHandler) failPost(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "post not found")
	case errors.Is(err, service.ErrForbidden):
		api.Fail(c, http.StatusForbidden, api.CodeForbidden, "you are not the author of this post")
	case errors.Is(err, service.ErrSlugExhausted):
		api.Fail(c, http.StatusConflict, api.CodeSlugConflict, "could not generate a unique slug")
	default:
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// postDetailJSON：Post Detail shape（含 Phase 2 計數欄位）
func postDetailJSON(p *repository.Post) gin.H {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	var publishedAt any
	if p.PublishedAt != nil {
		publishedAt = p.PublishedAt.UTC()
	}
	return gin.H{
		"id":      p.ID,
		"slug":    p.Slug,
		"title":   p.Title,
		"content": p.ContentMD,
		"excerpt": p.Excerpt,
		"status":  p.Status,
		"author": gin.H{
			"username":     p.AuthorUsername,
			"display_name": p.AuthorDisplayName,
			"avatar_url":   p.AuthorAvatarURL,
		},
		"tags":          tags,
		"created_at":    p.CreatedAt.UTC(),
		"updated_at":    p.UpdatedAt.UTC(),
		"published_at":  publishedAt,
		"like_count":    p.LikeCount,
		"comment_count": p.CommentCount,
		"liked_by_me":   p.LikedByMe,
	}
}

// postSummaryJSON：列表用（不含 content 與 updated_at）
func postSummaryJSON(p *repository.Post) gin.H {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	var publishedAt any
	if p.PublishedAt != nil {
		publishedAt = p.PublishedAt.UTC()
	}
	return gin.H{
		"id":      p.ID,
		"slug":    p.Slug,
		"title":   p.Title,
		"excerpt": p.Excerpt,
		"status":  p.Status,
		"author": gin.H{
			"username":     p.AuthorUsername,
			"display_name": p.AuthorDisplayName,
			"avatar_url":   p.AuthorAvatarURL,
		},
		"tags":          tags,
		"created_at":    p.CreatedAt.UTC(),
		"published_at":  publishedAt,
		"like_count":    p.LikeCount,
		"comment_count": p.CommentCount,
		"liked_by_me":   p.LikedByMe,
	}
}
