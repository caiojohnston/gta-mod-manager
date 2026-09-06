# GTA V Enhanced Mod Manager

A native (Fyne) mod manager for GTA V Enhanced: install mods from a `.zip`/folder with a
reviewable file-placement step (Vortex-style), and toggle installed mods on/off with
saved profiles (like the reference toggle-only tool). Built following spec-driven
development — see `docs/SPEC.md` for what it does and doesn't do, and `docs/PLAN.md` for
the architecture.

## Status

This is a **v1 skeleton**, built in one pass following the build order in `docs/PLAN.md`.
What's here:

- ✅ `internal/model`, `internal/config`, `internal/procguard`, `internal/installer`,
  `internal/toggler`, `internal/profiles`, `internal/scanner` — all the core logic.
  **These compile and pass `go vet` cleanly** (verified in the sandbox this was built in).
- ✅ `internal/ui` + `main.go` — the Fyne screens, written against the packages above.
  **Not build-verified** in the sandbox this was built in, because that sandbox's network
  is locked down to a small allowlist that doesn't include `proxy.golang.org`,
  `golang.org`, or `gopkg.in` — and Fyne's dependency tree reaches all three. On a normal
  machine with unrestricted internet, `go mod tidy` resolves this in the usual way.

## Building (on your machine, with normal internet access)

```
go mod tidy
go build ./...
```

`go mod tidy` will download and pin the actual dependency versions (Fyne, its transitive
deps, uuid, go-ps) into `go.sum`. The `go.mod` here lists direct requirements only; you
don't need any of the GitHub-mirror workarounds that were needed to verify the logic
packages in the build sandbox.

To run it:
```
go run .
```

To produce a Windows `.exe` (the only real target, since that's what GTA V Enhanced runs
on for PC modding):
```
GOOS=windows GOARCH=amd64 go build -o gta-mod-manager.exe .
```
(Building on Windows directly, or via `fyne-cross`, is more reliable for the CGO/OpenGL
bindings Fyne needs than cross-compiling from Linux — worth trying a native Windows build
first if the cross-compile gives you linker trouble.)

## First run

1. Launch the app, click **Set game folder...**, point it at your GTA V Enhanced install.
2. If you already had mods installed manually, they should show up automatically (scanned
   via the same heuristic as most toggle tools: `.asi` files, `dinput8.dll`,
   `ScriptHookV.dll`, `scripts/`, `mods/`).
3. **Install mod...** to add something new from a `.zip`; you'll get a review screen
   before anything is copied.
4. Toggle mods with the checkboxes in the main list; use **Profiles...** to save/switch
   named sets.

## What's next (see docs/SPEC.md §5 for the full stretch-goal list)

- `.rar`/`.7z` support (v1 only handles `.zip` and plain folders).
- ScriptHookV version-mismatch warning.
- Conflict detection when two mods write the same destination path.

## Safety notes

- This only manages loose files (`.asi`, `.dll`, `.lua`, `scripts/`, `mods/`). It never
  touches `.rpf` archives — use OpenRPF for that.
- It refuses to move files while the game process is running (`procguard`).
- Installing copies files (originals untouched); toggling moves files (nothing is ever
  deleted without an explicit uninstall action, which isn't wired up yet in this v1 pass).
