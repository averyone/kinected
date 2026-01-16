package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	kinplaid "github.com/kinected/kinected/plaid"
	plaidhttp "github.com/kinected/kinected/plaid/http"
	"github.com/kinected/kinected/plaid/storage"
)

func main() {
	// Load configuration from environment variables
	clientID := os.Getenv("PLAID_CLIENT_ID")
	secret := os.Getenv("PLAID_SECRET")
	env := os.Getenv("PLAID_ENV")
	if env == "" {
		env = "sandbox"
	}

	if clientID == "" || secret == "" {
		log.Fatal("PLAID_CLIENT_ID and PLAID_SECRET must be set")
	}

	// Ensure encryption key is set (32 bytes for AES-256)
	encryptionKey := os.Getenv("PLAID_ENCRYPTION_KEY")
	if encryptionKey == "" {
		// For demo purposes, generate a warning but use a default
		log.Println("WARNING: PLAID_ENCRYPTION_KEY not set. Using default key for demo (DO NOT USE IN PRODUCTION)")
		os.Setenv("PLAID_ENCRYPTION_KEY", "demo-key-32-bytes-for-aes-256!!")
	}

	// Initialize SQLite storage
	store, err := storage.NewSQLiteStorage(storage.SQLiteConfig{
		Path: "demo.db",
	})
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize Plaid client
	client, err := kinplaid.NewClient(kinplaid.Config{
		ClientID:    clientID,
		Secret:      secret,
		Environment: kinplaid.Environment(env),
		Products:    []string{"transactions", "liabilities"},
	}, store)
	if err != nil {
		log.Fatalf("Failed to initialize Plaid client: %v", err)
	}
	defer client.Close()

	// Create HTTP handler
	handler := plaidhttp.NewHandler(plaidhttp.HandlerConfig{
		Client:           client,
		GetUserID:        getDemoUserID,
		BaseURL:          "/plaid",
		PlaidEnvironment: env,
	})

	// Set up routes
	mux := http.NewServeMux()

	// Root page with instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Kinected Demo</title>
    <style>
        body { font-family: sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; }
        h1 { color: #333; }
        .endpoint { background: #f5f5f5; padding: 12px; margin: 8px 0; border-radius: 4px; }
        .method { font-weight: bold; color: #667eea; }
        code { background: #e0e0e0; padding: 2px 6px; border-radius: 3px; }
        a { color: #667eea; }
    </style>
</head>
<body>
    <h1>Kinected Plaid Demo</h1>
    <p>This demo server shows how to use the Kinected Plaid library.</p>

    <h2>Getting Started</h2>
    <p><a href="/plaid/link">Open Plaid Link UI</a> - Connect a bank account using the embedded UI</p>

    <h2>API Endpoints</h2>

    <h3>Link Flow</h3>
    <div class="endpoint"><span class="method">POST</span> <code>/plaid/link/token</code> - Create a link token</div>
    <div class="endpoint"><span class="method">POST</span> <code>/plaid/link/exchange</code> - Exchange public token</div>

    <h3>Items (Connected Banks)</h3>
    <div class="endpoint"><span class="method">GET</span> <code>/plaid/items</code> - List connected items</div>
    <div class="endpoint"><span class="method">DELETE</span> <code>/plaid/items/{id}</code> - Remove an item</div>

    <h3>Accounts</h3>
    <div class="endpoint"><span class="method">GET</span> <code>/plaid/accounts</code> - List accounts</div>
    <div class="endpoint"><span class="method">POST</span> <code>/plaid/accounts/sync</code> - Sync balances</div>

    <h3>Transactions</h3>
    <div class="endpoint"><span class="method">GET</span> <code>/plaid/transactions?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD</code> - Get transactions</div>
    <div class="endpoint"><span class="method">POST</span> <code>/plaid/transactions/sync?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD</code> - Sync transactions</div>

    <h3>Liabilities</h3>
    <div class="endpoint"><span class="method">GET</span> <code>/plaid/liabilities</code> - Get liabilities</div>
    <div class="endpoint"><span class="method">POST</span> <code>/plaid/liabilities/sync</code> - Sync liabilities</div>

    <h2>Environment</h2>
    <p>Running in <strong>%s</strong> mode</p>
    <p>Demo User ID: <code>%s</code></p>
</body>
</html>`, env, demoUserID.String())
	})

	// Register Plaid routes
	handler.RegisterRoutes(mux)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting demo server on http://localhost:%s", port)
	log.Printf("Open http://localhost:%s/plaid/link to connect a bank account", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// demoUserID is a fixed user ID for the demo.
// In a real application, this would come from your authentication system.
var demoUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// getDemoUserID returns the demo user ID for all requests.
// In a real application, you would extract this from a session or JWT.
func getDemoUserID(r *http.Request) (uuid.UUID, error) {
	return demoUserID, nil
}
