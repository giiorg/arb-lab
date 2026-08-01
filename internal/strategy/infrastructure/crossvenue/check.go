package crossvenue

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// ErrNoStreams reports a check that found no archive file for the requested
// day. An empty walk is not a clean walk: a wrong -root or a wrong day would
// otherwise report "0 refusals" and read as a pass (the vacuous-run lesson).
var ErrNoStreams = errors.New("crossvenue: no archive files for the requested day")

// StreamCheck is one stream's aggregate over a UTC day's files.
type StreamCheck struct {
	// Venue, Product, Contract, Symbol and Topic identify the stream the way
	// the archive path does.
	Venue    string
	Product  string
	Contract string
	Symbol   string
	Topic    string

	// Files is how many hour files of the day contributed.
	Files int

	// Data, Rest and Meta count records by kind: venue frames, REST responses,
	// and capture bookkeeping (open/close/alive/gap/clock-step).
	Data int
	Rest int
	Meta int

	// Decoded counts data frames the stream's decoder consumed without
	// refusing — including transport chatter and no-news ticker deltas.
	// Emitted counts the subset that produced a reading or a book message.
	Decoded int
	Emitted int

	// Refused counts decode refusals; FirstRefusal keeps the first one's text
	// (with its capture sequence) so the report names a place to look rather
	// than a tally.
	Refused      int
	FirstRefusal string

	// TornTail reports a file that ended mid-record — a crash boundary,
	// reported here because a smoke check that silently measured around one
	// would under-count the day (the verify owns judging it).
	TornTail bool

	// Skipped, when non-empty, names why this stream was not decoded at all.
	// A skip is a named absence, never a silent one: the §42c evidence stream
	// has no decoder on purpose, and anything else showing up here is a
	// stream this package has not been taught.
	Skipped string
}

// Report is a whole day's smoke check.
type Report struct {
	Root string
	Day  time.Time

	// Streams is sorted by (venue, product, contract, symbol, topic) so two
	// runs over the same archive render identically.
	Streams []StreamCheck
}

// Refusals sums decode refusals across streams.
func (r Report) Refusals() int {
	n := 0
	for _, s := range r.Streams {
		n += s.Refused
	}
	return n
}

// Check walks every archive file whose window falls on the given UTC day and
// smoke-decodes each stream with the decoder that owns its wire. The day is
// an argument, never a clock read — the caller says which day, exactly as the
// calibration tool's freeze instant works (§41f).
func Check(root string, day time.Time) (Report, error) {
	day = day.UTC()
	rep := Report{Root: root, Day: day}
	perStream := map[string]*StreamCheck{}

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".arbcap") {
			return nil
		}
		key, window, perr := capture.ParseFilePath(root, p)
		if perr != nil {
			// Not a stream file (the _manifest tree, a stray) — the manifest
			// layer owns those; a smoke check that errored on them would
			// refuse the archive for containing its own bookkeeping.
			return nil
		}
		w := window.UTC()
		if w.Year() != day.Year() || w.Month() != day.Month() || w.Day() != day.Day() {
			return nil
		}
		return checkFile(p, key, perStream)
	})
	if walkErr != nil {
		return Report{}, fmt.Errorf("crossvenue check: %w", walkErr)
	}
	if len(perStream) == 0 {
		return Report{}, fmt.Errorf("crossvenue check %s day %s: %w",
			root, day.Format("2006-01-02"), ErrNoStreams)
	}

	for _, s := range perStream {
		rep.Streams = append(rep.Streams, *s)
	}
	sort.Slice(rep.Streams, func(i, j int) bool {
		a, b := rep.Streams[i], rep.Streams[j]
		ka := a.Venue + "|" + a.Product + "|" + a.Contract + "|" + a.Symbol + "|" + a.Topic
		kb := b.Venue + "|" + b.Product + "|" + b.Contract + "|" + b.Symbol + "|" + b.Topic
		return ka < kb
	})
	return rep, nil
}

func checkFile(p string, key capture.StreamKey, perStream map[string]*StreamCheck) error {
	inst := key.Source.Instrument
	id := string(inst.Venue) + "|" + string(inst.Product) + "|" + string(inst.Contract) + "|" + inst.Symbol + "|" + key.Topic
	s, ok := perStream[id]
	if !ok {
		s = &StreamCheck{
			Venue:    string(inst.Venue),
			Product:  string(inst.Product),
			Contract: string(inst.Contract),
			Symbol:   inst.Symbol,
			Topic:    key.Topic,
		}
		perStream[id] = s
	}
	s.Files++

	// The decoder is resolved per FILE so stateful decoders (the Bybit ticker
	// accumulator) start cold at each file boundary — a file replayed from a
	// mid-connection rotation emits nothing until its next snapshot, which is
	// §39p-2's documented cold start, counted here as decoded-not-emitted.
	decode, skip := decoderFor(key)
	if skip != "" {
		s.Skipped = skip
	}

	f, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("open %s: %w", p, err)
	}
	defer f.Close()

	sum, err := capture.WalkRecords(f, func(rec capture.Record, _ []byte) error {
		switch rec.Kind {
		case capture.RecordKindData:
			s.Data++
			if decode == nil {
				return nil
			}
			emitted, derr := decode(rec.Payload)
			if derr != nil {
				s.Refused++
				if s.FirstRefusal == "" {
					s.FirstRefusal = fmt.Sprintf("seq %d: %v", rec.CaptureSeq, derr)
				}
				return nil
			}
			s.Decoded++
			if emitted {
				s.Emitted++
			}
		case capture.RecordKindRest:
			s.Rest++
			if _, derr := capture.DecodeRestPayload(rec.Payload); derr != nil {
				s.Refused++
				if s.FirstRefusal == "" {
					s.FirstRefusal = fmt.Sprintf("seq %d: rest framing: %v", rec.CaptureSeq, derr)
				}
			}
		case capture.RecordKindUnspecified,
			capture.RecordKindOpen,
			capture.RecordKindAlive,
			capture.RecordKindGap,
			capture.RecordKindClose,
			capture.RecordKindClockStep:
			s.Meta++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", p, err)
	}
	if sum.TornBytes > 0 {
		s.TornTail = true
	}
	return nil
}

// decoderFor routes a stream to the decoder that owns its wire, or names why
// none does. Default-deny: an unrecognized stream is a named skip, because
// "decoded 0 of 0 recognized frames" reading as a pass is exactly the
// vacuous-run shape.
func decoderFor(key capture.StreamKey) (func(raw []byte) (bool, error), string) {
	inst := key.Source.Instrument
	topic := key.Topic

	switch string(inst.Venue) {
	case "bybit":
		switch {
		case strings.HasPrefix(topic, "tickers."):
			state := venue.NewBybitTickerState()
			return func(raw []byte) (bool, error) {
				_, emitted, err := state.Apply(raw)
				return emitted, err
			}, ""
		case strings.HasPrefix(topic, "orderbook."):
			base, quote, err := usdtPairAssets(inst.Symbol)
			if err != nil {
				return nil, err.Error()
			}
			return func(raw []byte) (bool, error) {
				_, emitted, err := venue.DecodeBybitOrderbook(raw, base, quote)
				return emitted, err
			}, ""
		}
		return nil, fmt.Sprintf("no decoder for bybit topic %q", topic)

	case "binance":
		switch {
		case strings.Contains(topic, "@markPrice"):
			return func(raw []byte) (bool, error) {
				_, err := venue.DecodeBinanceMarkPrice(raw)
				return err == nil, err
			}, ""
		case strings.Contains(topic, "@depth20@"):
			base, quote, err := usdtPairAssets(inst.Symbol)
			if err != nil {
				return nil, err.Error()
			}
			if inst.Product == ids.ProductSpot {
				return func(raw []byte) (bool, error) {
					_, err := venue.DecodeBinanceSpotDepth20(raw, base, quote)
					return err == nil, err
				}, ""
			}
			return func(raw []byte) (bool, error) {
				_, err := venue.DecodeBinanceFuturesDepth20(raw, base, quote)
				return err == nil, err
			}, ""
		case strings.Contains(topic, "@depth@"):
			// The BTC diff evidence stream: captured to prove the
			// reconstruction pipeline, deliberately never decoded here
			// (§42c's absence roster).
			return nil, "evidence stream, no decoder by design (§42c)"
		}
		return nil, fmt.Sprintf("no decoder for binance topic %q", topic)
	}
	return nil, fmt.Sprintf("unknown venue %q", inst.Venue)
}

// usdtPairAssets derives the two asset tags from a USDT-quoted symbol. Every
// stream in the §42c roster is a USDT pair; anything else is a stream this
// function must be taught, refused by name rather than guessed.
func usdtPairAssets(symbol string) (base, quote fixedpoint.Asset, err error) {
	b, ok := strings.CutSuffix(symbol, "USDT")
	if !ok || b == "" {
		return "", "", fmt.Errorf("symbol %q is not a USDT pair this check knows", symbol)
	}
	return fixedpoint.Asset(b), "USDT", nil
}
