package game

import (
	"math"
	"testing"
)

// LaneStrength: Level × √count per drive kind, summed.
func TestLaneStrength(t *testing.T) {
	d := func(drives, hypers int) Design {
		var p []Gizmo
		for range drives {
			p = append(p, StarLaneDrive)
		}
		for range hypers {
			p = append(p, StarLaneHyperdrive)
		}
		return Design{Hull: Medium, Parts: p}
	}
	for _, tc := range []struct {
		drives, hypers int
		want           float64
	}{{0, 0, 0}, {1, 0, 1}, {2, 0, math.Sqrt2}, {3, 0, math.Sqrt(3)}, {0, 1, 2}, {0, 2, 2 * math.Sqrt2}, {1, 1, 3}} {
		if got := d(tc.drives, tc.hypers).LaneStrength(); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("%dD+%dH: strength %g, want %g", tc.drives, tc.hypers, got, tc.want)
		}
	}
}

// The original's ETA on the lane entry day (measured in DOSBox,
// save 09.SAV's lane lengths in file units; radius 1 = 440).
func TestLaneDaysMeasured(t *testing.T) {
	for _, m := range []struct {
		file           float64
		red            bool
		drives, hypers int
		days           int
	}{
		{149.5, false, 1, 0, 15}, {113.2, false, 1, 0, 12}, {74.8, false, 1, 0, 8},
		{141.2, true, 1, 0, 43}, {212.3, true, 1, 0, 64},
		{149.5, false, 0, 1, 8}, {113.2, false, 0, 1, 6}, {74.8, false, 0, 1, 4}, {77.8, false, 0, 1, 4},
		{141.2, true, 0, 1, 22}, {212.3, true, 0, 1, 32},
		{149.5, false, 2, 0, 11}, {212.3, true, 2, 0, 46}, {149.5, false, 3, 0, 9},
		{149.5, false, 1, 1, 5}, {212.3, true, 0, 2, 23},
	} {
		c := &Cluster{Stars: []Star{{}, {Pos: Vec3{X: m.file / 440}}}, Lanes: []StarLane{{A: 0, B: 1, Red: m.red}}}
		var p []Gizmo
		for range m.drives {
			p = append(p, StarLaneDrive)
		}
		for range m.hypers {
			p = append(p, StarLaneHyperdrive)
		}
		if got := LaneDays(c, 0, Design{Parts: p}.LaneStrength(), Base); got != m.days {
			t.Errorf("%.1f red %v %dD+%dH: %d days, measured %d", m.file, m.red, m.drives, m.hypers, got, m.days)
		}
	}
}

// The ETA read on the day a ship enters a lane is its LaneDays, no extra
// day (as measured): before the hop from the endpoint, after it, and the
// ship is out at the far star after exactly that many closes.
func TestLaneEntryETA(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	c := e.Cluster
	home := e.Colonies[0]
	lane := c.Stars[home.Star].Lanes[0]
	far := c.Other(lane, home.Star)
	s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	_, end, _ := laneEnd(c, home.Star, far)
	s.Orbit, s.Pos = -1, c.Stars[home.Star].System.Lanes[end].Pos
	if err := e.Order(s, Order{Kind: GoToStar, Star: far}); err != nil {
		t.Fatal(err)
	}
	n := LaneDays(c, lane, laneShip.LaneStrength(), s.Species.Traits())
	if n < 2 {
		t.Fatalf("fixture: %d-day lane", n)
	}
	if eta, ok := s.ETA(c, e.Day); !ok || eta != n {
		t.Fatalf("ETA before entry %d %v, want %d", eta, ok, n)
	}
	if _, hit := s.Hop(c, e.Day, nil, nil); !hit || s.Lane != lane {
		t.Fatalf("lane not entered: lane %d", s.Lane)
	}
	if eta, ok := s.ETA(c, e.Day); !ok || eta != n {
		t.Fatalf("ETA on entry %d %v, want %d", eta, ok, n)
	}
	for k := 1; k <= n; k++ {
		e.EndDay()
		if out := s.Lane < 0; out != (k == n) {
			t.Fatalf("after %d of %d closes: lane %d star %d", k, n, s.Lane, s.Star)
		}
	}
	if s.Star != far {
		t.Errorf("out at star %d, want %d", s.Star, far)
	}
}
