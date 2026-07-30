package capture

import (
	"fmt"
	"time"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/platform/events"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Config is §32j's capture configuration, loaded through platform/config.
//
// The SCHEMA lives here rather than in platform/config, and that placement is
// the kernel package's own rule: it owns the mechanism — strict decode,
// Validate, the hot-reload policy, revisions — and every schema belongs to the
// code that owns its meaning. Capture's meaning lives in this composition root.
//
// # No field is hot-reloadable, and that is written by omission
//
// platform/config is default-DENY: a field may change across a Reload only if
// it carries `config:"hot"`, and none of these do. §32j's argument: every
// change is restart-required, a restart is a gap of a few seconds, and the gap
// is RECORDED — which is the honest trade against a hot-reload path whose
// failure mode is a source that silently stops. TestNoFieldIsHotReloadable
// asserts the refusal rather than the tags, because the claim is about
// behaviour and a tag added later should fail a test, not a reading pass.
//
// # Every field is required
//
// WriterConfig and StreamConfig both refuse zero values and say why: a default
// applied silently at a constructor is a value with no name. This schema is
// where the values GET their names, so a zero here is refused too — by
// Validate, naming the field, at boot, where an operator can still fix it.
type Config struct {
	// Root is the archive root — the directory the v<major> tree goes under.
	// The same value verify takes, so the two tools read one tree.
	Root string `json:"root"`

	// RotationPeriod is the archive file window (§32e says hourly), and
	// AliveInterval is the unconditional Alive tick cadence (§32g). Verify
	// takes both rather than inferring them from the archive, so its coverage
	// check compares the archive against the CONFIG (§36o).
	RotationPeriod config.Duration `json:"rotation_period"`
	AliveInterval  config.Duration `json:"alive_interval"`

	// SyncInterval is how often an open archive file is fsynced (§32f).
	SyncInterval config.Duration `json:"sync_interval"`

	// QueueDepth bounds §32f's reader→writer hand-off, per source. Overflow
	// drops newest and records the hole; depth zero would be an unbuffered
	// channel, which capture.NewQueue refuses.
	QueueDepth int `json:"queue_depth"`

	// MaxRecord caps one venue frame, in bytes. It is passed to the socket as
	// its read limit, so a frame the archive could not hold is refused at the
	// transport rather than at the writer — where it would already have been
	// counted as captured. Validate bounds it by the format's own MaxPayload
	// for exactly that reason.
	MaxRecord int64 `json:"max_record"`

	// MinAlivePercent is verify's coverage threshold, in whole percent of the
	// expected Alive count, 1..100. An integer because §3.6 rule 4 forbids
	// float64 and the comparison is ticks*100 >= expected*percent.
	MinAlivePercent uint `json:"min_alive_percent"`

	// DialTimeout, SubscribeTimeout and ReadTimeout are transport deadlines,
	// shared by every source. FirstDataTimeout is NOT here: it is per source,
	// because the honest value is a property of the feed (StreamConfig).
	DialTimeout      config.Duration `json:"dial_timeout"`
	SubscribeTimeout config.Duration `json:"subscribe_timeout"`
	ReadTimeout      config.Duration `json:"read_timeout"`

	// Backoff is the reconnect schedule, walked by attempt and holding at the
	// last entry. A slice rather than a formula so the schedule is exactly
	// what this file says.
	Backoff []config.Duration `json:"backoff"`

	// Sources is the capture set — §32c's seven rows, as data.
	Sources []SourceConfig `json:"sources"`
}

// SourceConfig is one capture stream: the identity half names §3.5's SourceID,
// and the wire half names where and how to connect.
//
// Host, Prefix and Topic are three separate fields, and that split is §36c's
// finding as schema: Binance runs three hosts with three path disciplines, and
// Bybit's linear and spot books have byte-identical topic strings distinguished
// only by URL. A config that carried a single endpoint-or-topic string could
// not express the capture set at all.
type SourceConfig struct {
	// Venue, Product, Contract, Symbol and Stream name the SourceID. Contract
	// is set exactly when Product is a derivative, per ids.VenueInstrument.
	Venue    ids.Venue           `json:"venue"`
	Product  ids.ProductCategory `json:"product"`
	Contract ids.ContractType    `json:"contract,omitempty"`
	Symbol   string              `json:"symbol"`
	Stream   events.StreamKind   `json:"stream"`

	// Protocol is the venue dialect; Scheme, Host and Prefix are the endpoint;
	// Topic is the venue's own subscription topic, verbatim — it is also the
	// archive directory segment, which is why the two halves live on one row.
	Protocol transport.ProtocolID `json:"protocol"`
	Scheme   string               `json:"scheme"`
	Host     string               `json:"host"`
	Prefix   string               `json:"prefix,omitempty"`
	Topic    string               `json:"topic"`

	// FirstDataTimeout is §32k's first-data assertion window, per source: a
	// mark-price stream at 1s and a thin spot book do not share a threshold,
	// and a single global value would be set to the slowest feed — which is
	// the same as not having the check on the faster ones.
	FirstDataTimeout config.Duration `json:"first_data_timeout"`
}

// Key is the (SourceID, Topic) capture key this source writes under (§32c).
func (s SourceConfig) Key() enginecapture.StreamKey {
	return enginecapture.StreamKey{
		Source: events.SourceID{
			Instrument: ids.VenueInstrument{
				Venue:    s.Venue,
				Product:  s.Product,
				Symbol:   s.Symbol,
				Contract: s.Contract,
			},
			Stream: s.Stream,
		},
		Topic: s.Topic,
	}
}

// Endpoint is the wire half as the transport's type.
func (s SourceConfig) Endpoint() transport.Endpoint {
	return transport.Endpoint{Scheme: s.Scheme, Host: s.Host, Prefix: s.Prefix}
}

// Validate reports whether the source could be captured. It delegates to the
// types that own each rule — StreamKey for the identity (including §3.5's
// product-vs-stream table) and Endpoint for the wire shape — so this schema
// cannot grow a second spelling of either.
func (s SourceConfig) Validate() error {
	if err := s.Key().Validate(); err != nil {
		return err
	}
	if err := s.Endpoint().Validate(); err != nil {
		return err
	}
	if err := s.Protocol.Validate(); err != nil {
		return err
	}
	if s.FirstDataTimeout.Duration() <= 0 {
		return fmt.Errorf("source %s: first_data_timeout %s: %w",
			s.Key(), s.FirstDataTimeout, ErrTimeoutRequired)
	}
	return nil
}

// Validate reports whether the configuration could run capture. It implements
// config.Validator, so a document that fails here never becomes a Snapshot.
func (c Config) Validate() error {
	if c.Root == "" {
		return ErrRootRequired
	}
	for name, d := range map[string]config.Duration{
		"rotation_period":   c.RotationPeriod,
		"alive_interval":    c.AliveInterval,
		"sync_interval":     c.SyncInterval,
		"dial_timeout":      c.DialTimeout,
		"subscribe_timeout": c.SubscribeTimeout,
		"read_timeout":      c.ReadTimeout,
	} {
		if d.Duration() <= 0 {
			return fmt.Errorf("%s %s: %w", name, d, ErrDurationRequired)
		}
	}
	// The coverage check is ticks against period/interval. An interval longer
	// than the period expects zero ticks per window, and a threshold over an
	// expectation of zero passes every window — a vacuous check that reads as
	// a passing one, which is this repo's recurring failure shape.
	if c.AliveInterval.Duration() > c.RotationPeriod.Duration() {
		return fmt.Errorf("alive_interval %s exceeds rotation_period %s: %w",
			c.AliveInterval, c.RotationPeriod, ErrAliveIntervalRange)
	}
	if c.QueueDepth <= 0 {
		return fmt.Errorf("queue_depth %d: %w", c.QueueDepth, ErrQueueDepthRange)
	}
	if c.MaxRecord <= 0 || c.MaxRecord > enginecapture.MaxPayload {
		return fmt.Errorf("max_record %d is outside 1..%d: %w",
			c.MaxRecord, int64(enginecapture.MaxPayload), ErrMaxRecordRange)
	}
	if c.MinAlivePercent == 0 || c.MinAlivePercent > 100 {
		return fmt.Errorf("min_alive_percent %d: %w", c.MinAlivePercent, ErrAlivePercentRange)
	}
	if len(c.Backoff) == 0 {
		return ErrBackoffRequired
	}
	for i, d := range c.Backoff {
		if d.Duration() <= 0 {
			return fmt.Errorf("backoff[%d] %s: %w", i, d, ErrDurationRequired)
		}
	}
	if len(c.Sources) == 0 {
		return ErrSourcesRequired
	}
	// Two sources with one capture key are two writers over one tree. The
	// flock refuses the second at runtime; refusing here names the row at
	// boot instead. Note what is NOT refused: identical topics on different
	// keys — Bybit's linear and spot books — which is §36c's whole point.
	seen := make(map[enginecapture.StreamKey]int, len(c.Sources))
	for i, s := range c.Sources {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("sources[%d]: %w", i, err)
		}
		key := s.Key()
		if j, dup := seen[key]; dup {
			return fmt.Errorf("sources[%d] and sources[%d] both name %s: %w",
				j, i, key, ErrSourceDuplicate)
		}
		seen[key] = i
	}
	return nil
}

// Verify is the capture.VerifyConfig this configuration implies for a window
// range. Verify takes the rotation period and Alive interval from here rather
// than inferring them from the archive, so the check compares the archive
// against the config (§36o) — and taking them from the SAME struct capture ran
// with is what makes that comparison mean something.
func (c Config) Verify(from, to time.Time) enginecapture.VerifyConfig {
	return enginecapture.VerifyConfig{
		Root:            c.Root,
		From:            from,
		To:              to,
		RotationPeriod:  c.RotationPeriod.Duration(),
		AliveInterval:   c.AliveInterval.Duration(),
		MinAlivePercent: c.MinAlivePercent,
	}
}

// streamConfig is the transport configuration for one source.
func (c Config) streamConfig(s SourceConfig, sink transport.Sink) transport.StreamConfig {
	backoff := make([]time.Duration, len(c.Backoff))
	for i, d := range c.Backoff {
		backoff[i] = d.Duration()
	}
	return transport.StreamConfig{
		Endpoint:         s.Endpoint(),
		Topic:            s.Topic,
		Protocol:         s.Protocol,
		Sink:             sink,
		DialTimeout:      c.DialTimeout.Duration(),
		SubscribeTimeout: c.SubscribeTimeout.Duration(),
		FirstDataTimeout: s.FirstDataTimeout.Duration(),
		ReadTimeout:      c.ReadTimeout.Duration(),
		Backoff:          backoff,
		ReadLimit:        c.MaxRecord,
	}
}
