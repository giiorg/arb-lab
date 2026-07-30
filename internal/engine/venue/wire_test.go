package venue_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
)

// --- Binance markPrice -------------------------------------------------------

// A frame as the archive holds it (hour-15 fixture shapes): event token,
// epoch-ms integers, string rate.
const binanceFrame = `{"e":"markPriceUpdate","E":1785425440415,"s":"BTCUSDT","p":"64550.10","i":"64577.73","P":"64560.00","r":"0.00010000","T":1785427200000}`

func TestDecodeBinanceMarkPrice(t *testing.T) {
	m, err := venue.DecodeBinanceMarkPrice([]byte(binanceFrame))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Symbol != "BTCUSDT" {
		t.Errorf("symbol = %q", m.Symbol)
	}
	if want := time.UnixMilli(1785427200000).UTC(); !m.NextSettle.Equal(want) {
		t.Errorf("NextSettle = %v, want %v", m.NextSettle, want)
	}
	if want := time.UnixMilli(1785425440415).UTC(); !m.EventTime.Equal(want) {
		t.Errorf("EventTime = %v, want %v", m.EventTime, want)
	}
	if want := rate(t, "0.0001"); m.Rate != want {
		t.Errorf("Rate = %v, want %v", m.Rate, want)
	}
}

func TestDecodeBinanceMarkPriceRefusals(t *testing.T) {
	cases := []struct {
		name, frame string
		want        error
	}{
		{"not json", `{`, venue.ErrWireUndecodable},
		{"foreign event", `{"e":"kline","E":1,"s":"BTCUSDT","r":"0.0001","T":1}`, venue.ErrWireForeign},
		{"no symbol", `{"e":"markPriceUpdate","E":1,"r":"0.0001","T":1}`, venue.ErrWireIncomplete},
		{"no next settle", `{"e":"markPriceUpdate","E":1,"s":"BTCUSDT","r":"0.0001"}`, venue.ErrWireIncomplete},
		{"no event time", `{"e":"markPriceUpdate","s":"BTCUSDT","r":"0.0001","T":1}`, venue.ErrWireIncomplete},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := venue.DecodeBinanceMarkPrice([]byte(c.frame)); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
	// A malformed rate is fixedpoint's refusal, not a wire sentinel — the
	// money boundary names itself.
	if _, err := venue.DecodeBinanceMarkPrice([]byte(`{"e":"markPriceUpdate","E":1,"s":"BTCUSDT","r":"abc","T":1}`)); err == nil {
		t.Error("malformed rate accepted")
	}
}

// --- fundingInfo overlay -----------------------------------------------------

// The measured response shape (Profiles §5, 2026-07-29): updateTime is NULL
// for BTCUSDT and an epoch-ms integer elsewhere — one array holding both is
// the nullable trap a typed field dies on.
const fundingInfoBody = `[
	{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.00300","adjustedFundingRateFloor":"-0.00300","fundingIntervalHours":8,"disclaimer":true,"updateTime":null},
	{"symbol":"ALTUSDT","adjustedFundingRateCap":"0.02","adjustedFundingRateFloor":"-0.02","fundingIntervalHours":4,"disclaimer":false,"updateTime":1785425440000}
]`

func mark(t *testing.T, symbol string) venue.BinanceMarkPrice {
	t.Helper()
	return venue.BinanceMarkPrice{
		Symbol:     symbol,
		Rate:       rate(t, "0.0001"),
		NextSettle: time.UnixMilli(1785427200000).UTC(),
		EventTime:  time.UnixMilli(1785425440415).UTC(),
	}
}

func TestFundingInfoOverlayComposes(t *testing.T) {
	o := venue.NewFundingInfoOverlay()
	if err := o.Apply([]byte(fundingInfoBody)); err != nil {
		t.Fatalf("apply: %v", err)
	}

	f, ok, err := o.Compose(mark(t, "BTCUSDT"))
	if err != nil || !ok {
		t.Fatalf("compose = (%v, %v)", ok, err)
	}
	if f.Interval != 8*time.Hour {
		t.Errorf("interval = %v, want 8h", f.Interval)
	}
	if want := rate(t, "0.003"); f.Cap != want {
		t.Errorf("cap = %v, want %v", f.Cap, want)
	}
	if want := rate(t, "-0.003"); f.Floor != want {
		t.Errorf("floor = %v, want %v", f.Floor, want)
	}
	// The 4h row must not bleed into the 8h symbol — the row match is exact.
	f4, ok, err := o.Compose(mark(t, "ALTUSDT"))
	if err != nil || !ok {
		t.Fatalf("alt compose = (%v, %v)", ok, err)
	}
	if f4.Interval != 4*time.Hour {
		t.Errorf("alt interval = %v, want 4h", f4.Interval)
	}
}

func TestFundingInfoOverlayAbsentSymbolEmitsNothing(t *testing.T) {
	o := venue.NewFundingInfoOverlay()

	// Cold: no answer applied at all.
	if _, ok, err := o.Compose(mark(t, "BTCUSDT")); ok || err != nil {
		t.Fatalf("cold compose = (%v, %v), want (false, nil)", ok, err)
	}

	// Applied, but the symbol is not in the answer (§39p-2: no default).
	if err := o.Apply([]byte(fundingInfoBody)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok, err := o.Compose(mark(t, "ETHUSDT")); ok || err != nil {
		t.Fatalf("absent-symbol compose = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestFundingInfoOverlayApplyIsAllOrNothing(t *testing.T) {
	o := venue.NewFundingInfoOverlay()
	if err := o.Apply([]byte(fundingInfoBody)); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// A later answer with one malformed row must leave the previous state
	// standing — a half-applied list is a state no poll produced.
	bad := `[{"symbol":"BTCUSDT","adjustedFundingRateCap":"nope","adjustedFundingRateFloor":"-0.003","fundingIntervalHours":8}]`
	if err := o.Apply([]byte(bad)); err == nil {
		t.Fatal("malformed row accepted")
	}
	if _, ok, err := o.Compose(mark(t, "BTCUSDT")); !ok || err != nil {
		t.Fatalf("previous state lost after refused apply: (%v, %v)", ok, err)
	}

	// A zero interval is refused through the same door — the unit rule.
	zero := `[{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.003","adjustedFundingRateFloor":"-0.003","fundingIntervalHours":0}]`
	if err := o.Apply([]byte(zero)); !errors.Is(err, venue.ErrIntervalInvalid) {
		t.Errorf("zero interval: err = %v, want ErrIntervalInvalid", err)
	}

	// A full replace drops symbols the venue withdrew.
	only := `[{"symbol":"ALTUSDT","adjustedFundingRateCap":"0.02","adjustedFundingRateFloor":"-0.02","fundingIntervalHours":4}]`
	if err := o.Apply([]byte(only)); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, ok, _ := o.Compose(mark(t, "BTCUSDT")); ok {
		t.Error("withdrawn symbol still composes — apply merged instead of replacing")
	}
}

// --- Bybit ticker ------------------------------------------------------------

// The archive's own snapshot shape (hour-09 seq 1, trimmed): every value a
// string, fundingIntervalHour "8", nextFundingTime epoch-ms as a string.
const bybitSnapshot = `{"topic":"tickers.BTCUSDT","type":"snapshot","data":{"symbol":"BTCUSDT","markPrice":"64551.03","fundingIntervalHour":"8","fundingCap":"0.005","nextFundingTime":"1785427200000","fundingRate":"0.00008114","bid1Price":"64550.10"},"cs":726745355965,"ts":1785405239383}`

func TestBybitTickerSnapshotEmits(t *testing.T) {
	s := venue.NewBybitTickerState()
	u, emitted, err := s.Apply([]byte(bybitSnapshot))
	if err != nil || !emitted {
		t.Fatalf("snapshot: (%v, %v)", emitted, err)
	}
	if want := rate(t, "0.00008114"); u.Funding.Rate != want {
		t.Errorf("rate = %v, want %v", u.Funding.Rate, want)
	}
	if u.Funding.Interval != 8*time.Hour {
		t.Errorf("interval = %v, want 8h", u.Funding.Interval)
	}
	if want := time.UnixMilli(1785427200000).UTC(); !u.Funding.NextSettle.Equal(want) {
		t.Errorf("next settle = %v, want %v", u.Funding.NextSettle, want)
	}
	if want := rate(t, "0.005"); u.Funding.Cap != want {
		t.Errorf("cap = %v, want %v", u.Funding.Cap, want)
	}
	// The floor is the cap mirrored — the dated instruments-info measurement
	// (2026-07-30: upper 0.005 / lower -0.005, exactly symmetric).
	if want := rate(t, "-0.005"); u.Funding.Floor != want {
		t.Errorf("floor = %v, want %v", u.Funding.Floor, want)
	}
	if want := time.UnixMilli(1785405239383).UTC(); !u.SourceTime.Equal(want) {
		t.Errorf("source time = %v, want %v", u.SourceTime, want)
	}
}

func TestBybitTickerDeltaSemantics(t *testing.T) {
	s := venue.NewBybitTickerState()
	if _, _, err := s.Apply([]byte(bybitSnapshot)); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// A non-funding delta is consumed without news — most of the stream.
	quiet := `{"topic":"tickers.BTCUSDT","type":"delta","data":{"symbol":"BTCUSDT","bid1Price":"64757.90","ask1Price":"64758.00"},"cs":1,"ts":1785405240000}`
	if _, emitted, err := s.Apply([]byte(quiet)); emitted || err != nil {
		t.Fatalf("non-funding delta: (%v, %v), want (false, nil)", emitted, err)
	}

	// A funding-bearing delta emits the MERGED state: the new rate rides with
	// the held interval/cap/settlement — Profiles §1's absent-field-means-
	// unchanged rule made observable.
	bump := `{"topic":"tickers.BTCUSDT","type":"delta","data":{"symbol":"BTCUSDT","fundingRate":"0.00009626","bid1Price":"64762.40"},"cs":2,"ts":1785405241000}`
	u, emitted, err := s.Apply([]byte(bump))
	if err != nil || !emitted {
		t.Fatalf("funding delta: (%v, %v)", emitted, err)
	}
	if want := rate(t, "0.00009626"); u.Funding.Rate != want {
		t.Errorf("rate = %v, want %v", u.Funding.Rate, want)
	}
	if u.Funding.Interval != 8*time.Hour {
		t.Errorf("interval lost across delta merge: %v", u.Funding.Interval)
	}
	if want := rate(t, "0.005"); u.Funding.Cap != want {
		t.Errorf("cap lost across delta merge: %v", u.Funding.Cap)
	}
}

func TestBybitTickerColdStartEmitsNothing(t *testing.T) {
	s := venue.NewBybitTickerState()
	// A funding-bearing delta BEFORE any snapshot — the rotation-boundary
	// cold start (§39p-2): absorbed, but state is incomplete, so no reading.
	bump := `{"topic":"tickers.BTCUSDT","type":"delta","data":{"symbol":"BTCUSDT","fundingRate":"0.00009626"},"cs":2,"ts":1785405241000}`
	if _, emitted, err := s.Apply([]byte(bump)); emitted || err != nil {
		t.Fatalf("cold funding delta: (%v, %v), want (false, nil)", emitted, err)
	}
	// The absorbed field is not thrown away: the remaining three arriving
	// completes the state without a snapshot ever appearing.
	rest := `{"topic":"tickers.BTCUSDT","type":"delta","data":{"symbol":"BTCUSDT","nextFundingTime":"1785427200000","fundingIntervalHour":"8","fundingCap":"0.005"},"cs":3,"ts":1785405242000}`
	u, emitted, err := s.Apply([]byte(rest))
	if err != nil || !emitted {
		t.Fatalf("completing delta: (%v, %v)", emitted, err)
	}
	if want := rate(t, "0.00009626"); u.Funding.Rate != want {
		t.Errorf("rate absorbed pre-completion was lost: %v", u.Funding.Rate)
	}
}

func TestBybitTickerRefusals(t *testing.T) {
	cases := []struct {
		name, frame string
		want        error
	}{
		{"not json", `{`, venue.ErrWireUndecodable},
		{"no topic no op", `{"ts":1,"type":"delta","data":{}}`, venue.ErrWireForeign},
		{"unknown type", `{"topic":"tickers.BTCUSDT","type":"stream","data":{},"ts":1}`, venue.ErrWireForeign},
		{"no ts", `{"topic":"tickers.BTCUSDT","type":"delta","data":{}}`, venue.ErrWireIncomplete},
		{"snapshot missing funding state", `{"topic":"tickers.BTCUSDT","type":"snapshot","data":{"symbol":"BTCUSDT","fundingRate":"0.0001"},"ts":1}`, venue.ErrWireIncomplete},
		{"bad interval", `{"topic":"tickers.BTCUSDT","type":"delta","data":{"fundingIntervalHour":"eight"},"ts":1}`, venue.ErrWireIncomplete},
		{"bad next funding", `{"topic":"tickers.BTCUSDT","type":"delta","data":{"nextFundingTime":"soon"},"ts":1}`, venue.ErrWireIncomplete},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := venue.NewBybitTickerState()
			if _, _, err := s.Apply([]byte(c.frame)); !errors.Is(err, c.want) {
				t.Errorf("err = %v, want %v", err, c.want)
			}
		})
	}
}

// TestBybitTickerResubscribeSnapshotResets is the one place the snapshot
// reset is observable: on a cold accumulator, reset and no-reset agree, so
// every refusal case above is blind to it. Warm state first, then a snapshot
// claiming full state while missing one funding field — the reset makes that
// a refusal; a merge would emit the PREVIOUS connection's cap as current,
// which is the null-coerce mistake with extra steps (the wire_bybit comment's
// own words, here as the failing test they imply).
func TestBybitTickerResubscribeSnapshotResets(t *testing.T) {
	s := venue.NewBybitTickerState()
	if _, _, err := s.Apply([]byte(bybitSnapshot)); err != nil {
		t.Fatalf("warm-up snapshot: %v", err)
	}
	partial := `{"topic":"tickers.BTCUSDT","type":"snapshot","data":{"symbol":"BTCUSDT","fundingRate":"0.0001","nextFundingTime":"1785427200000","fundingIntervalHour":"8"},"ts":1785405243000}`
	_, emitted, err := s.Apply([]byte(partial))
	if emitted || !errors.Is(err, venue.ErrWireIncomplete) {
		t.Errorf("post-complete partial snapshot: (%v, %v), want a refusal — a stale cap emitted as current is the defect", emitted, err)
	}
}

func TestBybitTickerOpFrameIsChatter(t *testing.T) {
	s := venue.NewBybitTickerState()
	ack := `{"success":true,"ret_msg":"","conn_id":"x","req_id":"tickers.BTCUSDT","op":"subscribe"}`
	if _, emitted, err := s.Apply([]byte(ack)); emitted || err != nil {
		t.Fatalf("op frame: (%v, %v), want (false, nil)", emitted, err)
	}
}
