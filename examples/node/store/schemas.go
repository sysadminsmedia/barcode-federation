package store

import (
	"database/sql"
	"fmt"
)

type CachedSchema struct {
	URL       string `db:"url"`
	Digest    string `db:"digest"`
	Content   string `db:"content"`
	FetchedAt string `db:"fetched_at"`
}

func (s *Store) GetCachedSchema(url string) (*CachedSchema, error) {
	var cs CachedSchema
	err := s.DB.Get(&cs, "SELECT * FROM schema_cache WHERE url = ?", url)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get cached schema: %w", err)
	}
	return &cs, nil
}

func (s *Store) CacheSchema(url, digest, content string) error {
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO schema_cache (url, digest, content, fetched_at)
		VALUES (?, ?, ?, datetime('now'))`, url, digest, content)
	return err
}
