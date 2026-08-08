package venue_test

import (
	"errors"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
)

// This file checks core §7.1's margin-ACT history surface, venue-side (task
// #214).
//
// # The collapse checklist, run before the assertions were written
//
//   - THE TWO ROWS IN THE MULTI-ROW FIXTURE DIFFER IN EVERY FIELD — amount,
//     time, cause and direction. With any of them shared, a decoder that
//     returned the first row twice, or that read one row's field off its
//     neighbour, mints the same slice.
//   - THE DIRECTION FIXTURE CARRIES BOTH `type` VALUES, because with only
//     `type:1` present "1 means add" and "anything means add" are the same rule.
//   - `deltaType` IS FIXTURED AT THREE VALUES — the verified `USER_ADJUST`, a
//     plausible other one, and the EMPTY string. Two would leave "recognised vs
//     not" and "present vs absent" as the same distinction.
//   - THE AMOUNTS ARE NOT ROUND AND NOT EQUAL TO THE TIMESTAMPS' LOW DIGITS. The
//     row's `amount` is a decimal string and its `time` an integer; a fixture
//     where they render alike would let a field-order slip mint the same struct.
//   - THE PAGE-FULL FIXTURE IS AT EXACTLY THE LIMIT, not far past it, because
//     `>=` and `>` are the same predicate on any other count.
//   - THE ENCODE FIXTURE'S WINDOW EDGES ARE DIFFERENT INSTANTS, so a swapped
//     start/end is visible in the rendered parameters rather than symmetric.

const (
	// The verified row shape, from Binance's spec-generated connector's own
	// mocked response for this endpoint (Profiles §4).
	actAmountA = "23.36332311"
	actAmountB = "41.07584219" // no trailing zeros: Decimal() normalizes them away
	actTimeA   = 1578047897183
	actTimeB   = 1578047899471
)

func actWindow() venue.MarginActWindow {
	return venue.MarginActWindow{
		From: time.UnixMilli(1578047890000).UTC(),
		To:   time.UnixMilli(1578047920000).UTC(),
	}
}

// binanceActRow renders one history row with every field Binance publishes.
func binanceActRow(symbol string, typ int, deltaType, amount string, at int64, side string) string {
	return `{"symbol":"` + symbol + `",` +
		`"type":` + itoa(typ) + `,` +
		`"deltaType":"` + deltaType + `",` +
		`"amount":"` + amount + `",` +
		`"asset":"USDT",` +
		`"time":` + i64toa(at) + `,` +
		`"positionSide":"` + side + `"}`
}

func itoa(v int) string { return i64toa(int64(v)) }
func i64toa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestBinanceEncodeMarginActsRendersTheWindowAndDirection(t *testing.T) {
	instrument := perpInstrument(testBinance)
	w := actWindow()

	for _, tc := range []struct {
		name     string
		adds     bool
		wantType string
	}{
		{"a top-up asks for type 1", true, "1"},
		{"a reduction asks for type 2", false, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call, err := binanceCodec(t).EncodeMarginActs(instrument, w, tc.adds)
			if err != nil {
				t.Fatalf("EncodeMarginActs: %v", err)
			}
			if call.Path != "/fapi/v1/positionMargin/history" {
				t.Errorf("path = %q", call.Path)
			}
			if call.Method != "GET" {
				t.Errorf("method = %q, want GET", call.Method)
			}

			params := map[string]string{}
			for _, p := range call.Params {
				params[p.Key] = p.Value
			}
			// startTime and endTime are asserted to the exact rendered
			// milliseconds rather than "is present": a swapped pair is a valid
			// call that fetches an inverted window, which returns no rows and no
			// error — the reading that turns a correct top-up into an incident.
			want := map[string]string{
				"symbol":    "BTCUSDT",
				"type":      tc.wantType,
				"startTime": "1578047890000",
				"endTime":   "1578047920000",
				"limit":     "500",
			}
			for k, v := range want {
				if params[k] != v {
					t.Errorf("param %s = %q, want %q", k, params[k], v)
				}
			}
			if call.Cost.Weight == 0 {
				t.Error("the call is charged nothing")
			}
		})
	}
}

func TestBinanceEncodeMarginActsRefusals(t *testing.T) {
	c := binanceCodec(t)

	t.Run("spot has no surface", func(t *testing.T) {
		if _, err := c.EncodeMarginActs(spotInstrument(testBinance), actWindow(), true); !errors.Is(err, venue.ErrPositionSurfaceAbsent) {
			t.Fatalf("EncodeMarginActs(spot): got %v, want %v", err, venue.ErrPositionSurfaceAbsent)
		}
	})

	t.Run("an inverted or empty window is refused", func(t *testing.T) {
		w := actWindow()
		for _, bad := range []venue.MarginActWindow{
			{From: w.To, To: w.From},        // inverted
			{From: w.From, To: w.From},      // empty
			{From: time.Time{}, To: w.To},   // unset open edge
			{From: w.From, To: time.Time{}}, // unset close edge
		} {
			if _, err := c.EncodeMarginActs(perpInstrument(testBinance), bad, true); !errors.Is(err, venue.ErrMarginActWindowInvalid) {
				t.Errorf("EncodeMarginActs(%v..%v): got %v, want %v", bad.From, bad.To, err, venue.ErrMarginActWindowInvalid)
			}
		}
	})

	// The positive control: without it every refusal above is satisfied by an
	// encoder that refuses everything.
	if _, err := c.EncodeMarginActs(perpInstrument(testBinance), actWindow(), true); err != nil {
		t.Fatalf("EncodeMarginActs on a well-formed read: %v", err)
	}
}

func TestBinanceDecodeMarginActsReadsEveryFieldOfTwoDIFFERENTRows(t *testing.T) {
	body := `[` +
		binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, actTimeA, "BOTH") + `,` +
		binanceActRow("BTCUSDT", 2, "AUTO_DELEVERAGE", actAmountB, actTimeB, "BOTH") +
		`]`

	acts, err := binanceCodec(t).DecodeMarginActs(perpInstrument(testBinance), marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginActs: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("decoded %d rows, want 2", len(acts))
	}

	// Row 0: an ADD, recognised cause, the earlier instant, the smaller amount.
	if acts[0].Amount.Decimal() != actAmountA {
		t.Errorf("row 0 amount = %s, want %s", acts[0].Amount.Decimal(), actAmountA)
	}
	if acts[0].Amount.Asset() != "USDT" {
		t.Errorf("row 0 asset = %s, want USDT", acts[0].Amount.Asset())
	}
	if !acts[0].Adds {
		t.Error("row 0 (type 1) decoded as a reduction")
	}
	if !acts[0].At.Equal(time.UnixMilli(actTimeA).UTC()) {
		t.Errorf("row 0 at = %v, want %v", acts[0].At, time.UnixMilli(actTimeA).UTC())
	}
	if acts[0].Cause != venue.MarginActCauseUserAdjust {
		t.Errorf("row 0 cause = %s, want %s", acts[0].Cause, venue.MarginActCauseUserAdjust)
	}

	// Row 1: a REDUCE, an unrecognised cause, the later instant, the larger
	// amount. Every field differs from row 0's, so a decoder that duplicated a
	// row or read across rows fails here rather than agreeing by coincidence.
	if acts[1].Amount.Decimal() != actAmountB {
		t.Errorf("row 1 amount = %s, want %s", acts[1].Amount.Decimal(), actAmountB)
	}
	if acts[1].Adds {
		t.Error("row 1 (type 2) decoded as a top-up")
	}
	if !acts[1].At.Equal(time.UnixMilli(actTimeB).UTC()) {
		t.Errorf("row 1 at = %v, want %v", acts[1].At, time.UnixMilli(actTimeB).UTC())
	}
	if acts[1].Cause != venue.MarginActCauseOther {
		t.Errorf("row 1 cause = %s, want %s", acts[1].Cause, venue.MarginActCauseOther)
	}
}

// TestAnUNRECOGNISEDDeltaTypeIsREPORTEDNotDropped is the finding this slice
// added to §7.1's stated match.
//
// The section words the match as "symbol + type + amount + time-window", which
// says nothing about WHY the venue moved the margin — and §4.1's posture check
// reads `isAutoAddMargin` per configured symbol precisely because the venue can
// add margin on its own. A decoder that dropped rows it could not classify would
// hide such an act behind a verdict computed from the weaker source.
//
// The empty string is fixtured beside a plausible unknown, because "unrecognised"
// and "absent" are different inputs that must reach the same conservative member.
func TestAnUNRECOGNISEDDeltaTypeIsREPORTEDNotDropped(t *testing.T) {
	for _, deltaType := range []string{"AUTO_DELEVERAGE", "", "user_adjust", "USER_ADJUST_V2"} {
		body := `[` + binanceActRow("BTCUSDT", 1, deltaType, actAmountA, actTimeA, "BOTH") + `]`
		acts, err := binanceCodec(t).DecodeMarginActs(perpInstrument(testBinance), marginAssets(), okResponse(body))
		if err != nil {
			t.Fatalf("DecodeMarginActs(deltaType %q): %v", deltaType, err)
		}
		if len(acts) != 1 {
			t.Fatalf("deltaType %q: decoded %d rows, want 1 — the row was DROPPED", deltaType, len(acts))
		}
		if acts[0].Cause != venue.MarginActCauseOther {
			t.Errorf("deltaType %q: cause = %s, want %s", deltaType, acts[0].Cause, venue.MarginActCauseOther)
		}
	}

	// The positive control, and it is what makes the four cases above mean
	// "not recognised" rather than "nothing is ever recognised".
	body := `[` + binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, actTimeA, "BOTH") + `]`
	acts, err := binanceCodec(t).DecodeMarginActs(perpInstrument(testBinance), marginAssets(), okResponse(body))
	if err != nil {
		t.Fatalf("DecodeMarginActs: %v", err)
	}
	if acts[0].Cause != venue.MarginActCauseUserAdjust {
		t.Errorf("the verified deltaType: cause = %s, want %s", acts[0].Cause, venue.MarginActCauseUserAdjust)
	}
}

func TestBinanceDecodeMarginActsRefusals(t *testing.T) {
	c := binanceCodec(t)
	instrument := perpInstrument(testBinance)

	cases := []struct {
		name string
		body string
		want error
	}{
		{
			name: "a hedge-mode row",
			body: `[` + binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, actTimeA, "LONG") + `]`,
			want: venue.ErrPositionModeNotOneWay,
		},
		{
			name: "a row in a foreign asset",
			body: `[{"symbol":"BTCUSDT","type":1,"deltaType":"USER_ADJUST","amount":"` + actAmountA +
				`","asset":"USDC","time":` + i64toa(actTimeA) + `,"positionSide":"BOTH"}]`,
			want: venue.ErrMarginAssetsDisagree,
		},
		{
			name: "an undeclared direction",
			body: `[` + binanceActRow("BTCUSDT", 3, "USER_ADJUST", actAmountA, actTimeA, "BOTH") + `]`,
			want: venue.ErrMarginActUnreadable,
		},
		{
			name: "a zero timestamp",
			body: `[` + binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, 0, "BOTH") + `]`,
			want: venue.ErrMarginActUnreadable,
		},
		{
			name: "a negative timestamp",
			body: `[` + binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, -1, "BOTH") + `]`,
			want: venue.ErrMarginActUnreadable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.DecodeMarginActs(instrument, marginAssets(), okResponse(tc.body)); !errors.Is(err, tc.want) {
				t.Fatalf("DecodeMarginActs: got %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("an empty answer is not an error", func(t *testing.T) {
		// The venue was asked and recorded nothing. That is EVIDENCE, and
		// reporting it as a failure would delete the observation that separates
		// NotApplied from Ambiguous.
		acts, err := c.DecodeMarginActs(instrument, marginAssets(), okResponse(`[]`))
		if err != nil {
			t.Fatalf("DecodeMarginActs([]): %v", err)
		}
		if len(acts) != 0 {
			t.Errorf("decoded %d rows from an empty answer", len(acts))
		}
	})
}

// TestAFullPageIsRefused pins `>=` at exactly the limit.
//
// A truncated page can only UNDERCOUNT matching rows, which turns §7.1's
// Ambiguous duplicate case into a single-row Applied — a false "it landed".
//
// The fixture sits at exactly 500 and its control at 499, because at any other
// pair `>=` and `>` are the same predicate.
func TestAFullPageIsRefused(t *testing.T) {
	rows := func(n int) string {
		body := "["
		for i := 0; i < n; i++ {
			if i > 0 {
				body += ","
			}
			body += binanceActRow("BTCUSDT", 1, "USER_ADJUST", actAmountA, actTimeA+int64(i), "BOTH")
		}
		return body + "]"
	}

	c := binanceCodec(t)
	instrument := perpInstrument(testBinance)

	if _, err := c.DecodeMarginActs(instrument, marginAssets(), okResponse(rows(500))); !errors.Is(err, venue.ErrMarginActPageFull) {
		t.Fatalf("DecodeMarginActs(500 rows): got %v, want %v", err, venue.ErrMarginActPageFull)
	}
	acts, err := c.DecodeMarginActs(instrument, marginAssets(), okResponse(rows(499)))
	if err != nil {
		t.Fatalf("DecodeMarginActs(499 rows): %v", err)
	}
	if len(acts) != 499 {
		t.Errorf("decoded %d rows, want 499", len(acts))
	}
}

// TestBybitDeclaresNoActHistory is the ruling, asserted rather than assumed.
//
// Profiles §4: the V5 transaction-log's `type` enum has no value for a manual
// position-margin adjustment, and whether a top-up emits a record at all is
// undocumented and MUST NOT be relied on.
//
// The two refusing methods matter as much as the false: they are unreachable
// through OrderClient.MarginActs, and a later caller reaching them directly must
// get an error rather than an empty slice — because an empty slice from a venue
// that publishes a history is the observation that turns a correct top-up into
// an operator incident.
func TestBybitDeclaresNoActHistory(t *testing.T) {
	c := bybitCodec(t)
	instrument := perpInstrument(testBybit)

	if c.PublishesMarginActs() {
		t.Fatal("bybit claims a margin-act history")
	}
	if _, err := c.EncodeMarginActs(instrument, actWindow(), true); !errors.Is(err, venue.ErrMarginActSurfaceAbsent) {
		t.Errorf("EncodeMarginActs: got %v, want %v", err, venue.ErrMarginActSurfaceAbsent)
	}
	acts, err := c.DecodeMarginActs(instrument, marginAssets(), okResponse(`[]`))
	if !errors.Is(err, venue.ErrMarginActSurfaceAbsent) {
		t.Errorf("DecodeMarginActs: got %v, want %v", err, venue.ErrMarginActSurfaceAbsent)
	}
	if acts != nil {
		t.Errorf("DecodeMarginActs returned %d rows beside its refusal", len(acts))
	}

	// The positive control: the OTHER venue answers true, so the false above is
	// this codec's answer rather than a method that always refuses.
	if !binanceCodec(t).PublishesMarginActs() {
		t.Error("binance claims no margin-act history")
	}
}
