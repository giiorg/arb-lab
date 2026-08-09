#!/usr/bin/env python3
"""Task #225 -- the available-balance read, BOTH venues. PASS 1 IS READ-ONLY.

Two questions, and they are different in kind.

Q1 (the warning on the task, and it BLOCKS the design): record 89 F5 measured
Bybit serving EMPTY STRINGS for totalAvailableBalance / availableToWithdraw
under UNIFIED + ISOLATED_MARGIN. Two causes are consistent with that one
observation and they have OPPOSITE design consequences:

    H-demo     the demo deployment does not compute them -> production
               publishes A, the port has one shape on both venues
    H-posture  ISOLATED_MARGIN does not compute them -> A is DERIVED on Bybit
               forever, and the provenance split is permanent

One observation cannot separate them. Flipping margin mode on the SAME demo
account can: H-posture predicts the fields POPULATE under REGULAR_MARGIN,
H-demo predicts they stay empty. That flip is pass 2 and is NOT in this file.

Q2 (the design half): on Binance, WHICH published field is A? The task names
`availableBalance` on GET /fapi/v3/account. But /fapi/v3/balance publishes a
per-asset `availableBalance` AND a `maxWithdrawAmount`, and the account row
publishes its own `maxWithdrawAmount` and `availableBalance`. Four numbers,
two bases -- withdrawable cash is not postable margin, and section 9's
two-published-fields-on-different-bases rule says deriving one from the other
agrees in the quiet case and fails where it matters. So all four are captured
and their DIFFERENCES are printed, on an account that currently holds whatever
it holds; a run where they all agree is a run that separated nothing, and the
script says so rather than reporting a clean sweep.

Everything is read-only. Nothing here sends an order, changes a mode, or moves
a coin. The account posture is READ so that pass 2 knows what to restore.
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

BYBIT = "api-demo.bybit.com"
BINANCE = "demo-fapi.binance.com"
ENV = {}
HERE = os.path.dirname(os.path.abspath(__file__))
D = Decimal


def load_env(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                ENV[k.strip()] = v.strip().strip('"').strip("'")


# ---------------------------------------------------------------- transports

def bybit(method, path, payload=None, signed=True):
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
        raw = e.read().decode("utf-8", "replace")
        try:
            return json.loads(raw)
        except Exception:  # noqa: BLE001
            return {"_http": e.code, "_raw": raw[:300]}
    except Exception as e:  # noqa: BLE001
        return {"_err": f"{type(e).__name__}: {e}"}


def binance(method, path, params=None, signed=True):
    key, sec = ENV["BINANCE_DEMO_API_KEY"], ENV["BINANCE_DEMO_API_SECRET"]
    params = dict(params or {})
    hdrs = {}
    if signed:
        params["timestamp"] = str(int(time.time() * 1000))
        params["recvWindow"] = "5000"
        q = urllib.parse.urlencode(params)
        q = q + "&signature=" + hmac.new(sec.encode(), q.encode(),
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


# ---------------------------------------------------------------- helpers

def spell(v):
    """How a field is ABSENT is itself a venue fact (section 9, Bybit's `` liqPrice).

    Distinguish, in the report, the four ways a number can fail to arrive --
    they are not interchangeable and a reader who sees `-` for all of them
    cannot tell a missing key from a published empty string.
    """
    if v is None:
        return "KEY-ABSENT"
    if v == "":
        return 'EMPTY-STRING ""'
    return repr(v)


def dec(v):
    try:
        return D(str(v))
    except Exception:  # noqa: BLE001
        return None


def sect(title):
    print()
    print("=" * 78)
    print(title)
    print("=" * 78)


# ---------------------------------------------------------------- bybit pass

BYBIT_ACCOUNT_FIELDS = (
    "totalEquity", "totalWalletBalance", "totalMarginBalance",
    "totalAvailableBalance", "totalPerpUPL", "totalInitialMargin",
    "totalMaintenanceMargin", "accountIMRate", "accountMMRate",
    "accountLTV", "accountType",
)
BYBIT_COIN_FIELDS = (
    "coin", "equity", "walletBalance", "availableToWithdraw", "availableToBorrow",
    "totalPositionIM", "totalOrderIM", "totalPositionMM", "unrealisedPnl",
    "cumRealisedPnl", "locked", "borrowAmount", "accruedInterest",
    "marginCollateral", "collateralSwitch", "spotHedgingQty",
)


def bybit_wallet(account_type):
    return bybit("GET", "/v5/account/wallet-balance", {"accountType": account_type})


def bybit_report(tag, raw):
    """Print the availability surface for one wallet-balance response."""
    lst = ((raw.get("result") or {}).get("list")) or []
    print(f"\n-- {tag}: retCode={raw.get('retCode')} retMsg={raw.get('retMsg')!r} "
          f"rows={len(lst)}")
    if not lst:
        return None
    acct = lst[0]
    print("   account-level:")
    for f in BYBIT_ACCOUNT_FIELDS:
        print(f"     {f:26s} {spell(acct.get(f))}")
    coin = next((c for c in acct.get("coin", []) if c.get("coin") == "USDT"), None)
    if coin is None:
        print("   USDT coin row: ABSENT")
        return acct
    print("   USDT coin row:")
    for f in BYBIT_COIN_FIELDS:
        print(f"     {f:26s} {spell(coin.get(f))}")
    unknown = sorted(set(coin) - set(BYBIT_COIN_FIELDS))
    if unknown:
        print(f"   coin fields NOT in our list: {unknown}")
    return acct


def run_bybit():
    sect("BYBIT demo -- posture, then every surface that could carry A")

    info = bybit("GET", "/v5/account/info")
    res = info.get("result") or {}
    print(f"-- /v5/account/info retCode={info.get('retCode')}")
    for f in ("unifiedMarginStatus", "marginMode", "isMasterTrader",
              "spotHedgingStatus", "updatedTime", "dcpStatus"):
        print(f"     {f:26s} {spell(res.get(f))}")

    raw_unified = bybit_wallet("UNIFIED")
    acct = bybit_report("wallet-balance accountType=UNIFIED", raw_unified)
    raw_contract = bybit_wallet("CONTRACT")
    bybit_report("wallet-balance accountType=CONTRACT", raw_contract)

    print("\n-- the asset family (record 89 F5 measured retCode 10032 here)")
    coins = bybit("GET", "/v5/asset/transfer/query-account-coins-balance",
                  {"accountType": "UNIFIED", "coin": "USDT"})
    print(f"     query-account-coins-balance  retCode={coins.get('retCode')} "
          f"retMsg={coins.get('retMsg')!r}")
    wd = bybit("GET", "/v5/asset/withdraw/withdrawable-amount", {"coin": "USDT"})
    print(f"     withdraw/withdrawable-amount retCode={wd.get('retCode')} "
          f"retMsg={wd.get('retMsg')!r}")
    tfr = bybit("GET", "/v5/asset/transfer/query-transfer-coin-list",
                {"fromAccountType": "UNIFIED", "toAccountType": "FUND"})
    print(f"     transfer/query-transfer-coin-list retCode={tfr.get('retCode')} "
          f"retMsg={tfr.get('retMsg')!r}")

    print("\n-- preconditions for pass 2's margin-mode flip (needs flat + no orders)")
    pos = bybit("GET", "/v5/position/list",
                {"category": "linear", "settleCoin": "USDT"})
    plist = ((pos.get("result") or {}).get("list")) or []
    live = [p for p in plist if (dec(p.get("size")) or D(0)) != 0]
    print(f"     positions rows={len(plist)} NON-FLAT={len(live)}")
    for p in live:
        print(f"       {p.get('symbol')} size={p.get('size')} "
              f"side={p.get('side')} positionIM={spell(p.get('positionIM'))} "
              f"tradeMode={p.get('tradeMode')}")
    orders = bybit("GET", "/v5/order/realtime",
                   {"category": "linear", "settleCoin": "USDT"})
    olist = ((orders.get("result") or {}).get("list")) or []
    print(f"     open orders = {len(olist)}")

    # The derivation the task names: A = walletBalance - totalPositionIM - totalOrderIM.
    if acct:
        coin = next((c for c in acct.get("coin", []) if c.get("coin") == "USDT"), None)
        if coin:
            w = dec(coin.get("walletBalance"))
            pim = dec(coin.get("totalPositionIM"))
            oim = dec(coin.get("totalOrderIM"))
            print("\n-- the DERIVED A (task #225's formula), computed here so pass 2"
                  "\n   can check whether a published A agrees with it:")
            if None in (w, pim, oim):
                print(f"     NOT COMPUTABLE: walletBalance={spell(coin.get('walletBalance'))} "
                      f"totalPositionIM={spell(coin.get('totalPositionIM'))} "
                      f"totalOrderIM={spell(coin.get('totalOrderIM'))}")
            else:
                print(f"     {w} - {pim} - {oim} = {w - pim - oim}")
                if pim == 0 and oim == 0:
                    print("     *** COLLAPSE WARNING: both margin terms are ZERO, so the"
                          "\n         derivation equals walletBalance and this run cannot"
                          "\n         distinguish `A = W - IM` from `A = W`. Take the"
                          "\n         comparison with a position OPEN.")
    return {"info": info, "unified": raw_unified, "contract": raw_contract,
            "coins_balance": coins, "withdrawable": wd, "transfer_coins": tfr,
            "positions": pos, "orders": orders}


# ---------------------------------------------------------------- binance pass

def run_binance():
    sect("BINANCE demo -- FOUR candidate A's, and whether this run separates them")

    st_a, acct = binance("GET", "/fapi/v3/account")
    print(f"-- /fapi/v3/account http={st_a}")
    top = {}
    if isinstance(acct, dict):
        for f in ("totalWalletBalance", "totalMarginBalance", "availableBalance",
                  "maxWithdrawAmount", "totalInitialMargin", "totalMaintMargin",
                  "totalUnrealizedProfit", "totalPositionInitialMargin",
                  "totalOpenOrderInitialMargin", "multiAssetsMargin"):
            top[f] = acct.get(f)
            print(f"     {f:30s} {spell(acct.get(f))}")
    usdt_asset = None
    if isinstance(acct, dict):
        usdt_asset = next((a for a in (acct.get("assets") or [])
                           if a.get("asset") == "USDT"), None)
    if usdt_asset:
        print("   assets[] USDT row:")
        for f in ("walletBalance", "marginBalance", "availableBalance",
                  "maxWithdrawAmount", "initialMargin", "maintMargin",
                  "unrealizedProfit", "positionInitialMargin",
                  "openOrderInitialMargin", "crossWalletBalance"):
            print(f"     {f:30s} {spell(usdt_asset.get(f))}")

    st_b, bal = binance("GET", "/fapi/v3/balance")
    print(f"\n-- /fapi/v3/balance http={st_b}")
    usdt_bal = None
    if isinstance(bal, list):
        usdt_bal = next((b for b in bal if b.get("asset") == "USDT"), None)
    if usdt_bal:
        for f in ("balance", "crossWalletBalance", "crossUnPnl", "availableBalance",
                  "maxWithdrawAmount", "marginAvailable"):
            print(f"     {f:30s} {spell(usdt_bal.get(f))}")

    st_p, pos = binance("GET", "/fapi/v3/positionRisk")
    live = [p for p in pos if (dec(p.get("positionAmt")) or D(0)) != 0] \
        if isinstance(pos, list) else []
    print(f"\n-- /fapi/v3/positionRisk http={st_p} NON-FLAT={len(live)}")
    for p in live:
        print(f"     {p.get('symbol')} amt={p.get('positionAmt')} "
              f"isolated={p.get('isolated')} isolatedWallet={p.get('isolatedWallet')} "
              f"entry={p.get('entryPrice')}")

    # Do the four candidates actually DISAGREE in this run? If they do not, the
    # run separated nothing, and saying so is the point of printing it.
    sect("BINANCE -- do the four candidate A's separate?")
    cands = {
        "account.availableBalance": top.get("availableBalance"),
        "account.maxWithdrawAmount": top.get("maxWithdrawAmount"),
        "assets[USDT].availableBalance": (usdt_asset or {}).get("availableBalance"),
        "assets[USDT].maxWithdrawAmount": (usdt_asset or {}).get("maxWithdrawAmount"),
        "balance[USDT].availableBalance": (usdt_bal or {}).get("availableBalance"),
        "balance[USDT].maxWithdrawAmount": (usdt_bal or {}).get("maxWithdrawAmount"),
    }
    vals = {}
    for k, v in cands.items():
        d = dec(v)
        vals[k] = d
        print(f"     {k:34s} {spell(v)}")
    distinct = sorted({str(v) for v in vals.values() if v is not None})
    print(f"\n     DISTINCT VALUES: {len(distinct)} -> {distinct}")
    if len(distinct) <= 1:
        print("     *** COLLAPSE WARNING: every candidate is the same number, so this")
        print("         run CANNOT tell postable margin from withdrawable cash. That is")
        print("         expected on a FLAT account with no unrealized PnL and no open")
        print("         orders -- the separating state is a position OPEN with UPL.")
    return {"account": acct, "balance": bal, "positionRisk": pos}


def main():
    load_env(os.environ.get("ARBLAB_ENV",
                            os.path.join(HERE, "..", "..", "..", ".env")))
    out = {}
    try:
        out["bybit"] = run_bybit()
    except Exception as e:  # noqa: BLE001
        print(f"BYBIT PASS FAILED: {type(e).__name__}: {e}")
    try:
        out["binance"] = run_binance()
    except Exception as e:  # noqa: BLE001
        print(f"BINANCE PASS FAILED: {type(e).__name__}: {e}")
    dest = os.path.join(HERE, "captures", "probe225-pass1-readonly.json")
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    with open(dest, "w") as fh:
        json.dump(out, fh, indent=1, sort_keys=True)
    print(f"\nraw responses -> {dest}")


if __name__ == "__main__":
    sys.exit(main())
