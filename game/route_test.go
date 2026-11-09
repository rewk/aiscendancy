package game

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// routeCluster is a hand cluster: stars at pos named S0.., lanes in
// order, each star's Lanes in cluster-lane order with a System endpoint
// per lane (radius 1.1, spread round the disc), no planets.
func routeCluster(pos []Vec3, lanes []StarLane) *Cluster {
	c := &Cluster{Lanes: lanes}
	for i, p := range pos {
		c.Stars = append(c.Stars, Star{Name: fmt.Sprintf("S%d", i), Pos: p, System: &StarSystem{Name: fmt.Sprintf("S%d", i)}})
	}
	for li, l := range lanes {
		for _, end := range [2][2]int{{l.A, l.B}, {l.B, l.A}} {
			st := &c.Stars[end[0]]
			a := float64(len(st.Lanes)) * 2 * math.Pi / 5
			st.Lanes = append(st.Lanes, li)
			st.System.Lanes = append(st.System.Lanes, Lane{Pos: Vec3{X: 1.1 * math.Cos(a), Z: 1.1 * math.Sin(a)}, To: end[1]})
		}
	}
	return c
}

// routeEmpire is a bare empire on c knowing every star, with one lane
// ship "R" at star 0 in space at its centre.
func routeEmpire(c *Cluster) (*Empire, *Starship) {
	e := &Empire{Cluster: c, Species: Snovemdomas, Visited: make([]bool, len(c.Stars))}
	for i := range e.Visited {
		e.Visited[i] = true
	}
	s := &Starship{Name: "R", Design: laneShip, Orbit: -1, Lane: -1, Species: e.Species, Shields: true, Weapon: -1}
	s.Hull = s.MaxHull()
	s.Refill()
	e.Ships = append(e.Ships, s)
	return e, s
}

// detour: S0-S3 one red lane (lane 0), or three blue ones by S1, S2
// (lanes 1-3), far fewer days.
func detour() *Cluster {
	return routeCluster([]Vec3{{}, {X: 0.2, Y: 0.1}, {X: 0.4, Y: 0.1}, {X: 0.6}},
		[]StarLane{{A: 0, B: 3, Red: true}, {A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}})
}

// laneDays is a route's days the long way: LaneDays per lane.
func laneDays(c *Cluster, s *Starship, lanes ...int) int {
	n := 0
	for _, li := range lanes {
		n += LaneDays(c, li, s.Design.LaneStrength(), s.Species.Traits())
	}
	return n
}

// The fewest days win over the fewest lanes; RouteLanes gives the lanes
// and their days.
func TestRouteDaysNotHops(t *testing.T) {
	c := detour()
	e, s := routeEmpire(c)
	if red, blue := laneDays(c, s, 0), laneDays(c, s, 1, 2, 3); red <= blue {
		t.Fatalf("fixture: red %d days, blue path %d", red, blue)
	}
	r, err := e.Route(s, 3)
	if err != nil || !slices.Equal(r, []int{1, 2, 3}) {
		t.Fatalf("route %v, %v; want [1 2 3]", r, err)
	}
	if lanes, days := e.RouteLanes(s, r); !slices.Equal(lanes, []int{1, 2, 3}) || days != laneDays(c, s, 1, 2, 3) {
		t.Errorf("lanes %v days %d", lanes, days)
	}
}

// The fog: a star unexplored is no stop, though its lanes are seen from
// the known end; it is a destination down a known lane; the refusals.
func TestRouteFog(t *testing.T) {
	c := detour()
	e, s := routeEmpire(c)
	e.Visited[2] = false
	if !e.KnowsLane(2) {
		t.Fatal("fixture: lane 1-2 unseen")
	}
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{3}) {
		t.Errorf("route %v, %v; want the red lane [3]", r, err)
	}
	if r, err := e.Route(s, 2); err != nil || !slices.Equal(r, []int{1, 2}) {
		t.Errorf("to the unexplored star 2: %v, %v; want [1 2]", r, err)
	}
	if _, err := e.Route(s, 0); err == nil {
		t.Error("a route to the star the ship stands at")
	}
	e.Visited[1] = false // through star 1 no more: the red lane, then 3-2
	if r, err := e.Route(s, 2); err != nil || !slices.Equal(r, []int{3, 2}) {
		t.Errorf("around star 1: %v, %v; want [3 2]", r, err)
	}
	e.Visited[3] = false // 2's lanes have no known end now
	if _, err := e.Route(s, 2); err == nil || err.Error() != "no known route to "+c.Stars[2].Name {
		t.Errorf("down unknown lanes: %v", err)
	}
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{3}) {
		t.Errorf("to the unexplored star 3 down the red lane: %v, %v", r, err)
	}
	e.Visited[2], e.Visited[3] = true, true // the red lane alone
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{3}) {
		t.Errorf("route %v, %v", r, err)
	}
	s.Design = Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}
	if _, err := e.Route(s, 3); err == nil {
		t.Error("a route without a lane drive")
	}
}

// A lane another empire's Lane Blocker closes today is skipped; open
// again, it is taken.
func TestRouteBlocked(t *testing.T) {
	c := detour()
	e, s := routeEmpire(c)
	o := &Empire{Index: 1, Cluster: c, LaneBlocks: map[int]int{2: 10}}
	e.peers, o.peers = []*Empire{e, o}, []*Empire{e, o}
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{3}) {
		t.Errorf("blocked: route %v, %v; want [3]", r, err)
	}
	e.Day = 10
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{1, 2, 3}) {
		t.Errorf("open: route %v, %v; want [1 2 3]", r, err)
	}
}

// Two routes of equal days: the lower star index wins, whatever the
// lanes' order or which side is drawn first.
func TestRouteTie(t *testing.T) {
	pos := []Vec3{{}, {X: 0.2, Y: 0.2}, {X: 0.2, Y: -0.2}, {X: 0.4}}
	for _, ls := range [][]StarLane{
		{{A: 0, B: 1}, {A: 0, B: 2}, {A: 1, B: 3}, {A: 2, B: 3}},
		{{A: 2, B: 3}, {A: 1, B: 3}, {A: 0, B: 2}, {A: 0, B: 1}},
		{{A: 3, B: 2}, {A: 2, B: 0}, {A: 3, B: 1}, {A: 1, B: 0}},
	} {
		c := routeCluster(pos, ls)
		e, s := routeEmpire(c)
		if laneDays(c, s, 0) != laneDays(c, s, 1) || laneDays(c, s, 2) != laneDays(c, s, 3) {
			t.Fatal("fixture: no tie")
		}
		if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{1, 3}) {
			t.Errorf("lanes %v: route %v, %v; want [1 3]", ls, r, err)
		}
		// From S3 to S0 the tie is S0's own predecessor: S1 again.
		s.Star = 3
		if r, err := e.Route(s, 0); err != nil || !slices.Equal(r, []int{1, 0}) {
			t.Errorf("lanes %v: back %v, %v; want [1 0]", ls, r, err)
		}
	}
	// S2 is settled first (5 days) and reaches S3 first, S1 (10 days)
	// as cheaply later: the lower star still wins.
	u := LaneSpeed
	c := routeCluster([]Vec3{{}, {Y: 9.5 * u}, {X: 4.5 * u}, {X: 0.10227, Y: 0.21591}},
		[]StarLane{{A: 0, B: 1}, {A: 0, B: 2}, {A: 1, B: 3}, {A: 2, B: 3}})
	e, s := routeEmpire(c)
	if a, b := laneDays(c, s, 0, 2), laneDays(c, s, 1, 3); a != b || laneDays(c, s, 1) >= laneDays(c, s, 0) {
		t.Fatalf("fixture: by S1 %d days, by S2 %d, S2 first %v", a, b, laneDays(c, s, 1) < laneDays(c, s, 0))
	}
	if r, err := e.Route(s, 3); err != nil || !slices.Equal(r, []int{1, 3}) {
		t.Errorf("settled later: route %v, %v; want [1 3]", r, err)
	}
}

// A ship in a lane is routed from the lane's far end, its first stop:
// that end is no "already there", the lane it is in counts whole.
func TestRouteFromLane(t *testing.T) {
	c := routeCluster([]Vec3{{}, {X: 0.2}, {X: 0.4}}, []StarLane{{A: 0, B: 1}, {A: 1, B: 2}})
	e, s := routeEmpire(c)
	s.Lane, s.Arrive = 0, 5
	for _, tc := range []struct {
		dest int
		want []int
	}{{2, []int{1, 2}}, {1, []int{1}}, {0, []int{1, 0}}} {
		r, err := e.Route(s, tc.dest)
		if err != nil || !slices.Equal(r, tc.want) {
			t.Errorf("to %d: %v, %v; want %v", tc.dest, r, err, tc.want)
		}
		if lanes, days := e.RouteLanes(s, r); lanes[0] != 0 || days != laneDays(c, s, lanes...) {
			t.Errorf("to %d: lanes %v, %d days", tc.dest, lanes, days)
		}
	}
}

// On rolled clusters, under a partial fog, Route's days are the least
// days over the lanes known between stars known (a Bellman-Ford oracle),
// every lane known, every stop known and adjacent to the last; no route
// exactly when the oracle reaches nothing.
func TestRouteShortestOnClusters(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		g := NewGame(seed, []Species{Snovemdomas}, Average)
		e := g.Empires[0]
		c := e.Cluster
		home := e.Colonies[0]
		for i := range c.Stars {
			if i%11 != 3 {
				e.Visit(i)
			}
		}
		s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
		// The oracle: relax every allowed directed lane n times.
		best := make([]int, len(c.Stars))
		for i := range best {
			best[i] = -1
		}
		best[s.Star] = 0
		for range c.Stars {
			for li, l := range c.Lanes {
				for _, d := range [2][2]int{{l.A, l.B}, {l.B, l.A}} {
					u, v := d[0], d[1]
					if best[u] < 0 || !e.KnowsLane(li) || !e.Knows(v) {
						continue
					}
					if w := best[u] + laneDays(c, s, li); best[v] < 0 || w < best[v] {
						best[v] = w
					}
				}
			}
		}
		routed := 0
		for dest := range c.Stars {
			if dest == s.Star || !e.Knows(dest) {
				continue
			}
			r, err := e.Route(s, dest)
			if best[dest] < 0 {
				if err == nil {
					t.Errorf("seed %d: route to %d %v, the oracle reaches none", seed, dest, r)
				}
				continue
			}
			if err != nil {
				t.Errorf("seed %d: to %d: %v, the oracle's %d days", seed, dest, err, best[dest])
				continue
			}
			routed++
			lanes, days := e.RouteLanes(s, r)
			if len(lanes) != len(r) || r[len(r)-1] != dest || days != best[dest] {
				t.Errorf("seed %d: to %d: route %v lanes %v, %d days, oracle %d", seed, dest, r, lanes, days, best[dest])
			}
			for k, li := range lanes {
				if !e.KnowsLane(li) || !e.Knows(r[k]) {
					t.Errorf("seed %d: to %d: lane %d or star %d unknown", seed, dest, li, r[k])
				}
			}
		}
		if routed < 5 {
			t.Errorf("seed %d: only %d routes", seed, routed)
		}
	}
}

// A ship sent 3+ lanes away through Empire.Order passes the route's
// stars in order, keeps its order and makes no news on the way, and is
// out at the destination after the lanes' days plus the in-system legs
// (moveDays each, a day each for the lane exit's idle close), with one
// Arrived, the order done. The order is journalled as "system".
func TestGoToSystemArrives(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	c := e.Cluster
	home := e.Colonies[0]
	for i := range c.Stars {
		e.Visit(i)
	}
	s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	dest, r := -1, []int(nil)
	for i := range c.Stars {
		if x, err := e.Route(s, i); err == nil && len(x) >= 3 {
			dest, r = i, x
			break
		}
	}
	if dest < 0 {
		t.Fatal("fixture: no star 3 lanes away")
	}
	lanes, days := e.RouteLanes(s, r)
	slack, at, pos := 0, s.Star, s.Pos
	for _, to := range r {
		_, end, _ := laneEnd(c, at, to)
		slack += moveDays(s.Design, dist(pos, c.Stars[at].System.Lanes[end].Pos)+ShipGap) + 1
		_, back, _ := laneEnd(c, to, at)
		at, pos = to, c.Stars[to].System.Lanes[back].Pos
	}
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	if en := one(t, &buf, "Order system", "order", 0, e.Order(s, Order{Kind: GoToSystem, Star: dest})); en.Kind != "system" || en.Star != dest {
		t.Errorf("journal: %+v", en)
	}
	day0 := e.Day
	var passed []int
	arrivals := 0
	for range days + slack + 5 {
		before := s.Star
		evs := g.EndDay()[0]
		for _, x := range evs {
			if x.Kind == Arrived {
				arrivals++
				if s.Star != dest || x.Star != dest {
					t.Errorf("day %d: Arrived %q on the way", e.Day, x.Text)
				}
			}
		}
		if s.Star != before {
			passed = append(passed, s.Star)
		}
		if s.Lane < 0 && s.Star == dest {
			break
		}
		if s.Order.Kind != GoToSystem || s.Order.Star != dest {
			t.Fatalf("day %d: order %+v on the way", e.Day, s.Order)
		}
	}
	took := e.Day - day0
	if !slices.Equal(passed, r) || s.Star != dest || s.Lane >= 0 {
		t.Fatalf("passed %v, route %v; at %d lane %d", passed, r, s.Star, s.Lane)
	}
	if took < days || took > days+slack {
		t.Errorf("out at the destination after %d days, route %d lane days + %d slack (lanes %v)", took, days, slack, lanes)
	}
	if s.Order.Kind != NoOrder || arrivals != 1 {
		t.Errorf("order %+v kept, %d Arrived", s.Order, arrivals)
	}
}

// runTo closes e's days until done holds (at most n), the events kept.
func runTo(t *testing.T, e *Empire, n int, done func() bool) []Event {
	t.Helper()
	var evs []Event
	for range n {
		if done() {
			return evs
		}
		evs = append(evs, e.EndDay()...)
	}
	if !done() {
		t.Fatalf("not done in %d days", n)
	}
	return evs
}

// The route is recomputed at each stop: a lane learnt while the ship is
// in the first lane (S2 explored by others) is taken over the red lane
// it was heading for.
func TestRouteRecomputed(t *testing.T) {
	pos := []Vec3{{}, {X: 0.2}, {X: 0.3, Y: 0.15}, {X: 0.4}}
	ls := []StarLane{{A: 0, B: 1}, {A: 1, B: 3, Red: true}, {A: 1, B: 2}, {A: 2, B: 3}}
	for _, learn := range []bool{false, true} {
		c := routeCluster(pos, ls)
		e, s := routeEmpire(c)
		e.Visited[2] = false
		if err := e.Order(s, Order{Kind: GoToSystem, Star: 3}); err != nil {
			t.Fatal(err)
		}
		if r, _ := e.Route(s, 3); !slices.Equal(r, []int{1, 3}) {
			t.Fatalf("fixture: route %v", r)
		}
		runTo(t, e, 50, func() bool { return s.Lane == 0 })
		if learn {
			e.Visited[2] = true
		}
		var passed []int
		runTo(t, e, 200, func() bool {
			if n := len(passed); s.Lane < 0 && (n == 0 || passed[n-1] != s.Star) {
				passed = append(passed, s.Star)
			}
			return s.Star == 3 && s.Lane < 0
		})
		want := []int{1, 3}
		if learn {
			want = []int{1, 2, 3}
		}
		if !slices.Equal(passed, want) || s.Order.Kind != NoOrder {
			t.Errorf("learn %v: passed %v, want %v; order %+v", learn, passed, want, s.Order)
		}
	}
}

// No route left at a stop (the lane on is blocked meanwhile): the order
// is dropped at the next close with a NoRoute at the ship's star, and no
// Arrived for the stop on the way. Order refuses a route-less GoToSystem
// and one in a lane.
func TestGoToSystemNoRoute(t *testing.T) {
	c := routeCluster([]Vec3{{}, {X: 0.2}, {X: 0.4}}, []StarLane{{A: 0, B: 1}, {A: 1, B: 2}})
	e, s := routeEmpire(c)
	o := &Empire{Index: 1, Cluster: c}
	e.peers, o.peers = []*Empire{e, o}, []*Empire{e, o}
	if err := e.Order(s, Order{Kind: GoToSystem, Star: 2}); err != nil {
		t.Fatal(err)
	}
	runTo(t, e, 50, func() bool { return s.Lane == 0 })
	if err := e.Order(s, Order{Kind: GoToSystem, Star: 0}); err == nil {
		t.Error("an order in a lane")
	}
	o.LaneBlocks = map[int]int{1: 1000}
	evs := runTo(t, e, 200, func() bool { return s.Order.Kind != GoToSystem })
	if s.Star != 1 || s.Lane >= 0 || s.Order.Kind != NoOrder {
		t.Fatalf("star %d lane %d order %+v", s.Star, s.Lane, s.Order)
	}
	var kinds []EventKind
	for _, x := range evs {
		kinds = append(kinds, x.Kind)
	}
	if len(evs) != 1 || evs[0].Kind != NoRoute || evs[0].Star != 1 || evs[0].Planet != -1 || evs[0].Other != -1 ||
		evs[0].Text != "R has no known route to S2" {
		t.Errorf("events %v: %+v", kinds, evs)
	}
	if err := e.Order(s, Order{Kind: GoToSystem, Star: 2}); err == nil {
		t.Error("an order with no route")
	}
	if s.Order.Kind != NoOrder {
		t.Errorf("refused order kept: %+v", s.Order)
	}

	// Standing at the destination out of a lane (put there by other
	// means): done at the close, no news.
	s.Order = Order{Kind: GoToSystem, Star: 1}
	if evs := e.EndDay(); len(evs) != 0 || s.Order.Kind != NoOrder {
		t.Errorf("at the destination: events %+v, order %+v", evs, s.Order)
	}
	// A route but a ship that cannot move (no engine): the hop drops the
	// order as it drops a GoToStar's, not put back.
	o.LaneBlocks = nil
	s.Design = Design{Hull: Small, Parts: []Gizmo{ProtonShaver, StarLaneDrive}}
	s.Order = Order{Kind: GoToSystem, Star: 2}
	if _, err := e.Route(s, 2); err != nil {
		t.Fatal(err)
	}
	if e.EndDay(); s.Order.Kind != NoOrder || s.Star != 1 {
		t.Errorf("no engine: order %+v at %d", s.Order, s.Star)
	}
}

// A combat round's move flies the route's next lane as a GoToStar would
// (the order kept); with no route the round leaves the order to the
// close.
func TestGoToSystemRound(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	if err := SpawnFixtures(g, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	e := g.Empires[0]
	c := e.Cluster
	for i := range c.Stars {
		e.Visit(i)
	}
	s := e.Ships[0]
	home := s.Star
	dest := -1
	var r []int
	for i := range c.Stars {
		if x, err := e.Route(s, i); err == nil && len(x) >= 2 {
			dest, r = i, x
			break
		}
	}
	if dest < 0 {
		t.Fatal("fixture: no star 2 lanes away")
	}
	_, end, _ := laneEnd(c, home, r[0])
	entry := c.Stars[home].System.Lanes[end].Pos
	if err := e.Order(s, Order{Kind: GoToSystem, Star: dest}); err != nil {
		t.Fatal(err)
	}
	seen := slices.Clone(e.Visited)
	for i := range e.Visited {
		e.Visited[i] = i == home // dest, 2 lanes off, out of reach
	}
	from := s.Pos
	g.Round(home, nil)
	if s.Pos != from || s.Order != (Order{Kind: GoToSystem, Star: dest}) {
		t.Fatalf("no route: moved %v -> %v, order %+v", from, s.Pos, s.Order)
	}
	copy(e.Visited, seen)
	g.Round(home, nil)
	if dist(s.Pos, entry) >= dist(from, entry) || s.Order != (Order{Kind: GoToSystem, Star: dest}) {
		t.Errorf("round: %v -> %v, entry %v; order %+v", from, s.Pos, entry, s.Order)
	}
}
