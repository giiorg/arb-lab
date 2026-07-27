// Package engine is a context-local package that happens to be named engine.
// It is not internal/engine/, so it is not §3.5's parity seam, and importing it
// from the domain is not rule 1's violation. This is the fixture that separates
// classifying engine POSITIONALLY from classifying it by segment name.
package engine

type Reducer struct{}
