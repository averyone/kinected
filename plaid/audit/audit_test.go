package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewLogEntry(t *testing.T) {
	correlationID := uuid.New()
	op := OpAccountsGet
	attempt := 1

	entry := NewLogEntry(correlationID, op, attempt)

	if entry.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if entry.CorrelationID != correlationID {
		t.Errorf("expected CorrelationID=%v, got %v", correlationID, entry.CorrelationID)
	}
	if entry.Operation != op {
		t.Errorf("expected Operation=%v, got %v", op, entry.Operation)
	}
	if entry.Attempt != attempt {
		t.Errorf("expected Attempt=%d, got %d", attempt, entry.Attempt)
	}
	if entry.Status != StatusAttempt {
		t.Errorf("expected Status=%v, got %v", StatusAttempt, entry.Status)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestLogEntry_MarkSuccess(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	requestID := "test-request-id"
	duration := 100 * time.Millisecond

	entry.MarkSuccess(requestID, duration)

	if entry.Status != StatusSuccess {
		t.Errorf("expected Status=%v, got %v", StatusSuccess, entry.Status)
	}
	if entry.RequestID != requestID {
		t.Errorf("expected RequestID=%s, got %s", requestID, entry.RequestID)
	}
	if entry.Duration != duration {
		t.Errorf("expected Duration=%v, got %v", duration, entry.Duration)
	}
}

func TestLogEntry_MarkFailure(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	err := &testError{message: "test error"}
	duration := 100 * time.Millisecond

	entry.MarkFailure(err, duration)

	if entry.Status != StatusFailure {
		t.Errorf("expected Status=%v, got %v", StatusFailure, entry.Status)
	}
	if entry.Duration != duration {
		t.Errorf("expected Duration=%v, got %v", duration, entry.Duration)
	}
	if entry.ErrorMessage != err.Error() {
		t.Errorf("expected ErrorMessage=%s, got %s", err.Error(), entry.ErrorMessage)
	}
}

func TestLogEntry_MarkRetry(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	err := &testError{message: "rate limit exceeded"}
	duration := 100 * time.Millisecond

	entry.MarkRetry(err, duration)

	if entry.Status != StatusRetry {
		t.Errorf("expected Status=%v, got %v", StatusRetry, entry.Status)
	}
	if entry.Duration != duration {
		t.Errorf("expected Duration=%v, got %v", duration, entry.Duration)
	}
}

func TestLogEntry_SetPlaidError(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	errorType := "RATE_LIMIT_EXCEEDED"
	errorCode := "RATE_LIMIT"
	errorMessage := "Rate limit exceeded"
	requestID := "req-123"

	entry.SetPlaidError(errorType, errorCode, errorMessage, requestID)

	if entry.ErrorType != errorType {
		t.Errorf("expected ErrorType=%s, got %s", errorType, entry.ErrorType)
	}
	if entry.ErrorCode != errorCode {
		t.Errorf("expected ErrorCode=%s, got %s", errorCode, entry.ErrorCode)
	}
	if entry.ErrorMessage != errorMessage {
		t.Errorf("expected ErrorMessage=%s, got %s", errorMessage, entry.ErrorMessage)
	}
	if entry.RequestID != requestID {
		t.Errorf("expected RequestID=%s, got %s", requestID, entry.RequestID)
	}
}

func TestLogEntry_MetadataJSON(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	entry.Metadata = map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	jsonStr := entry.MetadataJSON()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}

	if parsed["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", parsed["key1"])
	}
	if parsed["key2"] != float64(42) {
		t.Errorf("expected key2=42, got %v", parsed["key2"])
	}
	if parsed["key3"] != true {
		t.Errorf("expected key3=true, got %v", parsed["key3"])
	}
}

func TestLogEntry_MetadataJSON_Empty(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	jsonStr := entry.MetadataJSON()
	if jsonStr != "{}" {
		t.Errorf("expected empty metadata to be {}, got %s", jsonStr)
	}
}

func TestLogEntry_SetMetadataFromJSON(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	jsonStr := `{"key1": "value1", "key2": 42}`
	if err := entry.SetMetadataFromJSON(jsonStr); err != nil {
		t.Fatalf("failed to set metadata from JSON: %v", err)
	}

	if entry.Metadata["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", entry.Metadata["key1"])
	}
	if entry.Metadata["key2"] != float64(42) {
		t.Errorf("expected key2=42, got %v", entry.Metadata["key2"])
	}
}

func TestLogEntry_SetMetadataFromJSON_Empty(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	if err := entry.SetMetadataFromJSON("{}"); err != nil {
		t.Fatalf("failed to set empty metadata: %v", err)
	}
	if err := entry.SetMetadataFromJSON(""); err != nil {
		t.Fatalf("failed to set empty string metadata: %v", err)
	}
}

func TestLogEntry_SetMetadataFromJSON_Invalid(t *testing.T) {
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)

	err := entry.SetMetadataFromJSON("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNopLogger(t *testing.T) {
	logger := NopLogger{}

	// NopLogger should not error on any operation
	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	if err := logger.Log(nil, entry); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestOperationTypes(t *testing.T) {
	// Ensure all operations are defined
	operations := []Operation{
		OpLinkTokenCreate,
		OpPublicTokenExchange,
		OpItemGet,
		OpItemRemove,
		OpAccountsGet,
		OpAccountsBalanceGet,
		OpTransactionsGet,
		OpLiabilitiesGet,
	}

	for _, op := range operations {
		if op == "" {
			t.Error("operation should not be empty")
		}
	}
}

func TestStatusTypes(t *testing.T) {
	// Ensure all statuses are defined
	statuses := []Status{
		StatusAttempt,
		StatusSuccess,
		StatusFailure,
		StatusRetry,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("status should not be empty")
		}
	}
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
