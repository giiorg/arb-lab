// Package engine is published by somebody else and has nothing to do with
// §3.5's parity seam. It is the fixture that separates "classify by module-root
// position" from "scan the path for a segment named engine": under the latter,
// a domain package importing this is reported, and the diagnostic is invented.
package engine

type Widget struct{}
