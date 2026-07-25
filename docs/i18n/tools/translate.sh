#!/usr/bin/env bash
# Translates one fragment of docs/tutorial.md into Georgian.
#
#   ./tools/translate.sh s-3-9        # a ### subsection
#   ./tools/translate.sh part-8       # a ## section with no subsections
#   ./tools/translate.sh list         # every fragment id
#
# Writes i18n/ka/tutorial/<id>.md for review. Assemble with:
#   node tools/assemble.mjs
#
# Fragments live on disk on purpose. R2d — a second round-4 reviewer response — is still
# pending and will edit tutorial.md; when it does, the Georgian costs one command per
# changed section instead of a re-translation project.
#
# The glossary is the guide's, read in place and never copied. It is what makes the two
# Georgian documents agree with each other. Never edit it mid-run: the guide's first bulk
# run omitted `leg` and produced three different Georgian words across three sections.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
id="${1:?usage: translate.sh <fragment-id>   e.g. s-3-9   (or: translate.sh list)}"

if [ "$id" = "list" ]; then node "$here/split.mjs" list; exit 0; fi

out="$here/../ka/tutorial/${id}.md"
mkdir -p "$(dirname "$out")"

fragment="$(node "$here/split.mjs" get "$id")"
parent="$(node "$here/split.mjs" parent "$id")"
glossary="$(cat "$here/../../guide/i18n/glossary.md")"

context=""
[ -n "$parent" ] && context="This fragment sits under the part heading below. It is CONTEXT ONLY —
do not translate it, do not output it:

    $parent
"

# The preamble carries two provenance lines written for spec maintainers, citing every
# FM/Cx tag in the project and four doc filenames. They are the densest drift surface in
# the file and the least teaching value, so they are reproduced rather than translated.
# Keeping this a rule rather than hand-writing the fragment matters: the Status line
# changes on every version bump, and a hand-written header would need hand work each time.
frozen=""
[ "$id" = "header" ] && frozen="0. FROZEN LINES. The line beginning \"**Status:**\" and the line beginning
   \"**Doc set:**\" are provenance metadata. Reproduce BOTH byte-for-byte in English,
   including every § pointer, version number, date and backticked filename. Do not
   translate them, do not reformat them, do not summarise them. Translate the \`#\` title
   and the \"**Audience.**\" and \"**What changed from the legacy tutorial.**\" paragraphs
   normally.
"

read -r -d '' prompt <<EOF || true
You are translating one section of a technical tutorial about crypto funding-rate
arbitrage from English into Georgian, for a Georgian-speaking software engineer who
trades. Write natural, fluent Georgian prose — not a word-for-word rendering. Keep the
original's plain, direct, unhyped teaching register.

$context
FIVE HARD RULES:

$frozen

1. HEADINGS. If the fragment starts with a Markdown heading, reproduce its \`#\` marks and
   its section NUMBER byte-for-byte, and translate only the title text after the number.
       "### 3.9 A complete worked trade" -> "### 3.9 <Georgian title>"
       "## Part 4 — The strategy-agnostic foundation" -> "## Part 4 — <Georgian title>"
   Section numbers are addressed from other documents; changing one silently breaks them.
   A trailing tag in backticks such as \`[→ R1-F4, R2-tutorial]\` stays verbatim, at the end.

2. NEVER translate, transliterate or reword anything inside backticks. \`C_allocated\`,
   \`DispatchAuthorized\`, \`AbsentAfterFinality\` and \`cid-perp\` are identifiers in a
   normative English spec, not prose. Reproduce every backtick span exactly.

3. NEVER alter a section pointer or a link target. "§spec §2.2", "SM §2", "Profiles §1"
   and a bare "§3.9" point into English documents — keep the label, the § and the number
   exactly as given. Markdown link targets like ](./build-spec.md) are paths: unchanged.

4. NUMBERS DO NOT MOVE BETWEEN DIGITS AND WORDS, in either direction. Every digit in the
   English must appear as the same digit in the Georgian, and no English number word may
   become a digit. "two legs" stays a word; "20 days" stays "20". Currency and percentages
   keep their English form: \$10,000 and 3.41% and 0.015%/8h. These figures are the source
   the spec and the interactive guide are pinned to.

5. TABLES keep their exact shape: the same number of rows, the same number of \`|\`
   separators per row, and the \`|---|---|---|\` alignment row untouched. Translate only
   the cell text.

Output ONLY the translated Markdown fragment. No preamble, no explanation, no code fence.

GLOSSARY — follow it exactly, every occurrence:

$glossary

FRAGMENT TO TRANSLATE:

$fragment
EOF

echo "translating $id …" >&2
agy --model "Gemini 3.1 Pro (High)" -p "$prompt" > "$out"
echo "wrote $out ($(wc -c < "$out" | tr -d ' ') bytes)" >&2
