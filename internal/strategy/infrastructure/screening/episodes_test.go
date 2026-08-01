package screening

// The episode model's tests. Every fixture is built to CONTAIN the
// distinction it checks (the nine-times checklist): the trigger print
// differs from the accrued prints, hysteresis fixtures sit strictly
// between exit and entry, the gap fixture re-triggers on the print
// after the hole, and the annualization fixture's two candidate modal
// intervals tie so the tie-break is exercised rather than assumed.

import (
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

var t0 = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// series builds a matched series at fixed 4h spacing (the §42b majority
// grid) from decimal spread strings.
func series(tb testing.TB, spreads ...string) []Match {
	tb.Helper()
	out := make([]Match, len(spreads))
	for i, s := range spreads {
		r, err := fixedpoint.ParseRate(s)
		if err != nil {
			tb.Fatalf("parse %q: %v", s, err)
		}
		out[i] = Match{At: t0.Add(time.Duration(i) * 4 * time.Hour), Spread: r}
	}
	return out
}

// levelAt finds the LevelResult for an entry level by its decimal.
func levelAt(tb testing.TB, res PairResult, entry string) LevelResult {
	tb.Helper()
	for _, l := range res.Levels {
		if l.Entry.Decimal() == entry {
			return l
		}
	}
	tb.Fatalf("no level with entry %s", entry)
	return LevelResult{}
}

func screenOK(tb testing.TB, matched []Match) PairResult {
	tb.Helper()
	res, err := ScreenPair("TESTUSDT", matched, PairStats{})
	if err != nil {
		tb.Fatalf("ScreenPair: %v", err)
	}
	return res
}

func wantRate(tb testing.TB, got fixedpoint.Rate, want string, what string) {
	tb.Helper()
	if got.Decimal() != want {
		tb.Fatalf("%s: got %s, want %s", what, got.Decimal(), want)
	}
}

// The trigger print is observed, never collected: a two-print series
// where accruing the trigger would double the gross.
func TestEpisodeTriggerPrintIsNotAccrued(t *testing.T) {
	res := screenOK(t, series(t, "0.0025", "0.0004"))
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 1 || lvl.Accrued != 1 {
		t.Fatalf("episodes %d accrued %d, want 1 and 1", lvl.Episodes, lvl.Accrued)
	}
	wantRate(t, lvl.Gross, "0.0004", "gross") // 0.0029 would mean the trigger was accrued
	wantRate(t, lvl.Net, "-0.0017", "net")    // a losing episode stays in the sum
	if lvl.Wins != 0 {
		t.Fatalf("wins %d, want 0", lvl.Wins)
	}
}

// Hysteresis: prints strictly between exit (entry/2) and entry keep the
// episode open — an exit-at-entry implementation closes at the first
// 0.0012 and flips the verdict from win to loss.
func TestEpisodeHysteresisHoldsBetweenExitAndEntry(t *testing.T) {
	res := screenOK(t, series(t, "0.0025", "0.0012", "0.0012", "0.0004"))
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 1 || lvl.MaxRun != 3 {
		t.Fatalf("episodes %d maxrun %d, want 1 and 3", lvl.Episodes, lvl.MaxRun)
	}
	wantRate(t, lvl.Gross, "0.0028", "gross")
	wantRate(t, lvl.Net, "0.0007", "net")
	if lvl.Wins != 1 {
		t.Fatalf("wins %d, want 1", lvl.Wins)
	}
}

// A sign flip charges the flipped print to the open episode, closes it,
// and the same print may trigger a fresh episode in its own direction.
func TestEpisodeSignFlipAccruesAndReenters(t *testing.T) {
	res := screenOK(t, series(t, "0.0025", "-0.003", "-0.0005"))
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 2 {
		t.Fatalf("episodes %d, want 2", lvl.Episodes)
	}
	// Episode 1 accrues -0.003 (the flip, charged), episode 2 accrues
	// +0.0005 (|-0.0005| in the short-bybit direction).
	wantRate(t, lvl.Gross, "-0.0025", "gross")
	wantRate(t, lvl.Net, "-0.0067", "net") // two round trips charged
	// Continuation demands the SAME sign: both prints are at or above
	// entry and adjacent, but the flip means neither pair persists.
	if lvl.AboveCount != 2 || lvl.StayCount != 0 {
		t.Fatalf("above %d stay %d, want 2 and 0 (sign guard)", lvl.AboveCount, lvl.StayCount)
	}
}

// The three boundary prints where a comparison direction flips the
// answer (the `<=` vs `<` family): exactly at the entry level triggers,
// exactly at the exit level holds, and a hole exactly at the gap bound
// does NOT split the episode.
func TestEpisodeBoundaries(t *testing.T) {
	// Exactly at entry: 0.002 at E=0.002 opens an episode.
	res := screenOK(t, series(t, "0.002", "0.0004"))
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 1 {
		t.Fatalf("entry boundary: episodes %d, want 1 (>= entry triggers)", lvl.Episodes)
	}

	// Exactly at exit: 0.001 at exit level E/2 holds the episode open.
	res = screenOK(t, series(t, "0.0025", "0.001", "0.0004"))
	lvl = levelAt(t, res, "0.002")
	if lvl.Episodes != 1 || lvl.MaxRun != 2 {
		t.Fatalf("exit boundary: episodes %d maxrun %d, want 1 and 2 (at-exit holds)", lvl.Episodes, lvl.MaxRun)
	}
	wantRate(t, lvl.Gross, "0.0014", "exit boundary gross")

	// A hole of exactly MaxGapFactor × modal (8h on a 4h grid) is still
	// adjacent: one episode, no forced exit.
	matched := []Match{
		{At: t0, Spread: scrRate("0.0025")},
		{At: t0.Add(4 * time.Hour), Spread: scrRate("0.0015")},
		{At: t0.Add(12 * time.Hour), Spread: scrRate("0.0015")}, // 8h = the bound exactly
		{At: t0.Add(16 * time.Hour), Spread: scrRate("0.0004")},
	}
	res = screenOK(t, matched)
	lvl = levelAt(t, res, "0.002")
	if lvl.Episodes != 1 || lvl.MaxRun != 3 {
		t.Fatalf("gap boundary: episodes %d maxrun %d, want 1 and 3 (at-bound holds)", lvl.Episodes, lvl.MaxRun)
	}
}

// A hole wider than MaxGapFactor × modal interval closes the open
// episode BEFORE the print after the hole is considered — that print
// re-triggers, it does not accrue into the pre-gap episode.
func TestEpisodeGapForcesExitThenRetrigger(t *testing.T) {
	matched := []Match{
		{At: t0, Spread: scrRate("0.0025")},
		{At: t0.Add(4 * time.Hour), Spread: scrRate("0.0015")},
		{At: t0.Add(16 * time.Hour), Spread: scrRate("0.0025")}, // 12h hole > 2×4h
		{At: t0.Add(20 * time.Hour), Spread: scrRate("0.0004")},
	}
	res := screenOK(t, matched)
	if res.ModalInterval != 4*time.Hour || res.IntervalChanges != 1 {
		t.Fatalf("modal %s changes %d, want 4h and 1", res.ModalInterval, res.IntervalChanges)
	}
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 2 || lvl.MaxRun != 1 {
		t.Fatalf("episodes %d maxrun %d, want 2 and 1", lvl.Episodes, lvl.MaxRun)
	}
	// 0.0015 (pre-gap) + 0.0004 (post-gap episode's exit print); an
	// episode that accrued across the hole would add the second 0.0025.
	wantRate(t, lvl.Gross, "0.0019", "gross")
}

// Data ending mid-episode closes it and still charges the round trip.
func TestEpisodeForceClosedAtSeriesEnd(t *testing.T) {
	res := screenOK(t, series(t, "0.0025", "0.0015", "0.0015"))
	lvl := levelAt(t, res, "0.002")
	if lvl.Episodes != 1 || lvl.MaxRun != 2 {
		t.Fatalf("episodes %d maxrun %d, want 1 and 2", lvl.Episodes, lvl.MaxRun)
	}
	wantRate(t, lvl.Net, "0.0009", "net") // 0.003 gross − 0.0021 round trip
}

// Annualization divides by the measured span in 365.25-day years, and
// the modal-interval tie (one 4h delta, one 4379h delta) resolves to
// the smaller interval.
func TestEpisodeAnnualizationAndModalTieBreak(t *testing.T) {
	matched := []Match{
		{At: t0, Spread: scrRate("0.0025")},
		{At: t0.Add(4 * time.Hour), Spread: scrRate("0.0004")},
		{At: t0.Add(4383 * time.Hour), Spread: scrRate("0")}, // span = exactly 0.5 years
	}
	res := screenOK(t, matched)
	if res.ModalInterval != 4*time.Hour {
		t.Fatalf("modal %s, want 4h (tie resolves small)", res.ModalInterval)
	}
	lvl := levelAt(t, res, "0.002")
	wantRate(t, lvl.Net, "-0.0017", "net")
	wantRate(t, lvl.NetAnnual, "-0.0034", "net annual") // −0.0017 over half a year
}

// Continuation evidence counts adjacent same-sign persistence at the
// entry level: of the two prints at or above entry, one is followed by
// another.
func TestEpisodeContinuationCounts(t *testing.T) {
	res := screenOK(t, series(t, "0.0025", "0.0025", "0.0004"))
	lvl := levelAt(t, res, "0.002")
	if lvl.AboveCount != 2 || lvl.StayCount != 1 || lvl.Per10k != 5000 {
		t.Fatalf("above %d stay %d per10k %d, want 2, 1, 5000", lvl.AboveCount, lvl.StayCount, lvl.Per10k)
	}
}

// Best picks the level with the highest annualized net: a long run of
// small prints ranks at the smallest entry level, where the larger
// levels never trigger at all.
func TestBestLevelSelection(t *testing.T) {
	spreads := make([]string, 12)
	for i := range spreads {
		spreads[i] = "0.0003"
	}
	res := screenOK(t, series(t, spreads...))
	if res.Levels[res.Best].Entry.Decimal() != "0.0002" {
		t.Fatalf("best entry %s, want 0.0002", res.Levels[res.Best].Entry.Decimal())
	}
	best := res.Levels[res.Best]
	wantRate(t, best.Net, "0.0012", "net") // 11 accrued × 0.0003 − 0.0021
	if s := mustSign(best.NetAnnual); s <= 0 {
		t.Fatalf("net annual sign %d, want positive", s)
	}
	for _, l := range res.Levels {
		if l.Entry.Decimal() != "0.0002" && l.Episodes != 0 {
			t.Fatalf("level %s has %d episodes, want 0", l.Entry.Decimal(), l.Episodes)
		}
	}
}

// The quantile fields use nearest-rank on |spread|: upper median for
// even counts, floor(0.99 n) for the p99 index, the last for max.
func TestSpreadQuantiles(t *testing.T) {
	res := screenOK(t, series(t, "0.0001", "-0.0004", "0.0002", "-0.0003"))
	wantRate(t, res.MedianAbs, "0.0003", "median abs") // sorted [1,2,3,4]e-4, index 2
	wantRate(t, res.MaxAbs, "0.0004", "max abs")
}

func TestYearsRate(t *testing.T) {
	cases := []struct {
		span time.Duration
		want string
	}{
		{span: 31_557_600 * time.Second, want: "1"},       // exactly one 365.25d year
		{span: 2 * 31_557_600 * time.Second, want: "2"},   // two years
		{span: 31_557_600 / 2 * time.Second, want: "0.5"}, // half a year
	}
	for _, c := range cases {
		got, err := yearsRate(c.span)
		if err != nil {
			t.Fatalf("yearsRate(%s): %v", c.span, err)
		}
		wantRate(t, got, c.want, c.span.String())
	}
	if _, err := yearsRate(0); err == nil {
		t.Fatalf("yearsRate(0): want error")
	}
}
