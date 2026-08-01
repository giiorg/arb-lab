package driving

// The family-2 collectors (§41b, §41j slice C): the archive-reading half of
// the book-derived measurements. Everything a NUMBER means once two book
// instants are in hand lives in strategy/domain/calibrate (the pure half);
// what lives here is the committed OBSERVATION PROCEDURE the §41b-ii-i rule
// demands be defined rather than implied — the sampling cadence, the
// pairing rule, the gap and discontinuity treatment, and the H2 guard on
// every file read. The split is the gate's own: this file consumes
// engine/capture records and engine/venue decoders, which hexboundary
// forbids the domain to see.
//
// # The pairing rule, stated once
//
// Sample instants are the UTC epoch grid at SampleEvery. At instant t the
// book "at t" is the last book received AT OR BEFORE t, required fresh
// within PairSlack; the book "at t+latency" is the FIRST book received at
// or after t+latency, required within PairSlack of it. A pair that spans a
// recorded Gap is discarded — a reconnect hole is a discontinuity, not a
// 30-second adverse move. Records are processed in file order, which the
// capture contract makes receive order; the anchor is RecvTime on both
// legs because the spot depth wire carries no venue time at all (the
// decoders' stated fact), and one anchor for both legs is what makes the
// two legs' instants the same question.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"errors"

	"github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/engine/venue"
	mdomain "github.com/giiorg/arb-lab/internal/marketdata/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
	"github.com/giiorg/arb-lab/internal/strategy/domain/calibrate"
	driven "github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

// Family-2 sentinels.
var (
	// ErrArchiveCorrupt reports a manifested file that does not decode
	// end to end. A closed, vouched file is a complete record chain by
	// §32's contract; a torn one here is integrity damage, and measuring
	// around it would silently thin the sample.
	ErrArchiveCorrupt = fmt.Errorf("strategy/driving: manifested archive file does not decode")

	// ErrManifestMalformed reports a day manifest that does not parse as
	// capture's own schema.
	ErrManifestMalformed = fmt.Errorf("strategy/driving: day manifest does not parse")

	// ErrNoManifestDays reports an empty H2 universe: nothing manifested
	// strictly before the freeze day. The derivation cannot measure
	// family 2 from nothing, and an empty enumeration must refuse rather
	// than emit a books-free artifact wearing captured-books provenance.
	ErrNoManifestDays = fmt.Errorf("strategy/driving: no manifested archive day precedes the freeze day")
)

// ManifestDay is one H2-eligible archive day: its manifest file's digest
// (the identifier the artifact enumerates and step 5 refuses to score) and
// the file paths that manifest vouches for.
type ManifestDay struct {
	Day    string
	SHA256 string

	vouched map[string]bool
}

// Vouches reports whether the day's manifest lists the root-relative path.
func (d ManifestDay) Vouches(rel string) bool { return d.vouched[rel] }

// LoadManifestDays enumerates the H2 universe: every day manifest under
// the archive root whose day is STRICTLY BEFORE the freeze instant's UTC
// day, sorted ascending. The strict bound is the determinism rule: the
// freeze day's own manifest is still gaining hours while capture runs, so
// two invocations hours apart would read two different universes; a closed
// prior day's manifest is settled bytes.
func LoadManifestDays(root string, freeze time.Time) ([]ManifestDay, error) {
	if freeze.IsZero() {
		return nil, fmt.Errorf("freeze instant: %w", ErrInstantUnset)
	}
	freezeDay := freeze.UTC().Format("2006-01-02")
	dir := filepath.Join(root, "v1", "_manifest")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("manifest dir: %w", err)
	}
	var days []ManifestDay
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		day := strings.TrimSuffix(name, ".json")
		if day >= freezeDay {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("manifest %s: %w", day, err)
		}
		var m capture.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest %s: %w: %w", day, ErrManifestMalformed, err)
		}
		if m.Day != day {
			return nil, fmt.Errorf("manifest file %s claims day %s: %w", day, m.Day, ErrManifestMalformed)
		}
		vouched := make(map[string]bool, len(m.Entries))
		for _, entry := range m.Entries {
			vouched[entry.Path] = true
		}
		sum := sha256.Sum256(data)
		days = append(days, ManifestDay{Day: day, SHA256: hex.EncodeToString(sum[:]), vouched: vouched})
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("before %s: %w", freezeDay, ErrNoManifestDays)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Day < days[j].Day })
	return days, nil
}

// scanStream walks one stream's hour files across the H2 days in path
// order, decoding every record and handing it to fn. Every file read is
// H2-guarded: a file the day's manifest does not vouch for is refused by
// name (GuardArchiveDay's rule at file granularity). A stream directory
// absent for a day is coverage, not integrity — the verify layer owns that
// ledger — so it is skipped; a vouched file that fails to decode is
// integrity and refuses.
func scanStream(root string, key capture.StreamKey, days []ManifestDay, fn func(capture.Record) error) error {
	dir := key.Dir(root)
	for _, d := range days {
		// The day itself is manifested by construction — it came from
		// LoadManifestDays — so H2's teeth here are per FILE: a file the
		// day's manifest does not vouch for refuses below.
		dayDir := filepath.Join(dir, d.Day[0:4], d.Day[5:7], d.Day[8:10])
		entries, err := os.ReadDir(dayDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("day %s: %w", d.Day, err)
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".arbcap") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			full := filepath.Join(dayDir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				return fmt.Errorf("file %s: %w", full, err)
			}
			rel = filepath.ToSlash(rel)
			if !d.Vouches(rel) {
				return fmt.Errorf("file %s: %w", rel, ErrDayUnmanifested)
			}
			if err := scanFile(full, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// scanFile decodes one archive file end to end.
func scanFile(path string, fn func(capture.Record) error) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("archive file: %w", err)
	}
	off, err := capture.ParseFileHeader(data)
	if err != nil {
		return fmt.Errorf("%s: %w: %w", path, ErrArchiveCorrupt, err)
	}
	rest := data[off:]
	for len(rest) > 0 {
		rec, n, err := capture.DecodeRecord(rest)
		if err != nil {
			return fmt.Errorf("%s at offset %d: %w: %w", path, len(data)-len(rest), ErrArchiveCorrupt, err)
		}
		rest = rest[n:]
		if err := fn(rec); err != nil {
			return err
		}
	}
	return nil
}

// BookStreamSpec names one archived book stream and its wire dialect.
type BookStreamSpec struct {
	Key        capture.StreamKey
	Instrument ids.VenueInstrument

	// Decode is the engine/venue decoder for this stream's payloads.
	Decode func(raw []byte) (venue.BinanceBookSnapshot, error)
}

// Family2Params is the committed observation procedure — every value here
// is a procedure parameter pinned at the §41e commit, recorded in the
// artifact's method identities, and frozen with the artifact.
type Family2Params struct {
	// SampleEvery is the sampling cadence on the UTC epoch grid.
	SampleEvery time.Duration

	// Latency is the assumed decision-to-arrival bound (family 3's
	// declared value): the counterfactual walks the book this far after t.
	Latency time.Duration

	// PairSlack bounds both legs' staleness: the book at t must be at
	// most this old, the book after t+latency at most this late.
	PairSlack time.Duration

	// Candidates are the walked quantities — each is a cell's MaxQty.
	Candidates []fixedpoint.Quantity

	// Classifier is the frozen stratum classifier (§41b-ii-b), the same
	// parameters the artifact freezes.
	Classifier domain.StratumClassifier
}

// Validate reports whether the procedure is completely declared.
func (p Family2Params) Validate() error {
	if p.SampleEvery <= 0 || p.Latency <= 0 || p.PairSlack <= 0 {
		return fmt.Errorf("family-2 cadence/latency/slack: %w", domain.ErrUnset)
	}
	if len(p.Candidates) == 0 {
		return fmt.Errorf("family-2 candidates: %w", domain.ErrUnset)
	}
	for i, c := range p.Candidates {
		if err := c.Validate(); err != nil {
			return fmt.Errorf("family-2 candidate %d: %w", i, err)
		}
		if i > 0 {
			cmp, err := p.Candidates[i-1].Cmp(c)
			if err != nil {
				return fmt.Errorf("family-2 candidate %d: %w", i, err)
			}
			if cmp >= 0 {
				return fmt.Errorf("family-2 candidate %d not above %d: %w", i, i-1, ErrDatasetOrder)
			}
		}
	}
	return p.Classifier.Validate()
}

// CellKey keys one measured population.
type CellKey struct {
	Instrument ids.VenueInstrument
	Side       mdomain.BookSide
	Stratum    string
	MaxQty     fixedpoint.Quantity
}

// BookMeasurements is one stream's collected family-2 samples: raw, so the
// assembly applies the quantile selection and the MinEvidence rule in one
// place, visibly.
type BookMeasurements struct {
	// Slips maps each cell to its counterfactual slippage samples, in
	// observation order.
	Slips map[CellKey][]fixedpoint.Rate

	// Moves are the adverse fill move samples, both sides pooled.
	Moves []fixedpoint.Rate

	// Pairs counts completed book pairs; Skipped counts sampling instants
	// with no fresh book or no completable pair; DepthShort counts
	// (pair, side, candidate) observations excluded because the later
	// book could not fill the candidate — excluded-and-counted, §41b's
	// insufficient-depth rule made visible.
	Pairs, Skipped, DepthShort uint64
}

// bookInstant is one decoded book state.
type bookInstant struct {
	book mdomain.BookSnapshot
	at   time.Time
}

// pendingSample is a sampling instant waiting for its t+latency book.
type pendingSample struct {
	t  time.Time
	at bookInstant
}

// MeasureBooks runs the committed observation over one stream: §41b-ii's
// counterfactual per candidate per side, plus the execution-scale adverse
// move, per the pairing rule in the package comment.
func MeasureBooks(root string, days []ManifestDay, spec BookStreamSpec, p Family2Params) (BookMeasurements, error) {
	if err := p.Validate(); err != nil {
		return BookMeasurements{}, err
	}
	if spec.Decode == nil {
		return BookMeasurements{}, fmt.Errorf("book stream decoder: %w", domain.ErrUnset)
	}
	out := BookMeasurements{Slips: map[CellKey][]fixedpoint.Rate{}}

	var cur *bookInstant
	var pending []pendingSample
	var lastGrid time.Time

	measure := func(at bookInstant, later bookInstant) error {
		out.Pairs++
		for _, side := range []mdomain.BookSide{mdomain.SideBid, mdomain.SideAsk} {
			move, err := calibrate.AdverseFillMove(at.book, later.book, side)
			if err != nil {
				// An empty side at either instant measures nothing.
				out.Skipped++
				continue
			}
			out.Moves = append(out.Moves, move)

			feat, err := driven.BookFeaturesFor(at.book, side)
			if err != nil {
				out.Skipped++
				continue
			}
			stratum, err := p.Classifier.Classify(feat)
			if err != nil {
				// A book the frozen classifier cannot place is a refusal
				// at runtime; at measurement it is an unmeasurable
				// instant, counted.
				out.Skipped++
				continue
			}
			for _, cand := range p.Candidates {
				slip, err := calibrate.CounterfactualSlippage(at.book, later.book, side, cand)
				if err != nil {
					if errors.Is(err, calibrate.ErrDepthShort) {
						out.DepthShort++
						continue
					}
					return fmt.Errorf("counterfactual at %s: %w", at.at.Format(time.RFC3339Nano), err)
				}
				key := CellKey{Instrument: spec.Instrument, Side: side, Stratum: stratum, MaxQty: cand}
				out.Slips[key] = append(out.Slips[key], slip)
			}
		}
		return nil
	}

	err := scanStream(root, spec.Key, days, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			// A discontinuity: the held book and every pending pair
			// straddling it are unusable.
			cur = nil
			out.Skipped += uint64(len(pending))
			pending = nil
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no book.
			return nil
		}

		decoded, err := spec.Decode(rec.Payload)
		if err != nil {
			return fmt.Errorf("%s at %s: %w", spec.Key, rec.RecvTime.Format(time.RFC3339Nano), err)
		}
		book := bookInstant{
			at: rec.RecvTime,
			book: mdomain.BookSnapshot{
				Instrument: spec.Instrument,
				Bids:       toMarketLevels(decoded.Book.Bids),
				Asks:       toMarketLevels(decoded.Book.Asks),
				Watermark: mdomain.Watermark{
					SourceTime: rec.RecvTime,
					RecvTime:   rec.RecvTime,
					VenueSeq:   decoded.LastUpdateID,
				},
			},
		}

		// 1. Sampling instants strictly before this record use the book
		// held BEFORE it.
		grid := rec.RecvTime.Truncate(p.SampleEvery)
		if lastGrid.IsZero() {
			lastGrid = grid
		}
		for t := lastGrid.Add(p.SampleEvery); !t.After(rec.RecvTime); t = t.Add(p.SampleEvery) {
			if t.Equal(rec.RecvTime) {
				// The instant lands exactly on this record; it is
				// sampled on the NEXT record with this book as "at t".
				break
			}
			lastGrid = t
			if cur == nil || t.Sub(cur.at) > p.PairSlack {
				out.Skipped++
				continue
			}
			pending = append(pending, pendingSample{t: t, at: *cur})
		}

		// 2. This book completes or expires pending pairs.
		kept := pending[:0]
		for _, ps := range pending {
			due := ps.t.Add(p.Latency)
			switch {
			case rec.RecvTime.Before(due):
				kept = append(kept, ps)
			case rec.RecvTime.Sub(due) <= p.PairSlack:
				if err := measure(ps.at, book); err != nil {
					return err
				}
			default:
				out.Skipped++
			}
		}
		pending = kept

		// 3. The book becomes current.
		cur = &book
		return nil
	})
	if err != nil {
		return BookMeasurements{}, err
	}
	out.Skipped += uint64(len(pending))
	return out, nil
}

// ImminentMeasurements is the markPrice-stream half of family 2: residuals
// of the venue's live per-settlement estimate against the realized
// settlement, and the transport-latency samples that inform family 3's
// declaration.
type ImminentMeasurements struct {
	// Residuals are realized − estimate, one per sampled estimate whose
	// settlement was observed.
	Residuals []fixedpoint.Rate

	// Estimates counts sampled estimates; Unrealized counts those whose
	// settlement the archive never (or too staleley) observed — dropped
	// AND counted, the gap rule made visible.
	Estimates, Unrealized uint64

	// Latencies are RecvTime − EventTime per decoded frame.
	Latencies []time.Duration
}

// ImminentParams is the committed imminent-residual procedure.
type ImminentParams struct {
	// SampleEvery is the estimate sampling cadence.
	SampleEvery time.Duration

	// RealizedSlack bounds both the sampled estimate's staleness and how
	// far before its settlement instant the period's last frame may sit
	// and still testify to the settled rate.
	RealizedSlack time.Duration
}

// Validate reports whether the procedure is declared.
func (p ImminentParams) Validate() error {
	if p.SampleEvery <= 0 || p.RealizedSlack <= 0 {
		return fmt.Errorf("imminent cadence/slack: %w", domain.ErrUnset)
	}
	return nil
}

// imminentFrame is one decoded markPrice frame.
type imminentFrame struct {
	rate   fixedpoint.Rate
	settle time.Time
	recv   time.Time
}

// estimateSample is one sampled estimate awaiting its settlement.
type estimateSample struct {
	estimate fixedpoint.Rate
	settle   time.Time
}

// MeasureImminent runs the imminent-residual observation over the
// markPrice stream: estimates sampled on the epoch grid, the realized rate
// for a settlement taken from the period's LAST frame — the venue's own
// estimate converges to the value it settles, and the venue's published
// number IS the definition (the venue-published rule) — required within
// RealizedSlack of the settlement instant, or the period's samples are
// dropped and counted rather than graded against a stale reading.
func MeasureImminent(root string, days []ManifestDay, key capture.StreamKey, p ImminentParams) (ImminentMeasurements, error) {
	if err := p.Validate(); err != nil {
		return ImminentMeasurements{}, err
	}
	out := ImminentMeasurements{}

	var cur *imminentFrame
	var lastGrid time.Time
	var samples []estimateSample
	realized := map[time.Time]imminentFrame{}

	err := scanStream(root, key, days, func(rec capture.Record) error {
		switch rec.Kind {
		case capture.RecordKindGap:
			cur = nil
			return nil
		case capture.RecordKindData:
		case capture.RecordKindUnspecified, capture.RecordKindOpen, capture.RecordKindAlive,
			capture.RecordKindClose, capture.RecordKindRest, capture.RecordKindClockStep:
			// Framing and liveness records carry no estimate.
			return nil
		}
		m, err := venue.DecodeBinanceMarkPrice(rec.Payload)
		if err != nil {
			return fmt.Errorf("%s at %s: %w", key, rec.RecvTime.Format(time.RFC3339Nano), err)
		}
		out.Latencies = append(out.Latencies, rec.RecvTime.Sub(m.EventTime))
		frame := imminentFrame{rate: m.Rate, settle: m.NextSettle, recv: rec.RecvTime}

		// A settlement boundary: the previous period's last frame is its
		// realized testimony.
		if cur != nil && !cur.settle.Equal(frame.settle) {
			realized[cur.settle] = *cur
		}

		grid := rec.RecvTime.Truncate(p.SampleEvery)
		if lastGrid.IsZero() {
			lastGrid = grid
		}
		for t := lastGrid.Add(p.SampleEvery); !t.After(rec.RecvTime); t = t.Add(p.SampleEvery) {
			if t.Equal(rec.RecvTime) {
				break
			}
			lastGrid = t
			if cur == nil || t.Sub(cur.recv) > p.RealizedSlack {
				continue
			}
			out.Estimates++
			samples = append(samples, estimateSample{estimate: cur.rate, settle: cur.settle})
		}

		cur = &frame
		return nil
	})
	if err != nil {
		return ImminentMeasurements{}, err
	}

	for _, s := range samples {
		r, ok := realized[s.settle]
		if !ok || s.settle.Sub(r.recv) > p.RealizedSlack {
			out.Unrealized++
			continue
		}
		resid, err := r.rate.Sub(s.estimate)
		if err != nil {
			return ImminentMeasurements{}, fmt.Errorf("residual at %s: %w", s.settle.Format(time.RFC3339), err)
		}
		out.Residuals = append(out.Residuals, resid)
	}
	return out, nil
}

// TransportStats summarizes the archive's transport-latency distribution —
// measured, never frozen directly; family 3's latency declaration cites it
// (§41b: "transport statistics with a stated multiplier").
type TransportStats struct {
	Count                uint64
	P50, P90, P99, Max   time.Duration
	NegativeObservations uint64
}

// LatencyStats computes the summary at the upper order statistic per
// quantile — ceil, the same selection every measured quantile here uses.
// Negative samples (venue clock ahead of ours — observed live) stay in the
// distribution and are counted, so the summary is honest about skew rather
// than clamping it away.
func LatencyStats(samples []time.Duration) (TransportStats, error) {
	if len(samples) == 0 {
		return TransportStats{}, fmt.Errorf("latency samples: %w", domain.ErrUnset)
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	stats := TransportStats{Count: uint64(len(sorted)), Max: sorted[len(sorted)-1]}
	for _, s := range sorted {
		if s < 0 {
			stats.NegativeObservations++
		}
	}
	pick := func(num, den int) time.Duration {
		// ceil(q·(n−1)) with q = num/den in exact integers.
		rank := num * (len(sorted) - 1)
		idx := rank / den
		if rank%den != 0 {
			idx++
		}
		return sorted[idx]
	}
	stats.P50 = pick(50, 100)
	stats.P90 = pick(90, 100)
	stats.P99 = pick(99, 100)
	return stats, nil
}

// toMarketLevels translates the venue wire levels into the domain's — the
// two types are field-identical by design, and the translation is the seam
// where a venue shape change becomes a compile error.
func toMarketLevels(levels []venue.BookLevel) []mdomain.BookLevel {
	out := make([]mdomain.BookLevel, len(levels))
	for i, l := range levels {
		out[i] = mdomain.BookLevel{Price: l.Price, Quantity: l.Quantity}
	}
	return out
}
