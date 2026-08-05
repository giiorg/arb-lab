// Package baseline is the spot exposure baseline's document: the stored form
// of `arblab baseline record`'s attestation and the verifying loader the boot
// runs before a baseline governs anything (task #179, record §70b).
//
// # The document stores INPUTS, not conclusions
//
// This repo's stored-form rule, tightened by the design review: a stored
// scalar `ours` — or a stored baseline — would be a conclusion, and
// schema-valid arbitrary bytes with a true chain mark would have loaded. What
// is stored instead is the venue's balance words VERBATIM and the per-order
// rows `ours` is a sum of, and the loader RECOMPUTES everything: the balance
// through the same per-venue derivation the live read uses, `ours` through
// §70c's own formula, and `baseline = B − ours` last. What remains attested
// rather than recomputable is only each row's venue cumulative (fills are not
// journaled); a late fill admitted after record therefore shows up as a
// diagnosable per-order disagreement, not an opaque delta.
//
// # Why this package lives under oms
//
// The loader verifies the chain mark against the journal and cross-checks the
// stored rows against the replayed prefix, which takes the journal adapter —
// and §3.6 rule 6 forbids every package outside internal/oms/ from importing
// the write path. Composition roots import THIS package and hand the
// application layer already-verified baseline data.
package baseline

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
)

// Kind and Version identify the document shape. A file that does not carry
// them is not a baseline document, whatever else it parses as.
const (
	Kind    = "arblab-spot-baseline"
	Version = 1
)

// Document is one baseline attestation: everything `arblab baseline record`
// established, in the venue's own words, bound four ways.
type Document struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	// Venue and Account bind the document to the deployment: a baseline is a
	// claim about ONE subaccount's wallet at one exchange.
	Venue   string `json:"venue"`
	Account string `json:"account"`

	// CredentialSHA256 is the spending credential's fingerprint at record time
	// — the same digest the spend lock computes. The account LABEL is mutable
	// configuration, so a credential swapped to another subaccount under an
	// unchanged label would pass every other binding; this one refuses it, and
	// key rotation therefore forces a re-record, deliberately.
	CredentialSHA256 string `json:"credential_sha256"`

	// ObservedAt is the venue clock instant of the record run's balance read,
	// RFC3339. It is the operator's diagnosis anchor, never a staleness gate:
	// a baseline is invalidated by events, not by time (§70b).
	ObservedAt string `json:"observed_at"`

	// ChainSeq and ChainHash are the journal's head at record time — the mark
	// that pins WHICH history `ours` attested against. A journal restored from
	// backup, reset, or truncated below the mark fails the verification and
	// the baseline is refused by name. Seq 0 is a record over an EMPTY journal:
	// every journal extends the empty one, `ours` was zero, nothing was
	// attested — the verification is vacuous and ChainHash stays empty.
	ChainSeq  uint64 `json:"chain_seq"`
	ChainHash string `json:"chain_hash,omitempty"`

	// Balances is one row per baselined asset: the venue's balance fields
	// verbatim, or the venue's silence stated out loud.
	Balances []BalanceRow `json:"balances"`

	// Orders is one row per spot order the record run counted — the rows
	// `ours` is a sum of, each cross-checked against the chain-marked journal
	// prefix at load.
	Orders []OrderRow `json:"orders"`
}

// BalanceRow is one asset's balance evidence at record time.
//
// Exactly one of Words and Absent carries the answer: Words is the venue's
// fields verbatim, and Absent is the enumeration's silence — on Bybit an
// absent coin IS a zero balance (the venue's documented rule), and storing
// that reading explicitly is what keeps "the venue said nothing" from being
// indistinguishable from "nobody asked".
type BalanceRow struct {
	Asset  string `json:"asset"`
	Absent bool   `json:"absent,omitempty"`
	Words  []Word `json:"words,omitempty"`
}

// Word is one venue-published balance field, exactly as the wire carried it.
type Word struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// OrderRow is one journaled spot order as the record run counted it.
type OrderRow struct {
	ClientOrderID string `json:"client_order_id"`
	Side          string `json:"side"`

	// Base is the asset the cumulative is denominated in — carried because the
	// recomputation groups rows by it and the config that once named it may
	// have changed by load time.
	Base string `json:"base"`

	// Cumulative is the venue's executed total for this order, the one field
	// that stays attested rather than recomputable.
	Cumulative string `json:"cumulative"`

	// Fees are the per-asset fee totals the projection retained for this
	// order, in the venue's asset spelling. Fees in non-baselined assets ride
	// along — they are inputs the record run held, and the recomputation
	// consumes exactly the ones §70c's formula names.
	Fees []Fee `json:"fees,omitempty"`
}

// Fee is one asset's fee total on one order.
type Fee struct {
	Asset  string `json:"asset"`
	Amount string `json:"amount"`
}

// Validate reports whether the document is shaped like an attestation —
// bindings present, every row readable, nothing stated twice, no half-stated
// mark. It checks SHAPE only: the bindings' truth is Load's job, against the
// journal and the deployment.
func (d Document) Validate() error {
	if d.Kind != Kind {
		return fmt.Errorf("kind %q, want %q: %w", d.Kind, Kind, ErrDocumentMalformed)
	}
	if d.Version != Version {
		return fmt.Errorf("version %d, want %d: %w", d.Version, Version, ErrDocumentMalformed)
	}
	if d.Venue == "" {
		return fmt.Errorf("venue unset: %w", ErrDocumentMalformed)
	}
	if d.Account == "" {
		return fmt.Errorf("account unset: %w", ErrDocumentMalformed)
	}
	if d.CredentialSHA256 == "" {
		return fmt.Errorf("credential fingerprint unset: %w", ErrDocumentMalformed)
	}
	if _, err := time.Parse(time.RFC3339, d.ObservedAt); err != nil {
		return fmt.Errorf("observed_at %q: %w: %w", d.ObservedAt, err, ErrDocumentMalformed)
	}
	if err := d.validateChainMark(); err != nil {
		return err
	}
	if err := d.validateBalances(); err != nil {
		return err
	}
	return d.validateOrders()
}

func (d Document) validateChainMark() error {
	if d.ChainSeq == 0 {
		if d.ChainHash != "" {
			// A hash with no sequence is half a mark: it names no record, so
			// nothing could ever verify it, and a loader skipping "by the seq"
			// would skip a mark somebody thought they wrote.
			return fmt.Errorf("chain hash without a chain seq: %w", ErrDocumentMalformed)
		}
		return nil
	}
	raw, err := hex.DecodeString(d.ChainHash)
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("chain hash %q is not a 32-byte hex digest: %w", d.ChainHash, ErrDocumentMalformed)
	}
	return nil
}

func (d Document) validateBalances() error {
	if len(d.Balances) == 0 {
		// A document with no balance rows pins nothing: it is a claim nobody
		// consumes, and loading it would report success about an empty set.
		return fmt.Errorf("no balance rows: %w", ErrDocumentMalformed)
	}
	seen := make(map[string]struct{}, len(d.Balances))
	for i, row := range d.Balances {
		if row.Asset == "" {
			return fmt.Errorf("balance row %d carries no asset: %w", i, ErrDocumentMalformed)
		}
		if _, dup := seen[row.Asset]; dup {
			return fmt.Errorf("balance row %d states %s twice: %w", i, row.Asset, ErrDocumentMalformed)
		}
		seen[row.Asset] = struct{}{}
		if row.Absent && len(row.Words) > 0 {
			return fmt.Errorf("balance row %s is absent AND carries words: %w", row.Asset, ErrDocumentMalformed)
		}
		if !row.Absent && len(row.Words) == 0 {
			return fmt.Errorf("balance row %s carries neither words nor the absence: %w", row.Asset, ErrDocumentMalformed)
		}
	}
	return nil
}

func (d Document) validateOrders() error {
	seen := make(map[string]struct{}, len(d.Orders))
	for i, row := range d.Orders {
		if row.ClientOrderID == "" {
			return fmt.Errorf("order row %d carries no client order id: %w", i, ErrDocumentMalformed)
		}
		if _, dup := seen[row.ClientOrderID]; dup {
			// The same order twice would count its cumulative twice in `ours`.
			return fmt.Errorf("order row %d states %q twice: %w", i, row.ClientOrderID, ErrDocumentMalformed)
		}
		seen[row.ClientOrderID] = struct{}{}
		if err := domain.Side(row.Side).Validate(); err != nil {
			return fmt.Errorf("order row %q side: %w: %w", row.ClientOrderID, err, ErrDocumentMalformed)
		}
		if row.Base == "" {
			return fmt.Errorf("order row %q carries no base asset: %w", row.ClientOrderID, ErrDocumentMalformed)
		}
		cumulative, err := fixedpoint.ParseQuantity(fixedpoint.Asset(row.Base), row.Cumulative)
		if err != nil {
			return fmt.Errorf("order row %q cumulative %q: %w: %w", row.ClientOrderID, row.Cumulative, err, ErrDocumentMalformed)
		}
		sign, err := cumulative.Sign()
		if err != nil {
			return fmt.Errorf("order row %q cumulative: %w: %w", row.ClientOrderID, err, ErrDocumentMalformed)
		}
		if sign < 0 {
			return fmt.Errorf("order row %q cumulative %q is negative: %w", row.ClientOrderID, row.Cumulative, ErrDocumentMalformed)
		}
		feeAssets := make(map[string]struct{}, len(row.Fees))
		for _, fee := range row.Fees {
			if fee.Asset == "" {
				return fmt.Errorf("order row %q fee carries no asset: %w", row.ClientOrderID, ErrDocumentMalformed)
			}
			if _, dup := feeAssets[fee.Asset]; dup {
				// One entry per asset actually charged (ports.OrderRecord's
				// contract); a second would double-subtract.
				return fmt.Errorf("order row %q states a %s fee twice: %w", row.ClientOrderID, fee.Asset, ErrDocumentMalformed)
			}
			feeAssets[fee.Asset] = struct{}{}
			if _, err := fixedpoint.ParseQuantity(fixedpoint.Asset(fee.Asset), fee.Amount); err != nil {
				return fmt.Errorf("order row %q fee %s %q: %w: %w", row.ClientOrderID, fee.Asset, fee.Amount, err, ErrDocumentMalformed)
			}
		}
	}
	return nil
}

// Write persists the document at path under O_EXCL — a NEW dated file every
// time, #183's shape: no overwrite path exists, the old document stays on disk
// as history, and replacing the effective baseline takes the second act (the
// config digest edit) by construction.
func Write(path string, doc Document) (err error) {
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("baseline document: %w", err)
	}
	body, err := Encode(doc)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("baseline document: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			// A partial file would be a document that exists and cannot load —
			// worse than none, because O_EXCL then refuses the retry.
			_ = f.Close()
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(body); err != nil {
		return fmt.Errorf("baseline document: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("baseline document: %w", err)
	}
	complete = true
	return nil
}
