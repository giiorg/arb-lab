// Package engineering is the substring trap for the engine check. It sits
// directly under internal/, so a prefix comparison that did not compare whole
// segments would classify it as the parity seam and report an import of it.
package engineering

type Note struct{}
