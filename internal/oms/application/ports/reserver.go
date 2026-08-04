package ports

import (
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// CapacityReserver is segregated from OrderPlacer because package admission and
// sending are different use-case capabilities. The wiring carrier is
// deliberately not split, though: one venue surface must back both capabilities
// so reservation on one governor and placement on another is inexpressible.
type CapacityReserver interface {
	// ReservePlacements returns one non-nil hold per leg, index-aligned with the
	// input. Today every leg is on one venue and each element is the same pointer;
	// the per-leg shape is the seam a future cross-venue package needs. An error
	// means nothing was reserved and nothing was sent.
	ReservePlacements(orders []domain.Order, u Urgency) ([]*ratelimit.Hold, error)
}
