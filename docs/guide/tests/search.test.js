import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

const RECORDS = [
  { route: "world/perps", title: "Spot, futures, perpetuals", text: "A perpetual has no expiry and uses a funding rate." },
  { route: "edge/kill",   title: "The kill math",             text: "Round trip at VIP0 taker is about 0.38% of notional." }
];

test("finds a term that appears only in a non-first record", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "notional");
  expect(hits.length).toBe(1);
  expect(hits[0].route).toBe("edge/kill");
});

test("matches titles as well as body text", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "kill");
  expect(hits[0].route).toBe("edge/kill");
});

test("is case-insensitive", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "PERPETUAL");
  expect(hits[0].route).toBe("world/perps");
});

test("returns nothing for a blank or one-character query", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(RECORDS);
  expect(pure.searchIndex(index, "")).toEqual([]);
  expect(pure.searchIndex(index, "a")).toEqual([]);
});

test("returns a snippet containing the match", () => {
  const pure = loadPure();
  const hits = pure.searchIndex(pure.buildSearchIndex(RECORDS), "funding");
  expect(hits[0].snippet.toLowerCase()).toContain("funding");
});

const BILINGUAL = [
  { route: "world/funding", lang: "en", title: "The funding rate", text: "The crowded side pays the other side." },
  { route: "world/funding", lang: "ka", title: "funding rate", text: "ბირჟა ხალხმრავალ მხარეს აიძულებს გადახდას." }
];

test("search returns only results in the requested language", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(BILINGUAL);
  const en = pure.searchIndex(index, "crowded", "en");
  expect(en.length).toBe(1);
  expect(en[0].lang).toBe("en");
  expect(pure.searchIndex(index, "crowded", "ka")).toEqual([]);
});

test("a Latin glossary term finds the Georgian section too", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex(BILINGUAL);
  const hits = pure.searchIndex(index, "funding", "ka");
  expect(hits.length).toBe(1);
  expect(hits[0].lang).toBe("ka");
});

test("records without a language default to English", () => {
  const pure = loadPure();
  const index = pure.buildSearchIndex([{ route: "a/b", title: "T", text: "body text here" }]);
  expect(pure.searchIndex(index, "body", "en").length).toBe(1);
});
