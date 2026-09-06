package installer

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/caiojohnston/gta-mod-manager/internal/config"
	"github.com/caiojohnston/gta-mod-manager/internal/model"
	"github.com/google/uuid"
)

// ProposedFile is one entry in the review screen (SPEC.md §4.2 step 3):
// where a file came from, where the rule table thinks it should go, and
// whether the user has approved it. UI toggles Approved / overrides Dest.
type ProposedFile struct {
	SourcePath   string // absolute path in the staging dir
	RelInArchive string
	Kind         model.FileKind
	Dest         string // relative to game root; empty if Kind == KindOther and unresolved
	Approved     bool
}

// Inspect extracts a zip (or walks a plain folder) and classifies every file
// found, without copying anything into the game folder yet. Non-archive,
// non-installable files (readmes, images) are still returned so the review
// screen can show them, but Approved defaults to false for anything with no
// resolved Dest.
func Inspect(archiveOrFolder string, rules []config.RulePattern) (stagingDir string, files []ProposedFile, cleanup func(), err error) {
	info, err := os.Stat(archiveOrFolder)
	if err != nil {
		return "", nil, nil, fmt.Errorf("installer: stat %q: %w", archiveOrFolder, err)
	}

	if info.IsDir() {
		// Keep the picked folder's own name in RelInArchive (base is its
		// parent), so a mod distributed as a bare "a80/" or "vremastered/"
		// folder still carries the name the dlcpack rules need. A shared
		// wrapper dir is stripped later for display / custom-path guesses.
		files, err = classifyDir(archiveOrFolder, filepath.Dir(archiveOrFolder), rules)
		return archiveOrFolder, files, func() {}, err
	}

	stagingDir, err = os.MkdirTemp("", "gtamod-install-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("installer: creating staging dir: %w", err)
	}
	cleanup = func() { os.RemoveAll(stagingDir) }

	if err := extractZip(archiveOrFolder, stagingDir); err != nil {
		cleanup()
		return "", nil, nil, err
	}
	files, err = classifyDir(stagingDir, stagingDir, rules)
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	return stagingDir, files, cleanup, nil
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("installer: opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)
		// Guard against zip-slip: never let an archive entry escape destDir.
		if !isWithin(destDir, targetPath) {
			return fmt.Errorf("installer: unsafe path in archive: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := extractOneFile(f, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func extractOneFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func isWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !filepathHasDotDotPrefix(rel)
}

func filepathHasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[0] == '.' && rel[1] == '.'
}

func classifyDir(root, base string, rules []config.RulePattern) ([]ProposedFile, error) {
	var out []ProposedFile
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		kind := Classify(relSlash, rules)
		dest, resolved := DestinationPath(relSlash, kind)
		out = append(out, ProposedFile{
			SourcePath:   p,
			RelInArchive: relSlash,
			Kind:         kind,
			Dest:         dest,
			// Pre-tick only files with a firm home; content routed by the
			// mods/ heuristics is left for the user to confirm (SPEC.md §G6).
			Approved: resolved && AutoApprove(relSlash, kind),
		})
		return nil
	})
	return out, err
}

// Commit copies every Approved file into gameDir and returns a new Mod
// tracking exactly those files, per SPEC.md decision "copy on install".
func Commit(gameDir, modName, source string, files []ProposedFile) (model.Mod, error) {
	mod := model.Mod{
		ID:          uuid.NewString(),
		Name:        modName,
		Enabled:     true,
		InstalledAt: time.Now(),
		Source:      source,
	}
	for _, f := range files {
		if !f.Approved || f.Dest == "" {
			continue
		}
		destAbs := filepath.Join(gameDir, filepath.FromSlash(f.Dest))
		// A user-typed Custom path must not escape the game folder.
		if !isWithin(gameDir, destAbs) {
			return model.Mod{}, fmt.Errorf("installer: destination %q escapes the game folder", f.Dest)
		}
		if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
			return model.Mod{}, err
		}
		if err := copyFile(f.SourcePath, destAbs); err != nil {
			return model.Mod{}, fmt.Errorf("installer: copying %q: %w", f.RelInArchive, err)
		}
		mod.Files = append(mod.Files, model.ModFile{RelPath: f.Dest, Kind: f.Kind})
	}
	return mod, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
