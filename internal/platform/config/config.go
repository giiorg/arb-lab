// Package config is build-spec §5.2's typed configuration machinery: strict
// load, Validate(), safe hot-reload, and the revision identity §3.5 replays
// against.
//
// # What this package owns, and what it does not
//
// It owns the MECHANISM. It does not own §5.2's schema. There is no
// config.Config here holding risk caps and venue endpoints and strategy params;
// those structs belong to the contexts that own their meaning — risk's caps in
// risk, a venue's endpoints in engine/venue, a strategy's params in strategy —
// and app/ composes them. §11 makes the kernel a shared kernel rather than a
// place domain vocabulary accumulates, and the §3.2 table's entry for this
// package is a list of verbs: "typed load + Validate() + safe hot-reload;
// revisions recorded for replay".
//
// The practical test of that split: adding a risk cap should touch risk. If the
// cap lived here, every context's parameters would be a kernel edit, and risk
// would import the definition of its own caps from a package that cannot import
// risk back.
//
// # Four things a config file does silently, and what happens to them here
//
// Configuration is read once, at boot, by nobody, and a mistake in it does not
// crash — it computes a different number. Each of these is default-deny.
//
//	unknown key         REJECTED. json ignores it by default, so a typo'd
//	                    per_trade_notionl leaves the real cap at its zero value
//	                    and nothing reports anything.
//	duplicate key       REJECTED. json takes the LAST silently, and
//	                    DisallowUnknownFields does not see it. Only a
//	                    token-level scan does, which is why decode makes one.
//	trailing data       REJECTED. json.Decode reads one value and stops, so a
//	                    file with a second appended object loads the first.
//	untagged field      REJECTED at New. Every exported field must carry an
//	                    explicit json name, so the wire contract is a decision
//	                    rather than a consequence of a Go identifier.
//
// # Hot-reload is default-deny
//
// §5.2 says config is "hot-reloadable where safe". Safe is not a property of
// configuration in general; it is a property of each field. A field may change
// at runtime only if it is tagged `config:"hot"`, and Reload rejects a change to
// anything else, naming the field. The default is the safe one: a field added to
// a schema later is restart-required until somebody decides otherwise, in the
// same way nofloat denies float64 until somebody writes a reason.
//
// Tagging a struct field hot makes its whole subtree hot. An unrecognised tag
// value is an error rather than a silent deny — failing safe silently would
// leave an operator believing a field reloads when it does not.
//
// # The reload policy and the document checks name paths, never values
//
// ErrNotReloadable, ErrDuplicateKey, ErrTrailingData and every audit record name
// a PATH — risk.max_open, venues["binance"].endpoint — and never the value at
// it. An error travels to logs and alerting, and a config struct is exactly
// where a credential ends up when somebody declined to use SecretRef. Paths are
// enough to act on; a diff of the file is one command away on the host that
// already has both versions. ParseSecretRef holds the same line for the same
// reason: an inline credential is precisely the input that fails it, so its
// message describes the expected form rather than echoing what it got.
//
// The claim is scoped, and the exclusions are deliberate rather than overlooked.
// A Duration that fails to parse echoes the text that failed, because that text
// is the operator's own typo and never became configuration. A decoder syntax
// error can name the character it stopped on. And a schema's own Validate is
// outside this package entirely — a schema that puts a credential in a plain
// string field and prints it has defeated SecretRef, which is why SecretRef
// exists.
//
// # What is deliberately NOT here
//
// The FILE WATCHER is not. Deciding when to re-read bytes is a composition-root
// concern with its own failure semantics (a truncated write, an editor's
// atomic-rename dance, a debounce window), and it needs a Clock. Store.Reload
// takes bytes and an instant; who produces them is app/'s problem.
//
// SECRET RESOLUTION is not. SecretRef names a credential and cannot hold one —
// see secret.go. A kernel type that could resolve is a kernel type that could
// log a resolved value, and this package's one invariant is that it prints no
// values at all.
//
// The REVISION LOG is not. §3.5 requires config revisions be recorded so a
// backtest can replay which configuration was in force at each IngressSeq. This
// package mints the identity — a content hash plus a monotonic number — and
// records nothing. Persistence is the journal's job, and the Revision is what it
// persists.
//
// The APPROVAL KERNEL is not. §5.2 makes emergency-policy edits pass through
// platform/foureyes before taking effect; that is a different gate at a
// different layer, and this package would happily reload a policy struct that
// had never been approved. Wiring the two is app/operator's job.
//
// # One sharp edge, stated rather than hidden
//
// Current() returns a copy of the snapshot struct, so a caller cannot swap the
// Store's value. It does NOT deep-copy: if a schema holds a map or a slice, the
// copy shares that backing store with every other reader. Treat a loaded
// configuration as immutable. Deep-copying on every Current() would put an
// allocation on a path meant to be a pointer load, and a schema built from
// scalars — which §5.2's are — has nothing to share.
//
// # No clock
//
// Load and Reload take the instant as an argument rather than reading one.
// notimenow (§3.6 rule 5) does not police platform/, so this is discipline
// rather than enforcement — but a Revision's LoadedAt is replayed, and a
// timestamp a backtest cannot control is a timestamp that makes a replay differ
// from the run it replays.
package config

import "errors"

// Sentinel errors. Callers branch on these with errors.Is, so they are part of
// the package's public contract.
//
// They divide into three groups by who has to fix what. The DOCUMENT errors
// (ErrEmptyDocument…ErrDuplicateKey) name a defect in the bytes — an operator
// edited a file wrongly. The SCHEMA errors (ErrSchemaNotStruct…ErrTagUnknown)
// name a defect in the Go type — a programmer wrote a schema this package
// refuses to load, and the failure is the same on every input. ErrNotReloadable
// is neither: both sides are well-formed and the operator asked for a change
// that requires a restart.
var (
	// ErrEmptyDocument reports configuration bytes with no JSON value in them.
	// Distinguished from a syntax error because an empty file is what a
	// truncated write leaves behind, and "unexpected end of JSON input" reads
	// like a typo rather than like a half-written file.
	ErrEmptyDocument = errors.New("config: empty document")

	// ErrTrailingData reports a document with content after its first JSON
	// value. json.Decoder reads one value and leaves the rest, so without this
	// check a file holding two objects loads the first and never mentions the
	// second.
	ErrTrailingData = errors.New("config: trailing data after the configuration document")

	// ErrDuplicateKey reports the same key twice in one JSON object. Go's
	// decoder takes the last occurrence and reports nothing, and
	// DisallowUnknownFields does not see it because both occurrences are known
	// keys. The wrapped message names the path, not the values.
	ErrDuplicateKey = errors.New("config: duplicate key")

	// ErrSchemaNotStruct reports a schema type that is not a struct. A
	// configuration document is an object; a schema that is a map or a slice
	// has no field names for the reload policy to be expressed against.
	ErrSchemaNotStruct = errors.New("config: schema is not a struct")

	// ErrSchemaField reports a schema field this package refuses to load. The
	// wrapped message names the path and the reason: an unexported field
	// (decode can never populate it, so it is permanently zero), an embedded
	// field (deferred — the tag and path rules are not defined for one), a
	// missing or empty json tag, a json:"-" field, or a kind that cannot
	// survive the trip: float (§3.6 rule 4), interface (a JSON number decodes
	// into one as a float64, silently), chan, func, complex, unsafe.Pointer.
	ErrSchemaField = errors.New("config: unloadable schema field")

	// ErrTagUnknown reports a config tag whose value is not one this package
	// recognises. Deliberately an error rather than a silent deny: denying
	// reload is the SAFE direction, so a typo'd `config:"hto"` would fail safe
	// and leave an operator believing a field hot-reloads when it does not.
	ErrTagUnknown = errors.New("config: unknown config tag value")

	// ErrNotReloadable reports a Reload whose bytes change a field that is not
	// tagged hot. The wrapped message names the field's path. Nothing is
	// swapped: the store keeps the configuration it was already serving.
	ErrNotReloadable = errors.New("config: field is not hot-reloadable")

	// ErrRevisionInvalid reports a Revision that never identified anything — an
	// unset hash, an unset source, a zero instant, or revision number zero.
	// Revisions are numbered from 1 precisely so that the zero value fails.
	ErrRevisionInvalid = errors.New("config: invalid revision")

	// ErrSourceUnset reports a load with no source name. The source is what an
	// audit record and a replay use to say WHICH configuration this was, and
	// "revision 4" without it names a number and no document.
	ErrSourceUnset = errors.New("config: source name unset")

	// ErrInstantUnset reports a load with the zero time.Time. The instant is a
	// replayed field (§3.5); the zero value means nobody supplied a clock
	// reading, which is not the same as "the epoch".
	ErrInstantUnset = errors.New("config: load instant unset")

	// ErrSecretRefMalformed reports a secret reference that is not
	// "<scheme>:<name>" with a recognised scheme. An inline credential fails
	// this check, which is the point: §5.2 says key refs, never inline secrets,
	// and a bare secret has no scheme.
	ErrSecretRefMalformed = errors.New("config: malformed secret reference")

	// ErrDurationMalformed reports a duration that is not a JSON string in
	// time.ParseDuration's syntax. A bare number is rejected rather than read
	// as nanoseconds: "300" in a config file means five minutes to whoever
	// typed it and 300ns to Go, and one of those is a stale-mark budget that
	// expires before it is checked.
	ErrDurationMalformed = errors.New("config: malformed duration")

	// ErrDurationNegative reports a negative duration. Every §5.2 duration is a
	// budget, a half-life or a dwell, and a negative one of any of those is a
	// timer that has already expired.
	ErrDurationNegative = errors.New("config: negative duration")
)
