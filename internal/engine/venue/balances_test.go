package venue_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file checks §7.5's spot balance surface, venue-side (task #179, record
// §70d).
//
// # The collapse checklist was run first, and it fixed the shape of every
// # fixture below
//
//   - free and locked are DIFFERENT nonzero values on the row that matters, so
//     total = free + locked disagrees with free alone, locked alone, and with
//     either value doubled — a codec reading one field, or one field twice,
//     produces a number this fixture refuses.
//   - Every decode fixture holds ≥2 rows, so "this asset's row" and "the first
//     row" are different answers (§70h item 6's cardinality half).
//   - Rows arrive OUT of alphabetical order, because the report promises a
//     sorted answer and a pre-sorted fixture cannot tell a sort from a copy.
//   - Bybit's pinned row (locked "0") and unpinned row (locked != "0") are in
//     ONE answer, so the both-readings rule is exercised beside the row it
//     spares rather than in a fixture that collapses "always pin" and "pin
//     where readings coincide" (§70h item 5).
//   - The two borrow fields each unpin on their OWN fixture: a check reading
//     only borrowAmount agrees with a combined fixture on every answer.

// ---------------------------------------------------------------------------
// The shared per-venue derivations — the ONE meaning the loader reuses
// ---------------------------------------------------------------------------

func TestBinanceSpotTotalIsFreePlusLocked(t *testing.T) {
	t.Parallel()

	total, err := venue.BinanceSpotTotal("BTC", []venue.SpotBalanceWord{
		{Name: "free", Value: "0.50000000"},
		{Name: "locked", Value: "0.25000000"},
	})
	if err != nil {
		t.Fatalf("BinanceSpotTotal: %v", err)
	}
	want := quantity(t, "BTC", "0.75")
	if eq, err := total.Cmp(want); err != nil || eq != 0 {
		t.Errorf("total = %s, want %s (free + locked, two distinct nonzero terms)", total, want)
	}
}

func TestBinanceSpotTotalRefusesAWordSetThatIsNotTheVenueFieldSet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		words []venue.SpotBalanceWord
	}{
		{"missing locked", []venue.SpotBalanceWord{{Name: "free", Value: "1"}}},
		{"free stated twice", []venue.SpotBalanceWord{
			{Name: "free", Value: "1"}, {Name: "free", Value: "2"},
		}},
		{"a word nothing consumes", []venue.SpotBalanceWord{
			{Name: "free", Value: "1"}, {Name: "locked", Value: "0"}, {Name: "extra", Value: "3"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := venue.BinanceSpotTotal("BTC", tc.words); !errors.Is(err, venue.ErrSpotBalanceWordSet) {
				t.Errorf("BinanceSpotTotal(%s) = %v, want ErrSpotBalanceWordSet", tc.name, err)
			}
		})
	}
}

func TestBybitSpotTotalPinsOnlyWhereTheReadingsCoincide(t *testing.T) {
	t.Parallel()

	// locked == 0: the two readings of walletBalance coincide; the total IS
	// walletBalance and the row pins.
	total, reason, err := venue.BybitSpotTotal("BTC", []venue.SpotBalanceWord{
		{Name: "walletBalance", Value: "1.25000000"},
		{Name: "locked", Value: "0"},
	})
	if err != nil {
		t.Fatalf("BybitSpotTotal(locked 0): %v", err)
	}
	if reason != venue.SpotUnpinnedReasonUnspecified {
		t.Fatalf("BybitSpotTotal(locked 0) unpinned as %s, want pinned", reason)
	}
	want := quantity(t, "BTC", "1.25")
	if eq, err := total.Cmp(want); err != nil || eq != 0 {
		t.Errorf("total = %s, want %s", total, want)
	}

	// locked != 0: the readings differ by exactly locked, and the row reports
	// the split rather than betting on the named inference.
	_, reason, err = venue.BybitSpotTotal("BTC", []venue.SpotBalanceWord{
		{Name: "walletBalance", Value: "1.25000000"},
		{Name: "locked", Value: "0.10000000"},
	})
	if err != nil {
		t.Fatalf("BybitSpotTotal(locked != 0): %v", err)
	}
	if reason != venue.SpotUnpinnedLockedSplit {
		t.Errorf("BybitSpotTotal(locked != 0) = %s, want SpotUnpinnedLockedSplit", reason)
	}
}

func TestBybitSpotTotalRefusesAWordSetThatIsNotTheVenueFieldSet(t *testing.T) {
	t.Parallel()

	if _, _, err := venue.BybitSpotTotal("BTC", []venue.SpotBalanceWord{
		{Name: "walletBalance", Value: "1"},
	}); !errors.Is(err, venue.ErrSpotBalanceWordSet) {
		t.Errorf("BybitSpotTotal(missing locked) = %v, want ErrSpotBalanceWordSet", err)
	}
}

// ---------------------------------------------------------------------------
// The encodes — each parameter is a venue fact with a section behind it
// ---------------------------------------------------------------------------

func TestBinanceEncodeSpotBalances(t *testing.T) {
	t.Parallel()

	call, err := binanceCodec(t).EncodeSpotBalances()
	if err != nil {
		t.Fatalf("EncodeSpotBalances: %v", err)
	}
	if call.Method != "GET" || call.Path != "/api/v3/account" {
		t.Errorf("call = %s %s, want GET /api/v3/account", call.Method, call.Path)
	}
	if call.Family != ids.ProductSpot {
		t.Errorf("family = %s, want spot (the account read charges the spot weight budget)", call.Family)
	}
	// Weight 20 is a verified venue figure (Profiles §4's dated block) and the
	// governor charges the caller's word verbatim, so the number is a CLAIM:
	// the 20→10 mutation §70h.22 names must die here.
	if call.Cost.Weight != 20 {
		t.Errorf("cost weight = %d, want the verified 20", call.Cost.Weight)
	}
	// omitZeroBalances stays UNSET: the default (false) is the setting under
	// which every asset comes back and absence carries no conclusion. A
	// parameter appearing here would be the codec choosing absence semantics.
	if len(call.Params) != 0 {
		t.Errorf("params = %v, want none (omitZeroBalances deliberately unset)", call.Params)
	}
}

func TestBybitEncodeSpotBalances(t *testing.T) {
	t.Parallel()

	call, err := bybitCodec(t).EncodeSpotBalances()
	if err != nil {
		t.Fatalf("EncodeSpotBalances: %v", err)
	}
	if call.Method != "GET" || call.Path != "/v5/account/wallet-balance" {
		t.Errorf("call = %s %s, want GET /v5/account/wallet-balance", call.Method, call.Path)
	}
	if call.Family != ids.ProductSpot {
		t.Errorf("family = %s, want spot", call.Family)
	}
	// Exactly ONE parameter: accountType. A `coin` key here would flip the
	// enumeration's absence semantics from "absent coin is zero" to
	// "absent coin means nothing" (Profiles §4's verbatim rule).
	if len(call.Params) != 1 || call.Params[0].Key != "accountType" || call.Params[0].Value != "UNIFIED" {
		t.Errorf("params = %v, want exactly accountType=UNIFIED and never coin", call.Params)
	}
	if call.Cost != ratelimit.RequestOnly() {
		t.Errorf("cost = %+v, want RequestOnly (the endpoint is metered 50/s per UID, no weight exists)", call.Cost)
	}
}

// ---------------------------------------------------------------------------
// The decodes
// ---------------------------------------------------------------------------

func TestBinanceDecodeSpotBalances(t *testing.T) {
	t.Parallel()

	// ETH before BTC (sort vs copy), a zero row beside nonzero ones, and
	// free != locked on the row the total claim rides on.
	body := `{"balances":[
		{"asset":"ETH","free":"2.00000000","locked":"0.00000000"},
		{"asset":"BTC","free":"0.50000000","locked":"0.25000000"},
		{"asset":"USDT","free":"0.00000000","locked":"0.00000000"}
	]}`
	report, err := binanceCodec(t).DecodeSpotBalances(okResponse(body))
	if err != nil {
		t.Fatalf("DecodeSpotBalances: %v", err)
	}
	if len(report.Rows) != 3 || len(report.Unpinned) != 0 {
		t.Fatalf("report = %d rows %d unpinned, want 3/0 (binance rows never unpin)",
			len(report.Rows), len(report.Unpinned))
	}
	if report.Rows[0].Asset != "BTC" || report.Rows[1].Asset != "ETH" || report.Rows[2].Asset != "USDT" {
		t.Errorf("rows are %s/%s/%s, want BTC/ETH/USDT sorted from unsorted wire order",
			report.Rows[0].Asset, report.Rows[1].Asset, report.Rows[2].Asset)
	}
	btc := report.Rows[0]
	want := quantity(t, "BTC", "0.75")
	if eq, err := btc.Total.Cmp(want); err != nil || eq != 0 {
		t.Errorf("BTC total = %s, want %s", btc.Total, want)
	}
	// The words are the venue's fields VERBATIM — the baseline document stores
	// exactly these, so a decoded row that normalized or re-rendered them
	// would store a reconstruction rather than evidence.
	if len(btc.Words) != 2 ||
		btc.Words[0] != (venue.SpotBalanceWord{Name: "free", Value: "0.50000000"}) ||
		btc.Words[1] != (venue.SpotBalanceWord{Name: "locked", Value: "0.25000000"}) {
		t.Errorf("BTC words = %v, want free/locked verbatim", btc.Words)
	}
	// With omitZeroBalances unset every asset comes back, so this report's
	// silences conclude NOTHING — the value here is a claim, not a default,
	// and the Bybit codec must stamp the opposite one (§70i).
	if report.Absence != venue.SpotAbsenceInconclusive {
		t.Errorf("Absence = %s, want SpotAbsenceInconclusive", report.Absence)
	}
}

func TestBinanceDecodeSpotBalancesRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		// An empty balances list cannot be an answer: with omitZeroBalances
		// left false every listed asset comes back, so a rowless body is a
		// wrong-shaped answer and "empty wallet" is exactly the conclusion
		// §5.3's absence rule forbids inferring from it.
		{"no rows", `{"balances":[]}`},
		{"no balances key at all", `{}`},
		{"row with no asset", `{"balances":[{"asset":"BTC","free":"1","locked":"0"},{"asset":"","free":"1","locked":"0"}]}`},
		{"asset answered twice", `{"balances":[{"asset":"BTC","free":"1","locked":"0"},{"asset":"BTC","free":"2","locked":"0"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := binanceCodec(t).DecodeSpotBalances(okResponse(tc.body))
			if !errors.Is(err, venue.ErrSpotBalancesUnreadable) {
				t.Errorf("DecodeSpotBalances(%s) = %v, want ErrSpotBalancesUnreadable", tc.name, err)
			}
		})
	}
}

func TestBybitDecodeSpotBalances(t *testing.T) {
	t.Parallel()

	// One answer holding every arm: a pinned row (locked "0", borrow fields
	// empty — absent evidence is not a borrow), a locked-split row, and the
	// rows arriving out of sort order.
	body := `{"result":{"list":[{"accountType":"UNIFIED","coin":[
		{"coin":"ETH","walletBalance":"2.00000000","locked":"0.30000000","borrowAmount":"0","spotBorrow":"0"},
		{"coin":"BTC","walletBalance":"1.25000000","locked":"0","borrowAmount":"","spotBorrow":""}
	]}]}}`
	report, err := bybitCodec(t).DecodeSpotBalances(okResponse(body))
	if err != nil {
		t.Fatalf("DecodeSpotBalances: %v", err)
	}
	if len(report.Rows) != 1 || len(report.Unpinned) != 1 {
		t.Fatalf("report = %d rows %d unpinned, want 1/1 (the pinned and split arms in one answer)",
			len(report.Rows), len(report.Unpinned))
	}
	btc := report.Rows[0]
	if btc.Asset != "BTC" {
		t.Fatalf("pinned row is %s, want BTC", btc.Asset)
	}
	want := quantity(t, "BTC", "1.25")
	if eq, err := btc.Total.Cmp(want); err != nil || eq != 0 {
		t.Errorf("BTC total = %s, want %s (walletBalance, the coinciding reading)", btc.Total, want)
	}
	if len(btc.Words) != 2 ||
		btc.Words[0] != (venue.SpotBalanceWord{Name: "walletBalance", Value: "1.25000000"}) ||
		btc.Words[1] != (venue.SpotBalanceWord{Name: "locked", Value: "0"}) {
		t.Errorf("BTC words = %v, want walletBalance/locked verbatim", btc.Words)
	}
	if report.Unpinned[0].Asset != "ETH" || report.Unpinned[0].Reason != venue.SpotUnpinnedLockedSplit {
		t.Errorf("unpinned = %+v, want ETH under SpotUnpinnedLockedSplit", report.Unpinned[0])
	}
	// The coin-less call "returns non-zero asset info" — the venue's own
	// sentence — so this report's silences ARE zeros, the opposite stamp from
	// the Binance codec's (§70i).
	if report.Absence != venue.SpotAbsenceZero {
		t.Errorf("Absence = %s, want SpotAbsenceZero", report.Absence)
	}
}

func TestBybitDecodeSpotBalancesUnpinsEachBorrowFieldOnItsOwn(t *testing.T) {
	t.Parallel()

	// Two fixtures, one per field: a combined one would pass a check that
	// reads only borrowAmount.
	cases := []struct {
		name string
		row  string
	}{
		{"borrowAmount", `{"coin":"BTC","walletBalance":"1","locked":"0","borrowAmount":"0.5","spotBorrow":"0"}`},
		{"spotBorrow", `{"coin":"BTC","walletBalance":"1","locked":"0","borrowAmount":"0","spotBorrow":"0.5"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `{"result":{"list":[{"accountType":"UNIFIED","coin":[` + tc.row +
				`,{"coin":"ETH","walletBalance":"2","locked":"0","borrowAmount":"","spotBorrow":""}]}]}}`
			report, err := bybitCodec(t).DecodeSpotBalances(okResponse(body))
			if err != nil {
				t.Fatalf("DecodeSpotBalances: %v", err)
			}
			if len(report.Unpinned) != 1 || report.Unpinned[0].Asset != "BTC" ||
				report.Unpinned[0].Reason != venue.SpotUnpinnedBorrow {
				t.Fatalf("unpinned = %+v, want BTC under SpotUnpinnedBorrow via %s", report.Unpinned, tc.name)
			}
			if len(report.Rows) != 1 || report.Rows[0].Asset != "ETH" {
				t.Errorf("rows = %+v, want the ETH row still pinned beside the refused one", report.Rows)
			}
		})
	}
}

func TestBybitDecodeSpotBalancesEmptyCoinListIsARealAnswer(t *testing.T) {
	t.Parallel()

	// "It returns non-zero asset info": an account holding nothing answers
	// with no rows, and every balance is zero — the one enumeration shape
	// where emptiness is a conclusion rather than a gap.
	body := `{"result":{"list":[{"accountType":"UNIFIED","coin":[]}]}}`
	report, err := bybitCodec(t).DecodeSpotBalances(okResponse(body))
	if err != nil {
		t.Fatalf("DecodeSpotBalances(empty coins): %v", err)
	}
	if len(report.Rows) != 0 || len(report.Unpinned) != 0 {
		t.Errorf("report = %+v, want empty rows and empty unpinned", report)
	}
}

func TestBybitDecodeSpotBalancesRefusals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{"no account entry", `{"result":{"list":[]}}`},
		{"two account entries", `{"result":{"list":[{"accountType":"UNIFIED","coin":[]},{"accountType":"UNIFIED","coin":[]}]}}`},
		{"foreign accountType", `{"result":{"list":[{"accountType":"CONTRACT","coin":[]}]}}`},
		{"coin with no name", `{"result":{"list":[{"accountType":"UNIFIED","coin":[{"coin":"","walletBalance":"1","locked":"0"}]}]}}`},
		{"coin answered twice", `{"result":{"list":[{"accountType":"UNIFIED","coin":[
			{"coin":"BTC","walletBalance":"1","locked":"0"},
			{"coin":"BTC","walletBalance":"2","locked":"0"}]}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := bybitCodec(t).DecodeSpotBalances(okResponse(tc.body))
			if !errors.Is(err, venue.ErrSpotBalancesUnreadable) {
				t.Errorf("DecodeSpotBalances(%s) = %v, want ErrSpotBalancesUnreadable", tc.name, err)
			}
		})
	}
}

func TestBybitDecodeSpotBalancesRefusesAnUnparseableBorrowFigure(t *testing.T) {
	t.Parallel()

	// A NON-empty borrow figure that will not parse is a decode error, never a
	// zero: only the venue saying nothing is "no evidence".
	body := `{"result":{"list":[{"accountType":"UNIFIED","coin":[
		{"coin":"BTC","walletBalance":"1","locked":"0","borrowAmount":"not-a-number","spotBorrow":""}]}]}}`
	if _, err := bybitCodec(t).DecodeSpotBalances(okResponse(body)); err == nil {
		t.Error("DecodeSpotBalances(malformed borrowAmount) = nil, want an error")
	}
}

// ---------------------------------------------------------------------------
// The client method — routing, refusals, and the ObservedAt stamp
// ---------------------------------------------------------------------------

// TestBinanceSpotBalancesRideTheSpotHost is the balance read's version of the
// place test's two-server claim: on Binance the families are different HOSTS,
// and an account read sent to fapi is answered by a wallet that is not the one
// the baseline describes.
func TestBinanceSpotBalancesRideTheSpotHost(t *testing.T) {
	t.Parallel()

	account := `{"balances":[
		{"asset":"BTC","free":"1.00000000","locked":"0.00000000"},
		{"asset":"ETH","free":"0.00000000","locked":"0.00000000"}]}`
	futuresSrv, futuresPaths := recordingServer(t, account)
	spotSrv, spotPaths := recordingServer(t, account)

	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductLinearPerp: signedClient(t, testBinance, futuresSrv, gov),
			ids.ProductSpot:       signedClient(t, testBinance, spotSrv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	report, err := client.SpotBalances(context.Background(), ratelimit.PrioritySafety)
	if err != nil {
		t.Fatalf("SpotBalances: %v", err)
	}
	if len(*spotPaths) != 1 || (*spotPaths)[0] != "/api/v3/account" {
		t.Errorf("the spot host saw %v, want exactly [/api/v3/account]", *spotPaths)
	}
	if len(*futuresPaths) != 0 {
		t.Errorf("the futures host saw %v, want nothing", *futuresPaths)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("report holds %d rows, want 2", len(report.Rows))
	}
	// The stamp comes from the venue-calibrated clock the request was signed
	// with — an unset ServerTime here would mean the client concluded a read
	// it could never have signed.
	if err := report.ObservedAt.Validate(); err != nil {
		t.Errorf("ObservedAt does not validate: %v", err)
	}
}

func TestSpotBalancesRefusesAnUnroutedSpotFamily(t *testing.T) {
	t.Parallel()

	srv, paths := recordingServer(t, `{}`)
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
	if _, err := client.SpotBalances(context.Background(), ratelimit.PrioritySafety); !errors.Is(err, venue.ErrOrderFamilyUnrouted) {
		t.Errorf("SpotBalances(no spot route) = %v, want ErrOrderFamilyUnrouted", err)
	}
	if len(*paths) != 0 {
		t.Errorf("the futures host saw %v, want nothing (the refusal is pre-send)", *paths)
	}
}

func TestSpotBalancesRefusesTheZeroUrgency(t *testing.T) {
	t.Parallel()

	srv, paths := recordingServer(t, `{}`)
	gov := testGovernor(t, testBinance)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: binanceCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductSpot: signedClient(t, testBinance, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}
	if _, err := client.SpotBalances(context.Background(), 0); err == nil {
		t.Error("SpotBalances(zero urgency) = nil, want a refusal — a forgotten rung must not inherit one")
	}
	if len(*paths) != 0 {
		t.Errorf("the host saw %v, want nothing", *paths)
	}
}

// TestBybitSpotBalancesSendTheOneParameterAndNeverCoin reads the WIRE, not the
// encode: the query that leaves the client is the one place "coin is never
// passed" is a fact rather than an intention.
func TestBybitSpotBalancesSendTheOneParameterAndNeverCoin(t *testing.T) {
	t.Parallel()

	body := `{"retCode":0,"retMsg":"OK","result":{"list":[{"accountType":"UNIFIED","coin":[
		{"coin":"BTC","walletBalance":"1.25000000","locked":"0","borrowAmount":"","spotBorrow":""},
		{"coin":"ETH","walletBalance":"2.00000000","locked":"0","borrowAmount":"","spotBorrow":""}]}]}}`
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v5/account/wallet-balance" {
			queries = append(queries, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	gov := testGovernor(t, testBybit)
	client, err := venue.NewOrderClient(venue.OrderClientConfig{
		Codec: bybitCodec(t), Governor: gov,
		Transports: map[ids.ProductCategory]*transport.SignedClient{
			ids.ProductSpot: signedClient(t, testBybit, srv, gov),
		},
	})
	if err != nil {
		t.Fatalf("NewOrderClient: %v", err)
	}

	report, err := client.SpotBalances(context.Background(), ratelimit.PrioritySafety)
	if err != nil {
		t.Fatalf("SpotBalances: %v", err)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("report holds %d rows, want 2", len(report.Rows))
	}
	if len(queries) != 1 {
		t.Fatalf("the host saw %d wallet-balance calls, want 1", len(queries))
	}
	q := queries[0]
	if !strings.Contains(q, "accountType=UNIFIED") {
		t.Errorf("query %q does not carry accountType=UNIFIED", q)
	}
	if strings.Contains(q, "coin=") {
		t.Errorf("query %q carries a coin parameter — the absence semantics just flipped", q)
	}
}

func quantity(t *testing.T, asset fixedpoint.Asset, value string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(asset, value)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %s): %v", asset, value, err)
	}
	return q
}
