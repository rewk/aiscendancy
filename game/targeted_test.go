package game

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The targeted specials and the area ones (docs/rules.md "Ship
// specials"): targetCheck's rules, the three day flags, each
// effect's success and own refusal, the victims' reports.

// aimer is a user design for the targeted specials: four Nanotwirlers
// (the energy for any of them), a drive, then gs.
func aimer(gs ...Gizmo) Design {
	return Design{Name: "Aimer", Hull: Large, Parts: append([]Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, TonklinMotor}, gs...)}
}

// foeShip carries a bit of everything: a generator, a drive, two guns
// (parts 2 and 5), a shield, a Colonizer.
var foeShip = Design{Name: "Foe", Hull: Medium, Parts: []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun, IonWrap, Colonizer, MassBarrageGun}}

// aimGame is a two-empire game (Snovemdomas, Orfa) at War: the user's s
// (empire 0, design d) in space on empire 0's home planet, foe (empire
// 1, design fd) in space off along X from it, both refilled with the
// shields off; tg is foe as empire 0 sees it, a ShipTarget.
func aimGame(t *testing.T, d, fd Design, off float64) (g *Game, e *Empire, s, foe *Starship, tg Target) {
	t.Helper()
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e = g.Empires[0]
	home := e.Colonies[0]
	s = e.Spawn(d, home.Star, home.PlanetIdx)
	foe = g.Empires[1].Spawn(fd, home.Star, home.PlanetIdx)
	s.Shields, foe.Shields = false, false
	at := s.Pos
	inSpaceAt(s, at)
	at.X += off
	inSpaceAt(foe, at)
	return g, e, s, foe, shipTarget(t, g, e, foe)
}

// shipTarget is x as e sees it, a ShipTarget.
func shipTarget(t *testing.T, g *Game, e *Empire, x *Starship) Target {
	t.Helper()
	sg, ok := g.SightingOf(e, x)
	if !ok {
		t.Fatalf("%s does not see %s", e.Species, x.Name)
	}
	return Target{Kind: ShipTarget, Ship: sg}
}

// targetCheck's refusals, the ship's state unchanged each time: the
// hostile aim (shooter in orbit, a Mine target, both set, not hostile, a
// lane, another system, in orbit, out of range, gone), the ally aim (a
// hostile Sighting, itself, out of range, a lane; an own ship in orbit
// accepted), the planet aim (no planet, own planet), and the
// Carbonizer's lane and planet targets not modelled.
func TestTargetCheck(t *testing.T) {
	d := aimer(MolecularTieDown, SacrificialOrb, MyrmidonicCarbonizer, PhaseBomb)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	tie, orb, carb, bomb := part(d, MolecularTieDown), part(d, SacrificialOrb), part(d, MyrmidonicCarbonizer), part(d, PhaseBomb)
	home := e.Colonies[0]
	mate := e.Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(mate, s.Pos)
	mine := Target{Kind: ShipTarget, Mine: mate}

	s.Orbit = home.PlanetIdx
	useRefused(t, e, s, tie, tg, "in orbit: leave it first")
	s.Orbit = -1
	useRefused(t, e, s, tie, mine, "not hostile")
	both := tg
	both.Mine = mate
	useRefused(t, e, s, tie, both, "no such target")
	e.Rel(1).Stance = Peace
	useRefused(t, e, s, tie, tg, "not hostile")
	e.Rel(1).Stance = War
	foe.Lane = 0
	useRefused(t, e, s, tie, tg, "the target is in a star lane")
	foe.Lane = -1
	foe.Star = (home.Star + 1) % len(g.Cluster.Stars)
	useRefused(t, e, s, tie, tg, "not in the same system")
	foe.Star = home.Star
	foe.Orbit = home.PlanetIdx
	useRefused(t, e, s, tie, tg, "fire at the planet")
	foe.Orbit = -1
	r := float64(Gizmos[MolecularTieDown].Range) * RangeUnit
	foe.Pos = s.Pos
	foe.Pos.X += r + 0.01
	useRefused(t, e, s, tie, tg, "Molecular Tie Down: out of range")
	if foe.TiedDown {
		t.Fatal("a refusal tied it down")
	}

	useRefused(t, e, s, orb, tg, "not an ally")
	useRefused(t, e, s, orb, Target{Kind: ShipTarget, Mine: s}, "not on itself")
	useRefused(t, e, s, orb, Target{Kind: ShipTarget}, "no such target")
	useRefused(t, e, s, orb, mine, "nothing to repair") // targetCheck passed: the Orb's own refusal
	mate.Hull--
	mate.Lane = 0
	useRefused(t, e, s, orb, mine, "the target is in a star lane")
	mate.Lane = -1
	mate.Pos.X += float64(Gizmos[SacrificialOrb].Range)*RangeUnit + 0.01
	useRefused(t, e, s, orb, mine, "Sacrificial Orb: out of range")
	mate.Orbit, mate.Pos = home.PlanetIdx, home.Planet.Pos // in orbit: allowed
	useAt(t, e, s, orb, mine)
	if mate.Hull != mate.MaxHull() {
		t.Errorf("mate in orbit not repaired: %d of %d", mate.Hull, mate.MaxHull())
	}

	useRefused(t, e, s, carb, Target{Kind: PlanetTarget, Index: home.PlanetIdx}, "not modelled yet")
	useRefused(t, e, s, carb, Target{Kind: LaneTarget}, "not modelled yet")
	useRefused(t, e, s, bomb, Target{Kind: PlanetTarget, Index: home.PlanetIdx}, "not hostile")
	useRefused(t, e, s, bomb, Target{Kind: PlanetTarget, Index: 99}, "no planet 99")

	foe.Pos = s.Pos
	mate.Orbit = -1
	g.Destroy(foe) // the Sighting is stale, the hull still whole
	useRefused(t, e, s, tie, tg, "no such target")
	if err := e.Arm(mate, part(foeShip, MassBarrageGun), tg.Ship); err == nil || err.Error() != "no such target" {
		t.Errorf("Arm at a destroyed ship's Sighting: %v", err)
	}
}

// The ally aim through a Sighting: refused at Peace, accepted at
// Alliance.
func TestAllyTarget(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	g.Contact(0, 1)
	e := g.Empires[0]
	home := e.Colonies[0]
	d := aimer(PlasmaCoupler)
	s := e.Spawn(d, home.Star, home.PlanetIdx)
	inSpaceAt(s, s.Pos)
	friend := g.Empires[1].Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(friend, s.Pos)
	tg := shipTarget(t, g, e, friend)
	k := part(d, PlasmaCoupler)
	useRefused(t, e, s, k, tg, "not an ally")
	ally(t, g, 0, 1)
	power := friend.Power
	useAt(t, e, s, k, tg)
	if friend.Power <= power {
		t.Errorf("no energy beamed: %d", friend.Power)
	}
}

// The day flags: TiedDown (Hop and the Round's moves find nothing, the
// order and the energy kept), Scrambled (Arm refuses, the Round drops a
// shot armed before), ShieldsDown (Absorb 0, Shields kept); the Refill
// clears all three.
func TestDayFlags(t *testing.T) {
	d := aimer(MolecularTieDown, IntellectScrambler, ShieldBlaster)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	e1 := g.Empires[1]
	gun := part(foeShip, MassBarrageGun)

	useAt(t, e, s, part(d, MolecularTieDown), tg)
	if !foe.TiedDown {
		t.Fatal("not tied down")
	}
	away := foe.Pos
	away.X += 0.3
	foe.Order = Order{Kind: GoToPoint, Pos: away}
	power, pos := foe.Power, foe.Pos
	if moved, arrived := foe.Hop(g.Cluster, e1.Day, nil, nil); moved || arrived || foe.Power != power || foe.Pos != pos || foe.Order.Kind != GoToPoint {
		t.Fatalf("tied down hopped: %v %v power %d pos %v order %v", moved, arrived, foe.Power, foe.Pos, foe.Order)
	}
	if _, _, log := g.Round(foe.Star, nil); slices.ContainsFunc(log.Moves, func(m Move) bool { return m.Ship == foe }) {
		t.Errorf("tied down moved in the round: %+v", log.Moves)
	}

	if err := arm(t, g, foe, gun, s); err != nil {
		t.Fatal(err)
	}
	useAt(t, e, s, part(d, IntellectScrambler), tg)
	if !foe.Scrambled || foe.Armed != nil {
		t.Fatalf("scrambled %v armed %v", foe.Scrambled, foe.Armed)
	}
	if err := arm(t, g, foe, gun, s); err == nil || !strings.Contains(err.Error(), "crew scrambled") {
		t.Errorf("Arm scrambled: %v", err)
	}
	foe.Armed = []Shot{{Part: gun, Target: shipTarget(t, g, e1, s).Ship}} // armed before the scramble
	power, hull := foe.Power, s.Hull
	if _, _, log := g.Round(foe.Star, nil); len(log.Shots) != 0 || foe.Power != power || foe.Shots[gun] != 0 || s.Hull != hull {
		t.Errorf("a scrambled shot fired: %+v power %d", log.Shots, foe.Power)
	}

	foe.Shields = true
	score := foe.Design.ShieldScore()
	if score == 0 || foe.Absorb() != score {
		t.Fatalf("shields: score %d absorb %d", score, foe.Absorb())
	}
	useAt(t, e, s, part(d, ShieldBlaster), tg)
	if !foe.ShieldsDown || foe.Absorb() != 0 || !foe.Shields {
		t.Errorf("blasted: down %v absorb %d shields %v", foe.ShieldsDown, foe.Absorb(), foe.Shields)
	}

	foe.Refill()
	if foe.TiedDown || foe.Scrambled || foe.ShieldsDown {
		t.Errorf("Refill kept %v %v %v", foe.TiedDown, foe.Scrambled, foe.ShieldsDown)
	}
	if foe.Shields && foe.Absorb() != score {
		t.Errorf("shields not back: %d", foe.Absorb())
	}
	if err := arm(t, g, foe, gun, s); err != nil {
		t.Errorf("unscrambled Arm: %v", err)
	}
	foe.Armed = nil
	if moved, _ := foe.Hop(g.Cluster, e1.Day, nil, nil); !moved {
		t.Error("untied ship does not move")
	}
}

// Brunswik Dissipator: the target's energy 0, its armed shots dropped.
func TestBrunswik(t *testing.T) {
	d := aimer(BrunswikDissipator)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	if err := arm(t, g, foe, part(foeShip, MassBarrageGun), s); err != nil || foe.Power == 0 {
		t.Fatalf("arm %v power %d", err, foe.Power)
	}
	reps := useAt(t, e, s, part(d, BrunswikDissipator), tg)
	if foe.Power != 0 || foe.Armed != nil || len(reps) != 2 { // the user's, the victim's
		t.Errorf("power %d armed %v reps %d", foe.Power, foe.Armed, len(reps))
	}
	if s.Power != d.PowerPerDay()-Gizmos[BrunswikDissipator].Power {
		t.Errorf("unpaid: %d", s.Power)
	}
}

// Disarmer: the target's first weapon lost; its chosen Weapon reset, its
// armed shot at the second gun shifted down with it and still fired; the
// template's Parts untouched; refused with no weapon aboard.
func TestDisarmer(t *testing.T) {
	d := aimer(Disarmer)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	second := 5
	foe.Weapon = second
	if err := arm(t, g, foe, second, s); err != nil {
		t.Fatal(err)
	}
	lost := foe.Design.Parts[firstOf(foe, isCat(Weapon))]
	reps := useAt(t, e, s, part(d, Disarmer), tg)
	// The user's line names the target "a <Species> ship" (never a foreign
	// name); its Use carries the part lost, no name.
	if u := reps[0].Event.Use; u.Outcome != lost.String()+" destroyed" ||
		!strings.Contains(reps[0].Event.Text, s.Name+"'s Disarmer destroys "+aShip(g.Empires, tg.Ship.Empire)+"'s") {
		t.Errorf("user's line %q, outcome %q", reps[0].Event.Text, u.Outcome)
	}
	// The victim reads it too, the user by its species alone (a gun
	// lost unannounced).
	owner := tg.Ship.Empire
	if len(reps) != 2 || reps[0].Empire != e.Index || reps[1].Empire != owner || reps[1].Event.Kind != SpecialUsed ||
		strings.Contains(reps[1].Event.Text, s.Name) || !strings.Contains(reps[1].Event.Text, "a "+s.Species.String()+" ship's Disarmer destroys "+foe.Name+"'s") {
		t.Errorf("reports %+v", reps)
	}
	want := []Gizmo{ProtonShaver, TonklinMotor, IonWrap, Colonizer, MassBarrageGun}
	if !slices.Equal(foe.Design.Parts, want) || len(foe.Shots) != len(want) || foe.Weapon != -1 {
		t.Fatalf("parts %v shots %v weapon %d", foe.Design.Parts, foe.Shots, foe.Weapon)
	}
	if !slices.Equal(foeShip.Parts, []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun, IonWrap, Colonizer, MassBarrageGun}) {
		t.Errorf("the template changed: %v", foeShip.Parts)
	}
	if len(foe.Armed) != 1 || foe.Armed[0].Part != second-1 {
		t.Fatalf("armed %+v", foe.Armed)
	}
	if _, _, log := g.Round(s.Star, nil); len(log.Shots) != 1 || log.Shots[0].Shooter != foe || log.Shots[0].Gizmo != MassBarrageGun {
		t.Errorf("the shifted shot: %+v", log.Shots)
	}

	_, e, s, _, tg = aimGame(t, d, hulk, 0.1)
	useRefused(t, e, s, part(d, Disarmer), tg, "no weapon aboard")
}

// Containment Device: the first live Colonizer lost with its Cargo
// (one of two left: the count goes down, not just capped at the parts);
// a spent Colonizer's part is skipped for a live module (Invaders down,
// one of two modules spent); refused with neither left.
func TestContainment(t *testing.T) {
	d := aimer(ContainmentDevice)
	k := part(d, ContainmentDevice)
	settler := Design{Name: "Settler", Hull: Medium, Parts: []Gizmo{Nanotwirler, TonklinMotor, Colonizer, Colonizer}}
	_, e, s, foe, tg := aimGame(t, d, settler, 0.1)
	foe.Cargo = 1 // one founded already
	useAt(t, e, s, k, tg)
	if foe.Cargo != 0 || !slices.Equal(foe.Design.Parts, settler.Parts[:3]) {
		t.Fatalf("cargo %d parts %v", foe.Cargo, foe.Design.Parts)
	}
	s.Refill()
	useRefused(t, e, s, k, tg, "no colonizer nor invasion module aboard")

	lander := Design{Name: "Lander", Hull: Medium, Parts: []Gizmo{Nanotwirler, TonklinMotor, Colonizer, InvasionModule, InvasionModule}}
	_, e, s, foe, tg = aimGame(t, d, lander, 0.1)
	foe.Cargo, foe.Invaders = 0, 1 // colonised already (the part stays, spent), one module spent
	useAt(t, e, s, k, tg)
	if foe.Invaders != 0 || !slices.Equal(foe.Design.Parts, []Gizmo{Nanotwirler, TonklinMotor, Colonizer, InvasionModule}) {
		t.Errorf("invaders %d parts %v", foe.Invaders, foe.Design.Parts)
	}
}

// Backfirer: Σ Level of the target's weapons, its shields not absorbing;
// a kill destroys it with the two reports credited to the user and the
// ledger counting it; refused with no weapon aboard.
func TestBackfirer(t *testing.T) {
	d := aimer(Backfirer)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	foe.Shields = true
	if foe.Absorb() == 0 {
		t.Fatal("no shield to ignore")
	}
	sum := 0
	for _, x := range foeShip.Parts {
		if Gizmos[x].Cat == Weapon {
			sum += Gizmos[x].Level
		}
	}
	hull := foe.Hull
	useAt(t, e, s, part(d, Backfirer), tg)
	if foe.Hull != hull-sum {
		t.Fatalf("hull %d, want %d", foe.Hull, hull-sum)
	}
	s.Refill()
	foe.Hull = sum
	reps := useAt(t, e, s, part(d, Backfirer), tg)
	destroyedPair(t, g, reps, s, foe)

	_, e, s, _, tg = aimGame(t, d, hulk, 0.1)
	useRefused(t, e, s, part(d, Backfirer), tg, "no weapon aboard")
}

// destroyedPair checks reps[1:3] are the Destroyed pair of foe (empire
// 1) credited to s (empire 0), foe gone, empire 1's ledger counting it.
func destroyedPair(t *testing.T, g *Game, reps []Report, s, foe *Starship) {
	t.Helper()
	if len(reps) != 3 {
		t.Fatalf("reps %+v", reps)
	}
	mine, theirs := reps[1], reps[2]
	if mine.Empire != 0 || theirs.Empire != 1 || mine.Event.Kind != Destroyed || theirs.Event.Kind != Destroyed ||
		mine.Event.Text != s.Name+" destroys "+aShip(g.Empires, 1)+" at "+g.Cluster.Stars[foe.Star].Name ||
		theirs.Event.Text != foe.Name+" lost to "+aShip(g.Empires, 0)+" at "+g.Cluster.Stars[foe.Star].Name {
		t.Errorf("pair %+v", reps[1:])
	}
	if g.OwnerOf(foe) != nil || g.Empires[1].Rel(0).ShipsLost != 1 {
		t.Errorf("owner %v, lost %d", g.OwnerOf(foe), g.Empires[1].Rel(0).ShipsLost)
	}
}

// Moving Part Exploiter: a hull point per part aboard, no absorb; an
// Invulnerable target takes nothing.
func TestExploiter(t *testing.T) {
	d := aimer(MovingPartExploiter)
	_, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	foe.Shields = true
	hull := foe.Hull
	reps := useAt(t, e, s, part(d, MovingPartExploiter), tg)
	if foe.Hull != hull-len(foeShip.Parts) || reps[0].Event.Use.Outcome != fmt.Sprintf("-%d", hull-foe.Hull) {
		t.Errorf("hull %d, want %d; outcome %q", foe.Hull, hull-len(foeShip.Parts), reps[0].Event.Use.Outcome)
	}
	s.Refill()
	foe.Invulnerable, hull = true, foe.Hull
	reps = useAt(t, e, s, part(d, MovingPartExploiter), tg)
	if foe.Hull != hull || reps[0].Event.Use.Outcome != "-0" {
		t.Errorf("invulnerable harmed: %d of %d, outcome %q", foe.Hull, hull, reps[0].Event.Use.Outcome)
	}
}

// Gizmogrifier: the target's Parts[0] lost (a Colonizer here: Cargo
// follows the parts); refused on a ship with no part.
func TestGizmogrifier(t *testing.T) {
	d := aimer(Gizmogrifier)
	fd := Design{Name: "Settler", Hull: Medium, Parts: []Gizmo{Colonizer, ProtonShaver, TonklinMotor}}
	_, e, s, foe, tg := aimGame(t, d, fd, 0.1)
	if foe.Cargo != 1 {
		t.Fatalf("cargo %d", foe.Cargo)
	}
	useAt(t, e, s, part(d, Gizmogrifier), tg)
	if !slices.Equal(foe.Design.Parts, fd.Parts[1:]) || foe.Cargo != 0 {
		t.Errorf("parts %v cargo %d", foe.Design.Parts, foe.Cargo)
	}
	_, e, s, _, tg = aimGame(t, d, Design{Name: "Shell", Hull: Small}, 0.1)
	useRefused(t, e, s, part(d, Gizmogrifier), tg, "no part aboard")
}

// Specialty Blaster: the first Special lost; refused with none aboard.
func TestSpecialtyBlaster(t *testing.T) {
	d := aimer(SpecialtyBlaster)
	_, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	useAt(t, e, s, part(d, SpecialtyBlaster), tg)
	if slices.Contains(foe.Design.Parts, Colonizer) || len(foe.Design.Parts) != len(foeShip.Parts)-1 {
		t.Errorf("parts %v", foe.Design.Parts)
	}
	_, e, s, _, tg = aimGame(t, d, hulk, 0.1)
	useRefused(t, e, s, part(d, SpecialtyBlaster), tg, "no special aboard")
}

// Disintegrator: the target destroyed outright, the device lost; an
// Invulnerable target shrugs it off, the use spent and the device lost.
func TestDisintegrator(t *testing.T) {
	d := aimer(Disintegrator)
	k := part(d, Disintegrator)
	g, e, s, foe, tg := aimGame(t, d, foeShip, 0.1)
	reps := useAt(t, e, s, k, tg)
	destroyedPair(t, g, reps, s, foe)
	if u := reps[0].Event.Use; u.Outcome != "destroyed" || u.Target != foe {
		t.Errorf("use %+v", u)
	}
	if slices.Contains(s.Design.Parts, Disintegrator) {
		t.Error("the device kept")
	}

	_, e, s, foe, tg = aimGame(t, d, foeShip, 0.1)
	foe.Invulnerable = true
	reps = useAt(t, e, s, k, tg)
	if len(reps) != 2 || !strings.Contains(reps[0].Event.Text, "shrugged off") || reps[0].Event.Use.Outcome != "shrugged off" || !slices.Contains(e.peerList()[1].Ships, foe) {
		t.Errorf("invulnerable: %+v", reps)
	}
	if slices.Contains(s.Design.Parts, Disintegrator) || s.Power != d.PowerPerDay()-Gizmos[Disintegrator].Power {
		t.Errorf("not spent: parts %v power %d", s.Design.Parts, s.Power)
	}
}

// The Carbonizer's curve: Level at the ideal range, half at half of it,
// 0 at the user, near 0 just inside Range; a use at the ideal range does
// Level less the target's shields.
func TestCarbonizer(t *testing.T) {
	in := &Gizmos[MyrmidonicCarbonizer]
	ideal := float64(in.Special1) * RangeUnit
	r := float64(in.Range) * RangeUnit
	for _, c := range []struct {
		d    float64
		want int
	}{{ideal, in.Level}, {ideal / 2, in.Level / 2}, {0, 0}, {r - 1e-3, 0}, {(ideal + r) / 2, in.Level / 2}} {
		if got := carbonizerLevel(c.d); got != c.want {
			t.Errorf("at %v: %d, want %d", c.d, got, c.want)
		}
	}
	d := aimer(MyrmidonicCarbonizer)
	_, e, s, foe, tg := aimGame(t, d, foeShip, ideal)
	foe.Shields = true
	hull := foe.Hull
	useAt(t, e, s, part(d, MyrmidonicCarbonizer), tg)
	if want := hull - (in.Level - foe.Absorb()); foe.Hull != want {
		t.Errorf("hull %d, want %d", foe.Hull, want)
	}
}

// Sacrificial Orb: Special1 of the target's damage moves onto the user
// (at most the damage); refused when it would scuttle the user.
func TestSacrificialOrb(t *testing.T) {
	d := aimer(SacrificialOrb)
	k := part(d, SacrificialOrb)
	_, e, s, _, _ := aimGame(t, d, hulk, 0.1)
	home := e.Colonies[0]
	mate := e.Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(mate, s.Pos)
	mine := Target{Kind: ShipTarget, Mine: mate}
	n := Gizmos[SacrificialOrb].Special1
	mate.Hull = mate.MaxHull() - n - 1
	hull := s.Hull
	useAt(t, e, s, k, mine)
	if mate.Hull != mate.MaxHull()-1 || s.Hull != hull-n {
		t.Fatalf("mate %d user %d", mate.Hull, s.Hull)
	}
	useAt(t, e, s, k, mine) // Uses 0: again, one point left to take
	if mate.Hull != mate.MaxHull() || s.Hull != hull-n-1 {
		t.Fatalf("capped: mate %d user %d", mate.Hull, s.Hull)
	}
	mate.Hull -= n
	s.Hull = n
	useRefused(t, e, s, k, mine, "would scuttle the ship")
}

// Plasma Coupler: half the energy left after the cost goes to the
// target; refused when that half is 0.
func TestPlasmaCoupler(t *testing.T) {
	d := aimer(PlasmaCoupler)
	k := part(d, PlasmaCoupler)
	_, e, s, _, _ := aimGame(t, d, hulk, 0.1)
	home := e.Colonies[0]
	mate := e.Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(mate, s.Pos)
	mine := Target{Kind: ShipTarget, Mine: mate}
	p := Gizmos[PlasmaCoupler].Power
	s.Power = p + 1
	useRefused(t, e, s, k, mine, "no energy to beam")
	s.Power, mate.Power = p+9, 0
	useAt(t, e, s, k, mine)
	if s.Power != 5 || mate.Power != 4 {
		t.Errorf("user %d target %d", s.Power, mate.Power)
	}
}

// Smart Bomb: every hostile ship in space within Special1 × RangeUnit
// takes shotDamage(Level, Absorb) (a kill reported); spared: one in
// orbit, one in a lane, one out of reach, an Invulnerable one (no
// damage), a mate, a ship of an empire at Peace. Refused with no hostile
// in reach; the device lost.
func TestSmartBomb(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	atWar(t, g, 0, 1)
	g.Contact(0, 2)
	e, e1 := g.Empires[0], g.Empires[1]
	home := e.Colonies[0]
	d := aimer(SmartBomb)
	k := part(d, SmartBomb)
	in := &Gizmos[SmartBomb]
	reach := float64(in.Special1) * RangeUnit
	spawn := func(o *Empire, fd Design, dx float64) *Starship {
		x := o.Spawn(fd, home.Star, home.PlanetIdx)
		x.Shields = false
		p := home.Planet.Pos
		p.X += dx
		inSpaceAt(x, p)
		return x
	}
	s := spawn(e, d, 0)
	mate := spawn(e, foeShip, 0.05)
	neutral := spawn(g.Empires[2], foeShip, 0.05)
	useRefused(t, e, s, k, Target{}, "no hostile ship in range")

	kill := spawn(e1, foeShip, 0.05)
	kill.Hull = in.Level
	shielded := spawn(e1, foeShip, -0.05)
	shielded.Shields = true
	orbiting := spawn(e1, foeShip, 0)
	orbiting.Orbit = home.PlanetIdx
	laning := spawn(e1, foeShip, 0.05)
	laning.Lane = 0
	far := spawn(e1, foeShip, reach+0.01)
	invul := spawn(e1, foeShip, 0.1)
	invul.Invulnerable = true
	hulls := map[*Starship]int{}
	for _, x := range []*Starship{mate, neutral, shielded, orbiting, laning, far, invul} {
		hulls[x] = x.Hull
	}
	reps := useAt(t, e, s, k, Target{})
	destroyedPair(t, g, reps, s, kill)
	if want := hulls[shielded] - shotDamage(in.Level, shielded.Absorb()); shielded.Hull != want || shielded.Absorb() == 0 {
		t.Errorf("shielded %d, want %d", shielded.Hull, want)
	}
	for _, x := range []*Starship{mate, neutral, orbiting, laning, far, invul} {
		if x.Hull != hulls[x] {
			t.Errorf("%v hit: %d of %d", x.Pos, x.Hull, hulls[x])
		}
	}
	if !strings.Contains(reps[0].Event.Text, "3 ships") || slices.Contains(s.Design.Parts, SmartBomb) {
		t.Errorf("report %q parts %v", reps[0].Event.Text, s.Design.Parts)
	}
}

// Self Destructotron: every other ship in space within Range × RangeUnit
// (a mate too) takes shotDamage(Level, Absorb), a kill reported and
// credited to the user (a mate's pair both to the user's empire, its
// ledger not counting its own blast); one in orbit and one out of range
// spared; the user destroyed.
func TestSelfDestructotron(t *testing.T) {
	d := aimer(SelfDestructotron)
	in := &Gizmos[SelfDestructotron]
	g, e, s, foe, _ := aimGame(t, d, foeShip, 0.1)
	foe.Hull = in.Level
	home := e.Colonies[0]
	mate := e.Spawn(Design{Name: "Mate", Hull: Large, Parts: []Gizmo{ProtonShaver}}, home.Star, home.PlanetIdx)
	inSpaceAt(mate, s.Pos)
	doomed := e.Spawn(foeShip, home.Star, home.PlanetIdx)
	inSpaceAt(doomed, s.Pos)
	doomed.Shields, doomed.Hull = false, in.Level
	orbiting := g.Empires[1].Spawn(foeShip, home.Star, home.PlanetIdx)
	far := g.Empires[1].Spawn(foeShip, home.Star, home.PlanetIdx)
	p := s.Pos
	p.X += float64(in.Range)*RangeUnit + 0.01
	inSpaceAt(far, p)
	hm, ho, hf := mate.Hull, orbiting.Hull, far.Hull
	name := s.Name
	reps := useAt(t, e, s, part(d, SelfDestructotron), Target{})
	if reps[0].Event.Text != name+" self-destructs" || g.OwnerOf(s) != nil {
		t.Fatalf("user: %q owner %v", reps[0].Event.Text, g.OwnerOf(s))
	}
	if len(reps) != 5 || reps[1].Empire != 0 || reps[2].Empire != 0 || reps[1].Event.Kind != Destroyed ||
		reps[2].Event.Kind != Destroyed || g.OwnerOf(doomed) != nil || e.Rel(0) != nil && e.Rel(0).ShipsLost != 0 {
		t.Fatalf("own kill: %+v", reps)
	}
	destroyedPair(t, g, append(reps[:1:1], reps[3:]...), s, foe)
	// Its Use lists the three ships gone, Empires then Ships order, the
	// user among them; nothing moved.
	if u := reps[0].Event.Use; !slices.Equal(u.Destroyed, []*Starship{s, doomed, foe}) || u.Moves != nil {
		t.Errorf("use's destroyed %v, moves %v", u.Destroyed, u.Moves)
	}
	if mate.Hull != hm-shotDamage(in.Level, mate.Absorb()) || mate.Hull == hm {
		t.Errorf("mate %d of %d", mate.Hull, hm)
	}
	if orbiting.Hull != ho || far.Hull != hf {
		t.Errorf("spared hit: orbit %d/%d far %d/%d", orbiting.Hull, ho, far.Hull, hf)
	}
}

// Phase Bomb at empire 1's home world: refused with only the Colony Base
// standing; the first structure in cell order demolished, the owner's
// Razed report, the device lost; out of range refused; an owner
// invincible today shrugs it off (spent, nothing razed).
func TestPhaseBomb(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e, e1 := g.Empires[0], g.Empires[1]
	home1 := e1.Colonies[0]
	p := home1.Planet
	d := aimer(PhaseBomb, PhaseBomb)
	s := e.Spawn(d, home1.Star, home1.PlanetIdx)
	inSpaceAt(s, p.Pos)
	k := part(d, PhaseBomb)
	tg := Target{Kind: PlanetTarget, Index: home1.PlanetIdx}
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item != ColonyBase {
			p.Demolish(c)
		}
	}
	useRefused(t, e, s, k, tg, "nothing to destroy")

	var free []Cell
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item == NoItem {
			free = append(free, c)
		}
	}
	first, second := free[0], free[len(free)-1]
	p.Surface[first].Item, p.Surface[second].Item = Factory, Laboratory

	s.Pos.X += float64(Gizmos[PhaseBomb].Range)*RangeUnit + 0.01
	useRefused(t, e, s, k, tg, "Phase Bomb: out of range")
	s.Pos = p.Pos

	reps := useAt(t, e, s, k, tg)
	if p.Surface[first].Item != NoItem || p.Surface[second].Item != Laboratory {
		t.Fatalf("surface: %v %v", p.Surface[first].Item, p.Surface[second].Item)
	}
	if len(reps) != 2 || reps[1].Empire != 1 || reps[1].Event.Kind != Razed || reps[1].Event.Item != Factory ||
		reps[1].Event.Star != home1.Star || reps[1].Event.Planet != home1.PlanetIdx || reps[1].Event.Other != -1 {
		t.Errorf("reps %+v", reps)
	}
	if n := len(s.Design.Parts); n != len(d.Parts)-1 {
		t.Errorf("parts %d", n)
	}

	e1.Invincible = e1.Day
	k = part(s.Design, PhaseBomb)
	power := s.Power
	reps = useAt(t, e, s, k, tg)
	if len(reps) != 1 || !strings.Contains(reps[0].Event.Text, "shrugged off") || p.Surface[second].Item != Laboratory {
		t.Errorf("invincible: %+v, %v", reps, p.Surface[second].Item)
	}
	if slices.Contains(s.Design.Parts, PhaseBomb) || s.Power != power-Gizmos[PhaseBomb].Power {
		t.Errorf("not spent: %v power %d", s.Design.Parts, s.Power)
	}
}

// The effects with a target and their own refusals are reached through
// UseSpecial only: every modelled ship-targeted special refuses an empty
// ShipTarget without a panic.
func TestEmptyShipTarget(t *testing.T) {
	for g, fx := range specialEffects {
		if fx.aim != aimHostile && fx.aim != aimAlly {
			continue
		}
		d := aimer(g)
		_, e, s, _, _ := aimGame(t, d, hulk, 0.1)
		useRefused(t, e, s, part(d, g), Target{Kind: ShipTarget}, "no such target")
	}
}
