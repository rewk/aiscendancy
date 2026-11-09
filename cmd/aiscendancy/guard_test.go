package main

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The orbit on the system display: the coloured circle, the one sign of
// it; the ships in orbit hidden, a move into orbit ending at the planet
// (live hop and replay).

// orbitApp is a two-empire game (Snovemdomas, Orfa) with the system
// screen open at the player's home, an uninhabited planet pl there and
// the player's ship x in space 0.1 off it (its day's power), after one
// of the player's ships already in orbit of pl.
func orbitApp(t *testing.T) (a *app, v *systemScreen, pl int, x *game.Starship) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	pl = -1
	for i, p := range g.Cluster.Stars[home.Star].System.Planets {
		if p.Owner < 0 && pl < 0 {
			pl = i
		}
	}
	if pl < 0 {
		t.Fatal("no uninhabited planet at the home star: the fixture proves nothing")
	}
	e.Spawn(game.FixtureColonyShip, home.Star, pl)
	x = e.Spawn(game.FixtureColonyShip, home.Star, home.PlanetIdx)
	at := e.Planet(home.Star, pl).Pos
	x.Orbit, x.Pos = -1, game.Vec3{X: at.X + 0.1, Y: at.Y, Z: at.Z}
	x.Refill()
	a = testApp(g)
	v = newSystemScreen(home.Star)
	a.push(v)
	return a, v, pl, x
}

// drawnBody is ship sh's body in bs.
func drawnBody(t *testing.T, bs []body, sh *game.Starship) body {
	t.Helper()
	for _, b := range bs {
		if b.kind == shipBody && b.ship == sh {
			return b
		}
	}
	t.Fatalf("%s not drawn", sh.Name)
	return body{}
}

// The circle: on a planet a ship guards, in its empire's colour; on the
// home a foreign ship orbits unguarded, that ship's colour; with an
// Orbital Shield there, the owner's whoever orbits; none on a planet
// with an empty orbit, a ship, or without a game.
func TestGuardRing(t *testing.T) {
	a, v, pl, _ := orbitApp(t)
	g := a.g
	home := a.e.Colonies[0]
	other := g.Empires[1]
	g.SetColour(1, game.Orange)
	planet := func(i int) *body {
		return &body{kind: planetBody, idx: i, planet: g.Cluster.Stars[v.star].System.Planets[i]}
	}
	if c, ok := guardRing(g, nil, v.star, planet(pl)); !ok || c != empireColour(colourOf(g, 0)) {
		t.Errorf("player's ship in orbit: %v %v, want the player's colour", c, ok)
	}
	other.Spawn(game.FixtureColonyShip, v.star, home.PlanetIdx)
	for _, sh := range a.e.Ships {
		if sh.Orbit == home.PlanetIdx {
			sh.Orbit = -1 // the player's own ships out of the home's orbit
		}
	}
	if _, ok := g.Guard(v.star, home.PlanetIdx); ok {
		t.Fatal("empire 1's ship guards the player's home: the fallback is not walked")
	}
	if c, ok := guardRing(g, nil, v.star, planet(home.PlanetIdx)); !ok || c != empireColour(game.Orange) {
		t.Errorf("empire 1 in orbit of the home: %v %v, want Orange", c, ok)
	}
	if err := home.Planet.BuildOrbital(game.OrbitSlots-1, game.OrbitalShield); err != nil {
		t.Fatal(err)
	}
	if c, ok := guardRing(g, nil, v.star, planet(home.PlanetIdx)); !ok || c != empireColour(colourOf(g, 0)) {
		t.Errorf("shielded home: %v %v, want the owner's colour", c, ok)
	}
	empty := 0
	for i := range g.Cluster.Stars[v.star].System.Planets {
		_, guarded := g.Guard(v.star, i)
		orbited := false
		for _, e := range g.Empires {
			for _, sh := range e.Ships {
				orbited = orbited || sh.Star == v.star && sh.Lane < 0 && sh.Orbit == i
			}
		}
		if !guarded && !orbited {
			empty++
			if _, ok := guardRing(g, nil, v.star, planet(i)); ok {
				t.Errorf("planet %d with an empty orbit ringed", i)
			}
		}
	}
	if empty == 0 {
		t.Error("no planet with an empty orbit: the none case is not walked")
	}
	if _, ok := guardRing(g, nil, v.star, &body{kind: shipBody, ship: a.e.Ships[0]}); ok {
		t.Error("a ship ringed")
	}
	if _, ok := guardRing(nil, nil, v.star, planet(pl)); ok {
		t.Error("no game: ringed")
	}
}

// A ship in orbit is a body (the lists show it) marked orbiting, at its
// planet, hidden: placeAll leaves it out (not drawn, not picked); one in
// space is placed at its Pos.
func TestOrbitHidden(t *testing.T) {
	_, v, _, x := orbitApp(t)
	placed := map[*game.Starship]bool{}
	for _, p := range v.placeAll() {
		if p.kind == shipBody {
			placed[p.ship] = true
		}
	}
	n := 0
	for _, b := range v.bodies {
		if b.kind != shipBody {
			continue
		}
		in := b.ship.Orbit >= 0
		if b.orbiting != in || in && (b.orbit != b.ship.Orbit || b.pos != v.sys.Planets[b.orbit].Pos) {
			t.Errorf("%s orbit %d: orbiting %v on %d at %v", b.ship.Name, b.ship.Orbit, b.orbiting, b.orbit, b.pos)
		}
		if placed[b.ship] == in {
			t.Errorf("%s orbit %d: placed %v", b.ship.Name, b.ship.Orbit, placed[b.ship])
		}
		if in {
			n++
		}
	}
	if n == 0 {
		t.Fatal("no ship in orbit: nothing hidden")
	}
	if b := drawnBody(t, v.bodies, x); b.orbiting || b.pos != x.Pos || !placed[x] {
		t.Errorf("in space: orbiting %v at %v placed %v, want its Pos %v", b.orbiting, b.pos, placed[x], x.Pos)
	}
}

// A live hop into orbit tweens to the planet, the ship drawn until the
// anim is over, then hidden, even while an older anim of the ship ran
// (Repeat Move); the order dropped it at once.
func TestHopIntoOrbitEndsAtPlanet(t *testing.T) {
	a, v, pl, x := orbitApp(t)
	v.sel = x
	v.anim = anim{ship: x, from: x.Pos, to: x.Pos, t0: v.tick, day: a.e.Day}
	if err := v.order(a.e, game.Order{Kind: game.GoToPlanet, Planet: pl}); err != nil {
		t.Fatal(err)
	}
	if x.Orbit != pl || v.anim.ship != x {
		t.Fatalf("orbit %d anim %v: the hop did not enter the orbit", x.Orbit, v.anim.ship)
	}
	if want := a.e.Planet(v.star, pl).Pos; v.anim.to != want {
		t.Errorf("anim to %v, want the planet at %v", v.anim.to, want)
	}
	v.rebuild()
	if b := drawnBody(t, v.bodies, x); orbitHidden(&b) || v.sel != nil {
		t.Errorf("mid-anim: hidden %v, sel %v", orbitHidden(&b), v.sel)
	}
	v.tick = v.anim.t0 + hopTicks
	v.advance(a.e)
	v.rebuild()
	if b := drawnBody(t, v.bodies, x); !orbitHidden(&b) || v.sel != nil {
		t.Errorf("anim over: hidden %v, sel %v", orbitHidden(&b), v.sel)
	}
}

// A replayed move ending in orbit (Move.Orbit) ends at the planet, where
// the live view keeps the ship after it (hidden), flying (in space) until
// then.
func TestReplayIntoOrbitEndsAtPlanet(t *testing.T) {
	a, v, pl, x := orbitApp(t)
	cast := v.replayCast()
	from := x.Pos
	x.Orbit, x.Pos = pl, a.e.Planet(v.star, pl).Pos // the model after the round
	logs := []game.RoundLog{{Star: v.star, Moves: []game.Move{{Ship: x, From: from, To: x.Pos, Orbit: pl}}}}
	v.startReplay(a.e, cast, logs)
	if v.replay == nil {
		t.Fatal("no replay")
	}
	want := drawnBody(t, shipBodies(a.g, v.star, nil, game.Vec3{}), x).pos
	v.tick = v.replay.t0 + moveTicks/2
	if b := drawnBody(t, v.replayBodies(), x); b.orbiting {
		t.Errorf("mid-move: orbiting at %v", b.pos)
	}
	v.tick = v.replay.t0 + moveTicks
	if b := drawnBody(t, v.replayBodies(), x); b.pos != want || !orbitHidden(&b) || b.orbit != pl || want != x.Pos {
		t.Errorf("move's end: %v hidden %v on %d, want at the planet %v (live %v)", b.pos, orbitHidden(&b), b.orbit, x.Pos, want)
	}
}

// While a replay runs the rings are the ones snapped before its rounds
// (a shield shot down mid-series keeps its ring until the end); a replay
// already running keeps its snap; without a replay the model's again.
func TestRingsHeldByReplay(t *testing.T) {
	a, v, pl, _ := orbitApp(t)
	g := a.g
	pb := func() *body {
		for i := range v.bodies {
			if b := &v.bodies[i]; b.kind == planetBody && b.idx == pl {
				return b
			}
		}
		t.Fatal("planet not drawn")
		return nil
	}
	want, ok := guardRing(g, a.e, v.star, pb())
	if !ok {
		t.Fatal("no ring to hold")
	}
	v.snapRings()
	v.replay = &replay{}
	for _, sh := range a.e.Ships {
		if sh.Orbit == pl {
			sh.Orbit = -1 // the guard gone, as the round left the model
		}
	}
	if _, live := guardRing(g, a.e, v.star, pb()); live {
		t.Fatal("the model still rings the planet")
	}
	v.snapRings() // the next round's step, mid-replay
	if c, ok := v.ringOf(pb()); !ok || c != want {
		t.Errorf("during the replay: %v %v, want %v", c, ok, want)
	}
	v.replay = nil
	if err := v.Update(a); err != nil {
		t.Fatal(err)
	}
	if _, ok := v.ringOf(pb()); ok || v.rings != nil {
		t.Errorf("after the replay: ring %v, snap %v", ok, v.rings)
	}
}
