#!/usr/bin/env python3
"""Task #225 pass 3 -- BREAK THE COLLAPSE. Writes, with cleanup in `finally`.

Pass 1 left two claims unfalsifiable, both for the same reason: the account was
FLAT, so every margin term was zero.

  Bybit    A_derived = walletBalance - totalPositionIM - totalOrderIM
           With both IM terms 0 this is walletBalance, so the run could not tell
           `A = W - IM` from `A = W`. The substantive question is whether
           walletBalance ALREADY nets out posted margin -- if it does, the
           subtraction DOUBLE-COUNTS it and sizes the re-post SHORT; if it does
           not, omitting the subtraction sizes it LONG. Both directions are
           wrong and the flat account reports neither.

  Binance  six candidate A's, one distinct value. Postable margin and
           withdrawable cash are the same number on a flat account with no
           unrealized PnL.

So this pass creates the two states that separate them, cheapest first:

  ORDER    a resting limit 4% below mark (Binance's PERCENT_PRICE floor is 5%).
           Makes ORDER margin non-zero with no fill, no PnL and no fee. This
           isolates the netting question from everything a position drags in.
  POSITION a small isolated market position, then an EXPLICIT add-margin -- the
           actual 7.1 operation. Order margin and position margin are different
           quantities and a fixture that only moves one is a fixture that does
           not contain the thing under test.

Every phase snapshots BEFORE and AFTER, and the report prints DELTAS, because
an absolute number agreeing with a formula on one reading is the weaker claim.

CLEANUP: cancel-all and flatten run in `finally` on every path. The script
re-reads and refuses to exit quiet if anything is left behind.
"""
import hashlib
import hmac
import json
import os
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from decimal import Decimal, getcontext

getcontext().prec = 40
D = Decimal

try:
    import certifi
    _CTX = ssl.create_default_context(cafile=certifi.where())
except ImportError:
    _CTX = ssl.create_default_context(cafile="/etc/ssl/cert.pem")

BYBIT = "api-demo.bybit.com"
BINANCE = "demo-fapi.binance.com"
SYMBOL = "BTCUSDT"
QTY = D(os.environ.get("ARBLAB_QTY", "0.002"))
ADD_MARGIN = D("50")
HERE = os.path.dirname(os.path.abspath(__file__))
ENV = {}


def load_env(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                ENV[k.strip()] = v.strip().strip('"').strip("'")


def dec(v):
    try:
        s = str(v)
        return D(s) if s not in ("", "None") else None
    except Exception:  # noqa: BLE001
        return None


def spell(v):
    if v is None:
        return "KEY-ABSENT"
    if v == "":
        return 'EMPTY ""'
    return str(v)


# ---------------------------------------------------------------- transports

def bb(method, path, payload=None, signed=True):
    key, sec = ENV["BYBIT_DEMO_API_KEY"], ENV["BYBIT_DEMO_API_SECRET"]
    ts, recv = str(int(time.time() * 1000)), "5000"
    hdrs = {}
    if method == "GET":
        q = urllib.parse.urlencode(payload or {})
        url = f"https://{BYBIT}{path}" + (f"?{q}" if q else "")
        body, sign_src = None, f"{ts}{key}{recv}{q}"
    else:
        raw = json.dumps(payload or {}, separators=(",", ":"))
        url = f"https://{BYBIT}{path}"
        body, sign_src = raw.encode(), f"{ts}{key}{recv}{raw}"
        hdrs["Content-Type"] = "application/json"
    if signed:
        hdrs.update({"X-BAPI-API-KEY": key, "X-BAPI-TIMESTAMP": ts,
                     "X-BAPI-RECV-WINDOW": recv,
                     "X-BAPI-SIGN": hmac.new(sec.encode(), sign_src.encode(),
                                             hashlib.sha256).hexdigest()})
    req = urllib.request.Request(url, data=body, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=25, context=_CTX) as r:
            return json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            return json.loads(raw)
        except Exception:  # noqa: BLE001
            return {"_http": e.code, "_raw": raw[:300]}
    except Exception as e:  # noqa: BLE001
        return {"_err": f"{type(e).__name__}: {e}"}


def bn(method, path, params=None, signed=True):
    key, sec = ENV["BINANCE_DEMO_API_KEY"], ENV["BINANCE_DEMO_API_SECRET"]
    params = dict(params or {})
    hdrs = {}
    if signed:
        params["timestamp"] = str(int(time.time() * 1000))
        params["recvWindow"] = "5000"
        q = urllib.parse.urlencode(params)
        q += "&signature=" + hmac.new(sec.encode(), q.encode(),
                                      hashlib.sha256).hexdigest()
        hdrs["X-MBX-APIKEY"] = key
    else:
        q = urllib.parse.urlencode(params)
    url = f"https://{BINANCE}{path}" + (f"?{q}" if q else "")
    req = urllib.request.Request(url, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=25, context=_CTX) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            return e.code, json.loads(raw)
        except Exception:  # noqa: BLE001
            return e.code, {"_raw": raw[:300]}
    except Exception as e:  # noqa: BLE001
        return 0, {"_err": f"{type(e).__name__}: {e}"}


# ================================================================== BYBIT

BB_FIELDS = ("walletBalance", "equity", "availableToWithdraw", "totalPositionIM",
             "totalOrderIM", "totalPositionMM", "unrealisedPnl", "locked")


def bb_snap(label):
    raw = bb("GET", "/v5/account/wallet-balance", {"accountType": "UNIFIED"})
    lst = ((raw.get("result") or {}).get("list")) or []
    acct = lst[0] if lst else {}
    coin = next((c for c in acct.get("coin", []) if c.get("coin") == "USDT"), {})
    pos = bb("GET", "/v5/position/list", {"category": "linear", "symbol": SYMBOL})
    prow = next((p for p in ((pos.get("result") or {}).get("list") or [])
                 if dec(p.get("size"))), None)
    s = {"label": label,
         "coin": {f: coin.get(f) for f in BB_FIELDS},
         "totalAvailableBalance": acct.get("totalAvailableBalance"),
         "position": {k: prow.get(k) for k in
                      ("size", "side", "positionIM", "positionMM", "avgPrice",
                       "unrealisedPnl", "liqPrice", "tradeMode", "leverage")}
         if prow else None,
         "raw_coin": coin}
    return s


def bb_print(s):
    print(f"\n  [{s['label']}]")
    for f in BB_FIELDS:
        print(f"     USDT.{f:22s} {spell(s['coin'].get(f))}")
    print(f"     account.totalAvailableBalance {spell(s['totalAvailableBalance'])}")
    if s["position"]:
        p = s["position"]
        print(f"     position: size={p['size']} side={p['side']} "
              f"positionIM={p['positionIM']} tradeMode={p['tradeMode']} "
              f"lev={p['leverage']} uPnL={p['unrealisedPnl']}")
    else:
        print("     position: FLAT")


def bb_derived(s):
    w = dec(s["coin"].get("walletBalance"))
    pim = dec(s["coin"].get("totalPositionIM"))
    oim = dec(s["coin"].get("totalOrderIM"))
    if None in (w, pim, oim):
        return None, None, None
    return w, pim + oim, w - pim - oim


def run_bybit(out):
    print("=" * 78)
    print("BYBIT demo -- does walletBalance ALREADY net out posted margin?")
    print("=" * 78)
    snaps = []

    tk = bb("GET", "/v5/market/tickers", {"category": "linear", "symbol": SYMBOL},
            signed=False)
    mark = dec(((tk.get("result") or {}).get("list") or [{}])[0].get("markPrice"))
    print(f"   mark = {mark}")
    limit_px = (mark * D("0.96")).quantize(D("0.1"))

    bb("POST", "/v5/position/set-leverage",
       {"category": "linear", "symbol": SYMBOL, "buyLeverage": "10", "sellLeverage": "10"})

    snaps.append(bb_snap("A. baseline (flat, no orders)"))
    bb_print(snaps[-1])

    order_id = None
    try:
        # ---------- phase B: resting order, no fill, no fee, no PnL
        r = bb("POST", "/v5/order/create",
               {"category": "linear", "symbol": SYMBOL, "side": "Buy",
                "orderType": "Limit", "qty": str(QTY), "price": str(limit_px),
                "timeInForce": "GTC"})
        print(f"\n   resting BUY {QTY} @ {limit_px}: retCode={r.get('retCode')} "
              f"retMsg={r.get('retMsg')!r}")
        order_id = ((r.get("result") or {}).get("orderId"))
        if order_id:
            time.sleep(2)
            snaps.append(bb_snap("B. resting order live (ORDER margin only)"))
            bb_print(snaps[-1])
            c = bb("POST", "/v5/order/cancel",
                   {"category": "linear", "symbol": SYMBOL, "orderId": order_id})
            print(f"   cancel: retCode={c.get('retCode')}")
            order_id = None
            time.sleep(2)

        # ---------- phase C: real isolated position, then an explicit add-margin
        r = bb("POST", "/v5/order/create",
               {"category": "linear", "symbol": SYMBOL, "side": "Buy",
                "orderType": "Market", "qty": str(QTY)})
        print(f"\n   market BUY {QTY}: retCode={r.get('retCode')} "
              f"retMsg={r.get('retMsg')!r}")
        time.sleep(3)
        snaps.append(bb_snap("C. position open (POSITION margin)"))
        bb_print(snaps[-1])

        am = bb("POST", "/v5/position/add-margin",
                {"category": "linear", "symbol": SYMBOL, "margin": str(ADD_MARGIN),
                 "positionIdx": 0})
        print(f"\n   add-margin +{ADD_MARGIN}: retCode={am.get('retCode')} "
              f"retMsg={am.get('retMsg')!r}")
        time.sleep(3)
        snaps.append(bb_snap("D. after add-margin (7.1's actual operation)"))
        bb_print(snaps[-1])
    finally:
        print("\n   CLEANUP")
        if order_id:
            print(f"     cancel {order_id}: "
                  f"{bb('POST', '/v5/order/cancel', {'category': 'linear', 'symbol': SYMBOL, 'orderId': order_id}).get('retCode')}")
        ca = bb("POST", "/v5/order/cancel-all", {"category": "linear", "symbol": SYMBOL})
        print(f"     cancel-all: retCode={ca.get('retCode')}")
        pos = bb("GET", "/v5/position/list", {"category": "linear", "symbol": SYMBOL})
        prow = next((p for p in ((pos.get("result") or {}).get("list") or [])
                     if dec(p.get("size"))), None)
        if prow:
            side = "Sell" if prow.get("side") == "Buy" else "Buy"
            cl = bb("POST", "/v5/order/create",
                    {"category": "linear", "symbol": SYMBOL, "side": side,
                     "orderType": "Market", "qty": str(prow.get("size")),
                     "reduceOnly": True})
            print(f"     flatten {side} {prow.get('size')}: retCode={cl.get('retCode')} "
                  f"retMsg={cl.get('retMsg')!r}")
            time.sleep(3)
        snaps.append(bb_snap("E. final (must be back to flat)"))
        bb_print(snaps[-1])

    # ----------------------------------------------------------- the reading
    print("\n" + "-" * 78)
    print("  BYBIT READING -- deltas against baseline")
    print("-" * 78)
    base_w, base_im, base_a = bb_derived(snaps[0])
    print(f"  {'phase':44s} {'walletBalance':>18s} {'IM(pos+ord)':>14s} {'W-IM':>16s}")
    for s in snaps:
        w, im, a = bb_derived(s)
        if w is None:
            print(f"  {s['label']:44s} {'NOT COMPUTABLE':>18s}")
            continue
        print(f"  {s['label']:44s} {w:>18} {im:>14} {a:>16}")
    print()
    for s in snaps[1:]:
        w, im, a = bb_derived(s)
        if w is None or base_w is None:
            continue
        dw, dim = w - base_w, im - base_im
        print(f"  {s['label']:44s} dW={dw:>16} dIM={dim:>12}")
    print("\n  THE QUESTION: when IM went up, did walletBalance move with it?")
    moved = [s for s in snaps[1:] if bb_derived(s)[1] and bb_derived(s)[1] != base_im]
    if not moved:
        print("     *** IM NEVER MOVED -- the collapse is NOT broken and nothing here")
        print("         separates `A = W - IM` from `A = W`.")
        out["bybit"] = snaps
        return

    # Only phases that move margin ALONE are admissible evidence. A market fill
    # moves margin AND charges a taker fee, so `did walletBalance move?` cannot
    # attribute the change to either -- the first draft of this script read
    # phase C that way and drew the OPPOSITE conclusion from the right data.
    # add-margin is the isolating phase: no fill, no fee, no PnL, and it is the
    # operation 7.1 actually issues.
    ISOLATING = ("B.", "D.")
    for s in moved:
        w, im, _ = bb_derived(s)
        dw, dim = w - base_w, im - base_im
        if not s["label"].startswith(ISOLATING):
            print(f"     {s['label']}: IM +{dim}, dW={dw} -- NOT ADMISSIBLE, this phase"
                  f"\n        also charged a fee; the delta is not attributable to margin.")
            continue
        prev = snaps[snaps.index(s) - 1]
        pw = bb_derived(prev)[0]
        verdict = ("W is GROSS, the subtraction is REQUIRED" if w == pw
                   else "W already nets margin; subtracting DOUBLE-COUNTS")
        print(f"     {s['label']}: margin moved alone, dW vs previous phase = {w - pw}"
              f"\n        => {verdict}")
    print("\n     The full reading, recomputed from the saved capture (including the"
          "\n     fee reconstruction that disqualifies phase C): analyze225.py")
    out["bybit"] = snaps


# ================================================================== BINANCE

def bn_snap(label):
    _, acct = bn("GET", "/fapi/v3/account")
    _, bal = bn("GET", "/fapi/v3/balance")
    _, pos = bn("GET", "/fapi/v3/positionRisk", {"symbol": SYMBOL})
    a_usdt = next((a for a in (acct.get("assets") or []) if a.get("asset") == "USDT"),
                  {}) if isinstance(acct, dict) else {}
    b_usdt = next((b for b in bal if b.get("asset") == "USDT"), {}) \
        if isinstance(bal, list) else {}
    prow = next((p for p in pos if dec(p.get("positionAmt"))), None) \
        if isinstance(pos, list) else None
    return {"label": label,
            "top": {k: (acct.get(k) if isinstance(acct, dict) else None) for k in
                    ("totalWalletBalance", "totalMarginBalance", "availableBalance",
                     "maxWithdrawAmount", "totalInitialMargin",
                     "totalPositionInitialMargin", "totalOpenOrderInitialMargin",
                     "totalUnrealizedProfit")},
            "asset": {k: a_usdt.get(k) for k in
                      ("walletBalance", "marginBalance", "availableBalance",
                       "maxWithdrawAmount", "initialMargin", "positionInitialMargin",
                       "openOrderInitialMargin", "unrealizedProfit",
                       "crossWalletBalance")},
            "bal": {k: b_usdt.get(k) for k in
                    ("balance", "crossWalletBalance", "crossUnPnl",
                     "availableBalance", "maxWithdrawAmount")},
            "position": {k: prow.get(k) for k in
                         ("positionAmt", "entryPrice", "isolated", "isolatedWallet",
                          "isolatedMargin", "unRealizedProfit", "liquidationPrice",
                          "initialMargin", "maintMargin")} if prow else None}


def bn_print(s):
    print(f"\n  [{s['label']}]")
    for k, v in s["top"].items():
        print(f"     account.{k:30s} {spell(v)}")
    for k in ("walletBalance", "availableBalance", "maxWithdrawAmount",
              "initialMargin", "positionInitialMargin", "openOrderInitialMargin",
              "unrealizedProfit"):
        print(f"     assets[USDT].{k:25s} {spell(s['asset'].get(k))}")
    for k in ("availableBalance", "maxWithdrawAmount"):
        print(f"     balance[USDT].{k:24s} {spell(s['bal'].get(k))}")
    if s["position"]:
        p = s["position"]
        print(f"     position: amt={p['positionAmt']} isolated={p['isolated']} "
              f"isolatedWallet={p['isolatedWallet']} uPnL={p['unRealizedProfit']}")
    else:
        print("     position: FLAT")


def bn_candidates(s):
    return {
        "account.availableBalance": dec(s["top"].get("availableBalance")),
        "account.maxWithdrawAmount": dec(s["top"].get("maxWithdrawAmount")),
        "assets[USDT].availableBalance": dec(s["asset"].get("availableBalance")),
        "assets[USDT].maxWithdrawAmount": dec(s["asset"].get("maxWithdrawAmount")),
        "balance[USDT].availableBalance": dec(s["bal"].get("availableBalance")),
        "balance[USDT].maxWithdrawAmount": dec(s["bal"].get("maxWithdrawAmount")),
    }


def run_binance(out):
    print("\n" + "=" * 78)
    print("BINANCE demo -- do the six candidate A's ever DISAGREE?")
    print("=" * 78)
    snaps = []
    _, pr = bn("GET", "/fapi/v1/premiumIndex", {"symbol": SYMBOL}, signed=False)
    mark = dec(pr.get("markPrice")) if isinstance(pr, dict) else None
    print(f"   mark = {mark}")
    limit_px = (mark * D("0.96")).quantize(D("0.1"))

    print("   set ISOLATED:", bn("POST", "/fapi/v1/marginType",
                                 {"symbol": SYMBOL, "marginType": "ISOLATED"})[1])
    print("   set leverage:", bn("POST", "/fapi/v1/leverage",
                                 {"symbol": SYMBOL, "leverage": "10"})[1])

    snaps.append(bn_snap("A. baseline (flat, no orders)"))
    bn_print(snaps[-1])

    oid = None
    try:
        st, r = bn("POST", "/fapi/v1/order",
                   {"symbol": SYMBOL, "side": "BUY", "type": "LIMIT",
                    "quantity": str(QTY), "price": str(limit_px),
                    "timeInForce": "GTC"})
        print(f"\n   resting BUY {QTY} @ {limit_px}: http={st} {r if st != 200 else 'ok'}")
        if st == 200:
            oid = r.get("orderId")
            time.sleep(2)
            snaps.append(bn_snap("B. resting order live (ORDER margin only)"))
            bn_print(snaps[-1])
            st, c = bn("DELETE", "/fapi/v1/order", {"symbol": SYMBOL, "orderId": str(oid)})
            print(f"   cancel: http={st}")
            oid = None
            time.sleep(2)

        st, r = bn("POST", "/fapi/v1/order",
                   {"symbol": SYMBOL, "side": "BUY", "type": "MARKET",
                    "quantity": str(QTY)})
        print(f"\n   market BUY {QTY}: http={st} {r if st != 200 else 'ok'}")
        time.sleep(3)
        snaps.append(bn_snap("C. isolated position open"))
        bn_print(snaps[-1])

        st, am = bn("POST", "/fapi/v1/positionMargin",
                    {"symbol": SYMBOL, "amount": str(ADD_MARGIN), "type": "1"})
        print(f"\n   add-margin +{ADD_MARGIN}: http={st} {am}")
        time.sleep(3)
        snaps.append(bn_snap("D. after add-margin (7.1's actual operation)"))
        bn_print(snaps[-1])
    finally:
        print("\n   CLEANUP")
        st, _ = bn("DELETE", "/fapi/v1/allOpenOrders", {"symbol": SYMBOL})
        print(f"     cancel-all: http={st}")
        _, pos = bn("GET", "/fapi/v3/positionRisk", {"symbol": SYMBOL})
        prow = next((p for p in pos if dec(p.get("positionAmt"))), None) \
            if isinstance(pos, list) else None
        if prow:
            amt = dec(prow["positionAmt"])
            st, cl = bn("POST", "/fapi/v1/order",
                        {"symbol": SYMBOL, "side": "SELL" if amt > 0 else "BUY",
                         "type": "MARKET", "quantity": str(abs(amt)),
                         "reduceOnly": "true"})
            print(f"     flatten {abs(amt)}: http={st} {cl if st != 200 else 'ok'}")
            time.sleep(3)
        snaps.append(bn_snap("E. final (must be back to flat)"))
        bn_print(snaps[-1])

    print("\n" + "-" * 78)
    print("  BINANCE READING -- distinct candidate values per phase")
    print("-" * 78)
    ever_split = False
    for s in snaps:
        c = bn_candidates(s)
        distinct = sorted({str(v) for v in c.values() if v is not None})
        flag = "SEPARATED" if len(distinct) > 1 else "collapsed"
        print(f"\n  [{s['label']}] {len(distinct)} distinct -> {flag}")
        if len(distinct) > 1:
            ever_split = True
            for k, v in c.items():
                print(f"       {k:34s} {v}")
        w = dec(s["asset"].get("walletBalance"))
        iso = dec((s["position"] or {}).get("isolatedWallet"))
        av = c["assets[USDT].availableBalance"]
        if None not in (w, av):
            print(f"       walletBalance - availableBalance = {w - av}"
                  + (f"   (isolatedWallet = {iso})" if iso is not None else ""))
    if not ever_split:
        print("\n  *** NO PHASE SEPARATED THE CANDIDATES. Any claim about WHICH field")
        print("      is A would rest on a run in which they were all the same number.")
    out["binance"] = snaps


def main():
    load_env(os.environ.get("ARBLAB_ENV",
                            os.path.join(HERE, "..", "..", "..", ".env")))
    which = sys.argv[1] if len(sys.argv) > 1 else "both"
    out = {}
    if which in ("both", "bybit"):
        run_bybit(out)
    if which in ("both", "binance"):
        run_binance(out)
    dest = os.path.join(HERE, "captures", f"probe225-pass3-{which}.json")
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    with open(dest, "w") as fh:
        json.dump(out, fh, indent=1, sort_keys=True, default=str)
    print(f"\nsnapshots -> {dest}")


if __name__ == "__main__":
    sys.exit(main())
