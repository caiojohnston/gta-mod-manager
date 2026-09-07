package rpf

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestPackRoundTrip(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"setup2.xml":              "<setup/>",
		"content.xml":             "<content/>",
		"carx/data/vehicles.meta": "vehicles-meta-text-body",
		"carx/data/handling.meta": "handling-body",
		"carx/x64/vehicles.rpf":   "packed-rpf-bytes-0123456789abcdef",
		"cary/data/vehicles.meta": "cary-vehicles",
	}
	for rel, body := range files {
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	packed := filepath.Join(t.TempDir(), "out.rpf")
	if err := Pack(src, packed); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	ar, err := Open(packed)
	if err != nil {
		t.Fatalf("Open packed: %v", err)
	}
	if len(ar.entries) == 0 || !ar.entries[0].isDir {
		t.Fatalf("bad root entry")
	}

	for rel, want := range files {
		got, err := ReadInner(packed, rel)
		if err != nil {
			t.Errorf("ReadInner(%s): %v", rel, err)
			continue
		}
		if !bytes.Equal(got, []byte(want)) {
			t.Errorf("%s: got %q want %q", rel, got, want)
		}
	}

	ext := filepath.Join(t.TempDir(), "ext")
	n, err := ExtractTo(packed, ext)
	if err != nil {
		t.Fatalf("ExtractTo: %v", err)
	}
	if n != len(files) {
		t.Errorf("extracted %d files, want %d", n, len(files))
	}
	b, _ := os.ReadFile(filepath.Join(ext, "carx", "data", "vehicles.meta"))
	if string(b) != files["carx/data/vehicles.meta"] {
		t.Errorf("extracted mismatch: %q", b)
	}
}

// A nested vehicles.rpf routinely runs 20-30 MB, past RPF7's u24 on-disk size
// field. Pack must store such a file with the size in the u32 word so it reads
// back intact instead of being mistaken for a deflate stream.
func TestPackLargeFile(t *testing.T) {
	src := t.TempDir()
	big := make([]byte, 20<<20+123) // 20 MB + change, > 0xFFFFFF
	for i := range big {
		big[i] = byte(i*2654435761 + 7)
	}
	writeTree(t, src, map[string][]byte{
		"setup2.xml":          []byte("<setup/>"),
		"content.xml":         []byte("<content/>"),
		"car/x64/vehicles.rpf": big,
		"car/data/x.meta":     []byte("small"),
	})

	packed := filepath.Join(t.TempDir(), "big.rpf")
	if err := Pack(src, packed); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	got, err := ReadInner(packed, "car/x64/vehicles.rpf")
	if err != nil {
		t.Fatalf("ReadInner big: %v", err)
	}
	if !bytes.Equal(got, big) {
		t.Fatalf("large file round-trip mismatch: got %d bytes want %d", len(got), len(big))
	}
}

// Pack must recognise an RSC7-wrapped input (a compiled model, as CodeWalker
// exports it), store the deflate body as a resource entry with the flags on the
// row, and hand it back through ReadInner with an RSC7 header again.
func TestPackResource(t *testing.T) {
	body := bytes.Repeat([]byte("model-bytes-"), 400)
	var deflated bytes.Buffer
	fw, _ := flate.NewWriter(&deflated, flate.BestCompression)
	fw.Write(body)
	fw.Close()

	const sysFlags, gfxFlags = 0x9000A1B2, 0x8000C3D4
	rsc7 := make([]byte, 16)
	copy(rsc7, "RSC7")
	binary.LittleEndian.PutUint32(rsc7[4:8], 162) // .yft version
	binary.LittleEndian.PutUint32(rsc7[8:12], sysFlags)
	binary.LittleEndian.PutUint32(rsc7[12:16], gfxFlags)
	rsc7 = append(rsc7, deflated.Bytes()...)

	src := t.TempDir()
	writeTree(t, src, map[string][]byte{
		"content.xml":       []byte("<c/>"),
		"x64/tailgater.yft": rsc7,
	})
	packed := filepath.Join(t.TempDir(), "res.rpf")
	if err := Pack(src, packed); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	got, err := ReadInner(packed, "x64/tailgater.yft")
	if err != nil {
		t.Fatalf("ReadInner: %v", err)
	}
	if string(got[0:4]) != "RSC7" {
		t.Fatalf("no RSC7 header back: % x", got[0:4])
	}
	if g := binary.LittleEndian.Uint32(got[8:12]); g != sysFlags {
		t.Errorf("sysFlags = %#x want %#x", g, sysFlags)
	}
	if g := binary.LittleEndian.Uint32(got[12:16]); g != gfxFlags {
		t.Errorf("gfxFlags = %#x want %#x", g, gfxFlags)
	}
	if !bytes.Equal(got[16:], deflated.Bytes()) {
		t.Errorf("deflate body mismatch: got %d want %d bytes", len(got)-16, deflated.Len())
	}
}

func writeTree(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
