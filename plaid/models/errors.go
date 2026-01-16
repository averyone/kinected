package models

import "errors"

// Common errors returned by storage operations.
var (
	ErrItemNotFound    = errors.New("item not found")
	ErrAccountNotFound = errors.New("account not found")
	ErrUserNotFound    = errors.New("user not found")
)
