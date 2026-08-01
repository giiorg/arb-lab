package driving

// The artifact assembly: derived values → the §41d document. Every
// rationale here is written AT derivation and embeds this run's own
// counts and statistics — deterministic given the same inputs, which the
// two-invocation claim checks. The §41b family pinnings are the schema's
// own validation; this file's job is to cite honestly: each quantity
// lists exactly the dataset digests its derivation read.

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// assembly carries everything the document needs.
type assembly struct {
	funding, spotKlines, markKlines loadedDataset
	days                            []ManifestDay
	revision, exeSHA                string

	merged                []domain.QuantilePoint
	ewmaResidualCount     int
	entryDrift, bandWidth fixedpoint.Rate
	basis                 fixedpoint.Rate
	utilization, minUtil  fixedpoint.Rate
	cells                 []driven.CostCell
	droppedCells          uint64
	adverseFill           fixedpoint.Rate
	transport             TransportStats
	imminent              ImminentMeasurements
	spotBooks, perpBooks  BookMeasurements
	freeze                time.Time
}

func assembleDoc(a assembly) (driven.ArtifactDoc, error) {
	manifestRefs := make([]driven.ManifestRefDoc, 0, len(a.days))
	manifestDigests := make([]string, 0, len(a.days))
	for _, d := range a.days {
		manifestRefs = append(manifestRefs, driven.ManifestRefDoc{Day: d.Day, SHA256: d.SHA256})
		manifestDigests = append(manifestDigests, d.SHA256)
	}

	points := make([]driven.QuantilePointDoc, 0, len(a.merged))
	for _, p := range a.merged {
		points = append(points, driven.QuantilePointDoc{Q: p.Q.Decimal(), Shift: p.Shift.Decimal()})
	}

	windowNote := fmt.Sprintf(
		"capture window is %d archive day(s) (%s..%s) — SHORT, stated per §41b: a longer-windowed v2 is a new version and a complete rerun",
		len(a.days), a.days[0].Day, a.days[len(a.days)-1].Day)

	dispersionProv := driven.ProvenanceDoc{
		Family:     driven.FamilyMerged,
		Datasets:   append([]string{a.funding.sha256}, manifestDigests...),
		Derivation: "merge-adverse-v1: pointwise more-adverse of causal trailing-EWMA residual quantiles (72h half-life, saturation warmup, unclamped forecast) and live markPrice@1s estimate residuals (60s grid, settlement-flip realized rule), on the committed 7-point grid; outward-from-median order statistics both sides",
		Rationale: fmt.Sprintf(
			"§41b's recorded two-family exception: %d EWMA residuals over the 2023-01-01..2026-01-31 era merged with %d imminent residuals (%d estimates, %d unrealized, dropped and counted); %s",
			a.ewmaResidualCount, len(a.imminent.Residuals), a.imminent.Estimates, a.imminent.Unrealized, windowNote),
	}

	fillProv := driven.ProvenanceDoc{
		Family:     driven.FamilyCapturedBooks,
		Datasets:   manifestDigests,
		Derivation: procMethodIdentity,
		Rationale: fmt.Sprintf(
			"counterfactual p90 per (leg, side, frozen stratum, candidate): spot %d pairs (%d skipped, %d depth-short), perp %d pairs (%d skipped, %d depth-short); %d cell(s) below min_evidence=%d are ABSENT (a refusal at runtime, never a thin number); real-fill recalibration arrives with milestone 3's order path; %s",
			a.spotBooks.Pairs, a.spotBooks.Skipped, a.spotBooks.DepthShort,
			a.perpBooks.Pairs, a.perpBooks.Skipped, a.perpBooks.DepthShort,
			a.droppedCells, ProcMinEvidence, windowNote),
	}

	latencyProv := driven.ProvenanceDoc{
		Family:   driven.FamilyDeclared,
		Datasets: manifestDigests,
		Rationale: fmt.Sprintf(
			"DECLARED upper bound — no order path exists before milestone 3's OMS, so decision-to-arrival latency is structurally unmeasurable this milestone (the §8 marker's subject; a measured order-path latency is required before capital). Informed by the archive's transport statistics (markPrice venue stamp to our clock, %d samples: p50 %dms, p90 %dms, p99 %dms, max %dms, %d negative — venue-clock skew kept, not clamped) with a stated x2.5 multiplier on the healthy-hour p99 (~100ms) for the unmeasured order path. The runtime adapter refuses any longer assumption at decision time.",
			a.transport.Count, a.transport.P50.Milliseconds(), a.transport.P90.Milliseconds(),
			a.transport.P99.Milliseconds(), a.transport.Max.Milliseconds(), a.transport.NegativeObservations),
	}

	classifier, err := classifierDoc()
	if err != nil {
		return driven.ArtifactDoc{}, err
	}
	cellDocs := make([]driven.CostCellDoc, 0, len(a.cells))
	for _, c := range a.cells {
		cellDocs = append(cellDocs, driven.CostCellDoc{
			Venue:        string(c.Instrument.Venue),
			Product:      string(c.Instrument.Product),
			Symbol:       c.Instrument.Symbol,
			Contract:     string(c.Instrument.Contract),
			Side:         string(c.Side),
			Stratum:      c.Stratum,
			Base:         string(calibBase),
			MaxQty:       c.MaxQty.Decimal(),
			P90:          c.P90.Decimal(),
			Observations: c.Observations,
			Method:       c.Method,
		})
	}

	restProv := func(member, derivation, rationale string, digests ...string) driven.ProvenanceDoc {
		return driven.ProvenanceDoc{
			Family:     driven.FamilyRESTHistory,
			Datasets:   digests,
			Derivation: derivation,
			Rationale:  rationale,
		}
	}
	declaredProv := func(rationale string) driven.ProvenanceDoc {
		return driven.ProvenanceDoc{Family: driven.FamilyDeclared, Rationale: rationale}
	}

	sampleStart := a.freeze.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)

	doc := driven.ArtifactDoc{
		SchemaVersion: 1,
		Quantities: driven.ArtifactQuantities{
			Dispersion: driven.DispersionDoc{Points: points, Provenance: dispersionProv},
			EntryDrift: driven.RateQuantityDoc{
				Value: a.entryDrift.Decimal(),
				Provenance: restProv("entry_drift",
					"drift-v1: the family-1 EWMA residual quantile at q=0.05 (the committed adverse tail, a grid member), negative shift annualized via cashcarry.Annualized over the 8h interval; non-negative tail freezes a declared zero",
					"the C3 adverse-drift margin from the calibration era's own residual tail; the family-1 pinning is checked at emit — a family-2-dominated tail refuses rather than understating",
					a.funding.sha256),
			},
			ExitBand: driven.RateQuantityDoc{
				Value: a.bandWidth.Decimal(),
				Provenance: restProv("exit_band",
					"band-v1: q=0.9 upper statistic of recovered-dip depths below the frame's entry-clearing level (dips strictly below, at-level ends a dip, recovered = shorter than the 2-settlement dwell, truncated runs are evidence of nothing), annualized",
					"measured regime persistence per §2.4's own words, against the same clearing definition the occupancy replay enters at (one frame, §41b round 3)",
					a.funding.sha256),
			},
			AdverseBasisMove: driven.RateQuantityDoc{
				Value: a.basis.Decimal(),
				Provenance: restProv("adverse_basis_move",
					"basis-v1: relative rise of (mark - spot)/spot over 24h horizons across every horizon-separated hourly pair, q=0.99 upper statistic; negative tail freezes a declared zero",
					"the HOLD-scale move (§41b-note; the execution-scale adverse_fill_move is family 2's) — a RATE because the runtime charges rate x notional and only a relative move transfers across an era in which the underlying tripled; kline era starts 2023-04-01 because H3 fired on the 2023-03-24 spot-halt hole",
					a.spotKlines.sha256, a.markKlines.sha256),
			},
			RescueMargin: driven.MoneyQuantityDoc{
				Asset:  string(calibQuote),
				Amount: ProcCapitalRescue,
				Provenance: declaredProv(
					"§40k-4's interim figure: $100 on the $1,550 decomposition — sized to one adverse-basis top-up at the frame's leverage without breaching the isolated margin; the REAL derivation is milestone 4's §7.1 protocol, and this number exists so the decomposition validates rather than defaulting to a free rescue"),
			},
			ExpectedUtilization: driven.RateQuantityDoc{
				Value: a.utilization.Decimal(),
				Provenance: restProv("expected_utilization",
					"occupancy-v1: the deployed Enter/Continue replay over the calibration era under the pre-registered constant-cost surrogate (era-first-kline frame prices, candidate 0.02 BTC, calm-stratum frame cost, frozen dispersion/band/basis), same-instant re-entry, era truncation stated; occupied over deployable, RoundFloor",
					"occupancy, not eligibility (§41b round 2): holding historical costs at the frozen bound suppresses entries and understates occupancy — the direction that hardens the gate; the fixed price frame is a stated limitation (§41n reading ii), not absorbed",
					a.funding.sha256),
			},
			Cost: driven.CostDoc{
				AdverseFillMove:       a.adverseFill.Decimal(),
				LatencyAssumedBoundMs: uint64(ProcLatencyAssumedBound / time.Millisecond),
				MinEvidence:           ProcMinEvidence,
				Classifier:            classifier,
				Cells:                 cellDocs,
				FillProvenance:        fillProv,
				LatencyProvenance:     latencyProv,
			},
			Scoring: driven.ScoringDoc{
				RollingWindowDays: ProcRollingWindowDays,
				RegimeRule:        ProcRegimeRule,
				StandbyBenchmark: driven.RateQuantityDoc{
					Value: ProcStandbyBenchmark.Decimal(),
					Provenance: declaredProv(
						"§8's standby is what idle/deployable capital earns: the committed reference's 13-week bill close (3.69%, treasury.gov, as-of 2026-07-30) — NOT the full hurdle; no venue haircuts apply to capital that never goes on-venue"),
				},
				MaxDrawdownCeiling: driven.RateQuantityDoc{
					Value: ProcMaxDrawdownCeiling.Decimal(),
					Provenance: declaredProv(
						"5% of allocated capital: ~6x the frame's measured round-trip cost share, so an ordinary stand-down cannot trip it while a genuinely wrong regime does"),
				},
				ConfidenceLevel:  ProcConfidenceLevel.Decimal(),
				ConfidenceMethod: ProcConfidenceMethod,
				MinUtilization: driven.RateQuantityDoc{
					Value: a.minUtil.Decimal(),
					Provenance: declaredProv(
						"declared BY RELATION: " + ProcMinUtilizationRelation + " — a run idling below half the pre-registered expectation is not the strategy the artifact priced, whatever its LCB says"),
				},
				MinUtilizationRelation: ProcMinUtilizationRelation,
				Provenance: declaredProv(
					"scoring-protocol choices (window, regime rule, confidence method): declared per §41b family 3 — each is a protocol decision milestone 2 cannot measure, frozen here so step 5's runner synthesizes nothing"),
			},
		},
		Datasets: driven.DatasetBlockDoc{
			H1Boundary: H1Boundary.Format(time.RFC3339),
			History: []driven.HistoryDatasetDoc{
				historyDoc(a.funding),
				historyDoc(a.spotKlines),
				historyDoc(a.markKlines),
			},
			Manifests: manifestRefs,
		},
		Procedure: driven.ProcedureIdentityDoc{
			Commit:           a.revision,
			ExecutableSHA256: a.exeSHA,
		},
		Promotion: driven.PromotionIdentityDoc{
			Venue: string(SealVenue),
			Fork:  string(SealFork),
			Instruments: []driven.CostCellInstrumentDoc{
				{Venue: string(calibSpotInst.Venue), Product: string(calibSpotInst.Product), Symbol: calibSpotInst.Symbol, Contract: string(calibSpotInst.Contract)},
				{Venue: string(calibPerpInst.Venue), Product: string(calibPerpInst.Product), Symbol: calibPerpInst.Symbol, Contract: string(calibPerpInst.Contract)},
			},
			SampleStart:       sampleStart.Format(time.RFC3339),
			EndRule:           "sample_start + 90 days — the first complete three-month interval after the freeze, a closed rule; scoring covers EVERYTHING eligible inside it under the sealed identity, never a chosen subset",
			MissingDataPolicy: "hybrid (§41h, sealed 2026-07-31): negative-regime sub-windows score stand-down correctness against venue-published funding history (a standing-down strategy fills nothing); fill-bearing windows require captured books, and a fill-bearing window with absent book coverage is scored as stand-down with the absence recorded — never interpolated",
		},
	}
	return doc, nil
}

// historyDoc renders one committed dataset's closed identity.
func historyDoc(d loadedDataset) driven.HistoryDatasetDoc {
	return driven.HistoryDatasetDoc{
		Venue:        d.file.Venue,
		Symbol:       d.file.Symbol,
		Series:       d.file.Series,
		Start:        d.file.Start,
		End:          d.file.End,
		File:         d.rel,
		SHA256:       d.sha256,
		Completeness: "closed identity (§41c H3): every grid instant from start to end inclusive present exactly once (1-minute inclusive jitter tolerance), none extra, rows H1-guarded; pinned against edits by committed_test.go",
	}
}

// classifierDoc renders the frozen classifier parameters.
func classifierDoc() (driven.ClassifierDoc, error) {
	cl := ProcClassifier()
	if err := cl.Validate(); err != nil {
		return driven.ClassifierDoc{}, err
	}
	rules := make([]driven.StratumRuleDoc, 0, len(cl.Rules))
	for _, r := range cl.Rules {
		rules = append(rules, driven.StratumRuleDoc{
			Name:   r.Name,
			Spread: intervalDoc(r.Spread.Lo.Kind, rateText(r.Spread.Lo), r.Spread.Hi.Kind, rateText(r.Spread.Hi)),
			Depth:  intervalDoc(r.Depth.Lo.Kind, qtyText(r.Depth.Lo), r.Depth.Hi.Kind, qtyText(r.Depth.Hi)),
		})
	}
	return driven.ClassifierDoc{DepthAsset: string(calibBase), Rules: rules}, nil
}

func rateText(b domain.RateBound) string {
	if b.Kind == domain.BoundUnbounded {
		return ""
	}
	return b.Value.Decimal()
}

func qtyText(b domain.QuantityBound) string {
	if b.Kind == domain.BoundUnbounded {
		return ""
	}
	return b.Value.Decimal()
}

func intervalDoc(loKind domain.BoundKind, lo string, hiKind domain.BoundKind, hi string) driven.IntervalDoc {
	return driven.IntervalDoc{
		Lo: driven.BoundDoc{Kind: string(loKind), Value: lo},
		Hi: driven.BoundDoc{Kind: string(hiKind), Value: hi},
	}
}
