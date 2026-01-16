package plaid

import (
	"context"
	"time"

	"github.com/google/uuid"
	plaidgo "github.com/plaid/plaid-go/v29/plaid"

	"github.com/kinected/kinected/plaid/audit"
)

// SyncTransactions fetches transactions for an item within the specified date range.
func (c *Client) SyncTransactions(ctx context.Context, itemID uuid.UUID, dateRange DateRange) ([]*Transaction, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	return c.syncTransactions(ctx, item, accessToken, dateRange)
}

// syncTransactions is an internal method that syncs transactions with a pre-decrypted token.
func (c *Client) syncTransactions(ctx context.Context, item *Item, accessToken string, dateRange DateRange) ([]*Transaction, error) {
	// Fetch accounts first to map Plaid account IDs to our account IDs
	accounts, err := c.storage.GetAccountsByItemID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	accountMap := make(map[string]*Account)
	for _, acc := range accounts {
		accountMap[acc.PlaidAccountID] = acc
	}

	startDate := dateRange.StartDate.Format("2006-01-02")
	endDate := dateRange.EndDate.Format("2006-01-02")

	var allTransactions []*Transaction
	var removedTransactionIDs []string

	// Use pagination to fetch all transactions
	offset := int32(0)
	count := int32(500)

	for {
		// Set up call context for audit logging (one per page)
		cc := newCallContext(audit.OpTransactionsGet).
			withUserID(item.UserID).
			withItemID(item.ID).
			withMetadata("start_date", startDate).
			withMetadata("end_date", endDate).
			withMetadata("offset", offset)

		req := plaidgo.NewTransactionsGetRequest(accessToken, startDate, endDate)
		req.SetOptions(plaidgo.TransactionsGetRequestOptions{
			Offset: &offset,
			Count:  &count,
		})

		var resp plaidgo.TransactionsGetResponse
		var requestID string

		err := c.api.call(ctx, cc, func() error {
			var callErr error
			resp, _, callErr = c.plaid.PlaidApi.TransactionsGet(ctx).TransactionsGetRequest(*req).Execute()
			if callErr != nil {
				return handlePlaidError(callErr, ctx)
			}
			requestID = resp.GetRequestId()
			return nil
		}, func() string {
			return requestID
		})

		if err != nil {
			return nil, err
		}

		// Process transactions
		for _, plaidTxn := range resp.GetTransactions() {
			account, ok := accountMap[plaidTxn.GetAccountId()]
			if !ok {
				// Account not found, skip this transaction
				continue
			}

			txn := c.convertPlaidTransaction(account, plaidTxn)
			allTransactions = append(allTransactions, txn)
		}

		// Check if we've fetched all transactions
		totalTransactions := resp.GetTotalTransactions()
		offset += int32(len(resp.GetTransactions()))
		if offset >= totalTransactions {
			break
		}
	}

	// Save transactions
	if len(allTransactions) > 0 {
		if err := c.storage.UpsertTransactions(ctx, allTransactions); err != nil {
			return nil, err
		}
	}

	// Delete removed transactions (pending transactions that have been posted)
	if len(removedTransactionIDs) > 0 {
		if err := c.storage.DeletePendingTransactions(ctx, removedTransactionIDs); err != nil {
			return nil, err
		}
	}

	// Update item last synced time
	now := time.Now()
	c.storage.UpdateItemLastSynced(ctx, item.ID, now)

	return allTransactions, nil
}

// SyncTransactionsByUserID syncs transactions for all items belonging to a user.
func (c *Client) SyncTransactionsByUserID(ctx context.Context, userID uuid.UUID, dateRange DateRange) ([]*Transaction, error) {
	items, err := c.storage.GetItemsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var allTransactions []*Transaction
	for _, item := range items {
		if item.Status != ItemStatusActive {
			continue // Skip items that need re-authentication
		}

		transactions, err := c.SyncTransactions(ctx, item.ID, dateRange)
		if err != nil {
			// Log error but continue with other items
			continue
		}
		allTransactions = append(allTransactions, transactions...)
	}

	return allTransactions, nil
}

// GetTransactions retrieves transactions for an account within the specified date range.
func (c *Client) GetTransactions(ctx context.Context, accountID uuid.UUID, dateRange DateRange) ([]*Transaction, error) {
	return c.storage.GetTransactionsByAccountID(ctx, accountID, dateRange)
}

// GetTransactionsByUserID retrieves transactions for a user within the specified date range.
func (c *Client) GetTransactionsByUserID(ctx context.Context, userID uuid.UUID, dateRange DateRange) ([]*Transaction, error) {
	return c.storage.GetTransactionsByUserID(ctx, userID, dateRange)
}

// convertPlaidTransaction converts a Plaid transaction to our Transaction model.
func (c *Client) convertPlaidTransaction(account *Account, plaidTxn plaidgo.Transaction) *Transaction {
	txn := &Transaction{
		AccountID:          account.ID,
		UserID:             account.UserID,
		PlaidTransactionID: plaidTxn.GetTransactionId(),
		Amount:             plaidTxn.GetAmount(),
		Name:               plaidTxn.GetName(),
		Pending:            plaidTxn.GetPending(),
	}

	// Parse date
	if date, err := time.Parse("2006-01-02", plaidTxn.GetDate()); err == nil {
		txn.Date = date
	}

	// Parse authorized date
	if authDate := plaidTxn.GetAuthorizedDate(); authDate != "" {
		if parsed, err := time.Parse("2006-01-02", authDate); err == nil {
			txn.AuthorizedDate = &parsed
		}
	}

	// Optional fields
	if currency, ok := plaidTxn.GetIsoCurrencyCodeOk(); ok && currency != nil {
		txn.CurrencyCode = *currency
	} else if unofficial, ok := plaidTxn.GetUnofficialCurrencyCodeOk(); ok && unofficial != nil {
		txn.CurrencyCode = *unofficial
	}

	if merchant := plaidTxn.GetMerchantName(); merchant != "" {
		txn.MerchantName = merchant
	}

	// Category (use first category if available)
	if categories := plaidTxn.GetCategory(); len(categories) > 0 {
		txn.Category = categories[0]
	}

	if categoryID := plaidTxn.GetCategoryId(); categoryID != "" {
		txn.CategoryID = categoryID
	}

	if pendingTxnID := plaidTxn.GetPendingTransactionId(); pendingTxnID != "" {
		txn.PendingTransactionID = pendingTxnID
	}

	if paymentChannel := plaidTxn.GetPaymentChannel(); paymentChannel != "" {
		txn.PaymentChannel = string(paymentChannel)
	}

	return txn
}
