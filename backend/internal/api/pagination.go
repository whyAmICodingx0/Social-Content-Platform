package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

type PageQuery struct {
	Page  int
	Limit int
	Sort  string
}

func (q PageQuery) Offset() int { return (q.Page - 1) * q.Limit }

func (q PageQuery) Asc() bool { return q.Sort == "oldest" }

func ParsePageQuery(c *gin.Context) PageQuery {
	q := PageQuery{Page: DefaultPage, Limit: DefaultLimit, Sort: "newest"}

	if v, err := strconv.Atoi(c.Query("page")); err == nil && v >= 1 {
		q.Page = v
	}
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v >= 1 {
		if v > MaxLimit {
			v = MaxLimit
		}
		q.Limit = v
	}
	if s := c.Query("sort"); s == "oldest" {
		q.Sort = s
	}
	return q
}

func NewPagination(q PageQuery, total int) Pagination {
	totalPages := 0
	if total > 0 {
		totalPages = (total + q.Limit - 1) / q.Limit
	}
	return Pagination{
		Page:       q.Page,
		Limit:      q.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    q.Page < totalPages,
	}
}
