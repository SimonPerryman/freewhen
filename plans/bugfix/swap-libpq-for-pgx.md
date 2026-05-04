# Swap lib/pq for pgx

Status: done

## Goal

Eliminate intermittent 500s on calendar GET / suggestions endpoints caused by
`lib/pq`'s unnamed-prepared-statement cache colliding across connections handed
out by Neon's PgBouncer pooler. Symptom captured in Cloud Run logs:

```
ERROR internal error
error="get calendar: pq: bind message supplies 1 parameters,
       but prepared statement \"\" requires 2"
```

`pgx` (`github.com/jackc/pgx/v5/stdlib`) is the standard Neon-recommended
driver and handles pooled connections without this collision.

## Plan

- Replace `_ "github.com/lib/pq"` with `_ "github.com/jackc/pgx/v5/stdlib"`
  everywhere it's imported.
- Change `sql.Open("postgres", …)` to `sql.Open("pgx", …)`.
- Port `internal/store/pgerror.go` from `*pq.Error` (`Constraint`) to
  `*pgconn.PgError` (`ConstraintName`); same SQLSTATE check.
- Run `make lint`, `make test`, `make test-integration`.

## Log

- 2026-05-04 21:10 — Created plan, swapped driver in `cmd/server/main.go`,
  `internal/testdb/testdb.go`, and three integration test files. Rewrote
  `internal/store/pgerror.go` against `pgconn.PgError`. `make lint`, `make
  test-backend`, and `make test-integration` all pass. Marked done.
