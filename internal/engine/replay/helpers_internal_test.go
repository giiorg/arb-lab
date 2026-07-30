package replay

// The battery's shared machinery: a crafted archive small enough to read, the
// §39p-8 "fixture 2" — hand-written minimal valid venue JSON wrapped through
// capture's own codec, with cross-stream equal-RecvTime records to force
// multi-source batches, a delta that omits funding fields, one that changes
// them, and a failed poll — every distinction the decoders claim to make,
// present in a fixture whose every instant is named below.
//
// Tests are in-package deliberately: the tape's merge comparator, the
// tapeEntry unit and match() are exactly the constructs §39p-10's mutation
// roster aims at, and driving them only through Load/Generate would leave the
// CaptureSeq tie-break arm untestable — within one stream, file order IS seq
// order, so the stable sort never needs the arm on honestly-built input.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// battBase anchors every crafted instant. A fixed past instant, never derived
// from the wall clock — the tests' time domain is the crafted archive's, the
// same rule the sim lives by.
func battBase() time.Time {
	return time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC)
}

// at is battBase()+d, the only spelling of an instant the fixtures use.
func at(d time.Duration) time.Time { return battBase().Add(d) }

func binancePerpInst() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}
}

func bybitPerpInst() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}
}

func binanceMarkKey() capture.StreamKey {
	return capture.StreamKey{
		Source: events.SourceID{Instrument: binancePerpInst(), Stream: events.StreamFunding},
		Topic:  "btcusdt@markPrice@1s",
	}
}

func binanceInfoKey() capture.StreamKey {
	return capture.StreamKey{
		Source: events.SourceID{Instrument: binancePerpInst(), Stream: events.StreamFunding},
		Topic:  "fundingInfo",
	}
}

func bybitTickerKey() capture.StreamKey {
	return capture.StreamKey{
		Source: events.SourceID{Instrument: bybitPerpInst(), Stream: events.StreamFunding},
		Topic:  "tickers.BTCUSDT",
	}
}

// canonicalSpecs is the roster every canonical-archive test loads, in a fixed
// order so stream indices are stable: 0 = markPrice, 1 = fundingInfo,
// 2 = bybit ticker.
func canonicalSpecs() []StreamSpec {
	return []StreamSpec{
		{Key: binanceMarkKey(), Decoder: DecodeBinanceMarkPrice},
		{Key: binanceInfoKey(), Decoder: DecodeBinanceFundingInfo},
		{Key: bybitTickerKey(), Decoder: DecodeBybitTicker},
	}
}

// The wire payloads, spelled as the venues spell them (wire_test.go is the
// authority on the shapes; these restate only what each test needs to vary).

func bybitSnapJSON(ts time.Time, rate string) []byte {
	next := strconv.FormatInt(at(time.Hour).UnixMilli(), 10)
	return []byte(fmt.Sprintf(
		`{"topic":"tickers.BTCUSDT","type":"snapshot","ts":%d,"data":{"symbol":"BTCUSDT","fundingRate":%q,"nextFundingTime":%q,"fundingIntervalHour":"8","fundingCap":"0.005"}}`,
		ts.UnixMilli(), rate, next))
}

func bybitRateDeltaJSON(ts time.Time, rate string) []byte {
	return []byte(fmt.Sprintf(
		`{"topic":"tickers.BTCUSDT","type":"delta","ts":%d,"data":{"symbol":"BTCUSDT","fundingRate":%q}}`,
		ts.UnixMilli(), rate))
}

// bybitQuoteDeltaJSON is funding-silent: bid churn, the ~10/s majority the
// emission gate exists to not dispatch.
func bybitQuoteDeltaJSON(ts time.Time) []byte {
	return []byte(fmt.Sprintf(
		`{"topic":"tickers.BTCUSDT","type":"delta","ts":%d,"data":{"symbol":"BTCUSDT","bid1Price":"50000"}}`,
		ts.UnixMilli()))
}

// bybitChatterJSON is transport chatter riding the same socket: no topic, an
// op token — consumed without news, ordinal still burned.
func bybitChatterJSON() []byte {
	return []byte(`{"op":"subscribe","conn_id":"crafted","success":true}`)
}

func binanceMarkJSON(event time.Time, rate string) []byte {
	return []byte(fmt.Sprintf(
		`{"e":"markPriceUpdate","E":%d,"s":"BTCUSDT","r":%q,"T":%d}`,
		event.UnixMilli(), rate, at(time.Hour).UnixMilli()))
}

const fundingInfoBodyJSON = `[{"symbol":"BTCUSDT","adjustedFundingRateCap":"0.00300","adjustedFundingRateFloor":"-0.00300","fundingIntervalHours":8}]`

// fundingInfoBodyWithoutSymbol answers with a real list that lacks our row —
// §39p-2's absent case, which must compose to silence, not to a default.
const fundingInfoBodyWithoutSymbol = `[{"symbol":"ETHUSDT","adjustedFundingRateCap":"0.00300","adjustedFundingRateFloor":"-0.00300","fundingIntervalHours":8}]`

func restOKPayload(t *testing.T, asked time.Time, body string) []byte {
	t.Helper()
	p, err := capture.EncodeRestPayload(capture.RestPayload{
		Meta: capture.RestMeta{Method: "GET", URL: "https://fapi.binance.com/fapi/v1/fundingInfo", Status: 200, AskedAt: asked},
		Body: []byte(body),
	})
	if err != nil {
		t.Fatalf("encode rest payload: %v", err)
	}
	return p
}

func restFailedPayload(t *testing.T, asked time.Time) []byte {
	t.Helper()
	p, err := capture.EncodeRestPayload(capture.RestPayload{
		Meta: capture.RestMeta{Method: "GET", URL: "https://fapi.binance.com/fapi/v1/fundingInfo", Error: "dial timeout", AskedAt: asked},
	})
	if err != nil {
		t.Fatalf("encode failed rest payload: %v", err)
	}
	return p
}

func dataRec(seq uint64, recv time.Time, payload []byte) capture.Record {
	return capture.Record{Kind: capture.RecordKindData, CaptureSeq: seq, RecvTime: recv, Payload: payload}
}

func restRec(seq uint64, recv time.Time, payload []byte) capture.Record {
	return capture.Record{Kind: capture.RecordKindRest, CaptureSeq: seq, RecvTime: recv, Payload: payload}
}

// writeStreamFile lands recs as one archive file in the window holding the
// first record — through capture's own codec, so the fixture is real-format by
// construction rather than by intention.
func writeStreamFile(t *testing.T, root string, key capture.StreamKey, recs []capture.Record) {
	t.Helper()
	if len(recs) == 0 {
		t.Fatalf("writeStreamFile: no records for %s", key)
	}
	window := capture.Window(recs[0].RecvTime, time.Hour)
	path := key.FilePath(root, window, time.Hour)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	buf := capture.AppendFileHeader(nil)
	var err error
	for _, r := range recs {
		if buf, err = capture.AppendRecord(buf, r); err != nil {
			t.Fatalf("append record seq %d: %v", r.CaptureSeq, err)
		}
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// The canonical archive: seven records over four seconds, every §39p-10
// distinction present. Instants (offsets from battBase):
//
//	40.0s  fundingInfo 200 answer  ┐ equal instant — a cross-stream tie the
//	40.0s  bybit snapshot          ┘ merge must order by stream key
//	41.0s  binance markPrice                      (emits: overlay applied)
//	42.0s  binance markPrice  ┐ equal instant — BOTH emit, the multi-source
//	42.0s  bybit rate delta   ┘ batch that makes pickMarket genuinely contend
//	43.0s  bybit funding-silent chatter           (ordinal burned, no news)
//	44.0s  bybit rate delta                       (emits; ordinal 7, not 6)
//
// Expected raw-arrival ordinals are therefore 1..7 in the order listed, with
// emissions at 2, 3, 4, 5 and 7.
const (
	tPoll  = 40 * time.Second
	tSnap  = 40 * time.Second
	tMark1 = 41 * time.Second
	tTie   = 42 * time.Second
	tChat  = 43 * time.Second
	tLast  = 44 * time.Second
)

func writeCanonicalArchive(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeStreamFile(t, root, binanceMarkKey(), []capture.Record{
		dataRec(1, at(tMark1), binanceMarkJSON(at(tMark1-10*time.Millisecond), "0.00010")),
		dataRec(2, at(tTie), binanceMarkJSON(at(tTie-10*time.Millisecond), "0.00011")),
	})
	writeStreamFile(t, root, binanceInfoKey(), []capture.Record{
		restRec(1, at(tPoll), restOKPayload(t, at(tPoll-50*time.Millisecond), fundingInfoBodyJSON)),
	})
	writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
		dataRec(1, at(tSnap), bybitSnapJSON(at(tSnap-20*time.Millisecond), "0.00012")),
		dataRec(2, at(tTie), bybitRateDeltaJSON(at(tTie-15*time.Millisecond), "0.00013")),
		dataRec(3, at(tChat), bybitChatterJSON()),
		dataRec(4, at(tLast), bybitRateDeltaJSON(at(tLast-15*time.Millisecond), "0.00014")),
	})
	return root
}

// newWorld assembles one run's fresh state over an archive — the same pieces
// sim.assemble wires, minus marketdata, because this battery's subject is the
// replay engine and its sink is the test's own.
func newWorld(t *testing.T, root string, specs []StreamSpec) (*Tape, *Ingester, *clock.TestClock, *events.MarketDataLane) {
	t.Helper()
	tape, err := Load(root, specs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ing, err := NewIngester(specs)
	if err != nil {
		t.Fatalf("NewIngester: %v", err)
	}
	cals, err := Calibrate(tape, specs)
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	clk, err := clock.NewTestClock(tape.Start(), cals)
	if err != nil {
		t.Fatalf("NewTestClock: %v", err)
	}
	lane, err := events.NewMarketDataLane(64, events.NoCoalescing)
	if err != nil {
		t.Fatalf("NewMarketDataLane: %v", err)
	}
	return tape, ing, clk, lane
}

func newDispatcher(t *testing.T, lane *events.MarketDataLane, clk *clock.TestClock, sink dispatch.Sink) *dispatch.Dispatcher {
	t.Helper()
	ctl, err := events.NewControlLane(1)
	if err != nil {
		t.Fatalf("NewControlLane: %v", err)
	}
	d, err := dispatch.New(dispatch.Config{Control: ctl, Market: lane, Timers: clk, Sink: sink})
	if err != nil {
		t.Fatalf("dispatch.New: %v", err)
	}
	return d
}

// delivered is one sink observation, flattened for asserting order, ordinal
// inheritance, and the clock's position at delivery.
type delivered struct {
	kind    string // "env" or "timer"
	source  events.SourceID
	seq     uint64
	ordinal uint64
	recv    time.Time
	timerID clock.TimerID
	firedAt time.Time
	now     time.Time
	// peers counts envelopes queued in OTHER sources' mailboxes at the moment
	// this one was delivered — the §39p-10 witness that a batch reached
	// pickMarket with ≥2 genuine candidates, not merely that two publishes
	// happened at some point.
	peers int
}

// recSink records every delivery. When errOn is set its result is returned for
// matching envelopes, so a test can stage an ordinary-vs-fatal sink error.
type recSink struct {
	clk   *clock.TestClock
	lane  *events.MarketDataLane
	got   []delivered
	errOn func(events.Envelope) error
}

func (s *recSink) OnEnvelope(e events.Envelope) error {
	d := delivered{kind: "env", source: e.Source, seq: e.IngressSeq, ordinal: e.AcceptOrdinal, recv: e.RecvTime}
	if s.clk != nil {
		d.now = s.clk.Now()
	}
	if s.lane != nil {
		for _, src := range s.lane.Sources() {
			if src != e.Source {
				d.peers += s.lane.Len(src)
			}
		}
	}
	s.got = append(s.got, d)
	if s.errOn != nil {
		return s.errOn(e)
	}
	return nil
}

func (s *recSink) OnTimer(seq uint64, f clock.Fired) error {
	d := delivered{kind: "timer", seq: seq, timerID: f.ID, firedAt: f.At}
	if s.clk != nil {
		d.now = s.clk.Now()
	}
	s.got = append(s.got, d)
	return nil
}

// logCollect replays a dispatch log into a flat entry list for order asserts.
type logCollect struct {
	entries []delivered
}

func (c *logCollect) OnEnvelopeRecord(r dispatch.EnvelopeRecord) error {
	c.entries = append(c.entries, delivered{kind: "env", source: r.Source, seq: r.IngressSeq, ordinal: r.AcceptOrdinal, recv: r.RecvTime})
	return nil
}

func (c *logCollect) OnTimerRecord(r dispatch.TimerRecord) error {
	c.entries = append(c.entries, delivered{kind: "timer", seq: r.IngressSeq, timerID: r.Fired.ID, firedAt: r.Fired.At})
	return nil
}

func readLogEntries(t *testing.T, path string) []delivered {
	t.Helper()
	var c logCollect
	if _, err := dispatch.ReplayLog(path, &c); err != nil {
		t.Fatalf("ReplayLog %s: %v", path, err)
	}
	return c.entries
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
