package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

// SearchHandler
//
// ⚠️ 欄位名刻意是 PostSvc / UserSvc 而非 Posts / Users ——
// Go 不允許 struct 的欄位與方法同名，若欄位叫 Posts 而方法也叫 Posts，
// 會直接編譯失敗：
//
//	type SearchHandler has both field and method named Posts
type SearchHandler struct {
	PostSvc *service.PostService
	UserSvc *service.UserService
}

// ---------- GET /api/v1/search/posts（optional auth） ----------
//
// optional auth 是因為回應含 liked_by_me（決策 #47）。
// viewerID(c) 在匿名時回 nil，pgx 送出真正的 SQL NULL（決策 #49）。
func (h *SearchHandler) SearchPosts(c *gin.Context) {
	terms, err := service.ParseSearchQuery(c.Query("q"))
	if err != nil {
		h.fail(c, err)
		return
	}

	q := api.ParsePageQuery(c)

	posts, total, err := h.PostSvc.List(c.Request.Context(), service.ListPostsInput{
		SearchPattern: &terms.Pattern,
		SearchQuery:   &terms.Query,

		// ⚠️ 必須顯式帶 OnlyPublished —— 同一個 List 也服務 /me/posts（要看草稿），
		// 不可依賴任何預設值，往哪邊倒都會有一邊出錯。
		OnlyPublished: true,

		Limit:    q.Limit,
		Offset:   q.Offset(),
		ViewerID: viewerID(c),

		// 排序由搜尋的相關性公式接管（repository.List 的 SearchPattern 分支），
		// 這裡不需要設 OrderByPublished / Asc。
	})
	if err != nil {
		h.fail(c, err)
		return
	}

	items := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		items = append(items, postSummaryJSON(p))
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

// ---------- GET /api/v1/search/users（純公開） ----------
//
// 回應不含任何個人化欄位，故不需要 optional auth（同 GET /posts/{id}/comments
// 的判斷）。這是刻意的，不是漏改。
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	terms, err := service.ParseSearchQuery(c.Query("q"))
	if err != nil {
		h.fail(c, err)
		return
	}

	q := api.ParsePageQuery(c)

	users, total, err := h.UserSvc.SearchUsers(c.Request.Context(), terms, q.Limit, q.Offset())
	if err != nil {
		h.fail(c, err)
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, searchUserJSON(u))
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

// ---------- 共用 ----------

func (h *SearchHandler) fail(c *gin.Context, err error) {
	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		api.FailWithFields(c, http.StatusBadRequest, api.CodeValidationError,
			"Request validation failed", map[string]string{vErr.Field: vErr.Message})
	case errors.Is(err, repository.ErrNotFound):
		api.Fail(c, http.StatusNotFound, api.CodeNotFound, "not found")
	default:
		// 決策 #80：default 的定義就是「非預期的錯誤」，不 log 等於沒有線索
		log.Printf("search handler: unexpected error: %v", err)
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
	}
}

// searchUserJSON：與 FollowUser 相同的形狀，前端可重用列表元件
func searchUserJSON(u *repository.SearchUser) gin.H {
	return gin.H{
		"id":           u.ID,
		"username":     u.Username,
		"display_name": u.DisplayName,
		"avatar_url":   u.AvatarURL,
		"bio":          u.Bio,
	}
}
