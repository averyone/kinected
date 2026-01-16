package plaid

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures the retry behavior for API calls.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries).
	MaxRetries int

	// InitialBackoff is the initial backoff duration.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration

	// BackoffMultiplier is the factor by which backoff increases each retry.
	BackoffMultiplier float64

	// Jitter adds randomness to backoff to prevent thundering herd.
	// Value between 0 and 1, where 0.1 means ±10% jitter.
	Jitter float64
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}
}

// NoRetryConfig returns a configuration that disables retries.
func NoRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 0,
	}
}

// RetryResult contains information about a retry operation.
type RetryResult struct {
	// Attempts is the total number of attempts made.
	Attempts int

	// LastError is the last error encountered (nil if successful).
	LastError error

	// Duration is the total time spent including all retries.
	Duration time.Duration

	// Backoffs contains the backoff durations for each retry.
	Backoffs []time.Duration
}

// retrier handles retry logic with exponential backoff.
type retrier struct {
	config RetryConfig
	rng    *rand.Rand
}

// newRetrier creates a new retrier with the given configuration.
func newRetrier(config RetryConfig) *retrier {
	return &retrier{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// shouldRetry determines if an error is retryable.
func (r *retrier) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a Plaid error
	if plaidErr, ok := err.(*PlaidError); ok {
		// Retry on rate limits
		if plaidErr.IsRateLimitError() {
			return true
		}

		// Don't retry on auth errors - they need user intervention
		if plaidErr.NeedsReauthentication() {
			return false
		}

		// Retry on certain transient errors
		switch plaidErr.ErrorCode {
		case "INTERNAL_SERVER_ERROR",
			"PLANNED_MAINTENANCE",
			"SERVICE_UNAVAILABLE":
			return true
		}

		// Don't retry on other Plaid errors (invalid requests, etc.)
		return false
	}

	// For non-Plaid errors (network issues, etc.), retry
	return true
}

// calculateBackoff calculates the backoff duration for a given attempt.
func (r *retrier) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffMultiplier, float64(attempt-1))

	// Apply max backoff limit
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	// Apply jitter
	if r.config.Jitter > 0 {
		jitterRange := backoff * r.config.Jitter
		jitter := (r.rng.Float64()*2 - 1) * jitterRange // Random value between -jitterRange and +jitterRange
		backoff += jitter
	}

	return time.Duration(backoff)
}

// Do executes the given function with retry logic.
// The function should return an error if it fails.
func (r *retrier) Do(ctx context.Context, fn func() error) *RetryResult {
	result := &RetryResult{
		Backoffs: make([]time.Duration, 0, r.config.MaxRetries),
	}

	startTime := time.Now()

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Execute the function
		err := fn()
		if err == nil {
			// Clear any previous error on success
			result.LastError = nil
			result.Duration = time.Since(startTime)
			return result
		}

		result.LastError = err

		// Check if we should retry
		if attempt >= r.config.MaxRetries || !r.shouldRetry(err) {
			result.Duration = time.Since(startTime)
			return result
		}

		// Calculate and wait for backoff
		backoff := r.calculateBackoff(attempt + 1)
		result.Backoffs = append(result.Backoffs, backoff)

		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			result.Duration = time.Since(startTime)
			return result
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// DoWithResult executes a function that returns a value and error with retry logic.
func DoWithResult[T any](ctx context.Context, r *retrier, fn func() (T, error)) (T, *RetryResult) {
	var result T
	var lastResult T

	retryResult := r.Do(ctx, func() error {
		var err error
		lastResult, err = fn()
		if err == nil {
			result = lastResult
		}
		return err
	})

	return result, retryResult
}
