package screening

// The episode model (M1) — the ranking's core (§42b step 3). The §2.3
// bar is measured spread PERSISTENCE, so the unit of evidence is the
// episode, not the print: an entry triggers at a settlement whose
// |spread| reaches the entry level, accrual starts at the NEXT matched
// settlement (the trigger is observed, not collected — no look-ahead),
// the position holds under hysteresis, and every episode is charged the
// full four-leg round trip. Episodes that lose money COUNT — the sum
// keeps negative nets, because a screening that only sums winners is
// selecting on the answer.

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// LevelResult is the episode accounting at one entry level.
type LevelResult struct {
	Entry fixedpoint.Rate

	Episodes int
	Wins     int
	// Accrued counts settlements collected across all episodes
	// (triggers excluded); MaxRun is the longest single episode.
	Accrued int
	MaxRun  int

	Gross     fixedpoint.Rate // sum of accrued spread, before costs
	Net       fixedpoint.Rate // Gross minus Episodes × round trip
	NetAnnual fixedpoint.Rate // Net over the matched span, per year

	// Continuation evidence: of AboveCount adjacent-settlement prints
	// at or above the entry level, StayCount were followed by another
	// same-sign print at or above it. Per10k is their ratio ×10000,
	// -1 when AboveCount is zero.
	AboveCount int
	StayCount  int
	Per10k     int
}

// PairResult is one pair's full screening measurement.
type PairResult struct {
	Symbol string
	Stats  PairStats

	First time.Time
	Last  time.Time
	// ModalInterval is the measured (not declared) dominant spacing of
	// the matched series — §42a's grid honesty.
	ModalInterval time.Duration
	// IntervalChanges counts adjacent spacings that differ from the
	// modal interval — a nonzero count is a grid that moved (or holes).
	IntervalChanges int

	MedianAbs fixedpoint.Rate
	P99Abs    fixedpoint.Rate
	MaxAbs    fixedpoint.Rate

	Levels []LevelResult
	// Best indexes the level with the highest NetAnnual — the pair's
	// rank score. Reported, not hidden: picking the best of the grid
	// is mild selection and the record carries it.
	Best int
}

// mustCmp compares two set rates; both operands are always set here, so
// an error is a programmer error, same contract as the roster parsers.
func mustCmp(a, b fixedpoint.Rate) int {
	c, err := a.Cmp(b)
	if err != nil {
		panic(err)
	}
	return c
}

func mustAdd(a, b fixedpoint.Rate) fixedpoint.Rate {
	r, err := a.Add(b)
	if err != nil {
		panic(err)
	}
	return r
}

func mustSub(a, b fixedpoint.Rate) fixedpoint.Rate {
	r, err := a.Sub(b)
	if err != nil {
		panic(err)
	}
	return r
}

func mustAbs(a fixedpoint.Rate) fixedpoint.Rate {
	r, err := a.Abs()
	if err != nil {
		panic(err)
	}
	return r
}

func mustSign(a fixedpoint.Rate) int {
	s, err := a.Sign()
	if err != nil {
		panic(err)
	}
	return s
}

func mustNeg(a fixedpoint.Rate) fixedpoint.Rate {
	r, err := a.Neg()
	if err != nil {
		panic(err)
	}
	return r
}

// ScreenPair runs the full measurement over one matched series. The
// caller has already applied eligibility (MinMatchedSettlements), so a
// series here always has at least two points and a positive span.
func ScreenPair(symbol string, matched []Match, stats PairStats) (PairResult, error) {
	if len(matched) < 2 {
		return PairResult{}, fmt.Errorf("screen %s: %d matched settlements, need at least 2", symbol, len(matched))
	}
	res := PairResult{
		Symbol: symbol,
		Stats:  stats,
		First:  matched[0].At,
		Last:   matched[len(matched)-1].At,
	}
	res.ModalInterval, res.IntervalChanges = modalInterval(matched)
	maxGap := time.Duration(MaxGapFactor) * res.ModalInterval

	abs := make([]fixedpoint.Rate, len(matched))
	for i, m := range matched {
		abs[i] = mustAbs(m.Spread)
	}
	sort.Slice(abs, func(i, j int) bool { return mustCmp(abs[i], abs[j]) < 0 })
	res.MedianAbs = abs[len(abs)/2]
	res.P99Abs = abs[len(abs)*99/100]
	res.MaxAbs = abs[len(abs)-1]

	years, err := yearsRate(res.Last.Sub(res.First))
	if err != nil {
		return PairResult{}, fmt.Errorf("screen %s: %w", symbol, err)
	}
	for _, entry := range EntryLevels {
		lvl, err := screenLevel(matched, entry, maxGap, years)
		if err != nil {
			return PairResult{}, fmt.Errorf("screen %s at %s: %w", symbol, entry.Decimal(), err)
		}
		res.Levels = append(res.Levels, lvl)
	}
	res.Best = 0
	for i := range res.Levels {
		if mustCmp(res.Levels[i].NetAnnual, res.Levels[res.Best].NetAnnual) > 0 {
			res.Best = i
		}
	}
	return res, nil
}

// screenLevel is the episode state machine at one entry level.
func screenLevel(matched []Match, entry fixedpoint.Rate, maxGap time.Duration, years fixedpoint.Rate) (LevelResult, error) {
	exit, err := entry.Div(scrRate(strconv.Itoa(ExitDivisor)), fixedpoint.RoundTowardZero)
	if err != nil {
		return LevelResult{}, err
	}
	roundTrip := RoundTripCost()

	lvl := LevelResult{
		Entry: entry,
		Gross: fixedpoint.ZeroRate(),
		Net:   fixedpoint.ZeroRate(),
	}

	inEpisode := false
	sign := 0
	accrued := fixedpoint.ZeroRate()
	run := 0

	closeEpisode := func() {
		net := mustSub(accrued, roundTrip)
		lvl.Gross = mustAdd(lvl.Gross, accrued)
		lvl.Net = mustAdd(lvl.Net, net)
		lvl.Episodes++
		if mustSign(net) > 0 {
			lvl.Wins++
		}
		if run > lvl.MaxRun {
			lvl.MaxRun = run
		}
		inEpisode = false
		sign = 0
		accrued = fixedpoint.ZeroRate()
		run = 0
	}

	open := func(s int) {
		inEpisode = true
		sign = s
		accrued = fixedpoint.ZeroRate()
		run = 0
	}

	for i, m := range matched {
		// A hole wider than the gap bound ends any open episode first:
		// the settlements inside the hole are unmatched — evidence we
		// do not have — and an episode may not accrue across them.
		if inEpisode && m.At.Sub(matched[i-1].At) > maxGap {
			closeEpisode()
		}

		s := m.Spread
		absS := mustAbs(s)
		sgn := mustSign(s)

		if !inEpisode {
			if mustCmp(absS, entry) >= 0 {
				open(sgn) // the trigger print is observed, never accrued
			}
			continue
		}

		// Accrue this print in the episode's direction — including the
		// exit print: the position is still open when it settles, so a
		// flipped print charges the episode honestly.
		signed := s
		if sign < 0 {
			signed = mustNeg(s)
		}
		accrued = mustAdd(accrued, signed)
		lvl.Accrued++
		run++

		if sgn != sign || mustCmp(absS, exit) < 0 {
			closeEpisode()
			// The same print may trigger a fresh episode in its own
			// direction (a sign flip that lands beyond the entry level).
			if sgn != 0 && mustCmp(absS, entry) >= 0 {
				open(sgn)
			}
		}
	}
	if inEpisode {
		// Data ends with the episode open: close it and charge the full
		// round trip anyway — the conservative reading of an unfinished
		// episode.
		closeEpisode()
	}

	// Continuation evidence over adjacent pairs.
	for i := 0; i+1 < len(matched); i++ {
		if matched[i+1].At.Sub(matched[i].At) > maxGap {
			continue
		}
		si := mustSign(matched[i].Spread)
		if si == 0 || mustCmp(mustAbs(matched[i].Spread), entry) < 0 {
			continue
		}
		lvl.AboveCount++
		if mustSign(matched[i+1].Spread) == si && mustCmp(mustAbs(matched[i+1].Spread), entry) >= 0 {
			lvl.StayCount++
		}
	}
	lvl.Per10k = -1
	if lvl.AboveCount > 0 {
		lvl.Per10k = lvl.StayCount * 10_000 / lvl.AboveCount
	}

	lvl.NetAnnual, err = lvl.Net.Div(years, fixedpoint.RoundTowardZero)
	if err != nil {
		return LevelResult{}, err
	}
	return lvl, nil
}

// modalInterval measures the dominant spacing of the matched series and
// how many spacings differ from it. Spacings are bucketed at the
// pairing round, ties resolved toward the smaller interval (the
// conservative reading: more settlements per day, more chances for the
// grid to disagree with the assumption).
func modalInterval(matched []Match) (time.Duration, int) {
	counts := make(map[time.Duration]int)
	for i := 0; i+1 < len(matched); i++ {
		d := matched[i+1].At.Sub(matched[i].At).Round(PairingRound)
		counts[d]++
	}
	var modal time.Duration
	best := -1
	for d, n := range counts {
		if n > best || (n == best && d < modal) {
			modal, best = d, n
		}
	}
	changes := 0
	for d, n := range counts {
		if d != modal {
			changes += n
		}
	}
	return modal, changes
}

// yearsRate renders a span as years (365.25 days) at the fixedpoint
// scale, for annualization.
func yearsRate(span time.Duration) (fixedpoint.Rate, error) {
	if span <= 0 {
		return fixedpoint.Rate{}, fmt.Errorf("span %s: not positive", span)
	}
	const secondsPerYear = 31_557_600 // 365.25 days
	e9 := span.Milliseconds() * 1_000_000 / secondsPerYear
	if e9 == 0 {
		return fixedpoint.Rate{}, fmt.Errorf("span %s: rounds to zero years", span)
	}
	return fixedpoint.ParseRate(fmt.Sprintf("%d.%09d", e9/1_000_000_000, e9%1_000_000_000))
}
