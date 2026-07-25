import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

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
