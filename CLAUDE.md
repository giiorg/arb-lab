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

§3.6 of the core has ten structural rules. Six are gates; three are enforced
today as custom `golang.org/x/tools/go/analysis` analyzers in `tools/lint/`,
because a linter you cannot unit-test cannot be *shown* to bite:

| Gate | Enforces |
|---|---|
| `nofloat` | No `float64` anywhere under `internal/`. Type-based, default-deny, `//nofloat:allow <reason>` escape. |
| `notimenow` | No ambient clock reads in `domain/`, `application/` or `engine/`. Transport-only escape, `engine/` alone. |
| `kernelpurity` | A `platform/` package may import only the standard library and other `platform/` packages. |

`hexboundary`, `singlewriter` and `exhaustiveswitch` are deferred until there is
a bounded context, an OMS write repository and a state machine to police. A lint
over an empty tree passes trivially, which is indistinguishable from a broken
lint.

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
- **The zero value is unusable.** Every constructed type in `platform/` fails
  `Validate()` when nobody populated it, because an unpopulated cost reads as
  free and an unpopulated rate reads as zero fee — both compute the economic
  gate in our favour with nothing failing.
- **Write the oracle before trusting the implementation.** Real defects in this
  repo have been found by two independent implementations disagreeing, not by
  review. `fixedpoint` has a `math/big` oracle; `ids` has one for its base32
  codec.
- **Every claim gets bite-checked.** Before a test is trusted, mutate the code it
  covers and confirm it fails. Scope the mutation to a line and print the diff —
  a file changing is not evidence about *which* function changed. A surviving
  mutant is either a test gap or a wrong mutant, and telling which is the work.
- **Positive controls, always.** Any harness reporting "the broken thing failed"
  must also show the real thing passes, or a universally-objecting predicate
  reads as a perfect score.
- **A clean run and a vacuous run look identical.** Coverage claims are
  measurements. Counting executions says nothing until you have counted how many
  reach the interesting branch.
- **Deviations from the spec are marked, not absorbed.** Take the deviation if it
  is right, mark it `[SPEC EDIT OWED]`, and land it in the spec before the slice
  closes. External review has to be able to tell deliberate change from drift.

## Commits

No AI or model attribution trailers in commit messages, PR descriptions, or any
other authored content.
