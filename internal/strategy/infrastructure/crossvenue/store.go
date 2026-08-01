package crossvenue

// The settled-funding REST-history store (§45a): the gate's accrual truth
// is the venues' settled prints, fetched over REST as the evidence window
// accumulates and stored in the screening's raw-file + digest-ledger
// pattern — verbatim response pages under a git-ignored root, digests as
// the auditable identity. The screening package is imported rather than
// copied: its fetchers and raw-file schema are generic over (symbol,
// cutoff), and a second transcription of a venue's paging dialect would be
// a drift surface, not a boundary.
//
// Unlike the screening's one-shot store (one file per identity), this
// store accumulates: each fetch lands in its own file named by the fetch's
// cutoff day, and assembly MERGES the files — with any disagreement
// between two files about one instant's rate refused as a different
// dataset, never averaged or last-write-wins.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	dcv "github.com/giiorg/arb-lab/internal/strategy/domain/crossvenue"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/screening"
)

// Store sentinels.
var (
	// ErrStoreConflict reports two stored fetches disagreeing about one
	// settlement instant's rate. A re-fetch that contradicts stored bytes
	// is a different dataset, visibly — the digest-ledger rule; keeping
	// either row would silently choose which history is real.
	ErrStoreConflict = fmt.Errorf("strategy/crossvenue: stored settled-funding fetches disagree")

	// ErrStoreEmpty reports a symbol with no stored settled-funding fetch
	// for a venue. An empty store must not read as a market that never
	// settled (the vacuous-run shape).
	ErrStoreEmpty = fmt.Errorf("strategy/crossvenue: no stored settled-funding history")

	// ErrStoreVenueUnknown reports a venue this store has not been taught.
	ErrStoreVenueUnknown = fmt.Errorf("strategy/crossvenue: unknown venue for the settled store")

	// ErrStoreExists reports an identity already fetched: the resume path
	// is to SKIP it, and the check runs BEFORE the venue walk so a resume
	// costs no requests (the screening harness's discipline).
	ErrStoreExists = fmt.Errorf("strategy/crossvenue: settled-funding identity already stored")
)

// FetchSettled fetches one venue's settled-funding history for one symbol
// up to cutoff and stores it verbatim. The file is named by the cutoff's
// UTC day, and WriteRaw's refusal to overwrite is inherited deliberately:
// re-fetching one identity twice is a SKIP on the resume path, not a
// silent replacement of bytes a ledger may already cite.
func FetchSettled(t screening.Transport, root, venueName, symbol string, cutoff, fetchedAt time.Time, pageLimit int) (path, digest string, err error) {
	path = settledPath(root, venueName, symbol, cutoff)
	if _, statErr := os.Stat(path); statErr == nil {
		return "", "", fmt.Errorf("%s: %w", path, ErrStoreExists)
	}
	var raw screening.RawFile
	switch venueName {
	case "binance":
		raw, err = screening.FetchBinanceFunding(t, symbol, cutoff, fetchedAt, pageLimit)
	case "bybit":
		raw, err = screening.FetchBybitFunding(t, symbol, cutoff, fetchedAt, pageLimit)
	default:
		return "", "", fmt.Errorf("venue %q: %w", venueName, ErrStoreVenueUnknown)
	}
	if err != nil {
		return "", "", fmt.Errorf("settled fetch %s %s: %w", venueName, symbol, err)
	}
	digest, err = screening.WriteRaw(path, raw)
	if err != nil {
		return "", "", fmt.Errorf("settled store %s %s: %w", venueName, symbol, err)
	}
	return path, digest, nil
}

// settledPath is the store layout: one file per (venue, symbol, cutoff
// day), so repeated fetches over the evidence window accumulate beside
// each other instead of colliding with the overwrite refusal.
func settledPath(root, venueName, symbol string, cutoff time.Time) string {
	name := fmt.Sprintf("funding.%s.%s.json", symbol, cutoff.UTC().Format("2006-01-02"))
	return filepath.Join(root, venueName, name)
}

// StoredFile is one raw fetch's provenance, surfaced for the run report
// and slice D's dataset ledger.
type StoredFile struct {
	Path   string
	SHA256 string
	Rows   int
}

// LoadSettled assembles one venue's settled series for a symbol from every
// stored fetch: rows merge by exact venue-published instant, an identical
// (instant, rate) in two files deduplicates, and a disagreement refuses
// (ErrStoreConflict). The result is ascending — the shape Pair and
// WindowAccrual require.
func LoadSettled(root, venueName, symbol string) ([]dcv.Settlement, []StoredFile, error) {
	pattern := filepath.Join(root, venueName, "funding."+symbol+".*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("settled glob: %w", err)
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("%s %s under %s: %w", venueName, symbol, root, ErrStoreEmpty)
	}
	sort.Strings(paths)

	byInstant := map[int64]dcv.Settlement{}
	var files []StoredFile
	for _, p := range paths {
		raw, digest, err := screening.LoadRaw(p)
		if err != nil {
			return nil, nil, fmt.Errorf("settled load: %w", err)
		}
		rows, err := screening.RowsFromRaw(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("settled rows %s: %w", p, err)
		}
		for _, r := range rows {
			ms := r.At.UnixMilli()
			if have, ok := byInstant[ms]; ok {
				c, err := have.Rate.Cmp(r.Rate)
				if err != nil {
					return nil, nil, fmt.Errorf("settled merge %s at %s: %w", p, r.At.Format(time.RFC3339Nano), err)
				}
				if c != 0 {
					return nil, nil, fmt.Errorf("%s at %s: %s vs %s: %w",
						p, r.At.Format(time.RFC3339Nano), r.Rate.Decimal(), have.Rate.Decimal(), ErrStoreConflict)
				}
				continue
			}
			byInstant[ms] = dcv.Settlement{At: r.At, Rate: r.Rate}
		}
		files = append(files, StoredFile{Path: p, SHA256: digest, Rows: len(rows)})
	}

	out := make([]dcv.Settlement, 0, len(byInstant))
	for _, s := range byInstant {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, files, nil
}
