// Package outbox is build-spec D2's transactional outbox: the at-least-once
// export from the OMS write-ahead journal to idempotent consumers (audit,
// observability, tax).
//
// # The one sentence that governs everything here
//
// D2 states it as an aside and it is the most important line in the package:
// "the outbox is *not* the exactly-once execution-truth mechanism — the journal
// is" `[→ X1]`. The journal (§7.5) is authoritative for reconciliation; audit is
// a LAGGED PROJECTION over this export. So the failure this package must never
// produce is a LOST event, and the failure it is permitted to produce — and
// will, routinely — is a DUPLICATE one. Everything below picks that direction.
//
// # What is deliberately NOT here, and why its absence is not an oversight
//
// There is no Append, no SQL, no schema and no driver. The outbox's defining
// guarantee is that the event append and the business write commit in ONE
// transaction (core §5.3, §6 step 6) — which is a property of the STORE, not of
// any type in this package, and the store it must join is the OMS write-ahead
// journal, which does not exist yet (milestone 3).
//
// Writing the SQL now would mean guessing at the transaction shape of a thing
// that has not been designed, and then testing that guess against itself. That
// is the same argument that defers hexboundary, singlewriter and exhaustiveswitch
// (§3.6 gate status): a check over an empty tree passes trivially, which is
// indistinguishable from a broken check.
//
// What DOES exist here is the half that is fully checkable today: the value
// types that cross the durability boundary, the two ports the relay drives, and
// the relay itself — whose ordering, retry and never-drop behaviour is decidable
// against a fake store with no database in sight.
//
// # Per-key FIFO, and why it is not global FIFO
//
// [SPEC EDIT LANDED — v0.9.6] §3.2 and D2 described the outbox as an
// "at-least-once export to idempotent consumers" and said nothing about ORDER.
// Order was load-bearing anyway: D2 also calls audit "a projection of the OMS
// journal", and a projection that applies a fill before the order that produced
// it does not reproduce its source. So the spec stated a delivery-count
// guarantee and relied, unstated, on a delivery-order one. This package made
// the order guarantee explicit and scoped it — FIFO within a Key, nothing
// across two — because "ordered" without a scope would be read as global FIFO,
// which is strictly stronger, unnecessary, and would let one stuck record halt
// every consumer. §3.2 now states it.
//
// Two events about the same trade must reach the audit projection in the order
// they were journaled: a fill applied before the order that produced it is a
// projection that contradicts its own source. But events about DIFFERENT trades
// carry no ordering relationship, and forcing a global order would let one stuck
// record halt every consumer.
//
// So the relay preserves FIFO within a Key and nothing across Keys. This is the
// same structural rule §3.5 states for dispatch — "an unready SourceID never
// stalls dispatch of ready inputs from other SourceIDs" — and the agreement is
// deliberate rather than coincidental.
//
// The cost is stated rather than hidden: a record that fails forever blocks ITS
// OWN key forever. That is the safe direction. Skipping past it would publish
// K7 while K5 is unsent, which silently corrupts the order of a tamper-evident
// projection; blocking is loud, because the key's backlog grows and its attempt
// count climbs where a metric can see it.
//
// # At-least-once means the consumer needs a dedup key, and Seq is it
//
// [SPEC EDIT LANDED — v0.9.7] D2 called the export at-least-once "to idempotent
// consumers" and never said idempotent ON WHAT, which leaves content-hash dedup
// as a reading that satisfies the spec and loses data. Named in §3.2 now, along
// with Payload being the committed bytes (Message.Payload).
//
// The relay publishes, then acks. A crash in that window redelivers. This is not
// a weakness to engineer away — closing it needs a distributed transaction with
// each consumer, which is exactly the exactly-once mechanism D2 assigns to the
// journal instead.
//
// Record.Seq is therefore the deduplication key, and it is durable and unique by
// the store's construction. A consumer MUST dedup on it, and specifically must
// NOT dedup on payload bytes: two legitimately identical events (the same funding
// accrual on two intervals) are distinct facts, and content-hash dedup would
// silently drop the second.
package outbox
