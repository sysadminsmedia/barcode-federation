package store

import (
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sqlx.DB
}

func New(dbPath string) (*Store, error) {
	dsn := dbPath + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite handles one writer at a time
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	slog.Info("database initialized", "path", dbPath)
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}
