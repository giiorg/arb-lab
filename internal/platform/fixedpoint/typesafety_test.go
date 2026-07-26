package fixedpoint_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Build-spec §3.6 rule 4 splits the guarantee in two: the four *categories* are
// distinct compile-time types so cross-category arithmetic will not build,
// while the asset tag within a category is checked at run time. Every other
// test in this package exercises the run-time half. This one exercises the
// compile-time half, which is otherwise an untested claim in a doc comment —
// and an untested claim about a type system is exactly the kind that survives a
// refactor that quietly turns a category into an alias.
//
// Each fixture under testdata/typesafety is a package that must fail to
// compile, with a substring naming *why*, so a fixture that breaks for an
// unrelated reason (a renamed method, a moved import path) does not read as a
// pass. The `valid` fixture is the positive control: it must build. Without it,
// a typo in the fixture path would make every case "fail to compile" and the
// whole test would be vacuous.
func TestCategoriesDoNotMix(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH; this test compiles fixtures")
	}

	cases := []struct {
		fixture string
		// wantErr is a substring the compiler's complaint must contain. It is
		// deliberately the type names rather than the full message, which
		// varies between Go releases.
		wantErr string
	}{
		{fixture: "valid"}, // must build
		{fixture: "money_plus_quantity", wantErr: "fixedpoint.Quantity"},
		{fixture: "quantity_plus_money", wantErr: "fixedpoint.Money"},
		{fixture: "money_plus_rate", wantErr: "fixedpoint.Rate"},
		{fixture: "assign_across_categories", wantErr: "fixedpoint.Quantity"},
		{fixture: "notional_with_price_as_quantity", wantErr: "fixedpoint.Quantity"},
		{fixture: "raw_mantissa_access", wantErr: "m"},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command("go", "build",
				"-o", filepath.Join(t.TempDir(), "out"),
				"./testdata/typesafety/"+tc.fixture)
			out, err := cmd.CombinedOutput()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("the positive-control fixture must compile, but did not:\n%s", out)
				}
				return
			}
			if err == nil {
				t.Fatalf("testdata/typesafety/%s compiled; it must not — "+
					"the categories are no longer distinct types", tc.fixture)
			}
			if !strings.Contains(string(out), tc.wantErr) {
				t.Errorf("testdata/typesafety/%s failed to compile, but not for the expected reason.\n"+
					"want the message to mention %q, got:\n%s", tc.fixture, tc.wantErr, out)
			}
		})
	}
}
