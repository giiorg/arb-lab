package capture

import (
	"context"
	"errors"
	"fmt"
	"sync"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
)

// Build identifies the running binary for the Open record's provenance. Six
// weeks later "which build wrote this file" is unanswerable from the frames
// alone, which is the whole reason Provenance exists (§32g).
type Build struct {
	Tool   string
	GitSHA string
}

// pipeline is one source's capture chain: transport.Stream → sink →
// capture.Queue → consumer → capture.Writer.
//
// # Who touches what, exactly
//
// The Writer permits exactly one goroutine. Here that is TWO goroutines in
// SEQUENCE, never in parallel: the stream goroutine opens the writer (see
// sink.open for why it must be the one to do it), seeds the sink's counter,
// sends the writer down openCh and never touches it again. The channel send is
// the ownership transfer, and everything after it happens on the consumer.
//
// The sinkState fields belong to the stream goroutine alone — every
// transport.Sink callback runs there, in order. The gap list is shared and
// mutex-guarded: the sink appends, the consumer drains. Queue, Stream and the
// metrics registry carry their own locks; sourceMetrics' delta fields are
// consumer-only.
type pipeline struct {
	cfg        Config
	src        SourceConfig
	clock      enginecapture.TimeSource
	build      Build
	configHash string

	stream *transport.Stream
	queue  *enginecapture.Queue
	items  chan item
	openCh chan *enginecapture.Writer
	errs   chan<- error
	met    *sourceMetrics

	state sinkState

	mu   sync.Mutex
	gaps []taggedGap
}

// sinkState is the stream-goroutine-only half of the pipeline. Declared as its
// own type so the ownership boundary is a field name rather than a comment on
// each field.
type sinkState struct {
	seq    uint64
	opened bool
	dead   bool
}

// item mirrors what Queue.Receive returns, so the pump can forward it into a
// channel the consumer can select over alongside its tick.
type item struct {
	rec enginecapture.Record
	gap *enginecapture.Gap
}

// taggedGap is a transport discontinuity waiting to be written in order.
//
// afterSeq is the sink's counter at the moment of the disruption: every frame
// numbered at or below it arrived before the gap, every frame above it after.
// The consumer writes the gap before the first record whose sequence exceeds
// afterSeq, which puts the record in the file exactly where the event sits in
// the stream — the ordering the queue provides for records, restored for the
// one thing that cannot ride the queue.
type taggedGap struct {
	gap      enginecapture.Gap
	afterSeq uint64
}

func newPipeline(cfg Config, src SourceConfig, clock enginecapture.TimeSource,
	build Build, configHash string, met *sourceMetrics, errs chan<- error) (*pipeline, error) {

	queue, err := enginecapture.NewQueue(cfg.QueueDepth)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src.Key(), err)
	}
	p := &pipeline{
		cfg:        cfg,
		src:        src,
		clock:      clock,
		build:      build,
		configHash: configHash,
		queue:      queue,
		items:      make(chan item),
		openCh:     make(chan *enginecapture.Writer, 1),
		errs:       errs,
		met:        met,
	}
	stream, err := transport.NewStream(cfg.streamConfig(src, (*sink)(p)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src.Key(), err)
	}
	p.stream = stream
	return p, nil
}

// fail reports an error to the run without blocking. The channel is sized for
// one report per pipeline; anything beyond the first is detail the first has
// already made moot.
func (p *pipeline) fail(err error) {
	select {
	case p.errs <- fmt.Errorf("%s: %w", p.src.Key(), err):
	default:
	}
}

// addGap parks a transport gap for the consumer.
func (p *pipeline) addGap(g taggedGap) {
	p.mu.Lock()
	p.gaps = append(p.gaps, g)
	p.mu.Unlock()
}

// takeGaps removes and returns every parked gap whose afterSeq is at or below
// bound — the ones whose preceding frames have all been written or accounted.
func (p *pipeline) takeGaps(bound uint64) []taggedGap {
	p.mu.Lock()
	defer p.mu.Unlock()
	var due []taggedGap
	rest := p.gaps[:0]
	for _, g := range p.gaps {
		if g.afterSeq <= bound {
			due = append(due, g)
		} else {
			rest = append(rest, g)
		}
	}
	p.gaps = rest
	return due
}

// sink is the pipeline seen from the stream goroutine. It is the same struct —
// the conversion exists so that transport.Sink's methods, which run on the
// stream's goroutine, are visibly separate from the consumer's methods.
type sink pipeline

var _ transport.Sink = (*sink)(nil)

// Data stamps a venue frame and offers it to the queue (§32d): CaptureSeq,
// RecvTime and Mono are assigned HERE, the instant the frame comes off the
// socket, so a frame the queue then drops still leaves a hole in the sequence.
func (s *sink) Data(payload []byte) {
	// The clock is read before the (one-time) open below: RecvTime is the
	// arrival instant, and the first frame's stamp must not include the file
	// open that its own arrival triggered.
	recv := s.clock.Now()
	mono := s.clock.Mono()

	if !s.state.opened {
		// Data before any ack: the venue reordered its handshake. The writer
		// opens now with the request we sent and no ack — honest, because at
		// this instant no ack exists — rather than holding venue data hostage
		// to a handshake message that may never come.
		s.open(s.stream.SubscribeFrame(), nil)
	}
	if s.state.dead {
		return
	}
	s.state.seq++
	s.queue.Push(enginecapture.Record{
		Kind:       enginecapture.RecordKindData,
		CaptureSeq: s.state.seq,
		RecvTime:   recv,
		Mono:       mono,
		Payload:    payload,
	})
	// A refused push needs nothing from us: the queue has already recorded
	// the hole, and it will ride with the next accepted record (§32f).
}

// Subscribed opens the writer on the run's first successful subscription, so
// the venue's ack lands in the Open record's provenance (§32g). On a reconnect
// the writer is already open and no new Open record is written, so only the
// run's first ack becomes provenance — §36h's recorded limit.
func (s *sink) Subscribed(request, ack []byte) {
	if s.state.opened {
		return
	}
	s.open(request, ack)
}

// Disrupted parks the discontinuity for the consumer, tagged with the counter
// so it is written between the frames it actually interrupted. Before the
// first successful open there is no writer; the gap is parked all the same and
// becomes the first thing the run's archive reports after its Open record —
// what this run found on the way up.
func (s *sink) Disrupted(d transport.Disruption) {
	(*pipeline)(s).addGap(taggedGap{
		gap: enginecapture.Gap{
			Cause:  mapCause(d.Cause),
			Detail: string(d.Cause) + ": " + d.Detail,
		},
		afterSeq: s.state.seq,
	})
}

// open opens the archive writer, seeds the sink's counter, and hands the
// writer to the consumer. It runs on the stream goroutine, once.
//
// # Why the STREAM goroutine opens the writer
//
// Two constraints meet here and only this ordering satisfies both. The ack can
// only be in the Open record if the writer opens after the subscription is
// acked — so the open is triggered by a Sink callback, which runs on the
// stream goroutine. And §36m's obligation is that the sink's counter is seeded
// from Stats().LastSeq BEFORE any frame is stamped — so the seeding must
// happen synchronously inside the callback, before Data can run again. Doing
// the open on the consumer would meet neither: the sink would stamp frames it
// cannot yet number.
//
// The open is file I/O on a callback documented "must not block". That rule
// guards against sustained back-pressure turning the bounded queue into TCP
// back-pressure; a single millisecond-scale open per process run is not that,
// and the socket's kernel buffer absorbs it.
func (s *sink) open(request, ack []byte) {
	s.state.opened = true
	w, err := enginecapture.OpenWriter(enginecapture.WriterConfig{
		Root:  s.cfg.Root,
		Key:   s.src.Key(),
		Clock: s.clock,
		Provenance: enginecapture.Provenance{
			Tool:         s.build.Tool,
			GitSHA:       s.build.GitSHA,
			ConfigHash:   s.configHash,
			Endpoint:     s.stream.URL(),
			Subscribe:    request,
			SubscribeAck: ack,
		},
		RotationPeriod: s.cfg.RotationPeriod.Duration(),
		AliveInterval:  s.cfg.AliveInterval.Duration(),
		SyncInterval:   s.cfg.SyncInterval.Duration(),
	})
	if err != nil {
		// A stream that cannot open its archive cannot capture: dying loudly
		// beats reading a venue feed into nowhere. The run fails as a whole —
		// the realistic cause (a second process holding the flock, a bad
		// root) applies to every other source too.
		s.state.dead = true
		(*pipeline)(s).fail(err)
		return
	}
	// §36m: the reader's counter continues from what the archive already
	// holds. Without this, a restart into an hour this process had already
	// opened has every Data record refused for the rest of that hour, and the
	// archive fills with Alive ticks.
	s.state.seq = w.Stats().LastSeq
	s.openCh <- w
}

// mapCause translates the transport's vocabulary into the archive's.
//
// A switch rather than a map so exhaustiveswitch (§3.6 rule 10) binds: a
// DisruptionCause added to the transport and not decided here is a compile
// failure, not a silently-invented classification. Dial, subscribe and read
// failures are all "the socket dropped and came back" from the archive's side;
// no-first-data is §32k's own cause. The zero value maps to reconnect — the
// original cause string travels in the gap's Detail either way, so an
// unspecified one remains visible in the record it produced.
func mapCause(c transport.DisruptionCause) enginecapture.Cause {
	switch c {
	case transport.DisruptionDial, transport.DisruptionSubscribe, transport.DisruptionRead,
		transport.DisruptionUnspecified:
		return enginecapture.CauseReconnect
	case transport.DisruptionNoFirstData:
		return enginecapture.CauseNoFirstData
	}
	return enginecapture.CauseReconnect
}

// pump forwards the queue into a channel the consumer can select over. It
// exits when the queue is closed and drained, closing items behind it. It has
// no context: the producer never blocks (Push is non-blocking), so the only
// thing that stops the pump is the queue closing, and stopping it any earlier
// would lose frames a clean shutdown is supposed to flush.
func (p *pipeline) pump() {
	defer close(p.items)
	for {
		rec, gap, ok := p.queue.Receive()
		if !ok {
			return
		}
		p.items <- item{rec: rec, gap: gap}
	}
}

// consume owns the writer from hand-off to close. tick drives the writer's
// own timekeeping (rotation, Alive, fsync) and the metrics export; in
// production it is fed by a time.Ticker, in tests by the test.
func (p *pipeline) consume(ctx context.Context, tick <-chan struct{}) error {
	// The pump must always reach items' close, even if this function returns
	// early on a write error — its send would otherwise block forever. The
	// drain goroutine exits as soon as items closes, which on the normal path
	// is immediately.
	defer func() {
		go func() {
			for range p.items {
			}
		}()
	}()

	// Phase 1: no writer yet. An item implies the writer is already in openCh
	// — the sink opens before it stamps — so items doubles as a wake-up.
	var w *enginecapture.Writer
	var first *item
	for w == nil {
		select {
		case w = <-p.openCh:
		case it, ok := <-p.items:
			if !ok {
				return nil // shut down before any successful connection
			}
			w = <-p.openCh
			first = &it
		case <-tick:
			p.exportTransport()
		case <-ctx.Done():
			// Cancelled before any connection succeeded. An open may still be
			// in flight on the stream goroutine; closeLate waits the pipeline
			// out and closes a writer that arrived after all.
			return p.closeLate()
		}
	}

	if first != nil {
		if err := p.write(w, *first); err != nil {
			p.fail(err)
			return errors.Join(err, w.Close())
		}
	}

	// Phase 2: the steady loop. It does not watch ctx: shutdown arrives as
	// the items channel closing (stream stops → queue closes → pump drains),
	// which is what lets every frame pushed before cancellation reach the
	// file.
	for {
		select {
		case it, ok := <-p.items:
			if !ok {
				return p.shutdown(w)
			}
			if err := p.write(w, it); err != nil {
				p.fail(err)
				return errors.Join(err, w.Close())
			}
		case <-tick:
			if err := p.flushGaps(w, w.Stats().LastSeq); err != nil {
				p.fail(err)
				return errors.Join(err, w.Close())
			}
			if err := w.Tick(); err != nil {
				p.fail(err)
				return errors.Join(err, w.Close())
			}
			p.export(w)
		}
	}
}

// write puts one queued item into the archive: first any transport gap that
// happened before this frame, then the overflow hole that immediately
// preceded it, then the frame itself.
func (p *pipeline) write(w *enginecapture.Writer, it item) error {
	if err := p.flushGaps(w, it.rec.CaptureSeq-1); err != nil {
		return err
	}
	if it.gap != nil {
		if err := w.Gap(*it.gap); err != nil {
			return err
		}
		p.met.countGap(it.gap.Cause)
	}
	if err := w.Data(it.rec); err != nil {
		return err
	}
	p.met.noteData(it.rec.RecvTime)
	return nil
}

// flushGaps writes every parked transport gap whose preceding frames are
// settled — written, or accounted by an overflow hole at or below bound.
func (p *pipeline) flushGaps(w *enginecapture.Writer, bound uint64) error {
	for _, g := range p.takeGaps(bound) {
		if err := w.Gap(g.gap); err != nil {
			return err
		}
		p.met.countGap(g.gap.Cause)
	}
	return nil
}

// shutdown is the clean end: the queue is drained, so everything parked is
// eligible. A hole still open at the queue (we stopped while overflowing) is
// written last — the tidy case must not be the one case with no record of the
// loss (§32f).
func (p *pipeline) shutdown(w *enginecapture.Writer) error {
	var errs []error
	if err := p.flushGaps(w, ^uint64(0)); err != nil {
		errs = append(errs, err)
	}
	if g, ok := p.queue.PendingGap(); ok {
		if err := w.Gap(g); err != nil {
			errs = append(errs, err)
		} else {
			p.met.countGap(g.Cause)
		}
	}
	p.export(w)
	errs = append(errs, w.Close())
	return errors.Join(errs...)
}

// closeLate closes a writer whose hand-off raced a cancellation. By the time
// items is closed the stream goroutine has exited, so any send into openCh has
// already happened; a non-blocking receive after that point is definitive.
func (p *pipeline) closeLate() error {
	for range p.items {
		// Discard: these frames were stamped but the run is over and the
		// writer, if any, is about to be closed. The sequence holes they
		// leave are exactly what §32d makes them: visible.
	}
	select {
	case w := <-p.openCh:
		return errors.Join(p.flushGaps(w, ^uint64(0)), w.Close())
	default:
		return nil
	}
}

// export folds every snapshot into the metrics; exportTransport is the subset
// available before the writer exists.
func (p *pipeline) export(w *enginecapture.Writer) {
	p.met.exportWriter(w.Stats())
	p.exportTransport()
}

func (p *pipeline) exportTransport() {
	p.met.exportQueue(p.queue.Stats())
	p.met.exportStream(p.stream.Stats().ConsecutiveFailures)
}
