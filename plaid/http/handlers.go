package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	kinplaid "github.com/kinected/kinected/plaid"
)

// Handler provides HTTP handlers for Plaid operations.
type Handler struct {
	client     *kinplaid.Client
	getUserID  func(r *http.Request) (uuid.UUID, error)
	baseURL    string
	plaidEnv   string
}

// HandlerConfig configures the HTTP handler.
type HandlerConfig struct {
	// Client is the Plaid client instance.
	Client *kinplaid.Client

	// GetUserID is a function that extracts the user ID from a request.
	// This should be provided by your authentication middleware.
	GetUserID func(r *http.Request) (uuid.UUID, error)

	// BaseURL is the base URL for this handler (e.g., "/plaid").
	// Used for redirects and link generation.
	BaseURL string

	// PlaidEnvironment is "sandbox", "development", or "production".
	// Used by the Link UI to load the correct Plaid script.
	PlaidEnvironment string
}

// NewHandler creates a new HTTP handler.
func NewHandler(cfg HandlerConfig) *Handler {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "/plaid"
	}
	if cfg.PlaidEnvironment == "" {
		cfg.PlaidEnvironment = "sandbox"
	}

	return &Handler{
		client:    cfg.Client,
		getUserID: cfg.GetUserID,
		baseURL:   cfg.BaseURL,
		plaidEnv:  cfg.PlaidEnvironment,
	}
}

// CreateLinkToken handles POST /link/token - creates a new link token.
func (h *Handler) CreateLinkToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resp, err := h.client.CreateLinkToken(r.Context(), kinplaid.LinkTokenRequest{
		UserID: userID,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, resp)
}

// CreateUpdateLinkToken handles POST /link/token/update - creates a link token for re-auth.
func (h *Handler) CreateUpdateLinkToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ItemID string `json:"item_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	itemID, err := uuid.Parse(req.ItemID)
	if err != nil {
		h.error(w, http.StatusBadRequest, "invalid item_id")
		return
	}

	// Verify the item belongs to this user
	item, err := h.client.GetItem(r.Context(), itemID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if item.UserID != userID {
		h.error(w, http.StatusForbidden, "forbidden")
		return
	}

	resp, err := h.client.CreateUpdateLinkToken(r.Context(), itemID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, resp)
}

// ExchangeToken handles POST /link/exchange - exchanges public token for access token.
func (h *Handler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		PublicToken     string `json:"public_token"`
		InstitutionID   string `json:"institution_id"`
		InstitutionName string `json:"institution_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PublicToken == "" {
		h.error(w, http.StatusBadRequest, "public_token is required")
		return
	}

	item, err := h.client.ExchangePublicToken(r.Context(), kinplaid.ExchangeTokenRequest{
		UserID:          userID,
		PublicToken:     req.PublicToken,
		InstitutionID:   req.InstitutionID,
		InstitutionName: req.InstitutionName,
	})
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, item)
}

// GetItems handles GET /items - returns all items for the current user.
func (h *Handler) GetItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	items, err := h.client.GetItemsByUserID(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"items": items})
}

// RemoveItem handles DELETE /items/{id} - removes an item.
func (h *Handler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Extract item ID from path - expects /items/{id}
	itemIDStr := r.URL.Path[len("/items/"):]
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		h.error(w, http.StatusBadRequest, "invalid item_id")
		return
	}

	// Verify the item belongs to this user
	item, err := h.client.GetItem(r.Context(), itemID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	if item.UserID != userID {
		h.error(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.client.RemoveItem(r.Context(), itemID); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAccounts handles GET /accounts - returns all accounts for the current user.
func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accounts, err := h.client.GetAccountsByUserID(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"accounts": accounts})
}

// SyncBalances handles POST /accounts/sync - syncs balances for all accounts.
func (h *Handler) SyncBalances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	accounts, err := h.client.SyncBalancesByUserID(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"accounts": accounts})
}

// GetTransactions handles GET /transactions - returns transactions for the current user.
func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	dateRange, err := parseDateRange(r)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	transactions, err := h.client.GetTransactionsByUserID(r.Context(), userID, dateRange)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"transactions": transactions})
}

// SyncTransactions handles POST /transactions/sync - syncs transactions for all items.
func (h *Handler) SyncTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	dateRange, err := parseDateRange(r)
	if err != nil {
		h.error(w, http.StatusBadRequest, err.Error())
		return
	}

	transactions, err := h.client.SyncTransactionsByUserID(r.Context(), userID, dateRange)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	})
}

// GetLiabilities handles GET /liabilities - returns liabilities for the current user.
func (h *Handler) GetLiabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	liabilities, err := h.client.GetLiabilities(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"liabilities": liabilities})
}

// SyncLiabilities handles POST /liabilities/sync - syncs liabilities for all items.
func (h *Handler) SyncLiabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserID(r)
	if err != nil {
		h.error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	liabilities, err := h.client.SyncLiabilitiesByUserID(r.Context(), userID)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{"liabilities": liabilities})
}

// LinkPage handles GET /link - serves the Plaid Link UI page.
func (h *Handler) LinkPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	data := map[string]string{
		"BaseURL":  h.baseURL,
		"PlaidEnv": h.plaidEnv,
	}

	if err := linkTemplate.Execute(w, data); err != nil {
		h.error(w, http.StatusInternalServerError, "template error")
		return
	}
}

// CallbackPage handles GET /callback - serves the OAuth callback page.
func (h *Handler) CallbackPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := callbackTemplate.Execute(w, nil); err != nil {
		h.error(w, http.StatusInternalServerError, "template error")
		return
	}
}

// json writes a JSON response.
func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// error writes an error response.
func (h *Handler) error(w http.ResponseWriter, status int, message string) {
	h.json(w, status, map[string]string{"error": message})
}

// handleError converts Plaid errors to HTTP responses.
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	if err == kinplaid.ErrItemNotFound || err == kinplaid.ErrAccountNotFound {
		h.error(w, http.StatusNotFound, err.Error())
		return
	}

	if plaidErr, ok := err.(*kinplaid.PlaidError); ok {
		status := http.StatusBadRequest
		if plaidErr.IsRateLimitError() {
			status = http.StatusTooManyRequests
		} else if plaidErr.NeedsReauthentication() {
			status = http.StatusUnauthorized
		}
		h.json(w, status, map[string]interface{}{
			"error":           plaidErr.ErrorMessage,
			"error_code":      plaidErr.ErrorCode,
			"error_type":      plaidErr.ErrorType,
			"display_message": plaidErr.DisplayMessage,
		})
		return
	}

	h.error(w, http.StatusInternalServerError, "internal server error")
}
