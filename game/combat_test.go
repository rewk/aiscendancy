package game

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// fightFixture is a Small warship: the brief's Proton Shaver, Tonklin
// Motor, Mass Barrage Gun and Ion Wrap draw 3 for a supply of 2, so a
// second Proton Shaver fills the fifth slot.
var fightFixture = Design{Name: "Fixture", Hull: Small,
	Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, MassBarrageGun, IonWrap}}

// partTechs knows the hull's and every part's research, nothing else.
func partTechs(d Design) *Research {
	ts := []Tech{d.Hull.Tech()}
	for _, g := range d.Parts {
		ts = append(ts, Gizmos[g].ResearchReq)
	}
	return knowing(ts...)
}

// duel is a two-empire game (Snovemdomas, Orfa): sh of empire 0 armed
// with the design a, t of empire 1 of the design b, both in space on
// empire 0's home planet's Pos (out of orbit: a ship in orbit is hit
// through its planet only), the two at War.
func duel(a, b Design) (g *Game, sh, t *Starship) {
	g = NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	if err := war(g, 0, 1); err != nil {
		panic(err)
	}
	home := g.Empires[0].Colonies[0]
	sh = g.Empires[0].Spawn(a, home.Star, home.PlanetIdx)
	t = g.Empires[1].Spawn(b, home.Star, home.PlanetIdx)
	sh.Orbit, t.Orbit = -1, -1
	return g, sh, t
}

// part is the index of g in d's parts.
func part(d Design, g Gizmo) int { return slices.Index(d.Parts, g) }

// fire arms sh's part at tg (as sh's empire sees it) and plays one Round
// at sh's star: the damage is tg's hull lost, err Arm's refusal.
func fire(g *Game, sh *Starship, p int, tg *Starship) (dmg int, reps []Report, err error) {
	e := g.OwnerOf(sh)
	sg, ok := g.SightingOf(e, tg)
	if !ok {
		sg = Sighting{Empire: e.Index, ship: tg} // unseen: Arm says why
	}
	if err := e.Arm(sh, p, sg); err != nil {
		return 0, nil, err
	}
	hull := tg.Hull
	_, reps, _ = g.Round(sh.Star, nil)
	return hull - tg.Hull, reps, nil
}

func TestFightFixtureValid(t *testing.T) {
	brief := Design{Name: "Brief", Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun, IonWrap}}
	if err := brief.Valid(partTechs(brief)); err == nil {
		t.Error("the brief's fixture is valid: the second Proton Shaver is not needed")
	}
	if err := fightFixture.Valid(partTechs(fightFixture)); err != nil {
		t.Fatal(err)
	}
}

// The guide's hull strengths; a Snovemdomas ship has double (SA_DoubleHull).
func TestHullStrength(t *testing.T) {
	if HullStrength != [NumHulls]int{10, 20, 30, 40} {
		t.Errorf("HullStrength %v", HullStrength)
	}
	_, sn, or := duel(fightFixture, fightFixture)
	if or.Hull != Small.Strength() || or.MaxHull() != or.Hull || or.Hull != 10 {
		t.Errorf("Orfa Small ship: hull %d/%d", or.Hull, or.MaxHull())
	}
	if sn.Hull != 2*Small.Strength() || sn.MaxHull() != sn.Hull || sn.Hull != 20 {
		t.Errorf("Snovemdomas Small ship: hull %d/%d", sn.Hull, sn.MaxHull())
	}
	if !sn.Shields || len(sn.Shots) != len(sn.Design.Parts) {
		t.Errorf("spawned with shields %v, %d shot counters for %d parts", sn.Shields, len(sn.Shots), len(sn.Design.Parts))
	}
}

// A shot does the weapon's Level less the target's shields while on.
func TestFireDamage(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	gun := part(fightFixture, MassBarrageGun)
	_ = tg.SetShields(false)
	hull, power := tg.Hull, sh.Power
	dmg, reps, err := fire(g, sh, gun, tg)
	if want := Gizmos[MassBarrageGun].Level; err != nil || len(reps) != 0 || dmg != want || dmg != 1 || tg.Hull != hull-want {
		t.Errorf("unshielded: damage %d, reports %v, err %v, hull %d of %d", dmg, reps, err, tg.Hull, hull)
	}
	if sh.Power != power-Gizmos[MassBarrageGun].Power || sh.Shots[gun] != 1 { // no draw: reserved already
		t.Errorf("power %d of %d, shots %v", sh.Power, power, sh.Shots)
	}

	g, sh, tg = duel(fightFixture, fightFixture)
	hull = tg.Hull
	dmg, _, err = fire(g, sh, gun, tg)
	if want := max(0, Gizmos[MassBarrageGun].Level-Gizmos[IonWrap].Level); err != nil || dmg != want || dmg != 0 || tg.Hull != hull {
		t.Errorf("behind an Ion Wrap: damage %d, err %v, hull %d of %d", dmg, err, tg.Hull, hull)
	}
}

// The fan manual's examples: 3 Ion Wraps against a Plasmatron take 1,
// 2 Hyperwave Nullifiers against a Hypersphere Driver take 2. The
// target is given the energy for its shields' draw (a Nanotwirler cannot
// carry two Nullifiers and a hit short of it knocks them down instead):
// the hit spends it.
func TestShieldExamples(t *testing.T) {
	for _, c := range []struct {
		weapon, shield Gizmo
		shields, want  int
	}{{Plasmatron, IonWrap, 3, 1}, {HypersphereDriver, HyperwaveNullifier, 2, 2}} {
		a := Design{Hull: Small, Parts: []Gizmo{QuarkExpress, TonklinMotor, c.weapon}}
		b := Design{Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor}}
		for range c.shields {
			b.Parts = append(b.Parts, c.shield)
		}
		g, sh, tg := duel(a, b)
		if !tg.Shields || tg.Absorb() != c.shields*Gizmos[c.shield].Level {
			t.Fatalf("%d %s: shields %v, absorb %d", c.shields, c.shield, tg.Shields, tg.Absorb())
		}
		hull, draw := tg.Hull, b.ShieldDraw()
		tg.Power = draw
		dmg, _, err := fire(g, sh, part(a, c.weapon), tg)
		if want := Gizmos[c.weapon].Level - tg.Absorb(); err != nil || dmg != want || dmg != c.want || tg.Hull != hull-dmg || tg.Power != 0 || tg.ShieldsDown {
			t.Errorf("%s vs %d %s: damage %d (err %v), want %d; energy %d of %d, down %v", c.weapon, c.shields, c.shield, dmg, err, c.want, tg.Power, draw, tg.ShieldsDown)
		}
	}
}

// The Ueberlaser fires its Uses a day (one a round), then refuses until
// Refill.
func TestShotsPerDay(t *testing.T) {
	a := Design{Hull: Small, Parts: []Gizmo{QuarkExpress, TonklinMotor, Ueberlaser}}
	g, sh, tg := duel(a, fightFixture)
	laser, uses := part(a, Ueberlaser), Gizmos[Ueberlaser].Uses
	if uses != 2 || sh.Power < uses*Gizmos[Ueberlaser].Power {
		t.Fatalf("uses %d, power %d", uses, sh.Power)
	}
	for i := range uses {
		if _, _, err := fire(g, sh, laser, tg); err != nil || sh.Shots[laser] != i+1 {
			t.Fatalf("shot %d: %v, shots %v", i+1, err, sh.Shots)
		}
	}
	if _, _, err := fire(g, sh, laser, tg); err == nil || !strings.Contains(err.Error(), "no shot left today") {
		t.Errorf("shot %d: %v", uses+1, err)
	}
	// An ETA on the ship steps a copy: its Refill must not reset the ship's shots.
	sh.Order = Order{Kind: GoToPlanet, Planet: (sh.Orbit + 1) % len(g.Cluster.Stars[sh.Star].System.Planets)}
	sh.ETA(g.Cluster, g.Day)
	if sh.Shots[laser] != uses {
		t.Errorf("ETA reset the shots: %v", sh.Shots)
	}
	sh.Refill()
	if sh.Shots[laser] != 0 || sh.CanFire(g.Cluster, laser, tg) != nil {
		t.Errorf("after Refill: shots %v, %v", sh.Shots, sh.CanFire(g.Cluster, laser, tg))
	}
}

// CanFire's refusals (the window's aim line, the bot's choice of target),
// from the table's Range and Power, the shields' reservation already off
// Power.
func TestCanFire(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	gun, w := part(fightFixture, MassBarrageGun), Gizmos[MassBarrageGun]
	edge := float64(w.Range) * RangeUnit
	if math.Abs(edge-0.25) > 1e-12 { // the unit: 25 is a quarter of the disc radius
		t.Fatalf("Mass Barrage Gun reach %v, want 0.25", edge)
	}
	refuse := func(name string, p int, target *Starship, want string) {
		t.Helper()
		if err := sh.CanFire(g.Cluster, p, target); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", name, err, want)
		}
	}
	refuse("not a weapon", part(fightFixture, IonWrap), tg, "not a weapon")
	refuse("no part", len(fightFixture.Parts), tg, "no part")
	refuse("itself", gun, sh, "itself")

	tg.Pos = Vec3{sh.Pos.X + edge, sh.Pos.Y, sh.Pos.Z}
	if err := sh.CanFire(g.Cluster, gun, tg); err != nil {
		t.Errorf("at the Range edge %.2f: %v", edge, err)
	}
	tg.Pos.X += 1e-6
	refuse("beyond Range", gun, tg, "out of range")
	tg.Pos = sh.Pos

	tg.Lane = 0
	refuse("target in a lane", gun, tg, "lane")
	tg.Lane = -1
	tg.Star = (sh.Star + 1) % len(g.Cluster.Stars)
	refuse("another system", gun, tg, "same system")
	tg.Star = sh.Star

	if !sh.Shields || fightFixture.ShieldDraw() == 0 {
		t.Fatal("the fixture's shields are not on, or draw nothing")
	}
	sh.Power = w.Power - 1
	refuse("power", gun, tg, "not enough power")
	sh.Power = w.Power // the reservation is not counted again
	if err := sh.CanFire(g.Cluster, gun, tg); err != nil {
		t.Errorf("shields on, exactly the weapon's power: %v", err)
	}
}

// The shields reserve their draw (measured in the original): SetShields
// on takes ShieldDraw off Power at once, refused when short, off gives it
// back in full; Refill takes it again while on, and switches off shields
// the day cannot carry.
func TestShieldDraw(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, HyperwaveNullifier, HyperwaveNullifier}}
	_, sh, _ := duel(d, fightFixture)
	draw := 2 * Gizmos[HyperwaveNullifier].Power
	if d.ShieldDraw() != draw || d.ShieldScore() != 2*Gizmos[HyperwaveNullifier].Level || draw == 0 || draw > d.PowerPerDay() {
		t.Fatalf("draw %d score %d supply %d", d.ShieldDraw(), d.ShieldScore(), d.PowerPerDay())
	}
	if !sh.Shields || sh.Power != d.PowerPerDay()-draw || sh.Absorb() != d.ShieldScore() {
		t.Errorf("spawned, shields on: power %d, want %d; absorb %d", sh.Power, d.PowerPerDay()-draw, sh.Absorb())
	}
	if err := sh.SetShields(true); err != nil || sh.Power != d.PowerPerDay()-draw {
		t.Errorf("on again: %v, power %d (taken twice?)", err, sh.Power)
	}
	if err := sh.SetShields(false); err != nil || sh.Shields || sh.Power != d.PowerPerDay() || sh.Absorb() != 0 {
		t.Errorf("off: %v, shields %v, power %d, want %d (refunded), absorb %d", err, sh.Shields, sh.Power, d.PowerPerDay(), sh.Absorb())
	}
	if err := sh.SetShields(false); err != nil || sh.Power != d.PowerPerDay() {
		t.Errorf("off again: %v, power %d (refunded twice?)", err, sh.Power)
	}
	sh.Refill()
	if sh.Shields || sh.Power != d.PowerPerDay() {
		t.Errorf("refilled off: shields %v, power %d", sh.Shields, sh.Power)
	}
	sh.Power = draw - 1
	if err := sh.SetShields(true); err == nil || sh.Shields || sh.Power != draw-1 {
		t.Errorf("on short of the draw: %v, shields %v, power %d", err, sh.Shields, sh.Power)
	}
	sh.Power = draw
	if err := sh.SetShields(true); err != nil || !sh.Shields || sh.Power != 0 {
		t.Errorf("on with exactly the draw: %v, shields %v, power %d", err, sh.Shields, sh.Power)
	}
	sh.Refill()
	if !sh.Shields || sh.Power != d.PowerPerDay()-draw {
		t.Errorf("refilled on: shields %v, power %d, want %d", sh.Shields, sh.Power, d.PowerPerDay()-draw)
	}

	weak := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, HyperwaveNullifier}}
	if weak.ShieldDraw() <= weak.PowerPerDay() {
		t.Fatal("the fixture's draw fits its supply")
	}
	_, sh, _ = duel(weak, fightFixture)
	if sh.Shields || sh.ShieldsDown || sh.Power != weak.PowerPerDay() {
		t.Errorf("draw %d over supply %d, spawned: shields %v down %v, power %d",
			weak.ShieldDraw(), weak.PowerPerDay(), sh.Shields, sh.ShieldsDown, sh.Power)
	}
	sh.Power, sh.Shields = weak.ShieldDraw()+5, true // a reserve (Hyperfuel) paid them
	sh.Power -= weak.ShieldDraw()
	sh.Refill()
	if sh.Shields || sh.Power != weak.PowerPerDay() {
		t.Errorf("draw %d over supply %d, refilled: shields %v, power %d (the day cannot carry them: off)",
			weak.ShieldDraw(), weak.PowerPerDay(), sh.Shields, sh.Power)
	}
}

// A dark shield (the Shield Blaster) keeps the switch and the
// reservation: Absorb 0, Power untouched; off refunds the draw, the
// Refill lights it again with the draw taken.
func TestShieldsDownReservation(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	d := fightFixture
	useShieldBlaster(g.Empires[1], tg, 0, Target{Mine: sh})
	if !sh.ShieldsDown {
		t.Fatal("the Shield Blaster left the shields lit")
	}
	if !sh.Shields || sh.Absorb() != 0 || sh.Power != d.PowerPerDay()-d.ShieldDraw() {
		t.Errorf("dark: shields %v, absorb %d, power %d, want %d", sh.Shields, sh.Absorb(), sh.Power, d.PowerPerDay()-d.ShieldDraw())
	}
	sh.Refill()
	if !sh.Shields || sh.ShieldsDown || sh.Absorb() != d.ShieldScore() || sh.Power != d.PowerPerDay()-d.ShieldDraw() {
		t.Errorf("refilled: shields %v down %v, absorb %d, power %d", sh.Shields, sh.ShieldsDown, sh.Absorb(), sh.Power)
	}
	sh.ShieldsDown = true
	if err := sh.SetShields(false); err != nil || sh.Shields || sh.Power != d.PowerPerDay() {
		t.Errorf("dark, off: %v, shields %v, power %d, want %d (refunded)", err, sh.Shields, sh.Power, d.PowerPerDay())
	}
}

// Arm refuses a friendly target; a destruction by Round removes the ship
// from its empire (the others' order kept) and reports to both empires.
func TestRoundDestroy(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	e0, e1 := g.Empires[0], g.Empires[1]
	gun := part(fightFixture, MassBarrageGun)
	mate := e0.Spawn(fightFixture, sh.Star, g.Empires[0].Colonies[0].PlanetIdx)
	mate.Shields = false
	if _, _, err := fire(g, sh, gun, mate); err == nil || !strings.Contains(err.Error(), "not hostile") || mate.Hull != mate.MaxHull() {
		t.Errorf("friendly fire: %v, hull %d", err, mate.Hull)
	}
	if g.OwnerOf(sh) != e0 || g.OwnerOf(tg) != e1 {
		t.Fatal("OwnerOf")
	}
	if en := g.EnemiesAt(e0, sh.Star); !slices.Equal(en, []*Starship{tg}) {
		t.Errorf("enemies of empire 0: %v", en)
	}
	if en := g.EnemiesAt(e1, sh.Star); !slices.Equal(en, []*Starship{sh, mate}) {
		t.Errorf("enemies of empire 1: %v", en)
	}

	after := e1.Spawn(fightFixture, sh.Star, g.Empires[0].Colonies[0].PlanetIdx)
	tg.Shields, tg.Hull = false, Gizmos[MassBarrageGun].Level
	dmg, reps, err := fire(g, sh, gun, tg)
	if err != nil || dmg != Gizmos[MassBarrageGun].Level || len(reps) != 2 {
		t.Fatalf("damage %d, %d reports, %v", dmg, len(reps), err)
	}
	if g.OwnerOf(tg) != nil || !slices.Equal(e1.Ships, []*Starship{after}) {
		t.Errorf("empire 1's ships after the destruction: %v", e1.Ships)
	}
	for i, c := range []struct {
		empire int
		text   string
	}{{0, sh.Name + " destroys " + aShip(g.Empires, 1) + " at " + g.Cluster.Stars[sh.Star].Name},
		{1, tg.Name + " lost to " + aShip(g.Empires, 0) + " at " + g.Cluster.Stars[sh.Star].Name}} {
		r := reps[i]
		if r.Empire != c.empire || r.Event.Kind != Destroyed || r.Event.Text != c.text || r.Event.Star != sh.Star ||
			r.Event.Planet != -1 || r.Event.Item != NoItem || r.Event.Tech != NoTech {
			t.Errorf("report %d: %+v, want empire %d %q", i, r, c.empire, c.text)
		}
	}
	if Destroyed.String() != "destroyed" {
		t.Errorf("%q", Destroyed.String())
	}
	if g.Destroy(tg) {
		t.Error("destroyed twice")
	}
}

// ReadyWeapon: the chosen Weapon when it CanFire, else the first weapon
// that can (a spent choice, a Weapon past Parts or on a non-weapon);
// when none can, the first weapon's refusal; "no weapon" without one.
func TestReadyWeapon(t *testing.T) {
	a := Design{Hull: Small, Parts: []Gizmo{QuarkExpress, MassBarrageGun, Ueberlaser, IonWrap}}
	g, sh, tg := duel(a, fightFixture)
	gun, laser := part(a, MassBarrageGun), part(a, Ueberlaser)
	sh.Power = 100
	ready := func(name string, want int) {
		t.Helper()
		if p, err := sh.ReadyWeapon(g.Cluster, tg); p != want || err != nil {
			t.Errorf("%s: part %d %v, want %d", name, p, err, want)
		}
	}
	if sh.Weapon != -1 {
		t.Errorf("Spawn: Weapon %d", sh.Weapon)
	}
	ready("no choice", gun)
	sh.Weapon = laser
	ready("the laser chosen", laser)
	sh.Shots[laser] = Gizmos[Ueberlaser].Uses
	if sh.CanFire(g.Cluster, laser, tg) == nil {
		t.Fatal("the laser still fires")
	}
	ready("the laser spent", gun)
	sh.Weapon = len(a.Parts) + 3
	ready("a Weapon past Parts", gun)
	sh.Weapon = part(a, IonWrap)
	ready("a Weapon on a shield", gun)

	tg.Star = (sh.Star + 1) % len(g.Cluster.Stars)
	p, err := sh.ReadyWeapon(g.Cluster, tg)
	if want := sh.CanFire(g.Cluster, gun, tg); p != -1 || err == nil || err.Error() != want.Error() {
		t.Errorf("none can: part %d %v, want %v", p, err, want)
	}
	tg.Star = sh.Star

	unarmed := g.Empires[0].Spawn(Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}, sh.Star, g.Empires[0].Colonies[0].PlanetIdx)
	if p, err := unarmed.ReadyWeapon(g.Cluster, tg); p != -1 || err == nil || err.Error() != "no weapon" {
		t.Errorf("unarmed: part %d %v", p, err)
	}
}

// ShieldHits is the count of takeShot hits the shields absorb before one
// knocks them down, from every energy up to a few draws, for each shield
// part; 0 off, dark or without a shield.
func TestShieldHits(t *testing.T) {
	for g := range NumGizmos {
		if Gizmos[g].Cat != Shield {
			continue
		}
		d := Design{Name: "S", Hull: Small, Parts: []Gizmo{ProtonShaver, g}}
		for p := range 4*d.ShieldDraw() + 2 {
			s := &Starship{Design: d, Shields: true, Power: p, Hull: 100}
			n := s.ShieldHits()
			if d.ShieldScore() == 0 {
				if n != 0 {
					t.Errorf("%s (score 0): %d hits", Gizmos[g].Name, n)
				}
				continue
			}
			taken := 0
			for !s.ShieldsDown && taken <= p+1 {
				s.takeShot(100)
				if !s.ShieldsDown {
					taken++
				}
			}
			if taken > p { // still up past every energy point: it draws nothing
				taken = NoShieldFall
			}
			if n != taken {
				t.Errorf("%s at energy %d: ShieldHits %d, takeShot absorbed %d before falling", Gizmos[g].Name, p, n, taken)
			}
			if n != NoShieldFall && s.ShieldHits() != 0 {
				t.Errorf("%s dark: %d hits", Gizmos[g].Name, s.ShieldHits())
			}
		}
	}
	if s := (&Starship{Design: fightFixture, Shields: false, Power: 10}); s.ShieldHits() != 0 {
		t.Errorf("off: %d hits", s.ShieldHits())
	}
	if s := (&Starship{Design: FixtureColonyShip, Shields: true, Power: 10}); s.ShieldHits() != 0 {
		t.Errorf("no shield part: %d hits", s.ShieldHits())
	}
}
