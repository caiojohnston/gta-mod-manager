package rpf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// errTooBig reports a resource body that overflows the RPF7 u24 on-disk size
// field. Plain binary entries dodge this (their length lives in the u32 size
// word) but a resource's packed size must fit 24 bits.
func errTooBig(rel string, n int) error {
	return fmt.Errorf("rpf: resource %q is %d bytes, over the 16 MB RPF7 entry limit", rel, n)
}

// Pack builds a fresh RPF7 "OpenFormats" archive from the file tree under
// srcDir and writes it to outPath. Every file is stored uncompressed as a
// plain binary entry — no deflate, no resource (RSC7) handling — which is all
// the merged-dlcpack case needs (XML/text metas plus already-packed nested
// .rpf files). The result is a valid unencrypted RPF7 that OpenRPF/the game
// mount as a dlcpack.
func Pack(srcDir, outPath string) error {
	type packEntry struct {
		name       string
		isDir      bool
		data       []byte // files only (raw bytes to write to the data section)
		childIndex uint32 // dirs only
		childCount uint32
		blockOff   uint32 // files only, data offset in 512-byte blocks
		// resource files: an RSC7-wrapped input is unwrapped so the deflate
		// body goes to the data section and these flags to the entry row.
		isResource bool
		sysFlags   uint32
		gfxFlags   uint32
	}

	// --- read the tree, sorted parents-before-children, siblings grouped ---
	type raw struct {
		rel   string
		isDir bool
		data  []byte
	}
	var items []raw
	err := filepath.Walk(srcDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil || rel == "." {
			return err
		}
		r := raw{rel: filepath.ToSlash(rel), isDir: info.IsDir()}
		if !info.IsDir() {
			if r.data, err = os.ReadFile(p); err != nil {
				return err
			}
		}
		items = append(items, r)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool {
		di, dj := strings.Count(items[i].rel, "/"), strings.Count(items[j].rel, "/")
		if di != dj {
			return di < dj
		}
		return items[i].rel < items[j].rel
	})

	// --- build entry list with contiguous child ranges (BFS) ---
	entries := []packEntry{{name: "", isDir: true}}
	kids := map[string][]int{} // parent rel -> child entry indexes (temp)
	relIndex := map[string]int{"": 0}
	for _, it := range items {
		parent := ""
		if k := strings.LastIndexByte(it.rel, '/'); k >= 0 {
			parent = it.rel[:k]
		}
		if _, ok := relIndex[parent]; !ok {
			continue
		}
		e := packEntry{name: baseName(it.rel), isDir: it.isDir, data: it.data}
		// An RSC7-wrapped file becomes a resource entry: strip the 16-byte
		// header, keep the deflate body, carry the flags on the entry row.
		if !it.isDir && len(it.data) >= 16 && string(it.data[0:4]) == "RSC7" {
			e.isResource = true
			e.sysFlags = binary.LittleEndian.Uint32(it.data[8:12])
			e.gfxFlags = binary.LittleEndian.Uint32(it.data[12:16])
			e.data = it.data[16:]
		}
		if e.isResource && len(e.data) > 0xFFFFFF {
			return errTooBig(it.rel, len(e.data))
		}
		idx := len(entries)
		entries = append(entries, e)
		kids[parent] = append(kids[parent], idx)
		if it.isDir {
			relIndex[it.rel] = idx
		}
	}
	// relByIndex for kids lookup
	idxRel := make([]string, len(entries))
	for r, i := range relIndex {
		idxRel[i] = r
	}
	// BFS renumber so each dir's children are one contiguous block
	final := []packEntry{entries[0]}
	queue := []int{0}
	oldToNew := map[int]int{0: 0}
	for qi := 0; qi < len(queue); qi++ {
		curOld := queue[qi]
		start := uint32(len(final))
		cs := kids[idxRel[curOld]]
		for _, c := range cs {
			oldToNew[c] = len(final)
			final = append(final, entries[c])
			if entries[c].isDir {
				queue = append(queue, c) // only dirs get expanded
			}
		}
		final[oldToNew[curOld]].childIndex = start
		final[oldToNew[curOld]].childCount = uint32(len(cs))
	}

	// --- names block ---
	var names bytes.Buffer
	names.WriteByte(0) // root name at offset 0
	nameOff := make([]uint32, len(final))
	for i := 1; i < len(final); i++ {
		nameOff[i] = uint32(names.Len())
		names.WriteString(final[i].name)
		names.WriteByte(0)
	}
	for names.Len()%16 != 0 {
		names.WriteByte(0)
	}

	headerArea := 16 + 16*len(final) + names.Len()
	dataStart := (headerArea + 511) / 512 * 512

	// --- data section + file offsets ---
	var data bytes.Buffer
	for i := range final {
		if final[i].isDir {
			continue
		}
		final[i].blockOff = uint32((dataStart + data.Len()) / 512)
		data.Write(final[i].data)
		for data.Len()%512 != 0 {
			data.WriteByte(0)
		}
	}

	// --- entry table ---
	tbl := make([]byte, 16*len(final))
	for i, e := range final {
		row := tbl[i*16 : i*16+16]
		binary.LittleEndian.PutUint16(row[0:2], uint16(nameOff[i]))
		if e.isDir {
			row[5], row[6], row[7] = 0xFF, 0xFF, 0x7F // fileOffset = 0x7FFFFF marks a directory
			binary.LittleEndian.PutUint32(row[8:12], e.childIndex)
			binary.LittleEndian.PutUint32(row[12:16], e.childCount)
		} else if e.isResource {
			sz := uint32(len(e.data)) // deflate body length
			off := e.blockOff | 0x800000
			row[2], row[3], row[4] = byte(sz), byte(sz>>8), byte(sz>>16)
			row[5], row[6], row[7] = byte(off), byte(off>>8), byte(off>>16)
			binary.LittleEndian.PutUint32(row[8:12], e.sysFlags)
			binary.LittleEndian.PutUint32(row[12:16], e.gfxFlags)
		} else {
			// Uncompressed binary: leave the u24 "fileSize" at 0 and put the
			// real on-disk length in the u32 size word, so there is no 16 MB
			// ceiling — a nested vehicles.rpf is routinely larger.
			sz := uint32(len(e.data))
			row[2], row[3], row[4] = 0, 0, 0
			row[5], row[6], row[7] = byte(e.blockOff), byte(e.blockOff>>8), byte(e.blockOff>>16)
			binary.LittleEndian.PutUint32(row[8:12], sz)
			binary.LittleEndian.PutUint32(row[12:16], 0)
		}
	}

	// --- assemble ---
	var out bytes.Buffer
	hdr := make([]byte, 16)
	binary.LittleEndian.PutUint32(hdr[0:4], magic7)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(final)))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(names.Len()))
	binary.LittleEndian.PutUint32(hdr[12:16], encOpen)
	out.Write(hdr)
	out.Write(tbl)
	out.Write(names.Bytes())
	for out.Len() < dataStart {
		out.WriteByte(0)
	}
	out.Write(data.Bytes())

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, out.Bytes(), 0o644)
}

func baseName(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[i+1:]
	}
	return rel
}
