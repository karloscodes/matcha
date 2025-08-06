package database

import (
	"fmt"
	"math"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func New(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(databaseURL+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=on"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	// Configure SQLite connection pool for single writer
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SQLite only supports one writer, so limit connections
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

// PerformWrite executes a database write operation with retry logic and exponential backoff
func PerformWrite(db *gorm.DB, operation func(*gorm.DB) error) error {
	maxRetries := 5
	baseDelay := 50 * time.Millisecond
	maxDelay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation(db)
		if err == nil {
			return nil // Success
		}

		// Check if it's a database locked error
		if isLockError(err) && attempt < maxRetries {
			// Calculate exponential backoff delay with jitter
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			if delay > maxDelay {
				delay = maxDelay
			}

			// Add some jitter (±25% of the delay)
			jitterFactor := (float64(time.Now().UnixNano()%1000)/1000.0 - 0.5) * 0.5 // -0.25 to +0.25
			jitter := time.Duration(float64(delay) * jitterFactor)
			delay = delay + jitter

			time.Sleep(delay)
			continue
		}

		return err // Non-recoverable error or max retries exceeded
	}

	return fmt.Errorf("database write failed after %d attempts", maxRetries+1)
}

// isLockError checks if the error is related to database locking
func isLockError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "database is locked") ||
		contains(errStr, "SQLITE_BUSY") ||
		contains(errStr, "database table is locked") ||
		contains(errStr, "cannot start a transaction within a transaction")
}

// contains is a simple string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || s[0:len(substr)] == substr || contains(s[1:], substr))
}
