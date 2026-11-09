package game

import (
	"slices"
	"strings"
	"testing"
)

// The cloakers (fog.go, orbit.go): the Orbital Cloaker is the cascade's
// first layer and hides the orbit and its ships from the other empires;
// it guards nothing; the Surface Cloaker hides the surface.

// cloak builds an Orbital Cloaker in p's slot.
func cloak(t *testing.T, p *Planet, slot int) {
	t.Helper()
	if err := p.BuildOrbital(slot, OrbitalCloaker); err != nil {
		t.Fatal(err)
	}
}

// The cloaker first, whatever its slot (after a shield's here), at its
// table Strength; once razed the shields come next and the orbit shows.
func TestCloakerCascade(t *testing.T) {
	g, sh, home := siege(t)
	p, pl := home.Planet, home.PlanetIdx
	e1 := g.Empires[1]
	if err := p.BuildOrbital(1, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	cloak(t, p, 4)
	L, c := Gizmos[Nanomanipulator].Level, OrbitalDefences[OrbitalCloaker].Strength
	if L >= c || 2*L < c {
		t.Fatalf("fixture: shot %d against the cloaker's %d", L, c)
	}
	if e1.SeesOrbit(p) {
		t.Fatal("a foreign cloaked orbit is seen")
	}
	_, log := volley(t, g, sh, pl, 1)
	if f := log.Shots[0]; f.HitSlot != 4 || f.HitItem != OrbitalCloaker || f.Damage != L || p.OrbitDamage[4] != L || p.OrbitDamage[1] != 0 {
		t.Fatalf("first shot: %+v, damage %v", f, p.OrbitDamage)
	}
	_, log = volley(t, g, sh, pl, 1)
	if p.Orbit[4] != NoItem || !slices.Equal(log.Razed, []Razing{{Planet: pl, Slot: 4, Item: OrbitalCloaker}}) {
		t.Fatalf("second shot: orbit %v razed %v", p.Orbit, log.Razed)
	}
	if !e1.SeesOrbit(p) {
		t.Error("the cloaker razed: the orbit still hidden")
	}
	_, log = volley(t, g, sh, pl, 1)
	if f := log.Shots[0]; f.HitSlot != 1 || f.HitItem != OrbitalShield {
		t.Errorf("after the cloaker: %+v", f)
	}
}

// A cloaker alone guards nothing: no Guard, the orbit open, an invasion
// allowed; CanHitPlanet takes the planet (the cloaker to hit).
func TestCloakerNoGuard(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	e0 := g.Empires[0]
	star, pl := col.Star, col.PlanetIdx
	cloak(t, col.Planet, 0)
	if who, ok := g.Guard(star, pl); ok {
		t.Fatalf("a cloaker guards: %d", who)
	}
	if e0.Barred(star, pl) {
		t.Error("a cloaker bars the orbit")
	}
	inv := e0.Spawn(invader(2), star, pl)
	if err := e0.CanInvade(inv); err != nil {
		t.Errorf("invasion refused under a cloaker: %v", err)
	}
	if err := g.CanHitPlanet(e0, star, pl); err != nil {
		t.Errorf("CanHitPlanet: %v", err)
	}
}

// ShipsSeen hides a foreign ship in a cloaked orbit (Hostiles and the
// player's Engaged too), shows it once out of orbit or the cloaker razed;
// the owner sees the ships in its orbit, ScanAll and ShipsSeenAll (the
// bots') see through.
func TestShipsSeenCloaked(t *testing.T) {
	g, sh, home := siege(t)
	p, star, pl := home.Planet, home.Star, home.PlanetIdx
	e0, e1 := g.Empires[0], g.Empires[1]
	e1.Visit(star)
	cloak(t, p, 2)
	in := e0.Spawn(orbitShip, star, pl)
	has := func(sgs []Sighting, s *Starship) bool {
		return slices.ContainsFunc(sgs, func(sg Sighting) bool { return sg.Of(s) })
	}
	if has(g.ShipsSeen(e1, star), in) || has(g.Hostiles(e1, star), in) {
		t.Error("a ship in a foreign cloaked orbit is seen")
	}
	if !has(g.ShipsSeenAll(e1, star), in) || !has(g.HostilesAll(e1, star), in) {
		t.Error("the bots' view misses the cloaked ship")
	}
	if g.Engaged(e1, star) {
		t.Error("engaged against a hidden ship alone")
	}
	if !slices.Contains(g.CombatStars(), star) {
		t.Error("fixture: no fight at the star")
	}
	if _, ok := g.SightingOf(e1, in); ok {
		t.Error("SightingOf a hidden ship")
	}

	// The owner sees a foreign ship in its own cloaked orbit.
	sh.Orbit = pl
	if !has(g.ShipsSeen(e0, star), sh) {
		t.Error("the owner misses a ship in its orbit")
	}
	sh.Orbit = -1

	e1.ScanAll = true
	if !has(g.ShipsSeen(e1, star), in) {
		t.Error("ScanAll misses the cloaked ship")
	}
	e1.ScanAll = false

	in.Orbit = -1 // out of orbit: seen again
	if !has(g.ShipsSeen(e1, star), in) || !g.Engaged(e1, star) {
		t.Error("a ship out of the cloaked orbit is not seen")
	}
	in.Orbit = pl
	p.DemolishOrbital(2)
	if !has(g.ShipsSeen(e1, star), in) {
		t.Error("the cloaker gone: the ship in orbit still hidden")
	}
}

// SeesSurface: the Surface Cloaker hides the surface from the others,
// not from the owner nor ScanAll; SeesOrbit ignores it.
func TestSeesSurface(t *testing.T) {
	g, _, home := siege(t)
	p := home.Planet
	e0, e1 := g.Empires[0], g.Empires[1]
	if !e1.SeesSurface(p) {
		t.Fatal("an uncloaked surface hidden")
	}
	for _, c := range p.Cells() {
		if p.Surface[c].Item == NoItem {
			p.Surface[c].Item = SurfaceCloaker
			break
		}
	}
	if e1.SeesSurface(p) || !e0.SeesSurface(p) || !e1.SeesOrbit(p) {
		t.Errorf("cloaked surface: foreign %v, owner %v, orbit %v", e1.SeesSurface(p), e0.SeesSurface(p), e1.SeesOrbit(p))
	}
	e1.ScanAll = true
	if !e1.SeesSurface(p) {
		t.Error("ScanAll blind to a cloaked surface")
	}
	var nobody *Empire
	if !nobody.SeesSurface(p) || !nobody.SeesOrbit(p) {
		t.Error("a nil empire (the views' bare tests) must see everything")
	}
}

// CanHitPlanet on an orbit e does not see judges from what e sees:
// a cloaked planet of an empire e is not at War
// with, a hostile ship hidden in its orbit, is refused as if that orbit
// were empty (ArmAtPlanet too); ScanAll, or the cloaker razed, sees the
// ship and takes the planet.
func TestCanHitCloakedPlanet(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Mebes}, Average)
	atWar(t, g, 0, 2)
	e0, home := g.Empires[0], g.Empires[1].Colonies[0]
	p, star, pl := home.Planet, home.Star, home.PlanetIdx
	e0.Visit(star)
	if g.Hostile(e0, g.Empires[1]) || !g.Hostile(e0, g.Empires[2]) {
		t.Fatal("fixture: stances")
	}
	empty := g.CanHitPlanet(e0, star, pl) // nothing in orbit: the refusal to match
	if empty == nil {
		t.Fatal("fixture: a neutral's empty orbit taken")
	}
	cloak(t, p, 3)
	g.Empires[2].Spawn(orbitShip, star, pl)
	err := g.CanHitPlanet(e0, star, pl)
	if err == nil || err.Error() != empty.Error() {
		t.Errorf("a hidden hostile ship: %v, want %v", err, empty)
	}
	gun := e0.Spawn(siegeGun, star, pl)
	gun.Orbit = -1
	if err := e0.ArmAtPlanet(gun, 3, pl); err == nil || err.Error() != empty.Error() {
		t.Errorf("ArmAtPlanet: %v, want %v", err, empty)
	}
	e0.ScanAll = true
	if err := g.CanHitPlanet(e0, star, pl); err != nil {
		t.Errorf("ScanAll: %v", err)
	}
	e0.ScanAll = false
	p.DemolishOrbital(3)
	if err := g.CanHitPlanet(e0, star, pl); err != nil {
		t.Errorf("the cloaker razed: %v", err)
	}
}

// An invasion refused by a guard in an orbit the invader does not see
// names nobody: no species, no "guarded by"; seen, the guard is named.
func TestInvadeHiddenGuard(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa, Mebes)
	e0 := g.Empires[0]
	star, pl := col.Star, col.PlanetIdx
	g.Empires[2].Spawn(orbitShip, star, pl)
	inv := e0.Spawn(invader(2), star, pl)
	seen := e0.CanInvade(inv)
	who := whoIn(g.Empires, 2)
	if seen == nil || !strings.Contains(seen.Error(), who) {
		t.Fatalf("fixture: the seen guard %v, want %q named", seen, who)
	}
	cloak(t, col.Planet, 0)
	err := e0.CanInvade(inv)
	if err == nil {
		t.Fatal("the hidden guard no longer refuses")
	}
	if strings.Contains(err.Error(), who) || strings.Contains(err.Error(), "guarded by") {
		t.Errorf("hidden guard named: %v", err)
	}
	// The owner's hidden Orbital Shield: refused, the owner not named for it.
	destroyIn(g.Empires, g.Empires[2].Ships[0])
	if err := col.Planet.BuildOrbital(1, OrbitalShield); err != nil {
		t.Fatal(err)
	}
	if err := e0.CanInvade(inv); err == nil || strings.Contains(err.Error(), whoIn(g.Empires, 1)) {
		t.Errorf("hidden shield: %v", err)
	}
}

// A shot cascading through a cloaked orbit onto a ship there: the
// shooter's news names neither the ship (aShip) nor the cloaker it
// razed (the orbit unseen when they fell); the owner's names both.
func TestCloakedOrbitNews(t *testing.T) {
	g, sh, home := siege(t)
	p, star, pl := home.Planet, home.Star, home.PlanetIdx
	e0 := g.Empires[0]
	cloak(t, p, 4)
	L, c := Gizmos[Nanomanipulator].Level, OrbitalDefences[OrbitalCloaker].Strength
	if L >= c || 2*L < c {
		t.Fatalf("fixture: shot %d against the cloaker's %d", L, c)
	}
	in := e0.Spawn(orbitShip, star, pl)
	in.Hull = 1
	reps, log := volley(t, g, sh, pl, 3)
	if len(log.Destroyed) != 1 || log.Destroyed[0] != in || len(log.Razed) != 1 {
		t.Fatalf("fixture: destroyed %v razed %v", log.Destroyed, log.Razed)
	}
	var mine []Event
	theirs := 0
	for _, r := range reps {
		switch r.Empire {
		case 1:
			mine = append(mine, r.Event)
		case 0:
			theirs++
			if r.Event.Kind == Razed && r.Event.Item != OrbitalCloaker {
				t.Errorf("the owner's razing: %+v", r.Event)
			}
		}
	}
	if len(mine) != 2 || theirs < 2 {
		t.Fatalf("reports %+v", reps)
	}
	for _, ev := range mine {
		if strings.Contains(ev.Text, aShip(g.Empires, 0)) || strings.Contains(ev.Text, OrbitalCloaker.String()) ||
			ev.Kind == Razed && ev.Item != NoItem || !strings.Contains(ev.Text, p.Name+"'s orbit") {
			t.Errorf("the shooter's news: %+v", ev)
		}
	}
}

// A Phase Bomb on a surface its user does not see (a Surface Cloaker):
// the user's text and outcome do not name what it razed; the owner's
// Razed report does.
func TestPhaseBombCloakedSurface(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	atWar(t, g, 0, 1)
	e := g.Empires[0]
	home1 := g.Empires[1].Colonies[0]
	p := home1.Planet
	d := aimer(PhaseBomb)
	s := e.Spawn(d, home1.Star, home1.PlanetIdx)
	inSpaceAt(s, p.Pos)
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item != ColonyBase {
			p.Demolish(c)
		}
	}
	var free []Cell
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item == NoItem {
			free = append(free, c)
		}
	}
	p.Surface[free[0]].Item, p.Surface[free[len(free)-1]].Item = Factory, SurfaceCloaker
	if e.SeesSurface(p) {
		t.Fatal("fixture: the surface seen")
	}
	reps := useAt(t, e, s, part(d, PhaseBomb), Target{Kind: PlanetTarget, Index: home1.PlanetIdx})
	if p.Surface[free[0]].Item != NoItem || len(reps) != 2 {
		t.Fatalf("fixture: %v, reps %+v", p.Surface[free[0]].Item, reps)
	}
	if u := reps[0].Event; strings.Contains(u.Text, Factory.String()) || strings.Contains(u.Use.Outcome, Factory.String()) {
		t.Errorf("the user's: %q, outcome %q", u.Text, u.Use.Outcome)
	}
	if o := reps[1].Event; o.Item != Factory || !strings.Contains(o.Text, Factory.String()) {
		t.Errorf("the owner's: %+v", o)
	}
}
