// Package domain holds the CROSS-PACKAGE half, plus the edge cases where the
// gate must say nothing.
package domain

import (
	"reflect"

	"github.com/giiorg/arb-lab/internal/platform/enums"
)

// CrossPackage is offending over an IMPORTED enum. Rule 10's subject is the
// decision site, and most decision sites for a domain enum are in some other
// package — a gate that only saw same-package switches would be green over
// exactly the code that matters.
func CrossPackage(k enums.Kind) string {
	switch k { // want `does not name KindA, KindB`
	case enums.KindUnspecified:
		return "unspecified"
	}
	return ""
}

// CrossPackageClean names all three. maxKind is unexported and unreachable from
// here, and is not demanded — the gate can only ask for what is nameable.
func CrossPackageClean(k enums.Kind) string {
	switch k {
	case enums.KindUnspecified:
		return "unspecified"
	case enums.KindA:
		return "a"
	case enums.KindB:
		return "b"
	}
	return ""
}

// StdlibEnum is CLEAN. reflect.Kind is a 27-member enum this module did not
// declare, and switching on three of them is what everybody does. It is the
// fixture for the module-root scope: three of the eighteen reports `exhaustive`
// produced over the real tree were this, and they are the reason its
// -ignore-enum-types list would have had to start growing on day one.
func StdlibEnum(k reflect.Kind) int {
	switch k {
	case reflect.Int:
		return 1
	case reflect.String:
		return 2
	}
	return 0
}

// NonConstantCase is CLEAN because it cannot be judged. What the switch covers
// is not decidable statically, and this gate's diagnostics are build failures —
// one that guesses is one somebody configures away.
func NonConstantCase(k enums.Kind, dynamic enums.Kind) bool {
	switch k {
	case dynamic:
		return true
	}
	return false
}

// Tagless is CLEAN: a chain of booleans is not a dispatch over a value, so
// there is no member set to exhaust.
func Tagless(k enums.Kind) bool {
	switch {
	case k > enums.KindA:
		return true
	}
	return false
}

// TypeSwitch is CLEAN for the same reason — the tag is not a value of an
// enum-like, and ast.SwitchStmt does not even match it.
func TypeSwitch(v any) int {
	switch v.(type) {
	case int:
		return 1
	}
	return 0
}
