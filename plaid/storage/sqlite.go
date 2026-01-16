package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/kinected/kinected/plaid/models"
)

// SQLiteStorage implements Storage using SQLite.
type SQLiteStorage struct {
	db *sql.DB
}

// SQLiteConfig holds configuration for SQLite storage.
type SQLiteConfig struct {
	// Path is the path to the SQLite database file.
	// Use ":memory:" for an in-memory database.
	Path string

	// MaxOpenConns is the maximum number of open connections.
	// Defaults to 1 for SQLite (recommended for write-heavy workloads).
	MaxOpenConns int
}

// NewSQLiteStorage creates a new SQLite storage instance.
func NewSQLiteStorage(cfg SQLiteConfig) (*SQLiteStorage, error) {
	if cfg.Path == "" {
		cfg.Path = "models.db"
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 1
	}

	db, err := sql.Open("sqlite3", cfg.Path+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)

	s := &SQLiteStorage{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return s, nil
}

// migrate creates the database schema.
func (s *SQLiteStorage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS items (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		plaid_item_id TEXT UNIQUE NOT NULL,
		plaid_access_token TEXT NOT NULL,
		institution_id TEXT NOT NULL,
		institution_name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		error_code TEXT,
		error_message TEXT,
		consent_expiration_time DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_synced_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_items_user_id ON items(user_id);
	CREATE INDEX IF NOT EXISTS idx_items_plaid_item_id ON items(plaid_item_id);

	CREATE TABLE IF NOT EXISTS accounts (
		id TEXT PRIMARY KEY,
		item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL,
		plaid_account_id TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		official_name TEXT,
		type TEXT NOT NULL,
		subtype TEXT,
		mask TEXT,
		current_balance REAL,
		available_balance REAL,
		credit_limit REAL,
		currency_code TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_accounts_item_id ON accounts(item_id);
	CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
	CREATE INDEX IF NOT EXISTS idx_accounts_plaid_account_id ON accounts(plaid_account_id);

	CREATE TABLE IF NOT EXISTS transactions (
		id TEXT PRIMARY KEY,
		account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL,
		plaid_transaction_id TEXT UNIQUE NOT NULL,
		amount REAL NOT NULL,
		currency_code TEXT,
		date DATE NOT NULL,
		authorized_date DATE,
		name TEXT NOT NULL,
		merchant_name TEXT,
		category TEXT,
		category_id TEXT,
		pending BOOLEAN NOT NULL DEFAULT 0,
		pending_transaction_id TEXT,
		payment_channel TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id);
	CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
	CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
	CREATE INDEX IF NOT EXISTS idx_transactions_plaid_id ON transactions(plaid_transaction_id);

	CREATE TABLE IF NOT EXISTS liabilities (
		id TEXT PRIMARY KEY,
		account_id TEXT UNIQUE NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL,
		type TEXT NOT NULL,
		last_payment_amount REAL,
		last_payment_date DATE,
		last_statement_balance REAL,
		last_statement_date DATE,
		minimum_payment_amount REAL,
		next_payment_due_date DATE,
		apr_percentage REAL,
		apr_type TEXT,
		origination_date DATE,
		origination_principal REAL,
		interest_rate_percentage REAL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_liabilities_user_id ON liabilities(user_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// ============================================================================
// Item Operations
// ============================================================================

func (s *SQLiteStorage) CreateItem(ctx context.Context, item *models.Item) error {
	query := `
		INSERT INTO items (id, user_id, plaid_item_id, plaid_access_token, institution_id,
			institution_name, status, error_code, error_message, consent_expiration_time,
			created_at, updated_at, last_synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		item.ID.String(),
		item.UserID.String(),
		item.PlaidItemID,
		item.PlaidAccessToken,
		item.InstitutionID,
		item.InstitutionName,
		string(item.Status),
		nullString(item.ErrorCode),
		nullString(item.ErrorMessage),
		nullTime(item.ConsentExpirationTime),
		item.CreatedAt,
		item.UpdatedAt,
		nullTime(item.LastSyncedAt),
	)
	return err
}

func (s *SQLiteStorage) GetItem(ctx context.Context, id uuid.UUID) (*models.Item, error) {
	query := `
		SELECT id, user_id, plaid_item_id, plaid_access_token, institution_id,
			institution_name, status, error_code, error_message, consent_expiration_time,
			created_at, updated_at, last_synced_at
		FROM items WHERE id = ?
	`

	item := &models.Item{}
	var idStr, userIDStr string
	var status string
	var errorCode, errorMessage sql.NullString
	var consentExp, lastSynced sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &userIDStr, &item.PlaidItemID, &item.PlaidAccessToken,
		&item.InstitutionID, &item.InstitutionName, &status,
		&errorCode, &errorMessage, &consentExp,
		&item.CreatedAt, &item.UpdatedAt, &lastSynced,
	)
	if err == sql.ErrNoRows {
		return nil, models.ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}

	item.ID = uuid.MustParse(idStr)
	item.UserID = uuid.MustParse(userIDStr)
	item.Status = models.ItemStatus(status)
	item.ErrorCode = errorCode.String
	item.ErrorMessage = errorMessage.String
	if consentExp.Valid {
		item.ConsentExpirationTime = &consentExp.Time
	}
	if lastSynced.Valid {
		item.LastSyncedAt = &lastSynced.Time
	}

	return item, nil
}

func (s *SQLiteStorage) GetItemByPlaidID(ctx context.Context, plaidItemID string) (*models.Item, error) {
	query := `
		SELECT id, user_id, plaid_item_id, plaid_access_token, institution_id,
			institution_name, status, error_code, error_message, consent_expiration_time,
			created_at, updated_at, last_synced_at
		FROM items WHERE plaid_item_id = ?
	`

	item := &models.Item{}
	var idStr, userIDStr string
	var status string
	var errorCode, errorMessage sql.NullString
	var consentExp, lastSynced sql.NullTime

	err := s.db.QueryRowContext(ctx, query, plaidItemID).Scan(
		&idStr, &userIDStr, &item.PlaidItemID, &item.PlaidAccessToken,
		&item.InstitutionID, &item.InstitutionName, &status,
		&errorCode, &errorMessage, &consentExp,
		&item.CreatedAt, &item.UpdatedAt, &lastSynced,
	)
	if err == sql.ErrNoRows {
		return nil, models.ErrItemNotFound
	}
	if err != nil {
		return nil, err
	}

	item.ID = uuid.MustParse(idStr)
	item.UserID = uuid.MustParse(userIDStr)
	item.Status = models.ItemStatus(status)
	item.ErrorCode = errorCode.String
	item.ErrorMessage = errorMessage.String
	if consentExp.Valid {
		item.ConsentExpirationTime = &consentExp.Time
	}
	if lastSynced.Valid {
		item.LastSyncedAt = &lastSynced.Time
	}

	return item, nil
}

func (s *SQLiteStorage) GetItemsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Item, error) {
	query := `
		SELECT id, user_id, plaid_item_id, plaid_access_token, institution_id,
			institution_name, status, error_code, error_message, consent_expiration_time,
			created_at, updated_at, last_synced_at
		FROM items WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.Item
	for rows.Next() {
		item := &models.Item{}
		var idStr, userIDStr string
		var status string
		var errorCode, errorMessage sql.NullString
		var consentExp, lastSynced sql.NullTime

		err := rows.Scan(
			&idStr, &userIDStr, &item.PlaidItemID, &item.PlaidAccessToken,
			&item.InstitutionID, &item.InstitutionName, &status,
			&errorCode, &errorMessage, &consentExp,
			&item.CreatedAt, &item.UpdatedAt, &lastSynced,
		)
		if err != nil {
			return nil, err
		}

		item.ID = uuid.MustParse(idStr)
		item.UserID = uuid.MustParse(userIDStr)
		item.Status = models.ItemStatus(status)
		item.ErrorCode = errorCode.String
		item.ErrorMessage = errorMessage.String
		if consentExp.Valid {
			item.ConsentExpirationTime = &consentExp.Time
		}
		if lastSynced.Valid {
			item.LastSyncedAt = &lastSynced.Time
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (s *SQLiteStorage) UpdateItem(ctx context.Context, item *models.Item) error {
	query := `
		UPDATE items SET
			plaid_access_token = ?, institution_id = ?, institution_name = ?,
			status = ?, error_code = ?, error_message = ?,
			consent_expiration_time = ?, updated_at = ?, last_synced_at = ?
		WHERE id = ?
	`

	item.UpdatedAt = time.Now()

	result, err := s.db.ExecContext(ctx, query,
		item.PlaidAccessToken, item.InstitutionID, item.InstitutionName,
		string(item.Status), nullString(item.ErrorCode), nullString(item.ErrorMessage),
		nullTime(item.ConsentExpirationTime), item.UpdatedAt, nullTime(item.LastSyncedAt),
		item.ID.String(),
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

func (s *SQLiteStorage) UpdateItemStatus(ctx context.Context, id uuid.UUID, status models.ItemStatus, errorCode, errorMessage string) error {
	query := `
		UPDATE items SET status = ?, error_code = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := s.db.ExecContext(ctx, query,
		string(status), nullString(errorCode), nullString(errorMessage), time.Now(), id.String(),
	)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

func (s *SQLiteStorage) UpdateItemLastSynced(ctx context.Context, id uuid.UUID, syncedAt time.Time) error {
	query := `UPDATE items SET last_synced_at = ?, updated_at = ? WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, syncedAt, time.Now(), id.String())
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

func (s *SQLiteStorage) DeleteItem(ctx context.Context, id uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM items WHERE id = ?", id.String())
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return models.ErrItemNotFound
	}

	return nil
}

// ============================================================================
// Account Operations
// ============================================================================

func (s *SQLiteStorage) UpsertAccounts(ctx context.Context, accounts []*models.Account) error {
	query := `
		INSERT INTO accounts (id, item_id, user_id, plaid_account_id, name, official_name,
			type, subtype, mask, current_balance, available_balance, credit_limit,
			currency_code, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plaid_account_id) DO UPDATE SET
			name = excluded.name,
			official_name = excluded.official_name,
			current_balance = excluded.current_balance,
			available_balance = excluded.available_balance,
			credit_limit = excluded.credit_limit,
			currency_code = excluded.currency_code,
			updated_at = excluded.updated_at
	`

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, acc := range accounts {
		if acc.ID == uuid.Nil {
			acc.ID = uuid.New()
		}
		acc.UpdatedAt = now
		if acc.CreatedAt.IsZero() {
			acc.CreatedAt = now
		}

		_, err := stmt.ExecContext(ctx,
			acc.ID.String(), acc.ItemID.String(), acc.UserID.String(),
			acc.PlaidAccountID, acc.Name, nullString(acc.OfficialName),
			string(acc.Type), nullString(acc.Subtype), nullString(acc.Mask),
			nullFloat(acc.CurrentBalance), nullFloat(acc.AvailableBalance),
			nullFloat(acc.Limit), nullString(acc.CurrencyCode),
			acc.CreatedAt, acc.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStorage) GetAccount(ctx context.Context, id uuid.UUID) (*models.Account, error) {
	query := `
		SELECT id, item_id, user_id, plaid_account_id, name, official_name,
			type, subtype, mask, current_balance, available_balance, credit_limit,
			currency_code, created_at, updated_at
		FROM accounts WHERE id = ?
	`

	return s.scanAccount(s.db.QueryRowContext(ctx, query, id.String()))
}

func (s *SQLiteStorage) GetAccountByPlaidID(ctx context.Context, plaidAccountID string) (*models.Account, error) {
	query := `
		SELECT id, item_id, user_id, plaid_account_id, name, official_name,
			type, subtype, mask, current_balance, available_balance, credit_limit,
			currency_code, created_at, updated_at
		FROM accounts WHERE plaid_account_id = ?
	`

	return s.scanAccount(s.db.QueryRowContext(ctx, query, plaidAccountID))
}

func (s *SQLiteStorage) GetAccountsByItemID(ctx context.Context, itemID uuid.UUID) ([]*models.Account, error) {
	query := `
		SELECT id, item_id, user_id, plaid_account_id, name, official_name,
			type, subtype, mask, current_balance, available_balance, credit_limit,
			currency_code, created_at, updated_at
		FROM accounts WHERE item_id = ?
		ORDER BY name
	`

	return s.scanAccounts(s.db.QueryContext(ctx, query, itemID.String()))
}

func (s *SQLiteStorage) GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Account, error) {
	query := `
		SELECT id, item_id, user_id, plaid_account_id, name, official_name,
			type, subtype, mask, current_balance, available_balance, credit_limit,
			currency_code, created_at, updated_at
		FROM accounts WHERE user_id = ?
		ORDER BY name
	`

	return s.scanAccounts(s.db.QueryContext(ctx, query, userID.String()))
}

func (s *SQLiteStorage) DeleteAccountsByItemID(ctx context.Context, itemID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE item_id = ?", itemID.String())
	return err
}

func (s *SQLiteStorage) scanAccount(row *sql.Row) (*models.Account, error) {
	acc := &models.Account{}
	var idStr, itemIDStr, userIDStr string
	var accType string
	var officialName, subtype, mask, currencyCode sql.NullString
	var currentBal, availBal, limit sql.NullFloat64

	err := row.Scan(
		&idStr, &itemIDStr, &userIDStr, &acc.PlaidAccountID,
		&acc.Name, &officialName, &accType, &subtype, &mask,
		&currentBal, &availBal, &limit, &currencyCode,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, models.ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}

	acc.ID = uuid.MustParse(idStr)
	acc.ItemID = uuid.MustParse(itemIDStr)
	acc.UserID = uuid.MustParse(userIDStr)
	acc.Type = models.AccountType(accType)
	acc.OfficialName = officialName.String
	acc.Subtype = subtype.String
	acc.Mask = mask.String
	acc.CurrencyCode = currencyCode.String
	if currentBal.Valid {
		acc.CurrentBalance = &currentBal.Float64
	}
	if availBal.Valid {
		acc.AvailableBalance = &availBal.Float64
	}
	if limit.Valid {
		acc.Limit = &limit.Float64
	}

	return acc, nil
}

func (s *SQLiteStorage) scanAccounts(rows *sql.Rows, err error) ([]*models.Account, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*models.Account
	for rows.Next() {
		acc := &models.Account{}
		var idStr, itemIDStr, userIDStr string
		var accType string
		var officialName, subtype, mask, currencyCode sql.NullString
		var currentBal, availBal, limit sql.NullFloat64

		err := rows.Scan(
			&idStr, &itemIDStr, &userIDStr, &acc.PlaidAccountID,
			&acc.Name, &officialName, &accType, &subtype, &mask,
			&currentBal, &availBal, &limit, &currencyCode,
			&acc.CreatedAt, &acc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		acc.ID = uuid.MustParse(idStr)
		acc.ItemID = uuid.MustParse(itemIDStr)
		acc.UserID = uuid.MustParse(userIDStr)
		acc.Type = models.AccountType(accType)
		acc.OfficialName = officialName.String
		acc.Subtype = subtype.String
		acc.Mask = mask.String
		acc.CurrencyCode = currencyCode.String
		if currentBal.Valid {
			acc.CurrentBalance = &currentBal.Float64
		}
		if availBal.Valid {
			acc.AvailableBalance = &availBal.Float64
		}
		if limit.Valid {
			acc.Limit = &limit.Float64
		}

		accounts = append(accounts, acc)
	}

	return accounts, rows.Err()
}

// ============================================================================
// Transaction Operations
// ============================================================================

func (s *SQLiteStorage) UpsertTransactions(ctx context.Context, transactions []*models.Transaction) error {
	query := `
		INSERT INTO transactions (id, account_id, user_id, plaid_transaction_id, amount,
			currency_code, date, authorized_date, name, merchant_name, category, category_id,
			pending, pending_transaction_id, payment_channel, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plaid_transaction_id) DO UPDATE SET
			amount = excluded.amount,
			name = excluded.name,
			merchant_name = excluded.merchant_name,
			category = excluded.category,
			category_id = excluded.category_id,
			pending = excluded.pending,
			payment_channel = excluded.payment_channel,
			updated_at = excluded.updated_at
	`

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, txn := range transactions {
		if txn.ID == uuid.Nil {
			txn.ID = uuid.New()
		}
		txn.UpdatedAt = now
		if txn.CreatedAt.IsZero() {
			txn.CreatedAt = now
		}

		_, err := stmt.ExecContext(ctx,
			txn.ID.String(), txn.AccountID.String(), txn.UserID.String(),
			txn.PlaidTransactionID, txn.Amount, nullString(txn.CurrencyCode),
			txn.Date, nullTime(txn.AuthorizedDate), txn.Name,
			nullString(txn.MerchantName), nullString(txn.Category),
			nullString(txn.CategoryID), txn.Pending,
			nullString(txn.PendingTransactionID), nullString(txn.PaymentChannel),
			txn.CreatedAt, txn.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *SQLiteStorage) GetTransaction(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	query := `
		SELECT id, account_id, user_id, plaid_transaction_id, amount, currency_code,
			date, authorized_date, name, merchant_name, category, category_id,
			pending, pending_transaction_id, payment_channel, created_at, updated_at
		FROM transactions WHERE id = ?
	`

	return s.scanTransaction(s.db.QueryRowContext(ctx, query, id.String()))
}

func (s *SQLiteStorage) GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID, dateRange models.DateRange) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, user_id, plaid_transaction_id, amount, currency_code,
			date, authorized_date, name, merchant_name, category, category_id,
			pending, pending_transaction_id, payment_channel, created_at, updated_at
		FROM transactions
		WHERE account_id = ? AND date >= ? AND date <= ?
		ORDER BY date DESC
	`

	return s.scanTransactions(s.db.QueryContext(ctx, query, accountID.String(), dateRange.StartDate, dateRange.EndDate))
}

func (s *SQLiteStorage) GetTransactionsByUserID(ctx context.Context, userID uuid.UUID, dateRange models.DateRange) ([]*models.Transaction, error) {
	query := `
		SELECT id, account_id, user_id, plaid_transaction_id, amount, currency_code,
			date, authorized_date, name, merchant_name, category, category_id,
			pending, pending_transaction_id, payment_channel, created_at, updated_at
		FROM transactions
		WHERE user_id = ? AND date >= ? AND date <= ?
		ORDER BY date DESC
	`

	return s.scanTransactions(s.db.QueryContext(ctx, query, userID.String(), dateRange.StartDate, dateRange.EndDate))
}

func (s *SQLiteStorage) DeleteTransactionsByAccountID(ctx context.Context, accountID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM transactions WHERE account_id = ?", accountID.String())
	return err
}

func (s *SQLiteStorage) DeletePendingTransactions(ctx context.Context, pendingIDs []string) error {
	if len(pendingIDs) == 0 {
		return nil
	}

	// Build placeholders
	placeholders := "?"
	for i := 1; i < len(pendingIDs); i++ {
		placeholders += ",?"
	}

	query := fmt.Sprintf("DELETE FROM transactions WHERE plaid_transaction_id IN (%s)", placeholders)

	args := make([]interface{}, len(pendingIDs))
	for i, id := range pendingIDs {
		args[i] = id
	}

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *SQLiteStorage) scanTransaction(row *sql.Row) (*models.Transaction, error) {
	txn := &models.Transaction{}
	var idStr, accountIDStr, userIDStr string
	var currencyCode, merchantName, category, categoryID, pendingTxnID, paymentChannel sql.NullString
	var authorizedDate sql.NullTime

	err := row.Scan(
		&idStr, &accountIDStr, &userIDStr, &txn.PlaidTransactionID,
		&txn.Amount, &currencyCode, &txn.Date, &authorizedDate,
		&txn.Name, &merchantName, &category, &categoryID,
		&txn.Pending, &pendingTxnID, &paymentChannel,
		&txn.CreatedAt, &txn.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		return nil, err
	}

	txn.ID = uuid.MustParse(idStr)
	txn.AccountID = uuid.MustParse(accountIDStr)
	txn.UserID = uuid.MustParse(userIDStr)
	txn.CurrencyCode = currencyCode.String
	txn.MerchantName = merchantName.String
	txn.Category = category.String
	txn.CategoryID = categoryID.String
	txn.PendingTransactionID = pendingTxnID.String
	txn.PaymentChannel = paymentChannel.String
	if authorizedDate.Valid {
		txn.AuthorizedDate = &authorizedDate.Time
	}

	return txn, nil
}

func (s *SQLiteStorage) scanTransactions(rows *sql.Rows, err error) ([]*models.Transaction, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		txn := &models.Transaction{}
		var idStr, accountIDStr, userIDStr string
		var currencyCode, merchantName, category, categoryID, pendingTxnID, paymentChannel sql.NullString
		var authorizedDate sql.NullTime

		err := rows.Scan(
			&idStr, &accountIDStr, &userIDStr, &txn.PlaidTransactionID,
			&txn.Amount, &currencyCode, &txn.Date, &authorizedDate,
			&txn.Name, &merchantName, &category, &categoryID,
			&txn.Pending, &pendingTxnID, &paymentChannel,
			&txn.CreatedAt, &txn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		txn.ID = uuid.MustParse(idStr)
		txn.AccountID = uuid.MustParse(accountIDStr)
		txn.UserID = uuid.MustParse(userIDStr)
		txn.CurrencyCode = currencyCode.String
		txn.MerchantName = merchantName.String
		txn.Category = category.String
		txn.CategoryID = categoryID.String
		txn.PendingTransactionID = pendingTxnID.String
		txn.PaymentChannel = paymentChannel.String
		if authorizedDate.Valid {
			txn.AuthorizedDate = &authorizedDate.Time
		}

		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}

// ============================================================================
// Liability Operations
// ============================================================================

func (s *SQLiteStorage) UpsertLiability(ctx context.Context, liability *models.Liability) error {
	query := `
		INSERT INTO liabilities (id, account_id, user_id, type, last_payment_amount,
			last_payment_date, last_statement_balance, last_statement_date,
			minimum_payment_amount, next_payment_due_date, apr_percentage, apr_type,
			origination_date, origination_principal, interest_rate_percentage,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
			last_payment_amount = excluded.last_payment_amount,
			last_payment_date = excluded.last_payment_date,
			last_statement_balance = excluded.last_statement_balance,
			last_statement_date = excluded.last_statement_date,
			minimum_payment_amount = excluded.minimum_payment_amount,
			next_payment_due_date = excluded.next_payment_due_date,
			apr_percentage = excluded.apr_percentage,
			apr_type = excluded.apr_type,
			interest_rate_percentage = excluded.interest_rate_percentage,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	if liability.ID == uuid.Nil {
		liability.ID = uuid.New()
	}
	liability.UpdatedAt = now
	if liability.CreatedAt.IsZero() {
		liability.CreatedAt = now
	}

	_, err := s.db.ExecContext(ctx, query,
		liability.ID.String(), liability.AccountID.String(), liability.UserID.String(),
		string(liability.Type), nullFloat(liability.LastPaymentAmount),
		nullTime(liability.LastPaymentDate), nullFloat(liability.LastStatementBalance),
		nullTime(liability.LastStatementDate), nullFloat(liability.MinimumPaymentAmount),
		nullTime(liability.NextPaymentDueDate), nullFloat(liability.APRPercentage),
		nullString(liability.APRType), nullTime(liability.OriginationDate),
		nullFloat(liability.OriginationPrincipal), nullFloat(liability.InterestRatePercentage),
		liability.CreatedAt, liability.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetLiability(ctx context.Context, id uuid.UUID) (*models.Liability, error) {
	query := `
		SELECT id, account_id, user_id, type, last_payment_amount, last_payment_date,
			last_statement_balance, last_statement_date, minimum_payment_amount,
			next_payment_due_date, apr_percentage, apr_type, origination_date,
			origination_principal, interest_rate_percentage, created_at, updated_at
		FROM liabilities WHERE id = ?
	`

	return s.scanLiability(s.db.QueryRowContext(ctx, query, id.String()))
}

func (s *SQLiteStorage) GetLiabilityByAccountID(ctx context.Context, accountID uuid.UUID) (*models.Liability, error) {
	query := `
		SELECT id, account_id, user_id, type, last_payment_amount, last_payment_date,
			last_statement_balance, last_statement_date, minimum_payment_amount,
			next_payment_due_date, apr_percentage, apr_type, origination_date,
			origination_principal, interest_rate_percentage, created_at, updated_at
		FROM liabilities WHERE account_id = ?
	`

	return s.scanLiability(s.db.QueryRowContext(ctx, query, accountID.String()))
}

func (s *SQLiteStorage) GetLiabilitiesByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Liability, error) {
	query := `
		SELECT id, account_id, user_id, type, last_payment_amount, last_payment_date,
			last_statement_balance, last_statement_date, minimum_payment_amount,
			next_payment_due_date, apr_percentage, apr_type, origination_date,
			origination_principal, interest_rate_percentage, created_at, updated_at
		FROM liabilities WHERE user_id = ?
	`

	return s.scanLiabilities(s.db.QueryContext(ctx, query, userID.String()))
}

func (s *SQLiteStorage) DeleteLiabilityByAccountID(ctx context.Context, accountID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM liabilities WHERE account_id = ?", accountID.String())
	return err
}

func (s *SQLiteStorage) scanLiability(row *sql.Row) (*models.Liability, error) {
	l := &models.Liability{}
	var idStr, accountIDStr, userIDStr string
	var liabilityType string
	var lastPaymentAmt, lastStatementBal, minPaymentAmt, apr, originationPrincipal, interestRate sql.NullFloat64
	var lastPaymentDate, lastStatementDate, nextPaymentDate, originationDate sql.NullTime
	var aprType sql.NullString

	err := row.Scan(
		&idStr, &accountIDStr, &userIDStr, &liabilityType,
		&lastPaymentAmt, &lastPaymentDate, &lastStatementBal, &lastStatementDate,
		&minPaymentAmt, &nextPaymentDate, &apr, &aprType,
		&originationDate, &originationPrincipal, &interestRate,
		&l.CreatedAt, &l.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("liability not found")
	}
	if err != nil {
		return nil, err
	}

	l.ID = uuid.MustParse(idStr)
	l.AccountID = uuid.MustParse(accountIDStr)
	l.UserID = uuid.MustParse(userIDStr)
	l.Type = models.LiabilityType(liabilityType)
	l.APRType = aprType.String

	if lastPaymentAmt.Valid {
		l.LastPaymentAmount = &lastPaymentAmt.Float64
	}
	if lastStatementBal.Valid {
		l.LastStatementBalance = &lastStatementBal.Float64
	}
	if minPaymentAmt.Valid {
		l.MinimumPaymentAmount = &minPaymentAmt.Float64
	}
	if apr.Valid {
		l.APRPercentage = &apr.Float64
	}
	if originationPrincipal.Valid {
		l.OriginationPrincipal = &originationPrincipal.Float64
	}
	if interestRate.Valid {
		l.InterestRatePercentage = &interestRate.Float64
	}
	if lastPaymentDate.Valid {
		l.LastPaymentDate = &lastPaymentDate.Time
	}
	if lastStatementDate.Valid {
		l.LastStatementDate = &lastStatementDate.Time
	}
	if nextPaymentDate.Valid {
		l.NextPaymentDueDate = &nextPaymentDate.Time
	}
	if originationDate.Valid {
		l.OriginationDate = &originationDate.Time
	}

	return l, nil
}

func (s *SQLiteStorage) scanLiabilities(rows *sql.Rows, err error) ([]*models.Liability, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var liabilities []*models.Liability
	for rows.Next() {
		l := &models.Liability{}
		var idStr, accountIDStr, userIDStr string
		var liabilityType string
		var lastPaymentAmt, lastStatementBal, minPaymentAmt, apr, originationPrincipal, interestRate sql.NullFloat64
		var lastPaymentDate, lastStatementDate, nextPaymentDate, originationDate sql.NullTime
		var aprType sql.NullString

		err := rows.Scan(
			&idStr, &accountIDStr, &userIDStr, &liabilityType,
			&lastPaymentAmt, &lastPaymentDate, &lastStatementBal, &lastStatementDate,
			&minPaymentAmt, &nextPaymentDate, &apr, &aprType,
			&originationDate, &originationPrincipal, &interestRate,
			&l.CreatedAt, &l.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		l.ID = uuid.MustParse(idStr)
		l.AccountID = uuid.MustParse(accountIDStr)
		l.UserID = uuid.MustParse(userIDStr)
		l.Type = models.LiabilityType(liabilityType)
		l.APRType = aprType.String

		if lastPaymentAmt.Valid {
			l.LastPaymentAmount = &lastPaymentAmt.Float64
		}
		if lastStatementBal.Valid {
			l.LastStatementBalance = &lastStatementBal.Float64
		}
		if minPaymentAmt.Valid {
			l.MinimumPaymentAmount = &minPaymentAmt.Float64
		}
		if apr.Valid {
			l.APRPercentage = &apr.Float64
		}
		if originationPrincipal.Valid {
			l.OriginationPrincipal = &originationPrincipal.Float64
		}
		if interestRate.Valid {
			l.InterestRatePercentage = &interestRate.Float64
		}
		if lastPaymentDate.Valid {
			l.LastPaymentDate = &lastPaymentDate.Time
		}
		if lastStatementDate.Valid {
			l.LastStatementDate = &lastStatementDate.Time
		}
		if nextPaymentDate.Valid {
			l.NextPaymentDueDate = &nextPaymentDate.Time
		}
		if originationDate.Valid {
			l.OriginationDate = &originationDate.Time
		}

		liabilities = append(liabilities, l)
	}

	return liabilities, rows.Err()
}

// ============================================================================
// Helper functions
// ============================================================================

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullFloat(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
