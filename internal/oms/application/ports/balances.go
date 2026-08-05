package ports

import (
	"fmt"

	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/fixedpoint"
	"github.com/giiorg/arb-lab/internal/platform/ids"
)

// SpotBalances is the venue's spot BALANCE surface: what the account HOLDS,
// per asset — the surface §7.5's spot assertion compares against a recorded
// baseline, because neither venue has a position surface for spot and a spot
// balance is not a position (task #179, record §70).
//
// # A fourth read port, on VenuePositions' own argument
//
// A balance read is an assertion about the account, and its failure mode is an
// answer that arrives complete and describes a scope nobody meant to ask
// about. A use case holding only this port cannot reach for a per-order query
// to explain a delta away — the same separation that keeps the position
// enumeration honest, one surface over.
//
// # The enumeration is UNSCOPED by asset
//
// The read must be able to answer for an asset the caller did not name: the
// baseline's whole premise is coins this system never bought, and a per-asset
// query is exactly the read that cannot surface one nobody expected. Bybit's
// dialect makes the parameter itself load-bearing — the coin-less call returns
// every non-zero asset, so an ABSENT coin means a zero balance, while a
// coin-scoped call's silence means nothing (Profiles §4's dated 2026-08-05
// block). An implementation MUST NOT scope the venue call by asset.
type SpotBalances interface {
	// Balances enumerates the account's spot holdings for one scope.
	//
	// An error means the enumeration was not answered. It is never an empty
	// report: §5.3's rule about not inferring an absence from an unanswered
	// query, one surface over again — an empty report reads as an empty
	// wallet, which is what the baseline comparison concludes "flat" on.
	Balances(SpotBalanceScope) (BalanceReport, error)
}

// SpotBalanceScope is which venue's spot wallet to enumerate.
//
// It carries the venue and nothing else: the enumeration is deliberately
// unscoped by asset (see SpotBalances), the account is the credential's (the
// venue answers for the subaccount the signature names), and there is no
// settlement dimension — a wallet is enumerated whole.
type SpotBalanceScope struct {
	Venue ids.Venue
}

// Validate reports whether the scope names an enumeration a venue could serve.
func (s SpotBalanceScope) Validate() error {
	if err := s.Venue.Validate(); err != nil {
		return fmt.Errorf("spot balance scope venue: %w", err)
	}
	return nil
}

func (s SpotBalanceScope) String() string { return fmt.Sprintf("%s/spot-balances", s.Venue) }

// VerbatimField is one venue-published balance field, exactly as the wire
// carried it.
//
// The record command writes these into the baseline document, and the loader
// re-derives the typed total from them via the same per-venue rule the live
// adapter uses — so what the document preserves is the venue's word, and what
// every consumer computes from it goes through ONE meaning in one place
// (record §70d, review round 1 finding 2).
type VerbatimField struct {
	Name  string
	Value string
}

// AssetBalance is one asset's spot holding at the venue.
type AssetBalance struct {
	// Asset is the coin, tagged in the venue's own spelling.
	Asset fixedpoint.Asset

	// Total is the whole holding — free plus locked on Binance, walletBalance
	// on a pinned Bybit row — derived from Words by the adapter's per-venue
	// rule.
	Total fixedpoint.Quantity

	// Words are the venue's balance fields verbatim, the inputs Total is
	// derived from. Never empty on a well-formed row.
	Words []VerbatimField
}

// UnpinnedReason is why an asset's balance could not be given ONE total this
// read.
//
// It mirrors UnassertedReason's shape — a property of the reason, named
// members, exhaustiveswitch forcing the choice — because its consumer makes
// the same kind of decision: not "how big", but "may the assertion conclude".
type UnpinnedReason uint8

const (
	// UnpinnedReasonUnspecified is the zero value and names no reason.
	// Refused by Validate.
	UnpinnedReasonUnspecified UnpinnedReason = iota

	// UnpinnedLockedSplitUnverified is Bybit's unverified split: no doc
	// sentence states whether walletBalance includes locked, so a row with
	// locked != 0 has two candidate totals that differ by exactly locked.
	// Where locked == 0 the readings coincide and the row is pinned instead
	// (Profiles §4's named inference; #171's testnet session is the loosening
	// trigger).
	UnpinnedLockedSplitUnverified

	// UnpinnedBorrowOutstanding is a row with borrowAmount or spotBorrow
	// non-zero: borrowed coins inflate walletBalance without being holdings.
	// The §4.1 posture check refuses borrow separately; this port reports
	// about its OWN evidence rather than assuming a sibling gate ran.
	UnpinnedBorrowOutstanding
)

const maxUnpinnedReason = UnpinnedBorrowOutstanding

// Validate reports whether r names a reason, excluding the sentinel.
func (r UnpinnedReason) Validate() error {
	if r == UnpinnedReasonUnspecified || r > maxUnpinnedReason {
		return fmt.Errorf("unpinned reason %d: %w", uint8(r), ErrUnpinnedReasonInvalid)
	}
	return nil
}

// String implements fmt.Stringer.
func (r UnpinnedReason) String() string {
	switch r {
	case UnpinnedReasonUnspecified:
		return "UnpinnedReasonUnspecified"
	case UnpinnedLockedSplitUnverified:
		return "UnpinnedLockedSplitUnverified"
	case UnpinnedBorrowOutstanding:
		return "UnpinnedBorrowOutstanding"
	}
	return fmt.Sprintf("UnpinnedReason(%d)", uint8(r))
}

// UnpinnedBalance is one asset whose total this read could not pin.
type UnpinnedBalance struct {
	// Asset is the coin, in the venue's own spelling.
	Asset string

	// Reason is why the total could not be pinned.
	Reason UnpinnedReason
}

// BalanceReport is one enumeration's whole answer.
type BalanceReport struct {
	// ObservedAt is the venue clock instant of the read — carried because the
	// record command writes it into the baseline document, where it is the
	// operator's diagnosis anchor rather than a staleness gate (a baseline is
	// invalidated by events, never by time; record §70b).
	ObservedAt clock.ServerTime

	// Rows are the assets the venue reported with a pinnable total. An asset
	// ABSENT from Rows and Unpinned both is a zero balance — a real answer,
	// available only because the enumeration is unscoped (see SpotBalances).
	Rows []AssetBalance

	// Unpinned are the assets whose total this read could NOT pin, each with
	// the reason. The assertion neither concludes nor latches on one — it
	// reports the asset unconcluded, boot.OutcomeIncomplete's cell (record
	// §70c).
	Unpinned []UnpinnedBalance
}
