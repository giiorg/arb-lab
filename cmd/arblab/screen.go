package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/screening"
)

// screenRun is `arblab screen run`: the §42b screening harness — both
// venues' settled-funding history for the intersection universe, the
// ranked shortlist, and the dislocation census. Parameters are pinned
// in the screening package at its own commit; the flags here are only
// paths and stage selection. The fetch stage is resume-safe: rerunning
// skips every file already on disk.
func screenRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("screen run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "research/screening/raw", "raw store root (git-ignored)")
	out := fs.String("out", "research/screening", "committed outputs directory")
	stage := fs.String("stage", "all", "fetch|compute|all")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *stage != "fetch" && *stage != "compute" && *stage != "all" {
		fmt.Fprintln(stderr, "screen run: -stage must be fetch, compute or all")
		return 64
	}

	h := screening.Harness{
		T:         screening.Transport{Get: politeGet()},
		Root:      *root,
		OutDir:    *out,
		FetchedAt: time.Now().UTC(),
		Logf: func(format string, args ...any) {
			fmt.Fprintf(stdout, "screen: "+format+"\n", args...)
		},
	}

	if *stage == "fetch" || *stage == "all" {
		if err := h.FetchStage(); err != nil {
			fmt.Fprintln(stderr, "screen run:", err)
			return 1
		}
	}
	if *stage == "compute" || *stage == "all" {
		sum, err := h.ComputeStage()
		if err != nil {
			fmt.Fprintln(stderr, "screen run:", err)
			return 1
		}
		fmt.Fprintf(stdout, "screen run: %d intersection pairs, %d ranked, %d ineligible, %d census events\n",
			sum.Universe, sum.Ranked, sum.Ineligible, sum.Events)
		fmt.Fprintf(stdout, "  shortlist: %s\n", strings.Join(sum.Shortlist, " "))
	}
	return 0
}

// politeGet wires real HTTP with per-host pacing and a bounded retry.
// Binance's fundingRate endpoint shares a 500-requests-per-5-minutes IP
// limit (profiles §7): 650 ms per request keeps a full run near 460.
// Bybit's public market endpoints are far looser; 150 ms is manners,
// not a ceiling. One retry after a long pause on HTTP 429/418 — a
// second refusal is an answer, not noise.
func politeGet() func(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var mu sync.Mutex
	nextByHost := map[string]time.Time{}
	// pace RESERVES the host's next slot under the lock, then sleeps
	// outside it — concurrent callers to one host queue up at the
	// paced spacing instead of racing through a stale timestamp.
	pace := func(url string) {
		host := "other"
		pause := 150 * time.Millisecond
		if strings.Contains(url, "binance.com") {
			host, pause = "binance", 650*time.Millisecond
		} else if strings.Contains(url, "bybit.com") {
			host = "bybit"
		}
		mu.Lock()
		slot := nextByHost[host]
		if now := time.Now(); slot.Before(now) {
			slot = now
		}
		nextByHost[host] = slot.Add(pause)
		mu.Unlock()
		time.Sleep(time.Until(slot))
	}
	getOnce := func(url string) (int, []byte, error) {
		pace(url)
		resp, err := client.Get(url)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp.StatusCode, nil, err
		}
		return resp.StatusCode, body, nil
	}
	return func(url string) ([]byte, error) {
		status, body, err := getOnce(url)
		if err == nil && status == http.StatusOK {
			return body, nil
		}
		if status == 429 || status == 418 {
			time.Sleep(2 * time.Minute)
		} else {
			time.Sleep(5 * time.Second)
		}
		status, body, err = getOnce(url)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%s: HTTP %d: %s", url, status, body)
		}
		return body, nil
	}
}
