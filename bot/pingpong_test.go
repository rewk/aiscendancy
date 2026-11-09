package bot

import (
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// laner is a one-gun warship with a lane drive: its Mass Barrage Gun
// fires once a day (Uses 1), then its shots are spent.
var laner = game.Design{Name: "Laner", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.TonklinMotor, game.StarLaneDrive, game.MassBarrageGun}}

// shieldedHulk is a harmless hostile that a Mass Barrage Gun cannot
// scratch: no weapon, an Ion Wrap (absorbs the gun's Level 1).
var shieldedHulk = game.Design{Name: "Hulk", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.IonWrap}}

// The ping-pong detector: a warship at its home facing a hulk it cannot
// scratch (the threat never changes). Over 20 days the warship never
// comes back to a star it left while hostiles were there.
//   - spent: HullThreat 0 (the hulk a target); its one shot a day spent,
//     it must not flee by lane only for attack to send it back;
//   - pressure: HullThreat 50 (the hulk looks deadly); it flees home, and
//     attack, with no target, must not send it back to its colony star.
func TestNoPingPong(t *testing.T) {
	for _, tc := range []struct {
		name   string
		threat float64
	}{{"spent", 0}, {"pressure", 50}} {
		g, b, home, _ := fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small] = tc.threat }))
		me := b.E.Spawn(laner, home, 0)
		foe := put(g.Empires[1], shieldedHulk, home, game.Vec3{X: 0.3})
		if !armed(me.Design) || game.DesignDamage(laner, 1) != 0 {
			t.Fatalf("fixture: armed %v, damage through the wrap %d", armed(me.Design), game.DesignDamage(laner, 1))
		}
		bots := []*Bot{b, nil}
		fled := map[int]bool{}
		var trace []int
		for day := range 20 {
			at, out, hostile := me.Star, me.Lane < 0, len(g.Hostiles(b.E, me.Star)) > 0
			Day(g, bots)
			if !slices.Contains(b.E.Ships, me) {
				t.Fatalf("%s day %d: the warship is gone", tc.name, day)
			}
			if foe.Pos != (game.Vec3{X: 0.3}) || foe.Hull != foe.MaxHull() {
				t.Fatalf("%s day %d: the hulk changed: pos %+v hull %d", tc.name, day, foe.Pos, foe.Hull)
			}
			if me.Lane >= 0 {
				trace = append(trace, -1)
			} else {
				trace = append(trace, me.Star)
			}
			if out && hostile && (me.Lane >= 0 || me.Star != at) {
				fled[at] = true
			}
			if me.Lane < 0 && fled[me.Star] {
				t.Errorf("%s day %d: back at %d, left under the same threat (trace %v, -1 = in a lane)", tc.name, day, me.Star, trace)
				break
			}
		}
		t.Logf("%s: trace %v", tc.name, trace)
	}
}

// search never reaches a blocked star: never settled (stop never sees
// it), no path through it (the costs are a relaxation that skips it, the
// stars behind it only reached the long way or not at all), from never
// asked, each star asked at most once.
func TestSearchBlocked(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	b := NewAll(g, Config{NoResearch: true})[0]
	e, c := b.E, g.Cluster
	for st := range c.Stars {
		e.Visit(st)
	}
	from := e.Colonies[0].Star
	tr := e.Species.Traits()
	// oracle: lane days by relaxation, never into m.
	oracle := func(m int) []float64 {
		d := make([]float64, len(c.Stars))
		for i := range d {
			d[i] = math.Inf(1)
		}
		d[from] = 0
		for changed := true; changed; {
			changed = false
			for li, l := range c.Lanes {
				for _, uv := range [2][2]int{{l.A, l.B}, {l.B, l.A}} {
					u, v := uv[0], uv[1]
					if v == m || math.IsInf(d[u], 1) {
						continue
					}
					if x := d[u] + float64(game.LaneDays(c, li, 1, tr)); x < d[v] {
						d[v], changed = x, true
					}
				}
			}
		}
		return d
	}
	free := oracle(-1)
	detours := 0
	for _, m := range neighbours(c, from) {
		want := oracle(m)
		asked := map[int]int{}
		var order []int
		to, _ := b.search(from, 1, tr, 0, func(v int) bool { asked[v]++; return v == m }, func(u, first int) bool {
			if first == m {
				t.Errorf("block %d: %d settled through it", m, u)
			}
			order = append(order, u)
			return false
		})
		if to != -1 {
			t.Errorf("block %d: returned %d", m, to)
		}
		if asked[from] > 0 {
			t.Errorf("block %d: from asked", m)
		}
		for v, n := range asked {
			if n > 1 {
				t.Errorf("block %d: star %d asked %d times", m, v, n)
			}
		}
		settled := map[int]bool{}
		last := 0.0
		for _, u := range order {
			if u == m {
				t.Errorf("block %d settled", m)
			}
			if want[u] < last {
				t.Errorf("block %d: %d settled at cost %v after %v", m, u, want[u], last)
			}
			last, settled[u] = want[u], true
		}
		for v := range c.Stars {
			if reach := v != from && !math.IsInf(want[v], 1); reach != settled[v] {
				t.Errorf("block %d: star %d settled %v, oracle reaches it %v", m, v, settled[v], reach)
			}
			if want[v] > free[v] {
				detours++
			}
		}
	}
	if detours == 0 {
		t.Fatal("no block lengthened a path: the fixture proves nothing")
	}
	// nil blocks nothing.
	n := 0
	b.search(from, 1, tr, 0, nil, func(u, _ int) bool { n++; return false })
	reach := 0
	for v := range c.Stars {
		if v != from && !math.IsInf(free[v], 1) {
			reach++
		}
	}
	if n != reach {
		t.Errorf("nil blocked: %d settled, want %d", n, reach)
	}
}

// refuge prefers an adjacent star not in danger for the ship, even
// against more hostile damage seen there (my strong warship holds it);
// when every neighbour is in danger, the least damage as before.
func TestRefugeAvoidsDanger(t *testing.T) {
	// HullThreat 8: one unscanned Raider outguns the laner.
	g, b, home, adj := fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small] = 8 }))
	if len(adj) != 2 {
		t.Fatalf("fixture: home %d has %d lanes, want 2", home, len(adj))
	}
	e1 := g.Empires[1]
	me := b.E.Spawn(laner, home, 0)
	e1.Spawn(game.FixtureRaider, adj[0], 0)
	e1.Spawn(game.FixtureRaider, adj[1], 0)
	e1.Spawn(game.FixtureRaider, adj[1], 0)
	big := b.E.Spawn(laneGunboat(t), adj[1], 0)
	b.assess()
	danger := b.danger(me)
	if !danger(adj[0]) || danger(adj[1]) || b.their(adj[0]).dmg >= b.their(adj[1]).dmg {
		t.Fatalf("fixture: danger %v/%v, damage %v/%v", danger(adj[0]), danger(adj[1]), b.their(adj[0]).dmg, b.their(adj[1]).dmg)
	}
	if v := b.refuge(me); v != adj[1] {
		t.Errorf("refuge %d, want the safe %d", v, adj[1])
	}
	b.E.Ships = slices.DeleteFunc(b.E.Ships, func(s *game.Starship) bool { return s == big })
	b.assess()
	if danger := b.danger(me); !danger(adj[0]) || !danger(adj[1]) {
		t.Fatalf("fixture: danger %v/%v without the gunboat", danger(adj[0]), danger(adj[1]))
	}
	if v := b.refuge(me); v != adj[0] {
		t.Errorf("all in danger: refuge %d, want the least damage %d", v, adj[0])
	}
}

// danger: an unarmed ship is in danger wherever hostile damage is seen;
// an armed one where the pressure on my ships there with it (counted
// once, here or not) exceeds FleeRatio; nowhere without hostiles.
func TestDanger(t *testing.T) {
	g, b, home, adj := fleeFixture(t, nil)
	e1 := g.Empires[1]
	cs := b.E.Spawn(game.FixtureColonyShip, home, 0)
	me := b.E.Spawn(laner, home, 0)
	b.assess()
	for v := range g.Cluster.Stars {
		if b.danger(cs)(v) || b.danger(me)(v) {
			t.Fatalf("no hostile: star %d in danger", v)
		}
	}
	e1.Spawn(game.FixtureRaider, adj[0], 0)
	b.assess()
	t0 := b.their(adj[0])
	pAway := pressure(t0, shipSide(me))
	if !b.danger(cs)(adj[0]) || b.danger(cs)(home) {
		t.Error("unarmed: danger is not 'damage seen'")
	}
	if got := b.danger(me)(adj[0]); got != (pAway > b.cg.FleeRatio) {
		t.Errorf("armed: danger %v at pressure %v", got, pAway)
	}
	// Counted once: at adj[0] itself the same.
	me.Star = adj[0]
	if got := b.danger(me)(adj[0]); got != (pAway > b.cg.FleeRatio) {
		t.Errorf("armed, there: danger %v at pressure %v", got, pAway)
	}
	// FleeRatio just above / below that pressure.
	for _, tc := range []struct {
		ratio float64
		want  bool
	}{{pAway * 1.01, false}, {pAway * 0.99, true}} {
		b.Cfg.Genome = genomeWith(func(g *Genome) { g.FleeRatio = tc.ratio })
		b.assess()
		if got := b.danger(me)(adj[0]); got != tc.want {
			t.Errorf("FleeRatio %v, pressure %v: danger %v", tc.ratio, pAway, got)
		}
	}
}

// A colony ship's route never crosses a star in danger for it, even at
// ThreatAvoid 0 (where the plain route goes through): planShip passes
// danger. The detour configuration is TestThreatAvoid's search, redone.
func TestPlanShipAvoidsDanger(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	b := NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.ThreatAvoid = 0 })})[0]
	e, c := b.E, g.Cluster
	for st := range c.Stars {
		e.Visit(st)
	}
	tr := e.Species.Traits()
	n := len(c.Stars)
	// (from, to, m): every planet but to's claimed, the plain route's
	// first hop m, and to still reachable with m blocked.
	from, to, m := -1, -1, -1
	claimed := map[place]bool{}
	for a := 0; a < n && from < 0; a++ {
		for z := range n {
			if z == a {
				continue
			}
			claimed = map[place]bool{}
			for st := range c.Stars {
				if st != z {
					claimAll(c, st, claimed)
				}
			}
			b.assess()
			got, first := b.route(a, 1, tr, claimed, nil)
			if got != z || first == z {
				continue
			}
			if alt, _ := b.route(a, 1, tr, claimed, func(v int) bool { return v == first }); alt == z {
				from, to, m = a, z, first
				break
			}
		}
	}
	if from < 0 {
		t.Fatal("no detour configuration on seed 1")
	}
	g.Empires[1].Spawn(game.FixtureRaider, m, 0)
	b.assess()
	if got, first := b.route(from, 1, tr, claimed, nil); got != to || first != m {
		t.Fatalf("fixture: ThreatAvoid 0 route %d via %d, want %d via %d", got, first, to, m)
	}
	cs := b.E.Spawn(game.FixtureColonyShip, from, 0)
	cs.Orbit = -1
	b.planShip(cs, claimed)
	if cs.Order.Kind != game.GoToStar || cs.Order.Star == m || b.dest[cs] != to {
		t.Errorf("order %+v dest %d: want a GoToStar away from %d toward %d", cs.Order, b.dest[cs], m, to)
	}
}

// Clamp keeps AttackRatio at most FleeRatio, after bounding each.
func TestClampAttackBelowFlee(t *testing.T) {
	for _, tc := range []struct{ flee, attack, wantFlee, wantAttack float64 }{
		{1, 0.5, 1, 0.5},
		{0.3, 2, 0.3, 0.3},
		{20, 15, 10, 10},
		{0.01, 0.02, 0.05, 0.05},
		{math.NaN(), 3, 0.05, 0.05},
		{2, 2, 2, 2},
	} {
		g := Neutral()
		g.FleeRatio, g.AttackRatio = tc.flee, tc.attack
		g.Clamp()
		if g.FleeRatio != tc.wantFlee || g.AttackRatio != tc.wantAttack {
			t.Errorf("Clamp(flee %v, attack %v): %v, %v, want %v, %v", tc.flee, tc.attack, g.FleeRatio, g.AttackRatio, tc.wantFlee, tc.wantAttack)
		}
	}
}

// attack never heads through a star in danger for the warship: the
// nearest weak target's plain path starts with m, where a strong
// hostile sits; the order is a detour (the same configuration search as
// TestPlanShipAvoidsDanger, on search itself).
func TestAttackAvoidsDanger(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	b := NewAll(g, Config{NoResearch: true, Genome: genomeWith(func(g *Genome) { g.HullThreat[game.Small], g.HullThreat[game.Medium] = 8, 0 })})[0]
	e, c := b.E, g.Cluster
	for st := range c.Stars {
		e.Visit(st)
	}
	tr := e.Species.Traits()
	n := len(c.Stars)
	b.assess()
	from, to, m := -1, -1, -1
	for a := 0; a < n && from < 0; a++ {
		for z := range n {
			if z == a {
				continue
			}
			got, first := b.search(a, 1, tr, 0, nil, func(u, _ int) bool { return u == z })
			if got != z || first == z {
				continue
			}
			if alt, _ := b.search(a, 1, tr, 0, func(v int) bool { return v == first }, func(u, _ int) bool { return u == z }); alt == z {
				from, to, m = a, z, first
				break
			}
		}
	}
	if from < 0 {
		t.Fatal("no detour configuration on seed 1")
	}
	e1 := g.Empires[1]
	e1.Ships = nil
	e1.Spawn(game.Design{Name: "Big", Hull: game.Medium, Parts: unarmedScout.Parts}, to, 0) // HullThreat 0: pressure 0, a target
	e1.Spawn(game.FixtureRaider, m, 0)                                                      // HullThreat 8: the laner would flee there
	me := b.E.Spawn(laner, from, 0)
	b.assess()
	if !b.danger(me)(m) || b.danger(me)(to) {
		t.Fatalf("fixture: danger at m %v, at the target %v", b.danger(me)(m), b.danger(me)(to))
	}
	b.attack(me)
	if me.Order.Kind != game.GoToStar || me.Order.Star == m || b.dest[me] != to {
		t.Errorf("order %+v dest %d: want a GoToStar away from %d toward %d", me.Order, b.dest[me], m, to)
	}
}
