// Package monzo provides a client for interacting with the Monzo API.
//
// The client handles API requests, responses, and error handling.
// It is designed to be used with an authorized http.Client, typically
// one configured using the "golang.org/x/oauth2" package, which
// will handle token acquisition and refreshing.
//
// The package also includes helper functions for common tasks, such
// as parsing incoming webhooks.
package monzo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// BaseURL is the production base URL for the Monzo API.
	BaseURL = "https://api.monzo.com"
)

//####################################################################
//## 1. CLIENT AND CORE LOGIC
//####################################################################

// Client is the Monzo API client. It manages all interactions with
// the Monzo API.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// APIError represents an error returned from the Monzo API.
// It includes the HTTP status code and the raw response body.
type APIError struct {
	StatusCode int
	Body       string
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	return fmt.Sprintf("monzo: API error (status %d): %s", e.StatusCode, e.Body)
}

// NewClient creates a new Monzo API client.
// The httpClient provided should be an authorized client, typically
// from the golang.org/x/oauth2 package, as it must handle
// adding the "Authorization: Bearer <token>" header to requests.
func NewClient(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    BaseURL,
	}
}

// SetBaseURL allows overriding the default base URL. This is primarily
// used for testing purposes to point the client at a mock server.
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// doRequest is the central helper for making API requests.
// It handles context, method, path, query params, body encoding (JSON or form),
// and response decoding.
func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body, responseData interface{}) error {
	fullURL, err := url.Parse(c.baseURL)
	if err != nil {
		return err // Should not happen with constant BaseURL
	}
	fullURL.Path = path
	if query != nil {
		fullURL.RawQuery = query.Encode()
	}

	var reqBody io.Reader
	var contentType string

	switch b := body.(type) {
	case nil:
		// No body
	case url.Values:
		// Form data
		reqBody = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		// JSON data
		jsonBody, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if responseData != nil {
		if err := json.NewDecoder(resp.Body).Decode(responseData); err != nil {
			return fmt.Errorf("failed to decode response body: %w", err)
		}
	}

	return nil
}

//####################################################################
//## 2. API DATA MODELS
//####################################################################

// WhoAmIResponse defines the response for the /ping/whoami endpoint.
type WhoAmIResponse struct {
	// Authenticated is true if the access token is valid.
	Authenticated bool `json:"authenticated"`
	// ClientID is the ID of the OAuth client.
	ClientID string `json:"client_id"`
	// UserID is the ID of the authenticated user.
	UserID string `json:"user_id"`
}

// Account represents a Monzo account.
type Account struct {
	// ID is the unique identifier for the account.
	ID string `json:"id"`
	// Description is the user-defined description of the account.
	Description string `json:"description"`
	// Created is the timestamp when the account was created.
	Created time.Time `json:"created"`
	// Type is the type of account, e.g., "uk_retail", "uk_retail_joint".
	Type string `json:"type,omitempty"`
}

// ListAccountsResponse is the wrapper for the ListAccounts endpoint.
type ListAccountsResponse struct {
	// Accounts is a list of accounts.
	Accounts []Account `json:"accounts"`
}

// Balance represents the balance of a specific account.
type Balance struct {
	// Balance is the current available balance in minor units (e.g., pennies).
	Balance int64 `json:"balance"`
	// TotalBalance is the balance including all pots in minor units.
	TotalBalance int64 `json:"total_balance"`
	// Currency is the ISO 4217 currency code (e.g., "GBP").
	Currency string `json:"currency"`
	// SpendToday is the amount spent today in minor units.
	SpendToday int64 `json:"spend_today"`
}

// Pot represents a Monzo pot.
type Pot struct {
	// ID is the unique identifier for the pot.
	ID string `json:"id"`
	// Name is the user-defined name of the pot.
	Name string `json:"name"`
	// Style is the visual style of the pot (e.g., "beach_ball").
	Style string `json:"style"`
	// Balance is the current balance of the pot in minor units.
	Balance int64 `json:"balance"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// Created is the timestamp when the pot was created.
	Created time.Time `json:"created"`
	// Updated is the timestamp when the pot was last updated.
	Updated time.Time `json:"updated"`
	// Deleted is true if the pot has been deleted.
	Deleted bool `json:"deleted"`
}

// ListPotsResponse is the wrapper for the ListPots endpoint.
type ListPotsResponse struct {
	// Pots is a list of pots.
	Pots []Pot `json:"pots"`
}

// Transaction represents a single transaction.
type Transaction struct {
	// AccountID is the ID of the account this transaction belongs to.
	AccountID string `json:"account_id"`
	// Amount is the transaction amount in minor units. Negative for debits.
	Amount int64 `json:"amount"`
	// Created is the timestamp when the transaction was created.
	Created time.Time `json:"created"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// Description is the transaction description.
	Description string `json:"description"`
	// ID is the unique identifier for the transaction.
	ID string `json:"id"`
	// Merchant contains merchant data. It can be either a string (merchant ID)
	// or a full Merchant object if expanded.
	Merchant json.RawMessage `json:"merchant"`
	// Metadata contains key-value annotations for the transaction.
	Metadata map[string]string `json:"metadata"`
	// Notes contains user-added notes for the transaction.
	Notes string `json:"notes"`
	// IsLoad is true if this is a top-up transaction.
	IsLoad bool `json:"is_load"`
	// Settled is the timestamp when the transaction settled.
	Settled time.Time `json:"settled"`
	// Category is the transaction category (e.g., "eating_out").
	Category string `json:"category"`
	// DeclineReason is the reason for a declined transaction, if any.
	DeclineReason string `json:"decline_reason,omitempty"`
}

// MerchantID attempts to unmarshal the Merchant field as a string ID.
// Returns the ID and true if successful, or an empty string and false.
func (t *Transaction) MerchantID() (string, bool) {
	var id string
	if err := json.Unmarshal(t.Merchant, &id); err == nil {
		return id, true
	}
	return "", false
}

// ExpandedMerchant attempts to unmarshal the Merchant field as a full Merchant object.
// Returns the Merchant object and true if successful, or nil and false.
func (t *Transaction) ExpandedMerchant() (*Merchant, bool) {
	var m Merchant
	if err := json.Unmarshal(t.Merchant, &m); err == nil && m.ID != "" {
		return &m, true
	}
	return nil, false
}

// GetTransactionResponse is the wrapper for the GetTransaction endpoint.
type GetTransactionResponse struct {
	Transaction Transaction `json:"transaction"`
}

// ListTransactionsResponse is the wrapper for the ListTransactions endpoint.
type ListTransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
}

// Merchant represents a merchant.
type Merchant struct {
	// Address is the merchant's address.
	Address Address `json:"address"`
	// Created is the timestamp when the merchant was first seen.
	Created time.Time `json:"created"`
	// GroupID is the ID of the merchant group.
	GroupID string `json:"group_id"`
	// ID is the unique identifier for the merchant.
	ID string `json:"id"`
	// Logo is the URL of the merchant's logo.
	Logo string `json:"logo"`
	// Emoji is the emoji associated with the merchant.
	Emoji string `json:"emoji"`
	// Name is the name of the merchant.
	Name string `json:"name"`
	// Category is the default category for the merchant.
	Category string `json:"category"`
}

// Address represents a physical address.
type Address struct {
	// Address is the street address.
	Address string `json:"address"`
	// City is the city.
	City string `json:"city"`
	// Country is the country.
	Country string `json:"country"`
	// Latitude is the geographic latitude.
	Latitude float64 `json:"latitude"`
	// Longitude is the geographic longitude.
	Longitude float64 `json:"longitude"`
	// Postcode is the postal code.
	Postcode string `json:"postcode"`
	// Region is the region or state.
	Region string `json:"region"`
}

// PaginationOptions provides query parameters for pagination.
type PaginationOptions struct {
	// Limit restricts the number of results returned. Max 100.
	Limit int
	// Since is an RFC3339 timestamp or a transaction ID to start from.
	Since string
	// Before is an RFC3339 timestamp to end at.
	Before string
}

// Attachment represents a file attached to a transaction.
type Attachment struct {
	// ID is the unique identifier for the attachment.
	ID string `json:"id"`
	// UserID is the ID of the user who uploaded the attachment.
	UserID string `json:"user_id"`
	// ExternalID is the ID of the transaction this attachment is linked to.
	ExternalID string `json:"external_id"`
	// FileURL is the URL of the hosted file.
	FileURL string `json:"file_url"`
	// FileType is the MIME type of the file (e.g., "image/png").
	FileType string `json:"file_type"`
	// Created is the timestamp when the attachment was created.
	Created time.Time `json:"created"`
}

// UploadAttachmentResponse is the response from the /attachment/upload endpoint.
type UploadAttachmentResponse struct {
	// FileURL is the permanent URL of the file after upload.
	FileURL string `json:"file_url"`
	// UploadURL is the temporary, pre-signed URL to PUT/POST the file data to.
	UploadURL string `json:"upload_url"`
}

// RegisterAttachmentResponse is the wrapper for the RegisterAttachment endpoint.
type RegisterAttachmentResponse struct {
	Attachment Attachment `json:"attachment"`
}

// Receipt represents a transaction receipt.
type Receipt struct {
	// ID is the unique identifier for the receipt (generated by Monzo).
	ID string `json:"id,omitempty"`
	// TransactionID is the ID of the transaction to associate with.
	TransactionID string `json:"transaction_id"`
	// ExternalID is your own unique ID for this receipt (for idempotency).
	ExternalID string `json:"external_id"`
	// Total is the total amount of the receipt in minor units.
	Total int64 `json:"total"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// Items is a list of line items on the receipt.
	Items []ReceiptItem `json:"items"`
	// Taxes is a list of taxes applied to the receipt.
	Taxes []ReceiptTax `json:"taxes,omitempty"`
	// Payments is a list of payments made for this receipt.
	Payments []ReceiptPayment `json:"payments,omitempty"`
	// Merchant contains optional merchant details for the receipt.
	Merchant *ReceiptMerchant `json:"merchant,omitempty"`
}

// ReceiptItem represents a line item on a receipt.
type ReceiptItem struct {
	// Description is the name or description of the item.
	Description string `json:"description"`
	// Quantity is the number of units. Can be fractional.
	Quantity float64 `json:"quantity,omitempty"`
	// Unit is the unit of measurement (e.g., "kg").
	Unit string `json:"unit,omitempty"`
	// Amount is the total cost of this item (quantity * unit price) in minor units.
	Amount int64 `json:"amount"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// Tax is the tax amount for this item in minor units.
	Tax int64 `json:"tax,omitempty"`
	// SubItems is a list of sub-items (e.g., toppings, modifiers).
	SubItems []ReceiptItem `json:"sub_items,omitempty"`
}

// ReceiptTax represents tax information on a receipt.
type ReceiptTax struct {
	// Description is the name of the tax (e.g., "VAT").
	Description string `json:"description"`
	// Amount is the total tax amount in minor units.
	Amount int64 `json:"amount"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// TaxNumber is the merchant's tax ID number.
	TaxNumber string `json:"tax_number,omitempty"`
}

// ReceiptPayment represents payment details on a receipt.
type ReceiptPayment struct {
	// Type is the payment type: "card", "cash", or "gift_card".
	Type string `json:"type"`
	// Amount is the amount paid in minor units.
	Amount int64 `json:"amount"`
	// Currency is the ISO 4217 currency code.
	Currency string `json:"currency"`
	// LastFour is the last four digits of the card number.
	LastFour string `json:"last_four,omitempty"`
	// GiftCardType is a description of the gift card used.
	GiftCardType string `json:"gift_card_type,omitempty"`
	// BIN is the card's Bank Identification Number.
	BIN string `json:"bin,omitempty"`
	// AuthCode is the card authorization code.
	AuthCode string `json:"auth_code,omitempty"`
	// AID is the Application Identifier.
	AID string `json:"aid,omitempty"`
	// MID is the Merchant ID.
	MID string `json:"mid,omitempty"`
	// TID is the Terminal ID.
	TID string `json:"tid,omitempty"`
}

// ReceiptMerchant represents merchant details on a receipt.
type ReceiptMerchant struct {
	// Name is the merchant's name.
	Name string `json:"name,omitempty"`
	// Online indicates if this was an online purchase.
	Online bool `json:"online,omitempty"`
	// Phone is the merchant's contact phone number.
	Phone string `json:"phone,omitempty"`
	// Email is the merchant's contact email address.
	Email string `json:"email,omitempty"`
	// StoreName is the specific store name (e.g., "Old Street").
	StoreName string `json:"store_name,omitempty"`
	// StoreAddress is the store's street address.
	StoreAddress string `json:"store_address,omitempty"`
	// StorePostcode is the store's postal code.
	StorePostcode string `json:"store_postcode,omitempty"`
}

// GetReceiptResponse is the wrapper for the GetReceipt endpoint.
type GetReceiptResponse struct {
	Receipt Receipt `json:"receipt"`
}

// Webhook represents a registered webhook.
type Webhook struct {
	// ID is the unique identifier for the webhook.
	ID string `json:"id"`
	// AccountID is the ID of the account this webhook is for.
	AccountID string `json:"account_id"`
	// URL is the destination URL for the webhook.
	URL string `json:"url"`
}

// RegisterWebhookResponse is the wrapper for the RegisterWebhook endpoint.
type RegisterWebhookResponse struct {
	Webhook Webhook `json:"webhook"`
}

// ListWebhooksResponse is the wrapper for the ListWebhooks endpoint.
type ListWebhooksResponse struct {
	Webhooks []Webhook `json:"webhooks"`
}

// WebhookEvent represents the outer envelope of an incoming webhook.
// The Data field contains the specific payload, e.g., a Transaction.
type WebhookEvent struct {
	// Type is the event type, e.g., "transaction.created".
	Type string `json:"type"`
	// Data is the payload of the event.
	Data Transaction `json:"data"`
}

//####################################################################
//## 3. API ENDPOINT METHODS
//####################################################################

// --- Authentication ---

// WhoAmI checks if the current access token is valid and returns info about it.
// This is a good way to test your authentication.
func (c *Client) WhoAmI(ctx context.Context) (*WhoAmIResponse, error) {
	var resp WhoAmIResponse
	err := c.doRequest(ctx, http.MethodGet, "/ping/whoami", nil, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Logout invalidates the current access token.
func (c *Client) Logout(ctx context.Context) error {
	return c.doRequest(ctx, http.MethodPost, "/oauth2/logout", nil, nil, nil)
}

// --- Accounts ---

// ListAccounts returns a list of accounts owned by the user.
// accountType can be used to filter ("uk_retail", "uk_retail_joint").
// Pass an empty string to list all accounts.
func (c *Client) ListAccounts(ctx context.Context, accountType string) ([]Account, error) {
	query := url.Values{}
	if accountType != "" {
		query.Set("account_type", accountType)
	}

	var resp ListAccountsResponse
	err := c.doRequest(ctx, http.MethodGet, "/accounts", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Accounts, nil
}

// --- Balance ---

// GetBalance returns the balance for a specific account.
func (c *Client) GetBalance(ctx context.Context, accountID string) (*Balance, error) {
	query := url.Values{}
	query.Set("account_id", accountID)

	var resp Balance
	err := c.doRequest(ctx, http.MethodGet, "/balance", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Pots ---

// ListPots returns a list of pots for a specific account.
func (c *Client) ListPots(ctx context.Context, accountID string) ([]Pot, error) {
	query := url.Values{}
	query.Set("current_account_id", accountID)

	var resp ListPotsResponse
	err := c.doRequest(ctx, http.MethodGet, "/pots", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Pots, nil
}

// DepositToPot moves money from an account into a pot.
// amount is in minor units (e.g., pennies).
// dedupeID is a unique string to prevent duplicate deposits.
func (c *Client) DepositToPot(ctx context.Context, potID, sourceAccountID, dedupeID string, amount int64) (*Pot, error) {
	path := fmt.Sprintf("/pots/%s/deposit", potID)
	form := url.Values{
		"source_account_id": {sourceAccountID},
		"amount":            {strconv.FormatInt(amount, 10)},
		"dedupe_id":         {dedupeID},
	}

	var resp Pot
	err := c.doRequest(ctx, http.MethodPut, path, nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// WithdrawFromPot moves money from a pot into an account.
// amount is in minor units (e.g., pennies).
// dedupeID is a unique string to prevent duplicate withdrawals.
func (c *Client) WithdrawFromPot(ctx context.Context, potID, destinationAccountID, dedupeID string, amount int64) (*Pot, error) {
	path := fmt.Sprintf("/pots/%s/withdraw", potID)
	form := url.Values{
		"destination_account_id": {destinationAccountID},
		"amount":                 {strconv.FormatInt(amount, 10)},
		"dedupe_id":              {dedupeID},
	}

	var resp Pot
	err := c.doRequest(ctx, http.MethodPut, path, nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Transactions ---

// GetTransaction retrieves a single transaction by its ID.
// Set expandMerchant to true to expand the merchant object inline.
func (c *Client) GetTransaction(ctx context.Context, transactionID string, expandMerchant bool) (*Transaction, error) {
	path := fmt.Sprintf("/transactions/%s", transactionID)
	query := url.Values{}
	if expandMerchant {
		query.Set("expand[]", "merchant")
	}

	var resp GetTransactionResponse
	err := c.doRequest(ctx, http.MethodGet, path, query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Transaction, nil
}

// ListTransactions retrieves a list of transactions for an account.
// Pagination can be controlled using the options parameter.
func (c *Client) ListTransactions(ctx context.Context, accountID string, options *PaginationOptions) ([]Transaction, error) {
	query := url.Values{}
	query.Set("account_id", accountID)

	if options != nil {
		if options.Limit > 0 {
			query.Set("limit", strconv.Itoa(options.Limit))
		}
		if options.Since != "" {
			query.Set("since", options.Since)
		}
		if options.Before != "" {
			query.Set("before", options.Before)
		}
	}

	var resp ListTransactionsResponse
	err := c.doRequest(ctx, http.MethodGet, "/transactions", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Transactions, nil
}

// AnnotateTransaction adds or updates metadata for a transaction.
// Metadata keys are prefixed with `metadata[key]`.
// To delete a key, set its value to an empty string.
func (c *Client) AnnotateTransaction(ctx context.Context, transactionID string, metadata map[string]string) (*Transaction, error) {
	path := fmt.Sprintf("/transactions/%s", transactionID)
	form := url.Values{}
	for key, val := range metadata {
		form.Set(fmt.Sprintf("metadata[%s]", key), val)
	}

	var resp GetTransactionResponse
	err := c.doRequest(ctx, http.MethodPatch, path, nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Transaction, nil
}

// --- Feed ---

// CreateFeedItem creates a new item in the user's feed.
// itemType must be "basic".
// itemURL is an optional URL to open when the feed item is tapped.
// params is a map of feed item parameters (e.g., "title", "image_url", "body").
func (c *Client) CreateFeedItem(ctx context.Context, accountID, itemType, itemURL string, params map[string]string) error {
	form := url.Values{
		"account_id": {accountID},
		"type":       {itemType},
	}
	if itemURL != "" {
		form.Set("url", itemURL)
	}
	for key, val := range params {
		form.Set(fmt.Sprintf("params[%s]", key), val)
	}

	// This endpoint returns an empty JSON object {}
	return c.doRequest(ctx, http.MethodPost, "/feed", nil, form, &struct{}{})
}

// --- Attachments ---

// UploadAttachment gets a temporary URL for uploading an attachment.
// This is the first step. The file must be POSTed/PUT to the returned UploadURL.
// fileName is the name of the file, fileType is its MIME type (e.g., "image/png"),
// and contentLength is the size of the file in bytes.
func (c *Client) UploadAttachment(ctx context.Context, fileName, fileType string, contentLength int64) (*UploadAttachmentResponse, error) {
	form := url.Values{
		"file_name":      {fileName},
		"file_type":      {fileType},
		"content_length": {strconv.FormatInt(contentLength, 10)},
	}

	var resp UploadAttachmentResponse
	err := c.doRequest(ctx, http.MethodPost, "/attachment/upload", nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// RegisterAttachment associates an uploaded file (from UploadAttachment or an
// external URL) with a transaction.
// externalID is the ID of the transaction.
// fileURL is the URL of the file (e.g., the one from UploadAttachmentResponse).
// fileType is the MIME type.
func (c *Client) RegisterAttachment(ctx context.Context, externalID, fileURL, fileType string) (*Attachment, error) {
	form := url.Values{
		"external_id": {externalID},
		"file_url":    {fileURL},
		"file_type":   {fileType},
	}

	var resp RegisterAttachmentResponse
	err := c.doRequest(ctx, http.MethodPost, "/attachment/register", nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Attachment, nil
}

// DeregisterAttachment removes an attachment from a transaction.
func (c *Client) DeregisterAttachment(ctx context.Context, attachmentID string) error {
	form := url.Values{
		"id": {attachmentID},
	}
	// Returns an empty JSON object {}
	return c.doRequest(ctx, http.MethodPost, "/attachment/deregister", nil, form, &struct{}{})
}

// --- Receipts ---

// CreateReceipt creates or updates a receipt for a transaction.
// The receipt's ExternalID is used for idempotency.
// This endpoint uses a JSON request body.
func (c *Client) CreateReceipt(ctx context.Context, receipt *Receipt) (*Receipt, error) {
	var resp Receipt
	err := c.doRequest(ctx, http.MethodPut, "/transaction-receipts", nil, receipt, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetReceipt retrieves a receipt by its external_id.
func (c *Client) GetReceipt(ctx context.Context, externalID string) (*Receipt, error) {
	query := url.Values{}
	query.Set("external_id", externalID)

	var resp GetReceiptResponse
	err := c.doRequest(ctx, http.MethodGet, "/transaction-receipts", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Receipt, nil
}

// DeleteReceipt deletes a receipt by its external_id.
func (c *Client) DeleteReceipt(ctx context.Context, externalID string) error {
	query := url.Values{}
	query.Set("external_id", externalID)
	// Returns an empty JSON object {}
	return c.doRequest(ctx, http.MethodDelete, "/transaction-receipts", query, nil, &struct{}{})
}

// --- Webhooks ---

// RegisterWebhook registers a new webhook for an account.
func (c *Client) RegisterWebhook(ctx context.Context, accountID, webhookURL string) (*Webhook, error) {
	form := url.Values{
		"account_id": {accountID},
		"url":        {webhookURL},
	}

	var resp RegisterWebhookResponse
	err := c.doRequest(ctx, http.MethodPost, "/webhooks", nil, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Webhook, nil
}

// ListWebhooks lists all webhooks for a given account.
func (c *Client) ListWebhooks(ctx context.Context, accountID string) ([]Webhook, error) {
	query := url.Values{}
	query.Set("account_id", accountID)

	var resp ListWebhooksResponse
	err := c.doRequest(ctx, http.MethodGet, "/webhooks", query, nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Webhooks, nil
}

// DeleteWebhook deletes a registered webhook by its ID.
func (c *Client) DeleteWebhook(ctx context.Context, webhookID string) error {
	path := fmt.Sprintf("/webhooks/%s", webhookID)
	// Returns an empty JSON object {}
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil, &struct{}{})
}

// ParseWebhookTransactionCreated parses a 'transaction.created' webhook
// from an incoming HTTP request.
//
// It returns the parsed Transaction and an error if the payload cannot be
// read, is invalid JSON, or is not a 'transaction.created' event.
// It is recommended to respond with a 200 OK to Monzo even if you
// encounter an error, to prevent retries.
func ParseWebhookTransactionCreated(r *http.Request) (*Transaction, error) {
	// Good practice: defer body closing
	defer r.Body.Close()

	// Good practice: limit the request body size to prevent abuse (e.g., 1MB)
	const maxBodyBytes = 1_048_576
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)

	var payload WebhookEvent

	// Create a decoder and be strict about the payload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode webhook JSON: %w", err)
	}

	// Validate the event type
	if payload.Type != "transaction.created" {
		return nil, fmt.Errorf("invalid webhook type: expected 'transaction.created', got '%s'", payload.Type)
	}

	return &payload.Data, nil
}
