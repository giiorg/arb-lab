# Milestone 1 — foundation decisions (2026-07-26)

**Status:** in progress. Milestone 1's **first slice (§1) is code-complete** as of session 3 — scaffolding,
`platform/fixedpoint`, `platform/clock`, and the `nofloat`/`notimenow` gates. Branch
`feat/milestone-1-foundation`, first commit `e92f9f0`, head `5893ac0`, **pushed and tracking**.
**Doc set:** implements `build-spec.md` §12.1 (v0.9.4). Deviations from the spec are marked **[SPEC EDIT LANDED — v0.9.5]**.

This record exists so a fresh session can resume without re-deriving anything. Every decision below was
taken explicitly by the user during the session, with the rejected alternatives recorded — the same
discipline `docs/formal/` applies to design rejections. Where a decision *departs* from the reviewed
spec, that is flagged, because R2d must be able to tell a deliberate change from drift.

---

## 0. The language question — asked, assessed, answered

**Decision: Go is confirmed.** The user asked, before any significant investment, whether Rust would be
the better choice. It was assessed properly rather than deflected, and Go was confirmed **with one
compensating measure** (§4, the `exhaustiveswitch` gate). This is recorded because "we already started"
is not a reason, and a future session should not reopen it without new information.

### Cost of switching, measured rather than guessed

| Artifact | Would be discarded | Cost |
|---|---|---|
| Code written | ~700 lines, 1 commit | ~1 day — negligible |
| `docs/formal/` (TLA+, 1220 lines + 8 configs) | Nothing | Zero — TLA+ is language-agnostic |
| Guide + tutorials incl. Georgian (~530 KB) | ~Nothing | Near zero; guide Part 5 "the Go build" is **not yet written** |
| **Spec Go signatures** — §3.3, §3.4, §3.5, §4, §5.1, §5.3, §11, D1–D4, §3.6 rules 4/7/8 | Substantial | **1–2 sessions + its own review round** |

The sunk cost is *not* in code. It is in a 415 KB document set that survived four external review rounds
and speaks Go natively (`interface`, `type X string` + `Validate()`, "No DI container", `Envelope{Payload any}`).

### Where Rust genuinely wins — recorded honestly, not strawmanned

1. **Exhaustive state machines — the strongest argument.** The safety kernel is seven state machines and
   four TLA+ rounds found four spec defects. A Rust `enum` makes a new state break **every** `match`
   site at compile time. Go's `switch` silently falls through to `default`. → *Compensated by the
   `exhaustiveswitch` gate (§4).*
2. **The money kernel specifically.** Native stable `i128` would replace ~30 lines of hand-written
   `math/bits` overflow handling **and would make the 128-bit/scale-16 mantissa essentially free** — a
   decision taken *against* today partly because Go makes a 16-byte value type painful. `Result` + `?`
   would eliminate the entire `Calc` type, which exists solely to work around Go's lack of `?`.
   `#[must_use]` makes a dropped error a compiler warning rather than an `errcheck` finding.
3. **Mutation tooling.** `cargo-mutants` is actively maintained; Go's `gremlins`/`go-mutesting` are not.
   This cuts directly against the earlier in-session advice to rely on negative controls instead.

**Explicitly debunked: "no GC pauses".** Irrelevant here. This is a funding-carry strategy on 8-hour
settlement cycles, not latency arbitrage. Sub-millisecond GC pauses cannot affect it. Any future
argument for Rust on latency grounds for *strategy #1* is pattern-matching, not reasoning.

### Why Go still wins for *this* architecture

**The decisive argument is structural, not preferential: this design is ports-and-adapters maximalist,
and async trait objects are Rust's worst-supported combination.** §3.6 rule 2 makes *every* driven
dependency an interface; §4 declares segregated capability interfaces per context (`OrderGateway`,
`AccountSource`, `MarginGateway`, `ProtectionGateway`, `FenceGateway`, `EmergencyPlanStore`,
`HistoricalOrders`, …); §3.7 requires ports stay swappable for SDK clients; §3.5 makes live-vs-backtest
adapter interchangeability *the entire parity mechanism*. That is ~20 I/O-performing ports needing
dynamic dispatch. `dyn`-compatible async traits still require the `async-trait` macro or hand-boxed
`Pin<Box<dyn Future>>` (as of the assessing model's knowledge cutoff — **treat as a checkable fact and
re-verify if this is ever reopened**). The tax lands at every seam, repeatedly, for the project's life.

Secondary: six milestones remain (OMS, risk, treasury, audit, actuator daemon, backtester, control
plane) for a **solo developer**, and §3.4's typed lanes over goroutines/channels is what Go is best at.

### The framing to keep

The trade is **not** "safety vs velocity". It is: *Rust buys compile-time case coverage and better
mutation tooling; Go buys frictionless dynamic dispatch across ~20 ports plus everything already
written.* **The correctness ceiling is set by neither** — it is set by **TLC trace-checking the real
implementation against the TLA+ model**, which the spec already plans (HANDOFF formal-methods block) and
which is *stronger* than exhaustiveness: exhaustiveness proves every case is handled, not that it is
handled correctly.

**[SPEC EDIT LANDED — v0.9.5]** Record this as an ADR §3 entry — evaluated-and-rejected, with the reasons above,
so it reads as a decided question rather than an unexamined assumption.

---

## 1. Scope — what milestone 1's first slice actually is

§12.1 reads as one sentence but lists eleven kernel packages, two engine subsystems, the parity
dispatcher and four CI gates. That is several sessions. The **first slice** is:

- repo scaffolding: `go.mod`, build harness, CI, the analyzer harness
- `platform/fixedpoint` — the deep one
- `platform/clock`
- the `nofloat` and no-`time.Now()` gates

**Rationale, not preference.** The eleven kernel packages form a strict dependency spine:
`fixedpoint`/`apperror`/`ids`/`clock` import nothing; `events` needs `clock`; `config` needs
`fixedpoint`; `boot` needs everything; `engine/venue` and `engine/state` sit on top of all of it.
Anything except the bottom of the spine gets rewritten when the bottom lands. And the two gates chosen
are exactly the two that police the two bottom packages: `nofloat` makes `fixedpoint` unavoidable,
no-`time.Now()` makes `clock` unavoidable.

**Why `fixedpoint` first specifically:** milestone 2 is a **kill gate** (≈3.4% vs a ~4.6% hurdle,
§12.2). That verdict is the output of a money computation. A wrong rounding direction or a lossy float
conversion does not crash — it moves the number across the threshold and the project is killed or
green-lit on a bug.

### Deferred from this slice — each with a reason

| Deferred | Reason |
|---|---|
| `platform/events` (lanes), `outbox`, `ids`, `config`, `apperror`, `boot` | Higher up the dependency spine |
| `engine/venue`, `engine/state`, `platform/observability` | Sit on top of the entire kernel |
| The parity dispatcher | Needs `events` + `clock` + persisted `DispatchSeq` |
| `hexboundary`, `singlewriter` gates | **No contexts exist to police.** A lint over an empty tree passes trivially, which is indistinguishable from a broken lint |
| `exhaustiveswitch` gate | No state machines exist yet (see §4) |
| E2E tests | No binary, no venue, no data. First honest E2E is milestone 2's capture→replay→sim; milestone 3's testnet process-kill-mid-IOC |
| DB integration tests | `fixedpoint`'s SQL codec is a *value* codec, not a schema. "SQLite INTEGER round-trips int64 exactly" is a property of SQLite, not of our code |
| Consumer-driven contract tests | Belong on the venue REST/WS surface (recorded-response golden fixtures) — milestone 2/3 |
| Ubiquitous-language glossary | Would document imagined code. Terms get recorded as each package lands; formalize via the `domain-modeling` skill when the first context arrives in milestone 3 |

---

## 2. Repo layout

- **`go.mod` at repo root**, module `github.com/giiorg/arb-lab` (matches the push remote), `go 1.26.0`.
  `go mod init` wrote `1.25.0` despite a 1.26.0 toolchain; pinned up so the declared language semantics
  match what is actually tested.
- **`go` and `toolchain` are separate directives, and separating them is the decision (session 5).**
  On the 1.26.0 → **1.26.5** upgrade the obvious move — bump `go` to `1.26.5` — is the wrong one twice
  over. A patch release changes no language semantics, so as a *minimum* it buys nothing and only
  narrows who can build the module. But leaving it alone is also wrong here, because
  `.github/workflows/ci.yml` installs with **`go-version-file: go.mod`** deliberately, "so the
  toolchain is stated in exactly one place" — which quietly makes the `go` line double as the CI pin,
  and would have left CI on 1.26.0 while local work moved to 1.26.5. **`toolchain go1.26.5` resolves
  both**: `go` stays the permissive minimum, `toolchain` states what we build with, `setup-go`
  documents that it prefers `toolchain` when present, and the version still lives in one file.
  **Both modules carry it** — there is no `go.work`, so each resolves its own toolchain, and leaving
  the nested one unstated would let the two halves of `make ci` build on different releases.
- **`tools/lint` had been declaring `go 1.25.0` since the scaffold commit — fixed in session 5.** The
  same `go mod init` default that was caught and pinned up in the root module was missed in the nested
  one, in the same commit, so the gates had been built and tested on 1.26 while claiming 1.25 for
  months. Nothing broke, which is the point: **an untested compatibility claim fails silently until
  someone acts on it.** Now `1.26.0`, matching the root.
- **`main` was fast-forwarded to the v0.9.4 spec** (it was a strict ancestor, 0 ahead / 57 behind — no
  merge commit, no conflict), then `feat/milestone-1-foundation` cut from it. Rejected: branching code
  off the docs branch (leaves `main` stale), and branching off stale `main` (the branch's own `docs/`
  would disagree with the spec it implements). **`main` is not yet pushed** — see the resume block.
- **`tools/lint` is a nested module.** Keeps `golang.org/x/tools` out of the engine's dependency graph,
  which §11 cares about ("`platform/` … reimplemented fresh; never a dependency").
- **No `go.work`, and it is gitignored.** A workspace makes the root module resolve dependencies through
  `tools/lint`'s `go.mod`, so `x/tools` would become quietly importable from the engine — defeating the
  entire reason the module is nested. The `gopls` "not in your workspace" warnings are the accepted
  cost; they are editor-only and `go test` is unaffected.
- **`migrations/` is partitioned by dialect from the first file** (`migrations/<dialect>/`), keeping the
  documented SQLite→Postgres path (D2) cheap. `migrations/README.md` records what ports for free (the
  `int64` mantissa → `INTEGER`/`BIGINT`; single-writer OMS so no `FOR UPDATE` to port; SQL confined to
  `infrastructure/driven/`) and what is expected to be rewritten (pragma verification + online-backup
  durability story; the outbox reader's `LISTEN/NOTIFY`/`SKIP LOCKED`; upsert/DDL phrasing).

### The `boundary/` grouping segment is dropped **[SPEC EDIT LANDED — v0.9.5]**

Bounded contexts sit at **`internal/<ctx>/`** — `internal/oms/`, `internal/risk/`, `internal/marketdata/`
— alongside the machinery groups `internal/{platform,engine,app}/`.

**Why, in the order the argument actually ran:**

1. `boundary/` misleads: in DDD the term is *bounded context*, while "boundary" reads as the adapter
   *edge* — the opposite of what those directories hold (they hold domain cores, with adapters nested
   inside).
2. `contexts/` was **rejected by the user**: it collides cognitively with Go's `context.Context`, the
   most-used type in service code. Technically harmless (no package is ever named `contexts`), but it
   swaps one misreading for a more likely one.
3. No third word survives: `modules/` collides with Go modules; `services/` contradicts D1
   (single-process); `domains/`/`subdomains/` stutter against the inner `domain/` package and name the
   problem space; `core/` is taken by §3.2's `Core` structs; `boundedcontexts/` too long; `bc/` cryptic.
4. **The settling argument: none of §3.6's gates depend on the prefix.** Rule 1 keys on `domain/` and
   `application/`; rule 3 on `infrastructure/`; rule 6 names `oms` directly. The grouping segment
   carried **zero enforcement value** — the earlier concern that "the lint needs a path prefix" was
   wrong. And flattening makes `kernelpurity` *simpler*: a one-item allowlist rather than a deny-list.
5. Measured cost: ~30 occurrences, essentially all in `build-spec.md`. **One of them is not a path —
   `boundary/lint` in §3.6 means "a boundary-or-lint test" and must NOT be rewritten.**

**Organizing principle now in force:** first-class business capabilities live at the top of `internal/`;
supporting machinery is grouped under `platform/`, `engine/`, `app/`. The cost is that `ls internal/`
shows nine peers instead of four groups, so the taxonomy needs one line in `CLAUDE.md`.

---

## 3. The gates are custom analyzers, not golangci-lint config

**Decision: `golang.org/x/tools/go/analysis` analyzers for all structural rules, each with
`analysistest` `testdata` fixtures that MUST produce diagnostics.** `golangci-lint` runs alongside for
the standard suite (govet/staticcheck/errcheck) — not yet wired.

**Why not config.** `depguard` handles the two import rules well but **cannot be unit-tested**, so it
cannot be *shown* to bite; proving it needs a scratch repo. `forbidigo` for no-`time.Now()` needs
inverted path-exclusion logic that silently stops covering any newly-added context. And `nofloat`
**cannot be config at all**: it is not "no `float64` anywhere", it is no float *in the money path*,
which needs type information — a regex misses `x / y` where both operands are float-typed and misses
untyped constants that become float on assignment. §3.6's own wording is "checked by a boundary/lint
**test** in CI".

### `nofloat` scope: default-deny with an annotated escape

`float32`/`float64` is a diagnostic **anywhere under `internal/`** unless the line carries
`//nofloat:allow <reason>`, which the analyzer records so every exception is greppable. Rejected: a
configured list of money-path packages (a package nobody remembers to list is unpoliced — the same
failure mode as `depguard`), and zero-exception absolute denial (forces a conversion shim outside
`internal/` for the Prometheus seam). Legitimate escapes anticipated: `platform/observability` metric
emission, later `engine/backtest` statistics.

---

## 4. `kernelpurity` — a fifth gate, now §3.6 rule 9 **[SPEC EDIT LANDED — v0.9.5]**

**Built and green.** A package under `internal/platform/` may import only the standard library and other
`internal/platform/` packages. Catches two failure modes with one rule:

- **Domain leak** — the kernel importing a bounded context turns a technical shared library into a
  shared *domain* kernel, DDD's most tightly coupled integration pattern and the one it warns against.
  An 11-package kernel makes this a live risk.
- **Spine inversion** — `engine/` already sits on top of `platform/` (`engine/venue` consumes
  `fixedpoint`, `clock`, `ids`), so `platform → engine` inverts the spine and invites a cycle.

**Deliberate scope:** only imports *inside this module* are diagnosed. Third-party is out of scope
because `platform/observability` legitimately needs `prometheus/client_golang` (§11). Tightening that
would need a `nofloat`-style annotation escape and is **not** part of this rule.

**Implementation notes worth keeping:** the analyzer derives the module root by splitting the analyzed
package's own path at `/internal/` rather than hardcoding it — `analysistest` fixtures live in a
GOPATH-style `testdata/src/` tree, so a hardcoded module path would work in tests and mis-scope in
production. And `within()` compares whole path segments, so a hypothetical `internal/platformx` is not
mistaken for part of the kernel.

**Proven to bite:** neutering the `Reportf` makes both expected diagnostics vanish and the test fail.

### `exhaustiveswitch` — a sixth gate, now §3.6 rule 10, deferred **[SPEC EDIT LANDED — v0.9.5]**

The compensating measure for the strongest Rust argument (§0). A `switch` over a §3.6-rule-8 enum-like
(`type X string` + constants + `Validate()`) must cover every declared constant, or fail the build.
Adding a state to a safety machine then breaks compilation at every decision site instead of silently
falling through to `default`. **Deferred** because no state machines exist yet. `golangci-lint`'s
`exhaustive` linter is the off-the-shelf alternative to evaluate first — if it suffices, use it and skip
the custom analyzer.

### Analyzer roster: 6 planned, 3 built

| Analyzer | State | Notes |
|---|---|---|
| `kernelpurity` | **Built, green, bite-checked** | §3.6 rule 9 (added in v0.9.5) |
| `nofloat` | **Built, green, 8/8 mutations caught** | Type-aware; default-deny + enforced `//nofloat:allow` (§4b) |
| `notimenow` | **Built, green, 16/16 mutations caught** | Whole ambient-time family, `go/types`-resolved; `engine/` scope + transport escape (§4c, §4d) |
| `hexboundary` | Deferred | No contexts to police |
| `singlewriter` | Deferred | No OMS write repositories exist |
| `exhaustiveswitch` | Deferred | §3.6 rule 10, planned. No state machines yet; try `exhaustive` first |

---

## 4b. `nofloat` as built (session 3)

Commit `d4846cf`. **The rule is not "the token `float64` does not appear", it is "no value here is a
float"** — which is the whole reason it cannot be config. The three doors a regex cannot see:

```go
x := 1.5              // an untyped constant becoming float64 by inference
return a / b          // both operands float-typed; no float token on the line
s := d.Seconds()      // the standard library handing one over
```

The third is how it happens in practice: `time.Duration.Seconds()` and `strconv.ParseFloat` are the
doors, and **neither spells "float" at the call site**.

### Three implementation decisions

- **`Underlying()`**, so `type Basis float64` is policed at every *use*, not only at its
  declaration. Otherwise one type definition leaves the gate behind permanently.
- **Tuples unwrapped.** A multi-value call's type is a `*types.Tuple`, not a `*types.Basic`, and
  `strconv.ParseFloat` returns `(float64, error)` — without this the most likely single door is the
  one case the rule misses.
- **`complex` counts.** A pair of floats carries the same representation error, for one extra term
  rather than a second analyzer later.

### Diagnostics are per line because the escape is per line

Deliberate: a finer granularity would let one annotation silently answer several findings. The
suppression unit and the diagnostic unit are the same unit.

### The escape is enforced, not merely offered

- A bare `//nofloat:allow` **does not exempt** — an exception without a reason is an unexplained
  hole, and a malformed escape that worked as well as a valid one would be the only escape anyone
  ever wrote.
- An escape on a line with **no float is reported as unused**. Otherwise it outlives the float it
  was written for and pre-authorises the next one to land on that line.
- A **misspelled** directive (`//nofloat:allowance`) gets no diagnostic of its own. None is needed:
  the float it failed to suppress is still reported, which is how the author finds the typo.

**Both `//` and `/* */` forms are recognised, and the block form is load-bearing rather than
cosmetic.** Go allows one `//` comment per line, so a line-form directive followed by an
`analysistest` expectation is a *single* comment whose reason text swallows the expectation — the
escape would succeed and the fixture would prove nothing. Without the block form **the rule's own
malformed-escape fixtures could not be written at all**. Greppability is unaffected.

### Registering it made it fire on real code, correctly, twice

Both were genuine, and they wanted **opposite** treatment:

- **`testing.AllocsPerRun` returns a `float64` mean**, and the float *cannot* be converted away at
  the seam: a mean of 0.5 means half the runs allocated, so `int(allocs) != 0` would accept exactly
  what the assertion exists to catch. Confined to one `mustNotAllocate` helper — four annotated
  lines across two files collapsed into one place, and a duplicated assertion removed. **This is the
  behaviour the gate is for.**
- **The `"routes through float64"` negative control** is annotated *in place* on purpose. Those two
  lines exist to BE the forbidden thing so the property can be shown to reject it, so they should
  stay conspicuous rather than be tidied behind a helper.

Four escapes in the engine, each with a reason, all greppable.

---

## 4c. `notimenow` as built (session 3)

Commit `5ba3ed4`. Scope per `build-spec.md` §3.6 rule 5, unchanged: `domain/` and `application/`
segments, matched **whole-segment and at any depth** (so `oms/domain/order` is domain code and a
subdirectory is not an escape, while `domainless` is not caught by accident).

### The whole ambient-time family, not just `time.Now`

Rule 5 names `time.Now()`, and a gate that stopped there would be bypassed **by idiom rather than
by evasion**: `time.Since(start)` *is* `time.Now().Sub(start)`, and it is simply how elapsed time is
written in Go. Likewise `After`/`Tick`/`NewTimer`/`NewTicker`/`AfterFunc`/`Sleep`, which §3.5
assigns to `Clock.SetTimer` so the backtester can *drive* time rather than wait for it — a domain
that sleeps is a domain that cannot be replayed. This is rule 5's **intent**; its stated scope is
*which packages*, and that is untouched.

### Resolved through `go/types`, and reported at the reference

`import wall "time"; wall.Now()` and a dot-imported `Since(t)` are both caught; a textual
`"time.Now"` match finds nothing in either. And the diagnostic is on the **reference**, not the
call: `var deadline = time.Now` is the same violation, invisible to a call-shaped check.

### The receiver check is load-bearing — the mutation pass found it

`time.After` schedules against the clock. **`time.Time.After` is a pure comparison** between two
instants the caller already had, and comparing timestamps is the most common legitimate thing domain
code does with time. Without the `Recv() != nil` guard the rule reports every `t1.After(t2)` and
gets switched off within a week.

### No escape hatch, unlike `nofloat` — and the asymmetry has a reason

> **Superseded in part by §4d (session 4).** The reasoning below still governs `domain/` and
> `application/`, which admit no escape. `engine/` — added to the rule's scope in session 4 — does
> get one, because this argument does not transfer to transport.

`nofloat` needs one because Prometheus takes `float64` at the wire and something must convert: a
real seam with a real obligation. Here **the seam is `platform/clock` itself**, which sits outside
`domain/` and `application/` by construction, so the exception is already expressed *structurally*.
A domain layer that needs wall time is a design error, not an exception, and the fix — inject the
`Clock` — is always available. An annotation would only make it easy not to.

### The clean fixtures carry more weight than the offending ones

- **`platform/clock` must read the clock and must not be policed.** A rule that policed it could not
  be satisfied. No annotation, no configured exemption — where the code lives *is* the seam.
- **`engine/venue` pins the limit build-spec actually states.** Infrastructure stamping a received
  message with this machine's clock is arguably how a wrong timestamp enters the system, but
  widening the rule is **spec work, not an implementation choice**, so the fixture records the limit
  rather than quietly exceeding it. *(Session 4: the question was put to the user, who approved the
  widening. The fixture is now offending-and-escaped, and a new clean `engine/state` fixture takes
  over as the positive control — §4d.)*
- **`application/` holds five legitimate uses** — `Date`, `Parse`, `Duration`, `Time.Add`,
  `Time.Before`. A rule that objected to the `time` *package* would be turned off, and then rule 5
  would not be enforced at all.

### Verified through the real driver, not only the fixture tree

`notimenow` reports nothing on the engine today because **no `domain/` package exists yet**, so a
green `make lint` proves nothing about it. A probe `internal/oms/domain` package was added to the
real module: `make lint` failed with exactly one diagnostic, and the `.After` on the same line was
correctly ignored. Fixtures live in a GOPATH-style `testdata/src/` tree, and path handling that
passes there can still mis-scope in production — which is the hazard `kernelpurity`'s own notes flag.

---

## 4d. Rule 5 widened to `engine/` (session 4) — the milestone's first user question

The one item of the ten that was a question rather than a deviation. Asked, answered **yes**, built.

### The argument that actually decided it is the clamp, not parity

The obvious framing — "a venue adapter stamps a wrong timestamp, which breaks replay" — is the weak
one, and it is worth recording *why* it is weak so a future session does not re-run it:

- `engine/venue`'s live adapters **do not run under replay at all**: §3.5 substitutes a replay
  `DataSource` and a fill-model `ExecutionVenue`. Ambient time in live-only transport cannot corrupt
  a replay of code that is not replayed.
- §3.5 already states that `SourceTime`/`RecvTime` are for freshness, cross-leg skew and analysis —
  **never a reorder key**. So a raw-clock `RecvTime` does not touch `IngressSeq`, the replay key.

The argument that survives is different and sharper: **`platform/clock`'s monotonic clamp is only a
guarantee if everyone reads the same clock.** §5e decided that `Wall` *absorbs* backwards time
(high-water clamp) because it has no error to return. `time.Now` does not clamp. So the moment a
`RecvTime` stamped from the raw clock meets an age computed from the injected one, the subtraction
spans two clocks that disagree — and they disagree **exactly across the NTP step backwards the clamp
was built for**, yielding a negative or inflated age. That lands in code where freshness gates
execution (§3.4) and stale marks drive liq-distance `[→ R2c-1]`. A negative age reads as *perfectly
fresh*.

**The defect is two clocks meeting at a subtraction. The ambient call is only where it starts** —
which is also why policing the call site is the right intervention: the subtraction itself is
innocent-looking and correct.

`engine/state` is where this bites hardest, because unlike `engine/venue` it **runs under both live
and replay wiring**, and freshness there is `now − RecvTime`. Read ambiently under replay it compares
a replayed timestamp against real wall-clock now, so every snapshot reads as infinitely stale.

### Three alternatives, and why each lost

| Alternative | Why rejected |
|---|---|
| **Keep the narrow scope** (status quo) | Costs nothing today and everything later. `internal/` currently holds **only `platform/`** — no engine package exists — so widening now is one `policedLayers` entry and an escape mechanism lifted from `nofloat`. Widening after `engine/state` ships means retrofitting a freshness path that already passes its tests. §12.1 asks for the gates "from day one" for exactly this reason: the only moment a gate costs nothing is before the code it polices |
| **Widen with no escape** (strictest) | A gate with no legitimate exit gets disabled the first time it is legitimately wrong. Transport genuinely needs wall-clock scheduling, and forcing a socket read deadline onto `Clock.SetTimer` would put transport liveness on the **dispatch** timer queue — coupling a keepalive to the state machine's scheduler. That is worse architecture, not stricter discipline |
| **Police only the parity-shared engine packages** (`state`, `backtest`) | Sharpest against today's hazard, and **it is an allow-list** — the exact failure mode `notimenow`'s own design notes give as its reason for existing: "that list silently stops covering any bounded context added after it was written." A new parity-shared engine package added tomorrow would be unpoliced until somebody remembered. The principle does not get an exemption when it is inconvenient to itself |

### The claim this rests on, stated so it can be falsified

**Prediction: `engine/` transport will need wall-clock scheduling, so the escape will be used.** If
`engine/venue` ships and **no escape is ever written**, the prediction was wrong and the strict
no-escape variant was right — at which point *delete the mechanism*. That is a ten-minute change,
whereas retrofitting a missing escape after `engine/state` ships is not. The asymmetry is the whole
reason the escape exists rather than absolute denial, and it is recorded here as a claim to revisit
rather than a settled fact.

### What was built

- `policedLayers` gains `"engine"`; `escapableLayer` is a **single constant, not a set** — the moment
  it becomes a list it is the allow-list the design rejects.
- `//notimenow:allow <reason>`, per line, honoured **only** in `engine/`. Written in a policed layer
  that has no escape it suppresses nothing **and says so** — a directive that silently did nothing
  would read as a working suppression to whoever wrote it.
- Findings now **collapse to one per line**, because that is the unit the escape works in. Diagnosing
  per reference while suppressing per line would let one annotation silently answer several findings.
- The grammar is **duplicated from `nofloat`, not shared**: only the comment parsing is common, while
  the *policy* differs (`nofloat` allows an escape anywhere under `internal/`; this one only in
  `engine/`). A shared helper wrapping thirty lines of parsing under two policies would abstract the
  wrong seam. **Extract at a third gate**, not before.

### Bite-checked: 9/9, and the tenth was reported rather than counted

The widening, both directions of the escape's layer test, the mandatory reason, the unused-escape
report, the directive separator (`notimenow:allowance` must not read as a directive), the per-line
collapse, the receiver check, and the layer predicate. **A tenth pattern failed to match and the
harness printed `NO-OP!!` instead of `caught`** — the mutation script compares the file before and
after and refuses to interpret an unapplied mutation as a passing test. That guard paid for itself on
its first run, and it is the same lesson as the vacuous-coverage one: *a mutation that never applied
and a mutation that was caught produce identical test output.*

Two fixture gaps were closed by the same discipline. The per-line collapse had **no fixture with two
ambient reads on one line**, so it was untested in both directions until one was added to `domain/`
(two reads → one diagnostic) and one to `engine/venue` (two reads → one escape covers both). And
`engine/venue` flipping from clean control to offending meant the widened scope had **no positive
control at all** until `engine/state` was added — without it the suite would only ever show engine
code being rejected, which a universally-objecting gate also does.

---

## 5. `platform/fixedpoint` — the money kernel

### Representation

**`int64` mantissa at canonical `Scale = 9`, `math/bits` 128-bit intermediates.** NautilusTrader's
standard-mode precision, which §11 names as the reference. Range ±9,223,372,036.854775807 — ample for
USDT notionals and BTC quantities. **Maps 1:1 onto SQLite `INTEGER` and Postgres `BIGINT`** (both 8-byte
signed), which is what keeps the persistence codec exact and dialect-independent.

Rejected: **128-bit mantissa @ scale 16** (Nautilus high-precision) — more headroom, but a 16-byte value
type with no native `==`/`<` and a SQL codec that *diverges per dialect* (`TEXT`/paired columns on
SQLite vs `NUMERIC` on Postgres), and that divergence leaks into queries. Rejected: **`big.Int`-backed**
— allocates per operation, values stop being Go-comparable, nil-pointer zero value is a trap.

`Rate` is the one category where 9 places could eventually pinch (a funding rate compounded across
~1095 settlements). Because `Rate` is its own type rather than an alias, raising *its* scale later is a
change to one file and its codec, not a repo-wide audit.

### The four decisions that shape every call site

1. **Error style: `(T, error)` on every fallible op, plus a sticky-error `Calc` for chains.** The
   sticky error lives in a *local* calculation and never in a stored `Money`, so it cannot escape.
   Rejected: pure `(T, error)` with no helper (the break-even computation becomes a dozen consecutive
   checks); panic-on-invariant-violation (a panic mid-two-leg-execution hits exactly the
   `DispatchAuthorized` crash boundary §5.3 and the formal model were built around).
   **Note: `Calc` exists only because Go lacks `?` — see §0.**
2. **The zero value is INVALID** (asset unset): `Validate()` fails and arithmetic errors. Construct via
   `ParseMoney` / `ZeroMoney(asset)`. **Why it matters concretely:** a cost field a venue adapter forgot
   to populate would otherwise read as a *free* cost, and milestone 2's economic gate would be computed
   in our favour with nothing failing. Rejected: typeless additively-neutral zero (ergonomic, but that
   is exactly the failure); `UnknownAsset` sentinel (caught only at boundaries that remember to check).
3. **`Price` carries `base` and `quote`.** `Notional(Price, Quantity)` requires `qty.asset == px.base`
   and returns `Money` tagged `px.quote`. This is what makes the runtime tag actually catch something:
   multiplying a BTC quantity by an ETH price becomes an error rather than a plausible number.
4. **Explicit rounding mode per operation** (§3.6 rule 4). See the pending proposal below.

### Built and green (commit `e92f9f0`)

- `Scale = 9`, `scaleFactor`, `maxMantissaDigits = 19`, `maxExponent = 1000`
- `Asset` (`type Asset string`) — runtime tag, deliberately not phantom-typed because venue assets are
  discovered dynamically
- `Money` + `Validate()` + `ParseMoney` + `ZeroMoney` + `Asset()` + `Decimal()`
- `parseMantissa` / `formatMantissa` — exact, **no `float64` on any path**; grammar is strconv's minus
  Inf/NaN/hex/underscores; whitespace and trailing junk rejected rather than trimmed
- Sentinels: `ErrAssetUnset`, `ErrSyntax`, `ErrPrecisionLoss`, `ErrOverflow`

**Two findings worth not re-learning:**

- **The `int64` boundary is asymmetric.** `-9223372036.854775808` is representable but
  `9223372036.854775808` is not, because `MinInt64` has no positive counterpart. The parser therefore
  parses *unsigned* and applies the sign afterwards — a naive `strconv.ParseInt` on the digit string
  rejects the legitimate negative boundary. Both boundaries are now test cases.
- **Excess precision is `ErrPrecisionLoss`, not a silent round.** Policy, not caution: if a venue starts
  quoting finer than 9 decimals, that is a finding to act on, not something to absorb. Callers who want
  to round must say so with a `RoundingMode`.

### Tests: 25 round-trip cases, 27 rejection cases, all bite-checked

Three mutations were applied and each failed exactly the right cases: dropping the trailing-junk guard
(broke `1.2.3`, `0x10`, `1_000`, `1.2e3.4`, both whitespace cases); silently truncating excess precision
(broke `1.0000000001` **only** — revealing that the two precision-loss paths are genuinely distinct
branches, since `0.0000000001` and `1e-10` take the `drop >= len(digits)` path); letting the positive
bound wrap (broke `9223372036.854775808`).

### The rounding-mode set — CONFIRMED (session 2)

The user confirmed **the proposal as written**, unchanged: eight modes, `RoundInvalid = 0`,
`RoundExact` kept, `HalfDown` not added. Rejected at the same time: dropping `RoundHalfUp` (a
"minimal set" of seven), and the full nine-mode `java.math.RoundingMode` parity set.

**One thing the record did not say, decided during implementation:** `RoundingMode` is a
`uint8` + `iota`, *not* the `type X string` shape §3.6 rule 8 prescribes. **[SPEC EDIT LANDED — v0.9.5]**
Rule 8's shape exists for enums that cross a persistence or wire boundary, where a
self-describing stored token is worth the width. A `RoundingMode` is an *argument* — never
persisted, never transmitted — so the property that matters is the opposite one: that the zero
value be an unusable sentinel, which only an integer enum gives for free.

```go
type RoundingMode uint8

const (
    RoundInvalid    RoundingMode = iota // zero value — never a valid argument
    RoundExact                          // error if the result is not exact
    RoundTowardZero                     // truncate magnitude
    RoundAwayFromZero
    RoundFloor                          // toward -inf
    RoundCeil                           // toward +inf
    RoundHalfUp                         // ties away from zero
    RoundHalfEven                       // banker's; unbiased
)
```

`RoundInvalid = 0` makes a forgotten argument an error rather than silently whichever mode is first —
the same reasoning that made `Money`'s zero value invalid, applied consistently. `RoundExact` lets
invariant tests assert "this must not lose anything", which is how a scale bug is caught rather than
absorbed. Intended uses: `Ceil`/`Floor` for tick quantization where the side dictates direction;
`AwayFromZero` for costs (a fee is never understated); `TowardZero` for lot quantization (never size
beyond the balance); `HalfEven` for valuation/PnL where bias accumulates.

---

## 5b. `fixedpoint` as built (session 2) — the package is complete bar the fuzz target

**Six commits, `c0f056a` → `ff80713`. 1,482 production lines, 213 passing test cases, `make ci`
green.** The whole of resume items 1–4 except the fuzz target.

### The four categories, and what each op costs the caller

| | zero value | tag(s) | ops |
|---|---|---|---|
| `Money` | invalid | `asset` | `Add` `Sub` `Neg` `Abs` `Cmp` `Sign` `Mul(Rate)` `Div(Money)→Rate` |
| `Quantity` | invalid | `asset` | same, plus `Mul(Rate)` |
| `Price` | invalid | `base`+`quote` | `Cmp` only |
| `Rate` | invalid via a `set bool` | none | `Add` `Sub` `Neg` `Abs` `Cmp` `Sign` `Mul(Rate)` |

Package functions: `Notional(Price, Quantity, mode) → Money` tagged with the price's *quote*;
`AveragePrice(Money, Quantity, mode) → Price` as its inverse. `Chain(Money)` starts a `Calc`.

**`Rate` carries an explicit `set bool`.** It is dimensionless, so it has no asset tag to hang
validity on, and the failure is worse here than anywhere: an unpopulated *fee* rate reads as
zero fees, and §12.2's verdict is a comparison against a cost hurdle that free trading clears.
Rejected: a valid zero (ergonomic, and precisely the failure). Rates are plain ratios, never
percentages — nothing in the package multiplies by 100, so that conversion stays at the venue
adapter as one reviewable line.

**`Price` rejects `base == quote` at construction.** A price of an asset in itself is
degenerate and in practice means an adapter filled both tags from one field. Negative prices
*are* permitted: calendar spreads and some settled contracts trade below zero, and screening
that is a venue's business.

**`Sign` is fallible on every category.** Zero is a legitimate answer, so an infallible `Sign`
could not distinguish "genuinely zero" from "nobody populated this", and a cost that reads as
zero reads as free. This is the one place the design pays real ergonomic cost for consistency.

**`ErrUnset` is the general sentinel**, wrapped by `ErrAssetUnset` and `ErrRateUnset`, so one
`errors.Is` covers "something was left unpopulated" across all four categories.

### The integer kernel — `mantissa.go`

Everything routes through `mulDivMantissa(a, b, den, mode)` over a full 128-bit `math/bits`
intermediate. A scaled multiply is `mulDivMantissa(a, b, scaleFactor)`; a division is
`mulDivMantissa(a, scaleFactor, b)`. One implementation of "what happens to the part that does
not fit", not one per operation. Three details that a shorter version gets wrong:

1. **The mode is validated before anything reads the data.** Checked only on the rounding path,
   an unset mode succeeds whenever the result happens to be exact — green in every test written
   with round numbers, failing later on real prices.
2. **`bits.Div64` panics when the quotient will not fit**, so the `hi >= den` guard comes first.
   A panic here lands on the `DispatchAuthorized` crash boundary (§5.3) — the one place this
   package must not put one. *Proven: removing the guard panics rather than failing.*
3. **Rounding decides on magnitudes with the sign carried separately**, and half-way comparison
   uses `rem` against `den/2` rather than `2*rem` — overflow-free, and it makes the
   odd-denominator case (where no exact tie exists) explicit rather than accidental.

### Error precedence — a stated contract **[SPEC EDIT LANDED — v0.9.5]**

Invalid mode → divide-by-zero → **overflow** → precision loss. When a result is both out of
range and inexact, **overflow wins**: a value that cannot be held at all is more fundamental
than one that cannot be held exactly, and reporting "precision loss" sends a caller to their
rounding policy when the number is orders of magnitude too big.

**This was found by writing the oracle, and it was a real defect.** `mulDivMantissa` reported
`ErrPrecisionLoss` for magnitudes in [2^63, 2^64) and `ErrOverflow` above 2^64 — the same
condition classified two ways depending on which side of an internal boundary it fell. Fixed by
range-checking the truncated quotient before consulting the mode. Pinned by the table case
"out of range and inexact at once" (`4294967297² / 2`).

### `Calc` — why a sticky error is acceptable here

`Chain(m).Sub(fee).Mul(rate, mode).Result()`. The usual objection to swallowing errors does not
apply because the error lives in a *local* calculation: `Result` is the only exit, it always
returns the error, and when there is one it returns an **invalid `Money`** alongside it — so a
caller who ignores the error holds something that fails `Validate` and fails every subsequent
operation. The **first** error survives, not the last: later failures are consequences of
operating on a value that was never produced. `Ratio` is terminal because its result leaves the
Money category. Zero allocations per chain, asserted.

### The persistence codec

`Mantissa()` / `XFromMantissa(...)` on all four. **Two columns, not one encoded string:** a
single column forces the `sql.Scanner`/`driver.Valuer` shape, which cannot carry an asset tag
alongside a number, so the only way to fit both would be text — giving up ordering and `SUM()`
in the database to save a column. The `int64` mantissa *is* SQLite `INTEGER` and *is* Postgres
`BIGINT`, which is the concrete content of D2's cheap migration path.

`Mantissa` is the one place a raw scaled integer leaves the package. Documented as a
persistence seam, **not** an arithmetic escape hatch — sums on values obtained that way discard
the asset check and the overflow rules. *A future `nofloat`-style analyzer could police this;
not built.*

---

## 5c. Test strategy as executed — what is actually load-bearing

**213 cases. Every construct below was bite-checked: a mutation was applied and the intended
test failed.** Nothing here is asserted without having been shown to bite.

| Construct | What it covers | Bite-checked |
|---|---|---|
| Hand-written tables | boundaries, ties, both signs, the 128-bit path | 21 mutations across 5 rounds |
| `rapid` × `math/big` oracle | 4 properties, 100 checks each by default | see below |
| **19 negative controls** | one known-wrong impl per plausible defect | each rejected, with its counterexample logged |
| **4 positive controls** | the real function survives its own predicate | required, see below |
| `TestCategoriesDoNotMix` | §3.6 rule 4's *compile-time* half | 3 mutations, both directions |
| `AllocsPerRun == 0` | 11 hot operations + the `Calc` chain | 1 mutation |

### The oracle is a different algorithm, not a paraphrase

It computes the exact result as a `big.Rat` at unbounded precision, then rounds by **naming the
two integer neighbours and comparing the distance to each** — which is the *definition* of
rounding, where the remainder-and-sign formulation the fast path uses is an optimisation of it.
It shares no helper with production code: not `magnitude`, not `signedFromMagnitude`, not
`roundingIncrement`. An oracle that restated `mulDivMantissa` would have agreed with it for the
same reasons it was wrong — and in fact the one defect found was found precisely because the
two disagreed about error precedence.

### Predicate/generator split — the thing that made negative controls possible

Each property is written as a **predicate** (`checkMulDiv(op, a, b, den, mode) string`) plus a
rapid generator that feeds it. The predicate can then be run against a deliberately broken
implementation over a *fixed* sweep — deterministic, since a control that only sometimes
catches its defect is not a control. Reusable pattern for `clock` and everything after.

**Positive controls are not optional.** A predicate that always objects looks exactly like a
perfect set of negative controls, and a typo in a fixture path makes every compile-failure case
"fail to compile" for the wrong reason. Both are the NC1–NC5 vacuity problem from `docs/formal/`
in a new costume.

The most eloquent single result: the *"routes through `float64`"* control is rejected at
`123456789.123456789`, returning `123456789123456784`. §3.6 rule 4 demonstrated rather than
asserted.

### `rapid`'s seed semantics — VERIFIED from module source, no longer a guess

Read from `pgregory.net/rapid@v1.3.0` (`engine.go`, `data.go`):

- **`-rapid.seed` defaults to 0, and rapid reads 0 as "choose a random one"** (`baseSeed()`
  hashes the clock when the flag is zero). The properties are therefore **non-deterministic out
  of the box** — the wrong default for a gate, since a green build would not be reproducible.
- `RAPID_SEED` / `RAPID_CHECKS` env vars mirror `-rapid.seed` / `-rapid.checks`.
- `-rapid.checks` defaults to 100; `testing.Short()` divides it by 5.
- On failure rapid writes a fail file *and* prints the concrete `-rapid.seed=N` to replay.
- `rapid.TB` is an exported interface, so a recording fake is possible — **not used**, because a
  failing run writes a fail file into the tree. The fixed-sweep controls are better anyway.

**Policy, now in the Makefile:** `make test` / `make ci` pin `RAPID_SEED=20260726`;
`make test-properties` deliberately leaves it random at 20,000 checks (the nightly explorer);
`make test-properties-deep` is the reproducible deep run. Fail files are gitignored — unlike a
fuzz corpus entry they are keyed to a rapid version and an internal bit stream and go stale, so
a real find belongs in the hand-written table as a named case.

### The compile-failure fixtures

`testdata/typesafety/` holds seven packages built by `go build` from a test. Six must **fail**,
each asserted against a substring naming *why* (so a fixture broken by an unrelated rename does
not read as a pass); the seventh is the positive control that must build. This is the only test
of §3.6 rule 4's compile-time half, which was otherwise an untested claim in a doc comment —
and an untested claim about a type system is exactly the kind that survives a refactor that
quietly turns a category into an alias.

---

## 5d. The parser fuzz target (session 3) — `fixedpoint` is now complete

`parseMantissa` is the package's **only untrusted-input boundary**: every other entry point takes
an `int64` or another category, already constrained by the type system, while this one takes bytes
off a venue wire. Committed in `04bb5b9`.

### The body asserts four total invariants, not the absence of panics

A fuzz target that only checks for crashes is testing the wrong failure mode — this parser's
likely defect is *returning a plausible number* or the wrong error class. So the body is four
invariants that hold for every input, well-formed or not:

| | Invariant | The defect only it catches |
|---|---|---|
| **I1** | `ErrSyntax` **exactly when** the input is outside the grammar | prefix-consumption, i.e. a missing `i != len(s)`; and precision loss misfiled as bad syntax |
| **I2** | every error matches **exactly one** sentinel class, and a rejected input returns no value | classification collapsed into one opaque error; a truncated value handed back alongside `ErrPrecisionLoss` |
| **I3** | an accepted value survives the canonical round trip, and the canonical text is a **fixed point** of it | the scale applied once too few times |
| **I4** | agreement with the `math/big` oracle where the oracle is defined | **the exponent ignored** — self-consistent, correctly classified, right grammar; nothing else finds it |

**A regexp is the right second implementation of the grammar** (`decimalGrammar`), and for a
specific reason: the defects a hand-written index-advancing walker has — consuming a prefix and
not noticing the tail, accepting an empty exponent, letting a second point through — are *not
defects a regexp can have*. I1 is a **biconditional**, and the reverse direction is the
load-bearing one: well-formed text misreported as `ErrSyntax` sends a caller hunting a protocol
bug that does not exist.

### I4's gate — three documented divergences, not a suppression

The oracle is a second implementation of the **arithmetic**, not of the grammar and not of the
limits, and the fuzzer finds every place those diverge within seconds. So the cross-check runs
only inside *this package's* grammar with the exponent capped at four digits:

- **Grammar.** `big.Rat` rejects `"1."` and `".5"` (accepted here) and accepts `"1/2"`, hex
  floats and digit separators (rejected here). A disagreement about which characters spell a
  number is not a disagreement about what the number is.
- **Limits — verified from source.** `Rat.SetString` refuses `|exp| > 1e6` outright
  (`ratconv.go:181`, *"avoid excessively large exponents"*) and reports it as a **parse failure**.
  That is not what this package means by `ErrSyntax`: `"1e2000000"` is well-formed text naming an
  out-of-range value. Had this not been read from source first, the fuzzer's first "find" would
  have been an artefact of the two tools' differing cutoffs.
- **Cost.** `Rat` materialises `5^exp` to do this. The four-digit cap keeps it cheap **while
  still exercising `"1e9999"` and `"1e-7000"`** — i.e. straddling this package's own
  `maxExponent = 1000` cutoff, where an off-by-direction in the `expNeg` branch would live. Had
  the gate been set at `maxExponent` instead, that agreement would never have been checked.

### The reach was measured, not assumed

30.4M executions over 90s produced no find — which on its own is equally consistent with "nothing
was checked". So the generated corpus was counted: **162 of 348 entries open the I4 gate, and the
oracle has an opinion on all 162.** 178 are in-grammar; 31 parse successfully. The cross-check is
reached, not bypassed.

This is the same discipline as §5c's positive controls, applied to a fuzzer: a clean run and a
vacuous run are indistinguishable from the outside, so the coverage claim has to be a measurement.

### Six negative controls, each aimed at one invariant

Written as a predicate over a `parseFunc` — §5c's split, reused exactly as §5c predicted it would
be. `go test -fuzz` **cannot** demonstrate that its own assertions bite; a target whose
invariants are all vacuously true reports the same clean run as one that works. Each control is
caught by the invariant it targets, so a control that stops being caught names which invariant
went vacuous.

### Harness

`make fuzz` (bounded, `FUZZTIME ?= 60s`, one package because `-fuzz` takes one) is the
**explorer**; the seed corpus runs as an ordinary table under `make test`, so it is also a
committed regression suite the CI gate executes. Same division as `test` / `test-properties`.
`testdata/fuzz/` stays un-gitignored: Go writes there only on a **failing** input, so anything
appearing there is a regression case to commit — the opposite of rapid's fail files, which go
stale and are ignored.

---

## 5e. `platform/clock` as built (session 3) — the second kernel package

Commits `e851f79` (ServerTime) and `f146855` (the port, both implementations, the
conformance suite). The §5-family numbering is "packages built", so this sits here rather than
under §3.

### `ServerTime` is a type, and `Passed()` is why

§3.5 declares `ServerNow(Venue) ServerTime` — a *type*, not `(time.Time, time.Duration)` — and that
is the opportunity. A funding boundary decided against a band has **three** answers, so:

```go
func (s ServerTime) Passed(boundary time.Time) Certainty   // CertainlyPassed | CertainlyNotPassed | Uncertain
```

The rejected alternative is handing back the instant and the band and letting each call site
compare them. Every call site would do that arithmetic slightly differently, and **the one that
forgets the band answers "unknown" as either yes or no** — in the code path deciding whether to hold
a position through a settlement.

**The boundary IS the settlement instant**, so being exactly on it counts as passed. That
inclusivity is load-bearing in two directions and both are pinned: `Earliest() == boundary` is
`CertainlyPassed`, `Latest() == boundary` is only `Uncertain`.

`Certainty` follows §3.6 rule 8's `type X string` + constants + `Validate()` shape, so **no new
exception is owed** there (unlike `RoundingMode`).

### An uncalibrated venue is unusable, and that is the sharpest decision here

`ServerNow` on an unknown venue returns the **zero `ServerTime`**, which fails `Validate()` and
answers `Uncertain` to everything. The alternative — a zero offset and a zero band — is *the most
confident answer this package can give* ("that venue's clock is exactly ours"), and it would be
given on the strength of no data at all. Every funding boundary would then read as decided.

`ServerNow` has no error to return (§3.5 fixes the signature), so **`Validate()` is the loud path
and `Uncertain` is the safe default**: a caller who skips the check stalls rather than acting on a
clock it does not have.

### Timer delivery is pull-based **[SPEC EDIT LANDED — v0.9.5, an addition]**

The obvious design is a sink the Clock pushes into as timers come due. It is wrong here: §3.5 gives
the **dispatcher** one total order over events *and* timers, so a Clock pushing on its own goroutine
would force the dispatcher to re-queue everything it was handed, and **the arbitration that decides
the replay key would live in two places**. So consumption is pull-based, on an interface separate
from `Clock` because only the dispatcher pumps it while every domain component holds a `Clock`:

```go
type TimerSource interface { NextDue() (Fired, bool); Pending() int }
```

Side effect worth having: **this package contains no goroutines and no delivery-path concurrency at
all.** `Fired.At` is the *scheduled* instant, not the delivery instant — the scheduled one is
identical in a replay.

**Deliberately absent: waking a live consumer.** That belongs to the dispatcher's select loop, which
§1 defers (it needs `platform/events` and a persisted `DispatchSeq`). Building it now would be
guessing at a shape with nothing to check it against.

### The ordering rule, and the priority direction **[SPEC EDIT LANDED — v0.9.5]**

One `timerQueue`, shared by both implementations, so "earliest by `(At, Prio, ID)`" has one
definition. Sharing is deliberate: a suite passing against two independent comparators would only
prove they agree today.

**Lower `Prio` is more urgent, so the zero value is the MOST urgent.** Chosen rather than inherited:
under the opposite convention an unset priority sends a safety timer to the back of the queue, and
noisy-but-early beats quiet-but-late. The id tie-break makes the order **total**, which is what lets
a replay reproduce a drain sequence.

A map and a linear scan, **not a heap**: cancel-and-replace by id is the operation §3.5 asks for
most and it is exactly what costs a heap an index per entry, while the queue holds a handful of
funding-boundary and order-timeout timers. Revisit if `Pending()` is ever seen in the thousands.

### Clamping vs. erroring — the same question answered twice, differently

| | Backwards time | Why |
|---|---|---|
| `Wall.Now` | **absorbed** (high-water clamp) | No error to return (§3.5). A stalled clock fires a timer late; a reversed one re-evaluates a settlement |
| `TestClock.Advance` / `SetNow` | **rejected** | They *do* have an error to return, and in a replay backwards time means the dispatch log is out of order — a finding, not something to absorb |

The clamp is not hypothetical. NTP steps the system clock, and Go's monotonic reading protects the
*difference* between two `time.Time`s while `Now` hands out absolute instants compared against
deadlines computed earlier.

### `AdvanceToNextTimer` is the backtester's loop primitive

Jumping straight to the next scheduled instant is what makes replaying a month of 8-hour funding
cycles finish in seconds rather than iterating a nominal tick — a caller guessing a step size would
either skip timers or waste most of its iterations. An already-due timer does not rewind the clock
and still reports true, so an "advance then drain" loop makes progress instead of spinning.

### Bite-checked: 18/18 mutations, 11 conformance negative controls — and 3 of 4 findings were in the TESTS

This is the part worth carrying forward. The conformance suite is the easiest kind of test to write
vacuously: nine invariants passing against both implementations look like nine guarantees and look
identical to nine that could never fail.

1. **The first controls run HUNG rather than failed.** The "a consumed timer stays due" defect made
   `drain()` loop forever. **A predicate that can hang on a broken implementation is a bad
   predicate** — a failing test names the defect, a hanging one reports nothing, blocks CI, and
   reads as an infrastructure problem. `drain()` is now bounded with a legible overflow marker.
2. **`checkNowMonotonic` has two clauses and one defect only reached one.** A `Now` that retreats
   exactly as fast as time advances reads as *stalled*, not *reversed*, so the "went backwards"
   clause had never fired. Split into `nowGoesBackwards` and `nowStandsStill` so both are live —
   the vacuity problem found one level up, inside a control.
3. **`Wall`'s monotonic clamp survived having its store removed.** `clampMonotonic` had its own
   tests and the conformance suite passed, but the *wiring* was untested: without the store, the
   high-water mark stays the zero time and the clamp silently never engages. Closed with an
   unexported `systemNow` an internal test can replace — the public surface is unchanged — and the
   mutation now bites. **A guarantee whose logic is tested and whose wiring is not is still an
   untested guarantee.**
4. The positive control does double duty: the same broken-clock wrapper with **no** defect selected
   must pass every invariant, which shows the harness is not what causes the failures.

---

## 6. Test strategy

**Test-first per operation, red shown before green** (the `tdd` skill governs the loop). Rejected:
batched per-package red/green (individual failures less visible); spec-doc-first (the user chose to go
straight to the loop).

**In, and load-bearing:** table-driven units for boundaries · **`rapid` PBT against an independently
written `math/big` oracle** · **one negative control per property** · native `go test -fuzz` on the
parser · `testing.AllocsPerRun` == 0 assertions on hot ops · a **contract/conformance suite run against
both `Clock` implementations** · `analysistest` fixtures per analyzer.

**`rapid` is pre-authorized** by the spec's own formal-methods block: *"prefer `rapid` property tests +
the deterministic-sim harness the parity `Clock`/dispatcher enables + TLC trace-checking against the
real OMS"*. It is test-only, so §11's "never a dependency" rule (which governs production code) is
untouched.

**The oracle must be written as the obviously-correct algorithm** — exact arbitrary-precision, no shared
helpers with the `math/bits` path. A paraphrase of the fast path would be a tautological test, which the
`tdd` skill explicitly warns against.

**Testing the tests: negative controls in CI, `gremlins` as a timeboxed audit only.** Every property
gets a paired known-wrong implementation that MUST fail it. This is the discipline `docs/formal/`
already paid for — NC1–NC5 exist because *"a passing liveness property and a vacuous one look
identical"*. `gremlins`/`go-mutesting` are not maintained well enough to sit behind a CI gate; the
timeboxed attempt is **RUN as of session 5 — see §6b**, and the decision to keep it out of CI is now
supported by measurement rather than by reputation. (Note §0: this is a place where Rust's
`cargo-mutants` would have won outright.)

---

## 6b. The `gremlins` audit (session 5) — RUN, and it earned its keep

The debt §6 carried since session 1. Reported honestly either way, as promised — and the honest report
is *better* than the prediction in one direction and *worse* in another.

### It runs on Go 1.26. The §0 premise needs one word changed

`go install github.com/go-gremlins/gremlins/cmd/gremlins@latest` builds clean on **Go 1.26
darwin/arm64** and the binary self-reports `version dev`. §0's claim was that `gremlins` is *not
maintained*, which is true of the project; the inference nearby — that an unmaintained Go tool would
not survive the version gap — did **not** hold. Worth separating, because "it doesn't run" is a reason
to stop and "it runs but misreports" is a reason to be careful, and only the second one is true.

**Re-verified across the toolchain bump.** The audit was first run on **1.26.0** and re-run in full on
**1.26.5** after the upgrade later in the session: `gremlins` rebuilt from source without complaint,
and both modules produced **identical counts and a byte-identical survivor set** (root 256/24,
`tools/lint` 27/7). So none of what follows is an artefact of one patch release.

**It is still not in CI, and now for measured reasons rather than reputational ones** — see the misreport
modes below, particularly the last. It touched neither `go.mod`: installed to a scratch `GOBIN`, run
against a *copy* of the tree, because `gremlins` mutates source files in place and reverts them, and a
tool that edits your working tree gets a throwaway working tree.

### Four ways it misreports, and one of them is silent and total

1. **The per-mutant timeout is derived from a warm-cache baseline.** The first run reported
   `Killed: 0, Lived: 0, Timed out: 274` and **`Test efficacy: 0.00%`**. Nothing was wrong with the
   suite. Go's test cache made `gremlins`' own baseline measurement `100.87ms`, every real test run
   overran the budget derived from it, and *every mutant* came back a timeout. **`go clean -testcache`
   moved the baseline to `2.86s` and the identical tree scored 92.51%.** This is the failure mode the
   record keeps meeting: a broken harness and a catastrophic result print the same number. Standing
   fix: clean the test cache first, and pass `--timeout-coefficient 12`, which took the remaining
   7 timeouts to 0 — two of which turned out to be real survivors, including the important one below.
2. **`./...` silently finds nothing.** `gremlins unleash ./... ` prints *"No results to report"* and
   exits 0 — indistinguishable from a clean tree. `.` and a concrete package path both work. A path
   form that reads as "everything" and means "nothing" is worth writing down once.
3. **A non-viable mutant is scored as a kill.** `ARITHMETIC_BASE` is applied to string `+`
   concatenation — `"Certainty(" + string(c) + ")"` gets mutated to `-`, which cannot compile. The
   build fails, `go test` exits non-zero, and `gremlins` records **KILLED**. The `Not viable` counter
   read 0 in every run this session. **So every efficacy number here is an upper bound**, inflated by
   an unknown count of mutants no test could have caught because no compiler would accept them.
4. **Verdicts are not reproducible.** Four runs over one unchanged tree: three agreed exactly
   (`Killed: 256, Lived: 24`) and the fourth reported one extra survivor,
   `CONDITIONALS_NEGATION at clock/timers.go:116:12`. Applying that same mutation by hand and running
   the clock suite twenty times: **caught 20/20**. So `gremlins` produced a **false LIVED**, roughly
   once in four runs, on a mutant the tests kill deterministically. `--workers 1` did not change it,
   so it is not simple worker contention. **A false LIVED is the expensive direction of wrong** — it
   sends you hunting a gap that does not exist, and it means no `LIVED` from this tool can be believed
   until it is reproduced by hand. Every finding below was.

### The structural blind spot: bare-`switch` case expressions

20 mutants in the engine sit in `NOT COVERED` permanently, and 18 of them are on `case` expressions of
bare switches — `signMantissa`, `cmpMantissa`, `roundingIncrement`, and `parseMantissa`'s shift
classifier. Those functions are 100% / 95.7% / 98.6% statement-covered. Go's cover tool counts case
*bodies*, so a mutant on a case *expression* maps to no coverage block and `gremlins` refuses to run
it. (The other 2 are on `const minInt64 = -1 << 63`, which is not executable at all.)

**This is exactly the money kernel's three-way comparison logic** — the code whose whole job is that
`Cmp` and `Sign` agree by construction. **The mutation audit does not reach it, so the hand-written
tables and negative controls remain the only thing testing it.** Do not read a good efficacy number as
covering that ground.

Related, and cheaper: `gremlins` walks the filesystem, not the module graph, so `tools/lint`'s 36
mutants show as `NOT COVERED` from the root module. The nested module needs its **own run** from
`tools/lint/`.

### What it found — four items, none of which the 46 hand-applied mutations had reached

The prediction in §6 was that the hand-written controls already covered most of this and the residual
value was in *unanticipated* survivors. That was right, and the residual value was real.

**1. The `bits.Div64` panic guard was untested at its boundary — the significant one.**
`mulDivMantissa` guards `hi >= uden` before `bits.Div64`, because `Div64` *panics* when the quotient
will not fit in 64 bits. Loosening it to `hi > uden` survived the entire suite: every existing test
clears that guard by a wide margin, so nothing exercised the equality. Constructing the separating
input — `a = 2^32`, `b = scaleFactor·2^32`, making the 128-bit product exactly `scaleFactor·2^64`, so
`hi == uden` and `lo == 0` — the mutant panics with `runtime error: integer overflow` inside
`math/bits.Div64`.

That is precisely the outcome this package may not have. Its own comment says so: a panic here lands on
the **`DispatchAuthorized` crash boundary (§5.3)** the formal model is built around, which is why every
arithmetic failure in the kernel is converted to an error. **A guard that converts a panic into an error
is worth nothing until something proves it is still standing at the value where it starts to matter.**
Bite-checked in both directions: mutant applied + new test → fails; mutant applied + new test removed →
`ok`. The test also asserts the product really lands on the equality, so a change to `Scale` breaks it
loudly instead of quietly sliding off the boundary.

**2. `Quantity.Mul` had 0.0% coverage** — an exported money-kernel method with no test at all,
confirmed against `go tool cover -func`, not inferred from the mutation output. The way it was missed
is the lesson: `quantity_test.go` enumerates operations in two tables, *binary* (taking a `Quantity`)
and *unary* (taking nothing). `Mul` is the only `Quantity` method with a third shape,
`(Rate, RoundingMode)`, so it fell between two tables that each looked exhaustive from the inside.
**A table driven by signature shape is a coverage claim about the shapes it names, not about the method
set.** Now 100%, 17 cases, and 4/4 mutations caught — after the first bite-check attempt itself
misfired, see below.

**3. `Certainty.String` had 0.0% coverage** (`clock/servertime.go`), and the missing branch is the one
that matters: an invalid `Certainty` reaching a log line prints as `Certainty(maybe)` rather than as a
bare `maybe` that would read as a decision someone made. Both branches asserted together, because a
`String` that wrapped *everything* would pass a test that only checked the invalid side.

**4. Analyzer diagnostic ordering was unasserted, in both gates.** `sortedLines`/`sortedEscapes` exist
so that two runs over an unchanged tree emit byte-identical output. Reversing either comparator left the
whole analyzer suite green — because `analysistest` matches diagnostics against `// want` comments
attached to source lines and is **order-insensitive by construction**. This is §5e's lesson with the
blind spot moved: not untested wiring, but a property the harness structurally cannot see. Direct unit
tests on the comparators now, in both analyzers; the duplication is deliberate and matches the standing
decision to extract these two analyzers' common parts at the **third** gate, not the second.

### The survivors are the other half of the result — 24 in the engine, 7 in the gates

Every one was triaged, and the great majority are **provably equivalent mutants**, which is a good sign
rather than a gap. The two overflow guards account for 10 of them:

```go
if (a > 0 && b > 0 && s < 0) || (a < 0 && b < 0 && s >= 0)   // addMantissa
```

Loosening *any* of those five comparisons cannot change the verdict — admitting `a == 0` makes `s == b`,
which fails `s < 0`; `s == 0` is unreachable because `2·(2^63−1) < 2^64`. That is the argument the
function's own doc comment makes, now confirmed mechanically. `subMantissa` is the same shape.
Likewise `magnitude`'s `v < 0` (at zero, `uint64(-(0+1))+1 == 0`, so both branches agree), the `Abs`
zero-checks on all three categories (`negMantissa(0) == 0`), and `parse.go`'s exponent classifier at
`v == maxExponent`, where the shortcut and the full path return the same error class either way.

**One accepted survivor is worth naming because it is not equivalent, just unobservable.** Mutating
`len(digits)+shift > maxMantissaDigits` to `-` — the guard that checks width before materializing zeros
— survives. Measured rather than argued: across 14 exponent-heavy inputs the values and errors are
**identical**, and the only difference is bytes allocated on a path that errors either way (a flat 3080
bytes clean; 5912 at `e1000` mutated). The reason is that `maxExponent = 1000` already caps the
allocation at about a kilobyte, so the guard's stated motivation — "a gigabyte" — is not reachable
today. **The comment now says that**, and the guard stays, because it is what keeps the claim true if
`maxExponent` is ever raised. Pinning it would need a byte-level memory assertion, which is fragile
under `-race` and not worth it.

In the gates, the 4 reversed-comparator mutants are now killed; the surviving `<` → `<=` ones are
equivalent while positions are distinct, and a `token.Pos` identifies one place in one file. The
remaining `notimenow:271` negation is the same-line dedup tie-break, which needs two findings on one
source line *and* the ability to observe which column got reported — `analysistest` matches by line, not
column, so it cannot be reached from the current harness. Accepted and named, not overlooked.

### Scores, before and after

| | Killed | Lived | Not covered | Efficacy |
|---|---|---|---|---|
| root module, before | 249 | 25 | 62 | 90.88% |
| root module, after | **256** | **24** | **56** | **91.43%** |
| `tools/lint`, before | 23 | 11 | 2 | 67.65% |
| `tools/lint`, after | **27** | **7** | 2 | **79.41%** |

Read those percentages with misreport mode 3 in mind: they are upper bounds. The `Lived` column moving
25 → 24 understates the change, because the six newly-covered mutants moved *into* the runnable pool at
the same time; the mutant that matters, `mantissa.go:132:8`, went from LIVED to KILLED.

**Reproducing it:** install to a scratch `GOBIN`, copy the tree, then per module —
`go clean -testcache && gremlins unleash -E 'testdata' --timeout-coefficient 12 .` — once from the root
and once from `tools/lint/`. Never `./...`.

### Two lessons, both about harnesses rather than about code

**A mutation harness must prove the *intended* change, not *a* change.** §4d's script already `cmp`s
before and after and prints `NO-OP!!` when nothing moved. That is not enough. The first bite-check of
`Quantity.Mul` reported `drop the asset tag: SURVIVED` — alarming, and false: the pattern
`return Quantity{q: v, asset: q.asset}, nil` occurs **three times** in `quantity.go`, and the
substitution had landed on line 113 (`Neg`) instead of line 163 (`Mul`). The file changed, `cmp` was
satisfied, and the wrong function was mutated. Line-scoping it, the test caught it immediately.
**`cmp` proves the file moved; only the diff proves what moved.**

**A fixture can quietly agree with the thing it claims to rule out.** The first version of the analyzer
sort test asserted, in a comment, that a comparator reaching for the wrong field could not pass it —
and then a mutant sorting by `keys[i].num` **survived**, because the fixture's line numbers happened to
run in the same order as its positions. The comment stated the property; the data did not. Rebuilt with
line number running exactly *backwards* against position and file name scrambled against both: 8 of 10
mutants caught, and the 2 survivors were predicted as equivalent *before* the run rather than explained
after it. **A claim written in a test comment is not tested by the test unless the data can falsify it.**

---

## 7. Architecture audit — what was checked and what is still open

Assessed against idiomatic Go, "standard Go project structure", DDD (strategic + tactical) and
ports-and-adapters. Findings:

**Sound:** there is no official Go project layout (`golang-standards/project-layout` is community-made
and not Go-team endorsed); what *is* official (go.dev/doc/modules/layout) is `cmd/<binary>/` +
`internal/`, and §11 matches it. `internal/` is compiler-enforced, so `platform/`'s isolation is
structural rather than aspirational. Strategic DDD is genuinely present: bounded contexts, ACL
translation (§3.2 — *"each context wraps the capability it needs into its own port"*, and `MarketView`
being strategy-owned rather than `engine/state`'s type), shared kernel, aggregate consistency boundary
(§5.1, lint-enforced). Tactical DDD is rigorous (value objects via rules 4 + 8). **And §3.2 exempting
`engine/` from DDD layering is the decision that keeps this from becoming ceremony** — a WebSocket
client has no domain model.

**`application/ports/` survives the Go-idiom objection**, because `application/` *is* the consumer, so
the interface is still declared where it is used; and §4 keeps ports segregated (`ExecutionVenue` is a
composition of four narrow interfaces, not a god-interface). **The risk to watch in review is fat ports
— anything past ~5 methods — not the directory.**

**Open / deferred:**
- **`Core` structs and `<Ctx>Service` interfaces** are bitnet house style, not Go style (`oms.Core`
  tells a reader nothing). The `<Ctx>Service` *inbound* interface is justified by §3.7's service-split
  seam. Revisit the naming when `oms` lands in milestone 3, not before.
- **The shared kernel has 11 packages**, and a shared kernel is DDD's most coupled pattern. The bright
  line — `platform/` holds *technical* primitives only — is now mechanically enforced by
  `kernelpurity` (§4).
- **Ubiquitous-language glossary does not exist.** `docs/guide/i18n/glossary.md` is for *translating
  trading terms*, not for pinning code-facing domain terms. Deferred deliberately (see §1).

---

## 8. The second slice (session 6) — `platform/apperror` and `platform/ids`

Both packages sit at the bottom of §1's dependency spine. `apperror` genuinely imports nothing;
`ids` imports nothing *from the module*, which is not the same claim and is discussed below.

### 8a. `apperror` — two axes, because there are two questions

The spec gives this package four words (§5.1: *"Error taxonomy (`errclass`)"*). Its requirements are
not there — they are scattered across §5.3, §7.6 and §10, and they are load-bearing.

**Decision: `Class` and `Effect` are separate enums.** `Class` answers *what went wrong* and drives
backoff (§7.6), alerting and the error/rate-limit metric label (§10). `Effect` answers *did the side
effect happen* and drives the order lifecycle (§5.3): `EffectUnknown` is what puts an order into
`SubmissionUnknown` and sends recovery to `OrderByClientID` rather than to a resubmission.

**The argument that settled it.** A 429 and a socket read timeout are both HTTP failures reaching a
venue, and they differ absolutely on the safety axis. A connection refused and a read timeout share
the cause axis *exactly* and still differ on the safety axis. So neither axis determines the other,
and collapsing them makes the safety-critical fact something the OMS derives from the cause — which
is exactly the inference §5.3 forbids when it says "not sent" is never inferred.

**Rejected alternatives, both offered to the user:**

- **One flat `Class` enum** with ambiguity encoded in the constant names (`ClassTransportUnknown`,
  `ClassTransportNotSent`, …). Simplest, one metric label, one switch. Rejected because the safety
  property ends up spread across constant names, so a class added later can silently omit it, and
  the cross-product shows up as near-duplicate constants.
- **`Class` + an `Ambiguous bool`.** Middle ground: explicit, cannot be forgotten by a new `Class`.
  Rejected because it collapses "applied and still errored" into the same bucket as "proved not
  applied", and because the zero value of a bool is the *unsafe* reading.

**Shape decisions inside it, each with a reason:**

- **`Class` takes §3.6 rule 8's `type X string` shape; `Effect` takes `RoundingMode`'s integer-enum
  exception.** Rule 8's shape is for enums crossing a persistence or wire boundary, and `Class` does
  — §10 makes it a metric label and the audit projection stores it. An `Effect` is an argument:
  never persisted, never transmitted.
- **`EffectUnspecified` is a sentinel that fails `Validate` and is SAFE to encounter.** This is the
  difference between it and `RoundInvalid`. `mayHaveApplied` reads it as "yes, it may have", so a
  spec built with the field forgotten degrades to the conservative reading. *Loud where it can be,
  safe where it cannot* — the same division `ServerTime.Passed` takes for an uncalibrated venue.
- **`mayHaveApplied` is written as a negation** — everything except `EffectNotApplied`, the one value
  that is a claim about evidence. `Effect` is a defined `uint8`, so `Effect(9)` compiles; a negation
  puts every undeclared value on the safe side rather than off the end of a list.
- **`New(Spec)` never returns an error.** A constructor called on a failure path must not add a
  failure path of its own — the caller is already holding something that went wrong. An invalid Spec
  degrades to `ClassInternal`/`EffectUnknown` (safe), records what was wrong with it (loud), and
  answers `Malformed()` so a test or boot check can assert a code path produces none.
- **`Spec` is a struct, not a positional argument list.** The natural signature
  `New(class, effect, op, detail string)` puts two same-typed strings side by side on a failure path.

**The non-laundering rule — the sharpest thing in the package.** `EffectOf` reads the *outermost*
classification in a chain, because that is the one nearest the caller's layer. On its own that rule
has a hole: an application layer wrapping an ambiguous transport failure could restate it as certain,
turning "the order might be live" into "there is no order" through nothing but an error message.
`resolveEffect` closes it with one sentence — **agreement is preserved, disagreement collapses to
`EffectUnknown`** — so certainty can never be manufactured by re-wrapping. A caller with genuinely
new evidence (a resolving query that proved the order absent) constructs fresh rather than wrapping.

**One real bug, found by building it rather than by review.** `New` first read the cause's effect
with `EffectOf`, whose unclassified-input default is `EffectUnknown`. That made every wrap of a plain
`errors.New` look like a disagreement, and `EffectNotApplied` became **unreachable in practice**. The
taxonomy would still have been *safe* — it would have reported everything as ambiguous — which is
precisely why it would have shipped. `recordedEffect` is the same chain walk with the opposite
default, and the two are documented against each other so the next reader does not "simplify" them
back together.

**Bite-checked: 14 mutations, 14 caught.** One survived the first pass and the diagnosis is worth
keeping: **renaming the `"not_found"` wire token survived**, because the `classes` slice names the
*constants*, not the literals, so a rename moves the declaration and the lookup table together and
the completeness test still saw them agree. It is a real defect — `Class` crosses a wire and
persistence boundary, so a renamed token orphans every dashboard query and splits a stored series in
two, with nothing failing. Fixed with a frozen-token test. **`Effect` deliberately gets no
equivalent**, and the asymmetry is the point: it is never persisted, so its strings are debug output
rather than a contract.

### 8b. `ids` — the three decisions the user took

**1. `ClientOrderID` is structured and REVERSIBLE, not hashed.**

Format, fixed at 35 characters:

```
ab-<26 Crockford base32 of the TradeID>-<leg, 2 digits>-<attempt, 2 digits>
ab-01FWHE4YDGFK1SHH6W1G60EECF-01-00
```

§5.3 makes the id a **recovery correlation handle, not a venue idempotency key** — after a crash the
OMS re-derives it and asks the venue what happened, which is the only resolution §5.3 permits for an
order at `DispatchAuthorized` or beyond. Carrying the TradeID rather than a digest means an operator
reading a venue's order page recovers the trade **without database access**, and §11 says the journal
is host-local and lost on host death: the incident where that matters is the one where the lookup
table is gone. Uniqueness holds by construction rather than by a birthday-bound argument.

*Rejected:* **hashed and opaque** (truncated SHA-256 → base32) — fits any future length budget and
leaks no structure, but correlation needs the journal, uniqueness rests on a collision argument, and
a truncated hash cannot be widened later without changing every derived id. **Counter-based**
(`ab-<short trade>-<seq>`) — shortest and most readable, but the counter is state, so derivation
stops being a pure function and crash re-derivation depends on the journal surviving.

Sub-decisions: **Crockford base32 rather than base64** because these ids are read aloud and re-typed
during incidents and Crockford excludes the confusable `I`/`L`/`O`; **most-significant-first**
because that makes the text sort by trade age on a venue's order page with nothing parsing it;
**strict decoding** (canonical uppercase only, no lowercase, no alias folding) because Crockford's own
leniency would make two strings denote one id, and this id's whole job is exact correlation against
what the venue echoes back; **two-digit fixed-width leg and attempt** because fixed length makes a
truncation a length check rather than a parse that happens to succeed.

**2. `Venue` moved from `platform/clock`, retiring the TODO at `clock.go:43`.** It is an identity, and
`config`, `ratelimit`, `oms` and `engine/venue` will all need to name a venue without wanting a clock.
Moved **as a type alias** (`type Venue = ids.Venue`), so every existing call site keeps compiling and
the two names are one type rather than two that convert. `clock.ErrVenueUnset` was **removed** rather
than kept beside `ids.ErrVenueUnset` — two errors for one condition is two mechanisms that can
disagree. *Rejected:* leaving it in `clock`, which costs no churn now and makes every future package
import a clock for a string type.

**3. Who mints what is explicit in the types.** `TradeID` and `IntentID` are ours — minted as v7,
wrapping an unexported UUID so the only ways to hold one are minting and parsing, with the Nil UUID
as an unusable zero value. `ExecutionID` is the venue's: **received, never minted**, an opaque string
with no structure we may assume. It carries §5.3's caveat that it is unique **only within an
instrument**, which is why the type has **no `Equal` method** — any such helper would compare the bare
value, and the bare value is not an identity. Comparison happens in the repository against the full
five-column key.

### 8c. `ids` cannot import nothing — and does anyway

The session-5 resume block predicted `apperror` and `ids` would "both import nothing". For `ids` that
is wrong as stated: a UUIDv7 embeds a 48-bit millisecond timestamp and 74 random bits, and **§3.5's
parity requires both to be injected** or a replay mints different ids from the same input and stops
being a replay.

It nonetheless imports nothing from the module, because the time seam is a **one-method interface
declared here** (`TimeSource`) rather than `clock.Clock`. Go's interfaces are structural, so
`clock.Wall` and `clock.TestClock` satisfy it with no import edge. That is not tidiness — **`clock`
imports `ids` for `Venue`, so an import in the other direction would be a cycle.**

Worth recording explicitly: **§3.6 rule 5 does not reach `platform/`.** No gate would have caught an
ambient `time.Now()` here. The injection is a design decision, not compliance.

### 8d. The generator's state machine

`rand_a`'s 12 bits hold a **monotonic counter**, not random bits. RFC 9562 method 1 permits it, and
determinism requires it — a random seed would be entropy the backtester has to reproduce for no
benefit, since unguessability is `rand_b`'s 62 bits' job. Three branches, each a decision:

- **New millisecond → counter resets to 0.** Per-millisecond state, not global.
- **Counter exhausted (4096 in one millisecond) → borrow the next millisecond.** Wrapping would
  reissue an id already minted; stalling would block a caller on a clock it does not control.
  Borrowing runs ids marginally ahead of the clock, bounded and self-correcting.
- **Backwards clock → clamp, do not error.** The same choice `clock.Wall` makes and for the same
  reason. Note the asymmetry against §5e: `Wall` clamps because it has *no error channel*, while this
  one *has* one and still does not use it — a mint that failed because the host stepped its clock
  would abort a trade for a reason with nothing to do with trading.

**Failure ordering is load-bearing.** Everything that can fail — reading the clock, validating its
range, drawing entropy — happens *before* any state is mutated, so a failed mint leaves the generator
exactly as it was. The test asserts the strong form: after a failed mint the next id is the one that
would have been minted had the failure never happened.

**Determinism is claimed precisely, and its limit is stated rather than prevented.** Identical clock
and entropy give an identical sequence *for a single-goroutine caller*, which is what §3.4's
dispatcher provides. Concurrent use is safe (the mutex guarantees uniqueness) but not deterministic,
because the entropy reader is drawn outside the lock. Stated in the doc comment because the constraint
belongs to the caller's architecture and this package cannot check it.

### 8e. Written fresh, and the tests are what pays for it

§11 forbids `github.com/google/uuid` (`platform/` is "reimplemented fresh; never a dependency"). That
is a genuine cost — a bit layout is what a library gets right once. It is paid back by testing against
inputs whose right answers were known **before this code existed**:

- The parser is checked against **RFC 9562 Appendix A.6's worked example**, including the decimal
  instant the RFC prints beside it. A round-trip test would prove only that the encoder and decoder
  agree with each other, which they would if both were wrong in the same direction.
- The base32 codec is checked against an **independently written `math/big` oracle** — repeated
  divmod by 32, sharing no code path with the implementation's bit-position loop — over generated
  input, plus boundary values and an order-preservation property.

**The oracle earned its place immediately.** The golden `ClientOrderID` in the test was first written
as a hand-encoding, and it was **wrong**. Had the fix been "copy whatever the implementation prints",
the golden test would have become an assertion that the code equals itself.

### 8f. Bite-checked: 25 mutations, 24 caught, 1 semantically equivalent

Two survived the first pass. Both were real test gaps and both diagnoses generalise:

1. **Deleting the counter reset survived.** Ids stayed unique and stayed ordered — which is everything
   the other tests checked. What breaks is quieter: after 4095 mints the generator has no counter room
   in any millisecond, so every id borrows and the timestamp **drifts permanently ahead of the clock**,
   and the 48 bits that make a v7 sortable-by-time stop meaning a time. Now pinned by the property
   that actually fails: *the first id of a millisecond must equal what a generator with no history
   would produce there.* **Lesson: when a mutant survives, ask what the code is FOR, not what it
   does — uniqueness and ordering were never the reset's job.**
2. **Deleting the prefix check survived.** The venue-system-id fixtures (`autoclose-` etc.) were
   zero-padded, so with no prefix check they still failed — on the *separator* check, returning the
   *same sentinel*. The test proved the ids were rejected and proved nothing about which check did it.
   Now isolated by a **structurally perfect id with only the prefix changed**. **Lesson: a rejection
   test with one sentinel cannot tell you which check fired; to isolate a check, build the input that
   passes every other one.**

The one survivor left is **semantically equivalent, not a gap**: `NewClientOrderID` ends with a
runtime assertion that the derived id is exactly 35 characters, which the range checks above it
already guarantee. Kept — it costs one branch and would catch a future edit to the format string
before an id reaches a venue — and the claim it stands for is pinned by an internal constants test
instead.

### 8g. One unverified venue assumption, stated rather than buried

The **36-character and `[A-Za-z0-9_-]` budget** the ClientOrderID format fits inside is **not recorded
in `docs/venue-profiles.md`** and has not been browser-confirmed to the §4 standard. It is the widely
documented limit for both venues and the format is designed conservatively inside it.

**Confirm before milestone 3 places a real order.** It is named as a constant (`clientOrderIDBudget`),
asserted by a test, and documented on the type. If a venue allows less, the 26-character TradeID field
is the only part with any give — and shortening it means hashing, which is the alternative this design
rejected.


---

## 9. The second slice continued (session 7) — the instrument identity and `platform/events`

Three decisions were put to the user before any code was written, because each one changes what gets
built rather than how. All three were taken as recommended; the rejected alternatives are below.

### 9a. Where the instrument value objects live — `platform/ids`

§5.1 declares `VenueInstrument{Venue, ProductCategory, Symbol, ContractType}` as "the canonical
identity for routing, market-data keys, orders, fills, reference calls, and recovery cursors". §3.5's
`SourceID` needs it, and `kernelpurity` (rule 9) forbids `platform/` importing `engine/venue` — so it
has to live somewhere in the kernel, and *which* somewhere is a real fork.

**Decision: `platform/ids`.** It is an identity, the package's own doc comment already claims "the
identities build-spec §5.1 assigns here", and `Venue` moved there last session for the same reason.

| Rejected | Why not |
|---|---|
| `platform/events` — define it where §3.5 first names it | Then `oms.Order{Instrument …}`, `risk` and `engine/venue` all import the **event bus** to name an instrument. An order type depending on the transport is backwards. |
| A new `platform/instrument` package | Cleanest naming, but it is in neither §11's tree nor §12.1's package list, so it would owe a spec edit to buy a name we already have a home for. |

### 9b. Scope of the `events` slice — the envelope and the three lanes, not the dispatcher

**Decision: `Envelope`/`SourceID`/`Prio`/`Lane`, the three lanes with their distinct overflow
semantics, and the coalescing policy including the adverse-preserving rule.**

**Deferred, with reasons rather than by omission:**

- **The parity dispatcher.** §3.5 gives it three jobs — assigning `IngressSeq`, arbitrating between
  sources that become ready at once, and waking a live consumer — and it needs a **persisted
  `DispatchSeq` log**, which does not exist. The lanes are the primitives it composes; none of them
  assigns a sequence number or decides an order *between* two sources.
- **§3.4's command sequencer.** It orders Risk→OMS commands and there is neither a Risk context nor an
  OMS to order anything between. Identical reasoning to the deferred `singlewriter` gate: a sequencer
  with nothing to sequence passes its tests trivially, which is indistinguishable from a broken one.

Also rejected: **envelope types only, lanes next session**. A type with no consumer is a guess about
its own shape, and the lanes are what make the delivery contract falsifiable.

### 9c. What a full control lane does — bounded, and it blocks

§3.4 says two things that pull against each other: the control lane is **never dropped**, and
market-data growth is **never unbounded**. The control lane's own overflow behaviour is not stated.

**Decision: bounded queue, blocking send, with a context and a close escape.** Back-pressure reaching
the socket reader is the *designed* behaviour — a stalled reader is visible in a latency graph, and a
dropped execution event is visible nowhere while the net-delta invariant asserts against a number that
is already wrong.

| Rejected | Why not |
|---|---|
| **Unbounded queue + high-water alarm** | Never blocks, never drops, and its failure mode is an **OOM kill** — which arrives mid-journal-write, taking down the thing whose durability the lane exists to protect. |
| **Fail fast on overflow** (`ClassInvariant` + `EffectNotApplied`) | Loud rather than silent, but it converts a transient consumer stall into a **trading halt**. A large, self-inflicted penalty for a condition that resolves itself in milliseconds. |

The escape matters as much as the choice: without a context on `Send`, a wedged consumer wedges
shutdown too — every producer parked in `Send`, and the process unkillable except by force, which
loses the backlog the lane exists to preserve.

### 9d. The three lanes are three answers to one question

Worth stating because a reviewer skimming the signatures will see three near-identical queues. They
differ only in what happens when the consumer falls behind, and that difference is the package:

| Lane | Full consumer | Drops | Rationale |
|---|---|---|---|
| control | **blocks** | never | a lost execution event is lost execution truth |
| market data | never blocks | **oldest**, counted | the newest update is closest to the truth |
| observability | never blocks | **newest**, counted | independent spans; none supersedes another, and refusing the newcomer touches nothing already accepted |

The two drop directions being opposite is deliberate and is pinned by a test in each file, because
"drop" alone does not say which and a later change to either would be silent.

`StreamKind.Lane()` makes routing a property of the **stream** rather than of the caller, and
`MarketDataLane.Publish` **refuses** traffic that routes elsewhere. That guard is the one place the
package can catch its own worst failure: a fill accepted onto the market-data lane is coalesced away,
and a fill that never arrives is a position the ledger does not know about — no error, no log.

### 9e. The adverse-preserving mark rule, and the fabrication it refuses `[→ R2c-1]`

§3.4's exception to latest-wins: "the worst adverse mark since the OMS last consumed it is never
dropped, so a brief adverse excursion that breaches a threshold cannot be hidden by a later recovery."

The implementation question the spec does not answer: **when the worst payload and the newest metadata
disagree, which envelope survives?**

**Decision: the worst payload brings its OWN envelope — its sequence numbers, its `RecvTime`.** Both
merge branches therefore yield a real observation.

**Rejected: newest metadata + worst payload.** It reads better on a freshness metric and it is a
fabrication — a price stamped at an instant it did not hold, flowing into threshold comparisons and
into an audit trail. The cost of the chosen rule is **stated rather than hidden**: while an unconsumed
excursion sits in a mailbox, the source reads marginally staler than it is. That direction is safe —
it can only *trigger* a freshness check, never suppress one — and the window is bounded by the
consumer's drain rate.

Two supporting decisions:

- **"Adverse" is the caller's to define.** It depends which way the position faces, and rule 9 forbids
  the kernel knowing. `AdversePreserving[T]` takes the comparison; the cash-and-carry short (§2.2)
  supplies "higher is worse".
- **An uncomparable payload declines to merge.** Keeping both costs memory pressure and a visible
  drop-oldest count; merging what the rule cannot compare would discard a mark silently.

### 9f. What is deliberately NOT claimed

- **Coalescing is not deterministic under concurrency, and the code says so.** §3.5 requires a
  deterministic *rule* keyed on `(Source, field)`, "never a function of goroutine scheduling". The
  `MergeFunc` signature enforces that half — it sees two envelopes and nothing else. Whether two
  updates ever *meet* in a mailbox depends on whether the consumer drained between them, which **is**
  scheduling; §3.5 resolves that with **persistence**, not purity, and replay re-applies the frozen
  dispatch log. Claiming more would be claiming something false.
- **Lanes do not validate envelopes.** `AcceptOrdinal` is stamped at ingestion and `IngressSeq` by the
  dispatcher, so an envelope in flight *toward* the dispatcher legitimately fails `Envelope.Validate`.
  A lane that validated on `Send` would reject exactly the traffic it exists to carry.
- **`VenueInstrument.String` and `SourceID.String` are not injective.** A venue symbol containing the
  separator would collide. Inventing a symbol charset to prevent that would over-fit to venues
  `venue-profiles.md` has not verified; the identity is the **struct**, which is comparable and is used
  directly as a mailbox key.
- **Symbol case is neither folded nor checked.** Folding would be a claim about venues rather than
  about this package — `BTC-PERPETUAL` and lowercase-symbol venues exist, and a fold that corrupts a
  real symbol fails at order submission. Whitespace *is* rejected (not trimmed), because the
  one-spelling rule forbids synonyms and " BTCUSDT" would be a second mailbox.

### 9g. `ProductCategory` and `ContractType` encode linear-vs-inverse twice

§5.1 gives `VenueInstrument` both fields, so `ProductLinearPerp` already implies `ContractLinear`. A
redundant encoding is safe only while something forbids the copies from disagreeing — otherwise the
struct can hold a state no instrument has, and two consumers reading different fields compute
different notionals. `Validate` rejects the disagreement, which turns the redundancy into a checksum.

`ProductInversePerp` is declared although no MVP adapter trades one. Without it `ContractType` has a
single usable value, the agreement rule has nothing to be wrong about, and "this contract is linear"
becomes an assumption rather than a checked claim.

**Recorded, not taken:** dropping `ContractType` from the struct and deriving it from `Product` would
remove the redundancy outright. That is a spec change rather than an implementation choice, and
`ContractType` must exist regardless for `SymbolSpec`, which has no `Product` field.

### 9h. One [SPEC EDIT OWED], following the v0.9.5 precedent

**§3.5's `SourceID` is declared as four flat fields** — `{Venue, Product, Symbol, Stream}` — with the
trailing comment "= a VenueInstrument stream". The code **composes** `ids.VenueInstrument` instead.

Two reasons, and the second is the real one: the flat form has **no way to express the `ContractType`**
§5.1 gives an instrument, so a source could not name an inverse contract at all; and duplicating three
fields duplicates their validation, which is how two definitions of "a valid instrument" come to
disagree. The spec's own comment says this *is* a `VenueInstrument` stream — this makes that
structural.

**Not landed in `build-spec.md` yet, deliberately, and this follows the precedent v0.9.5 set:** the
first slice marked ten items `[SPEC EDIT OWED]` as they were taken and landed all ten together at
slice close, as one coherent release. The second slice is **still open** (`config`, `outbox`,
`observability`, `ratelimit`, `boot` remain), so this marker stays in the code — at
`internal/platform/events/envelope.go`, on `SourceID` — and lands with whatever else accumulates.
**The marker is the tracking mechanism; do not close the slice without grepping for it.**

### 9i. Bite-checked: 43 mutations, 41 caught, 2 diagnosed — and the harness gained two verdicts

Both survivors were **real gaps**, and both are the same lesson arriving twice: *a test that cannot
distinguish two implementations is not covering the distinction.*

1. **`Send`-after-`Close` was checked once, against a 50/50 select.** With room in the queue and the
   lane closed, both select cases are ready and Go picks at random — so a `Send` that consulted the
   closed signal only *inside* that select would accept post-close envelopes about half the time. The
   `Receive` side already ran 200 trials for exactly this reason. **The lesson had been applied in one
   place and not in its mirror**, which is how a fixed defect comes back through the other door.
2. **Every coalescing test used depth 1**, where "merge against the newest" and "merge against the
   oldest" are the same slot. The mutant was equivalent *under the fixtures* and a real defect outside
   them — reachable exactly when a merge **declines**, which is the only way a coalescing mailbox grows
   past one entry, and where the difference is an envelope vanishing without being queued or counted.
   §5d's vacuous-run lesson, one level in: **the branch was unreachable while looking like coverage.**

**The harness itself gained two verdicts this session, both found by real mutations, not by design:**

3. **`NOBUILD!!` — `go test` exits non-zero for a BUILD failure exactly as it does for a failing
   test.** A mutant that does not compile printed as `caught` and proved nothing. Go makes this easy to
   hit: deleting a call site orphans an import, which is a compile *error*. The only visible trace was
   a failing-test count of **0**, which is a contradiction — the harness was already printing the
   evidence and not acting on it. Re-aim such a mutation to substitute a **wrong value** rather than
   delete a call.
4. **`TIMEOUT!!` — a mutation that made a non-blocking `Send` block HUNG the suite** instead of failing
   it, wedging the run until the shell timed out. That is §5e's own lesson arriving through the
   harness: a hanging test names no defect. `go test` now runs bounded (`MUTATE_TIMEOUT`, default 60s)
   and a hang is reported as its own verdict — **not** a kill. The observability tests were fixed to
   fail within a bound rather than park on a full lane.

All four verdicts — `caught`, `SURVIVED`, `NO-OP!!`, `NOBUILD!!` — plus `TIMEOUT!!` were exercised as
controls before any of the results above were believed.

---

## 10. The second slice continued (session 8) — `platform/config`

### 10a. What the package owns — the machinery, not §5.2's schema `[user decision]`

Two shapes were put to the user, and the first was taken.

**Taken: `platform/config` is generic machinery.** `Decode[T]`, `Store[T]`, `Revision`, the reload
policy, and two shared value types. §5.2's risk caps, per-venue block, per-strategy params and
emergency policy are defined by the contexts that own their meaning — `risk`, `engine/venue`,
`strategy` — and `app/` composes them.

**Rejected: the whole §5.2 schema in the kernel.** §5.2 does say "Config is a typed struct with
`Validate()`", and the struct is expressible entirely from `platform` types (`fixedpoint.Money`,
`ids.Venue`), so `kernelpurity` would have passed. It was rejected on two grounds. The §3.2 table's
entry for this package is a list of **verbs** — "typed load + `Validate()` + safe hot-reload;
revisions recorded for replay" — not a list of fields. And the practical test decides it: adding a
risk cap should touch `risk`. If the cap lived in the kernel, every context's parameters would be a
kernel edit, and `risk` would import the definition of its own caps from a package that cannot
import `risk` back.

**Nothing here is a spec deviation.** §5.2 enumerates the schema's *content* and never says where it
lives. This is an interpretation of an open question, recorded so it is not silently reopened.

### 10b. "Hot-reloadable where safe" is default-deny `[user decision]` `[SPEC EDIT OWED]`

§5.2 says configuration is "hot-reloadable where safe" and stops there. Somebody has to decide *where*
per field, and three mechanisms were put to the user.

**Taken: a struct tag, `config:"hot"`, checked by a reflective diff, default-deny.** A field may
differ across a `Reload` only if it — or an ancestor — carries the tag; the first field that moves
without permission returns `ErrNotReloadable` naming its path, and **nothing is swapped**.

The argument is entirely about which way it fails when somebody forgets. Default-**allow** means a
field added to a schema next year is silently hot, and the noticing happens when a running process
picks up a journal path or a fencing generation mid-trade. Default-**deny** means it is
restart-required until somebody writes the tag, and the noticing happens when a reload is refused
with the field named. This is §3.6 rule 4's argument for `float64` — deny until a reason is written
down — applied to a requirement the spec left open rather than to one it decided differently.

An **unrecognised** tag value is an error rather than a silent deny, which is the one place the safe
direction is not the right one: denying is safe, so a typo'd `config:"hto"` would fail safe *and
silently*, leaving an operator believing a field reloads when it does not.

**Rejected: splitting the schema into `Static` and `Hot` types.** The compiler would enforce it, with
no reflection and no tag to forget — genuinely stronger. It was rejected because the safety boundary
would then dictate the schema's shape, and §5.2's shape cuts across it: a per-venue block holds a
static endpoint *and* a hot rate-limit budget, so it would have to be split in two and the config
file's grouping would stop matching the Go types one-to-one.

**Rejected: a schema-owned `ValidateTransition(prev T) error`.** The most expressive — it could say
"leverage may drop but never rise while a position is open" — but it carries the unsafe default: a
schema that never implements it gets everything hot-reloadable, and nothing fails. That is the exact
shape of the `apperror` `EffectOf` defect (§8a), which this project has already paid for once.

The tag is marked `[SPEC EDIT OWED]` in `internal/platform/config/schema.go`, following the v0.9.5
precedent of batching a slice's spec edits into one release at slice close. It joins §9h's `SourceID`
item; **both are found by `grep -rn "SPEC EDIT OWED" internal/`**.

### 10c. Composites recurse; adding an entry and editing one are different acts

The reload diff walks *into* maps, slices and pointers rather than comparing them wholesale, and that
is not a refinement — it is what makes §5.2's actual shape work. A per-venue map carries no tag, so a
wholesale `DeepEqual` would refuse **any** difference under it, and the per-venue rate-limit budget
§5.2 puts exactly there could never be tuned without a restart.

So entries present on both sides are compared element-wise, while **adding or removing** an entry is
a structural change to the map and needs the *map* to be hot. The asymmetry is deliberate: editing a
venue's budget is something a running process absorbs, and introducing a venue means wiring a client
it does not have.

### 10d. Four things a config file does silently

Configuration is read once, at boot, by nobody, and a mistake in it does not crash — it computes a
different number. Each of these is now default-deny, and the middle two are checks the standard
library does not make:

| | what Go does | what this does |
|---|---|---|
| unknown key | ignores it | rejected — a typo'd `per_trade_notionl` otherwise leaves the real cap at its zero value with nothing reported |
| **duplicate key** | **takes the LAST, silently** | rejected by a token-level scan. `DisallowUnknownFields` cannot see it: both occurrences are *known* fields |
| **trailing data** | **reads one value, ignores the rest** | rejected — a file holding two objects otherwise loads the first and never mentions the second |
| untagged field | matches the Go identifier, case-insensitively | rejected at load — the wire name must be a decision, not a consequence of an identifier a refactor can rename |

The two middle rows carry **measurement** tests rather than assertions
(`TestDecode…WouldOtherwiseBeSilent`): they run the same bytes through `encoding/json` directly and
assert that it accepts them and which value it takes. A guard is only worth its complexity while the
thing it guards against is real, and these say so rather than sitting there unjustified.

### 10e. Two shared value types, each because its absence is quiet

**`SecretRef` cannot hold a secret.** That is the design, not a usage convention. It holds a scheme
and a name — `env:BINANCE_API_KEY`, `file:/run/secrets/bybit.key` — with no field for a value and no
method producing one. So §5.2's "key refs (never inline secrets)" is enforced rather than requested:
a pasted credential **has no scheme**, so it fails to parse and the process does not boot. And
printing one is safe *by construction* — there is nothing else in there — rather than by remembering
to redact. Resolution is deliberately elsewhere: a kernel type that could resolve is a kernel type
that could log a resolved value. The scheme set is closed, and a `file:` reference must be
**absolute**, because a relative secret path resolves against a working directory a systemd unit or a
container entrypoint does not choose.

**`Duration` crosses the boundary as a string.** A JSON number decoded into a `time.Duration` is
**nanoseconds**. §5.2's `T_blind` of `300` — five minutes to whoever typed it — becomes 300ns: a
stale-mark blind budget that has expired before the first check, firing the §7.1 de-risk on the first
tick with nothing wrong. Nothing errors and nothing logs. So a bare number is **refused** rather than
interpreted, and `"5m"` is a string on whose meaning Go and the operator cannot disagree. Zero is
deliberately **valid** — "no dwell" and "check immediately" are real settings, unlike a zero cost —
and a schema for which zero is wrong says so in its own `Validate`, where it can name the field.

### 10f. The scoped no-values-in-messages invariant

`ErrNotReloadable`, `ErrDuplicateKey`, `ErrTrailingData` and every audit record name a **path** —
`risk.max_total_open`, `venues["binance"].endpoint` — and never the value at it. An error travels to
logs and alerting, and a config struct is exactly where a credential ends up when somebody declined
to use `SecretRef`. `ParseSecretRef` holds the same line for a sharper reason: the value it refuses
is, by construction, the one input that is definitely a credential.

**The claim is scoped, and the exclusions are stated rather than overlooked** — an unscoped version
would have been an overclaim. A `Duration` that fails to parse echoes the text that failed, because
that text is the operator's own typo and never became configuration; a decoder syntax error can name
the character it stopped on; and a schema's own `Validate` is outside this package entirely.

### 10g. What is deliberately NOT here

The **file watcher** — deciding when to re-read bytes is a composition-root concern with its own
failure semantics (a truncated write, an editor's atomic-rename dance, a debounce window) and it
needs a `Clock`. The **revision log** — this package *mints* the identity §3.5 replays against and
persists nothing; persistence is the journal's job. The **approval kernel** — §5.2 routes
emergency-policy edits through `platform/foureyes`, and this package would happily reload a policy
struct that had never been approved; wiring the two is `app/operator`'s job.

And there is **no clock**. `Decode`/`New`/`Reload` take the instant as an argument. `notimenow` does
not police `platform/`, so this is discipline rather than enforcement — but `Revision.LoadedAt` is a
replayed field, and a timestamp a backtest cannot control is a timestamp that makes a replay differ
from the run it replays.

### 10h. Bite-checked: 43 runs, 37 claims, all caught — and six survivors worth the diagnosis

31 of the first 37 were caught outright. **Every one of the six survivors was worth diagnosing, and
five of them were real test gaps of a single shape** — §8f's lesson arriving in bulk:

**Four fixtures were failing the NEXT check.** Every schema-audit refusal returns `ErrSchemaField`,
so a deleted check whose fixture happened to trip a later one produced an identical error and an
identical pass. An unexported field with no json tag failed the *no-json-tag* check; an embedded
*unexported* type failed the *exported-field* check; a field with no tag failed the *empty-name*
check; a zero `SecretRef` failed the *reparse*. Fixed by asserting the **message**, not only the
sentinel — the message is the diagnosis a programmer acts on, and "this field has no json tag" and
"this field's json name is empty" are different instructions.

One of those fixes ran into a wall worth recording: the obvious repair — give the unexported field a
json tag so only one check can reject it — **does not build under `make ci`**, because `go vet`'s
`structtag` analyzer refuses a json tag on an unexported field. The isolation had to come from the
message assertion instead.

**The fifth was a real gap, and the code was right.** Removing a map entry is caught by the per-key
presence walk; **adding** one is invisible to it, because every live key is still present. Only the
size comparison sees it. The removal case was tested and the addition case was not, so a mutant
deleting the size check survived while silently admitting a whole new venue.

**The sixth was redundant code, now deleted.** A `bytes.TrimSpace(data)) == 0` pre-check never fired
in a way that mattered: `json.Decode` returns `io.EOF` for an empty *or* all-whitespace document and
the branch below already maps that to `ErrEmptyDocument`. It was worse than redundant — a document of
one non-breaking space is not empty, and `TrimSpace` trims Unicode whitespace that JSON does not, so
the pre-check reported "empty document" where the decoder would have named the invalid character.
Deleted, and the **surviving path was then bite-checked in its place** rather than assumed.

The harness fired its `NO-OP!!` guard once, correctly: adding the `[SPEC EDIT OWED]` block to
`schema.go` moved a target line, and a re-run at the stale number reported no-op rather than a
phantom kill.

### 10i. `nofloat` fired on this session's own code

`config.Duration.Duration().Seconds()` in a freshly written test — `time.Duration.Seconds()` returns
`float64`. The gate caught it in `make ci` before it was ever committed, on a line written without
thinking about it at all. Worth noting because it is the second time (§5) a §3.6 gate has bitten real
code rather than a fixture: the value of a default-deny type-based rule is precisely that it does not
depend on the author remembering the rule.

---

## 11. The second slice continued (session 9) — `platform/outbox`

### 11a. Scope — ports and the relay, no store `[user decision]`

Three shapes were put to the user, and the first was taken.

**Taken: `Message`/`Record`, the `Store` and `Publisher` ports, and the `Relay`. No `Append`, no SQL,
no schema, no driver.** Everything whose behaviour is decidable against a fake store, and nothing
whose behaviour is a property of a database that does not exist.

**Rejected: ports plus a `database/sql` store now.** `database/sql` is stdlib, so `kernelpurity`
would have permitted it, and the third-party SQLite driver registers itself at the composition root
— the gate does not decide this one. It was rejected on the argument the gate-deferral section
already settled for `hexboundary`: the outbox's defining guarantee is that the event append commits
**in the same transaction as the OMS journal write** (core §5.3, §6 step 6), and the journal is
milestone 3. Writing the SQL now means guessing the transaction shape of an undesigned thing and then
testing the guess against itself. A check over an empty tree passes trivially, which is
indistinguishable from a broken check.

**Rejected: skip the outbox and build `observability` first.** Defensible — `observability` and
`ratelimit` are both fully self-contained today. Rejected because the split above recovers most of
the value: the relay's ordering, retry and never-drop behaviour is exactly the part that is subtle,
and it is all checkable now. Reordering would have deferred the subtle half to sit beside a database.

**`Append` is absent as a positive decision, not an omission**, and `ports.go` says so at the
declaration. An `Append(ctx, Message)` on the `Store` interface would be strictly WORSE than nothing:
it would compile, it would read as the outbox contract, and it would silently be a *second*
transaction. A lost race between those two commits is the split-brain the pattern exists to prevent.
So the value crossing the boundary is defined (it is knowable now) and the verb is not (its shape is
not).

### 11b. Per-key FIFO, scoped `[SPEC EDIT OWED]`

§3.2 and D2 describe the outbox as an "at-least-once export to idempotent consumers" and say nothing
about **order**. Order is nonetheless load-bearing, because D2 also calls audit "a projection of the
OMS journal", and a projection that applies a fill before the order that produced it does not
reproduce its source. The spec states a delivery-**count** guarantee and relies, unstated, on a
delivery-**order** one.

**Taken: FIFO within a `Key`, nothing across two.** Same structural rule §3.5 states for dispatch —
"an unready `SourceID` never stalls dispatch of ready inputs from other `SourceID`s" — and the
agreement between the two is deliberate.

**Rejected: global FIFO.** Strictly stronger, and it converts one poison record into a total halt of
every consumer. **Rejected: no ordering guarantee at all** — that is the status quo reading of the
spec, and it makes the audit projection's agreement with its source a coincidence.

The cost is stated in the package doc rather than hidden: a record that fails forever blocks **its own
key** forever. That is the safe direction, and the reasoning is §10b's question asked again. Skipping
past a stuck record publishes K7 while K5 is unsent, which silently reorders a hash-chained
projection; blocking is loud, because the key's backlog and attempt count climb where a metric sees
them.

### 11c. Two value-shape decisions, each because its absence is quiet

**`Payload` is `[]byte`, not `any`** — deliberately unlike `events.Envelope.Payload`, and the
difference is the durability boundary. An `Envelope` is dispatched in-process and never persisted as
a value, so it can carry a typed Go object. A `Record` already crossed a transaction: these are the
exact bytes that committed beside the journal write, and §7.5/Z4 hash-chain the projection they feed.
Holding a Go object and re-marshalling it at delivery time lets the exported bytes differ from the
committed ones — a map iteration order or a struct-tag change is enough — which breaks a hash chain
in a way that reads as **tampering** rather than as a bug.

**`Seq` is named as the dedup key in the contract.** At-least-once without a stated dedup key invites
the obvious wrong answer: content-hash dedup, which silently drops the second of two legitimately
identical events (the same funding accrual on two intervals).

**`Topic` and `Key` are open string types, not closed enums.** §3.6 rule 8's shape does not apply,
taking the same exception `clock.TimerID` already took: the set of names is open. A closed
`TopicAudit | TopicObs | TopicTax` would repeat §10a's rejected shape in a new place — adding a
consumer would become a kernel edit. What is enforced instead is a **format**, via an allow-list
rather than a deny-list of whitespace, because a topic is written to an indexed column, printed in
logs, and used as a §10 metric label, and a deny-list admits every character nobody thought to name.

**An empty `Key` is rejected rather than read as "unordered".** §10b's question again: a producer that
simply forgot a key would get its events interleaved, silently, visible only in the projection.

### 11d. The relay owns no goroutine

`RunOnce` performs one claim → publish → settle pass and returns a `Result`. **Rejected: an internal
ticker draining continuously** — that makes the instant of every publish a function of wall-clock
scheduling, which is not reproducible. `platform/clock` made the identical call for the identical
reason (§5e: pull-based timer delivery, no goroutines in the package at all). Whoever owns the total
order owns the cadence.

The relay is stateless between passes **by construction**: attempt counts and retry instants live on
the record in the store, so a process that crashes mid-backoff does not resume at attempt zero and
start hammering a consumer that has been rejecting it for an hour.

`Claim` takes `now` as an argument rather than letting the store read a clock. This is §3.6 rule 5's
argument reaching a port signature: retry eligibility is a comparison against `RetryAt`, and a store
reading its own clock puts two clocks either side of it, so a replay claims a different set than the
live run did.

### 11e. The backoff guard is on the right-hand side, and that is the whole trick

`Delay` computes `Base << attempts` capped at `Max`. The obvious form — shift, then compare — is
wrong in the damaging direction. `time.Duration` is an `int64` of nanoseconds, so with a `Base` of one
second the shift overflows at 34 attempts, and an overflowed shift does not produce a very long wait:
it produces a **negative** one. A negative delay schedules `RetryAt` in the past, so the record is
reclaimed on the very next pass — the backoff becomes a hot loop against a consumer that is already
failing, which is the exact opposite of its purpose.

Asking `Base > Max>>attempts` instead never performs the overflowing shift. Go defines a right shift
wider than the type as zero for a positive operand, so a huge attempt count answers `Base > 0` — true
for any valid schedule. The function is **total over every `uint32` with no special-case branch**,
which matters because a special case is a branch that has to be remembered and tested.

**No jitter, and that is a decision.** Jitter de-synchronises a herd; a single in-process pump over a
local store has no herd, and randomness would make the retry instant unreproducible, breaking the
replay determinism §3.5 rests on. If a multi-process deployment ever creates a herd, the jitter
belongs there with a seeded source.

**`RetryAfter` and the computed backoff resolve to the longer of the two, in both directions**, and
each direction is load-bearing. A longer `RetryAfter` must win, because retrying inside a venue's
stated window is guaranteed to fail and spends rate-limit budget §5.3 reserves for recovery. A
*shorter* one must lose, or a consumer advertising one second resets an hour of accumulated
escalation on every attempt — the backoff would stop escalating precisely against the consumer that
keeps failing. Both directions carry their own test; both mutants are caught by exactly one test each.

**`apperror`'s `Effect` axis is deliberately not consulted.** Under at-least-once, `EffectApplied` and
`EffectNotApplied` lead to the same action — redeliver — so branching on it would be a distinction the
retry logic cannot act on. It still matters to the operator, where it is the difference between a
duplicate that is certain and one that is merely possible; `ports.go` says so, because a reader who
knows §8a will expect the axis to be used and should find out why it is not.

### 11f. Bite-check — 38 runs, 30 claims, all caught

Four survived. **Three were real gaps**, all closed:

- **The relay sorted the store's slice in place.** `ordered := slices.Clone(claimed)` → `ordered :=
  claimed` survived because the fake returned its own slice and nothing looked at it afterwards. The
  concern is real rather than theoretical: a store implementation written later against this
  interface — reasonably returning a cached or reused slice — would have it destroyed by a package
  it only knows through the port, and the symptom is a silently reordered projection. §10b's question
  again, and the answer is the same: the consumer must not mutate.
- **Every malformed-identifier fixture put its bad character mid-string.** `i >= 0` → `i > 0`
  survived on **both** `Topic.Validate` and `Key.Validate`, because at index 0 the two predicates
  differ and no fixture reached index 0. `" audit"` validated. This is §9i's fixture-dimension lesson
  landing in a third place — after `events`'s mailbox depth of 1 and §5d's fuzz-corpus reach — and it
  is now clear that the lesson generalises to *any* fixture parameter pinned to the value that
  collapses a distinction the code makes.
- **The mirror was fixed in the same pass.** `Key` runs the same scan through the same helper, so it
  has the same blind spot; §9i's other lesson says go and look immediately rather than let the defect
  return through the other door.

**The fourth is semantically equivalent and is now flagged in the code as an expected survivor.**
Removing the empty-claim fast path changes nothing observable: the code below produces
`Result{Claimed: 0}` — which is `Result{}` — clones nil, iterates nothing, settles nothing and joins
no errors. It is kept because an idle outbox is the steady state of a continuously pumped relay, so it
saves an allocation on the hot-by-frequency path. **Naming an expected survivor in the code is new
here and worth keeping**: a mutation harness cannot tell a fast path from a defect, so the code has to.

Both harness guards fired and both were right. One **`NOBUILD!!`** — adding a `%s` verb without an
argument, caught by `go vet`'s printf check inside the mutant, re-aimed to substitute a wrong value
per §9i's own advice. Three **`NO-OP!!`** — stale line numbers after edits, which is exactly the
failure the line-scoped design exists to report rather than print as `caught`.

One claim had **no test at all until the bite-check demanded one**: `Record.String` omits the payload,
which the relay's error path depends on since it formats records with `%s`. The doc comment asserted
it; nothing checked it. §10f's ruling applied without the test being written first.

---

## RESUME HERE — updated end of session 9

**Environment, verified — do not re-derive.** **Go 1.26.5** (upgraded from 1.26.0 in session 5;
both modules declare `go 1.26.0` as the minimum and `toolchain go1.26.5` as what to build with —
see §2, and do not "simplify" that into one directive). Remote `github.com/giiorg/arb-lab`. Branch
`feat/milestone-1-foundation`, tree clean, `make ci` green (fmt-check → vet both
modules → custom gates → race tests both modules, `RAPID_SEED` pinned). Test-only dependency
`pgregory.net/rapid v1.3.0` added; §11's "never a dependency" governs production code, untouched.
`bitnet-backend-go` is **not on this machine** — a described style, not a readable source.

**The mutation harness is IN THE REPO at `tools/mutate.sh`** (session 6), rather than being rewritten
from the standing lessons every session. It now carries **four** harness lessons as code, each paid
for by a real mutation rather than designed in: it `cmp`s and reports `NO-OP!!` when a pattern fails
to match (session 4); it takes a line number and prints the diff it applied (session 5); it reports
`NOBUILD!!` when the mutant did not compile, because `go test` exits non-zero for a build failure
exactly as it does for a failing test (session 7); and it runs **bounded** and reports `TIMEOUT!!`,
because a mutation can make a test *hang* rather than fail, and a hanging test names no defect
(session 7). **A `caught` with 0 failing tests is a contradiction — that is what exposed `NOBUILD`.**

**Both branches were PUSHED and in sync at the end of session 9.**
`feat/milestone-1-foundation` tracks `origin/feat/milestone-1-foundation`. **Check it rather than
trust this paragraph** — `git rev-list --left-right --count @{u}...HEAD` should print `0 0`. The
head commit is deliberately not named here: a paragraph that cites its own commit hash is stale the
moment the commit containing it is written, and a hash that a reader then finds does not match is
worse than no hash. Every push in sessions 5–7 was verified as a clean fast-forward **before**
pushing rather than assumed — `git merge-base --is-ancestor @{u} HEAD` for the direction, and the
left-right count for the magnitude. **No outward-facing action is owed.**

**A PR has *not* been opened, and that is deliberate — the branch is pushed, not proposed.** Open
one only if asked. Note that the branch now carries the whole of milestone 1 and will keep growing:
if a PR is ever wanted, it is worth asking whether it should be one per slice rather than one for
the milestone.

**Milestone 1's first slice (§1) is CODE-COMPLETE and closed out (session 5). The SECOND SLICE is
six packages in:** `platform/apperror` and `platform/ids` (session 6, with the `Venue` move out of
`platform/clock`), then `ids`'s instrument identity and **`platform/events`** (session 7), then
**`platform/config`** (session 8), then **`platform/outbox`** (session 9). The kernel is **seven
packages** — `fixedpoint`, `clock`, `apperror`, `ids`, `events`, `config`, `outbox` — plus three
gates. **There are no open questions.** `platform/observability` is the next package up the spine.

**THREE SPEC EDITS ARE OWED, and they are tracked by markers in the code, not by this paragraph.**
§9h: §3.5 declares `SourceID` as four flat fields; the code composes `ids.VenueInstrument` instead,
because the flat form cannot express the `ContractType` §5.1 gives an instrument. §10b: §5.2 says
configuration is "hot-reloadable where safe" and stops; the code makes "where safe" a per-field
**default-deny** `config:"hot"` tag, on rule 4's own argument. §11b: §3.2 and D2 describe the outbox as
an at-least-once export and say nothing about ORDER, while relying on it — audit is called "a
projection of the OMS journal", and a projection that applies a fill before its order does not
reproduce its source; the code makes the guarantee explicit and scopes it to the Key. All three are
deliberately not landed yet:
the v0.9.5 precedent is to batch a slice's spec edits into one coherent release at slice close.
**Before closing the second slice, `grep -rn "SPEC EDIT OWED" internal/`** (code only — the doc set is full of PROSE about past markers, so a repo-wide grep is mostly false positives) and land what it
finds. It should report exactly three.

**The `CLAUDE.md` owed since §2 now exists** (repo root, written session 6). It carries the
`internal/` taxonomy, the doc-set map, the build split, the three enforced gates, and the standing
working rules. That item is discharged.

### Done, so do not redo

1. ✅ **RoundingMode** — the proposal confirmed unchanged (§5).
2. ✅ **All `fixedpoint` arithmetic** — four categories, the 128-bit kernel, `Calc` (§5b).
3. ✅ **The oracle suite** — `math/big` oracle, 4 rapid properties, 19 negative + 4 positive
   controls, and the seed policy verified against module source and wired into the Makefile (§5c).
4. ✅ **SQL codec, zero-alloc assertions, and the parser fuzz target** — `FuzzParseMantissa` with
   four total invariants, six negative controls, a gate on the oracle cross-check justified from
   `math/big` source, and its corpus reach measured rather than assumed (§5d). **`platform/fixedpoint`
   is COMPLETE.** Do not reopen it; `make fuzz` is available if a longer explorer run is wanted.
5. ✅ **`nofloat` (§4b) and `notimenow` (§4c, §4d)** — both built, registered in the multichecker, and
   bite-checked at 8/8 and 16/16 mutations. **The analyzer roster is 3 of 6 built**; the other three
   stay deferred for the reasons §4 gives (nothing to police yet). `nofloat` fired on real code and
   both hits were resolved, not suppressed. **`notimenow` now polices `engine/` too, with a
   transport-only escape** — user-approved, §4d, and it carries a falsifiable claim to revisit: if
   `engine/venue` ships without ever using the escape, delete the mechanism.
6. ✅ **`platform/clock`** (§5e) — the four spec'd methods, `Wall` + `TestClock`, `ServerTime` with a
   three-valued `Passed()`, the shared `(At, Prio, ID)` queue, and a conformance suite run against
   both implementations with **11 negative controls and a positive control**. 18/18 mutations caught.
   **Do not reopen it.** Timer *delivery* is pull-based via a separate `TimerSource`; waking a live
   consumer is deliberately left to the dispatcher.

7. ✅ **All ten [SPEC EDIT OWED] items landed (session 4)** — `build-spec.md` is **v0.9.5**, ADR
   `adr-history.md` carries the milestone-1 implementation-decision block, and the markers throughout
   this record now read **[SPEC EDIT LANDED — v0.9.5]**. The `boundary/` flatten rewrote 26 path
   occurrences and left `boundary/lint` in §3.6 alone, as instructed. **Do not re-apply any of them.**
8. ✅ **Rule 5's `engine/` scope — the milestone's one user question, asked and answered YES**
   (§4d). Built, bite-checked 9/9, committed. **Do not re-ask.**
9. ✅ **The `gremlins` audit — RUN and closed out (session 5, §6b). Do not re-run it as if it were
   still owed.** It *does* build on Go 1.26 (§0's "unmaintained" is about the project, not the
   build), it found **four real gaps, all four now fixed**, and it stays out of CI for reasons that
   are now measured: a warm test cache makes it report 0.00% efficacy, `./...` silently finds
   nothing, non-viable mutants are scored as kills, and **one run in four produced a false LIVED**
   on a mutant the suite kills 20/20 by hand. **Every `LIVED` it reports must be reproduced manually
   before it is believed.** If you want it again after new code lands, §6b has the exact invocation.

10. ✅ **`platform/apperror`** (§8a) — the two-axis taxonomy, `Class` + `Effect`, with the
    non-laundering wrap rule, the safe-by-default chain walkers and the malformed-spec degradation.
    14/14 mutations caught. **Do not collapse the two axes back into one**; §8a records why, and the
    two rejected shapes were both put to the user. **Do not "simplify" `recordedEffect` into
    `EffectOf`** — they are the same walk with deliberately opposite defaults, and merging them makes
    `EffectNotApplied` unreachable in practice while everything still passes.
11. ✅ **`platform/ids`** (§8b–§8g) — `Venue` (moved), Crockford base32 with a `math/big` oracle,
    RFC 9562 UUIDv7 checked against the RFC's own worked example, `TradeID`/`IntentID`/`ExecutionID`,
    and the reversible `ClientOrderID`. 25 mutations, 24 caught, 1 semantically equivalent.
    **Do not add an import of `platform/clock` to it** — `clock` imports `ids`, so that is a cycle;
    the time seam is the structural `TimeSource` interface and that is deliberate (§8c).
12. ✅ **`CLAUDE.md` written** at the repo root — the §2 item, discharged.
13. ✅ **`ids`'s instrument identity** (§9a, §9g) — `VenueInstrument`, `ProductCategory`,
    `ContractType`. **Do not move them to `events`**: an `oms.Order` would then import the event bus to
    name an instrument. **Do not "simplify away" the ContractType/Product redundancy** — §5.1 declares
    both, `SymbolSpec` needs `ContractType` with no `Product` beside it, and `Validate` turning the
    redundancy into a checksum is the point. 12 mutations caught, 1 intended survivor.
14. ✅ **`platform/events`** (§9b–§9f) — `Envelope`/`SourceID`/`Prio`/`Lane`, the three lanes, and the
    coalescing policy. 31 mutations, 29 caught, 2 real gaps found and closed.
    **Do not make the lanes validate envelopes** — `IngressSeq` is stamped by the dispatcher, so a
    lane that validated on `Send` would reject exactly the traffic it carries (§9f).
    **Do not give `AdversePreserving` the newest envelope's metadata with the worst payload** — that
    fabricates an observation, and §9e records why the honest version's staleness cost is the safe
    direction. **Do not sort any lane by `Prio`** — §3.5's total order is the dispatch order, and
    sorting inside a lane reorders within a `SourceID` where `VenueSeq` is authoritative.
    **`events` does NOT import `clock`**, and that turned out to be right: enforcing a max-age needs a
    Clock and a timer, both of which belong with the dispatcher; the lane only carries the `RecvTime`
    an age is computed from, and `Envelope.Age` is pure.

16. ✅ **`platform/outbox`** (§11a–§11f) — `Message`/`Record`, the `Store` and `Publisher` ports, the
    `Relay`, per-key FIFO and the backoff. 38 mutation runs, 30 claims, all caught.
    **Do not add an `Append` to `Store`** — §11a records why, and it is the one addition here that
    would be worse than nothing: it compiles, it reads as the outbox contract, and it is silently a
    second transaction. It lands with the journal, in milestone 3.
    **Do not make FIFO global** — §11b; one poison record would halt every consumer.
    **Do not "fix" the backoff by shifting first and comparing after** — §11e; `1s << 34` overflows
    `int64` into a NEGATIVE duration, which schedules the retry in the past.
    **Do not give the relay a goroutine or a ticker** — §11d, `clock`'s call for `clock`'s reason.
    **Do not delete the empty-claim fast path when a mutant survives on it** — it is flagged in the
    code as semantically equivalent and deliberately kept (§11f).
    **Do not make `Payload` an `any`** — §11c; the committed bytes are what the hash chain covers.

15. ✅ **`platform/config`** (§10a–§10i) — `Decode`/`Store`/`Revision`, the default-deny document
    checks, the `config:"hot"` reload policy, `SecretRef` and `Duration`. 43 mutation runs, 37 claims,
    all caught. **Do not move §5.2's schema into it** — §10a records why, and both shapes were put to
    the user. **Do not make the reload policy default-allow, and do not replace the tag with a
    schema-owned `ValidateTransition`** — §10b records both rejected alternatives and the
    `apperror`-shaped defect the second one reintroduces. **Do not "simplify" the composite walk into
    a wholesale `DeepEqual`** — a hot rate-limit budget inside a non-hot venue map would stop being
    tunable (§10c). **Do not add a `Resolve` method to `SecretRef`** — a kernel type that can produce
    a secret is one that can log one (§10e). **Do not delete the duplicate-key or trailing-data scans
    without reading their measurement tests first** — they assert what `encoding/json` does instead,
    and will say so if it ever changes.

### Next work, in order

1. **`platform/observability`**, then `platform/ratelimit` (which will consume `apperror`'s
   `ClassRateLimit` and `RetryAfter` — note `outbox.Relay` already honours the latter, §11e), then
   `boot`. **The parity dispatcher is what `events` was the last prerequisite for** — but it also
   needs a persisted `DispatchSeq` log, so it is not simply "next" (§9b).
1b. **`platform/outbox`'s store half is OWED, and it lands with the OMS journal in milestone 3** —
   the `Append` verb, the SQL, the migration and the driver. It is deferred on §11a's argument, not
   forgotten. When it lands, the thing to check first is that the append and the journal write are
   genuinely one transaction, because everything in `platform/outbox` assumes it and nothing there
   can test it.
2. **`hexboundary`, `singlewriter`, `exhaustiveswitch` stay deferred** until a bounded context, an OMS
   write repository and a state machine exist to police. Unchanged reasoning (§1, §4).
3. **Confirm the ClientOrderID venue budget before milestone 3** (§8g) — 36 characters and
   `[A-Za-z0-9_-]` on Binance `newClientOrderId` and Bybit `orderLinkId`, browser-confirmed to the §4
   standard and recorded in `docs/venue-profiles.md`. Not needed to keep building; needed before a
   real order is placed.
4. **`apperror` has no venue-code → `Class` table, on purpose** (§8a). That mapping is venue-specific
   knowledge and belongs in `engine/venue` beside the client that produces the codes, where
   `venue-profiles.md` can be cited next to it. Build it there, not here.

### Standing lessons — apply these, do not rediscover them

- **One sentinel cannot say WHICH check fired, and this scales.** §8f found it once; session 8 found
  it four times in one table (§10h). Every schema-audit refusal returns `ErrSchemaField`, so four
  fixtures were each failing the check AFTER the one under test, and four mutants survived on it. The
  fix is to assert the **message**, because the message is the diagnosis somebody acts on. Note the
  wall: the obvious repair — build a fixture that passes every other check — is not always available.
  Giving an unexported field a json tag to isolate the exported-field check **does not build under
  `make ci`**, because `go vet`'s `structtag` analyzer refuses exactly that.
- **A guard is worth keeping only while what it guards against is real, so MEASURE it.** The
  duplicate-key and trailing-data scans carry tests that run the same bytes through `encoding/json`
  directly and assert it accepts them silently (§10d). If the standard library ever changes, the test
  says the guard is no longer load-bearing rather than leaving a check nobody can justify.
- **When a mutant survives on a check that has a twin, ask what the twin cannot see.** Removing a map
  entry is caught by a per-key presence walk; **adding** one is invisible to it, because every live
  key is still there (§10h). Two checks that look like one thing were guarding two different failures,
  and only one of them was tested.
- **A surviving mutant is sometimes telling you the code is redundant.** A whitespace pre-check in
  `decode.go` never fired in a way that mattered — the `io.EOF` branch below already covered it — and
  it was strictly worse, reporting "empty document" for a document with an invalid character in it.
  Deleted. Then the SURVIVING path was bite-checked in its place, because removing the redundant one
  is only safe if the remaining one is known to bite (§10h).
- **Ask which way a default fails when somebody FORGETS it.** That single question decided the reload
  policy (§10b) the same way it decided rule 4. Default-allow is noticed when a running process picks
  up a journal path mid-trade; default-deny is noticed when a reload is refused with the field named.
  The corollary is not universal, though: an unrecognised tag *value* is an error rather than a silent
  deny, because there denying is safe **and silent**, which leaves an operator believing a field
  reloads when it does not.

- **A fix applied in one place must be applied to its MIRROR.** `Receive`-prefers-queued was tested
  across 200 trials precisely because a select picks at random; `Send`-after-`Close` — the same
  situation with the same 50/50 — was tested once, and a mutant survived on the coin flip (§9i). When a
  test earns a repetition, a bound or an isolation trick, go and look for the symmetric case
  immediately, because the defect will come back through the other door.
- **The fixture-dimension lesson has now fired THREE times, so treat it as a checklist item rather
  than a war story.** §5d: 162 of 348 fuzz corpus entries reached the oracle. §9i: every coalescing
  test used a mailbox depth of 1, where "merge against the newest" and "merge against the oldest" are
  the same slot. §11f: every malformed-identifier fixture put its bad character mid-string, where
  `i >= 0` and `i > 0` are the same predicate — so `" audit"` validated while the check looked
  covered. **Before believing a validation suite, ask which fixture parameter is pinned to the value
  that collapses the distinction the code makes**, and vary it.
- **Check the FIXTURE dimension before believing the coverage.** Every coalescing test used a mailbox
  depth of 1, where "merge against the newest" and "merge against the oldest" are the same slot — so
  the branch was unreachable while looking covered (§9i). §5d's vacuous-run lesson generalises past
  fuzz corpora: *any* fixture parameter pinned to its degenerate value can collapse a distinction the
  code makes.
- **`go test` exits non-zero for a BUILD failure exactly as it does for a failing test.** A mutant that
  does not compile is not a killed mutant, and Go makes this easy to hit — deleting a call site orphans
  an import, which is a compile error. `tools/mutate.sh` now reports `NOBUILD!!`; the tell that found
  it was a `caught` with **0 failing tests**, which is a contradiction (§9i). Re-aim such a mutation to
  substitute a **wrong value** rather than delete a call.
- **A mutation can make a test HANG rather than fail, which wedges the whole run.** §5e's
  no-hanging-predicate lesson, arriving through the harness: swapping a non-blocking send for a
  blocking one parked the suite until the shell timed out. The harness now runs bounded and reports
  `TIMEOUT!!` — **not** a kill (§9i). Any test that deliberately fills a queue past capacity needs its
  own bound, or the mutation that breaks the queue reports nothing.
- **When a mutant survives, ask what the code is FOR, not what it does.** Deleting the UUIDv7 counter
  reset kept ids unique and ordered, which was everything the tests checked; what it broke was the
  timestamp's meaning, four thousand mints later (§8f). The test that caught it states the purpose —
  the counter is per-millisecond state — rather than a consequence.
- **A rejection test with one sentinel cannot tell you which check fired.** Deleting the
  ClientOrderID prefix check survived because the fixtures failed the *separator* check instead, with
  the same sentinel (§8f). To isolate a check, build the input that passes every other one.
- **Safe-direction bugs are the ones that ship.** `New` reading the cause's effect with `EffectOf`
  made every wrapped plain error ambiguous, so `EffectNotApplied` became unreachable — and nothing
  failed, because reporting everything as ambiguous is *conservative* (§8a). A default that is safe
  in one direction is still wrong; test that the other branch is reachable.
- **A test's expected value must come from somewhere other than the implementation.** The golden
  ClientOrderID was a hand-encoding and it was wrong; the `math/big` oracle found it (§8e). "Copy
  whatever the code prints" turns a golden test into an assertion that the code equals itself.

- **Every claim gets bite-checked before it is written down.** 21 mutations were applied across the
  session and **two did not bite** — both worth the diagnosis rather than the paper-over. One was a
  *semantically equivalent* mutation (unreachable from outside the package, pinned by an internal
  test instead); one was simply a **bad mutation** that failed to allocate. A surviving mutant is
  either a test gap or a wrong mutant, and telling which is the actual work.
- **Positive controls, always.** Any harness that reports "the broken thing failed" must also show
  the real thing passes, or a universally-objecting predicate reads as a perfect result. Corollary
  found the hard way in §4d: when a fixture changes role — `engine/venue` went from clean control to
  offending — **check whether it was the only positive control**, and replace it before moving on.
- **A mutation harness must prove the mutation applied — and that it applied *where you meant*.** A
  pattern that fails to match produces *exactly* the output of a caught mutation, so the script
  `cmp`s before and after and prints `NO-OP!!` rather than `caught` when nothing changed; it fired on
  its first run (§4d). Session 5 found the next layer: `cmp` proves *a* change, not *the* change. A
  `Quantity.Mul` mutation reported SURVIVED because the pattern occurred three times in the file and
  the substitution landed on `Neg` instead (§6b). **Scope the mutation to a line and print the diff**
  — the file moving is not evidence about which function moved.
- **A fixture can quietly agree with the thing its comment claims it rules out.** The analyzer sort
  test asserted in prose that a comparator reaching for the wrong field could not pass it, and then a
  mutant sorting by line number survived, because the fixture's line numbers happened to ascend
  alongside its positions (§6b). **A claim in a test comment is not tested unless the data can
  falsify it** — build the fixture so the decoy orderings actively disagree.
- **A tool that runs is not a tool that reports.** `gremlins` scored an untouched, healthy suite at
  **0.00% efficacy** because a warm Go test cache shrank the baseline it derives its timeout from
  (§6b). Same tree, cache cleaned: 92.51%. And it emits a **false LIVED about once in four runs**.
  Before believing any number from a new tool, find the input whose right answer you already know.
- **Write the oracle before trusting the implementation.** The one real defect this session was
  found by the two disagreeing, not by review.
- **Verify tooling semantics from source.** Twice now. `-rapid.seed`'s default turned out to be the
  opposite of a safe gate default (session 2), and `big.Rat.SetString`'s `|exp| > 1e6` cutoff
  reports a *parse failure* for well-formed text (session 3) — which, unread, would have made the
  fuzzer's first "find" an artefact of the two tools' differing limits rather than a defect.
- **A clean run and a vacuous run look identical, so coverage claims are measurements.** 30.4M fuzz
  executions with no find says nothing until you have counted how many corpus entries actually reach
  the interesting branch (§5d: 162 of 348). It recurred in `clock` exactly as predicted, one level
  deeper than expected: a *negative control* turned out to reach only one of its predicate's two
  clauses (§5e).
- **A predicate that can HANG on a broken implementation is a bad predicate.** `clock`'s first
  controls run hung instead of failing. A failing test names the defect; a hanging one reports
  nothing, blocks CI, and reads as an infrastructure problem. Every drain/retry loop in a control
  gets a bound and a legible overflow marker (§5e).
- **Tested logic plus untested wiring is still an untested guarantee.** `Wall`'s monotonic clamp had
  its own passing tests and survived having its store deleted, because nothing checked that the
  clamp was connected. If a guarantee cannot be reached through the public API, indirect the one
  dependency that blocks it and reach it from an internal test — do not settle for a doc comment
  (§5e).
- **The same question can deserve opposite answers in two implementations.** Backwards time is
  absorbed by `Wall` (no error to return; a stalled clock beats a re-evaluated settlement) and
  rejected by `TestClock` (it has an error, and in replay it means the dispatch log is out of
  order). Look for the error channel before choosing (§5e).

**Do not over-build.** Milestone 1 exists to make **milestone 2 — the economic gate — measurable**,
and §12.2 is an explicit kill gate the spec predicts BTC/VIP0 may fail. The OMS is milestone 3.

**Standing rules:** build test-first, red shown before green, per operation; **no AI attribution** in
commits or any authored content.
