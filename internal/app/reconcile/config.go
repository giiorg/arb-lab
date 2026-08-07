package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
	riskdomain "github.com/giiorg/arb-lab/internal/risk/domain"
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation"
	riskdriven "github.com/giiorg/arb-lab/internal/risk/infrastructure/driven"
)

// Config is §5.2's configuration schema for the OMS, which is the construct three
// slices of deferral named as the thing `arblab reconcile` was waiting on.
//
// # It is the OMS's slice of §5.2 and NOT the whole schema
//
// §5.2 lists global risk caps, market-data freshness budgets, per-strategy
// params, a strategy registry and the emergency policy beside the per-venue
// block. None of those has a consumer in a reconcile-only boot: the risk caps are
// risk's, the freshness budgets are marketdata's, and a field declared here for
// them would be a schema frozen against no reader — this repo's recorded shape
// for a claim wearing a type annotation. They land with their contexts.
//
// What IS here is everything §7.5's boot reads, and the test of that is
// mechanical: every field below is an argument to something oms.New or
// venue.NewOrderClient refuses to be built without.
//
// # No field is hot-reloadable, and that is written by omission
//
// platform/config is default-DENY and none of these carries `config:"hot"`.
// Every one of them is read once, during a boot phase, by a process whose whole
// job finishes before a reload could arrive — and the two that would be
// disastrous to change under a running engine are exactly the two a careless
// `hot` would be added to: JournalPath (a running process picking up a new
// journal mid-trade is §5.2's own worked example) and the venue endpoints.
type Config struct {
	// JournalPath is where §5.1's write-ahead journal lives on this host.
	//
	// §13 adds a precondition on it that no constructor checks — local filesystem
	// only, never NFS or a network FS — and this is the value that meets or
	// misses it. See checkJournalStorage, which is the boot gate #168 asked for.
	JournalPath string `json:"journal_path"`

	// Account is §5.2's `subaccount id`: the venue subaccount this OMS instance
	// trades, and §5.1's first hedge-group and fill-dedupe key component.
	Account string `json:"account"`

	// Venue is §5.2's per-venue block. ONE, singular, and the singular is a
	// property of what it configures rather than a simplification: oms.Deps takes
	// one venue client, so a second block here would have nowhere to go. A
	// deployment trading two venues runs two of these, against two journals.
	Venue VenueConfig `json:"venue"`

	// Instruments is every instrument this deployment resolves orders for, with
	// the base/quote decomposition the read path needs to mint money from venue
	// strings. driven.NewOrderResolver states why it cannot be parsed from a
	// symbol.
	Instruments []InstrumentConfig `json:"instruments"`

	// PositionScopes are the enumerations §7.5's net-delta invariant is asserted
	// over.
	//
	// It is NOT derived from Instruments, and the reason is the one that makes the
	// enumeration a check at all: it exists to find exposure on instruments this
	// system has never heard of, so a scope computed from the ones it trades could
	// not widen past them. What Validate does instead is refuse a document whose
	// scopes do not COVER the derivative instruments — the coverage direction,
	// which is the safe one, since a scope may always name more.
	PositionScopes []ScopeConfig `json:"position_scopes"`

	// Resolution is the local policy §5.3's stable-not-found rule needs and no
	// venue publishes.
	Resolution ResolutionConfig `json:"resolution"`

	// SpotBaseline pins the recorded spot exposure baseline document — the
	// path AND its sha256, because the digest is what enforces the two-act
	// rule: `arblab baseline record` writes a NEW document (O_EXCL), and only
	// this config edit makes it govern anything (task #179, record §70b).
	//
	// OPTIONAL, and the optionality is load-bearing (§70g): a required field
	// is circular on a virgin deployment — the record command runs off this
	// same validated config. Absence is not a config error; it is the
	// INVARIANT's own named refusal: every spot instrument reports
	// UnassertedBaselineUnrecorded, which holds the posture. When the field
	// IS present, the document must load, chain-verify and bind, or the boot
	// errors — a pinned document that will not load is a wiring defect, not a
	// gate refusal.
	SpotBaseline *BaselinePinConfig `json:"spot_baseline,omitempty"`

	// SpotSettle is §5.2's `spot_settle`: the settle bound both the record
	// protocol and the boot's spot second look wait — one rule, two callers
	// (record §70c). Required whenever a spot instrument is configured; a
	// positive duration (seconds, for the IOC-only MVP). It is an OPERATOR
	// statement of how stale the venue's own read side may be about its own
	// matching engine — the same kind of claim as resolution.max_landing,
	// not the same claim.
	SpotSettle config.Duration `json:"spot_settle,omitempty"`

	// PortfolioMarginAttestation pins the operator's Portfolio-Margin
	// attestation document — the path AND its sha256, for the same reason
	// SpotBaseline carries both: the digest is the second act (task #198,
	// record §73c-1).
	//
	// OPTIONAL as a FIELD and not as a fact. Absence is not a config error and
	// it is not a pass either: it produces domain.AttestationUnstated, and
	// §4.1's Binance rule table turns that into a named gate refusal
	// (ReasonUnattested) that tells the operator which fact to establish. A
	// required field would report the same account state as a malformed
	// document, which sends them to the wrong file.
	//
	// Refused on a deployment whose account model has no check that reads it —
	// see Validate. A pinned document nothing consumes is #179's precedent
	// exactly: the operator who pinned it believes a check is running.
	PortfolioMarginAttestation *AttestationPinConfig `json:"portfolio_margin_attestation,omitempty"`

	// WatchdogSilence is §7.4's host-silence threshold and StopBudget is what each
	// phase's closer is allotted. boot.Sequence refuses a total teardown at or
	// above the threshold, because an ordinary restart that outlasts it presents
	// to the watchdog exactly as a dead host.
	WatchdogSilence config.Duration `json:"watchdog_silence"`
	StopBudget      config.Duration `json:"stop_budget"`
}

// AttestationPinConfig is the Portfolio-Margin attestation document's pin.
//
// Both halves required, on BaselinePinConfig's argument — and here the digest
// carries more weight than it does there, because the attestation package has no
// writer to guard with O_EXCL. It is the only thing standing between "somebody
// edited the file" and "the gate consumes a different claim".
type AttestationPinConfig struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func (p AttestationPinConfig) validate() error {
	if p.Path == "" {
		return fmt.Errorf("portfolio_margin_attestation.path: %w", ErrConfigFieldUnset)
	}
	if p.SHA256 == "" {
		return fmt.Errorf("portfolio_margin_attestation.sha256: %w", ErrConfigFieldUnset)
	}
	raw, err := hex.DecodeString(p.SHA256)
	if err != nil || len(raw) != sha256.Size {
		return fmt.Errorf("portfolio_margin_attestation.sha256 %q is not a 32-byte hex digest: %w",
			p.SHA256, ErrConfigFieldInvalid)
	}
	return nil
}

// Pin is the loader's value. A method rather than a shared struct so that the
// config schema and the attestation package can be edited independently.
func (p AttestationPinConfig) Pin() attestation.Pin {
	return attestation.Pin{Path: p.Path, SHA256: p.SHA256}
}

// BaselinePinConfig is the spot baseline document's pin: where it is and what
// its bytes digest to. Both halves required — a path-only pin would re-open
// the edit-in-place hole the digest exists to close.
type BaselinePinConfig struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func (p BaselinePinConfig) validate() error {
	if p.Path == "" {
		return fmt.Errorf("spot_baseline.path: %w", ErrConfigFieldUnset)
	}
	if p.SHA256 == "" {
		return fmt.Errorf("spot_baseline.sha256: %w", ErrConfigFieldUnset)
	}
	raw, err := hex.DecodeString(p.SHA256)
	if err != nil || len(raw) != sha256.Size {
		return fmt.Errorf("spot_baseline.sha256 %q is not a 32-byte hex digest: %w",
			p.SHA256, ErrConfigFieldInvalid)
	}
	return nil
}

// Dialect names a venue's REST family, and it selects FOUR collaborators at once.
//
// # One field rather than four, and the collapse is a safety property
//
// A signed call needs a codec, a signer, a rate-limit header reader and a
// response-envelope reader, and all four are venue-specific. Configuring them
// independently would make a Binance signer with a Bybit envelope expressible —
// and SignedClientConfig already records what that costs: on Bybit the body is
// where a refusal is stated at all (HTTP 200 + retCode), so a client with the
// wrong envelope reader classifies every Bybit rejection as an ACCEPTANCE. That
// is an order this system believes was refused and the venue believes it holds.
//
// So the four are chosen as a set, by one closed enumeration, and a document
// cannot express the mixture at all.
type Dialect string

const (
	// DialectBinance is Binance's REST discipline: HMAC over the query string,
	// X-MBX-USED-WEIGHT-* consumption headers, {"code","msg"} error bodies.
	DialectBinance Dialect = "binance"

	// DialectBybit is Bybit's: HMAC over timestamp+key+recvWindow+payload,
	// X-Bapi-Limit-Status REMAINING headers, {"retCode","retMsg"} envelopes on a
	// 200.
	DialectBybit Dialect = "bybit"
)

// Validate reports whether d names a dialect.
func (d Dialect) Validate() error {
	switch d {
	case DialectBinance, DialectBybit:
		return nil
	}
	return fmt.Errorf("dialect %q: %w", string(d), ErrDialectUnknown)
}

// PostureDialect is which ACCOUNT MODEL §4.1's rule table is keyed on.
//
// # A REST dialect and an account model are not the same thing
//
// This field selects a signer, a codec and two readers — all properties of the
// wire. risk/domain's Dialect selects a rule table, which is a property of the
// account: Binance's classic segregated account and its Portfolio Margin
// account speak the SAME REST dialect and require different postures. So the
// mapping is stated rather than assumed, and it is stated here, in the root
// that already owns the four-collaborators-from-one-name collapse.
//
// # Mapping binance → binance-classic is not circular
//
// It looks like it assumes the answer the posture is about to check. It does
// not: the binance-classic table's Portfolio-Margin row is precisely what
// refuses an account that turns out to be the other model. Choosing the classic
// table for a PM account produces a REFUSAL, not a wrong pass — the rules are
// asserted against the model we require, and the gate's whole job is to find
// out we are not on it.
//
// An unknown dialect is an error and never a default, on ErrDialectUnknown's
// standing argument: a default table would judge an account model nobody
// selected.
// postureFamilies is which product families §4.1's posture read rides on this
// dialect, for a deployment whose roster has the given coverage.
//
// # Why this is a CONFIG question at all (record §73n F20)
//
// A posture probe is routed by product family, and the transport map holds one
// entry per CONFIGURED endpoint — so a family this deployment declares no
// endpoint for makes the probe unissuable and §4.1's gate permanently
// unanswerable. That is §5.2's own "a value that makes the permitted region
// unreachable is refused at startup, by name", reached through an omission
// rather than through a value.
//
// # It is the coverage-narrowed set, not the union (task #199, record §73p)
//
// The first cut took the union of every step's family, which is the answer to a
// question nobody asked: it demanded a futures endpoint from a deployment
// holding only the spot leg of a cross-venue carry, whose rules have nothing to
// say about a perpetual and whose boot would then depend on a futures API it
// never calls. A family is required here iff some step ANSWERS a setting whose
// rule is live under this coverage.
//
// The narrowing is not a Binance exemption, and the two venues are the proof.
// Every one of Binance's five probed rules is SubjectDerivativesLeg, so a
// no-derivatives deployment there requires no family at all and still enforces
// Portfolio-Margin, which is attested and needs no endpoint. On Bybit,
// /v5/account/info answers three SubjectAccount settings, so a spot-only
// deployment there still requires the linear-perp family that call is routed to.
// §73k F14's "Bybit is not symmetric — answerable regardless" was true of the
// exchange and false of this routing; what makes the two differ is the rule
// table, not the venue.
func (d Dialect) postureFamilies(v ids.Venue, coverage riskdomain.RosterCoverage) ([]ids.ProductCategory, error) {
	steps, dialect, err := d.postureSteps(v)
	if err != nil {
		return nil, err
	}
	live, err := dialect.ProbedSettings(coverage)
	if err != nil {
		return nil, err
	}
	var families []ids.ProductCategory
	for _, s := range steps {
		answersLive, err := stepAnswersAny(s, live)
		if err != nil {
			return nil, err
		}
		if !answersLive {
			continue
		}
		if !slices.Contains(families, s.Call.Family) {
			families = append(families, s.Call.Family)
		}
	}
	return families, nil
}

// postureSteps returns the dialect's encoded posture steps beside the risk
// table they are checked against, validating each step on the way out.
func (d Dialect) postureSteps(v ids.Venue) ([]venue.PostureStep, riskdomain.Dialect, error) {
	codec, err := orderCodecFor(d, v)
	if err != nil {
		return nil, "", err
	}
	posture, ok := codec.(venue.AccountPostureCodec)
	if !ok {
		// §4.1 makes the posture a precondition of trading, so a dialect whose
		// codec cannot answer it is not one this system may be configured for.
		return nil, "", fmt.Errorf("dialect %q has no account posture surface: %w",
			string(d), ErrDialectUnknown)
	}
	steps, err := posture.EncodePosture()
	if err != nil {
		return nil, "", err
	}
	for _, s := range steps {
		if err := s.Validate(); err != nil {
			return nil, "", err
		}
	}
	dialect, err := d.PostureDialect()
	if err != nil {
		return nil, "", err
	}
	return steps, dialect, nil
}

// stepAnswersAny reports whether the step establishes any of the given domain
// settings, translating through the ACL that already owns the two vocabularies.
func stepAnswersAny(s venue.PostureStep, settings []riskdomain.Setting) (bool, error) {
	for _, a := range s.Answers {
		translated, err := riskdriven.TranslateSetting(a)
		if err != nil {
			return false, fmt.Errorf("posture step %s: %w", s.Probe, err)
		}
		if slices.Contains(settings, translated) {
			return true, nil
		}
	}
	return false, nil
}

// validatePostureCoverage refuses a dialect whose rule table names a probed
// setting that no step answers.
//
// # The check #186 wrote for fold names, one surface over
//
// The rule table and the codec's step list are two statements about one thing —
// which settings this system can establish — written in different packages and
// edited months apart. A rule with no probe can never be answered, so it fails
// on every boot with ReasonUnobserved: a permanent refusal whose cause is in
// neither file's own tests, because each half is internally consistent.
//
// It runs against RosterCoverageDerivatives rather than this deployment's own
// coverage on purpose. The question is whether the TABLES agree, which is a
// property of the code and not of one operator's config, and the maximal
// coverage is the only one that reaches every rule. A deployment-specific run
// would let a missing probe hide behind a lapsed rule until the day somebody
// configures a perpetual.
func (d Dialect) validatePostureCoverage(v ids.Venue) error {
	steps, dialect, err := d.postureSteps(v)
	if err != nil {
		return err
	}
	return postureCoverageAgrees(steps, dialect, maximalPostureCoverage)
}

// maximalPostureCoverage is the coverage the agreement check runs at.
//
// A named constant rather than a literal argument, because it is a CLAIM and
// this is what lets it be one: a check run at a narrower coverage passes
// vacuously for every rule that coverage lapses, and on Binance the narrow
// coverage lapses all five — so the whole check would evaporate while still
// running, returning nil, and looking exactly as it does now. With the value
// named and postureCoverageAgrees taking it as a parameter, the two answers are
// separable and a test can show they differ (task #199, record §73p).
const maximalPostureCoverage = riskdomain.RosterCoverageDerivatives

// postureCoverageAgrees is validatePostureCoverage's pure core: does this step
// list answer every setting the table probes at this coverage?
func postureCoverageAgrees(steps []venue.PostureStep, dialect riskdomain.Dialect, coverage riskdomain.RosterCoverage) error {
	probed, err := dialect.ProbedSettings(coverage)
	if err != nil {
		return err
	}
	answered := make(map[riskdomain.Setting]struct{}, len(probed))
	for _, s := range steps {
		for _, a := range s.Answers {
			translated, err := riskdriven.TranslateSetting(a)
			if err != nil {
				return fmt.Errorf("posture step %s: %w", s.Probe, err)
			}
			answered[translated] = struct{}{}
		}
	}
	for _, want := range probed {
		if _, ok := answered[want]; !ok {
			return fmt.Errorf("§4.1's %s table requires %s at %s and no posture probe answers it: %w",
				dialect, want, coverage, ErrPostureSettingUnprobed)
		}
	}
	return nil
}

func (d Dialect) PostureDialect() (riskdomain.Dialect, error) {
	switch d {
	case DialectBinance:
		return riskdomain.DialectBinanceClassic, nil
	case DialectBybit:
		return riskdomain.DialectBybitUTA, nil
	}
	return "", fmt.Errorf("dialect %q has no account posture model: %w", string(d), ErrDialectUnknown)
}

// VenueConfig is §5.2's per-venue block: "endpoints, key refs (never inline
// secrets), IP-allowlist expectations, region, subaccount id".
//
// Three of those six are here. The subaccount id is Config.Account, one level up,
// because this deployment has one account and it is what fills fill records
// rather than what addresses a host. IP-allowlist expectations and region have no
// consumer in a reconcile-only boot and are not declared — see Config.
type VenueConfig struct {
	// Venue is the exchange identity every instrument, scope and clock
	// calibration in this document must agree with.
	Venue ids.Venue `json:"venue"`

	// Dialect is which REST discipline this venue speaks. See Dialect.
	Dialect Dialect `json:"dialect"`

	// APIKeyRef and APISecretRef are §5.2's "key refs (never inline secrets)".
	// config.SecretRef cannot hold a secret, so a pasted credential is a
	// document that does not parse rather than a policy somebody follows.
	APIKeyRef    config.SecretRef `json:"api_key_ref"`
	APISecretRef config.SecretRef `json:"api_secret_ref"`

	// SpendLockDir carries the one-spender namespace for this credentialed root.
	// It is required beside the refs it protects; all lock-aware processes on one
	// host must share it or they deliberately do not share the guard.
	SpendLockDir string `json:"spend_lock_dir"`

	// Endpoints is one entry per product family, and an unconfigured family is
	// refused rather than defaulted anywhere below. On Binance the families are
	// different HOSTS, and "BTCUSDT" exists on both: a spot order sent to the
	// futures host is a valid, executable order on the wrong instrument.
	Endpoints []EndpointConfig `json:"endpoints"`

	// Timeout bounds one request, ReadLimit caps a response body, and RecvWindow
	// is how long the venue keeps honouring a delayed request.
	//
	// Timeout is a safety parameter rather than a responsiveness one — a signed
	// call that times out is an EffectUnknown that costs a resolving query and
	// blocks new trading — and Resolution.MaxLanding must sit above it and above
	// RecvWindow, which Validate checks.
	Timeout    config.Duration `json:"timeout"`
	ReadLimit  int64           `json:"read_limit"`
	RecvWindow config.Duration `json:"recv_window"`

	// Limits are §7.6's budgets and ThrottlePause/BanPause are its hard pauses.
	// This is the venue's own metering, stated as data because platform/ratelimit
	// deliberately holds no venue numbers.
	Limits        []LimitConfig   `json:"limits"`
	ThrottlePause config.Duration `json:"throttle_pause"`
	BanPause      config.Duration `json:"ban_pause"`

	// Uncounted is §7.6's written escape from the coverage rule: each entry
	// declares that one family deliberately budgets nothing for one dimension,
	// and why. The rule itself — every family budgets weight and orders or
	// writes down the reason it does not, and the venue request envelope is
	// never waivable — lives in ratelimit.Profile.Validate; this block only
	// carries the operator's words to it.
	Uncounted []UncountedConfig `json:"uncounted,omitempty"`

	// ClockOffset and ClockUncertainty calibrate this venue's clock frame against
	// the engine's — §5.3's anchor is recorded in the VENUE's frame with its
	// uncertainty band, because the retention cliff is evaluated by the venue on
	// the venue's arithmetic.
	//
	// Stated rather than measured, and the omission is deliberate rather than
	// hidden: a boot that calibrated from the venue's own server-time endpoint
	// would be a venue call before the journal has been replayed, which §3.2's
	// ordering forbids for a reason. A zero offset with an honest uncertainty is
	// the conservative reading, because both of §5.3's bounds take the band edge
	// that makes them refuse SOONER — so a band that is too wide costs refusals
	// and never a false proof.
	ClockOffset      config.Duration `json:"clock_offset"`
	ClockUncertainty config.Duration `json:"clock_uncertainty"`
}

// EndpointConfig is one product family's signed REST target.
type EndpointConfig struct {
	Product ids.ProductCategory `json:"product"`
	Scheme  string              `json:"scheme"`
	Host    string              `json:"host"`
}

// LimitConfig is one §7.6 budget, in the venue's own units.
//
// The three ceilings are absolute counts and are stated in the document rather
// than derived from Capacity. §7.6's "back off at ~70%" is a fraction of a count,
// and a fraction of a count is a rounding decision — every one of those in this
// repo is an explicit argument, so the operator who knows the venue's published
// capacity does the arithmetic once, in the open, in the file.
type LimitConfig struct {
	// Name is what a refusal and a §10 metric label say. Binance family-scoped
	// weight and orders names are fixed as weight_<window> and orders_<window>,
	// where the limit's own interval is spelled 10s/1m/1h/1d style; transport
	// refuses any mismatch. For endpoint scope it is both the fold address and
	// the request discriminator derived from the call path.
	Name string `json:"name"`

	// Scope is "venue" for §7.6's shared IP-ban envelope or "family" for one
	// product family's own budget. "endpoint" is one endpoint's venue-level
	// window, shared across product categories. Product is required for family
	// scope and refused for venue and endpoint scope.
	//
	// Where a venue publishes unequal category caps for one endpoint, the
	// ceilings use the minimum across the categories this profile enables. A
	// larger figure admits a burst the stricter category refuses. This stays an
	// operator rule: platform holds no venue numbers, and upgraded accounts can
	// legitimately have different caps.
	Scope   string              `json:"scope"`
	Product ids.ProductCategory `json:"product,omitempty"`

	// Dimension is "weight", "orders" or "requests". A profile that budgets no
	// order-count dimension charges literally nothing for placing a Binance
	// futures order, which is documented at zero weight — §7.6's own worked
	// example of a governor that counts nothing and therefore admits everything.
	Dimension string `json:"dimension"`

	// Interval is the window, applied as a sliding one.
	Interval config.Duration `json:"interval"`

	// OpeningCeiling, RoutineCeiling and Capacity are §7.6's ladder, in this
	// limit's units, non-decreasing.
	OpeningCeiling uint64 `json:"opening_ceiling"`
	RoutineCeiling uint64 `json:"routine_ceiling"`
	Capacity       uint64 `json:"capacity"`
}

// UncountedConfig is one written §7.6 waiver: this product family deliberately
// budgets nothing for this dimension, for this reason. See VenueConfig.Uncounted.
type UncountedConfig struct {
	Product   ids.ProductCategory `json:"product"`
	Dimension string              `json:"dimension"`
	Reason    string              `json:"reason"`
}

// InstrumentConfig is one instrument and its asset decomposition.
type InstrumentConfig struct {
	Venue    ids.Venue           `json:"venue"`
	Product  ids.ProductCategory `json:"product"`
	Contract ids.ContractType    `json:"contract,omitempty"`
	Symbol   string              `json:"symbol"`

	// Base and Quote are what the venue's own strings are tagged with. A wrong
	// split produces a Money whose asset tag is a lie, which fixedpoint's runtime
	// check then enforces against everything else and fails far from here.
	Base  fixedpoint.Asset `json:"base"`
	Quote fixedpoint.Asset `json:"quote"`

	// Leverage is §4.1's operator-declared leverage pin, as a whole multiplier.
	// REQUIRED on a derivative instrument, REFUSED on a spot one.
	//
	// # It lives on the instrument row rather than in a posture roster
	//
	// §4.1's per-symbol checks are answered over the CONFIGURED roster rather
	// than over the venue's returned rows (record §73b F4), so something has to
	// say which symbols those are. A separate `posture.symbols` block would be a
	// second list beside this one, and the disagreement that matters between
	// them is the silent one: an instrument this deployment trades that the
	// posture block does not name is a symbol traded without its margin mode,
	// position mode or leverage ever being checked. Deriving the roster from the
	// instruments makes that unrepresentable, which is the same argument
	// risk/domain's SymbolPin makes about the roster and the pins being one
	// list — here applied one level out.
	//
	// It is NOT the argument PositionScopes rejects. That enumeration is
	// deliberately independent because it exists to find exposure on
	// instruments this system never heard of, so a scope derived from the
	// configured set could not widen past it. This roster asks the opposite
	// question — "was every symbol we will trade verified" — where the derived
	// set is exactly the right one and cannot omit a traded symbol.
	//
	// The pin itself is not derivable and is stated here or nowhere: a pin read
	// from the venue at first boot compares the venue against itself and is
	// self-consistent under every mutation, so it cannot fail on the one boot
	// that would catch an account configured wrong before this system touched it
	// (record §73b F3).
	Leverage int `json:"leverage,omitempty"`
}

// Instrument is the identity this row names.
func (c InstrumentConfig) Instrument() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: c.Venue, Product: c.Product, Contract: c.Contract, Symbol: c.Symbol,
	}
}

// ScopeConfig is one position enumeration §7.5's invariant is asserted over.
type ScopeConfig struct {
	Venue   ids.Venue           `json:"venue"`
	Product ids.ProductCategory `json:"product"`

	// Settle is the settlement asset. Required even where a venue ignores it:
	// Bybit's enumeration requires a settleCoin when no symbol is given, and an
	// optional field would fail at run time for a reader of Binance's docs.
	Settle fixedpoint.Asset `json:"settle"`
}

// Scope is the port value this row names.
func (c ScopeConfig) Scope() application.PositionScope {
	return application.PositionScope{Venue: c.Venue, Product: c.Product, Settle: c.Settle}
}

// ResolutionConfig is §5.3's local policy.
type ResolutionConfig struct {
	// MaxLanding is the propagation bound and SweepMargin is how far past it the
	// executions sweep must prove coverage. Two fields rather than a sum, because
	// they answer different normative sentences.
	MaxLanding  config.Duration `json:"max_landing"`
	SweepMargin config.Duration `json:"sweep_margin"`

	// AssumedReach supplies gate 4's bound for a surface whose venue publishes
	// none — Binance spot's order endpoint states no retention at all, and
	// NEITHER default is available: "no cliff" writes an evidence class we do not
	// have into a terminal state, and "never provable" halts the spot leg of every
	// cash-and-carry trade for ever. So the assumption is an operator's and is
	// stated here or nowhere.
	AssumedReach []ReachConfig `json:"assumed_reach,omitempty"`
}

// ReachConfig is one surface's assumed retention.
type ReachConfig struct {
	Venue   ids.Venue           `json:"venue"`
	Product ids.ProductCategory `json:"product"`
	Reach   config.Duration     `json:"reach"`
}

// Validate reports whether the document describes a bootable reconcile.
//
// # Every refusal names the field, and the cross-document ones are the point
//
// The per-field checks below are what platform/config would force anyway. What
// this method adds is the four checks NO constructor can make, because each is
// about two fields agreeing:
//
//   - every instrument and every scope names the configured venue, so a
//     one-client deployment cannot be handed a two-venue table;
//   - every instrument's product family has an endpoint, so an unroutable family
//     is a refusal at boot rather than at the first query;
//   - every derivative instrument is covered by a scope, so the net-delta
//     invariant cannot be asserted over a subset nobody chose (see
//     application.UnassertedScopeNotEnumerated for what it costs when it is);
//   - MaxLanding sits above the transport bounds it is supposed to dominate.
func (c Config) Validate() error {
	if c.JournalPath == "" {
		return fmt.Errorf("journal_path: %w", ErrConfigFieldUnset)
	}
	if c.Account == "" {
		return fmt.Errorf("account: %w", ErrConfigFieldUnset)
	}
	if err := c.Venue.validate(); err != nil {
		return err
	}
	if len(c.Instruments) == 0 {
		return fmt.Errorf("instruments: %w", ErrConfigFieldUnset)
	}
	routed := make(map[ids.ProductCategory]struct{}, len(c.Venue.Endpoints))
	for _, e := range c.Venue.Endpoints {
		routed[e.Product] = struct{}{}
	}
	seen := make(map[ids.VenueInstrument]struct{}, len(c.Instruments))
	for i, in := range c.Instruments {
		if err := in.validate(); err != nil {
			return fmt.Errorf("instruments[%d]: %w", i, err)
		}
		if in.Venue != c.Venue.Venue {
			return fmt.Errorf("instruments[%d] names %s and the configured venue is %s: %w",
				i, in.Venue, c.Venue.Venue, ErrVenueMismatch)
		}
		if _, ok := routed[in.Product]; !ok {
			return fmt.Errorf("instruments[%d] is on %s and no endpoint serves it: %w",
				i, in.Product, ErrFamilyUnrouted)
		}
		if _, dup := seen[in.Instrument()]; dup {
			return fmt.Errorf("instruments[%d] repeats %s: %w", i, in.Instrument(), ErrConfigDuplicate)
		}
		seen[in.Instrument()] = struct{}{}
	}

	covered := make(map[scopeKey]struct{}, len(c.PositionScopes))
	for i, s := range c.PositionScopes {
		if err := s.Scope().Validate(); err != nil {
			return fmt.Errorf("position_scopes[%d]: %w", i, err)
		}
		if s.Venue != c.Venue.Venue {
			return fmt.Errorf("position_scopes[%d] names %s and the configured venue is %s: %w",
				i, s.Venue, c.Venue.Venue, ErrVenueMismatch)
		}
		if _, dup := covered[scopeKey{s.Venue, s.Product}]; dup {
			return fmt.Errorf("position_scopes[%d] repeats %s/%s: %w", i, s.Venue, s.Product, ErrConfigDuplicate)
		}
		covered[scopeKey{s.Venue, s.Product}] = struct{}{}
	}
	hasSpot := false
	for i, in := range c.Instruments {
		if !in.Product.IsPerp() {
			// No venue publishes a position surface for spot, so no scope could
			// ever cover it. The spot leg is asserted against a recorded
			// BASELINE instead (task #179, record §70), or reported
			// UnassertedBaselineUnrecorded — a coverage check that lives at
			// assert time, in the report, because the baseline is deliberately
			// optional here (see Config.SpotBaseline).
			hasSpot = true
			continue
		}
		if _, ok := covered[scopeKey{in.Venue, in.Product}]; !ok {
			return fmt.Errorf("instruments[%d] %s has no position_scopes entry: %w",
				i, in.Instrument(), ErrScopeNotCovered)
		}
	}

	if err := c.Resolution.validate(c.Venue); err != nil {
		return err
	}
	if c.SpotBaseline != nil {
		if err := c.SpotBaseline.validate(); err != nil {
			return err
		}
	}
	// The settle bound is required exactly when a spot instrument exists: the
	// record protocol and the boot's second look both wait it, and a spot
	// deployment without one would meet its first spot delta with an unwired
	// bracket — a failure at the moment that needs the bracket most. A
	// derivatives-only deployment carrying the field is refused too: a bound
	// nothing waits is a claim nobody checks.
	if hasSpot && c.SpotSettle.Duration() <= 0 {
		return fmt.Errorf("spot_settle: %w", ErrConfigFieldUnset)
	}
	if !hasSpot && c.SpotSettle.Duration() != 0 {
		return fmt.Errorf("spot_settle configured with no spot instrument: %w", ErrConfigFieldInvalid)
	}
	if !hasSpot && c.SpotBaseline != nil {
		return fmt.Errorf("spot_baseline configured with no spot instrument: %w", ErrConfigFieldInvalid)
	}
	if err := c.validatePosture(); err != nil {
		return err
	}
	if c.WatchdogSilence.Duration() <= 0 {
		return fmt.Errorf("watchdog_silence: %w", ErrConfigFieldUnset)
	}
	if c.StopBudget.Duration() <= 0 {
		return fmt.Errorf("stop_budget: %w", ErrConfigFieldUnset)
	}
	return nil
}

// validatePosture is §4.1's half of the document: whether a requirement can be
// derived from it at all, and whether this account model reads an attestation.
//
// # A deployment with no derivative instrument is VALID, and that is a finding
//
// The first cut of this refused one, on the argument that a cash-and-carry
// system's reconcile boot exists to establish that a carry can be traded. The
// argument does not survive §5.2's own schema: the venue block is SINGULAR — "a
// deployment trading two venues runs two of these, against two journals" — so
// the spot half of a cross-venue carry is a document with no perpetual in it,
// and refusing it would refuse a shape this project is actively building toward.
// A narrowing carries an argument, and that one was mine rather than the spec's
// (record §73k F11).
//
// So the empty roster is passed to risk/domain as a STATED fact instead, which
// is why RosterCoverage exists. What lapses on such a deployment is the
// per-symbol half of §4.1; the account-wide rules still run, and two of Bybit's
// (spot-margin mode, per-coin borrow) are about the spot leg specifically.
func (c Config) validatePosture() error {
	// Derived here and passed rather than re-derived by the caller: the roster
	// and the carrier must come from ONE read of the instrument table, or the
	// pair can disagree about a document neither of them changed.
	req, err := c.PostureRequirement(riskdomain.AttestationUnstated)
	if err != nil {
		return fmt.Errorf("§4.1 posture requirement: %w", err)
	}
	// Whether the two tables agree is a property of the CODE, so it is asked
	// before anything about this document: a rule no probe answers refuses every
	// boot of every config, and diagnosing that from a document is the wrong
	// file to be reading.
	if err := c.Venue.Dialect.validatePostureCoverage(c.Venue.Venue); err != nil {
		return err
	}
	// Every family the posture read RIDES must have an endpoint, or the gate is
	// unanswerable on every boot of this document (record §73n F20). It is
	// checked here rather than discovered at the probe, because a config-time
	// refusal names the line to add while an unrouted probe at boot arrives as
	// "the posture could not be established" — the retryable arm, for a
	// deployment no retry will ever change. The instrument→endpoint rule above
	// cannot see this: these families are the POSTURE's, and on the ordinary
	// deployment it reads two of them for instruments it does not trade.
	//
	// Narrowed by the requirement's OWN coverage rather than by a second walk of
	// c.Instruments (task #199): the roster and the families must come from one
	// derivation, or a document neither of them changed can be refused by one
	// and admitted by the other.
	families, err := c.Venue.Dialect.postureFamilies(c.Venue.Venue, req.Coverage)
	if err != nil {
		return err
	}
	routed := make(map[ids.ProductCategory]struct{}, len(c.Venue.Endpoints))
	for _, e := range c.Venue.Endpoints {
		routed[e.Product] = struct{}{}
	}
	for _, f := range families {
		if _, ok := routed[f]; !ok {
			return fmt.Errorf("§4.1's posture read rides %s and no endpoint serves it: %w",
				f, ErrFamilyUnrouted)
		}
	}
	if c.PortfolioMarginAttestation == nil {
		return nil
	}
	if err := c.PortfolioMarginAttestation.validate(); err != nil {
		return err
	}
	// Asked of the DIALECT's own rule table rather than restated as "binance
	// only": the table is where the Portfolio-Margin row lives, so a second
	// account model that grows one is covered without an edit here, and one
	// that loses it stops accepting the pin without an edit either.
	dialect, err := c.Venue.Dialect.PostureDialect()
	if err != nil {
		return err
	}
	if !dialect.ConsumesAttestation() {
		return fmt.Errorf("portfolio_margin_attestation pinned on a %s account, whose posture has no check that reads it: %w",
			dialect, ErrConfigFieldInvalid)
	}
	return nil
}

// scopeKey is the (venue, family) pair a scope covers. Settle is deliberately not
// in it: two scopes differing only by settlement asset cover the same family, and
// a key including it would let a document declare USDT and USDC for one family and
// read as two coverages of nothing.
type scopeKey struct {
	venue   ids.Venue
	product ids.ProductCategory
}

func (v VenueConfig) validate() error {
	if err := v.Venue.Validate(); err != nil {
		return fmt.Errorf("venue.venue: %w", err)
	}
	if err := v.Dialect.Validate(); err != nil {
		return fmt.Errorf("venue.dialect: %w", err)
	}
	if err := v.APIKeyRef.Validate(); err != nil {
		return fmt.Errorf("venue.api_key_ref: %w", err)
	}
	if err := v.APISecretRef.Validate(); err != nil {
		return fmt.Errorf("venue.api_secret_ref: %w", err)
	}
	if v.SpendLockDir == "" {
		return fmt.Errorf("venue.spend_lock_dir: %w", ErrConfigFieldUnset)
	}
	if len(v.Endpoints) == 0 {
		return fmt.Errorf("venue.endpoints: %w", ErrConfigFieldUnset)
	}
	seen := make(map[ids.ProductCategory]struct{}, len(v.Endpoints))
	for i, e := range v.Endpoints {
		if err := e.Product.Validate(); err != nil {
			return fmt.Errorf("venue.endpoints[%d].product: %w", i, err)
		}
		if e.Scheme != "http" && e.Scheme != "https" {
			return fmt.Errorf("venue.endpoints[%d].scheme %q: %w", i, e.Scheme, ErrConfigFieldInvalid)
		}
		if e.Host == "" {
			return fmt.Errorf("venue.endpoints[%d].host: %w", i, ErrConfigFieldUnset)
		}
		if _, dup := seen[e.Product]; dup {
			return fmt.Errorf("venue.endpoints[%d] repeats %s: %w", i, e.Product, ErrConfigDuplicate)
		}
		seen[e.Product] = struct{}{}
	}
	if v.Timeout.Duration() <= 0 {
		return fmt.Errorf("venue.timeout: %w", ErrConfigFieldUnset)
	}
	if v.ReadLimit <= 0 {
		return fmt.Errorf("venue.read_limit: %w", ErrConfigFieldUnset)
	}
	if v.RecvWindow.Duration() <= 0 {
		return fmt.Errorf("venue.recv_window: %w", ErrConfigFieldUnset)
	}
	if len(v.Limits) == 0 {
		return fmt.Errorf("venue.limits: %w", ErrConfigFieldUnset)
	}
	if v.ThrottlePause.Duration() <= 0 {
		return fmt.Errorf("venue.throttle_pause: %w", ErrConfigFieldUnset)
	}
	if v.BanPause.Duration() <= 0 {
		return fmt.Errorf("venue.ban_pause: %w", ErrConfigFieldUnset)
	}
	if v.ClockUncertainty.Duration() < 0 {
		return fmt.Errorf("venue.clock_uncertainty: %w", ErrConfigFieldInvalid)
	}
	// The limits themselves are validated by ratelimit.Profile, which is where
	// the ladder's ordering, the scope/family agreement and §7.6's
	// count-what-you-charge rule live. Restating them here would be the copy that
	// drifts; what this file owns is turning the document's strings into that
	// profile's enums, and an unrecognised one is refused there.
	if _, err := v.profile(); err != nil {
		return fmt.Errorf("venue.limits: %w", err)
	}
	return nil
}

func (c InstrumentConfig) validate() error {
	if err := c.Instrument().Validate(); err != nil {
		return err
	}
	if err := (venue.InstrumentAssets{Base: c.Base, Quote: c.Quote}).Validate(); err != nil {
		return err
	}
	// §3.6 rule 4 in its most literal form: an unpinned leverage reads as "any
	// leverage is acceptable", which computes the safety gate in our favour with
	// nothing failing. risk/domain refuses a non-positive pin too — this is the
	// same claim made where the operator can see which LINE of their file is
	// missing it, rather than as a requirement they never typed.
	if c.Product.IsPerp() && c.Leverage <= 0 {
		return fmt.Errorf("leverage pin %d on a derivative instrument: %w", c.Leverage, ErrConfigFieldUnset)
	}
	// A pin on a spot row is refused rather than ignored, on #179's precedent:
	// no §4.1 rule is scoped to a spot symbol, so an operator who wrote one
	// believes a check is running that is not.
	if !c.Product.IsPerp() && c.Leverage != 0 {
		return fmt.Errorf("leverage pin %d on a spot instrument, which no posture rule reads: %w",
			c.Leverage, ErrConfigFieldInvalid)
	}
	return nil
}

// PostureRequirement builds §4.1's requirement for this deployment, given the
// attestation the root loaded (or domain.AttestationUnstated where no document
// is pinned).
//
// The roster is DERIVED from the derivative instruments — see
// InstrumentConfig.Leverage for why that direction is the safe one here and the
// opposite of PositionScopes'. Spot rows are excluded because no §4.1 rule is
// scoped to a spot symbol; §7.5 asserts the spot leg against a recorded
// baseline instead.
//
// It returns a VALIDATED requirement or an error, so a caller cannot hand the
// guard one that would evaluate nothing.
func (c Config) PostureRequirement(attested riskdomain.Attestation) (riskdomain.Requirement, error) {
	dialect, err := c.Venue.Dialect.PostureDialect()
	if err != nil {
		return riskdomain.Requirement{}, err
	}
	var symbols []riskdomain.SymbolPin
	for _, in := range c.Instruments {
		if !in.Product.IsPerp() {
			continue
		}
		symbols = append(symbols, riskdomain.SymbolPin{Symbol: in.Symbol, Leverage: in.Leverage})
	}
	// The carrier comes from the SAME walk that built the roster, so the two
	// cannot describe different documents. Stating it from `len(symbols)` is
	// what makes it a fact about this config rather than a flag somebody set:
	// the reading it rules out — an empty roster meaning "forgotten" — is not
	// expressible here, because the roster is derived rather than typed.
	coverage := riskdomain.RosterCoverageDerivatives
	if len(symbols) == 0 {
		coverage = riskdomain.RosterCoverageNoDerivatives
	}
	req := riskdomain.Requirement{
		Venue:           c.Venue.Venue,
		Dialect:         dialect,
		Coverage:        coverage,
		Symbols:         symbols,
		PortfolioMargin: attested,
	}
	if err := req.Validate(); err != nil {
		return riskdomain.Requirement{}, err
	}
	return req, nil
}

func (r ResolutionConfig) validate(v VenueConfig) error {
	if r.MaxLanding.Duration() <= 0 {
		return fmt.Errorf("resolution.max_landing: %w", ErrConfigFieldUnset)
	}
	if r.SweepMargin.Duration() <= 0 {
		return fmt.Errorf("resolution.sweep_margin: %w", ErrConfigFieldUnset)
	}
	// §5.3: MaxLanding is how long after we commit to a Place the venue may still
	// accept it, and what bounds that at the transport is the request timeout with
	// recvWindow beneath it. "Set it ABOVE both, never below" is the sentence, and
	// below it the landing horizon expires while a submission can still land — so
	// the stable-not-found rule proves an absence that a live order is about to
	// contradict. Checked here because it is the one rule in this document that
	// spans two blocks, and neither block's own validator can see the other.
	if r.MaxLanding.Duration() <= v.Timeout.Duration() {
		return fmt.Errorf("resolution.max_landing %s is not above venue.timeout %s: %w",
			r.MaxLanding, v.Timeout, ErrLandingBoundTooShort)
	}
	if r.MaxLanding.Duration() <= v.RecvWindow.Duration() {
		return fmt.Errorf("resolution.max_landing %s is not above venue.recv_window %s: %w",
			r.MaxLanding, v.RecvWindow, ErrLandingBoundTooShort)
	}
	seen := make(map[application.SurfaceKey]struct{}, len(r.AssumedReach))
	for i, a := range r.AssumedReach {
		if err := a.Venue.Validate(); err != nil {
			return fmt.Errorf("resolution.assumed_reach[%d].venue: %w", i, err)
		}
		if err := a.Product.Validate(); err != nil {
			return fmt.Errorf("resolution.assumed_reach[%d].product: %w", i, err)
		}
		if a.Reach.Duration() <= 0 {
			return fmt.Errorf("resolution.assumed_reach[%d].reach: %w", i, ErrConfigFieldUnset)
		}
		key := application.SurfaceKey{Venue: a.Venue, Product: a.Product}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("resolution.assumed_reach[%d] repeats %s/%s: %w",
				i, a.Venue, a.Product, ErrConfigDuplicate)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// policy is the §5.3 policy this document states.
func (c Config) policy() application.ResolutionPolicy {
	reach := make(map[application.SurfaceKey]time.Duration, len(c.Resolution.AssumedReach))
	for _, a := range c.Resolution.AssumedReach {
		reach[application.SurfaceKey{Venue: a.Venue, Product: a.Product}] = a.Reach.Duration()
	}
	return application.ResolutionPolicy{
		MaxLanding:   c.Resolution.MaxLanding.Duration(),
		SweepMargin:  c.Resolution.SweepMargin.Duration(),
		AssumedReach: reach,
	}
}

// assets is the instrument table oms.Deps takes.
func (c Config) assets() map[ids.VenueInstrument]venue.InstrumentAssets {
	out := make(map[ids.VenueInstrument]venue.InstrumentAssets, len(c.Instruments))
	for _, in := range c.Instruments {
		out[in.Instrument()] = venue.InstrumentAssets{Base: in.Base, Quote: in.Quote}
	}
	return out
}

// scopes is the enumeration list oms.Deps takes.
func (c Config) scopes() []application.PositionScope {
	out := make([]application.PositionScope, 0, len(c.PositionScopes))
	for _, s := range c.PositionScopes {
		out = append(out, s.Scope())
	}
	return out
}

// Calibrations is the venue clock frame §5.3's anchor is stamped in.
//
// Exported because the process CLOCK must exist before Run is called and must
// already carry it: §5.3's anchor is recorded in the venue's own frame with its
// uncertainty band, and a Wall built without this table answers ServerNow for an
// uncalibrated venue — which is a claim of perfect knowledge about a clock nobody
// measured. One Wall per process, so the calibration cannot be added later.
func (c Config) Calibrations() []clock.Calibration {
	return []clock.Calibration{{
		Venue:       clock.Venue(c.Venue.Venue),
		Offset:      c.Venue.ClockOffset.Duration(),
		Uncertainty: c.Venue.ClockUncertainty.Duration(),
	}}
}

// profile turns the document's limit rows into §7.6's profile.
//
// The string→enum step is where an unrecognised scope or dimension is refused,
// and refusing is the only available answer: a limit whose dimension did not
// parse would either be dropped — a budget silently not counted, which is §7.6's
// own worked failure — or counted as something else.
func (v VenueConfig) profile() (ratelimit.Profile, error) {
	families := make([]ids.ProductCategory, 0, len(v.Endpoints))
	for _, e := range v.Endpoints {
		families = append(families, e.Product)
	}
	limits := make([]ratelimit.Limit, 0, len(v.Limits))
	for i, l := range v.Limits {
		scope, err := parseScope(l.Scope)
		if err != nil {
			return ratelimit.Profile{}, fmt.Errorf("[%d].scope: %w", i, err)
		}
		dimension, err := parseDimension(l.Dimension)
		if err != nil {
			return ratelimit.Profile{}, fmt.Errorf("[%d].dimension: %w", i, err)
		}
		limits = append(limits, ratelimit.Limit{
			Name:           l.Name,
			Scope:          scope,
			Family:         l.Product,
			Dimension:      dimension,
			Interval:       l.Interval.Duration(),
			OpeningCeiling: l.OpeningCeiling,
			RoutineCeiling: l.RoutineCeiling,
			Capacity:       l.Capacity,
		})
	}
	uncounted := make([]ratelimit.Uncounted, 0, len(v.Uncounted))
	for i, u := range v.Uncounted {
		dimension, err := parseDimension(u.Dimension)
		if err != nil {
			return ratelimit.Profile{}, fmt.Errorf("uncounted[%d].dimension: %w", i, err)
		}
		uncounted = append(uncounted, ratelimit.Uncounted{
			Family:    u.Product,
			Dimension: dimension,
			Reason:    u.Reason,
		})
	}
	p := ratelimit.Profile{
		Venue:         v.Venue,
		Families:      families,
		Limits:        limits,
		Uncounted:     uncounted,
		ThrottlePause: v.ThrottlePause.Duration(),
		BanPause:      v.BanPause.Duration(),
	}
	if err := p.Validate(); err != nil {
		return ratelimit.Profile{}, err
	}
	return p, nil
}

func parseScope(s string) (ratelimit.Scope, error) {
	switch s {
	case "venue":
		return ratelimit.ScopeVenue, nil
	case "family":
		return ratelimit.ScopeFamily, nil
	case "endpoint":
		return ratelimit.ScopeEndpoint, nil
	}
	return ratelimit.ScopeUnspecified,
		fmt.Errorf("%q (want \"venue\", \"family\" or \"endpoint\"): %w", s, ErrConfigFieldInvalid)
}

func parseDimension(s string) (ratelimit.Dimension, error) {
	switch s {
	case "weight":
		return ratelimit.DimensionWeight, nil
	case "orders":
		return ratelimit.DimensionOrders, nil
	case "requests":
		return ratelimit.DimensionRequests, nil
	}
	return ratelimit.DimensionUnspecified,
		fmt.Errorf("%q (want \"weight\", \"orders\" or \"requests\"): %w", s, ErrConfigFieldInvalid)
}
