#!/usr/bin/env python3
"""Task #225 -- the reading, recomputed from the SAVED captures.

This is deliberately a separate file from the probes. A probe capture is not
re-derivable (research/venue-probes/README.md): re-running pass 3 opens a
different position at a different price and produces a different measurement,
not the same one. So when the probe's own inline verdict turned out to be
WRONG, the fix could not be "re-run it" -- the fix is to re-derive the reading
from the bytes that were captured, which is what this does.

WHAT THE PROBE GOT WRONG, because it is the method lesson:

    probe225_pass3 asked "when IM went up, did walletBalance move?" and read
    phase C -- a MARKET FILL -- as evidence that walletBalance nets out margin,
    because walletBalance moved by -0.07171582.

    That number is the TAKER FEE, not margin. The action that created position
    margin also charged a fee, so the fixture moved TWO quantities and the test
    could not attribute the change to either. Phase D is the isolating one:
    add-margin moves margin ALONE -- no fill, no fee, no PnL, and it is exactly
    7.1's operation. It reverses the verdict.

That is the isolating-probe rule from CLAUDE.md ("a probe two gates can see")
landing on a venue measurement rather than a linter fixture.
"""
import json
import os
from decimal import Decimal as D, getcontext

getcontext().prec = 40
HERE = os.path.dirname(os.path.abspath(__file__))


def dec(v):
    try:
        s = str(v)
        return D(s) if s not in ("", "None", "null") else None
    except Exception:  # noqa: BLE001
        return None


def load(name):
    with open(os.path.join(HERE, "captures", name)) as fh:
        return json.load(fh)


def rule(title):
    print("\n" + "=" * 78)
    print(title)
    print("=" * 78)


# --------------------------------------------------------------------- bybit

def bybit_reading():
    rule("BYBIT -- is walletBalance GROSS of posted margin?")
    snaps = load("probe225-pass3-bybit.json")["bybit"]
    by = {s["label"][0]: s for s in snaps}

    print(f"  {'phase':14s} {'walletBalance':>18s} {'posIM':>14s} {'ordIM':>14s}")
    for k in "ABCDE":
        s = by[k]
        c = s["coin"]
        print(f"  {k + '. ':14s} {c['walletBalance']:>18s} "
              f"{c['totalPositionIM']:>14s} {c['totalOrderIM']:>14s}")

    wA = dec(by["A"]["coin"]["walletBalance"])
    wB = dec(by["B"]["coin"]["walletBalance"])
    wC = dec(by["C"]["coin"]["walletBalance"])
    wD = dec(by["D"]["coin"]["walletBalance"])
    imB = dec(by["B"]["coin"]["totalOrderIM"])
    imC = dec(by["C"]["coin"]["totalPositionIM"])
    imD = dec(by["D"]["coin"]["totalPositionIM"])

    print("\n  ORDER-MARGIN LEG (phase B) -- a resting limit, no fill, no fee:")
    print(f"     ordIM appeared: +{imB}")
    print(f"     walletBalance delta: {wB - wA}")
    print(f"     => {'GROSS' if wB == wA else 'NET'}: walletBalance "
          f"{'ignores' if wB == wA else 'absorbs'} committed order margin")

    print("\n  POSITION-MARGIN LEG, CONFOUNDED (phase C) -- a market fill:")
    print(f"     posIM appeared: +{imC}")
    print(f"     walletBalance delta: {wC - wA}")
    print("     This delta is NOT margin. Reconstructing it as a taker fee:")
    fill_notional = D("0.002") * D("65196.17")          # qty x mark at open
    fee = fill_notional * D("0.00055")                  # Bybit taker 0.055%
    print(f"        0.002 x 65196.17 x 0.00055 = {fee}")
    print(f"        observed                    = {-(wC - wA)}")
    print(f"        residual                    = {fee + (wC - wA)}")
    print("     The fee accounts for the whole move, so phase C attributes")
    print("     NOTHING to margin. A fixture that moves two quantities at once")
    print("     cannot assign the change to either.")

    print("\n  POSITION-MARGIN LEG, ISOLATED (phase D) -- add-margin alone:")
    print(f"     posIM: {imC} -> {imD}  (delta {imD - imC})")
    print(f"     walletBalance: {wC} -> {wD}  (delta {wD - wC})")
    verdict = "GROSS" if wD == wC else "NET"
    print(f"     => {verdict}. Posting {imD - imC} of margin moved walletBalance by "
          f"{wD - wC}.")
    print("\n  CONCLUSION: walletBalance is GROSS of posted margin on both legs, so")
    print("  A = walletBalance - totalPositionIM - totalOrderIM is the correct")
    print("  derivation and the subtraction is REQUIRED, not defensive.")
    return {"gross": wD == wC}


# ------------------------------------------------------------------- binance

def binance_reading():
    rule("BINANCE -- what IS availableBalance, and did the candidates ever split?")
    snaps = load("probe225-pass3-binance.json")["binance"]
    by = {s["label"][0]: s for s in snaps}

    print("  The identity, phase by phase:  walletBalance - availableBalance = ?")
    print(f"  {'phase':6s} {'W - A':>16s} {'isolatedWallet':>16s} "
          f"{'openOrderIM':>14s} {'positionIM':>14s} {'residual':>12s}")
    for k in "ABCDE":
        s = by[k]
        w = dec(s["asset"]["walletBalance"])
        a = dec(s["asset"]["availableBalance"])
        iso = dec((s["position"] or {}).get("isolatedWallet")) or D(0)
        oim = dec(s["asset"]["openOrderInitialMargin"]) or D(0)
        pim = dec(s["asset"]["positionInitialMargin"]) or D(0)
        resid = (w - a) - (iso + oim)
        print(f"  {k:6s} {w - a:>16} {iso:>16} {oim:>14} {pim:>14} {resid:>12}")

    print("\n  => availableBalance = walletBalance - (isolatedWallet + openOrderIM),")
    print("     exactly, in every phase. Binance PUBLISHES the number Bybit makes")
    print("     us derive, and it is the SAME construction.")

    # ---- the trap: two same-shaped names that are different quantities
    d = by["D"]
    w = dec(d["asset"]["walletBalance"])
    iso = dec(d["position"]["isolatedWallet"])
    pim = dec(d["asset"]["positionInitialMargin"])
    a_true = dec(d["asset"]["availableBalance"])
    rule("BINANCE -- the name-matching trap (same shape as record 89 F2)")
    print("  Bybit's totalPositionIM ABSORBED the added 50 (13.10 -> 63.10):")
    print("     it is POSTED margin.")
    print(f"  Binance's positionInitialMargin did NOT ({pim}): it is the")
    print("     REQUIREMENT. The posted figure is isolatedWallet "
          f"({iso}).")
    print("\n  So an adapter that name-matches -- Bybit totalPositionIM, Binance")
    print("  positionInitialMargin -- computes on Binance:")
    print(f"     {w} - {pim} = {w - pim}")
    print(f"  against the venue's own answer:")
    print(f"     {a_true}")
    print(f"     OVERSTATES available balance by {w - pim - a_true}")
    print("  which is the whole added margin, in the direction that sizes a")
    print("  re-post the account cannot fund, on the incident path.")

    # ---- what this run did NOT establish
    rule("BINANCE -- what the run did NOT establish")
    split = False
    for k in "ABCDE":
        s = by[k]
        cands = {
            "account.availableBalance": dec(s["top"]["availableBalance"]),
            "account.maxWithdrawAmount": dec(s["top"]["maxWithdrawAmount"]),
            "assets.availableBalance": dec(s["asset"]["availableBalance"]),
            "assets.maxWithdrawAmount": dec(s["asset"]["maxWithdrawAmount"]),
            "balance.availableBalance": dec(s["bal"]["availableBalance"]),
            "balance.maxWithdrawAmount": dec(s["bal"]["maxWithdrawAmount"]),
        }
        n = len({str(v) for v in cands.values() if v is not None})
        upl = dec(s["asset"]["unrealizedProfit"])
        print(f"  {k}: distinct candidates = {n}   unrealizedProfit = {upl}")
        split = split or n > 1
    print("\n  availableBalance == maxWithdrawAmount in EVERY phase -- but every")
    print("  phase had unrealized profit <= 0. maxWithdrawAmount is the field that")
    print("  should exclude unrealized PROFIT; with a loss there is nothing to")
    print("  exclude, so the two are pinned equal by the sign of a quantity this")
    print("  run never controlled.")
    print("  => 'availableBalance and maxWithdrawAmount are the same field' is")
    print("     NOT established. The separating state is a position in PROFIT.")
    print("     Recorded as an open question rather than a measured equality.")
    return {"ever_split": split}


# ----------------------------------------------------- pass 1 and pass 2 readings
#
# These exist for TWO reasons and the second is the important one.
#
# (1) F3's decomposition and F1's table are findings and belong in the reading.
#
# (2) THE REDACTION CHECK WAS VACUOUS WITHOUT THEM. README's rule is "re-run the
#     consumer and confirm it still reaches the same number". The only file
#     redact225.py actually changed was probe225-pass1-readonly.json (8
#     accountAlias fields) -- and this analyzer originally read ONLY the pass-3
#     captures. So the diff was guaranteed identical by construction: the
#     consumer never opened the file that was mutated. A scrub that had removed
#     a monetary input from pass 1 would have produced exactly the same clean
#     diff. The consumer now reads every capture the redactor writes.

def pass1_reading():
    rule("PASS 1 -- the coin table F3 rests on (reads the REDACTED capture)")
    d = load("probe225-pass1-readonly.json")
    acct = d["bybit"]["unified"]["result"]["list"][0]
    print(f"  accountType={acct.get('accountType')} "
          f"totalEquity={acct.get('totalEquity')}")
    print(f"  {'coin':6s} {'walletBalance':>18s} {'usdValue':>18s} {'collateralSwitch':>17s}")
    tot = D(0)
    for c in acct["coin"]:
        print(f"  {c['coin']:6s} {c.get('walletBalance',''):>18s} "
              f"{c.get('usdValue',''):>18s} {str(c.get('collateralSwitch')):>17s}")
        tot += dec(c.get("usdValue")) or D(0)
    print(f"  Σ usdValue (all coins) = {tot}   totalEquity = {acct.get('totalEquity')}")
    print(f"  residual = {tot - (dec(acct.get('totalEquity')) or D(0))}")
    print("\n  Surfaces that do NOT carry A:")
    print(f"    wallet-balance accountType=CONTRACT     "
          f"retCode={d['bybit']['contract'].get('retCode')} "
          f"{d['bybit']['contract'].get('retMsg')!r}")
    for k in ("coins_balance", "withdrawable", "transfer_coins"):
        print(f"    {k:38s} retCode={d['bybit'][k].get('retCode')} "
              f"{d['bybit'][k].get('retMsg')!r}")
    # The redaction marker is data about the capture, so assert on it here.
    aliases = {b.get("accountAlias") for b in d["binance"]["balance"]}
    print(f"\n  redaction marker present on binance balance rows: {aliases}")


def f3_reading():
    rule("F3 -- what totalAvailableBalance actually IS (single REGULAR_MARGIN response)")
    d = load("probe225-pass2-marginmode.json")
    b = d["B_probed_regular"]["raw"]["result"]["list"][0]
    tab = dec(b.get("totalAvailableBalance"))
    s_all = s_col = D(0)
    for c in b["coin"]:
        uv = dec(c.get("usdValue")) or D(0)
        s_all += uv
        if c.get("collateralSwitch"):
            s_col += uv
    print(f"  totalAvailableBalance                       {tab}")
    print(f"  Σ usdValue where collateralSwitch == true   {s_col}   "
          f"residual {tab - s_col}")
    print(f"  Σ usdValue over ALL coins                   {s_all}   "
          f"(= totalEquity {b.get('totalEquity')})")
    usdt = next(c for c in b["coin"] if c["coin"] == "USDT")
    print(f"\n  USDT walletBalance {usdt['walletBalance']} vs its own usdValue "
          f"{usdt['usdValue']}")
    print(f"  => the peg is not 1; the difference is "
          f"{dec(usdt['walletBalance']) - dec(usdt['usdValue'])}")
    print(f"\n  Sizing a USDT re-post from the published field would overstate by")
    print(f"  the non-USDT collateral leg: {tab - dec(usdt['usdValue'])}")
    print("  against a USDT wallet of "
          f"{usdt['walletBalance']} -- very nearly 2x.")

    rule("F1 -- the flip / read / flip-back, and the restore IS the control")
    for key, lbl in (("A_baseline_isolated", "A. ISOLATED_MARGIN"),
                     ("B_probed_regular", "B. REGULAR_MARGIN"),
                     ("C_restored_isolated", "C. ISOLATED_MARGIN restored")):
        s = d.get(key)
        if not s:
            continue
        print(f"  {lbl:32s} populated={len(s['populated'])} mode={s['mode']}")
    a = set(d["A_baseline_isolated"]["populated"])
    bb = set(d["B_probed_regular"]["populated"])
    c = set(d["C_restored_isolated"]["populated"])
    print(f"\n  gained by the flip : {sorted(bb - a)}")
    print(f"  survived the restore: {sorted(c - a) or 'NOTHING'}")
    print(f"  => the fields are a function of MARGIN MODE"
          if c == a else "  => AMBIGUOUS")
    print(f"\n  availableToWithdraw under REGULAR_MARGIN: "
          f"{'populated' if 'USDT.availableToWithdraw' in bb else 'STILL EMPTY'}"
          "  <- F2: not the posture's doing")


def main():
    pass1_reading()
    f3_reading()
    bybit_reading()
    binance_reading()
    rule("CROSS-VENUE -- the identity test has no independent witness")
    print("  Binance's availableBalance is venue-PUBLISHED but measured here to be")
    print("  exactly walletBalance - (isolatedWallet + openOrderIM). Bybit's A is")
    print("  LOCALLY DERIVED by the same construction.")
    print("\n  So task #225's warning about tautology reaches further than it says:")
    print("  it warns that dA = released + realized - fee is tautological against")
    print("  the DERIVED Bybit A. It is also uninformative against the PUBLISHED")
    print("  Binance A, because that number is a function of the same inputs.")
    print("  Neither venue offers an independent witness for the identity; what")
    print("  each publishes independently is the wallet and the margin, and those")
    print("  are what a test may assert on.")


if __name__ == "__main__":
    main()
