package capture

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// TestCaptureRunEndToEnd is the composition's positive control: a real
// document through platform/config, a real websocket through the transport, a
// real archive through the writer, checked by the real verify. Every seam this
// package wires is on the path, and the assertion at the end is the one the
// tool exists for — the archive holds the venue's frames and nothing about it
// lies.
func TestCaptureRunEndToEnd(t *testing.T) {
	frames := []string{`{"n":1}`, `{"n":2}`, `{"n":3}`}
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow() //nolint:errcheck
		for _, f := range frames {
			if err := c.Write(r.Context(), websocket.MessageText, []byte(f)); err != nil {
				return
			}
		}
		// Stay connected and silent: the test must see exactly one session's
		// worth of frames, not a reconnect loop's.
		select {
		case <-hold:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(hold) // runs before srv.Close, releasing the handler

	root := t.TempDir()
	cfg := validConfig()
	cfg.Root = root
	cfg.SyncInterval = config.NewDuration(200 * time.Millisecond)
	cfg.ReadTimeout = config.NewDuration(30 * time.Second)
	src := &cfg.Sources[0]
	src.Scheme = "ws"
	src.Host = strings.TrimPrefix(srv.URL, "http://")
	src.Prefix = ""
	src.Topic = "e2e"

	// Through the real loader, so the schema round-trips: what this test runs
	// is a document, not a struct a test happened to build.
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	wall, err := clock.NewWall(nil)
	if err != nil {
		t.Fatal(err)
	}
	start := wall.Now()
	store, err := config.New[Config]("e2e-fixture", start, data)
	if err != nil {
		t.Fatalf("the marshalled config does not load back: %v", err)
	}

	reg := observability.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(ctx, store.Current(), wall, reg, Build{Tool: "e2e", GitSHA: "e2e"})
	}()

	// Wait, bounded, for the three frames to land — observed through the
	// metrics, which also proves the export path runs.
	deadline := time.Now().Add(30 * time.Second)
	for {
		var got int64
		for _, s := range reg.Snapshot() {
			if s.Name == "capture_data_records_total" {
				got = s.Value.Mantissa()
			}
		}
		if got >= int64(len(frames)) {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("captured %d frames within the bound, want %d", got, len(frames))
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}

	// The run may have straddled an hour boundary; claim both possible
	// windows and sum. Absent-window findings are expected and are coverage;
	// what must NOT appear is a single integrity finding.
	from := enginecapture.Window(start, time.Hour)
	rep, err := enginecapture.Verify(cfg.Verify(from, from.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	var dataRecords uint64
	for _, w := range rep.Windows {
		dataRecords += w.DataRecords
	}
	if dataRecords != uint64(len(frames)) {
		t.Errorf("the archive holds %d venue frames, want %d", dataRecords, len(frames))
	}
	for _, f := range rep.Findings {
		if f.Class() == enginecapture.ClassIntegrity {
			t.Errorf("integrity finding over a healthy end-to-end run: %s", f)
		}
	}
	if rej := reg.Rejected(); rej != 0 {
		t.Errorf("the registry rejected %d observations", rej)
	}
}
