#!/usr/bin/env bash
# Builds the inlined Georgian serif face for index.html.
#
# The guide is published as a self-contained Artifact under a CSP that blocks font
# CDNs, and none of the body stack (Iowan Old Style, Charter, Palatino, Georgia) has
# Mkhedruli coverage — so without this, Georgian would silently fall back to a system
# sans while English rendered serif. Inlining a subset keeps one visual identity.
#
# Source: Noto Serif Georgian, SIL Open Font License 1.1, from google/fonts.
# Output: base64 for two @font-face rules, ~21 KB and ~22 KB raw (~58 KB base64).
#
# Requires: curl, fonttools (pip install 'fonttools[woff]' brotli)
set -euo pipefail

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

url="https://github.com/google/fonts/raw/main/ofl/notoserifgeorgian/NotoSerifGeorgian%5Bwdth,wght%5D.ttf"
curl -sL --max-time 60 -o "$work/src.ttf" "$url"

# Georgian: Asomtavruli + Mkhedruli (U+10A0-10FF) and Mtavruli (U+1C90-1CBF).
# Latin and punctuation are kept so a Latin glossary term inside Georgian prose
# renders in the same face rather than jumping to the fallback mid-sentence.
ranges="U+10A0-10FF,U+1C90-1CBF,U+0020-007E,U+00A0,U+2013,U+2014,U+2018,U+2019,U+201C,U+201D,U+2026,U+00B7"

for weight in 400 600; do
  fonttools varLib.instancer "$work/src.ttf" wdth=100 "wght=$weight" -o "$work/i.ttf" >/dev/null
  pyftsubset "$work/i.ttf" \
    --output-file="$work/g$weight.woff2" --flavor=woff2 \
    --unicodes="$ranges" --layout-features="kern,liga,locl" \
    --desubroutinize --no-hinting
  printf '=== weight %s (%s bytes raw) ===\n' "$weight" "$(wc -c < "$work/g$weight.woff2" | tr -d ' ')"
  base64 < "$work/g$weight.woff2" | tr -d '\n'
  printf '\n\n'
done
