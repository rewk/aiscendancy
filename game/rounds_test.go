package game

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Combat rounds (docs/rules.md "Combat rounds"): energy (Power) is the
// day's budget, power (Generators) what runs within a round.

// Round fixtures, all Small: a gun on one generator with a big supply,
// two guns on two generators, a drive-less target.
var (
	oneGen  = Design{Name: "OneGen", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, MassBarrageGun, MassBarrageGun}}
	twoGens = Design{Name: "TwoGens", Hull: Small, Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, MassBarrageGun, MassBarrageGun}}
	hulk    = Design{Name: "Hulk", Hull: Small, Parts: []Gizmo{ProtonShaver}}
)

// roundDuel is duel with the shields off on both sides (full damage, the
// full supply as energy).
func roundDuel(a, b Design) (g *Game, sh, tg *Starship) {
	g, sh, tg = duel(a, b)
	sh.Shields, tg.Shields = false, false
	sh.Refill()
	tg.Refill()
	return g, sh, tg
}

// arm is Arm through the Sighting sh's empire has of tg.
func arm(t *testing.T, g *Game, sh *Starship, p int, tg *Starship) error {
	t.Helper()
	e := g.OwnerOf(sh)
	sg, ok := g.SightingOf(e, tg)
	if !ok {
		t.Fatalf("%s does not see %s", e.Species, tg.Name)
	}
	return e.Arm(sh, p, sg)
}

func TestGenerators(t *testing.T) {
	for _, c := range []struct {
		d    Design
		want int
	}{{FixtureRaider, 2}, {FixtureColonyShip, 1}, {oneGen, 1}, {twoGens, 2}, {Design{Hull: Small, Parts: []Gizmo{TonklinMotor}}, 0}} {
		if got := c.d.Generators(); got != c.want {
			t.Errorf("%s: %d generators, want %d", c.d.Name, got, c.want)
		}
	}
}

// Arm's refusals; an accepted shot spends nothing before Round.
func TestArmRefusals(t *testing.T) {
	g, sh, tg := roundDuel(twoGens, hulk)
	e0, e1 := g.Empires[0], g.Empires[1]
	gun, gun2 := part(twoGens, MassBarrageGun), len(twoGens.Parts)-1
	w := Gizmos[MassBarrageGun]
	sg, _ := g.SightingOf(e0, tg)
	refuse := func(name string, e *Empire, p int, t2 Sighting, want string) {
		t.Helper()
		n, power := len(sh.Armed), sh.Power
		if err := e.Arm(sh, p, t2); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", name, err, want)
		}
		if len(sh.Armed) != n || sh.Power != power {
			t.Errorf("%s: armed %d->%d, energy %d->%d", name, n, len(sh.Armed), power, sh.Power)
		}
	}
	refuse("another empire's ship", e1, gun, sg, "not a ship of this empire")
	refuse("no part", e0, len(twoGens.Parts), sg, "no part")
	refuse("not a weapon", e0, part(twoGens, TonklinMotor), sg, "not a weapon")
	sh.Shots[gun] = w.Uses
	refuse("Uses spent", e0, gun, sg, "no shot left today")
	sh.Shots[gun] = 0
	sh.Lane = 0
	refuse("shooter in a lane", e0, gun, sg, "in a star lane")
	sh.Lane = -1
	tg.Lane = 0
	refuse("target in a lane", e0, gun, sg, "target is in a star lane")
	tg.Lane = -1
	tg.Star = (sh.Star + 1) % len(g.Cluster.Stars)
	refuse("target elsewhere", e0, gun, sg, "same system")
	tg.Star = sh.Star
	mate := e0.Spawn(hulk, sh.Star, g.Empires[0].Colonies[0].PlanetIdx)
	refuse("own ship", e0, gun, Sighting{Empire: e0.Index, ship: mate}, "not hostile")
	refuse("no target", e0, gun, Sighting{Empire: e1.Index}, "no such target")

	power := sh.Power
	if err := e0.Arm(sh, gun, sg); err != nil || sh.Power != power || len(sh.Armed) != 1 {
		t.Fatalf("arm: %v, energy %d of %d, armed %v", err, sh.Power, power, sh.Armed)
	}
	refuse("already armed", e0, gun, sg, "already armed this round")
	sh.Power = w.Power // the first shot's energy only
	refuse("energy short for both", e0, gun2, sg, "not enough energy")
	sh.Power = 2 * w.Power
	if err := e0.Arm(sh, gun2, sg); err != nil {
		t.Fatalf("second gun, two generators: %v", err)
	}
	e0.Disarm(sh)
	if len(sh.Armed) != 0 {
		t.Fatalf("Disarm: %v", sh.Armed)
	}

	// One generator, energy for ten shots: the second gun has no generator.
	g, sh, tg = roundDuel(oneGen, hulk)
	if sh.Power < 2*w.Power {
		t.Fatalf("fixture: energy %d", sh.Power)
	}
	og := part(oneGen, MassBarrageGun)
	if err := arm(t, g, sh, og, tg); err != nil {
		t.Fatal(err)
	}
	if err := arm(t, g, sh, og+1, tg); err == nil || !strings.Contains(err.Error(), "no generator free") {
		t.Errorf("more armed than generators: %v", err)
	}

	// A destroyed target is gone.
	g, sh, tg = roundDuel(twoGens, hulk)
	sg, _ = g.SightingOf(g.Empires[0], tg)
	g.Destroy(tg)
	tg.Hull = 0
	if err := g.Empires[0].Arm(sh, gun, sg); err == nil || !strings.Contains(err.Error(), "no such target") {
		t.Errorf("destroyed target: %v", err)
	}
}

// Two ships that kill each other in one round both die: the shots are
// computed before any damage lands. Swapping the empires changes nothing.
func TestRoundSimultaneous(t *testing.T) {
	a := Design{Name: "A", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun}}
	b := Design{Name: "B", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, QuantumSingularityLauncher}}
	for _, swap := range []bool{false, true} {
		d0, d1 := a, b
		if swap {
			d0, d1 = b, a
		}
		g, s0, s1 := roundDuel(d0, d1)
		s0.Hull = Gizmos[s1.Design.Parts[2]].Level
		s1.Hull = Gizmos[s0.Design.Parts[2]].Level
		if err := arm(t, g, s0, 2, s1); err != nil {
			t.Fatal(err)
		}
		if err := arm(t, g, s1, 2, s0); err != nil {
			t.Fatal(err)
		}
		acted, reps, _ := g.Round(s0.Star, nil)
		if !acted || g.OwnerOf(s0) != nil || g.OwnerOf(s1) != nil || len(reps) != 4 {
			t.Fatalf("swap %v: acted %v, alive %v %v, %d reports", swap, acted, g.OwnerOf(s0) != nil, g.OwnerOf(s1) != nil, len(reps))
		}
		at, as0, as1 := " at "+g.Cluster.Stars[s0.Star].Name, aShip(g.Empires, 0), aShip(g.Empires, 1) // never the other side's name
		want := []string{s1.Name + " destroys " + as0 + at, s0.Name + " lost to " + as1 + at,
			s0.Name + " destroys " + as1 + at, s1.Name + " lost to " + as0 + at}
		for i, r := range reps {
			if r.Event.Text != want[i] || r.Empire != []int{1, 0, 0, 1}[i] {
				t.Errorf("swap %v report %d: %+v, want %q", swap, i, r, want[i])
			}
		}
	}
}

// Shots come before the move: one generator armed cannot move that
// round, two can; the energy left after the armed shots must cover the
// MoveCost.
func TestShotsBeforeMove(t *testing.T) {
	away := func(sh *Starship) {
		sh.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	}
	for _, c := range []struct {
		name   string
		d      Design
		energy int // 0: the day's
		moves  bool
	}{
		{"one generator", oneGen, 0, false},
		{"two generators", twoGens, 0, true},
		{"two generators, energy for the shot only", twoGens, Gizmos[MassBarrageGun].Power, false},
		{"two generators, energy for the shot and the move", twoGens, Gizmos[MassBarrageGun].Power + twoGens.MoveCost(), true},
	} {
		g, sh, tg := roundDuel(c.d, hulk)
		if c.energy > 0 {
			sh.Power = c.energy
		}
		if sh.Power < Gizmos[MassBarrageGun].Power+c.d.MoveCost() && c.moves {
			t.Fatalf("%s: fixture energy %d", c.name, sh.Power)
		}
		away(sh)
		pos, hull, power := sh.Pos, tg.Hull, sh.Power
		if err := arm(t, g, sh, part(c.d, MassBarrageGun), tg); err != nil {
			t.Fatal(err)
		}
		g.Round(sh.Star, nil)
		moved := sh.Pos != pos
		cost := Gizmos[MassBarrageGun].Power
		if moved {
			cost += c.d.MoveCost()
		}
		if moved != c.moves || tg.Hull != hull-Gizmos[MassBarrageGun].Level || sh.Power != power-cost {
			t.Errorf("%s: moved %v (want %v), target hull %d of %d, energy %d of %d", c.name, moved, c.moves, tg.Hull, hull, sh.Power, power)
		}
		if !c.moves && sh.Order.Kind != GoToPoint {
			t.Errorf("%s: the order was dropped", c.name)
		}
	}
	// Unarmed, the one-generator ship moves.
	g, sh, _ := roundDuel(oneGen, hulk)
	away(sh)
	pos := sh.Pos
	if acted, _, _ := g.Round(sh.Star, nil); !acted || sh.Pos == pos {
		t.Errorf("unarmed: acted %v, moved %v", acted, sh.Pos != pos)
	}

	// A target in range at the round's start is hit, though its order
	// takes it one hop past the range in the same round.
	w := Gizmos[MassBarrageGun]
	rng := float64(w.Range) * RangeUnit
	g, sh, tg := roundDuel(oneGen, runner)
	reach := tg.Design.Reach()
	sh.Pos = Vec3{X: -SystemReach + 0.05}
	tg.Pos = Vec3{X: sh.Pos.X + rng - reach/2}
	tg.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	if tg.Pos.X+reach > SystemReach {
		t.Fatalf("fixture: the runner reaches its point (%.2f + %.2f)", tg.Pos.X, reach)
	}
	if err := arm(t, g, sh, part(oneGen, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	power, hull := sh.Power, tg.Hull
	acted, _, _ := g.Round(sh.Star, nil)
	if !acted || dist(sh.Pos, tg.Pos) <= rng || sh.Power != power-w.Power || tg.Hull != hull-w.Level {
		t.Errorf("fled out of range: acted %v, distance %.2f of %.2f, energy %d of %d, hull %d of %d",
			acted, dist(sh.Pos, tg.Pos), rng, sh.Power, power, tg.Hull, hull)
	}

	// A ship destroyed in the round does not move, and blocks nobody's
	// move: the shooter heading for the very spot it stood on reaches it.
	g, sh, tg = roundDuel(twoGens, runner)
	tg.Hull = w.Level
	tg.Pos = Vec3{X: sh.Pos.X + min(rng, twoGens.Reach())/2, Y: sh.Pos.Y, Z: sh.Pos.Z}
	tg.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	spot := tg.Pos
	sh.Order = Order{Kind: GoToPoint, Pos: spot}
	if err := arm(t, g, sh, part(twoGens, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	_, _, log := g.Round(sh.Star, nil)
	if g.OwnerOf(tg) != nil || tg.Pos != spot || sh.Pos != spot || len(log.Moves) != 1 || log.Moves[0].Ship != sh {
		t.Errorf("destroyed: alive %v, at %v (was %v), shooter at %v, moves %+v", g.OwnerOf(tg) != nil, tg.Pos, spot, sh.Pos, log.Moves)
	}
}

// runner is a target with a drive and no weapon.
var runner = Design{Name: "Runner", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}

// A shot is dropped at no cost when its target is out of range at the
// round's start; one whose target or shooter enters a lane this round
// fires first (shots before moves).
func TestDroppedShots(t *testing.T) {
	w := Gizmos[MassBarrageGun]
	rng := float64(w.Range) * RangeUnit
	// The target stands just past the range and comes closer this round:
	// dropped all the same.
	g, sh, tg := roundDuel(oneGen, runner)
	reach := tg.Design.Reach()
	sh.Pos = Vec3{X: -SystemReach + 0.05}
	tg.Pos = Vec3{X: sh.Pos.X + rng + reach/2}
	tg.Order = Order{Kind: GoToPoint, Pos: sh.Pos}
	if err := arm(t, g, sh, part(oneGen, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	power, shots, hull := sh.Power, slices.Clone(sh.Shots), tg.Hull
	acted, _, _ := g.Round(sh.Star, nil)
	if !acted || dist(sh.Pos, tg.Pos) > rng || sh.Power != power || !slices.Equal(sh.Shots, shots) || tg.Hull != hull {
		t.Errorf("out of range at the start: acted %v, distance %.2f of %.2f, energy %d of %d, shots %v, hull %d of %d",
			acted, dist(sh.Pos, tg.Pos), rng, sh.Power, power, sh.Shots, tg.Hull, hull)
	}
	if len(sh.Armed) != 0 {
		t.Errorf("armed after the round: %v", sh.Armed)
	}

	// The target enters a lane from its endpoint: hit first, then gone.
	g, sh, tg = roundDuel(oneGen, FixtureColonyShip)
	c := g.Cluster
	li := c.Stars[sh.Star].Lanes[0]
	tg.Pos = c.Stars[sh.Star].System.Lanes[0].Pos
	tg.Order = Order{Kind: GoToStar, Star: c.Other(li, sh.Star)}
	sh.Pos = tg.Pos
	if err := arm(t, g, sh, part(oneGen, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	power, hull = sh.Power, tg.Hull
	g.Round(sh.Star, nil)
	if tg.Lane != li || sh.Power != power-w.Power || tg.Hull != hull-w.Level {
		t.Errorf("into a lane: lane %d (want %d), energy %d of %d, hull %d of %d", tg.Lane, li, sh.Power, power, tg.Hull, hull)
	}

	// The shooter enters a lane (two generators: it moves): it fires first.
	armedRunner := Design{Name: "Armed runner", Hull: Small, Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, StarLaneDrive, MassBarrageGun}}
	g, sh, tg = roundDuel(armedRunner, hulk)
	sh.Pos = c.Stars[sh.Star].System.Lanes[0].Pos
	tg.Pos = sh.Pos
	sh.Order = Order{Kind: GoToStar, Star: c.Other(li, sh.Star)}
	if err := arm(t, g, sh, part(armedRunner, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	power, hull = sh.Power, tg.Hull
	g.Round(sh.Star, nil)
	if sh.Lane != li || sh.Power != power-w.Power-armedRunner.MoveCost() || tg.Hull != hull-w.Level {
		t.Errorf("shooter into a lane: lane %d, energy %d of %d, hull %d of %d", sh.Lane, sh.Power, power, tg.Hull, hull)
	}
}

// The rounds stop when the energy is spent, or the Uses; EndDay refills
// both, and leaves nothing armed.
func TestRoundEnergyAndUses(t *testing.T) {
	emp := Gizmos[ElectromagneticPulser]
	for _, c := range []struct {
		name  string
		d     Design
		shots int
		why   string
	}{
		{"energy", Design{Name: "E", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, ElectromagneticPulser}},
			Gizmos[ProtonShaver].Power / emp.Power, "not enough energy"},
		{"uses", Design{Name: "U", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, ElectromagneticPulser}},
			emp.Uses, "no shot left today"},
	} {
		g, sh, tg := roundDuel(c.d, hulk)
		tg.Hull = 1000
		if c.shots < 2 || c.name == "energy" && c.shots >= emp.Uses || c.name == "uses" && sh.Power < emp.Uses*emp.Power+1 {
			t.Fatalf("%s: degenerate fixture (%d shots, energy %d)", c.name, c.shots, sh.Power)
		}
		fired := 0
		for r := range 20 {
			err := arm(t, g, sh, 2, tg)
			acted, _, _ := g.Round(sh.Star, nil)
			if err != nil {
				if !strings.Contains(err.Error(), c.why) || acted {
					t.Errorf("%s round %d: %v, acted %v", c.name, r, err, acted)
				}
				break
			}
			if !acted {
				t.Fatalf("%s round %d: armed, nothing fired", c.name, r)
			}
			fired++
		}
		if fired != c.shots || tg.Hull != 1000-c.shots*emp.Level {
			t.Errorf("%s: %d shots, hull %d, want %d shots", c.name, fired, tg.Hull, c.shots)
		}
		if err := arm(t, g, sh, 2, tg); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Fatalf("%s: re-armed: %v", c.name, err)
		}
		g.EndDay()
		if sh.Power != c.d.PowerPerDay() || sh.Shots[2] != 0 {
			t.Errorf("%s: next day energy %d of %d, shots %v", c.name, sh.Power, c.d.PowerPerDay(), sh.Shots)
		}
		if err := arm(t, g, sh, 2, tg); err != nil {
			t.Errorf("%s: next day: %v", c.name, err)
		}
		g.EndDay()
		if len(sh.Armed) != 0 {
			t.Errorf("%s: EndDay left %v armed", c.name, sh.Armed)
		}
	}
}

// The ships at a star where rounds were played do not Step at the close
// (a passed empire's ship there keeps its place, refilled); the others
// do, and the mark lasts one day.
func TestFoughtStarNoStep(t *testing.T) {
	g, sh, _ := roundDuel(twoGens, hulk)
	e0 := g.Empires[0]
	other := -1
	for st := range g.Cluster.Stars {
		if st != sh.Star && len(g.Cluster.Stars[st].System.Planets) > 0 {
			other = st
			break
		}
	}
	far := e0.Spawn(twoGens, other, 0)
	far.Shields = false
	far.Refill()
	for _, s := range []*Starship{sh, far} {
		s.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	}
	if got := g.CombatStars(); !slices.Equal(got, []int{sh.Star}) {
		t.Fatalf("combat stars %v", got)
	}
	sh.Pos = Vec3{Y: 1.3} // clear of the planets' sweep: the close's push is no Step
	pos, farPos := sh.Pos, far.Pos
	if acted, _, _ := g.Round(sh.Star, []bool{true, false}); acted || sh.Pos != pos {
		t.Fatalf("a passed empire moved: acted %v", acted)
	}
	sh.Power = 0
	g.EndDay()
	if sh.Pos != pos || sh.Power != twoGens.PowerPerDay() {
		t.Errorf("fought star: moved %v, energy %d of %d", sh.Pos != pos, sh.Power, twoGens.PowerPerDay())
	}
	if far.Pos == farPos {
		t.Error("the ship at a quiet star did not Step")
	}
	g.EndDay()
	if sh.Pos == pos {
		t.Error("the next day, no round: the ship did not Step")
	}
}

// CombatStars: a star where two hostile empires both have a ship out of
// the lanes; ascending.
func TestCombatStars(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	allAtWar(t, g)
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	h0, h1 := e0.Colonies[0].Star, e1.Colonies[0].Star
	if got := g.CombatStars(); len(got) != 0 {
		t.Fatalf("no ship: %v", got)
	}
	e0.Spawn(hulk, h0, 0)
	e0.Spawn(hulk, h0, 0)
	if got := g.CombatStars(); len(got) != 0 {
		t.Fatalf("one empire: %v", got)
	}
	b := e1.Spawn(hulk, h0, 0)
	if got := g.CombatStars(); !slices.Equal(got, []int{h0}) {
		t.Errorf("two empires at %d: %v", h0, got)
	}
	b.Lane = 0
	if got := g.CombatStars(); len(got) != 0 {
		t.Errorf("the foe in a lane: %v", got)
	}
	b.Lane = -1
	e2.Spawn(hulk, h1, 0)
	e1.Spawn(hulk, h1, 0)
	want := []int{min(h0, h1), max(h0, h1)}
	if got := g.CombatStars(); !slices.Equal(got, want) {
		t.Errorf("two combat stars: %v, want %v", got, want)
	}
}

// weaponsDamage caps each round at Generators shots: two generators fire
// the Pulser and the Launcher together, then the Pulser on what is left;
// one generator spends it all on the Pulser first (the energy is the
// same, the order differs).
func TestDamageGeneratorCap(t *testing.T) {
	emp, qsl := Gizmos[ElectromagneticPulser], Gizmos[QuantumSingularityLauncher]
	two := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, ProtonShaver, ElectromagneticPulser, QuantumSingularityLauncher}}
	one := Design{Hull: Small, Parts: []Gizmo{SubatomicScoop, ElectromagneticPulser, QuantumSingularityLauncher}}
	energy := 2 * Gizmos[ProtonShaver].Power
	if Gizmos[SubatomicScoop].Power != energy || emp.Power != 1 || energy < emp.Power+qsl.Power || energy >= emp.Uses*emp.Power+qsl.Power {
		t.Fatalf("degenerate fixture: energy %d", energy)
	}
	// Two: round 1 the Pulser and the Launcher, then the Pulser alone.
	wantTwo := qsl.Level + (energy-qsl.Power)/emp.Power*emp.Level
	// One: the Pulser each round until the energy is out (Uses ≥ energy).
	wantOne := min(emp.Uses, energy/emp.Power) * emp.Level
	if got := DesignDamage(two, 0); got != wantTwo {
		t.Errorf("two generators: %d, want %d", got, wantTwo)
	}
	if got := DesignDamage(one, 0); got != wantOne {
		t.Errorf("one generator: %d, want %d", got, wantOne)
	}
	if DesignDamage(Design{Hull: Small, Parts: []Gizmo{MassBarrageGun}}, 0) != 0 {
		t.Error("no generator, damage")
	}
}

// twoGuns is a Medium ship of two weapons and one shield on a Subatomic
// Scoop: not a valid design (no engine), a damage fixture.
var twoGuns = Design{Name: "Two Guns", Hull: Medium,
	Parts: []Gizmo{SubatomicScoop, QuantumSingularityLauncher, ElectromagneticPulser, IonWrap}}

// DamagePerDay against a hand computation from the table: one shot a
// round (one generator), the shooter's shields' draw reserved once while
// on, the Launcher's shot first, the Pulser's Uses shots on what is left,
// each hit as takeShot: Level less the target's shield score while its
// energy pays their draw, nothing for the hit short of it, then the full
// Level; a target with shields off takes full damage; the shooter's live
// Power and Shots are not read. A fresh ship against a fresh target
// equals DesignDamageOn; DesignDamage, the energy unknown, never knocks
// the shields down. A shield draw above the supply leaves the full
// supply to the guns (the shields off, as Refill).
func TestDamagePerDay(t *testing.T) {
	target := Design{Name: "Target", Hull: Small, Parts: []Gizmo{ProtonShaver, ConcussionShield}}
	_, sh, tg := duel(twoGuns, target)
	scoop, wrap := Gizmos[SubatomicScoop], Gizmos[IonWrap]
	qsl, emp := Gizmos[QuantumSingularityLauncher], Gizmos[ElectromagneticPulser]
	if twoGuns.Generators() != 1 || twoGuns.ShieldDraw() != wrap.Power {
		t.Fatalf("fixture: %d generators, draw %d", twoGuns.Generators(), twoGuns.ShieldDraw())
	}
	// hand plays the day: the draw once (the reservation, none when
	// short), then the Launcher's one Use, then the Pulser's, one a round,
	// while the energy lasts.
	tDraw, tEnergy := target.ShieldDraw(), tg.Power
	// hand plays the day against a target whose energy pays the draws.
	handOn := func(draw, absorb, energy int) int {
		power, dmg := scoop.Power, 0
		if draw <= power {
			power -= draw
		}
		for _, w := range append([]GizmoInfo{qsl}, slices.Repeat([]GizmoInfo{emp}, emp.Uses)...) {
			if power < w.Power {
				break
			}
			power -= w.Power
			switch {
			case absorb == 0:
				dmg += w.Level
			case energy < tDraw:
				absorb = 0 // knocked down: this hit does nothing
			default:
				energy -= tDraw
				dmg += max(0, w.Level-absorb)
			}
		}
		return dmg
	}
	hand := func(draw, absorb int) int { return handOn(draw, absorb, tEnergy) }
	absorb := Gizmos[ConcussionShield].Level
	if !tg.Shields || tg.Absorb() != absorb {
		t.Fatalf("target shields %v absorb %d", tg.Shields, tg.Absorb())
	}
	cases := []struct {
		name            string
		shields, tShlds bool
		want            int
	}{
		{"both shields on", true, true, hand(wrap.Power, absorb)},
		{"target shields off", true, false, hand(wrap.Power, 0)},
		{"shooter shields off", false, true, hand(0, absorb)},
		{"both off", false, false, hand(0, 0)},
	}
	for _, k := range cases {
		sh.Shields, tg.Shields = k.shields, k.tShlds
		sh.Power, sh.Shots[1] = 0, 5 // spent today: not read
		if got := sh.DamagePerDay(tg); got != k.want {
			t.Errorf("%s: %d, want %d", k.name, got, k.want)
		}
	}
	// The target's shields and the shooter's draw both tell, or the test
	// proves nothing (against the Concussion Shield the Pulser's extra
	// shots do nothing: cases 0 and 2 are equal by the table).
	if cases[0].want == cases[1].want || cases[1].want == cases[3].want {
		t.Fatalf("degenerate fixture: %+v", cases)
	}
	// The target's energy pays fewer draws than the Launcher and the
	// Pulser's shots, or the knock-down is not walked.
	if tEnergy/tDraw >= 1+emp.Uses || tEnergy != target.PowerPerDay()-tDraw {
		t.Fatalf("fixture: target energy %d, draw %d", tEnergy, tDraw)
	}
	sh.Shields, tg.Shields = true, true
	if got, want := DesignDamageOn(twoGuns, target), sh.DamagePerDay(tg); got != want {
		t.Errorf("DesignDamageOn %d, a fresh ship's %d", got, want)
	}
	unknown := handOn(wrap.Power, absorb, math.MaxInt/2) // DesignDamage: every hit absorbed
	if got := DesignDamage(twoGuns, absorb); got != unknown || got == DesignDamageOn(twoGuns, target) {
		t.Errorf("DesignDamage %d, want %d (≠ DesignDamageOn %d: the knock-down walked)", got, unknown, DesignDamageOn(twoGuns, target))
	}
	// A Hyperwave Nullifier draws more than a Proton Shaver supplies: the
	// shields are off (Refill), the gun fires on the full supply.
	dark := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, MassBarrageGun, MassBarrageGun, HyperwaveNullifier}}
	mbg := Gizmos[MassBarrageGun]
	if got, want := DesignDamage(dark, 0), min(2, Gizmos[ProtonShaver].Power/mbg.Power)*mbg.Level; got != want {
		t.Errorf("dark shields: %d, want %d", got, want)
	}
}

// Round's log: the moves (From before the Hop, To after), the shots that
// fired (the round's starting positions: shots before moves; the damage
// and what the shields took off), the destroyed in destruction order; a
// lane entry is a move to the lane endpoint, a dropped shot is not logged.
func TestRoundLog(t *testing.T) {
	w := Gizmos[MassBarrageGun]
	// A move and a shot the target's shields absorb.
	wrapped := Design{Name: "Wrapped", Hull: Small, Parts: []Gizmo{ProtonShaver, IonWrap}}
	g, sh, tg := roundDuel(twoGens, wrapped)
	tg.Shields = true
	if tg.Absorb() < w.Level {
		t.Fatalf("fixture: absorb %d < level %d", tg.Absorb(), w.Level)
	}
	sh.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	from := sh.Pos
	if err := arm(t, g, sh, part(twoGens, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	dmg := shotDamage(w.Level, tg.Absorb())
	_, _, log := g.Round(sh.Star, nil)
	want := RoundLog{Star: sh.Star,
		Moves: []Move{{Ship: sh, Empire: 0, From: from, To: sh.Pos, Orbit: -1}},
		Shots: []Fired{{Shooter: sh, Target: tg, ShooterEmpire: 0, TargetEmpire: 1, Gizmo: MassBarrageGun,
			From: from, To: tg.Pos, Damage: dmg, Absorbed: w.Level - dmg}}}
	if sh.Pos == from || !reflect.DeepEqual(log, want) {
		t.Errorf("move and absorbed shot:\n got %+v\nwant %+v", log, want)
	}

	// Two shots, the target destroyed; nothing moves (both generators armed).
	g, sh, tg = roundDuel(twoGens, hulk)
	tg.Hull = w.Level
	for _, p := range []int{3, 4} {
		if err := arm(t, g, sh, p, tg); err != nil {
			t.Fatal(err)
		}
	}
	_, _, log = g.Round(sh.Star, nil)
	shot := Fired{Shooter: sh, Target: tg, ShooterEmpire: 0, TargetEmpire: 1, Gizmo: MassBarrageGun,
		From: sh.Pos, To: tg.Pos, Damage: w.Level}
	want = RoundLog{Star: sh.Star, Shots: []Fired{shot, shot}, Destroyed: []*Starship{tg}}
	if g.OwnerOf(tg) != nil || !reflect.DeepEqual(log, want) {
		t.Errorf("kill:\n got %+v\nwant %+v", log, want)
	}

	// The target enters a lane: shot first, from where it stood, then its
	// move is logged.
	g, sh, tg = roundDuel(oneGen, FixtureColonyShip)
	c := g.Cluster
	li := c.Stars[sh.Star].Lanes[0]
	end := c.Stars[sh.Star].System.Lanes[0].Pos
	tg.Pos = Vec3{X: end.X * 0.99, Y: end.Y * 0.99, Z: end.Z * 0.99}
	if dist(tg.Pos, end) > tg.Design.Reach() {
		t.Fatalf("fixture: the lane end is %.3f away, reach %.3f", dist(tg.Pos, end), tg.Design.Reach())
	}
	from = tg.Pos
	tg.Order = Order{Kind: GoToStar, Star: c.Other(li, sh.Star)}
	sh.Pos = end
	if err := arm(t, g, sh, part(oneGen, MassBarrageGun), tg); err != nil {
		t.Fatal(err)
	}
	_, _, log = g.Round(sh.Star, nil)
	want = RoundLog{Star: sh.Star, Moves: []Move{{Ship: tg, Empire: 1, From: from, To: end, Orbit: -1}},
		Shots: []Fired{{Shooter: sh, Target: tg, ShooterEmpire: 0, TargetEmpire: 1, Gizmo: MassBarrageGun,
			From: end, To: from, Damage: w.Level}}}
	if tg.Lane != li || !reflect.DeepEqual(log, want) {
		t.Errorf("into a lane (lane %d):\n got %+v\nwant %+v", tg.Lane, log, want)
	}
}

// A combat day costs the shields once: the reservation off Power from
// the refill, no round draws anything (passed or not, shields on or off),
// a shot pays only its weapon's Power, the hit its target's draw
// (takeShot); the next refill takes the draw again.
func TestRoundNoShieldDraw(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	draw, full := fightFixture.ShieldDraw(), fightFixture.PowerPerDay()
	if draw < 1 || !sh.Shields || sh.Power != full-draw || !tg.Shields || tg.Power != full-draw {
		t.Fatalf("fixture: draw %d, powers %d %d of %d, shields %v %v", draw, sh.Power, tg.Power, full, sh.Shields, tg.Shields)
	}
	if err := tg.SetShields(false); err != nil || tg.Power != full {
		t.Fatalf("foe off: %v, power %d of %d", err, tg.Power, full)
	}
	for k := range 3 {
		passed := []bool{k%2 == 1, false}
		if acted, _, _ := g.Round(sh.Star, passed); acted {
			t.Fatalf("round %d acted", k)
		}
		if sh.Power != full-draw || tg.Power != full || sh.ShieldsDown || sh.Absorb() != fightFixture.ShieldScore() {
			t.Fatalf("round %d: power %d, want %d; foe %d of %d; down %v absorb %d", k, sh.Power, full-draw, tg.Power, full, sh.ShieldsDown, sh.Absorb())
		}
	}
	mbg := Gizmos[MassBarrageGun]
	if err := arm(t, g, tg, part(fightFixture, MassBarrageGun), sh); err != nil {
		t.Fatal(err)
	}
	hull := sh.Hull
	_, _, log := g.Round(sh.Star, nil)
	if want := shotDamage(mbg.Level, fightFixture.ShieldScore()); len(log.Shots) != 1 || log.Shots[0].Damage != want || sh.Hull != hull-want ||
		tg.Power != full-mbg.Power || sh.Power != full-2*draw {
		t.Fatalf("a shot: %+v, hull %d of %d; foe power %d, want %d; power %d, want %d", log.Shots, sh.Hull, hull, tg.Power, full-mbg.Power, sh.Power, full-2*draw)
	}
	g.EndDay()
	if !sh.Shields || sh.Power != full-draw || tg.Shields || tg.Power != full {
		t.Errorf("next day: shields %v power %d, want %d; foe shields %v power %d of %d", sh.Shields, sh.Power, full-draw, tg.Shields, tg.Power, full)
	}
}

// The reservation holds on a day with no round too: shields on, the
// energy PowerPerDay less ShieldDraw every day (the original's 10 → 7).
func TestShieldReservationDaily(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	sh := e.Spawn(fightFixture, e.Colonies[0].Star, e.Colonies[0].PlanetIdx)
	want := fightFixture.PowerPerDay() - fightFixture.ShieldDraw()
	for range 3 {
		if sh.Power != want || !sh.Shields || len(g.CombatStars()) != 0 {
			t.Fatalf("day %d: power %d, want %d, shields %v, combat stars %v", g.Day, sh.Power, want, sh.Shields, g.CombatStars())
		}
		g.EndDay()
	}
}

// Arm counts the energy as it stands (the reservation off it): a shot it
// cannot pay is refused, accepted with exactly enough; shields switched
// on after arming take their draw at once, and Round drops the shot the
// energy no longer covers, at no cost.
func TestArmAfterReservation(t *testing.T) {
	gun := part(fightFixture, MassBarrageGun)
	w, draw := Gizmos[MassBarrageGun], fightFixture.ShieldDraw()
	g, sh, tg := duel(fightFixture, fightFixture)
	_ = tg.SetShields(false)
	sh.Power = w.Power - 1
	if err := arm(t, g, sh, gun, tg); err == nil || !strings.Contains(err.Error(), "not enough energy") {
		t.Errorf("short of the shot: %v", err)
	}
	if err := sh.SetShields(false); err != nil || sh.Power != w.Power-1+draw {
		t.Fatalf("off: %v, power %d", err, sh.Power)
	}
	sh.Power = w.Power
	if err := arm(t, g, sh, gun, tg); err != nil {
		t.Fatalf("shields off, exactly the shot: %v", err)
	}
	sh.Power = w.Power - 1 + draw
	if err := sh.SetShields(true); err != nil || sh.Power != w.Power-1 {
		t.Fatalf("on after arming: %v, power %d", err, sh.Power)
	}
	hull := tg.Hull
	_, _, log := g.Round(sh.Star, nil)
	if len(log.Shots) != 0 || sh.Shots[gun] != 0 || tg.Hull != hull || sh.Power != w.Power-1 {
		t.Errorf("shield on after arming: shots %+v, used %d, hull %d of %d, power %d", log.Shots, sh.Shots[gun], tg.Hull, hull, sh.Power)
	}

	g, sh, tg = duel(fightFixture, fightFixture)
	_ = tg.SetShields(false)
	sh.Power = w.Power
	if err := arm(t, g, sh, gun, tg); err != nil {
		t.Fatalf("exactly enough: %v", err)
	}
	_, _, log = g.Round(sh.Star, nil)
	if len(log.Shots) != 1 || sh.Power != 0 || !sh.Shields || sh.ShieldsDown {
		t.Errorf("exactly enough: shots %+v, power %d, shields %v down %v", log.Shots, sh.Power, sh.Shields, sh.ShieldsDown)
	}
}

// Fired.ShieldsDown marks the one shot that knocks the shields down: the
// target's energy short of its ShieldDraw, the hit does no damage (takeShot)
// and is logged down; the next round's hit on the dark shields does the
// full level, not logged down again.
func TestFiredShieldsDown(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	gun, w := part(fightFixture, MassBarrageGun), Gizmos[MassBarrageGun]
	tg.Power = fightFixture.ShieldDraw() - 1
	if !tg.Shields || tg.ShieldsDown || tg.Absorb() == 0 {
		t.Fatalf("fixture: shields %v down %v absorb %d", tg.Shields, tg.ShieldsDown, tg.Absorb())
	}
	for k, want := range []Fired{{Damage: 0, Absorbed: w.Level, ShieldsDown: true}, {Damage: w.Level}} {
		sh.Shots[gun], sh.Power = 0, w.Power // a shot a day: the next round's given back
		if err := arm(t, g, sh, gun, tg); err != nil {
			t.Fatalf("round %d: %v", k, err)
		}
		_, _, log := g.Round(sh.Star, nil)
		if len(log.Shots) != 1 {
			t.Fatalf("round %d: %d shots", k, len(log.Shots))
		}
		if f := log.Shots[0]; f.Target != tg || f.Damage != want.Damage || f.Absorbed != want.Absorbed || f.ShieldsDown != want.ShieldsDown || !tg.ShieldsDown {
			t.Errorf("round %d: damage %d absorbed %d down %v (target dark %v), want %d %d %v",
				k, f.Damage, f.Absorbed, f.ShieldsDown, tg.ShieldsDown, want.Damage, want.Absorbed, want.ShieldsDown)
		}
	}
}
