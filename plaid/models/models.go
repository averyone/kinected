package models

import (
	"time"

	"github.com/google/uuid"
)

// Item represents a Plaid Item (a connection to a financial institution).
type Item struct {
	// ID is the unique identifier for this item in our system.
	ID uuid.UUID `json:"id"`

	// UserID is the ID of the user who owns this item.
	UserID uuid.UUID `json:"user_id"`

	// PlaidItemID is Plaid's identifier for this item.
	PlaidItemID string `json:"plaid_item_id"`

	// PlaidAccessToken is the encrypted access token for API calls.
	// This is stored encrypted and decrypted only when needed.
	PlaidAccessToken string `json:"-"`

	// InstitutionID is Plaid's identifier for the financial institution.
	InstitutionID string `json:"institution_id"`

	// InstitutionName is the display name of the financial institution.
	InstitutionName string `json:"institution_name"`

	// Status indicates the current status of the item.
	Status ItemStatus `json:"status"`

	// ErrorCode contains any error code if the item is in an error state.
	ErrorCode string `json:"error_code,omitempty"`

	// ErrorMessage contains any error message if the item is in an error state.
	ErrorMessage string `json:"error_message,omitempty"`

	// ConsentExpirationTime is when the user's consent expires (if applicable).
	ConsentExpirationTime *time.Time `json:"consent_expiration_time,omitempty"`

	// CreatedAt is when the item was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the item was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// LastSyncedAt is when data was last fetched from this item.
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
}

// ItemStatus represents the status of a Plaid item.
type ItemStatus string

const (
	ItemStatusActive          ItemStatus = "active"
	ItemStatusPendingAuth     ItemStatus = "pending_auth"
	ItemStatusLoginRequired   ItemStatus = "login_required"
	ItemStatusError           ItemStatus = "error"
	ItemStatusRevoked         ItemStatus = "revoked"
)

// Account represents a financial account from Plaid.
type Account struct {
	// ID is the unique identifier for this account in our system.
	ID uuid.UUID `json:"id"`

	// ItemID is the ID of the item this account belongs to.
	ItemID uuid.UUID `json:"item_id"`

	// UserID is the ID of the user who owns this account.
	UserID uuid.UUID `json:"user_id"`

	// PlaidAccountID is Plaid's identifier for this account.
	PlaidAccountID string `json:"plaid_account_id"`

	// Name is the account name (e.g., "Plaid Checking").
	Name string `json:"name"`

	// OfficialName is the official account name from the institution.
	OfficialName string `json:"official_name,omitempty"`

	// Type is the account type (e.g., depository, credit, loan).
	Type AccountType `json:"type"`

	// Subtype is the account subtype (e.g., checking, savings, credit card).
	Subtype string `json:"subtype,omitempty"`

	// Mask is the last 4 digits of the account number.
	Mask string `json:"mask,omitempty"`

	// CurrentBalance is the current balance of the account.
	CurrentBalance *float64 `json:"current_balance,omitempty"`

	// AvailableBalance is the available balance of the account.
	AvailableBalance *float64 `json:"available_balance,omitempty"`

	// Limit is the credit limit (for credit accounts).
	Limit *float64 `json:"limit,omitempty"`

	// CurrencyCode is the ISO currency code (e.g., "USD").
	CurrencyCode string `json:"currency_code,omitempty"`

	// CreatedAt is when the account was first seen.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the account was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// AccountType represents the type of financial account.
type AccountType string

const (
	AccountTypeDepository  AccountType = "depository"
	AccountTypeCredit      AccountType = "credit"
	AccountTypeLoan        AccountType = "loan"
	AccountTypeInvestment  AccountType = "investment"
	AccountTypeBrokerage   AccountType = "brokerage"
	AccountTypeOther       AccountType = "other"
)

// Transaction represents a financial transaction from Plaid.
type Transaction struct {
	// ID is the unique identifier for this transaction in our system.
	ID uuid.UUID `json:"id"`

	// AccountID is the ID of the account this transaction belongs to.
	AccountID uuid.UUID `json:"account_id"`

	// UserID is the ID of the user who owns this transaction.
	UserID uuid.UUID `json:"user_id"`

	// PlaidTransactionID is Plaid's identifier for this transaction.
	PlaidTransactionID string `json:"plaid_transaction_id"`

	// Amount is the transaction amount (positive = money out, negative = money in).
	Amount float64 `json:"amount"`

	// CurrencyCode is the ISO currency code (e.g., "USD").
	CurrencyCode string `json:"currency_code,omitempty"`

	// Date is the date the transaction occurred.
	Date time.Time `json:"date"`

	// AuthorizedDate is when the transaction was authorized (if available).
	AuthorizedDate *time.Time `json:"authorized_date,omitempty"`

	// Name is the merchant name or transaction description.
	Name string `json:"name"`

	// MerchantName is the cleaned merchant name (if available).
	MerchantName string `json:"merchant_name,omitempty"`

	// Category is the primary category of the transaction.
	Category string `json:"category,omitempty"`

	// CategoryID is Plaid's category identifier.
	CategoryID string `json:"category_id,omitempty"`

	// Pending indicates if the transaction is still pending.
	Pending bool `json:"pending"`

	// PendingTransactionID links to the pending version of a posted transaction.
	PendingTransactionID string `json:"pending_transaction_id,omitempty"`

	// PaymentChannel indicates how the transaction was made (online, in store, etc.).
	PaymentChannel string `json:"payment_channel,omitempty"`

	// CreatedAt is when we first saw this transaction.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the transaction was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// Liability represents credit card or loan liability information.
type Liability struct {
	// ID is the unique identifier for this liability in our system.
	ID uuid.UUID `json:"id"`

	// AccountID is the ID of the account this liability belongs to.
	AccountID uuid.UUID `json:"account_id"`

	// UserID is the ID of the user who owns this liability.
	UserID uuid.UUID `json:"user_id"`

	// Type is the type of liability (credit, mortgage, student, etc.).
	Type LiabilityType `json:"type"`

	// LastPaymentAmount is the amount of the last payment.
	LastPaymentAmount *float64 `json:"last_payment_amount,omitempty"`

	// LastPaymentDate is when the last payment was made.
	LastPaymentDate *time.Time `json:"last_payment_date,omitempty"`

	// LastStatementBalance is the balance on the last statement.
	LastStatementBalance *float64 `json:"last_statement_balance,omitempty"`

	// LastStatementDate is the date of the last statement.
	LastStatementDate *time.Time `json:"last_statement_date,omitempty"`

	// MinimumPaymentAmount is the minimum payment due.
	MinimumPaymentAmount *float64 `json:"minimum_payment_amount,omitempty"`

	// NextPaymentDueDate is when the next payment is due.
	NextPaymentDueDate *time.Time `json:"next_payment_due_date,omitempty"`

	// APRPercentage is the APR for credit cards.
	APRPercentage *float64 `json:"apr_percentage,omitempty"`

	// APRType is the type of APR (purchase, balance_transfer, etc.).
	APRType string `json:"apr_type,omitempty"`

	// OriginationDate is when the loan originated.
	OriginationDate *time.Time `json:"origination_date,omitempty"`

	// OriginationPrincipal is the original loan amount.
	OriginationPrincipal *float64 `json:"origination_principal,omitempty"`

	// InterestRatePercentage is the interest rate for loans.
	InterestRatePercentage *float64 `json:"interest_rate_percentage,omitempty"`

	// CreatedAt is when we first saw this liability.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the liability was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// LiabilityType represents the type of liability.
type LiabilityType string

const (
	LiabilityTypeCredit   LiabilityType = "credit"
	LiabilityTypeMortgage LiabilityType = "mortgage"
	LiabilityTypeStudent  LiabilityType = "student"
	LiabilityTypeOther    LiabilityType = "other"
)

// LinkTokenRequest contains parameters for creating a Link token.
type LinkTokenRequest struct {
	// UserID is the unique identifier for the user.
	UserID uuid.UUID

	// ClientUserID is an optional client-defined user ID for Plaid.
	// If not provided, UserID.String() will be used.
	ClientUserID string

	// Products overrides the default products for this link session.
	Products []string

	// AccessToken is used for update mode (re-authenticating an existing item).
	AccessToken string
}

// LinkTokenResponse contains the response from creating a Link token.
type LinkTokenResponse struct {
	// LinkToken is the token to use with Plaid Link.
	LinkToken string `json:"link_token"`

	// Expiration is when the link token expires.
	Expiration time.Time `json:"expiration"`

	// RequestID is Plaid's request identifier.
	RequestID string `json:"request_id"`
}

// ExchangeTokenRequest contains parameters for exchanging a public token.
type ExchangeTokenRequest struct {
	// UserID is the unique identifier for the user.
	UserID uuid.UUID

	// PublicToken is the public token from Plaid Link.
	PublicToken string

	// InstitutionID is the ID of the institution (from Link metadata).
	InstitutionID string

	// InstitutionName is the name of the institution (from Link metadata).
	InstitutionName string
}

// DateRange represents a date range for querying transactions.
type DateRange struct {
	// StartDate is the beginning of the range (inclusive).
	StartDate time.Time

	// EndDate is the end of the range (inclusive).
	EndDate time.Time
}
