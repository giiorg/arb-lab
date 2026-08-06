# Design record §72 — M3 task #197: the §12 closure pass, and a clause the register had already filed elsewhere

**Date:** 2026-08-05 · **Session:** 108 · **Docs:** v0.9.50 · **Task:** #197

**Reads:** core §12 (the milestone roster — the subject), core §6 step 4 (the
sizing requirement), core §7.1 (*"Sizing for survival & the margin top-up
protocol"* — where it lands), core §1.1 (capital tiers), `adr-history` §3's
traceability table (the `C4` row, which decided it), `venue-profiles` §4/§7/§9,
the slice records §55–§59, §47c/§47e (the admission boundary and the two scope
findings this pass is a third instance of).

---

## §72a — Why a closure pass, and the question it had to answer first

Milestone 3's seven slices and its twenty-eight-task tail were all landed and
recorded. What was **not** recorded was that the milestone was done. §12's item 3
carried a landing annotation for every slice through **E (v0.9.35)** and then
stopped, with the doc set at **v0.9.49** — fourteen versions of unannotated work,
and no sentence in the normative core stating completion.

The session opened on the observation that this is **§47e finding 1's shape**:
§12 recorded milestone 1 complete while half of `engine/venue` had no code at
all, and the `SPEC EDIT OWED` grep is structurally blind to it because an
**unmarked** claim leaves no marker to find. The roster is the one artifact that
asserts what *exists*.

**The question that had to be settled before any editing**, because the wrong
answer would have made fourteen versions of annotation the wrong fix:

> Was the roster **deliberately demoted** to a summary once `adr-history` §3 and
> the per-slice design records became the traceability record?

Under that reading the gap is correct, the annotations are redundant, and the
only defect is that the demotion is unwritten — a one-sentence fix. This is the
reading a later session would most plausibly reach for, so it was tested rather
than argued.

**It is false, and the evidence is the records' own close lists.** The
slice-C2a record names *"§12's slice-C2a annotation"* among what the slice lands.
The slice-C2b record names a four-part landing: *"§12's slice annotation, ADR
§3's dated block, HANDOFF, v0.9.33 across all"*. From **slice F1 onward — the
same day as slice E** — no record names it, and **no ruling retired it**. A close
checklist item vanished between two same-day sessions.

`adr-history` had already named this exact mechanism once, on the capture
release: roster rows left unmarked because *"no code asserts them yet"*, a state
that **expired silently**, where *"the close's reading pass is what re-derived
it; the grep, by construction, could not."* The recurrence is what makes it
worth recording rather than just fixing.

**Ruled (user, 2026-08-05):** annotate through G3 and state completion. The
lapse is stated in place, so the record shows what happened rather than only its
repair.

## §72b — The pass: every clause against the code, not against the summaries

§47e's discipline is the whole method here. A session summary reports what a
session **believed** it delivered; the roster asserts what **exists**; only the
code settles it. So item 3's nine clauses were each checked against `internal/`
independently of every HANDOFF block written about them.

| Clause | Verified against |
|---|---|
| write-ahead journal | `oms/infrastructure/driven/journal.go` (#160) |
| `DispatchAuthorized`→query-only crash boundary (§5.3) | `dispatch.go`, `authority.go`, `resolve.go`, `execute.go`, `replay.go` |
| deterministic client IDs | `platform/ids` |
| parallel-IOC | #181 / record §59 |
| `OrderByClientID`/`Fills` recovery, venue-specific stable-not-found, Bybit history fallback | #163 / §53, #172 / §52 |
| one-leg-failure + net-delta invariant | `classify.go`, `join.go`, `module.go` (#178 / §56) |
| ambiguous-order recovery state machine model-tested before live capital | formal model (a), `FM-1`…`FM-9` |
| testnet integration incl. process-kill-mid-IOC | `app/exercise/chaos_test.go` (#181) |
| **sizing-by-live-tier** | **nothing** |

## §72c — THE FINDING: a clause with no code, and the register had already filed it elsewhere

`MMR`, `maintenance margin`, `leverageBracket`, `risk limit`, `marginTier` —
**zero** non-test hits across `internal/`. No `internal/risk/`. No sizing
function of any kind: `min(per-trade cap, per-venue headroom,
fillable-within-slippage, margin-for-target-liq-distance)` has no
implementation, and the live-tier MMR read it needs is performed nowhere.

The disposition is a **reassignment**, on four grounds, and the first is the
cheapest and was available the whole time:

1. **`C4`'s traceability row reads `| C4 tiered MMR | Profiles §1, core §7.1 |`.**
   §6 step 4 tags the requirement `[→ C4]`; §7.1 is titled *"Sizing for survival
   & the margin top-up protocol"*; §12 gives §7.1 to **milestone 4** in terms
   (*"the margin top-up protocol (§7.1, via `MarginGateway`)"*). **The clause's
   own tag pointed into the next milestone for as long as the roster has
   existed** — one row in this project's own traceability register, never opened
   because nobody had cause to ask where the clause lived.
2. **The OMS's delegation is designed, not omitted.** `planning.go`'s `Quantity`
   doc comment assigns ownership upstream (*"§6 step 4's sizing is responsible
   for having quantized it"*) and `Plan` then **verifies** that claim against the
   instrument's `TradingFilters` rather than trusting it — the signature of a
   deliberate seam. A milestone that had "completed" sizing would have had to put
   a producer inside the single-writer OMS, which is the wrong home for one.
3. **The inputs belong to milestone 4.** The live-tier MMR needs a signed
   `GET /fapi/v1/leverageBracket`, and §4.1's account/margin-mode verification —
   leverage pin, one-way position mode, isolated margin — is what makes a tier
   reading mean anything at all.
4. **The teaching companion had already paired them.** `tutorial.md`'s acceptance
   checklist reads *"live-tier MMR sizing + **margin top-up protocol**"* as one
   clause. Independent of the spec's own filing, and it agrees.

**Ruled (user, 2026-08-05):** misfiled — move it to milestone 4.

## §72d — What the finding is actually about: a chain of deferrals with no terminal

The roster defect is the visible half. The half worth carrying:

- **The venue fact is recorded and read by nobody.** `venue-profiles` §4 carries
  *"fetch `GET /fapi/v1/leverageBracket` at runtime — table not otherwise
  readable `[→ C4]`"*, and §7 records it as signed `USER_DATA` at IP weight 1.
  The research was done; the consumer never arrived. This is
  *schema-field-frozen-before-its-consumer* inverted — the **fact** frozen before
  its reader, rather than the field before its writer.
- **Eight comments across seven packages defer liq-distance to a computation no
  package owns.** `marketdata/application/funding.go`,
  `marketdata/infrastructure/driven/publisher.go`, `marketdata/domain/freshness.go`,
  `platform/events/coalesce.go`, `platform/events/envelope.go`,
  `platform/ids/instrument.go`, `platform/config/reload.go`,
  `platform/observability/spec.go` — each names liq-distance as a quantity
  computed *elsewhere*, and each is **locally true**. Collectively they describe a
  computation with no owner. No gate can see this: every comment is correct, and
  the marker grep polices only *marked* claims.
  - **The figure was wrong on its first writing, and the correction is the
    method.** The first pass said *"nine comments across five packages"* — the
    ninth was `strategy/infrastructure/driving/procedure.go`, which the grep
    matched on **`headroom`** rather than on liq-distance, and the package count
    was simply miscounted. It had already reached five documents before being
    re-derived. §54 finding 2 is the same shape (a stale `NC20` state count
    copied into six documents): **a figure no run reproduces reads exactly like
    one that does**, and the only defence is re-running the grep that produced it
    with the pattern narrowed to the claim actually being made.
- **The failure direction is a halt, not a wrong number.** With no producer,
  `Plan` cannot be handed a size at all — nothing mis-sizes. That is precisely
  how the clause survived five slices, a twenty-eight-task tail, two formal
  models and a mutation campaign totalling several hundred mutants without
  anybody noticing. **A missing thing that fails loudly gets built; a missing
  thing that fails by never being reached gets filed.**

§47e finding 1 is the same defect one milestone up. That makes this the
**second** instance, and the roster the artifact both were invisible on.

## §72e — Rejected alternatives

| Rejected | Why |
|---|---|
| **Record the demotion instead of annotating** (declare ADR §3 + the design records the traceability record, §12 a summary that no longer carries per-slice annotations) | Cheaper, and arguably how the docs already work in practice — but it means the roster **never states what is delivered**, which is the exact property that let §47e finding 1 hide on milestone 1 for four sessions. Offered to the user and declined. Had it been taken, this pass's one real finding would still have been found, since the clause check does not depend on the annotation decision |
| **Take the roster at its word — milestone 3 is incomplete, build sizing now** | Pulls the `leverageBracket` fetch and a liq-distance computation ahead of §4.1's account/margin-mode verification, which is what makes a tier reading meaningful. It would build a consumer for a posture nothing has yet verified — and §7.1, the section that owns the requirement, is milestone 4's by §12's own text. Offered and declined |
| **Split the clause** (mark the quantization half delivered as #182, move only the MMR half) | Defensible: §6 step 4 does contain both, and `[→ X7]`'s quantization genuinely landed. Rejected as a *roster* edit because #182 is already recorded on its own, and splitting one roster phrase across two milestones makes the roster harder to read for no gain in what it asserts. The distinction is preserved in §72c ground 2 instead. Offered and declined |
| **Delete `sizing-by-live-tier` from item 3's text** | Rejected on milestone 1's own precedent: §47e's scope correction **annotated** rather than rewrote, so external review can tell a deliberate change from drift. The phrase stays; the annotation strikes it |
| **Treat the second summarizer's `positionRisk` reading as settling #180** | It carries all three tells already recorded against the first summary (one aggregated catalog page for every answer, V2/V3 reported byte-identical, changelog cited via search-engine redirects), and it disagrees with the first by 2×. A summarized fetch is not a source — the rule that produced v0.9.49's `/api/v3/account` correction. Recorded as a third candidate; the charge stays at the max |
| **Lower the charged weight to 10** on the newest reading | The charge is set by the **worst** candidate, not the newest. 20 is correct-or-pessimistic under all three, costs 15 extra weight once per reconcile pass at worst, and under-charging is the direction that ends in the §7.6 ban |

## §72f — What this pass did NOT do

- **No new normative process rule.** The obvious move — writing "a slice does not
  close until §12 says it landed" into §3.6 or §12 — was not taken. It was not
  asked for, and a rule added in reaction to one lapse is the kind of thing that
  reads as settled practice a year later without ever having been weighed.
- **No code change.** The `positionRisk` file comment was updated to state three
  readings instead of two and to say why the max is charged; nothing executable
  moved. `go build ./...` and the venue suite were re-run against it.
- **No milestone-4 scoping.** §47c's *"admitted is not scheduled"* boundary is
  untouched. Milestone 4 gained a clause it already owned by traceability, not a
  schedule.

## §72g — Gate results

| Gate | Result |
|---|---|
| `grep -rn "SPEC EDIT OWED" internal/ tools/` | **0** |
| `make ci` (both modules, uncached — `go clean -testcache` first) | **green**, incl. all six structural gates and `tutorial-ka-check` |
| `go build ./...` + `go test ./internal/engine/venue/ -run Position` | pass (comment-only code change) |
| Doc set version | build-spec, venue-profiles, adr-history → **v0.9.50**; state-machines (v0.9.42) and tutorial (v0.9.44) unchanged, neither doc touched |
