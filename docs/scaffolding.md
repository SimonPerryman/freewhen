# Project Scaffolding

What we're setting up before any product work begins.

## Folder Structure

```
freewhen/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # Entrypoint
│   ├── internal/
│   │   ├── handler/                  # HTTP handlers (one file per resource)
│   │   ├── model/                    # Structs (Calendar, Participant, Availability)
│   │   ├── store/                    # Postgres queries
│   │   └── server/                   # Server setup, routing, middleware
│   ├── migrations/                   # SQL migration files
│   ├── go.mod
│   └── go.sum
│
├── frontend/
│   ├── src/
│   │   ├── app/                      # Next.js App Router pages
│   │   ├── components/               # React components
│   │   └── lib/                      # API client, utils
│   ├── public/
│   ├── package.json
│   ├── tsconfig.json
│   ├── next.config.ts
│   └── oxlintrc.json
│
├── docs/                             # Project documentation
│
├── .env.example                      # DATABASE_URL placeholder
├── .gitignore
├── Makefile                          # Dev commands: `make dev`, `make lint`, etc.
├── README.md
└── AGENTS.md
```

## Tech & Tooling

| Concern         | Choice                     | Notes                                        |
|-----------------|----------------------------|----------------------------------------------|
| Backend         | Go (standard library)      | Single binary, `net/http`                    |
| Frontend        | Next.js (App Router, TS)   | React with TypeScript                        |
| Database        | Postgres (hosted)          | Supabase/Neon/Railway - no local Docker      |
| Go linting      | golangci-lint              |                                              |
| JS/TS linting   | Oxlint                     | Faster ESLint alternative                    |
| JS/TS formatting| oxfmt                      | Faster Prettier alternative                  |
| Dev workflow    | Makefile                   | Unified commands across both stacks          |

## Testing

| Stack    | Tool                     | Notes                                                        |
|----------|--------------------------|--------------------------------------------------------------|
| Backend  | `go test` (std library)  | Unit tests alongside code, integration tests via build tags  |
| Frontend | Vitest                   | Fast, native ESM/TypeScript support                          |

### Backend Testing

- **Unit tests** - `_test.go` files next to the code they test (same package). Run with `go test ./...`.
- **Integration tests** - `_test.go` files with a `//go:build integration` build tag. Skipped by default, run explicitly with `go test -tags=integration ./...`. These hit a real database.

### Frontend Testing

- Vitest as the test runner
- Test files in `__tests__/` directories or as `*.test.ts(x)` files alongside components
- `make test-frontend` to run

### Makefile Targets

- `make test` - run all unit tests (both stacks)
- `make test-backend` - Go unit tests only
- `make test-frontend` - Vitest only
- `make test-integration` - Go integration tests (requires `DATABASE_URL`)

## Decisions

- **No Docker Compose.** Database is hosted (free tier). No need for local Postgres.
- **No `apps/services/` nesting.** `backend/` and `frontend/` are clear enough for two pieces. Rename if mobile ever arrives.
- **One README at the root.** Add per-directory READMEs only when needed.
- **Migrations live in `backend/`.** They're Go-managed and run against the hosted DB.
