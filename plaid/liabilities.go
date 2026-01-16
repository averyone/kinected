package plaid

import (
	"context"
	"time"

	"github.com/google/uuid"
	plaidgo "github.com/plaid/plaid-go/v29/plaid"
)

// SyncLiabilities fetches and updates liability information for an item.
func (c *Client) SyncLiabilities(ctx context.Context, itemID uuid.UUID) ([]*Liability, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	return c.syncLiabilities(ctx, item, accessToken)
}

// syncLiabilities is an internal method that syncs liabilities with a pre-decrypted token.
func (c *Client) syncLiabilities(ctx context.Context, item *Item, accessToken string) ([]*Liability, error) {
	// Fetch accounts first to map Plaid account IDs to our account IDs
	accounts, err := c.storage.GetAccountsByItemID(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	accountMap := make(map[string]*Account)
	for _, acc := range accounts {
		accountMap[acc.PlaidAccountID] = acc
	}

	req := plaidgo.NewLiabilitiesGetRequest(accessToken)
	resp, _, err := c.plaid.PlaidApi.LiabilitiesGet(ctx).LiabilitiesGetRequest(*req).Execute()
	if err != nil {
		return nil, handlePlaidError(err, ctx)
	}

	var allLiabilities []*Liability

	liabilities := resp.GetLiabilities()

	// Process credit card liabilities
	for _, creditCard := range liabilities.GetCredit() {
		account, ok := accountMap[creditCard.GetAccountId()]
		if !ok {
			continue
		}

		liability := c.convertCreditCardLiability(account, creditCard)
		if err := c.storage.UpsertLiability(ctx, liability); err != nil {
			return nil, err
		}
		allLiabilities = append(allLiabilities, liability)
	}

	// Process mortgage liabilities
	for _, mortgage := range liabilities.GetMortgage() {
		account, ok := accountMap[mortgage.GetAccountId()]
		if !ok {
			continue
		}

		liability := c.convertMortgageLiability(account, mortgage)
		if err := c.storage.UpsertLiability(ctx, liability); err != nil {
			return nil, err
		}
		allLiabilities = append(allLiabilities, liability)
	}

	// Process student loan liabilities
	for _, student := range liabilities.GetStudent() {
		account, ok := accountMap[student.GetAccountId()]
		if !ok {
			continue
		}

		liability := c.convertStudentLoanLiability(account, student)
		if err := c.storage.UpsertLiability(ctx, liability); err != nil {
			return nil, err
		}
		allLiabilities = append(allLiabilities, liability)
	}

	// Update item last synced time
	now := time.Now()
	c.storage.UpdateItemLastSynced(ctx, item.ID, now)

	return allLiabilities, nil
}

// SyncLiabilitiesByUserID syncs liabilities for all items belonging to a user.
func (c *Client) SyncLiabilitiesByUserID(ctx context.Context, userID uuid.UUID) ([]*Liability, error) {
	items, err := c.storage.GetItemsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var allLiabilities []*Liability
	for _, item := range items {
		if item.Status != ItemStatusActive {
			continue // Skip items that need re-authentication
		}

		liabilities, err := c.SyncLiabilities(ctx, item.ID)
		if err != nil {
			// Log error but continue with other items
			continue
		}
		allLiabilities = append(allLiabilities, liabilities...)
	}

	return allLiabilities, nil
}

// GetLiabilities retrieves all liabilities for a user from storage.
func (c *Client) GetLiabilities(ctx context.Context, userID uuid.UUID) ([]*Liability, error) {
	return c.storage.GetLiabilitiesByUserID(ctx, userID)
}

// GetLiabilityByAccountID retrieves the liability for a specific account.
func (c *Client) GetLiabilityByAccountID(ctx context.Context, accountID uuid.UUID) (*Liability, error) {
	return c.storage.GetLiabilityByAccountID(ctx, accountID)
}

// convertCreditCardLiability converts a Plaid credit card liability to our Liability model.
func (c *Client) convertCreditCardLiability(account *Account, cc plaidgo.CreditCardLiability) *Liability {
	liability := &Liability{
		AccountID: account.ID,
		UserID:    account.UserID,
		Type:      LiabilityTypeCredit,
	}

	// APR info - use the first APR entry
	aprs := cc.GetAprs()
	if len(aprs) > 0 {
		apr := aprs[0]
		percentage := apr.GetAprPercentage()
		liability.APRPercentage = &percentage
		liability.APRType = string(apr.GetAprType())
	}

	// Last payment
	if amt, ok := cc.GetLastPaymentAmountOk(); ok && amt != nil {
		liability.LastPaymentAmount = amt
	}
	if dateStr := cc.GetLastPaymentDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.LastPaymentDate = &date
		}
	}

	// Last statement
	if balance, ok := cc.GetLastStatementBalanceOk(); ok && balance != nil {
		liability.LastStatementBalance = balance
	}
	if dateStr := cc.GetLastStatementIssueDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.LastStatementDate = &date
		}
	}

	// Minimum payment
	if minPay, ok := cc.GetMinimumPaymentAmountOk(); ok && minPay != nil {
		liability.MinimumPaymentAmount = minPay
	}

	// Next payment due date
	if dateStr := cc.GetNextPaymentDueDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.NextPaymentDueDate = &date
		}
	}

	return liability
}

// convertMortgageLiability converts a Plaid mortgage liability to our Liability model.
func (c *Client) convertMortgageLiability(account *Account, mortgage plaidgo.MortgageLiability) *Liability {
	liability := &Liability{
		AccountID: account.ID,
		UserID:    account.UserID,
		Type:      LiabilityTypeMortgage,
	}

	// Interest rate - use interest rate from the mortgage interest rate struct
	interestRate := mortgage.GetInterestRate()
	if interestRate.Percentage.IsSet() {
		val := interestRate.Percentage.Get()
		if val != nil {
			liability.InterestRatePercentage = val
		}
	}

	// Last payment
	if amt, ok := mortgage.GetLastPaymentAmountOk(); ok && amt != nil {
		liability.LastPaymentAmount = amt
	}
	if dateStr := mortgage.GetLastPaymentDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.LastPaymentDate = &date
		}
	}

	// Origination info
	if dateStr := mortgage.GetOriginationDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.OriginationDate = &date
		}
	}
	if principal, ok := mortgage.GetOriginationPrincipalAmountOk(); ok && principal != nil {
		liability.OriginationPrincipal = principal
	}

	// Next payment due date
	if dateStr := mortgage.GetNextPaymentDueDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.NextPaymentDueDate = &date
		}
	}

	return liability
}

// convertStudentLoanLiability converts a Plaid student loan liability to our Liability model.
func (c *Client) convertStudentLoanLiability(account *Account, student plaidgo.StudentLoan) *Liability {
	liability := &Liability{
		AccountID: account.ID,
		UserID:    account.UserID,
		Type:      LiabilityTypeStudent,
	}

	// Interest rate
	if rate, ok := student.GetInterestRatePercentageOk(); ok && rate != nil {
		liability.InterestRatePercentage = rate
	}

	// Last payment
	if amt, ok := student.GetLastPaymentAmountOk(); ok && amt != nil {
		liability.LastPaymentAmount = amt
	}
	if dateStr := student.GetLastPaymentDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.LastPaymentDate = &date
		}
	}

	// Last statement - StudentLoan doesn't have GetLastStatementBalance
	// Use outstanding interest balance instead
	if balance, ok := student.GetOutstandingInterestAmountOk(); ok && balance != nil {
		liability.LastStatementBalance = balance
	}
	if dateStr := student.GetLastStatementIssueDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.LastStatementDate = &date
		}
	}

	// Origination info
	if dateStr := student.GetOriginationDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.OriginationDate = &date
		}
	}
	if principal, ok := student.GetOriginationPrincipalAmountOk(); ok && principal != nil {
		liability.OriginationPrincipal = principal
	}

	// Minimum payment
	if minPay, ok := student.GetMinimumPaymentAmountOk(); ok && minPay != nil {
		liability.MinimumPaymentAmount = minPay
	}

	// Next payment due date
	if dateStr := student.GetNextPaymentDueDate(); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			liability.NextPaymentDueDate = &date
		}
	}

	return liability
}
