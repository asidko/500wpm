package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/500wpm/backend/internal/service"
)

// RouterConfig holds router configuration
type RouterConfig struct {
	BookService *service.BookService
	RateLimit   RateLimitConfig
	StaticDir   string // Directory for static files (frontend)
}

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(Recovery)
	r.Use(Logger)
	r.Use(CORS)
	r.Use(middleware.RealIP)
	r.Use(middleware.Compress(5))

	// Create handler
	handler := NewHandler(cfg.BookService)

	// API routes with per-IP rate limiting
	r.Route("/api", func(r chi.Router) {
		r.Use(NewRateLimitMiddleware(cfg.RateLimit))

		// Upload file (EPUB, MOBI, TXT)
		r.Post("/upload", handler.UploadFile)

		// Upload text (pasted content)
		r.Post("/text", handler.UploadText)

		// Get book by code
		r.Get("/book/{code}", handler.GetBook)
	})

	// Health check (no rate limit)
	r.Get("/health", handler.Health)

	// Serve static files for frontend
	if cfg.StaticDir != "" {
		// Serve frontend files
		fileServer := http.FileServer(http.Dir(cfg.StaticDir))

		// Serve index.html for root
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, cfg.StaticDir+"/index.html")
		})

		// Serve reader.html
		r.Get("/reader.html", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, cfg.StaticDir+"/reader.html")
		})

		// Handle numeric code paths - serve reader.html for direct book links
		r.Get("/{code:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, cfg.StaticDir+"/reader.html")
		})

		// Serve static assets
		r.Handle("/dist/*", http.StripPrefix("/", fileServer))
		r.Handle("/styles.css", fileServer)
		r.Handle("/polyfills.js", fileServer)
		r.Handle("/favicon.ico", fileServer)
	}

	return r
}
