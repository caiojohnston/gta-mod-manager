# GTA V Enhanced Mod Manager

A native (Fyne) mod manager for GTA V Enhanced: install mods from a `.zip` or
folder with a reviewable file-placement step (Vortex-style), and toggle
installed mods on/off with saved profiles (like a toggle-only tool). Built
spec-driven — see [`docs/SPEC.md`](docs/SPEC.md) for scope and
[`docs/PLAN.md`](docs/PLAN.md) for the architecture.

## What works (v1)

- **Auto-detect the game folder** — reads the Rockstar/Steam/Epic registry keys,
  Steam's `libraryfolders.vdf`, Epic's manifests, and common install paths, then
  confirms each by looking for `GTA5_Enhanced.exe` (Legacy `GTA5.exe` is picked
  up too, ranked lower). Manual folder picker as a fallback.
- **Base-tool buttons** — *Install ScriptHookV* and *Install OpenRPF* open the
  official download page, take the `.zip` (auto-spotted in your Downloads
  folder when possible), and fill in the correct placement for you
  (`ScriptHookV.dll`/`dinput8.dll` in root; only `OpenIV.asi` for OpenRPF, its
  proxy DLLs skipped so they don't clobber ScriptHookV's). Set a direct URL in
  `config.json` → `downloads` and the button downloads it itself.
- **Get OpenIV** — opens openiv.com (and, with `downloads.openIVURL` set,
  saves the installer to your Downloads folder — never runs it). OpenIV is the
  separate app that installs `.oiv` packages, which this tool doesn't handle.
- **Run GTA** — launches the game via `PlayGTAV.exe` (the shim that hands off to
  Steam/Epic/Rockstar), falling back to `GTA5_Enhanced.exe`.

`.rar`/`.7z` archives and `.oiv` packages are detected and refused with a note
on what to do instead (extract first / use OpenIV).
- **Install from `.zip` or folder** with a review screen: every file is
  classified by the rule table, and you can untick extras or override any file's
  destination — Game root / `scripts/` / `mods/` (path preserved) / a free-form
  **Custom path**, or **Set base path for all** to prefix every file at once
  (the fast path for content mods whose ReadMe says "everything under `mods/…`").
- **Add-on dlcpacks** — a `dlc.rpf` (in `<pack>/dlc.rpf` or `…/dlcpacks/<pack>/
  dlc.rpf`) is routed to `mods/update/x64/dlcpacks/<pack>/dlc.rpf`, and after
  install the app offers to add `<Item>dlcpacks:/<pack>/</Item>` to the loose
  `mods/update/update.rpf/common/data/dlclist.xml` (**Register dlcpacks** button
  does it for already-installed packs too). One-time prep it can't do: extract
  `dlclist.xml` from `update.rpf` once with CodeWalker/OpenIV, and copy
  `update/update.rpf` to `mods/update/update.rpf`.
- **Content mods** (RPF-tree mods: `.rpf`, `common/`, `dlcpacks/`, `platform/`,
  `.ymt`/`.meta`/`.ytd` …) route to the `mods/` folder with their sub-path kept,
  left unticked for you to confirm the exact target. They load in-game only with
  **OpenRPF** (or OpenIV) installed in the game folder — this tool places the
  files, OpenRPF makes the game read them.
- **`.oiv` packages (partial, native)** — an OpenIV package whose `assembly.xml`
  only *adds* files is installed by placing each file as a loose override under
  `mods/` (OpenRPF then merges it). Packages that also do XML-fragment merges,
  deletes, or need resource compilation (XML `.ymt` → binary) are only partially
  handled: those steps are listed and you finish them in OpenIV. Full RPF write
  + resource compilation is OpenIV's job, not this tool's.
- **Toggle mods** on/off individually or in bulk (Enable all / Disable all).
  Disabling moves a mod's tracked files to `Disabled mods/<name>/` inside the
  game folder; enabling moves them back. Empty folders left behind are pruned.
- **Launch Clean** — disables every enabled mod for one session and remembers
  what to turn back on (survives an app restart); the button becomes
  **Restore mods**.
- **Profiles** — save the current on/off set under a name, apply one later
  (only the difference is moved), or delete one.
- **Uninstall** — the one action that deletes rather than moves a mod's files,
  behind a confirmation.
- **Safety**: refuses every file move while `GTA5_Enhanced.exe` is running;
  never touches `.rpf` archives.
- First-run scan (and **Rescan folder**) imports mods already sitting in the
  folder — loose `.asi`/`dinput8.dll`/`dsound.dll`/`ScriptHookV.dll`, the
  `scripts/` folder, and **one entry per child of `mods/`** (`mods/update`,
  `mods/x64`, `mods/common.rpf`, …) so an OpenIV "install to mods folder" is
  toggleable here in useful chunks. Only files not already tracked are picked
  up, so app installs and OpenIV installs don't collide.

Config + mod registry live at `%AppData%\GTAVGoModManager\config.json`.

## Building (Windows)

Fyne needs CGO and a C compiler. Install **MinGW-w64** once (e.g.
`choco install mingw`), then:

```powershell
./build.ps1            # builds gta-mod-manager.exe
./build.ps1 -Run       # build then launch
./build.ps1 -Test      # go test ./...
```

`build.ps1` finds MinGW in the usual spots and sets `CGO_ENABLED=1` for you.
Manual equivalent:

```powershell
$env:Path = "C:\msys64\mingw64\bin;" + $env:Path
$env:CGO_ENABLED = "1"
go build -o gta-mod-manager.exe .
```

## First run

1. Launch the app. It tries to find GTA V automatically — confirm the folder it
   found, or pick between several, or use **Set folder manually...**.
2. Mods already installed by hand are imported and shown in the list.
3. **Install mod...** to add one from a `.zip` (or an extracted folder); review
   the file placement, then confirm.
4. Toggle with the checkboxes; **Profiles...** to save/switch sets;
   **Launch Clean** before playing GTA Online.

## Tests

Core logic (everything except the Fyne UI) is covered by unit tests:

```powershell
go test ./internal/...
```

`internal/gamedetect` also has an opt-in real-machine probe:
`GAMEDETECT_MANUAL=1 go test ./internal/gamedetect -run Manual -v`.

## Not in v1

- `.rar` / `.7z` archives (only `.zip` and plain folders).
- ScriptHookV version-mismatch warning.
- Conflict detection when two mods write the same destination path
  (currently last-write-wins, silently).
- GTA Online firewall / offline-mode automation.

## Safety notes

- Manages only loose files (`.asi`, `.dll`, `.lua`, `scripts/`, `mods/`). It
  never edits `.rpf` archives — use OpenRPF for that.
- Installing copies files (originals untouched). Toggling moves files. Only
  **Uninstall** deletes, and it asks first.
