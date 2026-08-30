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
		excluded []string
	}{
		{"0.0002", 61, nil},
		{"0.0005", 60, []string{"XAUTUSDT"}},
		{"0.001", 56, []string{"CAPUSDT", "CFXUSDT", "WLDUSDT", "XAUTUSDT", "XPLUSDT"}},
		{"0.002", 49, []string{
			"BNBUSDT", "BTWUSDT", "CAPUSDT", "CFXUSDT", "FARTCOINUSDT",
			"LDOUSDT", "OPUSDT", "ORDIUSDT", "UAIUSDT", "WLDUSDT",
			"XAUTUSDT", "XPLUSDT",
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
			for i, name := range c.excluded {
				if got.Excluded[i] != name {
					t.Errorf("Excluded[%d] = %s, want %s", i, got.Excluded[i], name)
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
