package game

import "testing"

// settle founds a colony of e on the first planet of star it can settle,
// false when none.
func settle(e *Empire, star int) bool {
	for k, p := range e.Cluster.Stars[star].System.Planets {
		if p.Owner >= 0 {
			continue
		}
		if _, ok := p.BestCell(ColonyBase, e.Species.Traits()); !ok {
			continue
		}
		s := e.Spawn(FixtureColonyShip, star, k)
		if e.Found(s) == nil {
			e.Ships = e.Ships[:len(e.Ships)-1]
			return true
		}
	}
	return false
}

// Each victory where it holds, none at the start; the close keeps the
// first won and tells it once, to the winner and to the others.
func TestVictories(t *testing.T) {
	g := dipGame(t)
	e0 := g.Empires[0]
	if v := g.VictoryOf(e0); v != NoVictory {
		t.Fatalf("at the start: %v", v)
	}
	ally(t, g, 0, 1)
	if v := g.VictoryOf(e0); v != NoVictory {
		t.Errorf("one ally of two: %v", v)
	}
	ally(t, g, 0, 2)
	if v := g.VictoryOf(e0); v != UniteVictory {
		t.Errorf("both allied: %v", v)
	}
	evs := g.EndDay()
	won := func(i int) int {
		n := 0
		for _, x := range evs[i] {
			if x.Kind == Won {
				n++
			}
		}
		return n
	}
	if e0.Won != UniteVictory || e0.WonDay != g.Day || won(0) != 1 || won(1) != 1 || won(2) != 1 {
		t.Fatalf("the close: won %v day %d, events %d %d %d", e0.Won, e0.WonDay, won(0), won(1), won(2))
	}
	if g.Empires[1].Won != NoVictory {
		t.Errorf("the ally won %v", g.Empires[1].Won)
	}
	for i, want := range []int{e0.HomeStar(), -1, -1} { // the others' copy names no star: fog
		for _, x := range evs[i] {
			if x.Kind == Won && x.Star != want {
				t.Errorf("empire %d's Won at star %d, want %d", i, x.Star, want)
			}
		}
	}
	evs = g.EndDay()
	if won(0) != 0 || e0.Won != UniteVictory {
		t.Errorf("told again: %d, %v", won(0), e0.Won)
	}

	g = dipGame(t)
	e0 = g.Empires[0]
	for _, o := range g.Empires[1:] { // their home colony captured, as invade does
		col := o.Colonies[0]
		o.Colonies = o.Colonies[1:]
		col.adopt(e0)
		e0.Colonies = append(e0.Colonies, col)
		col.Planet.Owner = e0.Index
	}
	if v := g.VictoryOf(e0); v != CaptureVictory {
		t.Errorf("a colony at each home: %v", v)
	}
	for _, o := range g.Empires[1:] {
		o.Lost = 1
	}
	if v := g.VictoryOf(e0); v != DestroyVictory {
		t.Errorf("both defeated: %v", v)
	}
	if v := g.VictoryOf(g.Empires[1]); v != NoVictory {
		t.Errorf("a defeated empire won %v", v)
	}

	g = dipGame(t)
	e0 = g.Empires[0]
	n := len(g.Cluster.Stars)
	for st := range g.Cluster.Stars {
		if 3*e0.ControlledSystems() > 2*n {
			break
		}
		if g.Cluster.Stars[st].System.Controller() < 0 {
			settle(e0, st)
		}
	}
	if 3*e0.ControlledSystems() <= 2*n {
		t.Fatalf("fixture: %d systems of %d", e0.ControlledSystems(), n)
	}
	if v := g.VictoryOf(e0); v != ControlVictory {
		t.Errorf("%d systems of %d: %v", e0.ControlledSystems(), n, v)
	}
}
