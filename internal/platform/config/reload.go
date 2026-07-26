package config

import (
	"fmt"
	"reflect"
	"sort"
)

// diffPolicy compares the configuration in force against a candidate and answers
// two questions at once: may this change be applied at all, and which fields
// moved.
//
// The rule is default-deny. A field may differ only if it, or an ancestor of it,
// is tagged `config:"hot"`; the first field that differs without that permission
// returns ErrNotReloadable naming its path, and nothing is applied. On success
// the returned paths are the hot fields that actually changed, in a stable
// order, which is what §5.2's "every change is audit-logged" records.
//
// Paths are in the DOCUMENT's language — risk.liq_distance.rebalance_pct,
// venues["binance"].endpoint — because the person reading the error is holding
// the file, not the Go source. Values never appear; see the package doc.
//
// Composites recurse rather than compare wholesale, which matters for the shape
// §5.2 actually has. A per-venue map whose entries hold a hot rate-limit budget
// would otherwise compare unequal at the map and be refused, because the map
// itself carries no tag. So: entries present in both sides are compared
// element-wise, while ADDING or REMOVING an entry is a structural change to the
// map and needs the map to be hot. That asymmetry is deliberate — editing a
// venue's budget and introducing a venue are different acts, and only the first
// is one the running process can absorb without re-wiring anything.
func diffPolicy(prev, next reflect.Value) ([]string, error) {
	var changed []string
	if err := diff(prev, next, "", false, &changed); err != nil {
		return nil, err
	}
	sort.Strings(changed)
	return changed, nil
}

func diff(prev, next reflect.Value, path string, hot bool, changed *[]string) error {
	t := prev.Type()

	// A type that decodes itself is opaque: its fields are not config fields, so
	// it is compared as one leaf. Duration and SecretRef are exactly this.
	if decodesItself(t) {
		return leaf(prev, next, path, hot, changed)
	}

	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			fieldHot, err := hotTag(f)
			if err != nil {
				// auditSchema has already rejected this type, so reaching here
				// means diff ran on a schema that was never audited.
				return fmt.Errorf("%s: %w", join(path, f.Name), err)
			}
			name, err := jsonName(f)
			if err != nil {
				return fmt.Errorf("%w: %s: %w", ErrSchemaField, join(path, f.Name), err)
			}
			if err := diff(prev.Field(i), next.Field(i), join(path, name), hot || fieldHot, changed); err != nil {
				return err
			}
		}
		return nil

	case reflect.Pointer:
		// A pointer going nil↔non-nil is the field appearing or disappearing,
		// which is structural rather than a change of value.
		if prev.IsNil() != next.IsNil() {
			return structural(path, hot, changed)
		}
		if prev.IsNil() {
			return nil
		}
		return diff(prev.Elem(), next.Elem(), path, hot, changed)

	case reflect.Slice, reflect.Array:
		if prev.Len() != next.Len() {
			return structural(path, hot, changed)
		}
		for i := range prev.Len() {
			if err := diff(prev.Index(i), next.Index(i), fmt.Sprintf("%s[%d]", path, i), hot, changed); err != nil {
				return err
			}
		}
		return nil

	case reflect.Map:
		if prev.IsNil() != next.IsNil() {
			return structural(path, hot, changed)
		}
		keys, sameSet := pairedKeys(prev, next)
		if !sameSet {
			return structural(path, hot, changed)
		}
		for _, k := range keys {
			at := fmt.Sprintf("%s[%q]", path, fmt.Sprint(k.Interface()))
			if err := diff(prev.MapIndex(k), next.MapIndex(k), at, hot, changed); err != nil {
				return err
			}
		}
		return nil
	}

	return leaf(prev, next, path, hot, changed)
}

// leaf compares two values that have no further structure worth descending into.
func leaf(prev, next reflect.Value, path string, hot bool, changed *[]string) error {
	if reflect.DeepEqual(prev.Interface(), next.Interface()) {
		return nil
	}
	return record(path, hot, changed)
}

// structural handles a change to a composite's SHAPE — a differing length, key
// set, or nilness. There is nothing to descend into, so the whole composite is
// the unit of permission.
func structural(path string, hot bool, changed *[]string) error {
	return record(path, hot, changed)
}

func record(path string, hot bool, changed *[]string) error {
	if !hot {
		return fmt.Errorf("%w: %s (restart required)", ErrNotReloadable, pathOrRoot(path))
	}
	*changed = append(*changed, pathOrRoot(path))
	return nil
}

// pairedKeys returns prev's keys in a stable order, and reports whether the two
// maps have identical key sets. The sort is by the key's rendered form so the
// audit record and any error are deterministic regardless of Go's map ordering.
func pairedKeys(prev, next reflect.Value) ([]reflect.Value, bool) {
	if prev.Len() != next.Len() {
		return nil, false
	}
	keys := prev.MapKeys()
	for _, k := range keys {
		if !next.MapIndex(k).IsValid() {
			return nil, false
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
	})
	return keys, true
}
