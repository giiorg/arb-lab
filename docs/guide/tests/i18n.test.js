import { test, expect } from "bun:test";
import { readGuide, loadPure } from "./extract.js";

test("pick returns the requested language", () => {
  const pure = loadPure();
  expect(pure.pick({ en: "Costs", ka: "ხარჯები" }, "ka")).toBe("ხარჯები");
});

test("pick falls back to English when a language is missing", () => {
  const pure = loadPure();
  expect(pure.pick({ en: "Costs" }, "ka")).toBe("Costs");
});

test("pick passes a plain string through unchanged", () => {
  const pure = loadPure();
  expect(pure.pick("Costs", "ka")).toBe("Costs");
});

test("every part and route title is a bundle with English", () => {
  const pure = loadPure();
  for (const p of pure.PARTS) expect(typeof p.title.en).toBe("string");
  for (const r of pure.ROUTES) expect(typeof r.title.en).toBe("string");
});

function bundles(node, path = "", out = []) {
  if (node && typeof node === "object") {
    if (typeof node.en === "string") out.push([path, node]);
    else for (const k of Object.keys(node)) bundles(node[k], path ? path + "." + k : k, out);
  }
  return out;
}

test("every UI string is a bundle with an English value", () => {
  const pure = loadPure();
  const found = bundles(pure.UI);
  expect(found.length).toBeGreaterThan(20);
  for (const [path, b] of found) {
    expect(typeof b.en, `UI.${path} has no English string`).toBe("string");
    expect(b.en.length, `UI.${path} is empty`).toBeGreaterThan(0);
  }
});

test("the verdict templates carry a points placeholder", () => {
  const pure = loadPure();
  expect(pure.UI.calc.verdictPass.en).toContain("{pts}");
  expect(pure.UI.calc.verdictFail.en).toContain("{pts}");
});

test("fmt substitutes named placeholders", () => {
  const pure = loadPure();
  expect(pure.fmt("Clears by {pts} points.", { pts: "1.19" })).toBe("Clears by 1.19 points.");
});

test("fmt leaves an unknown placeholder alone rather than printing undefined", () => {
  const pure = loadPure();
  expect(pure.fmt("a {missing} b", { pts: 1 })).toBe("a {missing} b");
});

test("preset labels and notes are bundles", () => {
  const pure = loadPure();
  for (const id of Object.keys(pure.PRESETS)) {
    expect(typeof pure.PRESETS[id].label.en, `${id} label`).toBe("string");
    expect(typeof pure.PRESETS[id].note.en, `${id} note`).toBe("string");
  }
});

const MKHEDRULI = /[Ⴀ-ჿᲐ-Ჿ]/;

test("every UI string has a Georgian value", () => {
  const pure = loadPure();
  for (const [path, b] of bundles(pure.UI)) {
    expect(typeof b.ka, `UI.${path} has no Georgian`).toBe("string");
  }
});

test("every route and part title has a Georgian value", () => {
  const pure = loadPure();
  for (const p of pure.PARTS) expect(typeof p.title.ka, `part ${p.id}`).toBe("string");
  for (const r of pure.ROUTES) expect(typeof r.title.ka, `route ${r.route}`).toBe("string");
});

test("the Georgian verdict templates keep the placeholder", () => {
  const pure = loadPure();
  expect(pure.UI.calc.verdictPass.ka).toContain("{pts}");
  expect(pure.UI.calc.verdictFail.ka).toContain("{pts}");
});

test("Georgian UI strings are actually Georgian where they are prose", () => {
  const pure = loadPure();
  expect(MKHEDRULI.test(pure.UI.calc.title.ka)).toBe(true);
  expect(MKHEDRULI.test(pure.UI.calc.verdictFail.ka)).toBe(true);
  expect(MKHEDRULI.test(pure.UI.searchPlace.ka)).toBe(true);
});

test("the Georgian calculator intro keeps both citation chips", () => {
  const pure = loadPure();
  expect(pure.UI.calc.intro.ka).toContain('data-src="core §2.1"');
  expect(pure.UI.calc.intro.ka).toContain('data-src="core §2.2"');
});

test("the Georgian preset notes keep their spec citations verbatim", () => {
  const pure = loadPure();
  for (const id of Object.keys(pure.PRESETS)) {
    expect(MKHEDRULI.test(pure.PRESETS[id].cite), `${id} cite was translated`).toBe(false);
    expect(typeof pure.PRESETS[id].label.ka, `${id} label`).toBe("string");
    expect(typeof pure.PRESETS[id].note.ka, `${id} note`).toBe("string");
  }
});

test("the calculator markup binds its text through i18n keys", () => {
  const html = readGuide();
  const calc = html.match(/<article data-route="edge\/calculator"[\s\S]*?<\/article>/)[0];
  for (const key of [
    "calc.title", "calc.field.rate", "calc.field.notional", "calc.field.hurdle"
  ]) {
    expect(calc, `missing data-i18n="${key}"`).toContain(`"${key}"`);
  }
});
