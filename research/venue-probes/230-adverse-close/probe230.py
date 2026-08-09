#!/usr/bin/env python3
"""Task #230 — the adverse-close measurement record §87 §7 owes. BINANCE demo.

#221 measured a partial isolated close at a mark near ENTRY, where the realized
term r*(e-P) is ~0 -- so "margin released" and "cash received" were the same
number and the design's two quantities were pinned equal by the price the run
happened to be taken at. This probe breaks that collapse deliberately.

The claim under test (record 87 R1a):

    dA = (1-alpha)*m + r*(e-P) - fees          [+ funding, if any settles]

rather than dA = (1-alpha)*m.

Everything is Decimal; the venue's own income ledger is pulled so the identity
is reconciled against venue-reported terms rather than against our arithmetic.

Phases (read-only first, nothing is sent before its precondition is read back):

    state    balances, position row, mark, brackets, server time, funding clock
    open     set ISOLATED + leverage, market SELL, snapshot
    watch    poll until the position is materially ADVERSE (or favourable), no writes
    half     the measured partial close: full pre-state, reduce-only BUY, full post-state
    verify   reconcile dA against the ledger; predicted vs observed liqPrice
    close    cleanup: flatten
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

# This framework Python ships no CA bundle, so the default context cannot verify
# anything. These calls carry an API key and a signature; verification stays ON
# and we hand it a real bundle instead.
try:
    import certifi
    _CTX = ssl.create_default_context(cafile=certifi.where())
except ImportError:
    _CTX = ssl.create_default_context(cafile="/etc/ssl/cert.pem")

SYMBOL = os.environ.get("ARBLAB_SYMBOL", "BTCUSDT")
FUT = "demo-fapi.binance.com"
ENV = {}
STATE = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                     "probe230-binance-%s.json" % SYMBOL)

D = Decimal


def load_env(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                ENV[k.strip()] = v.strip().strip('"').strip("'")


def call(method, path, params=None, signed=True):
    key, sec = ENV["BINANCE_DEMO_API_KEY"], ENV["BINANCE_DEMO_API_SECRET"]
    params = dict(params or {})
    hdrs = {}
    if signed:
        params["timestamp"] = str(int(time.time() * 1000))
        params["recvWindow"] = "5000"
        q = urllib.parse.urlencode(params)
        q = q + "&signature=" + hmac.new(sec.encode(), q.encode(), hashlib.sha256).hexdigest()
        hdrs["X-MBX-APIKEY"] = key
    else:
        q = urllib.parse.urlencode(params)
    url = f"https://{FUT}{path}" + (f"?{q}" if q else "")
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


# ---------------------------------------------------------------- reads

def position():
    st, j = call("GET", "/fapi/v3/positionRisk", {"symbol": SYMBOL})
    if st != 200 or not isinstance(j, list):
        return None
    live = [r for r in j if r.get("symbol") == SYMBOL]
    return live[0] if live else None


def balance():
    st, j = call("GET", "/fapi/v2/balance")
    if st != 200 or not isinstance(j, list):
        return None
    return next((b for b in j if b.get("asset") == "USDT"), None)


def brackets():
    st, j = call("GET", "/fapi/v1/leverageBracket", {"symbol": SYMBOL})
    if st != 200 or not j:
        return []
    return j[0].get("brackets", [])


def bracket_for(notional, brs):
    n = abs(D(str(notional)))
    for b in brs:
        if D(str(b["notionalFloor"])) <= n < D(str(b["notionalCap"])):
            return b
    return brs[-1] if brs else None


def snapshot(tag):
    """Everything the identity needs, in one atomic-ish read."""
    st, sv = call("GET", "/fapi/v1/time", signed=False)
    p, b = position(), balance()
    snap = {
        "tag": tag,
        "serverTime": sv.get("serverTime") if isinstance(sv, dict) else None,
        "position": p,
        "balance": b,
    }
    return snap


def fmt_snapshot(s):
    p, b = s.get("position") or {}, s.get("balance") or {}
    out = [f"  [{s['tag']}] t={s.get('serverTime')}"]
    if b:
        out.append("    wallet  balance=%s  available=%s  maxWithdraw=%s  crossUnPnl=%s"
                   % (b.get("balance"), b.get("availableBalance"),
                      b.get("maxWithdrawAmount"), b.get("crossUnPnl")))
    if p:
        out.append("    pos     amt=%s entry=%s mark=%s liq=%s isoWallet=%s isoMargin=%s "
                   "notional=%s maintMargin=%s uPnl=%s lev=%s marginType=%s"
                   % (p.get("positionAmt"), p.get("entryPrice"), p.get("markPrice"),
                      p.get("liquidationPrice"), p.get("isolatedWallet"),
                      p.get("isolatedMargin"), p.get("notional"),
                      p.get("maintMargin"), p.get("unRealizedProfit"),
                      p.get("leverage"), p.get("marginType")))
    return "\n".join(out)


def income(start_ms, end_ms=None):
    params = {"symbol": SYMBOL, "startTime": str(start_ms), "limit": "500"}
    if end_ms:
        params["endTime"] = str(end_ms)
    st, j = call("GET", "/fapi/v1/income", params)
    return j if st == 200 and isinstance(j, list) else []


def trades(start_ms):
    st, j = call("GET", "/fapi/v1/userTrades",
                 {"symbol": SYMBOL, "startTime": str(start_ms), "limit": "200"})
    return j if st == 200 and isinstance(j, list) else []


def order(side, qty, reduce_only=False):
    params = {"symbol": SYMBOL, "side": side, "type": "MARKET", "quantity": qty}
    if reduce_only:
        params["reduceOnly"] = "true"
    st, j = call("POST", "/fapi/v1/order", params)
    print("  %s %s reduceOnly=%s -> %s %s" % (
        side, qty, reduce_only, st,
        json.dumps({k: j.get(k) for k in ("orderId", "status", "executedQty",
                                          "avgPrice", "code", "msg") if k in j})))
    return st, j


def save(key, value):
    d = {}
    if os.path.exists(STATE):
        with open(STATE) as fh:
            d = json.load(fh)
    d[key] = value
    with open(STATE, "w") as fh:
        json.dump(d, fh, indent=1)


def loadst(key, default=None):
    if not os.path.exists(STATE):
        return default
    with open(STATE) as fh:
        return json.load(fh).get(key, default)


# ---------------------------------------------------------------- phases

def main():
    load_env(os.environ["ARBLAB_ENV"])
    phase = sys.argv[1]

    if phase == "state":
        s = snapshot("now")
        print(fmt_snapshot(s))
        st, mk = call("GET", "/fapi/v1/premiumIndex", {"symbol": SYMBOL}, signed=False)
        print("  mark=%s index=%s lastFundingRate=%s nextFundingTime=%s"
              % (mk.get("markPrice"), mk.get("indexPrice"),
                 mk.get("lastFundingRate"), mk.get("nextFundingTime")))
        if mk.get("nextFundingTime"):
            dt = (int(mk["nextFundingTime"]) - int(s["serverTime"])) / 60000.0
            print("  next funding in %.1f min  <-- a settlement inside the window credits A too"
                  % dt)
        brs = brackets()
        print("  brackets:", json.dumps(brs[:4]))
        st, ei = call("GET", "/fapi/v1/exchangeInfo", signed=False)
        sym = next((x for x in ei.get("symbols", []) if x["symbol"] == SYMBOL), None)
        if sym:
            for f in sym["filters"]:
                if f["filterType"] in {"LOT_SIZE", "MARKET_LOT_SIZE", "MIN_NOTIONAL"}:
                    print("  filter", json.dumps(f))

    elif phase == "book":
        # The spread is the DETERMINISTIC route to a materially adverse close:
        # a short opened and closed with market orders sells at the bid and buys
        # back at the ask, so e - P = -(spread) with no waiting on the market.
        st, b = call("GET", "/fapi/v1/ticker/bookTicker", {"symbol": SYMBOL},
                     signed=False)
        bid, ask = D(b["bidPrice"]), D(b["askPrice"])
        print("  %s  bid=%s (%s)  ask=%s (%s)  spread=%s (%.4f%%)"
              % (SYMBOL, bid, b["bidQty"], ask, b["askQty"], ask - bid,
                 (ask - bid) / bid * 100))
        st, mk = call("GET", "/fapi/v1/premiumIndex", {"symbol": SYMBOL}, signed=False)
        print("  mark=%s  index=%s" % (mk.get("markPrice"), mk.get("indexPrice")))
        st, ei = call("GET", "/fapi/v1/exchangeInfo", signed=False)
        sym = next((x for x in ei.get("symbols", []) if x["symbol"] == SYMBOL), None)
        if sym:
            for f in sym["filters"]:
                if f["filterType"] in {"LOT_SIZE", "MARKET_LOT_SIZE", "MIN_NOTIONAL",
                                       "PRICE_FILTER"}:
                    print("  filter", json.dumps(f))
        print("  brackets:", json.dumps(brackets()[:3]))

    elif phase == "open":
        qty, lev = sys.argv[2], sys.argv[3]
        # The tier-basis MIRROR needs a LONG: for a short q*L* exceeds the
        # notional, for a long it falls below it, so the two sides select
        # brackets in OPPOSITE directions. That is what separates "the bracket
        # is the self-consistent one at q*L*" from "the bracket is the next one up".
        side = sys.argv[4] if len(sys.argv) > 4 else "SELL"
        st, j = call("POST", "/fapi/v1/marginType",
                     {"symbol": SYMBOL, "marginType": "ISOLATED"})
        print("  set ISOLATED -> %s %s  (-4046 = already isolated)" % (st, j))
        st, j = call("POST", "/fapi/v1/leverage", {"symbol": SYMBOL, "leverage": lev})
        print("  set leverage %s -> %s %s" % (lev, st, j))
        pre = snapshot("before-open")
        print(fmt_snapshot(pre))
        t0 = pre["serverTime"]
        order(side, qty)
        time.sleep(2)
        post = snapshot("after-open")
        print(fmt_snapshot(post))
        save("openT0", t0)
        save("open", post)

    elif phase == "watch":
        thresh = D(sys.argv[2]) if len(sys.argv) > 2 else D("0.0015")
        limit = int(sys.argv[3]) if len(sys.argv) > 3 else 40
        for i in range(limit):
            p = position()
            if not p or D(p["positionAmt"]) == 0:
                print("  flat"); return
            e, mk = D(p["entryPrice"]), D(p["markPrice"])
            amt = D(p["positionAmt"])
            delta = (mk - e) / e
            adverse = (delta > 0) if amt < 0 else (delta < 0)
            print("  %2d  mark=%s entry=%s  delta=%+.5f%%  uPnl=%s  %s"
                  % (i, mk, e, delta * 100, p.get("unRealizedProfit"),
                     "ADVERSE" if adverse else "favourable"))
            if adverse and abs(delta) >= thresh:
                print("  -> threshold %s reached; run `half`" % thresh)
                return
            time.sleep(20)
        print("  timeout without reaching %s" % thresh)

    elif phase == "half":
        r = sys.argv[2]
        pre = snapshot("before-half")
        print(fmt_snapshot(pre))
        brs = brackets()
        p = pre["position"]
        br = bracket_for(p["notional"], brs)
        print("  bracket:", json.dumps(br))
        save("brackets", brs)
        save("pre_half", pre)
        st, j = order("BUY", r, reduce_only=True)
        save("half_order", j)
        time.sleep(3)
        post = snapshot("after-half")
        print(fmt_snapshot(post))
        save("post_half", post)
        save("r", r)

    elif phase == "repost":
        # 87 R1a's re-post rung, and plan item 4: predicted-vs-observed liqPrice
        # with a SEPARATELY IDENTIFIED added-margin term. A single predicted/
        # observed point is satisfiable by more than one candidate formula, so
        # the added margin has to move independently of m0, q and e.
        add = sys.argv[2]
        pre = snapshot("before-repost")
        print(fmt_snapshot(pre))
        st, j = call("POST", "/fapi/v1/positionMargin",
                     {"symbol": SYMBOL, "amount": add, "type": "1"})
        print("  add margin %s -> %s %s" % (add, st, j))
        time.sleep(2)
        post = snapshot("after-repost")
        print(fmt_snapshot(post))
        save("pre_repost", pre)
        save("post_repost", post)
        p = post["position"]
        if p and D(p["positionAmt"]) != 0:
            brs = loadst("brackets") or brackets()
            br = bracket_for(p["notional"], brs)
            mmr, cum = D(str(br["maintMarginRatio"])), D(str(br["cum"]))
            m, q, e = D(p["isolatedWallet"]), abs(D(p["positionAmt"])), D(p["entryPrice"])
            L = (m / q + e + cum / q) / (1 + mmr)
            print("\n  predicted L = (m/q + e + cum/q)/(1+MMR) = %s" % L)
            print("  observed  liquidationPrice              = %s" % p["liquidationPrice"])
            print("  error                                   = %s"
                  % (L - D(p["liquidationPrice"])))
            pm = pre["position"]
            if pm:
                print("  (liq moved %s -> %s on an added-margin term of %s)"
                      % (pm["liquidationPrice"], p["liquidationPrice"], add))

    elif phase == "verify":
        pre, post = loadst("pre_half"), loadst("post_half")
        r = D(loadst("r"))
        verify(pre, post, r)

    elif phase == "ledger":
        t0 = int(sys.argv[2])
        for it in income(t0):
            print("  income %s  %s %s  time=%s  info=%s"
                  % (it.get("incomeType"), it.get("income"), it.get("asset"),
                     it.get("time"), it.get("info")))
        for t in trades(t0):
            print("  trade  id=%s side=%s qty=%s price=%s realized=%s fee=%s %s time=%s"
                  % (t.get("id"), t.get("side"), t.get("qty"), t.get("price"),
                     t.get("realizedPnl"), t.get("commission"),
                     t.get("commissionAsset"), t.get("time")))

    elif phase == "close":
        p = position()
        if p and D(p["positionAmt"]) != 0:
            q = abs(D(p["positionAmt"]))
            # NOT rstrip("0"): "7500.000000" -> "75." -> "75" closes 1% of the
            # position and reports success. Only strip zeros AFTER a ".".
            s = f"{q:f}"
            if "." in s:
                s = s.rstrip("0").rstrip(".")
            order("BUY" if D(p["positionAmt"]) < 0 else "SELL", s, reduce_only=True)
            time.sleep(2)
        print(fmt_snapshot(snapshot("after-close")))


def verify(pre, post, r):
    """Reconcile dA against the venue's own ledger."""
    pp, qp = pre["position"], post["position"]
    pb, qb = pre["balance"], post["balance"]

    m0, m1 = D(pp["isolatedWallet"]), D(qp["isolatedWallet"] or "0")
    q0, q1 = abs(D(pp["positionAmt"])), abs(D(qp["positionAmt"] or "0"))
    e = D(pp["entryPrice"])
    A0, A1 = D(pb["availableBalance"]), D(qb["availableBalance"])
    W0, W1 = D(pb["balance"]), D(qb["balance"])

    alpha = q1 / q0
    dA, dW, dm = A1 - A0, W1 - W0, m1 - m0

    tl = trades(int(pre["serverTime"]) - 1000)
    closing = [t for t in tl if int(t["time"]) >= int(pre["serverTime"]) - 1000]
    realized = sum((D(t["realizedPnl"]) for t in closing), D(0))
    fee = sum((D(t["commission"]) for t in closing), D(0))
    P = (sum((D(t["price"]) * D(t["qty"]) for t in closing), D(0))
         / sum((D(t["qty"]) for t in closing), D(0))) if closing else D(0)

    inc = income(int(pre["serverTime"]) - 1000)
    other = [i for i in inc if i.get("incomeType") not in ("REALIZED_PNL", "COMMISSION")]

    print("\n  === measured ===")
    print("  q  %s -> %s   alpha = q1/q0 = %s" % (q0, q1, alpha))
    print("  m  %s -> %s   dm = %s        (released = %s)" % (m0, m1, dm, -dm))
    print("  A  %s -> %s   dA = %s" % (A0, A1, dA))
    print("  W  %s -> %s   dW = %s   (wallet: realized - fee + funding)" % (W0, W1, dW))
    print("  e  = %s   P = %s   e-P = %s   r = %s" % (e, P, e - P, r))
    print("  venue-reported: realizedPnl = %s   commission = %s" % (realized, fee))
    if other:
        print("  OTHER income in window (confounder):",
              json.dumps([(i["incomeType"], i["income"], i["time"]) for i in other]))

    naive = (1 - alpha) * m0
    model = naive + r * (e - P) - fee
    ledger = (-dm) + realized - fee
    print("\n  === the two readings ===")
    print("  #221's reading      (1-alpha)*m          = %s" % naive)
    print("  87 R1a's reading    + r*(e-P) - fees     = %s" % model)
    print("  ledger form         -dm + realized - fee = %s" % ledger)
    print("  observed dA                              = %s" % dA)
    print("\n  R1a error vs observed  = %s" % (model - dA))
    print("  #221 error vs observed = %s" % (naive - dA))
    print("  ledger error           = %s" % (ledger - dA))
    print("  the realized term      = %s   (%.3f%% of released margin)"
          % (r * (e - P), (r * (e - P) / (-dm) * 100) if dm else 0))

    # liq price, predicted vs observed, on the POST state
    brs = loadst("brackets") or brackets()
    if qp and q1 > 0:
        br = bracket_for(qp["notional"], brs)
        mmr, cum = D(str(br["maintMarginRatio"])), D(str(br["cum"]))
        L = (m1 / q1 + D(qp["entryPrice"]) + cum / q1) / (1 + mmr)
        print("\n  === liq price, post-close ===")
        print("  bracket mmr=%s cum=%s" % (mmr, cum))
        print("  predicted L = (m/q + e + cum/q)/(1+MMR) = %s" % L)
        print("  observed  liquidationPrice              = %s" % qp["liquidationPrice"])
        print("  error                                   = %s"
              % (L - D(qp["liquidationPrice"])))


if __name__ == "__main__":
    main()
