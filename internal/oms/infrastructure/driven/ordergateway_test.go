package driven_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// recordingVenue captures the translated request and answers with a fixed
// result. It is the far side of the ACL, which is the only place the
// translation is observable.
type recordingVenue struct {
	ack  venue.OrderAck
	err  error
	seen []venue.OrderRequest
}

func (v *recordingVenue) Place(_ context.Context, r venue.OrderRequest) (venue.OrderAck, error) {
	v.seen = append(v.seen, r)
	return v.ack, v.err
}

func gatewayOrder(t *testing.T) domain.Order {
	t.Helper()

	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: 1, Attempt: 3})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	q, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	p, err := fixedpoint.ParsePrice("BTC", "USDT", "50100.25")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}

	// A SELL on the perp: the flagship's short leg, and the direction that makes
	// a side mapping able to fail. Buy is the value a translator that answered
	// one constant would accidentally agree with.
	return domain.Order{
		ClientOrderID: id,
		Instrument: ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
		},
		Side:        domain.SideSell,
		Type:        domain.OrderTypeLimit,
		TimeInForce: domain.TimeInForceIOC,
		Quantity:    q,
		LimitPrice:  p,
	}
}

func newGateway(t *testing.T, v *recordingVenue) *driven.OrderGateway {
	t.Helper()
	g, err := driven.NewOrderGateway(v)
	if err != nil {
		t.Fatalf("NewOrderGateway: %v", err)
	}
	return g
}

// TestPlaceTranslatesEveryField.
//
// The expectation is written out rather than produced by calling the adapter's
// own converter. A test that built its `want` from the code under test would
// agree with it by construction — including with every mapping reversed.
func TestPlaceTranslatesEveryField(t *testing.T) {
	t.Parallel()

	v := &recordingVenue{ack: venue.OrderAck{VenueOrderID: "V-77"}}
	o := gatewayOrder(t)

	ack, err := newGateway(t, v).Place(o, ports.UrgencyOpening)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if ack.VenueOrderID != "V-77" {
		t.Errorf("Ack.VenueOrderID = %q, want V-77", ack.VenueOrderID)
	}
	if len(v.seen) != 1 {
		t.Fatalf("the venue saw %d requests, want exactly 1 — §5.3 permits one Place per order", len(v.seen))
	}

	want := venue.OrderRequest{
		Instrument:    o.Instrument,
		ClientOrderID: o.ClientOrderID,
		Side:          venue.OrderSideSell,
		Quantity:      o.Quantity,
		LimitPrice:    o.LimitPrice,
		ReduceOnly:    false,
		Urgency:       ratelimit.PriorityOpening,
	}
	if v.seen[0] != want {
		t.Errorf("the venue received %+v, want %+v", v.seen[0], want)
	}
}

// TestPlaceMapsBothSidesAndBothShedClasses.
//
// Both directions of both mappings, in one table, because each is a two-valued
// function and a fixture pinned to one value cannot tell a mapping from a
// constant. The shed class is the one that matters most: §7.6 puts Opening under
// the ~70% back-off ceiling and Safety at capacity, so a translator that
// answered Opening to both would shed a flatten during the incident the reserve
// exists for.
func TestPlaceMapsBothSidesAndBothShedClasses(t *testing.T) {
	t.Parallel()

	sides := map[domain.Side]venue.OrderSide{
		domain.SideBuy:  venue.OrderSideBuy,
		domain.SideSell: venue.OrderSideSell,
	}
	classes := map[ports.Urgency]ratelimit.Priority{
		ports.UrgencyOpening: ratelimit.PriorityOpening,
		ports.UrgencySafety:  ratelimit.PrioritySafety,
	}

	for side, wantSide := range sides {
		for urgency, wantPriority := range classes {
			t.Run(string(side)+"/"+string(urgency), func(t *testing.T) {
				t.Parallel()

				v := &recordingVenue{}
				o := gatewayOrder(t)
				o.Side = side

				if _, err := newGateway(t, v).Place(o, urgency); err != nil {
					t.Fatalf("Place: %v", err)
				}
				if got := v.seen[0].Side; got != wantSide {
					t.Errorf("Side = %q, want %q", got, wantSide)
				}
				if got := v.seen[0].Urgency; got != wantPriority {
					t.Errorf("Urgency = %v, want %v", got, wantPriority)
				}
			})
		}
	}
}

// TestPlaceRefusesWhatSixStepSixForbidsSending.
//
// venue.OrderRequest cannot EXPRESS a market order — it has no OrderType field —
// so this adapter is the layer that has to refuse one, and the refusal must
// happen before anything reaches the venue.
func TestPlaceRefusesWhatSixStepSixForbidsSending(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*domain.Order){
		"a market order": func(o *domain.Order) {
			o.Type = domain.OrderTypeMarket
			o.LimitPrice = fixedpoint.Price{}
		},
		"a resting GTC limit": func(o *domain.Order) { o.TimeInForce = domain.TimeInForceGTC },
		"a FOK limit":         func(o *domain.Order) { o.TimeInForce = domain.TimeInForceFOK },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			v := &recordingVenue{}
			o := gatewayOrder(t)
			mutate(&o)

			_, err := newGateway(t, v).Place(o, ports.UrgencyOpening)
			if !errors.Is(err, driven.ErrNotSendableAtVenue) {
				t.Fatalf("Place = %v, want ErrNotSendableAtVenue", err)
			}
			if got := apperror.EffectOf(err); got != apperror.EffectNotApplied {
				t.Errorf("Effect = %v, want EffectNotApplied — nothing was translated, let alone sent", got)
			}
			if len(v.seen) != 0 {
				t.Errorf("the venue saw %+v, want nothing", v.seen)
			}
		})
	}
}

// TestPlaceRefusesAnUnsetShedClass.
//
// Urgency is a string, so the zero value compiles. It must not be readable as
// either class: one would shed a flatten and the other would spend the reserve.
func TestPlaceRefusesAnUnsetShedClass(t *testing.T) {
	t.Parallel()

	v := &recordingVenue{}
	_, err := newGateway(t, v).Place(gatewayOrder(t), "")
	if !errors.Is(err, ports.ErrUrgencyInvalid) {
		t.Fatalf("Place = %v, want ErrUrgencyInvalid", err)
	}
	if len(v.seen) != 0 {
		t.Errorf("the venue saw %+v, want nothing", v.seen)
	}
}

// TestPlacePassesTheVenuesEffectThroughUNCHANGED is the §5.3 rule this adapter
// must not break.
//
// engine/venue is the only layer that knows whether the request left the socket.
// Re-wrapping its error here — with apperror.Precondition, say — would overwrite
// EffectUnknown with EffectNotApplied, which is exactly the inference §5.3
// forbids: it records "no order exists" for an order the venue may be holding.
func TestPlacePassesTheVenuesEffectThroughUNCHANGED(t *testing.T) {
	t.Parallel()

	for name, effect := range map[string]apperror.Effect{
		"unknown":     apperror.EffectUnknown,
		"applied":     apperror.EffectApplied,
		"not applied": apperror.EffectNotApplied,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			v := &recordingVenue{err: apperror.Transport("venue.Place", effect, errors.New("boom"))}
			_, err := newGateway(t, v).Place(gatewayOrder(t), ports.UrgencySafety)
			if got := apperror.EffectOf(err); got != effect {
				t.Errorf("Effect = %v, want %v — the adapter re-classified the venue's own answer", got, effect)
			}
		})
	}
}

// TestNewOrderGatewayRefusesNothingBehindIt.
func TestNewOrderGatewayRefusesNothingBehindIt(t *testing.T) {
	t.Parallel()

	if _, err := driven.NewOrderGateway(nil); !errors.Is(err, driven.ErrVenueOrderPlacerMissing) {
		t.Fatalf("NewOrderGateway(nil) = %v, want ErrVenueOrderPlacerMissing", err)
	}
	// Positive control: a constructor that refused everything would pass above.
	if _, err := driven.NewOrderGateway(&recordingVenue{}); err != nil {
		t.Fatalf("NewOrderGateway(client) = %v, want nil", err)
	}
}
