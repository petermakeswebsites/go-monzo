package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"

	// Import the official Google OAuth2 library
	"golang.org/x/oauth2"

	// Import YOUR Monzo SDK package
	// The import path MUST match your module path + subdirectory
	"github.com/petermakeswebsites/go-monzo/monzo"
)

// --- Configuration ---
// These must be filled in from your Monzo Developer Portal
// (https://developers.monzo.com/)

const (
	clientID     = "YOUR_CLIENT_ID_HERE"
	clientSecret = "YOUR_CLIENT_SECRET_HERE"
	// This MUST match the "Redirect URI" you set in your Monzo client settings
	redirectURL = "http://localhost:8080/auth/callback"
)

// --- Globals ---

var oauth2Config = &oauth2.Config{
	ClientID:     clientID,
	ClientSecret: clientSecret,
	RedirectURL:  redirectURL,
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://auth.monzo.com/",
		TokenURL: "https://api.monzo.com/oauth2/token",
	},
}

// In a real app, this state should be stored in a short-lived,
// encrypted cookie or user session.
var globalState string

// We'll store the token in a cookie named "monzo-token"
const tokenCookieName = "monzo-token"

func main() {
	// --- Our web server's routes ---
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/auth/login", handleLogin)
	http.HandleFunc("/auth/callback", handleCallback)

	// NEW: A "safe" page to land on after the callback
	http.HandleFunc("/dashboard", handleDashboard)
	// NEW: A way to log out
	http.HandleFunc("/logout", handleLogout)

	// --- Start the server ---
	log.Println("Starting example server on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// handleHome serves the home page.
// It checks if the user is already logged in (has a cookie).
func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check if we already have a token
	_, err := r.Cookie(tokenCookieName)
	if err == nil {
		// User is logged in
		fmt.Fprint(w, `
			<h2>Monzo API Example App</h2>
			<p>You are already logged in.</p>
			<p><a href="/dashboard">Go to Dashboard</a></p>
			<p><a href="/logout">Logout</a></p>
		`)
		return
	}

	// User is not logged in
	fmt.Fprint(w, `
		<h2>Monzo API Example App</h2>
		<p>Click the link below to log in with your Monzo account.</p>
		<a href="/auth/login">Login with Monzo</a>
	`)
}

// handleLogin starts the OAuth flow by redirecting the user to Monzo.
func handleLogin(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	rand.Read(b)
	globalState = hex.EncodeToString(b)
	url := oauth2Config.AuthCodeURL(globalState)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleCallback is the endpoint Monzo redirects to after login.
// Its ONLY job is to exchange the code for a token and save it.
func handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// 1. Check state
	returnedState := r.FormValue("state")
	if returnedState != globalState {
		log.Println("Invalid state token")
		http.Error(w, "Invalid state token.", http.StatusForbidden)
		return
	}

	// 2. Get code
	code := r.FormValue("code")
	if code == "" {
		http.Error(w, "No code returned.", http.StatusBadRequest)
		return
	}

	// 3. Exchange for token
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange token: %v\n", err)
		http.Error(w, "Failed to exchange token.", http.StatusInternalServerError)
		return
	}

	// 4. *** NEW: Save the token in a cookie ***
	// In a real app, you would encrypt this!
	// We save the AccessToken. The oauth2 library is smart
	// enough to use the RefreshToken (if we saved that too)
	// to get a new one when this expires.
	cookie := &http.Cookie{
		Name:     tokenCookieName,
		Value:    token.AccessToken,
		Path:     "/",
		HttpOnly: true, // Makes it safer (not accessible by JS)
		// Secure: true, // Uncomment this in production (requires HTTPS)
	}
	http.SetCookie(w, cookie)
	log.Println("Token exchanged and saved to cookie.")

	// 5. *** NEW: Redirect to a safe dashboard page ***
	// This prevents the "token has been used" error on refresh.
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// handleDashboard is our new, "safe" landing page.
// It can be refreshed without breaking the OAuth flow.
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// 1. Read the token from the cookie
	cookie, err := r.Cookie(tokenCookieName)
	if err != nil {
		// No cookie, user is not logged in.
		log.Println("No token cookie found, redirecting to home.")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	accessToken := cookie.Value
	if accessToken == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	// 2. Create a "static" token source
	// We just have the access token string, so we build
	// a simple oauth2.Token object.
	token := &oauth2.Token{
		AccessToken: accessToken,
		// We're not saving the refresh token or expiry in this
		// simple example, but in a real app you would!
	}

	// 3. Create the HTTP client
	// The oauth2.Config adds the *context* (including your
	// client_id and secret) needed to refresh the token.
	httpClient := oauth2Config.Client(ctx, token)

	// 4. Create YOUR Monzo SDK client
	monzoClient := monzo.NewClient(httpClient)

	// 5. Use the client!
	accounts, err := monzoClient.ListAccounts(ctx, "")
	if err != nil {
		// *** THIS IS THE NEW UX ***
		// If it fails, we show an error and tell the user
		// to approve the app and refresh THIS page.
		log.Printf("Failed to list accounts (needs approval?): %v\n", err)

		fmt.Fprintln(w, "<h2>Error Fetching Accounts</h2>")
		fmt.Fprintln(w, "<p>Could not fetch your accounts. This is normal if it's your first time logging in.</p>")
		fmt.Fprintln(w, "<p><b>ACTION REQUIRED:</b> Please open your Monzo app on your phone and approve this application.</p>")
		fmt.Fprintln(w, "<p>Once approved, just refresh this page.</p>")
		fmt.Fprintf(w, "<hr><p>Error details: %v</p>", err)
		return
	}

	// 6. Success!
	fmt.Fprintln(w, "<h2>Successfully Fetched Accounts!</h2>")
	fmt.Fprintln(w, "<p>Your Accounts:</p><ul>")
	for _, acc := range accounts {
		fmt.Fprintf(w, "<li>%s (%s)</li>", acc.Description, acc.ID)
	}
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, `<p><a href="/logout">Logout</a></p>`)
}

// handleLogout clears the cookie.
func handleLogout(w http.ResponseWriter, r *http.Request) {
	// Expire the cookie by setting its MaxAge to -1
	cookie := &http.Cookie{
		Name:   tokenCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(w, cookie)

	log.Println("User logged out.")
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
