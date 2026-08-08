package venue_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks core §7.1's margin TOP-UP surface, venue-side (task #215).
//
// # The collapse checklist, run before the assertions were written
//
//   - THE ECHOED AMOUNT IS FIXTURED AT THREE VALUES, not two: the amount asked
//     for, the SAME amount written with a different number of trailing digits,
//     and a different amount. With only the first and third, "compares decimal
//     VALUES" and "compares the response text to our own rendering" are the same
//     rule — and the second is the one a real venue sends, since Binance
//     publishes this field as a JSON number.
//   - `type` IS FIXTURED AT BOTH DIRECTIONS. With only `1` present, "type 1 is an
//     add" and "any type is an acknowledgement" give the same answer.
//   - BYBIT'S THREE ECHO CHECKS ARE BROKEN ONE AT A TIME — category, symbol,
//     positionIdx — because a fixture breaking two makes `||` and `&&` the same
//     predicate.
//   - THE WRONG SYMBOL IS A DIFFERENT MARKET (ETHUSDT), not a case variant, and
//     the wrong category is the other real one (`spot`), so neither refusal can
//     pass on a normalization the codec does not perform.
//   - THE AMOUNT IS NOT ROUND AND SHARES NO DIGITS WITH THE POSITION FIGURES in
//     the Bybit snapshot, so a decoder reading `positionIM` where it meant the
//     request's amount cannot coincide.
//   - THE ONE-WAY INDEX IS ASSERTED AS RENDERED BYTES, `"positionIdx":0`, rather
//     than by decoding the body. The value that must be sent is ZERO, which is
//     exactly what `omitempty` drops, so a check that round-trips through a
//     struct would agree with a body that never carried the field.
//   - EVERY PRE-SEND REFUSAL ASSERTS THE HOST SAW NOTHING. On a read surface that
//     would be tidiness; here the thing that must not have happened is a margin
//     transfer.

const (
	// The top-up: not round, and no digit run shared with the Bybit snapshot's
	// figures below.
	topUpAmount = "137.45"

	// The same VALUE with a trailing zero — what a JSON number can legitimately
	// come back as, and what separates a decimal comparison from a string one.
	topUpAmountPadded = "137.450"

	// Two different amounts, one on EACH side of the asked-for figure. One alone
	// leaves `!= 0` and whichever inequality happens to agree with it as the
	// same predicate — and both directions are real: a venue that applied MORE
	// than we asked and one that applied less are equally unattributable.
	topUpAmountOver  = "137.54"
	topUpAmountUnder = "137.35"
)

func topUp(t *testing.T, decimal string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney("USDT", decimal)
	if err != nil {
		t.Fatalf("ParseMoney(%q): %v", decimal, err)
	}
	return m
}

// binanceTopUpAck renders the acknowledgement Profiles §4 records verbatim:
// `{amount, code, msg, type}`, with `amount` as a JSON NUMBER.
func binanceTopUpAck(amount string, typ int) string {
	return `{"amount":` + amount + `,` +
		`"code":200,` +
		`"msg":"Successfully modify position margin.",` +
		`"type":` + itoa(typ) + `}`
}

// bybitTopUpAck renders Bybit's acknowledgement: a position snapshot.
//
// `positionIM` and `positionMM` are present and deliberately unlike the top-up
// amount — this codec must not read them, and a fixture where one equalled the
// request would let a decoder that did pass.
func bybitTopUpAck(category, symbol string, idx int) string {
	return `{"retCode":0,"retMsg":"OK","result":{` +
		`"category":"` + category + `",` +
		`"symbol":"` + symbol + `",` +
		`"positionIdx":` + itoa(idx) + `,` +
		`"riskId":1,` +
		`"positionIM":"2941.88",` +
		`"positionMM":"612.03",` +
		`"updatedTime":"1578047899471"}}`
}

// ---------------------------------------------------------------------------
// Binance — the encode half
// ---------------------------------------------------------------------------

func TestBinanceEncodeAddIsolatedMarginRendersTheAddDirection(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodeAddIsolatedMargin(
		perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount))
	if err != nil {
		t.Fatalf("EncodeAddIsolatedMargin: %v", err)
	}
	if call.Method != "POST" {
		t.Errorf("method = %q, want POST", call.Method)
	}
	if call.Path != "/fapi/v1/positionMargin" {
		t.Errorf("path = %q", call.Path)
	}

	params := map[string]string{}
	for _, p := range call.Params {
		params[p.Key] = p.Value
	}
	// `type` is asserted to the exact "1". A top-up rendered as type 2 is a
	// REDUCTION on a position whose liq distance provoked the call — the
	// opposite act, accepted by the venue.
	want := map[string]string{"symbol": "BTCUSDT", "amount": topUpAmount, "type": "1"}
	for k, v := range want {
		if params[k] != v {
			t.Errorf("param %s = %q, want %q", k, params[k], v)
		}
	}
	if call.Cost.Weight == 0 {
		t.Error("the call is charged nothing")
	}
}

func TestBinanceEncodeAddIsolatedMarginRefusals(t *testing.T) {
	t.Parallel()
	c := binanceCodec(t)

	t.Run("spot has no position to post margin against", func(t *testing.T) {
		_, err := c.EncodeAddIsolatedMargin(spotInstrument(testBinance), marginAssets(), topUp(t, topUpAmount))
		if !errors.Is(err, venue.ErrPositionSurfaceAbsent) {
			t.Fatalf("got %v, want ErrPositionSurfaceAbsent", err)
		}
	})

	t.Run("a zero amount is refused", func(t *testing.T) {
		_, err := c.EncodeAddIsolatedMargin(perpInstrument(testBinance), marginAssets(), topUp(t, "0"))
		if !errors.Is(err, venue.ErrMarginWriteAmountNotPositive) {
			t.Fatalf("got %v, want ErrMarginWriteAmountNotPositive", err)
		}
	})

	t.Run("a negative amount is refused rather than rendered as type 2", func(t *testing.T) {
		_, err := c.EncodeAddIsolatedMargin(perpInstrument(testBinance), marginAssets(), topUp(t, "-137.45"))
		if !errors.Is(err, venue.ErrMarginWriteAmountNotPositive) {
			t.Fatalf("got %v, want ErrMarginWriteAmountNotPositive", err)
		}
	})

	t.Run("an amount in the BASE asset is refused", func(t *testing.T) {
		base, err := fixedpoint.ParseMoney("BTC", topUpAmount)
		if err != nil {
			t.Fatalf("ParseMoney: %v", err)
		}
		_, err = c.EncodeAddIsolatedMargin(perpInstrument(testBinance), marginAssets(), base)
		if !errors.Is(err, venue.ErrMarginAssetsDisagree) {
			t.Fatalf("got %v, want ErrMarginAssetsDisagree", err)
		}
	})

	// The positive control: without it every refusal above is satisfied by an
	// encoder that refuses everything.
	if _, err := c.EncodeAddIsolatedMargin(perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount)); err != nil {
		t.Fatalf("EncodeAddIsolatedMargin on a well-formed top-up: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Binance — the decode half
// ---------------------------------------------------------------------------

func TestBinanceDecodeAddIsolatedMarginComparesTheAmountBYVALUE(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		echoed string
		want   error
	}{
		{"the amount asked for", topUpAmount, nil},
		{"the same value with a trailing zero", topUpAmountPadded, nil},
		{"MORE than was asked for", topUpAmountOver, venue.ErrMarginWriteUnacknowledged},
		{"LESS than was asked for", topUpAmountUnder, venue.ErrMarginWriteUnacknowledged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := binanceCodec(t).DecodeAddIsolatedMargin(
				perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount),
				transport.Response{Status: 200, Body: []byte(binanceTopUpAck(tc.echoed, 1))})
			if tc.want == nil {
				if err != nil {
					t.Fatalf("DecodeAddIsolatedMargin(%s): %v", tc.echoed, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("DecodeAddIsolatedMargin(%s): got %v, want %v", tc.echoed, err, tc.want)
			}
		})
	}
}

func TestBinanceDecodeAddIsolatedMarginRefusesTheOppositeDirection(t *testing.T) {
	t.Parallel()

	// type 2 is a REDUCTION. Accepted as an acknowledgement it would journal
	// `applied` for an act that took margin OFF the position the ladder was
	// rescuing, and the act-history match would then exclude the venue's own row
	// by direction — which reads as a venue that recorded nothing.
	err := binanceCodec(t).DecodeAddIsolatedMargin(
		perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount),
		transport.Response{Status: 200, Body: []byte(binanceTopUpAck(topUpAmount, 2))})
	if !errors.Is(err, venue.ErrMarginWriteUnacknowledged) {
		t.Fatalf("got %v, want ErrMarginWriteUnacknowledged", err)
	}
}

func TestBinanceDecodeAddIsolatedMarginRefusesAnUnreadableBody(t *testing.T) {
	t.Parallel()

	err := binanceCodec(t).DecodeAddIsolatedMargin(
		perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount),
		transport.Response{Status: 200, Body: []byte(`not json`)})
	if err == nil {
		t.Fatal("an unreadable acknowledgement decoded cleanly")
	}
}

// ---------------------------------------------------------------------------
// Bybit — the encode half
// ---------------------------------------------------------------------------

func TestBybitEncodeAddIsolatedMarginSendsTheOneWayIndexAsBYTES(t *testing.T) {
	t.Parallel()

	call, err := bybitCodec(t).EncodeAddIsolatedMargin(
		perpInstrument(testBybit), marginAssets(), topUp(t, topUpAmount))
	if err != nil {
		t.Fatalf("EncodeAddIsolatedMargin: %v", err)
	}
	if call.Method != "POST" {
		t.Errorf("method = %q, want POST", call.Method)
	}
	if call.Path != "/v5/position/add-margin" {
		t.Errorf("path = %q — Profiles §4 records a slug/path mismatch on this endpoint", call.Path)
	}

	body := string(call.Body)
	// Asserted as rendered bytes. `positionIdx` must be ZERO on the wire, and
	// zero is exactly what `omitempty` removes — so a body checked by decoding
	// it back into a struct would agree with a request that never carried the
	// field at all.
	for _, want := range []string{
		`"positionIdx":0`,
		`"category":"linear"`,
		`"symbol":"BTCUSDT"`,
		`"margin":"` + topUpAmount + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s does not contain %s", body, want)
		}
	}
	// RequestOnly rather than a bare zero Cost: the venue meters this family per
	// request, and ratelimit.Cost.Validate is what tells the two apart — a zero
	// Cost is the field nobody filled in and fails.
	if err := call.Cost.Validate(); err != nil {
		t.Errorf("Cost.Validate: %v — the call must declare what it charges", err)
	}
}

func TestBybitEncodeAddIsolatedMarginRefusals(t *testing.T) {
	t.Parallel()
	c := bybitCodec(t)

	t.Run("spot has no position to post margin against", func(t *testing.T) {
		_, err := c.EncodeAddIsolatedMargin(spotInstrument(testBybit), marginAssets(), topUp(t, topUpAmount))
		if !errors.Is(err, venue.ErrPositionSurfaceAbsent) {
			t.Fatalf("got %v, want ErrPositionSurfaceAbsent", err)
		}
	})

	t.Run("a non-positive amount is refused", func(t *testing.T) {
		_, err := c.EncodeAddIsolatedMargin(perpInstrument(testBybit), marginAssets(), topUp(t, "0"))
		if !errors.Is(err, venue.ErrMarginWriteAmountNotPositive) {
			t.Fatalf("got %v, want ErrMarginWriteAmountNotPositive", err)
		}
	})

	if _, err := c.EncodeAddIsolatedMargin(perpInstrument(testBybit), marginAssets(), topUp(t, topUpAmount)); err != nil {
		t.Fatalf("EncodeAddIsolatedMargin on a well-formed top-up: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Bybit — the decode half, one echo broken at a time
// ---------------------------------------------------------------------------

func TestBybitDecodeAddIsolatedMarginChecksEachEchoSEPARATELY(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{"the acknowledgement this call asked for", bybitTopUpAck("linear", "BTCUSDT", 0), nil},
		{"another category", bybitTopUpAck("spot", "BTCUSDT", 0), venue.ErrMarginWriteUnacknowledged},
		{"another market", bybitTopUpAck("linear", "ETHUSDT", 0), venue.ErrMarginWriteUnacknowledged},
		{"a hedge-mode leg", bybitTopUpAck("linear", "BTCUSDT", 1), venue.ErrPositionModeNotOneWay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := bybitCodec(t).DecodeAddIsolatedMargin(
				perpInstrument(testBybit), marginAssets(), topUp(t, topUpAmount),
				transport.Response{Status: 200, Body: []byte(tc.body)})
			if tc.want == nil {
				if err != nil {
					t.Fatalf("DecodeAddIsolatedMargin: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The client — every refusal is PRE-SEND, and the send happens once
// ---------------------------------------------------------------------------

func topUpClient(t *testing.T, body string) (*venue.OrderClient, *[]string) {
	t.Helper()
	srv, paths := recordingServer(t, body)
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}
	return client, paths
}

func TestAddIsolatedMarginSendsOnceAndReturnsNilOnAnAcknowledgedTopUp(t *testing.T) {
	t.Parallel()

	client, paths := topUpClient(t, binanceTopUpAck(topUpAmount, 1))

	err := client.AddIsolatedMargin(context.Background(),
		perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount), ratelimit.PrioritySafety)
	if err != nil {
		t.Fatalf("AddIsolatedMargin: %v", err)
	}
	// EXACTLY once. §7.1 forbids the blind delta retry, and a transport that
	// retried would be indistinguishable from a doubled top-up by any evidence
	// either venue publishes.
	if len(*paths) != 1 || (*paths)[0] != "/fapi/v1/positionMargin" {
		t.Errorf("the host saw %v, want exactly [/fapi/v1/positionMargin]", *paths)
	}
}

func TestAddIsolatedMarginRefusalsAreALLPreSend(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		instrument ids.VenueInstrument
		assets     venue.InstrumentAssets
		amount     string
		urgency    ratelimit.Priority
		want       error
	}{
		{
			"§7.6's Routine class states nothing true of a state-changing call",
			perpInstrument(testBinance), marginAssets(), topUpAmount,
			ratelimit.PriorityRoutine, venue.ErrMarginWriteUrgencyRoutine,
		},
		{
			"another venue's instrument would be signed with these credentials",
			perpInstrument(testBybit), marginAssets(), topUpAmount,
			ratelimit.PrioritySafety, venue.ErrOrderVenueMismatch,
		},
		{
			"an unrouted family",
			spotInstrument(testBinance), marginAssets(), topUpAmount,
			ratelimit.PrioritySafety, venue.ErrOrderFamilyUnrouted,
		},
		{
			"unset assets",
			perpInstrument(testBinance), venue.InstrumentAssets{}, topUpAmount,
			ratelimit.PrioritySafety, venue.ErrInstrumentAssetsUnset,
		},
		{
			"a non-positive amount",
			perpInstrument(testBinance), marginAssets(), "0",
			ratelimit.PrioritySafety, venue.ErrMarginWriteAmountNotPositive,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, paths := topUpClient(t, binanceTopUpAck(topUpAmount, 1))

			err := client.AddIsolatedMargin(context.Background(),
				tc.instrument, tc.assets, topUp(t, tc.amount), tc.urgency)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			// The claim that matters. A refusal AFTER the send would mean a
			// margin transfer happened and was reported as a refusal — the one
			// shape §7.1's "err means nothing was sent" contract cannot survive.
			if len(*paths) != 0 {
				t.Errorf("the host saw %v, want nothing — the refusal must be pre-send", *paths)
			}
			// Every path out carries an effect, and a pre-send refusal's is the
			// one a caller may conclude NotApplied from.
			if got := apperror.EffectOf(err); got != apperror.EffectNotApplied {
				t.Errorf("effect = %v, want EffectNotApplied", got)
			}
		})
	}
}

func TestAnUnreadableAcknowledgementIsEffectAPPLIED(t *testing.T) {
	t.Parallel()

	// A 2xx the decoder cannot reconcile with the call made. The margin MAY have
	// moved, so the caller must not conclude that nothing happened — which is
	// what EffectNotApplied here would license.
	client, paths := topUpClient(t, binanceTopUpAck(topUpAmountOver, 1))

	err := client.AddIsolatedMargin(context.Background(),
		perpInstrument(testBinance), marginAssets(), topUp(t, topUpAmount), ratelimit.PrioritySafety)
	if err == nil {
		t.Fatal("an unacknowledged top-up returned nil")
	}
	if got := apperror.EffectOf(err); got != apperror.EffectApplied {
		t.Fatalf("effect = %v, want EffectApplied", got)
	}
	if len(*paths) != 1 {
		t.Errorf("the host saw %v, want exactly one send", *paths)
	}
}
