package ids

import (
	"fmt"
	"strings"
	"unicode"
)

// Sentinel errors for the instrument value objects. Callers branch on these with
// errors.Is, so they are part of the package's public contract.
var (
	// ErrProductUnset reports an empty ProductCategory — an unpopulated field
	// rather than a wrong one.
	ErrProductUnset = fmt.Errorf("ids: product category unset")

	// ErrProductUnknown reports a non-empty value outside the declared set.
	// Distinct from unset on purpose: unset is a wiring mistake, unknown is
	// either a typo in config or a venue product we do not support, and the two
	// send an operator to different places.
	ErrProductUnknown = fmt.Errorf("ids: unknown product category")

	// ErrSymbolUnset reports an empty symbol.
	ErrSymbolUnset = fmt.Errorf("ids: symbol unset")

	// ErrSymbolSpace reports a symbol carrying whitespace. See Symbol's note on
	// VenueInstrument for why this is rejected rather than trimmed.
	ErrSymbolSpace = fmt.Errorf("ids: symbol contains whitespace")

	// ErrContractUnset reports a derivative instrument with no ContractType.
	ErrContractUnset = fmt.Errorf("ids: contract type unset")

	// ErrContractUnknown reports a non-empty ContractType outside the declared
	// set.
	ErrContractUnknown = fmt.Errorf("ids: unknown contract type")

	// ErrContractOnSpot reports a spot instrument carrying a ContractType.
	// Spot has none, and permitting a decorative one would give a spot
	// instrument three spellings — three mailboxes for one instrument (§3.4).
	ErrContractOnSpot = fmt.Errorf("ids: spot instrument carries a contract type")

	// ErrContractMismatch reports a ProductCategory and a ContractType that
	// disagree about linear-vs-inverse. This is the error the redundant
	// encoding exists to make impossible; see VenueInstrument.Validate.
	ErrContractMismatch = fmt.Errorf("ids: contract type disagrees with product category")
)

// ProductCategory names the venue product family an instrument belongs to.
//
// §3.6 rule 8's `type X string` shape rather than fixedpoint.RoundingMode's
// integer one, and the deciding question is rule 8's own: does the value cross a
// persistence or wire boundary? It does. §5.3's fill dedup key is
// UNIQUE(account, venue, product_category, symbol, execution_id) — the token is
// stored, so a self-describing one is worth its width, and the constants below
// are schema, not labels.
//
// The set is deliberately NOT open the way Venue is. Venue comes from
// configuration and grows without code changes; a product family does not — each
// one implies different sizing, different margin semantics and a different REST
// surface, so a value nobody wrote code for must be rejected rather than passed
// through.
type ProductCategory string

const (
	// ProductSpot is the long leg of the cash-and-carry (§2.2). No funding, no
	// mark price, no liquidation.
	ProductSpot ProductCategory = "spot"

	// ProductLinearPerp is a USDT-margined perpetual — the short leg, and the
	// only derivative the MVP trades (§12.2's universe is linear BTC).
	// Quantity is in the base asset and margin is in the quote.
	ProductLinearPerp ProductCategory = "linear_perp"

	// ProductInversePerp is a coin-margined perpetual. No venue adapter trades
	// one in the MVP, and it is declared anyway: without it ContractType has a
	// single usable value, VenueInstrument's agreement rule has nothing to be
	// wrong about, and "this contract is linear" is an assumption rather than a
	// checked claim. Quantity is in USD and margin is in the base coin, which
	// is why mislabelling one as linear is not a cosmetic error.
	ProductInversePerp ProductCategory = "inverse_perp"
)

// Validate reports whether p is one of the declared values.
func (p ProductCategory) Validate() error {
	switch p {
	case ProductSpot, ProductLinearPerp, ProductInversePerp:
		return nil
	case "":
		return ErrProductUnset
	}
	return fmt.Errorf("%q: %w", string(p), ErrProductUnknown)
}

func (p ProductCategory) String() string { return string(p) }

// IsPerp reports whether p is a perpetual future.
//
// Nearly every caller that asks about the product family wants this rather than
// the specific value: funding accrues on perps only, and mark price and
// liquidation distance (§6, §7.1) exist only for them.
//
// An undeclared value answers false. That direction is chosen: a caller asking
// "is this a perp" is usually about to reach for a mark price or a funding rate,
// and answering yes for a value we do not recognise invents both. Answering no
// makes it visible as a missing instrument instead.
func (p ProductCategory) IsPerp() bool {
	return p == ProductLinearPerp || p == ProductInversePerp
}

// ContractType distinguishes linear from inverse settlement.
//
// Same `type X string` reasoning as ProductCategory: §5.1 puts it in SymbolSpec,
// which is persisted reference data.
type ContractType string

const (
	// ContractLinear settles in the quote asset. Quantity is in the base asset,
	// so notional is quantity × price.
	ContractLinear ContractType = "linear"

	// ContractInverse settles in the base asset. Quantity is a USD amount and
	// notional is quantity ÷ price — the inverted arithmetic that makes a
	// mislabelled contract wrong by a factor of the price squared, not by a
	// rounding step.
	ContractInverse ContractType = "inverse"
)

// Validate reports whether c is one of the declared values.
func (c ContractType) Validate() error {
	switch c {
	case ContractLinear, ContractInverse:
		return nil
	case "":
		return ErrContractUnset
	}
	return fmt.Errorf("%q: %w", string(c), ErrContractUnknown)
}

func (c ContractType) String() string { return string(c) }

// VenueInstrument is §5.1's canonical identity: "for routing, market-data keys,
// orders, fills, reference calls, and recovery cursors" `[→ Z1]`.
//
// # Spot BTCUSDT and perp BTCUSDT are different instruments
//
// That is the whole reason Product is in the identity. The two have independent
// order and execution-ID namespaces, separate REST endpoints, and different fee
// tiers, and §5.3's fill dedup key names product_category as its own column for
// exactly this reason. An identity that collapsed them would dedup two real fills
// into one, and a dropped fill is a position the ledger does not know about.
//
// # It is a map key, so there is exactly one spelling of each instrument
//
// §3.4 gives every VenueInstrument its own market-data mailbox. Two spellings of
// one instrument are two mailboxes, and a consumer reading one of them sees half
// its updates with nothing failing. Validate is what enforces the canonical form:
// no whitespace in the symbol, and a ContractType present exactly when the
// product is a derivative.
//
// Struct fields are exported for repository hydration, as §5.1 specifies for its
// aggregate types.
//
// # The linear/inverse distinction is encoded twice, and Validate is why that is safe
//
// §5.1 declares both a ProductCategory and a ContractType here, so ProductLinearPerp
// already implies ContractLinear. A redundant encoding is only safe while
// something forbids the copies from disagreeing — otherwise the struct can hold a
// state no instrument has, and the two consumers that read different fields
// compute different notionals from the same value. Validate rejects the
// disagreement, which turns the redundancy from a hazard into a checksum.
//
// (Recorded rather than silently simplified: dropping ContractType from this
// struct and deriving it from Product would remove the redundancy outright. That
// is a spec change, not an implementation choice, so it is not taken here —
// ContractType still has to exist for SymbolSpec, which has no Product field.)
type VenueInstrument struct {
	Venue   Venue
	Product ProductCategory

	// Symbol is the venue's own routing symbol — "BTCUSDT", not the display-only
	// VenueSymbol of §5.1.
	//
	// It is neither case-folded nor trimmed here, and both omissions are
	// deliberate. Folding case would be a claim about venues rather than about
	// this package: the two in docs/venue-profiles.md uppercase, but "BTC-PERPETUAL"
	// and lowercase-symbol venues exist, and a fold that corrupts a venue's real
	// symbol fails at order submission. Trimming would silently accept a
	// config-file artefact and make " BTCUSDT" a synonym — the one-spelling rule
	// forbids synonyms. So whitespace is rejected, loudly, at the boundary where
	// somebody can still fix the config.
	Symbol string

	// Contract is set exactly when Product is a derivative, and unset for spot.
	Contract ContractType
}

// Validate reports whether the instrument is one that could exist.
//
// Field order matters here, and it is the order a reader would debug in: the
// venue, then the product family, then the symbol, then the contract agreement
// that depends on the product already being known. A caller sees the outermost
// mistake first rather than a downstream consequence of it.
func (vi VenueInstrument) Validate() error {
	if err := vi.Venue.Validate(); err != nil {
		return err
	}
	if err := vi.Product.Validate(); err != nil {
		return err
	}
	if vi.Symbol == "" {
		return ErrSymbolUnset
	}
	if strings.ContainsFunc(vi.Symbol, unicode.IsSpace) {
		return fmt.Errorf("%q: %w", vi.Symbol, ErrSymbolSpace)
	}
	return vi.validateContract()
}

// validateContract holds the agreement rule of the type comment.
//
// Split out because it is the only part of Validate with a real decision in it,
// and because the spot branch and the derivative branch reject for genuinely
// different reasons — spot has no contract type to give, a perp has one it must
// state correctly.
func (vi VenueInstrument) validateContract() error {
	if !vi.Product.IsPerp() {
		if vi.Contract != "" {
			return fmt.Errorf("%s carries %q: %w", vi.Product, vi.Contract, ErrContractOnSpot)
		}
		return nil
	}
	if err := vi.Contract.Validate(); err != nil {
		return err
	}

	// Reached only for a declared product and a declared contract type, so this
	// is the disagreement check and nothing else.
	if want := vi.Product.contractType(); vi.Contract != want {
		return fmt.Errorf("%s with %q, want %q: %w",
			vi.Product, vi.Contract, want, ErrContractMismatch)
	}
	return nil
}

// contractType is the settlement each declared perp product implies.
//
// It answers only for perps — the empty return for spot is not a third contract
// type, it is "the question does not apply", and validateContract never asks it
// on that branch. Kept as a method next to the constants so that adding a product
// family puts the answer here rather than leaving a silently-wrong default at the
// one call site.
func (p ProductCategory) contractType() ContractType {
	switch p {
	case ProductSpot:
		// "The question does not apply", named rather than left to the trailing
		// return (§3.6 rule 10). Spot has no settlement, which is a different
		// fact from the unset product the trailing return still answers for —
		// this enum is string-backed, so its zero value is "" and is not a
		// declared member.
		return ""
	case ProductLinearPerp:
		return ContractLinear
	case ProductInversePerp:
		return ContractInverse
	}
	return ""
}

// String renders the instrument for logs and error messages.
//
// It is NOT claimed to be injective: a venue symbol containing the separator
// would collide, and inventing a symbol charset to prevent that would over-fit
// this type to venues docs/venue-profiles.md has not verified. Nothing depends on
// the text being unambiguous — the identity is the struct, which is comparable
// and is used directly as a map key. If a parseable form is ever needed it should
// be a real codec with its own round-trip test, not this.
//
// The contract type appears only where it is meaningful, which is exactly where
// Validate requires it, so a spot instrument does not render a trailing empty
// field that reads like missing data.
func (vi VenueInstrument) String() string {
	if vi == (VenueInstrument{}) {
		return "instrument(unset)"
	}
	s := fmt.Sprintf("%s:%s:%s", vi.Venue, vi.Product, vi.Symbol)
	if vi.Contract != "" {
		s += ":" + vi.Contract.String()
	}
	return s
}
