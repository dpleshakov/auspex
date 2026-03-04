# Auspex — project-brief.md

> Date: 21.02.2026
> Status: Immutable

---

## Problem Statement

EVE Online is a game with deep production mechanics. A serious industrialist runs multiple characters: each has their own manufacturing and research slots, their own blueprints, their own mineral stockpiles. With 3 characters on a single account and maxed-out skills, that's 30 manufacturing slots and 30 research slots.

Keeping the full picture in your head is impossible. An idle slot is lost time and lost ISK. A completed job that nobody picked up is also a loss. It's unclear what's profitable to produce right now. It's unclear which minerals are in short supply. Tools that cover adjacent problems (Evernus and similar) are oriented toward trading — manufacturing remains a blind spot.

**Root of the problem:** there is no single place where an industrialist can see their entire operation at a glance.

---

## Proposed Solution

**Auspex** is a monitoring and analysis tool for EVE Online manufacturing activity, designed for players who run multiple production characters.

Data is pulled automatically via the ESI API. No manual entry for core parameters. A single screen shows the state of all characters: occupied and free slots, active jobs, completion dates, overdue and soon-to-finish work — all highlighted, all in one table.

On top of monitoring — analytics: what's profitable to produce right now given current mineral prices and the market, and which minerals are in short supply for planned production volumes.

---

## Target Audience

EVE Online players who take manufacturing seriously — running multiple production characters (3 or more), working with corporate BPOs, focused on efficiency and ISK/hour rather than casual crafting once a week.

This is not a tool for beginners. It's a tool for players who already understand the mechanics but spend too much time and mental energy on manual tracking.

---

## Key Success Metrics

- **Zero idle slots** — the user sees a free slot before it has been idle for more than one cycle
- **Time to "check production status"** drops from 10–15 minutes to a single glance at the dashboard
- **Decisions about what to produce** are made based on up-to-date data, not intuition
- The tool is used every in-game day, not "when I remembered"

---

## Risks and Constraints

**ESI API:** the entire project depends on the availability and stability of ESI. CCP may change or restrict endpoints — this is an external risk outside the project's control.

**Data freshness:** ESI is cached on CCP's side; data updates are not real-time. Users must understand they are seeing a picture with a few minutes' delay.

**Audience:** the industrialist community in EVE is niche. There will be no broad reach. This is a deliberate choice of depth over breadth.

**Competing with "nothing":** part of the audience is used to keeping everything in their head or in Excel and isn't actively looking for a tool. Value needs to be demonstrated, not explained.

**Delivery format:** a public web service requires infrastructure, costs, and responsibility for other people's OAuth tokens. A local utility is simpler but limits the audience. The decision on delivery format is a separate phase.

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

- OAuth2 tokens are stored locally in SQLite and are never sent anywhere except to ESI — this is an explicit trust boundary decision
- The application is single-user — no multi-user authentication, no public access
- Tokens must never appear in application logs; Chi Logger logs method, URL, status, and response time only — Authorization headers are not logged
- All user-supplied input arriving at API boundaries (corporation_id, delegate_id, query parameters) must be validated before use
- ESI credentials (client_id, client_secret) are stored in a local config file that is gitignored — never committed to the repository

**Performance**

- The UI responds instantly (data from local SQLite); ESI requests run in the background

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
