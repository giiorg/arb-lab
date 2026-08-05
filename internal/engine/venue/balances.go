package venue

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
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
