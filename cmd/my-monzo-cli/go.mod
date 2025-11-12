module my-monzo-cli

go 1.25.3

require (
	// 1. It needs the official Google OAuth2 library
	golang.org/x/oauth2 v0.20.0

	// 2. It needs YOUR Monzo library
	// (Replace with your actual GitHub username)
	github.com/your-username/go-monzo v1.0.0
)

// !!! IMPORTANT FOR LOCAL DEVELOPMENT !!!
// This "replace" directive tells Go: "When you look for the module
// 'github.com/your-username/go-monzo', DON'T download it from GitHub.
// Instead, use the local directory '../..' (two levels up)."
//
// You can remove this line after you've published your library.
replace github.com/your-username/go-monzo => ../../
