# Design — Funding-arb build-spec doc-split (v0.7.1 → v0.8)

**Status:** approved design, 2026-07-19. Next: implementation plan (writing-plans).
**Goal:** split the converged 782-line `docs/build-spec.md` (v0.7.1) into a focused document set, and re-express the safety kernel as explicit state-machine specs — **reorganization only, no normative change**.

## 1. Why

Both round-3 external reviewers escalated the doc-split from "optional housekeeping" to **recommended before implementation**. R2c specifically: *"the safety kernel should move into dedicated state-machine specifications before implementation; the present large prose paragraphs are too difficult to validate mechanically."* The spec now carries three review generations + five Codex convergence rounds in one dense file; splitting it makes each concern implementable and mechanically checkable. The reviewers' own conclusion was "then start milestone 1" — this split is the clean handoff to that.

## 2. Hard constraints (load-bearing)

- **No normative change.** The content converged over three external review rounds (R1/R2, R1b/R2b, R1c/R2c) and five Codex rounds (Cx2–Cx4). The split **moves and re-expresses**, it **does not re-decide**. Every MUST/SHOULD/MAY and every finding resolution survives verbatim in meaning.
- **No new behavior in the state machines.** Doc C *structures* the already-converged prose (state list + transition table + invariants). If the prose is genuinely ambiguous about a transition, that is a **flagged finding**, never a silently-invented one.
- **Losslessness is an acceptance criterion.** Every normative sentence from v0.7.1 lands in exactly one doc; the finding→section map is complete; no dangling cross-references. Verified by a Codex content-equivalence pass before the split is called done.
- **Traceability preserved.** The Appendix finding→section table survives as finding→(doc, §).
- **Core-vs-mechanism boundary (the split line).** The normative core states *what must hold, when, and at which tier/gate* — the invariant + requirement + finding tag. The Profiles/State-Machines docs state *how* — venue-specific facts; states, transitions, guards. **When in doubt, the requirement + its tag stay in the core with a `[mechanism: …]` pointer, and only the mechanism detail moves out.** This keeps the core a complete requirements contract that reads standalone.

## 3. Target document set (v0.8)

Five documents; the whole set is versioned **v0.8 — a structural release**, each header stating *"content frozen from build-spec v0.7.1; reorganization only, no normative change."*

| Doc | File | Holds | Source |
|---|---|---|---|
| **A. Normative core** | `docs/build-spec.md` (trimmed in place — keeps name, git history, inbound refs) | Scope + capital tiers (§1), strategy model (§2), architecture *decisions* (§3.0–3.7), data model (§5), lifecycle + risk/custody at the **invariant/MUST** level (§6–§7 requirements), backtester/testing/obs/stack/milestones (§8–§12). Points to B/C for mechanism detail. | Extracted verbatim |
| **B. Venue capability profiles** | `docs/venue-profiles.md` (new sibling) | Per-venue quirk matrices (§4 tables, §4.1 account/margin, §4.2 spot surface), **each fact dated with its source URL** (folding in the review-record §2 citations). | §4 tables + verification-record §2 |
| **C. Execution + safety-kernel state-machine specs** | `docs/state-machines.md` (new sibling) | The seven machines below — each as *state list + transition table + invariants + its §9 chaos cases*. | Re-expressed from §5.3/§6.7/§7.4 |
| **D. ADR / review-history** | `docs/adr-history.md` (new sibling) | The Appendix finding→(doc, §) traceability + the convergence narrative (R/Codex rounds) + pointers to the review records. | Appendix + records |
| **E. Tutorial** | `docs/tutorial.md` (unchanged) | Teaching companion. | Unchanged (ref updates only) |

### 3.1 The seven state machines (doc C)

All re-expressed from converged prose, **no new behavior**. Each gets: enumerated states, a transition table (state × event → next state + action/guard), the invariants it preserves, and the §9 chaos cases that exercise it.

1. **OMS order lifecycle** — `Planned`→`DispatchAuthorized`→`Submitted`→`SubmissionUnknown`/`Acknowledged`→`PartiallyFilled`/`Filled`/`Canceled`/`Rejected`/`CancelPending`/`CancelUnknown`, plus the orthogonal cumulative-execution fill ledger (admits deduped fills for a known order indefinitely). (from §5.3)
2. **Ambiguous-order classifier + per-venue finality contract** — determined vs ambiguous by remaining executable quantity; quiesce-to-finality (tear-down/fence → terminal status → executions sweep to watermark → reconcile); the deadline-exceeded branch with the reserved `IntervalFloorReduction` generation. (from §6.7)
3. **`FenceReadyLost` arbitration** — the differentiated fence state (signing / primary-revoke / backup-fence / durably-fenced) → three branches (bot wind-down · actuator takeover under a durable latched revoke epoch · uncertain → read-only + manual), with the tiered fence-unavailable floor ($1K+ read-only vs canary de-minimis). (from §7.4)
4. **Takeover protocol** — fence-first with a durable, monotonic revoke epoch the signer honors across restart; quiesce every resting-order surface to finality; reduce the *reconciled* residual (not the pre-allocated qty); bot-recovers-mid-takeover stays reconcile-only. (from §7.4)
5. **Protective-order disarm** — `Armed`→`DisarmPending`→`Disarmed`; a partial protective fill stays `DisarmPending`, reclassified per fill, compensated only after the §6.7 finality contract. (from §7.4)
6. **EmergencyPlan deactivation lifecycle** — `Replicate` (monotonic gen upsert/CAS) / `ListActive` / `Load` / idempotent terminal `Deactivate` tombstone; the durable `DeactivationPending` close sub-state + reconcile-only-boot re-drive. (from §4 port + §6 step 9 + §7.4/§7.5)
7. **`PackageDelever` generations + reserved reductions** — perp-short-first sequencing (transient never net-short), bounded generations after prior §6.7 finality, the reserved second leg the classifier subtracts, incl. `IntervalFloorReduction`. (from §5.3 + §7.1)

## 4. Mechanics

- **Numbering & cross-refs.** Each doc keeps its own section numbering. The normative core **retains current §-numbers where content stays** (so HANDOFF/memory citing §1.1/§6.7/§7.4 mostly still resolve); where a mechanism moves out, the core keeps a **one-line requirement + pointer** (e.g. §6.7 → "classify determined/ambiguous, quiesce to finality before compensating `[mechanism: State-Machines §2]`"). Cross-doc refs read `(Profiles §N)` / `(State-Machines §N)` / `(ADR §N)`.
- **Monolith handling.** `build-spec.md` is trimmed in place; the three new docs are siblings in `docs/` (no new subdir, so existing paths don't break). Its header points to B/C/D and states the v0.8 structural framing.
- **Traceability.** `adr-history.md` carries the full finding→(doc, §) table + the round-by-round convergence narrative; the review records stay in `docs/review/` and are linked, not duplicated.
- **Downstream refs.** `HANDOFF.md`, the tutorial's `§spec` cross-refs, and project memory are updated to the new set.

## 5. Non-goals

- No normative/behavioral change; no re-opening converged decisions.
- No new venue verification (facts already verified 2026-07-19; the one open verify-item — Binance sub-account availability — is carried forward, not resolved here).
- No formal/checkable *notation* for the state machines beyond structured tables + invariants (that was the rejected "full formalization" option); tables + prose invariants are the target.
- Not milestone 1 — this is the doc pass that *precedes* it.

## 6. Acceptance

1. Five docs exist per §3; the seven machines in doc C are each state-list + transition-table + invariants + chaos-cases.
2. **Content-equivalence verified** (Codex pass): no normative sentence lost, added, or changed; finding→(doc, §) map complete; zero dangling cross-refs.
3. HANDOFF + memory + tutorial refs updated; the set stamped v0.8.
4. A short "what moved where" note (old §X.Y → new (doc, §)) so the next reader — and the next reviewer — can navigate the transition.

## 7. Implementation order (for the plan)

Sequenced to avoid a broken mid-state (content in two places at once):
1. `adr-history.md` — move the Appendix + convergence narrative out first (frees the core, lowest risk).
2. `venue-profiles.md` — move §4/§4.1/§4.2 + dated citations.
3. `state-machines.md` — extract + structure the seven machines.
4. Trim `build-spec.md` to the normative core with requirement+pointer stubs.
5. Update all cross-refs + HANDOFF + memory + tutorial; stamp v0.8.
6. Losslessness / content-equivalence verification (Codex), fix any gaps.
