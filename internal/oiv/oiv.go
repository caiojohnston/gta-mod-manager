// Package oiv reads OpenIV ".oiv" packages far enough to install the common
// case natively: packages whose assembly.xml only ADDs files into RPF archives.
// Those "add" operations map cleanly onto loose files under the game's mods/
// folder, which OpenRPF merges over the real RPFs at load time — no RPF writing
// and no resource compilation.
//
// Operations this package deliberately does NOT perform (they need OpenIV's
// engine): <xml>/<text> fragment merges, <delete>, and XML->binary resource
// compilation. Read() reports those in Package.Unsupported instead of faking
// them.
package oiv

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// AddOp is one "put this file here" operation extracted from assembly.xml.
type AddOp struct {
	// SourceName is the file's path inside the .oiv archive (its "content/"
	// prefix already stripped if present).
	SourceName string
	// ModsRelPath is where the file must land, relative to the game folder:
	// "mods/" + <archive path chain> + "/" + <inner target path>.
	ModsRelPath string
	// ExtractedPath is filled by Extract(): the real file on disk to copy from.
	ExtractedPath string
}

// Package is a parsed .oiv.
type Package struct {
	Name    string
	Version string
	Adds    []AddOp
	// Unsupported lists assembly.xml operations skipped because they need
	// OpenIV (xml/text merges, deletes). Human-readable, for the UI to show.
	Unsupported []string

	zipPath string
	tmpDir  string
}

// --- assembly.xml shape (only what we use) --------------------------------

type asmPackage struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata asmMetadata `xml:"metadata"`
	Content  asmContent  `xml:"content"`
}

type asmMetadata struct {
	Name    string `xml:"name"`
	Version struct {
		Major string `xml:"major"`
		Minor string `xml:"minor"`
		Tag   string `xml:"tag"`
	} `xml:"version"`
}

type asmContent struct {
	Archives []asmArchive `xml:"archive"`
	// content-level add/delete are rare but valid
	Adds    []asmAdd  `xml:"add"`
	Deletes []asmNode `xml:"delete"`
}

type asmArchive struct {
	Path     string       `xml:"path,attr"`
	Archives []asmArchive `xml:"archive"`
	Adds     []asmAdd     `xml:"add"`
	Deletes  []asmNode    `xml:"delete"`
	XMLOps   []asmNode    `xml:"xml"`
	TextOps  []asmNode    `xml:"text"`
}

type asmAdd struct {
	Source string `xml:"source,attr"`
	Target string `xml:",chardata"`
}

type asmNode struct {
	Path string `xml:"path,attr"`
}

// Read opens a .oiv (a zip), parses assembly.xml and returns the package. Call
// Extract to materialise the add-op source files, and Close when done.
func Read(oivPath string) (*Package, error) {
	zr, err := zip.OpenReader(oivPath)
	if err != nil {
		return nil, fmt.Errorf("oiv: open %q: %w", oivPath, err)
	}
	defer zr.Close()

	var asmBytes []byte
	for _, f := range zr.File {
		if strings.EqualFold(path.Base(f.Name), "assembly.xml") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			asmBytes, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if asmBytes == nil {
		return nil, fmt.Errorf("oiv: %q has no assembly.xml — not an OpenIV package", oivPath)
	}

	var asm asmPackage
	if err := xml.Unmarshal(asmBytes, &asm); err != nil {
		return nil, fmt.Errorf("oiv: parsing assembly.xml: %w", err)
	}

	p := &Package{
		Name:    strings.TrimSpace(asm.Metadata.Name),
		Version: strings.Trim(asm.Metadata.Version.Major+"."+asm.Metadata.Version.Minor, "."),
		zipPath: oivPath,
	}
	walkArchive("mods", asm.Content.Archives, asm.Content.Adds, asm.Content.Deletes, nil, nil, p)
	return p, nil
}

// walkArchive turns one nesting level of <archive>/<add> into AddOps under
// prefix ("mods" then "mods/update/update.rpf" then ...).
func walkArchive(prefix string, archives []asmArchive, adds []asmAdd, deletes, xmlOps, textOps []asmNode, p *Package) {
	for _, a := range adds {
		inner := cleanInner(a.Target)
		if a.Source == "" || inner == "" {
			continue
		}
		p.Adds = append(p.Adds, AddOp{
			SourceName:  normSource(a.Source),
			ModsRelPath: path.Join(prefix, inner),
		})
	}
	for _, d := range deletes {
		p.Unsupported = append(p.Unsupported, "delete "+path.Join(prefix, cleanInner(d.Path)))
	}
	for _, x := range xmlOps {
		p.Unsupported = append(p.Unsupported, "xml-merge into "+path.Join(prefix, cleanInner(x.Path)))
	}
	for _, tx := range textOps {
		p.Unsupported = append(p.Unsupported, "text-edit of "+path.Join(prefix, cleanInner(tx.Path)))
	}
	for _, sub := range archives {
		np := path.Join(prefix, cleanInner(sub.Path))
		walkArchive(np, sub.Archives, sub.Adds, sub.Deletes, sub.XMLOps, sub.TextOps, p)
	}
}

// Extract writes every add-op's source file into a temp dir and fills
// AddOp.ExtractedPath. The caller owns cleanup via Close.
func (p *Package) Extract() error {
	tmp, err := os.MkdirTemp("", "gtamod-oiv-*")
	if err != nil {
		return err
	}
	p.tmpDir = tmp

	zr, err := zip.OpenReader(p.zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	// Index by a normalised name so "content\foo\bar" and "foo/bar" both match.
	byName := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		byName[normSource(f.Name)] = f
	}

	for i := range p.Adds {
		want := p.Adds[i].SourceName
		zf := byName[want]
		if zf == nil {
			zf = byName["content/"+want]
		}
		if zf == nil {
			return fmt.Errorf("oiv: %q lists %q but the archive has no such file", p.Name, want)
		}
		dst := filepath.Join(tmp, filepath.FromSlash(want))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := extractOne(zf, dst); err != nil {
			return err
		}
		p.Adds[i].ExtractedPath = dst
	}
	return nil
}

// Close removes the temp dir created by Extract.
func (p *Package) Close() {
	if p.tmpDir != "" {
		os.RemoveAll(p.tmpDir)
		p.tmpDir = ""
	}
}

func extractOne(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// normSource lowercases separators and trims a leading "content/" so lookups
// are stable across the "\" the .oiv uses and the "/" zip readers report.
func normSource(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "content/")
	return strings.Trim(s, "/")
}

// cleanInner normalises an <archive path> or <add> target: backslashes to
// slashes, no leading slash, no "." / ".." segments.
func cleanInner(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\\", "/")
	parts := make([]string, 0, strings.Count(s, "/")+1)
	for _, seg := range strings.Split(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "/")
}
