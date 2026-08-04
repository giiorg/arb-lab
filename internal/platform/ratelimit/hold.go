package ratelimit

import (
	"fmt"
)

type holdState uint8

const (
	holdReserved holdState = iota
	holdConsumed
	holdCanceled
)

type holdEntry struct {
	req     Request
	windows []*window
	state   holdState
}

// Hold is one package's provisional occupancy. Each entry transitions exactly
// once from reserved to consumed or canceled under the governor lock; neither
// terminal can transition again. That state machine makes Consume racing Cancel
// a complete serialization: a sent leg keeps its charge, while a canceled leg
// fails closed before send.
type Hold struct {
	gov     *Governor
	entries []holdEntry
}

// MintedBy answers governor identity without exposing the governor itself. The
// last pre-send seam uses it to fail closed if future wiring ever separates the
// governor that reserved a package from the governor that prices its send.
func (h *Hold) MintedBy(g *Governor) bool {
	return h.gov == g
}

// Consume converts one matching occupancy into a charge at the last local
// admission instant before send. It rechecks only the venue's word: a pause or
// a folded count that no longer fits under absolute capacity. Re-running the
// priority ceiling would let an unrelated local spender invalidate the promise
// reservation exists to make.
func (h *Hold) Consume(r Request) (Decision, error) {
	h.gov.mu.Lock()
	defer h.gov.mu.Unlock()

	var entry *holdEntry
	for i := range h.entries {
		if h.entries[i].state == holdReserved && h.entries[i].req == r {
			entry = &h.entries[i]
			break
		}
	}
	if entry == nil {
		return Decision{}, fmt.Errorf("request has no reserved entry: %w", ErrHoldEntryUnmatched)
	}

	now := h.gov.clk.Now()
	d := Decision{Venue: h.gov.profile.Venue, At: now}
	if now.Before(h.gov.pausedUntil) {
		d.Reason = ReasonPaused
		d.NotBefore = h.gov.pausedUntil
		return d, nil
	}

	refused := false
	for _, w := range entry.windows {
		used := w.usedAt(now)
		units := r.Cost.units(w.limit.Dimension)
		if used <= w.limit.Capacity && units <= w.limit.Capacity-used {
			continue
		}
		at := w.freeAt(now, units, w.limit.Capacity)
		if !refused || worseThan(at, d.NotBefore) {
			d.NotBefore = at
			d.Limit = w.limit.Name
		}
		refused = true
	}
	if refused {
		d.Reason = ReasonCeiling
		return d, nil
	}

	entry.state = holdConsumed
	for _, w := range entry.windows {
		w.consume(now, r.Cost.units(w.limit.Dimension))
	}
	d.Admitted = true
	d.Reason = ReasonAdmitted
	return d, nil
}

// Cancel releases every entry that never reached send. Consumed entries are
// untouched because only the venue can say that their charge no longer counts;
// terminal states make repeated cancellation an idempotent no-op.
func (h *Hold) Cancel() error {
	h.gov.mu.Lock()
	defer h.gov.mu.Unlock()

	for i := range h.entries {
		entry := &h.entries[i]
		switch entry.state {
		case holdReserved:
			entry.state = holdCanceled
			for _, w := range entry.windows {
				w.release(entry.req.Cost.units(w.limit.Dimension))
			}
		case holdConsumed:
			// The venue counted the call, so local completion is not authority to
			// release its charge early.
		case holdCanceled:
			// Idempotence is required because package cleanup blankets every exit.
		}
	}
	return nil
}
