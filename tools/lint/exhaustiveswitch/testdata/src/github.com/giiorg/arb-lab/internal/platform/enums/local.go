package enums

// Same-package switches. These are where the alias sentinel is visible at all:
// maxKind is unexported, so a switch in another package could never be asked
// for it and the value-grouping claim would go unmeasured there.

// Exhaustive returns true and is CLEAN. It names all three members; maxKind is
// KindB under a second name and is therefore already covered. Key the members
// by name instead of by value and this file starts failing.
func Exhaustive(k Kind) string {
	switch k {
	case KindUnspecified:
		return "unspecified"
	case KindA:
		return "a"
	case KindB:
		return "b"
	}
	return ""
}

// MissingLast omits KindB, which maxKind also names. The diagnostic must say
// KindB — the name a reader has to go and write — rather than the unexported
// alias that happens to sort first.
func MissingLast(k Kind) string {
	switch k { // want `does not name KindB`
	case KindUnspecified:
		return "unspecified"
	case KindA:
		return "a"
	}
	return ""
}

// WithDefault has a default clause and is STILL offending. This is the whole
// rule: the default is where a new member lands silently, so treating it as
// coverage would remove the only thing the gate does.
func WithDefault(k Kind) string {
	switch k { // want `does not name KindA, KindB`
	case KindUnspecified:
		return "unspecified"
	default:
		return "something else"
	}
}

// StringEnum shows the string-underlying enum-like is policed too.
func StringEnum(l Label) int {
	switch l { // want `does not name LabelY`
	case LabelX:
		return 1
	}
	return 0
}

// SingleMember is CLEAN: one constant of a named type is a named constant, not
// a set to exhaust over.
//
// It names NO member, and that is the fixture rather than an oversight. The
// obvious version — `case SoloOnly:` — is exhaustive over a one-member enum, so
// it reads identically whether the minimum is one or two, and the mutation
// lowering it survived against exactly that. The distinction only exists on a
// switch that would be reported if Solo counted.
func SingleMember(s Solo) bool {
	switch s {
	default:
		return false
	}
}

// BoolBacked is CLEAN: basic, two constants, but neither integer nor string.
func BoolBacked(f Flag) int {
	switch f {
	case FlagOn:
		return 1
	}
	return 0
}

// Grouped is CLEAN, and it is the alternative this gate offers instead of an
// escape annotation: a switch that genuinely cares about one member names the
// rest in one grouped case. The decision is recorded at the decision site, in
// code the compiler checks.
func Grouped(k Kind) bool {
	switch k {
	case KindB:
		return true
	case KindUnspecified, KindA:
		// Not tradeable, and named rather than defaulted.
		return false
	}
	return false
}
