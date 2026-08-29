#!/usr/bin/env python3
"""Derive the persistence summary from episodes.json (record §99).

Exact decimal arithmetic throughout: accrued figures are decimal strings
straight out of fixedpoint.Rate, so no float touches a money quantity.
Run from the repo root:  python3 research/persistence/summarise.py
"""
import json, datetime as dt, collections
from decimal import Decimal as D

SRC = "research/persistence/episodes.json"
OUT = "research/persistence/summary.json"

# The three round-trip readings the record contrasts. The screening's own
# constant is FEE-ONLY; the other two are the spec's model and the single
# live observation, which HANDOFF states is not a measurement.
COSTS = ["0.0021", "0.0029", "0.0090"]
LEVELS = ["0.0002", "0.0005", "0.001", "0.002"]

data = json.load(open(SRC))


def eps(sym, level):
    for lv in sym["levels"]:
        if lv["entry"] == level:
            return lv["episodes"] or []
    return []


def days(s):
    a = dt.datetime.fromisoformat(s["first"].replace("Z", "+00:00"))
    b = dt.datetime.fromisoformat(s["last"].replace("Z", "+00:00"))
    return max(1, (b - a).days)


def pctile(v, q):
    v = sorted(v)
    return v[min(len(v) - 1, (len(v) * q) // 100)]


out = {
    "generated_from": SRC,
    "costs": COSTS,
    "levels": LEVELS,
    "symbols_eligible": len(data),
    "symbols_shortlist": sum(1 for s in data if s["shortlist"]),
    "span": {
        "first": min(s["first"] for s in data),
        "last": max(s["last"] for s in data),
    },
    "matched_settlements": sum(s["matched"] for s in data),
    "pooled": [],
    "run_vs_breakeven": [],
    "per_symbol": [],
}

# --- pooled run-length and break-even, per level -------------------------
for lv in LEVELS:
    allep = [e for s in data for e in eps(s, lv)]
    if not allep:
        continue
    runs = [e["run"] for e in allep]
    accs = [D(e["accrued"]) for e in allep]
    row = {
        "entry": lv,
        "episodes": len(allep),
        "run_mean": str(round(sum(runs) / len(runs), 3)),
        "run_p50": pctile(runs, 50),
        "run_p75": pctile(runs, 75),
        "run_p90": pctile(runs, 90),
        "run_max": max(runs),
        "mean_gross": str(sum(accs) / len(accs)),
        "wins": {}, "mean_net": {},
    }
    for c in COSTS:
        cd = D(c)
        row["wins"][c] = sum(1 for a in accs if a > cd)
        row["mean_net"][c] = str((sum(accs) - cd * len(accs)) / len(accs))
    out["pooled"].append(row)

# --- run length is NOT the break-even question ---------------------------
by_run = collections.defaultdict(lambda: [0, 0])
for s in data:
    for e in eps(s, "0.0005"):
        b = min(e["run"], 12)
        by_run[b][0] += 1
        if D(e["accrued"]) > D("0.0021"):
            by_run[b][1] += 1
for r in sorted(by_run):
    n, w = by_run[r]
    out["run_vs_breakeven"].append(
        {"run": "12+" if r == 12 else r, "episodes": n, "cleared_cost": w}
    )

# --- per symbol, at the 10bp level, annualised on $2000 -------------------
CAP, LEG = D(2000), D(1000)
for s in data:
    e = eps(s, "0.001")
    if not e:
        continue
    d = days(s)
    rec = {
        "symbol": s["symbol"], "shortlist": s["shortlist"],
        "episodes": len(e), "days": d, "modal_minutes": s["modal_minutes"],
        "first": s["first"], "last": s["last"],
        "net_usd_1000_per_leg": {}, "annual_pct_on_2000": {},
    }
    for c in COSTS:
        tot = sum(D(x["accrued"]) * LEG - D(c) * LEG for x in e)
        rec["net_usd_1000_per_leg"][c] = str(tot.quantize(D("0.01")))
        rec["annual_pct_on_2000"][c] = str(
            (tot / CAP * D(365) / D(d) * 100).quantize(D("0.1"))
        )
    out["per_symbol"].append(rec)
out["per_symbol"].sort(key=lambda r: -D(r["annual_pct_on_2000"]["0.0021"]))

json.dump(out, open(OUT, "w"), indent=1)
print(f"wrote {OUT}")

# --- console report ------------------------------------------------------
print(f"\neligible {out['symbols_eligible']} symbols "
      f"({out['symbols_shortlist']} shortlist), "
      f"{out['matched_settlements']} matched settlements, "
      f"{out['span']['first'][:10]} -> {out['span']['last'][:10]}\n")
print(f"{'entry':>7}{'eps':>7}{'mean':>7}{'p50':>5}{'p90':>5}{'max':>5}"
      + "".join(f"{'net@'+c:>13}" for c in COSTS))
print("-" * 74)
for r in out["pooled"]:
    print(f"{r['entry']:>7}{r['episodes']:>7}{r['run_mean']:>7}"
          f"{r['run_p50']:>5}{r['run_p90']:>5}{r['run_max']:>5}"
          + "".join(f"{D(r['mean_net'][c]):>+13.6f}" for c in COSTS))
print()
# The sign test runs on the RAW net, never on the rounded percentage: three
# symbols earn a couple of dollars over four years, which is positive and
# rounds to +0.0%/yr. Reported beside the hurdle count, because "positive"
# and "worth doing" are two different questions and only the second is the
# gate's (build-spec §55: the benchmark is a dated, haircut-adjusted input;
# HURDLE_PCT below is illustrative and is NOT a calibrated constant).
HURDLE_PCT = D("4.6")
for c in COSTS:
    v = sorted(D(r["annual_pct_on_2000"][c]) for r in out["per_symbol"])
    pos = sum(1 for r in out["per_symbol"] if D(r["net_usd_1000_per_leg"][c]) > 0)
    beat = sum(1 for x in v if x > HURDLE_PCT)
    print(f"  cost {c}: median {v[len(v)//2]:+7.1f}%/yr  best {v[-1]:+7.1f}%  "
          f"worst {v[0]:+8.1f}%  positive {pos}/{len(v)}  "
          f"beat {HURDLE_PCT}% hurdle {beat}/{len(v)}")
    out.setdefault("verdict", {})[c] = {
        "median_annual_pct": str(v[len(v) // 2]),
        "positive_symbols": pos,
        "beat_hurdle_symbols": beat,
        "total_symbols": len(v),
    }
json.dump(out, open(OUT, "w"), indent=1)
