package game

import (
	"math"
	"testing"
)

// The exclusion radius (ShipGap): stops, round moves, lane exits, the
// arena, FreeSpot.

// inSpaceAt takes s out of orbit at p with its day's power.
func inSpaceAt(s *Starship, p Vec3) {
	s.Orbit, s.Pos = -1, p
	s.Refill()
}

// Two ships ordered to one point: the first gets there, the second, the
// point being taken, arrives at its FreeSpot (ShipGap or more from the
// first) and its order clears, then pays nothing on the following days.
// A ship in orbit is no obstacle.
func TestSameStand(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	a, b := e.Spawn(laneShip, c.Home, 0), e.Spawn(laneShip, c.Home, 0)
	p := Vec3{X: 0.5, Y: 0.2, Z: -0.6}
	for _, s := range []*Starship{a, b} {
		if err := e.Order(s, Order{Kind: GoToPoint, Pos: p}); err != nil {
			t.Fatal(err)
		}
	}
	for range 100 {
		e.EndDay()
	}
	if a.Pos != p || a.Order.Kind != NoOrder {
		t.Fatalf("first: pos %v order %+v", a.Pos, a.Order)
	}
	if d := dist(b.Pos, p); d < ShipGap-1e-9 || d > ShipGap+1e-9+ShipGap || b.Order.Kind != NoOrder || !b.InSpace() {
		t.Fatalf("second: %v from the point, order %+v, want a FreeSpot (ShipGap..2·ShipGap) and cleared", d, b.Order)
	}
	pos := b.Pos
	e.EndDay()
	if b.Pos != pos || b.Power != b.Design.PowerPerDay() {
		t.Errorf("idle: pos %v -> %v, power %d of %d", pos, b.Pos, b.Power, b.Design.PowerPerDay())
	}

	// A ship in orbit does not block a point on its planet.
	o := e.Spawn(laneShip, c.Home, 0)
	m := e.Spawn(laneShip, c.Home, 0)
	inSpaceAt(m, Vec3{X: o.Pos.X + 0.1, Y: o.Pos.Y, Z: o.Pos.Z})
	m.Order = Order{Kind: GoToPoint, Pos: o.Pos}
	if moved, arrived := m.Hop(c, e.Day, spaceAt([]*Empire{e}, c.Home, m), nil); !moved || !arrived || m.Pos != o.Pos {
		t.Errorf("onto an orbit: moved %v arrived %v pos %v", moved, arrived, m.Pos)
	}
}

// Round resolves its moves in the round's ship order, each against the
// earlier movers' ENDS: two ships leaving one orbit for one far point in
// one round end ShipGap apart, the first in Ships a full Hop out, the
// second ShipGap behind it on the same path.
func TestRoundSameEnd(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	x, y := e.Spawn(laneShip, home.Star, home.PlanetIdx), e.Spawn(laneShip, home.Star, home.PlanetIdx)
	start := x.Pos
	p := Vec3{X: -start.X, Y: 0.3, Z: -start.Z} // far: several Hops
	for _, s := range []*Starship{x, y} {
		s.Refill()
		s.Order = Order{Kind: GoToPoint, Pos: p}
	}
	g.Round(home.Star, nil)
	want, _ := hopEnd(start, p, laneShip.Reach())
	if dist(x.Pos, want) > 1e-12 || !x.InSpace() {
		t.Errorf("first: %v, want %v in space", x.Pos, want)
	}
	if d := dist(x.Pos, y.Pos); math.Abs(d-ShipGap) > 1e-9 || !y.InSpace() {
		t.Errorf("second %v from the first, want ShipGap %v (in space %v)", d, ShipGap, y.InSpace())
	}
	if d := dist(start, y.Pos); math.Abs(d+ShipGap-dist(start, x.Pos)) > 1e-9 {
		t.Errorf("second off the path: %v from the orbit", d)
	}
}

// Two ships 0.0499 apart (FreeSpot's float) ordered to one point beside
// them never end closer than ShipGap, round after round (round-start
// obstacles let them converge: each was checked against the other's old
// place). A taken point sends the second to its FreeSpot, which may be
// nearer than 0.0499 but never within ShipGap.
func TestRoundNoConverge(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	base := Vec3{X: 0.2, Y: 0.1, Z: -0.3}
	x, y := e.Spawn(laneShip, home.Star, 0), e.Spawn(laneShip, home.Star, 0)
	inSpaceAt(x, base)
	inSpaceAt(y, Vec3{X: base.X + 0.0499, Y: base.Y, Z: base.Z})
	p := Vec3{X: base.X + 0.025, Y: base.Y + 0.8, Z: base.Z}
	x.Order = Order{Kind: GoToPoint, Pos: p}
	y.Order = Order{Kind: GoToPoint, Pos: p}
	last := dist(x.Pos, y.Pos)
	for r := range 20 {
		x.Refill()
		y.Refill()
		g.Round(home.Star, nil)
		d := dist(x.Pos, y.Pos)
		if d < ShipGap-1e-9 && d < last-1e-12 {
			t.Fatalf("round %d: %v apart, %v before", r, d, last)
		}
		last = d
	}
	if dist(x.Pos, p) > 1e-12 && dist(y.Pos, p) > 1e-12 {
		t.Errorf("nobody on the point: %v %v", x.Pos, y.Pos)
	}
}

// The invariant, scripted: two empires' ships (two in orbit, two in
// space) sent round after round to common points: every pair ends every
// round ShipGap apart, or no closer than it started the round.
func TestRoundGapInvariant(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	home := g.Empires[0].Colonies[0]
	var ships []*Starship
	for k, e := range g.Empires {
		for i := range 2 {
			s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
			if i == 1 {
				inSpaceAt(s, Vec3{X: -0.4 + 0.03*float64(k), Y: 0.2, Z: 0.5})
			}
			ships = append(ships, s)
		}
	}
	points := []Vec3{{X: 0.3, Y: -0.1, Z: 0.2}, {X: -0.5, Y: 0.4, Z: -0.1}, {X: 0.1, Z: 0.9}, {X: 0.3, Y: -0.1, Z: 0.2}}
	moves := 0
	for r := range 60 {
		was := make([]Vec3, len(ships))
		for i, s := range ships {
			if r%15 == 0 {
				s.Order = Order{Kind: GoToPoint, Pos: points[r/15]}
			}
			s.Refill()
			was[i] = s.Pos
		}
		_, _, log := g.Round(home.Star, nil)
		moves += len(log.Moves)
		for i, a := range ships {
			for j := i + 1; j < len(ships); j++ {
				b := ships[j]
				if !a.InSpace() || !b.InSpace() {
					continue
				}
				if d, d0 := dist(a.Pos, b.Pos), dist(was[i], was[j]); d < min(ShipGap, d0)-1e-9 {
					t.Errorf("round %d: %s/%d and %s/%d %v apart, started %v", r,
						a.Name, i, b.Name, j, d, d0)
				}
			}
		}
	}
	if moves < 40 {
		t.Errorf("only %d moves: the script does not crowd", moves)
	}
}

// A ship starting within ShipGap of others may leave and may pass them,
// but not end closer than it started; exactly stacked, nothing holds it.
func TestHopStacked(t *testing.T) {
	c := twoStars()
	q := Vec3{X: 0.2}
	near := Vec3{X: q.X + 0.03}
	obs := []Vec3{q, near}
	hop := func(to Vec3) (*Starship, bool, bool) {
		s := &Starship{Design: laneShip, Lane: -1}
		inSpaceAt(s, q)
		s.Order = Order{Kind: GoToPoint, Pos: to}
		moved, arrived := s.Hop(c, 1, obs, nil)
		return s, moved, arrived
	}
	reach := laneShip.Reach()
	if s, moved, _ := hop(Vec3{X: q.X - 0.5}); !moved || dist(s.Pos, Vec3{X: q.X - reach}) > 1e-12 {
		t.Errorf("away: moved %v pos %v", moved, s.Pos)
	}
	if s, moved, _ := hop(Vec3{X: q.X + 0.5}); !moved || dist(s.Pos, Vec3{X: q.X + reach}) > 1e-12 {
		t.Errorf("past the near one: moved %v pos %v", moved, s.Pos)
	}
	short := Vec3{X: q.X - 0.02} // within ShipGap of both, but farther than it started
	if s, moved, arrived := hop(short); !moved || !arrived || s.Pos != short {
		t.Errorf("a short step away: moved %v arrived %v pos %v", moved, arrived, s.Pos)
	}
	// Would end 0.01 from near, started 0.03: the point is taken, so the
	// ship goes to its FreeSpot and the order clears.
	s, moved, arrived := hop(Vec3{X: q.X + 0.04})
	if !moved || !arrived || s.Order.Kind != NoOrder || crowds(s.Pos, obs) {
		t.Errorf("closer: moved %v arrived %v pos %v order %+v", moved, arrived, s.Pos, s.Order)
	}
}

func TestFreeSpot(t *testing.T) {
	p := Vec3{X: 0.4, Y: 0.1, Z: -0.2}
	if got := FreeSpot(p, nil); got != p {
		t.Errorf("empty: %v", got)
	}
	if got := FreeSpot(p, []Vec3{{X: p.X + ShipGap, Y: p.Y, Z: p.Z}}); got != p {
		t.Errorf("a ship ShipGap away: %v, want p", got)
	}
	k := ShipGap / math.Sqrt(3)
	first := Vec3{X: p.X - k, Y: p.Y - k, Z: p.Z - k} // (−1, −1, −1) first
	if got := FreeSpot(p, []Vec3{p}); dist(got, first) > 1e-12 {
		t.Errorf("crowded: %v, want %v", got, first)
	}
	// p and the first direction taken: the next free one, ShipGap from both.
	got := FreeSpot(p, []Vec3{p, first})
	if dist(got, p) < ShipGap-1e-9 || dist(got, first) < ShipGap-1e-9 {
		t.Errorf("second: %v crowds p or %v", got, first)
	}
	// On the reach's sphere: the outward directions are skipped.
	edge := Vec3{X: -SystemReach}
	in := Vec3{X: edge.X + k, Y: -k, Z: -k} // (1, −1, −1), the first inward
	if got := FreeSpot(edge, []Vec3{edge}); dist(got, in) > 1e-12 || dist(got, Vec3{}) > SystemReach {
		t.Errorf("at the edge: %v, want %v", got, in)
	}
}

// Two empires' ships out of one lane on the same day: the first on the
// endpoint, the second ShipGap from it.
func TestLaneExitGap(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	c := g.Cluster
	home := g.Empires[0].Colonies[0].Star
	lane := c.Stars[home].Lanes[0]
	far := c.Other(lane, home)
	var ships []*Starship
	for _, e := range g.Empires {
		s := e.Spawn(laneShip, home, 0)
		s.Orbit, s.Lane, s.Arrive = -1, lane, g.Day+1
		ships = append(ships, s)
	}
	g.EndDay()
	_, end, _ := laneEnd(c, far, home)
	at := c.Stars[far].System.Lanes[end].Pos
	a, b := ships[0], ships[1]
	if a.Star != far || b.Star != far || a.Lane != -1 || b.Lane != -1 {
		t.Fatalf("not out: %+v %+v", a, b)
	}
	if a.Pos != at || math.Abs(dist(a.Pos, b.Pos)-ShipGap) > 1e-12 {
		t.Errorf("exits %v %v (endpoint %v), %v apart", a.Pos, b.Pos, at, dist(a.Pos, b.Pos))
	}
}

// Every move ending in space keeps ShipGap, a GoToPlanet's on the way
// too; entering an orbit or a lane does not (the ship leaves space).
func TestHopGapEnds(t *testing.T) {
	c := twoStars() // planet 0 at (0.5, 0, 0), lane endpoint (−1, 0, 0)
	reach := laneShip.Reach()
	fly := func(o Order, from Vec3, obs []Vec3) (*Starship, bool, bool) {
		s := &Starship{Design: laneShip, Lane: -1}
		inSpaceAt(s, from)
		s.Order = o
		moved, arrived := s.Hop(c, 1, obs, nil)
		return s, moved, arrived
	}
	pl := c.Stars[0].System.Planets[0].Pos
	if s, moved, arrived := fly(Order{Kind: GoToPlanet}, Vec3{}, []Vec3{{X: reach}}); !moved || arrived ||
		dist(s.Pos, Vec3{X: reach - ShipGap}) > 1e-12 || s.Orbit != -1 {
		t.Errorf("to a planet, on the way: moved %v arrived %v pos %v", moved, arrived, s.Pos)
	}
	if s, moved, arrived := fly(Order{Kind: GoToPlanet}, Vec3{X: pl.X - reach/2}, []Vec3{pl}); !moved || !arrived || s.Orbit != 0 {
		t.Errorf("into the orbit: moved %v arrived %v orbit %d", moved, arrived, s.Orbit)
	}
	end := c.Stars[0].System.Lanes[0].Pos
	if s, moved, arrived := fly(Order{Kind: GoToStar, Star: 1}, Vec3{X: end.X + reach/2}, []Vec3{end}); !moved || !arrived || s.Lane != 0 {
		t.Errorf("into the lane: moved %v arrived %v lane %d", moved, arrived, s.Lane)
	}
}
