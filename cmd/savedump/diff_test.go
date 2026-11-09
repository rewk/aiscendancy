package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/sav"
)

// shipRec is the ship record size sav's package comment documents;
// synth checks it against the walker.
const shipRec = 358

// synth lays out a save holding ships only: the header with the record
// sizes sav checks (star 96, planet 123, player 494, lane 39 at 0x78), no
// tech blob, empty arrays but the ships, each named, ids 0, 1, 2…
func synth(t *testing.T, names ...string) []byte {
	ids := make([]int, len(names))
	for i := range ids {
		ids[i] = i
	}
	return synthIDs(t, ids, names...)
}

// synthIDs is synth with each ship's id (u32 at +0) given.
func synthIDs(t *testing.T, ids []int, names ...string) []byte {
	b := make([]byte, 180)
	for i, v := range []uint32{96, 123, 494, 39} {
		binary.LittleEndian.PutUint32(b[0x78+4*i:], v)
	}
	u32 := func(v int) { b = binary.LittleEndian.AppendUint32(b, uint32(v)) }
	u32(0) // stars
	u32(0) // squares
	u32(0) // planets
	u32(len(names))
	for i, n := range names {
		r := make([]byte, shipRec)
		binary.LittleEndian.PutUint32(r, uint32(ids[i]))
		copy(r[sav.ShipName:], n)
		b = append(b, r...)
	}
	u32(0) // players
	u32(0) // lanes
	secs, err := sav.Sections(b)
	if err != nil || secs[5].Name != "ships" || secs[5].Size != shipRec || secs[5].Count != len(names) {
		t.Fatalf("synth: %v %+v", err, secs)
	}
	return b
}

// putF32 writes v at +off of ship record i.
func putF32(b []byte, i, off int, v float32) {
	secs, _ := sav.Sections(b)
	binary.LittleEndian.PutUint32(secs[5].Record(b, i)[off:], math.Float32bits(v))
}

func diffLines(t *testing.T, a, b []byte, only string) []string {
	var w bytes.Buffer
	n, err := diffSaves(&w, a, b, only)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(w.String(), "\n"), "\n")
	if n != len(lines) && !(n == 0 && len(lines) == 1) {
		t.Errorf("%d lines counted, %d written:\n%s", n, len(lines), w.String())
	}
	return lines
}

func TestDiffSame(t *testing.T) {
	a := synth(t, "Scout", "Medium")
	putF32(a, 1, 124, 1056)
	if got := diffLines(t, a, bytes.Clone(a), ""); len(got) != 1 || got[0] != "no difference" {
		t.Errorf("same file: %q", got)
	}
}

// One f32 changed in one ship record: one line naming the record, the
// run's offset and the float's old and new values, aligned (+124, only
// its third byte changes) or packed on 2 bytes (+162).
func TestDiffShipF32(t *testing.T) {
	for _, c := range []struct {
		off      int
		old, new float32
		head     string // the line's start
		f32      string // its float reading
	}{
		{124, 1056, 1152, "ships[id 1:Medium]+126 1  84 → 90 ", "f32@124 1056→1152"},
		{162, 70, 40.0 / 3, "ships[id 1:Medium]+162 4  00008c42 → 55555541 ", "f32@162 70→13.333333"},
	} {
		a := synth(t, "Scout", "Medium", "Large")
		putF32(a, 1, c.off, c.old)
		b := bytes.Clone(a)
		putF32(b, 1, c.off, c.new)
		got := diffLines(t, a, b, "")
		if len(got) != 1 || !strings.HasPrefix(got[0], c.head) || !strings.Contains(got[0], c.f32) {
			t.Errorf("+%d: %q, want one line %q … %q", c.off, got, c.head, c.f32)
		}
		if got := diffLines(t, a, b, "header"); len(got) != 1 || got[0] != "no difference" {
			t.Errorf("-diff-section header: %q", got)
		}
	}
}

// Two bytes changed in one word make one run; in two words, two runs.
func TestDiffRuns(t *testing.T) {
	a := synth(t, "Scout")
	b := bytes.Clone(a)
	rec := func(x []byte) []byte { s, _ := sav.Sections(x); return s[5].Record(x, 0) }
	rec(b)[200], rec(b)[203] = 1, 1
	if got := diffLines(t, a, b, ""); len(got) != 1 || !strings.HasPrefix(got[0], "ships[id 0:Scout]+200 4 ") {
		t.Errorf("one word: %q", got)
	}
	rec(b)[205] = 1
	if got := diffLines(t, a, b, "ships"); len(got) != 2 || !strings.HasPrefix(got[1], "ships[id 0:Scout]+205 1 ") {
		t.Errorf("two words: %q", got)
	}
}

// A ship more in the second save: listed as added.
func TestDiffCount(t *testing.T) {
	a, b := synth(t, "Scout"), synth(t, "Scout", "TEST")
	got := diffLines(t, a, b, "")
	want := []string{"ships[id 1:TEST] added"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%q, want %q", got, want)
	}
}

// A ship inserted mid-array (a reused id) shifts the records after it:
// they are still paired by id, so the one change in the last ship is its
// only line, then the new ship; a ship gone is listed as removed.
func TestDiffShipInserted(t *testing.T) {
	a := synthIDs(t, []int{0, 1, 3}, "Scout", "Medium", "Large")
	b := synthIDs(t, []int{0, 1, 2, 3}, "Scout", "Medium", "TEST", "Large")
	putF32(a, 2, 162, 70)
	putF32(b, 3, 162, 40.0/3)
	got := diffLines(t, a, b, "ships")
	if len(got) != 2 || !strings.HasPrefix(got[0], "ships[id 3:Large]+162 4 ") || got[1] != "ships[id 2:TEST] added" {
		t.Errorf("inserted: %q", got)
	}
	got = diffLines(t, b, a, "ships")
	if len(got) != 2 || !strings.HasPrefix(got[0], "ships[id 3:Large]+162 4 ") || got[1] != "ships[id 2:TEST] removed" {
		t.Errorf("removed: %q", got)
	}
}

// -ships lists the 4-aligned floats and the unaligned one at +162.
func TestShipFloats(t *testing.T) {
	a := synth(t, "Scout")
	putF32(a, 0, 124, 1056)
	putF32(a, 0, 162, 40.0/3)
	var w bytes.Buffer
	if err := shipFloats(&w, a, 5000); err != nil {
		t.Fatal(err)
	}
	if got := w.String(); !strings.Contains(got, "Scout") || !strings.HasSuffix(got, " +124 1056 +162 13.333333\n") {
		t.Errorf("%q", got)
	}
}
