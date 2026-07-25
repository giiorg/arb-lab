# Design — Interactive comprehensive guide (`docs/guide/index.html`)

**Status:** approved design, 2026-07-25. Next: implementation plan (writing-plans).
**Goal:** a single self-contained interactive HTML guide that teaches (a) the domain and its environment broadly and (b) exactly what this project builds, in detail — for the operator/engineer who will build it solo and sign its risk acceptances.
**Spec baseline:** v0.9.4. **Codebase baseline:** zero lines of Go.

## 1. Why

The specification set is 415 KB across five documents plus a TLA+ model, converged over eight external/adversarial review rounds. It is *complete* for milestones 1–2 and *not navigable* by a person who has to hold the whole thing in their head. `docs/tutorial.md` teaches the strategy and architecture well but deliberately excludes the safety kernel — by its own header, FM-1…FM-4 and the Cx6/7/8 findings are "below this tutorial's altitude." That excluded material is roughly 60% of the specification weight and ~90% of the review findings.

The gap is therefore not more specification and not more prose. It is a single artifact that makes the existing corpus comprehensible, with the safety kernel included at a teachable altitude, before implementation begins.

This guide is the orientation half of the agreed sequencing: **orientation, then build milestone 1.**

## 2. Hard constraints (load-bearing)

- **Teach-and-point, never restate.** `build-spec.md` and `state-machines.md` are the normative source of truth and are cross-referenced by the ADR §3 traceability table. The guide explains *why a control exists, what breaks without it, and how it composes*; it cites the section that states the requirement. It never becomes a second place where a requirement is written down. R2d is still pending — anything the guide duplicates is something that can silently drift.
- **Every factual claim carries a citation chip.** `core §7.4`, `SM §3`, `Profiles §4`, `ADR §3`, `tutorial §3.9`. A chip is a **rendered inline text label, not a hyperlink** — a published artifact cannot reach the repo's local markdown files, so a link would be dead on the hosted copy. The chip names the document and section precisely enough to open by hand. A claim with no chip is a defect. This is what makes drift *detectable* rather than silent, and it is the acceptance criterion that keeps "teach-and-point" honest rather than aspirational.
- **No invented numbers.** Every quantity traces to the docs. Figures that exist only to illustrate are explicitly labelled illustrative.
- **Honest about non-existence.** The platform does not exist. A persistent marker (`spec v0.9.4 · 0 lines of Go`) sits in the header, and no section may be phrased as though the system is running.
- **Self-contained, CSP-strict.** Published Artifacts block every external host: no CDN scripts, no external stylesheets, no web fonts, no remote images, no fetch/XHR. All CSS and JS inline; any asset a data URI. This is not a preference, it is what the platform enforces.
- **No framework, no build step.** Vanilla JS. The file is authored, committed, and published as-is.

## 3. Deliverable

| | |
|---|---|
| **Source of truth** | `docs/guide/index.html` — committed, version-controlled |
| **Rendered view** | published Artifact; redeploys target the same URL so the link is stable |
| **Ownership** | the guide owns *understanding*; the five-doc set keeps owning *requirements* |
| **Relationship to `tutorial.md`** | retained and cited, not absorbed (§9) |

## 4. Page architecture

**Layout.** CSS grid: fixed ~260 px left rail, scrolling content pane. Below ~860 px the rail collapses to a top drawer. Wide content (transition tables, code, diagrams) scrolls inside its own `overflow-x: auto` container; the page body never scrolls horizontally.

**Routing.** Hash-based — `#/edge/kill-math`. Any single idea is linkable. All six parts remain in the DOM and are hidden via CSS rather than unloaded; that is what allows search to reach unvisited content and lets calculator state survive navigation. Router: ~40 lines of vanilla JS, with a fallback to Part 1 on an unrecognized hash.

**Search.** An index is built at load by walking section headings and body text into `{partId, sectionId, heading, text}` records. Typing filters; selecting a result navigates and highlights. Native browser find only sees the visible section, so this search is the compensating mechanism, not a nicety.

**Theme.** `prefers-color-scheme` supplies the default; `:root[data-theme="dark"]` / `:root[data-theme="light"]` overrides ensure the viewer's own toggle wins in both directions.

**Diagrams.** Artifacts render Mermaid natively from `<pre class="mermaid">` blocks — no library involved. Used for the seven state machines and the three-failure-domain topology.

## 5. Interactive components

Three, in priority order. Each exists because the material is genuinely better poked than read.

### 5.1 Economics calculator (Part 2)

The centrepiece, because milestone 2 is a kill gate and its predicted outcome is failure.

- **Inputs:** funding rate (%/interval), funding interval (8 h / 1 h — live state per `core C8`), fee tier (dropdown driving spot + perp maker/taker), entry style (taker / maker), utilization %, allocated capital, hold length.
- **Outputs:** gross carry APY, round-trip cost, utilization-adjusted net, the `C_allocated` hurdle, and an explicit pass/fail verdict.
- **Presets** — the four documented milestone-2 forks (`core §12`): *BTC VIP0 today* (the predicted failure), *maker entry* (≈ −40% round-trip cost, `core §2.4`), *regime-selective*, *alt niche*.
- **Teaching intent:** yield is reported on full capital at risk, never on posted margin (`tutorial §1.5`). The calculator makes the leverage mirage impossible to fall for because the denominator is a visible input.

### 5.2 State-machine explorer (Part 4)

The largest content job and the highest-value one: 105 KB that currently opens only as a wall of tables.

- **Data:** one JS object per machine (seven), each carrying states, transitions (`from → event/guard → to`), invariants, and chaos cases — *summarized for teaching*, with a `normative: SM §n` chip on the machine and on each state group.
- **UI:** machine picker → Mermaid state diagram + state list; selecting a state filters the transition table to that state and surfaces its invariants.
- **The seven:** OMS order lifecycle · ambiguous-order classifier + finality contract · `FenceReadyLost` arbitration · actuator takeover · protective-order disarm · `EmergencyPlan` lifecycle · `PackageDelever` generations (`SM §1`–`§7`).

### 5.3 Liquidation sandbox (Part 1, lowest priority)

Notional, isolated margin, leverage, mark move → distance to liquidation, where the `add-margin` top-up protocol fires (`core §7.1`), and where `PackageDelever` takes over when the top-up fails. Turns "liquidation" and "isolated margin" from defined terms into a felt relationship.

**It lands in Part 1 but is built last (stage 4).** Part 1's prose ships complete in stage 1 and must read correctly without the sandbox; the sandbox is added into an already-finished section. This is deliberate: it is the one interactive piece the guide does not need, so it is the one that gets cut if effort has to be reclaimed.

## 6. Content map

Six parts. Every one sourced; nothing invented.

| Part | Content | Sources |
|---|---|---|
| **1 The world** | Spot / futures / perpetuals; the funding rate and its formula; damper vs cap; mark vs index vs last; funding as a discrete payment at a timestamp; liquidation and isolated margin; ADL and insurance funds; the two venues and their quirks; who is on the other side of this trade; custody and counterparty risk; who industrialises it (Ethena) and what that implies about edge decay | `tutorial §1`, `Profiles §1`, `core §4` |
| **2 The edge, honestly** | Funding *level* vs funding *spread* — two different trades; why cash-and-carry is the flagship; real round-trip costs; the hurdle; the kill math; the economic gate and its pre-registered calibration artifact; the four forks when the gate fails | `tutorial §2–3`, `core §2.4`, `core §8`, `core §12` |
| **3 Why the design looks like this** | ~10 near-miss cards: what we almost built, what it would have cost, what the rule is now. Includes the ambiguous-order compensation that could *create* a naked short, the un-fenced `$1K` position, and FM-1…FM-4 (composition, spurious-fire silence, branch-2 re-engagement silence, pre-branch arbitration silence) | `ADR §1`/`§3`, the `docs/review/` records, `docs/formal/` |
| **4 What we're building** | The three failure domains; the fence (signer isolation primary, exchange-side disable backup) and why two; the actuator and takeover; the safety kernel via the explorer; crash boundary and reconcile-only boot; the capital-tier gating that decides what must exist when | `core §3`, `core §7`, `core §11`, all of `SM` |
| **5 The Go build** | Four binaries and why they are separate processes; bounded contexts and the ports/adapters seams; `platform/fixedpoint` and why no `float64`; the parity `Clock` and total-order dispatcher; SQLite WAL + `synchronous=FULL`; the repo tree; which milestone builds which piece | `core §11`, `core §3`, `core §12` |
| **6 Operator runbook** | What you watch and what each alert means; the kill switch and pre-authorized ladder; RTOs per fault; the dead-man alert; what to do when the fence goes unhealthy; the risk acceptances you personally sign and the numbers you must choose before the canary is approved | `core §7.3`, `core §9`, `core §10`, `core §12` |

## 7. Writing rules

- **Failure first, control second.** State what goes wrong, then the mechanism that prevents it. This is the order the system was actually designed in and the only order in which seven state machines are justifiable rather than baffling.
- **Name the thing that is hard.** Where the spec made a genuinely difficult call (signer isolation over exchange-side disable; the one-way rule binding from *detection* rather than *selection*), say why the obvious alternative fails. The rejected alternatives are recorded in the review records and are the most instructive material in the corpus.
- **Plain language for the domain, precise language for the mechanism.** Part 1 assumes no trading background. Part 4 assumes Part 1.
- **No cheerleading.** The predicted milestone-2 outcome is failure; the guide says so where it is relevant rather than only in Part 2.

## 8. Build staging

Staged so the voice can be corrected before the bulk is written. Each stage republishes to the same URL.

| Stage | Contents | Gate |
|---|---|---|
| **1** | Shell: layout, routing, search, theme, nav. Part 1 + Part 2 with the economics calculator | User reviews voice and density before further writing |
| **2** | Part 4 + the state-machine explorer | Explorer usable for all seven machines |
| **3** | Part 3 near-miss cards + Part 5 Go build | — |
| **4** | Part 6 runbook + liquidation sandbox + polish: accessibility, mobile, print | Full pass |

## 9. `docs/tutorial.md` — retained, cited, not absorbed

The guide covers `tutorial.md`'s ground in Parts 1–2, which raises the same duplication hazard the teach-and-point rule exists to prevent. It is nonetheless **retained**, for a specific reason: it is in the reviewer packet, and spec↔tutorial consistency has produced real findings in every review round (`HANDOFF` step 0). Retiring it would remove a working review instrument.

Resolution: the guide **derives from it with explicit citations** (`tutorial §3.9`), applying teach-and-point discipline internally. Because the guide never states a Part 1–2 fact without pointing at its source, divergence between the two is mechanically findable rather than silent.

## 10. Verification

Each stage is opened and exercised, not merely written.

1. **Renders** — file opens, no console errors, all six parts reachable.
2. **Routing** — every nav entry resolves; deep links restore the correct section; an unknown hash falls back cleanly.
3. **Search** — a term appearing only in an unvisited part is found.
4. **Theme** — light and dark both legible; the viewer toggle overrides the OS preference in both directions.
5. **Responsive** — no horizontal body scroll at 375 px; wide tables scroll within their own container.
6. **Calculator correctness (the named test):** the default preset must reproduce the spec's documented figure — **≈3.4% net at 60% utilization against the ~4.6% hurdle** (`core §2.4`, `tutorial §3.9`). A disagreement means either the calculator or my reading of the spec is wrong, and which one must be established before the number is trusted.
7. **Citation audit** — spot-check that chips point at sections that exist and say what the guide claims they say.

## 11. Out of scope (YAGNI)

- Restating normative requirements (§2).
- Live venue data — the CSP forbids it, and a cached rate would rot into a lie.
- Persistence beyond in-page state; no accounts, no server, no storage.
- Editing the normative doc set. If writing the guide surfaces a genuine spec defect, that is a **finding** to be recorded and triaged like any other — never a silent fix, and never fixed in the guide alone.

## 12. Risks

| Risk | Mitigation |
|---|---|
| The guide drifts from the spec after R2d lands | Citation chips make every claim traceable; the guide states no requirement of its own |
| Seven state machines summarized into teaching altitude lose a load-bearing detail | Explorer content is a summary that always links to the normative machine; the diagram is never presented as the contract |
| Single file grows unmanageable | Staged build with a review gate after stage 1; sections are independent DOM subtrees with no cross-coupling |
| Effort displaces implementation | Explicitly time-boxed as the orientation half of "orientation, then build" — milestone 1 follows |
