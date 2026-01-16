package plaid

import "github.com/kinected/kinected/plaid/models"

// Re-export types from models package for convenience.
// This allows consumers to use plaid.Item instead of models.Item.
type (
	Item                 = models.Item
	ItemStatus           = models.ItemStatus
	Account              = models.Account
	AccountType          = models.AccountType
	Transaction          = models.Transaction
	Liability            = models.Liability
	LiabilityType        = models.LiabilityType
	DateRange            = models.DateRange
	LinkTokenRequest     = models.LinkTokenRequest
	LinkTokenResponse    = models.LinkTokenResponse
	ExchangeTokenRequest = models.ExchangeTokenRequest
)

// Re-export constants from models package.
const (
	ItemStatusActive        = models.ItemStatusActive
	ItemStatusPendingAuth   = models.ItemStatusPendingAuth
	ItemStatusLoginRequired = models.ItemStatusLoginRequired
	ItemStatusError         = models.ItemStatusError
	ItemStatusRevoked       = models.ItemStatusRevoked

	AccountTypeDepository = models.AccountTypeDepository
	AccountTypeCredit     = models.AccountTypeCredit
	AccountTypeLoan       = models.AccountTypeLoan
	AccountTypeInvestment = models.AccountTypeInvestment
	AccountTypeBrokerage  = models.AccountTypeBrokerage
	AccountTypeOther      = models.AccountTypeOther

	LiabilityTypeCredit   = models.LiabilityTypeCredit
	LiabilityTypeMortgage = models.LiabilityTypeMortgage
	LiabilityTypeStudent  = models.LiabilityTypeStudent
	LiabilityTypeOther    = models.LiabilityTypeOther
)
