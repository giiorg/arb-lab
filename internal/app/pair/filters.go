package pair

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

// tradingFilters resolves BOTH sides' instruments from ONE reference revision —
// the maps the two boots thread into their own oms.Deps.InstrumentFilters,
// where each side's planner and executor verify every leg against its venue's
// grid.
//
// # This is the cross-context adapter, and THIS root is where it may live
//
// The argument is exercise's, unchanged: the revision document and its as-of
// store belong to strategy (§40d), the vocabulary the OMS verifies against
// belongs to oms/domain, and app/reconcile's import fence forbids that root
// from touching strategy — §7.5's boot must not acquire a collaborator from a
// context whose posture it has not yet decided. So the adapter sits in the
// binary that PLANS and hands each boot only the finished map.
//
// # ONE revision for two venues, which is this root's own addition
//
// Every time this root sizes a fill on one venue as an order on the other it
// compares two grids implicitly. Two revisions of different ages would put that
// comparison across two observation instants, so a hedge could be refused by a
// grid that had moved while the other side's row still said it had not.
//
// # The returned slice is indexed by SIDE, and each map holds only its own rows
//
// Each side's planner verifies against the grid its own venue will judge the
// order by. A map holding the other venue's rows would let a leg validate
// against a filter no venue applies to it — and on this root that is not a
// hypothetical, because the two sides trade the SAME SYMBOL and the maps are
// keyed by ids.VenueInstrument, so only the venue component tells the two rows
// apart.
//
// # Everything-or-error, before either boot starts
//
// Every instrument either document declares must resolve to a fresh,
// tick-bearing row, or the run refuses here — named against the file, before
// any venue call or fsync. The two refusals are exercise's and their arguments
// are unchanged: a tick-less row (ErrFilterTickUnrecorded) is a missing
// constraint to a consumer that prices LIMIT orders, and a row older than
// Reference.MaxAge (ErrFilterRowStale) re-opens the asymmetric-rejection window
// behind a green run.
func (c Config) tradingFilters(clk clock.Clock) ([]map[ids.VenueInstrument]omsdomain.TradingFilters, error) {
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

	out := make([]map[ids.VenueInstrument]omsdomain.TradingFilters, len(c.Sides))
	for side := range c.Sides {
		rows := c.Sides[side].OMS.Instruments
		out[side] = make(map[ids.VenueInstrument]omsdomain.TradingFilters, len(rows))
		for _, row := range rows {
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
			out[side][inst] = omsdomain.TradingFilters{
				Step:        lots.Step,
				MinQty:      lots.MinQty,
				Tick:        lots.Tick,
				MinNotional: lots.MinNotional,
				// The row's own observation instant, carried into the OMS
				// vocabulary so the executor can re-judge the age at every
				// package admission (§69) — the check above proves freshness
				// only at THIS instant.
				ObservedAt: lots.Provenance.ObservedAt,
			}
		}
	}
	return out, nil
}
