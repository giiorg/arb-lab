# §36 — the capture tool, slice C: the transport (2026-07-29 / 2026-07-30)

**Status: PARTLY BUILT — the transport is done, three components remain.** `make
ci` green, all six gates clean, `-race` clean, docs 102 pass. Implements the
*first* part of §32m slice C of `2026-07-29-capture-tool-design.md`, over slice
A's codec (§33) and slice B's writer (§34), carrying the four items §35's
`RESUME HERE` handed forward.

**Slice C is NOT closed and funding capture is NOT yet running.** §32m's slice C
is transport + `arblab capture run` + config + observability + minimal `capture
verify` with its positive control. What exists is the transport and the live
`Mono` source. See `## RESUME HERE` at the foot of this record for the rest,
which is stated as work items rather than as prose so nothing has to be
re-derived.

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

## RESUME HERE — slice C, the four remaining items

Ordered. Nothing below is started.

1. **`capture verify`, minimal** (§32k) — CRC + `CaptureSeq` continuity within and
   across files + `Alive` tick coverage, per `(source, hour)`, verdict
   `replayable`/`gapped`/`absent`, **two exit codes** (1 = integrity failure, the
   archive is lying; 2 = coverage below threshold, the archive is honest and
   thin). Carry §35's item 2: **source 4 (Binance spot partial depth) has no
   wire-level gap check at all**, so its arm cannot assert what the other six can
   and **the positive control must not be written so the asymmetry disappears.**
   The positive control is §32k's own: build a healthy archive and assert `verify`
   reports clean, THEN kill a stream mid-capture, corrupt one byte, truncate
   mid-record and delete an hour, asserting `verify` names each one specifically.
2. **Config schema** (§32j) via `platform/config` — archive root, the source list
   (`SourceID` + **host + prefix + topic separately**, §36c), rotation period,
   queue depth, `alive_interval`, `sync_interval`, `first_data_timeout`,
   `read_timeout`, backoff schedule, `maxRecord`. **No field carries
   `config:"hot"`** — restart-required is correct for every one of them, and the
   restart's gap is recorded.
3. **`internal/app/capture`** — the composition root: config → sources →
   `transport.Stream` → a `Sink` that stamps `CaptureSeq`/`RecvTime`/`Mono` and
   pushes to `capture.Queue` → `capture.Writer` → observability. Three specific
   obligations:
   - the `DisruptionCause` → `capture.Cause` mapping as a **switch**, so
     `exhaustiveswitch` binds (dial/subscribe/read → `CauseReconnect`,
     `no_first_data` → `CauseNoFirstData`);
   - `var _ capture.TimeSource = (*clock.Wall)(nil)` — the assertion §36b could
     not write yet, and **one** `Wall` for the process, since `Mono`'s baseline is
     per-`Wall`;
   - §32j's observability labels `venue`/`product`/`symbol`/`stream`/`topic`, each
     a **closed set declared from config at registration**, so §10's default-deny
     needs no `Unbounded` reason.
4. **`cmd/arblab`** — `capture run` and `capture verify`, plus §32b's
   **import-graph test for `internal/app/capture`** (`go list -deps`, failing on
   `internal/marketdata`, `internal/oms`, `internal/strategy`) **with its positive
   control** asserting `platform/clock` is allowed. `internal/engine/capture`
   already has its copy.

Then: the fixture-collapse checklist over the new code **before** the mutations,
§32l's mutation targets for the parts now written, and the slice close —
`grep -rn "SPEC EDIT OWED" internal/ tools/` plus the reading pass, which has
out-produced the grep in three consecutive releases.

**Marker count at the end of this part: FOUR.** The three §34 left on §12's
roster, plus §36a's `nhooyr.io/websocket` rename in the transport package doc.
