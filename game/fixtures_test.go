package game

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// An empire 0 left without colonies (all captured: mksave -load of such a
// save): no fixture asked is no error and nothing spawned, any asked is
// an error, never a panic.
func TestSpawnFixturesNoColony(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	g.Empires[0].Colonies = nil
	if err := SpawnFixtures(g, 0, 0, 0); err != nil {
		t.Errorf("nothing asked: %v", err)
	}
	for _, n := range [][3]int{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		if err := SpawnFixtures(g, n[0], n[1], n[2]); err == nil || !strings.Contains(err.Error(), "no colony") {
			t.Errorf("%v asked: %v", n, err)
		}
	}
	if len(g.Empires[0].Ships) != 0 || len(g.Empires[1].Ships) != 0 {
		t.Errorf("spawned %d and %d ships", len(g.Empires[0].Ships), len(g.Empires[1].Ships))
	}
}

// The catalogue: any case; "colony" is the start's lane colony ship,
// valid on day 0; an unknown name lists the known ones; parts are copies.
func TestArenaDesign(t *testing.T) {
	r, err := ArenaDesign(" RAIDER ")
	if err != nil || !reflect.DeepEqual(r, FixtureRaider) {
		t.Fatalf("raider: %+v %v", r, err)
	}
	if l, err := ArenaDesign("lancer"); err != nil || !reflect.DeepEqual(l, FixtureLancer) ||
		DesignDamage(l, FixtureRaider.ShieldScore()) <= 0 || DesignDamage(FixtureRaider, FixtureRaider.ShieldScore()) != 0 {
		t.Errorf("lancer: %+v %v", l, err)
	}
	r.Parts[0] = Colonizer
	if FixtureRaider.Parts[0] == Colonizer {
		t.Fatal("ArenaDesign shares FixtureRaider's parts")
	}
	c, err := ArenaDesign("Colony")
	if err != nil {
		t.Fatal(err)
	}
	// The lane colony template, named "Lane Colonizer"; its parts those
	// of the guide's lane colony ship (FixtureColonyShip), the first
	// engine and generator.
	all := NewResearch()
	for t := range NumTechs {
		all.Known[t] = true
	}
	tpl := ColonyTemplate(true)
	tpl.Name = "Lane Colonizer"
	late, _ := tpl.Realise(all)
	got, guide := slices.Clone(c.Parts), slices.Clone(FixtureColonyShip.Parts)
	slices.Sort(got)
	slices.Sort(guide)
	if c.Name != tpl.Name || c.Hull != tpl.Hull || !slices.Equal(got, guide) || len(c.Parts) != len(late.Parts) {
		t.Errorf("colony %+v, template %+v, guide %+v", c, tpl, FixtureColonyShip)
	}
	if err := c.Valid(all); err != nil || c.LaneDrives() != 1 || c.Colonizers() != 2 {
		t.Errorf("colony %+v: valid %v, %d drives, %d Colonizers", c, err, c.LaneDrives(), c.Colonizers())
	}
	if _, err := ArenaDesign("gunboat"); err == nil || !strings.Contains(err.Error(), "raider, lancer, colony") {
		t.Errorf("gunboat: %v", err)
	}
}

func TestParseArena(t *testing.T) {
	a, b, err := ParseArena("raider, raider:colony")
	if err != nil || len(a) != 2 || len(b) != 1 || a[1].Name != "Raider" || b[0].Name != "Lane Colonizer" {
		t.Fatalf("%v %v %v", a, b, err)
	}
	for _, bad := range []string{"", "raider", "raider:", ":raider", " , :raider", "raider:x", "raider:raider:raider"} {
		if a, b, err := ParseArena(bad); err == nil {
			t.Errorf("%q accepted: %v %v", bad, a, b)
		}
	}
}

// Side A in orbit of empire 0's home planet, side B at the home system's
// endpoint of its first lane, where Step drops a ship arriving through
// that lane (the oracle: the first on it, the next where Step drops a
// second arrival, ShipGap away); owners, names, full hull and energy.
func TestSpawnArena(t *testing.T) {
	a, b, err := ParseArena("raider,colony:raider,raider")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGame(1, []Species{Minions, Orfa, Mebes}, Average)
	if err := SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	home := g.Empires[0].Colonies[0]
	c := g.Cluster

	// The oracle: a ship in the star's first lane, coming from the far
	// end, Stepped out of it.
	lane := c.Stars[home.Star].Lanes[0]
	far := c.Other(lane, home.Star)
	in := &Starship{Design: FixtureRaider, Star: far, Lane: lane, Arrive: 0, Orbit: -1}
	if !in.Step(c, 1, nil, nil) || in.Star != home.Star {
		t.Fatalf("oracle ship did not arrive: %+v", in)
	}

	e0, e1 := g.Empires[0], g.Empires[1]
	if len(e0.Ships) != 2 || len(e1.Ships) != 2 || len(g.Empires[2].Ships) != 0 {
		t.Fatalf("ships %d/%d/%d", len(e0.Ships), len(e1.Ships), len(g.Empires[2].Ships))
	}
	for i, s := range e0.Ships {
		if g.OwnerOf(s) != e0 || s.Star != home.Star || s.Orbit != home.PlanetIdx || s.Lane != -1 ||
			s.Pos != e0.Planet(home.Star, home.PlanetIdx).Pos || s.Design.Name != a[i].Name {
			t.Errorf("A %d: %+v", i, s)
		}
	}
	next := &Starship{Design: FixtureRaider, Star: far, Lane: lane, Arrive: 0, Orbit: -1}
	if !next.Step(c, 1, func(int) []Vec3 { return []Vec3{in.Pos} }, nil) {
		t.Fatalf("second oracle ship did not arrive: %+v", next)
	}
	if d := dist(in.Pos, next.Pos); math.Abs(d-ShipGap) > 1e-12 {
		t.Fatalf("the second arrival %v from the first, want ShipGap", d)
	}
	for i, s := range e1.Ships {
		want := []Vec3{in.Pos, next.Pos}[i]
		if g.OwnerOf(s) != e1 || s.Star != home.Star || s.Orbit != -1 || s.Lane != -1 ||
			s.Pos != want || s.Order != (Order{}) {
			t.Errorf("B %d: %+v, the arrival at %+v", i, s, want)
		}
	}
	if !e1.Knows(home.Star) {
		t.Error("side B does not know the arena star")
	}
	if names := []string{e0.Ships[0].Name, e0.Ships[1].Name, e1.Ships[0].Name, e1.Ships[1].Name}; !slices.Equal(names,
		[]string{shipName("Raider", 1), shipName("Lane Colonizer", 1), shipName("Raider", 1), shipName("Raider", 2)}) {
		t.Errorf("names %v", names)
	}
	for _, s := range append(slices.Clone(e0.Ships), e1.Ships...) {
		full := *s
		full.Shots = nil // a value copy shares the slice
		full.Refill()
		if s.Hull != s.MaxHull() || s.Power != full.Power || s.Power == 0 || !s.Shields {
			t.Errorf("%s: hull %d/%d, power %d/%d, shields %v", s.Name, s.Hull, s.MaxHull(), s.Power, full.Power, s.Shields)
		}
	}

	// Deterministic: a second game is the same JSON.
	g2 := NewGame(1, []Species{Minions, Orfa, Mebes}, Average)
	if err := SpawnArena(g2, a, b); err != nil {
		t.Fatal(err)
	}
	j1, _ := json.Marshal(g)
	j2, _ := json.Marshal(g2)
	if string(j1) != string(j2) {
		t.Error("two arenas on one seed differ")
	}

	// One empire: refused, nothing spawned.
	solo := NewGame(1, []Species{Minions}, Average)
	if err := SpawnArena(solo, a, b); err == nil || len(solo.Empires[0].Ships) != 0 {
		t.Errorf("one empire: %v, %d ships", err, len(solo.Empires[0].Ships))
	}
}
