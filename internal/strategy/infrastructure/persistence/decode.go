package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// The two timestamp layouts research/persistence/harness.go.txt emits, and
// F102 is that there are two.
//
// Line 120–121 formats episode instants as "2006-01-02T15:04Z"; line 253–254
// formats symbol-span instants as "2006-01-02T15:04:05Z". Neither is
// time.RFC3339, and time.RFC3339 REJECTS the first:
//
//	parsing time "2025-10-27T08:00Z" as "2006-01-02T15:04:05Z07:00":
//	cannot parse "Z" as ":"
//
// Measured over the committed file: 122 span instants at 20 characters, 34 956
// episode instants at 17. Every other committed research artifact stamps
// RFC3339, and screening/report.go:178 formats with time.RFC3339 by name.
//
// This reader accepts the file's ACTUAL layout and names it. Correcting the
// emitter would require regenerating episodes.json, whose input
// research/screening/raw/ is gitignored — a re-fetch of 468 venue files. The
// defect is recorded in record §103 §7, not fixed here.
//
// The general form: a field nothing parses is not a typed field. A format
// defect survives indefinitely behind a permissive decoder, and the first
// strict reader is what dates it.
const (
	// EpisodeInstantLayout is the minute-resolution layout on every episode.
	EpisodeInstantLayout = "2006-01-02T15:04Z"

	// SpanInstantLayout is the second-resolution layout on every symbol span.
	SpanInstantLayout = "2006-01-02T15:04:05Z"
)

// wireSymbol, wireLevel and wireEpisode are episodes.json verbatim. They exist
// only inside Load; nothing outside this file sees a JSON tag.
type wireSymbol struct {
	Symbol       string      `json:"symbol"`
	Shortlist    bool        `json:"shortlist"`
	Matched      int         `json:"matched"`
	First        string      `json:"first"`
	Last         string      `json:"last"`
	ModalMinutes int         `json:"modal_minutes"`
	Levels       []wireLevel `json:"levels"`
}

type wireLevel struct {
	Entry    string        `json:"entry"`
	Episodes []wireEpisode `json:"episodes"`
}

type wireEpisode struct {
	// Run is a POINTER so that an absent "run" refuses rather than decoding to
	// 0. A missing field landing on Go's zero would be classified censored by
	// the F103 split below — a decode accident wearing a measurement's clothes,
	// and the one missing field in this file that produces a plausible answer
	// instead of an obviously empty one.
	Run     *int   `json:"run"`
	Accrued string `json:"accrued"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// Episode is one entry-to-exit run, decoded.
type Episode struct {
	// Run is the number of settlements the position was held across.
	Run int
	// Accrued is the gross funding collected over the run, as a ratio.
	Accrued fixedpoint.Rate
	// Start and End are the run's bounds, at minute resolution (see
	// EpisodeInstantLayout).
	Start time.Time
	End   time.Time
}

// symbolLevel is one symbol's episodes at one entry level, already split.
type symbolLevel struct {
	symbol   *symbolRecord
	episodes []Episode // runs: what a run-length or net question answers over
	censored []Episode // right-censored observations: NOT runs
}

type symbolRecord struct {
	name         string
	shortlist    bool
	matched      int
	first        time.Time
	last         time.Time
	modalMinutes int
}

// Set is a decoded episodes.json. Its zero value carries no data and every
// query against it refuses with ErrLevelUndeclared, which is the correct
// reading: a Set nobody loaded declares no levels.
type Set struct {
	symbols []*symbolRecord
	// byLevel is keyed by the entry level's mantissa, which is exact. Keying by
	// the file's decimal TEXT would make "0.001" and "0.0010" different levels.
	byLevel map[int64][]symbolLevel
	levels  []fixedpoint.Rate
}

// Load decodes episodes.json at path. There is no ambient default path and no
// clock read: the caller names the file.
func Load(path string) (Set, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Set{}, fmt.Errorf("persistence: read %s: %w", path, err)
	}
	var wire []wireSymbol
	if err := json.Unmarshal(data, &wire); err != nil {
		return Set{}, fmt.Errorf("persistence: decode %s: %w", path, err)
	}

	set := Set{byLevel: map[int64][]symbolLevel{}}
	seen := map[int64]fixedpoint.Rate{}

	for _, ws := range wire {
		first, err := time.Parse(SpanInstantLayout, ws.First)
		if err != nil {
			return Set{}, fmt.Errorf("persistence: %s: span first %q: %w", ws.Symbol, ws.First, err)
		}
		last, err := time.Parse(SpanInstantLayout, ws.Last)
		if err != nil {
			return Set{}, fmt.Errorf("persistence: %s: span last %q: %w", ws.Symbol, ws.Last, err)
		}
		rec := &symbolRecord{
			name: ws.Symbol, shortlist: ws.Shortlist, matched: ws.Matched,
			first: first, last: last, modalMinutes: ws.ModalMinutes,
		}
		set.symbols = append(set.symbols, rec)

		for _, wl := range ws.Levels {
			entry, err := fixedpoint.ParseRate(wl.Entry)
			if err != nil {
				return Set{}, fmt.Errorf("persistence: %s: entry %q: %w", ws.Symbol, wl.Entry, err)
			}
			key := entry.Mantissa()
			if _, ok := seen[key]; !ok {
				seen[key] = entry
				set.levels = append(set.levels, entry)
			}
			sl := symbolLevel{symbol: rec}
			for _, we := range wl.Episodes {
				ep, err := decodeEpisode(ws.Symbol, we)
				if err != nil {
					return Set{}, err
				}
				// F103: five records carry "run": 0, all at level 0.0002, all
				// with start == end, all accruing "0", all sitting at the
				// tape's final decision point. They are entries triggered with
				// no settlement after them — right-censored observations, not
				// runs of length zero — so they do not join a run-length
				// population. The count is REPORTED rather than swallowed.
				if ep.Run == 0 {
					sl.censored = append(sl.censored, ep)
					continue
				}
				sl.episodes = append(sl.episodes, ep)
			}
			set.byLevel[key] = append(set.byLevel[key], sl)
		}
	}

	sort.Slice(set.levels, func(i, j int) bool {
		return set.levels[i].Mantissa() < set.levels[j].Mantissa()
	})
	return set, nil
}

func decodeEpisode(symbol string, we wireEpisode) (Episode, error) {
	accrued, err := fixedpoint.ParseRate(we.Accrued)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: accrued %q: %w", symbol, we.Accrued, err)
	}
	start, err := time.Parse(EpisodeInstantLayout, we.Start)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: episode start %q: %w", symbol, we.Start, err)
	}
	end, err := time.Parse(EpisodeInstantLayout, we.End)
	if err != nil {
		return Episode{}, fmt.Errorf("persistence: %s: episode end %q: %w", symbol, we.End, err)
	}
	if we.Run == nil {
		return Episode{}, fmt.Errorf("persistence: %s: episode has no run field", symbol)
	}
	if *we.Run < 0 {
		return Episode{}, fmt.Errorf("persistence: %s: negative run %d", symbol, *we.Run)
	}
	return Episode{Run: *we.Run, Accrued: accrued, Start: start, End: end}, nil
}

// Levels returns the entry levels the file declares, ascending.
func (s Set) Levels() []fixedpoint.Rate {
	out := make([]fixedpoint.Rate, len(s.levels))
	copy(out, s.levels)
	return out
}

// SymbolCount returns how many symbols the file carries, before any level's
// own exclusions. Compare SymbolReturns.Excluded, which is where the
// denominator actually moves.
func (s Set) SymbolCount() int { return len(s.symbols) }

// at resolves a coordinate to the file's rows, refusing an undeclared level.
// Every query method goes through here.
func (s Set) at(a EntryAt) ([]symbolLevel, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	rows, ok := s.byLevel[a.Entry.Mantissa()]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrLevelUndeclared, a.Entry.Decimal())
	}
	return rows, nil
}
