package exercise_test

import (
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
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
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

		case r.Method == http.MethodGet && r.URL.Path == "/fapi/v3/positionRisk":
			fmt.Fprint(w, `[]`)

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
	doc := fmt.Sprintf(`{
 "oms": {
  "journal_path": %q,
  "account": "chaos-sub",
  "venue": {
    "venue": "binance",
    "dialect": "binance",
    "api_key_ref": "env:ARBLAB_CHAOS_KEY",
    "api_secret_ref": "env:ARBLAB_CHAOS_SECRET",
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
      {"name": "spot_weight", "scope": "family", "product": "spot", "dimension": "weight",
       "interval": "1m", "opening_ceiling": 4200, "routine_ceiling": 5400, "capacity": 6000},
      {"name": "spot_orders", "scope": "family", "product": "spot", "dimension": "orders",
       "interval": "10s", "opening_ceiling": 70, "routine_ceiling": 90, "capacity": 100},
      {"name": "perp_weight", "scope": "family", "product": "linear_perp", "dimension": "weight",
       "interval": "1m", "opening_ceiling": 1680, "routine_ceiling": 2160, "capacity": 2400},
      {"name": "perp_orders", "scope": "family", "product": "linear_perp", "dimension": "orders",
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
     "base": "BTC", "quote": "USDT"}
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
  }
}`, filepath.Join(dir, "journal.db"), host, host)

	path := filepath.Join(dir, "exercise.json")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
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
	for i := 0; i < 2; i++ {
		select {
		case <-venue.arrivals:
		case <-time.After(60 * time.Second):
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
