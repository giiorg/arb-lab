import { test, expect } from "bun:test";
import { readGuide, loadPure } from "./extract.js";

const MKHEDRULI = /[Ⴀ-ჿᲐ-Ჿ]/;

function articles(html) {
  const out = [];
  const re = /<article data-route="([^"]+)"(?: lang="(en|ka)")?>([\s\S]*?)<\/article>/g;
  for (const m of html.matchAll(re)) out.push({ route: m[1], lang: m[2] || null, body: m[3] });
  return out;
}

const cites = (body) => [...body.matchAll(/data-src="([^"]+)"/g)].map((m) => m[1]).sort();
const translated = () => articles(readGuide()).filter((a) => a.lang === "ka");

test("every prose route has a Georgian article", () => {
  const ka = new Set(translated().map((a) => a.route));
  for (const r of loadPure().ROUTES) {
    if (r.route === "edge/calculator") continue; // a control, localized in place
    expect(ka.has(r.route), `${r.route} has no Georgian article`).toBe(true);
  }
});

test("every translated section has an English sibling", () => {
  const en = new Set(articles(readGuide()).filter((a) => a.lang === "en").map((a) => a.route));
  for (const ka of translated()) expect(en.has(ka.route), `${ka.route} has no en sibling`).toBe(true);
});

test("citation chips are identical between languages", () => {
  const all = articles(readGuide());
  for (const ka of translated()) {
    const en = all.find((a) => a.route === ka.route && a.lang === "en");
    expect(cites(ka.body), `citations differ for ${ka.route}`).toEqual(cites(en.body));
  }
});

test("no citation chip text is Georgian", () => {
  for (const ka of translated()) {
    for (const m of ka.body.matchAll(/<span class="cite" data-src="[^"]*">([^<]*)<\/span>/g)) {
      expect(MKHEDRULI.test(m[1]), `translated citation in ${ka.route}: ${m[1]}`).toBe(false);
    }
  }
});

test("every translated section actually contains Georgian", () => {
  for (const ka of translated()) {
    expect(MKHEDRULI.test(ka.body), `${ka.route} contains no Georgian`).toBe(true);
  }
});

test("no translated section is a copy of its English sibling", () => {
  const all = articles(readGuide());
  for (const ka of translated()) {
    const en = all.find((a) => a.route === ka.route && a.lang === "en");
    expect(ka.body.trim(), `${ka.route} is untranslated`).not.toBe(en.body.trim());
  }
});

// Terms the glossary pins to Latin script. Checked per route rather than globally:
// a term used in an English section must survive into its Georgian sibling, or the
// model paraphrased it away. "leg" is here because the first bulk run omitted it
// from the glossary and produced three different Georgian words across three
// sections — ნაწილი (part), ეტაპი (stage), მხარე (side) — for the noun the whole
// two-sided strategy rests on.
// `except` excludes a different word that merely contains the term: the pinned
// "basis" is the perp-spot basis, while "a few basis points" is the unit (bps),
// which Georgian correctly renders as საბაზისო პუნქტი.
const PINNED = [
  { term: "leg" }, { term: "spot" }, { term: "perp" }, { term: "funding" },
  { term: "mark price" }, { term: "basis", except: " point" }, { term: "maker" },
  { term: "taker" }, { term: "slippage" }, { term: "notional" }, { term: "hurdle" },
  { term: "utilization" }, { term: "settlement" }, { term: "ADL" }
];

const matcher = ({ term, except }) =>
  new RegExp(`\\b${term}\\b${except ? `(?! ?${except.trim()})` : ""}`, "i");

test("the pinned terms are actually in the glossary", async () => {
  const glossary = await Bun.file(
    new URL("../i18n/glossary.md", import.meta.url).pathname
  ).text();
  const latinSection = glossary.split("## Translate into Georgian")[0].toLowerCase();
  for (const { term } of PINNED) {
    expect(latinSection, `"${term}" is checked by tests but not pinned in the glossary`)
      .toContain(term.toLowerCase());
  }
});

test("pinned Latin terms survive into every Georgian section that uses them", () => {
  const all = articles(readGuide());
  const misses = [];
  for (const ka of all.filter((a) => a.lang === "ka")) {
    const en = all.find((a) => a.route === ka.route && a.lang === "en");
    if (!en) continue;
    for (const pinned of PINNED) {
      const re = matcher(pinned);
      if (re.test(en.body) && !re.test(ka.body)) misses.push(`${ka.route}: "${pinned.term}"`);
    }
  }
  expect(misses, `Latin terms lost in translation:\n  ${misses.join("\n  ")}`).toEqual([]);
});

test("each route has at most one article per language", () => {
  const seen = new Set();
  for (const a of articles(readGuide())) {
    const key = a.route + "/" + a.lang;
    expect(seen.has(key), `duplicate article ${key}`).toBe(false);
    seen.add(key);
  }
});

test("every declared route still has an English article", () => {
  const en = new Set(articles(readGuide()).filter((a) => a.lang === "en").map((a) => a.route));
  for (const r of loadPure().ROUTES) {
    if (r.route === "edge/calculator") continue; // a control, localized in place
    expect(en.has(r.route), `${r.route} has no English article`).toBe(true);
  }
});
