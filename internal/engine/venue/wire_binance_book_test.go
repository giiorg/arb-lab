package venue

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Fixture discipline (the nine-instance lesson): every numeric field is
// DISTINCT — lastUpdateId, u, E, every price and every quantity — so a
// decoder that reads one field where another belongs cannot produce the
// expected value by coincidence. The payload shapes are the archive's own
// (2026-07-31 capture, trimmed to two levels).

const (
	baseBTC  = fixedpoint.Asset("BTC")
	quoteUSD = fixedpoint.Asset("USDT")
)

const spotDepthFrame = `{"lastUpdateId":98071966646,` +
	`"bids":[["64294.78000000","0.96889000"],["64294.77000000","0.00098000"]],` +
	`"asks":[["64294.79000000","0.51000000"],["64295.01000000","0.02418000"]]}`

const futuresDepthFrame = `{"e":"depthUpdate","E":1785471501803,"T":1785471501800,` +
	`"s":"BTCUSDT","ps":"BTCUSDT","U":11177881911418,"u":11177881916526,"pu":11177881911348,` +
	`"b":[["64266.60","10.287"],["64266.50","0.039"]],` +
	`"a":[["64266.70","5.100"],["64266.80","0.017"]]}`

func TestDecodeBinanceSpotDepth20(t *testing.T) {
	got, err := DecodeBinanceSpotDepth20([]byte(spotDepthFrame), baseBTC, quoteUSD)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.LastUpdateID != 98071966646 {
		t.Fatalf("lastUpdateId = %d, want 98071966646", got.LastUpdateID)
	}
	if !got.EventTime.IsZero() {
		t.Fatalf("spot frame minted an event time %v; the wire carries none", got.EventTime)
	}
	if len(got.Book.Bids) != 2 || len(got.Book.Asks) != 2 {
		t.Fatalf("levels = %d/%d, want 2/2", len(got.Book.Bids), len(got.Book.Asks))
	}
	if s := got.Book.Bids[0].Price.String(); !strings.Contains(s, "64294.78") {
		t.Fatalf("best bid = %s, want 64294.78", s)
	}
	if s := got.Book.Asks[0].Quantity.String(); !strings.Contains(s, "0.51") {
		t.Fatalf("best ask qty = %s, want 0.51", s)
	}
}

// The u-vs-U claim must hold independent of KEY ORDER: encoding/json's
// case-insensitive fallback would let "u" and "U" share a destination,
// making the last key in the byte stream win — so a frame with "u" FIRST
// must still answer u. (The survived-mutation finding: the wire struct
// declares "U" its own field precisely so each key has an exact match.)
func TestFuturesDepthPositionIndependentOfKeyOrder(t *testing.T) {
	reordered := `{"e":"depthUpdate","E":1785471501803,"s":"BTCUSDT",` +
		`"u":11177881916526,"pu":11177881911348,"U":11177881911418,` +
		`"b":[["64266.60","10.287"]],"a":[["64266.70","5.100"]]}`
	got, err := DecodeBinanceFuturesDepth20([]byte(reordered), baseBTC, quoteUSD)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.LastUpdateID != 11177881916526 {
		t.Fatalf("LastUpdateID = %d, want u = 11177881916526 (key order must not matter)", got.LastUpdateID)
	}
}

func TestDecodeBinanceFuturesDepth20(t *testing.T) {
	got, err := DecodeBinanceFuturesDepth20([]byte(futuresDepthFrame), baseBTC, quoteUSD)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// u, not U and not pu — the three ids are distinct in the fixture so a
	// wrong pick cannot pass.
	if got.LastUpdateID != 11177881916526 {
		t.Fatalf("LastUpdateID = %d, want u = 11177881916526", got.LastUpdateID)
	}
	// E, not T — one field is 3ms after the other.
	want := time.UnixMilli(1785471501803).UTC()
	if !got.EventTime.Equal(want) {
		t.Fatalf("EventTime = %v, want E = %v", got.EventTime, want)
	}
	if s := got.Book.Asks[0].Price.String(); !strings.Contains(s, "64266.7") {
		t.Fatalf("best ask = %s, want 64266.7", s)
	}
}

// A misfiled file decodes under the WRONG dialect: each decoder must refuse
// the other's bytes as foreign, not absorb them.
func TestDepthDecodersRefuseForeignDialect(t *testing.T) {
	// The futures frame under the spot decoder: its position field is
	// spelled `u`, so lastUpdateId decodes to zero.
	if _, err := DecodeBinanceSpotDepth20([]byte(futuresDepthFrame), baseBTC, quoteUSD); !errors.Is(err, ErrWireForeign) {
		t.Fatalf("spot decoder on futures bytes: %v, want ErrWireForeign", err)
	}
	// The spot frame under the futures decoder: no event token.
	if _, err := DecodeBinanceFuturesDepth20([]byte(spotDepthFrame), baseBTC, quoteUSD); !errors.Is(err, ErrWireForeign) {
		t.Fatalf("futures decoder on spot bytes: %v, want ErrWireForeign", err)
	}
	// A markPrice frame under the futures decoder: wrong event token.
	mark := `{"e":"markPriceUpdate","E":1785471501000,"s":"BTCUSDT","r":"0.0001","T":1785500400000}`
	if _, err := DecodeBinanceFuturesDepth20([]byte(mark), baseBTC, quoteUSD); !errors.Is(err, ErrWireForeign) {
		t.Fatalf("futures decoder on markPrice bytes: %v, want ErrWireForeign", err)
	}
}

func TestFuturesDepthIncompleteFields(t *testing.T) {
	cases := []struct {
		name, frame string
	}{
		{"zero E", `{"e":"depthUpdate","E":0,"s":"BTCUSDT","u":7,"b":[],"a":[]}`},
		{"zero u", `{"e":"depthUpdate","E":1785471501803,"s":"BTCUSDT","u":0,"b":[],"a":[]}`},
		{"no symbol", `{"e":"depthUpdate","E":1785471501803,"u":7,"b":[],"a":[]}`},
	}
	for _, c := range cases {
		if _, err := DecodeBinanceFuturesDepth20([]byte(c.frame), baseBTC, quoteUSD); !errors.Is(err, ErrWireIncomplete) {
			t.Fatalf("%s: %v, want ErrWireIncomplete", c.name, err)
		}
	}
}

func TestDepthLevelRefusals(t *testing.T) {
	// A zero-quantity level in a SNAPSHOT is not a resting order —
	// BookLevel.Validate's refusal must surface, naming the side.
	zeroQty := `{"lastUpdateId":9,"bids":[["64294.78","0"]],"asks":[]}`
	if _, err := DecodeBinanceSpotDepth20([]byte(zeroQty), baseBTC, quoteUSD); !errors.Is(err, ErrLevelQuantityNotPositive) {
		t.Fatalf("zero-qty level: %v, want ErrLevelQuantityNotPositive", err)
	}
	// An unparsable price is fixedpoint's refusal, not a skipped level.
	badPrice := `{"lastUpdateId":9,"bids":[["not-a-price","1.0"]],"asks":[]}`
	if _, err := DecodeBinanceSpotDepth20([]byte(badPrice), baseBTC, quoteUSD); err == nil {
		t.Fatal("unparsable price decoded")
	}
	// Undecodable bytes.
	if _, err := DecodeBinanceSpotDepth20([]byte("{"), baseBTC, quoteUSD); !errors.Is(err, ErrWireUndecodable) {
		t.Fatalf("truncated json: %v, want ErrWireUndecodable", err)
	}
	if _, err := DecodeBinanceFuturesDepth20([]byte("{"), baseBTC, quoteUSD); !errors.Is(err, ErrWireUndecodable) {
		t.Fatalf("truncated json: %v, want ErrWireUndecodable", err)
	}
}

// The decoder COPIES the venue's ordering rather than repairing it: a
// mis-ordered side must decode cleanly here — the ordering check belongs to
// marketdata's domain, where the refusal can name the instrument. This test
// pins the deliberate absence of a sort.
func TestDepthDecoderDoesNotRepairOrdering(t *testing.T) {
	misordered := `{"lastUpdateId":9,` +
		`"bids":[["64294.77","0.5"],["64294.78","0.7"]],` + // ascending — wrong for bids
		`"asks":[]}`
	got, err := DecodeBinanceSpotDepth20([]byte(misordered), baseBTC, quoteUSD)
	if err != nil {
		t.Fatalf("mis-ordered side refused at decode: %v", err)
	}
	if s := got.Book.Bids[0].Price.String(); !strings.Contains(s, "64294.77") {
		t.Fatalf("bids[0] = %s; the decoder re-sorted the venue's order", s)
	}
}

// An empty side is legal — a halted instrument publishes nothing on a side.
func TestDepthEmptySideDecodes(t *testing.T) {
	empty := `{"lastUpdateId":9,"bids":[],"asks":[["64294.79","0.51"]]}`
	got, err := DecodeBinanceSpotDepth20([]byte(empty), baseBTC, quoteUSD)
	if err != nil {
		t.Fatalf("empty bid side: %v", err)
	}
	if len(got.Book.Bids) != 0 || len(got.Book.Asks) != 1 {
		t.Fatalf("levels = %d/%d, want 0/1", len(got.Book.Bids), len(got.Book.Asks))
	}
}
