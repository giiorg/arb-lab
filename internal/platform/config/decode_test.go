package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

// TestDecodeAcceptsAValidDocument is the positive control for every rejection
// test in this file. Without it a Decode that refused everything would score a
// perfect result on all of them (§4d's standing lesson).
func TestDecodeAcceptsAValidDocument(t *testing.T) {
	got, err := config.Decode[testConfig]([]byte(baseDoc))
	if err != nil {
		t.Fatalf("Decode(baseDoc) = %v, want nil", err)
	}

	if got.Journal != "/var/lib/arblab/oms.db" {
		t.Errorf("Journal = %q", got.Journal)
	}
	if got.Risk.MaxTotalOpen != 3 {
		t.Errorf("Risk.MaxTotalOpen = %d, want 3", got.Risk.MaxTotalOpen)
	}
	// Compared as a Duration rather than via .Seconds(), which returns a
	// float64 and is exactly what nofloat (§3.6 rule 4) exists to stop — it
	// fired on this line as first written.
	if got.Risk.TBlind.Duration() != 90*time.Second {
		t.Errorf("Risk.TBlind = %v, want 90s", got.Risk.TBlind)
	}
	if len(got.Venues) != 2 {
		t.Fatalf("len(Venues) = %d, want 2", len(got.Venues))
	}
	if ref := got.Venues["binance"].KeyRef; ref.Scheme() != config.SchemeEnv || ref.Name() != "BINANCE_API_KEY" {
		t.Errorf("binance KeyRef = %v", ref)
	}
	if got.Strategy == nil || got.Strategy.MinFundingAPR != "0.05" {
		t.Errorf("Strategy = %+v", got.Strategy)
	}
}

// TestDecodeRejectsADuplicateKey is the check Go's decoder does not make.
//
// The interesting fixture is the one where BOTH occurrences are valid: json
// takes the last and reports nothing, so without the token scan the document
// loads, validates, and runs at a cap nobody chose. The assertion on which value
// would have won is what makes the test name a consequence rather than a rule.
func TestDecodeRejectsADuplicateKey(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		path string
	}{
		{
			name: "at the document root",
			doc:  `{"journal":"a","journal":"b","risk":{"max_total_open":3,"t_blind":"90s","mark_blind_action":"close"},"venues":{},"symbols":[],"strategy":null}`,
			path: "journal",
		},
		{
			name: "nested, both values valid",
			doc:  strings.Replace(baseDoc, `"max_total_open": 3,`, `"max_total_open": 3, "max_total_open": 99,`, 1),
			path: "risk.max_total_open",
		},
		{
			name: "inside a map entry",
			doc:  strings.Replace(baseDoc, `"rate_limit": 1200`, `"rate_limit": 1200, "rate_limit": 5`, 1),
			path: "venues.binance.rate_limit",
		},
		{
			name: "inside an array element",
			doc:  strings.Replace(baseDoc, `{"min_funding_apr": "0.05"}`, `{"min_funding_apr": "0.05", "min_funding_apr": "0.99"}`, 1),
			path: "strategy.min_funding_apr",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Decode[testConfig]([]byte(tc.doc))
			if !errors.Is(err, config.ErrDuplicateKey) {
				t.Fatalf("Decode = %v, want ErrDuplicateKey", err)
			}
			if !strings.Contains(err.Error(), tc.path) {
				t.Errorf("error %q does not name the path %q", err, tc.path)
			}
		})
	}
}

// TestDecodeDuplicateKeyWouldOtherwiseBeSilent measures what the check is worth,
// rather than asserting that it exists.
//
// It decodes the same document WITHOUT the guard — through encoding/json
// directly — and asserts that json accepts it and takes the second value. If
// that ever stops being true the guard is no longer load-bearing, and this test
// says so rather than leaving a check nobody can justify.
func TestDecodeDuplicateKeyWouldOtherwiseBeSilent(t *testing.T) {
	const dup = `{"max_total_open":3,"t_blind":"90s","mark_blind_action":"close","max_total_open":99}`

	var got testRisk
	if err := jsonUnmarshalStrict(dup, &got); err != nil {
		t.Fatalf("encoding/json rejected a duplicate key (%v) — the guard in decode.go "+
			"may no longer be load-bearing; re-derive it before deleting it", err)
	}
	if got.MaxTotalOpen != 99 {
		t.Fatalf("encoding/json resolved the duplicate to %d, want the LAST value 99", got.MaxTotalOpen)
	}

	// And the same bytes through this package.
	if _, err := config.Decode[testRisk]([]byte(dup)); !errors.Is(err, config.ErrDuplicateKey) {
		t.Errorf("Decode = %v, want ErrDuplicateKey", err)
	}
}

// Validate lets testRisk stand alone as a schema for the test above.
func (r testRisk) Validate() error { return nil }

func TestDecodeRejectsAnUnknownKey(t *testing.T) {
	bad := strings.Replace(baseDoc, `"max_total_open": 3,`, `"max_total_open": 3, "max_total_openn": 9,`, 1)

	_, err := config.Decode[testConfig]([]byte(bad))
	if err == nil {
		t.Fatal("Decode accepted an unknown key")
	}
	if !strings.Contains(err.Error(), "max_total_openn") {
		t.Errorf("error %q does not name the unknown key", err)
	}
}

// TestDecodeRejectsTrailingData covers the second thing json.Decoder does
// quietly: it reads ONE value and leaves the rest of the reader alone.
func TestDecodeRejectsTrailingData(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"a second object", baseDoc + "\n" + baseDoc},
		{"a stray scalar", baseDoc + "\n7"},
		{"an unterminated fragment", baseDoc + "\n{"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := config.Decode[testConfig]([]byte(tc.doc)); !errors.Is(err, config.ErrTrailingData) {
				t.Fatalf("Decode = %v, want ErrTrailingData", err)
			}
		})
	}
}

// TestDecodeTrailingDataWouldOtherwiseBeSilent is the companion measurement: two
// concatenated documents decode to the FIRST one, with no error.
func TestDecodeTrailingDataWouldOtherwiseBeSilent(t *testing.T) {
	first := `{"max_total_open":1,"t_blind":"1s","mark_blind_action":"close"}`
	second := `{"max_total_open":2,"t_blind":"2s","mark_blind_action":"close"}`

	var got testRisk
	if err := jsonUnmarshalStrict(first+second, &got); err != nil {
		t.Fatalf("encoding/json rejected trailing data (%v) — re-derive the guard before deleting it", err)
	}
	if got.MaxTotalOpen != 1 {
		t.Fatalf("encoding/json read %d, want the FIRST document's 1", got.MaxTotalOpen)
	}

	if _, err := config.Decode[testRisk]([]byte(first + second)); !errors.Is(err, config.ErrTrailingData) {
		t.Errorf("Decode = %v, want ErrTrailingData", err)
	}
}

func TestDecodeRejectsAnEmptyDocument(t *testing.T) {
	for _, doc := range []string{"", "   ", "\n\t\n", "  \r\n  "} {
		if _, err := config.Decode[testConfig]([]byte(doc)); !errors.Is(err, config.ErrEmptyDocument) {
			t.Errorf("Decode(%q) = %v, want ErrEmptyDocument", doc, err)
		}
	}
}

// TestDecodeRunsTheSchemasOwnValidate confirms the last step of the chain: a
// syntactically perfect document whose values are wrong is still refused.
//
// The max_total_open case is the one that matters for §11's unusable-zero rule.
// Omitting the field entirely is legal JSON and leaves the cap at 0, and a cap
// of zero is not "no limit" or "block everything" until you know which side of a
// comparison it lands on — so the schema, not this package, has to say so.
func TestDecodeRunsTheSchemasOwnValidate(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"a field omitted entirely leaves a zero cap", strings.Replace(baseDoc, `"max_total_open": 3,`, "", 1), "max_total_open"},
		{"an explicit zero", withField(fMaxOpen, "0"), "max_total_open"},
		{"an empty journal path", withField(fJournal, `""`), "journal"},
		{"an unrecognised blind action", withField(fBlindAction, `"panic"`), "mark_blind_action"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Decode[testConfig]([]byte(tc.doc))
			if err == nil {
				t.Fatal("Decode accepted a document its schema rejects")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestDecodeChecksTheSchemaBeforeTheDocument pins the ORDER of the two failure
// families. A programmer's broken schema must not be reported as an operator's
// broken file: the document here is empty, which is the loudest possible
// document error, and the schema error must still be what comes back.
func TestDecodeChecksTheSchemaBeforeTheDocument(t *testing.T) {
	_, err := config.Decode[untaggedSchema]([]byte(""))
	if !errors.Is(err, config.ErrSchemaField) {
		t.Fatalf("Decode = %v, want ErrSchemaField (the schema, not the empty document)", err)
	}
}
