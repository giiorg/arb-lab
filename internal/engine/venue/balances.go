package venue

import (
	"context"
	"fmt"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/apperror"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// This file holds the spot balance ENUMERATION's vocabulary (task #179,
// record §70): what a venue's spot wallet read answers with, before the OMS
// adapter translates it into the context's port. The per-venue codecs and the
// client method live beside the other read surfaces in this package.
//
// The enumeration is UNSCOPED by asset, per venue rule (Profiles §4's dated
// 2026-08-05 block): Binance returns every asset with omitZeroBalances left
// false; Bybit's coin-less call returns every non-zero asset, so an ABSENT
// coin is a zero balance — the parameter itself decides what silence means,
// which is why no caller may scope the call.

// SpotBalanceWord is one venue-published balance field, exactly as the wire
// carried it — the input the typed total is derived from, preserved so the
// baseline document can store the venue's word and the loader can re-derive
// the total through the SAME per-venue rule the live read used.
type SpotBalanceWord struct {
	Name  string
	Value string
}

// SpotBalanceRow is one asset's spot holding with a pinnable total.
type SpotBalanceRow struct {
	// Asset is the coin, tagged in the venue's own spelling.
	Asset fixedpoint.Asset

	// Total is the whole holding, derived from Words by the venue's rule:
	// free + locked on Binance (both documented partitions of holdings),
	// walletBalance on a Bybit row whose locked is zero.
	Total fixedpoint.Quantity

	// Words are the venue's balance fields verbatim. Never empty on a
	// well-formed row.
	Words []SpotBalanceWord
}

// SpotUnpinnedReason is why a row's total could not be pinned this read.
type SpotUnpinnedReason uint8

const (
	// SpotUnpinnedReasonUnspecified is the zero value and names no reason.
	SpotUnpinnedReasonUnspecified SpotUnpinnedReason = iota

	// SpotUnpinnedLockedSplit is Bybit's unverified split: no doc sentence
	// states whether walletBalance includes locked, so a row with locked != 0
	// has two candidate totals differing by exactly locked. Where locked is
	// zero the readings coincide and the row is pinned instead.
	SpotUnpinnedLockedSplit

	// SpotUnpinnedBorrow is a row with borrowAmount or spotBorrow non-zero:
	// borrowed coins inflate walletBalance without being holdings.
	SpotUnpinnedBorrow
)

const maxSpotUnpinnedReason = SpotUnpinnedBorrow

// Validate reports whether r names a reason, excluding the sentinel.
func (r SpotUnpinnedReason) Validate() error {
	if r == SpotUnpinnedReasonUnspecified || r > maxSpotUnpinnedReason {
		return fmt.Errorf("spot unpinned reason %d: %w", uint8(r), ErrSpotUnpinnedReasonInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (r SpotUnpinnedReason) String() string {
	switch r {
	case SpotUnpinnedReasonUnspecified:
		return "SpotUnpinnedReasonUnspecified"
	case SpotUnpinnedLockedSplit:
		return "SpotUnpinnedLockedSplit"
	case SpotUnpinnedBorrow:
		return "SpotUnpinnedBorrow"
	}
	return fmt.Sprintf("SpotUnpinnedReason(%d)", uint8(r))
}

// SpotUnpinnedRow is one asset whose total this read could not pin.
type SpotUnpinnedRow struct {
	// Asset is the coin, in the venue's own spelling.
	Asset string

	// Reason is why.
	Reason SpotUnpinnedReason
}

// SpotBalanceReport is one wallet enumeration's whole answer.
type SpotBalanceReport struct {
	// ObservedAt is the venue clock instant of the read.
	ObservedAt clock.ServerTime

	// Rows are the assets with a pinnable total. Absence from Rows AND
	// Unpinned is a zero balance (see the package comment above).
	Rows []SpotBalanceRow

	// Unpinned are the assets whose total could not be pinned, each with the
	// reason.
	Unpinned []SpotUnpinnedRow
}

// SpotBalanceCodec is one venue's translation of the spot balance surface.
//
// Embedded into OrderCodec on PositionCodec's argument, one leg over: §7.5's
// spot assertion compares the wallet against a recorded baseline before
// trading resumes, so a venue whose spot balances cannot be read is not a
// venue this system may hold a spot leg on — and cash-and-carry's spot half is
// the strategy, not an option.
//
// There is no scope parameter anywhere on this surface: the venue is the
// codec's, the account is the credential's, and the asset dimension is
// deliberately unscoped (see the file comment) — a parameter would be a knob
// whose only settings are "the whole answer" and "the call whose silence means
// nothing".
type SpotBalanceCodec interface {
	// EncodeSpotBalances renders the wallet enumeration as one signed call.
	EncodeSpotBalances() (transport.Call, error)

	// DecodeSpotBalances reads the venue's answer.
	//
	// It does NOT stamp ObservedAt — a codec is pure, and the instant belongs
	// to the client that owns the venue-calibrated clock.
	DecodeSpotBalances(resp transport.Response) (SpotBalanceReport, error)
}

// SpotBalances enumerates the account's spot wallet.
//
// # The urgency is the caller's, and the two rungs are the whole point
//
// §7.5's boot assertion is Safety traffic; `arblab baseline record` is an
// operator maintenance read that must be shed before the emergency reserve
// rather than out of it — Routine. The OMS adapter pins the choice at
// construction per call site (record §70d); this method just refuses to guess.
//
// # ObservedAt is stamped HERE, from the transport's venue-calibrated clock
//
// The instant is read after the response arrives, from the same clock the
// request was signed with, so "the venue clock instant of the read" is one
// authority rather than a codec inventing a second one from a response field
// whose semantics differ per venue (Binance's updateTime is the account's last
// change, not this read).
func (c *OrderClient) SpotBalances(ctx context.Context, urgency ratelimit.Priority) (SpotBalanceReport, error) {
	op := string(c.codec.Venue()) + ".SpotBalances"

	if err := urgency.Validate(); err != nil {
		return SpotBalanceReport{}, apperror.Precondition(op, "urgency", err)
	}
	client, ok := c.transports[ids.ProductSpot]
	if !ok {
		// The wallet the baseline describes is the spot account's, and on
		// Binance that is a different HOST (api. vs fapi.) — a deployment with
		// no spot route has no wallet this surface could honestly answer for.
		return SpotBalanceReport{}, apperror.Precondition(op, "route",
			fmt.Errorf("spot family: %w", ErrOrderFamilyUnrouted))
	}

	call, err := c.codec.EncodeSpotBalances()
	if err != nil {
		return SpotBalanceReport{}, apperror.Precondition(op, "encode", err)
	}
	call.Priority = urgency

	resp, err := client.Do(ctx, call)
	if err != nil {
		// Unwrapped, on the read path's standing rule: the transport already
		// classified the effect, and an enumeration that was not answered must
		// not be re-derived into one that came back empty.
		return SpotBalanceReport{}, err
	}

	report, err := c.codec.DecodeSpotBalances(resp)
	if err != nil {
		return SpotBalanceReport{}, apperror.Precondition(op, "decode", err)
	}
	for _, row := range report.Rows {
		if err := row.Validate(); err != nil {
			return SpotBalanceReport{}, apperror.Precondition(op, "decode", err)
		}
	}
	for _, row := range report.Unpinned {
		if row.Asset == "" {
			return SpotBalanceReport{}, apperror.Precondition(op, "decode",
				fmt.Errorf("unpinned row carries no asset: %w", ErrSpotBalancesUnreadable))
		}
		if err := row.Reason.Validate(); err != nil {
			return SpotBalanceReport{}, apperror.Precondition(op, "decode",
				fmt.Errorf("unpinned %s: %w", row.Asset, err))
		}
	}

	report.ObservedAt = client.ServerNow()
	return report, nil
}

// Validate reports whether the row is one a caller may reason from.
func (r SpotBalanceRow) Validate() error {
	if r.Asset == "" {
		return fmt.Errorf("spot balance row carries no asset: %w", ErrSpotBalancesUnreadable)
	}
	if err := r.Total.Validate(); err != nil {
		return fmt.Errorf("spot balance %s: %w", r.Asset, err)
	}
	if r.Total.Asset() != r.Asset {
		return fmt.Errorf("spot balance %s carries a %s total: %w",
			r.Asset, r.Total.Asset(), ErrSpotBalancesUnreadable)
	}
	if len(r.Words) == 0 {
		// A row with no words is a conclusion with no inputs: the baseline
		// document stores the words, and a wordless row would store nothing
		// the loader could re-derive the total from.
		return fmt.Errorf("spot balance %s carries no verbatim words: %w",
			r.Asset, ErrSpotBalancesUnreadable)
	}
	return nil
}

// spotWordValues returns the named fields' values, requiring the words to be
// EXACTLY the named set — each present once, nothing extra.
//
// Strict in both directions because both consumers need it: the live decode
// building words it just read cannot miss, so a miss here is a codec defect;
// and the loader re-deriving a total from a STORED document must refuse a
// document carrying a word nothing consumes (§70b's rule about rows, applied
// to fields) as loudly as one missing the word the total rests on.
func spotWordValues(words []SpotBalanceWord, names ...string) ([]string, error) {
	if len(words) != len(names) {
		return nil, fmt.Errorf("%d words, want the %d-field set %v: %w",
			len(words), len(names), names, ErrSpotBalanceWordSet)
	}
	out := make([]string, len(names))
	for i, name := range names {
		found := false
		for _, w := range words {
			if w.Name != name {
				continue
			}
			if found {
				return nil, fmt.Errorf("word %q stated twice: %w", name, ErrSpotBalanceWordSet)
			}
			out[i] = w.Value
			found = true
		}
		if !found {
			return nil, fmt.Errorf("word %q missing: %w", name, ErrSpotBalanceWordSet)
		}
	}
	return out, nil
}
