package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/500wpm/backend/internal/api"
	"github.com/500wpm/backend/internal/database"
	"github.com/500wpm/backend/internal/service"
)

// Config holds application configuration
type Config struct {
	Port               int
	Host               string
	DatabasePath       string
	StaticDir          string
	RateLimitPerMinute int
	StorageDays        int // 0 = forever, >0 = auto-delete after N days
}

func main() {
	// Parse command line flags
	cfg := Config{}
	flag.IntVar(&cfg.Port, "port", 8000, "Server port")
	flag.StringVar(&cfg.Host, "host", "", "Server host (empty for all interfaces)")
	flag.StringVar(&cfg.DatabasePath, "db", "./books.db", "Path to SQLite database")
	flag.StringVar(&cfg.StaticDir, "static", "../", "Path to static files directory")
	flag.IntVar(&cfg.RateLimitPerMinute, "rate-limit", 120, "API rate limit per IP (requests per minute)")
	flag.IntVar(&cfg.StorageDays, "storage-days", 0, "Book storage duration in days (0 = forever)")
	flag.Parse()

	// Override with environment variables if set
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &cfg.Port)
	}
	if envDBPath := os.Getenv("DATABASE_PATH"); envDBPath != "" {
		cfg.DatabasePath = envDBPath
	}
	if envStaticDir := os.Getenv("STATIC_DIR"); envStaticDir != "" {
		cfg.StaticDir = envStaticDir
	}
	if envRateLimit := os.Getenv("RATE_LIMIT"); envRateLimit != "" {
		fmt.Sscanf(envRateLimit, "%d", &cfg.RateLimitPerMinute)
	}
	if envStorageDays := os.Getenv("STORAGE_DAYS"); envStorageDays != "" {
		fmt.Sscanf(envStorageDays, "%d", &cfg.StorageDays)
	}

	// Open database
	db, err := database.Open(database.Config{Path: cfg.DatabasePath})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close(db)

	log.Printf("Database opened at %s", cfg.DatabasePath)

	// Create book service with TTL config
	bookService := service.NewBookService(db, cfg.StorageDays)

	// Create router
	router := api.NewRouter(api.RouterConfig{
		BookService: bookService,
		RateLimit: api.RateLimitConfig{
			RequestsPerMinute: cfg.RateLimitPerMinute,
			Window:            time.Minute,
		},
		StaticDir: cfg.StaticDir,
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on %s", addr)
		log.Printf("Static files: %s", cfg.StaticDir)
		log.Printf("Rate limit: %d req/min per IP", cfg.RateLimitPerMinute)
		if cfg.StorageDays > 0 {
			log.Printf("Book storage: %d days (auto-cleanup on upload)", cfg.StorageDays)
		} else {
			log.Printf("Book storage: forever (no auto-cleanup)")
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
