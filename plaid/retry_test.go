package plaid

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != time.Second {
		t.Errorf("expected InitialBackoff=1s, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("expected MaxBackoff=30s, got %v", cfg.MaxBackoff)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %f", cfg.BackoffMultiplier)
	}
	if cfg.Jitter != 0.1 {
		t.Errorf("expected Jitter=0.1, got %f", cfg.Jitter)
	}
}

func TestRetrier_SuccessOnFirstAttempt(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	attempts := 0
	result := r.Do(context.Background(), func() error {
		attempts++
		return nil
	})

	if result.LastError != nil {
		t.Errorf("expected no error, got %v", result.LastError)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
	if result.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", result.Attempts)
	}
}

func TestRetrier_SuccessAfterRetries(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	attempts := 0
	result := r.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return &PlaidError{
				ErrorType: "RATE_LIMIT_EXCEEDED",
				ErrorCode: "RATE_LIMIT",
			}
		}
		return nil
	})

	if result.LastError != nil {
		t.Errorf("expected no error, got %v", result.LastError)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetrier_ExhaustsRetries(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	attempts := 0
	permanentErr := &PlaidError{
		ErrorType: "RATE_LIMIT_EXCEEDED",
		ErrorCode: "RATE_LIMIT",
	}
	result := r.Do(context.Background(), func() error {
		attempts++
		return permanentErr
	})

	if result.LastError == nil {
		t.Error("expected error after exhausting retries")
	}
	// MaxRetries=2 means: 1 initial + 2 retries = 3 total attempts
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetrier_NonRetryableError(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	attempts := 0
	nonRetryableErr := &PlaidError{
		ErrorType: "INVALID_REQUEST",
		ErrorCode: "INVALID_FIELD",
	}
	result := r.Do(context.Background(), func() error {
		attempts++
		return nonRetryableErr
	})

	if result.LastError == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetrier_ContextCancellation(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        10,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	result := r.Do(ctx, func() error {
		attempts++
		if attempts == 2 {
			cancel()
		}
		return &PlaidError{
			ErrorType: "RATE_LIMIT_EXCEEDED",
			ErrorCode: "RATE_LIMIT",
		}
	})

	// Should stop after context is cancelled
	if result.LastError == nil {
		t.Error("expected error")
	}
	if attempts > 3 {
		t.Errorf("expected to stop retrying after context cancel, got %d attempts", attempts)
	}
}

func TestRetrier_ShouldRetry(t *testing.T) {
	r := newRetrier(DefaultRetryConfig())

	testCases := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{"nil error", nil, false},
		// Non-Plaid errors are retried (network errors, etc.)
		{"generic error", errors.New("generic"), true},
		// Rate limit errors (checked by ErrorType)
		{"rate limit", &PlaidError{ErrorType: "RATE_LIMIT_EXCEEDED"}, true},
		// Internal server error (checked by ErrorCode, not ErrorType)
		{"internal server error", &PlaidError{ErrorCode: "INTERNAL_SERVER_ERROR"}, true},
		// Planned maintenance
		{"planned maintenance", &PlaidError{ErrorCode: "PLANNED_MAINTENANCE"}, true},
		// Service unavailable
		{"service unavailable", &PlaidError{ErrorCode: "SERVICE_UNAVAILABLE"}, true},
		// Auth errors should not be retried
		{"login required", &PlaidError{ErrorCode: "ITEM_LOGIN_REQUIRED"}, false},
		// Invalid request should not be retried
		{"invalid request", &PlaidError{ErrorType: "INVALID_REQUEST"}, false},
		// Invalid input should not be retried
		{"invalid input", &PlaidError{ErrorType: "INVALID_INPUT"}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := r.shouldRetry(tc.err)
			if result != tc.shouldRetry {
				t.Errorf("expected shouldRetry=%v, got %v", tc.shouldRetry, result)
			}
		})
	}
}

func TestRetrier_BackoffCalculation(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}
	r := newRetrier(cfg)

	// Test backoff progression (without jitter)
	// calculateBackoff uses 1-indexed attempts
	expected := []time.Duration{
		0,                      // attempt 0 (no backoff)
		100 * time.Millisecond, // attempt 1
		200 * time.Millisecond, // attempt 2
		400 * time.Millisecond, // attempt 3
		800 * time.Millisecond, // attempt 4
		1 * time.Second,        // attempt 5 (capped at MaxBackoff)
	}

	for i, exp := range expected {
		backoff := r.calculateBackoff(i)
		if backoff != exp {
			t.Errorf("attempt %d: expected %v, got %v", i, exp, backoff)
		}
	}
}

func TestRetrier_BackoffWithJitter(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.5, // 50% jitter
	}
	r := newRetrier(cfg)

	// With jitter, backoff should vary but be within expected range
	// Using attempt 1 (first retry) for base calculation
	baseBackoff := 100 * time.Millisecond
	minBackoff := time.Duration(float64(baseBackoff) * (1 - cfg.Jitter))
	maxBackoff := time.Duration(float64(baseBackoff) * (1 + cfg.Jitter))

	// Run multiple times to test randomness
	for i := 0; i < 20; i++ {
		backoff := r.calculateBackoff(1)
		if backoff < minBackoff || backoff > maxBackoff {
			t.Errorf("backoff %v outside expected range [%v, %v]", backoff, minBackoff, maxBackoff)
		}
	}
}
