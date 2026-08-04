package venue_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks §4's read half, venue-side (slice C2b, record §52).
//
// # The collapse checklist was run BEFORE these were written, and it moved five
// # fixtures
//
// Each of the following is a parameter pinned to a value that would make a
// distinction the code makes invisible, and each is varied here:
//
//   - The window span is 48 HOURS, which is between Binance spot's published 24
//     hours and USDⓈ-M's 7 days. Anything under a day is legal on both families
//     and a shared cap would pass; anything over a week is illegal on both.
//   - The Bybit execution fixture has TWO rows sharing one execTime, because
//     execTime is documented as non-unique and a fixture of distinct instants
//     makes "rewind to the tied instant" and "advance past the newest" the same
//     cursor.
//   - The Bybit page fixture is FULL and its rows are newest-first, because a
//     short page cannot show direction and a single row cannot show ordering.
//   - The retention fixtures compare 3 days against 24 hours against unpublished
//     — three distinct answers, so a reach that returned one constant fails.
//   - The absent fixtures separate "the venue said no such order" from "the
//     query was rate-limited", which the obvious `err != nil` assertion cannot
//     tell apart and which is the whole of §52c.

func testAssets() venue.InstrumentAssets {
	return venue.InstrumentAssets{Base: "BTC", Quote: "USDT"}
}

func perpRef(t *testing.T, v ids.Venue) venue.OrderRef {
	t.Helper()
	return venue.OrderRef{Instrument: perpInstrument(v), ClientOrderID: mintedID(t)}
}

func spotRef(t *testing.T, v ids.Venue) venue.OrderRef {
	t.Helper()
	return venue.OrderRef{Instrument: spotInstrument(v), ClientOrderID: mintedID(t)}
}

// sweepStart is a fixed instant; the clock is injected everywhere in this repo
// and a test that read the wall clock would produce a different window each run.
var sweepStart = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

// window48h spans two days — deliberately BETWEEN the two Binance caps.
func window48h() venue.ExecutionWindow {
	return venue.ExecutionWindow{
		Since: venue.Cursor{At: sweepStart},
		Until: sweepStart.Add(48 * time.Hour),
	}
}

func okResponse(body string) transport.Response {
	return transport.Response{Status: 200, Body: []byte(body)}
}

// ---------------------------------------------------------------------------
// §52c — an absence is a value, and a failed query is not one
// ---------------------------------------------------------------------------

func TestBinanceReadsItsNoSuchOrderCodeAsAnAbsence(t *testing.T) {
	codec := binanceCodec(t)

	// The shape SignedClient.Do produces for {"code":-2013,"msg":"Order does not
	// exist."} beside a 4xx: a classified venue rejection.
	absent := apperror.VenueRejected("binance.OrderByClientID", "-2013", "Order does not exist.", nil)
	if !codec.AbsenceFromError(absent) {
		t.Errorf("AbsenceFromError(-2013 VenueRejected) = false, want true — §5.3's stable-not-found rule has no input without it")
	}
}

// TestAQueryThatFailedIsNeverReadAsAnAbsence is the assertion the obvious
// `err != nil` test cannot make. §5.3: "'not submitted' is never inferred from a
// transient not-found", and "if the resolving query is unavailable/rate-limited,
// new trading stays disabled".
func TestAQueryThatFailedIsNeverReadAsAnAbsence(t *testing.T) {
	codec := binanceCodec(t)

	for name, err := range map[string]error{
		"rate limited":      apperror.RateLimited("binance.OrderByClientID", "-1003", time.Second, nil),
		"unauthorized":      apperror.Unauthorized("binance.OrderByClientID", "-2015", nil),
		"venue unavailable": apperror.Transport("binance.OrderByClientID", apperror.EffectUnknown, errors.New("503")),

		// A -2013 that did NOT arrive as EffectNotApplied is a response this
		// project does not understand, and reading an absence out of it is
		// exactly the forbidden inference: EffectUnknown means we do not know
		// what became of the REQUEST, so a definite "no such order" cannot be
		// read out of it.
		//
		// # This one is built with apperror.New on purpose, and the reason is a
		// # finding of its own
		//
		// Every apperror constructor that sets a Code — RateLimited,
		// Unauthorized, VenueRejected — also PINS Effect to EffectNotApplied,
		// and the only constructor taking an arbitrary effect (Transport) sets
		// no code at all. So today this combination is unreachable through the
		// classifier, and the effect half of the guard survived its mutation for
		// that reason rather than for a missing test: it is an equivalent mutant
		// under the constructors as they stand.
		//
		// It is kept, and tested through the Spec, because its necessity rests
		// on an invariant of a DIFFERENT package that nothing states — a future
		// constructor for "a 5xx that carried a venue code" would be entirely
		// natural and would make this branch live the day it lands. Reaching it
		// here is what turns "unreachable so harmless" into a claim that fails
		// out loud if that changes.
		"right code, wrong effect": apperror.New(apperror.Spec{
			Class:  apperror.ClassTransport,
			Effect: apperror.EffectUnknown,
			Op:     "binance.OrderByClientID",
			Code:   "-2013",
		}),

		"not an apperror at all": errors.New("dial tcp: connection refused"),
	} {
		if codec.AbsenceFromError(err) {
			t.Errorf("AbsenceFromError(%s) = true, want false — a query that could not be answered would become proved absence", name)
		}
	}
}

func TestBybitNeverReadsAnAbsenceOutOfAnError(t *testing.T) {
	codec := bybitCodec(t)

	// On Bybit an absence is a SUCCESS with an empty list, so no error is one.
	// This also makes the file's one unverified assumption fail closed: if Bybit
	// does signal absence with a retCode, the caller sees a failed query.
	if codec.AbsenceFromError(apperror.VenueRejected("bybit.OrderByClientID", "110001", "order does not exist", nil)) {
		t.Error("bybit AbsenceFromError = true; the unverified retCode branch must fail CLOSED")
	}
}

func TestBybitReadsAnEmptyListAsAnAbsence(t *testing.T) {
	codec := bybitCodec(t)
	ref := perpRef(t, testBybit)

	got, err := codec.DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus,
		okResponse(`{"retCode":0,"retMsg":"OK","result":{"category":"linear","list":[],"nextPageCursor":""}}`))
	if err != nil {
		t.Fatalf("DecodeOrderStatus on an empty list: %v — an absence must not be an error", err)
	}
	if got.Presence != venue.OrderAbsent {
		t.Errorf("presence = %q, want %q", got.Presence, venue.OrderAbsent)
	}
}

// ---------------------------------------------------------------------------
// §52d — the retention horizons, and they are three different answers
// ---------------------------------------------------------------------------

func TestTheRetentionHorizonsDifferPerSurfaceAndFamily(t *testing.T) {
	binance := binanceCodec(t)
	bybit := bybitCodec(t)

	perpReach, err := binance.StatusReach(perpInstrument(testBinance), venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("binance perp StatusReach: %v", err)
	}
	// "created time + 3 days < current time" for a CANCELED-or-EXPIRED order
	// with no filled trade — which is every unfilled marketable IOC we send.
	if perpReach.UnfilledRetention != 3*24*time.Hour {
		t.Errorf("binance perp unfilled retention = %v, want 72h", perpReach.UnfilledRetention)
	}
	if perpReach.FilledRetention != 90*24*time.Hour {
		t.Errorf("binance perp filled retention = %v, want 2160h", perpReach.FilledRetention)
	}
	if perpReach.Unpublished {
		t.Error("binance perp reach reports Unpublished; the venue publishes both horizons")
	}

	spotReach, err := binance.StatusReach(spotInstrument(testBinance), venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("binance spot StatusReach: %v", err)
	}
	// Spot publishes NO retention. Encoding that as "same as futures" would
	// assume the unverified in the unsafe direction; encoding it as "no cliff"
	// would license treating a spot absence as proved absence forever.
	if !spotReach.Unpublished {
		t.Errorf("binance spot reach = %+v, want Unpublished — the spot page states no retention", spotReach)
	}

	histReach, err := bybit.StatusReach(perpInstrument(testBybit), venue.SurfaceOrderHistory)
	if err != nil {
		t.Fatalf("bybit history StatusReach: %v", err)
	}
	// "The orders in the last 24 hours: orders with 'Cancelled', 'Rejected',
	// 'Deactivated' can be queried."
	if histReach.UnfilledRetention != 24*time.Hour {
		t.Errorf("bybit history unfilled retention = %v, want 24h", histReach.UnfilledRetention)
	}
	if histReach.FilledRetention != 7*24*time.Hour {
		t.Errorf("bybit history filled retention = %v, want 168h", histReach.FilledRetention)
	}

	// Realtime's reach is a COUNT and a restart event, not a duration.
	rtReach, err := bybit.StatusReach(perpInstrument(testBybit), venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("bybit realtime StatusReach: %v", err)
	}
	if !rtReach.Unpublished {
		t.Errorf("bybit realtime reach = %+v, want Unpublished — \"the most recent 500 orders\" is not a duration", rtReach)
	}
}

func TestQueryReachRefusesAnUnpopulatedOrContradictoryHorizon(t *testing.T) {
	// The zero value is unusable: a zero retention would read as "forever",
	// which is what makes a stale absence look like proved absence.
	zero := venue.QueryReach{Surface: venue.SurfaceOrderStatus}
	if err := zero.Validate(); !errors.Is(err, venue.ErrQueryReachUnset) {
		t.Errorf("zero reach = %v, want ErrQueryReachUnset", err)
	}
	both := venue.QueryReach{Surface: venue.SurfaceOrderStatus, Unpublished: true, FilledRetention: time.Hour}
	if err := both.Validate(); !errors.Is(err, venue.ErrQueryReachContradictory) {
		t.Errorf("unpublished-with-a-duration = %v, want ErrQueryReachContradictory", err)
	}
}

// ---------------------------------------------------------------------------
// §52b-2 — the window caps, and they are a factor of seven apart
// ---------------------------------------------------------------------------

// TestTheTwoBinanceFamiliesWindowCapsDiffer is the fixture the checklist moved.
// 48 hours is legal on USDⓈ-M ("cannot be longer than 7 days") and illegal on
// spot ("can't be longer than 24 hours"), so one shared cap fails whichever
// number it named.
func TestTheTwoBinanceFamiliesWindowCapsDiffer(t *testing.T) {
	codec := binanceCodec(t)
	w := window48h()

	if _, err := codec.EncodeExecutions(perpInstrument(testBinance), w); err != nil {
		t.Errorf("48h sweep on USDⓈ-M = %v, want accepted (its cap is 7 days)", err)
	}
	_, err := codec.EncodeExecutions(spotInstrument(testBinance), w)
	if !errors.Is(err, venue.ErrExecutionWindowTooWide) {
		t.Errorf("48h sweep on spot = %v, want ErrExecutionWindowTooWide (its cap is 24 hours)", err)
	}
}

func TestBybitRefusesAWindowPastItsSevenDayCap(t *testing.T) {
	codec := bybitCodec(t)
	w := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(8 * 24 * time.Hour)}
	if _, err := codec.EncodeExecutions(perpInstrument(testBybit), w); !errors.Is(err, venue.ErrExecutionWindowTooWide) {
		t.Errorf("8-day bybit sweep = %v, want ErrExecutionWindowTooWide", err)
	}
	// And the boundary: exactly 7 days is "endTime - startTime <= 7 days".
	at := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(7 * 24 * time.Hour)}
	if _, err := codec.EncodeExecutions(perpInstrument(testBybit), at); err != nil {
		t.Errorf("exactly-7-day bybit sweep = %v, want accepted — the cap is inclusive", err)
	}
}

// TestTheSweepSendsItsHorizonRatherThanDefaulting is §6.7 step 3's requirement.
// All three surfaces return data with the times omitted and prove coverage
// through nothing.
func TestTheSweepSendsItsHorizonRatherThanDefaulting(t *testing.T) {
	w := window48h()
	for name, call := range map[string]transport.Call{
		"binance": mustEncodeExecutions(t, binanceCodec(t), perpInstrument(testBinance), w),
		"bybit":   mustEncodeExecutions(t, bybitCodec(t), perpInstrument(testBybit), w),
	} {
		params := map[string]string{}
		for _, p := range call.Params {
			params[p.Key] = p.Value
		}
		if got, want := params["startTime"], fmt.Sprintf("%d", w.Since.At.UnixMilli()); got != want {
			t.Errorf("%s startTime = %q, want %q", name, got, want)
		}
		if got, want := params["endTime"], fmt.Sprintf("%d", w.Until.UnixMilli()); got != want {
			t.Errorf("%s endTime = %q, want %q", name, got, want)
		}
	}
}

func mustEncodeExecutions(t *testing.T, c venue.OrderQueryCodec, in ids.VenueInstrument, w venue.ExecutionWindow) transport.Call {
	t.Helper()
	call, err := c.EncodeExecutions(in, w)
	if err != nil {
		t.Fatalf("EncodeExecutions: %v", err)
	}
	return call
}

// ---------------------------------------------------------------------------
// §52b / §52b-1 — the cursor, its direction and its ties
// ---------------------------------------------------------------------------

// TestAnEmptySweepStillAdvancesTheWatermark is the commonest case and the one
// §6.7 step 3 depends on: "nothing happened through T" is a positive answer.
func TestAnEmptySweepStillAdvancesTheWatermark(t *testing.T) {
	w := window48h()

	binancePage, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(`[]`))
	if err != nil {
		t.Fatalf("binance empty sweep: %v", err)
	}
	if !binancePage.Watermark.At.Equal(w.Until) {
		t.Errorf("binance empty watermark = %v, want the horizon %v", binancePage.Watermark.At, w.Until)
	}
	if binancePage.More != nil {
		t.Error("binance empty sweep reports More; an exhausted window has no continuation")
	}

	bybitPage, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w,
		okResponse(`{"retCode":0,"result":{"list":[],"nextPageCursor":""}}`))
	if err != nil {
		t.Fatalf("bybit empty sweep: %v", err)
	}
	if !bybitPage.Watermark.At.Equal(w.Until) {
		t.Errorf("bybit empty watermark = %v, want the horizon %v", bybitPage.Watermark.At, w.Until)
	}
}

// TestAShortPageProvesCoverageThroughTheHorizon — fewer rows than the venue's
// limit means the window was exhausted, so the watermark is Until and NOT the
// last row's instant. Those two differ here by design.
func TestAShortPageProvesCoverageThroughTheHorizon(t *testing.T) {
	w := window48h()
	body := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d,"side":"SELL"}]`,
		sweepStart.Add(time.Hour).UnixMilli())

	page, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	if !page.Watermark.At.Equal(w.Until) {
		t.Errorf("watermark = %v, want the horizon %v — a short page exhausted the window", page.Watermark.At, w.Until)
	}
	if page.More != nil {
		t.Error("short page reports More")
	}
	if len(page.Executions) != 1 {
		t.Fatalf("got %d executions, want 1", len(page.Executions))
	}
	// The join key on Binance is the venue's order id, because the trade rows
	// carry no client order id at all (§52's finding 4).
	if page.Executions[0].ClientOrderID != "" {
		t.Errorf("binance execution carries ClientOrderID %q; the venue does not report one",
			page.Executions[0].ClientOrderID)
	}
	if page.Executions[0].VenueOrderID != "991" {
		t.Errorf("VenueOrderID = %q, want %q — it is the only attribution handle on this venue",
			page.Executions[0].VenueOrderID, "991")
	}
}

// TestBybitPagesBackwardAndSaysSoInTheContinuation is finding 1b.
//
// A FULL page on a descending venue means the rows we have not seen are OLDER
// than every row we have. Advancing the time cursor past the newest would skip
// them silently, so the continuation is the venue's own token and Since is left
// exactly where it was.
func TestBybitPagesBackwardAndSaysSoInTheContinuation(t *testing.T) {
	w := window48h()
	page, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w,
		okResponse(bybitFullDescendingPage(sweepStart)))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	if page.More == nil {
		t.Fatal("a full page reports no More; the older rows behind it would be lost")
	}
	if page.More.Continuation != "page-2" {
		t.Errorf("continuation = %q, want the venue's nextPageCursor %q", page.More.Continuation, "page-2")
	}
	if !page.More.Since.At.Equal(w.Since.At) {
		t.Errorf("More.Since = %v, want the ORIGINAL %v — advancing it skips the older rows this page did not return",
			page.More.Since.At, w.Since.At)
	}
	if !page.More.Until.Equal(w.Until) {
		t.Errorf("More.Until = %v, want %v", page.More.Until, w.Until)
	}
}

// TestExecutionsAreNormalizedOldestFirst — Bybit sends newest first, and a
// caller taking the last row's instant as a watermark must not be reading
// Bybit's ordering.
func TestExecutionsAreNormalizedOldestFirst(t *testing.T) {
	w := window48h()
	page, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w,
		okResponse(bybitFullDescendingPage(sweepStart)))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	for i := 1; i < len(page.Executions); i++ {
		if page.Executions[i].At.Before(page.Executions[i-1].At) {
			t.Fatalf("execution %d at %v precedes %d at %v; the page is not ascending",
				i, page.Executions[i].At, i-1, page.Executions[i-1].At)
		}
	}
	// And the venue really did send them the other way, or this proves nothing.
	first, last := page.Executions[0], page.Executions[len(page.Executions)-1]
	if !first.At.Before(last.At) {
		t.Fatal("the fixture's rows all share one instant; it cannot show ordering")
	}
}

// TestTiedExecutionTimesAreOrderedByExecutionID — Bybit documents execTime as
// non-unique ("when execTime matches, it is recommended to sort according to
// execId+OrderId+leavesQty"), so a time-only sort leaves tied rows in whatever
// order the venue sent them and two sweeps disagree about the same data.
func TestTiedExecutionTimesAreOrderedByExecutionID(t *testing.T) {
	w := window48h()
	page, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w,
		okResponse(bybitFullDescendingPage(sweepStart)))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}

	var tied []venue.Execution
	for i := 1; i < len(page.Executions); i++ {
		if page.Executions[i].At.Equal(page.Executions[i-1].At) {
			tied = append(tied, page.Executions[i-1], page.Executions[i])
		}
	}
	if len(tied) < 2 {
		t.Fatal("the fixture contains no two executions sharing an instant, so it cannot check the tiebreak")
	}
	if tied[0].ExecutionID >= tied[1].ExecutionID {
		t.Errorf("tied executions ordered %q then %q; the tiebreak must be ascending by execution id",
			tied[0].ExecutionID, tied[1].ExecutionID)
	}
}

// TestAFullPageResumesATtheLastInstantRatherThanAfterIt — duplicates are free
// (§5.1 dedupes on the execution id) and gaps are not, so the cursor rewinds to
// the tied instant.
func TestAFullPageResumesAtTheLastInstantRatherThanAfterIt(t *testing.T) {
	w := window48h()
	page, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w,
		okResponse(binanceFullPage(sweepStart)))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	if page.More == nil {
		t.Fatal("a full page reports no More")
	}
	last := page.Executions[len(page.Executions)-1].At
	if !page.More.Since.At.Equal(last) {
		t.Errorf("resume = %v, want the LAST instant %v (inclusive); advancing past it drops any fill tied to it",
			page.More.Since.At, last)
	}
	if !page.Watermark.At.Equal(last) {
		t.Errorf("watermark = %v, want %v — a full page proves coverage through nothing beyond its own last row",
			page.Watermark.At, last)
	}
}

// TestAFullPageAtOneInstantIsRefusedRatherThanLooped — an inclusive cursor that
// cannot advance is an infinite sweep in which every request succeeds.
func TestAFullPageAtOneInstantIsRefusedRatherThanLooped(t *testing.T) {
	w := window48h()
	rows := make([]string, 0, 1000)
	for i := 0; i < 1000; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"orderId":991,"symbol":"BTCUSDT","price":"50100.25",
			"qty":"0.001","commission":"0.01","commissionAsset":"USDT","time":%d,"side":"SELL"}`,
			i, sweepStart.UnixMilli()))
	}
	body := "[" + joinRows(rows) + "]"

	_, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
	if !errors.Is(err, venue.ErrExecutionsNoProgress) {
		t.Errorf("a full page all at the resume instant = %v, want ErrExecutionsNoProgress", err)
	}
}

// TestAnExecutionOutsideTheSweptWindowIsRefused checks BOTH ends of the bound.
//
// The one-ended version of this test passed while a mutant that stopped checking
// the lower bound survived, which is the standing two-sided-bound trap: a window
// has two edges and a fixture past only one of them exercises only one predicate.
func TestAnExecutionOutsideTheSweptWindowIsRefused(t *testing.T) {
	w := window48h()
	for name, at := range map[string]time.Time{
		"past the horizon":        w.Until.Add(time.Hour),
		"before the resume point": w.Since.At.Add(-time.Hour),
	} {
		body := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
			"commission":"0.02","commissionAsset":"USDT","time":%d,"side":"SELL"}]`, at.UnixMilli())
		_, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
		if !errors.Is(err, venue.ErrExecutionsUnreadable) {
			t.Errorf("an execution %s = %v, want ErrExecutionsUnreadable — the watermark does not account for it", name, err)
		}
	}

	// The positive control: a row INSIDE the window is accepted, or a decoder
	// that refused everything would score perfectly above.
	inside := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d,"side":"SELL"}]`, w.Since.At.Add(time.Hour).UnixMilli())
	if _, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(inside)); err != nil {
		t.Errorf("an execution inside the window = %v, want accepted", err)
	}
}

// TestAnExecutionStatingBothDirectionSpellingsIsRefused — USDⓈ-M sends `side`
// and spot sends `isBuyer`, and a row carrying both is either a venue change or
// a response from somewhere this decoder does not understand. Choosing which to
// believe is what it declines to do, and declining is only a claim if something
// checks it.
func TestAnExecutionStatingBothDirectionSpellingsIsRefused(t *testing.T) {
	w := window48h()
	body := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d,"side":"SELL","isBuyer":true}]`,
		sweepStart.Add(time.Hour).UnixMilli())
	_, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
	if !errors.Is(err, venue.ErrExecutionsUnreadable) {
		t.Errorf("an execution stating side AND isBuyer = %v, want ErrExecutionsUnreadable", err)
	}
}

func TestTheCursorAndWindowRefuseTheirZeroValues(t *testing.T) {
	if err := (venue.Cursor{}).Validate(); !errors.Is(err, venue.ErrCursorUnset) {
		t.Errorf("zero cursor = %v, want ErrCursorUnset — there is no \"from the beginning\"", err)
	}
	if err := (venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}}).Validate(); !errors.Is(err, venue.ErrExecutionWindowUnset) {
		t.Errorf("window with no horizon = %v, want ErrExecutionWindowUnset", err)
	}
	inverted := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(-time.Hour)}
	if err := inverted.Validate(); !errors.Is(err, venue.ErrExecutionWindowInverted) {
		t.Errorf("inverted window = %v, want ErrExecutionWindowInverted", err)
	}
	// The boundary: Until == Since is not a window, because "strictly later"
	// is what §6.7 step 3 asks of a watermark.
	empty := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart}
	if err := empty.Validate(); !errors.Is(err, venue.ErrExecutionWindowInverted) {
		t.Errorf("zero-width window = %v, want ErrExecutionWindowInverted", err)
	}
}

// ---------------------------------------------------------------------------
// The direction field, which the two Binance families spell differently
// ---------------------------------------------------------------------------

// TestBinanceReadsBothFamiliesDirectionSpellings — USDⓈ-M sends "side":"SELL"
// and spot sends "isBuyer":false. A decoder reading one of them turns every fill
// on the other family into a BUY, which is a wrong SIGN in the fill ledger and
// the one error in this package that changes the net delta rather than failing.
func TestBinanceReadsBothFamiliesDirectionSpellings(t *testing.T) {
	w := window48h()
	at := sweepStart.Add(time.Hour).UnixMilli()

	perpBody := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d,"side":"SELL"}]`, at)
	perpPage, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(perpBody))
	if err != nil {
		t.Fatalf("perp decode: %v", err)
	}
	if perpPage.Executions[0].Side != venue.OrderSideSell {
		t.Errorf("USDⓈ-M side = %q, want sell", perpPage.Executions[0].Side)
	}

	// Spot, and it BUYS — the two fixtures differ in direction as well as in
	// spelling, so a codec answering one constant fails on one of them.
	spotBody := fmt.Sprintf(`[{"id":12,"orderId":992,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.0004","commissionAsset":"BNB","time":%d,"isBuyer":true}]`, at)
	spotPage, err := binanceCodec(t).DecodeExecutions(spotInstrument(testBinance), testAssets(), w, okResponse(spotBody))
	if err != nil {
		t.Fatalf("spot decode: %v", err)
	}
	if spotPage.Executions[0].Side != venue.OrderSideBuy {
		t.Errorf("spot side = %q, want buy", spotPage.Executions[0].Side)
	}

	// A spot SELL, so "isBuyer" is read rather than assumed. Without this the
	// spot arm would pass against a decoder that returned buy unconditionally.
	sellBody := fmt.Sprintf(`[{"id":13,"orderId":993,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d,"isBuyer":false}]`, at)
	sellPage, err := binanceCodec(t).DecodeExecutions(spotInstrument(testBinance), testAssets(), w, okResponse(sellBody))
	if err != nil {
		t.Fatalf("spot sell decode: %v", err)
	}
	if sellPage.Executions[0].Side != venue.OrderSideSell {
		t.Errorf("spot isBuyer=false side = %q, want sell", sellPage.Executions[0].Side)
	}
}

func TestAnExecutionStatingNoDirectionIsRefused(t *testing.T) {
	w := window48h()
	body := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"USDT","time":%d}]`, sweepStart.Add(time.Hour).UnixMilli())
	_, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
	if !errors.Is(err, venue.ErrExecutionsUnreadable) {
		t.Errorf("an execution with neither side nor isBuyer = %v, want ErrExecutionsUnreadable — the zero value names a real direction", err)
	}
}

func TestAFeeWithNoAssetIsRefused(t *testing.T) {
	w := window48h()
	at := sweepStart.Add(time.Hour).UnixMilli()

	binanceBody := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.02","commissionAsset":"","time":%d,"side":"SELL"}]`, at)
	if _, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(binanceBody)); !errors.Is(err, venue.ErrExecutionsUnreadable) {
		t.Errorf("binance fee with no commissionAsset = %v, want ErrExecutionsUnreadable", err)
	}

	bybitBody := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"execId":"e1","orderId":"o1","orderLinkId":"x",
		"symbol":"BTCUSDT","side":"Sell","execQty":"0.5","execPrice":"50100.25","execFee":"0.02",
		"feeCurrency":"","execTime":"%d"}],"nextPageCursor":""}}`, at)
	if _, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w, okResponse(bybitBody)); !errors.Is(err, venue.ErrExecutionsUnreadable) {
		t.Errorf("bybit fee with no feeCurrency = %v, want ErrExecutionsUnreadable", err)
	}
}

// TestBybitCarriesTheClientOrderIDAndBinanceDoesNot is finding 4 stated as a
// test: the two venues' execution surfaces are NOT symmetric in what they let a
// fill be attributed by.
func TestBybitCarriesTheClientOrderIDAndBinanceDoesNot(t *testing.T) {
	w := window48h()
	at := sweepStart.Add(time.Hour).UnixMilli()
	body := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"execId":"e1","orderId":"o1","orderLinkId":"OUR-ID",
		"symbol":"BTCUSDT","side":"Sell","execQty":"0.5","execPrice":"50100.25","execFee":"0.02",
		"feeCurrency":"USDT","execTime":"%d"}],"nextPageCursor":""}}`, at)

	page, err := bybitCodec(t).DecodeExecutions(perpInstrument(testBybit), testAssets(), w, okResponse(body))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	if page.Executions[0].ClientOrderID != "OUR-ID" {
		t.Errorf("bybit execution ClientOrderID = %q, want %q — Bybit reports orderLinkId and Binance reports none",
			page.Executions[0].ClientOrderID, "OUR-ID")
	}
}

// TestBybitReadsBothExecutionSides is C2a's §51g survivor arriving on the mirror
// path, and it is worth naming as such.
//
// That slice found binanceSide(OrderSideBuy) mutable-and-unnoticed on the WRITE
// path, fixed it with TestBinanceRendersBothSides, and the repo's standing rule
// is that a fix applied in one place gets applied to its mirror. The mirror
// checked then was the other VENUE. The mirror that was missed is the other
// DIRECTION of travel: every execution fixture in this file sells, because the
// flagship's perp leg sells, so bybitExecutionSide("Buy") was unreached and a
// mutant returning Sell for both survived. The spot leg of every cash-and-carry
// trade is a BUY, so the unreached arm is half of all real fills.
func TestBybitReadsBothExecutionSides(t *testing.T) {
	w := window48h()
	at := sweepStart.Add(time.Hour).UnixMilli()

	for _, tc := range []struct {
		wire string
		want venue.OrderSide
	}{
		{"Buy", venue.OrderSideBuy},
		{"Sell", venue.OrderSideSell},
	} {
		body := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"execId":"e1","orderId":"o1","orderLinkId":"OUR-ID",
			"symbol":"BTCUSDT","side":%q,"execQty":"0.5","execPrice":"50100.25","execFee":"0.02",
			"feeCurrency":"USDT","execTime":"%d"}],"nextPageCursor":""}}`, tc.wire, at)
		page, err := bybitCodec(t).DecodeExecutions(spotInstrument(testBybit), testAssets(), w, okResponse(body))
		if err != nil {
			t.Fatalf("%s: %v", tc.wire, err)
		}
		if got := page.Executions[0].Side; got != tc.want {
			t.Errorf("bybit execution side %q -> %q, want %q", tc.wire, got, tc.want)
		}
	}

	// And the spelling is Bybit's own title case, not Binance's upper — the
	// asymmetry OrderSide exists for, now on the read path.
	upper := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"execId":"e1","orderId":"o1","orderLinkId":"OUR-ID",
		"symbol":"BTCUSDT","side":"BUY","execQty":"0.5","execPrice":"50100.25","execFee":"0.02",
		"feeCurrency":"USDT","execTime":"%d"}],"nextPageCursor":""}}`, at)
	if _, err := bybitCodec(t).DecodeExecutions(spotInstrument(testBybit), testAssets(), w, okResponse(upper)); !errors.Is(err, venue.ErrOrderSideInvalid) {
		t.Errorf("bybit side \"BUY\" = %v, want ErrOrderSideInvalid — that is Binance's spelling", err)
	}
}

// ---------------------------------------------------------------------------
// The order-status surfaces
// ---------------------------------------------------------------------------

func TestBinanceDecodesOrderStatus(t *testing.T) {
	ref := perpRef(t, testBinance)
	body := fmt.Sprintf(`{"symbol":"BTCUSDT","orderId":991,"clientOrderId":%q,"status":"PARTIALLY_FILLED","executedQty":"0.2"}`, ref.ClientOrderID)

	got, err := binanceCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body))
	if err != nil {
		t.Fatalf("DecodeOrderStatus: %v", err)
	}
	if got.Presence != venue.OrderPresent {
		t.Errorf("presence = %q, want present", got.Presence)
	}
	if got.State != venue.OrderStatePartiallyFilled {
		t.Errorf("state = %q, want partially_filled", got.State)
	}
	// §6.7 step 2's "authoritative final cumulative executed quantity". 0.2 of
	// 0.5, so neither zero nor the whole order — a decoder reading the wrong
	// field would have to produce exactly this to pass.
	if got.ExecutedQuantity.Decimal() != "0.2" {
		t.Errorf("executed = %q, want 0.2", got.ExecutedQuantity.Decimal())
	}
	if got.VenueOrderID != "991" {
		t.Errorf("venue order id = %q, want 991", got.VenueOrderID)
	}
}

func TestOrderStatusRefusesAnEchoOfAnotherOrder(t *testing.T) {
	ref := perpRef(t, testBinance)
	body := `{"symbol":"BTCUSDT","orderId":991,"clientOrderId":"SOMEBODY-ELSES","status":"FILLED","executedQty":"0.5"}`
	if _, err := binanceCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body)); !errors.Is(err, venue.ErrAckUnreadable) {
		t.Errorf("a status for another order = %v, want ErrAckUnreadable", err)
	}

	bybitRef := perpRef(t, testBybit)
	bybitBody := `{"retCode":0,"result":{"list":[{"orderId":"o1","orderLinkId":"SOMEBODY-ELSES","symbol":"BTCUSDT","orderStatus":"Filled","cumExecQty":"0.5"}]}}`
	if _, err := bybitCodec(t).DecodeOrderStatus(bybitRef, testAssets(), venue.SurfaceOrderStatus, okResponse(bybitBody)); !errors.Is(err, venue.ErrAckUnreadable) {
		t.Errorf("bybit status for another order = %v, want ErrAckUnreadable", err)
	}
}

func TestBybitRefusesMoreThanOneOrderForAPointLookup(t *testing.T) {
	ref := perpRef(t, testBybit)
	body := fmt.Sprintf(`{"retCode":0,"result":{"list":[
		{"orderId":"o1","orderLinkId":%q,"symbol":"BTCUSDT","orderStatus":"Filled","cumExecQty":"0.5"},
		{"orderId":"o2","orderLinkId":%q,"symbol":"BTCUSDT","orderStatus":"New","cumExecQty":"0"}]}}`,
		ref.ClientOrderID, ref.ClientOrderID)
	if _, err := bybitCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body)); !errors.Is(err, venue.ErrAckUnreadable) {
		t.Errorf("two rows for one client id = %v, want ErrAckUnreadable", err)
	}
}

// TestATerminatedBybitOrderStillReportsItsExecutions — "in derivatives, orders
// with this status may have an executed qty", and spot has
// PartiallyFilledCanceled. An implementation shortcutting a non-Filled terminal
// to zero loses real fills on this system's ordinary outcome.
func TestATerminatedBybitOrderStillReportsItsExecutions(t *testing.T) {
	ref := perpRef(t, testBybit)
	for _, status := range []string{"Cancelled", "PartiallyFilledCanceled", "Deactivated"} {
		body := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"orderId":"o1","orderLinkId":%q,"symbol":"BTCUSDT","orderStatus":%q,"cumExecQty":"0.3"}]}}`,
			ref.ClientOrderID, status)
		got, err := bybitCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body))
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if got.State != venue.OrderStateTerminated {
			t.Errorf("%s -> %q, want terminated", status, got.State)
		}
		if got.ExecutedQuantity.Decimal() != "0.3" {
			t.Errorf("%s executed = %q, want 0.3 — a terminal Bybit order can carry fills", status, got.ExecutedQuantity.Decimal())
		}
	}
}

// TestTheSameEventNormalizesTheSameWayOnBothVenues is why OrderStateTerminated
// exists instead of a Canceled/Expired pair: an unfilled marketable IOC is
// EXPIRED on Binance and Cancelled on Bybit, and downstream must not be able to
// tell which venue answered.
func TestTheSameEventNormalizesTheSameWayOnBothVenues(t *testing.T) {
	binanceRef := perpRef(t, testBinance)
	binanceBody := fmt.Sprintf(`{"symbol":"BTCUSDT","orderId":991,"clientOrderId":%q,"status":"EXPIRED","executedQty":"0"}`, binanceRef.ClientOrderID)
	binanceGot, err := binanceCodec(t).DecodeOrderStatus(binanceRef, testAssets(), venue.SurfaceOrderStatus, okResponse(binanceBody))
	if err != nil {
		t.Fatalf("binance: %v", err)
	}

	bybitRef := perpRef(t, testBybit)
	bybitBody := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"orderId":"o1","orderLinkId":%q,"symbol":"BTCUSDT","orderStatus":"Cancelled","cumExecQty":"0"}]}}`, bybitRef.ClientOrderID)
	bybitGot, err := bybitCodec(t).DecodeOrderStatus(bybitRef, testAssets(), venue.SurfaceOrderStatus, okResponse(bybitBody))
	if err != nil {
		t.Fatalf("bybit: %v", err)
	}

	if binanceGot.State != bybitGot.State {
		t.Errorf("an unfilled IOC is %q on binance and %q on bybit; the ACL must erase that difference",
			binanceGot.State, bybitGot.State)
	}
	if binanceGot.State != venue.OrderStateTerminated {
		t.Errorf("state = %q, want terminated", binanceGot.State)
	}
}

// TestBybitConditionalStatesAreNotTerminal checks the direction the codec's
// comment argues for and nothing was verifying.
//
// Untriggered and Triggered are conditional-order states this system cannot
// create — §6 step 6 sends IOC limits and nothing else — and they are mapped
// rather than refused, in the direction that is safe if the impossible happens.
// Which direction that is, is the whole claim: NOT terminal, so §6.7 keeps the
// possible-delta interval open instead of collapsing one that has not collapsed.
// A mutant reading them as terminal survived until this existed, because a
// prose statement of a direction is not a check of it.
func TestBybitConditionalStatesAreNotTerminal(t *testing.T) {
	ref := perpRef(t, testBybit)
	for _, status := range []string{"Untriggered", "Triggered"} {
		body := fmt.Sprintf(`{"retCode":0,"result":{"list":[{"orderId":"o1","orderLinkId":%q,"symbol":"BTCUSDT","orderStatus":%q,"cumExecQty":"0"}]}}`,
			ref.ClientOrderID, status)
		got, err := bybitCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body))
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		terminal, err := got.State.Terminal()
		if err != nil {
			t.Fatalf("%s Terminal(): %v", status, err)
		}
		if terminal {
			t.Errorf("%s -> %q, which is TERMINAL; §6.7 would collapse an interval that has not collapsed",
				status, got.State)
		}
	}
}

func TestAnUnrecognisedStatusIsRefusedRatherThanDefaulted(t *testing.T) {
	ref := perpRef(t, testBinance)
	body := fmt.Sprintf(`{"symbol":"BTCUSDT","orderId":991,"clientOrderId":%q,"status":"SOME_NEW_STATUS","executedQty":"0"}`, ref.ClientOrderID)
	if _, err := binanceCodec(t).DecodeOrderStatus(ref, testAssets(), venue.SurfaceOrderStatus, okResponse(body)); !errors.Is(err, venue.ErrOrderStateInvalid) {
		t.Errorf("an unknown status = %v, want ErrOrderStateInvalid — a default arm would silently make it terminal", err)
	}
}

func TestOrderStateTerminalIsExhaustive(t *testing.T) {
	terminal := map[venue.OrderState]bool{
		venue.OrderStateOpen:            false,
		venue.OrderStatePartiallyFilled: false,
		venue.OrderStateFilled:          true,
		venue.OrderStateTerminated:      true,
		venue.OrderStateRejected:        true,
	}
	for state, want := range terminal {
		got, err := state.Terminal()
		if err != nil {
			t.Fatalf("%s.Terminal(): %v", state, err)
		}
		if got != want {
			t.Errorf("%s.Terminal() = %v, want %v", state, got, want)
		}
	}
	if _, err := venue.OrderState("invented").Terminal(); !errors.Is(err, venue.ErrOrderStateInvalid) {
		t.Errorf("an undeclared state = %v, want ErrOrderStateInvalid", err)
	}
}

// ---------------------------------------------------------------------------
// §5.3's normative Bybit fallback
// ---------------------------------------------------------------------------

func TestBybitFallsBackFromRealtimeToHistoryAndBinanceHasNoFallback(t *testing.T) {
	next, ok := bybitCodec(t).FallbackSurface(venue.SurfaceOrderStatus)
	if !ok || next != venue.SurfaceOrderHistory {
		t.Errorf("bybit fallback from realtime = (%q, %v), want (%q, true) — §5.3 makes it MUST",
			next, ok, venue.SurfaceOrderHistory)
	}
	if _, ok := bybitCodec(t).FallbackSurface(venue.SurfaceOrderHistory); ok {
		t.Error("bybit reports a fallback after history; there is no third surface")
	}
	if _, ok := binanceCodec(t).FallbackSurface(venue.SurfaceOrderStatus); ok {
		t.Error("binance reports a fallback; it has one order surface per family")
	}
}

func TestTheTwoBybitSurfacesUseDifferentPaths(t *testing.T) {
	codec := bybitCodec(t)
	ref := perpRef(t, testBybit)

	realtime, err := codec.EncodeOrderStatus(ref, venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("realtime encode: %v", err)
	}
	history, err := codec.EncodeOrderStatus(ref, venue.SurfaceOrderHistory)
	if err != nil {
		t.Fatalf("history encode: %v", err)
	}
	if realtime.Path != "/v5/order/realtime" || history.Path != "/v5/order/history" {
		t.Errorf("paths = %q and %q, want /v5/order/realtime and /v5/order/history", realtime.Path, history.Path)
	}
}

func TestBinanceRefusesASurfaceItDoesNotHave(t *testing.T) {
	ref := perpRef(t, testBinance)
	if _, err := binanceCodec(t).EncodeOrderStatus(ref, venue.SurfaceOrderHistory); !errors.Is(err, venue.ErrQuerySurfaceInvalid) {
		t.Errorf("binance history encode = %v, want ErrQuerySurfaceInvalid", err)
	}
	if _, err := binanceCodec(t).StatusReach(perpInstrument(testBinance), venue.SurfaceOrderHistory); !errors.Is(err, venue.ErrQuerySurfaceInvalid) {
		t.Errorf("binance history reach = %v, want ErrQuerySurfaceInvalid", err)
	}
}

// ---------------------------------------------------------------------------
// Costs, which differ per family on Binance
// ---------------------------------------------------------------------------

func TestBinanceReadCostsDifferPerFamily(t *testing.T) {
	codec := binanceCodec(t)

	perpStatus, err := codec.EncodeOrderStatus(perpRef(t, testBinance), venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("perp status encode: %v", err)
	}
	spotStatus, err := codec.EncodeOrderStatus(spotRef(t, testBinance), venue.SurfaceOrderStatus)
	if err != nil {
		t.Fatalf("spot status encode: %v", err)
	}
	// "IP Weight 1" against "Weight: 4".
	if perpStatus.Cost.Weight != 1 || spotStatus.Cost.Weight != 4 {
		t.Errorf("order-status weights = %d (perp) and %d (spot), want 1 and 4",
			perpStatus.Cost.Weight, spotStatus.Cost.Weight)
	}
	if perpStatus.Path != "/fapi/v1/order" || spotStatus.Path != "/api/v3/order" {
		t.Errorf("paths = %q and %q", perpStatus.Path, spotStatus.Path)
	}

	w := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(time.Hour)}
	perpSweep := mustEncodeExecutions(t, codec, perpInstrument(testBinance), w)
	spotSweep := mustEncodeExecutions(t, codec, spotInstrument(testBinance), w)
	// "Weight 5" against the conditional "Without orderId | 20". This sweep is
	// per-instrument and never carries an orderId, so 20 is the honest charge —
	// charging 5 would under-count the most expensive read by four against the
	// budget that bans the IP.
	if perpSweep.Cost.Weight != 5 || spotSweep.Cost.Weight != 20 {
		t.Errorf("executions weights = %d (perp) and %d (spot), want 5 and 20",
			perpSweep.Cost.Weight, spotSweep.Cost.Weight)
	}
}

// TestNoReadChargesAnOrderSlot — §7.6 meters order counts on the ORDER budget,
// and a read that charged one would spend the budget that bans an account for
// placing too many orders, on a call that places none.
func TestNoReadChargesAnOrderSlot(t *testing.T) {
	w := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(time.Hour)}
	calls := []transport.Call{
		mustEncodeOrderStatus(t, binanceCodec(t), perpRef(t, testBinance), venue.SurfaceOrderStatus),
		mustEncodeOrderStatus(t, bybitCodec(t), perpRef(t, testBybit), venue.SurfaceOrderStatus),
		mustEncodeOrderStatus(t, bybitCodec(t), perpRef(t, testBybit), venue.SurfaceOrderHistory),
		mustEncodeExecutions(t, binanceCodec(t), perpInstrument(testBinance), w),
		mustEncodeExecutions(t, bybitCodec(t), perpInstrument(testBybit), w),
	}
	for i, call := range calls {
		if call.Cost.Orders != 0 {
			t.Errorf("read call %d (%s) charges %d order slots, want 0", i, call.Op, call.Cost.Orders)
		}
	}
}

// TestBybitReadCostsAreRequestOnlyAndValid pins the defect #173's review found
// LIVE here: Bybit's read surfaces truthfully charge nothing but the implicit
// request, that truth was spelled as the zero Cost, and SignedClient.Do
// validates the cost BEFORE Acquire — so every Bybit read failed at the
// precondition while this file's builder tests all passed. The two assertions
// are the two halves: the cost must carry the request-only claim (not the
// unpopulated zero), and it must survive the validation Do actually runs.
func TestBybitReadCostsAreRequestOnlyAndValid(t *testing.T) {
	w := venue.ExecutionWindow{Since: venue.Cursor{At: sweepStart}, Until: sweepStart.Add(time.Hour)}
	positions, err := bybitCodec(t).EncodePositions(perpScope())
	if err != nil {
		t.Fatalf("EncodePositions: %v", err)
	}
	calls := []transport.Call{
		mustEncodeOrderStatus(t, bybitCodec(t), perpRef(t, testBybit), venue.SurfaceOrderStatus),
		mustEncodeOrderStatus(t, bybitCodec(t), perpRef(t, testBybit), venue.SurfaceOrderHistory),
		mustEncodeExecutions(t, bybitCodec(t), perpInstrument(testBybit), w),
		positions,
	}
	for _, call := range calls {
		if call.Cost != ratelimit.RequestOnly() {
			t.Errorf("%s cost = %+v, want RequestOnly() — the zero Cost is the field nobody filled in", call.Op, call.Cost)
		}
		if err := call.Cost.Validate(); err != nil {
			t.Errorf("%s cost fails the validation Do runs before Acquire: %v", call.Op, err)
		}
	}
}

func mustEncodeOrderStatus(t *testing.T, c venue.OrderQueryCodec, ref venue.OrderRef, s venue.QuerySurface) transport.Call {
	t.Helper()
	call, err := c.EncodeOrderStatus(ref, s)
	if err != nil {
		t.Fatalf("EncodeOrderStatus: %v", err)
	}
	return call
}

// ---------------------------------------------------------------------------
// InstrumentAssets
// ---------------------------------------------------------------------------

func TestInstrumentAssetsRefuseTheUnpopulatedAndTheIdentical(t *testing.T) {
	if err := (venue.InstrumentAssets{Quote: "USDT"}).Validate(); !errors.Is(err, venue.ErrInstrumentAssetsUnset) {
		t.Errorf("missing base = %v, want ErrInstrumentAssetsUnset", err)
	}
	if err := (venue.InstrumentAssets{Base: "BTC"}).Validate(); !errors.Is(err, venue.ErrInstrumentAssetsUnset) {
		t.Errorf("missing quote = %v, want ErrInstrumentAssetsUnset", err)
	}
	if err := (venue.InstrumentAssets{Base: "BTC", Quote: "BTC"}).Validate(); !errors.Is(err, venue.ErrInstrumentAssetsIdentical) {
		t.Errorf("base == quote = %v, want ErrInstrumentAssetsIdentical", err)
	}
	if err := testAssets().Validate(); err != nil {
		t.Errorf("BTC/USDT = %v, want accepted — the positive control", err)
	}
}

// TestTheDecodedMoneyCarriesTheAssetsItWasGiven — the read path MINTS money, so
// a wrong tag here is a Money the kernel's runtime check rejects somewhere else.
func TestTheDecodedMoneyCarriesTheAssetsItWasGiven(t *testing.T) {
	w := window48h()
	body := fmt.Sprintf(`[{"id":11,"orderId":991,"symbol":"BTCUSDT","price":"50100.25","qty":"0.5",
		"commission":"0.0004","commissionAsset":"BNB","time":%d,"side":"SELL"}]`,
		sweepStart.Add(time.Hour).UnixMilli())
	page, err := binanceCodec(t).DecodeExecutions(perpInstrument(testBinance), testAssets(), w, okResponse(body))
	if err != nil {
		t.Fatalf("DecodeExecutions: %v", err)
	}
	e := page.Executions[0]

	// The fee's asset is the VENUE's, not the instrument's — a Binance fill can
	// be paid in BNB against a USDT notional, which is the case §5.1 cites for
	// having no separate FeeAsset column.
	if got := e.Fee.Asset(); got != fixedpoint.Asset("BNB") {
		t.Errorf("fee asset = %q, want BNB — it comes from commissionAsset, not from the quote asset", got)
	}
	if got := e.Quantity.Asset(); got != fixedpoint.Asset("BTC") {
		t.Errorf("quantity asset = %q, want BTC", got)
	}
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// bybitFullDescendingPage is 100 rows — the venue's published page limit, so the
// page is FULL — listed newest first as Bybit lists them, with the two OLDEST
// rows sharing one execTime so the tiebreak is reachable.
func bybitFullDescendingPage(start time.Time) string {
	rows := make([]string, 0, bybitTestPageLimit)
	for i := bybitTestPageLimit - 1; i >= 0; i-- {
		// Newest first: row i is at start + i minutes, emitted descending.
		offset := time.Duration(i) * time.Minute
		if i <= 1 {
			// The two oldest share one instant. Their execIds are emitted in
			// DESCENDING order ("e01" after "e00" would already be ascending),
			// so a decoder that kept the venue's order would fail the tiebreak.
			offset = 0
		}
		rows = append(rows, fmt.Sprintf(`{"execId":"e%02d","orderId":"o1","orderLinkId":"OUR-ID",
			"symbol":"BTCUSDT","side":"Sell","execQty":"0.001","execPrice":"50100.25","execFee":"0.02",
			"feeCurrency":"USDT","execTime":"%d"}`, i, start.Add(offset).UnixMilli()))
	}
	return fmt.Sprintf(`{"retCode":0,"result":{"list":[%s],"nextPageCursor":"page-2"}}`, joinRows(rows))
}

const bybitTestPageLimit = 100

// binanceFullPage is 1000 rows — Binance's page limit — ascending, with the last
// two sharing one instant so the inclusive rewind is observable.
func binanceFullPage(start time.Time) string {
	rows := make([]string, 0, binanceTestPageLimit)
	for i := 0; i < binanceTestPageLimit; i++ {
		offset := time.Duration(i) * time.Minute
		if i >= binanceTestPageLimit-2 {
			offset = time.Duration(binanceTestPageLimit-2) * time.Minute
		}
		rows = append(rows, fmt.Sprintf(`{"id":%d,"orderId":991,"symbol":"BTCUSDT","price":"50100.25",
			"qty":"0.001","commission":"0.01","commissionAsset":"USDT","time":%d,"side":"SELL"}`,
			i, start.Add(offset).UnixMilli()))
	}
	return "[" + joinRows(rows) + "]"
}

const binanceTestPageLimit = 1000

func joinRows(rows []string) string {
	out := ""
	for i, r := range rows {
		if i > 0 {
			out += ","
		}
		out += r
	}
	return out
}
