// Package rpf reads GTA V RPF7 archives (the "7FPR" format used by Legacy GTA V
// and by CodeWalker/OpenIV "OpenFormats" mod redistributions) and extracts
// them to a loose folder tree.
//
// This is deliberately a *reader* for the unencrypted case only: archives
// tagged "OPEN" (OpenFormats) or with no encryption. It does NOT decrypt the
// AES/NG-encrypted archives that ship inside the game, and it does NOT write
// RPF8 (Enhanced) — that stays OpenIV/CodeWalker territory. Its one job is to
// turn a mod's packed dlc.rpf into the loose files OpenRPF loads.
package rpf

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const magic7 = 0x52504637 // "7FPR" little-endian

// encryption tag values seen in the 4th header word.
const (
	encNone = 0x00000000
	encOpen = 0x4E45504F // "OPEN"
	encAES  = 0xFEFFFFFF
	encNG   = 0x0FFFFFF9
)

type entry struct {
	name       string
	isDir      bool
	childIndex uint32
	childCount uint32
	// file fields
	sizePacked   uint32 // on-disk size
	offsetBlocks uint32 // *512 = byte offset of data
	sizeRaw      uint32 // uncompressed size (binary) / unused (resource)
	deflated     bool
	// resource fields
	isResource    bool
	systemFlags   uint32
	graphicsFlags uint32
}

// resourceVersion is the RSC7 header version by file extension, as CodeWalker
// reconstructs it (RPF7 doesn't store it in the entry).
var resourceVersion = map[string]uint32{
	".ydr": 165, ".ydd": 165, ".yld": 165, ".yed": 165,
	".yft": 162, ".ytd": 13, ".ybn": 43, ".ycd": 8,
	".ynv": 2, ".ymap": 2, ".ytyp": 2, ".ymt": 2, ".ymf": 2,
	".ypt": 68, ".ywd": 6, ".yfd": 4, ".ypdb": 2, ".ycd2": 8,
}

// Archive is a parsed RPF7.
type Archive struct {
	data    []byte
	entries []entry
}

// Open parses the RPF7 at path. Returns an error for encrypted archives.
func Open(p string) (*Archive, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

func parse(data []byte) (*Archive, error) {
	if len(data) < 16 || binary.LittleEndian.Uint32(data[0:4]) != magic7 {
		return nil, fmt.Errorf("rpf: not an RPF7 archive")
	}
	entryCount := binary.LittleEndian.Uint32(data[4:8])
	namesLen := binary.LittleEndian.Uint32(data[8:12])
	enc := binary.LittleEndian.Uint32(data[12:16])
	if enc == encAES || enc == encNG {
		return nil, fmt.Errorf("rpf: archive is encrypted (0x%08X) — extract it with OpenIV/CodeWalker", enc)
	}

	tableStart := 16
	tableEnd := tableStart + int(entryCount)*16
	namesEnd := tableEnd + int(namesLen)
	if namesEnd > len(data) {
		return nil, fmt.Errorf("rpf: truncated header (need %d bytes, have %d)", namesEnd, len(data))
	}
	names := data[tableEnd:namesEnd]

	nameAt := func(off uint32) string {
		if int(off) >= len(names) {
			return ""
		}
		end := bytes.IndexByte(names[off:], 0)
		if end < 0 {
			return string(names[off:])
		}
		return string(names[off : int(off)+end])
	}

	ar := &Archive{data: data, entries: make([]entry, entryCount)}
	for i := 0; i < int(entryCount); i++ {
		b := data[tableStart+i*16 : tableStart+i*16+16]
		nameOff := uint32(binary.LittleEndian.Uint16(b[0:2]))
		fileSize := u24(b[2:5])
		fileOff := u24(b[5:8])
		w3 := binary.LittleEndian.Uint32(b[8:12])

		w4 := binary.LittleEndian.Uint32(b[12:16])
		e := entry{name: nameAt(nameOff)}
		switch {
		case fileSize == 0 && fileOff == 0x7FFFFF:
			e.isDir = true
			e.childIndex = w3
			e.childCount = w4
		case fileOff&0x800000 != 0:
			// Resource file: high bit of the offset field is the flag; the
			// last two words are the RSC7 system/graphics flags. Data is the
			// deflate stream, written back with a reconstructed RSC7 header.
			e.isResource = true
			e.offsetBlocks = fileOff & 0x7FFFFF
			e.sizePacked = fileSize
			e.systemFlags = w3
			e.graphicsFlags = w4
			e.deflated = true
		case fileSize == 0:
			// Binary, stored uncompressed; on-disk length is the raw-size word.
			e.offsetBlocks = fileOff
			e.sizePacked = w3
			e.sizeRaw = w3
		default:
			e.offsetBlocks = fileOff
			e.sizePacked = fileSize
			e.sizeRaw = w3
			e.deflated = fileSize != w3
		}
		ar.entries[i] = e
	}
	if entryCount == 0 || !ar.entries[0].isDir {
		return nil, fmt.Errorf("rpf: no root directory")
	}
	return ar, nil
}

// ExtractTo writes the whole archive as a loose tree under destDir. Nested
// unencrypted .rpf entries are recursed into as sub-folders (OpenRPF reads
// them that way); an encrypted nested .rpf is written out as-is.
func ExtractTo(rpfPath, destDir string) (fileCount int, err error) {
	ar, err := Open(rpfPath)
	if err != nil {
		return 0, err
	}
	return ar.extractDir(0, destDir)
}

// NOTE: an in-place "PatchFile" that appended a modified dlclist.xml and
// rewrote one TOC row was tried and abandoned — the game rejected the result
// (ERR_FIL_PACK). RPF7 carries integrity structure this reader doesn't model,
// so writing stays with CodeWalker/OpenIV. This package is read-only.

// ReadInner returns the bytes of one file inside the archive by its
// forward-slash path (e.g. "common/data/dlclist.xml"). Resources come back with
// a reconstructed RSC7 header. Errors if the path isn't found.
func ReadInner(rpfPath, innerPath string) ([]byte, error) {
	ar, err := Open(rpfPath)
	if err != nil {
		return nil, err
	}
	segs := strings.Split(strings.Trim(strings.ReplaceAll(innerPath, "\\", "/"), "/"), "/")
	idx := 0
	for si, seg := range segs {
		e := ar.entries[idx]
		if !e.isDir {
			return nil, fmt.Errorf("rpf: %q is not a directory", strings.Join(segs[:si], "/"))
		}
		found := -1
		for c := e.childIndex; c < e.childIndex+e.childCount; c++ {
			if strings.EqualFold(ar.entries[c].name, seg) {
				found = int(c)
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("rpf: %q not found", innerPath)
		}
		idx = found
	}
	e := ar.entries[idx]
	if e.isDir {
		return nil, fmt.Errorf("rpf: %q is a directory", innerPath)
	}
	raw, err := ar.fileBytes(e)
	if err != nil {
		return nil, err
	}
	if e.isResource {
		raw = prependRSC7(e.name, e.systemFlags, e.graphicsFlags, raw)
	}
	return raw, nil
}

func (ar *Archive) extractDir(idx int, dir string) (int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	e := ar.entries[idx]
	count := 0
	for c := e.childIndex; c < e.childIndex+e.childCount; c++ {
		if int(c) >= len(ar.entries) {
			return count, fmt.Errorf("rpf: child index %d out of range", c)
		}
		child := ar.entries[c]
		if child.isDir {
			n, err := ar.extractDir(int(c), filepath.Join(dir, safeName(child.name)))
			count += n
			if err != nil {
				return count, err
			}
			continue
		}
		raw, err := ar.fileBytes(child)
		if err != nil {
			return count, fmt.Errorf("rpf: %q: %w", child.name, err)
		}
		outPath := filepath.Join(dir, safeName(child.name))

		// Recurse into an unencrypted nested .rpf so OpenRPF sees loose files.
		if strings.EqualFold(path.Ext(child.name), ".rpf") {
			if sub, perr := parse(raw); perr == nil {
				n, serr := sub.extractDir(0, outPath) // outPath becomes a folder named "x.rpf"
				count += n
				if serr != nil {
					return count, serr
				}
				continue
			}
			// encrypted / unparseable: fall through and write the raw .rpf
		}

		if child.isResource {
			raw = prependRSC7(child.name, child.systemFlags, child.graphicsFlags, raw)
		}
		if err := os.WriteFile(outPath, raw, 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (ar *Archive) fileBytes(e entry) ([]byte, error) {
	if e.sizePacked == 0 {
		return nil, nil // empty file
	}
	start := int(e.offsetBlocks) * 512
	end := start + int(e.sizePacked)
	if end > len(ar.data) {
		return nil, fmt.Errorf("data range %d..%d past end %d", start, end, len(ar.data))
	}
	blob := ar.data[start:end]
	// Resource files keep their deflate body as-is; prependRSC7 wraps it so
	// the game inflates it from the header flags (how CodeWalker exports them).
	if e.isResource || !e.deflated {
		return blob, nil
	}
	fr := flate.NewReader(bytes.NewReader(blob))
	defer fr.Close()
	out := make([]byte, 0, e.sizeRaw)
	buf := bytes.NewBuffer(out)
	if _, err := io.Copy(buf, fr); err != nil {
		return nil, fmt.Errorf("inflate: %w", err)
	}
	return buf.Bytes(), nil
}

func u24(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

// prependRSC7 wraps a resource's stored (deflate) body with the 16-byte RSC7
// header CodeWalker writes on extract: "RSC7", version (by extension), and the
// system/graphics flags straight from the RPF entry.
func prependRSC7(name string, sysFlags, gfxFlags uint32, body []byte) []byte {
	ver := resourceVersion[strings.ToLower(path.Ext(name))]
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h[0:4], 0x37435352) // "RSC7"
	binary.LittleEndian.PutUint32(h[4:8], ver)
	binary.LittleEndian.PutUint32(h[8:12], sysFlags)
	binary.LittleEndian.PutUint32(h[12:16], gfxFlags)
	return append(h, body...)
}

// safeName strips path separators / traversal from an archive entry name.
func safeName(n string) string {
	n = strings.ReplaceAll(n, "\\", "/")
	n = strings.TrimLeft(n, "/")
	parts := n
	if i := strings.LastIndexByte(parts, '/'); i >= 0 {
		parts = parts[i+1:]
	}
	if parts == "" || parts == "." || parts == ".." {
		return "_"
	}
	return parts
}
