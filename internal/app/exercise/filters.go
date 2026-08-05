package exercise

import (
	"fmt"
	"os"

	omsdomain "github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	strategydriven "github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// tradingFilters resolves every configured instrument's §2.4 trading filters
// from the committed reference revision — the map the boot threads into
// oms.Deps.InstrumentFilters, where the planner and the executor verify every
// leg against it.
//
// # This is the cross-context adapter, and THIS root is where it may live
//
// The revision document and its as-of store belong to strategy (§40d: one
// dated-revision seam for fees, lot rules and the hurdle); the vocabulary the
// OMS verifies against belongs to oms/domain. app/reconcile's import fence
// forbids that root from touching strategy — §7.5's boot must not acquire a
// collaborator from a context whose posture it has not yet decided — so the
// adapter sits in the binary that PLANS, and hands the boot only the finished
// map through reconcile.WithInstrumentFilters. A future engine binary
// composes the same way.
//
// # Everything-or-error, before the boot starts
//
// Every instrument the document declares must resolve to a fresh,
// tick-bearing row, or the run refuses here — named against the file, before
// any venue call or fsync. The alternative — skip the bad row and let the
// planner refuse later — reports the wiring defect on a real order's refusal,
// attributed to the trade instead of the file.
//
// # The two refusals that are this function's own
//
// A row without a tick (ErrFilterTickUnrecorded): the strategy domain admits
// tick-less rows so revisions journaled before the field existed keep
// decoding, and its scoring path never prices with one. This consumer prices
// LIMIT orders, so for it a missing tick is a missing constraint — the
// flattering direction — and the guard is restated here in terms of its
// consumer rather than widened in the domain.
//
// A row older than Reference.MaxAge (ErrFilterRowStale): filters move
// server-side without notice, and every check this slice adds passes against
// a stale row while the venue judges the current one — the
// asymmetric-rejection window re-opened behind a green run. The bound is the
// operator's declared tolerance for that window; the fix for tripping it is a
// fresh probe into the revision, not a wider bound.
func (c Config) tradingFilters(clk clock.Clock) (map[ids.VenueInstrument]omsdomain.TradingFilters, error) {
	path := c.Reference.Path
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reference %s: %v: %w", path, err, ErrReferenceUnreadable)
	}
	now := clk.Now()
	store, err := config.New[strategydriven.Document](path, now, data)
	if err != nil {
		return nil, fmt.Errorf("reference %s: %w", path, err)
	}
	ref, err := strategydriven.NewReferenceStore(store)
	if err != nil {
		return nil, fmt.Errorf("reference %s: %w", path, err)
	}

	out := make(map[ids.VenueInstrument]omsdomain.TradingFilters, len(c.OMS.Instruments))
	for _, row := range c.OMS.Instruments {
		inst := row.Instrument()
		lots, err := ref.LotRules(inst, now)
		if err != nil {
			return nil, fmt.Errorf("reference %s: %w", path, err)
		}
		if lots.Tick == (fixedpoint.Price{}) {
			return nil, fmt.Errorf("reference %s row for %s: %w", path, inst, ErrFilterTickUnrecorded)
		}
		if age := now.Sub(lots.Provenance.ObservedAt); age > c.Reference.MaxAge.Duration() {
			return nil, fmt.Errorf("reference %s row for %s observed %s, %s ago against a %s bound: %w",
				path, inst, lots.Provenance.ObservedAt.Format("2006-01-02"),
				age.Round(1_000_000_000), c.Reference.MaxAge.Duration(), ErrFilterRowStale)
		}
		out[inst] = omsdomain.TradingFilters{
			Step:        lots.Step,
			MinQty:      lots.MinQty,
			Tick:        lots.Tick,
			MinNotional: lots.MinNotional,
			// The row's own observation instant, carried into the OMS
			// vocabulary so the executor can re-judge the age at every
			// package admission (§69) — the boot check above proves
			// freshness only at THIS instant.
			ObservedAt: lots.Provenance.ObservedAt,
		}
	}
	return out, nil
}
