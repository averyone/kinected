package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/kinected/kinected/plaid/models"
)

// Storage defines the interface for persisting Plaid data.
// Implementations must be safe for concurrent use.
type Storage interface {
	// Item operations
	ItemStore

	// Account operations
	AccountStore

	// Transaction operations
	TransactionStore

	// Liability operations
	LiabilityStore

	// Close releases any resources held by the storage.
	Close() error
}

// ItemStore defines operations for Plaid items.
type ItemStore interface {
	// CreateItem creates a new item.
	CreateItem(ctx context.Context, item *models.Item) error

	// GetItem retrieves an item by ID.
	GetItem(ctx context.Context, id uuid.UUID) (*models.Item, error)

	// GetItemByPlaidID retrieves an item by Plaid's item ID.
	GetItemByPlaidID(ctx context.Context, plaidItemID string) (*models.Item, error)

	// GetItemsByUserID retrieves all items for a user.
	GetItemsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Item, error)

	// UpdateItem updates an existing item.
	UpdateItem(ctx context.Context, item *models.Item) error

	// UpdateItemStatus updates only the status fields of an item.
	UpdateItemStatus(ctx context.Context, id uuid.UUID, status models.ItemStatus, errorCode, errorMessage string) error

	// UpdateItemLastSynced updates the last synced timestamp.
	UpdateItemLastSynced(ctx context.Context, id uuid.UUID, syncedAt time.Time) error

	// DeleteItem deletes an item and all associated data.
	DeleteItem(ctx context.Context, id uuid.UUID) error
}

// AccountStore defines operations for accounts.
type AccountStore interface {
	// UpsertAccounts creates or updates accounts for an item.
	// Uses PlaidAccountID as the unique key for updates.
	UpsertAccounts(ctx context.Context, accounts []*models.Account) error

	// GetAccount retrieves an account by ID.
	GetAccount(ctx context.Context, id uuid.UUID) (*models.Account, error)

	// GetAccountByPlaidID retrieves an account by Plaid's account ID.
	GetAccountByPlaidID(ctx context.Context, plaidAccountID string) (*models.Account, error)

	// GetAccountsByItemID retrieves all accounts for an item.
	GetAccountsByItemID(ctx context.Context, itemID uuid.UUID) ([]*models.Account, error)

	// GetAccountsByUserID retrieves all accounts for a user.
	GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Account, error)

	// DeleteAccountsByItemID deletes all accounts for an item.
	DeleteAccountsByItemID(ctx context.Context, itemID uuid.UUID) error
}

// TransactionStore defines operations for transactions.
type TransactionStore interface {
	// UpsertTransactions creates or updates transactions.
	// Uses PlaidTransactionID as the unique key for updates.
	UpsertTransactions(ctx context.Context, transactions []*models.Transaction) error

	// GetTransaction retrieves a transaction by ID.
	GetTransaction(ctx context.Context, id uuid.UUID) (*models.Transaction, error)

	// GetTransactionsByAccountID retrieves transactions for an account within a date range.
	GetTransactionsByAccountID(ctx context.Context, accountID uuid.UUID, dateRange models.DateRange) ([]*models.Transaction, error)

	// GetTransactionsByUserID retrieves transactions for a user within a date range.
	GetTransactionsByUserID(ctx context.Context, userID uuid.UUID, dateRange models.DateRange) ([]*models.Transaction, error)

	// DeleteTransactionsByAccountID deletes all transactions for an account.
	DeleteTransactionsByAccountID(ctx context.Context, accountID uuid.UUID) error

	// DeletePendingTransactions deletes pending transactions that have been posted.
	// This is used when a transaction moves from pending to posted.
	DeletePendingTransactions(ctx context.Context, pendingIDs []string) error
}

// LiabilityStore defines operations for liabilities.
type LiabilityStore interface {
	// UpsertLiability creates or updates a liability.
	UpsertLiability(ctx context.Context, liability *models.Liability) error

	// GetLiability retrieves a liability by ID.
	GetLiability(ctx context.Context, id uuid.UUID) (*models.Liability, error)

	// GetLiabilityByAccountID retrieves the liability for an account.
	GetLiabilityByAccountID(ctx context.Context, accountID uuid.UUID) (*models.Liability, error)

	// GetLiabilitiesByUserID retrieves all liabilities for a user.
	GetLiabilitiesByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Liability, error)

	// DeleteLiabilityByAccountID deletes the liability for an account.
	DeleteLiabilityByAccountID(ctx context.Context, accountID uuid.UUID) error
}
