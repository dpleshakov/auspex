# Auspex — tasks-icon.md

> Module: Windows Executable Metadata
> Phase 6: Task Breakdown
> Date: 01.03.2026
> Status: Archived

---

## Scope

Embed Windows PE version resource (`VERSIONINFO`) and application icon into the `auspex.exe` binary. This makes the executable display product name, version, company, copyright, and icon in Windows Explorer's file properties dialog.

Includes: `versioninfo-meta.json`, `tools/gen-versioninfo.go`, Makefile target `versioninfo`, goreleaser integration, CONTRIBUTING.md update.

Does not include: macOS/Linux bundle metadata, code signing, notarization.

---

## Overview

Total: 3 tasks, single layer.

| Layer | Tasks | Description |
|-------|-------|-------------|
| 1 | TASK-01 – TASK-03 | Meta file, generator, build integration |

---

## Layer 1 — Implementation

### TASK-01 `versioninfo-meta.json`
**Status:** ✅ Done — commit 012ad9f

**Description:** Create `versioninfo-meta.json` containing all static fields for the Windows VERSIONINFO resource. This file holds project-level constants that do not change between releases. It is committed to the repository and edited manually only when project metadata changes (e.g. product name, copyright holder).

**Contents:**

```json
{
  "company_name": "Dmitry Pleshakov",
  "product_name": "Auspex",
  "file_description": "EVE Online manufacturing monitor",
  "internal_name": "auspex",
  "original_filename": "auspex.exe",
  "legal_copyright": "Copyright © 2026 Dmitry Pleshakov. MIT License. Not affiliated with CCP Games.",
  "icon_path": "docs/assets/auspex.ico"
}
```

**Definition of done:**
- File exists at `versioninfo-meta.json`
- All fields present and correct
- Tests: none (static data file)

**Dependencies:** none

---

### TASK-02 `tools/gen-versioninfo.go`
**Status:** ✅ Done — commit 012ad9f

**Description:** Create `tools/gen-versioninfo.go` — a Go script tagged `//go:build ignore` that reads `versioninfo-meta.json`, accepts a version string as a CLI argument, and writes the final `cmd/auspex/versioninfo.json` in the format expected by `goversioninfo`.

The script is invoked via `go run tools/gen-versioninfo.go <version>`. It must be run from the repository root.

**What the script does:**
- Reads `versioninfo-meta.json`
- Accepts version string (e.g. `0.1.0`) as `os.Args[1]`; exits with a clear error if missing or malformed
- Parses version into Major, Minor, Patch integers for `FixedFileInfo`; Build is always `0`
- Constructs the full `goversioninfo`-compatible JSON structure
- Writes `cmd/auspex/versioninfo.json`

**Output file structure (`cmd/auspex/versioninfo.json`):**

```json
{
  "FixedFileInfo": {
    "FileVersion":    { "Major": 0, "Minor": 1, "Patch": 0, "Build": 0 },
    "ProductVersion": { "Major": 0, "Minor": 1, "Patch": 0, "Build": 0 }
  },
  "StringFileInfo": {
    "CompanyName":      "Dmitry Pleshakov",
    "ProductName":      "Auspex",
    "FileDescription":  "EVE Online manufacturing monitor",
    "InternalName":     "auspex",
    "OriginalFilename": "auspex.exe",
    "LegalCopyright":   "Copyright © 2026 Dmitry Pleshakov. MIT License. Not affiliated with CCP Games.",
    "FileVersion":      "0.1.0",
    "ProductVersion":   "0.1.0"
  },
  "VarFileInfo": {
    "Translation": { "LangID": 1033, "CharsetID": 1200 }
  },
  "IconPath": "docs/assets/auspex.ico"
}
```

**Definition of done:**
- `go run tools/gen-versioninfo.go 0.1.0` produces valid `cmd/auspex/versioninfo.json`
- Missing or malformed version argument exits with a descriptive error message
- Tests: none (`//go:build ignore` script, verified manually)

**Dependencies:** TASK-01

---

### TASK-03 Build integration
**Status:** ✅ Done — commit 012ad9f

**Description:** Wire the generation into the Makefile and goreleaser. Add `goversioninfo` to CONTRIBUTING.md prerequisites. Add generated artifacts to `.gitignore`.

**Makefile — new target `versioninfo`:**

```makefile
# Generates cmd/auspex/versioninfo.json and cmd/auspex/resource.syso.
# VERSION must be set: make versioninfo VERSION=0.1.0
versioninfo:
	go run tools/gen-versioninfo.go $(VERSION)
	goversioninfo -o cmd/auspex/resource.syso cmd/auspex/versioninfo.json
```

The `versioninfo` target is standalone — it is not added to the `build` target because `resource.syso` is only meaningful when cross-compiling for Windows (`GOOS=windows`). Running it unconditionally on macOS/Linux would produce a `.syso` that the linker silently ignores, which is harmless but misleading.

**goreleaser — add hooks before the Windows build step in `.goreleaser.yaml`:**

```yaml
before:
  hooks:
    - make frontend
    - make sqlc
    - make versioninfo VERSION={{ .Version }}
    - make release-notes VERSION={{ if .IsSnapshot }}Unreleased{{ else }}{{ .Version }}{{ end }}
```

**`.gitignore` — add:**

```
cmd/auspex/versioninfo.json
cmd/auspex/resource.syso
```

**CONTRIBUTING.md — add to Prerequisites table:**

```
| goversioninfo | latest | go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest |
```

**Definition of done:**
- `make versioninfo VERSION=0.1.0` runs without error and produces both `versioninfo.json` and `resource.syso`
- `make versioninfo` without `VERSION` exits with a clear error (goversioninfo or gen-versioninfo will fail on empty string)
- goreleaser hooks call `make versioninfo VERSION={{ .Version }}` before building
- Generated files are gitignored
- `goversioninfo` listed in CONTRIBUTING.md prerequisites
- Manual verification: `go build ./cmd/auspex/` with `GOOS=windows` produces an `.exe` whose Properties → Details tab shows correct version and description

**Dependencies:** TASK-01, TASK-02

---

## Dependency Graph

```
TASK-01 (versioninfo-meta.json)
  └── TASK-02 (gen-versioninfo.go)
        └── TASK-03 (build integration)
```

---

## Status Reference

| Symbol | Meaning |
|--------|---------|
| 🔲 Pending | Not started |
| 🔄 In progress | Currently being worked on |
| ✅ Done — commit {hash} | Completed and committed |
| ⏸ Blocked — {reason} | Cannot proceed, waiting for something |
