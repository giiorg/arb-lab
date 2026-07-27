#!/usr/bin/env bash
#
# Line-scoped mutation harness — the bite-check tool.
#
# Every correctness claim in this repo is bite-checked before it is written
# down: break the code a test covers, and confirm the test fails. A test that
# has never been seen to fail is a hope, not a check.
#
#   usage: tools/mutate.sh <file> <line> <sed-command> <description> [test-target]
#
#   tools/mutate.sh internal/platform/ids/generator.go 152 \
#       's/ms, counter = unixMS, 0/ms, counter = unixMS, g.counter/' \
#       "counter not reset on a new millisecond" ./internal/platform/ids
#
# The file is always restored, including if the test run is killed.
#
# ---------------------------------------------------------------------------
# Two design points, each of which is a lesson this project paid for
# ---------------------------------------------------------------------------
#
# 1. IT PROVES THE MUTATION APPLIED (session 4).
#
#    A pattern that fails to match produces EXACTLY the output of a caught
#    mutation: green before, red never happens, and a naive script reports
#    success. So this one `cmp`s and prints NO-OP!! rather than `caught` when
#    nothing changed. It fired on its first run in session 4, and again on its
#    first run in session 6 — that time on a bug in the harness itself, which
#    would otherwise have reported twelve phantom kills.
#
# 2. IT PROVES THE MUTATION APPLIED WHERE YOU MEANT (session 5).
#
#    `cmp` proves *a* change, not *the* change. A `Quantity.Mul` mutation once
#    reported SURVIVED because the pattern occurred three times in the file and
#    the substitution landed on `Neg` instead. Hence the line number, and hence
#    the printed diff: the file moving is not evidence about which function
#    moved.
#
# 3. IT PROVES THE MUTANT COMPILED (session 7).
#
#    `go test` exits non-zero for a BUILD failure exactly as it does for a
#    failing test, so a mutant that does not compile prints as `caught` and
#    proves nothing. Go makes this easy to hit: deleting a call site leaves an
#    unused import, which is a compile error, not a warning. Found on a real
#    mutation of ids.VenueInstrument.Validate — replacing a
#    strings.ContainsFunc(...) call with `false` orphaned two imports, and the
#    only visible trace was the failing-test count printing as 0. A kill with
#    zero failing tests is a contradiction, so the count is now checked rather
#    than merely displayed.
#
# ---------------------------------------------------------------------------
# Reading the result
# ---------------------------------------------------------------------------
#
#   caught    the test suite failed — the claim is checked
#   SURVIVED  the suite passed with broken code
#   NO-OP!!   the pattern did not match; the mutation never ran
#   NOBUILD!! the mutant did not compile; the tests never ran, so nothing was
#             proved. Re-aim the mutation so the code still builds — usually by
#             substituting a WRONG value rather than deleting a call.
#   PANIC!!   the mutant compiled, ran, and CRASHED. A kill, but a weaker one:
#             the suite observed a crash rather than a wrong answer, so the
#             claim you aimed at may still be unchecked. Usual cause is
#             mutating a nil guard.
#   TIMEOUT!! the suite HUNG on the mutant. Not a kill: a hanging test names no
#             defect, blocks CI and reads as an infrastructure problem (the §5e
#             lesson). Fix the TEST to fail within a bound, then re-run.
#
# A SURVIVED is either a test gap or a wrong mutant, and TELLING WHICH IS THE
# ACTUAL WORK. Do not paper over it, and do not assume it is a gap: some
# mutations are semantically equivalent (unreachable code, a value nothing
# outside the package can observe). Diagnose, then either add the test or
# record the equivalence with its reasoning.
#
# When a mutant survives, ask what the code is FOR rather than what it does.
# Deleting the UUIDv7 counter reset kept ids unique and ordered — everything
# the tests checked — while breaking the timestamp's meaning four thousand
# mints later.
#
# ---------------------------------------------------------------------------
# What this is NOT
# ---------------------------------------------------------------------------
#
# Not a substitute for `gremlins`, and not run by CI. `gremlins` was audited in
# session 5 (decision record §6b): it works, it found four real gaps, and it
# stays out of CI on measured grounds — a warm test cache makes it report 0.00%
# efficacy, and roughly one run in four emits a false LIVED. Run it by hand
# after new code lands, with `go clean -testcache` first, and reproduce every
# LIVED by hand before believing it.
#
# This script is the hand-aimed complement: you choose the mutation, so you are
# stating the claim you want checked.

set -uo pipefail

if [ "$#" -lt 4 ]; then
    sed -n '3,10p' "$0" | sed 's/^# \{0,1\}//'
    exit 64
fi

repo=$(cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo" || exit 1

file="$1"; line="$2"; expr="$3"; desc="$4"; target="${5:-./...}"

if [ ! -f "$file" ]; then
    printf '  ERROR    %s: no such file\n' "$file"
    exit 66
fi

backup=$(mktemp)
cp "$file" "$backup"
trap 'cp "$backup" "$file"; rm -f "$backup"' EXIT

# expr is a complete sed command ("s/a/b/"), applied only to the named line.
# BSD sed (macOS) needs the empty argument to -i; GNU sed accepts -i''.
if ! sed -i '' "${line}${expr}" "$file" 2>/dev/null; then
    sed -i "${line}${expr}" "$file" || {
        printf '  ERROR    %-58s  (sed rejected the expression)\n' "$desc"
        exit 65
    }
fi

if cmp -s "$backup" "$file"; then
    printf '  NO-OP!!  %-58s  (line %s did not match — the mutation never applied)\n' "$desc" "$line"
    exit 2
fi

# Print the exact change, so "which line moved" is evidence rather than belief.
applied=$(diff "$backup" "$file" | grep -E '^[<>]' | sed 's/^/           /')

# A bounded run, because a mutation can make a test BLOCK rather than fail —
# swapping a lane's non-blocking send for a blocking one is exactly that. Go's
# own default is 10 minutes, which in a loop of twenty mutations is an afternoon,
# and the result is not a kill anyway: §5e's lesson is that a hanging test names
# no defect. Bounded here so the hang is reported instead of waited on.
# MUTATE_MODULE names the module the target lives in, because this repo has two
# and only one of them is reachable from the root. `go test ./tools/lint/...`
# from here does not fail usefully — it reports "main module does not contain
# package", which go test exits non-zero for, so an unmutated analyzer would
# have printed `caught` and every gate's bite-check would have been a phantom.
# The mutation is still applied to the file at its repo-relative path; only the
# test run relocates.
#
#   MUTATE_MODULE=tools/lint tools/mutate.sh tools/lint/hexboundary/hexboundary.go \
#       213 's/inward()/outward()/' "polices the wrong layers" ./hexboundary
#
if out=$(go test -C "${MUTATE_MODULE:-.}" -timeout "${MUTATE_TIMEOUT:-60s}" "$target" 2>&1); then
    printf '  SURVIVED %-58s  <-- TEST GAP or an equivalent mutant; diagnose which\n' "$desc"
    printf '%s\n' "$applied"
    exit 1
fi

# go test exits non-zero for a build failure exactly as it does for a failing
# test, so a red run is not yet evidence. Two independent signals say "compiled
# but failed": at least one `--- FAIL` line, and no build-failure banner. Both
# are checked because either alone has a hole — a package can fail to build with
# no banner on the tested package itself, and a `--- FAIL` can appear in output
# that ALSO contains a build failure from a sibling package.
# A hang is checked BEFORE the build check, because a timeout also produces no
# `--- FAIL` line and would otherwise be misreported as a compile failure.
if printf '%s' "$out" | grep -qE 'test timed out|^panic: test timed out'; then
    printf '  TIMEOUT!! %-57s  (the suite HUNG — a hanging test names no defect; bound it)\n' "$desc"
    printf '%s\n' "$out" | grep -E 'test timed out|^\s*[A-Za-z0-9_]+\.func|^Test' | sed 's/^/           /' | head -4
    exit 4
fi

failing=$(printf '%s' "$out" | grep -cE '^\s*--- FAIL')

# A PANICKING mutant compiled and ran, but a panic prints no `--- FAIL` line, so
# the build check below would call it NOBUILD!! — "the tests never ran" — about a
# mutant that built cleanly and crashed. Two different diagnoses under one
# verdict, which is the same defect NOBUILD and TIMEOUT were each added to fix.
#
# It IS a kill: the suite went red on broken code. It is reported separately
# because what it proves is weaker. The test did not observe a wrong ANSWER, it
# observed a crash, so the claim you were aiming at may still be unchecked —
# nil-guard mutations are the usual source. Found in session 15 on
# exhaustiveswitch, where dropping a nil check made the analyzer segfault
# instead of report.
if [ "$failing" -eq 0 ] && printf '%s' "$out" | grep -qE '^panic:|^\[signal SIG'; then
    printf '  PANIC!!  %-58s  (a kill, but by CRASH — the claim itself may be unchecked)\n' "$desc"
    printf '%s\n' "$out" | grep -E '^panic:|^\[signal SIG' | sed 's/^/           /' | head -2
    exit 5
fi

if [ "$failing" -eq 0 ] || printf '%s' "$out" | grep -qE '\[build failed\]|^# '; then
    printf '  NOBUILD!! %-57s  (the mutant did not compile — the tests never ran)\n' "$desc"
    printf '%s\n' "$out" | grep -E '^[^[:space:]].*\.go:[0-9]+:|\[build failed\]' | sed 's/^/           /' | head -5
    exit 3
fi

printf '  caught   %-58s  (%s failing test(s))\n' "$desc" "$failing"
exit 0
