package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/giiorg/arb-lab/internal/platform/config"
)

// Schemas this package refuses to load. Each names the defect it carries, so a
// failure reads as a claim rather than as a type name.

type untaggedSchema struct {
	Journal string // no json tag
}

type dashTaggedSchema struct {
	Journal string `json:"-"`
}

type emptyNameSchema struct {
	Journal string `json:","`
}

// unexportedSchema's defect field deliberately carries NO json tag, which is the
// second attempt at isolating this check and the one that works.
//
// The problem §8f names is real here: a mutant deleting the exported-field check
// survived, because the fixture then failed the no-json-tag check with the same
// sentinel. The obvious fix — give the field a json tag so only one check can
// reject it — is not available: `go vet`'s structtag analyzer refuses a json tag
// on an unexported field, so the fixture would not build under `make ci`. The
// isolation comes from the MESSAGE assertion in the table instead, which is a
// weaker fixture and an equally sharp test.
type unexportedSchema struct {
	Journal string `json:"journal"`
	secret  string //nolint:unused // the defect under test
}

// Exported so the embedded field below is itself exported. Embedding the
// unexported testRisk made the fixture fail the exported-field check first, and
// a mutant that deleted the embedded-field check survived on it.
type Exported struct {
	X int `json:"x"`
}

type embeddedSchema struct {
	Exported `json:"embedded"`
	Journal  string `json:"journal"`
}

type anySchema struct {
	Params map[string]any `json:"params"`
}

type interfaceFieldSchema struct {
	Hook any `json:"hook"`
}

type badTagSchema struct {
	MaxOpen int `json:"max_total_open" config:"hto"`
}

type emptyTagSchema struct {
	MaxOpen int `json:"max_total_open" config:""`
}

type nestedDefectSchema struct {
	Risk struct {
		MaxOpen int // the untagged field is two levels down
	} `json:"risk"`
}

type chanSchema struct {
	Done chan struct{} `json:"done"`
}

type badMapKeySchema struct {
	Caps map[[2]string]int `json:"caps"`
}

func (Exported) Validate() error             { return nil }
func (untaggedSchema) Validate() error       { return nil }
func (dashTaggedSchema) Validate() error     { return nil }
func (emptyNameSchema) Validate() error      { return nil }
func (unexportedSchema) Validate() error     { return nil }
func (embeddedSchema) Validate() error       { return nil }
func (anySchema) Validate() error            { return nil }
func (interfaceFieldSchema) Validate() error { return nil }
func (badTagSchema) Validate() error         { return nil }
func (emptyTagSchema) Validate() error       { return nil }
func (nestedDefectSchema) Validate() error   { return nil }
func (chanSchema) Validate() error           { return nil }
func (badMapKeySchema) Validate() error      { return nil }

// notAStruct is a schema that is not an object. Its Validate makes it satisfy
// the constraint, which is exactly why the check has to exist: the type system
// cannot express "must be a struct" as a constraint.
type notAStruct map[string]int

func (notAStruct) Validate() error { return nil }

// TestSchemaAuditAcceptsTheFixture is the positive control. testConfig exercises
// every construct the audit walks — nested structs, a map of structs, a slice, a
// pointer, and two types that decode themselves — so an audit that rejected
// everything would fail here rather than score twelve kills below.
func TestSchemaAuditAcceptsTheFixture(t *testing.T) {
	if _, err := config.Decode[testConfig]([]byte(baseDoc)); err != nil {
		t.Fatalf("the fixture schema does not pass its own audit: %v", err)
	}
}

// TestSchemaAuditRejects checks each refusal by SENTINEL AND BY MESSAGE.
//
// The sentinel alone is not enough, and four surviving mutants proved it: every
// row below returns ErrSchemaField, so a deleted check whose fixture happened to
// trip the next check produced an identical error and an identical pass. The
// `says` column is what distinguishes them — it is the diagnosis the programmer
// acts on, and "this field has no json tag" and "this field's json name is
// empty" are different instructions.
func TestSchemaAuditRejects(t *testing.T) {
	tests := []struct {
		name    string
		decode  func() error
		wantErr error
		names   string   // where the defect is
		says    []string // and which check refused it
	}{
		{"a schema that is not a struct", decodes[notAStruct], config.ErrSchemaNotStruct, "", nil},
		{"a field with no json tag", decodes[untaggedSchema], config.ErrSchemaField, "Journal", []string{"no json tag"}},
		{"a json:\"-\" field", decodes[dashTaggedSchema], config.ErrSchemaField, "Journal", []string{`json:"-"`}},
		{"an empty json name", decodes[emptyNameSchema], config.ErrSchemaField, "Journal", []string{"empty json name"}},
		{"an unexported field", decodes[unexportedSchema], config.ErrSchemaField, "secret", []string{"unexported"}},
		{"an embedded field", decodes[embeddedSchema], config.ErrSchemaField, "Exported", []string{"embedded"}},
		{"a map to any", decodes[anySchema], config.ErrSchemaField, "params", []string{"interface"}},
		{"an interface field", decodes[interfaceFieldSchema], config.ErrSchemaField, "hook", []string{"interface"}},
		{"a channel field", decodes[chanSchema], config.ErrSchemaField, "done", []string{"chan"}},
		{"a map key that is not a JSON key", decodes[badMapKeySchema], config.ErrSchemaField, "caps", []string{"map key"}},
		{"a misspelled config tag", decodes[badTagSchema], config.ErrTagUnknown, "MaxOpen", []string{`"hto"`, `"hot"`}},
		{"an empty config tag", decodes[emptyTagSchema], config.ErrTagUnknown, "MaxOpen", []string{`"hot"`}},
		{"a defect nested two levels down", decodes[nestedDefectSchema], config.ErrSchemaField, "risk.MaxOpen", []string{"no json tag"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.decode()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Decode = %v, want %v", err, tc.wantErr)
			}
			if tc.names != "" && !strings.Contains(err.Error(), tc.names) {
				t.Errorf("error %q does not name %q", err, tc.names)
			}
			for _, phrase := range tc.says {
				if !strings.Contains(err.Error(), phrase) {
					t.Errorf("error %q does not say %q — a different check may have fired", err, phrase)
				}
			}
		})
	}
}

// decodes is the generic call the table above cannot write inline: a []struct
// field cannot hold a call with a type parameter, so each row stores this
// instantiated closure instead.
func decodes[T config.Validator]() error {
	_, err := config.Decode[T]([]byte(`{}`))
	return err
}

// TestSchemaAuditStopsAtSelfDecodingTypes is the rule that makes the fixture
// legal at all.
//
// SecretRef holds two unexported fields and Duration holds one, so an audit that
// walked into them would reject the very types this package ships. The stop rule
// is "implements json.Unmarshaler or encoding.TextUnmarshaler", and time.Time is
// the third-party case that proves it is a rule rather than a special case for
// two local types.
func TestSchemaAuditStopsAtSelfDecodingTypes(t *testing.T) {
	if _, err := config.Decode[selfDecoding]([]byte(
		`{"key":"env:K","dwell":"5m","asof":"2026-07-26T12:00:00Z"}`)); err != nil {
		t.Fatalf("Decode = %v, want nil: the audit walked into a self-decoding type", err)
	}
}

type selfDecoding struct {
	Key   config.SecretRef `json:"key"`
	Dwell config.Duration  `json:"dwell"`
	AsOf  time.Time        `json:"asof"`
}

func (selfDecoding) Validate() error { return nil }

// TestSchemaAuditTerminatesOnASelfReferentialType pins the cycle guard. Without
// it the walk recurses forever and the failure is a stack overflow rather than a
// diagnosis — and a linked-list-shaped config (an escalation ladder, say) is not
// an unreasonable thing for somebody to write.
func TestSchemaAuditTerminatesOnASelfReferentialType(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := config.Decode[recursiveSchema]([]byte(`{"name":"a","next":null}`))
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Decode = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the schema audit did not terminate on a self-referential type")
	}
}

type recursiveSchema struct {
	Name string           `json:"name"`
	Next *recursiveSchema `json:"next"`
}

func (recursiveSchema) Validate() error { return nil }

// TestSchemaAuditIsCachedButStillCorrect exercises the memoisation, which is the
// kind of optimisation that turns a per-type answer into a per-process one. Two
// different types must not share a verdict.
func TestSchemaAuditIsCachedButStillCorrect(t *testing.T) {
	for range 3 {
		if _, err := config.Decode[testConfig]([]byte(baseDoc)); err != nil {
			t.Fatalf("repeat Decode of a good schema = %v", err)
		}
		if err := decodes[untaggedSchema](); !errors.Is(err, config.ErrSchemaField) {
			t.Fatalf("repeat Decode of a bad schema = %v, want ErrSchemaField", err)
		}
	}
}
