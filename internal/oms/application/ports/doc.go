// Package ports declares the driven dependencies oms's use cases need, as
// §3.6 rule 2 requires: "every driven dependency is an interface there; adapters
// live in infrastructure/driven/ with a compile-time var _ ports.X =
// (*Impl)(nil)".
//
// # It is a subpackage rather than a file in application/, and that is rule 2
// # taken literally
//
// The alternative — declaring these in application/ports.go, in package
// application — is what internal/marketdata does today, and it is the shape a Go
// reviewer would prefer, since an interface is conventionally declared in the
// package that consumes it. The repo's own assessment already answered that
// objection (decision record §7): application/ IS the consumer, so the interface
// is still declared where it is used, one directory down.
//
// The cost of the subpackage is real and worth naming, because it constrains
// what may live here: package ports must not import package application, or the
// import cycles. So every type that crosses one of these seams lives HERE or in
// domain/ — never in application/. That is a discipline rather than a
// limitation, and it has a useful effect: a use-case type that would be
// convenient to hand an adapter has to be moved deliberately, which is the
// moment somebody asks whether the adapter should see it at all.
//
// # Every port here is DRIVEN, and there is no interface for the driving side
//
// The use cases in application/ are the driving port, and they are concrete
// types. An interface exists to allow substitution, and nothing substitutes a
// use case — §8's backtester substitutes the adapters beneath it, which is
// exactly what these interfaces are for.
//
// # Segregation is §4's rule, not a preference
//
// §4 declares one OrderGateway with seven methods and says contexts "consume
// segregated capability interfaces declared in their own application/ports/,
// satisfied by thin ACL adapters". So the interfaces here are narrower than §4's
// by design: a use case that only submits declares only the submit method, and
// the one adapter satisfies several of them. The stated risk to watch is fat
// ports — anything past ~5 methods — rather than the count of interfaces.
package ports
