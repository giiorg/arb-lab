// Package fixedpoint is a fixture proving platform -> platform is allowed.
// The dependency is not a real one; it exists only to exercise the rule.
package fixedpoint

import (
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

var _ = clock.Now
