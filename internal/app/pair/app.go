package pair

import (
	"context"
	"fmt"
	"io"

	"github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// Run performs one cross-venue pair cycle: two reconcile-only boots, and — if
// and only if BOTH verdicts are ExitClean AND both legs' instruments are flat
// at their own venues — the paired drive.
//
// # The error contract is reconcile.Run's
//
// An error means a boot could not be PERFORMED. Every refusal, including the
// gate below, is in the Report.
//
// # Both boots and both venue-position reads finish before EITHER side is touched
//
// A drive that began against one clean boot and one unknown one is exactly the
// half-open pair this root exists to avoid producing, arrived at on purpose.
// The two boots are sequential rather than concurrent, and the choice is
// deliberate: each acquires a spend lock, opens a journal and makes signed
// venue calls, and running them concurrently would interleave two boots'
// diagnoses on one stderr with no way to tell which venue refused. There is no
// latency argument for a reconcile-only boot.
//
// # It builds its own wall, and does not take one
//
// D9: one *clock.Wall over BOTH sides' calibrations. A caller could only supply
// a wall built from one side's document, which would leave the other venue's
// frame uncalibrated — and §5.3's two time bounds are measured in the venue's
// frame, so an uncalibrated side resolves its orders against this host's clock.
func Run(ctx context.Context, snap config.Snapshot[Config], newRegistry func() *observability.Registry,
	stderr io.Writer) (Report, error) {
	return runWithWait(ctx, snap, newRegistry, stderr)
}

// runWithWait is Run with the drive's blocking wait injected. See Run.
func runWithWait(ctx context.Context, snap config.Snapshot[Config],
	newRegistry func() *observability.Registry, stderr io.Writer) (Report, error) {

	cfg := snap.Value

	// One wall over BOTH venues' calibrations (D9). clock.NewWall refuses a
	// duplicate venue, which backstops Validate's ErrSideVenueDuplicate — but
	// Validate is what NAMES the config field, so both exist.
	cals := append(cfg.Sides[0].OMS.Calibrations(), cfg.Sides[1].OMS.Calibrations()...)
	wall, err := clock.NewWall(cals)
	if err != nil {
		return Report{}, err
	}

	// The §2.4 filters, resolved BEFORE either boot exists, so a bad reference
	// revision refuses the RUN with the file named — before any venue call,
	// any fsync and any spend lock.
	filters, err := cfg.tradingFilters(wall)
	if err != nil {
		return Report{}, err
	}

	var rep Report
	var runners [2]*reconcile.Runner
	var running [2]*boot.Running

	// Teardown is deferred over both sides in reverse, and its failures are
	// reported rather than returned: a journal that will not close is a real
	// problem and it is not a reason to discard the verdicts just established.
	defer func() {
		for i := len(running) - 1; i >= 0; i-- {
			if running[i] == nil {
				continue
			}
			shutdown, _ := running[i].Shutdown(ctx, wall)
			for _, p := range shutdown.Phases {
				if p.Err != nil {
					fmt.Fprintf(stderr, "pair: side %d phase %q close: %v\n", i, p.Name, p.Err)
				}
			}
		}
	}()

	for i := range cfg.Sides {
		rep.Sides[i].Venue = cfg.Sides[i].OMS.Venue.Venue
		bootSnap := config.Snapshot[reconcile.Config]{Value: cfg.Sides[i].OMS, Rev: snap.Rev}
		// A registry PER SIDE: a collector registered by one side must not be
		// able to refuse the other side's boot.
		r, err := reconcile.NewRunner(bootSnap, wall, newRegistry(), stderr,
			reconcile.WithInstrumentFilters(filters[i], cfg.Reference.MaxAge.Duration()))
		if err != nil {
			return rep, err
		}
		runners[i] = r
		live, err := r.Sequence().Run(ctx)
		if err != nil {
			rep.Sides[i].Boot = r.BootReport()
			return rep, err
		}
		running[i] = live
		rep.Sides[i].Boot = r.BootReport()
		rep.Sides[i].RateLimit = r.RateLimit()
	}

	// The gate. Both, before either.
	for i := range rep.Sides {
		if class := rep.Sides[i].Boot.ExitClass(); class != reconcile.ExitClean {
			rep.Refusal = fmt.Errorf("side %d (%s) boot verdict is %s: %w",
				i, rep.Sides[i].Venue, class, ErrBootNotClean)
			return rep, nil
		}
		if runners[i].Halted() || runners[i].Module() == nil {
			rep.Refusal = fmt.Errorf("side %d (%s) boot halted: %w", i, rep.Sides[i].Venue, ErrBootNotClean)
			return rep, nil
		}
	}

	// A clean boot is not evidence of a flat account. Net-delta asserts only
	// DISAGREEMENT between venue and journal, and a fresh journal agrees with
	// nothing; Reconciliation.Assertion's zero value also means the assertion
	// did not run. So each leg's VENUE position is read directly, because the
	// reduce-only inverse this root unwinds with is legal as RESTORATION only
	// when the opening leg created the entire position it closes.
	for leg := range cfg.Pair.Legs {
		side, _ := cfg.sideFor(cfg.Pair.Legs[leg].Venue) // Validate made this total.
		intent, _ := cfg.legIntent(leg)                  // Validate made this total.
		scope, err := positionScopeFor(cfg.Sides[side].OMS, intent.Instrument)
		if err != nil {
			return rep, fmt.Errorf("side %d (%s): %w", side, rep.Sides[side].Venue, err)
		}
		pos, err := runners[side].Module().VenuePosition(scope, intent.Instrument)
		if err != nil {
			return rep, fmt.Errorf("side %d (%s) starting position: %w", side, rep.Sides[side].Venue, err)
		}
		sign, err := pos.Net.Sign()
		if err != nil {
			return rep, fmt.Errorf("side %d (%s) starting position: %w", side, rep.Sides[side].Venue, err)
		}
		if sign != 0 {
			rep.Refusal = fmt.Errorf("side %d (%s) starts at %s on %s: %w",
				side, rep.Sides[side].Venue, pos.Net, intent.Instrument, ErrPositionNotFlat)
			return rep, nil
		}
	}

	rep.Drove = true
	return rep, nil
}

// positionScopeFor is the configured enumeration covering one instrument.
//
// A missing scope is a WIRING DEFECT and is returned as an error. It is not
// treated as "nothing to enumerate": the enumeration is how this root learns
// the account is flat, so an absent scope would turn "we never asked" into
// "we asked and it was flat" — the concluding direction, on the read the whole
// unwind rests on.
func positionScopeFor(side reconcile.Config, instrument ids.VenueInstrument) (application.PositionScope, error) {
	for _, s := range side.PositionScopes {
		if s.Venue == instrument.Venue && s.Product == instrument.Product {
			return s.Scope(), nil
		}
	}
	return application.PositionScope{}, fmt.Errorf(
		"no position_scopes row covers %s; the starting-position read has no enumeration to make", instrument)
}
