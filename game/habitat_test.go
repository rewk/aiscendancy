package game

import (
	"slices"
	"strings"
	"testing"
)

// habShip carries two Space Habitats.
var habShip = Design{Name: "Hab", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, SpaceHabitat, SpaceHabitat}}

// habRefused reports err non-nil and saying want.
func habRefused(t *testing.T, what string, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s: %v, want %q", what, err, want)
	}
}

func TestBerths(t *testing.T) {
	if got, want := habShip.Berths(), 2*Gizmos[SpaceHabitat].Special1; got != want || got != 6 {
		t.Errorf("two modules: %d berths, want %d", got, want)
	}
	if fightFixture.Berths() != 0 {
		t.Error("berths without a habitat")
	}
	s := &Starship{Design: habShip}
	if s.Berths() != habShip.Berths() {
		t.Error("Starship.Berths")
	}
	if !slices.Contains(func() []Gizmo { _, gs := (&Research{}).Unlocks(PlanetaryReplenishment); return gs }(), SpaceHabitat) {
		t.Error("Planetary Replenishment does not unlock the Space Habitat")
	}
}

// One unit per call each way, the colony's and the ship's counts moving
// together; Growth untouched.
func TestLoadUnload(t *testing.T) {
	e, home := shipEmpire(t)
	s := e.Spawn(habShip, home.Star, home.PlanetIdx)
	pop, free, growth := home.Pop, home.Free(), home.Growth
	if free < 1 {
		t.Fatalf("fixture: free %d", free)
	}
	if err := e.Load(s); err != nil || home.Pop != pop-1 || s.Pop != 1 {
		t.Fatalf("load: %v, colony %d ship %d", err, home.Pop, s.Pop)
	}
	if err := e.Unload(s); err != nil || home.Pop != pop || s.Pop != 0 || home.Free() != free || home.Growth != growth {
		t.Fatalf("unload: %v, colony %d (free %d) ship %d", err, home.Pop, home.Free(), s.Pop)
	}
}

func TestLoadRefusals(t *testing.T) {
	e, home := shipEmpire(t)
	s := e.Spawn(habShip, home.Star, home.PlanetIdx)
	pop := home.Pop
	check := func(what, want string) {
		t.Helper()
		habRefused(t, what, e.CanLoad(s), want)
		habRefused(t, what+" (Load)", e.Load(s), want)
		if home.Pop != pop {
			t.Fatalf("%s: colony %d, want %d", what, home.Pop, pop)
		}
	}
	if err := NewEmpire(2, Orfa, Average).Load(s); err != errNotOurs {
		t.Errorf("another empire's: %v", err)
	}
	s.Orbit = -1
	check("in space", "not in orbit")
	s.Orbit, s.Lane = home.PlanetIdx, 0
	check("in a lane", "not in orbit")
	s.Lane = -1
	s.Pop = s.Berths()
	pop = home.Pop
	check("full", "is full")
	s.Pop = 0
	plain := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	habRefused(t, "no habitat", e.CanLoad(plain), "no Space Habitat")
	habRefused(t, "no habitat (Unload)", e.CanUnload(plain), "no Space Habitat")
	home.Pop = home.Busy()
	pop = home.Pop
	if pop < 2 {
		t.Fatalf("fixture: busy %d", pop)
	}
	check("no free population", "no free population")
	// The last unit, free: a home with nothing employed (its Colony Base
	// automated).
	e2 := NewEmpire(1, Snovemdomas, Average)
	h2 := e2.Colonies[0]
	for _, c := range h2.Planet.Cells() {
		if h2.Planet.Surface[c].Item == ColonyBase {
			h2.Planet.Surface[c].Automated = true
		}
	}
	h2.Pop = 1
	if h2.Free() != 1 {
		t.Fatalf("fixture: free %d", h2.Free())
	}
	s2 := e2.Spawn(habShip, h2.Star, h2.PlanetIdx)
	if err := e2.Load(s2); err == nil || !strings.Contains(err.Error(), "last colonist") || h2.Pop != 1 || s2.Pop != 0 {
		t.Errorf("last unit: %v", err)
	}
	// Under refit: not commanded.
	e.Research.Grant(Gravimetrics)
	e.Research.Grant(PlanetaryReplenishment)
	home.Pop = pop + 4
	home.Planet.Orbit[1] = OrbitalDocks
	s.Hull--
	if err := e.Refit(home, s, s.Design); err != nil {
		t.Fatal(err)
	}
	pop = home.Pop
	check("refitting", "being refitted")
}

func TestUnloadRefusals(t *testing.T) {
	e, home := shipEmpire(t)
	s := e.Spawn(habShip, home.Star, home.PlanetIdx)
	habRefused(t, "nobody aboard", e.Unload(s), "nobody aboard")
	s.Pop = 1
	home.Pop = home.Capacity()
	habRefused(t, "colony full", e.Unload(s), "is full")
	if home.Pop != home.Capacity() || s.Pop != 1 {
		t.Errorf("full: colony %d ship %d", home.Pop, s.Pop)
	}
	home.Pop--
	if err := e.Unload(s); err != nil || home.Pop != home.Capacity() || s.Pop != 0 {
		t.Errorf("unload to capacity: %v", err)
	}
	s.Pop, s.Orbit = 1, -1
	habRefused(t, "in space", e.Unload(s), "not in orbit")
	if err := NewEmpire(2, Orfa, Average).Unload(s); err != errNotOurs {
		t.Errorf("another empire's: %v", err)
	}
}

// Neither an uninhabited planet, a foreign colony nor an ally's takes or
// gives colonists.
func TestHabitatOnlyOwnColonies(t *testing.T) {
	g := dipGame(t)
	e, other := g.Empires[0], g.Empires[1].Colonies[0]
	other.Pop = 6
	s := e.Spawn(habShip, other.Star, other.PlanetIdx)
	s.Pop = 1
	pop := other.Pop
	for _, stance := range []string{"foreign", "ally"} {
		if stance == "ally" {
			ally(t, g, 0, 1)
		}
		habRefused(t, stance+" load", e.Load(s), "not our colony")
		habRefused(t, stance+" unload", e.Unload(s), "not our colony")
		if other.Pop != pop || s.Pop != 1 {
			t.Errorf("%s: colony %d ship %d", stance, other.Pop, s.Pop)
		}
	}
	star, pl := -1, -1
	for k := range g.Cluster.Stars {
		for j, p := range g.Cluster.Stars[k].System.Planets {
			if p.Owner < 0 && star < 0 {
				star, pl = k, j
			}
		}
	}
	if star < 0 {
		t.Fatal("fixture: no uninhabited planet")
	}
	s.Star, s.Orbit = star, pl
	habRefused(t, "uninhabited", e.Unload(s), "not our colony")
}

func TestRefitKeepsBerths(t *testing.T) {
	e, home, _ := refitFixture(t)
	e.Research.Grant(PlanetaryReplenishment)
	s := e.Spawn(habShip, home.Star, home.PlanetIdx)
	d := s.Design
	d.Parts = slices.Clone(d.Parts)
	d.Parts[len(d.Parts)-1] = ProtonShaver
	s.Pop = d.Berths() + 1
	habRefused(t, "refit under the passengers", e.CanRefit(home, s, d), "unload first")
	s.Pop = d.Berths()
	if err := e.CanRefit(home, s, d); err != nil {
		t.Errorf("refit with a berth each: %v", err)
	}
}

func TestAbandonWithColonists(t *testing.T) {
	e, home := shipEmpire(t)
	s := e.Spawn(habShip, home.Star, home.PlanetIdx)
	s.Pop = 1
	habRefused(t, "abandon", e.Abandon(s), "unload first")
	if !slices.Contains(e.Ships, s) {
		t.Error("abandoned with a colonist")
	}
	s.Pop = 0
	if err := e.Abandon(s); err != nil {
		t.Error(err)
	}
}

// LosePart caps Pop at the berths left.
func TestLoseHabitat(t *testing.T) {
	s := &Starship{Design: habShip, Pop: 5}
	s.LosePart(slices.Index(habShip.Parts, SpaceHabitat))
	if s.Pop != s.Berths() || s.Pop != Gizmos[SpaceHabitat].Special1 {
		t.Errorf("pop %d, berths %d", s.Pop, s.Berths())
	}
}

// colonistReports is the reports in reps saying colonists were lost.
func colonistReports(reps []Report) []Report {
	return slices.DeleteFunc(slices.Clone(reps), func(r Report) bool { return !strings.Contains(r.Event.Text, "lost aboard") })
}

// A ship shot down with colonists aboard: one more report, its owner's
// alone, after the pair.
func TestColonistsLostInCombat(t *testing.T) {
	g, sh, tg := duel(fightFixture, habShip)
	tg.Shields, tg.Hull, tg.Pop = false, Gizmos[MassBarrageGun].Level, 2
	name := tg.Name
	_, reps, err := fire(g, sh, part(fightFixture, MassBarrageGun), tg)
	if err != nil || g.OwnerOf(tg) != nil || len(reps) != 3 {
		t.Fatalf("%v, %d reports", err, len(reps))
	}
	r := reps[2]
	if r.Empire != 1 || r.Event.Kind != Destroyed || r.Event.Text != "2 colonists lost aboard "+name || r.Event.Star != sh.Star ||
		r.Event.Planet != -1 || r.Event.Other != -1 || r.Event.Item != NoItem || r.Event.Tech != NoTech {
		t.Errorf("report %+v", r)
	}
	for _, x := range reps[:2] {
		if strings.Contains(x.Event.Text, "colonist") {
			t.Errorf("the pair tells the cargo: %q", x.Event.Text)
		}
	}
	// Empty: no report.
	g, sh, tg = duel(fightFixture, habShip)
	tg.Shields, tg.Hull = false, Gizmos[MassBarrageGun].Level
	if _, reps, _ := fire(g, sh, part(fightFixture, MassBarrageGun), tg); len(reps) != 2 {
		t.Errorf("empty habitat: %d reports", len(reps))
	}
}

func TestColonistsLostSelfDestruct(t *testing.T) {
	d := aimer(SelfDestructotron)
	_, e, s, _, _ := aimGame(t, d, foeShip, 5)
	s.Pop = 1
	name := s.Name
	got := colonistReports(useAt(t, e, s, part(d, SelfDestructotron), Target{}))
	if len(got) != 1 || got[0].Empire != e.Index || got[0].Event.Text != "1 colonist lost aboard "+name {
		t.Errorf("reports %+v", got)
	}
}

func TestColonistsLostScrapped(t *testing.T) {
	g, a, _, second := capGame(t)
	home := a.Colonies[0]
	for range 2 * ShipsPerSystem {
		a.Spawn(FixtureColonyShip, home.Star, home.PlanetIdx)
	}
	last := a.Ships[len(a.Ships)-1]
	last.Pop = 3
	settleAt(t, g, 1, second, 1)
	evs := g.EndDay()
	if slices.Contains(a.Ships, last) {
		t.Fatal("fixture: the last ship kept")
	}
	var mine []string
	for _, x := range evs[0] {
		if strings.Contains(x.Text, "lost aboard") {
			mine = append(mine, x.Text)
		}
	}
	if !slices.Equal(mine, []string{"3 colonists lost aboard " + last.Name}) {
		t.Errorf("owner's: %q", mine)
	}
	for _, x := range evs[1] {
		if strings.Contains(x.Text, "colonist") {
			t.Errorf("the other empire hears %q", x.Text)
		}
	}
}
