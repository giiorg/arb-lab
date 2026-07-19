# Doc-Split Execution Plan (build-spec v0.7.1 → v0.8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the converged 782-line `docs/build-spec.md` (v0.7.1) into the five-doc v0.8 set — trimmed normative core + `venue-profiles.md` + `state-machines.md` (seven safety-kernel machines) + `adr-history.md` + `tutorial.md` — with zero normative change, per the approved design `docs/superpowers/specs/2026-07-19-doc-split-design.md`.

**Architecture:** Pure documentation restructuring on branch `docs/funding-arb-spec`. Content is extracted from the **frozen v0.7.1 blob** (`git show 6d211cd:docs/build-spec.md` — the working-tree file is currently identical to it) and placed into sibling docs in `docs/`; the core is trimmed in place, keeping its filename, git history, and §-numbers. Verification = mechanical set-equality checks (finding tags, table row keys, headings, cross-refs) + a final Codex content-equivalence pass.

**Tech Stack:** Markdown only. Bash (`git show`, `sed`, `grep`, `awk`, `comm`, `diff`) for extraction and verification. No build, no code.

## Global Constraints

Copied from the approved design (§2) — every task's requirements implicitly include these:

- **No normative change.** The split **moves and re-expresses**, it **does not re-decide**. Every MUST/SHOULD/MAY and every finding resolution survives **verbatim in meaning** (re-formatting prose into tables is allowed; changing meaning is not).
- **No new behavior in the state machines.** `state-machines.md` *structures* already-converged prose. If the prose is genuinely ambiguous about a state or transition, that is a **flagged finding** (see "Flagged ambiguities" rule below), **never a silently-invented resolution**.
- **Losslessness is an acceptance criterion.** Every normative sentence from v0.7.1 lands in **exactly one** doc (stated once, cross-referenced elsewhere); the finding→(doc, §) map is complete; zero dangling cross-references. Final gate: a Codex content-equivalence pass (Task 7).
- **Traceability preserved.** Every `[→ …]` finding tag survives: **requirements + their tags stay in the core**; mechanism detail moves out behind a `[mechanism: State-Machines §N]` / `(Profiles §N)` pointer. The Appendix table survives as finding→(doc, §) in `adr-history.md`.
- **Core-vs-mechanism split line.** The core states *what must hold, when, at which tier/gate* (invariant + requirement + tag). Profiles/State-Machines state *how* (venue facts; states, transitions, guards). **When in doubt, the requirement + tag stay in the core with a pointer; only mechanism detail moves.** The core must still read standalone as a complete requirements contract.
- **Frozen source of truth:** all extractions read `git show 6d211cd:docs/build-spec.md` (line numbers below refer to that blob). Edits apply to the working tree. Working tree `docs/build-spec.md` == that blob at plan time; after Task 1 the live file's line numbers shift — always re-locate by heading/bold-lead text, use blob line numbers for extraction only.
- **v0.8 header (exact wording, every doc in the set):** `**Status:** v0.8 (2026-07-19) — structural release. Content frozen from build-spec v0.7.1; reorganization only, no normative change.` New docs carry it from creation; `build-spec.md` and `tutorial.md` get it in Task 6.
- **Doc-set footer line for every new doc's header block (exact wording):** `**Doc set:** normative core `build-spec.md` · venue facts `venue-profiles.md` · kernel machines `state-machines.md` · history/traceability `adr-history.md` · teaching companion `tutorial.md`.`
- **Cross-ref style:** `(Profiles §N)`, `(State-Machines §N)`, `(ADR §N)` for cross-doc; bare `§N` stays an intra-doc reference. Core mechanism pointers use the form `[mechanism: State-Machines §N]`.
- **Commits:** one commit per task, message given in the task, `docs:` prefix. **NEVER add AI/model attribution trailers of any kind** (no Co-Authored-By, no session links) — user rule, overrides all defaults.
- **Flagged ambiguities rule:** if source prose under-determines a state/transition/guard, add it under that machine's `### Flagged ambiguities` heading as `**[FLAG-N]** <what is ambiguous> — <the ≥2 readings>` (global running number N). Do not resolve it in normative text. `None.` if empty.
- **Scratch dir** for all verification temp files: `SCRATCH=/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/0f5a8670-003d-4a4a-8ec7-513024ce4df1/scratchpad`
- **Never** edit `docs/legacy/` or `docs/review/` (frozen records; Profiles *copies* citations from a review record, the record itself is untouched).

### v0.7.1 blob map (frozen coordinates, `git show 6d211cd:docs/build-spec.md`)

| Lines | Content |
|---|---|
| 1–8 | Title (1), **Status/convergence narrative** (3), relation-to-docs (4), conventions (5), house style (6) |
| 10–92 | §1 scope + §1.1 tiers (31–44); §2 strategy (45–92) |
| 93–236 | §3 architecture (§3.0–3.7) |
| 237–374 | §4 venue contract: intro (237–240), **Go ports block (241–317)**, **quirk matrix (318–337, table 321–337)**, gap (338–340), **§4.1 (341–357, tables ~345–354)**, **§4.2 (358–374, table ~362–374)** |
| 375–427 | §5: §5.1 (377–387), §5.2 (388–395), **§5.3 (396–427)** — identities (398–400), state-machine intro (402), **state table (404–413)**, rules (415), **command verbs (417–424: OpenTrade/CloseTrade 418, ExposureReduction 419–422, PackageDelever 423, EmergencyClosePackage 424)** |
| 428–445 | §6 lifecycle steps 1–10 (one line each 430–435; **step 7 = "§6.7" spans 436–439**; step 8 = 440; step 9 = 441; step 10 = 442) |
| 446–570 | §7: §7.1 (448–460), §7.2 (461–467), §7.3 (468–473), **§7.4 (474–497)** — modes 475, policy/ladder 476, **actuator+EmergencyPlan 477**, **takeover 478**, **Fence 479–482**, **FenceReady/FenceReadyLost 483–487**, **fence-unavailable 488–491**, blast radius 492, tiering 493, subproject framing 494, **host-independent coverage 495**, **disarm 496**, out-of-band caveat 497; §7.5 (499–500), §7.6 (502–503), §7.7 sequence diagrams (505–570) |
| 571–643 | §8 (571–577), **§9 (578–588: chaos cases line 583, RTOs/gate line 584)**, §10 (589–595), §11 (596–632), §12 (633–643) |
| 644–782 | **Appendix traceability table** (heading 644, table header 646–647, data rows 648–782) |

---

### Task 1: Create `adr-history.md` — move the Appendix + convergence narrative (design §7 step 1)

**Files:**
- Create: `docs/adr-history.md`
- Modify: `docs/build-spec.md` (remove Appendix; replace the line-3 narrative with a short status ¶ + pointer)

**Interfaces:**
- Consumes: frozen blob lines 3 (narrative) and 644–782 (Appendix).
- Produces: `docs/adr-history.md` with stable headings `## 1. Convergence narrative`, `## 2. Review records`, `## 3. Finding → resolution traceability` (+ `## 4. v0.8 doc-split map` reserved, added in Task 6). Later tasks cross-ref these as `(ADR §N)`.

- [ ] **Step 1: Build the new doc.** Create `docs/adr-history.md`:

```markdown
# ADR & Review History — Funding-Rate Arbitrage Platform

**Status:** v0.8 (2026-07-19) — structural release. Content frozen from build-spec v0.7.1; reorganization only, no normative change.
**Doc set:** <doc-set footer line from Global Constraints>
**Role:** the convergence record — how the spec got here (review rounds, Codex convergence), plus the finding→resolution traceability that used to be the build-spec Appendix. Nothing here is normative on its own; the requirements live in the core.

## 1. Convergence narrative (v0.1 → v0.7.1)

<the FULL text of frozen line 3, verbatim, minus its leading "**Status:** " label — introduce it with one neutral sentence: "The v0.7.1 status paragraph, frozen verbatim:">

## 2. Review records

- `docs/review/2026-07-19-08-v0.7.1-review-and-verification-record.md` — round-3 (R1c/R2c) findings + resolutions + Codex convergence pass.
- `docs/review/2026-07-19-07-v0.7-review-and-verification-record.md` — round-2 (R1/R2 second round) findings + resolutions + venue/tooling verification citations (§2 there).
- `docs/review/2026-07-18-06-v0.6-review-and-verification-record.md` — v0.6 round.
- `docs/review/2026-07-18-05-tutorial-critical-review.md` — the earlier critical review the spec resolves.
- `docs/legacy/` — frozen research record (the "why"). Do not edit.

## 3. Finding → resolution traceability

<the FULL Appendix table, frozen lines 646–782, verbatim (heading text becomes this section; "Resolved in" refs still say §N of the core — Task 6 upgrades them to (doc, §))>
```

- [ ] **Step 2: Trim the core.** In `docs/build-spec.md`: (a) delete lines 644–782 (the whole `## Appendix …` section) and append in its place:

```markdown
## Appendix — traceability & history (moved)

The finding→resolution traceability table and the full convergence narrative moved to `adr-history.md` (ADR §3 and §1). Conventions for the `[→ …]` tags are in the header above.
```

(b) Replace line 3 (the giant `**Status:** Draft v0.7.1 …` paragraph) with exactly:

```markdown
**Status:** Draft v0.7.1 (2026-07-19) — three external review rounds (R1/R2, R1b/R2b, R1c/R2c) + five Codex convergence rounds applied; implementation-ready *by capital tier* (§1.1). Full convergence narrative + finding traceability: `adr-history.md` (ADR §1, §3). Doc-split to the v0.8 set in progress.
```

(Task 6 re-stamps this to v0.8; the interim wording keeps the tree honest mid-split.)

- [ ] **Step 3: Verify losslessness mechanically.**

```bash
SCRATCH=/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/0f5a8670-003d-4a4a-8ec7-513024ce4df1/scratchpad
# (a) Appendix finding keys: old vs new — expect NO output (sets equal)
git show 6d211cd:docs/build-spec.md | sed -n '648,782p' | awk -F'|' 'NF>2{print $2}' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t1_old_keys.txt"
sed -n '/^## 3\./,$p' docs/adr-history.md | awk -F'|' 'NF>2{print $2}' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t1_new_keys.txt"
diff "$SCRATCH/t1_old_keys.txt" "$SCRATCH/t1_new_keys.txt"
# (b) narrative tags survived the move — expect NO output (every [→ …] tag from old line 3 appears in adr-history.md)
git show 6d211cd:docs/build-spec.md | sed -n '3p' | grep -oE '\[→ [^]]+\]' | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t1_old_ntags.txt"
grep -oE '\[→ [^]]+\]' docs/adr-history.md | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t1_new_ntags.txt"
comm -23 "$SCRATCH/t1_old_ntags.txt" "$SCRATCH/t1_new_ntags.txt"
# (c) core no longer holds the table; stub present — expect: 0, then ≥1
grep -c '^| P1 spread-vs-level' docs/build-spec.md; grep -c 'moved to `adr-history.md`' docs/build-spec.md
```

Expected: (a) no diff output, (b) no comm output, (c) `0` then `1`.

- [ ] **Step 4: Commit.**

```bash
git add docs/adr-history.md docs/build-spec.md
git commit -m "docs: doc-split 1/6 — extract ADR/review-history to adr-history.md"
```

---

### Task 2: Create `venue-profiles.md` — move §4 venue facts + fold dated citations (design §7 step 2)

**Files:**
- Create: `docs/venue-profiles.md`
- Modify: `docs/build-spec.md` (§4/§4.1/§4.2: venue tables out, requirement stubs + tags stay)
- Read-only source: `docs/review/2026-07-19-07-v0.7-review-and-verification-record.md` §2 (lines 13–37) — copy citations from it; do not edit it.

**Interfaces:**
- Consumes: frozen blob 318–337 (quirk matrix), 341–357 (§4.1), 358–374 (§4.2); review-record §2 citation lines.
- Produces: `docs/venue-profiles.md` with headings `## 1. Per-venue quirk matrix (perp/futures)`, `## 2. Account & margin architecture (posture facts)`, `## 3. Spot-leg surface`, `## 4. Verification record (dated sources)`. Cross-ref target for `(Profiles §N)`.

**Split line applied to §4 (exact):** the **Go ports block (241–317) and §4 intro prose (237–240) stay in the core untouched** — they are the N-venue seam architecture. The **venue-fact tables move**: quirk matrix (321–337 + its bold lead 319), §4.1 venue tables (~345–354), §4.2 table (~362–374). §4.1's normative safety-premise prose (heading + any non-table requirement sentences, e.g. the assert-or-refuse rule) **stays in the core** with its tags; the per-venue *how* rows move. Every `[→ …]` tag that appears **only** inside a moved table must be re-attached to the core stub sentence for that concern (tags stay in core; copies inside the moved rows are fine).

- [ ] **Step 1: Build the new doc.** Header block (title `# Venue Capability Profiles — Binance USDⓈ-M / Bybit V5`, v0.8 status line, doc-set footer, role line: "per-venue facts + quirk matrices behind the core's venue-contract requirements (`build-spec.md` §4); every fact dated with its verification source (§4 here)."). Then move the three table groups **verbatim** (do not reword cells) under §1/§2/§3, each introduced by its original bold lead sentence. Add `## 4. Verification record (dated sources)`: copy the citation lines from review-record §2 (Binance block, Bybit block, SQLite block) verbatim, introduced as "Verified against official docs 2026-07-18 / 2026-07-19 (from the v0.7 review record §2, frozen there):". Facts already carrying inline dates keep them.
- [ ] **Step 2: Trim the core.** Under §4: replace the moved quirk matrix (319–337) with: `**Per-venue quirk matrix the adapter MUST encode** — normative venue facts, each dated: (Profiles §1). The adapter MUST encode every row; a row change is a spec change, not an adapter patch.` Under §4.1: keep heading + requirement prose + tags; replace moved tables with `Per-venue posture facts + assert-or-refuse read paths: (Profiles §2).` Under §4.2: keep heading + `[→ R1-F5]` + one requirement line (`The spot leg is a first-class venue surface — separate hosts/streams/filters; no mark/funding exists on spot. Facts: (Profiles §3).`); table moves.
- [ ] **Step 3: Verify.**

```bash
SCRATCH=/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/0f5a8670-003d-4a4a-8ec7-513024ce4df1/scratchpad
# (a) table row keys (first cell) old §4-region vs new doc — expect NO comm output (nothing lost)
git show 6d211cd:docs/build-spec.md | sed -n '318,374p' | awk -F'|' 'NF>2{print $2}' | sed 's/^ *//;s/ *$//' | grep -v -e '^-*$' -e '^Concern' -e '^Spot concern' | sort -u > "$SCRATCH/t2_old_rows.txt"
awk -F'|' 'NF>2{print $2}' docs/venue-profiles.md | sed 's/^ *//;s/ *$//' | grep -v -e '^-*$' -e '^Concern' -e '^Spot concern' | sort -u > "$SCRATCH/t2_new_rows.txt"
comm -23 "$SCRATCH/t2_old_rows.txt" "$SCRATCH/t2_new_rows.txt"
# (b) tags from the moved §4 region all still in core build-spec.md — expect NO comm output
git show 6d211cd:docs/build-spec.md | sed -n '237,374p' | grep -oE '\[→ [^]]+\]' | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t2_old_tags.txt"
grep -oE '\[→ [^]]+\]' docs/build-spec.md | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sort -u > "$SCRATCH/t2_core_tags.txt"
comm -23 "$SCRATCH/t2_old_tags.txt" "$SCRATCH/t2_core_tags.txt"
# (c) ports block untouched — expect NO diff output
git show 6d211cd:docs/build-spec.md | sed -n '241,317p' > "$SCRATCH/t2_old_ports.txt"; sed -n '/^## 4\. Venue contract/,/^### 4\.1/p' docs/build-spec.md | awk '/^```go$/{f=1} f{print} f&&/^```$/{exit}' > "$SCRATCH/t2_new_ports.txt"; diff "$SCRATCH/t2_old_ports.txt" "$SCRATCH/t2_new_ports.txt"
```

Expected: all three checks print nothing. If (b) prints tags, re-attach each to its core stub sentence.

- [ ] **Step 4: Commit.** `git add docs/venue-profiles.md docs/build-spec.md && git commit -m "docs: doc-split 2/6 — extract venue capability profiles to venue-profiles.md"`

---

### Task 3: `state-machines.md` — skeleton + execution-cluster machines 1, 2, 7 (design §7 step 3a)

**Files:**
- Create: `docs/state-machines.md`
- Modify: none (the core is trimmed in Task 5, not here — transient duplication between doc C and core §5.3/§6.7 is expected until Task 5).

**Interfaces:**
- Consumes: frozen blob §5.3 (396–427), §6 step 7 (436–439), §7.1 (448–460), §9 (583–584).
- Produces: `docs/state-machines.md` with `## 1. OMS order lifecycle`, `## 2. Ambiguous-order classifier & the finality contract`, `## 7. PackageDelever generations & reserved reductions` (§§3–6 added by Task 4 — leave placeholder headings `## 3.`–`## 6.` with `*(Task 4)*` so numbering is stable). Machine-section skeleton (identical for every machine, Tasks 3+4):

```markdown
## §N. <Machine name> `[→ <tags of the source prose>]`

**Source (v0.7.1):** build-spec <§refs> — re-expressed, no new behavior. **Normative owner:** the requirement + tag live in core <§ref>; this section is the mechanism.

### States
### Transitions
| From | Event / condition | To | Action / guard |
### Invariants
### Chaos & acceptance cases (§9 of the core)
### Flagged ambiguities
None.
```

**Machine content (extract + structure; invent nothing):**
1. **§1 OMS order lifecycle** — from §5.3: states `Planned, DispatchAuthorized, Submitted, SubmissionUnknown, Acknowledged, PartiallyFilled, Filled, Canceled, Rejected, CancelPending, CancelUnknown` (the 404–413 table, expanded so every "→ next on" cell becomes one transition row); the **orthogonal cumulative-execution fill ledger** as an explicit invariant block (fills admissible indefinitely for a known order; dedupe key; finality window governs only acceptance-resolution — line 413); rules line 415 (single `Place`, query-only recovery, per-venue stable-not-found incl. the Bybit realtime→history fallback, one-transaction fill application). Chaos cases: from line 583 — process kill mid-IOC, post-cancel late fills, Bybit realtime-cache-cleared → history fallback, identical spot/perp execution IDs.
2. **§2 Ambiguous-order classifier & finality contract** — from §6 step 7 (436–439): classifier states per hedge-group residual: `Determined / Ambiguous / DeadlineExceededAmbiguous`; which order-states make a counter-leg "unresolved" (line 438 list); the 4-step finality contract (tear-down/fence → terminal status + final cumulative qty → executions sweep past the watermark → position/balance reconcile to the same watermark); the determined branch (≤5 s `ExposureReduction` sized `realized_delta − outstanding_reserved_reductions`); the deadline-exceeded branch with the **interval-floor rule** and the reserved **`IntervalFloorReduction`** generation (never issued twice; own ambiguous result re-enters this contract; straddle-zero ⇒ de-minimis only); the **single-gate rule** (every compensating path routes through this classifier — the line-439 enumeration). Invariants: no compensation of the uncertain part before finality; no naked-short/doubled-short in any reachable venue state; §9 RTOs (determined <5 s, quiesce <30 s — line 584). Chaos: line 583's ambiguous-one-leg-failure clause with all its sub-cases.
3. **§7 PackageDelever generations** — from §5.3 line 423 + §7.1 (448–460 trigger/latency/reserve) + line 583's margin-top-up-under-stress clause: generation states (`Planned → PerpReduceInFlight → PerpReduceFinal → SpotReduceReserved → SpotReduceInFlight → Complete | Held/Alert | TerminalFlatten`— **derive names strictly from the prose's phases; if a phase boundary is unclear, FLAG it, don't invent**); perp-short-first sequencing invariant (transient never net-short); bounded generations only after prior §6.7 finality; the reserved second leg subtracted by the §2 classifier; ambiguous top-up result ⇒ re-query margin+position first; supersession by `EmergencyClosePackage`/`FenceReadyLost` (quiesce reserved leg first).

- [ ] **Step 1: Write the doc** (header block like Task 1/2, role: "the safety kernel + execution state machines, re-expressed from converged core prose — state lists, transition tables, invariants, chaos cases; the core owns the requirements") + doc-level preamble: "**No new behavior:** every row derives from core v0.7.1 prose; ambiguities are FLAGged, not resolved. Cross-refs: (core §N) = build-spec.md." Then machines §1, §2, §7 per above; placeholders §3–§6.
- [ ] **Step 2: Self-check per machine** — every state name in the section appears in (or is a FLAGged derivation from) the frozen source text; every source "→ / MUST / never" clause about that machine appears as a transition row or invariant; chaos block quotes/maps every §9 clause naming this machine.

```bash
# each expected count > 0; transition tables non-trivial
grep -c '^## §\?[127]\.' docs/state-machines.md   # expect 3 machine heads present (plus placeholders)
for s in 'DispatchAuthorized' 'SubmissionUnknown' 'IntervalFloorReduction' 'outstanding_reserved_reductions' 'never net-short'; do grep -c "$s" docs/state-machines.md; done
grep -c '^| ' docs/state-machines.md   # expect: dozens of transition rows, not <10
```

- [ ] **Step 3: Commit.** `git add docs/state-machines.md && git commit -m "docs: doc-split 3a/6 — state-machines.md: OMS lifecycle, ambiguous-order classifier, PackageDelever"`

---

### Task 4: `state-machines.md` — protection-cluster machines 3, 4, 5, 6 (design §7 step 3b)

**Files:**
- Modify: `docs/state-machines.md` (fill placeholder §§3–6)

**Interfaces:**
- Consumes: frozen blob §7.4 (474–497: esp. 477 plan/replication, 478 takeover, 479–482 Fence, 483–487 FenceReady/branches/RTOs, 488–491 fence-unavailable, 492–493 blast/tiering, 495 coverage, 496 disarm), §6 step 9 (441), §7.5 (500), the `EmergencyPlanStore` port in the §4 ports block, §9 (583–584). Uses the same section skeleton and FLAG rule as Task 3.
- Produces: `## 3. FenceReadyLost arbitration`, `## 4. Actuator takeover protocol`, `## 5. Protective-order disarm`, `## 6. EmergencyPlan lifecycle`.

**Machine content:**
3. **§3 FenceReadyLost arbitration** — from 483–491: the differentiated `FenceReady` state (signing / primary-revoke / backup-fence / durably-fenced dimensions; epoch-validated `VerifyFence`, never point-in-time); loss ⇒ first-safe-action (block `OpenTrade`, supersede active `PackageDelever` via §6.7-finality on its reserved leg) then the **three branches** (1 revoke-lost-signing-works → bot wind-down, overrides `hard-halt-and-alert`; 2 signing-unavailable + durable-latch → hand to takeover (§4 machine); 3 uncertain → pre-armed protection + manual only, never a trade, never an un-latched takeover); the tiered fence-unavailable floor (`$1K`+ read-only+manual vs canary de-minimis/interval-floor as signed bounded loss); RTO structure (first-safe-action <5 s; composite <30 s branches 1–2; branch 3 = manual target) from 487+584. Invariants: never two writers; never knowingly grow exposure while lost; guarantee wording from 487.
4. **§4 Actuator takeover protocol** — from 478 (+477 handles, 479–482 fence mechanisms, 488–491, 494): fence-first (durable, monotonic revoke **epoch** the signer honors across restart until operator clearance); quiesce **every** bot-owned exposure-changing order incl. all resting-protection surfaces (the separate cancel surfaces fact) to finality; reconcile via the §2 finality contract **same watermark**; reduce the **reconciled** residual (never the pre-allocated qty); bot-recovers-mid-takeover ⇒ finds itself fenced, reconcile-only until cleared; fence-unavailable ⇒ §3's tiered floor. Chaos: 583's actuator-takeover-fence clause (partition, late-fill-after-takeover, host-death sub-cases, no double-reduce, ListActive boot).
5. **§5 Protective-order disarm** — from 495–496: `Armed → DisarmPending → Disarmed`; entry-protection reserves the leg allocation (hot-path reduction amends the same fenced protective order, never duplicates); neutral-hedge confirmation ⇒ cancel then **§2 finality contract** (never cancel-ack alone) before `Disarmed`; partial protective fill stays `DisarmPending`, reclassified per fill, package-compensated only after finality collapses remaining qty; acceptance = measurable detect→re-neutralize bound, not "never naked". Chaos: disarm race clause; host-death with armed stop / `DisarmPending`.
6. **§6 EmergencyPlan lifecycle** — from 477 + 441 + 500 + the port: plan states across replicate/act/deactivate: pushed-before-exposure (ack required before entry submits), monotonic generation upsert/CAS as created, actuator boot `ListActive()` (no TradeID list post-host-death) applies latest generation, `Deactivate` = idempotent **terminal tombstone** rejecting same-or-later-generation replication; close-side `DeactivationPending` (durable before/atomic-with close finality; lease retained until **persisted ack**; reconcile-only boot re-drives idempotently, bounded backoff, alert; lease never released with a plan active). Chaos: `Replicate`↔`Deactivate` reorder/retry, crash between close and deactivation, actuator restart ⇒ no stale plan re-actioned.

- [ ] **Step 1: Fill §§3–6** per skeleton; remove the `*(Task 4)*` placeholders. Where §3 and §4 touch the same sentences (fence epoch, fence-unavailable), **state it once in the owning machine (§4 owns takeover/epoch mechanics; §3 owns arbitration + tier floor) and cross-ref the other** — no duplicated normative text inside doc C.
- [ ] **Step 2: Self-check** as Task 3 Step 2, plus:

```bash
for s in 'revoke epoch' 'ListActive' 'DisarmPending' 'DeactivationPending' 'tombstone' 'never two writers' 'reconciled residual' 'first-safe-action' 'T_fence' 'Sub Member Transfer' 'never net-short' 'IntervalFloorReduction'; do printf '%s: ' "$s"; grep -ci "$s" docs/state-machines.md; done   # every count ≥ 1
grep -n '\*(Task 4)\*' docs/state-machines.md   # expect: nothing
```

- [ ] **Step 3: Commit.** `git add docs/state-machines.md && git commit -m "docs: doc-split 3b/6 — state-machines.md: FenceReadyLost, takeover, disarm, EmergencyPlan"`

---

### Task 5: Trim `build-spec.md` to the normative core (design §7 step 4)

**Files:**
- Modify: `docs/build-spec.md` only.

**Interfaces:**
- Consumes: `docs/state-machines.md` §§1–7 as they now exist (pointer targets), Global-Constraints split line.
- Produces: trimmed core; **all `##`/`###` §-headings keep their exact current text and order** (Appendix stub aside). Pointer form `[mechanism: State-Machines §N]`.

**Trim map (each: keep requirement+tags+numbers, move mechanism):**
- **§5.3:** keep identities (398–400), the state-machine *requirement* (one ¶: the state set + crash boundary + query-only recovery + fill-ledger orthogonality + single-`Place` + stable-not-found/Bybit-fallback + one-transaction rule, each with its tags) — **delete the 404–413 table** → `[mechanism: State-Machines §1]`. Keep all four command-verb definitional requirements (verb, origin, authority, `stop-opening` exemption, when used, tags); trim `ExposureReduction`'s state-dependent `reduceOnly` case detail (420–422) → one requirement sentence + `[mechanism: State-Machines §2]`; trim `PackageDelever` line-423 mechanics to trigger + both-legs + sequencing-invariant sentence + `[mechanism: State-Machines §7]`; trim `EmergencyClosePackage` line-424 quiesce mechanics to definition + rationale (+ its R2c-7 note stays) + `[mechanism: State-Machines §2, §7]`.
- **§6 step 7:** collapse 436–439 to a requirement ¶: classify determined-vs-ambiguous by remaining-executable-quantity before ANY compensating action; determined ⇒ ≤5 s reduction (reservation-aware sizing); ambiguous ⇒ quiesce-to-finality first via the per-venue finality contract; deadline-exceeded ⇒ hold + alert + only provably-safe action (interval floor where the summed interval doesn't straddle zero, else de-minimis; reserved `IntervalFloorReduction` generation); **the classifier is the single gate for every compensating path** (keep the enumeration sentence); all tags (`Z2, R2b-B1, R1c-4` etc.) stay + `[mechanism: State-Machines §2]`.
- **§7.4:** keep bullets 475 (modes), 476 (policy/approval/ladder), 497 (out-of-band caveat) intact. Keep 477's *requirements* (three failure domains; host-independent coverage mandatory; plan replicated before exposure with ack-before-entry; coarse-balance fallback forbidden; watcher→actuator auth) → plan-content list + boot/tombstone mechanics `[mechanism: State-Machines §6]`. Collapse 478 to: takeover MUST be fence-first, quiesce-all-surfaces-to-finality, reduce only the reconciled residual, single-writer preserved by transfer `[mechanism: State-Machines §4]`. Keep 479–482's requirements: two mechanisms, ≥1 available+propagation-tested per venue before `$1K`, signer-isolation primary (and why), exchange-side backup with the verified per-venue facts' *requirement* (Bybit narrowest-transfer-grant; Binance sub-account-conditional + verify-item) `(Profiles §1/§4)`, ack semantics + propagation test required; trim the deep how (480–481 internals) → `[mechanism: State-Machines §4]` — **note (Task 2 boundary): 479–482 stays requirement-level in core; the dated endpoint facts are already in Profiles**. Collapse 483–491 to: `FenceReady` is a continuously-monitored differentiated state feeding a `FenceReadyLost` arbitration that first blocks openings then branches by failed dimension (wind-down / durable-latch takeover / manual-only), with branch RTO structure + the tiered fence-unavailable floor + the honest-guarantee sentence (487's "exposure is reduced toward flat…") + tags `[mechanism: State-Machines §3]`. Keep 492 (blast radius policy), 493 (tiering), 494 (subproject framing), 495 (coverage requirements + attach facts pointer `(Profiles §1/§3)`) — trim 496 disarm mechanics to the requirement (armed leg-level stop post-neutral is a naked-leg generator ⇒ fenced disarm through the finality contract; measurable re-neutralize bound) `[mechanism: State-Machines §5]`.
- **§6 step 9 / §7.5:** keep; trim step-9 `DeactivationPending` mechanics to the invariant (close never releases the lease before the persisted `Deactivate` ack; re-driven on boot) `[mechanism: State-Machines §6]`.
- **Do not touch:** §§1–3, §4 (already split in Task 2 — leave its stubs as Task 2 wrote them), §5.1–5.2, §6 steps 1–6/8/10, §7.1–7.3, §7.6, §7.7 sequence diagrams, §§8–12. (§7.7's diagrams complement doc C; fine as-is per design.)

- [ ] **Step 1: Apply the trim map.**
- [ ] **Step 2: Verify.**

```bash
SCRATCH=/private/tmp/claude-501/-Users-sofo-Projects-misc-arb-lab/0f5a8670-003d-4a4a-8ec7-513024ce4df1/scratchpad
# (a) headings unchanged (minus Appendix rename) — expect: only the Appendix line differs
git show 6d211cd:docs/build-spec.md | grep -E '^#{2,3} ' > "$SCRATCH/t5_old_h.txt"; grep -E '^#{2,3} ' docs/build-spec.md > "$SCRATCH/t5_new_h.txt"; diff "$SCRATCH/t5_old_h.txt" "$SCRATCH/t5_new_h.txt"
# (b) tag union across the doc set ⊇ v0.7.1 full set — expect NO output
for f in docs/build-spec.md docs/venue-profiles.md docs/state-machines.md docs/adr-history.md docs/tutorial.md; do grep -oE '\[→ [^]]+\]' "$f"; done | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | grep -v '^$' | sort -u > "$SCRATCH/t5_new_tags.txt"
git show 6d211cd:docs/build-spec.md | grep -oE '\[→ [^]]+\]' | tr -d '[]' | sed 's/^→ *//' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | grep -v '^$' | sort -u > "$SCRATCH/t5_old_tags.txt"
comm -23 "$SCRATCH/t5_old_tags.txt" "$SCRATCH/t5_new_tags.txt"
# (c) every [mechanism: …] pointer resolves — expect NO output from the loop
grep -oE '\[mechanism: [^]]+\]' docs/build-spec.md | sort -u
grep -oE '\[mechanism: State-Machines §[0-9]+' docs/build-spec.md | grep -oE '§[0-9]+' | sort -u | while read s; do grep -q "^## ${s}\." docs/state-machines.md || echo "DANGLING $s"; done
# (d) the moved state table gone from core — expect 0
grep -c '| `Planned` | intent + client ID journaled' docs/build-spec.md
```

- [ ] **Step 3: Commit.** `git add docs/build-spec.md && git commit -m "docs: doc-split 4/6 — trim build-spec.md to the normative core"`

---

### Task 6: Cross-refs, HANDOFF, tutorial, what-moved-where map; stamp v0.8 (design §7 step 5)

**Files:**
- Modify: `docs/build-spec.md` (header → v0.8 + doc-set map), `docs/tutorial.md` (header + refs), `docs/adr-history.md` (add `## 4. v0.8 doc-split map`; upgrade §3 refs to (doc, §)), `docs/HANDOFF.md` (full state update), `docs/state-machines.md` + `docs/venue-profiles.md` (only if a cross-ref fix is needed).

**Interfaces:** consumes the final §-structure of all five docs.

- [ ] **Step 1: Stamp the core.** Replace the interim Status ¶ (Task 1 Step 2b) with the exact v0.8 status line from Global Constraints, followed by: `This is the **normative core** of a five-doc set (v0.8): venue facts → `venue-profiles.md`; execution + safety-kernel state machines → `state-machines.md`; convergence history + finding traceability → `adr-history.md`; teaching companion → `tutorial.md`. Requirements and their `[→ …]` tags live HERE; mechanism detail lives behind `[mechanism: …]` pointers. What-moved-where: ADR §4.` Keep lines 4–6 (relation/conventions/house-style) as-is, updating line 4's doc enumeration to the five-doc set.
- [ ] **Step 2: Tutorial.** Add under its title: the v0.8 status line + one sentence: "The spec is now a five-doc set — mechanism references below may point at `state-machines.md` (SM) and `venue-profiles.md` (Profiles)." Then walk every `spec §…` reference (`grep -n '§' docs/tutorial.md`): refs to requirements keep pointing at core §N (they still resolve — headings unchanged); refs whose *sentence is about mechanism that moved* (the §5.3 state table, §6.7 branch mechanics, §7.4 fence/takeover/disarm internals, §4 venue tables) gain the new home, e.g. `spec §6.7 (SM §2)`. Do not reword tutorial prose otherwise.
- [ ] **Step 3: ADR map + ref upgrade.** Append `## 4. v0.8 doc-split map (what moved where)` — a table: v0.7.1 location → v0.8 home, one row per moved block (narrative→ADR §1; Appendix→ADR §3; §4 quirk matrix→Profiles §1; §4.1 tables→Profiles §2; §4.2 table→Profiles §3; record-§2 citations→copied to Profiles §4; §5.3 state table→SM §1; §5.3 reduceOnly cases→SM §2; §5.3 PackageDelever mechanics→SM §7; §6.7 mechanics→SM §2; §7.4 takeover→SM §4; §7.4 Fence/FenceReady branches→SM §3; §7.4 disarm→SM §5; EmergencyPlan lifecycle→SM §6) + FLAG list (or "no flags"). Then upgrade §3's "Resolved in" refs: where a finding's resolving content moved, qualify it (`SM §2`, `Profiles §1`, …); where it stayed, prefix `core` (e.g. `core §6.7`). Every row gets a doc-qualified ref (design: table survives as finding→(doc, §)).
- [ ] **Step 4: HANDOFF.** Update `docs/HANDOFF.md`: status → **v0.8 (structural release; content = v0.7.1)**; remove/close the ⏭ ACTIVE TASK block (record: doc-split executed, date, commit range); Artifacts list → the five-doc set (read order: core → state-machines → venue-profiles → tutorial → adr-history → review records) with one-line roles; "Doc split" section → done (pointer to ADR §4); Next steps → 0. new external review round on the v0.8 set, 1. milestone-1 implementation (unchanged text), keep the fresh-tags rule (next → R1d/R2d) and Open-decisions block as-is.
- [ ] **Step 5: Verify cross-refs mechanically.**

```bash
# every (Profiles §N) / (State-Machines §N) / (SM §N) / (ADR §N) target exists — expect NO DANGLING lines
for f in docs/build-spec.md docs/tutorial.md docs/state-machines.md docs/venue-profiles.md docs/adr-history.md docs/HANDOFF.md; do
  grep -oE '\((Profiles|State-Machines|SM|ADR) §[0-9]+' "$f" | sort -u | while read -r ref; do
    doc=$(echo "$ref" | tr -d '(' | awk '{print $1}'); sec=$(echo "$ref" | grep -oE '[0-9]+$')
    case "$doc" in Profiles) t=docs/venue-profiles.md;; State-Machines|SM) t=docs/state-machines.md;; ADR) t=docs/adr-history.md;; esac
    grep -qE "^## (§)?${sec}\." "$t" || echo "DANGLING in $f: $ref"
  done
done
grep -c 'v0.8 (2026-07-19) — structural release' docs/build-spec.md docs/tutorial.md docs/state-machines.md docs/venue-profiles.md docs/adr-history.md   # expect 1 per file
grep -n 'ACTIVE TASK' docs/HANDOFF.md   # expect: nothing (or only the closed record line)
```

- [ ] **Step 6: Commit.** `git add docs/ && git commit -m "docs: doc-split 5/6 — cross-refs, HANDOFF, tutorial; stamp v0.8"`

---

### Task 7: Losslessness / content-equivalence verification (design §7 step 6) — controller-led

**Files:** read-only over the whole set; fixes (if any) via a fix dispatch touching whichever doc failed.

- [ ] **Step 1 (controller): re-run every mechanical gate** from Tasks 1–6 (they must all still pass on the final tree) plus a duplication sweep: no normative sentence stated in *two* docs (spot-check the Task 5 trim map lines: each moved block's distinctive phrases appear in exactly one doc, e.g. `grep -c 'tear down / fence that order' docs/*.md` ⇒ exactly one file with count ≥1).
- [ ] **Step 2 (controller): Codex content-equivalence pass** (invoke via the codex skill): give Codex (a) `git show 6d211cd:docs/build-spec.md` (the frozen v0.7.1 monolith), (b) the five v0.8 docs, (c) the ADR §4 moved-map, and ask for an adversarial equivalence audit: 1. any normative statement (MUST/SHOULD/MAY, invariant, number, RTO, tag) lost, weakened, strengthened, or duplicated-with-drift; 2. any state/transition/guard in `state-machines.md` **not derivable** from v0.7.1 prose (invented behavior); 3. any v0.7.1 prose machine behavior **missing** from the tables; 4. dangling or wrong cross-refs; 5. finding→(doc, §) completeness against the old Appendix. Findings → fix dispatch → re-run Codex until clean (cap: 3 rounds; if still dirty, STOP and surface to the user).
- [ ] **Step 3: Final commit (if fixes landed).** `git commit -m "docs: doc-split 6/6 — losslessness fixes from equivalence verification"` (skip if tree already clean).

---

## Execution notes (controller)

- **Branch:** `docs/funding-arb-spec` (already checked out; never main). Push after the final review passes.
- **Model per dispatch:** T1 sonnet · T2 sonnet · T3 fable · T4 fable · T5 fable · T6 sonnet · T7 fixes sonnet (mechanical) / opus (semantic). Task reviewers: T1/T2/T6 opus · T3/T4/T5 fable (no-behavior-change validation against dense safety prose is the hardest judgment in the project). Final whole-branch review: fable.
- **Reviewer lens per task:** spec compliance = this plan's task + Global Constraints (losslessness, split line, tags-stay-in-core, no-new-behavior, FLAG rule); quality = readability of the produced doc as a standalone artifact.
- **Ledger:** `.superpowers/sdd/progress.md` (untracked scratch; delete before the final push).
- **After Task 7 passes (controller, outside the repo):** update project memory (`~/.claude/projects/-Users-sofo-Projects-misc-arb-lab/memory/` — the arb-lab platform memory + MEMORY.md hook) to v0.8 + the five-doc set — design §6 acceptance criterion 3 includes memory.
- Design acceptance (§6) mapped: criterion 1 → T3+T4, 2 → T7, 3 → T6, 4 → T6 Step 3.
