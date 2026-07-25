#!/usr/bin/env bash
# Translates the UI string table (chrome, nav titles, calculator) in one call.
#
# One call rather than one per string, so the labels stay consistent with each
# other — a rail of nav titles translated independently drifts in register.
#
# Input:  i18n/ka/_strings.en.tsv   (key <TAB> English)
# Output: i18n/ka/_strings.ka.tsv   (key <TAB> Georgian)
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
src="$here/../i18n/ka/_strings.en.tsv"
out="$here/../i18n/ka/_strings.ka.tsv"
glossary="$(cat "$here/../i18n/glossary.md")"

read -r -d '' prompt <<EOF || true
Translate the interface strings of a crypto funding-rate arbitrage guide from
English into Georgian. The audience is a Georgian-speaking engineer who trades.

Input is one string per line: a key, a TAB, then the English text.
Output the SAME keys in the SAME order, one per line, key TAB Georgian text.
No preamble, no code fence, no extra lines.

HARD RULES:
- Keep {pts} placeholders exactly as written.
- Keep any HTML tags and their attributes byte-identical; translate only visible text.
- Never translate spec citations: "core §2.1", "tutorial §3.9", "core §12" stay verbatim.
- "spec v0.9.4 · 0 lines of Go" keeps the version and the numeral; translate only the words.
- Numbers, currency and percentages keep their English form.
- These are UI labels: keep them short. A Georgian label much longer than its English
  original will not fit the rail or the calculator grid.

GLOSSARY — follow it exactly:

$glossary

STRINGS:

$(cat "$src")
EOF

agy --model "Gemini 3.1 Pro (High)" -p "$prompt" > "$out"
echo "wrote $out ($(wc -l < "$out" | tr -d ' ') lines)" >&2
