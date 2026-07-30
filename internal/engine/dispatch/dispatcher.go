package dispatch

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// Sink is where dispatched items go — the state-machine side of the dispatcher.
// In slice C's wiring it is engine/state behind an adapter; in slice B the
// dispatch log writer joins it. The two methods share one sequence: a timer
// firing and an envelope are both consumed inputs under §3.5's total order, so
// the seq handed to OnTimer and the IngressSeq stamped into an OnEnvelope
// envelope come from the same dense counter.
//
// A timer's seq rides beside the Fired rather than inside it, because Fired is
// clock's type and teaching the clock about dispatch sequences would put the
// replay key in two packages. An envelope's seq is NOT passed beside it — the
// envelope already has the field, and a second copy is a second thing to
// disagree.
type Sink interface {
	// OnEnvelope receives a dispatched envelope, IngressSeq stamped and
	// Validate already passed.
	OnEnvelope(e events.Envelope) error

	// OnTimer receives a fired timer and the sequence position its firing
	// occupies in the consumed order. f.At is the SCHEDULED instant — identical
	// in a replay — never the delivery instant.
	OnTimer(seq uint64, f clock.Fired) error
}

// Config wires a Dispatcher. Every field is required: the zero value is
// unusable (§11), and a dispatcher missing an input would not be wrong loudly —
// it would simply never dispatch that kind, which reads downstream as a world
// where those events never happen.
type Config struct {
	Control *events.ControlLane
	Market  *events.MarketDataLane
	Timers  clock.TimerSource
	Sink    Sink
}

// Validate reports whether the config could run.
func (c Config) Validate() error {
	if c.Control == nil {
		return ErrControlLaneNil
	}
	if c.Market == nil {
		return ErrMarketLaneNil
	}
	if c.Timers == nil {
		return ErrTimerSourceNil
	}
	if c.Sink == nil {
		return ErrSinkNil
	}
	return nil
}

// Dispatcher is §3.5's single total-order dispatcher. One caller pumps Step;
// that contract is about ordering authority, not memory safety — the mutex
// below makes concurrent misuse memory-safe so the race detector reports it as
// the logic error it is rather than as corruption.
type Dispatcher struct {
	mu      sync.Mutex
	control *events.ControlLane
	market  *events.MarketDataLane
	timers  clock.TimerSource
	sink    Sink

	// seq is the last IngressSeq assigned. It advances ONLY when an item is
	// released to the sink, which is what makes the dispatched sequence dense:
	// a refused envelope burns nothing, so the log slice B writes has no holes
	// to explain.
	seq uint64

	// prios holds each source's Prio, fixed by its first DISPATCHED envelope —
	// not its first offered one, because a refused envelope's claim about its
	// source never entered the consumed order and should bind nothing.
	prios map[events.SourceID]events.Prio

	// lastMarket is the round-robin cursor over market sources. A pure function
	// of dispatch history, which is what makes the rotation replay-safe: the
	// log freezes the history, so it freezes the cursor.
	lastMarket    events.SourceID
	hasLastMarket bool
}

// New returns a Dispatcher over cfg's inputs.
func New(cfg Config) (*Dispatcher, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Dispatcher{
		control: cfg.Control,
		market:  cfg.Market,
		timers:  cfg.Timers,
		sink:    cfg.Sink,
		prios:   make(map[events.SourceID]events.Prio),
	}, nil
}

// LastSeq reports the most recently assigned IngressSeq — zero before the first
// dispatch. For assertions and metrics, never for minting: the next sequence is
// this dispatcher's to assign.
func (d *Dispatcher) LastSeq() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.seq
}

// Step dispatches at most one item and reports whether it dispatched one.
//
// The rank between input kinds is fixed, and it is a rank rather than a merge
// because the primitives are pop-only on purpose — TimerSource and the lanes
// each refuse to be peeked across kinds, so any content-aware arbitration
// between kinds would mean popping an item and holding it back, which is the
// re-queuing the pull design exists to forbid:
//
//  1. A due timer, in the queue's own (At, Prio, ID) order. A due timer is a
//     deadline already in the past — §3.4's silence timer and the funding
//     boundary are exactly the events that must not queue behind traffic.
//  2. The control lane's head, FIFO. Lossless private truth — fills, account
//     events, commands — precedes lossy derived data, the same rank §3.4's
//     lane list states. Deterministic because a FIFO's head is its only
//     candidate.
//  3. One ready market-data mailbox's head — see pickMarket for the
//     arbitration.
//
// The rank is honest about starvation: continuous due timers or nonstop
// control traffic starve market data. That direction is chosen — safety truth
// first — and bounded in practice by both being low-rate lanes; the reverse
// starvation is the one that kills (a book firehose delaying a fill).
//
// An unready source never stalls a ready one (§39f-2): Step never blocks, an
// empty mailbox is simply not a candidate, and there is no cross-source order
// to wait for.
//
// On a refusal (ErrIngressSeqMinted, ErrPrioInconstant, a Validate failure)
// the offending envelope has been consumed from its lane — leaving it there
// would jam its source forever — but was NOT dispatched: no sequence is
// burned, nothing reached the sink, and the error names the source. Step
// returns (false, err) and the driver decides whether the process survives.
// A sink error is the opposite case: the item WAS dispatched — the order was
// decided and the stamp spent — so Step returns (true, err).
func (d *Dispatcher) Step() (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if f, ok := d.timers.NextDue(); ok {
		d.seq++
		return true, d.sink.OnTimer(d.seq, f)
	}

	if e, ok := d.control.TryReceive(); ok {
		return d.release(e)
	}

	if s, ok := d.pickMarket(); ok {
		e, ok := d.market.Consume(s)
		if !ok {
			// The head evaporated between Peek and Consume — a concurrent
			// producer's overflow evicted it. Live arbitration is allowed to
			// race (§3.5 freezes outcomes, not deliberations); re-arbitrating
			// next Step is the correct recovery, not an error.
			return false, nil
		}
		d.lastMarket, d.hasLastMarket = s, true
		return d.release(e)
	}

	return false, nil
}

// release stamps e and hands it to the sink. The check order is the order a
// reader would debug in: the second-authority refusal first (a minted
// IngressSeq is a producer contesting this package's one job), then the
// post-stamp Validate §3.5 places at exactly this boundary — it requires both
// sequence fields, which is why it cannot run producer-side — then the
// cross-envelope constancy check, judged only on an envelope that is itself
// well-formed.
func (d *Dispatcher) release(e events.Envelope) (bool, error) {
	if e.IngressSeq != 0 {
		return false, fmt.Errorf("dispatch: source %s arrived carrying IngressSeq %d: %w",
			e.Source, e.IngressSeq, ErrIngressSeqMinted)
	}

	e.IngressSeq = d.seq + 1
	if err := e.Validate(); err != nil {
		return false, fmt.Errorf("dispatch: source %s: %w", e.Source, err)
	}

	if fixed, known := d.prios[e.Source]; known && fixed != e.Prio {
		return false, fmt.Errorf("dispatch: source %s published Prio %d after its first dispatch fixed %d: %w",
			e.Source, e.Prio, fixed, ErrPrioInconstant)
	}

	d.seq++
	d.prios[e.Source] = e.Prio
	return true, d.sink.OnEnvelope(e)
}

// pickMarket chooses which ready market mailbox dispatches next: the most
// urgent head Prio wins, and ties rotate round-robin from the last market
// source dispatched.
//
// Prio first because that is what the per-source constant is FOR — a safety
// stream outranking bulk data in a drain — and the constancy rule (§39f-5) is
// what makes arbitrating on it sound: a constant cannot reorder a source
// against itself. Round-robin within a Prio class because the alternative,
// first-in-sorted-order, would let a firehose source starve an equally urgent
// peer for as long as it stays ready — which for a book stream is always.
//
// Determinism: the candidate scan runs over a SORTED roster, never over
// map-iteration order. Lane.Sources() is map-random and Go randomises that
// deliberately; an arbitration that read it raw would give the sim pump a
// different dispatch log on every run, which is the exact property the parity
// probe (§39j slice C) exists to refuse.
func (d *Dispatcher) pickMarket() (events.SourceID, bool) {
	sources := d.market.Sources()
	slices.SortFunc(sources, cmpSource)

	var cands []events.SourceID
	var best events.Prio
	for _, s := range sources {
		head, ok := d.market.Peek(s)
		if !ok {
			continue
		}
		switch {
		case len(cands) == 0 || head.Prio.MoreUrgentThan(best):
			cands, best = append(cands[:0], s), head.Prio
		case head.Prio == best:
			cands = append(cands, s)
		}
	}
	if len(cands) == 0 {
		return events.SourceID{}, false
	}

	// Rotate: the first candidate strictly after the cursor in sorted order,
	// wrapping to the front. With no cursor yet, sorted-first.
	if d.hasLastMarket {
		for _, s := range cands {
			if cmpSource(s, d.lastMarket) > 0 {
				return s, true
			}
		}
	}
	return cands[0], true
}

// cmpSource is the total order the market arbitration ranks and rotates over.
//
// Field-wise over all five fields, so it is injective by construction: two
// sources compare equal exactly when they are the same source. SourceID.String
// is deliberately not used — it does not claim injectivity, and a sort key
// that merges two distinct sources would make the round-robin cursor skip one
// of them forever.
//
// The order itself carries no meaning. It exists to be TOTAL and STABLE, so
// that the sim pump's arbitration is a pure function of lane state rather than
// of map iteration; which source sorts first is as arbitrary as the venue
// names are.
func cmpSource(a, b events.SourceID) int {
	if c := strings.Compare(string(a.Instrument.Venue), string(b.Instrument.Venue)); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Instrument.Product), string(b.Instrument.Product)); c != 0 {
		return c
	}
	if c := strings.Compare(a.Instrument.Symbol, b.Instrument.Symbol); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Instrument.Contract), string(b.Instrument.Contract)); c != 0 {
		return c
	}
	return strings.Compare(string(a.Stream), string(b.Stream))
}
