# Auspex — requirements.md

> Phase 2: Requirements
> Date: 21.02.2026
> Status: Current

---

## MVP Scope

MVP = **BPO library + research slot monitoring** for all added characters and corporations.

Everything else is a subsequent module after MVP.

---

## Functional Requirements

### Priorities: must / should / could

**Must (MVP)**

- EVE Online character authorization via ESI OAuth2 (scope: read blueprints and industry jobs)
- Corporation support via a delegate character with the required corp roles (scope: corp blueprints, corp industry jobs)
- Ability to add multiple characters and multiple corporations
- Display of the BPO library: all BPOs from all added characters and corporations in a single table
- For each BPO: Name, Category, ME%, TE%, current status (Idle / ME Research / TE Research / Copying), assigned character/corp, location, end date of the current job
- Summary row above the table: count of Idle BPOs / overdue jobs / completing today / free research slots
- Characters section: Used Slots / Total Slots / Available Slots for each character
- Visual highlighting: overdue jobs (red), completing today (yellow), idle BPOs (separate label), free slots (highlight in characters section)
- BPO table sorting by status and end date; overdue and idle float to the top by default
- BPO table filtering by status, character/corporation, category
- Automatic data refresh from ESI every N minutes (N is configurable, default 10)
- Manual force-refresh button
- Backend respects ESI cache headers — does not request data before cache expiry

**Should (post-MVP)**

- Manufacturing slot monitoring
- BPC library
- Profitability analytics: what's worth producing given current market prices
- Mineral tracking and shortage reporting

**Could (future ideas)**

- Wails wrapper for a native desktop application
- External alerts: Discord, Web Push, email
- Historical data and performance graphs

---

## User Stories

**Adding characters and corporations**

- As a user, I want to authorize a character via EVE SSO so the application gains access to their data
- As a user, I want to add a corporation via a director character so I can see corp BPOs and corp jobs
- As a user, I want to manage the list of tracked characters and corporations (add / remove)

**Monitoring**

- As a user, I want to see all BPOs in one table so I don't have to switch between characters
- As a user, I want to see BPOs that are sitting idle so I can immediately queue them for work
- As a user, I want to see jobs that have already completed so I know a slot is effectively free
- As a user, I want to see which jobs complete today and tomorrow so I can plan the next batch of work
- As a user, I want to see a summary at the top so I can assess the state of the entire operation at a glance without scrolling

**Data updates**

- As a user, I want data to update automatically while the application is open
- As a user, I want to force a data refresh with a button when I need the current state right now
- As a user, I want to configure the auto-refresh interval to my preference

---

## Non-Functional Requirements

**Architecture (hard constraints)**

- The backend is implemented as a REST API returning JSON — no server-side rendering, no Go templates
- The frontend is entirely static files (HTML/CSS/JS) that communicates with the backend only via HTTP API
- This enables future replacement of the UI (Wails, native UI) without touching business logic

**Distribution**

- A single executable binary (Go); frontend static files embedded in the binary via `embed`
- Database: SQLite, a single file next to the binary
- No external dependencies — no PostgreSQL, no Redis, no Docker

**Startup**

- User runs the binary, opens `localhost:PORT` in their browser
- Port is configurable (default: 8080)

**Data and ESI**

- Data updates with ESI cache delay (5–30 minutes depending on the endpoint) — the user sees this explicitly (last-updated timestamp)
- The backend does not ignore ESI cache headers to avoid rate limiting

**Security**

- OAuth2 tokens are stored locally in SQLite and are never sent anywhere except to ESI
- The application is single-user — no multi-user authentication, no public access

**Performance**

- The UI responds instantly (data from local SQLite); ESI requests run in the background

**Testability**

- Key packages (`esi`, `sync`, `api`) are designed with dependency injection — dependencies are passed as interfaces, not hardcoded
- This enables unit testing without a real ESI connection or SQLite database

---

## Constraints

- Platform: desktop (Windows / macOS / Linux), browser as UI
- ESI API: the entire project depends on the availability and stability of ESI; changes on CCP's side are an external risk
- Data is not real-time: ESI cache delay is unavoidable; users must understand this
- Corp data requires a character with director/manager role and the corresponding ESI scopes

---

## MVP Boundary

| Included in MVP | Not included in MVP |
|---|---|
| ESI OAuth, adding characters and corporations | Manufacturing monitoring |
| BPO library (all characters + corporations) | BPC library |
| Research monitoring (statuses, dates, slots) | Profitability analytics |
| Summary row, highlights, sorting, filters | Mineral tracking |
| Auto-refresh + manual refresh | Wails / native UI |
| Configurable refresh interval | External alerts (Discord, Push, email) |
