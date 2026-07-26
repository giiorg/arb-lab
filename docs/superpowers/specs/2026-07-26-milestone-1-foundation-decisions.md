# Milestone 1 — foundation decisions (2026-07-26)

**Status:** in progress. First code in the repo. Branch `feat/milestone-1-foundation`, first commit `e92f9f0`.
**Doc set:** implements `build-spec.md` §12.1 (v0.9.4). Deviations from the spec are marked **[SPEC EDIT OWED]**.

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

**[SPEC EDIT OWED]** Record this as an ADR §3 entry — evaluated-and-rejected, with the reasons above,
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

### The `boundary/` grouping segment is dropped **[SPEC EDIT OWED]**

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

## 4. `kernelpurity` — a fifth structural rule **[SPEC EDIT OWED]**

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

### `exhaustiveswitch` — a sixth rule, deferred **[SPEC EDIT OWED]**

The compensating measure for the strongest Rust argument (§0). A `switch` over a §3.6-rule-8 enum-like
(`type X string` + constants + `Validate()`) must cover every declared constant, or fail the build.
Adding a state to a safety machine then breaks compilation at every decision site instead of silently
falling through to `default`. **Deferred** because no state machines exist yet. `golangci-lint`'s
`exhaustive` linter is the off-the-shelf alternative to evaluate first — if it suffices, use it and skip
the custom analyzer.

### Analyzer roster: 6 planned, 1 built

| Analyzer | State | Notes |
|---|---|---|
| `kernelpurity` | **Built, green, bite-checked** | 5th rule, not in §3.6 |
| `nofloat` | Pending, this slice | Type-aware; default-deny + `//nofloat:allow` |
| `notimenow` | Pending, this slice | No `time.Now()` in `domain/`/`application/` |
| `hexboundary` | Deferred | No contexts to police |
| `singlewriter` | Deferred | No OMS write repositories exist |
| `exhaustiveswitch` | Deferred | No state machines yet; try `exhaustive` first |

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

### PENDING USER CONFIRMATION — the rounding-mode set

Proposed and **not yet confirmed**:

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

The user was invited to drop `RoundExact`, add `HalfDown`, or rename — **no answer given yet.**

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
timeboxed attempt is **still owed** and its result must be reported honestly either way. (Note §0: this
is a place where Rust's `cargo-mutants` would have won outright.)

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

## RESUME HERE

**Environment, verified — do not re-derive.** Go 1.26.0. Remote `github.com/giiorg/arb-lab`. Branch
`feat/milestone-1-foundation` at `e92f9f0`, tree clean. `make ci` green (fmt-check → vet both modules →
custom gates → race tests both modules). `bitnet-backend-go` is **not on this machine** — a described
style, not a readable source.

**One outward-facing action is owed and was deliberately not taken:** `main` was fast-forwarded locally
to the v0.9.4 spec but **has not been pushed** (it is 57 commits ahead of `origin/main`), and
`feat/milestone-1-foundation` has never been pushed. Confirm with the user before pushing either.

**Next work, in order:**

1. **Confirm the rounding-mode set** (§5, pending) — then build `RoundingMode`.
2. **`fixedpoint` arithmetic, test-first per operation:** `Add`/`Sub` (exact, same-asset) →
   `Quantity` → `Price{base,quote}` + `Notional` → `Rate` + `Money.Mul(Rate)` → the `Calc` chain.
   128-bit intermediates via `math/bits`; overflow **errors, never wraps**.
3. **The oracle suite:** `rapid` properties against an independently written `math/big` oracle, one
   negative control each. Verify `rapid`'s `-rapid.seed` semantics from the module source before
   wiring a Makefile target — the CI/nightly seed policy was designed but the flag behaviour is
   **unverified**.
4. **SQL codec** (mantissa ↔ `int64` + asset column), parser **fuzz** target with committed seed
   corpus, **zero-alloc** assertions.
5. **`nofloat` + `notimenow` analyzers**, each `analysistest`-proven and bite-checked; register both in
   `tools/lint/cmd/arblab-lint`.
6. **`platform/clock`** + the two-implementation conformance suite (§3.5: `Now`, `ServerNow(Venue)` with
   uncertainty band, `SetTimer(id, at, prio)` with cancel/replace by id and ties broken by prio then id,
   `CancelTimer`).
7. **Land the four [SPEC EDIT OWED] items** — the `boundary/` flatten across `build-spec.md`
   §3.2/§11 (~30 occurrences, and **do not rewrite `boundary/lint` in §3.6**), `kernelpurity` as a
   fifth rule, `exhaustiveswitch` as a sixth, and the Go-vs-Rust ADR §3 entry.
8. **Still owed:** the timeboxed `gremlins` attempt, reported honestly either way.

**Do not over-build.** Milestone 1 exists to make **milestone 2 — the economic gate — measurable**, and
§12.2 is an explicit kill gate the spec predicts BTC/VIP0 may fail. The OMS is milestone 3.

**Standing rules:** build test-first; **no AI attribution** in commits or any authored content.
