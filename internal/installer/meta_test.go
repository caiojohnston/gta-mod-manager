package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTmp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNeedsCompile(t *testing.T) {
	xmlYmt := writeTmp(t, "physicstasks.ymt", "\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<CTuningFile>...")
	binYmt := writeTmp(t, "compiled.ymt", "\x07\x00\x00\x00RSC7\x00\x00binary junk")
	plainXML := writeTmp(t, "dlclist.xml", "<?xml version=\"1.0\"?><SMandatoryPacksData/>")
	readme := writeTmp(t, "notes.txt", "hello")

	cases := []struct {
		path string
		want bool
	}{
		{xmlYmt, true},
		{binYmt, false},
		{plainXML, false}, // .xml is excluded — the game reads it as text
		{readme, false},
	}
	for _, c := range cases {
		f := ProposedFile{SourcePath: c.path, RelInArchive: filepath.Base(c.path)}
		if got := NeedsCompile(f); got != c.want {
			t.Errorf("NeedsCompile(%s) = %v, want %v", filepath.Base(c.path), got, c.want)
		}
	}
}

func TestUncompiledMetaOnlyApproved(t *testing.T) {
	xmlYmt := writeTmp(t, "a.ymt", "<?xml version=\"1.0\"?><x/>")
	files := []ProposedFile{
		{SourcePath: xmlYmt, RelInArchive: "a.ymt", Dest: "mods/update/update.rpf/x/a.ymt", Approved: true},
		{SourcePath: xmlYmt, RelInArchive: "b.ymt", Dest: "", Approved: false},
	}
	got := UncompiledMeta(files)
	if len(got) != 1 || got[0] != "a.ymt" {
		t.Fatalf("UncompiledMeta = %v, want [a.ymt]", got)
	}
}
