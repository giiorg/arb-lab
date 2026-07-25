// The six raw-text invariants a Georgian fragment must share with its English original.
//
// These guarantees live in the file's *text*, not in its rendered output, so they need
// raw-text checks. Shared by `assemble.mjs` (which refuses to write on a violation) and
// by `tests/parity.test.js` (which re-runs them file-wide, so a hand-edit of the
// generated tutorial.ka.md is caught even if assemble is never run again).

const multiset = (re) => (text) => [...text.matchAll(re)].map((m) => m[0]).sort();

// Digits, and the subscript digits the §3.9 failure timeline uses for t₀…t₅.
export const numbers = multiset(/\d+(?:[.,]\d+)*|[₀-₉]/g);

// Identifiers, not prose: `AbsentAfterFinality`, `C_allocated`, `cid-perp`.
export const codeSpans = multiset(/`[^`\n]+`/g);

// Three syntactic shapes, three different breakages: `§spec §2.2` into the normative
// core, `SM §2` / `Profiles §1` into sibling docs, and a bare `§3.9` self-reference.
// The optional label is part of the token so that dropping `SM` is caught too.
export const pointers = multiset(
  /(?:\b(?:SM|Profiles|core|spec)\s+)?§[A-Za-z]*\d*(?:\.\d+)*/g
);

export const linkTargets = multiset(/\]\([^)\s]+\)/g);

// Row count, per-row column count, and which rows are `|---|` alignment rows — per
// contiguous table block, in order.
export function tableShape(text) {
  const out = [];
  let block = null;
  for (const line of text.split("\n")) {
    if (line.trimStart().startsWith("|")) {
      if (!block) out.push((block = []));
      block.push({
        cols: (line.match(/\|/g) || []).length,
        rule: /^\s*\|[\s:|-]*\|\s*$/.test(line),
      });
    } else block = null;
  }
  return out;
}

// The heading number is what every external pointer addresses — `build-spec.md`'s
// `[mechanism: …]` pointers, ADR §3 traceability, the guide's `tutorial §3.9` citation
// chips. The title after it is translated; the number and any trailing `[→ …]` tag are
// not.
const HEADING_PREFIX = /^#{2,3}\s+(?:Part\s+\d+\s+—\s+|\d+\.\d+\s+)?/;

export function headingParts(line) {
  return {
    prefix: (HEADING_PREFIX.exec(line) || [null])[0],
    tag: (/\s*(`\[→[^`]*\]`)\s*$/.exec(line) || [null, null])[1],
  };
}

const eq = (a, b) => a.length === b.length && a.every((v, i) => v === b[i]);

/**
 * @returns {string[]} human-readable violations; empty means the fragment is clean.
 */
export function check(id, en, ka) {
  const problems = [];
  const cmp = (name, fn) => {
    const a = fn(en);
    const b = fn(ka);
    if (!eq(a, b)) {
      const missing = a.filter((x) => !b.includes(x));
      const extra = b.filter((x) => !a.includes(x));
      problems.push(
        `${name} differs` +
          (missing.length ? `\n      missing: ${missing.join(" ")}` : "") +
          (extra.length ? `\n      extra:   ${extra.join(" ")}` : "") +
          (!missing.length && !extra.length ? " (same tokens, different counts)" : "")
      );
    }
  };

  const enHead = en.split("\n")[0];
  if (/^#{2,3}\s/.test(enHead)) {
    const kaHead = ka.split("\n")[0];
    const a = headingParts(enHead);
    const b = headingParts(kaHead);
    if (!kaHead.startsWith(a.prefix)) {
      problems.push(`heading number changed\n      en: ${a.prefix}\n      ka: ${kaHead.slice(0, 40)}`);
    }
    if (a.tag !== b.tag) problems.push(`heading tag changed: ${a.tag} → ${b.tag}`);
    if (kaHead.trim() === enHead.trim()) problems.push("heading is untranslated");
  }

  cmp("numbers", numbers);
  cmp("inline code", codeSpans);
  cmp("§-pointers", pointers);
  cmp("link targets", linkTargets);

  const ta = JSON.stringify(tableShape(en));
  const tb = JSON.stringify(tableShape(ka));
  if (ta !== tb) problems.push(`table shape differs\n      en: ${ta}\n      ka: ${tb}`);

  return problems.map((p) => `${id}: ${p}`);
}

// The header fragment carries a Georgian banner and two verbatim-English provenance
// lines that have no English counterpart to compare against. Only the region from
// `**Status:**` onward is comparable; the banner above it is exempt by construction.
export const STATUS = "**Status:**";
export const comparableHeader = (text) => {
  const i = text.indexOf(STATUS);
  if (i < 0) throw new Error(`header fragment has no "${STATUS}" line`);
  return text.slice(i);
};
