package replay

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// TestCalibrateMeasuresPerVenueBound pins the three claims the doc comment
// makes, each against the fixture parameter that would collapse it:
//
//   - the offset is the MAX of (SourceTime − RecvTime) over emitted readings —
//     so the fixture's bybit stream emits twice, +20ms then +5ms, and only
//     "max" (not "last", not "first") answers 20ms;
//   - non-emitting records contribute nothing — so the funding-silent delta
//     carries the largest skew in the file (+50ms), where a mutant that
//     measured consumed records instead of emitted ones reads 50 and fails;
//   - a venue whose readings never run ahead — or never emit at all — still
//     gets a row, floored at zero, because a missing row makes ServerNow
//     unanswerable (binance here has no overlay stream, so its markPrice
//     frames are consumed without emission for the whole tape).
func TestCalibrateMeasuresPerVenueBound(t *testing.T) {
	root := t.TempDir()
	r1, r2, r3 := at(10*time.Second), at(20*time.Second), at(30*time.Second)
	writeStreamFile(t, root, bybitTickerKey(), []capture.Record{
		dataRec(1, r1, bybitSnapJSON(r1.Add(20*time.Millisecond), "0.0001")),
		dataRec(2, r2, bybitQuoteDeltaJSON(r2.Add(50*time.Millisecond))),
		dataRec(3, r3, bybitRateDeltaJSON(r3.Add(5*time.Millisecond), "0.0002")),
	})
	writeStreamFile(t, root, binanceMarkKey(), []capture.Record{
		dataRec(1, at(15*time.Second), binanceMarkJSON(at(15*time.Second-3*time.Millisecond), "0.0001")),
	})
	specs := []StreamSpec{
		{Key: binanceMarkKey(), Decoder: DecodeBinanceMarkPrice},
		{Key: bybitTickerKey(), Decoder: DecodeBybitTicker},
	}
	tape, err := Load(root, specs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cals, err := Calibrate(tape, specs)
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}

	got := map[ids.Venue]clock.Calibration{}
	for _, c := range cals {
		got[ids.Venue(c.Venue)] = c
	}
	if len(got) != 2 {
		t.Fatalf("calibrations for %d venues, want 2: %+v", len(got), cals)
	}
	if c, ok := got["bybit"]; !ok || c.Offset != 20*time.Millisecond {
		t.Errorf("bybit offset = %v (present %t), want 20ms — the max over EMITTED readings", c.Offset, ok)
	}
	if c, ok := got["binance"]; !ok || c.Offset != 0 {
		t.Errorf("binance offset = %v (present %t), want 0 — a lagging, never-emitting venue still gets a row", c.Offset, ok)
	}
}

// TestCalibrateIsAPureFunctionOfTheTape: two calls over one tape agree — the
// property that lets Generate and Rerun share one skew model without sharing
// state.
func TestCalibrateIsAPureFunctionOfTheTape(t *testing.T) {
	root := writeCanonicalArchive(t)
	tape, err := Load(root, canonicalSpecs())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	a, err := Calibrate(tape, canonicalSpecs())
	if err != nil {
		t.Fatalf("first Calibrate: %v", err)
	}
	b, err := Calibrate(tape, canonicalSpecs())
	if err != nil {
		t.Fatalf("second Calibrate: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("lengths differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("calibration %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}
