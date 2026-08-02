#!/usr/bin/env bash
# Model-check both formal models. Nominal safety + liveness must PASS;
# every negative control must FAIL on its predicted invariant or temporal
# property -- proving each model actually catches the documented hazards,
# and that the properties are not passing vacuously.
#
# Model (b) FenceTakeover: SM SS3/SS4/SS6 -- the fence/takeover/plan
#   composition. 14 runs: 3 nominals (safety + two liveness), 5 negative
#   controls, and SLICE E's 6 CONTROL-WITNESS probes (W1-W6), which asked
#   model (a)'s witness-degeneracy question of controls that predate it.
#   Every one was REACHED: each control's stated harm is real, and TLC had
#   simply been reporting a shallower witness. NC4's is the sharpest --
#   FM-1's full cost is its committed 11-state trace plus ONE VEntryFill.
# Model (a) AmbiguousOrder: SM SS2 composed with SM SS7 -- the
#   ambiguous-order classifier, the finality contract, the interval-floor
#   generation and PackageDelever's generations on one reservation ledger.
#   Slices A+B+C+E+F: 61 runs. 27 are the nominals and the negative controls
#   (composed nominal + the SS2-only nominal + NC1-NC12 + NC17 + NC19 + NC20
#   + NC21 + NC22 + P6, then the nominal LIVENESS config + NC13-NC16 + NC18
#   + NC23); 29 are POSITIVE controls, expected to be VIOLATED because each names an
#   interesting state and a probe that PASSES is a coverage gap (see
#   AmbiguousOrderCoverage.tla); and 5 are CONTROL-WITNESS probes (W1-W5),
#   which ask of a control's own counterexample whether it demonstrates the
#   HARM or only the flag.
#
#   SLICE F encodes `FM-8`'s ruling (SM SS7 v0.9.25: "no naked residual" is a
#   STANDING condition, not only an entry test) and is four runs: NC21, the
#   PRE-RULING world, which must violate INV9; NC22, the landed transition
#   row read LITERALLY, which must violate INV10; PC29, which says SS7's
#   new third supersession row FIRES in the nominal model -- without it, a
#   green INV9 is equally consistent with a guard doing work and with a rule
#   nothing ever exercises; and W5, which asks NC21's own witness whether it
#   reaches the money (a NONZERO spot leg sent on the dissolved premise) or
#   only the flag. Plus P6, which is the FIX'S OWN JUSTIFICATION converted
#   into a run: FM-9's resolution rests on the premise being a quantity the
#   generation cannot move, and FM-7 is this repo's evidence for what happens
#   to a justification nobody re-derives. And NC23, which is a control on this
#   slice's OWN first answer: the premise-lost row was first left OUT of the
#   fairness set, with a reason, and the nominal liveness config refuted it --
#   a generation halted forever on a dissolved premise, P5 violated. NC23 is
#   that refutation kept as a run instead of as a war story.
#
#   SLICE E added NC20 (SS7d: does "never issued twice" carry safety weight
#   independent of the reservation? It PASSES the complete graph, so the rule
#   is defence in depth -- FM-7 is that its RECORDED JUSTIFICATION assumed the
#   reservation is not counted) and PC27/PC28 (SS8f: PC27 shows a generation
#   IS opened over a physically-exposed package, PC28 shows its premise can
#   then dissolve -- FM-8, ruled and landed, model encoding owed as slice F).
#
#   SLICE E also added the SECOND PARK: SM SS7 enters Held/Alert from
#   `PerpReduceInFlight` OR `SpotReduceInFlight` and slices A-D modeled only
#   the second, so `HeldPerp`, P5, INV8, NC17 (the cause-blind exit -- model
#   (b)'s Cx8-2 asked here for the first time), NC18 and probes PC25/PC26
#   are that gap closed.
#
#   Slice C's four temporal properties run under LiveSpec (Spec + a fairness
#   set argued action-by-action in the module). Fairness constrains
#   BEHAVIOURS, not reachable states, so the liveness config's state graph is
#   the safety config's -- which is what lets the probes, all run against the
#   safety constants, stand as the non-vacuity evidence for P1-P4's
#   antecedents.
#
#   AmbiguousOrder_SM2 is the SS2-ONLY configuration (MAX_GEN = 0). It is
#   where INV1's persistent conjunct INV1b is checked -- that conjunct's
#   premise is slice A's "no new counter-leg order appears mid-episode",
#   which SS7's perp reduce legitimately breaks. It also reproduces slice A's
#   recorded state graph exactly (25,514 / 5,169, depth 15), which is what
#   says the slice-B refactor did not quietly change the object slice A
#   verified.
#
# 75 runs in total (14 + 61). A FILTERED run is not the suite result -- the
# summary line says so, and prose downstream must not launder one into the
# other.
#
# CLEAR `.tools/states-*` BEFORE A FULL RUN. TLC keeps one -metadir per config
# and never reclaims them, so they accumulate across sessions -- measured at
# 12 GB over 114 directories on 2026-08-02, and a full run under that pressure
# killed FenceTakeover_Live2's JVM with SIGBUS in an mmap'd arraycopy as the
# temporal phase started. The .out files are the results; the metadirs are
# scratch, so `rm -rf .tools/states-*` loses nothing. Matters most beside the
# market-data capture, where disk is the shared resource that cannot be
# re-fetched.
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
    deadlock)
      # SLICE E. A control whose predicted signature is a DEAD END -- the
      # state has no successors at all. Kept as its own verdict rather than
      # folded into `fail` for the reason NC15's config already draws out:
      # a deadlock and an invariant violation are different claims, and a
      # harness that accepts either would let a control that STRANDED the
      # model read as a control that CAUGHT something. Here the dead end IS
      # the finding (FM-6), so it is named.
      if [ $rc -ne 0 ] && grep -q "Deadlock reached" "$out"; then
        echo "RESULT: DEADLOCK (as predicted -- $want)"; return 0
      else echo "RESULT: UNEXPECTED (rc=$rc; wanted a deadlock -- $want) -- see $out"; return 1; fi ;;
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
#
# The `AmbiguousOrder_` prefix is LOAD-BEARING, not decoration: $FILTER is
# matched against the config TAG, so a probe named `PC_*` sits outside the
# namespace its own model owns and `./run.sh AmbiguousOrder` -- the usage
# documented above -- silently selected 14 of model (a)'s 35 runs, skipping
# every coverage probe. The summary's FILTERED line said so, but the
# example claimed "model (a) + its positive controls" and did not deliver
# them. A probe that a plausible invocation cannot select is a probe that
# is not run.
# $1 = probe name, $2 = INVARIANTS | PROPERTIES
gen_probe_cfg() {
  mkdir -p "$TOOLS_DIR"
  cat > "$TOOLS_DIR/AmbiguousOrder_PC_$1.cfg" <<EOF
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
  RECLASSIFY_ON_FINALITY    = TRUE
  RECLASSIFY_AFTER_DEADLINE = TRUE
  HELD_RESUMES              = TRUE
  BLIND_RETRY               = FALSE
  HELD_EXIT_CAUSE_BLIND     = FALSE
  RECLASSIFY_ON_LEDGER_CHANGE = TRUE
  PREMISE_IS_STANDING       = TRUE
  PREMISE_NETS_OWN_LEGS     = TRUE
  PREMISE_SUPERSESSION_FAIR = TRUE
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
# SLICE E item 2 -- the CONTROL-WITNESS probes model (a) has had since
# slice C, asked of model (b)'s five controls for the first time. Same
# polarity as every positive control here: the probe claims the harmful
# state is never reached, so VIOLATED = reached = the control's harm is
# real. Each config lists exactly ONE probe and NO I-invariant -- the NC5
# attribution rule, and here it also keeps the search alive past the
# shallow violation that is the control's own result.
r FenceTakeover FenceTakeover_W1 probe "W1_NoPostLatchWriteOverExposure"
r FenceTakeover FenceTakeover_W2 probe "W2_NoTombstoneOverExposure"
r FenceTakeover FenceTakeover_W3 probe "W3_NoTwoWritersBothLanded"
r FenceTakeover FenceTakeover_W4 probe "W4_NoExposureAfterActuatedFlat"
r FenceTakeover FenceTakeover_W5 probe "W5_NoNakedUnarbitratedStrand"
# W6 exists because W4's TRACE was read: W4 was reached and showed only the
# first half of FM-1's claim (exposure re-admitted, coverage still intact).
# A probe being violated says the state is reachable; only the trace says
# whether it is the harm the control names.
r FenceTakeover FenceTakeover_W6 probe "W6_NoStaleTombstoneOverExposure"

echo "###### MODEL (a): AmbiguousOrder -- SM SS2 x SS7, slices A+B+C+E ##########"
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
# SLICE E -- the second park. NC17 is model (b)'s Cx8-2 asked of model (a):
# a cause-blind park exit. INV8 is the isolating invariant (NC9 leaves it
# green; see the config).
r AmbiguousOrder AmbiguousOrder_NC17 fail "INV8_CompleteMeansLegsFinal"
# NC19 is FM-6: classification on ORDER events only. Its signature is a
# DEAD END, not a violated invariant -- see the config for why that is the
# honest report and why no slice before E could have produced it.
r AmbiguousOrder AmbiguousOrder_NC19 deadlock "FM-6 stale verdict, no admissible action"
# SLICE E item 3. NC2's constants with INV2 REMOVED and INV1 alone: NC2
# halts at the first floorCount = 2 state, so SS7d's "overshoot stays FALSE
# throughout" was read off a truncated search in which overshoot was not a
# checked invariant at all. Non-vacuity is free -- INV2 IS `floorCount <=
# 1`, so NC2's violation already proves double issuance is reached here.
r AmbiguousOrder AmbiguousOrder_NC20 pass
# SLICE F -- FM-8's ruling encoded. NC21 is the PRE-RULING world (the
# precondition as an entry guard alone, which is how slices A-E modeled it),
# and it must violate INV9. NC22 is the landed transition row read LITERALLY
# -- a control on the SPEC'S OWN SENTENCE, not on an invented defect -- and
# it must violate INV10. One invariant each: with two listed, TLC halts on
# whichever is shallower and the other is never demonstrated (the I2 defect
# slice E found in model (b)).
r AmbiguousOrder AmbiguousOrder_NC21 fail "INV9_GenerationNeverAdvancesOnLostPremise"
r AmbiguousOrder AmbiguousOrder_NC22 fail "INV10_PremiseSupersessionOnlyOnRealLoss"
# P6 is FM-9's FIX ARGUING FOR ITSELF, converted into a run: the trigger's
# quantity is the right one BECAUSE the generation cannot move it, and that
# claim is otherwise prose in three documents. FM-7 is what an unre-derived
# justification costs. Nominal constants -- it is a claim about the model's
# own arithmetic.
r AmbiguousOrder AmbiguousOrder_P6 pass

# SLICE C -- liveness. The nominal config checks P1-P4 under LiveSpec (the
# fairness set), and each control below flips ONE constant and must produce a
# TEMPORAL violation. `failprop` cannot name the property TLC violated, so
# each of these configs lists exactly one -- the NC5 discipline, applied to
# four more.
r AmbiguousOrder AmbiguousOrder_Live pass
r AmbiguousOrder AmbiguousOrder_NC13 failprop "P1_AmbiguousEventuallyClassified"
r AmbiguousOrder AmbiguousOrder_NC14 failprop "P2_DeadlineExceededResolves"
r AmbiguousOrder AmbiguousOrder_NC15 failprop "P3_HeldGenerationResumes"
r AmbiguousOrder AmbiguousOrder_NC16 failprop "P4_FloorGenerationResolves"
# SLICE E. NC18 is NC15's constant with P5 listed alone -- the same ruling
# governs both parks, and a property whose sibling fails is an argument, not
# evidence that IT bites.
r AmbiguousOrder AmbiguousOrder_NC18 failprop "P5_HeldPerpGenerationResumes"
# SLICE F. NC23 is a control on THIS SLICE'S OWN FIRST ANSWER: the
# premise-lost supersession was first left OUT of the fairness set, with a
# reason, and the nominal liveness config refuted it. Without this run that
# refutation is a session anecdote and a later reader tidying the fairness set
# finds no objection -- FM-7's lesson applied to a mistake made here.
r AmbiguousOrder AmbiguousOrder_NC23 failprop "P5_HeldPerpGenerationResumes"

echo "###### MODEL (a) POSITIVE CONTROLS -- each MUST be violated (reached) ######"
echo
# PC2 and PC20 share a suffix and are NOT duplicates -- do not delete one as
# redundant. PC2 is CONCURRENCY (two ledger entries active at once, which is
# what makes outstanding_reserved_reductions a SUM); PC20 is the SEQUENTIAL
# ladder (a second generation is ever started, which SM SS7 admits only after
# the prior one reaches finality). Both are REACHED, so both are load-bearing.
for p in PC1_FloorNeverDispatched PC2_NeverTwoGenerations \
         PC3_SubtrahendAlwaysZero PC4_StraddleNeverReached \
         PC5_FloorNeverRecursed PC6_NoShortSideFloor PC7_NoLongSideFloor \
         PC9_NoExposureSubtrahend PC10_NoFloorSubtrahend \
         PC11_NoPkgSpotSubtrahend PC12_NeverThreeProducers \
         PC13_PdNeverCompletes PC14_PdNeverHeld PC15_PdNeverInFlight \
         PC16_NeverSupersededByEmergency PC17_NeverSupersededByFence \
         PC18_PerpAlwaysFullyConfirmed PC19_PdNeverTerminalFlatten \
         PC20_NeverTwoGenerations PC21_TransientAlwaysZero \
         PC22_ClsNeverAmbiguous PC23_ClsNeverDeadlineExceeded \
         PC24_FloorNeverOutstanding \
         PC25_PdNeverHeldPerp PC26_PerpParkAlwaysEmpty \
         PC27_GenNeverOpenedOverReservedExposure \
         PC28_GenPremiseNeverEvaporates \
         PC29_PremiseLostNeverSupersedes; do
  gen_probe_cfg "$p" INVARIANTS
  r AmbiguousOrderCoverage "$TOOLS_DIR/AmbiguousOrder_PC_$p" probe "$p"
done
gen_probe_cfg PC8_NoCancelWithZeroExecutions PROPERTIES
r AmbiguousOrderCoverage "$TOOLS_DIR/AmbiguousOrder_PC_PC8_NoCancelWithZeroExecutions" \
  probe "PC8_NoCancelWithZeroExecutions"

echo "###### MODEL (a) CONTROL-WITNESS PROBES -- is the CONTROL'S witness harmful? #####"
echo
# TLC reports A counterexample, not the most harmful one. Each of these runs
# a CONTROL's constants and asks whether the harmful version of that
# control's state is reachable at all -- so a control cannot demonstrate a
# flag flipping while saying nothing about the cost. Same polarity as the
# positive controls: the claim is the good outcome, so VIOLATED = reached.
# W1 was asked by hand in session 66 and is committed here because the
# session that added its mirrors (W2/W3) is the session that owed it.
r AmbiguousOrder AmbiguousOrder_W1 probe "NoNonzeroOrphan"
r AmbiguousOrder AmbiguousOrder_W2 probe "NoStrandedNakedResidual"
r AmbiguousOrder AmbiguousOrder_W3 probe "NoStrandedNakedResidual"
r AmbiguousOrder AmbiguousOrder_W4 probe "NoStuckFloorSubtrahend"
# SLICE F. Asked of NC21 in the session that built it, rather than three
# slices later -- which is the only change slice E's finding actually asks
# for. Its probe lives in the coverage module because it reads a flag no
# invariant does.
r AmbiguousOrderCoverage AmbiguousOrder_W5 probe "W5_NoSpotLegSentOnLostPremise"

# A filtered run is NOT the suite result. Say so in the summary line, so a
# partial green can never be quoted as the whole one.
suffix=""
[ "$skipped" -gt 0 ] && suffix=" (FILTERED: $skipped run(s) skipped -- NOT the full suite)"
if [ $fails -eq 0 ]; then echo "ALL $runs RUNS MATCH PREDICTIONS$suffix"
else echo "$fails OF $runs RUN(S) OFF-PREDICTION$suffix"; exit 1; fi
