package plaid

import (
	"context"
	"fmt"

	plaidgo "github.com/plaid/plaid-go/v29/plaid"

	"github.com/kinected/kinected/plaid/storage"
)

// Client provides methods for interacting with Plaid and managing financial data.
type Client struct {
	config  *Config
	plaid   *plaidgo.APIClient
	storage storage.Storage
}

// NewClient creates a new Plaid client with the given configuration and storage.
func NewClient(cfg Config, store storage.Storage) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Configure Plaid client
	plaidConfig := plaidgo.NewConfiguration()
	plaidConfig.AddDefaultHeader("PLAID-CLIENT-ID", cfg.ClientID)
	plaidConfig.AddDefaultHeader("PLAID-SECRET", cfg.Secret)
	plaidConfig.UseEnvironment(cfg.plaidEnvironment())

	apiClient := plaidgo.NewAPIClient(plaidConfig)

	return &Client{
		config:  &cfg,
		plaid:   apiClient,
		storage: store,
	}, nil
}

// Storage returns the underlying storage implementation.
// This can be used for direct database access if needed.
func (c *Client) Storage() storage.Storage {
	return c.storage
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	return c.storage.Close()
}

// encryptAccessToken encrypts an access token for storage.
func (c *Client) encryptAccessToken(token string) (string, error) {
	encrypted, err := encrypt(token, c.config.EncryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}
	return encrypted, nil
}

// decryptAccessToken decrypts an access token from storage.
func (c *Client) decryptAccessToken(encrypted string) (string, error) {
	decrypted, err := decrypt(encrypted, c.config.EncryptionKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	return decrypted, nil
}

// convertPlaidProducts converts string product names to Plaid's Products type.
func convertPlaidProducts(products []string) []plaidgo.Products {
	result := make([]plaidgo.Products, len(products))
	for i, p := range products {
		result[i] = plaidgo.Products(p)
	}
	return result
}

// convertPlaidCountryCodes converts string country codes to Plaid's CountryCode type.
func convertPlaidCountryCodes(codes []string) []plaidgo.CountryCode {
	result := make([]plaidgo.CountryCode, len(codes))
	for i, c := range codes {
		result[i] = plaidgo.CountryCode(c)
	}
	return result
}

// handlePlaidError converts a Plaid API error to our error type.
func handlePlaidError(err error, ctx context.Context) error {
	if plaidErr, ok := err.(*plaidgo.GenericOpenAPIError); ok {
		if errorResponse, ok := plaidErr.Model().(plaidgo.PlaidError); ok {
			return &PlaidError{
				ErrorType:      string(errorResponse.GetErrorType()),
				ErrorCode:      errorResponse.GetErrorCode(),
				ErrorMessage:   errorResponse.GetErrorMessage(),
				DisplayMessage: errorResponse.GetDisplayMessage(),
				RequestID:      errorResponse.GetRequestId(),
				Cause:          err,
			}
		}
	}
	return err
}
