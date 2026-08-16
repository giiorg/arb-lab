// Package driven holds the risk context's driven adapters — the translations
// between this context's ports and the machinery that serves them.
package driven

import (
	"context"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
	"github.com/giiorg/arb-lab/internal/risk/application/ports"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// AccountPosture is §4.1's posture read as an ACL adapter: engine/venue's
// posture surface translated into this context's port (task #198, record §73).
//
// # The translation is the point, not boilerplate
//
// The two vocabularies are near-identical today and will not stay that way: the
// venue side's shape is driven by what an exchange can be asked (its probe enum
// grows with endpoints), and the domain side's by what §4.1 requires. Mapping
// them explicitly means a venue that grows a setting arrives here as a compile
// error or a named refusal, rather than as a silently-compiling struct change
// that adds an unjudged field to a report the rules fold over.
//
// # Priority is a CONSTANT here, and the record is why
//
// §7.6 puts read priority at the call site, and both of this port's callers are
// Safety by record §73c-2's own argument: §7.5's boot assert is Safety traffic,
// and §6 step 3's pre-trade re-verify is the gate immediately before an
// authorization fsync — shedding it as Routine would leave the trade to proceed
// on a stale snapshot, which is the reading that ruling rejected. SpotBalances
// takes the rung as a constructor argument because ITS two callers genuinely
// sit on different rungs (a maintenance record command must not spend the
// emergency reserve); here a third caller wanting a different rung has to edit
// this line and argue with this paragraph, which is the intended cost.
type AccountPosture struct {
	client  AccountPostureReader
	venueID ids.Venue
}

// posturePriority is the rung every posture read rides. See the type comment.
const posturePriority = ratelimit.PrioritySafety

// AccountPostureReader is the venue-side capability this adapter needs,
// declared where it is CONSUMED rather than imported as a concrete client —
// the shape SpotBalanceReader and VenuePositionReader already take.
type AccountPostureReader interface {
	AccountPosture(ctx context.Context, urgency ratelimit.Priority, want []venue.PostureSetting, symbols []string) (venue.AccountPostureReport, error)
}

// NewAccountPosture builds the adapter.
func NewAccountPosture(client AccountPostureReader, v ids.Venue) (*AccountPosture, error) {
	if client == nil {
		return nil, ErrAccountPostureReaderMissing
	}
	if err := v.Validate(); err != nil {
		return nil, fmt.Errorf("account posture venue: %w", err)
	}
	return &AccountPosture{client: client, venueID: v}, nil
}

// AccountPosture implements ports.AccountPostureSource.
func (a *AccountPosture) AccountPosture(scope ports.PostureScope) (domain.Report, error) {
	if err := scope.Validate(); err != nil {
		return domain.Report{}, err
	}
	if scope.Venue != a.venueID {
		// One adapter speaks for one venue's client, and a scope naming another
		// would be answered by the wrong exchange entirely — with every
		// individual field still decoding, and every rule still evaluating.
		return domain.Report{}, fmt.Errorf("scope %s: %w", scope, ErrAccountPostureVenueMismatch)
	}

	// Not cancellable, on SpotBalances' argument: cancelling converts "the
	// venue was about to say what the account's posture is" into "the read was
	// not answered", and the second refuses to conclude the assertion — which
	// is correct, but it would make an unrelated shutdown look like a posture
	// failure at the one gate an operator reads most carefully.
	want := make([]venue.PostureSetting, 0, len(scope.Settings))
	for _, s := range scope.Settings {
		translated, err := venueSettingFor(s)
		if err != nil {
			return domain.Report{}, err
		}
		want = append(want, translated)
	}

	// The roster travels UNTRANSLATED, unlike the settings beside it: a setting
	// is this context's vocabulary and needs mapping, while a symbol is the
	// venue's own spelling all the way down — domain.SymbolPin holds it
	// unnormalised for exactly this reason, so a translation here would be the
	// second place two spellings reconcile.
	report, err := a.client.AccountPosture(context.Background(), posturePriority, want, scope.Symbols)
	if err != nil {
		// Unwrapped: engine/venue already classified the effect, and a read
		// that failed must not be re-derived into an empty report.
		return domain.Report{}, err
	}

	out := domain.Report{
		Venue:        a.venueID,
		ObservedAt:   report.ObservedAt,
		Observations: make([]domain.Observation, 0, len(report.Observations)),
	}
	for _, o := range report.Observations {
		translated, err := translateObservation(o)
		if err != nil {
			return domain.Report{}, fmt.Errorf("posture on %s: %w", a.venueID, err)
		}
		out.Observations = append(out.Observations, translated)
	}
	if err := out.Validate(); err != nil {
		// The venue surface validates its own observations, so this catches
		// what the TRANSLATION could break rather than what the venue sent —
		// a mapping that drops a subject, or one that pairs a value with the
		// wrong presence.
		return domain.Report{}, err
	}
	return out, nil
}

// translateObservation maps one venue observation into the domain's.
func translateObservation(o venue.PostureObservation) (domain.Observation, error) {
	setting, err := TranslateSetting(o.Setting)
	if err != nil {
		return domain.Observation{}, err
	}
	scope, err := translateScope(o.Scope)
	if err != nil {
		return domain.Observation{}, err
	}
	presence, err := translatePresence(o.Presence)
	if err != nil {
		return domain.Observation{}, err
	}
	return domain.Observation{
		Setting:  setting,
		Scope:    scope,
		Subject:  o.Subject,
		Value:    o.Value,
		Presence: presence,
		// The probe's NAME travels, not its identity: the domain renders it in
		// a refusal so an operator knows which endpoint answered, and nothing
		// there branches on it.
		Source: o.Probe.String(),
	}, nil
}

// TranslateSetting maps the venue's setting vocabulary onto the domain's.
//
// Every member is named (§3.6 rule 10), so a setting added to the venue surface
// has to choose a side here. The sentinel refuses rather than mapping to the
// domain's own sentinel, because a zero-valued setting arriving in a report
// would be refused later with a message about the DOMAIN's vocabulary — which
// sends a reader to the wrong file.
//
// venue.PostureSetting has no PortfolioMargin member and this switch therefore
// cannot produce one, which is correct and load-bearing: that setting has no
// venue read path at all (record §73b F1), so the only thing that could
// satisfy it is the operator attestation the requirement carries.
//
// # Why it is exported, having been a detail of the read path
//
// It has a second caller that is not on the wire path at all: the composition
// root, which joins a codec's PostureStep.Answers against the domain's rule
// table to decide which product families §4.1's read rides (task #199, record
// §73p). That join has to cross the same vocabulary boundary this ACL exists
// for, at CONFIG time rather than at decode time, and a second translation
// written beside it would be a second statement of one fact.
// venueSettingFor is TranslateSetting's inverse: the domain says which settings
// it needs established, and the read is issued in the venue's vocabulary.
//
// # Why it is a second switch and not a reversed map
//
// A map built by inverting the one above would agree with it by construction,
// which is the property that makes it worthless as a check — the pair could
// then be jointly wrong and nothing would notice. Written out, the two are
// independent statements and the round-trip test below can disagree with either.
//
// SettingPortfolioMargin is the one domain setting with no venue member, and it
// is UNREACHABLE here rather than merely absent: ProbedSettings excludes every
// attested rule, so the guard never asks for it. The refusal is kept because
// "unreachable" is a claim about today's callers, and the alternative — mapping
// it to some nearby member — would silently satisfy §4.1's one attestation with
// a venue read (record §73b F1 is why that must never happen).
func venueSettingFor(s domain.Setting) (venue.PostureSetting, error) {
	switch s {
	case domain.SettingMarginMode:
		return venue.PostureMarginMode, nil
	case domain.SettingMultiAssets:
		return venue.PostureMultiAssets, nil
	case domain.SettingAccountKind:
		return venue.PostureAccountKind, nil
	case domain.SettingSpotHedging:
		return venue.PostureSpotHedging, nil
	case domain.SettingSpotMarginMode:
		return venue.PostureSpotMarginMode, nil
	case domain.SettingPositionMode:
		return venue.PosturePositionMode, nil
	case domain.SettingAutoAddMargin:
		return venue.PostureAutoAddMargin, nil
	case domain.SettingLeverage:
		return venue.PostureLeverage, nil
	case domain.SettingBorrowAmount:
		return venue.PostureBorrowAmount, nil
	case domain.SettingSpotBorrow:
		return venue.PostureSpotBorrow, nil
	case domain.SettingPortfolioMargin:
		return venue.PostureSettingUnspecified, fmt.Errorf(
			"setting %s has no venue read path and must be attested: %w",
			s, ErrAccountPostureUntranslatable)
	case domain.SettingUnspecified:
		return venue.PostureSettingUnspecified, fmt.Errorf("posture scope states no setting: %w",
			ErrAccountPostureUntranslatable)
	}
	return venue.PostureSettingUnspecified, fmt.Errorf("domain setting %s: %w", s, ErrAccountPostureUntranslatable)
}

func TranslateSetting(s venue.PostureSetting) (domain.Setting, error) {
	switch s {
	case venue.PostureMarginMode:
		return domain.SettingMarginMode, nil
	case venue.PostureMultiAssets:
		return domain.SettingMultiAssets, nil
	case venue.PostureAccountKind:
		return domain.SettingAccountKind, nil
	case venue.PostureSpotHedging:
		return domain.SettingSpotHedging, nil
	case venue.PostureSpotMarginMode:
		return domain.SettingSpotMarginMode, nil
	case venue.PosturePositionMode:
		return domain.SettingPositionMode, nil
	case venue.PostureAutoAddMargin:
		return domain.SettingAutoAddMargin, nil
	case venue.PostureLeverage:
		return domain.SettingLeverage, nil
	case venue.PostureBorrowAmount:
		return domain.SettingBorrowAmount, nil
	case venue.PostureSpotBorrow:
		return domain.SettingSpotBorrow, nil
	case venue.PostureSettingUnspecified:
		return domain.SettingUnspecified, fmt.Errorf("venue observation states no setting: %w",
			ErrAccountPostureUntranslatable)
	}
	return domain.SettingUnspecified, fmt.Errorf("venue setting %s: %w", s, ErrAccountPostureUntranslatable)
}

// translateScope maps the venue's scope vocabulary onto the domain's.
func translateScope(s venue.PostureScope) (domain.Scope, error) {
	switch s {
	case venue.PostureScopeAccount:
		return domain.ScopeAccount, nil
	case venue.PostureScopeSymbol:
		return domain.ScopeSymbol, nil
	case venue.PostureScopeAsset:
		return domain.ScopeAsset, nil
	case venue.PostureScopeUnspecified:
		return domain.ScopeUnspecified, fmt.Errorf("venue observation states no scope: %w",
			ErrAccountPostureUntranslatable)
	}
	return domain.ScopeUnspecified, fmt.Errorf("venue scope %s: %w", s, ErrAccountPostureUntranslatable)
}

// translatePresence maps the venue's presence vocabulary onto the domain's.
//
// This is the mapping with the most at stake in the whole adapter: collapsing
// the two silences into one, or either of them into "stated", is how a field
// the venue never mentioned becomes a value the rules conclude from (record
// §73b F2). All three non-stated members are carried across distinctly.
func translatePresence(p venue.PosturePresence) (domain.Presence, error) {
	switch p {
	case venue.PostureStated:
		return domain.PresenceStated, nil
	case venue.PostureStatedEmpty:
		return domain.PresenceStatedEmpty, nil
	case venue.PostureAbsent:
		return domain.PresenceAbsent, nil
	case venue.PosturePresenceUnspecified:
		return domain.PresenceUnspecified, fmt.Errorf("venue observation states no presence: %w",
			ErrAccountPostureUntranslatable)
	}
	return domain.PresenceUnspecified, fmt.Errorf("venue presence %s: %w", p, ErrAccountPostureUntranslatable)
}

// Compile-time proof that the adapter satisfies the port (§3.6 rule 2).
var _ ports.AccountPostureSource = (*AccountPosture)(nil)
