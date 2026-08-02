// Package oms is §3.2's order-management bounded context, and this file is its
// composition seam: the one place that knows which adapter satisfies which
// port.
//
// # Why a seam here rather than wiring in app/
//
// §3.6 rule 7 gives every context a module.go and has the binary wire modules by
// hand. internal/marketdata/module.go states the argument for the split in full
// — app/ owns what varies BETWEEN binaries, this owns which adapter implements
// this context's ports — and it is not restated here.
//
// What is different here is that the split has a gate behind it. singlewriter
// polices application/ports and infrastructure/driven against every importer
// including app/, so "the context wires its own adapters" is no longer only a
// convention about where wiring reads best: for this context it is the only
// shape a root can express. §26e records the decision and its failure direction.
//
// # Scope: the write path is assembled, and the hole this file used to state is
// # closed
//
// Until slice B this file recorded an absence: driven.MemoryOrderWriteRepository
// was not constructed, because §6's write path is journal-then-apply and the
// apply half had no caller — ApplyPlanned belonged to §6 step 5's planning use
// case, which did not exist, and Dispatch took a domain.Order carrying no OCC
// version, so wiring a repository into it would have meant inventing the
// planning path to have produced a record for it to advance.
//
// That is what slice B built, and the two halves are now wired to ONE
// repository instance: application.Planner writes the record §6 step 5 journals,
// and application.Dispatcher reads its order back out and advances it. The
// shared instance is the point rather than an implementation detail — two would
// make Plan and Dispatch disagree about which orders exist, and Dispatch's
// refusal of an unplanned order would refuse everything.
//
// What is still NOT wired, stated so the absence stays visible: ApplyFill has no
// caller. §6's private order and execution stream handlers are slices C and D,
// and §5.3's one-transaction requirement for the fill path needs a transaction
// seam no port here declares.
//
// # What Module must NOT hand out, which is the decision this file exists for
//
// Exposing the repository — Module.Orders, typed ports.OrderWriteRepository —
// is the shape that would make the wiring look complete, and it is the one
// shape §3.6 rule 6 cannot survive. singlewriter reads DECLARED IMPORT EDGES,
// and Go's interfaces are structural: a root that receives such a field can
// hand it to any other context's constructor, and the receiving context needs
// no import at all to accept it. Every diagnostic the gate would have produced
// disappears, and what is left is a second writer to order state with a clean
// lint run — precisely the outcome §26e reasoned the gate's scope backwards
// from.
//
// So the write authority stays inside this package tree, and the rule that
// follows from it is worth stating in the general form: when the repository
// does get wired, it is wired INTO a use case here, and the use case is what
// Module exposes. A field of a policed type on this struct is never the answer.
package oms

import (
	"errors"
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
)

// Deps is what a composition root supplies.
//
// # One port and one piece of configuration, which is the shape the gate forced
//
// This struct used to hold two ports, and its own doc predicted the change that
// has now happened: "Journal's adapter is this context's own … it is a Deps
// field because it has not been written yet, not because a root is the right
// owner of it. When it lands, singlewriter FORCES the correction rather than
// merely inviting it."
//
// Slice A landed driven.OpenSQLiteJournal inside the policed package, and the
// prediction went live immediately: with the adapter unnameable from outside
// internal/oms, the ONLY way a root could still have filled a
// `Journal ports.Journal` field was by writing its own journal implementation —
// a second write path to order state, which is the outcome §3.6 rule 6 exists to
// prevent. A field whose sole remaining legal value is a violation is not a
// seam; so the field is configuration now and this package constructs the
// adapter. §26e point 3's "move the type rather than widen the gate", arriving
// as scheduled.
//
// Placer's adapter is a venue client. Venue clients live outside this context
// entirely (§20a defers them), which is marketdata.Deps.Resnapshotter's
// situation exactly, so that field is permanent: a root supplies it or the
// context does not build.
//
// # Neither field is refused HERE
//
// A root passing a nil placer gets application.ErrPlacerMissing from the
// constructor that owns the wire; an unset path gets driven.ErrJournalPathUnset
// from the adapter that owns the file — see New.
type Deps struct {
	// JournalPath is where §5.1's durable, append-only, hash-chained
	// write-ahead journal lives on this host.
	//
	// # A path rather than a ports.Journal, and what the root gave up
	//
	// What a root can no longer do is supply an implementation — including a
	// fast one that batches, which is what the port's durability contract
	// forbids in prose and could not enforce. The failure that shape invited is
	// the one nothing reports: no test fails, and the defect surfaces once,
	// during the crash the fsync existed for, when recovery finds no record of
	// an order the venue is holding. Configuration cannot express it.
	//
	// What a root retains is the decision that is genuinely deployment's:
	// WHERE. §11 makes this journal host-local and lost on host death, and §13
	// adds a precondition this constructor does not yet check — local
	// filesystem only, never NFS/network FS — which is scheduled to §7.5's boot
	// posture list (task #168). Until then, the value supplied here is the only
	// place that requirement is met or missed.
	JournalPath string

	// Placer is §4's OrderGateway narrowed to its one submitting method.
	//
	// The submit-once rule binds the adapter as well as the caller: an
	// implementation that retries inside Place breaks §5.3 exactly as
	// thoroughly as a caller that calls it twice, and the caller cannot see it
	// happen. A root wiring a transport with retries configured on it has
	// disabled this context's central safety property without touching this
	// context.
	//
	// A failure must carry an apperror.Effect, and an adapter that forgets gets
	// the conservative answer rather than a sharp one — apperror.EffectOf
	// reports EffectUnknown for anything it does not recognise, which sends the
	// order to SubmissionUnknown and the OMS to the venue to ask.
	Placer ports.OrderPlacer
}

// Module is the context's driving surface.
//
// # One field, and it is a use case rather than a driving adapter
//
// marketdata exposes driving.FundingDispatch and driving.BookDispatch, because
// its inbound edge is an events.Envelope that something has to translate. This
// context has no infrastructure/driving/ at all: §6's inbound edges are the
// private order and execution streams, which are a later slice, and driven/
// doc.go already records why an empty driving/ package is not created to
// satisfy a literal reading of §3.6 rule 3.
//
// So Dispatch is exported as what it is. Interposing an adapter that only
// forwards would be the same empty shape one directory over, and it would put a
// translation layer between a caller and the one method in this system whose
// signature is load-bearing — Dispatch's (Outcome, error) split exists so that
// `if err != nil { return }` cannot silently drop a live order, and a
// forwarding wrapper is one more place for that to be re-derived wrongly.
//
// The struct rather than a bare *application.Dispatcher return is the extension
// point: marketdata's Module went from one field to two when its book path
// landed, and slice B is that happening here.
//
// The two fields are ORDERED in the sense that matters: nothing Dispatch will
// accept exists until Plan has produced it, because Dispatch loads what it sends
// from the repository Plan fills. That is §6 steps 5 and 6 expressed as a
// dependency rather than as a convention.
type Module struct {
	// Plan is §6 step 5: deterministic ClientOrderID minting, and intents
	// fsynced to the write-ahead journal before any submission.
	Plan *application.Planner

	// Dispatch is §6 step 6's crash boundary: the one place in this system
	// where an order becomes a request at a venue.
	Dispatch *application.Dispatcher

	// journal is held only so Close can release it, and it is unexported for
	// the same reason ports.OrderWriteRepository never becomes a field here: a
	// *driven.SQLiteJournal handed to a root is a write path to order state
	// that singlewriter cannot see, because the root needs no import to use a
	// value it was given.
	journal *driven.SQLiteJournal
}

// New assembles the context, refusing an incomplete one.
//
// It adds no refusals of its own, on marketdata.New's argument and for its
// reason: the constructors below each name what they are missing, phrased in
// terms of the wire it is about, and a second statement of the same rule here is
// the copy that drifts — it is always the one written where the value is merely
// passing through. What this function contributes is ORDER, not validation.
//
// The consequence a caller can rely on: an error from New is the constructor's
// own sentinel, so a root can branch on application.ErrPlacerMissing or
// driven.ErrJournalPathUnset directly rather than on a wrapped string.
//
// # This constructor now ACQUIRES something, which it never did before
//
// Opening the journal is a resource acquisition, and every refusal after it must
// release it or leak a file handle and a SQLite connection for the lifetime of
// the process. The refusals after it are real — a root with a good path and no
// placer is exactly the half-configured case — so each failure path joins
// journal.Close() into its error rather than discarding it. Joining rather than
// ignoring is deliberate: a Close that fails during a failed New is the one
// report nobody would otherwise get, since there is no Module to call Close on.
func New(deps Deps) (*Module, error) {
	journal, err := driven.OpenSQLiteJournal(deps.JournalPath)
	if err != nil {
		return nil, fmt.Errorf("oms: journal: %w", err)
	}

	// One repository, shared. See the package comment: two would make Plan and
	// Dispatch disagree about which orders exist.
	orders := driven.NewMemoryOrderWriteRepository()

	planner, err := application.NewPlanner(journal, orders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: planner: %w", err), journal.Close())
	}
	dispatcher, err := application.NewDispatcher(journal, deps.Placer, orders)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("oms: dispatcher: %w", err), journal.Close())
	}

	return &Module{Plan: planner, Dispatch: dispatcher, journal: journal}, nil
}

// Close releases what New acquired.
//
// The journal is the only closeable thing this context owns; the repository is a
// map. It is a method on Module rather than a returned func because the caller
// that must not forget it is a composition root holding the Module — and a
// cleanup function is a value a root can drop, while a missing Close on a type
// it stores is a shape review notices.
//
// It does NOT stop in-flight work, and the distinction matters for a root
// writing a shutdown sequence: closing the journal while a Dispatch is between
// its authorization fsync and its Place produces an order sent with no result
// record. Ordering the shutdown so that nothing is dispatching belongs to the
// root, and §7.5's boot is what makes the surviving journal enough.
func (m *Module) Close() error { return m.journal.Close() }
