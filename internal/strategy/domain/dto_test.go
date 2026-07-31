package domain_test

// The wire battery (§40f): byte-level round-trips over every dimensional
// field, every revision identity, a refusal, and the dwell state — plus the
// strict-decode refusals, because on this wire a silently-defaulted member is
// a flattering term in a journal that recovery would faithfully preserve.

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

// roundTrip pins both directions: domain → bytes → domain equality, and
// bytes → domain → bytes equality. The byte half is what catches a lossy
// representation (a float rounding, a dropped tag) that value equality alone
// can miss when BOTH sides lose the same information.
func roundTrip(t *testing.T, d domain.Decision) {
	t.Helper()

	data, err := domain.EncodeDecision(d)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	back, err := domain.DecodeDecision(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(d, back) {
		t.Fatalf("value round-trip diverged:\n was %+v\n got %+v", d, back)
	}
	again, err := domain.EncodeDecision(back)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if !bytes.Equal(data, again) {
		t.Fatalf("byte round-trip diverged:\n was %s\n got %s", data, again)
	}
}

func TestRoundTripEntryDecision(t *testing.T)   { roundTrip(t, entryDecision(t)) }
func TestRoundTripExitDecision(t *testing.T)    { roundTrip(t, exitDecision(t)) }
func TestRoundTripRefusalDecision(t *testing.T) { roundTrip(t, refusalDecision(t)) }

// TestRoundTripDeclineAndClose covers the two scored negative verdicts, so
// verdict text itself is exercised across the full set.
func TestRoundTripDeclineAndClose(t *testing.T) {
	d := entryDecision(t)
	d.Verdict = domain.VerdictDecline
	roundTrip(t, d)

	x := exitDecision(t)
	x.Verdict = domain.VerdictClose
	roundTrip(t, x)
}

// TestWireCarriesExactMantissaPast2p53 pins the string channel: the fixture's
// fee/slippage reserve mantissa (9_007_199_254_740_993) and the perp
// watermark's venue sequence (9_007_199_254_740_995) are chosen to be
// unrepresentable in a float64 — if either ever rides a JSON number, the
// round-trip above still passes VALUE equality on some platforms, but the
// bytes here pin the exact digits.
func TestWireCarriesExactMantissaPast2p53(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, digits := range []string{
		`"9007199254740993"`, // fee/slippage reserve mantissa, as a string
		`"9007199254740995"`, // perp book venue seq, as a string
	} {
		if !bytes.Contains(data, []byte(digits)) {
			t.Fatalf("wire does not carry %s as a string token:\n%s", digits, data)
		}
	}
}

// TestEncodeRefusesInvalidDecision: the journal write path validates before
// encoding — an invalid Decision durably written is a hole recovery would
// preserve.
func TestEncodeRefusesInvalidDecision(t *testing.T) {
	d := entryDecision(t)
	d.RefusedBy = "rule on a scored verdict"
	if _, err := domain.EncodeDecision(d); err == nil {
		t.Fatal("encode accepted an invalid decision")
	}
}

// --- strict decode ---------------------------------------------------------

func TestDecodeRefusesUnknownField(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	// Inject an unknown top-level field.
	patched := bytes.Replace(data, []byte(`{"v":1`), []byte(`{"v":1,"surprise":true`), 1)
	if bytes.Equal(patched, data) {
		t.Fatal("patch did not apply")
	}
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("unknown field: want ErrWireMalformed, got %v", err)
	}
}

func TestDecodeRefusesTrailingData(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := domain.DecodeDecision(append(data, []byte("{}")...)); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("trailing data: want ErrWireMalformed, got %v", err)
	}
}

func TestDecodeRefusesWrongVersion(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(data, []byte(`{"v":1`), []byte(`{"v":2`), 1)
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireVersion) {
		t.Fatalf("v2: want ErrWireVersion, got %v", err)
	}
	// And a missing version outright.
	patched = bytes.Replace(data, []byte(`{"v":1,`), []byte(`{`), 1)
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("no version: want ErrWireMalformed, got %v", err)
	}
}

func TestDecodeRefusesForeignScale(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(data, []byte(`"s":9`), []byte(`"s":8`), 1)
	if bytes.Equal(patched, data) {
		t.Fatal("patch did not apply")
	}
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireScale) {
		t.Fatalf("scale 8: want ErrWireScale, got %v", err)
	}
}

func TestDecodeRefusesBadMantissa(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(data, []byte(`"m":"9007199254740993"`), []byte(`"m":"90071992547409x3"`), 1)
	if bytes.Equal(patched, data) {
		t.Fatal("patch did not apply")
	}
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("bad mantissa: want ErrWireMalformed, got %v", err)
	}
}

func TestDecodeRefusesUppercaseHash(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	// The revision hash starts abcd03…; uppercase ONE letter, same length,
	// so the length rule cannot answer for the case rule. Case-folding
	// would give the journal token two spellings.
	patched := bytes.Replace(data, []byte(`"hash":"abcd03`), []byte(`"hash":"Abcd03`), 1)
	if bytes.Equal(patched, data) {
		t.Fatal("patch did not apply")
	}
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("uppercase hash: want ErrWireMalformed, got %v", err)
	}
}

// TestDecodeMissingMemberIsARefusal drops one required member from the wire
// (the hurdle) and expects the DOMAIN refusal — the missing-member rule rides
// the one validator, not a per-field decode check.
func TestDecodeMissingMemberIsARefusal(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(m["input"], &input); err != nil {
		t.Fatal(err)
	}
	input["hurdle"] = json.RawMessage("null")
	patchedInput, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	m["input"] = patchedInput
	patched, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	_, err = domain.DecodeDecision(patched)
	if err == nil {
		t.Fatal("decision with no hurdle decoded")
	}
	if !strings.Contains(err.Error(), "hurdle") {
		t.Fatalf("refusal does not name the hurdle: %v", err)
	}
}

// TestDecodeIsDomainValidated: bytes that parse cleanly but describe an
// invalid decision (verdict/kind disagreement) are refused by the same
// domain rule that would refuse the value.
func TestDecodeIsDomainValidated(t *testing.T) {
	data, err := domain.EncodeDecision(entryDecision(t))
	if err != nil {
		t.Fatal(err)
	}
	patched := bytes.Replace(data, []byte(`"verdict":"enter"`), []byte(`"verdict":"close"`), 1)
	if bytes.Equal(patched, data) {
		t.Fatal("patch did not apply")
	}
	if _, err := domain.DecodeDecision(patched); !errors.Is(err, domain.ErrVerdictKindMismatch) {
		t.Fatalf("close on entry: want ErrVerdictKindMismatch, got %v", err)
	}
}

func TestDecodeRefusesNonJSON(t *testing.T) {
	if _, err := domain.DecodeDecision([]byte("not json")); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("want ErrWireMalformed, got %v", err)
	}
	if _, err := domain.DecodeDecision(nil); !errors.Is(err, domain.ErrWireMalformed) {
		t.Fatalf("empty: want ErrWireMalformed, got %v", err)
	}
}
