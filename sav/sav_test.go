package sav

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// build lays out a minimal save the way Decode documents it: no tech
// blob, no squares, no ships, no players. pos are the stars, lanes their
// pairs, planets the planet count per star.
func build(pos [][3]float32, lanes []Lane, planets []int) []byte {
	return buildShips(pos, lanes, planets, 0, nil)
}

// buildShips is build with a tech blob of tech zero bytes and the given
// ship records (shipSize each).
func buildShips(pos [][3]float32, lanes []Lane, planets []int, tech int, ships [][]byte) []byte {
	b := make([]byte, headerSize+tech)
	for i, v := range []uint32{starSize, planetSize, playerSize, laneSize} {
		binary.LittleEndian.PutUint32(b[0x78+4*i:], uint32(v))
	}
	binary.LittleEndian.PutUint32(b[0x74:], uint32(tech))
	u32 := func(v int) { b = binary.LittleEndian.AppendUint32(b, uint32(v)) }
	const laneBase = 0x25d2a3
	u32(len(pos))
	for i, p := range pos {
		r := make([]byte, starSize)
		binary.LittleEndian.PutUint32(r[4:], uint32(i))
		for k := 0; k < 3; k++ {
			binary.LittleEndian.PutUint32(r[8+4*k:], math.Float32bits(p[k]))
		}
		copy(r[28:], "Star"+string(rune('A'+i)))
		n := 0
		for k, l := range lanes {
			if l.A == i || l.B == i {
				binary.LittleEndian.PutUint32(r[44+4*n:], uint32(laneBase+laneSize*k))
				n++
			}
		}
		binary.LittleEndian.PutUint16(r[68:], uint16(n))
		binary.LittleEndian.PutUint16(r[90:], uint16(planets[i]))
		b = append(b, r...)
	}
	u32(0) // squares
	total := 0
	for _, n := range planets {
		total += n
	}
	u32(total)
	for i, n := range planets {
		for j := 0; j < n; j++ {
			r := make([]byte, planetSize)
			binary.LittleEndian.PutUint16(r[12:], uint16(i))
			binary.LittleEndian.PutUint16(r[20:], 2)
			binary.LittleEndian.PutUint16(r[22:], uint16(j))
			copy(r[36:], "P")
			b = append(b, r...)
		}
	}
	u32(len(ships))
	for _, r := range ships {
		b = append(b, r...)
	}
	u32(0) // players
	u32(len(lanes))
	for _, l := range lanes {
		r := make([]byte, laneSize)
		binary.LittleEndian.PutUint32(r[0:], uint32(l.A))
		binary.LittleEndian.PutUint32(r[4:], uint32(l.B))
		if l.Red {
			r[35] = 1
		}
		b = append(b, r...)
	}
	return b
}

// A hand-made galaxy: a chain 0-1-2-3 off a hub 3 with two more spokes
// 4 and 5, and a red lane 4-5; star 5 off the axis at (3,2,0).
func handMade() ([][3]float32, []Lane) {
	pos := [][3]float32{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}, {4, 0, 0}, {3, 2, 0}}
	lanes := []Lane{{0, 1, false}, {1, 2, false}, {2, 3, false}, {3, 4, false}, {3, 5, false}, {4, 5, true}}
	return pos, lanes
}

func TestDecodeHandMade(t *testing.T) {
	pos, lanes := handMade()
	g, err := Decode(build(pos, lanes, []int{1, 0, 2, 0, 0, 3}))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Stars) != 6 || len(g.Lanes) != 6 || len(g.Planets) != 6 {
		t.Fatalf("%d stars %d lanes %d planets", len(g.Stars), len(g.Lanes), len(g.Planets))
	}
	for k, l := range g.Lanes {
		if l != lanes[k] {
			t.Errorf("lane %d = %v, want %v", k, l, lanes[k])
		}
	}
	if p := g.Planets[1]; p.Star != 2 || p.Size != 3 || p.Type != 0 || p.Name != "P" {
		t.Errorf("planet 1 = %+v", p)
	}
	if s := g.Stars[5]; s.Name != "StarF" || s.Pos != pos[5] || s.PlanetCount != 3 {
		t.Errorf("star 5 = %+v", s)
	}

	s := g.Stats()
	if s.Red != 1 || !s.Connected || s.LanesPerStar != 2 {
		t.Errorf("red %d connected %v l/s %v", s.Red, s.Connected, s.LanesPerStar)
	}
	// degrees: 0:1 1:2 2:2 3:3 4:2 5:2
	if s.Degree[1] != 1 || s.Degree[2] != 4 || s.Degree[3] != 1 {
		t.Errorf("degrees %v", s.Degree)
	}
	if s.LongestChain != 3 || math.Abs(s.DeadEnds-1.0/6) > 1e-9 {
		t.Errorf("chain %d dead %v", s.LongestChain, s.DeadEnds)
	}
	// 0 to 4 or 5: 0-1-2-3-4, four hops
	if s.Diameter != 4 {
		t.Errorf("diameter %d", s.Diameter)
	}
	if s.BlueLen != (1+1+1+1+2)/5.0 || math.Abs(s.RedLen-math.Sqrt(5)) > 1e-6 {
		t.Errorf("blue %v red %v", s.BlueLen, s.RedLen)
	}
	// one blue island, the red inside it is no bridge; every nearest
	// neighbour (ties to the lowest index) is a blue partner
	if s.Islands != 1 || s.NNBlue != 1 || s.RedBridge != 0 {
		t.Errorf("islands %d nnblue %v bridge %v", s.Islands, s.NNBlue, s.RedBridge)
	}
}

// The hand-made galaxy cut in two blue islands {0,1,2} {3,4,5}, bridged
// by a red 0-3 three times the islands' closest pair (2-3, length 1); the
// red 4-5 inside an island is not counted. Star 3's nearest neighbour is
// 2 (tied with 4, lowest index), across the cut: NNBlue 5/6.
func TestStatsIslands(t *testing.T) {
	pos, _ := handMade()
	lanes := []Lane{{0, 1, false}, {1, 2, false}, {3, 4, false}, {3, 5, false}, {0, 3, true}, {4, 5, true}}
	g, err := Decode(build(pos, lanes, []int{1, 1, 1, 1, 1, 1}))
	if err != nil {
		t.Fatal(err)
	}
	s := g.Stats()
	if s.Islands != 2 || math.Abs(s.NNBlue-5.0/6) > 1e-9 || math.Abs(s.RedBridge-3) > 1e-6 || !s.Connected {
		t.Errorf("islands %d nnblue %v bridge %v connected %v", s.Islands, s.NNBlue, s.RedBridge, s.Connected)
	}
	// 2 to 4 or 5: 2-1-0-3-4, the red 0-3 counted as one hop
	if s.Diameter != 4 {
		t.Errorf("diameter %d", s.Diameter)
	}
}

func TestDecodeRejects(t *testing.T) {
	pos, lanes := handMade()
	b := build(pos, lanes, []int{1, 0, 2, 0, 0, 3})
	// a lane whose ends disagree with the stars' lane pointers
	o := len(b) - laneSize
	binary.LittleEndian.PutUint32(b[o:], 0)
	if _, err := Decode(b); err == nil {
		t.Error("a lane 0-5 that stars 4 and 5 point at, decoded")
	}
	// a disconnected galaxy is decoded, and said so
	g, err := Decode(build(pos, lanes[:4], []int{1, 0, 2, 0, 0, 3}))
	if err != nil {
		t.Fatal(err)
	}
	if s := g.Stats(); s.Connected || s.Diameter != -1 {
		t.Errorf("star 5 has no lane, yet connected %v, diameter %d", s.Connected, s.Diameter)
	}
	if _, err := Decode(b[:200]); err == nil {
		t.Error("a truncated file decoded")
	}
}

// testSave opens testdata/name, one of the original's saves (01-18) the
// fixtures were measured on.
func testSave(t *testing.T, name string) *Game {
	g, err := Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSave07(t *testing.T) {
	g := testSave(t, "07.SAV")
	if len(g.Stars) != 50 {
		t.Fatalf("%d stars", len(g.Stars))
	}
	sum := 0
	for i, s := range g.Stars {
		sum += s.PlanetCount
		if s.Type < 0 || s.Type > 12 || s.PlanetCount < 1 || s.PlanetCount > 5 {
			t.Errorf("star %d %+v", i, s)
		}
	}
	if sum != len(g.Planets) {
		t.Errorf("planet counts sum to %d, %d planet records", sum, len(g.Planets))
	}
	// the planets are grouped by star, in star order, as the counts say
	k := 0
	for i, s := range g.Stars {
		for j := 0; j < s.PlanetCount && k < len(g.Planets); j++ {
			if g.Planets[k].Star != i {
				t.Fatalf("planet %d %q: star %d, want %d", k, g.Planets[k].Name, g.Planets[k].Star, i)
			}
			k++
		}
	}
	// No name pinned: the live slots are rewritten as the game is played.
	if g.Planets[0].Name != g.Stars[0].Name+" I" {
		t.Errorf("first star %q, first planet %q", g.Stars[0].Name, g.Planets[0].Name)
	}
	s := g.Stats()
	if !s.Connected {
		t.Error("lane graph not connected")
	}
	if s.LanesPerStar < 2.0 || s.LanesPerStar > 2.6 {
		t.Errorf("lanes per star %.2f", s.LanesPerStar)
	}
	if s.Red == 0 || s.RedShare > 0.25 || s.RedLen < 1.5*s.BlueLen {
		t.Errorf("red %d (%.2f), red %.1f blue %.1f", s.Red, s.RedShare, s.RedLen, s.BlueLen)
	}
	if s.DeadEnds < 0.05 || s.DeadEnds > 0.4 {
		t.Errorf("dead ends %.2f", s.DeadEnds)
	}
	seen := map[[2]int]bool{}
	for _, l := range g.Lanes {
		key := [2]int{min(l.A, l.B), max(l.A, l.B)}
		if seen[key] {
			t.Errorf("lane %v twice", key)
		}
		seen[key] = true
	}
}

// The population capacity rule found in the saves (docs/rules.md): a
// planet with nothing on its squares holds 5 + N/3, N its non-black
// surface squares, on every bare planet of every save at hand.
func TestBareCapacity(t *testing.T) {
	names := []string{"01.sav", "02.SAV", "03.SAV", "04.SAV", "05.SAV", "06.SAV", "07.SAV", "08.SAV", "09.SAV", "10.SAV", "11.SAV"}
	bare := 0
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			g := testSave(t, name)
			if name == "03.SAV" && g.Day != 1466 {
				t.Errorf("day %d, want 1466", g.Day)
			}
			for _, p := range g.Planets {
				empty, n := true, 0
				for _, q := range p.Squares {
					empty = empty && q.Item == 0xff
				}
				for _, q := range p.Surface() {
					if !q.Black() {
						n++
					}
				}
				if len(p.Surface()) != []int{7, 11, 23, 45, 73}[p.Size-1] {
					t.Fatalf("%s size %d: %d surface squares", p.Name, p.Size, len(p.Surface()))
				}
				if !empty {
					continue
				}
				bare++
				if want := 5 + n/3; p.Capacity != want {
					t.Errorf("%s (%d not black): capacity %d, want %d", p.Name, n, p.Capacity, want)
				}
			}
		})
	}
	t.Logf("%d bare planets", bare)
}

// Xeno ruins (codes 8-12) sit on at most one surface square of a planet,
// never in an orbit slot, in every save at hand; some planet has them.
func TestRuinsOnePerPlanet(t *testing.T) {
	names := []string{"01.sav", "02.SAV", "03.SAV", "04.SAV", "05.SAV", "06.SAV", "07.SAV", "08.SAV", "09.SAV", "10.SAV", "11.SAV"}
	read, with := 0, 0
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			g := testSave(t, name)
			read++
			for _, p := range g.Planets {
				n := 0
				for _, q := range p.Surface() {
					if q.Ruins() {
						n++
					}
				}
				for _, q := range p.Squares[len(p.Surface()):] {
					if q.Ruins() {
						t.Errorf("%s: ruins in orbit", p.Name)
					}
				}
				if n > 1 {
					t.Errorf("%s: %d ruin squares", p.Name, n)
				}
				if n == 1 {
					with++
				}
			}
		})
	}
	if read > 0 && with == 0 {
		t.Error("no planet with ruins")
	}
	t.Logf("%d planets with ruins", with)
}

// Sections lays the file out back to back: each array's offset is the
// previous section's end plus its u32 count, the rest runs to the end.
func TestSections(t *testing.T) {
	pos, lanes := handMade()
	ship := make([]byte, shipSize)
	copy(ship[ShipName:], "Scout\x00junk")
	b := append(buildShips(pos, lanes, []int{1, 0, 2, 0, 0, 0}, 10, [][]byte{ship, make([]byte, shipSize)}), "tail"...)
	secs, err := Sections(b)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name        string
		size, count int
	}{
		{"header", headerSize, 1}, {"tech", 10, 1}, {"stars", starSize, 6}, {"squares", 4, 0},
		{"planets", planetSize, 3}, {"ships", shipSize, 2}, {"players", playerSize, 0},
		{"lanes", laneSize, len(lanes)}, {"rest", 4, 1},
	}
	if len(secs) != len(want) {
		t.Fatalf("%d sections: %+v", len(secs), secs)
	}
	end := 0
	for i, w := range want {
		s := secs[i]
		if s.Name != w.name || s.Size != w.size || s.Count != w.count {
			t.Errorf("section %d %+v, want %+v", i, s, w)
		}
		gap := 4 // the array's count
		if s.Count == 1 && (s.Name == "header" || s.Name == "tech" || s.Name == "rest") {
			gap = 0
		}
		if s.Off != end+gap {
			t.Errorf("%s at %d, want %d", s.Name, s.Off, end+gap)
		}
		end = s.End()
	}
	if end != len(b) || string(secs[8].Record(b, 0)) != "tail" {
		t.Errorf("sections end at %d of %d", end, len(b))
	}
	sh := secs[5]
	if got := Label("ships", sh.Record(b, 0)); got != "Scout" {
		t.Errorf("ship label %q", got)
	}
	if got := Label("ships", sh.Record(b, 1)); got != "" {
		t.Errorf("empty ship label %q", got)
	}
	if got := Label("stars", secs[2].Record(b, 1)); got != "StarB" {
		t.Errorf("star label %q", got)
	}
	if got := Label("lanes", secs[7].Record(b, 0)); got != "" {
		t.Errorf("lane label %q", got)
	}
	if _, err := Decode(b); err != nil {
		t.Errorf("Decode on the walker's file: %v", err)
	}
	if _, err := Sections(b[:headerSize+5]); err == nil {
		t.Error("a file cut inside the tech blob walked")
	}
}

// On a real save the walker covers the whole file and every ship record
// carries a printable name at ShipName.
func TestSectionsSave09(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "09.SAV"))
	if err != nil {
		t.Fatal(err)
	}
	secs, err := Sections(b)
	if err != nil {
		t.Fatal(err)
	}
	if last := secs[len(secs)-1]; last.Name != "rest" || last.End() != len(b) {
		t.Errorf("last section %+v, file %d", last, len(b))
	}
	sh := secs[5]
	if sh.Name != "ships" || sh.Count == 0 {
		t.Fatalf("ships %+v", sh)
	}
	for i := range sh.Count {
		if Label("ships", sh.Record(b, i)) == "" {
			t.Errorf("ship %d: no name at +%d", i, ShipName)
		}
	}
}
