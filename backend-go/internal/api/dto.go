package api

import "github.com/500wpm/backend/internal/models"

// UploadResponse is returned after successful file or text upload
type UploadResponse struct {
	Code string              `json:"code"`
	Book *models.BookContent `json:"book"`
}

// BookResponse is returned when fetching a book by code
type BookResponse struct {
	Book *models.BookContent `json:"book"`
}

// ErrorResponse is returned on errors
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is returned by the health check endpoint
type HealthResponse struct {
	Status    string `json:"status"`
	BookCount int64  `json:"bookCount"`
}
