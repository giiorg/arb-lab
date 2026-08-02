# §48 — Milestone 3 slice A: the durable hash-chained journal (2026-08-02, session 74)

`ports.Journal` has an adapter. §47e finding 2 — that §5.3's fsync-before-network
crash boundary was "today proven only against a test fake" — is closed.

`internal/oms/infrastructure/driven/`: `journal.go` (the adapter),
`journalcodec.go` (the canonical encoding and the chain rule), `journalerrors.go`
(four sentinels), `journal_internal_test.go` (35 tests including the independent
replayer). Twelve mutations, twelve kills. `make ci` green, both modules; marker
grep zero.

## §48a — the fork §47f did not name, and why the answer was not the obvious one

§47f described slice A as *"append-only, hash-chained, fsync-committed"* and named
no storage engine. Session 73's handoff predicted the first design fork would be
**whether to reuse `engine/capture`'s frame codec**, on the ground that it is
already fuzzed and already has a hash chain.

**That fork was mis-stated in its premise: `engine/capture` has no hash chain.**
It has CRC32C per record, a **dense sequence** chain (`verify.go`'s `chainState`
carries `chainLast`/`chainKnown` — sequence numbers, not digests), and whole-file
`.sha256` sidecars written at close. Those are three real integrity mechanisms and
none of them is a per-record hash chain. Reuse would have supplied framing, and
the chain — slice A's defining requirement — would have been new code anyway.

This is [[rejected-alternative-is-an-unchecked-claim]] in its ordinary form: the
benefit credited to the reuse option was asserted in a handoff and never checked
against the package.

**The fork that actually decided the slice is normative and was in the core the
whole time.** §13 and D2 (core lines 643 and 101) name the store: *"embedded
durable store (SQLite for MVP: WAL + `synchronous=FULL`… boot verifies
`journal_mode`/`synchronous` pragmas; Postgres-ready)"* holding *"the OMS
write-ahead journal + outbox + audit projection"*. ADR `R2b-9` and `R2c-9` are the
findings behind it.

### The argument that settles it is §5.3's, not §13's

A spec naming a technology is weaker ground than it looks — technologies get
re-argued. The load-bearing sentence is §5.3's:

> Applying a fill (dedup → ledger → `PositionAllocation`), advancing the
> **per-instrument** REST recovery cursor, and appending journal/outbox events
> happen in **one transaction**.

and `driven/doc.go` had already drawn the consequence, one slice before anything
needed it:

> That needs a transaction seam no port here declares, and it lands with the
> durable adapter and the fill use case together, because **a transaction
> spanning two stores cannot be added to one of them alone.**

A hash-chained log file is the smaller implementation and it is the one a reader
reaches for. It is also a store the fill path **cannot join**, so slices D and F
would meet §5.3 by rewriting this adapter or would not meet it. Neither of
`AppendPlanned` or `AppendState` needs a transaction today, which is exactly why
the constraint is easy to miss from inside slice A: **the store is chosen by a
requirement that binds two slices later.**

**Rejected: the bespoke append-only file.** It wins on every local measure —
no dependency, an explicit `os.File.Sync()` that is directly mutable, a format
fully under our control — and loses on the only one that spans slices. Recorded
here so slice D does not rediscover the log file as an attractive simplification.

**Accepted by the user 2026-08-02**, including the dependency cost below.

## §48b — the dependency, priced

`transport/doc.go` records the engine module's first third-party runtime
dependency and that it *"has zero transitive dependencies, so the graph gains one
node"*. This is the second and the honest figure is worse: `modernc.org/sqlite`
brings **nine more modules** (`libc`, `mathutil`, `memory`, `bigfft`,
`go-humanize`, `uuid`, `go-isatty`, `go-strftime`, `golang.org/x/sys`).

§13 names the store and **no document names a driver** — five docs grepped — so
the driver was an open choice. cgo-free decided it: a cgo driver makes the OMS the
one component in a Linux-target deployment that cannot be cross-built, and there
is no exception for that on §12 milestone 3.

The `tools/lint` trick does not apply. That module exists so `golang.org/x/tools`
stays out of the engine's graph; a driver the engine *runs* has to be in it.

## §48c — the design, and the three decisions inside it

**Three tables, not one, and the argument is `ports.Journal`'s own.** The port
declares two methods rather than one `Append(Record)` because a single record type
would need *"an optional field, whose zero value is then a legal value with no
meaning"*. One wide table is that rejected shape rendered in SQL — half the
columns NULL on every row, and a state record carrying a stray `side` is
representable. A `journal` chain table plus one detail table per kind lets every
column in every table be `NOT NULL`. *Rejected: one table plus a CHECK constraint
asserting per-kind nullability.* It is expressible, and it states the field list
twice — once in the columns, once in the CHECK — and the copy that drifts is
always the one that is not the definition.

**Typed columns rather than an opaque body BLOB — and the reason is the oracle,
not the storage.** `fixedpoint/codec.go` argues the persistence half already
(int64 mantissa = SQLite INTEGER = Postgres BIGINT, asset in its own column). The
decisive half is testing: if the hashed bytes are stored as a blob, an oracle that
reads the blob and re-hashes it verifies the writer **against its own output**. It
would agree with any encoding, including a wrong one. Typed columns force the
replayer to rebuild the preimage from semantic values by a path that shares no
code with the writer, which is what makes agreement evidence. This generalises
past this slice: **a stored form that already contains the answer cannot be
oracle-checked.**

**Append-only is enforced by the database.** Six `BEFORE UPDATE`/`BEFORE DELETE`
triggers raising `ABORT`. An adapter that merely never issues `UPDATE` is
append-only by convention, and the convention holds until the first migration
script or `sqlite3` session. The standing rule about defaults with two directions,
applied to a table. It shows up in the tests as a cost worth seeing: the tamper
fixture has to **drop a trigger** before it can corrupt a record.

## §48d — the trap in the pragma verification, and the shape it belongs to

§13 requires boot to verify `journal_mode`/`synchronous`, so `Open` does. Two
things about it are worth carrying forward.

**Pragmas go in the DSN, not in a statement after opening.** `synchronous` is
**per connection** and `database/sql` owns a **pool**: a pragma issued once as a
statement configures whichever connection served it, and the pool's next
connection is created with the default. Silent, intermittent, and in the direction
that loses data.

**The verification and the DSN share their constants — and that sharing is what
makes the obvious test vacuous.** `journalDSN` and `verifyPragmas` both read
`wantSynchronous`, deliberately, so they cannot drift. The cost is that a mutation
of the constant is **self-consistent**: the DSN requests NORMAL, the verification
demands NORMAL, both agree, and every test that only asserts "Open succeeds"
passes against a journal that no longer fsyncs on commit.

This is capture's own note about a wrong CRC polynomial — *"a checksum that is
self-consistently wrong verifies its own output perfectly"* — arriving as a
**configuration constant** rather than as an algorithm. The fix is the same one:
the required values are named as **literals** in
`TestOpenSetsTheDurabilityPragmasToTheirREQUIREDValues`, not read from the
constants under test. Mutating `wantSynchronous = 2` → `1` is caught (4 tests);
without the literals it would have survived.

Generalised, because it is not about SQLite: **when a value is deliberately shared
between the code that SETS a thing and the code that CHECKS it, the check cannot
also be the test.** The test has to state the value independently. Any
configuration a system asserts about itself has this shape.

## §48e — the collapse checklist, run before the mutations

Four fixture parameters could have collapsed a distinction this code makes. Each
was varied before a single mutation ran, which is the order this repo settled on
after `boot` (it changed five fixtures for free).

| # | Dimension | What collapses if it is pinned | Fixture |
|---|---|---|---|
| 1 | **Chain length** | At length 1, "link to the previous record" and "link to the genesis seed" are the same predicate. **At length 2 they are still collapsed** — record 1 is both the previous record and the first record | lengths 1, 2, 3, 7; and `TestLinkIsToThePredecessorNotToTheFirst` asserts the **inequality** at length 3 |
| 2 | **Record kind** | A single-kind chain makes the kind tag a constant, and a constant in a preimage is a field no test can see | chains interleave planned and state records |
| 3 | **Record content** | Two records identical in everything but sequence are what makes the sequence's presence in the preimage observable | `TestIdenticalRecordsHashDifferently` — and the fixture is **ordinary**, not contrived: §5.3's duplicate deliveries make a repeated `AppendState` the normal case several times per fill |
| 4 | **Field split point** | A length prefix is invisible unless two records differ only in where one string ends and the next begins | two orders whose `clientID+venue` concatenations are equal, **both sides non-empty on both records** |

**Dimension 1 is the one §47f named in advance and it was named half-way.** The
handoff said *"a chain of length 1 makes 'verify against the previous record' and
'verify against the first' the same predicate"*. True — and the repair a reader
reaches for from that sentence is a chain of **two**, where the two readings are
**still** the same answer. Three is the shortest chain that separates them. This is
[[a-debt-that-names-a-count]] with the number one short: a warning worded around a
specific value pre-answers the question of which value fixes it.

**Dimension 4 is observability's series-key defect one layer down**, and the guard
against repeating it is written into the fixture as an assertion rather than a
comment: the test *fails* if either side of either split is empty, because
`("ab","c")` vs `("a","bc")` collides only under the **empty** separator and passes
against every wrong choice a person would actually make.

## §48f — bite-check results

Twelve mutations, twelve `caught`, no `SURVIVED` / `NO-OP!!` / `NOBUILD!!` /
`TIMEOUT!!` / `PANIC!!`. §47f required the fsync and the chain link to be checked
**separately**, and they are — different mutations, disjoint failure sets.

| Mutation | Verdict | Failing |
|---|---|---|
| `wantSynchronous` 2 → 1 (commits stop fsyncing) | caught | 4 |
| `prev` dropped from the chain preimage | caught | 8 |
| sequence dropped from the chain preimage | caught | 8 |
| length prefix dropped from `appendString` | caught | 11 |
| genesis seed → 32 zero bytes | caught | 8 |
| the two record kinds share a tag | caught | 8 |
| `chainLabel` v1 → v2 | caught | 8 |
| sequence stops advancing | caught | 9 |
| `verifyPragmas` stops checking `synchronous` | caught | 3 |
| the chain table loses its append-only trigger | caught | 2 |
| `AppendPlanned` stops validating the order | caught | 1 |
| `AppendState` stops checking state membership | caught | 3 |

The genesis mutation is worth its row. A zero seed is not merely a different
constant: under it, a record whose `prev_hash` column was **never written** — an
uninitialised buffer, a partially applied migration — verifies as the first record
of a chain.

## §48g — what this slice did NOT do, each with its reason

**1. `Deps.Journal` was not converted to a configuration, and that is slice B's.**
`module.go` predicts the change in full: *"When it lands, `singlewriter` FORCES the
correction… the adapter will sit in a policed package, so no root can name its
constructor, and this field becomes the journal's configuration (a path, a fsync
policy) with `module.go` constructing the adapter from it."* The prediction is
**correct and now live** — `driven` is policed, so a root can fill
`Deps.Journal ports.Journal` only by writing its own journal implementation
outside `oms`, which is what rule 6 exists to prevent. It is not done here because
§47f assigns *"Closes `module.go`'s stated hole"* to slice B, and because no
binary wires `oms` at all today (`app/capture`'s `imports_test.go` forbids it), so
the conversion would change an unused seam while **removing** the fake-journal
injection `module_test.go`'s one distinctive claim rests on. Slice B lands the
planning use case that gives the wiring something to be for. Task #161 carries it.

**2. §13's third durability precondition is not checked.** §13 names three — WAL,
`synchronous=FULL`, and *"local filesystem only — ext4/xfs with write barriers,
never NFS/network FS"*. `Open` verifies two.

It carries **no `SPEC EDIT OWED` marker**, and the distinction matters because the
marker is the wrong instrument twice over: a marker means *the code departs from
the spec and the spec must change*, and nothing here departs — the requirement is
scheduled, not declined. What the grep cannot see is an **unmarked** claim, which
is why this paragraph and task #168 exist instead. This is the §47e shape caught
on the way in rather than two milestones later.

Placement is §7.5's reconcile-only boot (slice F), on grounds rather than effort:
§7.5 already owns a list of refuse-to-trade posture assertions — margin mode,
spot-as-collateral, position mode, leverage pin — and "the journal is on a
filesystem SQLite can be trusted on" is a member of that list, not a property of
opening a file. Splitting it off would put one posture check in an adapter
constructor and the rest in boot, which is how a checklist acquires a member
nobody knows to look for. Task #168 also flags the open sub-question: the default
direction. An allow-list of local filesystems refuses working configurations
(btrfs, zfs); a deny-list of known-remote ones is the wrong default direction by
this repo's standing rule. That is a ruling, not an implementation detail.

**3. Chain VERIFICATION is not implemented, only chain CONSTRUCTION.** The only
replayer is the test oracle. A production verifier belongs with §7.5's boot, which
is the first caller that has a reason to run one; `ErrJournalChainCorrupt` covers
only the narrow case the append path can see for itself (a head row whose hash is
not 32 bytes). Stated because "hash-chained journal" reads as though verification
shipped with it.

**4. No new §3.6 gate is due, and this is a re-verification rather than an
inheritance.** §47f flagged the check for slice B and the reasoning applies here
too: `singlewriter` polices `oms/application/ports` and `oms/infrastructure/driven`
**by position**, so the journal adapter is covered the day it is declared, by
nobody remembering anything. Confirmed by running the gate, not by reading the
analyzer's doc.

## §48h — what lands in the normative core

- **§12 milestone 3** — the slice-A line gains a dated annotation: the adapter
  exists, §47e finding 2 is closed, the store is SQLite on §5.3's
  one-transaction ground, and §13's local-filesystem precondition is scheduled to
  slice F (task #168).
- **§13** — the backend roster gains the driver, `modernc.org/sqlite`, which it
  did not name, plus the graph cost. The rest of §13's store sentence is
  unchanged: it was right.
- **§5.1, §5.3, §6, §7.5** — **no change.** The slice implements them as written.
- **ADR §3** — dated block.
- **Doc set → v0.9.29.**
