package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func sizeAt(t *testing.T, entry, cost string) persistence.SizeAt {
	t.Helper()
	return persistence.SizeAt{
		CostAt:      costAt(t, entry, cost),
		LegNotional: mustMoney(t, "1000"),
		Capital:     mustMoney(t, "2000"),
	}
}

func TestPerSymbol_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	got, err := set.PerSymbol(sizeAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("PerSymbol: %v", err)
	}
	if len(got.Rows) != 56 {
		t.Fatalf("rows = %d, want 56", len(got.Rows))
	}
	byName := map[string]persistence.SymbolReturn{}
	for _, r := range got.Rows {
		byName[r.Symbol] = r
	}
	lab, ok := byName["LABUSDT"]
	if !ok {
		t.Fatal("LABUSDT missing from the rows")
	}
	if lab.Days != 277 {
		t.Errorf("LABUSDT Days = %d, want 277", lab.Days)
	}
	if lab.Episodes != 71 {
		t.Errorf("LABUSDT Episodes = %d, want 71", lab.Episodes)
	}
	// Shortlist and ModalMinutes come straight off the symbol record, and both
	// separate LABUSDT from SOLUSDT below: true/240 against false/480. A single
	// symbol could not tell a field wired to the right record from one wired to
	// a constant, and 8 of the file's 61 symbols are on the shortlist, so
	// "always false" would also pass on a majority of rows.
	if !lab.Shortlist {
		t.Error("LABUSDT Shortlist = false, want true")
	}
	if lab.ModalMinutes != 240 {
		t.Errorf("LABUSDT ModalMinutes = %d, want 240", lab.ModalMinutes)
	}
	// No symbol carries a censored record at level 0.001; all five in the file
	// are at 0.0002, and TestPerSymbol_CensoringIsCarriedPerRow asserts them
	// where they are non-zero.
	if lab.Censored != 0 {
		t.Errorf("LABUSDT Censored = %d, want 0", lab.Censored)
	}
	// The task brief quoted the file's cents-rounded display value ("543.88")
	// as the expected Net. Net is a full scale-9 fixedpoint.Money — nothing in
	// symbolRow quantizes it to cents — so the true figure carries the ".166"
	// the file's display rounding discards. Verified two independent ways
	// (a decimal.Decimal fold over episodes.json, and the actual fixedpoint
	// package run against the same records outside this test); both give
	// 543.88166. summary.json's own quantize(D("0.01")) is what produces
	// "543.88" — a display choice, not the underlying total.
	if lab.Net.Decimal() != "543.88166" {
		t.Errorf("LABUSDT Net = %s, want 543.88166", lab.Net.Decimal())
	}
	// The annual return is a RATIO. 35.8% is 0.358. rate.go:8 is explicit that
	// nothing in fixedpoint multiplies by 100, so the percentage the artifact
	// publishes is a rendering choice, not a stored value.
	//
	// The brief's literal here (0.357983221) was also wrong, by the same
	// composed-not-run arithmetic CLAUDE.md warns about: it does not match
	// either an exact Decimal fold or the fixedpoint package's own output.
	// 0.358333585 is what both independently produce, and it is also the only
	// one of the two consistent with the file's own annual_pct_on_2000 ("35.8"
	// — 35.7983 would also round to "35.8", so that check alone cannot tell
	// them apart, which is why two independent recomputations were run rather
	// than trusting the loose percentage match).
	if lab.Annual.Decimal() != "0.358333585" {
		t.Errorf("LABUSDT Annual = %s, want 0.358333585", lab.Annual.Decimal())
	}
	// The decoded span instants are read here, not merely parsed: a symbol
	// whose First/Last decoded into the wrong instant (right layout, wrong
	// value) would still produce a Days figure of some kind, but it would not
	// reproduce these two exact timestamps, which the file's "first"/"last"
	// fields for LABUSDT give as 2025-10-27T08:00:00Z and 2026-07-31T20:00:00Z.
	wantFirst := time.Date(2025, 10, 27, 8, 0, 0, 0, time.UTC)
	wantLast := time.Date(2026, 7, 31, 20, 0, 0, 0, time.UTC)
	if !lab.First.Equal(wantFirst) {
		t.Errorf("LABUSDT First = %s, want %s", lab.First, wantFirst)
	}
	if !lab.Last.Equal(wantLast) {
		t.Errorf("LABUSDT Last = %s, want %s", lab.Last, wantLast)
	}

	// The sign check: most symbols lose money at these costs, and a fold that
	// mishandles sign (e.g. subtracts cost in the wrong direction) passes an
	// all-positive fixture. SOLUSDT is negative at this coordinate — verified
	// the same two independent ways as LABUSDT above.
	sol, ok := byName["SOLUSDT"]
	if !ok {
		t.Fatal("SOLUSDT missing from the rows")
	}
	if sol.Days != 1858 {
		t.Errorf("SOLUSDT Days = %d, want 1858", sol.Days)
	}
	if sol.Episodes != 36 {
		t.Errorf("SOLUSDT Episodes = %d, want 36", sol.Episodes)
	}
	if sol.Shortlist {
		t.Error("SOLUSDT Shortlist = true, want false")
	}
	if sol.ModalMinutes != 480 {
		t.Errorf("SOLUSDT ModalMinutes = %d, want 480", sol.ModalMinutes)
	}
	if sol.Net.Decimal() != "-13.10829" {
		t.Errorf("SOLUSDT Net = %s, want -13.10829", sol.Net.Decimal())
	}
	if sol.Annual.Decimal() != "-0.001287547" {
		t.Errorf("SOLUSDT Annual = %s, want -0.001287547", sol.Annual.Decimal())
	}
}

// F60, made visible. The denominator is a function of the entry level and no
// field in the artifact records it: verdict.total_symbols reads 56 while the
// file's header declares symbols_eligible 61.
func TestPerSymbol_TheDenominatorMovesWithTheLevel(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		level    string
		rows     int
		excluded []persistence.Exclusion
	}{
		// Every committed exclusion carries Censored 0 — all 18 of them are
		// symbols the level never triggered for, not symbols whose every
		// record was censored. Asserting the count rather than only the name
		// is what separates the two facts; the all-censored fixture below
		// carries the other case, where the count is non-zero.
		{"0.0002", 61, nil},
		{"0.0005", 60, []persistence.Exclusion{{Symbol: "XAUTUSDT"}}},
		{"0.001", 56, []persistence.Exclusion{
			{Symbol: "CAPUSDT"}, {Symbol: "CFXUSDT"}, {Symbol: "WLDUSDT"},
			{Symbol: "XAUTUSDT"}, {Symbol: "XPLUSDT"},
		}},
		{"0.002", 49, []persistence.Exclusion{
			{Symbol: "BNBUSDT"}, {Symbol: "BTWUSDT"}, {Symbol: "CAPUSDT"},
			{Symbol: "CFXUSDT"}, {Symbol: "FARTCOINUSDT"}, {Symbol: "LDOUSDT"},
			{Symbol: "OPUSDT"}, {Symbol: "ORDIUSDT"}, {Symbol: "UAIUSDT"},
			{Symbol: "WLDUSDT"}, {Symbol: "XAUTUSDT"}, {Symbol: "XPLUSDT"},
		}},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			got, err := set.PerSymbol(sizeAt(t, c.level, "0.0021"))
			if err != nil {
				t.Fatalf("PerSymbol: %v", err)
			}
			if len(got.Rows) != c.rows {
				t.Errorf("rows = %d, want %d", len(got.Rows), c.rows)
			}
			if len(got.Excluded) != len(c.excluded) {
				t.Fatalf("Excluded = %v, want %v", got.Excluded, c.excluded)
			}
			for i, want := range c.excluded {
				if got.Excluded[i] != want {
					t.Errorf("Excluded[%d] = %+v, want %+v", i, got.Excluded[i], want)
				}
			}
			if len(got.Rows)+len(got.Excluded) != set.SymbolCount() {
				t.Errorf("rows %d + excluded %d != SymbolCount %d",
					len(got.Rows), len(got.Excluded), set.SymbolCount())
			}
		})
	}
}

// ErrSpanDegenerate fires on ZERO rows of the committed file, so it takes a
// hand-built fixture. summarise.py:33's max(1, days) would silently floor the
// denominator here and report an annualised return computed over a day that
// did not happen.
func TestPerSymbol_DegenerateSpanRefuses(t *testing.T) {
	set := loadFixture(t, "testdata/degenerate-span.json")
	_, err := set.PerSymbol(sizeAt(t, "0.001", "0.0021"))
	if !errors.Is(err, persistence.ErrSpanDegenerate) {
		t.Fatalf("PerSymbol = %v, want ErrSpanDegenerate", err)
	}
}

// The positive control for the refusal above: a span of exactly one day is
// ACCEPTED. Without it, a PerSymbol that refused every fixture would score a
// pass on the test above.
func TestPerSymbol_ExactlyOneDaySpanIsAccepted(t *testing.T) {
	set := loadFixture(t, "testdata/one-day-span.json")
	got, err := set.PerSymbol(sizeAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("PerSymbol: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0].Days != 1 {
		t.Fatalf("rows = %+v, want one row with Days 1", got.Rows)
	}
}

// F103's censoring is carried per ROW, and this asserts it where the count is
// FIVE as well as where it is zero. A field asserted only at a level where it
// is zero is satisfied by a field that is always zero.
//
// All five censored records in the committed file sit at entry 0.0002, one
// each on five DIFFERENT symbols (1000RATSUSDT, SOONUSDT, UAIUSDT, USUSDT,
// XMRUSDT), and each of those symbols also has runs at that level — so all five
// land on rows and none on an exclusion. Level 0.001 is the zero control.
func TestPerSymbol_CensoringIsCarriedPerRow(t *testing.T) {
	set := loadCommitted(t)

	censoredAt := func(level string) map[string]int {
		t.Helper()
		got, err := set.PerSymbol(sizeAt(t, level, "0.0021"))
		if err != nil {
			t.Fatalf("PerSymbol at %s: %v", level, err)
		}
		out := map[string]int{}
		for _, r := range got.Rows {
			if r.Censored != 0 {
				out[r.Symbol] = r.Censored
			}
		}
		for _, e := range got.Excluded {
			if e.Censored != 0 {
				out[e.Symbol] = e.Censored
			}
		}
		return out
	}

	want := map[string]int{
		"1000RATSUSDT": 1, "SOONUSDT": 1, "UAIUSDT": 1, "USUSDT": 1, "XMRUSDT": 1,
	}
	got := censoredAt("0.0002")
	if len(got) != len(want) {
		t.Fatalf("censored rows at 0.0002 = %v, want %v", got, want)
	}
	total := 0
	for sym, n := range want {
		if got[sym] != n {
			t.Errorf("%s Censored = %d, want %d", sym, got[sym], n)
		}
		total += got[sym]
	}
	if total != 5 {
		t.Errorf("total censored at 0.0002 = %d, want 5", total)
	}

	// The zero control, at a level where the field must be 0 on every row.
	if zero := censoredAt("0.001"); len(zero) != 0 {
		t.Errorf("censored at 0.001 = %v, want none", zero)
	}
}

// An exclusion carries WHY the symbol was dropped, and this is the case the
// committed file does not contain: a symbol that entered and whose every record
// was right-censored. Its Censored is 3, against the committed exclusions'
// 0 — without this the two reasons are one fact under two names.
func TestPerSymbol_AnAllCensoredSymbolIsExcludedWithItsCount(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.PerSymbol(sizeAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("PerSymbol: %v", err)
	}
	if len(got.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(got.Rows))
	}
	want := persistence.Exclusion{Symbol: "FIXTURE", Censored: 3}
	if len(got.Excluded) != 1 || got.Excluded[0] != want {
		t.Fatalf("Excluded = %+v, want [%+v]", got.Excluded, want)
	}
}

// The documented sort order, asserted directly as an invariant over the rows.
//
// It is NOT asserted index-for-index against summary.json, and that is
// deliberate: summarise.py:115 sorts every published block by
// -annual_pct_on_2000["0.0021"] — ONE fixed cost — while this fold sorts at
// whatever coordinate the caller asked for, and the artifact's key is a
// one-decimal PERCENTAGE while this compares the unrounded ratio. Two symbols
// tying at one decimal place may legitimately order either way, so an
// index-for-index comparison would pass for the wrong reason.
func TestPerSymbol_RowsAreSortedByAnnualThenSymbol(t *testing.T) {
	set := loadCommitted(t)
	for _, cost := range []string{"0.0021", "0.0029", "0.0090"} {
		t.Run(cost, func(t *testing.T) {
			got, err := set.PerSymbol(sizeAt(t, "0.001", cost))
			if err != nil {
				t.Fatalf("PerSymbol: %v", err)
			}
			if len(got.Rows) != 56 {
				t.Fatalf("rows = %d, want 56", len(got.Rows))
			}
			assertSorted(t, got.Rows)
		})
	}
}

// The tiebreak half of the rule needs an exact tie, and the committed file has
// none: all 56 annual ratios at 0.001/0.0021 are DISTINCT (measured), so
// "ties broken by Symbol ascending" is unexercised against it and any tiebreak
// at all — or none — would pass. This fixture is two symbols whose Annual is
// exactly equal, laid out so that neither the file's order nor a plain
// name sort gives the right answer on its own.
func TestPerSymbol_TiesBreakOnSymbolAscending(t *testing.T) {
	set := loadFixture(t, "testdata/annual-tie.json")
	got, err := set.PerSymbol(sizeAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("PerSymbol: %v", err)
	}
	if len(got.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(got.Rows))
	}
	// OMEGA earns twice as much, so it leads despite sorting LAST by name —
	// which is what stops a pure name sort from passing. ALPHA and ZEBRA are
	// exactly equal and appear in the file in the opposite order, which is what
	// stops file order from passing.
	if got.Rows[0].Annual.Decimal() != "0.00895" {
		t.Errorf("Rows[0].Annual = %s, want 0.00895", got.Rows[0].Annual.Decimal())
	}
	if got.Rows[1].Annual.Decimal() != got.Rows[2].Annual.Decimal() {
		t.Fatalf("the tie is not a tie: %s vs %s",
			got.Rows[1].Annual.Decimal(), got.Rows[2].Annual.Decimal())
	}
	want := []string{"OMEGA", "ALPHA", "ZEBRA"}
	for i, name := range want {
		if got.Rows[i].Symbol != name {
			t.Errorf("Rows[%d].Symbol = %s, want %s", i, got.Rows[i].Symbol, name)
		}
	}
	assertSorted(t, got.Rows)
}

// assertSorted checks the documented order directly: Annual non-increasing, and
// Symbol strictly ascending wherever consecutive Annual values are equal.
func assertSorted(t *testing.T, rows []persistence.SymbolReturn) {
	t.Helper()
	for i := 1; i < len(rows); i++ {
		cmp, err := rows[i-1].Annual.Cmp(rows[i].Annual)
		if err != nil {
			t.Fatalf("Cmp rows %d/%d: %v", i-1, i, err)
		}
		if cmp < 0 {
			t.Errorf("Rows[%d].Annual %s < Rows[%d].Annual %s — not descending",
				i-1, rows[i-1].Annual.Decimal(), i, rows[i].Annual.Decimal())
		}
		if cmp == 0 && rows[i-1].Symbol >= rows[i].Symbol {
			t.Errorf("tie at %s: Rows[%d].Symbol = %s, Rows[%d].Symbol = %s — "+
				"ties must break on Symbol ascending",
				rows[i].Annual.Decimal(), i-1, rows[i-1].Symbol, i, rows[i].Symbol)
		}
	}
}
