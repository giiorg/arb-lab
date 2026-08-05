# Task #153 — the per-hour `restart_boundary` on REST-overlay sources (§71)

**Status:** CLOSED (2026-08-05, session 107) — the ruling was taken by the user from three
options with their arguments (§71b, §71d); implemented, fixtured against the collapse
checklist (§71f), campaigned at 22 mutants with two survivors found and fixed (§71g),
`make ci` green uncached on both modules, marker grep zero. **No set-version change and no
normative section moves** — `restart_boundary` appears in none of the five documents, so the
edit set is this record, §32k's forward pointer and adr-history's dated block. That absence is
stated rather than left to be read as an oversight.
**Task:** #153, open since session ~60 (`2026-08-01-strategy2-capture-expansion.md` §3, and
HANDOFF's "awaits a user ruling" line in every status block since).
**Reads:** record §32a (the two decisions capture rests on — a restart is recorded honestly),
§32d (the counter capture does not mint), §32e (`prev_last_seq` in the Open record), §32g (the
record kinds), §32h (the REST overlay), §32k (liveness, verdicts, the positive control),
§38i (a file a restart reopened and appended to), §45n-1 (the crossvenue walkers' carry rule,
which keys on `OpenPrevLastSeq` and is NOT affected by this slice).

## §71a — what is wrong, measured

`arblab capture verify` reports `restart_boundary` on
`binance/…/BTCUSDT/funding/fundingInfo` **every hour** — **24 of the 50 findings** on a clean
day (§45's measurement), chronic rather than a restart artifact, and it scales with every
REST-overlay source added.

The cause is a designed property meeting an undesigned consequence, and both halves are
correct in isolation:

- Slice E settled that **"a Rest record is not a Data record"**, so a REST-overlay source
  mints `records 63, data 0` per hour. `writer.go`'s writer-owned records (Open, Close,
  Alive, Gap) carry `w.lastSeq` rather than minting one, so `lastSeq` never leaves zero.
- At rotation `w.prevLastSeq = w.lastSeq` (writer.go), so every hourly Open honestly
  declares `prev_last_seq: 0` — the documented "no predecessor".
- `verify.go`'s `checkOpenContinuity` reads that zero beside an existing predecessor and
  reports `restart_boundary`.

Nothing here is lying. What is wrong is the **name**: `FindingRestartBoundary`'s own doc says
*"the process was down across the boundary"*, and on these streams the process was up the
whole time.

**The stake is larger than the finding list, and this was not in the task's framing.**
`assignVerdicts` flags a window on ANY finding but `window_absent`, so every REST-overlay hour
currently reports **`VerdictGapped`** — "replay would not get everything" — for an hour in
which a living writer rotated a file and lost nothing. The verdict column is §32k's answer to
*what would a replay get*, and for these hours it has been answering wrongly, 24 hours a day,
per source. No production code consumes a verdict (the crossvenue walkers key on
`OpenPrevLastSeq` per §45n-1), so the damage is confined to what an operator reads — which is
exactly what §37g says gets learned-past.

## §71b — RULING (user, session 107): split the kind by the evidence

`prev_last_seq == 0` is not one fact. It is a point in a **product of two properties** — the
claim, and whether any sequence existed to lose — and one name over a product is this repo's
recurring defect shape (memory: *a vocabulary with one axis loses a case*). The classifier
takes the product:

| `prev_last_seq` | predecessor's observed end | writer identity across the seam | verdict |
|---|---|---|---|
| `0` | `> 0` | either | `restart_boundary` — a real loss |
| `0` | `0` | **same** `(host, pid)` | `unsequenced_boundary` — nothing to lose |
| `0` | `0` | **differs** | `restart_boundary` — a real process boundary |
| `0` | unknown (a walk that aborted or a torn tail) | either | `restart_boundary` — conservative |
| `≠ 0` | — | — | unchanged (`cross_file_break` / `continuity_unknown` / clean) |

**Nothing becomes silent.** Every boundary is still a finding, still coverage, still on the
report. One class gets a name that says what is actually known, and `restart_boundary` gets
its meaning back: *a restart cost this stream sequenced data*.

**The new kind is verdict-neutral, and that is the honest answer rather than the quiet one.**
A verdict is about REPLAY. When the same writer rotated an unsequenced stream, a replay of
that hour gets everything the archive holds, so `VerdictReplayable` is true and today's
`VerdictGapped` is false. Suppressing the row while leaving the verdict wrong would move the
noise one column right and call it a fix.

**The evidence is read from the archive, never from configuration.** The classifier is handed
the predecessor's own end and the two Open records' `(host, pid)`, both of which are already
in the file. A rule keyed on "this source is a REST overlay" would be a claim about the
roster, checked nowhere, and wrong the day a REST overlay starts sequencing.

### The claim is about the BOUNDARY, not about the stream's history

Worth stating because it is what makes the rule sound under a narrowed `--from`: verify walks
at most **one** file earlier than the range, so "this stream never sequenced" is not knowable
in general. It does not have to be. A same-`(host, pid)` seam with a zero claim on both sides
means *this writer* never held a sequence, and a living writer's rotation cannot be a restart
— whatever earlier processes did. A stream that sequenced under an earlier process and had
its data lost at some earlier seam gets its `restart_boundary` **at that seam**, which is
where the loss was.

## §71c — the mechanism

**`chainState` gains three facts, all carried the way `lastSeq`/`known` already are:**

- `sequenced bool` — monotone: some Data record has been seen, or a NON-ZERO `prev_last_seq`
  claim has been adopted, in this walk. The claim counts, and that is the load-bearing half:
  a narrowed range whose first walked file is a silent hour (`no_data`) still adopts that
  file's non-zero claim, so the next seam is correctly a `restart_boundary`.
- `truncated bool` — monotone: some preceding walk aborted (`scan.aborted`) or ended in a
  torn tail (`scan.tornTail`), so unread bytes could hold the Data record that would make
  `sequenced` true. Absence of evidence stops being evidence of absence, and the
  classification falls back to `restart_boundary`.
- `writer {host string; pid int; known bool}` — the previous Open record's identity. Reset to
  `known: false` when an Open payload does not parse, because an unreadable identity must not
  read as an equal one.

**`checkOpenContinuity`'s zero arm** takes the three and picks the kind per §71b's table. The
`(0, false)` return is unchanged — the reset to "unknown" exists so the file's first Data
record does not read as a hole back to sequence one, which is a different question from this
one and stays answered the same way.

**`FindingKind` gains `FindingUnsequencedBoundary`** (`"unsequenced_boundary"`), classified
`ClassCoverage` in `Class()`'s exhaustive switch.

**`FindingKind` gains `AffectsVerdict() bool`,** an exhaustive switch with no default, and
`assignVerdicts` consults it instead of its inline `FindingWindowAbsent` skip. Every existing
kind returns `true` — today's behaviour, preserved deliberately and pinned by a golden —
`FindingWindowAbsent` keeps its exclusion (moved, not changed), and the new kind returns
`false`. The point of the method is the direction of the default: a kind added next year must
make the verdict decision at a site the compiler and `exhaustiveswitch` both check, instead of
inheriting "flags the window" from a `for` loop nobody reads.

## §71d — the two rejected options, with the arguments that killed them

**Rejected: key it on `(host, pid)` alone** — two consecutive Opens agreeing on host and PID
are one living writer, so no finding. It is the cheapest rule and it is one premise short:
PIDs are **reused** by the OS, so `pid == pid` across a seam consumes a non-reuse guarantee
nobody offers (memory: *a comparison that outlives its guarantee*). Under this rule a genuine
restart landing on the recycled PID is **silent**. Under §71b's rule the same accident is
merely mis-*named* — the row is still on the report — because the identity comparison is a
conjunct that narrows a name, never a predicate that removes a row. That difference is the
whole reason the predecessor's end is in the rule at all.

**Rejected: suppress by source class** (the roster's REST-overlay kind). It hides a real
restart on those sources permanently, it keys the archive's classification on configuration
the archive does not carry, and it adds a second place to forget a source. It also fails the
day a REST overlay starts sequencing — the rule would still be reading the roster.

**Rejected: leave it and teach the operator to read past it.** That is §37g's named failure,
and the measurement (24 of 50) is what it costs.

## §71e — a finding this slice tripped over: the roster is stale and cannot say so

`finding_test.go`'s `declaredFindingKinds` is the map whose own doc says *"a kind with no
producer is a declared member no code path can reach — which is exactly the defect this file
exists because of"*. It lists **19** kinds. The enum declares **24**: the five manifest kinds
(`manifest_missing_file`, `manifest_mismatch`, `manifest_unreadable`, `unmanifested`,
`manifest_absent`) were added to `finding.go` and `Class()` and never to the roster. All five
DO have producing tests in `manifest_internal_test.go`, so nothing was unreachable — but the
roster's own completeness was checked by nothing, because a test that ranges over a map cannot
see the member that is not in it.

Fixed here rather than filed, because this slice adds the 25th member and the fix is what
makes the addition checkable: the map becomes a `producerFor(FindingKind) string` **switch**
with no default. `exhaustiveswitch` is in scope for a switch in a `_test.go` file over a
production enum (that is a documented, deliberate half of the gate), so a kind added without a
producing test is now a lint failure at the roster, and the compiler's own exhaustiveness is
not what is relied on.

## §71f — the collapse checklist, run before the campaign

Run first, as the rule says, and it changed four fixtures for free:

- **Cardinality.** The motivating fixture covers THREE windows, not two. With one
  boundary in range, "one finding per boundary" and "one finding per stream" are
  the same number, and the test would have passed against a check that fired once
  and stopped.
- **Order/identity.** The identity rows vary host and pid **independently**. With
  only the pid row, "compares the pid" and "compares host and pid" are the same
  predicate — and an archive synced from a second machine is exactly the case
  that separates them.
- **Value.** No fixture uses pid `0`: the zero pid is the zero VALUE, where "an
  identity nobody read" and "a process whose pid is zero" collapse. The
  predecessor's sequence is 5 rather than 1, and the adopted claim is 5 against a
  control of 0.
- **The predicate.** The per-row `wantDetail` was first written as the bare word
  `writer`, which both identity rows satisfy — so the assertion could not see
  which identity had changed. It now pins the rendered pair
  (`writer fixture/4242 to fixture/1`, `writer another-host/1 to fixture/1`,
  `writer unknown to fixture/1`), which also covers `why()`'s precedence and
  `writerIdentity.String()`'s unknown arm.
- **The fixture must contain the thing under test.** Every unsequenced fixture
  holds a real `Rest` record, because a file with neither Data nor Rest is §32k's
  dead feed and earns `no_data` — which would flag the window through a second
  door and make every verdict assertion vacuous. The motivating test asserts
  `RestRecords > 0` and `DataRecords == 0` on each window before asserting
  anything else.
- **The verdict needs both directions.** "unsequenced_boundary does not flag its
  window" says nothing without a row where a boundary DOES, so every table row
  asserts its window's verdict and the two answers are `Replayable` and `Gapped`.

## §71g — the mutation campaign

Twenty-two mutants, and **two SURVIVED on the first pass — both real gaps, both
now fixed**. Every claim below is checked by a test that has been seen to fail.

| # | Mutation | Verdict |
|---|---|---|
| M1 | drop the no-sequence conjunct | caught (9) |
| M2 | drop the same-writer conjunct | caught (8) |
| M3 | `same()` ignores both `known` flags | caught (1) |
| M4 | `same()` ignores this side's `known` | caught (1) |
| M5 | `same()` compares only the pid | caught (3) |
| M6 | `same()` compares only the host | caught (5) |
| M7 | `none()` ignores truncation | caught (3) |
| M8 | `none()` ignores a seen sequence | caught (9) |
| M9 | an adopted claim is not evidence | **SURVIVED → fixed → caught (2)** |
| M10 | a Data record is not evidence | caught (2) |
| M11 | a Close record's claim is not evidence | caught (2) |
| M12 | truncation is not carried | caught (2) |
| M13 | an unreadable Open keeps the stale identity | **SURVIVED → fixed → caught (2)** |
| M14 | a second Open in one file is not a seam | caught (5) |
| M15 | the verdict filter is inverted | caught (11) |
| M16 | the new kind flags its window | caught (4) |
| M17 | the new kind has no exit class | caught (2) |
| M18 | the detail names the two writers backwards | caught (4) |
| M19 | a read identity is recorded as unknown | caught (11) |
| M20 | an unknown identity renders as a real one | caught (3) |
| M21 | the two carried facts are swapped | caught (5) |
| M22 | identity is never carried across files | caught (9) |

**M9 — a fixture that stated its fact twice.** The adopted-claim test sealed its
silent hour, and a sealed file states its sequence in its Open claim AND in its
Close payload. Either alone answers §71's question, so deleting the Open-claim
half changed nothing: the Close path kept `sequenced` true and the test passed
against broken code. The silent hour is now UNSEALED — a crashed hour, which is
also the realistic shape — and the Close record's own half is a separate table
row (`the predecessor's Close record claims a sequence its records never
showed`). It is the collapse checklist's own lesson arriving one layer in: the
fixture was varied correctly and a second source of the same fact absorbed the
variation.

**M13 — a fixture whose RANGE excluded the thing under test.** The stale-identity
test built three files so an identity could go stale across the middle one, and
then asked verify for the THIRD window only. Verify walks exactly one file
earlier than the range, so the walk began at the middle file and the first file's
identity — the stale one the test exists for — was never read. Deleting the reset
was therefore invisible. The request now names windows two and three, which also
makes the readable control expect TWO boundaries rather than one.

**Both survivors share a shape worth naming:** the mutation was aimed correctly
and the FIXTURE could not observe it, in one case because a second source
supplied the same fact and in the other because the walk never reached the
planted state. Neither would have been found by reading the test.

**The §71e fix is demonstrated, not asserted.** Deleting the
`FindingUnsequencedBoundary` case from `producerFor` and running `make lint`:

```
finding_test.go:30:2: switch over FindingKind does not name FindingUnsequencedBoundary:
  every declared member must be named so that adding one breaks the build here
  rather than falling through (§3.6 rule 10)
make: *** [lint] Error 3
```

That is the roster's completeness becoming checkable for the first time.
