package monzo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setup creates a mock server and a client configured to talk to it.
// It returns the client, the server's router (mux), and a teardown function.
func setup(t *testing.T) (client *Client, mux *http.ServeMux, teardown func()) {
	t.Helper()

	mux = http.NewServeMux()
	server := httptest.NewServer(mux)

	// Create a basic http client
	httpClient := server.Client()

	// Create our Monzo client
	client = NewClient(httpClient)
	client.SetBaseURL(server.URL)

	teardown = func() {
		server.Close()
	}

	return client, mux, teardown
}

func TestListAccounts_Success(t *testing.T) {
	// 1. Setup our mock server and client
	client, mux, teardown := setup(t)
	defer teardown()

	// 2. Define the mock API response
	mockResponse := `
	{
		"accounts": [
			{
				"id": "acc_001",
				"description": "Test Account",
				"created": "2020-01-01T00:00:00Z"
			}
		]
	}`

	// 3. Register the handler for the endpoint
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		// Test that the method is correct
		if r.Method != http.MethodGet {
			t.Fatalf("expected method GET, got %s", r.Method)
		}
		// Send the response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, mockResponse)
	})

	// 4. Run the client method
	ctx := context.Background()
	accounts, err := client.ListAccounts(ctx, "")

	// 5. Assert the results
	if err != nil {
		t.Fatalf("ListAccounts returned an unexpected error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].ID != "acc_001" {
		t.Errorf("expected account ID 'acc_001', got %s", accounts[0].ID)
	}
	if accounts[0].Description != "Test Account" {
		t.Errorf("expected description 'Test Account', got %s", accounts[0].Description)
	}
}

func TestGetBalance_Success(t *testing.T) {
	client, mux, teardown := setup(t)
	defer teardown()

	mockResponse := `
	{
		"balance": 5000,
		"total_balance": 6000,
		"currency": "GBP",
		"spend_today": 0
	}`

	mux.HandleFunc("/balance", func(w http.ResponseWriter, r *http.Request) {
		// Test that query params are correctly set
		query := r.URL.Query()
		if query.Get("account_id") != "acc_123" {
			t.Errorf("expected account_id query param 'acc_123', got %s", query.Get("account_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	})

	ctx := context.Background()
	balance, err := client.GetBalance(ctx, "acc_123")

	if err != nil {
		t.Fatalf("GetBalance returned an error: %v", err)
	}
	if balance.Balance != 5000 {
		t.Errorf("expected balance 5000, got %d", balance.Balance)
	}
	if balance.Currency != "GBP" {
		t.Errorf("expected currency 'GBP', got %s", balance.Currency)
	}
}

func TestDepositToPot_Success(t *testing.T) {
	client, mux, teardown := setup(t)
	defer teardown()

	mockResponse := `
	{
		"id": "pot_001",
		"name": "Savings",
		"balance": 11000,
		"currency": "GBP",
		"created": "2020-01-01T00:00:00Z",
		"updated": "2020-01-02T00:00:00Z",
		"deleted": false
	}`

	mux.HandleFunc("/pots/pot_001/deposit", func(w http.ResponseWriter, r *http.Request) {
		// Test this is a PUT request
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got %s", r.Method)
		}

		// Test it sent form-urlencoded data
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected Content-Type 'application/x-www-form-urlencoded', got %s", ct)
		}

		// Test the form data
		r.ParseForm()
		if r.PostForm.Get("source_account_id") != "acc_001" {
			t.Errorf("expected source_account_id 'acc_001', got %s", r.PostForm.Get("source_account_id"))
		}
		if r.PostForm.Get("amount") != "1000" {
			t.Errorf("expected amount '1000', got %s", r.PostForm.Get("amount"))
		}
		if r.PostForm.Get("dedupe_id") != "dedupe-123" {
			t.Errorf("expected dedupe_id 'dedupe-123', got %s", r.PostForm.Get("dedupe_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	})

	ctx := context.Background()
	pot, err := client.DepositToPot(ctx, "pot_001", "acc_001", "dedupe-123", 1000)

	if err != nil {
		t.Fatalf("DepositToPot returned an error: %v", err)
	}
	if pot.Balance != 11000 {
		t.Errorf("expected balance 11000, got %d", pot.Balance)
	}
}

func TestCreateReceipt_Success(t *testing.T) {
	client, mux, teardown := setup(t)
	defer teardown()

	// Mock response from the API
	mockResponse := `
	{
		"id": "receipt_001",
		"transaction_id": "tx_001",
		"external_id": "order-123",
		"total": 1299,
		"currency": "GBP",
		"items": []
	}`

	mux.HandleFunc("/transaction-receipts", func(w http.ResponseWriter, r *http.Request) {
		// Test this is a PUT request
		if r.Method != http.MethodPut {
			t.Errorf("expected method PUT, got %s", r.Method)
		}

		// Test it sent JSON data
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %s", ct)
		}

		// Test the JSON body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal("could not read request body")
		}
		if !strings.Contains(string(body), `"external_id":"order-123"`) {
			t.Errorf("request body missing correct external_id: got %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	})

	// This is the data we will send
	receiptToCreate := &Receipt{
		TransactionID: "tx_001",
		ExternalID:    "order-123",
		Total:         1299,
		Currency:      "GBP",
		Items:         []ReceiptItem{},
	}

	ctx := context.Background()
	receipt, err := client.CreateReceipt(ctx, receiptToCreate)

	if err != nil {
		t.Fatalf("CreateReceipt returned an error: %v", err)
	}
	if receipt.ID != "receipt_001" {
		t.Errorf("expected receipt ID 'receipt_001', got %s", receipt.ID)
	}
}

func TestAPIError(t *testing.T) {
	client, mux, teardown := setup(t)
	defer teardown()

	// 2. Define the mock API *error* response
	mockErrorResponse := `
	{
		"code": "unauthorized",
		"message": "Authentication required."
	}`

	// 3. Register the handler to return a 401
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized) // 401
		fmt.Fprint(w, mockErrorResponse)
	})

	// 4. Run the client method
	ctx := context.Background()
	_, err := client.ListAccounts(ctx, "")

	// 5. Assert the results
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	// Check if it's our specific APIError type
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected error type *APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "unauthorized") {
		t.Errorf("error body does not contain expected message: %s", apiErr.Body)
	}
}

func TestTransaction_MerchantHelpers(t *testing.T) {
	t.Run("Merchant as ID string", func(t *testing.T) {
		// Create a transaction where the merchant field is just an ID
		tx := &Transaction{
			ID:       "tx_001",
			Merchant: []byte(`"merch_001"`), // Note the escaped quotes
		}

		id, ok := tx.MerchantID()
		if !ok {
			t.Error("MerchantID() returned ok=false, expected true")
		}
		if id != "merch_001" {
			t.Errorf("expected merchant ID 'merch_001', got %s", id)
		}

		m, ok := tx.ExpandedMerchant()
		if ok {
			t.Error("ExpandedMerchant() returned ok=true, expected false")
		}
		if m != nil {
			t.Errorf("expected expanded merchant to be nil, got %v", m)
		}
	})

	t.Run("Merchant as expanded object", func(t *testing.T) {
		// Create a transaction where the merchant is a full JSON object
		merchantJSON := `
		{
			"id": "merch_002",
			"name": "Coffee Shop",
			"category": "eating_out"
		}`
		tx := &Transaction{
			ID:       "tx_002",
			Merchant: []byte(merchantJSON),
		}

		id, ok := tx.MerchantID()
		if ok {
			t.Errorf("MerchantID() returned ok=true, expected false (got id: %s)", id)
		}

		m, ok := tx.ExpandedMerchant()
		if !ok {
			t.Error("ExpandedMerchant() returned ok=false, expected true")
		}
		if m == nil {
			t.Fatal("expected expanded merchant, got nil")
		}
		if m.Name != "Coffee Shop" {
			t.Errorf("expected merchant name 'Coffee Shop', got %s", m.Name)
		}
	})
}

func TestListTransactions_Pagination(t *testing.T) {
	client, mux, teardown := setup(t)
	defer teardown()

	mockResponse := `{"transactions": []}` // We only care about the request

	mux.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		if query.Get("account_id") != "acc_001" {
			t.Errorf("expected account_id 'acc_001', got %s", query.Get("account_id"))
		}
		if query.Get("limit") != "50" {
			t.Errorf("expected limit '50', got %s", query.Get("limit"))
		}
		if query.Get("since") != "2025-01-01T00:00:00Z" {
			t.Errorf("expected since '2025-01-01T00:00:00Z', got %s", query.Get("since"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	})

	opts := &PaginationOptions{
		Limit: 50,
		Since: "2025-01-01T00:00:00Z",
	}

	ctx := context.Background()
	_, err := client.ListTransactions(ctx, "acc_001", opts)
	if err != nil {
		t.Fatalf("ListTransactions returned an error: %v", err)
	}
}

func TestParseWebhookTransactionCreated_Success(t *testing.T) {
	// 1. Define the mock webhook body from the Monzo docs
	mockWebhookBody := `
	{
		"type": "transaction.created",
		"data": {
			"account_id": "acc_00008gju41AHyfLUzBUk8A",
			"amount": -350,
			"created": "2015-09-04T14:28:40Z",
			"currency": "GBP",
			"description": "Ozone Coffee Roasters",
			"id": "tx_00008zjky19HyFLAzlUk7t",
			"category": "eating_out",
			"is_load": false,
			"settled": "2015-09-05T14:28:40Z",
			"merchant": {
				"id": "merch_00008zIcpbAKe8shBxXUtl",
				"group_id": "grp_00008zIcpbBOaAr7TTP3sv",
				"name": "The De Beauvoir Deli Co.",
				"category": "eating_out",
				"address": {},
				"created": "2015-08-22T12:20:18Z",
				"logo": "",
				"emoji": ""
			}
		}
	}`

	// 2. Create a mock HTTP request
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(mockWebhookBody))
	req.Header.Set("Content-Type", "application/json")

	// 3. Run the parser
	tx, err := ParseWebhookTransactionCreated(req)

	// 4. Assert the results
	if err != nil {
		t.Fatalf("ParseWebhookTransactionCreated returned an unexpected error: %v", err)
	}
	if tx == nil {
		t.Fatal("expected a transaction, got nil")
	}

	if tx.AccountID != "acc_00008gju41AHyfLUzBUk8A" { // <-- ADD THIS ASSERTION
		t.Errorf("expected account ID 'acc_00008gju41AHyfLUzBUk8A', got %s", tx.AccountID)
	}
	if tx.ID != "tx_00008zjky19HyFLAzlUk7t" {
		t.Errorf("expected transaction ID 'tx_00008zjky19HyFLAzlUk7t', got %s", tx.ID)
	}

	// 5. Test the merchant helper on the parsed data
	m, ok := tx.ExpandedMerchant()
	if !ok {
		t.Fatal("ExpandedMerchant() failed")
	}
	if m.Name != "The De Beauvoir Deli Co." {
		t.Errorf("expected merchant name 'The De Beauvoir Deli Co.', got %s", m.Name)
	}
}

func TestParseWebhookTransactionCreated_Failure(t *testing.T) {
	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("{not json}"))
		_, err := ParseWebhookTransactionCreated(req)
		if err == nil {
			t.Fatal("expected an error for invalid JSON, got nil")
		}
	})

	t.Run("wrong event type", func(t *testing.T) {
		mockWebhookBody := `{"type": "account.updated", "data": {}}`
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(mockWebhookBody))
		_, err := ParseWebhookTransactionCreated(req)
		if err == nil {
			t.Fatal("expected an error for wrong event type, got nil")
		}
		if !strings.Contains(err.Error(), "invalid webhook type") {
			t.Errorf("expected error message 'invalid webhook type', got %v", err)
		}
	})
}
