package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Decode strictly decodes a configuration document into T and validates it.
//
// It is the whole load path minus the revision bookkeeping, exported for the
// callers that want to check a document without holding one — a boot preflight,
// a configcheck command, a test. Store.New and Store.Reload run exactly this.
//
// Four things are checked before T's own Validate ever runs, in this order:
//
//  1. the schema type is loadable at all (see auditSchema);
//  2. no object in the document repeats a key;
//  3. the document decodes with no unknown fields;
//  4. nothing follows the document's first JSON value.
//
// The order matters for the message a human gets. Checking the schema first
// means a programmer's mistake is not reported as though the operator's file
// were at fault, and the duplicate-key scan runs before decoding because the
// decoder would silently resolve the duplicate and hand back a value that
// validates.
func Decode[T Validator](data []byte) (T, error) {
	var zero, v T

	if err := auditSchema[T](); err != nil {
		return zero, err
	}
	if err := decodeStrict(data, &v); err != nil {
		return zero, err
	}
	if err := v.Validate(); err != nil {
		return zero, err
	}
	return v, nil
}

// Validator is what a configuration schema must implement: §5.2's "typed struct
// with Validate()".
//
// The method is on the VALUE, matching the rest of the kernel — Validate answers
// a question about a value and has no business mutating one. A schema with a
// pointer-receiver Validate does not satisfy this constraint, which is a
// compile-time error at the call site rather than a surprise at boot.
//
// Every §11 "the zero value is unusable" rule applies here with the most force
// in the repo: an unpopulated risk cap reads as zero, and a zero cap is either
// "block everything" or "permit everything" depending on which side of the
// comparison it lands on — one of which is a gate that never fires.
type Validator interface {
	Validate() error
}

// decodeStrict decodes into dst with every default-deny check the package doc
// lists. dst must be a non-nil pointer.
func decodeStrict(data []byte, dst any) error {
	// There is deliberately no "is this whitespace?" pre-check here. One was
	// written and a surviving mutant proved it did nothing: json.Decode returns
	// io.EOF for an empty or all-whitespace document, and the branch below maps
	// that to ErrEmptyDocument already. It was worse than redundant — a document
	// of one non-breaking space is not empty, and bytes.TrimSpace trims Unicode
	// whitespace that JSON does not, so the pre-check reported "empty document"
	// where the decoder would have named the invalid character.
	if err := checkDuplicateKeys(data); err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		// io.EOF from Decode means the document held no JSON value at all —
		// empty, or nothing but whitespace. That is what a truncated write
		// leaves behind, and "unexpected end of JSON input" reads like a typo
		// rather than like a half-written file, so it is named for what it is.
		if errors.Is(err, io.EOF) {
			return ErrEmptyDocument
		}
		return fmt.Errorf("config: decode: %w", err)
	}

	// json.Decoder reads ONE value and stops. Without this, a file holding two
	// objects — an editor's failed merge, an appended block — loads the first
	// and never mentions the second.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("config: %w: %w", ErrTrailingData, err)
		}
		return ErrTrailingData
	}
	return nil
}

// checkDuplicateKeys reports the first object key that appears twice.
//
// This exists because Go's decoder does not. Given {"cap": 100, "cap": 999999}
// it decodes 999999 and reports nothing, and DisallowUnknownFields does not help
// because both occurrences are known fields. The failure mode is the worst shape
// a config bug can take: the document looks right to whoever reads it, one of
// the two values is silently in force, and which one depends on file order.
//
// The scan is over the token stream rather than over a map, because decoding
// into map[string]any is exactly the operation that loses the duplicate.
func checkDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // never construct a float64 from a config number (§3.6 rule 4)
	err := scanValue(dec, "")
	if errors.Is(err, io.EOF) {
		// A truncated document. The real decode reports it with a better
		// message, so say nothing here and let it.
		return nil
	}
	if err != nil && !errors.Is(err, ErrDuplicateKey) {
		// Any other malformed input: same reasoning.
		return nil
	}
	return err
}

// scanValue consumes exactly one JSON value from dec, recursing through objects
// and arrays. path is the dotted location of that value, used only to name a
// duplicate.
func scanValue(dec *json.Decoder, path string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return nil // a scalar: nothing nested to check
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyTok.(string)
			if !ok {
				// Unreachable for well-formed JSON — an object key is always a
				// string — but the decoder is a state machine and this is the
				// branch that would loop forever if it were not handled.
				return fmt.Errorf("config: object key is not a string at %q", path)
			}
			at := join(path, key)
			if _, dup := seen[key]; dup {
				return fmt.Errorf("%w: %s", ErrDuplicateKey, at)
			}
			seen[key] = struct{}{}
			if err := scanValue(dec, at); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			if err := scanValue(dec, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}

	// Consume the closing delimiter, so the caller resumes at the next value.
	_, err = dec.Token()
	return err
}

// join builds a dotted path. The empty parent is the document root, whose
// children are named without a leading dot.
func join(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}
