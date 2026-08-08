package application

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// MarginOpResolver is §7.5's in-flight `MarginOp`, resolved by core §7.1's
// evidence contract — "never re-fired blind".
//
// # What it is FOR, in one sentence that is not about margin
//
// A margin top-up is a non-idempotent external write that neither venue gives an
// operation id to (Profiles §4). So after a crash the question "did it apply" has
// no lookup, and what replaces the lookup is a before-state this system wrote
// down and a live reading taken now. This use case is the comparison, plus the
// decision about what an unattributable difference means.
//
// # It ISSUES NOTHING, and the prohibition is normative rather than layering
//
// §7.1: "blind delta retries are prohibited — the dangerous direction is assuming
// a top-up applied when it did not". A resolver that could re-fire would have to
// hold the write path, and the one reading under which re-firing looks safe — the
// op resolved NotApplied, so nothing happened — is exactly the reading a stale
// evidence window produces for an op that DID apply. The ladder above does not
// need it: it re-reads LIVE liq distance, which is indifferent to whether a
// top-up was attempted.
//
// What it does write is the VERDICT, and that is a §5.3 reconciliation act of the
// same class §7.5 already licenses for the DeactivationPending re-drive: it
// records what already happened.
//
// # Not safe for concurrent use
//
// The Reconciler's argument, unchanged.
type MarginOpResolver struct {
	journal ports.MarginOpJournal
	margin  ports.VenueMarginStatus
	acts    ports.VenueMarginActs
	clk     clock.Clock
	bound   time.Duration
}

// NewMarginOpResolver constructs the use case.
//
// # The bound is SM §2 step 1's propagation bound, taken as a parameter
//
// ResolutionPolicy.MaxLanding is the same quantity for orders — how long after we
// stop waiting the venue may still accept the write — and it belongs to the
// TRANSPORT rather than to either surface. Both go through the signed client, so
// a constant here would be a second assumption about one transport, free to drift
// from the configured one. It is passed rather than reached for because this
// package's use cases take their policy from the root.
//
// # The venue clock is NOT checked here, and that is forced rather than an
// # omission
//
// LiqMonitor refuses an uncalibrated venue at construction, because its policy
// table names every venue it will ever be asked about. This one's instruments
// come from the LOG, which does not exist yet at construction time. The refusal
// therefore lands one layer in: an uncalibrated venue yields an unset ServerTime,
// whose zero instant domain.MarginOp.Resolve refuses by name
// (ErrMarginOpResolutionInstantUnset) rather than measuring an op's age against
// it. Late, and it fails as an error — a wiring defect, not a verdict.
// # The act-history port is REQUIRED, and a nil is not "this venue has none"
//
// The tempting wiring is to let a nil acts port mean the delta is the only
// source, which reads as economical and is the one shortcut this design cannot
// afford: it would put the venue-capability fact back into a nil check, where
// the value a forgetful composition root produces is indistinguishable from the
// value Bybit's adapter deliberately returns. domain.MarginActHistory spent a
// three-state enum making that pairing unspellable; accepting a nil here would
// hand it back one layer up. A deployment with no Binance instruments still
// wires an adapter, and that adapter answers NoMarginActHistory — a positive
// statement, made by something with a name.
func NewMarginOpResolver(j ports.MarginOpJournal, m ports.VenueMarginStatus,
	a ports.VenueMarginActs, clk clock.Clock, bound time.Duration) (*MarginOpResolver, error) {

	if j == nil {
		return nil, ErrMarginOpJournalMissing
	}
	if m == nil {
		return nil, ErrVenueMarginStatusMissing
	}
	if a == nil {
		return nil, ErrVenueMarginActsMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if bound <= 0 {
		return nil, fmt.Errorf("margin op resolver: bound %v: %w", bound, domain.ErrMarginOpBoundNotPositive)
	}
	return &MarginOpResolver{journal: j, margin: m, acts: a, clk: clk, bound: bound}, nil
}

// MarginOpResolution is what one op's resolution concluded.
type MarginOpResolution struct {
	// Op is the record the verdict was reached against, carried whole so an
	// operator reading a boot report can see the comparison rather than only its
	// answer.
	Op domain.MarginOp

	// Status is the verdict.
	Status domain.MarginOpStatus

	// LivePosted is the position's posted isolated margin as this resolution read
	// it, and Held is whether the venue reported a position at all.
	//
	// Reported because they are the other half of the comparison and exist
	// nowhere else afterwards: the journal stores the before-state, the venue
	// moves on, and an operator asked to reconcile a float has no way to
	// reconstruct what the resolution actually saw.
	LivePosted fixedpoint.Money
	Held       bool
}

// MarginResolution is what one boot's margin-op phase found.
type MarginResolution struct {
	// Resolved is one entry per in-flight op this phase concluded, in the order
	// Recovery listed them (by id).
	Resolved []MarginOpResolution

	// Ambiguous counts the ops that constrain this process — the ones this phase
	// resolved to Ambiguous PLUS the ones a previous process already did and
	// wrote down.
	//
	// One count over two sources, because the gate does not care which process
	// discovered the incident: §7.5 blocks openings until the ledger is
	// explicitly reconciled, and a restart is not a reconciliation. Carried()
	// below keeps the two separable for the operator report.
	Ambiguous int

	// Carried is the subset of Ambiguous that came from the LOG rather than from
	// this phase's own reads — a §9 incident an earlier process recorded and
	// nothing has cleared.
	Carried []domain.MarginOpID

	// Pending counts the ops too recently dispatched for the venue's silence to
	// be evidence. It is a COUNT and the ops are in Resolved, for
	// Reinstatement.AwaitingQuiesce's reason: the gate must not have to
	// re-derive the condition by walking the list.
	Pending int
}

// Outcome is what this resolution reports to §7.5's checklist.
//
// # The fold is by CONSEQUENCE, not by posture ordering, and the two disagree
//
// An ambiguity is stop-opening (posture 2) and a pending is reconcile-only
// (posture 1), so "the most restrictive wins" would report a boot holding both as
// OutcomeIncomplete. That is the wrong answer twice over.
//
// It loses the LATCH, which is the property that must survive: an ambiguity
// reported as a freely re-reportable outcome is one the next report lifts, and
// the next report is what a live re-resolution produces. And its extra
// restriction is one §7.1 explicitly does not want — reconcile-only forbids
// ActReduce, so the safety ladder would be taken away during the very incident
// the section says safety actions must continue through ("not safety actions,
// which read live state").
//
// A pending alongside an ambiguity argues for neither of those, so the ambiguity
// decides. What the pending costs is one more boot before the gate can pass,
// which the next report supplies.
func (m MarginResolution) Outcome() boot.Outcome {
	switch {
	case m.Ambiguous > 0:
		return boot.OutcomeBlocksOpening
	case m.Pending > 0:
		// The check RAN and did not establish its precondition, with nothing
		// failed and no operator needed — OutcomeIncomplete's own description,
		// and §5.3's stable-not-found rule is the producer it was written for.
		return boot.OutcomeIncomplete
	}
	return boot.OutcomePassed
}

// Resolve concludes every in-flight margin op the replay found, and counts the
// ones a previous process already concluded as ambiguous.
//
// # A failure leaves what it built
//
// Rebuild's rule and Reinstate's. The verdicts written before the failure stay
// written — they are true, and re-deriving them later is the thing the durable
// verdict exists to prevent — and nothing may trade on a boot that reported a
// failure, because the posture enforces that rather than the emptiness of a
// list.
//
// # A venue that will not answer is an ERROR here, unlike everywhere else in
// # §7.1's chain
//
// LiqMonitor turns every venue-side failure into a ladder state, because it runs
// continuously and an error a caller logs and moves past would silently lift a
// block. This runs once, inside a boot, where the checklist has a member for
// exactly this — OutcomeFailed, "a venue that would not answer", halted and
// freely re-reportable — and the caller that maps to it is the Reconciler. The
// two treatments are the same rule applied to two lifetimes: the state must be
// visible to whatever decides next, and here that is the gate rather than a
// returned struct.
func (r *MarginOpResolver) Resolve(rec Recovery) (MarginResolution, error) {
	out := MarginResolution{
		Ambiguous: len(rec.AmbiguousMarginOps),
		Carried:   rec.AmbiguousMarginOps,
	}
	for _, op := range rec.InFlightMarginOps {
		one, err := r.resolveOne(op)
		if err != nil {
			return out, err
		}
		out.Resolved = append(out.Resolved, one)
		switch one.Status {
		case domain.MarginOpAmbiguous:
			out.Ambiguous++
		case domain.MarginOpPending:
			out.Pending++
		case domain.MarginOpApplied, domain.MarginOpNotApplied:
			// Finished business. NotApplied is deliberately not a retry trigger:
			// §7.1 prohibits the blind delta retry, and the ladder above re-reads
			// live liq distance, which is the input that actually decides whether
			// another top-up is wanted.
		case domain.MarginOpUnresolved:
			// Unreachable from Resolve, which either answers a declared member or
			// returns an error. Named rather than defaulted (§3.6 rule 10),
			// because a status added later must land here rather than in whichever
			// arm a fallthrough chose.
			return out, fmt.Errorf("margin op %q: %w", op.ID, domain.ErrMarginOpStatusUnset)
		}
	}
	return out, nil
}

// resolveOne reads the venue and records the verdict.
//
// # The journal write happens BEFORE this returns, and only for a verdict
//
// A concluded status is durable before the caller sees it, so a crash between the
// read and the report leaves the log holding the answer rather than an op that
// must be resolved again from a staler window. `Pending` is not written, because
// it is not an answer — see ports.MarginOpJournal.
func (r *MarginOpResolver) resolveOne(op domain.MarginOp) (MarginOpResolution, error) {
	reading, held, err := r.margin.MarginStatus(op.Instrument)
	if err != nil {
		return MarginOpResolution{}, fmt.Errorf("resolve margin op %q: %w", op.ID, err)
	}

	// The venue's clock, because the watermark is on it. See
	// domain.MarginOp.Resolve — nothing there can check the two agree, so the one
	// call site that chooses is here.
	now := r.clk.ServerNow(op.Instrument.Venue).Instant()

	// The fetch window is the op's own, and it is derived from the SAME two
	// quantities the match uses — the watermark and the propagation bound — so
	// there is no second window to drift from the first. A fetch narrower than the
	// match would starve it silently; a wider one costs nothing, because the match
	// re-applies both edges to every row it is given.
	hist, err := r.acts.MarginActs(op.Instrument, op.Watermark, op.Watermark.Add(r.bound))
	if err != nil {
		return MarginOpResolution{}, fmt.Errorf("resolve margin op %q act history: %w", op.ID, err)
	}

	status, err := op.Resolve(reading.PostedMargin, held, hist, now, r.bound)
	if err != nil {
		return MarginOpResolution{}, fmt.Errorf("resolve margin op %q: %w", op.ID, err)
	}

	concluded, err := status.Concluded()
	if err != nil {
		return MarginOpResolution{}, fmt.Errorf("resolve margin op %q: %w", op.ID, err)
	}
	if concluded {
		if err := r.journal.AppendMarginOpResolved(op.ID, status); err != nil {
			return MarginOpResolution{}, fmt.Errorf("record margin op %q resolution: %w", op.ID, err)
		}
	}
	return MarginOpResolution{
		Op:         op,
		Status:     status,
		LivePosted: reading.PostedMargin,
		Held:       held,
	}, nil
}
