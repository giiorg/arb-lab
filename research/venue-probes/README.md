# Venue probes

One directory per task that took a **credentialed measurement against a live
venue**: the probe scripts that were run, and the verbatim responses they
captured. Sessions 133 (#230) and 134 (#233) are the first entries; before them
no credentialed session preserved its probes, and #233 is what showed the cost —
its oracle could only be made exhaustive because #230's raw bracket table was
still on disk. A two-row hand-check was the alternative, and a two-row check is
what §90 §6 caught passing vacuously.

These are **not** the §42b screening store or the §45a settled-funding store,
both of which are gitignored. Those are re-derivable: the venue will answer the
same REST call again, and each file's sha256 is pinned in a committed ledger. A
probe capture is **not re-derivable** — it is a snapshot of one account holding
one position at one moment, and the position has since been closed. Re-running
`probe230.py` today produces a different measurement, not the same one.

## Layout

```
<task-id>-<slug>/
  probe*.py            the scripts as run
  captures/*.json      verbatim responses, redacted per the policy below
```

## Redaction policy

Credentials are read from the environment (`os.environ[...]`), never written to
disk and never present in this tree. `.env` is gitignored. Nothing here needs
redacting on that axis; the policy below is about **account identity**, which is
a different question.

**Removed** — replaced with the string `"<redacted>"`, which is deliberately not
a plausible value of the field, so a reader cannot mistake it for data:

| Field | Why |
|---|---|
| `accountAlias` | Names the sub-account. |
| `orderId`, `clientOrderId` | Venue-side identifiers for real orders. |

Note the type change: `orderId` is a JSON **number** in the raw response and a
string here. Nothing parses it — see the reproduction check below — and an
obviously-wrong type is a louder marker than a legal-looking `0` would be.

**Kept, deliberately** — every monetary and position field: `walletBalance`,
`availableBalance`, `maxWithdrawAmount`, `totalEquity`, `positionAmt`,
`entryPrice`, `liquidationPrice`, the `brackets`/`riskLimit` tables, and the
timestamps.

These are **the measurement**. §89's four rows and §90's fixed-point derivation
consume them as inputs; §90's uniqueness sweep runs over the captured bracket
table itself. Scrubbing them would leave an archive that cannot reproduce the
finding it exists to support — which turns recorded evidence back into an
asserted claim, the exact substitution this directory was created to avoid. The
balances are a few thousand USDT on a dedicated account, and that is the trade
that was accepted.

## The check that keeps the policy honest

A redaction pass is a mutation of the evidence, so it gets the same treatment as
any other mutation: re-run the consumer and confirm it still reaches the same
number.

```
python3 research/venue-probes/233-bracket-fixed-point/fixedpoint_oracle.py
```

Reproduces §89 F3's four rows to `5.59E-10` / `3.23E-9` off the redacted capture
and prints `ALL CHECKS PASS`. If a future redaction breaks a field the oracle
reads, this is what says so — a scrub that silently removed an input would
otherwise look identical to one that removed metadata.
