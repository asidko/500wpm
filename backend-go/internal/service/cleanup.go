package service

import (
	"context"
	"log"
	"sync"
	"time"
)

// CleanupRepository defines the interface for cleanup database operations
type CleanupRepository interface {
	// DeleteExpiredBooks removes books created before the cutoff time
	DeleteExpiredBooks(ctx context.Context, cutoff time.Time) (int64, error)
}

// cleanupThrottleInterval is how often cleanup can run (once per day)
const cleanupThrottleInterval = 24 * time.Hour

// Cleaner handles cleanup of expired books
// Triggered asynchronously on each upload, throttled to once per day
type Cleaner struct {
	repo        CleanupRepository
	ttlDays     int // 0 = disabled, books stored forever
	lastCleanup time.Time
	mu          sync.Mutex
}

// NewCleaner creates a new cleaner
// ttlDays: number of days after which books expire (0 = disabled, forever)
func NewCleaner(repo CleanupRepository, ttlDays int) *Cleaner {
	return &Cleaner{
		repo:    repo,
		ttlDays: ttlDays,
	}
}

// IsEnabled returns true if cleanup is enabled (ttlDays > 0)
func (c *Cleaner) IsEnabled() bool {
	return c.ttlDays > 0
}

// TTLDays returns the configured TTL in days
func (c *Cleaner) TTLDays() int {
	return c.ttlDays
}

// TriggerAsync runs cleanup in a background goroutine
// Throttled to run at most once per day for efficiency
func (c *Cleaner) TriggerAsync(ctx context.Context) {
	if !c.IsEnabled() {
		return
	}

	// Check if we should skip (throttle to once per day)
	c.mu.Lock()
	if time.Since(c.lastCleanup) < cleanupThrottleInterval {
		c.mu.Unlock()
		return
	}
	c.lastCleanup = time.Now()
	c.mu.Unlock()

	go c.runCleanup(ctx)
}

// runCleanup performs a single cleanup operation
func (c *Cleaner) runCleanup(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -c.ttlDays)
	deleted, err := c.repo.DeleteExpiredBooks(ctx, cutoff)
	if err != nil {
		log.Printf("Cleanup error: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("Cleaned up %d expired books (older than %d days)", deleted, c.ttlDays)
	}
}

// RunOnce performs a single cleanup operation synchronously (useful for testing)
func (c *Cleaner) RunOnce(ctx context.Context) (int64, error) {
	if !c.IsEnabled() {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -c.ttlDays)
	return c.repo.DeleteExpiredBooks(ctx, cutoff)
}
