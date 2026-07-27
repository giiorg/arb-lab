package sub_test

// An EXTERNAL test package for a NESTED domain package. Go names it
// ".../domain/sub_test", which still carries the "domain" segment, so this one
// is policed whether or not anybody decided it should be.
//
// Its pair lives beside the top-level domain package, whose external test
// package is named ".../domain_test" and carries no "domain" segment at all.
// The two fixtures exist together because they are the measurement: before the
// suffix normalisation they disagreed, and nothing in the analyzer said so.
import (
	"testing"

	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven" // want `domain must not import infrastructure`
)

func TestExternalIsPolicedToo(t *testing.T) {
	_ = driven.Client{}
}
