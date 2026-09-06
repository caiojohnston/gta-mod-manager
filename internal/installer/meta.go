package installer

import (
	"bytes"
	"os"
	"path"
	"strings"
)

// metaExts are extensions whose game-side form is a compiled binary resource
// even when a mod author ships them as editable XML — dropping the raw XML in
// place does nothing, the game only reads the binary, and converting needs
// CodeWalker. Excluded on purpose: .meta and .xml — the RAGE parser reads
// those as text at load time (weapons.meta, handling.meta, dlclist.xml, …), so
// loose XML there is fine.
var metaExts = map[string]bool{
	".ymt": true, ".pso": true, ".ymap": true, ".ytyp": true, ".ymf": true,
}

// NeedsCompile reports whether f is an uncompiled XML meta file: a metaExts
// extension whose content starts with an XML declaration or tag. Such a file
// must be run through CodeWalker (XML -> binary) before the game will use it;
// the manager only copies files, it can't convert them.
func NeedsCompile(f ProposedFile) bool {
	if !metaExts[strings.ToLower(path.Ext(f.RelInArchive))] {
		return false
	}
	fh, err := os.Open(f.SourcePath)
	if err != nil {
		return false
	}
	defer fh.Close()
	head := make([]byte, 64)
	n, _ := fh.Read(head)
	head = bytes.TrimLeft(head[:n], " \t\r\n\xEF\xBB\xBF")
	return bytes.HasPrefix(head, []byte("<?xml")) || bytes.HasPrefix(head, []byte("<"))
}

// UncompiledMeta returns the archive-relative paths of every approved file in
// files that NeedsCompile — for the wizard's pre-install warning.
func UncompiledMeta(files []ProposedFile) []string {
	var out []string
	for _, f := range files {
		if f.Approved && f.Dest != "" && NeedsCompile(f) {
			out = append(out, f.RelInArchive)
		}
	}
	return out
}
