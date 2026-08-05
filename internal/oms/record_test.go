package oms_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// # What this file claims (task #179, record §70b/§70h)
//
// RecordSpotBaseline is the six-step protocol run against the REAL journal,
// replay, join, resolver and verifying loader, with only the venue faked — the
// same composition posture as the rest of this package's tests, because the
// protocol's whole content is the ORDER of those parts and no layer test can
// see an ordering.
//
// The fakes sit at the VENUE seam (driven's adapters are real): a per-id
// scriptable order-read surface whose statuses are walked per call so the two
// reads of §70b step 5 can disagree, ONE shared execution page per symbol —
// both spot orders share BTCUSDT and `attribute` filters by id, which is the
// realistic answer — and a balance surface whose reports are walked per call
// for the same two-reads reason. Both log the urgency of every call, because a
// pinned shed class and an ignored argument look identical from answers alone
// (§70h item 13).
const (
	recordFilledID  = "ord-filled"
	recordPlannedID = "ord-planned"
	recordOpenID    = "ord-open"
	recordAbsentID  = "ord-absent"
	recordVenueID   = "V-88"
)

// recordReader is the venue order-read surface, scriptable per order id.
type recordReader struct {
	// statuses is walked per call per id; the last entry repeats, so a single
	// entry scripts a surface that answers consistently and two script the
	// settle window's before/after disagreement.
	statuses map[string][]venue.OrderStatus
	seen     map[string]int

	// pages is ONE page per symbol, shared by every order on the instrument.
	pages map[string]venue.ExecutionPage

	urgency []ratelimit.Priority
}

func (r *recordReader) OrderByClientID(_ context.Context, ref venue.OrderRef, _ venue.InstrumentAssets, u ratelimit.Priority) (venue.OrderStatus, error) {
	r.urgency = append(r.urgency, u)
	key := string(ref.ClientOrderID)
	list := r.statuses[key]
	if len(list) == 0 {
		// Loud rather than a zero answer: a fixture that forgot to script an
		// order would otherwise be read as a venue absence, which is a §5.3
		// path with its own meaning.
		return venue.OrderStatus{}, fmt.Errorf("record reader: no scripted status for %q", key)
	}
	if r.seen == nil {
		r.seen = make(map[string]int)
	}
	i := min(r.seen[key], len(list)-1)
	r.seen[key]++
	return list[i], nil
}

func (r *recordReader) Executions(_ context.Context, inst ids.VenueInstrument, _ venue.InstrumentAssets, _ venue.ExecutionWindow, u ratelimit.Priority) (venue.ExecutionPage, error) {
	r.urgency = append(r.urgency, u)
	page, ok := r.pages[inst.Symbol]
	if !ok {
		return venue.ExecutionPage{}, fmt.Errorf("record reader: no scripted page for %q", inst.Symbol)
	}
	return page, nil
}

var _ driven.VenueOrderReader = (*recordReader)(nil)

// recordBalances is the venue wallet surface, its reports walked per call with
// the last repeating. The ENGINE normally stamps ObservedAt from the signed
// transport's calibrated clock; a fake standing in for the engine must stamp
// it too, or every report it answers is unusable by construction.
type recordBalances struct {
	reports []venue.SpotBalanceReport
	calls   int
	urgency []ratelimit.Priority
}

func (b *recordBalances) SpotBalances(_ context.Context, u ratelimit.Priority) (venue.SpotBalanceReport, error) {
	b.urgency = append(b.urgency, u)
	if len(b.reports) == 0 {
		return venue.SpotBalanceReport{}, errors.New("record balances: no scripted report")
	}
	i := min(b.calls, len(b.reports)-1)
	b.calls++
	return b.reports[i], nil
}

var _ driven.SpotBalanceReader = (*recordBalances)(nil)

// recordedWait is the settle bracket's blocking wait as a test can hold it:
// the durations it was asked to sleep, and no actual sleeping.
type recordedWait struct{ waits []time.Duration }

func (w *recordedWait) fn(d time.Duration) { w.waits = append(w.waits, d) }

func serverAt(t *testing.T, at time.Time) clock.ServerTime {
	t.Helper()
	st, err := clock.NewServerTime(at, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("NewServerTime: %v", err)
	}
	return st
}

func mustMoney(t *testing.T, asset, v string) fixedpoint.Money {
	t.Helper()
	m, err := fixedpoint.ParseMoney(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseMoney(%s, %q): %v", asset, v, err)
	}
	return m
}

// qtyEquals asserts by value rather than by rendered string, so the claim is
// about the quantity and not about Decimal's formatting.
func qtyEquals(t *testing.T, label string, got fixedpoint.Quantity, asset, want string) {
	t.Helper()
	w, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), want)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %q): %v", asset, want, err)
	}
	c, err := got.Cmp(w)
	if err != nil {
		t.Fatalf("%s: Cmp: %v", label, err)
	}
	if c != 0 {
		t.Errorf("%s = %s, want %s", label, got, want)
	}
}

func recordSpotOrder(t *testing.T, id string) domain.Order {
	t.Helper()
	return domain.Order{
		ClientOrderID: ids.ClientOrderID(id),
		Instrument:    spotInstrument(),
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeLimit,
		TimeInForce:   domain.TimeInForceIOC,
		Quantity:      mustQty(t, "0.5"),
		LimitPrice:    mustPx(t, "50000"),
	}
}

// seedJournal pre-populates the journal the module will open, through the same
// adapter the module uses — module_test.go's second-module shape, without a
// first module: the record protocol's input is a journal with history, and
// building that history through Plan/Dispatch would cap it at the states the
// live path can reach in-process (a dispatched order ends Submitted; the
// protocol needs terminals).
func seedJournal(t *testing.T, path string, seed func(j *driven.SQLiteJournal)) {
	t.Helper()
	j, err := driven.OpenSQLiteJournal(path)
	if err != nil {
		t.Fatalf("OpenSQLiteJournal: %v", err)
	}
	seed(j)
	if err := j.Close(); err != nil {
		t.Fatalf("Close seeded journal: %v", err)
	}
}

// appendFilled journals one spot order's whole ordinary life: planned,
// authorized (the §5.3 anchor), acked, submitted, acknowledged, filled — six
// records, every edge one CanTransition admits.
func appendFilled(t *testing.T, j *driven.SQLiteJournal, id string, at clock.ServerTime) {
	t.Helper()
	o := recordSpotOrder(t, id)
	if err := j.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned(%s): %v", id, err)
	}
	if err := j.AppendAuthorized(o.ClientOrderID, at); err != nil {
		t.Fatalf("AppendAuthorized(%s): %v", id, err)
	}
	if err := j.AppendAcked(o.ClientOrderID, recordVenueID); err != nil {
		t.Fatalf("AppendAcked(%s): %v", id, err)
	}
	for _, s := range []domain.OrderState{domain.StateSubmitted, domain.StateAcknowledged, domain.StateFilled} {
		if err := j.AppendState(o.ClientOrderID, s); err != nil {
			t.Fatalf("AppendState(%s, %s): %v", id, s, err)
		}
	}
}

func appendOpen(t *testing.T, j *driven.SQLiteJournal, id string, at clock.ServerTime) {
	t.Helper()
	o := recordSpotOrder(t, id)
	if err := j.AppendPlanned(o); err != nil {
		t.Fatalf("AppendPlanned(%s): %v", id, err)
	}
	if err := j.AppendAuthorized(o.ClientOrderID, at); err != nil {
		t.Fatalf("AppendAuthorized(%s): %v", id, err)
	}
	if err := j.AppendAcked(o.ClientOrderID, recordVenueID); err != nil {
		t.Fatalf("AppendAcked(%s): %v", id, err)
	}
	if err := j.AppendState(o.ClientOrderID, domain.StateSubmitted); err != nil {
		t.Fatalf("AppendState(%s, submitted): %v", id, err)
	}
}

func presentStatus(t *testing.T, state venue.OrderState, executed string) venue.OrderStatus {
	t.Helper()
	return venue.OrderStatus{
		Presence:         venue.OrderPresent,
		Reach:            venue.QueryReach{Surface: venue.SurfaceOrderStatus, UnfilledRetention: 3 * time.Hour, FilledRetention: 90 * time.Hour},
		State:            state,
		ExecutedQuantity: mustQty(t, executed),
		VenueOrderID:     recordVenueID,
	}
}

func absentStatus() venue.OrderStatus {
	return venue.OrderStatus{
		Presence: venue.OrderAbsent,
		Reach:    venue.QueryReach{Surface: venue.SurfaceOrderHistory, Unpublished: true},
	}
}

// recordAuthorizedAt is the filled fixture's anchor — an hour before the test
// clock's start, so the sweep window is real and the §5.3 horizon has long
// passed for it.
func recordAuthorizedAt(t *testing.T) clock.ServerTime {
	t.Helper()
	return serverAt(t, time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC))
}

// completeWatermark is past every `until` this file's clock can produce, so a
// sweep carrying it proves coverage; shortWatermark is inside the window, so a
// sweep carrying it does not.
func completeWatermark() venue.Cursor {
	return venue.Cursor{At: time.Date(2026, 8, 3, 13, 0, 0, 0, time.UTC)}
}

func shortWatermark() venue.Cursor {
	return venue.Cursor{At: time.Date(2026, 8, 3, 11, 30, 0, 0, time.UTC)}
}

func recordFillRow(t *testing.T) venue.Execution {
	t.Helper()
	return venue.Execution{
		ExecutionID:   "x-1",
		ClientOrderID: recordFilledID,
		Quantity:      mustQty(t, "0.5"),
		Price:         mustPx(t, "50000"),
		Fee:           mustMoney(t, "BTC", "0.0005"),
		At:            time.Date(2026, 8, 3, 11, 30, 0, 0, time.UTC),
	}
}

func btcReport(t *testing.T, free, locked string, total string) venue.SpotBalanceReport {
	t.Helper()
	return venue.SpotBalanceReport{
		ObservedAt: serverAt(t, time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)),
		Rows: []venue.SpotBalanceRow{{
			Asset: "BTC",
			Total: mustQty(t, total),
			Words: []venue.SpotBalanceWord{
				{Name: "free", Value: free},
				{Name: "locked", Value: locked},
			},
		}},
		// The Binance kind, matching the words: every asset comes back, so
		// this report's silences conclude nothing (§70i).
		Absence: venue.SpotAbsenceInconclusive,
	}
}

// recordHarness is one test's whole wiring: fullDeps with the record root's
// posture (RecoveryReadPriority Routine — §70f's rung for a maintenance
// sweep), the two venue fakes, the recording wait, and the protocol's deps.
type recordHarness struct {
	deps     oms.Deps
	reader   *recordReader
	balances *recordBalances
	wait     *recordedWait
	rdeps    oms.RecordBaselineDeps
	path     string
}

func newRecordHarness(t *testing.T) *recordHarness {
	t.Helper()
	h := &recordHarness{
		reader:   &recordReader{statuses: make(map[string][]venue.OrderStatus), pages: make(map[string]venue.ExecutionPage)},
		balances: &recordBalances{},
		wait:     &recordedWait{},
	}
	h.deps = fullDeps(t)
	h.deps.RecoveryReadPriority = ratelimit.PriorityRoutine
	h.deps.VenueOrderReads = h.reader
	h.deps.SpotBalances = h.balances
	h.rdeps = oms.RecordBaselineDeps{
		Fingerprint: "fp-recorded",
		Dialect:     baseline.DialectBinance,
		SettleWait:  h.wait.fn,
		SpotSettle:  2 * time.Second,
	}
	h.path = filepath.Join(t.TempDir(), "2026-08-03-baseline.json")
	return h
}

// happyHarness is the recordable deployment every refusal case then breaks in
// exactly one place: a terminal filled spot order beside a Planned-only one on
// the journal, a consistent venue surface, and a pinnable wallet.
func happyHarness(t *testing.T) *recordHarness {
	t.Helper()
	h := newRecordHarness(t)
	seedJournal(t, h.deps.JournalPath, func(j *driven.SQLiteJournal) {
		appendFilled(t, j, recordFilledID, recordAuthorizedAt(t))
		if err := j.AppendPlanned(recordSpotOrder(t, recordPlannedID)); err != nil {
			t.Fatalf("AppendPlanned(%s): %v", recordPlannedID, err)
		}
	})
	h.reader.statuses[recordFilledID] = []venue.OrderStatus{presentStatus(t, venue.OrderStateFilled, "0.5")}
	h.reader.pages["BTCUSDT"] = venue.ExecutionPage{
		Executions: []venue.Execution{recordFillRow(t)},
		Watermark:  completeWatermark(),
	}
	h.balances.reports = []venue.SpotBalanceReport{btcReport(t, "0.60000000", "0.40000000", "1")}
	return h
}

// TestRecordSpotBaselineRecordsBesideAPlannedOnlyOrder is the protocol's happy
// path and §70h item 12's claim in one: a Planned-only order beside a terminal
// filled one RECORDS — excluded by MayExistAtVenue, the same predicate the
// join derives its scope from, because a crash between plan and dispatch must
// not make recording permanently impossible.
//
// Every number the operator is told is asserted against the arithmetic §70c
// pins: ours = 0.5 − 0.0005 (the base-asset fee) = 0.4995 from the document's
// one order row; B = 1.0 from the venue's words free+locked; baseline =
// 0.5005 — and it is the VERIFYING LOADER's recomputation of the written
// file, not the protocol's own sum, which is what RecordedBaseline.Baselines
// promises.
func TestRecordSpotBaselineRecordsBesideAPlannedOnlyOrder(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	m := newModule(t, h.deps)

	out, err := m.RecordSpotBaseline(h.path, h.rdeps)
	if err != nil {
		t.Fatalf("RecordSpotBaseline: %v", err)
	}

	// §70h item 13's seam evidence: the settle bracket ran exactly once with
	// the configured bound, and EVERY venue call on BOTH surfaces rode
	// Routine. The order-read count is a known exact count and therefore an
	// instrument: the join's status read, the join's one-page sweep, and the
	// settle re-read — a fourth call would mean a surface was read twice
	// somewhere, a second would mean one was skipped.
	if len(h.wait.waits) != 1 || h.wait.waits[0] != h.rdeps.SpotSettle {
		t.Errorf("settle waits = %v, want exactly [%v]", h.wait.waits, h.rdeps.SpotSettle)
	}
	if len(h.reader.urgency) != 3 {
		t.Errorf("order-read surface saw %d calls, want 3 (join read, sweep, re-read)", len(h.reader.urgency))
	}
	for i, u := range h.reader.urgency {
		if u != ratelimit.PriorityRoutine {
			t.Errorf("order read %d rode %s, want %s", i, u, ratelimit.PriorityRoutine)
		}
	}
	if len(h.balances.urgency) != 2 {
		t.Errorf("balance surface saw %d calls, want 2 (the two settle-bracket reads)", len(h.balances.urgency))
	}
	for i, u := range h.balances.urgency {
		if u != ratelimit.PriorityRoutine {
			t.Errorf("balance read %d rode %s, want %s", i, u, ratelimit.PriorityRoutine)
		}
	}

	// The loader's recomputation: B − ours = 1.0 − 0.4995.
	if len(out.Baselines) != 1 {
		t.Fatalf("Baselines = %+v, want exactly one BTC row", out.Baselines)
	}
	if out.Baselines[0].Asset != "BTC" {
		t.Errorf("baseline asset = %s, want BTC", out.Baselines[0].Asset)
	}
	qtyEquals(t, "baseline(BTC)", out.Baselines[0].Amount, "BTC", "0.5005")

	// The document rows are exactly the filled order — the Planned-only one
	// contributed nothing and appears nowhere.
	doc := out.Document
	if len(doc.Orders) != 1 {
		t.Fatalf("document orders = %+v, want exactly the filled order", doc.Orders)
	}
	row := doc.Orders[0]
	if row.ClientOrderID != recordFilledID || row.Side != "buy" || row.Base != "BTC" {
		t.Errorf("order row = %+v, want %s/buy/BTC", row, recordFilledID)
	}
	cumulative, err := fixedpoint.ParseQuantity("BTC", row.Cumulative)
	if err != nil {
		t.Fatalf("stored cumulative %q: %v", row.Cumulative, err)
	}
	qtyEquals(t, "stored cumulative", cumulative, "BTC", "0.5")
	if len(row.Fees) != 1 || row.Fees[0].Asset != "BTC" {
		t.Fatalf("order row fees = %+v, want one BTC fee", row.Fees)
	}
	fee, err := fixedpoint.ParseQuantity("BTC", row.Fees[0].Amount)
	if err != nil {
		t.Fatalf("stored fee %q: %v", row.Fees[0].Amount, err)
	}
	qtyEquals(t, "stored fee", fee, "BTC", "0.0005")

	// The balance row holds the venue's words VERBATIM — inputs, never the
	// derived total.
	if len(doc.Balances) != 1 || doc.Balances[0].Asset != "BTC" || doc.Balances[0].Absent {
		t.Fatalf("document balances = %+v, want one non-absent BTC row", doc.Balances)
	}
	wantWords := []baseline.Word{{Name: "free", Value: "0.60000000"}, {Name: "locked", Value: "0.40000000"}}
	if len(doc.Balances[0].Words) != len(wantWords) {
		t.Fatalf("balance words = %+v, want %+v", doc.Balances[0].Words, wantWords)
	}
	for i, w := range wantWords {
		if doc.Balances[0].Words[i] != w {
			t.Errorf("balance word %d = %+v, want %+v", i, doc.Balances[0].Words[i], w)
		}
	}

	// The chain mark is the seeded history's head: seven records (six for the
	// filled order's life, one for the Planned-only intent) — an exact count,
	// so a mark taken before the resolution loop or against a second journal
	// cannot hide.
	if doc.ChainSeq != 7 {
		t.Errorf("ChainSeq = %d, want 7", doc.ChainSeq)
	}
	if doc.Venue != "binance" || doc.Account != "sub-1" || doc.CredentialSHA256 != "fp-recorded" {
		t.Errorf("document bindings = %s/%s/%s, want binance/sub-1/fp-recorded",
			doc.Venue, doc.Account, doc.CredentialSHA256)
	}

	// The pin the operator is told to configure is the digest of the exact
	// bytes on disk.
	if out.Path != h.path {
		t.Errorf("Path = %q, want %q", out.Path, h.path)
	}
	body, err := os.ReadFile(out.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", out.Path, err)
	}
	digest := sha256.Sum256(body)
	if got := hex.EncodeToString(digest[:]); got != out.SHA256 {
		t.Errorf("SHA256 = %s, want the on-disk digest %s", out.SHA256, got)
	}

	// No prior baseline was configured, so nothing was absorbed and no prior
	// load failed.
	if len(out.Absorbed) != 0 {
		t.Errorf("Absorbed = %+v, want empty with no prior baseline", out.Absorbed)
	}
	if out.PriorError != nil {
		t.Errorf("PriorError = %v, want nil", out.PriorError)
	}
}

// TestRecordSpotBaselineReportsTheAbsorbedDelta is recording-IS-absorption's
// guard: the delta folded into the baseline is PRINTED, per asset, computed
// against the prior the module governs — visibility plus the two-act rule,
// instead of an --absorb-delta flag.
func TestRecordSpotBaselineReportsTheAbsorbedDelta(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.deps.SpotBaselines = []application.SpotBaseline{
		{Venue: "binance", Asset: "BTC", Amount: mustQty(t, "0.3")},
	}
	m := newModule(t, h.deps)

	out, err := m.RecordSpotBaseline(h.path, h.rdeps)
	if err != nil {
		t.Fatalf("RecordSpotBaseline: %v", err)
	}
	if len(out.Absorbed) != 1 {
		t.Fatalf("Absorbed = %+v, want exactly one BTC delta", out.Absorbed)
	}
	got := out.Absorbed[0]
	if got.Asset != "BTC" {
		t.Errorf("absorbed asset = %s, want BTC", got.Asset)
	}
	qtyEquals(t, "absorbed prior", got.Prior, "BTC", "0.3")
	qtyEquals(t, "absorbed new", got.New, "BTC", "0.5005")
	qtyEquals(t, "absorbed delta", got.Delta, "BTC", "0.2005")
}

// TestRecordSpotBaselineRefusesAnOpenSpotOrder: a spot order that is not
// terminal has an open fill ledger in the strong sense — more executions are
// EXPECTED, not §5.3-residual — so `ours` is not a number yet.
func TestRecordSpotBaselineRefusesAnOpenSpotOrder(t *testing.T) {
	t.Parallel()

	h := newRecordHarness(t)
	seedJournal(t, h.deps.JournalPath, func(j *driven.SQLiteJournal) {
		appendOpen(t, j, recordOpenID, recordAuthorizedAt(t))
	})
	// The venue agrees the order is live and unfilled; the sweep concludes
	// with nothing for it. Every gate before the terminal check passes, so the
	// refusal is the terminal check's own.
	h.reader.statuses[recordOpenID] = []venue.OrderStatus{presentStatus(t, venue.OrderStateOpen, "0")}
	h.reader.pages["BTCUSDT"] = venue.ExecutionPage{Watermark: completeWatermark()}
	h.balances.reports = []venue.SpotBalanceReport{btcReport(t, "0.60000000", "0.40000000", "1")}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordSpotOrderOpen) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordSpotOrderOpen", err)
	}
}

// TestRecordSpotBaselineRefusesAnIncompleteSweep: a truncated sweep's silence
// is not evidence (§5.3), and recording over it would attest an `ours` whose
// missing rows are exactly the unknown.
func TestRecordSpotBaselineRefusesAnIncompleteSweep(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.reader.pages["BTCUSDT"] = venue.ExecutionPage{
		Executions: []venue.Execution{recordFillRow(t)},
		Watermark:  shortWatermark(),
	}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordSweepIncomplete) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordSweepIncomplete", err)
	}
}

// TestRecordSpotBaselineRefusesAnUnattributedExecution is THE non-negotiable
// refusal: the order surface's cumulative disagrees with the rows the sweep
// could attribute, so real exposure exists with no owner — and recording over
// it folds that exposure into "not ours" permanently.
func TestRecordSpotBaselineRefusesAnUnattributedExecution(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.reader.statuses[recordFilledID] = []venue.OrderStatus{presentStatus(t, venue.OrderStateFilled, "0.6")}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordUnattributed) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordUnattributed", err)
	}
}

// TestRecordSpotBaselineRefusesAnUnpinnedBalance: a baseline is an
// attestation, and attesting an ambiguous B stores the ambiguity as fact — so
// an unpinned coverage row refuses on the FIRST read, before the settle
// bracket spends its wait.
func TestRecordSpotBaselineRefusesAnUnpinnedBalance(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.balances.reports = []venue.SpotBalanceReport{{
		ObservedAt: serverAt(t, time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)),
		Unpinned:   []venue.SpotUnpinnedRow{{Asset: "BTC", Reason: venue.SpotUnpinnedBorrow}},
		Absence:    venue.SpotAbsenceInconclusive,
	}}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordBalanceUnpinned) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordBalanceUnpinned", err)
	}
	if len(h.wait.waits) != 0 {
		t.Errorf("settle waits = %v, want none — the first read refused before the bracket", h.wait.waits)
	}
}

// TestRecordSpotBaselineRefusesAnInconclusiveAbsence is §70i: the coverage
// asset is missing from a report whose enumeration kind concludes nothing
// from silence (Binance's — every asset comes back, so a missing one is an
// anomaly). A baseline is an attestation, and attesting the absence as a zero
// would store missing evidence as fact — refused on the first read, before
// the bracket spends its wait.
func TestRecordSpotBaselineRefusesAnInconclusiveAbsence(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.balances.reports = []venue.SpotBalanceReport{{
		ObservedAt: serverAt(t, time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)),
		Rows: []venue.SpotBalanceRow{{
			Asset: "ETH",
			Total: mustAssetQty(t, "ETH", "1"),
			Words: []venue.SpotBalanceWord{
				{Name: "free", Value: "1.00000000"},
				{Name: "locked", Value: "0.00000000"},
			},
		}},
		Absence: venue.SpotAbsenceInconclusive,
	}}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordBalanceAbsent) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordBalanceAbsent", err)
	}
	if len(h.wait.waits) != 0 {
		t.Errorf("settle waits = %v, want none — the first read refused before the bracket", h.wait.waits)
	}
}

// TestRecordSpotBaselineRefusesACumulativeThatMoved: the order surface said
// two different totals across the settle window, so the first look described
// an instant that had not settled. §70h item 18 keeps this fixture SEPARATE
// from the balance-moved one — two surfaces, two refusals, two fixtures.
func TestRecordSpotBaselineRefusesACumulativeThatMoved(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.reader.statuses[recordFilledID] = []venue.OrderStatus{
		presentStatus(t, venue.OrderStateFilled, "0.5"),
		presentStatus(t, venue.OrderStateFilled, "0.6"),
	}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordCumulativeMoved) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordCumulativeMoved", err)
	}
}

// TestRecordSpotBaselineRefusesABalanceThatMoved: the two reads disagree in
// their WORDS while agreeing in the derived total — free and locked traded
// 0.1 across the window — and the refusal must still fire, because the
// comparison is verbatim-words equality, not total equality: a total-level
// check would bless a wallet that visibly changed between the two instants
// the attestation claims are one.
func TestRecordSpotBaselineRefusesABalanceThatMoved(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	h.balances.reports = []venue.SpotBalanceReport{
		btcReport(t, "0.60000000", "0.40000000", "1"),
		btcReport(t, "0.70000000", "0.30000000", "1"),
	}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordBalanceMoved) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordBalanceMoved", err)
	}
	if len(h.wait.waits) != 1 {
		t.Errorf("settle waits = %v, want exactly one — the second read is what refused", h.wait.waits)
	}
}

// TestRecordSpotBaselineRefusesAnUnprovedAbsence: an absent order §5.3 has not
// concluded about is an open question, and the gauntlet refuses rather than
// waits. The anchor is seconds old, so the stable-not-found horizon has not
// passed and the resolver answers OutcomeUnproved.
func TestRecordSpotBaselineRefusesAnUnprovedAbsence(t *testing.T) {
	t.Parallel()

	h := newRecordHarness(t)
	seedJournal(t, h.deps.JournalPath, func(j *driven.SQLiteJournal) {
		o := recordSpotOrder(t, recordAbsentID)
		if err := j.AppendPlanned(o); err != nil {
			t.Fatalf("AppendPlanned: %v", err)
		}
		if err := j.AppendAuthorized(o.ClientOrderID, serverAt(t, time.Date(2026, 8, 3, 11, 59, 59, 0, time.UTC))); err != nil {
			t.Fatalf("AppendAuthorized: %v", err)
		}
	})
	h.reader.statuses[recordAbsentID] = []venue.OrderStatus{absentStatus()}
	h.balances.reports = []venue.SpotBalanceReport{btcReport(t, "0.60000000", "0.40000000", "1")}
	m := newModule(t, h.deps)

	if _, err := m.RecordSpotBaseline(h.path, h.rdeps); !errors.Is(err, oms.ErrRecordAbsenceUnproved) {
		t.Fatalf("RecordSpotBaseline = %v, want ErrRecordAbsenceUnproved", err)
	}
}

// TestBootBalanceReadRidesSafety is §70h item 13's other rung: the module's
// OWN balance adapter — the one the boot's spot assertion reads through — is
// constructed at Safety, while RecordSpotBaseline's tests above prove the
// record path rides Routine on the same surface. One fake, two call sites,
// two logged rungs; a constant pinned at either constructor would fail one of
// the pair.
func TestBootBalanceReadRidesSafety(t *testing.T) {
	t.Parallel()

	h := newRecordHarness(t)
	h.deps.PositionScopes = nil
	h.deps.SpotBaselines = []application.SpotBaseline{
		{Venue: "binance", Asset: "BTC", Amount: mustQty(t, "0.3")},
	}
	h.balances.reports = []venue.SpotBalanceReport{btcReport(t, "0.60000000", "0.40000000", "1")}
	m := newModule(t, h.deps)

	if _, err := m.Join.Assert(nil); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if len(h.balances.urgency) != 1 {
		t.Fatalf("balance surface saw %d calls, want 1", len(h.balances.urgency))
	}
	if h.balances.urgency[0] != ratelimit.PrioritySafety {
		t.Errorf("the boot's balance read rode %s, want %s — §7.6 names §7.5's reconcile in the Safety row",
			h.balances.urgency[0], ratelimit.PrioritySafety)
	}
}

// TestRecordSpotBaselineRefusesAnUnwiredSettleBracket: a record without the
// bracket would mint a permanent fact from ONE read of two surfaces that can
// lag each other. Both halves are the rule — a nil wait and a zero bound each
// refuse, because half a bracket is a wait nothing bounds or a bound nothing
// waits.
func TestRecordSpotBaselineRefusesAnUnwiredSettleBracket(t *testing.T) {
	t.Parallel()

	h := happyHarness(t)
	m := newModule(t, h.deps)

	noWait := h.rdeps
	noWait.SettleWait = nil
	if _, err := m.RecordSpotBaseline(h.path, noWait); !errors.Is(err, oms.ErrRecordSettleUnwired) {
		t.Fatalf("RecordSpotBaseline(nil wait) = %v, want ErrRecordSettleUnwired", err)
	}

	noBound := h.rdeps
	noBound.SpotSettle = 0
	if _, err := m.RecordSpotBaseline(h.path, noBound); !errors.Is(err, oms.ErrRecordSettleUnwired) {
		t.Fatalf("RecordSpotBaseline(zero bound) = %v, want ErrRecordSettleUnwired", err)
	}
}
