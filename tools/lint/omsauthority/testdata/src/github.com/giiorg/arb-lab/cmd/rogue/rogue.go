// Package main is a binary OUTSIDE internal/ that is NOT granted, and it is the
// fixture that separates the two ways to resolve the module root.
//
// It has to be a package that must NOT be silent, and that requirement is what
// this fixture was rewritten around. The suite used to carry a GRANTED cmd/
// package beside this one, mirroring the cmd/arblab grant F51 deleted, and it
// could not do this job: resolve the root from the ANALYZED package instead of
// from the imported one and a cmd/ package stays silent too — the derived root
// is its own path, every policed dir built from it is strictly longer than the
// import, and nothing matches. Silent for the right reason and silent for the
// wrong one are the same output.
//
// So this is the only cmd/ fixture, deliberately. The granted twin was deleted
// with the grant rather than re-pointed at the surface: reported-and-outside-
// internal/ is one predicate, and a second fixture asserting it would read in
// review as covering something this one does not.
package main

import "github.com/giiorg/arb-lab/internal/oms" // want `not granted OMS write authority.*OMS control surface`

var _ oms.Module

func main() {}
