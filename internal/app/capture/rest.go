package capture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
)

// restPipeline is one §32h overlay stream: an hourly GET whose every outcome
// — rows, an empty array, a 429, a refused connection — becomes a Rest record
// on this stream's own archive.
//
// ONE goroutine owns everything (§37f). §32f's queue exists because a socket
// pushes faster than a disk accepts; here the producer is a scheduled pull
// whose result is one record an hour, so the Writer's one-goroutine rule is
// satisfied by having exactly one, and there is no sink, no pump and no
// ownership hand-off to reason about.
type restPipeline struct {
	cfg        Config
	src        RestSourceConfig
	clock      enginecapture.TimeSource
	build      Build
	configHash string

	poller *transport.Poller
	errs   chan<- error
	met    *sourceMetrics
}

func newRestPipeline(cfg Config, src RestSourceConfig, clock enginecapture.TimeSource,
	build Build, configHash string, met *sourceMetrics, errs chan<- error) (*restPipeline, error) {

	poller, err := transport.NewPoller(cfg.pollerConfig(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src.Key(), err)
	}
	return &restPipeline{
		cfg:        cfg,
		src:        src,
		clock:      clock,
		build:      build,
		configHash: configHash,
		poller:     poller,
		errs:       errs,
		met:        met,
	}, nil
}

// fail reports an error to the run without blocking — pipeline's rule, same
// one-report sizing.
func (p *restPipeline) fail(err error) {
	select {
	case p.errs <- fmt.Errorf("%s: %w", p.src.Key(), err):
	default:
	}
}

// run polls until ctx is cancelled. The writer opens at START, not on a first
// response (§37f): the WS side delays open so the venue's ack lands in the
// Open record, but REST has no ack, and opening early means a run whose polls
// all fail still archives Alive ticks and status-0 Rest records — "capture
// alive, asks failing", which is the honest description and reaches verify as
// exactly that.
func (p *restPipeline) run(ctx context.Context, tick <-chan struct{}) error {
	w, err := enginecapture.OpenWriter(enginecapture.WriterConfig{
		Root:  p.cfg.Root,
		Key:   p.src.Key(),
		Clock: p.clock,
		Provenance: enginecapture.Provenance{
			Tool:       p.build.Tool,
			GitSHA:     p.build.GitSHA,
			ConfigHash: p.configHash,
			Endpoint:   p.poller.URL(),
			Subscribe:  []byte(http.MethodGet + " " + p.src.Path),
		},
		RotationPeriod: p.cfg.RotationPeriod.Duration(),
		AliveInterval:  p.cfg.AliveInterval.Duration(),
		SyncInterval:   p.cfg.SyncInterval.Duration(),
	})
	if err != nil {
		// Same argument as the WS open: a stream that cannot open its archive
		// cannot capture, and the realistic causes apply to every source.
		p.fail(err)
		return err
	}

	// The first ask is immediate rather than grid-aligned (§37d): a process
	// that starts at :59 and dies at :02 should still have asked once. The
	// next grid instant is fixed BEFORE the ask, here and in the loop below,
	// so an ask that straddles a boundary cannot erase that boundary's poll
	// — the schedule is a function of when asks become due, never of how
	// long the venue took to answer.
	next := p.nextAfter(p.clock.Now())
	if err := p.poll(ctx, w); err != nil {
		p.fail(err)
		return errors.Join(err, w.Close())
	}

	for {
		select {
		case <-ctx.Done():
			p.met.exportWriter(w.Stats())
			return w.Close()
		case <-tick:
			// Tick BEFORE poll, so a poll due at the hour boundary lands in
			// the file the rotation just opened — one Rest record per hourly
			// file when the cadences agree (§37d).
			if err := w.Tick(); err != nil {
				p.fail(err)
				return errors.Join(err, w.Close())
			}
			if !p.clock.Now().Before(next) {
				next = p.nextAfter(p.clock.Now())
				if err := p.poll(ctx, w); err != nil {
					p.fail(err)
					return errors.Join(err, w.Close())
				}
			}
			p.met.exportWriter(w.Stats())
		}
	}
}

// poll performs one ask and archives its outcome. The returned error is a
// WRITE failure — the one thing that stops the pipeline. An ask that failed
// is not an error here; it is the record (§37b).
func (p *restPipeline) poll(ctx context.Context, w *enginecapture.Writer) error {
	meta := enginecapture.RestMeta{
		Method:  http.MethodGet,
		URL:     p.poller.URL(),
		AskedAt: p.clock.Now(),
	}
	var body []byte
	res, err := p.poller.Fetch(ctx)
	if err != nil {
		meta.Error = err.Error()
	} else {
		meta.Status = res.Status
		body = res.Body
	}
	payload, err := enginecapture.EncodeRestPayload(enginecapture.RestPayload{Meta: meta, Body: body})
	if err != nil {
		// Unreachable when this file is correct — the meta is built above
		// and always names one outcome. Refusing loudly beats archiving a
		// frame the decoder would reject at read time.
		return err
	}
	return w.Rest(payload)
}

// nextAfter is the first poll instant on the grid strictly after now. The
// grid is time.Time.Truncate's — the rotation grid's own anchor (§36j) — so
// an hourly cadence asks once per hourly file.
func (p *restPipeline) nextAfter(now time.Time) time.Time {
	return now.Truncate(p.src.PollInterval.Duration()).Add(p.src.PollInterval.Duration())
}
