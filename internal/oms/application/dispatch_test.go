package application_test

import (
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// # The fakes, and the two properties the checklist demanded of them
//
// Both were chosen before the tests were written, because each closes a
// fixture collapse that would otherwise make several assertions vacuous.
//
//  1. The journal fails a NAMED append rather than every call. A fake that
//     fails everything cannot tell "refuses to place when authorization is not
//     durable" from "reports any journal error", and those are two different
//     claims about two different steps — one a safety property, one an alerting
//     one.
//  2. The placer COUNTS its calls. §5.3's rule is a count, and every other
//     assertion in this file is satisfied by a dispatcher that calls Place
//     twice.

// journalCall is one durable record, in the order it was written.
type journalCall struct {
	id    ids.ClientOrderID
	state domain.OrderState
}

type fakeJournal struct {
	calls   []journalCall
	planned []domain.Order

	// failOn maps a target state to the error its append returns. Keyed by
	// state rather than by call index so a test names the step it is breaking.
	failOn map[domain.OrderState]error
}

func newFakeJournal() *fakeJournal {
	return &fakeJournal{failOn: make(map[domain.OrderState]error)}
}

func (j *fakeJournal) AppendPlanned(o domain.Order) error {
	j.planned = append(j.planned, o)
	return nil
}

func (j *fakeJournal) AppendState(id ids.ClientOrderID, state domain.OrderState) error {
	j.calls = append(j.calls, journalCall{id: id, state: state})
	return j.failOn[state]
}

// states returns what was durably recorded, in order.
func (j *fakeJournal) states() []domain.OrderState {
	out := make([]domain.OrderState, len(j.calls))
	for i, c := range j.calls {
		out[i] = c.state
	}
	return out
}

type fakePlacer struct {
	ack ports.Ack
	err error

	calls  int
	orders []domain.Order

	// watch, when set, is read at the moment Place is entered: appendsBefore
	// records how many journal appends had completed by then. That is the
	// journal-before-network ordering, asserted as a fact about interleaving
	// rather than inferred from the two call counts at the end.
	watch         *fakeJournal
	appendsBefore []int
}

func (p *fakePlacer) Place(o domain.Order) (ports.Ack, error) {
	p.calls++
	p.orders = append(p.orders, o)
	if p.watch != nil {
		p.appendsBefore = append(p.appendsBefore, len(p.watch.calls))
	}
	return p.ack, p.err
}

var _ ports.Journal = (*fakeJournal)(nil)
var _ ports.OrderPlacer = (*fakePlacer)(nil)

// mintedID returns a ClientOrderID this system minted, built through the real
// constructor rather than written as a literal — a hand-written id that stops
// parsing after a format change would silently turn every admit test into a
// test of ErrOrderNotOMSMinted.
func mintedID(t *testing.T, leg int) ids.ClientOrderID {
	t.Helper()
	trade, err := ids.ParseTradeID("018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f")
	if err != nil {
		t.Fatalf("ParseTradeID: %v", err)
	}
	id, err := ids.NewClientOrderID(ids.ClientOrderRef{Trade: trade, Leg: leg})
	if err != nil {
		t.Fatalf("NewClientOrderID: %v", err)
	}
	return id
}

func perp() ids.VenueInstrument {
	return ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductLinearPerp,
		Symbol: "BTCUSDT", Contract: ids.ContractLinear,
	}
}

// dispatchableOrder is §6 step 6's own shape — a marketable-IOC limit — so
// every test below varies exactly the field it is about.
func dispatchableOrder(t *testing.T) domain.Order {
	t.Helper()
	q, err := fixedpoint.ParseQuantity("BTC", "0.5")
	if err != nil {
		t.Fatalf("ParseQuantity: %v", err)
	}
	px, err := fixedpoint.ParsePrice("BTC", "USDT", "50000")
	if err != nil {
		t.Fatalf("ParsePrice: %v", err)
	}
	return domain.Order{
		ClientOrderID: mintedID(t, 1),
		Instrument:    perp(),
		Side:          domain.SideSell,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      q,
		LimitPrice:    px,
	}
}

// newDispatcher builds a dispatcher over fresh fakes, with the placer watching
// the journal so every test gets the ordering assertion for free.
func newDispatcher(t *testing.T) (*application.Dispatcher, *fakeJournal, *fakePlacer) {
	t.Helper()
	j := newFakeJournal()
	p := &fakePlacer{watch: j}
	d, err := application.NewDispatcher(j, p)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	return d, j, p
}

func TestNewDispatcherRefusesAMissingPort(t *testing.T) {
	t.Parallel()

	if _, err := application.NewDispatcher(nil, &fakePlacer{}); !errors.Is(err, application.ErrJournalMissing) {
		t.Errorf("NewDispatcher(nil, placer) = %v, want ErrJournalMissing", err)
	}
	if _, err := application.NewDispatcher(newFakeJournal(), nil); !errors.Is(err, application.ErrPlacerMissing) {
		t.Errorf("NewDispatcher(journal, nil) = %v, want ErrPlacerMissing", err)
	}

	// The positive control. Without it a constructor that refused everything
	// would score two out of two above.
	if _, err := application.NewDispatcher(newFakeJournal(), &fakePlacer{}); err != nil {
		t.Errorf("NewDispatcher with both ports = %v, want nil", err)
	}
}

func TestDispatchPlacesAValidOrderExactlyOnce(t *testing.T) {
	t.Parallel()

	d, j, p := newDispatcher(t)
	p.ack = ports.Ack{VenueOrderID: "V-1"}
	o := dispatchableOrder(t)

	out, err := d.Dispatch(o)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want exactly 1", p.calls)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if out.Ack != p.ack {
		t.Errorf("Ack = %+v, want %+v", out.Ack, p.ack)
	}
	if out.Cause != nil {
		t.Errorf("Cause = %v, want nil on a clean place", out.Cause)
	}
	if !out.Journaled {
		t.Error("Journaled = false, want true")
	}

	// Both records, in order, under the order's own id. The first is the crash
	// boundary; the second is the result.
	wantStates := []domain.OrderState{domain.StateDispatchAuthorized, domain.StateSubmitted}
	if got := j.states(); !equalStates(got, wantStates) {
		t.Errorf("journal recorded %v, want %v", got, wantStates)
	}
	for _, c := range j.calls {
		if c.id != o.ClientOrderID {
			t.Errorf("journal recorded id %q, want %q", c.id, o.ClientOrderID)
		}
	}
	if len(p.orders) == 1 && p.orders[0] != o {
		t.Errorf("Place received %+v, want %+v", p.orders[0], o)
	}
}

// TestDispatchJournalsAuthorizationBeforePlacing asserts the interleaving, not
// the totals.
//
// §6 step 6 fsyncs DispatchAuthorized "immediately before the single Place",
// and the reason is a crash between the two: a send whose authorization is not
// on disk is an order recovery cannot find. A dispatcher that placed first and
// journaled both records afterwards would satisfy every count in the test above.
func TestDispatchJournalsAuthorizationBeforePlacing(t *testing.T) {
	t.Parallel()

	d, j, p := newDispatcher(t)
	if _, err := d.Dispatch(dispatchableOrder(t)); err != nil {
		t.Fatalf("Dispatch = %v", err)
	}
	if len(p.appendsBefore) != 1 {
		t.Fatalf("Place entered %d times, want 1", len(p.appendsBefore))
	}
	if p.appendsBefore[0] != 1 {
		t.Errorf("%d journal appends had completed when Place was entered, want 1",
			p.appendsBefore[0])
	}
	if len(j.calls) == 0 || j.calls[0].state != domain.StateDispatchAuthorized {
		t.Errorf("first journal record = %v, want %s", j.states(), domain.StateDispatchAuthorized)
	}
}

// TestDispatchRefusesBeforeSendingAnything covers admit's three checks.
//
// The type and the time in force vary INDEPENDENTLY: a fixture wrong in both at
// once cannot tell which comparison fired, and a suite of only such fixtures
// would pass against a dispatcher that checked either one alone.
func TestDispatchRefusesBeforeSendingAnything(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*domain.Order)
		want   error
	}{
		{
			name:   "a venue-minted id",
			mutate: func(o *domain.Order) { o.ClientOrderID = "autoclose-1712345678901" },
			want:   application.ErrOrderNotOMSMinted,
		},
		{
			// Right shape, right prefix, right length — and a UUID that is not
			// a v7. This is the id the domain layer's own fixtures use, and it
			// is here to prove the check is ParseClientOrderID rather than a
			// prefix test somebody could write instead.
			name:   "a well-shaped id this system did not mint",
			mutate: func(o *domain.Order) { o.ClientOrderID = "ab-0123456789ABCDEFGHJKMNPQRS-00-00" },
			want:   application.ErrOrderNotOMSMinted,
		},
		{
			name:   "the wrong type, with the right time in force",
			mutate: func(o *domain.Order) { o.Type = domain.OrderTypeMarket; o.LimitPrice = fixedpoint.Price{} },
			want:   application.ErrNotMarketableIOCLimit,
		},
		{
			name:   "the right type, with the wrong time in force",
			mutate: func(o *domain.Order) { o.TimeInForce = domain.TimeInForceGTC },
			want:   application.ErrNotMarketableIOCLimit,
		},
		{
			name:   "another wrong time in force, to show the check is not a GTC test",
			mutate: func(o *domain.Order) { o.TimeInForce = domain.TimeInForceFOK },
			want:   application.ErrNotMarketableIOCLimit,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, j, p := newDispatcher(t)
			o := dispatchableOrder(t)
			tc.mutate(&o)

			out, err := d.Dispatch(o)
			if err == nil {
				t.Fatal("Dispatch = nil, want a refusal")
			}
			if tc.want != domain.ErrOrderUnset && !errors.Is(err, tc.want) {
				t.Errorf("Dispatch = %v, want %v", err, tc.want)
			}
			// The refusal's whole value: nothing left the process and nothing
			// reached the disk, so §5.3's "safe to replan or discard" holds.
			if p.calls != 0 {
				t.Errorf("Place called %d times on a refused order, want 0", p.calls)
			}
			if len(j.calls) != 0 {
				t.Errorf("journal recorded %v on a refused order, want nothing", j.states())
			}
			if out != (application.Outcome{}) {
				t.Errorf("Outcome = %+v on a refusal, want the zero value", out)
			}
		})
	}
}

// TestDispatchRefusesAnInvalidOrderWithTheDomainsOwnError is the one admit case
// the table above cannot phrase, since its error comes from domain rather than
// from this package.
func TestDispatchRefusesAnInvalidOrderWithTheDomainsOwnError(t *testing.T) {
	t.Parallel()

	d, _, p := newDispatcher(t)
	o := dispatchableOrder(t)
	o.Quantity = fixedpoint.Quantity{}

	_, err := d.Dispatch(o)
	if err == nil {
		t.Fatal("Dispatch of an order with no quantity = nil, want a refusal")
	}
	if errors.Is(err, application.ErrNotMarketableIOCLimit) || errors.Is(err, application.ErrOrderNotOMSMinted) {
		t.Errorf("Dispatch = %v, want the domain's validation error", err)
	}
	if p.calls != 0 {
		t.Errorf("Place called %d times, want 0", p.calls)
	}
}

// TestDispatchRefusesToPlaceWhenAuthorizationIsNotDurable is the safety claim of
// step 2, and it is the reason the journal fake fails a NAMED append.
func TestDispatchRefusesToPlaceWhenAuthorizationIsNotDurable(t *testing.T) {
	t.Parallel()

	diskFull := errors.New("no space left on device")
	d, j, p := newDispatcher(t)
	j.failOn[domain.StateDispatchAuthorized] = diskFull

	out, err := d.Dispatch(dispatchableOrder(t))
	if !errors.Is(err, diskFull) {
		t.Fatalf("Dispatch = %v, want the journal's error", err)
	}
	if p.calls != 0 {
		t.Errorf("Place called %d times after a failed authorization append, want 0", p.calls)
	}
	if out != (application.Outcome{}) {
		t.Errorf("Outcome = %+v, want the zero value when nothing was sent", out)
	}
}

// TestDispatchReportsAnUnjournaledResultWithoutFailing is step 6, and it is the
// claim the whole-journal-fails fake would have collapsed into the one above.
//
// A failure here is not a safety failure: the journal still holds
// DispatchAuthorized, which recovery resolves by query — exactly right for an
// order whose fate could not be recorded. So err stays nil, the Outcome is
// authoritative, and Journaled is what reports the alerting condition.
func TestDispatchReportsAnUnjournaledResultWithoutFailing(t *testing.T) {
	t.Parallel()

	d, j, p := newDispatcher(t)
	j.failOn[domain.StateSubmitted] = errors.New("no space left on device")

	out, err := d.Dispatch(dispatchableOrder(t))
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil — a lost result record is not a failed dispatch", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want 1", p.calls)
	}
	if out.State != domain.StateSubmitted {
		t.Errorf("State = %s, want %s", out.State, domain.StateSubmitted)
	}
	if out.Journaled {
		t.Error("Journaled = true after the result append failed, want false")
	}
	// The authorization record survives, and it is what recovery reads.
	if got := j.states(); len(got) == 0 || got[0] != domain.StateDispatchAuthorized {
		t.Errorf("journal = %v, want the authorization record to remain", got)
	}
}

// TestDispatchClassifiesThePlaceOutcome walks the Effect × Class table through
// the public entry point.
//
// The EffectNotApplied rows include ClassVenueReject AND three non-reject
// classes. With only one, "checks the class" and "ignores the class" give the
// same answer — and the three are different constructors rather than three
// spellings of one, because §5.3's whole point is that NotApplied covers both
// "never left the socket" and "refused on its merits".
func TestDispatchClassifiesThePlaceOutcome(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")

	cases := []struct {
		name      string
		placeErr  error
		wantState domain.OrderState
	}{
		{
			name:      "a clean place is Submitted",
			placeErr:  nil,
			wantState: domain.StateSubmitted,
		},
		{
			name:      "an applied effect is Submitted, because only the ack was lost",
			placeErr:  apperror.Transport("place", apperror.EffectApplied, boom),
			wantState: domain.StateSubmitted,
		},
		{
			name:      "an explicit venue rejection is the Rejected terminal",
			placeErr:  apperror.VenueRejected("place", "-2010", "insufficient margin", boom),
			wantState: domain.StateRejected,
		},
		{
			name:      "not-applied over transport is SubmissionUnknown, not Rejected",
			placeErr:  apperror.Transport("place", apperror.EffectNotApplied, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "our own precondition refusal is SubmissionUnknown, not Rejected",
			placeErr:  apperror.Precondition("place", "governor declined", boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "a rate-limit refusal is SubmissionUnknown, not Rejected",
			placeErr:  apperror.RateLimited("place", "429", 0, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			name:      "an unknown effect is SubmissionUnknown",
			placeErr:  apperror.Transport("place", apperror.EffectUnknown, boom),
			wantState: domain.StateSubmissionUnknown,
		},
		{
			// An adapter that forgot to classify. apperror answers
			// EffectUnknown for anything it never saw, so the order goes to the
			// venue to ask rather than to a terminal.
			name:      "an unclassified error is SubmissionUnknown",
			placeErr:  boom,
			wantState: domain.StateSubmissionUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d, j, p := newDispatcher(t)
			p.err = tc.placeErr

			out, err := d.Dispatch(dispatchableOrder(t))
			if err != nil {
				t.Fatalf("Dispatch = %v, want nil — Place ran, so there is a state to report", err)
			}
			if p.calls != 1 {
				t.Errorf("Place called %d times, want exactly 1 — there is no transport retry", p.calls)
			}
			if out.State != tc.wantState {
				t.Errorf("State = %s, want %s", out.State, tc.wantState)
			}
			if tc.placeErr == nil {
				if out.Cause != nil {
					t.Errorf("Cause = %v, want nil", out.Cause)
				}
			} else if !errors.Is(out.Cause, tc.placeErr) {
				t.Errorf("Cause = %v, want it to wrap %v", out.Cause, tc.placeErr)
			}
			// A declared effect is never the fallback.
			if errors.Is(out.Cause, application.ErrEffectUnhandled) {
				t.Errorf("Cause = %v, want no ErrEffectUnhandled for a declared effect", out.Cause)
			}
			if !out.Journaled {
				t.Error("Journaled = false, want true")
			}
			want := []domain.OrderState{domain.StateDispatchAuthorized, tc.wantState}
			if got := j.states(); !equalStates(got, want) {
				t.Errorf("journal recorded %v, want %v", got, want)
			}
		})
	}
}

// TestDispatchRefusesASecondDispatchOfTheSameOrder uses a VALID order whose
// first dispatch SUCCEEDED, so "refuses a repeat" cannot collapse into "refuses
// an invalid order".
func TestDispatchRefusesASecondDispatchOfTheSameOrder(t *testing.T) {
	t.Parallel()

	d, j, p := newDispatcher(t)
	o := dispatchableOrder(t)

	if _, err := d.Dispatch(o); err != nil {
		t.Fatalf("first Dispatch = %v, want nil", err)
	}

	out, err := d.Dispatch(o)
	if !errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Fatalf("second Dispatch = %v, want ErrOrderAlreadyDispatched", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want 1 — the second dispatch must send nothing", p.calls)
	}
	if len(j.calls) != 2 {
		t.Errorf("journal recorded %v, want only the first dispatch's two records", j.states())
	}
	if out != (application.Outcome{}) {
		t.Errorf("Outcome = %+v, want the zero value", out)
	}
}

// TestDispatchAdmitsADifferentOrder is the guard's positive control: it is keyed
// on the ClientOrderID, not a latch that closes after one dispatch.
func TestDispatchAdmitsADifferentOrder(t *testing.T) {
	t.Parallel()

	d, _, p := newDispatcher(t)
	first := dispatchableOrder(t)
	second := dispatchableOrder(t)
	second.ClientOrderID = mintedID(t, 0) // the other leg of the same trade

	if _, err := d.Dispatch(first); err != nil {
		t.Fatalf("first Dispatch = %v", err)
	}
	if _, err := d.Dispatch(second); err != nil {
		t.Fatalf("Dispatch of the other leg = %v, want nil", err)
	}
	if p.calls != 2 {
		t.Errorf("Place called %d times, want 2", p.calls)
	}
}

// TestDispatchDoesNotBurnTheIDOnARefusal covers the placement of step 3.
//
// The id is recorded only AFTER the authorization append succeeds, because
// everything before that point sent nothing — so a caller that corrects a
// refused order and calls again has not double-dispatched anything. Recording
// it in admit would look equivalent and would permanently reject an order whose
// only fault was a typo.
func TestDispatchDoesNotBurnTheIDOnARefusal(t *testing.T) {
	t.Parallel()

	t.Run("after an admit refusal", func(t *testing.T) {
		t.Parallel()

		d, _, p := newDispatcher(t)
		bad := dispatchableOrder(t)
		bad.TimeInForce = domain.TimeInForceGTC

		if _, err := d.Dispatch(bad); !errors.Is(err, application.ErrNotMarketableIOCLimit) {
			t.Fatalf("Dispatch of a GTC order = %v, want ErrNotMarketableIOCLimit", err)
		}
		if _, err := d.Dispatch(dispatchableOrder(t)); err != nil {
			t.Fatalf("Dispatch of the corrected order = %v, want nil", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
	})

	t.Run("after a failed authorization append", func(t *testing.T) {
		t.Parallel()

		d, j, p := newDispatcher(t)
		j.failOn[domain.StateDispatchAuthorized] = errors.New("transient")

		if _, err := d.Dispatch(dispatchableOrder(t)); err == nil {
			t.Fatal("Dispatch with a failing journal = nil, want a refusal")
		}
		// Nothing was sent, so a retry is legitimate — and this is the case the
		// guard must NOT swallow, since the alternative is an order that can
		// never be placed because its own journal hiccupped once.
		delete(j.failOn, domain.StateDispatchAuthorized)
		if _, err := d.Dispatch(dispatchableOrder(t)); err != nil {
			t.Fatalf("Dispatch after the journal recovered = %v, want nil", err)
		}
		if p.calls != 1 {
			t.Errorf("Place called %d times, want 1", p.calls)
		}
	})
}

// TestDispatchGuardsAgainstARepeatAfterAFailedPlace is the misuse the signature
// invites, and the reason the guard exists at all: err == nil with a non-nil
// Cause reads like "that failed, try again", and it is the one reading that
// must not send a second order.
func TestDispatchGuardsAgainstARepeatAfterAFailedPlace(t *testing.T) {
	t.Parallel()

	d, _, p := newDispatcher(t)
	p.err = apperror.Transport("place", apperror.EffectUnknown, errors.New("timeout"))
	o := dispatchableOrder(t)

	out, err := d.Dispatch(o)
	if err != nil {
		t.Fatalf("Dispatch = %v, want nil", err)
	}
	if out.Cause == nil {
		t.Fatal("Cause = nil, want the timeout")
	}
	if _, err := d.Dispatch(o); !errors.Is(err, application.ErrOrderAlreadyDispatched) {
		t.Errorf("retry after a failed Place = %v, want ErrOrderAlreadyDispatched", err)
	}
	if p.calls != 1 {
		t.Errorf("Place called %d times, want exactly 1", p.calls)
	}
}

func equalStates(got, want []domain.OrderState) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
