package venue

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Fixture discipline (the nine-instance lesson): every numeric field is
// DISTINCT — u, seq, ts and cts all differ, so a decoder reading one field
// where another belongs cannot produce the expected value by coincidence.
// The payload shapes are the archive's own (2026-08-01 capture, LABUSDT
// linear + BTCUSDT spot, trimmed), and the alt fixture deliberately uses the
// alt's asset tag — the shortlist's streams are what this decoder exists for.

const (
	baseLAB = "LAB"
)

const bybitBookSnapshotFrame = `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790927,` +
	`"data":{"s":"LABUSDT",` +
	`"b":[["0.14640","4789"],["0.14630","11187"]],` +
	`"a":[["0.14650","3437"],["0.14660","11038"]],` +
	`"u":6850738,"seq":163946559126},"cts":1785582790781}`

const bybitBookDeltaFrame = `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790967,` +
	`"data":{"s":"LABUSDT",` +
	`"b":[["0.14540","20735"],["0.14530","0"]],` +
	`"a":[["0.14750","3110"]],` +
	`"u":6850739,"seq":163946559689},"cts":1785582790958}`

func TestDecodeBybitOrderbookSnapshot(t *testing.T) {
	got, ok, err := DecodeBybitOrderbook([]byte(bybitBookSnapshotFrame), baseLAB, quoteUSD)
	if err != nil || !ok {
		t.Fatalf("decode: ok=%t err=%v", ok, err)
	}
	if got.Kind != BybitBookSnapshotKind {
		t.Fatalf("kind = %q, want snapshot", got.Kind)
	}
	// u, not seq — the two are distinct in the fixture.
	if got.UpdateID != 6850738 {
		t.Fatalf("UpdateID = %d, want u = 6850738", got.UpdateID)
	}
	if got.Seq != 163946559126 {
		t.Fatalf("Seq = %d, want seq = 163946559126", got.Seq)
	}
	// ts, not cts — 146ms apart in the fixture.
	want := time.UnixMilli(1785582790927).UTC()
	if !got.SourceTime.Equal(want) {
		t.Fatalf("SourceTime = %v, want ts = %v", got.SourceTime, want)
	}
	if len(got.Snapshot.Bids) != 2 || len(got.Snapshot.Asks) != 2 {
		t.Fatalf("levels = %d/%d, want 2/2", len(got.Snapshot.Bids), len(got.Snapshot.Asks))
	}
	if s := got.Snapshot.Bids[0].Price.String(); !strings.Contains(s, "0.1464") {
		t.Fatalf("best bid = %s, want 0.14640", s)
	}
	if s := got.Snapshot.Asks[1].Quantity.String(); !strings.Contains(s, "11038") {
		t.Fatalf("asks[1] qty = %s, want 11038", s)
	}
	if len(got.Delta.Changes) != 0 {
		t.Fatalf("snapshot minted %d delta changes", len(got.Delta.Changes))
	}
}

func TestDecodeBybitOrderbookDelta(t *testing.T) {
	got, ok, err := DecodeBybitOrderbook([]byte(bybitBookDeltaFrame), baseLAB, quoteUSD)
	if err != nil || !ok {
		t.Fatalf("decode: ok=%t err=%v", ok, err)
	}
	if got.Kind != BybitBookDeltaKind {
		t.Fatalf("kind = %q, want delta", got.Kind)
	}
	if got.UpdateID != 6850739 || got.Seq != 163946559689 {
		t.Fatalf("ids = %d/%d, want 6850739/163946559689", got.UpdateID, got.Seq)
	}
	// The span of length one: FirstSeq is u.
	if got.Delta.FirstSeq != 6850739 {
		t.Fatalf("FirstSeq = %d, want u = 6850739", got.Delta.FirstSeq)
	}
	// Bid changes first, then asks; the middle change is the delete.
	if len(got.Delta.Changes) != 3 {
		t.Fatalf("changes = %d, want 3", len(got.Delta.Changes))
	}
	if c := got.Delta.Changes[0]; c.Side != SideBid {
		t.Fatalf("changes[0].Side = %s, want bid", c.Side)
	}
	del := got.Delta.Changes[1]
	if del.Side != SideBid || !strings.Contains(del.Price.String(), "0.1453") {
		t.Fatalf("changes[1] = %s @ %s, want the bid delete at 0.14530", del.Side, del.Price)
	}
	if sign, err := del.Quantity.Sign(); err != nil || sign != 0 {
		t.Fatalf("delete qty sign = %d (%v), want 0 — size 0 means remove", sign, err)
	}
	if c := got.Delta.Changes[2]; c.Side != SideAsk || !strings.Contains(c.Price.String(), "0.1475") {
		t.Fatalf("changes[2] = %s @ %s, want the ask change at 0.14750", c.Side, c.Price)
	}
	if len(got.Snapshot.Bids) != 0 && len(got.Snapshot.Asks) != 0 {
		t.Fatal("delta minted snapshot levels")
	}
}

// The same bytes mean "resting order" in a snapshot and "removal" in a delta
// — the two frame kinds must NOT share a level parser. A zero-size pair is
// legal in the delta above; here the identical pair inside a snapshot is
// refused, naming the level rule.
func TestBybitBookZeroSizeSplitsByKind(t *testing.T) {
	zeroInSnapshot := `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790901,` +
		`"data":{"s":"LABUSDT","b":[["0.14530","0"]],"a":[],"u":6850741,"seq":163946559801}}`
	if _, _, err := DecodeBybitOrderbook([]byte(zeroInSnapshot), baseLAB, quoteUSD); !errors.Is(err, ErrLevelQuantityNotPositive) {
		t.Fatalf("zero-size level in snapshot: %v, want ErrLevelQuantityNotPositive", err)
	}
}

// Op frames (subscribe acks, pongs) ride the same socket: consumed without
// news, exactly as on the ticker stream.
func TestBybitBookOpChatterConsumed(t *testing.T) {
	ack := `{"success":true,"ret_msg":"","conn_id":"a1b2","req_id":"orderbook.50.LABUSDT","op":"subscribe"}`
	msg, ok, err := DecodeBybitOrderbook([]byte(ack), baseLAB, quoteUSD)
	if err != nil || ok {
		t.Fatalf("op frame: ok=%t err=%v, want consumed silently", ok, err)
	}
	if msg.Kind != "" {
		t.Fatalf("op frame minted kind %q", msg.Kind)
	}
}

func TestBybitBookForeignFrames(t *testing.T) {
	cases := []struct {
		name, frame string
	}{
		// A ticker frame routed here: well-formed Bybit bytes, wrong topic —
		// the reader goes to the wiring, not to the Profiles entry.
		{"ticker topic", `{"topic":"tickers.LABUSDT","type":"snapshot","ts":1785582790903,` +
			`"data":{"symbol":"LABUSDT","fundingRate":"0.0001"}}`},
		{"no topic no op", `{"ts":1785582790904,"data":{}}`},
		{"unknown type word", `{"topic":"orderbook.50.LABUSDT","type":"partial","ts":1785582790905,` +
			`"data":{"s":"LABUSDT","b":[],"a":[],"u":6850742,"seq":163946559902}}`},
	}
	for _, c := range cases {
		if _, _, err := DecodeBybitOrderbook([]byte(c.frame), baseLAB, quoteUSD); !errors.Is(err, ErrWireForeign) {
			t.Fatalf("%s: %v, want ErrWireForeign", c.name, err)
		}
	}
}

func TestBybitBookIncompleteFields(t *testing.T) {
	cases := []struct {
		name, frame string
	}{
		{"zero ts", `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":0,` +
			`"data":{"s":"LABUSDT","b":[],"a":[],"u":6850743,"seq":163946560001}}`},
		{"no symbol", `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790906,` +
			`"data":{"b":[],"a":[],"u":6850744,"seq":163946560002}}`},
		{"zero u", `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790907,` +
			`"data":{"s":"LABUSDT","b":[],"a":[],"u":0,"seq":163946560003}}`},
		{"zero seq", `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790908,` +
			`"data":{"s":"LABUSDT","b":[],"a":[],"u":6850745,"seq":0}}`},
	}
	for _, c := range cases {
		if _, _, err := DecodeBybitOrderbook([]byte(c.frame), baseLAB, quoteUSD); !errors.Is(err, ErrWireIncomplete) {
			t.Fatalf("%s: %v, want ErrWireIncomplete", c.name, err)
		}
	}
}

// u=1 is the venue's restart floor ("resets to 1 during service restarts"),
// so a frame carrying it must DECODE — monotonicity across the reset is the
// consumer's judgement, and a decoder that refused the restart frame would
// blind the reconstruction to the very event it must re-anchor on.
func TestBybitBookRestartFloorDecodes(t *testing.T) {
	restart := `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790909,` +
		`"data":{"s":"LABUSDT","b":[["0.14620","15112"]],"a":[["0.14670","14428"]],"u":1,"seq":163946560101}}`
	got, ok, err := DecodeBybitOrderbook([]byte(restart), baseLAB, quoteUSD)
	if err != nil || !ok {
		t.Fatalf("restart snapshot: ok=%t err=%v", ok, err)
	}
	if got.UpdateID != 1 {
		t.Fatalf("UpdateID = %d, want 1", got.UpdateID)
	}
}

func TestBybitBookUndecodable(t *testing.T) {
	if _, _, err := DecodeBybitOrderbook([]byte("{"), baseLAB, quoteUSD); !errors.Is(err, ErrWireUndecodable) {
		t.Fatalf("truncated json: %v, want ErrWireUndecodable", err)
	}
	garbageData := `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790910,"data":[1,2]}`
	if _, _, err := DecodeBybitOrderbook([]byte(garbageData), baseLAB, quoteUSD); !errors.Is(err, ErrWireUndecodable) {
		t.Fatalf("non-object data: %v, want ErrWireUndecodable", err)
	}
}

// An empty side is legal in BOTH kinds — the archive's own deltas carry
// a:[] constantly, and a halted instrument may snapshot a bare side.
func TestBybitBookEmptySidesDecode(t *testing.T) {
	emptyDelta := `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790911,` +
		`"data":{"s":"LABUSDT","b":[],"a":[],"u":6850746,"seq":163946560201}}`
	got, ok, err := DecodeBybitOrderbook([]byte(emptyDelta), baseLAB, quoteUSD)
	if err != nil || !ok {
		t.Fatalf("empty delta: ok=%t err=%v", ok, err)
	}
	if len(got.Delta.Changes) != 0 {
		t.Fatalf("empty delta produced %d changes", len(got.Delta.Changes))
	}
	emptySnapshot := `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790912,` +
		`"data":{"s":"LABUSDT","b":[],"a":[],"u":6850747,"seq":163946560301}}`
	if _, ok, err := DecodeBybitOrderbook([]byte(emptySnapshot), baseLAB, quoteUSD); err != nil || !ok {
		t.Fatalf("empty snapshot: ok=%t err=%v", ok, err)
	}
}

// The decoder COPIES the venue's ordering rather than repairing it — the
// mirror of TestDepthDecoderDoesNotRepairOrdering, applied per the
// fix-applied-in-one-place rule: the ordering check belongs to the consumer,
// where the refusal can name the instrument.
func TestBybitBookDecoderDoesNotRepairOrdering(t *testing.T) {
	misordered := `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790913,` +
		`"data":{"s":"LABUSDT",` +
		`"b":[["0.14620","15112"],["0.14640","4789"]],` + // ascending — wrong for bids
		`"a":[],"u":6850748,"seq":163946560401}}`
	got, ok, err := DecodeBybitOrderbook([]byte(misordered), baseLAB, quoteUSD)
	if err != nil || !ok {
		t.Fatalf("mis-ordered side refused at decode: ok=%t err=%v", ok, err)
	}
	if s := got.Snapshot.Bids[0].Price.String(); !strings.Contains(s, "0.1462") {
		t.Fatalf("bids[0] = %s; the decoder re-sorted the venue's order", s)
	}
}

// A level that cannot parse refuses the frame, naming side and index — a
// skipped level would price liquidity that is not there.
func TestBybitBookBadLevelRefuses(t *testing.T) {
	badPrice := `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790914,` +
		`"data":{"s":"LABUSDT","b":[["not-a-price","1"]],"a":[],"u":6850749,"seq":163946560501}}`
	if _, _, err := DecodeBybitOrderbook([]byte(badPrice), baseLAB, quoteUSD); err == nil {
		t.Fatal("unparsable price decoded")
	}
}
