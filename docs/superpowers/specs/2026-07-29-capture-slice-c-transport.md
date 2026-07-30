# §36 — the capture tool, slice C: the transport and verify (2026-07-29 / 2026-07-30)

**Status: FULLY BUILT, NOT CLOSED — every §32m slice C component now exists;
the fixture-collapse checklist, the mutations over the new code, and the close
have not run.** `make ci` green, all six gates clean, `-race` clean on a cold
cache. Implements §32m slice C of `2026-07-29-capture-tool-design.md`, over
slice A's codec (§33) and slice B's writer (§34), carrying the four items §35's
`RESUME HERE` handed forward.

**Slice C is NOT closed.** §32m's slice C is transport + `arblab capture run` +
config + observability + minimal `capture verify` with its positive control.
All of it exists and its suites pass; none of the new session-35 code has been
mutation-tested, and the closing reading pass has not happened. See `## RESUME
HERE` at the foot of this record.

**Three parts, three sessions.** §36a–§36h are the transport (session 33);
§36i–§36o are verify (session 34); §36p–§36s are the config schema, the
composition root and the CLI (session 35). The `RESUME HERE` list is the same
list, now holding only the verification work.

**What landed**

| Path | Holds |
|---|---|
| `internal/platform/clock/wall.go` | `Wall.Mono()` — §34c's open decision, resolved |
| `internal/engine/venue/transport/doc.go` | the §32b fence, the parsing line, the classification default |
| `internal/engine/venue/transport/endpoint.go` | §35b's three-part split as a type; the topic charset and traversal guard |
| `internal/engine/venue/transport/protocol.go` | `Class`, `ProtocolID`, the `Protocol` seam, `ProtocolFor` |
| `internal/engine/venue/transport/binance.go` | the path discipline — three hosts, one dialect, no ack |
| `internal/engine/venue/transport/bybit.go` | the message discipline — subscribe, ack, 20 s heartbeat |
| `internal/engine/venue/transport/stream.go` | `Sink`, `StreamConfig`, `Stream`: dial, subscribe, first-data assertion, read, reconnect |
| `internal/engine/venue/transport/errors.go` | the configuration-versus-runtime split |
| two `*_internal_test.go` | the protocol tables and the real-websocket integration suite |
| `internal/engine/capture/finding.go` | `FindingClass`, `FindingKind` and its `Class()`, `Finding`, `Verdict` |
| `internal/engine/capture/verify.go` | `VerifyConfig`, `Report`, `WindowReport`, `Verify` — the tree walk, the per-file walk, the continuity rules |
| `internal/engine/capture/verify_internal_test.go` | the harness over real writers; the healthy positive control and §32k's four faults |
| `internal/engine/capture/verifyseq_internal_test.go` | the hand-built builder, for the states the writer refuses |
| `internal/engine/capture/finding_test.go` | the three declared-set tests and the kind-to-producing-test roster |

**Dependency:** `github.com/coder/websocket v1.8.15`, the engine module's first
runtime third-party dependency. Zero transitive dependencies — `go mod graph`
gains one node. §36a.

Verification: **7 mutations, 7 kills**, after re-aiming one `NOBUILD!!` and
diagnosing one `SURVIVED` that was a **vacuous fixture** (§36e). Both gates that
bind on this package were probed by hand rather than assumed: stripping the
`notimenow` escape makes the gate name the line, and dropping a `Class` member
makes `exhaustiveswitch` name it — and restoring each makes the gate silent
again.

---

## §36a — the websocket library, and why the spec's first option could not be taken as written

§11's backend roster names *"nhooyr.io/websocket or gorilla/websocket"*. The
first module was **renamed to `github.com/coder/websocket` in 2024** and the old
path no longer resolves, so the spec names something that does not exist under
that name. The user took `coder/websocket` — the same library under its current
path — and §11 owes the rename. Marked `[SPEC EDIT OWED]` in the package doc.

The marker's wording matters and is the reason it is not absorbed: *"we chose
differently from the spec"* and *"the spec names a module that moved"* need
different edits, and only the second is true.

**Rejected: writing our own RFC 6455 client.** It matches the house
"reimplemented fresh" ethos and would give byte-level control over exactly what
reaches the archive. Rejected on the ordering argument that governs this whole
milestone (§31a): masking, fragmentation, control frames and the close handshake
all have to be right, this repo's own rules would then demand an oracle and a
fuzz target for them, and slice C is the **calendar-bound** slice. A month of
funding history is worth more than a socket stack we control.

Per §35's own lesson — a rejected alternative's stated benefit is an unchecked
claim — the benefit credited above is *asserted*, not measured: nothing here has
shown that byte-level control over the frame layer would catch a defect the
library hides. It is recorded as an assertion so a later session does not read it
as a finding.

---

## §36b — §34c's open decision: `Mono` lives on `platform/clock.Wall`

§34c left two candidate homes and one prohibition. Resolved to a `Mono()` method
on `*clock.Wall`, because the alternative — an adapter in `internal/app/capture`
— has to read the clock ambiently in a package that would then hold the second
time authority §32i explicitly rejected, to serve a record *about* the first one.
`Wall` already owns the raw `time.Now()` read and already exposes `Backsteps` for
this reason; `Mono` is the third reading of the same source, not a new one.

`*clock.Wall` now satisfies `capture.TimeSource` structurally. The compile-time
assertion for that belongs in the composition root (`kernelpurity` forbids
`platform/` importing `capture`), and it is on the RESUME list rather than
written, because the composition root does not exist yet.

### The finding: the forbidden spelling is one token from the right one, and Go hides the difference

§34c states the contract a signature cannot: `Mono` must keep advancing across a
wall-clock step, and deriving it from `Now` defeats the field. What §34c could
not know is how *small* the difference is in Go:

```go
time.Since(w.monoStart)        // step-immune
w.Now().Sub(w.monoStart)       // stands still exactly when it matters
```

Same package, same field, same arithmetic. A `time.Time` from `time.Now()`
carries a hidden monotonic reading that `Sub` silently prefers, and nothing in
the type system separates the two spellings.

**What separates them is where `monoStart` is filled from.** `NewWall` reads
`time.Now()` **directly** rather than through the `systemNow` seam, so no fake
clock can reach it and the independence is structural rather than promised. That
is deliberate and is the opposite of the usual instinct to route everything
through the seam.

**The fixture had to make the clamp ENGAGE.** Against a forward-only clock both
spellings agree on every reading, so a test that never steps the clock backwards
is a test the forbidden version passes. `TestWall_MonoAdvancesAcrossABackstep`
uses **two** clamped reads, not one — with a single held read, "the clamp holds
`Now`" and "`Now` happened to repeat once" are the same observation, and the run
of identical instants that `Mono` exists to disambiguate is not in the fixture at
all.

Both wrong spellings are killed (§36g), and the second — `Mono` from the raw
system read — is killed **because the helper polling for an advance is bounded**.
Unbounded it would have hung and reported `TIMEOUT!!`.

---

## §36c — the three-part endpoint, and the classification default

**§35b's finding is now a type.** `Endpoint{Scheme, Host, Prefix}` plus a topic
held separately, because the seven sources of §32c cannot be expressed by a topic
string: three Binance hosts with three path disciplines, and two Bybit sources
whose topic strings are byte-identical. `Prefix`'s meaning depends on the
discipline — the path in front of the topic, or the complete path — and that is
documented as the honest reading rather than hidden behind two fields, one of
which would always be empty.

`TestBybitDialURLSeparatesTwoSourcesWithIdenticalTopics` asserts the pair that
motivated the finding, and it asserts the topic is **absent** from both URLs,
which is what makes the URL the only thing separating them.

### Decision: a frame is `Data` unless it is positively recognised as noise

`Class` has three members and the default is `ClassData`. An envelope nobody has
seen — including bytes that are not JSON — is Data and reaches the archive
verbatim.

The direction is chosen for what happens when somebody forgets. A venue adding an
envelope next year is archived (recoverable, and possibly the error message that
explains a gap six weeks later) rather than discarded by the one component in the
pipeline whose purpose is not discarding things. The opposite default reads
tidier and quietly makes this package the thing that decides what history
contains.

The **two** exclusions are each load-bearing rather than tidiness, and §36d is
why:

- a **pong** counted as data lets §32k's first-data assertion be satisfied by our
  own keepalive;
- an **ack** counted as data does the same one message earlier, and it already
  has a home in §32g's Open-record provenance.

### Decision: control frames are counted, not archived, and no record kind was added

A pong is our own echo and carries no venue state; the ack is archived as
provenance. Neither becomes a `Data` record. §32g's record kinds are fixed and
the format is v1.0 by the user's call (§33c), so adding a `Control` kind would be
a format change to hold bytes we generated. **The safety valve is the default
above:** anything not positively recognised is archived, so the set of things
this decision can silently lose is exactly {our ack, our pong} and cannot grow by
omission.

### §34d's traversal finding, applied to its mirror before it bit

Slice B found a topic of `..` climbed out of the **archive** tree. A topic is a
path segment **in a URL** too, under the path discipline, and the same string
does the same thing: `../private/ws` against prefix `/market/ws` resolves to a
path we did not mean to dial and `net/http` will happily send.

The repo's rule is that a fix applied in one place gets applied to its mirror
immediately, so the guard went in **the session the second path-segment site
appeared** rather than after a second finding. The charset is a positive list,
for the reason every default here is written that way, and the guard applies
under **both** disciplines — a topic reaching the message discipline goes into
JSON where traversal means nothing, but the same config file feeds both, and a
rule that holds for one and not the other is a rule nobody can state from the
config alone.

`TestTopicTraversalIsRefused` carries its positive control in the same function:
all six real capture-set topics must pass, including the ones carrying `@`, `!`,
`.` and `:`.

---

## §36d — the finding that justifies the whole slice: a pong renewing the window

`Classify` refusing to count a pong as data is **not sufficient** for §32k, and
the insufficiency is one indirection further out than the obvious bug.

With a **per-read** deadline, every frame that is not data **restarts** the
first-data window. A Bybit heartbeat every 20 s under a 30 s first-data timeout
therefore renews it forever and the assertion never fires. The liveness monitor is
answered by our own keepalive after all — §31g rule 4's failure wearing a
disguise:

> not *"a pong counted as data"*, which `Classify` already refuses, but *"a pong
> resetting the clock that is watching for data"*, which costs nothing to write
> and is **invisible in a test whose server is silent**.

The fix is one line of intent: the first-data deadline is **absolute from
subscription**, created once and shared by every read in the phase.

This was found by writing the test, not by review — which is the fourth time in
this project that has been the sequence, and worth stating plainly: the defect
was in code written by someone who had just spent a page arguing that pongs must
not satisfy the assertion.

---

## §36e — the `SURVIVED` verdict, and the tenth fixture collapse

The mutation for §36d — restoring the per-read deadline — **survived**, and
diagnosing it was the work.

The test's fake venue looked like this:

```go
for {
    send(c, pong)
    time.Sleep(30 * time.Millisecond)
    if c.Ping(context.Background()) != nil { return }   // <-- BLOCKS
}
```

`Conn.Ping` waits for its reply, so **exactly one pong was ever sent.** The
window then closed because nothing was arriving — the ordinary silent-socket case
— and the renewal defect was never exercised. The test passed against the broken
implementation and read as coverage.

**This is fixture collapse number ten**, and it is the *cadence* variant: the
parameter pinned to a collapsing value was not a size or a depth but a **rate**,
and it was pinned by a side effect of a library call rather than by a literal in
the fixture. The nine before it could be found by reading the fixture's
arguments; this one could not.

**The guard that makes it impossible to hide again** is an assertion on the
fixture rather than on the code:

```go
if got := s.Stats().PongFrames; got < 3 { ... }
```

Counting executions says nothing until you have counted how many reach the
interesting branch. Here that count is *pongs actually delivered inside the
window*, and below three this test is the silent-socket test wearing a different
name. **The reusable form: when a test's discriminating power depends on a
fixture's rate, assert the rate.**

---

## §36f — a claim written inline can be unmutatable, and that is a design defect

The §36d mutation first reported **`NOBUILD!!`**: written inline, breaking the
absolute deadline also made `firstCtx` unused.

`NOBUILD!!` means re-aim (except for a compiler-enforced claim, which this is
not), and the re-aim was not a different `sed` — it was **naming the decision**.
The choice moved into `readDeadline(ctx, firstCtx, gotData)`, a function whose
whole body is the claim, and the mutation then compiled and was caught.

**The lesson is about code, not about the harness.** The one line in this package
that carries §32k's assertion was written in a form that could not be aimed at,
and a claim that cannot be shown to bite is indistinguishable from one that does
not. Extracting it is better code for an independent reason — it names the
question "which deadline governs this read?" — but the *reason it got extracted*
was mutability. Worth generalising: **if a claim cannot be mutated without
breaking the build for an unrelated reason, the claim is entangled with its
neighbours and wants a name.**

---

## §36g — the mutations, and the two gate probes

`tools/mutate.sh`, root module.

| Target | Mutation | Verdict |
|---|---|---|
| `clock/wall.go` `Mono` | `time.Since(monoStart)` → `w.Now().Sub(monoStart)` | caught (2 tests) |
| `clock/wall.go` `Mono` | → `w.systemNow().Sub(monoStart)` | caught (2 tests) — via the **bound**, else `TIMEOUT!!` |
| `stream.go` `readDeadline` | absolute first-data deadline → per-read | `NOBUILD!!` → re-aimed → `SURVIVED` → fixture fixed → **caught** |
| `stream.go` `readDeadline` | `if gotData` → `if true` (no first-data phase) | caught (2 tests) |
| `bybit.go` `Classify` | `return ClassPong` → `ClassData` | caught (4 tests) |
| `bybit.go` `AckResult` | `if !*env.Success` → `if false` | caught (4 tests) |
| `stream.go` `Run` | the pre-record `ctx.Err()` check → `if false` | caught (1 test) |

Gate probes, because a clean gate run is not evidence:

- **`notimenow`** — removing the escape from the backoff timer produces
  `stream.go:314: time.NewTimer reads the wall clock…`; restoring it is silent.
- **`exhaustiveswitch`** — dropping `ClassPong` from `Class.Validate` produces
  `switch over Class does not name ClassPong`; restoring it is silent.

### §32i's prediction about the `notimenow` escape SURVIVES, and the reason is not the one predicted

The escape's own doc says that if `engine/venue` ships and no escape is ever
used, the prediction was wrong and the mechanism should be deleted. There are two
uses in production code and both are genuine transport scheduling: the reconnect
backoff timer and the venue's documented 20 s heartbeat ticker. Neither stamps
anything.

**What did NOT need one is the interesting half.** Every socket deadline — dial,
subscribe, first-data, read — goes through `context.WithTimeout`, which reads no
clock in our code at all. §32i predicted the escape's first use would be *"socket
read deadlines"*; those turned out to need nothing, and a **ctx-first websocket
library is why**. Had we taken `gorilla/websocket` with its `SetReadDeadline` API,
the read path would have needed escapes on every read. The library choice of §36a
therefore changed which lines are policed — worth recording, because that
coupling is invisible from either document alone.

**And a scope fact nobody had hit yet: `notimenow` polices `engine/` TEST files.**
Four escapes went into `stream_internal_test.go` — three test-harness bounds and
the fake venue's send cadence. That is the gate working correctly (a test that
stamped from the ambient clock would be exactly as wrong), and it is recorded
because the first encounter with it reads like a false positive.

---

## §36h — what this part of slice C deliberately does not have

- **No composition root, no CLI, no config, no observability.** Slice C's
  remaining four items.
- **No `capture verify`.** §32m puts the minimal version in this slice
  deliberately, and it is not written.
- **No REST client.** §32h's overlay is slice E.
- **No permanent-failure classification.** A mistyped host retries at the backoff
  ceiling forever. The seam provided instead is
  `StreamStats.ConsecutiveFailures`, which is what an operator alert keys on; the
  reason a classifier was not attempted is that "permanent" is not decidable from
  a dial error, and guessing it wrong stops capture during a venue outage.
- **No reconnect ack in the archive.** `Subscribed` fires on every connection, but
  a reconnect inside an hour writes no new Open record, so only the hour's first
  ack becomes provenance. A known limit, not a defect: the reconnect itself is
  recorded as a gap, which is the fact that matters.
- **No jitter in the backoff.** Deterministic by choice, so the schedule is what
  the config says and a test asserts it without a random source. Seven sources
  reconnecting in lockstep is far below Bybit's documented 500-connections-per-
  5-minutes ceiling.

---

## §36i — verify's shape: two axes that are never summed

§32k names three verdicts and two exit codes, and the structure only works if
neither is derived from the other. So:

- **`Verdict`** — `replayable` / `gapped` / `absent`, per `(stream, window)`. It
  is about REPLAY: what step 5 would get if it read this hour.
- **`FindingClass`** — `integrity` (exit 1, the archive is lying) or `coverage`
  (exit 2, the archive is honest and thin). It is about TRUST: whether to believe
  the report at all.

A corrupt window is `gapped` **and** carries an integrity finding. Folding the two
together was considered and is wrong in both directions: a fourth verdict would be
a spec edit to §32k's three, and letting integrity findings pass as `gapped`
without a class would send an operator to widen a capture window when what they
need to do is stop trusting the file. `Report.ExitClass()` puts integrity first,
because thinness measured over bytes that contradict themselves is not a
measurement.

**The class is a METHOD on the kind, not a field on the finding**, and that is the
load-bearing design decision in the file. A field can disagree with its kind. A
method with an exhaustive switch cannot — and it makes adding a twentieth finding
kind a **compile-time failure** until somebody decides which exit code it earns.
The gate was probed rather than assumed: removing one case from `Class()` produces
*"switch over FindingKind does not name FindingContinuityUnknown"*, by name.

**Nineteen finding kinds.** More than §32k's list, and the additions are within
its stated purpose rather than departures from it — §32k's positive control
requires verify to name a deleted hour and a corrupted byte *specifically*, and
"specifically" is not expressible with a free-text detail an acceptance test has
to match on substrings. Each kind has a test that produces it, and
`declaredFindingKinds` in `finding_test.go` maps every kind to that test's name so
a kind added without one fails a test that says so.

**What verify refuses rather than reports:** a range holding no complete rotation
window (`ErrVerifyRangeEmpty`), and a threshold outside 1..100. The first is the
vacuity refusal — a verify over an empty range reports zero findings and exits
zero, which is indistinguishable from an archive that verified, and it is
reachable by a plausible typo in a `--from`/`--to` pair.

**Whole windows only, and partial ones are dropped rather than pro-rated.** The
expected Alive count is a constant from the config; pro-rating it for a
half-elapsed window would compute the expectation from the quantity being judged.
The cost is one flag: the operator excludes the live hour. This is a decision
§32k's text does not make, and it is recorded here rather than marked, because
§32k names no behaviour it departs from.

---

## §36j — `time.Time.Truncate` anchors at the ZERO TIME, not the Unix epoch

The defect, found by the healthy fixture reporting one window over and the next
one under for an archive with nothing wrong with it.

`Writer.nextTick` anchors Alive ticks with `time.Time.Truncate`, which rounds down
to a multiple of `d` **since January 1, year 1** — not since the Unix epoch. The
epoch sits 62135596800 seconds after the zero time, and **62135596800 mod 420 =
60**, so a 7-minute grid computed over `UnixNano` is shifted one minute from the
writer's own, and exactly one boundary crosses each hour edge.

The two grids agree **exactly when the interval divides 60 seconds.** With the
one-minute Alive interval anybody would reach for in a fixture, this is completely
invisible. §32l's coprime requirement — written for a different reason, that a
tick firing only on a rotation boundary makes "wrote a tick" and "rotated"
indistinguishable — is what exposed it.

**The reusable form:** when a check recomputes what another component computes,
the two must share the ANCHOR, not merely the formula. A grid, an epoch, a
rounding origin and a timezone are all anchors, and a disagreement in one produces
a result that is correct-looking, off by a small integer, and stable — which is the
hardest kind to notice.

**A second collapse in the same function, caught by the checklist rather than by a
mutation.** `expectedAliveTicks` ends `(end.Sub(first)-1)/interval + 1`, and the
`-1` matters only when a boundary lands exactly on the window's END. With a
7-minute interval in a 60-minute window that never happens — 60 is not a multiple
of 7 — so every fixture would have passed with the off-by-one in place. The unit
test now tables five `(period, interval)` pairs, and asserts that the 20-minute row
really does land on both window edges, because a row that did not would be a
duplicate of the first one.

---

## §36k — a declared enum member nothing emits, and its mirror one branch over

`FindingAliveShort` was declared, classified, documented, and **never produced**.
The coverage shortfall was computed inline in `assignVerdicts` to set the verdict,
so:

- the verdict came out right (`gapped`),
- the finding list said **nothing** about the only thing wrong with the window,
- and an exit code read off that list said **clean**.

Nothing could see it. The compiler was satisfied, `exhaustiveswitch` was satisfied
(`Class()` named the member), and coverage of `Class()` was complete. It is
`[[unreachable-sentinel-reads-as-handled]]` one layer out: there the defect is a
sentinel no code path returns, here it is an enum member no code path emits, and
in both cases the branch reads as handled.

**The fix is structural, not a line:** every reason for a window to be gapped is
now a FINDING, and the verdict is derived from the finding list alone. A verdict
computed from anything the list does not mention is a verdict that can be right
while the report is wrong.

**And it had a mirror, one branch over.** `scan.aborted` was added to suppress
coverage complaints about records past a corruption — and the *header* early
return did not set it, so a file whose magic was wrong reported "0 Alive ticks
against 8 expected" beside it: a coverage claim about bytes verify had declined to
read. That is the most aborted a walk can be. **A fix applied in one place gets
applied to its mirror** is this repo's standing rule, and the mirror here was
eleven lines above the fix.

---

## §36l — §35's item 2 resolved: source 4's asymmetry does not reach verify

§35's `RESUME HERE` handed this slice a constraint: source 4 (Binance **spot**
partial depth) carries `{lastUpdateId, bids, asks}` — no span, no back-link, no
event time — so it is the one source that **cannot be gap-checked from its own wire
bytes**, and "the positive control must not be written so the asymmetry
disappears."

**The asymmetry does not reach verify at all**, and the reason is §32n's fence
rather than a convenience. Verify reads **no payload a venue wrote**. Its
continuity check is over `CaptureSeq`, which is our reader's counter (§32d),
stamped identically for all seven sources — so source 4 is verified exactly as
strictly as the other six. Wire-level sequence checking is assigned to
`marketdata` at read time, which `doc.go` has said since slice A.

The constraint was addressed to the wrong slice, and that is worth naming rather
than quietly dropping: **a debt written against a document can outlive the reason
it applied.** §35 wrote it while looking at §32k's list of what verify reports,
where "CaptureSeq continuity" and "gap records by cause" both appear and neither
says whose sequence.

Source 4's shape (`sourceFourKey`, binance/spot/book) is in the healthy fixture
anyway, so "verify treats every source alike" is a **checked** claim rather than an
assumption that happens to hold.

---

## §36m — a live slice-B defect, found by a fixture rather than by review

**A restart into an hour the writer had already opened refuses every `Data` record
for the remainder of that hour.**

`OpenWriter` recovers the last Data sequence from an existing hour file and takes
it as its own floor — `w.lastSeq = max(w.lastSeq, scan.lastSeq)`, documented as
"it also bounds what this run may write" — while `Writer.Data` refuses any
sequence that does not exceed `lastSeq`. A restarted process whose reader counts
from 1 therefore has **every frame refused**, at whatever rate the venue sends
them, until the counter passes the old high-water mark. The archive fills with
Alive ticks; the process emits errors it has nowhere to put.

Nothing in slice B is wrong. The missing statement is about the **reader**, and it
belongs to the composition root: the counter must be seeded from
`writer.Stats().LastSeq` immediately after `OpenWriter`. No new API is needed —
`Stats()` already reports the recovered value — and the verify harness does it at
the line, with the reasoning attached, because that harness is the first code to
open a second writer over an existing archive.

**This is an obligation on item 2 of the remaining list**, and it is why verify
found it: writing a fixture for "a restart" is what first made a second writer
open a file the first had written. Review had read that line three times.

Verify would have *caught* it in production — a window of nothing but Alive ticks
is `FindingNoData`, which is precisely the Binance-trap signature — which is a
small piece of evidence that the mechanism works on a fault nobody designed it for.

---

## §36n — verify's mutations: 20 kills, two survivors, three re-aims

| Claim | Verdict |
|---|---|
| the Alive grid is `Truncate`'s, not the epoch's | caught (12) |
| a boundary exactly on the window end belongs to the next window | caught (6) |
| a boundary before the window is not counted inside it | caught (14) |
| coverage exactly at the threshold is not below it | caught (17) |
| short Alive coverage is a FINDING, not only a verdict | caught (1) |
| a gap record must COVER a hole to excuse it | caught (2) |
| a repeated `CaptureSeq` is not ascending | caught (2) |
| a drop of exactly one is a hole | caught (3) |
| a bad magic aborts the walk, so coverage is not measured | caught (2) |
| a corrupt file is not also faulted past the corruption | caught (3) |
| integrity outranks coverage in the exit class | caught (5) |
| parsing starts one file EARLIER than the range | caught (15) |
| a window with no file is reported absent | caught (2) |
| a `prev_last_seq` of zero beside a predecessor is a restart | caught (2) |
| the chain carries the Close record's claim, not the file's truth | caught (1) |
| an Open record is compared against the path it was found at | caught (1) |
| a crashed file is not faulted for the sidecar its crash prevented | caught (2) |
| the sidecar digest is compared, not merely read | caught (9) |
| an unrecognised file in the tree is reported under a class | caught (2) |
| a recorded overflow gap registers as covering its hole | caught (3) |

**Three `NOBUILD!!` verdicts, all §36f's finding repeating.** Removing
`case integrity > 0:` left `integrity` unused; removing `start = i` left `i`
unused; removing `if !ok {` left `ok` unused. Each re-aim was a *different
predicate* rather than a different `sed` — `integrity > 0 && coverage == 0`,
`start = i + 1`, `if ok && false` — and each expresses the claim more exactly than
the deletion would have. §36f's rule holds and needs no revision: a claim that
cannot be deleted without breaking the build for an unrelated reason is entangled
with its neighbours, and the fix is to state it more precisely.

**Two SURVIVED, and telling which is which was the work.**

1. `start = i * 0` **survived, and it is a WRONG MUTANT.** It sets the chain start
   to zero, which makes verify parse *every* file of the stream instead of only
   the range plus one predecessor — strictly MORE work, never a worse answer. The
   claim is "start one file earlier", and its negation is `start = i + 1`, which
   kills with 15 failing tests. **A mutant that makes the code do more is not a
   mutant of a claim about doing less.**
2. `if sawClose {` on the chain hand-off **survived, and it is a REAL TEST GAP.**
   The Close record's `last_seq` and the file's actual last Data sequence agree in
   every fixture where the Close record tells the truth, so which one the chain
   carries forward was unobservable. They disagree exactly when the Close record
   **lies** — and the successor's `prev_last_seq` is written from the same value
   the Close record carries (`writer.closeFile` sets `prevLastSeq = lastSeq`), so a
   successor agreeing with a lying Close is the EXPECTED pairing. Carrying the true
   value forward reports one fault twice.
   `TestVerifyReportsALyingCloseRecordOnce` now asserts exactly one finding, in the
   window whose Close lied. **Diagnosing the survivor produced a sharper claim than
   the one being tested.**

---

## §36o — what verify deliberately does not have

- **No repair, ever.** The writer's recovery truncates a torn tail (§32e); this
  finds the same tails and touches nothing. A verify that repaired what it found
  would destroy the evidence of the fault it was invoked to measure.
- **No wire-level gap check** (§36l), no payload parsing of any kind. §32n.
- **No inference of the rotation period or Alive interval from the archive.** Both
  are supplied, so the check compares the archive against the CONFIG rather than
  against itself.
- **No check that a virgin stream's first `Data` is sequence 1.** A reader seeded
  from the archive (§36m) legitimately starts above 1, and a restart into a fresh
  file writes `prev_last_seq = 0` while the counter continues — so the check would
  fire on every restart. Omitted deliberately.
- **No PID-based two-writer check.** An Open record whose PID differs from the
  previous Open's looks like evidence of two writers, and is not decidable: a
  process that crashed exactly on a record boundary leaves no torn tail and no
  Close record, so the next run legitimately appends a second Open with no gap
  record between. Non-ascending `CaptureSeq` is the evidence that IS decidable, and
  it is what §32k names.

---

## §36p — the config schema, and what the RESUME list undersold

**All three remaining items landed in session 35**: `internal/app/capture`
(config schema + composition root, §36p–§36q), `cmd/arblab` (§36r), and
`configs/capture.json` — §32c's seven verified sources as a checked-in
document, which the schema's positive-control test loads through the real
`platform/config` machinery. The tests are what keep the operator's actual
document loadable; there is exactly one spelling of the capture set in the
repository.

The schema lives in `internal/app/capture`, per `platform/config`'s own rule:
the kernel owns the mechanism, each schema belongs to the code that owns its
meaning. **No field carries `config:"hot"`**, and the claim is asserted as
BEHAVIOUR — `TestNoFieldIsHotReloadable` edits a top-level scalar, a nested
source field and a backoff entry and requires `ErrNotReloadable` for each —
because a tag added later must fail a test, not a reading pass.

Two facts the RESUME list's field roster did not carry:

- **`dial_timeout` and `subscribe_timeout` had to be added.** The list named
  `first_data_timeout` and `read_timeout`; `transport.StreamConfig.Validate`
  refuses a zero for all FOUR timeouts, so a config without the other two
  cannot construct a stream at all. The list was written from §32j's roster,
  which predates the transport — a debt list refreshes its assumption instead
  of examining it, the same shape as §35's mis-stated debts.
- **Two derived bounds are enforced at Validate, both anti-vacuity.**
  `alive_interval` must not exceed `rotation_period`, because an expected tick
  count of zero makes the coverage threshold pass every window vacuously; and
  `max_record` must not exceed `capture.MaxPayload`, because a socket allowed
  to accept a frame the archive cannot hold captures bytes the writer must
  then refuse — after they were counted as captured. Boundary fixtures pin
  both at equality, where `<` and `<=` collapse.

## §36q — the composition root: the writer opens on the first Subscribed, and the stream goroutine is the one that opens it

The one structural decision the record had left open was WHEN `OpenWriter`
runs, and two written constraints turn out to force everything:

1. §32g puts **the venue's subscribe ack** in the Open record's provenance,
   and §36h says the run's first ack becomes provenance. An ack can only be in
   the Open record if the writer opens AFTER the subscription is acked — so
   the open is triggered by the first `Subscribed` callback (or the first
   `Data`, defensively: a venue that sends data before its ack gets an Open
   record honestly carrying the request and no ack, and the later ack does not
   reopen — `TestDataBeforeAckOpensOnce`).
2. §36m's obligation — seed the sink's counter from `Stats().LastSeq` BEFORE
   any frame is stamped — means the seeding must happen synchronously inside
   the callback, before `Data` can run again. So the STREAM goroutine opens
   the writer, seeds the counter, and hands the writer to the consumer through
   a one-shot channel. The Writer's single-goroutine rule holds as two
   goroutines in SEQUENCE, with the channel send as the ownership transfer.

**Rejected: eager open at boot.** It would take the flock at startup (a second
process refused at boot rather than at first connect) and would archive dial
failures that precede the first successful connection. The cost that decided
it: the ack could then never reach ANY Open record without new writer API, and
slice B is closed. Both foregone benefits are recorded as assertions, not
findings; the flock refusal still fails the run loudly at first connect
(`TestOpenFailureFailsTheRun`), and pre-open disruptions are parked and become
the first records after the Open — what the run found on the way up
(`TestPreOpenDisruptionIsTheRunsFirstFinding`).

**Transport gaps ride a tagged side list, ordered by `afterSeq`.** A
disruption cannot ride `capture.Queue` (it carries records), and writing it
whenever the consumer happens to see it would misplace it relative to the
frames it interrupted. Each gap is tagged with the sink's counter at the
moment of disruption; the consumer writes it before the first record whose
sequence exceeds that value (`TestGapSitsBetweenTheFramesItInterrupted` pins
Open, 3×Data, Gap, 2×Data, Close, and that the gap echoes sequence 3). A gap
whose frames were dropped rather than written flushes on the next accepted
record, tick, or close — the same settlement rule the queue's own pending
overflow gap follows.

**The §36m fixture now exists against the root itself**:
`TestRestartIntoAnOpenHourContinuesTheSequence` runs two pipelines into one
hour file and requires data sequences 1..5 with an integrity-clean verify.

**Metrics: the by-cause gap counter counts what the ROOT writes** (reconnect,
no_first_data, overflow — a closed `cause` label set), not what the writer
mints during open-time recovery (torn_tail, process_restart). Those are
archive facts for verify; a live counter would double-report what the file
states, and their cadence (at most one each per open) alerts nobody. All five
§32j labels are closed sets from config; the end-to-end test asserts
`Registry.Rejected() == 0`, which is the "no Unbounded reason needed" claim
measured rather than declared.

**A gate fact, new in kind: `exhaustiveswitch` polices TEST files.** A test
switch over `RecordKind` naming two members was refused by `make ci` and
gained a grouped case naming the rest. Same family as §36g's "notimenow
polices engine/ test files" — recorded because the first encounter reads like
a false positive.

## §36r — cmd/arblab: the exit map is a renaming, and the report never sums

`arblab capture run -config F` and `arblab capture verify -config F -from T
-to T [-root D] [-min-alive P]`. Verify reads the CONFIG for the rotation
period and Alive interval always (§36o's comparison-against-config), and takes
`-root`/`-min-alive` as overrides — verifying a copied archive or asking a
stricter question changes neither of the two values the writer ran with.

`exitCode` is an exhaustive switch over `FindingClass` — integrity 1, coverage
2, unspecified 0 — and its test deliberately does NOT exercise precedence:
that decision is `Report.ExitClass`'s, and a second test of it here would be a
second place it appears to live. `formatReport` prints "findings: N integrity,
M coverage" and per-window verdict rows; the fixture for
`TestFormatReportKeepsTheAxesApart` holds 1+2 findings so a summed count (3)
is a number no honest line contains. The git SHA in provenance comes from
`debug.ReadBuildInfo`'s `vcs.revision` rather than ldflags — a stale build
script cannot write a wrong SHA into every Open record; "unknown" is the
honest value for an un-stamped binary.

The end-to-end test (`TestCaptureRunEndToEnd`) is the composition's positive
control: a marshalled config through `config.New`, a real websocket server,
`Run` under `-race`, the archive checked by the real `Verify` — data count
exact, zero integrity findings, zero rejected observations.

## §36s — what session 35 deliberately does not have

- **No verification of the new code yet.** The fixture-collapse checklist and
  the mutations have not run over internal/app/capture or cmd/arblab. That is
  the next session's first work, and nothing here should be trusted until it
  has survived it.
- **No REST client** (§32h is slice E), no `capture cat`, no /metrics
  exporter (the registry is in-process; §10's exporter remains deferred).
- **No hot path for a second run.** A second `capture run` over the same root
  fails at first connect on the flock, by design; there is no takeover.
- **Reconnect acks still never reach the archive** (§36h's limit, inherited —
  now precise: it is the RUN's first ack that becomes provenance, since hourly
  rotations copy the provenance fixed at open).

## RESUME HERE — slice C, what remains before the close

**All components are BUILT (§36p–§36s) and `make ci` is green on a cold
cache. Nothing below is verification-complete.**

1. **The fixture-collapse checklist over the session-35 code, BEFORE the
   mutations** (the standing order). Candidates worth suspicion: the
   `afterSeq` ordering tests all use gaps that flush via the next-record path
   — vary one to flush via tick and via close; the overflow fixture drops
   exactly 3 — vary a drop of 1; `TestRestartIntoAnOpenHour...` restarts with
   the SAME clock hour — the cross-hour restart is a different code path
   (fresh file, prev_last_seq 0).
2. **Mutations over the new code** (`tools/mutate.sh`, root module). Targets
   aimed at claims: drop the `Stats().LastSeq` seed (§36m — the motivating
   defect); make `takeGaps` ignore its bound; make `mapCause` collapse
   no_first_data into reconnect; drop the `PendingGap` flush in shutdown; make
   `exitCode` return coverage for integrity; un-close a metrics label set.
   Remember: no lock claims (`mutate.sh` runs without `-race`), and a
   compiler-enforced claim reports `NOBUILD!!` as its kill.
3. **The slice close**: `grep -rn "SPEC EDIT OWED" internal/ tools/` (expect
   FOUR, unchanged — session 35 added no markers) plus the READING PASS over
   what the new code asserts normatively, which has out-produced the grep
   three releases running. Then the doc-set release and §12's roster update in
   the design record.
4. **The calendar moment**: `go build -o bin/arblab ./cmd/arblab && bin/arblab
   capture run -config configs/capture.json` against the live venues, and a
   first real `capture verify` an hour later. Capture is the only work whose
   cost grows while other work happens; nothing else in milestone 2 starts
   before this does.

**Marker count at the end of this part: FOUR, unchanged.** The three §34 left
on §12's roster, plus §36a's `nhooyr.io/websocket` rename in the transport
package doc. Session 35 added none: the config schema and the root compose
what the spec already states, and the two Validate bounds in §36p are
anti-vacuity rules over this tool's own config, not claims about the core.
