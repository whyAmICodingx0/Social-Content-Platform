package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Tag struct {
	ID        string
	Name      string
	Slug      string
	PostCount int
}

type TagRepository struct {
	pool *pgxpool.Pool
}

func NewTagRepository(pool *pgxpool.Pool) *TagRepository {
	return &TagRepository{pool: pool}
}

func (r *TagRepository) List(ctx context.Context, sortBy string, limit, offset int) ([]*Tag, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM tags`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Tag{}, 0, nil
	}

	orderSQL := "ORDER BY post_count DESC, t.slug ASC"
	if sortBy == "name" {
		orderSQL = "ORDER BY t.slug ASC"
	}

	q := fmt.Sprintf(`
		SELECT t.id, t.name, t.slug, count(p.id) AS post_count
		FROM tags t
		LEFT JOIN post_tags pt ON pt.tag_id = t.id
		LEFT JOIN posts p ON p.id = pt.post_id
		                 AND p.deleted_at IS NULL
		                 AND p.status = 'published'
		LEFT JOIN users u ON u.id = p.author_id AND u.deleted_at IS NULL
		GROUP BY t.id, t.name, t.slug
		%s
		LIMIT $1 OFFSET $2`, orderSQL)

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tags := []*Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.PostCount); err != nil {
			return nil, 0, err
		}
		tags = append(tags, &t)
	}
	return tags, total, rows.Err()
}
