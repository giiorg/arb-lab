package domain_test

// The fixture the suffix normalisation exists for, and the ONLY path shape that
// separates it from doing nothing.
//
// This package's import path is ".../marketdata/domain_test" — the last segment
// is "domain_test", not "domain", so a segment scan finds no layer here and the
// file goes unpoliced. Its sibling at domain/sub/external_test.go is the same
// construct one directory deeper, where the "domain" segment survives and the
// scan does police it. Delete withoutTestSuffix and exactly this file stops
// failing while that one keeps passing, which is what an unprincipled rule
// looks like from the inside.
import (
	"testing"

	"github.com/giiorg/arb-lab/internal/marketdata/infrastructure/driven" // want `domain must not import infrastructure`
)

func TestTopLevelExternalIsPolicedToo(t *testing.T) {
	_ = driven.Client{}
}
