package rpf

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsNonRPF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.bin")
	os.WriteFile(p, []byte("not an rpf at all, just text"), 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("Open should reject a file without the 7FPR magic")
	}
}

func TestOpenRejectsEncrypted(t *testing.T) {
	// Minimal 16-byte header: 7FPR, 0 entries, 0 names, NG encryption.
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h[0:4], magic7)
	binary.LittleEndian.PutUint32(h[12:16], encNG)
	p := filepath.Join(t.TempDir(), "ng.rpf")
	os.WriteFile(p, h, 0o644)
	if _, err := Open(p); err == nil {
		t.Fatal("Open should refuse an NG-encrypted archive")
	}
}

func TestU24(t *testing.T) {
	if got := u24([]byte{0x15, 0x02, 0x00}); got != 0x0215 {
		t.Errorf("u24 = %#x, want 0x215", got)
	}
	if got := u24([]byte{0xFF, 0xFF, 0x7F}); got != 0x7FFFFF {
		t.Errorf("u24 = %#x, want 0x7FFFFF", got)
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"a80.yft":            "a80.yft",
		"x64\\vehicles.rpf":  "vehicles.rpf",
		"../../escape":       "escape",
		"":                   "_",
		"..":                 "_",
		"deep/path/file.dat": "file.dat",
	}
	for in, want := range cases {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrependRSC7(t *testing.T) {
	out := prependRSC7("model.yft", 0xAABBCCDD, 0x11223344, []byte("body"))
	if len(out) != 20 {
		t.Fatalf("len = %d, want 20 (16 header + 4 body)", len(out))
	}
	if !bytes.Equal(out[0:4], []byte("RSC7")) {
		t.Errorf("magic = %q, want RSC7", out[0:4])
	}
	if binary.LittleEndian.Uint32(out[4:8]) != resourceVersion[".yft"] {
		t.Errorf("version = %d, want %d", binary.LittleEndian.Uint32(out[4:8]), resourceVersion[".yft"])
	}
	if binary.LittleEndian.Uint32(out[8:12]) != 0xAABBCCDD {
		t.Error("system flags not carried through")
	}
	if string(out[16:]) != "body" {
		t.Error("body not appended")
	}
}
