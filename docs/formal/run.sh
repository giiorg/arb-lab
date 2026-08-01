#!/usr/bin/env bash
# Model-check both formal models. Nominal safety + liveness must PASS;
# every negative control must FAIL on its predicted invariant or temporal
# property -- proving each model actually catches the documented hazards,
# and that the properties are not passing vacuously.
#
# Model (b) FenceTakeover: SM SS3/SS4/SS6 -- the fence/takeover/plan
#   composition. 8 runs.
# Model (a) AmbiguousOrder: SM SS2 composed with SM SS7 -- the
#   ambiguous-order classifier, the finality contract, the interval-floor
#   generation and PackageDelever's generations on one reservation ledger.
#   Slices A+B: 14 runs (composed nominal + the SS2-only nominal + NC1-NC12)
#   plus 20 POSITIVE controls, which are expected to be VIOLATED: each names
#   an interesting state, and a probe that PASSES is a coverage gap, not a
#   success. See AmbiguousOrderCoverage.tla.
#
#   AmbiguousOrder_SM2 is the SS2-ONLY configuration (MAX_GEN = 0). It is
#   where INV1's persistent conjunct INV1b is checked -- that conjunct's
#   premise is slice A's "no new counter-leg order appears mid-episode",
#   which SS7's perp reduce legitimately breaks. It also reproduces slice A's
#   recorded state graph exactly (25,514 / 5,169, depth 15), which is what
#   says the slice-B refactor did not quietly change the object slice A
#   verified.
#
# Requires: java (17+). Fetches a pinned tla2tools.jar on first run.
set -u
cd "$(dirname "$0")"

# Prefer a real JDK over Apple's /usr/bin/java stub when Homebrew has one.
[ -d /opt/homebrew/opt/openjdk/bin ] && PATH="/opt/homebrew/opt/openjdk/bin:$PATH"

# Worker count. `auto` = every core, which is what you want on a quiet
# machine. Override when something un-backfillable is running beside it --
# the market-data capture is I/O-bound but its data cannot be re-fetched,
# and model (b)'s liveness configs hold every core for ~30 minutes.
#   TLC_WORKERS=4 ./run.sh
WORKERS="${TLC_WORKERS:-auto}"

# Optional argument: an extended-regex filter on the config tag, so one
# model or one control can be re-checked without paying for the other
# model's liveness configs (~30 min). No argument = the whole suite, which
# is what the recorded results mean.
#   ./run.sh AmbiguousOrder      # model (a) + its positive controls
#   ./run.sh 'NC[0-9]'           # every negative control, both models
FILTER="${1:-.}"

TOOLS_DIR=".tools"
JAR="$TOOLS_DIR/tla2tools.jar"
JAR_URL="https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar"
JAR_SHA256="936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88"

fetch_jar() {
  mkdir -p "$TOOLS_DIR"
  if [ ! -f "$JAR" ]; then
    echo "Fetching tla2tools.jar (TLA+ tools v1.7.4)..."
    curl -sSL -o "$JAR" "$JAR_URL" || { echo "FATAL: download failed"; exit 2; }
  fi
  local got
  got="$(shasum -a 256 "$JAR" | awk '{print $1}')"
  if [ "$got" != "$JAR_SHA256" ]; then
    echo "FATAL: tla2tools.jar checksum mismatch (got $got)"; exit 2
  fi
}

# run <module> <cfg-basename-or-path> <expect: pass|fail|failprop|probe> \
#     <expected-violation-regex-if-not-pass>
run() {
  local mod="$1" cfg="$2" expect="$3" want="${4:-}"
  local tag; tag="$(basename "$cfg")"
  local out="$TOOLS_DIR/$tag.out"
  echo "=== $tag [$mod] (expect: $expect${want:+ on $want}) ==="
  # -lncheck final: liveness is checked ONCE over the completed state
  # graph. TLC's default periodic liveness passes re-walk the growing
  # graph inside its single-threaded temporal phase -- with model (b)'s
  # fairness set that made the liveness configs impractically slow (>1 h);
  # final-only checking is minutes. (No effect on the NC configs: no
  # PROPERTIES there. Model (b)'s nominal safety config carries one ACTION
  # property -- P1, the Cx7-3 completion-capability check -- which the
  # same final pass covers. Trade-off: a liveness VIOLATION is now
  # reported only after the full graph is built -- acceptable for a
  # verification suite whose liveness configs are expected green.)
  java -XX:+UseParallelGC -cp "$JAR" tlc2.TLC \
    -workers "$WORKERS" -lncheck final -config "$cfg.cfg" -metadir "$TOOLS_DIR/states-$tag" \
    "$mod.tla" >"$out" 2>&1
  local rc=$?
  grep -E "Model checking completed|Error:|violated|states generated" "$out" | head -6
  case "$expect" in
    pass)
      if [ $rc -eq 0 ]; then echo "RESULT: PASS (as predicted)"; return 0
      else echo "RESULT: UNEXPECTED FAILURE (rc=$rc) -- see $out"; return 1; fi ;;
    failprop)
      # A temporal-property violation, not an invariant one. TLC does not
      # name the offending property, so the config MUST list exactly one
      # (NC5) for the attribution to be unambiguous.
      if [ $rc -ne 0 ] && grep -q "Temporal properties were violated" "$out"; then
        echo "RESULT: VIOLATED $want (as predicted)"; return 0
      else echo "RESULT: UNEXPECTED (rc=$rc; wanted temporal violation of $want) -- see $out"; return 1; fi ;;
    probe)
      # POSITIVE control: the probe states the interesting state is NEVER
      # reached, so a violation is the GOOD outcome and a pass is a
      # coverage gap. Invariant probes and the one action probe are both
      # named by TLC, so attribution stays exact.
      if [ $rc -ne 0 ] && grep -qE "(Invariant|Action property) ($want) is violated" "$out"; then
        echo "RESULT: REACHED -- $want violated (as predicted)"; return 0
      else echo "RESULT: *** COVERAGE GAP *** (rc=$rc; $want was never violated) -- see $out"; return 1; fi ;;
    *)
      if [ $rc -ne 0 ] && grep -qE "Invariant ($want) is violated" "$out"; then
        echo "RESULT: VIOLATED $want (as predicted)"; return 0
      else echo "RESULT: UNEXPECTED (rc=$rc; wanted violation of $want) -- see $out"; return 1; fi ;;
  esac
}

# The positive-control configs are pure boilerplate around ONE probe name
# each -- the content is in AmbiguousOrderCoverage.tla, which is committed.
# Generated here so adding a probe means adding a property, not a file.
# $1 = probe name, $2 = INVARIANTS | PROPERTIES
gen_probe_cfg() {
  mkdir -p "$TOOLS_DIR"
  cat > "$TOOLS_DIR/PC_$1.cfg" <<EOF
\* GENERATED by run.sh -- edit AmbiguousOrderCoverage.tla, not this file.
SPECIFICATION Spec
CONSTANTS
  NORD                     = 3
  MAXQ                     = 2
  NPEND                    = 3
  MAX_DISPATCH             = 3
  MAX_GEN                  = 2
  FLOOR_NEAREST_ZERO       = TRUE
  FLOOR_ONCE               = TRUE
  STRADDLE_GUARD           = TRUE
  BLIND_COMPENSATION       = FALSE
  CLASSIFY_ON_FILLS_ONLY   = FALSE
  RESERVATION_AWARE_SIZING = TRUE
  RESERVE_AT_CREATION      = FALSE
  SPOT_CAPPED_BY_PERP      = TRUE
  PERP_FIRST               = TRUE
  QUIESCE_BEFORE_SUPERSEDE = TRUE
  RELEASE_ON_FINALITY      = TRUE
  RELEASE_ON_SUPERSEDE     = TRUE
$2
  $1
EOF
}

fetch_jar
fails=0
runs=0
skipped=0
r() {
  local tag; tag="$(basename "$2")"
  if ! printf '%s' "$tag" | grep -qE "$FILTER"; then skipped=$((skipped+1)); return 0; fi
  runs=$((runs+1)); run "$@" || fails=$((fails+1)); echo
}

echo "################ MODEL (b): FenceTakeover -- SM SS3/SS4/SS6 ################"
echo
r FenceTakeover FenceTakeover       pass
r FenceTakeover FenceTakeover_Live  pass
r FenceTakeover FenceTakeover_Live2 pass
r FenceTakeover FenceTakeover_NC1   fail "I1_NeverTwoWriters"
r FenceTakeover FenceTakeover_NC2   fail "I2_HostIndependentCoverage|I3_EvidenceGatedTerminal"
r FenceTakeover FenceTakeover_NC3   fail "I1_NeverTwoWriters"
r FenceTakeover FenceTakeover_NC4   fail "I2_HostIndependentCoverage|I3_EvidenceGatedTerminal"
r FenceTakeover FenceTakeover_NC5   failprop "P4_ArbitrationCompletes"

echo "################ MODEL (a): AmbiguousOrder -- SM SS2 (slice A) #############"
echo
r AmbiguousOrder AmbiguousOrder      pass
r AmbiguousOrder AmbiguousOrder_SM2  pass
r AmbiguousOrder AmbiguousOrder_NC1 fail "INV1_NeverOvershootSign"
r AmbiguousOrder AmbiguousOrder_NC2 fail "INV2_FloorIssuedAtMostOnce"
r AmbiguousOrder AmbiguousOrder_NC3 fail "INV3_NoUncertainCompensation"
r AmbiguousOrder AmbiguousOrder_NC4 fail "INV7_DeterminedIsReallyDetermined"
r AmbiguousOrder AmbiguousOrder_NC5 fail "INV3_NoUncertainCompensation"
r AmbiguousOrder AmbiguousOrder_NC6 fail "INV1_NeverOvershootSign"
r AmbiguousOrder AmbiguousOrder_NC7  fail "INV4_ReservationLedgerSound"
r AmbiguousOrder AmbiguousOrder_NC8  fail "INV5_PackageDeleverNeverNetShort"
r AmbiguousOrder AmbiguousOrder_NC9  fail "INV5_PackageDeleverNeverNetShort"
r AmbiguousOrder AmbiguousOrder_NC10 fail "INV6_SupersessionQuiescesFirst"
r AmbiguousOrder AmbiguousOrder_NC11 fail "INV4_ReservationLedgerSound"
r AmbiguousOrder AmbiguousOrder_NC12 fail "INV4_ReservationLedgerSound"

echo "###### MODEL (a) POSITIVE CONTROLS -- each MUST be violated (reached) ######"
echo
for p in PC1_FloorNeverDispatched PC2_NeverTwoGenerations \
         PC3_SubtrahendAlwaysZero PC4_StraddleNeverReached \
         PC5_FloorNeverRecursed PC6_NoShortSideFloor PC7_NoLongSideFloor \
         PC9_NoExposureSubtrahend PC10_NoFloorSubtrahend \
         PC11_NoPkgSpotSubtrahend PC12_NeverThreeProducers \
         PC13_PdNeverCompletes PC14_PdNeverHeld PC15_PdNeverInFlight \
         PC16_NeverSupersededByEmergency PC17_NeverSupersededByFence \
         PC18_PerpAlwaysFullyConfirmed PC19_PdNeverTerminalFlatten \
         PC20_NeverTwoGenerations PC21_TransientAlwaysZero; do
  gen_probe_cfg "$p" INVARIANTS
  r AmbiguousOrderCoverage "$TOOLS_DIR/PC_$p" probe "$p"
done
gen_probe_cfg PC8_NoCancelWithZeroExecutions PROPERTIES
r AmbiguousOrderCoverage "$TOOLS_DIR/PC_PC8_NoCancelWithZeroExecutions" \
  probe "PC8_NoCancelWithZeroExecutions"

# A filtered run is NOT the suite result. Say so in the summary line, so a
# partial green can never be quoted as the whole one.
suffix=""
[ "$skipped" -gt 0 ] && suffix=" (FILTERED: $skipped run(s) skipped -- NOT the full suite)"
if [ $fails -eq 0 ]; then echo "ALL $runs RUNS MATCH PREDICTIONS$suffix"
else echo "$fails OF $runs RUN(S) OFF-PREDICTION$suffix"; exit 1; fi
