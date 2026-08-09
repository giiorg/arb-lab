#!/usr/bin/env python3
"""Task #230 — the adverse-close measurement, BYBIT demo. Mirrors probe230.py.

Same claim under test (record 87 R1a):

    dA = (1-alpha)*m + r*(e-P) - fees          [+ funding, if any settles]

Bybit's names for the same quantities:

    m  positionIM          (Binance isolatedWallet)
    q  size
    e  avgPrice
    A  totalAvailableBalance / coin availableToWithdraw
    L  liqPrice
    ledger  /v5/execution/list (execPrice, execFee, closedSize)
            /v5/position/closed-pnl (closedPnl, avgExitPrice, openFee, closeFee)

87 section 5c: Bybit's published liquidation material carries leverage, taker-fee
and extra-margin terms the common relation does not, and section 85 verified the
relation on BINANCE ONLY -- so here predicted-vs-observed is a genuine test of the
formula, not a confirmation of arithmetic. Two ORTHOGONAL cases are needed
(section 7.6): a single point is satisfiable by more than one candidate formula,
which is why the added-margin term is moved on its own in `repost`.
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

try:
    import certifi
    _CTX = ssl.create_default_context(cafile=certifi.where())
except ImportError:
    _CTX = ssl.create_default_context(cafile="/etc/ssl/cert.pem")

HOST = "api-demo.bybit.com"
SYMBOL = os.environ.get("ARBLAB_SYMBOL", "BTCUSDT")
ENV = {}
STATE = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                     "probe230-bybit-%s.json" % SYMBOL)

D = Decimal


def load_env(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                ENV[k.strip()] = v.strip().strip('"').strip("'")


def call(method, path, payload=None, signed=True):
    key, sec = ENV["BYBIT_DEMO_API_KEY"], ENV["BYBIT_DEMO_API_SECRET"]
    ts, recv = str(int(time.time() * 1000)), "5000"
    hdrs = {}
    if method == "GET":
        q = urllib.parse.urlencode(payload or {})
        url = f"https://{HOST}{path}" + (f"?{q}" if q else "")
        body, sign_src = None, f"{ts}{key}{recv}{q}"
    else:
        raw = json.dumps(payload or {}, separators=(",", ":"))
        url = f"https://{HOST}{path}"
        body, sign_src = raw.encode(), f"{ts}{key}{recv}{raw}"
        hdrs["Content-Type"] = "application/json"
    if signed:
        hdrs.update({
            "X-BAPI-API-KEY": key, "X-BAPI-TIMESTAMP": ts,
            "X-BAPI-RECV-WINDOW": recv,
            "X-BAPI-SIGN": hmac.new(sec.encode(), sign_src.encode(),
                                    hashlib.sha256).hexdigest(),
        })
    req = urllib.request.Request(url, data=body, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=25, context=_CTX) as r:
            return json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return {"_http": e.code, "_raw": e.read().decode("utf-8", "replace")[:300]}
    except Exception as e:  # noqa: BLE001
        return {"_err": f"{type(e).__name__}: {e}"}


# ---------------------------------------------------------------- reads

def position():
    j = call("GET", "/v5/position/list", {"category": "linear", "symbol": SYMBOL})
    lst = ((j.get("result") or {}).get("list")) or []
    live = [r for r in lst if r.get("symbol") == SYMBOL and D(r.get("size") or "0") != 0]
    return live[0] if live else None


def wallet():
    j = call("GET", "/v5/account/wallet-balance", {"accountType": "UNIFIED"})
    lst = ((j.get("result") or {}).get("list")) or []
    if not lst:
        return None, j
    acct = lst[0]
    coin = next((c for c in acct.get("coin", []) if c.get("coin") == "USDT"), None)
    return {"account": {k: acct.get(k) for k in
                        ("totalEquity", "totalAvailableBalance", "totalMarginBalance",
                         "totalInitialMargin", "totalMaintenanceMargin", "accountIMRate")},
            "usdt": coin}, j


def risklimit():
    j = call("GET", "/v5/market/risk-limit",
             {"category": "linear", "symbol": SYMBOL}, signed=False)
    return ((j.get("result") or {}).get("list")) or []


def tier_for(notional, tiers):
    n = abs(D(str(notional)))
    for t in tiers:
        if n <= D(str(t["riskLimitValue"])):
            return t
    return tiers[-1] if tiers else None


def snapshot(tag):
    p = position()
    w, _ = wallet()
    return {"tag": tag, "t": int(time.time() * 1000), "position": p, "wallet": w}


def fmt_snapshot(s):
    p, w = s.get("position"), s.get("wallet") or {}
    out = [f"  [{s['tag']}] t={s.get('t')}"]
    a, c = w.get("account") or {}, w.get("usdt") or {}
    if a:
        out.append("    acct    equity=%s availBal=%s IM=%s MM=%s"
                   % (a.get("totalEquity"), a.get("totalAvailableBalance"),
                      a.get("totalInitialMargin"), a.get("totalMaintenanceMargin")))
    if c:
        out.append("    usdt    wallet=%s availToWithdraw=%s equity=%s unrealised=%s "
                   "totalPositionIM=%s"
                   % (c.get("walletBalance"), c.get("availableToWithdraw"),
                      c.get("equity"), c.get("unrealisedPnl"),
                      c.get("totalPositionIM")))
    if p:
        out.append("    pos     size=%s side=%s avg=%s mark=%s liq=%s positionIM=%s "
                   "positionMM=%s posBal=%s posValue=%s lev=%s tradeMode=%s"
                   % (p.get("size"), p.get("side"), p.get("avgPrice"),
                      p.get("markPrice"), p.get("liqPrice"), p.get("positionIM"),
                      p.get("positionMM"), p.get("positionBalance"),
                      p.get("positionValue"), p.get("leverage"), p.get("tradeMode")))
    else:
        out.append("    pos     flat")
    return "\n".join(out)


def execs(start_ms):
    j = call("GET", "/v5/execution/list",
             {"category": "linear", "symbol": SYMBOL, "startTime": str(start_ms),
              "limit": "50"})
    return ((j.get("result") or {}).get("list")) or []


def closed_pnl(start_ms):
    j = call("GET", "/v5/position/closed-pnl",
             {"category": "linear", "symbol": SYMBOL, "startTime": str(start_ms),
              "limit": "50"})
    return ((j.get("result") or {}).get("list")) or []


def order(side, qty, reduce_only=False):
    j = call("POST", "/v5/order/create", {
        "category": "linear", "symbol": SYMBOL, "side": side,
        "orderType": "Market", "qty": qty, "reduceOnly": reduce_only,
    })
    print("  %s %s reduceOnly=%s -> retCode=%s %s" % (
        side, qty, reduce_only, j.get("retCode"), j.get("retMsg")))
    return j


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
        print(fmt_snapshot(snapshot("now")))
        j = call("GET", "/v5/account/info")
        print("  account:", json.dumps({k: v for k, v in
              ((j.get("result") or {}) or {}).items()
              if k in ("marginMode", "unifiedMarginStatus")}))
        t = call("GET", "/v5/market/tickers",
                 {"category": "linear", "symbol": SYMBOL}, signed=False)
        tk = ((t.get("result") or {}).get("list") or [{}])[0]
        print("  mark=%s last=%s fundingRate=%s nextFundingTime=%s"
              % (tk.get("markPrice"), tk.get("lastPrice"),
                 tk.get("fundingRate"), tk.get("nextFundingTime")))
        if tk.get("nextFundingTime"):
            print("  next funding in %.1f min"
                  % ((int(tk["nextFundingTime"]) - time.time() * 1000) / 60000.0))
        ii = call("GET", "/v5/market/instruments-info",
                  {"category": "linear", "symbol": SYMBOL}, signed=False)
        it = ((ii.get("result") or {}).get("list") or [{}])[0]
        print("  lotSizeFilter:", json.dumps(it.get("lotSizeFilter")))
        # 87: the tier table is /v5/market/risk-limit, NOT instruments-info
        print("  risk-limit tiers(first 3):", json.dumps(risklimit()[:3]))

    elif phase == "book":
        # Mirror of probe230.py's `book`: the spread is the deterministic route
        # to a materially adverse close (sell the bid, buy back the ask).
        t = call("GET", "/v5/market/tickers",
                 {"category": "linear", "symbol": SYMBOL}, signed=False)
        tk = ((t.get("result") or {}).get("list") or [{}])[0]
        bid, ask = D(tk["bid1Price"]), D(tk["ask1Price"])
        print("  %s  bid=%s (%s)  ask=%s (%s)  spread=%s (%.4f%%)"
              % (SYMBOL, bid, tk.get("bid1Size"), ask, tk.get("ask1Size"),
                 ask - bid, (ask - bid) / bid * 100))
        print("  mark=%s last=%s" % (tk.get("markPrice"), tk.get("lastPrice")))
        ii = call("GET", "/v5/market/instruments-info",
                  {"category": "linear", "symbol": SYMBOL}, signed=False)
        it = ((ii.get("result") or {}).get("list") or [{}])[0]
        print("  lotSizeFilter:", json.dumps(it.get("lotSizeFilter")))
        print("  risk-limit tier1:", json.dumps(risklimit()[:1]))

    elif phase == "isolate":
        j = call("POST", "/v5/position/switch-isolated",
                 {"category": "linear", "symbol": SYMBOL, "tradeMode": 1,
                  "buyLeverage": sys.argv[2], "sellLeverage": sys.argv[2]})
        print("  switch-isolated -> retCode=%s %s" % (j.get("retCode"), j.get("retMsg")))

    elif phase == "lev":
        j = call("POST", "/v5/position/set-leverage",
                 {"category": "linear", "symbol": SYMBOL,
                  "buyLeverage": sys.argv[2], "sellLeverage": sys.argv[2]})
        print("  set-leverage %s -> retCode=%s %s"
              % (sys.argv[2], j.get("retCode"), j.get("retMsg")))

    elif phase == "open":
        qty = sys.argv[2]
        pre = snapshot("before-open")
        print(fmt_snapshot(pre))
        save("openT0", pre["t"])
        order("Sell", qty)
        time.sleep(3)
        post = snapshot("after-open")
        print(fmt_snapshot(post))
        save("open", post)

    elif phase == "watch":
        thresh = D(sys.argv[2]) if len(sys.argv) > 2 else D("0.0025")
        limit = int(sys.argv[3]) if len(sys.argv) > 3 else 60
        for i in range(limit):
            p = position()
            if not p:
                print("  flat"); return
            e, mk = D(p["avgPrice"]), D(p["markPrice"])
            delta = (mk - e) / e
            adverse = (delta > 0) if p["side"] == "Sell" else (delta < 0)
            print("  %2d  mark=%s avg=%s  delta=%+.5f%%  uPnl=%s  %s"
                  % (i, mk, e, delta * 100, p.get("unrealisedPnl"),
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
        tiers = risklimit()
        save("tiers", tiers)
        print("  tier:", json.dumps(tier_for(pre["position"]["positionValue"], tiers)))
        save("pre_half", pre)
        order("Buy", r, reduce_only=True)
        time.sleep(4)
        post = snapshot("after-half")
        print(fmt_snapshot(post))
        save("post_half", post)
        save("r", r)

    elif phase == "repost":
        add = sys.argv[2]
        pre = snapshot("before-repost")
        print(fmt_snapshot(pre))
        j = call("POST", "/v5/position/add-margin",
                 {"category": "linear", "symbol": SYMBOL, "margin": add})
        print("  add-margin %s -> retCode=%s %s" % (add, j.get("retCode"), j.get("retMsg")))
        time.sleep(3)
        post = snapshot("after-repost")
        print(fmt_snapshot(post))
        save("pre_repost", pre)
        save("post_repost", post)
        liqcheck(post["position"], loadst("tiers") or risklimit(), "after re-post")
        if pre.get("position"):
            print("  (liq moved %s -> %s on an added-margin term of %s)"
                  % (pre["position"]["liqPrice"], post["position"]["liqPrice"], add))

    elif phase == "verify":
        verify(loadst("pre_half"), loadst("post_half"), D(loadst("r")))

    elif phase == "ledger":
        t0 = int(sys.argv[2])
        for e in execs(t0):
            print("  exec  side=%s qty=%s price=%s fee=%s closedSize=%s time=%s"
                  % (e.get("side"), e.get("execQty"), e.get("execPrice"),
                     e.get("execFee"), e.get("closedSize"), e.get("execTime")))
        for c in closed_pnl(t0):
            print("  closedPnl  qty=%s entry=%s exit=%s pnl=%s openFee=%s closeFee=%s"
                  % (c.get("closedSize"), c.get("avgEntryPrice"), c.get("avgExitPrice"),
                     c.get("closedPnl"), c.get("openFee"), c.get("closeFee")))

    elif phase == "close":
        p = position()
        if p:
            order("Buy" if p["side"] == "Sell" else "Sell", p["size"], reduce_only=True)
            time.sleep(3)
        print(fmt_snapshot(snapshot("after-close")))


def liqcheck(p, tiers, tag):
    """The COMMON relation, which section 85 verified on Binance only."""
    if not p:
        return
    t = tier_for(p["positionValue"], tiers)
    mmr = D(str(t["maintenanceMargin"]))
    ded = D(str(t.get("mmDeduction") or "0") or "0")
    m, q, e = D(p["positionIM"]), D(p["size"]), D(p["avgPrice"])
    L = (m / q + e + ded / q) / (1 + mmr)
    print("\n  === liq price, %s ===" % tag)
    print("  tier riskLimitValue=%s maintenanceMargin=%s mmDeduction=%r"
          % (t.get("riskLimitValue"), t.get("maintenanceMargin"), t.get("mmDeduction")))
    print("  predicted L = (m/q + e + ded/q)/(1+MMR) = %s" % L)
    print("  observed  liqPrice                     = %s" % p["liqPrice"])
    print("  error                                  = %s" % (L - D(p["liqPrice"])))
    print("  positionBalance=%s  positionIM=%s  (which one is m?)"
          % (p.get("positionBalance"), p.get("positionIM")))


def verify(pre, post, r):
    pp, qp = pre["position"], post["position"]
    pw = (pre["wallet"] or {}).get("usdt") or {}
    qw = (post["wallet"] or {}).get("usdt") or {}
    pa = (pre["wallet"] or {}).get("account") or {}
    qa = (post["wallet"] or {}).get("account") or {}

    m0, m1 = D(pp["positionIM"]), D(qp["positionIM"]) if qp else D(0)
    q0, q1 = D(pp["size"]), D(qp["size"]) if qp else D(0)
    e = D(pp["avgPrice"])
    W0, W1 = D(pw["walletBalance"]), D(qw["walletBalance"])

    # A is DERIVED here, and that is a finding rather than a convenience: under
    # UNIFIED + ISOLATED_MARGIN this venue serves totalAvailableBalance and
    # availableToWithdraw as EMPTY STRINGS, and the asset-transfer balance family
    # answers demo with retCode 10032. So there is no published A to read.
    #
    # The consequence has to be stated, not buried: with A := W - IM - orderIM,
    # the identity dA = released + realized - fee is TRUE BY CONSTRUCTION
    # (dA = dW - dIM), so confirming it re-derives our own definition and
    # measures nothing. The substantive claims on this venue are the two
    # independent VENUE-PUBLISHED facts underneath it:
    #     (i)  dW      == realized - fee     (wallet moves by the realized terms)
    #     (ii) dIM     == -(1-alpha)*m0      (margin releases pro rata)
    # A's behaviour then follows arithmetically and is reported as derived.
    def avail(c):
        if c.get("availableToWithdraw"):
            return D(c["availableToWithdraw"]), "published"
        return (D(c["walletBalance"]) - D(c.get("totalPositionIM") or "0")
                - D(c.get("totalOrderIM") or "0")), "DERIVED"

    A0, src0 = avail(pw)
    A1, src1 = avail(qw)

    alpha = q1 / q0
    dA, dW, dm = A1 - A0, W1 - W0, m1 - m0

    ex = [x for x in execs(pre["t"] - 2000) if int(x["execTime"]) >= pre["t"] - 2000]
    fee = sum((D(x["execFee"]) for x in ex), D(0))
    P = ((sum((D(x["execPrice"]) * D(x["execQty"]) for x in ex), D(0))
          / sum((D(x["execQty"]) for x in ex), D(0))) if ex else D(0))
    cp = [c for c in closed_pnl(pre["t"] - 2000) if int(c["updatedTime"]) >= pre["t"] - 2000]
    realized = sum((D(c["closedPnl"]) for c in cp), D(0))

    print("\n  === measured ===")
    print("  q  %s -> %s   alpha = %s" % (q0, q1, alpha))
    print("  m  %s -> %s   dm = %s   (released = %s)" % (m0, m1, dm, -dm))
    print("  A  %s -> %s   dA = %s   [%s/%s]" % (A0, A1, dA, src0, src1))
    print("  W  %s -> %s   dW = %s" % (W0, W1, dW))
    print("  e  = %s   P = %s   e-P = %s   r = %s" % (e, P, e - P, r))
    print("  venue-reported: closedPnl = %s   execFee = %s" % (realized, fee))
    print("  (Bybit closedPnl is NET of fees; execFee is listed separately)")

    # The two SUBSTANTIVE, independently-published claims.
    #
    # closedPnl is NOT the realized term of 87 R1a. Bybit reports it NET OF BOTH
    # the open and the close fee, where Binance's per-trade realizedPnl is GROSS
    # with commission listed separately. Feeding closedPnl into the identity
    # double-charges the opening fee -- measured here as an 0.357 discrepancy on
    # a 0.005 close. The realized term is the GROSS r*(e-P); the fee that belongs
    # in the identity is the CLOSING execFee alone.
    gross = r * (e - P)
    print("\n  === the two venue-published claims ===")
    print("  (i)  dW = %s   vs  gross r*(e-P) - closeFee = %s   err = %s"
          % (dW, gross - fee, dW - (gross - fee)))
    print("       closedPnl = %s  =  gross %s - openFee - closeFee  (NET, not the"
          " identity's term)" % (realized, gross))
    print("  (ii) dm = %s   vs  -(1-alpha)*m0 = %s   err = %s"
          % (dm, -(1 - alpha) * m0, dm + (1 - alpha) * m0))

    naive = (1 - alpha) * m0
    model = naive + r * (e - P) - fee
    print("\n  === the two readings (A is %s -- see note) ===" % src1)
    print("  #221's reading    (1-alpha)*m       = %s" % naive)
    print("  87 R1a's reading  + r*(e-P) - fees  = %s" % model)
    print("  observed dA                         = %s" % dA)
    print("  R1a error  = %s" % (model - dA))
    print("  #221 error = %s" % (naive - dA))
    print("  the realized term = %s  (%.3f%% of released margin)"
          % (r * (e - P), (r * (e - P) / (-dm) * 100) if dm else 0))
    if src1 == "DERIVED":
        print("  NOTE: dA above is TRUE BY CONSTRUCTION (dA = dW - dIM). It is not")
        print("        independent evidence; (i) and (ii) are.")

    liqcheck(qp, loadst("tiers") or risklimit(), "post-close")


if __name__ == "__main__":
    main()
