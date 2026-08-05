package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// Dialect selects the per-venue balance derivation — the ONE shared meaning
// the live adapter also uses (record §70d). ids.Venue is an open key from
// configuration, so the venue NAME cannot pick a decoder; the dialect is the
// same discriminator the composition root already holds for the signer.
type Dialect string

const (
	DialectBinance Dialect = "binance"
	DialectBybit   Dialect = "bybit"
)

// Validate reports whether the dialect names a derivation this package holds.
func (d Dialect) Validate() error {
	switch d {
	case DialectBinance, DialectBybit:
		return nil
	}
	return fmt.Errorf("dialect %q: %w", string(d), ErrDialectUnknown)
}

// Pin is §5.2's `spot_baseline` config field: the document's path AND its
// digest. O_EXCL guards creation only — a path-pinned file edited in place
// would change the effective baseline with no second act. With the digest in
// config, replacing the document WITHOUT the config edit is a named refusal,
// so the two-act rule is enforced by construction rather than by convention
// (record §70b, review round 2).
type Pin struct {
	Path   string
	SHA256 string
}

// Bindings is what the deployment knows that the document must agree with.
type Bindings struct {
	// Venue and Account are the deployment config's.
	Venue   ids.Venue
	Account string

	// Fingerprint is the constructed transport's credential fingerprint — the
	// digest the spend lock computes. Comparing it here is what makes a
	// credential swapped under an unchanged account label a refusal.
	Fingerprint string

	// Dialect selects the balance derivation.
	Dialect Dialect
}

// Journal is the read capability the loader needs: the chain-mark lookup and
// the verified walk the rows are cross-checked against. *driven.SQLiteJournal
// satisfies it; a test may hand a fake.
type Journal interface {
	HashAt(seq uint64) ([32]byte, error)
	Replay(ports.JournalVisitor) error
}

// Load reads, verifies and RECOMPUTES the pinned baseline document.
//
// The ladder, each rung its own named refusal, in the order the claims depend
// on each other (record §70b):
//
//  1. digest — the bytes are the ones the config pins (the two-act rule);
//  2. shape — the bytes are a document;
//  3. chain mark — the journal is the history `ours` attested against
//     (seq 0 verifies vacuously; a truncated journal is refused DISTINCTLY);
//  4. credential fingerprint — the account is the one the credential reaches,
//     not merely the one the label names;
//  5. venue and account label — the deployment is the one described;
//  6. rows-vs-prefix — every stored order id exists in the chain-marked
//     journal prefix, so the rows are bound to the same history the mark
//     names.
//
// Only then is anything computed: B from the verbatim words via the shared
// per-venue derivation, ours(S) by §70c's formula over the stored rows, and
// baseline = B − ours last.
func Load(pin Pin, journal Journal, bind Bindings) ([]application.SpotBaseline, error) {
	if pin.Path == "" || pin.SHA256 == "" {
		return nil, fmt.Errorf("baseline pin path=%q sha256=%q: %w", pin.Path, pin.SHA256, ErrPinIncomplete)
	}
	if journal == nil {
		return nil, ErrJournalMissing
	}
	if err := bind.Dialect.Validate(); err != nil {
		return nil, err
	}

	body, err := os.ReadFile(pin.Path)
	if err != nil {
		return nil, fmt.Errorf("baseline document: %w", err)
	}

	// 1. Digest. Before parsing: a mismatch means these are not the pinned
	// bytes, and diagnosing their content would be describing a document the
	// operator never pinned.
	digest := sha256.Sum256(body)
	if got := hex.EncodeToString(digest[:]); !strings.EqualFold(got, pin.SHA256) {
		return nil, fmt.Errorf("document at %s digests %s, config pins %s: %w",
			pin.Path, got, strings.ToLower(pin.SHA256), ErrDigestMismatch)
	}

	// 2. Shape.
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("baseline document: %w: %w", err, ErrDocumentMalformed)
	}
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("baseline document: %w", err)
	}

	// 3. Chain mark. Skipped by the seq, not by a flag beside it: a baseline
	// recorded over an empty journal attested nothing.
	if doc.ChainSeq > 0 {
		want, err := hex.DecodeString(doc.ChainHash)
		if err != nil {
			return nil, fmt.Errorf("baseline document chain hash: %w: %w", err, ErrDocumentMalformed)
		}
		stored, err := journal.HashAt(doc.ChainSeq)
		if err != nil {
			// ErrJournalMarkBeyondHead passes through NAMED: the journal LOST
			// records relative to whoever recorded the mark, which is a
			// different operator conversation than a hash mismatch.
			return nil, fmt.Errorf("baseline chain mark at seq %d: %w", doc.ChainSeq, err)
		}
		if stored != [32]byte(want) {
			return nil, fmt.Errorf("journal hash at seq %d is not the recorded mark: %w",
				doc.ChainSeq, ErrChainMarkMismatch)
		}
	}

	// 4. Credential fingerprint.
	if bind.Fingerprint == "" {
		return nil, ErrFingerprintUnset
	}
	if doc.CredentialSHA256 != bind.Fingerprint {
		return nil, fmt.Errorf("document recorded under credential %s, transport holds %s: %w",
			doc.CredentialSHA256, bind.Fingerprint, ErrCredentialMismatch)
	}

	// 5. Venue and account label.
	if err := bind.Venue.Validate(); err != nil {
		return nil, fmt.Errorf("baseline bindings venue: %w", err)
	}
	if doc.Venue != string(bind.Venue) {
		return nil, fmt.Errorf("document describes venue %q, deployment is %q: %w",
			doc.Venue, bind.Venue, ErrVenueMismatch)
	}
	if bind.Account == "" {
		return nil, fmt.Errorf("baseline bindings account: %w", ErrAccountMismatch)
	}
	if doc.Account != bind.Account {
		return nil, fmt.Errorf("document describes account %q, deployment is %q: %w",
			doc.Account, bind.Account, ErrAccountMismatch)
	}

	// 6. Rows against the chain-marked prefix.
	prefix := prefixOrders{mark: doc.ChainSeq, ids: make(map[string]struct{})}
	if err := journal.Replay(&prefix); err != nil {
		return nil, fmt.Errorf("baseline prefix replay: %w", err)
	}
	for _, row := range doc.Orders {
		if _, held := prefix.ids[row.ClientOrderID]; !held {
			return nil, fmt.Errorf("order row %q is not in the journal prefix at seq %d: %w",
				row.ClientOrderID, doc.ChainSeq, ErrOrderOutsidePrefix)
		}
	}

	// Recompute. B first, per stored asset.
	totals := make(map[string]fixedpoint.Quantity, len(doc.Balances))
	assets := make([]string, 0, len(doc.Balances))
	for _, row := range doc.Balances {
		total, err := storedTotal(bind.Dialect, row)
		if err != nil {
			return nil, err
		}
		totals[row.Asset] = total
		assets = append(assets, row.Asset)
	}
	sort.Strings(assets)

	// ours(S) = Σ signed cumulative over rows with base S
	//         − Σ fees with fee asset S over ALL rows (§70c).
	out := make([]application.SpotBaseline, 0, len(assets))
	for _, asset := range assets {
		tag := fixedpoint.Asset(asset)
		ours := fixedpoint.ZeroQuantity(tag)
		for _, row := range doc.Orders {
			if row.Base == asset {
				cumulative, err := fixedpoint.ParseQuantity(tag, row.Cumulative)
				if err != nil {
					return nil, fmt.Errorf("order row %q cumulative: %w", row.ClientOrderID, err)
				}
				signed, err := signedCumulative(domain.Side(row.Side), cumulative)
				if err != nil {
					return nil, fmt.Errorf("order row %q: %w", row.ClientOrderID, err)
				}
				ours, err = ours.Add(signed)
				if err != nil {
					return nil, fmt.Errorf("order row %q: %w", row.ClientOrderID, err)
				}
			}
			for _, fee := range row.Fees {
				if fee.Asset != asset {
					continue
				}
				q, err := fixedpoint.ParseQuantity(tag, fee.Amount)
				if err != nil {
					return nil, fmt.Errorf("order row %q fee %s: %w", row.ClientOrderID, fee.Asset, err)
				}
				ours, err = ours.Sub(q)
				if err != nil {
					return nil, fmt.Errorf("order row %q fee %s: %w", row.ClientOrderID, fee.Asset, err)
				}
			}
		}
		amount, err := totals[asset].Sub(ours)
		if err != nil {
			return nil, fmt.Errorf("baseline %s: %w", asset, err)
		}
		out = append(out, application.SpotBaseline{
			Venue:  bind.Venue,
			Asset:  tag,
			Amount: amount,
		})
	}
	return out, nil
}

// storedTotal re-derives one balance row's total through the SAME per-venue
// function the live adapter uses — one meaning in one place (§70d).
func storedTotal(d Dialect, row BalanceRow) (fixedpoint.Quantity, error) {
	tag := fixedpoint.Asset(row.Asset)
	if row.Absent {
		// The enumeration's silence, stored out loud: an absent coin is a zero
		// balance (the Bybit rule; on Binance every asset comes back, so an
		// absent row there records the same conclusion from a narrower cause).
		return fixedpoint.ZeroQuantity(tag), nil
	}
	words := make([]venue.SpotBalanceWord, 0, len(row.Words))
	for _, w := range row.Words {
		words = append(words, venue.SpotBalanceWord{Name: w.Name, Value: w.Value})
	}
	switch d {
	case DialectBinance:
		total, err := venue.BinanceSpotTotal(tag, words)
		if err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("balance row %s: %w", row.Asset, err)
		}
		return total, nil
	case DialectBybit:
		total, unpinned, err := venue.BybitSpotTotal(tag, words)
		if err != nil {
			return fixedpoint.Quantity{}, fmt.Errorf("balance row %s: %w", row.Asset, err)
		}
		if unpinned != venue.SpotUnpinnedReasonUnspecified {
			// The record command refuses to mint a baseline from an unpinned
			// row, so a stored row the derivation cannot pin was never written
			// by it — this is a hand-made document, refused by name.
			return fixedpoint.Quantity{}, fmt.Errorf("balance row %s words are unpinnable (%s): %w",
				row.Asset, unpinned, ErrStoredRowUnpinnable)
		}
		return total, nil
	}
	return fixedpoint.Quantity{}, fmt.Errorf("balance row %s: %w", row.Asset, ErrDialectUnknown)
}

// signedCumulative applies the order's side: a buy acquired the base, a sell
// disposed of it.
func signedCumulative(side domain.Side, cumulative fixedpoint.Quantity) (fixedpoint.Quantity, error) {
	switch side {
	case domain.SideBuy:
		return cumulative, nil
	case domain.SideSell:
		return cumulative.Neg()
	}
	return fixedpoint.Quantity{}, fmt.Errorf("side %q: %w", side, ErrDocumentMalformed)
}

// prefixOrders collects the client order ids journaled at or below the mark.
// The walk rides the verifying Replay, so the chain is checked on the way.
//
// Only PLANNED records mint an id — the other kinds reference one — and a
// mark of 0 collects nothing, which is what makes an order row on an
// empty-journal document refuse.
type prefixOrders struct {
	mark uint64
	ids  map[string]struct{}
}

func (p *prefixOrders) Planned(seq uint64, o domain.Order) error {
	if seq <= p.mark {
		p.ids[string(o.ClientOrderID)] = struct{}{}
	}
	return nil
}

func (p *prefixOrders) Authorized(uint64, ids.ClientOrderID, clock.ServerTime) error { return nil }
func (p *prefixOrders) Acked(uint64, ids.ClientOrderID, string) error                { return nil }
func (p *prefixOrders) State(uint64, ids.ClientOrderID, domain.OrderState) error     { return nil }

// Encode renders the document exactly as Write persists it. Exported because
// the record command must print the digest of the BYTES ON DISK for the
// operator's config pin, and a second marshalling could disagree with the
// first about them.
func Encode(doc Document) ([]byte, error) {
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("baseline document: %w", err)
	}
	return append(body, '\n'), nil
}
