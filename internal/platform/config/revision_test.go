package config_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

// TestRevisionZeroValueIsUnusable is §11's rule on the type a journal entry
// carries. A zero Revision reaching a replay log would record "configuration
// unknown, from nowhere, at the epoch" in the shape of a fact.
func TestRevisionZeroValueIsUnusable(t *testing.T) {
	var r config.Revision
	if err := r.Validate(); !errors.Is(err, config.ErrRevisionInvalid) {
		t.Fatalf("zero Revision Validate = %v, want ErrRevisionInvalid", err)
	}
}

// TestRevisionValidateChecksEachFieldIndependently is the isolation discipline
// §8f paid for: with one sentinel error, a test whose fixture fails several
// checks cannot say which one fired. Each row is valid in every respect but one.
func TestRevisionValidateChecksEachFieldIndependently(t *testing.T) {
	good := config.Revision{
		Num:      1,
		Hash:     sha256.Sum256([]byte("x")),
		Source:   "configs/test.json",
		LoadedAt: testAt,
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("the positive control does not validate: %v", err)
	}

	tests := []struct {
		name  string
		mut   func(config.Revision) config.Revision
		names string
	}{
		{"revision number zero", func(r config.Revision) config.Revision { r.Num = 0; return r }, "number"},
		{"unset hash", func(r config.Revision) config.Revision { r.Hash = [32]byte{}; return r }, "hash"},
		{"unset source", func(r config.Revision) config.Revision { r.Source = ""; return r }, "source"},
		{"zero instant", func(r config.Revision) config.Revision { r.LoadedAt = time.Time{}; return r }, "instant"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mut(good).Validate()
			if !errors.Is(err, config.ErrRevisionInvalid) {
				t.Fatalf("Validate = %v, want ErrRevisionInvalid", err)
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("error %q does not say which field (%q)", err, tc.names)
			}
		})
	}
}

// TestRevisionHashIsContentAddressed is what makes a revision replayable. The
// expected value comes from crypto/sha256 directly rather than from whatever
// this package produced, so the test is an independent statement about the bytes
// and not an assertion that the code equals itself (§8e).
func TestRevisionHashIsContentAddressed(t *testing.T) {
	s := mustStore(t)
	rev := s.Current().Rev

	want := sha256.Sum256([]byte(baseDoc))
	if rev.Hash != want {
		t.Errorf("Hash = %x, want sha256(baseDoc) = %x", rev.Hash, want)
	}
	if got := rev.HashString(); got != hex.EncodeToString(want[:]) {
		t.Errorf("HashString = %q", got)
	}

	// Reproducible across stores: the same document loaded again — on another
	// host, a year later, in a backtest — has the same identity.
	other, err := config.New[testConfig]("elsewhere.json", testAt.Add(999*time.Hour), []byte(baseDoc))
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if other.Current().Rev.Hash != rev.Hash {
		t.Error("the same bytes hashed differently in two stores")
	}
}

// TestRevisionStringNamesTheDocument checks the log line an operator reads. The
// abbreviation is deliberate — HashString is the recording form — so the test
// asserts the prefix relationship rather than a hard-coded string.
func TestRevisionStringNamesTheDocument(t *testing.T) {
	rev := mustStore(t).Current().Rev
	got := rev.String()

	for _, want := range []string{"rev 1", "sha256:" + rev.HashString()[:12], "configs/test.json"} {
		if !strings.Contains(got, want) {
			t.Errorf("String = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, rev.HashString()) {
		t.Error("String printed the full hash; the abbreviation is what makes it readable")
	}
}
