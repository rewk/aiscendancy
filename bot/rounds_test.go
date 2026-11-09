package bot

import (
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// roundFixture: two empires on seed 1, empire 0's bot (gn) and none for
// empire 1, whose ships stay put; home is empire 0's home star.
func roundFixture(t *testing.T, gn *Genome) (g *game.Game, b *Bot, home int) {
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	b = NewAll(g, Config{NoResearch: true, Genome: gn})[0]
	return g, b, b.E.Colonies[0].Star
}

// put spawns d for e at star, out of orbit at pos.
func put(e *game.Empire, d game.Design, star int, pos game.Vec3) *game.Starship {
	s := e.Spawn(d, star, 0)
	s.Orbit, s.Pos = -1, pos
	return s
}

// noFlight is Neutral with a FleeRatio no pressure reaches: the fight
// path alone.
func noFlight() *Genome { return genomeWith(func(g *Genome) { g.FleeRatio = 1e9 }) }

// unarmedScout has an engine and nothing else.
var unarmedScout = game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}

// A one-generator warship under pressure above FleeRatio arms nothing:
// the round orders it to the lane flee would pick (no threat known next
// door: fewest lane days, then lane order) and it ends in that lane, the
// harmless foe untouched.
func TestRoundFleeToLane(t *testing.T) {
	// HullThreat 50: the unarmed foe looks deadly.
	g, b, home, adj := fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small] = 50 }))
	c := g.Cluster
	d := game.Design{Name: "Lone", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.StarLaneDrive, game.MassBarrageGun}}
	if d.Generators() != 1 || !armed(d) {
		t.Fatalf("fixture: %d generators, armed %v", d.Generators(), armed(d))
	}
	me := b.E.Spawn(d, home, 0)
	foe := g.Empires[1].Spawn(unarmedScout, home, 0)
	b.assess()
	if p := pressure(b.their(home), b.mine(home)); p <= b.cg.FleeRatio {
		t.Fatalf("fixture: pressure %v", p)
	}
	want, wantDays := -1, 0
	for k, li := range c.Stars[home].Lanes {
		if days := game.LaneDays(c, li, 1, me.Species.Traits()); want < 0 || days < wantDays {
			want, wantDays = adj[k], days
		}
	}
	// A GoToStar already given stands (it leads to a lane as well).
	other := adj[0]
	if other == want {
		other = adj[1]
	}
	me.Order = game.Order{Kind: game.GoToStar, Star: other}
	if b.Round(home); me.Order.Star != other {
		t.Errorf("a standing GoToStar %d replaced: %+v", other, me.Order)
	}
	me.Order = game.Order{}
	b.Round(home)
	if len(me.Armed) != 0 || me.Order.Kind != game.GoToStar || me.Order.Star != want {
		t.Fatalf("armed %+v, order %+v, want GoToStar %d", me.Armed, me.Order, want)
	}
	bots := []*Bot{b, nil}
	for d := 0; d < 60 && me.Lane < 0; d++ {
		Day(g, bots)
	}
	if me.Lane < 0 || c.Other(me.Lane, home) != want {
		t.Errorf("not in the lane to %d: lane %d, star %d", want, me.Lane, me.Star)
	}
	if foe.Hull != foe.MaxHull() {
		t.Errorf("the fleeing ship fired: foe hull %d of %d", foe.Hull, foe.MaxHull())
	}
}

// candidates is the safest-point oracle's: the ship's position, then the
// 26 directions at SystemReach.
func candidates(at game.Vec3) []game.Vec3 {
	out := []game.Vec3{at}
	for dx := -1.0; dx <= 1; dx++ {
		for dy := -1.0; dy <= 1; dy++ {
			for dz := -1.0; dz <= 1; dz++ {
				if n := math.Sqrt(dx*dx + dy*dy + dz*dz); n > 0 {
					out = append(out, game.Vec3{X: dx / n * game.SystemReach, Y: dy / n * game.SystemReach, Z: dz / n * game.SystemReach})
				}
			}
		}
	}
	return out
}

// An unarmed ship without a lane drive under a gun goes for the point
// farthest out of the hostiles' reach, recomputed here from the ships
// (scanned: their move when the energy covers it plus their range;
// unscanned: HullReach); where it stands is best, it stays, its
// GoToPoint cleared.
func TestRoundFleePoint(t *testing.T) {
	scanner := game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.SubspacePhaseArray, game.XRayMegaglasses}}
	for _, tc := range []struct {
		name    string
		d       game.Design
		scanned bool
	}{{"unscanned", unarmedScout, false}, {"scanned", scanner, true}} {
		const hullReach = 1.0 // Pos units
		g, b, home := roundFixture(t, genomeWith(func(g *Genome) { g.HullReach[game.Small] = hullReach }))
		e1 := g.Empires[1]
		me := put(b.E, tc.d, home, game.Vec3{X: 0.2, Z: -0.3})
		r1 := put(e1, game.FixtureRaider, home, game.Vec3{X: 0.5, Z: -0.1})
		r2 := put(e1, game.FixtureRaider, home, game.Vec3{X: -0.2, Y: 0.1, Z: -0.3})
		r2.Power = 0 // no move left: scanned, only its guns reach
		reach := func(h *game.Starship) float64 {
			if !tc.scanned {
				return hullReach
			}
			r := float64(game.Gizmos[game.MassBarrageGun].Range) * game.RangeUnit
			if h.Power >= h.Design.MoveCost() {
				r += h.Design.Reach()
			}
			return r
		}
		if sg, _ := g.SightingOf(b.E, r1); (sg.Scan != nil) != tc.scanned {
			t.Fatalf("%s: scan %v", tc.name, sg.Scan)
		}
		best, bestS := game.Vec3{}, math.Inf(-1)
		for _, p := range candidates(me.Pos) {
			sc := math.Min(dist(p, r1.Pos)-reach(r1), dist(p, r2.Pos)-reach(r2))
			if sc > bestS {
				best, bestS = p, sc
			}
		}
		if best == me.Pos {
			t.Fatalf("%s: the fixture stays put", tc.name)
		}
		b.Round(home)
		if me.Order.Kind != game.GoToPoint || dist(me.Order.Pos, best) > 1e-6 || len(me.Armed) != 0 {
			t.Errorf("%s: order %+v, want GoToPoint %+v", tc.name, me.Order, best)
		}
		// Far out of the system, where it is beats the rim: no order.
		me.Pos = game.Vec3{X: -100}
		me.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{}}
		if b.Round(home); me.Order.Kind != game.NoOrder {
			t.Errorf("%s: staying, order %+v", tc.name, me.Order)
		}
	}
}

// A fighting ship out of range heads for its stand, Standoff × its range
// from a stationary target, on its own side, and fires there: no shot
// lands while it is farther than the range, it ends at the stand.
func TestRoundFightStandoff(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	// Two generators, two guns, an Ion Banger (a long hop).
	d := game.Design{Name: "Lancer", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.IonBanger, game.MassBarrageGun, game.MassBarrageGun}}
	w := game.Gizmos[game.MassBarrageGun]
	rng := float64(w.Range) * game.RangeUnit
	stand, hop := rng*NeutralCombat().Standoff, d.Reach()
	foe := put(g.Empires[1], unarmedScout, home, game.Vec3{X: -1.4})
	// Two hops out of the stand: the first leaves it out of range.
	me := put(b.E, d, home, game.Vec3{X: foe.Pos.X + stand + 2*hop})
	if stand+hop <= rng || stand >= rng {
		t.Fatalf("fixture: stand %.2f, hop %.2f, range %.2f", stand, hop, rng)
	}
	hits := 0
	for r := range RoundCap {
		b.Round(home)
		if r == 0 && (me.Order.Kind != game.GoToPoint || len(me.Armed) != 1) {
			t.Fatalf("round 0: order %+v, armed %d: want a move and one shot", me.Order, len(me.Armed))
		}
		hull := foe.Hull
		acted, _, _ := g.Round(home, []bool{false, true})
		if foe.Hull < hull {
			hits += (hull - foe.Hull) / w.Level
			if dd := dist(me.Pos, foe.Pos); math.Abs(dd-stand) > 1e-9 {
				t.Errorf("round %d: a hit at %.4f, the stand is %.4f", r, dd, stand)
			}
		}
		if !acted {
			break
		}
	}
	if dd := dist(me.Pos, foe.Pos); math.Abs(dd-stand) > 1e-9 || me.Pos.X <= foe.Pos.X {
		t.Errorf("ended at %+v, %.4f from the target: want the stand %.4f on its side", me.Pos, dd, stand)
	}
	if hits < 1 || foe.Hull != foe.MaxHull()-hits*w.Level {
		t.Errorf("%d hits, foe hull %d of %d", hits, foe.Hull, foe.MaxHull())
	}
	// Without the Game the bot arms and orders nothing.
	solo := New(b.E, b.Cfg)
	me.Refill()
	me.Pos, me.Order = game.Vec3{X: 1.4}, game.Order{}
	if solo.Round(home); len(me.Armed) != 0 || me.Order.Kind != game.NoOrder {
		t.Errorf("a bot with no Game: armed %+v, order %+v", me.Armed, me.Order)
	}
}

// A ship that moves this round keeps a generator for the move: with one
// it arms nothing, with two it arms one; holding its stand it arms every
// generator's worth.
func TestRoundFightGenerators(t *testing.T) {
	one := game.Design{Name: "One", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun}}
	for _, tc := range []struct {
		d          game.Design
		move, hold int // armed when moving, when at its stand
		energy     int // -1: the day's
	}{{one, 0, 1, -1}, {warship, 1, 2, -1}, {warship, 0, 1, 1}} { // energy 1: the move's, no shot beside it
		g, b, home := roundFixture(t, noFlight())
		put(g.Empires[1], unarmedScout, home, game.Vec3{X: -1.4})
		me := put(b.E, tc.d, home, game.Vec3{X: 1.4})
		if tc.energy >= 0 {
			me.Power = tc.energy
		}
		if b.Round(home); me.Order.Kind != game.GoToPoint || len(me.Armed) != tc.move {
			t.Errorf("%s moving: order %+v, armed %d, want %d", tc.d.Name, me.Order, len(me.Armed), tc.move)
		}
		b.E.Disarm(me)
		me.Pos = me.Order.Pos
		if b.Round(home); me.Order.Kind != game.NoOrder || len(me.Armed) != tc.hold {
			t.Errorf("%s holding: order %+v, armed %d, want %d", tc.d.Name, me.Order, len(me.Armed), tc.hold)
		}
	}
}

// Within range of its target but off its stand, the shots come first:
// with energy for one action it arms and stays (the old order chased the
// stand every day and never fired); with the day's energy it arms a gun
// per generator, none left for the move, and stays too.
func TestRoundFightShotsFirst(t *testing.T) {
	for _, tc := range []struct {
		energy, armed int
	}{{1, 1}, {-1, warship.Generators()}} {
		g, b, home := roundFixture(t, noFlight())
		put(g.Empires[1], unarmedScout, home, game.Vec3{X: -0.05})
		me := put(b.E, warship, home, game.Vec3{X: 0.05})
		me.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 1}}
		if tc.energy >= 0 {
			me.Power = tc.energy
		}
		if dist(me.Pos, game.Vec3{X: -0.05}) >= longestRange(me)*b.genes().Standoff {
			t.Fatal("fixture: the target must be in range and the ship inside its stand")
		}
		b.Round(home)
		if len(me.Armed) != tc.armed || me.Order.Kind != game.NoOrder {
			t.Errorf("energy %d: order %+v, armed %d; want armed %d, no order", tc.energy, me.Order, len(me.Armed), tc.armed)
		}
	}
}

// Outgunned with nowhere to go (no lane drive, every point within the
// hostile's reach) an armed ship fights instead of running in vain; with
// a reach it can outrun, it flees to the safest point.
func TestRoundNoEscapeFights(t *testing.T) {
	for _, tc := range []struct {
		reach float64 // HullReach, Pos units
		flee  bool
	}{{5, false}, {0.1, true}} {
		gn := genomeWith(func(g *Genome) { g.HullThreat[game.Small], g.HullReach[game.Small] = 50, tc.reach })
		g, b, home := roundFixture(t, gn)
		put(g.Empires[1], unarmedScout, home, game.Vec3{X: -0.5})
		me := put(b.E, warship, home, game.Vec3{X: 0.5})
		b.Round(home)
		if fled := me.Order.Kind == game.GoToPoint && len(me.Armed) == 0; fled != tc.flee || !tc.flee && len(me.Armed) == 0 {
			t.Errorf("HullReach %v: order %+v, armed %d, want fled %v", tc.reach, me.Order, len(me.Armed), tc.flee)
		}
	}
}

// An armed ship whose shots are spent today, under a pressure at most
// FleeRatio, holds: it arms nothing, a GoToStar stands (a lane departure
// decided), any other order is cleared (it does not flee, attack would
// only send it back); above FleeRatio it flees as before. An unarmed
// ship among hostiles dealing no damage does nothing.
func TestRoundSpentOrHarmless(t *testing.T) {
	g, b, home, adj := fleeFixture(t, noFlight())
	gun := slices.Index(laner.Parts, game.MassBarrageGun)
	me := b.E.Spawn(laner, home, 0)
	put(g.Empires[1], unarmedScout, home, game.Vec3{X: 1})
	me.Shots[gun] = game.Gizmos[game.MassBarrageGun].Uses
	for _, tc := range []struct{ order, want game.Order }{
		{game.Order{}, game.Order{}},
		{game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: -1}}, game.Order{}},
		{game.Order{Kind: game.GoToPlanet, Planet: 1}, game.Order{}},
		{game.Order{Kind: game.GoToStar, Star: adj[0]}, game.Order{Kind: game.GoToStar, Star: adj[0]}},
	} {
		me.Order = tc.order
		if b.Round(home); me.Order != tc.want || len(me.Armed) != 0 {
			t.Errorf("spent, order %+v: now %+v, armed %d, want %+v", tc.order, me.Order, len(me.Armed), tc.want)
		}
	}
	// Above FleeRatio (HullThreat 50: the scout looks deadly), it flees.
	b.Cfg.Genome = genomeWith(func(g *Genome) { g.HullThreat[game.Small] = 50 })
	b.assess()
	me.Order = game.Order{}
	if b.Round(home); me.Order.Kind != game.GoToStar || me.Order.Star != b.refuge(me) || len(me.Armed) != 0 {
		t.Errorf("spent under pressure: order %+v, armed %d, want GoToStar %d", me.Order, len(me.Armed), b.refuge(me))
	}
	g, b, home, _ = fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small] = 0 }))
	cs := b.E.Spawn(game.FixtureColonyShip, home, 0)
	put(g.Empires[1], unarmedScout, home, game.Vec3{X: 1})
	cs.Order = game.Order{Kind: game.GoToPlanet, Planet: 1}
	if b.Round(home); cs.Order != (game.Order{Kind: game.GoToPlanet, Planet: 1}) {
		t.Errorf("harmless hostiles: order %+v", cs.Order)
	}
}

// The target is the hostile of least estimated hull points (unscanned:
// the hull's full strength; scanned: its points left), ties the nearest.
func TestRoundTarget(t *testing.T) {
	eye := game.Design{Name: "Eye", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.SubspacePhaseArray, game.XRayMegaglasses}}
	mediumScout := game.Design{Name: "Big", Hull: game.Medium, Parts: unarmedScout.Parts}
	target := func(b *Bot, home int, me *game.Starship) game.Vec3 {
		if b.Round(home); len(me.Armed) == 0 {
			t.Fatalf("nothing armed, order %+v", me.Order)
		}
		return me.Armed[0].Target.Pos
	}
	// Unscanned: the Small far beats the Medium near.
	g, b, home := roundFixture(t, noFlight())
	me := put(b.E, warship, home, game.Vec3{})
	put(g.Empires[1], mediumScout, home, game.Vec3{X: 0.3})
	small := put(g.Empires[1], unarmedScout, home, game.Vec3{X: -1})
	if got := target(b, home, me); got != small.Pos {
		t.Errorf("unscanned: target at %+v, want the Small at %+v", got, small.Pos)
	}
	// Two Smalls: the nearer, listed second.
	g, b, home = roundFixture(t, noFlight())
	me = put(b.E, warship, home, game.Vec3{})
	put(g.Empires[1], unarmedScout, home, game.Vec3{X: 1})
	near := put(g.Empires[1], unarmedScout, home, game.Vec3{X: -0.4})
	if got := target(b, home, me); got != near.Pos {
		t.Errorf("tie: target at %+v, want the nearer at %+v", got, near.Pos)
	}
	// Scanned: a worn Medium below a Small's full hull wins.
	g, b, home = roundFixture(t, noFlight())
	me = put(b.E, eye, home, game.Vec3{})
	worn := put(g.Empires[1], mediumScout, home, game.Vec3{X: -0.45})
	fresh := put(g.Empires[1], unarmedScout, home, game.Vec3{X: 0.3})
	worn.Hull = fresh.Hull - 1
	if sg, _ := g.SightingOf(b.E, worn); sg.Scan == nil {
		t.Fatal("fixture: not scanned")
	}
	if got := target(b, home, me); got != worn.Pos {
		t.Errorf("scanned: target at %+v, want the worn Medium at %+v", got, worn.Pos)
	}
}

// Act clears a GoToPoint left by a fight once the star holds no combat;
// at a combat star it stands.
func TestActClearsStaleGoToPoint(t *testing.T) {
	g, b, home := roundFixture(t, nil)
	me := put(b.E, unarmedScout, home, game.Vec3{X: 0.2})
	pt := game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 1}}
	me.Order = pt
	if b.Act(); me.Order.Kind != game.NoOrder {
		t.Errorf("no combat: order %+v", me.Order)
	}
	put(g.Empires[1], game.FixtureRaider, home, game.Vec3{})
	me.Order = pt
	if b.Act(); me.Order != pt {
		t.Errorf("combat star: order %+v", me.Order)
	}
}

// The round genes: Neutral's from the tables, Clamp's bounds.
func TestRoundGenes(t *testing.T) {
	n := Neutral()
	reach := float64(game.Gizmos[game.TonklinMotor].Level)*game.SugarCube + float64(game.Gizmos[game.MassBarrageGun].Range)*game.RangeUnit
	if math.Abs(reach-0.45) > 1e-12 {
		t.Errorf("Tonklin move + Mass Barrage Gun range %v, want 0.45", reach)
	}
	for h, x := range n.HullReach {
		if math.Abs(x-reach) > 1e-12 {
			t.Errorf("HullReach[%d] %v, want %v", h, x, reach)
		}
	}
	if n.Standoff != 0.9 || n.Rounds != RoundCap || n.Combat() != NeutralCombat() {
		t.Errorf("Neutral: Standoff %v, Rounds %v, Combat %+v", n.Standoff, n.Rounds, n.Combat())
	}
	if b := New(game.NewEmpire(1, game.Orfa, game.Average), Config{}); b.rounds() != RoundCap {
		t.Errorf("nil genome: %d rounds", b.rounds())
	}
	for _, tc := range []struct{ in, reach, standoff, rounds float64 }{
		{-1, 0, 0.1, 1},
		{1000, 5, 1, RoundCap},
		{math.NaN(), 0, 0.1, 1},
		{0.5, 0.5, 0.5, 1},
	} {
		g := Neutral()
		g.HullReach[game.Large], g.Standoff, g.Rounds = tc.in, tc.in, tc.in
		g.Clamp()
		if g.HullReach[game.Large] != tc.reach || g.Standoff != tc.standoff || g.Rounds != tc.rounds {
			t.Errorf("Clamp(%v): HullReach %v, Standoff %v, Rounds %v", tc.in, g.HullReach[game.Large], g.Standoff, g.Rounds)
		}
	}
}

// The arena's trace (fuzz -mode duel -arena lancer,lancer:raider,raider,
// seed 1, Orfa vs Mebes, Default bots): every day, no two ships in space
// at one star end closer than ShipGap (two Lancers leaving one orbit
// together, two Raiders FreeSpot'ed side by side, used to stack).
func TestArenaShipGap(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa, game.Mebes}, game.Average)
	a, b, err := game.ParseArena("lancer,lancer:raider,raider")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	bots := NewAll(g, Default())
	var ships []*game.Starship
	for _, e := range g.Empires {
		ships = append(ships, e.Ships...)
	}
	for day := 1; day <= 25; day++ {
		Day(g, bots)
		for i, x := range ships {
			for _, y := range ships[i+1:] {
				if x.Hull <= 0 || y.Hull <= 0 || !x.InSpace() || !y.InSpace() || x.Star != y.Star {
					continue
				}
				if d := math.Sqrt(sq(x.Pos.X-y.Pos.X) + sq(x.Pos.Y-y.Pos.Y) + sq(x.Pos.Z-y.Pos.Z)); d < game.ShipGap-1e-9 {
					t.Errorf("day %d: %s and %s %.4f apart", day, x.Name, y.Name, d)
				}
			}
		}
	}
}

func sq(x float64) float64 { return x * x }

// A hostile scanned without X Ray Megaglasses shows no parts (Scan.Parts
// false): threatReach keeps its hull's HullReach, as unscanned; with the
// glasses aboard my scanner, its move plus its gun's range.
func TestThreatReachNoParts(t *testing.T) {
	const hullReach = 1.0
	g, b, home := roundFixture(t, genomeWith(func(g *Genome) { g.HullReach[game.Small] = hullReach }))
	eye := put(b.E, game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.SubspacePhaseArray}}, home, game.Vec3{X: 0.2})
	r := put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.3})
	sg, _ := g.SightingOf(b.E, r)
	if sg.Scan == nil || sg.Scan.Parts {
		t.Fatalf("fixture: scan %+v", sg.Scan)
	}
	if got := threatReach(sg, b.genes()); got != hullReach {
		t.Errorf("no parts: reach %v, want HullReach %v", got, hullReach)
	}
	eye.Design.Parts = append(eye.Design.Parts, game.XRayMegaglasses)
	sg, _ = g.SightingOf(b.E, r)
	want := r.Design.Reach() + float64(game.Gizmos[game.MassBarrageGun].Range)*game.RangeUnit
	if !sg.Scan.Parts || want == hullReach {
		t.Fatalf("fixture: parts %v, reach %v", sg.Scan.Parts, want)
	}
	if got := threatReach(sg, b.genes()); math.Abs(got-want) > 1e-12 {
		t.Errorf("parts: reach %v, want %v", got, want)
	}
}

// TestFightKeepsRange: raiders that never flee, NoDiplomacy, the default
// arena: shots are fired within 30 days, each within its range (an armed
// ship in range backing off to its stand, the target likewise, the moves
// resolving first, dropped every shot out of range).
func TestFightKeepsRange(t *testing.T) {
	a, b, err := game.ParseArena("raider,raider:raider,raider")
	if err != nil {
		t.Fatal(err)
	}
	g := game.NewGame(1, []game.Species{game.Orfa, game.Mebes}, game.Average)
	if err := game.SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	gen := Neutral()
	gen.FleeRatio, gen.AttackRatio = 10, 10
	bots := make([]*Bot, 2)
	for k := range bots {
		c := Default()
		c.Genome, c.NoDiplomacy = gen, true
		bots[k] = New(g.Empires[k], c)
		bots[k].G = g
	}
	shots := 0
	for range 30 {
		_, _, logs := Day(g, bots)
		for _, l := range logs {
			for _, f := range l.Shots {
				shots++
				if d, r := dist(f.From, f.To), f.Shooter.WeaponRange(&game.Gizmos[f.Gizmo]); d > r+1e-9 {
					t.Errorf("shot at %.3f, range %.3f", d, r)
				}
			}
		}
	}
	if shots == 0 {
		t.Error("no shot in 30 days")
	}
	if g.Empires[0].StanceWith(1) != game.War {
		t.Errorf("stance %v", g.Empires[0].StanceWith(1))
	}
}
