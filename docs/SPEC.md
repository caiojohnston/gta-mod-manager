# GTA V Enhanced Mod Manager (Go + Fyne) — Spec

**Status:** Draft v1
**Created:** 2026-09-06

## 1. Problem statement

Managing mods for GTA V Enhanced today means juggling two separate concerns by hand:

1. **Installing** a mod means unzipping an archive and copying the right files to the right
   place in the game folder (root for `.asi`/`dinput8.dll`, `scripts/` for `.dll`/`.lua`
   script mods, etc.), while ignoring parts of the archive you don't want (extra readmes,
   alternate color variants, optional sub-mods).
2. **Toggling** mods on/off means knowing which files belong to which mod, and moving them
   in and out of the game folder without breaking anything — useful for keeping GTA Online
   clean, or for going back to a vanilla install to test a crash.

Existing tools solve one half each: Vortex-style managers solve (1), toggle-only tools
solve (2). This project combines both in a single Go binary with a native (Fyne) GUI.

## 2. Goals (v1 scope)

- **G1 — Install from archive or folder.** User points the app at a `.zip` (v1; `.rar`/`.7z`
  are a stretch goal, see §5) or an already-extracted folder. The app inspects the contents,
  proposes a destination for each file based on type/name rules (§4.2), and lets the user
  confirm or override before copying anything.
- **G2 — Track mods as units.** Every install is recorded as a "mod" (name + list of files it
  placed, with their destination paths). This is what makes toggling and uninstalling
  possible without guesswork later.
- **G3 — Toggle mods on/off.** Disabling a mod moves its tracked files into
  `Disabled mods/<mod-name>/...` inside the game folder (mirroring their original relative
  path); enabling moves them back. The game never sees a disabled mod's files.
- **G4 — Profiles.** Named sets of "which mods are enabled" (e.g. "Graphics only", "Clean"),
  switchable with one click. A profile switch computes the diff and only moves what changed.
- **G5 — Safety rails.**
  - Refuse to move files while `GTA5_Enhanced.exe` (or configured process name) is running.
  - Never touch loose `.rpf` files — those are out of scope (see Non-goals) and a game
    update could add new ones; the app must not mistake them for mod files.
  - Every destructive action (toggle, install, uninstall) is undoable by construction,
    because files are moved, never deleted, until the user explicitly uninstalls.
- **G6 — Manual placement fallback.** When a file's destination can't be determined
  confidently, the app must show it to the user in a checklist/dropdown rather than
  guess. Automating *where a known file type goes* is fine; automating *which optional
  component the user wants* is explicitly not attempted (see conversation history: this
  is a human decision, not a technical one).

## 3. Non-goals (v1)

- **RPF archive editing/repacking.** Replacing vanilla `.rpf` assets is OpenRPF's job.
  This tool only manages loose files (`.asi`, `.dll`, `.lua`, `scripts/` contents, `mods/`
  folder contents already produced by OpenRPF).
- **GTA Online safety automation** (firewall rules, forcing offline mode). Real, but
  higher-risk surface (touches Windows firewall/network config) — deferred to a later
  phase once the core toggle/install loop is solid.
- **`.rar` / `.7z` extraction.** Needs a CGO or external-binary dependency; v1 supports
  `.zip` (stdlib `archive/zip`) and raw folders. Tracked as a fast-follow.
- **Auto-updating ScriptHookV or other third-party tools.** Out of scope; the app only
  manages files the user explicitly installs through it, plus mods it detects already
  sitting in the game folder (imported on first scan, same heuristic as the reference
  tool: `.asi`, `dinput8.dll`, `ScriptHookV.dll`, `scripts/`, `mods/`).
- **Multiplayer/online mod support of any kind.**

## 4. Functional requirements

### 4.1 First-run / folder scan
- User selects (or app auto-detects, e.g. default Steam path) the GTA V Enhanced install
  folder.
- App scans the folder for files/folders matching known mod patterns not already tracked
  in its config, and offers to import them as one "Unmanaged mods" entry per top-level
  item, so pre-existing manual installs aren't lost or duplicated.

### 4.2 Install pipeline (the "Vortex-like" half)
Given a `.zip` or folder:
1. Extract to a temp staging dir (if archive).
2. Walk all files, classify each by extension/name using this default rule table
   (user-editable in config, not hard-coded assumptions):

   | Pattern                              | Destination            |
   |---------------------------------------|-------------------------|
   | `dinput8.dll`, `ScriptHookV.dll`, `*.asi` | game root            |
   | already under a `scripts/` path        | preserve path under `scripts/` |
   | already under a `mods/` path           | preserve path under `mods/`    |
   | `*.dll` not matched above               | `scripts/` (ScriptHookVDotNet convention) |
   | `*.lua`                                | `scripts/`               |
   | anything else (readme, images, `.txt`) | **not installed** — shown as "extras", user can still force-include |

3. Present a review screen: checklist of files with their proposed destination; user can
   uncheck files they don't want, or override an individual file's destination.
4. On confirm, copy (not move) the chosen files into place, record them under a new mod
   entry (name prompted from the user, default = archive/folder name).

### 4.3 Toggle
- Mod list UI shows every tracked mod with an on/off switch.
- Flipping a switch moves that mod's files to/from `Disabled mods/<mod-name>/`.
- Bulk actions: "Launch Clean" (disable everything for this run without changing saved
  state), "Enable All", "Disable All".

### 4.4 Profiles
- "Save current state as profile" (name it).
- Switching profile = diff current enabled set vs target profile's set, move only the
  delta.

### 4.5 Persistence
- Config + mod registry stored at `%AppData%\GTAVGoModManager\config.json` (Windows).
- Schema versioned from day one (`"schemaVersion": 1`) so future migrations don't break
  existing installs.

## 5. Stretch goals (explicitly deferred, not v1)
- `.rar`/`.7z` support.
- ScriptHookV version-mismatch warning (compare `ScriptHookV.dll` version vs game exe).
- Firewall/offline-mode automation for GTA Online safety.
- Conflict detection when two mods claim the same destination path.

## 6. Acceptance criteria for v1
- [x] Can point the app at a real GTA V Enhanced folder and see existing mods imported.
      (Plus: the folder is auto-detected on first run — see §4.1.)
- [x] Can install a `.zip` containing an `.asi` mod and a folder of `scripts/` and end up
      with correct files in the correct places, with the mod tracked as one unit.
      (Covered by `installer.TestInspectAndCommitZip`.)
- [x] Can toggle that mod off and confirm the files are gone from the live game folder
      (moved to `Disabled mods/`), then toggle it back on.
      (Covered by `toggler.TestDisableEnableRoundtrip` and `ui.TestToggleMovesFilesAndPersists`.)
- [x] Can create two profiles and switch between them without manual file moves.
      (Covered by `profiles.TestSwitchMovesOnlyDelta`.)
- [x] App refuses to act while the game process is running.
      (Covered by `toggler.TestGuardBlocksWhenGameRunning`.)
