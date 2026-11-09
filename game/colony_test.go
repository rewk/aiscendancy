package game

import (
	"errors"
	"strings"
	"testing"
)

// handPlanet builds a planet from a cell->square list. NoItem is -1, not
// the zero value: the orbit is emptied by hand.
func handPlanet(size int, sq map[Cell]*Square) *Planet {
	p := &Planet{Name: "Hand", Type: Congenial, Size: size, Surface: sq, Owner: -1}
	for i := range p.Orbit {
		p.Orbit[i] = NoItem
	}
	return p
}

// baseWorld: the base on white at the centre, a red square west, a white
// one south, everything else white out to distance 2.
func baseWorld() *Planet {
	sq := map[Cell]*Square{}
	for _, c := range Layout(3) {
		if abs(c.I-Centre)+abs(c.J-Centre) <= 2 {
			sq[c] = &Square{Kind: White, Item: NoItem}
		}
	}
	sq[Cell{Centre, Centre}].Item = ColonyBase
	sq[Cell{4, 5}].Kind = BonusIndustry
	return handPlanet(3, sq)
}

// runUntil runs days until an event of kind k, returning the day (1-based)
// and the event; it fails after max days.
func runUntil(t *testing.T, c *Colony, r *Research, k EventKind, max int) (int, Event) {
	t.Helper()
	for d := 1; d <= max; d++ {
		for _, e := range c.EndDay(r, 0) {
			if e.Kind == k {
				return d, e
			}
		}
	}
	t.Fatalf("no %v event in %d days", k, max)
	return 0, Event{}
}

func TestNewColonyPanicsOnEmptyPlanet(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewColony on an empty planet should panic")
		}
	}()
	p := baseWorld()
	p.Demolish(Cell{Centre, Centre})
	NewColony(p)
}

func TestColonyPopulation(t *testing.T) {
	c := NewColony(baseWorld())
	in := ColonyBase.Info()
	if c.Pop != StartPop || c.Busy() != in.UsePop || c.Free() != StartPop-in.UsePop {
		t.Errorf("pop %d busy %d free %d", c.Pop, c.Busy(), c.Free())
	}
	if want := bareCapacity(c.Planet) + in.MaxPop; c.Capacity() != want {
		t.Errorf("capacity %d, want %d", c.Capacity(), want)
	}
	r := NewResearch()
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	// The project reserves its workers once, from its start.
	if c.Busy() != in.UsePop+Factory.Info().UsePop {
		t.Errorf("busy %d with a project", c.Busy())
	}
}

func TestCanStartRefuses(t *testing.T) {
	r := NewResearch()
	at := Cell{4, 5}

	c := NewColony(baseWorld())
	if err := c.CanStart(r, Factory, at); err != nil {
		t.Fatalf("the base case should start: %v", err)
	}
	if err := c.Start(r, Factory, at); err != nil {
		t.Fatal(err)
	}
	if err := c.CanStart(r, Agriplot, Cell{6, 5}); !errors.Is(err, ErrProjectRunning) {
		t.Errorf("running project: %v", err)
	}

	c = NewColony(baseWorld())
	c.Pop = c.Busy()
	if err := c.CanStart(r, Factory, at); !errors.Is(err, ErrNoFreePop) {
		t.Errorf("no free pop: %v", err)
	}

	c = NewColony(baseWorld())
	if err := c.CanStart(r, ResearchCampus, at); err == nil || !strings.Contains(err.Error(), "needs Hyperlogic") {
		t.Errorf("research campus: %v", err)
	}
	if err := c.CanStart(r, Factory, Cell{3, 5}); err == nil || !strings.Contains(err.Error(), "reachable") {
		t.Errorf("unreachable: %v", err)
	}
	if err := c.CanStart(r, Shipyard, at); err == nil {
		t.Error("an orbital item on the surface should be refused")
	}
	if err := c.CanStartOrbital(r, Shipyard, 0); err == nil {
		t.Error("a shipyard without Orbital Structures should be refused")
	}
	r.Grant(OrbitalStructures)
	if err := c.CanStartOrbital(r, Shipyard, 0); err != nil {
		t.Errorf("shipyard: %v", err)
	}
	if c.Project != nil || c.Busy() != ColonyBase.Info().UsePop {
		t.Error("a refused or checked start changed the colony")
	}
}

// With industry 1 a Factory (cost 30) completes on day 30, not 29 or 31.
func TestFactoryCompletesOnTheCostDay(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	ind, _, _ := c.Output()
	if ind != 1 {
		t.Fatalf("base industry %d", ind)
	}
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	day, e := runUntil(t, c, r, Built, 100)
	if want := Factory.Info().Cost / ind; day != want {
		t.Errorf("built on day %d, want %d", day, want)
	}
	if e.Item != Factory || c.Project != nil || c.Planet.Surface[Cell{4, 5}].Item != Factory {
		t.Errorf("event %+v, project %v, square %v", e, c.Project, c.Planet.Surface[Cell{4, 5}].Item)
	}
	if want := "Factory built on " + c.Planet.Name; e.Text != want {
		t.Errorf("text %q, want %q", e.Text, want)
	}
	if ind, _, _ := c.Output(); ind != 1+2 {
		t.Errorf("industry %d after a factory on red, want 3", ind)
	}
}

// Industry 7 against cost 30: day 5 (28 < 30 <= 35), progress clamped.
// Seven comes from a sum of 10 (base 1, three factories on red 6, three on
// white 3): round(10^0.85) = 7.
func TestIndustrySurplusLost(t *testing.T) {
	p := baseWorld()
	for _, c := range []Cell{{4, 5}, {6, 5}, {5, 4}} {
		p.Surface[c].Kind = BonusIndustry
		p.Surface[c].Item = Factory
	}
	for _, c := range []Cell{{4, 4}, {6, 6}, {6, 4}} {
		p.Surface[c].Kind = White
		p.Surface[c].Item = Factory
	}
	c := NewColony(p)
	c.Pop = 10
	r := NewResearch()
	if ind, _, _ := c.Output(); ind != 7 {
		t.Fatalf("industry %d, want 7", ind)
	}
	if err := c.Start(r, Factory, Cell{5, 6}); err != nil {
		t.Fatal(err)
	}
	pj := c.Project
	if day, _ := runUntil(t, c, r, Built, 10); day != 5 {
		t.Errorf("built on day %d, want 5", day)
	}
	if pj.Progress != Factory.Info().Cost {
		t.Errorf("progress %d, want clamped to %d", pj.Progress, Factory.Info().Cost)
	}
	// ASSUMPTION 1 and 2: nothing is carried to the next project.
	for range 3 {
		c.EndDay(r, 0)
	}
	if err := c.Start(r, Laboratory, Cell{4, 6}); err != nil {
		t.Fatal(err)
	}
	if c.Project.Progress != 0 {
		t.Errorf("a new project starts at %d", c.Project.Progress)
	}
}

// A Laboratory built by the colony (50 days at industry 1), then Orbital
// Structures (cost 50) at research 1: discovered 50 days after the lab.
func TestColonyResearch(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	if err := r.Study(OrbitalStructures); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(r, Laboratory, Cell{5, 6}); err != nil {
		t.Fatal(err)
	}
	lab, _ := runUntil(t, c, r, Built, 200)
	if r.Progress[OrbitalStructures] != 0 {
		t.Errorf("research before the lab: %d", r.Progress[OrbitalStructures])
	}
	day, e := runUntil(t, c, r, Discovered, 200)
	if day != r.Cost(OrbitalStructures) || e.Tech != OrbitalStructures {
		t.Errorf("discovered %v on day %d after the lab (day %d), want %d", e.Tech, day, lab, r.Cost(OrbitalStructures))
	}
	if !r.Known[OrbitalStructures] || r.Topic != NoTech {
		t.Error("the tech should be known and the topic cleared")
	}
}

func TestColonyGrowth(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	if _, _, pros := c.Output(); pros != 1 {
		t.Fatalf("base prosperity %d", pros)
	}
	day, e := runUntil(t, c, r, Grew, 100)
	if day != GrowthCost || c.Pop != StartPop+1 || c.Growth != 0 {
		t.Errorf("grew on day %d to %d (growth %d), want day %d to %d", day, c.Pop, c.Growth, GrowthCost, StartPop+1)
	}
	if !strings.Contains(e.Text, "grows to 3") {
		t.Errorf("text %q", e.Text)
	}

	c = NewColony(baseWorld())
	c.Pop = c.Capacity()
	for d := 0; d < 3*GrowthCost; d++ {
		for _, e := range c.EndDay(r, 0) {
			if e.Kind == Grew {
				t.Fatalf("grew past capacity on day %d", d+1)
			}
		}
	}
	// A full colony on a lone base has prosperity 1 − Capacity/4 < 0: the
	// bank starves to 0 and nobody leaves.
	if c.Pop != c.Capacity() || c.Growth != 0 {
		t.Errorf("full colony: pop %d growth %d", c.Pop, c.Growth)
	}
}

// TestOutputFormulas pins the manual's curves: Colony Base + Agriplot show
// prosperity 3 (1 → 3 seen in the original), industry 1 + 2 shows 3,
// 10 shows 7, and four people cost one point of prosperity.
func TestOutputFormulas(t *testing.T) {
	// The original's values, sums 1 to 19 (test saves; 9 → 7 its quirk).
	want := []int{1, 2, 3, 3, 4, 5, 5, 6, 7, 7, 8, 8, 9, 9, 10, 11, 11, 12, 12}
	for i, w := range want {
		if got := power(float64(i + 1)); got != w {
			t.Errorf("power(%d) = %d, want %d", i+1, got, w)
		}
	}
	c := NewColony(baseWorld())
	cell, ok := c.Planet.BestCell(Agriplot, Base)
	if !ok {
		t.Fatal("no cell for the agriplot")
	}
	if err := c.Planet.Build(cell, Agriplot); err != nil {
		t.Fatal(err)
	}
	if _, _, pros := c.Output(); pros != 3 {
		t.Errorf("base + agriplot: prosperity %d, want 3", pros)
	}
	c.Pop = 4
	if _, _, pros := c.Output(); pros != 2 {
		t.Errorf("four people: prosperity %d, want 2", pros)
	}
	for i := range c.Planet.Orbit {
		c.Planet.Orbit[i] = NoItem
	}
	c.Planet.Orbit[0] = FertilizationPlant // orbital or not, Count sees it: ×1.5 → floor(4.5)=4 → 4^0.85=3.25 → 3, −1
	if _, _, pros := c.Output(); pros != 2 {
		t.Errorf("with a fertilization plant: prosperity %d, want 2", pros)
	}
}

// One day completing a build, a discovery and a growth reports them in
// that order.
func TestEndDayEventOrder(t *testing.T) {
	p := baseWorld()
	p.Surface[Cell{6, 5}].Item = Laboratory
	c := NewColony(p)
	c.Pop = 3 // four people would cost the one point of prosperity
	r := NewResearch()
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	c.Project.Progress = Factory.Info().Cost - 1
	if err := r.Study(OrbitalStructures); err != nil {
		t.Fatal(err)
	}
	r.Progress[OrbitalStructures] = r.Cost(OrbitalStructures) - 1
	c.Growth = GrowthCost - 1
	ev := c.EndDay(r, 0)
	if len(ev) != 3 || ev[0].Kind != Built || ev[1].Kind != Discovered || ev[2].Kind != Grew {
		t.Fatalf("events %+v", ev)
	}
	if ev[0].Tech != NoTech || ev[1].Item != NoItem || ev[2].Item != NoItem || ev[2].Tech != NoTech {
		t.Errorf("unused fields not None: %+v", ev)
	}
}

func TestBestCell(t *testing.T) {
	sq := map[Cell]*Square{
		{5, 5}: {Kind: White, Item: ColonyBase},
		{4, 5}: {Kind: BonusIndustry, Item: NoItem},
		{6, 5}: {Kind: BonusResearch, Item: NoItem},
		{5, 4}: {Kind: BonusProsperity, Item: NoItem},
		{5, 6}: {Kind: White, Item: NoItem},
		{5, 7}: {Kind: BonusIndustry, Item: NoItem}, // not reachable
	}
	p := handPlanet(3, sq)
	for _, tc := range []struct {
		it   Item
		want Cell
	}{
		{Factory, Cell{4, 5}},
		{Laboratory, Cell{6, 5}},
		{Agriplot, Cell{5, 4}},
		{Outpost, Cell{5, 6}},
	} {
		if got, ok := p.BestCell(tc.it, Base); !ok || got != tc.want {
			t.Errorf("%v: %v %v, want %v", tc.it, got, ok, tc.want)
		}
	}
	// Without the reachable red: the far red is ignored, and of the whites
	// (4,5) and (5,6), both at distance 1, Layout order picks (4,5).
	sq[Cell{4, 5}].Kind = White
	if got, _ := p.BestCell(Factory, Base); got != (Cell{4, 5}) {
		t.Errorf("factory without red: %v, want (4,5)", got)
	}
	// Nothing reachable and empty: no cell.
	for _, c := range []Cell{{4, 5}, {6, 5}, {5, 4}, {5, 6}} {
		sq[c].Item = Factory
	}
	sq[Cell{5, 7}].Kind = Black
	if c, ok := p.BestCell(Agriplot, Base); ok {
		t.Errorf("agriplot on %v of a full disc", c)
	}
}

// The scripted opening on Snovemdomas' home: Agriplot, Laboratory,
// Factory, each at its BestCell, started as soon as the colony is idle
// with free population.
func TestScriptedOpening(t *testing.T) {
	p := Snovemdomas.HomePlanet(1, "x")
	c := NewColony(p)
	r := NewResearch()
	script := []Item{Agriplot, Laboratory, Factory}
	done := map[Item]int{}
	grew := 0
	next := 0
	for day := 1; day <= 1000 && len(done) < len(script); day++ {
		if c.Project == nil && next < len(script) {
			it := script[next]
			at, ok := p.BestCell(it, Base)
			if !ok {
				t.Fatalf("day %d: no cell for %v", day, it)
			}
			if err := c.Start(r, it, at); err == nil {
				if it == Agriplot {
					// Green if any green square was reachable, else white.
					green := false
					for _, cl := range p.Cells() {
						if p.CanBuild(cl, it, Base) && p.Surface[cl].Kind == BonusProsperity {
							green = true
						}
					}
					if k := p.Surface[at].Kind; green && k != BonusProsperity || !green && k != White {
						t.Errorf("agriplot on %v (%v), green reachable %v", at, k, green)
					}
				}
				t.Logf("day %d: start %v at %v (%v)", day, it, at, p.Surface[at].Kind)
				next++
			} else if !errors.Is(err, ErrNoFreePop) {
				t.Fatalf("day %d: %v: %v", day, it, err)
			}
		}
		for _, e := range c.EndDay(r, 0) {
			switch e.Kind {
			case Built:
				done[e.Item] = day
			case Grew:
				if grew == 0 {
					grew = day
				}
			}
		}
	}
	for i, it := range script {
		if done[it] == 0 {
			t.Fatalf("%v never completed", it)
		}
		if i > 0 && done[it] <= done[script[i-1]] {
			t.Errorf("%v completed day %d, before %v (%d)", it, done[it], script[i-1], done[script[i-1]])
		}
	}
	t.Logf("opening: Agriplot day %d, Laboratory day %d, Factory day %d, first growth day %d",
		done[Agriplot], done[Laboratory], done[Factory], grew)
}

// A Colony Base is never a project: it lands with the colony.
func TestColonyBaseNotBuildable(t *testing.T) {
	c := NewColony(baseWorld())
	c.Pop = 5
	r := NewResearch()
	cell, ok := c.Planet.BestCell(Factory, Base)
	if !ok {
		t.Fatal("no square")
	}
	if err := c.CanStart(r, Factory, cell); err != nil {
		t.Fatalf("the square refuses a Factory: %v, the next check proves nothing", err)
	}
	if err := c.CanStart(r, ColonyBase, cell); !errors.Is(err, ErrNotBuildable) {
		t.Errorf("Colony Base: %v, want %v", err, ErrNotBuildable)
	}
}

// Cancel loses the project's industry and frees its workers; DaysLeft
// rounds the rest up at today's industry, false when idle; DaysFor counts
// a whole cost the same way.
func TestColonyCancel(t *testing.T) {
	c := NewColony(baseWorld())
	c.Pop = 5
	r := NewResearch()
	if _, ok := c.DaysLeft(); ok {
		t.Error("an idle colony has days left")
	}
	cell, _ := c.Planet.BestCell(Factory, Base)
	free := c.Free()
	if err := c.Start(r, Factory, cell); err != nil {
		t.Fatal(err)
	}
	c.EndDay(r, 0)
	ind, _, _ := c.Output()
	if ind <= 0 || c.Project == nil || c.Project.Progress == 0 {
		t.Fatalf("industry %d, project %+v: the checks below prove nothing", ind, c.Project)
	}
	cost, done := c.Project.Cost(), c.Project.Progress
	if d, ok := c.DaysLeft(); !ok || d != (cost-done+ind-1)/ind {
		t.Errorf("days left %d %v, want %d", d, ok, (cost-done+ind-1)/ind)
	}
	if d, ok := c.DaysFor(cost); !ok || d != (cost+ind-1)/ind {
		t.Errorf("days for %d: %d %v, want %d", cost, d, ok, (cost+ind-1)/ind)
	}
	if c.Cancel(); c.Project != nil || c.Free() != free {
		t.Fatalf("cancel: project %v, free %d want %d", c.Project, c.Free(), free)
	}
	if err := c.Start(r, Factory, cell); err != nil || c.Project.Progress != 0 {
		t.Errorf("restart: %v, progress %d: the cancelled industry came back", err, c.Project.Progress)
	}
}

// CanStartProject passes the common checks, then refuses every project
// but the Lush Growth Bomb and the ongoing ones (Alien Hospitality among
// them) with ErrNotModelled; a
// structure is not a project, a Targeted one is not planet-wide.
func TestCanStartProject(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Colonies[0]
	if err := c.CanStartProject(e.Research, Factory); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("Factory: %v", err)
	}
	if err := c.CanStartProject(e.Research, Automation); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("Automation, a Targeted one: %v", err)
	}
	it := AlienHospitality
	if err := c.CanStartProject(e.Research, it); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("%s unresearched: %v", it, err)
	}
	e.Research.Grant(Tech(it.Info().ResearchReq))
	if err := c.CanStartProject(e.Research, it); err != nil {
		t.Errorf("%s researched: %v (an ongoing project since diplomacy lot 2)", it, err)
	}
	if err := c.StartProject(e.Research, it); err != nil || c.Project == nil || c.Project.Item != it {
		t.Errorf("%s started: %v, project %v", it, err, c.Project)
	}
}

// bareCapacity is the saves' rule counted on p's squares: 5 + N/3, N the
// squares that are not black.
func bareCapacity(p *Planet) int {
	n := 0
	for _, sq := range p.Surface {
		if sq.Kind != Black {
			n++
		}
	}
	return CapacityBase + n/CapacityPerSquares
}

// A bare planet's capacity is CapacityBase + N/CapacityPerSquares whatever
// its type and size, N counted on the rolled squares; the placed
// structures add their MaxPop.
func TestCapacityBare(t *testing.T) {
	for i, f := range []struct {
		typ  PlanetType
		size int
	}{{Husk, 5}, {Primordial, 5}, {Congenial, 2}, {Cornucopia, 3}, {Tycoon, 5}, {Eden, 1}, {Cornucopia, 4}} {
		p := NewPlanet(uint64(i+1), "P", f.typ, f.size)
		c := &Colony{Planet: p}
		n := 0
		for _, sq := range p.Surface {
			if sq.Kind != Black {
				n++
			}
		}
		if want := 5 + n/3; c.Capacity() != want {
			t.Errorf("%v size %d (%d not black of %d): capacity %d, want %d", f.typ, f.size, n, len(p.Surface), c.Capacity(), want)
		}
	}
	p := baseWorld()
	p.Orbit[0] = Habitat
	c := &Colony{Planet: p}
	if want := bareCapacity(p) + ColonyBase.Info().MaxPop + Habitat.Info().MaxPop; c.Capacity() != want {
		t.Errorf("base + orbital habitat: capacity %d, want %d", c.Capacity(), want)
	}
}

// A structure under construction adds nothing: the Habitat's MaxPop
// counts from the day it is placed.
func TestCapacityProjectAddsNothing(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	r.Grant(Tech(Habitat.Info().ResearchReq))
	before := c.Capacity()
	if err := c.Start(r, Habitat, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	for day := 1; c.Project != nil; day++ {
		if c.Capacity() != before {
			t.Fatalf("day %d, project at %d/%d: capacity %d, want %d", day, c.Project.Progress, c.Project.Cost(), c.Capacity(), before)
		}
		if day > 2000 {
			t.Fatal("the Habitat never completes")
		}
		c.EndDay(r, 0)
	}
	if c.Planet.Surface[Cell{4, 5}].Item != Habitat {
		t.Fatal("no Habitat placed")
	}
	if want := before + Habitat.Info().MaxPop; c.Capacity() != want {
		t.Errorf("built: capacity %d, want %d", c.Capacity(), want)
	}
}

// Terraforming raises the capacity by itself: three black squares turned
// white make one more unit.
func TestCapacityTerraformed(t *testing.T) {
	p := baseWorld()
	var blacks []Cell
	for _, at := range p.Cells() {
		if sq, ok := p.Surface[at]; ok && sq.Item == NoItem && sq.Kind == White && len(blacks) < 3 {
			sq.Kind, sq.Item = Black, TransportTubes // tubes: terraformable unreached
			blacks = append(blacks, at)
		}
	}
	c := &Colony{Planet: p}
	room := ColonyBase.Info().MaxPop + 3*TransportTubes.Info().MaxPop
	before := c.Capacity()
	if want := bareCapacity(p) + room; before != want {
		t.Fatalf("capacity %d, want %d", before, want)
	}
	for _, at := range blacks {
		if err := p.Terraform(at); err != nil {
			t.Fatal(err)
		}
		room -= TransportTubes.Info().MaxPop
		if want := bareCapacity(p) + room; c.Capacity() != want {
			t.Errorf("terraformed %v: capacity %d, want %d", at, c.Capacity(), want)
		}
	}
	if c.Capacity() != before+1-3*TransportTubes.Info().MaxPop {
		t.Errorf("three terraformed: capacity %d from %d", c.Capacity(), before)
	}
}

// The Lush Growth Bomb starts once per planet: on completion a Built
// event, Planet.Lush, Capacity +LushBonus; then it is refused.
func TestLushGrowthBomb(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Colonies[0]
	r := e.Research
	it := LushGrowthBomb
	if err := c.CanStartProject(r, it); err == nil {
		t.Error("unresearched bomb allowed")
	}
	r.Grant(Tech(it.Info().ResearchReq))
	before := c.Capacity()
	if err := c.StartProject(r, it); err != nil {
		t.Fatal(err)
	}
	var ev *Event
	for day := 0; day < 2000 && ev == nil; day++ {
		if c.Capacity() != before || c.Planet.Lush {
			t.Fatalf("day %d: capacity %d, lush %v before completion", day, c.Capacity(), c.Planet.Lush)
		}
		for _, x := range c.EndDay(r, 0) {
			if x.Kind == Built {
				ev = &x
			}
		}
	}
	if ev == nil {
		t.Fatal("never completed")
	}
	if want := "Lush Growth Bomb built on " + c.Planet.Name; ev.Item != it || ev.Text != want || ev.Star != c.Star || ev.Planet != c.PlanetIdx {
		t.Errorf("event %+v, want %q", *ev, want)
	}
	if !c.Planet.Lush || c.Capacity() != before+LushBonus {
		t.Errorf("lush %v, capacity %d, want %d", c.Planet.Lush, c.Capacity(), before+LushBonus)
	}
	if err := c.CanStartProject(r, it); err == nil || errors.Is(err, ErrNotModelled) || !strings.Contains(err.Error(), "already been bombed") {
		t.Errorf("second bomb: %v", err)
	}
	if err := c.StartProject(r, it); err == nil || c.Project != nil {
		t.Errorf("second bomb started: %v", err)
	}
}

// Terraforming starts where Transport Tubes go on black (an empty
// reachable black square) and turns it white when done; white, holding a
// structure or unreachable, it is refused.
func TestTerraforming(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Colonies[0]
	c.Pop += 2
	r := e.Research
	r.Grant(Tech(Terraforming.Info().ResearchReq))
	p := c.Planet
	// The home has no reachable black square: two reachable white ones,
	// one turned black, and an unreachable black one.
	var black, white, far Cell
	var nb, nw, nf bool
	for _, at := range p.Cells() {
		sq := p.Surface[at]
		switch {
		case sq.Item != NoItem:
		case !p.Reachable(at) && sq.Kind == Black:
			far, nf = at, true
		case !p.Reachable(at) || sq.Kind != White:
		case !nb:
			black, nb = at, true
			sq.Kind = Black
		default:
			white, nw = at, true
		}
	}
	if !nb || !nw || !nf {
		t.Fatalf("home lacks a test square: black %v white %v far %v", nb, nw, nf)
	}
	if !p.CanBuild(black, TransportTubes, c.Traits()) {
		t.Fatalf("tubes refused on %v: not the tubes' case", black)
	}
	for _, at := range []Cell{white, far} {
		if err := c.CanStart(r, Terraforming, at); err == nil {
			t.Errorf("Terraforming allowed at %v (%s)", at, p.Surface[at].Kind)
		}
	}
	if err := c.CanStartProject(r, Terraforming); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("Terraforming as a planet-wide project: %v", err)
	}
	if err := c.Start(r, Terraforming, black); err != nil {
		t.Fatal(err)
	}
	var built bool
	var text string
	for day := 0; day < 1000 && !built; day++ {
		for _, ev := range c.EndDay(r, 0) {
			if ev.Kind == Built && ev.Item == Terraforming {
				built, text = true, ev.Text
			}
		}
	}
	if sq := p.Surface[black]; !built || sq.Kind != White || sq.Item != NoItem {
		t.Errorf("after the project: built %v, square %+v", built, *sq)
	}
	// The log names the square as the view does, (i,j).
	if want := "Terraforming built on " + p.Name; text != want {
		t.Errorf("event %q, want %q", text, want)
	}
	// Black under tubes: terraformed, the tubes removed; under another
	// structure, refused.
	sq := p.Surface[black]
	sq.Kind, sq.Item = Black, TransportTubes
	if err := c.CanStart(r, Terraforming, black); err != nil {
		t.Errorf("tubes on black: %v", err)
	}
	if err := p.Terraform(black); err != nil || sq.Kind != White || sq.Item != NoItem {
		t.Errorf("tubes terraformed: %v, square %+v", err, *sq)
	}
	sq.Kind, sq.Item = Black, Outpost
	if err := c.CanStart(r, Terraforming, black); err == nil {
		t.Error("Terraforming under an Outpost")
	}
}

// GrowthDays agrees with EndDay: the colony grows exactly on the day it
// announces (no project, research off the point); full or barren: false.
func TestGrowthDays(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	c.Project = nil
	n, ok := c.GrowthDays()
	if !ok {
		t.Fatal("the home does not grow: the fixture proves nothing")
	}
	pop := c.Pop
	for d := 1; d <= n; d++ {
		c.EndDay(e.Research, 0)
		if grew := c.Pop > pop; grew != (d == n) {
			t.Fatalf("day %d of %d: grew %v", d, n, grew)
		}
	}
	c.Pop = c.Capacity()
	if _, ok := c.GrowthDays(); ok {
		t.Error("full: still growing")
	}
}

// The Cloning Plant doubles a positive prosperity on its way into the
// growth bank: the colony grows in half the days; the output shown is
// unchanged; a negative prosperity is not doubled.
func TestCloningPlant(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	at, ok := c.Planet.BestCell(CloningPlant, Base)
	if !ok {
		t.Fatal("no cell")
	}
	if err := c.Planet.Build(at, CloningPlant); err != nil {
		t.Fatal(err)
	}
	c.Pop++ // its worker
	_, _, pros := c.Output()
	if pros != 1 {
		t.Fatalf("prosperity %d, want 1 (the plant changes no output)", pros)
	}
	if n, ok := c.GrowthDays(); !ok || n != (GrowthCost+1)/2 {
		t.Errorf("GrowthDays %d %v, want %d (the planet view's countdown)", n, ok, (GrowthCost+1)/2)
	}
	day, _ := runUntil(t, c, r, Grew, 100)
	if want := (GrowthCost + 1) / 2; day != want {
		t.Errorf("grew on day %d, want %d (growth doubled)", day, want)
	}
	c.Growth = 10
	c.Pop = 12 // upkeep beyond the prosperity: negative, not doubled
	c.EndDay(r, 0)
	if _, _, pros := c.Output(); pros >= 0 || c.Growth != max(0, 10+pros) {
		t.Errorf("negative prosperity %d: growth %d", pros, c.Growth)
	}
}

// A built Outpost employs nobody and only adds its room, nor do the
// Tubes (both FlagOnBlack); the Habitat and the Metroplex employ their
// worker (Item.Staff). The
// project takes its worker while it builds, abandoning the structure
// takes the room and frees nobody.
func TestStaff(t *testing.T) {
	for _, w := range []struct {
		it    Item
		staff int
	}{{Outpost, 0}, {TransportTubes, 0}, {Habitat, 1}, {Metroplex, 1}, {Factory, Factory.Info().UsePop}} {
		if got := w.it.Staff(); got != w.staff {
			t.Errorf("%s: staff %d, want %d", w.it, got, w.staff)
		}
		if w.it == TransportTubes { // the tubes' placement is not baseWorld's: the count alone
			continue
		}
		c := NewColony(baseWorld())
		c.Pop = 5
		r := NewResearch()
		if q := w.it.Info().ResearchReq; q != NoResearch {
			learn(r, Tech(q))
		}
		busy, room := c.Busy(), c.Capacity()
		at, ok := c.Planet.BestCell(w.it, Base)
		if !ok {
			t.Fatalf("%s: no square", w.it)
		}
		if err := c.Start(r, w.it, at); err != nil {
			t.Fatalf("%s: %v", w.it, err)
		}
		if c.Busy() != busy+w.it.Info().UsePop {
			t.Errorf("%s building: busy %d, want %d", w.it, c.Busy(), busy+w.it.Info().UsePop)
		}
		for d := 0; c.Project != nil; d++ {
			if d > 1000 {
				t.Fatalf("%s never completes", w.it)
			}
			c.EndDay(r, 0)
		}
		if c.Busy() != busy+w.staff || c.Capacity() != room+w.it.Info().MaxPop {
			t.Errorf("%s built: busy %d room %d, want %d %d", w.it, c.Busy(), c.Capacity(), busy+w.staff, room+w.it.Info().MaxPop)
		}
		if err := c.Abandon(at); err != nil {
			t.Fatalf("%s: abandon: %v", w.it, err)
		}
		if c.Busy() != busy || c.Capacity() != room {
			t.Errorf("%s abandoned: busy %d room %d, want %d %d", w.it, c.Busy(), c.Capacity(), busy, room)
		}
	}
}

// CanAbandon refuses a structure whose room the population needs; an
// Automation refuses a structure that employs nobody; FreeAtCompletion
// forecasts Free after the running project: a Factory keeps its worker,
// an Outpost gives it back, an Automation frees its target's too.
func TestAbandonRoomAndForecast(t *testing.T) {
	c := NewColony(baseWorld())
	r := NewResearch()
	learn(r, Tech(Automation.Info().ResearchReq))
	at, ok := c.Planet.BestCell(Outpost, Base)
	if !ok || c.Planet.Build(at, Outpost) != nil {
		t.Fatal("no Outpost square")
	}
	c.Pop = c.Capacity()
	if err := c.CanAbandon(at); err == nil || c.Abandon(at) == nil || c.Planet.Surface[at].Item != Outpost {
		t.Errorf("abandoned with the room used: %v", err)
	}
	if err := c.CanStart(r, Automation, at); err == nil {
		t.Error("Automation offered on an Outpost")
	}
	c.Pop = c.Capacity() - Outpost.Info().MaxPop
	if err := c.CanAbandon(at); err != nil {
		t.Errorf("room spare: %v", err)
	}

	c = NewColony(baseWorld())
	c.Pop = 6
	for _, w := range []struct {
		it   Item
		gain int // Free after completion less Free while building
	}{{Factory, Factory.Info().UsePop - Factory.Staff()}, {Outpost, Outpost.Info().UsePop}} {
		cell, ok := c.Planet.BestCell(w.it, Base)
		if !ok || c.Start(r, w.it, cell) != nil {
			t.Fatalf("%s: cannot start", w.it)
		}
		want := c.Free() + w.gain
		if got := c.FreeAtCompletion(); got != want {
			t.Errorf("%s: forecast %d, want %d", w.it, got, want)
		}
		for d := 0; c.Project != nil; d++ {
			if d > 1000 {
				t.Fatalf("%s never completes", w.it)
			}
			c.EndDay(r, 0)
		}
		if c.Free() != want {
			t.Errorf("%s: free %d after, forecast %d", w.it, c.Free(), want)
		}
	}
	f, ok := c.Planet.BestCell(Factory, Base)
	if !ok || c.Planet.Build(f, Factory) != nil {
		t.Fatal("no Factory square")
	}
	if err := c.Start(r, Automation, f); err != nil {
		t.Fatal(err)
	}
	want := c.Free() + Automation.Info().UsePop + Factory.Staff()
	if got := c.FreeAtCompletion(); got != want {
		t.Errorf("Automation: forecast %d, want %d", got, want)
	}
}
