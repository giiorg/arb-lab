package crossvenue_test

// Pairing fixtures, checklist-first: no fixture in here puts both venues
// on one shared grid origin with zero jitter, because that is exactly the
// collapse §45g names — "published-instant pairing" degenerating into
// "grid arithmetic". The jitter fixture is chosen so nearest-rounding and
// truncation DISAGREE: an implementation that floors instants into
// buckets loses the match.

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
)

func TestPair_MatchesOnlyViaRounding(t *testing.T) {
	// Binance publishes 5 ms EARLY (negative jitter), Bybit exact: the two
	// instants straddle a minute boundary, so Round matches them and
	// Truncate would not.
	bnAt := cvBase.Add(4*time.Hour - 5*time.Millisecond)
	byAt := cvBase.Add(4 * time.Hour)
	matches, stats, err := crossvenue.Pair(
		[]crossvenue.Settlement{settle(t, bnAt, "0.0003")},
		[]crossvenue.Settlement{settle(t, byAt, "0.0001")},
		time.Minute,
	)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if len(matches) != 1 || stats.Matched != 1 {
		t.Fatalf("matched %d (stats %d), want 1 — straddling jitter must pair under nearest-rounding", len(matches), stats.Matched)
	}
	m := matches[0]
	if !m.At.Equal(cvBase.Add(4 * time.Hour)) {
		t.Fatalf("bucket %s, want %s", m.At, cvBase.Add(4*time.Hour))
	}
	// The exact venue instants survive — the phase ledger needs them.
	if !m.BinanceAt.Equal(bnAt) || !m.BybitAt.Equal(byAt) {
		t.Fatalf("exact instants lost: bn %s by %s", m.BinanceAt, m.BybitAt)
	}
	wantRate(t, "spread", m.Spread, "0.0002")
	wantRate(t, "binance leg", m.Binance, "0.0003")
	wantRate(t, "bybit leg", m.Bybit, "0.0001")
}

func TestPair_DistinctInstantsDoNotMatch(t *testing.T) {
	// Two minutes apart at a one-minute round: distinct buckets, no match —
	// the fixture §45g demands alongside the jitter one.
	matches, stats, err := crossvenue.Pair(
		[]crossvenue.Settlement{settle(t, cvBase, "0.0003")},
		[]crossvenue.Settlement{settle(t, cvBase.Add(2*time.Minute), "0.0001")},
		time.Minute,
	)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if len(matches) != 0 || stats.Matched != 0 {
		t.Fatalf("matched %d, want 0", len(matches))
	}
	if stats.BinanceRows != 1 || stats.BybitRows != 1 {
		t.Fatalf("row stats %+v, want 1/1", stats)
	}
}

func TestPair_CollisionBucketDroppedEntirely(t *testing.T) {
	// Binance prints twice inside one rounded minute (venue dirt) beside a
	// clean pair one grid step later: the dirty bucket drops BOTH rows and
	// counts them, the clean bucket still matches — dropping is per
	// bucket, not per series.
	bn := []crossvenue.Settlement{
		settle(t, cvBase.Add(10*time.Second), "0.0001"),
		settle(t, cvBase.Add(20*time.Second), "0.0002"),
		settle(t, cvBase.Add(4*time.Hour), "0.0005"),
	}
	by := []crossvenue.Settlement{
		settle(t, cvBase, "0.0004"),
		settle(t, cvBase.Add(4*time.Hour).Add(3*time.Second), "0.0001"),
	}
	matches, stats, err := crossvenue.Pair(bn, by, time.Minute)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matched %d, want 1 (the clean bucket)", len(matches))
	}
	wantRate(t, "clean spread", matches[0].Spread, "0.0004")
	if stats.BinanceCollisions != 2 {
		t.Fatalf("binance collisions %d, want 2 — both rows of the dirty bucket count", stats.BinanceCollisions)
	}
	if stats.BybitCollisions != 0 {
		t.Fatalf("bybit collisions %d, want 0", stats.BybitCollisions)
	}
}

func TestPair_TripleCollisionCountsEveryRow(t *testing.T) {
	bn := []crossvenue.Settlement{
		settle(t, cvBase.Add(1*time.Second), "0.0001"),
		settle(t, cvBase.Add(2*time.Second), "0.0002"),
		settle(t, cvBase.Add(3*time.Second), "0.0003"),
	}
	_, stats, err := crossvenue.Pair(bn, nil, time.Minute)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if stats.BinanceCollisions != 3 {
		t.Fatalf("collisions %d, want 3", stats.BinanceCollisions)
	}
}

func TestPair_Refusals(t *testing.T) {
	good := []crossvenue.Settlement{settle(t, cvBase, "0.0001")}

	if _, _, err := crossvenue.Pair(good, good, 0); !errors.Is(err, crossvenue.ErrRoundInvalid) {
		t.Fatalf("zero round: %v, want ErrRoundInvalid", err)
	}
	if _, _, err := crossvenue.Pair(good, good, -time.Minute); !errors.Is(err, crossvenue.ErrRoundInvalid) {
		t.Fatalf("negative round: %v, want ErrRoundInvalid", err)
	}

	disordered := []crossvenue.Settlement{
		settle(t, cvBase.Add(4*time.Hour), "0.0001"),
		settle(t, cvBase, "0.0002"),
	}
	if _, _, err := crossvenue.Pair(disordered, nil, time.Minute); !errors.Is(err, crossvenue.ErrSeriesDisordered) {
		t.Fatalf("disordered binance: %v, want ErrSeriesDisordered", err)
	}
	if _, _, err := crossvenue.Pair(nil, disordered, time.Minute); !errors.Is(err, crossvenue.ErrSeriesDisordered) {
		t.Fatalf("disordered bybit: %v, want ErrSeriesDisordered", err)
	}

	unset := []crossvenue.Settlement{{At: cvBase}}
	if _, _, err := crossvenue.Pair(unset, nil, time.Minute); err == nil {
		t.Fatal("unset rate accepted")
	}
}

func TestPair_OutputAscending(t *testing.T) {
	var bn, by []crossvenue.Settlement
	for i := 5; i >= 0; i-- {
		// Build inputs ascending; map iteration scrambles, sort restores.
		at := cvBase.Add(time.Duration(5-i) * 4 * time.Hour)
		bn = append(bn, settle(t, at.Add(8*time.Millisecond), "0.0003"))
		by = append(by, settle(t, at, "0.0001"))
	}
	matches, _, err := crossvenue.Pair(bn, by, time.Minute)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	if len(matches) != 6 {
		t.Fatalf("matched %d, want 6", len(matches))
	}
	for i := 1; i < len(matches); i++ {
		if !matches[i-1].At.Before(matches[i].At) {
			t.Fatalf("output not ascending at %d: %s then %s", i, matches[i-1].At, matches[i].At)
		}
	}
}
