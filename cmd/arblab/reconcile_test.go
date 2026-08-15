package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/platform/boot"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// TestReconcileDispatchRoutesAndRefusesAMissingConfig: the subcommand is reachable
// and its usage refusal is EX_USAGE rather than a run that failed.
//
// The two are worth separating because they are what an operator's shell script
// branches on: 64 means the invocation was wrong, and 0/1/2 are verdicts about an
// account.
func TestReconcileDispatchRoutesAndRefusesAMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := dispatch([]string{"reconcile", "run"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(reconcile run) = %d, want 64 (EX_USAGE)", got)
	}
	if !strings.Contains(stderr.String(), "-config is required") {
		t.Errorf("stderr = %q, want the missing flag named", stderr.String())
	}

	// And an unrouted subcommand still falls through to usage, so the new `if`
	// above did not swallow the arm below it.
	stdout.Reset()
	stderr.Reset()
	if got := dispatch([]string{"reconcile", "nonesuch"}, &stdout, &stderr); got != 64 {
		t.Errorf("dispatch(reconcile nonesuch) = %d, want 64", got)
	}
	if !strings.Contains(stderr.String(), "arblab reconcile run") {
		t.Errorf("usage does not advertise the subcommand: %q", stderr.String())
	}
}

// TestReconcileReportNamesTheLatchAndDisclaimsThePosture is the operator-facing
// half of this slice's finding.
//
// The fixture's posture is PostureReconcileOnly — which is what a completely
// clean run of this binary also reports — so the rendered report has to make the
// difference legible, and the only thing that does is the latch. A report that
// printed the posture and stopped would be identical for both.
func TestReconcileReportNamesTheLatchAndDisclaimsThePosture(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateVenueJoined, boot.GateAccountPosture},
		Latched:  []boot.Gate{boot.GateVenueJoined},
	})
	got := buf.String()
	for _, want := range []string{
		"GateVenueJoined",    // the latch is named
		"operator clearance", // and what it means
		"no producer yet",    // the posture carries its own disclaimer
		appreconcile.ExitOperator.String(),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report does not contain %q:\n%s", want, got)
		}
	}

	// The clean case says "none" rather than printing an empty slice, because
	// "latched:  []" is the line an operator skims past.
	buf.Reset()
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateAccountPosture},
	})
	if !strings.Contains(buf.String(), "latched:  none") {
		t.Errorf("clean report does not say the latch set is empty:\n%s", buf.String())
	}
}

// TestWriteRateLimitStatusDrawsSection76sLine: a ban is an OPERATOR ALERT, a
// throttle pause is information, and a run with no venue traffic prints
// nothing. The three cases are asserted against each other's markers — a
// renderer that alerted on every pause would pass the first and fail the
// second, and one that printed a standing "nothing" line would fail the third.
func TestWriteRateLimitStatusDrawsSection76sLine(t *testing.T) {
	until := time.Date(2026, 8, 3, 14, 7, 9, 0, time.UTC)

	var buf bytes.Buffer
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue: ids.Venue("binance"), Bans: 1,
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyBan,
	})
	got := buf.String()
	for _, want := range []string{
		"OPERATOR ALERT",
		"binance",
		"banned this IP 1 time(s)",
		"do not retry",
		"paused until 2026-08-03T14:07:09Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ban line does not contain %q:\n%s", want, got)
		}
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Paused: true, PausedUntil: until, PauseKind: ratelimit.PenaltyThrottle,
	})
	got = buf.String()
	if !strings.Contains(got, "paused until 2026-08-03T14:07:09Z") ||
		!strings.Contains(got, "PenaltyThrottle") {
		t.Errorf("throttle line does not name the pause and its kind:\n%s", got)
	}
	if strings.Contains(got, "OPERATOR ALERT") {
		t.Errorf("a throttle pause is §7.6's hard pause, NOT its alert:\n%s", got)
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue:  ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{{Name: "order_realtime"}},
	})
	if buf.Len() != 0 {
		t.Errorf("zero fold misses printed %q, want no line", buf.String())
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue: ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{
			{Name: "order_realtime", FoldMisses: 1},
			{Name: "order_history", FoldMisses: 2},
		},
	})
	wantMisses := "" +
		"ratelimit: bybit order_realtime: 1 response(s) carried no usable budget reading this run (§7.6 fold-health — venue-side corrections were missed)\n" +
		"ratelimit: bybit order_history: 2 response(s) carried no usable budget reading this run (§7.6 fold-health — venue-side corrections were missed)\n"
	if got := buf.String(); got != wantMisses {
		t.Errorf("fold-miss lines = %q, want %q", got, wantMisses)
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue: ids.Venue("bybit"),
		Limits: []ratelimit.LimitState{
			{Name: "order_realtime", Reserved: 1},
			{Name: "order_history", Reserved: 2},
		},
	})
	wantReserved := "" +
		"ratelimit: bybit order_realtime: 1 unit(s) reserved for an in-flight package\n" +
		"ratelimit: bybit order_history: 2 unit(s) reserved for an in-flight package\n"
	if got := buf.String(); got != wantReserved {
		t.Errorf("reserved lines = %q, want %q", got, wantReserved)
	}

	buf.Reset()
	writeRateLimitStatus(&buf, ratelimit.Snapshot{
		Venue: ids.Venue("bybit"), Bans: 1,
		Limits: []ratelimit.LimitState{{Name: "order_realtime", FoldMisses: 1}},
	})
	got = buf.String()
	if strings.Count(got, "ratelimit:") != 2 ||
		!strings.Contains(got, "carried no usable budget reading this run") ||
		!strings.Contains(got, "OPERATOR ALERT") {
		t.Errorf("ban-and-miss snapshot must print both lines:\n%s", got)
	}
	if strings.Index(got, "carried no usable budget reading") > strings.Index(got, "OPERATOR ALERT") {
		t.Errorf("fold-miss line must precede the ban line:\n%s", got)
	}
}

// TestReconcileReportCarriesTheBanAlert: the alert through the full report
// renderer, once — the count matters, because the exercise renderer composes
// this one and adds its own call, and "printed" alone cannot see a double.
// Bans is 2 here and 1 in the unit test above, so a renderer gating on an
// exact count fails one of the pair.
func TestReconcileReportCarriesTheBanAlert(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Blocking: []boot.Gate{boot.GateAccountPosture},
		RateLimit: ratelimit.Snapshot{
			Venue: ids.Venue("binance"), Bans: 2,
			Paused: true, PausedUntil: time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC),
			PauseKind: ratelimit.PenaltyBan,
		},
	})
	got := buf.String()
	if n := strings.Count(got, "OPERATOR ALERT"); n != 1 {
		t.Errorf("report contains %d alert lines, want exactly 1:\n%s", n, got)
	}
	if !strings.Contains(got, "banned this IP 2 time(s)") {
		t.Errorf("report does not carry the count:\n%s", got)
	}
	if !strings.Contains(got, appreconcile.ExitOperator.String()) {
		t.Errorf("a banned report's verdict is not ExitOperator:\n%s", got)
	}
}

// TestReconcileReportGoldenNoMiss pins the COMPLETE no-miss report output.
// The renderer was split (verdict + ratelimit + diagnosis) for the exercise
// composition, and "the reconcile output is unchanged" is a claim substring
// assertions cannot check — a format drift in any standing line survived
// every test in this file until this one existed. Unexplained deltas are the
// one section absent here: their fixture needs a position surface and their
// format is owned by the join tests.
//
// # The two join findings carry their own collapse guard (task #242)
//
// Unattributed and Contradicted are the two findings that LATCH
// GateVenueJoined, and until #242 the renderer printed neither — a boot could
// halt naming no order at all. They are pinned here rather than in a test of
// their own because this is the golden whose subject is completeness, and a
// section nobody added to the no-miss fixture is exactly the section that
// silently stops rendering.
//
// The fixture gives them DIFFERENT order ids (ord-3, ord-4) and the renderer
// gives them different line prefixes, and both halves are load-bearing: with
// one shared id, "prints both lists" and "prints one list twice" produce
// identical output, which is the copy-paste defect two adjacent loops over
// two same-typed slices actually invite. Distinct ids also make the pair
// non-collapsible under a swap of the two loops.
func TestReconcileReportGoldenNoMiss(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture: boot.PostureReconcileOnly,
		Latched: []boot.Gate{boot.GateNetDeltaAsserted},
		RateLimit: ratelimit.Snapshot{
			Venue:  ids.Venue("bybit"),
			Paused: true, PausedUntil: time.Date(2026, 8, 4, 16, 30, 0, 0, time.UTC),
			PauseKind: ratelimit.PenaltyThrottle,
			Limits: []ratelimit.LimitState{{
				Name: "order_realtime", FoldMisses: 1, Reserved: 2,
			}},
		},
		Blocking: []boot.Gate{boot.GateAccountPosture},
		Storage:  errors.New("journal on nfs"),
		Reconciliation: application.Reconciliation{
			Recovery: application.Recovery{Records: 7, Dispatched: []ids.ClientOrderID{"ord-1", "ord-2"}},
			Join: application.Join{
				Orders: 2, Fills: 3, Duplicates: 1,
				Absent: []ids.ClientOrderID{"ord-2"},
				// Distinct ids, deliberately — see the doc above. A shared id
				// here would make the two loops indistinguishable from one
				// loop rendered twice.
				Unattributed: []ids.ClientOrderID{"ord-3"},
				Contradicted: []ids.ClientOrderID{"ord-4"},
				Incomplete:   []ids.ClientOrderID{"ord-5"},
			},
			Resolved: 0,
			Unproved: []application.UnprovedOrder{{Order: "ord-2", Reason: application.ReasonNoAnchor}},
			Stale:    []ids.ClientOrderID{"ord-1"},
			Cause:    errors.New("venue join stopped early"),
		},
	})
	want := "posture:  PostureReconcileOnly (this binary reports 5 of §7.5's 8 gates; the rest have no producer yet)\n" +
		"verdict:  ExitOperator\n" +
		"latched:  [GateNetDeltaAsserted]  <- operator clearance required; a retry will not lift these\n" +
		"ratelimit: bybit order_realtime: 1 response(s) carried no usable budget reading this run (§7.6 fold-health — venue-side corrections were missed)\n" +
		"ratelimit: bybit order_realtime: 2 unit(s) reserved for an in-flight package\n" +
		"ratelimit: bybit paused until 2026-08-04T16:30:00Z (PenaltyThrottle — a hard pause, not an operator matter)\n" +
		"blocking: [GateAccountPosture]\n" +
		"storage:  journal on nfs\n" +
		"replayed: 7 records, 2 dispatched orders\n" +
		"joined:   2 orders, 3 fills admitted, 1 duplicates\n" +
		"  unattributed ord-3: the venue's cumulative disagrees with the executions this join could attribute\n" +
		"  contradicted ord-4: the venue and this system hold lifecycle states SM §1 has no edge between\n" +
		"  incomplete ord-5: the executions sweep did not prove coverage through the instant it was asked about (a retry may lift this)\n" +
		"absent:   1 order(s) the venue does not hold; 0 resolved, 1 unproved\n" +
		"  unproved ord-2: no_anchor\n" +
		"  stale projection for ord-1\n" +
		"cause:    venue join stopped early\n"
	if got := buf.String(); got != want {
		t.Errorf("report drifted from the golden output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestReconcileReportRendersEveryJoinFindingList enforces the claim the
// renderer's comment used to merely assert (ruling R9).
//
// # Why a reflective test and not a sentence
//
// This file has now carried two wrong totality claims about the same six
// lines. Task #242 found the renderer printing neither Unattributed nor
// Contradicted; the comment written to repair that said the renderer
// enumerated "every other finding", while Join.Incomplete was unrendered the
// whole time. Both were true-sounding, both were checked by nothing, and a
// third would read exactly like the first two. A count or a list in prose
// cannot fail — so the property is derived from application.Join's own fields
// here, and a per-order finding list added there fails this test by NAME
// rather than going quietly unrendered.
//
// # Substring, deliberately, and it does not collide with the goldens
//
// The goldens pin FORMAT and order; this pins EXISTENCE, which is a different
// claim and the one a new field breaks. A substring is the right instrument
// for "the id reached the output at all", and the probe id is unique per
// field — so no other line can supply it, which is what the fallback-superset
// trap punishes.
//
// # The selector is every SLICE, and that is the third version of this defect
//
// The first version of this test selected fields whose type was exactly
// []ids.ClientOrderID. That guard went stale silently in precisely the way
// the prose it replaced went stale: adding Repudiated []UnprovedOrder to
// application.Join left it PASSING, no output, `checked` untouched (ruling
// R12). Reconciliation.Unproved is already that shape, and the obvious next
// edit to Contradicted — carry the venue's state beside the id — would have
// dropped an already-covered field out of the guard while `checked` merely
// went 3 to 2, which no assertion here could see.
//
// A guard's selector has to be at least as wide as the claim it backs, or the
// claim is narrower than it reads. The claim is about every per-order finding
// list, so the selector is every slice: Join's counters are ints and drop out
// on their own, a field that changes ELEMENT type stays selected, and a slice
// this test cannot plant a probe into fails loudly rather than falling out.
func TestReconcileReportRendersEveryJoinFindingList(t *testing.T) {
	// Absent is exempt, and the reason is a claim about EVIDENCE rather than a
	// convenience: an absence is not a finding yet. The venue not holding an
	// order licenses nothing until §5.3's rule runs, so it renders as the
	// `absent:` count plus that rule's own per-order `unproved` detail — which
	// IS the rendering, one phase down. A per-order "absent ord-N" line here
	// would report a verdict the join is forbidden to reach.
	exempt := map[string]string{"Absent": "rendered as the absent: count plus §5.3's per-order unproved lines"}

	isFindingList := func(f reflect.StructField) bool { return f.Type.Kind() == reflect.Slice }

	// The exemption pre-check, and it is this test's real vacuity anchor
	// rather than the checked > 0 below: it demands that Absent still EXIST on
	// application.Join and still be selected by the same predicate the loop
	// uses. So a rename, a retype, or a broken selector fails HERE, by name,
	// before the loop can quietly match nothing.
	joinType := reflect.TypeOf(application.Join{})
	for name := range exempt {
		f, ok := joinType.FieldByName(name)
		if !ok || !isFindingList(f) {
			t.Fatalf("exemption names %q, which is not a per-order finding list on application.Join — "+
				"a stale exemption silently un-checks a field", name)
		}
	}

	checked := 0
	for i := range joinType.NumField() {
		f := joinType.Field(i)
		if !isFindingList(f) {
			continue
		}
		if _, skip := exempt[f.Name]; skip {
			continue
		}
		checked++

		// One field at a time, with an id nothing else in the report can
		// produce: if the line appears, it appeared because THIS list was
		// rendered.
		probe := "probe-" + f.Name
		elem := reflect.New(f.Type.Elem()).Elem()
		if !plantProbe(elem, probe) {
			t.Fatalf("application.Join.%s is a finding list this test cannot probe: nothing settable and "+
				"string-shaped in %s. Give it an id the probe can plant, or exempt it by name with its "+
				"reason — a list the guard cannot probe must not fall OUT of the guard",
				f.Name, f.Type.Elem())
		}
		list := reflect.MakeSlice(f.Type, 1, 1)
		list.Index(0).Set(elem)

		var join application.Join
		reflect.ValueOf(&join).Elem().Field(i).Set(list)

		var buf bytes.Buffer
		writeReconcileReport(&buf, appreconcile.Report{
			Reconciliation: application.Reconciliation{Join: join},
		})
		if !strings.Contains(buf.String(), probe) {
			t.Errorf("application.Join.%s is a per-order finding list the diagnosis never renders — "+
				"a boot it holds would name no order and no reason:\n%s", f.Name, buf.String())
		}
	}

	// A vacuous pass and a clean pass look identical. This is the weaker of
	// the two vacuity guards — the pre-check above catches a broken selector
	// first, and by name — but it is what remains true if every field were
	// exempted. No expected COUNT is pinned: that would be the same decaying
	// claim this test replaces, only that the property has subjects.
	if checked == 0 {
		t.Fatal("no per-order finding list was checked; the field selection matched nothing and this test proved nothing")
	}
}

// plantProbe writes id somewhere inside v — one ELEMENT of a finding list —
// and reports whether it found anywhere to write it.
//
// A finding list's element is an id today and may be a struct carrying the id
// beside its reason tomorrow (Reconciliation.Unproved already is), so the
// probe follows the shape rather than naming a type. It prefers a
// ClientOrderID field, because a struct with several strings should be probed
// through the one the renderer is most likely to print; failing that it takes
// the first settable string it finds. Returning false is not a fallback, it is
// the caller's t.Fatalf: default-deny, so an element shape nobody anticipated
// stops this test rather than silently leaving its field unchecked.
func plantProbe(v reflect.Value, id string) bool {
	orderID := reflect.TypeOf(ids.ClientOrderID(""))
	switch v.Kind() {
	case reflect.String:
		if !v.CanSet() {
			return false
		}
		v.SetString(id)
		return true
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).Type == orderID {
				return plantProbe(v.Field(i), id)
			}
		}
		for i := range v.NumField() {
			if plantProbe(v.Field(i), id) {
				return true
			}
		}
	}
	return false
}

func goldenQty(t *testing.T, asset, v string) fixedpoint.Quantity {
	t.Helper()
	q, err := fixedpoint.ParseQuantity(fixedpoint.Asset(asset), v)
	if err != nil {
		t.Fatalf("ParseQuantity(%s, %q): %v", asset, v, err)
	}
	return q
}

// TestReconcileReportGoldenSpotSections pins the COMPLETE output of a report
// carrying every assertion section at once (task #179, record §70h items 11
// and 21). Two claims live in the ORDER, and only a golden can hold an order:
//
//   - The spot unexplained line sits BESIDE the derivative delta — same
//     finding, one leg over — before the foreign/unasserted/unconcluded tail,
//     and no substring battery can see the render loops swapped.
//   - The latch and the unconcluded line appear in ONE report with the
//     verdict ExitOperator: an unpinned row is re-reportable work, and it
//     must not mask a latched delta (NeedsOperator > Incomplete).
//
// The fixture is a RENDERING fixture: it populates every section at once so
// their order is observable, not a claim that one join can produce this exact
// combination — section membership is the joiner's tests' subject.
func TestReconcileReportGoldenSpotSections(t *testing.T) {
	var buf bytes.Buffer
	writeReconcileReport(&buf, appreconcile.Report{
		Posture:  boot.PostureReconcileOnly,
		Latched:  []boot.Gate{boot.GateNetDeltaAsserted},
		Blocking: []boot.Gate{boot.GateAccountPosture},
		Reconciliation: application.Reconciliation{
			Recovery: application.Recovery{Records: 9, Dispatched: []ids.ClientOrderID{"ord-1", "ord-2"}},
			Join:     application.Join{Orders: 2, Fills: 2},
			Assertion: application.Assertion{
				Unexplained: []application.InstrumentDelta{{
					Instrument: ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp,
						Symbol: "BTCUSDT", Contract: ids.ContractLinear},
					Expected:    goldenQty(t, "BTC", "-0.4"),
					Observed:    goldenQty(t, "BTC", "-0.5"),
					Unexplained: goldenQty(t, "BTC", "-0.1"),
				}},
				SpotUnexplained: []application.AssetDelta{{
					Venue:       "binance",
					Asset:       "BTC",
					Expected:    goldenQty(t, "BTC", "0.4995"),
					Observed:    goldenQty(t, "BTC", "0.7"),
					Unexplained: goldenQty(t, "BTC", "0.2005"),
				}},
				Foreign: []string{"ETHBTC"},
				Unasserted: []application.UnassertedInstrument{{
					Instrument: ids.VenueInstrument{Venue: "binance", Product: ids.ProductSpot, Symbol: "ETHUSDT"},
					Reason:     application.UnassertedBaselineUnrecorded,
				}},
				// Both declared reasons, one line each (§70h item 21): the
				// renderer enumerates fields by hand, so a reason whose line
				// nobody pinned could vanish silently.
				SpotUnconcluded: []application.UnconcludedBalance{
					{Venue: "binance", Asset: "ETH", Reason: application.UnpinnedBorrowOutstanding},
					{Venue: "binance", Asset: "SOL", Reason: application.UnpinnedLockedSplitUnverified},
				},
			},
		},
	})
	want := "posture:  PostureReconcileOnly (this binary reports 5 of §7.5's 8 gates; the rest have no producer yet)\n" +
		"verdict:  ExitOperator\n" +
		"latched:  [GateNetDeltaAsserted]  <- operator clearance required; a retry will not lift these\n" +
		"blocking: [GateAccountPosture]\n" +
		"replayed: 9 records, 2 dispatched orders\n" +
		"joined:   2 orders, 2 fills admitted, 0 duplicates\n" +
		"  unexplained binance:linear_perp:BTCUSDT:linear: venue -0.5, ours -0.4, delta -0.1\n" +
		"  spot unexplained binance BTC: balance-minus-baseline 0.7, ours 0.4995, delta 0.2005\n" +
		"  foreign symbol ETHBTC\n" +
		"  unasserted binance:spot:ETHUSDT: UnassertedBaselineUnrecorded\n" +
		"  spot unconcluded binance ETH: UnpinnedBorrowOutstanding (re-reportable; a further attempt may pin it)\n" +
		"  spot unconcluded binance SOL: UnpinnedLockedSplitUnverified (re-reportable; a further attempt may pin it)\n"
	if got := buf.String(); got != want {
		t.Errorf("report drifted from the golden output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
