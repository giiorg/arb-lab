package ids

import "fmt"

// TradeID identifies a trade — the two-leg package §5.1 makes the unit of
// ownership, not an individual order.
//
// The UUID is unexported so that the only ways to hold one are minting
// (Generator.NewTradeID) and parsing (ParseTradeID). The zero value is the Nil
// UUID and fails Validate, so a struct field nobody populated is caught rather
// than silently addressing whatever trade shares its id — which, since the Nil
// UUID is a constant, would be all of them.
type TradeID struct{ u UUID }

func (t TradeID) Validate() error { return t.u.Validate() }
func (t TradeID) String() string  { return t.u.String() }

// UUID exposes the underlying value for persistence and for the ClientOrderID
// derivation. It is a copy — UUID is an array, not a slice — so a caller cannot
// reach back through it and mutate the id.
func (t TradeID) UUID() UUID { return t.u }

// ParseTradeID reads the canonical UUID text and rejects anything that is not a
// well-formed v7. Round-trips with String.
func ParseTradeID(s string) (TradeID, error) {
	u, err := ParseUUID(s)
	if err != nil {
		return TradeID{}, err
	}
	if err := u.Validate(); err != nil {
		return TradeID{}, err
	}
	return TradeID{u: u}, nil
}

// IntentID identifies a strategy's emitted intent. §5.1 pairs it with a Revision
// and a ValidUntil so that a strategy re-emitting the same signal on successive
// snapshots is explicitly idempotent and a stale intent is dropped rather than
// executed late.
//
// Worth reading §5.1 carefully before relying on it: the IntentID is NOT the
// primary race guard. The deterministic command sequencer (§3.4) and the
// exclusive per-hedge-group lease (§5.1) are what serialize commands; this is
// what makes re-emission distinguishable from a second decision.
type IntentID struct{ u UUID }

func (i IntentID) Validate() error { return i.u.Validate() }
func (i IntentID) String() string  { return i.u.String() }
func (i IntentID) UUID() UUID      { return i.u }

func ParseIntentID(s string) (IntentID, error) {
	u, err := ParseUUID(s)
	if err != nil {
		return IntentID{}, err
	}
	if err := u.Validate(); err != nil {
		return IntentID{}, err
	}
	return IntentID{u: u}, nil
}

// ExecutionID is the venue's own immutable fill identifier — Binance tradeId,
// Bybit execId (§5.3).
//
// It is the odd one out here: RECEIVED, never minted, so it is an opaque string
// with no structure this package may assume. It lives in this package anyway
// because it is an identity and because of the caveat below, which is easier to
// enforce next to the type than in prose somewhere else.
//
// # It is unique only WITHIN an instrument
//
// §5.3 is explicit, and the dedup key it mandates —
// UNIQUE(account, venue, product_category, symbol, execution_id) — has five
// columns for that reason: spot and perp id namespaces are independent, so the
// same ExecutionID can name two different fills. Deduping on this value alone
// would silently drop a real fill, and a dropped fill is a position the ledger
// does not know about.
//
// Which is why there is no Equal method and no comparison helper: any such
// helper would be a comparison of the bare value, and the bare value is not an
// identity. Comparison happens in the repository, against the full key.
type ExecutionID string

// ErrExecutionIDUnset reports an empty ExecutionID. It has no other validation —
// the venue's format is the venue's business, and inventing a shape here would
// reject a legitimate fill the day a venue changes one.
var ErrExecutionIDUnset = fmt.Errorf("ids: execution id unset")

func (e ExecutionID) Validate() error {
	if e == "" {
		return ErrExecutionIDUnset
	}
	return nil
}

func (e ExecutionID) String() string { return string(e) }
