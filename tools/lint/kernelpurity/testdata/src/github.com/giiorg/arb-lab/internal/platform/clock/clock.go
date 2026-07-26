// Package clock is a clean shared-kernel package: stdlib imports only.
package clock

import (
	"time"
)

func Now() time.Time { return time.Time{} }
