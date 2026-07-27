package enums

// The two test-file situations, which get OPPOSITE answers. Having both in one
// file is the point: a gate that simply skipped _test.go would pass the first
// and lose the second, and the two are indistinguishable without a fixture
// containing each.

// defect is declared HERE, in a test file — platform/clock's real one, in
// miniature: a set of injectable faults where a double implements the one or
// two it can express and delegates the rest. Exhaustiveness over it is no-op
// case labels, so it is out of scope and this switch is CLEAN.
type defect uint8

const (
	noDefect defect = iota
	clockGoesBackwards
	clockStandsStill
	orderingReversed
)

func broken(d defect) int {
	switch d {
	case clockGoesBackwards:
		return -1
	case clockStandsStill:
		return 0
	}
	return 1
}

// This switch is in a test file too, but over Kind — a PRODUCTION enum. It is
// policed, and not as a technicality: fixedpoint's math/big oracle dispatches
// on RoundingMode exactly like this, so a member added without a matching
// branch is a member whose independent implementation silently stops existing
// while the suite goes on reporting green.
func oracle(k Kind) string {
	switch k { // want `does not name KindB`
	case KindUnspecified:
		return "unspecified"
	case KindA:
		return "a"
	}
	return ""
}

var _, _ = broken, oracle
