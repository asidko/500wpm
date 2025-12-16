package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/500wpm/backend/internal/service"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	bookService *service.BookService
}

// NewHandler creates a new handler with dependencies
func NewHandler(bookService *service.BookService) *Handler {
	return &Handler{
		bookService: bookService,
	}
}

// UploadFile handles file uploads (EPUB, MOBI, TXT)
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Limit request body size (50MB + overhead for multipart encoding)
	r.Body = http.MaxBytesReader(w, r.Body, 55*1024*1024)

	// Parse multipart form
	if err := r.ParseMultipartForm(55 * 1024 * 1024); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Failed to parse form", err.Error())
		return
	}

	// Get the file
	file, header, err := r.FormFile("file")
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "No file provided", err.Error())
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Failed to read file", err.Error())
		return
	}

	// Process the file
	result, err := h.bookService.ProcessFile(r.Context(), content, header.Filename)
	if err != nil {
		if errors.Is(err, service.ErrFileTooLarge) {
			h.errorResponse(w, http.StatusRequestEntityTooLarge, "File too large", "Maximum file size is 50MB")
			return
		}
		h.errorResponse(w, http.StatusBadRequest, "Failed to process file", err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, UploadResponse{
		Code: result.Code,
		Book: result.Book,
	})
}

// UploadText handles pasted text
func (h *Handler) UploadText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text  string `json:"text"`
		Title string `json:"title,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid JSON", err.Error())
		return
	}

	if strings.TrimSpace(req.Text) == "" {
		h.errorResponse(w, http.StatusBadRequest, "Empty text", "Text content is required")
		return
	}

	// Process the text
	result, err := h.bookService.ProcessText(r.Context(), req.Text, req.Title)
	if err != nil {
		if errors.Is(err, service.ErrFileTooLarge) {
			h.errorResponse(w, http.StatusRequestEntityTooLarge, "Text too large", "Maximum size is 50MB")
			return
		}
		h.errorResponse(w, http.StatusBadRequest, "Failed to process text", err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, UploadResponse{
		Code: result.Code,
		Book: result.Book,
	})
}

// GetBook retrieves a book by its code
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	codeParam := chi.URLParam(r, "code")

	// Extract code from input (handles full URLs or raw codes)
	code := service.ExtractCodeFromInput(codeParam)
	if code == "" {
		code = codeParam // Use as-is if extraction fails
	}

	book, err := h.bookService.GetBookByCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, service.ErrBookNotFound) {
			h.errorResponse(w, http.StatusNotFound, "Book not found", "No book found with this code")
			return
		}
		if errors.Is(err, service.ErrInvalidCode) {
			h.errorResponse(w, http.StatusBadRequest, "Invalid code", "Code must be 6-10 digits")
			return
		}
		h.errorResponse(w, http.StatusInternalServerError, "Failed to get book", err.Error())
		return
	}

	h.jsonResponse(w, http.StatusOK, BookResponse{Book: book})
}

// Health returns a health check response
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	stats, _ := h.bookService.GetStats(r.Context())

	h.jsonResponse(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		BookCount: stats,
	})
}

// jsonResponse writes a JSON response
func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse writes an error response
func (h *Handler) errorResponse(w http.ResponseWriter, status int, message, details string) {
	h.jsonResponse(w, status, ErrorResponse{
		Error:   message,
		Details: details,
	})
}
