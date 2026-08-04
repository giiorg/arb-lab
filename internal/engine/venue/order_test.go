package venue_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks §4's Place, venue-side. The encode/decode halves are pure and
// are checked against the venues' own documented shapes (Profiles §7); the
// routing half needs a socket and gets two of them, because the claim it makes
// is that a spot order reaches a DIFFERENT HOST.

const (
	testBinance ids.Venue = "binance"
	testBybit   ids.Venue = "bybit"
)

// mintedID is a real ClientOrderID from ids, not a hand-written string.
//
// It is the fixture that closes ids.ClientOrderID's own recorded assumption —
// "the 36-character budget and the [A-Za-z0-9_-] character set this format fits
// inside are NOT recorded in venue-profiles.md … Confirm before milestone 3
// places a real order." Using the real derivation here means the two venues'
// published rules are checked against the ids this system actually mints,
// rather than against a string a test author chose to be safe.
func mintedID(t *testing.T) ids.ClientOrderID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	// Leg 1, attempt 3: neither field at zero, so a codec that dropped or
	// transposed one would change the string.
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: 1, Attempt: 3})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	return id
}

func perpInstrument(v ids.Venue) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: v, Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func spotInstrument(v ids.Venue) ids.VenueInstrument {
	return ids.VenueInstrument{Venue: v, Product: ids.ProductSpot, Symbol: "BTCUSDT"}
}

// perpRequest is a SELL, which is the direction the flagship's perp leg takes
// and — the reason it is chosen here — the one that makes a side mapping able to
// fail. Every codec under test renders buy and sell differently, so a fixture
// that only ever bought would agree with a codec that answered "BUY" always.
func perpRequest(t *testing.T, v ids.Venue) venue.OrderRequest {
	t.Helper()
	return venue.OrderRequest{
		Instrument:    perpInstrument(v),
		ClientOrderID: mintedID(t),
		Side:          venue.OrderSideSell,
		Quantity:      orderQty(t, "0.5"),
		LimitPrice:    orderPx(t, "50100.25"),
		Urgency:       ratelimit.PriorityOpening,
	}
}

func spotRequest(t *testing.T, v ids.Venue) venue.OrderRequest {
	t.Helper()
	r := perpRequest(t, v)
	r.Instrument = spotInstrument(v)
	r.Side = venue.OrderSideBuy
	return r
}

func orderQty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", s)
	if err != nil {
		t.Fatalf("ParseQuantity(%q): %v", s, err)
	}
	return q
}

func orderPx(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

// ---------------------------------------------------------------------------
// The request's own rules
// ---------------------------------------------------------------------------

func TestOrderRequestValidate(t *testing.T) {
	t.Parallel()

	// Positive control first: without it, a Validate that refused everything
	// would score a clean sweep on the table below.
	if err := perpRequest(t, testBinance).Validate(); err != nil {
		t.Fatalf("a complete perp request was refused: %v", err)
	}
	if err := spotRequest(t, testBinance).Validate(); err != nil {
		t.Fatalf("a complete spot request was refused: %v", err)
	}

	cases := map[string]struct {
		mutate func(*venue.OrderRequest)
		want   error
	}{
		// The rule with a direction: dropping the flag instead of refusing
		// would send an order that can INCREASE a holding when the caller
		// asked for one that cannot.
		"reduce-only on spot": {
			func(r *venue.OrderRequest) { r.Instrument = spotInstrument(testBinance); r.ReduceOnly = true },
			venue.ErrReduceOnlyOnSpot,
		},
		// §7.6's Routine class is read traffic. It sits BETWEEN the two
		// ceilings, so both roundings are a real decision and neither may be
		// taken silently.
		"the routine shed class": {
			func(r *venue.OrderRequest) { r.Urgency = ratelimit.PriorityRoutine },
			venue.ErrOrderUrgencyRoutine,
		},
		"an unset shed class": {
			func(r *venue.OrderRequest) { r.Urgency = ratelimit.PriorityUnspecified },
			ratelimit.ErrPriorityInvalid,
		},
		"an unset client order id": {
			func(r *venue.OrderRequest) { r.ClientOrderID = "" },
			venue.ErrOrderClientIDUnset,
		},
		"a zero quantity": {
			func(r *venue.OrderRequest) { r.Quantity = orderQty(t, "0") },
			venue.ErrOrderQuantityNotPositive,
		},
		"an unset limit price": {
			func(r *venue.OrderRequest) { r.LimitPrice = fixedpoint.Price{} },
			fixedpoint.ErrAssetUnset,
		},
		"an empty side": {
			func(r *venue.OrderRequest) { r.Side = "" },
			venue.ErrOrderSideInvalid,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := perpRequest(t, testBinance)
			tc.mutate(&r)
			if err := r.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestReduceOnlyIsAllowedOnAPerp is the positive control for the spot refusal
// above: without it, a Validate that refused reduce-only everywhere would pass.
func TestReduceOnlyIsAllowedOnAPerp(t *testing.T) {
	t.Parallel()

	r := perpRequest(t, testBinance)
	r.ReduceOnly = true
	if err := r.Validate(); err != nil {
		t.Fatalf("reduce-only on a perp was refused: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The client order id, against BOTH venues' published rules
// ---------------------------------------------------------------------------

// TestMintedClientOrderIDSatisfiesBothVenues closes the assumption
// ids.ClientOrderID has carried since it was written.
//
// Both rules are now verified (Profiles §7): Binance's
// ^[\.A-Z\:/a-z0-9_-]{1,36}$ and Bybit's "max of 36 characters … numbers,
// letters (upper and lower cases), dashes, and underscores". The minted format
// is 35 characters of [a-zA-Z0-9-], which is inside both — with ONE character of
// headroom, which is why the length is asserted here rather than left implied.
func TestMintedClientOrderIDSatisfiesBothVenues(t *testing.T) {
	t.Parallel()

	id := mintedID(t)
	if len(id) != 35 {
		t.Fatalf("minted id %q is %d characters; the venues allow 36, and this test is the record of the margin", id, len(id))
	}

	for name, codec := range map[string]venue.OrderCodec{
		"binance": binanceCodec(t),
		"bybit":   bybitCodec(t),
	} {
		r := perpRequest(t, testBinance)
		if name == "bybit" {
			r.Instrument = perpInstrument(testBybit)
		}
		r.ClientOrderID = id
		if _, err := codec.EncodePlace(r); err != nil {
			t.Errorf("%s refused a minted client order id %q: %v", name, id, err)
		}
	}
}

// TestTheTwoVenuesClientOrderIDRulesDiffer is the discriminating fixture.
//
// A "." is INSIDE Binance's published class and OUTSIDE Bybit's. A test that
// only ever used ids both venues accept could not tell a per-venue check from
// one shared check written to whichever rule the author remembered — the two
// would agree on every fixture. This one is the case where they must disagree.
func TestTheTwoVenuesClientOrderIDRulesDiffer(t *testing.T) {
	t.Parallel()

	const dotted ids.ClientOrderID = "ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-01.00"

	rBinance := perpRequest(t, testBinance)
	rBinance.ClientOrderID = dotted
	if _, err := binanceCodec(t).EncodePlace(rBinance); err != nil {
		t.Errorf("binance refused %q, which is inside its published class: %v", dotted, err)
	}

	rBybit := perpRequest(t, testBybit)
	rBybit.ClientOrderID = dotted
	if _, err := bybitCodec(t).EncodePlace(rBybit); !errors.Is(err, venue.ErrOrderIDUnacceptable) {
		t.Errorf("bybit accepted %q, which its published class excludes: err = %v", dotted, err)
	}
}

// TestClientOrderIDLengthIsRefusedAtTheBoundary.
//
// 37 characters rather than 100: the failure worth catching is one character
// over, where a "<" written as "<=" is the whole defect. A grossly long id would
// be refused by either predicate.
func TestClientOrderIDLengthIsRefusedAtTheBoundary(t *testing.T) {
	t.Parallel()

	const at36 ids.ClientOrderID = "ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-01-000"
	const at37 ids.ClientOrderID = "ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-01-0000"
	if len(at36) != 36 || len(at37) != 37 {
		t.Fatalf("fixture lengths are %d and %d, want 36 and 37", len(at36), len(at37))
	}

	for name, codec := range map[string]venue.OrderCodec{"binance": binanceCodec(t), "bybit": bybitCodec(t)} {
		v := testBinance
		if name == "bybit" {
			v = testBybit
		}
		r := perpRequest(t, v)

		r.ClientOrderID = at36
		if _, err := codec.EncodePlace(r); err != nil {
			t.Errorf("%s refused a 36-character id, which both venues allow: %v", name, err)
		}

		r.ClientOrderID = at37
		if _, err := codec.EncodePlace(r); !errors.Is(err, venue.ErrOrderIDUnacceptable) {
			t.Errorf("%s accepted a 37-character id: err = %v", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Binance
// ---------------------------------------------------------------------------

func binanceCodec(t *testing.T) *venue.BinanceOrderCodec {
	t.Helper()
	c, err := venue.NewBinanceOrderCodec(testBinance)
	if err != nil {
		t.Fatalf("NewBinanceOrderCodec: %v", err)
	}
	return c
}

// TestBinanceEncodePlaceRoutesAndCostsPerFamily.
//
// The cost row is the one worth reading. Binance documents the futures
// new-order endpoint as "0 on IP rate limit(x-mbx-used-weight-1m)" and the spot
// one as "Weight: 1", so the two families genuinely charge different budgets —
// and ratelimit.Cost.Validate had to be corrected to permit the zero.
func TestBinanceEncodePlaceRoutesAndCostsPerFamily(t *testing.T) {
	t.Parallel()

	perp, err := binanceCodec(t).EncodePlace(perpRequest(t, testBinance))
	if err != nil {
		t.Fatalf("EncodePlace(perp): %v", err)
	}
	if perp.Path != "/fapi/v1/order" {
		t.Errorf("perp path = %q, want /fapi/v1/order", perp.Path)
	}
	if want := (ratelimit.Cost{Weight: 0, Orders: 1}); perp.Cost != want {
		t.Errorf("perp cost = %+v, want %+v — the venue documents zero weight for this endpoint", perp.Cost, want)
	}
	if perp.Family != ids.ProductLinearPerp {
		t.Errorf("perp family = %v, want linear_perp", perp.Family)
	}

	spot, err := binanceCodec(t).EncodePlace(spotRequest(t, testBinance))
	if err != nil {
		t.Fatalf("EncodePlace(spot): %v", err)
	}
	if spot.Path != "/api/v3/order" {
		t.Errorf("spot path = %q, want /api/v3/order", spot.Path)
	}
	if want := (ratelimit.Cost{Weight: 1, Orders: 1}); spot.Cost != want {
		t.Errorf("spot cost = %+v, want %+v", spot.Cost, want)
	}
	if spot.Family != ids.ProductSpot {
		t.Errorf("spot family = %v, want spot", spot.Family)
	}
}

// TestBinanceEncodePlaceParameters asserts the whole ordered list.
//
// Ordered rather than set-compared, because both venues sign the query string as
// a STRING: a codec that produced the right parameters in a different order
// would sign one thing and be read as another, and the failure arrives as an
// authentication error that reads like a credential fault.
func TestBinanceEncodePlaceParameters(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodePlace(perpRequest(t, testBinance))
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	want := transport.Params{
		{Key: "symbol", Value: "BTCUSDT"},
		{Key: "side", Value: "SELL"},
		{Key: "type", Value: "LIMIT"},
		{Key: "timeInForce", Value: "IOC"},
		{Key: "quantity", Value: "0.5"},
		{Key: "price", Value: "50100.25"},
		{Key: "newClientOrderId", Value: string(mintedID(t))},
		{Key: "newOrderRespType", Value: "ACK"},
	}
	if len(call.Params) != len(want) {
		t.Fatalf("params = %+v, want %+v", call.Params, want)
	}
	for i := range want {
		if call.Params[i] != want[i] {
			t.Errorf("param %d = %+v, want %+v", i, call.Params[i], want[i])
		}
	}
}

// TestBinanceRendersBothSides.
//
// It exists because its absence was found by a MUTATION rather than by reading:
// `binanceSide(OrderSideBuy)` was changed to return "SELL" and every test still
// passed. The fixtures were varied correctly — the perp request sells and the
// spot request buys — and no Binance assertion ever READ the buy fixture's side.
// That is the collapse checklist's sibling: the fixture contained the thing
// under test, and no assertion could see it.
//
// The Bybit mirror was already covered, because its body assertion is a
// byte-for-byte comparison that necessarily includes the side. Two codecs, the
// same code shape, and asymmetric evidence — which is slice C1's finding
// arriving one layer down, in the tests instead of the venues.
func TestBinanceRendersBothSides(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		side venue.OrderSide
		want string
	}{
		{venue.OrderSideBuy, "BUY"},
		{venue.OrderSideSell, "SELL"},
	} {
		r := perpRequest(t, testBinance)
		r.Side = tc.side
		call, err := binanceCodec(t).EncodePlace(r)
		if err != nil {
			t.Fatalf("EncodePlace(%s): %v", tc.side, err)
		}
		if got, _ := paramValue(call.Params, "side"); got != tc.want {
			t.Errorf("side %s rendered as %q, want %q", tc.side, got, tc.want)
		}
	}
}

// TestBinanceRequestsAnACKResponseOnSpot.
//
// The reason this is its own test rather than one row above: on SPOT a LIMIT
// order's response DEFAULTS to FULL, which carries a `fills` array. §4 makes the
// private stream the single authority over an order's state, so the venue's
// default hands this system executions on the submission path at the moment
// there is no transaction open to apply them in. Asking for ACK removes them
// from the wire rather than relying on a decoder to keep ignoring them.
func TestBinanceRequestsAnACKResponseOnSpot(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodePlace(spotRequest(t, testBinance))
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	var found bool
	for _, p := range call.Params {
		if p.Key == "newOrderRespType" {
			found = p.Value == "ACK"
		}
	}
	if !found {
		t.Errorf("spot params %+v do not ask for ACK — the venue would default this LIMIT order to FULL", call.Params)
	}
}

// TestBinanceSendsReduceOnlyOnlyWhenItIsSet.
//
// Both directions, because with ReduceOnly false the parameter is ABSENT: a
// fixture that never set it would leave the true branch unexercised while
// looking like it covered the field.
func TestBinanceSendsReduceOnlyOnlyWhenItIsSet(t *testing.T) {
	t.Parallel()

	off, err := binanceCodec(t).EncodePlace(perpRequest(t, testBinance))
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if hasParam(off.Params, "reduceOnly") {
		t.Error("reduceOnly was sent for an order that did not set it")
	}

	r := perpRequest(t, testBinance)
	r.ReduceOnly = true
	on, err := binanceCodec(t).EncodePlace(r)
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if v, ok := paramValue(on.Params, "reduceOnly"); !ok || v != "true" {
		t.Errorf("reduceOnly = %q, %v; want \"true\", true", v, ok)
	}
}

// TestBinanceRefusesCoinMargined: the inverse family has no recorded venue fact
// and no arithmetic written for it, and routing it to the USDⓈ-M path would
// place a coin-margined order against linear sizing.
func TestBinanceRefusesCoinMargined(t *testing.T) {
	t.Parallel()

	r := perpRequest(t, testBinance)
	r.Instrument = ids.VenueInstrument{
		Venue: testBinance, Product: ids.ProductInversePerp, Symbol: "BTCUSD", Contract: ids.ContractInverse,
	}
	if _, err := binanceCodec(t).EncodePlace(r); !errors.Is(err, venue.ErrOrderFamilyUnrouted) {
		t.Errorf("EncodePlace(inverse) = %v, want ErrOrderFamilyUnrouted", err)
	}
}

func TestBinanceDecodePlaceAck(t *testing.T) {
	t.Parallel()

	r := perpRequest(t, testBinance)
	body := `{"orderId":22542179,"clientOrderId":"` + string(r.ClientOrderID) + `","status":"NEW"}`

	ack, err := binanceCodec(t).DecodePlaceAck(r, transport.Response{Status: 200, Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePlaceAck: %v", err)
	}
	if ack.VenueOrderID != "22542179" {
		t.Errorf("VenueOrderID = %q, want the venue's own digits", ack.VenueOrderID)
	}

	// A venue id past 2^53, which is where a float64 decode stops being exact.
	// The digits are the fixture: an operator searching a venue UI types them.
	big := `{"orderId":9007199254740993,"clientOrderId":"` + string(r.ClientOrderID) + `"}`
	ack, err = binanceCodec(t).DecodePlaceAck(r, transport.Response{Status: 200, Body: []byte(big)})
	if err != nil {
		t.Fatalf("DecodePlaceAck(large id): %v", err)
	}
	if ack.VenueOrderID != "9007199254740993" {
		t.Errorf("VenueOrderID = %q, want 9007199254740993 — a float64 decode loses the last digit", ack.VenueOrderID)
	}
}

// TestDecodePlaceAckRefusesAnEchoOfAnotherOrder, on BOTH venues.
//
// This is the check the whole decode step exists for. §5.3 makes the
// ClientOrderID the only handle recovery has, so recording an ack that describes
// a different order attaches someone else's venue id to ours and every later
// resolving query answers about the wrong one.
func TestDecodePlaceAckRefusesAnEchoOfAnotherOrder(t *testing.T) {
	t.Parallel()

	rB := perpRequest(t, testBinance)
	binanceBodies := map[string]string{
		"a different id": `{"orderId":1,"clientOrderId":"ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-09-09"}`,
		"no id at all":   `{"orderId":1}`,
		"no order id":    `{"clientOrderId":"` + string(rB.ClientOrderID) + `"}`,
	}
	for name, body := range binanceBodies {
		if _, err := binanceCodec(t).DecodePlaceAck(rB, transport.Response{Body: []byte(body)}); !errors.Is(err, venue.ErrAckUnreadable) {
			t.Errorf("binance %s: err = %v, want ErrAckUnreadable", name, err)
		}
	}

	rY := perpRequest(t, testBybit)
	bybitBodies := map[string]string{
		"a different id": `{"retCode":0,"result":{"orderId":"1","orderLinkId":"ab-01J8ZQ4M7K3P9V2XW5R6T8Y0BN-09-09"}}`,
		"no id at all":   `{"retCode":0,"result":{"orderId":"1"}}`,
		"no order id":    `{"retCode":0,"result":{"orderLinkId":"` + string(rY.ClientOrderID) + `"}}`,
	}
	for name, body := range bybitBodies {
		if _, err := bybitCodec(t).DecodePlaceAck(rY, transport.Response{Body: []byte(body)}); !errors.Is(err, venue.ErrAckUnreadable) {
			t.Errorf("bybit %s: err = %v, want ErrAckUnreadable", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Bybit
// ---------------------------------------------------------------------------

func bybitCodec(t *testing.T) *venue.BybitOrderCodec {
	t.Helper()
	c, err := venue.NewBybitOrderCodec(testBybit)
	if err != nil {
		t.Fatalf("NewBybitOrderCodec: %v", err)
	}
	return c
}

// TestBybitEncodePlaceBody asserts the exact bytes, on both families.
//
// Exact bytes rather than a decoded comparison, because Bybit signs the
// "jsonBodyString" — the bytes actually transmitted. A test that decoded the
// body and compared fields would pass against an encoder whose output does not
// match what the signature covers.
func TestBybitEncodePlaceBody(t *testing.T) {
	t.Parallel()

	id := string(mintedID(t))

	perp, err := bybitCodec(t).EncodePlace(perpRequest(t, testBybit))
	if err != nil {
		t.Fatalf("EncodePlace(perp): %v", err)
	}
	wantPerp := `{"category":"linear","symbol":"BTCUSDT","side":"Sell","orderType":"Limit",` +
		`"qty":"0.5","price":"50100.25","timeInForce":"IOC","orderLinkId":"` + id + `"}`
	if string(perp.Body) != wantPerp {
		t.Errorf("perp body =\n%s\nwant\n%s", perp.Body, wantPerp)
	}
	if perp.Path != "/v5/order/create" {
		t.Errorf("path = %q, want /v5/order/create", perp.Path)
	}
	if want := (ratelimit.Cost{Orders: 1}); perp.Cost != want {
		t.Errorf("cost = %+v, want %+v — Bybit publishes no per-endpoint weight", perp.Cost, want)
	}

	spot, err := bybitCodec(t).EncodePlace(spotRequest(t, testBybit))
	if err != nil {
		t.Fatalf("EncodePlace(spot): %v", err)
	}
	wantSpot := `{"category":"spot","symbol":"BTCUSDT","side":"Buy","orderType":"Limit",` +
		`"qty":"0.5","price":"50100.25","timeInForce":"IOC","orderLinkId":"` + id + `","isLeverage":0}`
	if string(spot.Body) != wantSpot {
		t.Errorf("spot body =\n%s\nwant\n%s", spot.Body, wantSpot)
	}
}

// TestBybitSpotCarriesIsLeverageZero is the §4.1 premise, checked as a byte.
//
// isLeverage=0 is what keeps the spot leg an unborrowed cash purchase, which is
// the premise of the whole "spot cannot be seized to cover the perp loss"
// property. The value that must be sent is ZERO, and `omitempty` — the obvious
// struct tag — drops exactly zero. So this asserts the field is PRESENT, not
// merely that it decodes to 0: a dropped field also decodes to 0.
func TestBybitSpotCarriesIsLeverageZero(t *testing.T) {
	t.Parallel()

	call, err := bybitCodec(t).EncodePlace(spotRequest(t, testBybit))
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if !strings.Contains(string(call.Body), `"isLeverage":0`) {
		t.Errorf("spot body %s does not carry isLeverage:0 — the spot leg would be margin-enabled", call.Body)
	}

	// And absent on a perp, where the parameter does not apply.
	perp, err := bybitCodec(t).EncodePlace(perpRequest(t, testBybit))
	if err != nil {
		t.Fatalf("EncodePlace(perp): %v", err)
	}
	if strings.Contains(string(perp.Body), "isLeverage") {
		t.Errorf("perp body %s carries isLeverage, which is a spot parameter", perp.Body)
	}
}

func TestBybitSendsReduceOnlyOnlyWhenItIsSet(t *testing.T) {
	t.Parallel()

	off, err := bybitCodec(t).EncodePlace(perpRequest(t, testBybit))
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if strings.Contains(string(off.Body), "reduceOnly") {
		t.Errorf("body %s carries reduceOnly for an order that did not set it", off.Body)
	}

	r := perpRequest(t, testBybit)
	r.ReduceOnly = true
	on, err := bybitCodec(t).EncodePlace(r)
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if !strings.Contains(string(on.Body), `"reduceOnly":true`) {
		t.Errorf("body %s does not carry reduceOnly:true", on.Body)
	}
}

func TestBybitDecodePlaceAck(t *testing.T) {
	t.Parallel()

	r := perpRequest(t, testBybit)
	body := `{"retCode":0,"retMsg":"OK","result":{"orderId":"1321003749386327552","orderLinkId":"` +
		string(r.ClientOrderID) + `"},"retExtInfo":{},"time":1672211918471}`

	ack, err := bybitCodec(t).DecodePlaceAck(r, transport.Response{Status: 200, Body: []byte(body)})
	if err != nil {
		t.Fatalf("DecodePlaceAck: %v", err)
	}
	if ack.VenueOrderID != "1321003749386327552" {
		t.Errorf("VenueOrderID = %q, want 1321003749386327552", ack.VenueOrderID)
	}
}

// ---------------------------------------------------------------------------
// Routing — the half that needs sockets
// ---------------------------------------------------------------------------

func testClock(t *testing.T, v ids.Venue) clock.Clock {
	t.Helper()
	c, err := clock.NewTestClock(time.UnixMilli(1700000000000).UTC(), []clock.Calibration{
		{Venue: clock.Venue(v), Offset: 0, Uncertainty: time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	return c
}

// testGovernor declares BOTH families because the two signed clients share one
// venue governor. Binance carries family budgets; Bybit carries its complete
// venue-level endpoint roster so every recovery surface is routable at boot.
func testGovernor(t *testing.T, v ids.Venue) *ratelimit.Governor {
	t.Helper()
	limits := []ratelimit.Limit{{
		Name: "ip_5s", Scope: ratelimit.ScopeVenue,
		Dimension: ratelimit.DimensionRequests, Interval: 5 * time.Second,
		OpeningCeiling: 100000, RoutineCeiling: 100000, Capacity: 100000,
	}}
	var uncounted []ratelimit.Uncounted
	if v == testBybit {
		for _, endpoint := range (transport.BybitLimitReader{}).Endpoints() {
			capacity := uint64(50)
			opening, routine := uint64(40), uint64(45)
			if endpoint.Name == transport.BybitOrderCreateLimit {
				capacity, opening, routine = 10, 8, 9
			}
			limits = append(limits, ratelimit.Limit{
				Name: endpoint.Name, Scope: ratelimit.ScopeEndpoint,
				Dimension: endpoint.Dimension, Interval: time.Second,
				OpeningCeiling: opening, RoutineCeiling: routine, Capacity: capacity,
			})
		}
		for _, f := range []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp} {
			uncounted = append(uncounted,
				ratelimit.Uncounted{Family: f, Dimension: ratelimit.DimensionWeight, Reason: "bybit publishes no per-endpoint weight"},
				ratelimit.Uncounted{Family: f, Dimension: ratelimit.DimensionOrders, Reason: "orders are metered per endpoint; order_create is the counter"},
			)
		}
	} else {
		// Weight and orders are both required: a futures place costs an order
		// slot at zero weight, so either dimension omitted leaves a real call
		// uncounted. Ceilings stay roomy because admission policy is tested in
		// platform/ratelimit.
		for _, f := range []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp} {
			limits = append(limits, ratelimit.Limit{
				Name: "weight_1m", Scope: ratelimit.ScopeFamily, Family: f,
				Dimension: ratelimit.DimensionWeight, Interval: time.Minute,
				OpeningCeiling: 1400, RoutineCeiling: 1800, Capacity: 2400,
			}, ratelimit.Limit{
				Name: "orders_10s", Scope: ratelimit.ScopeFamily, Family: f,
				Dimension: ratelimit.DimensionOrders, Interval: 10 * time.Second,
				OpeningCeiling: 700, RoutineCeiling: 900, Capacity: 1200,
			})
		}
	}
	g, err := ratelimit.New(testClock(t, v), ratelimit.Profile{
		Venue:         v,
		Families:      []ids.ProductCategory{ids.ProductSpot, ids.ProductLinearPerp},
		Limits:        limits,
		Uncounted:     uncounted,
		ThrottlePause: 30 * time.Second,
		BanPause:      2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("ratelimit.New: %v", err)
	}
	return g
}

func signedClient(t *testing.T, v ids.Venue, srv *httptest.Server, gov *ratelimit.Governor) *transport.SignedClient {
	t.Helper()
	cred, err := transport.NewCredential("k", "s")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	var signer transport.Signer
	var limits transport.LimitReader
	var env transport.EnvelopeReader
	if v == testBybit {
		signer, err = transport.NewBybitSigner(v, cred)
		limits, env = transport.BybitLimitReader{}, transport.BybitEnvelope{}
	} else {
		signer, err = transport.NewBinanceSigner(v, cred)
		limits, env = transport.BinanceLimitReader{}, transport.BinanceEnvelope{}
	}
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	spend, err := transport.AcquireSpendLock(t.TempDir(), signer.SpendDomain(), cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock: %v", err)
	}
	t.Cleanup(func() { _ = spend.Close() })
	c, err := transport.NewSignedClient(transport.SignedClientConfig{
		Scheme:     "http",
		Host:       strings.TrimPrefix(srv.URL, "http://"),
		Timeout:    5 * time.Second,
		ReadLimit:  1 << 16,
		RecvWindow: 3 * time.Second,
		Signer:     signer,
		Limits:     limits,
		Envelope:   env,
		Governor:   gov,
		Clock:      testClock(t, v),
		Spend:      spend,
	})
	if err != nil {
		t.Fatalf("NewSignedClient: %v", err)
	}
	return c
}

// recordingServer answers every request with body and records the paths it saw.
func recordingServer(t *testing.T, body string) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &paths
}

// TestBinancePlaceRoutesEachFamilyToItsOwnHost is the claim §4's
// "key on VenueInstrument" has one layer beneath it.
//
// Two servers, not one. With a single server the test could only check the PATH,
// and a client that sent every order to whichever host it had would pass — which
// is the accident worth catching, because "BTCUSDT" is a real, executable
// instrument on both hosts and the venue would simply accept the wrong one.
func TestBinancePlaceRoutesEachFamilyToItsOwnHost(t *testing.T) {
	t.Parallel()

	id := string(mintedID(t))
	ack := `{"orderId":22542179,"clientOrderId":"` + id + `"}`
	futuresSrv, futuresPaths := recordingServer(t, ack)
	spotSrv, spotPaths := recordingServer(t, ack)

	// ONE governor for both clients: §7.6 keeps per-family budgets under a
	// SHARED IP-ban envelope per venue, and two governors would be two
	// envelopes for one IP, each seeing half the traffic.
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, futuresSrv, gov),
			ids.ProductSpot:       signedClient(t, testBinance, spotSrv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	if _, err := client.Place(context.Background(), perpRequest(t, testBinance)); err != nil {
		t.Fatalf("Place(perp): %v", err)
	}
	if _, err := client.Place(context.Background(), spotRequest(t, testBinance)); err != nil {
		t.Fatalf("Place(spot): %v", err)
	}

	if len(*futuresPaths) != 1 || (*futuresPaths)[0] != "/fapi/v1/order" {
		t.Errorf("the futures host saw %v, want exactly [/fapi/v1/order]", *futuresPaths)
	}
	if len(*spotPaths) != 1 || (*spotPaths)[0] != "/api/v3/order" {
		t.Errorf("the spot host saw %v, want exactly [/api/v3/order]", *spotPaths)
	}
}

// TestPlaceRefusesAnUnroutedFamily: refusing beats defaulting, because a spot
// order sent to the futures client is a valid leveraged short on a real
// instrument rather than an error.
func TestPlaceRefusesAnUnroutedFamily(t *testing.T) {
	t.Parallel()

	srv, paths := recordingServer(t, `{}`)
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	_, err = client.Place(context.Background(), spotRequest(t, testBinance))
	if !errors.Is(err, venue.ErrOrderFamilyUnrouted) {
		t.Fatalf("Place = %v, want ErrOrderFamilyUnrouted", err)
	}
	if got := apperror.EffectOf(err); got != apperror.EffectNotApplied {
		t.Errorf("Effect = %v, want EffectNotApplied — nothing was encoded, let alone sent", got)
	}
	if len(*paths) != 0 {
		t.Errorf("the venue saw %v, want nothing", *paths)
	}
}

// TestPlaceRefusesAnotherVenuesOrder: the request would otherwise be signed with
// these credentials and sent to this host, where the symbol usually exists.
func TestPlaceRefusesAnotherVenuesOrder(t *testing.T) {
	t.Parallel()

	srv, paths := recordingServer(t, `{}`)
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	if _, err := client.Place(context.Background(), perpRequest(t, testBybit)); !errors.Is(err, venue.ErrOrderVenueMismatch) {
		t.Fatalf("Place = %v, want ErrOrderVenueMismatch", err)
	}
	if len(*paths) != 0 {
		t.Errorf("the venue saw %v, want nothing", *paths)
	}
}

// TestBybitPlaceClassifiesARefusalUnderHTTP200 is the transport finding reaching
// the order surface.
//
// The body is the one captured live. A Place that read the status alone would
// return a successful ack here — for an order the venue never created.
func TestBybitPlaceClassifiesARefusalUnderHTTP200(t *testing.T) {
	t.Parallel()

	srv, _ := recordingServer(t, `{"retCode":10001,"retMsg":"Illegal category","result":{},"retExtInfo":{}}`)
	gov := testGovernor(t, testBybit)
	signed := signedClient(t, testBybit, srv, gov)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: bybitCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signed,
			ids.ProductSpot:       signed, // one host serves both families
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	ack, err := client.Place(context.Background(), perpRequest(t, testBybit))
	if err == nil {
		t.Fatalf("Place returned ack %+v and no error for a refused order", ack)
	}
	if got := apperror.EffectOf(err); got != apperror.EffectNotApplied {
		t.Errorf("Effect = %v, want EffectNotApplied", got)
	}
}

// TestBybitPlaceSendsTheSignedBodyVerbatim.
//
// Bybit signs the bytes it receives, so the body on the wire must be the body
// the codec produced. Any layer free to re-serialize between the two breaks
// every signature, and the failure reads as a credential problem.
func TestBybitPlaceSendsTheSignedBodyVerbatim(t *testing.T) {
	t.Parallel()

	id := string(mintedID(t))
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = readAll(r)
		_, _ = w.Write([]byte(`{"retCode":0,"result":{"orderId":"77","orderLinkId":"` + id + `"}}`))
	}))
	t.Cleanup(srv.Close)

	gov := testGovernor(t, testBybit)
	signed := signedClient(t, testBybit, srv, gov)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec:      bybitCodec(t),
		Governor:   gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{ids.ProductLinearPerp: signed},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	req := perpRequest(t, testBybit)
	ack, err := client.Place(context.Background(), req)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}
	if ack.VenueOrderID != "77" {
		t.Errorf("VenueOrderID = %q, want 77", ack.VenueOrderID)
	}

	encoded, err := bybitCodec(t).EncodePlace(req)
	if err != nil {
		t.Fatalf("EncodePlace: %v", err)
	}
	if string(seen) != string(encoded.Body) {
		t.Errorf("the wire carried\n%s\nbut the signature covers\n%s", seen, encoded.Body)
	}
}

// TestPlaceReportsAnUndecodableAckAsApplied.
//
// The uncomfortable and correct reading: the venue answered 2xx, so the order
// EXISTS. EffectUnknown would send the OMS to ask the venue; EffectApplied says
// the order is live and the local record is what needs correcting.
func TestPlaceReportsAnUndecodableAckAsApplied(t *testing.T) {
	t.Parallel()

	srv, _ := recordingServer(t, `{"orderId":1,"clientOrderId":"somebody-elses-order"}`)
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	_, err = client.Place(context.Background(), perpRequest(t, testBinance))
	if err == nil {
		t.Fatal("an ack describing another order was accepted")
	}
	if got := apperror.EffectOf(err); got != apperror.EffectApplied {
		t.Errorf("Effect = %v, want EffectApplied — the venue accepted the call, so an order exists", got)
	}
}

func TestNewOrderClientRefusesAnIncompleteSurface(t *testing.T) {
	t.Parallel()

	srv, _ := recordingServer(t, `{}`)
	gov := testGovernor(t, testBinance)
	good := venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, srv, gov),
		},
	}
	if _, err := venue.NewOrderClient(good); err != nil {
		t.Fatalf("a complete surface was refused: %v", err)
	}

	if _, err := venue.NewOrderClient(venue.OrderClientConfig{Transports: good.Transports}); !errors.Is(err, venue.ErrOrderCodecUnset) {
		t.Errorf("a client with no codec = %v, want ErrOrderCodecUnset", err)
	}
	if _, err := venue.NewOrderClient(venue.OrderClientConfig{Codec: good.Codec}); !errors.Is(err, venue.ErrOrderTransportsUnset) {
		t.Errorf("a client with no transports = %v, want ErrOrderTransportsUnset", err)
	}
	nilRoute := venue.OrderClientConfig{
		Codec:      good.Codec,
		Governor:   good.Governor,
		Transports: map[ids.ProductCategory]*transport.SignedClient{ids.ProductSpot: nil},
	}
	if _, err := venue.NewOrderClient(nilRoute); !errors.Is(err, venue.ErrOrderTransportsUnset) {
		t.Errorf("a client with a nil route = %v, want ErrOrderTransportsUnset", err)
	}
}

func TestOrderClientReservesAndConsumesExactPlacementCosts(t *testing.T) {
	t.Parallel()
	id := string(mintedID(t))
	ack := `{"orderId":22542179,"clientOrderId":"` + id + `"}`
	perpSrv, _ := recordingServer(t, ack)
	spotSrv, _ := recordingServer(t, ack)
	g := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: g,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, perpSrv, g),
			ids.ProductSpot:       signedClient(t, testBinance, spotSrv, g),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}
	reqs := []venue.OrderRequest{perpRequest(t, testBinance), spotRequest(t, testBinance)}
	h, d, err := client.ReservePlace(reqs)
	if err != nil || !d.Admitted {
		t.Fatalf("ReservePlace = (%v, %+v, %v)", h, d, err)
	}
	for _, l := range g.Snapshot().Limits {
		want := uint64(0)
		switch {
		case l.Scope == ratelimit.ScopeVenue:
			want = 2
		case l.Family == ids.ProductLinearPerp && l.Dimension == ratelimit.DimensionOrders:
			want = 1
		case l.Family == ids.ProductSpot && l.Dimension == ratelimit.DimensionWeight:
			want = 1
		case l.Family == ids.ProductSpot && l.Dimension == ratelimit.DimensionOrders:
			want = 1
		}
		if l.Reserved != want || l.Used != 0 {
			t.Errorf("limit %q/%s = Reserved %d Used %d, want %d/0",
				l.Name, l.Family, l.Reserved, l.Used, want)
		}
	}
	for _, req := range reqs {
		req.Hold = h
		if _, err := client.Place(context.Background(), req); err != nil {
			t.Fatalf("Place(%s): %v", req.Instrument.Product, err)
		}
	}
	for _, l := range g.Snapshot().Limits {
		if l.Reserved != 0 {
			t.Errorf("limit %q residual Reserved = %d, want 0", l.Name, l.Reserved)
		}
	}
}

type secondLegUnpreparableCodec struct {
	venue.OrderCodec
}

func (c secondLegUnpreparableCodec) EncodePlace(r venue.OrderRequest) (transport.Call, error) {
	call, err := c.OrderCodec.EncodePlace(r)
	if err == nil && r.Instrument.Product == ids.ProductSpot {
		// Invalid URL escape, selected on the SECOND request in the fixture. The
		// first call is fully encoded and prepared before this one refuses.
		call.Path = "/%zz"
	}
	return call, err
}

// TestReservePlacePreparesEveryLegBeforeOccupyingCapacity is the prepare-seam
// ordering fixture. Each request is valid and the first leg is buildable; only
// the second codec result cannot compose into an HTTP URL, so a reserve-as-you-go
// implementation would leave visible occupancy. No request may reach the wire.
func TestReservePlacePreparesEveryLegBeforeOccupyingCapacity(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(srv.Close)

	gov := testGovernor(t, testBinance)
	signed := signedClient(t, testBinance, srv, gov)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: secondLegUnpreparableCodec{OrderCodec: binanceCodec(t)}, Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signed, ids.ProductSpot: signed,
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}
	before := gov.Snapshot()
	_, _, err = client.ReservePlace([]venue.OrderRequest{
		perpRequest(t, testBinance), spotRequest(t, testBinance),
	})
	if !errors.Is(err, transport.ErrCallUnpreparable) {
		t.Fatalf("ReservePlace = %v, want ErrCallUnpreparable", err)
	}
	if after := gov.Snapshot(); !reflect.DeepEqual(after, before) {
		t.Errorf("snapshot after prepare refusal = %+v, want byte-for-byte state %+v", after, before)
	}
	if got := requests.Load(); got != 0 {
		t.Errorf("HTTP requests = %d, want 0", got)
	}
}

func TestNewOrderClientChecksGovernorIdentity(t *testing.T) {
	t.Parallel()
	srv, _ := recordingServer(t, `{}`)
	gA := testGovernor(t, testBinance)
	gB := testGovernor(t, testBinance)
	clientA := signedClient(t, testBinance, srv, gA)
	if _, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gA,
		Transports: map[ids.ProductCategory]*transport.SignedClient{ids.ProductLinearPerp: clientA},
	}); err != nil {
		t.Fatalf("matching governor refused: %v", err)
	}
	if _, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gB,
		Transports: map[ids.ProductCategory]*transport.SignedClient{ids.ProductLinearPerp: clientA},
	}); !errors.Is(err, venue.ErrOrderGovernorMismatch) {
		t.Fatalf("mismatched governor = %v, want ErrOrderGovernorMismatch", err)
	}
	if _, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec:      binanceCodec(t),
		Transports: map[ids.ProductCategory]*transport.SignedClient{ids.ProductLinearPerp: clientA},
	}); !errors.Is(err, venue.ErrOrderGovernorUnset) {
		t.Fatalf("nil governor = %v, want ErrOrderGovernorUnset", err)
	}
}

// ---------------------------------------------------------------------------

func hasParam(ps transport.Params, key string) bool {
	_, ok := paramValue(ps, key)
	return ok
}

func paramValue(ps transport.Params, key string) (string, bool) {
	for _, p := range ps {
		if p.Key == key {
			return p.Value, true
		}
	}
	return "", false
}

func readAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	var buf []byte
	dec := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	buf = raw
	return buf, nil
}
