# migrations

Schema for the embedded durable store: the OMS write-ahead journal, the
transactional outbox, and the audit projection (**D2**, §11).

## Why this directory is partitioned by dialect

SQLite is the MVP store and Postgres is the documented upgrade path (**D2**:
"Postgres-ready"). Splitting by dialect from the first migration costs nothing
now and avoids a rename later, so every migration lives under
`migrations/<dialect>/`.

The migration files are the *only* place the two dialects are expected to
diverge. Three things keep that true, and they are architectural rather than
disciplinary:

- **`fixedpoint` stores an `int64` mantissa** at a fixed canonical scale
  (`Scale = 9`), which is SQLite `INTEGER` and Postgres `BIGINT` — both 8-byte
  signed, exact in both directions, numerically sortable in both. A 128-bit
  mantissa would have needed `TEXT` or paired columns on SQLite against
  `NUMERIC` on Postgres, and that divergence leaks into queries.
- **The OMS is a single writer** (§3.6 rule 6, lint-enforced), so correctness
  never rests on database-level concurrency control. There is no
  `SELECT … FOR UPDATE` to port and no isolation-level reasoning to redo under
  MVCC.
- **All SQL lives in `infrastructure/driven/`** behind ports (§3.6 rules 1–2),
  so a dialect swap is one adapter package plus these files.

## What does NOT port, and is expected to be rewritten

- **Durability and backup.** `journal_mode`/`synchronous=FULL` pragma
  verification at boot, the online-backup API / `VACUUM INTO`, and RPO ≤ 5 min
  file shipping (§11) are SQLite-shaped. Postgres replaces all of it with WAL
  archiving and PITR. This is an ops change, not a code change.
- **The outbox reader.** Postgres has `LISTEN/NOTIFY` and
  `FOR UPDATE SKIP LOCKED`; SQLite has neither. Written as a single-reader
  polling loop it runs unchanged on both, and Postgres becomes an optimisation
  rather than a prerequisite.
- **Upsert and DDL phrasing.** `INSERT OR IGNORE` vs `ON CONFLICT DO NOTHING`,
  `AUTOINCREMENT` vs `IDENTITY`.
