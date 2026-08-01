package crossvenue

// Sampler battery. Fixture discipline (§45g + the nine-times checklist):
// one frame lands EXACTLY ON a grid instant — the at-or-before convention
// is only visible there, and the assertion pins WHICH rate the boundary
// instant reports; the Bybit fixtures separate "funding news" from
// "bid/ask churn", because the churn-refreshes-the-anchor rule is the
// distinction alt tickers live on (§45j's measured observation); and the
// merge fixture's two rates differ in magnitude so the Binance-minus-Bybit
// orientation cannot pass mirrored.

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

var sampleDay = "2026-07-30"
var sampleBase = time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)

var bnInstFix = ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "ABCUSDT"}
var bnFundingKeyFix = capture.StreamKey{Source: events.SourceID{Instrument: bnInstFix, Stream: events.StreamFunding}, Topic: "abcusdt@markPrice@1s"}

var byInstFix = ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "ABCUSDT"}
var byFundingKeyFix = capture.StreamKey{Source: events.SourceID{Instrument: byInstFix, Stream: events.StreamFunding}, Topic: "tickers.ABCUSDT"}

// The topic path segment is percent-encoded on disk (capture's
// encodeSegment): "@" is %40.
func bnFundingRel(name string) string {
	return "v1/binance/linear_perp/linear/ABCUSDT/funding/abcusdt%40markPrice%401s/2026/07/30/" + name
}

func byFundingRel(name string) string {
	return "v1/bybit/linear_perp/linear/ABCUSDT/funding/tickers.ABCUSDT/2026/07/30/" + name
}

func markFrame(recv time.Time, rate string) string {
	return fmt.Sprintf(`{"e":"markPriceUpdate","E":%d,"s":"ABCUSDT","r":%q,"T":%d}`,
		recv.UnixMilli(), rate, recv.Add(4*time.Hour).UnixMilli())
}

func tickerSnapshot(recv time.Time, rate string) string {
	return fmt.Sprintf(`{"topic":"tickers.ABCUSDT","type":"snapshot","data":{"symbol":"ABCUSDT","markPrice":"1.00","fundingIntervalHour":"4","fundingCap":"0.005","nextFundingTime":"%d","fundingRate":%q,"bid1Price":"1.00"},"ts":%d}`,
		recv.Add(4*time.Hour).UnixMilli(), rate, recv.UnixMilli())
}

func tickerChurn(recv time.Time) string {
	return fmt.Sprintf(`{"topic":"tickers.ABCUSDT","type":"delta","data":{"symbol":"ABCUSDT","bid1Price":"1.01"},"ts":%d}`, recv.UnixMilli())
}

func tickerFundingDelta(recv time.Time, rate string) string {
	return fmt.Sprintf(`{"topic":"tickers.ABCUSDT","type":"delta","data":{"symbol":"ABCUSDT","fundingRate":%q},"ts":%d}`, rate, recv.UnixMilli())
}

var sampleP = SampleParams{Every: 5 * time.Second, Slack: 4 * time.Second}

func sampleRoot(t *testing.T, rel func(string) string, name string, recs ...fixRec) (string, []ManifestDay) {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, filepath_Join(root, rel(name)), false, recs...)
	writeManifest(t, root, sampleDay, rel(name))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	return root, days
}

// filepath_Join joins a root and a slash-relative path portably.
func filepath_Join(root, rel string) string {
	return root + "/" + rel
}

// The boundary instant: a frame EXACTLY ON the grid is "at t" for that
// instant, and the instant is emitted while processing the NEXT frame —
// so :15 must report the :15 frame's rate, not the :10 frame's.
func TestSampleBinanceGridBoundaryUsesFrameAtInstant(t *testing.T) {
	a, b, c := sampleBase.Add(10*time.Second), sampleBase.Add(15*time.Second), sampleBase.Add(18*time.Second)
	root, days := sampleRoot(t, bnFundingRel, "2026-07-30T10.arbcap",
		dataAt(a, markFrame(a, "0.0001")),
		dataAt(b, markFrame(b, "0.0002")),
		dataAt(c, markFrame(c, "0.0003")),
	)
	got, err := SampleBinanceEstimates(root, days, bnFundingKeyFix, sampleP)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Points) != 1 {
		t.Fatalf("points %d, want 1: %+v", len(got.Points), got.Points)
	}
	p := got.Points[0]
	if !p.At.Equal(b) {
		t.Fatalf("instant %s, want %s", p.At, b)
	}
	if p.Rate.Decimal() != "0.0002" {
		t.Fatalf("rate %s, want the boundary frame's 0.0002", p.Rate.Decimal())
	}
	if got.Frames != 3 {
		t.Fatalf("frames %d, want 3", got.Frames)
	}
}

// An estimate older than Slack at the instant is a skip, counted — never
// a stale value sampled as fresh. The :16 frame sits EXACTLY Slack before
// the :20 instant: at-the-bound is fresh, and only that instant separates
// "older than" from "at least as old as".
func TestSampleBinanceStaleSkipped(t *testing.T) {
	a := sampleBase.Add(10 * time.Second)
	b := sampleBase.Add(16 * time.Second)
	c := sampleBase.Add(31 * time.Second)
	root, days := sampleRoot(t, bnFundingRel, "2026-07-30T10.arbcap",
		dataAt(a, markFrame(a, "0.0001")),
		dataAt(b, markFrame(b, "0.0007")),
		dataAt(c, markFrame(c, "0.0009")),
	)
	got, err := SampleBinanceEstimates(root, days, bnFundingKeyFix, sampleP)
	if err != nil {
		t.Fatal(err)
	}
	// :15 → the :10 frame is 5s old: skip. :20 → the :16 frame is exactly
	// 4s old, AT the slack bound: sampled. :25 and :30 → 9s and 14s: skip.
	if len(got.Points) != 1 || got.Skipped != 3 {
		t.Fatalf("points %d skipped %d, want 1 and 3: %+v", len(got.Points), got.Skipped, got.Points)
	}
	if !got.Points[0].At.Equal(sampleBase.Add(20*time.Second)) || got.Points[0].Rate.Decimal() != "0.0007" {
		t.Fatalf("point %+v, want the at-bound sample of 0.0007 at :20", got.Points[0])
	}
}

// A recorded Gap invalidates the held estimate even when Slack would have
// tolerated its age.
func TestSampleBinanceGapInvalidates(t *testing.T) {
	wide := SampleParams{Every: 5 * time.Second, Slack: time.Minute}
	a := sampleBase.Add(10 * time.Second)
	b := sampleBase.Add(19 * time.Second)
	root, days := sampleRoot(t, bnFundingRel, "2026-07-30T10.arbcap",
		dataAt(a, markFrame(a, "0.0001")),
		gapAt(sampleBase.Add(12*time.Second)),
		dataAt(b, markFrame(b, "0.0002")),
	)
	got, err := SampleBinanceEstimates(root, days, bnFundingKeyFix, wide)
	if err != nil {
		t.Fatal(err)
	}
	// :15 falls after the gap with no fresh frame yet — skipped despite
	// the one-minute slack.
	if got.Skipped != 1 {
		t.Fatalf("skipped %d, want 1", got.Skipped)
	}
	if len(got.Points) != 0 {
		t.Fatalf("points %+v, want none (next instant :20 lands after frame b's emission window)", got.Points)
	}
}

// The churn rule: a delta with no funding fields refreshes the anchor —
// the venue's absent-means-unchanged semantics — so the held rate stays
// fresh; and before any snapshot the stream is cold, skipped.
func TestSampleBybitChurnRefreshesAnchor(t *testing.T) {
	cold := sampleBase.Add(2 * time.Second)   // churn before any snapshot
	snap := sampleBase.Add(4 * time.Second)   // snapshot, rate A
	churn := sampleBase.Add(14 * time.Second) // churn at :14
	news := sampleBase.Add(22 * time.Second)  // funding delta, rate B
	tail := sampleBase.Add(31 * time.Second)  // pushes the grid past :30
	root, days := sampleRoot(t, byFundingRel, "2026-07-30T10.arbcap",
		dataAt(cold, tickerChurn(cold)),
		dataAt(snap, tickerSnapshot(snap, "0.0001")),
		dataAt(churn, tickerChurn(churn)),
		dataAt(news, tickerFundingDelta(news, "0.0003")),
		dataAt(tail, tickerChurn(tail)),
	)
	got, err := SampleBybitEstimates(root, days, byFundingKeyFix, sampleP)
	if err != nil {
		t.Fatal(err)
	}
	// :05 → snapshot 1s old: rate A. :10 → snapshot 6s stale: skip.
	// :15 → churn refreshed the anchor at :14: rate A again. :20 → 6s
	// stale: skip. :25 → news at :22: rate B. :30 → 8s stale: skip.
	if len(got.Points) != 3 {
		t.Fatalf("points %d, want 3: %+v", len(got.Points), got.Points)
	}
	wantRates := []string{"0.0001", "0.0001", "0.0003"}
	for i, w := range wantRates {
		if got.Points[i].Rate.Decimal() != w {
			t.Fatalf("point %d rate %s, want %s", i, got.Points[i].Rate.Decimal(), w)
		}
	}
	if got.Skipped != 3 {
		t.Fatalf("skipped %d, want 3", got.Skipped)
	}
}

// A file boundary restarts the ticker state cold (§39p-2): the next
// file's FUNDING delta — which against retained state would merge into a
// complete, emitting reading — emits nothing until that file's own
// snapshot. A churn frame here could not tell the state reset from the
// anchor reset; only a funding delta exercises both.
func TestSampleBybitFileBoundaryResets(t *testing.T) {
	root := t.TempDir()
	snap := sampleBase.Add(4 * time.Second)
	news2 := sampleBase.Add(1*time.Hour + 3*time.Second)
	tail2 := sampleBase.Add(1*time.Hour + 9*time.Second)
	writeFixture(t, filepath_Join(root, byFundingRel("2026-07-30T10.arbcap")), false,
		dataAt(snap, tickerSnapshot(snap, "0.0001")))
	writeFixture(t, filepath_Join(root, byFundingRel("2026-07-30T11.arbcap")), false,
		dataAt(news2, tickerFundingDelta(news2, "0.0004")),
		dataAt(tail2, tickerChurn(tail2)),
	)
	writeManifest(t, root, sampleDay,
		byFundingRel("2026-07-30T10.arbcap"), byFundingRel("2026-07-30T11.arbcap"))
	days, err := LoadManifestDays(root, BooksCutoff)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SampleBybitEstimates(root, days, byFundingKeyFix, sampleP)
	if err != nil {
		t.Fatal(err)
	}
	// 11:00:05 sits 2s after the second file's churn — which, had the
	// snapshot's state survived the boundary, would have refreshed a
	// "fresh" rate. Cold start means: skipped.
	if len(got.Points) != 0 {
		t.Fatalf("points %+v, want none across the boundary", got.Points)
	}
	if got.Skipped == 0 {
		t.Fatal("nothing skipped — the boundary instant was never even considered")
	}
}

// The join keeps only instants BOTH venues sampled, oriented
// Binance-minus-Bybit; one-sided instants are counted, not silently gone.
func TestMergeDecisionPoints(t *testing.T) {
	t1, t2, t3 := sampleBase, sampleBase.Add(5*time.Second), sampleBase.Add(10*time.Second)
	mk := func(at time.Time, r string) EstimatePoint {
		rate, err := fixedpoint.ParseRate(r)
		if err != nil {
			t.Fatal(err)
		}
		return EstimatePoint{At: at, Rate: rate}
	}
	pts, stats, err := MergeDecisionPoints(
		[]EstimatePoint{mk(t1, "0.0003"), mk(t2, "0.0003")},
		[]EstimatePoint{mk(t2, "0.0001"), mk(t3, "0.0001")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 || !pts[0].At.Equal(t2) {
		t.Fatalf("points %+v, want one at t2", pts)
	}
	if pts[0].Signal.Decimal() != "0.0002" {
		t.Fatalf("signal %s, want 0.0002 (binance minus bybit)", pts[0].Signal.Decimal())
	}
	if stats.Joined != 1 || stats.BinanceOnly != 1 || stats.BybitOnly != 1 {
		t.Fatalf("stats %+v", stats)
	}
}

// An undeclared cadence refuses rather than sampling nothing.
func TestSampleParamsUnsetRefuses(t *testing.T) {
	_, err := SampleBinanceEstimates("", nil, bnFundingKeyFix, SampleParams{})
	if !errors.Is(err, ErrSampleParamsUnset) {
		t.Fatalf("err %v, want ErrSampleParamsUnset", err)
	}
}
