// Package usecases is a SUBpackage of the application tree. It exists to
// separate the surface's tree arm from its exact arm: if internal/oms/application
// were matched exactly, an import of this package would be silent while carrying
// the same authority.
package usecases

// Delever is SM §7's ladder.
type Delever struct{}
