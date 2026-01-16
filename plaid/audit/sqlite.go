package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SQLiteLogger implements Logger using SQLite.
type SQLiteLogger struct {
	db *sql.DB
}

// NewSQLiteLogger creates a new SQLite audit logger.
// It uses the same database connection as the main storage.
func NewSQLiteLogger(db *sql.DB) (*SQLiteLogger, error) {
	logger := &SQLiteLogger{db: db}
	if err := logger.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate audit tables: %w", err)
	}
	return logger, nil
}

// migrate creates the audit log table.
func (l *SQLiteLogger) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		correlation_id TEXT NOT NULL,
		user_id TEXT,
		item_id TEXT,
		operation TEXT NOT NULL,
		status TEXT NOT NULL,
		attempt INTEGER NOT NULL,
		request_id TEXT,
		duration_ns INTEGER NOT NULL,
		error_code TEXT,
		error_type TEXT,
		error_message TEXT,
		metadata TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_audit_logs_correlation_id ON audit_logs(correlation_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_item_id ON audit_logs(item_id);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_operation ON audit_logs(operation);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_status ON audit_logs(status);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
	`

	_, err := l.db.Exec(schema)
	return err
}

// Log writes a log entry to the audit log.
func (l *SQLiteLogger) Log(ctx context.Context, entry *LogEntry) error {
	query := `
		INSERT INTO audit_logs (id, correlation_id, user_id, item_id, operation, status,
			attempt, request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var userID, itemID sql.NullString
	if entry.UserID != nil {
		userID = sql.NullString{String: entry.UserID.String(), Valid: true}
	}
	if entry.ItemID != nil {
		itemID = sql.NullString{String: entry.ItemID.String(), Valid: true}
	}

	_, err := l.db.ExecContext(ctx, query,
		entry.ID.String(),
		entry.CorrelationID.String(),
		userID,
		itemID,
		string(entry.Operation),
		string(entry.Status),
		entry.Attempt,
		nullString(entry.RequestID),
		entry.Duration.Nanoseconds(),
		nullString(entry.ErrorCode),
		nullString(entry.ErrorType),
		nullString(entry.ErrorMessage),
		entry.MetadataJSON(),
		entry.CreatedAt,
	)
	return err
}

// GetByCorrelationID retrieves all log entries for a correlation ID.
func (l *SQLiteLogger) GetByCorrelationID(ctx context.Context, correlationID uuid.UUID) ([]*LogEntry, error) {
	query := `
		SELECT id, correlation_id, user_id, item_id, operation, status, attempt,
			request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at
		FROM audit_logs
		WHERE correlation_id = ?
		ORDER BY attempt ASC
	`

	return l.scanEntries(l.db.QueryContext(ctx, query, correlationID.String()))
}

// GetByUserID retrieves log entries for a user within a time range.
func (l *SQLiteLogger) GetByUserID(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*LogEntry, error) {
	query := `
		SELECT id, correlation_id, user_id, item_id, operation, status, attempt,
			request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at
		FROM audit_logs
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
	`

	return l.scanEntries(l.db.QueryContext(ctx, query, userID.String(), start, end))
}

// GetByItemID retrieves log entries for an item within a time range.
func (l *SQLiteLogger) GetByItemID(ctx context.Context, itemID uuid.UUID, start, end time.Time) ([]*LogEntry, error) {
	query := `
		SELECT id, correlation_id, user_id, item_id, operation, status, attempt,
			request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at
		FROM audit_logs
		WHERE item_id = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
	`

	return l.scanEntries(l.db.QueryContext(ctx, query, itemID.String(), start, end))
}

// GetByOperation retrieves log entries for an operation type within a time range.
func (l *SQLiteLogger) GetByOperation(ctx context.Context, op Operation, start, end time.Time) ([]*LogEntry, error) {
	query := `
		SELECT id, correlation_id, user_id, item_id, operation, status, attempt,
			request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at
		FROM audit_logs
		WHERE operation = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
	`

	return l.scanEntries(l.db.QueryContext(ctx, query, string(op), start, end))
}

// GetFailures retrieves failed log entries within a time range.
func (l *SQLiteLogger) GetFailures(ctx context.Context, start, end time.Time) ([]*LogEntry, error) {
	query := `
		SELECT id, correlation_id, user_id, item_id, operation, status, attempt,
			request_id, duration_ns, error_code, error_type, error_message,
			metadata, created_at
		FROM audit_logs
		WHERE status = ? AND created_at >= ? AND created_at <= ?
		ORDER BY created_at DESC
	`

	return l.scanEntries(l.db.QueryContext(ctx, query, string(StatusFailure), start, end))
}

// Cleanup removes log entries older than the given duration.
func (l *SQLiteLogger) Cleanup(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := l.db.ExecContext(ctx, "DELETE FROM audit_logs WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// scanEntries scans multiple log entries from a query result.
func (l *SQLiteLogger) scanEntries(rows *sql.Rows, err error) ([]*LogEntry, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*LogEntry
	for rows.Next() {
		entry, err := l.scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// scanEntry scans a single log entry from a row.
func (l *SQLiteLogger) scanEntry(rows *sql.Rows) (*LogEntry, error) {
	entry := &LogEntry{}
	var idStr, correlationIDStr string
	var userIDStr, itemIDStr sql.NullString
	var operation, status string
	var requestID, errorCode, errorType, errorMessage sql.NullString
	var metadataStr string
	var durationNS int64

	err := rows.Scan(
		&idStr,
		&correlationIDStr,
		&userIDStr,
		&itemIDStr,
		&operation,
		&status,
		&entry.Attempt,
		&requestID,
		&durationNS,
		&errorCode,
		&errorType,
		&errorMessage,
		&metadataStr,
		&entry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	entry.ID = uuid.MustParse(idStr)
	entry.CorrelationID = uuid.MustParse(correlationIDStr)
	entry.Operation = Operation(operation)
	entry.Status = Status(status)
	entry.Duration = time.Duration(durationNS)
	entry.RequestID = requestID.String
	entry.ErrorCode = errorCode.String
	entry.ErrorType = errorType.String
	entry.ErrorMessage = errorMessage.String

	if userIDStr.Valid {
		id := uuid.MustParse(userIDStr.String)
		entry.UserID = &id
	}
	if itemIDStr.Valid {
		id := uuid.MustParse(itemIDStr.String)
		entry.ItemID = &id
	}

	if err := entry.SetMetadataFromJSON(metadataStr); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return entry, nil
}

// nullString converts a string to a sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
