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
	Port              int
	Host              string
	BaseURL           string
	DatabasePath      string
	StaticDir         string
	RateLimit         float64
	RateLimitBurst    int
}

func main() {
	// Parse command line flags
	cfg := Config{}
	flag.IntVar(&cfg.Port, "port", 8000, "Server port")
	flag.StringVar(&cfg.Host, "host", "", "Server host (empty for all interfaces)")
	flag.StringVar(&cfg.BaseURL, "base-url", "", "Base URL for shareable links (auto-detected if empty)")
	flag.StringVar(&cfg.DatabasePath, "db", "./books.db", "Path to SQLite database")
	flag.StringVar(&cfg.StaticDir, "static", "../", "Path to static files directory")
	flag.Float64Var(&cfg.RateLimit, "rate-limit", 2.0, "API rate limit (requests per second)")
	flag.IntVar(&cfg.RateLimitBurst, "rate-limit-burst", 5, "Rate limit burst allowance")
	flag.Parse()

	// Override with environment variables if set
	if envPort := os.Getenv("PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &cfg.Port)
	}
	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}
	if envDBPath := os.Getenv("DATABASE_PATH"); envDBPath != "" {
		cfg.DatabasePath = envDBPath
	}
	if envStaticDir := os.Getenv("STATIC_DIR"); envStaticDir != "" {
		cfg.StaticDir = envStaticDir
	}
	if envRateLimit := os.Getenv("RATE_LIMIT"); envRateLimit != "" {
		fmt.Sscanf(envRateLimit, "%f", &cfg.RateLimit)
	}

	// Auto-detect base URL if not set
	if cfg.BaseURL == "" {
		if cfg.Host == "" {
			cfg.BaseURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
		} else {
			cfg.BaseURL = fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
		}
	}

	// Open database
	db, err := database.Open(database.Config{Path: cfg.DatabasePath})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close(db)

	log.Printf("Database opened at %s", cfg.DatabasePath)

	// Create book service
	bookService := service.NewBookService(db, cfg.BaseURL)

	// Start cleanup worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bookService.StartCleanupWorker(ctx)

	// Create router
	router := api.NewRouter(api.RouterConfig{
		BookService: bookService,
		RateLimit: api.RateLimitConfig{
			RequestsPerSecond: cfg.RateLimit,
			Burst:             cfg.RateLimitBurst,
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
		log.Printf("Base URL: %s", cfg.BaseURL)
		log.Printf("Static files: %s", cfg.StaticDir)
		log.Printf("Rate limit: %.1f req/s (burst: %d)", cfg.RateLimit, cfg.RateLimitBurst)

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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
