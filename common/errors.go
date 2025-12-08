package common

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

// Sentinel errors for common failure modes.
var (
	ErrUnauthorized  = errors.New("authentication failed")
	ErrNotFound      = errors.New("resource not found")
	ErrRateLimited   = errors.New("rate limit exceeded")
	ErrServerError   = errors.New("server error")
	ErrInvalidConfig = errors.New("invalid configuration")
)

// APIError wraps an error from the Datadog API with additional context.
type APIError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s (status %d)", e.Err.Error(), e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s (status %d)", e.Err.Error(), e.StatusCode)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// WrapAPIError creates an appropriate error from an HTTP response.
// It sanitizes the response body to never expose credentials.
func WrapAPIError(resp *http.Response, cfg Config) error {
	body, _ := io.ReadAll(resp.Body)
	message := sanitizeErrorMessage(string(body), cfg)

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Err:        ErrUnauthorized,
		}
	case http.StatusNotFound:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Err:        ErrNotFound,
		}
	case http.StatusTooManyRequests:
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Err:        ErrRateLimited,
		}
	default:
		if resp.StatusCode >= 500 {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    message,
				Err:        ErrServerError,
			}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
			Err:        fmt.Errorf("API error"),
		}
	}
}

// sanitizeErrorMessage removes any credentials from error messages.
func sanitizeErrorMessage(message string, cfg Config) string {
	result := message

	// Remove API key if present
	if cfg.APIKey != "" {
		result = strings.ReplaceAll(result, cfg.APIKey, "[REDACTED]")
	}

	// Remove App key if present
	if cfg.AppKey != "" {
		result = strings.ReplaceAll(result, cfg.AppKey, "[REDACTED]")
	}

	// Truncate very long messages
	const maxLen = 500
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}

	return result
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if an error is an authentication error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsRateLimited checks if an error is a rate limit error.
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// HandleSDKError converts Datadog SDK errors to OpsOrch APIError format.
// It extracts status codes and messages from SDK GenericOpenAPIError types.
func HandleSDKError(err error, cfg Config) error {
	if err == nil {
		return nil
	}

	// Check if it's a GenericOpenAPIError from the SDK
	var apiErr datadog.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		statusCode := 0
		message := apiErr.Error()

		// Try to extract status code from the error body
		if body := apiErr.Body(); len(body) > 0 {
			message = sanitizeErrorMessage(string(body), cfg)
		}

		// The SDK doesn't expose status code directly, so we infer from error message
		// or use the model() method if available
		if strings.Contains(message, "403") || strings.Contains(message, "401") {
			statusCode = http.StatusForbidden
			return &APIError{
				StatusCode: statusCode,
				Message:    message,
				Err:        ErrUnauthorized,
			}
		}

		if strings.Contains(message, "404") {
			statusCode = http.StatusNotFound
			return &APIError{
				StatusCode: statusCode,
				Message:    message,
				Err:        ErrNotFound,
			}
		}

		if strings.Contains(message, "429") {
			statusCode = http.StatusTooManyRequests
			return &APIError{
				StatusCode: statusCode,
				Message:    message,
				Err:        ErrRateLimited,
			}
		}

		if strings.Contains(message, "500") || strings.Contains(message, "502") || strings.Contains(message, "503") {
			statusCode = http.StatusInternalServerError
			return &APIError{
				StatusCode: statusCode,
				Message:    message,
				Err:        ErrServerError,
			}
		}

		// Generic API error
		return &APIError{
			StatusCode: statusCode,
			Message:    message,
			Err:        fmt.Errorf("API error"),
		}
	}

	// Not an SDK error, return as-is
	return err
}
