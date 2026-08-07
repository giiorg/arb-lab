package application

import (
	"fmt"
	"sync"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// Executor is §6 step 6's package half: "parallel marketable-IOC limit orders
// (never raw MARKET), identical base quantity, unique client IDs" — the two-leg
// entry and, by §6 step 9, the close, executed as ONE act over legs the Planner
// has already journaled.
//
// # Parallelism is confinement, not synchronisation
//
// Dispatcher is not safe for concurrent use, and its doc places serialisation
// "one layer out". This is that layer: Execute constructs one dispatcher PER
// LEG, each confined to one goroutine, so no dispatcher is ever shared between
// goroutines and the parallel legs never meet on the dispatch path. The one
// piece the legs do share, the DispatchAuthority, is synchronised and
// monotone; see its doc for why that is not the mutex the dispatcher rejects.
//
// The dispatchers are per CALL, not a table kept across calls, and the earlier
// shape is worth recording because it looked safer and held nothing: a slot
// table keyed by minted leg index promised that "the same order meets the same
// admission history" — but the admission history lives in the shared
// authority, which every dispatcher consults, so a fresh dispatcher and a
// remembered one answer identically. A table whose only content is what a
// shared collaborator already holds is a second copy of a fact, and §59a
// records the removal.
//
// # Not safe for concurrent use
//
// §5.3's single writer means there is no legitimate second caller: one
// strategy loop plans and executes. Two concurrent Execute calls of one
// package would also present the same ids concurrently, which is the same-id
// window the authority's check-then-mark deliberately does not close (see
// DispatchAuthority). Serialisation of Execute itself is the composition
// root's, on the same terms the dispatcher states for Dispatch.
type Executor struct {
	journal  ports.Journal
	placer   ports.OrderPlacer
	orders   ports.OrderWriteRepository
	clk      clock.Clock
	auth     *DispatchAuthority
	filters  filterPolicy
	reserver ports.CapacityReserver
}

// NewExecutor constructs the use case FROM the process's dispatcher, and the
// signature is the safety property rather than a convenience.
//
// The first shape took the same five collaborators NewDispatcher takes, with
// the authority among them — and a composition seam that could pass the
// dispatcher one authority and the executor another would fragment the
// double-dispatch backstop exactly as §59b describes, with no test able to
// see it: the fragmentation is observable only through a broken projection,
// which the real repository cannot be made to produce. The battery proved the
// gap (a two-authorities wiring mutant SURVIVED, §59g), and the fix is this
// repo's recorded move for a check nothing can make: close it with the
// signature. An executor built from a dispatcher IS on that dispatcher's
// authority, journal, placer, repository and clock; a second authority in one
// process is no longer expressible from outside this package.
//
// # The filter policy is INHERITED, on exactly the authority's argument
//
// The preflight RE-verifies every leg against §2.4's trading filters even
// though Plan refused nonconforming legs before journaling them — because the
// repository the refs point into is also rebuilt by §7.5's replay, and a
// replayed StatePlanned record was conformed by whatever revision (or
// pre-filters build) planned it, not by this process's. filters.go states the
// argument in full, and NewPlanner's nil-map contract holds here identically:
// a nil map executes nothing, refusing each package on its first leg.
//
// The map and the row-age bound were parameters here until #184, and they
// moved to NewDispatcher for the authority's own reason. The Deleverer is the
// process's SECOND admission-time enforcer of the same policy, and a seam that
// could hand it a different grid — a stale revision, or the empty map that
// refuses every safety leg during an incident — would fragment §2.4's
// enforcement exactly as two authorities fragment the dispatch backstop, with
// no test able to see it. Inherited, a second policy in one process is not
// expressible from outside this package.
func NewExecutor(d *Dispatcher, reserver ports.CapacityReserver) (*Executor, error) {
	if d == nil {
		return nil, ErrDispatcherMissing
	}
	if reserver == nil {
		return nil, ErrReserverMissing
	}
	// d's own constructor refused every nil and validated the policy, so these
	// copies are known good.
	return &Executor{
		journal:  d.journal,
		placer:   d.placer,
		orders:   d.orders,
		clk:      d.clk,
		auth:     d.auth,
		filters:  d.filters,
		reserver: reserver,
	}, nil
}

// UrgencyOpening and UrgencySafety re-export §7.6's two order classes.
//
// They exist because the class is an argument every dispatch caller must
// supply — ports.Urgency records why no default is available — and the callers
// now include composition roots, which §3.6 rule 6 bars from importing
// oms/application/ports by position. A root that cannot NAME a class would be
// pushed toward a string cast, which is exactly the undeclared value the type
// exists to refuse. Re-exporting the constants keeps the policed package
// unimported and the type system intact; the type itself is deliberately not
// aliased, because a root has no business declaring parameters of it.
const (
	UrgencyOpening = ports.UrgencyOpening
	UrgencySafety  = ports.UrgencySafety
)

// DispatchRef names one planned leg: the id, and the caller's assertion about
// WHICH record it decided to execute — Dispatch's (id, version) contract,
// package-wide. There is no field through which a different order could
// arrive, for Dispatch's own reason.
type DispatchRef struct {
	ID      ids.ClientOrderID
	Version int64
}

// LegResult is where one leg ended up.
//
// Exactly one of Outcome and Err is meaningful, on Dispatch's contract: a
// non-nil Err means NOTHING WAS SENT for this leg and the order is still
// dispatchable; a nil Err means Place ran exactly once and Outcome.State is
// where the order is.
type LegResult struct {
	// ID names the leg, because results must be correlated by identity: the
	// slice order of Execution.Legs matches the refs a caller passed, but a
	// reader keying on position is one presentation-order change away from
	// reading leg 0's fate as leg 1's.
	ID ids.ClientOrderID

	Outcome Outcome
	Err     error
}

// Execution is what one package execution produced, one result per leg, in
// the order the refs were presented.
type Execution struct {
	Legs []LegResult

	// Hold is §6 step 7's report failing to reach the checklist: nil always,
	// unless the fan-in found a stale leg and Checklist.Report refused the
	// filing.
	//
	// It is believed unreachable, and the belief is admission's postcondition
	// rather than an assumption: Execute refused the package unless the
	// net-delta gate was DECLARED, and PermitsNow refused it unless every
	// concluded verdict permitted this class — which leaves the gate, at the
	// fan-in, in a state Report admits OutcomeProjectionStale over (unlatched,
	// or already latched at exactly this outcome, which Report accepts as the
	// honest repeat). Checked rather than assumed on the dispatcher loop's
	// precedent one method down: a hold that could not be RECORDED must not
	// vanish, and this field is its only possible home — Execute's error result
	// means nothing was sent, and after the fan-out that is never true again.
	Hold error
}

// HoldsNewTrading reports whether any leg's result requires new trading to be
// held — Outcome.HoldsNewTrading, package-wide.
//
// A leg that was never sent (Err != nil) does not hold: §5.3 makes an unsent
// planned order "safe to replan or discard", and the projection is not stale
// about it. The legs that DID send answer for themselves.
func (e Execution) HoldsNewTrading() bool {
	for _, leg := range e.Legs {
		if leg.Err == nil && leg.Outcome.HoldsNewTrading() {
			return true
		}
	}
	return false
}

// Dispatched counts the legs whose Place ran.
func (e Execution) Dispatched() int {
	n := 0
	for _, leg := range e.Legs {
		if leg.Err == nil {
			n++
		}
	}
	return n
}

// Execute submits every leg of one planned package, in parallel, each exactly
// once, reporting §6 step 7's hold into c when a sent leg's apply fails.
//
// # err != nil means NO LEG WAS ATTEMPTED
//
// The contract is Dispatch's, one level up, and the reason is §6 step 6's
// fsync-side rule applied to the package: every check that can refuse ANY leg
// runs before ANY leg's authorization fsync. A refusal discovered at leg 2
// after leg 1 was authorized would strand a durable authorization — or a sent
// IOC — on a package the caller was told was refused. So the preflight below
// is a read pass over every leg, and only a package that passes whole is
// fanned out at all.
//
// # The checklist is a parameter because the mapping must not be rememberable
//
// Outcome.Applied's doc placed §6 step 7's consequence with "the composition
// root that owns a boot.Checklist" back when nothing constructed one. The
// Reconciler then settled what that means in practice: the root OWNS the
// checklist and the use case that holds the evidence REPORTS into it, handed
// the checklist per call. This is the same settlement for the live path — the
// fan-in below is the one place that knows a send happened and its apply did
// not, so it files GateNetDeltaAsserted → OutcomeProjectionStale itself. A
// root that had to read Execution.HoldsNewTrading and file the report would be
// a root that could forget to, and a forgotten filing is precisely a stale
// projection nothing refuses an opening over — the signature carries the
// mapping so no future root re-derives it.
//
// # The admission read, and why it is PermitsNow and not the boot verdict
//
// Before anything is sent, the package's class must be permitted by the
// checks that have concluded: ActOpen for an Opening package, ActReduce for a
// Safety one (see boot.PermitsNow for why unproduced gates do not constrain
// and why an empty verdict set refuses everything). The composition root's
// own gate — the boot VERDICT, per §59d — runs once per run and stays; this
// read runs per package, which is what closes §6 step 7's loop inside one
// process: the hold this method's fan-in filed refuses the next Opening
// package at this line, while a Safety package still passes, because a
// latched OutcomeProjectionStale is stop-opening and stop-opening is the
// posture whose whole meaning is that the safety verbs stay available.
//
// # After the fan-out there is no composite error
//
// A sent IOC cannot be un-sent, so once dispatch begins the only honest report
// is per leg: Execution carries each leg's Outcome or its refusal, and the
// caller reads each. An aggregate error here would invite
// `if err != nil { return }` over live orders — the exact reflex Dispatch's
// split signature exists to make inexpressible.
//
// # What the preflight does NOT close, stated on admit's precedent
//
// The preflight reads records and the fan-out dispatches them; a write landing
// between the two is caught by each leg's own admit (before anything is sent)
// or by OCC at the apply (after, visibly). The window is the same one admit
// documents for its own read-then-write, and the single-writer premise that
// bounds it there bounds it here.
func (e *Executor) Execute(c *boot.Checklist, refs []DispatchRef, urgency ports.Urgency) (Execution, error) {
	if err := urgency.Validate(); err != nil {
		return Execution{}, fmt.Errorf("execute package urgency: %w", err)
	}
	if len(refs) == 0 {
		return Execution{}, ErrPackageEmpty
	}
	if err := admitPackage(c, urgency); err != nil {
		return Execution{}, err
	}

	// ONE admission instant for the whole package: a two-leg package judged at
	// two instants could straddle the filter-age bound and refuse
	// asymmetrically, which is the package-atomicity harm §62d closed on the
	// conformance side re-opened on the freshness side.
	records, err := e.preflight(refs, e.clk.Now())
	if err != nil {
		return Execution{}, err
	}

	orders := make([]domain.Order, len(records))
	for i, record := range records {
		orders[i] = record.Order
	}
	holds, err := e.reserver.ReservePlacements(orders, urgency)
	if err != nil {
		return Execution{}, err
	}
	// Occupancy never ages out, so cleanup is armed immediately after the
	// successful reserve and before any dispatcher construction or fsync. Pointer
	// identity avoids redundant calls today without assuming future per-venue
	// holds are all the same token; Cancel cannot touch consumed entries.
	distinct := make([]*ratelimit.Hold, 0, len(holds))
	seenHolds := make(map[*ratelimit.Hold]struct{}, len(holds))
	for _, h := range holds {
		if h == nil {
			continue
		}
		if _, seen := seenHolds[h]; seen {
			continue
		}
		seenHolds[h] = struct{}{}
		distinct = append(distinct, h)
	}
	defer func() {
		for _, h := range distinct {
			_ = h.Cancel()
		}
	}()
	if len(holds) != len(refs) {
		return Execution{}, fmt.Errorf("hold vector got %d entries, want %d: %w",
			len(holds), len(refs), ErrHoldVectorInvalid)
	}
	for i, h := range holds {
		if h == nil {
			return Execution{}, fmt.Errorf("hold vector entry %d is nil, got %d entries, want %d non-nil: %w",
				i, len(holds), len(refs), ErrHoldVectorInvalid)
		}
	}

	// The per-leg dispatchers are constructed HERE, serially and before the
	// first fsync, so a dispatcher that cannot be built refuses the package
	// with nothing sent.
	dispatchers := make([]*Dispatcher, len(refs))
	for i := range refs {
		d, err := newDispatcher(e.journal, e.placer, e.orders, e.clk, e.auth, e.filters)
		if err != nil {
			// Unreachable while NewExecutor and newDispatcher refuse the same
			// nils, and refused rather than assumed: a constructor growing a
			// new refusal must not become a half-sent package.
			return Execution{}, fmt.Errorf("execute package leg %d: %w", i, err)
		}
		dispatchers[i] = d
	}

	// One goroutine per leg, each on its own confined dispatcher, each writing
	// only its own index — the fan-in is the WaitGroup, not a channel, so
	// there is no receive a broken send path could starve (the bound on the
	// venue call itself is the transport's, §5.2's Timeout).
	results := make([]LegResult, len(refs))
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref DispatchRef, d *Dispatcher) {
			defer wg.Done()
			outcome, err := d.Dispatch(ref.ID, ref.Version, urgency, holds[i])
			results[i] = LegResult{ID: ref.ID, Outcome: outcome, Err: err}
		}(i, ref, dispatchers[i])
	}
	wg.Wait()

	ex := Execution{Legs: results}
	// §6 step 7's producer: a sent leg whose apply failed holds new trading,
	// and the filing happens HERE — after the fan-in, in the caller's
	// goroutine, because the checklist is not safe for concurrent use and a
	// per-leg report from inside the fan-out would race the other legs'.
	if ex.HoldsNewTrading() {
		if err := c.Report(boot.GateNetDeltaAsserted, boot.OutcomeProjectionStale); err != nil {
			ex.Hold = err
		}
	}
	return ex, nil
}

// admitPackage is the write path's read of the checklist, before any fsync:
// the checklist must exist, must declare the gate the fan-in files §6 step 7's
// hold against, and must permit this package's class through the verdicts so
// far. Everything here is a read, and a refusal means nothing was sent.
func admitPackage(c *boot.Checklist, urgency ports.Urgency) error {
	if c == nil {
		return fmt.Errorf("execute package: %w", ErrChecklistMissing)
	}
	if _, declared := c.Outcome(boot.GateNetDeltaAsserted); !declared {
		return fmt.Errorf("execute package: %w", ErrHoldGateUndeclared)
	}
	if act := actFor(urgency); !c.PermitsNow(act) {
		return fmt.Errorf("execute package: %s needs %s (latched: %v): %w",
			urgency, act, c.Latched(), ErrExecutionHeld)
	}
	return nil
}

// actFor maps §7.6's two order classes onto the acts the posture ladder
// answers for: an Opening order creates exposure and a Safety order reduces or
// bounds it — ports.Urgency's own definitions, restated as the type the
// checklist speaks.
//
// The trailing return is the safe direction rather than a guess: urgency was
// validated before this runs, so a value reaching it is corrupted, and
// ActUnspecified is the act every posture refuses.
func actFor(u ports.Urgency) boot.Act {
	switch u {
	case ports.UrgencyOpening:
		return boot.ActOpen
	case ports.UrgencySafety:
		return boot.ActReduce
	}
	return boot.ActUnspecified
}

// preflight establishes that the whole package may be sent. Everything here is
// a read. now is the package's one admission instant, stamped by Execute.
func (e *Executor) preflight(refs []DispatchRef, now time.Time) ([]ports.OrderRecord, error) {
	seen := make(map[ids.ClientOrderID]struct{}, len(refs))
	recs := make([]ports.OrderRecord, 0, len(refs))
	var trade ids.TradeID
	attempt := -1
	var first *ports.OrderRecord

	for i, ref := range refs {
		// The strict parse, HERE rather than only in admit, because §6 step
		// 6's unique-client-IDs and one-planning-per-package clauses are
		// checked against what the ids themselves encode.
		parsed, err := ids.ParseClientOrderID(string(ref.ID))
		if err != nil {
			return nil, fmt.Errorf("execute package leg %d: %q: %w: %w",
				i, ref.ID, ErrOrderNotOMSMinted, err)
		}
		if _, dup := seen[ref.ID]; dup {
			return nil, fmt.Errorf("execute package leg %d: %q: %w", i, ref.ID, ErrPackageDuplicateLeg)
		}
		seen[ref.ID] = struct{}{}

		if i == 0 {
			trade, attempt = parsed.Trade, parsed.Attempt
		} else {
			if parsed.Trade != trade {
				return nil, fmt.Errorf("execute package leg %d: %q names trade %s, leg 0 names %s: %w",
					i, ref.ID, parsed.Trade, trade, ErrPackageMixedTrades)
			}
			if parsed.Attempt != attempt {
				return nil, fmt.Errorf("execute package leg %d: %q is attempt %d, leg 0 is attempt %d: %w",
					i, ref.ID, parsed.Attempt, attempt, ErrPackageMixedAttempts)
			}
		}
		rec, err := e.orders.Load(ref.ID)
		if err != nil {
			return nil, fmt.Errorf("execute package leg %d: %q: %w", i, ref.ID, err)
		}
		if rec.Version != ref.Version {
			return nil, fmt.Errorf("execute package leg %d: %q is at version %d, execute expected %d: %w",
				i, ref.ID, rec.Version, ref.Version, ports.ErrVersionConflict)
		}
		if err := domain.CanTransition(rec.State, domain.StateDispatchAuthorized); err != nil {
			return nil, fmt.Errorf("execute package leg %d: %q: %w", i, ref.ID, err)
		}
		if rec.Order.Type != domain.OrderTypeLimit || rec.Order.TimeInForce != domain.TimeInForceIOC {
			return nil, fmt.Errorf("execute package leg %d: %q is %s/%s: %w",
				i, ref.ID, rec.Order.Type, rec.Order.TimeInForce, ErrNotMarketableIOCLimit)
		}
		recs = append(recs, rec)

		if first == nil {
			r := rec
			first = &r
			continue
		}
		// §6 step 6's "identical base quantity", against leg 0 — Cmp refuses a
		// mismatched asset, so a package sized in two different bases reports
		// as a mismatch rather than comparing mantissas that happen to agree.
		cmp, err := rec.Order.Quantity.Cmp(first.Order.Quantity)
		if err != nil || cmp != 0 {
			return nil, fmt.Errorf("execute package leg %d: %q quantity %s, leg 0 quantity %s: %w",
				i, ref.ID, rec.Order.Quantity, first.Order.Quantity,
				joinNonNil(ErrPackageQuantityMismatch, err))
		}
	}

	// §2.4's verification half, AGAIN — against THIS process's filters, not
	// whichever revision planned the records. filters.go states why a
	// replayed leg makes the re-check load-bearing, and here it is
	// package-atomic: refused now, no sibling has been fsynced or sent. A
	// SECOND pass, after the package-shape checks above, so a mixed-base
	// package keeps its sharper diagnosis (ErrPackageQuantityMismatch) rather
	// than surfacing as a filters/asset disagreement on whichever leg the
	// first pass reached.
	for i, rec := range recs {
		// Conformance AND its date, at the package's one instant — the pair is
		// filterPolicy's, so the Deleverer's per-leg admission enforces the
		// same two halves and cannot enforce only one.
		if err := e.filters.admits(rec.Order.Instrument, rec.Order.Quantity,
			rec.Order.LimitPrice, now); err != nil {
			return nil, fmt.Errorf("execute package leg %d: %q: %w", i, rec.Order.ClientOrderID, err)
		}
	}
	return recs, nil
}

// joinNonNil wraps base with detail when there is one, so a caller always has
// the sentinel and an asset mismatch keeps its diagnosis.
func joinNonNil(base, detail error) error {
	if detail == nil {
		return base
	}
	return fmt.Errorf("%w: %w", base, detail)
}
