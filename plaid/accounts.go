package plaid

import (
	"context"
	"time"

	"github.com/google/uuid"
	plaidgo "github.com/plaid/plaid-go/v29/plaid"
)

// SyncAccounts fetches and updates account information for an item.
func (c *Client) SyncAccounts(ctx context.Context, itemID uuid.UUID) ([]*Account, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	return c.syncAccounts(ctx, item, accessToken)
}

// syncAccounts is an internal method that syncs accounts with a pre-decrypted token.
func (c *Client) syncAccounts(ctx context.Context, item *Item, accessToken string) ([]*Account, error) {
	req := plaidgo.NewAccountsGetRequest(accessToken)
	resp, _, err := c.plaid.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*req).Execute()
	if err != nil {
		return nil, handlePlaidError(err, ctx)
	}

	accounts := make([]*Account, 0, len(resp.GetAccounts()))
	for _, plaidAccount := range resp.GetAccounts() {
		account := c.convertPlaidAccount(item, plaidAccount)
		accounts = append(accounts, account)
	}

	if err := c.storage.UpsertAccounts(ctx, accounts); err != nil {
		return nil, err
	}

	return accounts, nil
}

// GetAccounts retrieves all accounts for an item from storage.
func (c *Client) GetAccounts(ctx context.Context, itemID uuid.UUID) ([]*Account, error) {
	return c.storage.GetAccountsByItemID(ctx, itemID)
}

// GetAccountsByUserID retrieves all accounts for a user from storage.
func (c *Client) GetAccountsByUserID(ctx context.Context, userID uuid.UUID) ([]*Account, error) {
	return c.storage.GetAccountsByUserID(ctx, userID)
}

// GetAccount retrieves a single account by ID.
func (c *Client) GetAccount(ctx context.Context, accountID uuid.UUID) (*Account, error) {
	return c.storage.GetAccount(ctx, accountID)
}

// SyncBalances fetches and updates current balances for all accounts in an item.
func (c *Client) SyncBalances(ctx context.Context, itemID uuid.UUID) ([]*Account, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	// Use accounts/balance/get for real-time balances
	req := plaidgo.NewAccountsBalanceGetRequest(accessToken)
	resp, _, err := c.plaid.PlaidApi.AccountsBalanceGet(ctx).AccountsBalanceGetRequest(*req).Execute()
	if err != nil {
		return nil, handlePlaidError(err, ctx)
	}

	accounts := make([]*Account, 0, len(resp.GetAccounts()))
	for _, plaidAccount := range resp.GetAccounts() {
		account := c.convertPlaidAccount(item, plaidAccount)
		accounts = append(accounts, account)
	}

	if err := c.storage.UpsertAccounts(ctx, accounts); err != nil {
		return nil, err
	}

	// Update item last synced time
	now := time.Now()
	c.storage.UpdateItemLastSynced(ctx, itemID, now)

	return accounts, nil
}

// SyncBalancesByUserID syncs balances for all items belonging to a user.
func (c *Client) SyncBalancesByUserID(ctx context.Context, userID uuid.UUID) ([]*Account, error) {
	items, err := c.storage.GetItemsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var allAccounts []*Account
	for _, item := range items {
		if item.Status != ItemStatusActive {
			continue // Skip items that need re-authentication
		}

		accounts, err := c.SyncBalances(ctx, item.ID)
		if err != nil {
			// Log error but continue with other items
			continue
		}
		allAccounts = append(allAccounts, accounts...)
	}

	return allAccounts, nil
}

// convertPlaidAccount converts a Plaid account to our Account model.
func (c *Client) convertPlaidAccount(item *Item, plaidAccount plaidgo.AccountBase) *Account {
	account := &Account{
		ItemID:         item.ID,
		UserID:         item.UserID,
		PlaidAccountID: plaidAccount.GetAccountId(),
		Name:           plaidAccount.GetName(),
		OfficialName:   plaidAccount.GetOfficialName(),
		Type:           AccountType(plaidAccount.GetType()),
		Subtype:        string(plaidAccount.GetSubtype()),
		Mask:           plaidAccount.GetMask(),
	}

	// Handle balances
	balances := plaidAccount.GetBalances()
	if current, ok := balances.GetCurrentOk(); ok && current != nil {
		account.CurrentBalance = current
	}
	if available, ok := balances.GetAvailableOk(); ok && available != nil {
		account.AvailableBalance = available
	}
	if limit, ok := balances.GetLimitOk(); ok && limit != nil {
		account.Limit = limit
	}
	if currency, ok := balances.GetIsoCurrencyCodeOk(); ok && currency != nil {
		account.CurrencyCode = *currency
	} else if unofficial, ok := balances.GetUnofficialCurrencyCodeOk(); ok && unofficial != nil {
		account.CurrencyCode = *unofficial
	}

	return account
}
