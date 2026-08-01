package driven

// The §45 cross-venue gate artifact's schema and loader — strategy #2's
// milestone-2b verdict, written once and approved out of band.
//
// It is a different document from the step-4 calibration artifact and
// deliberately not a variant of it: that one freezes NUMBERS a later run
// consumes (quantiles, cost cells, a band), while this one records a
// VERDICT and the evidence behind it. Sharing a schema would have meant a
// document where half the blocks are absent in either mode, and an absent
// block is indistinguishable from a forgotten one.
//
// What it shares instead is governance, and it shares it by reusing the
// same machinery rather than restating it: the execution binding is the
// same two fields, the approval is the same detached AttestationDoc
// verified through the same BindIdentity, and the write-once rule is the
// same GuardNewVersion. §41e's ceremony is not calibration-specific — it is
// what this project does before a number is allowed to authorize anything.
//
// # Why the verdict block is a POINTER
//
// A gate run that cannot score writes no artifact at all (the runner
// refuses by name), so every artifact that exists carries a verdict. The
// pointer is not for that case — it is so that a document with the block
// MISSING fails to load rather than loading with a false verdict and a
// zero LCB. A bool and a string default to "did not clear", which reads as
// a legitimate kill; the one thing worse than a wrong verdict here is a
// wrong verdict in the direction that looks like the honest outcome.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Gate artifact sentinels.
var (
	// ErrGateArtifactInvalid reports a malformed member; the wrapping
	// message names it.
	ErrGateArtifactInvalid = fmt.Errorf("strategy/driven: invalid gate artifact member")

	// ErrGateArtifactSchema reports a document whose schema token is not
	// one this build knows. A gate artifact outlives the binary that wrote
	// it, and silently reading a v2 document with v1 rules is how a
	// verdict comes to mean something its author did not write.
	ErrGateArtifactSchema = fmt.Errorf("strategy/driven: unknown gate artifact schema")
)

// GateArtifactSchema is this document's schema token.
const GateArtifactSchema = "arblab.crossvenue.gate.v1"

// GateProcedureDoc is §41e's execution binding, restated for this artifact:
// the commit whose procedure_crossvenue.go declared every parameter, and
// the digest of the executable that ran it.
type GateProcedureDoc struct {
	Label            string `json:"label"`
	Commit           string `json:"commit"`
	ExecutableSHA256 string `json:"executable_sha256"`

	// ReferenceFile and ReferenceSHA256 name the committed reference
	// revision the fees and hurdle were read from. Without them the
	// artifact records the NUMBERS it used but not the revision that
	// supplied them, and §2.4's replayed-revision rule needs the revision.
	ReferenceFile   string `json:"reference_file"`
	ReferenceSHA256 string `json:"reference_sha256"`
}

// GateParametersDoc echoes the roster the run was pinned to. It is written
// out rather than left implicit in the commit hash because the hash proves
// the values did not change and says nothing about what they were — an
// artifact a reader cannot audit without checking out its commit is an
// artifact nobody audits.
type GateParametersDoc struct {
	Symbols []GateSymbolParamDoc `json:"symbols"`

	MatchedFloor    int    `json:"matched_floor"`
	RefusalCeiling  string `json:"refusal_ceiling"`
	VoidImbalance   bool   `json:"void_imbalance"`
	SampleEvery     string `json:"sample_every"`
	SampleSlack     string `json:"sample_slack"`
	MaxGap          string `json:"max_gap"`
	ExitDivisor     int    `json:"exit_divisor"`
	PairingRound    string `json:"pairing_round"`
	P90             string `json:"p90"`
	Period          string `json:"period"`
	MeanBlock       int    `json:"mean_block"`
	Resamples       int    `json:"resamples"`
	Confidence      string `json:"confidence"`
	Seed            uint64 `json:"seed"`
	NotionalTarget  string `json:"notional_target_per_leg"`
	BinanceTakerFee string `json:"binance_taker_fee"`
	BybitTakerFee   string `json:"bybit_taker_fee"`

	// Denominator and Holdout are the two declarations that change how
	// every number above should be READ. They are prose on purpose: what
	// they record is an argument, and an argument compressed to a flag is
	// an argument nobody can check.
	Denominator string `json:"denominator"`
	Holdout     string `json:"holdout"`
}

// GateSymbolParamDoc is one walked symbol's declared level and size.
type GateSymbolParamDoc struct {
	Symbol string `json:"symbol"`
	Entry  string `json:"entry"`
	Qty    string `json:"qty"`
	Weight string `json:"weight"`
}

// GateEvidenceDoc is the dataset ledger: exactly what the run read.
type GateEvidenceDoc struct {
	// The seal's two instants and the run's declared bound. The bound is
	// what makes two runs days apart name different universes in their
	// artifacts rather than by when they were invoked.
	SettledCutoff string `json:"settled_cutoff"`
	BooksCutoff   string `json:"books_cutoff"`
	Before        string `json:"before"`

	// ArchiveDays enumerates the manifest days walked, each with the
	// manifest's own recorded digest.
	ArchiveDays []GateArchiveDayDoc `json:"archive_days"`

	// SettledFiles enumerates the settled-funding store files read, with
	// the digests the store recorded when it fetched them.
	SettledFiles []LedgerEntryDoc `json:"settled_files"`
}

// GateArchiveDayDoc is one walked archive day.
type GateArchiveDayDoc struct {
	Day    string `json:"day"`
	SHA256 string `json:"sha256"`
}

// GateSymbolDoc is one symbol's evidence and its own verdict — §45f-3's
// "per-symbol verdicts reported as evidence".
type GateSymbolDoc struct {
	Symbol string `json:"symbol"`

	// MatchedSettlements is what the end rule (§45f-1) reads.
	MatchedSettlements int `json:"matched_settlements"`

	// DecisionPoints is the joined two-venue signal grid's size, and the
	// skip counts are the coverage behind it. A symbol with few episodes
	// and heavy skipping is a thin WALK, not a quiet market, and only
	// these three numbers together say which.
	DecisionPoints int `json:"decision_points"`
	BinanceSkipped int `json:"binance_skipped"`
	BybitSkipped   int `json:"bybit_skipped"`

	Episodes int `json:"episodes"`
	Scored   int `json:"scored"`
	Refused  int `json:"refused_episodes"`

	// SealedExcluded counts settled rows outside the declared window.
	SealedExcluded int `json:"sealed_excluded"`

	// AnnualizedReturn is this symbol's own standalone figure, on the same
	// basis as the portfolio's. Evidence, never a pass condition — ruling
	// §45f-3 rejected any-symbol-clears explicitly.
	AnnualizedReturn string `json:"annualized_return"`
}

// GateVerdictDoc is the pass/fail and the arithmetic behind it.
type GateVerdictDoc struct {
	Periods int `json:"periods"`

	// PortfolioLCB is the bootstrap lower bound on the mean PER-PERIOD
	// portfolio return; AnnualizedLCB is that figure on the year basis the
	// hurdle is quoted on. Both are recorded because the annualization is
	// a step somebody will want to check.
	PortfolioLCB  string `json:"portfolio_lcb_per_period"`
	AnnualizedLCB string `json:"portfolio_lcb_annualized"`

	// Hurdle is the effective rate after §2.1's three recorded
	// adjustments; HurdlePublished and HurdleAsOf are the row it came from.
	Hurdle          string `json:"hurdle_effective"`
	HurdlePublished string `json:"hurdle_published"`
	HurdleAsOf      string `json:"hurdle_as_of"`

	// Clears is the verdict: annualized LCB at or above the effective
	// hurdle.
	Clears bool `json:"clears"`
}

// GateArtifactDoc is the whole committed document.
type GateArtifactDoc struct {
	Schema     string            `json:"schema"`
	Procedure  GateProcedureDoc  `json:"procedure"`
	Parameters GateParametersDoc `json:"parameters"`
	Evidence   GateEvidenceDoc   `json:"evidence"`
	Symbols    []GateSymbolDoc   `json:"symbols"`
	Verdict    *GateVerdictDoc   `json:"verdict"`
}

// Validate makes the document loadable. Validation is by CONVERSION where a
// member has a domain type — the rate members parse, so a document carrying
// an unparseable confidence cannot load and then be believed.
func (d GateArtifactDoc) Validate() error {
	if d.Schema != GateArtifactSchema {
		return fmt.Errorf("schema %q, want %q: %w", d.Schema, GateArtifactSchema, ErrGateArtifactSchema)
	}
	if d.Procedure.Label == "" {
		return fmt.Errorf("procedure label: %w", ErrGateArtifactInvalid)
	}
	if d.Procedure.Commit == "" {
		return fmt.Errorf("procedure commit: %w", ErrGateArtifactInvalid)
	}
	if err := CheckHashText(d.Procedure.ExecutableSHA256); err != nil {
		return fmt.Errorf("executable_sha256: %w", err)
	}
	if d.Procedure.ReferenceFile == "" {
		return fmt.Errorf("reference_file: %w", ErrGateArtifactInvalid)
	}
	if err := CheckHashText(d.Procedure.ReferenceSHA256); err != nil {
		return fmt.Errorf("reference_sha256: %w", err)
	}

	if len(d.Parameters.Symbols) == 0 {
		return fmt.Errorf("parameters symbols: %w", ErrGateArtifactInvalid)
	}
	for n, s := range d.Parameters.Symbols {
		if s.Symbol == "" {
			return fmt.Errorf("parameters symbols[%d]: %w", n, ErrGateArtifactInvalid)
		}
		for member, text := range map[string]string{"entry": s.Entry, "qty": s.Qty, "weight": s.Weight} {
			if _, err := fixedpoint.ParseRate(text); err != nil {
				return fmt.Errorf("parameters symbols[%d] %s %q: %w: %w", n, member, text, ErrGateArtifactInvalid, err)
			}
		}
	}
	if d.Parameters.MatchedFloor < 1 {
		return fmt.Errorf("matched_floor %d: %w", d.Parameters.MatchedFloor, ErrGateArtifactInvalid)
	}
	for member, text := range map[string]string{
		"refusal_ceiling":   d.Parameters.RefusalCeiling,
		"p90":               d.Parameters.P90,
		"confidence":        d.Parameters.Confidence,
		"binance_taker_fee": d.Parameters.BinanceTakerFee,
		"bybit_taker_fee":   d.Parameters.BybitTakerFee,
	} {
		if _, err := fixedpoint.ParseRate(text); err != nil {
			return fmt.Errorf("parameters %s %q: %w: %w", member, text, ErrGateArtifactInvalid, err)
		}
	}
	for member, text := range map[string]string{
		"sample_every":  d.Parameters.SampleEvery,
		"sample_slack":  d.Parameters.SampleSlack,
		"max_gap":       d.Parameters.MaxGap,
		"pairing_round": d.Parameters.PairingRound,
		"period":        d.Parameters.Period,
	} {
		if _, err := time.ParseDuration(text); err != nil {
			return fmt.Errorf("parameters %s %q: %w: %w", member, text, ErrGateArtifactInvalid, err)
		}
	}
	if d.Parameters.Denominator == "" {
		return fmt.Errorf("parameters denominator: %w", ErrGateArtifactInvalid)
	}
	if d.Parameters.Holdout == "" {
		return fmt.Errorf("parameters holdout: %w", ErrGateArtifactInvalid)
	}

	for member, text := range map[string]string{
		"settled_cutoff": d.Evidence.SettledCutoff,
		"books_cutoff":   d.Evidence.BooksCutoff,
		"before":         d.Evidence.Before,
	} {
		if _, err := time.Parse(time.RFC3339, text); err != nil {
			return fmt.Errorf("evidence %s %q: %w: %w", member, text, ErrGateArtifactInvalid, err)
		}
	}
	if len(d.Evidence.ArchiveDays) == 0 {
		return fmt.Errorf("evidence archive_days: %w", ErrGateArtifactInvalid)
	}
	for n, day := range d.Evidence.ArchiveDays {
		if _, err := time.Parse("2006-01-02", day.Day); err != nil {
			return fmt.Errorf("evidence archive_days[%d] %q: %w: %w", n, day.Day, ErrGateArtifactInvalid, err)
		}
		if err := CheckHashText(day.SHA256); err != nil {
			return fmt.Errorf("evidence archive_days[%d]: %w", n, err)
		}
	}
	if len(d.Evidence.SettledFiles) == 0 {
		return fmt.Errorf("evidence settled_files: %w", ErrGateArtifactInvalid)
	}
	for n, f := range d.Evidence.SettledFiles {
		if f.File == "" {
			return fmt.Errorf("evidence settled_files[%d] file: %w", n, ErrGateArtifactInvalid)
		}
		if err := CheckHashText(f.SHA256); err != nil {
			return fmt.Errorf("evidence settled_files[%d]: %w", n, err)
		}
	}

	// The scored roster must be the declared roster. A document scoring
	// seven of eight declared symbols is a portfolio nobody declared, and
	// ruling §45f-3's whole protection is that the policy was fixed first.
	if len(d.Symbols) != len(d.Parameters.Symbols) {
		return fmt.Errorf("%d scored symbols against %d declared: %w",
			len(d.Symbols), len(d.Parameters.Symbols), ErrGateArtifactInvalid)
	}
	for n, s := range d.Symbols {
		if s.Symbol != d.Parameters.Symbols[n].Symbol {
			return fmt.Errorf("symbols[%d] is %s, declared %s: %w",
				n, s.Symbol, d.Parameters.Symbols[n].Symbol, ErrGateArtifactInvalid)
		}
		if s.MatchedSettlements < d.Parameters.MatchedFloor {
			return fmt.Errorf("symbols[%d] %s has %d matched settlements against floor %d: %w",
				n, s.Symbol, s.MatchedSettlements, d.Parameters.MatchedFloor, ErrGateArtifactInvalid)
		}
		if _, err := fixedpoint.ParseRate(s.AnnualizedReturn); err != nil {
			return fmt.Errorf("symbols[%d] annualized_return %q: %w: %w", n, s.AnnualizedReturn, ErrGateArtifactInvalid, err)
		}
	}

	if d.Verdict == nil {
		return fmt.Errorf("verdict: %w", ErrGateArtifactInvalid)
	}
	if d.Verdict.Periods < 1 {
		return fmt.Errorf("verdict periods %d: %w", d.Verdict.Periods, ErrGateArtifactInvalid)
	}
	for member, text := range map[string]string{
		"portfolio_lcb_per_period": d.Verdict.PortfolioLCB,
		"portfolio_lcb_annualized": d.Verdict.AnnualizedLCB,
		"hurdle_effective":         d.Verdict.Hurdle,
		"hurdle_published":         d.Verdict.HurdlePublished,
	} {
		if _, err := fixedpoint.ParseRate(text); err != nil {
			return fmt.Errorf("verdict %s %q: %w: %w", member, text, ErrGateArtifactInvalid, err)
		}
	}
	if _, err := time.Parse(time.RFC3339, d.Verdict.HurdleAsOf); err != nil {
		return fmt.Errorf("verdict hurdle_as_of %q: %w: %w", d.Verdict.HurdleAsOf, ErrGateArtifactInvalid, err)
	}

	// The recorded verdict must be the recorded arithmetic's own answer.
	// This is the one check that catches a document whose every member is
	// well-formed and whose bool disagrees with its numbers — hand-edited,
	// or written by a runner with the comparison inverted.
	lcb, err := fixedpoint.ParseRate(d.Verdict.AnnualizedLCB)
	if err != nil {
		return err
	}
	hurdle, err := fixedpoint.ParseRate(d.Verdict.Hurdle)
	if err != nil {
		return err
	}
	c, err := lcb.Cmp(hurdle)
	if err != nil {
		return fmt.Errorf("verdict comparison: %w", err)
	}
	if got := c >= 0; got != d.Verdict.Clears {
		return fmt.Errorf("verdict records clears=%t while LCB %s against hurdle %s says %t: %w",
			d.Verdict.Clears, d.Verdict.AnnualizedLCB, d.Verdict.Hurdle, got, ErrGateArtifactInvalid)
	}
	return nil
}

// GateArtifact is the loaded document plus the identity an attestation
// binds. The content hash is computed HERE, from the exact bytes, because a
// hash the document carried would sit inside its own preimage (§41d).
type GateArtifact struct {
	Doc      GateArtifactDoc
	Identity ApprovedIdentity
}

// LoadGateArtifact turns a gate artifact's exact bytes into the loaded
// value. The id is the artifact's identity — §41d's rule, the file's
// basename without extension — supplied by the caller because the bytes do
// not know their own filename.
func LoadGateArtifact(id string, data []byte) (*GateArtifact, error) {
	if id == "" {
		return nil, fmt.Errorf("gate artifact id: %w", ErrGateArtifactInvalid)
	}
	doc, err := config.Decode[GateArtifactDoc](data)
	if err != nil {
		return nil, fmt.Errorf("gate artifact: %w", err)
	}
	sum := sha256.Sum256(data)
	return &GateArtifact{
		Doc: doc,
		Identity: ApprovedIdentity{
			ID:               id,
			ContentSHA256:    hex.EncodeToString(sum[:]),
			ProcedureCommit:  doc.Procedure.Commit,
			ExecutableSHA256: doc.Procedure.ExecutableSHA256,
		},
	}, nil
}

// LoadGateArtifactFile loads a written gate artifact from disk, taking its
// identity from its filename.
func LoadGateArtifactFile(path string) (*GateArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("gate artifact file: %w", err)
	}
	return LoadGateArtifact(ArtifactIDFromPath(path), data)
}
