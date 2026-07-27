# arb-lab

A funding-rate arbitrage platform in Go. Cash-and-carry first: hold spot, short
the perpetual, collect funding. Milestone 2 is a deliberate **kill gate** — if
measured net carry does not clear the cost hurdle, the project stops.

## Read these before changing anything

The specification is five documents in `docs/`, and it is **normative**. Code
that departs from it is either a bug or a spec edit, and the difference has to be
recorded rather than left as drift.

| Document | Holds |
|---|---|
| `docs/build-spec.md` | The normative core. Requirements live here. |
| `docs/state-machines.md` | The safety kernel's seven state machines. Normative by reference. |
| `docs/venue-profiles.md` | Verified venue facts. Nothing goes in unverified. |
| `docs/adr-history.md` | Decisions and finding traceability. |
| `docs/tutorial.md` | Teaching companion (+ `tutorial.ka.md`, **generated** — never hand-edit). |
| `docs/HANDOFF.md` | **Current state and what to do next. Start here.** |

`docs/formal/` holds a TLA+ model of the fence/takeover/plan composition. It has
found four spec defects (`FM-1`…`FM-4`) that four rounds of human and machine
review did not. Treat it as authoritative about the safety kernel's behaviour.

Session decision records live in `docs/superpowers/specs/`. Each one records
**why** a decision was taken *and the alternatives that were rejected*, so a
later session does not silently reopen a settled question. Add to them; do not
skip them.

## Layout

`internal/` puts **first-class business capabilities at the top level** and
groups **supporting machinery** beneath `platform/`, `engine/` and `app/`. So
`ls internal/` shows contexts and machinery groups as peers, and which is which
is a claim about the domain, not about the filesystem:

```
internal/
  marketdata/ strategy/ risk/ oms/ treasury/ audit/   <- bounded contexts
                                                          (domain/ + application/ + infrastructure/)
  platform/    shared kernel — reimplemented fresh, never a dependency (§11)
  engine/      technical subsystems, no DDD layering (venue, state, backtest)
  app/         composition roots, one per binary
```

There is no `boundary/` or `contexts/` grouping segment above the contexts, and
that is settled (decision record 2026-07-26 §2): none of §3.6's gates key on such
a prefix, so it carried zero enforcement value.

## Build

```
make ci        # what CI runs: fmt-check, vet, custom gates, race tests — both modules
make test      # race tests, pinned rapid seed
make fuzz      # bounded fuzz run of the decimal parser
```

**Two Go modules on purpose.** The root module is the engine; `tools/lint` is
nested so `golang.org/x/tools` never enters the engine's dependency graph. There
is no `go.work`, so every target visits both explicitly and a local run resolves
identically to CI.

**`go` and `toolchain` are separate directives and stay that way.** `go` is the
permissive minimum; `toolchain` is what we build with. `.github/workflows/ci.yml`
installs via `go-version-file: go.mod`, which quietly makes the `go` line double
as the CI pin — the split is what keeps both honest.

## The rules that bite

§3.6 of the core has ten structural rules. Six are gates, and **all six are
enforced** as custom `golang.org/x/tools/go/analysis` analyzers in `tools/lint/`,
because a linter you cannot unit-test cannot be *shown* to bite:

| Gate | Enforces |
|---|---|
| `nofloat` | No `float64` anywhere under `internal/`. Type-based, default-deny, `//nofloat:allow <reason>` escape. |
| `notimenow` | No ambient clock reads in `domain/`, `application/` or `engine/`. Transport-only escape, `engine/` alone. |
| `kernelpurity` | A `platform/` package may import only the standard library and other `platform/` packages. |
| `hexboundary` | `domain/` and `application/` may not import `infrastructure/` or `engine/`. Classifies both ends of the edge, so it scopes to this module's `internal/` rather than matching a bare path segment. |
| `exhaustiveswitch` | A switch over a module-declared enum-like must name every member. A `default:` does not excuse a missing one — that is where the new member lands silently. No escape: name the rest in a grouped case, which the compiler checks. |
| `singlewriter` | Only `oms` may import the OMS write path — `oms/application/ports` **and** `oms/infrastructure/driven`, by position, `app/` roots included. The port is policed as well as the adapter, because depending on the abstraction is the *recommended* way to depend on anything. `oms/domain` is deliberately out: a projection naming the state enum is §5.1's pattern working. |

**The last deferral expired on 2026-07-27 and the gate landed the same session
as its subject**, which is now the rule rather than the exception: three gates
were deferred against the "build it before the code it polices" preference, for
the one reason that survives scrutiny — a lint over an empty tree passes
trivially, which is indistinguishable from a broken lint — and all three landed
the session their subject appeared.

A deferral must name **the construct the analyzer reads**, not the feature that
motivated it. Rule 10's named the wrong one: it waited on a *state machine*,
while what actually made it buildable was the kernel's eleven enum-likes, which
had existed for several slices — it found 13 silent fallthroughs on its first
real run, all reachable the whole time. Rule 6's named the right one ("the
write repository"), and §12's milestone roster named the same trigger, so
nothing had to be re-derived to know when it was due.

**A clean gate run is not evidence, and neither is a probe two gates can see.**
`singlewriter`'s obvious probe — `marketdata/application` importing
`oms/infrastructure/driven` — trips `hexboundary` as well, so it shows that
something objected rather than that this rule did. The isolating probe is a
context **root** importing `oms/application/ports`, where rule 1 is silent
because the importer is not an inward layer. That is also the realistic
accident, so the isolating fixture and the motivating case are the same one.

## How work is done here

These are not style preferences. Each one was paid for.

- **Money is never a float.** All money is dimensional `platform/fixedpoint`:
  `Money`/`Quantity`/`Price`/`Rate` are distinct compile-time types, the
  asset tag is checked at runtime, and every rounding decision is an explicit
  argument. Milestone 2's kill-gate verdict is the output of a money
  computation — a wrong rounding direction does not crash, it moves the number
  across the threshold.
- **Time is injected.** Never `time.Now()`. `platform/clock` is the seam, and
  live-vs-backtest interchangeability *is* the parity mechanism, not a testing
  convenience.
- **When a default has two directions, pick the one that is right when somebody
  forgets it.** `nofloat` denies `float64` until a reason is written down.
  `platform/config` denies hot-reload of a field until a `config:"hot"` tag says
  otherwise, so a field added next year is restart-required rather than silently
  live. `platform/observability` denies a metric label with no declared value
  set until an `Unbounded` reason is written, because a label with an open
  domain is the metrics equivalent of an unbounded queue — one time series per
  `ClientOrderID`, forever, with every scrape succeeding and zero drops
  reported. The failure you get from forgetting should be a refusal that names
  the field, not a running process that picked up a new journal path mid-trade.
  There is one deliberate exception, and it is instructive: an *unrecognised*
  `config` tag value is an error rather than a silent deny, because there the
  safe direction is also the silent one.
- **The zero value is unusable.** Every constructed type in `platform/` fails
  `Validate()` when nobody populated it, because an unpopulated cost reads as
  free and an unpopulated rate reads as zero fee — both compute the economic
  gate in our favour with nothing failing.
- **Write the oracle before trusting the implementation.** Real defects in this
  repo have been found by two independent implementations disagreeing, not by
  review. `fixedpoint` has a `math/big` oracle; `ids` has one for its base32
  codec.
- **Every claim gets bite-checked.** Before a test is trusted, mutate the code it
  covers and confirm it fails. `tools/mutate.sh` does this, and it reports six
  verdicts rather than two — because five separate things have masqueraded as a
  kill here, and each guard exists because its absence produced a false
  all-clear:

  | Verdict | Means |
  |---|---|
  | `caught` | the suite failed — the claim is checked |
  | `SURVIVED` | the suite passed with broken code |
  | `NO-OP!!` | the pattern did not match; the mutation never ran |
  | `NOBUILD!!` | the mutant did not compile, so the tests never ran — **unless the compiler error is the claim**, see below |
  | `TIMEOUT!!` | the suite **hung**; a hanging test names no defect |
  | `PANIC!!` | the mutant compiled, ran and **crashed** — a kill, but by crash, so the claim you aimed at may still be unchecked |

  Set `MUTATE_MODULE=tools/lint` for the analyzers: `go test ./tools/lint/...`
  from the root fails with *"main module does not contain package"*, which is
  non-zero, which the harness would read as a kill.

  **A claim the COMPILER enforces reports `NOBUILD!!`, and that is a kill rather
  than a non-result.** §3.6 rule 2's `var _ ports.X = (*Impl)(nil)` exists
  precisely to fail `go build`, so breaking the adapter it guards produces the
  verdict that means "re-aim" while actually meaning "caught". Read the compiler
  error the harness prints: *"does not implement ports.X (missing method Y)"* is
  the assertion doing its job, and it names its own line. Every other `NOBUILD!!`
  still means re-aim. Same trap as the two above it, one layer out — the verdict
  and the outcome are not the same thing, and here they point opposite ways.

  It scopes the mutation to a line and prints the diff, because a file moving is
  not evidence about which function moved. A surviving mutant is either a test
  gap or a wrong mutant, and **telling which is the work**.

  ```
  tools/mutate.sh <file> <line> <sed-command> <description> [test-target]
  ```

  `gremlins` is the automated complement, audited in the decision record §6b: it
  works and it found four real gaps, but it stays out of CI on measured grounds
  — a warm test cache makes it report 0.00% efficacy, and roughly one run in four
  emits a false `LIVED`. Run `go clean -testcache` first, and reproduce every
  `LIVED` by hand before believing it.
- **Positive controls, always.** Any harness reporting "the broken thing failed"
  must also show the real thing passes, or a universally-objecting predicate
  reads as a perfect score.
- **A clean run and a vacuous run look identical.** Coverage claims are
  measurements. Counting executions says nothing until you have counted how many
  reach the interesting branch — and this reaches past fuzz corpora to ordinary
  fixtures. Every `events` coalescing test used a mailbox depth of 1, where
  "merge against the newest" and "merge against the oldest" are the same slot, so
  a real defect was unreachable while looking covered. Every `outbox` malformed-
  identifier fixture then put its bad character mid-string, where `i >= 0` and
  `i > 0` are the same predicate — so `" audit"` validated. Then
  `observability`'s series-key collision fixture, `("ab","c")` vs `("a","bc")`,
  contained **no separator** — so it collided only under the *empty* separator
  and passed against every wrong choice a person would actually make. Then
  `ratelimit` did it twice in one session: two tests used a profile whose worst
  refusal was also the **first one evaluated**, so "name the first" and "name the
  worst" were the same answer; and every `freeAt` case sat strictly inside or
  strictly outside the ceiling, where `<=` and `<` are the same predicate. Then
  `boot`'s reverse-teardown test used **three** closers, where reversing the
  slice and swapping only its ends produce the same order. Then
  `exhaustiveswitch`'s single-member fixture *covered* its one member, where
  "one constant is a set" and "one constant is not a set" give the same answer —
  and the mirror of it, `within()`'s `pkgPath == dir` arm, was untested in
  **both** `hexboundary` and the `kernelpurity` it was copied from, because
  every fixture in either imported a SUBpackage. **Nine times now**,
  which makes it a checklist item and not a war story: before believing a suite,
  find the fixture parameter pinned to the value that collapses the distinction
  the code makes, and vary it. The tell is that the fixture does not contain the
  thing under test. Run the checklist **before** the mutations rather than after
  — in `boot` it changed five fixtures for free.
- **A test that hangs is not a test that fails.** A mutation can make a test
  *block* rather than break, so the harness reports `TIMEOUT!!` instead of a
  kill and the claim was never checked while looking covered. `clock` found it
  on a non-blocking send; `boot` found it on an unbounded channel receive, where
  breaking the signal subscription left the test waiting for a signal that would
  never arrive. Any test reading from a channel a mutation could stop feeding
  needs its own bound.
- **A fix applied in one place gets applied to its mirror.** `Receive` was tested
  across 200 trials because a Go `select` picks a ready case at random; `Send` —
  same situation, same coin flip — was tested once, and a mutant survived on it.
  When a test earns a repetition, a bound or an isolation trick, go and look for
  the symmetric case immediately.
- **Deviations from the spec are marked, not absorbed.** Take the deviation if it
  is right, mark it `[SPEC EDIT OWED]`, and land it in the spec before the slice
  closes. External review has to be able to tell deliberate change from drift.
  The slice-close gate is `grep -rn "SPEC EDIT OWED" internal/ tools/` — **both
  paths**, and the second was added the session normative claims first appeared
  outside `internal/` (the analyzers assert things about §3.6). Scoped to code
  because the doc set is full of prose about past markers. Two limits, both paid
  for: the grep cannot see an **unmarked** claim, so a reading pass over what the
  slice asserts normatively goes beside it, not into it; and it cannot see a
  marker in a directory nobody added to the pattern.

## Commits

No AI or model attribution trailers in commit messages, PR descriptions, or any
other authored content.
