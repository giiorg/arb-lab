// Package fixedpoint is the positive control: integer money arithmetic, which
// must produce no diagnostic at all. Without it a rule that objected to every
// line would look like a flawless gate.
package fixedpoint

import "strconv"

const Scale = 9

type Money struct {
	mantissa int64
	asset    string
}

// Integer division with an explicit remainder — the shape the real kernel uses,
// and the one the rule exists to make unavoidable.
func (m Money) Split(n int64) (Money, int64) {
	if n == 0 {
		return Money{}, 0
	}
	return Money{m.mantissa / n, m.asset}, m.mantissa % n
}

// Decimal text in and out, with no numeric detour. strconv is imported here on
// purpose: the package that owns ParseFloat is not itself a violation, so an
// import-based rule would have to allow it and a type-based rule simply does.
func (m Money) Decimal() string {
	return strconv.FormatInt(m.mantissa, 10)
}

func Parse(s string) (Money, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	return Money{v, "USDT"}, err
}
