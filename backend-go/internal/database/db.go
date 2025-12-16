package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Config holds database configuration
type Config struct {
	Path string
}

// DefaultConfig returns the default database configuration
func DefaultConfig() Config {
	return Config{
		Path: "./books.db",
	}
}

// Open opens a database connection and ensures schema is applied
func Open(cfg Config) (*sql.DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open database with WAL mode for better concurrency
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", cfg.Path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite only supports one writer
	db.SetMaxIdleConns(1)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Apply schema
	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return db, nil
}

// applySchema creates the necessary tables if they don't exist
func applySchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS books (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		code          TEXT UNIQUE NOT NULL,
		file_hash     TEXT UNIQUE NOT NULL,
		content       TEXT NOT NULL,
		file_size     INTEGER NOT NULL,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
		access_count  INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_books_code ON books(code);
	CREATE INDEX IF NOT EXISTS idx_books_file_hash ON books(file_hash);
	CREATE INDEX IF NOT EXISTS idx_books_last_accessed ON books(last_accessed);
	`

	_, err := db.Exec(schema)
	return err
}

// Close closes the database connection
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
