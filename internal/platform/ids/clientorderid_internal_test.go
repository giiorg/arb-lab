package ids

import "testing"

// The format's constants, pinned against each other.
//
// NewClientOrderID ends with a runtime assertion that the derived id is exactly
// clientOrderIDLen characters. That assertion is UNREACHABLE — the range checks
// above it already guarantee two digits per numeric field and the base32 field
// is fixed at 26 — and a mutation disabling it therefore survives the suite. It
// is a semantically equivalent mutant rather than a test gap, and this file is
// where the claim it stands for gets tested instead: that the length constant
// and the format that produces it have not drifted apart.
//
// (Recorded rather than papered over, and the runtime check is kept: it costs
// one branch and it is what would catch a future edit to the format string
// before an id reaches a venue.)
func TestClientOrderIDConstantsAgree(t *testing.T) {
	// prefix + base32 + '-' + 2 digits + '-' + 2 digits.
	const composed = len(clientOrderIDPrefix) + base32Len + 1 + 2 + 1 + 2
	if clientOrderIDLen != composed {
		t.Errorf("clientOrderIDLen = %d but the format composes to %d", clientOrderIDLen, composed)
	}

	// The venue-side budget. This is the constant that carries the UNVERIFIED
	// venue assumption documented on ClientOrderID — 36 characters and
	// [A-Za-z0-9_-] on both Binance and Bybit, not yet browser-confirmed to the
	// standard docs/venue-profiles.md sets. Confirm before milestone 3 places a
	// real order. The assertion is here so that a future widening of the format
	// fails a test rather than a submission.
	if clientOrderIDLen > clientOrderIDBudget {
		t.Errorf("the format is %d characters, past the %d-character venue budget",
			clientOrderIDLen, clientOrderIDBudget)
	}

	// The numeric fields are two digits, so their bounds are two digits' worth.
	// A maxLeg of 100 would render as three characters and silently push the id
	// past the budget.
	if maxLeg != 99 {
		t.Errorf("maxLeg = %d, but the format allocates two digits", maxLeg)
	}
	if maxAttempt != 99 {
		t.Errorf("maxAttempt = %d, but the format allocates two digits", maxAttempt)
	}
}

// TestPrefixIsClearOfVenueSystemIDs checks our prefix against the ones Binance
// stamps on its own closes (venue-profiles §R1d-7). Neither can be a prefix of
// the other, or a system id could parse as ours — attributing a liquidation to a
// trade we placed.
func TestPrefixIsClearOfVenueSystemIDs(t *testing.T) {
	for _, system := range []string{"autoclose-", "adl_autoclose", "settlement_autoclose-"} {
		n := min(len(clientOrderIDPrefix), len(system))
		if clientOrderIDPrefix[:n] == system[:n] {
			t.Errorf("our prefix %q collides with the venue system prefix %q",
				clientOrderIDPrefix, system)
		}
	}
}
