package driven

// This file is the step-4 calibration artifact's schema and loader (§41d,
// §41g): the one committed JSON document whose exact bytes are the identity
// every class-2 value cites, and the door that turns those bytes into the
// typed values step 3 defined — refusing everything the schema promises.
//
// The schema is context-owned, beside its loader — the reference-document
// pattern one file over. Numbers ride mantissa-as-string at declared scale
// (fixedpoint has no JSON face, §40f). The approval record is deliberately
// NOT in this file's schema: the attestation (attestation.go) names this
// document's content hash, so an in-file approval block would sit inside
// its own preimage (§41d).
//
// Validation is by CONVERSION: every block parses into its domain value and
// the domain validators are the one authority — a parallel rule set here
// would drift. The one rule that IS the schema's own: each quantity's
// provenance family is pinned to §41b's roster assignment, so a measured
// quantity arriving "declared" — or a declared one wearing a derivation
// identity — refuses by name. That is the roster as machinery rather than
// prose.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// Sentinels.
var (
	// ErrArtifactInvalid reports a document member that does not parse or
	// is incomplete; the wrapping message names the member.
	ErrArtifactInvalid = fmt.Errorf("strategy/driven: invalid calibration artifact member")

	// ErrArtifactFixtureID reports a fixture:-prefixed identity arriving
	// through the production loader. The marker belongs to test helpers
	// (§40f); a committed artifact wearing it is an impersonation.
	ErrArtifactFixtureID = fmt.Errorf("strategy/driven: fixture identity in the production loader")

	// ErrArtifactHashMismatch reports loaded bytes whose recomputed digest
	// disagrees with the caller's pinned expectation.
	ErrArtifactHashMismatch = fmt.Errorf("strategy/driven: artifact hash disagrees with the pinned expectation")

	// ErrArtifactHashFormat reports a hash that is not 64 lowercase hex
	// characters. §41d fixes the encoding; normalizing a malformed hash
	// would paper over a hand-edited identity.
	ErrArtifactHashFormat = fmt.Errorf("strategy/driven: hash is not lowercase hex sha-256")

	// ErrFamilyWrong reports a provenance family outside the declared set,
	// or a quantity citing a family §41b assigns to a different one — a
	// declared number impersonating a measurement, or the reverse.
	ErrFamilyWrong = fmt.Errorf("strategy/driven: provenance family disagrees with the §41b roster")
)

// Provenance families (§41b). Merged is the recorded exception: the
// dispersion alone, measured in two parts from two families and frozen as
// one distribution by a committed merge rule.
const (
	FamilyRESTHistory   = "rest_history"
	FamilyCapturedBooks = "captured_books"
	FamilyDeclared      = "declared"
	FamilyMerged        = "merged"
)

// ProvenanceDoc is one quantity's provenance: the family, the dataset
// digests it read, the derivation procedure identity, and the rationale.
type ProvenanceDoc struct {
	Family string `json:"family"`

	// Datasets cites the inputs by digest — dataset-file SHA-256s for
	// family 1, manifest-entry SHA-256s for family 2. A declared quantity
	// MAY cite datasets that informed it (the latency bound cites the
	// transport statistics).
	Datasets []string `json:"datasets"`

	// Derivation identifies the committed procedure that produced the
	// number. Required for a measured family; FORBIDDEN for declared — a
	// declared number wearing a derivation identity reads as measured.
	Derivation string `json:"derivation"`

	// Rationale is required everywhere; family 3's states why the quantity
	// is declared rather than measured.
	Rationale string `json:"rationale"`
}

// validateFor checks the provenance against the family §41b pins the
// quantity to.
func (p ProvenanceDoc) validateFor(member, want string) error {
	switch p.Family {
	case FamilyRESTHistory, FamilyCapturedBooks, FamilyDeclared, FamilyMerged:
	default:
		return fmt.Errorf("%s provenance family %q: %w", member, p.Family, ErrFamilyWrong)
	}
	if p.Family != want {
		return fmt.Errorf("%s provenance family %q, roster says %q: %w", member, p.Family, want, ErrFamilyWrong)
	}
	if p.Rationale == "" {
		return fmt.Errorf("%s rationale: %w", member, ErrArtifactInvalid)
	}
	if p.Family == FamilyDeclared {
		if p.Derivation != "" {
			return fmt.Errorf("%s declared with derivation %q: %w", member, p.Derivation, ErrFamilyWrong)
		}
		return nil
	}
	if p.Derivation == "" {
		return fmt.Errorf("%s derivation: %w", member, ErrArtifactInvalid)
	}
	if len(p.Datasets) == 0 {
		return fmt.Errorf("%s datasets: %w", member, ErrArtifactInvalid)
	}
	for n, d := range p.Datasets {
		if err := checkHashText(d); err != nil {
			return fmt.Errorf("%s dataset %d: %w", member, n, err)
		}
	}
	return nil
}

// RateQuantityDoc is a rate-valued frozen quantity.
type RateQuantityDoc struct {
	Value      string        `json:"value"`
	Provenance ProvenanceDoc `json:"provenance"`
}

// MoneyQuantityDoc is a dimensional frozen quantity.
type MoneyQuantityDoc struct {
	Asset      string        `json:"asset"`
	Amount     string        `json:"amount"`
	Provenance ProvenanceDoc `json:"provenance"`
}

// QuantilePointDoc is one dispersion point.
type QuantilePointDoc struct {
	Q     string `json:"q"`
	Shift string `json:"shift"`
}

// DispersionDoc is the merged adverse-drift distribution (§41b's recorded
// two-family exception).
type DispersionDoc struct {
	Points     []QuantilePointDoc `json:"points"`
	Provenance ProvenanceDoc      `json:"provenance"`
}

// BoundDoc is one interval end. Value is absent for an unbounded end.
type BoundDoc struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// IntervalDoc is one feature interval.
type IntervalDoc struct {
	Lo BoundDoc `json:"lo"`
	Hi BoundDoc `json:"hi"`
}

// StratumRuleDoc is one frozen classifier rule.
type StratumRuleDoc struct {
	Name   string      `json:"name"`
	Spread IntervalDoc `json:"spread"`
	Depth  IntervalDoc `json:"depth"`
}

// ClassifierDoc carries the frozen classifier parameters (§41b-ii-b).
// DepthAsset names the base asset the depth cut points are measured in.
type ClassifierDoc struct {
	DepthAsset string           `json:"depth_asset"`
	Rules      []StratumRuleDoc `json:"rules"`
}

// CostCellDoc is one p90 cell (§41b-ii): the key names the population.
type CostCellDoc struct {
	Venue    string `json:"venue"`
	Product  string `json:"product"`
	Symbol   string `json:"symbol"`
	Contract string `json:"contract"`
	Side     string `json:"side"`
	Stratum  string `json:"stratum"`

	// Base names the asset MaxQty is measured in.
	Base   string `json:"base"`
	MaxQty string `json:"max_qty"`
	P90    string `json:"p90"`

	Observations uint64 `json:"observations"`
	Method       string `json:"method"`
}

// CostDoc is the execution-cost block. The two provenances are two
// families: the fill measurements are family 2 (our books); the latency
// bound is family 3 (declared — no order path exists to measure it, §41b).
// §41b-note: adverse_fill_move is the execution-scale move; the hold-scale
// adverse_basis_move lives at the top level. They share an adjective and
// nothing else.
type CostDoc struct {
	AdverseFillMove string `json:"adverse_fill_move"`

	// LatencyAssumedBoundMs is the ASSUMED decision-to-arrival upper
	// bound, milliseconds. Named an assumption, never "measured at" — the
	// owed §8 edit is marked on CostCalibration.Latency, deliberately once.
	LatencyAssumedBoundMs uint64 `json:"latency_assumed_bound_ms"`

	MinEvidence uint64        `json:"min_evidence"`
	Classifier  ClassifierDoc `json:"classifier"`
	Cells       []CostCellDoc `json:"cells"`

	FillProvenance    ProvenanceDoc `json:"fill_provenance"`
	LatencyProvenance ProvenanceDoc `json:"latency_provenance"`
}

// ScoringDoc is the scoring-protocol block: the declared numbers step 5's
// runner consumes (§41b family 3). Freezing them here keeps every
// verdict-affecting number inside the one-artifact governance — the runner
// synthesizes nothing.
type ScoringDoc struct {
	// RollingWindowDays is the rolling scoring window's length.
	RollingWindowDays uint32 `json:"rolling_window_days"`

	// RegimeRule is the written rule that classifies a sub-window as
	// negative; step 5 applies it mechanically.
	RegimeRule string `json:"regime_rule"`

	// StandbyBenchmark is §8's idle/deployable-capital benchmark the
	// negative-regime criterion compares against.
	StandbyBenchmark RateQuantityDoc `json:"standby_benchmark"`

	// MaxDrawdownCeiling is the drawdown ceiling as a fraction of
	// allocated capital.
	MaxDrawdownCeiling RateQuantityDoc `json:"max_drawdown_ceiling"`

	// ConfidenceLevel and ConfidenceMethod pin the LCB.
	ConfidenceLevel  string `json:"confidence_level"`
	ConfidenceMethod string `json:"confidence_method"`

	// MinUtilization is the eligibility cutoff — a DISTINCT number from
	// the top-level expected utilization (§41b), related to it by the
	// written relation below, never by implication.
	MinUtilization         RateQuantityDoc `json:"min_utilization"`
	MinUtilizationRelation string          `json:"min_utilization_relation"`

	// Provenance covers the block's non-quantity members (window, regime
	// rule, confidence); the rate quantities carry their own.
	Provenance ProvenanceDoc `json:"provenance"`
}

// ArtifactQuantities is §41b's roster.
//
// AdverseBasisMove is a RATE — the relative hold-scale move, Δbasis over
// the spot price — for two reasons that arrived together in slice B: the
// runtime consumer (NewConservativeBasis) charges rate × notional, so a
// Money here had no consumer without an unstated price to divide by; and
// only a relative move transfers across a calibration era in which the
// underlying's price tripled — an absolute 2023 USDT move is not a 2026
// risk statement.
type ArtifactQuantities struct {
	Dispersion          DispersionDoc    `json:"dispersion"`
	EntryDrift          RateQuantityDoc  `json:"entry_drift"`
	ExitBand            RateQuantityDoc  `json:"exit_band"`
	AdverseBasisMove    RateQuantityDoc  `json:"adverse_basis_move"`
	RescueMargin        MoneyQuantityDoc `json:"rescue_margin"`
	ExpectedUtilization RateQuantityDoc  `json:"expected_utilization"`
	Cost                CostDoc          `json:"cost"`
	Scoring             ScoringDoc       `json:"scoring"`
}

// HistoryDatasetDoc is one family-1 dataset: a CLOSED identity (§41c H3) —
// venue, symbol, series, start AND end, the completeness rule, and the
// committed file with its digest.
type HistoryDatasetDoc struct {
	Venue        string `json:"venue"`
	Symbol       string `json:"symbol"`
	Series       string `json:"series"`
	Start        string `json:"start"`
	End          string `json:"end"`
	File         string `json:"file"`
	SHA256       string `json:"sha256"`
	Completeness string `json:"completeness"`
}

// ManifestRefDoc is one family-2 identifier: an archive day's manifest
// entry digest (§41c H2). Step 5's runner refuses to score any window
// whose day appears here.
type ManifestRefDoc struct {
	Day    string `json:"day"`
	SHA256 string `json:"sha256"`
}

// DatasetBlockDoc is §41c's enumeration: everything derivation read.
type DatasetBlockDoc struct {
	// H1Boundary echoes the frozen split instant; the calibrate tool
	// refuses any historical input on or after it.
	H1Boundary string              `json:"h1_boundary"`
	History    []HistoryDatasetDoc `json:"history"`
	Manifests  []ManifestRefDoc    `json:"manifests"`
}

// ProcedureIdentityDoc is §41e's execution binding: the commit the
// derivation ran from and the digest of the executable that ran.
type ProcedureIdentityDoc struct {
	Commit           string `json:"commit"`
	ExecutableSHA256 string `json:"executable_sha256"`
}

// PromotionIdentityDoc is the frozen promotion identity (§41c H3): closed
// on every axis, inside the hashed artifact, so no verdict-affecting
// choice survives approval.
type PromotionIdentityDoc struct {
	Venue string `json:"venue"`

	// Fork is the §41h step-5 data fork: wait, buy or hybrid.
	Fork string `json:"fork"`

	Instruments []CostCellInstrumentDoc `json:"instruments"`

	SampleStart string `json:"sample_start"`

	// EndRule is the deterministic sample-end rule — a closed rule, never
	// an operator-declared run date, which would permit outcome-dependent
	// stopping.
	EndRule string `json:"end_rule"`

	MissingDataPolicy string `json:"missing_data_policy"`
}

// CostCellInstrumentDoc names one instrument.
type CostCellInstrumentDoc struct {
	Venue    string `json:"venue"`
	Product  string `json:"product"`
	Symbol   string `json:"symbol"`
	Contract string `json:"contract"`
}

// Forks: the declared step-5 data forks (§41h).
const (
	ForkWait   = "wait"
	ForkBuy    = "buy"
	ForkHybrid = "hybrid"
)

// ArtifactDoc is the whole committed document.
type ArtifactDoc struct {
	SchemaVersion int                  `json:"schema_version"`
	Quantities    ArtifactQuantities   `json:"quantities"`
	Datasets      DatasetBlockDoc      `json:"datasets"`
	Procedure     ProcedureIdentityDoc `json:"procedure"`
	Promotion     PromotionIdentityDoc `json:"promotion"`
}

// Validate makes the document loadable: every block converts into its
// domain value against a probe identity, and every provenance family
// matches §41b's pinning.
func (d ArtifactDoc) Validate() error {
	probe := domain.CalibrationRef{ID: "validate", Hash: "validate"}
	_, err := d.toArtifact(probe)
	return err
}

// Artifact is the loaded, typed value: the class-2 values step 3 defined,
// each carrying the artifact's identity with the hash recomputed from the
// file's bytes at load (§41g).
type Artifact struct {
	Ref domain.CalibrationRef

	Dispersion          domain.Dispersion
	EntryDrift          domain.MeasuredRate
	ExitBand            domain.MeasuredRate
	ExpectedUtilization domain.MeasuredRate
	AdverseBasisMove    domain.MeasuredRate
	RescueMargin        domain.MeasuredMoney
	Cost                CostCalibration
	Scoring             ScoringProtocol
	Promotion           PromotionIdentity

	// Doc keeps the decoded document for the consumers that read the
	// enumerations rather than the values: the H2 runner refusal reads
	// Datasets.Manifests, the binding check reads Procedure.
	Doc ArtifactDoc
}

// ScoringProtocol is the typed scoring block.
type ScoringProtocol struct {
	RollingWindow          time.Duration
	RegimeRule             string
	StandbyBenchmark       domain.MeasuredRate
	MaxDrawdownCeiling     domain.MeasuredRate
	ConfidenceLevel        fixedpoint.Rate
	ConfidenceMethod       string
	MinUtilization         domain.MeasuredRate
	MinUtilizationRelation string
}

// PromotionIdentity is the typed promotion identity.
type PromotionIdentity struct {
	Venue             ids.Venue
	Fork              string
	Instruments       []ids.VenueInstrument
	SampleStart       time.Time
	EndRule           string
	MissingDataPolicy string
}

// LoadArtifact turns the document's exact bytes into the typed value. The
// id is the artifact's identity (the file's basename without extension,
// per §41d); the hash is recomputed from data here, unconditionally —
// nothing upstream can hand the loader a friendlier digest.
func LoadArtifact(id string, data []byte) (*Artifact, error) {
	if id == "" {
		return nil, fmt.Errorf("artifact id: %w", domain.ErrUnset)
	}
	if strings.HasPrefix(id, domain.FixturePrefix) {
		return nil, fmt.Errorf("artifact id %q: %w", id, ErrArtifactFixtureID)
	}
	doc, err := config.Decode[ArtifactDoc](data)
	if err != nil {
		return nil, fmt.Errorf("artifact %s: %w", id, err)
	}
	sum := sha256.Sum256(data)
	ref := domain.CalibrationRef{ID: id, Hash: hex.EncodeToString(sum[:])}
	a, err := doc.toArtifact(ref)
	if err != nil {
		return nil, fmt.Errorf("artifact %s: %w", id, err)
	}
	return a, nil
}

// LoadArtifactFile loads a committed artifact from disk, deriving the
// identity from the file's basename (§41d).
func LoadArtifactFile(path string) (*Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("artifact file: %w", err)
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return LoadArtifact(id, data)
}

// VerifyPinned refuses an artifact whose recomputed hash disagrees with
// the caller's pinned expectation (§41g). The expectation must already be
// lowercase hex — a malformed pin is its own refusal, never normalized.
func (a *Artifact) VerifyPinned(expected string) error {
	if err := checkHashText(expected); err != nil {
		return fmt.Errorf("pinned expectation: %w", err)
	}
	if a.Ref.Hash != expected {
		return fmt.Errorf("artifact %s hash %s, pinned %s: %w",
			a.Ref.ID, a.Ref.Hash, expected, ErrArtifactHashMismatch)
	}
	return nil
}

// CheckHashText demands 64 lowercase hex characters — §41d's fixed
// encoding for every digest the artifact and attestation carry. Exported
// because the calibrate driving adapter checks recorded digests against
// the same rule: one implementation, both consumers.
func CheckHashText(s string) error { return checkHashText(s) }

// checkHashText is CheckHashText's body; internal call sites use it
// directly.
func checkHashText(s string) error {
	if len(s) != 64 {
		return fmt.Errorf("digest %q length %d: %w", s, len(s), ErrArtifactHashFormat)
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("digest %q: %w", s, ErrArtifactHashFormat)
		}
	}
	return nil
}

// toArtifact converts the document under ref's identity — validation by
// conversion, the domain validators being the one authority.
func (d ArtifactDoc) toArtifact(ref domain.CalibrationRef) (*Artifact, error) {
	if d.SchemaVersion != 1 {
		return nil, fmt.Errorf("schema_version %d: %w", d.SchemaVersion, ErrArtifactInvalid)
	}

	disp, err := d.Quantities.Dispersion.toDomain(ref)
	if err != nil {
		return nil, fmt.Errorf("dispersion: %w", err)
	}
	entryDrift, err := d.Quantities.EntryDrift.toRate("entry_drift", FamilyRESTHistory, ref)
	if err != nil {
		return nil, err
	}
	exitBand, err := d.Quantities.ExitBand.toRate("exit_band", FamilyRESTHistory, ref)
	if err != nil {
		return nil, err
	}
	util, err := d.Quantities.ExpectedUtilization.toRate("expected_utilization", FamilyRESTHistory, ref)
	if err != nil {
		return nil, err
	}
	if err := checkUnitInterval("expected_utilization", util.Value); err != nil {
		return nil, err
	}
	basis, err := d.Quantities.AdverseBasisMove.toRate("adverse_basis_move", FamilyRESTHistory, ref)
	if err != nil {
		return nil, err
	}
	rescue, err := d.Quantities.RescueMargin.toMoney("rescue_margin", FamilyDeclared, ref)
	if err != nil {
		return nil, err
	}
	cost, err := d.Quantities.Cost.toDomain(ref)
	if err != nil {
		return nil, fmt.Errorf("cost: %w", err)
	}
	scoring, err := d.Quantities.Scoring.toDomain(ref)
	if err != nil {
		return nil, fmt.Errorf("scoring: %w", err)
	}
	promo, err := d.Promotion.toDomain()
	if err != nil {
		return nil, fmt.Errorf("promotion: %w", err)
	}
	if err := d.Datasets.validate(); err != nil {
		return nil, fmt.Errorf("datasets: %w", err)
	}
	if err := d.Procedure.validate(); err != nil {
		return nil, fmt.Errorf("procedure: %w", err)
	}

	return &Artifact{
		Ref:                 ref,
		Dispersion:          disp,
		EntryDrift:          entryDrift,
		ExitBand:            exitBand,
		ExpectedUtilization: util,
		AdverseBasisMove:    basis,
		RescueMargin:        rescue,
		Cost:                cost,
		Scoring:             scoring,
		Promotion:           promo,
		Doc:                 d,
	}, nil
}

func (q RateQuantityDoc) toRate(member, family string, ref domain.CalibrationRef) (domain.MeasuredRate, error) {
	if err := q.Provenance.validateFor(member, family); err != nil {
		return domain.MeasuredRate{}, err
	}
	v, err := fixedpoint.ParseRate(q.Value)
	if err != nil {
		return domain.MeasuredRate{}, fmt.Errorf("%s value: %w: %w", member, ErrArtifactInvalid, err)
	}
	m := domain.MeasuredRate{Value: v, Artifact: ref}
	if err := m.Validate(); err != nil {
		return domain.MeasuredRate{}, fmt.Errorf("%s: %w", member, err)
	}
	return m, nil
}

func (q MoneyQuantityDoc) toMoney(member, family string, ref domain.CalibrationRef) (domain.MeasuredMoney, error) {
	if err := q.Provenance.validateFor(member, family); err != nil {
		return domain.MeasuredMoney{}, err
	}
	if q.Asset == "" {
		return domain.MeasuredMoney{}, fmt.Errorf("%s asset: %w", member, ErrArtifactInvalid)
	}
	v, err := fixedpoint.ParseMoney(fixedpoint.Asset(q.Asset), q.Amount)
	if err != nil {
		return domain.MeasuredMoney{}, fmt.Errorf("%s amount: %w: %w", member, ErrArtifactInvalid, err)
	}
	m := domain.MeasuredMoney{Amount: v, Artifact: ref}
	if err := m.Validate(); err != nil {
		return domain.MeasuredMoney{}, fmt.Errorf("%s: %w", member, err)
	}
	return m, nil
}

func (d DispersionDoc) toDomain(ref domain.CalibrationRef) (domain.Dispersion, error) {
	if err := d.Provenance.validateFor("dispersion", FamilyMerged); err != nil {
		return domain.Dispersion{}, err
	}
	pts := make([]domain.QuantilePoint, 0, len(d.Points))
	for n, p := range d.Points {
		q, err := fixedpoint.ParseRate(p.Q)
		if err != nil {
			return domain.Dispersion{}, fmt.Errorf("point %d q: %w: %w", n, ErrArtifactInvalid, err)
		}
		s, err := fixedpoint.ParseRate(p.Shift)
		if err != nil {
			return domain.Dispersion{}, fmt.Errorf("point %d shift: %w: %w", n, ErrArtifactInvalid, err)
		}
		pts = append(pts, domain.QuantilePoint{Q: q, Shift: s})
	}
	disp := domain.Dispersion{Points: pts, Artifact: ref}
	if err := disp.Validate(); err != nil {
		return domain.Dispersion{}, err
	}
	return disp, nil
}

func (b BoundDoc) toRateBound() (domain.RateBound, error) {
	kind := domain.BoundKind(b.Kind)
	if kind == domain.BoundUnbounded {
		if b.Value != "" {
			return domain.RateBound{}, fmt.Errorf("unbounded end carries value %q: %w", b.Value, ErrArtifactInvalid)
		}
		return domain.RateBound{Kind: kind}, nil
	}
	v, err := fixedpoint.ParseRate(b.Value)
	if err != nil {
		return domain.RateBound{}, fmt.Errorf("bound value: %w: %w", ErrArtifactInvalid, err)
	}
	return domain.RateBound{Kind: kind, Value: v}, nil
}

func (b BoundDoc) toQuantityBound(asset fixedpoint.Asset) (domain.QuantityBound, error) {
	kind := domain.BoundKind(b.Kind)
	if kind == domain.BoundUnbounded {
		if b.Value != "" {
			return domain.QuantityBound{}, fmt.Errorf("unbounded end carries value %q: %w", b.Value, ErrArtifactInvalid)
		}
		return domain.QuantityBound{Kind: kind}, nil
	}
	v, err := fixedpoint.ParseQuantity(asset, b.Value)
	if err != nil {
		return domain.QuantityBound{}, fmt.Errorf("bound value: %w: %w", ErrArtifactInvalid, err)
	}
	return domain.QuantityBound{Kind: kind, Value: v}, nil
}

func (c ClassifierDoc) toDomain() (domain.StratumClassifier, error) {
	if c.DepthAsset == "" {
		return domain.StratumClassifier{}, fmt.Errorf("classifier depth_asset: %w", ErrArtifactInvalid)
	}
	asset := fixedpoint.Asset(c.DepthAsset)
	rules := make([]domain.StratumRule, 0, len(c.Rules))
	for n, r := range c.Rules {
		spreadLo, err := r.Spread.Lo.toRateBound()
		if err != nil {
			return domain.StratumClassifier{}, fmt.Errorf("rule %d spread lo: %w", n, err)
		}
		spreadHi, err := r.Spread.Hi.toRateBound()
		if err != nil {
			return domain.StratumClassifier{}, fmt.Errorf("rule %d spread hi: %w", n, err)
		}
		depthLo, err := r.Depth.Lo.toQuantityBound(asset)
		if err != nil {
			return domain.StratumClassifier{}, fmt.Errorf("rule %d depth lo: %w", n, err)
		}
		depthHi, err := r.Depth.Hi.toQuantityBound(asset)
		if err != nil {
			return domain.StratumClassifier{}, fmt.Errorf("rule %d depth hi: %w", n, err)
		}
		rules = append(rules, domain.StratumRule{
			Name:   r.Name,
			Spread: domain.RateInterval{Lo: spreadLo, Hi: spreadHi},
			Depth:  domain.QuantityInterval{Lo: depthLo, Hi: depthHi},
		})
	}
	cl := domain.StratumClassifier{Rules: rules}
	if err := cl.Validate(); err != nil {
		return domain.StratumClassifier{}, err
	}
	return cl, nil
}

func (c CostCellDoc) toDomain() (CostCell, error) {
	inst := ids.VenueInstrument{
		Venue:    ids.Venue(c.Venue),
		Product:  ids.ProductCategory(c.Product),
		Symbol:   c.Symbol,
		Contract: ids.ContractType(c.Contract),
	}
	side := mdomain.BookSide(c.Side)
	if err := side.Validate(); err != nil {
		return CostCell{}, fmt.Errorf("side: %w: %w", ErrArtifactInvalid, err)
	}
	if c.Base == "" {
		return CostCell{}, fmt.Errorf("base: %w", ErrArtifactInvalid)
	}
	maxQty, err := fixedpoint.ParseQuantity(fixedpoint.Asset(c.Base), c.MaxQty)
	if err != nil {
		return CostCell{}, fmt.Errorf("max_qty: %w: %w", ErrArtifactInvalid, err)
	}
	p90, err := fixedpoint.ParseRate(c.P90)
	if err != nil {
		return CostCell{}, fmt.Errorf("p90: %w: %w", ErrArtifactInvalid, err)
	}
	return CostCell{
		Instrument:   inst,
		Side:         side,
		Stratum:      c.Stratum,
		MaxQty:       maxQty,
		P90:          p90,
		Observations: c.Observations,
		Method:       c.Method,
	}, nil
}

func (c CostDoc) toDomain(ref domain.CalibrationRef) (CostCalibration, error) {
	if err := c.FillProvenance.validateFor("fill", FamilyCapturedBooks); err != nil {
		return CostCalibration{}, err
	}
	if err := c.LatencyProvenance.validateFor("latency", FamilyDeclared); err != nil {
		return CostCalibration{}, err
	}
	adverse, err := fixedpoint.ParseRate(c.AdverseFillMove)
	if err != nil {
		return CostCalibration{}, fmt.Errorf("adverse_fill_move: %w: %w", ErrArtifactInvalid, err)
	}
	classifier, err := c.Classifier.toDomain()
	if err != nil {
		return CostCalibration{}, fmt.Errorf("classifier: %w", err)
	}
	cells := make([]CostCell, 0, len(c.Cells))
	for n, cd := range c.Cells {
		cell, err := cd.toDomain()
		if err != nil {
			return CostCalibration{}, fmt.Errorf("cell %d: %w", n, err)
		}
		cells = append(cells, cell)
	}
	cal := CostCalibration{
		Artifact:    ref,
		AdverseMove: adverse,
		Latency:     time.Duration(c.LatencyAssumedBoundMs) * time.Millisecond,
		Classifier:  classifier,
		MinEvidence: c.MinEvidence,
		Cells:       cells,
	}
	if err := cal.Validate(); err != nil {
		return CostCalibration{}, err
	}
	return cal, nil
}

func (s ScoringDoc) toDomain(ref domain.CalibrationRef) (ScoringProtocol, error) {
	if err := s.Provenance.validateFor("scoring", FamilyDeclared); err != nil {
		return ScoringProtocol{}, err
	}
	if s.RollingWindowDays == 0 {
		return ScoringProtocol{}, fmt.Errorf("rolling_window_days: %w", domain.ErrUnset)
	}
	if s.RegimeRule == "" {
		return ScoringProtocol{}, fmt.Errorf("regime_rule: %w", domain.ErrUnset)
	}
	standby, err := s.StandbyBenchmark.toRate("standby_benchmark", FamilyDeclared, ref)
	if err != nil {
		return ScoringProtocol{}, err
	}
	ceiling, err := s.MaxDrawdownCeiling.toRate("max_drawdown_ceiling", FamilyDeclared, ref)
	if err != nil {
		return ScoringProtocol{}, err
	}
	level, err := fixedpoint.ParseRate(s.ConfidenceLevel)
	if err != nil {
		return ScoringProtocol{}, fmt.Errorf("confidence_level: %w: %w", ErrArtifactInvalid, err)
	}
	if err := checkUnitInterval("confidence_level", level); err != nil {
		return ScoringProtocol{}, err
	}
	if s.ConfidenceMethod == "" {
		return ScoringProtocol{}, fmt.Errorf("confidence_method: %w", domain.ErrUnset)
	}
	minUtil, err := s.MinUtilization.toRate("min_utilization", FamilyDeclared, ref)
	if err != nil {
		return ScoringProtocol{}, err
	}
	if err := checkUnitInterval("min_utilization", minUtil.Value); err != nil {
		return ScoringProtocol{}, err
	}
	if s.MinUtilizationRelation == "" {
		return ScoringProtocol{}, fmt.Errorf("min_utilization_relation: %w", domain.ErrUnset)
	}
	return ScoringProtocol{
		RollingWindow:          time.Duration(s.RollingWindowDays) * 24 * time.Hour,
		RegimeRule:             s.RegimeRule,
		StandbyBenchmark:       standby,
		MaxDrawdownCeiling:     ceiling,
		ConfidenceLevel:        level,
		ConfidenceMethod:       s.ConfidenceMethod,
		MinUtilization:         minUtil,
		MinUtilizationRelation: s.MinUtilizationRelation,
	}, nil
}

func (p PromotionIdentityDoc) toDomain() (PromotionIdentity, error) {
	venue := ids.Venue(p.Venue)
	if err := venue.Validate(); err != nil {
		return PromotionIdentity{}, fmt.Errorf("venue: %w: %w", ErrArtifactInvalid, err)
	}
	switch p.Fork {
	case ForkWait, ForkBuy, ForkHybrid:
	default:
		return PromotionIdentity{}, fmt.Errorf("fork %q: %w", p.Fork, ErrArtifactInvalid)
	}
	if len(p.Instruments) == 0 {
		return PromotionIdentity{}, fmt.Errorf("instruments: %w", domain.ErrUnset)
	}
	insts := make([]ids.VenueInstrument, 0, len(p.Instruments))
	for n, i := range p.Instruments {
		inst := ids.VenueInstrument{
			Venue:    ids.Venue(i.Venue),
			Product:  ids.ProductCategory(i.Product),
			Symbol:   i.Symbol,
			Contract: ids.ContractType(i.Contract),
		}
		if err := inst.Validate(); err != nil {
			return PromotionIdentity{}, fmt.Errorf("instrument %d: %w: %w", n, ErrArtifactInvalid, err)
		}
		insts = append(insts, inst)
	}
	start, err := parseInstant("sample_start", p.SampleStart)
	if err != nil {
		return PromotionIdentity{}, err
	}
	if p.EndRule == "" {
		return PromotionIdentity{}, fmt.Errorf("end_rule: %w", domain.ErrUnset)
	}
	if p.MissingDataPolicy == "" {
		return PromotionIdentity{}, fmt.Errorf("missing_data_policy: %w", domain.ErrUnset)
	}
	return PromotionIdentity{
		Venue:             venue,
		Fork:              p.Fork,
		Instruments:       insts,
		SampleStart:       start,
		EndRule:           p.EndRule,
		MissingDataPolicy: p.MissingDataPolicy,
	}, nil
}

func (d DatasetBlockDoc) validate() error {
	if _, err := parseInstant("h1_boundary", d.H1Boundary); err != nil {
		return err
	}
	for n, h := range d.History {
		if h.Venue == "" || h.Symbol == "" || h.Series == "" {
			return fmt.Errorf("history %d identity: %w", n, ErrArtifactInvalid)
		}
		if _, err := parseInstant(fmt.Sprintf("history %d start", n), h.Start); err != nil {
			return err
		}
		if _, err := parseInstant(fmt.Sprintf("history %d end", n), h.End); err != nil {
			return err
		}
		if h.File == "" {
			return fmt.Errorf("history %d file: %w", n, ErrArtifactInvalid)
		}
		if err := checkHashText(h.SHA256); err != nil {
			return fmt.Errorf("history %d: %w", n, err)
		}
		if h.Completeness == "" {
			return fmt.Errorf("history %d completeness: %w", n, ErrArtifactInvalid)
		}
	}
	for n, m := range d.Manifests {
		if m.Day == "" {
			return fmt.Errorf("manifest %d day: %w", n, ErrArtifactInvalid)
		}
		if err := checkHashText(m.SHA256); err != nil {
			return fmt.Errorf("manifest %d: %w", n, err)
		}
	}
	return nil
}

func (p ProcedureIdentityDoc) validate() error {
	if p.Commit == "" {
		return fmt.Errorf("commit: %w", domain.ErrUnset)
	}
	if err := checkHashText(p.ExecutableSHA256); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	return nil
}

// checkUnitInterval refuses a rate outside (0, 1] — utilization figures
// and confidence levels are fractions, and a zero one is a forgotten one.
func checkUnitInterval(member string, r fixedpoint.Rate) error {
	sign, err := r.Sign()
	if err != nil {
		return fmt.Errorf("%s: %w", member, err)
	}
	if sign <= 0 {
		return fmt.Errorf("%s %s: %w", member, r, ErrArtifactInvalid)
	}
	one, err := fixedpoint.ParseRate("1")
	if err != nil {
		return fmt.Errorf("%s: %w", member, err)
	}
	c, err := r.Cmp(one)
	if err != nil {
		return fmt.Errorf("%s: %w", member, err)
	}
	if c > 0 {
		return fmt.Errorf("%s %s above 1: %w", member, r, ErrArtifactInvalid)
	}
	return nil
}
