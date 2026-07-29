module github.com/giiorg/arb-lab

go 1.26.0

// `go` is the MINIMUM and the language semantics; `toolchain` is what we
// actually build with. Keeping them apart matters here because .github/ci.yml
// installs from this file (go-version-file: go.mod) — setup-go documents that
// it prefers the toolchain directive when present, so CI and a local run stay
// on the same patch release without narrowing who can build the module.
toolchain go1.26.5

require pgregory.net/rapid v1.3.0

require github.com/coder/websocket v1.8.15
