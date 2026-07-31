package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Side is a leg's direction, in this context's own spelling.
//
// oms/domain declares a Side with the same two members, and importing it was
// considered and refused: an Intent is the strategy's output vocabulary and an
// Order is the OMS's input vocabulary, and the place they meet is the
// milestone-3 translation the foundation owns (§3.3: "the foundation owns …
// execution"). Importing would make the pure signal layer's domain depend on
// the order-management context to spell "buy" — a dependency arrow between two
// peer contexts bought to avoid one two-line translation that has to exist
// anyway. The two spellings are kept identical on purpose, so the translation
// is a rename and never a mapping table.
type Side string

const (
	// SideBuy acquires the base asset — the cash-and-carry's spot leg.
	SideBuy Side = "buy"

	// SideSell disposes of it — on a perpetual, the short that collects
	// funding.
	SideSell Side = "sell"
)

// Validate reports whether s is one of the declared directions. The empty
// side takes the same path as an unknown one: with two members there is no
// "which column" question a separate unset sentinel would answer.
func (s Side) Validate() error {
	switch s {
	case SideBuy, SideSell:
		return nil
	}
	return fmt.Errorf("%q: %w", string(s), ErrSideInvalid)
}

func (s Side) String() string { return string(s) }

// IntentKind is §3.3's OpenTrade | CloseTrade.
//
// `type X string` under §3.6 rule 8: an Intent's fields reach the decision
// journal through the Decision that produced it, and a stored token should
// describe itself.
type IntentKind string

const (
	// IntentOpen proposes opening a package — exposure-increasing, subject to
	// stop-opening (§5.3).
	IntentOpen IntentKind = "open_trade"

	// IntentClose proposes closing one — risk-reducing, permitted while
	// stop-opening, else a now-negative carry could not be stood down (§5.3).
	IntentClose IntentKind = "close_trade"
)

// Validate reports whether k is one of the declared kinds.
func (k IntentKind) Validate() error {
	switch k {
	case IntentOpen, IntentClose:
		return nil
	}
	return fmt.Errorf("%q: %w", string(k), ErrKindInvalid)
}

func (k IntentKind) String() string { return string(k) }

// LegSpec names one leg of an Intent: which instrument, and which way.
//
// §3.3 says only that "each leg names a VenueInstrument (spot vs perp on one
// symbol are distinct)"; the direction is here because a leg without one
// defers "which way" to the consumer, and a consumer inferring direction from
// product family would invert the moment a strategy shorts spot margin or
// longs a perp (§2.3's cross-venue trade does exactly that).
type LegSpec struct {
	Instrument ids.VenueInstrument
	Side       Side
}

// Validate reports whether the leg names a real instrument and direction.
func (l LegSpec) Validate() error {
	if err := l.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if err := l.Side.Validate(); err != nil {
		return fmt.Errorf("side: %w", err)
	}
	return nil
}

// Intent is §3.3's strategy output, verbatim in field and meaning.
type Intent struct {
	// IntentID is the strategy-minted dedupe key — re-emitting the same
	// signal is idempotent [→ R2-#8].
	IntentID ids.IntentID

	// Revision supersedes a prior IntentID's spec (e.g. re-sized on a fresh
	// snapshot). Counting from 1; zero is the unset value Validate refuses,
	// for §5.1's OCC reason — a zero from a caller that never populated it
	// must match nothing.
	Revision uint64

	// ValidUntil is engine Clock time; a stale Intent is dropped, never
	// executed late. Stamped by the APPLICATION layer from the injected
	// Clock — the domain core has no business knowing the wall time (§40b).
	ValidUntil time.Time

	// Kind is OpenTrade | CloseTrade.
	Kind IntentKind

	// Legs each name a VenueInstrument; spot and perp on one symbol are
	// distinct [→ Z1].
	Legs []LegSpec

	// TargetQty is dimensional and identical across legs for exact
	// neutrality, lot-quantized per §2.4's intersection rule.
	TargetQty fixedpoint.Quantity

	// Reason is for the audit log.
	Reason string
}

// Validate reports whether the Intent could have been emitted by a conforming
// strategy. Field order follows the struct, outermost mistake first.
func (i Intent) Validate() error {
	if err := i.IntentID.Validate(); err != nil {
		return fmt.Errorf("intent id: %w", err)
	}
	if i.Revision == 0 {
		return fmt.Errorf("revision: %w", ErrUnset)
	}
	if i.ValidUntil.IsZero() {
		return fmt.Errorf("valid until: %w", ErrUnset)
	}
	if err := i.Kind.Validate(); err != nil {
		return fmt.Errorf("kind: %w", err)
	}
	if len(i.Legs) == 0 {
		return fmt.Errorf("legs: %w", ErrUnset)
	}
	for n, l := range i.Legs {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("leg %d: %w", n, err)
		}
	}
	if err := i.TargetQty.Validate(); err != nil {
		return fmt.Errorf("target qty: %w", err)
	}
	if i.Reason == "" {
		return fmt.Errorf("reason: %w", ErrUnset)
	}
	return nil
}
