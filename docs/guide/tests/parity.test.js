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

test("at least one section is translated", () => {
  expect(translated().length).toBeGreaterThan(0);
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

test("glossary terms stay in Latin script inside Georgian prose", () => {
  const all = articles(readGuide());
  const checks = [
    { route: "world/funding", term: "funding" },
    { route: "world/perps", term: "spot" }
  ];
  for (const { route, term } of checks) {
    const ka = all.find((a) => a.route === route && a.lang === "ka");
    if (!ka) continue; // not translated yet
    expect(ka.body.toLowerCase(), `${route} lost the Latin term "${term}"`).toContain(term);
  }
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
