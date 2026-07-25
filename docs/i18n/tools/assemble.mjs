#!/usr/bin/env node
// Assembles docs/tutorial.ka.md from the Georgian fragments.
//
//   node tools/assemble.mjs           -> writes docs/tutorial.ka.md
//   node tools/assemble.mjs --check   -> validates and prints, writes nothing
//
// The guide has to *splice* Georgian into index.html because both languages ship in one
// file. Here the Georgian is its own file, so it is assembled instead: the output is a
// pure function of the fragments, there is no insert-vs-replace branch, and no regex
// ever runs over translated prose (the guide had to defend against `$&` inside a
// replacement string). A test asserts the file on disk equals a fresh assemble, so
// hand-editing tutorial.ka.md is a test failure rather than a silent divergence.
//
// It refuses to write unless every fragment matches its English original on the six
// invariants in ./invariants.mjs. A fragment that cannot satisfy them is a translation
// to fix — not a check to relax.
import { readFileSync, writeFileSync, existsSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { fragments } from "./split.mjs";
import { check, comparableHeader } from "./invariants.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
export const OUT = join(HERE, "..", "..", "tutorial.ka.md");
export const KA_DIR = join(HERE, "..", "ka", "tutorial");
export const fragmentPath = (id) => join(KA_DIR, `${id}.md`);

// Injected mechanically after the header fragment's title line, not translated: it is
// chrome about the file, and it must stay stable across re-runs. It sits *above* the
// `**Status:**` line, which is where every invariant comparison starts — so the banner
// is exempt from the checks by construction rather than by a special case.
const BANNER = `
> **ეს არის [\`docs/tutorial.md\`](./tutorial.md)-ის ქართული თარგმანი.** ორიგინალი
> ინგლისურენოვანია და უთანხმოების ნებისმიერ შემთხვევაში მას აქვს უპირატესობა.
> ფაილი გენერირებულია \`docs/i18n/ka/tutorial/\`-ში მდებარე ფრაგმენტებისგან — ხელით ნუ
> შეასწორებთ: შეასწორეთ ფრაგმენტი და გაუშვით \`node tools/assemble.mjs\`.
`;

const withBanner = (header) => {
  const nl = header.indexOf("\n");
  return header.slice(0, nl) + "\n" + BANNER + header.slice(nl);
};

// Models sometimes wrap output in a code fence despite being told not to.
const unfence = (s) =>
  s.replace(/^﻿/, "").replace(/^```[a-z]*\n?/i, "").replace(/\n?```\s*$/, "").trim();

export function assemble() {
  const frags = fragments();
  const missing = frags.filter((f) => !existsSync(fragmentPath(f.id))).map((f) => f.id);
  if (missing.length) {
    throw new Error(
      `${missing.length} fragment(s) not translated yet:\n  ${missing.join("\n  ")}` +
        `\n\nrun: ./tools/translate.sh <id>`
    );
  }

  const problems = [];
  const parts = [];
  for (const f of frags) {
    const ka = unfence(readFileSync(fragmentPath(f.id), "utf8"));
    const [en, kaCmp] =
      f.id === "header" ? [comparableHeader(f.body), comparableHeader(ka)] : [f.body, ka];
    problems.push(...check(f.id, en, kaCmp));
    parts.push((f.id === "header" ? withBanner(ka) : ka) + f.tail);
  }
  if (problems.length) {
    throw new Error(
      `refusing to assemble — ${problems.length} invariant violation(s):\n\n  ` +
        problems.join("\n  ") +
        `\n\nFix the fragment in ${KA_DIR}, then re-run. Do not relax the check:` +
        `\n  a §-number, a figure or an identifier that drifts here is a finding.`
    );
  }
  return parts.join("\n");
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const text = assemble();
  if (process.argv.includes("--check")) {
    console.log(`OK — ${fragments().length} fragments, ${Buffer.byteLength(text)} bytes`);
  } else {
    mkdirSync(dirname(OUT), { recursive: true });
    writeFileSync(OUT, text);
    console.log(`wrote ${OUT} (${Buffer.byteLength(text)} bytes)`);
  }
}
