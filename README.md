# Go Monzo SDK

[![Go Doc](https://img.shields.io/badge/go.dev-docs-blue.svg)](https://pkg.go.dev/github.com/petermakeswebsites/go-monzo/monzo)
[![Go Report Card](https://goreportcard.com/badge/github.com/petermakeswebsites/go-monzo)](https://goreportcard.com/report/github.com/petermakeswebsites/go-monzo)

A Go (Golang) client library for interacting with the [Monzo API](https://docs.monzo.com/). It provides Go-native models, a client for all REST endpoints, and helpers for handling webhooks.

This client is designed to be used with the standard `golang.org/x/oauth2` package, allowing you to easily manage authentication and automatic token refreshes.

**Disclaimer:** This is not an official Monzo library.

## Features

* **Complete API Coverage:** Client methods for all major Monzo API resources:
    * Authentication (`WhoAmI`, `Logout`)
    * Accounts & Balance
    * Pots (List, Deposit, Withdraw)
    * Transactions (Get, List, Annotate)
    * Feed Items
    * Attachments & Receipts
    * Webhooks (Register, List, Delete)
* **Go-native Models:** Clear, documented Go structs for all API objects (e.g., `monzo.Transaction`, `monzo.Account`, `monzo.Pot`).
* **Webhook Helper:** A simple `monzo.ParseWebhookTransactionCreated()` helper to securely parse incoming webhook calls.
* **OAuth2 Ready:** Designed for use with `golang.org/x/oauth2` to handle the full auth flow.
* **Fully Tested:** Includes a comprehensive test suite using a mock API server.
* **Rich Examples:** Comes with two complete, runnable examples:
    * A full **web application** (`_examples/simple_app/`)
    * A **command-line interface (CLI)** (`cmd/my-monzo-cli/`)

## Installation

```bash
go get [github.com/petermakeswebsites/go-monzo/monzo](https://github.com/petermakeswebsites/go-monzo/monzo)
````

## 1\. Monzo Setup (Critical)

Before you can use this library, you **must** configure an OAuth client in the Monzo Developer Portal.

1.  Go to [https://developers.monzo.com/](https://developers.monzo.com/) and log in.
2.  Click on the **"Clients"** tab.
3.  Create a **"New OAuth Client"**.
4.  You will be given a **Client ID** and **Client Secret**. You will need these.
5.  Set your **Redirect URI(s)**. This is the most important step.
      * For the example web app (`_examples/simple_app`):
        `http://localhost:8080/auth/callback`
      * For the example CLI (`cmd/my-monzo-cli`):
        `http://localhost:8080/auth/callback`

### ⭐️ Important: First-Time App Approval

Due to Monzo's Strong Customer Authentication (SCA), the very first time a user authenticates with your client, they **must approve the application in their Monzo app on their phone.**

Until this is done, the API will return a `403 forbidden.insufficient_permissions` error.

**This is not a bug.** Our example applications are designed to handle this: they will show an error and ask the user to approve the app and then refresh the page.

-----

## 2\. Quick Start: Using the Client

This example shows how to list a user's accounts, assuming you have already completed the OAuth flow and have a token.

For a *complete, runnable example* that shows the full OAuth flow, please see the [Example Web App](https://www.google.com/search?q=%23example-web-app).

```go
package main

import (
	"context"
	"fmt"
	"log"

	"[github.com/petermakeswebsites/go-monzo/monzo](https://github.com/petermakeswebsites/go-monzo/monzo)"
	"golang.org/x/oauth2"
)

func main() {
	ctx := context.Background()

	// 1. Your Monzo client credentials
	conf := &oauth2.Config{
		ClientID:     "YOUR_CLIENT_ID",
		ClientSecret: "YOUR_CLIENT_SECRET",
		RedirectURL:  "YOUR_REDIRECT_URL",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "[https://auth.monzo.com/](https://auth.monzo.com/)",
			TokenURL: "[https://api.monzo.com/oauth2/token](https://api.monzo.com/oauth2/token)",
		},
	}

	// 2. An OAuth token (you would get this from the OAuth flow)
	// In a real app, you would save and load this from a database.
	token := &oauth2.Token{
		AccessToken:  "your-user-access-token",
		RefreshToken: "your-user-refresh-token",
	}

	// 3. Create an authorized HTTP client
	// This client will automatically refresh the token if it expires!
	httpClient := conf.Client(ctx, token)

	// 4. Create the Monzo SDK client
	client := monzo.NewClient(httpClient)

	// 5. Use the client!
	accounts, err := client.ListAccounts(ctx, "")
	if err != nil {
		log.Fatalf("Failed to list accounts: %v", err)
	}

	fmt.Println("Successfully fetched accounts!")
	for _, acc := range accounts {
		fmt.Printf("- %s (%s)\n", acc.Description, acc.ID)
	}
}
```

## 3\. Handling Webhooks

This library makes it easy to parse incoming webhooks (e.g., `transaction.created`).

The `monzo.ParseWebhookTransactionCreated()` helper takes an `*http.Request`, validates the payload, and returns a `*monzo.Transaction` struct.

```go
import (
    "log"
    "net/http"
    "[github.com/petermakeswebsites/go-monzo/monzo](https://github.com/petermakeswebsites/go-monzo/monzo)"
)

// handleMonzoWebhook is an http.HandlerFunc for your webhook listener
func handleMonzoWebhook(w http.ResponseWriter, r *http.Request) {

    // Use the helper to parse and validate the webhook
    transaction, err := monzo.ParseWebhookTransactionCreated(r)
    if err != nil {
        log.Printf("Error parsing Monzo webhook: %v", err)
        // It's best to still return 200 OK to Monzo so they don't
        // retry sending a payload you can't parse.
        w.WriteHeader(http.StatusOK)
        return
    }

    // Success! You have a valid transaction
    log.Printf("🎉 New Transaction! ID: %s, Amount: %d, Description: %s",
        transaction.ID,
        transaction.Amount,
        transaction.Description,
    )

    // Tell Monzo you received it successfully
    w.WriteHeader(http.StatusOK)
}

func main() {
    http.HandleFunc("/monzo-webhook", handleMonzoWebhook)
    log.Println("Starting webhook server...")
    http.ListenAndServe(":8080", nil)
}
```

## API Overview

### Client

  * `monzo.NewClient(httpClient *http.Client) *monzo.Client`

### Authentication

  * `client.WhoAmI(ctx context.Context) (*monzo.WhoAmIResponse, error)`
  * `client.Logout(ctx context.Context) error`

### Accounts & Balance

  * `client.ListAccounts(ctx context.Context, accountType string) ([]monzo.Account, error)`
  * `client.GetBalance(ctx context.Context, accountID string) (*monzo.Balance, error)`

### Pots

  * `client.ListPots(ctx context.Context, accountID string) ([]monzo.Pot, error)`
  * `client.DepositToPot(ctx context.Context, potID, sourceAccountID, dedupeID string, amount int64) (*monzo.Pot, error)`
  * `client.WithdrawFromPot(ctx context.Context, potID, destAccountID, dedupeID string, amount int64) (*monzo.Pot, error)`

### Transactions

  * `client.GetTransaction(ctx context.Context, txID string, expandMerchant bool) (*monzo.Transaction, error)`
  * `client.ListTransactions(ctx context.Context, accountID string, options *monzo.PaginationOptions) ([]monzo.Transaction, error)`
  * `client.AnnotateTransaction(ctx context.Context, txID string, metadata map[string]string) (*monzo.Transaction, error)`

### Feed

  * `client.CreateFeedItem(ctx context.Context, accountID, itemType, itemURL string, params map[string]string) error`

### Attachments

  * `client.UploadAttachment(ctx context.Context, fileName, fileType string, contentLength int64) (*monzo.UploadAttachmentResponse, error)`
  * `client.RegisterAttachment(ctx context.Context, externalID, fileURL, fileType string) (*monzo.Attachment, error)`
  * `client.DeregisterAttachment(ctx context.Context, attachmentID string) error`

### Receipts

  * `client.CreateReceipt(ctx context.Context, receipt *monzo.Receipt) (*monzo.Receipt, error)`
  * `client.GetReceipt(ctx context.Context, externalID string) (*monzo.Receipt, error)`
  * `client.DeleteReceipt(ctx context.Context, externalID string) error`

### Webhooks

  * `monzo.ParseWebhookTransactionCreated(r *http.Request) (*monzo.Transaction, error)`
  * `client.RegisterWebhook(ctx context.Context, accountID, webhookURL string) (*monzo.Webhook, error)`
  * `client.ListWebhooks(ctx context.Context, accountID string) ([]monzo.Webhook, error)`
  * `client.DeleteWebhook(ctx context.Context, webhookID string) error`

## Runnable Examples

This repository includes two complete, runnable applications to demonstrate the OAuth2 flow.

### Example Web App

See: [`_examples/simple_app/`](https://www.google.com/search?q=./_examples/simple_app/)

A simple web server that handles the full OAuth2 dance, saves the token in a cookie, and shows how to deal with the "In-App Approval" flow.

### Example CLI

See: [`cmd/my-monzo-cli/`](https://www.google.com/search?q=./cmd/my-monzo-cli/)

A command-line tool that performs the OAuth2 flow by:

1.  Starting a temporary local server.
2.  Opening your browser to log in.
3.  "Catching" the redirect.
4.  Saving the token to a file in your user config directory (e.g., `~/.config/my-monzo-cli/token.json`).
5.  Using the saved token on all future runs.

**Usage:**

```bash
# From the cmd/my-monzo-cli/ directory:
go run main.go list-accounts
go run main.go whoami
```

## License

This library is licensed under the MIT License. See the `LICENSE` file for details.
