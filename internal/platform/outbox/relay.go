package outbox

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
)

// RelayConfig wires a Relay. Every field is required; there are no defaults,
// because each one that could have a default has a wrong value that looks
// plausible (a batch of 1 works and is 100× too slow; a backoff of a
// microsecond works and is a hot loop).
type RelayConfig struct {
	Store     Store
	Publisher Publisher

	// Clock is the injected time seam. The relay reads it once per pass and
	// uses that single instant for both the claim cutoff and every computed
	// RetryAt, so a pass is internally consistent even if it is slow.
	Clock clock.Clock

	// Batch is the maximum number of records claimed per pass.
	Batch int

	Backoff Backoff
}

// Relay is the at-least-once pump: claim, publish, settle.
//
// # It owns no goroutine, and that is deliberate
//
// The obvious design — the relay runs its own ticker and drains continuously —
// would make the instant of every publish a function of wall-clock scheduling,
// which is not reproducible. platform/clock made the same call for the same
// reason (its timer delivery is pull-based and it contains no goroutines at
// all): whoever owns the total order owns the cadence.
//
// So RunOnce does exactly one pass and returns what happened. A live deployment
// pumps it from the composition root; a backtest pumps it from the dispatcher,
// on the same code path.
//
// A Relay is stateless between passes by construction. Everything that must
// survive a restart — attempt counts, retry instants — lives on the record in
// the store, so a process that crashes mid-backoff does not resume at attempt
// zero and start hammering a consumer that has been rejecting for an hour.
type Relay struct {
	store   Store
	pub     Publisher
	clk     clock.Clock
	batch   int
	backoff Backoff
}

// NewRelay validates the wiring and returns a Relay.
func NewRelay(cfg RelayConfig) (*Relay, error) {
	if cfg.Store == nil {
		return nil, ErrStoreNil
	}
	if cfg.Publisher == nil {
		return nil, ErrPublisherNil
	}
	if cfg.Clock == nil {
		return nil, ErrClockNil
	}
	if cfg.Batch <= 0 {
		return nil, fmt.Errorf("batch %d: %w", cfg.Batch, ErrBatchInvalid)
	}
	if err := cfg.Backoff.Validate(); err != nil {
		return nil, err
	}
	return &Relay{
		store:   cfg.Store,
		pub:     cfg.Publisher,
		clk:     cfg.Clock,
		batch:   cfg.Batch,
		backoff: cfg.Backoff,
	}, nil
}

// Result is what one pass did. It is the input to the export-lag and
// backlog metrics, and to the assertion a test makes.
//
// Claimed is not guaranteed to equal Published+Failed+Blocked: a pass cut short
// by a cancelled context leaves the remainder untouched, which redelivers them.
// The identity holding is a property of a pass that ran to completion, not an
// invariant of the type.
type Result struct {
	// Claimed is how many records the store handed over.
	Claimed int

	// Published is how many were delivered AND acked-for. A record counted here
	// whose Ack transaction then failed is still counted — the publish happened,
	// and the consumer will see the redelivery as a duplicate.
	Published int

	// Failed is how many were attempted and rejected, or found invalid.
	Failed int

	// Blocked is how many were skipped because an earlier record on the same
	// key failed in this pass. They were not attempted and their attempt counts
	// are untouched.
	Blocked int
}

// RunOnce performs one claim → publish → settle pass.
//
// # A non-nil error does NOT mean nothing was exported
//
// Per-key FIFO means one key's failure must not stop another key's progress, so
// the pass continues past a failure and joins the errors. Callers alert on the
// error and read the Result for what actually moved; treating a non-nil error as
// "the pass did nothing" would be wrong in the common case of a single poison
// record beside healthy traffic.
func (r *Relay) RunOnce(ctx context.Context) (Result, error) {
	now := r.clk.Now()

	claimed, err := r.store.Claim(ctx, now, r.batch)
	if err != nil {
		return Result{}, fmt.Errorf("outbox: claim: %w", err)
	}
	// A fast path with no semantic content, and it is flagged as such because a
	// mutation harness cannot tell the difference. Removing it is EQUIVALENT:
	// the code below would produce Result{Claimed: 0} — which is Result{} —
	// clone nil, iterate nothing, settle nothing and join no errors. It is kept
	// because an idle outbox is the steady state of a continuously pumped relay,
	// so this saves a map allocation on the hot-by-frequency path.
	//
	// Expect it to SURVIVE a mutation. That is not a test gap.
	if len(claimed) == 0 {
		return Result{}, nil
	}

	res := Result{Claimed: len(claimed)}

	// Copy before sorting. The store's slice is the store's, and reordering it
	// in place would be a mutation of a value handed across a port — the kind
	// of aliasing bug that only shows up in the implementation that reuses its
	// buffer.
	ordered := slices.Clone(claimed)
	slices.SortFunc(ordered, func(a, b Record) int { return cmp.Compare(a.Seq, b.Seq) })

	var (
		acks    []uint64
		fails   []Failure
		errs    []error
		blocked = make(map[Key]struct{})
	)

	for _, record := range ordered {
		if _, stuck := blocked[record.Key]; stuck {
			// Skipped, not failed: it was never attempted, and charging it an
			// attempt would back it off for somebody else's failure — which
			// matters because ITS backoff is what the whole key then waits on.
			res.Blocked++
			continue
		}
		if cerr := ctx.Err(); cerr != nil {
			// Stop attempting. Everything not yet settled keeps its current
			// RetryAt and simply redelivers, which is the at-least-once
			// contract doing its job during a shutdown.
			errs = append(errs, fmt.Errorf("outbox: pass cut short: %w", cerr))
			break
		}

		perr := record.Validate()
		if perr == nil {
			perr = r.pub.Publish(ctx, record)
		}
		if perr == nil {
			acks = append(acks, record.Seq)
			res.Published++
			continue
		}

		blocked[record.Key] = struct{}{}
		res.Failed++
		fails = append(fails, Failure{
			Seq:     record.Seq,
			RetryAt: now.Add(r.retryDelay(record, perr)),
		})
		errs = append(errs, fmt.Errorf("outbox: %s: %w", record, perr))
	}

	// Settle. Ack first: it is the operation whose omission causes a duplicate
	// (harmless, by contract) while Fail's omission causes a busy reclaim, so
	// the one more likely to be interrupted should be the cheaper mistake.
	if len(acks) > 0 {
		if err := r.store.Ack(ctx, acks); err != nil {
			errs = append(errs, fmt.Errorf("outbox: ack %d records: %w", len(acks), err))
		}
	}
	if len(fails) > 0 {
		if err := r.store.Fail(ctx, fails); err != nil {
			errs = append(errs, fmt.Errorf("outbox: reschedule %d records: %w", len(fails), err))
		}
	}

	return res, errors.Join(errs...)
}

// retryDelay is the schedule's delay, raised to the venue's own hint when the
// venue asked for longer.
//
// The LONGER of the two, in both directions, and each direction is load-bearing.
// A RetryAfter beyond the computed backoff must win, because retrying before a
// venue's stated window is guaranteed to fail and spends rate-limit budget §5.3
// reserves for recovery. A RetryAfter SHORTER than the computed backoff must
// lose, because otherwise a consumer advertising one second resets an hour of
// accumulated escalation on every attempt — the backoff would stop escalating
// precisely against the consumer that keeps failing.
//
// errors.As rather than a chain walker: apperror exposes ClassOf and EffectOf as
// walkers but keeps RetryAfter on the concrete type, and the outermost *Error is
// the right one to read anyway — §8a's wrap rule makes the outermost
// classification the authoritative one.
func (r *Relay) retryDelay(record Record, err error) time.Duration {
	d := r.backoff.Delay(record.Attempts)

	var ae *apperror.Error
	if errors.As(err, &ae) {
		if hint := ae.RetryAfter(); hint > d {
			return hint
		}
	}
	return d
}
