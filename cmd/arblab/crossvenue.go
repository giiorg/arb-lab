package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/screening"
)

// crossvenueCheck is `arblab crossvenue check`: the §45d smoke-decode over
// one UTC day of the capture archive. The verify proves the bytes are intact;
// this proves they decode as the streams we think they are — it exists
// because the shortlist's 32 sources started recording 2026-08-01 and a
// wire-shape surprise should surface while the loss is hours, not weeks.
//
// The day is a REQUIRED flag, never a clock default — the same rule as the
// calibrate tool's freeze instant: a command that stamps "today" cannot be
// re-run to reproduce yesterday's answer.
func crossvenueCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("crossvenue check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "archive", "archive root (the directory holding v1/)")
	dayStr := fs.String("day", "", "UTC day to check, YYYY-MM-DD (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *dayStr == "" {
		fmt.Fprintln(stderr, "crossvenue check: -day is required")
		return 64
	}
	day, err := time.ParseInLocation("2006-01-02", *dayStr, time.UTC)
	if err != nil {
		fmt.Fprintf(stderr, "crossvenue check: -day %q: %v\n", *dayStr, err)
		return 64
	}

	rep, err := crossvenue.Check(*root, day)
	if err != nil {
		fmt.Fprintf(stderr, "crossvenue check: %v\n", err)
		return 1
	}

	for _, s := range rep.Streams {
		id := fmt.Sprintf("%s:%s:%s/%s", s.Venue, s.Product, s.Symbol, s.Topic)
		switch {
		case s.Skipped != "" && s.Data == 0 && s.Rest > 0:
			// A REST-only stream (the fundingInfo overlay): its records were
			// framing-decoded above; only a Data frame would have needed the
			// wire decoder the skip names.
			fmt.Fprintf(stdout, "  ok(rest) %-58s %6d rest framing-decoded  %d file(s)\n", id, s.Rest, s.Files)
		case s.Skipped != "":
			fmt.Fprintf(stdout, "  skip     %-58s %6d data  SKIPPED: %s\n", id, s.Data, s.Skipped)
		case s.Refused > 0:
			fmt.Fprintf(stdout, "  REFUSED  %-58s %6d data %6d decoded %6d refused  first: %s\n",
				id, s.Data, s.Decoded, s.Refused, s.FirstRefusal)
		default:
			torn := ""
			if s.TornTail {
				torn = "  (torn tail)"
			}
			fmt.Fprintf(stdout, "  ok       %-58s %6d data %6d decoded %6d emitted  %d file(s)%s\n",
				id, s.Data, s.Decoded, s.Emitted, s.Files, torn)
		}
	}
	fmt.Fprintf(stdout, "\n%d stream(s), %d refusal(s), day %s\n",
		len(rep.Streams), rep.Refusals(), rep.Day.Format("2006-01-02"))
	if rep.Refusals() > 0 {
		return 1
	}
	return 0
}

// crossvenueFetch is `arblab crossvenue fetch`: one settled-funding REST
// fetch per (venue, symbol) into the crossvenue store, verbatim pages
// digested. The cutoff is a REQUIRED instant (§41f's freeze rule); an
// identity already stored for that cutoff day is a named SKIP, the
// screening's resume discipline.
func crossvenueFetch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("crossvenue fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rest := fs.String("rest", "research/crossvenue/rest", "settled-funding store root (git-ignored)")
	symbolsArg := fs.String("symbols", "", "comma-separated symbols (required)")
	cutoffArg := fs.String("cutoff", "", "fetch history strictly before this instant, RFC3339 (required)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *symbolsArg == "" || *cutoffArg == "" {
		fmt.Fprintln(stderr, "crossvenue fetch: -symbols and -cutoff are required")
		return 64
	}
	cutoff, err := time.Parse(time.RFC3339, *cutoffArg)
	if err != nil {
		fmt.Fprintf(stderr, "crossvenue fetch: -cutoff %q: %v\n", *cutoffArg, err)
		return 64
	}

	t := screening.Transport{Get: politeGet()}
	fetchedAt := time.Now().UTC()
	failures := 0
	for _, sym := range strings.Split(*symbolsArg, ",") {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		for _, v := range []struct {
			name  string
			limit int
		}{{"binance", screening.BinancePageLimit}, {"bybit", screening.BybitPageLimit}} {
			path, digest, err := crossvenue.FetchSettled(t, *rest, v.name, sym, cutoff, fetchedAt, v.limit)
			if err != nil {
				if errors.Is(err, crossvenue.ErrStoreExists) {
					fmt.Fprintf(stdout, "  skip  %-8s %-14s already stored for %s\n", v.name, sym, cutoff.Format("2006-01-02"))
					continue
				}
				fmt.Fprintf(stderr, "crossvenue fetch: %s %s: %v\n", v.name, sym, err)
				failures++
				continue
			}
			fmt.Fprintf(stdout, "  ok    %-8s %-14s %s sha256=%s\n", v.name, sym, path, digest[:12])
		}
	}
	if failures > 0 {
		return 1
	}
	return 0
}

// simParamsFile is the exploration procedure as the operator writes it:
// every value a string in the units the domain parses, no defaults — an
// absent field fails validation rather than silently choosing.
type simParamsFile struct {
	Label   string `json:"label"`
	Symbols []struct {
		Symbol string `json:"symbol"`
		Qty    string `json:"qty"`
		Entry  string `json:"entry"`
	} `json:"symbols"`
	SampleEvery string `json:"sample_every"`
	Slack       string `json:"slack"`
	ExitDivisor int    `json:"exit_divisor"`
	MaxGap      string `json:"max_gap"`
	Round       string `json:"round"`
	BinanceFee  string `json:"binance_fee"`
	BybitFee    string `json:"bybit_fee"`
	P90         string `json:"p90"`
	Period      string `json:"period"`
	Policy      []struct {
		Symbol string `json:"symbol"`
		Weight string `json:"weight"`
	} `json:"policy"`
	MeanBlock  int    `json:"mean_block"`
	Resamples  int    `json:"resamples"`
	Confidence string `json:"confidence"`
	Seed       uint64 `json:"seed"`

	// VoidImbalance mirrors the gate's §45n-3 episode-integrity rule so
	// exploration can exercise what the gate enforces.
	VoidImbalance bool `json:"void_imbalance"`
}

func loadSimParams(path string) (crossvenue.SimParams, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return crossvenue.SimParams{}, err
	}
	var f simParamsFile
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return crossvenue.SimParams{}, fmt.Errorf("params %s: %w", path, err)
	}

	dur := func(name, s string) (time.Duration, error) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("params %s: %s %q: %w", path, name, s, err)
		}
		return d, nil
	}
	rate := func(name, s string) (fixedpoint.Rate, error) {
		r, err := fixedpoint.ParseRate(s)
		if err != nil {
			return fixedpoint.Rate{}, fmt.Errorf("params %s: %s %q: %w", path, name, s, err)
		}
		return r, nil
	}

	p := crossvenue.SimParams{
		Label:         f.Label,
		ExitDivisor:   f.ExitDivisor,
		MeanBlock:     f.MeanBlock,
		Resamples:     f.Resamples,
		Seed:          f.Seed,
		VoidImbalance: f.VoidImbalance,
	}
	var err2 error
	if p.Sample.Every, err2 = dur("sample_every", f.SampleEvery); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.Sample.Slack, err2 = dur("slack", f.Slack); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.MaxGap, err2 = dur("max_gap", f.MaxGap); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.Round, err2 = dur("round", f.Round); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.Period, err2 = dur("period", f.Period); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.BinanceFee, err2 = rate("binance_fee", f.BinanceFee); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.BybitFee, err2 = rate("bybit_fee", f.BybitFee); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.P90, err2 = rate("p90", f.P90); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	if p.Confidence, err2 = rate("confidence", f.Confidence); err2 != nil {
		return crossvenue.SimParams{}, err2
	}
	for _, s := range f.Symbols {
		base, ok := strings.CutSuffix(s.Symbol, "USDT")
		if !ok || base == "" {
			return crossvenue.SimParams{}, fmt.Errorf("params %s: symbol %q is not a USDT pair", path, s.Symbol)
		}
		qty, err := fixedpoint.ParseQuantity(fixedpoint.Asset(base), s.Qty)
		if err != nil {
			return crossvenue.SimParams{}, fmt.Errorf("params %s: qty for %s: %w", path, s.Symbol, err)
		}
		entry, err := rate("entry for "+s.Symbol, s.Entry)
		if err != nil {
			return crossvenue.SimParams{}, err
		}
		p.Symbols = append(p.Symbols, crossvenue.SymbolSpec{Symbol: s.Symbol, Qty: qty, Entry: entry})
	}
	for _, a := range f.Policy {
		w, err := rate("policy weight", a.Weight)
		if err != nil {
			return crossvenue.SimParams{}, err
		}
		p.Policy = append(p.Policy, dcv.Allocation{Symbol: a.Symbol, Weight: w})
	}
	return p, nil
}

// crossvenueSim is `arblab crossvenue sim`: the §45e research simulator
// over the PRE-CUTOFF corpus — the post-cutoff seal is structural in the
// package (ruling §45f-2), and the run refuses an unlabeled procedure or
// an output path already written (the labeled-and-logged trail is
// write-once).
func crossvenueSim(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("crossvenue sim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "archive", "archive root (the directory holding v1/)")
	rest := fs.String("rest", "research/crossvenue/rest", "settled-funding store root")
	paramsPath := fs.String("params", "", "exploration parameters JSON (required)")
	out := fs.String("out", "", "report output path (required; refuses an existing file)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *paramsPath == "" || *out == "" {
		fmt.Fprintln(stderr, "crossvenue sim: -params and -out are required")
		return 64
	}
	if _, err := os.Stat(*out); err == nil {
		fmt.Fprintf(stderr, "crossvenue sim: %s exists — exploration reports are write-once, pick a new label\n", *out)
		return 1
	}

	p, err := loadSimParams(*paramsPath)
	if err != nil {
		fmt.Fprintln(stderr, "crossvenue sim:", err)
		return 64
	}

	rep, err := crossvenue.RunSim(*root, *rest, p)
	if err != nil {
		fmt.Fprintln(stderr, "crossvenue sim:", err)
		return 1
	}

	data, err := json.MarshalIndent(rep, "", " ")
	if err != nil {
		fmt.Fprintln(stderr, "crossvenue sim: encode:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "crossvenue sim:", err)
		return 1
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(stderr, "crossvenue sim:", err)
		return 1
	}

	fmt.Fprintf(stdout, "sim %q over %d day(s) (%s..%s)\n", rep.Label, len(rep.Days), rep.Days[0], rep.Days[len(rep.Days)-1])
	for _, s := range rep.Symbols {
		fmt.Fprintf(stdout, "  %-14s points=%d episodes=%d scored=%d refused=%d matched=%d sealed-excluded=%d\n",
			s.Symbol, s.Merge.Joined, len(s.Episodes), s.Scored, s.RefusedN, s.Pairing.Matched, s.SealedExcluded)
	}
	fmt.Fprintf(stdout, "  portfolio: %d period(s), LCB %s\n", rep.Periods, rep.PortfolioLCB)
	fmt.Fprintf(stdout, "  report: %s\n", *out)
	return 0
}
