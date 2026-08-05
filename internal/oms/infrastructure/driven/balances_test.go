package driven_test

import (
	"context"
	"errors"
	"testing"

	"github.com/giiorg/arb-lab/internal/engine/venue"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
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

// scriptedSpotBalanceReader answers with one fixed report.
type scriptedSpotBalanceReader struct{ report venue.SpotBalanceReport }

func (s scriptedSpotBalanceReader) SpotBalances(context.Context, ratelimit.Priority) (venue.SpotBalanceReport, error) {
	return s.report, nil
}

// TestSpotBalancesTranslatesTheAbsenceSemantics is §70i: the engine's absence
// statement crosses the ACL verbatim in meaning — each named member to its
// port twin, and the zero value refused HERE, at the earliest seam, because a
// report that never stated its silences' meaning must not reach a consumer
// that branches on it. The composed fixtures exercise only the codec each
// venue actually stamps, so the other arms need this table.
func TestSpotBalancesTranslatesTheAbsenceSemantics(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   venue.SpotAbsenceSemantics
		want ports.AbsenceSemantics
		fail bool
	}{
		"zero maps to AbsenceIsZero":                 {in: venue.SpotAbsenceZero, want: ports.AbsenceIsZero},
		"inconclusive maps to AbsenceInconclusive":   {in: venue.SpotAbsenceInconclusive, want: ports.AbsenceInconclusive},
		"the unspecified zero value refuses instead": {in: venue.SpotAbsenceUnspecified, fail: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reader := scriptedSpotBalanceReader{report: venue.SpotBalanceReport{Absence: tc.in}}
			adapter, err := driven.NewSpotBalances(reader, "binance", ratelimit.PrioritySafety)
			if err != nil {
				t.Fatalf("NewSpotBalances: %v", err)
			}
			got, err := adapter.Balances(ports.SpotBalanceScope{Venue: "binance"})
			if tc.fail {
				if !errors.Is(err, venue.ErrSpotAbsenceSemanticsInvalid) {
					t.Errorf("Balances = %v, want ErrSpotAbsenceSemanticsInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Balances: %v", err)
			}
			if got.Absence != tc.want {
				t.Errorf("Absence = %s, want %s", got.Absence, tc.want)
			}
		})
	}
}
