package game

import (
	"math"
	"testing"
)

// angleOf is p's angle about the Y axis, from +X towards +Z.
func angleOf(p Vec3) float64 { return math.Atan2(p.Z, p.X) }

// A planet after N steps sits N·2π/OrbitPeriod(r) further clockwise in
// atan2(Z, X), i.e. counterclockwise seen from +Y; r and Y kept; the
// innermost radius comes round in OrbitDays0 days; the outer is slower.
func TestOrbitStep(t *testing.T) {
	if p := OrbitPeriod(OrbitR0); math.Abs(p-OrbitDays0) > 1e-12 {
		t.Fatalf("period at OrbitR0 %v, want %d", p, OrbitDays0)
	}
	if OrbitPeriod(0.9) <= OrbitPeriod(0.5) {
		t.Fatal("an outer orbit is no slower")
	}
	for _, p0 := range []Vec3{{X: 0.25, Y: 0.1}, {X: -0.3, Y: 0.55, Z: 0.4}, {X: 0.1, Y: 0.3, Z: -0.85}} {
		r := math.Hypot(p0.X, p0.Z)
		for _, n := range []int{1, 7, 45} {
			p := p0
			for range n {
				p = orbitStep(p)
			}
			want := math.Remainder(angleOf(p0)-float64(n)*2*math.Pi/OrbitPeriod(r), 2*math.Pi)
			if d := math.Remainder(angleOf(p)-want, 2*math.Pi); math.Abs(d) > 1e-9 {
				t.Errorf("%v after %d days: angle %.6f, want %.6f", p0, n, angleOf(p), want)
			}
			if math.Abs(math.Hypot(p.X, p.Z)-r) > 1e-12 || p.Y != p0.Y {
				t.Errorf("%v after %d days: r %.15f y %v", p0, n, math.Hypot(p.X, p.Z), p.Y)
			}
		}
	}
	// Seen from above (+X right, +Z towards the viewer): +X goes to −Z.
	if p := orbitStep(Vec3{X: 0.25}); p.Z >= 0 {
		t.Errorf("+X stepped to %v: not counterclockwise from above", p)
	}
	p := Vec3{X: OrbitR0, Y: 0.2}
	for range OrbitDays0 {
		p = orbitStep(p)
	}
	if dist(p, Vec3{X: OrbitR0, Y: 0.2}) > 1e-9 {
		t.Errorf("innermost after %d days at %v", OrbitDays0, p)
	}
}

// The close turns every planet once, before the ships move, the lanes
// staying; a ship in orbit is at its planet's new Pos, its order kept.
func TestEndDayTurnsPlanets(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0].Star
	s := e.Spawn(laneShip, home, 0)
	var before []Vec3
	var lanes []Vec3
	for _, st := range g.Cluster.Stars {
		for _, p := range st.System.Planets {
			before = append(before, p.Pos)
		}
		for _, l := range st.System.Lanes {
			lanes = append(lanes, l.Pos)
		}
	}
	g.EndDay()
	i, j := 0, 0
	for _, st := range g.Cluster.Stars {
		for _, p := range st.System.Planets {
			if p.Pos != orbitStep(before[i]) {
				t.Fatalf("%s at %v, want %v", p.Name, p.Pos, orbitStep(before[i]))
			}
			i++
		}
		for _, l := range st.System.Lanes {
			if l.Pos != lanes[j] {
				t.Fatalf("lane endpoint moved: %v -> %v", lanes[j], l.Pos)
			}
			j++
		}
	}
	if pl := g.Cluster.Stars[home].System.Planets[0]; s.Orbit != 0 || s.Pos != pl.Pos {
		t.Errorf("orbiting ship at %v orbit %d, planet at %v", s.Pos, s.Orbit, pl.Pos)
	}
}

// A ship in space on a planet's swept path is pushed PlanetClear off its
// new Pos, away from it, its order kept; one off the path stays, and so
// does one within PlanetClear of the old Pos the planet moved away from;
// two pushed keep ShipGap between them and PlanetClear off the planet.
func TestTurnPlanetsPush(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0].Star
	pl := g.Cluster.Stars[home].System.Planets[1]
	old := pl.Pos
	next := orbitStep(old)
	mid := lerp(old, next, 0.5)
	far := Vec3{X: -old.X, Y: old.Y, Z: -old.Z} // the other side of the star
	// Behind the old Pos and up, clear of where the first lands.
	back := unit(next, old)
	behind := along(old, unit(Vec3{}, Vec3{back.X, back.Y + 1, back.Z}), PlanetClear-0.01)
	if dist(behind, next) < PlanetClear {
		t.Fatalf("fixture: a step of %.3f", dist(old, next))
	}
	a, b, c := e.Spawn(laneShip, home, 0), e.Spawn(laneShip, home, 0), e.Spawn(laneShip, home, 0)
	d := e.Spawn(laneShip, home, 0)
	d.Orbit, d.Pos = -1, behind
	a.Orbit, a.Pos = -1, mid
	a.Order = Order{Kind: GoToPoint, Pos: Vec3{X: 1}}
	b.Orbit, b.Pos = -1, lerp(old, next, 0.55)
	c.Orbit, c.Pos = -1, far
	g.turnPlanets()
	if pl.Pos != next {
		t.Fatalf("planet at %v, want %v", pl.Pos, next)
	}
	if d := dist(a.Pos, next); math.Abs(d-PlanetClear) > 1e-9 {
		t.Errorf("first pushed to %.6f off the planet, want PlanetClear %.2f", d, PlanetClear)
	}
	// Away from the new Pos: along mid − next, which points back along the path.
	if u, v := unit(next, a.Pos), unit(next, mid); dist(u, v) > 1e-9 {
		t.Errorf("first pushed along %v, want %v", u, v)
	}
	if a.Order != (Order{Kind: GoToPoint, Pos: Vec3{X: 1}}) || !a.InSpace() {
		t.Errorf("first: order %+v orbit %d lane %d", a.Order, a.Orbit, a.Lane)
	}
	if d := dist(b.Pos, next); d < PlanetClear-1e-9 {
		t.Errorf("second %.6f off the planet", d)
	}
	if d := dist(a.Pos, b.Pos); d < ShipGap-1e-9 {
		t.Errorf("pushed ships %.6f apart, want ShipGap", d)
	}
	if c.Pos != far {
		t.Errorf("a ship off the path moved: %v -> %v", far, c.Pos)
	}
	if d.Pos != behind {
		t.Errorf("a ship the planet moved away from moved: %v -> %v", behind, d.Pos)
	}
}

// swept: within PlanetClear of the end, or passed over mid-path (a step
// longer than PlanetClear, as a short OrbitDays0 would make), never only
// near the start.
func TestSwept(t *testing.T) {
	a, b := Vec3{}, Vec3{X: 1}
	for _, c := range []struct {
		p    Vec3
		want bool
	}{
		{Vec3{X: 0.9}, true},                        // near the end
		{Vec3{X: 1.1, Z: 0.1}, true},                // just past it
		{Vec3{X: 0.5, Z: PlanetClear - 0.01}, true}, // passed over mid-path
		{Vec3{X: 0.5, Z: PlanetClear + 0.01}, false},
		{Vec3{X: -0.1}, false}, // behind the start: moved away from
		{Vec3{X: 0, Z: 0.1}, false},
	} {
		if got := swept(c.p, a, b); got != c.want {
			t.Errorf("%v: swept %v, want %v", c.p, got, c.want)
		}
	}
}

// A ship on the planet's new Pos itself goes on along the planet's motion.
func TestTurnPlanetsPushOnTop(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0].Star
	pl := g.Cluster.Stars[home].System.Planets[1]
	old := pl.Pos
	next := orbitStep(old)
	s := e.Spawn(laneShip, home, 0)
	s.Orbit, s.Pos = -1, next
	g.turnPlanets()
	if want := along(next, unit(old, next), PlanetClear); dist(s.Pos, want) > 1e-9 {
		t.Errorf("pushed to %v, want %v", s.Pos, want)
	}
}

// Every pair of planets of every system keeps its bodies OrbitMargin
// apart where their orbits come closest, heights within [0, 0.8]: every
// density's clusters, and games whose homes replaced planet 0.
func TestOrbitGap(t *testing.T) {
	check := func(what string, sys *StarSystem) {
		for k, p := range sys.Planets {
			if p.Pos.Y < 0 || p.Pos.Y > 0.8 {
				t.Errorf("%s %s: height %.3f", what, p.Name, p.Pos.Y)
			}
			for _, q := range sys.Planets[:k] {
				need := PlanetRadius(p.Size) + PlanetRadius(q.Size) + OrbitMargin
				dr := math.Hypot(p.Pos.X, p.Pos.Z) - math.Hypot(q.Pos.X, q.Pos.Z)
				if d := math.Hypot(dr, p.Pos.Y-q.Pos.Y); d < need-1e-9 {
					t.Errorf("%s: %s and %s %.4f apart at closest, want %.4f", what, q.Name, p.Name, d, need)
				}
			}
		}
	}
	seeds := uint64(200)
	if testing.Short() {
		seeds = 20
	}
	for d := Vacuous; d <= VeryDense; d++ {
		for seed := uint64(1); seed <= seeds; seed++ {
			for _, st := range NewCluster(seed, d).Stars {
				check(d.String(), st.System)
			}
		}
	}
	for seed := uint64(1); seed <= 20; seed++ {
		g := NewGame(seed, []Species{Govorom, Orfa, Capelons, Minions}, Average)
		for _, e := range g.Empires {
			check("game home", g.Cluster.Stars[e.Colonies[0].Star].System)
		}
	}
}
