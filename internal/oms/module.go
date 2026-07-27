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
// # Scope: the dispatch path is WIRED; the write repository is NOT
//
// The context has two assembled-looking halves and only one of them is
// assembled, so the boundary is stated here rather than left to be inferred from
// what the file happens to construct.
//
// application.Dispatcher is wired: §6 step 6's crash boundary, its two ports
// supplied below.
//
// driven.MemoryOrderWriteRepository is NOT constructed, and the absence is the
// same kind marketdata's mark path is — a component with nothing yet to compose
// it into, left visible in the TYPE rather than argued for in prose somewhere
// else. §6's write path is journal-then-apply, and the apply half has no caller:
// ApplyPlanned belongs to §6 step 5's planning use case, ApplyState and
// ApplyFill to the private order and execution stream handlers, and none of
// those exist. Dispatch takes a domain.Order and carries no OCC version, so
// wiring a repository into it today would mean inventing the planning path to
// have produced a record for it to advance.
//
// Constructing it anyway and holding it in an unread field was the tempting
// alternative and is refused on driven/doc.go's own argument about the empty
// driving/ package: a shape that looks compliant while asserting nothing. The
// worse reading is the one it invites — that the dispatch path applies, when it
// journals only.
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
	"fmt"

	"github.com/giiorg/arb-lab/internal/oms/application"
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
)

// Deps is what a composition root supplies.
//
// # Two ports and nothing else, which inverts marketdata's asymmetry
//
// That context takes three concrete kernel types and one port, and its doc
// records the port as the exception worth explaining. Here every field is a
// port, and the inversion is not a change of taste: neither of these two
// adapters is constructible in this package today, for two DIFFERENT reasons —
// and only one of them is permanent.
//
// # The gate decides this struct's future shape, and that is worth predicting
//
// Placer's adapter is a venue client. Venue clients live outside this context
// entirely (§20a defers them), which is marketdata.Deps.Resnapshotter's
// situation exactly, so this field is permanent: a root supplies it or the
// context does not build.
//
// Journal's adapter is this context's own. §5.1's write-ahead journal is
// oms infrastructure and belongs in infrastructure/driven/ beside the write
// repository; it is a Deps field because it has not been written yet, not
// because a root is the right owner of it. When it lands, singlewriter FORCES
// the correction rather than merely inviting it — the adapter will sit in a
// policed package, so no root can name its constructor, and this field becomes
// the journal's configuration (a path, a fsync policy) with module.go
// constructing the adapter from it. That is §26e point 3's "move the type
// rather than widen the gate" arriving as a scheduled consequence instead of as
// a surprise.
//
// # Neither field is refused here
//
// A root passing a nil interface gets application.ErrJournalMissing or
// ErrPlacerMissing from the constructor that owns the wire — see New.
type Deps struct {
	// Journal is §5.1's durable, append-only, hash-chained write-ahead journal.
	//
	// Its contract is durability on return with no Sync to pair, stated once on
	// ports.Journal and binding on every method: an implementation that batches
	// or buffers does not satisfy it, whatever its method set says. That is the
	// half of the port a compile-time assertion cannot check, and it is a
	// property of what a ROOT wires here rather than of anything this file can
	// verify — so it is repeated at the seam where the wrong value is supplied.
	//
	// What it costs when it is wrong: nothing fails, the tests pass, and the
	// defect surfaces once — during the crash the fsync existed for, when
	// recovery finds no record of an order the venue is holding.
	Journal ports.Journal

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
// landed, and the recovery and fill paths will do the same here.
type Module struct {
	// Dispatch is §6 step 6's crash boundary: the one place in this system
	// where an order becomes a request at a venue.
	Dispatch *application.Dispatcher
}

// New assembles the context, refusing an incomplete one.
//
// It adds no refusals of its own, on marketdata.New's argument and for its
// reason: application.NewDispatcher already names a missing journal and a
// missing placer, each phrased in terms of the wire it is about, and a second
// statement of the same rule here is the copy that drifts — it is always the one
// written where the value is merely passing through. What this function
// contributes is ORDER, not validation, and today the order is one step.
//
// The consequence a caller can rely on: an error from New is the constructor's
// own sentinel, so a root can branch on application.ErrJournalMissing directly
// rather than on a wrapped string.
func New(deps Deps) (*Module, error) {
	dispatcher, err := application.NewDispatcher(deps.Journal, deps.Placer)
	if err != nil {
		return nil, fmt.Errorf("oms: dispatcher: %w", err)
	}
	return &Module{Dispatch: dispatcher}, nil
}
