// This is the go.mod file for your *example application*.
// It lists its own dependencies.
module my-monzo-example

go 1.25.3

require (
	// 1. It needs the official Google OAuth2 library
	golang.org/x/oauth2 v0.20.0

	// 2. It needs YOUR Monzo library
	github.com/your-username/go-monzo v1.0.0
)

// !!! IMPORTANT !!!
// This "replace" directive tells Go: "When you look for the module
// 'github.com/your-username/go-monzo', DON'T download it from GitHub.
// Instead, use the local directory '../' (one level up from this one)."
//
// This is how you test your library locally before you ever publish it!
replace github.com/your-username/go-monzo => ../../
