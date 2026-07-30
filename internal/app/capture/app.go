// Package capture is the composition root for arblab's capture commands
// (§32b): configuration → sources → transport streams → stamping sinks →
// bounded queues → archive writers → observability.
//
// §11 makes app/ the place where things are allowed to meet: this package may
// read the ambient clock to SCHEDULE (the tick driver below), may import both
// the engine and the platform, and owns every decision about how the pieces
// wire together. What it may never import is arbitration — no bounded context
// and no dispatcher — which imports_test.go enforces with the same fence
// internal/engine/capture carries.
//
// Every timestamp that reaches a record still comes from the injected
// TimeSource: the ambient reads here drive WHEN Tick is called, and the writer
// stamps WHAT time it is from its own clock. Scheduling and stamping are the
// two halves §32i splits, and this package sits exactly on the line.
package capture

import (
	"context"
	"sync"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// The compile-time assertion §36b could not write yet: platform/clock's Wall
// is capture's live TimeSource. It lives here rather than beside either type
// because kernelpurity forbids platform/ importing capture, and the engine
// package deliberately declares the port without naming its implementation —
// the composition root is the one place that knows both halves.
var _ enginecapture.TimeSource = (*clock.Wall)(nil)

// Run captures every configured source until ctx is cancelled or a pipeline
// fails. It returns nil on a clean cancellation.
//
// wall is THE process clock — one Wall, because Mono's baseline is per-Wall
// and two Walls in one process would stamp records on two unrelated timelines.
// The snapshot's revision hash goes into every Open record, so an archive file
// can name the exact configuration it was captured under.
func Run(ctx context.Context, snap config.Snapshot[Config], wall *clock.Wall,
	reg *observability.Registry, build Build) error {

	cfg := snap.Value
	met, err := newMetrics(reg, cfg)
	if err != nil {
		return err
	}

	// Sized so every pipeline can report one error without blocking; the
	// first is the one Run returns, the rest are consequences of it.
	errs := make(chan error, len(cfg.Sources))

	pipes := make([]*pipeline, 0, len(cfg.Sources))
	for _, src := range cfg.Sources {
		p, err := newPipeline(cfg, src, wall, build, snap.Rev.HashString(),
			newSourceMetrics(met, wall, src), errs)
		if err != nil {
			return err
		}
		pipes = append(pipes, p)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, p := range pipes {
		p := p
		tick := make(chan struct{}, 1)

		// The shutdown chain is this ordering: cancelling runCtx returns
		// stream.Run, whose goroutine then closes the queue — it is the
		// producer's close, made on the producer's behalf after the producer
		// goroutine has provably stopped pushing. The pump drains what
		// remains and closes items, and the consumer flushes and closes the
		// writer. Every frame pushed before the cancellation reaches the
		// file.
		wg.Add(4)
		go func() {
			defer wg.Done()
			if err := p.stream.Run(runCtx); err != nil {
				p.fail(err)
			}
			p.queue.Close()
		}()
		go func() {
			defer wg.Done()
			p.pump()
		}()
		go func() {
			defer wg.Done()
			if err := p.consume(runCtx, tick); err != nil {
				p.fail(err)
			}
		}()
		go func() {
			defer wg.Done()
			driveTicks(runCtx, tick, tickEvery(cfg))
		}()
	}

	// First failure stops the run: the realistic open-time causes — a second
	// process holding a flock, an unwritable root — apply to every source,
	// and a run limping on a subset would look healthy from outside.
	var firstErr error
	select {
	case <-ctx.Done():
	case firstErr = <-errs:
		cancel()
	}
	wg.Wait()
	if firstErr == nil {
		select {
		case firstErr = <-errs:
		default:
		}
	}
	return firstErr
}

// tickEvery is the drive cadence for the writers' pull-driven Tick: finer than
// the finest configured cadence so no boundary is missed by more than a
// fraction of itself, and capped at a second so an idle stream still rotates
// and fsyncs promptly.
func tickEvery(cfg Config) time.Duration {
	d := min(cfg.AliveInterval.Duration(), cfg.SyncInterval.Duration()) / 4
	return min(max(d, 100*time.Millisecond), time.Second)
}

// driveTicks feeds the consumer's tick channel from the ambient clock. The
// send is non-blocking with a one-slot buffer: a consumer mid-write misses
// nothing (one tick is already waiting for it), and a stalled consumer is not
// backed up behind a queue of stale ticks — Tick reads the injected clock when
// it RUNS, so queued ticks would all describe the same instant anyway.
func driveTicks(ctx context.Context, tick chan<- struct{}, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			select {
			case tick <- struct{}{}:
			default:
			}
		}
	}
}
