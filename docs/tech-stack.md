# Auspex — tech-stack.md

> Phase 3: Tech Stack
> Date: 21.02.2026
> Status: Current

---

## Selected Technologies

### Backend

| Component | Solution | Version |
|-----------|----------|---------|
| Language | Go | 1.22+ |
| HTTP routing | Chi | v5 |
| Database | SQLite | — |
| SQLite driver | modernc.org/sqlite | latest |
| SQL code generation | sqlc | v2 |
| OAuth2 | golang.org/x/oauth2 | latest |
| HTTP client (ESI) | standard net/http | — |
| Static file embedding | standard embed | — |
| Testing | standard testing | — |
| Mocks | testify/mock | latest |

### Frontend

| Component | Solution | Version |
|-----------|----------|---------|
| Framework | React | 18+ |
| Build tool | Vite | latest |
| Tables | TanStack Table | v8 |
| HTTP client | fetch API (standard) | — |
| Styles | CSS (no framework) | — |

### Infrastructure

| Component | Solution |
|-----------|----------|
| Database | SQLite, single file next to the binary |
| Distribution | Single Go binary, static files embedded via embed |
| Configuration | Launch flags + config file (port, refresh interval) |

---

## Rationale

### Go

Driven by requirements: a single binary with no external dependencies, cross-platform compilation (Windows / macOS / Linux), embedding static files via `embed`, future compatibility with Wails. Go compiles to a single executable that requires no runtime on the user's machine.

### Chi

A lightweight HTTP router fully compatible with the standard `net/http` — no custom context, no vendor lock-in. Solves a specific problem: Auspex has 10+ endpoints and several middleware (logging, recover, CORS). Chi provides a middleware stack with route grouping without the boilerplate of the standard library. Built-in middleware (`Logger`, `Recoverer`) is ready to use out of the box. Compatibility with the standard `http.Handler` is important for a future migration to Wails.

### SQLite

Driven by requirements: a single-user local application, a single file, no external dependencies. SQLite performance is more than sufficient — ESI data updates every 10+ minutes and the record count is in the hundreds, not millions.

### modernc.org/sqlite (driver)

A pure Go implementation of SQLite — compiles without CGO. This is critical for cross-platform single-binary builds: CGO-dependent drivers require a C toolchain on the build machine for each target platform. modernc.org/sqlite eliminates this problem entirely.

### sqlc

A code generator: takes a SQL schema and SQL queries, generates typed Go code. Solves the schema/code desync problem — when the schema changes, `sqlc generate` fails if queries are not updated. No manual `Scan`, the compiler checks types. SQL remains clean SQL with no ORM magic. Important for future modules (manufacturing, analytics) where queries will be more complex.

### golang.org/x/oauth2

The official extended Go library for OAuth2. Handles the Authorization Code flow, automatic token refresh on expiry, and token storage. EVE SSO uses standard OAuth2 — the library fits without adaptation. The alternative (manual implementation) is possible but x/oauth2 already handles edge cases and is battle-tested in production.

### testify/mock

Go's standard `testing` package handles test execution and assertions. `testify/mock` adds interface-based mocking — necessary for testing `sync` and `api` in isolation without real ESI or SQLite. The alternative (manual mock structs) works but requires significant boilerplate for every interface. testify/mock generates this automatically and integrates cleanly with the standard `testing` package.

### React + Vite + TanStack Table

A BPO table with sorting, filters, highlighting, and periodic data refresh is exactly the class of problem TanStack Table was built for. Vanilla JS would require manual implementation of most of TanStack Table's functionality. React provides a component model (BPO table, characters section, summary row are natural components) and state management. Vite removes build complexity: `npm run build` produces static files that embed into the Go binary via `embed` just as easily as vanilla files.

---

## Considered Alternatives and Reasons for Rejection

| Component | Alternative | Reason for rejection |
|-----------|-------------|----------------------|
| Chi | `net/http` (stdlib) | No middleware stack; boilerplate when grouping routes with different middleware |
| Chi | Gin | Custom `gin.Context` instead of standard — friction during integration and future Wails migration |
| Chi | Echo | Same issues as Gin — custom context, vendor lock-in |
| sqlc | `database/sql` (stdlib) | Manual `Scan` for every query; schema and code easily fall out of sync; significant boilerplate |
| sqlc | GORM | Hides SQL behind ORM magic; hard to know what query actually runs; overkill for Auspex's predictable schema |
| modernc.org/sqlite | mattn/go-sqlite3 | Requires CGO — complicates cross-platform builds |
| React | Vue | Smaller ecosystem; TanStack Table is React-oriented; no meaningful advantage for this task |
| React | Vanilla JS | An interactive table with filters, state, and polling is exactly the class of task where vanilla JS turns into spaghetti |

---

## Known Stack Risks

**ESI API as an external dependency** — the entire project depends on ESI stability. CCP may change endpoints or restrict scopes. Mitigation: isolate the ESI client in a dedicated package so changes don't spread through the entire codebase.

**sqlc and schema** — when the DB schema changes, `sqlc generate` must be run and queries updated. This is an easy step to forget. Mitigation: add `sqlc generate` to the Makefile as a mandatory step before building.

**modernc.org/sqlite performance** — the pure Go implementation is slower than the CGO variant. For Auspex this is irrelevant (hundreds of records, updates every 10+ minutes), but worth keeping in mind if data volume were to grow significantly.

**React bundle size** — the frontend is embedded in the binary. React + TanStack Table will add ~200–300 KB gzip to the binary size. For a desktop application this is acceptable.

---

## References and Versions

- Go: https://go.dev/doc/ (1.22+)
- Chi: https://github.com/go-chi/chi (v5)
- sqlc: https://docs.sqlc.dev (v2)
- modernc.org/sqlite: https://pkg.go.dev/modernc.org/sqlite
- golang.org/x/oauth2: https://pkg.go.dev/golang.org/x/oauth2
- testify/mock: https://github.com/stretchr/testify
- EVE ESI: https://esi.evetech.net/ui/
- EVE SSO OAuth2: https://developers.eveonline.com/blog/article/sso-to-authenticated-calls
- React: https://react.dev (v18+)
- Vite: https://vitejs.dev
- TanStack Table: https://tanstack.com/table/v8
