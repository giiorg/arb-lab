package capture

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/config"
)

// realConfigPath is the checked-in capture set — §32c's seven BTC sources
// plus §44's thirty-two shortlist sources (eight symbols × four streams).
// Tests load the real document rather than a fixture copy so there is exactly
// one spelling of the capture set in the repository, and the tests are what
// keep it loadable.
const realConfigPath = "../../../configs/capture.json"

func loadReal(t *testing.T) config.Snapshot[Config] {
	t.Helper()
	data, err := os.ReadFile(realConfigPath)
	if err != nil {
		t.Fatalf("reading the checked-in capture config: %v", err)
	}
	store, err := config.New[Config](realConfigPath, time.Unix(1753747200, 0), data)
	if err != nil {
		t.Fatalf("loading the checked-in capture config: %v", err)
	}
	return store.Current()
}

// TestRealCaptureConfigLoads is the schema's positive control: the document
// the operator will actually run is loadable, valid, and holds §32c's seven
// BTC sources plus §44's shortlist expansion. A schema test suite that only
// ever loads synthetic fixtures can pass while the real document does not
// parse.
func TestRealCaptureConfigLoads(t *testing.T) {
	snap := loadReal(t)
	if got := len(snap.Value.Sources); got != 39 {
		t.Fatalf("the capture set has %d sources, want §32c's 7 + §44's 32", got)
	}

	// §44b's roster shape: BTCUSDT keeps §32c's seven exactly, and each
	// shortlisted symbol has exactly four — both venues' perp funding and
	// perp book, nothing else (no spot legs, no diff evidence stream).
	shortlist := []string{
		"LABUSDT", "BEATUSDT", "KAITOUSDT", "MMTUSDT",
		"MUSDT", "SOONUSDT", "ESPORTSUSDT", "BCHUSDT",
	}
	bySymbol := make(map[string][]SourceConfig)
	for _, s := range snap.Value.Sources {
		bySymbol[s.Symbol] = append(bySymbol[s.Symbol], s)
	}
	if got := len(bySymbol["BTCUSDT"]); got != 7 {
		t.Errorf("BTCUSDT has %d sources, want §32c's 7 unchanged", got)
	}
	for _, sym := range shortlist {
		srcs := bySymbol[sym]
		if len(srcs) != 4 {
			t.Errorf("%s has %d sources, want §44b's 4", sym, len(srcs))
			continue
		}
		kinds := make(map[string]int)
		for _, s := range srcs {
			if s.Product != "linear_perp" {
				t.Errorf("%s has a %q source; §44b has no spot legs", sym, s.Product)
			}
			kinds[string(s.Venue)+"/"+string(s.Stream)]++
		}
		for _, want := range []string{
			"binance/funding", "binance/book", "bybit/funding", "bybit/book",
		} {
			if kinds[want] != 1 {
				t.Errorf("%s has %d %s sources, want exactly 1", sym, kinds[want], want)
			}
		}
	}

	// §36c's pair: Bybit's linear and spot books have byte-identical topics
	// and are distinguished only by endpoint. The config must express them as
	// two distinct capture keys — this is the finding that forced the
	// host+prefix+topic split, asserted against the real document.
	keys := make(map[string]int)
	topics := make(map[string]int)
	for _, s := range snap.Value.Sources {
		keys[s.Key().String()]++
		topics[s.Topic]++
	}
	if len(keys) != 39 {
		t.Errorf("the 39 sources produce %d distinct capture keys, want 39", len(keys))
	}
	if topics["orderbook.50.BTCUSDT"] != 2 {
		t.Errorf("the identical-topic pair is not in the real config (found %d); "+
			"the fixture no longer contains the thing §36c is about",
			topics["orderbook.50.BTCUSDT"])
	}

	// Every source must yield a transport config the transport itself
	// accepts — the schema is only as good as the StreamConfig it builds.
	for i, src := range snap.Value.Sources {
		if err := snap.Value.streamConfig(src, nopSink{}).Validate(); err != nil {
			t.Errorf("sources[%d] (%s) builds an invalid stream config: %v", i, src.Key(), err)
		}
	}

	// §32h's overlay: one row, and it is the Profiles-verified target — the
	// unauthenticated fundingInfo GET, hourly. Asserted against the real
	// document for the same reason the seven are.
	//
	// [SPEC EDIT LANDED — v0.9.14] This assertion widens the capture set the
	// core states — the v0.9.13 rows-1/4 lesson said a scope asserted in
	// internal/ with no marker expires silently, so this one is marked at
	// the asserting site: core §8 and §12.2 now carry the overlay.
	if got := len(snap.Value.RestSources); got != 1 {
		t.Fatalf("the capture set has %d rest sources, want §32h's 1", got)
	}
	r := snap.Value.RestSources[0]
	if want := "https://fapi.binance.com/fapi/v1/fundingInfo"; snap.Value.pollerConfig(r).URL() != want {
		t.Errorf("overlay URL %q, want the Profiles-verified %q",
			snap.Value.pollerConfig(r).URL(), want)
	}
	if r.PollInterval.Duration() != time.Hour {
		t.Errorf("poll_interval %s, want §32h's hourly", r.PollInterval)
	}
}

type nopSink struct{}

func (nopSink) Data([]byte)                    {}
func (nopSink) Subscribed(_, _ []byte)         {}
func (nopSink) Disrupted(transport.Disruption) {}

// validConfig is a minimal valid document for mutation-style refusal tests.
func validConfig() Config {
	return Config{
		Root:             "root",
		RotationPeriod:   config.NewDuration(time.Hour),
		AliveInterval:    config.NewDuration(time.Minute),
		SyncInterval:     config.NewDuration(5 * time.Second),
		QueueDepth:       16,
		MaxRecord:        1 << 20,
		MinAlivePercent:  90,
		DialTimeout:      config.NewDuration(time.Second),
		SubscribeTimeout: config.NewDuration(time.Second),
		ReadTimeout:      config.NewDuration(time.Second),
		Backoff:          []config.Duration{config.NewDuration(time.Second)},
		Sources:          []SourceConfig{validSource()},
	}
}

func validSource() SourceConfig {
	return SourceConfig{
		Venue: "binance", Product: "linear_perp", Contract: "linear",
		Symbol: "BTCUSDT", Stream: "book",
		Protocol: "binance-path", Scheme: "wss",
		Host: "fstream.binance.com", Prefix: "/public/ws",
		Topic:            "btcusdt@depth20@100ms",
		FirstDataTimeout: config.NewDuration(10 * time.Second),
	}
}

func TestConfigValidateRefusals(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Config)
		want error
	}{
		{"empty root", func(c *Config) { c.Root = "" }, ErrRootRequired},
		{"zero rotation", func(c *Config) { c.RotationPeriod = config.Duration{} }, ErrDurationRequired},
		{"zero alive", func(c *Config) { c.AliveInterval = config.Duration{} }, ErrDurationRequired},
		{"zero sync", func(c *Config) { c.SyncInterval = config.Duration{} }, ErrDurationRequired},
		{"zero dial timeout", func(c *Config) { c.DialTimeout = config.Duration{} }, ErrDurationRequired},
		{"zero subscribe timeout", func(c *Config) { c.SubscribeTimeout = config.Duration{} }, ErrDurationRequired},
		{"zero read timeout", func(c *Config) { c.ReadTimeout = config.Duration{} }, ErrDurationRequired},
		{"alive exceeds rotation", func(c *Config) {
			c.AliveInterval = config.NewDuration(time.Hour + time.Second)
		}, ErrAliveIntervalRange},
		{"zero queue depth", func(c *Config) { c.QueueDepth = 0 }, ErrQueueDepthRange},
		{"negative queue depth", func(c *Config) { c.QueueDepth = -1 }, ErrQueueDepthRange},
		{"zero max record", func(c *Config) { c.MaxRecord = 0 }, ErrMaxRecordRange},
		{"max record over the format cap", func(c *Config) {
			c.MaxRecord = enginecapture.MaxPayload + 1
		}, ErrMaxRecordRange},
		{"zero alive percent", func(c *Config) { c.MinAlivePercent = 0 }, ErrAlivePercentRange},
		{"alive percent over 100", func(c *Config) { c.MinAlivePercent = 101 }, ErrAlivePercentRange},
		{"empty backoff", func(c *Config) { c.Backoff = nil }, ErrBackoffRequired},
		{"zero backoff entry", func(c *Config) {
			c.Backoff = []config.Duration{config.NewDuration(time.Second), {}}
		}, ErrDurationRequired},
		{"no sources", func(c *Config) { c.Sources = nil }, ErrSourcesRequired},
		{"source without first-data timeout", func(c *Config) {
			c.Sources[0].FirstDataTimeout = config.Duration{}
		}, ErrTimeoutRequired},
		{"duplicate capture key", func(c *Config) {
			c.Sources = append(c.Sources, c.Sources[0])
		}, ErrSourceDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.edit(&cfg)
			if err := cfg.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestConfigValidateBoundaries pins the two range checks at their edges, where
// `<` and `<=` collapse in any fixture that sits strictly inside or outside.
func TestConfigValidateBoundaries(t *testing.T) {
	cfg := validConfig()
	cfg.AliveInterval = cfg.RotationPeriod // equal is legal; only exceeding is refused
	if err := cfg.Validate(); err != nil {
		t.Errorf("alive_interval == rotation_period refused: %v", err)
	}

	cfg = validConfig()
	cfg.MaxRecord = enginecapture.MaxPayload // exactly the cap is legal
	if err := cfg.Validate(); err != nil {
		t.Errorf("max_record == MaxPayload refused: %v", err)
	}

	cfg = validConfig()
	cfg.MinAlivePercent = 100
	if err := cfg.Validate(); err != nil {
		t.Errorf("min_alive_percent == 100 refused: %v", err)
	}
	cfg = validConfig()
	cfg.MinAlivePercent = 1
	if err := cfg.Validate(); err != nil {
		t.Errorf("min_alive_percent == 1 refused: %v", err)
	}
}

// TestIdenticalTopicsOnDistinctKeysAreNotDuplicates guards the refusal from
// the opposite side: the duplicate check must key on (SourceID, Topic), not on
// the topic — §36c's whole point is that Bybit's two books share a topic
// string legitimately.
func TestIdenticalTopicsOnDistinctKeysAreNotDuplicates(t *testing.T) {
	cfg := validConfig()
	spot := validSource()
	spot.Product, spot.Contract = "spot", ""
	spot.Prefix = "/ws"
	cfg.Sources = append(cfg.Sources, spot) // same topic, different SourceID
	if err := cfg.Validate(); err != nil {
		t.Fatalf("two sources sharing a topic on distinct keys refused: %v", err)
	}
}

// TestNoFieldIsHotReloadable asserts §32j's decision as behaviour: every
// change to the capture configuration is restart-required, so any edited
// document must be refused by Reload with ErrNotReloadable. Asserted through
// the Store rather than by reading struct tags, because the claim is about
// what a reload DOES and a `config:"hot"` added later must fail a test.
func TestNoFieldIsHotReloadable(t *testing.T) {
	data, err := os.ReadFile(realConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(1753747200, 0)
	store, err := config.New[Config](realConfigPath, at, data)
	if err != nil {
		t.Fatal(err)
	}

	// One top-level scalar, one nested source field, one list entry: the deny
	// must hold for the whole document tree, not only for its root.
	edits := []struct{ name, from, to string }{
		{"root", `"./archive"`, `"/somewhere/else"`},
		{"alive_interval", `"1m"`, `"2m"`},
		{"a source topic", `"btcusdt@markPrice@1s"`, `"btcusdt@markPrice@3s"`},
		{"a backoff entry", `"30s"`, `"31s"`},
	}
	for _, e := range edits {
		t.Run(e.name, func(t *testing.T) {
			doc := strings.Replace(string(data), e.from, e.to, 1)
			if doc == string(data) {
				t.Fatalf("the fixture does not contain %s — this edit tested nothing", e.from)
			}
			_, err := store.Reload(realConfigPath, at.Add(time.Minute), []byte(doc))
			if !errors.Is(err, config.ErrNotReloadable) {
				t.Fatalf("Reload after editing %s = %v, want ErrNotReloadable", e.name, err)
			}
		})
	}
}

// TestVerifyCarriesTheConfigsValues asserts the four values verify shares with
// the writer arrive under the same names, from the same struct (§36o): the
// coverage check is a comparison of the archive against the config, which only
// means something if both tools read one document.
func TestVerifyCarriesTheConfigsValues(t *testing.T) {
	cfg := validConfig()
	from, to := time.Unix(0, 0).UTC(), time.Unix(7200, 0).UTC()
	v := cfg.Verify(from, to)
	if v.Root != cfg.Root {
		t.Errorf("Root %q, want %q", v.Root, cfg.Root)
	}
	if v.RotationPeriod != cfg.RotationPeriod.Duration() {
		t.Errorf("RotationPeriod %s, want %s", v.RotationPeriod, cfg.RotationPeriod)
	}
	if v.AliveInterval != cfg.AliveInterval.Duration() {
		t.Errorf("AliveInterval %s, want %s", v.AliveInterval, cfg.AliveInterval)
	}
	if v.MinAlivePercent != cfg.MinAlivePercent {
		t.Errorf("MinAlivePercent %d, want %d", v.MinAlivePercent, cfg.MinAlivePercent)
	}
	if !v.From.Equal(from) || !v.To.Equal(to) {
		t.Errorf("range %s..%s, want %s..%s", v.From, v.To, from, to)
	}
}

// TestStreamConfigCarriesMaxRecordAsReadLimit pins the socket-side half of the
// max_record contract: a frame the archive could not hold is refused at the
// transport, which only happens if the limit actually reaches the socket.
func TestStreamConfigCarriesMaxRecordAsReadLimit(t *testing.T) {
	cfg := validConfig()
	sc := cfg.streamConfig(cfg.Sources[0], nopSink{})
	if sc.ReadLimit != cfg.MaxRecord {
		t.Fatalf("ReadLimit %d, want max_record %d", sc.ReadLimit, cfg.MaxRecord)
	}
	if sc.FirstDataTimeout != cfg.Sources[0].FirstDataTimeout.Duration() {
		t.Fatalf("FirstDataTimeout %s is not the source's own", sc.FirstDataTimeout)
	}
}
