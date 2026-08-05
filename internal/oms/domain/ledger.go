package domain

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Ledger is §5.3's cumulative-execution fill ledger.
//
// # It never consults an order's state, and that is the whole design
//
// §5.3: "Fills are NOT gated by order lifecycle: a cumulative-execution ledger
// accepts any deduped fill for a KNOWN order regardless of its current state —
// so a post-cancel or post-SubmissionUnknown late fill still applies." Nothing
// in this file imports OrderState, calls IsTerminal, or takes a state argument.
// That absence is the requirement; a state parameter would be the defect.
//
// The reason it matters is that both halves of §6.7's classifier read this. Net
// delta is recomputed from the ledger on every fill, so a fill the ledger
// refused because the order looked finished is a position the system holds and
// does not know it holds. §5.3 makes the admission window explicitly unbounded —
// "admissible INDEFINITELY, since a legitimate fill can surface after prolonged
// downtime" — and scopes the bounded finality window to ambiguous-order
// ACCEPTANCE resolution, never to fill admission.
//
// # Known, though — the scope word is load-bearing
//
// "For a KNOWN order" is the one gate. A fill whose ClientOrderID this ledger
// has never registered is refused, because the two things it can be are a
// foreign order on the account (§6 step 10 routes those to operator clearance)
// and a journal that lost an order it should hold. A ledger that registered
// orders on demand would make the second indistinguishable from the first, and
// it would always reconcile.
//
// # Not safe for concurrent use
//
// Deliberately unsynchronised. §5.1 gives the aggregate an OCC Version and §6
// gives it a journal-then-apply write path, so serialisation is a property of
// how a write reaches this type, decided one layer out. A mutex here would make
// two concurrent writers safe from the race detector while leaving them exactly
// as wrong about §5.1's version — the failure would move from a detected crash
// to a silently lost update.
type Ledger struct {
	orders map[ids.ClientOrderID]*ledgerOrder
	seen   map[FillKey]struct{}
}

// ledgerOrder is what the ledger knows about one order: its instrument, the
// quantity it was placed for, and what has executed.
//
// The ordered quantity lives here rather than only on the Order aggregate
// because §6.7's ambiguity test is phrased in terms of "remaining executable
// quantity", and remaining is ordered minus cumulative. Splitting the two
// halves of that subtraction across two types is how they drift.
type ledgerOrder struct {
	instrument ids.VenueInstrument
	ordered    fixedpoint.Quantity
	cumulative fixedpoint.Quantity
	fills      int

	// fees is what the venue charged for this order's executions, summed per
	// asset the venue charged in. Retained since task #179: a spot BUY's fee is
	// deducted from the RECEIVED base asset on both venues, so §7.5's spot
	// balance comparison is off by exactly this sum unless it is subtractable —
	// and the fee's asset is the venue's word (a BNB-discount fee on a BTCUSDT
	// trade debits BNB), so the key is the asset actually charged, never the
	// order's own base.
	fees map[fixedpoint.Asset]fixedpoint.Money
}

// NewLedger returns an empty ledger.
func NewLedger() *Ledger {
	return &Ledger{
		orders: make(map[ids.ClientOrderID]*ledgerOrder),
		seen:   make(map[FillKey]struct{}),
	}
}

// Register makes an order known to the ledger, so its fills can be admitted.
//
// The ordered quantity supplies the cumulative total's asset tag: it starts as
// zero OF THAT ASSET rather than as fixedpoint's untagged zero, which would fail
// its first Add and make an order's first fill the one that cannot apply.
//
// A repeat registration is refused rather than resolved either way, because both
// resolutions are recoveries that look like they worked: replacing the entry
// discards a cumulative execution whose fills stay applied at the venue, and
// keeping it attaches the new order's fills to the old order's instrument.
func (l *Ledger) Register(id ids.ClientOrderID, instrument ids.VenueInstrument, ordered fixedpoint.Quantity) error {
	if id == "" {
		return ErrOrderUnset
	}
	if err := instrument.Validate(); err != nil {
		return fmt.Errorf("order %q instrument: %w", id, err)
	}
	if err := ordered.Validate(); err != nil {
		return fmt.Errorf("order %q ordered quantity: %w", id, err)
	}
	sign, err := ordered.Sign()
	if err != nil {
		return fmt.Errorf("order %q ordered quantity: %w", id, err)
	}
	if sign <= 0 {
		// Zero is the one that would otherwise pass. An order for nothing is
		// fully executed the moment it is registered, so §6.7 reads its residual
		// as determined and its possible-delta interval as the point zero —
		// while the venue holds a live order whose quantity nobody recorded.
		return fmt.Errorf("order %q is for %s: %w", id, ordered, ErrOrderedQuantityNotPositive)
	}
	if _, ok := l.orders[id]; ok {
		return fmt.Errorf("order %q: %w", id, ErrDuplicateOrder)
	}
	l.orders[id] = &ledgerOrder{
		instrument: instrument,
		ordered:    ordered,
		cumulative: fixedpoint.ZeroQuantity(ordered.Asset()),
	}
	return nil
}

// Apply admits a fill, reporting whether it changed the ledger.
//
// # A duplicate is (false, nil), not an error, and that is not leniency
//
// §4 puts the same execution on two streams — "fills also arrive here — same
// ExecutionID, applied exactly once" — and §5.3 has REST recovery re-reading
// fills the private stream already delivered. So a duplicate is the ORDINARY
// case on the ordinary path, several times per fill. Reporting it as an error
// would mean the healthy system logs errors continuously, which is the condition
// under which a real error stops being noticed.
//
// What the boolean gives a caller is the thing that actually matters: §5.3
// requires applying a fill, advancing the recovery cursor and appending journal
// and outbox events to happen in ONE transaction, and (false, nil) is how the
// caller knows there is no transaction to open.
//
// # It does not refuse an over-fill
//
// A fill taking cumulative past ordered is recorded, not rejected. §3.0's rule
// is that venue reality wins, and the over-execution is real exposure whether or
// not our arithmetic likes it — Cx6-1's "over-close fill semantics" is the same
// point from the emergency path. Remaining reports the overshoot as a negative
// quantity, where §6.7 can see it.
func (l *Ledger) Apply(f Fill) (bool, error) {
	if err := f.Validate(); err != nil {
		return false, err
	}
	key, err := f.Key()
	if err != nil {
		return false, err
	}
	order, ok := l.orders[f.Order]
	if !ok {
		return false, fmt.Errorf("execution %q names order %q: %w",
			f.ExecutionID, f.Order, ErrFillUnknownOrder)
	}
	if f.Instrument != order.instrument {
		return false, fmt.Errorf("execution %q on %s, order %q is on %s: %w",
			f.ExecutionID, f.Instrument.Symbol, f.Order, order.instrument.Symbol,
			ErrFillInstrumentMismatch)
	}

	// The dedupe check comes AFTER the order and instrument checks on purpose. A
	// fill that is both a duplicate and misattributed is a bug in whatever
	// produced it, and reporting "already seen" would retire it silently — the
	// second copy is exactly where a mapping error becomes visible.
	if _, seen := l.seen[key]; seen {
		return false, nil
	}

	// fixedpoint refuses a disagreeing asset tag here, and this is the only
	// place that question is asked — see errors.go for why there is no second
	// check of it. The wrap is what turns its message into a diagnosis.
	total, err := order.cumulative.Add(f.Quantity)
	if err != nil {
		return false, fmt.Errorf("order %q cumulative + execution %q: %w",
			f.Order, f.ExecutionID, err)
	}

	// The fee sum is computed BEFORE any mutation for the same reason total is:
	// an overflow here must leave the ledger exactly as it was. A zero fee is
	// not accumulated — an asset charged nothing and an asset never charged are
	// the same fact for the balance arithmetic this sum exists for, and an
	// entry saying "0 BNB" would be a row every reader has to learn to skip.
	feeSign, err := f.Fee.Sign()
	if err != nil {
		return false, fmt.Errorf("order %q execution %q fee: %w",
			f.Order, f.ExecutionID, err)
	}
	var feeTotal fixedpoint.Money
	if feeSign != 0 {
		existing, ok := order.fees[f.Fee.Asset()]
		if !ok {
			existing = fixedpoint.ZeroMoney(f.Fee.Asset())
		}
		feeTotal, err = existing.Add(f.Fee)
		if err != nil {
			return false, fmt.Errorf("order %q fees + execution %q: %w",
				f.Order, f.ExecutionID, err)
		}
	}

	// Mutation happens only once nothing further can fail, so a refused fill
	// leaves no half-applied state behind. §5.3's one-transaction rule is about
	// durability and this is its in-memory counterpart: the caller's rollback
	// cannot reach a map this method already wrote.
	order.cumulative = total
	order.fills++
	if feeSign != 0 {
		if order.fees == nil {
			order.fees = make(map[fixedpoint.Asset]fixedpoint.Money, 1)
		}
		order.fees[f.Fee.Asset()] = feeTotal
	}
	l.seen[key] = struct{}{}
	return true, nil
}

// Cumulative reports the total executed quantity for a known order.
//
// Zero-of-the-right-asset for an order with no fills, which is a real answer
// rather than a missing one — §6.7 recomputes net delta across a hedge group and
// an unfilled leg contributes zero to it.
func (l *Ledger) Cumulative(id ids.ClientOrderID) (fixedpoint.Quantity, error) {
	order, ok := l.orders[id]
	if !ok {
		return fixedpoint.Quantity{}, fmt.Errorf("order %q: %w", id, ErrFillUnknownOrder)
	}
	return order.cumulative, nil
}

// Remaining reports ordered minus cumulative — §6.7's "remaining executable
// quantity".
//
// It is SIGNED and may be negative, and the sign is information rather than an
// error condition: a negative remainder is a venue over-execution, which §6.7
// needs to see because the residual it implies points the other way. Clamping at
// zero would report a package as flat while it holds reverse exposure.
func (l *Ledger) Remaining(id ids.ClientOrderID) (fixedpoint.Quantity, error) {
	order, ok := l.orders[id]
	if !ok {
		return fixedpoint.Quantity{}, fmt.Errorf("order %q: %w", id, ErrFillUnknownOrder)
	}
	remaining, err := order.ordered.Sub(order.cumulative)
	if err != nil {
		return fixedpoint.Quantity{}, fmt.Errorf("order %q remaining: %w", id, err)
	}
	return remaining, nil
}

// FillCount reports how many distinct executions the ledger has admitted for an
// order.
//
// It exists for the reconciliation path rather than for the arithmetic: a
// cumulative quantity that matches the venue's while the fill count does not
// means two executions netted to the right total, which is what a
// double-counted fill plus a missed one looks like from the quantity alone.
func (l *Ledger) FillCount(id ids.ClientOrderID) (int, error) {
	order, ok := l.orders[id]
	if !ok {
		return 0, fmt.Errorf("order %q: %w", id, ErrFillUnknownOrder)
	}
	return order.fills, nil
}

// Fees reports what the venue charged for a known order's executions, one
// entry per asset actually charged, sorted by asset.
//
// Sorted rather than a map, because its consumer is §7.5's balance arithmetic
// and its tests: a map hands every caller an iteration-order coin flip, which
// is this repo's recorded shape for a defect that reproduces only sometimes.
// An order with no fees (or only zero fees) reports an empty slice — the same
// fact either way, see Apply.
func (l *Ledger) Fees(id ids.ClientOrderID) ([]fixedpoint.Money, error) {
	order, ok := l.orders[id]
	if !ok {
		return nil, fmt.Errorf("order %q: %w", id, ErrFillUnknownOrder)
	}
	if len(order.fees) == 0 {
		return nil, nil
	}
	fees := make([]fixedpoint.Money, 0, len(order.fees))
	for _, fee := range order.fees {
		fees = append(fees, fee)
	}
	sort.Slice(fees, func(a, b int) bool { return fees[a].Asset() < fees[b].Asset() })
	return fees, nil
}

// Knows reports whether the ledger holds the order.
//
// Separate from the accessors above rather than inferred from their errors,
// because §6 step 10's recovery genuinely asks this question — "auto-recovers
// known journaled client IDs; only genuinely foreign orders … require operator
// clearance" — and a caller answering it by discarding an error is a caller that
// will one day discard a different one.
func (l *Ledger) Knows(id ids.ClientOrderID) bool {
	_, ok := l.orders[id]
	return ok
}
