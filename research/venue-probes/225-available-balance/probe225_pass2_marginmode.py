#!/usr/bin/env python3
"""Task #225 pass 2 -- THE DISCRIMINATING EXPERIMENT for the task's warning.

Record 89 F5 observed ONE state: Bybit demo, UNIFIED + ISOLATED_MARGIN, empty
availability strings. Two causes fit that observation and they have opposite
consequences for the port's design:

    H-demo     the DEMO deployment does not compute availability
    H-posture  ISOLATED_MARGIN does not compute it (any deployment)

They differ on a state nobody has visited: the SAME demo account under a
DIFFERENT margin mode.

    H-demo    predicts the fields stay EMPTY under REGULAR_MARGIN
    H-posture predicts they POPULATE under REGULAR_MARGIN

So the experiment is a flip, a read, and a flip back. THE FLIP BACK IS NOT
CLEANUP -- it is the positive control. If the fields populate under REGULAR and
go empty again under ISOLATED, the MODE moved them. If they stayed populated
after reverting, something else did (a lazy first computation, an account
touch, the passage of time) and the reading would have been wrong in the
direction that looks like a clean result.

PRECONDITION, checked and refused rather than assumed: Bybit rejects a margin
mode change while any position or order is live. Pass 1 read the account flat.
This re-reads it, because the state that mattered was read minutes ago.

RESTORATION runs in `finally`. The account posture is what build-spec 4.1 pins,
so leaving it flipped would silently invalidate every later probe in this tree.
The script verifies the restore and shouts if it fails.
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

try:
    import certifi
    _CTX = ssl.create_default_context(cafile=certifi.where())
except ImportError:
    _CTX = ssl.create_default_context(cafile="/etc/ssl/cert.pem")

HOST = "api-demo.bybit.com"
HERE = os.path.dirname(os.path.abspath(__file__))
ENV = {}

BASELINE = "ISOLATED_MARGIN"     # what 4.1 pins, and what we must restore
PROBED = "REGULAR_MARGIN"        # the other mode a UTA account can hold flat

# The fields whose emptiness is the whole question.
WATCH_ACCOUNT = ("totalMarginBalance", "totalAvailableBalance", "totalInitialMargin",
                 "totalMaintenanceMargin", "accountIMRate", "accountMMRate",
                 "accountLTV")
WATCH_COIN = ("availableToWithdraw", "availableToBorrow", "walletBalance",
              "totalPositionIM", "totalOrderIM")


def load_env(path):
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                ENV[k.strip()] = v.strip().strip('"').strip("'")


def call(method, path, payload=None):
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
    hdrs.update({
        "X-BAPI-API-KEY": key, "X-BAPI-TIMESTAMP": ts, "X-BAPI-RECV-WINDOW": recv,
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


def spell(v):
    if v is None:
        return "KEY-ABSENT"
    if v == "":
        return 'EMPTY ""'
    return str(v)


def posture():
    r = (call("GET", "/v5/account/info").get("result")) or {}
    return r.get("marginMode"), r.get("unifiedMarginStatus")


def snapshot(label):
    """Read the availability surface and report which watched fields are populated."""
    raw = call("GET", "/v5/account/wallet-balance", {"accountType": "UNIFIED"})
    lst = ((raw.get("result") or {}).get("list")) or []
    acct = lst[0] if lst else {}
    coin = next((c for c in acct.get("coin", []) if c.get("coin") == "USDT"), {})
    mode, status = posture()
    print(f"\n--- {label}  (marginMode={mode} unifiedMarginStatus={status} "
          f"retCode={raw.get('retCode')})")
    populated = []
    for f in WATCH_ACCOUNT:
        v = acct.get(f)
        print(f"      account.{f:24s} {spell(v)}")
        if v not in (None, ""):
            populated.append(f"account.{f}")
    for f in WATCH_COIN:
        v = coin.get(f)
        print(f"      USDT.{f:27s} {spell(v)}")
        if v not in (None, ""):
            populated.append(f"USDT.{f}")
    return {"mode": mode, "status": status, "raw": raw, "populated": populated}


def set_mode(mode):
    r = call("POST", "/v5/account/set-margin-mode", {"setMarginMode": mode})
    print(f"      set-margin-mode -> {mode}: retCode={r.get('retCode')} "
          f"retMsg={r.get('retMsg')!r}")
    return r


def flat():
    """Refuse the experiment unless the account is genuinely flat, re-read NOW."""
    pos = call("GET", "/v5/position/list", {"category": "linear", "settleCoin": "USDT"})
    plist = ((pos.get("result") or {}).get("list")) or []
    live = [p for p in plist if (p.get("size") or "0") not in ("0", "", "0.0")]
    od = call("GET", "/v5/order/realtime", {"category": "linear", "settleCoin": "USDT"})
    olist = ((od.get("result") or {}).get("list")) or []
    print(f"   precondition: non-flat positions={len(live)} open orders={len(olist)}")
    return not live and not olist


def main():
    load_env(os.environ.get("ARBLAB_ENV",
                            os.path.join(HERE, "..", "..", "..", ".env")))
    out = {}
    print("=" * 78)
    print("BYBIT demo -- does MARGIN MODE or the DEMO deployment empty the "
          "availability fields?")
    print("=" * 78)

    mode, _ = posture()
    if mode != BASELINE:
        print(f"REFUSING: account is in {mode!r}, expected the pinned {BASELINE!r}. "
              "Restore it by hand before running this.")
        return 1
    if not flat():
        print("REFUSING: account is not flat, so the venue will reject the mode "
              "change and the run would prove nothing.")
        return 1

    out["A_baseline_isolated"] = snapshot("A. BASELINE -- ISOLATED_MARGIN")

    restored = False
    try:
        print(f"\n   flipping {BASELINE} -> {PROBED}")
        out["flip_to_probed"] = set_mode(PROBED)
        if out["flip_to_probed"].get("retCode") != 0:
            print("   FLIP REFUSED by the venue -- the experiment did not run. "
                  "Nothing is concluded.")
        else:
            time.sleep(2)   # the mode change is acknowledged before it is reflected
            out["B_probed_regular"] = snapshot("B. PROBED -- REGULAR_MARGIN")
    finally:
        print(f"\n   restoring {PROBED} -> {BASELINE}")
        out["restore"] = set_mode(BASELINE)
        time.sleep(2)
        back, _ = posture()
        restored = back == BASELINE
        print(f"      posture now {back!r} -- {'RESTORED' if restored else '*** NOT RESTORED ***'}")
        if restored:
            out["C_restored_isolated"] = snapshot("C. CONTROL -- ISOLATED_MARGIN again")

    # ------------------------------------------------------------------ verdict
    print("\n" + "=" * 78)
    print("VERDICT")
    print("=" * 78)
    a = out.get("A_baseline_isolated", {}).get("populated")
    b = out.get("B_probed_regular", {}).get("populated")
    c = out.get("C_restored_isolated", {}).get("populated")
    if b is None:
        print("  INCONCLUSIVE -- the flip never happened.")
    else:
        gained = sorted(set(b) - set(a or []))
        print(f"  populated under ISOLATED_MARGIN : {len(a or [])} {a}")
        print(f"  populated under REGULAR_MARGIN  : {len(b)} {b}")
        print(f"  populated after restore         : {len(c or [])} {c}")
        print(f"\n  GAINED by the flip: {gained or 'NOTHING'}")
        if gained and c is not None and set(c) == set(a or []):
            print("\n  => H-posture. The fields are a function of MARGIN MODE, not of the")
            print("     demo deployment: they appeared under REGULAR_MARGIN and went away")
            print("     again when the mode was restored. A is DERIVED on Bybit under")
            print("     4.1's pinned posture, in production as well as on demo.")
        elif gained and c is not None and set(c) != set(a or []):
            print("\n  => AMBIGUOUS. Fields appeared under REGULAR_MARGIN and did NOT go")
            print("     away on restore, so the mode is not established as the cause --")
            print("     a lazy first computation fits this trace equally well.")
        elif not gained:
            print("\n  => H-demo SURVIVES, and note what it does NOT say: 'empty under both")
            print("     modes on demo' is consistent with the demo deployment never")
            print("     computing availability AND with some third account property. It")
            print("     REFUTES H-posture-alone; it does not establish H-demo. Production")
            print("     cannot be probed from here.")
    if not restored:
        print("\n  *** THE ACCOUNT WAS LEFT IN THE PROBED MODE. Restore it before any")
        print("      other probe runs -- 4.1's posture is a premise of all of them.")

    dest = os.path.join(HERE, "captures", "probe225-pass2-marginmode.json")
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    with open(dest, "w") as fh:
        json.dump(out, fh, indent=1, sort_keys=True)
    print(f"\nraw responses -> {dest}")
    return 0 if restored else 2


if __name__ == "__main__":
    sys.exit(main())
