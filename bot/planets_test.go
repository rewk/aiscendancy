package bot

import (
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
)

// The bots' side of planets in combat: a colony's orbital
// weapons defend it (defend), a warship fires at a hostile planet when no
// hostile ship in space is there (roundFight), the estimates count the
// hostile colonies' defences (planetSide), a warship in orbit leaves it
// to fight.

// freeSlots puts its in the first empty orbit slots of p, the slots
// returned in order.
func freeSlots(t *testing.T, p *game.Planet, its ...game.Item) []int {
	t.Helper()
	var out []int
	for k := range p.Orbit {
		if len(out) == len(its) {
			break
		}
		if p.Orbit[k] == game.NoItem {
			p.Orbit[k] = its[len(out)]
			out = append(out, k)
		}
	}
	if len(out) < len(its) {
		t.Fatalf("%s: %d free orbit slots, want %d", p.Name, len(out), len(its))
	}
	return out
}

// mediumHulk and largeHulk are unarmed engines on bigger hulls: more
// estimated hull points than unarmedScout.
var (
	mediumHulk = game.Design{Name: "Mid", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}
	largeHulk  = game.Design{Name: "Big", Hull: game.Large, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}
)

// lancer: two generators, two guns, an Ion Banger.
var lancer = game.Design{Name: "Lancer", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.ProtonShaver, game.IonBanger, game.MassBarrageGun, game.MassBarrageGun}}

// A colony's orbital weapons arm, each at the hostile ship in space of
// least estimated hull within its own range of the planet (ties the
// nearest): a ship in orbit is skipped, one out of range too; a weapon
// whose shots are spent today arms nothing; the armed shots fire in the
// round.
func TestDefendArmsWeakestInRange(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	col := b.E.Colonies[0]
	p := col.Planet
	sl := freeSlots(t, p, game.ShortRangeOrbitalWhopper, game.LongRangeOrbitalWhopper, game.OrbitalMissileBase)
	short, long, spent := sl[0], sl[1], sl[2]
	ds, _ := game.ShortRangeOrbitalWhopper.Defence()
	dl, _ := game.LongRangeOrbitalWhopper.Defence()
	rs, rl := float64(ds.Range)*game.RangeUnit, float64(dl.Range)*game.RangeUnit
	dm, _ := game.OrbitalMissileBase.Defence()
	col.OrbitShots[spent] = dm.Shots
	e1 := g.Empires[1]
	at := func(x float64) game.Vec3 { return game.Vec3{X: p.Pos.X + x, Y: p.Pos.Y, Z: p.Pos.Z} }
	big := put(e1, largeHulk, home, at(0.05))
	mid := put(e1, mediumHulk, home, at(-rs+0.05))
	// Two Small ships out of the short range, within the long one: the
	// nearer is the long gun's.
	far := put(e1, unarmedScout, home, at(-(rs+rl)/2))
	near := put(e1, unarmedScout, home, at((rs+rl)/2-0.1))
	// A Small ship in orbit of my planet: the nearest, the weakest, never
	// a planet's target.
	orb := e1.Spawn(unarmedScout, home, col.PlanetIdx)
	hp := func(s *game.Starship) float64 {
		sg, ok := g.SightingOf(b.E, s)
		if !ok {
			t.Fatalf("%s unseen", s.Name)
		}
		return sightingSide(g, b.genes(), sg).hp
	}
	if !(hp(near) == hp(far) && hp(far) < hp(mid) && hp(mid) < hp(big)) || hp(orb) != hp(near) {
		t.Fatalf("fixture hp: near %v far %v mid %v big %v orb %v", hp(near), hp(far), hp(mid), hp(big), hp(orb))
	}
	if d := dist(p.Pos, near.Pos); d <= rs || d > rl || dist(p.Pos, far.Pos) <= dist(p.Pos, near.Pos) || dist(p.Pos, far.Pos) > rl {
		t.Fatalf("fixture ranges: near %.3f far %.3f (short %.2f long %.2f)", d, dist(p.Pos, far.Pos), rs, rl)
	}
	b.Round(home)
	got := map[int]*game.Starship{}
	for _, a := range col.Armed {
		if a.AtPlanet {
			t.Errorf("slot %d armed at a planet", a.Part)
		}
		for _, s := range []*game.Starship{big, mid, far, near, orb} {
			if sg, _ := g.SightingOf(b.E, s); sg == a.Target {
				got[a.Part] = s
			}
		}
	}
	if len(col.Armed) != 2 || got[short] != mid || got[long] != near {
		t.Errorf("armed %d shots: short at %v, long at %v; want the short at %s, the long at %s", len(col.Armed), got[short], got[long], mid.Name, near.Name)
	}
	_, _, lg := g.Round(home, []bool{false, true})
	fired := map[int]*game.Starship{}
	for _, f := range lg.Shots {
		if f.FromPlanet && f.ShooterPlanet == col.PlanetIdx {
			fired[f.ShooterSlot] = f.Target
		}
	}
	if len(fired) != 2 || fired[short] != mid || fired[long] != near || col.OrbitShots[short] != 1 || col.OrbitShots[long] != 1 {
		t.Errorf("fired %v, shots %v", fired, col.OrbitShots)
	}
	// Every hostile out of every range: nothing armed.
	for i, s := range []*game.Starship{big, mid, near, far} {
		s.Pos = at(rl + 0.05 + 0.1*float64(i))
	}
	if b.Round(home); len(col.Armed) != 0 {
		t.Errorf("out of range: armed %+v", col.Armed)
	}
}

// With only a hostile planet to hit (its shield; the hostile ships at
// its star in orbit, out of Arm's reach) a warship stands off from the
// planet's Pos as from a ship and arms ArmAtPlanet, and the rounds strike
// the shield; a hostile ship in space is preferred to the planet.
func TestFightPlanet(t *testing.T) {
	g, b, _ := roundFixture(t, noFlight())
	e1 := g.Empires[1]
	hc := e1.Colonies[0]
	star, p := hc.Star, hc.Planet
	sh := freeSlots(t, p, game.OrbitalShield)[0]
	b.E.Visit(star)
	w := game.Gizmos[game.MassBarrageGun]
	rng := float64(w.Range) * game.RangeUnit
	stand := rng * NeutralCombat().Standoff
	me := put(b.E, lancer, star, game.Vec3{X: p.Pos.X + stand + 0.5*lancer.Reach(), Y: p.Pos.Y, Z: p.Pos.Z})
	if hs := g.Hostiles(b.E, star); len(hs) != 0 {
		t.Fatalf("fixture: %d hostile ships seen at empire 1's home", len(hs))
	}
	if dist(me.Pos, p.Pos) <= rng {
		t.Fatalf("fixture: in range already")
	}
	if pls := b.planetTargets(star); len(pls) != 1 || pls[0] != hc.PlanetIdx {
		t.Fatalf("planetTargets %v, want [%d]", pls, hc.PlanetIdx)
	}
	b.Round(star)
	if me.Order.Kind != game.GoToPoint || len(me.Armed) != 0 { // ArmAtPlanet refuses out of range
		t.Fatalf("round 0: order %+v, armed %+v: want a move and no shot", me.Order, me.Armed)
	}
	if want := (game.Vec3{X: p.Pos.X + stand, Y: p.Pos.Y, Z: p.Pos.Z}); dist(me.Order.Pos, want) > 1e-9 {
		t.Errorf("stand %+v, want %+v", me.Order.Pos, want)
	}
	// A hostile ship in orbit there is no target for Arm: the planet
	// stays the target.
	e1.Spawn(unarmedScout, star, hc.PlanetIdx)
	struck := 0
	for range RoundCap {
		b.Round(star)
		acted, _, lg := g.Round(star, []bool{false, true})
		for _, f := range lg.Shots {
			if f.Shooter == me && f.AtPlanet && f.HitSlot == sh {
				struck += f.Damage
			}
		}
		if !acted {
			break
		}
	}
	ds, _ := game.OrbitalShield.Defence()
	if struck == 0 || (p.Orbit[sh] == game.OrbitalShield) != (struck < ds.Strength) ||
		p.Orbit[sh] == game.OrbitalShield && p.OrbitDamage[sh] != struck {
		t.Errorf("struck %d: slot %v, damage %d", struck, p.Orbit[sh], p.OrbitDamage[sh])
	}
	if math.Abs(dist(me.Pos, p.Pos)-stand) > 1e-9 {
		t.Errorf("ended %.4f from the planet, the stand is %.4f", dist(me.Pos, p.Pos), stand)
	}
	// A hostile ship in space beside the planet: it is the target.
	me.Refill()
	foe := put(e1, unarmedScout, star, game.Vec3{X: p.Pos.X - 0.1, Y: p.Pos.Y, Z: p.Pos.Z})
	b.Round(star)
	sg, _ := g.SightingOf(b.E, foe)
	if len(me.Armed) == 0 || me.Armed[0].AtPlanet || me.Armed[0].Target != sg {
		t.Errorf("a ship in space there: armed %+v, want at %s", me.Armed, foe.Name)
	}
}

// assess and Round's sides count a hostile colony's orbital defences:
// Σ Damage × Shots of its weapons, Σ Strength of every defence, at full
// strength whatever its hidden OrbitDamage; my own colony's are not
// theirs.
func TestAssessPlanetDefences(t *testing.T) {
	g, b, home := roundFixture(t, nil)
	hc := g.Empires[1].Colonies[0]
	b.E.Visit(hc.Star)
	b.assess()
	before := b.their(hc.Star)
	sl := freeSlots(t, hc.Planet, game.LongRangeOrbitalWhopper, game.OrbitalMegaShield)
	hc.Planet.OrbitDamage[sl[0]], hc.Planet.OrbitDamage[sl[1]] = 7, 30 // hidden
	freeSlots(t, b.E.Colonies[0].Planet, game.LongRangeOrbitalWhopper)
	b.assess()
	w, _ := game.LongRangeOrbitalWhopper.Defence()
	m, _ := game.OrbitalMegaShield.Defence()
	want := before.plus(side{float64(w.Damage * w.Shots), float64(w.Strength + m.Strength), 1})
	if got := b.their(hc.Star); got != want {
		t.Errorf("their(%d) = %+v, want %+v", hc.Star, got, want)
	}
	if got := b.planetSide(home); got != (side{}) {
		t.Errorf("my own colony's defences counted: %+v", got)
	}
	// The danger reads it: an unarmed ship is never sent there; and
	// Round's: one there flees though no hostile ship is in space.
	cs := b.E.Spawn(game.FixtureColonyShip, home, 0)
	if !b.danger(cs)(hc.Star) {
		t.Error("an armed hostile colony is no danger to a colony ship")
	}
	for _, s := range g.Empires[1].Ships {
		if s.Star == hc.Star && s.Orbit < 0 {
			s.Star = home // out of the way: the planet alone
		}
	}
	there := put(b.E, game.FixtureColonyShip, hc.Star, game.Vec3{X: hc.Planet.Pos.X + 0.3, Y: hc.Planet.Pos.Y, Z: hc.Planet.Pos.Z})
	if b.Round(hc.Star); there.Order.Kind != game.GoToStar {
		t.Errorf("a colony ship under a hostile planet's guns: order %+v, want a flight by lane", there.Order)
	}
}

// A warship in orbit at a combat star leaves orbit to fight (in orbit it
// cannot fire: it arms nothing) and fires from the next round on.
func TestRoundLeaveOrbitToFight(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	col := b.E.Colonies[0]
	me := b.E.Spawn(lancer, home, col.PlanetIdx)
	foe := put(g.Empires[1], unarmedScout, home, game.Vec3{X: col.Planet.Pos.X + 0.2, Y: col.Planet.Pos.Y, Z: col.Planet.Pos.Z})
	b.Round(home)
	if me.Order.Kind != game.GoToPoint || len(me.Armed) != 0 {
		t.Fatalf("round 0 in orbit: order %+v, armed %+v: want a move, no shot", me.Order, me.Armed)
	}
	g.Round(home, []bool{false, true})
	if me.Orbit >= 0 {
		t.Fatalf("still in orbit %d after round 0", me.Orbit)
	}
	hull := foe.Hull
	b.Round(home)
	_, _, lg := g.Round(home, []bool{false, true})
	n := 0
	for _, f := range lg.Shots {
		if f.Shooter == me && f.Target == foe {
			n++
		}
	}
	if n == 0 || foe.Hull >= hull {
		t.Errorf("round 1: %d shots at the foe, hull %d of %d", n, foe.Hull, hull)
	}
}

// A hostile ship in orbit of my own colony is hit through it. With
// several planets to hit the nearest is the target: a planet guarded by a
// hostile ship in its orbit is one.
func TestFightPlanetNearestOrHold(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	e1 := g.Empires[1]
	col := b.E.Colonies[0]
	scout := e1.Spawn(unarmedScout, home, col.PlanetIdx)
	w := put(b.E, lancer, home, game.Vec3{X: col.Planet.Pos.X + 0.1, Y: col.Planet.Pos.Y, Z: col.Planet.Pos.Z})
	w.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 0.5}}
	if pls := b.planetTargets(home); len(pls) != 1 || pls[0] != col.PlanetIdx {
		t.Fatalf("planetTargets %v, want my colony %d", pls, col.PlanetIdx)
	}
	if b.Round(home); len(w.Armed) == 0 || slices.ContainsFunc(w.Armed, func(a game.Shot) bool { return !a.AtPlanet || a.Planet != col.PlanetIdx }) {
		t.Errorf("a foe in orbit of my colony: armed %+v", w.Armed)
	}
	b.E.Disarm(w)
	e1.Ships = slices.DeleteFunc(e1.Ships, func(s *game.Starship) bool { return s == scout })
	// Two free planets of my home guarded by hostile ships in orbit.
	planets := g.Cluster.Stars[home].System.Planets
	var free []int
	for k, p := range planets {
		if p.Owner < 0 && len(free) < 2 {
			free = append(free, k)
			e1.Spawn(unarmedScout, home, k)
		}
	}
	if len(free) < 2 {
		t.Fatalf("fixture: %d free planets at home", len(free))
	}
	if pls := b.planetTargets(home); len(pls) != 2 {
		t.Fatalf("planetTargets %v, want %v", pls, free)
	}
	// Nearer the second: it is the target, not the first in planet order.
	op := planets[free[1]].Pos
	w.Pos = game.Vec3{X: op.X + min(0.3, 0.9*longestRange(w)), Y: op.Y, Z: op.Z} // in range: ArmAtPlanet arms
	if dist(w.Pos, op) >= dist(w.Pos, planets[free[0]].Pos) {
		t.Fatalf("fixture: planet %d is not the nearer", free[1])
	}
	b.Round(home)
	if len(w.Armed) == 0 || slices.ContainsFunc(w.Armed, func(a game.Shot) bool { return !a.AtPlanet || a.Planet != free[1] }) {
		t.Errorf("armed %+v, want at planet %d", w.Armed, free[1])
	}
}

// A colony's orbital weapon with no hostile ship in space in its range
// arms at the nearest planet it may hit (CanArmPlanetAt), journaling only
// that arm; a planet out of its range is skipped; a hostile ship in
// space within range comes first.
func TestDefendArmsPlanet(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	col := b.E.Colonies[0]
	p := col.Planet
	gun := freeSlots(t, p, game.LongRangeOrbitalWhopper)[0]
	d, _ := game.LongRangeOrbitalWhopper.Defence()
	rl := float64(d.Range) * game.RangeUnit
	at := func(x float64) game.Vec3 { return game.Vec3{X: p.Pos.X + x, Y: p.Pos.Y, Z: p.Pos.Z} }
	// Three planets of the home star taken by empire 1, shielded: two in
	// range (the farther first in planet order), one out of it.
	e1 := g.Empires[1]
	var foes []int
	for pl, q := range g.Cluster.Stars[home].System.Planets {
		if q.Owner < 0 && len(foes) < 3 {
			q.Owner = e1.Index
			e1.Colonies = append(e1.Colonies, &game.Colony{Planet: q, Star: home, PlanetIdx: pl})
			freeSlots(t, q, game.OrbitalShield)
			foes = append(foes, pl)
		}
	}
	if len(foes) < 3 {
		t.Skipf("fixture: %d free planets at the home star", len(foes))
	}
	ps := g.Cluster.Stars[home].System.Planets
	far, near, out := foes[0], foes[1], foes[2]
	ps[far].Pos, ps[near].Pos, ps[out].Pos = at(rl-0.05), at(-rl/2), at(rl+0.1)
	b.Round(home)
	if want := []game.Shot{{Part: gun, AtPlanet: true, Planet: near}}; !slices.Equal(col.Armed, want) {
		t.Errorf("armed %+v, want %+v (the nearer of %v)", col.Armed, want, foes)
	}
	if n := strings.Count(buf.String(), "\n"); n != 1 || !strings.Contains(buf.String(), `"act":"arm"`) {
		t.Errorf("journal: %d lines\n%s", n, buf.String())
	}
	_, _, lg := g.Round(home, []bool{false, true})
	if len(lg.Shots) != 1 || !lg.Shots[0].FromPlanet || !lg.Shots[0].AtPlanet || lg.Shots[0].TargetPlanet != near ||
		ps[near].OrbitDamage[slices.Index(ps[near].Orbit[:], game.OrbitalShield)] != d.Damage {
		t.Errorf("fired %+v", lg.Shots)
	}
	// Only the out-of-range planet left: nothing armed.
	ps[far].Pos, ps[near].Pos = at(rl+0.2), at(-rl-0.2)
	if b.Round(home); len(col.Armed) != 0 {
		t.Errorf("out of range: armed %+v", col.Armed)
	}
	// A hostile ship in space in range: the ship, not the planet.
	ps[near].Pos = at(-rl / 2)
	foe := put(e1, unarmedScout, home, at(rl/2))
	b.Round(home)
	if sg, _ := g.SightingOf(b.E, foe); len(col.Armed) != 1 || col.Armed[0].AtPlanet || col.Armed[0].Target != sg {
		t.Errorf("a ship in range: armed %+v", col.Armed)
	}
}
