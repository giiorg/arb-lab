# Milestone 2 step 2 — the parity dispatcher + persisted `DispatchSeq`: design (2026-07-30)

**Status:** design only — **nothing in this step is built.** Session 40. Implements §31a's step 2
(`2026-07-29-milestone-2-capture-decisions.md`), whose deferral §31f declares "stops being deferrable
at this milestone." The normative contract is core **§3.4** (lanes, mailboxes, the drain) and **§3.5**
(the sequence discipline, the seams, the persisted log). Milestone 2's capture step (§31a step 1) is
closed at v0.9.15; capture runs detached and owes this work nothing.

**Doc set at design time:** v0.9.15. Branch `feat/milestone-1-foundation`. Verify with
`git status --short` and `make ci` rather than believing this line.

The live statement of what to do next is at **`## RESUME HERE`** at the bottom.

---

## §39 — the dispatcher design

### §39a — placement ratified: this is step 2, and the sketch that skipped it was a compression, not a decision

Session 39's closing summary sketched the next work as "calibration artifact → simulator" — steps 4
and 5 of §31a's roster — without naming step 2. **§31a's ordering is settled with rejected
alternatives recorded, and this record follows it rather than the sketch.** The reason step 2 cannot
be jumped is §31f's, and it is about the credibility of the kill-gate number itself: §8 defines the
research simulator as *"the §3.5 engine … over the `Clock`/`MarketDataSource`/`ExecutionVenue` seam
with a single total-order dispatcher, **not** a parallel simulator."* A simulator built before the
dispatcher exists would have to invent its own event ordering — which *is* the parallel simulator §8
forbids, wearing a temporary costume. The kill-gate verdict is produced by the engine or it is
produced by something whose agreement with the engine is unproven.

The accommodations §31f flagged as "already leaking" are the standing evidence: `engine/state`
accepts an equal `RecvTime` on write *because* every observation dispatched between `TestClock`
advances shares a timestamp, and its own doc comment says the FIFO invariant it would otherwise rely
on "lives inside a component that does not exist." Each further component built dispatcher-less adds
one more such accommodation, and collectively they become the parallel simulator.

### §39b — what exists and what this step adds

Everything below was verified against the tree this session, not quoted from memory:

| Seam | State | The fence it left for this step |
|---|---|---|
| `platform/events` | three lanes + `Envelope` built | doc comment: the dispatcher "needs a persisted DispatchSeq log, which does not exist yet"; the lanes "carry envelopes … and none of them assigns a sequence number or decides an order between two different sources" |
| `platform/events.ControlLane` | built, pull primitive present | "This is the shutdown drain's primitive, and it is also how a dispatcher pumps" |
| `platform/clock.TimerSource` | built, pull-based | "waking a live consumer … belongs to the dispatcher's select loop — which does not exist yet" |
| `platform/outbox` | relay + ports half built | store half explicitly deferred to the OMS journal (milestone 1 record item 1b) — see §39d |
| `engine/state` | cache built | accepts equal `RecvTime`; refuses a `RecvTime` regression *because* the dispatcher's FIFO guarantee is unbuilt — see §39h |
| `engine/capture` | codec + writer + verify, production-proven | append-only crash-safe framing with CRC, torn-tail recovery, sidecar digests — the discipline §39e reuses |
| `migrations/` | README only | stays README-only through this step (§39d) |

What this step adds: **the dispatcher core** (arbitration + `IngressSeq` stamping + timer pump +
wake), **the persisted dispatch log** (writer, reader, replay re-application), and the **wiring**
that drives `marketdata` from a replayed capture archive through the dispatcher into `engine/state`.

### §39c — where it lives: `engine/dispatch`

**Decision: the dispatcher is `internal/engine/dispatch` — a technical subsystem, not a kernel
package.**

- **Rejected: a `platform/` package.** Three grounds. §12.1's own roster separates them — it lists
  the kernel packages *and then* "parity dispatcher" as a distinct item, beside `engine/venue` and
  `engine/state`, not inside the platform list. The dispatcher is one-per-process spine, not a
  reusable primitive: `platform/` holds things many components hold (a `Clock`, an `Envelope`), and
  §3.5 is explicit that exactly one component pumps timers and stamps sequence — a kernel package
  whose contract is "only one caller may exist" is a category error. And structurally, kernelpurity
  binds `platform/` to stdlib + `platform/` imports only, while the dispatch *log* half of this step
  reuses `engine/capture`'s framing (§39e) — legal from `engine/`, forbidden from `platform/`.
- **Rejected: inside `platform/events`.** The lanes' doc comment already fences this out, with the
  reason recorded: the lanes are the primitives the dispatcher composes. Folding the composer into
  the primitives would also put the one-caller contract inside the kernel, above.

`engine/` placement gets the right gates for free: `notimenow` already denies ambient clock reads in
`engine/`, so the dispatcher holds an injected `Clock`/`TimerSource` or it does not compile the gate
run; `hexboundary` keeps `domain/`/`application/` from importing it, so contexts stay driven through
their ports from `app/` composition roots, never reaching into the spine.

### §39d — the persisted log does NOT pull in the outbox store half, and §31f's sentence saying it does is corrected here

§31f closes with: *"Pulling this in also pulls in `platform/outbox`'s store half (milestone 1 record
item 1b): the `Append` verb, the SQL, the migration, the driver."* **That sentence is wrong, and it
is wrong in a way this project has already named: a debt written against a document outliving the
reason it applied (§36l).** Two authorities contradict it:

1. **Milestone 1's item 1b itself** — the record §31f cites — binds the store half to a different
   trigger: *"`platform/outbox`'s store half is OWED, and it lands **with the OMS journal in
   milestone 3**."* The deferral's reason is the one-transaction property (append and journal write
   must commit atomically), which is a property of a store that must *join the journal* — and the
   journal still does not exist. Building the store half now would mean testing a guessed
   transaction shape against itself, the exact hazard item 1b defers on.
2. **Core §3.5** places the persisted order somewhere else entirely: *"The persisted order lives in
   a durable **capture lane** — the self-captured market-data streams (§8) plus the
   dispatched-sequence log, **separate from the OMS control-lane journal** (which persists
   order/account truth)."* The dispatch log is append-only replay data with per-stream sequence
   discipline — the capture lane's shape — not transactional business truth. SQLite, the migration
   and the driver are the journal's machinery, and the journal is milestone 3.

**Decision: the outbox store half stays where item 1b put it — milestone 3, with the journal.
`migrations/` stays README-only through this step.** The correction lands here, in the record that
inherits the debt, so the next session does not re-derive it from the contradiction.

### §39e — the dispatch log: capture-lane discipline, sequences not payloads

**Decision: the dispatch log is an append-only file in `engine/capture`'s framing discipline
(record + CRC + torn-tail recovery), written by `engine/dispatch`, holding sequence records —
`(IngressSeq, Source, AcceptOrdinal, VenueSeq, SourceTime, RecvTime, Prio)` plus the coalescing
lineage — and NOT payloads.**

- **Payloads stay in the raw archive, which is already the authority.** §3.5's own wording persists
  *sequences*: "both the pre- and post-coalescing dispatched sequences are persisted for replay."
  Replay re-applies the persisted order and re-derives each dispatched payload deterministically
  from the raw capture (reconstruction is deterministic in `VenueSeq` order per source; coalescing
  is "a deterministic rule keyed on `(Source, field)`" whose merge points are exactly what the log
  freezes). A second payload copy is the second-authority shape §31b/§31e exist to refuse: two
  records of one book snapshot can disagree, and then one of them is lying.
- **Rejected: SQLite for the log.** §39d's grounds — §3.5 separates this lane from the journal by
  name, and the log's access pattern (append during dispatch, sequential read at replay) is what
  append-only framing is for. SQLite would also drag the driver dependency into `engine/` a
  milestone early, for no property the framing lacks.
- **Rejected: a second bespoke framing.** Two crash-safe framings mean two torn-tail recoveries to
  prove, two fuzz corpora, two oracles. `engine/capture`'s codec has all three plus a production
  incident history (§38i). Reuse is of the **codec package**, not the capture tool: §31e's
  permanent fence — the capture *tool* must never become the engine's ingestion path — is about
  ordering authority, and a codec decides no order. The import is `engine/dispatch` →
  `engine/capture`, engine-internal, which no gate objects to and §39c's placement makes legal.
- **Scope honesty, named now:** milestone 2's streams are market data plus timers — there are no
  private-stream envelopes until the OMS exists. Whether *private* (order/account) payloads are
  captured raw the way market data is, or the journal's record suffices for their replay, is a
  **milestone-3 design question owed to that record**, not silently answered here.

### §39f — the dispatcher's invariants, stated as the claims tests will bite

The core is a single consumer loop draining three input kinds — the control lane, the per-`SourceID`
market-data mailboxes, `TimerSource` — and stamping each released envelope. The invariants, each a
mutation target when the slice lands:

1. **`IngressSeq` is dense, monotonic, starts at 1, and is stamped only at dispatch.** A
   pre-dispatch envelope carries zero; `Envelope.Validate` is the post-stamp check (already true in
   `platform/events` and asserted there — the dispatcher is what finally exercises it).
2. **An unready `SourceID` never stalls dispatch of ready inputs from other `SourceID`s** (§3.5).
   Readiness is per-mailbox; there is no cross-source ordering to wait for.
3. **Per-`SourceID` FIFO is preserved through dispatch.** This is the invariant `engine/state`'s
   regression check currently guards *against the absence of* — §39h.
4. **Timer drain order is `(At, Prio, ID)`, lower `Prio` more urgent**, and a fired timer's
   envelope carries `AcceptOrdinal == 0` exactly when it derives from the absence of input (§3.5's
   silence rule); the dispatcher **refuses** a zero ordinal on an input-derived envelope rather
   than passing it through.
5. **`Prio` is a per-`SourceID` constant and the dispatcher asserts it**: the first envelope from a
   source fixes the value, a later disagreement is refused loudly. §3.5 makes constancy the
   producer's obligation; asserting it here is the checked half, because the failure it prevents —
   a degrading feed appearing to recover — is silent by construction.
6. **The dispatcher never advances time.** The driver owns the clock: live mode's select loop waits
   on real time and the wake signal; the sim's driver advances `TestClock` to the next due timer
   only when no envelope is dispatchable. One arbitration, two pumps — §39g.

### §39g — one arbitration, two pumps, and what "frozen" buys

§3.5: *"live dispatch resolves any concurrent reconstruction-ready timing … and freezes that
arbitration in the log, so replay is deterministic without re-deriving order."* Consequence, made
explicit so the slice does not over-build: **the live pump's tie-breaks are permitted to be
scheduler-dependent, because the log is the authority for what they resolved to; the sim pump must
be deterministic given the archive, because in the sim the dispatcher is generating the order, not
replaying one.** Two runs of the sim over the same archive and config MUST produce byte-identical
dispatch logs — that is the parity probe, and it is the "done when" of the wiring slice (§39j).
A replayed log MUST re-drive consumers identically whether it was written by the live pump or the
sim pump; the log format cannot record "which pump wrote me" as anything a consumer branches on.

### §39h — `engine/state`'s accommodation is RATIFIED, and its check stays

The cache accepts an equal `RecvTime` (the `TestClock` parity case) and refuses a regression,
documented as guarding against "the day that component lands with a different [ordering]." The
dispatcher now lands, with FIFO preserved (§39f-3) — and **the check stays anyway.** A consumer that
verifies an invariant with a single comparison is strictly better than one that trusts it, the
comparison already exists and is bite-checked, and removing it would convert a documented defence
into a documented regression for zero saved work. The doc comment's "should not arise" clause gets
updated by the slice to name the dispatcher as built; the behaviour does not change.

### §39i — the command sequencer stays absent, and the comment that promises it gets scoped

§3.4's fourth bullet (Risk→OMS command sequencer) is fenced out of `platform/events` with "Built
with the dispatcher." **It is not built with this dispatcher, and the reason is the standing
empty-tree argument:** neither a Risk context nor an OMS exists, so a sequencer now would order
nothing between nobody — a check over an empty tree, indistinguishable from a broken one. The slice
that lands the dispatcher updates that one comment sentence to name the real trigger (the first
Risk→OMS command path, milestone 4), so the promise carries its construct the way a deferral must
(the rule 10 lesson: name the construct the component reads, not the feature that motivated it).

### §39j — build order

| Slice | Contents | Done when |
|---|---|---|
| **A** | dispatcher core: drain loop, `IngressSeq` stamping, timer pump, `Prio` assertion — fake sources and `TestClock`, no persistence, no I/O | §39f's invariants 1–6 each have a test that a mutation kills; `-race` clean |
| **B** | the dispatch log: sequence-record schema over `engine/capture` framing, writer, reader, replay re-application; positive control = a truncated/vandalized log is refused, not absorbed | a log written by slice A's core replays to an identical consumer-visible order; torn tail recovers |
| **C** | wiring: a replayed capture archive → ingestion (`AcceptOrdinal` at the seam) → `marketdata` → mailboxes → dispatcher → `engine/state`, under `TestClock` | **the parity probe: two runs over the same archive produce byte-identical dispatch logs**, and the replayed log re-drives `engine/state` to the same terminal cache state |

Slice C's fixture is the real archive format over a *small committed fixture archive*, not the 6
GB/day production tree; the production archive is a manual soak, not CI.

Standing verification discipline applies unchanged: fixture-collapse checklist **before** mutations
(named suspects at slice time — the known family: a fixture whose sources never become ready
simultaneously collapses the arbitration under test; a single-source fixture collapses invariant 2;
a mailbox depth pinned to 1 collapses coalescing-lineage recording), then aimed mutations, then the
close's reading pass beside the marker grep.

### §39k — spec-edit roster

No edits are known owed at design time. The §31f correction (§39d) is a **session-record
correction, in-record here**, not a core edit — core §3.5 was never wrong about where the log
lives. Watch items the slices may promote to marked edits:

| # | Doc | Watch |
|---|---|---|
| 1 | core §3.5 | if the sequence-record's field set needs stating normatively once it exists (the §32e-manifest precedent: the field list landed as a dated in-record edit at slice close) |
| 2 | core §3.4 | if the command-sequencer sentence's timing ("built with the dispatcher", currently only in code comments) turns out to be stated in the core anywhere — the reading pass checks; grep at design time found it only in `platform/events` |

### §39l — open questions owed to later steps, named without pre-answering

Recorded so step 3–5 designers inherit questions, not assumptions (the §36l/§38 lesson about debts
that pre-answer themselves):

1. **The ≥3-month historical window for §8's fill model.** Self-capture began 2026-07-30, so the
   window §8 requires cannot come from our archive until late October — and §8 additionally
   requires a negative-funding regime inside the sample, which no calendar guarantees. What data
   backs the window (Tardis purchase, venue REST funding history, a bound whose structure needs
   less than books) is **step 4/5's design decision**, with §38f's funding-is-refetchable finding
   and §31d's books reasoning as inputs.
2. **Where paper decisions are journaled** (§12.2 "paper decisions journaled") given no OMS journal
   exists in this milestone.
3. **Whether the live pump is exercised in milestone 2 at all**, or only the sim pump — §12.2's
   read-path phrasing suggests a live paper path, which would also unblock the raw venue WS clients
   whose milestone-1 objection ("no dispatcher to feed") this step dissolves. Step 3's record
   decides; nothing in slices A–C depends on the answer.

---

## RESUME HERE

**This record is the design; nothing of it is built.** The next session builds **slice A** (§39j):
`internal/engine/dispatch`, the core loop against fake sources under `TestClock`, no persistence.
Before anything else, the standing rituals: `pgrep -fl "arblab capture run"`, then a `capture
verify` over the accumulated range (expect 0 integrity findings, always), then `git status --short`
and the marker grep.

Standing calendar item unchanged: **slice D's one-day disk figure ~09:53 UTC 2026-07-31**
(`du -sh archive/` + full-day verify) — one command, records into the capture design record, does
not gate this work. The off-host destination remains the user's open §38f decision; the laptop is
still the only copy.

Do not reopen: §31a's ordering (this record is its step 2), §31b (funding consumed, never derived),
§39c's placement, §39d's correction (the outbox store half is milestone 3's, with the journal).
