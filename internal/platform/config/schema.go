package config

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// The reload policy tag. A field carrying it may change across a Reload; a field
// without it may not. There is exactly one recognised value, and an unrecognised
// one is an error — see ErrTagUnknown for why silence would be worse here than
// elsewhere.
//
// [SPEC EDIT LANDED — v0.9.6] §5.2 said configuration is "hot-reloadable where
// safe" and stopped there. "Where safe" has to be decided per field by somebody,
// and the two ways to decide it are not equally wrong when forgotten:
// default-ALLOW means a field added next year is silently hot until somebody
// notices, and the noticing happens when a running process picks up a journal
// path or a fencing generation mid-trade. Default-DENY means it is
// restart-required until somebody writes the tag, and the noticing happens when
// a reload is refused with the field named. This is the same argument §3.6
// rule 4 already settled for float64 — deny until a reason is written down —
// applied to a requirement the spec had left open rather than to one it decided
// differently. §5.2 now says so, including this file's one deliberate
// asymmetry below.
const (
	tagKey = "config"
	tagHot = "hot"
)

// schemaCache memoises auditSchema per type. The audit is a pure function of the
// type, it runs on every load and reload, and its answer cannot change while the
// process lives.
var schemaCache sync.Map // reflect.Type -> error

// auditSchema reports whether T can be loaded at all.
//
// This is a check on the PROGRAMMER's work rather than the operator's, and it
// fails identically on every input, which is why it runs first and why its
// errors are a separate group. What it rules out:
//
//	not a struct        a document is an object; there are no field names for a
//	                    reload policy to name.
//	unexported field    decode cannot populate one, so it is permanently zero —
//	                    and a permanently-zero cap is §11's unusable zero value
//	                    made unreachable rather than made loud.
//	embedded field      deferred. The json name of an embedded struct's fields
//	                    is promoted, so neither the path naming nor the hot tag
//	                    has a defined meaning here yet. Refused rather than
//	                    half-supported.
//	no json tag         the wire name would fall back to the Go identifier,
//	                    matched case-insensitively. Renaming a field would then
//	                    rename a config key, and the wire contract would be a
//	                    consequence of a refactor rather than a decision.
//	json:"-"            a field the document can never set, so always zero.
//	float / interface   §3.6 rule 4. The interface case is the subtle one: a
//	                    JSON number decoded into `any` becomes a float64, and
//	                    nofloat cannot see it because no float64 appears in the
//	                    source.
//	chan/func/complex   not representable in JSON; a field that can never be
//	                    populated, wearing a type that suggests otherwise.
//
// Recursion stops at any type that decodes itself — a json.Unmarshaler or an
// encoding.TextUnmarshaler. Those own their wire format, so their fields are not
// config fields; SecretRef and Duration below are exactly that case, and so is
// time.Time.
func auditSchema[T any]() error {
	t := reflect.TypeFor[T]()
	if cached, ok := schemaCache.Load(t); ok {
		if cached == nil {
			return nil
		}
		return cached.(error)
	}
	err := auditType(t, "", make(map[reflect.Type]bool))
	if err != nil {
		schemaCache.Store(t, err)
	} else {
		schemaCache.Store(t, nil)
	}
	return err
}

var (
	jsonUnmarshaler = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// decodesItself reports whether t (or *t, since a decode target is addressable)
// takes charge of its own wire format.
func decodesItself(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(jsonUnmarshaler) || pt.Implements(jsonUnmarshaler) ||
		t.Implements(textUnmarshaler) || pt.Implements(textUnmarshaler)
}

func auditType(t reflect.Type, path string, seen map[reflect.Type]bool) error {
	if path == "" && t.Kind() != reflect.Struct {
		return fmt.Errorf("%w: %s", ErrSchemaNotStruct, t)
	}
	if decodesItself(t) {
		return nil
	}
	if seen[t] {
		return nil // a self-referential type through a pointer; already audited
	}
	seen[t] = true
	defer delete(seen, t)

	switch t.Kind() {
	case reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fmt.Errorf("%w: %s: %s cannot be a configuration value", ErrSchemaField, pathOrRoot(path), t.Kind())

	case reflect.Pointer, reflect.Slice, reflect.Array:
		return auditType(t.Elem(), path+"[]", seen)

	case reflect.Map:
		if !validMapKey(t.Key()) {
			return fmt.Errorf("%w: %s: map key %s is not a JSON object key", ErrSchemaField, pathOrRoot(path), t.Key())
		}
		return auditType(t.Elem(), path+"[]", seen)

	case reflect.Struct:
		return auditStruct(t, path, seen)
	}
	return nil
}

func auditStruct(t reflect.Type, path string, seen map[reflect.Type]bool) error {
	for i := range t.NumField() {
		f := t.Field(i)
		at := join(path, f.Name) // Go name here: these errors address a programmer

		if f.Anonymous {
			return fmt.Errorf("%w: %s: embedded fields are not supported", ErrSchemaField, at)
		}
		if !f.IsExported() {
			return fmt.Errorf("%w: %s: unexported, so decode can never populate it", ErrSchemaField, at)
		}

		name, err := jsonName(f)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrSchemaField, at, err)
		}
		if _, err := hotTag(f); err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}

		if err := auditType(f.Type, join(path, name), seen); err != nil {
			return err
		}
	}
	return nil
}

// validMapKey reports whether k can be a JSON object key. encoding/json accepts
// strings, integer kinds, and anything implementing encoding.TextMarshaler.
func validMapKey(k reflect.Type) bool {
	switch k.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return k.Implements(textUnmarshaler) || reflect.PointerTo(k).Implements(textUnmarshaler)
}

// jsonName is the wire name of a field, which this package requires be written
// down rather than derived. The options after the comma (omitempty, string) are
// the decoder's business and are not inspected.
func jsonName(f reflect.StructField) (string, error) {
	tag, ok := f.Tag.Lookup("json")
	if !ok {
		return "", fmt.Errorf("no json tag: the wire name must be explicit, not derived from the Go identifier")
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", fmt.Errorf(`json:"-": a field the document can never set is permanently zero`)
	}
	if name == "" {
		return "", fmt.Errorf("empty json name")
	}
	return name, nil
}

// hotTag reports whether the field is tagged hot-reloadable, and rejects any
// other value of the tag.
func hotTag(f reflect.StructField) (bool, error) {
	tag, ok := f.Tag.Lookup(tagKey)
	if !ok {
		return false, nil
	}
	if tag != tagHot {
		return false, fmt.Errorf("%w: %q (the only recognised value is %q)", ErrTagUnknown, tag, tagHot)
	}
	return true, nil
}

// pathOrRoot names the document root for the empty path, so an error about the
// schema type itself does not read as an error about a field with no name.
func pathOrRoot(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
