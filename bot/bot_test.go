package bot

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

func TestDefaultPlaysAGalaxy(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	b := New(e, Default())
	for range 400 {
		b.Act()
		e.EndDay()
		b.Note()
	}
	if len(e.Colonies) < 2 {
		t.Fatalf("%d colonies after 400 days", len(e.Colonies))
	}
	if b.Founded[e.Colonies[0]] != 0 {
		t.Errorf("home founded on day %d", b.Founded[e.Colonies[0]])
	}
	for _, col := range e.Colonies[1:] {
		if d := b.Founded[col]; d < 1 || d > e.Day {
			t.Errorf("%s founded on day %d", col.Planet.Name, d)
		}
	}
}

func TestTryOrbitalWithoutSlot(t *testing.T) {
	col := game.NewColony(game.Snovemdomas.HomePlanet(1, "T"))
	r := game.NewResearch()
	r.Known[game.OrbitalStructures] = true
	for i := range col.Planet.Orbit {
		col.Planet.Orbit[i] = game.Shipyard
	}
	if err := Try(col, r, game.Shipyard); err == nil {
		t.Error("Shipyard started with every orbit slot taken")
	}
}

func TestPickTopicNone(t *testing.T) {
	r := game.NewResearch()
	q := []game.Tech{game.OrbitalStructures}
	PickTopic(r, &q, true)
	if r.Topic != game.NoTech || len(q) != 1 {
		t.Errorf("topic %v queue %v", r.Topic, q)
	}
	PickTopic(r, &q, false)
	if r.Topic == game.NoTech {
		t.Error("no topic picked")
	}
}

// NewAll gives each empire its bot, in order; a bot never counts another
// empire's planet as foundable (Inhabited reads Planet.Owner).
func TestNewAll(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	bots := NewAll(g, Default())
	if len(bots) != 2 {
		t.Fatalf("%d bots", len(bots))
	}
	for i, b := range bots {
		if b.E != g.Empires[i] || b.Founded[b.E.Colonies[0]] != 0 {
			t.Errorf("bot %d plays %v", i, b.E.Species)
		}
	}
	other := g.Empires[1].Colonies[0]
	bots[0].E.Visit(other.Star) // past the fog: only the Owner refuses
	// The other home has a square for a base; only its Owner refuses it.
	if _, ok := other.Planet.BestCell(game.ColonyBase, bots[0].E.Species.Traits()); ok {
		if bots[0].foundable(other.Star, 0) {
			t.Error("empire 0's bot would found on empire 1's home")
		}
		other.Planet.Owner = -1
		if !bots[0].foundable(other.Star, 0) {
			t.Error("an unowned home with room is not foundable")
		}
	} else {
		t.Fatal("empire 1's home has no square for a base: the fixture proves nothing")
	}
}

// warship is a Small ship with two Mass Barrage Guns and the power for
// both shots and a move.
var warship = game.Design{Name: "War", Hull: game.Small,
	Parts: []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.MassBarrageGun}}

// Day's round loop: the player (nil bot) plays round 0 only, its armed
// shot firing there; a bot ship keeps moving one Hop a round until its
// bot's round cap (RoundCap for a nil genome, else the Rounds gene); the
// star's ships do not Step at the close.
func TestDayRounds(t *testing.T) {
	for _, tc := range []struct {
		gn   *Genome
		hops int
	}{{nil, RoundCap}, {genomeWith(func(g *Genome) { g.Rounds = 2 }), 2}} {
		dayRounds(t, tc.gn, tc.hops)
	}
}

func dayRounds(t *testing.T, gn *Genome, hops int) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	bots := NewAll(g, Config{NoResearch: true, Genome: gn})
	bots[0] = nil
	home := g.Empires[0].Colonies[0]
	me := g.Empires[0].Spawn(warship, home.Star, home.PlanetIdx)
	me.Shields, me.Orbit = false, -1 // in space: a ship in orbit does not fire
	me.Pos = game.Vec3{Y: 1.3}       // clear of the planets' sweep: the close's push is no hop
	me.Refill()
	// A bot warship with energy for far more than RoundCap hops, far out
	// of the system: each round it fights, heading one Hop for its stand
	// off the player's ship (its gun armed, out of range: dropped).
	big := game.Design{Name: "Big", Hull: game.Enormous, Parts: []game.Gizmo{game.TonklinMotor, game.MassBarrageGun}}
	for len(big.Parts) < game.Enormous.Slots() {
		big.Parts = append(big.Parts, game.Nanotwirler)
	}
	mover := g.Empires[1].Spawn(big, home.Star, home.PlanetIdx)
	mover.Pos = game.Vec3{X: -100}
	mover.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: game.SystemReach}}
	if mover.Power < (RoundCap+1)*big.MoveCost() {
		t.Fatalf("fixture: energy %d", mover.Power)
	}
	foe := g.Empires[1].Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	foe.Shields, foe.Orbit = false, -1 // in space: hit directly
	foe.Power = 0                      // it stays: after the player's hop (0.2) it is still within the gun's range (0.25)
	sg, ok := g.SightingOf(g.Empires[0], foe)
	if !ok {
		t.Fatal("the foe is not seen")
	}
	p := slices.Index(warship.Parts, game.MassBarrageGun)
	if err := g.Empires[0].Arm(me, p, sg); err != nil {
		t.Fatal(err)
	}
	me.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{Z: game.SystemReach}}
	mePos, hull, start := me.Pos, foe.Hull, mover.Pos
	foe.Pos = me.Pos
	Day(g, bots)
	w := game.Gizmos[game.MassBarrageGun]
	if foe.Hull != hull-w.Level {
		t.Errorf("the player's shot: foe hull %d of %d", foe.Hull, hull)
	}
	// Round 0 only: one hop (two generators: the shot and the move).
	if got, want := dist(me.Pos, mePos), warship.Reach(); math.Abs(got-want) > 1e-9 {
		t.Errorf("the player moved %.3f, want one hop %.3f", got, want)
	}
	// Its stand moves with the player's round-0 hop: a bend of ~1e-3 rad
	// at ~100 units, far below a hop.
	if got, want := dist(mover.Pos, start), float64(hops)*big.Reach(); math.Abs(got-want) > 1e-4 {
		t.Errorf("the mover went %.5f, want %d hops %.5f", got, hops, want)
	}
}

// A two-empire fixture fight, played twice from the same start, ends the
// same; its destructions report to both sides.
func TestDayFightDeterministic(t *testing.T) {
	play := func() (digest []int, reps []game.Report) {
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
		if err := game.SpawnFixtures(g, 2, 1, 2); err != nil {
			t.Fatal(err)
		}
		for _, s := range g.Empires[1].Ships {
			s.Shields = false
		}
		// No flight (each side overrates the other's unscanned Smalls).
		bots := NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.FleeRatio = 1e9 })})
		for range 30 {
			_, r, _ := Day(g, bots)
			reps = append(reps, r...)
		}
		for _, e := range g.Empires {
			digest = append(digest, len(e.Ships))
			for _, s := range e.Ships {
				digest = append(digest, s.Hull, s.Power)
			}
		}
		return digest, reps
	}
	d1, r1 := play()
	d2, r2 := play()
	if !slices.Equal(d1, d2) || !reflect.DeepEqual(r1, r2) {
		t.Fatalf("not deterministic: %v vs %v", d1, d2)
	}
	sides := map[int]bool{}
	for _, r := range r1 {
		sides[r.Empire] = true
	}
	if !sides[0] || !sides[1] {
		t.Errorf("reports %+v: want both sides", r1)
	}
	t.Logf("digest %v, %d reports", d1, len(r1))
}

// scout is a Small ship with a lane drive and two Colonizers.
var scout = game.Design{Name: "Scout", Hull: game.Small,
	Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.StarLaneDrive, game.Colonizer, game.Colonizer}}

// claimAll claims every planet of star.
func claimAll(c *game.Cluster, star int, claimed map[place]bool) {
	for pl := range c.Stars[star].System.Planets {
		claimed[place{star, pl}] = true
	}
}

// The fog hides the planets of a star the empire does not Know: no
// target there, so a lane ship whose home system is all claimed explores
// a neighbour instead of flying to a planet; once visited, the star's
// planets count. reachable takes an unknown neighbour for a reason with a
// lane drive, never without.
func TestFogHidesTargets(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	e, c := b.E, g.Cluster
	home := e.Colonies[0]
	far := -1
	for _, li := range c.Stars[home.Star].Lanes {
		n := c.Other(li, home.Star)
		for pl := range c.Stars[n].System.Planets {
			if !e.Inhabited(n, pl) {
				if _, ok := e.Planet(n, pl).BestCell(game.ColonyBase, e.Species.Traits()); ok && far < 0 {
					far = n
				}
			}
		}
	}
	if far < 0 {
		t.Fatal("no neighbour of the home with a foundable planet: the fixture proves nothing")
	}
	if e.Knows(far) || b.targets(far, map[place]bool{}) != 0 {
		t.Fatalf("star %d known %v before any visit", far, e.Knows(far))
	}
	claimed := map[place]bool{}
	claimAll(c, home.Star, claimed)
	if !b.reachable(home, scout, claimed) {
		t.Error("an unknown neighbour is no reason for a lane ship")
	}
	if b.reachable(home, game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}, claimed) {
		t.Error("an unknown neighbour is a reason for a ship without a lane drive")
	}
	s := e.Spawn(scout, home.Star, home.PlanetIdx)
	b.planShip(s, claimed)
	if s.Order.Kind != game.GoToStar || e.Knows(s.Order.Star) {
		t.Errorf("order %+v: want GoToStar an unknown star", s.Order)
	}
	e.Visit(far)
	if n := b.targets(far, map[place]bool{}); n == 0 {
		t.Errorf("visited star %d: no target", far)
	}
	// Every neighbour known, the home claimed: the most targets win (far
	// has some), no exploring.
	for _, li := range c.Stars[home.Star].Lanes {
		e.Visit(c.Other(li, home.Star))
	}
	s.Order = game.Order{}
	b.planShip(s, claimed)
	// The ship's Colonizers now count as inbound to its destination: take
	// them back out to see the targets it chose.
	d := b.dest[s]
	if b.inbound[d] != s.Cargo {
		t.Errorf("inbound to %d: %d, want the ship's cargo %d", d, b.inbound[d], s.Cargo)
	}
	b.inbound[d] -= s.Cargo
	if s.Order.Kind != game.GoToStar || b.targets(d, claimed) == 0 {
		t.Errorf("order %+v to %d: want GoToStar a star with targets", s.Order, d)
	}
}

// Exploring picks the unknown neighbour of fewest lane days (ties: lane
// order); with every neighbour known and no target, the ship stays idle.
func TestExploreNearest(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	e, c := b.E, g.Cluster
	home := e.Colonies[0]
	claimed := map[place]bool{}
	claimAll(c, home.Star, claimed)
	s := e.Spawn(scout, home.Star, home.PlanetIdx)
	// nearest is the oracle: the first unknown neighbour of fewest days.
	nearest := func() int {
		to, toD := -1, 0
		for _, li := range c.Stars[home.Star].Lanes {
			n, d := c.Other(li, home.Star), game.LaneDays(c, li, 1, e.Species.Traits())
			if !e.Knows(n) && (to < 0 || d < toD) {
				to, toD = n, d
			}
		}
		return to
	}
	lanes := len(c.Stars[home.Star].Lanes)
	if lanes < 2 {
		t.Fatalf("home has %d lanes: the fixture proves nothing", lanes)
	}
	for range lanes {
		want := nearest()
		s.Order = game.Order{}
		b.planShip(s, claimed)
		if s.Order.Kind != game.GoToStar || s.Order.Star != want {
			t.Fatalf("order %+v: want GoToStar %d", s.Order, want)
		}
		// Known, claimed: the next pick is the next nearest.
		e.Visit(want)
		claimAll(c, want, claimed)
	}
	// Every neighbour known and claimed: the ship routes past them, its
	// order the first hop, its destination route's, kept in dest.
	s.Order = game.Order{}
	to, first := b.route(home.Star, 1, e.Species.Traits(), claimed, nil)
	if b.planShip(s, claimed); to < 0 || s.Order.Kind != game.GoToStar || s.Order.Star != first || b.dest[s] != to || !claimed[place{to, -1}] {
		t.Errorf("every neighbour known and claimed: order %+v dest %d, want first hop %d to %d", s.Order, b.dest[s], first, to)
	}
}

// Day: the bots' reports (a nil bot skipped), one events slice per
// empire, the day closed and noted.
func TestDay(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	bots := NewAll(g, Config{NoResearch: true})
	bots[1] = nil
	home := g.Empires[0].Colonies[0]
	sh := g.Empires[0].Spawn(warship, home.Star, home.PlanetIdx)
	foe := g.Empires[1].Spawn(warship, home.Star, home.PlanetIdx)
	foe.Shields, foe.Pos, foe.Orbit = false, sh.Pos, -1 // in space: hit directly
	foe.Hull = game.Gizmos[game.MassBarrageGun].Level   // one shot
	evs, reps, _ := Day(g, bots)
	if len(evs) != 2 || g.Day != 1 {
		t.Fatalf("%d event slices, day %d", len(evs), g.Day)
	}
	if len(reps) != 2 || reps[0].Empire != 0 || reps[1].Empire != 1 || reps[0].Event.Kind != game.Destroyed {
		t.Errorf("reports %+v", reps)
	}
	if bots[0].invading != nil { // Act sets it, Note drops it
		t.Error("not noted")
	}
}

// oracle is route's distances by plain relaxation over c.Lanes until
// stable: (days, hops) from star from, lexicographic, passing only
// through from and the stars e Knows. -1 days: unreached.
func oracle(e *game.Empire, c *game.Cluster, from int, strength float64) (days, hops []int) {
	days, hops = make([]int, len(c.Stars)), make([]int, len(c.Stars))
	for i := range days {
		days[i] = -1
	}
	days[from] = 0
	for changed := true; changed; {
		changed = false
		for li, l := range c.Lanes {
			for _, uv := range [2][2]int{{l.A, l.B}, {l.B, l.A}} {
				u, v := uv[0], uv[1]
				if days[u] < 0 || u != from && !e.Knows(u) {
					continue
				}
				d, h := days[u]+game.LaneDays(c, li, strength, e.Species.Traits()), hops[u]+1
				if days[v] < 0 || d < days[v] || d == days[v] && h < hops[v] {
					days[v], hops[v], changed = d, h, true
				}
			}
		}
	}
	return days, hops
}

// nearestWhere is the reached star other than from of least (days, hops,
// index) satisfying ok, -1 none.
func nearestWhere(days, hops []int, from int, ok func(int) bool) int {
	best := -1
	for v := range days {
		if v == from || days[v] < 0 || !ok(v) {
			continue
		}
		if best < 0 || days[v] < days[best] || days[v] == days[best] && hops[v] < hops[best] {
			best = v
		}
	}
	return best
}

// checkRoute asserts b.route from from matches the oracle: the target
// star of least days less ruinW for ruins (ruinsTarget), ties the nearest
// (nearestWhere's order), else the nearest unexplored one, and a first
// hop adjacent to from that starts a shortest path to it.
func checkRoute(t *testing.T, b *Bot, from int, claimed map[place]bool) (to, first int) {
	t.Helper()
	e, c := b.E, b.E.Cluster
	days, hops := oracle(e, c, from, 1)
	cost := func(v int) float64 {
		if b.ruinsTarget(v, claimed) {
			return float64(days[v]) - b.ruinW()
		}
		return float64(days[v])
	}
	want := -1
	for v := range c.Stars {
		if v == from || days[v] < 0 || b.targets(v, claimed) == 0 {
			continue
		}
		if want < 0 || cost(v) < cost(want) || cost(v) == cost(want) && nearestWhere(days, hops, from, func(u int) bool { return u == v || u == want }) == v {
			want = v
		}
	}
	if want < 0 {
		want = nearestWhere(days, hops, from, func(v int) bool { return b.unexplored(v, claimed) })
	}
	to, first = b.route(from, 1, e.Species.Traits(), claimed, nil)
	if to != want {
		t.Fatalf("route from %d: to %d, want %d", from, to, want)
	}
	if to < 0 {
		return to, first
	}
	li, _, ok := laneOf(c, from, first)
	if !ok {
		t.Fatalf("first hop %d not adjacent to %d", first, from)
	}
	via := game.LaneDays(c, li, 1, e.Species.Traits())
	if first != to {
		if !e.Knows(first) {
			t.Fatalf("first hop %d unknown, yet passed through to %d", first, to)
		}
		d, _ := oracle(e, c, first, 1)
		via += d[to]
	}
	if via != days[to] {
		t.Errorf("path via %d takes %d days, the shortest %d", first, via, days[to])
	}
	return to, first
}

// laneOf is the lane between adjacent stars a and b.
func laneOf(c *game.Cluster, a, b int) (lane, k int, ok bool) {
	for k, li := range c.Stars[a].Lanes {
		if c.Other(li, a) == b {
			return li, k, true
		}
	}
	return -1, -1, false
}

// route crosses known stars and stops at unknown ones (leaves): with the
// home's neighbours unknown and claimed as destinations, nothing beyond
// them is reached, not even a visited star with targets further out;
// once the stars between are known the ship goes through them, to that
// target star rather than to an unexplored one; a claimed destination is
// skipped for the next.
func TestRoute(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	e, c := b.E, g.Cluster
	home := e.Colonies[0].Star
	claimed := map[place]bool{}
	claimAll(c, home, claimed)
	adjacent := map[int]bool{}
	var adj []int // lane order: the fixture's picks are deterministic
	for _, li := range c.Stars[home].Lanes {
		adjacent[c.Other(li, home)] = true
		adj = append(adj, c.Other(li, home))
	}
	// beyond: the nearest star (BFS hops, lane order) not adjacent to the
	// home with a foundable planet once visited; chain the stars between.
	parent := make([]int, len(c.Stars))
	for i := range parent {
		parent[i] = -2
	}
	parent[home] = -1
	beyond := -1
	for q := []int{home}; len(q) > 0 && beyond < 0; q = q[1:] {
		for _, li := range c.Stars[q[0]].Lanes {
			v := c.Other(li, q[0])
			if parent[v] != -2 {
				continue
			}
			parent[v], q = q[0], append(q, v)
			if adjacent[v] || beyond >= 0 {
				continue
			}
			e.Visit(v)
			if b.targets(v, claimed) > 0 {
				beyond = v
			}
			e.Visited[v] = false
		}
	}
	if beyond < 0 {
		t.Fatal("no star past the home's neighbours with a foundable planet: the fixture proves nothing")
	}
	var chain []int // the stars between the home and beyond
	for v := parent[beyond]; v != home; v = parent[v] {
		chain = append(chain, v)
	}
	if len(chain) == 0 {
		t.Fatal("beyond is adjacent")
	}
	e.Visit(beyond)
	for _, n := range adj {
		claimed[place{n, -1}] = true
	}
	if to, _ := checkRoute(t, b, home, claimed); to != -1 {
		t.Errorf("neighbours unknown and claimed: route reached %d through an unknown star", to)
	}
	for _, n := range adj {
		delete(claimed, place{n, -1})
	}

	// The chain known, its planets claimed: the target star beyond (the
	// only known one with targets) wins over the unexplored neighbours.
	for _, v := range chain {
		e.Visit(v)
		claimAll(c, v, claimed)
	}
	unexplored := 0
	for v := range c.Stars {
		if b.unexplored(v, claimed) {
			unexplored++
		}
	}
	if unexplored == 0 {
		t.Fatal("every star known: no unexplored alternative to prefer the target over")
	}
	if to, first := checkRoute(t, b, home, claimed); to != beyond || first != chain[len(chain)-1] {
		t.Errorf("to %d first %d: want %d through %v", to, first, beyond, chain)
	}

	// No target left: the nearest unexplored star; claimed, the next one.
	claimAll(c, beyond, claimed)
	for v := range c.Stars {
		if e.Knows(v) {
			claimAll(c, v, claimed)
		}
	}
	to, _ := checkRoute(t, b, home, claimed)
	if to < 0 || e.Knows(to) {
		t.Fatalf("to %d: want an unexplored star", to)
	}
	claimed[place{to, -1}] = true
	if next, _ := checkRoute(t, b, home, claimed); next == to || next < 0 {
		t.Errorf("claimed %d: route picked %d", to, next)
	}
}

// A ship's far destination is claimed by claimed() while it is under way
// (its order names only the first hop), dropped once idle; Save/Resume
// keep it.
func TestDestClaimedAndSaved(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	e, c := b.E, g.Cluster
	home := e.Colonies[0]
	claimed := map[place]bool{}
	claimAll(c, home.Star, claimed)
	for _, li := range c.Stars[home.Star].Lanes {
		n := c.Other(li, home.Star)
		e.Visit(n)
		claimAll(c, n, claimed)
	}
	e.Spawn(scout, home.Star, home.PlanetIdx) // idle, first: Dest[0] is -1
	s := e.Spawn(scout, home.Star, home.PlanetIdx)
	b.planShip(s, claimed)
	to := b.dest[s]
	if s.Order.Kind != game.GoToStar || to == s.Order.Star {
		t.Fatalf("order %+v dest %d: want a first hop short of a far destination", s.Order, to)
	}
	if !b.claimed()[place{to, -1}] {
		t.Errorf("destination %d not claimed", to)
	}
	st := b.Save()
	if !reflect.DeepEqual(st.Dest, []int{-1, to}) {
		t.Errorf("saved Dest %v, want [-1 %d]", st.Dest, to)
	}
	r := Resume(e, st)
	if len(r.dest) != 1 || r.dest[s] != to || !reflect.DeepEqual(r.Save(), st) {
		t.Errorf("resumed dest %v", r.dest)
	}
	s.Order = game.Order{}
	if b.claimed()[place{to, -1}] || len(b.dest) != 0 {
		t.Errorf("idle ship: destination %d still claimed, dest %v", to, b.dest)
	}
}

// The stall regression: seed 1 Average solo used to stop at 4 systems
// from day ~700 (every neighbour explored and barren); route reaches the
// stars two lanes and more away (32 systems on day 1500).
func TestSoloKeepsExpanding(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	bots := NewAll(g, Default())
	e := g.Empires[0]
	for g.Day < 1500 {
		Day(g, bots)
		if g.Day != 1250 {
			continue
		}
		// spare: no fleet built for nowhere. Before it, 42 of 84 ships
		// sat loaded with no destination on day 1250.
		idle := 0
		for _, s := range e.Ships {
			if s.Cargo > 0 && s.Lane < 0 && s.Order.Kind == game.NoOrder {
				idle++
			}
		}
		if idle > 1 && idle*10 > len(e.Ships) { // one just launched waits its day

			t.Errorf("day 1250: %d of %d ships idle and loaded", idle, len(e.Ships))
		}
	}
	if n := e.ControlledSystems(); n <= 4 {
		t.Errorf("%d systems after 1500 days, the stall's 4", n)
	}
}

// spare: an idle loaded ship takes its Colonizers off the budget when it
// can go somewhere (a lane drive), not when it is stranded (no drive, no
// target in its system); a Colonizer under construction takes its share
// too. Counting stranded ships froze seed 3's Swaparamans and Baliflids
// in their home system for 1000 days.
func TestSpareCountsReachableShips(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	b := NewAll(g, Default())[0]
	e, c := g.Empires[0], g.Cluster
	for st := range c.Stars {
		e.Visit(st)
	}
	bare := -1
	for st := range c.Stars {
		if b.targets(st, map[place]bool{}) == 0 {
			bare = st
			break
		}
	}
	if bare < 0 {
		t.Fatal("no star without a target on seed 1")
	}
	b.claimed()
	base := b.spare
	stranded := e.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}, bare, 0)
	if b.claimed(); stranded.Cargo < 1 || b.spare != base {
		t.Errorf("stranded ship (cargo %d): spare %d, want %d", stranded.Cargo, b.spare, base)
	}
	lane := e.Spawn(game.FixtureColonyShip, bare, 0)
	if b.claimed(); b.spare != base-lane.Cargo {
		t.Errorf("lane ship (cargo %d): spare %d, want %d", lane.Cargo, b.spare, base-lane.Cargo)
	}
	home := e.Colonies[0]
	home.Project = &game.Project{Item: game.Ship, Design: &game.FixtureColonyShip}
	if b.claimed(); b.spare != base-lane.Cargo-game.FixtureColonyShip.Colonizers() {
		t.Errorf("a colony ship under construction: spare %d, want %d", b.spare, base-lane.Cargo-game.FixtureColonyShip.Colonizers())
	}
}

// A planet another empire guards (its ship in orbit) is no colonisation
// target: not foundable, so planShip never orders a GoToPlanet there (a
// refused order, retried daily) and a ship without a lane drive stays
// idle; unguarded, the same ship goes. Over days of play the bot never
// heads for a barred orbit.
func TestGuardedNotTargeted(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	e, e1 := b.E, g.Empires[1]
	home := e.Colonies[0]
	var pls []int
	for pl := range g.Cluster.Stars[home.Star].System.Planets {
		if b.foundable(home.Star, pl) {
			pls = append(pls, pl)
		}
	}
	if len(pls) == 0 {
		t.Fatal("no foundable planet at home: the fixture proves nothing")
	}
	inSystem := game.Design{Name: "Settler", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}
	s := e.Spawn(inSystem, home.Star, home.PlanetIdx)
	b.planShip(s, map[place]bool{})
	if s.Order.Kind != game.GoToPlanet {
		t.Fatalf("unguarded: order %+v, want a GoToPlanet", s.Order)
	}
	s.Order = game.Order{}
	hulk := game.Design{Name: "Hulk", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver}}
	for _, pl := range pls {
		e1.Spawn(hulk, home.Star, pl)
		if b.foundable(home.Star, pl) {
			t.Errorf("planet %d guarded by empire 1: foundable", pl)
		}
	}
	claimed := map[place]bool{}
	b.planShip(s, claimed)
	if s.Order.Kind != game.NoOrder || b.reachable(home, inSystem, claimed) {
		t.Fatalf("guarded: order %+v, reachable %v", s.Order, b.reachable(home, inSystem, claimed))
	}
	for day := range 60 {
		Day(g, []*Bot{b, nil})
		for _, sh := range e.Ships {
			if sh.Lane < 0 && sh.Order.Kind == game.GoToPlanet && e.Barred(sh.Star, sh.Order.Planet) {
				t.Fatalf("day %d: %s heads for barred planet %d", day, sh.Name, sh.Order.Planet)
			}
		}
	}
	if s.Order.Kind != game.NoOrder || s.Orbit != home.PlanetIdx {
		t.Errorf("after 60 days: order %+v orbit %d, want idle at home", s.Order, s.Orbit)
	}
}
