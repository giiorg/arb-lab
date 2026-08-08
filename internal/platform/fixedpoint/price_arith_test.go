package fixedpoint_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Price.Sub and Price.Div landed with §7.1's liq distance (task #210), which is
// `(liqPrice − mark) / mark` and had no operations to express it: Price carried
// Cmp, Sign and OnTick, so the only way to subtract two prices was to leave the
// type. These are Money.Sub and Money.Div's twins on the price axis, and the
// mirror rule is why they are tested against the same shapes rather than against
// the happy path they were added for.

func TestPriceSubKeepsThePairAndTheSign(t *testing.T) {
	t.Parallel()

	// The liq-distance shapes, both directions, and a crossing. The third is the
	// one a defensive Abs() would destroy: a SHORT whose mark has passed its
	// liquidation price is the top of §7.1's ladder, and it is spelled by a
	// negative difference.
	for _, tc := range []struct {
		name     string
		a, b     string
		want     string
		wantSign int
	}{
		{"liq above mark — a short with headroom", "120", "100", "20", 1},
		{"liq below mark — a short past liquidation", "95", "100", "-5", -1},
		{"equal", "100", "100", "0", 0},
		{"fractional", "43250.5", "43000.25", "250.25", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := mustPrice(t, btc, usdt, tc.a).Sub(mustPrice(t, btc, usdt, tc.b))
			if err != nil {
				t.Fatalf("Sub: %v", err)
			}
			if got.Decimal() != tc.want {
				t.Errorf("%s - %s = %s, want %s", tc.a, tc.b, got.Decimal(), tc.want)
			}
			// The pair survives the operation. A difference of two BTC/USDT
			// prices is a BTC/USDT price, and a result that lost its tags would
			// still Decimal() correctly while failing every later Cmp.
			if got.Base() != btc || got.Quote() != usdt {
				t.Errorf("difference is %s/%s, want %s/%s", got.Quote(), got.Base(), usdt, btc)
			}
			sign, err := got.Sign()
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if sign != tc.wantSign {
				t.Errorf("sign = %d, want %d", sign, tc.wantSign)
			}
		})
	}
}

func TestPriceSubAndDivRefuseAMismatchedPair(t *testing.T) {
	t.Parallel()

	// BOTH tags are varied, one at a time, because a check on the quote alone
	// admits a BTC mark into an ETH position's arithmetic — the accident the
	// two-tag Price type exists to catch — and a check on the base alone admits
	// the same symbol quoted in two currencies. A fixture that varied only one
	// would pass against a predicate that read only the other.
	btcUSDT := mustPrice(t, btc, usdt, "100")
	for _, tc := range []struct {
		name  string
		other fixedpoint.Price
	}{
		{"different base", mustPrice(t, eth, usdt, "100")},
		{"different quote", mustPrice(t, btc, eth, "100")},
		{"unset", fixedpoint.Price{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := btcUSDT.Sub(tc.other); err == nil {
				t.Error("Sub accepted a price it cannot subtract")
			}
			if _, err := btcUSDT.Div(tc.other, fixedpoint.RoundFloor); err == nil {
				t.Error("Div accepted a price it cannot divide by")
			}
			// And the mirror: the mismatch is refused whichever operand carries
			// it. A helper that validated only the receiver would pass the block
			// above and fail here.
			if _, err := tc.other.Sub(btcUSDT); err == nil {
				t.Error("Sub accepted a mismatched receiver")
			}
		})
	}
}

func TestPriceDivCancelsBothUnits(t *testing.T) {
	t.Parallel()

	// §7.1's own numbers: a liquidation price 20% above the mark yields 0.2,
	// dimensionless. The gap and the mark are both prices; the quotient is not.
	gap, err := mustPrice(t, btc, usdt, "120").Sub(mustPrice(t, btc, usdt, "100"))
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	d, err := gap.Div(mustPrice(t, btc, usdt, "100"), fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div: %v", err)
	}
	if d.Decimal() != "0.2" {
		t.Errorf("distance = %s, want 0.2", d.Decimal())
	}

	// The denominator is not interchangeable with the numerator's other operand
	// — dividing by the LIQ price instead gives a systematically smaller number
	// for a short, and a policy calibrated on one and evaluated on the other is
	// wrong in the permissive direction. Pinned here so the two are visibly
	// different values rather than "both about 0.2".
	byLiq, err := gap.Div(mustPrice(t, btc, usdt, "120"), fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div: %v", err)
	}
	if byLiq.Decimal() != "0.166666666" {
		t.Errorf("distance over liq price = %s, want 0.166666666", byLiq.Decimal())
	}
}

func TestPriceDivRefusesAZeroDivisorAndOnlyAZeroOne(t *testing.T) {
	t.Parallel()

	// The mark this system would be dividing by. Every layer above already
	// refuses a non-positive one, so reaching here means one of those refusals
	// was bypassed — and any number returned would be a liq distance computed
	// from a mark of nothing.
	_, err := mustPrice(t, btc, usdt, "20").Div(mustPrice(t, btc, usdt, "0"), fixedpoint.RoundFloor)
	if !errors.Is(err, fixedpoint.ErrDivideByZero) {
		t.Fatalf("Div by zero: %v, want ErrDivideByZero", err)
	}

	// # The guard's own contribution, and a mutation is what found it
	//
	// mulDivMantissa ALREADY refuses a zero denominator with this same sentinel,
	// so the explicit check above changes the error MESSAGE and nothing else —
	// `o.p == 0` → `o.p < 0` leaves the zero case behaving identically, and the
	// assertion above cannot tell them apart. What that mutant does change is
	// the NEGATIVE divisor, which it starts refusing.
	//
	// A negative price is legal (Price says so: calendar spreads and some
	// settled contracts trade below zero), so refusing one is a real narrowing
	// — and nothing anywhere in this package divided by a negative value, on
	// Money's side either. These two assertions are what make the guard's
	// boundary a checked claim rather than a redundant one.
	if !strings.Contains(err.Error(), "by zero") {
		t.Errorf("the zero-divisor message is %q and does not name the case — which is the only thing this guard adds over mulDivMantissa's", err)
	}
	got, err := mustPrice(t, btc, usdt, "20").Div(mustPrice(t, btc, usdt, "-100"), fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div by a negative price: %v", err)
	}
	if got.Decimal() != "-0.2" {
		t.Errorf("20 / -100 = %s, want -0.2", got.Decimal())
	}
}

func TestPriceDivRoundsWhereTheModeSaysAndTheModesDisagreeOnNegatives(t *testing.T) {
	t.Parallel()

	// The fixture that splits RoundFloor from RoundTowardZero. They agree on
	// every positive quotient, so a suite built only from healthy positions
	// cannot tell §7.1's chosen mode from the one that looks right — and the
	// case they disagree on is exactly the past-liquidation short, where
	// RoundTowardZero reports the position as marginally LESS breached.
	gap := mustPrice(t, btc, usdt, "-2")
	mark := mustPrice(t, btc, usdt, "3")

	floor, err := gap.Div(mark, fixedpoint.RoundFloor)
	if err != nil {
		t.Fatalf("Div floor: %v", err)
	}
	if floor.Decimal() != "-0.666666667" {
		t.Errorf("RoundFloor = %s, want -0.666666667", floor.Decimal())
	}

	toward, err := gap.Div(mark, fixedpoint.RoundTowardZero)
	if err != nil {
		t.Fatalf("Div toward zero: %v", err)
	}
	if toward.Decimal() != "-0.666666666" {
		t.Errorf("RoundTowardZero = %s, want -0.666666666", toward.Decimal())
	}

	if floor.Decimal() == toward.Decimal() {
		t.Fatal("the two modes agree on this fixture — then no test here can tell them apart")
	}

	// RoundExact still refuses an inexact quotient, which is what makes the
	// mode an argument rather than a default: a caller that has not decided
	// gets an error instead of a silent policy.
	if _, err := gap.Div(mark, fixedpoint.RoundExact); !errors.Is(err, fixedpoint.ErrPrecisionLoss) {
		t.Errorf("RoundExact on an inexact quotient: %v, want ErrPrecisionLoss", err)
	}
	if _, err := gap.Div(mark, fixedpoint.RoundInvalid); err == nil {
		t.Error("Div accepted the zero-value rounding mode")
	}
}

func TestPriceSubOverflows(t *testing.T) {
	t.Parallel()

	// Money.Sub's boundary case, restated for the mirror: the difference leaves
	// the range rather than wrapping.
	if _, err := mustPrice(t, btc, usdt, minDecimal).Sub(mustPrice(t, btc, usdt, ulp)); err == nil {
		t.Error("Sub wrapped past the negative bound")
	}
	// The positive control: one unit inside the bound subtracts cleanly, so the
	// test above is measuring the boundary and not a broken helper.
	got, err := mustPrice(t, btc, usdt, minPlusUlp).Sub(mustPrice(t, btc, usdt, ulp))
	if err != nil {
		t.Fatalf("Sub at the bound: %v", err)
	}
	if got.Decimal() != minDecimal {
		t.Errorf("got %s, want %s", got.Decimal(), minDecimal)
	}
}
