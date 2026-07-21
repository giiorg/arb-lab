#!/usr/bin/env bash
# Model-check the FenceTakeover spec: nominal safety + liveness must PASS;
# the three negative controls must FAIL on their predicted invariant
# (proving the model actually catches the documented hazards).
#
# Requires: java (17+). Fetches a pinned tla2tools.jar on first run.
set -u
cd "$(dirname "$0")"

# Prefer a real JDK over Apple's /usr/bin/java stub when Homebrew has one.
[ -d /opt/homebrew/opt/openjdk/bin ] && PATH="/opt/homebrew/opt/openjdk/bin:$PATH"

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

# run <cfg-basename> <expect: pass|fail> <expected-violation-regex-if-fail>
run() {
  local cfg="$1" expect="$2" want="${3:-}"
  local out="$TOOLS_DIR/$cfg.out"
  echo "=== $cfg (expect: $expect${want:+ on $want}) ==="
  # -lncheck final: liveness is checked ONCE over the completed state
  # graph. TLC's default periodic liveness passes re-walk the growing
  # graph inside its single-threaded temporal phase -- with this model's
  # fairness set that made the liveness configs impractically slow (>1 h);
  # final-only checking is minutes. (No effect on the safety/NC configs:
  # no PROPERTIES there. Trade-off: a liveness VIOLATION is now reported
  # only after the full graph is built -- acceptable for a verification
  # suite whose liveness configs are expected green.)
  java -XX:+UseParallelGC -cp "$JAR" tlc2.TLC \
    -workers auto -lncheck final -config "$cfg.cfg" -metadir "$TOOLS_DIR/states-$cfg" \
    FenceTakeover.tla >"$out" 2>&1
  local rc=$?
  grep -E "Model checking completed|Error:|violated|states generated" "$out" | head -6
  if [ "$expect" = "pass" ]; then
    if [ $rc -eq 0 ]; then echo "RESULT: PASS (as predicted)"; return 0
    else echo "RESULT: UNEXPECTED FAILURE (rc=$rc) -- see $out"; return 1; fi
  else
    if [ $rc -ne 0 ] && grep -qE "Invariant ($want) is violated" "$out"; then
      echo "RESULT: VIOLATED $want (as predicted)"; return 0
    else echo "RESULT: UNEXPECTED (rc=$rc; wanted violation of $want) -- see $out"; return 1; fi
  fi
}

fetch_jar
fails=0
run "FenceTakeover"      pass                                            || fails=$((fails+1))
echo
run "FenceTakeover_Live" pass                                            || fails=$((fails+1))
echo
run "FenceTakeover_Live2" pass                                           || fails=$((fails+1))
echo
run "FenceTakeover_NC1"  fail "I1_NeverTwoWriters"                       || fails=$((fails+1))
echo
run "FenceTakeover_NC2"  fail "I2_HostIndependentCoverage|I3_EvidenceGatedTerminal" || fails=$((fails+1))
echo
run "FenceTakeover_NC3"  fail "I1_NeverTwoWriters"                       || fails=$((fails+1))
echo
run "FenceTakeover_NC4"  fail "I2_HostIndependentCoverage|I3_EvidenceGatedTerminal" || fails=$((fails+1))
echo
if [ $fails -eq 0 ]; then echo "ALL 7 RUNS MATCH PREDICTIONS"; else echo "$fails RUN(S) OFF-PREDICTION"; exit 1; fi
