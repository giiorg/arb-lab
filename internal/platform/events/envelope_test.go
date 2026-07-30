package events_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// An Envelope is what §3.5 makes the unit of replay, and its two sequence
// numbers are the subtlest thing in this package. They are NOT redundant:
//
//   - AcceptOrdinal is stamped in raw ARRIVAL order on every raw input. It is
//     the causal/audit order — you reacted to what you received, in the order
//     you received it — and it is the capture lane's authority.
//   - IngressSeq is stamped on the DISPATCHED envelope in the order the state
//     machine actually consumes it. It is the replay key.
//
// They differ for two concrete reasons §3.5 spells out: reconstruction may HOLD
// a book delta behind a sequence gap while an independent source stays
// dispatchable, and coalescing means some raw inputs produce no dispatched
// envelope at all. So the dispatched stream is derived from arrival but is not
// forced to equal it, and replaying AcceptOrdinal order would replay an order
// the state machine never saw.
//
// The tests below exist mostly to stop those two from being quietly merged.

func aSource() events.SourceID {
	return events.SourceID{
		Instrument: ids.VenueInstrument{
			Venue: "binance", Product: ids.ProductLinearPerp,
			Symbol: "BTCUSDT", Contract: ids.ContractLinear,
		},
		Stream: events.StreamMark,
	}
}

func anEnvelope() events.Envelope {
	return events.Envelope{
		AcceptOrdinal: 1,
		IngressSeq:    1,
		Source:        aSource(),
		SourceTime:    time.Unix(1700000000, 0).UTC(),
		RecvTime:      time.Unix(1700000000, 5_000_000).UTC(),
		VenueSeq:      42,
		Prio:          0,
		Payload:       "a mark update",
	}
}

// TestEnvelopeFixtureIsValid is the positive control for every rejection below.
func TestEnvelopeFixtureIsValid(t *testing.T) {
	if err := anEnvelope().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if err := aSource().Validate(); err != nil {
		t.Fatalf("SourceID.Validate() = %v, want nil", err)
	}
}

// TestEnvelopeAcceptsAbsenceDerivedOrdinal pins the absence door: zero is
// §3.5's one legal AcceptOrdinal spelling for an envelope derived from the
// absence of input, so Validate — which cannot see derivation — must pass it.
// The input-derived refusal of zero belongs to producers (application's
// ErrNoTrigger), not here; a clause refusing zero in Validate would refuse
// the silence re-judger's envelope at the dispatcher's post-stamp check.
func TestEnvelopeAcceptsAbsenceDerivedOrdinal(t *testing.T) {
	e := anEnvelope()
	e.AcceptOrdinal = 0
	if err := e.Validate(); err != nil {
		t.Fatalf("Validate() with AcceptOrdinal 0 = %v, want nil — the absence-derived envelope must dispatch", err)
	}
}

// --- SourceID ----------------------------------------------------------------

// TestSourceIDIsAStreamOfAnInstrument checks the composition §3.5 asks for: a
// SourceID is "a VenueInstrument stream".
//
// Two streams of one instrument are distinct sources, and so are the same stream
// of two instruments. Both matter because SourceID keys the market-data
// mailboxes: collapsing either direction merges two feeds into one mailbox.
func TestSourceIDIsAStreamOfAnInstrument(t *testing.T) {
	mark := aSource()

	book := mark
	book.Stream = events.StreamBook

	// Spot BOOK rather than spot mark. Spot has no mark (§4), so the source this
	// fixture used to build is one no venue can produce — and since
	// SourceID.Validate now says so, a fixture that could not be subscribed to
	// would be arguing the mailbox point with a mailbox that cannot exist.
	spot := mark
	spot.Stream = events.StreamBook
	spot.Instrument = ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
	}

	boxes := map[events.SourceID]string{mark: "perp mark", book: "perp book", spot: "spot book"}
	if len(boxes) != 3 {
		t.Errorf("map[SourceID] holds %d entries for three distinct sources, want 3", len(boxes))
	}
	for s := range boxes {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil — a mailbox key must be a source that can exist", s, err)
		}
	}
}

// TestSourceIDRejects starts each case from a valid fixture and breaks one
// thing, so the rejection is attributable.
func TestSourceIDRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*events.SourceID)
		want error
	}{
		{
			// Delegated, not restated: one definition of a valid instrument.
			name: "instrument unset",
			mut:  func(s *events.SourceID) { s.Instrument = ids.VenueInstrument{} },
			want: ids.ErrVenueUnset,
		},
		{
			name: "instrument internally inconsistent",
			mut:  func(s *events.SourceID) { s.Instrument.Contract = ids.ContractInverse },
			want: ids.ErrContractMismatch,
		},
		{
			name: "stream unset",
			mut:  func(s *events.SourceID) { s.Stream = "" },
			want: events.ErrStreamUnset,
		},
		{
			name: "stream undeclared",
			mut:  func(s *events.SourceID) { s.Stream = "trades" },
			want: events.ErrStreamUnknown,
		},
		{
			// §4: mark is perp-only — "spot has none; price the spot leg from
			// its book". A mailbox for it is one nothing will ever deliver to,
			// which presents downstream as a dead feed rather than as a wiring
			// mistake.
			name: "mark asked of a spot instrument",
			mut: func(s *events.SourceID) {
				s.Instrument = ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
				}
			},
			want: events.ErrStreamProductMismatch,
		},
		{
			name: "funding asked of a spot instrument",
			mut: func(s *events.SourceID) {
				s.Stream = events.StreamFunding
				s.Instrument = ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
				}
			},
			want: events.ErrStreamProductMismatch,
		},
		{
			// Both halves are wrong at once, and the STREAM error is the one
			// that must come out. Without this case the ordering inside
			// Validate is untested: a mismatch check placed first would report
			// that a source disagrees with itself when in fact half of it is
			// simply unset, sending a reader to the instrument instead of to
			// the empty field.
			name: "unset stream on a spot instrument reports the stream",
			mut: func(s *events.SourceID) {
				s.Stream = ""
				s.Instrument = ids.VenueInstrument{
					Venue: "binance", Product: ids.ProductSpot, Symbol: "BTCUSDT",
				}
			},
			want: events.ErrStreamUnset,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := aSource()
			c.mut(&s)
			if err := s.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestStreamKindDeclaredSet pins §3.6 rule 8's declared set and the persisted
// tokens.
//
// `type X string` because SourceID is written to the capture lane — §3.5 makes
// the RawIngressSeq→DispatchSeq mapping "explicit and persisted", and replay
// re-applies it. A stored token that renames itself orphans the log.
func TestStreamKindDeclaredSet(t *testing.T) {
	declared := []events.StreamKind{
		events.StreamBook, events.StreamMark, events.StreamFunding,
		events.StreamOrders, events.StreamExecutions, events.StreamAccount, events.StreamControl,
	}
	want := []string{"book", "mark", "funding", "orders", "executions", "account", "control"}

	for i, s := range declared {
		if err := s.Validate(); err != nil {
			t.Errorf("declared %q: Validate() = %v, want nil", s, err)
		}
		if string(s) != want[i] {
			t.Errorf("token %d = %q, want %q — this value is persisted", i, s, want[i])
		}
		if s.String() != string(s) {
			t.Errorf("String() = %q, want %q", s.String(), string(s))
		}
	}

	if err := events.StreamKind("").Validate(); !errors.Is(err, events.ErrStreamUnset) {
		t.Errorf("empty: Validate() = %v, want ErrStreamUnset", err)
	}
	if err := events.StreamKind("ticker").Validate(); !errors.Is(err, events.ErrStreamUnknown) {
		t.Errorf("undeclared: Validate() = %v, want ErrStreamUnknown", err)
	}
}

// TestStreamLane is the routing claim: every declared stream names exactly one
// §3.4 lane, and the assignment is a property of the stream rather than of
// whoever happens to publish it.
//
// The two that matter are the private ones. Executions and account updates are
// control-lane traffic — losing one loses execution truth — and a stream
// misrouted onto the market-data lane would be silently coalesced, which is the
// specific way a fill disappears with nothing failing.
func TestStreamLane(t *testing.T) {
	cases := []struct {
		s    events.StreamKind
		want events.Lane
	}{
		{events.StreamBook, events.LaneMarketData},
		{events.StreamMark, events.LaneMarketData},
		{events.StreamFunding, events.LaneMarketData},
		// Orders and executions are SEPARATE kinds, and both are control lane.
		// §4's OrderGateway declares two channels because a venue sequences them
		// independently; one kind for both would put an accept, a reject and a
		// fill in one per-instrument mailbox, where the exactly-once fill rule
		// would have to deduplicate against a queue it shares.
		{events.StreamOrders, events.LaneControl},
		{events.StreamExecutions, events.LaneControl},
		{events.StreamAccount, events.LaneControl},
		{events.StreamControl, events.LaneControl},
	}
	for _, c := range cases {
		if got := c.s.Lane(); got != c.want {
			t.Errorf("StreamKind(%q).Lane() = %v, want %v", c.s, got, c.want)
		}
	}

	// An undeclared stream routes to the control lane. That direction is the
	// safe one: the control lane never drops and never coalesces, so an
	// unrecognised stream is delivered intact rather than silently thinned. The
	// opposite default would lose data for a stream nobody had classified yet.
	if got := events.StreamKind("trades").Lane(); got != events.LaneControl {
		t.Errorf("undeclared stream routes to %v, want LaneControl (never-drop is the safe default)", got)
	}
}

// TestStreamRequiresPerp is §4's per-stream product constraint, carried across
// into §3.5's SourceID.
//
// The two that matter are funding and mark. Spot has neither — §4 says so of
// both, and §2.2's cash-and-carry holds spot against a perp, so a spot leg
// reading "funding 0" is indistinguishable from a perp in a flat regime and the
// carry computation nets them.
func TestStreamRequiresPerp(t *testing.T) {
	cases := []struct {
		s    events.StreamKind
		want bool
	}{
		{events.StreamFunding, true},
		{events.StreamMark, true},
		{events.StreamBook, false},
		// The private streams carry no product constraint, and that is a claim
		// rather than an oversight: spot fills and spot balances are real, and
		// §4.2 makes the spot leg a full API family with its own user-data
		// stream. Constraining them would silently unsubscribe half the trade.
		{events.StreamOrders, false},
		{events.StreamExecutions, false},
		{events.StreamAccount, false},
		{events.StreamControl, false},
	}
	for _, c := range cases {
		if got := c.s.RequiresPerp(); got != c.want {
			t.Errorf("StreamKind(%q).RequiresPerp() = %v, want %v", c.s, got, c.want)
		}
	}

	// An undeclared stream answers true — the narrower claim. Unreachable
	// through Validate, which refuses an undeclared StreamKind first, so this
	// pins the direction rather than a behaviour anything depends on.
	if !events.StreamKind("trades").RequiresPerp() {
		t.Error("an undeclared stream answers false; the safe default denies a source nobody classified")
	}
}

// TestPerpConstraintIsAboutPerpsNotAboutLinearOnes is the positive control for
// the check SourceID.Validate applies, and it exists because the obvious
// fixture collapses it.
//
// Every perp in this repo's fixtures is a LINEAR perp, so `Product.IsPerp()`
// and `Product == ProductLinearPerp` return the same answer for all of them and
// a mismatch check written as the second would pass every test. An inverse perp
// separates them: it has funding and a mark, and §5.1 declares the product for
// exactly this reason — "no venue adapter trades one in the MVP, and it is
// declared anyway", so that "this contract is linear" is a checked claim rather
// than an assumption.
func TestPerpConstraintIsAboutPerpsNotAboutLinearOnes(t *testing.T) {
	inverse := ids.VenueInstrument{
		Venue: "binance", Product: ids.ProductInversePerp,
		Symbol: "BTCUSD", Contract: ids.ContractInverse,
	}
	for _, stream := range []events.StreamKind{events.StreamFunding, events.StreamMark} {
		s := events.SourceID{Instrument: inverse, Stream: stream}
		if err := s.Validate(); err != nil {
			t.Errorf("%s on an inverse perp: Validate() = %v, want nil — an inverse perp has funding and a mark", stream, err)
		}
	}
}

// --- Envelope ----------------------------------------------------------------

func TestEnvelopeRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*events.Envelope)
		want error
	}{
		{
			name: "source invalid",
			mut:  func(e *events.Envelope) { e.Source.Stream = "" },
			want: events.ErrStreamUnset,
		},
		{
			name: "ingress seq unstamped",
			mut:  func(e *events.Envelope) { e.IngressSeq = 0 },
			want: events.ErrIngressSeqUnset,
		},
		{
			name: "recv time unset",
			mut:  func(e *events.Envelope) { e.RecvTime = time.Time{} },
			want: events.ErrRecvTimeUnset,
		},
		{
			name: "payload nil",
			mut:  func(e *events.Envelope) { e.Payload = nil },
			want: events.ErrPayloadNil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := anEnvelope()
			c.mut(&e)
			if err := e.Validate(); !errors.Is(err, c.want) {
				t.Errorf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestSourceTimeMayBeUnset is the deliberate asymmetry between the two
// timestamps, and it is worth stating as a test because "both are times" invites
// treating them alike.
//
// RecvTime is OURS — we stamped it when the bytes arrived, so its absence is a
// wiring bug. SourceTime is the VENUE'S, and not every message carries one:
// §3.5 keeps it for freshness, cross-leg skew and offline analysis, explicitly
// never as a reorder key. Requiring it would force adapters to invent one, and
// an invented venue timestamp is worse than an absent one — it would be silently
// compared against a real one in a skew metric.
func TestSourceTimeMayBeUnset(t *testing.T) {
	e := anEnvelope()
	e.SourceTime = time.Time{}
	if err := e.Validate(); err != nil {
		t.Errorf("Validate() with no SourceTime = %v, want nil", err)
	}
	if e.HasSourceTime() {
		t.Error("HasSourceTime() = true for the zero time")
	}
	if !anEnvelope().HasSourceTime() {
		t.Error("HasSourceTime() = false for a stamped envelope")
	}
}

// TestEnvelopeAgeUsesRecvTime pins which clock reading freshness is measured
// from.
//
// §3.4 gates execution on freshness and drives liq-distance off a stale-mark
// timer, and §3.6 rule 5 spells out the hazard: two disagreeing clocks either
// side of a subtraction yield a negative or inflated age exactly across an NTP
// step. Age is therefore computed from RecvTime — the instant WE stamped, from
// the injected clock — not from the venue's SourceTime, whose offset is a
// separate modelled quantity (clock.ServerNow's uncertainty band).
func TestEnvelopeAgeUsesRecvTime(t *testing.T) {
	e := anEnvelope()
	e.RecvTime = time.Unix(1700000000, 0).UTC()
	e.SourceTime = time.Unix(1600000000, 0).UTC() // a year earlier; must not be used

	now := time.Unix(1700000030, 0).UTC()
	if got, want := e.Age(now), 30*time.Second; got != want {
		t.Errorf("Age() = %v, want %v (must measure from RecvTime, not SourceTime)", got, want)
	}
}

// TestEnvelopeAgeNeverNegative is the clamp §3.6 rule 5 argues for.
//
// A negative age reads downstream as "fresher than now", which passes every
// max-age check ever written. Under a backwards clock step that is precisely how
// a stale mark keeps gating execution — so an out-of-order reading is reported
// as zero age, the most conservative answer, rather than as a negative one.
func TestEnvelopeAgeNeverNegative(t *testing.T) {
	e := anEnvelope()
	e.RecvTime = time.Unix(1700000000, 0).UTC()

	before := time.Unix(1699999990, 0).UTC()
	if got := e.Age(before); got != 0 {
		t.Errorf("Age(before RecvTime) = %v, want 0 — a negative age passes every freshness check", got)
	}
}

// --- Prio --------------------------------------------------------------------

// TestPrioZeroIsMostUrgent pins the direction §3.5 makes normative, in the one
// place a reader is most likely to assume the opposite.
//
// It matches clock's timer ordering deliberately: an envelope priority and a
// timer priority that disagreed about direction would put a safety timer and a
// safety event on opposite ends of the same queue.
func TestPrioZeroIsMostUrgent(t *testing.T) {
	if !events.PrioSafety.MoreUrgentThan(events.PrioNormal) {
		t.Error("PrioSafety is not more urgent than PrioNormal; lower Prio must be MORE urgent")
	}
	if events.PrioSafety != 0 {
		t.Errorf("PrioSafety = %d, want 0 — the zero value must be the most urgent, so an unset priority cannot send a safety event to the back", events.PrioSafety)
	}
	if events.PrioNormal.MoreUrgentThan(events.PrioSafety) {
		t.Error("MoreUrgentThan is symmetric; it must be a strict order")
	}
	if events.PrioNormal.MoreUrgentThan(events.PrioNormal) {
		t.Error("MoreUrgentThan(self) = true; it must be strict")
	}
}

// --- rendering ---------------------------------------------------------------

func TestSourceIDString(t *testing.T) {
	got := aSource().String()
	for _, want := range []string{"binance", "BTCUSDT", "mark"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, want it to mention %q", got, want)
		}
	}
}
