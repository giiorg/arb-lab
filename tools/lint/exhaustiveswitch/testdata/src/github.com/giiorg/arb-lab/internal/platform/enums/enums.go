// Package enums declares the enum-likes the fixtures switch over. Each one
// exists to separate the rule from a specific wrong reading of it.
package enums

// Kind is the ordinary integer enum-like, carrying a bound sentinel that
// ALIASES its last member. maxKind is the same value as KindB under a second
// name, so a switch naming KindB has named it — and a gate keyed on names
// rather than values would demand a case for a constant that is already
// handled, and an unexported one at that.
type Kind uint8

const (
	KindUnspecified Kind = iota
	KindA
	KindB
)

const maxKind = KindB

// Label is a STRING enum-like. Rule 8's letter says integer, but the hazard is
// identical and excluding this would make the gate's coverage depend on a
// representation choice rather than on what the type is for.
type Label string

const (
	LabelX Label = "x"
	LabelY Label = "y"
)

// Solo has ONE member, which makes it a named constant rather than a set to
// exhaust over. Without the minimum, every switch over a type whose package
// happens to declare a single sentinel becomes a diagnostic.
type Solo uint8

const SoloOnly Solo = 1

// Flag is basic but neither integer nor string. It is the fixture for the
// Info() mask: a bool-underlying named type with two constants has exactly the
// shape the member scan looks for, and only the kind check excludes it.
type Flag bool

const (
	FlagOff Flag = false
	FlagOn  Flag = true
)
