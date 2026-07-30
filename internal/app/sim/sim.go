// Package sim is the parity probe's composition root (§39p-8): the wiring
// slice that closes §39j — replayed archive → ingestion → marketdata →
// mailboxes → dispatcher+LogWriter → engine/state, under TestClock.
//
// Like every app package it is allowed to import everything — contexts,
// engine, platform — and contributes wiring, not behaviour: which lane, which
// budgets, which sink chain. The behaviour it composes is proven in the
// packages that own it; what the probe over this root proves is that the
// COMPOSITION is deterministic — two Generate runs over one archive produce
// byte-identical dispatch logs, and a Rerun following that log re-drives the
// state cache to the same terminal entries.
package sim

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/engine/replay"
	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/marketdata"
	"github.com/giiorg/arb-lab/internal/marketdata/application"
	"github.com/giiorg/arb-lab/internal/marketdata/application/ports"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driving"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Sentinel errors.
var (
	// ErrPayloadUnrouted reports a dispatched payload type no route claims.
	// With only the funding path wired, anything else reaching the sink is a
	// wiring hole, and dropping it silently would be a consumer that reads as
	// subscribed while consuming nothing.
	ErrPayloadUnrouted = errors.New("sim: dispatched payload has no route")

	// ErrResidue reports a run that ended with envelopes still queued or
	// drops recorded — either means the driver's advance rule failed to
	// deliver the world it was handed (§39p-6's end-of-run assertions).
	ErrResidue = errors.New("sim: run ended with mailbox residue or drops")
)

// Config is one sim run's wiring choices.
type Config struct {
	// Root is the archive root (the committed fixture in CI; the production
	// tree in a manual soak).
	Root string

	// Streams is the replay roster: which streams, which decoders.
	Streams []replay.StreamSpec

	// FreshnessJSON is marketdata's §5.2 section, as document bytes — through
	// platform/config's strict decode, because the sim must exercise the same
	// config path the live root uses, not a typed bypass of it.
	FreshnessJSON []byte

	// MailboxDepth bounds each per-source mailbox. The sim drains between
	// batches so the working depth is small; the bound exists because an
	// unbounded lane is refused by its own constructor.
	MailboxDepth int

	// LogPath is where THIS run writes its dispatch log. Fresh files only —
	// LogWriter's O_EXCL contract.
	LogPath string
}

// Result is what a run left behind.
type Result struct {
	Report replay.Report

	// LastSeq is the final IngressSeq (Generate) or the followed log's last
	// entry (Rerun).
	LastSeq uint64

	// Funding is the terminal cache state for every instrument on the roster
	// — the object the parity probe compares across runs.
	Funding map[ids.VenueInstrument]state.FundingEntry

	// FollowedLog reports what Rerun's followed log turned out to be; zero on
	// Generate.
	FollowedLog dispatch.LogReport
}

// rig is one assembled run: everything between the tape and the cache.
type rig struct {
	tape  *replay.Tape
	ing   *replay.Ingester
	clk   *clock.TestClock
	lane  *events.MarketDataLane
	log   *dispatch.LogWriter
	cache *state.Cache
}

// assemble builds a fresh world for one run. Every run gets its own — decoder
// state, ordinal counter, clock, lane, use cases and cache are all run state,
// and the parity claim is exactly that two fresh worlds over one archive
// agree.
func assemble(cfg Config) (*rig, error) {
	tape, err := replay.Load(cfg.Root, cfg.Streams)
	if err != nil {
		return nil, err
	}
	ing, err := replay.NewIngester(cfg.Streams)
	if err != nil {
		return nil, err
	}

	cals, err := replay.Calibrate(tape, cfg.Streams)
	if err != nil {
		return nil, err
	}
	start := tape.Start()
	clk, err := clock.NewTestClock(start, cals)
	if err != nil {
		return nil, fmt.Errorf("sim: clock: %w", err)
	}

	store, err := config.New[driven.FreshnessConfig]("sim-freshness", start, cfg.FreshnessJSON)
	if err != nil {
		return nil, fmt.Errorf("sim: freshness config: %w", err)
	}

	lane, err := events.NewMarketDataLane(cfg.MailboxDepth, events.NoCoalescing)
	if err != nil {
		return nil, fmt.Errorf("sim: lane: %w", err)
	}

	module, err := marketdata.New(marketdata.Deps{
		Clock:         clk,
		Config:        store,
		Lane:          lane,
		Resnapshotter: refuseResnapshot{},
		BookHoldDepth: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("sim: marketdata: %w", err)
	}

	cache := state.NewCache()
	logw, err := dispatch.NewLogWriter(cfg.LogPath, &router{funding: module.Funding, cache: cache})
	if err != nil {
		return nil, err
	}

	return &rig{tape: tape, ing: ing, clk: clk, lane: lane, log: logw, cache: cache}, nil
}

// Generate runs the arbitrating pump over the archive and freezes the order
// at cfg.LogPath.
func Generate(cfg Config) (Result, error) {
	r, err := assemble(cfg)
	if err != nil {
		return Result{}, err
	}

	d, err := dispatch.New(dispatch.Config{
		Control: controlLane(),
		Market:  r.lane,
		Timers:  r.clk,
		Sink:    r.log,
	})
	if err != nil {
		r.log.Close()
		return Result{}, fmt.Errorf("sim: dispatcher: %w", err)
	}

	rep, runErr := replay.Generate(r.tape, r.ing, r.clk, r.lane, d, superseded)
	if err := r.log.Close(); runErr == nil && err != nil {
		runErr = err
	}
	if runErr != nil {
		return Result{}, runErr
	}
	if err := r.residue(cfg.Streams); err != nil {
		return Result{}, err
	}
	return Result{Report: rep, LastSeq: d.LastSeq(), Funding: r.funding(cfg.Streams)}, nil
}

// Rerun follows the dispatch log at follow, re-deriving payloads from the
// archive, and writes the re-driven order to cfg.LogPath — which must
// byte-equal the log it followed (§39p-7).
func Rerun(cfg Config, follow string) (Result, error) {
	r, err := assemble(cfg)
	if err != nil {
		return Result{}, err
	}

	rep, lr, runErr := replay.Rerun(r.tape, r.ing, r.clk, r.lane, r.log, follow, superseded)
	if err := r.log.Close(); runErr == nil && err != nil {
		runErr = err
	}
	if runErr != nil {
		return Result{}, runErr
	}
	if err := r.residue(cfg.Streams); err != nil {
		return Result{}, err
	}
	return Result{Report: rep, LastSeq: lr.LastSeq, Funding: r.funding(cfg.Streams), FollowedLog: lr}, nil
}

// residue enforces §39p-6's end-of-run assertions: no drops anywhere, no
// envelope left queued. A drop in the sim means the driver's rule is wrong
// (live drops are legitimate and recorded; sim drops are a bug), and residue
// means the run stopped while the world still had something to say.
func (r *rig) residue(specs []replay.StreamSpec) error {
	for _, s := range r.lane.Sources() {
		if n := r.lane.Dropped(s); n != 0 {
			return fmt.Errorf("sim: source %s dropped %d: %w", s, n, ErrResidue)
		}
		if n := r.lane.Len(s); n != 0 {
			return fmt.Errorf("sim: source %s still holds %d: %w", s, n, ErrResidue)
		}
	}
	_ = specs
	return nil
}

// funding extracts the terminal cache entries for the roster's instruments.
func (r *rig) funding(specs []replay.StreamSpec) map[ids.VenueInstrument]state.FundingEntry {
	out := map[ids.VenueInstrument]state.FundingEntry{}
	for _, s := range specs {
		inst := s.Key.Source.Instrument
		if e, ok := r.cache.Funding(inst); ok {
			out[inst] = e
		}
	}
	return out
}

// superseded classifies the one sink refusal that is an ordinary outcome: an
// at-least-once transport re-delivering a reading the use case already holds
// (application.ErrSuperseded). Everything else is a wiring fault and fatal.
func superseded(err error) bool { return errors.Is(err, application.ErrSuperseded) }

// controlLane returns the dispatcher's control input: present because the
// dispatcher requires all four inputs (its zero value is unusable), empty
// because milestone 2's read path has no control producers — a fill or a
// command reaching this sim would be a milestone-3 world leaking in.
func controlLane() *events.ControlLane {
	l, err := events.NewControlLane(1)
	if err != nil {
		// NewControlLane refuses only a non-positive capacity; 1 is not.
		panic(fmt.Sprintf("sim: control lane: %v", err))
	}
	return l
}

// refuseResnapshot satisfies the one port marketdata's Deps demands from a
// root. The book path is not wired in this slice (§39p-1), so no delta can
// ever be held and no request can ever be made — and if one IS made, the
// wiring claim above is false, which must be loud, not absorbed by a stub
// that quietly succeeds.
type refuseResnapshot struct{}

func (refuseResnapshot) RequestSnapshot(inst ids.VenueInstrument) error {
	return fmt.Errorf("sim: resnapshot requested for %s: %w", inst, ErrPayloadUnrouted)
}

// router is the state-machine side of the sink chain: dispatched envelopes
// routed by payload type, timers to the funding path's namespace.
//
// The two payload cases are the two-hop design (§39p): a raw venue.Funding
// dispatches INTO marketdata, whose published snapshot re-enters the lane
// pre-dispatch and lands here again as a ports.FundingSnapshot for the cache.
// Both hops ride one mailbox per source, ordered by one dispatcher.
type router struct {
	funding *driving.FundingDispatch
	cache   *state.Cache
}

func (r *router) OnEnvelope(e events.Envelope) error {
	switch p := e.Payload.(type) {
	case venue.Funding:
		// The snapshot return travels to the lane through the use case's own
		// publisher port; nothing here forwards it, or it would arrive twice.
		_, err := r.funding.Deliver(e)
		return err
	case ports.FundingSnapshot:
		return r.cache.PutFunding(p)
	}
	return fmt.Errorf("sim: payload %T from %s: %w", e.Payload, e.Source, ErrPayloadUnrouted)
}

func (r *router) OnTimer(_ uint64, f clock.Fired) error {
	// Only the funding path arms timers in this wiring, so its namespace is
	// the only route; a foreign ID surfaces as application.ErrUnknownTimer,
	// which is fatal here — a timer no context claims is a wiring hole.
	_, err := r.funding.Timer(f.ID)
	return err
}
