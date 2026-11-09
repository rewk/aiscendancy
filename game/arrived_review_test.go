package game

import "testing"

// A one-day lane entered on the closing day's leftover power (Step's
// first proceed, day−1) is left in the same close: still an arrival out
// of a lane (though the ship was not in the lane before the Step).
func TestArrivedOneDayLane(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	c := e.Cluster
	home := e.Colonies[0]
	lane := c.Stars[home.Star].Lanes[0]
	far := c.Other(lane, home.Star)
	c.Stars[far].Pos = c.Stars[home.Star].Pos // the lane now takes one day
	if n := LaneDays(c, lane, laneShip.LaneStrength(), Snovemdomas.Traits()); n != 1 {
		t.Fatalf("lane days %d", n)
	}
	s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	_, end, _ := laneEnd(c, home.Star, far)
	s.Orbit, s.Pos = -1, c.Stars[home.Star].System.Lanes[end].Pos // a hop from the lane
	s.Refill()
	if err := e.Order(s, Order{Kind: GoToStar, Star: far}); err != nil {
		t.Fatal(err)
	}
	n := 0
	for d := 0; s.Star != far || s.Lane >= 0; d++ {
		if d > 20 {
			t.Fatalf("not at %d: %+v", far, s)
		}
		for _, x := range e.EndDay() {
			if x.Kind == Arrived && x.Star == far && x.Planet == -1 {
				n++
			}
		}
	}
	if n != 1 {
		t.Errorf("%d arrivals at the far star, want 1", n)
	}
}

// An order to the orbit a ship is in completes at the close without a
// move: no arrival (no "enters orbit" report).
func TestArrivedNotForOwnOrbit(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: home.PlanetIdx}); err != nil {
		t.Fatal(err)
	}
	for _, x := range e.EndDay() {
		if x.Kind == Arrived {
			t.Errorf("reported %q", x.Text)
		}
	}
}
