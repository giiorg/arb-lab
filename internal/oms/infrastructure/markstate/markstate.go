package markstate

import (
	"errors"
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/state"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// ErrMarkCacheNil reports a mark-risk adapter constructed over no cache.
//
// Refused at construction rather than answering false for ever, and the two are
// not close: an adapter over a nil cache would report "no mark for this
// instrument" on every read, which is the same answer a healthy process gives
// at boot — so §7.1's trigger would wait for a mark that could never arrive,
// with nothing anywhere reporting a fault.
var ErrMarkCacheNil = errors.New("oms mark risk: no market-state cache")

// Reader is ports.MarkRiskReader over engine/state's latest-value cache
// — §4's "OMS-owned mark-risk read port over engine/state", and the adapter
// §7.1's liq-distance trigger reads the mark through.
//
// # It lives HERE and not in engine/state, and §3.6 rule 6 is the whole reason
//
// The obvious home is engine/state, beside StrategyReader — strategy's own
// ports doc even names that pattern: adapters live in infrastructure/driven
// "or, for the read seam, in the engine subsystem that holds the data". That
// precedent does NOT transfer to this context, and the transfer was attempted
// and refused.
//
// Rule 6 closes oms/application/ports to every package outside oms, PORT as
// well as adapter, "because depending on the abstraction is the recommended way
// to depend on anything". So an engine/state file implementing this interface
// has to import the closed package, and `singlewriter` names it. The rule is
// about the PACKAGE rather than about which interface is being taken, and that
// is what makes it enforceable: an exemption reading "this one is only a read
// port" would have to be re-judged for every member added afterwards, by
// whoever added it.
//
// The ordinary hexagonal shape has no such problem. This is the consumer's own
// infrastructure layer, which may import engine/ (rule 1 classifies the
// IMPORTER, and infrastructure/ is not an inward layer) and may import its own
// ports. Nothing needs loosening.
//
// # And it is its OWN package, not a file in infrastructure/driven — because a
// # PACKAGE is the unit of dependency
//
// The first attempt put it beside the journal, gateway and position adapters,
// and app/reconcile's import fence refused THAT: this adapter's backing store
// is engine/state, whose graph reaches marketdata and strategy, and the
// reconcile-only boot root imports infrastructure/driven for adapters it
// genuinely needs. One file was therefore enough to pull two forbidden contexts
// into a boot binary transitively, with every individual import defensible.
//
// The fence's own comment names the shape it was written to catch — "a
// composition root is exactly where such an edge arrives most naturally: while
// we are here, read the mark off marketdata so the report can price the delta"
// — and it caught it one hop out, through a package nobody was thinking about
// the graph of.
//
// The split is honest rather than a workaround, and the test is what each
// adapter is over. driven/'s are over durable stores and venue surfaces, which
// a boot must reach to establish what this process may do. This one is over the
// LIVE market-state pipeline, which by §7.5's own argument does not exist yet at
// that point. An adapter whose dependency a boot must not acquire does not
// belong in the package a boot imports.
//
// # It computes the AGES, and that is the work this adapter exists to do
//
// The port returns durations, not a watermark — ports.MarkRiskReader records
// why, and the consequence lands here: this package may import marketdata, so
// the age arithmetic runs through domain.Watermark's own Age and DataAge rather
// than being restated in the consumer. The half a reader gets wrong is that the
// DATA age is measured against the venue's server time and never the engine's,
// and keeping one implementation is what stops that being got wrong twice.
//
// The clock-inversion error propagates rather than being flattened. A negative
// age means two clocks disagree, and marketdata's rule is that this is
// FreshnessUnknown and not fresh; an adapter that returned the negative
// duration would hand §7.1 an age that compares as fresher than everything.
//
// The AcceptOrdinal is deliberately dropped: it is §3.5 provenance for the
// capture lane, and a liq-distance computation has no use for it. See
// ports.MarkReading for why the projection is narrow rather than "everything
// the cache holds".
type Reader struct {
	c *state.Cache
}

// The §3.6 rule 2 assertion. Breaking conformance is a NOBUILD, which for this
// line means CAUGHT.
var _ ports.MarkRiskReader = Reader{}

// NewReader wraps the cache, refusing a nil one.
//
// An error rather than a panic, unlike engine/state's NewStrategyReader: this
// constructor runs in the OMS composition root beside the others in this
// package, which return errors, and a root that panics on one dependency and
// reports another is a root whose failure mode depends on which field was
// forgotten.
func NewReader(c *state.Cache) (Reader, error) {
	if c == nil {
		return Reader{}, ErrMarkCacheNil
	}
	return Reader{c: c}, nil
}

// Mark implements ports.MarkRiskReader.
//
// Absence answers false rather than a zero reading — the cache's own rule, and
// the one that bites hardest for this class: a zero-valued MarkReading is a
// mark of zero, and §7.1's liq distance divides by it.
//
// The two ages are computed here, at the instants the caller supplied, through
// marketdata's own Age and DataAge. An inversion in EITHER refuses the whole
// reading rather than returning the one age that computed: a caller handed a
// reading with a valid transport age and a silently zeroed data age would read
// a venue-side stall as a fresh view, which is the failure the second age
// exists to catch.
func (m Reader) Mark(vi ids.VenueInstrument, now, serverNow time.Time) (ports.MarkReading, bool, error) {
	e, ok := m.c.Mark(vi)
	if !ok {
		return ports.MarkReading{}, false, nil
	}
	age, err := e.Age(now)
	if err != nil {
		return ports.MarkReading{}, false, fmt.Errorf("oms mark risk: %s transport age: %w", vi, err)
	}
	dataAge, err := e.DataAge(serverNow)
	if err != nil {
		return ports.MarkReading{}, false, fmt.Errorf("oms mark risk: %s data age: %w", vi, err)
	}
	return ports.MarkReading{
		Instrument: e.Observation.Instrument,
		Price:      e.Observation.Price,
		Age:        age,
		DataAge:    dataAge,
	}, true, nil
}
