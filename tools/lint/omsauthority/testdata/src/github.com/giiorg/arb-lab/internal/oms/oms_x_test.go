// Package oms_test is the owner's EXTERNAL test package, which Go loads as
// internal/oms_test — outside internal/oms by path, inside it by directory.
// This is the shape singlewriter shipped broken: within() alone answers NO,
// because Go suffixes the last SEGMENT rather than adding a directory.
package oms_test

import (
	"testing"

	"github.com/giiorg/arb-lab/internal/oms"
)

func TestModule(t *testing.T) {
	var _ oms.Module
}
