package outbox_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/giiorg/arb-lab/internal/platform/outbox"
)

// validMessage is the fixture every rejection case starts from, so that a test
// asserting "this field is rejected" is not quietly passing because a DIFFERENT
// field was also wrong. §8f's standing lesson: to isolate a check, build the
// input that passes every other one.
func validMessage() outbox.Message {
	return outbox.Message{
		Topic:   "audit",
		Key:     "trade-01J000000000000000000000",
		Type:    "oms.FillApplied",
		Payload: []byte(`{"seq":1}`),
	}
}

func TestMessageValidateAcceptsTheFixture(t *testing.T) {
	// The positive control. Without it every rejection below would pass
	// against a Validate that objects to everything.
	if err := validMessage().Validate(); err != nil {
		t.Fatalf("the shared valid fixture must validate, got %v", err)
	}
}

func TestMessageValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*outbox.Message)
		want error
	}{
		{
			name: "zero value",
			mut:  func(m *outbox.Message) { *m = outbox.Message{} },
			want: outbox.ErrTopicUnset,
		},
		{
			name: "topic unset",
			mut:  func(m *outbox.Message) { m.Topic = "" },
			want: outbox.ErrTopicUnset,
		},
		{
			name: "topic with a space",
			mut:  func(m *outbox.Message) { m.Topic = "audit projection" },
			want: outbox.ErrTopicInvalid,
		},
		{
			name: "topic with a newline",
			mut:  func(m *outbox.Message) { m.Topic = "audit\nprojection" },
			want: outbox.ErrTopicInvalid,
		},
		{
			name: "topic too long",
			mut:  func(m *outbox.Message) { m.Topic = outbox.Topic(strings.Repeat("a", outbox.MaxTopicLen+1)) },
			want: outbox.ErrTopicInvalid,
		},
		{
			// The offending character at index 0, deliberately. Every other
			// malformed fixture here puts it mid-string, where `i >= 0` and
			// `i > 0` are the same predicate — so without this case the
			// leading-character branch is unreachable while looking covered,
			// and " audit" validates. Two mutants survived on exactly this.
			name: "topic leading with a bad character",
			mut:  func(m *outbox.Message) { m.Topic = " audit" },
			want: outbox.ErrTopicInvalid,
		},
		{
			name: "key unset",
			mut:  func(m *outbox.Message) { m.Key = "" },
			want: outbox.ErrKeyUnset,
		},
		{
			name: "key with a newline",
			mut:  func(m *outbox.Message) { m.Key = "trade\n1" },
			want: outbox.ErrKeyInvalid,
		},
		{
			// The mirror of "topic leading with a bad character". Key runs the
			// same scan through the same helper, so it has the same blind spot
			// and gets the same fixture — applied here rather than left for the
			// next session to rediscover through the other door.
			name: "key leading with a bad character",
			mut:  func(m *outbox.Message) { m.Key = "\ttrade-1" },
			want: outbox.ErrKeyInvalid,
		},
		{
			name: "key too long",
			mut:  func(m *outbox.Message) { m.Key = outbox.Key(strings.Repeat("k", outbox.MaxKeyLen+1)) },
			want: outbox.ErrKeyInvalid,
		},
		{
			name: "type unset",
			mut:  func(m *outbox.Message) { m.Type = "" },
			want: outbox.ErrTypeUnset,
		},
		{
			name: "payload nil",
			mut:  func(m *outbox.Message) { m.Payload = nil },
			want: outbox.ErrPayloadEmpty,
		},
		{
			name: "payload empty but non-nil",
			mut:  func(m *outbox.Message) { m.Payload = []byte{} },
			want: outbox.ErrPayloadEmpty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validMessage()
			tc.mut(&m)

			err := m.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

// The message names the field, not just the class of fault. A sentinel says
// WHICH check fired; the message is what an operator reads at 3am, and §10h
// found four mutants surviving on tests that asserted only the sentinel.
func TestMessageValidateMessageNamesTheField(t *testing.T) {
	m := validMessage()
	m.Topic = "audit projection"

	err := m.Validate()
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "audit projection") {
		t.Fatalf("error must quote the offending value, got %q", err.Error())
	}
}

func TestRecordValidateAcceptsTheFixture(t *testing.T) {
	r := outbox.Record{Seq: 1, Message: validMessage()}
	if err := r.Validate(); err != nil {
		t.Fatalf("the shared valid record must validate, got %v", err)
	}
}

func TestRecordValidateRejectsUnassignedSeq(t *testing.T) {
	// Seq is assigned by the store on append and is the at-least-once DEDUP
	// key. A zero Seq means the record never committed, so a consumer
	// deduplicating on it would treat every unassigned record as the same one.
	r := outbox.Record{Seq: 0, Message: validMessage()}

	if err := r.Validate(); !errors.Is(err, outbox.ErrSeqUnset) {
		t.Fatalf("Validate() = %v, want %v", err, outbox.ErrSeqUnset)
	}
}

// Record.String must not print the payload, and the relay depends on it: a
// publish failure is formatted with %s, and that error travels to alerting. An
// outbox payload is an OMS event carrying sizes and account identifiers. Same
// ruling as §10f took for config's ErrNotReloadable — the claim is scoped, and
// what is excluded is stated rather than assumed.
func TestRecordStringOmitsThePayload(t *testing.T) {
	m := validMessage()
	m.Payload = []byte(`{"account":"sub-7","qty":"0.5"}`)
	r := outbox.Record{Seq: 42, Message: m, Attempts: 2}

	got := r.String()
	for _, secret := range []string{"sub-7", "0.5", "account"} {
		if strings.Contains(got, secret) {
			t.Fatalf("String() = %q, must not leak payload content %q", got, secret)
		}
	}
	// The positive half: it must still identify the record, or an operator
	// cannot act on the alert it appears in.
	for _, want := range []string{"42", "audit", "oms.FillApplied"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String() = %q, want it to name %q", got, want)
		}
	}
}

func TestRecordValidateChecksTheMessage(t *testing.T) {
	// A valid Seq must not launder an invalid message past the check.
	m := validMessage()
	m.Payload = nil
	r := outbox.Record{Seq: 7, Message: m}

	if err := r.Validate(); !errors.Is(err, outbox.ErrPayloadEmpty) {
		t.Fatalf("Validate() = %v, want %v", err, outbox.ErrPayloadEmpty)
	}
}
