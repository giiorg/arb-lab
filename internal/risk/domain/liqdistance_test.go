package domain_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/risk/domain"
)

// §5.2's `liq_distance` policy and the ladder it defines (task #210).
//
// The fixtures below were built from the collapse checklist BEFORE the mutation
// campaign, and three of them exist only because of it: the [25%, 30%) case
// (without which "band == Clear" and "may open" are the same answer), the pair
// of instruments with DIFFERENT policies (without which "this instrument's
// policy" and "the only policy" are the same value), and the exactly-on-a-
// threshold case (without which `<` and `<=` are the same predicate).

const (
	// §5.2's BTC numbers, verbatim: "open ≥30% / alert <25% / rebalance <20% /
	// emergency <15%".
	btcOpen      = "0.30"
	btcAlert     = "0.25"
	btcRebalance = "0.20"
	btcEmergency = "0.15"
)

func rate(t *testing.T, s string) fixedpoint.Rate {
	t.Helper()
	r, err := fixedpoint.ParseRate(s)
	if err != nil {
		t.Fatalf("ParseRate(%q): %v", s, err)
	}
	return r
}

func price(t *testing.T, s string) fixedpoint.Price {
	t.Helper()
	p, err := fixedpoint.ParsePrice("BTC", "USDT", s)
	if err != nil {
		t.Fatalf("ParsePrice(%q): %v", s, err)
	}
	return p
}

// btcPolicy is §5.2's stated ladder for BTC.
func btcPolicy(t *testing.T) domain.LiqDistancePolicy {
	t.Helper()
	return domain.LiqDistancePolicy{
		Open:      rate(t, btcOpen),
		Alert:     rate(t, btcAlert),
		Rebalance: rate(t, btcRebalance),
		Emergency: rate(t, btcEmergency),
	}
}

// altPolicy is §5.2's "tighter for alts" — every threshold strictly above BTC's,
// so that any distance landing in a different band under the two tables proves
// the lookup read the instrument rather than the only row present.
func altPolicy(t *testing.T) domain.LiqDistancePolicy {
	t.Helper()
	return domain.LiqDistancePolicy{
		Open:      rate(t, "0.50"),
		Alert:     rate(t, "0.45"),
		Rebalance: rate(t, "0.40"),
		Emergency: rate(t, "0.35"),
	}
}

func btcPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

func altPerp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "bybit", Product: ids.ProductLinearPerp,
		Symbol: "SOLUSDT", Contract: ids.ContractLinear,
	}
}

func TestLiqDistanceIsSignedAndDirectional(t *testing.T) {
	t.Parallel()

	// Every case uses the SAME two prices and varies only the side, so nothing
	// but the direction can be producing the difference. Mark 100, liq 120: a
	// short has 20% of headroom and a long is 20% past its liquidation.
	//
	// Without the long rows the side is a collapsed dimension — swapping the two
	// arms of the switch would produce identical output on every short-only
	// fixture, since the arms differ only in operand order.
	for _, tc := range []struct {
		name      string
		side      domain.PositionSide
		mark, liq string
		want      string
		wantIsNeg bool
	}{
		{"short with headroom", domain.PositionSideShort, "100", "120", "0.2", false},
		{"long with headroom", domain.PositionSideLong, "100", "80", "0.2", false},
		{"short past liquidation", domain.PositionSideShort, "100", "80", "-0.2", true},
		{"long past liquidation", domain.PositionSideLong, "100", "120", "-0.2", true},
		// The mark exactly AT the liquidation price. Zero rather than an error:
		// a position is not yet liquidated at the instant it touches, and the
		// band below every threshold is where it belongs.
		{"short exactly at liquidation", domain.PositionSideShort, "100", "100", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, err := domain.LiqDistance(tc.side, price(t, tc.mark), price(t, tc.liq))
			if err != nil {
				t.Fatalf("LiqDistance: %v", err)
			}
			if d.Decimal() != tc.want {
				t.Errorf("distance = %s, want %s", d.Decimal(), tc.want)
			}
			sign, err := d.Sign()
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if (sign < 0) != tc.wantIsNeg {
				t.Errorf("sign = %d, want negative = %v — an Abs() here would make a breached position report growing headroom",
					sign, tc.wantIsNeg)
			}
		})
	}
}

func TestLiqDistanceDividesByTheMarkAndNotTheLiquidationPrice(t *testing.T) {
	t.Parallel()

	// The two denominators give different numbers, and the fixture is chosen so
	// they land in DIFFERENT §5.2 bands: 0.2 is exactly the rebalance threshold
	// (so Clear-of-emergency, on the rebalance boundary) while 0.166… is below
	// it. A policy calibrated on price moves and evaluated against the other
	// denominator is wrong in the permissive direction by that ratio.
	d, err := domain.LiqDistance(domain.PositionSideShort, price(t, "100"), price(t, "120"))
	if err != nil {
		t.Fatalf("LiqDistance: %v", err)
	}
	if d.Decimal() != "0.2" {
		t.Fatalf("distance = %s, want 0.2 (the gap over the MARK)", d.Decimal())
	}
	if d.Decimal() == "0.166666666" {
		t.Error("the distance was taken over the liquidation price")
	}
}

func TestLiqDistanceRoundsDownSoAnInexactQuotientNeverOverstatesHeadroom(t *testing.T) {
	t.Parallel()

	// Mark 3, liq 1: a short 66.6…% past liquidation. RoundFloor and
	// RoundTowardZero agree on every positive distance, so this is the only
	// shape in which the chosen mode is observable — and it is the shape where
	// picking the wrong one understates the severity.
	d, err := domain.LiqDistance(domain.PositionSideShort, price(t, "3"), price(t, "1"))
	if err != nil {
		t.Fatalf("LiqDistance: %v", err)
	}
	if d.Decimal() != "-0.666666667" {
		t.Errorf("distance = %s, want -0.666666667 (RoundFloor); -0.666666666 is RoundTowardZero", d.Decimal())
	}
}

func TestLiqDistanceRefusesInputsNoLadderCanUse(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		side      domain.PositionSide
		mark, liq fixedpoint.Price
		want      error
	}{
		{"unset side", domain.PositionSideUnspecified, price(t, "100"), price(t, "120"), domain.ErrPositionSideInvalid},
		{"undeclared side", domain.PositionSide(9), price(t, "100"), price(t, "120"), domain.ErrPositionSideInvalid},
		// A mark of zero would divide; a negative one inverts the whole ladder
		// while every individual number still validates.
		{"zero mark", domain.PositionSideShort, price(t, "0"), price(t, "120"), domain.ErrLiqMarkNotPositive},
		{"negative mark", domain.PositionSideShort, price(t, "-100"), price(t, "120"), domain.ErrLiqMarkNotPositive},
		// A zero liq price is how an ABSENCE would arrive if a codec read one as
		// a number, and for a short it reads as maximum headroom.
		{"zero liq price", domain.PositionSideShort, price(t, "100"), price(t, "0"), domain.ErrLiqPriceNotPositive},
		{"negative liq price", domain.PositionSideShort, price(t, "100"), price(t, "-120"), domain.ErrLiqPriceNotPositive},
		{"unset mark", domain.PositionSideShort, fixedpoint.Price{}, price(t, "120"), nil},
		{"unset liq price", domain.PositionSideShort, price(t, "100"), fixedpoint.Price{}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.LiqDistance(tc.side, tc.mark, tc.liq)
			if err == nil {
				t.Fatal("accepted an input no ladder can use")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPolicyBandNamesEveryRungAndTheBoundariesAreExclusive(t *testing.T) {
	t.Parallel()

	p := btcPolicy(t)

	// Each rung has its own row AND each threshold has a row exactly ON it, so
	// the strict comparison §5.2 writes ("alert <25%") is distinguishable from
	// the inclusive one. Both readings put a distance of 0.25 somewhere; they
	// disagree about where, and only these rows can tell them apart.
	//
	// The rungs are also each reachable independently, which is what pins the
	// ORDER the thresholds are tested in: a loop that asked about Alert first
	// would report a 0.10 distance as Alert rather than Emergency.
	for _, tc := range []struct {
		name string
		d    string
		want domain.LiqBand
	}{
		{"far above every threshold", "1.5", domain.LiqBandClear},
		{"at the open threshold", btcOpen, domain.LiqBandClear},
		{"between open and alert", "0.27", domain.LiqBandClear},
		{"exactly at the alert threshold", btcAlert, domain.LiqBandClear},
		{"just below the alert threshold", "0.249999999", domain.LiqBandAlert},
		{"exactly at the rebalance threshold", btcRebalance, domain.LiqBandAlert},
		{"just below the rebalance threshold", "0.199999999", domain.LiqBandRebalance},
		{"exactly at the emergency threshold", btcEmergency, domain.LiqBandRebalance},
		{"just below the emergency threshold", "0.149999999", domain.LiqBandEmergency},
		{"zero", "0", domain.LiqBandEmergency},
		// The past-liquidation short. It lands on the TOP rung rather than off
		// the ladder, which is why the sign is preserved upstream.
		{"negative — already breached", "-0.4", domain.LiqBandEmergency},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			band, err := p.Band(rate(t, tc.d))
			if err != nil {
				t.Fatalf("Band: %v", err)
			}
			if band != tc.want {
				t.Errorf("Band(%s) = %s, want %s", tc.d, band, tc.want)
			}
		})
	}
}

func TestPolicyOpeningGateIsASecondAxisAndNotDerivableFromTheBand(t *testing.T) {
	t.Parallel()

	p := btcPolicy(t)

	// THE fixture the two-function design exists for. In [25%, 30%) the band is
	// Clear and openings are refused — the region a four-member enum deletes.
	// Without this row, "PermitsOpening == (band == Clear)" holds on every case
	// and the second axis is unobservable.
	for _, tc := range []struct {
		name      string
		d         string
		wantBand  domain.LiqBand
		wantOpens bool
	}{
		{"above the open threshold", "0.35", domain.LiqBandClear, true},
		{"exactly at the open threshold", btcOpen, domain.LiqBandClear, true},
		{"below open, above alert — clear but closed", "0.27", domain.LiqBandClear, false},
		{"exactly at the alert threshold — still clear, still closed", btcAlert, domain.LiqBandClear, false},
		{"alerting", "0.22", domain.LiqBandAlert, false},
		{"emergency", "0.05", domain.LiqBandEmergency, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := rate(t, tc.d)
			band, err := p.Band(d)
			if err != nil {
				t.Fatalf("Band: %v", err)
			}
			opens, err := p.PermitsOpening(d)
			if err != nil {
				t.Fatalf("PermitsOpening: %v", err)
			}
			if band != tc.wantBand {
				t.Errorf("Band(%s) = %s, want %s", tc.d, band, tc.wantBand)
			}
			if opens != tc.wantOpens {
				t.Errorf("PermitsOpening(%s) = %v, want %v", tc.d, opens, tc.wantOpens)
			}
		})
	}

	// And the axes are stated to be independent rather than left to be inferred
	// from the table: there is a distance at which they disagree.
	d := rate(t, "0.27")
	band, err := p.Band(d)
	if err != nil {
		t.Fatalf("Band: %v", err)
	}
	opens, err := p.PermitsOpening(d)
	if err != nil {
		t.Fatalf("PermitsOpening: %v", err)
	}
	if band == domain.LiqBandClear && opens {
		t.Fatal("no distance in this fixture separates the band from the opening gate — then the second axis is untested")
	}
}

func TestPolicyValidateRefusesAnUnoccupiableRung(t *testing.T) {
	t.Parallel()

	base := btcPolicy(t)

	// One violated pair per case, so each comparison is separately demonstrated
	// — a fixture that broke the whole ordering at once would pass against a
	// validator checking any single pair.
	for _, tc := range []struct {
		name   string
		mutate func(domain.LiqDistancePolicy) domain.LiqDistancePolicy
		want   error
	}{
		{"open equals alert", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Open = p.Alert
			return p
		}, domain.ErrLiqThresholdsNotDescending},
		{"alert below rebalance", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Alert, p.Rebalance = rate(t, "0.20"), rate(t, "0.25")
			return p
		}, domain.ErrLiqThresholdsNotDescending},
		{"rebalance equals emergency", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Rebalance = p.Emergency
			return p
		}, domain.ErrLiqThresholdsNotDescending},
		{"emergency zero", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Emergency = rate(t, "0")
			return p
		}, domain.ErrLiqThresholdNotPositive},
		{"open negative", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Open = rate(t, "-0.30")
			return p
		}, domain.ErrLiqThresholdNotPositive},
		{"alert unset", func(p domain.LiqDistancePolicy) domain.LiqDistancePolicy {
			p.Alert = fixedpoint.Rate{}
			return p
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := tc.mutate(base)
			err := p.Validate()
			if err == nil {
				t.Fatal("accepted a policy with a rung no distance can occupy")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
			// Band and PermitsOpening validate first, so a broken policy cannot
			// silently answer. Without this the refusal is a claim about
			// Validate alone and the two entry points are unguarded.
			if _, err := p.Band(rate(t, "0.22")); err == nil {
				t.Error("Band answered from a policy that does not validate")
			}
			if _, err := p.PermitsOpening(rate(t, "0.22")); err == nil {
				t.Error("PermitsOpening answered from a policy that does not validate")
			}
		})
	}

	// The positive control: §5.2's own numbers pass, so the block above is
	// measuring the ordering rule and not a universally-objecting validator.
	if err := base.Validate(); err != nil {
		t.Fatalf("§5.2's stated BTC policy does not validate: %v", err)
	}
	// And a policy above 100% is legal: a short under 1x effective leverage has
	// its liquidation price at more than twice the mark, so capping at 1 would
	// refuse a conservatively-sized deployment at startup.
	wide := domain.LiqDistancePolicy{
		Open: rate(t, "2.0"), Alert: rate(t, "1.5"), Rebalance: rate(t, "1.2"), Emergency: rate(t, "1.1"),
	}
	if err := wide.Validate(); err != nil {
		t.Errorf("a policy with thresholds above 1 was refused: %v", err)
	}
}

func TestZeroPolicyReportsEveryPositionClearAndIsRefused(t *testing.T) {
	t.Parallel()

	// The zero value's specific hazard, stated as a test rather than as a
	// comment: were it admitted, every threshold comparison would be against
	// zero, no distance would ever breach, and the monitor would report clear
	// for ever with nothing failing.
	var p domain.LiqDistancePolicy
	if err := p.Validate(); err == nil {
		t.Fatal("the zero-value policy validates — it would report every position clear for ever")
	}
}

func TestPolicyTableHasNoDefaultRow(t *testing.T) {
	t.Parallel()

	// TWO instruments with DIFFERENT policies, and a distance that lands in
	// different bands under each. With a single-row table, "this instrument's
	// policy" and "the only policy" are the same value and the lookup is
	// untested; with two identical rows, the lookup is tested and the tightening
	// §5.2 requires is not.
	ps := domain.LiqDistancePolicies{
		btcPerp(): btcPolicy(t),
		altPerp(): altPolicy(t),
	}
	if err := ps.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	d := rate(t, "0.38")
	btcBand, err := mustFor(t, ps, btcPerp()).Band(d)
	if err != nil {
		t.Fatalf("Band: %v", err)
	}
	altBand, err := mustFor(t, ps, altPerp()).Band(d)
	if err != nil {
		t.Fatalf("Band: %v", err)
	}
	if btcBand != domain.LiqBandClear {
		t.Errorf("38%% on BTC = %s, want clear", btcBand)
	}
	if altBand != domain.LiqBandRebalance {
		t.Errorf("38%% on the alt = %s, want rebalance — 'tighter for alts' is what the second row is for", altBand)
	}

	// An instrument nobody wrote a row for is REFUSED, never given a default. A
	// fallback here would hand an alt the BTC ladder, which §5.2 says in as many
	// words is too loose for it.
	unlisted := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "ETHUSDT", Contract: ids.ContractLinear,
	}
	if _, err := ps.For(unlisted); !errors.Is(err, domain.ErrLiqPolicyUnconfigured) {
		t.Errorf("For(unlisted) = %v, want ErrLiqPolicyUnconfigured", err)
	}
}

func TestPolicyTableRefusesEmptyAndInvalidRows(t *testing.T) {
	t.Parallel()

	if err := (domain.LiqDistancePolicies{}).Validate(); !errors.Is(err, domain.ErrLiqPoliciesEmpty) {
		t.Errorf("empty table: %v, want ErrLiqPoliciesEmpty", err)
	}
	if err := domain.LiqDistancePolicies(nil).Validate(); !errors.Is(err, domain.ErrLiqPoliciesEmpty) {
		t.Errorf("nil table: %v, want ErrLiqPoliciesEmpty", err)
	}

	// A table whose FIRST row is valid and whose second is not. Map iteration is
	// unordered, so a validator that stopped at the first row would pass this
	// only sometimes — which is why the assertion is on the error and the table
	// carries a good row as well as a bad one.
	bad := btcPolicy(t)
	bad.Rebalance = bad.Emergency
	if err := (domain.LiqDistancePolicies{btcPerp(): btcPolicy(t), altPerp(): bad}).Validate(); err == nil {
		t.Error("a table with an unoccupiable rung in one row validated")
	}

	// And an unusable KEY: a spot instrument would validate as a policy and
	// describes a surface neither venue publishes a liquidation price on.
	broken := domain.LiqDistancePolicies{{Venue: "binance", Symbol: "BTCUSDT"}: btcPolicy(t)}
	if err := broken.Validate(); err == nil {
		t.Error("a table keyed on an invalid instrument validated")
	}
}

func TestLiqBandAndPositionSideValidateTheirMembers(t *testing.T) {
	t.Parallel()

	// LiqBandUnknown PASSES, and that is the asymmetry with every other
	// zero-value rule in this package: it is a real answer with #209's ruling
	// attached, and what makes it safe is that it permits nothing.
	for _, b := range []domain.LiqBand{
		domain.LiqBandUnknown, domain.LiqBandClear, domain.LiqBandAlert,
		domain.LiqBandRebalance, domain.LiqBandEmergency,
	} {
		if err := b.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", b, err)
		}
		if b.String() == "" {
			t.Errorf("band %d renders empty", uint8(b))
		}
	}
	if err := domain.LiqBand(200).Validate(); !errors.Is(err, domain.ErrLiqBandInvalid) {
		t.Errorf("undeclared band: %v, want ErrLiqBandInvalid", err)
	}

	// PositionSide's zero value is REFUSED, and the contrast is the point: an
	// unset side is not an answer, it is a caller that did not say which way its
	// position faces — and either guess is right half the time.
	if err := domain.PositionSideUnspecified.Validate(); !errors.Is(err, domain.ErrPositionSideInvalid) {
		t.Errorf("unset side: %v, want ErrPositionSideInvalid", err)
	}
	for _, s := range []domain.PositionSide{domain.PositionSideShort, domain.PositionSideLong} {
		if err := s.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", s, err)
		}
	}

	// Every member renders distinctly. Two bands sharing a string would collide
	// as metric labels and in every operator message, silently.
	seen := map[string]bool{}
	for _, s := range []string{
		domain.LiqBandUnknown.String(), domain.LiqBandClear.String(), domain.LiqBandAlert.String(),
		domain.LiqBandRebalance.String(), domain.LiqBandEmergency.String(),
	} {
		if seen[s] {
			t.Errorf("two bands render as %q", s)
		}
		seen[s] = true
	}
}

func mustFor(t *testing.T, ps domain.LiqDistancePolicies, vi ids.VenueInstrument) domain.LiqDistancePolicy {
	t.Helper()
	p, err := ps.For(vi)
	if err != nil {
		t.Fatalf("For(%s): %v", vi.Symbol, err)
	}
	return p
}
