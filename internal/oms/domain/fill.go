package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// FillKey is §5.3's single normative dedupe identity:
// UNIQUE(account, venue, product_category, symbol, execution_id).
//
// # Four spec columns, three fields, and the fourth is a checksum
//
// The spec names venue, product_category and symbol separately; this holds one
// ids.VenueInstrument, which carries those three AND a ContractType the key does
// not name. A wider key is a WEAKER uniqueness constraint — it admits rows the
// narrow one would collide — so the extra field looks like a dedupe hole.
//
// It is not, and the reason is a property of the type rather than of this
// package: VenueInstrument.Validate refuses a Contract that disagrees with its
// Product (ProductLinearPerp implies ContractLinear), so for a validated
// instrument the fourth field is functionally determined by the second and
// carries no distinguishing power. ids' own doc calls this "a checksum".
//
// The clause that matters is FOR A VALIDATED INSTRUMENT. An unvalidated
// {linear_perp, inverse} is a distinct map key from the {linear_perp, linear}
// naming the same real instrument, and the same execution would then apply
// twice. That is why Key validates rather than assembling blindly, and why the
// error is worth more than the convenience of an infallible constructor.
type FillKey struct {
	// Account is §5.1's venue subaccount, and the spec puts it first in the key
	// because one deployment holds several. It is a plain string here rather
	// than a typed id because §5.2 sources it from per-venue configuration
	// (`subaccount id`) and nothing in this repo has yet earned a type for it.
	Account string

	// Instrument is the canonical §5.1 identity — three of the key's columns,
	// plus the checksum discussed above.
	Instrument ids.VenueInstrument

	// ExecutionID is the venue's immutable fill id: Binance tradeId, Bybit
	// execId. §5.3 makes it "unique only WITHIN an instrument", which is the
	// whole reason the other two fields are here — the §9 chaos list carries
	// "identical spot/perp execution IDs" as a case to survive, and on one venue
	// those namespaces genuinely do collide.
	ExecutionID string
}

// Fill is §5.1's immutable execution record.
//
// # It is immutable in the strong sense: nothing here amends one
//
// §5.1 says corrections and reversals are "explicit events, never in-place
// edits", and this type has no methods that change it. A venue correcting a fill
// sends another fill; the ledger holds both.
//
// # FeeAsset is absent, and that is a deviation worth naming
//
// [SPEC EDIT OWED] §5.1's field list is `Fill{… Fee Money, FeeAsset, Ts}`, with
// the asset as its own column. That list predates the money kernel. A
// fixedpoint.Money already carries its asset tag as a runtime-checked field —
// §3.6 rule 4's own design — so a separate FeeAsset would be a second copy of a
// fact the first field states, and this repo has already paid for that shape
// once (ErrDeltaSeqDisagrees exists because a payload restated its envelope).
// The failure it invites is specific: fee attribution reads one field and the
// tax export reads the other, and a Binance fill paid in BNB against a USDT
// notional reconciles to two different numbers with nothing failing.
//
// Owed to core §5.1: drop FeeAsset from the Fill field list, since Fee Money
// carries it.
type Fill struct {
	// ExecutionID, Account and Instrument are the dedupe identity — see Key.
	ExecutionID string
	Account     string
	Instrument  ids.VenueInstrument

	// Order is the OMS-minted ClientOrderID this execution belongs to. §5.3
	// calls it "a recovery correlation handle, not a venue idempotency key", and
	// correlation is exactly the job it does here: it is how a venue execution
	// finds the order whose cumulative quantity it moves.
	Order ids.ClientOrderID

	// Quantity is the executed size, strictly positive. A fill's direction comes
	// from its ORDER's side, not from a signed quantity here: §5.1 gives Order a
	// Side and gives Fill none, and a signed fill quantity would be a second
	// place for the direction to be stated — with a sign error producing a
	// perfectly valid ledger that reports a short leg as long.
	Quantity fixedpoint.Quantity

	// Price is the execution price. Carried and validated; nothing in the ledger
	// computes with it yet. §6 step 9's realized-PnL decomposition is what reads
	// it, and that is a later slice.
	Price fixedpoint.Price

	// Fee is what the venue charged, in whatever asset it charged in. It may be
	// NEGATIVE — a maker rebate is a real fill outcome on both venues — so no
	// sign check is made here, and code that treats fees as costs must subtract
	// rather than take an absolute value.
	Fee fixedpoint.Money

	// Ts is the venue's execution timestamp. A field on a record, not a clock
	// read, so §3.6 rule 5 has nothing to say about it: the ban is on this layer
	// ASKING what time it is, and this is the venue telling us when something
	// happened.
	Ts time.Time
}

// Key returns the fill's dedupe identity, refusing one that cannot be a key.
//
// The validation is not ceremony. Every field here is load-bearing for
// uniqueness, and each has a distinct way of failing quietly: an unset
// ExecutionID collapses every unidentified fill on an instrument into one key,
// so the second is dropped as a duplicate; an unset Account merges two
// subaccounts that share a venue's per-instrument execution-ID sequence; an
// unvalidated Instrument splits one real instrument across two keys, so the same
// execution applies twice.
func (f Fill) Key() (FillKey, error) {
	if f.ExecutionID == "" {
		return FillKey{}, ErrExecutionIDUnset
	}
	if f.Account == "" {
		return FillKey{}, fmt.Errorf("execution %q: %w", f.ExecutionID, ErrAccountUnset)
	}
	if err := f.Instrument.Validate(); err != nil {
		return FillKey{}, fmt.Errorf("execution %q instrument: %w", f.ExecutionID, err)
	}
	return FillKey{
		Account:     f.Account,
		Instrument:  f.Instrument,
		ExecutionID: f.ExecutionID,
	}, nil
}

// Validate reports whether the fill is one a venue could have sent.
//
// Key's checks are a subset and are re-run through it, so a caller that only
// validates gets the identity rules too — the alternative, two entry points with
// overlapping coverage, is how one of them ends up the weaker check that
// something calls.
func (f Fill) Validate() error {
	if _, err := f.Key(); err != nil {
		return err
	}
	if f.Order == "" {
		return fmt.Errorf("execution %q: %w", f.ExecutionID, ErrOrderUnset)
	}
	if err := f.Quantity.Validate(); err != nil {
		return fmt.Errorf("execution %q quantity: %w", f.ExecutionID, err)
	}
	sign, err := f.Quantity.Sign()
	if err != nil {
		return fmt.Errorf("execution %q quantity: %w", f.ExecutionID, err)
	}
	if sign <= 0 {
		return fmt.Errorf("execution %q is %s: %w", f.ExecutionID, f.Quantity, ErrFillQuantityNotPositive)
	}
	if err := f.Price.Validate(); err != nil {
		return fmt.Errorf("execution %q price: %w", f.ExecutionID, err)
	}
	if err := f.Fee.Validate(); err != nil {
		return fmt.Errorf("execution %q fee: %w", f.ExecutionID, err)
	}
	if f.Ts.IsZero() {
		return fmt.Errorf("execution %q: %w", f.ExecutionID, ErrFillTimestampUnset)
	}
	return nil
}
