package domain

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Side is §5.1's order direction.
//
// It is `type X string` under §3.6 rule 8 for the same reason OrderState is: it
// crosses the write-ahead journal, and §6 step 9 decomposes realized PnL per leg
// from stored fills against the side of the order that produced them. Under an
// integer encoding, inserting a constant renumbers every stored row and the
// corruption is a hedge read as a doubling rather than a crash.
type Side string

const (
	// SideBuy is the direction that acquires the base asset — a cash-and-carry
	// trade's spot leg.
	SideBuy Side = "buy"

	// SideSell is the direction that disposes of it, and — on a perpetual — the
	// direction that opens the short this strategy collects funding on.
	SideSell Side = "sell"
)

// Validate reports whether s is one of the declared directions.
//
// The empty side is refused by the same path as an unknown one, unlike
// OrderState, which separates them. The asymmetry is deliberate: OrderState's
// unset value names a hydration that missed a column, and a state has twelve
// members so "which column" is a real question. A side has two, so a blank one
// is a blank one, and a second sentinel would be a distinction no caller could
// act on differently.
func (s Side) Validate() error {
	switch s {
	case SideBuy, SideSell:
		return nil
	}
	return fmt.Errorf("%q: %w", string(s), ErrSideInvalid)
}

func (s Side) String() string { return string(s) }

// OrderType is §5.1's order type.
//
// # Market exists in the type even though this system never sends one
//
// §6 step 6 fixes the execution pattern as "parallel marketable-IOC limit orders
// (never raw MARKET)", so a market order is one this OMS does not place. It is
// declared anyway, because the type also has to hold orders we did not create:
// §4's OpenOrders returns every order on the account, and §6 step 10 routes the
// foreign ones to operator clearance rather than discarding them. A domain that
// could not REPRESENT a foreign market order would force the adapter to either
// drop it — losing the exposure it represents from the net-delta invariant — or
// mislabel it as a limit order with no price.
//
// The rule that we never send one is therefore enforced where sending happens,
// not here. See the application layer's dispatch path.
type OrderType string

const (
	// OrderTypeLimit carries a price the venue may not cross.
	OrderTypeLimit OrderType = "limit"

	// OrderTypeMarket carries none. §6 step 6 forbids this system from placing
	// one; it is representable because other participants in this account can.
	OrderTypeMarket OrderType = "market"
)

// Validate reports whether t is one of the declared types.
func (t OrderType) Validate() error {
	switch t {
	case OrderTypeLimit, OrderTypeMarket:
		return nil
	}
	return fmt.Errorf("%q: %w", string(t), ErrOrderTypeInvalid)
}

func (t OrderType) String() string { return string(t) }

// TimeInForce is §5.1's TIF.
//
// The member list is what the two venues accept and what SM §1's lifecycle has
// to explain terminations for, which are not the same question — GTD is here
// because the venue-initiated termination this package owes SM §1 names "GTD
// expiry" as one of its four causes, and a cause the state machine names must be
// a value the model can hold.
type TimeInForce string

const (
	// TimeInForceIOC is immediate-or-cancel: whatever crosses now executes and
	// the venue cancels the rest itself. It is the ONLY value §6 step 6 permits
	// this system to send, and SM §2 step 1 states its consequence — "an IOC is
	// immediately terminal at the venue" — which is the whole reason the
	// venue-initiated Canceled edge has to exist.
	TimeInForceIOC TimeInForce = "ioc"

	// TimeInForceGTC rests on the book until canceled.
	TimeInForceGTC TimeInForce = "gtc"

	// TimeInForceFOK executes in full or not at all.
	TimeInForceFOK TimeInForce = "fok"

	// TimeInForceGTD rests until a venue-side expiry instant. The expiry itself
	// is not modelled here — nothing in this slice reads it, and a field only
	// the venue enforces would be a second copy of the venue's answer.
	TimeInForceGTD TimeInForce = "gtd"
)

// Validate reports whether tif is one of the declared values.
func (tif TimeInForce) Validate() error {
	switch tif {
	case TimeInForceIOC, TimeInForceGTC, TimeInForceFOK, TimeInForceGTD:
		return nil
	}
	return fmt.Errorf("%q: %w", string(tif), ErrTimeInForceInvalid)
}

func (tif TimeInForce) String() string { return string(tif) }

// Order is §5.1's order, minus the two things that change.
//
// # State and fills are absent, and their absence is the same decision twice
//
// §5.1's field list ends `…, State`. This struct has no State field and no fill
// list, because this package already holds both mechanisms separately —
// OrderState with CanTransition over SM §1's edges, and Ledger over §5.3's
// cumulative executions — and §5.3's own design is that the two are ORTHOGONAL:
// "order state tracks submission/finality; the fill ledger tracks realized
// quantity". A struct carrying an Order's identity alongside a mutable State is
// a second home for a fact CanTransition governs, and the two disagree the first
// time something writes one without the other.
//
// What this type is, then, is the IMMUTABLE half: everything about an order that
// was fixed when it was planned and that no venue event ever changes. That is
// exactly the payload §6 step 5 fsyncs to the journal before any submission, and
// exactly what §4's Place takes. The aggregate that binds an Order to its state
// and its fills is a later slice; when it lands, it composes this rather than
// restating it.
//
// # PositionSide/Idx is absent, and that is a layering call rather than an
// # omission
//
// §5.1 lists it. §4.1 pins the account to ONE-WAY position mode and §7.5 makes a
// Hedge position mode a refuse-to-trade condition asserted at boot and before
// every trade, so the field's legal domain here has exactly one member. What the
// venues actually need is a wire field whose spelling differs between them
// (Binance positionSide, Bybit positionIdx), which is an adapter's job and is
// derivable from the pinned mode without being carried. Carried instead, it
// would be a field with one legal value that a construction site can set wrong
// and that Validate could only check against a constant.
type Order struct {
	// ClientOrderID is §5.3's recovery correlation handle.
	//
	// Validate requires only that it be present, NOT that it parse as one of
	// ours — and the permissiveness is load-bearing. ids.ParseClientOrderID is
	// deliberately strict so that a Binance "autoclose-…" cannot be read as a
	// trade we placed, and §4's OpenOrders returns exactly such ids for the
	// foreign orders §6 step 10 sends to operator clearance. A domain type that
	// refused them could not hold the orders the reconciler exists to find.
	//
	// The stronger check — that an id we are about to SUBMIT is one we minted —
	// belongs at the dispatch boundary, where getting it wrong means an order
	// whose recovery handle cannot be correlated after a crash.
	ClientOrderID ids.ClientOrderID

	// Instrument is §5.1's canonical identity. Spot BTCUSDT and perp BTCUSDT are
	// different instruments with independent order and execution-ID namespaces,
	// and a cash-and-carry trade holds one order on each at the same instant.
	Instrument ids.VenueInstrument

	// Side is the direction. It is the ONLY statement of direction on an order
	// and its fills: Fill carries an unsigned quantity precisely so that the sign
	// lives in one place (see Fill.Quantity).
	Side Side

	// Type and TimeInForce are the execution instructions. §6 step 6 constrains
	// which combination this system may send; both members are representable
	// because recovery holds orders this system did not send.
	Type        OrderType
	TimeInForce TimeInForce

	// Quantity is the ordered size, strictly positive. Direction is Side's job.
	Quantity fixedpoint.Quantity

	// LimitPrice is the price a limit order may not cross, and MUST be the zero
	// Price on a market order — see Validate, where the pairing is checked in
	// both directions.
	LimitPrice fixedpoint.Price

	// ReduceOnly is §5.3's flag, and the spec's rule about it is one this type
	// cannot enforce: it may be set "only when the order reduces an existing
	// position on that instrument — never merely because it reduces package
	// delta". That is a claim about a position this package does not hold, so it
	// is checked where the reduction is sized, not here. Recorded so the absence
	// reads as scoped rather than forgotten.
	ReduceOnly bool
}

// Validate reports whether the order is well-formed.
//
// It is well-formedness only, and the boundary is worth stating because two
// nearby questions are deliberately NOT asked here:
//
//   - Whether the order matches §6 step 6's execution pattern (marketable-IOC
//     limit, never raw MARKET). That is a rule about what this system SENDS, and
//     this type must also hold what other participants sent.
//   - Whether Quantity's asset and LimitPrice's asset pair agree with the
//     instrument. VenueInstrument carries a Symbol string rather than a
//     base/quote asset pair, so there is nothing here to compare against; the
//     check becomes possible when SymbolSpec (§5.1) lands and belongs with it.
//     Stated rather than left as an apparent oversight, because a Validate that
//     looks total is one nobody re-reads.
func (o Order) Validate() error {
	if o.ClientOrderID == "" {
		return ErrOrderUnset
	}
	if err := o.Instrument.Validate(); err != nil {
		return fmt.Errorf("order %q instrument: %w", o.ClientOrderID, err)
	}
	if err := o.Side.Validate(); err != nil {
		return fmt.Errorf("order %q side: %w", o.ClientOrderID, err)
	}
	if err := o.Type.Validate(); err != nil {
		return fmt.Errorf("order %q type: %w", o.ClientOrderID, err)
	}
	if err := o.TimeInForce.Validate(); err != nil {
		return fmt.Errorf("order %q time in force: %w", o.ClientOrderID, err)
	}
	if err := o.Quantity.Validate(); err != nil {
		return fmt.Errorf("order %q quantity: %w", o.ClientOrderID, err)
	}
	sign, err := o.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("order %q quantity: %w", o.ClientOrderID, err)
	}
	if sign <= 0 {
		// Zero is the reading that would otherwise pass, and Ledger.Register
		// refuses it for the §6.7 reason: an order for nothing has no remaining
		// executable quantity from the instant it exists, so the classifier reads
		// its residual as determined while the venue holds a live order whose size
		// nobody recorded. Checked here as well as there because these are two
		// entry points to the same fact and only one of them is on the dispatch
		// path.
		return fmt.Errorf("order %q is for %s: %w", o.ClientOrderID, o.Quantity, ErrOrderedQuantityNotPositive)
	}
	return o.validatePrice()
}

// validatePrice enforces the price/type pairing in BOTH directions.
//
// The direction that gets written is "a limit order needs a price". The one that
// does not is "a market order must not carry one", and it is the more expensive
// omission: §6 step 4 budgets the naked-leg gap from a price this order is
// supposed to be bounded by, so a market order carrying a limit price presents a
// bound the venue will ignore. The order fills through it, the slippage budget
// reconciles against a number that never bound anything, and nothing fails.
//
// A market order's LimitPrice must be the ZERO Price rather than merely
// unchecked, because fixedpoint's zero value is unusable by construction (§3.6
// rule 4's design) — so "absent" has exactly one spelling and a caller cannot
// express it two ways.
func (o Order) validatePrice() error {
	switch o.Type {
	case OrderTypeLimit:
		if err := o.LimitPrice.Validate(); err != nil {
			return fmt.Errorf("order %q limit price: %w", o.ClientOrderID, err)
		}
		return nil
	case OrderTypeMarket:
		if o.LimitPrice != (fixedpoint.Price{}) {
			return fmt.Errorf("order %q is a market order carrying limit price %s: %w",
				o.ClientOrderID, o.LimitPrice, ErrMarketOrderHasLimitPrice)
		}
		return nil
	}
	// Reachable only from a DECLARED type with no case above, which is §3.6 rule
	// 10's failure arriving at runtime because something removed the gate. It is
	// unconditional rather than a trailing `return nil` for the reason this
	// package established in IsTerminal: a trailing return puts one group's
	// members beyond the reach of any test, since deleting a name from the case
	// yields the answer the fallthrough already gave.
	return fmt.Errorf("order %q: %q: %w", o.ClientOrderID, string(o.Type), ErrOrderTypeUnhandled)
}
