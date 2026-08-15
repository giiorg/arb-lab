package application

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Joiner is the second half of build-spec §7.5's reconcile-only boot: the
// replayed journal "joined to venue truth", and the net-delta invariant asserted
// before trading resumes.
//
// # What the replay leaves it, and why the gap is the dangerous kind
//
// Replayer rebuilds STATE and cannot rebuild quantity: no fill is journaled, so
// every replayed record carries a zero cumulative. That is not a missing number
// with an obvious shape — it is a number whose absence reads as a conclusion.
// §6.7's classifier folds those cumulatives into a residual, and a group of
// zeroes is Determined and flat, which is the verdict that PERMITS a compensating
// trade. A process that booted, replayed, and started classifying would size its
// reductions against a position it cannot see.
//
// So this use case exists to turn the venue's own record into the projection's,
// and the boot checklist is what holds the interval: GateNetDeltaAsserted is a
// separate gate from GateJournalReplayed, the posture is the weakest over the
// required set, and nothing may trade until this has run.
//
// # It joins THREE venue surfaces, and the third is what makes the join a check
//
// §7.5 names OrderByClientID, Fills, Positions and FundingLedger. The first two
// answer "what did OUR orders do" and the third answers "what does the ACCOUNT
// hold", and only the pair of them can disagree. An assertion built from our own
// orders alone would be the shape this repo has paid for twice — a checker
// recomputing the thing it checks — and it would agree with itself over an
// account holding a position no order of ours explains, which is the exact
// condition §7.5 alerts on.
//
// FundingLedger is deliberately not here. It settles in the quote asset on both
// venues' linear perps, so it moves no position and can explain no delta, and a
// port whose consumer does not exist yet is a shape frozen against nothing —
// ports.OrderResolver's own argument. It lands with §8's funding accrual.
//
// # Not safe for concurrent use
//
// Dispatcher's and Resolver's argument, unchanged.
type Joiner struct {
	venue     ports.OrderResolver
	positions ports.VenuePositions
	orders    ports.OrderWriteRepository

	// journal is the durable half of §6's journal-then-apply. It arrived with
	// #242: until then this use case only ADMITTED FILLS, and ApplyFill has no
	// journal counterpart at all (ports.Journal is "narrowed to what the dispatch
	// path writes"). Writing a STATE is the first thing the join does that §7.5
	// replays, so it is the first thing that needs the log.
	journal ports.Journal

	clk     clock.Clock
	account string
	scopes  []ports.PositionScope
	spot    SpotLeg
}

// SpotLeg is the spot half of §7.5's assertion: the configured spot roster,
// the recorded baselines, and the balance surface to compare them against
// (task #179, record §70).
//
// # The roster comes from CONFIGURATION, never from the journal
//
// Assert's instrument set is otherwise derived from dispatched orders, and a
// virgin deployment — configured spot instruments, empty journal, no baseline
// — would have empty sets everywhere and PASS: §7.5's "satisfied on a boot
// that issued no position query at all", arriving through the spot leg. The
// roster is what makes every configured spot instrument answered for, orders
// or none.
//
// # Baselines arrive VERIFIED, as data
//
// The chain mark, credential fingerprint, config digest and per-order row
// recomputation are the loader's (the composition root holds the journal and
// the transport; this layer must not). What crosses this seam is the
// conclusion of that verification: per (venue, asset), the recomputed
// baseline quantity.
type SpotLeg struct {
	// Balances is the venue's spot balance surface. Required exactly when
	// Baselines is non-empty — an assertion cannot compare against a wallet it
	// cannot read — and unused otherwise.
	Balances ports.SpotBalances

	// Roster is every configured spot instrument with its base asset.
	Roster []SpotInstrument

	// Baselines is the recorded not-ours quantity per (venue, base asset).
	Baselines []SpotBaseline
}

// SpotInstrument is one configured spot instrument and the asset decomposition
// its balance assertion is keyed and guarded by.
//
// The base is carried rather than derived because the roster's whole purpose
// is instruments with NO orders — there is no cumulative to read an asset tag
// from — and the config's asset decomposition is the validated authority. The
// QUOTE is carried for one rule: a baselined asset must not be another spot
// pair's quote (validateSpotLeg), because `ours` counts base flows and
// same-asset fees, so a pair spending the baselined asset as consideration
// would move the wallet invisibly and latch a false delta.
type SpotInstrument struct {
	Instrument ids.VenueInstrument
	Base       fixedpoint.Asset
	Quote      fixedpoint.Asset
}

// SpotBaseline is one recorded baseline: the portion of the venue's balance of
// Asset that is NOT attributable to this system's journaled orders.
type SpotBaseline struct {
	Venue  ids.Venue
	Asset  fixedpoint.Asset
	Amount fixedpoint.Quantity
}

// NewJoiner constructs the use case, refusing a missing port.
//
// # account is §5.1's first dedupe-key component, and this is its producer
//
// domain.Fill's Account had a consumer and no producer at all until here: §5.1
// puts it first in the fill dedupe key and first in the hedge-group key, and
// nothing in the system minted one. A venue execution does not carry it — the
// venue knows which subaccount it answered for and does not repeat it per row —
// so it is deployment configuration, exactly as §5.2 says (`subaccount id`), and
// this is the layer that knows it.
//
// Empty is refused rather than defaulted, on the write repository's argument for
// the same string: an empty account merges every subaccount into one dedupe
// identity, so two subaccounts sharing a venue's per-instrument execution-ID
// sequence would silently drop each other's fills as duplicates.
//
// # scopes are the position enumerations to assert over, and they are the
// # deployment's rather than derived
//
// A scope names a venue, a derivative family and a settlement asset. It is not
// derived from the instruments this system trades, because the derivation would
// be exactly backwards: the enumeration exists to find exposure on instruments
// NOTHING here knows about, and a scope computed from what we know cannot widen
// past it. An empty list is legal and means "assert nothing", which the report
// says out loud — see Join.Unasserted.
func NewJoiner(v ports.OrderResolver, p ports.VenuePositions, orders ports.OrderWriteRepository,
	jr ports.Journal, clk clock.Clock, account string, spot SpotLeg, scopes ...ports.PositionScope) (*Joiner, error) {
	if v == nil {
		return nil, ErrResolverMissing
	}
	if p == nil {
		return nil, ErrVenuePositionsMissing
	}
	if orders == nil {
		return nil, ErrOrdersMissing
	}
	if jr == nil {
		return nil, ErrJournalMissing
	}
	if clk == nil {
		return nil, ErrClockMissing
	}
	if account == "" {
		return nil, ErrJoinAccountUnset
	}
	for _, s := range scopes {
		if err := s.Validate(); err != nil {
			return nil, err
		}
	}
	if err := validateSpotLeg(spot); err != nil {
		return nil, err
	}
	return &Joiner{
		venue:     v,
		positions: p,
		orders:    orders,
		journal:   jr,
		clk:       clk,
		account:   account,
		scopes:    append([]ports.PositionScope(nil), scopes...),
		spot: SpotLeg{
			Balances:  spot.Balances,
			Roster:    append([]SpotInstrument(nil), spot.Roster...),
			Baselines: append([]SpotBaseline(nil), spot.Baselines...),
		},
	}, nil
}

// validateSpotLeg refuses a spot configuration whose zero values would read as
// conclusions.
//
// The one cross-field rule is the balances port: required exactly when a
// baseline exists, because a baseline without a readable wallet is an
// assertion that can never run — refused at construction, where the wiring is
// the thing being described, rather than at assert time, where it would be
// diagnosed as a venue problem.
//
// A baseline no roster base consumes is refused HERE, which is where §70b's
// "refused at load" lands in code: the loader hands rows to this constructor
// in the same call that verified them, so this seam is the one every consumer
// passes through — including the tests that never build a document.
func validateSpotLeg(spot SpotLeg) error {
	bases := make(map[spotAssetKey]struct{}, len(spot.Roster))
	seen := make(map[ids.VenueInstrument]struct{}, len(spot.Roster))
	for _, r := range spot.Roster {
		if err := r.Instrument.Validate(); err != nil {
			return fmt.Errorf("spot roster instrument: %w", err)
		}
		if r.Instrument.Product != ids.ProductSpot {
			// Spelled against the member rather than !IsPerp(): a product family
			// added next year is not spot by default, it is a refusal by default.
			return fmt.Errorf("spot roster holds %s: %w", r.Instrument, ErrSpotRosterNotSpot)
		}
		if r.Base == "" {
			return fmt.Errorf("spot roster %s: %w", r.Instrument, ErrSpotRosterBaseUnset)
		}
		if r.Quote == "" {
			return fmt.Errorf("spot roster %s: %w", r.Instrument, ErrSpotRosterQuoteUnset)
		}
		if _, dup := seen[r.Instrument]; dup {
			return fmt.Errorf("spot roster %s: %w", r.Instrument, ErrSpotRosterDuplicate)
		}
		seen[r.Instrument] = struct{}{}
		bases[spotAssetKey{r.Instrument.Venue, r.Base}] = struct{}{}
	}

	baselines := make(map[spotAssetKey]struct{}, len(spot.Baselines))
	for _, b := range spot.Baselines {
		if err := b.Venue.Validate(); err != nil {
			return fmt.Errorf("spot baseline venue: %w", err)
		}
		if b.Asset == "" {
			return fmt.Errorf("spot baseline on %s: %w", b.Venue, ErrSpotBaselineAssetUnset)
		}
		if err := b.Amount.Validate(); err != nil {
			return fmt.Errorf("spot baseline %s on %s: %w", b.Asset, b.Venue, err)
		}
		if b.Amount.Asset() != b.Asset {
			return fmt.Errorf("spot baseline %s on %s carries a %s amount: %w",
				b.Asset, b.Venue, b.Amount.Asset(), ErrSpotBaselineAssetMismatch)
		}
		key := spotAssetKey{b.Venue, b.Asset}
		if _, dup := baselines[key]; dup {
			return fmt.Errorf("spot baseline %s on %s: %w", b.Asset, b.Venue, ErrSpotBaselineDuplicate)
		}
		baselines[key] = struct{}{}
		if _, consumed := bases[key]; !consumed {
			return fmt.Errorf("spot baseline %s on %s: %w", b.Asset, b.Venue, ErrSpotBaselineUnconsumed)
		}
		// The quote-overlap deny (record §70c, round-3 finding 4): `ours`
		// counts base flows and same-asset fees, so a configured pair SPENDING
		// the baselined asset as its quote consideration moves the wallet
		// where the formula cannot see — the wrong number is made
		// inexpressible rather than computed. The deny is about the baseline,
		// not the pair: the same roster without this baseline constructs.
		for _, r := range spot.Roster {
			if r.Instrument.Venue == b.Venue && r.Quote == b.Asset {
				return fmt.Errorf("spot baseline %s on %s is %s's quote: %w",
					b.Asset, b.Venue, r.Instrument, ErrSpotBaselineIsQuote)
			}
		}
	}

	if len(spot.Baselines) > 0 && spot.Balances == nil {
		return ErrSpotBalancesMissing
	}
	return nil
}

// spotAssetKey is the spot assertion's granularity: one venue's balance of one
// asset. Two spot instruments sharing a base share the cell (§70c).
type spotAssetKey struct {
	Venue ids.Venue
	Asset fixedpoint.Asset
}

// PositionScope is ports.PositionScope, re-exported for the composition seam.
//
// A composition root has to NAME this type to configure which enumerations the
// invariant is asserted over, and §3.6 rule 6's gate forbids any root importing
// oms/application/ports — the port package is write authority as far as
// singlewriter is concerned, and it polices the abstraction as well as the
// adapter. An alias here is the narrow seam that keeps both true: the root names
// application.PositionScope, which it may already import for ResolutionPolicy,
// and no import edge to the policed package appears anywhere.
type PositionScope = ports.PositionScope

// InstrumentDelta is one instrument's disagreement between the venue's position
// and the exposure this system's own orders account for.
type InstrumentDelta struct {
	// Instrument is which market.
	Instrument ids.VenueInstrument

	// Expected is the signed net this system's orders explain: the sum over
	// every joined order on this instrument of its side applied to the
	// cumulative quantity the venue reported for it.
	Expected fixedpoint.Quantity

	// Observed is the venue's own signed position.
	Observed fixedpoint.Quantity

	// Unexplained is Observed − Expected, and it is carried rather than left to
	// the reader because it is the number the operator acts on: its SIGN says
	// which way the account is exposed beyond what we can account for, and a
	// report that made the reader subtract would be one where a subtraction in
	// the wrong order is invisible.
	Unexplained fixedpoint.Quantity
}

// Join is what one venue join established.
type Join struct {
	// Orders is how many orders were joined to the venue, and Fills is how many
	// executions the join admitted into the projection.
	//
	// Duplicates counts the executions the fill ledger already held. It is not
	// noise: this join re-sweeps from each order's authorization anchor on every
	// boot rather than advancing a cursor, so on the second boot after a fill
	// EVERY row is a duplicate — a run reporting fills and no duplicates where
	// duplicates were expected means the projection lost them.
	Orders     int
	Fills      int
	Duplicates int

	// Advanced is how many orders' lifecycle STATE this join moved to the
	// venue's own reading.
	//
	// It counts writes, not orders: a re-read whose stored state already equals
	// the venue's is not an advance, which matters because that is the ordinary
	// case on every boot after the first. A run reporting advances equal to
	// Orders on a warm boot means the projection lost its states.
	Advanced int

	// Contradicted are the orders whose venue reading SM §1 has no edge to from
	// the state this system holds — a venue reporting Rejected about an order we
	// watched reach Acknowledged, say.
	//
	// It is a REPORT entry rather than an error because it is a fact about ONE
	// order's two histories, and aborting the join over it would leave every
	// later order unqueried. It is not retryable and must not be: the same
	// account answers the same way, so the reconciliation seam maps it to
	// NeedsOperator beside Unattributed — see Reconciler.joinOutcome.
	Contradicted []ids.ClientOrderID

	// Absent are the orders the venue says it does not hold. They are NOT a
	// failure and NOT a resolution: §5.3's stable-not-found rule is what decides
	// what an absence licenses, through four gates this use case does not
	// implement, so they are handed back for Resolver to run.
	Absent []ids.ClientOrderID

	// Incomplete are the orders whose executions sweep did not prove coverage
	// through the instant it was asked about — a page budget spent, a venue that
	// stopped answering. Retryable, and the reason the join must not conclude:
	// a truncated sweep's silence is not evidence, and admitting the rows it did
	// return would rebuild a cumulative that is right about everything except
	// how much is missing.
	Incomplete []ids.ClientOrderID

	// Unattributed are the orders whose venue-reported cumulative disagrees with
	// what this join could admit for them.
	//
	// It is the join's own cross-check and it uses two DIFFERENT venue surfaces:
	// the order surface states a total, the executions sweep states the rows. A
	// disagreement means an execution exists that the sweep did not attribute to
	// this order — on Binance the ordinary cause is a trade list carrying no
	// client order id for an order whose venue id we could not read — and the
	// exposure it represents is real whether or not we can name its row.
	Unattributed []ids.ClientOrderID
}

// Assertion is §7.5's net-delta invariant as this system can state it: what the
// venue's position enumeration holds, against what our own orders account for.
//
// # It is a separate result from Join because it is a separate GATE
//
// Until slice G1 the two were one call and one report, and the fusion had a
// consequence nothing could see from inside the join: §5.3's absence resolution
// belongs BETWEEN them. An order the venue's order surface does not hold is
// handed to Resolver, and Resolver is what sweeps its executions — so at the
// instant the fused call asserted, every absent order contributed the zero
// cumulative the replay gave it, while the position those executions produced was
// standing at the venue and counted. See Reconciler, which is the composition
// that made the ordering visible.
type Assertion struct {
	// Unexplained are the instruments whose venue position exceeds what our
	// orders account for. §7.5's "unexplained position deltas", which "alert for
	// operator clearance" and are "not auto-corrected".
	Unexplained []InstrumentDelta

	// Foreign are the venue's own symbols that this deployment does not trade at
	// all, in the venue's spelling, sorted. §7.5's "foreign orders".
	Foreign []string

	// SpotUnexplained are the spot balance deltas no journaled order explains:
	// per (venue, base asset) with a recorded baseline, the venue's balance
	// minus the baseline, against what our spot orders account for (fee-
	// adjusted — a spot buy's fee is deducted from the received base asset, so
	// ours is signed cumulative minus fees charged in the asset). §7.5's
	// "unexplained position delta" for the leg with no position surface:
	// latching, operator-cleared, never auto-corrected. Both signs latch —
	// positive is exposure we cannot account for, negative is coins the
	// journal says we hold that the venue does not.
	//
	// The granularity is the ASSET, not the instrument: two spot instruments
	// sharing a base share one balance, and a delta cannot be attributed
	// between them. Pinning it on whichever sorted first would become
	// load-bearing the day a second pair lands, with nothing refusing it.
	SpotUnexplained []AssetDelta

	// SpotUnconcluded are the baselined assets whose balance semantics this
	// read could not pin (ports.UnpinnedReason — Bybit's unverified
	// walletBalance/locked split, or outstanding borrow). Not a delta and not
	// a pass: the check RAN, concluded, and left work a further attempt may
	// finish — a resting spot order completes and the next read pins. It does
	// not join Holds (latching a timer is the halts-with-every-light-green
	// failure) and it must not vanish into a pass; the reconciliation seam
	// maps it to boot.OutcomeIncomplete, below NeedsOperator in precedence.
	SpotUnconcluded []UnconcludedBalance

	// Unasserted are the instruments this join rebuilt fills for and could NOT
	// compare against a position, each with the reason it could not.
	//
	// # It is a field rather than a doc comment because ONE of its reasons is
	// # permanent
	//
	// Every spot leg is in it. Neither venue has a position surface for spot — a
	// spot balance is not a position — and the balance that exists instead
	// includes coins this system never bought, so it is comparable to our
	// exposure only against a baseline nothing records. §7.5 lists `Positions`
	// among the surfaces its join uses without saying that one of a
	// cash-and-carry package's two legs is outside it.
	//
	// That leg does NOT block the gate, and the direction is deliberate: a leg
	// whose invariant can never be asserted would hold the posture below trading
	// for ever, which is this repo's recorded halts-itself-with-every-light-green
	// failure. So the limit is reported on every boot instead, where an operator
	// sees it, rather than inherited silently by whoever reads a clean report.
	//
	// # The reason is carried because the exemption was argued from ONE cause and
	// # this bucket has two
	//
	// Slice F2 wrote the exemption above and computed membership as "no configured
	// scope covered this instrument", which a DERIVATIVE reaches too — an
	// enumeration the deployment did not configure, or a symbol dropped from the
	// config while the journal still holds an open order on it. That instrument is
	// not one whose invariant can never be asserted; it is one whose invariant did
	// not run, over an account that may be holding exposure. Exempting it makes
	// GateNetDeltaAsserted report Passed on a boot that issued no position query
	// at all. See UnassertedReason, which is where the two causes were separated.
	Unasserted []UnassertedInstrument
}

// UnassertedInstrument is one instrument the net-delta invariant did not cover,
// and why.
type UnassertedInstrument struct {
	Instrument ids.VenueInstrument
	Reason     UnassertedReason
}

// AssetDelta is one baselined asset's disagreement between the venue's balance
// and what this system's spot orders account for.
//
// Observed is carried PRE-SUBTRACTED (balance minus baseline) for the same
// reason InstrumentDelta carries Unexplained: the operator acts on the number,
// and a report that makes the reader subtract is one where a subtraction in
// the wrong order is invisible.
type AssetDelta struct {
	Venue ids.Venue
	Asset fixedpoint.Asset

	// Expected is the fee-adjusted signed net our spot orders explain.
	Expected fixedpoint.Quantity

	// Observed is the venue's balance minus the recorded baseline — the
	// portion of the wallet that OUGHT to be ours.
	Observed fixedpoint.Quantity

	// Unexplained is Observed − Expected.
	Unexplained fixedpoint.Quantity
}

// UnconcludedBalance is one baselined asset the assertion could neither
// conclude about nor fail: the balance read reported its total unpinnable
// this read, with the reason.
type UnconcludedBalance struct {
	Venue  ids.Venue
	Asset  string
	Reason ports.UnpinnedReason
}

// UnpinnedReason is ports.UnpinnedReason, re-exported for the composition
// seam — PositionScope's argument, one type over: a root that renders the
// report must NAME the reason type to switch on it, and §3.6 rule 6 forbids
// any root importing oms/application/ports. The members ride along for the
// same reason.
type UnpinnedReason = ports.UnpinnedReason

const (
	UnpinnedLockedSplitUnverified = ports.UnpinnedLockedSplitUnverified
	UnpinnedBorrowOutstanding     = ports.UnpinnedBorrowOutstanding
)

// UnassertedReason is why an instrument the join rebuilt fills for was left out
// of the net-delta assertion.
//
// [SPEC EDIT LANDED — v0.9.39] §7.5 states the exemption over one cause — "an
// instrument whose exposure cannot be compared MUST be reported … and MUST NOT
// hold the posture", argued from the fact that no venue publishes a position
// surface for spot — and the code that implemented it collected instruments by a
// WIDER predicate. §7.5 now distinguishes the two and says which one holds.
//
// # Why the property is a method on the reason rather than a set the caller knows
//
// UnprovedReason's argument one type over, and it is the same failure it exists
// to foreclose: a caller that recognised "the reasons that do not hold" would
// give a reason added next year the permissive answer by omission — an instrument
// silently outside the invariant, reported as ordinary boot progress for ever.
// Naming every member here makes exhaustiveswitch force the choice instead.
type UnassertedReason uint8

const (
	// UnassertedReasonUnspecified is the zero value and names no reason. Refused
	// by Validate: an instrument outside the invariant with no account of why is
	// exactly the state this type exists to make unwritable.
	UnassertedReasonUnspecified UnassertedReason = iota

	// UnassertedBaselineUnrecorded is a SPOT instrument whose base asset has no
	// recorded baseline at its venue, so the balance comparison cannot run.
	//
	// It replaced UnassertedNoPositionSurface in task #179 (record §70), and
	// the posture answer flipped with it. The old member was §7.5's permanent,
	// venue-imposed gap — no venue publishes a position surface for spot, so no
	// enumeration could EVER cover it, and holding the posture on a gap no
	// action could close would halt a correct system for ever. The baseline
	// construct closes the gap: recording one is a one-time operator act
	// (`arblab baseline record`, then pin the document), on the same footing as
	// spend_lock_dir. What remains is a deployment that has not done it — and
	// that deployment's spot balance may be hiding exposure the journal does
	// not explain, which is exactly the epistemic position of a derivative
	// outside every scope. The old exemption's membership is empty and §7.5's
	// spot paragraph now says so.
	UnassertedBaselineUnrecorded

	// UnassertedScopeNotEnumerated is a DERIVATIVE instrument that no configured
	// scope covers: an enumeration this deployment did not declare, or one it
	// declared for a venue or family other than the instrument's.
	//
	// It is the same words as the case above — "we could not compare this" — about
	// a completely different situation. A position surface EXISTS for it and was
	// not asked. So the venue may be holding exposure on this instrument, and the
	// only reason nothing reported it is that the query was never issued.
	UnassertedScopeNotEnumerated
)

const maxUnassertedReason = UnassertedScopeNotEnumerated

// The roster above holds NO member that exempts an instrument from the
// posture: HoldsPosture answers true for every valid reason since task #179.
// The method stays — the question is the reason's to answer, and a member
// added next year gets asked it by exhaustiveswitch rather than defaulted.

// Validate reports whether r names a reason, excluding the sentinel.
func (r UnassertedReason) Validate() error {
	if r == UnassertedReasonUnspecified || r > maxUnassertedReason {
		return fmt.Errorf("unasserted reason %d: %w", uint8(r), ErrUnassertedReasonInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (r UnassertedReason) String() string {
	switch r {
	case UnassertedReasonUnspecified:
		return "UnassertedReasonUnspecified"
	case UnassertedBaselineUnrecorded:
		return "UnassertedBaselineUnrecorded"
	case UnassertedScopeNotEnumerated:
		return "UnassertedScopeNotEnumerated"
	}
	return fmt.Sprintf("UnassertedReason(%d)", uint8(r))
}

// HoldsPosture reports whether an instrument left out for this reason must hold
// the boot posture below trading.
//
// Every current member answers TRUE, and the method survives that: the
// question belongs to the reason rather than to a caller counting reasons it
// recognises, and the exemption that once lived here (a permanent gap held
// against the posture halts a correct system for ever) lost its member when
// task #179 made the spot gap closable. An unrecorded baseline exempted would
// be the invariant reporting a pass over a wallet it did not compare, which is
// the check saying "clear" about a question it never asked.
//
// It mirrors UnprovedReason.RetryMayLift down to the signature, deliberately: the
// two are the same shape of decision — a property of the reason, not of the
// caller — and a reader who has understood one has understood this one.
func (r UnassertedReason) HoldsPosture() (bool, error) {
	switch r {
	case UnassertedBaselineUnrecorded:
		return true, nil
	case UnassertedScopeNotEnumerated:
		return true, nil
	case UnassertedReasonUnspecified:
		// Named rather than left to the trailing return (§3.6 rule 10). A zero
		// reason holds, for the reason below, and it is a distinct fact from a
		// corrupted one.
		return true, r.Validate()
	}
	// Anything unrecognised holds AND reports. Holding is the safe direction — the
	// permissive one is an instrument outside the invariant that nobody is told
	// about — and the error is what stops a caller reading the bool alone.
	return true, r.Validate()
}

// Holds reports whether this assertion found something §7.5 refuses to
// auto-correct: a position no order of ours explains, a symbol this deployment
// does not trade at all, or an instrument left outside the invariant for a reason
// that is not the permanent one.
//
// The third disjunct is what slice G2 added. Without it the invariant answers
// "clear" about instruments it never queried — see UnassertedReason.
//
// # The error result is discarded, and the reason it is SAFE to discard is a
// # property of HoldsPosture rather than a hope
//
// HoldsPosture answers true for every value it also reports an error on — the
// zero reason and any unrecognised one — so the bool alone is total in the
// direction that matters, and its table test asserts both halves for exactly
// those values. The first version of this loop tested `err != nil || holds`,
// which reads as defence in depth and is not: no input reaches it with err
// non-nil and holds false, so it is a branch nothing can execute, and a mutation
// removing it SURVIVED. An unreachable guard is worse than no guard — it reads as
// handled, and the next reader stops asking who guarantees the invariant.
func (a Assertion) Holds() bool {
	if len(a.Unexplained) > 0 || len(a.Foreign) > 0 || len(a.SpotUnexplained) > 0 {
		return true
	}
	for _, u := range a.Unasserted {
		if holds, _ := u.Reason.HoldsPosture(); holds {
			return true
		}
	}
	// SpotUnconcluded is deliberately NOT here: it is the ran-and-concluded,
	// work-remains outcome, and latching it would demand operator clearance
	// for a timer. The reconciliation seam maps it to OutcomeIncomplete,
	// BELOW NeedsOperator — so a boot carrying both a delta and an unpinned
	// row latches on the delta, and the unpinned row never masks it.
	return false
}

// Join rebuilds executed quantity from the venue.
//
// # It takes the ids whose state means the venue MAY hold a record
//
// Recovery.Dispatched, not Recovery.Unresolved. The unresolved set is §6.7's —
// orders whose exposure is still uncertain — and it excludes every terminal,
// which is where this system's executions actually are: a marketable IOC that
// took part of the book ends Filled or Canceled with real fills, and the replay
// gave it a zero cumulative like everything else. Joining only the uncertain
// orders would rebuild the quantity for the orders that have none.
//
// # err != nil means the projection is PART-REBUILT and must not be traded on
//
// A venue query that fails, a repository that refuses a fill, a quantity whose
// asset tag disagrees: each leaves the fills admitted so far in place, on
// Rebuild's argument — they are what the venue said, and discarding them would
// leave an operator inspecting a store that disagrees with both the log and the
// exchange. The posture is what stops trading, not the emptiness of a map.
func (j *Joiner) Join(dispatched []ids.ClientOrderID) (Join, error) {
	report := Join{}
	for _, id := range dispatched {
		rec, err := j.load(id)
		if err != nil {
			return report, err
		}
		if _, err := j.joinOne(rec, &report); err != nil {
			return report, err
		}
		report.Orders++
	}
	return report, nil
}

// Assert is §7.5's net-delta invariant: "every unit of the venue's position is
// attributable to an order this system journaled", per derivative instrument in
// the enumerated scopes.
//
// # It re-reads the projection rather than taking Join's arithmetic
//
// Join could have handed its running total over and the two would agree on the
// boot where nothing else writes between them — which is exactly the boot this
// system does not have. §5.3's absence resolution runs BETWEEN the two calls and
// admits executions of its own, so a total computed during the join is a total
// taken before the last writer finished. Re-deriving here makes the assertion a
// function of the projection as it stands at the instant it is asserted, which is
// the only reading §7.5's "before trading resumes" can mean.
//
// It takes the same dispatched set for the same reason Join does: a Planned-only
// order was never sent, so it contributes no exposure and asking about it would
// be asking the venue about an order this system has not placed.
func (j *Joiner) Assert(dispatched []ids.ClientOrderID) (Assertion, error) {
	report := Assertion{}
	expected := make(map[ids.VenueInstrument]fixedpoint.Quantity, len(dispatched))
	fees := make(map[spotAssetKey]fixedpoint.Quantity)

	for _, id := range dispatched {
		rec, err := j.load(id)
		if err != nil {
			return report, err
		}
		signed, err := signedQuantity(rec.Order.Side, rec.Cumulative)
		if err != nil {
			return report, fmt.Errorf("assert order %q executed: %w", id, err)
		}
		running, ok := expected[rec.Order.Instrument]
		if !ok {
			running = fixedpoint.ZeroQuantity(rec.Cumulative.Asset())
		}
		running, err = running.Add(signed)
		if err != nil {
			return report, fmt.Errorf("assert order %q: %w", id, err)
		}
		expected[rec.Order.Instrument] = running

		// Fees are collected over SPOT orders only, keyed by the asset the venue
		// charged in — never by the order's own base. Both scopings are §70c's:
		// a BNB-discount fee on a BTCUSDT spot trade debits the spot BNB
		// balance, so the fee that moves a baselined asset can ride another
		// pair's order; while a Binance FUTURES fee debits the futures wallet,
		// which the spot balance read never sees, so summing perp fees would
		// subtract from a wallet they never touched.
		if rec.Order.Instrument.Product == ids.ProductSpot {
			for _, fee := range rec.Fees {
				q, err := feeQuantity(fee)
				if err != nil {
					return report, fmt.Errorf("assert order %q fee: %w", id, err)
				}
				key := spotAssetKey{rec.Order.Instrument.Venue, fee.Asset()}
				total, ok := fees[key]
				if !ok {
					total = fixedpoint.ZeroQuantity(fee.Asset())
				}
				total, err = total.Add(q)
				if err != nil {
					return report, fmt.Errorf("assert order %q fees: %w", id, err)
				}
				fees[key] = total
			}
		}
	}

	if err := j.assert(expected, &report); err != nil {
		return report, err
	}
	if err := j.assertSpot(expected, fees, &report); err != nil {
		return report, err
	}

	// Sorted on the WHOLE instrument identity rather than on the symbol.
	// A cash-and-carry package is spot BTCUSDT and perp BTCUSDT, so the symbol is
	// the one component the two legs share — a symbol-keyed comparison makes them
	// equal, sort.Slice is not stable, and the two legs then swap places between
	// runs. That is Checklist.Blocking's argument arriving on a report whose most
	// common content is exactly one such pair.
	sort.Slice(report.Unexplained, func(a, b int) bool {
		return report.Unexplained[a].Instrument.String() < report.Unexplained[b].Instrument.String()
	})
	sort.Strings(report.Foreign)
	sort.Slice(report.Unasserted, func(a, b int) bool {
		return report.Unasserted[a].Instrument.String() < report.Unasserted[b].Instrument.String()
	})
	sort.Slice(report.SpotUnexplained, func(a, b int) bool {
		x, y := report.SpotUnexplained[a], report.SpotUnexplained[b]
		if x.Venue != y.Venue {
			return x.Venue < y.Venue
		}
		return x.Asset < y.Asset
	})
	sort.Slice(report.SpotUnconcluded, func(a, b int) bool {
		x, y := report.SpotUnconcluded[a], report.SpotUnconcluded[b]
		if x.Venue != y.Venue {
			return x.Venue < y.Venue
		}
		return x.Asset < y.Asset
	})
	return report, nil
}

// feeQuantity is the ONE site where a fee crosses dimensions: a fee charged in
// an asset is a flow OF that asset, and where flows net against holdings the
// dimension is quantity-of-asset. Same asset tag, same canonical scale —
// lossless, no rounding decision arises.
func feeQuantity(fee fixedpoint.Money) (fixedpoint.Quantity, error) {
	return fixedpoint.QuantityFromMantissa(fee.Asset(), fee.Mantissa())
}

// load reads one order and refuses one the venue cannot hold.
//
// Shared by Join and Assert so the two cannot disagree about which orders are in
// scope. That is not tidiness: they are handed the same list by the same caller
// and a divergence would show up as an assertion computed over a different set
// from the one whose quantity was rebuilt — a mismatch with no symptom except a
// wrong number.
func (j *Joiner) load(id ids.ClientOrderID) (ports.OrderRecord, error) {
	rec, err := j.orders.Load(id)
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	mayExist, err := rec.State.MayExistAtVenue()
	if err != nil {
		return rec, fmt.Errorf("join order %q at %s: %w", id, rec.State, err)
	}
	if !mayExist {
		// A Planned-only order was never dispatched, so there is nothing at the
		// venue to ask about and a query would run to its deadline over an
		// order this system merely has not sent. Refused rather than skipped:
		// the caller derived this list from a state predicate, and one that
		// disagrees means the two readings have drifted.
		return rec, fmt.Errorf("join order %q at %s: %w", id, rec.State, ErrOrderNotAtVenue)
	}
	return rec, nil
}

// joinOne reads one order's venue record and admits the executions belonging to
// it, returning the record as the projection now holds it.
func (j *Joiner) joinOne(rec ports.OrderRecord, report *Join) (ports.OrderRecord, error) {
	id := rec.Order.ClientOrderID

	status, err := j.venue.Resolve(rec.Order)
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if err := status.Presence.Validate(); err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if status.Presence == ports.VenueHasNoOrder {
		// Handed to §5.3's rule rather than concluded here. An absence is the one
		// answer this use case must not act on: it is the input to the four-gate
		// stable-not-found contract, and reading it as "no executions" would be
		// the inference §5.3 forbids in as many words.
		//
		// It returns SHORT of advance, and that is the same refusal stated about
		// the state write: the terminal an absence resolves to is
		// AbsentAfterFinality, which the four gates alone can license — see
		// ErrVenueStateNotAnAbsence, which refuses the mirror case where the
		// venue hands us the order and calls it absent.
		report.Absent = append(report.Absent, id)
		return rec, nil
	}

	// The sweep runs from the order's own anchor to the venue's clock now. The
	// anchor's EARLIEST edge cannot be after the venue created the order, so no
	// execution of ours precedes it; `now`'s earliest edge is the conservative
	// end, because a window claiming to reach further than the clock can prove
	// would make a truncated sweep look complete.
	if err := rec.AuthorizedAt.Validate(); err != nil {
		// The venue holds an order whose authorization instant this system does
		// not have. The replay is what supplies it (§5.3's anchor, journaled since
		// slice F1), so an unset one here means the record and the log disagree —
		// and sweeping from the epoch instead would ask both venues for a window
		// they refuse as too wide.
		return rec, fmt.Errorf("join order %q: %w: %w", id, ports.ErrAuthorizationNeedsAnchor, err)
	}
	until := j.clk.ServerNow(rec.Order.Instrument.Venue)
	if err := until.Validate(); err != nil {
		return rec, fmt.Errorf("join order %q on %s: %w", id, rec.Order.Instrument.Venue, err)
	}

	// Two authorities can now name this order's venue id — the live query's
	// answer and the ack the dispatch persisted — and they are compared BEFORE
	// any row is attributed, because the loser of a disagreement decides which
	// sweep rows become this order's fills. A non-empty mismatch is refused
	// outright: under the banked non-reuse fact there is no innocent reading,
	// and picking either id would apply executions on a coin flip's authority.
	// Either side being empty is not a mismatch — a lost ack left the store
	// silent, and a surface that returns no id left the query silent; silence
	// agrees with everything.
	if rec.VenueOrderID != "" && status.VenueOrderID != "" && rec.VenueOrderID != status.VenueOrderID {
		return rec, fmt.Errorf("join order %q: store holds venue id %q, venue reports %q: %w",
			id, rec.VenueOrderID, status.VenueOrderID, ErrVenueOrderIDMismatch)
	}

	sweep, err := j.venue.SweepExecutions(rec.Order, rec.AuthorizedAt.Earliest(), until.Earliest())
	if err != nil {
		return rec, fmt.Errorf("join order %q: %w", id, err)
	}
	if sweep.ProvenThrough.Before(until.Earliest()) {
		// Short of advance as well, and deliberately: SM §2 step 3's final
		// executions sweep is what closes a terminal, and the contract is "the ONE
		// canonical boundary … never a weaker cancel-ack-only variant" — which a
		// state written from a status read alone would be.
		//
		// It is STRICTER than SM §2 requires, in the safe direction, and the
		// asymmetry with Resolver is worth naming rather than leaving to be found:
		// Resolver.present skips the sweep for a NON-terminal reading ("the order
		// is live, its interval has not collapsed, and there is no finality to
		// prove"), while here the gate precedes every target including
		// Acknowledged. A state not advanced HOLDS the possible-delta interval,
		// which is the direction that costs nothing but a retry, and a short sweep
		// is retryable on the next poll.
		report.Incomplete = append(report.Incomplete, id)
		return rec, nil
	}

	// Past the mismatch check the two sources agree or one is silent, so
	// "this order's venue id" has exactly one non-empty answer and either
	// field may carry it — the live id ordinarily, the persisted ack when a
	// surface answered without one. The index is deliberately NOT consulted —
	// nil, not an oversight: an entry for this order could only restate the
	// id resolved here, and entries for OTHER orders only re-derive the
	// rule-out the different-id arm already makes. See attribute's doc.
	venueID := status.VenueOrderID
	if venueID == "" {
		venueID = rec.VenueOrderID
	}
	ours, _ := attribute(sweep.Executions, id, venueID, nil)
	for _, e := range ours {
		fill := domain.Fill{
			ExecutionID: string(e.ExecutionID),
			Account:     j.account,
			Instrument:  rec.Order.Instrument,
			Order:       id,
			Quantity:    e.Quantity,
			Price:       e.Price,
			Fee:         e.Fee,
			Ts:          e.At,
		}
		applied, changed, err := j.orders.ApplyFill(fill, rec.Version)
		if err != nil {
			return rec, fmt.Errorf("join order %q execution %q: %w", id, e.ExecutionID, err)
		}
		rec = applied
		if changed {
			report.Fills++
			continue
		}
		report.Duplicates++
	}

	// The cross-check: the order surface's own total against the rows we could
	// attribute. Two venue surfaces, and a disagreement is exposure that exists
	// and cannot be named.
	agree, err := rec.Cumulative.Cmp(status.Executed)
	if err != nil {
		return rec, fmt.Errorf("join order %q cumulative against venue: %w", id, err)
	}
	if agree != 0 {
		report.Unattributed = append(report.Unattributed, id)
	}

	// LAST, after the executions are admitted and cross-checked. The order is not
	// arithmetic — nothing in advance reads a quantity — it is what the record
	// MEANS: the reading is complete before its finality is journaled.
	//
	// It runs even when the cross-check disagreed. §5.3 keeps state and ledger
	// orthogonal ("nothing relates State to Cumulative"), the disagreement is
	// about AMOUNT and this is about FINALITY, and refusing to advance would
	// leave the order in the state this slice exists to move it out of in order
	// to punish a number the report already carries. The consumer that needs both
	// gates on both.
	return j.advance(rec, status, report)
}

// advance journals and applies the venue's own lifecycle reading. It is the step
// that moves an order this system placed off StateSubmitted.
//
// # The reading was in hand for four slices and was never written
//
// ports.VenueOrder.State is "the venue's lifecycle reading, already translated
// into SM §1's vocabulary by the ACL", and joinOne has fetched it since slice F2
// to attribute fills and run the Unattributed cross-check. Nothing wrote it, and
// nothing noticed, because every consumer's success condition stopped short of
// finality: `arblab exercise run` treats StateSubmitted AS its success state
// (exercise/app.go), so the single-venue root never needed to know whether a leg
// filled. #236 is the first thing that does.
//
// # Legality is established BEFORE the journal, and the ordering is forced
//
// Resolver.record's argument, unchanged: ports.Journal is append-only and
// "refuses to hold a second copy of the edge set", while the write repository
// checks domain.CanTransition and would refuse. Journaling first and discovering
// the refusal second would leave the durable log holding a transition the
// projection rejected — and §7.5 replays the log.
//
// # A store's refusal is an ERROR here, and a VALUE in Resolver. Both follow
// # from their own contract
//
// Resolver promises "err != nil means NOTHING was written", so a half-written
// resolution has to leave as a Resolution with Journaled/Applied. Join promises
// the reverse — "err != nil means the projection is PART-REBUILT and must not be
// traded on" — and the boot posture is what stops trading. A log that did not
// take the record is exactly that condition, so it leaves by the error return.
func (j *Joiner) advance(rec ports.OrderRecord, status ports.VenueOrder, report *Join) (ports.OrderRecord, error) {
	id := rec.Order.ClientOrderID

	if err := status.State.Validate(); err != nil {
		// A malformed reading is a broken translation, not a disagreement about
		// this order's history, so it leaves by the error return rather than
		// joining Contradicted — whose members are two WELL-FORMED readings SM §1
		// has no edge between. Filing them together would put an adapter defect
		// and a venue disagreement in one bucket an operator has one response for.
		return rec, fmt.Errorf("join order %q: venue state: %w", id, err)
	}

	if status.State == domain.StateAbsentAfterFinality {
		// The venue HAS this order, so proved absence is not among the answers
		// available here whatever the reading says. §5.3 gives that terminal four
		// gates and Resolver is what runs them; recording it from a present order
		// is the evidence-class conflation FLAG-2 exists to prevent. The guard is
		// not hypothetical bookkeeping: Submitted -> AbsentAfterFinality became a
		// legal edge in this same slice, so the domain would now permit precisely
		// the write this refuses.
		return rec, fmt.Errorf("join order %q: venue reports %s: %w",
			id, status.State, ErrVenueStateNotAnAbsence)
	}

	if rec.State == status.State {
		// The ordinary case on every boot after the first. Checked here rather
		// than by inspecting CanTransition's ErrTransitionNoOp, for replay.State's
		// reason one layer over: the two are not the same claim — that sentinel is
		// about a self-edge, this is about whether the READING advances the
		// history — and they part company the moment anything else can write
		// between the load and this line.
		return rec, nil
	}

	if err := domain.CanTransition(rec.State, status.State); err != nil {
		report.Contradicted = append(report.Contradicted, id)
		return rec, nil
	}

	if err := j.journal.AppendState(id, status.State); err != nil {
		return rec, fmt.Errorf("join order %q: journal %s: %w", id, status.State, err)
	}
	applied, err := j.orders.ApplyState(id, rec.Version, status.State)
	if err != nil {
		return rec, fmt.Errorf("join order %q: apply %s: %w", id, status.State, err)
	}
	report.Advanced++
	return applied, nil
}

// assert compares each configured scope's positions against what the join
// accounted for.
//
// # Every instrument is answered for exactly once, and the loop is written to
// # make that checkable
//
// Three sets are in play and they overlap: instruments we joined orders on,
// instruments the venue reported a position for, and instruments in neither. An
// instrument we hold orders on but that the venue did not report is FLAT at the
// venue, which is a real answer here — the enumeration is not scoped by symbol,
// so absence from it is the venue's statement rather than a question we failed
// to ask. An instrument the venue reported that we joined no orders on has an
// expected of zero, and every unit of it is unexplained.
func (j *Joiner) assert(expected map[ids.VenueInstrument]fixedpoint.Quantity, report *Assertion) error {
	asserted := make(map[ids.VenueInstrument]struct{}, len(expected))

	for _, scope := range j.scopes {
		venuePositions, err := j.positions.Positions(scope)
		if err != nil {
			return fmt.Errorf("assert net delta over %s: %w", scope, err)
		}
		report.Foreign = append(report.Foreign, venuePositions.Unrecognized...)

		observed := make(map[ids.VenueInstrument]fixedpoint.Quantity, len(venuePositions.Positions))
		for _, p := range venuePositions.Positions {
			observed[p.Instrument] = p.Net
		}

		// Our instruments in this scope first, then the venue's rows that were not
		// among them. The union is walked rather than either side alone: iterating
		// only our own would miss a position on an instrument we hold no orders
		// for, and iterating only the venue's would miss the opposite — a
		// cumulative we rebuilt against a venue that reports nothing, which is the
		// direction that means a fill was admitted twice.
		for instrument, mine := range expected {
			if instrument.Venue != scope.Venue || instrument.Product != scope.Product {
				continue
			}
			theirs, ok := observed[instrument]
			if !ok {
				theirs = fixedpoint.ZeroQuantity(mine.Asset())
			}
			if err := j.compare(instrument, mine, theirs, report); err != nil {
				return err
			}
			asserted[instrument] = struct{}{}
		}
		for instrument, theirs := range observed {
			if _, done := asserted[instrument]; done {
				continue
			}
			if err := j.compare(instrument, fixedpoint.ZeroQuantity(theirs.Asset()), theirs, report); err != nil {
				return err
			}
			asserted[instrument] = struct{}{}
		}
	}

	for instrument := range expected {
		if _, done := asserted[instrument]; done {
			continue
		}
		if instrument.Product == ids.ProductSpot {
			// The spot half answers for every spot instrument — asserted against
			// its baseline or reported UnassertedBaselineUnrecorded — so a spot
			// instrument here is not uncovered, it is merely not a derivative.
			// See assertSpot.
			continue
		}
		// A DERIVATIVE no scope covered: an enumeration this deployment did not
		// declare, or one declared for another venue or family. A position
		// surface EXISTS for it and was not asked, so the venue may be holding
		// exposure the account was never queried about.
		report.Unasserted = append(report.Unasserted, UnassertedInstrument{
			Instrument: instrument,
			Reason:     UnassertedScopeNotEnumerated,
		})
	}
	return nil
}

// assertSpot is the spot half of §7.5's invariant (task #179, record §70):
// per (venue, base asset) with a recorded baseline, the venue's balance minus
// the baseline must equal what our spot orders account for.
//
// # The coverage set is the roster UNION the journal, and the roster is what
// # makes a virgin boot honest
//
// A configured spot instrument with no orders must still be answered for —
// asserted or reported — or a boot over an empty journal passes having
// compared nothing. A journal-held spot instrument the config has DROPPED
// still contributes: its fills moved the same wallet, so its expected and its
// fees stay in the cell's arithmetic, and if its base has no baseline it is
// reported unrecorded exactly like a configured one. The cell is the asset,
// so "covered" is a property of the base, never of which instrument earned it.
func (j *Joiner) assertSpot(expected map[ids.VenueInstrument]fixedpoint.Quantity, fees map[spotAssetKey]fixedpoint.Quantity, report *Assertion) error {
	// Every spot instrument in play, with its base: the roster's word first,
	// the cumulative's asset tag for journal-only instruments.
	instruments := make(map[ids.VenueInstrument]fixedpoint.Asset, len(j.spot.Roster))
	for _, r := range j.spot.Roster {
		instruments[r.Instrument] = r.Base
	}
	for instrument, quantity := range expected {
		if instrument.Product != ids.ProductSpot {
			continue
		}
		if _, known := instruments[instrument]; !known {
			instruments[instrument] = quantity.Asset()
		}
	}
	if len(instruments) == 0 {
		return nil
	}

	baselines := make(map[spotAssetKey]fixedpoint.Quantity, len(j.spot.Baselines))
	for _, b := range j.spot.Baselines {
		baselines[spotAssetKey{b.Venue, b.Asset}] = b.Amount
	}

	// Group instruments into cells and split covered from unrecorded.
	cells := make(map[spotAssetKey][]ids.VenueInstrument, len(instruments))
	for instrument, base := range instruments {
		key := spotAssetKey{instrument.Venue, base}
		cells[key] = append(cells[key], instrument)
	}

	balances := make(map[ids.Venue]ports.BalanceReport)
	keys := make([]spotAssetKey, 0, len(cells))
	for key := range cells {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].Venue != keys[b].Venue {
			return keys[a].Venue < keys[b].Venue
		}
		return keys[a].Asset < keys[b].Asset
	})

	for _, key := range keys {
		baseline, recorded := baselines[key]
		if !recorded {
			for _, instrument := range cells[key] {
				report.Unasserted = append(report.Unasserted, UnassertedInstrument{
					Instrument: instrument,
					Reason:     UnassertedBaselineUnrecorded,
				})
			}
			continue
		}

		venueReport, err := j.spotBalances(key.Venue, balances)
		if err != nil {
			return err
		}
		if reason, unpinned := unpinnedReason(venueReport, key.Asset); unpinned {
			if err := reason.Validate(); err != nil {
				return fmt.Errorf("assert spot %s on %s: %w", key.Asset, key.Venue, err)
			}
			report.SpotUnconcluded = append(report.SpotUnconcluded, UnconcludedBalance{
				Venue:  key.Venue,
				Asset:  string(key.Asset),
				Reason: reason,
			})
			continue
		}

		// What an asset absent from the rows means is the REPORT's answer, not
		// a universal zero (§70i): under AbsenceIsZero the silence is the
		// venue's own word for a zero balance; under AbsenceInconclusive the
		// venue's documented shape contains every asset, so a missing one is
		// an anomaly the assertion must not conclude a quantity from — the
		// asset is reported unconcluded, boot.OutcomeIncomplete's cell, the
		// same non-latching non-pass an unpinned row earns.
		total := fixedpoint.ZeroQuantity(key.Asset)
		found := false
		for _, row := range venueReport.Rows {
			if row.Asset == key.Asset {
				total = row.Total
				found = true
				break
			}
		}
		if !found {
			switch venueReport.Absence {
			case ports.AbsenceIsZero:
				// total stays zero.
			case ports.AbsenceInconclusive:
				report.SpotUnconcluded = append(report.SpotUnconcluded, UnconcludedBalance{
					Venue:  key.Venue,
					Asset:  string(key.Asset),
					Reason: ports.UnpinnedAbsenceInconclusive,
				})
				continue
			case ports.AbsenceSemanticsUnspecified:
				return fmt.Errorf("assert spot %s on %s: %w",
					key.Asset, key.Venue, ports.ErrAbsenceSemanticsInvalid)
			}
		}

		ours := fixedpoint.ZeroQuantity(key.Asset)
		for _, instrument := range cells[key] {
			mine, has := expected[instrument]
			if !has {
				continue
			}
			ours, err = ours.Add(mine)
			if err != nil {
				return fmt.Errorf("assert spot %s on %s: %w", key.Asset, key.Venue, err)
			}
		}
		if feeTotal, charged := fees[key]; charged {
			ours, err = ours.Sub(feeTotal)
			if err != nil {
				return fmt.Errorf("assert spot %s on %s fees: %w", key.Asset, key.Venue, err)
			}
		}

		observed, err := total.Sub(baseline)
		if err != nil {
			return fmt.Errorf("assert spot %s on %s baseline: %w", key.Asset, key.Venue, err)
		}
		unexplained, err := observed.Sub(ours)
		if err != nil {
			return fmt.Errorf("assert spot %s on %s: %w", key.Asset, key.Venue, err)
		}
		sign, err := unexplained.Sign()
		if err != nil {
			return fmt.Errorf("assert spot %s on %s: %w", key.Asset, key.Venue, err)
		}
		if sign == 0 {
			continue
		}
		report.SpotUnexplained = append(report.SpotUnexplained, AssetDelta{
			Venue:       key.Venue,
			Asset:       key.Asset,
			Expected:    ours,
			Observed:    observed,
			Unexplained: unexplained,
		})
	}
	return nil
}

// spotBalances reads one venue's enumeration exactly once per Assert, however
// many cells it serves — the report is one answer about one wallet, and two
// reads could disagree mid-walk, attributing one instant's balance to one
// asset and another's to its neighbour.
func (j *Joiner) spotBalances(venue ids.Venue, cache map[ids.Venue]ports.BalanceReport) (ports.BalanceReport, error) {
	if report, done := cache[venue]; done {
		return report, nil
	}
	report, err := j.spot.Balances.Balances(ports.SpotBalanceScope{Venue: venue})
	if err != nil {
		return ports.BalanceReport{}, fmt.Errorf("assert spot balances on %s: %w", venue, err)
	}
	if err := report.Absence.Validate(); err != nil {
		// Checked here, once per read, whether or not any asset is absent: a
		// report that never stated its silences' meaning is a wiring defect,
		// and it must be loud on the boot where nothing happens to be absent,
		// not first on the boot where something is.
		return ports.BalanceReport{}, fmt.Errorf("assert spot balances on %s: %w", venue, err)
	}
	seen := make(map[fixedpoint.Asset]struct{}, len(report.Rows))
	for _, row := range report.Rows {
		if _, dup := seen[row.Asset]; dup {
			// Two rows for one asset would make the cell's total a declaration-
			// order coin flip — the adapter contract violation is refused here,
			// where it is an error, not downstream where it is a wrong number.
			return ports.BalanceReport{}, fmt.Errorf("assert spot balances on %s: asset %s reported twice: %w",
				venue, row.Asset, ErrBalanceRowDuplicate)
		}
		seen[row.Asset] = struct{}{}
	}
	cache[venue] = report
	return report, nil
}

// unpinnedReason reports whether the venue's read left this asset without one
// pinnable total.
func unpinnedReason(report ports.BalanceReport, asset fixedpoint.Asset) (ports.UnpinnedReason, bool) {
	for _, u := range report.Unpinned {
		if u.Asset == string(asset) {
			return u.Reason, true
		}
	}
	return ports.UnpinnedReasonUnspecified, false
}

// compare records one instrument's disagreement, if there is one.
//
// A zero difference is not recorded, and that is the whole shape of the
// invariant: §7.5 asserts it before trading resumes, so the ordinary answer is
// an empty report and a report is a list of exceptions.
func (j *Joiner) compare(instrument ids.VenueInstrument, mine, theirs fixedpoint.Quantity, report *Assertion) error {
	unexplained, err := theirs.Sub(mine)
	if err != nil {
		return fmt.Errorf("assert net delta on %s: %w", instrument.Symbol, err)
	}
	sign, err := unexplained.Sign()
	if err != nil {
		return fmt.Errorf("assert net delta on %s: %w", instrument.Symbol, err)
	}
	if sign == 0 {
		return nil
	}
	report.Unexplained = append(report.Unexplained, InstrumentDelta{
		Instrument:  instrument,
		Expected:    mine,
		Observed:    theirs,
		Unexplained: unexplained,
	})
	return nil
}
