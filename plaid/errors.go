package plaid

import (
	"errors"
	"fmt"

	"github.com/kinected/kinected/plaid/models"
)

// Re-export model errors for convenience.
var (
	ErrItemNotFound    = models.ErrItemNotFound
	ErrAccountNotFound = models.ErrAccountNotFound
	ErrUserNotFound    = models.ErrUserNotFound
)

// Client-specific errors.
var (
	ErrInvalidPublicToken = errors.New("plaid: invalid public token")
	ErrItemLoginRequired  = errors.New("plaid: item requires re-authentication")
	ErrEncryptionFailed   = errors.New("plaid: encryption failed")
	ErrDecryptionFailed   = errors.New("plaid: decryption failed")
	ErrStorageFailure     = errors.New("plaid: storage operation failed")
)

// PlaidError wraps errors from the Plaid API with additional context.
type PlaidError struct {
	// ErrorType is the Plaid error type (e.g., ITEM_ERROR, API_ERROR).
	ErrorType string

	// ErrorCode is the specific Plaid error code.
	ErrorCode string

	// ErrorMessage is the human-readable error message.
	ErrorMessage string

	// DisplayMessage is a user-friendly message (if available).
	DisplayMessage string

	// RequestID is the Plaid request ID for debugging.
	RequestID string

	// Cause is the underlying error.
	Cause error
}

func (e *PlaidError) Error() string {
	if e.DisplayMessage != "" {
		return fmt.Sprintf("plaid: %s - %s (request_id: %s)", e.ErrorCode, e.DisplayMessage, e.RequestID)
	}
	return fmt.Sprintf("plaid: %s - %s (request_id: %s)", e.ErrorCode, e.ErrorMessage, e.RequestID)
}

func (e *PlaidError) Unwrap() error {
	return e.Cause
}

// IsItemError returns true if this is an item-related error (e.g., login required).
func (e *PlaidError) IsItemError() bool {
	return e.ErrorType == "ITEM_ERROR"
}

// IsRateLimitError returns true if this is a rate limit error.
func (e *PlaidError) IsRateLimitError() bool {
	return e.ErrorType == "RATE_LIMIT_EXCEEDED"
}

// NeedsReauthentication returns true if the item needs to be re-authenticated.
func (e *PlaidError) NeedsReauthentication() bool {
	return e.ErrorCode == "ITEM_LOGIN_REQUIRED" ||
		e.ErrorCode == "INVALID_CREDENTIALS" ||
		e.ErrorCode == "MFA_NOT_SUPPORTED"
}
