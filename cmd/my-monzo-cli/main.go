package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/petermakeswebsites/go-monzo/monzo"

	"golang.org/x/oauth2"
)

// --- Configuration ---
// These must be filled in from your Monzo Developer Portal
const (
	clientID     = "YOUR_CLIENT_ID_HERE"
	clientSecret = "YOUR_CLIENT_SECRET_HERE"
	// This MUST match the "Redirect URI" you set in your Monzo client settings
	redirectURL = "http://localhost:8080/auth/callback"
)

// tokenFileName is the name of the file where we'll store the token.
const tokenFileName = "token.json"

// oauth2Config holds the static configuration for our OAuth2 flow.
var oauth2Config = &oauth2.Config{
	ClientID:     clientID,
	ClientSecret: clientSecret,
	RedirectURL:  redirectURL,
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https.auth.monzo.com/",
		TokenURL: "https://api.monzo.com/oauth2/token",
	},
}

// These are used to communicate between the main app and the
// temporary web server.
var (
	// globalState is used to prevent CSRF attacks.
	globalState string
	// tokenChan receives the token from the web handler.
	tokenChan = make(chan *oauth2.Token)
	// errChan receives an error from the web handler.
	errChan = make(chan error)
)

func main() {
	// 1. Check for command-line arguments
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	cmd := os.Args[1]
	ctx := context.Background()

	// 2. Get the API token.
	// This will either read it from a file or start the
	// full browser-based auth flow.
	token, err := getCLIToken(ctx)
	if err != nil {
		log.Fatalf("Failed to get token: %v", err)
	}

	// 3. Create the authorized HTTP client.
	// This client will automatically use the RefreshToken
	// to get new AccessTokens when needed.
	httpClient := oauth2Config.Client(ctx, token)

	// 4. Create our Monzo SDK client
	monzoClient := monzo.NewClient(httpClient)

	// 5. Run the requested command
	log.Println("---")
	switch cmd {
	case "whoami":
		runWhoAmI(ctx, monzoClient)
	case "list-accounts":
		runListAccounts(ctx, monzoClient)
	default:
		log.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}

// printUsage shows the user how to use the CLI
func printUsage() {
	fmt.Println("Usage: my-monzo-cli <command>")
	fmt.Println("Available commands:")
	fmt.Println("  whoami         - Checks authentication and shows user/client IDs")
	fmt.Println("  list-accounts  - Lists all your Monzo accounts")
}

// --- CLI Commands ---

func runWhoAmI(ctx context.Context, client *monzo.Client) {
	log.Println("Checking authentication...")
	who, err := client.WhoAmI(ctx)
	if err != nil {
		log.Fatalf("Failed to check auth: %v", err)
	}
	if who.Authenticated {
		log.Println("Success! You are authenticated.")
		log.Printf("User ID:  %s\n", who.UserID)
		log.Printf("Client ID: %s\n", who.ClientID)
	} else {
		log.Println("Authentication failed. Token may be invalid.")
	}
}

func runListAccounts(ctx context.Context, client *monzo.Client) {
	log.Println("Fetching accounts...")
	accounts, err := client.ListAccounts(ctx, "")
	if err != nil {
		log.Fatalf("Failed to list accounts: %v", err)
	}
	if len(accounts) == 0 {
		log.Println("No accounts found.")
		return
	}
	for _, acc := range accounts {
		fmt.Printf("  - ID:          %s\n", acc.ID)
		fmt.Printf("    Description: %s\n", acc.Description)
		fmt.Printf("    Created:     %s\n", acc.Created.Local())
		fmt.Println("    ---")
	}
}

// --- Token & Auth Flow Management ---

// getCLIToken is the core auth logic for the CLI.
// It tries to read a token from a file. If it can't, it
// starts the browser-based auth flow.
func getCLIToken(ctx context.Context) (*oauth2.Token, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, fmt.Errorf("could not get token path: %w", err)
	}

	// Try to read the token from the file
	token, err := readToken(tokenPath)
	if err == nil {
		log.Println("Using saved token from:", tokenPath)
		// We have a token. We're done.
		return token, nil
	}

	// No token file found, or it's invalid.
	// Start the full auth flow.
	log.Println("No valid token file found. Starting browser authentication...")

	// 1. Start the temporary local server in the background
	http.HandleFunc("/auth/callback", handleAuthCallback)
	go func() {
		// This will block until the server gets the /auth/callback request
		if err := http.ListenAndServe(":8080", nil); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 2. Generate a state token
	b := make([]byte, 16)
	rand.Read(b)
	globalState = hex.EncodeToString(b)
	authURL := oauth2Config.AuthCodeURL(globalState)

	// 3. Tell the user to open the URL
	log.Println("---")
	log.Println("Please open this URL in your browser to log in:")
	fmt.Printf("\n%s\n\n", authURL)
	log.Println("Waiting for authentication...")
	log.Println("(This will start a local server on port 8080 to catch the redirect)")
	log.Println("---")

	// 4. Wait for the token or an error from the web handler
	select {
	case token := <-tokenChan:
		log.Println("Authentication successful!")
		// 5. Save the new token
		if err := saveToken(tokenPath, token); err != nil {
			return nil, fmt.Errorf("failed to save new token: %w", err)
		}
		return token, nil
	case err := <-errChan:
		return nil, fmt.Errorf("authentication failed: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// handleAuthCallback is the HTTP handler for our temporary server.
// It runs when Monzo redirects the user back to localhost.
func handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	// 1. Check the state
	returnedState := r.FormValue("state")
	if returnedState != globalState {
		msg := "Invalid state token"
		log.Println(msg)
		http.Error(w, msg, http.StatusForbidden)
		errChan <- fmt.Errorf(msg)
		return
	}

	// 2. Get the code
	code := r.FormValue("code")
	if code == "" {
		msg := "No code returned"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		errChan <- fmt.Errorf(msg)
		return
	}

	// 3. Exchange the code for a token
	// We use context.Background() here because this handler's
	// request context (r.Context()) will be cancelled when
	// the server shuts down, but we need the token exchange
	// to complete.
	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		msg := fmt.Sprintf("Failed to exchange token: %v", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		errChan <- fmt.Errorf(msg)
		return
	}

	// 4. Send the token back to the main app
	log.Println("Token received by local server.")
	fmt.Fprintln(w, "Authentication successful! You can close this window and return to your terminal.")
	tokenChan <- token
}

// --- File Helpers ---

// getTokenPath finds the OS-specific config directory and returns
// the full path for our token file.
func getTokenPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	// e.g., /home/user/.config/my-monzo-cli/token.json
	// or C:\Users\user\AppData\Roaming\my-monzo-cli\token.json
	return filepath.Join(configDir, "my-monzo-cli", tokenFileName), nil
}

// readToken loads a token from a JSON file.
func readToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

// saveToken saves a token to a JSON file, creating directories if needed.
func saveToken(path string, token *oauth2.Token) error {
	log.Println("Saving token to:", path)
	// Create the directory (e.g., ~/.config/my-monzo-cli)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	// Create the file
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the token as JSON
	return json.NewEncoder(file).Encode(token)
}
