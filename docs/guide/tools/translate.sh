#!/usr/bin/env bash
# Translates one guide section into Georgian.
#
#   ./tools/translate.sh world/perps
#
# Writes i18n/ka/<route>.html for review. Splice it in with:
#   node tools/i18n.mjs splice world/perps
#
# Fragments live on disk on purpose: the English prose is still pending a voice
# review, so re-translating a section has to cost one command, not human work.
set -euo pipefail

route="${1:?usage: translate.sh <route>   e.g. world/perps}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
out="$here/../i18n/ka/${route//\//__}.html"
mkdir -p "$(dirname "$out")"

fragment="$(node "$here/i18n.mjs" extract "$route" en)"
glossary="$(cat "$here/../i18n/glossary.md")"

read -r -d '' prompt <<EOF || true
You are translating one section of a technical guide about crypto funding-rate
arbitrage from English into Georgian, for a Georgian-speaking software engineer
who trades. Write natural, fluent Georgian prose — not a word-for-word rendering.
Keep the original's plain, direct, unhyped register.

THREE HARD RULES:

1. Preserve every HTML tag and attribute EXACTLY as given, including class and
   data-src values and the order of attributes. Translate only visible text.
   The opening tag must become exactly:
       <article data-route="$route" lang="ka">
   Every other tag stays byte-identical.

2. NEVER translate or transliterate the text inside <span class="cite"> elements,
   and never alter a data-src attribute. "core §7.4" and "tutorial §3.9" are
   pointers into English normative documents and stay verbatim.

3. Output ONLY the translated HTML fragment. No preamble, no explanation, no
   markdown code fence.

Numbers, currency and percentages keep their English form: \$10,000 and 3.41%.

GLOSSARY — follow it exactly, every occurrence:

$glossary

SECTION TO TRANSLATE:

$fragment
EOF

echo "translating $route …" >&2
agy --model "Gemini 3.1 Pro (High)" -p "$prompt" > "$out"
echo "wrote $out ($(wc -c < "$out" | tr -d ' ') bytes)" >&2
