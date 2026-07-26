package config

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot is one configuration and the revision that identifies it. It is a
// value: two goroutines holding snapshots taken either side of a reload hold
// different configurations, and neither changes underneath the other.
type Snapshot[T Validator] struct {
	Value T
	Rev   Revision
}

// Change is the audit record §5.2 requires of "every change".
//
// Fields names the hot paths that moved, in the document's language and in a
// stable order. It is empty when Applied is false, and it can also be empty on
// an applied change — a reformatted file whose decoded value is identical is a
// new revision with no field changes, which is the honest record of what
// happened: somebody edited the file and nothing about the configuration moved.
type Change struct {
	Prev, Next Revision
	Fields     []string

	// Applied is false only for the no-op case: bytes identical to the ones
	// already in force. Nothing is swapped, no revision is minted, and Prev
	// equals Next. A file watcher firing twice on one write must not churn the
	// revision log, and a revision number is a replay coordinate — spending one
	// on an event that changed nothing makes the log lie about how many times
	// the operator acted.
	Applied bool
}

// Store holds the configuration currently in force and mints revisions as it
// changes.
//
// Reads are lock-free: Current is an atomic pointer load, which is what makes it
// callable from a hot path — risk.Gate reads caps per decision (§6.3), and a
// mutex there would put every trading decision behind the operator's config
// editor.
//
// Writes are serialised. Reload validates, checks the reload policy, and only
// then swaps; a rejected reload leaves the previous configuration in force,
// untouched. That ordering is the whole safety property of hot-reload: the
// process is never briefly running a configuration that failed its own checks.
type Store[T Validator] struct {
	mu  sync.Mutex // serialises Reload; Current never takes it
	cur atomic.Pointer[Snapshot[T]]
}

// New loads the first configuration and returns a Store serving it at revision 1.
//
// source names the document for audit and replay; at is the instant it took
// effect, supplied by the caller from a Clock. Both are required — see
// ErrSourceUnset and ErrInstantUnset for why the zero values are refused rather
// than defaulted.
//
// A failure here is a failure to boot. There is no partial success: the Store is
// nil and nothing is serving.
func New[T Validator](source string, at time.Time, data []byte) (*Store[T], error) {
	snap, err := load[T](source, at, data, 1)
	if err != nil {
		return nil, err
	}
	s := &Store[T]{}
	s.cur.Store(&snap)
	return s, nil
}

// Current is the configuration in force. Safe for concurrent use, and cheap
// enough to call per decision rather than caching.
//
// The returned Snapshot is a copy, so a caller cannot swap what the Store
// serves. It is not a DEEP copy — see the package doc's sharp edge — so a schema
// holding a map or a slice shares that backing store with every other reader.
func (s *Store[T]) Current() Snapshot[T] { return *s.cur.Load() }

// Reload applies a new configuration document, or refuses it and changes
// nothing.
//
// The sequence, and each step's reason for being where it is:
//
//  1. hash the bytes. Identical to what is in force → a no-op Change, so a
//     watcher firing twice on one write does not spend a revision number.
//  2. decode strictly and Validate. A document that cannot be trusted must not
//     reach the policy check, where it would produce a diff against garbage.
//  3. check the reload policy against the configuration in force. First
//     untagged field that moved → ErrNotReloadable, nothing swapped.
//  4. swap atomically and mint the next revision.
//
// Errors from steps 2 and 3 are distinguishable by the caller: a Validate
// failure is a bad document, ErrNotReloadable is a good document asking for
// something that needs a restart. An operator can act on the difference.
func (s *Store[T]) Reload(source string, at time.Time, data []byte) (Change, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := *s.cur.Load()

	if hashOf(data) == prev.Rev.Hash {
		return Change{Prev: prev.Rev, Next: prev.Rev, Applied: false}, nil
	}

	next, err := load[T](source, at, data, prev.Rev.Num+1)
	if err != nil {
		return Change{}, err
	}

	changed, err := diffPolicy(reflect.ValueOf(prev.Value), reflect.ValueOf(next.Value))
	if err != nil {
		return Change{}, err
	}

	s.cur.Store(&next)
	return Change{Prev: prev.Rev, Next: next.Rev, Fields: changed, Applied: true}, nil
}

// load is the shared body of New and Reload: the checked decode plus the
// revision that identifies what was decoded.
func load[T Validator](source string, at time.Time, data []byte, num uint64) (Snapshot[T], error) {
	var zero Snapshot[T]

	if source == "" {
		return zero, ErrSourceUnset
	}
	if at.IsZero() {
		return zero, ErrInstantUnset
	}

	v, err := Decode[T](data)
	if err != nil {
		return zero, err
	}
	return Snapshot[T]{
		Value: v,
		Rev: Revision{
			Num:      num,
			Hash:     hashOf(data),
			Source:   source,
			LoadedAt: at,
		},
	}, nil
}
