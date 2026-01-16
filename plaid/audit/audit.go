package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Operation represents the type of Plaid API operation.
type Operation string

const (
	OpLinkTokenCreate       Operation = "link_token_create"
	OpPublicTokenExchange   Operation = "public_token_exchange"
	OpItemGet               Operation = "item_get"
	OpItemRemove            Operation = "item_remove"
	OpAccountsGet           Operation = "accounts_get"
	OpAccountsBalanceGet    Operation = "accounts_balance_get"
	OpTransactionsGet       Operation = "transactions_get"
	OpLiabilitiesGet        Operation = "liabilities_get"
)

// Status represents the outcome of an API call.
type Status string

const (
	StatusAttempt Status = "attempt"  // Call is being attempted
	StatusSuccess Status = "success"  // Call succeeded
	StatusFailure Status = "failure"  // Call failed (after all retries)
	StatusRetry   Status = "retry"    // Call failed, will retry
)

// LogEntry represents a single audit log entry for an API call.
type LogEntry struct {
	// ID is the unique identifier for this log entry.
	ID uuid.UUID `json:"id"`

	// CorrelationID groups related log entries (e.g., retries of the same call).
	CorrelationID uuid.UUID `json:"correlation_id"`

	// UserID is the ID of the user associated with this call (if applicable).
	UserID *uuid.UUID `json:"user_id,omitempty"`

	// ItemID is the ID of the Plaid item associated with this call (if applicable).
	ItemID *uuid.UUID `json:"item_id,omitempty"`

	// Operation is the type of API operation.
	Operation Operation `json:"operation"`

	// Status is the outcome of this attempt.
	Status Status `json:"status"`

	// Attempt is the attempt number (1 for first attempt, 2 for first retry, etc.).
	Attempt int `json:"attempt"`

	// RequestID is the Plaid request ID (if available).
	RequestID string `json:"request_id,omitempty"`

	// Duration is how long this attempt took.
	Duration time.Duration `json:"duration_ns"`

	// ErrorCode is the Plaid error code (if applicable).
	ErrorCode string `json:"error_code,omitempty"`

	// ErrorType is the Plaid error type (if applicable).
	ErrorType string `json:"error_type,omitempty"`

	// ErrorMessage is the error message (if applicable).
	ErrorMessage string `json:"error_message,omitempty"`

	// Metadata contains additional context about the call.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// CreatedAt is when this log entry was created.
	CreatedAt time.Time `json:"created_at"`
}

// MetadataJSON returns the metadata as a JSON string.
func (e *LogEntry) MetadataJSON() string {
	if e.Metadata == nil {
		return "{}"
	}
	data, _ := json.Marshal(e.Metadata)
	return string(data)
}

// SetMetadataFromJSON parses JSON into the metadata field.
func (e *LogEntry) SetMetadataFromJSON(data string) error {
	if data == "" || data == "{}" {
		e.Metadata = nil
		return nil
	}
	return json.Unmarshal([]byte(data), &e.Metadata)
}

// Logger defines the interface for audit logging.
type Logger interface {
	// Log writes a log entry to the audit log.
	Log(ctx context.Context, entry *LogEntry) error

	// GetByCorrelationID retrieves all log entries for a correlation ID.
	GetByCorrelationID(ctx context.Context, correlationID uuid.UUID) ([]*LogEntry, error)

	// GetByUserID retrieves log entries for a user within a time range.
	GetByUserID(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*LogEntry, error)

	// GetByItemID retrieves log entries for an item within a time range.
	GetByItemID(ctx context.Context, itemID uuid.UUID, start, end time.Time) ([]*LogEntry, error)

	// GetByOperation retrieves log entries for an operation type within a time range.
	GetByOperation(ctx context.Context, op Operation, start, end time.Time) ([]*LogEntry, error)

	// GetFailures retrieves failed log entries within a time range.
	GetFailures(ctx context.Context, start, end time.Time) ([]*LogEntry, error)

	// Cleanup removes log entries older than the given duration.
	Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)
}

// NewLogEntry creates a new log entry with common fields populated.
func NewLogEntry(correlationID uuid.UUID, op Operation, attempt int) *LogEntry {
	return &LogEntry{
		ID:            uuid.New(),
		CorrelationID: correlationID,
		Operation:     op,
		Attempt:       attempt,
		Status:        StatusAttempt,
		CreatedAt:     time.Now(),
		Metadata:      make(map[string]interface{}),
	}
}

// MarkSuccess marks the log entry as successful.
func (e *LogEntry) MarkSuccess(requestID string, duration time.Duration) {
	e.Status = StatusSuccess
	e.RequestID = requestID
	e.Duration = duration
}

// MarkRetry marks the log entry as needing retry.
func (e *LogEntry) MarkRetry(err error, duration time.Duration) {
	e.Status = StatusRetry
	e.Duration = duration
	e.populateError(err)
}

// MarkFailure marks the log entry as failed.
func (e *LogEntry) MarkFailure(err error, duration time.Duration) {
	e.Status = StatusFailure
	e.Duration = duration
	e.populateError(err)
}

// populateError extracts error information into the log entry.
func (e *LogEntry) populateError(err error) {
	if err == nil {
		return
	}

	e.ErrorMessage = err.Error()

	// Extract Plaid-specific error info if available
	// Note: We use interface assertion here to avoid import cycle
	type plaidErrorLike interface {
		Error() string
	}
	type plaidErrorWithFields interface {
		plaidErrorLike
	}

	// Try to get error details via reflection-like approach
	// The actual PlaidError type will be set by the caller
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata["raw_error"] = err.Error()
}

// SetPlaidError sets Plaid-specific error fields.
func (e *LogEntry) SetPlaidError(errorType, errorCode, errorMessage, requestID string) {
	e.ErrorType = errorType
	e.ErrorCode = errorCode
	e.ErrorMessage = errorMessage
	e.RequestID = requestID
}

// NopLogger is a no-op implementation of Logger for testing or when logging is disabled.
type NopLogger struct{}

func (NopLogger) Log(ctx context.Context, entry *LogEntry) error                             { return nil }
func (NopLogger) GetByCorrelationID(ctx context.Context, id uuid.UUID) ([]*LogEntry, error)  { return nil, nil }
func (NopLogger) GetByUserID(ctx context.Context, id uuid.UUID, s, e time.Time) ([]*LogEntry, error) { return nil, nil }
func (NopLogger) GetByItemID(ctx context.Context, id uuid.UUID, s, e time.Time) ([]*LogEntry, error)  { return nil, nil }
func (NopLogger) GetByOperation(ctx context.Context, op Operation, s, e time.Time) ([]*LogEntry, error) { return nil, nil }
func (NopLogger) GetFailures(ctx context.Context, s, e time.Time) ([]*LogEntry, error)       { return nil, nil }
func (NopLogger) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error)        { return 0, nil }
