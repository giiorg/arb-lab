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

### §39m — slice A is BUILT and CLOSED (session 41, 2026-07-30): the core loop, its arbitration, and what the battery proved

**What landed:** `internal/platform/events.MarketDataLane.Peek` (one commit) and
`internal/engine/dispatch` (one commit): `Config`/`New` (zero value unusable, each missing wire
named), `Step` (at most one item per call), `release` (the stamp-and-check boundary), `pickMarket`
(the market arbitration), `cmpSource` (the total order), `Sink` (two methods, one counter). The
§39h and §39i comment updates rode the same commit: `engine/state`'s regression-check comment now
cites the ratification instead of "not yet built", and `platform/events`' command-sequencer fence
names its real trigger (the first Risk→OMS command path, milestone 4) instead of "built with the
dispatcher".

**§39m-1 — the arbitration is a RANK between kinds, and that shape was forced, not preferred.**
Due timers first (in the queue's own `(At, Prio, ID)` order), then the control lane's FIFO head,
then one ready market mailbox. **Rejected: cross-kind `Prio` arbitration.** It requires seeing
every kind's head without consuming it, and `TimerSource.NextDue` and `ControlLane.TryReceive` are
pop-only *on purpose* — arbitrating on their contents would mean popping an item and holding it
back, which is the re-queuing the pull design exists to forbid (the clock package's own recorded
reason). Timers outrank envelopes because a due timer is a deadline already in the past — §3.4's
silence timer and the funding boundary are exactly the events that must not queue behind traffic —
and control outranks market because lossless private truth (a fill) losing a race to lossy derived
data (a book snapshot) is §3.4's lane order inverted. The starvation direction is stated rather
than hidden: continuous due timers or nonstop control traffic starve market data, which is safety
first; the reverse starvation (a book firehose delaying a fill) is the one that kills.

**§39m-2 — market arbitration: most-urgent head `Prio` wins, ties rotate round-robin over a
field-wise-sorted roster.** Three rejections, each with the failure it avoids: **a learned-Prio
map as the arbitration key** (unknowable for a source before its first dispatch — the constant is
an output of dispatching, not an input to it); **sorted-first-wins** (a firehose source at the
lexically lowest key starves an equally urgent peer for as long as it stays ready, which for a
book stream is always); **`Sources()` map order** (Go randomises it deliberately, and the sim
pump's arbitration must be a pure function of lane state or the parity probe's byte-identical-logs
claim is false on arrival). The rotation cursor is dispatch history, which the log freezes, so the
rotation is replay-safe by construction.

**§39m-3 — `Peek` is the kernel addition the arbitration needed, and its contract is advisory on
purpose.** Choosing which ready mailbox dispatches next is a decision over the heads' priorities;
the pop-only `Consume` would force consuming in order to look. Peek reports the oldest envelope
without removing it, and under concurrent producers the head can change between Peek and Consume
(overflow eviction; a depth-1 coalescing mailbox merging into its own head) — permitted, because
§3.5 freezes arbitration *outcomes*, not deliberations. Every per-envelope check runs on what
Consume returned. **Rejected: consume-and-holdback** (a one-slot buffer per input) — dispatcher
state that re-queues what the lanes were handed.

**§39m-4 — refusals are loud, consume the offender, and burn no sequence; a sink error spends the
sequence.** The check order in `release`: the second-authority refusal (`ErrIngressSeqMinted` — a
producer contesting the one job this package has), then the post-stamp `Envelope.Validate` exactly
where §3.5 places it, then `ErrPrioInconstant` judged only on a well-formed envelope. The refused
envelope is consumed — leaving it would jam its source forever — but nothing reached the sink and
`LastSeq` is unmoved, so the dispatched sequence stays dense with no holes for slice B's log to
explain. A sink error is the opposite case: the order was decided, the stamp is spent, `Step`
reports `(true, err)`.

**§39m-5 — the absence-derived door is deferred, and the deferral names its construct.** §3.5's
one legal `AcceptOrdinal == 0` envelope derives from the *absence* of input. Its entry door must
REQUIRE zero where every other door refuses it — and no producer of such envelopes exists (§3.4's
silence re-judger lives in marketdata's freshness machinery, unbuilt). A door only tests would
walk through is a check over an empty tree. **The trigger is the first producer of an
absence-derived envelope — the §3.4 silence re-judger — and the door lands in the same slice.**
Until then the dispatcher refuses a zero ordinal from every input (`Validate`'s existing check),
which is §39f-4's refusal half, built and bitten.

**§39m-6 — a timer firing occupies an `IngressSeq` position.** `OnTimer(seq, f)` and stamped
envelopes share one dense counter — D4's "total order over all inputs (events *and* timers)" made
mechanical, and the seq rides *beside* `Fired` because teaching the clock about dispatch sequences
would put the replay key in two packages. Watch (reinforces §39k item 1): whether the core needs
to state timer positions in the log normatively is slice B's schema question.

**§39m-7 — the core is clockless, and prio constants fix at first successful dispatch.** The
dispatcher holds `TimerSource` and no `Clock`, making §39f-6 compile-shaped (no `Advance`, no
`SetNow`, no `Now` to call). A source's `Prio` constant is fixed by its first *dispatched*
envelope, not its first offered one — a refusal never entered the consumed order and binds nothing.

**§39m-8 — verification, full rhythm.** The checklist ran BEFORE the battery and found two gaps at
authoring review: **every market fixture differed only in `Symbol`**, so a comparator collapsed to
symbol-only would have passed the whole suite — and the realistic accident is the actual §32c
capture set (BTCUSDT on two venues, spot and perp); the added fixture holds four same-symbol
sources differing pairwise in venue, product/contract and stream. And **control FIFO through
dispatch was untested while market FIFO was** — the mirror rule, applied. Then **24/24 aimed
mutations caught** (M1–M24: the rank rungs, both seq advances, the stamp, the minted-seq and
Validate refusals, constancy in both directions plus never-learned and learned-on-refusal, the
map-order roster, the empty-mailbox candidate, the inverted urgency compare, dropped ties, cursor
repick/ignore/wrap-to-back, comparator blind to venue/symbol/stream, Peek-returns-newest). Honest
non-aims, named so a later reader does not mistake them for gaps: **the `Product` and `Contract`
comparisons in `cmpSource` are mutually redundant on valid instruments** (product ↔ contract is
bijective: spot↔``""``, linear_perp↔`linear`, inverse_perp↔`inverse`), so blinding either alone is
an equivalent mutant — the pair stays because the comparator must remain total if `ids` ever adds
a product whose contract collides; **the Peek-race branch in `Step`** (head evaporates between
Peek and Consume) is deterministically unreachable from tests and deliberately defensive — its
chosen behaviour (re-arbitrate next Step, not an error) is documented at the branch; and **a
rank-reorder mutant** (timers checked after lanes) is not expressible as a line sed — M1/M4
disable each rung instead, and the order tests assert the rank directly. Invariant 6 has **no
mutable line at all**: its guarantee is the type system's, which is the strongest shape a claim
takes here.

**§39m-9 — no spec edit owed.** The kind rank and the rotation are arbitration §3.5 deliberately
leaves to the dispatcher to decide and freeze; nothing this slice asserts contradicts or extends
the core. Marker grep zero. The reading pass's one borderline claim — timer seq positions — is
§39m-6's watch item, owed to slice B's schema if the log needs it stated.

---

### §39n — slice B design (session 42, 2026-07-30): the dispatch log's format, decided before building

§39e fixed the discipline (capture framing, sequences not payloads, codec reuse); this section fixes
the remaining shape. Each decision carries its rejected alternative.

**§39n-1 — the container is §32e's frame VERBATIM, and the kinds are reused with their capture
meanings.** A dispatch log file is `ARBCAP` v1.0: file header, then `RecordKindData` frames whose
payloads are sequence records, closed by one `RecordKindClose` seal. **Rejected: new `RecordKind`s
(`DispatchEnvelope`/`DispatchTimer`).** Adding a kind bumps `FormatMinor` (the codec's own
versioning sentence), and `ParseFileHeader` refuses an exact mismatch in BOTH directions — so the
bumped build refuses the v1.0 production archive it is mid-way through capturing, or the refusal
semantics get relaxed mid-production for the benefit of a file that is not the archive. §32e's kind
roster is also normative core text, so the two kinds would be a core edit spent on container
plumbing. **Rejected: a dispatch-owned magic/version.** That is the second framing §39e already
refused, wearing one field's clothes. The kinds transfer with their meanings intact: `Data` is "a
payload this container did not construct", `Close` is "the absence of me is the crash signal" —
both exactly what the log needs them to say. The capture PACKAGE stays ignorant of all of this
(§32n's fence: the import is dispatch → capture; capture never learns what a `CaptureSeq` means to
its caller).

**§39n-2 — frame field mapping: nothing is stored twice.** `CaptureSeq` **is** the
`IngressSeq`/`DispatchSeq` — the frame already has a dense-per-stream sequence field, and a payload
copy would be a second thing to disagree (the `Sink` doc's own rule, applied to the file).
`RecvTime` is the input's own replay-stable instant: the envelope's `RecvTime`, the timer's
scheduled `At` — never a delivery-time reading, because the parity probe's byte-identity claim
makes any per-run stamp a defect by construction. `Mono` is the constant 0 for the same reason,
stated rather than left as an accident: Mono is per-run process timing, the log's authority is
order, and a real reading here would break byte-identity while measuring nothing the log answers
for. (A timer scheduled for the zero time is refused by the codec's own `RecvTime` check — an
honest refusal, since such a timer cannot exist under §11's zero-value rule.)

**§39n-3 — the payload is JSON, discriminated by a `k` token, with no version integer.** Precedent
is gap.go's header/payload boundary: fixed-width lives in the frame header, self-describing tokens
live in the payload where they cost bytes only on records that carry them. An envelope entry
(`k:"env"`) holds the five `SourceID` fields flat, `accept`, `venue_seq`, `source_time_ns` (omitted
when the venue sent none — absence is the semantic, so absence is the encoding) and `prio`; a timer
entry (`k:"timer"`) holds `id` and `prio` (`At` rides in the frame). **Rejected: a payload-schema
version field.** Nothing would ever bump it and the reader would compare it against a constant — a
door only tests walk through, §39m-5's argument one layer down. The `k` token IS the refusal
surface: an unknown token is refused, a shape change is a new token, an additive field is
compatible (readers ignore unknown JSON fields, stated as the evolution contract), and a frame-
layout change belongs to the container's version, not ours.

**§39n-4 — one `Close` record seals the log, and it is what bounds the frameLen residual.**
`errors.go`'s documented residual — a corrupt final frame length mis-reads as a torn tail — is
WIDER here than in the archive until a seal exists, because the log has no digest layer. Worse, a
buffered writer can crash having flushed only whole frames: `TornBytes` 0, and without a seal that
file is byte-for-byte indistinguishable from a clean end. `RecordKindClose`'s own argument ("its
absence is the signal") covers both: the reader reports `Closed`, and a mis-diagnosed tail always
surfaces as a missing seal. The Close frame's `CaptureSeq` repeats the last entry's seq and its
payload carries the entry count (`entries` alone — no `k` token, because the frame kind already
discriminates a Close and a second discriminator could disagree with it) — the one deliberate
duplication, because a seal that asserts nothing seals nothing; the reader cross-checks both. Its `RecvTime` is the last
entry's instant — the only honest replay-stable time a clockless writer has. An EMPTY log gets no
seal (no instant exists to stamp one); header-only reads as empty, torn-unknown. **No Open record:**
its content would be provenance, and provenance + rotation + fsync policy are one bundle owed to
the live pump's design — the deferral names its construct, the composition root that first holds a
`TimeSource` for this log (§39l-3's question decides when that exists).

**§39n-5 — the writer is a `Sink`, write-ahead of the sink it wraps.** `LogWriter` freezes the
entry to the file BEFORE the wrapped sink sees the item: a log missing an entry the state applied
un-freezes the arbitration §3.5 persists (replay would re-derive an order live already decided),
while the reverse — a logged entry the state never applied because the process died between the
two — replays harmlessly, since replay re-applies from the log into a fresh state. Density is
ASSERTED, not trusted (`seq == last+1` from 1): §39h's argument verbatim — a writer that verifies
the dispatcher's one guarantee with one comparison is strictly better than one that trusts it, and
the refusal also catches a `LogWriter` joined to a dispatcher that has already dispatched (a log
that starts mid-run is a lie about where the order began). **Fresh files only** (`O_EXCL`): an
append door would be walked only by tests, because no dispatcher counter survives a crash in this
milestone — resuming a dispatch sequence is the journal's business (milestone 3), and the deferral
names that construct. **Rejected: `capture.Writer` wholesale.** `StreamKey` cannot name a
non-venue stream (its `SourceID` validates against real venues); rotation, alive ticks, the
advisory lock and the manifest are archive-tool policy on the far side of §31e's fence; and the
writer's clock requirement would put a `TimeSource` in a package whose clocklessness is
compile-shaped (§39m-7).

**§39n-6 — the reader recovers exactly what capture recovers, and refuses exactly what capture
refuses.** `ReplayLog(path, consumer)` walks the file with the container's own scanner: a torn tail
is the good prefix plus a reported `TornBytes` — recovered, never silent, never truncated on disk
(the reader owns recovery here; there is no appending writer to hand a clean file to) — and
everything else (`ErrChecksum`, a reserved bit, an undeclared kind, a zero-filled region) is a
refusal naming the offset, capture's `ErrTruncated`-versus-everything-else split inherited whole.
On top of the container's checks, the log's own: `CaptureSeq` dense from 1 (a hole in an
in-process-written log is not a drop, it is a wrong file), frame kinds outside {`Data`, `Close`}
refused, a record after the seal refused, the seal's count cross-checked against entries seen, and
the decoded `SourceID` re-validated (the writer trusted the dispatcher's post-stamp check; the
reader re-deriving it is the two-independent-sides discipline, cheap). The consumer mirrors `Sink`'s
shape — `OnEnvelopeRecord`/`OnTimerRecord` — because replay's job is re-driving, and a consumer
shaped like the write side composes with slice C's wiring without an adapter.

**§39n-7 — capture exports `WalkRecords`, and the recovery stays proved once.** The sliding-window
scanner (`ensure()`'s decode-only-on-exhausted-buffer property, the one whose loss truncates a
healthy file to its first 64 KiB) is the part §39e's "two torn-tail recoveries to prove" rejection
protects. It gets ONE public door — `WalkRecords(f, fn(Record, raw []byte)) (WalkSummary, error)` —
and `scanArchive` is rebuilt on it, so the existing recovery battery keeps biting the shared
machinery and the export cannot drift from the recovery it claims to be. (The raw-bytes argument
exists because scanArchive's digest needs the exact frame bytes; the file header's contribution is
the constant `AppendFileHeader(nil)`.)

**§39n-8 — checklist suspects and mutation roster, named at design time.** Suspects, before any
battery runs: a fixture log holding only ONE entry kind (collapses the `k` discriminator and the
timer-positions claim — the realistic log interleaves timers among envelopes); truncation pinned to
a record boundary (collapses `TornBytes` math — the tear must land mid-record, and separately AT a
boundary to prove the seal catches what `TornBytes` cannot); every envelope fixture carrying a
`SourceTime` (collapses the absent-vs-present encoding — the §32c reality is that some streams
never send one); vandalism only in CRC-covered bytes (the frameLen field is outside the checksum —
§39n-4's residual needs its own fixture); a single-entry log (collapses "dense" against "starts at
1" — two rules, one fixture value); and the WAL-order claim needs a FAILING wrapped sink, or
log-before-next and next-before-log are indistinguishable (the §38h driver lesson: nothing had ever
driven the refusing branch). Mutation targets: the density comparison (`==` vs `>=`), start-at-1,
the WAL order swap, seq advance on Close, the kind-roster refusal, the `k`-token dispatch swap, the
seal's count cross-check, the after-Close refusal, `TornBytes` reporting, the `source_time_ns`
absence round-trip, and `Mono`/`RecvTime` byte-identity (two identical runs, one comparison).

---

## RESUME HERE

**Slice A is closed (§39m). The next session builds slice B** (§39j): the dispatch log —
sequence-record schema over `engine/capture`'s framing, writer, reader, replay re-application.
Positive control: a truncated/vandalized log is refused, not absorbed. Done when a log written by
slice A's core replays to an identical consumer-visible order and a torn tail recovers. §39m-6's
watch item is slice B's first design question: whether the sequence-record field set (and timer
positions in it) needs stating normatively in core §3.5 — the §32e manifest-field-list precedent
says a dated in-record edit at slice close is the shape.

Before anything else, the standing rituals: `pgrep -fl "arblab capture run"`, then a `capture
verify` over the accumulated range (expect 0 integrity findings, always), then `git status --short`
and the marker grep.

Standing calendar item unchanged: **slice D's one-day disk figure ~09:53 UTC 2026-07-31**
(`du -sh archive/` + full-day verify) — one command, records into the capture design record, does
not gate this work. The off-host destination remains the user's open §38f decision; the laptop is
still the only copy.

Do not reopen: §31a's ordering (this record is its step 2), §31b (funding consumed, never derived),
§39c's placement, §39d's correction (the outbox store half is milestone 3's, with the journal),
§39m-1's kind rank and §39m-5's absence-door deferral (both carry their rejected alternatives).
