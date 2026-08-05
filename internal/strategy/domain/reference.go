package domain

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// This file holds the three faces of the ReferenceData seam (§40b): fees, lot
// rules, and the hurdle benchmark — one mechanism, dated reference revisions
// with provenance refusal, consumed as-of an instant.

// ProvenanceClass names who wrote a reference revision. §40d's composition
// rule makes the fetcher one more WRITER of revisions; this label is what
// keeps a hand-recorded revision from ever impersonating a fetched one — the
// same labeled-provenance rule the §38 manifest backfill established.
type ProvenanceClass string

const (
	// ProvenanceFetched marks a revision landed by a programmatic fetch from
	// the venue. Two writer shapes carry it, split by what the endpoint
	// answers for (§69): FEE rows require the authenticated fetch (the venue
	// answers for the ACCOUNT's tier; milestone 3/4 — no adapter writes fee
	// rows yet, and §40d's account-real precondition keys on exactly that),
	// while LOT-FILTER rows are written by the unsigned public fetch (arblab
	// reference fetch), because there the public endpoint is the venue's own
	// definition of the fact.
	ProvenanceFetched ProvenanceClass = "fetched"

	// ProvenanceHandRecorded marks a revision a human verified and entered,
	// citing dated sources — milestone 2's modeled-tier proxy (§40d's §2.4
	// clause (ii) authorizes it for the pre-capital gate, labeled).
	ProvenanceHandRecorded ProvenanceClass = "hand_recorded"
)

// Validate reports whether c is one of the declared classes.
func (c ProvenanceClass) Validate() error {
	switch c {
	case ProvenanceFetched, ProvenanceHandRecorded:
		return nil
	}
	return fmt.Errorf("%q: %w", string(c), ErrKindInvalid)
}

func (c ProvenanceClass) String() string { return string(c) }

// Provenance is a reference fact's origin: who wrote it, when it was
// observed, and which document revision carries it. §40d: "a schedule
// without provenance is a hardcoded constant wearing a config file's
// clothes" — so absence of any member is a refusal.
type Provenance struct {
	// Class is who wrote the revision.
	Class ProvenanceClass

	// ObservedAt is when the fact was read from its source (the Profiles
	// verification date for a hand-recorded revision).
	ObservedAt time.Time

	// Rev identifies the reference document revision that carries the fact.
	Rev config.Revision
}

// Validate reports whether the provenance is complete.
func (p Provenance) Validate() error {
	if err := p.Class.Validate(); err != nil {
		return fmt.Errorf("class: %w", err)
	}
	if p.ObservedAt.IsZero() {
		return fmt.Errorf("observed at: %w", ErrUnset)
	}
	if err := p.Rev.Validate(); err != nil {
		return fmt.Errorf("rev: %w", err)
	}
	return nil
}

// LegFees is one leg's fee schedule row: maker and taker, per §2.4 fetched
// separately per (venue, product) — "spot taker fees are materially higher
// than perp and MUST be fetched separately".
type LegFees struct {
	Instrument ids.VenueInstrument

	// Maker and Taker are per-side rates. A ZERO rate is admissible — venue
	// promotions publish real 0.00% maker tiers — and an UNSET one is not:
	// fixedpoint.Rate's set bit is what tells §40d's "an unpopulated rate
	// reads as zero fee" apart from a venue's actual zero.
	Maker, Taker fixedpoint.Rate

	// Tier labels the fee tier the numbers describe — "VIP0" for §40d's
	// modeled-tier proxy, the account's tier for a fetched revision. The
	// milestone-2 gate's verdict is labeled as scored on this tier, which is
	// clause (ii)'s requirement; a tier lives here rather than in Provenance
	// because it is a FEE fact — lot filters and the hurdle have none.
	Tier string

	// Provenance is the row's origin; see Provenance.
	Provenance Provenance
}

// Validate reports whether the row could be consumed by a decision.
func (f LegFees) Validate() error {
	if err := f.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if err := f.Maker.Validate(); err != nil {
		return fmt.Errorf("maker: %w", err)
	}
	if err := f.Taker.Validate(); err != nil {
		return fmt.Errorf("taker: %w", err)
	}
	if f.Tier == "" {
		return fmt.Errorf("tier: %w", ErrUnset)
	}
	if err := f.Provenance.Validate(); err != nil {
		return fmt.Errorf("provenance: %w", err)
	}
	return nil
}

// LotRules is one leg's quantity constraints: §40g's inputs to the
// intersection quantization. Venue reference facts, same dated-revision
// pattern as fees.
type LotRules struct {
	Instrument ids.VenueInstrument

	// Step is the lot increment: every order quantity is a whole multiple.
	Step fixedpoint.Quantity

	// MinQty is the smallest order the venue admits. May be declared zero.
	MinQty fixedpoint.Quantity

	// MinNotional is the venue's minimum order value. Carried because the
	// venue publishes it beside the lot filter and a sizing that ignores it
	// produces Intents the venue will reject; may be declared zero where a
	// venue has none.
	MinNotional fixedpoint.Money

	// Tick is the venue's price increment (Binance PRICE_FILTER tickSize;
	// Bybit priceFilter.tickSize), carried beside the lot members because the
	// venue publishes them as one filter set and the order path verifies them
	// as one (§2.4's verification half; oms/domain.TradingFilters).
	//
	// It is OPTIONAL here — the zero value means "not recorded" — where Step
	// is not, and the asymmetry is each consumer's own safe direction, not an
	// oversight: this context's scoring path quantizes QUANTITY and never
	// consumes a tick, while revisions journaled before the field existed
	// must keep decoding. The consumer that does need it — the order path's
	// filters resolver — REFUSES a row whose tick is absent, so the optional
	// zero here cannot reach a venue as an unchecked price; it can only
	// refuse a plan while naming this row.
	Tick fixedpoint.Price

	// Provenance is the row's origin.
	Provenance Provenance
}

// Validate reports whether the rules could quantize anything.
func (l LotRules) Validate() error {
	if err := l.Instrument.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	if err := l.Step.Validate(); err != nil {
		return fmt.Errorf("step: %w", err)
	}
	sign, err := l.Step.Sign()
	if err != nil {
		return fmt.Errorf("step: %w", err)
	}
	// A zero step admits every quantity, which is the unpopulated field
	// reading as "no constraint" — the flattering direction. Refused.
	if sign <= 0 {
		return fmt.Errorf("step %s: %w", l.Step, ErrUnset)
	}
	if err := l.MinQty.Validate(); err != nil {
		return fmt.Errorf("min qty: %w", err)
	}
	if err := l.MinNotional.Validate(); err != nil {
		return fmt.Errorf("min notional: %w", err)
	}
	// A PRESENT tick must be coherent; only absence is admitted. A declared
	// zero tick is refused for Step's reason — it reads as "every price
	// conforms", the flattering direction.
	if l.Tick != (fixedpoint.Price{}) {
		if err := l.Tick.Validate(); err != nil {
			return fmt.Errorf("tick: %w", err)
		}
		if l.Tick.Mantissa() <= 0 {
			return fmt.Errorf("tick %s: %w", l.Tick, ErrUnset)
		}
		if l.Tick.Base() != l.Step.Asset() {
			return fmt.Errorf("tick prices %s, step counts %s: %w",
				l.Tick.Base(), l.Step.Asset(), ErrKindInvalid)
		}
		if l.Tick.Quote() != l.MinNotional.Asset() {
			return fmt.Errorf("tick quoted in %s, min notional in %s: %w",
				l.Tick.Quote(), l.MinNotional.Asset(), ErrKindInvalid)
		}
	}
	if err := l.Provenance.Validate(); err != nil {
		return fmt.Errorf("provenance: %w", err)
	}
	return nil
}

// HurdleBenchmark is §2.1's opportunity floor: dated, sourced,
// haircut-adjusted, revision-controlled — "never a hardcoded 4.6% constant".
// The components are carried separately rather than pre-netted so the
// journaled Decision can show the haircut arithmetic, not just its result.
type HurdleBenchmark struct {
	// AnnualRate is the benchmark's published annualized rate.
	AnnualRate fixedpoint.Rate

	// AccessCost, LiquidityHaircut and CounterpartyHaircut are §2.1's three
	// recorded adjustments. Each may be a declared zero; none may be absent.
	AccessCost, LiquidityHaircut, CounterpartyHaircut fixedpoint.Rate

	// AsOf is the benchmark reading's own date (distinct from when we
	// recorded it — that is Provenance.ObservedAt).
	AsOf time.Time

	// Provenance is the row's origin.
	Provenance Provenance
}

// Validate reports whether the benchmark is complete.
func (h HurdleBenchmark) Validate() error {
	if err := h.AnnualRate.Validate(); err != nil {
		return fmt.Errorf("annual rate: %w", err)
	}
	if err := h.AccessCost.Validate(); err != nil {
		return fmt.Errorf("access cost: %w", err)
	}
	if err := h.LiquidityHaircut.Validate(); err != nil {
		return fmt.Errorf("liquidity haircut: %w", err)
	}
	if err := h.CounterpartyHaircut.Validate(); err != nil {
		return fmt.Errorf("counterparty haircut: %w", err)
	}
	if h.AsOf.IsZero() {
		return fmt.Errorf("as of: %w", ErrUnset)
	}
	if err := h.Provenance.Validate(); err != nil {
		return fmt.Errorf("provenance: %w", err)
	}
	return nil
}
