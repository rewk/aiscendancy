package game

import (
	"errors"
	"strings"
	"testing"
)

// The generator lays at most one ruin per planet, none on a home (the
// cluster's or NewGame's other empires'), about RuinChanceTiny of the
// tiny planets with one, RuinChance of the others.
func TestRuinsGenerated(t *testing.T) {
	planets, ruins := 0, 0
	tiny, tinyRuins := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		if cl := NewCluster(seed, Average); !cl.Stars[cl.Home].System.Planets[0].HasStructure() {
			t.Fatalf("seed %d: the cluster's home has no base", seed)
		} else if _, ok := cl.Stars[cl.Home].System.Planets[0].RuinsAt(); ok {
			t.Fatalf("seed %d: ruins on the cluster's home", seed)
		}
		g := NewGame(seed, []Species{Snovemdomas, Orfa, Mebes}, Average)
		for i := range g.Cluster.Stars {
			for _, p := range g.Cluster.Stars[i].System.Planets {
				n := 0
				for _, sq := range p.Surface {
					if sq.Ruins {
						n++
					}
				}
				at, ok := p.RuinsAt()
				switch {
				case n > 1:
					t.Fatalf("seed %d %s: %d ruins", seed, p.Name, n)
				case ok != (n == 1) || ok && !p.Surface[at].Ruins:
					t.Fatalf("seed %d %s: RuinsAt %v %v, %d ruins", seed, p.Name, at, ok, n)
				case n == 1 && (p.HasStructure() || p.Owner >= 0):
					t.Fatalf("seed %d %s: ruins on a home", seed, p.Name)
				}
				if p.Size == MinSize {
					tiny++
					tinyRuins += n
					continue
				}
				planets++
				ruins += n
			}
		}
	}
	share := float64(ruins) / float64(planets)
	t.Logf("%d of %d planets with ruins (%.3f)", ruins, planets, share)
	if share < RuinChance-0.03 || share > RuinChance+0.03 {
		t.Errorf("share %.3f, want about %.3f", share, RuinChance)
	}
	// Fewer tiny planets: a wider margin.
	ts := float64(tinyRuins) / float64(tiny)
	t.Logf("%d of %d tiny planets with ruins (%.3f)", tinyRuins, tiny, ts)
	if ts < RuinChanceTiny-0.06 || ts > RuinChanceTiny+0.06 {
		t.Errorf("tiny share %.3f, want about %.3f", ts, RuinChanceTiny)
	}
}

// The ruins are part of the seeded roll: one seed twice agrees square for
// square, Ruins included. (That the other squares did not move when the
// ruins came is the byte-identical sim and fuzz runs' proof.)
func TestRuinsDeterministic(t *testing.T) {
	a, b := NewCluster(3, Average), NewCluster(3, Average)
	for i := range a.Stars {
		for k, p := range a.Stars[i].System.Planets {
			q := b.Stars[i].System.Planets[k]
			for c, sq := range p.Surface {
				if *sq != *q.Surface[c] {
					t.Fatalf("star %d planet %d %v: %+v vs %+v", i, k, c, *sq, *q.Surface[c])
				}
			}
		}
	}
}

// digHome is a Snovemdomas home with the dig researched, two more people,
// and ruins on a reachable empty square, made black (a black ruin is
// dug as any).
func digHome(t *testing.T, seed uint64) (*Empire, *Colony, Cell) {
	t.Helper()
	e := NewEmpire(seed, Snovemdomas, Average)
	c := e.Colonies[0]
	c.Pop += 2
	e.Research.Grant(Tech(XenoArcheologicalDig.Info().ResearchReq))
	p := c.Planet
	for _, at := range p.Cells() {
		if sq := p.Surface[at]; sq.Item == NoItem && p.Reachable(at) {
			sq.Kind, sq.Ruins = Black, true
			return e, c, at
		}
	}
	t.Fatal("no reachable empty square")
	return nil, nil, Cell{}
}

// The dig starts on the ruins square only, black included, reachable and
// empty, past the common checks; never as a planet-wide project.
func TestDigCanStart(t *testing.T) {
	e, c, at := digHome(t, 1)
	r, p, it := e.Research, c.Planet, XenoArcheologicalDig
	if err := c.CanStart(r, it, at); err != nil {
		t.Fatalf("on the black ruins: %v", err)
	}
	if !p.Offers(it, at) {
		t.Error("the ruins square does not offer the dig")
	}
	for _, x := range p.Cells() {
		if x == at {
			continue
		}
		if p.Offers(it, x) {
			t.Errorf("%v offers the dig", x)
		}
		if err := c.CanStart(r, it, x); err == nil || !strings.Contains(err.Error(), "no Xeno ruins at "+x.String()) {
			t.Errorf("dig at %v: %v", x, err)
		}
	}
	if err := c.CanStartProject(r, it); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("dig as a planet-wide project: %v", err)
	}
	// Unresearched, no free worker: the common refusals.
	r.Known[it.Info().ResearchReq] = false
	if err := c.CanStart(r, it, at); err == nil {
		t.Error("unresearched dig allowed")
	}
	r.Grant(Tech(it.Info().ResearchReq))
	pop := c.Pop
	c.Pop = c.Busy()
	if err := c.CanStart(r, it, at); !errors.Is(err, ErrNoFreePop) {
		t.Errorf("no free worker: %v", err)
	}
	c.Pop = pop
	// A structure on the ruins.
	p.Surface[at].Item = TransportTubes
	if err := c.CanStart(r, it, at); err == nil || !strings.Contains(err.Error(), "holds") {
		t.Errorf("ruins under tubes: %v", err)
	}
	p.Surface[at].Item = NoItem
	// Ruins moved to an unreachable empty square.
	p.Surface[at].Ruins = false
	far, found := Cell{}, false
	for _, x := range p.Cells() {
		if sq := p.Surface[x]; sq.Item == NoItem && !p.Reachable(x) {
			far, found = x, true
			sq.Ruins = true
			break
		}
	}
	if !found {
		t.Fatal("no unreachable square")
	}
	if err := c.CanStart(r, it, far); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Errorf("unreachable ruins: %v", err)
	}
	p.Surface[far].Ruins, p.Surface[at].Ruins = false, true
	if err := c.Start(r, it, at); err != nil {
		t.Fatal(err)
	}
	if err := c.CanStart(r, it, at); !errors.Is(err, ErrProjectRunning) {
		t.Errorf("second dig: %v", err)
	}
}

// runDig plays the colony's days until the dig completes, its events.
func runDig(t *testing.T, c *Colony, r *Research, key uint64) []Event {
	t.Helper()
	for day := 0; day < 2000; day++ {
		ev := c.EndDay(r, key)
		for _, x := range ev {
			if x.Kind == Built && x.Item == XenoArcheologicalDig {
				return ev
			}
		}
		if c.Project == nil {
			t.Fatal("dig dropped")
		}
	}
	t.Fatal("dig never completed")
	return nil
}

func known(r *Research) int {
	n := 0
	for _, k := range r.Known {
		if k {
			n++
		}
	}
	return n
}

// A completed dig clears the ruins, places nothing and grants one tech
// not known before: Built then Discovered, at the colony. All known, it is
// still dug, nothing learnt.
func TestDigCompletes(t *testing.T) {
	e, c, at := digHome(t, 1)
	r, p := e.Research, c.Planet
	if err := c.Start(r, XenoArcheologicalDig, at); err != nil {
		t.Fatal(err)
	}
	before, n := r.Known, known(r)
	ev := runDig(t, c, r, 7)
	if sq := p.Surface[at]; sq.Ruins || sq.Item != NoItem || sq.Kind != Black {
		t.Errorf("dug square %+v", *sq)
	}
	if _, ok := p.RuinsAt(); ok {
		t.Error("ruins left")
	}
	if len(ev) < 2 || ev[0].Kind != Built || ev[1].Kind != Discovered {
		t.Fatalf("events %+v", ev)
	}
	if want := "Dig done on " + p.Name; ev[0].Text != want {
		t.Errorf("built %q, want %q", ev[0].Text, want)
	}
	d := ev[1]
	if d.Tech >= NumTechs || before[d.Tech] || !r.Known[d.Tech] || known(r) != n+1 {
		t.Errorf("tech %v: known before %v, now %d from %d", d.Tech, d.Tech < NumTechs && before[d.Tech], known(r), n)
	}
	if want := d.Tech.String() + " found in ruins"; d.Text != want || d.Star != c.Star || d.Planet != c.PlanetIdx || d.Item != NoItem {
		t.Errorf("discovered %+v, want %q", d, want)
	}

	// Everything known: dug, no Discovered.
	_, c2, at2 := digHome(t, 2)
	r2 := NewResearch()
	for i := range r2.Known {
		r2.Known[i] = true
	}
	if err := c2.Start(r2, XenoArcheologicalDig, at2); err != nil {
		t.Fatal(err)
	}
	for _, x := range runDig(t, c2, r2, 7) {
		if x.Kind == Discovered {
			t.Errorf("all known: %+v", x)
		}
	}
	if c2.Planet.Surface[at2].Ruins {
		t.Error("all known: ruins left")
	}
}

// The draw is a pure function of the key and the colony's place: two
// identical games dig up the same tech; over many keys and planets the
// draws spread over the unknown techs, and only those.
func TestDigDraw(t *testing.T) {
	var got [2]Tech
	for k := range got {
		g := NewGame(5, []Species{Snovemdomas, Orfa}, Average)
		c := g.Empires[0].Colonies[0]
		c.Pop += 2
		r := g.Empires[0].Research
		r.Grant(Tech(XenoArcheologicalDig.Info().ResearchReq))
		var at Cell
		for _, x := range c.Planet.Cells() {
			if sq := c.Planet.Surface[x]; sq.Item == NoItem && c.Planet.Reachable(x) {
				sq.Ruins, at = true, x
				break
			}
		}
		if err := c.Start(r, XenoArcheologicalDig, at); err != nil {
			t.Fatal(err)
		}
		got[k] = NoTech
		for day := 0; day < 2000 && got[k] == NoTech; day++ {
			for _, x := range g.EndDay()[0] {
				if x.Kind == Discovered && x.Star == c.Star {
					got[k] = x.Tech
				}
			}
		}
		if got[k] == NoTech {
			t.Fatal("no dig discovery")
		}
	}
	if got[0] != got[1] {
		t.Errorf("same game, techs %v and %v", got[0], got[1])
	}

	r := NewResearch()
	r.Known[0], r.Known[1] = true, true
	seen := map[Tech]int{}
	for day := range 300 {
		for pl := range 5 {
			c := &Colony{Star: day % 7, PlanetIdx: pl}
			tech, ok := c.dig(r, DigKey(1, 0, day))
			if !ok || r.Known[tech] {
				t.Fatalf("day %d planet %d: %v %v", day, pl, tech, ok)
			}
			seen[tech]++
		}
	}
	if want := int(NumTechs) - 2; len(seen) != want {
		t.Errorf("%d distinct techs over 1500 draws, want %d", len(seen), want)
	}
	if DigKey(1, 0, 3) == DigKey(1, 1, 3) || DigKey(1, 0, 3) == DigKey(2, 0, 3) || DigKey(1, 0, 3) == DigKey(1, 0, 4) {
		t.Error("DigKey ignores an argument")
	}
}

// Nothing but the dig goes on Xeno ruins: CanBuild refuses every surface
// item there, black or white, under any species' traits (where the same
// square without ruins takes some), CanStart says why, Terraforming is
// refused on a black ruin, BestCell steps aside, Offers lists the dig
// alone, and the dig is still taken.
func TestRuinsRefuseBuilds(t *testing.T) {
	e, c, at := digHome(t, 1)
	r, p := e.Research, c.Planet
	for i := range r.Known {
		r.Known[i] = true
	}
	sq := p.Surface[at]
	without := 0
	for _, kind := range []SquareKind{White, Black} {
		sq.Kind = kind
		for sp := Species(0); sp < NumSpecies; sp++ {
			tr := sp.Traits()
			for it := Item(0); it < NumItems; it++ {
				if !it.Surface() {
					continue
				}
				sq.Ruins = false
				if p.CanBuild(at, it, tr) {
					without++
				}
				sq.Ruins = true
				if p.CanBuild(at, it, tr) {
					t.Errorf("%s builds %s on %v ruins", sp, it, kind)
				}
			}
		}
	}
	if without == 0 {
		t.Fatal("the square takes nothing even without ruins: the refusals prove nothing")
	}
	sq.Kind = White
	if err := c.CanStart(r, Factory, at); err == nil || !strings.Contains(err.Error(), at.String()+" holds Xeno ruins: dig them first") {
		t.Errorf("Factory on the ruins: %v", err)
	}
	sq.Kind = Black
	if p.CanTerraform(at) {
		t.Error("terraforming black ruins")
	}
	if err := c.CanStart(r, Terraforming, at); err == nil || !strings.Contains(err.Error(), "dig them first") {
		t.Errorf("Terraforming on the ruins: %v", err)
	}
	sq.Ruins = false
	if !p.CanTerraform(at) {
		t.Fatal("the black square takes no Terraforming even without ruins")
	}
	sq.Ruins = true
	for it := Item(0); it < NumItems; it++ {
		if got := p.Offers(it, at); got != (it == XenoArcheologicalDig) {
			t.Errorf("Offers(%s) on the ruins: %v", it, got)
		}
	}
	if err := c.CanStart(r, XenoArcheologicalDig, at); err != nil {
		t.Errorf("dig: %v", err)
	}
	// BestCell steps off the square it would pick, once ruins are there.
	sq.Ruins = false
	best, ok := p.BestCell(Factory, Base)
	if !ok {
		t.Fatal("no cell for a Factory")
	}
	p.Surface[best].Ruins = true
	if x, ok := p.BestCell(Factory, Base); !ok || x == best {
		t.Errorf("BestCell %v %v on the ruins %v", x, ok, best)
	}
}
