package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// scenarioGame is cmd/mksave's default new game: seed 1, average, three
// empires, the player Snovemdomas.
func scenarioGame() *Game {
	return NewGame(1, PickSpecies(1, Snovemdomas, 3), Average)
}

// freePlanets is the unowned planets of star, in order.
func freePlanets(g *Game, star int) []int {
	var out []int
	for k, p := range g.Cluster.Stars[star].System.Planets {
		if p.Owner < 0 {
			out = append(out, k)
		}
	}
	return out
}

func gameJSON(t *testing.T, g *Game) string {
	t.Helper()
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseScenario(t *testing.T) {
	s, err := ParseScenario(strings.NewReader(`{"star": 9, "stances": [],
		"planets": [{"planet": 1, "empire": 1, "orbit": ["orbital shield"], "surface": ["SURFACE SHIELD"]}],
		"ships": [{"empire": 2, "design": " Cruiser ", "at": "lane 1"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Star != "9" || s.Stances == nil || len(s.Planets) != 1 || len(s.Ships) != 1 || s.Ships[0].Count != 0 {
		t.Errorf("%+v", s)
	}
	for _, home := range []string{`{}`, `{"star": "home"}`, `{"star": ""}`} {
		if s, err := ParseScenario(strings.NewReader(home)); err != nil || (s.Star != "" && s.Star != "home") || s.Stances != nil {
			t.Errorf("%s: %+v %v", home, s, err)
		}
	}
	for _, bad := range []string{
		``,
		`{"star": "home", "fleet": []}`, // unknown field
		`{"ships": [{"design": "x", "at": "orbit 0"}], "colour": 1}`,
		`{"planets": [{"planet": 1, "empire": 1, "pop": 2, "owner": 1}]}`,
		`{} {}`, // trailing data
		`{"star": "away"}`, `{"star": -1}`, `{"star": 1.5}`,
		`{"ships": [{"empire": 0, "design": "gunboat", "at": "orbit 0"}]}`,
		`{"ships": [{"empire": 0, "design": "raider", "at": "orbit"}]}`,
		`{"ships": [{"empire": 0, "design": "raider", "at": "dock 0"}]}`,
		`{"ships": [{"empire": 0, "design": "raider", "at": "near -1"}]}`,
		`{"ships": [{"empire": 0, "design": "raider", "at": "lane 1 2"}]}`,
		`{"ships": [{"empire": 0, "design": "raider", "count": -1, "at": "lane 0"}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "orbit": ["Orbital Sheild"]}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "orbit": ["Surface Shield"]}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "orbit": ["Ship"]}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "surface": ["Orbital Shield"]}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "surface": ["Terraforming"]}]}`,
		`{"planets": [{"planet": 1, "empire": 1, "pop": -2}]}`,
		`{"planets": [{"planet": 1, "empire": -1, "orbit": ["Orbital Shield"]}]}`,
		`{"planets": [{"planet": 1, "empire": -1, "pop": 3}]}`,
		`{"stances": [{"a": 0, "b": 1, "stance": "truce"}]}`,
		`{"stances": [{"a": 1, "b": 1, "stance": "war"}]}`,
	} {
		if s, err := ParseScenario(strings.NewReader(bad)); err == nil {
			t.Errorf("%s accepted: %+v", bad, s)
		}
	}
	for st := Unknown; st <= Alliance; st++ {
		if got, err := ParseStance(strings.ToUpper(st.String())); err != nil || got != st {
			t.Errorf("%s: %v %v", st, got, err)
		}
	}
}

// Refused against the game, the game untouched: every refusal comes
// after valid parts that the dry run applied to its copy.
func TestApplyScenarioRefusals(t *testing.T) {
	g := scenarioGame()
	home := g.Empires[0].Colonies[0]
	free := freePlanets(g, home.Star)
	np := len(g.Cluster.Stars[home.Star].System.Planets)
	nl := len(g.Cluster.Stars[home.Star].Lanes)
	ok := ScenarioPlanet{Planet: free[0], Empire: 1, Orbit: []string{"Orbital Shield"}, Surface: []string{"Surface Shield"}}
	okShips := ScenarioShips{Empire: 2, Design: "frigate", Count: 2, At: "lane 0"}
	tenOrbitals := slices.Repeat([]string{"Orbital Missile Base"}, OrbitSlots+1)
	for _, c := range []struct {
		name string
		s    Scenario
		want string
	}{
		{"star", Scenario{Star: "999"}, "star 999"},
		{"planet's empire", Scenario{Planets: []ScenarioPlanet{ok, {Planet: free[1], Empire: 3}}}, "empire 3"},
		{"ships' empire", Scenario{Planets: []ScenarioPlanet{ok}, Ships: []ScenarioShips{okShips, {Empire: 3, Design: "raider", At: "orbit 0"}}}, "empire 3"},
		{"stance's empire", Scenario{Planets: []ScenarioPlanet{ok}, Stances: []ScenarioStance{{A: 0, B: 3, Stance: "war"}}}, "empire 3"},
		{"another's planet", Scenario{Planets: []ScenarioPlanet{ok, {Planet: home.PlanetIdx, Empire: 1}}}, "empire 0's"},
		{"uninhabited owned", Scenario{Planets: []ScenarioPlanet{{Planet: home.PlanetIdx, Empire: -1}}}, "empire 0's"},
		{"planet index", Scenario{Planets: []ScenarioPlanet{ok, {Planet: np, Empire: 1}}}, "planet"},
		{"orbit index", Scenario{Planets: []ScenarioPlanet{ok}, Ships: []ScenarioShips{{Empire: 0, Design: "raider", At: "orbit " + itoa(np)}}}, "planet"},
		{"lane index", Scenario{Planets: []ScenarioPlanet{ok}, Ships: []ScenarioShips{okShips, {Empire: 0, Design: "raider", At: "lane " + itoa(nl)}}}, "lane"},
		{"pop", Scenario{Planets: []ScenarioPlanet{{Planet: free[0], Empire: 1, Pop: 1000}}}, "capacity"},
		{"slots", Scenario{Planets: []ScenarioPlanet{{Planet: free[0], Empire: 1, Orbit: tenOrbitals}}}, "orbit slot"},
		{"bad design", Scenario{Planets: []ScenarioPlanet{ok}, Ships: []ScenarioShips{{Empire: 0, Design: "gunboat", At: "orbit 0"}}}, "gunboat"},
	} {
		before := gameJSON(t, g)
		err := ApplyScenario(g, &c.s)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if gameJSON(t, g) != before {
			t.Fatalf("%s: the game changed", c.name)
		}
	}

	// No square left: a planet filled by its own Colony Base's
	// neighbourhood refuses the surplus.
	p := g.Cluster.Stars[home.Star].System.Planets[free[0]]
	many := slices.Repeat([]string{"Factory"}, len(p.Surface)+1)
	before := gameJSON(t, g)
	if err := ApplyScenario(g, &Scenario{Planets: []ScenarioPlanet{{Planet: free[0], Empire: 1, Surface: many}}}); err == nil ||
		!strings.Contains(err.Error(), "no square") || gameJSON(t, g) != before {
		t.Errorf("a full surface: %v", err)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// The whole scenario, checked against the model: owners, colonies and
// their pops, the orbit slots taken first free, the squares BuildWith
// takes, the ships in orbit and out of it where FreeSpot puts them, the
// stances; deterministic, and the dry run leaves no trace.
func TestApplyScenario(t *testing.T) {
	g := scenarioGame()
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	home := e0.Colonies[0]
	star := home.Star
	c := g.Cluster
	sys := c.Stars[star].System
	free := freePlanets(g, star)
	if len(free) < 2 || len(c.Stars[star].Lanes) < 2 {
		t.Fatalf("home %d: free planets %v, lanes %v", star, free, c.Stars[star].Lanes)
	}
	a, b := free[0], free[1]
	fresh := scenarioGame()
	homeSlot := slices.Index(fresh.Cluster.Stars[star].System.Planets[home.PlanetIdx].Orbit[:], NoItem)
	baseAt, _ := fresh.Cluster.Stars[star].System.Planets[a].BestCell(ColonyBase, e1.Species.Traits())
	s := &Scenario{
		Stances: []ScenarioStance{{A: 1, B: 2, Stance: "alliance"}, {A: 0, B: 1, Stance: "war"}, {A: 2, B: 0, Stance: "War"}},
		Planets: []ScenarioPlanet{
			{Planet: a, Empire: 1, Pop: 5, Orbit: []string{"Orbital Shield", "Orbital Missile Base"}, Surface: []string{"Surface Shield"}},
			{Planet: home.PlanetIdx, Empire: 0, Orbit: []string{"orbital mega shield"}},
			{Planet: b, Empire: -1},
		},
		Ships: []ScenarioShips{
			{Empire: 0, Design: "cruiser", Count: 2, At: "orbit " + itoa(home.PlanetIdx)},
			{Empire: 1, Design: "frigate", Count: 2, At: "lane 1"},
			{Empire: 2, Design: "corvette", At: "near " + itoa(b)},
		},
	}
	if err := ApplyScenario(g, s); err != nil {
		t.Fatal(err)
	}

	pa := sys.Planets[a]
	col := e1.Colony(star, a)
	if pa.Owner != 1 || col == nil || len(e1.Colonies) != 2 || col.Pop != 5 || col.owner != e1 ||
		pa.Surface[baseAt].Item != ColonyBase {
		t.Errorf("planet %d: owner %d, colony %+v", a, pa.Owner, col)
	}
	if !slices.Equal(pa.Orbit[:2], []Item{OrbitalShield, OrbitalMissileBase}) || pa.Count(SurfaceShield) != 1 {
		t.Errorf("planet %d: orbit %v, %d Surface Shields", a, pa.Orbit, pa.Count(SurfaceShield))
	}
	if sys.Planets[home.PlanetIdx].Orbit[homeSlot] != OrbitalMegaShield || len(e0.Colonies) != 1 {
		t.Errorf("home orbit %v (first free %d)", sys.Planets[home.PlanetIdx].Orbit, homeSlot)
	}
	if sys.Planets[b].Owner != -1 {
		t.Errorf("planet %d owned by %d", b, sys.Planets[b].Owner)
	}

	if len(e0.Ships) != 2 || len(e1.Ships) != 2 || len(e2.Ships) != 1 {
		t.Fatalf("ships %d/%d/%d", len(e0.Ships), len(e1.Ships), len(e2.Ships))
	}
	for _, sh := range e0.Ships {
		if sh.Design.Name != FixtureCruiser.Name || sh.Star != star || sh.Orbit != home.PlanetIdx || sh.Pos != sys.Planets[home.PlanetIdx].Pos {
			t.Errorf("cruiser %+v", sh)
		}
	}
	_, end, ok := laneEnd(c, star, c.Other(c.Stars[star].Lanes[1], star))
	if !ok {
		t.Fatal("no lane end")
	}
	at := sys.Lanes[end].Pos
	wantPos := []Vec3{at, FreeSpot(at, []Vec3{at})}
	for i, sh := range e1.Ships {
		if sh.Design.Name != FixtureFrigate.Name || sh.Star != star || sh.Orbit != -1 || sh.Lane != -1 || sh.Pos != wantPos[i] {
			t.Errorf("frigate %d at %v, want %v: %+v", i, sh.Pos, wantPos[i], sh)
		}
	}
	if sh := e2.Ships[0]; sh.Design.Name != FixtureCorvette.Name || sh.Orbit != -1 || sh.Pos != FreeSpot(LeavePoint(g.Cluster, star, b), wantPos) ||
		dist(sh.Pos, sys.Planets[b].Pos) < PlanetClear-1e-9 { // off the orbit ring, as Leave Orbit
		t.Errorf("corvette %+v", sh)
	}
	if !e1.Knows(star) || !e2.Knows(star) {
		t.Error("the empires there do not know the star")
	}

	for _, st := range []struct {
		a, b int
		want Stance
	}{{1, 2, Alliance}, {0, 1, War}, {0, 2, War}} {
		if got := g.Empires[st.a].StanceWith(st.b); got != st.want || g.Empires[st.b].StanceWith(st.a) != st.want ||
			g.Empires[st.a].Relations[st.b].Met != g.Day {
			t.Errorf("%d-%d: %v, want %v", st.a, st.b, got, st.want)
		}
	}
	// Founded before they met: no encroachment tallied.
	if e0.Relations[1].Encroached != 0 {
		t.Errorf("encroached %d", e0.Relations[1].Encroached)
	}

	// Deterministic, and what the dry run did stayed on its copy: the
	// direct application on a fresh game is the same JSON.
	if err := applyScenario(fresh, s); err != nil {
		t.Fatal(err)
	}
	if gameJSON(t, g) != gameJSON(t, fresh) {
		t.Error("ApplyScenario differs from a single application")
	}
}

// No Stances: every pair of the empires named is at War, the others
// untouched; a star away from home is explored for empire 0, its ships
// or not; a colony founded among empires that met tallies Encroached
// (settle's encroach); "unknown" unmeets.
func TestApplyScenarioDefaults(t *testing.T) {
	g := scenarioGame()
	home := g.Empires[0].Colonies[0].Star
	away := g.Cluster.Other(g.Cluster.Stars[home].Lanes[0], home)
	free := freePlanets(g, away)
	if g.Empires[0].Knows(away) || len(free) == 0 {
		t.Fatalf("star %d: known %v, free %v", away, g.Empires[0].Knows(away), free)
	}
	s := &Scenario{Star: StarRef(itoa(away)), Planets: []ScenarioPlanet{{Planet: free[0], Empire: 2}},
		Ships: []ScenarioShips{{Empire: 1, Design: "raider", At: "orbit " + itoa(free[0])}}}
	if err := ApplyScenario(g, s); err != nil {
		t.Fatal(err)
	}
	if !g.Empires[0].Knows(away) {
		t.Error("empire 0 does not see the scenario's star")
	}
	if g.Empires[1].StanceWith(2) != War || g.Empires[0].StanceWith(1) != Unknown || g.Empires[0].StanceWith(2) != Unknown {
		t.Errorf("stances 1-2 %v, 0-1 %v, 0-2 %v", g.Empires[1].StanceWith(2), g.Empires[0].StanceWith(1), g.Empires[0].StanceWith(2))
	}

	h := scenarioGame()
	hs := h.Empires[0].Colonies[0].Star
	h.Contact(0, 1)
	s = &Scenario{Planets: []ScenarioPlanet{{Planet: freePlanets(h, hs)[0], Empire: 1}}, Stances: []ScenarioStance{}}
	if err := ApplyScenario(h, s); err != nil {
		t.Fatal(err)
	}
	if h.Empires[0].Relations[1].Encroached != 1 || h.Empires[0].StanceWith(1) != Peace {
		t.Errorf("met: encroached %d, stance %v", h.Empires[0].Relations[1].Encroached, h.Empires[0].StanceWith(1))
	}
	if err := ApplyScenario(h, &Scenario{Stances: []ScenarioStance{{A: 1, B: 0, Stance: "unknown"}}}); err != nil {
		t.Fatal(err)
	}
	if r := h.Empires[0].Relations[1]; r.Stance != Unknown || r.Met != -1 || h.Empires[1].Relations[0].Met != -1 {
		t.Errorf("unmet: %+v", r)
	}
}

// A staged colony is staffed as a played one: its Pop at least the
// structures' Staff (Colony.Busy), a pop of 0 raised to it, a pop given
// under it refused (threeway.json's Orbital Shield and Missile Base on a
// new colony of 2 people left it 1 short).
func TestApplyScenarioStaffed(t *testing.T) {
	g := scenarioGame()
	home := g.Empires[0].Colonies[0]
	free := freePlanets(g, home.Star)
	two := []string{"Orbital Shield", "Orbital Missile Base"}
	s := &Scenario{Planets: []ScenarioPlanet{
		{Planet: free[0], Empire: 1, Orbit: two, Surface: []string{"Surface Shield"}},
		{Planet: home.PlanetIdx, Empire: 0, Orbit: two},
	}}
	if err := ApplyScenario(g, s); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*Colony{g.Empires[1].Colony(home.Star, free[0]), home} {
		if c.Free() != 0 || c.Busy() <= StartPop {
			t.Errorf("%s: pop %d, busy %d", c.Planet.Name, c.Pop, c.Busy())
		}
	}
	h := scenarioGame()
	before := gameJSON(t, h)
	s = &Scenario{Planets: []ScenarioPlanet{{Planet: free[0], Empire: 1, Pop: 2, Orbit: two}}}
	if err := ApplyScenario(h, s); err == nil || !strings.Contains(err.Error(), "staff") || gameJSON(t, h) != before {
		t.Errorf("pop under the staff: %v", err)
	}
}
