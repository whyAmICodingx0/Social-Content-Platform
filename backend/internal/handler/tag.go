package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/whyAmICodingx0/Social-Content-Platform/internal/api"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/repository"
	"github.com/whyAmICodingx0/Social-Content-Platform/internal/service"
)

type TagHandler struct {
	Svc *service.TagService
}

func (h *TagHandler) List(c *gin.Context) {
	q := api.ParsePageQuery(c)

	sortBy := "popular"
	if s := c.Query("sort"); s == "name" {
		sortBy = s
	}

	tags, total, err := h.Svc.List(c.Request.Context(), sortBy, q.Limit, q.Offset())
	if err != nil {
		api.Fail(c, http.StatusServiceUnavailable, api.CodeServiceUnavailable,
			"Service temporarily unavailable")
		return
	}

	items := make([]gin.H, 0, len(tags))
	for _, t := range tags {
		items = append(items, tagJSON(t))
	}
	api.OKList(c, items, api.NewPagination(q, total))
}

func tagJSON(t *repository.Tag) gin.H {
	return gin.H{
		"id":         t.ID,
		"name":       t.Name,
		"slug":       t.Slug,
		"post_count": t.PostCount,
	}
}
