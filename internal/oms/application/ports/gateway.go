package ports

import "github.com/giiorg/arb-lab/internal/oms/domain"

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
	Place(domain.Order) (Ack, error)
}

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
	// It may legitimately be empty: §5.3's recovery path keys on ClientOrderID
	// (Bybit resolves by orderLinkId, and its order-history fallback is what
	// makes a filled order always recoverable), so nothing in this system
	// REQUIRES the venue's id. It is carried because an operator reconciling
	// against a venue UI has only this number to search by, and recovering it
	// later means a query the ack already answered.
	VenueOrderID string
}
