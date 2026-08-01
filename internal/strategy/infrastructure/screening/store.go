package screening

// The raw store: every venue response the screening consumed, verbatim,
// one file per (venue, kind, symbol), digested into the committed
// ledger. Raw files live under a git-ignored root (they are hundreds of
// megabytes of venue words); the ledger's digests are what make the run
// auditable — a re-fetch that disagrees with a ledger digest is a
// different dataset, visibly.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RawFile is one fetched identity: the venue's response pages, verbatim,
// plus the provenance that names what was asked.
type RawFile struct {
	Venue     string            `json:"venue"`
	Kind      string            `json:"kind"`
	Symbol    string            `json:"symbol,omitempty"`
	Endpoint  string            `json:"endpoint"`
	FetchedAt string            `json:"fetched_at"`
	CutoffUTC string            `json:"cutoff_utc"`
	Pages     []json.RawMessage `json:"pages"`
}

// Raw file kinds. Strings rather than an enum-like: these are report
// vocabulary, never switched over for behaviour.
const (
	KindFunding      = "funding"
	KindExchangeInfo = "exchange_info"
	KindFundingInfo  = "funding_info"
	KindInstruments  = "instruments"
	KindTickers      = "tickers"
)

// RawPath is the store layout: <root>/<venue>/<kind>.<symbol>.json for
// per-symbol files, <root>/<venue>/<kind>.json for venue-wide ones.
func RawPath(root string, venue, kind, symbol string) string {
	name := kind + ".json"
	if symbol != "" {
		name = kind + "." + symbol + ".json"
	}
	return filepath.Join(root, venue, name)
}

// WriteRaw stores a raw file and returns its digest. It refuses to
// overwrite: an existing path means this identity was already fetched,
// and the resume path is to SKIP it, not to silently replace bytes a
// ledger may already cite.
func WriteRaw(path string, f RawFile) (digest string, err error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return "", fmt.Errorf("raw file exists: %s", path)
	}
	data, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("raw encode: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("raw dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("raw write: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LoadRaw reads a raw file back and returns it with the digest of the
// bytes on disk.
func LoadRaw(path string) (RawFile, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RawFile{}, "", fmt.Errorf("raw read: %w", err)
	}
	var f RawFile
	if err := json.Unmarshal(data, &f); err != nil {
		return RawFile{}, "", fmt.Errorf("raw decode %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return f, hex.EncodeToString(sum[:]), nil
}
