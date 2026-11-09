package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"

	"github.com/rewk/aiscendancy/sav"
)

// diffHexMax caps the hex shown for one run, in bytes per side.
const diffHexMax = 32

// diffSaves writes the byte differences between saves a and b, section by
// section in file order (sav.Sections), and returns the lines written;
// only, when not "", keeps that section alone.
//
// Records of an array are compared by index. A changed run is a stretch of
// differing bytes within one record; two runs touching the same 4-byte word
// (record-relative) are one, so a float's scattered byte changes make one
// line. Arrays of different counts are compared over their common records,
// then the extra ones listed (ships are paired by id, diffShips); blobs
// of different lengths (the rest) over their common prefix only: nothing
// realigns a shifted tail.
func diffSaves(w io.Writer, a, b []byte, only string) (int, error) {
	sa, err := sav.Sections(a)
	if err != nil {
		return 0, fmt.Errorf("first save: %w", err)
	}
	sb, err := sav.Sections(b)
	if err != nil {
		return 0, fmt.Errorf("second save: %w", err)
	}
	lines := 0
	for k, x := range sa {
		y := sb[k] // the walker always yields the same sections in order
		if only != "" && x.Name != only {
			continue
		}
		blob := x.Count == 1 && (x.Name == "header" || x.Name == "tech" || x.Name == "rest")
		switch {
		case blob && x.Size != y.Size:
			fmt.Fprintf(w, "%s: %d bytes → %d, the common prefix compared\n", x.Name, x.Size, y.Size)
			lines++
			n := min(x.Size, y.Size)
			lines += diffRecord(w, x.Name, "", a[x.Off:x.Off+n], b[y.Off:y.Off+n])
			continue
		case blob:
			lines += diffRecord(w, x.Name, "", x.Record(a, 0), y.Record(b, 0))
			continue
		}
		if x.Name == "ships" {
			lines += diffShips(w, a, b, x, y)
			continue
		}
		if x.Count != y.Count {
			fmt.Fprintf(w, "%s: %d records → %d, the first %d compared by index\n", x.Name, x.Count, y.Count, min(x.Count, y.Count))
			lines++
		}
		for i := range min(x.Count, y.Count) {
			ra, rb := x.Record(a, i), y.Record(b, i)
			lines += diffRecord(w, x.Name, recordTag(x.Name, i, ra, rb), ra, rb)
		}
		for i := y.Count; i < x.Count; i++ {
			fmt.Fprintf(w, "%s only in the first\n", recordTag(x.Name, i, x.Record(a, i), nil))
			lines++
		}
		for i := x.Count; i < y.Count; i++ {
			fmt.Fprintf(w, "%s only in the second\n", recordTag(y.Name, i, nil, y.Record(b, i)))
			lines++
		}
	}
	if lines == 0 {
		fmt.Fprintln(w, "no difference")
	}
	return lines, nil
}

// diffShips pairs the ship records by their id (u32 at +0): the records
// are sorted by it and a freed id is reused, so a ship built between two
// saves lands mid-array and an index comparison would misalign the rest.
// Pairs are diffed in the first file's order, then the ids found in one
// file only are listed.
func diffShips(w io.Writer, a, b []byte, x, y sav.Section) int {
	id := func(r []byte) uint32 { return binary.LittleEndian.Uint32(r) }
	inB := map[uint32]int{}
	for i := range y.Count {
		inB[id(y.Record(b, i))] = i
	}
	lines, paired := 0, map[uint32]bool{}
	var gone []string
	for i := range x.Count {
		ra := x.Record(a, i)
		k := id(ra)
		j, ok := inB[k]
		if !ok {
			gone = append(gone, shipTag(k, ra, nil)+" removed")
			continue
		}
		paired[k] = true
		rb := y.Record(b, j)
		lines += diffRecord(w, "ships", shipTag(k, ra, rb), ra, rb)
	}
	for i := range y.Count {
		if rb := y.Record(b, i); !paired[id(rb)] {
			gone = append(gone, shipTag(id(rb), nil, rb)+" added")
		}
	}
	for _, l := range gone {
		fmt.Fprintln(w, l)
	}
	return lines + len(gone)
}

// shipTag is "ships[id N:label]", the label as recordTag reads it.
func shipTag(id uint32, ra, rb []byte) string {
	l := ""
	if ra != nil {
		l = sav.Label("ships", ra)
	}
	if l == "" && rb != nil {
		l = sav.Label("ships", rb)
	}
	return fmt.Sprintf("ships[id %d:%s]", id, l)
}

// recordTag is "name[i]" or "name[i:label]", the label read from the
// first record given, else the second (a renamed ship shows its old name).
func recordTag(name string, i int, ra, rb []byte) string {
	l := ""
	if ra != nil {
		l = sav.Label(name, ra)
	}
	if l == "" && rb != nil {
		l = sav.Label(name, rb)
	}
	if l != "" {
		return fmt.Sprintf("%s[%d:%s]", name, i, l)
	}
	return fmt.Sprintf("%s[%d]", name, i)
}

// diffRecord writes one line per changed run of two equal-length records;
// tag "" means a blob, named by section alone.
func diffRecord(w io.Writer, section, tag string, ra, rb []byte) int {
	if tag == "" {
		tag = section
	}
	lines := 0
	for i := 0; i < len(ra); {
		if ra[i] == rb[i] {
			i++
			continue
		}
		start, end := i, i+1 // end: past the last differing byte
		for j := end; j < len(ra); j++ {
			if j != end && j/4 != (end-1)/4 { // past a gap and out of the last word
				break
			}
			if ra[j] != rb[j] {
				end = j + 1
			}
		}
		fmt.Fprintf(w, "%s+%d %d  %s → %s  %s\n", tag, start, end-start,
			hexCap(ra[start:end]), hexCap(rb[start:end]), interpret(ra, rb, start))
		lines++
		i = end
	}
	return lines
}

func hexCap(p []byte) string {
	if len(p) > diffHexMax {
		return hex.EncodeToString(p[:diffHexMax]) + fmt.Sprintf("…(+%d)", len(p)-diffHexMax)
	}
	return hex.EncodeToString(p)
}

// interpret reads the run's start as u8, then as u16/i16 at the start
// aligned to 2 and u32/i32/f32 aligned to 4 (record-relative), "@off" when
// the aligned offset is not the start; a width past the record is left out.
func interpret(ra, rb []byte, start int) string {
	s := fmt.Sprintf("u8 %d→%d", ra[start], rb[start])
	at := func(o int) string {
		if o == start {
			return ""
		}
		return fmt.Sprintf("@%d", o)
	}
	if o := start &^ 1; o+2 <= len(ra) {
		x, y := binary.LittleEndian.Uint16(ra[o:]), binary.LittleEndian.Uint16(rb[o:])
		s += fmt.Sprintf("  u16%s %d→%d  i16 %d→%d", at(o), x, y, int16(x), int16(y))
	}
	if o := start &^ 3; o+4 <= len(ra) {
		x, y := binary.LittleEndian.Uint32(ra[o:]), binary.LittleEndian.Uint32(rb[o:])
		s += fmt.Sprintf("  u32%s %d→%d  i32 %d→%d  f32%s %g→%g", at(o), x, y, int32(x), int32(y), at(o),
			math.Float32frombits(x), math.Float32frombits(y))
	}
	// the records are packed: a ship's f32 at +162 sits on a 2-byte boundary
	if o := start &^ 1; o&3 != 0 && o+4 <= len(ra) {
		s += fmt.Sprintf("  f32@%d %g→%g", o, math.Float32frombits(binary.LittleEndian.Uint32(ra[o:])),
			math.Float32frombits(binary.LittleEndian.Uint32(rb[o:])))
	}
	return s
}

// energyAt is the ship record's unaligned f32 read as the energy
// candidate (sav's package comment).
const energyAt = 162

// shipFloats writes one line per ship record: its index, its name and the
// f32 at each 4-aligned offset, plus energyAt, whose magnitude is in
// [1e-3, fmax]: the candidates for a position and the energy.
func shipFloats(w io.Writer, b []byte, fmax float64) error {
	secs, err := sav.Sections(b)
	if err != nil {
		return err
	}
	for _, s := range secs {
		if s.Name != "ships" {
			continue
		}
		fmt.Fprintf(w, "%d ships\n", s.Count)
		for i := range s.Count {
			r := s.Record(b, i)
			fmt.Fprintf(w, "%3d %-16s", i, sav.Label("ships", r))
			for o := 0; o+4 <= len(r); o += 2 {
				if o%4 != 0 && o != energyAt {
					continue
				}
				v := math.Float32frombits(binary.LittleEndian.Uint32(r[o:]))
				if a := math.Abs(float64(v)); a >= 1e-3 && a <= fmax {
					fmt.Fprintf(w, " +%d %g", o, v)
				}
			}
			fmt.Fprintln(w)
		}
	}
	return nil
}
