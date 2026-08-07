package exercise_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	appexercise "github.com/giiorg/arb-lab/internal/app/exercise"
	appreconcile "github.com/giiorg/arb-lab/internal/app/reconcile"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
	"github.com/giiorg/arb-lab/internal/risk/infrastructure/attestation"
)

// # §9's process-kill-mid-IOC chaos case, end to end
//
// Run 1 builds the REAL arblab binary and runs `exercise run` against a fake
// venue whose Place handlers signal receipt and then BLOCK — so when both legs'
// requests have arrived, the process provably holds two in-flight IOCs whose
// DispatchAuthorized records are provably fsynced (dispatch orders the fsync
// before the send, and the send arrived). SIGKILL lands inside that window:
// the §5.3 crash boundary, hit deterministically rather than raced for.
//
// Run 2 reconciles the surviving journal in-process, against the same fake now
// answering queries: -2013 for both orders, empty execution sweeps, an empty
// position surface. §9's acceptance bar, asserted directly:
//
//   - journal recovery: both orders replay at DispatchAuthorized and RESOLVE by
//     §5.3's stable-not-found rule — never by resubmission;
//   - no duplicate submission: the fake counts Place calls per ClientOrderID,
//     and every count is exactly 1, across BOTH runs — the §12 named control;
//   - net-delta invariant: the assertion runs and explains every position;
//   - the verdict is clean — a crash mid-IOC whose orders never landed is a
//     condition recovery discharges without a human.
//
// # Why a fake venue and not a testnet (§59e)
//
// A real testnet cannot hold an IOC open while the harness kills the client,
// cannot count our Place calls, and needs credentials #171 has not validated.
// The §9 evidence wanted here is about OUR crash boundary, not a matching
// engine. Every wait below carries its own deadline arm (§59f: a test reading
// a channel a mutation could stop feeding needs its own bound).

// chaosVenue is the fake. Handlers route by path; the two Place paths signal
// and block, everything else answers immediately.
type chaosVenue struct {
	mu sync.Mutex
	// places counts POST /order calls per client order id, across the fake's
	// whole life — the no-duplicate-submission oracle.
	places map[string]int

	// arrivals signals each Place's entry; release, once closed, unblocks
	// every in-flight and future Place.
	arrivals chan string
	release  chan struct{}

	// rejectPerp, when set, refuses the perp family's Place on its merits —
	// the one-leg-failure shape, for the exit-code test.
	rejectPerp bool

	// unexpected records paths the flow was not expected to touch.
	unexpected []string
}

// binanceBootBodies and serveBootReads mirror the in-package fixtures
// (app_test.go) because this file is an EXTERNAL test package and cannot see
// them. The duplication is one map and one function; sharing them would mean
// exporting test helpers from the production package.
//
// The account is FLAT and the boot is CLEAN. Before record §73o it could not be
// both: §4.1's auto-add-margin was read from a POSITION enumeration, so a
// configured symbol holding no position refused (§73j F9's Binance half) and
// this fixture had to establish a position row to boot. All three per-symbol
// settings now ride `symbolConfig`, which enumerates every configured symbol.
var binanceBootBodies = map[string]string{
	// §4.1's three posture probes.
	"/fapi/v1/multiAssetsMargin": `{"multiAssetsMargin":false}`,
	"/fapi/v1/positionSide/dual": `{"dualSidePosition":false}`,
	"/fapi/v1/symbolConfig":      `[{"symbol":"BTCUSDT","marginType":"ISOLATED","leverage":5,"isAutoAddMargin":false}]`,

	// §7.5's net-delta enumeration — a DIFFERENT question, which is why V3 is
	// right here and was wrong for §4.1.
	"/fapi/v3/positionRisk": `[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"0"}]`,
}

func serveBootReads(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	body, ok := binanceBootBodies[r.URL.Path]
	if !ok {
		return false
	}
	w.Header().Set("X-Mbx-Used-Weight-1m", "20")
	fmt.Fprint(w, body)
	return true
}

func newChaosVenue() *chaosVenue {
	return &chaosVenue{
		places:   make(map[string]int),
		arrivals: make(chan string, 8),
		release:  make(chan struct{}),
	}
}

func (v *chaosVenue) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch {
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v3/order" || r.URL.Path == "/fapi/v1/order"):
			id := r.Form.Get("newClientOrderId")
			v.mu.Lock()
			v.places[id]++
			reject := v.rejectPerp && r.URL.Path == "/fapi/v1/order"
			v.mu.Unlock()
			v.arrivals <- id
			// Block until the harness releases — for run 1 that release never
			// comes before the SIGKILL, which is the point.
			<-v.release
			if reject {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"code":-2019,"msg":"Margin is insufficient."}`)
				return
			}
			fmt.Fprintf(w, `{"orderId":22542179,"clientOrderId":%q,"status":"NEW"}`, id)

		case r.Method == http.MethodGet && (r.URL.Path == "/api/v3/order" || r.URL.Path == "/fapi/v1/order"):
			// The orders never landed: Binance's one documented shape for it.
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-2013,"msg":"Order does not exist."}`)

		case r.Method == http.MethodGet && (r.URL.Path == "/api/v3/myTrades" || r.URL.Path == "/fapi/v1/userTrades"):
			fmt.Fprint(w, `[]`)

		case serveBootReads(w, r):

		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/account":
			// Both runs' boots assert the spot leg against the pinned seq-0
			// baseline; the wallet answers the same words the document stores,
			// so the assertion concludes with a zero delta. Never blocks: a
			// recovery read must not wait on the Place gate.
			fmt.Fprint(w, `{"balances":[`+
				`{"asset":"BTC","free":"1.00000000","locked":"0.00000000"},`+
				`{"asset":"USDT","free":"1000.00000000","locked":"0.00000000"}]}`)

		default:
			v.mu.Lock()
			v.unexpected = append(v.unexpected, r.Method+" "+r.URL.Path)
			v.mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-1000,"msg":"unexpected path in chaos fake"}`)
		}
	})
}

func (v *chaosVenue) placeCounts() map[string]int {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make(map[string]int, len(v.places))
	for id, n := range v.places {
		out[id] = n
	}
	return out
}

// chaosConfig writes the ONE document both runs read. Host is the fake; the
// two legs are asymmetric in every dimension (§59f: a two-leg fixture must not
// let "reversed" and "swapped" coincide — sides, products and prices all
// differ, and every assertion keys on ClientOrderID).
func chaosConfig(t *testing.T, dir, host string) string {
	t.Helper()
	baselinePath, baselineDigest := chaosBaseline(t, dir)
	attestPath, attestDigest := chaosAttestation(t, dir)
	doc := fmt.Sprintf(`{
 "oms": {
  "journal_path": %q,
  "account": "chaos-sub",
  "venue": {
    "venue": "binance",
    "dialect": "binance",
    "api_key_ref": "env:ARBLAB_CHAOS_KEY",
    "api_secret_ref": "env:ARBLAB_CHAOS_SECRET",
    "spend_lock_dir": %q,
    "endpoints": [
      {"product": "spot", "scheme": "http", "host": %q},
      {"product": "linear_perp", "scheme": "http", "host": %q}
    ],
    "timeout": "3s",
    "read_limit": 1048576,
    "recv_window": "2s",
    "limits": [
      {"name": "ip_requests", "scope": "venue", "dimension": "requests", "interval": "5s",
       "opening_ceiling": 300, "routine_ceiling": 400, "capacity": 500},
      {"name": "weight_1m", "scope": "family", "product": "spot", "dimension": "weight",
       "interval": "1m", "opening_ceiling": 4200, "routine_ceiling": 5400, "capacity": 6000},
      {"name": "orders_10s", "scope": "family", "product": "spot", "dimension": "orders",
       "interval": "10s", "opening_ceiling": 70, "routine_ceiling": 90, "capacity": 100},
      {"name": "weight_1m", "scope": "family", "product": "linear_perp", "dimension": "weight",
       "interval": "1m", "opening_ceiling": 1680, "routine_ceiling": 2160, "capacity": 2400},
      {"name": "orders_1m", "scope": "family", "product": "linear_perp", "dimension": "orders",
       "interval": "1m", "opening_ceiling": 840, "routine_ceiling": 1080, "capacity": 1200}
    ],
    "throttle_pause": "2m",
    "ban_pause": "10m",
    "clock_offset": "0s",
    "clock_uncertainty": "100ms"
  },
  "instruments": [
    {"venue": "binance", "product": "spot", "symbol": "BTCUSDT", "base": "BTC", "quote": "USDT"},
    {"venue": "binance", "product": "linear_perp", "contract": "linear", "symbol": "BTCUSDT",
     "base": "BTC", "quote": "USDT", "leverage": 5}
  ],
  "position_scopes": [
    {"venue": "binance", "product": "linear_perp", "settle": "USDT"}
  ],
  "resolution": {
    "max_landing": "4s",
    "sweep_margin": "500ms",
    "assumed_reach": [
      {"venue": "binance", "product": "spot", "reach": "72h"}
    ]
  },
  "spot_baseline": {"path": %q, "sha256": %q},
  "portfolio_margin_attestation": {"path": %q, "sha256": %q},
  "spot_settle": "2s",
  "watchdog_silence": "30s",
  "stop_budget": "2s"
 },
  "trade": {
    "id": "018f0c1a-2b3c-7d4e-8f90-1a2b3c4d5e6f",
    "attempt": 0,
    "legs": [
      {"product": "spot", "symbol": "BTCUSDT", "side": "buy", "quantity": "0.5", "limit_price": "50100"},
      {"product": "linear_perp", "contract": "linear", "symbol": "BTCUSDT", "side": "sell",
       "quantity": "0.5", "limit_price": "49900"}
    ]
  },
  "reference": {"path": %q, "max_age": "24h"}
}`, filepath.Join(dir, "journal.db"), dir, host, host,
		baselinePath, baselineDigest, attestPath, attestDigest, chaosReference(t, dir))

	path := filepath.Join(dir, "exercise.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// chaosBaseline writes the seq-0 spot baseline document both runs' boots load:
// account "chaos-sub", the chaos credential's fingerprint, and the same BTC
// words the fake's /api/v3/account answer carries — so run 1's boot passes the
// spot assertion, and run 2's reconcile over the survivor journal recomputes a
// zero delta (the killed orders never filled). Seq 0 verifies vacuously
// against BOTH journals, because every journal extends the empty one.
func chaosBaseline(t *testing.T, dir string) (string, string) {
	t.Helper()
	cred, err := transport.NewCredential("chaos-key", "chaos-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	doc := baseline.Document{
		Kind: baseline.Kind, Version: baseline.Version,
		Venue: "binance", Account: "chaos-sub",
		CredentialSHA256: cred.Fingerprint(),
		ObservedAt:       "2026-08-03T11:00:00Z",
		Balances: []baseline.BalanceRow{{Asset: "BTC", Words: []baseline.Word{
			{Name: "free", Value: "1.00000000"}, {Name: "locked", Value: "0.00000000"},
		}}},
	}
	body, err := baseline.Encode(doc)
	if err != nil {
		t.Fatalf("baseline.Encode: %v", err)
	}
	path := filepath.Join(dir, "spot-baseline.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write baseline document: %v", err)
	}
	digest := sha256.Sum256(body)
	return path, hex.EncodeToString(digest[:])
}

// chaosReference writes the reference revision the run resolves its §2.4
// trading filters from: the real BTCUSDT rows (profiles §5, 2026-08-03
// probes), observed one hour ago so freshness is a relative fact rather than
// a wall-clock bomb. The 0.5 @ 50100/49900 trade above conforms to these
// filters non-trivially — see validConfig's note.
func chaosReference(t *testing.T, dir string) string {
	t.Helper()
	observed := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	doc := fmt.Sprintf(`{
  "fees": [
    {"venue": "binance", "product": "spot", "contract": "", "maker": "0.001", "taker": "0.001",
     "tier": "VIP0-modeled", "class": "hand_recorded", "observed_at": %[1]q, "source": "test fixture"},
    {"venue": "binance", "product": "linear_perp", "contract": "linear", "maker": "0.0002", "taker": "0.0005",
     "tier": "VIP0-modeled", "class": "hand_recorded", "observed_at": %[1]q, "source": "test fixture"}
  ],
  "lots": [
    {"venue": "binance", "product": "spot", "contract": "", "symbol": "BTCUSDT",
     "base": "BTC", "step": "0.00001", "min_qty": "0.00001",
     "quote": "USDT", "min_notional": "5", "tick": "0.01",
     "class": "hand_recorded", "observed_at": %[1]q,
     "source": "test fixture mirroring docs/venue-profiles.md trading filters (2026-08-03)"},
    {"venue": "binance", "product": "linear_perp", "contract": "linear", "symbol": "BTCUSDT",
     "base": "BTC", "step": "0.001", "min_qty": "0.001",
     "quote": "USDT", "min_notional": "50", "tick": "0.1",
     "class": "hand_recorded", "observed_at": %[1]q,
     "source": "test fixture mirroring docs/venue-profiles.md trading filters (2026-08-03)"}
  ],
  "hurdle": {"annual_rate": "0.0369", "access_cost": "0", "liquidity_haircut": "0",
    "counterparty_haircut": "0", "as_of": %[1]q, "class": "hand_recorded",
    "observed_at": %[1]q, "source": "test fixture"}
}`, observed)
	path := filepath.Join(dir, "reference.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write reference fixture: %v", err)
	}
	return path
}

// buildArblab compiles the real binary into dir — NEVER over bin/arblab, which
// belongs to the running capture (standing rule).
func buildArblab(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "arblab")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/giiorg/arb-lab/cmd/arblab")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build arblab: %v\n%s", err, out)
	}
	return bin
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	return filepath.Dir(string(out[:len(out)-1]))
}

func TestProcessKillMidIOC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGKILL semantics; the deployment targets are darwin/linux")
	}
	// NOT t.Parallel(): run 2 resolves credentials from this process's
	// environment via t.Setenv, which forbids it.

	dir := t.TempDir()
	venue := newChaosVenue()
	srv := httptest.NewServer(venue.handler(t))
	defer srv.Close()

	cfgPath := chaosConfig(t, dir, srv.Listener.Addr().String())
	bin := buildArblab(t, dir)

	// ---- Run 1: the kill target ------------------------------------------
	cmd := exec.Command(bin, "exercise", "run", "-config", cfgPath)
	cmd.Env = append(os.Environ(),
		"ARBLAB_CHAOS_KEY=chaos-key", "ARBLAB_CHAOS_SECRET=chaos-secret")
	var stderr, stdout syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start exercise run: %v", err)
	}
	killed := false
	defer func() {
		if !killed {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	// Both legs in flight — the mid-IOC window, entered deterministically.
	//
	// # The bound is 10s because a 60s one was not a bound at all
	//
	// It read 60s until task #199, and tools/mutate.sh's own default timeout is
	// 60s (MUTATE_TIMEOUT). The two tied, the harness won, and a mutation that
	// stopped this subprocess reaching the venue — any refusal in config
	// validation does it — was reported as TIMEOUT!! rather than as this
	// t.Fatalf, which prints the child's stderr and names the cause outright. A
	// hanging test names no defect, and a bound set at the harness's own limit
	// is the shape of that trap one layer out: the guard exists, it is correct,
	// and the tool built to fire it can never see it do so.
	//
	// 10s matches every other bound in this repo and is ~20x the healthy path.
	for i := 0; i < 2; i++ {
		select {
		case <-venue.arrivals:
		case <-time.After(10 * time.Second):
			t.Fatalf("leg %d never reached the venue; exercise stderr:\n%s\nstdout:\n%s",
				i, stderr.String(), stdout.String())
		}
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	killed = true
	if err := cmd.Wait(); err == nil {
		t.Fatal("the killed process exited cleanly — the kill missed the window")
	}

	counts := venue.placeCounts()
	if len(counts) != 2 {
		t.Fatalf("run 1 placed %d distinct orders, want 2: %v", len(counts), counts)
	}

	// The landing horizon: §5.3 refuses to prove an absence while a killed
	// process's submission could still land. Waiting it out is not slack — it
	// is the bound under test doing its job in real time.
	time.Sleep(4*time.Second + 500*time.Millisecond + 400*time.Millisecond)

	// ---- Run 2: reconcile the survivor journal ---------------------------
	t.Setenv("ARBLAB_CHAOS_KEY", "chaos-key")
	t.Setenv("ARBLAB_CHAOS_SECRET", "chaos-secret")
	close(venue.release) // place handlers must never block a recovery query

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	wall, err := clock.NewWall(nil)
	if err != nil {
		t.Fatalf("NewWall: %v", err)
	}
	store, err := config.New[appexercise.Config](cfgPath, wall.Now(), raw)
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	snap := store.Current()
	boot := config.Snapshot[appreconcile.Config]{Value: snap.Value.OMS, Rev: snap.Rev}
	calibrated, err := clock.NewWall(snap.Value.OMS.Calibrations())
	if err != nil {
		t.Fatalf("NewWall(calibrations): %v", err)
	}

	report, err := appreconcile.Run(t.Context(), boot, calibrated,
		observability.NewRegistry(), io.Discard)
	if err != nil {
		t.Fatalf("reconcile.Run: %v", err)
	}

	// §9's bar, item by item.
	if got := report.ExitClass(); got != appreconcile.ExitClean {
		t.Errorf("reconcile ExitClass = %s, want ExitClean; report: %s", got, describe(t, report))
	}
	if len(report.Latched) != 0 {
		t.Errorf("latched gates = %v, want none — a crash whose orders never landed needs no operator", report.Latched)
	}
	rec := report.Reconciliation
	if got := len(rec.Recovery.Dispatched); got != 2 {
		t.Errorf("replay recovered %d dispatched orders, want 2", got)
	}
	if rec.Resolved != 2 {
		t.Errorf("resolved %d absences, want 2 (both by §5.3's stable-not-found rule); unproved: %+v",
			rec.Resolved, rec.Unproved)
	}
	if len(rec.Unproved) != 0 {
		t.Errorf("unproved = %+v, want none", rec.Unproved)
	}
	if n := len(rec.Assertion.Unexplained); n != 0 {
		t.Errorf("net-delta assertion left %d unexplained deltas: %+v", n, rec.Assertion.Unexplained)
	}

	// The no-duplicate-submission oracle, across the fake's whole life: the
	// recovery QUERIED and never re-Placed.
	for id, n := range venue.placeCounts() {
		if n != 1 {
			t.Errorf("order %q was placed %d times across both runs, want exactly 1", id, n)
		}
	}
	venue.mu.Lock()
	unexpected := append([]string(nil), venue.unexpected...)
	venue.mu.Unlock()
	if len(unexpected) != 0 {
		t.Errorf("the flow touched unexpected venue paths: %v", unexpected)
	}
}

// TestExerciseCycleCleanRun is the chaos test's positive control: the same
// binary, the same fake, no kill — the §9 cycle runs to a clean exit. Without
// it, a harness whose exercise binary dies on every boot would still pass the
// kill test's early waits by timing out somewhere that reads as "venue slow".
func TestExerciseCycleCleanRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the deployment targets are darwin/linux")
	}

	dir := t.TempDir()
	venue := newChaosVenue()
	close(venue.release) // acks flow immediately: no kill window wanted here
	srv := httptest.NewServer(venue.handler(t))
	defer srv.Close()

	cfgPath := chaosConfig(t, dir, srv.Listener.Addr().String())
	bin := buildArblab(t, dir)

	cmd := exec.Command(bin, "exercise", "run", "-config", cfgPath)
	cmd.Env = append(os.Environ(),
		"ARBLAB_CHAOS_KEY=chaos-key", "ARBLAB_CHAOS_SECRET=chaos-secret")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exercise run = %v, want exit 0; output:\n%s", err, out)
	}

	for i := 0; i < 2; i++ {
		select {
		case <-venue.arrivals:
		default:
			t.Fatalf("only %d Place arrivals recorded; output:\n%s", i, out)
		}
	}
	for id, n := range venue.placeCounts() {
		if n != 1 {
			t.Errorf("order %q placed %d times, want 1", id, n)
		}
	}
	if want := "2 leg(s) dispatched"; !strings.Contains(string(out), want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
}

// TestExerciseExitCodeOnARejectedLeg drives the one-leg-failure shape through
// the BINARY: the perp leg is refused on its merits, the spot leg is accepted,
// and the process must exit 3 — dispatched-and-needs-a-human, which is
// deliberately not 2's "retry may lift" (§59d: a re-run with the same trade id
// is exactly the §5.3 reuse the planner refuses).
func TestExerciseExitCodeOnARejectedLeg(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the deployment targets are darwin/linux")
	}

	dir := t.TempDir()
	venue := newChaosVenue()
	venue.rejectPerp = true
	close(venue.release)
	srv := httptest.NewServer(venue.handler(t))
	defer srv.Close()

	cfgPath := chaosConfig(t, dir, srv.Listener.Addr().String())
	bin := buildArblab(t, dir)

	cmd := exec.Command(bin, "exercise", "run", "-config", cfgPath)
	cmd.Env = append(os.Environ(),
		"ARBLAB_CHAOS_KEY=chaos-key", "ARBLAB_CHAOS_SECRET=chaos-secret")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("exercise run err = %v (output:\n%s), want an exit error", err, out)
	}
	if code := exit.ExitCode(); code != 3 {
		t.Fatalf("exit code = %d, want 3 (ExitExecution); output:\n%s", code, out)
	}
	if !strings.Contains(string(out), "rejected") {
		t.Errorf("output does not name the rejected leg's state:\n%s", out)
	}
}

// describe renders the report for a failure message.
func describe(t *testing.T, r appreconcile.Report) string {
	t.Helper()
	b, err := json.Marshal(struct {
		Posture  string
		Blocking any
		Latched  any
		Storage  string
		Cause    string
	}{
		Posture:  r.Posture.String(),
		Blocking: r.Blocking,
		Latched:  r.Latched,
		Storage:  fmt.Sprint(r.Storage),
		Cause:    fmt.Sprint(r.Reconciliation.Cause),
	})
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// syncBuffer is a locked buffer for the subprocess's streams — the subprocess
// writes from its goroutines while the test reads on a timeout path.
type syncBuffer struct {
	mu sync.Mutex
	b  []byte
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b = append(s.b, p...)
	return len(p), nil
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.b)
}

// chaosAttestation writes the Portfolio-Margin attestation both runs' boots
// consume (task #198 item 5). It is bound to the chaos credential and the
// chaos account, so it is the SAME two-act pin a deployment uses — the
// fingerprint check is not skipped for being a test.
func chaosAttestation(t *testing.T, dir string) (string, string) {
	t.Helper()
	cred, err := transport.NewCredential("chaos-key", "chaos-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	body, err := json.MarshalIndent(attestation.Document{
		Kind:             attestation.Kind,
		Version:          attestation.Version,
		Venue:            "binance",
		Account:          "chaos-sub",
		CredentialSHA256: cred.Fingerprint(),
		ObservedAt:       "2026-08-06T09:30:00Z",
		Observer:         "gb",
		Method:           "binance.com account page: Portfolio Margin shows not enrolled",
		Observed:         attestation.ObservedOff,
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the attestation: %v", err)
	}
	body = append(body, '\n')
	path := filepath.Join(dir, "portfolio-margin.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	digest := sha256.Sum256(body)
	return path, hex.EncodeToString(digest[:])
}
