#!/usr/bin/env python3
"""Task #225 -- the redaction pass over this directory's captures.

Policy is research/venue-probes/README.md. Account-identity fields are replaced
with the string "<redacted>", which is deliberately NOT a plausible value of any
of them, so a reader cannot mistake the marker for data. Every monetary and
position field is KEPT -- they ARE the measurement, and record §93's
residual-0E-8 decompositions cannot be reproduced without them.

Idempotent: re-running finds nothing to do and says so, rather than reporting a
successful pass over already-clean files, which would read the same.

THE CHECK THAT KEEPS IT HONEST (README): a redaction is a mutation of the
evidence, so re-run the consumer and confirm it still reaches the same number.
Here the consumer is analyze225.py:

    python3 analyze225.py > before.txt      # BEFORE redacting
    python3 redact225.py
    python3 analyze225.py | diff before.txt -

A scrub that silently removed an INPUT would otherwise look identical to one
that removed metadata.
"""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
CAPTURES = os.path.join(HERE, "captures")
MARK = "<redacted>"
FIELDS = ("accountAlias", "orderId", "clientOrderId", "orderLinkId")


def scrub(node, hits):
    if isinstance(node, dict):
        for k in list(node):
            if k in FIELDS and node[k] != MARK:
                hits.append(k)
                node[k] = MARK
            else:
                scrub(node[k], hits)
    elif isinstance(node, list):
        for v in node:
            scrub(v, hits)


def main():
    total = 0
    for name in sorted(os.listdir(CAPTURES)):
        if not name.endswith(".json"):
            continue
        path = os.path.join(CAPTURES, name)
        with open(path) as fh:
            doc = json.load(fh)
        hits = []
        scrub(doc, hits)
        if hits:
            with open(path, "w") as fh:
                json.dump(doc, fh, indent=1, sort_keys=True)
                fh.write("\n")
            counts = {f: hits.count(f) for f in sorted(set(hits))}
            print(f"  {name}: redacted {len(hits)} -> {counts}")
            total += len(hits)
        else:
            print(f"  {name}: clean (nothing to redact)")
    print(f"\n{total} field(s) redacted.")
    if total:
        print("NOW RUN THE REPRODUCTION CHECK -- a redaction is a mutation:")
        print("    python3 analyze225.py | diff <before>.txt -")
    return 0


if __name__ == "__main__":
    sys.exit(main())
