package reconcile

import "errors"

// This root's own refusals.
//
// They are the ones no constructor beneath it can make, which is the test for
// whether a check belongs here at all: everything about ONE field's shape is
// refused by the type or the package that owns it, and what is left is the
// agreements BETWEEN fields and the facts about this HOST.
var (
	// ErrConfigFieldUnset and ErrConfigFieldInvalid report a field the schema
	// requires and a value it cannot use.
	//
	// Two sentinels rather than one because they tell an operator different
	// things: the first says a line is missing from the file, the second that a
	// line is there and wrong. The message names the field either way — this
	// repo's four-checks-one-sentinel lesson from platform/config, where a
	// fixture failing the check AFTER the one under test looked like a pass.
	ErrConfigFieldUnset   = errors.New("reconcile: required configuration field unset")
	ErrConfigFieldInvalid = errors.New("reconcile: configuration field invalid")

	// ErrConfigDuplicate reports a list declaring the same key twice — two
	// endpoints for one family, two scopes for one family, one instrument twice.
	//
	// Refused rather than last-wins, because last-wins over a JSON array is a
	// silent choice between two operator intentions, and the two entries are
	// usually a copy-paste with one field edited.
	ErrConfigDuplicate = errors.New("reconcile: configuration declares the same key twice")

	// ErrDialectUnknown reports a venue dialect this build has no collaborators
	// for. See Dialect for why one name selects four of them.
	ErrDialectUnknown = errors.New("reconcile: unknown venue dialect")

	// ErrVenueClientMissing reports a phase asked to read the venue on a boot
	// whose venue client was never built (record §73n).
	//
	// It exists because the nil is invisible where it would otherwise be caught:
	// a nil *venue.OrderClient satisfies a consumer-declared interface as a
	// NON-nil value holding a nil pointer, so the adapter's own nil check passes
	// it through to a dereference. Named here, at the composition root that
	// knows the concrete type, rather than defended for a second time downstream.
	ErrVenueClientMissing = errors.New("reconcile: no venue client was built for this boot")

	// ErrVenueMismatch reports an instrument or a scope naming a venue other than
	// the configured one.
	//
	// This deployment holds ONE venue client, so a row naming another venue
	// describes exposure nothing here can read — and the failure it produces is
	// not an error at the call site. driven.VenuePositions derives its own venue
	// from the instrument table, so a mixed table is refused there, and a scope
	// for an unwired venue is answered by the wrong exchange or by nothing at
	// all. Both are cheaper to refuse in the document that wrote them.
	ErrVenueMismatch = errors.New("reconcile: instrument or scope names a venue this deployment has no client for")

	// ErrFamilyUnrouted reports an instrument whose product family has no
	// endpoint. On Binance the families are separate hosts, so an unrouted family
	// is not a missing base URL — it is a query that cannot be issued at all.
	ErrFamilyUnrouted = errors.New("reconcile: no endpoint serves this instrument's product family")

	// ErrPostureSettingUnprobed reports a §4.1 rule with a venue read path that
	// no posture probe answers.
	//
	// It is a CODE defect and never an operator one, which is why it is its own
	// sentinel rather than a shape of ErrFamilyUnrouted: no document can cause
	// it and no document can fix it. The failure it prevents is the expensive
	// kind — the rule refuses every boot with ReasonUnobserved, a permanent halt
	// whose cause is invisible to either half's own tests, because the rule table
	// and the codec's step list are each internally consistent (task #199,
	// record §73p; the shape is #186's fold-name check).
	ErrPostureSettingUnprobed = errors.New("reconcile: a §4.1 rule has no posture probe to answer it")

	// ErrScopeNotCovered reports a DERIVATIVE instrument that no position scope
	// covers.
	//
	// It is the configuration half of slice G2's finding. The use case reports the
	// same condition as application.UnassertedScopeNotEnumerated and holds the
	// boot posture for it, which is the right answer at run time — but the run
	// time answer is a latching refusal an operator has to read, and this one is a
	// message naming the instrument and the missing field before the process has
	// opened a journal. The two are not the same check twice: this one is about
	// the CONFIGURED universe, and the use case's is about the JOURNAL's, which
	// outlives any one config file and can name an instrument this document
	// dropped.
	ErrScopeNotCovered = errors.New("reconcile: derivative instrument is covered by no position scope")

	// ErrLandingBoundTooShort reports §5.3's propagation bound at or below the
	// transport bounds it must dominate.
	ErrLandingBoundTooShort = errors.New("reconcile: max_landing does not exceed the transport bounds beneath it")

	// ErrSecretSchemeUnknown, ErrSecretUnresolved and ErrSecretEmpty report the
	// three ways a key ref fails to produce a credential.
	//
	// Three rather than one, and the third is the one that earns its place: an
	// environment variable that is SET AND EMPTY resolves without error, and the
	// credential it would build signs correctly over an empty key. What the venue
	// then returns is an authentication failure, which reads to an operator as a
	// revoked or wrong key rather than as an unset variable on this host.
	ErrSecretSchemeUnknown = errors.New("reconcile: secret reference names an unsupported scheme")
	ErrSecretUnresolved    = errors.New("reconcile: secret reference could not be resolved")
	ErrSecretEmpty         = errors.New("reconcile: secret reference resolved to an empty value")

	// ErrJournalStorageUnsupported reports a host this build cannot decide the
	// filesystem question on.
	//
	// It is a REFUSAL rather than a pass, and that is #168's whole content: SQLite
	// documents NFS and network filesystems as corruption-prone, and a journal on
	// one is a journal whose hash chain still verifies while records that were
	// never seen are missing — §7.5's own stated limit of the tamper-evidence,
	// made real. A build that cannot tell must not report that it did.
	ErrJournalStorageUnsupported = errors.New("reconcile: this build cannot verify the journal's filesystem")

	// ErrJournalStorageRemote reports a journal on a filesystem §13 forbids.
	ErrJournalStorageRemote = errors.New("reconcile: journal is on a network filesystem, which §13 forbids")

	// ErrJournalStorageVolatile reports a journal on a LOCAL filesystem that does
	// not survive a reboot — tmpfs, ramfs.
	//
	// A separate sentinel from the network one because §13's words admit it and
	// its harm is the opposite shape: nothing is corrupt, the fsyncs are real, the
	// chain verifies, and the whole log is simply gone at the next boot — which is
	// the boot §7.5 exists for. Reported as its own refusal so an operator reading
	// "network filesystem" about /dev/shm does not conclude the check is confused.
	ErrJournalStorageVolatile = errors.New("reconcile: journal is on a volatile filesystem that does not survive a reboot")

	// ErrJournalStorageUnknown reports a filesystem this build does not recognise
	// at all.
	//
	// Refused rather than admitted, on the standing default-deny rule and on the
	// asymmetry between the two mistakes. Admitting an unknown filesystem is how
	// NFS arrives under a name this table has not seen — a new version, a FUSE
	// client, a vendor's own driver — and it arrives silently. Refusing a
	// legitimate local filesystem produces a boot that will not start, naming the
	// filesystem, which an operator fixes in one edit.
	ErrJournalStorageUnknown = errors.New("reconcile: journal is on a filesystem this build does not recognise")
)
