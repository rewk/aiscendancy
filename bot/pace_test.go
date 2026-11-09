package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// settleables splits the settleable planets the empire knows into those
// of Size ≥ floor (big) and below (small).
func settleables(b *Bot, floor int) (big, small []place) {
	for st, star := range b.E.Cluster.Stars {
		if !b.E.Knows(st) {
			continue
		}
		for pl, p := range star.System.Planets {
			switch {
			case !b.settleable(st, pl):
			case p.Size >= floor:
				big = append(big, place{st, pl})
			default:
				small = append(small, place{st, pl})
			}
		}
	}
	return big, small
}

// FoundFloor: while a settleable planet of Size ≥ floor is known (claimed
// or not), a smaller one is not foundable, not counted by targets, not
// colonised by a ship in its orbit; once every big one is settled the
// floor drops to 1 and the small ones are foundable again. Floor 1 founds
// every size, the fixed policy holds at foundFloor.
func TestFoundFloor(t *testing.T) {
	const floor = 4
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b := NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.FoundFloor = floor })})[0]
	e := b.E
	for st := range g.Cluster.Stars { // known stars until both kinds exist
		if big, small := settleables(b, floor); len(big) > 0 && len(small) > 0 {
			break
		}
		e.Visit(st)
	}
	big, small := settleables(b, floor)
	if len(big) == 0 || len(small) == 0 {
		t.Fatalf("%d big and %d small settleable planets: the fixture proves nothing", len(big), len(small))
	}
	check := func(when string, wantFloor int) {
		t.Helper()
		claimed := b.claimed()
		if b.floor != wantFloor {
			t.Errorf("%s: floor %d, want %d", when, b.floor, wantFloor)
		}
		for st, star := range g.Cluster.Stars {
			n := 0
			for pl, p := range star.System.Planets {
				want := b.settleable(st, pl) && p.Size >= wantFloor
				if b.foundable(st, pl) != want {
					t.Errorf("%s: star %d planet %d (size %d) foundable %v", when, st, pl, p.Size, !want)
				}
				if want && !claimed[place{st, pl}] {
					n++
				}
			}
			if got := b.targets(st, claimed); got != max(0, n-b.inbound[st]) {
				t.Errorf("%s: star %d targets %d, want %d", when, st, got, max(0, n-b.inbound[st]))
			}
		}
	}
	check("floor 4, big known", floor)
	// A ship in a small planet's orbit does not colonise it.
	sm := small[0]
	cs := e.Spawn(game.FixtureColonyShip, sm.star, sm.planet)
	if b.planShip(cs, b.claimed()); cs.Order.Kind == game.Colonise {
		t.Error("floor 4: a ship colonises the small planet it orbits")
	}
	e.Ships = e.Ships[:len(e.Ships)-1]
	// A big one claimed still holds the floor up.
	bg := big[0]
	cs = e.Spawn(game.FixtureColonyShip, bg.star, bg.planet)
	cs.Order = game.Order{Kind: game.GoToPlanet, Planet: bg.planet}
	if b.claimed()[bg] != true {
		t.Fatal("the GoToPlanet does not claim the big planet")
	}
	check("floor 4, the big one claimed", floor)
	e.Ships = e.Ships[:len(e.Ships)-1]
	// Every big one settled: the floor drops.
	for i, p := range big {
		s := e.Spawn(game.FixtureColonyShip, p.star, p.planet)
		if err := e.Found(s); err != nil {
			t.Fatal(err)
		}
		e.Ships = e.Ships[:len(e.Ships)-1]
		if i < len(big)-1 {
			check("floor 4, a big one left", floor)
		}
	}
	check("every big one settled", 1)
	if !b.foundable(sm.star, sm.planet) {
		t.Error("every big one settled: the small planet not foundable")
	}

	// Every star known, nothing settled: the fixed policy holds at
	// foundFloor, a FoundFloor 1 genome at 1; the planets under the floor
	// are not foundable, those at or over it are.
	for _, tc := range []struct {
		cfg   Config
		floor int
	}{{Config{NoResearch: true}, foundFloor}, {Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.FoundFloor = 1 })}, 1}} {
		g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
		b := NewAll(g, tc.cfg)[0]
		for st := range g.Cluster.Stars {
			b.E.Visit(st)
		}
		b.claimed()
		big, small := settleables(b, tc.floor)
		if len(big) == 0 {
			t.Fatalf("genome %v: no settleable planet of Size >= %d: the fixture proves nothing", tc.cfg.Genome != nil, tc.floor)
		}
		if b.floor != tc.floor {
			t.Errorf("genome %v: floor %d, want %d", tc.cfg.Genome != nil, b.floor, tc.floor)
		}
		for _, p := range big {
			if !b.foundable(p.star, p.planet) {
				t.Errorf("genome %v: planet %v of Size >= %d not foundable", tc.cfg.Genome != nil, p, tc.floor)
			}
		}
		for _, p := range small {
			if b.foundable(p.star, p.planet) {
				t.Errorf("genome %v: planet %v under Size %d foundable", tc.cfg.Genome != nil, p, tc.floor)
			}
		}
		if tc.floor > 1 && len(small) == 0 {
			t.Errorf("genome %v: no planet under Size %d: the floor is not exercised", tc.cfg.Genome != nil, tc.floor)
		}
	}
}

// inFlightFixture: seed 1, one empire, every star known (spare positive),
// every tech known, the home and a second colony each with a Shipyard
// and a free worker, a loaded colony ship at home under a GoToPlanet
// order, InFlight cap.
func inFlightFixture(t *testing.T, cap float64) (b *Bot, cols []*game.Colony, d game.Design) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b = NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.InFlight = cap })})[0]
	e := b.E
	e.Research = allKnown()
	for st := range g.Cluster.Stars {
		e.Visit(st)
	}
	home := e.Colonies[0]
	home.Planet.Orbit[0] = game.Shipyard
	home.Pop = home.Busy() + 1
	second := secondColony(t, b, home)
	s := e.Spawn(game.FixtureColonyShip, home.Star, home.PlanetIdx)
	s.Order = game.Order{Kind: game.GoToPlanet, Planet: home.PlanetIdx} // only its Kind matters to inFlight
	d, ok := game.ColonyShip(e.Research, e.Research.AllowsGizmo(game.StarLaneDrive))
	if !ok {
		t.Fatal("no colony ship design with every tech")
	}
	return b, []*game.Colony{home, second}, d
}

func colonyShipStarted(col *game.Colony) bool {
	d := building(col)
	return d != nil && d.Colonizers() > 0
}

// InFlight: the Colonizers aboard plus those being built, plus the new
// design's, must fit the cap; a ship build() starts counts at once, so a
// cap with room for one starts one of two shipyards' colony ships.
func TestInFlightCap(t *testing.T) {
	b, cols, d := inFlightFixture(t, 50)
	aboard := game.FixtureColonyShip.Colonizers()
	claimed := b.claimed()
	if b.inFlight != aboard || b.spare < 2*d.Colonizers() {
		t.Fatalf("inFlight %d (want %d), spare %d: the fixture proves nothing", b.inFlight, aboard, b.spare)
	}
	// Idle, it still counts while it can go (planShip sends it after
	// build); a driveless one at a star with no target blocks nothing.
	idle := b.E.Ships[len(b.E.Ships)-1]
	idle.Order = game.Order{}
	if b.claimed(); b.inFlight != aboard {
		t.Errorf("idle loaded ship able to go: inFlight %d, want %d", b.inFlight, aboard)
	}
	keep, star := idle.Design, idle.Star
	idle.Design = game.Design{Name: "Stranded", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer, game.Colonizer}}
	for st := range b.E.Cluster.Stars {
		if b.targets(st, claimed) == 0 {
			idle.Star = st
			break
		}
	}
	if b.claimed(); b.targets(idle.Star, claimed) != 0 || b.inFlight != 0 {
		t.Errorf("idle driveless ship, %d targets at its star: inFlight %d, want 0", b.targets(idle.Star, claimed), b.inFlight)
	}
	idle.Design, idle.Star = keep, star
	b.E.Ships[len(b.E.Ships)-1].Order = game.Order{Kind: game.GoToPlanet, Planet: cols[0].PlanetIdx}
	claimed = b.claimed()
	for _, col := range cols {
		b.planColony(1, col, claimed)
	}
	if !colonyShipStarted(cols[0]) || !colonyShipStarted(cols[1]) {
		t.Fatalf("cap 50: projects %+v, %+v, want two colony ships", cols[0].Project, cols[1].Project)
	}
	if b.claimed(); b.inFlight != aboard+2*d.Colonizers() {
		t.Errorf("two colony ships building: inFlight %d, want %d", b.inFlight, aboard+2*d.Colonizers())
	}

	for _, tc := range []struct {
		cap  int
		want int // colony ships started
	}{{aboard + d.Colonizers() - 1, 0}, {aboard + d.Colonizers(), 1}, {aboard + 2*d.Colonizers() - 1, 1}, {aboard + 2*d.Colonizers(), 2}} {
		b, cols, _ := inFlightFixture(t, float64(tc.cap))
		claimed := b.claimed()
		for _, col := range cols {
			b.planColony(1, col, claimed)
		}
		n := 0
		for _, col := range cols {
			if colonyShipStarted(col) {
				n++
			}
		}
		if n != tc.want {
			t.Errorf("cap %d: %d colony ships started, want %d", tc.cap, n, tc.want)
		}
	}
}

func TestPaceGenesClamp(t *testing.T) {
	g := Neutral()
	if g.FoundFloor != foundFloor || g.InFlight != inFlightCap {
		t.Errorf("Neutral: FoundFloor %v InFlight %v, want %d and %d (the fixed policy's)", g.FoundFloor, g.InFlight, foundFloor, inFlightCap)
	}
	g.FoundFloor, g.InFlight = 0, 0
	g.Clamp()
	if g.FoundFloor != 1 || g.InFlight != 1 {
		t.Errorf("low: FoundFloor %v InFlight %v, want 1 and 1", g.FoundFloor, g.InFlight)
	}
	g.FoundFloor, g.InFlight = 9, 99
	g.Clamp()
	if g.FoundFloor != game.MaxSize || g.InFlight != 50 {
		t.Errorf("high: FoundFloor %v InFlight %v, want %d and 50", g.FoundFloor, g.InFlight, game.MaxSize)
	}
}

// InFlight sees the colony ship that came out of a shipyard last night:
// idle in orbit with somewhere to go, it is dispatched by this Act's
// planShip, AFTER build has run. Counted out, the cap (its own load) let
// build start a second colony ship the same day.
func TestInFlightFreshIdleShip(t *testing.T) {
	aboard := game.FixtureColonyShip.Colonizers()
	b, cols, _ := inFlightFixture(t, float64(aboard))
	s := b.E.Ships[len(b.E.Ships)-1]
	s.Order = game.Order{} // as Spawn leaves a built ship
	b.Act()
	if s.Order.Kind == game.NoOrder && s.Lane < 0 {
		t.Fatal("the fresh ship was not dispatched: the fixture proves nothing")
	}
	for _, col := range cols {
		if colonyShipStarted(col) {
			t.Errorf("cap %d with %d aboard a ship sent today: a colony ship started at %s", aboard, aboard, col.Planet.Name)
		}
	}
}
