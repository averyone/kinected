package plaid

import (
	"context"
	"time"

	"github.com/google/uuid"
	plaidgo "github.com/plaid/plaid-go/v29/plaid"
)

// CreateLinkToken creates a Plaid Link token for initiating the Link flow.
func (c *Client) CreateLinkToken(ctx context.Context, req LinkTokenRequest) (*LinkTokenResponse, error) {
	clientUserID := req.ClientUserID
	if clientUserID == "" {
		clientUserID = req.UserID.String()
	}

	products := c.config.Products
	if len(req.Products) > 0 {
		products = req.Products
	}

	linkReq := plaidgo.NewLinkTokenCreateRequest(
		"Kinected",
		c.config.Language,
		convertPlaidCountryCodes(c.config.CountryCodes),
		plaidgo.LinkTokenCreateRequestUser{
			ClientUserId: clientUserID,
		},
	)

	// Set products (not needed for update mode)
	if req.AccessToken == "" {
		linkReq.SetProducts(convertPlaidProducts(products))
	} else {
		// Update mode - use existing access token
		linkReq.SetAccessToken(req.AccessToken)
	}

	// Set optional webhook URL
	if c.config.WebhookURL != "" {
		linkReq.SetWebhook(c.config.WebhookURL)
	}

	// Set redirect URI for OAuth
	if c.config.RedirectURI != "" {
		linkReq.SetRedirectUri(c.config.RedirectURI)
	}

	resp, _, err := c.plaid.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*linkReq).Execute()
	if err != nil {
		return nil, handlePlaidError(err, ctx)
	}

	return &LinkTokenResponse{
		LinkToken:  resp.GetLinkToken(),
		Expiration: resp.GetExpiration(),
		RequestID:  resp.GetRequestId(),
	}, nil
}

// ExchangePublicToken exchanges a public token from Link for an access token,
// creates an Item record, and fetches initial account information.
func (c *Client) ExchangePublicToken(ctx context.Context, req ExchangeTokenRequest) (*Item, error) {
	// Exchange public token for access token
	exchangeReq := plaidgo.NewItemPublicTokenExchangeRequest(req.PublicToken)
	resp, _, err := c.plaid.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*exchangeReq).Execute()
	if err != nil {
		return nil, handlePlaidError(err, ctx)
	}

	accessToken := resp.GetAccessToken()
	plaidItemID := resp.GetItemId()

	// Encrypt the access token
	encryptedToken, err := c.encryptAccessToken(accessToken)
	if err != nil {
		return nil, err
	}

	// Create the item record
	now := time.Now()
	item := &Item{
		ID:               uuid.New(),
		UserID:           req.UserID,
		PlaidItemID:      plaidItemID,
		PlaidAccessToken: encryptedToken,
		InstitutionID:    req.InstitutionID,
		InstitutionName:  req.InstitutionName,
		Status:           ItemStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := c.storage.CreateItem(ctx, item); err != nil {
		return nil, err
	}

	// Fetch initial account information
	if _, err := c.syncAccounts(ctx, item, accessToken); err != nil {
		// Log error but don't fail - item was created successfully
		// The accounts can be synced later
	}

	return item, nil
}

// CreateUpdateLinkToken creates a Link token for re-authenticating an existing item.
func (c *Client) CreateUpdateLinkToken(ctx context.Context, itemID uuid.UUID) (*LinkTokenResponse, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	return c.CreateLinkToken(ctx, LinkTokenRequest{
		UserID:      item.UserID,
		AccessToken: accessToken,
	})
}

// RemoveItem removes a Plaid item (disconnects the bank account).
func (c *Client) RemoveItem(ctx context.Context, itemID uuid.UUID) error {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return err
	}

	// Remove from Plaid
	removeReq := plaidgo.NewItemRemoveRequest(accessToken)
	_, _, err = c.plaid.PlaidApi.ItemRemove(ctx).ItemRemoveRequest(*removeReq).Execute()
	if err != nil {
		// Continue with local deletion even if Plaid fails
		// (item might already be removed on Plaid's side)
	}

	// Delete from local storage (cascades to accounts, transactions, etc.)
	return c.storage.DeleteItem(ctx, itemID)
}

// GetItem retrieves an item by ID.
func (c *Client) GetItem(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	return c.storage.GetItem(ctx, itemID)
}

// GetItemsByUserID retrieves all items for a user.
func (c *Client) GetItemsByUserID(ctx context.Context, userID uuid.UUID) ([]*Item, error) {
	return c.storage.GetItemsByUserID(ctx, userID)
}

// RefreshItemStatus fetches the latest status of an item from Plaid.
func (c *Client) RefreshItemStatus(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	item, err := c.storage.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}

	accessToken, err := c.decryptAccessToken(item.PlaidAccessToken)
	if err != nil {
		return nil, err
	}

	// Get item status from Plaid
	getReq := plaidgo.NewItemGetRequest(accessToken)
	resp, _, err := c.plaid.PlaidApi.ItemGet(ctx).ItemGetRequest(*getReq).Execute()
	if err != nil {
		plaidErr := handlePlaidError(err, ctx)
		if pe, ok := plaidErr.(*PlaidError); ok && pe.NeedsReauthentication() {
			// Update item status to reflect auth issue
			c.storage.UpdateItemStatus(ctx, itemID, ItemStatusLoginRequired, pe.ErrorCode, pe.ErrorMessage)
			item.Status = ItemStatusLoginRequired
			item.ErrorCode = pe.ErrorCode
			item.ErrorMessage = pe.ErrorMessage
			return item, plaidErr
		}
		return nil, plaidErr
	}

	// Update item status based on response
	plaidItem := resp.GetItem()
	itemError := plaidItem.GetError()

	// Check if there's an error on the item
	if itemError.ErrorCode != "" {
		errorCode := itemError.ErrorCode
		errorMessage := itemError.ErrorMessage

		status := ItemStatusError
		if errorCode == "ITEM_LOGIN_REQUIRED" {
			status = ItemStatusLoginRequired
		}

		c.storage.UpdateItemStatus(ctx, itemID, status, errorCode, errorMessage)
		item.Status = status
		item.ErrorCode = errorCode
		item.ErrorMessage = errorMessage
	} else {
		// Item is healthy
		if item.Status != ItemStatusActive {
			c.storage.UpdateItemStatus(ctx, itemID, ItemStatusActive, "", "")
			item.Status = ItemStatusActive
			item.ErrorCode = ""
			item.ErrorMessage = ""
		}
	}

	return item, nil
}
