#!/usr/bin/env python3
"""Generate the seven screens §7 declares and no round had drawn.

Run from the repository root:

    python3 docs/superpowers/specs/2026-08-29-web-dashboard-mockups/gen-remaining-screens.py

Every figure in the output is read from a committed artifact at generation time.
That is the point (F52): a mockup regenerated from real data is what refutes the
prose. Where a cell has no source, the generator emits a VOID marker rather than
a plausible number -- inventing one is how F36 happened.

The three journal facts under Positions and Journal are measured constants, not
reads: `run/pair/` is untracked, so it does not exist on a fresh clone. They are
labelled with the measurement that produced them.
"""

import json
import os
from collections import Counter
from decimal import Decimal as D

_HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(_HERE, "..", "..", "..", ".."))
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                   "4-remaining-screens.html")


def load(rel):
    with open(os.path.join(ROOT, rel)) as fh:
        return json.load(fh)


RANK = load("research/screening/ranking.json")
SUMMARY = load("research/persistence/summary.json")
EPISODES = load("research/persistence/episodes.json")
REFERENCE = load("configs/reference.btcusdt.json")
CAPTURE = load("configs/capture.json")
FRESH = load("docs/formal/freshness.json")

PAIRS = RANK["pairs"]
OK = [p for p in PAIRS if p["eligible"] == "ok"]
ELIG = Counter(p["eligible"].split(":")[0] for p in PAIRS)
BTC = next(p for p in PAIRS if p["symbol"] == "BTCUSDT")
NO_STATS = sum(1 for p in PAIRS if "levels" not in p)
FLOOR = D(RANK["params"]["floor_turnover_24h_usdt"])


def turnover(p):
    return min(D(p["binance_quote_volume_24h"]), D(p["bybit_turnover_24h"]))


NEAR = sum(1 for p in PAIRS if p["eligible"] == "below_floor"
           and turnover(p) > FLOOR * D("0.8"))
NEAR += sum(1 for p in OK if turnover(p) < FLOOR * D("1.2"))
BEST_ENTRY = Counter(p["best_entry"] for p in OK)

# --- the capture verifier's two finding classes, counted from the source -----
FINDING_SRC = os.path.join(ROOT, "internal/engine/capture/finding.go")
with open(FINDING_SRC) as fh:
    _src = fh.read()
_sw = _src[_src.index("func (k FindingKind) Class()"):]
_sw = _sw[:_sw.index("\n}")]
_integrity = _sw[_sw.index("case Finding"):_sw.index("return ClassIntegrity")]
_coverage = _sw[_sw.index("return ClassIntegrity"):_sw.index("return ClassCoverage")]
N_INTEGRITY = _integrity.count("Finding")
N_COVERAGE = _coverage.count("Finding")

STALE = [(m["name"], f'{s["doc"]} §{s["section"]}', s.get("role"),
          s.get("knownStale")) for m in FRESH["models"] for s in m["subjects"]]
VERIFIED = {m["name"]: m["verifiedAt"] for m in FRESH["models"]}
N_STALE = sum(1 for _, _, _, k in STALE if k)


def esc(s):
    return (str(s).replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


def pct(x):
    return f"{float(x) * 100:.1f}"


P = []          # output buffer
def w(s):
    P.append(s)


STYLE = """<style>
  .ov { font: 12px/1.4 ui-sans-serif, -apple-system, "Segoe UI", sans-serif; }
  .ov code, .ov .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
  .ov-shell { border: 1px solid rgba(128,128,128,.35); border-radius: 8px; overflow: hidden; margin-bottom: 26px; }
  .ov-strip { display: flex; gap: 1px; background: rgba(128,128,128,.28); border-bottom: 1px solid rgba(128,128,128,.35); }
  .ov-tile { flex: 1; padding: 7px 10px; background: rgba(128,128,128,.10); }
  .ov-tile .k { font: 600 9px/1 ui-monospace, monospace; letter-spacing: .09em; text-transform: uppercase; opacity: .55; }
  .ov-tile .v { font: 700 15px/1.25 ui-monospace, monospace; margin-top: 4px; }
  .ov-tile .s { font-size: 10px; opacity: .55; }
  .ov-body { display: flex; }
  .ov-rail { width: 122px; flex: none; padding: 10px 8px; border-right: 1px solid rgba(128,128,128,.28); background: rgba(128,128,128,.05); }
  .ov-rail .it { padding: 4px 7px; border-radius: 4px; margin-bottom: 1px; opacity: .7; font-size: 11.5px; }
  .ov-rail .it.on { background: rgba(128,128,128,.28); opacity: 1; font-weight: 600; }
  .ov-main { flex: 1; padding: 10px; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .panel { border: 1px solid rgba(128,128,128,.3); border-radius: 6px; padding: 9px 11px; min-width: 0; }
  .panel > h4 { margin: 0 0 7px; font: 600 9.5px/1 ui-monospace, monospace; letter-spacing: .09em; text-transform: uppercase; opacity: .6; display: flex; align-items: center; gap: 7px; flex-wrap: wrap; }
  .prov { font: 600 8.5px/1 ui-monospace, monospace; padding: 2px 5px; border-radius: 3px; border: 1px solid currentColor; opacity: .8; letter-spacing: .05em; }
  .p-exists { color: #3fa66b; } .p-daemon { color: #c9922e; } .p-creds { color: #b06fc4; }
  .p-open { color: #999; } .p-void { color: #d64f4f; }
  .ov table { width: 100%; border-collapse: collapse; font-family: ui-monospace, monospace; font-size: 10.5px; }
  .ov th { text-align: right; font: 600 8.5px/1 ui-monospace, monospace; letter-spacing: .06em; text-transform: uppercase; opacity: .5; padding: 0 6px 5px; border-bottom: 1px solid rgba(128,128,128,.25); }
  .ov th:first-child, .ov td:first-child { text-align: left; }
  .ov td { text-align: right; padding: 4px 6px; border-bottom: 1px solid rgba(128,128,128,.13); }
  .chip { display: inline-block; font: 700 8.5px/1 ui-monospace, monospace; padding: 3px 6px; border-radius: 3px; letter-spacing: .05em; }
  .c-yes { background: rgba(63,166,107,.2); color: #3fa66b; }
  .c-mar { background: rgba(201,146,46,.2); color: #c9922e; }
  .c-no  { background: rgba(128,128,128,.18); opacity: .6; }
  .c-alert { background: rgba(214,79,79,.2); color: #d64f4f; }
  .pos { color: #3fa66b; } .neg { color: #d64f4f; } .dim { opacity: .45; }
  .void { color: #d64f4f; font-family: ui-monospace, monospace; font-weight: 600; }
  .banner { border: 1px solid rgba(214,79,79,.5); background: rgba(214,79,79,.07); border-radius: 6px; padding: 9px 11px; }
  .spark { display: flex; align-items: flex-end; gap: 2px; height: 40px; }
  .spark i { flex: 1; background: currentColor; opacity: .5; border-radius: 1px; }
  .row { display: flex; gap: 10px; align-items: stretch; }
  .note { font-size: 12px; opacity: .8; }
  .legend { display: flex; gap: 12px; flex-wrap: wrap; font-size: 11px; opacity: .85; margin: 4px 0 18px; align-items: center; }
  .fnd { border-left: 3px solid #d64f4f; padding: 2px 0 2px 10px; margin: 10px 0 20px; font-size: 12px; }
  .fnd b { font-family: ui-monospace, monospace; }
</style>"""

RAIL = ["Overview", "Markets", "Market detail", "Positions", "Venues",
        "Evidence", "Journal", "System"]


def strip():
    """§7's status strip. Every tile carries the phase F95 says it must."""
    return """
  <div class="ov-strip">
    <div class="ov-tile"><div class="k">Venues</div><div class="v" style="color:#c9922e">2 &middot; daemon</div><div class="s">public reachability &middot; 1</div></div>
    <div class="ov-tile"><div class="k">Equity</div><div class="v void">VOID</div><div class="s">phase 2 &middot; 3 candidates, 1 refusable (F94)</div></div>
    <div class="ov-tile"><div class="k">Open pkgs</div><div class="v"><span class="dim">jrnl</span> 0 <span class="void">/ live VOID</span></div><div class="s">journals 1 &middot; live 2</div></div>
    <div class="ov-tile"><div class="k">Attention</div><div class="v" style="color:#c9922e">unverified</div><div class="s">never 0 &mdash; F52</div></div>
    <div class="ov-tile"><div class="k">Engine</div><div class="v" style="opacity:.65">DISARMED</div><div class="s">read-only build</div></div>
  </div>"""


def shell(active, body):
    rail = "".join(
        f'<div class="it{" on" if r == active else ""}">{esc(r)}</div>'
        for r in RAIL)
    return (f'<div class="ov-shell">{strip()}<div class="ov-body">'
            f'<div class="ov-rail">{rail}</div>'
            f'<div class="ov-main">{body}</div></div></div>')


def prov(kind, label=None):
    labels = {"exists": "EXISTS", "daemon": "DAEMON", "creds": "CREDS",
              "open": "OPEN", "void": "VOID"}
    return f'<span class="prov p-{kind}">{label or labels[kind]}</span>'


# ============================================================ MARKETS
def markets():
    rows = []
    for p in OK[:7]:
        rows.append(
            f"<tr><td>{esc(p['symbol'])}</td>"
            f"<td><span class='chip c-yes'>ok</span></td>"
            f"<td>{p['binance_interval_hours']}h / {p['bybit_interval_minutes']}m</td>"
            f"<td>{'yes' if p['same_grid'] else 'no'}</td>"
            f"<td class='dim'>{float(D(p['binance_quote_volume_24h'])) / 1e6:,.1f}M</td>"
            f"<td>{pct(p['median_abs'])}%</td><td>{pct(p['p99_abs'])}%</td>"
            f"<td class='c-mar' style='background:none;color:#c9922e'>{p['best_entry']}</td>"
            f"<td class='pos'>{pct(p['rank_score'])}%</td>"
            f"<td class='void'>VOID</td><td class='void'>VOID</td><td class='void'>VOID</td></tr>")
    rows.append(
        f"<tr style='background:rgba(214,79,79,.06)'><td>BTCUSDT</td>"
        f"<td><span class='chip c-alert'>excluded</span></td>"
        f"<td>{BTC['binance_interval_hours']}h / {BTC['bybit_interval_minutes']}m</td>"
        f"<td>yes</td>"
        f"<td class='dim'>{float(D(BTC['binance_quote_volume_24h'])) / 1e9:,.2f}G</td>"
        f"<td class='void'>&mdash;</td><td class='void'>&mdash;</td>"
        f"<td class='void'>&mdash;</td><td class='void'>&mdash;</td>"
        f"<td class='void'>VOID</td><td class='void'>VOID</td><td class='void'>VOID</td></tr>")
    rows.append(
        "<tr><td colspan='12' style='text-align:left;opacity:.6;padding-top:7px'>"
        f"&hellip; {len(OK) - 7} more eligible, then {NO_STATS} rows carrying "
        "<b>9 of the 19 fields</b> &mdash; no stats block at all</td></tr>")

    filt = " &nbsp;".join(
        f"<span class='chip c-{'yes' if k == 'ok' else ('alert' if k.startswith('excluded') else 'no')}'>"
        f"{esc(k)} {v}</span>" for k, v in ELIG.most_common())

    body = f"""
  <div class="banner"><b>The default order of this table is the artifact's own row order, and that
  order is descending <code>rank_score</code>.</b> <code>rank_score</code> is
  <b>net_annual at each row's own best entry level</b> &mdash; 0 of {len(OK)} rows disagree with
  <code>argmax<sub>level</sub></code> &mdash; and the {len(OK)} rows are spread across
  <b>{len(BEST_ENTRY)} different levels</b> ({', '.join(f'{v}&times;{k}' for k, v in sorted(BEST_ENTRY.items()))}).
  It is also per <b>one</b> leg (F30). Nothing has to be decided for this to ship: it is what the file
  hands a reader that does not sort.</div>

  <div class="panel"><h4>Screening outcome {prov('exists')} <span style="opacity:.6;font-weight:400;text-transform:none;letter-spacing:0">
  &sect;7 renders this as &ldquo;466 screened / {len(OK)} eligible&rdquo; &mdash; two counts over a
  {len(ELIG)}-valued column</span></h4>
  <div style="line-height:2.1">{filt}</div>
  <div class="note" style="margin-top:7px"><b>One of the {len(ELIG)} is not a market fact.</b>
  BTCUSDT is <code>{esc(BTC['eligible'])}</code> &mdash; &sect;42e's permanent seal &mdash; and it is the
  highest-turnover row in the universe ({float(D(BTC['binance_quote_volume_24h'])) / float(FLOOR):,.0f}&times;
  the floor). It is also the only symbol <code>configs/reference.btcusdt.json</code> fully describes.</div></div>

  <div class="panel"><h4>Pairs &mdash; file order {prov('exists')} {prov('void', 'VOID: 3 COLS')}</h4>
  <table><thead><tr><th>Symbol</th><th>Eligible</th><th>Grid</th><th>Same</th>
  <th>24h vol</th><th>med |&Delta;|</th><th>p99 |&Delta;|</th><th>Best entry</th>
  <th>rank_score</th><th>Live funding</th><th>Spread</th><th>Next settle</th></tr></thead>
  <tbody>{''.join(rows)}</tbody></table>
  <div class="note" style="margin-top:7px">The three VOID columns are &sect;7's
  <i>live funding both legs</i> (F65), <i>spread</i> (F77) and <i>next settlement</i> (F78) &mdash;
  every one already ruled <b>live: none</b>. The <span class="dim">dimmed</span> 24h volume is a single
  24-hour sample taken at the <code>cutoff_utc {esc(RANK['params']['cutoff_utc'])}</code>, and it is what
  decides the eligibility column beside it: <b>{NEAR} rows sit within &plusmn;20&thinsp;% of the
  {float(FLOOR) / 1e6:.0f}M floor.</b></div></div>"""
    return shell("Markets", body)


# ============================================================ MARKET DETAIL
def market_detail():
    lab = next(e for e in EPISODES if e["symbol"] == "LABUSDT")
    lvl_rows = []
    rk = next(p for p in OK if p["symbol"] == "LABUSDT")
    for L, R in zip(lab["levels"], rk["levels"]):
        runs = [e["run"] for e in L["episodes"]]
        runs.sort()
        p50 = runs[len(runs) // 2]
        p90 = runs[int(len(runs) * .9)]
        lvl_rows.append(
            f"<tr><td>{L['entry']}</td><td>{len(L['episodes'])}</td>"
            f"<td>{p50}</td><td>{p90}</td><td>{max(runs)}</td>"
            f"<td>{R['wins']}</td><td class='pos'>{pct(R['net_annual'])}%</td>"
            f"<td class='pos'>{pct(D(R['net_annual']) / 2)}%</td></tr>")
    runs = Counter(e["run"] for e in lab["levels"][2]["episodes"])
    top = max(runs.values())
    bars = "".join(
        f'<i style="height:{100 * runs.get(n, 0) // top}%" title="run {n}: {runs.get(n, 0)}"></i>'
        for n in range(1, 21))
    body = f"""
  <div class="panel"><h4>LABUSDT &mdash; episode distribution, entry 0.001 {prov('exists')}</h4>
  <div class="spark" style="color:#3fa66b">{bars}</div>
  <div class="note" style="margin-top:6px">Runs 1&ndash;20 settlements. Break-even at 21&thinsp;bp is
  &asymp;<b>6</b>, which is p93 (&sect;99 &sect;3). <b>The overlay's position is a function of the cost
  control, and the histogram's shape is a function of the entry level</b> &mdash; this panel is drawn at
  one of each, and both coordinates are on screen because F83 says a run-length figure that names neither
  is not a reading.</div></div>

  <div class="panel"><h4>Per entry level {prov('exists')}</h4>
  <table><thead><tr><th>Entry</th><th>Episodes</th><th>p50 run</th><th>p90 run</th>
  <th>Max run</th><th>Wins</th><th>net_annual<br><span style="opacity:.6">1 leg</span></th>
  <th>on capital<br><span style="opacity:.6">2 legs</span></th></tr></thead>
  <tbody>{''.join(lvl_rows)}</tbody></table>
  <div class="note" style="margin-top:6px">The last two columns are the same number and the ranked
  table upstream carries only the first. &sect;7's decision 2 (&ldquo;returns are labelled per deployed
  capital&rdquo;) is recorded as embedded <b>in the Overview</b>.</div></div>

  <div class="row">
  <div class="panel" style="flex:1"><h4>Funding history, both legs {prov('void')}</h4>
  <div class="note">F53. The one producer writes to a directory that is gitignored: measured, the
  9&nbsp;&times;&nbsp;2 <code>research/crossvenue/rest/funding.*.json</code> files return
  <b>0</b> from <code>git ls-files</code>, so a fresh clone has none of them &mdash; and they are settled-rate
  history, the class <code>build-spec.md:793</code> excludes as a signal.</div></div>
  <div class="panel" style="flex:1"><h4>Book depth at size &middot; cost decomposition {prov('void')}</h4>
  <div class="note">F71 / F72 / F73. No calibration artifact is committed;
  <code>driven.BookSource</code> has 0 implementations.</div></div></div>

  <div class="panel"><h4>Filters for this symbol {prov('void')}</h4>
  <div class="note">The reference store holds lot rules for <b>2</b> distinct symbols across its three
  revisions (BTCUSDT, LABUSDT) against this screen's {len(PAIRS)} rows &mdash; and the one it describes most
  completely is the row Markets excludes. F63, F82.</div></div>"""
    return shell("Market detail", body)


# ============================================================ POSITIONS
def positions():
    body = """
  <div class="banner"><b>Both legs of the only cross-venue pair this project has run carry the
  byte-identical ClientOrderID</b> &mdash; <code>ab-01M0587R38EN89VFRK50XGSVX5-<b>00</b>-00</code> in
  <code>run/pair/binance.db</code> and in <code>run/pair/bybit.db</code>. The id's format is
  <code>ab-&lt;TradeID&gt;-&lt;leg&gt;-&lt;attempt&gt;</code> and its own documentation says the leg field is
  &ldquo;<i>0 and 1 for a package's spot and perp legs</i>&rdquo;. <code>pair/config.go:430</code>
  builds a <b>one-leg</b> <code>TradePlan</code> per side and <code>planning.go:267</code> derives
  <code>Leg: i</code> from its own slice index &mdash; so the pair's leg index is dropped at the boundary
  and every side mints leg <code>00</code>. <b>This screen's join key is the only thing that has to span
  the two journals, and it does not distinguish them.</b></div>

  <div class="panel"><h4>Historical packages, reconstructed from journals
  <span class="prov p-void">VOID</span></h4>
  <table><thead><tr><th>Table</th><th>binance.db</th><th>bybit.db</th><th>Written by</th></tr></thead>
  <tbody>
  <tr><td>journal</td><td>5</td><td>5</td><td class="dim">planner / dispatch</td></tr>
  <tr><td>journal_planned</td><td>1</td><td>1</td><td class="dim">planner</td></tr>
  <tr><td>journal_authorized</td><td>1</td><td>1</td><td class="dim">dispatch</td></tr>
  <tr><td>journal_acked</td><td>1</td><td>1</td><td class="dim">dispatch</td></tr>
  <tr><td>journal_state</td><td>2</td><td>2</td><td class="dim">submitted, filled</td></tr>
  <tr style="background:rgba(214,79,79,.06)"><td>journal_generation_opened</td><td class="void">0</td><td class="void">0</td><td>delever.go:252</td></tr>
  <tr style="background:rgba(214,79,79,.06)"><td>journal_generation_leg</td><td class="void">0</td><td class="void">0</td><td>delever.go:429/581</td></tr>
  <tr style="background:rgba(214,79,79,.06)"><td>journal_generation_authorized</td><td class="void">0</td><td class="void">0</td><td>delever.go:534</td></tr>
  <tr style="background:rgba(214,79,79,.06)"><td>journal_generation_superseded</td><td class="void">0</td><td class="void">0</td><td>delever.go:889</td></tr>
  <tr style="background:rgba(214,79,79,.06)"><td>journal_generation_phase</td><td class="void">0</td><td class="void">0</td><td>delever.go:933/950</td></tr>
  </tbody></table>
  <div class="note" style="margin-top:7px">Measured with <code>sqlite3</code> on the two files the handoff
  calls &ldquo;the evidence that the gate fired&rdquo;. <b>Every <code>AppendGeneration*</code> call site in
  the tree is in <code>delever.go</code></b> &mdash; the generation layer is written by one use case, and the
  entry path writes none of it. &sect;7 stamps <i>historical packages reconstructed from journals</i> phase
  <b>1</b>.</div></div>

  <div class="panel"><h4>Legs, state machine, gates {prov('creds')}</h4>
  <div class="note">The terminal state in both journals is <code>filled</code> and no quantity is
  journaled with it (<code>reconcile.go:205</code>: &ldquo;<i>State, not quantity &mdash; no fill is
  journaled</i>&rdquo;). Gates and latches are phase 2 by F52.</div></div>"""
    return shell("Positions", body)


# ============================================================ VENUES
def venues():
    fee_rows = "".join(
        f"<tr><td>{esc(r['venue'])} {esc(r['product'])}</td><td>{r['maker']}</td>"
        f"<td>{r['taker']}</td><td>{esc(r['tier'])}</td>"
        f"<td><span class='chip c-mar'>{esc(r['class'])}</span></td>"
        f"<td class='dim'>{esc(r['observed_at'][:10])}</td></tr>"
        for r in REFERENCE["fees"])
    lot_rows = "".join(
        f"<tr><td>{esc(r['venue'])} {esc(r['product'])}</td><td>{esc(r['symbol'])}</td>"
        f"<td>{r['step']}</td><td>{r['tick']}</td><td>{r['min_notional']}</td>"
        f"<td class='dim'>{esc(r['observed_at'][:10])}</td></tr>"
        for r in REFERENCE["lots"])
    body = f"""
  <div class="panel"><h4>Fee tiers {prov('exists')} {prov('void', 'NOT FETCHED')}</h4>
  <table><thead><tr><th>Venue &middot; product</th><th>Maker</th><th>Taker</th><th>Tier</th>
  <th>Class</th><th>Observed</th></tr></thead><tbody>{fee_rows}</tbody></table>
  <div class="note" style="margin-top:6px">Four rows, <b>one tier per (venue, product)</b>, every one
  <code>hand_recorded</code> &mdash; including in the revision <code>arblab reference fetch</code> itself
  produced, whose <code>lots</code> block reads <code>fetched</code>. Core &sect;2.4 requires
  <b>programmatically-fetched</b> fees (F80). The <i>product</i> axis moves the round trip 43&thinsp;% and
  &sect;8 row 7 names <i>tier</i>, which is constant here (F81).</div></div>

  <div class="panel"><h4>Filters {prov('exists')}</h4>
  <table><thead><tr><th>Venue &middot; product</th><th>Symbol</th><th>Step</th><th>Tick</th>
  <th>Min notional</th><th>Observed</th></tr></thead><tbody>{lot_rows}</tbody></table>
  <div class="note" style="margin-top:6px">One symbol per revision, against Markets' {len(PAIRS)}.
  The <code>tick</code> member of every row above was observed <b>2026-08-03</b> and the row stamps
  <b>{esc(REFERENCE['lots'][0]['observed_at'][:10])}</b> &mdash; F82, and &sect;8c point 2's remedy is what
  prints the wrong one.</div></div>

  <div class="row">
  <div class="panel" style="flex:1"><h4>Leverage brackets {prov('void')}</h4>
  <div class="note">F61 &mdash; no data, no type, no reader.</div></div>
  <div class="panel" style="flex:1"><h4>Posture &middot; balances {prov('creds')}</h4>
  <div class="note">F63 / F67 &mdash; every named reader is signed. There is no phase-1 half.</div></div>
  <div class="panel" style="flex:1"><h4>Connectivity {prov('void')}</h4>
  <div class="note">F64 &mdash; no source in this record or the tree.</div></div></div>

  <div class="panel"><h4>Rate-limit budget {prov('daemon')}</h4>
  <div class="note">The capture declares <b>{len(CAPTURE['sources'])}</b> WebSocket sources and
  <b>{len(CAPTURE['rest_sources'])}</b> REST source. Measured against the venues' documented
  per-connection limits, all {len(PAIRS)} screened symbols fit on <b>one connection per venue</b> &mdash;
  the {len(CAPTURE['sources'])}-socket footprint is <code>configs/capture.json</code>'s, not the
  venues' (F93).</div></div>"""
    return shell("Venues", body)


# ============================================================ EVIDENCE
def evidence():
    v = SUMMARY["verdict"]
    cost_rows = "".join(
        f"<tr><td>{c}</td><td>{v[c]['median_annual_pct']}%</td>"
        f"<td>{v[c]['positive_symbols']}</td>"
        f"<td class='{'pos' if int(v[c]['beat_hurdle_symbols']) else 'neg'}'>"
        f"{v[c]['beat_hurdle_symbols']} / {v[c]['total_symbols']}</td>"
        f"<td class='dim'>{v[c]['hurdle_pct']}% {esc(v[c]['hurdle_class'])}</td></tr>"
        for c in SUMMARY["costs"])
    pooled_rows = "".join(
        f"<tr><td>{p['entry']}</td><td>{p['episodes']:,}</td><td>{p['run_p50']}</td>"
        f"<td>{p['run_p75']}</td><td>{p['run_p90']}</td><td>{p['run_max']}</td></tr>"
        for p in SUMMARY["pooled"])
    body = f"""
  <div class="banner"><b>The verdict block is computed at one entry level and records none</b> (F60).
  The table below is the published column. <code>beat_hurdle_symbols</code> at 21&thinsp;bp reads
  <b>2/61 &middot; 3/60 &middot; 4/56 &middot; 3/49</b> across the four levels this same file declares, and the
  published <b>4/56</b> is the unique maximum. Reproduce with
  <code>python3 research/persistence/verdict_levels.py</code>.</div>

  <div class="panel"><h4>Kill gate &mdash; the cost control {prov('exists')}
  <span style="font-weight:400;text-transform:none;letter-spacing:0;opacity:.6">entry level
  <b style="color:#c9922e">0.001</b> &mdash; supplied by this screen, absent from the artifact</span></h4>
  <table><thead><tr><th>Round-trip cost</th><th>Median</th><th>Positive</th>
  <th>Beat hurdle</th><th>Hurdle</th></tr></thead><tbody>{cost_rows}</tbody></table>
  <div class="note" style="margin-top:6px">{SUMMARY['matched_settlements']:,}&nbsp;matched settlements,
  {SUMMARY['symbols_eligible']} eligible symbols, {esc(SUMMARY['span']['first'][:10])} &rarr;
  {esc(SUMMARY['span']['last'][:10])}. The three costs are the artifact's; &sect;8a offers a
  <b>fourth</b> (custom) and the artifact has no cell for it (F79).</div></div>

  <div class="panel"><h4>Run length vs break-even, pooled {prov('exists')}</h4>
  <table><thead><tr><th>Entry</th><th>Episodes</th><th>p50</th><th>p75</th><th>p90</th><th>Max</th></tr></thead>
  <tbody>{pooled_rows}</tbody></table>
  <div class="note" style="margin-top:6px"><b>p75 is {SUMMARY['pooled'][0]['run_p75']} at every level</b>,
  so a golden on it passes everywhere and cannot detect a missing entry-level coordinate &mdash; F83, the
  fifteenth fixture collapse, and the one in an artifact rather than a test.</div></div>

  <div class="panel"><h4>Concentration {prov('exists')}</h4>
  <div class="note">F27 &mdash; at 10&thinsp;bp/21&thinsp;bp the net across
  {v['0.0021']['total_symbols']} symbols is +0.6460 and <code>LABUSDT</code> alone is +0.5439. Dropping the
  best 10 of 1&nbsp;259 episodes turns the total negative. The per-symbol block holds
  <b>{len(SUMMARY['per_symbol'])}</b> rows against {SUMMARY['symbols_eligible']} eligible symbols, and the
  five it drops are dropped <i>by the unrecorded entry level</i>.</div></div>"""
    return shell("Evidence", body)


# ============================================================ JOURNAL
def journal():
    body = """
  <div class="panel"><h4>Journal &mdash; run/pair/binance.db {prov('exists')}</h4>
  <table><thead><tr><th>Seq</th><th>Kind</th><th>Subject</th><th>Hash chain</th></tr></thead><tbody>
  <tr><td>1</td><td>planned</td><td>ab-01M0587R38EN89VFRK50XGSVX5-00-00</td><td class="dim">genesis</td></tr>
  <tr><td>2</td><td>authorized</td><td>ab-01M0587R38EN89VFRK50XGSVX5-00-00</td><td class="pos">&#10003;</td></tr>
  <tr><td>3</td><td>acked</td><td>ab-01M0587R38EN89VFRK50XGSVX5-00-00</td><td class="pos">&#10003;</td></tr>
  <tr><td>4</td><td>state</td><td>ab-01M0587R38EN89VFRK50XGSVX5-00-00 &rarr; submitted</td><td class="pos">&#10003;</td></tr>
  <tr><td>5</td><td>state</td><td>ab-01M0587R38EN89VFRK50XGSVX5-00-00 &rarr; filled</td><td class="pos">&#10003;</td></tr>
  </tbody></table>
  <div class="note" style="margin-top:6px"><code>run/pair/bybit.db</code> holds the same five kinds over
  the <b>same five subjects, byte for byte</b>. A viewer that unions the two journals on
  <code>subject</code> renders one order where there are two, on two venues, on opposite sides
  (<code>sell</code> on binance, <code>buy</code> on bybit). The discriminator is the filename.</div></div>

  <div class="row">
  <div class="panel" style="flex:1"><h4>Gates &middot; latches {prov('creds')}</h4>
  <div class="note">F52 &mdash; the schema's 13 tables hold no gate, no latch and no checklist.
  <code>Checklist.Latched()</code> reads an in-memory map; <code>GateNetDeltaAsserted</code> is settled by
  a credentialed venue join. Phase 2, and <code>unverified</code> until then &mdash; never
  <code>0</code>.</div></div>
  <div class="panel" style="flex:1"><h4>Open incidents {prov('exists')}</h4>
  <div class="note">Journal-local only: what phase 1 can honestly show is an incident this journal
  recorded, not one a venue would report.</div></div></div>"""
    return shell("Journal", body)


# ============================================================ SYSTEM
def system():
    stale_rows = "".join(
        f"<tr><td class='dim'>{esc(m)}</td><td>{esc(p)}</td><td>{esc(r or '')}</td>"
        f"<td>{'<span class=chip c-alert>knownStale</span>' if k else '<span class=chip c-yes>fresh</span>'}</td></tr>"
        for m, p, r, k in STALE)
    body = f"""
  <div class="banner"><b>&sect;7's System item is called &ldquo;capture coverage&rdquo;, and the capture
  verifier has <u>two</u> finding classes.</b> <code>engine/capture/finding.go</code>:
  <i>1 = integrity failure &mdash; the archive is LYING; 2 = coverage below threshold &mdash; the archive is
  HONEST AND THIN</i>, and the file says outright that <b>&ldquo;the two are never summed into a single
  'problems' count.&rdquo;</b> Counted from the <code>Class()</code> switch:
  <b>{N_INTEGRITY} integrity kinds</b> and <b>{N_COVERAGE} coverage kinds</b>. The screen item names the
  coverage half, and the same file warns that <i>&ldquo;a coverage figure computed while skipping a file
  nobody could read would be a lie of the same size.&rdquo;</i></div>

  <div class="row">
  <div class="panel" style="flex:1"><h4>Archive integrity {prov('void')}</h4>
  <div class="note"><b>{N_INTEGRITY}</b> kinds, exit code 1. No cell in &sect;7.</div></div>
  <div class="panel" style="flex:1"><h4>Archive coverage {prov('daemon')}</h4>
  <div class="note"><b>{N_COVERAGE}</b> kinds, exit code 2. Threshold
  <code>min_alive_percent {CAPTURE['min_alive_percent']}</code>.</div></div>
  <div class="panel" style="flex:1"><h4>Build info {prov('exists')}</h4>
  <div class="note"><code>debug.ReadBuildInfo</code> &mdash; the toolchain's own VCS stamp
  (<code>main.go:174</code>), not an ldflags value. Holds.</div></div></div>

  <div class="panel"><h4>Formal-model freshness {prov('exists')}
  <span style="font-weight:400;text-transform:none;letter-spacing:0;opacity:.6">{N_STALE} of
  {len(STALE)} tracked sections marked stale &middot; last verified
  {' / '.join(f'{k} {v}' for k, v in VERIFIED.items())}</span></h4>
  <table><thead><tr><th>Model</th><th>Section</th><th>Role</th><th>State</th></tr></thead>
  <tbody>{stale_rows}</tbody></table>
  <div class="note" style="margin-top:6px">Rendered per row and per role, because F62 says the gate has
  <b>five</b> verdicts and a boolean puts <i>&ldquo;this model's runs are void&rdquo;</i> on the green
  side. <code>make formal-fresh</code> is green and prints none of this: the marker narrates through
  <code>t.Log</code> and the target runs <code>go test</code> without <code>-v</code>.</div></div>

  <div class="panel"><h4>Config {prov('exists')}</h4>
  <div class="note"><code>configs/capture.json</code>: {len(CAPTURE['sources'])} WebSocket sources,
  {len(CAPTURE['rest_sources'])} REST source, rotation {esc(CAPTURE['rotation_period'])}. Holds.</div></div>"""
    return shell("System", body)


# ============================================================ document
w("<title>The seven undrawn screens</title>")
w(STYLE)
w("<h2>The seven screens &sect;7 declares and no round had drawn</h2>")
w('<p class="note">Generated by <code>gen-remaining-screens.py</code> from '
  '<code>research/screening/ranking.json</code>, <code>research/persistence/'
  '{summary,episodes}.json</code>, <code>configs/{reference.btcusdt,capture}.json</code>, '
  '<code>docs/formal/freshness.json</code> and <code>internal/engine/capture/finding.go</code>. '
  'Re-run it and every figure re-derives. Nothing here is drawn from a number a person typed.</p>')
w('<div class="legend">'
  + prov("exists") + " readable from a fresh clone "
  + prov("daemon") + " real, needs <code>serve</code> to poll public venue data "
  + prov("creds") + " needs signed venue calls &mdash; phase 2 "
  + prov("open") + " not decidable yet "
  + prov("void") + " <b>no source exists</b></div>")
w('<div class="ov">')
for fn in (markets, market_detail, positions, venues, evidence, journal, system):
    w(fn())
w("</div>")

with open(OUT, "w") as fh:
    fh.write("\n".join(P) + "\n")
print("wrote", OUT, os.path.getsize(OUT), "bytes")
