package ports

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/domain"
)

// OrderPlacer is §4's OrderGateway narrowed to its first method.
//
// One method rather than seven, per §4's own rule that contexts consume
// "segregated capability interfaces". The dispatch path submits; it does not
// cancel, query or subscribe, and a use case declaring dependencies it never
// calls is one §3.7's service split would have to carry across a process
// boundary for nothing. It also keeps the test doubles honest — a fake that must
// implement OrderByClientID to be handed to the dispatcher is a fake asserting a
// relationship that does not exist.
type OrderPlacer interface {
	// Place submits the order to the venue EXACTLY ONCE.
	//
	// §4: "NON-idempotent: submit once, NO transport retry; ClientOrderID is a
	// recovery handle, NOT a venue idempotency key". The submit-once rule binds
	// the adapter as well as the caller: an implementation that retries inside
	// this method breaks §5.3 just as thoroughly as a caller that calls it twice,
	// and the caller cannot see it happen.
	//
	// # The error must carry an apperror.Effect, and this is the seam that
	// # decides it
	//
	// The adapter is the only layer that knows whether the request left the
	// socket, and §5.3 forbids the OMS from re-deriving that from the cause —
	// "never infer 'not sent'". So a failure here is expected to be an
	// *apperror.Error carrying EffectNotApplied, EffectUnknown or EffectApplied,
	// and the dispatcher classifies the order's resulting state from it.
	//
	// An unclassified error is safe rather than sharp: apperror.EffectOf answers
	// EffectUnknown for anything it does not recognise, which sends the order to
	// SubmissionUnknown and the OMS to the venue to ask. That is the conservative
	// answer, and it is the one an adapter that forgot gets.
	//
	// # The Urgency argument, and why the order cannot supply it
	//
	// [SPEC EDIT LANDED — v0.9.32] §4's signature was Place(Order) (Ack, error),
	// with no second argument. It could not stay that way, and §7.6's
	// shed classes have NO CARRIER anywhere in this system: the class of an
	// order is a property of the §5.3 COMMAND that produced it — OpenTrade
	// creates exposure, ExposureReduction and EmergencyClosePackage remove it —
	// and neither domain.Order nor ports.OrderRecord holds which command that
	// was. The adapter is the layer that must state it and the only layer with
	// no way to know it.
	//
	// Defaulting is not available, because the two defaults are not equally
	// wrong. Opening puts the order under §7.6's ~70% back-off ceiling, so a
	// FLATTEN defaulted to Opening is shed during exactly the incident the
	// reserve exists for; Safety puts it at capacity, so ordinary trading
	// spends the reserve and needs a second coincidence before it hurts. A
	// silent default here would be the first kind.
	//
	// The alternative considered and not taken: a field on domain.Order naming
	// its originating command. It is the better long-term shape — the same fact
	// also decides §5.3's `stop-opening` exemption, which likewise has no
	// carrier — and it is a §5.1 aggregate change that reaches the journal
	// codec, the planner and recovery. Landed as §4's Place signature, §5.3's
	// command→class table and §7.6's carrier paragraph; the rejected field is
	// recorded there too, so a later slice reopens it deliberately.
	Place(o domain.Order, u Urgency) (Ack, error)
}

// Urgency is §7.6's shed class, narrowed to the two values an ORDER can carry.
//
// # Two members, not three, and the missing one is the point
//
// §7.6 declares Opening, Routine and Safety. Routine is "the reads that keep the
// risk view live — marks, books, account posture", which no order is under any
// §5.3 command. Declaring it here would put a value in the type that no caller
// can correctly choose and that maps to neither ceiling: Routine sits BETWEEN
// the two, so an adapter translating it must round, and both roundings are the
// silent version of a decision this type exists to make explicit.
//
// So the narrowing is the ACL doing its job — platform/ratelimit.Priority is the
// rate limiter's vocabulary and has all three, and the translation happens in
// the driven adapter, which is the layer entitled to speak both.
type Urgency string

const (
	// UrgencyOpening is §7.6's Opening: "requests that create new exposure —
	// the same act §5.3 calls stop-opening". A cash-and-carry entry leg.
	UrgencyOpening Urgency = "opening"

	// UrgencySafety is §7.6's Safety: §6.7 ambiguity resolution, §7.1's margin
	// top-up, a flatten. It spends the reserve band, so it is the class that
	// must not be reached for by default.
	UrgencySafety Urgency = "safety"
)

// Validate reports whether u is one of the declared classes.
func (u Urgency) Validate() error {
	switch u {
	case UrgencyOpening, UrgencySafety:
		return nil
	}
	return fmt.Errorf("%q: %w", string(u), ErrUrgencyInvalid)
}

func (u Urgency) String() string { return string(u) }

// Ack is what a venue returns from an accepted Place.
//
// # The dispatcher does not branch on its contents, and that is deliberate
//
// It is the ARRIVAL of an ack that moves the order to Submitted; §5.3 defines
// that state as "the single Place returned a transport result; awaiting/parsing
// venue ack", so the parse is downstream of the transition rather than part of
// it. Both venues do return an order status in the Place response — a marketable
// IOC can come back FILLED — and mapping that directly into Filled here is the
// tempting shortcut. It is refused because the private order stream reports the
// same event, so the two paths would be two authorities over one order's state,
// racing, with no rule for which wins when they disagree. SM §1's Submitted →
// Acknowledged → Filled path costs nothing and has one authority.
//
// So this type carries what reconciliation needs and nothing the state machine
// reads.
type Ack struct {
	// VenueOrderID is the venue's own identifier for the order.
	//
	// # It may be empty, and on ONE venue that costs more than it looks
	//
	// This field once carried the claim that "nothing in this system REQUIRES
	// the venue's id", justified by a parenthetical naming Bybit. The claim was
	// generalized from the one venue where it holds, and §4 now records the
	// correction (Profiles §8). Bybit's GET /v5/execution/list returns
	// orderLinkId on every row, so a fill there is attributable by our own
	// handle. Binance's trade lists return NEITHER a clientOrderId NOR an
	// origClientOrderId — not on /fapi/v1/userTrades and not on
	// /api/v3/myTrades — so on Binance this number is the ONLY join key between
	// a swept execution and one of our orders.
	//
	// What closes the gap is a second venue fact rather than anything here:
	// Binance retains an order WITH FILLS for 90 days, so OrderByClientID can
	// recover the orderId for any fill worth attributing. That makes Fills and
	// OrderByClientID a COMPOSITION where §4's signatures suggest independence,
	// and it leaves the sweep useless alone in exactly the scenario it exists
	// for — a lost ack, where §6.7 runs precisely because the Place response was
	// never read.
	//
	// So it is carried whenever the venue supplies it, and it is still not
	// REQUIRED to be non-empty: a venue that returns none leaves it empty, and
	// §5.3's recovery path keys on ClientOrderID. The reason to carry it has not
	// changed — an operator reconciling against a venue UI has only this number
	// to search by, and recovering it later means a query the ack already
	// answered.
	VenueOrderID string
}
