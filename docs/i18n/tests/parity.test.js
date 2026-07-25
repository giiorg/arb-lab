import { test, expect } from "bun:test";
import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { join } from "node:path";
import { fragments } from "../tools/split.mjs";
import { assemble, OUT, fragmentPath } from "../tools/assemble.mjs";
import {
  check, comparableHeader, numbers, codeSpans, pointers, linkTargets,
  tableShape, headingParts, STATUS,
} from "../tools/invariants.mjs";

const MKHEDRULI = /[Ⴀ-ჿᲐ-Ჿ]/;
const DOCS = new URL("../..", import.meta.url).pathname;
const GLOSSARY = join(DOCS, "guide", "i18n", "glossary.md");

const EN = fragments();
const ka = (id) => readFileSync(fragmentPath(id), "utf8").trim();
const generated = () => readFileSync(OUT, "utf8");

// ── the fragments ────────────────────────────────────────────────────────────

test("every English fragment has a Georgian counterpart", () => {
  const missing = EN.filter((f) => !existsSync(fragmentPath(f.id))).map((f) => f.id);
  expect(missing, `untranslated fragments:\n  ${missing.join("\n  ")}`).toEqual([]);
});

test("every fragment satisfies the six invariants", () => {
  const problems = [];
  for (const f of EN) {
    const k = ka(f.id);
    const [en, kk] =
      f.id === "header" ? [comparableHeader(f.body), comparableHeader(k)] : [f.body, k];
    problems.push(...check(f.id, en, kk));
  }
  expect(problems, `\n  ${problems.join("\n  ")}`).toEqual([]);
});

test("every fragment actually contains Georgian", () => {
  for (const f of EN) {
    expect(MKHEDRULI.test(ka(f.id)), `${f.id} contains no Georgian`).toBe(true);
  }
});

test("no fragment is a copy of its English original", () => {
  for (const f of EN) {
    expect(ka(f.id).trim(), `${f.id} is untranslated`).not.toBe(f.body.trim());
  }
});

// ── the generated file ───────────────────────────────────────────────────────

test("tutorial.ka.md is byte-identical to a fresh assemble", () => {
  expect(
    generated(),
    "tutorial.ka.md is generated — edit the fragment and re-run `node tools/assemble.mjs`"
  ).toBe(assemble());
});

// Re-run file-wide, not only per-fragment at assemble time, so a hand-edit of the
// generated file is caught even if assemble is never run again.
test("heading numbers match tutorial.md line for line", () => {
  const heads = (md) =>
    md.split("\n").filter((l) => /^#{2,3}\s/.test(l)).map((l) => {
      const { prefix, tag } = headingParts(l);
      return prefix + (tag ? ` ${tag}` : "");
    });
  expect(heads(generated())).toEqual(heads(readFileSync(join(DOCS, "tutorial.md"), "utf8")));
});

for (const [name, fn] of Object.entries({
  numbers, "inline code": codeSpans, "§-pointers": pointers, "link targets": linkTargets,
})) {
  test(`file-wide ${name} multiset matches tutorial.md`, () => {
    const en = readFileSync(join(DOCS, "tutorial.md"), "utf8");
    // From the Status line onward: the Georgian banner above it has no English
    // counterpart by design.
    expect(fn(comparableHeader(generated()))).toEqual(fn(comparableHeader(en)));
  });
}

test("file-wide table shape matches tutorial.md", () => {
  const en = readFileSync(join(DOCS, "tutorial.md"), "utf8");
  expect(tableShape(generated())).toEqual(tableShape(en));
});

test("the frozen provenance lines are reproduced verbatim", () => {
  const en = readFileSync(join(DOCS, "tutorial.md"), "utf8").split("\n");
  const out = generated();
  for (const marker of [STATUS, "**Doc set:**"]) {
    const line = en.find((l) => l.startsWith(marker));
    expect(out, `${marker} line is not verbatim in tutorial.ka.md`).toContain(line);
  }
});

test("the banner names the file as generated", () => {
  const head = generated().slice(0, generated().indexOf(STATUS));
  expect(MKHEDRULI.test(head), "the banner is not in Georgian").toBe(true);
  expect(head).toContain("tools/assemble.mjs");
  expect(head).toContain("tutorial.md");
});

// ── the shared glossary ──────────────────────────────────────────────────────

// Ported from docs/guide/tests/parity.test.js. `leg` is here because the guide's first
// bulk run omitted it from the glossary and produced three different Georgian words
// across three sections — ნაწილი (part), ეტაპი (stage), მხარე (side) — for the noun the
// whole two-sided strategy rests on. `except` excludes a different word that merely
// contains the term: the pinned "basis" is the perp-spot basis, while "a few basis
// points" is the unit, which Georgian correctly renders as საბაზისო პუნქტი.
const PINNED = [
  { term: "leg" }, { term: "spot" }, { term: "perp" }, { term: "funding" },
  { term: "mark price" }, { term: "basis", except: " point" }, { term: "maker" },
  { term: "taker" }, { term: "slippage" }, { term: "notional" }, { term: "hurdle" },
  { term: "utilization" }, { term: "settlement" }, { term: "ADL" },
];

const matcher = ({ term, except }) =>
  new RegExp(`\\b${term}\\b${except ? `(?! ?${except.trim()})` : ""}`, "i");

test("the pinned terms are actually in the glossary", () => {
  const latin = readFileSync(GLOSSARY, "utf8")
    .split("## Translate into Georgian")[0].toLowerCase();
  for (const { term } of PINNED) {
    expect(latin, `"${term}" is checked by tests but not pinned in the glossary`)
      .toContain(term.toLowerCase());
  }
});

test("pinned Latin terms survive into every fragment that uses them", () => {
  const misses = [];
  for (const f of EN) {
    const k = ka(f.id);
    for (const p of PINNED) {
      const re = matcher(p);
      if (re.test(f.body) && !re.test(k)) misses.push(`${f.id}: "${p.term}"`);
    }
  }
  expect(misses, `Latin terms lost in translation:\n  ${misses.join("\n  ")}`).toEqual([]);
});

// Two glossaries would drift, and drift is exactly the failure this project already
// paid for once. The tutorial and the guide read the same file or the test fails.
test("exactly one glossary.md exists under docs/", () => {
  const found = [];
  (function walk(dir) {
    for (const e of readdirSync(dir)) {
      if (e === "node_modules" || e === ".git") continue;
      const p = join(dir, e);
      if (statSync(p).isDirectory()) walk(p);
      else if (e === "glossary.md") found.push(p);
    }
  })(DOCS);
  expect(found).toEqual([GLOSSARY]);
});
