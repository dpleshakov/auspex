# Auspex — idea.md

> Phase 1: Discovery
> Date: 20.02.2026
> Status: Current

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
