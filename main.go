package main

import (
	"context"
	"fmt"
	"log"
	"time" // Added for the example

	// This import path is your module name + package name
	"github.com/petermakeswebsites/go-monzo/monzo"

	"golang.org/x/oauth2"
)

func main() {
	ctx := context.Background()

	// --- This is just an example ---
	// In a real app, you would get this token from the full
	// OAuth web flow and store it securely.
	token := &oauth2.Token{
		AccessToken:  "YOUR_ACCESS_TOKEN",
		RefreshToken: "YOUR_REFRESH_TOKEN",
		Expiry:       time.Now().Add(-1 * time.Hour), // Force a refresh
	}

	conf := &oauth2.Config{
		ClientID:     "YOUR_CLIENT_ID",
		ClientSecret: "YOUR_CLIENT_SECRET",
		RedirectURL:  "YOUR_REDIRECT_URL",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://auth.monzo.com/",
			TokenURL: "https://api.monzo.com/oauth2/token",
		},
	}
	// --- End of Example Setup ---

	// 1. Create the authorized http.Client
	httpClient := conf.Client(ctx, token)

	// 2. Create the Monzo SDK client
	client := monzo.NewClient(httpClient)

	// 3. Use the client!
	log.Println("Fetching accounts...")
	accounts, err := client.ListAccounts(ctx, "")
	if err != nil {
		log.Fatalf("Failed to list accounts: %v", err)
	}

	fmt.Println("Successfully fetched accounts!")
	for _, acc := range accounts {
		fmt.Printf("- Account ID: %s, Description: %s\n", acc.ID, acc.Description)
	}
}
