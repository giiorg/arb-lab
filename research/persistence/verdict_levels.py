#!/usr/bin/env python3
"""F60's measurement: what `summary.json`'s `verdict` block would say at each of
the four entry levels the file's own header declares.

`verdict` is derived from `per_symbol`, which `summarise.py:98` computes from
`eps(s, "0.001")` — an inline literal. The block emits four hurdle-provenance
fields and no level, so the entry level is unrecoverable from the artifact.

Run from the repo root:  python3 research/persistence/verdict_levels.py

Two controls, and both must pass before any row below is believed:

  CONTROL           level 0.001 reproduces the COMMITTED verdict on all four
                    fields at all three costs. Without it the sweep is measuring
                    this script rather than the artifact.
  NEGATIVE CONTROL  level 0.0005 does NOT reproduce it. Without it a harness
                    that returns the committed values whatever it is asked
                    would also print PASS.

Expect the published row (0.001) to be the unique MAXIMUM of
`beat_hurdle_symbols` at 21bp and 29bp, and the only row reading 0/56 at 90bp
where level 0.002 reads 2/49.

This exists so that §100 §14 debt 10's fix can be VERIFIED rather than trusted:
after `summarise.py` learns to emit the level, re-run this and confirm the
published row still matches the level the artifact now names.
"""
import json, datetime as dt
from decimal import Decimal as D

data = json.load(open("research/persistence/episodes.json"))
committed = json.load(open("research/persistence/summary.json"))
COSTS = ["0.0021", "0.0029", "0.0090"]
LEVELS = ["0.0002", "0.0005", "0.001", "0.002"]
HURDLE = D("4.6")
CAP, LEG = D(2000), D(1000)

def eps(sym, level):
    for lv in sym["levels"]:
        if lv["entry"] == level:
            return lv["episodes"] or []
    return []

def days(s):
    a = dt.datetime.fromisoformat(s["first"].replace("Z", "+00:00"))
    b = dt.datetime.fromisoformat(s["last"].replace("Z", "+00:00"))
    return max(1, (b - a).days)

def verdict_at(level):
    rows = []
    for s in data:
        e = eps(s, level)
        if not e:
            continue
        d = days(s)
        rec = {}
        for c in COSTS:
            tot = sum(D(x["accrued"]) * LEG - D(c) * LEG for x in e)
            rec[c] = ((tot / CAP * D(365) / D(d) * 100).quantize(D("0.1")),
                      tot.quantize(D("0.01")))
        rows.append(rec)
    outv = {}
    for c in COSTS:
        v = sorted(r[c][0] for r in rows)
        pos = sum(1 for r in rows if r[c][1] > 0)
        beat = sum(1 for x in v if x > HURDLE)
        outv[c] = dict(median_annual_pct=str(v[len(v)//2]), positive_symbols=pos,
                       beat_hurdle_symbols=beat, total_symbols=len(v))
    return outv

res = {lv: verdict_at(lv) for lv in LEVELS}

# --- CONTROL: level 0.001 must reproduce the committed verdict byte-for-byte
ok = True
for c in COSTS:
    got, want = res["0.001"][c], committed["verdict"][c]
    for k in ("median_annual_pct", "positive_symbols", "beat_hurdle_symbols", "total_symbols"):
        if got[k] != want[k]:
            ok = False
            print(f"CONTROL MISMATCH cost {c} {k}: got {got[k]!r} want {want[k]!r}")
print(f"CONTROL (level 0.001 == committed verdict): {'PASS' if ok else 'FAIL'}")

# --- NEGATIVE CONTROL: some other level must NOT reproduce it
diff = any(res["0.0005"][c] != {k: v for k, v in res["0.001"][c].items()} for c in COSTS)
print(f"NEGATIVE CONTROL (level 0.0005 != level 0.001): {'PASS' if diff else 'FAIL'}")

print()
hdr = f"{'entry level':>12}" + "".join(f"{'cost '+c:>26}" for c in COSTS)
print(hdr); print("-"*len(hdr))
print(f"{'':>12}" + "".join(f"{'beat/total   median  pos':>26}" for c in COSTS))
for lv in LEVELS:
    mark = "  <- published" if lv == "0.001" else ""
    line = f"{lv:>12}"
    for c in COSTS:
        r = res[lv][c]
        line += f"{r['beat_hurdle_symbols']:>10}/{r['total_symbols']:<4}{r['median_annual_pct']:>7}{r['positive_symbols']:>5}"
    print(line + mark)
