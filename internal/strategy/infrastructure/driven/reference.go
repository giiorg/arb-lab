// Package driven holds the strategy context's driven adapters. Slice A lands
// one: the reference-revision store behind ports.ReferenceData (§40d).
//
// # The schema lives HERE, beside its adapter — not in platform/config
//
// The document types below ride platform/config's load/validate/revision
// machinery (strict decode, schema audit, content-hash revisions), but they
// are declared in this package: the kernel owns the MECHANISM and never a
// context's schema — §11's dependency direction, and the round-4 review's
// recovered catch.
package driven

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/application/ports"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Sentinels.
var (
	// ErrRowInvalid reports a document row that does not parse or is
	// incomplete; the wrapping message names the row and member.
	ErrRowInvalid = fmt.Errorf("strategy/driven: invalid reference row")

	// ErrCellDuplicate reports two rows answering for one cell — a document
	// with a silent winner otherwise.
	ErrCellDuplicate = fmt.Errorf("strategy/driven: duplicate reference cell")

	// ErrCellAbsent reports a lookup for a cell the document does not hold.
	ErrCellAbsent = fmt.Errorf("strategy/driven: no reference row for instrument")

	// ErrNotPeriodCorrect is §40k-2's rule arriving early: an asOf instant
	// for which no period-correct revision exists is a refusal naming the
	// input, never a silent apply-current. In milestone 2's single-revision
	// world that means an asOf BEFORE the revision's observation date — the
	// one store cannot answer for a past it never observed.
	ErrNotPeriodCorrect = fmt.Errorf("strategy/driven: no period-correct revision for this instant")

	// ErrAsOfUnset reports a zero asOf. A zero instant is the forgotten
	// argument, and it would otherwise read as "before every revision" and
	// surface as ErrNotPeriodCorrect — a true message pointing at the wrong
	// cause.
	ErrAsOfUnset = fmt.Errorf("strategy/driven: asOf instant unset")
)

// Document is the milestone-2 reference revision: the four fee cells, the
// four lot rows, and the hurdle benchmark, each row citing its dated source.
// It is loaded through config.Store[Document], whose Revision (content hash,
// source, load instant) becomes every row's provenance.
type Document struct {
	Fees   []FeeRow `json:"fees"`
	Lots   []LotRow `json:"lots"`
	Hurdle Hurdle   `json:"hurdle"`
}

// FeeRow is one (venue, product) fee cell — §40d's granularity: spot and
// perp are separate cells because §2.4 says their numbers materially differ,
// and a symbol column is deliberately absent for milestone 2's BTC universe
// (venue base-tier fees are schedule-wide, and a per-symbol override earns a
// column when a venue publishes one).
type FeeRow struct {
	Venue    string `json:"venue"`
	Product  string `json:"product"`
	Contract string `json:"contract"`

	// Maker and Taker are decimal rates ("0.001" = 10 bps).
	Maker string `json:"maker"`
	Taker string `json:"taker"`

	// Tier labels the published tier the numbers describe ("VIP0").
	Tier string `json:"tier"`

	// Class is who wrote the row: "hand_recorded" | "fetched".
	Class string `json:"class"`

	// ObservedAt is when the source was read, RFC3339.
	ObservedAt string `json:"observed_at"`

	// Source cites where — the venue-profiles row that verified it.
	Source string `json:"source"`
}

// LotRow is one instrument's lot filter.
type LotRow struct {
	Venue    string `json:"venue"`
	Product  string `json:"product"`
	Contract string `json:"contract"`
	Symbol   string `json:"symbol"`

	// Step and MinQty are decimal base-asset quantities; Base names the
	// asset they are measured in.
	Base   string `json:"base"`
	Step   string `json:"step"`
	MinQty string `json:"min_qty"`

	// MinNotional is a decimal amount in Quote; "0" where the venue has no
	// notional floor (a declared zero, distinct from an absent member).
	Quote       string `json:"quote"`
	MinNotional string `json:"min_notional"`

	Class      string `json:"class"`
	ObservedAt string `json:"observed_at"`
	Source     string `json:"source"`
}

// Hurdle is §2.1's benchmark row: rate plus the three recorded adjustments.
type Hurdle struct {
	AnnualRate          string `json:"annual_rate"`
	AccessCost          string `json:"access_cost"`
	LiquidityHaircut    string `json:"liquidity_haircut"`
	CounterpartyHaircut string `json:"counterparty_haircut"`

	// AsOf is the benchmark reading's own date.
	AsOf string `json:"as_of"`

	Class      string `json:"class"`
	ObservedAt string `json:"observed_at"`
	Source     string `json:"source"`
}

// Validate makes the document loadable: every row parses into its domain
// value against a probe revision. Validation by CONVERSION, not by a second
// set of checks — the domain validators are the one authority on what a
// usable row is, and a parallel rule set here would drift.
func (d Document) Validate() error {
	// The probe revision stands in for the real one, which does not exist
	// until config.Store mints it after this returns.
	probe := config.Revision{Num: 1, Hash: [32]byte{1}, Source: "validate", LoadedAt: time.Unix(1, 0).UTC()}

	seenFee := map[ids.VenueInstrument]bool{}
	for n, row := range d.Fees {
		fees, err := row.toDomain(probe)
		if err != nil {
			return fmt.Errorf("fees[%d]: %w", n, err)
		}
		if seenFee[fees.Instrument] {
			return fmt.Errorf("fees[%d] %s: %w", n, fees.Instrument, ErrCellDuplicate)
		}
		seenFee[fees.Instrument] = true
		if row.Source == "" {
			return fmt.Errorf("fees[%d] source: %w", n, ErrRowInvalid)
		}
	}
	if len(seenFee) == 0 {
		return fmt.Errorf("fees: %w", ErrRowInvalid)
	}

	seenLot := map[ids.VenueInstrument]bool{}
	for n, row := range d.Lots {
		lots, err := row.toDomain(probe)
		if err != nil {
			return fmt.Errorf("lots[%d]: %w", n, err)
		}
		if seenLot[lots.Instrument] {
			return fmt.Errorf("lots[%d] %s: %w", n, lots.Instrument, ErrCellDuplicate)
		}
		seenLot[lots.Instrument] = true
		if row.Source == "" {
			return fmt.Errorf("lots[%d] source: %w", n, ErrRowInvalid)
		}
	}
	if len(seenLot) == 0 {
		return fmt.Errorf("lots: %w", ErrRowInvalid)
	}

	if _, err := d.Hurdle.toDomain(probe); err != nil {
		return fmt.Errorf("hurdle: %w", err)
	}
	if d.Hurdle.Source == "" {
		return fmt.Errorf("hurdle source: %w", ErrRowInvalid)
	}
	return nil
}

// feeInstrument is the cell key a FeeRow answers for. The symbol is the
// LOOKUP's; a fee cell is symbol-independent (see FeeRow).
func (r FeeRow) instrument(symbol string) ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    ids.Venue(r.Venue),
		Product:  ids.ProductCategory(r.Product),
		Symbol:   symbol,
		Contract: ids.ContractType(r.Contract),
	}
}

func parseClass(s string) (domain.ProvenanceClass, error) {
	c := domain.ProvenanceClass(s)
	if err := c.Validate(); err != nil {
		return "", err
	}
	return c, nil
}

func parseInstant(field, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("%s: %w", field, ErrRowInvalid)
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q: %w", field, s, ErrRowInvalid)
	}
	return t, nil
}

// toDomain builds the LegFees this row asserts, carrying rev as its
// provenance. The symbol placeholder keeps the probe instrument valid; Fees()
// substitutes the lookup's.
func (r FeeRow) toDomain(rev config.Revision) (domain.LegFees, error) {
	maker, err := fixedpoint.ParseRate(r.Maker)
	if err != nil {
		return domain.LegFees{}, fmt.Errorf("maker: %w: %w", ErrRowInvalid, err)
	}
	taker, err := fixedpoint.ParseRate(r.Taker)
	if err != nil {
		return domain.LegFees{}, fmt.Errorf("taker: %w: %w", ErrRowInvalid, err)
	}
	class, err := parseClass(r.Class)
	if err != nil {
		return domain.LegFees{}, err
	}
	observed, err := parseInstant("observed_at", r.ObservedAt)
	if err != nil {
		return domain.LegFees{}, err
	}
	fees := domain.LegFees{
		Instrument: r.instrument("PROBE"),
		Maker:      maker,
		Taker:      taker,
		Tier:       r.Tier,
		Provenance: domain.Provenance{Class: class, ObservedAt: observed, Rev: rev},
	}
	if err := fees.Validate(); err != nil {
		return domain.LegFees{}, err
	}
	return fees, nil
}

func (r LotRow) toDomain(rev config.Revision) (domain.LotRules, error) {
	step, err := fixedpoint.ParseQuantity(fixedpoint.Asset(r.Base), r.Step)
	if err != nil {
		return domain.LotRules{}, fmt.Errorf("step: %w: %w", ErrRowInvalid, err)
	}
	minQty, err := fixedpoint.ParseQuantity(fixedpoint.Asset(r.Base), r.MinQty)
	if err != nil {
		return domain.LotRules{}, fmt.Errorf("min_qty: %w: %w", ErrRowInvalid, err)
	}
	minNotional, err := fixedpoint.ParseMoney(fixedpoint.Asset(r.Quote), r.MinNotional)
	if err != nil {
		return domain.LotRules{}, fmt.Errorf("min_notional: %w: %w", ErrRowInvalid, err)
	}
	class, err := parseClass(r.Class)
	if err != nil {
		return domain.LotRules{}, err
	}
	observed, err := parseInstant("observed_at", r.ObservedAt)
	if err != nil {
		return domain.LotRules{}, err
	}
	lots := domain.LotRules{
		Instrument: ids.VenueInstrument{
			Venue:    ids.Venue(r.Venue),
			Product:  ids.ProductCategory(r.Product),
			Symbol:   r.Symbol,
			Contract: ids.ContractType(r.Contract),
		},
		Step:        step,
		MinQty:      minQty,
		MinNotional: minNotional,
		Provenance:  domain.Provenance{Class: class, ObservedAt: observed, Rev: rev},
	}
	if err := lots.Validate(); err != nil {
		return domain.LotRules{}, err
	}
	return lots, nil
}

func (h Hurdle) toDomain(rev config.Revision) (domain.HurdleBenchmark, error) {
	parse := func(field, s string) (fixedpoint.Rate, error) {
		r, err := fixedpoint.ParseRate(s)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("%s: %w: %w", field, ErrRowInvalid, err)
		}
		return r, nil
	}
	annual, err := parse("annual_rate", h.AnnualRate)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	access, err := parse("access_cost", h.AccessCost)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	liq, err := parse("liquidity_haircut", h.LiquidityHaircut)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	cpty, err := parse("counterparty_haircut", h.CounterpartyHaircut)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	asOf, err := parseInstant("as_of", h.AsOf)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	class, err := parseClass(h.Class)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	observed, err := parseInstant("observed_at", h.ObservedAt)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	b := domain.HurdleBenchmark{
		AnnualRate:          annual,
		AccessCost:          access,
		LiquidityHaircut:    liq,
		CounterpartyHaircut: cpty,
		AsOf:                asOf,
		Provenance:          domain.Provenance{Class: class, ObservedAt: observed, Rev: rev},
	}
	if err := b.Validate(); err != nil {
		return domain.HurdleBenchmark{}, err
	}
	return b, nil
}

// ReferenceStore answers ports.ReferenceData from a config.Store[Document].
//
// Milestone 2's single-revision world: the store holds the current document,
// and as-of means "the revision observed at or before this instant". With
// one revision that is a refusal for instants before its observation
// (ErrNotPeriodCorrect) and the current answer otherwise; when §40k-2's
// append-only repository lands (step 5, with the runner), the same port
// shape answers from the archive and nothing here has to change its callers.
type ReferenceStore struct {
	store *config.Store[Document]
}

// The §3.6 rule 2 assertion. Breaking conformance is a NOBUILD, which for
// this line means CAUGHT.
var _ ports.ReferenceData = (*ReferenceStore)(nil)

// NewReferenceStore wraps a loaded document store.
func NewReferenceStore(store *config.Store[Document]) (*ReferenceStore, error) {
	if store == nil {
		return nil, fmt.Errorf("store: %w", ErrRowInvalid)
	}
	return &ReferenceStore{store: store}, nil
}

// Fees answers the fee face: the (venue, product) cell, as-of.
func (r *ReferenceStore) Fees(inst ids.VenueInstrument, asOf time.Time) (domain.LegFees, error) {
	if err := inst.Validate(); err != nil {
		return domain.LegFees{}, err
	}
	if asOf.IsZero() {
		return domain.LegFees{}, fmt.Errorf("fees for %s: %w", inst, ErrAsOfUnset)
	}
	snap := r.store.Current()
	for _, row := range snap.Value.Fees {
		if row.instrument(inst.Symbol) != inst {
			continue
		}
		fees, err := row.toDomain(snap.Rev)
		if err != nil {
			return domain.LegFees{}, err
		}
		fees.Instrument = inst
		if asOf.Before(fees.Provenance.ObservedAt) {
			return domain.LegFees{}, fmt.Errorf("fees for %s as of %s, revision observed %s: %w",
				inst, asOf, fees.Provenance.ObservedAt, ErrNotPeriodCorrect)
		}
		return fees, nil
	}
	return domain.LegFees{}, fmt.Errorf("fees for %s: %w", inst, ErrCellAbsent)
}

// LotRules answers the lot face: the instrument's row, as-of.
func (r *ReferenceStore) LotRules(inst ids.VenueInstrument, asOf time.Time) (domain.LotRules, error) {
	if err := inst.Validate(); err != nil {
		return domain.LotRules{}, err
	}
	if asOf.IsZero() {
		return domain.LotRules{}, fmt.Errorf("lots for %s: %w", inst, ErrAsOfUnset)
	}
	snap := r.store.Current()
	for _, row := range snap.Value.Lots {
		lots, err := row.toDomain(snap.Rev)
		if err != nil {
			return domain.LotRules{}, err
		}
		if lots.Instrument != inst {
			continue
		}
		if asOf.Before(lots.Provenance.ObservedAt) {
			return domain.LotRules{}, fmt.Errorf("lots for %s as of %s, revision observed %s: %w",
				inst, asOf, lots.Provenance.ObservedAt, ErrNotPeriodCorrect)
		}
		return lots, nil
	}
	return domain.LotRules{}, fmt.Errorf("lots for %s: %w", inst, ErrCellAbsent)
}

// Hurdle answers the benchmark face, as-of.
func (r *ReferenceStore) Hurdle(asOf time.Time) (domain.HurdleBenchmark, error) {
	if asOf.IsZero() {
		return domain.HurdleBenchmark{}, fmt.Errorf("hurdle: %w", ErrAsOfUnset)
	}
	snap := r.store.Current()
	b, err := snap.Value.Hurdle.toDomain(snap.Rev)
	if err != nil {
		return domain.HurdleBenchmark{}, err
	}
	if asOf.Before(b.Provenance.ObservedAt) {
		return domain.HurdleBenchmark{}, fmt.Errorf("hurdle as of %s, revision observed %s: %w",
			asOf, b.Provenance.ObservedAt, ErrNotPeriodCorrect)
	}
	return b, nil
}
