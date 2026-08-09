#!/usr/bin/env python3
"""Oracle for #233: the maintenance-bracket fixed point at the LIQUIDATION notional.

Three independent things are checked, none of them by review:

  A. REPRODUCTION -- the derived closed form regenerates all four rows of record
     §89 F3's tables (both legs x both candidate brackets) from the raw capture's
     own bracket table, to the digits the record states.

  B. EXISTENCE + UNIQUENESS -- over a grid of (quantity, entry, margin) spanning
     every bracket of a real Binance table, exactly one bracket is self-consistent.
     This is the claim the spec is about to make normative; a paper proof is not
     evidence here.

  C. THE STEP IDENTITY -- sign(N_{i+1} - N_i) == sign(b_i - N_i) for a short and
     its exact mirror for a long, which is WHY the search never overshoots and
     why the measurement saw a short reach up while a long reached down.

Decimal throughout: this is the same class of quantity the engine holds in
fixedpoint, and a float check of a 5.6E-10 residual proves nothing.
"""
from decimal import Decimal as D, getcontext
import json
import sys

getcontext().prec = 40

CAPTURE = ("research/venue-probes/230-adverse-close/captures/"
           "probe230-binance-DRIFTUSDT.json")


def load_brackets(path):
    with open(path) as fh:
        raw = json.load(fh)["brackets"]
    return [
        {
            "i": b["bracket"],
            "lo": D(str(b["notionalFloor"])),
            "hi": D(str(b["notionalCap"])),
            "mmr": D(str(b["maintMarginRatio"])),
            "cum": D(str(b["cum"])),
        }
        for b in raw
    ]


# --- the derivation -------------------------------------------------------
#
# Isolated SHORT liquidates when  m + (e - L)*q == MM(q*L),  MM(N) = N*mmr - cum.
#   => N_i := q*L_i = (m + q*e + cum_i) / (1 + mmr_i)
# Isolated LONG  liquidates when  m - (e - L)*q == MM(q*L).
#   => N_i := q*L_i = (q*e - m - cum_i) / (1 - mmr_i)
#
# Note N depends on q only through (m + q*e): the liquidation NOTIONAL, not the
# liquidation price, is the natural variable, and it is what selects the bracket.

def liq_notional(side, q, e, m, br):
    if side == "short":
        return (m + q * e + br["cum"]) / (1 + br["mmr"])
    return (q * e - m - br["cum"]) / (1 - br["mmr"])


def self_consistent(side, q, e, m, brackets):
    """Every bracket whose own predicted liquidation notional lands back inside it."""
    hits = []
    for br in brackets:
        n = liq_notional(side, q, e, m, br)
        if br["lo"] <= n < br["hi"]:
            hits.append((br, n))
    return hits


# --- A. reproduction of record §89 F3 ------------------------------------

def check_reproduction(brackets):
    print("A. REPRODUCTION of record §89 F3's four rows")
    cases = [
        # side,   q,        e,        m,           observed L,      §89's q*L*, expected verdict
        ("short", "370000", "0.0131", "966.9765",  "0.01554780", {1: "5756", 2: "5753"}),
        ("long",  "400000", "0.0132", "1093.36",   "0.01057232", {1: "4229", 2: "4225"}),
    ]
    ok = True
    for side, q, e, m, obs, expect_n in cases:
        q, e, m, obs = D(q), D(e), D(m), D(obs)
        print(f"   {side:5s} q={q} e={e} m={m}  observed liqPrice {obs}")
        for br in brackets[:2]:
            n = liq_notional(side, q, e, m, br)
            L = n / q
            inside = br["lo"] <= n < br["hi"]
            err = L - obs
            got_n = str(int(n.quantize(D("1"))))
            match_n = got_n == expect_n[br["i"]]
            ok &= match_n
            print(f"      bracket {br['i']}: q*L = {got_n:>6s} "
                  f"(§89 says {expect_n[br['i']]:>6s}) {'OK' if match_n else 'MISMATCH'}"
                  f" | self-consistent={str(inside):5s} | L={L:.10f} err={err:+.2E}")
        hits = self_consistent(side, q, e, m, brackets)
        assert len(hits) == 1, f"{side}: {len(hits)} self-consistent brackets"
        br, n = hits[0]
        resid = n / q - obs
        print(f"      -> fixed point is bracket {br['i']}, residual vs published "
              f"liqPrice = {resid:+.2E}")
        ok &= abs(resid) < D("1E-7")
    return ok


# --- B. existence and uniqueness over the real table ---------------------

def sample_notionals(brackets):
    """Notionals to sweep.

    A uniform grid over [0, top_cap] is a COLLAPSED fixture: the top cap is
    3,000,000 while bracket 1 is [0, 5000), so any grid coarse enough to walk the
    whole table in a few hundred steps never enters the low brackets -- which is
    exactly where §89 measured this. Sample boundary-RELATIVE instead, so every
    bracket edge is approached from both sides at several distances, and keep a
    coarse global sweep on top of it.
    """
    out = []
    top = brackets[-1]["hi"]
    for br in brackets:
        for edge in (br["lo"], br["hi"]):
            if edge == 0:
                continue
            for mult in ("0.3", "0.5", "0.8", "0.95", "0.999", "1.001", "1.05", "1.3", "2"):
                n = edge * D(mult)
                if D(1) < n < top:
                    out.append(n)
    for pct in range(1, 200):
        out.append(top * D(pct) / D(200))
    return sorted(set(out))


def check_existence_uniqueness(brackets):
    print("\nB. EXISTENCE + UNIQUENESS over the real bracket table")
    top = brackets[-1]["hi"]
    e = D("0.0131")
    bad = 0
    tested = 0
    # Sweep notionals across every bracket, and leverages across the §7.1 range
    # and well past it -- m is what moves N relative to q*e.
    notionals = sample_notionals(brackets)
    covered = {br["i"]: 0 for br in brackets}
    for notional in notionals:
        q = notional / e
        for lev in ("1.5", "2", "3", "5", "10", "25"):
            m = notional / D(lev)
            for side in ("short", "long"):
                if side == "long" and m >= q * e:
                    continue  # margin exceeding notional is not a real long
                tested += 1
                hits = self_consistent(side, q, e, m, brackets)
                n_top = liq_notional(side, q, e, m, brackets[-1])
                if len(hits) == 1:
                    covered[hits[0][0]["i"]] += 1
                    continue
                if len(hits) == 0 and n_top >= top:
                    continue  # off the top of the table: the REFUSAL case, expected
                bad += 1
                if bad <= 5:
                    print(f"   VIOLATION {side} notional={notional:.0f} lev={lev} "
                          f"-> {len(hits)} self-consistent brackets")
    print(f"   {tested} configurations tested, {bad} violations")
    # Counting executions says nothing until you count how many reach the
    # interesting branch: a sweep that never resolves INTO a low bracket has not
    # tested the region §89 measured.
    print("   fixed point landed in bracket: " +
          ", ".join(f"{i}:{n}" for i, n in sorted(covered.items())))
    unreached = [i for i, n in covered.items() if n == 0]
    if unreached:
        print(f"   VACUOUS: brackets never selected: {unreached}")
    return bad == 0 and not unreached


# --- C. the step identity ------------------------------------------------

def check_step_identity(brackets):
    print("\nC. STEP IDENTITY  sign(N_{i+1} - N_i) == sign(b_i - N_i)  [short]")
    print("                  sign(N_{i+1} - N_i) == sign(N_i - b_i)  [long]")
    e = D("0.0131")
    bad = 0
    tested = 0
    for notional_pct in range(1, 200):
        notional = brackets[-1]["hi"] * D(notional_pct) / D(200)
        q = notional / e
        for lev in ("2", "3", "10"):
            m = notional / D(lev)
            for side in ("short", "long"):
                if side == "long" and m >= q * e:
                    continue
                for a, b in zip(brackets, brackets[1:]):
                    n_a = liq_notional(side, q, e, m, a)
                    n_b = liq_notional(side, q, e, m, b)
                    sign = lambda x: (x > 0) - (x < 0)
                    step = sign(n_b - n_a)
                    if side == "short":
                        pred = sign(a["hi"] - n_a)
                    else:
                        pred = sign(n_a - a["hi"])
                    tested += 1
                    if step != pred:
                        bad += 1
                        if bad <= 5:
                            print(f"   VIOLATION {side} n_a={n_a:.2f} b={a['hi']} "
                                  f"step={step} predicted={pred}")
    print(f"   {tested} adjacent-bracket steps tested, {bad} violations")
    return bad == 0


# --- D. where the criterion starts to bite, in CURRENT-notional terms ----

def check_bite_point(brackets):
    print("\nD. WHERE IT BITES: the bracket-1 ceiling in CURRENT-notional terms")
    b1 = brackets[0]
    print(f"   bracket 1 = [{b1['lo']}, {b1['hi']}) mmr={b1['mmr']} cum={b1['cum']}")
    for lev in ("1.5", "2", "3", "5"):
        lam = D(lev)
        # short: N = q*e*(1 + 1/lam)/(1 + mmr) < hi_1  =>  q*e < hi_1*(1+mmr)/(1+1/lam)
        cap = b1["hi"] * (1 + b1["mmr"]) / (1 + 1 / lam)
        print(f"   short at {lev:>4s}x: current notional must stay under "
              f"{cap:.0f} for the LIQUIDATION notional to remain in bracket 1 "
              f"({(cap / b1['hi'] * 100):.0f}% of the published cap)")
    return True


if __name__ == "__main__":
    brackets = load_brackets(CAPTURE)
    print(f"bracket table from {CAPTURE}: {len(brackets)} brackets, "
          f"top cap {brackets[-1]['hi']}\n")
    results = [
        check_reproduction(brackets),
        check_existence_uniqueness(brackets),
        check_step_identity(brackets),
        check_bite_point(brackets),
    ]
    print("\n" + ("ALL CHECKS PASS" if all(results) else "FAILURES ABOVE"))
    sys.exit(0 if all(results) else 1)
