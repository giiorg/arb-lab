// Package sub is a package BENEATH a policed root.
//
// It exists so that within()'s two arms are exercised by different fixtures:
// an import of ports itself takes the equality arm, and an import of this one
// takes the prefix arm. That distinction is untested in both hexboundary and
// the kernelpurity it was copied from, because every fixture in either imported
// a subpackage — this suite carries both from the start.
package sub

// Repo stands in for a repository declared under a policed root rather than at
// it, which is where the second one lands when the first package fills up.
type Repo interface{ Apply() error }
