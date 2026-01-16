package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	return db
}

func TestNewSQLiteLogger(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestSQLiteLogger_Log(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()
	userID := uuid.New()
	itemID := uuid.New()

	entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	entry.UserID = &userID
	entry.ItemID = &itemID
	entry.Metadata = map[string]interface{}{"test": "value"}
	entry.MarkSuccess("req-123", 100*time.Millisecond)

	if err := logger.Log(ctx, entry); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// Verify the entry was inserted
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM audit_logs")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count logs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 log entry, got %d", count)
	}
}

func TestSQLiteLogger_GetByCorrelationID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()
	correlationID := uuid.New()

	// Create multiple entries with same correlation ID
	for i := 1; i <= 3; i++ {
		entry := NewLogEntry(correlationID, OpAccountsGet, i)
		entry.MarkSuccess("req-"+string(rune('0'+i)), time.Duration(i)*time.Millisecond)
		if err := logger.Log(ctx, entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}

	// Create an entry with different correlation ID
	otherEntry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	if err := logger.Log(ctx, otherEntry); err != nil {
		t.Fatalf("failed to log other entry: %v", err)
	}

	entries, err := logger.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		t.Fatalf("failed to get by correlation ID: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify ordering by attempt
	for i, entry := range entries {
		if entry.Attempt != i+1 {
			t.Errorf("expected attempt %d, got %d", i+1, entry.Attempt)
		}
	}
}

func TestSQLiteLogger_GetByUserID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()

	// Create entries for the target user
	for i := 0; i < 3; i++ {
		entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
		entry.UserID = &userID
		if err := logger.Log(ctx, entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}

	// Create entry for different user
	otherEntry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	otherEntry.UserID = &otherUserID
	if err := logger.Log(ctx, otherEntry); err != nil {
		t.Fatalf("failed to log other entry: %v", err)
	}

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	entries, err := logger.GetByUserID(ctx, userID, start, end)
	if err != nil {
		t.Fatalf("failed to get by user ID: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestSQLiteLogger_GetByItemID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()
	itemID := uuid.New()

	// Create entries for the target item
	for i := 0; i < 2; i++ {
		entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
		entry.ItemID = &itemID
		if err := logger.Log(ctx, entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	entries, err := logger.GetByItemID(ctx, itemID, start, end)
	if err != nil {
		t.Fatalf("failed to get by item ID: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSQLiteLogger_GetByOperation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Create entries with different operations
	ops := []Operation{OpAccountsGet, OpAccountsGet, OpTransactionsGet}
	for _, op := range ops {
		entry := NewLogEntry(uuid.New(), op, 1)
		if err := logger.Log(ctx, entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	entries, err := logger.GetByOperation(ctx, OpAccountsGet, start, end)
	if err != nil {
		t.Fatalf("failed to get by operation: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries for AccountsGet, got %d", len(entries))
	}
}

func TestSQLiteLogger_GetFailures(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Create entries with different statuses
	entry1 := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	entry1.MarkSuccess("req-1", time.Millisecond)
	if err := logger.Log(ctx, entry1); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	entry2 := NewLogEntry(uuid.New(), OpAccountsGet, 1)
	entry2.MarkFailure(&testError{message: "error"}, time.Millisecond)
	if err := logger.Log(ctx, entry2); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	entry3 := NewLogEntry(uuid.New(), OpTransactionsGet, 1)
	entry3.MarkFailure(&testError{message: "another error"}, time.Millisecond)
	if err := logger.Log(ctx, entry3); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	entries, err := logger.GetFailures(ctx, start, end)
	if err != nil {
		t.Fatalf("failed to get failures: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 failure entries, got %d", len(entries))
	}
}

func TestSQLiteLogger_Cleanup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()

	// Create some entries
	for i := 0; i < 5; i++ {
		entry := NewLogEntry(uuid.New(), OpAccountsGet, 1)
		if err := logger.Log(ctx, entry); err != nil {
			t.Fatalf("failed to log entry: %v", err)
		}
	}

	// Cleanup entries older than 1 hour (should keep all since they're new)
	deleted, err := logger.Cleanup(ctx, time.Hour)
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}

	// Verify entries still exist
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM audit_logs")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 entries, got %d", count)
	}
}

func TestSQLiteLogger_RoundTrip(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	logger, err := NewSQLiteLogger(db)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	ctx := context.Background()
	correlationID := uuid.New()
	userID := uuid.New()
	itemID := uuid.New()

	// Create a full entry with all fields populated
	entry := NewLogEntry(correlationID, OpTransactionsGet, 2)
	entry.UserID = &userID
	entry.ItemID = &itemID
	entry.SetPlaidError("RATE_LIMIT_EXCEEDED", "RATE_LIMIT", "Too many requests", "req-456")
	entry.Metadata = map[string]interface{}{
		"start_date": "2024-01-01",
		"end_date":   "2024-01-31",
	}
	// Set status and duration directly to preserve SetPlaidError values
	entry.Status = StatusRetry
	entry.Duration = 500 * time.Millisecond

	if err := logger.Log(ctx, entry); err != nil {
		t.Fatalf("failed to log entry: %v", err)
	}

	// Retrieve the entry
	entries, err := logger.GetByCorrelationID(ctx, correlationID)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	retrieved := entries[0]

	// Verify all fields
	if retrieved.ID != entry.ID {
		t.Errorf("ID mismatch: expected %v, got %v", entry.ID, retrieved.ID)
	}
	if retrieved.CorrelationID != correlationID {
		t.Errorf("CorrelationID mismatch")
	}
	if retrieved.UserID == nil || *retrieved.UserID != userID {
		t.Errorf("UserID mismatch")
	}
	if retrieved.ItemID == nil || *retrieved.ItemID != itemID {
		t.Errorf("ItemID mismatch")
	}
	if retrieved.Operation != OpTransactionsGet {
		t.Errorf("Operation mismatch: expected %v, got %v", OpTransactionsGet, retrieved.Operation)
	}
	if retrieved.Status != StatusRetry {
		t.Errorf("Status mismatch: expected %v, got %v", StatusRetry, retrieved.Status)
	}
	if retrieved.Attempt != 2 {
		t.Errorf("Attempt mismatch: expected 2, got %d", retrieved.Attempt)
	}
	if retrieved.RequestID != "req-456" {
		t.Errorf("RequestID mismatch: expected req-456, got %s", retrieved.RequestID)
	}
	if retrieved.ErrorType != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("ErrorType mismatch")
	}
	if retrieved.ErrorCode != "RATE_LIMIT" {
		t.Errorf("ErrorCode mismatch")
	}
	if retrieved.ErrorMessage != "Too many requests" {
		t.Errorf("ErrorMessage mismatch")
	}
	if retrieved.Metadata["start_date"] != "2024-01-01" {
		t.Errorf("Metadata mismatch")
	}
}
