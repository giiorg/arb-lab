package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// §32k's third liveness mechanism, and the only one that runs OUT of process:
//
//	arblab capture verify --root R --from T --to T, reporting per (source, hour):
//	record and Data counts, Alive ticks against expected, CaptureSeq continuity
//	within and across files, gap records by cause, CRC failures, digest
//	mismatches, and a verdict of replayable / gapped / absent.
//
// The other two — the first-data assertion and the unconditional Alive tick — are
// inside the capturing process, which is the thing whose death they are supposed
// to detect. That is why this one exists at all, and why it must never share a
// code path with the writer beyond the codec: a checker that agrees with the
// writer because it IS the writer confirms only that the code agrees with itself.
// What is shared here is DecodeRecord and the sliding-window scanner, both of
// which are the format rather than the writer's opinion of it; the digests, the
// counts and the continuity rules are all recomputed from the bytes.
//
// # It returns a report, not an error
//
// A corrupt archive is a successful run. Verify's contract is to DESCRIBE an
// archive, so it walks the whole tree and collects everything, because an
// operator holding one hour of a three-month archive needs the list rather than
// the first item on it. The error return is for a request that cannot be answered
// — an unreadable root, a range with no window in it.
//
// # Read-only, and that is load-bearing rather than polite
//
// The writer's recovery procedure truncates a torn tail (§32e). This must not,
// even though it finds the same tails: the two run at different times for
// different reasons, and a verify that repaired what it found would destroy the
// evidence of the very fault it was invoked to measure. errors.go's TRUNCATED-
// versus-everything-else split is the same distinction one layer down.

// VerifyConfig is a verify request. Every field is required; see Validate.
type VerifyConfig struct {
	// Root is the archive root — the directory holding the v<major> tree, the
	// same value the writer was given.
	Root string

	// From and To bound the windows verify holds the archive to. Only windows
	// lying COMPLETELY inside them are checked, and the set is derived from
	// these two values rather than from the files on disk.
	//
	// That direction is the whole mechanism. A set of expected windows derived
	// from the files present cannot notice a deleted hour, and a deleted hour is
	// one of the four faults §32k's positive control requires verify to name. It
	// also means the operator's range is a CLAIM — "capture should have been
	// running here" — which is the only form in which absence is checkable.
	From time.Time
	To   time.Time

	// RotationPeriod and AliveInterval are the writer's, and they are supplied
	// rather than inferred.
	//
	// Inferring the period from file names would work and is refused: the
	// expected Alive count for a window is period/interval, so a period read
	// back out of the archive would make the coverage check a comparison of the
	// archive against itself. Supplying them makes the check a comparison of the
	// archive against the CONFIG, which is what an operator means by "was
	// capture doing what I told it to".
	RotationPeriod time.Duration
	AliveInterval  time.Duration

	// MinAlivePercent is the Alive-tick coverage a window needs to escape
	// FindingAliveShort, in whole percent of expected, 1..100.
	//
	// Whole percent because §3.6 rule 1 forbids float64 under internal/ and this
	// is a threshold on a ratio of two counts — so the comparison is
	// ticks*100 >= expected*percent, which is exact integer arithmetic and has no
	// rounding direction to get wrong. A percentage that cannot be expressed in
	// whole points is not a distinction anybody operating this tool needs.
	MinAlivePercent uint
}

// Validate reports whether the request can be answered.
func (c VerifyConfig) Validate() error {
	if c.Root == "" {
		return ErrRootUnset
	}
	if c.RotationPeriod <= 0 {
		return fmt.Errorf("rotation period %s: %w", c.RotationPeriod, ErrPeriodRange)
	}
	if c.AliveInterval <= 0 {
		return fmt.Errorf("alive interval %s: %w", c.AliveInterval, ErrIntervalRange)
	}
	if c.MinAlivePercent == 0 || c.MinAlivePercent > 100 {
		return fmt.Errorf("threshold %d%%: %w", c.MinAlivePercent, ErrAlivePercentRange)
	}
	if len(c.windows()) == 0 {
		return fmt.Errorf("%s..%s holds no complete %s window: %w",
			c.From.UTC().Format(time.RFC3339), c.To.UTC().Format(time.RFC3339),
			c.RotationPeriod, ErrVerifyRangeEmpty)
	}
	return nil
}

// windows is every rotation window lying completely inside [From, To].
//
// Completely inside, and the partial ones are DROPPED rather than pro-rated. A
// half-elapsed window has fewer Alive ticks than its period implies for a
// perfectly healthy reason, and pro-rating the expectation would mean computing
// the expectation from how much of the window the archive covers — which is the
// quantity being judged. Dropping it costs the operator one flag and keeps the
// threshold a comparison against config.
func (c VerifyConfig) windows() []time.Time {
	if c.RotationPeriod <= 0 || !c.To.After(c.From) {
		return nil
	}
	var out []time.Time
	w := Window(c.From, c.RotationPeriod)
	if w.Before(c.From) {
		w = w.Add(c.RotationPeriod)
	}
	for ; !w.Add(c.RotationPeriod).After(c.To); w = w.Add(c.RotationPeriod) {
		out = append(out, w)
	}
	return out
}

// Report is what an archive turned out to be.
type Report struct {
	// From and To echo the request, because a report read six weeks later is
	// meaningless without the claim it was checked against.
	From time.Time
	To   time.Time

	// Windows is one row per expected (stream, window), including the absent
	// ones, sorted by stream then window.
	Windows []WindowReport

	// Findings is every finding, flat, in the order they were found. Flat rather
	// than nested under Windows because the consumer that matters is a person
	// reading a list — and because some findings (a stray file) belong to no
	// window at all, which a nested shape would have to hide somewhere.
	Findings []Finding
}

// WindowReport is §32k's per-(source, hour) row.
type WindowReport struct {
	Stream StreamKey
	Window time.Time

	// Path is the archive file, empty when the window is absent.
	Path string

	Verdict Verdict

	// Records is every record in the file; the named counts are subsets.
	Records     uint64
	DataRecords uint64
	AliveTicks  uint64

	// RestRecords counts §32h overlay records. Not coverage evidence — a
	// Rest record is on no schedule verify knows — but it IS a venue answer,
	// which is why a file holding one is not "a dead feed" (§37g).
	RestRecords uint64

	// AliveExpected is what the rotation period and Alive interval imply for a
	// fully covered window — a constant from the config, never from the file.
	AliveExpected uint64

	// Gaps counts recorded gaps by cause, holding only the causes that occurred.
	Gaps map[Cause]uint64

	// FirstSeq and LastSeq bound the Data CaptureSeqs in the file. Both zero for
	// a window whose file holds no Data at all.
	FirstSeq uint64
	LastSeq  uint64
}

// AliveOK reports whether the window's tick count clears a whole-percent
// threshold. Integer arithmetic throughout — see VerifyConfig.MinAlivePercent.
func (w WindowReport) AliveOK(minPercent uint) bool {
	return w.AliveTicks*100 >= w.AliveExpected*uint64(minPercent)
}

// Counts returns how many findings of each class the report holds.
func (r *Report) Counts() (integrity, coverage int) {
	for _, f := range r.Findings {
		switch f.Class() {
		case ClassIntegrity:
			integrity++
		case ClassCoverage:
			coverage++
		case ClassUnspecified:
		}
	}
	return integrity, coverage
}

// ExitClass is the exit code §32k assigns this report, as a class:
// ClassIntegrity for 1, ClassCoverage for 2, ClassUnspecified for 0.
//
// Integrity wins when both are present, and the precedence is the point rather
// than a tie-break: "the archive is lying" subsumes "the archive is thin",
// because a thinness measured over bytes that contradict themselves is not a
// measurement. Reporting 2 there would send an operator to widen a capture window
// when what they need to do is stop trusting the file.
func (r *Report) ExitClass() FindingClass {
	integrity, coverage := r.Counts()
	switch {
	case integrity > 0:
		return ClassIntegrity
	case coverage > 0:
		return ClassCoverage
	}
	return ClassUnspecified
}

// archiveFile is one file found by the tree walk.
type archiveFile struct {
	path      string
	window    time.Time
	hasDigest bool
}

// Verify walks the archive under cfg.Root and reports what it found.
func Verify(cfg VerifyConfig) (*Report, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	streams, strays, err := discover(cfg.Root)
	if err != nil {
		return nil, err
	}

	rep := &Report{From: cfg.From, To: cfg.To, Findings: strays}
	windows := cfg.windows()
	first, last := windows[0], windows[len(windows)-1]

	// facts is what the manifest pass (§38c) compares entries against, keyed
	// the way entries key themselves: root-relative slash paths.
	facts := make(map[string]manifestFact)

	keys := make([]StreamKey, 0, len(streams))
	for k := range streams {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b StreamKey) int { return strings.Compare(a.String(), b.String()) })

	for _, key := range keys {
		files := streams[key]
		slices.SortFunc(files, func(a, b archiveFile) int { return a.window.Compare(b.window) })

		// Parsing starts one file EARLIER than the range, because the first
		// in-range file's Open record makes a claim about its predecessor and
		// checking it needs that predecessor's last sequence. A verify that
		// started exactly at --from would skip the one cross-file check per run
		// that a narrowed range makes most likely to matter.
		start := 0
		for i, f := range files {
			if f.window.Before(first) {
				start = i
			}
		}

		st := chainState{}
		found := make(map[time.Time]*WindowReport, len(files))
		for _, f := range files[start:] {
			if f.window.After(last) {
				break
			}
			inRange := !f.window.Before(first)
			scan, findings, err := verifyFile(cfg, key, f, st)
			if err != nil {
				return nil, err
			}
			if inRange {
				if rel, err := filepath.Rel(cfg.Root, f.path); err == nil {
					facts[filepath.ToSlash(rel)] = manifestFact{
						stream: key, window: f.window, path: f.path, scan: scan,
					}
				}
				row := scan.window(cfg, key, f)
				// Suppressed on an aborted walk for the reason NoClose and NoData
				// are: the tick count is a count of the records the walk reached,
				// so reporting it short would be reporting the corruption again
				// as a coverage complaint about an hour nobody has read.
				if !scan.aborted && !row.AliveOK(cfg.MinAlivePercent) {
					findings = append(findings, Finding{
						Kind: FindingAliveShort, Stream: key, Window: f.window,
						Path: f.path, Offset: -1,
						Detail: fmt.Sprintf("%d Alive ticks against %d expected, below %d%%",
							row.AliveTicks, row.AliveExpected, cfg.MinAlivePercent),
					})
				}
				found[f.window] = &row
				rep.Findings = append(rep.Findings, findings...)
				st.holes = append(st.holes, scan.holes...)
			}
			st.gapRanges = append(st.gapRanges, scan.gapRanges...)
			st.hasPredecessor = true
			st.lastSeq, st.known = scan.chainLast, scan.chainKnown
		}

		// Holes are reconciled against gap records for the whole STREAM rather
		// than per file, because the two can legitimately land on opposite sides
		// of a rotation boundary: Writer.Gap does not rotate, so an overflow that
		// ends exactly at an hour boundary writes its gap record into the old
		// file and its hole into the new one. Reconciling per file would report
		// that as unexplained — an integrity finding, exit 1 — for an archive
		// that recorded everything correctly.
		rep.Findings = append(rep.Findings, reconcileHoles(st)...)

		for _, w := range windows {
			row, ok := found[w]
			if !ok {
				rep.Windows = append(rep.Windows, WindowReport{
					Stream: key, Window: w, Verdict: VerdictAbsent,
					AliveExpected: expectedAliveTicks(w, cfg.RotationPeriod, cfg.AliveInterval),
				})
				rep.Findings = append(rep.Findings, Finding{
					Kind: FindingWindowAbsent, Stream: key, Window: w, Offset: -1,
					Detail: "no archive file for a window the request expected",
				})
				continue
			}
			rep.Windows = append(rep.Windows, *row)
		}
	}

	manifestFindings, err := checkManifestDays(cfg, first, last, facts)
	if err != nil {
		return nil, err
	}
	rep.Findings = append(rep.Findings, manifestFindings...)

	// The verdict per window is decided last, because a hole reconciled at the
	// stream level can turn a window that looked clean into a gapped one.
	assignVerdicts(rep)
	return rep, nil
}

// manifestFact is one in-range file's recomputed truth, for the manifest pass.
type manifestFact struct {
	stream StreamKey
	window time.Time
	path   string
	scan   fileScan
}

// checkManifestDays holds every day manifest in range against the files, and
// the files against it — §38c. Both directions matter and they catch different
// events: an entry with no file is the deletion this layer exists to see; a
// closed file with no entry is §38a's crash window, or a day from before the
// layer existed.
func checkManifestDays(
	cfg VerifyConfig, first, last time.Time, facts map[string]manifestFact,
) ([]Finding, error) {
	var out []Finding

	// closedByDay is which in-range files a manifest OUGHT to list: the cleanly
	// closed ones, grouped by their window's UTC day.
	closedByDay := make(map[string][]string)
	for rel, fact := range facts {
		if fact.scan.sawClose && !fact.scan.aborted {
			day := fact.window.UTC().Format(manifestDayLayout)
			closedByDay[day] = append(closedByDay[day], rel)
		}
	}

	for day := Window(first, 24*time.Hour); !day.After(last); day = day.AddDate(0, 0, 1) {
		dayStr := day.Format(manifestDayLayout)
		m, err := loadManifest(cfg.Root, dayStr)
		if errors.Is(err, ErrManifestCorrupt) {
			out = append(out, Finding{
				Kind: FindingManifestUnreadable, Path: manifestPath(cfg.Root, dayStr),
				Offset: -1, Detail: err.Error(),
			})
			continue
		}
		if err != nil {
			return nil, err
		}
		if m == nil {
			// One finding per day, not per file (§38c) — and only for a day
			// that HAS something a manifest should be listing, because a day
			// with no closed files in range makes no claim to check.
			if n := len(closedByDay[dayStr]); n > 0 {
				out = append(out, Finding{
					Kind: FindingManifestAbsent, Path: manifestPath(cfg.Root, dayStr),
					Offset: -1,
					Detail: fmt.Sprintf(
						"no manifest for a day holding %d closed files — run the backfill", n),
				})
			}
			continue
		}

		listed := make(map[string]bool, len(m.Entries))
		for _, e := range m.Entries {
			w, err := time.Parse(time.RFC3339, e.Window)
			if err != nil {
				out = append(out, Finding{
					Kind: FindingManifestUnreadable, Path: manifestPath(cfg.Root, dayStr),
					Offset: -1,
					Detail: fmt.Sprintf("entry %s: window %q does not parse", e.Path, e.Window),
				})
				continue
			}
			listed[e.Path] = true
			if w.Before(first) || w.After(last) {
				continue
			}
			out = append(out, checkManifestEntry(cfg, e, w, facts)...)
		}

		days := closedByDay[dayStr]
		slices.Sort(days)
		for _, rel := range days {
			if listed[rel] {
				continue
			}
			fact := facts[rel]
			out = append(out, Finding{
				Kind: FindingUnmanifested, Stream: fact.stream, Window: fact.window,
				Path: fact.path, Offset: -1,
				Detail: "closed file absent from the day's manifest — " +
					"the §38a crash window, repairable by the labeled backfill",
			})
		}
	}
	return out, nil
}

// checkManifestEntry compares one in-range entry against the file's recomputed
// truth.
//
// The digest is compared whenever the file was READ — fullDigest covers the
// complete file past an aborted walk — while the counts are compared only for
// a cleanly closed, cleanly read file: an aborted walk's counts describe the
// records before the corruption, and a file the writer re-opened has moved on
// from the entry legitimately, with no_close or the next close's upsert owning
// that story.
func checkManifestEntry(
	cfg VerifyConfig, e ManifestEntry, w time.Time, facts map[string]manifestFact,
) []Finding {
	fact, ok := facts[e.Path]
	if !ok {
		var key StreamKey
		if k, _, err := ParseFilePath(cfg.Root, filepath.Join(cfg.Root, filepath.FromSlash(e.Path))); err == nil {
			key = k
		}
		return []Finding{{
			Kind: FindingManifestMissingFile, Stream: key, Window: w,
			Path: filepath.Join(cfg.Root, filepath.FromSlash(e.Path)), Offset: -1,
			Detail: fmt.Sprintf("manifest (%s entry, closed %s) lists a file that is gone",
				e.Source, e.ClosedAt),
		}}
	}
	var out []Finding
	mismatch := func(format string, args ...any) {
		out = append(out, Finding{
			Kind: FindingManifestMismatch, Stream: fact.stream, Window: fact.window,
			Path: fact.path, Offset: -1, Detail: fmt.Sprintf(format, args...),
		})
	}
	// Suppressed when the sidecar already reported the bytes changed: the
	// manifest and the sidecar carry the same close-time digest, so a file
	// modified after close would otherwise earn two findings for one event.
	// What this row is FOR is the case where the sidecar cannot testify — it
	// was deleted, or it is the manifest entry itself that went stale (§38a's
	// crash window on a re-close). The empty-digest guard is the header-abort
	// path, where the walk hashed nothing: comparing a real claim against an
	// empty hash would re-report file_header as a mismatch with no digest in it.
	if fact.scan.fullDigest != "" && e.SHA256 != fact.scan.fullDigest && !fact.scan.sidecarMismatch {
		mismatch("manifest says %s, the file hashes to %s", e.SHA256, fact.scan.fullDigest)
	}
	if fact.scan.aborted || !fact.scan.sawClose {
		return out
	}
	if e.Records != fact.scan.records {
		mismatch("manifest counts %d records, the file holds %d", e.Records, fact.scan.records)
	}
	if e.Gaps != fact.scan.gapRecords {
		mismatch("manifest counts %d gap records, the file holds %d", e.Gaps, fact.scan.gapRecords)
	}
	if e.LastSeq != fact.scan.lastSeq {
		mismatch("manifest says last CaptureSeq %d, the file's Data ends at %d",
			e.LastSeq, fact.scan.lastSeq)
	}
	return out
}

// assignVerdicts fills each window's verdict from the findings attached to it.
//
// A window is replayable only if NOTHING was found for it — integrity findings
// included. The verdict is about replay and the class is about trust, and a
// window whose bytes contradict themselves is not replayable whatever else is
// true of it.
//
// Every reason to be gapped therefore has to be a FINDING, including short Alive
// coverage. Deciding that one here from the counts instead was the first version,
// and it made FindingAliveShort a declared kind nothing emitted: the verdict came
// out right, the finding list did not mention the only thing wrong with the
// window, and an exit code derived from the list read clean. It is the unreachable-
// sentinel defect one layer out: a declared enum member no code path produces,
// invisible to the compiler and to coverage alike, and reported as handled.
func assignVerdicts(rep *Report) {
	type at struct {
		stream string
		window time.Time
	}
	flagged := make(map[at]bool, len(rep.Findings))
	for _, f := range rep.Findings {
		if f.Kind == FindingWindowAbsent {
			continue
		}
		flagged[at{f.Stream.String(), f.Window}] = true
	}
	for i := range rep.Windows {
		w := &rep.Windows[i]
		if w.Verdict == VerdictAbsent {
			continue
		}
		if flagged[at{w.Stream.String(), w.Window}] {
			w.Verdict = VerdictGapped
			continue
		}
		w.Verdict = VerdictReplayable
	}
}

// discover walks the archive root and groups its files by stream.
//
// Everything it cannot account for becomes a FindingStrayFile rather than being
// skipped, and that is the direction that matters: a walk that ignored what it did
// not recognise would report a clean archive for a tree somebody had emptied into.
func discover(root string) (map[StreamKey][]archiveFile, []Finding, error) {
	byStream := make(map[StreamKey][]archiveFile)
	var findings []Finding

	// Digests are collected in a first pass rather than stat'ed per archive
	// file, so a sidecar with no archive beside it is visible. That orphan is
	// the tell for a partially completed copy — the case §32a says is the one
	// failure that cannot be fixed afterwards.
	digests := make(map[string]bool)
	var archives []string

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The manifest subtree is part of the format, not a stray (§38a).
			// Skipped WHOLESALE — its files are read by the manifest pass, which
			// classifies an unreadable one precisely (manifest_unreadable) where
			// this walk could only call it a stray.
			if p == manifestDir(root) {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(p)
		switch {
		case base == lockName:
			return nil
		case strings.HasSuffix(base, archiveExt+digestExt):
			digests[strings.TrimSuffix(p, digestExt)] = true
		case strings.HasSuffix(base, archiveExt):
			archives = append(archives, p)
		default:
			findings = append(findings, Finding{
				Kind: FindingStrayFile, Path: p, Offset: -1,
				Detail: "not an archive file, a digest sidecar or a writer lock",
			})
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("capture: walk %s: %w", root, err)
	}

	slices.Sort(archives)
	for _, p := range archives {
		key, window, err := ParseFilePath(root, p)
		if err != nil {
			findings = append(findings, Finding{
				Kind: FindingStrayFile, Path: p, Offset: -1,
				Detail: err.Error(),
			})
			continue
		}
		byStream[key] = append(byStream[key], archiveFile{
			path: p, window: window, hasDigest: digests[p],
		})
		delete(digests, p)
	}
	orphans := make([]string, 0, len(digests))
	for p := range digests {
		orphans = append(orphans, p)
	}
	slices.Sort(orphans)
	for _, p := range orphans {
		findings = append(findings, Finding{
			Kind: FindingStrayFile, Path: p + digestExt, Offset: -1,
			Detail: "digest sidecar with no archive file beside it",
		})
	}
	return byStream, findings, nil
}

// expectedAliveTicks is how many unconditional ticks a fully covered window
// should hold: the number of AliveInterval boundaries inside [window, window+period).
//
// Counted as boundaries rather than as period/interval, because the two differ
// whenever the interval does not divide the period — and §32l's collapse table
// requires those two to be COPRIME in every fixture, so the disagreement is the
// normal case here rather than an edge one. period/interval would be off by one
// on some windows and right on others, which is the worst available failure: a
// threshold that trips on an arbitrary subset of healthy hours.
//
// # The grid is time.Time.Truncate's, and it is NOT the Unix epoch's
//
// This is the whole reason the function is written with time arithmetic instead
// of the obvious division over UnixNano. Writer.nextTick anchors ticks with
// time.Time.Truncate, which rounds down to a multiple of d SINCE THE ZERO TIME
// (January 1, year 1) — not since the Unix epoch. The epoch sits 62135596800
// seconds after the zero time, and 62135596800 mod 420 is 60, so a 7-minute grid
// anchored at the epoch is shifted one minute from the one the writer uses, and
// one boundary crosses each hour edge. The count then reads one high for one
// window and one low for the next, for a perfectly healthy archive.
//
// The two grids agree exactly when the interval divides 60 seconds — so with the
// one-minute Alive interval anybody would reach for in a fixture, the mistake is
// INVISIBLE. §32l's coprime requirement is what exposed it.
func expectedAliveTicks(window time.Time, period, interval time.Duration) uint64 {
	if period <= 0 || interval <= 0 {
		return 0
	}
	first := window.Truncate(interval)
	if first.Before(window) {
		first = first.Add(interval)
	}
	end := window.Add(period)
	if !first.Before(end) {
		return 0
	}
	// Boundaries at first, first+interval, … strictly below end. The -1 is what
	// excludes a boundary landing exactly on end, which belongs to the next
	// window and is where an off-by-one would be least visible.
	return uint64((end.Sub(first)-1)/interval) + 1
}

// seqRange is an inclusive CaptureSeq range.
type seqRange struct {
	from, to uint64
}

// holeAt is a sequence hole and where it was found, so a hole reconciled at the
// stream level still reports against its own file.
type holeAt struct {
	seqRange
	stream StreamKey
	window time.Time
	path   string
}

// chainState is what one file's verification hands to the next.
type chainState struct {
	// hasPredecessor reports that an earlier file exists for this stream, which
	// is what turns a prev_last_seq of zero from "the first file ever" into a
	// recorded restart.
	hasPredecessor bool

	// lastSeq is where the predecessor ended, and known reports whether that is
	// trustworthy — it is not, after a file the walk could not read to the end.
	lastSeq uint64
	known   bool

	// holes and gapRanges accumulate across the stream's files, for the
	// boundary-straddling reconciliation Verify's comment describes.
	holes     []holeAt
	gapRanges []seqRange
}

// reconcileHoles classifies every sequence hole as explained or not.
//
// §32a's verdict line, mechanically: a recorded gap is not an error; an
// unexplained silence is. A hole is explained when some recorded gap's range
// covers it completely — partial cover is not enough, because a gap claiming
// three lost frames beside a hole of thirty describes a different event.
func reconcileHoles(st chainState) []Finding {
	var out []Finding
	for _, h := range st.holes {
		explained := slices.ContainsFunc(st.gapRanges, func(g seqRange) bool {
			return g.from <= h.from && g.to >= h.to
		})
		kind, detail := FindingSeqHoleUnexplained, "no recorded gap covers it"
		if explained {
			kind, detail = FindingSeqHole, "covered by a recorded gap"
		}
		out = append(out, Finding{
			Kind: kind, Stream: h.stream, Window: h.window, Path: h.path, Offset: -1,
			Detail: fmt.Sprintf("CaptureSeq %d..%d missing: %s", h.from, h.to, detail),
		})
	}
	return out
}

// fileScan is one archive file's contents, as recomputed from its bytes.
type fileScan struct {
	records     uint64
	dataRecords uint64
	aliveTicks  uint64
	restRecords uint64
	gaps        map[Cause]uint64

	firstSeq uint64
	lastSeq  uint64

	holes     []holeAt
	gapRanges []seqRange

	chainLast  uint64
	chainKnown bool

	// aborted reports that the walk stopped at bytes that were present and
	// wrong, so every count below is a count of the records BEFORE that point
	// rather than of the file.
	aborted bool

	// sawClose, fullDigest and gapRecords are what the manifest pass (§38c)
	// compares an entry against. fullDigest covers the COMPLETE file — it is
	// computed past an aborted walk, like the sidecar check — so the digest
	// comparison survives corruption that the count comparisons do not.
	// gapRecords counts Gap records by KIND, parsed or not, because that is
	// what the writer's own per-file counter counted. sidecarMismatch reports
	// that checkDigests already found the bytes disagreeing with the sidecar,
	// which is what keeps one changed file from being reported twice.
	sawClose        bool
	fullDigest      string
	gapRecords      uint64
	sidecarMismatch bool
}

// window renders a scan as the report row for its (stream, window).
func (s fileScan) window(cfg VerifyConfig, key StreamKey, f archiveFile) WindowReport {
	return WindowReport{
		Stream: key, Window: f.window, Path: f.path,
		Records: s.records, DataRecords: s.dataRecords, AliveTicks: s.aliveTicks,
		RestRecords:   s.restRecords,
		AliveExpected: expectedAliveTicks(f.window, cfg.RotationPeriod, cfg.AliveInterval),
		Gaps:          s.gaps,
		FirstSeq:      s.firstSeq, LastSeq: s.lastSeq,
	}
}

// verifyFile walks one archive file and reports what it holds and what is wrong
// with it.
//
// The error return is for I/O only. Every archive-level fault is a finding, and
// the walk continues past as many of them as it can — a file whose Open record
// does not parse still has its Alive ticks counted, because the operator asking
// "was capture running" is not helped by a report that stopped at the first
// surprise.
//
// It stops on exactly one class of fault: bytes that are present and wrong.
// There is no resynchronization after a bad record, because a frame's length
// lives at its front and finding the next boundary would mean guessing — so the
// finding names the offset and the remaining checks that depend on reading
// further are SKIPPED rather than reported as additional faults. A corrupt file
// yields one precise finding and not five derived ones.
func verifyFile(cfg VerifyConfig, key StreamKey, f archiveFile, st chainState) (fileScan, []Finding, error) {
	scan := fileScan{gaps: make(map[Cause]uint64)}
	var findings []Finding
	add := func(kind FindingKind, offset int64, format string, args ...any) {
		findings = append(findings, Finding{
			Kind: kind, Stream: key, Window: f.window, Path: f.path, Offset: offset,
			Detail: fmt.Sprintf(format, args...),
		})
	}

	fh, err := os.Open(f.path)
	if err != nil {
		return scan, nil, fmt.Errorf("capture: open %s: %w", f.path, err)
	}
	defer func() { _ = fh.Close() }()

	digest := sha256.New()
	s := &scanner{f: fh, backing: make([]byte, scanBufInit)}
	if err := s.ensure(); err != nil {
		return scan, nil, fmt.Errorf("capture: read %s: %w", f.path, err)
	}
	headerLen, err := ParseFileHeader(s.pending())
	if err != nil {
		// Aborted, and this is the MOST aborted a walk can be — not one record
		// was read. The flag was added for the mid-file case first and missing
		// here produced exactly the defect it was added to prevent, one branch
		// over: a file whose magic is wrong reported "0 Alive ticks against 8
		// expected" beside it, which is a coverage claim about bytes verify
		// declined to read. Same fix, its mirror.
		scan.aborted = true
		add(FindingFileHeader, 0, "%v", err)
		return scan, findings, nil
	}
	digest.Write(s.pending()[:headerLen])
	offset := int64(headerLen)
	s.lo += headerLen

	running, runningKnown := st.lastSeq, st.known
	var opens int
	var aborted, sawClose bool
	var closeSeq uint64
	preClose := ""

	for {
		if err := s.ensure(); err != nil {
			return scan, nil, fmt.Errorf("capture: read %s: %w", f.path, err)
		}
		if len(s.pending()) == 0 {
			break
		}
		rec, used, err := DecodeRecord(s.pending())
		if err != nil {
			switch {
			case errors.Is(err, ErrTruncated):
				add(FindingTornTail, offset, "%d trailing bytes end inside a record", len(s.pending()))
			case errors.Is(err, ErrChecksum):
				add(FindingChecksum, offset, "%v", err)
				aborted = true
			default:
				add(FindingMalformed, offset, "%v", err)
				aborted = true
			}
			break
		}
		if rec.Kind == RecordKindClose {
			// Snapshotted before the Close frame is hashed, because the field it
			// carries cannot cover the record carrying it (writer.go's
			// closePayload). Taken here rather than for every record so the
			// hash state is copied once per file instead of once per frame.
			preClose = hex.EncodeToString(digest.Sum(nil))
		}
		digest.Write(s.pending()[:used])
		scan.records++

		switch rec.Kind {
		case RecordKindOpen:
			var op openPayload
			if err := json.Unmarshal(rec.Payload, &op); err != nil {
				add(FindingMalformed, offset, "Open payload does not parse: %v", err)
				break
			}
			findings = append(findings, checkOpenIdentity(key, f, offset, op)...)
			running, runningKnown = checkOpenContinuity(
				add, offset, op, opens, st.hasPredecessor, running, runningKnown)
			opens++
		case RecordKindData:
			if scan.dataRecords == 0 {
				scan.firstSeq = rec.CaptureSeq
			}
			scan.dataRecords++
			if runningKnown {
				switch {
				case rec.CaptureSeq <= running:
					add(FindingSeqNotAscending, offset,
						"CaptureSeq %d does not follow %d", rec.CaptureSeq, running)
				case rec.CaptureSeq > running+1:
					scan.holes = append(scan.holes, holeAt{
						seqRange: seqRange{from: running + 1, to: rec.CaptureSeq - 1},
						stream:   key, window: f.window, path: f.path,
					})
				}
			}
			running, runningKnown = rec.CaptureSeq, true
			scan.lastSeq = rec.CaptureSeq
		case RecordKindAlive:
			scan.aliveTicks++
		case RecordKindGap:
			scan.gapRecords++
			var g Gap
			if err := json.Unmarshal(rec.Payload, &g); err != nil {
				add(FindingMalformed, offset, "Gap payload does not parse: %v", err)
				break
			}
			if err := g.Validate(); err != nil {
				add(FindingMalformed, offset, "Gap payload is not self-consistent: %v", err)
				break
			}
			scan.gaps[g.Cause]++
			add(FindingGapRecord, offset, "recorded gap: %s", detailOf(g))
			findings[len(findings)-1].Cause = g.Cause
			if g.Count > 0 {
				scan.gapRanges = append(scan.gapRanges, seqRange{from: g.From, to: g.To})
			}
		case RecordKindClose:
			sawClose = true
			var cp closePayload
			if err := json.Unmarshal(rec.Payload, &cp); err != nil {
				add(FindingMalformed, offset, "Close payload does not parse: %v", err)
				break
			}
			closeSeq = cp.LastSeq
			if cp.SHA256 != preClose {
				add(FindingDigestMismatch, offset,
					"Close record says %s, the bytes before it hash to %s", cp.SHA256, preClose)
			}
			if want := scan.records - 1; cp.RecordCount != want {
				add(FindingRecordCount, offset,
					"Close record counts %d records, the file holds %d before it",
					cp.RecordCount, want)
			}
			if runningKnown && cp.LastSeq != running {
				add(FindingCrossFileBreak, offset,
					"Close record says last CaptureSeq %d, the file's records end at %d",
					cp.LastSeq, running)
			}
		case RecordKindRest:
			// Not coverage evidence — no expected count to fall short of —
			// but counted apart from records, because a Rest record is a
			// venue answer and its presence is what keeps an overlay-only
			// file from reading as a dead feed below (§37g).
			scan.restRecords++
		case RecordKindClockStep:
			// Counted in records and nothing more. A clock-step note is
			// provenance for a reader at step 5, not coverage evidence: it
			// is not emitted on a schedule, so it has no expected count to
			// fall short of.
		case RecordKindUnspecified:
			// DecodeRecord refuses it (ErrKindUnspecified), so reaching here
			// would mean the codec accepted a record Record.Validate rejects.
			add(FindingMalformed, offset, "record kind is unset")
		}
		offset += int64(used)
		s.lo += used
	}

	// The remaining bytes are hashed whatever stopped the walk, so the sidecar
	// check is a comparison against the COMPLETE file. That check needs no
	// decoding, which is why it survives an aborted walk while the Close
	// record's own digest does not.
	digest.Write(s.pending())
	if _, err := io.Copy(digest, fh); err != nil {
		return scan, nil, fmt.Errorf("capture: read %s: %w", f.path, err)
	}
	scan.sawClose = sawClose
	scan.fullDigest = hex.EncodeToString(digest.Sum(nil))
	digestFindings := checkDigests(key, f, sawClose, scan.fullDigest)
	for _, df := range digestFindings {
		if df.Kind == FindingDigestMismatch {
			scan.sidecarMismatch = true
		}
	}
	findings = append(findings, digestFindings...)

	// Both are skipped when the walk aborted, because both are statements about
	// records past the corruption — a Close may well be sitting there. Reporting
	// them would be reporting the corruption again under two other names. They
	// are INDEPENDENT of each other, though: a laptop closed mid-capture on a
	// dead feed has neither a Close record nor a single venue frame, and the two
	// facts send an operator to different places.
	if !aborted {
		if !sawClose {
			add(FindingNoClose, -1,
				"no Close record: the run that owned this file did not stop cleanly")
		}
		// A Rest record satisfies this check (§37g): any recorded ask — rows,
		// an empty array, a 429, even a recorded transport failure — is the
		// stream doing its job, and an overlay stream never holds Data. What
		// the finding means is unchanged: alive, and nothing ever answered.
		if scan.dataRecords == 0 && scan.restRecords == 0 {
			add(FindingNoData, -1,
				"%d records, %d Alive ticks and not one venue frame", scan.records, scan.aliveTicks)
		}
	}

	// The chain carries the CLOSE RECORD'S claim when there is one, not the last
	// sequence the records actually reached — and the two differ exactly when the
	// Close record lies, which is the case that decides it.
	//
	// The successor's prev_last_seq is written by the writer from the same value
	// its Close record carries (writer.closeFile sets prevLastSeq = lastSeq), so a
	// successor that agrees with a lying Close is the EXPECTED pairing. Carrying
	// the true value forward instead would report the same single fault twice: once
	// against the Close record, once against a successor that is doing exactly what
	// the writer would do.
	scan.chainLast, scan.chainKnown = running, runningKnown
	if sawClose {
		scan.chainLast, scan.chainKnown = closeSeq, true
	}
	scan.aborted = aborted
	if aborted {
		scan.chainKnown = false
	}
	return scan, findings, nil
}

// checkOpenIdentity compares an Open record's structural fields against the path
// the file was found at.
//
// This is the check writer.go's openPayload was widened for: the archive path is
// otherwise the only record of which stream a file belongs to, because a frame
// carries no SourceID, and one `mv` by a sync script makes a file anonymous. Six
// fields per hour buy a file that can identify itself — and they buy nothing at
// all unless something compares them.
func checkOpenIdentity(key StreamKey, f archiveFile, offset int64, op openPayload) []Finding {
	// A distinct comparable struct rather than two openPayloads, because
	// openPayload holds a Provenance holding []byte and so is not comparable —
	// and comparing field by field instead would be seven `||` clauses that a
	// later field addition would silently not join. This shape makes the added
	// field a compile-time hole in two composite literals.
	type identity struct {
		venue, product, contract, symbol, stream, topic, window string
	}
	vi := key.Source.Instrument
	want := identity{
		venue: string(vi.Venue), product: string(vi.Product), contract: string(vi.Contract),
		symbol: vi.Symbol, stream: string(key.Source.Stream), topic: key.Topic,
		window: f.window.Format(time.RFC3339),
	}
	got := identity{
		venue: op.Venue, product: op.Product, contract: op.Contract,
		symbol: op.Symbol, stream: op.Stream, topic: op.Topic,
		window: op.Window,
	}
	if got == want {
		return nil
	}
	return []Finding{{
		Kind: FindingStrayFile, Stream: key, Window: f.window, Path: f.path, Offset: offset,
		Detail: fmt.Sprintf("Open record names %+v; the path says %+v", got, want),
	}}
}

// checkOpenContinuity checks an Open record's prev_last_seq claim and returns the
// sequence state to carry forward.
//
// Zero is the documented "no predecessor" (writer.go's openPayload), and it is
// what a fresh writer always writes — it cannot know where the previous hour
// ended without reading that file. So zero beside a predecessor is a recorded
// restart rather than a break, and the state is reset to unknown so the file's
// first Data record does not read as a hole running back to sequence one.
func checkOpenContinuity(
	add func(FindingKind, int64, string, ...any),
	offset int64, op openPayload, opens int, hasPredecessorFile bool,
	running uint64, runningKnown bool,
) (uint64, bool) {
	hasPredecessor := hasPredecessorFile || opens > 0
	switch {
	case op.PrevLastSeq == 0:
		if hasPredecessor {
			add(FindingRestartBoundary, offset,
				"declares no predecessor while an earlier file or run exists")
		}
		return 0, false
	case !hasPredecessor:
		add(FindingCrossFileBreak, offset,
			"claims predecessor sequence %d with no earlier file for this stream", op.PrevLastSeq)
	case !runningKnown:
		add(FindingContinuityUnknown, offset,
			"claims predecessor sequence %d, which cannot be checked", op.PrevLastSeq)
	case op.PrevLastSeq != running:
		add(FindingCrossFileBreak, offset,
			"claims predecessor sequence %d, the predecessor ends at %d", op.PrevLastSeq, running)
	}
	// Adopted whatever the claim turned out to be, so one break is reported once
	// and the file's own records are still checked against each other. Carrying
	// the old value forward instead would report a second, derived hole at the
	// first Data record.
	return op.PrevLastSeq, true
}

// checkDigests compares the `.sha256` sidecar against the bytes.
//
// # The sidecar is checked whenever it EXISTS, and the missing one is only a
// # finding for a closed file
//
// The asymmetry is what makes the pair discriminate, and the case that forced it
// is a file closed cleanly and then TRUNCATED — by a copy that ran out of disk,
// which §32a names as the failure that cannot be fixed afterwards. Such a file
// has lost its Close record, so a sidecar check scoped to `sawClose` would skip
// it and report a torn tail: a coverage finding, exit 2, "the archive is honest
// and thin", for an archive that has been damaged. Checking the sidecar
// unconditionally catches it as the digest mismatch it is.
//
// The other direction is the ordinary crash: no Close record and no sidecar,
// because the writer writes the two together. Nothing is wrong with that file
// beyond the crash, and FindingNoClose is that crash — so a missing sidecar is
// only reported where one should exist.
func checkDigests(key StreamKey, f archiveFile, sawClose bool, full string) []Finding {
	finding := func(kind FindingKind, detail string) []Finding {
		return []Finding{{
			Kind: kind, Stream: key, Window: f.window, Path: f.path, Offset: -1, Detail: detail,
		}}
	}
	if !f.hasDigest {
		if sawClose {
			return finding(FindingDigestMissing,
				"closed file with no .sha256 sidecar to check a copy against")
		}
		return nil
	}
	body, err := os.ReadFile(f.path + digestExt)
	if err != nil {
		return finding(FindingDigestMissing, err.Error())
	}
	fields := strings.Fields(string(body))
	if len(fields) != 2 || fields[1] != filepath.Base(f.path) {
		return finding(FindingStrayFile,
			fmt.Sprintf("sidecar %q is not `<sha256>  %s`", strings.TrimSpace(string(body)),
				filepath.Base(f.path)))
	}
	if fields[0] != full {
		return finding(FindingDigestMismatch,
			fmt.Sprintf("sidecar says %s, the file hashes to %s", fields[0], full))
	}
	return nil
}

// detailOf renders a gap for a finding's detail line, naming whichever of the two
// shapes it carries — a sequence range or a byte count. Gap holds both because
// the causes need different ones (gap.go), and printing the empty half would
// report "0 bytes" for a hole and "0..0" for a torn tail.
func detailOf(g Gap) string {
	switch {
	case g.Count > 0:
		return fmt.Sprintf("%s, CaptureSeq %d..%d (%d)", g.Cause, g.From, g.To, g.Count)
	case g.Bytes > 0:
		return fmt.Sprintf("%s, %d bytes", g.Cause, g.Bytes)
	}
	return g.Cause.String()
}
