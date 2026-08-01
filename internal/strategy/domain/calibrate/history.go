package calibrate

import (
	"fmt"
	"time"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// validateHistory judges a settlement series once, up front: every row
// real, one instrument, strictly ascending. Shared by every derivation
// that walks history directly — the residual collection needs it for the
// rows EWMA's own window checks never reach (the last row joins no
// window), and the excursion walk calls no domain function that would
// check anything at all.
func validateHistory(history []domain.RealizedSettlement) error {
	if len(history) == 0 {
		return fmt.Errorf("history: %w", domain.ErrWindowEmpty)
	}
	for i, s := range history {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("history row %d: %w", i, err)
		}
		if s.Instrument != history[0].Instrument {
			return fmt.Errorf("history row %d is %s, history is %s: %w",
				i, s.Instrument, history[0].Instrument, domain.ErrLegsMismatch)
		}
		if i > 0 && !history[i-1].At.Before(s.At) {
			return fmt.Errorf("history row %d at %s, %d at %s: %w",
				i-1, history[i-1].At.Format(time.RFC3339), i, s.At.Format(time.RFC3339), domain.ErrNotAscending)
		}
	}
	return nil
}
