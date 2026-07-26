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
# ---------------------------------------------------------------------------
# Reading the result
# ---------------------------------------------------------------------------
#
#   caught    the test suite failed — the claim is checked
#   SURVIVED  the suite passed with broken code
#   NO-OP!!   the pattern did not match; the mutation never ran
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

if out=$(go test "$target" 2>&1); then
    printf '  SURVIVED %-58s  <-- TEST GAP or an equivalent mutant; diagnose which\n' "$desc"
    printf '%s\n' "$applied"
    exit 1
else
    failing=$(printf '%s' "$out" | grep -cE '^\s*--- FAIL')
    printf '  caught   %-58s  (%s failing test(s))\n' "$desc" "$failing"
    exit 0
fi
