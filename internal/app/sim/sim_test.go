package sim_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/giiorg/arb-lab/internal/app/sim"
	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/dispatch"
	"github.com/giiorg/arb-lab/internal/engine/replay"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// The committed fixture (§39p-8): three real production streams over
// 2026-07-30T15:30:40Z–15:33:40Z, cut at the 15:30:40 restart seam so a cold
// sim acquires complete state from real frames inside the first second — the
// fundingInfo poll at …40.415Z, the Bybit post-resubscribe snapshot at
// …40.947Z.
const fixtureRoot = "testdata/archive"

func binancePerp() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "binance", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}
}

func bybitPerp() ids.VenueInstrument {
	return ids.VenueInstrument{Venue: "bybit", Product: ids.ProductLinearPerp, Symbol: "BTCUSDT", Contract: ids.ContractLinear}
}

func fixtureStreams() []replay.StreamSpec {
	return []replay.StreamSpec{
		{
			Key:     capture.StreamKey{Source: events.SourceID{Instrument: binancePerp(), Stream: events.StreamFunding}, Topic: "btcusdt@markPrice@1s"},
			Decoder: replay.DecodeBinanceMarkPrice,
		},
		{
			Key:     capture.StreamKey{Source: events.SourceID{Instrument: binancePerp(), Stream: events.StreamFunding}, Topic: "fundingInfo"},
			Decoder: replay.DecodeBinanceFundingInfo,
		},
		{
			Key:     capture.StreamKey{Source: events.SourceID{Instrument: bybitPerp(), Stream: events.StreamFunding}, Topic: "tickers.BTCUSDT"},
			Decoder: replay.DecodeBybitTicker,
		},
	}
}

// The budgets are the probe's lever (§39p-8): Bybit's funding-bearing
// messages arrive ~60s apart, so MaxAge 30s fires real silence timers BETWEEN
// real messages — the absence door exercised on production data. Binance at
// 1/s never goes stale under the same budget, the mirror that proves the
// timers are per-instrument. The book section is required by the config's own
// validation even though no book stream is wired.
const freshnessJSON = `{
	"funding": {
		"binance": {"max_age": "30s", "blind": "90s"},
		"bybit":   {"max_age": "30s", "blind": "90s"}
	},
	"book": {
		"binance": {"max_age": "200ms", "blind": "2s"},
		"bybit":   {"max_age": "200ms", "blind": "2s"}
	}
}`

func fixtureConfig(t *testing.T, logPath string) sim.Config {
	t.Helper()
	return sim.Config{
		Root:          fixtureRoot,
		Streams:       fixtureStreams(),
		FreshnessJSON: []byte(freshnessJSON),
		MailboxDepth:  64,
		LogPath:       logPath,
	}
}

// logTally counts a dispatch log's entry kinds and remembers whether any
// envelope carried the absence-derived zero ordinal.
type logTally struct {
	envelopes, timers int
	zeroOrdinal       bool
}

func (c *logTally) OnEnvelopeRecord(r dispatch.EnvelopeRecord) error {
	c.envelopes++
	if r.AcceptOrdinal == 0 {
		c.zeroOrdinal = true
	}
	return nil
}

func (c *logTally) OnTimerRecord(dispatch.TimerRecord) error {
	c.timers++
	return nil
}

// TestParityProbe is §39j slice C's done-when, whole: two Generate runs over
// one archive produce byte-identical dispatch logs and equal terminal cache
// states, and a Rerun following the first log re-drives the same terminal
// state while re-writing a byte-identical log (§39p-7's strongest form).
func TestParityProbe(t *testing.T) {
	dir := t.TempDir()
	log1 := filepath.Join(dir, "run1.dlog")
	log2 := filepath.Join(dir, "run2.dlog")
	log3 := filepath.Join(dir, "rerun.dlog")

	r1, err := sim.Generate(fixtureConfig(t, log1))
	if err != nil {
		t.Fatalf("Generate run 1: %v", err)
	}
	r2, err := sim.Generate(fixtureConfig(t, log2))
	if err != nil {
		t.Fatalf("Generate run 2: %v", err)
	}

	b1 := readFile(t, log1)
	if b2 := readFile(t, log2); !bytes.Equal(b1, b2) {
		t.Fatalf("two Generate runs wrote different logs (%d vs %d bytes)", len(b1), len(b2))
	}
	if !reflect.DeepEqual(r1.Funding, r2.Funding) {
		t.Fatalf("two Generate runs reached different terminal cache states:\nrun1: %+v\nrun2: %+v", r1.Funding, r2.Funding)
	}

	r3, err := sim.Rerun(fixtureConfig(t, log3), log1)
	if err != nil {
		t.Fatalf("Rerun following run 1's log: %v", err)
	}
	if b3 := readFile(t, log3); !bytes.Equal(b1, b3) {
		t.Fatalf("Rerun rewrote a different log (%d vs %d bytes)", len(b1), len(b3))
	}
	if !reflect.DeepEqual(r1.Funding, r3.Funding) {
		t.Fatalf("Rerun reached a different terminal cache state:\ngenerate: %+v\nrerun: %+v", r1.Funding, r3.Funding)
	}
	if !r3.FollowedLog.Closed {
		t.Errorf("followed log not sealed: %+v", r3.FollowedLog)
	}
}

// TestProbeIsNotVacuous pins what the fixture must actually contain for the
// parity comparison to be evidence — the §38h lesson that a clean run and a
// vacuous run look identical. Each assertion names the distinction it keeps
// alive.
func TestProbeIsNotVacuous(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.dlog")
	r, err := sim.Generate(fixtureConfig(t, logPath))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Both venues must reach the cache, or the probe proves one-source
	// dispatch — §39j's named collapse.
	if _, ok := r.Funding[binancePerp()]; !ok {
		t.Errorf("no terminal binance funding entry — the overlay join never composed")
	}
	if _, ok := r.Funding[bybitPerp()]; !ok {
		t.Errorf("no terminal bybit funding entry — the ticker accumulator never completed")
	}

	var tally logTally
	if _, err := dispatch.ReplayLog(logPath, &tally); err != nil {
		t.Fatalf("ReplayLog over generate output: %v", err)
	}
	// Timer entries prove the silence path ran inside the window; a zero
	// ordinal proves the absence door passed a re-judged envelope end to end
	// (§39p-5). Without these the probe never exercises the door it opened.
	if tally.timers == 0 {
		t.Errorf("no timer entries in the log — no silence timer fired inside the window")
	}
	if !tally.zeroOrdinal {
		t.Errorf("no absence-derived envelope in the log — the re-judged snapshot never dispatched")
	}
	if tally.envelopes == 0 || r.Report.Emitted == 0 {
		t.Errorf("log holds %d envelopes, run emitted %d — an empty world proves nothing", tally.envelopes, r.Report.Emitted)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
