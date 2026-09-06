# Roadmap

Things intentionally out of the current build, with the reasoning so the
decision isn't re-litigated from scratch.

## RPF editing bridge (CodeWalker.Core C# helper)

**Goal:** let the app do the things that currently require OpenIV / CodeWalker:
- write a file *into* a packed `.rpf` (RPF7 and RPF8/Enhanced)
- edit an XML fragment inside a file that lives in a packed `.rpf`
  (`dlclist.xml`, `extratitleupdatedata.meta`, handling/weapon metas)
- compile an editable XML resource to its binary form (`.ymt`, `.pso`,
  `.ymap`, `.ytyp`) — the RSC7/RSC8 serializers

**Why it's not in the build:** RPF8 (GTA V Enhanced, 2025) read/write and the
RAGE resource serializers exist in exactly one place — **CodeWalker.Core**
(C#, GPLv3), on its `dev` branch (`v30_dev48+`). No Go, Python, C or Rust
library covers RPF8. OpenIV is closed with no SDK. Re-implementing is a
multi-month reverse-engineering effort; CodeWalker already did it.

**Shape of the work:**
1. A small C# CLI (`rpfhelper.exe`) built against a vendored CodeWalker.Core
   (`dev` branch). Commands: `add <rpf> <inner-path> <src>`,
   `edit-dlclist <rpf> <item>`, `compile <xml> <out>`.
2. The Go app shells out to it when a task needs packed-RPF work, and keeps
   doing everything else natively.
3. Detection + a clear "not available" path when the helper isn't present.

**Costs / open questions:**
- Licensing: bundling CodeWalker.Core (GPLv3) makes the whole app GPLv3.
  A separate optional download keeps the core app's licensing free.
- Size: ~70 MB for a self-contained .NET publish, or a .NET runtime
  dependency.
- Build: needs the .NET SDK and vendoring CodeWalker.Core source (not on
  NuGet); the `dev` branch may not build standalone without trimming.
- Risk: RPF8 write is new; needs real-mod round-trip testing before trusting
  it with a user's install.

**Decision pending:** embedded helper (bigger app, GPLv3) vs. optional
"advanced mode" download (app stays lean).

## Smaller deferred items

- `.rar` / `.7z` extraction (currently: "extract it first").
- `.oiv` packages with `<xml>` / `<text>` fragment merges — needs the RPF
  bridge above; today those ops are listed as unsupported.
- ScriptHookV version-vs-game-build mismatch warning.
- Conflict detection when two mods write the same destination path
  (currently silent last-write-wins).
- Reset-rules-to-default button (rules already auto-merge new defaults on
  load; a manual reset is still missing).
- LC200-style vehicle *replace* installs (`vehicles.rpf` inner paths) — needs
  the RPF bridge or a per-file custom path from the user.
