package crossvenue

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
)

// The fixture archive is built through capture's own encoder, so the check is
// exercised against the format the writer actually produces. Fixture
// discipline: the day under test HOLDS a file from the wrong day, a stream
// with no decoder, and a refused frame — a fixture where everything decodes
// would collapse "smoke-checked" into "counted" (§45g's suspect list).

var checkDay = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

const labSnapshotFrame = `{"topic":"orderbook.50.LABUSDT","type":"snapshot","ts":1785582790927,` +
	`"data":{"s":"LABUSDT","b":[["0.14640","4789"]],"a":[["0.14650","3437"]],"u":6850738,"seq":163946559126}}`

const labDeltaFrame = `{"topic":"orderbook.50.LABUSDT","type":"delta","ts":1785582790967,` +
	`"data":{"s":"LABUSDT","b":[["0.14540","20735"]],"a":[],"u":6850739,"seq":163946559689}}`

// A ticker frame misfiled into a book stream: well-formed, wrong topic.
const foreignFrame = `{"topic":"tickers.LABUSDT","type":"snapshot","ts":1785582790980,` +
	`"data":{"symbol":"LABUSDT","fundingRate":"0.0001"}}`

// An op acknowledgement: consumed without news — decoded, NOT emitted. It
// exists so Decoded and Emitted differ in the fixture; equal counts would
// collapse the distinction and a mutant conflating them would survive.
const opAckFrame = `{"success":true,"conn_id":"c1","req_id":"orderbook.50.LABUSDT","op":"subscribe"}`

// The two Binance partial-depth dialects (shapes from the venue battery's
// fixtures): the spot frame is bare, the futures frame wraps in depthUpdate.
// Each decodes ONLY under its own product's decoder, so the routing swap is
// visible as a refusal.
const binanceSpotFrame = `{"lastUpdateId":98071966646,` +
	`"bids":[["64294.78000000","0.96889000"]],"asks":[["64294.79000000","0.51000000"]]}`

const binanceFuturesFrame = `{"e":"depthUpdate","E":1785471501803,"s":"BTCUSDT",` +
	`"U":11177881911418,"u":11177881916526,"pu":11177881911348,` +
	`"b":[["64266.60","10.287"]],"a":[["64266.70","5.100"]]}`

func writeArbcap(t *testing.T, path string, tornTail bool, payloads ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	buf := capture.AppendFileHeader(nil)
	recv := time.Date(2026, 8, 1, 11, 13, 12, 0, time.UTC)
	var err error
	buf, err = capture.AppendRecord(buf, capture.Record{
		Kind: capture.RecordKindOpen, CaptureSeq: 0, RecvTime: recv, Mono: 1, Payload: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range payloads {
		buf, err = capture.AppendRecord(buf, capture.Record{
			Kind:       capture.RecordKindData,
			CaptureSeq: uint64(i + 1),
			RecvTime:   recv.Add(time.Duration(i+1) * time.Second),
			Mono:       time.Duration(i + 2),
			Payload:    []byte(p),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if tornTail {
		whole := len(buf)
		buf, err = capture.AppendRecord(buf, capture.Record{
			Kind:       capture.RecordKindData,
			CaptureSeq: uint64(len(payloads) + 1),
			RecvTime:   recv.Add(time.Hour),
			Mono:       time.Hour,
			Payload:    []byte(labDeltaFrame),
		})
		if err != nil {
			t.Fatal(err)
		}
		buf = buf[:whole+7] // end inside the appended record
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	lab := filepath.Join(root, "v1", "bybit", "linear_perp", "linear", "LABUSDT",
		"book", "orderbook.50.LABUSDT", "2026", "08", "01")
	writeArbcap(t, filepath.Join(lab, "2026-08-01T11.arbcap"), true,
		labSnapshotFrame, labDeltaFrame, foreignFrame, opAckFrame)

	// A second hour file of the same stream: aggregation across files.
	writeArbcap(t, filepath.Join(lab, "2026-08-01T12.arbcap"), false, labDeltaFrame)

	// A stream nothing decodes: the skip must be NAMED, not silent.
	unknown := filepath.Join(root, "v1", "bybit", "linear_perp", "linear", "LABUSDT",
		"book", "orderbook2.LABUSDT", "2026", "08", "01")
	writeArbcap(t, filepath.Join(unknown, "2026-08-01T11.arbcap"), false, labSnapshotFrame)

	// The WRONG day, one file per differing component — a fixture differing
	// only in month leaves the day-of-month comparison unexercised (found by
	// a survived mutant, this session), and mutatis mutandis for the others.
	stream := []string{"v1", "bybit", "linear_perp", "linear", "LABUSDT", "book", "orderbook.50.LABUSDT"}
	for _, wrong := range [][4]string{
		{"2026", "07", "31", "2026-07-31T09.arbcap"}, // month differs
		{"2026", "08", "02", "2026-08-02T09.arbcap"}, // only day-of-month differs
		{"2025", "08", "01", "2025-08-01T09.arbcap"}, // only year differs
	} {
		dir := filepath.Join(append(append([]string{root}, stream...), wrong[0], wrong[1], wrong[2])...)
		writeArbcap(t, filepath.Join(dir, wrong[3]), false, labSnapshotFrame)
	}
	return root
}

func TestCheckSmokeDecodesADay(t *testing.T) {
	rep, err := Check(fixtureRoot(t), checkDay)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(rep.Streams) != 2 {
		t.Fatalf("streams = %d, want 2 (the wrong-day file must not add one)", len(rep.Streams))
	}

	var book, unknown *StreamCheck
	for i := range rep.Streams {
		switch rep.Streams[i].Topic {
		case "orderbook.50.LABUSDT":
			book = &rep.Streams[i]
		case "orderbook2.LABUSDT":
			unknown = &rep.Streams[i]
		}
	}
	if book == nil || unknown == nil {
		t.Fatalf("missing streams in report: %+v", rep.Streams)
	}

	// Two hour files aggregated; the wrong-day file contributes nothing.
	if book.Files != 2 {
		t.Fatalf("book files = %d, want 2", book.Files)
	}
	// 4 data frames in hour 11 + 1 in hour 12; the torn trailing record is
	// NOT a data record that decoded.
	if book.Data != 5 {
		t.Fatalf("book data = %d, want 5", book.Data)
	}
	// The op ack is decoded-not-emitted: the two counts MUST differ here.
	if book.Decoded != 4 || book.Emitted != 3 {
		t.Fatalf("decoded/emitted = %d/%d, want 4/3", book.Decoded, book.Emitted)
	}
	if book.Refused != 1 || !strings.Contains(book.FirstRefusal, "tickers.LABUSDT") {
		t.Fatalf("refused = %d, first = %q; want the misfiled ticker frame named", book.Refused, book.FirstRefusal)
	}
	if !book.TornTail {
		t.Fatal("torn tail not reported")
	}
	if book.Meta != 2 {
		t.Fatalf("meta = %d, want 2 open records", book.Meta)
	}
	if book.Skipped != "" {
		t.Fatalf("book stream skipped: %q", book.Skipped)
	}

	// The unknown stream is counted AND named, never silently dropped.
	if unknown.Skipped == "" || !strings.Contains(unknown.Skipped, "orderbook2.LABUSDT") {
		t.Fatalf("unknown stream skip = %q, want the topic named", unknown.Skipped)
	}
	if unknown.Data != 1 || unknown.Decoded != 0 {
		t.Fatalf("unknown stream data/decoded = %d/%d, want 1/0", unknown.Data, unknown.Decoded)
	}
	// The untorn stream must NOT report a torn tail — without this a mutant
	// reporting every stream torn passes the book assertion above.
	if unknown.TornTail {
		t.Fatal("untorn stream reported a torn tail")
	}
	if rep.Refusals() != 1 {
		t.Fatalf("total refusals = %d, want 1", rep.Refusals())
	}
}

// The two Binance dialects route by PRODUCT: each stream decodes clean under
// its own decoder, and each frame is refused by the other's — so a routing
// swap is visible as refusals, not as a silently tolerated decode.
func TestCheckRoutesBinanceDepthDialects(t *testing.T) {
	root := t.TempDir()
	spot := filepath.Join(root, "v1", "binance", "spot", "BTCUSDT",
		"book", "btcusdt@depth20@100ms", "2026", "08", "01")
	writeArbcap(t, filepath.Join(spot, "2026-08-01T11.arbcap"), false, binanceSpotFrame)
	fut := filepath.Join(root, "v1", "binance", "linear_perp", "linear", "BTCUSDT",
		"book", "btcusdt@depth20@100ms", "2026", "08", "01")
	writeArbcap(t, filepath.Join(fut, "2026-08-01T11.arbcap"), false, binanceFuturesFrame)

	rep, err := Check(root, checkDay)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(rep.Streams) != 2 {
		t.Fatalf("streams = %d, want 2", len(rep.Streams))
	}
	for _, s := range rep.Streams {
		if s.Refused != 0 || s.Decoded != 1 {
			t.Fatalf("%s %s: refused=%d decoded=%d, want 0/1 — dialect routed to the wrong decoder?",
				s.Product, s.Topic, s.Refused, s.Decoded)
		}
	}
}

// An empty walk is an ERROR, not a clean report — a wrong -root reading as
// "0 refusals" is the vacuous-run shape.
func TestCheckRefusesAnEmptyDay(t *testing.T) {
	root := fixtureRoot(t)
	_, err := Check(root, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrNoStreams) {
		t.Fatalf("empty day: %v, want ErrNoStreams", err)
	}
}

// The named-skip table's other branch: a book stream whose symbol is not a
// USDT pair cannot derive asset tags — skipped by NAME.
func TestCheckNamesNonUSDTSymbolSkip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "v1", "bybit", "linear_perp", "linear", "ETHBTC",
		"book", "orderbook.50.ETHBTC", "2026", "08", "01")
	writeArbcap(t, filepath.Join(dir, "2026-08-01T11.arbcap"), false, labSnapshotFrame)
	rep, err := Check(root, checkDay)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(rep.Streams) != 1 || !strings.Contains(rep.Streams[0].Skipped, "ETHBTC") {
		t.Fatalf("skip = %+v, want the symbol named", rep.Streams)
	}
}
