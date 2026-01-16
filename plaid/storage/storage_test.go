package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/kinected/kinected/plaid/models"
)

func setupTestStorage(t *testing.T) *SQLiteStorage {
	storage, err := NewSQLiteStorage(SQLiteConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	return storage
}

func TestNewSQLiteStorage(t *testing.T) {
	storage := setupTestStorage(t)
	if storage == nil {
		t.Error("expected non-nil storage")
	}
}

// Item tests

func TestSQLiteStorage_CreateGetItem(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	item := &models.Item{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		PlaidItemID:      "plaid-item-123",
		PlaidAccessToken: "encrypted-token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	retrieved, err := storage.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if retrieved.ID != item.ID {
		t.Errorf("ID mismatch: expected %v, got %v", item.ID, retrieved.ID)
	}
	if retrieved.UserID != item.UserID {
		t.Errorf("UserID mismatch")
	}
	if retrieved.PlaidItemID != item.PlaidItemID {
		t.Errorf("PlaidItemID mismatch")
	}
	if retrieved.PlaidAccessToken != item.PlaidAccessToken {
		t.Errorf("PlaidAccessToken mismatch")
	}
	if retrieved.InstitutionID != item.InstitutionID {
		t.Errorf("InstitutionID mismatch")
	}
	if retrieved.Status != item.Status {
		t.Errorf("Status mismatch")
	}
}

func TestSQLiteStorage_GetItemsByUserID(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()
	now := time.Now()

	// Create items for target user
	for i := 0; i < 3; i++ {
		item := &models.Item{
			ID:               uuid.New(),
			UserID:           userID,
			PlaidItemID:      "plaid-" + uuid.NewString()[:8],
			PlaidAccessToken: "token",
			InstitutionID:    "ins_1",
			InstitutionName:  "Test Bank",
			Status:           models.ItemStatusActive,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := storage.CreateItem(ctx, item); err != nil {
			t.Fatalf("failed to create item: %v", err)
		}
	}

	// Create item for different user
	otherItem := &models.Item{
		ID:               uuid.New(),
		UserID:           otherUserID,
		PlaidItemID:      "plaid-other",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_2",
		InstitutionName:  "Other Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, otherItem); err != nil {
		t.Fatalf("failed to create other item: %v", err)
	}

	items, err := storage.GetItemsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get items: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestSQLiteStorage_UpdateItemStatus(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	item := &models.Item{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := storage.UpdateItemStatus(ctx, item.ID, models.ItemStatusLoginRequired, "ITEM_LOGIN_REQUIRED", "Login required"); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	retrieved, err := storage.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("failed to get item: %v", err)
	}

	if retrieved.Status != models.ItemStatusLoginRequired {
		t.Errorf("expected status %v, got %v", models.ItemStatusLoginRequired, retrieved.Status)
	}
	if retrieved.ErrorCode != "ITEM_LOGIN_REQUIRED" {
		t.Errorf("expected error code ITEM_LOGIN_REQUIRED, got %s", retrieved.ErrorCode)
	}
}

func TestSQLiteStorage_DeleteItem(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	item := &models.Item{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	if err := storage.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("failed to delete item: %v", err)
	}

	_, err := storage.GetItem(ctx, item.ID)
	if err != models.ErrItemNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

// Account tests

func TestSQLiteStorage_UpsertGetAccount(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	itemID := uuid.New()
	userID := uuid.New()

	// Create item first
	item := &models.Item{
		ID:               itemID,
		UserID:           userID,
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	current := 1000.50
	available := 900.00
	account := &models.Account{
		ID:               uuid.New(),
		ItemID:           itemID,
		UserID:           userID,
		PlaidAccountID:   "acct-123",
		Name:             "Checking",
		OfficialName:     "Premium Checking",
		Type:             models.AccountTypeDepository,
		Subtype:          "checking",
		Mask:             "1234",
		CurrentBalance:   &current,
		AvailableBalance: &available,
		CurrencyCode:     "USD",
	}

	if err := storage.UpsertAccounts(ctx, []*models.Account{account}); err != nil {
		t.Fatalf("failed to upsert account: %v", err)
	}

	retrieved, err := storage.GetAccount(ctx, account.ID)
	if err != nil {
		t.Fatalf("failed to get account: %v", err)
	}

	if retrieved.ID != account.ID {
		t.Errorf("ID mismatch")
	}
	if retrieved.Name != account.Name {
		t.Errorf("Name mismatch")
	}
	if *retrieved.CurrentBalance != *account.CurrentBalance {
		t.Errorf("CurrentBalance mismatch")
	}
}

func TestSQLiteStorage_GetAccountsByItemID(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	itemID := uuid.New()
	userID := uuid.New()

	// Create item
	item := &models.Item{
		ID:               itemID,
		UserID:           userID,
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	// Create accounts
	accounts := make([]*models.Account, 3)
	for i := 0; i < 3; i++ {
		accounts[i] = &models.Account{
			ID:             uuid.New(),
			ItemID:         itemID,
			UserID:         userID,
			PlaidAccountID: "acct-" + uuid.NewString()[:8],
			Name:           "Account " + string(rune('A'+i)),
			Type:           models.AccountTypeDepository,
			Subtype:        "checking",
		}
	}

	if err := storage.UpsertAccounts(ctx, accounts); err != nil {
		t.Fatalf("failed to upsert accounts: %v", err)
	}

	retrieved, err := storage.GetAccountsByItemID(ctx, itemID)
	if err != nil {
		t.Fatalf("failed to get accounts: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(retrieved))
	}
}

// Transaction tests

func TestSQLiteStorage_UpsertGetTransactions(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	itemID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Create item and account first
	item := &models.Item{
		ID:               itemID,
		UserID:           userID,
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	account := &models.Account{
		ID:             accountID,
		ItemID:         itemID,
		UserID:         userID,
		PlaidAccountID: "acct-123",
		Name:           "Checking",
		Type:           models.AccountTypeDepository,
		Subtype:        "checking",
	}
	if err := storage.UpsertAccounts(ctx, []*models.Account{account}); err != nil {
		t.Fatalf("failed to upsert account: %v", err)
	}

	// Create transactions
	transactions := []*models.Transaction{
		{
			AccountID:          accountID,
			UserID:             userID,
			PlaidTransactionID: "txn-1",
			Amount:             50.00,
			Name:               "Coffee Shop",
			Date:               now.AddDate(0, 0, -1),
			Category:           "Food and Drink",
			Pending:            false,
		},
		{
			AccountID:          accountID,
			UserID:             userID,
			PlaidTransactionID: "txn-2",
			Amount:             25.00,
			Name:               "Gas Station",
			Date:               now,
			Category:           "Travel",
			Pending:            true,
		},
	}

	if err := storage.UpsertTransactions(ctx, transactions); err != nil {
		t.Fatalf("failed to upsert transactions: %v", err)
	}

	dateRange := models.DateRange{
		StartDate: now.AddDate(0, 0, -7),
		EndDate:   now.AddDate(0, 0, 1),
	}

	retrieved, err := storage.GetTransactionsByAccountID(ctx, accountID, dateRange)
	if err != nil {
		t.Fatalf("failed to get transactions: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(retrieved))
	}
}

func TestSQLiteStorage_DeletePendingTransactions(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	itemID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Create item and account
	item := &models.Item{
		ID:               itemID,
		UserID:           userID,
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	account := &models.Account{
		ID:             accountID,
		ItemID:         itemID,
		UserID:         userID,
		PlaidAccountID: "acct-123",
		Name:           "Checking",
		Type:           models.AccountTypeDepository,
	}
	if err := storage.UpsertAccounts(ctx, []*models.Account{account}); err != nil {
		t.Fatalf("failed to upsert account: %v", err)
	}

	// Create a pending transaction
	txn := &models.Transaction{
		AccountID:          accountID,
		UserID:             userID,
		PlaidTransactionID: "pending-txn-1",
		Amount:             100.00,
		Name:               "Pending Purchase",
		Date:               now,
		Pending:            true,
	}
	if err := storage.UpsertTransactions(ctx, []*models.Transaction{txn}); err != nil {
		t.Fatalf("failed to upsert transaction: %v", err)
	}

	// Delete the pending transaction
	if err := storage.DeletePendingTransactions(ctx, []string{"pending-txn-1"}); err != nil {
		t.Fatalf("failed to delete pending transaction: %v", err)
	}

	// Verify it's deleted
	dateRange := models.DateRange{
		StartDate: now.AddDate(0, 0, -1),
		EndDate:   now.AddDate(0, 0, 1),
	}
	transactions, _ := storage.GetTransactionsByAccountID(ctx, accountID, dateRange)
	if len(transactions) != 0 {
		t.Errorf("expected 0 transactions after delete, got %d", len(transactions))
	}
}

// Liability tests

func TestSQLiteStorage_UpsertGetLiability(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	now := time.Now()
	itemID := uuid.New()
	userID := uuid.New()
	accountID := uuid.New()

	// Create item and account
	item := &models.Item{
		ID:               itemID,
		UserID:           userID,
		PlaidItemID:      "plaid-123",
		PlaidAccessToken: "token",
		InstitutionID:    "ins_1",
		InstitutionName:  "Test Bank",
		Status:           models.ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := storage.CreateItem(ctx, item); err != nil {
		t.Fatalf("failed to create item: %v", err)
	}

	account := &models.Account{
		ID:             accountID,
		ItemID:         itemID,
		UserID:         userID,
		PlaidAccountID: "acct-123",
		Name:           "Credit Card",
		Type:           models.AccountTypeCredit,
	}
	if err := storage.UpsertAccounts(ctx, []*models.Account{account}); err != nil {
		t.Fatalf("failed to upsert account: %v", err)
	}

	apr := 19.99
	minPayment := 25.00
	liability := &models.Liability{
		AccountID:            accountID,
		UserID:               userID,
		Type:                 models.LiabilityTypeCredit,
		APRPercentage:        &apr,
		MinimumPaymentAmount: &minPayment,
	}

	if err := storage.UpsertLiability(ctx, liability); err != nil {
		t.Fatalf("failed to upsert liability: %v", err)
	}

	retrieved, err := storage.GetLiabilityByAccountID(ctx, accountID)
	if err != nil {
		t.Fatalf("failed to get liability: %v", err)
	}

	if retrieved.Type != models.LiabilityTypeCredit {
		t.Errorf("Type mismatch")
	}
	if *retrieved.APRPercentage != apr {
		t.Errorf("APRPercentage mismatch")
	}
}

func TestSQLiteStorage_GetItemNotFound(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	_, err := storage.GetItem(ctx, uuid.New())
	if err != models.ErrItemNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestSQLiteStorage_GetAccountNotFound(t *testing.T) {
	storage := setupTestStorage(t)

	ctx := context.Background()
	_, err := storage.GetAccount(ctx, uuid.New())
	if err != models.ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}
