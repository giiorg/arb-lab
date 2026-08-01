// Package calibrate holds step 4's pure derivation math (§41f of the
// 2026-07-31 calibration-artifact design record): the committed rules that
// turn calibration-era history into the frozen numbers the artifact
// carries. It sits beside the estimator whose residuals it collects — the
// dispersion residuals are the DEPLOYED model's residuals, and measuring a
// different model's residuals would calibrate a stranger.
//
// # What lives here, and what does not
//
// Here: the causal residual collection over the trailing EWMA, the
// outward-conservative empirical quantiles, the EntryDrift tail rule, the
// entry-clearing level, the excursion statistics behind the exit band, the
// hold-scale adverse basis move, and the Enter/Continue occupancy replay.
// Every computation is fixedpoint (nofloat polices this package like any
// other), every selection rule states its conservative direction, and each
// derivation has an independent math/big oracle in the battery.
//
// Not here: I/O. Reading datasets, checking H1/H2/H3, and writing the
// artifact belong to the calibrate driving adapter
// (strategy/infrastructure/driving) — this package computes over values it
// is handed, so the battery can drive every rule without a filesystem.
//
// # Parameters are pre-registered, not chosen here
//
// Functions take their quantile grids, tail choices and horizons as
// arguments. The committed VALUES of those arguments live one layer out,
// in the driving adapter's declared constants, where §41e's procedure
// commit pins them — this package states the rules, the procedure commit
// states the numbers the rules ran with, and the artifact's derivation
// identities name both.
package calibrate
