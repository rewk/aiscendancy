package game

import (
	"slices"
	"strings"
	"testing"
)

// Planets in combat (orbit.go): the cascade, the targets, the planets'
// weapons, CombatStars.

// siegeGun is three Nanomanipulators on three Nanotwirlers (three
// generators, the energy for the three shots), no drive.
var siegeGun = Design{Name: "Siege", Hull: Medium,
	Parts: []Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanomanipulator, Nanomanipulator, Nanomanipulator}}

// orbitShip is a Small ship with an Ion Wrap (shields on: Absorb 1).
var orbitShip = Design{Name: "Orbiter", Hull: Small, Parts: []Gizmo{ProtonShaver, IonWrap}}

// siege is a two-empire game (Snovemdomas, Orfa): empire 1's siegeGun in
// space on empire 0's home planet's Pos, and the home colony; the two at
// War.
func siege(t *testing.T) (g *Game, sh *Starship, home *Colony) {
	t.Helper()
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	home = g.Empires[0].Colonies[0]
	sh = g.Empires[1].Spawn(siegeGun, home.Star, home.PlanetIdx)
	sh.Orbit = -1
	return g, sh, home
}

// volley refills sh, arms n of its guns at planet pl and plays a Round.
func volley(t *testing.T, g *Game, sh *Starship, pl, n int) ([]Report, RoundLog) {
	t.Helper()
	sh.Refill()
	e := g.OwnerOf(sh)
	for p, k := 0, 0; k < n; p++ {
		if Gizmos[sh.Design.Parts[p]].Cat != Weapon {
			continue
		}
		if err := e.ArmAtPlanet(sh, p, pl); err != nil {
			t.Fatalf("arm %d at %d: %v", p, pl, err)
		}
		k++
	}
	_, reps, log := g.Round(sh.Star, nil)
	return reps, log
}

func TestOrbitalDefencesTable(t *testing.T) {
	want := map[Item]OrbitalDefence{
		OrbitalCloaker:           {Strength: 15},
		OrbitalShield:            {Strength: 15},
		OrbitalMegaShield:        {Strength: 35},
		OrbitalMissileBase:       {Strength: 15, Damage: 7, Shots: 1, Range: 40},
		ShortRangeOrbitalWhopper: {Strength: 15, Damage: 7, Shots: 3, Range: 40},
		LongRangeOrbitalWhopper:  {Strength: 15, Damage: 8, Shots: 3, Range: 100},
	}
	for it := NoItem; it < NumItems; it++ {
		d, ok := it.Defence()
		if w, in := want[it]; d != w || ok != in {
			t.Errorf("%s: %+v %v, want %+v %v", it, d, ok, w, in)
		}
		if ok && d.Weapon() != (d.Damage > 0) || ok && d.Weapon() != it.Has(FlagWeapon) {
			t.Errorf("%s: Weapon %v, planitem flags %v", it, d.Weapon(), it.Info().Flags)
		}
	}
}

// The cascade: the shields in slot order (a plain one before a Mega in a
// later slot), each taking a shot's full damage until its Strength, the
// excess lost; then the weapons; then the ships in orbit (their shields
// absorb); a layer fallen to the round's pending damage passes the later
// shots on; the slot freed and its damage reset; the guard lifts with the
// last shield and ship; nothing left: refused.
func TestCascade(t *testing.T) {
	g, sh, home := siege(t)
	p, pl := home.Planet, home.PlanetIdx
	e0, e1 := g.Empires[0], g.Empires[1]
	for slot, it := range map[int]Item{1: OrbitalShield, 3: OrbitalMegaShield, 0: OrbitalMissileBase, 5: ShortRangeOrbitalWhopper} {
		if err := p.BuildOrbital(slot, it); err != nil {
			t.Fatal(err)
		}
	}
	guard := e0.Spawn(orbitShip, home.Star, pl)
	L := Gizmos[Nanomanipulator].Level
	s1, mega := OrbitalDefences[OrbitalShield].Strength, OrbitalDefences[OrbitalMegaShield].Strength
	mb, srw := OrbitalDefences[OrbitalMissileBase].Strength, OrbitalDefences[ShortRangeOrbitalWhopper].Strength
	if L >= s1 || 2*L < s1 || 2*L >= mega || 3*L < mega || L >= mb || 2*L < mb || 2*L < srw || guard.Absorb() < 1 {
		t.Fatalf("fixture: shot %d against %d/%d/%d/%d, absorb %d", L, s1, mega, mb, srw, guard.Absorb())
	}
	struck := func(log RoundLog, i, slot int, it Item) {
		t.Helper()
		f := log.Shots[i]
		if f.Shooter != sh || !f.AtPlanet || f.TargetPlanet != pl || f.Target != nil || f.HitSlot != slot || f.HitItem != it ||
			f.Damage != L || f.Absorbed != 0 || f.TargetEmpire != 0 || f.To != p.Pos || f.FromPlanet {
			t.Errorf("shot %d: %+v, want slot %d (%s) struck for %d", i, f, slot, it, L)
		}
	}

	// The plain shield (slot 1) first: one shot, then the second razes it,
	// its excess lost (the Mega untouched).
	_, log := volley(t, g, sh, pl, 1)
	struck(log, 0, 1, OrbitalShield)
	if p.OrbitDamage[1] != L || len(log.Razed) != 0 {
		t.Fatalf("one shot: damage %v razed %v", p.OrbitDamage, log.Razed)
	}
	reps, log := volley(t, g, sh, pl, 1)
	struck(log, 0, 1, OrbitalShield)
	if p.Orbit[1] != NoItem || p.OrbitDamage[1] != 0 || p.OrbitDamage[3] != 0 ||
		!slices.Equal(log.Razed, []Razing{{Planet: pl, Slot: 1, Item: OrbitalShield}}) {
		t.Fatalf("razed: orbit %v damage %v log %v", p.Orbit, p.OrbitDamage, log.Razed)
	}
	if len(reps) != 2 || reps[0].Empire != 1 || reps[1].Empire != 0 || reps[0].Event.Kind != Razed ||
		reps[0].Event.Item != OrbitalShield || reps[0].Event.Star != home.Star || reps[0].Event.Planet != pl ||
		reps[0].Event.Text != "Orbital Shield razed on "+p.Name ||
		reps[1].Event.Text != "Orbital Shield lost on "+p.Name {
		t.Errorf("reports %+v", reps)
	}
	if who, ok := g.Guard(home.Star, pl); !ok || who != 0 {
		t.Errorf("the Mega stands: guard %d %v", who, ok)
	}

	// The Mega (slot 3): three shots in one round, all on it (pending 2L
	// is short of its Strength), razed.
	_, log = volley(t, g, sh, pl, 3)
	for i := range 3 {
		struck(log, i, 3, OrbitalMegaShield)
	}
	if p.Orbit[3] != NoItem || p.OrbitDamage[0] != 0 {
		t.Fatalf("the Mega: orbit %v damage %v", p.Orbit, p.OrbitDamage)
	}
	if who, ok := g.Guard(home.Star, pl); !ok || who != 0 {
		t.Errorf("no shield, a ship in orbit: guard %d %v", who, ok)
	}

	// The weapons: the Missile Base (slot 0) falls to two shots, the third
	// passes on to the Whopper (slot 5).
	_, log = volley(t, g, sh, pl, 3)
	struck(log, 0, 0, OrbitalMissileBase)
	struck(log, 1, 0, OrbitalMissileBase)
	struck(log, 2, 5, ShortRangeOrbitalWhopper)
	if p.Orbit[0] != NoItem || p.OrbitDamage[5] != L {
		t.Fatalf("the weapons: orbit %v damage %v", p.Orbit, p.OrbitDamage)
	}

	// The Whopper falls to its second shot; the third reaches the ship in
	// orbit, which absorbs as from a ship.
	hull := guard.Hull
	_, log = volley(t, g, sh, pl, 2)
	struck(log, 0, 5, ShortRangeOrbitalWhopper)
	if f := log.Shots[1]; f.Target != guard || f.TargetEmpire != 0 || f.HitSlot != -1 || f.Damage != L-guard.Absorb() || f.Absorbed != guard.Absorb() {
		t.Errorf("the ship's shot: %+v", f)
	}
	if guard.Hull != hull-(L-guard.Absorb()) || p.Orbit[5] != NoItem {
		t.Fatalf("hull %d of %d, orbit %v", guard.Hull, hull, p.Orbit)
	}
	// Until it is destroyed: the guard lifts, nothing is left to hit.
	for i := 0; g.OwnerOf(guard) != nil; i++ {
		if i > 10 {
			t.Fatal("the ship in orbit never falls")
		}
		reps, log = volley(t, g, sh, pl, 1)
	}
	if len(reps) != 2 || reps[0].Event.Kind != Destroyed || !slices.Equal(log.Destroyed, []*Starship{guard}) {
		t.Errorf("the last shot: reports %+v destroyed %v", reps, log.Destroyed)
	}
	if who, ok := g.Guard(home.Star, pl); ok {
		t.Errorf("nothing in orbit: guard %d", who)
	}
	sh.Refill()
	n := len(sh.Armed)
	if err := e1.ArmAtPlanet(sh, part(siegeGun, Nanomanipulator), pl); err == nil || err.Error() != p.Name+": nothing to hit in orbit" || len(sh.Armed) != n {
		t.Errorf("an empty orbit: %v", err)
	}
}

// Targets: a ship in orbit only through its planet; a hostile colony and
// a planet another empire guards are; e's own, an unguarded uninhabited
// one, a planet with nothing in orbit are not; the shooter's own ships
// in orbit are skipped; range to the planet's Pos, checked by Round.
func TestPlanetTargets(t *testing.T) {
	g, sh, home := siege(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	p := part(siegeGun, Nanomanipulator)
	inOrbit := e0.Spawn(orbitShip, home.Star, home.PlanetIdx)
	sg, ok := g.SightingOf(e1, inOrbit)
	if !ok {
		t.Fatal("unseen")
	}
	want := "the target is in orbit of " + home.Planet.Name + ": fire at the planet"
	if err := e1.Arm(sh, p, sg); err == nil || err.Error() != want || len(sh.Armed) != 0 {
		t.Errorf("a ship in orbit: %v, want %q", err, want)
	}
	if err := e1.ArmAtPlanet(sh, p, home.PlanetIdx); err != nil {
		t.Errorf("a hostile colony with a ship in orbit: %v", err)
	}
	e1.Disarm(sh)

	// An uninhabited planet: refused unguarded, accepted guarded by
	// empire 0.
	free := -1
	for i, q := range g.Cluster.Stars[home.Star].System.Planets {
		if q.Owner < 0 {
			free = i
			break
		}
	}
	if free < 0 {
		t.Fatal("no uninhabited planet")
	}
	fp := g.Cluster.Stars[home.Star].System.Planets[free]
	if err := e1.ArmAtPlanet(sh, p, free); err == nil || err.Error() != fp.Name+": not hostile" {
		t.Errorf("unguarded: %v", err)
	}
	e0.Spawn(orbitShip, home.Star, free)
	if err := e1.ArmAtPlanet(sh, p, free); err != nil {
		t.Errorf("guarded by empire 0: %v", err)
	}
	e1.Disarm(sh)
	// Its own colony: only with a hostile ship in its orbit, which the
	// cascade hits past the colony's own structures.
	own := e1.Colonies[0]
	sh.Star, sh.Pos = own.Star, own.Planet.Pos // in range
	if err := e1.ArmAtPlanet(sh, p, own.PlanetIdx); err == nil || !strings.Contains(err.Error(), "not hostile") {
		t.Errorf("its own colony, nobody in orbit: %v", err)
	}
	foe := e0.Spawn(orbitShip, own.Star, own.PlanetIdx)
	own.Planet.Orbit[0] = OrbitalShield
	if err := e1.ArmAtPlanet(sh, p, own.PlanetIdx); err != nil {
		t.Errorf("its own colony, a foe in orbit: %v", err)
	}
	if hit := g.orbitTarget(e1, own.Star, own.PlanetIdx, nil, nil); hit.Ship != foe {
		t.Errorf("its own colony's cascade hits %+v, want the foe past the shield", hit)
	}
	own.Planet.Orbit[0] = NoItem
	e1.Disarm(sh)
	sh.Star = home.Star
	if err := e1.ArmAtPlanet(sh, p, len(g.Cluster.Stars[home.Star].System.Planets)); err == nil || err.Error() != "no such planet" {
		t.Errorf("no planet: %v", err)
	}
	// The shooter's refusals come first (Arm's).
	sh.Lane = 0
	if err := e1.ArmAtPlanet(sh, p, home.PlanetIdx); err == nil || err.Error() != "in a star lane" {
		t.Errorf("in a lane: %v", err)
	}
	sh.Lane = -1

	// The shooter's own ship in orbit of the foe's planet is skipped by
	// the cascade (empire 0 shooting at empire 1's home, its own ship
	// first in Empires order).
	g2 := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g2, 0, 1)
	h1 := g2.Empires[1].Colonies[0]
	gun := g2.Empires[0].Spawn(siegeGun, h1.Star, h1.PlanetIdx)
	gun.Orbit = -1
	mine := g2.Empires[0].Spawn(orbitShip, h1.Star, h1.PlanetIdx)
	theirs := g2.Empires[1].Spawn(orbitShip, h1.Star, h1.PlanetIdx)
	m0, t0 := mine.Hull, theirs.Hull
	_, log := volley(t, g2, gun, h1.PlanetIdx, 1)
	if len(log.Shots) != 1 || log.Shots[0].Target != theirs || mine.Hull != m0 || theirs.Hull >= t0 {
		t.Errorf("shots %+v: hull mine %d/%d theirs %d/%d", log.Shots, mine.Hull, m0, theirs.Hull, t0)
	}
	// A ship fallen to the round's pending damage passes the next shot on.
	if g2.OwnerOf(theirs) != nil {
		t.Fatal("the first ship in orbit survived: the next case proves nothing")
	}
	weak := g2.Empires[1].Spawn(orbitShip, h1.Star, h1.PlanetIdx)
	next := g2.Empires[1].Spawn(orbitShip, h1.Star, h1.PlanetIdx)
	weak.Hull = 1
	_, log = volley(t, g2, gun, h1.PlanetIdx, 2)
	if len(log.Shots) != 2 || log.Shots[0].Target != weak || log.Shots[1].Target != next ||
		!slices.Equal(log.Destroyed, []*Starship{weak, next}) {
		t.Errorf("pending: shots %+v destroyed %v", log.Shots, log.Destroyed)
	}

	// Out of range of the planet's Pos (Range × RangeUnit): refused at
	// arm time as CanFire does a ship (else the label
	// says "fire at" and Round drops the shot); armed anyway (the planet
	// moved off since), Round still drops it at no cost.
	w := Gizmos[Nanomanipulator]
	pp := home.Planet.Pos
	out := Vec3{X: pp.X + float64(w.Range)*RangeUnit + 0.01, Y: pp.Y, Z: pp.Z}
	inSpaceAt(sh, out)
	power := sh.Power
	if err := e1.ArmAtPlanet(sh, p, home.PlanetIdx); err == nil || err.Error() != w.Name+": out of range" {
		t.Errorf("out of range armed: %v", err)
	}
	if len(sh.Armed) != 0 {
		t.Fatalf("a refusal armed %+v", sh.Armed)
	}
	inSpaceAt(sh, pp)
	if err := e1.ArmAtPlanet(sh, p, home.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	sh.Pos = out
	if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != 0 || sh.Power != power || sh.Shots[p] != 0 || inOrbit.Hull != inOrbit.MaxHull() {
		t.Errorf("out of range: shots %v, energy %d of %d, uses %v", log.Shots, sh.Power, power, sh.Shots)
	}
	inSpaceAt(sh, Vec3{X: pp.X + float64(w.Range)*RangeUnit - 0.01, Y: pp.Y, Z: pp.Z})
	if err := e1.ArmAtPlanet(sh, p, home.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != 1 || sh.Shots[p] != 1 {
		t.Errorf("in range: shots %v", log.Shots)
	}
}

// A shot at a planet whose cascade fell to the round's earlier shots is
// given back with its energy and its generator: the shooter may move on
// them (shots before moves).
func TestGivenBackShotMoves(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	h1 := g.Empires[1].Colonies[0]
	sh := g.Empires[0].Spawn(twoGens, h1.Star, h1.PlanetIdx)
	inSpaceAt(sh, Vec3{X: h1.Planet.Pos.X + 0.1, Y: h1.Planet.Pos.Y, Z: h1.Planet.Pos.Z})
	sh.Shields = false
	weak := g.Empires[1].Spawn(orbitShip, h1.Star, h1.PlanetIdx)
	weak.Shields, weak.Hull = false, 1
	sh.Order = Order{Kind: GoToPoint, Pos: Vec3{X: SystemReach}}
	pos, power := sh.Pos, sh.Power
	w := Gizmos[MassBarrageGun]
	if power < 2*w.Power+twoGens.MoveCost() {
		t.Fatalf("fixture: energy %d", power)
	}
	_, log := volley(t, g, sh, h1.PlanetIdx, 2)
	if len(log.Shots) != 1 || !slices.Equal(log.Destroyed, []*Starship{weak}) || sh.Pos == pos ||
		sh.Power != power-w.Power-twoGens.MoveCost() {
		t.Errorf("shots %+v, destroyed %v, moved %v, energy %d of %d", log.Shots, log.Destroyed, sh.Pos != pos, sh.Power, power)
	}
}

// An armed ship shot whose target dodges into an orbit this round still
// hits (shots before moves: it was in space at the round's start); one
// at a target in an orbit at the round's start is dropped at no cost
// (hit through its planet only).
func TestShotDroppedIntoOrbit(t *testing.T) {
	g, sh, home := siege(t)
	e1 := g.Empires[1]
	runner := g.Empires[0].Spawn(Design{Name: "Runner", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}, home.Star, home.PlanetIdx)
	inSpaceAt(runner, home.Planet.Pos)
	sh.Pos = Vec3{X: home.Planet.Pos.X + 0.05, Y: home.Planet.Pos.Y, Z: home.Planet.Pos.Z}
	sg, _ := g.SightingOf(e1, runner)
	p := part(siegeGun, Nanomanipulator)
	if err := e1.Arm(sh, p, sg); err != nil {
		t.Fatal(err)
	}
	runner.Order = Order{Kind: GoToPlanet, Planet: home.PlanetIdx}
	w := Gizmos[Nanomanipulator]
	power := sh.Power
	if _, _, log := g.Round(sh.Star, nil); runner.Orbit != home.PlanetIdx || len(log.Shots) != 1 || sh.Power != power-w.Power ||
		runner.Hull != runner.MaxHull()-w.Level {
		t.Errorf("into orbit: orbit %d, shots %v, energy %d of %d, hull %d", runner.Orbit, log.Shots, sh.Power, power, runner.Hull)
	}
	// Armed in space, in an orbit by the round (staged by hand): dropped.
	inSpaceAt(runner, home.Planet.Pos)
	sh.Refill()
	sg, _ = g.SightingOf(e1, runner)
	if err := e1.Arm(sh, p, sg); err != nil {
		t.Fatal(err)
	}
	runner.Orbit = home.PlanetIdx
	power, hull := sh.Power, runner.Hull
	if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != 0 || sh.Power != power || runner.Hull != hull {
		t.Errorf("in orbit: shots %v, energy %d of %d, hull %d of %d", log.Shots, sh.Power, power, runner.Hull, hull)
	}
}

// A colony's orbital weapons: armed like ship parts (refusals), Shots a
// day per structure reset at the close, range from the planet's Pos, no
// energy; resolved with the ships' shots, a destroyed ship still firing;
// the log's planet shooter; the reports name the structure.
func TestPlanetShoots(t *testing.T) {
	g, foe, home := siege(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	p := home.Planet
	lr, mbSlot := 4, 6
	for slot, it := range map[int]Item{lr: LongRangeOrbitalWhopper, mbSlot: OrbitalMissileBase, 2: OrbitalShield} {
		if err := p.BuildOrbital(slot, it); err != nil {
			t.Fatal(err)
		}
	}
	d, mb := OrbitalDefences[LongRangeOrbitalWhopper], OrbitalDefences[OrbitalMissileBase]
	// The foe between the two ranges: the Whopper reaches it, not the base.
	dx := (float64(mb.Range)*RangeUnit + float64(d.Range)*RangeUnit) / 2
	inSpaceAt(foe, Vec3{X: p.Pos.X + dx, Y: p.Pos.Y, Z: p.Pos.Z})
	foe.Hull = 1000
	sg, _ := g.SightingOf(e0, foe)

	refuse := func(name string, e *Empire, c *Colony, slot int, t2 Sighting, want string) {
		t.Helper()
		n := len(c.Armed)
		if err := e.ArmPlanet(c, slot, t2); err == nil || !strings.Contains(err.Error(), want) || len(c.Armed) != n {
			t.Errorf("%s: %v, want %q", name, err, want)
		}
	}
	refuse("another empire's colony", e1, home, lr, sg, "not a colony of this empire")
	refuse("no slot", e0, home, OrbitSlots, sg, "no orbit slot")
	refuse("a shield", e0, home, 2, sg, "no orbital weapon")
	refuse("an empty slot", e0, home, 0, sg, "no orbital weapon")
	mate := e0.Spawn(orbitShip, home.Star, home.PlanetIdx)
	inSpaceAt(mate, Vec3{X: p.Pos.X - 0.1, Y: p.Pos.Y, Z: p.Pos.Z})
	refuse("its own ship", e0, home, lr, Sighting{Empire: 0, ship: mate}, "not hostile")
	inOrbit := e1.Spawn(orbitShip, home.Star, home.PlanetIdx)
	sgo, _ := g.SightingOf(e0, inOrbit)
	refuse("a ship in orbit", e0, home, lr, sgo, "is in orbit of "+p.Name+": fire at the planet")
	foe.Lane = 0
	refuse("a ship in a lane", e0, home, lr, Sighting{Empire: 1, ship: foe}, "star lane")
	foe.Lane = -1

	// Shots a day: one per round, then refused; the base out of range is
	// dropped (no use spent).
	hull := foe.Hull
	for r := range d.Shots {
		if err := e0.ArmPlanet(home, lr, sg); err != nil {
			t.Fatalf("round %d: %v", r, err)
		}
		refuse("armed twice", e0, home, lr, sg, "already armed this round")
		if err := e0.ArmPlanet(home, mbSlot, sg); err != nil {
			t.Fatalf("round %d, the base: %v", r, err)
		}
		_, _, log := g.Round(home.Star, nil)
		want := Fired{Target: foe, ShooterEmpire: 0, TargetEmpire: 1, From: p.Pos, To: foe.Pos,
			Damage: shotDamage(d.Damage, foe.Absorb()), Absorbed: foe.Absorb(),
			FromPlanet: true, ShooterPlanet: home.PlanetIdx, ShooterSlot: lr, ShooterItem: LongRangeOrbitalWhopper}
		if len(log.Shots) != 1 || log.Shots[0] != want {
			t.Fatalf("round %d: shots %+v, want %+v", r, log.Shots, want)
		}
		if home.OrbitShots[lr] != r+1 || home.OrbitShots[mbSlot] != 0 || len(home.Armed) != 0 {
			t.Errorf("round %d: shots %v armed %v", r, home.OrbitShots, home.Armed)
		}
	}
	if foe.Hull != hull-d.Shots*shotDamage(d.Damage, foe.Absorb()) {
		t.Errorf("hull %d of %d", foe.Hull, hull)
	}
	refuse("Shots spent", e0, home, lr, sg, "no shot left today")
	g.EndDay()
	if home.OrbitShots != [OrbitSlots]int{} {
		t.Errorf("after the close: %v", home.OrbitShots)
	}
	if err := e0.ArmPlanet(home, lr, sg); err != nil {
		t.Errorf("the next day: %v", err)
	}
	e0.DisarmPlanet(home)

	// Simultaneous with the ships: the planet destroys the foe while the
	// foe's shot at mate lands too; the reports name the structure.
	foe.Refill()
	foe.Hull = shotDamage(d.Damage, foe.Absorb())
	inSpaceAt(mate, Vec3{X: foe.Pos.X - 0.1, Y: foe.Pos.Y, Z: foe.Pos.Z}) // in the foe's gun's range
	gun := part(siegeGun, Nanomanipulator)
	sgm, _ := g.SightingOf(e1, mate)
	if err := e1.Arm(foe, gun, sgm); err != nil {
		t.Fatal(err)
	}
	if err := e0.ArmPlanet(home, lr, sg); err != nil {
		t.Fatal(err)
	}
	mh := mate.Hull
	_, reps, log := g.Round(home.Star, nil)
	name := p.Name
	if g.OwnerOf(foe) != nil || mate.Hull != mh-shotDamage(Gizmos[Nanomanipulator].Level, mate.Absorb()) || len(log.Shots) != 2 ||
		log.Shots[0].Shooter != foe || !log.Shots[1].FromPlanet {
		t.Errorf("foe owner %v, mate hull %d of %d, shots %+v", g.OwnerOf(foe), mate.Hull, mh, log.Shots)
	}
	at := " at " + g.Cluster.Stars[home.Star].Name // a planet keeps its name, a foreign ship never
	if len(reps) != 2 || reps[0].Empire != 0 || reps[0].Event.Text != name+" destroys "+aShip(g.Empires, 1)+at ||
		reps[1].Event.Text != foe.Name+" lost to "+name+at {
		t.Errorf("reports %+v", reps)
	}
}

// CombatStars with a planet: a colony with an orbital weapon faces any
// hostile ship; one with only a shield an armed one; none, nothing.
func TestCombatStarsPlanet(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	home := g.Empires[0].Colonies[0]
	unarmed := g.Empires[1].Spawn(orbitShip, home.Star, home.PlanetIdx)
	unarmed.Orbit = -1
	fights := func() bool { return slices.Contains(g.CombatStars(), home.Star) }
	if fights() {
		t.Fatal("no defence, an unarmed foe: a combat star")
	}
	if err := home.Planet.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	if fights() {
		t.Error("a shield, an unarmed foe: a combat star")
	}
	armed := g.Empires[1].Spawn(siegeGun, home.Star, home.PlanetIdx)
	armed.Lane = 0
	if fights() {
		t.Error("the armed foe in a lane: a combat star")
	}
	armed.Lane = -1
	if !fights() {
		t.Error("a shield, an armed foe: no combat star")
	}
	g.Destroy(armed)
	if err := home.Planet.BuildOrbital(1, OrbitalMissileBase); err != nil {
		t.Fatal(err)
	}
	if !fights() {
		t.Error("a missile base, an unarmed foe: no combat star")
	}
	if got := g.CombatStars(); !slices.IsSorted(got) || len(got) != 1 {
		t.Errorf("combat stars %v", got)
	}
}

// DefenceLeft: the owner reads Strength less OrbitDamage, nobody else.
func TestDefenceLeft(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	home := g.Empires[0].Colonies[0]
	p := home.Planet
	if err := p.BuildOrbital(3, OrbitalMegaShield); err != nil {
		t.Fatal(err)
	}
	p.OrbitDamage[3] = 10
	if left, ok := p.DefenceLeft(g.Empires[0], 3); !ok || left != OrbitalDefences[OrbitalMegaShield].Strength-10 {
		t.Errorf("owner: %d %v", left, ok)
	}
	if _, ok := p.DefenceLeft(g.Empires[1], 3); ok {
		t.Error("a foreign viewer reads the damage")
	}
	if _, ok := p.DefenceLeft(g.Empires[0], 4); ok {
		t.Error("an empty slot reads")
	}
	p.DemolishOrbital(3)
	if p.OrbitDamage[3] != 0 {
		t.Errorf("demolished: damage %d", p.OrbitDamage[3])
	}
}

// A structure is razed AT its Strength (damage + the shot = Strength).
func TestRazedAtStrength(t *testing.T) {
	g, sh, home := siege(t)
	p := home.Planet
	if err := p.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	p.OrbitDamage[0] = OrbitalDefences[OrbitalShield].Strength - Gizmos[Nanomanipulator].Level
	if _, log := volley(t, g, sh, home.PlanetIdx, 1); p.Orbit[0] != NoItem || len(log.Razed) != 1 {
		t.Errorf("at its Strength: orbit %v, razed %v", p.Orbit, log.Razed)
	}
}

// A ship in orbit does not fire (the original's: it
// must leave orbit to fight): Arm and ArmAtPlanet refuse it, and a shot
// armed before the round's move into an orbit is dropped at no cost.
func TestNoFireFromOrbit(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e0, e1 := g.Empires[0], g.Empires[1]
	home := e0.Colonies[0]
	pp := home.Planet.Pos
	sh := e0.Spawn(twoGens, home.Star, home.PlanetIdx)
	foe := e1.Spawn(hulk, home.Star, home.PlanetIdx)
	inSpaceAt(foe, Vec3{X: pp.X + 0.2, Y: pp.Y, Z: pp.Z})
	foe.Shields = false
	gun := part(twoGens, MassBarrageGun)
	sg, _ := g.SightingOf(e0, foe)
	want := sh.Name + " is in orbit of " + home.Planet.Name + ": leave orbit to fire"
	if err := e0.Arm(sh, gun, sg); err == nil || err.Error() != want || len(sh.Armed) != 0 {
		t.Errorf("Arm in orbit: %v, want %q", err, want)
	}
	e1.Spawn(hulk, home.Star, home.PlanetIdx) // something to hit in orbit
	if err := e0.ArmAtPlanet(sh, gun, home.PlanetIdx); err == nil || err.Error() != want {
		t.Errorf("ArmAtPlanet in orbit: %v, want %q", err, want)
	}
	// Armed in space, then into the orbit in the round's moves: it fires
	// first (shots before moves).
	g.Empires[1].Ships = g.Empires[1].Ships[:1]
	inSpaceAt(sh, Vec3{X: pp.X + 0.1, Y: pp.Y, Z: pp.Z})
	sh.Shields = false
	sh.Refill()
	if err := e0.Arm(sh, gun, sg); err != nil {
		t.Fatal(err)
	}
	if err := e0.Order(sh, Order{Kind: GoToPlanet, Planet: home.PlanetIdx}); err != nil {
		t.Fatal(err)
	}
	w := Gizmos[MassBarrageGun]
	power := sh.Power
	if _, _, log := g.Round(home.Star, nil); sh.Orbit != home.PlanetIdx || len(log.Shots) != 1 || sh.Shots[gun] != 1 ||
		sh.Power != power-w.Power-twoGens.MoveCost() || foe.Hull != foe.MaxHull()-w.Level {
		t.Errorf("orbit %d, shots %v, uses %v, energy %d of %d-%d-%d, foe hull %d", sh.Orbit, log.Shots, sh.Shots, sh.Power, power, w.Power, twoGens.MoveCost(), foe.Hull)
	}
	// Armed in space, in an orbit by the round (staged by hand): dropped.
	inSpaceAt(sh, Vec3{X: pp.X + 0.1, Y: pp.Y, Z: pp.Z})
	clear(sh.Shots)
	if err := e0.Arm(sh, gun, sg); err != nil {
		t.Fatal(err)
	}
	sh.Orbit = home.PlanetIdx
	power, hull := sh.Power, foe.Hull
	if _, _, log := g.Round(home.Star, nil); len(log.Shots) != 0 || sh.Shots[gun] != 0 || sh.Power != power || foe.Hull != hull {
		t.Errorf("from orbit: shots %v, uses %v, energy %d of %d, foe hull %d of %d", log.Shots, sh.Shots, sh.Power, power, foe.Hull, hull)
	}
}

// A colony whose only reason to fight is its gun does not face an
// unarmed hostile ship in orbit (a planet does not fire at a ship in
// orbit, and that ship cannot fire): no combat star, so the ships there
// Step at the close instead of waiting on rounds where nothing can act.
// Out of the orbit, the same ship is a target again.
func TestCombatStarsGunIgnoresOrbit(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	home := g.Empires[0].Colonies[0]
	if err := home.Planet.BuildOrbital(0, OrbitalMissileBase); err != nil {
		t.Fatal(err)
	}
	unarmed := g.Empires[1].Spawn(orbitShip, home.Star, home.PlanetIdx) // in orbit
	if slices.Contains(g.CombatStars(), home.Star) {
		t.Error("a missile base, an unarmed foe in orbit: a combat star")
	}
	unarmed.Orbit = -1
	if !slices.Contains(g.CombatStars(), home.Star) {
		t.Error("a missile base, an unarmed foe in space: no combat star")
	}
}

// raze on an unowned planet (a lost colony's orbitals left standing)
// reports to the shooter only: no Empires[-1].
func TestRazeUnowned(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e := g.Empires[0]
	st := e.Colonies[0].Star
	var pl int = -1
	for i, p := range g.Cluster.Stars[st].System.Planets {
		if p.Owner < 0 {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Skip("no unowned planet at the home star")
	}
	p := g.Cluster.Stars[st].System.Planets[pl]
	p.Orbit[0] = OrbitalShield
	reps := g.raze(e, p.Owner, st, pl, 0)
	if len(reps) != 1 || reps[0].Empire != e.Index || p.Orbit[0] != NoItem {
		t.Errorf("reports %+v, slot %v", reps, p.Orbit[0])
	}
}

// p2pFixture is siege's game with empire 1 settled on the first free
// planet of empire 0's home star, moved off the home planet by half
// between the Missile Base's range and the Long Range Whopper's (the
// Whopper reaches it, not the base); the Whopper in home slot lr, the
// base in mb. siege's ship is left out (Ships emptied).
func p2pFixture(t *testing.T) (g *Game, home, foe *Colony, lr, mb int) {
	t.Helper()
	g, _, home = siege(t)
	g.Empires[1].Ships = nil
	pl := slices.IndexFunc(g.Cluster.Stars[home.Star].System.Planets, func(p *Planet) bool { return p.Owner < 0 })
	if pl < 0 {
		t.Fatal("fixture: no free planet at the home star")
	}
	foe = settleAt(t, g, 1, home.Star, pl)
	d, b := OrbitalDefences[LongRangeOrbitalWhopper], OrbitalDefences[OrbitalMissileBase]
	dx := (float64(b.Range)*RangeUnit + float64(d.Range)*RangeUnit) / 2
	hp := home.Planet.Pos
	foe.Planet.Pos = Vec3{X: hp.X + dx, Y: hp.Y, Z: hp.Z}
	lr, mb = 4, 6
	for slot, it := range map[int]Item{lr: LongRangeOrbitalWhopper, mb: OrbitalMissileBase, 2: OrbitalShield} {
		if err := home.Planet.BuildOrbital(slot, it); err != nil {
			t.Fatal(err)
		}
	}
	return g, home, foe, lr, mb
}

// ArmPlanetAt's refusals, one by one in its order (ArmPlanet's own, the
// planet, CanHitPlanet's verdict, the range), nothing armed on any; then
// armed, the AtPlanet Shot in col.Armed.
func TestArmPlanetAt(t *testing.T) {
	g, home, foe, lr, mb := p2pFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	if err := foe.Planet.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	refuse := func(name string, e *Empire, c *Colony, slot, pl int, want string) {
		t.Helper()
		n := len(c.Armed)
		if err := e.CanArmPlanetAt(c, slot, pl); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: CanArmPlanetAt %v, want %q", name, err, want)
		}
		if err := e.ArmPlanetAt(c, slot, pl); err == nil || !strings.Contains(err.Error(), want) || len(c.Armed) != n {
			t.Errorf("%s: %v, want %q", name, err, want)
		}
	}
	np := len(g.Cluster.Stars[home.Star].System.Planets)
	refuse("another empire's colony", e1, home, lr, foe.PlanetIdx, "not a colony of this empire")
	refuse("no slot", e0, home, OrbitSlots, foe.PlanetIdx, "no orbit slot")
	refuse("a shield", e0, home, 2, foe.PlanetIdx, "no orbital weapon")
	refuse("an empty slot", e0, home, 0, foe.PlanetIdx, "no orbital weapon")
	refuse("no such planet", e0, home, lr, np, "no such planet")
	refuse("a negative planet", e0, home, lr, -1, "no such planet")
	refuse("itself", e0, home, lr, home.PlanetIdx, home.Planet.Name+" cannot fire at itself")
	free := slices.IndexFunc(g.Cluster.Stars[home.Star].System.Planets, func(p *Planet) bool { return p.Owner < 0 })
	if free >= 0 {
		refuse("a free planet, nothing in orbit", e0, home, lr, free, "not hostile")
	}
	refuse("out of range", e0, home, mb, foe.PlanetIdx, OrbitalMissileBase.String()+": out of range")
	home.Tractors = 1
	refuse("the beam used", e0, home, lr, foe.PlanetIdx, "Tractor Beam used this round")
	home.Tractors = 0

	if err := e0.CanArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	if err := e0.ArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	if want := []Shot{{Part: lr, AtPlanet: true, Planet: foe.PlanetIdx}}; !slices.Equal(home.Armed, want) {
		t.Errorf("armed %+v, want %+v", home.Armed, want)
	}
	refuse("armed twice", e0, home, lr, foe.PlanetIdx, "already armed this round")
	home.Armed = nil
	home.OrbitShots[lr] = OrbitalDefences[LongRangeOrbitalWhopper].Shots
	refuse("Shots spent", e0, home, lr, foe.PlanetIdx, "no shot left today")
}

// A planet's shot at a planet runs the cascade as a ship's: the
// Whopper's shots on the hostile planet's orbital shield until razed
// (Strength over Damage, the table's), then on the ship in its orbit
// (its Absorb off), one OrbitShots each; the log's planet shooter and
// planet target; the razing reported to both sides.
func TestPlanetShootsPlanet(t *testing.T) {
	g, home, foe, lr, _ := p2pFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	p, fp := home.Planet, foe.Planet
	if err := fp.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	guard := e1.Spawn(orbitShip, foe.Star, foe.PlanetIdx) // in orbit
	guard.Pos = fp.Pos
	d, sd := OrbitalDefences[LongRangeOrbitalWhopper], OrbitalDefences[OrbitalShield]
	raze := (sd.Strength + d.Damage - 1) / d.Damage // shots to raze the shield
	if raze+1 > d.Shots {
		t.Fatalf("fixture: %d shots to raze, %d a day", raze, d.Shots)
	}
	for r := range raze {
		if err := e0.ArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
			t.Fatalf("round %d: %v", r, err)
		}
		_, reps, log := g.Round(home.Star, nil)
		want := Fired{ShooterEmpire: 0, TargetEmpire: 1, From: p.Pos, To: fp.Pos, Damage: d.Damage,
			FromPlanet: true, ShooterPlanet: home.PlanetIdx, ShooterSlot: lr, ShooterItem: LongRangeOrbitalWhopper,
			AtPlanet: true, TargetPlanet: foe.PlanetIdx, HitSlot: 0, HitItem: OrbitalShield}
		if len(log.Shots) != 1 || log.Shots[0] != want {
			t.Fatalf("round %d: shots %+v, want %+v", r, log.Shots, want)
		}
		if home.OrbitShots[lr] != r+1 || len(home.Armed) != 0 {
			t.Errorf("round %d: shots %v armed %v", r, home.OrbitShots, home.Armed)
		}
		if r < raze-1 {
			if fp.OrbitDamage[0] != (r+1)*d.Damage || len(log.Razed) != 0 {
				t.Errorf("round %d: damage %v razed %v", r, fp.OrbitDamage, log.Razed)
			}
			continue
		}
		if fp.Orbit[0] != NoItem || !slices.Equal(log.Razed, []Razing{{Planet: foe.PlanetIdx, Slot: 0, Item: OrbitalShield}}) {
			t.Fatalf("razing round: orbit %v razed %v", fp.Orbit, log.Razed)
		}
		if len(reps) != 2 || reps[0].Empire != 0 || reps[1].Empire != 1 || reps[0].Event.Kind != Razed {
			t.Errorf("razing reports %+v", reps)
		}
	}
	if err := e0.ArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	hull, absorb := guard.Hull, guard.Absorb()
	_, _, log := g.Round(home.Star, nil)
	dm := shotDamage(d.Damage, absorb)
	want := Fired{Target: guard, ShooterEmpire: 0, TargetEmpire: 1, From: p.Pos, To: guard.Pos, Damage: dm, Absorbed: d.Damage - dm,
		FromPlanet: true, ShooterPlanet: home.PlanetIdx, ShooterSlot: lr, ShooterItem: LongRangeOrbitalWhopper,
		AtPlanet: true, TargetPlanet: foe.PlanetIdx, HitSlot: -1, HitItem: NoItem}
	if len(log.Shots) != 1 || log.Shots[0] != want || guard.Hull != hull-dm || home.OrbitShots[lr] != raze+1 {
		t.Errorf("at the guard: shots %+v, want %+v; hull %d of %d, shots %v", log.Shots, want, guard.Hull, hull, home.OrbitShots)
	}
}

// Round re-checks a planet shot's range (the target moved off after the
// arm: dropped, no shot spent), and drops one whose cascade has nothing
// left at no cost.
func TestPlanetShotDropped(t *testing.T) {
	g, home, foe, lr, _ := p2pFixture(t)
	e0 := g.Empires[0]
	fp := foe.Planet
	if err := fp.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	if err := e0.ArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	at := fp.Pos
	fp.Pos.X += float64(OrbitalDefences[LongRangeOrbitalWhopper].Range) * RangeUnit
	if _, _, log := g.Round(home.Star, nil); len(log.Shots) != 0 || home.OrbitShots[lr] != 0 || fp.OrbitDamage[0] != 0 {
		t.Errorf("out of range: shots %+v, uses %v, damage %v", log.Shots, home.OrbitShots, fp.OrbitDamage)
	}
	fp.Pos = at
	if err := e0.ArmPlanetAt(home, lr, foe.PlanetIdx); err != nil {
		t.Fatal(err)
	}
	fp.DemolishOrbital(0) // gone before the round: nothing left to strike
	if _, _, log := g.Round(home.Star, nil); len(log.Shots) != 0 || home.OrbitShots[lr] != 0 {
		t.Errorf("nothing to strike: shots %+v, uses %v", log.Shots, home.OrbitShots)
	}
}

// A neutral planet whose orbit an Orbital Cloaker hides is judged from
// what the shooter sees (CanHitPlanet): not hostile through its owner,
// refused though a hostile ship sits in its orbit; scanned, a target.
func TestArmPlanetAtCloaked(t *testing.T) {
	g, home, _, lr, _ := p2pFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	ps := g.Cluster.Stars[home.Star].System.Planets
	pl := slices.IndexFunc(ps, func(p *Planet) bool { return p.Owner < 0 })
	if pl < 0 {
		t.Skip("no second free planet at the home star")
	}
	p := ps[pl]
	p.Pos = home.Planet.Pos
	p.Pos.X += 0.1
	p.Orbit[0] = OrbitalCloaker
	e1.Spawn(orbitShip, home.Star, pl).Pos = p.Pos // a hostile ship in orbit
	want := g.CanHitPlanet(e0, home.Star, pl)
	if want == nil || e0.SeesOrbit(p) {
		t.Fatalf("fixture: CanHitPlanet %v, sees %v", want, e0.SeesOrbit(p))
	}
	if err := e0.ArmPlanetAt(home, lr, pl); err == nil || err.Error() != want.Error() || len(home.Armed) != 0 {
		t.Errorf("cloaked neutral: %v, want %v", err, want)
	}
	e0.ScanAll = true
	if err := e0.ArmPlanetAt(home, lr, pl); err != nil {
		t.Errorf("seen through: %v", err)
	}
}

// Two hostile colonies alone at a star fight when a gun reaches a planet
// it may hit (planetFoe): no ship anywhere; none out of range, nor with
// nothing to hit there.
func TestPlanetsStartCombat(t *testing.T) {
	g, home, foe, _, _ := p2pFixture(t)
	g.Empires[0].Ships, g.Empires[1].Ships = nil, nil
	if slices.Contains(g.CombatStars(), home.Star) {
		t.Fatal("a fight with nothing to hit at the foe's planet")
	}
	if err := foe.Planet.BuildOrbital(0, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(g.CombatStars(), home.Star) {
		t.Error("no fight: the Long Range Whopper reaches the foe's shield")
	}
	hp := home.Planet.Pos
	far := float64(OrbitalDefences[LongRangeOrbitalWhopper].Range)*RangeUnit + 0.01
	foe.Planet.Pos = Vec3{X: hp.X + far, Y: hp.Y, Z: hp.Z}
	if slices.Contains(g.CombatStars(), home.Star) {
		t.Error("a fight out of every gun's range")
	}
}

// A gun that may hit a hostile planet whose orbit an Orbital Cloaker
// hides engages its owner (defendedFoes through gunHits): before, the
// star fought (planetFoe) with the shooter out of it, unable to play a
// round, its ships held off the close's Step. The news names the
// target's owner's planets.
func TestEngagedBlindGun(t *testing.T) {
	g, home, foe, _, _ := p2pFixture(t)
	e0, e1 := g.Empires[0], g.Empires[1]
	e0.Ships = nil
	foe.Planet.Orbit[0] = OrbitalCloaker
	if e0.SeesOrbit(foe.Planet) || g.CanHitPlanet(e0, home.Star, foe.PlanetIdx) != nil {
		t.Fatalf("fixture: sees %v, CanHitPlanet %v", e0.SeesOrbit(foe.Planet), g.CanHitPlanet(e0, home.Star, foe.PlanetIdx))
	}
	if !slices.Contains(g.CombatStars(), home.Star) || !g.Engaged(e1, home.Star) {
		t.Fatalf("fixture: combat %v, the target's owner engaged %v", g.CombatStars(), g.Engaged(e1, home.Star))
	}
	if !g.Engaged(e0, home.Star) {
		t.Error("the gun's owner not engaged at the fight its gun starts")
	}
	reps := g.opened(nil)
	if !slices.ContainsFunc(reps, func(r Report) bool {
		return r.Empire == 0 && r.Event.Kind == Engaged && r.Event.Other == 1 && strings.HasSuffix(r.Event.Text, " planets")
	}) {
		t.Errorf("no planets news for the gun's owner: %+v", reps)
	}
	if _, _, _, err := g.StepRound(e0, home.Star, nil); err != nil {
		t.Errorf("StepRound: %v", err)
	}
}
