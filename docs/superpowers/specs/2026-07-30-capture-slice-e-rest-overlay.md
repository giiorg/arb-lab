# §37 — the capture tool, slice E: the REST overlay (2026-07-30)

**Status: CLOSED at v0.9.14 (2026-07-30, one session).** The checklist ran
before the mutations and improved three fixtures (§37h), **14/14 aimed
mutations were caught** (one honest `NOBUILD!!` re-aim), the one owed spec
edit landed the same session (§37i), the marker grep returns **zero**, and
the overlay is **LIVE** — the capture process was restarted onto the
eight-stream config, with the restart seam recorded as the coverage finding
§32a priced in. `make ci` green on a cold cache, docs 102 pass,
`docs/formal/` untouched at Cx8.

Implements §32m slice E of
`2026-07-29-capture-tool-design.md` — §32h's REST overlay: Binance's
`GET /fapi/v1/fundingInfo`, polled hourly onto its own `(SourceID, Topic)`
stream, same file discipline. Slice D's one-day disk figure is NOT this
slice's; it becomes measurable ~09:53 UTC 2026-07-31 and is recorded
separately. E was taken up before D's figure deliberately: the figure is a
one-command checkpoint, E's own data is calendar-bound (every hour before the
overlay runs is an hour where "we asked and it was empty" and "we never
asked" are permanently indistinguishable), and §32m's stated ordering
principle — the calendar-bound thing starts as early as it safely can — cuts
for E, not for idling behind D.

**What the earlier slices already hold, verified rather than assumed:**
`RecordKindRest` exists in the codec (§33) and verify counts it "and nothing
more" (§36i: a REST body is provenance for a reader at step 5, not coverage
evidence). `Writer.Rest(payload []byte)` exists (§34), stamps from the
injected clock, and rides `mintRaw` — the record carries the *current*
`lastSeq` and does not advance the chain, so Rest records sit outside the
sequence-continuity claim by construction. The framing inside the payload is
explicitly this slice's to define (writer.go: "The payload framing — u32
metaLen ‖ meta ‖ body — is slice E's; what slice B owes it is a way in that
does not require a second writer").

**The venue fact is verified, dated, and richer than §32h assumed** (Profiles
"The `fundingInfo` overlay, observed live", 2026-07-29): the endpoint is
unauthenticated `GET https://fapi.binance.com/fapi/v1/fundingInfo`; BTCUSDT
is **present** (cap ±0.3%, interval 8h), so the "absent ⇒ 8h" branch does not
fire for our one traded symbol today; 4h is the majority interval across the
742 listed rows; and `updateTime` is `null` on exactly BTCUSDT while other
rows carry epoch-ms — a decoder typing it non-nullable parses 740 rows and
fails on the one we trade. None of that changes this slice, and that is the
point: the body is stored **verbatim** and parsed by nobody here (§32n), so
the null lands in the archive intact and bites whoever reads it at step 5,
where a parser bug is redoable against the raw bytes forever.

## §37a — the Rest payload framing, and where it lives

`internal/engine/capture/rest.go`: `RestPayload{Meta, Body}` with
`EncodeRestPayload`/`DecodeRestPayload`, framed `u32 metaLen ‖ meta ‖ body`.
Meta is JSON: `{method, url, status, error?, asked_at}`. The **body stays raw
bytes after the length prefix** — §31g rule 1 ("nothing is re-encoded on the
way in") is a rule about *venue* bytes, and the meta is ours: the request we
made, the status we got, the instant we asked. `asked_at` is RFC3339Nano from
the injected TimeSource, distinct from the record header's `RecvTime` (which
`Writer.Rest` stamps at write time) because the two differ by up to the
request timeout and "we asked at T" is the §32h fact.

**Rejected: meta as JSON wrapping a JSON-string body** — §31g's NDJSON
rejection verbatim: escaping round-trips but the bytes stop being the venue's,
and a content hash over the payload stops meaning anything.

**Rejected: no framing, body-only payload.** The §32h fact has three parts —
we asked (when), the venue answered (status), with these bytes. A bare body
records only the third; an empty bare body is indistinguishable from a
truncated write.

The decoder lives here and not in slice F because a framing without its
decoder is unfalsifiable — the round-trip property is the test.

## §37b — a failed poll is a Rest record with status 0, not a Gap

A transport failure (timeout, refused connection, DNS) is recorded as a Rest
record: `status: 0`, `error` carrying the transport's words, empty body. The
§32h distinction is "we asked" versus "we never asked" — a failed ask is
still an ask, a positive fact with a timestamp, exactly the same *kind* of
fact as the empty answer. A non-2xx response is not even that case: it is a
response, recorded verbatim with its status like any other.

**Rejected: a Gap record with a new `Cause`.** A gap's vocabulary is "records
may be missing here" — a discontinuity in a stream the venue was pushing.
Nothing is missing after a failed poll; the ask itself is fully recorded. It
would also add an enum member every `exhaustiveswitch`-policed switch must
decide, verify reconciles gaps against sequence holes the REST stream cannot
have, and the live-counter metrics treat gap causes as the dead-feed taxonomy
— three consumers all forced to classify something that is not in their
domain.

**Rejected: retry with backoff inside the hour.** The next scheduled poll is
the retry. The overlay's value is dated provenance for a slow-moving venue
config, not freshness; §32a decision 2 already priced recorded gaps into this
archive, and a retry loop is the first step toward the transport complexity
the WS side needed and this side does not.

## §37c — the poller lives in `transport`, with its own scheme rule

`internal/engine/venue/transport/rest.go`: a `Poller` doing one
`GET` per `Fetch(ctx)` — status and body back, verbatim, error only for
transport failure (a non-2xx is a *result*, not an error). Body reads are
capped by the caller's limit (`max_record`, same argument as the socket's
ReadLimit: a frame the archive could not hold is refused at the transport,
before it is counted as captured). Timeouts via `http.Client{Timeout}`, so no
member of the ambient-time family appears at a call site this package owns.

Schemes are `http`/`https`, validated in the poller's own config — **not** by
widening `Endpoint`. `Endpoint.Validate`'s `ws`/`wss` set is a claim about
what `Stream` can dial, and admitting `https` there would make a mistyped WS
source validate at boot and fail at dial. `http` exists for the same stated
reason `ws` does: the tests speak real HTTP to a real `httptest` server.

**Rejected: the poller in `app/capture`.** The composition root may read the
ambient clock to schedule, but venue wire code is this package's declared
competence (§32b: "shared with the future live path and capture is one
consumer of it") — the REST overlay of milestone 3's fee-tier reads will want
the same poller.

## §37d — scheduling: the ambient ticker decides WHEN to look, the injected clock decides WHETHER it is due

The REST pipeline reuses the app's split exactly as `app.go`'s package
comment states it: `driveTicks` (ambient) wakes the loop, and the loop asks
the **injected** clock whether `Now()` has crossed the next poll instant.
Poll instants sit on the `poll_interval` grid via `Truncate` — the same
anchor as the writer's rotation grid (the recomputed-check-must-share-the-
anchor lesson; both are `time.Time.Truncate`'s zero-time grid, §36j), so an
hourly poll lands once per hourly file. The first poll fires immediately on
start rather than waiting for the grid: a process that starts at :59 and
dies at :02 should still have asked once.

## §37e — config: a second list, not a discriminator

`rest_sources []RestSourceConfig` beside `sources`. The identity half is the
same five fields (venue, product, contract, symbol, stream); the wire half is
`scheme`/`host`/`path`, plus `topic` (the archive directory segment, §32h's
"its own (SourceID, Topic) stream"), `poll_interval` and `request_timeout`.

**Rejected: one list with a `kind` discriminator.** `SourceConfig`'s wire
half is WS-shaped — `Protocol`, a subscription topic the dialect interprets,
`FirstDataTimeout` for an assertion about a push stream. A discriminated
union would make five of nine fields meaningless per row and `Validate` a
two-armed switch; two types let each half refuse its own zero values.

`topic` is explicit (`fundingInfo`) rather than derived from the path:
the WS topic is the venue's own name for the stream, and Binance's name for
this one is `fundingInfo`, not `%2Ffapi%2Fv1%2FfundingInfo`. The duplicate-
key check extends across **both** lists — one `seen` map, because two writers
on one key is the same accident regardless of which list each row sits in.
`closedLabels` likewise draws from both lists (§36t's one-source collapse
fixture polices the label sets staying closed and complete).

`request_timeout` is validated `0 < t ≤ alive_interval`: the poll loop owns
its writer single-threaded, so a request in flight delays `Tick` by at most
the timeout, and bounding it by the Alive interval keeps the worst case at
one late tick rather than a coverage finding.

## §37f — one goroutine owns the whole pipeline, and the writer opens at start

The REST pipeline is one loop: wake on tick → drive `Tick()` → poll when due
→ `Rest()` the result. No queue, no pump, no sink. §32f's queue exists
because a socket pushes faster than a disk accepts; here the producer is a
scheduled pull whose result is one record an hour. The Writer's one-goroutine
rule is satisfied by having exactly one goroutine.

The writer opens at pipeline start, not on first response. The WS side delays
open so the venue's ack lands in the Open record (§36q); REST has no ack, and
opening early means a run whose polls all fail still archives Alive ticks and
status-0 Rest records — "capture alive, asks failing", which is the honest
description and reaches verify as such. Provenance: `Endpoint` the full URL,
`Subscribe` the request line (`GET /fapi/v1/fundingInfo`), no ack.

## §37g — verify: a Rest record is a venue frame for `FindingNoData`'s purpose

`FindingNoData` fires on "not one venue frame" (`dataRecords == 0`). A
Rest-only stream would trip it every hour by construction — a permanent
finding on correct behaviour, which is §36i's over-strong-property lesson:
the operator learns to read past it, and then it is not a finding anymore.
The scan now counts Rest records, and the condition becomes
`dataRecords == 0 && restRecords == 0`. The message keeps its meaning: a REST
response — any status, even a recorded failure — is the venue (or the
network) answering an ask, which is exactly the "this stream was alive and
the feed was not" distinction the finding exists to draw. WS streams never
hold Rest records, so their behaviour is unchanged; the fixture pair is a
Rest+Alive file (no finding) against an Alive-only file (finding), same hour
shape.

**Not changed:** Rest records still contribute nothing to coverage counting —
they are not on a schedule verify knows, and §36o's "compare the archive
against the CONFIG" would require verify to learn `poll_interval`, which is
slice F's full-verify territory if it is anyone's.

## §37h — the checklist ran first, and what it changed

Three fixture improvements, each a collapse found by asking "does the fixture
contain the thing under test":

1. **The endianness test's comment claimed "the meta is 76 bytes" — an
   unverifiable claim in prose** (§5e's lesson: a claim in a test comment is
   not tested unless the data can falsify it). Reworded to state what the
   data actually pins: a big-endian prefix read little-endian is a length in
   the millions, which the truncation check refuses.
2. **The poller's charset guards were exercised at one character each.**
   `ContainsAny(path, "?# ")` had a fixture for `?` alone — a mutant
   shrinking the set to one character survived every test. Rows added for
   `#`, the space, and the host's space.
3. **The Open record's provenance was asserted nowhere.** The REST writer
   opens with `Subscribe: "GET /fapi/v1/fundingInfo"` and no ack; the
   first-poll test now decodes the Open payload and pins all three fields.

Two collapse-aware fixtures were designed in from the start rather than
found: the **metaLen == remaining** boundary (where `>` and `>=` part ways —
the legal empty-body payload sits exactly on it), and the **exact grid
instant** (a tick at 13:00:00.000000000 must poll; one nanosecond earlier
must not — the `>=`-vs-`>` pin on the due check). The scheduling-rate lesson
(§36e) shaped the grid test's whole design: more ticks than due polls, at
varied below-boundary instants, with the decoded `AskedAt` values naming
which instants polled — a per-tick poller produces a different *set*, not
just a different count.

**The mutations: 14/14 caught.** The six codec/poller claims (metaLen
boundary, outcome conflict, status range, at-limit body, the +1 sentinel
byte, big-endian), the four pipeline claims (poll-per-tick, strict-> on the
grid, the dropped first poll, the undriven Tick), and the four integration
claims (verify's rest counter, the pre-§37g no_data condition, the
closed-labels fold, the cross-list dup check + the timeout bound). One
`NOBUILD!!` was a real re-aim, not a kill: `if true {` orphaned `next`, and
the re-aimed `!next.IsZero()` kept it referenced — the harness's verdict
table working exactly as written.

**One determinism finding worth carrying:** the poll loop computes the next
grid instant BEFORE issuing the ask, not after it returns. Semantically an
ask that straddles a boundary must not erase that boundary's poll — the
schedule is a function of when asks become due, never of how long the venue
took — and mechanically it is what makes the test harness's ordering fence
(`settle()`: two sends into a cap-1 tick channel) sound. The first version
computed it after, and the tests were unwritable without a race.

## §37i — the spec edit, found by the v0.9.13 lesson firing again

The §12 roster pre-registered nothing for slice E, and the marker grep was
clean — and the close's reading pass found the same shape v0.9.13's rows 1/4
had: `TestRealCaptureConfigLoads` now asserts an **eighth** capture stream in
`internal/`, while core §8 and §12.2's scope sentences stopped at seven.
Landed as **v0.9.14**: both sentences (the pair v0.9.13 established) carry
the hourly `fundingInfo` overlay, the marker sits at the asserting site, and
ADR §3's milestone-2 block gains the row. The meta-lesson is now twice-paid:
**a positive-control test over a config IS a scope assertion**, and it will
never look like one at writing time.

## §37j — what slice E deliberately does not have

- **No authenticated overlays** — §32h's deferral stands (credentials on the
  capture laptop; fee tier is not calendar-bound).
- **No retry inside the hour** — the next poll is the retry (§37b).
- **No query strings, no headers, no non-GET** — §32h's one endpoint needs
  none of them, and a field nothing uses is a field nothing validates.
- **No coverage expectation for Rest records in verify** — an expected count
  would need verify to learn `poll_interval`, which is slice F's full-verify
  territory if it is anyone's (§37g).
- **No dataAge metric for the overlay stream** — the gauge means "time since
  the last venue frame", a Rest record is not a Data record, and a gauge fed
  a lie is worse than an absent series. The overlay's health is visible in
  `capture_records_total` rising and the archive's own Rest records.

## RESUME HERE — slice E is CLOSED; the capture runs on eight streams

The process was restarted onto the v0.9.14 build and config this session —
check it first, as always: `pgrep -fl "arblab capture run"`, and the restart
seam shows up as `restart_boundary`/`process_restart` evidence, which is it
working. **Still owed, in order:** slice D's one-day disk figure, measurable
**~09:53 UTC 2026-07-31** (`du -sh archive/` + a full-day verify; record it
in the design record §32m's slice D row's spirit — measured, not estimated);
then slice F (full verify: manifests, digests, coverage report; `capture
cat`; **off-host sync verified by digest** — the laptop still holds the only
copy of an unrecoverable archive, and F is where that stops being true).
Marker count: **zero**, and it should stay zero until the next deliberate
deviation.
