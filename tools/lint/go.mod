module github.com/giiorg/arb-lab/tools/lint

go 1.26.0

// Mirrors the root module deliberately. There is no go.work, so each module
// resolves its own toolchain; leaving this one unstated would let the two
// halves of `make ci` build on different releases.
toolchain go1.26.5

require golang.org/x/tools v0.48.0

require (
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
)
