// Package application is marketdata's use-case layer: it drives the domain and
// orchestrates the ports declared beside it in application/ports/.
//
// §3.2 gives this context one job on the funding path — "ingest + normalize
// funding; stamp venue-seq + source/receive time + freshness; publish". The
// domain owns what an observation MEANS (what a valid one is, when its
// settlements fall, how stale it is). This layer owns the sequencing decisions
// that need memory of what came before, and it is where §3.4's two feed-failure
// modes are actually detected.
//
// # The driven ports live in application/ports/, which is §3.6 rule 2
//
// They used to be declared in this package, in application/ports.go, and that
// is the shape a Go reviewer would prefer — an interface is conventionally
// declared in the package that consumes it. Rule 2 asks for the subpackage
// anyway, and decision record §7 already answered the objection: application/
// IS the consumer, so the interface is still declared where it is used, one
// directory down.
//
// The move happened later than it should have. This context shipped closed
// against a rule it did not meet on either half — no ports/ subpackage and no
// compile-time assertions — and nothing caught it, because §3.6's enforcement
// is six analyzers and rule 2 is not one of them. What made it visible was
// oms following the rule and the two contexts then disagreeing in the same
// repo; the drift was recorded as §24f and carried across three sessions before
// it was fixed rather than absorbed.
//
// The subpackage constrains what may live where, and the constraint is worth
// stating because it is the thing that actually costs something: package ports
// must not import package application, or the import cycles. So every type
// crossing one of those seams lives in ports/ or in domain/ — never here. That
// is why FundingSnapshot, BookView and AcceptOrdinal moved with the interfaces
// rather than staying behind: they appear in port signatures, and a type in a
// port signature belongs to the port.
//
// # There is no interface for the driving side, and that is not an omission
//
// The use cases here — FundingIngest and BookReconstruct — ARE the driving
// port, and they are concrete types. An interface exists to allow substitution,
// and nothing substitutes a use case: §8's backtester substitutes the adapters
// beneath it, which is precisely what the interfaces in ports/ are for. §3.6
// rule 1 (hexboundary) is the enforcement of the direction, and it is a build
// failure rather than a convention.
package application
