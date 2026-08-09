package venue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks §4's MarginStatus surface, venue-side (task #208).
//
// # The collapse checklist was run first, and it shaped every fixture below
//
// Each item is a parameter that, pinned to the value it naturally takes, would
// make a distinction this code makes invisible:
//
//   - THE FOUR DECIMALS ARE ALL DIFFERENT AND ALL DIFFERENT MAGNITUDES. The row
//     carries a position amount, a liquidation price, a maintenance margin and a
//     posted margin, all as decimal strings of the same character class — the
//     format-collapse shape. With any two equal, a transposed assignment mints
//     the identical struct. They are also different ORDERS of magnitude, so a
//     failure message names which pair swapped.
//   - THE POSITION IS A SHORT, so Binance's positionAmt is NEGATIVE. That is the
//     leg §7.1 exists for, and it is the only fixture under which `sign == 0`
//     and `sign > 0` are different predicates — a presence test written as
//     "positive" reports every real short flat, and a long-only fixture agrees
//     with it.
//   - THE TWO VENUES USE DIFFERENT NUMBERS. Both codecs read one venue answer
//     shape into one Go type, so a test that named the wrong codec would pass
//     against shared figures.
//   - EACH VENUE HAS A FULL-SHAPE FIXTURE carrying every field the venue really
//     publishes, not only the six each codec declares. A narrowed wire struct is
//     a claim about which neighbouring keys it does NOT pick up, and a fixture
//     containing only the declared keys cannot test that claim (§4's wire-tag
//     seam rule — `p`/`P`, record §78 §3).
//   - THE PORTFOLIO-MARGIN REFUSAL IS FIXTURED TWICE, once with positionMM
//     empty and once with positionIM empty. Emptying both at once makes `||` and
//     `&&` the same predicate.
//   - THE ASSET DISAGREEMENT IS FIXTURED ON EACH OF ITS THREE ARMS, and the
//     negative-figure refusal on each of its two call sites, because a shared
//     helper proves nothing about a call site that passed it the wrong argument.
//   - THE LIQ PRICE IS FIXTURED AT ALL THREE SPELLINGS — a real number, the
//     documented empty string, and a degenerate zero — because the absence rule
//     has three arms and two of them return the same pair.
//
// Task #219 added two decoded fields and three items to the list, the first of
// which was a collapse ALREADY PRESENT and harmless only while the field went
// undecoded:
//
//   - THE ENTRY PRICE DIFFERS BETWEEN THE VENUES. Both rows carried 64000.10,
//     which was free while neither codec read it and is the third checklist item
//     above the moment they both do: a test naming the wrong codec would agree
//     on the one number it now asserts.
//   - THE ENTRY PRICE DIFFERS FROM ITS NEIGHBOURS ON THE ROW, and those
//     neighbours are the point of the full-shape fixture. Binance publishes
//     `breakEvenPrice` — entry adjusted for fees — one key away from
//     `entryPrice`, same magnitude, same character class; Bybit puts `avgPrice`
//     beside `markPrice` and `positionValue`. Each would validate perfectly in
//     the decoded slot.
//   - BYBIT'S `side` IS VARIED, at Sell, at Buy and at the documented empty
//     string. The codec now signs its size, and against a Sell-only fixture
//     "apply the side" and "negate unconditionally" are the same function.
//     Binance's amount is fixtured at both signs for the mirror of that reason,
//     which is cheaper there and is taken anyway (record §56's rule: neither
//     venue's row is readable with the other's rule).

const (
	// The Binance figures. A 0.503 BTC SHORT.
	//
	// None of these ends in a trailing zero, and that is a constraint rather
	// than a style: fixedpoint's Decimal() normalizes one away, so a fixture
	// spelled "0.500" cannot be asserted against its own constant and the test
	// would have to restate the expected number somewhere the fixture does not
	// reach. Trailing-zero PARSING is fixedpoint's own claim and is checked
	// there.
	binMarginAmt   = "-0.503"
	binMarginEntry = "64180.41"
	binMarginLiq   = "71234.55"
	binMarginMaint = "312.75"
	binMarginPost  = "1850.25"

	// The Bybit figures, deliberately different from Binance's. Bybit's size is
	// documented "always positive" and the direction lives in `side`, which this
	// codec reads since #219 through §7.5's own rule.
	bybMarginSize  = "0.375"
	bybMarginSide  = "Sell"
	bybMarginEntry = "63105.61"
	bybMarginLiq   = "68420.75"
	bybMarginMaint = "298.65"
	bybMarginPost  = "1420.85"
)

func marginAssets() venue.InstrumentAssets {
	return venue.InstrumentAssets{Base: "BTC", Quote: "USDT"}
}

// binMarginRow is the Binance fixture's varying fields.
//
// A struct rather than six positional strings, which is this file's own thesis
// applied to its own fixtures: every field here is a decimal string of one
// character class, and at six parameters a transposed ARGUMENT mints a
// well-formed row exactly the way a transposed assignment mints a well-formed
// struct. Each call site below names only what it varies from the healthy short.
type binMarginRow struct {
	Amt, Entry, Liq, Maint, Posted, MarginAsset string
}

// bybMarginRow is the Bybit fixture's varying fields. See binMarginRow.
type bybMarginRow struct {
	Size, Side, Entry, Liq, Maint, Posted string
}

// binanceMarginBody renders one V3 row with every field Binance really
// publishes, edited by zero or more mutators.
//
// All twenty, per Profiles §9's dated transcription and the independent
// confirmation from Binance's spec-generated connector. The seven the codec
// declares are interpolated; the other thirteen carry values that would be
// WRONG if they landed in a declared field — `markPrice` differs from
// `liquidationPrice`, `initialMargin` and `maintMargin` differ, `isolatedMargin`
// and `isolatedWallet` differ, and `breakEvenPrice` differs from `entryPrice`.
// A narrowing that picked up a neighbour therefore fails on the value rather
// than only on the shape.
func binanceMarginBody(edits ...func(*binMarginRow)) string {
	r := binMarginRow{
		Amt: binMarginAmt, Entry: binMarginEntry, Liq: binMarginLiq,
		Maint: binMarginMaint, Posted: binMarginPost, MarginAsset: "USDT",
	}
	for _, edit := range edits {
		edit(&r)
	}
	return `[{` +
		`"symbol":"BTCUSDT",` +
		`"positionSide":"BOTH",` +
		`"positionAmt":"` + r.Amt + `",` +
		`"entryPrice":"` + r.Entry + `",` +
		`"breakEvenPrice":"64010.20",` +
		`"markPrice":"65500.30",` +
		`"unRealizedProfit":"-750.40",` +
		`"liquidationPrice":"` + r.Liq + `",` +
		`"isolatedMargin":"1099.85",` +
		`"notional":"-32750.15",` +
		`"marginAsset":"` + r.MarginAsset + `",` +
		`"isolatedWallet":"` + r.Posted + `",` +
		`"initialMargin":"1637.50",` +
		`"maintMargin":"` + r.Maint + `",` +
		`"positionInitialMargin":"1637.50",` +
		`"openOrderInitialMargin":"0",` +
		`"adl":2,` +
		`"bidNotional":"0",` +
		`"askNotional":"0",` +
		`"updateTime":1754640000000` +
		`}]`
}

// bybitMarginBody renders one /v5/position/list row with every field Bybit
// really publishes, including the three it documents as Deprecated.
//
// `positionBalance` is one of them ("Deprecated, can refer to positionIM"), and
// it is given a value DIFFERENT from positionIM here: it is the field whose name
// reads like §4's "available margin", so a codec that reached for it lands on a
// number the assertions can see.
func bybitMarginBody(edits ...func(*bybMarginRow)) string {
	r := bybMarginRow{
		Size: bybMarginSize, Side: bybMarginSide, Entry: bybMarginEntry,
		Liq: bybMarginLiq, Maint: bybMarginMaint, Posted: bybMarginPost,
	}
	for _, edit := range edits {
		edit(&r)
	}
	return `{"result":{"category":"linear","list":[{` +
		`"positionIdx":0,` +
		`"riskId":1,` +
		`"symbol":"BTCUSDT",` +
		`"side":"` + r.Side + `",` +
		`"size":"` + r.Size + `",` +
		`"avgPrice":"` + r.Entry + `",` +
		`"positionValue":"24000.05",` +
		`"tradeMode":0,` +
		`"positionStatus":"Normal",` +
		`"autoAddMargin":0,` +
		`"adlRankIndicator":2,` +
		`"leverage":"10",` +
		`"positionBalance":"1111.11",` +
		`"markPrice":"65500.30",` +
		`"liqPrice":"` + r.Liq + `",` +
		`"bustPrice":"",` +
		`"positionMM":"` + r.Maint + `",` +
		`"positionMMByMp":"999.99",` +
		`"positionIM":"` + r.Posted + `",` +
		`"positionIMByMp":"888.88",` +
		`"tpslMode":"Full",` +
		`"takeProfit":"0.00",` +
		`"stopLoss":"0.00",` +
		`"unrealisedPnl":"-750.40",` +
		`"curRealisedPnl":"0",` +
		`"seq":1234567,` +
		`"isReduceOnly":false,` +
		`"createdTime":"1754000000000",` +
		`"updatedTime":"1754640000000"` +
		`}],"nextPageCursor":""},"retCode":0,"retMsg":"OK"}`
}

// ---------------------------------------------------------------------------
// The encode: which endpoint, which parameters, what it costs
// ---------------------------------------------------------------------------

func TestBinanceMarginStatusReadsV3ScopedToOneSymbol(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodeMarginStatus(perpInstrument(testBinance))
	if err != nil {
		t.Fatalf("EncodeMarginStatus: %v", err)
	}
	if call.Path != "/fapi/v3/positionRisk" {
		t.Errorf("path = %q, want /fapi/v3/positionRisk — V2 answers a FLAT symbol with a row of zeros, and a liquidation price of 0 is a number §7.1 computes with", call.Path)
	}
	if call.Method != "GET" {
		t.Errorf("method = %q, want GET", call.Method)
	}

	// The symbol IS sent, which is the exact inverse of EncodePositions. The
	// enumeration must not narrow because the narrowed-away set is where a
	// foreign position lives; this call asks about one position already known.
	var sawSymbol bool
	for _, p := range call.Params {
		if p.Key == "symbol" {
			sawSymbol = true
			if p.Value != "BTCUSDT" {
				t.Errorf("symbol = %q, want BTCUSDT", p.Value)
			}
		}
	}
	if !sawSymbol {
		t.Error("no symbol parameter — the per-instrument read would enumerate the family and charge the expensive weight arm")
	}

	// 5, the maximum over #180's three candidate readings on the arm a
	// symbol-bearing call takes (a flat 5; "5 with symbol"; "1 with symbol").
	// The enumeration charges 20 for the other arm of the same unread line.
	if call.Cost.Weight != 5 {
		t.Errorf("weight = %d, want 5 — the max over #180's candidates for the CHEAP arm", call.Cost.Weight)
	}
	if call.Family != ids.ProductLinearPerp {
		t.Errorf("family = %v, want linear perp", call.Family)
	}
}

func TestBybitMarginStatusScopesByCategoryAndSymbol(t *testing.T) {
	t.Parallel()

	call, err := bybitCodec(t).EncodeMarginStatus(perpInstrument(testBybit))
	if err != nil {
		t.Fatalf("EncodeMarginStatus: %v", err)
	}
	if call.Path != "/v5/position/list" {
		t.Errorf("path = %q, want /v5/position/list", call.Path)
	}

	got := map[string]string{}
	for _, p := range call.Params {
		got[p.Key] = p.Value
	}
	if got["category"] != "linear" {
		t.Errorf("category = %q, want linear", got["category"])
	}
	if got["symbol"] != "BTCUSDT" {
		t.Errorf("symbol = %q, want BTCUSDT", got["symbol"])
	}
	// settleCoin is required only for the SYMBOL-LESS enumeration. Sending it
	// here would be cargo from EncodePositions, and Bybit documents the symbol
	// as having "a higher priority" anyway.
	if _, sent := got["settleCoin"]; sent {
		t.Error("settleCoin sent — it is required only when no symbol is given (EncodePositions' call, not this one)")
	}

	// No published weight: /v5/position/list is metered per request per UID.
	if call.Cost.Weight != 0 {
		t.Errorf("weight = %d, want 0 — Bybit publishes none for this endpoint", call.Cost.Weight)
	}
	if err := call.Cost.Validate(); err != nil {
		// RequestOnly rather than a bare zero Cost: the bare zero is the field
		// nobody filled in, and Cost.Validate refuses it before Acquire prices
		// the call.
		t.Errorf("Cost.Validate() = %v, want a request-dimension cost, not the unfilled zero", err)
	}
}

func TestSpotHasNoMarginStatusOnEitherVenue(t *testing.T) {
	t.Parallel()

	_, binanceErr := binanceCodec(t).EncodeMarginStatus(spotInstrument(testBinance))
	_, bybitErr := bybitCodec(t).EncodeMarginStatus(spotInstrument(testBybit))

	for name, err := range map[string]error{"binance": binanceErr, "bybit": bybitErr} {
		if !errors.Is(err, venue.ErrPositionSurfaceAbsent) {
			t.Errorf("%s EncodeMarginStatus(spot) = %v, want ErrPositionSurfaceAbsent", name, err)
		}
		// Not the routing error, on EncodePositions' argument: an unrouted family
		// is a wiring gap someone can close, and a spot margin surface exists
		// nowhere — a routing diagnosis sends a reader to add a route that
		// cannot exist.
		if errors.Is(err, venue.ErrOrderFamilyUnrouted) {
			t.Errorf("%s EncodeMarginStatus(spot) reports a routing gap, want the surface-absent fact", name)
		}
	}
}

// ---------------------------------------------------------------------------
// The decode — the healthy answer, on both venues
// ---------------------------------------------------------------------------

func TestBinanceMarginStatusDecodesAShort(t *testing.T) {
	t.Parallel()

	instrument := perpInstrument(testBinance)
	body := binanceMarginBody()

	status, held, err := binanceCodec(t).DecodeMarginStatus(instrument, marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false on a 0.5 BTC short — a NEGATIVE positionAmt is the leg §7.1 exists for, and a presence test written as `> 0` reports it flat")
	}
	assertMarginStatus(t, status, instrument, binMarginLiq, binMarginMaint, binMarginPost, binMarginAmt, binMarginEntry)
}

func TestBybitMarginStatusDecodesAShort(t *testing.T) {
	t.Parallel()

	instrument := perpInstrument(testBybit)
	body := bybitMarginBody()

	status, held, err := bybitCodec(t).DecodeMarginStatus(instrument, marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false on a live position")
	}
	// Bybit states an always-positive size beside `side: "Sell"`, so the
	// expected quantity is the NEGATED fixture string — the one place the two
	// venues' rows are not readable with each other's rule.
	assertMarginStatus(t, status, instrument, bybMarginLiq, bybMarginMaint, bybMarginPost, "-"+bybMarginSize, bybMarginEntry)
}

// assertMarginStatus checks all six decoded fields by VALUE.
//
// By value rather than by "is populated", because the fixtures are decimal
// strings of one character class: a transposition produces a fully populated
// struct, and the only assertion that can see it is one that names the expected
// number. The asset tags are checked too — the substitution available at every
// call site is InstrumentAssets.Base for .Quote, which every other quantity on
// this client legitimately uses.
//
// #219's two fields make that argument sharper rather than merely longer. The
// entry price and the liq price are the SAME Go type in the same pair, so
// nothing structural can tell them apart in each other's slots and Validate is
// deliberately not asked to try (see engine/venue's marginstatus.go: the guard
// a reader reaches for is the liquidation condition itself). These value
// assertions are the whole defence, on both venues.
//
// `signedQty` is the expected SIGNED quantity, which on Bybit is not the string
// the fixture puts on the wire: the row states an always-positive size beside a
// side, so the caller states what the pair should MEAN rather than what either
// field says.
func assertMarginStatus(t *testing.T, got venue.MarginStatus, instrument ids.VenueInstrument, liq, maint, posted, signedQty, entry string) {
	t.Helper()

	if got.Instrument != instrument {
		t.Errorf("instrument = %v, want %v", got.Instrument, instrument)
	}
	if got.SizingQuantity.Decimal() != signedQty {
		t.Errorf("SizingQuantity = %s, want %s — the SIGN is the half a short depends on", got.SizingQuantity.Decimal(), signedQty)
	}
	if got.SizingQuantity.Asset() != "BTC" {
		t.Errorf("SizingQuantity asset = %q, want BTC — the BASE, unlike every margin figure on this row", got.SizingQuantity.Asset())
	}
	if got.EntryPrice.Decimal() != entry {
		t.Errorf("EntryPrice = %s, want %s — `breakEvenPrice` and `markPrice` are one key away and would validate here", got.EntryPrice.Decimal(), entry)
	}
	if got.EntryPrice.Quote() != "USDT" {
		t.Errorf("EntryPrice quote = %q, want USDT", got.EntryPrice.Quote())
	}
	if got.EntryPrice.Base() != "BTC" {
		t.Errorf("EntryPrice base = %q, want BTC", got.EntryPrice.Base())
	}
	if !got.LiqPriceStated {
		t.Fatalf("LiqPriceStated = false, want true — the fixture states %s", liq)
	}
	if got.LiqPrice.Decimal() != liq {
		t.Errorf("LiqPrice = %s, want %s", got.LiqPrice.Decimal(), liq)
	}
	if got.LiqPrice.Quote() != "USDT" {
		t.Errorf("LiqPrice quote = %q, want USDT", got.LiqPrice.Quote())
	}
	if got.Maintenance.Decimal() != maint {
		t.Errorf("Maintenance = %s, want %s", got.Maintenance.Decimal(), maint)
	}
	if got.Maintenance.Asset() != "USDT" {
		t.Errorf("Maintenance asset = %q, want USDT — the QUOTE, not the base every other quantity here uses", got.Maintenance.Asset())
	}
	if got.PostedMargin.Decimal() != posted {
		t.Errorf("PostedMargin = %s, want %s", got.PostedMargin.Decimal(), posted)
	}
	if got.PostedMargin.Asset() != "USDT" {
		t.Errorf("PostedMargin asset = %q, want USDT", got.PostedMargin.Asset())
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil on a decoded live position", err)
	}
}

// ---------------------------------------------------------------------------
// "No position" is spelled OPPOSITELY on the two venues
// ---------------------------------------------------------------------------

func TestNoPositionIsSpelledDifferentlyOnEachVenue(t *testing.T) {
	t.Parallel()

	// Binance V3: "only symbol that has position or open orders will be
	// returned", so a flat symbol yields an empty array.
	status, held, err := binanceCodec(t).DecodeMarginStatus(
		perpInstrument(testBinance), marginAssets(), okResponse(`[]`))
	if err != nil {
		t.Fatalf("binance DecodeMarginStatus(empty array): %v", err)
	}
	if held {
		t.Error("binance held = true on an empty V3 answer, which is the venue saying this account holds nothing here")
	}
	if status != (venue.MarginStatus{}) {
		t.Errorf("binance returned %+v beside held=false — a flat instrument must carry no figures at all", status)
	}

	// Bybit, verbatim: "If symbol passed, it returns data regardless of having
	// position or not". So flat is a ROW with size 0 — the opposite spelling,
	// and the one §7.5's enumeration did not train.
	status, held, err = bybitCodec(t).DecodeMarginStatus(
		perpInstrument(testBybit), marginAssets(),
		okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Size = "0" })))
	if err != nil {
		t.Fatalf("bybit DecodeMarginStatus(size 0): %v", err)
	}
	if held {
		t.Error("bybit held = true on a size-0 row, which is the documented flat position")
	}
	if status != (venue.MarginStatus{}) {
		t.Errorf("bybit returned %+v beside held=false", status)
	}
}

func TestBinanceReportsAnOpenOrderRowWithNoPositionAsFlat(t *testing.T) {
	t.Parallel()

	// V3 returns a row for a symbol with "position OR open orders", so a resting
	// order on a closed position produces a row whose positionAmt is 0. Its
	// margin figures are the degenerate ones this surface exists to keep away
	// from §7.1, and the row's mere PRESENCE is not a position.
	body := binanceMarginBody(func(r *binMarginRow) {
		r.Amt, r.Entry, r.Liq, r.Maint, r.Posted = "0.000", "0", "0", "0", "0"
	})

	_, held, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginStatus: %v", err)
	}
	if held {
		t.Error("held = true on a zero-amount row — V3 answers for open orders too, so a row is not by itself a position")
	}
}

// ---------------------------------------------------------------------------
// The liquidation price has THREE spellings and two of them mean absence
// ---------------------------------------------------------------------------

func TestBybitEmptyLiqPriceIsAValueAndNotAZero(t *testing.T) {
	t.Parallel()

	// Verbatim: liqPrice "keeps \"\" when liqPrice <= minPrice or liqPrice >=
	// maxPrice". That is the WELL-MARGINED case — a short whose liquidation sits
	// above the symbol's price band — so it arrives on a healthy position.
	body := bybitMarginBody(func(r *bybMarginRow) { r.Liq = "" })

	status, held, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false — an unstated liq price does not make a live position disappear")
	}
	if status.LiqPriceStated {
		t.Error("LiqPriceStated = true on the documented empty string")
	}
	if status.LiqPrice != (fixedpoint.Price{}) {
		t.Errorf("LiqPrice = %s beside LiqPriceStated=false — the two halves must not be able to disagree", status.LiqPrice)
	}
	// The rest of the reading is still usable: maintenance and posted margin are
	// what a top-up is sized from, and they are stated.
	if status.Maintenance.Decimal() != bybMarginMaint {
		t.Errorf("Maintenance = %s, want %s", status.Maintenance.Decimal(), bybMarginMaint)
	}
	if err := status.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil — a stateless liq price is a legal reading", err)
	}
}

func TestADegenerateLiqPriceIsAbsenceRatherThanAPrice(t *testing.T) {
	t.Parallel()

	// Neither venue documents a numeric absence spelling, so this is the
	// conservative arm: a zero read as a PRICE puts a short's liquidation far
	// BELOW its mark, which every distance formula reads as maximum headroom.
	// Treating it as absence costs one escalation rung on a venue that never
	// sends it.
	cases := map[string]func() (venue.MarginStatus, bool, error){
		"binance": func() (venue.MarginStatus, bool, error) {
			return binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(),
				okResponse(binanceMarginBody(func(r *binMarginRow) { r.Liq = "0" })))
		},
		"bybit": func() (venue.MarginStatus, bool, error) {
			return bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
				okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Liq = "0" })))
		},
	}
	for name, decode := range cases {
		status, held, err := decode()
		if err != nil {
			t.Fatalf("%s DecodeMarginStatus: %v", name, err)
		}
		if !held {
			t.Fatalf("%s held = false — a degenerate liq price does not make the position disappear", name)
		}
		if status.LiqPriceStated {
			t.Errorf("%s LiqPriceStated = true on a liq price of 0 — a short's liquidation below its mark reads as maximum headroom", name)
		}
	}
}

func TestAnUnreadableLiqPriceIsAnErrorRatherThanAnAbsence(t *testing.T) {
	t.Parallel()

	// The third arm, and the one that keeps the second from swallowing a venue
	// that changed its format: an unreadable answer is not an absence (§5.3).
	_, _, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
		okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Liq = "not-a-price" })))
	if err == nil {
		t.Fatal("DecodeMarginStatus accepted an unreadable liq price — a format change would decode as a healthy position with no liquidation price")
	}
}

// ---------------------------------------------------------------------------
// The sizing fields (#219) — the SIGN, and the entry price that has no absence
// ---------------------------------------------------------------------------

func TestBybitAppliesTheSideRatherThanNegatingUnconditionally(t *testing.T) {
	t.Parallel()

	// The margin fixture is a SHORT, so against it alone "apply the side" and
	// "always negate" are the same function — and Bybit's size is documented
	// "always positive", which means the sign lives nowhere else on the row. A
	// codec that hardcoded the direction would report every long as a short and
	// size a delever the wrong way past flat.
	cases := map[string]struct {
		side string
		want string
	}{
		"Sell is the short §7.1 exists for":  {side: "Sell", want: "-" + bybMarginSize},
		"Buy is the same magnitude POSITIVE": {side: "Buy", want: bybMarginSize},
	}
	for name, c := range cases {
		status, held, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
			okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Side = c.side })))
		if err != nil {
			t.Fatalf("%s: DecodeMarginStatus: %v", name, err)
		}
		if !held {
			t.Fatalf("%s: held = false on a nonzero size", name)
		}
		if status.SizingQuantity.Decimal() != c.want {
			t.Errorf("%s: SizingQuantity = %s, want %s", name, status.SizingQuantity.Decimal(), c.want)
		}
	}
}

func TestBinanceCarriesTheSignTheRowStates(t *testing.T) {
	t.Parallel()

	// The mirror, and it is taken even though it is cheaper here: Binance signs
	// the amount itself, so the failure this catches is a codec that negated or
	// took an absolute value on the way out. "A fix applied in one place gets
	// applied to its mirror" — and the two venues' rules are opposites, which is
	// exactly the pair where one test standing in for both is tempting.
	cases := map[string]string{"-0.503": "-0.503", "0.503": "0.503"}
	for amt, want := range cases {
		status, held, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(),
			okResponse(binanceMarginBody(func(r *binMarginRow) { r.Amt = amt })))
		if err != nil {
			t.Fatalf("positionAmt %s: DecodeMarginStatus: %v", amt, err)
		}
		if !held {
			t.Fatalf("positionAmt %s: held = false", amt)
		}
		if status.SizingQuantity.Decimal() != want {
			t.Errorf("positionAmt %s: SizingQuantity = %s, want %s", amt, status.SizingQuantity.Decimal(), want)
		}
	}
}

func TestBybitDocumentedFlatRowIsFlatRatherThanUnreadable(t *testing.T) {
	t.Parallel()

	// The real flat row on Bybit is `side: ""` beside `size: "0"` beside an
	// `avgPrice` of 0 — three degenerate fields at once, and it arrives for every
	// symbol this system has closed. Since #219 the decode signs the size before
	// testing presence and refuses an entry price of zero, so this fixture is the
	// one that proves neither new rule fires on the ORDINARY answer. Refusing it
	// would take §7.1's trigger off every settled instrument.
	status, held, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
		okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Side, r.Size, r.Entry = "", "0", "0" })))
	if err != nil {
		t.Fatalf("DecodeMarginStatus on the documented flat row: %v", err)
	}
	if held {
		t.Error("held = true on `side: \"\"` with size 0, which is the documented empty position")
	}
	if status != (venue.MarginStatus{}) {
		t.Errorf("returned %+v beside held=false", status)
	}
}

func TestBybitRefusesALivePositionWhoseDirectionIsUnreadable(t *testing.T) {
	t.Parallel()

	// `side: ""` beside a NONZERO size is the contradiction §7.5's codec already
	// refuses, and #219 made it reachable here: before it, this codec read the
	// size as a presence test and would have reported the row held with an
	// unsigned quantity nobody could size a remedy from. The two fixtures differ
	// in exactly the field the rule reads.
	_, _, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
		okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Side = "" })))
	if !errors.Is(err, venue.ErrPositionSideUnreadable) {
		t.Errorf("empty side on a live position = %v, want ErrPositionSideUnreadable", err)
	}
}

func TestAnEntryPriceIsREFUSEDWhereALiqPriceIsFlagged(t *testing.T) {
	t.Parallel()

	// The two fields are decoded by rules of different ARITY, deliberately: the
	// liq price has a documented absence that arrives on the healthy path, and
	// this one has none on either venue and no consumer that could proceed with
	// one. Absorbed as zero it enlarges §85 §3's denominator `(1+MMR)·L* − e`,
	// so the delever runs harder than asked off a number no venue stated.
	//
	// Fixtured on BOTH venues and at BOTH degenerate spellings, because the two
	// arms return different sentinels and one fixture would leave the other arm
	// an untested line.
	cases := map[string]struct {
		decode func() (venue.MarginStatus, bool, error)
		want   error
	}{
		"binance empty": {binanceEntry(t, ""), venue.ErrMarginEntryPriceUnstated},
		"bybit empty":   {bybitEntry(t, ""), venue.ErrMarginEntryPriceUnstated},
		"binance zero":  {binanceEntry(t, "0"), venue.ErrMarginEntryPriceNotPositive},
		"bybit zero":    {bybitEntry(t, "0"), venue.ErrMarginEntryPriceNotPositive},
		"binance below": {binanceEntry(t, "-64180.41"), venue.ErrMarginEntryPriceNotPositive},
		"bybit below":   {bybitEntry(t, "-63105.61"), venue.ErrMarginEntryPriceNotPositive},
	}
	for name, c := range cases {
		if _, _, err := c.decode(); !errors.Is(err, c.want) {
			t.Errorf("%s: DecodeMarginStatus = %v, want %v", name, err, c.want)
		}
	}
}

func binanceEntry(t *testing.T, entry string) func() (venue.MarginStatus, bool, error) {
	return func() (venue.MarginStatus, bool, error) {
		return binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(),
			okResponse(binanceMarginBody(func(r *binMarginRow) { r.Entry = entry })))
	}
}

func bybitEntry(t *testing.T, entry string) func() (venue.MarginStatus, bool, error) {
	return func() (venue.MarginStatus, bool, error) {
		return bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(),
			okResponse(bybitMarginBody(func(r *bybMarginRow) { r.Entry = entry })))
	}
}

func TestAnUnreadableEntryPriceIsNeitherOfTheTwoRefusals(t *testing.T) {
	t.Parallel()

	// §5.3's rule, the same one that keeps marginLiqPrice's conservative arm from
	// swallowing a format change: a value that will not parse is a PARSE error
	// and must not be reported as either degenerate spelling, because those two
	// sentinels name diagnoses ("the venue sent nothing", "the venue sent a flat
	// row's value") that a format change does not match.
	_, _, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(),
		okResponse(binanceMarginBody(func(r *binMarginRow) { r.Entry = "not-a-price" })))
	if err == nil {
		t.Fatal("DecodeMarginStatus accepted an unreadable entry price")
	}
	if errors.Is(err, venue.ErrMarginEntryPriceUnstated) || errors.Is(err, venue.ErrMarginEntryPriceNotPositive) {
		t.Errorf("unreadable entry price = %v, want a parse error rather than either absence sentinel", err)
	}
}

// ---------------------------------------------------------------------------
// The differential test — one payload, both codecs, one quantity (record §85)
// ---------------------------------------------------------------------------

func TestBothCodecsReadTheSameQuantityFromOneVenueAnswer(t *testing.T) {
	t.Parallel()

	// Record §85 owes this test, and it is free: `marginstatus.go` records that
	// the margin surface and §7.5's enumeration read the SAME endpoint on both
	// venues, so one captured payload is a legal answer to both calls.
	//
	// What it covers, stated exactly, because the two venues are not symmetric
	// here. On BINANCE it is genuinely differential: the two codecs declare two
	// narrowings of one row and each parses `positionAmt` independently, with no
	// shared helper between them. On BYBIT the sign rule is shared
	// (bybitSignedSize), so a sign DISAGREEMENT is not a thing that can happen —
	// what is still covered there is which JSON keys each narrowing declares,
	// which is the live hazard on a row carrying `size` beside `positionValue`
	// and `positionIM` beside `positionIMByMp`.
	//
	// One row, not two: the margin codec refuses a second row on a symbol-scoped
	// call, so the payload cannot carry one. The position is therefore looked up
	// BY INSTRUMENT rather than taken at index 0 — with a single-element report
	// those are the same value, and naming the instrument is what keeps the
	// assertion honest if the report ever grows.
	type sharedPayload struct {
		v      ids.Venue
		body   string
		margin func(ids.VenueInstrument, string) (venue.MarginStatus, bool, error)
		posns  func(string) (venue.PositionReport, error)
	}
	cases := map[string]sharedPayload{
		"binance": {
			v: testBinance, body: binanceMarginBody(),
			margin: func(i ids.VenueInstrument, b string) (venue.MarginStatus, bool, error) {
				return binanceCodec(t).DecodeMarginStatus(i, marginAssets(), okResponse(b))
			},
			posns: func(b string) (venue.PositionReport, error) {
				return binanceCodec(t).DecodePositions(perpScope(), perpAssets(t, testBinance), okResponse(b))
			},
		},
		"bybit": {
			v: testBybit, body: bybitMarginBody(),
			margin: func(i ids.VenueInstrument, b string) (venue.MarginStatus, bool, error) {
				return bybitCodec(t).DecodeMarginStatus(i, marginAssets(), okResponse(b))
			},
			posns: func(b string) (venue.PositionReport, error) {
				return bybitCodec(t).DecodePositions(perpScope(), perpAssets(t, testBybit), okResponse(b))
			},
		},
	}
	for name, c := range cases {
		instrument := perpInstrument(c.v)

		status, held, err := c.margin(instrument, c.body)
		if err != nil {
			t.Fatalf("%s DecodeMarginStatus: %v", name, err)
		}
		if !held {
			t.Fatalf("%s held = false on the shared payload", name)
		}

		report, err := c.posns(c.body)
		if err != nil {
			t.Fatalf("%s DecodePositions on the SAME payload: %v", name, err)
		}
		var net string
		var found bool
		for _, p := range report.Positions {
			if p.Instrument == instrument {
				net, found = p.Net.Decimal(), true
			}
		}
		if !found {
			t.Fatalf("%s: the position codec found no row for %s in the payload the margin codec read", name, instrument.Symbol)
		}

		if status.SizingQuantity.Decimal() != net {
			t.Errorf("%s: margin codec reads %s, position codec reads %s from ONE payload — the two narrowings disagree about which field the quantity is",
				name, status.SizingQuantity.Decimal(), net)
		}
		if status.SizingQuantity.Asset() != "BTC" {
			t.Errorf("%s: margin codec types the quantity in %q, want the base BTC", name, status.SizingQuantity.Asset())
		}
	}
}

// ---------------------------------------------------------------------------
// The refusals
// ---------------------------------------------------------------------------

func TestMarginStatusRefusesAHedgeModeRow(t *testing.T) {
	t.Parallel()

	// A hedge-mode account answers with a LONG row and a SHORT row, each
	// carrying its own margin figures with no combined one stated. §4.1 makes
	// the posture refuse-to-trade; this arrives first, because the read runs
	// before the posture check has anything to compare against.
	binanceHedge := `[{"symbol":"BTCUSDT","positionSide":"SHORT","positionAmt":"` + binMarginAmt +
		`","liquidationPrice":"` + binMarginLiq + `","maintMargin":"` + binMarginMaint +
		`","isolatedWallet":"` + binMarginPost + `","marginAsset":"USDT"}]`
	_, _, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(), okResponse(binanceHedge))
	if !errors.Is(err, venue.ErrPositionModeNotOneWay) {
		t.Errorf("binance positionSide=SHORT = %v, want ErrPositionModeNotOneWay", err)
	}

	bybitHedge := `{"result":{"category":"linear","list":[{"positionIdx":2,"symbol":"BTCUSDT","size":"` +
		bybMarginSize + `","liqPrice":"` + bybMarginLiq + `","positionMM":"` + bybMarginMaint +
		`","positionIM":"` + bybMarginPost + `"}]}}`
	_, _, err = bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(bybitHedge))
	if !errors.Is(err, venue.ErrPositionModeNotOneWay) {
		t.Errorf("bybit positionIdx=2 = %v, want ErrPositionModeNotOneWay", err)
	}
}

func TestMarginStatusRefusesASecondRow(t *testing.T) {
	t.Parallel()

	// One symbol was asked about. A second row means either hedge mode or a
	// venue that ignored the symbol — and taking either row attributes one leg's
	// margin to the whole position.
	binanceTwo := `[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.5","liquidationPrice":"1","maintMargin":"1","isolatedWallet":"1","marginAsset":"USDT"},` +
		`{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.5","liquidationPrice":"2","maintMargin":"2","isolatedWallet":"2","marginAsset":"USDT"}]`
	_, _, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(), okResponse(binanceTwo))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("binance two rows = %v, want ErrPositionsUnreadable", err)
	}

	bybitTwo := `{"result":{"category":"linear","list":[` +
		`{"positionIdx":0,"symbol":"BTCUSDT","size":"1","liqPrice":"1","positionMM":"1","positionIM":"1"},` +
		`{"positionIdx":0,"symbol":"BTCUSDT","size":"2","liqPrice":"2","positionMM":"2","positionIM":"2"}]}}`
	_, _, err = bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(bybitTwo))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("bybit two rows = %v, want ErrPositionsUnreadable", err)
	}
}

func TestMarginStatusRefusesARowForAnotherSymbol(t *testing.T) {
	t.Parallel()

	// The symbol parameter not taking effect is the one failure here that yields
	// a confident liquidation price for the wrong market.
	binanceOther := `[{"symbol":"ETHUSDT","positionSide":"BOTH","positionAmt":"-0.5","liquidationPrice":"1","maintMargin":"1","isolatedWallet":"1","marginAsset":"USDT"}]`
	_, _, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(), okResponse(binanceOther))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("binance foreign symbol = %v, want ErrPositionsUnreadable", err)
	}

	bybitOther := `{"result":{"category":"linear","list":[{"positionIdx":0,"symbol":"ETHUSDT","size":"1","liqPrice":"1","positionMM":"1","positionIM":"1"}]}}`
	_, _, err = bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(bybitOther))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("bybit foreign symbol = %v, want ErrPositionsUnreadable", err)
	}
}

func TestBybitMarginStatusRefusesAForeignCategoryEcho(t *testing.T) {
	t.Parallel()

	body := `{"result":{"category":"inverse","list":[{"positionIdx":0,"symbol":"BTCUSDT","size":"1","liqPrice":"1","positionMM":"1","positionIM":"1"}]}}`
	_, _, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(body))
	if !errors.Is(err, venue.ErrPositionsUnreadable) {
		t.Errorf("category echo mismatch = %v, want ErrPositionsUnreadable — these margin figures belong to a different family's position", err)
	}
}

func TestBinanceMarginStatusChecksTheStatedMarginAsset(t *testing.T) {
	t.Parallel()

	// Binance is the only one of the two venues that states which asset its
	// margin figures are in, so this is the only place the linear-perp premise
	// (margin asset == quote asset) is falsifiable against the venue rather than
	// asserted from it. Bybit's codec has nothing to check against, which is why
	// this check is worth keeping even though only one codec can make it.
	body := binanceMarginBody(func(r *binMarginRow) { r.MarginAsset = "BNFCR" })

	_, _, err := binanceCodec(t).DecodeMarginStatus(perpInstrument(testBinance), marginAssets(), okResponse(body))
	if !errors.Is(err, venue.ErrMarginAssetsDisagree) {
		t.Errorf("marginAsset=BNFCR against a declared quote of USDT = %v, want ErrMarginAssetsDisagree", err)
	}
}

func TestBybitRefusesEachPortfolioMarginFigureSeparately(t *testing.T) {
	t.Parallel()

	// Verbatim on BOTH fields: "Portfolio margin mode: returns \"\"". §4.1's
	// posture gate refuses Portfolio Margin, so an empty figure means an account
	// reached §7.1's trigger that the boot gate should have stopped — and both
	// absorptions read permissively (a zero maintenance is a position requiring
	// nothing; a zero posted margin is one whose top-up is sized from nothing).
	//
	// Fixtured ONE AT A TIME. Emptying both makes `||` and `&&` the same
	// predicate, so a decoder checking only one field would pass.
	cases := map[string]string{
		"positionMM empty": bybitMarginBody(func(r *bybMarginRow) { r.Maint = "" }),
		"positionIM empty": bybitMarginBody(func(r *bybMarginRow) { r.Posted = "" }),
	}
	for name, body := range cases {
		_, _, err := bybitCodec(t).DecodeMarginStatus(perpInstrument(testBybit), marginAssets(), okResponse(body))
		if !errors.Is(err, venue.ErrMarginFigureUnstated) {
			t.Errorf("%s = %v, want ErrMarginFigureUnstated", name, err)
		}
	}
}

// ---------------------------------------------------------------------------
// The client method — routing, and the flat instrument that must not be
// validated
// ---------------------------------------------------------------------------

func marginClient(t *testing.T, body string) (*venue.OrderClient, *[]string) {
	t.Helper()
	srv, paths := recordingServer(t, body)
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
	return client, paths
}

func TestMarginStatusClientReadsThePerpFamily(t *testing.T) {
	t.Parallel()

	client, paths := marginClient(t,
		binanceMarginBody())

	status, held, err := client.MarginStatus(context.Background(),
		perpInstrument(testBinance), marginAssets(), ratelimit.PrioritySafety)
	if err != nil {
		t.Fatalf("MarginStatus: %v", err)
	}
	if !held {
		t.Fatal("held = false on a live short")
	}
	if len(*paths) != 1 || (*paths)[0] != "/fapi/v3/positionRisk" {
		t.Errorf("the host saw %v, want exactly [/fapi/v3/positionRisk]", *paths)
	}
	if status.LiqPrice.Decimal() != binMarginLiq {
		t.Errorf("LiqPrice = %s, want %s", status.LiqPrice.Decimal(), binMarginLiq)
	}
}

// TestMarginStatusClientDoesNotValidateAFlatInstrument is the client's own
// claim, and it is the one a reordering would break silently.
//
// A flat instrument returns the ZERO MarginStatus, which Validate refuses by
// design — that is the whole point of "the zero value is unusable". So the
// early return on !held is load-bearing: validating first turns the ordinary
// answer for every closed position into a failed read, and a failed read is
// §7.1's stale-input rung. The two rules are individually right and compose
// into a refusal unless the order is the one written here.
func TestMarginStatusClientDoesNotValidateAFlatInstrument(t *testing.T) {
	t.Parallel()

	client, _ := marginClient(t, `[]`)

	status, held, err := client.MarginStatus(context.Background(),
		perpInstrument(testBinance), marginAssets(), ratelimit.PrioritySafety)
	if err != nil {
		t.Fatalf("MarginStatus on a flat instrument = %v, want no error — the zero status must not reach Validate", err)
	}
	if held {
		t.Error("held = true on an empty V3 answer")
	}
	if status != (venue.MarginStatus{}) {
		t.Errorf("status = %+v, want the zero value beside held=false", status)
	}
}

func TestMarginStatusClientRefusesAnotherVenuesInstrument(t *testing.T) {
	t.Parallel()

	client, paths := marginClient(t, `[]`)

	_, _, err := client.MarginStatus(context.Background(),
		perpInstrument(testBybit), marginAssets(), ratelimit.PrioritySafety)
	if !errors.Is(err, venue.ErrOrderVenueMismatch) {
		t.Errorf("MarginStatus(another venue's instrument) = %v, want ErrOrderVenueMismatch", err)
	}
	if len(*paths) != 0 {
		t.Errorf("the host saw %v, want nothing — the refusal is pre-send", *paths)
	}
}

func TestMarginStatusClientRefusesAnUnroutedFamily(t *testing.T) {
	t.Parallel()

	client, paths := marginClient(t, `[]`)

	// Spot has no margin surface at all, and the routing table here holds only
	// the perp family — so this is refused before the codec is asked.
	_, _, err := client.MarginStatus(context.Background(),
		spotInstrument(testBinance), marginAssets(), ratelimit.PrioritySafety)
	if !errors.Is(err, venue.ErrOrderFamilyUnrouted) {
		t.Errorf("MarginStatus(spot) = %v, want ErrOrderFamilyUnrouted", err)
	}
	if len(*paths) != 0 {
		t.Errorf("the host saw %v, want nothing", *paths)
	}
}

func TestMarginStatusClientRefusesUnsetAssets(t *testing.T) {
	t.Parallel()

	client, paths := marginClient(t, `[]`)

	// An unpopulated asset tag builds a Money the runtime check then enforces
	// against every other Money, and the first failure is far from here.
	_, _, err := client.MarginStatus(context.Background(),
		perpInstrument(testBinance), venue.InstrumentAssets{}, ratelimit.PrioritySafety)
	if !errors.Is(err, venue.ErrInstrumentAssetsUnset) {
		t.Errorf("MarginStatus(no assets) = %v, want ErrInstrumentAssetsUnset", err)
	}
	if len(*paths) != 0 {
		t.Errorf("the host saw %v, want nothing", *paths)
	}
}

// ---------------------------------------------------------------------------
// MarginStatus.Validate — the zero value, the contradiction, and the assets
// ---------------------------------------------------------------------------

func liveMarginStatus(t *testing.T) venue.MarginStatus {
	t.Helper()
	liq, err := fixedpoint.ParsePrice("BTC", "USDT", binMarginLiq)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	maint, err := fixedpoint.ParseMoney("USDT", binMarginMaint)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	posted, err := fixedpoint.ParseMoney("USDT", binMarginPost)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	qty, err := fixedpoint.ParseQuantity("BTC", binMarginAmt)
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	entry, err := fixedpoint.ParsePrice("BTC", "USDT", binMarginEntry)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return venue.MarginStatus{
		Instrument:     perpInstrument(testBinance),
		LiqPrice:       liq,
		LiqPriceStated: true,
		Maintenance:    maint,
		PostedMargin:   posted,
		SizingQuantity: qty,
		EntryPrice:     entry,
	}
}

// TestMarginStatusValidateIsPositivelyControlled proves the predicate is not
// universally objecting — without it every refusal below reads as a perfect
// score.
func TestMarginStatusValidateIsPositivelyControlled(t *testing.T) {
	t.Parallel()

	if err := liveMarginStatus(t).Validate(); err != nil {
		t.Fatalf("Validate() on a real status = %v, want nil", err)
	}
	// And the unstated-liq-price variant, which is the OTHER legal shape.
	unstated := liveMarginStatus(t)
	unstated.LiqPrice, unstated.LiqPriceStated = fixedpoint.Price{}, false
	if err := unstated.Validate(); err != nil {
		t.Fatalf("Validate() with no stated liq price = %v, want nil", err)
	}

	// A ZERO margin figure is legal and must pass, on each of the two fields.
	// Without this the fixtures sit strictly inside the bound, where `< 0` and
	// `<= 0` are the same predicate — the shape ratelimit's freeAt cases were
	// caught on. Both zeros are real: a dust position rounds its maintenance to
	// zero, and a position carrying no isolated wallet is what the venue reports
	// the instant before a top-up lands.
	zero, err := fixedpoint.ParseMoney("USDT", "0")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	zeroMaint := liveMarginStatus(t)
	zeroMaint.Maintenance = zero
	if err := zeroMaint.Validate(); err != nil {
		t.Errorf("Validate() with a zero maintenance = %v, want nil", err)
	}
	zeroPosted := liveMarginStatus(t)
	zeroPosted.PostedMargin = zero
	if err := zeroPosted.Validate(); err != nil {
		t.Errorf("Validate() with a zero posted margin = %v, want nil", err)
	}
}

func TestMarginStatusValidateRefusesTheZeroValue(t *testing.T) {
	t.Parallel()

	if err := (venue.MarginStatus{}).Validate(); err == nil {
		t.Fatal("the zero MarginStatus validated — an unpopulated maintenance reads as a position requiring nothing")
	}
}

func TestMarginStatusValidateRefusesAPriceBesideAClearedFlag(t *testing.T) {
	t.Parallel()

	// The flag is the half consumers branch on, so an unflagged number would sit
	// unread until something copied the struct field by field and lost the flag.
	s := liveMarginStatus(t)
	s.LiqPriceStated = false
	if err := s.Validate(); !errors.Is(err, venue.ErrMarginLiqPriceContradicts) {
		t.Errorf("Validate() = %v, want ErrMarginLiqPriceContradicts", err)
	}
}

func TestMarginStatusValidateRefusesANonPositiveStatedLiqPrice(t *testing.T) {
	t.Parallel()

	zero, err := fixedpoint.ParsePrice("BTC", "USDT", "0")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	s := liveMarginStatus(t)
	s.LiqPrice = zero
	if err := s.Validate(); !errors.Is(err, venue.ErrMarginLiqPriceNotPositive) {
		t.Errorf("Validate() = %v, want ErrMarginLiqPriceNotPositive", err)
	}
}

func TestMarginStatusValidateRefusesEachAssetDisagreement(t *testing.T) {
	t.Parallel()

	// Three arms, fixtured separately: the two money fields against each other,
	// and the liq price's quote against the margin asset. A single fixture
	// exercises one comparison and leaves the others as untested lines.
	foreignMoney, err := fixedpoint.ParseMoney("USDC", binMarginPost)
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}
	foreignPrice, err := fixedpoint.ParsePrice("BTC", "USDC", binMarginLiq)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}

	// #219 added two arms. The entry price is the liq price's shape one field
	// over; the sizing quantity is the ONLY field on this struct typed in the
	// base, which makes it the one place the habitual substitution runs the other
	// way — a codec passing assets.Quote where every neighbour takes assets.Base.
	foreignQty, err := fixedpoint.ParseQuantity("USDT", binMarginAmt)
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	foreignEntry, err := fixedpoint.ParsePrice("BTC", "USDC", binMarginEntry)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}

	cases := map[string]func(venue.MarginStatus) venue.MarginStatus{
		"posted margin in another asset": func(s venue.MarginStatus) venue.MarginStatus {
			s.PostedMargin = foreignMoney
			return s
		},
		"maintenance in another asset": func(s venue.MarginStatus) venue.MarginStatus {
			s.Maintenance = foreignMoney
			return s
		},
		"liq price quoted in another asset": func(s venue.MarginStatus) venue.MarginStatus {
			s.LiqPrice = foreignPrice
			return s
		},
		"entry price quoted in another asset": func(s venue.MarginStatus) venue.MarginStatus {
			s.EntryPrice = foreignEntry
			return s
		},
		"sizing quantity in the QUOTE asset": func(s venue.MarginStatus) venue.MarginStatus {
			s.SizingQuantity = foreignQty
			return s
		},
	}
	for name, mutate := range cases {
		if err := mutate(liveMarginStatus(t)).Validate(); !errors.Is(err, venue.ErrMarginAssetsDisagree) {
			t.Errorf("%s: Validate() = %v, want ErrMarginAssetsDisagree", name, err)
		}
	}
}

func TestMarginStatusValidateRefusesAHeldPositionOfZeroSize(t *testing.T) {
	t.Parallel()

	// Unreachable from either codec — both read the quantity as their presence
	// test — and reachable from a status built directly, which is what a hand
	// written fixture is. Both of §7.1's remedies multiply by this number: a zero
	// sizes a top-up of zero and a delever toward nothing, so the ladder would
	// see two remedies run, change nothing, and count as attempted.
	zero, err := fixedpoint.ParseQuantity("BTC", "0")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	s := liveMarginStatus(t)
	s.SizingQuantity = zero
	if err := s.Validate(); !errors.Is(err, venue.ErrMarginQuantityZero) {
		t.Errorf("Validate() = %v, want ErrMarginQuantityZero", err)
	}

	// The positive control for the SIGN: a long validates. Without it a rule
	// written as "must be negative" — which the short-only fixtures above would
	// all agree with — reads as correct, and it would refuse every long this
	// system holds while §7.1 watched only shorts.
	long, err := fixedpoint.ParseQuantity("BTC", "0.500")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	s = liveMarginStatus(t)
	s.SizingQuantity = long
	if err := s.Validate(); err != nil {
		t.Errorf("Validate() on a LONG = %v, want nil — the sign is data, not a constraint", err)
	}
}

func TestMarginStatusValidateRefusesANonPositiveEntryPrice(t *testing.T) {
	t.Parallel()

	// Refused at both spellings a Price can take, unlike the liq price one field
	// over, which flags absence instead. See engine/venue's marginstatus.go for
	// the arity argument and for the cross-check that is deliberately NOT made:
	// "a short liquidates above its entry" solves out to "this position is not
	// currently liquidatable", so a validator enforcing it refuses exactly the
	// readings §7.1's ladder exists to act on.
	for name, raw := range map[string]string{"zero": "0", "negative": "-64180.41"} {
		bad, err := fixedpoint.ParsePrice("BTC", "USDT", raw)
		if err != nil {
			t.Fatalf("%s ParsePrice: %v", name, err)
		}
		s := liveMarginStatus(t)
		s.EntryPrice = bad
		if err := s.Validate(); !errors.Is(err, venue.ErrMarginEntryPriceNotPositive) {
			t.Errorf("%s entry price: Validate() = %v, want ErrMarginEntryPriceNotPositive", name, err)
		}
	}

	// And the control that keeps the paragraph above honest: an entry price
	// ABOVE the liq price is a long's ordinary shape and must validate, so the
	// refused cross-check stays refused rather than creeping back in.
	above, err := fixedpoint.ParsePrice("BTC", "USDT", "99999.99")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	s := liveMarginStatus(t)
	s.EntryPrice = above
	if err := s.Validate(); err != nil {
		t.Errorf("Validate() with entry above the liq price = %v, want nil — that ordering is a POSITION's direction, not an error", err)
	}
}

func TestMarginStatusValidateRefusesEitherNegativeFigure(t *testing.T) {
	t.Parallel()

	// Both call sites of the shared helper, because a tested helper proves
	// nothing about a site that never calls it. Both directions are permissive:
	// a negative maintenance makes any position look over-margined, a negative
	// posted margin makes a top-up look larger than it was.
	negative, err := fixedpoint.ParseMoney("USDT", "-1.00")
	if err != nil {
		t.Fatalf("ParseMoney: %v", err)
	}

	cases := map[string]func(venue.MarginStatus) venue.MarginStatus{
		"maintenance": func(s venue.MarginStatus) venue.MarginStatus {
			s.Maintenance = negative
			return s
		},
		"posted margin": func(s venue.MarginStatus) venue.MarginStatus {
			s.PostedMargin = negative
			return s
		},
	}
	for name, mutate := range cases {
		if err := mutate(liveMarginStatus(t)).Validate(); !errors.Is(err, venue.ErrMarginFigureNegative) {
			t.Errorf("negative %s: Validate() = %v, want ErrMarginFigureNegative", name, err)
		}
	}
}
