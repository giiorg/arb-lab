package driving

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Fixture discipline: the pairing distinctions the §41i checklist names
// are all exercised against values where collapsing them CHANGES the
// answer — the later book differs from the at-book (walking the wrong
// instant is visible), the two sides differ (a side swap is visible), and
// the gap fixture holds a pending pair the gap must kill.

var (
	f2SpotInst = ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT"}
	f2Day      = "2026-07-30"
	f2Window   = time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
)

func f2Key(t *testing.T) capture.StreamKey {
	t.Helper()
	src := events.SourceID{Instrument: f2SpotInst, Stream: events.StreamBook}
	if err := src.Validate(); err != nil {
		t.Fatalf("source: %v", err)
	}
	return capture.StreamKey{Source: src, Topic: "btcusdt@depth20@100ms"}
}

// writeArchiveFile writes one hour file of records and returns its
// root-relative path.
func writeArchiveFile(t *testing.T, root string, key capture.StreamKey, window time.Time, recs []capture.Record) string {
	t.Helper()
	full := key.FilePath(root, window, time.Hour)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	buf := capture.AppendFileHeader(nil)
	for _, r := range recs {
		var err error
		buf, err = capture.AppendRecord(buf, r)
		if err != nil {
			t.Fatalf("append record: %v", err)
		}
	}
	if err := os.WriteFile(full, buf, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	rel, err := filepath.Rel(root, full)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	return filepath.ToSlash(rel)
}

// writeManifest writes a day manifest vouching for the given paths.
func writeManifest(t *testing.T, root, day string, paths []string) {
	t.Helper()
	m := capture.Manifest{Format: "arbcap-manifest/1", Day: day}
	for _, p := range paths {
		m.Entries = append(m.Entries, capture.ManifestEntry{Path: p})
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	dir := filepath.Join(root, "v1", "_manifest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, day+".json"), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func dataRec(seq uint64, at time.Time, payload string) capture.Record {
	return capture.Record{
		Kind:       capture.RecordKindData,
		CaptureSeq: seq,
		RecvTime:   at,
		Mono:       time.Duration(seq) * time.Millisecond,
		Payload:    []byte(payload),
	}
}

func spotFrame(lastID uint64, bids, asks string) string {
	return fmt.Sprintf(`{"lastUpdateId":%d,"bids":%s,"asks":%s}`, lastID, bids, asks)
}

// catchAll is a single-rule classifier admitting every book.
func catchAll(t *testing.T) domain.StratumClassifier {
	t.Helper()
	c := domain.StratumClassifier{Rules: []domain.StratumRule{{
		Name:   "all",
		Spread: domain.RateInterval{Lo: domain.RateBound{Kind: domain.BoundUnbounded}, Hi: domain.RateBound{Kind: domain.BoundUnbounded}},
		Depth:  domain.QuantityInterval{Lo: domain.QuantityBound{Kind: domain.BoundUnbounded}, Hi: domain.QuantityBound{Kind: domain.BoundUnbounded}},
	}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("classifier: %v", err)
	}
	return c
}

func f2Params(t *testing.T) Family2Params {
	t.Helper()
	q1, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatal(err)
	}
	q2, err := fixedpoint.ParseQuantity("BTC", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	return Family2Params{
		SampleEvery: 10 * time.Second,
		Latency:     250 * time.Millisecond,
		PairSlack:   time.Second,
		Candidates:  []fixedpoint.Quantity{q1, q2},
		Classifier:  catchAll(t),
	}
}

func spotSpec(t *testing.T) BookStreamSpec {
	t.Helper()
	return BookStreamSpec{
		Key:        f2Key(t),
		Instrument: f2SpotInst,
		Decode: func(raw []byte) (venue.BinanceBookSnapshot, error) {
			return venue.DecodeBinanceSpotDepth20(raw, "BTC", "USDT")
		},
	}
}

// One pair, hand-derived: book A (at t) has asks 100.00×0.5, 100.10×0.5;
// the completing book C has asks 100.10×0.5, 100.20×2.0 — the best level
// vanished. For candidate 1.0: fill 0.5@100.10+0.5@100.20 = 100.15 vs ref
// 100.00 → slip 0.0015; walking A instead gives 0.0005, so the
// wrong-instant collapse is visible. Bids: A 99.90×1.0 → C 99.80×2.0; for
// 0.5: (49.95−49.90)/49.95 = 0.001001001.
func TestMeasureBooksCounterfactualPairing(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	tA := f2Window.Add(9*time.Minute + 59*time.Second + 900*time.Millisecond)
	tB := f2Window.Add(10*time.Minute + 150*time.Millisecond)
	tC := f2Window.Add(10*time.Minute + 300*time.Millisecond)
	recs := []capture.Record{
		dataRec(1, tA, spotFrame(11, `[["99.90","1.0"]]`, `[["100.00","0.5"],["100.10","0.5"]]`)),
		dataRec(2, tB, spotFrame(12, `[["99.90","1.0"]]`, `[["100.00","0.5"],["100.10","0.5"]]`)),
		dataRec(3, tC, spotFrame(13, `[["99.80","2.0"]]`, `[["100.10","0.5"],["100.20","2.0"]]`)),
	}
	rel := writeArchiveFile(t, root, key, f2Window, recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}

	got, err := MeasureBooks(root, days, spotSpec(t), f2Params(t))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if got.Pairs != 1 {
		t.Fatalf("pairs = %d, want 1", got.Pairs)
	}
	if len(got.Moves) != 2 {
		t.Fatalf("moves = %d, want 2 (both sides)", len(got.Moves))
	}
	if len(got.Slips) != 4 {
		t.Fatalf("cells = %d, want 4 (2 sides × 2 candidates)", len(got.Slips))
	}

	q1, _ := fixedpoint.ParseQuantity("BTC", "1.0")
	askKey := CellKey{Instrument: f2SpotInst, Side: mdomain.SideAsk, Stratum: "all", MaxQty: q1}
	askSlips := got.Slips[askKey]
	if len(askSlips) != 1 {
		t.Fatalf("ask 1.0 samples = %d, want 1", len(askSlips))
	}
	want, _ := fixedpoint.ParseRate("0.0015")
	if askSlips[0] != want {
		t.Fatalf("ask 1.0 slip = %s, want %s", askSlips[0], want)
	}
	qHalf, _ := fixedpoint.ParseQuantity("BTC", "0.5")
	bidKey := CellKey{Instrument: f2SpotInst, Side: mdomain.SideBid, Stratum: "all", MaxQty: qHalf}
	bidSlips := got.Slips[bidKey]
	if len(bidSlips) != 1 {
		t.Fatalf("bid 0.5 samples = %d, want 1", len(bidSlips))
	}
	wantBid, _ := fixedpoint.ParseRate("0.001001001")
	if bidSlips[0] != wantBid {
		t.Fatalf("bid 0.5 slip = %s, want %s", bidSlips[0], wantBid)
	}
}

// A Gap record between the sample and its completion kills the pending
// pair: the measurement never happens and the skip is counted.
func TestMeasureBooksGapDiscardsPending(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	tA := f2Window.Add(9*time.Minute + 59*time.Second + 900*time.Millisecond)
	tB := f2Window.Add(10*time.Minute + 100*time.Millisecond)
	tC := f2Window.Add(10*time.Minute + 400*time.Millisecond)
	gap := capture.Record{
		Kind: capture.RecordKindGap, CaptureSeq: 3,
		RecvTime: f2Window.Add(10*time.Minute + 200*time.Millisecond),
		Mono:     3 * time.Millisecond, Payload: []byte(`{"cause":"reconnect"}`),
	}
	recs := []capture.Record{
		dataRec(1, tA, spotFrame(21, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
		dataRec(2, tB, spotFrame(22, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
		gap,
		dataRec(4, tC, spotFrame(23, `[["99.90","1.0"]]`, `[["100.10","1.0"]]`)),
	}
	rel := writeArchiveFile(t, root, key, f2Window, recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}

	got, err := MeasureBooks(root, days, spotSpec(t), f2Params(t))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if got.Pairs != 0 {
		t.Fatalf("pairs = %d, want 0 — the pair straddles a recorded gap", got.Pairs)
	}
	if got.Skipped == 0 {
		t.Fatal("the killed pending pair was not counted")
	}
}

// H2 at file granularity: a file the day's manifest does not vouch for
// refuses by name, even though the day itself is manifested.
func TestMeasureBooksRefusesUnmanifestedFile(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	recs := []capture.Record{
		dataRec(1, f2Window.Add(time.Minute), spotFrame(31, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
	}
	writeArchiveFile(t, root, key, f2Window, recs)
	writeManifest(t, root, f2Day, []string{"v1/some/other/file.arbcap"})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}
	if _, err := MeasureBooks(root, days, spotSpec(t), f2Params(t)); !errors.Is(err, ErrDayUnmanifested) {
		t.Fatalf("unvouched file: %v, want ErrDayUnmanifested", err)
	}
}

// The H2 universe is the days STRICTLY BEFORE the freeze day — the freeze
// day's own manifest is still growing and reading it would make two
// invocations disagree.
func TestLoadManifestDaysFreezeBoundary(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "2026-07-30", []string{"a"})
	writeManifest(t, root, "2026-07-31", []string{"b"})
	writeManifest(t, root, "2026-08-01", []string{"c"})

	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(days) != 2 || days[0].Day != "2026-07-30" || days[1].Day != "2026-07-31" {
		t.Fatalf("days = %+v, want exactly 07-30 and 07-31", days)
	}
	for _, d := range days {
		if len(d.SHA256) != 64 {
			t.Fatalf("day %s digest %q not a sha256", d.Day, d.SHA256)
		}
	}
	// A freeze instant INSIDE 07-31 excludes 07-31 as well.
	days, err = LoadManifestDays(root, time.Date(2026, 7, 31, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(days) != 1 || days[0].Day != "2026-07-30" {
		t.Fatalf("days = %+v, want exactly 07-30", days)
	}
	// Nothing before the earliest day: an empty universe refuses.
	if _, err := LoadManifestDays(root, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)); !errors.Is(err, ErrNoManifestDays) {
		t.Fatalf("empty universe: %v, want ErrNoManifestDays", err)
	}
}

func markFrame(eMs int64, rate string, settleMs int64) string {
	return fmt.Sprintf(`{"e":"markPriceUpdate","E":%d,"s":"BTCUSDT","r":"%s","T":%d}`, eMs, rate, settleMs)
}

// The imminent observation: estimates sampled on the grid, realized taken
// from the period's LAST frame at a settlement flip, a trailing period
// with no flip dropped AND counted. Hand-derived: settle T1 = 12:00; the
// 11:59:00 sample carries estimate 0.0001, the 12:00:00 sample carries
// 0.00012 (the last pre-flip frame); realized(T1) = 0.00012 → residuals
// 0.00002 and 0. The 12:01:00 sample's settlement (20:00) never flips →
// Unrealized = 1.
func TestMeasureImminentResiduals(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	day := time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC)
	settle1 := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC).UnixMilli()
	settle2 := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC).UnixMilli()
	at := func(hh, mm, ss int) time.Time {
		return time.Date(2026, 7, 30, hh, mm, ss, 0, time.UTC)
	}
	mk := func(seq uint64, rt time.Time, rate string, settle int64) capture.Record {
		return dataRec(seq, rt, markFrame(rt.Add(-80*time.Millisecond).UnixMilli(), rate, settle))
	}
	recs := []capture.Record{
		mk(1, at(11, 58, 55), "0.00010000", settle1),
		mk(2, at(11, 59, 30), "0.00011000", settle1),
		mk(3, at(11, 59, 50), "0.00012000", settle1),
		mk(4, at(12, 0, 10), "0.00005000", settle2),
		mk(5, at(12, 1, 30), "0.00005000", settle2),
	}
	_ = day
	rel := writeArchiveFile(t, root, key, time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC), recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}

	got, err := MeasureImminent(root, days, key, ImminentParams{SampleEvery: time.Minute, RealizedSlack: time.Minute})
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if got.Estimates != 3 {
		t.Fatalf("estimates = %d, want 3 (11:59, 12:00, 12:01)", got.Estimates)
	}
	if got.Unrealized != 1 {
		t.Fatalf("unrealized = %d, want 1 (the 20:00 settlement never flipped)", got.Unrealized)
	}
	if len(got.Residuals) != 2 {
		t.Fatalf("residuals = %d, want 2", len(got.Residuals))
	}
	want0, _ := fixedpoint.ParseRate("0.00002")
	if got.Residuals[0] != want0 {
		t.Fatalf("residual 0 = %s, want %s (realized 0.00012 − estimate 0.0001)", got.Residuals[0], want0)
	}
	if sign, err := got.Residuals[1].Sign(); err != nil || sign != 0 {
		t.Fatalf("residual 1 = %s, want exactly zero (sampled the settling frame)", got.Residuals[1])
	}
	if len(got.Latencies) != len(recs) {
		t.Fatalf("latencies = %d, want %d (one per frame)", len(got.Latencies), len(recs))
	}
	// The fixture stamps E 80ms before recv, so every latency is exactly
	// 80ms — E anchored, not T.
	if got.Latencies[0] != 80*time.Millisecond {
		t.Fatalf("latency = %v, want 80ms", got.Latencies[0])
	}
}

// LatencyStats: the exact-rank/inexact-rank boundary and the negative
// count. Over 11 samples q90's rank 9.0 is exact (its own statistic);
// over 10 it is 8.1 and ceils to the largest.
func TestLatencyStats(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	ten := []time.Duration{ms(-30), ms(10), ms(20), ms(30), ms(40), ms(50), ms(60), ms(70), ms(80), ms(200)}
	got, err := LatencyStats(ten)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got.Count != 10 || got.Max != ms(200) || got.NegativeObservations != 1 {
		t.Fatalf("count/max/neg = %d/%v/%d", got.Count, got.Max, got.NegativeObservations)
	}
	if got.P90 != ms(200) {
		t.Fatalf("p90 of 10 = %v, want 200ms (ceil of rank 8.1)", got.P90)
	}
	eleven := append(append([]time.Duration{}, ten...), ms(300))
	got, err = LatencyStats(eleven)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got.P90 != ms(200) {
		t.Fatalf("p90 of 11 = %v, want 200ms (exact rank 9, its own statistic)", got.P90)
	}
	if got.P50 != ms(50) {
		t.Fatalf("p50 of 11 = %v, want 50ms", got.P50)
	}
	if _, err := LatencyStats(nil); err == nil {
		t.Fatal("empty samples summarized")
	}
}

// The pairing rule's EXPIRY arm, both sides of its boundary: a completion
// book exactly AT due+slack completes (inclusive), one past it expires
// the pending sample — <= and < separate here, and no other fixture
// reaches this arm at all.
func TestMeasureBooksExpiryBoundary(t *testing.T) {
	mk := func(completionOffset time.Duration) (BookMeasurements, error) {
		root := t.TempDir()
		key := f2Key(t)
		tA := f2Window.Add(9*time.Minute + 59*time.Second + 900*time.Millisecond)
		// due = 10:00:00.250; slack 1s → boundary 10:00:01.250.
		tC := f2Window.Add(10*time.Minute + 250*time.Millisecond + completionOffset)
		recs := []capture.Record{
			dataRec(1, tA, spotFrame(41, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
			dataRec(2, tC, spotFrame(42, `[["99.90","1.0"]]`, `[["100.10","1.0"]]`)),
		}
		rel := writeArchiveFile(t, root, key, f2Window, recs)
		writeManifest(t, root, f2Day, []string{rel})
		days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("manifest days: %v", err)
		}
		return MeasureBooks(root, days, spotSpec(t), f2Params(t))
	}

	atBoundary, err := mk(time.Second) // exactly due+slack
	if err != nil {
		t.Fatalf("at boundary: %v", err)
	}
	if atBoundary.Pairs != 1 {
		t.Fatalf("pairs at boundary = %d; want 1 (inclusive)", atBoundary.Pairs)
	}
	past, err := mk(time.Second + time.Millisecond) // one ms past
	if err != nil {
		t.Fatalf("past boundary: %v", err)
	}
	if past.Pairs != 0 || past.Skipped == 0 {
		t.Fatalf("pairs past boundary = %d skipped %d; want 0 pairs, the expiry counted", past.Pairs, past.Skipped)
	}
}

// The at-t freshness bound: a sampling instant whose newest book is older
// than PairSlack takes no sample — otherwise a pre-stall book would price
// an instant it did not see.
func TestMeasureBooksStaleBookSkipsSample(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	// Book 5s before the grid instant, next record after it: the 10:00:00
	// sample must be skipped (5s > 1s slack).
	tA := f2Window.Add(9*time.Minute + 55*time.Second)
	tB := f2Window.Add(10*time.Minute + 2*time.Second)
	recs := []capture.Record{
		dataRec(1, tA, spotFrame(51, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
		dataRec(2, tB, spotFrame(52, `[["99.90","1.0"]]`, `[["100.10","1.0"]]`)),
	}
	rel := writeArchiveFile(t, root, key, f2Window, recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}
	got, err := MeasureBooks(root, days, spotSpec(t), f2Params(t))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if got.Pairs != 0 {
		t.Fatalf("pairs = %d; want 0 — the at-t book was stale", got.Pairs)
	}
	if got.Skipped == 0 {
		t.Fatal("the stale skip was not counted")
	}
}

// The imminent slack arms, both of them: a settlement whose last frame
// sits farther than RealizedSlack before the instant testifies to
// nothing (samples dropped AND counted), and a sampling instant whose
// newest frame is stale takes no sample.
func TestMeasureImminentSlackArms(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	settle1 := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	settle2 := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC)
	at := func(hh, mm, ss int) time.Time { return time.Date(2026, 7, 30, hh, mm, ss, 0, time.UTC) }
	mk := func(seq uint64, rt time.Time, rate string, settle time.Time) capture.Record {
		return dataRec(seq, rt, markFrame(rt.Add(-80*time.Millisecond).UnixMilli(), rate, settle.UnixMilli()))
	}
	recs := []capture.Record{
		// An estimate sampled at 11:55:00, then a 5-minute silence: the
		// period's last frame sits 4m40s before the settlement — realized
		// testimony REFUSED, the sample dropped and counted. The frames
		// at 11:58/11:59 instants are ABSENT (stale-estimate arm: the
		// 11:57..11:59 grid instants take no sample).
		mk(1, at(11, 54, 40), "0.00020000", settle1),
		mk(2, at(11, 55, 20), "0.00020000", settle1),
		// The flip arrives late; realized(12:00) = frame at 11:55:20 —
		// 4m40s early, beyond the 60s slack.
		mk(3, at(12, 0, 30), "0.00005000", settle2),
	}
	rel := writeArchiveFile(t, root, key, time.Date(2026, 7, 30, 11, 0, 0, 0, time.UTC), recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}
	got, err := MeasureImminent(root, days, key, ImminentParams{SampleEvery: time.Minute, RealizedSlack: time.Minute})
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	// Two samples taken — 11:55:00 (20s fresh) and 11:56:00 (40s fresh);
	// the 11:57..12:00 instants exceed the 60s bound and take NO sample.
	// Both samples' settlement testimony sits 4m40s early → both dropped
	// and counted, zero residuals.
	if got.Estimates != 2 {
		t.Fatalf("estimates = %d; want 2 (11:57+ instants are stale and take no sample)", got.Estimates)
	}
	if got.Unrealized != 2 || len(got.Residuals) != 0 {
		t.Fatalf("unrealized = %d residuals = %d; want 2, 0 (stale realized testimony refused)",
			got.Unrealized, len(got.Residuals))
	}
}

// The at-t freshness bound is INCLUSIVE at exactly PairSlack: a book
// precisely one slack old still prices its instant — > and >= separate
// only here.
func TestMeasureBooksFreshnessBoundaryInclusive(t *testing.T) {
	root := t.TempDir()
	key := f2Key(t)
	tA := f2Window.Add(9*time.Minute + 59*time.Second) // exactly 1s before 10:00:00
	tC := f2Window.Add(10*time.Minute + 300*time.Millisecond)
	recs := []capture.Record{
		dataRec(1, tA, spotFrame(61, `[["99.90","1.0"]]`, `[["100.00","1.0"]]`)),
		dataRec(2, tC, spotFrame(62, `[["99.90","1.0"]]`, `[["100.10","1.0"]]`)),
	}
	rel := writeArchiveFile(t, root, key, f2Window, recs)
	writeManifest(t, root, f2Day, []string{rel})
	days, err := LoadManifestDays(root, time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("manifest days: %v", err)
	}
	got, err := MeasureBooks(root, days, spotSpec(t), f2Params(t))
	if err != nil {
		t.Fatalf("measure: %v", err)
	}
	if got.Pairs != 1 {
		t.Fatalf("pairs = %d; want 1 — a book exactly PairSlack old is fresh", got.Pairs)
	}
}
