package venue

import (
	"context"
	"fmt"
	"sort"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file is §4's AccountPostureSource surface, venue-side (task #198,
// record §73): the safety-relevant account settings §4.1 calls the flagship's
// premise, read so that §7.5's boot and §6 step 3's pre-trade guard can refuse
// to trade an account that is not in the posture the strategy assumes.
//
// # This surface REPORTS, it does not JUDGE
//
// Every value here is the venue's own word, verbatim, and nothing in this
// package knows that ISOLATED is the required margin mode or that
// dualSidePosition must be false. The rules are risk/domain's, and they are
// stated once there against a configured pin — because the moment a codec
// encodes "ISOLATED is correct", the check that compares the venue against the
// requirement has both halves supplied by the same file, and a wrong constant
// is self-consistent (record §73b F3, the leverage pin's version of the same
// trap). The codec's job is to make the venue's answer legible and to be honest
// about what it did not learn.
//
// # A posture read is SEVERAL calls, and the set differs per venue
//
// Profiles §2's assert-or-refuse row names four read paths on Binance and five
// on Bybit, on different hosts and different budgets. So the surface is an
// ordered list of PROBES rather than one call: the codec renders the steps, the
// client performs them, and each answer is decoded by the probe that asked for
// it. A single fused call would have to be invented, and the venue does not
// offer one.
//
// # Silence is never a pass
//
// The recurring defect at this seam is a required setting the venue did not
// mention being read as the value we wanted (record §73b F2 — the same field
// task #179's balance codec deliberately reads the OTHER way, for a reason that
// is correct there and wrong here). So PosturePresence is carried on every
// observation, an absent field is PostureAbsent rather than an empty string,
// and the domain refuses anything that is not PostureStated. A wire type here
// uses POINTER fields wherever absence must be distinguishable from the zero
// value, because encoding/json gives a missing key and an empty string the same
// "" without them.

// PostureProbe names one venue call in a posture read.
//
// Both venues' probes live in one enum rather than two, because the client that
// performs the steps is generic over venues and a per-venue enum would need a
// per-venue client. Each codec's decoder names the other venue's probes in a
// grouped refusal case, which exhaustiveswitch checks (§3.6 rule 10) — so a
// probe added for one venue cannot be silently unhandled by the other.
type PostureProbe uint8

const (
	// PostureProbeUnspecified is the zero value and names no probe. Refused.
	PostureProbeUnspecified PostureProbe = iota

	// PostureProbeMultiAssets is Binance GET /fapi/v1/multiAssetsMargin —
	// Single-Asset Mode, which Multi-Assets would force to Cross.
	PostureProbeMultiAssets

	// PostureProbeSymbolConfig is Binance GET /fapi/v1/symbolConfig — the
	// per-symbol margin type, configured leverage AND auto-add-margin.
	//
	// All three per-symbol settings come from this one call, and that is a
	// correction rather than a convenience (record §73o). §4.1's auto-add-
	// margin was read from GET /fapi/v3/positionRisk until 2026-08-06, when
	// Binance's own spec-generated connector confirmed what Profiles §9's
	// dated transcription had already listed: V3 does not publish the field.
	// A POSITION enumeration also cannot answer for a symbol holding no
	// position, which made the gate unpassable on every account (record §73j
	// F9 — this is its Binance half, dissolved rather than ruled on).
	PostureProbeSymbolConfig

	// PostureProbePositionDual is Binance GET /fapi/v1/positionSide/dual —
	// one-way vs Hedge position mode, which emits no venue event and is
	// therefore the reason §6.3 requires a REST re-verify at all.
	PostureProbePositionDual

	// PostureProbeAccountInfo is Bybit GET /v5/account/info — margin mode,
	// UTA status and spot hedging in one answer.
	PostureProbeAccountInfo

	// PostureProbeSpotMargin is Bybit GET /v5/spot-margin-trade/state — spot
	// margin mode, which must be off.
	PostureProbeSpotMargin

	// PostureProbeWallet is Bybit GET /v5/account/wallet-balance — read here
	// ONLY for the per-coin borrow figures. It is deliberately NOT the same
	// read as SpotBalances: that surface answers about holdings and reads an
	// absent borrow field permissively by design, which is the one reading
	// this gate must not inherit (record §73b F2).
	PostureProbeWallet

	// PostureProbePositionList is Bybit GET /v5/position/list — positionIdx
	// (one-way), per-symbol leverage and auto-add-margin.
	PostureProbePositionList
)

const maxPostureProbe = PostureProbePositionList

// Validate reports whether p names a probe, excluding the sentinel.
func (p PostureProbe) Validate() error {
	if p == PostureProbeUnspecified || p > maxPostureProbe {
		return fmt.Errorf("posture probe %d: %w", uint8(p), ErrPostureProbeInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (p PostureProbe) String() string {
	switch p {
	case PostureProbeUnspecified:
		return "PostureProbeUnspecified"
	case PostureProbeMultiAssets:
		return "PostureProbeMultiAssets"
	case PostureProbeSymbolConfig:
		return "PostureProbeSymbolConfig"
	case PostureProbePositionDual:
		return "PostureProbePositionDual"
	case PostureProbeAccountInfo:
		return "PostureProbeAccountInfo"
	case PostureProbeSpotMargin:
		return "PostureProbeSpotMargin"
	case PostureProbeWallet:
		return "PostureProbeWallet"
	case PostureProbePositionList:
		return "PostureProbePositionList"
	}
	return fmt.Sprintf("PostureProbe(%d)", uint8(p))
}

// PostureSetting names one safety-relevant account setting (§4.1).
//
// The names are this system's, not either venue's: Binance's marginType and
// Bybit's marginMode are the same concern under different spellings, and the
// domain rule is stated once over the concern. Where a setting exists on only
// one venue it is still named here, because "this venue has no such setting" is
// an answer the domain must be able to tell apart from "this venue did not say".
type PostureSetting uint8

const (
	// PostureSettingUnspecified is the zero value and names no setting.
	PostureSettingUnspecified PostureSetting = iota

	// PostureMarginMode is the perp margin mode: Binance marginType,
	// Bybit marginMode. Must be isolated on both (Profiles §2).
	PostureMarginMode

	// PostureMultiAssets is Binance's multi-assets collateral mode, which
	// forces Cross and is mutually exclusive with Isolated.
	PostureMultiAssets

	// PostureAccountKind is Bybit's unifiedMarginStatus — which account type
	// the credential is addressing.
	PostureAccountKind

	// PostureSpotHedging is Bybit's spotHedgingStatus: spot offsetting
	// derivatives re-entangles the two legs this posture exists to separate.
	PostureSpotHedging

	// PostureSpotMarginMode is Bybit's spot margin trading mode.
	PostureSpotMarginMode

	// PosturePositionMode is one-way vs Hedge: Binance dualSidePosition,
	// Bybit positionIdx.
	PosturePositionMode

	// PostureAutoAddMargin is whether the position may silently pull funds
	// from the wallet — which would defeat the isolation the posture rests on.
	PostureAutoAddMargin

	// PostureLeverage is the configured per-symbol leverage, which drives liq
	// distance and is compared against an operator-declared pin.
	PostureLeverage

	// PostureBorrowAmount is Bybit's borrowAmount on a coin.
	//
	// # Two settings for one requirement, deliberately
	//
	// Profiles §2 requires "every coin borrowAmount==0 ∧ spotBorrow==0" — two
	// venue fields, one conjunction. They are separate settings rather than one
	// PostureBorrow because an observation is keyed by (setting, scope,
	// subject): folding both fields under one setting would put two
	// observations on one key, differing only in Value, and any consumer
	// indexing by that key silently keeps one of them. Which one it keeps
	// depends on iteration order, and the one it drops is the one that was
	// non-zero half the time.
	PostureBorrowAmount

	// PostureSpotBorrow is Bybit's spotBorrow on a coin. See
	// PostureBorrowAmount for why the pair is not one member.
	PostureSpotBorrow
)

const maxPostureSetting = PostureSpotBorrow

// Validate reports whether s names a setting, excluding the sentinel.
func (s PostureSetting) Validate() error {
	if s == PostureSettingUnspecified || s > maxPostureSetting {
		return fmt.Errorf("posture setting %d: %w", uint8(s), ErrPostureSettingInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s PostureSetting) String() string {
	switch s {
	case PostureSettingUnspecified:
		return "PostureSettingUnspecified"
	case PostureMarginMode:
		return "PostureMarginMode"
	case PostureMultiAssets:
		return "PostureMultiAssets"
	case PostureAccountKind:
		return "PostureAccountKind"
	case PostureSpotHedging:
		return "PostureSpotHedging"
	case PostureSpotMarginMode:
		return "PostureSpotMarginMode"
	case PosturePositionMode:
		return "PosturePositionMode"
	case PostureAutoAddMargin:
		return "PostureAutoAddMargin"
	case PostureLeverage:
		return "PostureLeverage"
	case PostureBorrowAmount:
		return "PostureBorrowAmount"
	case PostureSpotBorrow:
		return "PostureSpotBorrow"
	}
	return fmt.Sprintf("PostureSetting(%d)", uint8(s))
}

// PostureScope is what an observation is about.
//
// Three scopes rather than one flat list, because the coverage question differs
// per scope and the domain asks it separately: an account-wide setting is
// present once or not at all, while a per-symbol setting has to be checked
// against the CONFIGURED roster — "every configured linear symbol, not only
// open positions" (Profiles §2), which a report keyed only by what the venue
// happened to return cannot answer (record §73b F4).
type PostureScope uint8

const (
	// PostureScopeUnspecified is the zero value and names no scope.
	PostureScopeUnspecified PostureScope = iota

	// PostureScopeAccount is an account-wide setting; Subject is empty.
	PostureScopeAccount

	// PostureScopeSymbol is per contract symbol; Subject is the venue symbol.
	PostureScopeSymbol

	// PostureScopeAsset is per coin; Subject is the asset in the venue's
	// spelling.
	PostureScopeAsset
)

const maxPostureScope = PostureScopeAsset

// Validate reports whether s names a scope, excluding the sentinel.
func (s PostureScope) Validate() error {
	if s == PostureScopeUnspecified || s > maxPostureScope {
		return fmt.Errorf("posture scope %d: %w", uint8(s), ErrPostureScopeInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (s PostureScope) String() string {
	switch s {
	case PostureScopeUnspecified:
		return "PostureScopeUnspecified"
	case PostureScopeAccount:
		return "PostureScopeAccount"
	case PostureScopeSymbol:
		return "PostureScopeSymbol"
	case PostureScopeAsset:
		return "PostureScopeAsset"
	}
	return fmt.Sprintf("PostureScope(%d)", uint8(s))
}

// PosturePresence is how much the venue actually said about a setting.
//
// # Why absence and emptiness are separate members
//
// Both refuse, so this is not two verdicts — it is two DIAGNOSES of one
// refusal, and an operator reading a refused boot needs to know whether the
// venue omitted the field (a shape change, or the wrong endpoint) or answered
// it blank (a value the venue does not hold). Task #179's codec already
// distinguishes them in behaviour at the same field, so collapsing them here
// would lose a distinction the wire genuinely carries.
type PosturePresence uint8

const (
	// PosturePresenceUnspecified is the zero value: an observation nobody
	// stamped. Refused, so a struct built by a test without thinking about
	// presence cannot read as a stated value.
	PosturePresenceUnspecified PosturePresence = iota

	// PostureStated is a field the venue answered with a non-empty value.
	// The ONLY presence a rule may conclude from.
	PostureStated

	// PostureStatedEmpty is a field present on the wire and empty.
	PostureStatedEmpty

	// PostureAbsent is a field the venue's answer did not carry at all.
	PostureAbsent
)

const maxPosturePresence = PostureAbsent

// Validate reports whether p names a presence, excluding the sentinel.
func (p PosturePresence) Validate() error {
	if p == PosturePresenceUnspecified || p > maxPosturePresence {
		return fmt.Errorf("posture presence %d: %w", uint8(p), ErrPosturePresenceInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (p PosturePresence) String() string {
	switch p {
	case PosturePresenceUnspecified:
		return "PosturePresenceUnspecified"
	case PostureStated:
		return "PostureStated"
	case PostureStatedEmpty:
		return "PostureStatedEmpty"
	case PostureAbsent:
		return "PostureAbsent"
	}
	return fmt.Sprintf("PosturePresence(%d)", uint8(p))
}

// PostureObservation is one setting as the venue answered it.
type PostureObservation struct {
	// Setting is which concern this is about.
	Setting PostureSetting

	// Scope is what it is about; Subject carries the symbol or asset.
	Scope PostureScope

	// Subject is the venue symbol or coin, in the venue's own spelling, and
	// is empty exactly when Scope is PostureScopeAccount.
	Subject string

	// Value is the venue's word, verbatim and untranslated. Meaningful only
	// when Presence is PostureStated.
	Value string

	// Presence is how much the venue said.
	Presence PosturePresence

	// Probe is the call this came from, so a refusal can name the endpoint
	// that answered rather than only the setting that failed.
	Probe PostureProbe
}

// Validate reports whether the observation is one a rule may reason from.
//
// It does NOT require PostureStated: an absent field is a legitimate, fully
// formed observation whose content is "the venue did not say". What it refuses
// is an observation that is incoherent — a scope with no subject, a subject on
// an account-wide setting, or a value carried by a presence that says there
// was none, which is the shape a hand-built fixture reaches for.
func (o PostureObservation) Validate() error {
	if err := o.Setting.Validate(); err != nil {
		return err
	}
	if err := o.Scope.Validate(); err != nil {
		return fmt.Errorf("%s: %w", o.Setting, err)
	}
	if err := o.Presence.Validate(); err != nil {
		return fmt.Errorf("%s: %w", o.Setting, err)
	}
	if err := o.Probe.Validate(); err != nil {
		return fmt.Errorf("%s: %w", o.Setting, err)
	}
	switch o.Scope {
	case PostureScopeAccount:
		if o.Subject != "" {
			return fmt.Errorf("%s is account-wide but names subject %q: %w",
				o.Setting, o.Subject, ErrPostureUnreadable)
		}
	case PostureScopeSymbol, PostureScopeAsset:
		if o.Subject == "" {
			return fmt.Errorf("%s is %s but names no subject: %w",
				o.Setting, o.Scope, ErrPostureUnreadable)
		}
	case PostureScopeUnspecified:
		return fmt.Errorf("%s: %w", o.Setting, ErrPostureScopeInvalid)
	}
	if o.Presence != PostureStated && o.Value != "" {
		// A value beside a presence that says there is none: whichever field
		// a rule reads, the other contradicts it, and the permissive reading
		// is the one that reads the value.
		return fmt.Errorf("%s is %s but carries value %q: %w",
			o.Setting, o.Presence, o.Value, ErrPostureUnreadable)
	}
	if o.Presence == PostureStated && o.Value == "" {
		return fmt.Errorf("%s is stated but carries no value: %w",
			o.Setting, ErrPostureUnreadable)
	}
	return nil
}

// AccountPostureReport is one venue's whole posture read.
//
// It carries observations and nothing else — no verdict, no "ok" flag. The
// report is the INPUT to §4.1's rules, and a report that already contained the
// answer would make the rule that checks it tautological (the stored-form
// lesson, one surface over).
type AccountPostureReport struct {
	// ObservedAt is the venue clock instant of the read, stamped by the
	// client after the last probe answered.
	ObservedAt clock.ServerTime

	// Observations are every setting the read learned about, sorted.
	Observations []PostureObservation
}

// AccountPostureCodec is one venue's translation of the posture surface.
type AccountPostureCodec interface {
	// EncodePosture renders the whole posture read as an ordered step list.
	//
	// Ordered because the operator-facing refusal reads better when the
	// account-wide settings come before the per-symbol ones, and because a
	// deterministic order makes the rate-limit cost of one posture read a
	// quotable number (record §73c-2 promised one).
	//
	// # symbols is the CONFIGURED roster, in the venue's own spelling
	//
	// Required, and empty is refused by the codecs that need it, on this file's
	// standing direction: an absent roster would have to mean "every symbol the
	// venue knows" or "none", and both readings return a posture nobody asked
	// for. Not normalised — the requirement matches the venue's answer verbatim,
	// so a normalisation here would be a second place two spellings reconcile.
	//
	// # It is an argument because one venue cannot answer per-symbol without it
	//
	// Binance's symbolConfig volunteers every configured symbol, traded or not,
	// so its codec ignores this. Bybit's position/list does NOT: scoped by
	// settleCoin it returns only symbols carrying an open position, so on a flat
	// account it observes nothing and §4.1's three per-symbol rows are
	// unobservable — in exactly the state a two-leg open requires. Measured
	// 2026-08-16, record §97 §4. The two codecs look symmetric and their evidence
	// is not.
	EncodePosture(symbols []string) ([]PostureStep, error)

	// DecodePostureStep reads one probe's answer.
	//
	// It does NOT stamp ObservedAt, on the codec-is-pure rule: the instant
	// belongs to the client that owns the venue-calibrated clock.
	DecodePostureStep(probe PostureProbe, resp transport.Response) ([]PostureObservation, error)
}

// PostureStep is one call in a posture read, tagged with the probe that will
// decode it.
//
// The tag travels with the call rather than being an index into the step list,
// because an index couples the decoder to the encoder's ordering and the two
// are edited months apart — a step inserted in the middle would silently
// re-point every later answer at the wrong decoder, and every field would still
// parse.
type PostureStep struct {
	Probe PostureProbe
	Call  transport.Call

	// Answers is every setting this one call establishes.
	//
	// # It exists so a family requirement can be DERIVED rather than unioned
	//
	// §4.1's posture read is routed by product family, and a family with no
	// configured endpoint makes its probe unissuable (record §73n F20). The
	// first cut required the union of every probe's family, which demanded a
	// futures endpoint from a deployment holding only the spot leg. The right
	// quantity is narrower — the families of the probes that answer a setting
	// this deployment's rules actually govern — and answering that needs the
	// probe→setting relation, which until now lived only inside
	// DecodePostureStep's body where nothing could read it (task #199).
	//
	// # It is a claim about the WIRE, and it is checked against the rule table
	//
	// This package does not know which settings matter; that is the risk
	// context's table, and this list must not restate it. The two are joined at
	// the composition root, which refuses a deployment whose table names a
	// probed setting no step answers — the shape #186 used for fold names, and
	// for the same reason: a constant and a table are two statements of one fact
	// and the pair disagrees the moment either moves.
	//
	// A step answering nothing is refused rather than skipped: it would be a
	// call this system issues, pays weight for, and derives nothing from.
	Answers []PostureSetting
}

// Validate reports whether the step is one this system may issue.
func (s PostureStep) Validate() error {
	if err := s.Probe.Validate(); err != nil {
		return err
	}
	if len(s.Answers) == 0 {
		return fmt.Errorf("posture step %s answers no setting: %w", s.Probe, ErrPostureStepAnswersNothing)
	}
	for _, a := range s.Answers {
		if err := a.Validate(); err != nil {
			return fmt.Errorf("posture step %s: %w", s.Probe, err)
		}
	}
	return nil
}

// PostureStepsFor narrows a codec's step list to the calls that establish at
// least one of the wanted settings.
//
// # This is NOT a partial read, and the distinction is the whole design
//
// §4.1's premise is a conjunction and AccountPostureSource's doc refuses a
// best-effort answer: a read that performed four of five calls has established
// nothing about the fifth. Narrowing here is a different act — it changes what
// the conjunction is OVER, decided before any call is issued, from a rule table
// that says which settings govern this deployment at all (task #199, record
// §73p). Every wanted setting is still established or the read fails.
//
// That is why a wanted setting no step answers is an ERROR rather than a
// silently shorter list. It is the same claim the composition root checks at
// config time, restated where the call is actually issued: those two run at
// different moments against different inputs, and this one is what stops a
// caller assembling a want-set the codec cannot serve.
//
// The empty want-set is refused for the reason every empty fold is refused
// here: it reads as a posture with nothing to check, and returns cleanly.
func PostureStepsFor(steps []PostureStep, want []PostureSetting) ([]PostureStep, error) {
	if len(want) == 0 {
		return nil, fmt.Errorf("posture read wants no setting: %w", ErrPostureUnreadable)
	}
	wanted := make(map[PostureSetting]struct{}, len(want))
	for _, s := range want {
		if err := s.Validate(); err != nil {
			return nil, err
		}
		wanted[s] = struct{}{}
	}

	var (
		narrowed []PostureStep
		answered = make(map[PostureSetting]struct{}, len(want))
	)
	for _, step := range steps {
		if err := step.Validate(); err != nil {
			return nil, err
		}
		var keep bool
		for _, a := range step.Answers {
			if _, ok := wanted[a]; ok {
				keep, answered[a] = true, struct{}{}
			}
		}
		if keep {
			narrowed = append(narrowed, step)
		}
	}
	for _, s := range want {
		if _, ok := answered[s]; !ok {
			return nil, fmt.Errorf("posture read wants %s and no probe answers it: %w",
				s, ErrPostureUnreadable)
		}
	}
	return narrowed, nil
}

// AccountPosture reads the account's whole safety posture.
//
// # Every probe must answer; a partial posture is a refusal
//
// There is no "best effort" reading here. §4.1's premise is a conjunction, so a
// posture read that performed four of five probes has established nothing about
// the fifth, and returning what it did learn invites a caller to fold over a
// short list and find no failures in it. The first probe error ends the read.
//
// # The urgency is the caller's
//
// §7.5's boot assert is Safety traffic. §6 step 3's pre-trade re-verify is the
// gate immediately before an authorization fsync, and is likewise Safety: a
// posture read shed as Routine would leave the trade to proceed on a stale
// snapshot, which is the exact reading §73c-2 rejected. The method refuses to
// guess, as SpotBalances does.
// The want-set names every setting the caller needs established. It is required
// rather than optional, on this file's standing direction: an absent list would
// have to mean "all of them", so a caller that forgot to build one would read
// the whole surface — issuing calls on families it may not have routed, which is
// the failure this argument exists to remove (task #199, record §73p).
func (c *OrderClient) AccountPosture(ctx context.Context, urgency ratelimit.Priority, want []PostureSetting, symbols []string) (AccountPostureReport, error) {
	op := string(c.codec.Venue()) + ".AccountPosture"

	if err := urgency.Validate(); err != nil {
		return AccountPostureReport{}, apperror.Precondition(op, "urgency", err)
	}
	codec, ok := c.codec.(AccountPostureCodec)
	if !ok {
		return AccountPostureReport{}, apperror.Precondition(op, "codec",
			fmt.Errorf("%s: %w", c.codec.Venue(), ErrPostureUnsupported))
	}

	encoded, err := codec.EncodePosture(symbols)
	if err != nil {
		return AccountPostureReport{}, apperror.Precondition(op, "encode", err)
	}
	steps, err := PostureStepsFor(encoded, want)
	if err != nil {
		return AccountPostureReport{}, apperror.Precondition(op, "encode", err)
	}
	if len(steps) == 0 {
		// A codec that renders no steps would report a posture nothing was
		// read for, and an empty observation list passes every rule that
		// folds over it looking for failures.
		//
		// Unreachable through PostureStepsFor, which refuses an empty want-set
		// and refuses a wanted setting no step answers — so a non-empty want
		// always yields a non-empty list. Kept because the reachable-looking
		// path here is the one that must never return a clean report, and the
		// cost of the guard is a comparison.
		return AccountPostureReport{}, apperror.Precondition(op, "encode",
			fmt.Errorf("%s: %w", c.codec.Venue(), ErrPostureUnreadable))
	}

	var (
		report AccountPostureReport
		last   *transport.SignedClient
	)
	for _, step := range steps {
		if err := step.Probe.Validate(); err != nil {
			return AccountPostureReport{}, apperror.Precondition(op, "encode", err)
		}
		client, ok := c.transports[step.Call.Family]
		if !ok {
			return AccountPostureReport{}, apperror.Precondition(op, "route",
				fmt.Errorf("%s family: %w", step.Probe, ErrOrderFamilyUnrouted))
		}
		call := step.Call
		call.Priority = urgency

		resp, err := client.Do(ctx, call)
		if err != nil {
			// Unwrapped, on the read path's standing rule: a probe that was
			// not answered must not be re-derived into one that answered
			// with nothing to report.
			return AccountPostureReport{}, err
		}

		obs, err := codec.DecodePostureStep(step.Probe, resp)
		if err != nil {
			return AccountPostureReport{}, apperror.Precondition(op, "decode",
				fmt.Errorf("%s: %w", step.Probe, err))
		}
		for _, o := range obs {
			if err := o.Validate(); err != nil {
				return AccountPostureReport{}, apperror.Precondition(op, "decode",
					fmt.Errorf("%s: %w", step.Probe, err))
			}
			if o.Probe != step.Probe {
				// An observation attributed to a probe that did not produce
				// it would name the wrong endpoint in a refusal, which is
				// the one line an operator acts on.
				return AccountPostureReport{}, apperror.Precondition(op, "decode",
					fmt.Errorf("%s produced an observation tagged %s: %w",
						step.Probe, o.Probe, ErrPostureUnreadable))
			}
		}
		report.Observations = append(report.Observations, obs...)
		last = client
	}

	sortPostureReport(&report)
	report.ObservedAt = last.ServerNow()
	return report, nil
}

// sortPostureReport orders observations by scope, then setting, then subject.
//
// Deterministic for the balance report's reason — the consumer renders this to
// an operator — and by SCOPE first because the account-wide refusals are the
// ones that explain a whole-venue failure, and burying them among per-symbol
// rows is how the one that matters gets skimmed past.
func sortPostureReport(r *AccountPostureReport) {
	sort.SliceStable(r.Observations, func(i, j int) bool {
		a, b := r.Observations[i], r.Observations[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Setting != b.Setting {
			return a.Setting < b.Setting
		}
		return a.Subject < b.Subject
	})
}

// postureObservation builds an observation from a pointer wire field, deriving
// the presence from the pointer rather than from the string.
//
// This is the one function that decides what silence means on this surface, and
// it is shared by both codecs so the two cannot drift: a nil pointer is
// PostureAbsent, a pointer to "" is PostureStatedEmpty, anything else is
// PostureStated. The alternative — each codec reading its own wire struct's
// strings — is how one venue's decoder ends up treating a missing field as an
// empty one, which is the permissive direction (record §73b F2).
func postureObservation(probe PostureProbe, setting PostureSetting, scope PostureScope, subject string, value *string) PostureObservation {
	o := PostureObservation{
		Setting:  setting,
		Scope:    scope,
		Subject:  subject,
		Probe:    probe,
		Presence: PostureAbsent,
	}
	switch {
	case value == nil:
		return o
	case *value == "":
		o.Presence = PostureStatedEmpty
		return o
	default:
		o.Presence = PostureStated
		o.Value = *value
		return o
	}
}

// postureBool renders a wire bool as the venue's word without inventing one for
// a field the venue omitted.
//
// Taking *bool rather than bool is the whole point: Go's zero value for bool is
// false, and false is the PASSING value for every boolean in Profiles §2's
// table — multiAssetsMargin, dualSidePosition, isAutoAddMargin, spot hedging.
// A missing field decoded into a plain bool therefore reads as "the posture is
// correct", which is the single most dangerous default this surface could have.
func postureBool(probe PostureProbe, setting PostureSetting, scope PostureScope, subject string, value *bool) PostureObservation {
	if value == nil {
		return postureObservation(probe, setting, scope, subject, nil)
	}
	word := "false"
	if *value {
		word = "true"
	}
	return postureObservation(probe, setting, scope, subject, &word)
}
