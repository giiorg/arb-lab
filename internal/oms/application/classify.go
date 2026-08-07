package application

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Classifier is §6.7 — "the SINGLE gate for EVERY compensating trade".
//
// # It decides and it cannot act, which is the shape the whole section asks for
//
// §6.7 routes six clients through this gate: the one-leg-failure handler,
// ADL/liquidation (§6 step 8), the kill-switch panic-MARKET (§7.4),
// protection-disarm (§7.4), PackageDelever/EmergencyClosePackage (§5.3), and
// actuator takeover (§7.4). Every one of them wants to TRADE, and the rule is
// that none may compensate the uncertain part of an ambiguous residual. So this
// type holds ports.HedgeGroupOrders — one read method — and no writer, no
// placer and no journal. There is no method here through which a compensating
// trade could be issued, which is Resolver's argument about resubmission applied
// to the other end of the same section.
//
// # The three classification inputs, and why only one of them has a guard here
//
// §6.7 (v0.9.24, after FM-6): "Classification is a function of THREE inputs, and
// re-runs when ANY changes."
//
//  1. Fill and position events move the REALIZED delta.
//  2. Order-lifecycle and finality events — ack, reject, cancel acknowledgement,
//     terminal order status, an executions-sweep/watermark completion, a
//     stable-not-found finality timer, a resolving-query result — collapse an
//     order's possible-delta INTERVAL.
//  3. A change to outstanding_reserved_reductions moves the SUBTRAHEND.
//
// The first two are recomputed by calling Classify on the event, which is the
// ordinary shape and the shape §6.7 had a stated re-run rule for. The third is
// different: its mutators live on THIS type — Reserve, Amend, Release — and each
// returns a fresh verdict, so a ledger change that does not re-classify is not a
// discipline anybody has to keep, it is a call that does not exist.
//
// That asymmetry is deliberate and it is FM-6's lesson rather than an
// inconsistency. §6.7 used to say the classification was a function of TWO
// inputs; the residual it gates on has always been
// `realized_delta − outstanding_reserved_reductions`, so the subtrahend decides
// the verdict exactly as the minuend does — and it moves on events that are not
// order events at all: a generation opening, amending as its perp leg confirms,
// completing, terminal-flattening or being superseded. TLC's signature for the
// two-input reading was a DEAD END, not a violated invariant: every interval
// collapsed, the verdict still DeadlineExceeded, its one permitted action spent,
// a determined naked residual and dispatch budget in hand, and nothing enabled.
// A re-run rule that is written down has a reader; the one that is missing has
// none, so the missing one is the one that gets a type.
//
// # Not safe for concurrent use
//
// Dispatcher's and Resolver's argument, unchanged: §5.1 gives the aggregate an
// OCC version and serialisation is decided one layer out.
type Classifier struct {
	orders ports.HedgeGroupOrders
	groups map[domain.HedgeGroup]*episode
}

// episode is the mutable state §6.7 attaches to one hedge group.
//
// Both flags are per DEADLINE-EXCEEDED EPISODE rather than per group lifetime,
// and they are reset together the moment the classifier reports Determined —
// which is what ends an episode. §6.7 states "never issued twice" flatly but
// states it INSIDE the DeadlineExceededAmbiguous row, and a later, distinct
// incident must plainly be allowed its own floor: a global reading would leave a
// second incident with no admissible safe action at all, for ever. That is a
// READING, and it is written down because an unstated reading is what later gets
// called drift. Formal model (a) implements the same one.
type episode struct {
	// asset is the group's base asset, fixed when the group is tracked. It is
	// what lets an EMPTY group answer "Determined, residual zero of this asset"
	// rather than failing on fixedpoint's untagged zero.
	asset fixedpoint.Asset

	// reserved is §6.7's outstanding_reserved_reductions for this group — the
	// subtrahend, and the ledger FM-5, FM-8 and FM-9 are all about.
	reserved *domain.ReservedReductions

	// deadlineExceeded records that the finality contract could not complete
	// within its deadline. It is set by an EVENT rather than by a timer here:
	// the deadline belongs to the quiesce attempt, which Resolver runs, and §9's
	// <30 s quiesce RTO is a chaos-gate number rather than a property of this
	// type. Formal model (a) makes the same abstraction — DeadlineBlows is an
	// environment event and there is no real time in the model.
	deadlineExceeded bool

	// floorIssued is FM-7's rule: the IntervalFloorReduction is "never issued
	// twice".
	//
	// It is DEFENCE IN DEPTH, deliberately retained, and its justification is
	// stated as such rather than as a live hazard. Under reservation-aware
	// sizing a second floor CANNOT reach the opposite sign, because the first is
	// already netted out of the subtrahend — formally checked over the complete
	// state graph (model (a) NC20, 1,086,199 distinct states, no violation). It
	// is kept because the proof it protects rests on TWO independent mechanisms
	// and this is the one that survives a defect in the other: if the
	// reservation is ever not counted — the FM-5 failure mode, an entry not
	// backed at the instant it is read — a second uncertain reduction DOES reach
	// the opposite sign.
	//
	// The figure is 1,093,953 distinct states, 0 left on queue, and it is the
	// CURRENT suite's rather than the one SM §2 carried: 1,086,199 was slice E's
	// measurement on the pre-FM-8 model, and it had been copied into six
	// documents by the time this slice went to copy it into the code. A figure
	// no run reproduces reads exactly like one that does.
	floorIssued bool
}

// NewClassifier returns a classifier over one order source.
func NewClassifier(orders ports.HedgeGroupOrders) (*Classifier, error) {
	if orders == nil {
		return nil, ErrHedgeGroupOrdersMissing
	}
	return &Classifier{
		orders: orders,
		groups: make(map[domain.HedgeGroup]*episode),
	}, nil
}

// Track makes a hedge group known to the classifier, fixing its base asset.
//
// # Why a group must be tracked before it can be classified
//
// The base asset cannot be derived when it is most needed. A group with no
// orders — before the first plan, and after §6 step 9's close — has no quantity
// to read an asset off, and its correct verdict is "Determined, residual zero",
// which cannot be expressed without one: fixedpoint's zero value carries no tag
// and fails its first operation. Deriving the asset from the first order that
// happens to exist would make the answer for an empty group depend on history
// the classifier does not keep.
//
// # This is NOT §5.1's hedge-group lease, and the distinction is worth keeping
//
// §5.1 gives the OMS "an exclusive lease per hedge-group (account, venue,
// symbol) … for the life of a trade; a second Intent on a leased key is
// rejected". That is a stronger thing than this: it arbitrates INTENTS, it has a
// lifetime tied to a trade, and §6 step 9 makes its release conditional on a
// persisted Deactivate acknowledgement. None of that is here, and calling this
// method the lease would be claiming a rule this slice does not implement. It
// lands with §5.1's Trade aggregate.
func (c *Classifier) Track(group domain.HedgeGroup, base fixedpoint.Asset) error {
	if err := group.Validate(); err != nil {
		return fmt.Errorf("classifier: %w", err)
	}
	// The empty asset is checked here rather than left to the first arithmetic.
	// fixedpoint.Asset is a bare string with no Validate — the set of venue
	// assets is discovered at run time, so there is nothing to check membership
	// against — and an empty one produces a Quantity that fails every operation.
	// Caught at Track, the refusal names the group; caught later, it surfaces
	// from inside a fold as a quantity error on whichever order was visited
	// first.
	if base == "" {
		return fmt.Errorf("classifier: group %s: %w", group, ErrHedgeGroupAssetUnset)
	}
	if _, ok := c.groups[group]; ok {
		return fmt.Errorf("classifier: group %s: %w", group, ErrHedgeGroupAlreadyTracked)
	}
	c.groups[group] = &episode{
		asset:    base,
		reserved: domain.NewReservedReductions(base),
	}
	return nil
}

// Verdict is one classification: §6.7's answer for one hedge group at one
// instant.
//
// It is a value rather than a handle. §6.7 requires the classification to be
// recomputed on every event that can move any of its three inputs, so a verdict
// held across an event is stale by construction — and a stale verdict is exactly
// the failure §6 step 7 describes at the projection layer, where a residual read
// from the wrong instant turns an ambiguous group into a determined one.
type Verdict struct {
	// Group is what was classified. It is carried so a verdict passed to an
	// alerting or journaling path names its own subject.
	Group domain.HedgeGroup

	// Class is the §6.7 verdict.
	Class domain.Classification

	// Residual is `realized_delta − outstanding_reserved_reductions`: the
	// quantity §6.7's determined branch sizes its ExposureReduction from.
	//
	// It is SIGNED in the group's base asset — positive is net long, negative is
	// net short — and it is meaningful for every verdict, not only Determined:
	// under Ambiguous it is the point the possible-delta range is centred on,
	// which is what makes Range's endpoints comparable across events.
	Residual fixedpoint.Quantity

	// Reserved is the subtrahend that produced Residual.
	//
	// It is carried because the residual alone cannot be explained: a group
	// reading flat is either genuinely flat or exactly offset by an in-progress
	// reserved reduction, and those are different situations for an operator
	// looking at an alert. It is also the quantity FM-5 corrupts, so a verdict
	// that reported only the difference would hide the input the failure is in.
	Reserved fixedpoint.Quantity

	// Range is Residual shifted by the summed possible-delta intervals of every
	// order that still contributes one — §6.7's "the group's net exposure lies
	// in a summed interval". It collapses to a point exactly when Class is
	// Determined.
	Range domain.PossibleDelta
}

// Reduction is the signed exposure change §6.7's determined branch must apply:
// "issue an ExposureReduction sized from realized_delta −
// outstanding_reserved_reductions".
//
// It is the NEGATION of the residual, because the residual is what the group
// holds and the reduction is what removes it. A net-long group yields a negative
// change (sell), a net-short group a positive one (buy).
//
// It refuses on any verdict but Determined, and the refusal is §6.7's central
// rule rather than defensiveness: "no path compensates the uncertain part of an
// ambiguous residual before the finality barrier". The deadline-exceeded branch
// is refused here too even though it permits the interval floor — that action
// has its own method, because it is sized from the RANGE and not from the
// residual, and a single entry point would let a caller size an ordinary
// reduction from a group whose exposure is not yet fixed.
func (v Verdict) Reduction() (fixedpoint.Quantity, error) {
	permitted, err := v.Class.PermitsCompensation()
	if err != nil {
		return fixedpoint.Quantity{}, err
	}
	if !permitted {
		return fixedpoint.Quantity{}, fmt.Errorf("group %s is %s: %w",
			v.Group, v.Class, ErrCompensationBeforeFinality)
	}
	return v.Residual.Neg()
}

// Classify recomputes the verdict for a tracked group.
//
// This is the call for classification inputs 1 and 2 — a fill or position event,
// and any hedge-group order-lifecycle or finality event. §6.7 is explicit that
// it must not wait for one when the other alone became conclusive: "a cancel
// confirmed with ZERO executions emits no fill and no position event, yet fully
// collapses that order's interval".
//
// Input 3 has its own methods, which call this. See the type doc.
func (c *Classifier) Classify(group domain.HedgeGroup) (Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return Verdict{}, err
	}
	return c.evaluate(group, ep)
}

// DeadlineExceeded records that the finality contract could not complete within
// its deadline for this group — §6.7's "queries unavailable/rate-limited" branch.
//
// The verdict it returns is DeadlineExceededAmbiguous ONLY if the group is still
// ambiguous. A group whose intervals have collapsed is Determined and stays
// Determined, because §6.7's row for the late resolution says so — "intervals
// later collapse … → Determined; the determined branch then applies" — and the
// opposite precedence is model (a)'s NC14, whose signature is a machine stranded
// with a frozen verdict, every interval collapsed and dispatch budget in hand.
//
// What the caller must do with the result is §6.7's, not this type's: hold new
// trading, alert the operator, and permit only provably-safe action.
func (c *Classifier) DeadlineExceeded(group domain.HedgeGroup) (Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return Verdict{}, err
	}
	ep.deadlineExceeded = true
	return c.evaluate(group, ep)
}

// Reserve creates a reserved-reduction entry for this group — classification
// input 3 — and returns the verdict that results.
//
// The entry is created at ZERO. See domain.ReservedReductions: the absence of an
// amount parameter is FM-5's fix, and the constructor here forwards that absence
// rather than papering over it with a convenience argument.
func (c *Classifier) Reserve(group domain.HedgeGroup, id domain.ReservationID, producer domain.ReductionProducer) (Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return Verdict{}, err
	}
	if err := ep.reserved.Reserve(id, producer); err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	return c.evaluate(group, ep)
}

// Amend sets an entry's outstanding net intent and returns the resulting
// verdict.
//
// The returned verdict is the whole reason this method exists on the classifier
// rather than on the ledger: a producer amending its entry has moved the
// subtrahend, and §6.7 requires the classification to re-run. Handing the ledger
// out and trusting producers to re-classify is the two-input world FM-6 came
// out of.
func (c *Classifier) Amend(group domain.HedgeGroup, id domain.ReservationID, outstanding fixedpoint.Quantity) (Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return Verdict{}, err
	}
	if err := ep.reserved.Amend(id, outstanding); err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	return c.evaluate(group, ep)
}

// Release removes an entry, reporting the amount that stopped being subtracted
// and the resulting verdict.
//
// # The returned quantity is the caller's obligation, not a convenience
//
// §6.7 row 4: a reduction whose OWN result comes back ambiguous "never blind
// retries, and its own ambiguous result routes through this same §6.7 finality
// contract". Releasing its entry does not make the exposure it was covering
// disappear — that exposure re-enters as an unresolved ORDER contributing a
// possible-delta interval, and a caller that discarded this quantity would drop
// it from the group's arithmetic entirely.
//
// This is also half of FM-8, and the half nothing here can close: what a
// released entry can leave behind is a PackageDelever generation still running
// on a premise that has stopped holding — "an entry's premise outliving it". The
// generation is SM §7's ladder and does not exist yet; when it does, it is what
// must be told, and the seam it needs is domain.ReservedReductions.OutstandingFor.
func (c *Classifier) Release(group domain.HedgeGroup, id domain.ReservationID) (fixedpoint.Quantity, Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return fixedpoint.Quantity{}, Verdict{}, err
	}
	released, err := ep.reserved.Release(id)
	if err != nil {
		return fixedpoint.Quantity{}, Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	verdict, err := c.evaluate(group, ep)
	if err != nil {
		return fixedpoint.Quantity{}, Verdict{}, err
	}
	return released, verdict, nil
}

// PremiseFor is SM §7's entry test re-evaluated NET OF ONE GENERATION'S OWN
// LEGS — the quantity FM-9 says the standing trigger must read, and the reason
// domain.ReservedReductions.OutstandingFor exists.
//
// # What "net of its own legs" removes, and why it must be both sides
//
// Model (a): `PremiseResidual == (realized − PdOwnRealized) − (TrueReserved −
// PdReserved)`. The generation's own orders come out of the minuend and its own
// ledger entry comes out of the subtrahend, and the result is INVARIANT under
// every step of the generation's own ladder — a perp fill raises the realized
// delta and amends the entry by the same amount, an authorization moves the
// intent and the transient together. That invariance is FM-5's fix restated as a
// quantity, and it is the whole claim: the premise moves only when something
// EXTERNAL moves it, which is what "the premise dissolved" means.
//
// Read RAW instead — "the §2 classifier leaves Determined for this hedge group",
// which is what SM §7 said until v0.9.25 — and the trigger fires on the
// generation's own first step, because its perp reduction is an exposure-changing
// order on that group and §7 DEPENDS on it making the group ambiguous. Model
// (a)'s control NC22 reaches that abort four states from Init with nothing
// realized, nothing outstanding and no external event at all. A spurious abort is
// not bookkeeping: §7's generations exist because the perp short is nearing
// liquidation, so cancelling one cancels the mechanism that was reducing the
// risk, and does it with a reserved leg out.
//
// # The one-generation precondition is CHECKED here rather than assumed
//
// The only handle the ledger offers on "this generation's own entry" is the
// PRODUCER, so the netting is correct exactly while one PackageDelever entry
// exists. SM §7 forbids a second — "each generation is a bounded attempt made
// only after the prior generation reaches §6.7 finality" — and a rule stated in
// another section and relied on here is a premise nobody re-derives. With two,
// this would net a SIBLING's entry out too, and the premise a generation is
// judged against would contain another generation's subject: FM-9 arriving one
// layer out, through the decomposition rather than through the trigger.
//
// At `Planned` the leg set is empty and the entry is zero, so the netting is
// vacuous and this returns the package's ordinary residual and verdict — which
// is what makes the entry precondition and the standing trigger the same test.
func (c *Classifier) PremiseFor(group domain.HedgeGroup, own []ids.ClientOrderID) (domain.Premise, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return domain.Premise{}, err
	}
	generations, err := ep.reserved.CountFor(domain.ProducerPackageDeleverSpot)
	if err != nil {
		return domain.Premise{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	if generations > 1 {
		return domain.Premise{}, fmt.Errorf("classifier: group %s holds %d PackageDelever reservations: %w",
			group, generations, ErrPremiseNettingAmbiguous)
	}

	exclude := make(map[ids.ClientOrderID]struct{}, len(own))
	for _, id := range own {
		if id == "" {
			return domain.Premise{}, fmt.Errorf("classifier: group %s: %w", group, ErrPremiseLegUnidentified)
		}
		exclude[id] = struct{}{}
	}

	verdict, err := c.evaluateExcluding(group, ep, exclude, true)
	if err != nil {
		return domain.Premise{}, err
	}
	return domain.Premise{Residual: verdict.Residual, Class: verdict.Class}, nil
}

// FloorAuthorization is a permitted IntervalFloorReduction: the one compensating
// action §6.7 admits before finality.
type FloorAuthorization struct {
	// Reservation is the entry this generation already holds in the group's
	// reserved-reduction ledger, amended to Change. It exists BEFORE the order
	// is submitted, which is §6.7's requirement rather than an ordering choice:
	// "before submission it reserves its own generation, which subsequent
	// net-delta recomputation counts in outstanding_reserved_reductions — so a
	// still-in-flight interval-floor order is not re-read as fresh naked
	// exposure".
	Reservation domain.ReservationID

	// Change is the signed exposure change to apply — the negation of the
	// interval endpoint nearest zero.
	Change fixedpoint.Quantity
}

// IssueFloor authorizes the interval-floor generation for a deadline-exceeded
// group, reserving it before returning.
//
// It is "the SOLE exception to 'no compensation before finality,' and only for
// the reachable-state-INVARIANT floor, never the uncertain part", and it applies
// ONLY in the deadline-exceeded branch — normal operation waits for finality and
// compensates the exact residual.
//
// Four refusals, and each is a separate rule rather than a defensive layer:
//
//   - not DeadlineExceededAmbiguous — the branch restriction above.
//   - the floor was already issued this episode — FM-7's defence in depth.
//   - the range straddles zero — "no nonzero compensation is safe in all states
//     → de-minimis only". It surfaces as domain's ErrIntervalStraddlesZero.
//   - the nearest-zero endpoint is zero — there is a safe action and it is to do
//     nothing, which must not consume the episode's one floor.
//
// # The reservation is amended here, in the same call
//
// The ledger's rule is that an entry is created at zero and becomes nonzero only
// when its producer has COMMITTED (domain.ReservedReductions). For this producer
// the commitment happens exactly here: authorizing the floor is the decision to
// reduce by this magnitude, and §6.7 requires the reservation to be counted
// before the order is submitted. So reserve-then-amend is one atomic step from a
// caller's side, and the caller's remaining obligation is to submit — or, if it
// does not, to Release, since an entry left behind subtracts from every later
// sizing.
func (c *Classifier) IssueFloor(group domain.HedgeGroup, id domain.ReservationID) (FloorAuthorization, Verdict, error) {
	ep, err := c.episodeFor(group)
	if err != nil {
		return FloorAuthorization{}, Verdict{}, err
	}
	current, err := c.evaluate(group, ep)
	if err != nil {
		return FloorAuthorization{}, Verdict{}, err
	}
	if current.Class != domain.ClassificationDeadlineExceededAmbiguous {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s is %s: %w",
			group, current.Class, ErrFloorOutsideDeadlineBranch)
	}
	if ep.floorIssued {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s: %w", group, ErrFloorAlreadyIssued)
	}

	change, err := current.Range.FloorReduction()
	if err != nil {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s: %w", group, err)
	}
	sign, err := change.Sign()
	if err != nil {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s floor magnitude: %w", group, err)
	}
	if sign == 0 {
		// The nearest-zero endpoint is zero: the group may already be flat under
		// some resolution, so no nonzero compensation is invariant. Refused
		// rather than returned as a zero-sized authorization, because a zero
		// change that consumed the episode's one floor would spend the safe
		// action on nothing — model (a)'s W5 is the same finding on a probe: the
		// shortest counterexample is the empty one.
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s over %s: %w",
			group, current.Range, ErrFloorMagnitudeZero)
	}

	// The ledger carries the EXPOSURE this reduction removes, which is the
	// negation of the change the order applies — see ReservedReductions.Amend,
	// where the two readings and their sign are set out. Amending `change`
	// itself is the defect that reading exists to name: the residual would then
	// grow by the floor's own size, and the determined branch that fires when
	// the intervals collapse would sell the floor's quantity a second time.
	claimed, err := change.Neg()
	if err != nil {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("group %s floor exposure: %w", group, err)
	}
	if err := ep.reserved.Reserve(id, domain.ProducerIntervalFloor); err != nil {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	if err := ep.reserved.Amend(id, claimed); err != nil {
		return FloorAuthorization{}, Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	ep.floorIssued = true

	after, err := c.evaluate(group, ep)
	if err != nil {
		return FloorAuthorization{}, Verdict{}, err
	}
	return FloorAuthorization{Reservation: id, Change: change}, after, nil
}

func (c *Classifier) episodeFor(group domain.HedgeGroup) (*episode, error) {
	if err := group.Validate(); err != nil {
		return nil, fmt.Errorf("classifier: %w", err)
	}
	ep, ok := c.groups[group]
	if !ok {
		return nil, fmt.Errorf("classifier: group %s: %w", group, ErrHedgeGroupNotTracked)
	}
	return ep, nil
}

// evaluate is the classification itself: the fold over the group's orders, the
// subtraction, and §6.7's verdict rule.
func (c *Classifier) evaluate(group domain.HedgeGroup, ep *episode) (Verdict, error) {
	return c.evaluateExcluding(group, ep, nil, false)
}

// evaluateExcluding is the classification with a named set of orders and,
// optionally, the PackageDelever producer's own ledger entry held OUT of both
// sides of §6.7's subtraction.
//
// The ordinary verdict passes an empty set and false, so there is exactly one
// fold and one subtraction in this package. That matters more than the code it
// saves: a second copy of the arithmetic is where the premise and the verdict
// would drift, and the whole of FM-9 is that the two must be the SAME test
// asked about different subjects.
func (c *Classifier) evaluateExcluding(
	group domain.HedgeGroup,
	ep *episode,
	exclude map[ids.ClientOrderID]struct{},
	netOwnReservation bool,
) (Verdict, error) {
	records, err := c.orders.OrdersIn(group)
	if err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}

	realized := fixedpoint.ZeroQuantity(ep.asset)
	movement := domain.NoPossibleDelta(ep.asset)
	for _, rec := range records {
		// Excluding a leg removes BOTH its realized quantity and its
		// possible-delta interval, which is exactly model (a)'s
		// `realized − PdOwnRealized` and `ReportedDetermined(orders, 0)` in one
		// operation. Doing it here rather than by subtracting a separately
		// tracked figure is what keeps the two agreeing: a generation's own
		// tally of what its legs did and the projection's tally are different
		// stores, and the projection is the one §7.5 rebuilds.
		if _, skip := exclude[rec.Order.ClientOrderID]; skip {
			continue
		}
		// The membership check is deliberately a SECOND one — the adapter that
		// answered already filtered — and the reason it is not the "second copy
		// of one rule" shape this repo has paid for is that the two checks are on
		// opposite sides of an INTERFACE. OrdersIn is a port with implementations
		// this package does not own, and this is the only place where both the
		// group key and the records are in hand. It also fails in the safe
		// direction: computing a residual over the wrong set is the one error
		// here that produces a confident answer rather than a refusal.
		if !group.Holds(rec.Order.Instrument) {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q is on %s: %w",
				group, rec.Order.ClientOrderID, rec.Order.Instrument.Symbol,
				domain.ErrOrderOutsideHedgeGroup)
		}

		executed, err := signedQuantity(rec.Order.Side, rec.Cumulative)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q executed: %w",
				group, rec.Order.ClientOrderID, err)
		}
		realized, err = realized.Add(executed)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q: %w",
				group, rec.Order.ClientOrderID, err)
		}

		contributes, err := rec.State.ContributesPossibleDelta()
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q: %w",
				group, rec.Order.ClientOrderID, err)
		}
		if !contributes {
			continue
		}
		remaining, err := rec.Order.Quantity.Sub(rec.Cumulative)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q remaining: %w",
				group, rec.Order.ClientOrderID, err)
		}
		interval, err := domain.PossibleDeltaFor(rec.Order.Side, remaining)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q: %w",
				group, rec.Order.ClientOrderID, err)
		}
		movement, err = movement.Add(interval)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: order %q: %w",
				group, rec.Order.ClientOrderID, err)
		}
	}

	reserved, err := ep.reserved.Outstanding()
	if err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	if netOwnReservation {
		own, err := ep.reserved.OutstandingFor(domain.ProducerPackageDeleverSpot)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
		}
		reserved, err = reserved.Sub(own)
		if err != nil {
			return Verdict{}, fmt.Errorf("classifier: group %s premise subtrahend: %w", group, err)
		}
	}
	residual, err := realized.Sub(reserved)
	if err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s residual: %w", group, err)
	}
	rng, err := movement.Shift(residual)
	if err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}
	class, err := domain.Classify(rng, ep.deadlineExceeded)
	if err != nil {
		return Verdict{}, fmt.Errorf("classifier: group %s: %w", group, err)
	}

	// Reporting Determined ENDS the deadline-exceeded episode, and both flags
	// reset together. §6.7's late-resolution row is what licenses the first
	// half; the second half is the per-episode reading of "never issued twice",
	// stated on the episode type.
	if class == domain.ClassificationDetermined {
		ep.deadlineExceeded = false
		ep.floorIssued = false
	}

	return Verdict{
		Group:    group,
		Class:    class,
		Residual: residual,
		Reserved: reserved,
		Range:    rng,
	}, nil
}

// signedQuantity applies an order's direction to an unsigned quantity.
//
// Direction lives on the order and nowhere else — domain.Fill carries an
// unsigned quantity precisely so the sign has one home — so this is where a
// hedge group's two legs become numbers that can be added. A buy raises the
// group's delta and a sell lowers it, and getting that backwards nets the two
// legs of a cash-and-carry into DOUBLE the exposure instead of zero, which is a
// number no invariant over the result could distinguish from a real position.
func signedQuantity(side domain.Side, q fixedpoint.Quantity) (fixedpoint.Quantity, error) {
	if err := side.Validate(); err != nil {
		return fixedpoint.Quantity{}, err
	}
	switch side {
	case domain.SideBuy:
		return q, nil
	case domain.SideSell:
		return q.Neg()
	}
	return fixedpoint.Quantity{}, fmt.Errorf("%q: %w", string(side), domain.ErrSideUnhandled)
}
