#!/usr/bin/env node
// Splits docs/tutorial.md into per-section fragments.
//
//   node tools/split.mjs list          -> id, bytes, heading (one per line)
//   node tools/split.mjs get <id>      -> the translatable body of one fragment
//   node tools/split.mjs parent <id>   -> its parent "## Part N — …" heading, for context
//
// A fragment is one `###` subsection, or the `##` section itself when it has no `###`
// children, plus the preamble before the first `##` (id `header`). The spans tile the
// file exactly, so assembling the fragments in document order reproduces it.
//
// Fragment IDs are spec section numbers (`s-3-9`, `part-8`) rather than ordinals: an
// English edit that inserts or deletes a section must not rename its neighbours, or the
// per-section regeneration this whole design rests on stops working.
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
export const TUTORIAL = join(HERE, "..", "..", "tutorial.md");

const slug = (s) =>
  s.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");

function idFor(heading) {
  let m = /^###\s+(\d+)\.(\d+)\b/.exec(heading);
  if (m) return `s-${m[1]}-${m[2]}`;
  m = /^##\s+Part\s+(\d+)\b/.exec(heading);
  if (m) return `part-${m[1]}`;
  m = /^##\s+(.+)$/.exec(heading);
  if (m) return slug(m[1]);
  throw new Error(`cannot derive an id for heading: ${heading}`);
}

// Trailing blank lines and `---` rules are structural separators, not prose. They are
// held back from the translator and re-attached mechanically at assemble time, so a
// model can never drop a horizontal rule or collapse the spacing between parts.
function splitTail(text) {
  const m = /(?:\n[ \t]*(?:---)?[ \t]*)+$/.exec(text);
  if (!m) return [text, ""];
  return [text.slice(0, m.index), text.slice(m.index)];
}

export function fragments(md = readFileSync(TUTORIAL, "utf8")) {
  const lines = md.split("\n");
  const starts = [];
  for (let i = 0; i < lines.length; i++) {
    if (/^###?\s/.test(lines[i])) starts.push(i);
  }
  if (!starts.length) throw new Error("tutorial.md has no ## or ### headings");

  const out = [];
  const push = (id, heading, parent, from, to) => {
    const [body, tail] = splitTail(lines.slice(from, to).join("\n"));
    out.push({ id, heading, parent, body, tail });
  };

  push("header", null, null, 0, starts[0]);

  let parent = null;
  for (let k = 0; k < starts.length; k++) {
    const from = starts[k];
    const to = k + 1 < starts.length ? starts[k + 1] : lines.length;
    const heading = lines[from];
    if (heading.startsWith("## ")) {
      parent = heading;
      // A `##` with `###` children contributes no prose of its own worth a separate
      // call only when the gap is empty; here it always has an intro paragraph or is
      // childless, so it is always its own fragment.
      const hasChildren = k + 1 < starts.length && lines[starts[k + 1]].startsWith("### ");
      push(idFor(heading), heading, null, from, to);
      if (hasChildren) continue;
    } else {
      push(idFor(heading), heading, parent, from, to);
    }
  }

  const seen = new Set();
  for (const f of out) {
    if (seen.has(f.id)) throw new Error(`duplicate fragment id: ${f.id}`);
    seen.add(f.id);
  }
  return out;
}

export const byId = (id, frags = fragments()) => {
  const f = frags.find((x) => x.id === id);
  if (!f) throw new Error(`no fragment with id "${id}"`);
  return f;
};

// Only when run directly: importing this module must not read process.argv, or
// `assemble.mjs --check` gets parsed as a split.mjs subcommand.
const [cmd, id] = process.argv[1] === fileURLToPath(import.meta.url)
  ? process.argv.slice(2)
  : [];
if (cmd === "list") {
  for (const f of fragments()) {
    process.stdout.write(
      `${f.id}\t${Buffer.byteLength(f.body)}\t${f.heading ?? "(preamble)"}\n`
    );
  }
} else if (cmd === "get") {
  process.stdout.write(byId(id).body);
} else if (cmd === "parent") {
  process.stdout.write(byId(id).parent ?? "");
} else if (cmd) {
  console.error("usage: split.mjs list | split.mjs get <id> | split.mjs parent <id>");
  process.exit(2);
}
