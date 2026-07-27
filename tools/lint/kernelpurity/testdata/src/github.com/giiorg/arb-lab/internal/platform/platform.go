// Package platform sits at EXACTLY internal/platform, the kernel root, rather
// than beneath it.
//
// It is the fixture for within()'s equality arm. Every other fixture here is a
// SUBpackage — internal/platform/clock, /fixedpoint, /leaky — all of which the
// prefix arm matches on its own, so `pkgPath == dir ||` could be deleted with
// the whole suite still green. Found by mutation in session 15, while
// bite-checking hexboundary, which copied this helper and inherited the gap
// along with it.
//
// The kernel root having no package today is why the arm went unmeasured; it is
// kept rather than deleted because the day somebody adds internal/platform/
// platform.go is exactly the day an unpoliced kernel package appears.
package platform

import "github.com/giiorg/arb-lab/internal/oms/domain" // want `platform must not import`

var _ = domain.Name
