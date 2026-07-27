# Milestone 1 — foundation decisions (2026-07-26)

**Status:** in progress — and **this line is not where to read it**. It described session 3 for sixteen
sessions after session 3 ended, naming a head commit that no longer exists, which is why the live
statement lives at **`## RESUME HERE`** at the bottom of this file (newest first) and the plan lives in
`docs/HANDOFF.md`. As of session 19: milestone 1's first **four** slices are closed, the doc set is at
**v0.9.9**, and nothing is owed. Branch `feat/milestone-1-foundation`, pushed — verify with
`git rev-list --left-right --count @{u}...HEAD` rather than believing a sentence about it.
**Doc set:** implements `build-spec.md` §12.1. Deviations from the spec are marked `[SPEC EDIT OWED]`
while open and `[SPEC EDIT LANDED — vX.Y.Z]` once the spec has caught up.

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

### 9h. One [SPEC EDIT LANDED — v0.9.6], following the v0.9.5 precedent

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

### 10b. "Hot-reloadable where safe" is default-deny `[user decision]` `[SPEC EDIT LANDED — v0.9.6]`

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

### 11b. Per-key FIFO, scoped `[SPEC EDIT LANDED — v0.9.6]`

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

## 12. The second slice continued (session 10) — `platform/observability`

§10's metrics and structured logs, on §3.4's isolated lossy lane. Eight files, 45 tests, and the
kernel is **eight packages**. Two scope questions were put to the user before any code was written,
because both are durable and neither is recoverable cheaply later.

### 12a. The Prometheus adapter is deferred — the user's call, with the alternative stated

**Decision: build the ports, the descriptor discipline and an in-memory meter now; the Prometheus
adapter lands with `platform/boot` / `app/`.** The engine module therefore still has **zero
production dependencies** — only test-only `pgregory.net/rapid`.

This is not the same argument as §11a's outbox store, and it is weaker, which is why it was asked
rather than assumed. §11 names `prometheus/client_golang` and §3.6 rule 9 scopes `kernelpurity` to
in-module imports **specifically so this package may take it**, so the dependency is sanctioned. Two
reasons to wait anyway:

1. **Nothing under `internal/` emits a metric yet**, so a registered exporter is wiring that no test
   can exercise end to end — the same "a lint over an empty tree passes trivially" reasoning that
   defers `hexboundary` and deferred the outbox store.
2. **Which binary exposes `/metrics`, on which interface, is a composition-root decision.** §11 gives
   `arblab-actuator` *no dashboard and no general HTTP surface*, so a kernel that mounted a
   `promhttp` handler would be wrong there and could not be argued out of it.

**Rejected alternative:** take the dependency now and ship a Prometheus-backed meter alongside the
in-memory one. Its real merit — proving the `Spec`→collector mapping against the actual backend
rather than on paper — is genuine and was stated to the user as such. It loses on the observation
that the HTTP surface defers regardless, so only part of the wiring would be proven, while the
repo's first production dependency lands permanently.

**What is NOT deferred is the float64 conversion** (`export.go`), and that distinction is the
substance. It does not belong to any one client library: the Prometheus text and protobuf formats,
OTLP and every other wire metric encoding carry a *double*. Whatever adapter arrives needs exactly
that function, and if it is not already here the adapter writes its own — inline, at the call site,
without the exactness argument — which is how a rounding decision stops being a decision.

### 12b. The port is integer-typed, and this is `outbox.Append` in a new place

`prometheus.Counter` has `Add(float64)`. **A port that mirrored it would be the §11a failure exactly:
it compiles, it reads as the metrics contract, and it silently dismantles §3.6 rule 4.** Every call
site reporting a fill count, a realized PnL or a funding rate would have to produce a `float64`, so
rule 4's *single sanctioned metric seam* becomes a `//nofloat:allow` at every instrumented line in
the system — each written by somebody thinking about a dashboard rather than about rounding.

So `Counter.Add`, `Gauge.Set` and `Histogram.Observe` take a `Sample`: an `int64` mantissa at
`fixedpoint`'s canonical scale, tagged with the unit it was converted into. `float64` appears in one
file. **The gate proves the confinement rather than the comment claiming it** — verified by a positive
control: deleting one escape makes `arblab-lint` fail on that exact line.

**`Sample` carries its unit, and the mismatch is a runtime error naming both.** The defect this
prevents is not exotic — it is observing `d.Milliseconds()` into a histogram whose buckets are in
seconds, which compiles, produces plausible numbers, and is wrong by three orders of magnitude on
every panel and alert threshold derived from it. Money samples additionally carry the **asset**, and
that is `fixedpoint`'s own runtime tag check extended one layer out: `Money` is a distinct
compile-time category so a `Quantity` cannot reach a money gauge, but BTC-`Money` and USD-`Money` are
the same Go type, and the one that lands on the USD PnL panel is wrong by a factor of the BTC price
with nothing failing.

**One conversion is exact, and it is pinned rather than trusted.** `time.Duration` counts
nanoseconds and `fixedpoint.Scale` is 9, so a duration's nanosecond count *is* its seconds mantissa —
no multiply, no rounding mode. That is a coincidence a later scale change would break silently (every
duration wrong by ten, in the same direction, on every panel — which reads as a system that got
slower). Two `uint` constant expressions pin `Scale == 9` from both sides, because a one-sided bound
on an equality is the kind of check that passes while the thing it guards is broken.

### 12c. Cardinality is the fourth `[SPEC EDIT LANDED — v0.9.6]`

§10 requires metrics cut per venue and per symbol and says nothing about label cardinality. §3.4
states the observability lane's isolation as *"back-pressure here never blocks trading"* — a
guarantee about a stalled **consumer**. Neither covers the way a metrics layer actually takes a
trading process down.

**A metric labelled by `ids.ClientOrderID` allocates one time series per order, forever, in the
trading process's own heap. Nothing stalls.** Every scrape succeeds, the lane reports zero drops, and
§3.4's guarantee holds perfectly for the entire duration of the outage it fails to prevent. This is
the §11b shape again: the spec requires labelled metrics and relies, unstated, on somebody bounding
their domains.

The code makes it explicit and **default-deny**: a `Label` declares its permitted `Values`, or writes
a `Unbounded` **reason**. Same shape as `//nofloat:allow <reason>` and `config:"hot"`, same purpose —
the person adding the risk writes down why they are entitled to. `MaxLabelValues` bounds even the
declared sets, because declaring ten thousand members satisfies the letter of the discipline and none
of its intent, and the *product* across labels is what sizes the registry.

**Rejected alternative:** enforce nothing and document the hazard. It fails the house rule about
which direction a forgotten default should fail in — the failure would be a process OOM under load,
not a refusal naming the label.

**A deliberate second-order refusal:** `Values: []string{}` is an error (`ErrLabelValuesEmpty`) and is
NOT read as unbounded. That reading would hand out the escape without the written reason, to a caller
whose enumeration simply came back empty.

### 12d. The logger exists because `slog` reads the wall clock where no seam reaches

`slog.Logger.Info` and its siblings call `time.Now()` internally when they build the `Record`. It is
not reachable through a `Handler`, an `Attr`, or any option. That is an ambient clock read on the far
side of §3.5's parity seam: a backtest replaying a `TestClock` through 2024 emits log lines stamped
with today's wall clock, and the log then cannot be joined to the replayed dispatch it describes,
which is the one thing a replay log is for.

**Note what does not catch this.** §3.6 rule 5's `notimenow` polices `domain/`, `application/` and
`engine/` — `policedLayers` was read to confirm it, not assumed. `platform/` is deliberately outside,
because `platform/clock` is where the ambient read has to live. **So the kernel is exactly the region
where an ambient clock read is invisible to the gate**, and the only defence is that the code does not
make one. `Logger` builds `slog.NewRecord` from `clock.Now` itself; everything else is a thin pass to
the underlying `slog.Handler`, so the stdlib's JSON and Text handlers work unchanged.

`BufferedHandler` is the **deliberate mirror of `events.ObservabilityLane`** — never blocks, drops the
**newest**, exact never-reset drop count — and the symmetry is the argument, not a coincidence: same
lane, same question, and a log line is not superseded by a later one, so refusing the newcomer is also
the choice that touches nothing already accepted. **No internal goroutine**, deliberately: a channel
and a drain goroutine would give the kernel a lifecycle to own (started by whom, stopped in what order
relative to the control lane's drain, leaked if a caller forgets). `Drain` is a periodic sweep the
caller schedules, exactly as the lane offers `TryReceive` and no blocking `Receive`. The mutex is held
across a slice append and never across a syscall.

### 12e. `NopMeter` validates, and that is the whole design decision

The obvious no-op meter accepts everything and returns nil. It would be smaller, faster, and it would
**make every test written against it vacuous**: a descriptor production would refuse — a missing unit
suffix, an undeclared label set, a bucket in the wrong unit — passes under test, and the failure
arrives at boot in the process that matters. So it runs the identical validation path and differs only
in retaining nothing. A test drives both meters through the same refusals and asserts the **error
strings match**, so the two cannot drift into disagreeing about which descriptors are legal.

### 12f. Bite-check — 55 runs, 54 caught, one documented equivalent

Both harness guards fired and both were right. Two **`NOBUILD!!`**, one of them on the package's
headline claim — `l.clock.Now()` → `time.Now()` does not compile, because `logger.go` has no `time`
import, so the mutation proved **nothing** while printing as a kill would have. Re-aimed per §9i's own
advice to substitute a wrong value: `l.clock.ServerNow(clock.Venue("")).Instant()`, which yields the
zero instant for an uncalibrated venue, compiles, and proves the stamp comes from `Clock.Now`
specifically. One **`NO-OP!!`** on a stale line number after an edit.

Four survivors were real gaps, and **the recurring fixture lesson fired for a fourth time**:

| Survivor | Diagnosis |
|---|---|
| `Drain` reslices instead of allocating | **Gap.** The test drained and asserted immediately, where aliasing is invisible. The real failure needs the sweep to still be *holding* the slice when the next records arrive — an encoder reading records that later logging overwrites. |
| Key separator changed to `_` | **Gap, and the fixture was the problem.** `("ab","c")` vs `("a","bc")` collide only under the *empty* separator, so the test passed against every wrong choice a person would actually make. Label **values** are not character-checked — only names are — so the fixture now drives a colliding pair per candidate separator (`_ . - : / | , ` and space). |
| `addChecked` underflow side | **Gap.** `Counter.Add` refuses a negative delta, so a counter never exercises it. A **histogram's** sum does, because negative observations are permitted (cross-leg skew is signed) — testing only the overflow side is a half-checked guard on a symmetric predicate. |
| Empty string admitted into a declared value set | **Gap.** `permits("")` would then return true, and `""` is what a caller passes when the value was never populated — every unlabelled observation merging into one series. |
| `r.Clone()` → `r` | **Equivalent, and now named in the code.** Probed rather than argued: the only mutation available to a caller is `AddAttrs`, which *appends*, and an append never overwrites an element the retained copy can already see. It becomes observable only when two records derive from one base and both write the same shared slot — a shape no current caller produces and any future one might, so the `Clone` stays on the contract rather than on a reachability argument a later edit would silently invalidate. |

**The fixture lesson, fourth occurrence.** Fuzz-corpus reach, mailbox depth of 1, a bad character
never at index 0, and now **a separator-collision fixture that contains no separator**. The
`TestBufferedHandlerDropsTheNewest` fixture was written at capacity **3** for this reason — at
capacity 1, "drop the newest" and "drop the oldest" are the same slot.

**Deferred with an argument, not forgotten:** tracing. §10 lists it beside metrics and logs. A span is
a claim about a causal chain, and this system's causal chain is §3.5's `DispatchSeq` — which does not
exist until the dispatcher does. A tracing API built now would model goroutine call trees, which is
the wrong spine, and it would be load-bearing by the time the right one arrived.

---

## 13. The second slice continued (session 11) — `platform/ratelimit`

§7.6's header-aware governors, plus the reserved allocation §6.7, §7.1 and §7.5 spend when the
system is recovering rather than trading. Eight files, 42 tests, and the kernel is **nine packages**.
**46 hand mutations, 46 caught**, two documented equivalents.

Two questions were settled by precedent before any code was written, and stated here so a later
session does not reopen them:

- **Machinery, not a venue table.** `config` §10a kept §5.2's schema out of the kernel and `apperror`
  §8a kept the venue-code → `Class` table out; a `Profile` is caller-supplied data for the same
  reason. Here the argument is **stronger than precedent**, and that is worth recording:
  `venue-profiles.md` records the rate-limit **header names** as verified facts and does **not**
  record the numeric budgets. A table in this package would have to be populated with numbers nobody
  verified, in a doc set whose standing rule is that nothing goes in unverified.
- **Non-blocking, no goroutines.** `Acquire` is a decision, not a wait. §6.3 makes rate-limit budget
  one of the things `risk.Gate` re-validates before a trade — a *predicate*, which a call that
  blocked until budget appeared would answer by hanging. And a blocking governor needs a timer,
  which needs a goroutine and an ambient clock: `clock` (§5e) and `outbox.Relay` (§11d) both refuse
  one, because the instant a retry becomes eligible has to be reproducible in a replay.

### 13a. The ladder — the milestone's second user question **[SPEC EDIT LANDED — v0.9.6]**

**Decision, put to the user and taken: `Capacity` is a hard ceiling for EVERY priority.
`PrioritySafety` gets an exclusive band, not a bypass.**

§7.6 says "back off at ~70% weight" without saying *what* backs off. §7.1 says the margin top-up
gets "a reserved rate-limit allocation so it is not starved by trading traffic". §6.7 says an
ambiguous residual holds new trading and reserves budget for recovery. Those are **one mechanism
stated three times in three sections**, and what is missing from all three is the priority ordering
that joins them:

```
used ─────────────────────────────────────────►
         │ Opening  │ Routine │ Safety │ ✗ refused
         0        70%       90%     100%
                            └─ reserve ─┘
```

Two things in that picture are not in the spec, and both are load-bearing.

**One: monitoring outranks new exposure, and that is not the obvious ordering.** Shedding
market-data and account reads to protect trading traffic would make the rate limiter itself the
cause of a stale mark — which is §7.1's own escalation trigger — so the governor would fire the
pre-authorized de-risk it exists to help avoid. `PriorityOpening` is shed first, and it is the same
act §5.3 already names `stop-opening`, so budget pressure and a stop-opening condition degrade the
system the same way rather than two different ways.

**Two: safety stops at capacity.** *Rejected alternative, stated to the user:* let safety bypass the
local ceiling and be refused only by an **observed** ban. Its merit is real — the local count is an
estimate, and refusing a flatten on an estimate converts a throughput guess into open directional
exposure. It loses on three counts. Past capacity the venue is expected to **refuse**, so an issued
flatten is not a flatten. On Binance repeat violations escalate a 2-minute ban toward **three days**,
so pushing lengthens the outage being pushed against. And the spec **already has a designed answer**
for "cannot act at the venue" — §7.4's pre-placed venue-native protection and the off-host actuator.
A governor that spent past the budget would substitute wishful throughput for that ladder, silently.

The ceilings are declared as **absolute counts, not percentages**. A fraction of a count is a
division this package would have to round, and every rounding decision in this repo is an explicit
argument (§3.6 rule 4); the profile that already knows the capacity does the arithmetic once, in the
open. `Limit.Reserve()` names the band rather than leaving it a subtraction, because it is the
quantity an operator has to size against a real incident's concurrent query volume.

### 13b. Header-aware means the venue's count wins, in ONE direction

`Observe` folds a venue-reported consumption in **upward only**. A reading is always stale: it
describes the server at the moment it processed some earlier request, with others in flight behind
it. So a reading **below** the local count is exactly what staleness looks like, and adopting it
would discard the record of every request sent after the one that carried the header. A reading
**above** it is information there is no other way to get — an endpoint whose weight we
under-estimated, or another consumer on the same IP.

The two failure directions are not symmetric, which is what settles it: ignoring a legitimate
decrease costs throughput until the window rolls; adopting an illegitimate one costs a ban.

The correction is applied as a **synthetic charge at the observation instant** rather than as a
separate floor. One mechanism instead of two, no floor/log reconciliation to drift, and it ages out
conservatively — the real usage expired earlier than the correction models it expiring.

`Penalize` carries the same ratchet twice over: a venue `RetryAfter` hint is a **floor, not a
ceiling** (the longer of hint and configured pause wins, because a hint that undershoots puts us
straight back into the ban being served), and a later, milder penalty neither shortens a running
pause nor downgrades its recorded kind.

### 13c. Two mechanisms, because there are two failure modes

A family's weight budget is **headered**, so local counting is an estimate that gets corrected. The
venue-scoped IP envelope is **not**: Bybit's ~600/5s gateway ban (retCode 10018) arrives with no
header at all, and §7.6 states in its own words that IP-ban-track separation between Binance's spot
and futures budgets is "**inferred, not documented**". For those, local counting is the only defence
there will ever be. Hence a plain request counter at venue scope beside the weight buckets at family
scope, and hence `Cost` having **no `Requests` field**: every `Acquire` charges exactly one to every
`DimensionRequests` limit, so the one budget with no header to correct a mistake is also the one a
caller cannot forget to charge.

A pause is likewise **venue-wide across every family**, and that is §7.6's hedge rather than a
simplification: scoping a pause to the family that earned it would encode exactly the separation the
spec declines to assume.

### 13d. `ids.ProductCategory` is the API-family key, and the fit is not a coincidence

§7.6 governs per `(venue, API family)`. On Binance the product families **are** separate API hosts
with separate budgets (spot, USDⓈ-M, COIN-M), which is precisely the distinction §5.1 already draws
for an instrument — so reusing `ProductCategory` means an adapter naming a family and an adapter
naming an instrument cannot drift apart. `Profile.Families` is nonetheless declared **explicitly**
rather than derived from the limits, so that a family with only the venue envelope is still a family
a request may legally name, and a **typo is not**: without it an undeclared family would match no
family-scoped limit and sail past every product budget, charging only the envelope — a request that
looks governed and is not.

### 13e. A sliding window, deliberately stricter than the venue's own fixed one

Binance meters over a fixed interval aligned to a server boundary we cannot observe. Modelling it as
fixed locally means guessing that alignment, and a wrong guess lets a burst land at the end of our
window **and again at the start of the next** — twice the intended rate across the seam, from a
limiter that looks correct. A sliding window cannot produce that seam: budget returns only once a
charge has been held for the whole interval. It errs by refusing.

`used` is **re-summed on eviction rather than decremented**. Subtracting is O(1) and accumulates
drift the moment any other path adjusts the total — and there is such a path, because `observe`
folds in a venue reading. A counter that drifts low is a governor that admits past capacity while
reporting that it did not. The cost is bounded by the budget itself: a charge is recorded only when
it was admitted, and admission caps the total at `Capacity`.

### 13f. Three smaller decisions whose absence would be quiet

- **A refusal charges nothing.** All-or-nothing across every applicable limit. Partial charging would
  mean a request the order budget turned away had still spent the weight budget that admitted it, so
  a caller retrying in a loop would drain budgets on requests it never sent — fastest exactly when
  the venue is most constrained.
- **`NotBefore` is the LATEST of every refusing limit**, not the earliest. Naming the earliest sends
  the caller back to be refused by another, turning a backoff into a poll against a venue already
  declining traffic. The zero time means "**not by waiting**" — the charge exceeds the ceiling at any
  consumption — and it has to dominate every real instant, which is why the comparison is a named
  helper: `time.Time{}.After(anything)` is false, so a never-releasable limit written inline would
  silently lose to every other refusal.
- **`Decision.Err()` is `EffectNotApplied`, and this package is uniquely entitled to say so.**
  `apperror.RateLimited` fixes that for a *venue* refusal; a *local* refusal is stronger evidence
  still, because nothing was sent. §5.3 forbids inferring "not submitted" from a venue's not-found —
  this is the one place where it is not an inference.

### 13g. Bite-check — 46 mutations, 46 caught, and one real defect found by the exercise

**The defect was found before a single mutation ran**, by working out what each mutation *should*
catch. `Penalize` read "was a pause already running?" **after** applying the new pause — at which
point it is a constant, because the hold is always positive. The clause was dead, its purpose had
silently migrated to the severity comparison, and the live consequence was that a ban which expired
last week kept outranking the throttle running now: `Snapshot.PauseKind` would report `PenaltyBan`
for a pause that is not one, and §7.6 alerts on exactly that. Fixed by reading the flag first, and
two tests now pin both directions (a ban that expired then a throttle; a throttle then a ban).

`PenaltyKind.moreSevereThan` was also narrowed from `>=` to `>` on the same pass, because
re-assigning a kind over itself changes nothing — the two forms are behaviourally identical, and
only one of them can be shown to bite.

**Five mutants survived the first pass. Three were real gaps and two are equivalent.**

- **The fixture-dimension lesson fired for the FIFTH and SIXTH time, in one session.** Two survivors
  were the same collapse: `TestNotBeforeIsTheLatestOfEveryRefusingLimit` and the never-releasable
  test both used a profile where the worst refusal happened to be **first in profile order** — and
  limits are evaluated in profile order, so "name the first refusal" and "name the worst refusal"
  were the same answer. A dedicated `ladderedProfile()` now puts the slower-freeing budget
  **second**, and both mutants die. Then `freeAt`'s already-fits boundary survived because every
  case in its table sat strictly inside or strictly outside the ceiling, where `<=` and `<` are the
  same predicate. **The tell is identical every time: the fixture does not contain the thing under
  test.**
- **A survivor reported redundant code rather than a missing test** (§10h's lesson, again).
  `Limit.key()` special-cased `ScopeVenue` to zero the family — but `Validate` already refuses a
  venue-scoped limit that names one, so the branch could not fire. Deleted, and the surviving path
  bite-checked in its place.
- **Two documented equivalents, both flagged in the code as expected survivors.** `Reserve`'s
  `>=` narrowed to `>` and `admits`'s `used >= ceiling` narrowed to `>` are each equivalent at
  exactly the boundary, because the subtraction already yields zero there. In both cases the guard's
  real content is the strictly-above case, so both are bite-checked by **deleting** the guard
  instead — and both die that way.

**Also worth recording: `addSaturating` had no test at all**, because nothing in the behavioural
suite drives a `uint64` anywhere near the top. It exists for one input class this package does not
get to bound — a venue reading — and the direction is the point: wrapping turns an absurd reading
into a *small* used value, which admits traffic. It now has both a direct table and a window-level
test asserting the refusal.

---

## 14. The second slice continued (session 12) — `platform/boot`

§3.2's bootstrap phases and §7.5's reconcile-only boot. Seven files, 54 tests, and the kernel is
**ten packages** — the last one §1's deferred table names. **62 hand mutations, 62 caught**, no
survivors after diagnosis. Three `[SPEC EDIT OWED]`, taking the slice's total to **eight**.

The package imports `platform/clock` and nothing else. It is the first kernel package whose subject
is the *process* rather than a value, and the two questions that shaped it were both about type
signatures rather than algorithms.

### 14a. Reconcile-only is a POSTURE, not a stage — the defect the obvious shape has

**Decision: `Sequence.Run` and `Checklist.Posture` return a four-valued `Posture`, not an error.**

§7.5 reads as a sequence, and the obvious implementation follows the prose: a list of steps, each a
`func() error`, trading starts when the last returns nil. That shape is wrong, and it is wrong in
the *signature* — nowhere a test would look.

A `func() error` has two outcomes. §7.5 has three. The third is **alive and refusing**, and the spec
states it three separate times without ever naming it: foreign orders and unexplained position
deltas "alert for operator clearance" and are "not auto-corrected"; an `Ambiguous` MarginOp blocks
new openings "until the staged-float ledger is explicitly reconciled"; and §7.4 puts a bot that
recovers mid-takeover in reconcile-only "until an operator clears it". None of the three is a boot
failure and none is a boot success.

Collapsing it costs money in **both** directions, which is what makes it worth a type rather than a
convention:

- **Folded into failure**, the process exits. A condition the spec answers with "alert an operator
  and keep monitoring" is instead answered by making §7.4's actuator cover a host that died
  *deliberately* — with the exposure it was watching now unmanaged by anything on the machine.
- **Folded into success**, trading resumes over an unexplained position delta, which is §7.5's own
  example of the thing that must not be auto-corrected.

So the ladder is `PostureHalted` → `PostureReconcileOnly` → `PostureStopOpening` → `PostureTrading`,
ordered by permissiveness with the **zero value the most restrictive**, and `Permits` is one `>=`.
Four rather than a boolean because §7.5 states two *different* refusals in one paragraph — "new
execution disabled while state is rebuilt", and an Ambiguous MarginOp blocking "new openings (not
safety actions, which read live state)". A boolean may-I-trade collapses them, and again both
directions cost: treating an ambiguous margin op as a full stop withholds a flatten the spec permits,
and treating a mid-reconcile boot as stop-opening admits a reduction sized from a position the
process has not finished rebuilding.

`Act` is deliberately three values and **reads are not one of them**. §7.5's own wording — safety
actions "read live state" — makes reading the thing that survives when writing does not, and §7.4's
watcher observes with no trading key at all. A posture able to withhold a read could blind the
monitoring that decides when to escalate.

**Do not collapse `Posture` into a boolean, and do not add a read `Act`.**

### 14b. §7.5's gates latch or they don't, and the spec never says which **[SPEC EDIT LANDED — v0.9.6]**

**Decision: `OutcomeNeedsOperator` is latching. Only `Clear` lifts it, and `Clear` returns the gate
to `OutcomeUnreported` — never to `OutcomePassed`.**

§7.5 gives every one of its preconditions the same verb — asserted before trading resumes — and
states, *in the same paragraph*, two conditions that do not clear the same way. A journaled MarginOp
resolves itself through its evidence contract. A foreign order or an unexplained position delta
"alert for operator clearance" and are "not auto-corrected". §7.4 then makes the second kind explicit
for the fence: return to normal operation requires the terminal outcome, "explicit operator
clearance, **and a fresh `VerifyFence` propagation test** — never health-flap re-entry".

So the spec has latching preconditions and self-clearing ones, and nowhere says the distinction
exists. A uniform checklist would let a gate that failed and then passed on its own retry re-admit
trading — **which is exactly the health-flap re-entry §7.4 forbids, arriving through §7.5's door.**

The `Clear`-to-`Unreported` half is the one that is easy to get wrong in the expensive direction.
§7.4 requires clearance *and* a fresh test, so clearance restores the gate's **ability to pass**, not
the pass. Clearing straight to `Passed` would turn an operator acknowledging an alert into an
assertion about venue state they never made.

**Do not make `Clear` pass the gate**, and **do not make every outcome freely re-reportable.**

### 14c. The gate roster is a closed vocabulary — where the "machinery, not a table" precedent stops

`ratelimit` §13, `config` §10a and `apperror` §8a all refused to hold data. `boot` holds §7.5's seven
gate names, and the precedent genuinely does not carry: those packages refused data the spec
**deliberately leaves to the caller** — venue budgets `venue-profiles.md` does not record, risk caps
whose meaning belongs to the owning context. §7.5 is the opposite case. It *names* its preconditions
normatively and the list is closed, so the roster is spec content, and holding it here is what lets
an unreported gate be a refusal that says **which one**.

What is still not here is *how* any gate is established. The journal join, the net-delta computation
and the §4.1 posture REST calls belong to `oms`, `risk` and `engine/venue`.

`GateWriterAuthority` is the one gate §7.5 does not list — §7.4 does, from the other end. It is also
the gate whose clearance rule differs from the rest, which is how §14b was found.

### 14d. The ordering constraints — the milestone's third user question **[SPEC EDIT LANDED — v0.9.6]**

**Decision, put to the user and taken: enforce the constraints §3.2's order implies, with an open
extension slot. Rejected: a closed five-value total order, and documentation only.**

§3.2 states the roster in a table cell — "signal, config, obs, reconcile-only-boot, graceful
shutdown" — as a list. Two of those positions are load-bearing, and **both failures are invisible
when they happen**:

- **Signal is first** because a Go process's default disposition for SIGINT and SIGTERM is to *die*.
  Installed after a forty-second journal join, an operator's interrupt during that join terminates
  the process outright — no graceful shutdown, no flush, no record that a shutdown was requested.
  The window where a boot is slowest is the window where an operator is most likely to interrupt it.
- **Observability precedes anything that can fail** because every later phase reports its failure
  through the machinery this one installs. Started late, a boot that dies in the journal join does
  not merely lose telemetry — it dies **silently**, leaving an exit code as the diagnosis.

The rejected alternatives, with why:

- **A closed five-value total order** was stronger but unwireable: §11 gives the three binaries
  genuinely different phase sets, and a venue client or a WebSocket subscription would have to be
  smuggled into a bucket that misnames it. `arblab-actuator` has no HTTP surface at all.
- **Documentation only** matched the "machinery, not a table" precedent most literally, and left both
  silent failures above to code review.

`KindService` is the open slot; `KindReconcile` is optional (a backtest run has no venue truth to
join to) but must be last if present. `KindSignal`/`KindConfig`/`KindObservability` are required, and
that carries a **falsifiable claim** in the spirit of `notimenow`'s escape: if a binary lands that
legitimately has none of one of them, relax the rule rather than declaring a phase that does nothing
— a required check satisfied by a stub is worse than no check.

Every refusal names the phase **and** the rule, because four constraints share `ErrPhaseOrder` and
§10h already paid for a sentinel that could not say which check fired.

### 14e. The shutdown budget is a safety number, not a timeout **[SPEC EDIT LANDED — v0.9.6]**

§7.4 gives the watchdog a host-silence trigger. §7.5 gives the process a graceful shutdown. Nothing
in the spec joins them — but they run against the same wall clock, and the join has a wrong side.

**A total stop budget at or above the silence threshold makes an ordinary restart indistinguishable,
to the watchdog, from a dead host.** It escalates, the actuator fences the bot's signer mid-flush and
latches a takeover epoch, and the process that comes back up finds itself fenced and — §7.4 — "stays
in reconcile-only until an operator clears it". **A deployment becomes an operator-cleared incident,
and the cause is a sum of durations in a wiring file that nothing was adding up.** `Sequence.Validate`
adds them up, and refuses at `>=` rather than `>`: at exactly the threshold the watchdog fires.

`WatchdogSilence` is required — unset is refused rather than read as unbounded, the standing
forgotten-field rule.

### 14f. Every closer runs, including after the budget is gone

The obvious design — stop calling closers once the budget is spent — is wrong here in a way specific
to **reverse order**. Reverse order means the closers that run *last* are the ones that started
*first*: the journal, the outbox, the log handler with records still buffered. A shared budget
drained by whichever venue client hangs first would skip precisely the closers whose work is the
reason a graceful shutdown exists — silently.

Skipping also **buys nothing**. This package has no `os.Exit` and no goroutine to abandon a wedged
closer on, so a skipped closer does not shorten the wall clock by one millisecond; it only means the
flush did not happen while the process sat there anyway.

So the bound is **declared, allotted and accounted for — not enforced**, and the package says so
rather than pretending otherwise. The enforcement it lacks lives where an operator can reach it:
`SignalImmediateHalt`. **Do not add an early exit to `Shutdown`, and do not give it a goroutine.**

### 14g. Why handing an operator a kill switch is the SAFE direction here

The instinct in a trading system runs the other way, so it is recorded. The signal policy escalates:
first interrupt → graceful shutdown, **every** subsequent one → immediate halt (ratcheting rather
than counting to two — an operator who has pressed twice and seen nothing presses again, and a
policy that counted to exactly two would answer the third press with the graceful path it has
already failed to complete).

That is safe because **host death is a covered state**: §7.4's off-host actuator exists precisely to
flatten exposure after the bot host and its journal die. A process that is *alive but wedged in
shutdown* is **not** covered — the watchdog reads liveness, so it will not escalate, and the operator
cannot fix it. Between a covered failure and an uncovered one, the second interrupt should produce
the covered one.

Two smaller decisions in the same file:

- **SIGINT and SIGTERM are deliberately not distinguished.** A container runtime sends SIGTERM then
  SIGKILL on its own schedule; treating the two stop signals differently would make the graceful path
  depend on which arrived.
- **`SignalBufferMin = 2`, and `Install` refuses a smaller channel.** `os/signal` never blocks on
  delivery — a send to a full channel is **dropped, with nothing reported**. That makes the buffer a
  correctness property, and it fails in the worst place: the second interrupt is *by construction*
  the one that arrives while the first is being handled, so an unbuffered channel drops exactly the
  signal an operator presses twice to send — the escape hatch of §14f goes missing.

### 14h. What is deliberately absent — the §11a line, drawn again

The journal is milestone 3, so absent **as a decision**: the join itself (`OrderByClientID`, `Fills`,
`Positions`, `FundingLedger`), the write-ahead journal replay, the net-delta computation and the §4.1
posture REST calls.

A `Reconciler` port here would compile and would read as §7.5's contract, and it would be a shape
guessed at with nothing to check it against — its arguments are the journal's cursors, which do not
exist. Worse than `outbox`'s rejected `Append`, in one respect: it would make the checklist look
**satisfiable**, since an implementation returning nil would report every gate `Passed` with nothing
joined, and the boot would be green.

What *is* buildable is everything that decides what the join's result **means** — the gates, their
outcomes, the clearance rule, the posture ladder, the ordering constraints, the teardown bound — and
none of it changes when the journal lands.

### 14i. Bite-check — 62 mutations, 62 caught, and four findings from the exercise

53 caught on the first pass; the other nine were the interesting ones, and none was a wrong mutant.

- **Two `TIMEOUT!!`s, and they were a real test defect.** Breaking the subscription — dropping SIGHUP
  from `Watched()`, and removing the `signal.Notify` call outright — made `TestInstallDelivers`
  **hang** on an unbounded channel receive rather than fail. §5e's lesson arriving through the
  harness for the second time: *a hanging test names no defect*. The two claims that file makes about
  `Install` actually subscribing were **unchecked while looking covered**. `recvSignal` is now a
  bounded receive, and both mutations kill.
- **One `SURVIVED`, and it was neither a gap nor a wrong mutant.** `minPostureFor`'s final
  `return PostureHalted` is unreachable from `Permits`, which validates the act one line earlier —
  but Go requires the function to return on every path, so it is not deletable either. Pinned by an
  internal test that calls it directly, the same treatment `fixedpoint` gave its unreachable-by-guard
  branches. The *direction* is what the test asserts: an act nothing recognises must map to the
  posture that permits nothing.
- **Five `NOBUILD!!`s, all mine.** Every one was an `if <condition> { → if false {` that orphaned the
  variable the condition read. Session 7's standing lesson, re-learned mechanically: **re-aim to
  substitute a wrong value rather than delete a use** (`if current != X` → `if current != current`,
  `if i < obs` → `if i < obs-100`). All five kill once re-aimed.
- **One redundancy found before mutating, by predicting the verdict.** `Install` had an explicit
  nil-channel guard that is unreachable: `cap` of a nil channel is 0, so the buffer check refuses it
  first — and "buffer 0, minimum 2" is a *true* statement about a nil channel. Deleted, with the
  surviving path bite-checked in its place. Likewise two lines in `unwind` (`r.started = nil`,
  `r.done = true`) that no caller can observe, since every caller is a `Run` failure path returning a
  nil `*Running`.

**The fixture-dimension checklist was applied in advance this time, and it changed five fixtures:**

| Fixture | What was pinned to the collapsing value |
|---|---|
| Reverse teardown | **Three** closers make "reverse" and "swap the ends" the same order. Five. |
| `Checklist.Posture` | Every gate reported ⇒ the default-deny path is unreachable. One gate is now never reported. |
| `Checklist.Posture` again | The worst outcome declared **first** ⇒ "name the first" and "weakest wins" agree. It is declared last, and the mirror order is asserted too. |
| `Blocking()` | Gates declared in sorted order ⇒ "sorted" and "declaration order" agree. Declared descending. |
| `Overran` / watchdog budget | Every case strictly inside or outside ⇒ `>` and `>=` agree. Both now have an **exact-equality** case, in opposite directions: a closer at exactly its budget has *not* overrun; a shutdown budget at exactly the silence threshold *is* refused. |
| Signal ratchet | Two interrupts ⇒ "count to two" and "ratchet" agree. A **third** interrupt is sent. |

Seven now, and the tell was constant again: **the fixture did not contain the thing under test.**

## 15. The second slice CLOSED (session 13) — the eight spec edits landed as v0.9.6

The fork §14's "Next work" left open was decided **(a): land the eight `[SPEC EDIT OWED]` items as
one release**, on the slice's own convention — the v0.9.5 precedent batches a slice's edits into one
coherent spec change at slice close, so external review reads deliberate change rather than drift.
(b), the first bounded context, is now the next slice and is unblocked.

**All eight landed as `build-spec.md` v0.9.6** (2026-07-27), with ADR §3 gaining a
*milestone-1 second-slice block* and all five doc-set stamps bumped. Where each went:

| # | Edit | Section | Record |
|---|---|---|---|
| 1 | Outbox export is at-least-once **and per-`Key` FIFO** | core §3.2 | §11b |
| 2 | Bootstrap phase roster is a set of **constraints**, and the roster stays open | core §3.2 | §14d |
| 3 | `SourceID` **composes `VenueInstrument`** | core §3.5 | §9h |
| 4 | Hot-reload is per-field **default-DENY**, unrecognised tag an error | core §5.2 | §10b |
| 5 | Total stop budget **< watchdog host-silence threshold**, refused at startup | core §7.4 | §14e |
| 6 | §7.5's preconditions **latch or self-clear**; clearance → *unreported*, never *passed* | core §7.5 | §14b |
| 7 | The **priority ladder** — one mechanism the core stated three times | core §7.6 (+ §7.1, §6 pointers) | §13a |
| 8 | Metric **label cardinality is default-deny** | core §10 | §12c |

**The eight are all one kind of finding, and that is the observation worth keeping.** Not one is a
disagreement with the core. Every one is a **requirement the core relied on without stating** —
an order guarantee behind a count guarantee, a constraint behind a list, a bound behind two numbers
in different sections, a cardinality limit behind a lossy-lane guarantee about something else. The
first slice's ten were a mix of deviations and additions; the second slice's eight are uniformly
*gaps*, and they were found by having to write the code that would otherwise have relied on them
silently. **That is the argument for building the kernel before the contexts, stated as evidence
rather than as taste.**

### 15a. No `M1-#` tag family — considered and rejected

The core's Conventions paragraph enumerates tag families (`R1d-#`, `Cx8-#`, `FM-#`, …), and these
eight went in **untagged**, matching v0.9.5's ten. A `[→ M1-#]` family was considered, since a
greppable tag is exactly what makes deliberate change mechanically separable from drift.

**Rejected on the project's own standing lesson.** Introducing it now would cover **eight of the
eighteen** milestone-1 edits, and a reader who greps `M1-` would find eight and conclude those are
all of them. A partial mechanism that reads as complete is the failure mode this repo has already
paid for repeatedly. The two honest options were *no tags* or *retro-tag all eighteen*; the second is
a larger change than the release needed, so the first was taken and the reason recorded here. ADR §3
now carries both blocks, each row naming its section — traceability for all eighteen in one place.

### 15b. Four unmarked candidates deliberately NOT landed in v0.9.6 — **ALL FOUR LANDED as v0.9.7 (session 14, §16)**

> **RESOLVED.** The scope call below was put to the user at the start of session 14 and answered:
> **land them, as a doc-only v0.9.7, before the next slice starts.** All four are now in the core
> (§3.2 + D2, §7.5, §7.6, §10), all five doc-set stamps are at v0.9.7, ADR §3 carries a third
> milestone-1 block, and each of the four code sites now carries a `[SPEC EDIT LANDED — v0.9.7]`
> marker. **Do not re-apply them.** The reasoning below is kept because *why each was originally
> left unmarked* is the part worth not losing — see §16 for what changed the verdict.


The release was held to **exactly the eight markers**, because the marker count is what makes the
gate mechanical: `grep -rn "SPEC EDIT OWED" internal/` returned eight before and returns **zero**
after, and landing an unmarked ninth would break that correspondence permanently. Four normative
statements the kernel makes are **not** in the spec and carry no marker — each was judged, at the
time it was written, to be an *implementation of* an existing requirement rather than a deviation
from it. That judgement is worth re-testing, but it is the user's scope call, not this session's:

1. **`outbox`** — `Seq` is **named as the dedup key** (content-hash dedup forbidden, because two
   legitimately identical funding accruals hash the same), and `Payload` is the **committed bytes**,
   never a Go object re-marshalled at delivery time (which would let exported bytes differ from what
   the §7.5/Z4 hash chain covers, reading downstream as tampering). Arguably part of D2's
   "idempotent consumers" — but D2 never says idempotent *on what*.
2. **`observability`** — the recording seam is **integer-typed**: a `Sample` at `fixedpoint`'s
   canonical scale, tagged with its **unit** and, for money, its **asset**. Arguably an
   implementation of §3.6 rule 4 — but the unit and asset tags are new normative content.
3. **`ratelimit`** — the header correction **ratchets upward only** (a reading below the local count
   is what staleness looks like; one above is information available no other way), and `Penalize`
   treats a venue `RetryAfter` as a **floor, not a ceiling**. Arguably an implementation of
   "header-aware governors".
4. **`boot`** — reconcile-only is a **posture, not a stage**: four rungs (halted → reconcile-only →
   stop-opening → trading) whose zero value is the most restrictive, reduced over the gates that
   were **required** rather than the ones that were reported. Arguably already implicit in §7.5's own
   "blocks new openings" / "stays in reconcile-only" language.

### 15c. A process finding — the generated Georgian tutorial was a version behind, and its own check knew

`docs/tutorial.ka.md` was stamped **v0.9.4** while `docs/tutorial.md` was **v0.9.5**: the v0.9.5
release bumped the English status line and never re-ran the i18n pipeline. The drift was **entirely**
in the frozen provenance line, and — the part that matters — **the invariant checker had already
caught it**: `node i18n/tools/assemble.mjs` refuses outright with `header: numbers differs / missing:
0.9.5 0.9.5 11 12 26 3.2 3.5 3.6`, and `bun test` in `docs/` reported **5 failing**. The mechanism
worked perfectly. Nobody ran it.

This is the fixture lesson wearing different clothes: a check that is never executed is
indistinguishable from a check that passes. **Fixed here** (header fragment re-synced to the v0.9.6
English line verbatim, re-assembled, `102 pass / 0 fail`), and the release checklist below now names
it, because "the doc set is stamped" and "the generated doc set is stamped" were two different
claims and only one of them was being checked.

## 16. Session 14 — §15b's four landed as v0.9.7, and what reversed the verdict

The user was asked the two open scope calls at the start of the session and answered both:
**land §15b's four first**, then **build `marketdata`** as the first bounded context. This section
records the first; the context is §17.

### 16a. The test that reversed the judgement

Each of the four was judged, when written, to be *an implementation of* an existing requirement
rather than a deviation from it — which is why none carried a marker. Re-testing that judgement
needed a sharper question than "is this in the spec?", because the honest answer for all four was
"sort of, by implication", which is what produced the original verdict. The question used instead:

> **Could a reimplementation get this wrong in the *unsafe* direction while still satisfying the
> core as written?**

All four fail it, and the failures are not close:

| Statement | The satisfying-and-wrong reading | What it costs |
|---|---|---|
| outbox dedup key | D2 says *idempotent consumers* and never says idempotent **on what** — content-hash dedup is a natural reading | two identical funding accruals on successive intervals are distinct facts; the second is dropped, surfacing as a **missing funding receipt in the §10 tax records** that nothing reports |
| boot posture | §7.5 calls reconcile-only a **phase of boot**, so a stage enum satisfies it | every §7.4 escalation puts a *running* process into it; and the natural way to write the combining minimum seeds the fold at the maximum, making **"nothing was checked" and "everything passed" the same answer** |
| header ratchet | §7.6 says **header-aware** and stops — *adopt the venue's reading* is the obvious implementation | a reading below the local count is what **staleness looks like**; adopting it discards every request sent since, and the downside is a ban escalating toward three days against a throughput loss that lasts until the window rolls |
| integer sample seam | §3.6 rule 4 bans `float64` under `internal/`, so a `//nofloat:allow` at the metric port satisfies it | that escape then propagates to **every instrumented call site**, each written by somebody thinking about a dashboard rather than rounding — and §12's kill-gate verdict is read off those panels |

**The distinction from the previous eighteen is worth keeping and is recorded in ADR §3.** v0.9.5's
ten and v0.9.6's eight were found by *writing* code that would otherwise have relied on an unstated
requirement. These four were found by *re-reading* code and asking what backs a claim it already
makes. That is a weaker signal, which is exactly why they were originally left unmarked — and the
weaker signal still produced four real gaps.

**Built with no new venue facts.** §7.6's edit leans on the rate-limit header names and Binance's
429→418 ban escalating 2 min → 3 days; both are already verified in `venue-profiles.md` §2, so they
are **cited, not introduced**, and that doc stays alignment-only. This was checked rather than
assumed, on the standing rule that nothing enters the profiles unverified.

### 16b. A process finding — the marker gate is necessary and not sufficient

v0.9.6's release gate was `grep -rn "SPEC EDIT OWED" internal/` returning zero, and that gate was
sound *for what it measures*: it proves nothing **flagged** is outstanding. It cannot see an
unmarked normative claim, because an unmarked claim is exactly the thing it does not match — and
the four above sat in the kernel through the whole of v0.9.6 with the gate reading green.

**This is the vacuous-coverage lesson applied to the release process itself,** and it is the third
place it has now appeared: a fixture pinned to a value that collapses a distinction (seven times, in
the code), a check that is never executed (§15c, the Georgian tutorial), and now a check whose
predicate cannot express the failure it is trusted to exclude. Zero markers means *nobody flagged
anything*; it does not mean *code and spec agree*. **The standing addition is a slice-close reading
pass over what the slice's code asserts normatively** — the question in §16a is the one to ask —
kept separate from the marker grep rather than folded into it, because the two answer different
questions and collapsing them would recreate exactly the gap being closed.

**§15c's checklist item worked this time.** `cd docs && bun test` was run as part of the release
rather than after it, the `ka` header fragment was re-synced to the new English status line before
assembly, and the suite reports **102 pass / 0 fail**. That item is now load-bearing rather than
aspirational.

### 16c. What landed

`build-spec.md` **v0.9.7**, all five doc-set stamps bumped, ADR §3 gains a **third** milestone-1
block (bringing the tagless total to **twenty-two**, which the "not tagged" paragraph now states),
and four `[SPEC EDIT LANDED — v0.9.7]` markers in `outbox/outbox.go`, `boot/posture.go`,
`ratelimit/ratelimit.go` and `observability/sample.go`. **No formal-model change** — `docs/formal/`
is untouched and still at the Cx8 verdict. **`state-machines.md` deliberately does not change**
despite §7.5's posture edit being its near neighbour: the rungs are what a *process* refuses to do,
while that doc's machines are what an *incident* does — the same reasoning v0.9.6 used for §7.5's
latching rule. `make ci` green.

## 17. Session 15 — the two sleeping gates woke, and each found something

The slice is `marketdata`, the first bounded context. Session 14 landed its `domain/`. This
session landed the two analyzers that context makes buildable, then the `application/` layer
underneath them. Order was chosen, not incidental: see §17a.

### 17a. Why the gates went in BEFORE the application layer

§1 states the preference for the other three — a gate is built first "so the code they police is
never written unpoliced". Rules 1, 6 and 10 were deferred against that preference on a *different*
argument, not a weakening of it: a lint over an empty tree passes trivially, which is
indistinguishable from a broken lint.

**That deferral expires the moment its subject exists**, and `internal/marketdata/domain` was its
subject. Writing `application/` first would have made the repo's first hexagonal layering the one
piece of code the layering lint never saw during authoring. So both gates landed first, and
`application/` is the first code in this repo written with rule 1 live.

`singlewriter` stays deferred. It is the one gate whose subject is genuinely absent — `oms`'s write
repository — and its reasoning is unchanged.

### 17b. `hexboundary` (rule 1), and the fixture that found a defect the design did not

domain/ and application/ may not import infrastructure/ or engine/. Scoped to **this module's own
`internal/` tree** rather than by a bare path segment, which is where it differs from `notimenow`
and the difference is not stylistic: `notimenow` only ever classifies the package under analysis,
ours by construction, while this classifies **both ends of an import edge** and the far end can be
anything on the network. A dependency published at `example.com/thirdparty/engine` is not §3.5's
parity seam, and reporting it would be the gate inventing a violation. testdata carries that import.

**The defect the fixtures found.** Go names an external test package by appending `_test` to the
last segment of the directory's import path. For `internal/marketdata/domain` that yields
`.../domain_test` — no `domain` segment, unpoliced — but for the nested `.../domain/sub` it yields
`.../domain/sub_test`, which keeps the segment and IS policed. Same language construct, opposite
treatment, decided by how deep the package happens to sit. A domain package would acquire or lose
lint coverage by being moved into a subdirectory.

Normalised away, and **both are policed** — the default-deny direction, on the standing rule. The
cost of policing is that a genuine wiring test moves, and it has two unpoliced homes (the adapter's
own package under `infrastructure/`, or the context root). The cost of not policing is the domain's
whole test surface naming concrete adapters, one file at a time, with nothing failing.

**Rejected, and recorded so it is not silently reopened:** widening rule 1 to deny *all* third-party
imports in `domain/`. It is the purest statement of the hexagonal rule and this gate does not
implement it. Rule 1 names two path-shaped targets, so widening is a spec change rather than an
implementation; and the engine module has zero production dependencies by recorded decision (§12a),
so it would be a lint over an empty tree — the very thing this gate's own deferral was about.
Revisit when the first production dependency lands, where the escape-hatch question (test-only?
generated?) can be answered against a real case.

12 claims, 12 caught, one after a `NOBUILD!!` re-aim. Verified over the real tree with a temporary
violation, because `internal/` today has no `engine/` or `infrastructure/` at all — a clean run
there proves nothing on its own.

### 17c. `exhaustiveswitch` (rule 10) — the evaluation the rule asked for, run

Rule 10 says to evaluate `golangci-lint`'s `exhaustive` first and build custom only if it does not
suffice. It was built against this module's `x/tools` and run over `./internal/...`:

| Configuration | Reports |
|---|---|
| defaults | 18 |
| — of which `reflect.Kind` | 3 (stdlib; never rule 10's subject) |
| — of which test-declared enums | 3 (partial by design) |
| — genuinely rule 10's | **12** |
| `-package-scope-only` | 18 — **no effect**; it means "members declared at package scope", not "enums from this package" |
| `-default-signifies-exhaustive` | 15 — inverts rule 10's stated purpose |

**Verdict: it finds the right things and cannot express rule 10's SCOPE.** "A rule-8 enum-like"
means an enum this module declares. Its only scoping knob is `-ignore-enum-types`, a regex exclusion
list that grows every time somebody switches on a new stdlib enum — the precise mechanism §3.6
rejects **by name** for `forbidigo`. Asking "was this enum declared in this module?" is structural.
Put to the user with the numbers; custom analyzer confirmed.

**The measurement worth more than the verdict:** only **3 of 18** sites use a `default:` clause at
all. This codebase's idiom is a switch with no default followed by a trailing return — a fallthrough
in different clothes — so rule 10 bites harder here than its own wording ("instead of falling
through to `default`") anticipates.

**Scope is asymmetric, and the asymmetry is the decision.** An enum *declared* in a `_test.go` file
is out: `clock`'s `defect` double would need ~40 no-op case labels across five methods, and a gate
that produces that is a gate somebody turns off. A switch *in* a `_test.go` file over a
**production** enum is in: `fixedpoint`'s `math/big` oracle dispatches on `RoundingMode`, and a mode
added without an oracle branch is a mode whose independent implementation silently stops existing
while the cross-check passes. That made it **13 sites, not 12**.

**No escape annotation, unlike `nofloat` and `notimenow`.** Their escape exists for a case with no
in-language alternative — a live-only socket deadline has nowhere else to go. A subset switch has an
alternative strictly better than an annotation: name the remaining members in one grouped case,
which the compiler checks and which documents the decision at the decision site. The asymmetry that
made `notimenow`'s escape cheap to add EARLY makes this one cheap to add LATE — an escape is purely
additive — whereas shipping one now would have turned 13 decisions into 13 annotations. **Put to the
user; "refactor, explicit zero case" chosen over "annotate".**

All 13 refactored, behaviour unchanged (every suite green under `-race`). Two sites came out better
than unchanged: `events.StreamKind.Lane` had ONE branch for "classified as control" and
"unclassified, therefore delivered intact" while its doc comment argued they were different facts,
and `ratelimit.Decision.detail` would have rendered "refused the request (admitted)".

15 claims caught, 1 documented equivalent (Go's grammar admits only case clauses in a switch body),
2 positive controls. One unmeasurable branch was **dropped rather than shipped**: no fixture can
produce a constant of unknown kind, since `analysistest` will not load a package that fails to
typecheck, and an untestable branch in a build-failing gate is indistinguishable from a broken one.

### 17d. Three harness/coverage findings, none of them in this slice's code

1. **`mutate.sh` could not reach the analyzers at all.** `go test ./tools/lint/...` from the root
   fails with *"main module does not contain package"* — non-zero, which the harness reads as a
   kill. **Every analyzer bite-check run through it would have been a phantom.** `nofloat` and
   `notimenow` predate the harness (session 6) and so were never exposed to it. Fixed with
   `MUTATE_MODULE`, and the fix is *measured* rather than assumed — a positive control confirms the
   harness can still report `SURVIVED` through the new `-C` path.

2. **A sixth verdict: `PANIC!!`.** A mutation dropping a nil guard made the analyzer segfault. A
   panic prints no `--- FAIL` line, so the harness reported `NOBUILD!!` — *"the tests never ran"* —
   about a mutant that compiled cleanly and crashed. Two diagnoses under one verdict, the same
   defect `NOBUILD` and `TIMEOUT` were each added to fix. It is a kill, reported separately because
   what it proves is weaker: the suite observed a **crash**, not a wrong answer, so the claim aimed
   at may still be unchecked.

3. **`within()`'s equality arm was untested in TWO gates.** Bite-checking `hexboundary` showed
   `pkgPath == dir` could be deleted with the suite green — every fixture imports a SUBpackage — and
   `kernelpurity`, which the helper was copied from, had the identical gap. This is the standing
   "apply the fix to its mirror" rule arriving from the direction of a *copied helper* rather than a
   symmetric API. Both now carry a fixture at the directory itself.

### 17e. The release-checklist grep had gone blind

The slice-close gate is `grep -rn "SPEC EDIT OWED" internal/`, scoped to `internal/` because the doc
set is full of prose about past markers. Correct while all code lived under `internal/`. It no
longer does — **the analyzers make normative claims about §3.6**, and the grep returned **0 while
two markers existed** in `tools/lint/`.

Third instance of this shape, after §16b: a gate reporting clean because it cannot see its subject.
The checklist grep is now `internal/ tools/` and `CLAUDE.md` carries both limits — it cannot see an
*unmarked* claim (hence the reading pass beside it, not merged into it), and it cannot see a marker
in a directory nobody added to the pattern.

**Two `[SPEC EDIT OWED]` markers are open**, both about §3.6's own status text:
- `hexboundary.go` — the "Gate status" paragraph names rule 1 as deferred and lists three enforced.
- `exhaustiveswitch.go` — rule 10 carries "(planned, not yet enforced)" and the "evaluate
  `exhaustive` first" instruction, both now discharged. It also records that **rule 10's deferral
  named the wrong trigger**: it waited on a *state machine*, while what actually made it buildable
  was the kernel's eleven enum-likes, which had existed for several slices.

### 17f. `marketdata/application` — and the spin it found in committed domain code

The funding use case: accept, decide whether it advances the source, stamp freshness, publish, arm
the silence timer. No goroutine and no ticker (`clock`'s argument, §3.5). **Not mutex-guarded,
deliberately** — §3.4 makes the reducer single-writer, and a mutex would make concurrent use
*compile* while leaving the published order nondeterministic under replay. Not locking is the honest
encoding; the race detector says so if the dispatcher gets it wrong.

**Two freshness verdicts, not one.** §3.4 says "per-venue max-age budget", singular, but a feed dies
two ways and the second is invisible to the check written first. Both are carried on the snapshot;
`Least()` is the decision and the pair is the diagnosis. *(This is the `Age`/`DataAge` question
session 14 flagged and declined to answer until the application layer existed. Answered: both, and
the spec question stands — see RESUME HERE.)*

**THE FINDING. `domain.Budget.NextTransition` returned the boundary instant, and `Budget.At` is
inclusive-fresh by deliberate design.** So the timer fired at an instant where the verdict had NOT
changed, and `arm()` re-armed for that same instant. Not a wasted wakeup — a **spin**: firing
forever while the observation stays marked fresh, with the ladder frozen short of the rung that
triggers §5.2's `mark_blind_action`. A dead feed, a busy process, and every rung of the escalation
§3.4 built the timer for unreachable.

**The domain suite had a test for exactly this.** `TestNextTransitionAgreesWithAssess`, written to
catch a shared off-by-one between the classifier and the timer arithmetic, checks the verdict one
tick BEFORE the named instant and one tick AFTER — **and never AT it**, which is the only instant
that matters because it is when the timer fires. Both neighbours pass under the defect. The bracket
missed by exactly one point. Two table cases named the spin and asserted it as correct ("still the
stale boundary" — which is *now*).

Fixed in the domain, where the arithmetic belongs, with a named `tick` constant so a later reader
does not tune it as a "margin" and reintroduce the spin. **Invisible from inside that package**:
`At` and `NextTransition` are each self-consistent and only their *composition* is wrong, which is
why it took a caller to surface it. This is the strongest argument yet for the layering itself —
the second implementation that disagrees was not an oracle, it was the layer above.

**Ordering is two rules because only one is always available.** `VenueSeq` where both readings carry
one (§3.5 makes it authoritative within a `SourceID`; a jump is §3.4's gap); arrival order
otherwise, since a REST-polled funding read carries none. **`SourceTime` is deliberately NOT the
fallback** — §3.5 rejects it as a reorder key, and a venue restamping an unchanged rate would make a
duplicate look like an advance. `SeqAvailable` rides on the snapshot so `GapDetected == false` on an
unsequenced source cannot read as an assurance.

18 mutations, 15 caught, 3 positive controls. Four diagnosed survivors were real gaps, **three of
them one root**: every route to `arm()`'s terminal branch went through `OnSilence`, where the timer
has already fired — so cancelling is a no-op and the delete was already done. Both cleanups looked
covered while the fixture never put the code in a state where they do anything. The path that
reaches it is an observation arriving *already blind*. The fourth: nothing tested that `Ingest`
refuses an invalid observation, because every other fixture is a valid one.

## §18 — session 16: `marketdata/infrastructure` + `module.go`, and the reading pass that found a real violation

Commit `c695cd2`. The slice's **code** is complete: `domain/` + `application/` + `infrastructure/{driven,driving}` + `module.go`. `make ci` green, 18 mutations / 18 caught, `hexboundary` shown to bite. **The slice is NOT closed** — see §18f for exactly what is owed.

### §18a — `driven/`: two adapters, and what each one's shape argues

**`ConfigBudgets` over `platform/config`.** `FreshnessConfig{Funding map[ids.Venue]BudgetSpec}`, tagged `config:"hot"`, with `BudgetSpec.Validate` delegating to `domain.Budget.Validate` rather than restating "MaxAge strictly below Blind".

Four decisions, each with its rejected alternative:

- **The section is scoped to `funding`, not named `freshness`.** §3.4 is explicit that freshness is per-data-class. A single map would invite one number for both, and the number that is right for funding is catastrophic for a book. *Rejected:* one `freshness` map with a per-class sub-key — same hazard, one level deeper.
- **The map is `config:"hot"`, and that tag is load-bearing rather than decorative.** `platform/config` treats a **key-set change as structural**, refused outright unless tagged. Without the tag, onboarding a venue or widening a budget during a degrading feed needs a process restart and *nothing in the code says so*. The test that proves the tag is present is `TestConfigBudgetsSeesAVenueAddedByReload`.
- **Validation sorts the map keys.** Go randomises map iteration, so a validator reporting "the first bad entry" names a different venue on different runs — two boot logs from one bad file that disagree about what is wrong with it. *Rejected:* reporting all of them (a config refusal an operator reads is better as one name than as a list they must diff).
- **An empty section is `ErrNoVenues`, refused at load.** *Rejected:* accepting it as deny-everything — safe in effect, but it presents as a total market-data outage and sends an operator to the exchange status page, which is `application.ErrNoBudget`'s own recorded argument one layer in.

**`LanePublisher` over the §3.4 market-data lane.** Two decisions worth carrying:

- **The probe.** `Publish` validates a **copy** of the envelope with both sequence fields forced to 1, then discards the copy. This reuses `events.Envelope.Validate` *in full* — so a required field added to `Envelope` next year is caught here — without stamping a replay key this layer has no authority to mint. *Rejected:* restating the field list (goes stale silently); *rejected:* calling `Validate` and tolerating the two sequence sentinels via `errors.Is` (`Validate` returns on first failure, so an envelope missing both a sequence *and* `RecvTime` would be let through — order-dependent and wrong).
- **Two checks that look redundant and are not.** `SourceTime` is **optional** on an envelope (not every venue message carries one) and **required** by `domain.Watermark` (an observation with no venue stamp cannot answer `DataAge`, the venue-side-stall detector). That single asymmetry is what makes both load-bearing. The tests assert **sentinels**, not `err != nil`, because otherwise deleting either check looks identical — and the discriminating case is a missing `RecvTime`: the probe gives `events.ErrRecvTimeUnset`, the observation check gives `domain.ErrWatermarkUnset`.

### §18b — `driving/`: the envelope is authoritative for provenance

`FundingDispatch.Deliver(events.Envelope)` + `Timer(clock.TimerID)`, and `FundingUpdate` — the normalized funding payload, **carrying no provenance and no instrument**.

- **Provenance from the envelope, venue reading from the payload, neither stated twice.** §4's `StreamFunding` comment lists "source, venueSeq" as payload contents, which are *also* `Envelope` fields §3.5 defines for every stream. Read literally it asks for both copies with nothing saying which wins. Removing the redundancy leaves `Deliver` with no reconciliation to get wrong — the inverse of `ids.VenueInstrument`'s "a redundant encoding is safe only while something rejects the disagreement".
- **The instrument comes from `Source`, which is the mailbox key.** A payload naming a different instrument is a state no feed can produce and every adapter would otherwise have to check.
- **The stream guard checks `StreamFunding`, not the lane.** The lane check would pass: mark and book ride the same market-data lane. The fixture proving the difference uses `StreamMark` — a control-lane fixture alone would make "is it funding" and "is it on my lane" the same predicate.
- **`Deliver` validates the envelope in full, sequence numbers included.** What reaches a context handler is post-dispatch. An observation accepted with no `IngressSeq` is not merely missing from a replay — it becomes the held reading the *next* observation's supersede check compares against, so an unreplayable envelope silently changes what a replay accepts.
- **No `Handles(TimerID) bool`.** *Rejected deliberately:* the membership test already exists once, inside `FundingIngest`'s armed map, and this package cannot see it. A second test written from the ID prefix would copy a naming rule that lives in `application/`, and the two would agree until somebody changed one. A foreign ID returns `application.ErrUnknownTimer`, which a dispatcher can branch on; what a dispatcher should *do* with it is a policy for a caller that does not exist yet.
- **`FundingUpdate` is declared in `driving/` because `engine/venue` does not exist.** It *is* the contract until then. When `engine/venue` lands, the translation is one function in this package and `domain/`/`application/` are untouched — which is the property §3.6 rule 1 exists to protect.

### §18c — `module.go`: the composition seam adds no refusals of its own

`Deps{Clock, Config, Lane}` → `Module{Funding}`. It contributes **order, not validation**: every dependency already has exactly one constructor that owns its refusal, and a nil check here would be the copy that drifts. The consequence a root can rely on: an error from `New` is one of the constructors' sentinels, so branching on `driven.ErrStoreNil` or `application.ErrClockNil` works directly.

*Rejected:* wiring in `app/` instead. What varies between §11's binaries is what the store and lane **are**; which adapter satisfies which port does **not** vary, and spreading it across four roots makes it four copies of one fact — while the backtest's parity claim rests on the wiring being the same wiring.

`Module` has **one field**, so the deferred book/mark paths are visible in the type rather than as a nil to check.

### §18d — the `hexboundary` positive control, and why the obvious probe fails

A clean gate run proves nothing. Shown to bite: **exit 3**, with the named diagnostic, then clean again at exit 0.

**The obvious probe does not work, and that is worth knowing.** Importing `infrastructure/driven` from `application/` is an **import cycle** — the type-checker refuses first and the analyzer never runs. Every infrastructure package here imports `application` or `domain`, so within one context Go's own no-cycles rule already covers most of rule 1. The probe that works is a package under `infrastructure/` importing **only `domain`**, imported from `application/`: no cycle, and the gate objects. **Where rule 1 earns its place is the edge Go does not catch** — `engine/`, and another context's infrastructure.

### §18e — mutations: 18 run, 18 caught

Re-aimed **two `NOBUILD!!`** (both mine: substituting an undefined helper, and orphaning a variable by returning a constant instead of using it) and **one `NO-OP!!`** (line numbers taken from a `grep -n` on a *pre-gofmt* file). All three are the harness working as designed.

The mutations that mattered: the stream guard weakened to a lane check (caught, 3 tests); priority derived from the freshness verdict (caught); the adapter minting its own sequence numbers (caught); each of the publisher's two checks disabled independently (caught, and by *different* tests, which is what proves neither is vacuous); sorted-key validation replaced by map order (caught) and by reverse order (caught — two separate claims, stability and which-one-is-named).

### §18f — THE READING PASS, and the one finding that is a real violation

Three claims were marked `[SPEC EDIT OWED]` in code. **The reading pass then found the first one is not an ambiguity at all — the code violates a rule the spec already states.**

**§3.5, line 242, states it outright:** *"the **dispatched** envelope — a reconstructed snapshot for book streams — carries the `AcceptOrdinal` of the delta that triggered it (causal traceability)"*. The parenthetical is an example; the rule is general. **`LanePublisher` publishes with `AcceptOrdinal == 0`, so a published `FundingSnapshot` cannot be traced back to the raw input that produced it.** The port signature is what makes it unimplementable: `FundingPublisher.Publish(FundingSnapshot)` carries no envelope context, and `ports.go`'s argument for that is about **`IngressSeq`** — the replay key, which this layer must not *invent* — while `AcceptOrdinal` is **inherited**, not invented, and `driving/` has it in hand.

**The fix, designed but NOT YET APPLIED** (the edit was reverted mid-flight; the tree is at `c695cd2`):

- `application.AcceptOrdinal uint64` — a **named type**, because `VenueSeq` and `AcceptOrdinal` are both `uint64`, mean entirely different things, and a call site crossing them would otherwise compile. Declared in `application/` rather than imported from `platform/events`, per that layer's rule about not taking the transport's types; `driving/` converts.
- `FundingSnapshot.Trigger AcceptOrdinal`, with `Ingest(obs, trigger)` **refusing a zero** — which is what makes zero mean something rather than mean "somebody forgot".
- **Zero = timer-triggered.** §3.5 states the inheritance rule for envelopes derived from raw inputs and has **no case for one derived from the ABSENCE of an input**, which is exactly what §3.4's silence timer produces — the one envelope on the lane whose entire news is that nothing arrived. *Rejected, and it is what an implementation with nothing to put there would reach for:* reusing the last input's ordinal. That makes the capture log say one raw input produced two dispatched envelopes, so a replay reconciling dispatched envelopes against raw inputs reports a phantom duplicate and finds nothing wrong with the feed.
- `LanePublisher` sets `env.AcceptOrdinal = snap.Trigger`; the probe still forces both fields, since a timer-triggered snapshot legitimately has zero.

**So the §3.5 edit owed is a SCOPING one, not a new rule:** the inheritance requirement is scoped to input-triggered envelopes, timer-triggered ones carry no ordinal, and `IngressSeq` is never stamped by a producer — an envelope resting on a §3.4 mailbox is **pre-dispatch**, so `Envelope.Validate` cannot be a producer-side check.

The other two markers stand as written:

- **Priority may not depend on content** (`driven/publisher.go`). §3.5 gives `Prio` a direction and never says a source's priority must be constant. Escalating a `FreshnessBlind` snapshot — tempting, since it fires §5.2's `mark_blind_action` — reorders within a `SourceID` where §3.5 makes `VenueSeq` authoritative, so **a consumer sees a degrading feed recover**.
- **The envelope is authoritative for provenance** (`driving/funding.go`), against §4's literal `StreamFunding` field list.

### §18g — the two spec questions, ANSWERED BY THE USER (session 16)

Both were put to the user and decided. **Do not reopen; do not re-ask.**

1. **`Age` vs `DataAge` → one budget, two measurements.** §3.4's per-venue budget is per **data class**, and it applies to *both* the transport age (`Clock.Now`, from `RecvTime`) and the venue-side data age (`Clock.ServerNow(venue)`, from `SourceTime`), reported separately and reduced by minimum for a trade decision. *Rejected:* a second independent budget — it buys a looser tolerance for a venue whose own clock is known to lag, at the cost of a config field per venue and a second unreachable-rung validation; the deciding argument is that §3.4's stated reason for per-class budgets is the data's tolerance for staleness, which does not change with which clock measured it.
2. **§3.2 vs §3.3 on `MarketView` → §3.3 wins; fix §3.2.** `marketdata` publishes normalized, freshness-stamped observations; **`engine/state` translates into the strategy-owned `MarketView`**. *Rejected:* §3.2 winning — it would make `marketdata` know cross-leg skew and the strategy's view shape, and reduce `engine/state` to a cache; it also needs the book path to exist first. The deciding argument is §3.6 rule 1: a context publishing a strategy-owned type couples the two across the boundary the read ports exist to keep.

### §18h — WHAT IS OWED (the slice close, user-approved as "run the full close")

The user chose **run the full slice close**. None of it is done. In order:

1. **Apply the §18f code fix** (`AcceptOrdinal`/`Trigger`). This touches `application/ports.go`, `application/funding.go` (the `Ingest` signature, `assess`, `OnSilence`), `application/funding_test.go` (every `Ingest` call site), `driving/funding.go`, `driven/publisher.go` and the three test files. **Do this before the spec edit** — the spec should describe what the code does.
2. **Land five `[SPEC EDIT OWED]` markers** as `build-spec.md` **v0.9.8**: the three from §18f (§3.5 ×2, §4 ×1) plus the two pre-existing §3.6 status-text ones (`tools/lint/hexboundary.go` — rule 1 no longer deferred; `tools/lint/exhaustiveswitch.go` — rule 10 no longer planned, and the "evaluate `exhaustive` first" instruction discharged).
3. **Land the two answered questions** from §18g: §3.4's freshness sentence (one budget, two measurements) and §3.2's `marketdata` row (no `MarketView`).
4. Bump **all five** doc-set stamps to v0.9.8 · ADR §3 **milestone-1 third-slice block** · flip code markers to `[SPEC EDIT LANDED — v0.9.8]` · re-sync `i18n/ka/tutorial/header.md` and run `node i18n/tools/assemble.mjs` · **`cd docs && bun test`** · `make ci`.

**Exact edit sites, already located, so they need not be re-found:** `docs/build-spec.md` line **131** (§3.2 marketdata row), line **199** (§3.4 freshness sentence), lines **224–225** + **242** (§3.5 `Envelope` field comments and the causal-traceability sentence), line **271** (§4 `StreamFunding` comment), line **~257** (§3.6 rule 10 text) and the **"Gate status"** paragraph below it.

**Worth asking in the reading pass, still unanswered:** rule 10's deferral named the wrong trigger — it waited on a *state machine*, while the kernel's eleven enum-likes are what actually made it buildable, and they had existed for several slices. `singlewriter` may be mis-triggered the same way.

## §19 — session 17: the slice CLOSED — the §18f fix applied, and `build-spec.md` v0.9.8

Commits `c8db027` (code) then `9136f90` (docs). **§18h's four steps are done in its stated order, and the order was the point: the code fix landed first so the spec describes what the code does rather than what it was about to do.**

### §19a — the fix, as built, and the two places §18f's design needed a decision it had not made

The design in §18f survived contact. `application.AcceptOrdinal` is a named type; `FundingSnapshot.Trigger` carries it; `Ingest(obs, trigger)` refuses zero with a new `ErrNoTrigger`; `OnSilence` passes zero; `driving/` converts from the envelope; `driven/` stamps it back. Two things §18f left open were decided while building:

- **Where the zero check goes in `Ingest`'s step order.** It goes **first**, ahead of `obs.Validate()`, and the argument is about who owns each failure. A driving adapter reaches `Ingest` only after `events.Envelope.Validate` has already refused an unstamped ordinal, so a zero arriving here means a **caller that built the call by hand** and left the trace out. Reporting a malformed *reading* first would send the reader to the feed for a defect in the wiring. *Rejected:* after validation, on the existing comment's "validate what arrived" ordering — but the trigger is not part of what arrived, it is a fact about the delivery.
- **The trigger is a PARAMETER, not a field on `domain.FundingObservation`.** An `AcceptOrdinal` is a fact about the delivery: the same funding numbers re-delivered are a second raw input with a second ordinal, and an observation is a value two deliveries can legitimately share. Putting it on the observation would also put a transport concern inside `domain/`, which is what §3.6 rule 1 exists to prevent.

`LanePublisher`'s probe keeps forcing **both** sequence numbers. That looked like something to tighten — the envelope really does carry an `AcceptOrdinal` now, so probing with the real value reads naturally — and it is wrong: a silence-timer snapshot legitimately has zero, so the tightened probe would refuse **precisely the envelope §3.4's timer exists to emit**. The one failure that must never be silent would be the one this adapter drops. It has a test of its own (`TestPublishAcceptsATimerTriggeredSnapshot`) rather than a comment, because a comment does not fail.

### §19b — the fixture-collapse checklist, applied to a value that would have got 1 by default

Run **before** the mutations, per the standing rule, and it changed three fixtures:

- **`TestPublishStampsNeitherSequenceNumber` asserted the OLD behaviour and passed.** It passed only because `snapshot()` left `Trigger` unset, so the fixture did not contain the thing under test — the tell, exactly as the checklist states it. It is now `TestPublishInheritsTheOrdinalAndStampsNoReplayKey`, and it is **one test rather than two** because a suite asserting the two sequence numbers separately cannot show they are treated *differently*, which is the whole claim.
- **Every trigger value is chosen against the sequence it sits beside.** `driven`'s is 9001 against a `VenueSeq` of 4242; `application`'s is 4242 against a seq of 3; `driving`'s three envelope numbers were already pairwise distinct. Both are `uint64` and both ride the same envelope, so a fixture reusing one value would make "inherited the trigger" and "stamped from the watermark's sequence" the same assertion — and the second is a real implementation somebody would write, because the watermark is the field already in hand.
- **The shared `anyOrdinal` is 77, not 1.** 1 would let a hardcoded `Trigger: 1` pass everywhere, and 77 collides with no `VenueSeq` in the file (0, 1, 2, 4, 5, 6, 8, 9999).

### §19c — mutations: 7 run, 7 caught, and one of them is only reachable end to end

`if trigger == 0` → `if false` (caught); `Trigger: trigger` → `AcceptOrdinal(w.VenueSeq)` (caught, 2 tests); `OnSilence`'s literal zero → the held observation's ordinal, **the rejected alternative itself** (caught); the publisher dropping the inherited ordinal, i.e. the original violation restored (caught); the probe forcing only `IngressSeq` (caught); `driving` passing `IngressSeq` as the trigger (caught).

The seventh is the one worth keeping: **the publisher's drop, run again against `./internal/marketdata`.** Each layer's own suite passes with the chain broken in the middle — `driving` proves the ordinal enters, `driven` proves a snapshot's trigger is stamped, and neither proves the value that enters is the value that leaves. `TestModuleCarriesAnEnvelopeThroughToTheLane` is the only test that can make that claim, and it asserts against the **inbound envelope's** field rather than a literal.

### §19d — what landed in the spec, and the one edit §18h did not have on its list

The eight ADR rows are the record. One correction to §18h's accounting: it counted "the three from §18f (§3.5 ×2, §4 ×1)", but the §3.5 marker was really **two rules wearing one marker** — *when* an envelope must carry each sequence number, and *what a timer-triggered envelope carries*. They landed as two, and `publisher.go` now carries two markers where it carried one. **A marker is not a unit of spec change**, and counting them as if it were is how a release gate keyed on marker count under-delivers.

Two edits went in that no marker demanded, both from §18g's answered questions: §3.4's one-budget-two-measurements sentence and §3.2's `marketdata` row. Neither was ever marked, because neither was a deviation the code took — they are a **silence** and a **contradiction** found by reading. That is the same lesson as the lead finding, from the other side.

### §19e — the finding this slice contributes to method

**The `[SPEC EDIT OWED]` grep cannot find a rule the code silently broke.** It is a gate over claims the code *marked*; a violation is by definition unmarked, and this one came with a doc comment confidently arguing the rule did not apply. It was found by the reading pass, which is why that pass sits **beside** the grep and not inside it — a limit already written down in `CLAUDE.md`, now with a case behind it.

The mechanism of the mistake generalizes past this file: **a true rule stated without its scope becomes a false rule.** "This layer must not invent a sequence" is true of `IngressSeq` and false of `AcceptOrdinal`, and one sentence covering both cost the trace. Worth looking for wherever two fields of one type sit behind one prohibition.

## §20 — session 18: `engine/venue`, the fourth slice — the placeholder contract is retired

**The user chose `engine/venue` from the three candidates HANDOFF listed, on the decay argument: `driving.FundingUpdate` was explicitly the contract "until `engine/venue` lands", and exactly ONE consumer held that placeholder. The book path would have added a second and `oms` a third and fourth.** The slice is code-complete and **NOT closed** — three `[SPEC EDIT OWED]` markers are open (§20e).

### §20a — what was built, and the three things deliberately NOT built

`internal/engine/venue/` — flat, no DDD layering, which is §3.2's rule for an engine subsystem rather than an omission. There is no domain here because there is no domain *language* here: a funding rate means one thing to `marketdata` and another to `strategy`, and this package owns neither meaning, only the shape both agree the venue's reading arrives in.

| File | Holds |
|---|---|
| `doc.go` | The charter, what is absent and why, and marker #1 |
| `funding.go` | `Funding` (the normalized DTO) + `Validate` + `FundingIntervalFromHours`, and marker #3 |
| `source.go` | `FundingSource` — the funding mailbox key, written down once |
| `errors.go` | Four sentinels, and a note recording one that is deliberately absent |

**Three absences, each with a reason that is the same reason:**

1. **No raw Binance/Bybit clients**, though §4 assigns them here. Nothing can consume one — §3.5 says the dispatcher is "not yet built", there is no composition root under `app/`, and a WS client with no dispatcher to feed would have to invent its own arbitration, which is the second-authority defect §3.5 spends a paragraph forbidding.
2. **None of §4's nine capability interfaces is declared.** Eight are consumed by contexts that do not exist (`oms`, `treasury`, `risk`). Declaring them is §3.6's own argument for deferring `singlewriter` — *a gate over an empty tree passes trivially, and an interface with no implementer and no caller is a declaration nothing can disagree with.* §4's own rule also cuts against it: contexts declare the segregated capability they need in their **own** application layer and this package satisfies it, which is what `marketdata` already does, so an interface here would be a third statement of a contract that already has two ends.
3. **No `DefaultFundingInterval = 8 * time.Hour` constant**, despite Profiles §1 recording Binance's "absent ⇒ 8h default" as a verified fact. Rejected on this repo's standing rule about default direction — see §20b.

### §20b — the decision the slice turns on: there is no 8-hour fallback

Profiles §1: Binance's funding stream carries **no interval field**; it comes from a REST overlay (`GET /fapi/v1/fundingInfo`), and the rule is "absent ⇒ 8h default". The obvious implementation is a fallback on the zero value — `if hours == 0 { return 8 * time.Hour }`.

**Rejected.** A zero reaches that conversion from three completely different situations, and a zero-value fallback answers all three identically:

| Zero because | 8h is |
|---|---|
| Binance's overlay was fetched and genuinely reported nothing | **correct** |
| the overlay was never fetched, or a decode dropped the field | a wiring failure wearing a correct BTC reading's clothes |
| Bybit sent a ticker **delta** whose unchanged field is simply absent (Profiles §1: *"absent field = unchanged, never null-coerce"*) | §2.4's **C8 defect** — the venue that flips an alt 8h↔1h with no notice, and the flip gets absorbed |

So zero is **refused**, and the Binance adapter (when it lands) applies its default from a *positive* fact — an overlay that was fetched and came back empty — which is a state the caller has to be holding in order to express. That is "when a default has two directions, pick the one that is right when somebody forgets it", applied to a case where forgetting produces a *plausible* number rather than a crash.

**`MaxFundingIntervalHours = 24` is a unit check, not venue policy.** Every perp either venue runs settles at 1/2/4/8h, so 24 admits every real value; what it rejects is an argument that is not in hours — 480 (minutes), 28800 (seconds), a truncated `time.Duration`. Each would otherwise be silently wrong by orders of magnitude, and §2.4's settlement count is a division by it, so the error surfaces as a plausible number of settlements rather than as a failure.

### §20c — the perp-only rule went into `platform/events`, not here, and that was a close call

§4 marks `StreamFunding` and `StreamMark` **perp-only**. §3.5 defines `SourceID` and gives it a `Validate` that never carries the constraint across — so `SourceID{spot BTCUSDT, StreamFunding}` validated cleanly, and so did an envelope built from it.

The rule was first written in `venue.FundingSource` with its own `ErrFundingOnSpot`. **Moved to `events.StreamKind.RequiresPerp` + `SourceID.Validate`**, on three grounds:

- **It is a rule about two fields of one struct**, and this is the type that holds both — the identical argument `ids.VenueInstrument` already makes for checking its product against its contract type.
- **Placement decides who is covered.** On `SourceID`, every producer gets it — including the ones not yet written — and `Envelope.Validate` enforces it process-wide. In `venue.FundingSource`, only callers who remember the helper get it.
- **The hole was real and covered by accident.** `marketdata/domain.FundingObservation` refuses a spot instrument, so the funding path was safe; every other consumer of the same source was not, and the coverage came from a *context's domain*, three layers downstream of the transport that had already accepted it.

*Rejected:* leaving it in `venue` (narrower coverage, and a second sentinel to keep in step); and putting a translated `venue.ErrFundingOnSpot` in front of the events one (a second name for one refusal — the value a caller branches on should name the rule's owner).

**Direction of the unrecognised-stream answer: `true`.** Unreachable through `Validate`, which refuses an undeclared `StreamKind` first, so it costs nothing and is chosen on principle. Constraining an unclassified stream to perps *denies* a source nobody has classified; the opposite default admits one. Loud-wrong beats silent-dead.

### §20d — two types with overlapping validation, and why that is structural rather than sloppy

`venue.Funding` and `marketdata/domain.FundingObservation` carry the same five fields and both validate them. **§3.6 rule 1 forbids a context's `domain/` from importing `engine/`, so the domain literally cannot be expressed in terms of the venue type.** That is the anti-corruption layer §4 asks for, and its cost is paid in one visible place — the field-by-field translation in `driving/` — rather than spread across a shared struct both layers would be free to add to.

Neither check is deletable. This one judges a **venue reading** and has producers the domain will never see (§8's capture lane records normalized DTOs; §3.5's replay `DataSource` produces them with no context in the process). The domain's judges an **observation** — this plus an instrument and a watermark — and is the stronger check wherever both apply.

**But the overlap made the venue check invisible, and that needed a test rather than a comment.** `domain`'s check is a strict superset over the shared five fields, so *no input exists* that the venue check refuses and the domain check accepts: delete `u.Validate()` from `Deliver` and every assertion about whether a malformed reading is refused still passes. What changes is only which boundary the error names — and the sentinels are distinct values in distinct packages, so `TestDeliverJudgesTheVenueReadingBeforeTranslatingIt` asserts `venue.ErrClampInverted` and explicitly asserts that `domain.ErrClampInverted` did **not** win. Its second subtest breaks a field the venue type does not carry (`SourceTime`), showing the outer check has not swallowed the boundary underneath it.

### §20e — three `[SPEC EDIT OWED]` markers, all of one kind

All three are **silences in the core, not disagreements with it** — the v0.9.6 shape rather than the v0.9.8 one.

1. **`engine/venue/doc.go` — §3.5 and §4 declare the market-data seam twice, with different shapes.** §3.5 gives `DataSource interface { Subscribe(VenueInstrument) <-chan Envelope }`; §4 gives `StreamBook`/`StreamFunding`/`StreamMark`. Same position in the pipeline — both are what the dispatcher reads and what §8 substitutes a replay adapter for — and a type satisfying one does not satisfy the other. §3.5's block *references* §4 everywhere else in it (`ExecutionVenue` is written as a composition of §4's four interfaces and says "see §4" inline); `DataSource` is the one line written as a definition, and its signature has not kept up. §4's shape wins, on three arguments recorded in the doc comment. **The edit is a resolution, not a new rule** — the same shape as v0.9.8's §3.2-vs-§3.3 `MarketView` call. It is deliberately *not* resolved by declaring the interface: that is the empty-tree case above.
2. **`platform/events/envelope.go` — §3.5's `SourceID` can name a source no venue can produce.** §4 states the constraint per stream; §3.5 never carries it across. The edit should state the full per-stream table, including that the three private streams are **not** product-constrained (§4.2 makes spot a full API family with its own user-data stream, so constraining them would silently unsubscribe half the trade).
3. **`engine/venue/funding.go` — §4 states a units discipline for money and none for time.** §4 requires numeric fields to cross as dimensional `fixedpoint`, "never `float64`" — money, and a good rule. Nothing covers the venue fields that are **durations**, and Bybit's `fundingIntervalHour` is load-bearing: a conforming reimplementation can multiply it by `time.Minute`, satisfy this spec completely, and hand §2.4 a 480-minute interval that enumerates a plausible settlement count while the kill-gate carry figure is wrong by a factor of sixty. **This is §10's own argument, landed in v0.9.7 for the observability seam** — where "a duration recorded in the wrong unit renders as a plausible dashboard number and fails nothing" — and the venue seam is where the duration is *created*, one layer earlier with the same consequence. The edit is the rule, not the constant.

### §20f — the fixture-collapse checklist, run BEFORE the mutations, found four

Per `CLAUDE.md`'s tenth entry. Each has a precedent in the ledger:

| Collapse | The fixture that hides it | Precedent |
|---|---|---|
| `c < 0` vs `c <= 0` on the clamps | every natural cap/floor pair is strictly ordered | `ratelimit`'s `freeAt` |
| `hours > Max` vs `>=` | testing only 480 | same |
| `IsPerp()` vs `== ProductLinearPerp` | **every perp fixture in this repo is linear** | `ids`' own redundancy rule |
| `FundingSource` returning the right stream | asserting only `err == nil` — `StreamMark` is *also* perp-only, so it validates identically | new |

Fixed before writing the assertions: a **Cap == Floor** case that must be *accepted* (a pinned rate is degenerate, not inverted); **exactly 24 accepted and 25 refused**; an **inverse perp** accepted for funding and mark, in both `events` and `venue`; and `FundingSource`'s returned `Stream` asserted directly. The valid fixture's cap and floor are also deliberately **asymmetric** (`0.0075` / `-0.0031`) — under the natural `±0.0075` pair, a comparison in the wrong direction and one against the wrong operand give the same verdict for every input.

Two non-checks are pinned by tests rather than left to comments: a **rate outside its own clamps is accepted** (the clamp bounds §2.4's *forecast*, not the wire reading — refusing it would drop the single most informative funding reading there is, and drop it as *malformed* rather than as *clamped*), and `FundingIntervalFromHours(0)` **does not return 8h**.

### §20g — mutations: 16 run, 14 caught, 2 SURVIVED — and both survivals are findings

The two survivals are wrong mutants, not test gaps, and diagnosing which was the work.

**SURVIVED 1 — `case StreamFunding, StreamMark:` → `case StreamMark:`.** `RequiresPerp` still answers `true` for funding, via the trailing `return true` that *is* the safe default. The claim is real and it is checked — by **`exhaustiveswitch`**, verified by hand rather than assumed: applying the mutation and running `make lint` gives exit 3 and *"switch over StreamKind does not name StreamFunding"*.

**The generalization, and it was tested rather than asserted.** A trailing `return X` makes the membership of the `X` group unobservable to tests: any member dropped from it falls through to the same answer. Probed against **committed code from an earlier slice** — `StreamKind.Lane`, whose default is `LaneControl`:

- dropping `StreamControl` from `case StreamExecutions, StreamAccount, StreamControl:` → **SURVIVED**, as predicted
- dropping `StreamFunding` from the market-data case (a member *outside* the default group) → **caught**

So the two gates are not redundant and neither substitutes for the other: the test suite covers the non-default groups, `exhaustiveswitch` covers the default one. Flipping the default to make it test-observable would trade a checkable claim for a dangerous default, which is the wrong trade.

**SURVIVED 2 — replacing `venue.FundingSource(...)` in `LanePublisher` with the literal it used to be, plus a nil error.** The spot instrument is still refused, because the probe two lines down calls `Envelope.Validate` → `SourceID.Validate` → §20c's rule. The rule is enforced by **construction**, not by remembering to call the constructor.

**That is the good outcome, and it falsified the comment I had written at that call site.** The comment claimed the constructor produced the outer, better-named diagnosis; in fact both paths produce the *identical* wrapped `ErrStreamProductMismatch`. **§19e's lesson arriving again inside one session** — a doc comment confidently arguing for a property the code does not have. Rewritten to say what the call actually buys, which is narrower and structural: one place in the codebase writes down which stream carries funding.

Everything else caught, including the two that matter most: `StreamFunding` → `StreamMark` in `FundingSource` (1 failing test — the assertion the collapse checklist added), and dropping `u.Validate()` from `Deliver` (2 failing tests — the diagnosis-ordering pair).

### §20h — WHAT IS OWED (the slice close)

1. **The reading pass is DONE and produced marker #3** — it is in the count above, not additional to it. The other two were taken while building.
2. **The spec edits**, as `build-spec.md` **v0.9.9**: §3.5 (the `DataSource` resolution + the `SourceID` product rule) and §4 (the time-units discipline). Then the release checklist: five doc stamps, ADR §3's fifth milestone-1 block, marker flip to `[SPEC EDIT LANDED — v0.9.9]`, Georgian header re-sync + `tutorial.ka.md` re-assemble, `cd docs && bun test`, `make ci`.
3. **Not owed and worth saying so:** no formal-model change. Nothing here touches the fence/takeover/plan composition; `docs/formal/` stays at the Cx8 verdict.

## §21 — session 19: the slice CLOSED — `build-spec.md` v0.9.9

§20h's list, done in its stated order. **This is milestone 1's first close with no code change in it**, and that is worth stating rather than passing over: §19's close *opened* with a code fix because its lead finding was a violation, and this one has nothing to fix because all three markers were silences. A doc-only close is what "the code already agreed with the spec, the spec had just never said it" looks like from the outside.

### §21a — two edits the release checklist did not have on its list, both found by doing it

The three ADR rows are the record of what was owed. Two more went in that no marker demanded:

- **Four stale `DataSource` references outside §3.5's declaration.** §3.2's backtest row, §3.5's own live-vs-backtest wiring sentence, §8's parity bullet and §11's repo tree all named the retired type. The marker was written against the **declaration**, because that is where the contradiction was visible; the rename's actual surface was **five sites**. Renaming the declaration and leaving the references is how a resolution becomes a *second* ambiguity — a reader greps `DataSource`, finds four hits and no definition, and cannot tell whether the type was renamed, deleted, or is simply missing. **The general form: a marker names where a defect was seen, never where its fix reaches, so the close has to re-derive the surface rather than trust the marker's location.**
- **The `C8` traceability row gained `core §4`.** The refuse-rather-than-default rule resolves part of a finding the ADR already tracks (Bybit dynamic settlement), and a resolution absent from the traceability table is invisible to exactly the review that table exists to serve.

### §21b — the evidence that settled the rename, and why it beat the argument that preceded it

`engine/venue/doc.go` argued §4's shape wins on three grounds — the mailbox key a merged channel forces the subscriber to re-derive, the per-stream product constraints `Subscribe(VenueInstrument)` cannot express, and §7.6's budget spent on a book feed a funding-only consumer never wanted. All three hold, and all three are in the spec now.

But the cheapest evidence turned up during the close and is worth more than the argument: **`venue-profiles.md` §3 was already writing `MarketDataSource.StreamMark`.** The rest of the doc set had adopted §4's name already; §3.5's `DataSource` was not a competing convention but a single line nobody had updated. An argument tells a reader which shape *should* win. A citation tells them which one the set was already using — and only the second is checkable by someone who disagrees with the argument. Look for it before writing the third paragraph of reasoning.

### §21c — a contradiction and a silence are not the same finding, and only one is findable alone

All three markers are silences in the core (the v0.9.6 shape). **Two of them are the sharper kind: the core does not omit the rule, it states it twice, incompatibly.** The market-data seam is declared in §3.5 *and* §4 with shapes no single type satisfies; the per-stream product constraint is stated in §4 and dropped where §3.5 defines the identity that carries both of its fields.

A gap is visible to a reader of one section. **A contradiction is visible only to a reader holding two sections at once** — and neither is visible to a grep. So §19e and this slice name the same limit of the slice-close gate from opposite sides: session 17 found a rule the *code* silently broke, session 18 found rules the *spec* silently disagreed with itself about. Both were found by the reading pass, which now has two independent cases behind it rather than one.

### §21d — what is owed: nothing

`grep -rn "SPEC EDIT OWED" internal/ tools/` returns **zero**, and the three markers read `[SPEC EDIT LANDED — v0.9.9]`. Doc set at v0.9.9 across all five stamps plus the Georgian header; `tutorial.ka.md` re-assembled from fragments (never hand-edited — a test asserts the file equals a fresh assemble); `docs` **102 pass**; `make ci` green. **No formal-model change** — nothing in this slice touches the fence/takeover/plan composition, so `docs/formal/` stays at the Cx8 verdict.

**Next is a new slice, and the choice is the user's.** `marketdata`'s book path (reconstruction, gap detection, resnapshot — the §3.4 machinery the funding path never needed) or `oms`, which **expires `singlewriter`'s deferral** and would complete the six-gate roster.

## §22 — session 20: `marketdata`'s book path COMPLETED and CLOSED — `build-spec.md` v0.9.10

### §22a — what was built

The book path was carried from "domain + use case, unwired" to composed and closed: `engine/venue`'s book DTOs (`BookSnapshot`, `BookDelta`, `BookLevel`, `LevelChange`, `BookSide`), `driving.BookDispatch`, `driven.BookLanePublisher`, and `module.go`'s second field with its two new `Deps` entries. `Module` now exposes `Funding` and `Book`; **mark** normalization stays unwired and the deferral is visible in the type rather than in prose.

### §22b — the decision the slice turns on: the delta payload carries `FirstSeq` and NOT `LastSeq`

§4 says the `Envelope` is authoritative for provenance and a payload carries none. A book delta breaks that cleanly in half: `domain.BookDelta` needs a *span*, and an envelope has one `VenueSeq`. So one end must travel in the payload, and the question is which, not whether.

`LastSeq` is the end the envelope already states — `domain.BookDelta.Validate` **requires** `Watermark.VenueSeq == LastSeq` — so a payload carrying it is §4's forbidden second copy in the one place the repo has already paid to learn the cost. `FirstSeq` is not provenance at all; it is the message's own extent, and nothing else states it.

**Rejected: carry both ends.** It reads as more complete and is strictly worse. It makes the payload internally consistent while contradicting its envelope, which is a *third* condition needing a check nobody would think to write. Under the split, the same defect is an inverted span and lands on `ErrDeltaSeqInverted`, which is exactly what it is.

The cost, paid knowingly: `venue.BookDelta.Validate` can no longer check the span, because it holds one end of it. That check lives where both ends meet, which is the domain — where it already was.

### §22c — the defect the wiring exposed: a book at position zero

`OnSnapshot` carried the comment *"Unreachable: a validated snapshot carries a non-zero VenueSeq."* It did not. `Watermark.Validate` **deliberately** admits a zero `VenueSeq`, because a REST-polled funding read genuinely has none — so a book could hold a complete watermark and no position.

The consequence is not a wrong book, and that is what makes it the interesting class: the book validates, is marked **synced** at position zero, and then `Follows(0)` returns `SeqRelationUnknown` for every subsequent delta forever. Each is refused, each refusal requests a snapshot, each snapshot re-establishes position zero. A permanent resync loop reporting a connected feed, a valid book and a healthy transport.

The fix is `domain.ErrBookSeqUnset`, and its placement is the **mirror rule** applied: `BookDelta` already refused an unset sequence, `BookSnapshot` did not, and they are the same claim about the same stream. No fixture reached it because every fixture states a sequence — the tell being that the fixture did not contain the thing under test.

### §22d — `StreamOrders` was missing from `platform/events`, and NO spec edit was owed

§4's `OrderGateway` declares `StreamOrders()` and `StreamExecutions()` as two channels; §3.5's product table names `StreamOrders` among the private streams. `platform/events` declared six kinds and not that one. **The spec was right and the code silently omitted a member** — the second instance after v0.9.8's `AcceptOrdinal`, which makes it a pattern rather than an incident.

**Rejected: fold it into `StreamExecutions`.** §4 notes fills arrive on both, which makes `StreamOrders` look like a superset. It is not, and §3.4's mailbox is why: `SourceID` is the mailbox key, so one kind puts an accept, a reject and a fill in one queue, and the exactly-once fill rule then has to deduplicate against a stream it shares that queue with.

Declared now rather than with `oms`, despite having no producer: the alternative is a **missing enum member**, which is precisely what rule 10 exists to make impossible, and two of the three switches over `StreamKind` default in the direction that is wrong for a private stream.

### §22e — the marker grep cannot see this class, and naming its pattern in code BREAKS it

Two limits of the slice-close gate were both demonstrated here in one session. It cannot find an *unmarked* claim — §22d is the proof, again. And writing a comment **about** the marker, inside `internal/`, makes the grep match forever: a gate whose pattern is a string is a gate the prose can trip. The comment was reworded. Worth stating because the natural instinct on finding §22d was to write the phrase down next to it.

### §22f — the fixture-collapse checklist, run before the mutations

Two collapses found and one written-then-corrected:

- **`First == Last` on the delta.** A fixture from a single-sequence venue (Bybit's real shape) makes "`LastSeq` from the envelope" and "`LastSeq` from the payload" **the same assignment** — the entire §22b design untested while looking covered. Every fixture now spans three sequences. This is the neighbour of the trap `Follows` already documents for `SeqRelationOverlap`.
- **Changes on one side only.** With bid-only changes, "translates the side" and "returns bid unconditionally" give the same book.
- **Corrected while writing:** the delta's inserted prices were first placed *outside* the spread, where a side-swapping translation still yields a **valid** book. Moved inside it, so a swap **crosses** and fails even with the value assertions deleted.

### §22g — mutations: 17 run, 15 caught, 2 SURVIVED, and the two needed OPPOSITE diagnoses

The pair is the reason `SURVIVED` is a question rather than a verdict.

**Snapshot payload unjudged — a THIRD copy of one check, not a test gap.** `toDomainLevels` validated each level as it copied it, so bypassing `venue.BookSnapshot.Validate` entirely still produced the venue sentinel and nothing could tell the two apart. The test was right; the code checked one thing in three places, and the boundary the check exists to *name* was untested while looking covered. Fixed by making `toDomainLevels` a pure translation that **cannot fail** — a helper called "to domain levels" that also refuses them is doing two jobs — which costs nothing, because `venue.BookSnapshot.Validate` already reports the side and index.

**Delta payload unjudged — a genuine test gap.** `toDomainChanges` validates only the *side*, so a negative quantity was checked in exactly one place. The new test uses a negative quantity specifically: a bad *side* would report the same venue sentinel either way and could not tell whether the payload was judged at all.

**One equivalent mutant, correctly.** Dropping `StreamOrders` from `Lane()`'s named case survives the suite, because both branches return `LaneControl` — no test can distinguish them. The guard is `exhaustiveswitch`, verified by hand against the mutant, and it is the known "a trailing `return X` hides the X group from tests" phenomenon.

### §22h — what is owed: nothing

`grep -rn "SPEC EDIT OWED" internal/ tools/` returns **zero**. `build-spec.md` at **v0.9.10** (§3.4 ×2, §4 ×1); the hold-buffer marker reads `[SPEC EDIT LANDED — v0.9.10]`. `make ci` green. **No formal-model change** — nothing here touches the fence/takeover/plan composition, so `docs/formal/` stays at the Cx8 verdict.

**Next is `oms`**, which expires `singlewriter`'s deferral and completes the six-gate roster. `StreamOrders` now exists for it.

## §23 — session 21: `oms/domain`, the sixth slice OPENED — the order lifecycle and the fill ledger

### §23a — what was built, and what this slice is NOT

`internal/oms/domain/`: `OrderState` (§5.3's fixed twelve-state set) with `Validate`, `IsTerminal` and `ContributesPossibleDelta`; `CanTransition` over SM §1's edge set; `Fill`, `FillKey` and `Ledger`. Five files, two test files, 22 mutations.

**The slice is OPEN, not closed.** There is no `oms/application/`, no `ports`, no `infrastructure/`, no `module.go`, and no `Trade` or `Order` aggregate — this is §5.3's *mechanism* layer only. `singlewriter`'s deferral has therefore **NOT expired**: it is scoped to "until `oms`'s write repository exists", and a repository is a port with an adapter, which is the next slice's subject rather than this one's. Recorded explicitly because the roster paragraph in §3.6 invites the opposite reading, and this repo's rule is that a deferral expires when its subject appears — the subject is the repository, not the package.

### §23b — the two silences in SM §1, both found by reading the table against the prose

Both are silences rather than disagreements (the v0.9.9 shape), and both fail the same way: **a stall, with every light green.**

**`DispatchAuthorized` has no way out.** SM §1's table gives it two rows — `send returns → Submitted`, and `no-return / crash → DispatchAuthorized (stays)`. The second names the resolution *in its action column* ("resolved by query only") and leaves the target column pointing back at itself, so the resolution's **outcome edges are unstated**. The requirement is stated three times elsewhere and reached the table none of them: core §5.3's rules paragraph ("an order at `DispatchAuthorized` or beyond is resolved **only** by `OrderByClientID`/`Fills`"), SM §1's own invariant ("crash recovery is defined for every pre-ack state"), and FLAG-2's ruling ("applies equally to `DispatchAuthorized`-recovery resolution"). Implemented as the table reads, a crash-recovered order is stuck in a state that contributes a possible-delta interval, so §6.7 classifies its hedge group **Ambiguous permanently**: the exposure stays open, new trading is held, an operator is alerted about an order the venue may have filled hours earlier, and nothing reports an error. **Owed: the five query-resolution edges, same set as `SubmissionUnknown`'s.**

**Venue-initiated termination is absent entirely.** Every path into `Canceled` leaves `CancelPending` or `CancelUnknown` — states entered by requesting a cancel *ourselves*. But §6 step 6 makes the execution pattern "parallel marketable-IOC limit orders", i.e. IOC is the **only** order type this system sends, and SM §2's finality contract states the consequence in its own step 1: **"an IOC is immediately terminal at the venue"**. A partially-filled IOC has its remainder canceled *by* the venue with no request from us; one that crosses nothing is canceled outright. Neither passes through `CancelPending`. So without these edges **the only order type this system sends can never terminate on its ordinary outcome** — it sits `Acknowledged` or `PartiallyFilled`, both delta-contributing, and the one-leg-failure handler waits for fills on an order the venue closed milliseconds after we sent it while a naked leg stays naked. **Owed: a venue-initiated termination row covering `Acknowledged`/`PartiallyFilled` → `Canceled` (IOC expiry, GTD expiry, venue cancel-all, self-trade prevention), distinct from the FLAG-1 rows, which are about a cancel WE requested.**

**Rejected framing for both: "the table is a summary, the prose governs."** It does not survive contact with §3.6 rule 10's own argument — the states are an enum-like precisely so a decision site cannot silently omit a member, and a transition *table* that omits an edge is the same defect one level up. SM is normative by reference; a mechanism document that cannot be implemented literally is one nobody can check an implementation against.

### §23c — `ContributesPossibleDelta` is not `!IsTerminal`, and Planned is the whole finding

§6.7's ambiguous-state list has **seven** members. There are **eight** non-terminal states. The difference is `Planned`, and §5.3 says why in a sentence written for another purpose: "a `Planned`-only order was never dispatched (no venue order can exist)". An order that cannot have reached the venue cannot move the venue's delta, so it contributes a **point — zero** — not an interval.

The negation is the plausible simplification, it reads as the conservative choice, and it **deadlocks the system**: §6.7 requires an ambiguous residual to be quiesced to finality *before* any compensating trade, and the finality contract resolves an order by querying the venue for it. There is nothing at the venue to query for a `Planned` order, so the contract runs to its deadline, the group enters `DeadlineExceededAmbiguous`, new trading is held and an operator is alerted — over an order the OMS has merely not sent yet and will happily leave unsent, **because §6.7 just told it to stop opening.**

### §23d — the mutation that found the mirror the reading pass missed

One `SURVIVED`, and it was a real gap of a **recorded** kind. `IsTerminal` and `ContributesPossibleDelta` both ended in `return false, s.Validate()`, which *looks* like the fix for the trailing-return-hides-its-own-group trap and is not: **`Validate` answers nil for every declared state**, so a member dropped from a *false-group* case falls through to `(false, nil)` — the identical answer, and the group stays untestable. Dropping `StatePlanned` from the no-delta case survived every test in the file, including the one written specifically for it.

What makes this worth recording rather than just fixing: the identical shape had been diagnosed and fixed **one function away**, in `successors`, thirty minutes earlier — where the terminal group would have been hidden the same way. This is the "a fix applied in one place gets applied to its mirror" rule, and the mirror was in the same file. The fix is validate-first, then switch, then an **unconditional** `ErrOrderStateUnhandled` reachable only from a *declared* state with no case.

### §23e — the third-copy check that was NOT written

`errors.go` was drafted with an `ErrFillAssetMismatch`. It was deleted before the ledger was written, because `fixedpoint.Quantity.Add` already refuses a disagreeing asset tag — so a check here would be §22g's surviving-mutant shape (a helper validating what its caller already validated, with no test able to tell which fired). The fold's own refusal stands, wrapped with the order and the execution. The deletion is recorded *in `errors.go`* as a parenthetical, so the next contributor does not add it back as an obvious omission.

### §23f — FillKey looks like a dedupe hole and is not, conditionally

§5.3's key is `UNIQUE(account, venue, product_category, symbol, execution_id)`. `FillKey` holds an `ids.VenueInstrument`, which carries a **`ContractType` the key does not name** — and a wider key is a *weaker* uniqueness constraint. It is safe because `VenueInstrument.Validate` refuses a Contract disagreeing with its Product, so the fourth field is functionally determined (ids' own doc calls it "a checksum").

**The conditional is the whole point.** An *unvalidated* `{linear_perp, inverse}` is a distinct map key from the `{linear_perp, linear}` naming the same real instrument, so the same execution applies twice. Hence `Key` validates rather than being an infallible constructor, and hence a test asserts the contradiction is refused at both `Register` and `Key`.

### §23g — the fixture-collapse checklist, run before the mutations; it changed four fixtures

- **One fill collapses sum, first and last.** Three fills at **1, 2, 4** make the total distinct from every individual fill *and every pair*, so no wrong fold reaches 7.
- **`Remaining` with ordered = 14 would equal cumulative.** Ordered **10** against 7 filled gives 3 — equal to neither operand, nor zero, nor any single fill.
- **The instrument-mismatch fixture was built from two symbols**, where "compares the whole instrument" and "compares the symbol" give the same answer. Rebuilt as **spot vs perp of the SAME symbol** — which is also the pair that actually occurs, since a cash-and-carry trade holds an order on each at the same instant. The mutation `f.Instrument != order.instrument` → symbol-only comparison is `caught` **only** because of this change.
- **The dedupe fixture varied the execution ID**, which passes against a key made of the execution ID alone. Rebuilt to hold the execution ID **fixed** while varying product and account — the §9 chaos case "identical spot/perp execution IDs".

### §23h — mutations: 22 run, 22 caught (after the §23d survivor was fixed)

Nine on the state machine, thirteen on the ledger. One `NOBUILD!!` and two `NO-OP!!` along the way, each the harness's own guard working rather than a result. The `successors` design was chosen *for* mutation observability: `(nil, nil)` for the three real terminals and an unconditional error at the bottom, so "Filled dropped from the terminal case" is `caught` by a test rather than only by `exhaustiveswitch`.

### §23i — what is owed: THREE markers, and the slice cannot close on them alone

`grep -rn "SPEC EDIT OWED" internal/ tools/` returns **three**: two in `transition.go` (§23b) and one in `fill.go` — §5.1's `Fill` field list carries **`Fee Money` AND `FeeAsset`**, a list that predates the money kernel, so the asset is stated twice and fee attribution and the tax export can reconcile to two different numbers. All three are silences or stale field lists; **none is a disagreement with the core.**

**No formal-model change** — nothing here touches the fence/takeover/plan composition, so `docs/formal/` stays at the Cx8 verdict.

## §24 — session 22: `oms`'s order intent, a fourth owed edge, and the dispatch ports

### §24a — what was built

`internal/oms/domain/order.go`: `Side`, `OrderType`, `TimeInForce` (§3.6 rule 8 enum-likes) and `Order` — §5.1's order minus the two things that change. `internal/oms/application/ports/`: `Journal`, `OrderPlacer`, `Ack`. `internal/oms/application/errors.go`: the refusals the dispatch path makes.

**The dispatch use case itself is NOT built.** Its design was taken and is recorded in §24e below in enough detail to be implemented without re-deriving it — that section is the deliverable of this record, not a summary of one.

### §24b — the fourth owed edge: `DispatchAuthorized → SubmissionUnknown`

Found by the same method as §23b's two — reading the table against the prose — but it is a **sharper** kind than either, and the sharpness is the finding. §23b's two silences are things the core requires *by implication*. This one the core **states aloud**: §6 step 6, "A timeout/5xx moves the order to `SubmissionUnknown`, resolved by `OrderByClientID`/`Fills`, never resubmitted." The order it moves is one at `DispatchAuthorized`, because the fsync of that state happens "immediately before the single `Place`". SM §1's only entry edge into `SubmissionUnknown` leaves `Submitted`.

**Neither path the table offers can carry a timeout, and each fails by recording something untrue.**

- **Through `Submitted`.** That state means "the single `Place` returned a **transport result**; awaiting/parsing venue ack". A timeout is precisely the absence of a transport result. §7.5 replays this journal to decide what to do about every order that was in flight when the process died, so the false record is not cosmetic — it is read back as evidence the request reached the venue's edge. (Note the split: a **5xx** *is* a transport result, so `DispatchAuthorized → Submitted → SubmissionUnknown` is correct for it and both statements already agree. The gap is the timeout, not the pair.)
- **Staying put.** The "no-return / crash → `DispatchAuthorized` (stays)" row's subject is a process that **died** — its action column is the recovery path, "resolved by query only". A call that returned an error is not a send still in flight, and the live path that just watched it fail is not crash recovery. Under this reading the live path would leave a timed-out order for a reconciler that is looking for crash survivors.

**The cheap argument is FLAG-1's own**, and it is worth more than either above because it is the spec arguing with itself: the round-4 ruling created `CancelUnknown` as a **first-class entry edge** on an "ambiguous cancel transport result … **by symmetry with `Submitted → SubmissionUnknown`**", rather than routing it through a state claiming an ack had been parsed. This is that same edge one step earlier. The asymmetry is its **absence**, not its presence.

**Rejected: "the timeout and the crash are the same event, so the stays-row covers it."** They are the same event to the *venue* and different events to *us*, and the state machine is ours. The whole purpose of `DispatchAuthorized` is to be the state a process that died leaves behind; a live process that knows its call failed holds strictly more information, and a machine that discards it makes recovery re-derive at boot what the process knew at the time.

**Owed to SM §1's transitions table:** `DispatchAuthorized → SubmissionUnknown` on an ambiguous transport result (timeout / 5xx / unknown ack), **distinct** from the "no-return / crash" row.

The **oracle caught the omission the moment the edge landed** — `TestCanTransitionMatrix` failed on exactly this pair. That is the "different shape from the implementation" rule paying: the transcribed edge list is flat `from->to` strings and the implementation is a switch returning a slice per source state, so the edge had to be added in both places by two different acts.

### §24c — `Order` carries no `State`, and that is §5.3's orthogonality, not a shortcut

§5.1's field list ends `…, State`. `domain.Order` has no `State` field and no fill list, because this package already holds both mechanisms separately and §5.3 makes them **orthogonal on purpose**: "order state tracks submission/finality; the fill ledger tracks realized quantity". A struct carrying an identity beside a mutable `State` is a second home for a fact `CanTransition` governs, and the two disagree the first time something writes one without the other.

So `Order` is the **immutable half** — everything fixed at planning time that no venue event changes. That is exactly what §6 step 5 fsyncs and exactly what §4's `Place` takes. The aggregate binding an `Order` to its state and fills **composes** this when it lands; it does not restate it.

**`PositionSide/Idx` is left out as a layering call, recorded rather than dropped.** §4.1 pins one-way position mode and §7.5 makes a Hedge position mode a refuse-to-trade condition asserted at boot and before every trade, so the field's legal domain here has exactly one member. What the venues need is a wire field whose spelling differs between them (Binance `positionSide`, Bybit `positionIdx`), derivable from the pinned mode. Carried instead, it would be a field with one legal value that a construction site can set wrong and that `Validate` could only check against a constant. **No marker** — this is layering, not a contradiction of a normative claim. It goes in the slice-close reading pass.

### §24d — the price rule that runs in the direction nobody writes

`Order.Validate` checks the type × price pairing **both** ways. "A limit order needs a price" is the check that gets written, and it is the one that fails loudly at the venue anyway. The mirror — **a market order MUST NOT carry a limit price** — fails silently everywhere: §6 step 4 sizes the naked-leg price gap against the price the order is supposed to be bounded by, so a market order carrying one presents a bound the venue ignores. The order fills through it and the slippage budget reconciles against a number that never bound anything.

"Absent" has exactly one spelling — the zero `fixedpoint.Price` — because fixedpoint's zero value is unusable by construction (§3.6 rule 4's design), so a caller cannot express absence two ways.

### §24e — THE DISPATCH USE CASE, AS DESIGNED AND NOT YET BUILT

Everything here was decided this session with the alternatives weighed. It is written out so the next session implements rather than re-derives.

**Signature, and the whole reason for its shape:**

```go
func (d *Dispatcher) Dispatch(o domain.Order) (Outcome, error)

type Outcome struct {
    State     domain.OrderState // where the order actually is
    Ack       ports.Ack
    Cause     error // why State is not StateSubmitted; nil on a clean place
    Journaled bool  // whether State reached the journal
}
```

**`err != nil` means NOTHING WAS SENT** — the order is still `Planned` and §5.3's "safe to replan or discard" applies. `err == nil` means `Place` ran exactly once and `Outcome.State` is where the order is.

**The rejected signature is `(domain.OrderState, error)`**, and rejecting it is the point. Under it, the returned state is authoritative *even when the error is non-nil* — a contract that reads fine and that the universal Go reflex `if err != nil { return }` silently breaks, dropping an order that may be live at the venue. Splitting the two makes the dangerous mistake **impossible to express**: when `err != nil` there is no state to ignore, because nothing was sent.

**The sequence:**

1. `admit(o)` — see below. Failure ⇒ `return Outcome{}, err`. Nothing journaled, nothing sent.
2. `journal.AppendState(id, StateDispatchAuthorized)`. Failure ⇒ `return Outcome{}, err` — **the `Place` MUST NOT run**, because a send whose authorization is not on disk is an order a crash cannot find. The port's error is read as "not durable", and that reading is safe **only because the journal is local**: a failed local fsync is a failed fsync, unlike the network write that follows it. That asymmetry is the entire reason the ordering is journal-then-network.
3. Record the id in the in-process `authorized` set (only now — steps 1 and 2 sent nothing, so a corrected retry there is legitimate).
4. `ack, placeErr := placer.Place(o)` — **once**, no retry, on any error.
5. `state, unhandled := classifyPlace(placeErr)`.
6. `journalErr := journal.AppendState(id, state)`.
7. `return Outcome{State: state, Ack: ack, Cause: errors.Join(placeErr, unhandled), Journaled: journalErr == nil}, nil`.

**Step 6 failing is not a safety failure, and the reason is worth keeping:** the journal still holds `DispatchAuthorized`, which recovery resolves by query — exactly the right behaviour. It is an alerting matter (the journal is failing, which threatens the *next* order), not a correctness one. Hence `Journaled` is a field rather than a returned error.

**`admit(o)` — three checks, in order:**

1. `o.Validate()`.
2. `ids.ParseClientOrderID(string(o.ClientOrderID))` ⇒ `ErrOrderNotOMSMinted`. **This is why `domain.Order.Validate` deliberately does not parse**: §6 step 10's foreign orders carry venue-minted ids and the reconciler must hold them. The strict check belongs at the one place an id stops being something we read and becomes something we write to a venue.
3. `o.Type == OrderTypeLimit && o.TimeInForce == TimeInForceIOC` ⇒ else `ErrNotMarketableIOCLimit` (§6 step 6). **A comparison, not an exhaustive switch, and that is deliberate** — a member added to either enum next year is refused without anyone editing this line. The gate-checked switch is the loud shape everywhere else in this repo and the wrong one here, because it would have to be re-answered, and answered wrongly once, for a value nobody intended to send.

**`classifyPlace(err error) (domain.OrderState, error)` — the safety-critical decision of the use case.** A **tagged** `switch apperror.EffectOf(err)`, so §3.6 rule 10 forces all four `Effect` members to be named:

| Input | State | Why |
|---|---|---|
| `err == nil` | `Submitted` | §5.3's own definition of the state |
| `EffectApplied` | `Submitted` | the send definitely happened; the *ack* is what could not be read, which is exactly what `Submitted` means. Not `SubmissionUnknown` — that state is ambiguous about **whether we submitted**, and here we are not |
| `EffectNotApplied` **and** `ClassOf == ClassVenueReject` | `Rejected` | FLAG-2 requires an **explicit venue rejection** for this terminal, and this is precisely that evidence class |
| `EffectNotApplied`, any other class | `SubmissionUnknown` | see below |
| `EffectUnknown`, `EffectUnspecified` | `SubmissionUnknown` | `EffectUnspecified` is `apperror`'s deliberate degrade-to-safe |
| fallthrough (an undeclared `Effect`) | `SubmissionUnknown` **+ `ErrEffectUnhandled`** | see below |

**The `EffectNotApplied`-without-`VenueReject` row is the one that took the argument.** We *know* no order exists — a connection refused before a byte was written, a local governor declining, our own `Validate` refusing — so a terminal looks justified and one query looks wasted. It is refused on evidence-class grounds: `Effect` answers "did the side effect happen", **not** "which evidence class proves it didn't", and SM §1's two absence terminals are distinguished by exactly that (FLAG-2: proved absence and venue rejection are different evidence classes). `EffectNotApplied` explicitly covers both "never left the socket" **and** "the venue rejected it on its merits", so the value alone cannot choose between `Rejected` and `AbsentAfterFinality` — and `AbsentAfterFinality` is unavailable anyway, since it requires the stable-not-found rule to have *completed*. Inventing either would write an evidence class into the journal that no evidence supports. One resolving query converts an honest ambiguity into the right terminal; the cost is one REST call on a path that already failed.

**The fallthrough returns a state AND an error, and this is §23d's lesson applied one layer up.** It must not return only an error: `Place` has already run, so a caller left without a state has a possibly-live order and nowhere to record it. It must not return only a state: `SubmissionUnknown` with `nil` is the identical answer to the `EffectUnknown` case, so dropping a member from that case would change nothing any test could observe — the trailing-return-hides-its-own-group trap. Returning `(SubmissionUnknown, ErrEffectUnhandled)` makes the group observable while keeping the state safe. **The contract to state in the doc comment: the state is always safe to act on; the error means the classification came from the fallback rather than from a rule, and must be reported.**

**The in-process double-dispatch guard** — `authorized map[ids.ClientOrderID]struct{}` ⇒ `ErrOrderAlreadyDispatched`. It is a **backstop, not the mechanism**: durable enforcement is the journal, and a fresh process correctly starts empty, because an order it finds at `DispatchAuthorized` must be queried about rather than dispatched again. It earns its place because it defends against the misuse the *use case's own signature* invites — a caller treating a failed dispatch as retryable. Follow `Ledger`'s precedent exactly, including the **not-safe-for-concurrent-use** statement and its reason: §5.1 gives the aggregate an OCC `Version` and §6 a journal-then-apply write path, so serialisation is decided one layer out, and a mutex here would make two writers safe from the race detector while leaving them just as wrong about the version.

**Assert the classifier against SM §1 in a TEST, not at runtime.** Every state `classifyPlace` can return must be a successor of `DispatchAuthorized` under `domain.CanTransition`, and the returned set must be exactly `{Submitted, SubmissionUnknown, Rejected}`. A runtime guard was considered and rejected: its only possible fallback is `SubmissionUnknown`, which is what the fallthrough already returns, so it would change no behaviour and cost a branch on the dispatch path. **This is why the fourth owed edge had to land first** — `DispatchAuthorized → SubmissionUnknown` is one of the three, and without it this test fails.

**Fixture-collapse checklist to run BEFORE the mutations on this file:**
- The journal fake must distinguish **which** append failed. A fake that fails every call cannot tell "refuses to place when authorization is not durable" from "reports any journal error", and those are the two claims of steps 2 and 6.
- The fake must **count** `Place` calls, not merely record the last one — the single-`Place` rule is a count, and every other assertion is satisfied by a dispatcher that calls it twice.
- The `EffectNotApplied` cases must include **both** `ClassVenueReject` and a non-reject class. With only one, "checks the class" and "ignores the class" give the same answer.
- The `admit` cases must vary the order type and the TIF **independently** — a fixture that is wrong in both cannot tell which check fired.
- At least one case must have a **valid** order that the guard refuses (the second dispatch), or "refuses a repeat" collapses into "refuses an invalid order".

### §24f — §3.6 rule 2 is silently unmet in `marketdata`, on both halves

Rule 2 reads: "**Ports in `application/ports/`.** Every driven dependency is an interface there; adapters live in `infrastructure/driven/` with a compile-time `var _ ports.X = (*Impl)(nil)`." The assertion is written `var _ ports.X`, so the rule presupposes the package.

`internal/marketdata` declares its ports **flat**, in `application/ports.go`, package `application`; and `grep -rn "var _ " internal/marketdata/infrastructure/` returns **nothing**. Both halves unmet, unmarked, in a **closed** slice. Decision record §7 had already ruled *for* the subdirectory ("`application/ports/` survives the Go-idiom objection, because `application/` *is* the consumer"), so this is drift against a recorded decision rather than an open question.

**This is the third demonstration of the class the OWED grep cannot see** — after v0.9.8's `AcceptOrdinal` and v0.9.10's missing `StreamOrders` — and the cheapest one yet to state: the grep finds claims the code *marked*, and a silently unmet rule is by definition unmarked.

**`oms` follows rule 2 as written**, and the resulting inconsistency between the two contexts is itself the flag. **Not fixed here**: moving `marketdata`'s ports is a real refactor of a closed slice (`FundingSnapshot`/`BookView` have methods and are used by the use case, so they move to `ports` and come back as imports), and scaling that into this slice is the user's call, not one to absorb. The `var _` assertions are separately cheap and should land regardless.

**Note the cost the subpackage imposes, since it constrains what may live there:** `ports` must not import `application`, so every type crossing one of these seams lives in `ports` or `domain` — never in `application`. That is a discipline with a useful side effect: a use-case type that would be convenient to hand an adapter has to be moved deliberately, which is the moment somebody asks whether the adapter should see it at all.

### §24g — mutations: 11 run, 11 caught

Ten on `order.go`, one on the new transition edge. The fixture-collapse checklist ran first and shaped four fixtures: quantity varies **zero and negative** (`<= 0` and `< 0` agree on a negative), the type × price matrix walks **all four cells** rather than the diagonal, each enum is refused on both an empty **and** a plausibly-miscased value (`"SELL"`, `"LIMIT"`, `"IOC"` — the mapping bug that actually happens is a venue token reaching the field unnormalised), and both sides are accepted **through `Order.Validate`** rather than only through `Side.Validate`.

`validatePrice`'s fallthrough is unreachable through the exported API — `Validate` runs `Type.Validate` first — so it is asserted from an **in-package** test, with a positive control that each declared type reaches its own case. Without the pair, an implementation whose switch had no cases at all would pass the reachability test and fail nothing else.

### §24h — what is owed: FOUR markers, and `singlewriter` still has not expired

`grep -rn "SPEC EDIT OWED" internal/ tools/` returns **four**: three in `transition.go` (§23b's two + §24b) and one in `fill.go`. All four are silences or stale field lists; **none is a disagreement with the core.**

`singlewriter`'s deferral is **still unexpired**. `oms/application/ports/` declares no repository — `Journal` is a write-ahead log, not §3.6 rule 6's "OMS write repositories" — and there is no `infrastructure/driven/`. The subject is a port **plus** an adapter, and neither exists.

**No formal-model change** — nothing here touches the fence/takeover/plan composition, so `docs/formal/` stays at the Cx8 verdict.

## RESUME HERE — updated end of session 22

**SESSION 22 UPDATE — READ THIS FIRST.** The **sixth slice (`oms`) is still OPEN**. `oms/domain` gained §5.1's order intent (`Side`, `OrderType`, `TimeInForce`, `Order`) and a **FOURTH** owed edge — `DispatchAuthorized → SubmissionUnknown`, the one the core states aloud (§24b). `oms/application/ports/` declares `Journal` and `OrderPlacer`; **the dispatch use case is designed but NOT built, and §24e is that design written out to be implemented rather than re-derived** — start there. `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **FOUR**, correct mid-slice. **`singlewriter` has still NOT expired**: a write-ahead journal is not a write repository, and there is no `infrastructure/driven/`. Doc set is **still v0.9.10** and correctly so. New finding to act on: **§3.6 rule 2 is silently unmet in `marketdata` on both halves** (§24f) — the user's call whether to fix it. `make ci` green, 11 mutations / 11 caught. The reasoning is **§24**; §23 is now history. What follows was written at the end of session 21 and earlier.

## RESUME HERE — updated end of session 21

**SESSION 21 UPDATE — READ THIS FIRST.** Milestone 1's **sixth slice (`oms`) is OPEN**, not closed. `internal/oms/domain/` holds §5.3's order lifecycle and fill ledger; **nothing above `domain/` exists yet**, so `singlewriter`'s deferral has **not** expired — its subject is the write *repository*, which is a port plus an adapter and belongs to the next slice. `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **THREE** (2 × `transition.go`, 1 × `fill.go`) — that is correct mid-slice; markers land at slice close. `make ci` green, 22 mutations / 22 caught, one diagnosed survivor fixed (§23d). Doc set is **still v0.9.10** and correctly so: no spec edit has landed yet. The reasoning is **§23**; §22 is now history. What follows was written at the end of session 20 and earlier.

## RESUME HERE — updated end of session 20

**SESSION 20 UPDATE — READ THIS FIRST.** `marketdata`'s **book path is COMPLETE and the slice is CLOSED**. `build-spec.md` is at **v0.9.10** with §3.4 ×2 and §4 ×1; `platform/events` gained **`StreamOrders`**, which was a code omission against an already-correct spec and therefore owed no edit. **`grep -rn "SPEC EDIT OWED" internal/ tools/` is CLEAN.** `make ci` green, 17 mutations / 15 caught / 2 diagnosed survivals (opposite diagnoses — §22g). The reasoning is §22; §21 is now history. **Next is `oms`.** What follows was written at the end of session 19 and earlier.

## RESUME HERE — updated end of session 19

**SESSION 19 UPDATE — READ THIS FIRST.** The `engine/venue` slice is **CLOSED**. `build-spec.md` is at **v0.9.9** with §3.5 ×2 and §4 ×1; all five doc stamps and the Georgian header are re-synced, `tutorial.ka.md` re-assembled, ADR §3 carries the **fifth** milestone-1 block (tagless total **thirty-three**), and the three code markers read `[SPEC EDIT LANDED — v0.9.9]`. **`grep -rn "SPEC EDIT OWED" internal/ tools/` is CLEAN.** `docs` 102 pass, `make ci` green. **Nothing is owed.** The reasoning is §21; §20 is now history. What follows was written at the end of session 18 and earlier.

## RESUME HERE — updated end of session 18

**SESSION 18 UPDATE — READ THIS FIRST.** `engine/venue` is **built and NOT closed**. §20 is the record. `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **three** — `engine/venue/doc.go`, `engine/venue/funding.go`, `platform/events/envelope.go`. `make ci` green, 16 mutations / 14 caught / 2 diagnosed survivals. `driving.FundingUpdate` no longer exists. The next work is §20h. What follows was written at the end of session 17 and earlier.

## RESUME HERE — updated end of session 17

**SESSION 17 UPDATE — READ THIS FIRST.** The `marketdata` slice is **CLOSED**. The §18f fix is applied (`c8db027`) and `build-spec.md` is at **v0.9.8** (`9136f90`), with all five doc stamps, ADR §3's **fourth** milestone-1 block (tagless total **thirty**), six code markers flipped to `[SPEC EDIT LANDED — v0.9.8]`, the Georgian header re-synced and `tutorial.ka.md` re-assembled. **`grep -rn "SPEC EDIT OWED" internal/ tools/` is CLEAN.** `make ci` green, `docs` 102 pass, 7/7 mutations caught. **Nothing is owed.** The reasoning is §19; §18 is now history. What follows was written at the end of session 16 and earlier, and the environment paragraphs below are still accurate.

## RESUME HERE — updated end of session 16

**SESSION 16 UPDATE — READ THIS FIRST.** The `marketdata` slice's **code is complete** (commit `c695cd2`: `infrastructure/{driven,driving}` + `module.go`), `make ci` green, 18/18 mutations caught. **The slice is NOT closed and the full close was user-approved — the complete owed list is §18h, and it starts with a code fix (§18f), not with a doc edit.** Five `[SPEC EDIT OWED]` markers are open; the grep is still `grep -rn "SPEC EDIT OWED" internal/ tools/` — **both paths**. The two spec questions session 15 left open are **ANSWERED** (§18g) — do not re-ask them. What follows was written at the end of session 15 and is otherwise still accurate.

## RESUME HERE — updated end of session 15

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

**`platform/ratelimit` (session 11) and `platform/boot` (session 12) are committed but NOT
pushed** — check rather than trust this sentence. Both branches were in sync at the end of
session 10.
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
EIGHT packages in:** `platform/apperror` and `platform/ids` (session 6, with the `Venue` move out of
`platform/clock`), then `ids`'s instrument identity and **`platform/events`** (session 7), then
**`platform/config`** (session 8), then **`platform/outbox`** (session 9), then
**`platform/observability`** (session 10), then **`platform/ratelimit`** (session 11), then
**`platform/boot`** (session 12). The kernel is **ten packages** — `fixedpoint`, `clock`, `apperror`,
`ids`, `events`, `config`, `outbox`, `observability`, `ratelimit`, `boot` — plus three gates.
**There are no open questions.**

**§1's deferred kernel table is EXHAUSTED (`boot` was the last one) and the SECOND SLICE IS CLOSED
(session 13):** the eight spec edits landed as `build-spec.md` **v0.9.6**, the doc set is stamped,
and nothing is owed. **Session 14 then landed §15b's four as a doc-only `build-spec.md` v0.9.7**
(§16) — the user's call, taken before the next slice rather than folded into it, so that
`grep OWED` returning zero once again means *code and spec agree* rather than *nobody flagged
anything*. The next work is not another kernel package — it is **the first bounded context, and the
user has chosen `marketdata`**. See "Next work" below.

**The engine module still has ZERO production dependencies** (only test-only `pgregory.net/rapid`).
That is now a decision with a record rather than an accident: §12a put taking
`prometheus/client_golang` to the user and it was deferred to the composition roots. Do not add it
casually — `kernelpurity` will not stop you, because §3.6 rule 9's scope is in-module imports and
third-party is deliberately out.

**`observability`'s PROMETHEUS ADAPTER and TRACING are owed** — the adapter lands with `boot`/`app`
where the per-binary `/metrics` decision lives (§11 gives `arblab-actuator` no HTTP surface at all),
and tracing lands once §3.5's `DispatchSeq` exists to hang a span on. Both deferred on argument
(§12a, §12f), not forgotten.

**SESSION 15 UPDATE — READ THIS BEFORE THE PARAGRAPHS BELOW.** Two `[SPEC EDIT OWED]` markers are
now open, both in `tools/lint/` and both about §3.6's own status text (§17e). The slice-close grep
is now **`grep -rn "SPEC EDIT OWED" internal/ tools/`** — scoped to `internal/` alone it returns
**0 while both markers exist**, which is the third instance of a gate reporting clean because it
cannot see its subject. The analyzer roster is **5 of 6 built**; only `singlewriter` is deferred.
`mutate.sh` needs **`MUTATE_MODULE=tools/lint`** to reach them, and has a sixth verdict, `PANIC!!`.
`marketdata` now has `domain/` **and** `application/`; `infrastructure/` and `module.go` are the
next work. What follows was written at the end of session 14 and is otherwise still accurate.

**NOTHING IS OWED. The doc set is at v0.9.7** (session 14, §16 — §15b's four, landed doc-only);
the eight before them landed as **v0.9.6** (session 13, §15).
`grep -rn "SPEC EDIT OWED" internal/` returns **zero**; the eight markers now read
`[SPEC EDIT LANDED — v0.9.6]`, ADR §3 carries the milestone-1 second-slice block, and all five
doc-set stamps are at v0.9.6. **Do not re-apply any of them.** What follows is the historical
statement of what each one was, kept because the reasoning is why the rules exist:
§14b: §7.5 gives every one of its preconditions the same verb — asserted before trading resumes —
while stating IN THE SAME PARAGRAPH two conditions that do not clear the same way (a MarginOp
resolves through its evidence contract; a foreign order "alerts for operator clearance" and is "not
auto-corrected"). §7.4 then makes the second kind explicit for the fence — clearance AND a fresh
propagation test, "never health-flap re-entry". So the spec has latching preconditions and
self-clearing ones and never says the distinction exists; a uniform checklist lets a retry re-admit
trading, which is §7.4's forbidden re-entry arriving through §7.5's door.
§14d: §3.2 states boot's phase roster as a LIST in a table cell and never says the positions are
constraints. Two are load-bearing and both failures are silent — signal installed late means an
interrupt during a slow boot kills the process on the default disposition with no flush, and
observability started late means a boot that dies in the journal join dies with an exit code as its
only diagnosis.
§14e: §7.4 gives the watchdog a host-silence trigger and §7.5 gives the process a graceful shutdown,
and nothing joins them — but a total stop budget at or above the threshold makes an ORDINARY RESTART
indistinguishable from a dead host, so the actuator fences a bot mid-flush and the restart comes back
up fenced, needing operator clearance.
§13a: §7.6 states a backoff THRESHOLD ("~70% weight") without saying what backs off, while §7.1 and
§6.7 state a RESERVATION without saying how much or what happens when it runs out — so the spec
relies, unstated, on the priority ordering that makes the two one mechanism, and on capacity being a
ceiling rather than a bypass for the safety class. Both were decided in the code (the second by the
user) and neither is in the spec.
§12c: §10 requires metrics cut per venue and per symbol and says nothing about label CARDINALITY,
while §3.4's lossy-lane guarantee covers only a stalled consumer — not the unbounded time-series
growth that takes the process down with every scrape succeeding and zero drops reported; the code
makes the bound explicit and default-deny.
§9h: §3.5 declares `SourceID` as four flat fields; the code composes `ids.VenueInstrument` instead,
because the flat form cannot express the `ContractType` §5.1 gives an instrument. §10b: §5.2 says
configuration is "hot-reloadable where safe" and stops; the code makes "where safe" a per-field
**default-deny** `config:"hot"` tag, on rule 4's own argument. §11b: §3.2 and D2 describe the outbox as
an at-least-once export and say nothing about ORDER, while relying on it — audit is called "a
projection of the OMS journal", and a projection that applies a fill before its order does not
reproduce its source; the code makes the guarantee explicit and scopes it to the Key.

**All eight landed together as v0.9.6 (session 13), on the v0.9.5 precedent** — batch a slice's spec
edits into one coherent release at slice close, so external review reads deliberate change rather
than drift. The gate was `grep -rn "SPEC EDIT OWED" internal/` (**code only** — the doc set is full
of PROSE about past markers, so a repo-wide grep is mostly false positives): it returned exactly
**eight** before, and returns **zero** now. The release was held to exactly those eight; four
unmarked candidates were recorded in §15b as a scope call for the user, deliberately not smuggled
in — and **session 14 landed all four as v0.9.7** (§16), which is where the limit of that gate is
written down: zero markers proves nothing was *flagged*, not that code and spec *agree*.

**Release checklist, for the next slice that closes** (§15c is why the last item is on it):
`grep -rn "SPEC EDIT OWED" internal/` → land what it finds → **then a reading pass over what the
slice's code asserts *normatively* and the spec does not** (§16b: the grep cannot see an *unmarked*
claim, which is how four of them sat through a whole release with the gate green; the question to
ask each one is "could a reimplementation get this wrong in the **unsafe** direction while still
satisfying the core as written?") → bump **all five** doc-set stamps → ADR §3 block → flip the code
markers to `[SPEC EDIT LANDED — v<version>]` → **re-sync `i18n/ka/tutorial/header.md`'s status line
to the new English one and `node i18n/tools/assemble.mjs`** → **`cd docs && bun test`**, because
`tutorial.ka.md` is generated and its stamp is a *copy* of the English one; the invariant checker
catches the drift and refuses to assemble, but only if somebody runs it → `make ci`.

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
   bite-checked at 8/8 and 16/16 mutations. **[SUPERSEDED session 15: the roster is now 5 of 6 —
   `hexboundary` and `exhaustiveswitch` landed, §17b/§17c. Only `singlewriter` is deferred.]**
   **The analyzer roster is 3 of 6 built**; the other three
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

17. ✅ **`platform/ratelimit`** (§13a–§13g) — `Priority`/`Dimension`/`Scope`, `Limit`/`Cost`/
    `Profile`, the sliding `window` with its upward-only ratchet, and `Governor` with
    `Check`/`Acquire`/`Observe`/`Penalize`/`Snapshot`. 46 mutations, 46 caught, 2 documented
    equivalents. **Do not put venue numbers in it** — §13's opening records why the argument is
    stronger here than the `config`/`apperror` precedent it follows.
    **Do not let `PrioritySafety` past `Capacity`** — §13a, the user's decision, with the rejected
    alternative and its three counter-arguments recorded.
    **Do not make `Observe` adopt a LOWER reading** — §13b; a stale header is exactly what a lower
    reading looks like, and adopting one discards every request sent after it.
    **Do not add a `Requests` field to `Cost`** — §13c; the IP envelope is the one budget with no
    header to correct a mistake, so it is the one a caller must not be able to forget.
    **Do not "simplify" the sliding window into a fixed one** — §13e; a wrong guess at the server's
    boundary alignment doubles the rate across the seam.
    **Do not give the governor a `Wait` or a goroutine** — §13's opening, `clock`'s call for
    `clock`'s reason, plus §6.3 needs a predicate rather than a wait.
    **Do not report the EARLIEST release instant when several limits refuse** — §13f.

18. ✅ **`platform/boot`** (§14a–§14i) — `Posture`/`Act`, `Gate`/`Outcome`/`Checklist`,
    `PhaseKind`/`Phase`/`Sequence` with §3.2's ordering constraints, `Run`/`Shutdown`, and the signal
    policy. 62 mutations, 62 caught, no survivors. **This exhausts §1's deferred kernel table.**
    **Do not collapse `Posture` into a boolean or an error** — §14a; §7.5 has three outcomes and the
    third is the one the package is named for, so a `func() error` loses it in the signature.
    **Do not add a read `Act`** — §14a; a posture able to withhold a read could blind the monitoring
    that decides when to escalate.
    **Do not make `Clear` return a gate to `OutcomePassed`** — §14b; §7.4 requires clearance AND a
    fresh propagation test, so clearance restores the ability to pass, not the pass.
    **Do not make every outcome freely re-reportable** — §14b, the same rule from the other side.
    **Do not add a `Reconciler` port** — §14h; worse than `outbox`'s rejected `Append`, because a
    nil-returning implementation reports every gate `Passed` with nothing joined and the boot is
    green. It lands with the journal, in milestone 3.
    **Do not close the `PhaseKind` roster** — §14d, the user's decision, with both rejected shapes
    recorded; §11's binaries have different phase sets.
    **Do not add an early exit to `Shutdown` when the budget runs out, and do not give it a
    goroutine** — §14f; reverse order makes the skipped closers the foundational ones, and skipping
    shortens nothing because this package cannot make the process exit.
    **Do not lower `SignalBufferMin` or drop the `Install` buffer check** — §14g; `os/signal` drops a
    send to a full channel silently, and the dropped one is the second interrupt.
    **Do not delete `minPostureFor`'s final return when a mutant survives on it** — §14i; it is
    unreachable from `Permits` and not deletable either, and `posture_internal_test.go` pins it.

### Next work, in order

0. ~~**DECIDE WHAT CLOSES THE SLICE**~~ — **DECIDED and DONE (session 13, §15): (a), the eight spec
   edits, landed as `build-spec.md` v0.9.6.** The slice's own convention said the spec edits come
   first, and they did. **The second slice is CLOSED.** Do not re-open it and do not re-apply any
   of the eight.
0b. ~~**§15b's four**~~ — **DECIDED and DONE (session 14, §16): landed as `build-spec.md` v0.9.7,
   doc-only, before the next slice started.** Do not re-apply them.
0c. **NEXT SLICE: `marketdata`, the first bounded context** — **DECIDED by the user (session 14).**
   It is what makes `hexboundary` and `exhaustiveswitch` buildable for the first time (§1, §4 defer
   them on "a lint over an empty tree passes trivially" — indistinguishable from a broken check).
   **`singlewriter` stays deferred**, because it is the one gate that genuinely needs `oms`'s write
   repository; two of the three wake here, not three. **The alternatives and why they lost, so this
   is not silently reopened:** `oms` is the context §3.2 makes the single-writer authority and the
   one all three gates are written about, but it needs the journal — and `platform/outbox`'s store
   half is deferred to milestone 3 (item 1b below), so `oms` first means either pulling milestone-3
   work forward or building against a **stubbed write repository**, which is precisely the hazard
   §14h rejected for the `Reconciler` port: *a nil-returning implementation reports every gate
   passed with nothing joined, and the boot is green*. `strategy` is closest to the milestone-2
   verdict but consumes `marketdata`'s output, so it would be built against invented inputs. The
   decisive argument is the kill gate: **milestone 2's verdict is measured net carry, and you
   cannot measure carry without funding-rate data while you can measure it without ever placing an
   order** — `marketdata` is on that critical path and `oms` is not.
1. ~~**`platform/boot`**~~ — **DONE (session 12, §14).** §1's deferred kernel table is exhausted.
   **The parity dispatcher is what `events` was the last prerequisite for** — but it also needs a
   persisted `DispatchSeq` log, so it is not simply "next" (§9b).
1b. **`platform/outbox`'s store half is OWED, and it lands with the OMS journal in milestone 3** —
   the `Append` verb, the SQL, the migration and the driver. It is deferred on §11a's argument, not
   forgotten. When it lands, the thing to check first is that the append and the journal write are
   genuinely one transaction, because everything in `platform/outbox` assumes it and nothing there
   can test it.
2. ~~**The three sleeping gates split at item 0c**~~ — **DONE (session 15, §17b/§17c).**
   `hexboundary` and `exhaustiveswitch` are built, registered and bite-checked, and they went in
   **before** `application/` rather than after it: §1's "so the code they police is never written
   unpoliced" applies the moment the deferral's subject exists. `singlewriter` remains deferred on
   unchanged reasoning. Historical statement below.

   **The three sleeping gates split at item 0c, and the split is the point.** `hexboundary` and
   `exhaustiveswitch` become buildable **with `marketdata`** — a context with `domain/` /
   `application/` / `infrastructure/` layering is all either needs to police, and building them
   against the first context is what proves the layering rather than asserting it. **`singlewriter`
   stays deferred** until `oms`'s write repository exists; it is the one gate whose subject is
   genuinely absent, and a lint over an empty tree passes trivially (§1, §4 — unchanged reasoning,
   now scoped to one gate instead of three).
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
- **A test that HANGS is not a test that fails, and the fix is a bounded receive — this is now
  twice.** §5e found it in `clock`; §14i found it in `boot`, where breaking the signal subscription
  made `TestInstallDelivers` block forever on an unbounded channel read rather than fail. Both
  claims that test made about `Install` actually subscribing were **unchecked while looking
  covered**, and the harness reported `TIMEOUT!!` rather than a kill. **Any test that receives from a
  channel a mutation could stop feeding needs its own bound**, or the mutation that breaks the feed
  reports nothing.
- **The fixture-dimension lesson has now fired SEVEN times, so treat it as a checklist item rather
  than a war story.** §5d: 162 of 348 fuzz corpus entries reached the oracle. §9i: every coalescing
  test used a mailbox depth of 1, where "merge against the newest" and "merge against the oldest" are
  the same slot. §11f: every malformed-identifier fixture put its bad character mid-string, where
  `i >= 0` and `i > 0` are the same predicate — so `" audit"` validated while the check looked
  covered. §12g: the series-key collision fixture `("ab","c")` vs `("a","bc")` contained no
  separator, so it collided only under the *empty* separator and passed against every wrong choice a
  person would actually make. §13g twice in one session: two `ratelimit` tests used a profile whose
  worst refusal happened to be **first in evaluation order**, making "name the first" and "name the
  worst" the same answer; and every `freeAt` case sat strictly inside or strictly outside the
  ceiling, where `<=` and `<` are the same predicate. **Before believing a validation suite, ask
  which fixture parameter is pinned to the value that collapses the distinction the code makes**, and
  vary it. §14i applied the checklist **in advance** for the first time and it changed five
  fixtures before a single mutation ran — most cheaply, a reverse-teardown test using three closers,
  where reversing the slice and swapping only its ends are the same order. The tell is constant
  across all seven: **the fixture does not contain the thing under test.**
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

### Next work, session 16 onward

1. **`marketdata/infrastructure/{driven,driving}` and `module.go`** — the context's adapters and its
   composition seam, and the last piece of the slice. `driven/` implements `application.Budgets`
   (over `platform/config`) and `application.FundingPublisher`; `driving/` is what a dispatcher
   calls. **`hexboundary` is live**, so an import running inward-to-outward is a build failure now,
   not a review comment. Note `internal/engine/` still does not exist — the funding path can be
   completed without it, and `engine/venue`'s `DataSource` seam is §3.5's, not this context's.

2. **Two spec questions this slice raised, both deliberately unanswered in code.** Decide them at
   slice close, not before:
   - **`Age` vs `DataAge`.** §3.4 says "per-venue max-age budget", **singular**, and the code now
     carries **two** verdicts because a feed dies two ways and the venue-side stall is invisible to
     the check written first. The application layer confirmed both are needed (§17f). Does the spec
     owe a second budget, or is one budget applied to two measurements the right reading? The code
     currently does the latter.
   - **§3.2 vs §3.3 on `MarketView`.** §3.2's marketdata row says this context publishes one; §3.3
     says `MarketView` is strategy-owned and `engine/state` translates into it. Built to the
     consistent reading — this context publishes a `FundingSnapshot`, not a `MarketView`.

3. **The slice-close release, when `infrastructure/` lands.** Run the checklist in the paragraph
   above, with the **widened** grep. The two open markers are §3.6 status text (§17e); the reading
   pass should also ask whether §3.6 rule 10's deferral trigger being **wrong** (it named a state
   machine; enum-likes were what mattered) is worth stating, since the same mis-naming could defer
   `singlewriter` past its own trigger.

4. **Do NOT re-run the `exhaustive` evaluation.** It is done and recorded with numbers (§17c). The
   verdict was scope, not capability, and the flags were measured rather than recalled —
   `-package-scope-only` does **not** mean what its name suggests.
