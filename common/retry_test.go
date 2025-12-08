package common

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetryWithBackoff_Success(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	fn := func() error {
		attempts++
		return nil
	}

	err := RetryWithBackoff(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryWithBackoff_RateLimitRetry(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return &APIError{
				StatusCode: http.StatusTooManyRequests,
				Message:    "rate limited",
				Err:        ErrRateLimited,
			}
		}
		return nil
	}

	start := time.Now()
	err := RetryWithBackoff(context.Background(), cfg, fn)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Should have waited at least 10ms + 20ms = 30ms
	minDuration := 30 * time.Millisecond
	if duration < minDuration {
		t.Errorf("Expected duration >= %v, got %v", minDuration, duration)
	}
}

func TestRetryWithBackoff_ServerErrorRetry(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 2 {
			return &APIError{
				StatusCode: http.StatusInternalServerError,
				Message:    "server error",
				Err:        ErrServerError,
			}
		}
		return nil
	}

	err := RetryWithBackoff(context.Background(), cfg, fn)
	if err != nil {
		t.Errorf("Expected no error after retries, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	expectedErr := &APIError{
		StatusCode: http.StatusNotFound,
		Message:    "not found",
		Err:        ErrNotFound,
	}

	fn := func() error {
		attempts++
		return expectedErr
	}

	err := RetryWithBackoff(context.Background(), cfg, fn)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestRetryWithBackoff_MaxRetriesExceeded(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		BackoffFactor:  2.0,
	}

	attempts := 0
	rateLimitErr := &APIError{
		StatusCode: http.StatusTooManyRequests,
		Message:    "rate limited",
		Err:        ErrRateLimited,
	}

	fn := func() error {
		attempts++
		return rateLimitErr
	}

	err := RetryWithBackoff(context.Background(), cfg, fn)
	if err != rateLimitErr {
		t.Errorf("Expected rate limit error, got %v", err)
	}

	// Should attempt: initial + 2 retries = 3 total
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     1 * time.Second,
		BackoffFactor:  2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	fn := func() error {
		attempts++
		if attempts == 2 {
			cancel() // Cancel after second attempt
		}
		return &APIError{
			StatusCode: http.StatusTooManyRequests,
			Message:    "rate limited",
			Err:        ErrRateLimited,
		}
	}

	err := RetryWithBackoff(ctx, cfg, fn)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	if attempts < 2 {
		t.Errorf("Expected at least 2 attempts, got %d", attempts)
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := RetryConfig{
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		BackoffFactor:  2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at MaxBackoff
		{5, 10 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		result := calculateBackoff(tt.attempt, cfg)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, expected %v", tt.attempt, result, tt.expected)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "rate limit error",
			err: &APIError{
				StatusCode: http.StatusTooManyRequests,
				Err:        ErrRateLimited,
			},
			expected: true,
		},
		{
			name: "server error 500",
			err: &APIError{
				StatusCode: http.StatusInternalServerError,
				Err:        ErrServerError,
			},
			expected: true,
		},
		{
			name: "server error 502",
			err: &APIError{
				StatusCode: http.StatusBadGateway,
				Err:        ErrServerError,
			},
			expected: true,
		},
		{
			name: "not found error",
			err: &APIError{
				StatusCode: http.StatusNotFound,
				Err:        ErrNotFound,
			},
			expected: false,
		},
		{
			name: "unauthorized error",
			err: &APIError{
				StatusCode: http.StatusUnauthorized,
				Err:        ErrUnauthorized,
			},
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldRetry(tt.err)
			if result != tt.expected {
				t.Errorf("shouldRetry(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries = 3, got %d", cfg.MaxRetries)
	}

	if cfg.InitialBackoff != 1*time.Second {
		t.Errorf("Expected InitialBackoff = 1s, got %v", cfg.InitialBackoff)
	}

	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("Expected MaxBackoff = 30s, got %v", cfg.MaxBackoff)
	}

	if cfg.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor = 2.0, got %f", cfg.BackoffFactor)
	}
}
