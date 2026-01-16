package plaid

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/kinected/kinected/plaid/audit"
)

// apiCaller wraps Plaid API calls with retry and audit logging.
type apiCaller struct {
	client  *Client
	retrier *retrier
	logger  audit.Logger
}

// newAPICaller creates a new API caller with retry and logging support.
func newAPICaller(client *Client, retryConfig RetryConfig, logger audit.Logger) *apiCaller {
	if logger == nil {
		logger = audit.NopLogger{}
	}
	return &apiCaller{
		client:  client,
		retrier: newRetrier(retryConfig),
		logger:  logger,
	}
}

// callContext holds context for a single API call.
type callContext struct {
	correlationID uuid.UUID
	operation     audit.Operation
	userID        *uuid.UUID
	itemID        *uuid.UUID
	metadata      map[string]interface{}
}

// newCallContext creates a new call context.
func newCallContext(op audit.Operation) *callContext {
	return &callContext{
		correlationID: uuid.New(),
		operation:     op,
		metadata:      make(map[string]interface{}),
	}
}

// withUserID sets the user ID for the call context.
func (c *callContext) withUserID(id uuid.UUID) *callContext {
	c.userID = &id
	return c
}

// withItemID sets the item ID for the call context.
func (c *callContext) withItemID(id uuid.UUID) *callContext {
	c.itemID = &id
	return c
}

// withMetadata adds metadata to the call context.
func (c *callContext) withMetadata(key string, value interface{}) *callContext {
	c.metadata[key] = value
	return c
}

// call executes a Plaid API call with retry and logging.
// The fn function should perform the actual API call and return an error.
// The getRequestID function extracts the request ID from a successful response.
func (a *apiCaller) call(ctx context.Context, cc *callContext, fn func() error, getRequestID func() string) error {
	var attempt int

	result := a.retrier.Do(ctx, func() error {
		attempt++
		startTime := time.Now()

		// Create log entry for this attempt
		entry := audit.NewLogEntry(cc.correlationID, cc.operation, attempt)
		entry.UserID = cc.userID
		entry.ItemID = cc.itemID
		entry.Metadata = cc.metadata

		// Log the attempt
		a.logger.Log(ctx, entry)

		// Execute the API call
		err := fn()
		duration := time.Since(startTime)

		// Update log entry based on result
		if err == nil {
			entry.MarkSuccess(getRequestID(), duration)
			a.logger.Log(ctx, entry)
			return nil
		}

		// Extract Plaid error details if available
		if plaidErr, ok := err.(*PlaidError); ok {
			entry.SetPlaidError(plaidErr.ErrorType, plaidErr.ErrorCode, plaidErr.ErrorMessage, plaidErr.RequestID)
		}

		// Determine if this is a retry or final failure
		if a.retrier.shouldRetry(err) && attempt < a.retrier.config.MaxRetries+1 {
			entry.MarkRetry(err, duration)
		} else {
			entry.MarkFailure(err, duration)
		}

		a.logger.Log(ctx, entry)

		return err
	})

	return result.LastError
}

// callWithResult executes a Plaid API call that returns a result with retry and logging.
func callWithResult[T any](ctx context.Context, a *apiCaller, cc *callContext, fn func() (T, string, error)) (T, error) {
	var result T
	var requestID string

	err := a.call(ctx, cc, func() error {
		var callErr error
		result, requestID, callErr = fn()
		return callErr
	}, func() string {
		return requestID
	})

	return result, err
}
