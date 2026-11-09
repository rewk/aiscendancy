package game

import (
	"math"
	"testing"
)

// The orbital guard (guard.go): who guards an orbit, the refused order,
// the stop short, the same-day race.

// guardFixture is a two-empire game (Snovemdomas, Orfa) and an
// uninhabited planet of empire 0's home star.
func guardFixture(t *testing.T) (g *Game, star, pl int) {
	t.Helper()
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	star = g.Empires[0].Colonies[0].Star
	for i, p := range g.Cluster.Stars[star].System.Planets {
		if p.Owner < 0 {
			return g, star, i
		}
	}
	t.Fatal("no uninhabited planet at the home star: the fixture proves nothing")
	return
}

func TestGuard(t *testing.T) {
	g, star, pl := guardFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	if who, ok := g.Guard(star, pl); ok {
		t.Fatalf("nobody in orbit: guarded by %d", who)
	}
	// In space at the planet's Pos is no guard.
	sp := e1.Spawn(laneShip, star, pl)
	inSpaceAt(sp, sp.Pos)
	if who, ok := g.Guard(star, pl); ok {
		t.Fatalf("a ship in space on the planet: guarded by %d", who)
	}
	// The first ship in orbit, Empires order before Ships order.
	e1.Spawn(laneShip, star, pl)
	if who, ok := g.Guard(star, pl); !ok || who != 1 {
		t.Fatalf("empire 1 in orbit: guard %d %v", who, ok)
	}
	e0.Spawn(laneShip, star, pl)
	if who, ok := g.Guard(star, pl); !ok || who != 0 {
		t.Fatalf("empires 1 and 0 in orbit: guard %d %v, want 0 (Empires order)", who, ok)
	}

	// A colony is guarded by its owner alone: a foreign ship in its
	// orbit guards nothing; a shield of either kind or one of the owner's
	// ships does.
	home := e0.Colonies[0]
	e1.Spawn(laneShip, star, home.PlanetIdx)
	if who, ok := g.Guard(star, home.PlanetIdx); ok {
		t.Fatalf("home, empire 1 in orbit: guard %d, want none", who)
	}
	for _, it := range []Item{OrbitalShield, OrbitalMegaShield} {
		if err := home.Planet.BuildOrbital(OrbitSlots-1, it); err != nil {
			t.Fatal(err)
		}
		if who, ok := g.Guard(star, home.PlanetIdx); !ok || who != 0 {
			t.Errorf("%s: guard %d %v, want the owner 0", it, who, ok)
		}
		home.Planet.DemolishOrbital(OrbitSlots - 1)
	}
	// Another orbital is no shield.
	if err := home.Planet.BuildOrbital(OrbitSlots-1, OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	if who, ok := g.Guard(star, home.PlanetIdx); ok {
		t.Errorf("Orbital Docks: guard %d, want none", who)
	}
	e0.Spawn(laneShip, star, home.PlanetIdx)
	if who, ok := g.Guard(star, home.PlanetIdx); !ok || who != 0 {
		t.Errorf("the owner's ship in orbit: guard %d %v, want the owner 0", who, ok)
	}
}

// Order accepts a GoToPlanet to an orbit another empire guards (the ship
// bounces off it at Hop, the original's way); Barred tells it, and the
// peers survive a Relink (a loaded save).
func TestOrderGuarded(t *testing.T) {
	g, star, pl := guardFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	e1.Spawn(laneShip, star, pl)
	home := e0.Colonies[0]
	mine := e0.Spawn(laneShip, star, home.PlanetIdx)
	theirs := e1.Spawn(laneShip, star, home.PlanetIdx)
	o := Order{Kind: GoToPlanet, Planet: pl}
	if err := e0.Order(mine, o); err != nil {
		t.Fatalf("foreign guard: %v, want the order accepted", err)
	}
	if mine.Order != o || !e0.Barred(star, pl) || e1.Barred(star, pl) {
		t.Errorf("order %+v, barred %v/%v", mine.Order, e0.Barred(star, pl), e1.Barred(star, pl))
	}
	if err := e1.Order(theirs, o); err != nil {
		t.Errorf("own guard: %v", err)
	}
	for _, e := range g.Empires {
		e.peers = nil
	}
	if err := g.Relink(); err != nil {
		t.Fatal(err)
	}
	if !e0.Barred(star, pl) {
		t.Error("after Relink: not barred")
	}
}

// A ship whose GoToPlanet target is guarded flies on, and on coming
// within PlanetClear of it bounces: it stops in space at FreeSpot by its
// clear point (PlanetClear back along the approach: ShipGap from the
// ships in space there), its order KEPT, tried again each day, standing
// PlanetClear off the planet as it turns; once the guard is gone it
// enters the orbit.
func TestStopShort(t *testing.T) {
	g, star, pl := guardFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	c := g.Cluster
	at := c.Stars[star].System.Planets[pl].Pos
	x := e0.Spawn(laneShip, star, e0.Colonies[0].PlanetIdx)
	if err := e0.Order(x, Order{Kind: GoToPlanet, Planet: pl}); err != nil {
		t.Fatal(err)
	}
	clear := planetClear(x.Pos, at) // a straight approach from home
	if d := dist(clear, at); math.Abs(d-PlanetClear) > 1e-9 || dist(x.Pos, clear)+d-dist(x.Pos, at) > 1e-9 {
		t.Fatalf("clear point %v: %v from the planet, not on the approach", clear, d)
	}
	e1.Spawn(laneShip, star, pl)                   // the guard, in orbit: no obstacle
	inSpaceAt(e1.Spawn(laneShip, star, pl), clear) // an obstacle on the clear point
	want := FreeSpot(clear, spaceAt(g.Empires, star, x))
	if want == clear {
		t.Fatal("FreeSpot did not move: the fixture proves nothing")
	}
	_ = want // the stop when a hop lands within PlanetClear: TestHopStopShort
	planet := c.Stars[star].System.Planets[pl]
	for day := 0; dist(x.Pos, planet.Pos) >= PlanetClear+ShipGap; day++ {
		if day > 50 {
			t.Fatalf("not bounced after 50 days: %+v at %v", x.Order, x.Pos)
		}
		g.EndDay()
	}
	for range 3 { // bouncing on, the planet turning: its order kept, PlanetClear out
		g.EndDay()
		if x.Order.Kind != GoToPlanet || x.Order.Planet != pl {
			t.Errorf("bounced: order %+v at %v, want it kept", x.Order, x.Pos)
		}
		if d := dist(x.Pos, planet.Pos); !x.InSpace() || d < PlanetClear-1e-9 || d >= PlanetClear+ShipGap {
			t.Errorf("stopped at %v orbit %d, %v from the planet, want in space PlanetClear out", x.Pos, x.Orbit, d)
		}
	}
	for _, o := range spaceAt(g.Empires, star, x) {
		if d := dist(o, x.Pos); d < ShipGap-1e-9 {
			t.Errorf("%v from a ship in space, under ShipGap", d)
		}
	}
	// The guard gone, the kept order enters the orbit.
	e1.Ships = e1.Ships[:0]
	x.Refill()
	if arrived := x.proceed(c, e0.Day, g.Obstacles(x), e0.Barred); !arrived || x.Orbit != pl {
		t.Errorf("unguarded: arrived %v orbit %d", arrived, x.Orbit)
	}
}

// Hop stopping short spends MoveCost and moves to the clear point, never
// the planet's centre: a hop ending within PlanetClear of the planet
// stops at PlanetClear on the approach; from closer it backs off to it;
// from the centre itself it leaves along +X; at the stop already it does
// nothing; the order is kept throughout.
func TestHopStopShort(t *testing.T) {
	g, star, pl := guardFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	c := g.Cluster
	at := c.Stars[star].System.Planets[pl].Pos
	e1.Spawn(laneShip, star, pl)
	x := e0.Spawn(laneShip, star, e0.Colonies[0].PlanetIdx)
	if r := laneShip.Reach(); r <= 0.1 || r >= 0.3 {
		t.Fatalf("reach %v: the 0.3 start does not end within PlanetClear", r)
	}
	for _, c0 := range []struct{ from, want float64 }{{0.3, 0.2}, {0.1, 0.2}, {0, 0.2}} {
		inSpaceAt(x, Vec3{X: at.X + c0.from, Y: at.Y, Z: at.Z})
		x.Order = Order{Kind: GoToPlanet, Planet: pl}
		pw := x.Power
		want := Vec3{X: at.X + c0.want, Y: at.Y, Z: at.Z}
		if moved, arrived := x.Hop(c, 1, g.Obstacles(x), e0.Barred); !moved || arrived || dist(x.Pos, want) > 1e-12 || !x.InSpace() ||
			x.Order.Kind != GoToPlanet || x.Power != pw-laneShip.MoveCost() {
			t.Errorf("from +%v: moved %v arrived %v pos %v (want %v) order %+v power %d", c0.from, moved, arrived, x.Pos, want, x.Order, x.Power)
		}
	}
	inSpaceAt(x, Vec3{X: at.X + PlanetClear, Y: at.Y, Z: at.Z})
	pw := x.Power
	x.Order = Order{Kind: GoToPlanet, Planet: pl}
	if moved, arrived := x.Hop(c, 1, g.Obstacles(x), e0.Barred); moved || arrived || x.Order.Kind != GoToPlanet || x.Power != pw {
		t.Errorf("there already: moved %v arrived %v order %+v power %d", moved, arrived, x.Order, x.Power)
	}
}

// Same-day race: two empires' ships reaching one free orbit on one day,
// at the close and in a round alike: the first in Empires order enters
// it and guards it, the later one bounces (its order kept); the round's log says
// which move ended in orbit.
func TestGuardRace(t *testing.T) {
	for _, round := range []bool{false, true} {
		g, star, pl := guardFixture(t)
		e0, e1 := g.Empires[0], g.Empires[1]
		at := g.Cluster.Stars[star].System.Planets[pl].Pos
		off := 0.1
		if !round { // the close turns the planet first: start clear of its sweep, two hops out
			at, off = orbitStep(at), 0.3
		}
		home := e0.Colonies[0].PlanetIdx
		a, b := e0.Spawn(laneShip, star, home), e1.Spawn(laneShip, star, home)
		inSpaceAt(a, Vec3{X: at.X + off, Y: at.Y, Z: at.Z})
		bFrom := Vec3{X: at.X - off, Y: at.Y, Z: at.Z}
		inSpaceAt(b, bFrom)
		for _, s := range []*Starship{a, b} {
			if err := g.OwnerOf(s).Order(s, Order{Kind: GoToPlanet, Planet: pl}); err != nil {
				t.Fatal(err)
			}
		}
		var log RoundLog
		if round {
			_, _, log = g.Round(star, nil)
		} else {
			g.EndDay()
		}
		if a.Orbit != pl || b.Orbit >= 0 || b.Order.Kind != GoToPlanet || b.Pos != FreeSpot(planetClear(bFrom, at), spaceAt(g.Empires, star, b)) {
			t.Fatalf("round %v: a orbit %d, b orbit %d order %+v pos %v", round, a.Orbit, b.Orbit, b.Order, b.Pos)
		}
		if who, _ := g.Guard(star, pl); who != 0 {
			t.Errorf("round %v: guard %d", round, who)
		}
		if round {
			if len(log.Moves) != 2 || log.Moves[0].Ship != a || log.Moves[0].Orbit != pl || log.Moves[1].Orbit != -1 {
				t.Errorf("log %+v: want a into orbit %d, b in space", log.Moves, pl)
			}
		}
	}
}
