# The capture tool — design (2026-07-29)

**Status:** design only. **No code written in this session.** Milestone 2 step 1 (`HANDOFF.md`
START HERE, session 28). Branch `feat/milestone-1-foundation`; verify state with `git status --short`
and `grep -rn "SPEC EDIT OWED" internal/ tools/` rather than believing this line.
**Doc set:** implements `build-spec.md` §12.2's *"self-capture both funding streams from day one"*
under §8's fidelity contract, on §3.5's capture-lane rules. Reasoning that produced the ordering is
`2026-07-29-milestone-2-capture-decisions.md` §31 and is **not** repeated here — this document starts
where that one stops.

Deviations from the spec are marked `[SPEC EDIT OWED]` at the point they are taken in code, and §12
below lists the ones this design already knows it will take, so the close has a roster rather than a
memory.

---

## §32a — the two decisions the user took, opening this session

### Decision 1: self-capture covers funding **and books**, resolving §31d

**Taken: funding on both venues, plus spot and perp books for BTC, plus one diff/delta book stream on
Binance perp as evidence.**

§31d framed this as "does §8 intend Tardis for books". The framing sharpened while answering it, and the
sharpened version is what the decision rests on: **§8's ≥3-month scoring sample comes from Tardis
either way.** Self-capture starting today cannot produce three months of history for three months, and
§8's data bullet says so itself — Tardis for the streams, *"self-capture both streams from day one for
parity + recent days."* So the question is not "where does the sample come from". It is:

> **What does the parity claim cover?**

§3.5 defines parity as replaying **our own persisted log**. If the simulator's books come only from
Tardis, then the book path — snapshot+delta reconstruction, sequence-gap detection, resnapshot, the
`marketdata` machinery that took a whole slice to build — is never replayed from a log we wrote, and it
is **the harder of the two paths by a wide margin**. The funding path is a scalar per second. Capturing
only funding would mean the parity claim is proven on the easy half and asserted on the hard one.

**Rejected: funding only (the literal §12.2 reading).** Cheapest, and the funding signal is what the
kill gate scores. Rejected because §8's fill model — the thing that makes the simulator's number
credible rather than optimistic — requires *"time-synchronized spot & perp books"* and models each fill
by *"walking the actual order quantity through book depth"*, and Profiles §3 closes the other door:
**spot has no mark and no funding at all**, so the spot leg is priced from its own book. Funding-only
capture cannot support that, and the discovery lands at step 5 when the missing months are
unrecoverable.

**Rejected: books via diff/delta streams as the primary.** It is the shape §3.4's reconstruction
discipline assumes, and it would exercise `marketdata` end to end. Rejected as the *primary* on two
counts: it needs the REST init-snapshot join captured and correctly interleaved, and one lost delta
makes the whole window non-replayable rather than one message. The second is the decisive one for a
capture tool, because losslessness is the property we are buying (§31c).

**This is plausibly a core §12.2 edit** — §12.2 says "both funding streams" and this is wider. Marked
in §12 below.

### Decision 1b — the correction the venue facts forced

**Bybit has no self-contained snapshot book feed in our verified facts.** Profiles §1 records
`orderbook.50` as *snapshot+delta, delete-on-size-0, reset-on-snapshot*, and §3 records
`orderbook.{1|50|200}` for spot. Only **Binance** offers a partial-book stream
(`@depth5/10/20@100/250/500ms`) where every message is a standalone snapshot.

So "snapshot feeds" is a Binance property, not a design-wide one, and three things follow that would
otherwise be silently assumed:

1. **The Bybit half is reconstructed at read time regardless.** The parity claim over reconstruction is
   therefore exercised — on Bybit — which removes most of what the rejected diff-primary option was
   buying, at Binance's lower risk.
2. **A dropped Bybit book message desyncs the window** until the venue's next snapshot. The writer's
   overflow policy (§32f) is load-bearing for Bybit specifically, and the gap marker must be readable as
   *"this window is non-replayable until the next snapshot record"*, not as a lost message.
3. **Standing debt #1 — the two venues' depth update-ID shapes — stops being background debt.**
   `engine/venue`'s `BookDelta` span design already cites an asymmetry between the two feeds that
   Profiles does not record. Reconstructing Bybit from our own archive requires that fact. It is now
   **blocking for the Bybit book source**, exactly as §31d's standing-debts note predicted it might be.

The Binance diff/evidence stream (§32c, source 7) is what turns the Binance half of that debt from a
verification we owe into a measurement we hold.

### Decision 2: capture runs on the dev machine for now

**Taken: start on the laptop.** The cost is stated rather than absorbed: sleep, reboots, and network
changes each become a recorded gap, and the parity window is the one thing that cannot be re-created.

Two consequences the design carries because of it:

- **`capture verify` is not a later nicety.** It ships with the first capture (§32k, slice C), because
  on this host gaps are *expected*, and an archive whose gaps are expected is worthless unless they are
  **measured**. The verdict line is: *a recorded gap is not an error; an unexplained silence is.*
- **The off-host copy still matters, and arguably more.** A laptop disk holding the only copy of an
  unrecoverable-by-construction archive is the single failure that cannot be fixed afterwards. Each
  closed file gets a digest (§32e) so the copy is verifiable rather than assumed.

---

## §32b — where it lives, and the fence against §31e

**The spec already chose the home.** §11's repo layout names `cmd/arblab/` as the *"ops CLI: capture,
replay, reconcile, key checks"*. Nothing has to be invented, and the placement is itself an argument:
capture is an **ops tool**, not a mode of the engine. That is §31e's requirement expressed as a
directory rather than as a discipline.

| Path | Holds | Why here |
|---|---|---|
| `cmd/arblab/` | the ops CLI: `capture run`, `capture verify`, `capture cat` | §11's layout, unchanged |
| `internal/app/capture/` | the composition root — config → sources → writers → observability | §11: `app/` is composition roots, one per binary |
| `internal/engine/capture/` | frame codec, writer, archive layout, verify, record kinds | `engine/` is technical subsystems with no DDD layering; capture is one |
| `internal/engine/venue/transport/` | raw WS/REST clients yielding **unparsed frames** | §11 puts "raw Binance/Bybit clients" in `engine/venue/` |

**The transport is shared with the future live path; the dispatcher never is.** This is the precise form
of §31e's fence, and getting it precise matters because the loose version ("keep capture separate")
would forbid the one sharing that makes capture *worth* doing. Capturing through a different socket
stack than the engine uses would make our archive a record of a pipeline we do not run. So:

- **Shared:** the raw connection, subscription, reconnect and framing — the bytes and when they arrived.
- **Never shared:** arbitration. Capture assigns no `AcceptOrdinal`, no `IngressSeq`, and makes no
  decision about the order in which two different sources are consumed (§32d).

**Enforced by an import-graph test, not by a seventh analyzer.** `internal/engine/capture` and
`internal/app/capture` get a test that runs `go list -deps` over themselves and fails on any dependency
under a forbidden root — `internal/marketdata`, `internal/oms`, `internal/strategy`, and (when it
exists) the dispatcher package.

**Rejected: a `nocapturedrift` §3.6 analyzer.** It would be the house style, and §3.6's own argument
("a linter you cannot unit-test cannot be *shown* to bite") is real. Rejected on proportionality: a new
gate is a spec edit to §3.6's ten rules, and the hazard here is a **single import edge in two packages**,
which a list test in those packages' own suites covers completely. If capture ever spreads past two
packages, revisit. The test carries a **positive control** — assert an allowed import (`platform/clock`)
is allowed — because a forbid-everything predicate reads as a perfect score.

---

## §32c — the capture set, concretely

Seven sources. BTC only, per §12.2's *"one venue / one symbol (BTC)"* scope widened to both venues
because §12.2 says "both funding streams".

**Every topic below is VERIFIED as of 2026-07-29** — the ⚠ markers are gone, and the URL each one needs
is now recorded in `venue-profiles.md` §5. The `Connect to` column is the fact that was missing, not the
topic string.

| # | Venue | Product | Stream | Venue topic | Connect to (Profiles §5) | Role |
|---|---|---|---|---|---|---|
| 1 | Binance | perp | funding | `btcusdt@markPrice@1s` | `wss://fstream.binance.com/market/ws/…` | the signal |
| 2 | Bybit | perp | funding | `tickers.BTCUSDT` | `wss://stream.bybit.com/v5/public/linear` | the signal |
| 3 | Binance | perp | book | `btcusdt@depth20@100ms` (partial) | `wss://fstream.binance.com/public/ws/…` | fill model |
| 4 | Binance | spot | book | `btcusdt@depth20@100ms` (partial) | `wss://stream.binance.com:9443/ws/…` | fill model + spot pricing |
| 5 | Bybit | perp | book | `orderbook.50.BTCUSDT` | `wss://stream.bybit.com/v5/public/linear` | fill model |
| 6 | Bybit | spot | book | `orderbook.50.BTCUSDT` | `wss://stream.bybit.com/v5/public/spot` | fill model + spot pricing |
| 7 | Binance | perp | book | `btcusdt@depth@100ms` (diff) | `wss://fstream.binance.com/public/ws/…` | **evidence** for standing debt #1 |
| R | Binance | perp | rest | `GET /fapi/v1/fundingInfo` | `https://fapi.binance.com` — **public**, weight 0 | the overlay, §32h |

**Three sources sit on three different hosts with three different path disciplines**, and that was
invisible while the topics were the only thing written down: Binance perp funding takes the **`/market`**
prefix, Binance perp book takes **`/public`**, and Binance *spot* book takes **neither** — the spot host
kept its own base on port 9443. A config that carries only a topic string cannot express this.

**Source 7's role has changed, and source 4 has inherited part of it.** Standing debt #1 is now cleared
from documentation (Profiles §5), so stream 7 no longer *measures* the Binance update-ID shape — it
confirms it live. The open question it should now answer is **source 4's**: Binance **spot** partial depth
carries `lastUpdateId` and nothing else — no span, no `pu`, no event time — so source 4 is the one feed in
this set that **cannot be gap-checked from its own wire bytes**, and capture's `CaptureSeq` is its only
ordering evidence. That is a reconstructability limit on the spot leg's fill model, and it is a fact
about the venue rather than a defect in the tool.

Sources 3 and 7 are the **same `SourceID`** under §3.5's definition — `(VenueInstrument, StreamKind)`
does not distinguish two wire feeds for one logical stream. That collision is what §32e's key is
designed around, and it is a finding rather than an inconvenience:

> **The capture key is `(SourceID, Topic)`, not `SourceID`.** The archive stores wire bytes, and one
> `SourceID` has more than one wire representation.

The realistic accident is not source 7 at all. It is somebody changing `@depth20@100ms` to
`@depth20@250ms` in config six weeks in. Keyed on `SourceID`, two cadences land in one file with nothing
recording the change, and the reader at step 5 finds a file whose message spacing doubles halfway
through. Keyed on `(SourceID, Topic)`, it is two directories and the change is visible from `ls`.

**This departs from §3.5's literal "append-only with per-`Source` sequence numbers"** — our counters are
per-`(Source, Topic)`. The departure is *finer*, so the drop-detection §3.5 asks for still holds, and it
is marked in §12 rather than absorbed.

---

## §32d — the counter, and the sequence capture deliberately does not mint

**Each `(SourceID, Topic)` gets a dense `CaptureSeq`, assigned at arrival by the reader goroutine.**
Not by the writer. That single placement is what makes losslessness *checkable* rather than asserted:

- The reader stamps `CaptureSeq` the instant a frame comes off the socket, then hands it to a bounded
  queue.
- If the queue is full the frame is **dropped and counted** — never blocked on (§32f).
- A dropped frame therefore leaves a **hole in the sequence**, visible in the file with no other evidence
  required. The `gap` record is corroboration, not the only witness.

Assigned at the writer instead, the sequence would be dense by construction and a drop would be
invisible unless the gap marker itself survived — the exact "silent losslessness" §3.5 spends a clause
forbidding.

**Capture mints no `AcceptOrdinal` and no `IngressSeq`, and this is structural rather than an omission.**
`AcceptOrdinal` is the engine's raw-arrival order across *all* inputs; `IngressSeq` is the replay key
assigned at dispatch. A capture process minting either would be a second authority over an order the
engine has not decided yet, which is precisely how §31e's hazard arrives. It is also unnecessary, and
§3.5 says why in a sentence written before this tool existed:

> *"parity is defined as replaying that persisted dispatch log, not regenerating dispatch order from
> raw capture"*

**Read forward, that sentence explains a property of the capture format: the capture log has no
cross-source order and does not need one.** Replay feeds per-source files into the dispatcher, which
mints both sequences then and persists the dispatch log that parity actually keys on. Each source's file
is totally ordered within itself; across sources, nothing is claimed.

---

## §32e — the archive: format, layout, integrity

### The record frame

```
file  := magic ‖ record*
magic := "ARBCAP" ‖ u8 formatMajor(1) ‖ u8 formatMinor(0)

record := u32 frameLen        // bytes following this field; bounded by maxRecord (8 MiB)
        ‖ u32 crc32c          // over everything after this field
        ‖ u8  kind            // RecordKind, §32g
        ‖ u8  flags           // reserved, MUST be 0
        ‖ u16 reserved        // MUST be 0
        ‖ u64 captureSeq      // dense per (SourceID, Topic); stamped at arrival
        ‖ i64 recvTimeUnixNs  // from the INJECTED clock (§32i)
        ‖ i64 monoNs          // monotonic nanoseconds since process start (§32i)
        ‖ payload             // frameLen - 32 bytes, VERBATIM
```

Big-endian, because the one time anybody reads this file as hex is when it is corrupt, and a big-endian
`captureSeq` is legible in a hexdump.

**Payloads are byte-exact. Nothing is re-encoded on the way in** (§31g rule 1). A parser bug is
redoable against the raw bytes forever; capture is not redoable at all.

**Rejected: NDJSON with the frame as a JSON string.** Greppable, trivially inspectable, and *lossless*
— JSON string escaping round-trips. Rejected because it is not **byte-identical**: a content hash over
the payload stops meaning anything about what the venue sent, and "we can re-parse this forever" quietly
becomes "we can re-parse our re-encoding of this forever". Inspectability is recovered as a **tool**
(`arblab capture cat` renders NDJSON on demand) rather than as the storage format. *The archive is the
fact; readability is a tool over it.*

**Rejected: mimicking Tardis's own CSV/NDJSON layout for interop.** The simulator has to read Tardis's
format regardless, so a second reader exists either way and no cost is saved — while the constraints
(byte-exactness, per-source counters, gap records) are ours and not theirs.

### Directory layout

```
<root>/v1/<venue>/<product>[/<contract>]/<symbol>/<stream>/<topic-slug>/
        2026/07/29/2026-07-29T14.arbcap
        2026/07/29/2026-07-29T14.arbcap.sha256      # written at close
<root>/v1/_manifest/2026-07-29.json                 # per-day roll-up
<root>/v1/.../.writer.lock                          # per-source, §32f
```

`topic-slug` percent-encodes anything outside `[A-Za-z0-9._-]`, because venue topics carry `!`, `@` and
`.`. One function derives the path from `(SourceID, Topic, hour)` and one parses it back, with a
**round-trip property test** under `rapid` — this is §5.1's "exactly one spelling" rule made mechanical
at the filesystem, the same way `SourceID` enforces it at the mailbox.

**Rotation is hourly on UTC `RecvTime`.** A rotation writes a `close` record (carrying the file's last
`CaptureSeq`, record count and rolling SHA-256) then opens the next with an `open` record naming the
previous file's last sequence — so cross-file continuity is checkable without reading either file in
full.

### Integrity

Per-record CRC32C, per-file SHA-256 at close, per-day manifest listing every file with its digest,
record count and gap count — widened by slice F (§38a, 2026-07-30) with `{window, last_seq, closed_at,
source}`: the fields that make an entry checkable without reading the file, and honest about whether it
was written at close or backfilled. This is not defence against an attacker; it is defence against **the
tail of a file after a crash and a bad byte on a laptop disk**, both of which otherwise present as a
successfully-parsed archive with a plausible hole.

**On opening an existing hour file:** scan forward validating CRCs, truncate to the last good record
boundary, and write `gap{cause: torn_tail, bytes: n}`. A torn tail is a *recorded* gap like any other.

---

## §32f — overflow, ownership, and durability

**Bounded queue, never blocking, drops recorded.** §3.5: *"under load it is bounded, and any drop is a
recorded capture gap — never a silent reorder."* Reader → bounded channel (default 4096) → writer.

**Blocking the reader is the rejected alternative and it is worth stating why**, because "never lose
data" sounds strictly better: blocking the socket reader applies TCP back-pressure, the venue eventually
disconnects, and the outcome is a *larger* gap arrived at by a slower path. Dropping loses less and
records what it lost.

**Drop-newest, not drop-oldest.** Either loses a contiguous run; drop-newest keeps the writer's stream
strictly increasing in `CaptureSeq` and needs no queue rewrite. The `gap` record names `{from, to,
count}` from the hole.

**One writer per source, enforced by an exclusive lock.** `flock` on `<sourcedir>/.writer.lock`, holding
pid and host, held for the process lifetime; a second writer refuses to start and says which process
holds it. Without it, two capture processes over one root interleave `CaptureSeq` into something that
looks exactly like a healthy dense log — and this is not hypothetical, because the per-source layout
otherwise makes "run a second process for the new source" an attractive way to add a source without
restarting the others.

**fsync on rotation and every `sync_interval` (default 5 s), plus before exit.** Losing the last few
seconds to a crash is acceptable *because it is recorded* on the next open.

---

## §32g — the record kinds, each named by the failure it prevents

`RecordKind` is a module-declared enum-like, so `exhaustiveswitch` binds: every switch over it must name
every member, and a `default:` does not excuse a missing one.

| Kind | Payload | The failure it prevents |
|---|---|---|
| `Open` | canonical JSON: tool version + git SHA, config revision hash, host, endpoint URL, subscription request bytes, venue's subscribe ack, previous file's last `CaptureSeq` | Provenance. Six weeks later, "which build, which topic, which endpoint" is unanswerable from bytes alone |
| `Data` | the venue frame, verbatim | — |
| `Alive` | empty | **A dead capture and a quiet market are identical from outside** (§31g rule 4). Written **unconditionally** every `alive_interval` (default 60 s), so coverage is *counted* (N ticks per hour) rather than inferred from the absence of data |
| `Gap` | JSON `{cause, from, to, count, detail}`; cause ∈ reconnect \| overflow \| torn_tail \| process_restart \| no_first_data \| clock_step | §3.5: any drop is a **recorded** capture gap, never a silent reorder |
| `Rest` | `u32 metaLen ‖ meta(JSON: method, URL, params, status, RecvTime) ‖ body(verbatim)` | §32h — an **empty** response must be recordable as a positive fact |
| `Close` | JSON `{lastSeq, recordCount, sha256}` | Its **absence** is the signal: a file with no `Close` ended in a crash, distinguishable from a clean stop |
| `ClockStep` | JSON `{backstepCount, backstepTotalNs, systemNow, clampedNow}` | §32i — a wall-clock step that `platform/clock` absorbs by design |

**The organizing rule underneath the table:** *absence must be recorded, because absence is what cannot
be reconstructed later.* `Gap`, `Alive` and the missing `Close` are three instances of one idea, and it
is the same idea as §3.4's silence timer — a feed that simply stops emits no event, so the stop has to
be manufactured by something that was watching.

**Capture never parses a payload, and the boundary is exactly here.** Transport-level discontinuities
(reconnects, socket errors, overflow, torn tails) are known *without* reading a message and get `Gap`
records. **Venue-sequence gaps are not capture's job** — reading `u`/`U` update IDs means parsing, which
is how a capture tool grows a parser and then a normalizer and then §31e's hazard. §3.4 already assigns
sequence-gap detection and resnapshot to `marketdata`, at read time, where it belongs.

---

## §32h — the REST overlay

§31g rule 2: Binance's `GET /fapi/v1/fundingInfo` has the property that **absent means 8 h**. Without
recording the fetch, *"we asked and it was empty"* and *"we never asked"* are indistinguishable in our
own archive — and §4's v0.9.9 rule already refuses a defaulted zero for exactly this reason.

The `Rest` record stores request, status and body verbatim, so an empty array is a recorded positive
fact with a timestamp. Polled hourly onto its own `(SourceID, Topic)` stream, same file discipline.

**Deferred, with the reason stated: the authenticated overlays.** Fee tier (`GET
/fapi/v1/commissionRate`, `GET /v5/account/fee-rate`) and Binance's `leverageBracket` are what step 3's
missing fee seam will need, and capturing them daily from day one would give the kill gate real fee
inputs with dated provenance. They are **not** in this tool, on two grounds:

1. **Credentials.** §7.3 treats key blast radius as a first-class concern, and putting an API key on the
   capture host — a laptop, per decision 2 — to poll an endpoint is a trade this design does not get to
   make silently.
2. **It is not calendar-bound, which is the whole ordering argument (§31a).** Fee tier is a function of
   30-day volume, ours is zero, and §2.4/§12.2 already predict the VIP0 outcome. A dated verification at
   step 3 captures the same fact.

**Which of these endpoints are public and which are authenticated is not recorded in Profiles and is not
asserted here** (§13).

---

## §32i — the clock, and a finding worth its own section

**`RecvTime` comes from the injected `platform/clock.Clock`, not from `time.Now()`.** §3.6 rule 5's
escape (`//notimenow:allow`) exists for *transport deadlines*, and the distinction this design draws is
sharp: **the escape covers deadlines, not stamps.** A socket read deadline is genuinely ambient and never
runs under replay; a `RecvTime` stamped on a captured record is the timestamp §3.4 measures freshness and
cross-leg skew against, and it must be substitutable under a `TestClock` or the writer cannot be tested
at all.

Incidentally, this slice is where `notimenow`'s escape gets its first real use — its own doc comment
predicts *"engine/ transport will need wall-clock scheduling"* and says that if `engine/venue` ships and
no escape is ever used, the prediction was wrong and the mechanism should be deleted. Socket read
deadlines are that use. The prediction survives.

### The finding: `Wall.Now()` absorbs a backwards clock step, by design, and capture is where that is wrong

`internal/platform/clock/wall.go` clamps `Now()` so it never goes backwards, and its comment states the
reasoning honestly:

> *"A backwards step is absorbed rather than reported, because Now has no error to return (§3.5).
> Absorbing it means a clock that briefly stands still, which is the conservative failure: a timer fires
> late rather than a settlement being evaluated twice."*

**That is correct for the engine and wrong for the archive**, and the composition is what makes it
quiet:

1. NTP steps the laptop's clock backwards — routine on the host decision 2 chose.
2. `Wall.Now()` clamps, so a run of records gets **identical** `RecvTime`s. Nothing errors.
3. §3.4's latest-value cache **accepts an equal `RecvTime`** — a rule added deliberately in session 27,
   for `TestClock` parity.
4. So the replay proceeds happily over a squashed time axis, and every freshness and cross-leg-skew
   number computed over that window is wrong with nothing failing.

The archive is unrecoverable, so this cannot be fixed after the fact.

**Chosen: `Wall` gains a pure accessor — `Backsteps() (count uint64, total time.Duration)`** — reporting
what the clamp has already absorbed. No change to `Now()`'s contract, no new dependency (so
`kernelpurity` is untouched), and it is exactly the information the clamp already computes and discards.
Capture polls it per `Alive` tick and writes a `ClockStep` record when it moves.

**Rejected: capture holds its own monotonic reference under `//notimenow:allow`.** Works, and keeps
`platform/` untouched. Rejected because it puts a **second time authority** in the capture tool to
detect a defect in the first one, and because the gap is genuinely in the kernel: *no* caller can
currently learn that the clamp engaged, and the engine has the same blind spot for the same reason.

**Rejected: making `Now()` report the step.** It has no error to return, and changing that changes every
call site in the codebase to fix a problem one caller has.

`[SPEC EDIT OWED]` — §3.5's `Clock` block states the clamp's existence nowhere and its observability
nowhere. Marked in §12.

---

## §32j — configuration

`platform/config`, and the notable thing is **how little needs writing**: hot-reload is per-field
default-**deny**, and for capture that default is simply correct. **No field carries `config:"hot"`.**
Every change is restart-required, a restart is a gap of a few seconds, and the gap is recorded — which
is the honest trade rather than a hot-reload path whose failure mode is a source that silently stops.

Fields: archive root, the source list (`SourceID` + topic + endpoint + role), rotation period, queue
depth, `alive_interval`, `sync_interval`, `first_data_timeout`, REST poll intervals, `maxRecord`.

Observability (`platform/observability`) — per source: records written, bytes written, gap records by
cause, last-record age, queue high-water mark, drops. Labels are `venue`/`product`/`symbol`/`stream`/
`topic`, every one a **closed set declared from config at registration**, so §10's default-deny label
rule is satisfied by declaration and no `Unbounded` reason is needed.

---

## §32k — liveness, and the positive control that makes it evidence

§31g rule 4 is the requirement: a dead capture process and a quiet market are identical from the
outside, and it is discovered three months later at scoring time. Three mechanisms, in-process to
out-of-process:

1. **First-data assertion at subscribe.** After the subscription is acked, if no `Data` record arrives
   within `first_data_timeout`, write `gap{cause: no_first_data}`, alert, and reconnect. **This is what
   kills the Binance trap**: the legacy `/ws/` path was decommissioned 2026-04-23 and **returns nothing
   rather than an error**, so an old code sample yields a process that connects, waits politely, and
   captures an empty file. Under this assertion it fails in seconds, loudly.
2. **Unconditional `Alive` ticks** (§32g), which put the liveness signal **inside the archive**. Any
   interval with no records at all is unexplained, which is a different and worse thing than a recorded
   gap.
3. **`arblab capture verify --root R --from T --to T`**, out of process, reporting per `(source, hour)`:
   record and `Data` counts, `Alive` ticks against expected, `CaptureSeq` continuity within and across
   files, gap records by cause, CRC failures, digest mismatches, and a verdict of
   `replayable` / `gapped` / `absent`.

**Two exit codes, because the distinction is the whole point on a laptop:** `1` = integrity failure
(CRC, digest, duplicate or non-monotonic `CaptureSeq`, evidence of two writers) — the archive is lying;
`2` = coverage below threshold — the archive is honest and thin.

**Amended by §71 (2026-08-05, task #153):** the continuity half of item 3 splits one of its
findings. A `prev_last_seq` of zero beside a predecessor was reported as `restart_boundary`
whatever the predecessor was, which on a REST-overlay source — no `Data` records, so `lastSeq`
never leaves zero (§32h, slice E) — fired once per rotation forever and made every such hour
`gapped`. The seam where the predecessor also ended at zero and both Open records name the same
`(host, pid)` is now `unsequenced_boundary`, coverage, and the one finding kind that does not
flag its window. Read §71 before citing this section on what a boundary means.

**The positive control, per §31g rule 4's own sentence** (*"a monitor that has never reported a problem
has not been shown able to"*): an acceptance test builds a healthy archive and asserts `verify` reports
**clean** — then, separately, kills a stream mid-capture, corrupts one byte, truncates a file mid-record,
and deletes an hour, asserting `verify` names each one specifically. Without the healthy case first, a
checker that objects to everything scores perfectly.

---

## §32l — the disciplines that will bite, named before they do

**Gates.** `nofloat` — capture handles bytes and integers, nothing to declare. `notimenow` — the
transport escape, deadlines only (§32i). `kernelpurity` — capture is not `platform/`. `hexboundary` —
`engine/` has no `domain/`/`application/` layering, so it does not apply. `exhaustiveswitch` — binds on
`RecordKind` and on gap `Cause`. `singlewriter` — capture must never import `oms`, which the
import-graph test (§32b) already covers more strictly.

**The fixture-collapse checklist, run *before* the mutations** (nine occurrences, so this is a checklist
item, not a war story). Candidates found by looking for the parameter pinned to the value that collapses
a distinction:

| Fixture | Collapses | Vary it to |
|---|---|---|
| One source in a writer test | per-source counters vs one global counter | **≥ 2 sources**, asserting each has its own dense sequence |
| One record per rotation window | "rotate on the hour" vs "rotate per record" | several records per hour, and an hour with none |
| A drop of exactly 1 | "hole" vs "off-by-one" | drops of 1 and ≥ 2, and a drop at the **first** record of a file, where "hole" and "file starts mid-sequence" are the same reading |
| Truncation mid-payload only | torn tail vs clean end | truncate mid-header, mid-payload, **and exactly on a boundary** — the last is where truncation and cleanliness collapse |
| `alive_interval` == rotation period | "tick" vs "rotation" | make them coprime |
| A clock step smaller than record spacing | detected vs undetected | a step larger *and* smaller than the spacing |
| Two writers where both are refused | "second refused" vs "both refused" | assert **which** holds the lock and that the first keeps writing |

**Mutation targets** (`tools/mutate.sh <file> <line> <sed> <desc>`), each aimed at a claim that would
otherwise be asserted only in prose: move `CaptureSeq` assignment from reader to writer; skip the CRC
check on read; drop the `gap` write on reconnect; make the lock non-exclusive; make `Alive` conditional
on idleness. Note that `mutate.sh` runs **without `-race`**, so no lock claim is reportable by it — the
writer's concurrency claims need `go test -race` and their own bounded assertions.

**Bounded channel reads.** Capture is channel-heavy and a mutation that breaks a feed makes a test
**block** rather than fail — `TIMEOUT!!`, a claim never checked while looking covered. Every test
reading from a channel a mutation could stop feeding gets its own deadline. `clock` and `boot` both
found this; this package has more channels than either.

**An oracle for the frame codec.** Real defects here have been found by two implementations disagreeing
(`fixedpoint` ↔ `math/big`, `ids` ↔ its base32 oracle). A framing codec is that class exactly: a
`rapid` round-trip property against an independently-written reader that walks the bytes by hand,
plus a bounded fuzz target over arbitrary bytes asserting the reader never panics and never reports a
record the writer did not write.

---

## §32m — build order

Ordered so the **calendar-bound** thing starts as early as it safely can (§31a), not so the architecture
is completed first.

| Slice | Contents | Done when |
|---|---|---|
| **A** | frame codec + oracle + round-trip property + fuzz. No I/O | mutants on the codec are caught; fuzz corpus committed |
| **B** | writer: files, hourly rotation, exclusive lock, torn-tail recovery, gap records, `Alive` ticks — driven by a **fake** source under `TestClock` | the fixture-collapse table above is satisfied; `-race` clean |
| **C** | raw WS transport, subscribe + ack + first-data assertion, reconnect-with-gap; `arblab capture run`; config; observability; **minimal `capture verify`** (CRC + sequence continuity + tick coverage) with its positive control | **⟵ funding capture for BTC on both venues STARTS RUNNING HERE.** This is the calendar moment |
| **D** | measure one day of funding on disk (**measure, do not estimate** — §31d); then add sources 3–6 and the diff evidence stream 7 | book capture running; one day's real disk figure recorded |
| **E** | the REST overlay (§32h) | `fundingInfo` recorded hourly, empty responses included |
| **F** | full `verify` (manifests, digests, coverage report), `capture cat`, off-host sync of closed files verified by digest | an off-host copy exists and is verified |

`verify` is deliberately **not** last. Slice C ships the minimal version, because a capture running
unverified for a week is the same failure this whole tool exists to prevent, one level up.

### §32m-D — the one-day disk figure (measured 2026-07-31, closing slice D's done-when)

Measured at 11:00 UTC 2026-07-31, capture live since ~09:53 UTC 2026-07-30 (PID 74192), all eight
streams. **`du -sh archive/` = 3.0G** (3,103,316 KB, 351 files) for the first elapsed calendar day.
The full-day verify over exactly that day (`-from 2026-07-30T09:53:00Z -to 2026-07-31T09:53:00Z`)
reports **0 integrity, 353 coverage — verdict COVERAGE**, so the figure is measured over an archive
that testifies to its own honesty.

Per-stream, largest first:

| Stream | One day |
|---|---|
| binance perp book `btcusdt@depth@100ms` (the diff evidence stream) | 890M |
| binance spot book `btcusdt@depth20@100ms` | 612M |
| bybit perp book `orderbook.50.BTCUSDT` | 531M |
| bybit spot book `orderbook.50.BTCUSDT` | 442M |
| binance perp book `btcusdt@depth20@100ms` | 441M |
| bybit perp funding `tickers.BTCUSDT` | 101M |
| binance funding `btcusdt@markPrice@1s` | 11M |
| binance `fundingInfo` REST overlay | 2.2M |
| `_manifest` | 68K |

Two readings, both worth the row. **§31d's actual question — one day of *funding* on disk — answers
at ~114M/day** (tickers + markPrice + fundingInfo); the other ~2.9G is books, and within books the
single diff evidence stream is the largest line item at 890M. And **the 3.0G is an under-measure of
a healthy day**: the day it covers contains the honest ~23:00–04:15 UTC all-source absent window
(laptop lid — the machine is the §38f problem restated) plus reconnect-churn hours at 12–42/60
alive through the 04:00–09:00 morning. A naive scale-up to full wall coverage lands ~3.6–3.8G/day —
**derived, not measured**, labeled so; the measured number is the 3.0G above. At either figure, a
laptop disk carries weeks, not months, before the §38f off-host decision stops being deferrable on
capacity grounds alone.

Stated as a roster because §31e's hazard is *growth*, and growth is easiest to refuse against a list:

- **No parsing.** Not of book updates, not of funding messages. Bytes in, bytes out.
- **No normalization.** It never constructs a `venue.Funding` or a `venue.BookSnapshot`.
- **No cross-source ordering**, no `AcceptOrdinal`, no `IngressSeq`, no coalescing.
- **No dropping without a record**, ever.
- **No credentials** (§32h).
- **No engine imports** — `marketdata`, `oms`, `strategy`, and the dispatcher when it exists (§32b).

---

## §12 — the spec edits this design already knows it owes

Marked in code as they are taken; landed before the slice closes; the close gate is
`grep -rn "SPEC EDIT OWED" internal/ tools/`. Recorded here so the close has a roster and not a memory
— and noting the gate's own limits: it cannot see an **unmarked** claim, so the reading pass sits beside
it, and it cannot see a rule the code silently fails to meet.

| # | Doc | Edit |
|---|---|---|
| 1 | core §12.2 | self-capture is **funding + books** (+ one evidence stream), not "both funding streams" — §32a |
| 2 | core §3.5 | the capture lane's counters are per-**`(Source, Topic)`**, because one `SourceID` has more than one wire representation — §32c |
| 3 | core §3.5 | `Clock` absorbs a backwards step and **no caller can learn it happened**; `Wall` gains `Backsteps()` and the capture lane records it — §32i |
| 4 | core §8 | (probable) the fidelity contract's data line, if §12.2's widening needs restating where the fill model is defined |
| 5 | core §3.6 rule 8 | **a SECOND documented integer-enum exception**: `capture.RecordKind` crosses a persistence boundary, which rule 8 says takes the `type X string` shape, and is `uint8` anyway — §33a. Found by building slice A, not by this design |

**ALL FIVE ROWS LANDED — v0.9.13 (2026-07-30, the slice-C close), plus §36a's module rename.** Rows
2, 3 and 5 travelled as code markers (row 3 at `platform/clock.Wall.Backsteps`, row 5 at
`capture.RecordKind` — both slice A, §33d and §33a; row 2 at `capture.StreamKey` — slice B, §34) and
those markers now read `[SPEC EDIT LANDED — v0.9.13]`, alongside the §36a rename marker in the
transport package doc. Rows 1 and 4 never got markers — "not yet marked because no code asserts them
yet" — **and that state expired silently**: the config schema's positive-control test
(`TestRealCaptureConfigLoads`, session 35) asserts §32c's seven-source capture set in `internal/`,
which IS §32a's widening of §12.2, unmarked because at writing time it read as loading a config
rather than asserting a scope. The close's reading pass is what re-derived it; the grep, by
construction, could not — a fourth instance of the gate's stated limit, this time on the roster's own
bookkeeping. Row 4's "(probable)" resolved to YES: §8's data bullet carried the same "both streams"
sentence as §12.2, and one statement updated with one left behind is the v0.9.9 two-statements
failure built by hand. `grep -rn "SPEC EDIT OWED" internal/ tools/` returns **zero**.

---

## §13 — verification debts into `venue-profiles.md`, blocking their sources

Nothing goes into Profiles unverified, and **nothing built may depend on an unverified fact.** Each of
these blocks the source that needs it, not the whole tool.

> **ALL FIVE CLEARED 2026-07-29 (session 32).** The facts are in `venue-profiles.md` **§5**, dated and
> cited; the resolution's own record is §35. **Slice C is unblocked.** One sub-item is deliberately left
> open and is named in item 5. The original statements are kept below because three of them were
> *mis-stated*, and the corrections are the finding.

1. **Binance per-symbol funding topic.** ~~Profiles §1 records `!markPrice@arr@1s` (the all-symbol array).
   The per-symbol form we want is **not recorded**.~~ Blocks source 1. — **CLEARED.**
   `btcusdt@markPrice@1s` confirmed, cadence is **3 s or 1 s only**, and the payload carries two fields
   Profiles had not recorded (`ap`, `st`). Confirmed there is **no interval field**, which is what §32h's
   whole existence rests on.
2. **Binance futures book stream host, path and available cadences** for partial-book depth. ~~Profiles §1
   records the stream family and the `/public` routing but not the pairing.~~ Blocks sources 3, 4, 7. —
   **CLEARED, and the question was under-specified**: it asked for *one* host and there are **three**.
   Futures book is `/public`, futures funding is `/market`, and Binance **spot** book is on
   `stream.binance.com:9443` with **no prefix and different speeds** (1000 ms / 100 ms, not 100/250/500).
3. **Bybit linear public WS URL.** ~~Profiles §1 says "single WS host" without the URL; §3 records the
   *spot* URL only.~~ Blocks sources 5, 6. — **CLEARED:** `wss://stream.bybit.com/v5/public/linear`. The
   *"single WS host"* phrasing in §1 was misleading rather than wrong — the host is single, the **path**
   is per-category, and that is the part a connector needs.
4. **The two venues' depth update-ID shapes** — standing debt #1, owed since session 20. **Now blocking
   for Bybit** (§32a decision 1b), because reconstructing our own archive requires it. ~~The Binance half
   is what evidence stream 7 measures.~~ — **CLEARED, and the debt named the wrong count.** There are
   **three** shapes, split by product and stream type rather than by venue (Profiles §5's table). The
   consequence that matters is source **4**, not source 7: Binance spot partial depth has **no span at
   all**.
5. **Public vs authenticated** for each REST overlay endpoint (§32h). — **CLEARED for everything the tool
   uses.** `GET /fapi/v1/fundingInfo` is **public** (doc-unmarked + live 200 unauthenticated), weight 0,
   sharing 500/5 min/IP with `fundingRate`. Of the deferred overlays, `leverageBracket` is `USER_DATA`
   and Bybit's `fee-rate` is signed — which **corroborates the deferral's first ground** rather than
   merely coexisting with it. **`GET /fapi/v1/commissionRate` is NOT verified and is not asserted**; it is
   deferred, so nothing depends on it, and it is left visible rather than closed with its neighbours.

---

## RESUME HERE — **SUPERSEDED by session 30.** Historical from here down

**The live statement is `docs/superpowers/specs/2026-07-29-capture-slice-a-frame-codec.md`
`## RESUME HERE`, and `HANDOFF.md` `## START HERE (session 30)`.** **Slice A is BUILT** — the frame
codec, its oracle, the `rapid` properties and `FuzzDecodeRecord`, plus §32i's `Wall.Backsteps()`. The
next work is **slice B, the writer**, not slice A.

§32a–n above stay live as the *design* and are what slices B–F implement; §12's roster gained row 5
and §33c raises a format question against §32e that is still open. Only this block is stale — and it
is the second one in this document set to go stale in two sessions, which is why supersession is
marked at the block rather than left for a reader to infer.

**Two user decisions taken (session 29, still live):** self-capture covers **funding + books** (Binance
via partial-book snapshots, Bybit snapshot+delta because it has no alternative, plus one Binance diff
stream as evidence), and capture **runs on the dev machine for now** — which makes `capture verify`
ship with the first capture rather than after it.

**§13's verifications are still NOT STARTED.** They block slice C, and slice C is where the calendar
starts running.

**Three things not to re-derive:**
- The funding rate is **consumed, never derived** (§31b, categorical).
- Capture is optimized for **losslessness and provenance, never latency** (§31c).
- Capture makes **no arbitration decision** and mints **no `AcceptOrdinal`/`IngressSeq`** (§31e, §32d) —
  and §3.5's "parity replays the persisted dispatch log, not raw capture" is what makes that safe.
