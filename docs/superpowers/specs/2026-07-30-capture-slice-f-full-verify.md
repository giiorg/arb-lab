# §38 — the capture tool, slice F: full verify, `cat`, and the sync claim (2026-07-30)

**Status: CLOSED (2026-07-30, session 39) — v0.9.15. The close's findings are
§38h at the bottom; the mid-slice seam it resumed from is preserved above it.**

Implements §32m slice F of `2026-07-29-capture-tool-design.md`: full `verify`
(manifests, digests, coverage report), `capture cat`, off-host sync of closed
files verified by digest. Taken up while slice D's one-day disk figure waits on
the calendar (~09:53 UTC 2026-07-31) — the same §32m ordering argument that put
E ahead of D cuts for F ahead of idling, and F has its own calendar component:
every day the manifest layer is not running is a day whose file-deletion
evidence can never exist.

**What the earlier slices already hold, verified rather than assumed — F is
smaller than its roster row reads.** The `.sha256` sidecar is written at every
close (slice B); the Close record carries the rolling digest-before-close; and
verify already computes full digests and reports `digest_mismatch` /
`digest_missing` (`checkDigests`, slice C). Of §32m's "manifests, digests,
coverage report", the digests and the per-window coverage report **exist**.
What F actually adds: the `_manifest/<day>.json` roll-up and its cross-check in
verify, `capture cat`, and the machinery that makes "an off-host copy is
verified by digest" a one-command claim.

**The destination decision is the user's, and it was taken 2026-07-30:**
build destination-agnostic (option 4 of the four offered — cloud object store,
own server over SSH, external volume, agnostic). The slice's done-when is
therefore amended from "an off-host copy exists and is verified" to **"the sync
claim's machinery is proven against a second root, with its positive control;
the real off-host copy waits on the destination choice"** — recorded here so
the deferral is a decision with a date, not drift. §38f holds the necessity
calculus that informed it.

## §38a — the manifest: written at close time, not generated after the fact

`internal/engine/capture/manifest.go`. `_manifest/<day>.json` under the archive
version root, per §32e. One entry per **closed** file:

```
{path, window, sha256, records, gaps, last_seq, closed_at, source}
```

`path` is **relative to the archive root**, because the manifest travels with
the copy (§38e) and an absolute path is a fact about one host. `sha256` is the
sidecar's value — the full-file digest. `day` is the **file window's** UTC day,
not the close instant's: the 23:00 window's Close record legitimately carries a
RecvTime in the next day, and the manifest answers "what does day D's archive
hold", not "what closed during day D". `source` is `"close"` — §38b adds the
one other value.

**The entry is written by the writer, inside the close path, after the sidecar
succeeds and only then.** Close-time is when the digest, the record count and
the gap count are *facts being created* rather than facts being re-derived. The
ordering makes the crash-window artifact one-sided: sidecar present + manifest
entry absent is the expected shape of a crash between the two writes
(repairable, labeled, §38b); manifest entry present + sidecar absent would mean
the outer claim was written before the inner one existed, and the code refuses
to produce that state.

Concurrency: eight writers close files independently, and §32f's per-source
flock deliberately permits *different* sources to be captured by *different
processes* over one root — so process-local serialization is not enough. The
manifest upsert takes its own `flock` on the manifest file, read-modify-writes
the day JSON, and lands it by temp-file + fsync + rename. ~200 closes/day makes
the cost nothing. A torn manifest is a JSON parse failure, which verify reports
as integrity (§38c) — the manifest is covered by the same "corruption is
detected, not survived" discipline as the archive itself.

**Rejected: generating the manifest after the fact by scanning the tree.** A
manifest re-derived from the files it vouches for adds no information — and it
launders precisely the event this layer exists to catch: a file deleted before
generation day reads as a file that never existed. Close-time entries are what
make deletion *visible*: the entry outlives the file.

**Rejected: re-hashing files at manifest time.** Bit rot between close and
manifest generation would be hashed into the authoritative record. The digest
of record is the close-time digest, always.

**Writer additions:** a per-file gap counter beside `fileRecords` (reset on
open, carried by the entry), and `scanArchive` counts recovered Gap records so
a continued file's entry is whole-file truth, not this-run truth — the same
distinction `fileRecords` already draws.

## §38b — backfill exists, and its entries say so

`arblab capture manifest -day <day>`: for every file of that day holding a
`.sha256` sidecar and absent from the manifest, derive an entry **from the
sidecar and the file's own records** (digest from the sidecar — close-time
truth, never re-hashed from a re-read... the *records* are re-read for the gap
count, which is safe because gap records are CRC-covered close-time statements)
and upsert it with `source: "backfill"`. It never overwrites a `"close"` entry,
and it refuses files without sidecars — an open or crashed file has no
close-time truth to backfill from.

Two standing jobs, not one: the transition (every file closed before this
slice's build runs is un-manifested forever otherwise), and the crash window
(§38a's one-sided artifact, repaired to a *labeled* entry rather than left as a
permanent finding). §37g's lesson decides this: a permanent finding on a
correct, explained state teaches the operator to read past findings, and the
pre-F era would have produced hundreds. The label is what keeps the repair
honest — a backfilled entry never claims close-time provenance.

**Rejected: no backfill** — the operator-training hazard above, at ~8 streams ×
every pre-F hour. **Rejected: unlabeled backfill** — indistinguishable from
close-time entries, which un-pays for §38a's whole design.

## §38c — verify learns the manifest

Four new `FindingKind` members, classified by finding.go's standing rule
(integrity = the bytes or the claims about them cannot be trusted; coverage =
an honest description of thinness):

| Kind | Class | Means |
|---|---|---|
| `manifest_missing_file` | integrity | an entry's file is gone — the deletion evidence this layer exists for |
| `manifest_mismatch` | integrity | entry digest / record count / gap count disagrees with the recomputed file |
| `manifest_unreadable` | integrity | the day's manifest exists and does not parse |
| `unmanifested` | coverage | a closed, sidecar'd file absent from an existing day manifest — §38a's crash window |
| `manifest_absent` | coverage | a day in range with closed files and no manifest at all — one finding per day, not one per file |

(Five members; the first three are the integrity set.) Scoping follows the
request range as everything in verify does: entries whose window falls in
range, files discovered in range, days intersecting the range. `discover`
learns that `_manifest/` is part of the format rather than a stray-file
finding. `exhaustiveswitch` covers the new members by construction.

## §38d — `capture cat`: the promised tool, with a refusal in it

`arblab capture cat <file>...` renders records as NDJSON, one line per record —
§32e's "the archive is the fact; readability is a tool over it", and the
sentence core §3.6 rule 8 already spends: "`arblab capture cat` is where a kind
becomes a name". Landing it makes that sentence true.

Line shape: `{seq, kind, recv_time, mono_ns, ...payload}`. A payload that is
valid JSON is embedded **verbatim** (it left the venue as JSON; escaping it
would be the §31g re-encoding by the back door); anything else is `payload_b64`.
A Rest record's framing is decoded — `{meta: {...}, body: ...}` — because
rendering the frame §37a defined as opaque bytes would be a tool refusing to
read its own format.

**cat stops at the first invalid frame**, reporting the byte offset to stderr
and exiting non-zero. **Rejected: skip-and-resync.** Resynchronizing past a bad
frame is recovery's job, with recovery's rules (truncate, record the gap). A
render tool that silently skips corrupt bytes is how a corrupt archive gets
quoted as clean fact — the §31g hazard, at read time.

## §38e — the sync claim: transport is not ours; the claim is

The tool ships **no transport and no credentials** — §32n's growth roster and
the no-credentials rule both cut against embedding a worse rsync inside a
capture tool. `rsync`/`rclone` move the bytes; the recipe is documented in
HANDOFF, and the open hour's file is explicitly **outside the claim** (§32m
says *closed* files; the open file has no close-time truth yet and its copy is
legitimately torn).

What is ours is the claim: **"the copy holds every closed file, byte-identical,
and can prove it without asking the source."** Three parts make it one command:

1. the manifest lives *inside* the version root with root-relative paths, so it
   travels with every copy;
2. `capture verify` gains a `-root` flag overriding the config's archive root,
   so the same binary verifies any copy against the same expectations;
3. §38c's `manifest_missing_file` is what turns "the copy verified clean" into
   "the copy is *complete*" — without the manifest, a copy missing whole files
   verifies clean because nothing on disk contradicts anything.

The **positive control is part of the slice**, per the standing rule: sync a
fixture archive to a second root, verify clean; delete one closed file from the
copy, verify must say `manifest_missing_file`; corrupt one byte, verify must
say `digest_mismatch`. A verification that has never caught a broken copy is a
predicate that has never been shown to bite.

## §38f — the destination, and the honest answer to "do we definitely need it"

Asked directly by the user this session. The honest calculus, recorded so the
decision isn't re-derived from vibes later:

- **Funding rates are re-fetchable** — both venues serve funding-rate history
  over public REST, so the kill gate's carry side survives an archive loss.
- **The books are not re-capturable** — no venue replays L2 history. The paid
  substitute exists (Tardis), at real cost and in their format, not ours; the
  execution-cost side of the kill gate leans on books.
- **The real currency is calendar time.** Losing week 3 of a 4-week
  measurement restarts the measurement clock or buys the substitute; the money
  loss is bounded, the schedule loss is not.
- **The insurance is cheap**: ~6 GB/day and a one-line rsync once a target
  exists.

So: **not "definitely" — the loss is bounded and partially substitutable — but
the asymmetry (cheap premium, calendar-denominated loss on a single laptop
disk) says carry the insurance.** The user's option-4 choice defers the
destination, not the machinery; when a destination is chosen, what remains is
one rsync line and one verify run, both already proven against the second-root
fixture.

## §38g — deliberate absences

- **No transport, no credentials** (§38e).
- **No compression.** ~6 GB/day is not a problem this slice has; compression
  would put a codec between the digest and the bytes.
- **No retention policy.** Nothing is deleted; a deletion policy is a decision
  for after the kill gate, taken over data whose value is then known.
- **No fast/digest-only verify mode.** Full verify re-reads every byte at
  ~seconds/GB; an optimization with no measured need is §31e's hazard.
- **No manifest configuration.** The manifest is integrity machinery, not a
  feature: it is always on, like the sidecar. A config knob would be a way to
  forget it.

## §12 — spec edits this slice expects to owe

**None in core, expected.** Core's only slice-F-adjacent sentence (§3.6 rule
8's "`capture cat` is where a kind becomes a name") is made true, not edited.
§32e's manifest field list ("digest, record count and gap count") is widened by
`{window, last_seq, closed_at, source}` — an addition inside this design
record's own §32e, edited in place with this slice's date rather than owed to
core. The close's reading pass holds the veto, per the twice-paid lesson: a
positive-control test over the new machinery **is** a scope assertion, and the
last two releases each found one the roster missed.

## RESUME HERE — mid-slice seam (session 38 paused deliberately, context limit)
## [HISTORICAL — session 39 resumed exactly here and closed the slice; see §38h]

**State: everything below is committed and `make ci` is green on it — six gates
clean, both modules. The slice is NOT closed: no checklist pass, no mutations,
no reading pass, and the live capture still runs the pre-manifest build.**

Built and tested this session:

- **§38a manifest** — `engine/capture/manifest.go`: `ManifestEntry` (+
  `ManifestSource` close/backfill, zero-value-refusing `Validate`),
  `upsertManifestEntry` (close path: replace), `insertManifestEntryIfAbsent`
  (backfill: never displaces close-time truth), locked read-modify-write under
  a BLOCKING flock (`flockExclusiveBlocking` in both lock_*.go — blocking is
  deliberate and argued in the comment), temp+fsync+rename. Writer wiring in
  `closeFile`: entry written only when mint+sync+close+sidecar ALL succeeded;
  `fileGaps` counter beside `fileRecords`; `scanArchive` counts recovered Gap
  records. Errors: eight `ErrManifest*` sentinels in errors.go.
- **§38b backfill** — `BackfillManifestDay(root, day, now)`: digest COPIED from
  sidecar (never re-hashed), counts from a full record walk, skips crashed and
  sidecar-less files, idempotent, labeled `backfill`.
- **§38c verify** — five new FindingKinds classified in `finding.go`
  (`manifest_missing_file`/`manifest_mismatch`/`manifest_unreadable` integrity;
  `unmanifested`/`manifest_absent` coverage); `discover` skips `_manifest/` as
  format-owned; `checkManifestDays`/`checkManifestEntry` in verify.go with two
  paid-for guards: the SUPPRESSION rule (a file changed after close earns
  `digest_mismatch` alone, not a manifest echo — `fileScan.sidecarMismatch`)
  and the EMPTY-DIGEST guard (header-abort path hashed nothing; comparing a
  claim against "" would re-report `file_header`). `manifest_absent` is one
  finding per day, only for days with ≥1 cleanly closed in-range file.
- **§38d cat** — `engine/capture/cat.go`: NDJSON, verbatim-JSON-else-base64,
  Rest framing decoded, kindToken switch (exhaustiveswitch-covered), stops at
  first invalid frame with offset AFTER flushing complete lines.
- **cmd** — `capture cat <file>...` and `capture manifest -config -day
  [-root]` wired + usage; the dispatch test's "unknown subcommand" fixture was
  `capture cat` by design ("not built yet") — now `capture replay`.
- **Discovered pre-paid:** `capture verify -root` (§38e item 2) already
  existed since slice C — verifying a copy needed NO new flag work.
- **Tests** — `manifest_internal_test.go`: writer closes land entries (the
  23:00-window day-boundary fixture — day from WINDOW, not close instant);
  upsert-replaces vs backfill-never-displaces; zero-value refusal table;
  backfill-copies-sidecar-not-bytes (planted digest fixture) + skip rules +
  idempotence; 8-writer concurrent upsert under -race; the §38c roster one
  distinguishing fixture per row (deleted file → `manifest_missing_file`
  integrity; stale entry caught ONE FIELD AT A TIME ×4; changed file reported
  ONCE; `unmanifested` crash window; `manifest_absent` once-per-day + backfill
  repairs it; crashed-only day makes no claim; unreadable ×3 shapes); the
  done-when test `TestVerifyProvesACopyCompleteByManifest` (faithful copy
  clean; partial copy → integrity). The verifyseq `archiveBuilder` now mirrors
  the writer contract (truthful manifest entry on `write()` when closed, gaps
  counter) — fixture lies live in the Close RECORD, the manifest stays true,
  same as the real writer.

**Still owed, in order:**

1. **cat tests** (`cat_internal_test.go`, not yet written): render-every-kind
   fixture (one file: open, data-JSON — assert payload VERBATIM by raw
   substring, data-binary → `payload_b64` round-trip, alive → neither field,
   gap, rest-success → meta+body split, rest-failed status 0 → meta.error and
   NO body field, close), seq/mono/recv_time echoed; stops-at-corrupt fixture
   (corrupt a middle byte → error names path+offset, output holds exactly the
   complete lines before it); non-arbcap file → ErrMagic wrap.
2. **Fixture-collapse checklist** over ALL new fixtures, BEFORE mutations —
   some checklist thinking is already inlined (two gaps not one; pairwise
   distinct records/gaps/lastSeq; one-wrong-field-at-a-time), but the pass has
   not been RUN as a pass.
3. **Aimed mutations** (`tools/mutate.sh`), targets at minimum: the
   mint+sync+close+sidecar gate in `closeFile` (drop one conjunct); day from
   window vs ClosedAt in `ManifestEntry.day`; upsert replace vs append-dup;
   insertIfAbsent's presence check; backfill's sidecar-copy (re-hash mutant),
   its sawClose/tornBytes skips; `scanArchive` gaps++ and `emit` fileGaps++;
   each of the four `checkManifestEntry` comparisons; the suppression flag;
   the empty-digest guard; `closedByDay`'s `sawClose && !aborted` gate;
   `manifest_absent`'s `n > 0` guard; cat's json.Valid branch, Rest decode
   branch, flush-before-error. Expect some NOBUILD re-aims; read the table.
4. **Close** — marker grep both paths (expect ZERO — this slice has written no
   `SPEC EDIT OWED` markers); the reading pass with §38's §12 in hand (core
   rule 8's cat sentence becomes TRUE, §32e's field list is widened in-record;
   the twice-paid lesson says look hardest at the new POSITIVE-CONTROL tests
   for scope assertions); HANDOFF + memory; docs suite.
5. **Operational** — restart the capture onto the manifest-writing build
   (recorded seam, same as v0.9.13→14); then `capture manifest -day
   2026-07-30` (and -day 2026-07-31 if the restart lands after midnight UTC)
   to backfill the pre-manifest hours; verify. `pgrep -fl "arblab capture
   run"` FIRST, always.
6. **Calendar** — slice D's one-day disk figure at ~09:53 UTC 2026-07-31
   (`du -sh archive/`, full-day verify; archive was 324M at 11:27 UTC day 1,
   on pace for the ~6 GB/day estimate). The off-host DESTINATION is still the
   user's open decision (§38f — option 4 taken 2026-07-30); the machinery no
   longer waits on it.

## §38h — the close (session 39, 2026-07-30)

**Order held: cat tests → checklist pass → mutations → reading pass → close.**
`make ci` green (six gates clean, both modules), marker grep
`grep -rn "SPEC EDIT OWED" internal/ tools/` **ZERO** — as §12 predicted, this
slice wrote no markers, and the one owed widening landed in-record.

### The cat tests (`cat_internal_test.go`)

The §38d roster as three tests, with the collapse discipline applied at design
time and stated in the file header: the verbatim payload carries `<` and `&`
(without them, `SetEscapeHTML(false)` is uncheckable), verbatim is asserted by
raw substring (a parser accepts the escaped form too), the binary payload is
non-empty AND invalid JSON (either collapse re-lands on a neighbouring row),
seqs are distinct from line indexes, and the corrupt frame has a valid record
BEHIND it — a corrupt LAST record cannot distinguish "stopped at the fault"
from "reached the end". The render-every-kind fixture covers all eight kinds
the writer can emit including `clock_step` (the roster's list stopped at
seven), and the intact-bytes run inside the corrupt-frame test is the positive
control the refusal needs.

### The checklist pass — three pinned parameters found, then a fourth fixture

Run as a pass over every slice-F fixture, BEFORE the mutations, per the
nine-times rule. It found three:

1. **Sorted order equal to insertion order** — the upsert test inserted
   `v1/a/…` then `v1/b/…` and asserted "sorted", which holds with the
   `SortFunc` deleted. The backfill path now sorts BEFORE the close path while
   inserting AFTER it.
2. **No eligible file outside the backfill's day** — `BackfillManifestDay`'s
   day filter had nothing to filter, so "walks the asked day" and "walks the
   whole tree" produced the same `added`. A sealed next-day file now sits in
   the fixture and the next day's manifest is asserted absent.
3. **A single-day universe** — every verify fixture lived inside one UTC day,
   so the manifest day loop never advanced: a loop pinned to the first day
   passed the whole suite. The new cross-midnight subtest (23:00 + 00:00
   windows, second day's manifest deleted) is the isolating fixture.

The pass also added the **torn-tail backfill fixture**: nothing exercised
`walkFileCounts`' `tornBytes` guard, and backfill is the one caller that never
verifies the digest it copies — the guard is all that stands between a
damaged file and a plausible entry for it.

### The mutations — 30/30 caught, after the battery found two real gaps

The full battery: 6 writer (close gate, window-vs-instant day, both counters,
both recovery restores), 1 recover (`gaps++`), 9 manifest (day derivation,
upsert-replace, insert-if-absent, sort, day filter, invented digest, crashed
skip, sidecar copy, torn tail), 10 verify (closedByDay gate, day-loop advance,
`n > 0`, all four entry comparisons, suppression, empty-digest guard,
aborted-walk guard), 4 cat (json.Valid, Rest decode, flush-before-error,
offset origin). First run: 26 caught, one `NOBUILD!!` (a single-use variable
sed — re-aimed as a digest rotation, then caught), and **three SURVIVED, of
which two were real test gaps the checklist could not see**, because the
missing fixtures were about *drivers*, not parameters:

- **Nothing drove a close whose sidecar write fails.** The gate in
  `closeFile` — entry only when mint+sync+close+sidecar ALL succeeded — had
  never been seen to refuse. `TestFailedCloseLandsNoManifestEntry` fails the
  sidecar specifically (file's directory read-only, `_manifest/` writable),
  because that is the conjunct whose failure leaves the manifest write itself
  fully able to proceed — the exact accident the ordering exists to prevent.
- **Nothing followed a crash-recovered file to its eventual close's entry.**
  The `w.fileRecords/fileGaps = scan.…` restores could be zeroed silently.
  `TestReopenedFileClosesWithFullFileCounts` crashes a file mid-record,
  reopens, writes more, closes, and anchors the entry against an independent
  walk — with two pre-crash gap records, so the retained portion contributes
  a number the post-reopen writes cannot reproduce.

The third SURVIVED was the same gate mutation before its test existed. All
three resolved; the re-run reports caught across the board.

### The reading pass and the one spec edit

Core §3.6 rule 8's sentence — "`arblab capture cat` is where a kind becomes a
name" — is now literally `kindToken`, refused zero-kind named honestly;
**made true, not edited**, as §12 predicted. The §32e widening landed in the
capture design's own Integrity paragraph, dated: the manifest lists "digest,
record count and gap count" **plus `{window, last_seq, closed_at, source}`**
— the fields that make an entry checkable without reading the file and honest
about its provenance. The positive-control scope check (the twice-paid
lesson) found no third instance: the done-when test's claim is §38e's own,
the entry-shape control is the §32e list as widened, and cat's token
vocabulary is §38d's.

### Operational close-out (the plan; the log is the evidence of the outcome)

Restart the capture onto the v0.9.15 build at a recorded seam (SIGTERM, clean
close, relaunch — same as v0.9.13→14), then `capture manifest -day 2026-07-30`
to backfill the pre-manifest hours, then a day verify: expected shape is
integrity ZERO, coverage findings only for the open hour, and the backfilled
day's `manifest_absent` gone. The hour spanning the restart is closed by the
NEW build with recovered counters — the exact path
`TestReopenedFileClosesWithFullFileCounts` pins. Slice D's disk figure stays
on calendar (~09:53 UTC 2026-07-31); the sync destination stays the user's
open §38f decision, which the machinery no longer waits on.

## §38i — the reopened hour: production's first finding, fixed the same hour

**Found by the close-out's own verify, 3 minutes after the restart.** The
restart closed every file mid-hour (sidecar + entry = that close's truth) and
the new run reopened and appended, so the whole-file digest disagreed with
both — and verify read that correct, already-recorded state as
`digest_mismatch`, verdict INTEGRITY. `capture cat`'s first forensic job
confirmed the shape: old Close at record 62872, a second Open, a
`process_restart` gap saying in its own words "was closed cleanly and is
being appended to by a new run", then live data. Only the busiest stream
(Bybit books) showed it — the other seven reopened files had not yet flushed
appended bytes to disk. Left unfixed it worsens: now that closes write
manifest entries, the next restart's mid-hour verify would add THREE
`manifest_mismatch` count findings per reopened file. "Expect 0 integrity
findings, always" false for up to an hour after every routine restart is
§37g's operator-training hazard verbatim.

**The rule:** the scan snapshots the rolling digest at each Close record —
`digestAtClose`, the exact span the sidecar and the entry hashed at close
time (the mirror of `preClose` one line up, and the anchor matters: taken
BEFORE the Close frame it never matches anything — the recomputed-check
anchor lesson, made mechanical by a caught mutant). A sidecar that equals it
describes an intact prefix; with a tail that DECODES, that is a continuation
and the gap record is its story — no finding. The two controls that keep the
integrity signal: garbage appended after a close leaves the same intact
prefix with an undecodable tail (`tornTail`/`aborted` block the suppression —
without the clean-tail conjunct the vandalism fixture and the continuation
fixture are one fixture), and a claim matching NEITHER digest stays the
mismatch it always was (truncation loses the Close record and with it
`digestAtClose`, so checkDigests' own motivating case stays caught).

**The manifest mirror differs deliberately in one conjunct:** an entry whose
digest equals `digestAtClose` is provably prefix-truth whatever the tail
holds, so its count comparisons are suppressed even over a vandalized tail —
the vandalism already earned its one `digest_mismatch`, and prefix counts
against a vandalized length would report the same event a second and third
time (§38c's one-event-one-finding rule). An entry matching neither digest
proves nothing and its counts ARE compared.

**Rejected: document-only** ("run verify only against closed hours") — an
operator instruction carrying what the tool should know, and the mid-hour
window after a restart is precisely when an anxious operator verifies.
**Rejected: rewriting the sidecar at reopen** — the sidecar is close-time
truth; rewriting it pre-close manufactures a claim no close ever made.

**Verification:** three-subtest fixture (continuation clean; garbage still
integrity, no manifest echo; planted claim still integrity, counts compared),
anti-vacuity guard asserting every compared field differs between prefix and
whole-file truth, then **6/6 aimed mutations caught** — the wrong-anchor
mutant (`preClose` for `digestAtClose`), both dropped conjuncts on the
sidecar side, `tornTail` pinned false, and both dropped conjuncts on the
manifest side. Live proof on the real archive mid-hour, 15:56 UTC: **0
integrity findings, verdict COVERAGE**, the eight `unmanifested` T15 rows the
honest transitional state that the next close heals.

### §38i postscript — the rotation race, same hour, and the rule that owns both

The confirming verify then found the finding's sibling: run as the 16:00
rotation fired (venue `RecvTime` runs ~35s ahead of this laptop's clock, so
the boundary arrived "early"), it scanned the binance T15 files MID-APPEND,
finished walking the archive, and only then loaded the manifest — by which
time the rotation's fresh close-time entries had landed. Stale scan against
fresh entries: eleven `manifest_mismatch` findings whose tell was the
DIRECTION — entries counting MORE than the file held. `capture cat` and
`shasum` acquitted the archive completely: file, sidecar and entry agree
exactly. Bybit read clean only because streams scan alphabetically and its
files were walked after the boundary.

Both production cases — the reopened hour and the rotation race — are one
predicate: **the file as scanned did not end at a Close record.** An entry
can never be convicted by comparison against a moving file, and the next
close's upsert replaces it regardless. So the manifest side's `digestAtClose`
guard is REPLACED by the `endsClosed` gate (scan's final decoded record is a
Close), which subsumes it, subsumes the empty-digest guard (a header-abort
decodes no Close), and is one stated rule instead of three overlapping ones.
The sidecar side keeps the §38i digest rule unchanged — the sidecar is read
at scan time, beside the bytes, so it cannot race the way the manifest does;
its own residual race (a close landing between the walk and the sidecar
read) is milliseconds wide, self-healing on any re-run, and not worth the
read-ordering machinery it would cost. Consequence pinned in the fixture: a
WRONG entry on a still-moving file earns no finding — the planted sidecar
beside it still reports, the claims fail independently. 4/4 aimed mutations
on the gate caught (both directions and the tracking line; two NOBUILD
re-aims on the single-read variable). Live verify after the boundary is the
close-out evidence below.
