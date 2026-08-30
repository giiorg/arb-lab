// Package main is a binary OUTSIDE internal/ that is NOT granted, and it is the
// fixture that separates the two ways to resolve the module root.
//
// cmd/arblab is granted and clean, which cannot do this job: resolve the root
// from the ANALYZED package instead of from the imported one and cmd/arblab
// stays silent too — the derived root is its own path, every policed dir built
// from it is strictly longer than the import, and nothing matches. Silent for
// the right reason and silent for the wrong one are the same output, so the
// separating fixture has to be one that must NOT be silent.
package main

import "github.com/giiorg/arb-lab/internal/oms" // want `not granted OMS write authority.*OMS control surface`

var _ oms.Module

func main() {}
