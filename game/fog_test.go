package game

import "testing"

// NewGame: each empire Knows its home star and nothing else, the lanes
// touching it and no other; StarLabel names the home, types the rest.
func TestFogAtStart(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Chronomyst}, Average)
	c := g.Cluster
	for i, e := range g.Empires {
		home := e.Colonies[0].Star
		for st := range c.Stars {
			if e.Knows(st) != (st == home) {
				t.Errorf("empire %d (home %d) knows star %d: %v", i, home, st, e.Knows(st))
			}
		}
		for li, l := range c.Lanes {
			if want := l.A == home || l.B == home; e.KnowsLane(li) != want {
				t.Errorf("empire %d knows lane %d (%d-%d): %v", i, li, l.A, l.B, !want)
			}
		}
		if got := e.StarLabel(home); got != c.Stars[home].Name {
			t.Errorf("empire %d home label %q", i, got)
		}
		other := notAdjacent(t, c, home)
		if got := e.StarLabel(other); got != c.Stars[other].Type.String() || got == c.Stars[other].Name {
			t.Errorf("empire %d label of unknown star %d: %q", i, other, got)
		}
	}
}

// A ship sent along a lane explores the far star on arrival, not
// before; Present is true at the home (colony), false at the far star
// while the ship is in the lane, true once it is out, false elsewhere.
func TestFogLaneArrival(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	home := e.Colonies[0].Star
	far := c.Other(c.Stars[home].Lanes[0], home)
	s := e.Spawn(laneShip, home, 1)
	if err := e.Order(s, Order{Kind: GoToStar, Star: far}); err != nil {
		t.Fatal(err)
	}
	inLane := false
	for day := 0; s.Lane >= 0 || s.Star != far; day++ {
		if day > 100 {
			t.Fatalf("not at star %d after 100 days: star %d lane %d", far, s.Star, s.Lane)
		}
		e.EndDay()
		if s.Lane >= 0 {
			inLane = true
			if e.Knows(far) || e.Present(far) {
				t.Errorf("day %d in the lane: knows %v, present %v at star %d", e.Day, e.Knows(far), e.Present(far), far)
			}
		}
	}
	if !inLane {
		t.Fatal("the ship never was in the lane")
	}
	if !e.Knows(far) || !e.Present(far) || !e.Present(home) {
		t.Errorf("arrived: knows %v, present %v, home present %v", e.Knows(far), e.Present(far), e.Present(home))
	}
	if other := notAdjacent(t, c, home); other != far && e.Present(other) {
		t.Errorf("present at star %d with nothing there", other)
	}
}

// Spawn explores its star; Present follows the ship there.
func TestFogSpawn(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	other := notAdjacent(t, e.Cluster, e.Colonies[0].Star)
	if e.Knows(other) || e.Present(other) {
		t.Fatalf("star %d known or present before the spawn", other)
	}
	e.Spawn(laneShip, other, 0)
	if !e.Knows(other) || !e.Present(other) {
		t.Errorf("after the spawn: knows %v, present %v", e.Knows(other), e.Present(other))
	}
}

// Oculons see every lane but know only their home star.
func TestFogOculons(t *testing.T) {
	e := NewEmpire(1, Oculons, Average)
	home := e.Colonies[0].Star
	for li := range e.Cluster.Lanes {
		if !e.KnowsLane(li) {
			t.Errorf("Oculons do not see lane %d", li)
		}
	}
	for st := range e.Cluster.Stars {
		if e.Knows(st) != (st == home) {
			t.Errorf("Oculons know star %d: %v", st, e.Knows(st))
		}
	}
}

// Visit grows a short Visited to the cluster's size and ignores −1;
// Knows is false off the slice.
func TestFogVisitBounds(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	n := len(e.Cluster.Stars)
	e.Visited = e.Visited[:2]
	last := n - 1
	if e.Knows(last) {
		t.Fatal("a star past the slice known")
	}
	e.Visit(last)
	if len(e.Visited) != n || !e.Knows(last) {
		t.Errorf("Visit(%d): len %d, knows %v", last, len(e.Visited), e.Knows(last))
	}
	before := append([]bool(nil), e.Visited...)
	e.Visit(-1)
	for i := range before {
		if e.Visited[i] != before[i] {
			t.Errorf("Visit(-1) changed star %d", i)
		}
	}
	if e.Knows(-1) || e.Knows(n) {
		t.Errorf("Knows(-1) %v, Knows(%d) %v", e.Knows(-1), n, e.Knows(n))
	}
}
