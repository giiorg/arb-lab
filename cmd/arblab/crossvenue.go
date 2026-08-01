package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/crossvenue"
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
