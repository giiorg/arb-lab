# §34 — the capture tool, slice B: the writer (2026-07-29)

**Status:** built. `make ci` green, all six gates clean, `-race` clean.
Implements §32m slice B of `2026-07-29-capture-tool-design.md`, over slice A's
codec (`§33`). Requirements are in §32; this record holds the decisions the build
forced and the findings it produced. Reasoning already in §31, §32 and §33 is
cited, not repeated.

**What landed**

| Path | Holds |
|---|---|
| `internal/engine/capture/path.go` | `StreamKey`, the archive layout, `ParseFilePath`, the segment codec |
| `internal/engine/capture/gap.go` | `Cause` (6 members + the zero sentinel), `Gap`, `Gap.Validate` |
| `internal/engine/capture/lock.go` + `lock_flock.go` + `lock_nolock.go` | §32f's exclusive per-stream lock, and the refusal where there is none |
| `internal/engine/capture/recover.go` | `scanArchive` — the torn-tail procedure and its sliding window |
| `internal/engine/capture/writer.go` | `TimeSource`, `WriterConfig`, `Writer`: rotation, Alive, gaps, digests |
| `internal/engine/capture/queue.go` | §32f's bounded drop-newest hand-off |
| five `*_internal_test.go` | the collapse table, the rejection tables, the `rapid` round trip, the `-race` partition property |

Verification: **23 mutations, 23 kills** after re-aiming three `NO-OP!!`/`NOBUILD!!`
and diagnosing **three `SURVIVED`** (§34e). The `exhaustiveswitch` binding on
`Cause` was probed by hand rather than assumed — removing one member makes the
gate name it, and restoring it makes the gate silent.

---

## §34a — `Cause` is a string enum, and that is rule 8 UNMODIFIED

`RecordKind` took a documented exception to §3.6 rule 8 (§33a) because it is a
fixed-width byte in the frame header, where a variable-length token means a
second length to parse before the payload's length is known. `Cause` takes no
exception at all: it lives in the gap record's **JSON payload**, where nothing is
fixed-width and the self-describing token costs a handful of bytes on a record
that is rare by construction.

**The header/payload boundary is the whole distinction**, and it is worth stating
because from a distance the two decisions look contradictory — one enum-like in
this package is an integer and the next is a string, in the same format, for
reasons that are not about taste. No new spec debt: rule 8 applies as written.

---

## §34b — three decisions the writer forced, none of them in §32

### 1. Writer-minted records echo the last Data `CaptureSeq`

§32d makes `CaptureSeq` the **reader's** counter, dense over captured frames, so
a hole in it means a dropped frame with no other evidence required. The build
had to answer what an `Alive` tick or a `Close` record carries, and §32 does not
say.

**Taken: the last Data sequence, echoed.** A writer that minted its own numbers
would be a second authority over that counter, and every `Alive` tick would fill
a hole a drop was supposed to leave. Echoing keeps the counter's meaning intact
and still locates the record: *this happened after frame N*. It also forces
`CaptureSeq` to be **1-based**, because zero is what an `Open` record's
`prev_last_seq` means by "no predecessor" — refused at the writer
(`ErrSeqNotAscending`), which catches the realistic version, a reader that reset
its counter mid-run.

### 2. Two digests, covering different things, on purpose

The `Close` record's `sha256` covers every byte **before that record** — it must,
since a digest cannot cover the record carrying it. The `.sha256` sidecar covers
the **complete file**. They differ by exactly one frame, and each is checkable on
its own: the sidecar by `sha256sum -c` with no code of ours involved, the field
by a reader that stops where the `Close` record starts.

A test asserting only one of them would pass against an implementation that
wrote the same digest twice, so the assertion is that they differ **and** that
each matches what it claims to cover.

### 3. `ClockStep` carries three of §32g's four fields

§32g names `{backstepCount, backstepTotalNs, systemNow, clampedNow}`. `systemNow`
is **not observable** through the accessor §32i chose: `Backsteps()` reports what
the clamp absorbed, and reading the system clock to fill the field would install
the second time authority §32i explicitly rejected — to decorate a record about
the first one. Three fields, and the reason is in the payload type's doc comment.

Two smaller reconciliations, recorded so they are not rediscovered as bugs: the
gap payload is the **union** of §32e's `bytes` and §32g's five fields, because
the two describe different causes (a sequence hole has a range and no byte count;
a torn tail the reverse); and every payload uses **snake_case**, where §32g mixes
`lastSeq` with `cause`. Both are design-doc edits, not core-spec debt.

---

## §34c — the `Mono` contract, stated because a signature cannot state it

`TimeSource` is capture's own port rather than `clock.Clock`, and each method has
its own reason: `Now` is §3.6 rule 5's injected stamp, `Backsteps` is a
`*clock.Wall` accessor that is not on the `Clock` interface at all (§32i), and
`Mono` carries a requirement no signature expresses:

> **`Mono` must be a reading that keeps advancing across a wall-clock step.**

Deriving it from `Now` satisfies the interface and **defeats the field**. `Wall.Now`
is clamped monotonic, so during a backstep a derived `Mono` stands still — which
is precisely the window `Mono` exists to measure, and a record's `Mono` would be
redundant with its `RecvTime` exactly when `RecvTime` is untrustworthy. The test
double models the three readings separately for this reason; a double that
derived `Mono` from `Now` would pass every test in the package while hiding the
defect §32i is about.

**The live adapter is deliberately not in this slice.** It cannot live in this
package (`engine/`, so `notimenow` forbids the ambient read), and its two
candidate homes — a `Mono` method on `platform/clock.Wall`, or an adapter in the
`internal/app/capture` composition root — are a slice C decision, where the
process that needs one first exists. **Open for slice C**, and it must not be
answered with `Now() - start`.

---

## §34d — the finding: a topic of `..` climbed out of the archive tree

**Found by the `rapid` round-trip property on its fourth case**, not by review,
and not by any hand-written fixture.

§32e's unencoded set is `[A-Za-z0-9._-]`, so a topic of `"."` encoded to `"."` —
and `filepath.Join` **cleans that away**. The consequences ran in both
directions:

| Topic | Path produced | Consequence |
|---|---|---|
| `.` | the parent's own directory | two distinct streams share one tree, interleaving their `CaptureSeq`s |
| `..` | one level **above** the archive root | the writer creates and locks a directory outside the tree it was given |

The encoder, the decoder and every fixture agreed that nothing had happened,
because the round trip is only checked through paths and both ends of the loop
had already lost the segment.

**Taken: escape a LEADING dot, and refuse a literal leading dot on the way back.**
`.` stays unreserved everywhere else, so `tickers.BTCUSDT` is unchanged and
§32c's "visible from `ls`" argument survives; only `.`, `..` and `.hidden` move.
The decoder gains the exactly-mirrored carve-out — `%2E` is legal at position 0
and nowhere else — so there is still one spelling per stream. It buys one thing
free: no stream directory can collide with a dotfile, so `.writer.lock` and the
archive tree can never be confused however a topic is spelled.

**The reusable part is about where the check lives.** The path is the ONLY record
of which stream a file belongs to, because a frame carries no `SourceID`. A
mapping that is not injective there is not a cosmetic problem, and the property
that found it is the same discipline `fixedpoint` and `ids` established: an
independently written inverse, checked over generated input, finds what review
does not.

The `Open` record now also names its stream **structurally** — venue, product,
contract, symbol, stream, topic — which is an addition to §32g's list. One `mv`
by a sync script otherwise leaves a file that cannot say what it is. Six fields
per file, once per hour.

---

## §34e — the three surviving mutants, and why each looked equivalent

All three had the shape §33e's second finding warns about in reverse: the mutant
looked equivalent because **every fixture sat on the side of a threshold where
the two behaviours agree**.

### 1. The torn tail was overwritten rather than truncated

Deleting the `Truncate` call changed nothing. The recovery seeks to the last good
boundary and writes an `Open` record and a gap record there, and those are
**longer** than the torn tails the fixtures produced — every fixture cut into the
small `Close` record — so the stale bytes were entirely overwritten and the file
came out identical.

They stop being equivalent when the torn tail is longer than what the new run
writes before it stops, which is the realistic case: a torn tail is a partially
written record, and records carry venue book updates. Then the file ends with
kilobytes of stale bytes past the last real record, and the **next** open reads
them as corruption — `ErrArchiveCorrupt` on a file whose recovery had already
been recorded as clean. Fixed by a fixture that cuts into a 4 KiB **Data**
record.

### 2. The scanner never had to grow

Removing the growth ceiling changed nothing, because no fixture wrote a record
larger than the 64 KiB initial buffer. Without growth the scanner buffers 64 KiB,
cannot decode a record needing 200, and reports a **healthy file as a torn tail**
— after which the recovery truncates it, discarding every record after the first
oversized one. A 200 KiB record is not exotic: a venue book snapshot passes 64
KiB routinely, and `MaxRecord` is 8 MiB precisely because records that large are
expected.

### 3. The half-populated gap range was checked by a second route

`hasRangeFields` reduced to `Count != 0` survived, because every half-populated
fixture used `CauseOverflow` — where `hasRange()` refuses it **anyway**. The
fixtures showed that something objected rather than that this predicate did,
which is §3.6's `singlewriter` probe note one abstraction down. The isolating
fixture is a cause that requires **no** range carrying half of one, where
`hasRangeFields` is the only thing between a caller who forgot `Count` and a gap
record describing a hole with no size.

### The checklist item this adds

**The three fixtures were all on the cheap side of a size threshold.** A torn
tail smaller than the recovery's own writes; a record smaller than the buffer; a
cause that fails two checks instead of one. That is a new entry beside the
collapse checklist's usual "pinned to the value that collapses the distinction":
*find the fixture that is too small for the mechanism under test to matter, and
make one that is too big.*

---

## §34f — the fixture-collapse checklist, run before the mutations

§32l's table for slice B was satisfied as written — two streams, several records
per window and a window with none, drops of one and of several and one at the
first record, truncation at four offset classes, coprime intervals, clock steps
at both magnitudes, and a lock test that asserts **which** writer holds it and
that the holder keeps writing. What the checklist added beyond it:

| Fixture | Collapsed | Varied to |
|---|---|---|
| Clean symbols only | "encodes the topic" vs "encodes every segment" | a `/` in the SYMBOL, which `ids` permits — it refuses whitespace and nothing else |
| Perpetuals only | the contract path segment present vs absent | both product families, counted, because `ParseFilePath` branches on it |
| Hourly rotation only | the three file-name precisions | 1 h, 4 h, 15 min, 10 s |
| `%` mid-string | `i > 0` vs `i >= 0` | first, middle and last position separately |
| Redundant escapes mid-string | the `%2E` carve-out vs a blanket exemption | `%2E` at position 0 accepted, elsewhere refused |
| Minting before any data | "echoes the last sequence" vs "always writes zero" | mint after three Data records |
| One file per test | `record_count` per file vs per writer lifetime | three rotations, each file's count checked |
| Every file in one read | "read until exhausted" vs "decode one read" | a 400 KiB file with records straddling the boundary |
| Ticks never on a boundary | `<=` vs `<` on `aliveDue` | a fixture exactly on the tick, and one a nanosecond short |
| A boundary truncation with a `Close` | "no torn tail on a boundary" vs "no torn tail when a Close is present" | both clean cases, asserted on two axes |
| A `rapid` property that skips invalid draws | a generator one change away from skipping everything | generators constrained so nothing skips, plus a branch-coverage count |

The tick row is the `<=`-versus-`<` shape for the **sixth** time in this repo
(`ratelimit` twice, `outbox`, `observability`, `exhaustiveswitch`, `boot`), and
it was missed on the checklist's first pass and caught only when the mutation
plan asked what would distinguish the two. **The checklist finds the collapse;
writing the mutation finds the checklist's own gaps.**

---

## §34g — what slice B deliberately does not have

- **No transport.** No socket, no subscribe, no reconnect. The writer is driven
  by a fake source, which is what §32m asks for.
- **No live `TimeSource`.** §34c — slice C's, with the contract stated.
- **No config or observability wiring.** §32j's fields exist as `WriterConfig`
  and `QueueStats`; binding them to `platform/config` and
  `platform/observability` is slice C, where a process exists to configure.
- **No `verify`.** Slice C ships the minimal version, deliberately not last:
  a capture running unverified for a week is the failure this tool exists to
  prevent, one level up.
- **No manifest.** §32e's `_manifest/<day>.json` is slice F's.
- **No venue facts**, so none of §13's five debts block anything here.

---

## RESUME HERE — end of slice B

**Next is slice C** (§32m), and it is **the calendar moment**: raw WS transport,
subscribe + ack + first-data assertion, reconnect-with-gap, `arblab capture run`,
config, observability, and the minimal `capture verify` with its positive
control. **Funding capture for BTC on both venues starts running there.**

**Slice C is the first slice that is BLOCKED**, and by the item below that has
not moved for three sessions.

1. **§13's five Profiles verifications — still NOT STARTED.** Three are missing
   venue endpoint facts: 1 (Binance per-symbol funding topic), 3/4/7 (Binance
   futures book host, path, cadences), 5/6 (Bybit linear public WS URL), plus
   standing debt #1 and the public-vs-authenticated question for the REST
   overlays. **Nothing goes into Profiles unverified, and nothing built may
   depend on an unverified fact.** This is a research task against primary venue
   documentation and it wants its own scope.
2. **The live `Mono` source (§34c).** A `Mono` method on `platform/clock.Wall`,
   or an adapter in `internal/app/capture`. It must not be `Now() - start`.

**§33c is CLOSED, and not the way it was parked.** The question was whether to
move the checksum ahead of `frameLen` before the format freezes. It would not
have closed the hole: the decoder must bound and check **availability** before it
can checksum anything, and no field order covers an absent span — reordering only
helps when `frameLen` is corrupted *smaller*, which already reports `ErrChecksum`
today. The format stays v1.0 by the user's decision, and §33c's rejected-
alternative analysis is corrected in place.

**Three spec edits are owed and marked** (design roster rows 2, 3 and 5):
`capture.StreamKey` (core §3.5's per-`Source` counters), `platform/clock.Wall.Backsteps`
(core §3.5's `Clock` block) and `capture.RecordKind` (core §3.6 rule 8).
`grep -rn "SPEC EDIT OWED" internal/ tools/` returns exactly those three.
