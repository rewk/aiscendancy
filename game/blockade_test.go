package game

import "testing"

// A ship at War in a colony's orbit stops its work in orbit: no
// progress, the industry lost (no Carry), the project kept; a surface
// project, a ship of an empire at peace, one in another orbit, or an
// Orbital Cloaker hiding the orbit from the blockader leave it working.
func TestBlockade(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e1 := g.Empires[1]
	home := g.Empires[0].Colonies[0]
	if ind, _, _ := home.Output(); ind <= 0 {
		t.Fatal("fixture: no industry at home")
	}
	cell, ok := home.Planet.BestCell(Factory, home.Traits())
	if !ok {
		t.Fatal("fixture: no free square")
	}
	free := -1
	for k, it := range home.Planet.Orbit {
		if it == NoItem && k > 0 {
			free = k
			break
		}
	}
	sh := e1.Spawn(orbitShip, home.Star, home.PlanetIdx)
	sh.Order = Order{}
	day := func(name string, pj *Project, blocked bool) {
		t.Helper()
		home.Project, home.Carry, home.Queue = pj, 0, nil
		if got := home.Blockaded(); got != blocked {
			t.Fatalf("%s: Blockaded %v, want %v", name, got, blocked)
		}
		if _, ok := home.DaysLeft(); ok == blocked {
			t.Errorf("%s: DaysLeft ok %v", name, ok)
		}
		g.EndDay()
		sh.Star, sh.Orbit, sh.Lane = home.Star, home.PlanetIdx, -1 // it stays
		switch {
		case blocked && (home.Project != pj || pj.Progress != 0 || home.Carry != 0):
			t.Errorf("%s: project %+v, progress %d, carry %d", name, home.Project, pj.Progress, home.Carry)
		case !blocked && pj.Progress == 0:
			t.Errorf("%s: no progress", name)
		}
	}
	orbital := func() *Project { return &Project{Item: OrbitalShield, Slot: free, Orbital: true} }
	if home.Blockader() != 1 {
		t.Fatalf("Blockader %d, want 1", home.Blockader())
	}
	day("an orbital structure", orbital(), true)
	day("a ship", &Project{Item: Ship, Slot: 0, Orbital: true, Design: &orbitShip}, true)
	day("a surface structure", &Project{Item: Factory, Cell: cell}, false)

	sh.Orbit = (home.PlanetIdx + 1) % len(g.Cluster.Stars[home.Star].System.Planets)
	day("in another orbit", orbital(), false)
	sh.Orbit = home.PlanetIdx

	was := home.Planet.Orbit[0]
	home.Planet.Orbit[0] = OrbitalCloaker
	if e1.SeesOrbit(home.Planet) || home.Blockader() != -1 {
		t.Fatal("fixture: the cloaker leaves the orbit seen")
	}
	day("under a cloaker", orbital(), false)
	home.Planet.Orbit[0] = was

	g.setStance(0, 1, Peace)
	day("at peace", orbital(), false)
}

// The close reports a blockade begun to both sides, nothing while it
// lasts, its lift to both sides; BlockedBy follows; EndDay delivers.
func TestBlockadeNews(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e1 := g.Empires[1]
	home := g.Empires[0].Colonies[0]
	sh := e1.Spawn(orbitShip, home.Star, home.PlanetIdx)
	news := func() []Report {
		var out []Report
		for _, r := range g.blockadeNews() {
			if r.Event.Kind != Blockade || r.Event.Star != home.Star || r.Event.Planet != home.PlanetIdx {
				t.Errorf("report %+v", r)
			}
			out = append(out, r)
		}
		return out
	}
	name := home.Planet.Name
	if r := news(); len(r) != 2 || r[0].Empire != 0 || r[0].Event.Other != 1 || r[0].Event.Text != name+" blockaded by the Orfa" ||
		r[1].Empire != 1 || r[1].Event.Other != 0 || r[1].Event.Text != name+" blockaded" || home.BlockedBy != 2 {
		t.Fatalf("begun: %+v, BlockedBy %d", r, home.BlockedBy)
	}
	if r := news(); len(r) != 0 {
		t.Errorf("lasting: %+v", r)
	}
	sh.Orbit = -1
	if r := news(); len(r) != 2 || r[0].Empire != 0 || r[1].Empire != 1 || r[0].Event.Text != "Blockade of "+name+" lifted" ||
		r[1].Event.Text != r[0].Event.Text || home.BlockedBy != 0 {
		t.Fatalf("lifted: %+v, BlockedBy %d", r, home.BlockedBy)
	}
	// Through EndDay: the reports reach both empires' events.
	sh.Star, sh.Orbit, sh.Lane, sh.Order = home.Star, home.PlanetIdx, -1, Order{}
	evs := g.EndDay()
	has := func(i int, text string) bool {
		for _, x := range evs[i] {
			if x.Kind == Blockade && x.Text == text {
				return true
			}
		}
		return false
	}
	if !has(0, name+" blockaded by the Orfa") || !has(1, name+" blockaded") {
		t.Errorf("EndDay events: %+v / %+v", evs[0], evs[1])
	}
}

// The blockader defeated at the close (its last colony gone): its ships
// go with it, so the lift is news at that same close, and the dead
// empire hears nothing.
func TestBlockadeLiftOnDefeat(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	atWar(t, g, 0, 1)
	home := g.Empires[0].Colonies[0]
	sh := g.Empires[1].Spawn(orbitShip, home.Star, home.PlanetIdx)
	sh.Order = Order{}
	g.EndDay()
	if home.BlockedBy != 2 {
		t.Fatalf("fixture: BlockedBy %d", home.BlockedBy)
	}
	g.Empires[1].Colonies = nil // its last colony lost today
	evs := g.EndDay()
	if g.Empires[1].Alive() || len(g.Empires[1].Ships) != 0 {
		t.Fatal("fixture: not defeated")
	}
	lifted := func(i int) bool {
		for _, x := range evs[i] {
			if x.Kind == Blockade && x.Text == "Blockade of "+home.Planet.Name+" lifted" {
				return true
			}
		}
		return false
	}
	if !lifted(0) || home.BlockedBy != 0 {
		t.Errorf("no lift at the defeat's close: BlockedBy %d, %+v", home.BlockedBy, evs[0])
	}
	if lifted(1) {
		t.Error("the defeated blockader told of the lift")
	}
}
