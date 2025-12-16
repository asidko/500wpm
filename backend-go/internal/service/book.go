package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/500wpm/backend/db/sqlc"
	"github.com/500wpm/backend/internal/converter"
	"github.com/500wpm/backend/internal/models"
)

// Configuration constants
const (
	DefaultCodeLength = 6
	MaxCodeLength     = 10
	MaxFileSize       = 50 * 1024 * 1024 // 50MB
)

// Common errors
var (
	ErrFileTooLarge = errors.New("file exceeds maximum size limit")
	ErrBookNotFound = errors.New("book not found")
	ErrInvalidCode  = errors.New("invalid code format")
)

// ProcessResult contains the result of processing a file or text
type ProcessResult struct {
	Code string
	Book *models.BookContent
}

// BookService handles book-related business logic
type BookService struct {
	queries   *sqlc.Queries
	db        *sql.DB
	converter *converter.Manager
	codeGen   *CodeGenerator
	cleaner   *Cleaner
}

// NewBookService creates a new book service
// ttlDays: number of days after which books expire (0 = disabled, forever)
func NewBookService(db *sql.DB, ttlDays int) *BookService {
	svc := &BookService{
		queries:   sqlc.New(db),
		db:        db,
		converter: converter.NewManager(),
		codeGen:   NewCodeGenerator(DefaultCodeLength, MaxCodeLength),
	}
	// Cleaner uses BookService itself as the repository (implements CleanupRepository)
	svc.cleaner = NewCleaner(svc, ttlDays)
	return svc
}

// ProcessFile processes an uploaded file and returns a unique code
func (s *BookService) ProcessFile(ctx context.Context, content []byte, filename string) (*ProcessResult, error) {
	if len(content) > MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// Calculate file hash for deduplication
	hash := s.hashContent(content)

	// Check if this file was already processed
	existing, err := s.queries.GetBookByHash(ctx, hash)
	if err == nil {
		// Found existing book - update access time and return
		_ = s.queries.UpdateLastAccessed(ctx, existing.Code)

		bookContent, err := models.ParseBookContent(existing.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stored content: %w", err)
		}

		return &ProcessResult{
			Code: existing.Code,
			Book: bookContent,
		}, nil
	}

	// Convert file to book content
	bookContent, err := s.converter.Convert(content, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to convert file: %w", err)
	}

	// Generate unique code
	code, err := s.codeGen.GenerateUniqueCode(ctx, s.codeExists)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	// Serialize book content to JSON
	jsonContent, err := bookContent.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize book: %w", err)
	}

	// Save to database
	_, err = s.queries.CreateBook(ctx, sqlc.CreateBookParams{
		Code:     code,
		FileHash: hash,
		Content:  jsonContent,
		FileSize: int64(len(content)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save book: %w", err)
	}

	// Trigger async cleanup (throttled to once per day)
	s.cleaner.TriggerAsync(ctx)

	return &ProcessResult{
		Code: code,
		Book: bookContent,
	}, nil
}

// ProcessText processes pasted text and returns a unique code
func (s *BookService) ProcessText(ctx context.Context, text, title string) (*ProcessResult, error) {
	content := []byte(text)

	if len(content) > MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// Calculate hash for deduplication
	hash := s.hashContent(content)

	// Check if this exact text was already saved
	existing, err := s.queries.GetBookByHash(ctx, hash)
	if err == nil {
		// Found existing - update access time and return
		_ = s.queries.UpdateLastAccessed(ctx, existing.Code)

		bookContent, err := models.ParseBookContent(existing.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stored content: %w", err)
		}

		return &ProcessResult{
			Code: existing.Code,
			Book: bookContent,
		}, nil
	}

	// Convert text to book content
	bookContent, err := s.converter.ConvertText(text, title)
	if err != nil {
		return nil, fmt.Errorf("failed to convert text: %w", err)
	}

	// Generate unique code
	code, err := s.codeGen.GenerateUniqueCode(ctx, s.codeExists)
	if err != nil {
		return nil, fmt.Errorf("failed to generate code: %w", err)
	}

	// Serialize book content to JSON
	jsonContent, err := bookContent.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize book: %w", err)
	}

	// Save to database
	_, err = s.queries.CreateBook(ctx, sqlc.CreateBookParams{
		Code:     code,
		FileHash: hash,
		Content:  jsonContent,
		FileSize: int64(len(content)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save book: %w", err)
	}

	// Trigger async cleanup (throttled to once per day)
	s.cleaner.TriggerAsync(ctx)

	return &ProcessResult{
		Code: code,
		Book: bookContent,
	}, nil
}

// GetBookByCode retrieves a book by its code
func (s *BookService) GetBookByCode(ctx context.Context, code string) (*models.BookContent, error) {
	// Validate code format (should be numeric)
	if !isValidCode(code) {
		return nil, ErrInvalidCode
	}

	book, err := s.queries.GetBookByCode(ctx, code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("failed to get book: %w", err)
	}

	// Update last accessed time
	_ = s.queries.UpdateLastAccessed(ctx, code)

	// Parse and return book content
	bookContent, err := models.ParseBookContent(book.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse book content: %w", err)
	}

	return bookContent, nil
}

// DeleteExpiredBooks implements CleanupRepository interface
func (s *BookService) DeleteExpiredBooks(ctx context.Context, cutoff time.Time) (int64, error) {
	deleted, err := s.queries.DeleteExpiredBooks(ctx, sql.NullTime{Time: cutoff, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired books: %w", err)
	}
	return deleted, nil
}

// GetStats returns basic statistics
func (s *BookService) GetStats(ctx context.Context) (int64, error) {
	return s.queries.GetBookCount(ctx)
}

// hashContent calculates SHA256 hash of content
func (s *BookService) hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// codeExists checks if a code already exists in the database
func (s *BookService) codeExists(ctx context.Context, code string) (bool, error) {
	exists, err := s.queries.CodeExists(ctx, code)
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

// isValidCode validates that a code is numeric
func isValidCode(code string) bool {
	if len(code) < DefaultCodeLength || len(code) > MaxCodeLength {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ExtractCodeFromInput extracts the last continuous sequence of digits from any input
// This allows users to paste full URLs or text containing the code
func ExtractCodeFromInput(input string) string {
	var lastSequence string
	var currentSequence string

	for _, c := range input {
		if c >= '0' && c <= '9' {
			currentSequence += string(c)
		} else {
			if len(currentSequence) >= DefaultCodeLength {
				lastSequence = currentSequence
			}
			currentSequence = ""
		}
	}

	// Check last sequence
	if len(currentSequence) >= DefaultCodeLength {
		lastSequence = currentSequence
	}

	return lastSequence
}
