package pair

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// eqQty fails the test unless got and want are the same quantity, INCLUDING
// the asset tag. An error from Cmp is a failure, not a mismatch — it means one
// of the two was never populated, which is the case this repo's fixedpoint
// refuses to let read as zero.
func eqQty(t *testing.T, got, want fixedpoint.Quantity) {
	t.Helper()
	cmp, err := got.Cmp(want)
	if err != nil {
		t.Fatalf("comparing %v and %v: %v", got, want, err)
	}
	if cmp != 0 {
		t.Errorf("quantity = %v, want %v", got, want)
	}
}

// eqPrice is the same for Price.
func eqPrice(t *testing.T, got, want fixedpoint.Price) {
	t.Helper()
	cmp, err := got.Cmp(want)
	if err != nil {
		t.Fatalf("comparing %v and %v: %v", got, want, err)
	}
	if cmp != 0 {
		t.Errorf("price = %v, want %v", got, want)
	}
}

// qty is the fixture's quantity constructor. LAB is the shortlist alt record
// §95 §2 measured trading on both demo venues.
func qty(t *testing.T, s string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("LAB", s)
	if err != nil {
		t.Fatalf("ParseQuantity(LAB, %q): %v", s, err)
	}
	return q
}

// leg builds an ESTABLISHED exposure. Every field that Compose reads is a
// parameter, so no test below can be right for a reason the fixture supplied.
func leg(t *testing.T, venue string, side domain.Side, filled string) LegExposure {
	t.Helper()
	return LegExposure{
		Venue:       ids.Venue(venue),
		ID:          ids.ClientOrderID("ab-01j0000000000000000000000-00-00"),
		Instrument:  ids.VenueInstrument{Venue: ids.Venue(venue), Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "LABUSDT"},
		Side:        side,
		State:       domain.StateFilled,
		Filled:      qty(t, filled),
		Established: true,
	}
}

func TestComposeVerdicts(t *testing.T) {
	t.Parallel()

	unestablished := func(l LegExposure) LegExposure {
		l.Established, l.Because = false, "state is not terminal"
		l.State = domain.StateSubmissionUnknown
		return l
	}

	cases := []struct {
		name string
		a, b LegExposure
		want Verdict
	}{
		{
			// Nothing filled on either side: the IOC-did-not-fill case, which
			// is the LIKELY outcome on a thin demo book and is a real clean run.
			name: "neither side filled",
			a:    leg(t, "binance", domain.SideSell, "0"),
			b:    leg(t, "bybit", domain.SideBuy, "0"),
			want: VerdictUnopened,
		},
		{
			name: "both filled the same quantity",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictIntact,
		},
		{
			// The one the whole slice exists for: A carries exposure, B does not.
			name: "only side A filled",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "0"),
			want: VerdictHalfOpen,
		},
		{
			// The mirror. It must NOT collapse into the case above — a rule that
			// only looked at A would answer HalfOpen for both and pass.
			name: "only side B filled",
			a:    leg(t, "binance", domain.SideSell, "0"),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictHalfOpen,
		},
		{
			// 100 vs 60. NOT 100 vs 50: at one half the remainder equals the
			// filled complement and two different wrong rules agree.
			name: "both filled, unequal",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    leg(t, "bybit", domain.SideBuy, "60"),
			want: VerdictSkewed,
		},
		{
			name: "side A not established",
			a:    unestablished(leg(t, "binance", domain.SideSell, "0")),
			b:    leg(t, "bybit", domain.SideBuy, "100"),
			want: VerdictIndeterminate,
		},
		{
			name: "side B not established",
			a:    leg(t, "binance", domain.SideSell, "100"),
			b:    unestablished(leg(t, "bybit", domain.SideBuy, "0")),
			want: VerdictIndeterminate,
		},
		{
			// Indeterminate OUTRANKS everything, including the reading that
			// would otherwise be Intact. An unestablished leg's zero is not a
			// measurement, and a rule that let the other side's number decide
			// would conclude on it.
			name: "unestablished outranks an otherwise-intact reading",
			a:    unestablished(leg(t, "binance", domain.SideSell, "100")),
			b:    unestablished(leg(t, "bybit", domain.SideBuy, "100")),
			want: VerdictIndeterminate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Compose(tc.a, tc.b)
			if err != nil {
				t.Fatalf("Compose(%s, %s) error = %v, want nil", tc.a.Venue, tc.b.Venue, err)
			}
			if got != tc.want {
				t.Errorf("Compose(%s, %s) = %s, want %s", tc.a.Venue, tc.b.Venue, got, tc.want)
			}
		})
	}
}

// TestComposeRefusesAnUnpopulatedQuantity is why Compose is fallible.
// fixedpoint.Quantity.Sign is fallible for the same reason it states in its own
// doc — "a flat position and a position nobody populated would otherwise be
// indistinguishable" — and here that difference is a naked position reading as
// no position. The fixture uses the ZERO Quantity, which carries no asset tag,
// against a legitimately-flat one that does.
func TestComposeRefusesAnUnpopulatedQuantity(t *testing.T) {
	t.Parallel()
	a := leg(t, "binance", domain.SideSell, "0") // flat, and tagged LAB
	b := leg(t, "bybit", domain.SideBuy, "0")
	b.Filled = fixedpoint.Quantity{} // nobody populated it

	got, err := Compose(a, b)
	if err == nil {
		t.Fatalf("Compose with an unpopulated quantity = %s, nil; want an error", got)
	}
	if got != VerdictIndeterminate {
		t.Errorf("Compose on error = %s, want VerdictIndeterminate — the fail-closed direction", got)
	}
	// The control: the SAME shape with both quantities populated is a clean
	// VerdictUnopened, so the test above is about the tag and not about zero.
	if got, err := Compose(a, leg(t, "bybit", domain.SideBuy, "0")); err != nil || got != VerdictUnopened {
		t.Fatalf("Compose(flat, flat) = %s, %v; want VerdictUnopened, nil", got, err)
	}
}

// TestComposeRefusesTwoLegsOnDifferentBases is the other half of Cmp's
// fallibility, and it is a DIFFERENT fixture from the one above: there both
// quantities are readable and the tags disagree, so Sign passes on each leg and
// only the comparison can refuse. Without it, a Compose that dropped the Cmp
// error would still be caught by nothing — the unpopulated case is refused two
// statements earlier.
func TestComposeRefusesTwoLegsOnDifferentBases(t *testing.T) {
	t.Parallel()
	a := leg(t, "binance", domain.SideSell, "100")
	b := leg(t, "bybit", domain.SideBuy, "100")
	other, err := fixedpoint.ParseQuantity("NOTLAB", "100")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	b.Filled = other

	got, err := Compose(a, b)
	if err == nil {
		t.Fatalf("Compose across two base assets = %s, nil; want an error", got)
	}
	if got != VerdictIndeterminate {
		t.Errorf("Compose on error = %s, want VerdictIndeterminate", got)
	}
}

// TestComposeRefusesSameSideLegs repeats Validate's opposite-side premise at
// the pure boundary. Compose is reached from two modes and can be called by
// code that never loaded Config; assuming the premise here lets two equal buys
// report VerdictIntact.
func TestComposeRefusesSameSideLegs(t *testing.T) {
	t.Parallel()
	a := leg(t, "binance", domain.SideBuy, "100")
	b := leg(t, "bybit", domain.SideBuy, "100")

	got, err := Compose(a, b)
	if !errors.Is(err, ErrLegSideSame) {
		t.Fatalf("Compose(two buys) = %s, %v; want VerdictIndeterminate, %v", got, err, ErrLegSideSame)
	}
	if got != VerdictIndeterminate {
		t.Errorf("Compose(two buys) = %s, want VerdictIndeterminate", got)
	}
}

// TestExposedNamesEveryVerdict is the completeness check the exhaustiveswitch
// gate cannot make: it asserts what each member MEANS for a running system,
// so a member added later has to choose a side here too.
func TestExposedNamesEveryVerdict(t *testing.T) {
	t.Parallel()
	want := map[Verdict]bool{
		VerdictUnopened:      false,
		VerdictIntact:        false,
		VerdictHalfOpen:      true,
		VerdictSkewed:        true,
		VerdictIndeterminate: true,
	}
	for v, w := range want {
		if got := Exposed(v); got != w {
			t.Errorf("Exposed(%s) = %t, want %t", v, got, w)
		}
	}
	if len(want) != 5 {
		t.Fatalf("the Verdict roster has %d members here; add the new one to this table", len(want))
	}
}

// legState is a projection reading with a NON-ZERO cumulative, so that
// "established" and "flat" cannot be the same observation: a reading that
// refused would carry the same zero a leg that filled nothing carries.
func legState(t *testing.T, state domain.OrderState) oms.LegState {
	t.Helper()
	return oms.LegState{
		Instrument: ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Contract: ids.ContractLinear, Symbol: "LABUSDT"},
		Side:       domain.SideSell,
		State:      state,
		Ordered:    qty(t, "100"),
		Cumulative: qty(t, "60"),
		Fills:      1,
	}
}

// TestEstablishIsThreeValued walks D4's reading. Each unfinished-reading fact is
// set ALONE, so "names the first fact" and "names this fact" are different
// answers — a table that set two at once would pass against a switch that only
// ever reported the earliest arm.
func TestEstablishIsThreeValued(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		state   domain.OrderState
		joined  JoinFacts
		want    bool
		because string // a substring unique to the arm that must have fired
	}{
		{
			// The positive control. Without it a predicate that refused
			// everything would score perfectly on the seven rows below.
			name:  "terminal and cleanly joined",
			state: domain.StateFilled,
			want:  true,
		},
		{
			// Terminal is not the same as filled: an IOC's unfilled remainder is
			// canceled, and the cancel is a real established reading of zero.
			name:  "terminal by cancel",
			state: domain.StateCanceled,
			want:  true,
		},
		{
			name:    "not terminal",
			state:   domain.StateSubmitted,
			want:    false,
			because: "not terminal",
		},
		{
			name:    "state the domain cannot classify",
			state:   domain.OrderState("not-a-state"),
			want:    false,
			because: "not-a-state",
		},
		{
			name:    "absent",
			state:   domain.StateFilled,
			joined:  JoinFacts{Absent: true},
			want:    false,
			because: "does not hold this order",
		},
		{
			name:    "incomplete",
			state:   domain.StateFilled,
			joined:  JoinFacts{Incomplete: true},
			want:    false,
			because: "prove coverage",
		},
		{
			name:    "unattributed",
			state:   domain.StateFilled,
			joined:  JoinFacts{Unattributed: true},
			want:    false,
			because: "disagrees with the attributed fills",
		},
		{
			// #242's fourth class, and the one D4 was corrected for. The join
			// latches boot.OutcomeNeedsOperator over it, so reading it as
			// established would conclude a quantity from two authorities that
			// disagree about what happened to the order.
			name:    "contradicted",
			state:   domain.StateFilled,
			joined:  JoinFacts{Contradicted: true},
			want:    false,
			because: "no edge between them",
		},
	}

	id := ids.ClientOrderID("ab-01j0000000000000000000000-00-00")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			st := legState(t, tc.state)
			got := Establish("binance", id, st, tc.joined)

			if got.Established != tc.want {
				t.Fatalf("Established = %t (%s), want %t", got.Established, got.Because, tc.want)
			}
			if tc.want {
				if got.Because != "" {
					t.Errorf("Because = %q on an established leg, want empty", got.Because)
				}
			} else if !strings.Contains(got.Because, tc.because) {
				t.Errorf("Because = %q, want it to name %q", got.Because, tc.because)
			}
			// The carried fields come from the projection whichever way the
			// reading went — the report names the leg it could not establish.
			if got.Venue != "binance" || got.ID != id {
				t.Errorf("identity = %s/%s, want binance/%s", got.Venue, got.ID, id)
			}
			if got.State != tc.state {
				t.Errorf("State = %q, want %q", got.State, tc.state)
			}
			eqQty(t, got.Filled, st.Cumulative)
		})
	}
}

// TestEstablishNamesTheFirstFactWhenSeveralHold pins the ladder's ORDER, which
// a table of one-fact-at-a-time cases cannot see. The order is the switch's own
// and nothing normative rests on it; what this test buys is that a reordering
// is a deliberate edit rather than an invisible one, since the operator report
// prints exactly one reason.
func TestEstablishNamesTheFirstFactWhenSeveralHold(t *testing.T) {
	t.Parallel()
	got := Establish("binance", "ab-01j0000000000000000000000-00-00",
		legState(t, domain.StateFilled),
		JoinFacts{Absent: true, Incomplete: true, Unattributed: true, Contradicted: true})

	if got.Established {
		t.Fatal("Established = true with every join fact set")
	}
	if !strings.Contains(got.Because, "does not hold this order") {
		t.Errorf("Because = %q, want the ABSENT reason — the first rung", got.Because)
	}
}

// TestEstablishNamesEveryJoinFact is what guards a FIFTH finding class, and it
// is reflective for the reason cmd/arblab's own totality test is.
//
// # Why not a backstop in the code
//
// The obvious defence is an `if joined.unfinished()` check below Establish's
// switch. It was written, and it is unreachable while every field has an arm —
// `tools/mutate.sh` replacing it with `if false` SURVIVED, which is the proof
// rather than the suspicion. A branch no test can kill reads as handled and
// checks nothing.
//
// # What this test can see that the table above cannot
//
// The table names four facts because four exist today. A fifth added to
// JoinFacts and forgotten in the switch falls through to ESTABLISHED — a leg
// the join could not finish reading, reported as a measured quantity — and no
// compiler objects, because it is one more bool. This derives the roster from
// the struct, so the new field fails here by name.
//
// # The reasons must be DISTINCT, which is the second half of the claim
//
// Refusing is not enough: two facts sharing one message make the operator's
// report unable to say which reading stopped the run, and an arm copied from
// its neighbour is exactly how that happens.
func TestEstablishNamesEveryJoinFact(t *testing.T) {
	t.Parallel()

	factsType := reflect.TypeOf(JoinFacts{})
	if factsType.NumField() == 0 {
		t.Fatal("JoinFacts has no fields; this test would pass vacuously")
	}

	seen := map[string]string{} // reason -> the field that produced it
	for i := range factsType.NumField() {
		field := factsType.Field(i)
		if field.Type.Kind() != reflect.Bool {
			// Loudly rather than skipped: a finding carried as something richer
			// than a bool is still a finding, and a selector narrower than the
			// claim is how this repo's totality guards have gone stale before.
			t.Errorf("JoinFacts.%s is a %s, not a bool; this test cannot set it — widen it deliberately",
				field.Name, field.Type.Kind())
			continue
		}
		var joined JoinFacts
		reflect.ValueOf(&joined).Elem().Field(i).SetBool(true)

		got := Establish("binance", "ab-01j0000000000000000000000-00-00",
			legState(t, domain.StateFilled), joined)
		if got.Established {
			t.Errorf("Establish with JoinFacts.%s set = established; the switch in Establish has no arm for it",
				field.Name)
			continue
		}
		if got.Because == "" {
			t.Errorf("Establish with JoinFacts.%s set gives no reason", field.Name)
			continue
		}
		if other, dup := seen[got.Because]; dup {
			t.Errorf("JoinFacts.%s and JoinFacts.%s give the same reason %q; the report cannot tell them apart",
				field.Name, other, got.Because)
		}
		seen[got.Because] = field.Name
	}
}
