package http

import (
	"fmt"
	"net/http"
	"time"

	kinplaid "github.com/kinected/kinected/plaid"
)

// RegisterRoutes registers all Plaid routes with the given ServeMux.
// The routes will be registered under the handler's base URL prefix.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	prefix := h.baseURL

	// Link flow
	mux.HandleFunc(prefix+"/link", h.LinkPage)
	mux.HandleFunc(prefix+"/link/token", h.CreateLinkToken)
	mux.HandleFunc(prefix+"/link/token/update", h.CreateUpdateLinkToken)
	mux.HandleFunc(prefix+"/link/exchange", h.ExchangeToken)
	mux.HandleFunc(prefix+"/callback", h.CallbackPage)

	// Items
	mux.HandleFunc(prefix+"/items", h.handleItems)

	// Accounts
	mux.HandleFunc(prefix+"/accounts", h.GetAccounts)
	mux.HandleFunc(prefix+"/accounts/sync", h.SyncBalances)

	// Transactions
	mux.HandleFunc(prefix+"/transactions", h.GetTransactions)
	mux.HandleFunc(prefix+"/transactions/sync", h.SyncTransactions)

	// Liabilities
	mux.HandleFunc(prefix+"/liabilities", h.GetLiabilities)
	mux.HandleFunc(prefix+"/liabilities/sync", h.SyncLiabilities)
}

// handleItems routes /items requests based on HTTP method and path.
func (h *Handler) handleItems(w http.ResponseWriter, r *http.Request) {
	// Check if there's an ID in the path
	path := r.URL.Path[len(h.baseURL+"/items"):]

	if path == "" || path == "/" {
		// /items - list items
		h.GetItems(w, r)
		return
	}

	// /items/{id} - delete item
	if r.Method == http.MethodDelete {
		// Rewrite path for RemoveItem handler
		r.URL.Path = "/items" + path
		h.RemoveItem(w, r)
		return
	}

	h.error(w, http.StatusMethodNotAllowed, "method not allowed")
}

// parseDateRange extracts start_date and end_date from query parameters.
// Defaults to the last 30 days if not provided.
func parseDateRange(r *http.Request) (kinplaid.DateRange, error) {
	now := time.Now()
	dateRange := kinplaid.DateRange{
		StartDate: now.AddDate(0, 0, -30),
		EndDate:   now,
	}

	if start := r.URL.Query().Get("start_date"); start != "" {
		parsed, err := time.Parse("2006-01-02", start)
		if err != nil {
			return dateRange, fmt.Errorf("invalid start_date format (expected YYYY-MM-DD)")
		}
		dateRange.StartDate = parsed
	}

	if end := r.URL.Query().Get("end_date"); end != "" {
		parsed, err := time.Parse("2006-01-02", end)
		if err != nil {
			return dateRange, fmt.Errorf("invalid end_date format (expected YYYY-MM-DD)")
		}
		dateRange.EndDate = parsed
	}

	if dateRange.StartDate.After(dateRange.EndDate) {
		return dateRange, fmt.Errorf("start_date must be before end_date")
	}

	return dateRange, nil
}
