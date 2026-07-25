#!/usr/bin/env node
// Extracts and splices per-language <article> blocks in index.html.
//
//   node tools/i18n.mjs extract <route> <lang>   -> prints the fragment
//   node tools/i18n.mjs splice  <route>          -> writes i18n/ka/<route>.html in
//
// Splice is idempotent: it inserts the ka article after its en sibling on the first
// run and replaces it in place afterwards, so re-translating one section is safe to
// repeat. It refuses to write a fragment whose citation chips do not match the
// English original — catching a mangled translation at build time, not review time.
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const GUIDE = join(HERE, "..", "index.html");

const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const articleRe = (route, lang) =>
  new RegExp(`<article data-route="${esc(route)}" lang="${lang}">[\\s\\S]*?<\\/article>`);

export const fragmentPath = (route) =>
  join(HERE, "..", "i18n", "ka", route.replace(/\//g, "__") + ".html");

export function extract(html, route, lang) {
  const m = html.match(articleRe(route, lang));
  if (!m) throw new Error(`no article for route "${route}" in language "${lang}"`);
  return m[0];
}

export const cites = (fragment) =>
  [...fragment.matchAll(/data-src="([^"]+)"/g)].map((m) => m[1]).sort();

function splice(route) {
  const html = readFileSync(GUIDE, "utf8");
  const en = extract(html, route, "en");
  let ka = readFileSync(fragmentPath(route), "utf8").trim();

  // Models sometimes wrap output in a code fence despite being told not to.
  ka = ka.replace(/^```[a-z]*\n?/i, "").replace(/\n?```$/, "").trim();

  const open = `<article data-route="${route}" lang="ka">`;
  if (!ka.startsWith(open)) {
    throw new Error(`fragment for ${route} must start with ${open}\n  got: ${ka.slice(0, 90)}`);
  }
  if (!ka.endsWith("</article>")) throw new Error(`fragment for ${route} is truncated`);

  const a = cites(en).join("|");
  const b = cites(ka).join("|");
  if (a !== b) throw new Error(`citation chips differ for ${route}\n  en: ${a}\n  ka: ${b}`);

  const existing = html.match(articleRe(route, "ka"));
  // Function replacements: a plain string would let $& or $1 inside translated
  // prose be read as a replacement pattern and corrupt the output.
  const next = existing
    ? html.replace(articleRe(route, "ka"), () => ka)
    : html.replace(en, () => en + "\n\n      " + ka);
  writeFileSync(GUIDE, next);
  console.log(`${existing ? "replaced" : "inserted"} ka article for ${route}`);
}

const [cmd, route, lang] = process.argv.slice(2);
if (cmd === "extract") {
  process.stdout.write(extract(readFileSync(GUIDE, "utf8"), route, lang || "en"));
} else if (cmd === "splice") {
  mkdirSync(dirname(fragmentPath(route)), { recursive: true });
  splice(route);
} else if (cmd) {
  console.error("usage: i18n.mjs extract <route> [lang] | i18n.mjs splice <route>");
  process.exit(2);
}
