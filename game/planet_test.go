package game

import (
	"bufio"
	"bytes"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

// planitemTxt reads the game's planitem.txt, or skips without the game.
func planitemTxt(t *testing.T) []byte {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	b, err := g.Read("planitem.txt")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// numbers splits a line into ints.
func numbers(t *testing.T, line string) []int {
	t.Helper()
	var out []int
	for _, f := range strings.Fields(line) {
		n, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		out = append(out, n)
	}
	return out
}

// The file after its '#' line: a pop growth number, 11 mix rows, two 5x15
// tables, the "count surface orbital" line, then name/row pairs.
func TestTablesMatchGame(t *testing.T) {
	txt := planitemTxt(t)
	_, body, ok := bytes.Cut(txt, []byte("\n#"))
	if !ok {
		t.Fatal("no # line")
	}
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" && !strings.HasPrefix(l, "//") {
			lines = append(lines, l)
		}
	}
	k := 1 // lines[0] is the pop growth amount
	for typ := PlanetType(0); typ < NumPlanetTypes; typ++ {
		got := numbers(t, lines[k])
		k++
		if Mix(got) != Mixes[typ] {
			t.Errorf("%s mix: game %v, typed %v", typ, got, Mixes[typ])
		}
	}
	for s := 0; s < MaxSize; s++ {
		got := numbers(t, lines[k])
		k++
		if len(got) != 15 || !slices.Equal(got[:GridSide], layoutFirst[s][:]) {
			t.Errorf("size %d first: game %v, typed %v", s+1, got, layoutFirst[s])
		}
	}
	for s := 0; s < MaxSize; s++ {
		got := numbers(t, lines[k])
		k++
		if !slices.Equal(got[:GridSide], layoutCount[s][:]) {
			t.Errorf("size %d count: game %v, typed %v", s+1, got, layoutCount[s])
		}
	}
	counts := numbers(t, lines[k])
	k++
	if counts[0] != int(NumItems) || counts[1] != int(FirstOrbital) || counts[2] != int(FirstProject-FirstOrbital) {
		t.Errorf("item counts: game %v, typed %d %d %d", counts, NumItems, FirstOrbital, FirstProject-FirstOrbital)
	}
	for it := Item(0); it < NumItems; it++ {
		name := strings.ReplaceAll(lines[k], "^", " ")
		row := numbers(t, lines[k+1])
		k += 2
		in := Items[it]
		if name != in.Name {
			t.Errorf("item %d: game %q, typed %q", it, name, in.Name)
		}
		want := []int{in.Industry, in.Research, in.Prosp, in.MaxPop, in.UsePop, in.Cost, in.ResearchReq}
		want = append(want, in.Flags...)
		want = append(want, 255)
		if !slices.Equal(row, want) {
			t.Errorf("%s: game %v, typed %v", name, row, want)
		}
	}
	if k != len(lines) {
		t.Errorf("%d lines left over after the items", len(lines)-k)
	}
}

func TestLayoutIsADisc(t *testing.T) {
	want := [MaxSize + 1]int{0, 7, 11, 23, 45, 73}
	for size := MinSize; size <= MaxSize; size++ {
		cells := Layout(size)
		if len(cells) != want[size] {
			t.Errorf("size %d: %d cells, want %d", size, len(cells), want[size])
		}
		// Isometric screen position with a 68x34 diamond: every cell sits
		// inside a circle whose radius grows with the size, and the centre
		// cell is always (5,5).
		seen := map[Cell]bool{}
		centre := false
		for _, c := range cells {
			x := float64(c.I-c.J) * 34
			y := float64(c.I+c.J-2*Centre) * 17
			if r := x*x + y*y; r > float64((size*34+30)*(size*34+30)) {
				t.Errorf("size %d: cell %v is %.0f px out", size, c, r)
			}
			if seen[c] {
				t.Errorf("size %d: cell %v twice", size, c)
			}
			seen[c] = true
			centre = centre || c == Cell{Centre, Centre}
		}
		if !centre {
			t.Errorf("size %d: no centre cell", size)
		}
		for i := 1; i < len(cells); i++ {
			a, b := cells[i-1], cells[i]
			if a.J > b.J || (a.J == b.J && a.I >= b.I) {
				t.Errorf("size %d: %v before %v is not back to front", size, a, b)
			}
		}
	}
}

func TestMixRolls(t *testing.T) {
	// A husk is all black; a cornucopia has no black and no white.
	for _, sq := range NewPlanet(1, "h", Husk, 5).Surface {
		if sq.Kind != Black {
			t.Fatalf("husk square %v", sq.Kind)
		}
	}
	for _, sq := range NewPlanet(1, "c", Cornucopia, 5).Surface {
		if sq.Kind == Black || sq.Kind == White {
			t.Fatalf("cornucopia square %v", sq.Kind)
		}
	}
	// Over many rolls an eden comes out near its 20% prosperity.
	n, pros := 0, 0
	for seed := uint64(0); seed < 200; seed++ {
		for _, sq := range NewPlanet(seed, "e", Eden, 5).Surface {
			n++
			if sq.Kind == BonusProsperity {
				pros++
			}
		}
	}
	if pct := 100 * pros / n; pct < 17 || pct > 23 {
		t.Errorf("eden prosperity squares: %d%%", pct)
	}
	// Same seed, same planet.
	a, b := NewPlanet(7, "a", SpecialPlanet, 4), NewPlanet(7, "b", SpecialPlanet, 4)
	for c, sq := range a.Surface {
		if b.Surface[c].Kind != sq.Kind {
			t.Fatal("seed 7 rolled twice differently")
		}
	}
}

func TestBuildRules(t *testing.T) {
	p := NewHomePlanet(3, "Home")
	c := Cell{Centre, Centre}
	if p.Surface[c].Item != ColonyBase || p.Surface[c].Kind != White {
		t.Fatalf("home centre: %+v", p.Surface[c])
	}
	if p.CanBuild(c, Factory, Base) {
		t.Error("built over the colony base")
	}
	if p.CanBuild(Cell{0, 0}, Factory, Base) {
		t.Error("built off the planet")
	}
	if p.CanBuild(c, Shipyard, Base) {
		t.Error("orbital item on the surface")
	}
	// A black square takes tubes, nothing else, not even the Outpost the
	// data flags (the original refuses it); the Orfa anything. It must
	// touch the base to be reachable, so make the base's neighbour black.
	black := Cell{Centre + 1, Centre}
	p.Surface[black].Kind = Black
	p.Surface[black].Item = NoItem
	if p.CanBuild(black, Factory, Base) || p.CanBuild(black, Outpost, Base) || !p.CanBuild(black, TransportTubes, Base) {
		t.Error("black square rules")
	}
	if !p.CanBuild(black, Outpost, Orfa.Traits()) {
		t.Error("the Orfa's Outpost on black")
	}
	if err := p.Build(black, TransportTubes); err != nil {
		t.Fatal(err)
	}
	if p.CanBuild(black, Outpost, Orfa.Traits()) {
		t.Error("built over the tubes")
	}
	p.Demolish(black)
	if p.Surface[black].Item != NoItem {
		t.Error("demolish")
	}
	// Orbit: no ship before a shipyard.
	if p.CanBuildOrbital(0, Ship) || p.CanBuildOrbital(0, Factory) || p.CanBuildOrbital(OrbitSlots, Shipyard) {
		t.Error("orbit rules")
	}
	if err := p.BuildOrbital(0, Shipyard); err != nil {
		t.Fatal(err)
	}
	if !p.CanBuildOrbital(1, Ship) || p.CanBuildOrbital(0, Ship) {
		t.Error("ship after shipyard")
	}
	ind, res, pros := p.Output()
	if ind != 2 || res != 0 || pros != 1 { // base 1/0/1 + shipyard 1/0/0
		t.Errorf("output %d %d %d", ind, res, pros)
	}
	used, max := p.Population()
	if used != 2 || max != 2 {
		t.Errorf("population %d/%d", used, max)
	}
}

// TestBonusSquares pins the +1 rule: a structure on a bonus square gets one
// more of that output only when it makes some by itself.
func TestBonusSquares(t *testing.T) {
	// A hand-made planet: NewPlanet would roll the kinds. NoItem is -1,
	// not the zero value, so the orbit must be emptied by hand.
	p := &Planet{Type: Cornucopia, Size: 1, Owner: -1, Surface: map[Cell]*Square{
		{5, 5}: {Kind: BonusIndustry, Item: Factory},    // 1 -> 2
		{4, 5}: {Kind: BonusResearch, Item: Agriplot},   // prosperity 2, research stays 0
		{5, 4}: {Kind: BonusProsperity, Item: Agriplot}, // 2 -> 3
		{6, 5}: {Kind: White, Item: Laboratory},         // 1
		{5, 6}: {Kind: BonusResearch, Item: Laboratory}, // 1 -> 2
	}}
	for i := range p.Orbit {
		p.Orbit[i] = NoItem
	}
	ind, res, pros := p.Output()
	if ind != 2 || res != 3 || pros != 5 {
		t.Errorf("output %d %d %d, want 2 3 5", ind, res, pros)
	}
}

// TestReachable pins the adjacency rule: a structure must share an edge
// with one already built, except on an empty planet.
func TestReachable(t *testing.T) {
	p := NewPlanet(3, "T", Eden, 3) // no black squares
	if !p.CanBuild(Cell{Centre, Centre + 2}, Factory, Base) {
		t.Fatal("an empty planet should accept a structure anywhere")
	}
	if err := p.Build(Cell{Centre, Centre}, ColonyBase); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Cell{{4, 5}, {6, 5}, {5, 4}, {5, 6}} {
		if !p.CanBuild(c, Factory, Base) {
			t.Errorf("%v next to the base should be buildable", c)
		}
	}
	for _, c := range []Cell{{4, 4}, {6, 6}, {3, 5}, {5, 7}} {
		if p.CanBuild(c, Factory, Base) {
			t.Errorf("%v away from the base should not be buildable", c)
		}
	}
	if err := p.Build(Cell{4, 5}, Factory); err != nil {
		t.Fatal(err)
	}
	if !p.CanBuild(Cell{3, 5}, Laboratory, Base) {
		t.Error("the colony should grow from the factory")
	}
}

// A home world's base never starts hemmed in: its four edge neighbours
// are not black, for every species and many seeds, except a species that
// builds on black (Orfa), whose home keeps its roll: it does get black
// neighbours on some seed.
func TestHomeBaseOpen(t *testing.T) {
	for sp := range NumSpecies {
		onBlack := sp.Traits().BuildsOnBlack()
		blacks := 0
		for seed := uint64(1); seed <= 200; seed++ {
			p := sp.HomePlanet(seed, "Home")
			for _, n := range (Cell{Centre, Centre}).edges() {
				if sq, ok := p.Surface[n]; ok && sq.Kind == Black {
					blacks++
					if !onBlack {
						t.Errorf("%v seed %d: neighbour %v of the base is black", sp, seed, n)
					}
				}
			}
		}
		if onBlack && blacks == 0 {
			t.Errorf("%v: no black neighbour of the base on seeds 1..200: still forced open", sp)
		}
	}
}

// orfaBlackSeed finds a seed in 1..200 whose Orfa home world has a black
// edge neighbour of the base, and that neighbour.
func orfaBlackSeed(t *testing.T) (uint64, Cell) {
	t.Helper()
	found := 0
	var seed uint64
	var at Cell
	for sd := uint64(1); sd <= 200; sd++ {
		p := Orfa.HomePlanet(sd, "Home")
		for _, n := range (Cell{Centre, Centre}).edges() {
			if sq, ok := p.Surface[n]; ok && sq.Kind == Black {
				if found == 0 {
					seed, at = sd, n
				}
				found++
				break
			}
		}
	}
	if found == 0 {
		t.Skip("no Orfa home world with a black neighbour of the base on seeds 1..200")
	}
	t.Logf("%d Orfa seeds of 200 have a black neighbour; seed %d, cell %v", found, seed, at)
	return seed, at
}

// An Orfa colony starts and completes a Factory on a black square next to
// its base; another species' colony on the same square cannot start it.
func TestOrfaBuildOnBlack(t *testing.T) {
	seed, at := orfaBlackSeed(t)
	e := NewEmpire(seed, Orfa, Average)
	col := e.Colonies[0]
	if !col.Traits().BuildsOnBlack() || col.Planet.Surface[at].Kind != Black {
		t.Fatalf("home colony on black %v, square %v", col.Traits().BuildsOnBlack(), col.Planet.Surface[at].Kind)
	}
	other := NewLoneColony(col.Planet, Snovemdomas)
	if err := other.CanStart(e.Research, Factory, at); err == nil || !strings.Contains(err.Error(), "black") {
		t.Errorf("Snovemdomas on black: %v", err)
	}
	if err := col.Start(e.Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	for range 400 {
		e.EndDay()
		if col.Project == nil {
			break
		}
	}
	if col.Project != nil || col.Planet.Surface[at].Item != Factory {
		t.Errorf("Factory on black not completed: project %v, square holds %v", col.Project, col.Planet.Surface[at].Item)
	}
	// A colony the Orfa found is theirs too.
	for pl := 1; pl < len(e.Cluster.Stars[col.Star].System.Planets); pl++ {
		if _, ok := e.Planet(col.Star, pl).BestCell(ColonyBase, Orfa.Traits()); !ok {
			continue
		}
		if err := e.Found(e.Spawn(laneShip, col.Star, pl)); err != nil {
			t.Fatal(err)
		}
		if nc := e.Colony(col.Star, pl); nc == nil || nc.owner != e {
			t.Errorf("founded colony %+v", nc)
		}
		break
	}
}

// Cell's text form "i,j" round-trips, and garbage is refused.
func TestCellText(t *testing.T) {
	for _, c := range []Cell{{0, 0}, {5, 10}, {-1, 3}} {
		b, err := c.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back Cell
		if err := back.UnmarshalText(b); err != nil || back != c {
			t.Errorf("%v -> %q -> %v %v", c, b, back, err)
		}
	}
	for _, s := range []string{"", "1", "1,", ",2", "1,2,3", "a,b"} {
		var c Cell
		if err := c.UnmarshalText([]byte(s)); err == nil {
			t.Errorf("%q read as %v", s, c)
		}
	}
}
