package plaid

import (
	"errors"
	"os"

	plaidgo "github.com/plaid/plaid-go/v29/plaid"
)

// Environment represents a Plaid environment.
type Environment string

const (
	Sandbox     Environment = "sandbox"
	Development Environment = "development"
	Production  Environment = "production"
)

// Config holds the configuration for the Plaid client.
type Config struct {
	// ClientID is your Plaid client ID.
	ClientID string

	// Secret is your Plaid secret for the specified environment.
	Secret string

	// Environment specifies which Plaid environment to use.
	Environment Environment

	// EncryptionKey is used to encrypt access tokens at rest.
	// If empty, will be read from PLAID_ENCRYPTION_KEY environment variable.
	// Must be exactly 32 bytes for AES-256.
	EncryptionKey []byte

	// Products specifies the Plaid products to request during Link.
	// Defaults to transactions if not specified.
	Products []string

	// CountryCodes specifies the countries to support.
	// Defaults to ["US"] if not specified.
	CountryCodes []string

	// Language specifies the language for Link.
	// Defaults to "en" if not specified.
	Language string

	// WebhookURL is the URL Plaid will send webhooks to (optional).
	WebhookURL string

	// RedirectURI is required for OAuth institutions.
	RedirectURI string

	// RetryConfig configures retry behavior for API calls.
	// If not set, defaults to DefaultRetryConfig().
	RetryConfig *RetryConfig

	// EnableAuditLog enables database-driven audit logging of API calls.
	// Default is true.
	EnableAuditLog *bool
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.ClientID == "" {
		return errors.New("plaid: ClientID is required")
	}
	if c.Secret == "" {
		return errors.New("plaid: Secret is required")
	}
	if c.Environment == "" {
		return errors.New("plaid: Environment is required")
	}

	// Load encryption key from environment if not provided
	if len(c.EncryptionKey) == 0 {
		envKey := os.Getenv("PLAID_ENCRYPTION_KEY")
		if envKey == "" {
			return errors.New("plaid: EncryptionKey is required (set PLAID_ENCRYPTION_KEY or provide in config)")
		}
		c.EncryptionKey = []byte(envKey)
	}

	if len(c.EncryptionKey) != 32 {
		return errors.New("plaid: EncryptionKey must be exactly 32 bytes for AES-256")
	}

	// Set defaults
	if len(c.Products) == 0 {
		c.Products = []string{"transactions"}
	}
	if len(c.CountryCodes) == 0 {
		c.CountryCodes = []string{"US"}
	}
	if c.Language == "" {
		c.Language = "en"
	}

	return nil
}

// plaidEnvironment converts our Environment to Plaid's environment type.
func (c *Config) plaidEnvironment() plaidgo.Environment {
	switch c.Environment {
	case Production:
		return plaidgo.Production
	case Development:
		// Development environment uses same base URL as sandbox in Plaid SDK
		return plaidgo.Sandbox
	default:
		return plaidgo.Sandbox
	}
}
