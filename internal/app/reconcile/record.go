package reconcile

import (
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// RecordBaseline is `arblab baseline record`'s composition: the SAME deployment
// config the reconcile boot runs off — the field's optionality (§70g) is what
// makes that possible on a virgin deployment — wired at the RECORD command's
// rungs, and the six-step protocol run once (task #179, record §70b).
//
// # Everything reads ROUTINE here, and that is the point of the constructor pins
//
// RecoveryReadPriority is Routine (the protocol's join and settle re-read can
// issue up to 32 pages per order) and the protocol builds its own Routine
// balance adapter — an operator maintenance sweep must be shed BEFORE the
// emergency reserve, because the reserve it would otherwise drain is the
// flatten's. The boot wires the same surfaces at Safety; the two call sites
// on two rungs are why the priorities are constructor arguments at all.
//
// # The prior pin rides as PriorPin, never as SpotBaselineDocument
//
// The boot hard-fails on a pinned document that will not load (a wiring
// defect there); the record command loads it BEST-EFFORT for the
// absorbed-delta print, because re-recording is the REMEDY for a prior that
// no longer verifies and must stay runnable exactly then.
//
// The transport is a NewSignedClient, so the command CANNOT run without
// holding the venue's one-spender lock: a second lock-aware spending process
// on this host cannot execute trades mid-record. Across hosts the standing
// operational premise applies, unchanged.
func RecordBaseline(cfg Config, clk clock.Clock, outPath string) (oms.RecordedBaseline, error) {
	var zero oms.RecordedBaseline
	if err := cfg.Validate(); err != nil {
		return zero, err
	}
	wiring, err := newOrderClient(cfg, clk)
	if err != nil {
		return zero, err
	}

	deps := oms.Deps{
		JournalPath:      cfg.JournalPath,
		VenueOrders:      wiring.client,
		VenueOrderReads:  wiring.client,
		VenuePositions:   wiring.client,
		PositionScopes:   cfg.scopes(),
		InstrumentAssets: cfg.assets(),
		Clock:            clk,
		Account:          cfg.Account,
		Resolution:       cfg.policy(),

		RecoveryReadPriority: ratelimit.PriorityRoutine,
		SpotBalances:         wiring.client,
	}
	if settle := cfg.SpotSettle.Duration(); settle > 0 {
		deps.SettleWait = time.Sleep
		deps.SpotSettle = settle
	}
	module, err := oms.New(deps)
	if err != nil {
		return zero, errors.Join(err, wiring.spend.Close())
	}

	recordDeps := oms.RecordBaselineDeps{
		Fingerprint: wiring.fingerprint,
		Dialect:     baseline.Dialect(cfg.Venue.Dialect),
		SettleWait:  time.Sleep,
		SpotSettle:  cfg.SpotSettle.Duration(),
	}
	if cfg.SpotBaseline != nil {
		recordDeps.PriorPin = &baseline.Pin{
			Path:   cfg.SpotBaseline.Path,
			SHA256: cfg.SpotBaseline.SHA256,
		}
	}
	recorded, err := module.RecordSpotBaseline(outPath, recordDeps)
	closeErr := errors.Join(module.Close(), wiring.spend.Close())
	if err != nil {
		return zero, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return zero, fmt.Errorf("record baseline teardown: %w", closeErr)
	}
	return recorded, nil
}
