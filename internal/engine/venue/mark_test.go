package venue_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

const (
	markBase  = fixedpoint.Asset("BTC")
	markQuote = fixedpoint.Asset("USDT")
)

// The mark value is 64000.5 everywhere, and it is deliberately NOT a value any
// other field in these frames takes: the Binance frame states a funding rate
// too, and a decoder that read `r` into the mark must produce a visibly
// different number rather than a coincidence.
const markText = "64000.5"

func wantMark(t *testing.T, got venue.Mark) {
	t.Helper()
	want, err := fixedpoint.ParsePrice(markBase, markQuote, markText)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	c, err := got.Price.Cmp(want)
	if err != nil {
		t.Fatalf("Cmp: %v", err)
	}
	if c != 0 {
		t.Errorf("mark = %s; want %s", got.Price, want)
	}
	// The asset tags are asserted, not just the number. A decoder that passed
	// (quote, base) in the wrong order yields an identical decimal and a price
	// that means the reciprocal.
	if got.Price.Base() != markBase || got.Price.Quote() != markQuote {
		t.Errorf("price tags = %s/%s; want %s/%s",
			got.Price.Quote(), got.Price.Base(), markQuote, markBase)
	}
}

// ---------------------------------------------------------------------------
// Mark.Validate
// ---------------------------------------------------------------------------

func TestMarkValidateRefusesNonPositive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want error
	}{
		{"zero", "0", venue.ErrMarkNotPositive},
		{"negative", "-1", venue.ErrMarkNotPositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := fixedpoint.ParsePrice(markBase, markQuote, tc.text)
			if err != nil {
				t.Fatalf("ParsePrice(%q): %v", tc.text, err)
			}
			if err := (venue.Mark{Price: p}).Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Validate() = %v; want %v", err, tc.want)
			}
		})
	}

	// The zero VALUE fails too, and by a different route — an unset asset tag
	// rather than a non-positive number. Both are refusals, and asserting only
	// one would leave the other free to become a pass.
	if err := (venue.Mark{}).Validate(); err == nil {
		t.Error("the zero Mark validated")
	}

	// Positive control: a real mark passes. Without it every case above is
	// satisfied by a Validate that refuses everything.
	p, err := fixedpoint.ParsePrice(markBase, markQuote, markText)
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	if err := (venue.Mark{Price: p}).Validate(); err != nil {
		t.Errorf("a real mark was refused: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Binance
// ---------------------------------------------------------------------------

func binanceMarkFrame(mark string) string {
	return `{"e":"markPriceUpdate","E":1785427199000,"s":"BTCUSDT","p":"` + mark +
		`","i":"63999.1","P":"64001.0","r":"0.0001","T":1785427200000}`
}

func TestDecodeBinanceMark(t *testing.T) {
	t.Parallel()

	got, err := venue.DecodeBinanceMark([]byte(binanceMarkFrame(markText)), markBase, markQuote)
	if err != nil {
		t.Fatalf("DecodeBinanceMark: %v", err)
	}
	wantMark(t, got.Mark)
	if got.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q; want BTCUSDT", got.Symbol)
	}
	// The event time is `E`, not `T`. The two are 1s apart in this fixture on
	// purpose: with equal values a decoder reading the settlement instant as the
	// frame's stamp would be indistinguishable.
	if want := time.UnixMilli(1785427199000).UTC(); !got.EventTime.Equal(want) {
		t.Errorf("EventTime = %s; want %s (E, not T)", got.EventTime, want)
	}
}

// The index (`i`) and estimated-settle (`P`) prices sit either side of the mark
// in the fixture, so a decoder reading the wrong key produces a valid price
// that is simply the wrong number — which only an exact assertion catches.
func TestDecodeBinanceMarkReadsPNotINorCapitalP(t *testing.T) {
	t.Parallel()

	got, err := venue.DecodeBinanceMark([]byte(binanceMarkFrame(markText)), markBase, markQuote)
	if err != nil {
		t.Fatalf("DecodeBinanceMark: %v", err)
	}
	for _, other := range []string{"63999.1", "64001.0", "0.0001"} {
		p, err := fixedpoint.ParsePrice(markBase, markQuote, other)
		if err != nil {
			t.Fatalf("ParsePrice(%q): %v", other, err)
		}
		if c, err := got.Mark.Price.Cmp(p); err == nil && c == 0 {
			t.Errorf("mark decoded as %s, which is another field of the same frame", other)
		}
	}
}

// The `p` / `P` collision, pinned by the fixture that makes it order-dependent.
//
// encoding/json falls back to case-INSENSITIVE matching for a key with no
// declared field, so a wire struct declaring only `p` lets `P` — Binance's
// estimated settlement price — land in the mark, and WHICH ONE WINS depends on
// key order in the document. This decoder read `P` before binanceMarkWire
// declared it.
//
// Both orders are asserted. One order alone passes against the broken struct
// whenever the real key happens to come last, which is precisely how an
// order-dependent defect survives a suite.
func TestDecodeBinanceMarkIsNotConfusedByCapitalP(t *testing.T) {
	t.Parallel()

	orders := map[string]string{
		"p before P": `{"e":"markPriceUpdate","E":1785427199000,"s":"BTCUSDT","p":"` + markText + `","P":"64001.0"}`,
		"P before p": `{"e":"markPriceUpdate","E":1785427199000,"s":"BTCUSDT","P":"64001.0","p":"` + markText + `"}`,
	}
	for name, raw := range orders {
		t.Run(name, func(t *testing.T) {
			got, err := venue.DecodeBinanceMark([]byte(raw), markBase, markQuote)
			if err != nil {
				t.Fatalf("DecodeBinanceMark: %v", err)
			}
			wantMark(t, got.Mark)
		})
	}
}

func TestDecodeBinanceMarkRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want error
	}{
		{"not json", `{`, venue.ErrWireUndecodable},
		{
			"foreign event",
			`{"e":"kline","E":1785427199000,"s":"BTCUSDT","p":"64000.5"}`,
			venue.ErrWireForeign,
		},
		{
			"no symbol",
			`{"e":"markPriceUpdate","E":1785427199000,"s":"","p":"64000.5"}`,
			venue.ErrWireIncomplete,
		},
		{
			"no mark",
			`{"e":"markPriceUpdate","E":1785427199000,"s":"BTCUSDT","p":""}`,
			venue.ErrWireIncomplete,
		},
		{
			"no event time",
			`{"e":"markPriceUpdate","E":0,"s":"BTCUSDT","p":"64000.5"}`,
			venue.ErrWireIncomplete,
		},
		{
			// The venue-business refusal, reached through the wire. It is here
			// rather than only on Mark.Validate because the decoder is what a
			// live feed calls, and a zero arriving from the wire is the case
			// that matters.
			"zero mark",
			binanceMarkFrame("0"),
			venue.ErrMarkNotPositive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := venue.DecodeBinanceMark([]byte(tc.raw), markBase, markQuote); !errors.Is(err, tc.want) {
				t.Errorf("err = %v; want %v", err, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Bybit
// ---------------------------------------------------------------------------

func bybitTickerFrame(typ, data string) string {
	return `{"topic":"tickers.BTCUSDT","type":"` + typ + `","ts":1785427199000,"data":` + data + `}`
}

func TestDecodeBybitMark(t *testing.T) {
	t.Parallel()

	got, ok, err := venue.DecodeBybitMark(
		[]byte(bybitTickerFrame("snapshot", `{"symbol":"BTCUSDT","markPrice":"`+markText+`","indexPrice":"63999.1","fundingRate":"0.0001"}`)),
		markBase, markQuote)
	if err != nil {
		t.Fatalf("DecodeBybitMark: %v", err)
	}
	if !ok {
		t.Fatal("a snapshot carrying markPrice reported no mark")
	}
	wantMark(t, got.Mark)
	if want := time.UnixMilli(1785427199000).UTC(); !got.SourceTime.Equal(want) {
		t.Errorf("SourceTime = %s; want %s", got.SourceTime, want)
	}
}

// The mark needs NO snapshot first, unlike funding. A delta carrying markPrice
// is a complete reading, and waiting for a snapshot would blind §7.1's trigger
// for the whole window after a reconnect.
func TestDecodeBybitMarkAcceptsADeltaWithNoPrecedingSnapshot(t *testing.T) {
	t.Parallel()

	got, ok, err := venue.DecodeBybitMark(
		[]byte(bybitTickerFrame("delta", `{"symbol":"BTCUSDT","markPrice":"`+markText+`"}`)),
		markBase, markQuote)
	if err != nil {
		t.Fatalf("DecodeBybitMark: %v", err)
	}
	if !ok {
		t.Fatal("a delta carrying markPrice reported no mark — the mark has no state to wait for")
	}
	wantMark(t, got.Mark)
}

// The asymmetry that is the whole point of the split, asserted as a PAIR: a
// delta with no markPrice is silence, a snapshot with none is a refusal.
// Testing either alone would pass against an implementation that treated both
// the same, in whichever direction.
func TestDecodeBybitMarkAbsenceIsSilenceOnDeltaAndRefusalOnSnapshot(t *testing.T) {
	t.Parallel()

	fundingOnly := `{"symbol":"BTCUSDT","fundingRate":"0.0001"}`

	got, ok, err := venue.DecodeBybitMark([]byte(bybitTickerFrame("delta", fundingOnly)), markBase, markQuote)
	if err != nil {
		t.Errorf("a mark-less DELTA errored: %v — it is the ordinary majority of the stream", err)
	}
	if ok {
		t.Errorf("a mark-less delta reported a mark: %+v", got)
	}

	if _, ok, err := venue.DecodeBybitMark([]byte(bybitTickerFrame("snapshot", fundingOnly)), markBase, markQuote); !errors.Is(err, venue.ErrWireIncomplete) {
		t.Errorf("a mark-less SNAPSHOT: err = %v, ok = %t; want ErrWireIncomplete — "+
			"a snapshot claims full ticker state", err, ok)
	}
}

func TestDecodeBybitMarkRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     string
		want    error
		wantErr bool
	}{
		{"not json", `{`, venue.ErrWireUndecodable, true},
		{
			// Transport chatter riding the same socket: consumed without news,
			// never an error. This is the one case where ok=false and err=nil
			// is the CORRECT answer rather than a swallowed fault.
			"op acknowledgement",
			`{"op":"subscribe","success":true}`,
			nil, false,
		},
		{
			"no topic and no op",
			`{"ts":1785427199000}`,
			venue.ErrWireForeign, true,
		},
		{
			"foreign frame type",
			`{"topic":"tickers.BTCUSDT","type":"whatever","ts":1785427199000,"data":{}}`,
			venue.ErrWireForeign, true,
		},
		{
			"no venue stamp",
			`{"topic":"tickers.BTCUSDT","type":"delta","ts":0,"data":{}}`,
			venue.ErrWireIncomplete, true,
		},
		{
			"zero mark",
			bybitTickerFrame("delta", `{"markPrice":"0"}`),
			venue.ErrMarkNotPositive, true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := venue.DecodeBybitMark([]byte(tc.raw), markBase, markQuote)
			if tc.wantErr {
				if !errors.Is(err, tc.want) {
					t.Errorf("err = %v; want %v", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Errorf("err = %v; want none", err)
			}
			if ok {
				t.Error("reported a mark where there is none")
			}
		})
	}
}
