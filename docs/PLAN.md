# Technical Plan

Derived from `SPEC.md`. This is the source of truth for structure/decisions before writing
task-level code changes.

## Stack
- **Language:** Go 1.22+
- **GUI:** Fyne v2 (native widgets, single binary, no webview dependency)
- **Archives:** stdlib `archive/zip` for v1 (rar/7z deferred, see SPEC §5)
- **Config/registry:** JSON file, versioned schema, via stdlib `encoding/json`
- **Process check:** `github.com/mitchellh/go-ps` (small, no CGO) to detect if the game is running
- **Target OS:** Windows (the only platform GTA V Enhanced runs on for PC modding purposes).
  Development/compilation can happen cross-platform; paths and process-list code are
  isolated behind an OS-specific file so a Linux dev build is possible for quick iteration.

## Package layout

```
gta-mod-manager/
├── docs/
│   ├── SPEC.md
│   └── PLAN.md
├── go.mod
├── build.ps1                    # CGO build helper (finds MinGW, sets env)
├── main.go                     # entrypoint: builds Fyne app, wires screens
├── internal/
│   ├── config/
│   │   └── config.go           # load/save %AppData% config.json, schema versioning
│   ├── model/
│   │   └── mod.go              # Mod, ModFile, Profile types shared everywhere
│   ├── scanner/
│   │   └── scanner.go          # first-run detection of pre-existing mods (§4.1)
│   ├── gamedetect/
│   │   ├── gamedetect.go       # validate + rank candidate GTA V folders (§4.1)
│   │   ├── detect_windows.go   # registry / Steam VDF / Epic manifests probe
│   │   └── detect_other.go     # no-op probe for non-Windows dev builds
│   ├── installer/
│   │   ├── installer.go        # zip/folder -> classified file list (§4.2)
│   │   └── rules.go            # the destination rule table, user-overridable
│   ├── toggler/
│   │   └── toggler.go          # enable/disable/uninstall, move to/from "Disabled mods/" (§4.3)
│   ├── profiles/
│   │   └── profiles.go         # profile diff + switch logic (§4.4)
│   ├── procguard/
│   │   └── procguard.go        # "is the game running?" check (§G5)
│   └── ui/
│       ├── modlist.go          # main screen: mod list + toggles + toolbar + status bar
│       ├── actions.go          # shared App methods (detect, scan, toggle, bulk, clean, uninstall)
│       ├── install_wizard.go   # install review screen with per-file destination override (§4.2 step 3)
│       └── profiles_panel.go   # profile save / apply / delete UI
└── README.md
```

Every non-UI package has `*_test.go` coverage; `internal/ui` is tested headless
via `fyne.io/fyne/v2/test`. `go test ./...` is green.

## Key decisions & rationale

1. **Files are units of a `Mod`, not paths guessed at toggle-time.** The installer records
   exactly which files it placed for a given mod (`model.Mod.Files []ModFile`, each with
   `RelPath` and `Kind`). Toggling never has to re-derive "what belongs to this mod" —
   this avoids the exact ambiguity problem discussed earlier (a manager can't read the
   user's mind about *what's inside* a package, but once files are placed and attributed
   to a mod, moving them later is 100% mechanical).

2. **Copy on install, move on toggle.** Installing copies from the staging dir so the
   original archive/folder is untouched (user can re-run install if something goes wrong).
   Toggling moves files (no duplication) between the live game folder and
   `Disabled mods/<mod-name>/`, preserving relative paths so re-enabling is a straight
   reverse move.

3. **Rule table is data, not code.** `installer/rules.go` holds the default rule table as a
   plain struct/slice that gets serialized into the config on first run, so users can edit
   it later (e.g. add a new pattern for a script framework not covered by default) without
   recompiling.

4. **Scanner heuristic reuses the same proven patterns** as the reference tool the user
   found (`.asi`, `dinput8.dll`, `ScriptHookV.dll`, `scripts/`, `mods/`), so pre-existing
   manual installs import cleanly instead of being flagged as "unknown vanilla changes".

5. **Fyne, not Wails/web.** Chosen by the user — single Go binary, no embedded webview,
   simplest packaging story for a Windows `.exe` a non-technical friend could also run.

## Build order (maps to tasks, one PR-sized chunk each)

1. `model` + `config` — types and persistence, no UI yet, unit-testable in isolation. ✅
2. `procguard` — trivial, needed before anything that touches files. ✅
3. `installer` (zip → classified list, no copy yet) + `rules.go` defaults. ✅
4. `toggler` — enable/disable/uninstall given a `Mod` already in the registry. ✅
5. `profiles` — diff-based switch on top of `toggler`. ✅
6. `scanner` — first-run import of pre-existing mods. ✅
7. `gamedetect` — auto-locate the install folder (registry / Steam / Epic). ✅
8. `ui` — wire all of the above into Fyne screens, last because it depends on everything
   else having a stable API. ✅

## Open questions to revisit after v1
- Conflict detection (two mods writing the same path) — currently silently last-write-wins.
- Where exactly ScriptHookVDotNet expects generic `.dll` mods vs `scripts/Plugins/` — the
  v1 rule table's guess should be validated against a couple of real mods before shipping.
