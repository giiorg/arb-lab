// Package ports is build-spec §3.6 rule 6's subject, not this gate's. An import
// of it is singlewriter's to report, and a second report here would make the
// isolating probe unable to say which gate answered.
package ports

// OrderWriteRepository is the write authority itself.
type OrderWriteRepository interface{ Journal() error }
