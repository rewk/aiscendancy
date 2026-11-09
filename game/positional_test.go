package game

import (
	"slices"
	"strings"
	"testing"
)

// The positional and lane specials (docs/rules.md "Ship specials", lot
// 3): each effect's geometry (distance and direction, the obstacles, the
// SystemReach clamp, the planets' PlanetClear, the orbit left), its own
// refusal, the area effects' victims (in space only, moved one after
// another against live positions), the lane target, the Lane Blocker's
// hold both ways and its expiry, the Destabilizer, the Magnetron, the
// Endoscope.

// openAt is a point of seed 1's home system (star 16) clear of its
// planets: every planet there has Y ≥ 0.03, so Y ≤ −0.3 is ≥ 0.33 away.
func openAt(x, z float64) Vec3 { return Vec3{X: x, Y: -0.6, Z: z} }

// sameAt reports a within 1e-9 of b.
func sameAt(a, b Vec3) bool { return dist(a, b) < 1e-9 }

// clearOfPlanets fails the test when p is within PlanetClear of a planet
// of star: a fixture whose expectations assume none.
func clearOfPlanets(t *testing.T, g *Game, star int, p Vec3) {
	t.Helper()
	for _, pl := range g.Cluster.Stars[star].System.Planets {
		if dist(p, pl.Pos) < PlanetClear {
			t.Fatalf("fixture point %v within PlanetClear of %s", p, pl.Name)
		}
	}
}

// posGame is aimGame with the user at at and the foe at foe, both in
// space.
func posGame(t *testing.T, d Design, at, foeAt Vec3) (g *Game, e *Empire, s, foe *Starship, tg Target) {
	t.Helper()
	g, e, s, foe, tg = aimGame(t, d, foeShip, 0.1)
	inSpaceAt(s, at)
	inSpaceAt(foe, foeAt)
	return
}

// extra spawns a ship of foeShip for empire i in space at p.
func extra(g *Game, i int, p Vec3) *Starship {
	home := g.Empires[0].Colonies[0]
	x := g.Empires[i].Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(x, p)
	return x
}

// The Positron Bouncer pushes the target Special1 × RangeUnit straight
// away from the user, out of the way of a ship there (ShipGap short),
// short of a planet (PlanetClear), clamped at SystemReach, its order
// kept; a target on the rim pushed outward does not move, the use spent.
func TestPositronBouncer(t *testing.T) {
	d := aimer(PositronBouncer)
	k := part(d, PositronBouncer)
	L := float64(Gizmos[PositronBouncer].Special1) * RangeUnit
	a := openAt(-0.5, 0)
	g, e, s, foe, tg := posGame(t, d, a, openAt(-0.4, 0))
	clearOfPlanets(t, g, s.Star, openAt(-0.4+L, 0))
	o := Order{Kind: GoToPoint, Pos: openAt(-0.4, 0.3)}
	foe.Order = o
	useAt(t, e, s, k, tg)
	if want := openAt(-0.4+L, 0); !sameAt(foe.Pos, want) || foe.Order != o || foe.Orbit != -1 {
		t.Errorf("pushed to %v order %v, want %v", foe.Pos, foe.Order, want)
	}

	// A ship at the destination: the target stops ShipGap short.
	s.Refill()
	inSpaceAt(foe, openAt(-0.4, 0))
	block := extra(g, 1, openAt(-0.4+L, 0))
	useAt(t, e, s, k, tg)
	if want := openAt(-0.4+L-ShipGap, 0); !sameAt(foe.Pos, want) {
		t.Errorf("blocked: at %v, want %v", foe.Pos, want)
	}
	g.Destroy(block)

	// The clamp: the destination beyond SystemReach lands on the rim.
	s.Refill()
	inSpaceAt(s, openAt(0.4, 0))
	inSpaceAt(foe, openAt(0.6, 0))
	useAt(t, e, s, k, tg)
	far := openAt(0.6+L, 0)
	if r := dist(foe.Pos, Vec3{}); r > SystemReach+1e-9 || r < SystemReach-1e-9 || dist(foe.Pos, Vec3{far.X * SystemReach / dist(far, Vec3{}), far.Y * SystemReach / dist(far, Vec3{}), 0}) > 1e-9 {
		t.Errorf("clamped to %v (radius %v)", foe.Pos, r)
	}

	// On the rim, pushed outward: not moved, the use spent.
	s.Refill()
	inSpaceAt(s, Vec3{Y: -SystemReach + 0.3})
	inSpaceAt(foe, Vec3{Y: -SystemReach})
	reps := useAt(t, e, s, k, tg)
	if !sameAt(foe.Pos, Vec3{Y: -SystemReach}) || s.Shots[k] != 1 || !strings.Contains(reps[0].Event.Text, "does not move") {
		t.Errorf("rim: at %v shots %v %q", foe.Pos, s.Shots, reps[0].Event.Text)
	}

	// A planet at the destination: the target stops PlanetClear short.
	p := g.Cluster.Stars[s.Star].System.Planets[1].Pos
	f0 := Vec3{p.X - L, p.Y, p.Z}
	s.Refill()
	inSpaceAt(s, Vec3{f0.X - 0.1, f0.Y, f0.Z})
	inSpaceAt(foe, f0)
	useAt(t, e, s, k, tg)
	if want := (Vec3{p.X - PlanetClear, p.Y, p.Z}); !sameAt(foe.Pos, want) {
		t.Errorf("planet: at %v, want %v", foe.Pos, want)
	}

	// A target in orbit is refused (targetCheck).
	s.Refill()
	foe.Orbit = 0
	useRefused(t, e, s, k, tg, "fire at the planet")
}

// The Tractor Beam pulls a hostile or an own ship Special1 toward the
// user, never closer than ShipGap; refused alongside, at a target in
// orbit and from orbit.
func TestTractorBeam(t *testing.T) {
	d := aimer(GizmoTractorBeam)
	k := part(d, GizmoTractorBeam)
	L := float64(Gizmos[GizmoTractorBeam].Special1) * RangeUnit
	a := openAt(-0.5, 0)
	g, e, s, foe, tg := posGame(t, d, a, openAt(-0.5+L+0.15, 0))
	useAt(t, e, s, k, tg)
	if want := openAt(-0.5+0.15, 0); !sameAt(foe.Pos, want) {
		t.Errorf("pulled to %v, want %v", foe.Pos, want)
	}
	s.Refill()
	useAt(t, e, s, k, tg) // 0.15 left: stops ShipGap from the user
	if want := openAt(-0.5+ShipGap, 0); !sameAt(foe.Pos, want) {
		t.Errorf("pulled to %v, want %v", foe.Pos, want)
	}
	s.Refill()
	useRefused(t, e, s, k, tg, "already alongside")

	mate := extra(g, 0, openAt(-0.5, 0.5))
	mine := Target{Kind: ShipTarget, Mine: mate}
	useAt(t, e, s, k, mine)
	if want := openAt(-0.5, ShipGap); !sameAt(mate.Pos, want) {
		t.Errorf("own ship pulled to %v, want %v", mate.Pos, want)
	}
	s.Refill()
	inSpaceAt(mate, openAt(-0.5, 0.5))
	mate.Orbit = 0
	useRefused(t, e, s, k, mine, "is in orbit")
	mate.Orbit = -1
	s.Orbit = 0
	useRefused(t, e, s, k, mine, "in orbit: leave it first")
}

// The Hyperswapper swaps the user and the target exactly, a hostile or
// an own ship; refused at a target in orbit, from orbit.
func TestHyperswapper(t *testing.T) {
	d := aimer(Hyperswapper)
	k := part(d, Hyperswapper)
	a, b := openAt(-0.5, 0), openAt(0.3, 0.2)
	g, e, s, foe, tg := posGame(t, d, a, b)
	reps := useAt(t, e, s, k, tg)
	if s.Pos != b || foe.Pos != a || s.Orbit != -1 || foe.Orbit != -1 {
		t.Errorf("swapped: %v %v", s.Pos, foe.Pos)
	}
	// Its Use carries both moves, Empires then Ships order: ours first.
	want := []Move{{Ship: s, Empire: 0, From: a, To: b, Orbit: -1}, {Ship: foe, Empire: 1, From: b, To: a, Orbit: -1}}
	if u := reps[0].Event.Use; !slices.Equal(u.Moves, want) || u.Destroyed != nil {
		t.Errorf("use's moves %+v, destroyed %v; want %+v", u.Moves, u.Destroyed, want)
	}
	s.Refill()
	mate := extra(g, 0, openAt(0, 0.5))
	useAt(t, e, s, k, Target{Kind: ShipTarget, Mine: mate})
	if s.Pos != openAt(0, 0.5) || mate.Pos != b {
		t.Errorf("own swap: %v %v", s.Pos, mate.Pos)
	}
	s.Refill()
	s.Orbit = 0
	useRefused(t, e, s, k, tg, "in orbit: leave it first")
	s.Orbit = -1
	foe.Orbit = 0
	useRefused(t, e, s, k, tg, "fire at the planet")
}

// The Fleet Disperser pushes every other ship in space within Special1
// of the target (the user included) out to Special1,
// straight away from it: two on one ray in order, the second stopping
// ShipGap short of where the first went; ships in orbit or in a lane,
// and one beyond, untouched.
func TestFleetDisperser(t *testing.T) {
	d := aimer(FleetDisperser)
	k := part(d, FleetDisperser)
	R := float64(Gizmos[FleetDisperser].Special1) * RangeUnit
	T := openAt(0, 0)
	g, e, s, foe, tg := posGame(t, d, openAt(0, 0.2), T)
	v1 := extra(g, 1, openAt(0.1, 0))
	v2 := extra(g, 1, openAt(0.2, 0))
	orb := extra(g, 1, openAt(-0.1, 0))
	orb.Orbit = 0
	lane := extra(g, 1, openAt(0, -0.1))
	lane.Lane = 0
	beyond := extra(g, 1, openAt(-R-0.1, 0))
	useAt(t, e, s, k, tg)
	if !sameAt(s.Pos, openAt(0, R)) || !sameAt(v1.Pos, openAt(R, 0)) || !sameAt(v2.Pos, openAt(R-ShipGap, 0)) {
		t.Errorf("pushed: user %v v1 %v v2 %v", s.Pos, v1.Pos, v2.Pos)
	}
	if foe.Pos != T || orb.Pos != openAt(-0.1, 0) || lane.Pos != openAt(0, -0.1) || beyond.Pos != openAt(-R-0.1, 0) {
		t.Errorf("moved: target %v orbit %v lane %v beyond %v", foe.Pos, orb.Pos, lane.Pos, beyond.Pos)
	}
	s.Refill()
	inSpaceAt(s, openAt(0, R+0.1))
	g.Destroy(v1)
	g.Destroy(v2)
	useRefused(t, e, s, k, tg, "no ship in range")
}

// The Gravity Distorter pushes every other ship in space within
// Special1 of the user out to Special1, any empire's; none: refused.
func TestGravityDistorter(t *testing.T) {
	d := aimer(GravityDistorter)
	k := part(d, GravityDistorter)
	R := float64(Gizmos[GravityDistorter].Special1) * RangeUnit
	U := openAt(0, 0)
	g, e, s, foe, _ := posGame(t, d, U, openAt(0.3, 0))
	mate := extra(g, 0, openAt(0, -0.1))
	reps := useAt(t, e, s, k, Target{})
	if !sameAt(foe.Pos, openAt(R, 0)) || !sameAt(mate.Pos, openAt(0, -R)) || s.Pos != U {
		t.Errorf("pushed: foe %v mate %v user %v", foe.Pos, mate.Pos, s.Pos)
	}
	if !strings.Contains(reps[0].Event.Text, "moves 2 ships") {
		t.Errorf("report %q", reps[0].Event.Text)
	}
	s.Refill()
	inSpaceAt(foe, openAt(R+0.1, 0))
	inSpaceAt(mate, openAt(0, -R-0.1))
	useRefused(t, e, s, k, Target{}, "no ship in range")
}

// The Mass Condensor pulls every other ship in space within Special1 of
// the target (the user included) to Special2 from it, in order against
// live positions (the second on a ray stops ShipGap off the first); one
// closer than Special2 stays; none to pull: refused.
func TestMassCondensor(t *testing.T) {
	in := &Gizmos[MassCondensor]
	d := aimer(MassCondensor)
	k := part(d, MassCondensor)
	R1, R2 := float64(in.Special1)*RangeUnit, float64(in.Special2)*RangeUnit
	T := openAt(0, 0)
	g, e, s, foe, tg := posGame(t, d, openAt(0, 0.4), T)
	v1 := extra(g, 1, openAt(0.2, 0))
	v2 := extra(g, 1, openAt(0.3, 0))
	inner := extra(g, 0, openAt(0, -R2/2))
	out := extra(g, 1, openAt(-R1-0.05, 0))
	useAt(t, e, s, k, tg)
	if !sameAt(s.Pos, openAt(0, R2)) || !sameAt(v1.Pos, openAt(R2, 0)) || !sameAt(v2.Pos, openAt(R2+ShipGap, 0)) {
		t.Errorf("pulled: user %v v1 %v v2 %v", s.Pos, v1.Pos, v2.Pos)
	}
	if foe.Pos != T || inner.Pos != openAt(0, -R2/2) || out.Pos != openAt(-R1-0.05, 0) {
		t.Errorf("moved: target %v inner %v out %v", foe.Pos, inner.Pos, out.Pos)
	}
	s.Refill()
	g.Destroy(v1)
	g.Destroy(v2)
	inSpaceAt(s, openAt(0, R1+0.05))
	useRefused(t, e, s, k, tg, "no ship in range")
}

// The Gravimetric Condensor pulls every ship in space within Special1
// of the star (the user included) to Special2 from it; one beyond stays,
// one within Special2 too; nobody to pull: refused.
func TestGravimetricCondensor(t *testing.T) {
	in := &Gizmos[GravimetricCondensor]
	d := aimer(GravimetricCondensor)
	k := part(d, GravimetricCondensor)
	R1, R2 := float64(in.Special1)*RangeUnit, float64(in.Special2)*RangeUnit
	g, e, s, foe, _ := posGame(t, d, Vec3{Y: -0.6}, Vec3{X: 0.4, Y: -0.4})
	beyond := extra(g, 1, Vec3{X: -R1, Y: -0.3})
	inner := extra(g, 1, Vec3{X: -R2 / 2})
	useAt(t, e, s, k, Target{})
	if !sameAt(s.Pos, Vec3{Y: -R2}) || !sameAt(foe.Pos, Vec3{X: R2 / 1.4142135623730951, Y: -R2 / 1.4142135623730951}) {
		t.Errorf("pulled: user %v foe %v", s.Pos, foe.Pos)
	}
	if beyond.Pos != (Vec3{X: -R1, Y: -0.3}) || inner.Pos != (Vec3{X: -R2 / 2}) {
		t.Errorf("moved: beyond %v inner %v", beyond.Pos, inner.Pos)
	}
	s.Refill()
	inSpaceAt(s, Vec3{Y: -R1 - 0.1})
	inSpaceAt(foe, Vec3{X: R1 + 0.1})
	g.Destroy(beyond)
	useRefused(t, e, s, k, Target{}, "no ship in range")
}

// The Gravimetric Catapult mirrors the user through the star (Pos →
// −Pos), FreeSpot off a ship standing there, out of orbit; refused at
// the star.
func TestGravimetricCatapult(t *testing.T) {
	d := aimer(GravimetricCatapult)
	k := part(d, GravimetricCatapult)
	p := Vec3{X: 0.3, Y: -0.6, Z: 0.1}
	g, e, s, _, _ := posGame(t, d, p, openAt(-0.5, 0.5))
	m := Vec3{X: -p.X, Y: -p.Y, Z: -p.Z}
	clearOfPlanets(t, g, s.Star, m)
	o := Order{Kind: GoToPoint, Pos: openAt(0, 0)}
	s.Order = o
	useAt(t, e, s, k, Target{})
	if s.Pos != m || s.Order != o {
		t.Errorf("thrown to %v order %v, want %v", s.Pos, s.Order, m)
	}
	s.Refill()
	inSpaceAt(s, p)
	x := extra(g, 1, m)
	useAt(t, e, s, k, Target{})
	if want := FreeSpot(m, []Vec3{x.Pos, openAt(-0.5, 0.5)}); !sameAt(s.Pos, want) || dist(s.Pos, x.Pos) < ShipGap-1e-9 {
		t.Errorf("crowded: at %v, want %v", s.Pos, want)
	}
	g.Destroy(x)

	home := e.Colonies[0]
	s.Refill()
	s.Orbit, s.Pos = home.PlanetIdx, home.Planet.Pos
	useRefused(t, e, s, k, Target{}, "in orbit: leave it first") // in orbit a ship only leaves
	s.Orbit = -1
	s.Refill()
	inSpaceAt(s, Vec3{X: ShipGap / 2})
	useRefused(t, e, s, k, Target{}, "at the star")
}

// laneGame is a two-empire game at Peace (no rounds), the user's s of d
// (empire 0) in space at the endpoint of its home's lane 0; li the
// cluster lane, h the home, n the far star.
func laneGame(t *testing.T, d Design) (g *Game, e *Empire, s *Starship, li, h, n int) {
	t.Helper()
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e = g.Empires[0]
	home := e.Colonies[0]
	h = home.Star
	li = g.Cluster.Stars[h].Lanes[0]
	n = g.Cluster.Other(li, h)
	s = e.Spawn(d, h, home.PlanetIdx)
	s.Shields = false
	inSpaceAt(s, g.Cluster.Stars[h].System.Lanes[0].Pos)
	return
}

// The lane target: the index must be one of the star's lane endpoints,
// within Range × RangeUnit of the user; the kind must be a lane.
func TestLaneTarget(t *testing.T) {
	d := aimer(LaneEndoscope)
	k := part(d, LaneEndoscope)
	g, e, s, _, h, _ := laneGame(t, d)
	ls := g.Cluster.Stars[h].System.Lanes
	useRefused(t, e, s, k, Target{Kind: LaneTarget, Index: len(ls)}, "no lane")
	useRefused(t, e, s, k, Target{Kind: LaneTarget, Index: -1}, "no lane")
	useRefused(t, e, s, k, Target{Kind: ShipTarget}, "takes no ship target")
	r := float64(Gizmos[LaneEndoscope].Range) * RangeUnit
	end := ls[0].Pos
	u := unit(end, Vec3{})
	inSpaceAt(s, along(end, u, r+0.01))
	useRefused(t, e, s, k, Target{Kind: LaneTarget}, "Lane Endoscope: out of range")
	inSpaceAt(s, along(end, u, r-0.01))
	useAt(t, e, s, k, Target{Kind: LaneTarget})
}

// The Lane Blocker closes the lane both ways for BlockDays to every
// empire but the user's: an order in is refused, a foreign ship ordered
// before waits at the entry (holdAtBlock) from either end, the user's own
// ship goes through; at the end of the block the waiters go through. A
// second blocker on the blocked lane is refused; the part is lost.
func TestLaneBlocker(t *testing.T) {
	d := aimer(LaneBlocker, LaneBlocker)
	k := part(d, LaneBlocker)
	g, e, s, li, h, n := laneGame(t, d)
	c := g.Cluster
	e1 := g.Empires[1]
	spawn := func(o *Empire, star int, pos Vec3) *Starship {
		x := o.Spawn(laneShip, h, 0)
		x.Star, x.Orbit, x.Pos = star, -1, pos
		return x
	}
	_, endN, _ := laneEnd(c, n, h)
	waitH := spawn(e1, h, Vec3{Z: 0.6})
	waitN := spawn(e1, n, Vec3{Z: 0.6})
	if err := e1.Order(waitH, Order{Kind: GoToStar, Star: n}); err != nil {
		t.Fatal(err)
	}
	if err := e1.Order(waitN, Order{Kind: GoToStar, Star: h}); err != nil {
		t.Fatal(err)
	}
	useAt(t, e, s, k, Target{Kind: LaneTarget})
	until := e.LaneBlocks[li]
	if until != g.Day+BlockDays || len(s.Design.Parts) != len(d.Parts)-1 {
		t.Fatalf("until %d (day %d), parts %d", until, g.Day, len(s.Design.Parts))
	}
	if e1.LaneBlocker(h, li, g.Day) != e || e1.LaneBlocker(n, li, until-1) != e || e1.LaneBlocker(h, li, until) != nil || e.LaneBlocker(h, li, g.Day) != nil {
		t.Errorf("LaneBlocker: %v %v %v %v", e1.LaneBlocker(h, li, g.Day), e1.LaneBlocker(n, li, until-1), e1.LaneBlocker(h, li, until), e.LaneBlocker(h, li, g.Day))
	}
	s.Refill()
	useRefused(t, e, s, part(s.Design, LaneBlocker), Target{Kind: LaneTarget}, "already blocked")
	other := spawn(e1, h, Vec3{Z: -0.6})
	if err := e1.Order(other, Order{Kind: GoToStar, Star: n}); err == nil || err.Error() != "the lane is blocked by the Snovemdomas" {
		t.Errorf("order into the blocked lane: %v", err)
	}
	mine := spawn(e, h, Vec3{Z: -0.6})
	if err := e.Order(mine, Order{Kind: GoToStar, Star: n}); err != nil {
		t.Errorf("own ship: %v", err)
	}
	for g.Day+1 < until {
		g.EndDay()
		if waitH.Lane >= 0 || waitH.Star != h || waitN.Lane >= 0 || waitN.Star != n {
			t.Fatalf("day %d: a waiter entered: %d/%d %d/%d", g.Day, waitH.Star, waitH.Lane, waitN.Star, waitN.Lane)
		}
	}
	if waitH.Order != (Order{Kind: GoToStar, Star: n}) || waitN.Order != (Order{Kind: GoToStar, Star: h}) {
		t.Errorf("orders lost: %v %v", waitH.Order, waitN.Order)
	}
	if dist(waitN.Pos, c.Stars[n].System.Lanes[endN].Pos) > ShipGap+1e-9 {
		t.Errorf("not waiting at the entry: %v", waitN.Pos)
	}
	if mine.Star != n {
		t.Errorf("own ship not through: star %d lane %d", mine.Star, mine.Lane)
	}
	for range 60 {
		g.EndDay()
		if waitH.Star == n && waitN.Star == h {
			break
		}
	}
	if waitH.Star != n || waitN.Star != h {
		t.Errorf("after the block: %d %d", waitH.Star, waitN.Star)
	}
	// A later use drops the expired entry.
	e.LaneBlocks[li+1000] = g.Day
	s.Refill()
	inSpaceAt(s, c.Stars[h].System.Lanes[0].Pos)
	useAt(t, e, s, part(s.Design, LaneBlocker), Target{Kind: LaneTarget})
	if _, ok := e.LaneBlocks[li+1000]; ok || e.LaneBlocks[li] != g.Day+BlockDays {
		t.Errorf("blocks %v", e.LaneBlocks)
	}
}

// The Lane Destabilizer brings every ship inside the lane out at the next
// close (Arrive Day+1 at most, never later than it was), either way, any
// empire; a ship in another lane is untouched; refused on an empty lane.
func TestLaneDestabilizer(t *testing.T) {
	d := aimer(LaneDestabilizer)
	k := part(d, LaneDestabilizer)
	g, e, s, li, h, n := laneGame(t, d)
	useRefused(t, e, s, k, Target{Kind: LaneTarget}, "no ship in the lane")
	e1 := g.Empires[1]
	put := func(o *Empire, star, lane, arrive int) *Starship {
		x := o.Spawn(laneShip, h, 0)
		x.Star, x.Orbit, x.Lane, x.Arrive = star, -1, lane, arrive
		x.Order = Order{Kind: GoToStar, Star: g.Cluster.Other(lane, star)}
		return x
	}
	slow := put(e1, h, li, g.Day+9)
	back := put(e, n, li, g.Day+9)
	soon := put(e1, h, li, g.Day+1)
	lj := g.Cluster.Stars[h].Lanes[1]
	elsewhere := put(e1, h, lj, g.Day+9)
	reps := useAt(t, e, s, k, Target{Kind: LaneTarget})
	if len(reps) != 1 || !strings.Contains(reps[0].Event.Text, "3 ships hurried") {
		t.Errorf("reports %+v", reps)
	}
	if slow.Arrive != g.Day+1 || back.Arrive != g.Day+1 || soon.Arrive != g.Day+1 || elsewhere.Arrive != g.Day+9 {
		t.Errorf("arrive %d %d %d %d (day %d)", slow.Arrive, back.Arrive, soon.Arrive, elsewhere.Arrive, g.Day)
	}
	g.EndDay()
	if slow.Star != n || slow.Lane >= 0 || back.Star != h || back.Lane >= 0 || elsewhere.Lane != lj {
		t.Errorf("next close: slow %d/%d back %d/%d elsewhere lane %d", slow.Star, slow.Lane, back.Star, back.Lane, elsewhere.Lane)
	}
}

// The Lane Magnetron puts the user into the lane now, the GoToStar order
// taken, out at the next close (Arrive Day+1, faster than LaneDays);
// refused without a lane drive and on a lane blocked to it.
func TestLaneMagnetron(t *testing.T) {
	d := aimer(StarLaneDrive, LaneMagnetron)
	k := part(d, LaneMagnetron)
	g, e, s, li, h, n := laneGame(t, d)
	if LaneDays(g.Cluster, li, d.LaneStrength(), s.Species.Traits()) < 2 {
		t.Fatal("fixture: a one-day lane")
	}
	s.TiedDown = true // the Tie Down damps the engines: no lane entry
	if _, err := e.UseSpecial(s, k, Target{Kind: LaneTarget}); err == nil || !strings.Contains(err.Error(), "tied down") {
		t.Errorf("tied down: %v", err)
	}
	s.TiedDown = false
	g.Empires[1].LaneBlocks = map[int]int{li: g.Day + 1}
	useRefused(t, e, s, k, Target{Kind: LaneTarget}, "the lane is blocked by the Orfa")
	g.Empires[1].LaneBlocks = nil
	s.Orbit = 0 // in orbit a ship only leaves: no lane slip from there
	useRefused(t, e, s, k, Target{Kind: LaneTarget}, "in orbit: leave it first")
	s.Orbit = -1
	useAt(t, e, s, k, Target{Kind: LaneTarget})
	end := g.Cluster.Stars[h].System.Lanes[0].Pos
	if s.Lane != li || s.Arrive != g.Day+1 || s.Order != (Order{Kind: GoToStar, Star: n}) || s.Orbit != -1 || s.Pos != end || len(s.Design.Parts) != len(d.Parts)-1 {
		t.Fatalf("lane %d arrive %d (day %d) order %v orbit %d pos %v", s.Lane, s.Arrive, g.Day, s.Order, s.Orbit, s.Pos)
	}
	g.EndDay()
	if s.Star != n || s.Lane >= 0 {
		t.Errorf("next close: star %d lane %d", s.Star, s.Lane)
	}

	nd := aimer(LaneMagnetron)
	_, e2, s2, _, _, _ := laneGame(t, nd)
	useRefused(t, e2, s2, part(nd, LaneMagnetron), Target{Kind: LaneTarget}, "no star lane drive")
}

// The Lane Endoscope explores the far star (planets known) and learns
// the lane; refused when that star is known already.
func TestLaneEndoscope(t *testing.T) {
	d := aimer(LaneEndoscope)
	k := part(d, LaneEndoscope)
	_, e, s, li, _, n := laneGame(t, d)
	if e.Knows(n) {
		t.Fatal("fixture: far star known")
	}
	useAt(t, e, s, k, Target{Kind: LaneTarget})
	if !e.Knows(n) || li >= len(e.LanesKnown) || !e.LanesKnown[li] {
		t.Errorf("knows %v lanes %v", e.Knows(n), e.LanesKnown)
	}
	useRefused(t, e, s, k, Target{Kind: LaneTarget}, "already explored")
}

// A ship being refitted is held at its Docks like one under construction:
// no special at all, the frame refuses (else the Catapult would whip the
// ship out and the refit complete in space; a one-time device spent
// meanwhile would come back free).
func TestCatapultRefitHeld(t *testing.T) {
	d := aimer(GravimetricCatapult)
	k := part(d, GravimetricCatapult)
	_, e, s, _, _ := posGame(t, d, openAt(0.3, 0.1), openAt(-0.5, 0.5))
	home := e.Colonies[0]
	s.Orbit, s.Pos = home.PlanetIdx, home.Planet.Pos
	home.Project = &Project{Item: OrbitalDocks, Slot: -1, Orbital: true, Refit: s.Name}
	useRefused(t, e, s, k, Target{}, "being refitted")
	if s.Orbit != home.PlanetIdx {
		t.Errorf("left the Docks' orbit: %d", s.Orbit)
	}
}
