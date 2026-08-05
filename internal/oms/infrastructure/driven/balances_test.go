package driven_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
	"github.com/giiorg/arb-lab/internal/platform/ratelimit"
)

// stubSpotBalanceReader exists for the construction cases, which never call
// it — silentReader's argument, one surface over.
type stubSpotBalanceReader struct{}

func (stubSpotBalanceReader) SpotBalances(context.Context, ratelimit.Priority) (venue.SpotBalanceReport, error) {
	return venue.SpotBalanceReport{}, errors.New("balances test: the venue surface is not exercised here")
}

var _ driven.SpotBalanceReader = stubSpotBalanceReader{}

// The behaviour of Balances — translation, venue scoping, the unpinned-reason
// switch — is exercised through the record protocol and the join against this
// adapter; what no fixture there can reach is the CONSTRUCTOR's refusals,
// because every composed caller supplies a complete wiring. The zero priority
// is the case that earns the file (record §70d): the priority is a constructor
// argument precisely because two call sites sit on two rungs, so a wiring that
// forgot to choose must be refused rather than inherit either.
func TestNewSpotBalancesRefusesAnIncompleteWiring(t *testing.T) {
	t.Parallel()

	if _, err := driven.NewSpotBalances(nil, "binance", ratelimit.PrioritySafety); !errors.Is(err, driven.ErrSpotBalanceReaderMissing) {
		t.Errorf("NewSpotBalances(nil reader) = %v, want ErrSpotBalanceReaderMissing", err)
	}
	if _, err := driven.NewSpotBalances(stubSpotBalanceReader{}, "", ratelimit.PrioritySafety); err == nil {
		t.Error("NewSpotBalances(unset venue) = nil, want a refusal")
	}
	if _, err := driven.NewSpotBalances(stubSpotBalanceReader{}, "binance", ratelimit.PriorityUnspecified); !errors.Is(err, ratelimit.ErrPriorityInvalid) {
		t.Errorf("NewSpotBalances(zero priority) = %v, want ErrPriorityInvalid", err)
	}

	// The positive control, without which three refusals above would be
	// satisfied by a constructor that refuses everything.
	if _, err := driven.NewSpotBalances(stubSpotBalanceReader{}, "binance", ratelimit.PrioritySafety); err != nil {
		t.Errorf("NewSpotBalances(complete wiring) = %v, want nil", err)
	}
}
