-- name: GetBookByCode :one
SELECT * FROM books WHERE code = ? LIMIT 1;

-- name: GetBookByHash :one
SELECT * FROM books WHERE file_hash = ? LIMIT 1;

-- name: CreateBook :one
INSERT INTO books (code, file_hash, content, file_size)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateLastAccessed :exec
UPDATE books
SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1
WHERE code = ?;

-- name: CodeExists :one
SELECT EXISTS(SELECT 1 FROM books WHERE code = ?) AS exists_flag;

-- name: DeleteExpiredBooks :execrows
DELETE FROM books WHERE created_at < ?;

-- name: GetBookCount :one
SELECT COUNT(*) FROM books;
