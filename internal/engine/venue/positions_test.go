package venue_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file checks §7.5's position surface, venue-side (slice F2, record §56).
//
// # The collapse checklist was run first, and it fixed the shape of every
// # fixture below
//
// Each of these is a parameter that, pinned to its obvious value, would make a
// distinction this code makes invisible:
//
//   - The positions are a SHORT and a LONG, never two of a kind. A decoder that
//     dropped the sign, or that read Bybit's side as a constant, agrees with a
//     one-directional fixture on every number it produces.
//   - There are TWO declared symbols, so "this row's instrument" and "the first
//     row's instrument" are different values — a decoder that attributed every
//     row to the first declared instrument passes a single-symbol fixture.
//   - The venue answers with a symbol the caller did NOT declare, so the
//     recognized and unrecognized paths are both exercised in one answer rather
//     than in two tests that could each be passing for the other's reason.
//   - The unrecognized symbols arrive OUT of alphabetical order, because the
//     report promises a sorted list and a pre-sorted fixture cannot tell a sort
//     from a copy.
//   - Bybit's flat row is `side: ""` with `size: "0"`, which is the documented
//     empty position, and there is a separate `side: ""` with a NONZERO size,
//     which must be refused. The two differ in exactly the field the rule reads.

func perpAssets(t *testing.T, v ids.Venue) map[ids.VenueInstrument]venue.InstrumentAssets {
	t.Helper()
	return map[ids.VenueInstrument]venue.InstrumentAssets{
		perpInstrument(v):    {Base: "BTC", Quote: "USDT"},
		ethPerpInstrument(v): {Base: "ETH", Quote: "USDT"},
		spotInstrument(v):    {Base: "BTC", Quote: "USDT"},
	}
}

func ethPerpInstrument(v ids.Venue) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: v, Product: ids.ProductLinearPerp, Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
}

func perpScope() venue.PositionScope {
	return venue.PositionScope{Product: ids.ProductLinearPerp, Settle: "USDT"}
}

// ---------------------------------------------------------------------------
// The scope, and the two refusals that are venue facts rather than wiring gaps
// ---------------------------------------------------------------------------

func TestPositionScopeRefusesAnUnsetSettlementAsset(t *testing.T) {
	t.Parallel()

	scope := venue.PositionScope{Product: ids.ProductLinearPerp}
	if err := scope.Validate(); !errors.Is(err, venue.ErrPositionSettleUnset) {
		t.Errorf("Validate() = %v, want ErrPositionSettleUnset", err)
	}

	// Both codecs must refuse it at the encode, not only the one that sends it:
	// Binance ignores the settlement asset entirely, so a caller that forgot it
	// would see the invariant assert cleanly there and fail on Bybit alone.
	if _, err := binanceCodec(t).EncodePositions(scope); !errors.Is(err, venue.ErrPositionSettleUnset) {
		t.Errorf("binance EncodePositions(no settle) = %v, want ErrPositionSettleUnset", err)
	}
	if _, err := bybitCodec(t).EncodePositions(scope); !errors.Is(err, venue.ErrPositionSettleUnset) {
		t.Errorf("bybit EncodePositions(no settle) = %v, want ErrPositionSettleUnset", err)
	}
}

func TestSpotHasNoPositionSurfaceOnEitherVenue(t *testing.T) {
	t.Parallel()

	spot := venue.PositionScope{Product: ids.ProductSpot, Settle: "USDT"}
	_, binanceErr := binanceCodec(t).EncodePositions(spot)
	_, bybitErr := bybitCodec(t).EncodePositions(spot)

	for name, err := range map[string]error{"binance": binanceErr, "bybit": bybitErr} {
		if !errors.Is(err, venue.ErrPositionSurfaceAbsent) {
			t.Errorf("%s EncodePositions(spot) = %v, want ErrPositionSurfaceAbsent", name, err)
		}
		// Not the routing error. An unrouted family is a wiring gap someone can
		// close; a spot balance is not a position anywhere, and a routing
		// diagnosis would send a reader to add a route that cannot exist.
		if errors.Is(err, venue.ErrOrderFamilyUnrouted) {
			t.Errorf("%s EncodePositions(spot) reports a routing gap, want the surface-absent fact", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Binance — /fapi/v3/positionRisk, and the enumeration is NOT symbol-scoped
// ---------------------------------------------------------------------------

func TestBinancePositionsEnumeratesWithoutASymbol(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodePositions(perpScope())
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	if call.Path != "/fapi/v3/positionRisk" {
		t.Errorf("path = %q, want /fapi/v3/positionRisk — V2 returns every symbol whether or not it has a position, so an absent row there is not a flat account", call.Path)
	}
	if call.Cost.Weight != 5 {
		t.Errorf("weight = %d, want 5 (V3's published IP weight; the changelog raised V2's to 10)", call.Cost.Weight)
	}
	for _, p := range call.Params {
		if p.Key == "symbol" {
			t.Errorf("params carry symbol=%q — a symbol-scoped read can only ask about instruments this system already knows, which is the one set that cannot contain a foreign position", p.Value)
		}
	}
}

func TestBinanceDecodesSignedPositionsAndNamesTheUndeclared(t *testing.T) {
	t.Parallel()

	// BTC is SHORT, ETH is LONG, and DOGE/SOL are symbols this deployment does
	// not trade — listed newest-first-ish and out of alphabetical order.
	body := `[
	  {"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.750"},
	  {"symbol":"SOLUSDT","positionSide":"BOTH","positionAmt":"12"},
	  {"symbol":"ETHUSDT","positionSide":"BOTH","positionAmt":"4.5"},
	  {"symbol":"DOGEUSDT","positionSide":"BOTH","positionAmt":"0"}
	]`

	report, err := binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance), okResponse(body))
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}

	want := map[string]string{"BTCUSDT": "-0.75", "ETHUSDT": "4.5"}
	if len(report.Positions) != len(want) {
		t.Fatalf("positions = %d, want %d", len(report.Positions), len(want))
	}
	for _, p := range report.Positions {
		got := p.Net.Decimal()
		if got != want[p.Instrument.Symbol] {
			t.Errorf("%s net = %s, want %s", p.Instrument.Symbol, got, want[p.Instrument.Symbol])
		}
		if p.Instrument.Product != ids.ProductLinearPerp || p.Instrument.Venue != testBinance {
			t.Errorf("%s instrument = %s/%s, want the scope's", p.Instrument.Symbol, p.Instrument.Venue, p.Instrument.Product)
		}
	}

	// Sorted, and both rows present: a zero-size row on an undeclared symbol is
	// V3 reporting an OPEN ORDER there, which is §7.5's foreign order and is not
	// less interesting for having no position behind it yet.
	if len(report.Unrecognized) != 2 || report.Unrecognized[0] != "DOGEUSDT" || report.Unrecognized[1] != "SOLUSDT" {
		t.Errorf("unrecognized = %v, want [DOGEUSDT SOLUSDT] sorted", report.Unrecognized)
	}
}

func TestBinanceEmptyAnswerIsAFlatAccount(t *testing.T) {
	t.Parallel()

	report, err := binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance), okResponse(`[]`))
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	if len(report.Positions) != 0 || len(report.Unrecognized) != 0 {
		t.Errorf("report = %+v, want empty", report)
	}
}

func TestBinanceRefusesHedgeModeAndDoubleRows(t *testing.T) {
	t.Parallel()

	// A hedge-mode account answers with LONG and SHORT rows for one symbol,
	// whose net is a subtraction no field states. §4.1 makes that posture
	// refuse-to-trade; here it is refused one layer earlier, because this query
	// runs before the posture check has anything to compare against.
	hedged := `[
	  {"symbol":"BTCUSDT","positionSide":"LONG","positionAmt":"1.0"},
	  {"symbol":"BTCUSDT","positionSide":"SHORT","positionAmt":"-0.4"}
	]`
	_, err := binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance), okResponse(hedged))
	if !errors.Is(err, venue.ErrPositionModeNotOneWay) {
		t.Errorf("DecodePositions(hedge rows) = %v, want ErrPositionModeNotOneWay", err)
	}

	twice := `[
	  {"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"1.0"},
	  {"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.4"}
	]`
	_, err = binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance), okResponse(twice))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("DecodePositions(two one-way rows) = %v, want ErrPositionsUnreadable", err)
	}
}

// ---------------------------------------------------------------------------
// Bybit — /v5/position/list, where the direction is OUTSIDE the number
// ---------------------------------------------------------------------------

func TestBybitPositionsScopesOnCategoryAndSettleCoin(t *testing.T) {
	t.Parallel()

	call, err := bybitCodec(t).EncodePositions(perpScope())
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	if call.Path != "/v5/position/list" {
		t.Errorf("path = %q, want /v5/position/list", call.Path)
	}
	params := map[string]string{}
	for _, p := range call.Params {
		params[p.Key] = p.Value
	}
	// settleCoin is what makes the call legal with no symbol: "either symbol or
	// settleCoin is required" for linear.
	if params["category"] != "linear" || params["settleCoin"] != "USDT" {
		t.Errorf("params = %v, want category=linear settleCoin=USDT", params)
	}
	if _, sent := params["symbol"]; sent {
		t.Errorf("params carry a symbol, which narrows the enumeration to instruments we already know")
	}
}

func TestBybitAppliesSideToAnUnsignedSize(t *testing.T) {
	t.Parallel()

	// Sell on BTC, Buy on ETH, a documented FLAT row on a declared symbol, and
	// one undeclared symbol.
	body := `{"result":{"category":"linear","list":[
	  {"symbol":"BTCUSDT","side":"Sell","size":"0.750","positionIdx":0},
	  {"symbol":"ETHUSDT","side":"Buy","size":"4.5","positionIdx":0},
	  {"symbol":"SOLUSDT","side":"Buy","size":"12","positionIdx":0}
	]}}`

	report, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(body))
	if err != nil {
		t.Fatalf("DecodePositions: %v", err)
	}
	want := map[string]string{"BTCUSDT": "-0.75", "ETHUSDT": "4.5"}
	if len(report.Positions) != len(want) {
		t.Fatalf("positions = %d, want %d", len(report.Positions), len(want))
	}
	for _, p := range report.Positions {
		if got := p.Net.Decimal(); got != want[p.Instrument.Symbol] {
			t.Errorf("%s net = %s, want %s — size is 'always positive' and the sign is in side", p.Instrument.Symbol, got, want[p.Instrument.Symbol])
		}
	}
	if len(report.Unrecognized) != 1 || report.Unrecognized[0] != "SOLUSDT" {
		t.Errorf("unrecognized = %v, want [SOLUSDT]", report.Unrecognized)
	}
}

func TestBybitEmptySideIsFlatOnlyWhenTheSizeIsZero(t *testing.T) {
	t.Parallel()

	flat := `{"result":{"category":"linear","list":[
	  {"symbol":"BTCUSDT","side":"","size":"0","positionIdx":0}
	]}}`
	report, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(flat))
	if err != nil {
		t.Fatalf("DecodePositions(documented empty position): %v", err)
	}
	if len(report.Positions) != 1 {
		t.Fatalf("positions = %d, want 1 — a flat row is an answer, and refusing it would turn a settled account into a failed boot", len(report.Positions))
	}
	if sign, err := report.Positions[0].Net.Sign(); err != nil || sign != 0 {
		t.Errorf("net sign = %d (err %v), want 0", sign, err)
	}

	live := `{"result":{"category":"linear","list":[
	  {"symbol":"BTCUSDT","side":"","size":"0.75","positionIdx":0}
	]}}`
	if _, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(live)); !errors.Is(err, venue.ErrPositionSideUnreadable) {
		t.Errorf("DecodePositions(empty side, live size) = %v, want ErrPositionSideUnreadable — reading it as flat is the one answer that hides a live position", err)
	}
}

func TestBybitRefusesNonOneWayRowsAndForeignCategories(t *testing.T) {
	t.Parallel()

	hedged := `{"result":{"category":"linear","list":[
	  {"symbol":"BTCUSDT","side":"Buy","size":"1","positionIdx":1}
	]}}`
	if _, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(hedged)); !errors.Is(err, venue.ErrPositionModeNotOneWay) {
		t.Errorf("DecodePositions(positionIdx 1) = %v, want ErrPositionModeNotOneWay", err)
	}

	spotAnswer := `{"result":{"category":"spot","list":[
	  {"symbol":"BTCUSDT","side":"Buy","size":"1","positionIdx":0}
	]}}`
	if _, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(spotAnswer)); !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("DecodePositions(category echo mismatch) = %v, want ErrPositionsUnreadable", err)
	}

	signed := `{"result":{"category":"linear","list":[
	  {"symbol":"BTCUSDT","side":"Sell","size":"-1","positionIdx":0}
	]}}`
	if _, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(signed)); !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("DecodePositions(negative size) = %v, want ErrPositionsUnreadable — a signed size beside a side would apply the direction twice", err)
	}
}

// ---------------------------------------------------------------------------
// The two venues answer the SAME account differently, and both must normalize
// to the same number. This is the positive control for the pair of decoders:
// without it, a decoder that inverted both venues consistently would pass every
// per-venue assertion above.
// ---------------------------------------------------------------------------

func TestBothVenuesNormalizeTheSameShortToTheSameNumber(t *testing.T) {
	t.Parallel()

	binance, err := binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance),
		okResponse(`[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.750"}]`))
	if err != nil {
		t.Fatalf("binance DecodePositions: %v", err)
	}
	bybit, err := bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit),
		okResponse(`{"result":{"category":"linear","list":[{"symbol":"BTCUSDT","side":"Sell","size":"0.750","positionIdx":0}]}}`))
	if err != nil {
		t.Fatalf("bybit DecodePositions: %v", err)
	}

	// The literal is stated here rather than compared between the two decoders:
	// a comparison alone would pass if both were long.
	const wantShort = "-0.75"
	if got := binance.Positions[0].Net.Decimal(); got != wantShort {
		t.Errorf("binance net = %s, want %s", got, wantShort)
	}
	if got := bybit.Positions[0].Net.Decimal(); got != wantShort {
		t.Errorf("bybit net = %s, want %s", got, wantShort)
	}
}

// TestAmbiguousDeclaredInstrumentsAreRefused is the coin flip removed.
//
// ids.VenueInstrument carries a ContractType the position lookup does not need,
// and an UNVALIDATED one splits a single real instrument across two map keys —
// domain.FillKey records the same hazard on the fill dedupe identity. Returning
// the first match would attribute the row to whichever key Go's randomized map
// iteration reached first, and the two keys carry different base assets in
// exactly the case where the mistake matters.
func TestAmbiguousDeclaredInstrumentsAreRefused(t *testing.T) {
	t.Parallel()

	assets := map[ids.VenueInstrument]venue.InstrumentAssets{
		perpInstrument(testBinance): {Base: "BTC", Quote: "USDT"},
		// The same real instrument under a second key: the contract tag
		// disagrees with the product, which VenueInstrument.Validate would
		// refuse and a map key never sees.
		{Venue: testBinance, Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractInverse}: {Base: "XBT", Quote: "USDT"},
	}

	_, err := binanceCodec(t).DecodePositions(perpScope(), assets,
		okResponse(`[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.750"}]`))
	if !errors.Is(err, venue.ErrPositionInstrumentAmbiguous) {
		t.Errorf("DecodePositions(two matching declarations) = %v, want ErrPositionInstrumentAmbiguous", err)
	}
}
