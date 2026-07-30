package replay

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/events"
)

// Rerun runs the directed pump: the dispatch log at logPath decides every
// step and this pump never arbitrates — §3.5's replay, "not regenerating
// dispatch order". The tape supplies payload content (the log stores
// sequences; the archive stays the content authority, §39n), a fresh Ingester
// re-derives the same envelopes, and each log entry is cross-checked against
// what the archive-derived world actually produces before it is applied.
//
// The clock trajectory is reproduced, not approximated: an envelope delivers
// at the instant of the batch that produced it (ingestion moves the clock
// exactly as Generate did), a timer at its scheduled instant — so every
// freshness verdict computed inside the sink chain is computed against the
// same Now both times. Wrap the sink in a fresh LogWriter and the strongest
// form of §39g's claim is checkable for free: the re-driven log byte-equals
// the log it followed.
func Rerun(t *Tape, ing *Ingester, c *clock.TestClock, lane *events.MarketDataLane, sink dispatch.Sink, logPath string, ordinary func(error) bool) (Report, dispatch.LogReport, error) {
	a := &applier{t: t, ing: ing, c: c, lane: lane, sink: sink, ordinary: ordinary}
	lr, err := dispatch.ReplayLog(logPath, a)
	if err != nil {
		return a.rep, lr, err
	}
	a.rep.RestFailures = ing.RestFailures()
	return a.rep, lr, nil
}

// applier is the dispatch.LogConsumer that follows the log.
type applier struct {
	t        *Tape
	ing      *Ingester
	c        *clock.TestClock
	lane     *events.MarketDataLane
	sink     dispatch.Sink
	ordinary func(error) bool

	i   int // next tape entry to ingest
	rep Report
}

// OnEnvelopeRecord materializes the entry's envelope from the archive and
// applies it. The mailbox head either matches the entry on every frozen
// field or the two records of the run have diverged — and a divergence is
// refused with the sequence position named, never repaired, because neither
// artifact is authoritative over the other's half (the log owns order, the
// archive owns content, and a mismatch means at least one is not from this
// run).
func (a *applier) OnEnvelopeRecord(r dispatch.EnvelopeRecord) error {
	e, err := a.materialize(r)
	if err != nil {
		return err
	}

	if e.IngressSeq != 0 {
		return fmt.Errorf("replay: rerun seq %d: head already stamped %d: %w",
			r.IngressSeq, e.IngressSeq, ErrDiverged)
	}
	e.IngressSeq = r.IngressSeq
	if err := e.Validate(); err != nil {
		return fmt.Errorf("replay: rerun seq %d: %w", r.IngressSeq, err)
	}

	a.rep.Dispatched++
	if err := a.sink.OnEnvelope(e); err != nil {
		if a.ordinary != nil && a.ordinary(err) {
			a.rep.Ordinary++
			return nil
		}
		return err
	}
	return nil
}

// materialize ingests tape entries forward until the entry's source mailbox
// offers a head, then cross-checks it against the entry.
func (a *applier) materialize(r dispatch.EnvelopeRecord) (events.Envelope, error) {
	for {
		if e, ok := a.lane.Peek(r.Source); ok {
			if err := match(e, r); err != nil {
				return events.Envelope{}, err
			}
			got, ok := a.lane.Consume(r.Source)
			if !ok {
				// Single-driver discipline makes Peek stable; an evaporated
				// head means that discipline broke.
				return events.Envelope{}, fmt.Errorf("replay: rerun seq %d: head evaporated: %w",
					r.IngressSeq, ErrDiverged)
			}
			return got, nil
		}
		if err := a.ingestNextBatch(fmt.Sprintf("envelope %d from %s", r.IngressSeq, r.Source)); err != nil {
			return events.Envelope{}, err
		}
	}
}

// ingestNextBatch consumes one same-instant batch — the same unit Generate
// ingests, so the clock visits the same instants in the same order.
func (a *applier) ingestNextBatch(waitingFor string) error {
	if a.i >= a.t.Len() {
		return fmt.Errorf("replay: rerun: log demands %s but the tape is exhausted: %w",
			waitingFor, ErrDiverged)
	}
	next := a.t.entries[a.i].rec.RecvTime
	if err := a.c.SetNow(next); err != nil {
		return fmt.Errorf("replay: rerun at entry %d: %w", a.i, err)
	}
	for a.i < a.t.Len() && a.t.entries[a.i].rec.RecvTime.Equal(next) {
		env, emitted, err := a.ing.Ingest(a.t.entries[a.i])
		a.i++
		a.rep.Ingested++
		if err != nil {
			return err
		}
		if !emitted {
			continue
		}
		if err := a.lane.Publish(env); err != nil {
			return fmt.Errorf("replay: rerun publish %s: %w", env.Source, err)
		}
		a.rep.Emitted++
	}
	return nil
}

// match compares a mailbox head against the log entry, field by frozen field.
func match(e events.Envelope, r dispatch.EnvelopeRecord) error {
	switch {
	case e.AcceptOrdinal != r.AcceptOrdinal:
		return divergence(r, "accept ordinal", e.AcceptOrdinal, r.AcceptOrdinal)
	case !e.RecvTime.Equal(r.RecvTime):
		return divergence(r, "recv time", e.RecvTime, r.RecvTime)
	case !e.SourceTime.Equal(r.SourceTime):
		return divergence(r, "source time", e.SourceTime, r.SourceTime)
	case e.VenueSeq != r.VenueSeq:
		return divergence(r, "venue seq", e.VenueSeq, r.VenueSeq)
	case e.Prio != r.Prio:
		return divergence(r, "prio", e.Prio, r.Prio)
	}
	return nil
}

func divergence(r dispatch.EnvelopeRecord, field string, got, want any) error {
	return fmt.Errorf("replay: rerun seq %d from %s: %s %v, log froze %v: %w",
		r.IngressSeq, r.Source, field, got, want, ErrDiverged)
}

// OnTimerRecord reproduces one timer firing: the clock moves to the frozen
// instant, and the TimerSource must yield the same firing the log froze — a
// missing or different timer means the sink chain armed differently this
// run, which is a divergence, not a repairable gap.
//
// Before the clock moves, every batch STRICTLY before the firing instant is
// ingested — Generate's own invariant, restated: it advances to a timer only
// when no earlier record remains, and its tie rule fires a timer BEFORE an
// equal-instant record. Without this pre-ingest the lazy pull (batches
// ingested only when an envelope entry demands a head) leaves the
// no-news tail before a timer unconsumed, and the next demanded batch would
// sit in the clock's past — the first fixture run found exactly that as a
// time-would-move-backwards refusal.
func (a *applier) OnTimerRecord(r dispatch.TimerRecord) error {
	for a.i < a.t.Len() && a.t.entries[a.i].rec.RecvTime.Before(r.Fired.At) {
		if err := a.ingestNextBatch(fmt.Sprintf("timer %d at %s", r.IngressSeq, r.Fired.At.Format(time.RFC3339Nano))); err != nil {
			return err
		}
	}
	if err := a.c.SetNow(r.Fired.At); err != nil {
		return fmt.Errorf("replay: rerun timer seq %d: %w", r.IngressSeq, err)
	}
	f, ok := a.c.NextDue()
	if !ok {
		return fmt.Errorf("replay: rerun timer seq %d: log froze %q at %s but nothing is due: %w",
			r.IngressSeq, r.Fired.ID, r.Fired.At.Format(time.RFC3339Nano), ErrDiverged)
	}
	if f.ID != r.Fired.ID || !f.At.Equal(r.Fired.At) || f.Prio != r.Fired.Prio {
		return fmt.Errorf("replay: rerun timer seq %d: due (%q, %s, %d), log froze (%q, %s, %d): %w",
			r.IngressSeq, f.ID, f.At.Format(time.RFC3339Nano), f.Prio,
			r.Fired.ID, r.Fired.At.Format(time.RFC3339Nano), r.Fired.Prio, ErrDiverged)
	}

	a.rep.Dispatched++
	if err := a.sink.OnTimer(r.IngressSeq, f); err != nil {
		if a.ordinary != nil && a.ordinary(err) {
			a.rep.Ordinary++
			return nil
		}
		return err
	}
	return nil
}
