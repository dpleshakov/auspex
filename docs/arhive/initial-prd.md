# EVE Production Tool — Initial Product Requirements Document

> **Version:** 0.1
> **Date:** 20.02.2026
> **Status:** Archive — superseded by `idea.md`, `requirements.md`, `architecture.md`

---

## Context and Purpose

A tool for managing manufacturing activity in EVE Online across multiple production characters. The core problem: with many characters and a large combined number of manufacturing and research slots, it is impossible to keep the full picture in mind. Efficiency is lost — idle slots go unnoticed, completed jobs are not picked up in time.

**Out of scope:** trading functionality (market orders, wallet journal, trade history) — covered by Evernus.

---

## Modules

### 1. Slot and Job Monitoring

**Problem:** An idle slot is lost time and ISK. With multiple characters, this is easy to miss.

**Features:**
- Display of used / free slots per character (manufacturing and research separately)
- List of active jobs: type (manufacturing / ME research / TE research / copying), blueprint, product, completion time
- Overview screen: all characters at a glance

**Notifications:**
- Job completed → notification (Telegram or other channel)
- Job completing in N minutes → warning (configurable threshold)
- A character has a free slot available

**Data source:** ESI `/characters/{id}/industry/jobs/`, `/characters/{id}/skills/`

---

### 2. BPO Library

**Problem:** A full blueprint list is needed as a data source for calculations — even if in-game it is accessible from a single corporate character.

**Features:**
- List of all corporation BPOs: name, ME, TE, location, assigned character
- Filtering by category, character, location
- Status: in research / ready for manufacturing / in copying

**Note:** A dedicated UI screen may not be required — data is consumed internally by the profitability module.

**Data source:** ESI `/corporations/{id}/blueprints/`, `/corporations/{id}/industry/jobs/`

---

### 3. Manufacturing Profitability Analysis

**Problem:** It is unclear what is profitable to produce right now given current mineral prices and the market.

**Features:**
- List of producible items (based on corporation BPOs)
- Production cost calculation taking into account:
  - Blueprint ME level
  - Current mineral prices (from the minerals module)
  - Optionally: purchase price of a specific mineral batch (manual input)
- Market price of the item (Jita sell orders, minimum or percentile)
- Profit (ISK) and margin (%) per unit
- Sorting by profitability

**Out of scope:** price prediction — low accuracy on thin markets, high implementation complexity.

**Data source:** ESI `/markets/{region_id}/orders/`, `/markets/{region_id}/history/`, BPOs from module 2

---

### 4. Mineral Management

**Problem:** Need to understand current stockpiles, average cost, and how much to buy for planned production volumes.

**Features:**
- Current stock per mineral (aggregated across all characters and locations)
- Average market price over N days (configurable period)
- Target stock levels (set manually or derived from production plan)
- Calculation: how much needs to be purchased and at what ISK cost
- Total stockpile value at current prices

**Data source:** ESI `/characters/{id}/assets/` or `/corporations/{id}/assets/`, `/markets/{region_id}/history/`

---

### 5. Summary Dashboard

**Goal:** A single screen providing a complete operational picture without navigating between sections.

**Contents:**
- Characters: slots (used / total), nearest job completion
- Total active jobs by type
- Free slots (highlighted when available)
- Mineral stockpiles: deviation from target levels (OK / needs restocking)
- Top-3 most profitable items right now

---

## Tech Stack (preliminary)

| Component | Solution |
|---|---|
| Language | Go |
| Storage | SQLite |
| Authorization | OAuth2 + ESI SSO, refresh tokens |
| Scheduler | In-process cron |
| Notifications | Telegram Bot API |
| Deployment | VPS, single binary |

---

## ESI Data vs Manual Input

| Data | Source |
|---|---|
| Active jobs, slots | ESI (automatic) |
| Corporation BPO / BPC | ESI (automatic) |
| Mineral prices | ESI market history (automatic) |
| Mineral stockpiles | ESI assets (automatic) |
| Item market prices | ESI market orders (automatic) |
| Specific mineral batch purchase price | Manual input |
| Target mineral stock levels | Manual input |
| Omega / MPT dates | Manual input (changes rarely) |

---

## Out of Scope

- Trading functionality (market orders, wallet, trade history) — Evernus
- Price prediction
- Multi-user support
- Mobile application (Telegram notifications are sufficient)

---

## Open Questions

- [ ] Where to host: VPS or home server?
- [ ] Is a web interface needed or is CLI + Telegram sufficient?
- [ ] What default price averaging period to use for minerals?
- [ ] Notifications via Telegram only, or other channels as well?
