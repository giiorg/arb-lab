# Record §100 — the web dashboard: a read side for a platform that has never had one

**Date:** 2026-08-29 (session 146, second subject) · **Task:** the operator's stated next topic, scoped from scratch · **Milestone:** 4 (pre-gate), new work item
**Predecessors:** §99 (F26, F27, F30 — the findings this UI is shaped around), §98 (the first live pair run, F23's latched journals), §42b (the screening harness whose artifacts are the UI's first data source), core §11 (names an *audit projection*), core §3.6 (the six gates this subsystem must satisfy)
**ADR rows:** F31 (the composition root is outside two of the six gates — §4a)
**Status:** design approved in session, **not yet externally reviewed**; no code written under this record.
**Nothing was sent to a venue. No normative document changed by this record.** A `build-spec.md` §12 roster entry is **owed** — see §14.

---

## 1. Why this record exists

`docs/HANDOFF.md` closed session 146 with a flag rather than a thread: *the
operator's stated next topic is a web UI, and nothing in the doc set covers it.*
§12's milestone roster has no UI item and no decision record discussed one. This
record is that scoping, taken as a fresh subject.

The requirement as stated: an elegant, comprehensive, responsive dashboard that
shows what markets are followed, which venues are available and their KPIs, what
the balances are, what the trends are visually and numerically, what positions
are open and what the PnL is — able to **monitor and control** funding-rate
arbitrage, and extensible to other strategies later.

The first thing exploration established is that the requirement, read literally,
describes a system that does not exist:

> **There is nothing running to monitor.** Every `arblab` subcommand is one-shot.
> `pair run` opens a package and exits; `reconcile run` boots, asserts and exits;
> `screen run` fetches and writes JSON. "Open positions", "balance" and "PnL" are
> not live state anywhere — they are SQLite journals under `run/pair/` after a
> command has finished, plus JSON artifacts under `research/`. There is no
> `serve`, no daemon, and no `net/http` server in the repo outside the venue REST
> client and test doubles.

So the dashboard needs a **subject** built alongside it. That is the expensive
part of this work, and it is why the classification is architectural rather than
bounded: there is no existing flow to modify.

The second thing exploration established is the opposite, and it is good news:
`internal/engine/venue/` is considerably further along than §12's milestone
prose implies. It holds `balances`, `positions`, `posture`, `marginstatus`,
`marginacts`, `marginwrite`, `order`, `orderquery`, `book`, `funding` and `mark`
— **each with both venues' codecs**. The credentialed half of this dashboard is
mostly wiring, not new engine code — which is what moves those panels from "needs
an engine" to "needs a session". §12 item 1's scope correction reads, in the
present tense, that account/order capability has *no code at all*; it is dated
and it names milestone 3 slice C as the destination, so it is not wrong — but see
§14.2.

## 2. What this subsystem is *for*, and the question it does not answer

Milestone 2 is a kill gate, and F26 says the verdict is not yet decidable — the
cost input spans the entire answer. That makes the naive framing ("a dashboard to
operate the strategy") premature. But the opposite framing ("a research artifact
viewer") gives up the thing the operator actually asked for.

The resolution taken here is that **the dashboard is an instrument for the open
question, not a readout of a settled one** — and that this is a stronger reason
to build it now, not a weaker one. §8 below shows why: a large fraction of the
handoff's stated next action ("measure the round trip") falls out of rendering
the cost honestly.

**The UI never becomes the actuator.** That is settled here so it is not
reopened. See §6.

## 3. Delivery in three phases

Each phase ends with something openable in a browser. Phase 1 is not a mockup:
it is a real dashboard over real data.

| Phase | Delivers | Data it can honestly show |
|---|---|---|
| **1** | Every screen, responsive, final visual design. `arblab serve`, HTTP + `go:embed`. API over everything needing **no credentials**. | Screening ledger, ranking, shortlist, census, persistence episodes, capture coverage, pair journals (read-only), configs, venue reference data, **live public market data** — funding both legs, predicted funding, next settlement, books, marks, basis. Most of the signal surface. |
| **2** | Credentialed panels; SSE push. | Balances, open positions, realized/unrealized PnL, margin posture, liquidation prices, live journal state. |
| **3** | Commands and supervision. | Safe idempotent commands; the arming store; kill switch; autonomous-engine supervision. |

Rejected: a single phase delivering everything. Rejected: an artifact browser
only (asked for and declined — it makes every live panel empty). Rejected: a full
shell with mocked live panels — a dashboard that partly reports nothing is the
failure mode this project's findings are made of.

## 4. Module layout

```
internal/audit/                  §11's audit context, built to §11's definition:
  domain/                        a PROJECTION OF THE OMS JOURNAL. Nothing else.
  application/
  infrastructure/driven/         read-only journal reader
<each context>/application/ports a read port for what that context already owns
internal/app/serve/              composition root: wiring, HTTP, JSON codec,
                                 pollers, embed. NOT gate-policed — see §4a
  application/                   cross-context view assembly. POLICED by
                                 hexboundary + notimenow, by segment name
  ui/dist/                       the embedded bundle (placeholder committed)
cmd/arblab/serve.go              subcommand, existing dispatch pattern
web/                             React app — own package.json, outside go.mod
```

**`audit` is not "the dashboard's read side", and the distinction is load-bearing.**
Core §11 defines it exactly: *"audit = projection of the OMS journal."* An earlier
draft of this record put the whole dashboard read model there — artifact reading,
venue snapshots, capture coverage — which would have silently widened a context's
meaning to fit a screen. **That is a spec edit wearing a layout decision's
clothes**, and this repo's standing rule is that deviations are marked, not
absorbed. It is not taken.

So the work splits three ways:

- **`internal/audit/`** is built as §11 defines it — the OMS journal projection,
  which does not exist yet and which the Positions and Journal screens need. It is
  owed regardless of this dashboard.
- **Each context exposes a read port for what it already owns**: `strategy` for
  screening and signal judgment, `marketdata` for book and funding snapshots,
  `risk` and `treasury` for their own. A context's read side belongs to that
  context.
- **`internal/app/serve/` composes across them.** A dashboard view is a
  cross-context composition, and composing across contexts is what a composition
  root is *for*. Per CLAUDE.md's layout rule, a read model for a screen is
  supporting machinery, not a first-class business capability — so it belongs
  under `app/`, not beside the contexts.

**`serve` composes; it never judges.** The entry classification
(`CANDIDATE` / `MARGINAL` / `BELOW COST`) is `strategy`'s, reusing what
`strategy/domain` already computes. `serve` assembles and formats.

> *Rejected alternative:* reimplement the classification in the read side for
> convenience. Rejected because two implementations of "is this a good entry" is
> exactly the drift this repo has been burned by, and because the screening
> ranking and the live signal board could then disagree about the same symbol with
> nothing failing.

> *Rejected alternative:* a new top-level `internal/dashboard/` context. Rejected
> on CLAUDE.md's layout claim: the top level is for business capabilities, and
> "what the operator sees" is not one. It would also have become the god-context
> that reaches into every other, which is the shape the three-way split above
> exists to avoid.

> *Rejected alternative:* widen `audit` to mean the whole read side, and mark the
> §11 edit as owed. Legitimate under the marking rule, and rejected because the
> narrower reading is *better* — the OMS journal projection is a real, separable
> thing that other consumers will want, and merging it with view assembly would
> couple it to a screen's shape. **The cheapest deviation is the one you find a
> reason not to need.**

### 4a. The split moves logic toward the gates, not away from them

The obvious objection to composing in a composition root is that **`app/` is
outside two of the six gates**. `hexboundary` and `notimenow` both scope by path
**segment** — `domain`, `application`, `infrastructure` — verified in
`tools/lint/hexboundary/hexboundary.go:150` and `notimenow.go:121`. A bare
`internal/app/serve/` package matches none of them, so neither gate would see it.
That matters here because §5 rule 4 — every response carries an injected
`server_now` — is precisely a `notimenow` claim, and it would have sat in the one
package the analyzer cannot read.

**So the view assembly goes in `internal/app/serve/application/`, and the wiring
stays in `internal/app/serve/`.** Because the gates key on segments, that single
naming decision puts every view under **both** analyzers: `hexboundary` then
forbids the assembly from importing `engine/` or any `infrastructure/`, forcing it
through the contexts' read ports, and `notimenow` forbids an ambient clock read
inside it. The composition root keeps only what a composition root is for — wiring
adapters, which is exactly the code that *should* touch `engine/`.

The name is not a trick to satisfy a linter. Cross-context view assembly **is**
application-layer logic; the previous draft's mistake was housing it somewhere
that did not say so, and the gates noticing is the point.

**The hole is real and has never been exploited — which is why it is invisible.**
`grep -rn "time\.Now()" internal/app/ --include="*.go"` excluding tests returns
**nothing**, across all five existing roots (`capture`, `pair`, `reconcile`,
`exercise`, `sim`). They do import `engine/`, which is correct and is what a
composition root is for. So `notimenow`'s discipline currently holds in `app/`
**by convention rather than by enforcement**, and a rule obeyed everywhere it is
unenforced is indistinguishable from a rule enforced. `serve` is the first app
root where an ambient clock read would be genuinely *tempting* — a long-running
process rendering countdowns — which is what turns a latent hole into a live one.
Recorded as **F31**.

The remaining cost is honest and stated: **more ports across more contexts** than
the single-context draft, and a `serve` tree that must be kept to one file per view
so it does not become the thing the earlier draft was rejected for.

**The journals are opened read-only.** `run/pair/*.db` is a single-writer store
and CLAUDE.md records that nothing lifts a latch inside a running process. A
`serve` process that opened them writable would be a second writer to the
platform's most safety-critical file.

### 4b. §3.6 gates, and how each one lands

| Gate | Effect on this subsystem |
|---|---|
| `nofloat` | Binds. One documented `//nofloat:allow` in the JSON encoder — see §5 rule 1. Precedent: `observability/export.go`'s metric wire seam. |
| `notimenow` | Binds on every `domain/` and `application/` package this work adds — including `app/serve/application/`, by §4a's naming. The clock is injected at the root and threaded through. This is what makes §5 rule 4 enforced rather than merely intended. |
| `kernelpurity` | Unaffected — nothing new enters `platform/`. |
| `hexboundary` | Binds on `audit/{domain,application}`, each context's new read port, and `app/serve/application/`. The venue snapshot cache is a driven adapter in `marketdata/infrastructure/`, reached through a port. |
| `exhaustiveswitch` | Binds on every new enum-like: attention severity, entry verdict, provenance class, venue connectivity state. |
| `singlewriter` | **Satisfied by construction.** The browser never reaches `oms/application/ports` or `oms/infrastructure/driven`; phase 3's control surface writes to an arming store, not to the OMS. No new rule is needed. |

## 5. The wire contract — four rules, each traceable to a finding

**Rule 1 — money and rates are decimal strings, never JSON numbers.** A JSON
number is a float64 on both sides of the wire. `nofloat` stops at `internal/`, so
without this rule every guarantee the dimensional types provide would end
silently at the network boundary, with the number still looking right. One
documented `//nofloat:allow` lives in the encoder and nowhere else.

**Rule 2 — the browser performs no money arithmetic.** Every derived quantity —
spread, net, break-even, annualised return — is computed in Go with `fixedpoint`
and shipped as a formatted string plus its raw decimal. The frontend formats; it
never computes. This is the rule that makes rule 1 worth having: a string parsed
into a JS number and multiplied by a position size is a float64 path through the
middle of the kill-gate calculation.

**Rule 3 — every ratio ships with its denominator.** A bare ratio is not
encodable by the response type:

```json
{ "value": "0.318", "basis": "deployed_capital_two_leg", "display": "+31.8 %/yr" }
```

This is **F30 as a type**. `ranking.json`'s `rank_score 0.71586586` is per *one
leg's* notional and declares that in no field, and the comparison core §2.1's
denominator rule exists to
protect was done at 2× by the two readers closest to it. A number that cannot
travel without its base cannot repeat that.

**Rule 4 — every response carries `server_now`.** Countdowns ("next settlement in
1h 12m") render against it, never against `Date.now()`. This is `notimenow`
projected across the network: a skewed laptop clock would shift every settlement
countdown on screen with nothing failing.

## 6. The control surface, and why the browser is never the actuator

| | The browser may | The browser may not |
|---|---|---|
| **v1** | read; nothing else | anything |
| **v2** | run safe idempotent commands: `screen run`, `reconcile run`, `reference fetch`, record a margin reconcile | place, amend or cancel orders |
| **v3** | **arm / disarm** a strategy per symbol; kill switch | compose an order — ever |

Order placement is the *engine's* job; the browser's job is to supervise it —
show what the engine is armed for, what it is about to do and why, what it did,
and carry an always-visible kill switch.

Arm/disarm is a write, but it is the right *shape* of write for this codebase:
it only ever changes a permission, and its safe direction is disarmed. That
matches the standing rule — when a default has two directions, pick the one that
is right when somebody forgets it. A "place order" button has no safe direction;
a disarm button's failure mode is that you trade less.

> *Rejected alternative:* full control including order placement from the browser.
> Rejected on F23: a *successful* `pair run` leaves a live position the root has no
> command to take off, latches `GateNetDeltaAsserted` on both journals, and no CLI
> lifts the latch. A button that can strand you needs more than a click, and the
> in-band close belongs with #241.

> *Rejected alternative:* permanently read-only. Rejected because it forces every
> action through a terminal even after an autonomous engine exists, and a kill
> switch that is not one tap away is not a kill switch.

## 7. Screens

Information architecture: a **flat seven-item sidebar**, plus a **status strip
pinned to every screen** (venues · equity · open packages · attention · engine
state). On mobile the sidebar becomes a bottom tab bar and the strip becomes a
2×3 tile grid.

> *Rejected alternative:* a `Run` / `Decide` mode split, mirroring the fact that
> milestone 2 is still a kill gate. Conceptually the most honest option, rejected
> because a mode switch is a wall paid on every navigation and the same honesty is
> delivered by the verdict banner (below) plus a top-level Evidence section.

> *Rejected alternative:* one dense console with drill-downs. Its best idea — the
> always-visible status strip — was taken; the rest loses a stable place for eight
> genuinely different subjects.

| Screen | Holds | Phase |
|---|---|---|
| **Overview** | verdict banner; attention; open positions; equity and PnL trend; signal board; persistence; concentration; capture health | 1 (equity/positions → 2) |
| **Markets** | all 466 screened / 61 eligible, sortable and filterable; live funding both legs, spread, next settlement, historical stats | 1 |
| **Market detail** | funding history both legs; episode distribution with break-even overlay; book depth at size; the cost decomposition; per-entry-level table | 1 |
| **Positions** | open packages, legs, state machine, gates; historical packages reconstructed from journals | journals 1, live 2 |
| **Venues** | reference data, fee tiers, filters, limits, leverage brackets, rate-limit budget, connectivity; posture and balances | 1 / 2 |
| **Evidence** | the kill gate — F26 cost sensitivity, F27 concentration, F28's overlap band, F29's open question, run-length vs break-even | 1 |
| **Journal** | OMS journal viewer, state machines, gates, latches, open incidents | 1 |
| **System** | capture coverage, config, build info, formal-model freshness | 1 |

Three decisions embedded in the Overview, recorded so they are not undone:

1. **The verdict banner sits above everything.** A signal board with no cost
   context is how F26 happens on a screen instead of in a document.
2. **Returns are labelled per deployed capital.** LABUSDT reads **+31.8 %/yr**,
   not the 71.6 % in `ranking.json`. See rule 3.
3. **Every signal row carries `p50 run` beside `need`.** A tempting spread never
   appears without the reason it usually is not enough (§99 §3: median 2, break-even ≈6, which is p93).

The **"needs a human"** panel is the highest-value item in phase 1 and exists
only because the journals are readable: F23 says a successful `pair run` latches
both journals and no CLI lifts them, so today the only way to know is to
remember.

## 8. The economic surface — what an operator can actually see

The requirement was *"every metric that matters for assessing a funding-rate
arbitrage opportunity."* This is that list, with provenance, honestly marked.

**Revenue**

| Quantity | Source | Phase |
|---|---|---|
| Funding rate, each leg, live | `venue/funding.go`, both venues | 1 |
| Predicted next funding (premium index) | venue REST | 1 |
| Funding interval and grid alignment (4h vs 8h) | `ranking.json` + live | 1 |
| Rate cap / floor per venue | reference data | 1 |
| Spread — the actual revenue | derived, server-side | 1 |
| Settlements accrued; countdown to next | derived | 1 |

**Entry and exit cost — where the verdict actually lives**

| Quantity | Source | Phase |
|---|---|---|
| Taker / maker fee, per venue, per tier | `configs/reference.*.json`, `venue-profiles.md` | 1 |
| The account's *actual* tier and discounts | signed `commissionRate` — ⚠ F24: the demo publishes the right number and charges another | 2 |
| Half-spread crossing at the configured size | live book, VWAP vs mid | 1 |
| Slippage at the configured size | live book depth | 1 |
| Round trip = 4 fee legs + 2 crossings | derived | 1 |
| Realized cost from actual fills | ⚠ one confounded observation (§98) | **open** |

**Carry, margin and risk**

| Quantity | Source | Phase |
|---|---|---|
| Leverage brackets, IM / MM tiers | `research/venue-probes/233-bracket-fixed-point` | 1 |
| Margin mode; portfolio-margin posture | `venue/posture.go`, `marginstatus.go` | 1 / 2 |
| Liquidation price per leg | derived from position and brackets | 2 |
| Free vs allocated collateral | `venue/balances.go` | 2 |
| Basis / mark divergence between legs | `venue/mark.go` | 1 |
| Tick, lot, min-notional filters | reference data | 1 |
| Position and order limits | reference data | 1 |

**Persistence and the decision**

| Quantity | Source | Phase |
|---|---|---|
| Settlements to break even | derived from spread and cost | 1 |
| p50 / p75 / p90 historical run length | `summary.json` | 1 |
| P(clearing break-even) at this run length | `summary.json`'s `run_vs_breakeven` | 1 |
| Expected net per episode | `episodes.json` | 1 |
| Annualised return, with denominator | derived | 1 |
| Hurdle comparison; concentration; sample size | `verdict`, F27, F29 | 1 |

### 8a. The cost assumption becomes a control

F26: the verdict is a function of an assumption whose span covers the whole
answer. So the assumed round-trip cost is **an input on screen** — 21 bp / 29 bp /
90 bp / custom — and every verdict, break-even and annualised figure on every
screen recomputes against it. Server-side, in `fixedpoint`, via a query
parameter; rule 2 makes this safe by construction.

This converts F26 from a caveat someone has to remember into an instrument.

### 8b. The cost stops being flat — and the spec already required this

`RoundTripCost()` is `0.0021` for every symbol at every size: four taker legs and
nothing else. But fees are per-venue-per-tier, and crossing and slippage are
computable from the **live book at the configured size**. The dashboard therefore
shows a per-symbol, per-size cost decomposition instead of one constant.

**This is not an improvement invented here.** Core §2.4 already requires it, in
terms that describe the panel almost exactly:

> *"**One source of truth for cost.** Round-trip cost MUST be computed from
> programmatically-fetched, per-leg, per-venue maker/taker fees plus a measured
> slippage model. **No hardcoded fee constants; no second copy of the number in
> another module.**"*

`screening/params.go` holds `BinanceTakerFee = 0.0005` and
`BybitTakerFee = 0.00055` as package-level constants. **That is not asserted here
as a breach** — the values are sourced and dated in the comment above them
(`venue-profiles.md` §280 block, observed 2026-07-31), the harness deliberately
"charges the worst published path", and §43g says step 1 is not gate evidence.
But it is a second copy of a number that has a dated revision-controlled home,
and the clause exists so a third copy is never made.

**So the dashboard does not read `screening.RoundTripCost()`.** See §8c.

The book-derived half needs **no credentials and no new venue contact beyond
public books**, and it is a large fraction of the handoff's stated next action —
arriving as a side effect of rendering the cost honestly rather than as a
separate measurement campaign.

**It is not the realized cost.** F24 and F25 leave that open, §98's 0.90 % stays
unattributed, and §8's table says so on the row.

### 8c. Where the cost and hurdle inputs come from — and where they must not

`configs/reference.*.json` already carries exactly the shape core §2.4 demands:
per-venue, per-product `maker`/`taker` rows with `tier`, `class`, `observed_at`
and `source`; `lots` rows with tick, step and min-notional; and a `hurdle` block
with `annual_rate`, three haircuts, `as_of`, `class` and `source`. `arblab
reference fetch` writes them. **These revisions are the dashboard's cost and
hurdle source of truth.** Three consequences:

1. **Fees are never fetched at render time.** Core §2.4: the decision path
   *"reads the fee (and lot-rule, and hurdle) revision **as-of the decision
   instant**, never by a scoring-time venue call."* The dashboard reads the dated
   revision. Books and marks are a different category — instantaneous market
   state, not a governed input — and are read live.
2. **The hurdle is displayed from the revision, with its provenance.** The
   committed revisions all carry `annual_rate 0.0369` with the T-bill source and
   an `as_of`; §99's verdict table compares against **4.6 %**, which core §2.1
   labels illustrative and which §99 §5 also labels illustrative. Both statements
   are correct in context, and a reader meeting either number alone cannot tell.
   Rendering `as_of`, `class` and `source` beside the figure is what makes the
   difference visible — the same disease as F30, treated the same way.
3. **A revision with no provenance is refused, not defaulted.** Core §2.4 says a
   revision missing writer class, observed-at or document revision is refused.
   The read side inherits that: an unpopulated cost reads as free and an
   unpopulated rate reads as zero fee, and both compute the gate in our favour.

> This is also the reason §5 rule 3 (every ratio carries its denominator) is not
> mere hygiene. Core §2.1 **requires** it: a gate may report on a denominator other
> than `C_allocated` *"on one condition: the denominator is explicitly declared in
> the artifact."* Rule 3 makes that condition structural instead of editorial.

## 9. Frontend stack

Node **24 (LTS Krypton)**, pinned by `.nvmrc`. Already installed locally as
v24.19.0; the shell default is v20.9.0, which is below both Vite 8 (`^20.19`) and
React Router 8 (`>=22.22`).

React 19.2 · TypeScript 7 · Vite 8 · React Router 8 · TanStack Query 5 ·
CSS Modules with custom properties · hand-written SVG charts · Vitest 4 ·
Playwright.

No state library: the URL is the state (routes plus query parameters), which also
makes every view linkable and every filter shareable.

> *Rejected alternative:* uPlot for charts. Proposed and then withdrawn in the
> same session. The justification was the 235 073-settlement funding history — but
> the server should downsample to the pixel budget regardless, because shipping
> 235k points to a browser is the actual mistake. Once responses carry ~1–2k
> points, hand-written SVG is fast and exactly controllable. **The constraint that
> justified the dependency was itself the bug.**

> *Rejected alternative:* Tailwind. Its value is taming an unconstrained design
> space; direction A is a deliberately constrained one — five colours, three type
> scales, hairlines, no cards. CSS Modules are native to Vite, need no plugin, and
> read better inside dense tables.

### Visual direction

**"Instrument, warmed."** Near-black ground lifted off pure black; monospace
numerics with tabular figures; hairline rules instead of cards and shadows;
generous row rhythm. **Colour is reserved: it only ever means state, never
decoration.** Both themes supported, dark default.

> *Rejected alternative:* the "console" register — elevated cards, rounded pills.
> Rejected on signal budget: a green pill reading "Candidate" and a red
> `−0.0421 %` compete for the same glance and only one of them is a measurement.

> *Rejected alternative:* an editorial, light, serif register. It suits the
> Evidence screen and was rejected as a global choice on density — 61 eligible
> symbols × both legs is a table you want to see at once, and styling that fits 12
> rows instead of 30 makes you filter before you have seen the data. **Density is a
> correctness property here, not a preference.**

## 10. Build integration

`web/` is a **third build unit**, documented exactly as `tools/lint` is: outside
`go.mod`, so no JS toolchain enters the engine's dependency graph and a local run
still resolves identically to CI.

- `make web` builds into `internal/app/serve/ui/dist/`.
- **A placeholder `index.html` is committed there**, so `go build` never requires
  node. It renders a page naming the make target — the zero value is unusable, and
  the failure is a refusal that names the fix rather than a blank screen.
- `make ci` gains `web-check`: `tsc --noEmit`, lint, `vitest run`.
- `make ci`'s Go half is unchanged and still runs without node.

> Without the committed placeholder, `//go:embed` fails at **compile** time on a
> missing directory, which would make `make ci` depend on node having run first.
> The two-module discipline exists precisely so that does not happen.

## 11. Testing

Go side follows the existing habits: golden JSON per endpoint generated from real
committed data by a script in the repo — **never hand-written**, because a
hand-written fixture lets a panel display a number no source can produce, and the
discovery would come at integration. `tools/mutate.sh` on the classifier and the
encoder. Positive controls on every harness.

**The fixture-collapse checklist runs before the mutations.** Five collapses are
already nameable and must be designed against:

| Collapse | Why it hides a defect |
|---|---|
| A **one-venue** fixture | "this venue's" and "the first venue's" are the same value |
| A **one-leg** package fixture | per-leg and per-deployed-capital are the **same number**, so rule 3 — the whole F30 defence — would pass untested |
| `p50 run` **equal to** break-even | the two thresholds the signal board contrasts collapse to one |
| Money strings with **trailing zeros** at scale 9 | two different encoders agree on `1.500000000` and `1.5` |
| An attention list of **one latch** | "the worst" and "the first" are the same row |

The one-leg case is the α = 0.5 trap in a new dress and deserves the emphasis: it
is the same shape as the partial-close reading that stayed unfalsifiable for four
sessions across two venues.

Frontend: Vitest over the formatters and status derivations; Playwright smoke
over three routes at two viewports.

## 12. What this record claims and does not claim

**Claimed.** The provenance column in §7 and §8 is an accurate reading of what
`internal/engine/venue/`, `research/` and `run/pair/` contain as of commit
`3893f73`. The gate analysis in §4 is a reading of `tools/lint/`'s six analyzers.
Package versions in §9 were resolved from the registry in session, and Node
v24.19.0 was confirmed installed. The `screening/params.go` fee constants and the
`configs/reference.*.json` row shapes in §8b–§8c were read directly, not recalled.

**Citations were resolved against the headings, not assumed.** Three drafted
here were wrong and are corrected: the *one source of truth for cost* clause is
**core §2.4** (not §2.1), and the *hurdle rate / declared denominator* clause is
**core §2.1** (not "§55" — `build-spec.md` has twelve sections, so every §N above
12 in this project's prose is a **decision record**, never a spec section). §11's
`audit` definition was read verbatim before §4 was written, and it is what changed
§4. A citation nobody re-derives is a premise, and this project has a finding
about exactly that.

**Not claimed.** Nothing here is evidence about the kill gate. §8b's book-derived
cost is a better estimate than a flat constant and is still **not** a realized
cost — F24 and F25 remain open, and §98's 0.90 % remains unattributed. No claim
is made that the credentialed phase-2 reads work against a live account: they
have codecs and tests, not a session behind this UI. **No claim is made that
`screening/params.go`'s fee constants breach core §2.4** — §8b states the reading
and its counter-argument and stops there; settling it needs a session that owns
§42b, not this one. **No claim is made about which hurdle figure is correct**
(3.69 % in every committed revision, 4.6 % in §99's table and core §2.1's prose,
both labelled illustrative where they appear); §8c's requirement is that whichever
is rendered arrives with its provenance attached.

## 13. Open questions, left open deliberately

- **Multi-strategy extension.** The requirement says "extensible to other
  strategies later." Nothing here designs that beyond keeping strategy judgment
  in `strategy/` rather than `audit/`. Committing to an abstraction over one
  strategy would be inventing a shape from a single instance.
- **Where the arming store lives.** Phase 3. It is a durable write and it is not
  the OMS journal; that is all this record settles.
- **Whether phase 2 needs a credential broker.** `serve` holding live venue
  credentials in a long-running process is a different security posture from a
  one-shot CLI. Deferred to phase 2's own record.

## 14. Debts this record opens

1. **`build-spec.md` §12 needs a roster entry** for this subsystem. §12 currently
   has no UI item; this record is a design, not a normative edit, and the roster
   entry is owed before phase 1 closes.
2. **§12 item 1's scope correction reads as current and is not.** It states that
   `engine/venue`'s account/order capabilities have *"no code at all"* (v0.9.28,
   §47e finding 1). They now do: `balances`, `positions`, `posture`,
   `marginstatus`, `marginacts`, `marginwrite`, `order`, `orderquery`, both
   venues. **The correction is not wrong** — it is dated 2026-08-02 and it names
   milestone 3 slice C (§47f) as where the capability lands, so a reader who
   follows the pointer and checks slice C's status arrives at the truth. The debt
   is smaller than a finding and worth one line: the sentence is written in the
   present tense with no closing back-reference, so a reader arriving at §12
   item 1 — the natural entry point for "what does this milestone contain" —
   gets a wrong picture from a true document. A one-clause *"(delivered; see
   slice C)"* closes it. **No claim is made here about slice C's own recorded
   completion status; that was not established this session.**
3. **`.gitignore` gained `.superpowers/`** during this session (the brainstorming
   companion's scratch directory). Unrelated to the subsystem; recorded so it is
   not mistaken for drift.
