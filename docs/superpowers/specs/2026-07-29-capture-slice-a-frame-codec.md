# §33 — the capture tool, slice A: the frame codec (2026-07-29)

**Status:** built. `make ci` green, all six gates clean, `-race` clean.
Implements §32m slice A of `2026-07-29-capture-tool-design.md`, which is where the
requirements are; this record holds the decisions the build forced and the two
findings it produced. Reasoning already in §31 and §32 is cited, not repeated.

**What landed**

| Path | Holds |
|---|---|
| `internal/engine/capture/doc.go` | the package's fences — §32n's roster, and why the transport is shared and arbitration never is |
| `internal/engine/capture/record.go` | `RecordKind` (7 members + the zero sentinel), `Record`, `Record.Validate` |
| `internal/engine/capture/errors.go` | eleven sentinels, and the truncated-versus-everything-else note the recovery procedure keys on |
| `internal/engine/capture/codec.go` | `AppendFileHeader` / `ParseFileHeader` / `AppendRecord` / `DecodeRecord` |
| `internal/engine/capture/codec_internal_test.go` | the oracle (bitwise CRC32C, cursor decoder, shift-based encoder), the `rapid` properties, the golden layout, the rejection tables |
| `internal/engine/capture/codec_fuzz_internal_test.go` | `FuzzDecodeRecord`, its four invariants, six negative controls, one positive control |
| `internal/engine/capture/record_test.go` | the declared-set and Stringer guards |
| `internal/engine/capture/imports_test.go` | §32b's import fence, with both controls |
| `internal/platform/clock/wall.go` | `Wall.Backsteps()` — §32i's chosen fix, §33d below |
| `Makefile` | `fuzz-decimal` / `fuzz-capture`, because `-fuzz` takes one target |

Verification: 21 mutations, 21 kills after re-aiming four `NO-OP!!`s and
diagnosing one `SURVIVED` (§33e). 25.5 M fuzz executions, no finds.

---

## §33a — `RecordKind` is an integer enum, and that is a §3.6 rule 8 exception

`[SPEC EDIT OWED]`, marked at the type. Rule 8 makes enum-likes `type X string`
+ constants + `Validate()`, with one documented exception
(`fixedpoint.RoundingMode`), and it forecloses this case explicitly: *"The `type X
string` shape exists for enums crossing a **persistence or wire** boundary, where
a self-describing stored token is worth the width. […] An enum-like that does
cross either boundary follows the rule."* A `RecordKind` is a byte in every frame
of the archive. By the letter, it should be a string.

**Taken: `uint8` + `iota`, as a second documented exception.** Two grounds.

The first is mechanical. §32e's frame is a **fixed 32-byte header** and
`payload := frameLen - 32` is only true if every header field has a fixed width.
A variable-length token means a second length to parse before the payload's
length is known, in the one part of the format that has to stay decodable from a
corrupt file.

The second is that rule 8's *rationale* is already answered here, by the
mechanism §32e chose one level up. Self-description was worth the width in a
database column because the column is the only artefact. In this archive the
storage format is deliberately **not** the readable one — §32e's own words when
it rejected NDJSON, *"the archive is the fact; readability is a tool over it"* —
and `arblab capture cat` is where a kind becomes a name. Spending eight bytes per
record to duplicate what a tool prints is width bought for nothing.

**What the integer shape buys instead is rule 8's other half, and here it earns
more than it does for `events.Lane`.** `Lane` takes the exception because a
routing argument's zero value should be an unusable sentinel. `RecordKind` is
read back **off a disk**, so `RecordKindUnspecified = 0` means a zero-filled
region — a sparse hole, a page the filesystem allocated before the process died —
**cannot decode as a valid record**. Had `Data` been zero, the most common
corruption pattern on disk would parse cleanly as an empty data record.

**Rejected: `type RecordKind string` with an explicit string↔u8 table for the
wire.** It satisfies rule 8's letter while defeating its reason — the *stored*
token is still an opaque byte — and it adds a mapping that can drift from the
constant block, which is a new failure mode bought to satisfy a rule whose
purpose the format already serves differently.

---

## §33b — one `Validate` on both sides, which makes the format canonical

`Record.Validate` is called by `AppendRecord` **and** by `DecodeRecord`. That is
one statement of the rules rather than two, and it buys a property worth more
than the tidiness:

> **Every byte string the reader accepts is exactly the one the writer would have
> produced for the record it reports.**

That is §32l's *"never reports a record the writer did not write"* as something
checkable. `FuzzDecodeRecord`'s D3 asserts it directly: re-encode any accepted
record and the bytes must equal the accepted prefix. It holds because flags and
reserved must be zero, the kind must be declared, `frameLen` is determined by the
payload, and the two fields with no wire-level constraint are **total** — every
`int64` of Unix nanoseconds is a valid instant, so `ErrRecvTimeUnset` and
`ErrRecvTimeRange` are reachable from the **encoder only**.

That asymmetry was checked deliberately rather than noticed: session 27's finding
was a sentinel `errors.Is` could name that **nothing could ever return**, and the
close-time check is one grep per sentinel for a `return`. Both of these have one,
in `Validate`, on the encode path. They are listed in the fuzz target's class set
with the asymmetry stated, so a later reader does not go looking for the decode
fixture that cannot exist.

---

## §33c — the finding: `frameLen` sits outside its own integrity cover

§32e orders the frame `frameLen ‖ crc32c ‖ …` with the checksum covering
*"everything after this field"*. So **the length field is the one header field
with no integrity cover**, and a bad byte there has three outcomes:

| Corruption | Outcome |
|---|---|
| implausible (below a header, above `MaxRecord`) | `ErrFrameLen` — caught by the bound |
| plausible, and the frame is **not** the last | the checksum spans the wrong bytes and fails — `ErrChecksum`, correct |
| plausible, and the frame **is** the last | the buffer ends early — `ErrTruncated`, **corruption reported as a torn tail** |

Only the third mis-diagnoses, and it matters because `ErrTruncated` is the one
verdict §32e's recovery acts on **by truncating the file**.

**Taken: keep §32e's field order, bound the length before reading past it, and
test all three outcomes.** The bound check placed *ahead* of the availability
check is what shrinks the window from "any corrupt length" to "a plausible one",
and `TestDecodeRecordFrameLenIsOutsideTheChecksum` asserts each outcome so the
residual is checked behaviour rather than prose.

The residual is genuinely covered from outside by the layering §32e already has:
a cleanly closed file carries a `Close` record and a SHA-256, so corruption is
caught by the digest whatever the codec called it — and a file with **no** `Close`
ended in a crash, which is exactly where a torn tail is the *expected* finding.
What is left is a crashed file whose last record's length field is *also* corrupt.

**Rejected: moving the checksum ahead of the length**, so the cover includes it —
`crc32c ‖ frameLen ‖ …`, single span, hole closed. It is strictly better as a
format and it is **a format-major change**, which is the design's call and not a
slice's to take unannounced. Recorded here because the format freezes when slice C
starts running on the calendar, so this is the last cheap moment to take it: it is
a five-line edit today and an archive migration afterwards. **Open for the user.**

---

### CORRECTION (session 31, before slice B): the rejected alternative does not close the hole

**The paragraph above is wrong about what reordering buys, and the error is worth
keeping rather than editing away, because it is the reason the question sat open
for two sessions looking like a cheap win that was being declined.**

A decoder must **bound `frameLen` and check that the claimed bytes are AVAILABLE**
before it can compute any checksum over them — `codec.go`'s availability check
precedes the CRC by necessity, not by choice. **No field order covers an absent
span.** So for the third row, the one that mis-diagnoses:

| `frameLen` corrupted | Under `frameLen ‖ crc` (today) | Under `crc ‖ frameLen` |
|---|---|---|
| **larger**, last frame | `ErrTruncated` — the mis-diagnosis | `ErrTruncated` — **unchanged** |
| **smaller**, last frame | `ErrChecksum` — correct | `ErrChecksum` — unchanged |

Reordering helps only in the second row, which is already diagnosed correctly.
The five-line edit was never available.

**What would actually close it** is a second, header-only checksum over the fixed
32-byte header including `frameLen` — verifiable **without** the payload, so a
corrupt length reports `ErrChecksum` in both directions. That costs 4 bytes per
record and a format-major bump, against a residual §32e's layering already covers
from outside (a `Close` record and a SHA-256 on a clean file; a torn tail is the
*expected* finding on a file with no `Close`).

**Decided by the user, session 31: keep v1.0.** The format is unchanged and the
"last cheap moment" pressure is gone, because there was no cheap moment to miss.
The residual stands as §33c already describes it, and
`TestDecodeRecordFrameLenIsOutsideTheChecksum` still asserts all three outcomes.

**The reusable part:** a *rejected* alternative recorded with a stated benefit is
an unchecked claim in the same way an untested guard is. This one survived four
reviews and two sessions' handoffs by being written down confidently. A rejection
is worth as much scrutiny as an acceptance, because nobody re-derives it — they
re-read it.

---

## §33d — `Wall.Backsteps()`, and what a backstep counts

Built as §32i specifies, signature unchanged: `Backsteps() (count uint64, total
time.Duration)`, a pure accessor under the existing mutex, no new import so
`kernelpurity` is untouched. `[SPEC EDIT OWED]` at the method — §3.5's `Clock`
block states neither the clamp's existence nor its observability. This is design
roster row 3.

**The decision the build forced: "the clamp engaged" and "the clock stepped
backwards" are different counts.** After a single correction the clamp keeps
engaging on every read until real time catches up, so counting *engagements*
reports one 2.5 s NTP event as four or five backsteps and sums several seconds of
absorbed time that never elapsed. A backstep is therefore a reading **below the
previous reading**, which needs `lastSys` as a field distinct from the high-water
mark `last`. §32g's payload names — `backstepCount`, `backstepTotalNs` — already
assume that reading.

Every backstep necessarily engages the clamp (`last` is the maximum of all
readings, so a reading below the previous one is below the maximum too), which
makes this the stricter and more informative of the two counts.

Both counters are **monotonic and never reset**: a caller wanting a delta
subtracts two observations. Resetting on read would let two readers silently
steal each other's events, and the realistic second reader is a metrics scrape.

---

## §33e — three findings from the verification, in the order they bit

### 1. An invariant set that consulted the oracle only on success

A negative control walked through the first draft of `FuzzDecodeRecord`
untouched: a decoder answering `ErrTruncated` to **every** refusal satisfied all
four invariants. D1 asks only that an error be exactly one sentinel, and
`ErrTruncated` is one; D4 compared against the oracle on the **success path
only**. So the set was blind to precisely the misclassification that costs the
most, since acting on `ErrTruncated` means truncating a file.

**Agreeing that an input is bad is worth little; agreeing WHY is the contract.**
D4 now runs on both paths, comparing by sentinel *class* — `errors.Is` against the
oracle's own error fails, because both sides wrap their sentinels with the values
that failed and neither wrapper is in the other's chain.

### 2. A guard nothing could trigger — a third verdict beside gap and wrong mutant

`nowLocked` was first written `if !w.lastSys.IsZero() && sys.Before(w.lastSys)`,
and the mutation removing the `IsZero` half **survived**. The repo's rule is that
a survivor is either a test gap or a wrong mutant and telling which is the work;
this was neither. The zero `lastSys` is year 1, so no reading a clock can produce
is before it — the guard was **unreachable**, and `clampMonotonic` two functions
along already relies on the identical property with no special case.

This is session 27's unreachable-sentinel finding wearing different clothes: **a
guard no input can trigger reads as protection while protecting nothing**, and
neither the compiler, coverage, nor a green suite says so. The mutation harness
does — which is an argument for running it over defensive code specifically, where
the survivor's meaning is "delete this" rather than "test this". Deleted.

### 3. The isolating probe, at the byte level

Every semantic-rejection fixture in this package mutates the frame and then
**restores the checksum**. Without that, a fixture that flips the flags byte is
refused by the *checksum*, and the test passes while showing that something
objected rather than that the rule under test did — §3.6's `singlewriter` note,
one abstraction down. The checksum's own coverage
(`TestChecksumCoversEveryHeaderField`) deliberately does not restore it, and
flips a bit in **each field region in turn**, because a fixture that corrupts only
the payload passes against a checksum computed over the payload alone.

An external anchor sits under both: CRC-32C's published check value
`0xE3069283` for `"123456789"`, asserted against the table **and** the oracle.
An oracle cannot catch two implementations wrong the same way, which for a
checksum is the realistic case — a wrong polynomial verifies its own output
perfectly, and the archive's integrity field would detect nothing while every test
passed.

### 4. A `.gitignore` rule that read as enforced and excluded nothing

`.gitignore` has carried `testdata/rapid/` since session 26 with an explicit
rationale — *"unlike a fuzz corpus entry these are not a durable regression suite
… the fail file is a working artifact only."* **A gitignore pattern containing a
mid-string slash is anchored to the repo root**, so it only ever matched a
top-level `testdata/rapid/`, which has never existed. `ids`' five fail files were
therefore tracked from the day the rule excluding them was written, and this
slice's would have been too.

Fixed to `**/testdata/rapid/`; the five untracked. Noted because it is the
session's own findings 2 and 3 one layer out in the *tooling*: a rule that reads
as enforced while doing nothing, and a mechanism whose silence was mistaken for
its consent. `git check-ignore -v <path>` is the one-command check, and it is
cheap enough to run whenever a rule is written.

---

## §33f — the fixture-collapse checklist, run before the mutations

Ran first, per the standing rule, and it changed fixtures for free. What it found,
beyond §32l's table:

| Fixture | Collapsed | Varied to |
|---|---|---|
| Small field values (seq 1, recv 2, mono 3) | u64 vs u32 read; a wrong offset | every byte of every field **distinct and non-zero** — `0x0102030405060708`, `0x1122334455667788` |
| One record per buffer | "advance by the frame" vs "advance by a constant" | ≥ 2 records of **different payload lengths**, walked by the reported consumption |
| Flags and reserved set together | "checks flags" vs "checks reserved" | each set alone, and the reserved field's two bytes separately |
| Bad byte mid-magic | `i > 0` vs `i >= 0` predicates | wrong byte at the **first, middle and last** position |
| Version: major only | "checks major" vs "checks both" | major ahead, major behind, minor ahead — separately |
| `Mono` at −1 and +5 s | `< 0` vs `<= 0` | **`Mono == 0` present as an ACCEPTED case** (the first record of a run) |
| Truncation mid-payload | torn tail vs clean end | mid-length-field, on the length field, mid-header, mid-payload, and **exactly on the boundary** as an accepted case |
| `frameLen` far from its bounds | `<` vs `<=` | 31 / 32 and `MaxRecord` / `MaxRecord+1`, with 32 accepted |
| Non-empty payloads only | `frameLen − 32 == 0` | the `Alive` shape, asserting nil and `[]byte{}` encode **identically** |
| One backstep, one clamped read | "counts steps" vs "counts clamped reads" | a step **followed by a recovery window**, and two steps of **different magnitudes** |
| A `go list` graph assumed non-empty | "no forbidden imports" vs "no imports reported" | assert the graph is non-empty **and** contains `hash/crc32` |

The last one is the vacuity trap in its purest form: a `go list` that silently
returned nothing would satisfy a forbid-everything check perfectly.

---

## §33g — what slice A deliberately does not have

- **No I/O.** `AppendRecord` extends a buffer; the writer is slice B. Append
  rather than Write also serves slice B directly: the writer must know a record's
  exact byte count before committing, to decide whether it crosses a rotation
  boundary.
- **No `Reader`/`Scanner` over a file.** Torn-tail *recovery* is slice B; what
  slice A owes it is the `ErrTruncated`-versus-everything-else distinction, which
  is built and tested.
- **No `capture cat`.** §32e makes readability a tool over the archive, and the
  tool is slice F.
- **No venue facts**, so none of §13's five verification debts block anything
  here — which was the reason this slice went first.

---

## RESUME HERE — **SUPERSEDED by session 31.** Historical from here down

**The live statement is `2026-07-29-capture-slice-b-writer.md` `## RESUME HERE`,
and `HANDOFF.md` `## START HERE (session 31)`.** Slice B is **BUILT**; the next
work is **slice C**, which is blocked on §13's five Profiles verifications.
§33c is **closed** — see the correction above; the format stays v1.0.

Marked at the block rather than left to the next reader, because a stale resume
block has now been the failure twice in this document set (§31's, then §32's),
and both times it pointed a fresh session at work that was already done.

**Historical, end of slice A:**

**Next is slice B** (§32m): the writer — files, hourly rotation on UTC
`RecvTime`, the exclusive `flock`, torn-tail recovery, gap records and `Alive`
ticks, driven by a **fake** source under `TestClock`. Done when §32l's
fixture-collapse table is satisfied and `-race` is clean. Slice B needs no venue
facts either.

**Still open, and both should be settled before slice C starts the calendar:**

1. **§33c's format question** — move the checksum ahead of the length so the cover
   includes it? Five lines today; an archive migration once capture is running.
2. **§13's five Profiles verifications.** Not started. Three of them are missing
   venue endpoint facts, and they block slice C's sources — 1 (Binance per-symbol
   funding topic), 3/4/7 (Binance futures book host, path, cadences), 5/6 (Bybit
   linear public WS URL), plus standing debt #1, now blocking for the Bybit book,
   and the public-vs-authenticated question for the REST overlays. **Nothing goes
   into Profiles unverified, and nothing built may depend on an unverified fact.**

**Two spec edits are owed and marked**, design roster rows 3 and 5:
`platform/clock.Wall.Backsteps` (core §3.5's `Clock` block) and
`capture.RecordKind` (core §3.6 rule 8's second integer-enum exception).
