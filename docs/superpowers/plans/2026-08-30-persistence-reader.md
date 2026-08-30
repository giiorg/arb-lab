# Persistence Reader Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/strategy/infrastructure/persistence`, a Go reader that recomputes the persistence research answers from `research/persistence/episodes.json` at coordinates the caller names, so that every returned figure carries the entry level, cost and notional it was computed at.

**Architecture:** One concrete package beside `screening/`, no port. `Load` decodes `episodes.json` only. Five query methods each take a coordinate struct (`EntryAt` ⊂ `CostAt` ⊂ `SizeAt`) and return a result struct that **embeds the coordinate it was computed at**. `summary.json` is never decoded in production code — it appears only in the oracle test, where the Go fold is a second independent implementation of `research/persistence/summarise.py` and a disagreement is a finding.

**Tech Stack:** Go 1.26, standard library only (`encoding/json`, `time`, `sort`, `strconv`, `errors`, `fmt`), plus `internal/platform/fixedpoint`. No third-party dependencies. No test framework — standard `testing`.

**Spec:** `docs/superpowers/specs/2026-08-30-persistence-reader-design.md` (design record §103). Read it before Task 1; every rejected alternative in its §3 is a question already settled, and reopening one silently is the failure this repo's decision records exist to prevent.

## Global Constraints

These apply to **every** task. They are enforced by `make ci`, not by review.

- **Module:** `github.com/giiorg/arb-lab`. Package path `internal/strategy/infrastructure/persistence`, package name `persistence`.
- **No `float64` anywhere under `internal/`.** The `nofloat` analyzer is type-based and default-deny. Percentiles, counts and bucket indices are `int`; every quantity is `fixedpoint.Rate` or `fixedpoint.Money`. If you reach for a float to compute a mean, you have taken a wrong turn — see Task 3 Step 3.
- **No ambient clock reads.** `time.Now()` never appears. `time.Parse` and `time.Time` arithmetic on parsed values are fine; the `notimenow` analyzer bans the *ambient read*, not the package.
- **Every rounding is an explicit argument.** `fixedpoint` operations that can lose precision take a `RoundingMode`. This plan uses `fixedpoint.RoundHalfEven` for every such call, because `rounding.go` names it for "valuation and PnL, where a biased tie rule accumulates across thousands of settlements" — and this fold runs over 17 478 episodes. Do not substitute another mode; the oracle expectations in Task 8 were measured under half-even.
- **`fixedpoint.Scale` is 9.** `summary.json` publishes values at up to 28 decimal places (Python `Decimal`). Every comparison against the file is *"agrees when the Python value is rounded to 9 places under the declared mode"*, never byte equality. Say so in the test's own words.
- **Rates are ratios, never percentages.** `rate.go:8`: *"Nothing in this package multiplies or divides by 100."* The artifact's `annual_pct_on_2000` and `hurdle_pct` are percentages; the ×100 conversion lives **only** in the oracle test.
- **The zero value is unusable.** Every coordinate struct gets a `Validate()` that fails when nobody populated it.
- **No spec edits.** `docs/build-spec.md` stays at v0.9.74. `internal/strategy/application/ports/ports.go` is not touched (its `"seven in all"` count word is a normative claim). No changes to `summarise.py`, `harness.go.txt`, or the committed artifacts.
- **Commit messages carry no AI or model attribution trailer.** Ever.
- **Verification command for every task:** `go test ./internal/strategy/infrastructure/persistence/...` — and before the final commit, `make ci`.

---

## File Structure

Nine files, all new, all in `internal/strategy/infrastructure/persistence/`.

| File | Responsibility |
|---|---|
| `doc.go` | Package doc: what this is, why there is no port, why it recomputes. |
| `errors.go` | The three refusal sentinels. |
| `coords.go` | `EntryAt`, `CostAt`, `SizeAt`, `Hurdle` and their `Validate()`. |
| `decode.go` | `Load`, the wire structs, the two timestamp layouts (F102), the censored/uncensored split (F103), `Set` and its lookup. |
| `runlength.go` | `RunLength`, `Set.RunLength`, the nearest-rank percentile. |
| `breakeven.go` | `BreakevenRow`, `BreakevenTable`, `Set.Breakeven`. |
| `net.go` | `NetPerEpisode`, `Set.NetPerEpisode`. |
| `persymbol.go` | `SymbolReturn`, `SymbolReturns`, `Set.PerSymbol`, the span-in-days rule. |
| `verdict.go` | `Verdict`, `Set.Verdict`. |

Six test files:

| File | Holds |
|---|---|
| `coords_test.go` | Coordinate refusals. |
| `decode_test.go` | Decode counts, both timestamp layouts, F102's RFC3339 demonstration, F103's censoring. |
| `runlength_test.go` | Percentile boundaries, censored exclusion, hand-built fixtures. |
| `net_test.go` | The `>` vs `>=` fixture (§6.2 item 2). |
| `persymbol_test.go` | `ErrSpanDegenerate`'s hand-built fixture, `Excluded`. |
| `oracle_test.go` | The full comparison against `summary.json`, plus its negative control. |

Hand-built fixtures live in `testdata/`, following `internal/platform/fixedpoint/testdata` and seven other packages.

---

## Task 1: Package skeleton, coordinates, refusals

**Files:**
- Create: `internal/strategy/infrastructure/persistence/doc.go`
- Create: `internal/strategy/infrastructure/persistence/errors.go`
- Create: `internal/strategy/infrastructure/persistence/coords.go`
- Test: `internal/strategy/infrastructure/persistence/coords_test.go`

**Interfaces:**
- Consumes: `internal/platform/fixedpoint` (`Rate`, `Money`, `ParseRate`, `ParseMoney`, `Asset`).
- Produces: `EntryAt`, `CostAt`, `SizeAt`, `Hurdle`, each with `Validate() error`; sentinels `ErrLevelUndeclared`, `ErrCoordinateUnset`, `ErrSpanDegenerate`.

- [ ] **Step 1: Write the failing test**

Create `coords_test.go`:

```go
package persistence_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func mustRate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func mustMoney(t *testing.T, s string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", s)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", s, err)
	}
	return m
}

// The zero value of every coordinate is refused. An unset entry level would
// otherwise read as 0.0002 rounded down to nothing, and an unset leg notional
// would read as a zero-size position whose annualised return is zero — both
// answer the research question in our favour with nothing failing.
func TestCoordinates_ZeroValueIsRefused(t *testing.T) {
	cases := map[string]interface{ Validate() error }{
		"EntryAt": persistence.EntryAt{},
		"CostAt":  persistence.CostAt{},
		"SizeAt":  persistence.SizeAt{},
		"Hurdle":  persistence.Hurdle{},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := c.Validate()
			if !errors.Is(err, persistence.ErrCoordinateUnset) {
				t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
			}
		})
	}
}

// Each coordinate validates EVERY field it carries, including the ones it
// inherits by embedding. A CostAt whose Cost is set but whose embedded Entry is
// not is the realistic accident: the caller filled in the field the method name
// mentions and forgot the one the type nests.
func TestCoordinates_EachFieldIsChecked(t *testing.T) {
	entry := mustRate(t, "0.001")
	cost := mustRate(t, "0.0021")
	leg := mustMoney(t, "1000")
	cap2000 := mustMoney(t, "2000")

	t.Run("CostAt missing the embedded entry", func(t *testing.T) {
		c := persistence.CostAt{Cost: cost}
		if err := c.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("CostAt missing its own cost", func(t *testing.T) {
		c := persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}}
		if err := c.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("SizeAt missing the capital", func(t *testing.T) {
		s := persistence.SizeAt{
			CostAt:      persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			LegNotional: leg,
		}
		if err := s.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("SizeAt missing the leg notional", func(t *testing.T) {
		s := persistence.SizeAt{
			CostAt:  persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			Capital: cap2000,
		}
		if err := s.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("Hurdle missing its class and source", func(t *testing.T) {
		h := persistence.Hurdle{Annual: mustRate(t, "0.046")}
		if err := h.Validate(); !errors.Is(err, persistence.ErrCoordinateUnset) {
			t.Fatalf("Validate() = %v, want ErrCoordinateUnset", err)
		}
	})
	t.Run("fully populated coordinates pass", func(t *testing.T) {
		// The positive control. Without it, a Validate() that returned an error
		// unconditionally would score a perfect five above.
		s := persistence.SizeAt{
			CostAt:      persistence.CostAt{EntryAt: persistence.EntryAt{Entry: entry}, Cost: cost},
			LegNotional: leg,
			Capital:     cap2000,
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("SizeAt.Validate() = %v, want nil", err)
		}
		h := persistence.Hurdle{
			Annual: mustRate(t, "0.046"),
			Class:  "illustrative_constant",
			Source: "research/persistence/summarise.py HURDLE_PCT",
		}
		if err := h.Validate(); err != nil {
			t.Fatalf("Hurdle.Validate() = %v, want nil", err)
		}
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestCoordinates -v`

Expected: FAIL — the package does not exist. Go reports `no required module provides package ...` or `no Go files in ...`.

- [ ] **Step 3: Write `doc.go`**

```go
// Package persistence reads research/persistence/episodes.json — the
// per-episode expansion the §99 persistence study produced — and recomputes
// its answers at coordinates the CALLER names.
//
// It exists because record §103 measured a structural absence: summary.json
// and episodes.json had zero Go readers, and four separate coordinate losses
// (F55, F60, F79, F83) live in that gap. The contrast is one directory over —
// screening/report.go declares Go types for its three artifacts and has none.
//
// # Why it recomputes rather than decoding summary.json
//
// summarise.py computes four blocks and records the coordinates of one. Its
// run_vs_breakeven block is pinned to entry level 0.0005 and cost 0.0021; its
// per_symbol and verdict blocks are pinned to level 0.001; none of the three
// writes the pin down. A reader that decoded that file could not supply what
// the file never wrote, so its refusals would be permanent — and serving the
// blocks anyway would mean hard-coding summarise.py's pins as Go constants,
// which is a constant asserting an external fact about a script that can
// change without the constant changing.
//
// So this package parses ONLY episodes.json, which carries `entry` on every
// element, and folds it here. Every result struct embeds the coordinate it was
// computed at, which makes the loss unrepresentable rather than annotated.
//
// The accepted consequence is that this fold is a second independent
// implementation of summarise.py. That is this repo's standing method — real
// defects here have been found by two implementations disagreeing, not by
// review — and a disagreement on the committed data is a finding, not a test
// to loosen.
//
// # Why there is no port
//
// strategy/application/ports declares "seven in all", which is a normative
// count. More decisively: the consumer that would own an eighth entry is a
// phase-3 dashboard context that does not exist, and a driven port with no
// caller is a check that cannot fail. Phase 3 declares the interface it needs;
// this package satisfies it.
package persistence
```

- [ ] **Step 4: Write `errors.go`**

```go
package persistence

import "errors"

// The refusals. Callers branch on these with errors.Is.
var (
	// ErrLevelUndeclared reports an entry level the file does not carry.
	// Asking for 0.0003 REFUSES; it does not return an empty table. An empty
	// table is indistinguishable from "this level was measured and nothing
	// happened", which is the reading that turns a typo into a finding.
	ErrLevelUndeclared = errors.New("persistence: entry level not declared in the file")

	// ErrCoordinateUnset reports a coordinate whose Validate() failed — a
	// field nobody populated reaching a computation as though it were a value.
	ErrCoordinateUnset = errors.New("persistence: coordinate unset")

	// ErrSpanDegenerate reports a symbol whose observed span is under one day.
	//
	// It fires on ZERO rows of the committed file (measured: 0 of 61 symbols).
	// It is declared anyway, because summarise.py:33 writes max(1, days) — a
	// papering-over that inflates an annualised return without failing
	// anything. A gate's refusing path is the untested one, so this one gets a
	// hand-built fixture rather than a committed-data one.
	ErrSpanDegenerate = errors.New("persistence: symbol span is under one day")
)
```

- [ ] **Step 5: Write `coords.go`**

```go
package persistence

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The coordinates, as nesting types.
//
// The nesting IS record §103's claim, expressed as a signature rather than as
// a sentence: run length is a function of the entry level alone
// (summarise.py:58 — cost enters only wins and mean_net), break-even and net
// need a cost as well, and annualisation needs a size on top of that. You
// cannot pass a cost to RunLength because EntryAt has no field for one, and you
// cannot call PerSymbol without naming the leg notional that the field name
// annual_pct_on_2000 omits.
//
// The leg notional is the fifth coordinate, and it is in no field and no row of
// the artifact: summarise.py:96 pins CAP = 2000 and LEG = 1000, the field name
// carries the capital, and the per-leg notional scales the numerator with
// nothing recording it. That is why SizeAt carries two Money values.

// EntryAt names the entry level. It is what run length depends on, and nothing
// more.
type EntryAt struct {
	Entry fixedpoint.Rate
}

// Validate reports whether the coordinate was populated.
func (a EntryAt) Validate() error {
	if err := a.Entry.Validate(); err != nil {
		return fmt.Errorf("%w: entry level: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// CostAt adds the round-trip cost, which break-even and net depend on.
type CostAt struct {
	EntryAt
	Cost fixedpoint.Rate
}

// Validate reports whether the coordinate and everything it embeds was
// populated.
func (a CostAt) Validate() error {
	if err := a.EntryAt.Validate(); err != nil {
		return err
	}
	if err := a.Cost.Validate(); err != nil {
		return fmt.Errorf("%w: cost: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// SizeAt adds the two notionals. LegNotional scales the numerator; Capital is
// the denominator the annualised return is expressed against.
type SizeAt struct {
	CostAt
	LegNotional fixedpoint.Money
	Capital     fixedpoint.Money
}

// Validate reports whether the coordinate and everything it embeds was
// populated.
func (a SizeAt) Validate() error {
	if err := a.CostAt.Validate(); err != nil {
		return err
	}
	if err := a.LegNotional.Validate(); err != nil {
		return fmt.Errorf("%w: leg notional: %v", ErrCoordinateUnset, err)
	}
	if err := a.Capital.Validate(); err != nil {
		return fmt.Errorf("%w: capital: %v", ErrCoordinateUnset, err)
	}
	return nil
}

// Hurdle is the benchmark a verdict compares against. It is an argument and
// never a constant.
//
// F45 is the reason. summarise.py's HURDLE_PCT is an illustrative constant: it
// is NOT reference data (record §100 §8c defines that as configs/reference.*.json
// specifically, which carry 0.0369 rather than 0.046), and it is not a
// calibrated benchmark per build-spec §55, whose input is dated and
// haircut-adjusted. A Go constant here would re-commit F45's defect in a second
// language, so Class and Source are required fields: a hurdle that cannot say
// what kind of number it is cannot be rendered honestly beside a count computed
// from it.
//
// Annual is a RATIO, not a percentage: 4.6% is 0.046.
type Hurdle struct {
	Annual fixedpoint.Rate
	Class  string
	Source string
}

// Validate reports whether the hurdle carries its value AND its provenance.
func (h Hurdle) Validate() error {
	if err := h.Annual.Validate(); err != nil {
		return fmt.Errorf("%w: hurdle rate: %v", ErrCoordinateUnset, err)
	}
	if h.Class == "" {
		return fmt.Errorf("%w: hurdle class", ErrCoordinateUnset)
	}
	if h.Source == "" {
		return fmt.Errorf("%w: hurdle source", ErrCoordinateUnset)
	}
	return nil
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestCoordinates -v`

Expected: PASS, all seven subtests.

- [ ] **Step 7: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): coordinate types and refusal sentinels

The nesting EntryAt < CostAt < SizeAt turns record 103's claim about which
coordinates each answer depends on into a signature rather than a sentence."
```

---

## Task 2: The decoder

**Files:**
- Create: `internal/strategy/infrastructure/persistence/decode.go`
- Test: `internal/strategy/infrastructure/persistence/decode_test.go`

**Interfaces:**
- Consumes: Task 1's `EntryAt`, `ErrLevelUndeclared`.
- Produces:
  - `func Load(path string) (Set, error)`
  - `type Set struct{ ... }` with `func (s Set) Levels() []fixedpoint.Rate` and `func (s Set) SymbolCount() int`
  - `type Episode struct { Run int; Accrued fixedpoint.Rate; Start, End time.Time }`
  - unexported `func (s Set) at(a EntryAt) ([]symbolLevel, error)` — every query method in Tasks 3–7 calls this and nothing else.
  - constants `EpisodeInstantLayout`, `SpanInstantLayout`

**Facts this task encodes (all measured against the committed file):**
- 61 symbols; 4 declared levels: `0.0002`, `0.0005`, `0.001`, `0.002`.
- Episode counts per level, censored excluded: 12 317 / 3 425 / 1 259 / 472. Censored counts: **5 / 0 / 0 / 0**.
- 34 956 episode instants at 17 characters, 122 symbol-span instants at 20 (F102).

- [ ] **Step 1: Write the failing test**

Create `decode_test.go`:

```go
package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

// committedEpisodes is the artifact this package was built to read. The path is
// relative to this package directory, four levels below the repo root.
const committedEpisodes = "../../../../research/persistence/episodes.json"

func loadCommitted(t *testing.T) persistence.Set {
	t.Helper()
	set, err := persistence.Load(committedEpisodes)
	if err != nil {
		t.Fatalf("Load(%q): %v", committedEpisodes, err)
	}
	return set
}

func TestLoad_DecodesTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)

	if got, want := set.SymbolCount(), 61; got != want {
		t.Errorf("SymbolCount() = %d, want %d", got, want)
	}
	levels := set.Levels()
	want := []string{"0.0002", "0.0005", "0.001", "0.002"}
	if len(levels) != len(want) {
		t.Fatalf("Levels() has %d entries, want %d", len(levels), len(want))
	}
	for i, lv := range levels {
		if lv.Decimal() != want[i] {
			t.Errorf("Levels()[%d] = %s, want %s", i, lv.Decimal(), want[i])
		}
	}
}

// The four levels carry four DIFFERENT episode counts. This is the separating
// member record §103 §6.2 names: p75 is 3 at every level, so an assertion on
// p75 passes whatever the code does with the level argument.
func TestLoad_EpisodeCountsSeparateTheLevels(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		level    string
		episodes int
		censored int
	}{
		{"0.0002", 12317, 5},
		{"0.0005", 3425, 0},
		{"0.001", 1259, 0},
		{"0.002", 472, 0},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, c.level)})
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}
			if rl.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", rl.Episodes, c.episodes)
			}
			if rl.Censored != c.censored {
				t.Errorf("Censored = %d, want %d", rl.Censored, c.censored)
			}
		})
	}
}

// An undeclared level REFUSES. It does not return an empty answer, which is
// what a caller with a typo would read as "measured, and nothing happened".
func TestLoad_UndeclaredLevelRefuses(t *testing.T) {
	set := loadCommitted(t)
	_, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0003")})
	if !errors.Is(err, persistence.ErrLevelUndeclared) {
		t.Fatalf("RunLength(0.0003) = %v, want ErrLevelUndeclared", err)
	}
}

// F102: the emitter writes TWO timestamp layouts, 133 lines apart, and neither
// is RFC3339. Go's time.RFC3339 rejects the episode layout outright. This test
// is the record of that, and it is what dates the defect: summarise.py parses
// only the symbol-level pair, and Python's fromisoformat accepts both anyway,
// so 34 956 episode instants have never been parsed by any consumer.
func TestLoad_TwoTimestampLayouts(t *testing.T) {
	t.Run("the episode layout is not RFC3339", func(t *testing.T) {
		const sample = "2025-10-27T08:00Z"
		if _, err := time.Parse(time.RFC3339, sample); err == nil {
			t.Fatal("time.RFC3339 parsed the episode layout; F102 would be stale")
		}
		if _, err := time.Parse(persistence.EpisodeInstantLayout, sample); err != nil {
			t.Fatalf("EpisodeInstantLayout rejected %q: %v", sample, err)
		}
	})
	t.Run("the span layout is RFC3339-shaped", func(t *testing.T) {
		const sample = "2025-10-27T08:00:00Z"
		if _, err := time.Parse(persistence.SpanInstantLayout, sample); err != nil {
			t.Fatalf("SpanInstantLayout rejected %q: %v", sample, err)
		}
	})
	t.Run("both are decoded from the committed file", func(t *testing.T) {
		set := loadCommitted(t)
		rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0002")})
		if err != nil {
			t.Fatalf("RunLength: %v", err)
		}
		// The censored records are the ones whose instants we can name exactly:
		// all five sit at the tape's final decision point.
		if rl.Censored != 5 {
			t.Fatalf("Censored = %d, want 5", rl.Censored)
		}
	})
}

// A path that is not a file refuses, rather than yielding an empty Set whose
// every answer would be a confident zero.
func TestLoad_MissingFileRefuses(t *testing.T) {
	if _, err := persistence.Load("testdata/no-such-file.json"); err == nil {
		t.Fatal("Load of a missing path returned nil error")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestLoad -v`

Expected: FAIL to compile — `undefined: persistence.Load`, `undefined: persistence.Set`, `undefined: persistence.EpisodeInstantLayout`.

`RunLength` is also undefined, and it arrives in Task 3. **Do not stub it.** A `RunLength` returning zeroes so that Task 2 can commit alone is a check that cannot fail, which is the exact shape this repo has paid to avoid. Instead, Task 2 and Task 3 share one commit: run Task 2's decode-only subtests with `-run 'TestLoad_(DecodesTheCommittedFile|MissingFileRefuses)'`, and Task 3 turns the rest green.

- [ ] **Step 3: Write `decode.go`**

```go
package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The two timestamp layouts research/persistence/harness.go.txt emits, and
// F102 is that there are two.
//
// Line 120–121 formats episode instants as "2006-01-02T15:04Z"; line 253–254
// formats symbol-span instants as "2006-01-02T15:04:05Z". Neither is
// time.RFC3339, and time.RFC3339 REJECTS the first:
//
//	parsing time "2025-10-27T08:00Z" as "2006-01-02T15:04:05Z07:00":
//	cannot parse "Z" as ":"
//
// Measured over the committed file: 122 span instants at 20 characters, 34 956
// episode instants at 17. Every other committed research artifact stamps
// RFC3339, and screening/report.go:178 formats with time.RFC3339 by name.
//
// This reader accepts the file's ACTUAL layout and names it. Correcting the
// emitter would require regenerating episodes.json, whose input
// research/screening/raw/ is gitignored — a re-fetch of 468 venue files. The
// defect is recorded in record §103 §7, not fixed here.
//
// The general form: a field nothing parses is not a typed field. A format
// defect survives indefinitely behind a permissive decoder, and the first
// strict reader is what dates it.
const (
	// EpisodeInstantLayout is the minute-resolution layout on every episode.
	EpisodeInstantLayout = "2006-01-02T15:04Z"

	// SpanInstantLayout is the second-resolution layout on every symbol span.
	SpanInstantLayout = "2006-01-02T15:04:05Z"
)

// wireSymbol, wireLevel and wireEpisode are episodes.json verbatim. They exist
// only inside Load; nothing outside this file sees a JSON tag.
type wireSymbol struct {
	Symbol       string      `json:"symbol"`
	Shortlist    bool        `json:"shortlist"`
	Matched      int         `json:"matched"`
	First        string      `json:"first"`
	Last         string      `json:"last"`
	ModalMinutes int         `json:"modal_minutes"`
	Levels       []wireLevel `json:"levels"`
}

type wireLevel struct {
	Entry    string        `json:"entry"`
	Episodes []wireEpisode `json:"episodes"`
}

type wireEpisode struct {
	Run     int    `json:"run"`
	Accrued string `json:"accrued"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// Episode is one entry-to-exit run, decoded.
type Episode struct {
	// Run is the number of settlements the position was held across.
	Run int
	// Accrued is the gross funding collected over the run, as a ratio.
	Accrued fixedpoint.Rate
	// Start and End are the run's bounds, at minute resolution (see
	// EpisodeInstantLayout).
	Start time.Time
	End   time.Time
}

// symbolLevel is one symbol's episodes at one entry level, already split.
type symbolLevel struct {
	symbol   *symbolRecord
	episodes []Episode // runs: what a run-length or net question answers over
	censored []Episode // right-censored observations: NOT runs
}

type symbolRecord struct {
	name         string
	shortlist    bool
	matched      int
	first        time.Time
	last         time.Time
	modalMinutes int
}

// Set is a decoded episodes.json. Its zero value carries no data and every
// query against it refuses with ErrLevelUndeclared, which is the correct
// reading: a Set nobody loaded declares no levels.
type Set struct {
	symbols []*symbolRecord
	// byLevel is keyed by the entry level's mantissa, which is exact. Keying by
	// the file's decimal TEXT would make "0.001" and "0.0010" different levels.
	byLevel map[int64][]symbolLevel
	levels  []fixedpoint.Rate
}

// Load decodes episodes.json at path. There is no ambient default path and no
// clock read: the caller names the file.
func Load(path string) (Set, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Set{}, fmt.Errorf("persistence: read %s: %w", path, err)
	}
	var wire []wireSymbol
	if err := json.Unmarshal(data, &wire); err != nil {
		return Set{}, fmt.Errorf("persistence: decode %s: %w", path, err)
	}

	set := Set{byLevel: map[int64][]symbolLevel{}}
	seen := map[int64]fixedpoint.Rate{}

	for _, ws := range wire {
		first, err := time.Parse(SpanInstantLayout, ws.First)
		if err != nil {
			return Set{}, fmt.Errorf("persistence: %s: span first %q: %w", ws.Symbol, ws.First, err)
		}
		last, err := time.Parse(SpanInstantLayout, ws.Last)
		if err != nil {
			return Set{}, fmt.Errorf("persistence: %s: span last %q: %w", ws.Symbol, ws.Last, err)
		}
		rec := &symbolRecord{
			name: ws.Symbol, shortlist: ws.Shortlist, matched: ws.Matched,
			first: first, last: last, modalMinutes: ws.ModalMinutes,
		}
		set.symbols = append(set.symbols, rec)

		for _, wl := range ws.Levels {
			entry, err := fixedpoint.ParseRate(wl.Entry)
			if err != nil {
				return Set{}, fmt.Errorf("persistence: %s: entry %q: %w", ws.Symbol, wl.Entry, err)
			}
			key := entry.Mantissa()
			if _, ok := seen[key]; !ok {
				seen[key] = entry
				set.levels = append(set.levels, entry)
			}
			sl := symbolLevel{symbol: rec}
			for _, we := range wl.Episodes {
				ep, err := decodeEpisode(ws.Symbol, we)
				if err != nil {
					return Set{}, err
				}
				// F103: five records carry "run": 0, all at level 0.0002, all
				// with start == end, all accruing "0", all sitting at the
				// tape's final decision point. They are entries triggered with
				// no settlement after them — right-censored observations, not
				// runs of length zero — so they do not join a run-length
				// population. The count is REPORTED rather than swallowed.
				if ep.Run == 0 {
					sl.censored = append(sl.censored, ep)
					continue
				}
				sl.episodes = append(sl.episodes, ep)
			}
			set.byLevel[key] = append(set.byLevel[key], sl)
		}
	}

	sort.Slice(set.levels, func(i, j int) bool {
		return set.levels[i].Mantissa() < set.levels[j].Mantissa()
	})
	return set, nil
}

func decodeEpisode(symbol string, we wireEpisode) (Episode, error) {
	accrued, err := fixedpoint.ParseRate(we.Accrued)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: accrued %q: %w", symbol, we.Accrued, err)
	}
	start, err := time.Parse(EpisodeInstantLayout, we.Start)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: episode start %q: %w", symbol, we.Start, err)
	}
	end, err := time.Parse(EpisodeInstantLayout, we.End)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: episode end %q: %w", symbol, we.End, err)
	}
	if we.Run < 0 {
		return Episode{}, fmt.Errorf("persistence: %s: negative run %d", symbol, we.Run)
	}
	return Episode{Run: we.Run, Accrued: accrued, Start: start, End: end}, nil
}

// Levels returns the entry levels the file declares, ascending.
func (s Set) Levels() []fixedpoint.Rate {
	out := make([]fixedpoint.Rate, len(s.levels))
	copy(out, s.levels)
	return out
}

// SymbolCount returns how many symbols the file carries, before any level's
// own exclusions. Compare SymbolReturns.Excluded, which is where the
// denominator actually moves.
func (s Set) SymbolCount() int { return len(s.symbols) }

// at resolves a coordinate to the file's rows, refusing an undeclared level.
// Every query method goes through here.
func (s Set) at(a EntryAt) ([]symbolLevel, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	rows, ok := s.byLevel[a.Entry.Mantissa()]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrLevelUndeclared, a.Entry.Decimal())
	}
	return rows, nil
}
```

- [ ] **Step 4: Run the two decode-only tests to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run 'TestLoad_(DecodesTheCommittedFile|MissingFileRefuses|TwoTimestampLayouts/the)' -v`

Expected: PASS. The subtests calling `RunLength` still fail to compile, so run Task 3 before committing the whole file — or comment nothing out and simply proceed to Task 3, committing both together.

- [ ] **Step 5: Commit (after Task 3 compiles the file)**

Deferred to Task 3 Step 6, because `decode_test.go` references `RunLength`. Do not stub `RunLength` to make this task commit alone; a stub returning zeroes is exactly the shape this repo calls a check that cannot fail.

---

## Task 3: RunLength, and the nearest-rank percentile

**Files:**
- Create: `internal/strategy/infrastructure/persistence/runlength.go`
- Create: `internal/strategy/infrastructure/persistence/runlength_test.go`
- Test: also turns Task 2's remaining subtests green.

**Interfaces:**
- Consumes: Task 2's `Set.at`, `symbolLevel`, `Episode`; Task 1's `EntryAt`.
- Produces:
  - `type RunLength struct { P50, P75, P90, Max int; Mean fixedpoint.Rate; Episodes, Censored int; At EntryAt }`
  - `func (s Set) RunLength(at EntryAt) (RunLength, error)`
  - unexported `func percentile(sorted []int, q int) int`

**Measured expectations (committed file, censored excluded):**

| Level | Episodes | Censored | P50 | P75 | P90 | Max | Mean (3 dp) |
|---|---|---|---|---|---|---|---|
| 0.0002 | 12317 | 5 | 1 | 3 | 5 | 71 | 2.434 |
| 0.0005 | 3425 | 0 | 2 | 3 | 6 | 36 | 2.630 |
| 0.001 | 1259 | 0 | 2 | 3 | 5 | 20 | 2.572 |
| 0.002 | 472 | 0 | 1 | 3 | 4 | 20 | 2.176 |

Note the 0.0002 row: `summary.json` publishes `run_mean` **2.433** and `episodes` **12322**, because summarise.py counts the five censored records as episodes of length zero. This reader reads 2.434 over 12 317. **That disagreement is F103's magnitude and is expected**; Task 8's oracle asserts it explicitly rather than tolerating it.

- [ ] **Step 1: Write the failing test**

Create `runlength_test.go`:

```go
package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func TestRunLength_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		level                 string
		p50, p75, p90, max    int
		episodes, censored    int
		mean                  string // the Rate's Decimal() form, 9-place canonical
	}{
		{"0.0002", 1, 3, 5, 71, 12317, 5, "2.433628318"},
		{"0.0005", 2, 3, 6, 36, 3425, 0, "2.629927007"},
		{"0.001", 2, 3, 5, 20, 1259, 0, "2.571882446"},
		{"0.002", 1, 3, 4, 20, 472, 0, "2.176271186"},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, c.level)})
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}
			if got.P50 != c.p50 || got.P75 != c.p75 || got.P90 != c.p90 || got.Max != c.max {
				t.Errorf("percentiles = %d/%d/%d max %d, want %d/%d/%d max %d",
					got.P50, got.P75, got.P90, got.Max, c.p50, c.p75, c.p90, c.max)
			}
			if got.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", got.Episodes, c.episodes)
			}
			if got.Censored != c.censored {
				t.Errorf("Censored = %d, want %d", got.Censored, c.censored)
			}
			if got.Mean.Decimal() != c.mean {
				t.Errorf("Mean = %s, want %s", got.Mean.Decimal(), c.mean)
			}
			// The coordinate rides on the value. This is the whole point of the
			// package: a screen renders what it holds, and HANDOFF.md restating
			// "median episode is 2 settlements" with no level is what a correct
			// call looks like three hops downstream — at two of the four levels
			// the median is 1.
			if got.At.Entry.Decimal() != c.level {
				t.Errorf("At.Entry = %s, want %s", got.At.Entry.Decimal(), c.level)
			}
		})
	}
}

// The percentile is nearest-rank on the SORTED population, replicating
// summarise.py:38 exactly: v[min(len-1, (len*q)//100)]. Both off-by-one
// directions must be visible, so the fixture is a population whose q-th element
// differs from its neighbours at every quantile the reader reports.
func TestRunLength_PercentileIsNearestRank(t *testing.T) {
	// runs 1..10, one episode each. Nearest-rank indices:
	//   p50 -> min(9, 100//100=... ) for len=10: (10*50)//100 = 5 -> value 6
	//   p75 -> (10*75)//100 = 7 -> value 8
	//   p90 -> (10*90)//100 = 9 -> value 10
	// Every one of the three differs from its neighbour by exactly 1, so an
	// index shifted either way changes the answer.
	set := loadFixture(t, "testdata/percentile.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.P50 != 6 || got.P75 != 8 || got.P90 != 10 {
		t.Fatalf("percentiles = %d/%d/%d, want 6/8/10", got.P50, got.P75, got.P90)
	}
	if got.Max != 10 {
		t.Errorf("Max = %d, want 10", got.Max)
	}
}

// Nearest-rank is not a midpoint rule, and a two-element population is where
// the difference is loudest: for runs 3 and 9 the 50th percentile is 9, the
// SECOND element, because (2*50)//100 = 1. Any implementation that averaged the
// two middle values, or that used (len-1)*q/100, would answer 3 or 6 here.
//
// It also pins the top of the index range: (2*90)//100 = 1 = len-1, the last
// legal index. The min(len-1, ...) clamp itself fires only for q >= 100, which
// this package never asks — so this is the boundary the code must not cross,
// not a test of the clamp.
func TestRunLength_PercentileIsNotAMidpoint(t *testing.T) {
	set := loadFixture(t, "testdata/two-episodes.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.P50 != 9 {
		t.Errorf("P50 = %d, want 9 — nearest rank on [3 9] at q=50 is index 1", got.P50)
	}
	if got.P90 != 9 {
		t.Errorf("P90 = %d, want 9 (index len-1)", got.P90)
	}
	if got.Max != 9 {
		t.Errorf("Max = %d, want 9", got.Max)
	}
}

// A level with censored records but no runs at all still answers, and answers
// honestly: zero episodes, the censored count reported, and no percentile
// invented from an empty population.
func TestRunLength_AllCensoredIsNotAnEmptyLevel(t *testing.T) {
	set := loadFixture(t, "testdata/all-censored.json")
	got, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.001")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if got.Episodes != 0 || got.Censored != 3 {
		t.Fatalf("Episodes/Censored = %d/%d, want 0/3", got.Episodes, got.Censored)
	}
	if got.P50 != 0 || got.P75 != 0 || got.P90 != 0 || got.Max != 0 {
		t.Errorf("percentiles over an empty population = %d/%d/%d/%d, want all 0",
			got.P50, got.P75, got.P90, got.Max)
	}
	if got.Mean.Validate() != nil {
		t.Error("Mean over an empty population must be a constructed zero, not the zero value")
	}
}
```

Add the fixture loader to `decode_test.go` (or a new `helpers_test.go` — keep it in `decode_test.go`):

```go
func loadFixture(t *testing.T, path string) persistence.Set {
	t.Helper()
	set, err := persistence.Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	return set
}
```

- [ ] **Step 2: Write the three fixtures**

Create `testdata/percentile.json`:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":10,"first":"2026-01-01T00:00:00Z","last":"2026-12-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":1,"accrued":"0.001","start":"2026-01-01T00:00Z","end":"2026-01-01T08:00Z"},
{"run":2,"accrued":"0.001","start":"2026-01-02T00:00Z","end":"2026-01-02T16:00Z"},
{"run":3,"accrued":"0.001","start":"2026-01-03T00:00Z","end":"2026-01-04T00:00Z"},
{"run":4,"accrued":"0.001","start":"2026-01-05T00:00Z","end":"2026-01-06T08:00Z"},
{"run":5,"accrued":"0.001","start":"2026-01-07T00:00Z","end":"2026-01-08T16:00Z"},
{"run":6,"accrued":"0.001","start":"2026-01-09T00:00Z","end":"2026-01-11T00:00Z"},
{"run":7,"accrued":"0.001","start":"2026-01-12T00:00Z","end":"2026-01-14T08:00Z"},
{"run":8,"accrued":"0.001","start":"2026-01-15T00:00Z","end":"2026-01-17T16:00Z"},
{"run":9,"accrued":"0.001","start":"2026-01-18T00:00Z","end":"2026-01-21T00:00Z"},
{"run":10,"accrued":"0.001","start":"2026-01-22T00:00Z","end":"2026-01-25T08:00Z"}
]}]}]
```

Create `testdata/two-episodes.json` — runs 3 and 9. Both `(2*50)//100` and `(2*90)//100` are 1, so both percentiles are the second element, 9:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":2,"first":"2026-01-01T00:00:00Z","last":"2026-06-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":3,"accrued":"0.001","start":"2026-01-01T00:00Z","end":"2026-01-02T00:00Z"},
{"run":9,"accrued":"0.002","start":"2026-01-03T00:00Z","end":"2026-01-06T00:00Z"}
]}]}]
```

Create `testdata/all-censored.json`:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":3,"first":"2026-01-01T00:00:00Z","last":"2026-06-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":0,"accrued":"0","start":"2026-05-31T16:00Z","end":"2026-05-31T16:00Z"},
{"run":0,"accrued":"0","start":"2026-05-31T20:00Z","end":"2026-05-31T20:00Z"},
{"run":0,"accrued":"0","start":"2026-06-01T00:00Z","end":"2026-06-01T00:00Z"}
]}]}]
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestRunLength -v`

Expected: FAIL to compile — `undefined: persistence.RunLength`.

- [ ] **Step 4: Write `runlength.go`**

```go
package persistence

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// RunLength is how long a persistence episode lasts, at one entry level.
//
// The coordinate rides on the value rather than being a required argument, and
// the reason is F55/F60/F83/F84/F100: the family is not "the function forgot an
// argument", it is "the value travelled without its base". A required argument
// cannot reach three hops downstream, where HANDOFF.md restates "median episode
// is 2 settlements" with no level attached — and at two of the four declared
// levels the median is 1. A coordinate on the struct can, because a screen
// renders what it holds.
type RunLength struct {
	// P50, P75, P90 are nearest-rank percentiles over the run lengths, and Max
	// is the longest. Over an empty population all four are 0.
	P50, P75, P90, Max int

	// Mean is the arithmetic mean run length, as a ratio at the canonical
	// scale. Over an empty population it is a CONSTRUCTED zero, never the zero
	// value: "no episodes" and "nobody computed this" are different facts.
	Mean fixedpoint.Rate

	// Episodes is the size of the run-length population — censored records
	// excluded.
	Episodes int

	// Censored is how many entries triggered with no settlement after them.
	//
	// F103: five such records exist in the committed file, all at level 0.0002,
	// all with start == end, all accruing "0", all at the tape's final decision
	// point. summarise.py counts them in `episodes` and averages them in, which
	// moves the published run_mean from 2.434 to 2.433. They are right-censored
	// observations, not runs of length zero, so they are reported here rather
	// than admitted to the population — and reported rather than dropped
	// silently, because a shrinking denominator nobody records is F60's defect.
	Censored int

	// At is the coordinate this answer was computed at.
	At EntryAt
}

// RunLength folds the file's episodes at one entry level.
func (s Set) RunLength(at EntryAt) (RunLength, error) {
	rows, err := s.at(at)
	if err != nil {
		return RunLength{}, err
	}

	var runs []int
	censored := 0
	total := 0
	for _, r := range rows {
		censored += len(r.censored)
		for _, e := range r.episodes {
			runs = append(runs, e.Run)
			total += e.Run
		}
	}
	sort.Ints(runs)

	out := RunLength{Episodes: len(runs), Censored: censored, At: at, Mean: fixedpoint.ZeroRate()}
	if len(runs) == 0 {
		return out, nil
	}
	out.P50 = percentile(runs, 50)
	out.P75 = percentile(runs, 75)
	out.P90 = percentile(runs, 90)
	out.Max = runs[len(runs)-1]

	sum, err := fixedpoint.ParseRate(strconv.Itoa(total))
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: run total %d: %w", total, err)
	}
	n, err := fixedpoint.ParseRate(strconv.Itoa(len(runs)))
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: run count %d: %w", len(runs), err)
	}
	mean, err := sum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return RunLength{}, fmt.Errorf("persistence: mean run length: %w", err)
	}
	out.Mean = mean
	return out, nil
}

// percentile is summarise.py:38's nearest-rank rule, replicated exactly:
// sorted[min(len-1, (len*q)//100)]. It is NOT a midpoint or interpolated
// percentile, and the difference is visible at small populations — for two
// elements the 50th percentile is the SECOND one.
//
// sorted must already be ascending and non-empty.
func percentile(sorted []int, q int) int {
	i := (len(sorted) * q) / 100
	if i > len(sorted)-1 {
		i = len(sorted) - 1
	}
	return sorted[i]
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run 'TestLoad|TestRunLength' -v`

Expected: PASS, including Task 2's previously-uncompilable subtests.

- [ ] **Step 6: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): decoder and run-length fold

Decodes both timestamp layouts the emitter writes (F102) and classifies the
five zero-settlement records as right-censored rather than as runs of length
zero (F103), reporting the count beside the percentiles."
```

---

## Task 4: Breakeven — run length is not the break-even question

**Files:**
- Create: `internal/strategy/infrastructure/persistence/breakeven.go`
- Test: `internal/strategy/infrastructure/persistence/breakeven_test.go`

**Interfaces:**
- Consumes: Task 2's `Set.at`; Task 1's `CostAt`.
- Produces:
  - `type BreakevenRow struct { Run int; Plus bool; Episodes, ClearedCost int }`
  - `type BreakevenTable struct { Rows []BreakevenRow; At CostAt }`
  - `func (s Set) Breakeven(at CostAt) (BreakevenTable, error)`

**Measured expectations** at the coordinate summarise.py pins (`entry 0.0005`, `cost 0.0021`), which is the block whose pin F55 names — the first three rows are `{run 1, 1648 episodes, 0 cleared}`, `{run 2, 746, 19}`, `{run 3, 341, 51}`. The last row is the `12+` bucket (`Plus: true`).

- [ ] **Step 1: Write the failing test**

```go
package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func costAt(t *testing.T, entry, cost string) persistence.CostAt {
	t.Helper()
	return persistence.CostAt{
		EntryAt: persistence.EntryAt{Entry: mustRate(t, entry)},
		Cost:    mustRate(t, cost),
	}
}

// The committed file's run_vs_breakeven block is computed at entry 0.0005 and
// cost 0.0021, and records NEITHER — that pin is F55. Here it is a coordinate
// the caller names and the answer carries.
func TestBreakeven_AtTheArtifactsPin(t *testing.T) {
	set := loadCommitted(t)
	got, err := set.Breakeven(costAt(t, "0.0005", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	if len(got.Rows) == 0 {
		t.Fatal("Breakeven returned no rows")
	}
	want := []persistence.BreakevenRow{
		{Run: 1, Episodes: 1648, ClearedCost: 0},
		{Run: 2, Episodes: 746, ClearedCost: 19},
		{Run: 3, Episodes: 341, ClearedCost: 51},
	}
	for i, w := range want {
		if got.Rows[i] != w {
			t.Errorf("Rows[%d] = %+v, want %+v", i, got.Rows[i], w)
		}
	}
	last := got.Rows[len(got.Rows)-1]
	if !last.Plus || last.Run != 12 {
		t.Errorf("last row = %+v, want the 12+ bucket", last)
	}
	for _, r := range got.Rows[:len(got.Rows)-1] {
		if r.Plus {
			t.Errorf("row %+v is marked Plus but is not the last bucket", r)
		}
	}
	if got.At.Entry.Decimal() != "0.0005" || got.At.Cost.Decimal() != "0.0021" {
		t.Errorf("At = %s/%s, want 0.0005/0.0021", got.At.Entry.Decimal(), got.At.Cost.Decimal())
	}
}

// The table is a function of BOTH coordinates. A table asserted at one cost
// cannot tell whether the cost argument is used at all — the cleared_cost
// column is the one that moves.
func TestBreakeven_ClearedCountMovesWithTheCost(t *testing.T) {
	set := loadCommitted(t)
	cheap, err := set.Breakeven(costAt(t, "0.0005", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven cheap: %v", err)
	}
	dear, err := set.Breakeven(costAt(t, "0.0005", "0.0090"))
	if err != nil {
		t.Fatalf("Breakeven dear: %v", err)
	}
	cheapTotal, dearTotal := 0, 0
	for i := range cheap.Rows {
		cheapTotal += cheap.Rows[i].ClearedCost
		dearTotal += dear.Rows[i].ClearedCost
		if cheap.Rows[i].Episodes != dear.Rows[i].Episodes {
			t.Errorf("row %d: episode count moved with the cost (%d vs %d); it must not",
				i, cheap.Rows[i].Episodes, dear.Rows[i].Episodes)
		}
	}
	if cheapTotal != 586 || dearTotal != 125 {
		t.Fatalf("cleared totals = %d/%d, want 586/125", cheapTotal, dearTotal)
	}
}

// Bucketing caps at 12, so runs of 12, 20 and 36 all land in one row. The
// fixture must contain a run ABOVE 12 and a run AT 12, or "cap at 12" and "cap
// at 13" are the same table.
func TestBreakeven_BucketsCapAtTwelve(t *testing.T) {
	set := loadFixture(t, "testdata/buckets.json")
	got, err := set.Breakeven(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	last := got.Rows[len(got.Rows)-1]
	if last.Run != 12 || !last.Plus || last.Episodes != 3 {
		t.Fatalf("12+ bucket = %+v, want run 12, Plus, 3 episodes", last)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 (run 11 and the 12+ bucket)", len(got.Rows))
	}
}
```

Create `testdata/buckets.json` — one run of 11, and runs of 12, 13 and 30 that must collapse into the `12+` bucket:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":4,"first":"2026-01-01T00:00:00Z","last":"2026-06-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":11,"accrued":"0.001","start":"2026-01-01T00:00Z","end":"2026-01-04T16:00Z"},
{"run":12,"accrued":"0.001","start":"2026-01-05T00:00Z","end":"2026-01-09T00:00Z"},
{"run":13,"accrued":"0.003","start":"2026-01-10T00:00Z","end":"2026-01-14T08:00Z"},
{"run":30,"accrued":"0.004","start":"2026-02-01T00:00Z","end":"2026-02-11T00:00Z"}
]}]}]
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestBreakeven -v`

Expected: FAIL to compile — `undefined: persistence.BreakevenRow`.

- [ ] **Step 3: Write `breakeven.go`**

```go
package persistence

import (
	"fmt"
	"sort"
)

// breakevenCap is the top bucket. Runs at or above it collapse into one row,
// replicating summarise.py:82's min(e["run"], 12).
const breakevenCap = 12

// BreakevenRow is one run-length bucket: how many episodes ran that long, and
// how many of them cleared the round-trip cost.
type BreakevenRow struct {
	// Run is the bucket's run length; Plus marks the open-ended top bucket,
	// which the artifact renders as "12+".
	Run  int
	Plus bool

	// Episodes is how many episodes fell in the bucket, and ClearedCost how
	// many of those accrued STRICTLY more than the cost.
	Episodes    int
	ClearedCost int
}

// BreakevenTable answers record §99's "run length is not the break-even
// question": a long run is not the same thing as a profitable one.
//
// summary.json publishes exactly one of these, at entry level 0.0005 and cost
// 0.0021, and records neither — F55. Here both are named by the caller and
// carried on the answer.
type BreakevenTable struct {
	Rows []BreakevenRow
	At   CostAt
}

// Breakeven buckets the episodes at one entry level by run length and counts,
// per bucket, how many cleared the named cost.
func (s Set) Breakeven(at CostAt) (BreakevenTable, error) {
	if err := at.Validate(); err != nil {
		return BreakevenTable{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return BreakevenTable{}, err
	}

	episodes := map[int]int{}
	cleared := map[int]int{}
	for _, r := range rows {
		for _, e := range r.episodes {
			b := e.Run
			if b > breakevenCap {
				b = breakevenCap
			}
			episodes[b]++
			cmp, err := e.Accrued.Cmp(at.Cost)
			if err != nil {
				return BreakevenTable{}, fmt.Errorf("persistence: compare accrued to cost: %w", err)
			}
			// Strictly greater. summarise.py:86 writes `>`, and the committed
			// data cannot separate `>` from `>=` — measured: exactly zero of
			// 17 478 episodes accrue any of the three costs. The hand-built
			// fixture in net_test.go is what makes the choice checkable.
			if cmp > 0 {
				cleared[b]++
			}
		}
	}

	buckets := make([]int, 0, len(episodes))
	for b := range episodes {
		buckets = append(buckets, b)
	}
	sort.Ints(buckets)

	table := BreakevenTable{At: at, Rows: make([]BreakevenRow, 0, len(buckets))}
	for _, b := range buckets {
		table.Rows = append(table.Rows, BreakevenRow{
			Run: b, Plus: b == breakevenCap,
			Episodes: episodes[b], ClearedCost: cleared[b],
		})
	}
	return table, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestBreakeven -v`

Expected: PASS, all three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): break-even table at a caller-named coordinate

The artifact publishes one of these, pinned to entry 0.0005 and cost 0.0021 and
recording neither. Both now ride on the answer."
```

---

## Task 5: NetPerEpisode, and the `>` vs `>=` fixture

**Files:**
- Create: `internal/strategy/infrastructure/persistence/net.go`
- Test: `internal/strategy/infrastructure/persistence/net_test.go`

**Interfaces:**
- Consumes: Task 2's `Set.at`; Task 1's `CostAt`.
- Produces:
  - `type NetPerEpisode struct { Episodes, Wins int; MeanGross, MeanNet fixedpoint.Rate; At CostAt }`
  - `func (s Set) NetPerEpisode(at CostAt) (NetPerEpisode, error)`

**Measured expectations, censored excluded** (the 0.0002 row therefore differs from `summary.json`; see Task 8):

| Level | Episodes | MeanGross | Cost | Wins | MeanNet |
|---|---|---|---|---|---|
| 0.0002 | 12317 | 0.000653608 | 0.0021 | 872 | -0.001446392 |
| 0.0005 | 3425 | 0.001518849 | 0.0021 | 586 | -0.000581151 |
| 0.001 | 1259 | 0.002613091 | 0.0021 | 343 | 0.000513091 |
| 0.002 | 472 | 0.003677870 | 0.0021 | 168 | 0.001577870 |

`Rate.Decimal()` trims trailing zeros, so `0.003677870` renders as `0.00367787`. Assert against `Decimal()` output, or compare mantissas — do not hand-write a trailing zero.

- [ ] **Step 1: Write the failing test**

```go
package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func TestNetPerEpisode_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		entry, cost      string
		episodes, wins   int
		meanGross, meanNet string
	}{
		{"0.0002", "0.0021", 12317, 872, "0.000653608", "-0.001446392"},
		{"0.0005", "0.0021", 3425, 586, "0.001518849", "-0.000581151"},
		{"0.001", "0.0021", 1259, 343, "0.002613091", "0.000513091"},
		{"0.002", "0.0021", 472, 168, "0.00367787", "0.00157787"},
	}
	for _, c := range cases {
		t.Run(c.entry, func(t *testing.T) {
			got, err := set.NetPerEpisode(costAt(t, c.entry, c.cost))
			if err != nil {
				t.Fatalf("NetPerEpisode: %v", err)
			}
			if got.Episodes != c.episodes {
				t.Errorf("Episodes = %d, want %d", got.Episodes, c.episodes)
			}
			if got.Wins != c.wins {
				t.Errorf("Wins = %d, want %d", got.Wins, c.wins)
			}
			if got.MeanGross.Decimal() != c.meanGross {
				t.Errorf("MeanGross = %s, want %s", got.MeanGross.Decimal(), c.meanGross)
			}
			if got.MeanNet.Decimal() != c.meanNet {
				t.Errorf("MeanNet = %s, want %s", got.MeanNet.Decimal(), c.meanNet)
			}
		})
	}
}

// THE SIXTEENTH COLLAPSE, from record §103 §6.2.
//
// Wins counts accrued > cost. Measured over all 17 478 committed episodes,
// EXACTLY ZERO accrue any of the three costs, so `>` and `>=` return the same
// three counts (1 969 / 1 485 / 430 pooled across levels). The committed data
// cannot separate the two comparisons, and the oracle test in Task 8 passes
// against both. Only a hand-built episode accruing EXACTLY the cost can tell
// them apart, and this is it.
func TestNetPerEpisode_TheBoundaryIsStrict(t *testing.T) {
	// Three episodes: one just under the cost, one exactly at it, one just over.
	// Strictly-greater gives Wins = 1. Greater-or-equal would give 2.
	set := loadFixture(t, "testdata/cost-boundary.json")
	got, err := set.NetPerEpisode(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("NetPerEpisode: %v", err)
	}
	if got.Episodes != 3 {
		t.Fatalf("Episodes = %d, want 3", got.Episodes)
	}
	if got.Wins != 1 {
		t.Fatalf("Wins = %d, want 1 — an episode accruing exactly the cost does NOT clear it", got.Wins)
	}
}

// The same fixture separates the comparison in the break-even table, which uses
// its own copy of the rule. A fix applied in one place gets applied to its
// mirror, and a claim checked in one place gets checked in its mirror.
func TestBreakeven_TheBoundaryIsStrict(t *testing.T) {
	set := loadFixture(t, "testdata/cost-boundary.json")
	got, err := set.Breakeven(costAt(t, "0.001", "0.0021"))
	if err != nil {
		t.Fatalf("Breakeven: %v", err)
	}
	total := 0
	for _, r := range got.Rows {
		total += r.ClearedCost
	}
	if total != 1 {
		t.Fatalf("cleared total = %d, want 1", total)
	}
}
```

Create `testdata/cost-boundary.json`:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":3,"first":"2026-01-01T00:00:00Z","last":"2026-06-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":2,"accrued":"0.00209999","start":"2026-01-01T00:00Z","end":"2026-01-01T16:00Z"},
{"run":2,"accrued":"0.0021","start":"2026-01-02T00:00Z","end":"2026-01-02T16:00Z"},
{"run":2,"accrued":"0.00210001","start":"2026-01-03T00:00Z","end":"2026-01-03T16:00Z"}
]}]}]
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run 'TestNetPerEpisode|TestBreakeven_TheBoundary' -v`

Expected: FAIL to compile — `undefined: persistence.NetPerEpisode`.

- [ ] **Step 3: Write `net.go`**

```go
package persistence

import (
	"fmt"
	"strconv"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// NetPerEpisode is the pooled per-episode economics at one (level, cost).
//
// Both figures are RATIOS, not percentages and not currency: this is what one
// episode accrues per unit of leg notional. Turning that into money needs a
// notional, which is why PerSymbol takes a SizeAt and this does not.
type NetPerEpisode struct {
	// Episodes is the population size — censored records excluded.
	Episodes int

	// Wins counts episodes whose accrual was STRICTLY greater than the cost.
	Wins int

	// MeanGross is the mean accrual before cost; MeanNet subtracts one
	// round-trip cost per episode.
	MeanGross fixedpoint.Rate
	MeanNet   fixedpoint.Rate

	// At is the coordinate this answer was computed at.
	At CostAt
}

// NetPerEpisode folds the accruals at one entry level against one cost.
func (s Set) NetPerEpisode(at CostAt) (NetPerEpisode, error) {
	if err := at.Validate(); err != nil {
		return NetPerEpisode{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return NetPerEpisode{}, err
	}

	sum := fixedpoint.ZeroRate()
	count, wins := 0, 0
	for _, r := range rows {
		for _, e := range r.episodes {
			count++
			sum, err = sum.Add(e.Accrued)
			if err != nil {
				return NetPerEpisode{}, fmt.Errorf("persistence: accumulate accrual: %w", err)
			}
			cmp, err := e.Accrued.Cmp(at.Cost)
			if err != nil {
				return NetPerEpisode{}, fmt.Errorf("persistence: compare accrued to cost: %w", err)
			}
			if cmp > 0 {
				wins++
			}
		}
	}

	out := NetPerEpisode{
		Episodes: count, Wins: wins, At: at,
		MeanGross: fixedpoint.ZeroRate(), MeanNet: fixedpoint.ZeroRate(),
	}
	if count == 0 {
		return out, nil
	}

	n, err := fixedpoint.ParseRate(strconv.Itoa(count))
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: episode count %d: %w", count, err)
	}
	gross, err := sum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: mean gross: %w", err)
	}
	// Total cost is one round trip per episode, subtracted before the mean —
	// the same order summarise.py:53 uses, so the two implementations round at
	// the same point rather than agreeing by luck.
	totalCost, err := at.Cost.Mul(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: total cost: %w", err)
	}
	netSum, err := sum.Sub(totalCost)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: net total: %w", err)
	}
	net, err := netSum.Div(n, fixedpoint.RoundHalfEven)
	if err != nil {
		return NetPerEpisode{}, fmt.Errorf("persistence: mean net: %w", err)
	}
	out.MeanGross, out.MeanNet = gross, net
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run 'TestNetPerEpisode|TestBreakeven' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): pooled per-episode net, with a strict cost boundary

The committed file cannot separate > from >= (zero of 17478 episodes accrue any
of the three costs), so the boundary gets a hand-built fixture."
```

---

## Task 6: PerSymbol — the denominator moves, and the reader says so

**Files:**
- Create: `internal/strategy/infrastructure/persistence/persymbol.go`
- Test: `internal/strategy/infrastructure/persistence/persymbol_test.go`
- Create: `testdata/degenerate-span.json`

**Interfaces:**
- Consumes: Task 2's `Set.at`, `symbolRecord`; Task 1's `SizeAt`, `ErrSpanDegenerate`.
- Produces:
  - `type SymbolReturn struct { Symbol string; Shortlist bool; Episodes, Days, ModalMinutes int; First, Last time.Time; Net fixedpoint.Money; Annual fixedpoint.Rate }`
  - `type SymbolReturns struct { Rows []SymbolReturn; Excluded []string; At SizeAt }`
  - `func (s Set) PerSymbol(at SizeAt) (SymbolReturns, error)`

**Measured expectations** at `entry 0.001`, `cost 0.0021`, `leg 1000`, `capital 2000`:
- 56 rows, 5 excluded: `CAPUSDT`, `CFXUSDT`, `WLDUSDT`, `XAUTUSDT`, `XPLUSDT` (sorted).
- Shrinkage across the four levels: **0 / 1 / 5 / 12** excluded, i.e. **61 / 60 / 56 / 49** rows. This is F60: `verdict.total_symbols` reads 56 while the file's own header declares `symbols_eligible: 61`, and no field records that the denominator is a function of the entry level.
- `LABUSDT` at cost `0.0021`: `Days` 277, `Net` 543.88, `Annual` 0.358 as a ratio (the file's `annual_pct_on_2000` is `"35.8"`).
- All 168 (56 symbols × 3 costs) `Net` and `Annual` cells reproduce the file exactly under scale-9 intermediates with `RoundHalfEven` — verified before this plan was written.

- [ ] **Step 1: Write the failing test**

```go
package persistence_test

import (
	"errors"
	"testing"

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
	if lab.Net.Decimal() != "543.88" {
		t.Errorf("LABUSDT Net = %s, want 543.88", lab.Net.Decimal())
	}
	// The annual return is a RATIO. 35.8% is 0.358. rate.go:8 is explicit that
	// nothing in fixedpoint multiplies by 100, so the percentage the artifact
	// publishes is a rendering choice, not a stored value.
	if lab.Annual.Decimal() != "0.357983221" {
		t.Errorf("LABUSDT Annual = %s, want 0.357983221", lab.Annual.Decimal())
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
```

Create `testdata/degenerate-span.json` — span of 18 hours:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":3,"first":"2026-01-01T00:00:00Z","last":"2026-01-01T18:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":2,"accrued":"0.003","start":"2026-01-01T00:00Z","end":"2026-01-01T16:00Z"}
]}]}]
```

Create `testdata/one-day-span.json` — span of exactly 24 hours:

```json
[{"symbol":"FIXTURE","shortlist":false,"matched":3,"first":"2026-01-01T00:00:00Z","last":"2026-01-02T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":2,"accrued":"0.003","start":"2026-01-01T00:00Z","end":"2026-01-01T16:00Z"}
]}]}]
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestPerSymbol -v`

Expected: FAIL to compile — `undefined: persistence.SymbolReturn`.

- [ ] **Step 3: Write `persymbol.go`**

```go
package persistence

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// daysPerYear is the annualisation factor summarise.py:111 uses. It is a
// calendar convention, not a claim about any venue.
const daysPerYear = 365

// SymbolReturn is one symbol's realised economics at one coordinate.
type SymbolReturn struct {
	Symbol       string
	Shortlist    bool
	Episodes     int
	Days         int
	ModalMinutes int
	First        time.Time
	Last         time.Time

	// Net is the total after cost, in the leg notional's asset, for a position
	// of one LegNotional per leg.
	Net fixedpoint.Money

	// Annual is Net over Capital, annualised across Days — as a RATIO. The
	// artifact's annual_pct_on_2000 is this times 100.
	Annual fixedpoint.Rate
}

// SymbolReturns is the per-symbol table, and Excluded is why it is shorter than
// the file's symbol count.
type SymbolReturns struct {
	// Rows are sorted by Annual descending, ties broken by Symbol ascending.
	//
	// The artifact sorts by the ROUNDED percentage at cost 0.0021 for every
	// block; this sorts by the unrounded ratio at the coordinate asked for.
	// Tests compare by symbol name, never by index, because those two orders
	// are not the same claim.
	Rows []SymbolReturn

	// Excluded names the symbols with no episodes at this entry level, sorted.
	//
	// summarise.py:99 writes `if not e: continue` and records nothing. Measured
	// over the committed file the excluded counts are 0 / 1 / 5 / 12 across the
	// four declared levels, which is why verdict.total_symbols reads 56 against
	// a header declaring symbols_eligible 61 — F60. The denominator is a
	// function of the entry level, and no field in the artifact says so. This
	// one does.
	Excluded []string

	At SizeAt
}

// PerSymbol computes each symbol's net and annualised return at one coordinate.
func (s Set) PerSymbol(at SizeAt) (SymbolReturns, error) {
	if err := at.Validate(); err != nil {
		return SymbolReturns{}, err
	}
	rows, err := s.at(at.EntryAt)
	if err != nil {
		return SymbolReturns{}, err
	}

	out := SymbolReturns{At: at}
	for _, r := range rows {
		if len(r.episodes) == 0 {
			out.Excluded = append(out.Excluded, r.symbol.name)
			continue
		}
		row, err := symbolRow(r, at)
		if err != nil {
			return SymbolReturns{}, err
		}
		out.Rows = append(out.Rows, row)
	}

	sort.Strings(out.Excluded)
	sort.Slice(out.Rows, func(i, j int) bool {
		cmp, err := out.Rows[i].Annual.Cmp(out.Rows[j].Annual)
		if err != nil || cmp == 0 {
			return out.Rows[i].Symbol < out.Rows[j].Symbol
		}
		return cmp > 0
	})
	return out, nil
}

func symbolRow(r symbolLevel, at SizeAt) (SymbolReturn, error) {
	days := int(r.symbol.last.Sub(r.symbol.first).Hours() / 24)
	if days < 1 {
		// summarise.py:33 writes max(1, days) here, which inflates an
		// annualised return over a day that did not happen and fails nothing.
		return SymbolReturn{}, fmt.Errorf("%w: %s spans %s to %s",
			ErrSpanDegenerate, r.symbol.name,
			r.symbol.first.Format(time.RFC3339), r.symbol.last.Format(time.RFC3339))
	}

	total, err := fixedpoint.ParseMoney(at.LegNotional.Asset(), "0")
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: zero total: %w", err)
	}
	costPerEpisode, err := at.LegNotional.Mul(at.Cost, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: cost per episode: %w", err)
	}
	for _, e := range r.episodes {
		gross, err := at.LegNotional.Mul(e.Accrued, fixedpoint.RoundHalfEven)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s gross: %w", r.symbol.name, err)
		}
		net, err := gross.Sub(costPerEpisode)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s net: %w", r.symbol.name, err)
		}
		total, err = total.Add(net)
		if err != nil {
			return SymbolReturn{}, fmt.Errorf("persistence: %s total: %w", r.symbol.name, err)
		}
	}

	ratio, err := total.Div(at.Capital, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s return on capital: %w", r.symbol.name, err)
	}
	year, err := fixedpoint.ParseRate(strconv.Itoa(daysPerYear))
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: days per year: %w", err)
	}
	scaled, err := ratio.Mul(year, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s annualise: %w", r.symbol.name, err)
	}
	span, err := fixedpoint.ParseRate(strconv.Itoa(days))
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s span %d: %w", r.symbol.name, days, err)
	}
	annual, err := scaled.Div(span, fixedpoint.RoundHalfEven)
	if err != nil {
		return SymbolReturn{}, fmt.Errorf("persistence: %s annual: %w", r.symbol.name, err)
	}

	return SymbolReturn{
		Symbol: r.symbol.name, Shortlist: r.symbol.shortlist,
		Episodes: len(r.episodes), Days: days, ModalMinutes: r.symbol.modalMinutes,
		First: r.symbol.first, Last: r.symbol.last,
		Net: total, Annual: annual,
	}, nil
}
```

**One check the implementer must make in Step 4:** `int(d.Hours() / 24)` divides two `float64` values, and the `nofloat` analyzer denies `float64` under `internal/`. Replace it with integer arithmetic before running the gate:

```go
	days := int(r.symbol.last.Sub(r.symbol.first) / (24 * time.Hour))
```

`time.Duration` is an `int64`, so this truncates toward zero exactly as Python's `timedelta.days` does for non-negative spans. Use this form. If `last` precedes `first` the result is negative and the `days < 1` guard refuses it, which is the right answer for a malformed span.

- [ ] **Step 4: Run the tests and the gate to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestPerSymbol -v`
Expected: PASS, all four tests.

Run: `go run ./tools/lint/... ./internal/strategy/infrastructure/persistence/...` — or whatever `make ci`'s custom-gate target invokes; check the `Makefile` for the exact target name and use it.
Expected: no `nofloat` diagnostic. If one appears, the `Hours()/24` form survived.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): per-symbol returns, with the shrinking denominator named

Excluded reports the symbols a level drops (0/1/5/12 across the four declared
levels), which is why total_symbols reads 56 against a header declaring 61."
```

---

## Task 7: Verdict, and the hurdle that must be an argument

**Files:**
- Create: `internal/strategy/infrastructure/persistence/verdict.go`
- Test: `internal/strategy/infrastructure/persistence/verdict_test.go`
- Create: `testdata/hurdle-boundary.json`

**Interfaces:**
- Consumes: Task 6's `Set.PerSymbol`, `SymbolReturns`; Task 1's `SizeAt`, `Hurdle`.
- Produces:
  - `type Verdict struct { MedianAnnual fixedpoint.Rate; PositiveSymbols, BeatHurdle, TotalSymbols int; Excluded []string; Hurdle Hurdle; At SizeAt }`
  - `func (s Set) Verdict(at SizeAt, h Hurdle) (Verdict, error)`

**Measured expectations** at `entry 0.001`, leg 1000, capital 2000, hurdle ratio `0.046`:

| Cost | MedianAnnual (×100, 1 dp) | Positive | Beat hurdle | Total |
|---|---|---|---|---|
| 0.0021 | -0.1 | 16 | 4 | 56 |
| 0.0029 | -0.2 | 12 | 3 | 56 |
| 0.0090 | -1.5 | 1 | 0 | 56 |

**A seventeenth collapse, found writing this plan.** On the committed file, comparing on the unrounded ratio and comparing on the rounded percentage give identical `BeatHurdle` counts (4 / 3 / 0) and identical medians, at every cost. So the committed data cannot separate "the comparison is on the stored ratio" from "the comparison is on the displayed percentage" — a distinction record §100's display-rounding finding says has bitten this project before. `testdata/hurdle-boundary.json` is what separates them.

- [ ] **Step 1: Write the failing test**

```go
package persistence_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

func illustrativeHurdle(t *testing.T) persistence.Hurdle {
	t.Helper()
	return persistence.Hurdle{
		// 4.6% as a ratio. summarise.py prints "4.6"; rate.go stores ratios.
		Annual: mustRate(t, "0.046"),
		Class:  "illustrative_constant",
		Source: "research/persistence/summarise.py HURDLE_PCT. NOT reference data: " +
			"configs/reference.*.json carry annual_rate 0.0369 at class hand_recorded.",
	}
}

func TestVerdict_AgainstTheCommittedFile(t *testing.T) {
	set := loadCommitted(t)
	cases := []struct {
		cost                          string
		medianPct                     string // the ratio's Decimal() form
		positive, beat, total         int
	}{
		{"0.0021", "-0.001406012", 16, 4, 56},
		{"0.0029", "-0.001615702", 12, 3, 56},
		{"0.0090", "-0.014744318", 1, 0, 56},
	}
	for _, c := range cases {
		t.Run(c.cost, func(t *testing.T) {
			got, err := set.Verdict(sizeAt(t, "0.001", c.cost), illustrativeHurdle(t))
			if err != nil {
				t.Fatalf("Verdict: %v", err)
			}
			if got.PositiveSymbols != c.positive {
				t.Errorf("PositiveSymbols = %d, want %d", got.PositiveSymbols, c.positive)
			}
			if got.BeatHurdle != c.beat {
				t.Errorf("BeatHurdle = %d, want %d", got.BeatHurdle, c.beat)
			}
			if got.TotalSymbols != c.total {
				t.Errorf("TotalSymbols = %d, want %d", got.TotalSymbols, c.total)
			}
			// The hurdle rides on the verdict, with its class and its source.
			// F45: a stored comparison carries its comparand, and the count
			// beside a hurdle whose provenance is missing is what cost this
			// project a screen rendering 3.69 beside a count computed at 4.6.
			if got.Hurdle.Class != "illustrative_constant" {
				t.Errorf("Hurdle.Class = %q, want illustrative_constant", got.Hurdle.Class)
			}
			if got.Hurdle.Annual.Decimal() != "0.046" {
				t.Errorf("Hurdle.Annual = %s, want 0.046", got.Hurdle.Annual.Decimal())
			}
		})
	}
}

// An unset hurdle is refused. A Go constant here would re-commit F45 in a
// second language: summarise.py's HURDLE_PCT is illustrative, is not reference
// data, and is not a calibrated benchmark per build-spec §55.
func TestVerdict_RefusesAnUnsetHurdle(t *testing.T) {
	set := loadCommitted(t)
	_, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), persistence.Hurdle{})
	if err == nil {
		t.Fatal("Verdict with a zero Hurdle returned nil error")
	}
}

// THE SEVENTEENTH COLLAPSE. On the committed file, comparing the hurdle against
// the stored RATIO and against the DISPLAYED percentage give identical counts
// at every cost (4 / 3 / 0). So no assertion over that file can tell which rule
// the code implements — and this project has already shipped a display-rounding
// predicate once.
//
// This fixture puts a symbol's annual return at 4.63% — strictly above the
// 4.6% hurdle as a ratio, and ALSO 4.6% when rounded to one decimal, where a
// strictly-greater comparison on the rounded value would exclude it. Beat = 1
// is the ratio rule; Beat = 0 is the display rule.
func TestVerdict_ComparesTheRatioNotTheRoundedPercentage(t *testing.T) {
	set := loadFixture(t, "testdata/hurdle-boundary.json")
	got, err := set.Verdict(sizeAt(t, "0.001", "0.0021"), illustrativeHurdle(t))
	if err != nil {
		t.Fatalf("Verdict: %v", err)
	}
	if got.TotalSymbols != 1 {
		t.Fatalf("TotalSymbols = %d, want 1", got.TotalSymbols)
	}
	if got.BeatHurdle != 1 {
		t.Fatalf("BeatHurdle = %d, want 1 — the comparison is on the ratio, not on the rounded percentage", got.BeatHurdle)
	}
}
```

**Before writing the fixture, compute its numbers.** The implementer must pick `accrued`, `run`, `first` and `last` such that the resulting `Annual` lands strictly between `0.046` and `0.0465`. Do it with the same scale-9 arithmetic the code uses:

```
Annual = round9( round9( round9(total / capital) * 365 ) / days )
total  = Σ round9(leg * accrued) − round9(leg * cost)
```

With `leg = 1000`, `capital = 2000`, `cost = 0.0021`, a span of exactly 365 days and a single episode, `Annual = total / 2000`. So `total = 92.6` gives `Annual = 0.0463`. One episode with `accrued = 0.0947` gives `total = 94.7 − 2.1 = 92.6`. Verify with a throwaway Go test before committing the fixture; if the arithmetic disagrees, adjust `accrued` and update the expectation — do not adjust the assertion.

Create `testdata/hurdle-boundary.json`:

```json
[{"symbol":"FIXTURE","shortlist":true,"matched":10,"first":"2025-01-01T00:00:00Z","last":"2026-01-01T00:00:00Z","modal_minutes":480,"levels":[{"entry":"0.001","episodes":[
{"run":3,"accrued":"0.0947","start":"2025-06-01T00:00Z","end":"2025-06-02T00:00Z"}
]}]}]
```

Note that 2025-01-01 to 2026-01-01 is 365 days.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestVerdict -v`

Expected: FAIL to compile — `undefined: persistence.Verdict`.

- [ ] **Step 3: Write `verdict.go`**

```go
package persistence

import (
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Verdict is the summary answer at one coordinate, against one hurdle.
//
// It is deliberately not called "the kill-gate verdict": build-spec §12.2's
// gate is a comparison against a dated, haircut-adjusted benchmark, and the
// hurdle this takes is whatever the caller supplies. Verdict carries the hurdle
// it used, with the hurdle's own class and source, so a rendered figure and its
// comparand travel together.
type Verdict struct {
	// MedianAnnual is the middle element of the annual returns, ascending, at
	// index len/2 — replicating summarise.py's v[len(v)//2] exactly. For an
	// even population that is the UPPER of the two middle values, not their
	// average. It is a ratio.
	MedianAnnual fixedpoint.Rate

	// PositiveSymbols counts symbols whose NET is strictly positive. This is
	// deliberately the raw net rather than the rounded percentage: three
	// symbols earn a couple of dollars over four years, which is positive and
	// renders as +0.0%/yr.
	PositiveSymbols int

	// BeatHurdle counts symbols whose annual RATIO is strictly greater than the
	// hurdle's. The comparison is on the stored ratio, never on a rounded
	// percentage — and the committed file cannot tell the two apart, so
	// testdata/hurdle-boundary.json is what makes this claim checkable.
	BeatHurdle int

	// TotalSymbols is the population these counts are out of — the level's
	// surviving symbols, not the file's symbol count.
	TotalSymbols int

	// Excluded names the symbols this level dropped, carried through from
	// PerSymbol so a verdict can state its own denominator's provenance.
	Excluded []string

	Hurdle Hurdle
	At     SizeAt
}

// Verdict folds the per-symbol table into counts against a hurdle.
func (s Set) Verdict(at SizeAt, h Hurdle) (Verdict, error) {
	if err := h.Validate(); err != nil {
		return Verdict{}, err
	}
	table, err := s.PerSymbol(at)
	if err != nil {
		return Verdict{}, err
	}

	out := Verdict{
		TotalSymbols: len(table.Rows), Excluded: table.Excluded,
		Hurdle: h, At: at, MedianAnnual: fixedpoint.ZeroRate(),
	}
	if len(table.Rows) == 0 {
		return out, nil
	}

	annuals := make([]fixedpoint.Rate, 0, len(table.Rows))
	for _, r := range table.Rows {
		annuals = append(annuals, r.Annual)

		sign, err := r.Net.Sign()
		if err != nil {
			return Verdict{}, fmt.Errorf("persistence: %s net sign: %w", r.Symbol, err)
		}
		if sign > 0 {
			out.PositiveSymbols++
		}
		cmp, err := r.Annual.Cmp(h.Annual)
		if err != nil {
			return Verdict{}, fmt.Errorf("persistence: %s hurdle comparison: %w", r.Symbol, err)
		}
		if cmp > 0 {
			out.BeatHurdle++
		}
	}

	var sortErr error
	sort.Slice(annuals, func(i, j int) bool {
		cmp, err := annuals[i].Cmp(annuals[j])
		if err != nil {
			sortErr = err
			return false
		}
		return cmp < 0
	})
	if sortErr != nil {
		return Verdict{}, fmt.Errorf("persistence: sort annual returns: %w", sortErr)
	}
	out.MedianAnnual = annuals[len(annuals)/2]
	return out, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestVerdict -v`

Expected: PASS. If `TestVerdict_AgainstTheCommittedFile`'s three median values disagree, **do not adjust the assertion** until you have checked that the Go median equals the file's `median_annual_pct` when multiplied by 100 and rounded to one decimal (`-0.1`, `-0.2`, `-1.5`). If it does, the plan's 9-place literal was mistyped and the assertion should be corrected to the measured value; if it does not, that is a genuine disagreement between two implementations and is a finding to record, not a test to loosen.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/infrastructure/persistence/
git commit -m "feat(persistence): verdict counts against a caller-supplied hurdle

The hurdle is an argument carrying its own class and source. A constant here
would re-commit F45 in a second language."
```

---

## Task 8: The oracle test, and its negative control

**Files:**
- Create: `internal/strategy/infrastructure/persistence/oracle_test.go`

**Interfaces:**
- Consumes: everything from Tasks 2–7.
- Produces: nothing exported. This task adds no production code.

This is the task the whole design turns on. The Go fold is a second independent implementation of `summarise.py`; this test measures whether they agree, and a disagreement is a finding either way.

**Three disagreements are EXPECTED and asserted explicitly**, all at level `0.0002` and all F103:

| Quantity | summary.json | This reader | Why |
|---|---|---|---|
| `episodes` | 12322 | 12317 + 5 censored | the five `run: 0` records are not runs |
| `run_mean` | 2.433 | 2.434 | averaging five zeros in |
| `mean_gross` | 0.000653343 | 0.000653608 | dividing by 12322 rather than 12317 |

`wins` agrees at every level and every cost, because a censored record accrues `"0"` and never clears any cost. The percentiles agree too — 5 in 12 322 cannot move one. That is F103 scoped honestly: a defect in the counts and in what an episode *means*, not in the run-length answer.

- [ ] **Step 1: Write the oracle test**

```go
package persistence_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/persistence"
)

const committedSummary = "../../../../research/persistence/summary.json"

// summaryFile is summary.json's shape, decoded for use as an ORACLE only.
// Nothing in the package's production code decodes this file; record §103 §3.1
// records why, and the short form is that three of its four blocks are computed
// at a pinned coordinate they do not write down.
type summaryFile struct {
	Costs  []string `json:"costs"`
	Levels []string `json:"levels"`
	Pooled []struct {
		Entry     string            `json:"entry"`
		Episodes  int               `json:"episodes"`
		RunMean   string            `json:"run_mean"`
		RunP50    int               `json:"run_p50"`
		RunP75    int               `json:"run_p75"`
		RunP90    int               `json:"run_p90"`
		RunMax    int               `json:"run_max"`
		MeanGross string            `json:"mean_gross"`
		Wins      map[string]int    `json:"wins"`
		MeanNet   map[string]string `json:"mean_net"`
	} `json:"pooled"`
}

func loadSummary(t *testing.T) summaryFile {
	t.Helper()
	data, err := os.ReadFile(committedSummary)
	if err != nil {
		t.Fatalf("read %s: %v", committedSummary, err)
	}
	var s summaryFile
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("decode %s: %v", committedSummary, err)
	}
	return s
}

// roundedTo9 renders a Python Decimal string at the canonical scale, half-even.
//
// summary.json publishes values at up to 28 decimal places — mean_gross at
// level 0.0002 is 0.0006533427008602499594221717254, straight out of Python's
// Decimal. fixedpoint.Scale is 9. So this comparison is "agrees when the Python
// value is rounded to 9 places under the declared mode", and NOT byte equality.
// An assertion that passes must not imply one was available.
func roundedTo9(t *testing.T, s string) string {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("not a decimal: %q", s)
	}
	// big.Rat.FloatString rounds half away from zero, not half-even. The
	// artifact's values are irrational-looking quotients and none of them lands
	// on a tie at the ninth place — verified when this test was written — so the
	// two modes agree here. If a future artifact hits a tie this helper will
	// disagree with fixedpoint, and that disagreement is the signal, not noise.
	return trimZeros(r.FloatString(fixedpoint.Scale))
}

func trimZeros(s string) string {
	if !containsDot(s) {
		return s
	}
	i := len(s)
	for i > 0 && s[i-1] == '0' {
		i--
	}
	if i > 0 && s[i-1] == '.' {
		i--
	}
	return s[:i]
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

// The oracle. The Go fold and summarise.py are two independent implementations
// of the same arithmetic; this measures whether they agree on the committed
// data. Where they do not, the disagreement is NAMED and its magnitude is
// asserted — a tolerance would turn a finding into noise.
func TestOracle_PooledAgreesWithSummariseDotPy(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	if len(sum.Pooled) != 4 {
		t.Fatalf("summary.json has %d pooled rows, want 4", len(sum.Pooled))
	}

	for _, row := range sum.Pooled {
		t.Run(row.Entry, func(t *testing.T) {
			at := persistence.EntryAt{Entry: mustRate(t, row.Entry)}
			rl, err := set.RunLength(at)
			if err != nil {
				t.Fatalf("RunLength: %v", err)
			}

			// Percentiles and max agree EXACTLY at every level, censored
			// records included or excluded — five in 12 322 cannot move one.
			if rl.P50 != row.RunP50 || rl.P75 != row.RunP75 ||
				rl.P90 != row.RunP90 || rl.Max != row.RunMax {
				t.Errorf("percentiles %d/%d/%d max %d, artifact %d/%d/%d max %d",
					rl.P50, rl.P75, rl.P90, rl.Max,
					row.RunP50, row.RunP75, row.RunP90, row.RunMax)
			}

			// F103's first magnitude: the artifact's episode count includes the
			// censored records; this reader's does not, and the two reconcile.
			if rl.Episodes+rl.Censored != row.Episodes {
				t.Errorf("episodes %d + censored %d != artifact %d",
					rl.Episodes, rl.Censored, row.Episodes)
			}

			for _, cost := range sum.Costs {
				npe, err := set.NetPerEpisode(persistence.CostAt{EntryAt: at, Cost: mustRate(t, cost)})
				if err != nil {
					t.Fatalf("NetPerEpisode at %s: %v", cost, err)
				}
				// Wins agrees exactly at every level and every cost: a censored
				// record accrues "0" and clears nothing.
				if npe.Wins != row.Wins[cost] {
					t.Errorf("cost %s: wins %d, artifact %d", cost, npe.Wins, row.Wins[cost])
				}
				if rl.Censored == 0 && npe.MeanNet.Decimal() != roundedTo9(t, row.MeanNet[cost]) {
					t.Errorf("cost %s: mean_net %s, artifact (to 9) %s",
						cost, npe.MeanNet.Decimal(), roundedTo9(t, row.MeanNet[cost]))
				}
			}

			npe, err := set.NetPerEpisode(persistence.CostAt{EntryAt: at, Cost: mustRate(t, sum.Costs[0])})
			if err != nil {
				t.Fatalf("NetPerEpisode: %v", err)
			}
			if rl.Censored == 0 {
				if npe.MeanGross.Decimal() != roundedTo9(t, row.MeanGross) {
					t.Errorf("mean_gross %s, artifact (to 9) %s",
						npe.MeanGross.Decimal(), roundedTo9(t, row.MeanGross))
				}
			}
		})
	}
}

// F103's magnitude, asserted rather than tolerated. At level 0.0002 the two
// implementations DISAGREE, in three places and by exactly this much, because
// summarise.py counts five right-censored records as episodes of length zero.
// Recording the delta is what stops a later reader from either "fixing" the Go
// fold or widening the oracle until the finding disappears.
func TestOracle_CensoringIsTheOnlyDisagreement(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	var row = struct {
		episodes  int
		runMean   string
		meanGross string
	}{}
	for _, p := range sum.Pooled {
		if p.Entry == "0.0002" {
			row.episodes, row.runMean, row.meanGross = p.Episodes, p.RunMean, p.MeanGross
		}
	}
	if row.episodes != 12322 {
		t.Fatalf("artifact episodes at 0.0002 = %d, want 12322", row.episodes)
	}

	rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, "0.0002")})
	if err != nil {
		t.Fatalf("RunLength: %v", err)
	}
	if rl.Episodes != 12317 || rl.Censored != 5 {
		t.Fatalf("episodes/censored = %d/%d, want 12317/5", rl.Episodes, rl.Censored)
	}
	// The artifact publishes run_mean 2.433 over 12 322; this reader reads
	// 2.434 over 12 317. Both are correct answers to different questions.
	if rl.Mean.Decimal() != "2.433628318" {
		t.Errorf("mean = %s, want 2.433628318 (which renders as 2.434 at three places)", rl.Mean.Decimal())
	}
	if row.runMean != "2.433" {
		t.Errorf("artifact run_mean = %s, want 2.433", row.runMean)
	}

	npe, err := set.NetPerEpisode(persistence.CostAt{
		EntryAt: persistence.EntryAt{Entry: mustRate(t, "0.0002")},
		Cost:    mustRate(t, "0.0021"),
	})
	if err != nil {
		t.Fatalf("NetPerEpisode: %v", err)
	}
	if npe.MeanGross.Decimal() != "0.000653608" {
		t.Errorf("mean_gross = %s, want 0.000653608", npe.MeanGross.Decimal())
	}
	if roundedTo9(t, row.meanGross) != "0.000653343" {
		t.Errorf("artifact mean_gross (to 9) = %s, want 0.000653343", roundedTo9(t, row.meanGross))
	}
}

// THE NEGATIVE CONTROL, required by record §103 §6.1 and by
// research/persistence/README.md's control C: "an agreement that was never
// shown capable of failing is the repo's standing trap."
//
// The same comparison at the WRONG level must fail. If it passes, the oracle is
// comparing something that does not depend on the level argument at all.
func TestOracle_NegativeControl_WrongLevelDisagrees(t *testing.T) {
	set := loadCommitted(t)
	sum := loadSummary(t)

	byEntry := map[string]int{}
	for _, p := range sum.Pooled {
		byEntry[p.Entry] = p.Episodes
	}
	// Every artifact row's episode count, against every OTHER level's fold.
	// Episodes is the separating member: 12322/3425/1259/472 are four distinct
	// values, where p75 is 3 at all four levels and separates nothing.
	for _, mine := range sum.Levels {
		rl, err := set.RunLength(persistence.EntryAt{Entry: mustRate(t, mine)})
		if err != nil {
			t.Fatalf("RunLength at %s: %v", mine, err)
		}
		got := rl.Episodes + rl.Censored
		for _, theirs := range sum.Levels {
			if theirs == mine {
				continue
			}
			if got == byEntry[theirs] {
				t.Errorf("fold at %s (%d episodes) matches the artifact row for %s; "+
					"the oracle cannot detect a level mix-up", mine, got, theirs)
			}
		}
		if got != byEntry[mine] {
			t.Errorf("fold at %s = %d, artifact = %d", mine, got, byEntry[mine])
		}
	}
}

// The per-symbol block, all 168 cells (56 symbols × 3 costs). The artifact
// publishes these at level 0.001 and records neither that level nor the leg
// notional that scales the numerator; this test names both.
func TestOracle_PerSymbolAgreesWithSummariseDotPy(t *testing.T) {
	data, err := os.ReadFile(committedSummary)
	if err != nil {
		t.Fatalf("read %s: %v", committedSummary, err)
	}
	var file struct {
		Costs     []string `json:"costs"`
		PerSymbol []struct {
			Symbol   string            `json:"symbol"`
			Episodes int               `json:"episodes"`
			Days     int               `json:"days"`
			Net      map[string]string `json:"net_usd_1000_per_leg"`
			Annual   map[string]string `json:"annual_pct_on_2000"`
		} `json:"per_symbol"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(file.PerSymbol) != 56 {
		t.Fatalf("artifact per_symbol has %d rows, want 56", len(file.PerSymbol))
	}

	set := loadCommitted(t)
	for _, cost := range file.Costs {
		got, err := set.PerSymbol(sizeAt(t, "0.001", cost))
		if err != nil {
			t.Fatalf("PerSymbol at %s: %v", cost, err)
		}
		byName := map[string]persistence.SymbolReturn{}
		for _, r := range got.Rows {
			byName[r.Symbol] = r
		}
		for _, want := range file.PerSymbol {
			mine, ok := byName[want.Symbol]
			if !ok {
				t.Errorf("cost %s: %s missing from the fold", cost, want.Symbol)
				continue
			}
			if mine.Episodes != want.Episodes || mine.Days != want.Days {
				t.Errorf("%s: episodes/days = %d/%d, artifact %d/%d",
					want.Symbol, mine.Episodes, mine.Days, want.Episodes, want.Days)
			}
			// Net: compare as decimals, not as strings. The artifact quantizes
			// to two places ("100.00"); Money.Decimal trims ("100").
			wantNet, err := fixedpoint.ParseMoney("USDT", want.Net[cost])
			if err != nil {
				t.Fatalf("parse artifact net %q: %v", want.Net[cost], err)
			}
			if cmp, err := mine.Net.Cmp(wantNet); err != nil || cmp != 0 {
				t.Errorf("%s at %s: net = %s, artifact = %s",
					want.Symbol, cost, mine.Net.Decimal(), wantNet.Decimal())
			}
			// Annual: the artifact publishes a PERCENTAGE at one decimal place.
			// The ×100 conversion lives here and nowhere else, because
			// rate.go:8 declines the unit.
			if pct := percentString(t, mine.Annual); pct != want.Annual[cost] {
				t.Errorf("%s at %s: annual = %s%%, artifact = %s%%",
					want.Symbol, cost, pct, want.Annual[cost])
			}
		}
	}
}

// percentString renders a ratio as the artifact's percentage: ×100, one decimal
// place, half-even. This is the only place in the package or its tests that
// multiplies by 100.
func percentString(t *testing.T, r fixedpoint.Rate) string {
	t.Helper()
	v, ok := new(big.Rat).SetString(r.Decimal())
	if !ok {
		t.Fatalf("not a decimal: %q", r.Decimal())
	}
	v.Mul(v, big.NewRat(100, 1))
	return v.FloatString(1)
}
```

- [ ] **Step 2: Run the oracle**

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestOracle -v`

Expected: PASS, all four tests.

**If any cell disagrees, stop and report it rather than adjusting the expectation.** The plan's author measured all 168 per-symbol cells and all pooled figures before writing this; a disagreement means either the Go implementation departed from the plan, or the artifact changed, and both are findings. `roundedTo9`'s half-away-from-zero rounding is the one known soft spot — if a mean disagrees in the ninth place only, check for a tie before concluding anything.

- [ ] **Step 3: Confirm the negative control can fail**

The negative control is a claim about the test, so it needs its own bite-check. Temporarily edit `runlength.go` so `Set.RunLength` ignores its argument and always folds the first declared level:

```go
	rows, err := s.at(EntryAt{Entry: s.levels[0]})
```

Run: `go test ./internal/strategy/infrastructure/persistence/... -run TestOracle_NegativeControl -v`
Expected: **FAIL**, naming the level mix-up.

Revert the edit. Run the same command again.
Expected: PASS.

An agreement that was never shown capable of failing is not evidence. Record both outputs in the commit message.

- [ ] **Step 4: Commit**

```bash
git add internal/strategy/infrastructure/persistence/oracle_test.go
git commit -m "test(persistence): oracle against summarise.py, with its negative control

The Go fold and summarise.py are two independent implementations. They agree on
every pooled figure and all 168 per-symbol cells, and disagree in exactly three
places at level 0.0002 — episodes, run_mean and mean_gross — which is F103's
magnitude, asserted rather than tolerated.

The negative control was bite-checked: pinning RunLength to the first declared
level makes it fail, naming the mix-up; reverting makes it pass."
```

---

## Task 9: Mutation run, collapse audit, and the record update

**Files:**
- Modify: `docs/superpowers/specs/2026-08-30-persistence-reader-design.md` (status line and the seventeenth-collapse note)
- Modify: `docs/HANDOFF.md`
- No production-code changes expected; any that the mutation run forces get their own commit.

**Interfaces:** none. This task produces evidence, not code.

- [ ] **Step 1: Run the four mutations record §103 §6.3 names**

Run each and record the verdict. `tools/mutate.sh <file> <line> <sed-command> <description> [test-target]` — find the line numbers with `grep -n` first; do not guess them.

```bash
TARGET=./internal/strategy/infrastructure/persistence

# 1. The percentile index, shifted DOWN. Killed by runlength_test.go's
#    nearest-rank fixture, whose p50/p75/p90 differ from their neighbours by 1.
tools/mutate.sh internal/strategy/infrastructure/persistence/runlength.go \
  "$(grep -n 'i := (len(sorted) \* q) / 100' internal/strategy/infrastructure/persistence/runlength.go | cut -d: -f1)" \
  's|i := (len(sorted) \* q) / 100|i := (len(sorted)*q)/100 - 1|' \
  "percentile index off by one, downward" "$TARGET"

# 2. The percentile index, shifted UP. A wrong mutant does more, not less: this
#    one must also be killed, and by a DIFFERENT assertion.
tools/mutate.sh internal/strategy/infrastructure/persistence/runlength.go \
  "$(grep -n 'i := (len(sorted) \* q) / 100' internal/strategy/infrastructure/persistence/runlength.go | cut -d: -f1)" \
  's|i := (len(sorted) \* q) / 100|i := (len(sorted)*q)/100 + 1|' \
  "percentile index off by one, upward" "$TARGET"

# 3. The wins comparison, > to >=. THIS IS THE ONE THAT MATTERS: it is killable
#    only by testdata/cost-boundary.json, because the committed file has zero
#    episodes accruing exactly any of the three costs. If it SURVIVES, the
#    fixture is not doing what §6.2 item 2 says it does.
tools/mutate.sh internal/strategy/infrastructure/persistence/net.go \
  "$(grep -n 'if cmp > 0 {' internal/strategy/infrastructure/persistence/net.go | cut -d: -f1)" \
  's|if cmp > 0 {|if cmp >= 0 {|' \
  "wins boundary: strictly greater becomes greater-or-equal" "$TARGET"

# 4. The span floor, restoring summarise.py's max(1, days). Killable only by
#    testdata/degenerate-span.json.
tools/mutate.sh internal/strategy/infrastructure/persistence/persymbol.go \
  "$(grep -n 'if days < 1 {' internal/strategy/infrastructure/persistence/persymbol.go | cut -d: -f1)" \
  's|if days < 1 {|if days < 0 {|' \
  "span floor: the degenerate refusal stops firing" "$TARGET"

# 5. The Excluded accumulation, dropped. Killable only by
#    TestPerSymbol_TheDenominatorMovesWithTheLevel.
tools/mutate.sh internal/strategy/infrastructure/persistence/persymbol.go \
  "$(grep -n 'out.Excluded = append(out.Excluded, r.symbol.name)' internal/strategy/infrastructure/persistence/persymbol.go | cut -d: -f1)" \
  's|out.Excluded = append(out.Excluded, r.symbol.name)|_ = r.symbol.name|' \
  "Excluded accumulation dropped" "$TARGET"

# 6. The censoring split, removed: censored records rejoin the run population.
#    Killable by the oracle's F103 assertions and by the decode counts.
tools/mutate.sh internal/strategy/infrastructure/persistence/decode.go \
  "$(grep -n 'if ep.Run == 0 {' internal/strategy/infrastructure/persistence/decode.go | cut -d: -f1)" \
  's|if ep.Run == 0 {|if false {|' \
  "censored records admitted to the run population" "$TARGET"
```

Expected: `caught` on all six.

Read the verdicts literally. `NO-OP!!` means the pattern did not match — fix the `sed` and re-run; it is not a kill. `NOBUILD!!` means the mutant did not compile — **read the compiler error**: if it says "does not implement" then a compile-time assertion caught it and that is a kill, and otherwise re-aim. `SURVIVED` on mutation 3 is the load-bearing failure: it would mean `testdata/cost-boundary.json` does not contain an episode accruing exactly the cost, and the §6.2 finding is unchecked.

- [ ] **Step 2: Run the fixture-collapse checklist over what was just written**

For each fixture in `testdata/`, and each table-driven case, ask the repo's standing question: *is there a parameter pinned to the value that collapses the distinction the code makes?* Check at least these, and write the answers into the commit message:

1. **`percentile.json`** — do p50, p75 and p90 land on three *different* values? (Yes: 6, 8, 10.) If they were equal, an index shift would be invisible.
2. **`cost-boundary.json`** — does it contain an episode accruing *exactly* the cost? (Yes, `"0.0021"`.) Without it, `>` and `>=` are one predicate.
3. **`buckets.json`** — does it contain a run *at* 12 and a run *above* it? (Yes: 12, 13, 30.) With only runs above 12, "cap at 12" and "cap at 13" agree.
4. **`all-censored.json`** — three censored records, not one. A fixture of one collapses "this record's" into "the first record's".
5. **`degenerate-span.json` and `one-day-span.json`** — the pair straddles the boundary. One alone cannot separate `< 1` from `<= 1`.
6. **`hurdle-boundary.json`** — does its annual return sit *between* the ratio hurdle and the rounded-percentage hurdle? This is the seventeenth collapse and the fixture exists only for it.
7. **The oracle's level loop** — does it assert on `episodes` (12322/3425/1259/472, four distinct values) rather than on `p75` (3/3/3/3)? This is F83's own collapse and it would reproduce inside the test written to fix it.

If any answer is "no", fix the fixture before committing. This checklist runs *before* the mutations in future work; it runs after here only because the mutations were already scripted.

- [ ] **Step 3: Run the full CI gate**

Run: `make ci`

Expected: PASS. In particular `nofloat` must be silent — if it fires, the `Hours()/24` form from Task 6 Step 3 survived somewhere.

Run: `grep -rn "SPEC EDIT OWED" internal/ tools/`

Expected: no matches. This package takes no deviation from the spec; if it did, the marker would be here and the spec edit would land before the slice closed.

- [ ] **Step 4: Update the design record and HANDOFF**

In `docs/superpowers/specs/2026-08-30-persistence-reader-design.md`:
- Change the status line from `design approved, unimplemented` to `implemented <date>, <short sha>`.
- Add to §6.2 a fourth numbered item recording the **seventeenth collapse**: on the committed file, comparing the hurdle against the stored ratio and against the rounded percentage give identical counts (4 / 3 / 0) at every cost and identical medians, so `testdata/hurdle-boundary.json` is what separates them. Say where it was found — writing the implementation plan, before any code — because that is the same "changing the instrument found it" pattern rounds 9 through 12 recorded.
- Add to §9 under **Claims** the measured result of §6.1: which figures agreed, and that the three disagreements at level 0.0002 are F103's magnitude and are asserted rather than tolerated.

In `docs/HANDOFF.md`: replace the open next action *"a reader for the persistence artifacts"* with what now exists, and note that the reader has no production consumer until phase 3 — §3.2 of the record argues that is the right shape rather than a gap.

**And fix one thing HANDOFF.md says.** Record §103 §3.3 names it: HANDOFF restates *"median episode is 2 settlements"* with no entry level attached, and at two of the four declared levels the median is **1**. That sentence is F83 three hops downstream. Give it its coordinate.

- [ ] **Step 5: Commit**

```bash
git add internal/strategy/infrastructure/persistence/ docs/
git commit -m "docs: record 103 implemented — the Go reader for the persistence artifacts

Six mutations, six kills. The load-bearing one is the wins boundary: > to >=
survives every assertion over the committed file, because zero of 17478
episodes accrue any of the three costs, and is killed only by the hand-built
fixture.

Adds the seventeenth collapse to the record: the committed data cannot separate
a hurdle comparison on the stored ratio from one on the rounded percentage,
found while writing the implementation plan.

HANDOFF's 'median episode is 2 settlements' now carries its entry level. At two
of the four declared levels it is 1."
```

---

## Self-Review

**Spec coverage.** Every section of record §103 maps to a task: §3.1's recompute-only decision is Task 2's `Load` plus the `doc.go` rationale, with `summary.json` appearing only in Task 8. §3.2's no-port decision is `doc.go` and the absence of any `ports.go` edit. §3.3's nesting coordinates are Task 1, and every result struct in Tasks 3–7 embeds its coordinate. §3.4's hurdle-as-argument is Task 1's `Hurdle` and Task 7's refusal test. §4's five methods are Tasks 3–7. §4.1's three refusals are Tasks 1, 2 and 6. §4.2's `Excluded` is Task 6. §5's arithmetic rules are the Global Constraints plus Task 8's `roundedTo9`. §6.1's oracle and negative control are Task 8. §6.2's three collapse results are Tasks 3, 5 and 6, and a fourth is added in Task 9. §6.3's mutation targets are Task 9, with two extra mutants. §7's F102 is Task 2's layout constants and test; §7's F103 is Task 2's split, Task 3's `Censored` field and Task 8's asserted disagreement. §8's scope boundary is the Global Constraints.

**Two known gaps, stated rather than hidden.** First, Task 2 cannot commit alone because its test file references `RunLength`; the plan says so and refuses the stub that would paper over it. Second, `testdata/hurdle-boundary.json`'s `accrued` value is derived arithmetically in Task 7 Step 1 rather than measured from a run, so the implementer must verify it produces an `Annual` strictly between `0.046` and `0.0465` before trusting the test — the plan says to adjust the fixture, never the assertion.

**One thing this plan does not decide.** Whether `RunLength.Mean` excluding censored records is *right* is a judgement, not a measurement: it is the reading record §103 §7 argues for, and the oracle asserts the resulting disagreement rather than hiding it. If a reviewer prefers the artifact's reading, that is a change to §7 of the record first and to `decode.go` second — in that order.
