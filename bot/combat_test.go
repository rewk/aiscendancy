package bot

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// allKnown is a Research knowing every tech.
func allKnown() *game.Research {
	r := game.NewResearch()
	for t := range game.NumTechs {
		r.Known[t] = true
	}
	return r
}

// genomeWith is Neutral with its combat genes edited by f.
func genomeWith(f func(g *Genome)) *Genome {
	g := Neutral()
	f(g)
	return g
}

// neighbours is star's adjacent stars in lane order.
func neighbours(c *game.Cluster, star int) []int {
	var out []int
	for _, li := range c.Stars[star].Lanes {
		out = append(out, c.Other(li, star))
	}
	return out
}

// An unscanned hostile counts its hull's HullThreat and HullStrength ×
// its owner's HullFactor; scanned, its design's damage and its hull
// points left. Contact follows the hostile, then a foreign planet.
func TestEstimate(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa, game.Snovemdomas}, game.Average)
	atWar(t, g, 0, 1)
	gn := genomeWith(func(g *Genome) { g.HullThreat[game.Small] = 7.5 })
	bots := NewAll(g, Config{NoResearch: true, Genome: gn})
	b, e0, e1 := bots[0], g.Empires[0], g.Empires[1]
	home := e0.Colonies[0].Star
	if b.assess(); b.contact {
		t.Fatal("contact before any hostile is seen")
	}
	raider := e1.Spawn(game.FixtureRaider, home, 0)
	raider.Hull -= 3
	b.assess()
	full := game.HullStrength[game.Small] * game.Snovemdomas.Traits().HullFactor()
	if got := b.their(home); got.dmg != 7.5 || got.hp != float64(full) || got.n != 1 || !b.contact {
		t.Errorf("unscanned: %+v contact %v, want dmg 7.5 hp %d", got, b.contact, full)
	}
	dd := game.DesignDamage(game.FixtureRaider, 0)
	if float64(dd) == 7.5 || raider.Hull == full {
		t.Fatal("scanned and unscanned estimates coincide: the fixture proves nothing")
	}
	eye := e0.Spawn(game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.TonklinFrequencyAnalyzer, game.XRayMegaglasses}}, home, 0)
	raider.Pos = eye.Pos
	b.assess()
	if got := b.their(home); got.dmg != float64(dd) || got.hp != float64(raider.Hull) {
		t.Errorf("scanned: %+v, want dmg %d hp %d", got, dd, raider.Hull)
	}
	if got := b.mine(home); got.dmg != 0 || got.hp != float64(eye.Hull) || got.n != 1 {
		t.Errorf("mine: %+v", got)
	}
	// No ship: contact through a known foreign planet only.
	e1.Ships = nil
	if b.assess(); b.contact {
		t.Error("contact with nothing foreign known")
	}
	e0.Visit(e1.Colonies[0].Star)
	if b.assess(); !b.contact {
		t.Error("no contact with a foreign colony known")
	}
	// Without the Game nothing is seen.
	b.G = nil
	if b.assess(); b.contact || b.their(home) != (side{}) {
		t.Error("a bot without G sees")
	}
}

func TestPressure(t *testing.T) {
	inf := math.Inf(1)
	for _, tc := range []struct {
		their, mine side
		want        float64
	}{
		{side{0, 10, 1}, side{0, 10, 1}, 0},
		{side{0, 10, 1}, side{5, 10, 1}, 0},
		{side{2, 10, 1}, side{0, 20, 1}, inf},
		{side{2, 10, 1}, side{}, inf},
		{side{2, 0, 1}, side{0, 20, 1}, inf}, // not 0/0
		{side{2, 10, 1}, side{1, 20, 1}, (2.0 / 20) / (1.0 / 10)},
		{side{4, 30, 2}, side{3, 15, 1}, (4.0 / 15) / (3.0 / 30)},
	} {
		if got := pressure(tc.their, tc.mine); got != tc.want {
			t.Errorf("pressure(%+v, %+v) = %v, want %v", tc.their, tc.mine, got, tc.want)
		}
	}
}

// fleeFixture: two empires on seed 1, empire 1's raiders at empire 0's
// home, every neighbour of it known.
func fleeFixture(t *testing.T, gn *Genome) (g *game.Game, b *Bot, home int, adj []int) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	b = NewAll(g, Config{NoResearch: true, Genome: gn})[0]
	home = b.E.Colonies[0].Star
	adj = neighbours(g.Cluster, home)
	if len(adj) < 2 {
		t.Fatalf("home %d has %d lanes", home, len(adj))
	}
	for _, v := range adj {
		b.E.Visit(v)
	}
	return g, b, home, adj
}

// A colony ship under a hostile gun flees to the adjacent star of least
// threat (ties: my colony, then fewer lane days); in orbit colonising it
// stays; with a gunboat beside it the pressure decides, against
// FleeRatio.
func TestFlee(t *testing.T) {
	g, b, home, adj := fleeFixture(t, nil)
	c := g.Cluster
	e1 := g.Empires[1]
	e1.Spawn(game.FixtureRaider, home, 0)
	cs := b.E.Spawn(game.FixtureColonyShip, home, 0)
	// A raider at the first neighbour: lane order alone would pick it.
	e1.Spawn(game.FixtureRaider, adj[0], 0)
	b.assess()
	b.dest[cs] = adj[0]
	if !b.flee(cs) || cs.Order.Kind != game.GoToStar || cs.Order.Star == adj[0] {
		t.Fatalf("flee: order %+v, want away from %d", cs.Order, adj[0])
	}
	if _, ok := b.dest[cs]; ok {
		t.Error("the fleeing ship kept its dest")
	}
	// Both neighbours safe: fewer lane days, then my colony first.
	e1.Ships = e1.Ships[:1]
	b.assess()
	days := func(v int) int {
		li, _, _ := laneOf(c, home, v)
		return game.LaneDays(c, li, 1, b.E.Species.Traits())
	}
	short, long := adj[0], adj[1]
	if days(short) > days(long) {
		short, long = long, short
	}
	if days(short) == days(long) {
		t.Fatal("neighbour lanes tie in days: the fixture proves nothing")
	}
	cs.Order = game.Order{}
	if b.flee(cs); cs.Order.Star != short {
		t.Errorf("safe neighbours: fled to %d, want the shorter %d", cs.Order.Star, short)
	}
	b.E.Colonies = append(b.E.Colonies, &game.Colony{Star: long, PlanetIdx: 0})
	cs.Order = game.Order{}
	if b.flee(cs); cs.Order.Star != long {
		t.Errorf("my colony at %d: fled to %d", long, cs.Order.Star)
	}
	b.E.Colonies = b.E.Colonies[:1]

	// Colonising in orbit: it founds today.
	cs.Order = game.Order{Kind: game.Colonise}
	if b.flee(cs) || cs.Order.Kind != game.Colonise {
		t.Errorf("a colonising ship fled: %+v", cs.Order)
	}
	// No lane drive: it stays.
	cs.Order = game.Order{}
	gb := b.E.Spawn(game.FixtureGunboat, home, 0)
	if b.flee(gb) {
		t.Error("a ship without a lane drive fled")
	}
	b.E.Ships = b.E.Ships[:1] // the colony ship alone

	// Pressure with a gunboat beside: add raiders until it exceeds
	// FleeRatio 1 (computed apart from the bot's estimate).
	mine := func() (dmg, hp float64) {
		for _, s := range b.E.Ships {
			dmg += float64(game.DesignDamage(s.Design, 0))
			hp += float64(s.Hull)
		}
		return
	}
	b.E.Ships = append(b.E.Ships, gb)
	md, mh := mine()
	k, p := 1, 0.0
	for ; k < 20; k++ {
		td, th := float64(k)*NeutralCombat().HullThreat[game.Small], float64(k*game.HullStrength[game.Small]*game.Orfa.Traits().HullFactor())
		if p = (td / mh) / (md / th); p > NeutralCombat().FleeRatio {
			break
		}
	}
	e1.Ships = nil
	for range k {
		e1.Spawn(game.FixtureRaider, home, 0)
	}
	for _, tc := range []struct {
		gn   *Genome
		flee bool
	}{{nil, true}, {genomeWith(func(g *Genome) { g.FleeRatio = p * 1.01 }), false}, {genomeWith(func(g *Genome) { g.FleeRatio = p * 0.99 }), true}} {
		b.Cfg.Genome = tc.gn
		b.assess()
		cs.Order = game.Order{}
		if got := b.flee(cs); got != tc.flee {
			t.Errorf("pressure %.3f, genome %v: fled %v, want %v", p, tc.gn != nil, got, tc.flee)
		}
	}
}

// laneGunboat is a warship with a lane drive (game.Warship, all techs).
func laneGunboat(t *testing.T) game.Design {
	t.Helper()
	d, ok := game.Warship(allKnown(), true)
	if !ok || d.LaneDrives() == 0 || !armed(d) {
		t.Fatalf("Warship: %v ok %v", d, ok)
	}
	return d
}

// An idle warship heads for a hostile star whose pressure with it added
// is below AttackRatio, not for one above; with no target it returns to
// my nearest colony star, and stays at one; with hostiles at its own
// star and the pressure at most FleeRatio it stays.
func TestAttack(t *testing.T) {
	g, b, home, adj := fleeFixture(t, nil)
	e1 := g.Empires[1]
	d := laneGunboat(t)
	gb := b.E.Spawn(d, home, 0)
	foe := adj[1]
	e1.Spawn(game.FixtureRaider, foe, 0)
	// Pressure with the gunboat alone at foe, apart from the estimate.
	td, th := NeutralCombat().HullThreat[game.Small], float64(game.HullStrength[game.Small]*game.Orfa.Traits().HullFactor())
	p := (td / float64(gb.Hull)) / (float64(game.DesignDamage(d, 0)) / th)
	for _, tc := range []struct {
		ratio float64
		go_   bool
	}{{p * 1.01, true}, {p * 0.99, false}} {
		b.Cfg.Genome = genomeWith(func(g *Genome) { g.AttackRatio = tc.ratio })
		b.assess()
		gb.Order = game.Order{}
		delete(b.dest, gb)
		b.attack(gb)
		if got := gb.Order.Kind == game.GoToStar && gb.Order.Star == foe && b.dest[gb] == foe; got != tc.go_ {
			t.Errorf("pressure %.3f, AttackRatio %.3f: order %+v dest %d, attack %v", p, tc.ratio, gb.Order, b.dest[gb], tc.go_)
		}
		if !tc.go_ && gb.Order.Kind != game.NoOrder {
			t.Errorf("at my home with no target: order %+v", gb.Order)
		}
	}
	// Away from home with no target: back home.
	b.Cfg.Genome = nil
	e1.Ships = nil
	gb.Star, gb.Order = adj[0], game.Order{}
	b.assess()
	if b.attack(gb); gb.Order.Kind != game.GoToStar || gb.Order.Star != home {
		t.Errorf("no target: order %+v, want home %d", gb.Order, home)
	}
	// Hostiles here, pressure at most FleeRatio: it stays, even with a
	// weak target next door.
	gb.Star, gb.Order = home, game.Order{}
	e1.Spawn(game.FixtureColonyShip, home, 0)
	e1.Spawn(game.FixtureColonyShip, foe, 0)
	b.assess()
	if b.attack(gb); gb.Order.Kind != game.NoOrder {
		t.Errorf("hostiles here at pressure %v: order %+v", pressure(b.their(home), b.mine(home)), gb.Order)
	}
	e1.Ships = e1.Ships[1:]
	if b.Act(); gb.Order.Star != foe { // through Act: a warship attacks
		t.Errorf("unarmed hostile next door: order %+v, want %d", gb.Order, foe)
	}
}

// wOracle is route's costs by plain relaxation: lane days plus avoid ×
// thr[v] into v, through from and the known stars.
func wOracle(e *game.Empire, c *game.Cluster, from int, avoid float64, thr []float64) []float64 {
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
				if math.IsInf(d[u], 1) || u != from && !e.Knows(u) {
					continue
				}
				if x := d[u] + float64(game.LaneDays(c, li, 1, e.Species.Traits())) + avoid*thr[v]; x < d[v] {
					d[v], changed = x, true
				}
			}
		}
	}
	return d
}

// A colony ship's route detours round a threatened star when another
// path costs less than its lane days plus ThreatAvoid × the threat, and
// goes through it at ThreatAvoid 0; a threatened target star is no
// target. The configuration is searched on seed 1, every star known.
func TestThreatAvoid(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	b := NewAll(g, Config{NoResearch: true})[0]
	e, c := b.E, g.Cluster
	for st := range c.Stars {
		e.Visit(st)
	}
	n := len(c.Stars)
	thrAt := func(m int, w float64) []float64 {
		thr := make([]float64, n)
		thr[m] = w
		return thr
	}
	w := NeutralCombat().HullThreat[game.Small]
	tr := e.Species.Traits()
	av := NeutralCombat().ThreatAvoid
	foundableAt := func(z int) bool {
		for pl := range c.Stars[z].System.Planets {
			if b.foundable(z, pl) {
				return true
			}
		}
		return false
	}
	// Find (from, to, m): to a target star not adjacent to from, m the
	// unique first hop of the shortest path, and with the threat w at m
	// a cheaper path avoiding m.
	from, to, m := -1, -1, -1
search:
	for a := range n {
		for z := range n {
			if z == a || !foundableAt(z) {
				continue
			}
			if _, _, adj := laneOf(c, a, z); adj {
				continue
			}
			d0 := wOracle(e, c, a, 0, make([]float64, n))
			if math.IsInf(d0[z], 1) {
				continue
			}
			var firsts []int
			for _, li := range c.Stars[a].Lanes {
				v := c.Other(li, a)
				if float64(game.LaneDays(c, li, 1, tr))+wOracle(e, c, v, 0, make([]float64, n))[z] == d0[z] {
					firsts = append(firsts, v)
				}
			}
			if len(firsts) != 1 {
				continue
			}
			mm := firsts[0]
			dw := wOracle(e, c, a, av, thrAt(mm, w))
			li, _, _ := laneOf(c, a, mm)
			viaM := float64(game.LaneDays(c, li, 1, tr)) + av*w + wOracle(e, c, mm, av, thrAt(mm, w))[z]
			if dw[z] < viaM {
				from, to, m = a, z, mm
				break search
			}
		}
	}
	if from < 0 {
		t.Fatal("no detour configuration on seed 1")
	}
	claimed := map[place]bool{}
	for st := range c.Stars {
		if st != to {
			claimAll(c, st, claimed)
		}
	}
	e1 := g.Empires[1]
	e1.Ships = nil
	e1.Spawn(game.FixtureRaider, m, 0)
	for _, tc := range []struct {
		gn     *Genome
		detour bool
	}{{nil, true}, {genomeWith(func(g *Genome) { g.ThreatAvoid = 0 }), false}} {
		b.Cfg.Genome = tc.gn
		b.assess()
		got, first := b.route(from, 1, tr, claimed, nil)
		if got != to {
			t.Fatalf("route from %d: to %d, want %d", from, got, to)
		}
		if tc.detour {
			li, _, ok := laneOf(c, from, first)
			dw := wOracle(e, c, from, av, thrAt(m, w))
			if !ok || first == m || float64(game.LaneDays(c, li, 1, tr))+wOracle(e, c, first, av, thrAt(m, w))[to] != dw[to] {
				t.Errorf("threat at %d: first hop %d, not a cheapest detour", m, first)
			}
		} else if first != m {
			t.Errorf("ThreatAvoid 0: first hop %d, want %d", first, m)
		}
	}
	// The threat at the target itself: no target left.
	e1.Ships = nil
	e1.Spawn(game.FixtureRaider, to, 0)
	b.Cfg.Genome = nil
	b.assess()
	if got, _ := b.route(from, 1, tr, claimed, nil); got == to {
		t.Errorf("a threatened target star %d is still a target", to)
	}
}

// hasWarship reports an armed ship or Ship project in e.
func hasWarship(e *game.Empire) bool {
	for _, s := range e.Ships {
		if armed(s.Design) {
			return true
		}
	}
	for _, col := range e.Colonies {
		if pj := col.Project; pj != nil && pj.Design != nil && armed(*pj.Design) {
			return true
		}
	}
	return false
}

// Alone, no warship is ever built.
func TestNoContactNoWarship(t *testing.T) {
	soloGenome(Default(), 1500, func(g *game.Game, b *Bot) {
		if hasWarship(b.E) {
			t.Fatalf("day %d: a warship without contact", b.E.Day)
		}
	})
}

// With contact a shipyard builds a warship while they number below
// ⌈WarshipShare × colonies⌉, or when the threat round the colony exceeds
// DefendW × its defence; without contact, never.
func TestWarshipBuilt(t *testing.T) {
	// The defence: a Gunboat at home against a Raider there (unscanned:
	// HullThreat), nothing seen next door.
	// At war the war wants 2 armed ships (warTarget): the defence cases
	// fly two Gunboats, the share cases are at peace.
	ratio := NeutralCombat().HullThreat[game.Small] / float64(2*game.DesignDamage(game.FixtureGunboat, 0))
	for _, tc := range []struct {
		name          string
		contact, foe  bool
		gunboat       bool
		share, defend float64
		want          bool
	}{
		{"no contact", false, false, false, 2, 0, false},
		{"share", true, false, false, 0.1, 1, true},
		{"share met", true, false, true, 0.1, 1, false},
		{"share 0", true, false, false, 0, 1, false},
		{"defend", true, true, true, 0, ratio * 0.99, true},
		{"defend outweighed", true, true, true, 0, ratio * 1.01, false},
	} {
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
		if tc.foe {
			atWar(t, g, 0, 1)
		}
		gn := genomeWith(func(g *Genome) { g.WarshipShare, g.DefendW = tc.share, tc.defend })
		b := NewAll(g, Config{NoResearch: true, Genome: gn})[0]
		e := b.E
		e.Research = allKnown()
		home := e.Colonies[0]
		home.Planet.Orbit[0] = game.Shipyard
		home.Planet.Orbit[1] = game.OrbitalDocks // standing: the Docks rule passes
		home.Pop = home.Busy() + 1               // free people for the ship
		if tc.contact {
			e.Visit(g.Empires[1].Colonies[0].Star)
		}
		if tc.foe {
			g.Empires[1].Spawn(game.FixtureRaider, home.Star, 0)
		}
		if tc.gunboat {
			e.Spawn(game.FixtureGunboat, home.Star, 0)
			if tc.foe {
				e.Spawn(game.FixtureGunboat, home.Star, 0)
			}
		}
		b.assess()
		b.planColony(1, home, b.claimed()) // i 1: past the home's script
		if got := home.Project != nil && home.Project.Design != nil && armed(*home.Project.Design); got != tc.want {
			t.Errorf("%s: warship %v, want %v (project %+v)", tc.name, got, tc.want, home.Project)
		}
	}
}

// colonyShipsLost plays seed 1, three empires, combat rounds, for days
// with cfg, and counts the colony ships destroyed (the rounds' Destroyed
// Reports naming a ship that carried Colonizers the day before).
func colonyShipsLost(cfg Config, days int) (lost int, digest []int) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	bots := NewAll(g, cfg)
	for range days {
		names := make([][]string, len(g.Empires))
		for i, e := range g.Empires {
			for _, s := range e.Ships {
				if s.Design.Colonizers() > 0 {
					names[i] = append(names[i], s.Name)
				}
			}
		}
		_, reps, _ := Day(g, bots)
		for _, r := range reps {
			if r.Event.Kind != game.Destroyed {
				continue
			}
			for _, nm := range names[r.Empire] {
				if strings.HasPrefix(r.Event.Text, nm+" lost to ") {
					lost++
				}
			}
		}
	}
	for _, e := range g.Empires {
		digest = append(digest, len(e.Colonies), len(e.Ships))
	}
	return lost, append(digest, lost)
}

// Versus: deterministic (1000 days, first contact on seed 1 ~day 800).
// The colony ships lost with Act's flight against FleeRatio +Inf (an
// unarmed ship's pressure is +Inf: only flee and the warships' flight
// turn off) are logged, not compared: a few ships of noise that flips
// sign with the horizon.
func TestVersusFlight(t *testing.T) {
	const days = 1000
	cfg := Default()
	cfg.Genome = Neutral()
	lost, d1 := colonyShipsLost(cfg, days)
	if _, d2 := colonyShipsLost(cfg, days); !equalInts(d1, d2) {
		t.Fatalf("not deterministic: %v vs %v", d1, d2)
	}
	cfg.Genome = genomeWith(func(g *Genome) { g.FleeRatio = math.Inf(1) })
	stay, d3 := colonyShipsLost(cfg, days)
	t.Logf("colony ships lost in %d days: %d fleeing, %d never fleeing; digests %v / %v", days, lost, stay, d1, d3)
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sightingSide's hull points: an unscanned ship's full hull less what my
// shots took off it (Sighting.Dealt), at least 1; a Scan's HullPts win.
func TestSightingSideDealt(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	cg := NeutralCombat()
	full := (&game.Starship{Design: game.Design{Hull: game.Small}, Species: g.Empires[1].Species}).MaxHull()
	for _, c := range []struct {
		dealt int
		scan  *game.Scan
		want  float64
	}{
		{0, nil, float64(full)},
		{1, nil, float64(full - 1)},
		{full - 1, nil, 1},
		{full + 5, nil, 1},
		{1, &game.Scan{HullPts: full}, float64(full)},
	} {
		sg := game.Sighting{Empire: 1, Hull: game.Small, Dealt: c.dealt, Scan: c.scan}
		if got := sightingSide(g, cg, sg); got.hp != c.want || got.dmg != cg.HullThreat[game.Small] {
			t.Errorf("Dealt %d scan %v: %+v, want hp %v", c.dealt, c.scan != nil, got, c.want)
		}
	}
}

// warship steps down from Enormous to the largest hull the colony builds
// within WarshipDays, and keeps Medium at a colony too slow for any.
func TestWarshipHull(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	b.E.Research = allKnown()
	home := b.E.Colonies[0]
	days := func(h game.Hull) int {
		d, ok := game.WarshipWith(b.E.Research, h, true, b.fitted(home))
		n, ok2 := home.DaysFor(d.Cost())
		if !ok || !ok2 {
			t.Fatalf("%v: design %v, days %v", h, ok, ok2)
		}
		return n
	}
	if d, ok := b.warship(home); !ok || d.Hull != game.Medium || days(game.Medium) <= WarshipDays {
		t.Errorf("a new home (Medium in %d days): %v %v", days(game.Medium), d.Hull, ok)
	}
	for _, q := range home.Planet.Surface {
		if q.Item == game.NoItem {
			q.Item, q.Automated = game.IndustrialMegafacility, true
		}
	}
	want := game.Medium
	for h := game.Enormous; h > game.Medium; h-- {
		if days(h) <= WarshipDays {
			want = h
			break
		}
	}
	if want == game.Medium {
		t.Fatalf("Megafacilities everywhere: Large in %d days", days(game.Large))
	}
	if d, ok := b.warship(home); !ok || d.Hull != want {
		t.Errorf("Megafacilities everywhere: %v %v, want %v", d.Hull, ok, want)
	}
}

// fitted draws one or two carried specials the research allows, the same
// on the same day and place, others on other days; warship carries them;
// a refit keeps them; with no special allowed, none.
func TestWarshipSpecials(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa, game.Mebes}, game.Average)
	b := New(g.Empires[0], Config{NoResearch: true})
	b.G = g
	home := b.E.Colonies[0]
	if x := b.fitted(home); len(x) != 0 {
		t.Errorf("nothing researched: %v", x)
	}
	b.E.Research = allKnown()
	draws := map[string]bool{}
	for day := range 20 {
		b.E.Day = day
		x := b.fitted(home)
		if len(x) < 1 || len(x) > 2 || !slices.Equal(x, b.fitted(home)) {
			t.Fatalf("day %d: %v", day, x)
		}
		for _, sp := range x {
			if !carried(sp) {
				t.Fatalf("day %d: %v not carried", day, sp)
			}
		}
		draws[fmt.Sprint(x)] = true
		d, ok := b.warship(home)
		if !ok || !armed(d) {
			t.Fatalf("day %d: warship %v %v", day, d, ok)
		}
		for _, sp := range x {
			if !slices.Contains(d.Parts, sp) {
				t.Errorf("day %d: %v without %v", day, d.Parts, sp)
			}
		}
	}
	if len(draws) < 5 {
		t.Errorf("20 days, %d draws", len(draws))
	}
	d, _ := b.warship(home)
	s := b.E.Spawn(d, home.Star, home.PlanetIdx)
	b.cg = b.genes()
	b.cg.RepairAt = 0.5
	s.Hull = 1 // under RepairAt: a refit is wanted
	to, ok := b.wantRefit(s)
	if !ok {
		t.Fatal("no refit wanted")
	}
	for _, g := range d.Parts {
		if carried(g) && !slices.Contains(to.Parts, g) {
			t.Errorf("refit %v drops %v", to.Parts, g)
		}
	}
}
