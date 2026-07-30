package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"time"
)

// TimeSource is the three time readings capture takes, and it is a port declared
// HERE rather than platform/clock.Clock used directly, for one reason per method.
//
// Now is §3.6 rule 5's injected stamp. The escape (`//notimenow:allow`) covers
// transport DEADLINES, not stamps, and §32i draws that line sharply: a socket
// read deadline is genuinely ambient and never runs under replay, while a
// RecvTime is the timestamp §3.4 measures freshness and cross-leg skew against.
//
// Backsteps is not on clock.Clock at all — it is a *clock.Wall accessor (§32i,
// §33d), because absorbing a backwards step is right for the engine and wrong
// for an archive. Capture polls it and records what the clamp swallowed.
//
// Mono carries a CONTRACT that a signature cannot state, so it is stated here: it
// must be a reading that keeps advancing across a wall-clock step. Deriving it
// from Now would satisfy the interface and defeat the field — Wall.Now is clamped
// monotonic, so during a backstep a derived Mono stands still, which is precisely
// the window Mono exists to measure. A record's Mono would then be redundant with
// its RecvTime exactly when RecvTime is untrustworthy.
//
// The live adapter is deliberately NOT in this slice. It cannot live in this
// package (engine/, so notimenow forbids the ambient read) and its two candidate
// homes — a Mono method on platform/clock.Wall, or an adapter in the
// internal/app/capture composition root — are a slice C decision, where the
// process that needs it first exists. Slice B is driven by a fake source under a
// test double, per §32m.
type TimeSource interface {
	Now() time.Time
	Mono() time.Duration
	Backsteps() (count uint64, total time.Duration)
}

// WriterConfig is everything a Writer needs. Every field is required.
//
// No defaults here, and that split is deliberate: §32j puts the defaults in
// platform/config, where a value has a name, a documented meaning and a
// restart-required tag. A default applied silently at the constructor is a value
// with no name — and the two failures it would paper over are a rotation period
// of zero (rotate on every record) and an alive interval of zero (a tick per
// pump), both of which fail as an archive that is enormous and looks healthy
// rather than as a refusal naming the field.
type WriterConfig struct {
	// Root is the archive root. The v<major> segment is appended by the layout,
	// so two format majors never share a tree.
	Root string

	// Key is the (SourceID, Topic) this writer owns — §32c's capture key.
	Key StreamKey

	// Clock supplies every timestamp the writer mints. Data records are stamped
	// by the reader before they arrive here (§32d).
	Clock TimeSource

	// Provenance is the caller's half of the Open record: which build, which
	// config, which endpoint. Six weeks later this is the only thing that
	// answers those questions from the archive alone.
	Provenance Provenance

	// RotationPeriod is the archive file's window. §32e says hourly; anything
	// positive works, and the file name grows precision to match.
	RotationPeriod time.Duration

	// AliveInterval is how often an unconditional Alive tick is written (§32g).
	AliveInterval time.Duration

	// SyncInterval is how often the file is fsynced between rotations (§32f).
	SyncInterval time.Duration
}

// Validate reports whether the config could open a writer.
func (c WriterConfig) Validate() error {
	if c.Root == "" {
		return ErrRootUnset
	}
	if err := c.Key.Validate(); err != nil {
		return err
	}
	if c.Clock == nil {
		return ErrTimeSourceUnset
	}
	if c.RotationPeriod <= 0 {
		return fmt.Errorf("rotation period %s: %w", c.RotationPeriod, ErrPeriodRange)
	}
	if c.AliveInterval <= 0 {
		return fmt.Errorf("alive interval %s: %w", c.AliveInterval, ErrIntervalRange)
	}
	if c.SyncInterval <= 0 {
		return fmt.Errorf("sync interval %s: %w", c.SyncInterval, ErrIntervalRange)
	}
	return nil
}

// Provenance is the caller's contribution to every Open record (§32g).
//
// Subscribe and SubscribeAck are raw bytes rather than json.RawMessage: a venue's
// subscribe frame is not guaranteed to be JSON, and this package parses nothing
// (§32n). encoding/json renders a []byte as base64, which is lossless — the
// archive keeps the bytes and `capture cat` is where they become readable, which
// is §32e's rule one level down.
type Provenance struct {
	Tool         string `json:"tool"`
	GitSHA       string `json:"git_sha"`
	ConfigHash   string `json:"config_hash"`
	Endpoint     string `json:"endpoint,omitempty"`
	Subscribe    []byte `json:"subscribe,omitempty"`
	SubscribeAck []byte `json:"subscribe_ack,omitempty"`
}

// openPayload is a RecordKindOpen record's payload.
//
// It names the stream STRUCTURALLY — venue, product, contract, symbol, stream,
// topic — and that is an addition to §32g's list with a reason. The archive path
// is otherwise the only record of which stream a file belongs to, because a frame
// carries no SourceID. One `mv` by a sync script and a file is anonymous. Six
// fields per file, once per hour, buys a file that can identify itself.
type openPayload struct {
	Venue    string `json:"venue"`
	Product  string `json:"product"`
	Contract string `json:"contract,omitempty"`
	Symbol   string `json:"symbol"`
	Stream   string `json:"stream"`
	Topic    string `json:"topic"`

	Format string `json:"format"`
	Window string `json:"window"`
	Host   string `json:"host"`
	PID    int    `json:"pid"`

	// PrevLastSeq is the last CaptureSeq of the file this one continues, so
	// cross-file continuity is checkable without reading either file in full
	// (§32e). Zero means no predecessor — which is why CaptureSeq is 1-based.
	PrevLastSeq uint64 `json:"prev_last_seq"`

	Provenance Provenance `json:"provenance"`
}

// closePayload is a RecordKindClose record's payload, §32g's {lastSeq,
// recordCount, sha256}.
//
// SHA256 covers every byte of the file BEFORE this record, which it must — the
// digest cannot cover the record carrying it. The `.sha256` sidecar written
// beside the file covers the complete file including this record, so the two
// digests differ by design and each is checkable on its own: the sidecar against
// `sha256sum -c`, and this field against a reader that stops where the Close
// record starts.
type closePayload struct {
	LastSeq     uint64 `json:"last_seq"`
	RecordCount uint64 `json:"record_count"`
	SHA256      string `json:"sha256"`
}

// clockStepPayload is a RecordKindClockStep record's payload.
//
// §32g names four fields — backstepCount, backstepTotalNs, systemNow, clampedNow
// — and this carries three. systemNow is NOT OBSERVABLE through the accessor
// §32i chose: Backsteps reports what the clamp absorbed, and reading the system
// clock to fill the field in would install the second time authority §32i
// explicitly rejected, to decorate a record about the first one. The count and
// the total are what the clamp actually computes, and clampedNow is what every
// neighbouring record was stamped with.
type clockStepPayload struct {
	BackstepCount   uint64 `json:"backstep_count"`
	BackstepTotalNs int64  `json:"backstep_total_ns"`
	ClampedNow      string `json:"clamped_now"`
}

// WriterStats is what the writer knows about itself, for §32j's observability
// and for assertions.
type WriterStats struct {
	Records     uint64
	DataRecords uint64
	AliveTicks  uint64
	Gaps        uint64
	Bytes       int64
	LastSeq     uint64
	Rotations   uint64
	Window      time.Time
	Path        string
}

// Writer owns one stream's archive tree: files, rotation, the exclusive lock,
// torn-tail recovery, gap records and Alive ticks. §32m slice B.
//
// # Exactly one goroutine may touch a Writer
//
// It is not goroutine-safe and it does not pretend to be, because a mutex here
// would buy the wrong thing. §32f's ordering guarantee is that the writer's
// stream is strictly increasing in CaptureSeq, and that is a property of ONE
// producer, not of serialized access — two goroutines writing under a lock
// produce an interleaving that is race-free and still wrong. The bounded queue
// (queue.go) is the seam where many producers become one consumer, and the
// single-writer property is enforced there and by the flock, not here.
type Writer struct {
	cfg    WriterConfig
	lock   *streamLock
	closed bool

	f      *os.File
	path   string
	window time.Time
	digest hash.Hash

	// buf is reused across records. AppendRecord returns dst unchanged on
	// error, so a refused record cannot leave a partial frame behind.
	buf []byte

	// lastSeq is the last Data CaptureSeq THIS writer saw, starting at zero, and
	// prevLastSeq is what the current file's Open record claimed. They are
	// separate because a new process legitimately restarts the reader's counter:
	// enforcing ascent against a value recovered from disk would refuse every
	// record after a restart, and enforcing nothing would let a reader that
	// silently reset produce a file whose sequence holes mean nothing.
	lastSeq     uint64
	prevLastSeq uint64

	// fileRecords counts records in the OPEN file, including any this writer
	// recovered rather than wrote, while stats.Records is the writer's lifetime
	// total. The Close record's record_count describes the file — a reader
	// checking it against a walk of that file's records would otherwise find
	// every file after the first claiming the whole run's total. fileGaps is
	// the same distinction for Gap records, carried by the manifest entry
	// (§38a), which likewise describes the file and not this run's share of it.
	fileRecords uint64
	fileGaps    uint64

	aliveDue  time.Time
	syncDue   time.Time
	dirty     bool
	backsteps uint64

	stats WriterStats
}

// OpenWriter opens (or recovers, or continues) the archive file for the current
// rotation window and takes the stream's exclusive lock.
//
// The order is lock, then recover, then Open record, then the gap records the
// recovery produced — and the gaps go AFTER the Open on purpose. A torn tail
// belongs to the previous run, so recording it before this run's provenance
// would put a fact about a dead process inside no run at all. After the Open it
// reads as what it is: the first thing this run found.
func OpenWriter(cfg WriterConfig) (*Writer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	dir := cfg.Key.Dir(cfg.Root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("capture: create stream directory: %w", err)
	}
	now := cfg.Clock.Now()
	lock, err := acquireLock(cfg.Key.LockPath(cfg.Root), now)
	if err != nil {
		return nil, err
	}

	w := &Writer{cfg: cfg, lock: lock, digest: sha256.New()}
	count, _ := cfg.Clock.Backsteps()
	w.backsteps = count
	w.aliveDue = nextTick(now, cfg.AliveInterval)
	w.syncDue = now.Add(cfg.SyncInterval)

	if err := w.openFile(Window(now, cfg.RotationPeriod), now); err != nil {
		_ = lock.Release()
		return nil, err
	}
	return w, nil
}

// nextTick is the first interval boundary strictly after t.
//
// Anchored to interval boundaries rather than to the writer's start, so the
// expected tick count for a window is period/interval — a constant §32k's verify
// can check against, rather than a number that depends on when the process
// happened to start.
func nextTick(t time.Time, interval time.Duration) time.Time {
	return t.UTC().Truncate(interval).Add(interval)
}

// Path is the archive file currently open.
func (w *Writer) Path() string { return w.path }

// Stats reports what the writer has done.
func (w *Writer) Stats() WriterStats {
	s := w.stats
	s.LastSeq, s.Window, s.Path = w.lastSeq, w.window, w.path
	return s
}

// Data writes a frame the reader captured. It is the only kind accepted from
// outside: every other record is minted here from the injected clock, and taking
// one from a caller would put a record in the archive whose RecvTime this writer
// cannot vouch for.
func (w *Writer) Data(r Record) error {
	if w.closed {
		return ErrWriterClosed
	}
	if r.Kind != RecordKindData {
		return fmt.Errorf("%s: %w", r.Kind, ErrKindNotData)
	}
	if err := r.Validate(); err != nil {
		return err
	}
	// Strictly ascending, which also forces CaptureSeq to be 1-based: lastSeq
	// starts at zero and zero is what an Open record's prev_last_seq means by
	// "no predecessor", so a reader whose counter starts at zero would make
	// those two states indistinguishable. Refusing here catches the realistic
	// version — a reader that reset its counter mid-run, whose holes would then
	// stop meaning what §32d says they mean.
	if r.CaptureSeq <= w.lastSeq {
		return fmt.Errorf("sequence %d follows %d: %w", r.CaptureSeq, w.lastSeq, ErrSeqNotAscending)
	}

	window := Window(r.RecvTime, w.cfg.RotationPeriod)
	switch {
	case window.Before(w.window):
		return fmt.Errorf("record at %s, file covers %s: %w",
			r.RecvTime.UTC().Format(time.RFC3339Nano), w.window.Format(time.RFC3339), ErrWindowBackwards)
	case window.After(w.window):
		if err := w.rotateTo(window, r.RecvTime); err != nil {
			return err
		}
	}

	if err := w.emit(r); err != nil {
		return err
	}
	w.lastSeq = r.CaptureSeq
	w.stats.DataRecords++
	return nil
}

// Gap records a discontinuity (§32g). Callers own the transport-level causes —
// reconnect, no-first-data, a clock step big enough to move the window — and the
// writer mints torn_tail and process_restart itself at open.
func (w *Writer) Gap(g Gap) error {
	if w.closed {
		return ErrWriterClosed
	}
	if err := g.Validate(); err != nil {
		return err
	}
	return w.mint(RecordKindGap, g, w.cfg.Clock.Now())
}

// Rest records a REST response (§32h). The payload framing — u32 metaLen ‖ meta ‖
// body — is slice E's; what slice B owes it is a way in that does not require a
// second writer.
func (w *Writer) Rest(payload []byte) error {
	if w.closed {
		return ErrWriterClosed
	}
	return w.mintRaw(RecordKindRest, payload, w.cfg.Clock.Now())
}

// Tick advances the writer's own timekeeping: rotation for an idle stream, the
// clock-step check, the unconditional Alive tick, and the periodic fsync.
//
// It is pull-driven, matching platform/clock's own stance — that package holds no
// goroutines and delivers timers by NextDue, because the dispatcher owns when a
// timer is consumed relative to an event. The same argument applies here one
// level down: a writer with its own ticker goroutine would need a mutex against
// the record path, and the mutex would serialize what a single consumer already
// orders.
//
// At most ONE Alive tick per call, and no backfilling of missed ones. That is the
// decision worth defending: a pump that stalled for ten intervals writes one tick
// on its return, so the window's tick count comes up short and §32k's verify sees
// it. Backfilling ten ticks stamped with the current instant would produce the
// expected count and ERASE the evidence that the process was stalled — which is
// the one thing the Alive mechanism exists to make visible.
func (w *Writer) Tick() error {
	if w.closed {
		return ErrWriterClosed
	}
	now := w.cfg.Clock.Now()

	// Rotation first, so a tick that lands past a boundary opens the new file
	// and is recorded there. The other order writes the last tick of an hour
	// into the wrong file.
	if window := Window(now, w.cfg.RotationPeriod); window.After(w.window) {
		if err := w.rotateTo(window, now); err != nil {
			return err
		}
	}

	if count, total := w.cfg.Clock.Backsteps(); count != w.backsteps {
		w.backsteps = count
		if err := w.mint(RecordKindClockStep, clockStepPayload{
			BackstepCount:   count,
			BackstepTotalNs: int64(total),
			ClampedNow:      now.UTC().Format(time.RFC3339Nano),
		}, now); err != nil {
			return err
		}
	}

	if !now.Before(w.aliveDue) {
		if err := w.mintRaw(RecordKindAlive, nil, now); err != nil {
			return err
		}
		w.stats.AliveTicks++
		w.aliveDue = nextTick(now, w.cfg.AliveInterval)
	}

	if w.dirty && !now.Before(w.syncDue) {
		if err := w.f.Sync(); err != nil {
			return fmt.Errorf("capture: sync %s: %w", w.path, err)
		}
		w.dirty = false
		w.syncDue = now.Add(w.cfg.SyncInterval)
	}
	return nil
}

// Close writes the Close record and its sidecar digest, fsyncs, and releases the
// lock. It is idempotent.
//
// The Close record's ABSENCE is the signal §32g names — a file without one ended
// in a crash — so the lock is released even when writing it fails: a writer that
// held its lock after failing to close would leave the stream uncapturable until
// the process died, turning a recorded bad close into an unrecorded silence.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	err := w.closeFile(w.cfg.Clock.Now())
	return errors.Join(err, w.lock.Release())
}

// rotateTo closes the current file and opens the one covering `window`.
//
// `at` is the timestamp that CAUSED the rotation — the arriving record's
// RecvTime, or the tick's instant — and both boundary records carry it. Using a
// fresh clock read instead would stamp the pair with an instant unrelated to the
// boundary they mark, and under replay the two would not even be the same value.
//
// The Close record therefore carries a RecvTime inside the NEXT window while
// living in the previous window's file. That is honest rather than tidy: it is
// when the file was closed, and the alternative — synthesizing the last instant
// of the old window — would put a timestamp in the archive that no clock ever
// reported.
func (w *Writer) rotateTo(window time.Time, at time.Time) error {
	if err := w.closeFile(at); err != nil {
		return err
	}
	w.stats.Rotations++
	return w.openFile(window, at)
}

// openFile opens or recovers the archive file for `window` and writes its Open
// record, plus any gap the recovery found.
func (w *Writer) openFile(window time.Time, at time.Time) error {
	path := w.cfg.Key.FilePath(w.cfg.Root, window, w.cfg.RotationPeriod)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("capture: create archive directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("capture: open %s: %w", path, err)
	}

	scan, err := scanArchive(f)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("capture: recover %s: %w", path, err)
	}
	if scan.tornBytes > 0 {
		if err := f.Truncate(scan.goodEnd); err != nil {
			_ = f.Close()
			return fmt.Errorf("capture: truncate torn tail of %s: %w", path, err)
		}
	}
	if _, err := f.Seek(scan.goodEnd, 0); err != nil {
		_ = f.Close()
		return fmt.Errorf("capture: seek %s: %w", path, err)
	}

	w.f, w.path, w.window, w.digest = f, path, window, scan.digest
	w.fileRecords = scan.records
	w.fileGaps = scan.gaps
	w.dirty = scan.tornBytes > 0
	if scan.lastSeq > 0 {
		// Continuing a file this stream already wrote: its last sequence is the
		// predecessor, and it also bounds what this run may write, so a restart
		// cannot append a record that appears to precede one already on disk.
		w.prevLastSeq = scan.lastSeq
		w.lastSeq = max(w.lastSeq, scan.lastSeq)
	}
	if scan.fresh {
		header := AppendFileHeader(nil)
		if _, err := f.Write(header); err != nil {
			return fmt.Errorf("capture: write file header: %w", err)
		}
		w.digest.Write(header)
		w.stats.Bytes += int64(len(header))
		w.dirty = true
	}

	if err := w.writeOpenRecord(window, at); err != nil {
		return err
	}
	if scan.tornBytes > 0 {
		if err := w.Gap(Gap{
			Cause:  CauseTornTail,
			Bytes:  scan.tornBytes,
			Detail: fmt.Sprintf("%d trailing bytes discarded at %s", scan.tornBytes, path),
		}); err != nil {
			return err
		}
	}
	if scan.hadClose {
		if err := w.Gap(Gap{
			Cause:  CauseProcessRestart,
			Detail: fmt.Sprintf("%s was closed cleanly and is being appended to by a new run", path),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeOpenRecord(window time.Time, at time.Time) error {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	vi := w.cfg.Key.Source.Instrument
	return w.mint(RecordKindOpen, openPayload{
		Venue:       string(vi.Venue),
		Product:     string(vi.Product),
		Contract:    string(vi.Contract),
		Symbol:      vi.Symbol,
		Stream:      string(w.cfg.Key.Source.Stream),
		Topic:       w.cfg.Key.Topic,
		Format:      fmt.Sprintf("v%d.%d", FormatMajor, FormatMinor),
		Window:      window.Format(time.RFC3339),
		Host:        host,
		PID:         os.Getpid(),
		PrevLastSeq: w.prevLastSeq,
		Provenance:  w.cfg.Provenance,
	}, at)
}

// closeFile writes the Close record, fsyncs, and writes the digest sidecar.
func (w *Writer) closeFile(at time.Time) error {
	if w.f == nil {
		return nil
	}
	// Taken before the Close record is written, because a digest cannot cover
	// the record that carries it.
	mintErr := w.mint(RecordKindClose, closePayload{
		LastSeq:     w.lastSeq,
		RecordCount: w.fileRecords,
		SHA256:      hex.EncodeToString(w.digest.Sum(nil)),
	}, at)

	f := w.f
	w.f = nil

	// The sidecar covers the COMPLETE file, Close record included, so it is the
	// digest `sha256sum -c` checks. Written in that tool's own format for the
	// same reason §32e made readability a tool rather than a storage decision:
	// the check should not need our code.
	full := hex.EncodeToString(w.digest.Sum(nil))
	syncErr := f.Sync()
	closeErr := f.Close()
	sidecarErr := os.WriteFile(w.path+digestExt,
		[]byte(fmt.Sprintf("%s  %s\n", full, filepath.Base(w.path))), 0o644)

	// The manifest entry (§38a) is written after the sidecar and only on a
	// fully clean close. The ordering makes the crash-window artifact
	// one-sided — sidecar present + entry absent is the repairable shape; the
	// reverse would be an outer claim about an inner one that never existed.
	// The gate on mintErr is the same fact one layer down: a file without a
	// Close record is not a closed file, and verify's no_close finding owns it.
	var manifestErr error
	if mintErr == nil && syncErr == nil && closeErr == nil && sidecarErr == nil {
		if rel, err := filepath.Rel(w.cfg.Root, w.path); err != nil {
			manifestErr = err
		} else {
			manifestErr = upsertManifestEntry(w.cfg.Root, ManifestEntry{
				Path:     filepath.ToSlash(rel),
				Window:   w.window.Format(time.RFC3339),
				SHA256:   full,
				Records:  w.fileRecords,
				Gaps:     w.fileGaps,
				LastSeq:  w.lastSeq,
				ClosedAt: at.UTC().Format(time.RFC3339Nano),
				Source:   ManifestSourceClose,
			})
		}
	}

	w.prevLastSeq = w.lastSeq
	w.dirty = false
	return errors.Join(mintErr, syncErr, closeErr, sidecarErr, manifestErr)
}

// mint encodes a JSON payload and writes it as a writer-owned record.
func (w *Writer) mint(kind RecordKind, payload any, at time.Time) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("capture: encode %s payload: %w", kind, err)
	}
	return w.mintRaw(kind, body, at)
}

// mintRaw writes a writer-owned record with a payload already in bytes.
//
// Writer-owned records carry the last Data CaptureSeq rather than one of their
// own, and that is the decision this slice forced. §32d makes CaptureSeq the
// READER's counter, dense over captured frames, so a hole in it means a dropped
// frame with no other evidence required. A writer that also minted sequence
// numbers would be a second authority over that counter, and every Alive tick
// would fill a hole that a drop was supposed to leave. Echoing the last data
// sequence keeps the counter's meaning intact and still locates the record in
// the stream: "this happened after frame N".
func (w *Writer) mintRaw(kind RecordKind, payload []byte, at time.Time) error {
	return w.emit(Record{
		Kind:       kind,
		CaptureSeq: w.lastSeq,
		RecvTime:   at,
		Mono:       w.cfg.Clock.Mono(),
		Payload:    payload,
	})
}

// emit encodes one record and appends it to the open file.
func (w *Writer) emit(r Record) error {
	if w.f == nil {
		return ErrWriterClosed
	}
	buf, err := AppendRecord(w.buf[:0], r)
	if err != nil {
		return err
	}
	w.buf = buf
	if _, err := w.f.Write(buf); err != nil {
		return fmt.Errorf("capture: write %s to %s: %w", r.Kind, w.path, err)
	}
	w.digest.Write(buf)
	w.stats.Records++
	w.fileRecords++
	w.stats.Bytes += int64(len(buf))
	if r.Kind == RecordKindGap {
		w.stats.Gaps++
		w.fileGaps++
	}
	w.dirty = true
	return nil
}
