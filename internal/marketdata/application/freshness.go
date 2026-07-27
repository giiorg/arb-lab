package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// assessFreshness computes §3.4's two verdicts for one observation.
//
// # One function because there is one rule, and it is the rule that reads wrong
//
// §3.4: "One budget per (data class, venue), measured TWICE." The transport
// measurement asks whether anything ARRIVED — RecvTime against Clock.Now — and
// the data measurement asks whether the venue's own view is MOVING — SourceTime
// against Clock.ServerNow(venue). A venue-side stall keeps updates arriving on
// schedule with a frozen SourceTime, so the transport verdict reads perfectly
// fresh for the entire outage.
//
// The half a reader gets wrong is the second one's clock. Comparing a venue's
// SourceTime against engine time is forbidden for §3.5's reason — it is the
// unsynchronized-clock comparison with one venue instead of two — and it is the
// comparison a person writes first, because Clock.Now is already in hand. Every
// path that measures freshness calls this rather than writing the pair out, so
// there is one place that can be wrong and one place to check.
//
// # A clock inversion is an ERROR here, not a verdict
//
// The domain returns FreshnessUnknown alongside its error so a caller that
// ignores the error still de-levers. This layer does not ignore it: an
// Unknown-verdict publication is indistinguishable downstream from a venue that
// has never connected, and §3.6 rule 5's two-disagreeing-clocks defect deserves
// to name itself rather than arrive as a feed that appears to have gone dark.
func assessFreshness(
	c clock.Clock, b domain.Budget, w domain.Watermark, inst ids.VenueInstrument,
) (transport, data domain.Freshness, err error) {
	transport, err = b.Assess(w, c.Now())
	if err != nil {
		return 0, 0, fmt.Errorf("instrument %s transport freshness: %w", inst, err)
	}
	data, err = b.AssessData(w, c.ServerNow(inst.Venue).Instant())
	if err != nil {
		return 0, 0, fmt.Errorf("instrument %s data freshness: %w", inst, err)
	}
	return transport, data, nil
}
