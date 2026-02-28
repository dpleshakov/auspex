# Changelog

All notable changes to Auspex are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [0.1.0] — 2026-02-28

### Added

- EVE SSO sign-in flow for adding manufacturing characters; each character can be removed at any time, which also removes all associated data.
- Corporation support: link a corporation through a delegate character to include its blueprint library alongside personal ones.
- Blueprint library table showing all BPOs across all connected characters and corporations in one view; columns include name, category, ME%, TE%, research status, owner, location, and remaining time.
- Research status labels shown in the table: Idle, ME Research, TE Research, Copying, Ready.
- Summary bar with aggregate counts: idle blueprints, overdue jobs, jobs completing today, and per-character research slot usage.
- Row highlighting: overdue jobs shown in red; jobs completing today shown in yellow.
- Filter dropdowns for status, owner, and category; a "Clear filters" button resets all active filters at once.
- Default sort order by urgency: overdue rows first, then Ready, Idle, Active; secondary sort by end date.
- Auto-refresh every 10 minutes; manual refresh button triggers an immediate sync with a live "Refreshing…" indicator.
- Single self-contained binary; SQLite database stored as a single file next to the binary; no external services required.
- `auspex.example.yaml` configuration template documenting all settings and required EVE Developer App scopes.
