package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driving"
)

// calibrateFetch is `arblab calibrate fetch`: pull one closed family-1
// dataset identity (§41c) from a venue's public REST history and commit
// it under datasets/. The fetch refuses a holdout-era identity (H1), a
// range the venue cannot fill completely (H3), and an existing file
// (write-once); what it writes is the venue's words, verbatim, digested.
func calibrateFetch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("calibrate fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	venue := fs.String("venue", "", "venue (binance|bybit)")
	series := fs.String("series", "", "series (funding|spot_kline_1h|mark_kline_1h)")
	symbol := fs.String("symbol", "", "symbol (e.g. BTCUSDT)")
	start := fs.String("start", "", "first grid instant, RFC3339")
	end := fs.String("end", "", "last grid instant, RFC3339 (inclusive)")
	out := fs.String("out", "datasets", "dataset root directory")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *venue == "" || *series == "" || *symbol == "" || *start == "" || *end == "" {
		fmt.Fprintln(stderr, "calibrate fetch: -venue, -series, -symbol, -start and -end are required")
		return 64
	}
	startAt, err := time.Parse(time.RFC3339, *start)
	if err != nil {
		fmt.Fprintln(stderr, "calibrate fetch: -start:", err)
		return 64
	}
	endAt, err := time.Parse(time.RFC3339, *end)
	if err != nil {
		fmt.Fprintln(stderr, "calibrate fetch: -end:", err)
		return 64
	}

	client := &http.Client{Timeout: 30 * time.Second}
	fetcher := driving.Fetcher{Get: func(url string) ([]byte, error) {
		// A fixed pause per page: the largest fetch is under thirty
		// pages, and politeness beats a ban mid-identity.
		time.Sleep(300 * time.Millisecond)
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: HTTP %d: %s", url, resp.StatusCode, body)
		}
		return body, nil
	}}

	file, err := fetcher.Fetch(ids.Venue(*venue), *series, *symbol, startAt, endAt, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(stderr, "calibrate fetch:", err)
		return 1
	}
	path, err := driving.DatasetPath(*out, file)
	if err != nil {
		fmt.Fprintln(stderr, "calibrate fetch:", err)
		return 1
	}
	digest, err := driving.WriteDatasetFile(path, file)
	if err != nil {
		fmt.Fprintln(stderr, "calibrate fetch:", err)
		return 1
	}
	fmt.Fprintf(stdout, "calibrate fetch: %s (%d pages)\n  sha256 %s\n", path, len(file.Pages), digest)
	return 0
}

// calibrateRun is `arblab calibrate run`: the governed derivation (§41e).
// The freeze instant is an ARGUMENT — the one wall-clock read here is the
// operator's stated intent, recorded on the command line, never an
// ambient default: a derivation that stamps "now" cannot produce
// byte-identical artifacts across the two invocations §41i demands.
func calibrateRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("calibrate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	datasets := fs.String("datasets", "datasets", "committed dataset root")
	archive := fs.String("archive", "archive", "capture archive root")
	reference := fs.String("reference", "configs/reference.btcusdt.json", "committed reference revision")
	out := fs.String("out", "configs/calibration.btcusdt.v1.json", "artifact output path (write-once)")
	freeze := fs.String("freeze", "", "freeze instant, RFC3339 (required; derivation reads archive days strictly before its UTC day)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *freeze == "" {
		fmt.Fprintln(stderr, "calibrate run: -freeze is required")
		return 64
	}
	freezeAt, err := time.Parse(time.RFC3339, *freeze)
	if err != nil {
		fmt.Fprintln(stderr, "calibrate run: -freeze:", err)
		return 64
	}

	res, err := driving.Calibrator{}.Run(driving.RunConfig{
		DatasetsDir:   *datasets,
		ArchiveRoot:   *archive,
		ReferencePath: *reference,
		OutPath:       *out,
		Freeze:        freezeAt,
	})
	if err != nil {
		fmt.Fprintln(stderr, "calibrate run:", err)
		return 1
	}
	fmt.Fprintf(stdout, "calibrate run: %s\n  id %s\n  sha256 %s\n", res.ArtifactPath, res.ArtifactID, res.Hash)
	return 0
}
