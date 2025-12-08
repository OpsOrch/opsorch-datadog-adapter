package common

import (
	"context"
	"math"
	"time"
)

// RetryConfig defines retry behavior for API calls.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

// DefaultRetryConfig returns sensible defaults for retrying rate-limited requests.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic.
// It only retries on rate limit errors (429) and server errors (5xx).
func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !shouldRetry(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		// Calculate backoff duration
		backoff := calculateBackoff(attempt, cfg)

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return lastErr
}

// shouldRetry determines if an error is retryable.
func shouldRetry(err error) bool {
	// Retry on rate limit errors
	if IsRateLimited(err) {
		return true
	}

	// Retry on server errors (5xx)
	var apiErr *APIError
	if errors, ok := err.(*APIError); ok {
		apiErr = errors
		if apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
			return true
		}
	}

	return false
}

// calculateBackoff calculates the backoff duration for a given attempt.
func calculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffFactor, float64(attempt))

	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	return time.Duration(backoff)
}
