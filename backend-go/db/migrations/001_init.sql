-- Books table stores uploaded/pasted books with their unique codes
CREATE TABLE IF NOT EXISTS books (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    code          TEXT UNIQUE NOT NULL,
    file_hash     TEXT UNIQUE NOT NULL,
    content       TEXT NOT NULL,           -- JSON blob with book structure
    file_size     INTEGER NOT NULL,        -- Original file/text size in bytes
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
    access_count  INTEGER DEFAULT 0
);

-- Index for fast code lookups (primary access pattern)
CREATE INDEX IF NOT EXISTS idx_books_code ON books(code);

-- Index for deduplication checks by hash
CREATE INDEX IF NOT EXISTS idx_books_file_hash ON books(file_hash);

-- Index for TTL cleanup (find old books)
CREATE INDEX IF NOT EXISTS idx_books_last_accessed ON books(last_accessed);
