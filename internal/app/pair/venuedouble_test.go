package pair

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// venueDouble is one side's exchange, over httptest.
//
// # What it has to be able to distinguish, because two tests depend on it
//
// It must tell *the order is terminal and filled nothing* from *the order is
// not terminal yet*. Both produce a cumulative of zero, and they are the
// fixtures behind two different tests — a double that could express only one of
// them would leave the other passing vacuously. So the terminal state is an
// EXPLICIT control (holdOrderNonTerminal) and there is a self-test below
// asserting the two modes answer the order query DIFFERENTLY.
//
// # fillIOC leaves the order TERMINAL, and a PARTIAL fill is Canceled
//
// domain.OrderState.IsTerminal is true for exactly Filled, Canceled, Rejected
// and AbsentAfterFinality. PartiallyFilled is explicitly NOT terminal — it
// means "some fills applied and MORE POSSIBLE" — so a double modelling a
// partial IOC fill as PartiallyFilled would be plausible and would make
// Establish refuse it, turning the test D5 exists for into an indeterminate run
// that never sends leg B. Wrong in the direction that looks safe.
//
// # The default position answer is FLAT, and it is stated rather than implicit
//
// The flat-start gate refuses a non-flat side, so a double defaulting to
// anything else would refuse every other test here, and one with no position
// surface at all would make Module.VenuePosition error rather than answer. Note
// what the flat default costs: an explicit position(instrument, "0") cannot
// distinguish "the gate read the venue" from "the gate read a default" — only
// the NON-flat subtests separate those, which is why both exist.
type venueDouble struct {
	t       *testing.T
	dialect string // "binance" or "bybit"
	symbol  string
	base    string

	server *httptest.Server

	mu sync.Mutex

	// places records every order that reached the wire, in arrival order.
	places []placedOrder

	// joins records when an order-query call arrived, so a test can assert the
	// drive re-read the venue rather than concluding from its own send.
	joins []time.Time

	// positionListCalls counts /v5/position/list arrivals. On Bybit the §4.1
	// posture probe and §7.5's net-delta enumeration are the SAME path with the
	// same parameters, so this counter is the only thing that tells them apart.
	positionListCalls int

	// enumerations counts §7.5's position ENUMERATIONS only — the Bybit posture
	// probe rides the same path and is excluded, which is what positionListCalls
	// above is for.
	enumerations int

	// rejected are the orders this venue refused ON ITS MERITS, by id. Per
	// order rather than a flag, because rejectPlaceAfter refuses the SECOND
	// order while the first stands — and the order query has to answer for both.
	rejected map[string]bool

	// Controls.
	refuseJoin      bool
	holdNonTerminal bool
	rejectOnPlace   bool

	// rejectFrom is the number of orders this venue accepts before it starts
	// refusing; negative means it never refuses on that ground.
	rejectFrom int
	fills      map[string]string // clientOrderID -> cumulative executed
	positions  map[string]string // symbol -> signed net, "" meaning flat
	leverage   string

	// afterBoot is a net that appears only once the boot's own enumeration has
	// been answered — see positionAppearingAfterBoot.
	afterBoot        string
	flatEnumerations int
}

// placedOrder is one order as it arrived at the venue.
type placedOrder struct {
	ID         string
	Side       string
	Quantity   string
	Price      string
	ReduceOnly bool
	At         time.Time
}

func newVenueDouble(t *testing.T, dialect, symbol, base string) *venueDouble {
	t.Helper()
	v := &venueDouble{
		t: t, dialect: dialect, symbol: symbol, base: base,
		fills:      map[string]string{},
		positions:  map[string]string{},
		rejected:   map[string]bool{},
		rejectFrom: -1,
		leverage:   "5",
	}
	v.server = httptest.NewServer(http.HandlerFunc(v.serve))
	t.Cleanup(v.server.Close)
	return v
}

func (v *venueDouble) host() string { return strings.TrimPrefix(v.server.URL, "http://") }

// ---------------------------------------------------------------------------
// Controls
// ---------------------------------------------------------------------------

// refuseVenueJoin makes the position ENUMERATION fail, which leaves §7.5's
// net-delta gate unasserted and the boot verdict short of ExitClean.
//
// On Bybit the posture probe rides the same path, so the counter above is what
// keeps this control aimed at the join: the first arrival is §4.1's probe and
// every later one is the enumeration.
func (v *venueDouble) refuseVenueJoin() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.refuseJoin = true
}

// position declares the venue's own net for one instrument, signed.
func (v *venueDouble) position(instrument ids.VenueInstrument, net string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.positions[instrument.Symbol] = net
}

// positionAppearingAfterBoot answers FLAT for the boot's own enumeration and
// the given net for every enumeration after it.
//
// # Why the fixture is shaped this way, which is a deviation worth stating
//
// The flat-start gate can only be reached through a CLEAN boot, and a venue
// holding a position no journal explains is §7.5's unexplained delta — so a
// double that simply answered 25 would be refused one gate earlier and this
// gate would never run. The plan's fixture seeds a journal that explains the
// position; this package cannot write one, because §3.6 rule 6 keeps the
// journal adapter out of every root and the only other route is a whole prior
// run whose fills opened the position.
//
// So the position ARRIVES between the boot and the gate — a human opening a
// position on the account mid-run, which is a real hazard and the one case
// where the boot's assertion structurally cannot help. The gate must read the
// venue for itself, which is the claim under test either way.
func (v *venueDouble) positionAppearingAfterBoot(instrument ids.VenueInstrument, net string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.positions[instrument.Symbol] = net
	v.afterBoot = net
	v.flatEnumerations = 1
}

// enumerationCount is how many position enumerations this venue answered. It
// exists so a test can SAY that the boot made the number its fixture assumed,
// rather than depending on it silently.
func (v *venueDouble) enumerationCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.enumerations
}

// fillIOC makes every order this venue receives come back TERMINAL with the
// given cumulative. A partial fill is Canceled rather than PartiallyFilled —
// see the type's comment.
func (v *venueDouble) fillIOC(quantity string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.fills["*"] = quantity
}

// holdOrderNonTerminal makes the order query answer NEW for ever: the order is
// live and nothing is attributed. It is the other zero-cumulative reading.
func (v *venueDouble) holdOrderNonTerminal() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.holdNonTerminal = true
}

// holdOrderNonTerminalWithFills is the same live order with REAL exposure
// against it: the venue publishes executions and still reports the order as
// working.
//
// It is the fixture that separates "not established" from "filled nothing".
// With a zero cumulative the two guards are one predicate — the leg is
// unestablished AND flat — so a drive that had lost its Established check would
// still stop, and the mutation proving that SURVIVED. Here the leg carries a
// non-zero, unfinished quantity, which is the most dangerous reading in this
// whole root: real exposure that has not been measured.
func (v *venueDouble) holdOrderNonTerminalWithFills(quantity string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.holdNonTerminal = true
	v.fills["*"] = quantity
}

// rejectPlace refuses the order ON ITS MERITS at the venue: the request
// crossed the wire and the order is terminally Rejected. It is NOT the same as
// a dispatch refused before Place, which never reaches here at all.
func (v *venueDouble) rejectPlace() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rejectOnPlace = true
}

// rejectPlaceAfter accepts the first n orders and refuses every one after them.
//
// It is what separates "the open worked and the UNWIND was refused" from "this
// venue refuses everything" — two fixtures that produce the same Place count on
// a one-order test and completely different reports on a two-order one.
func (v *venueDouble) rejectPlaceAfter(n int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rejectFrom = n
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

func (v *venueDouble) placeCalls() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.places)
}

func (v *venueDouble) lastPlace() (placedOrder, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.places) == 0 {
		return placedOrder{}, false
	}
	return v.places[len(v.places)-1], true
}

func (v *venueDouble) firstPlace() (placedOrder, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.places) == 0 {
		return placedOrder{}, false
	}
	return v.places[0], true
}

// lastPlaceQuantity is the size the venue was last ASKED for — the quantity
// D5's assertion is about, which is the order this system SENT and not what
// came back.
func (v *venueDouble) lastPlaceQuantity() string {
	last, ok := v.lastPlace()
	if !ok {
		return ""
	}
	return last.Quantity
}

// sawPlaceBefore reports whether this venue's first Place arrived before the
// given instant — the ordering claim the sequential drive rests on.
func (v *venueDouble) sawPlaceBefore(at time.Time) bool {
	first, ok := v.firstPlace()
	return ok && first.At.Before(at)
}

func (v *venueDouble) firstJoinAt() (time.Time, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.joins) == 0 {
		return time.Time{}, false
	}
	return v.joins[0], true
}

// ---------------------------------------------------------------------------
// The wire
// ---------------------------------------------------------------------------

func (v *venueDouble) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if v.dialect == "binance" {
		v.serveBinance(w, r)
		return
	}
	v.serveBybit(w, r)
}

func (v *venueDouble) serveBinance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Mbx-Used-Weight-1m", "20")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/multiAssetsMargin":
		fmt.Fprint(w, `{"multiAssetsMargin":false}`)

	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/positionSide/dual":
		fmt.Fprint(w, `{"dualSidePosition":false}`)

	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/symbolConfig":
		fmt.Fprintf(w, `[{"symbol":%q,"marginType":"ISOLATED","leverage":%s,"isAutoAddMargin":false}]`,
			v.symbol, v.leverage)

	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v3/positionRisk":
		v.noteEnumeration()
		if v.take(&v.refuseJoin, false) {
			http.Error(w, `{"code":-1001,"msg":"the position enumeration is refused by the double"}`,
				http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `[{"symbol":%q,"positionSide":"BOTH","positionAmt":%q}]`, v.symbol, v.net())

	case r.Method == http.MethodPost && r.URL.Path == "/fapi/v1/order":
		id := r.Form.Get("newClientOrderId")
		if v.record(placedOrder{
			ID:         id,
			Side:       r.Form.Get("side"),
			Quantity:   r.Form.Get("quantity"),
			Price:      r.Form.Get("price"),
			ReduceOnly: strings.EqualFold(r.Form.Get("reduceOnly"), "true"),
		}) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-2019,"msg":"Margin is insufficient."}`)
			return
		}
		fmt.Fprintf(w, `{"orderId":%d,"clientOrderId":%q,"status":"NEW"}`, v.venueOrderID(id), id)

	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/order":
		id := r.Form.Get("origClientOrderId")
		v.noteJoin()
		status, filled := v.outcome(id)
		fmt.Fprintf(w, `{"symbol":%q,"orderId":%d,"clientOrderId":%q,"status":%q,"executedQty":%q}`,
			v.symbol, v.venueOrderID(id), id, status, filled)

	case r.Method == http.MethodGet && r.URL.Path == "/fapi/v1/userTrades":
		fmt.Fprint(w, v.binanceTrades(sweptAt(r)))

	default:
		v.unexpected(w, r)
	}
}

func (v *venueDouble) serveBybit(w http.ResponseWriter, r *http.Request) {
	// The venue's own per-endpoint budget headers. Without them every Bybit
	// call folds nothing and the governor counts a FoldMiss — a double that is
	// silently less informative than the venue, on the surface §7.6 sheds by.
	w.Header().Set("X-Bapi-Limit", "50")
	w.Header().Set("X-Bapi-Limit-Status", "49")
	w.Header().Set("X-Bapi-Limit-Reset-Timestamp", fmt.Sprintf("%d", time.Now().Add(time.Second).UnixMilli()))
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v5/account/info":
		bybitOK(w, `{"marginMode":"ISOLATED_MARGIN","unifiedMarginStatus":3,"spotHedgingStatus":"OFF"}`)

	case r.Method == http.MethodGet && r.URL.Path == "/v5/spot-margin-uta/status":
		bybitOK(w, `{"spotMarginMode":"0"}`)

	case r.Method == http.MethodGet && r.URL.Path == "/v5/account/wallet-balance":
		bybitOK(w, `{"list":[{"accountType":"UNIFIED","coin":[`+
			`{"coin":"USDT","borrowAmount":"0","spotBorrow":"0"},`+
			fmt.Sprintf(`{"coin":%q,"borrowAmount":"0","spotBorrow":"0"}`, v.base)+
			`]}]}`)

	case r.Method == http.MethodGet && r.URL.Path == "/v5/position/list":
		// §4.1's posture probe and §7.5's enumeration are the same path with
		// the same parameters on this venue, so the arrival ORDER is the only
		// discriminator: the first is the probe, the rest are the enumeration.
		v.mu.Lock()
		v.positionListCalls++
		enumeration := v.positionListCalls > 1
		if enumeration {
			v.enumerations++
		}
		refuse := v.refuseJoin && enumeration
		v.mu.Unlock()
		if refuse {
			http.Error(w, `{"retCode":10001,"retMsg":"the position enumeration is refused by the double"}`,
				http.StatusInternalServerError)
			return
		}
		bybitOK(w, fmt.Sprintf(`{"category":"linear","list":[`+
			`{"symbol":%q,"side":%q,"size":%q,"positionIdx":0,"leverage":%q,"autoAddMargin":0}]}`,
			v.symbol, v.bybitSide(), v.bybitSize(), v.leverage))

	case r.Method == http.MethodPost && r.URL.Path == "/v5/order/create":
		// The body is read exactly ONCE. Two decoders over one request body is
		// how a fixture comes to record an empty order: the second read gets
		// nothing, silently, and every field assertion then compares "" to "".
		var body struct {
			OrderLinkID string `json:"orderLinkId"`
			Side        string `json:"side"`
			Qty         string `json:"qty"`
			Price       string `json:"price"`
			ReduceOnly  *bool  `json:"reduceOnly"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		if v.record(placedOrder{
			ID: body.OrderLinkID, Side: body.Side, Quantity: body.Qty, Price: body.Price,
			ReduceOnly: body.ReduceOnly != nil && *body.ReduceOnly,
		}) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"retCode":110007,"retMsg":"Insufficient available balance","result":{}}`)
			return
		}
		bybitOK(w, fmt.Sprintf(`{"orderId":"%d","orderLinkId":%q}`, v.venueOrderID(body.OrderLinkID), body.OrderLinkID))

	case r.Method == http.MethodGet && (r.URL.Path == "/v5/order/realtime" || r.URL.Path == "/v5/order/history"):
		id := r.Form.Get("orderLinkId")
		v.noteJoin()
		status, filled := v.outcome(id)
		bybitOK(w, fmt.Sprintf(`{"category":"linear","list":[`+
			`{"orderId":"%d","orderLinkId":%q,"symbol":%q,"orderStatus":%q,"cumExecQty":%q}],"nextPageCursor":""}`,
			v.venueOrderID(id), id, v.symbol, bybitStatus(status), filled))

	case r.Method == http.MethodGet && r.URL.Path == "/v5/execution/list":
		bybitOK(w, v.bybitExecutions(sweptAt(r)))

	default:
		v.unexpected(w, r)
	}
}

// record files an arriving order and reports whether the venue rejects it.
//
// # It takes a couple of milliseconds ON PURPOSE, and that is not padding
//
// The join sweeps executions over [AuthorizedAt−uncertainty, now−uncertainty].
// Both ends shift by the same clock uncertainty, so the window's WIDTH is just
// the real time between the dispatch and the sweep — and in an in-process test
// against a local server that is a fraction of a millisecond. Venues stamp
// executions in whole milliseconds, so a sub-millisecond window contains a
// representable instant only sometimes: the fixture would pass or fail by how
// fast the machine was, which is a coin flip in the harness rather than a test.
//
// A venue round trip is tens of milliseconds in reality. Two here is enough to
// make the window at least a millisecond wide, which makes `startTime + 1ms`
// always inside it — see sweptAt.
func (v *venueDouble) record(o placedOrder) bool {
	o.At = time.Now()
	v.mu.Lock()
	v.places = append(v.places, o)
	reject := v.rejectOnPlace || (v.rejectFrom >= 0 && len(v.places) > v.rejectFrom)
	if reject {
		v.rejected[o.ID] = true
	}
	v.mu.Unlock()
	time.Sleep(2 * time.Millisecond)
	return reject
}

func (v *venueDouble) noteEnumeration() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enumerations++
}

func (v *venueDouble) noteJoin() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.joins = append(v.joins, time.Now())
}

// outcome is the venue's lifecycle answer for one order: a Binance-vocabulary
// status and the cumulative executed quantity.
func (v *venueDouble) outcome(id string) (string, string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.rejected[id] {
		return "REJECTED", "0"
	}
	if v.holdNonTerminal {
		// Live. The cumulative is whatever has been executed so far — zero
		// unless holdOrderNonTerminalWithFills said otherwise, which is the
		// SAME zero as "terminal and filled nothing" and exactly why this
		// control exists.
		if filled, ok := v.fills["*"]; ok {
			return "NEW", filled
		}
		return "NEW", "0"
	}
	filled, ok := v.fills["*"]
	if !ok || filled == "0" {
		// An IOC that rested nowhere: canceled, nothing executed. TERMINAL.
		return "CANCELED", "0"
	}
	if v.fullyFilled(id, filled) {
		return "FILLED", filled
	}
	// A PARTIAL fill is Canceled with a non-zero cumulative — never
	// PartiallyFilled, which the domain treats as non-terminal.
	return "CANCELED", filled
}

// fullyFilled reports whether the recorded order asked for exactly what was
// filled. A held order is never fully filled — it is still working.
func (v *venueDouble) fullyFilled(id, filled string) bool {
	for _, p := range v.places {
		if p.ID == id {
			return p.Quantity == filled
		}
	}
	return false
}

// net is the venue's answer for this symbol, staged by positionAppearingAfterBoot.
func (v *venueDouble) net() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.afterBoot != "" && v.enumerations <= v.flatEnumerations {
		return "0"
	}
	if n, ok := v.positions[v.symbol]; ok && n != "" {
		return n
	}
	return "0"
}

// bybitSide and bybitSize split the signed net the way Bybit publishes it:
// size is always positive and the direction is a separate word, empty for a
// flat row.
func (v *venueDouble) bybitSide() string {
	switch n := v.net(); {
	case strings.HasPrefix(n, "-"):
		return "Sell"
	case n == "0":
		return ""
	default:
		return "Buy"
	}
}

func (v *venueDouble) bybitSize() string { return strings.TrimPrefix(v.net(), "-") }

// sweptAt is the instant the executions this sweep asks for must fall on.
//
// The sweep sends its own [startTime, endTime] window and the decoder REFUSES a
// row outside it — an execution page that does not match the sweep requested.
// So the double answers inside the window it was given rather than stamping
// wall-clock time: a fixture that stamped "now" would be refused whenever the
// join ran a millisecond after the fill, which is every time.
func sweptAt(r *http.Request) int64 {
	start, err := strconv.ParseInt(r.Form.Get("startTime"), 10, 64)
	if err != nil {
		return 0
	}
	// The parameter is the window's start FLOORED to a millisecond, so the true
	// lower bound sits somewhere inside [start, start+1ms). One millisecond on
	// is therefore always at or after it, and — because record() makes the
	// window at least a millisecond wide — always at or before the upper bound.
	return start + 1
}

// binanceTrades and bybitExecutions publish one execution row per filled order,
// so the join's attribution sweep can account for the cumulative it reads.
//
// The Binance row carries `side` and NOT `isBuyer`: those are the USDⓈ-M and
// the spot spellings of one fact, and the decoder refuses a row stating both —
// a fixture carrying each venue's whole vocabulary at once is not a venue.
func (v *venueDouble) binanceTrades(at int64) string {
	rows := v.executionRows()
	if len(rows) == 0 {
		return `[]`
	}
	out := make([]string, 0, len(rows))
	for i, e := range rows {
		out = append(out, fmt.Sprintf(
			`{"id":%d,"orderId":%d,"symbol":%q,"price":%q,"qty":%q,"commission":"0","commissionAsset":"USDT","time":%d,"side":%q}`,
			i+1, v.venueOrderID(e.ID), v.symbol, e.Price, e.Quantity, at, strings.ToUpper(e.Side)))
	}
	return "[" + strings.Join(out, ",") + "]"
}

func (v *venueDouble) bybitExecutions(at int64) string {
	rows := v.executionRows()
	out := make([]string, 0, len(rows))
	for i, e := range rows {
		out = append(out, fmt.Sprintf(
			`{"execId":%q,"orderId":"%d","orderLinkId":%q,"symbol":%q,"side":%q,"execQty":%q,"execPrice":%q,"execFee":"0","feeCurrency":"USDT","execTime":%q}`,
			fmt.Sprintf("x-%d", i+1), v.venueOrderID(e.ID), e.ID, v.symbol, bybitSideWord(e.Side), e.Quantity, e.Price,
			fmt.Sprintf("%d", at)))
	}
	return `{"list":[` + strings.Join(out, ",") + `],"nextPageCursor":""}`
}

// executionRows is one row per order that actually executed something.
func (v *venueDouble) executionRows() []placedOrder {
	v.mu.Lock()
	defer v.mu.Unlock()
	filled, ok := v.fills["*"]
	if !ok || filled == "0" {
		return nil
	}
	out := make([]placedOrder, 0, len(v.places))
	for _, p := range v.places {
		if v.rejected[p.ID] {
			// A refused order executed nothing. Publishing a row for it would
			// have the venue contradict its own order status, which is the one
			// shape #242's Contradicted class exists to catch — invented here by
			// the fixture rather than met at a venue.
			continue
		}
		row := p
		row.Quantity = filled
		out = append(out, row)
	}
	return out
}

// take reads a control under the lock.
func (v *venueDouble) take(flag *bool, _ bool) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return *flag
}

func (v *venueDouble) unexpected(w http.ResponseWriter, r *http.Request) {
	v.t.Errorf("%s venue double: unexpected %s %s", v.dialect, r.Method, r.URL.Path)
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in the pair venue double"}`)
}

// bybitStatus translates the double's Binance-vocabulary status into Bybit's.
func bybitStatus(status string) string {
	switch status {
	case "NEW":
		return "New"
	case "FILLED":
		return "Filled"
	case "CANCELED":
		return "Cancelled"
	case "REJECTED":
		return "Rejected"
	}
	return status
}

// venueOrderID is this venue's own identifier for one client order, and it is
// DISTINCT per order.
//
// A double answering one id for every order is not a shortcut, it is a
// different venue: two orders on one instrument acked with one venue id is the
// ack CONFLICT the OMS latches new trading over, so every fixture here would
// have been quietly running against a poisoned index. Derived from the client
// id so it is stable across the ack, the order query and the execution rows —
// three surfaces that must agree about which venue order this is.
func (v *venueDouble) venueOrderID(clientID string) int64 {
	var sum int64 = 22542179
	for _, c := range clientID {
		sum = sum*31 + int64(c)
	}
	if sum < 0 {
		sum = -sum
	}
	return sum
}

// bybitSideWord is the venue's own capitalization for an order side.
func bybitSideWord(side string) string {
	if strings.EqualFold(side, "buy") {
		return "Buy"
	}
	return "Sell"
}

func bybitOK(w http.ResponseWriter, result string) {
	fmt.Fprintf(w, `{"retCode":0,"retMsg":"OK","result":%s,"time":%d}`, result, time.Now().UnixMilli())
}
