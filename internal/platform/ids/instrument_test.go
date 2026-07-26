package ids_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// VenueInstrument is §5.1's canonical identity — "for routing, market-data keys,
// orders, fills, reference calls, and recovery cursors". Three claims carry that
// job, and they fail in three different ways:
//
//   - Spot BTCUSDT and perp BTCUSDT are DIFFERENT instruments [→ Z1]. Collapsing
//     them merges two order/execution-ID namespaces that venues keep independent,
//     which makes the §5.3 fill-dedup key wrong and silently drops a real fill.
//   - There is exactly ONE way to spell any instrument. It is a map key — for the
//     §3.4 per-VenueInstrument mailboxes — so two spellings of one instrument are
//     two mailboxes, and a consumer reads half its updates.
//   - Product and ContractType must agree. The struct encodes the linear/inverse
//     distinction twice (§5.1 declares both fields), and an inverse contract
//     labelled linear computes notional in the wrong unit entirely.
//
// The tests below are organised around those three.

// aPerp and aSpot are the two fixtures most cases start from: valid, minimal,
// and differing only where the case under test needs them to.
func aPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:    "binance",
		Product:  ids.ProductLinearPerp,
		Symbol:   "BTCUSDT",
		Contract: ids.ContractLinear,
	}
}

func aSpot() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue:   "binance",
		Product: ids.ProductSpot,
		Symbol:  "BTCUSDT",
	}
}

// TestInstrumentFixturesAreValid is the positive control.
//
// Every rejection case below starts from one of these two and breaks exactly one
// thing. Without this test a Validate that objected to everything would read as a
// perfect score — the failure mode §4d found the hard way.
func TestInstrumentFixturesAreValid(t *testing.T) {
	for _, vi := range []ids.VenueInstrument{aPerp(), aSpot()} {
		if err := vi.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", vi, err)
		}
	}
}

// --- claim 1: spot and perp on one symbol are distinct -----------------------

// TestSpotAndPerpAreDistinctInstruments is [→ Z1] stated as a test.
func TestSpotAndPerpAreDistinctInstruments(t *testing.T) {
	spot, perp := aSpot(), aPerp()

	if spot == perp {
		t.Fatal("spot BTCUSDT == perp BTCUSDT; they have independent order and execution-ID namespaces (§5.1)")
	}

	// The property that actually matters downstream: distinct as MAP KEYS, which
	// is how §3.4 addresses a per-VenueInstrument mailbox. Struct inequality
	// implies this in Go, but the mailboxes are the reason the field is in the
	// identity at all, so the claim is checked where it is relied on.
	box := map[ids.VenueInstrument]string{}
	box[spot] = "spot mailbox"
	box[perp] = "perp mailbox"
	if len(box) != 2 {
		t.Errorf("map[VenueInstrument] holds %d entries for spot+perp BTCUSDT, want 2", len(box))
	}
}

// --- claim 2: one spelling per instrument ------------------------------------

// TestInstrumentRejects covers every way to write an instrument that is not one.
//
// Each case starts from a valid fixture and breaks one field, so a rejection is
// attributable to that field — the isolation lesson §8f paid for, where a
// rejection test whose fixtures failed an EARLIER check could not tell which
// check fired.
func TestInstrumentRejects(t *testing.T) {
	cases := []struct {
		name string
		vi   ids.VenueInstrument
		want error
	}{
		{
			// Delegates to Venue.Validate rather than restating the check, so
			// there is one definition of "a venue is set".
			name: "venue unset",
			vi:   ids.VenueInstrument{Product: ids.ProductSpot, Symbol: "BTCUSDT"},
			want: ids.ErrVenueUnset,
		},
		{
			name: "product unset",
			vi:   ids.VenueInstrument{Venue: "binance", Symbol: "BTCUSDT"},
			want: ids.ErrProductUnset,
		},
		{
			name: "product undeclared",
			vi:   ids.VenueInstrument{Venue: "binance", Product: "option", Symbol: "BTCUSDT"},
			want: ids.ErrProductUnknown,
		},
		{
			name: "symbol unset",
			vi:   ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot},
			want: ids.ErrSymbolUnset,
		},
		{
			// A config file or a hand-edited fixture is where this comes from,
			// and it is the one-spelling claim: " BTCUSDT" and "BTCUSDT" would
			// be two mailboxes for one instrument.
			name: "symbol has leading space",
			vi:   ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: " BTCUSDT"},
			want: ids.ErrSymbolSpace,
		},
		{
			name: "symbol has trailing newline",
			vi:   ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT\n"},
			want: ids.ErrSymbolSpace,
		},
		{
			// Rejected rather than folded — see the note on Symbol in
			// instrument.go. Interior space is not a venue symbol on any venue
			// in docs/venue-profiles.md, and permitting it would let one
			// instrument be spelled two ways.
			name: "symbol has interior space",
			vi:   ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "BTC USDT"},
			want: ids.ErrSymbolSpace,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.vi.Validate()
			if !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// --- claim 3: product and contract type must agree ---------------------------

// TestContractTypeMustAgreeWithProduct is the reason Validate is more than four
// nil checks.
//
// §5.1 gives VenueInstrument both a ProductCategory and a ContractType, so the
// linear/inverse distinction is encoded twice. A redundant encoding is only safe
// while something forbids the two copies from disagreeing: an inverse contract
// carrying ContractLinear sizes in the wrong unit (inverse quantity is USD, margin
// is coin) and every notional computed from it is wrong by the price.
func TestContractTypeMustAgreeWithProduct(t *testing.T) {
	cases := []struct {
		name string
		vi   ids.VenueInstrument
		want error
	}{
		{
			// Spot has no contract type — "linear vs inverse" is a derivatives
			// question. Unset is the ONLY valid spelling, which is what makes
			// the spot instrument canonical as a map key.
			name: "spot carrying a contract type",
			vi: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductSpot,
				Symbol: "BTCUSDT", Contract: ids.ContractLinear,
			},
			want: ids.ErrContractOnSpot,
		},
		{
			name: "perp with no contract type",
			vi: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT",
			},
			want: ids.ErrContractUnset,
		},
		{
			name: "perp with an undeclared contract type",
			vi: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductLinearPerp,
				Symbol: "BTCUSDT", Contract: "quanto",
			},
			want: ids.ErrContractUnknown,
		},
		{
			// The case that motivates the whole rule.
			name: "linear perp labelled inverse",
			vi: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductLinearPerp,
				Symbol: "BTCUSDT", Contract: ids.ContractInverse,
			},
			want: ids.ErrContractMismatch,
		},
		{
			name: "inverse perp labelled linear",
			vi: ids.VenueInstrument{
				Venue: "binance", Product: ids.ProductInversePerp,
				Symbol: "BTCUSD", Contract: ids.ContractLinear,
			},
			want: ids.ErrContractMismatch,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.vi.Validate()
			if !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestInversePerpIsRepresentable is the positive half of the rule above.
//
// No venue adapter trades inverse in the MVP (the §12.2 universe is linear BTC),
// and the constant exists anyway: without it ContractType has one usable value,
// the mismatch rule above has nothing to be wrong about, and "this is a linear
// contract" becomes an assumption rather than a checked claim.
func TestInversePerpIsRepresentable(t *testing.T) {
	vi := ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductInversePerp,
		Symbol: "BTCUSD", Contract: ids.ContractInverse,
	}
	if err := vi.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// --- the declared sets -------------------------------------------------------

// TestProductCategoryDeclaredSet pins the set §3.6 rule 8 asks for.
//
// Both enum-likes are `type X string` rather than fixedpoint.RoundingMode's
// integer shape, because both cross a PERSISTENCE boundary: §5.3's fill dedup key
// is UNIQUE(account, venue, product_category, symbol, execution_id), so the token
// is stored. Rule 8's documented exception does not apply.
func TestProductCategoryDeclaredSet(t *testing.T) {
	declared := []ids.ProductCategory{
		ids.ProductSpot, ids.ProductLinearPerp, ids.ProductInversePerp,
	}
	for _, p := range declared {
		if err := p.Validate(); err != nil {
			t.Errorf("declared %q: Validate() = %v, want nil", p, err)
		}
		if p.String() != string(p) {
			t.Errorf("String() = %q, want %q", p.String(), string(p))
		}
	}

	// The stored tokens are part of the persisted schema: changing one silently
	// orphans every row already written under the old spelling, and the dedup
	// key stops matching, which re-applies fills that were already applied.
	want := []string{"spot", "linear_perp", "inverse_perp"}
	for i, p := range declared {
		if string(p) != want[i] {
			t.Errorf("token %d = %q, want %q — this value is persisted", i, p, want[i])
		}
	}

	if err := ids.ProductCategory("").Validate(); !errors.Is(err, ids.ErrProductUnset) {
		t.Errorf("empty: Validate() = %v, want ErrProductUnset", err)
	}
	if err := ids.ProductCategory("perp").Validate(); !errors.Is(err, ids.ErrProductUnknown) {
		t.Errorf("undeclared: Validate() = %v, want ErrProductUnknown", err)
	}
}

func TestContractTypeDeclaredSet(t *testing.T) {
	declared := []ids.ContractType{ids.ContractLinear, ids.ContractInverse}
	for _, c := range declared {
		if err := c.Validate(); err != nil {
			t.Errorf("declared %q: Validate() = %v, want nil", c, err)
		}
		if c.String() != string(c) {
			t.Errorf("String() = %q, want %q", c.String(), string(c))
		}
	}

	want := []string{"linear", "inverse"}
	for i, c := range declared {
		if string(c) != want[i] {
			t.Errorf("token %d = %q, want %q — this value is persisted", i, c, want[i])
		}
	}

	if err := ids.ContractType("").Validate(); !errors.Is(err, ids.ErrContractUnset) {
		t.Errorf("empty: Validate() = %v, want ErrContractUnset", err)
	}
	if err := ids.ContractType("quanto").Validate(); !errors.Is(err, ids.ErrContractUnknown) {
		t.Errorf("undeclared: Validate() = %v, want ErrContractUnknown", err)
	}
}

// TestIsPerp separates "is this a derivative" from the specific product, because
// nearly every caller that asks wants the former: funding accrues on perps, spot
// has no mark price and no liquidation (§6/§7.1).
func TestIsPerp(t *testing.T) {
	cases := []struct {
		p    ids.ProductCategory
		want bool
	}{
		{ids.ProductSpot, false},
		{ids.ProductLinearPerp, true},
		{ids.ProductInversePerp, true},
		// An undeclared value answers false. A caller asking "is this a perp"
		// is usually about to reach for a mark price or a funding rate, and
		// answering yes for a value we do not recognise invents both.
		{ids.ProductCategory("option"), false},
		{ids.ProductCategory(""), false},
	}
	for _, c := range cases {
		if got := c.p.IsPerp(); got != c.want {
			t.Errorf("ProductCategory(%q).IsPerp() = %v, want %v", c.p, got, c.want)
		}
	}
}

// --- rendering ---------------------------------------------------------------

// TestInstrumentString checks the human-facing rendering.
//
// Note what is NOT claimed: that String is injective. A venue symbol containing
// the separator would break that, and inventing a symbol charset to prevent it
// would over-fit the format to venues we have not verified. The identity is the
// STRUCT — comparable, and used directly as a map key — so nothing depends on the
// text being unambiguous. See the note in instrument.go.
func TestInstrumentString(t *testing.T) {
	if got, want := aSpot().String(), "binance:spot:BTCUSDT"; got != want {
		t.Errorf("spot String() = %q, want %q", got, want)
	}
	// The contract type appears only where it is meaningful, which is exactly
	// where Validate requires it.
	if got, want := aPerp().String(), "binance:linear_perp:BTCUSDT:linear"; got != want {
		t.Errorf("perp String() = %q, want %q", got, want)
	}

	// A zero value must render as something a log reader can act on rather than
	// as an empty string that reads like a missing field.
	if got := (ids.VenueInstrument{}).String(); !strings.Contains(got, "unset") {
		t.Errorf("zero String() = %q, want it to say the instrument is unset", got)
	}
}
