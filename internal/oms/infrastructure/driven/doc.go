// Package driven holds oms's driven adapters: the implementations of the ports
// application/ports declares.
//
// # What is here, and why the whole package is off-limits to other contexts
//
// This is §3.6 rule 6's subject. "Only oms mutates order/fill/trade/position
// state, through the journal-then-apply path (§6). A lint forbids other
// contexts from importing the OMS write repositories." The singlewriter
// analyzer polices this package and application/ports by POSITION rather than
// by naming a type, so a second repository added here is covered on the day it
// is declared rather than on the day somebody remembers to cover it.
//
// The gate's own doc records what it can and cannot see; the short version is
// that it makes a second writer impossible to declare by accident, not
// impossible to construct by determination.
//
// # The store is in memory, and that is the MVP's design rather than a stub
//
// Durability in this system is the JOURNAL's job, not this package's. §6 step 5
// fsyncs intent before any submission, §6 step 6 fsyncs the dispatch
// authorization immediately before the network write, and §7.5's reconcile-only
// boot rebuilds state "from journal ⋈ venue truth". A current-state store that
// is reconstructed at boot from an append-only log is what that architecture
// asks for, so holding it in a map is the honest implementation of the MVP —
// not a placeholder for one.
//
// Two limits follow from it, and both are scoped rather than open:
//
//   - §5.3 requires applying a fill, advancing the per-instrument REST recovery
//     cursor and appending journal and outbox events to happen in ONE
//     transaction. That needs a transaction seam no port here declares, and it
//     lands with the durable adapter and the fill use case together, because a
//     transaction spanning two stores cannot be added to one of them alone.
//   - Nothing here replays the journal. The repository starts empty, which is
//     correct for a process that has not booted §7.5's reconciliation yet and
//     wrong for one that has skipped it — a distinction the boot sequence owns,
//     not this package.
//
// # There is no driving/ beside this package, and rule 3 is worth reading twice
//
// [SPEC EDIT LANDED — v0.9.11] §3.6 rule 3 reads "Infrastructure shape. infrastructure/
// contains exactly driven/ + driving/", which admits two readings: that both
// directories must exist, or that no third kind may. This context takes the
// second — oms's inbound adapters are §6's private order and execution streams,
// which are a later slice, and creating an empty driving/ package to satisfy a
// literal reading would be the same mistake as a lint over an empty tree: a
// shape that looks compliant while asserting nothing.
//
// The first reading also contradicts how the rule is used elsewhere. Rule 3's
// purpose is to keep infrastructure/ from growing a third category — a
// persistence/, a shared/ — where the adapter direction stops being visible in
// the path. Requiring both halves of a context that only has one would force
// exactly the empty package this codebase rejects everywhere else.
//
// Core §3.6 rule 3 now reads as a PERMITTED-children rule — "contains only
// driven/ and driving/" — so a context with adapters in one direction is
// compliant without an empty package in the other, and this context is the
// case it cites.
package driven
