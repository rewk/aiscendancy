package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// Planets in combat on the system display (sysplanet.go, replay.go): the
// planet's selection, its panel, its fire mode, a ship's fire at a planet,
// the replay of planet shots and razings.

// defences builds on p's orbit the structures of slots (slot -> item).
func defences(t *testing.T, p *game.Planet, slots map[int]game.Item) {
	t.Helper()
	for k, it := range slots {
		if err := p.BuildOrbital(k, it); err != nil {
			t.Fatal(err)
		}
	}
}

// homeDefences is the player's home in panelFixture with a Long Range
// Orbital Whopper in slot 0, an Orbital Missile Base in slot 2, an
// Orbital Shield in slot 5 and Orbital Docks in slot 7.
var homeDefences = map[int]game.Item{0: game.LongRangeOrbitalWhopper, 2: game.OrbitalMissileBase, 5: game.OrbitalShield, 7: game.OrbitalDocks}

// pressSlot presses the centre of the selected planet's tile of slot.
func pressSlot(t *testing.T, v *systemScreen, a *app, slot int) error {
	t.Helper()
	for _, pt := range planetTiles(v.psel) {
		if pt.slot == slot {
			return v.planetClick(a, float64(pt.r.X0+pt.r.X1)/2, float64(pt.r.Y0+pt.r.Y1)/2)
		}
	}
	t.Fatalf("no tile for slot %d", slot)
	return nil
}

// A first click on a planet selects it, a second opens its screen; a ship
// click and a planet click take the selection from each other (one at a
// time), as do a filter press and a click on nothing; Esc ends the
// planet's fire mode, the selection kept.
func TestPlanetSelect(t *testing.T) {
	a, v, _, gun, _ := panelFixture(t)
	a.push(v) // the planet screen pops back to it
	home := a.e.Colonies[0]
	pb := v.planetBody(home.PlanetIdx)
	n := len(a.stack)
	if v.clickBody(a, pb); v.psel != home.Planet || v.sel != nil || v.box() != boxPlanet || len(a.stack) != n {
		t.Fatalf("first click: psel %v sel %v box %d, %d screens", v.psel, v.sel, v.box(), len(a.stack))
	}
	if v.clickBody(a, pb); len(a.stack) != n+1 {
		t.Fatalf("second click: %d screens", len(a.stack))
	}
	if ps := top[*planetScreen](t, a); ps.star != v.star || ps.idx != home.PlanetIdx {
		t.Errorf("opened star %d planet %d", ps.star, ps.idx)
	}
	a.pop()
	if v.clickBody(a, shipOf(v, gun)); v.sel != gun || v.psel != nil || v.box() != boxPanel {
		t.Fatalf("ship click: sel %v psel %v", v.sel, v.psel)
	}
	other := v.planetBody((home.PlanetIdx + 1) % len(v.sys.Planets))
	// A list row inspects even with a ship selected: no order.
	if v.clickList(a, other); v.psel != other.planet || v.sel != nil || gun.Order.Kind != game.NoOrder || len(a.stack) != n {
		t.Fatalf("planet row click with a ship selected: psel %v sel %v order %+v, %d screens", v.psel == other.planet, v.sel, gun.Order, len(a.stack))
	}
	if v.clickList(a, other); len(a.stack) != n+1 {
		t.Fatalf("second row click: %d screens", len(a.stack))
	}
	a.pop()
	if v.clickBody(a, shipOf(v, gun)); v.sel != gun || v.psel != nil {
		t.Errorf("ship click: sel %v psel %v", v.sel, v.psel)
	}
	v.cancel()
	v.clickBody(a, pb)
	if v.showTab(planetsTab); v.psel != nil || !v.listsOn() {
		t.Errorf("filter press: psel %v", v.psel)
	}
	v.clickBody(a, pb)
	if v.clickBody(a, nil); v.psel != nil {
		t.Errorf("click on nothing: psel %v", v.psel)
	}
	defences(t, home.Planet, homeDefences)
	v.clickBody(a, pb)
	if err := pressSlot(t, v, a, 0); err != nil || !v.aim {
		t.Fatalf("weapon press: %v, aim %v", err, v.aim)
	}
	if !v.cancel() || v.aim || v.psel != home.Planet {
		t.Errorf("Esc: aim %v psel %v", v.aim, v.psel)
	}
	// Esc again drops the planet, a third leaves the screen (app.back).
	if !v.cancel() || v.psel != nil {
		t.Errorf("second Esc: psel %v", v.psel)
	}
	if v.cancel() {
		t.Error("third Esc: cancelled something")
	}
	v.clickBody(a, pb)
	m := len(a.stack)
	if err := a.back(); err != nil || len(a.stack) != m || v.psel != nil {
		t.Errorf("back with a planet: %v, %d screens (was %d), psel %v", err, len(a.stack), m, v.psel)
	}
	// With none it leaves: popped, or the quit prompt on the last screen.
	if err := a.back(); err != nil || len(a.stack) == m && !a.quitting {
		t.Errorf("back with none: %v, %d screens (was %d), quitting %v", err, len(a.stack), m, a.quitting)
	}
}

// The panel: one tile per structure in slot order, weapons, shields, then
// the rest, inside the info box under the globe, none overlapping; the
// player's colony tells shots left (Shots − OrbitShots) and hit points
// left (DefenceLeft); another empire's planet shows the same structures
// and neither (its damage is hidden), and its weapons do not enter the
// fire mode.
func TestPlanetPanel(t *testing.T) {
	a, v, _, _, _ := panelFixture(t)
	home := a.e.Colonies[0]
	defences(t, home.Planet, homeDefences)
	home.OrbitShots[0] = 1
	home.Planet.OrbitDamage[5] = 4
	v.clickBody(a, v.planetBody(home.PlanetIdx))

	// The bands from game's table: a weapon has Damage, a shield none.
	band := func(it game.Item) int {
		switch d := game.OrbitalDefences[it]; {
		case d.Damage > 0:
			return 0
		case d.Strength > 0:
			return 1
		}
		return 2
	}
	var want []planetTile
	for b := range 3 {
		for k, it := range home.Planet.Orbit {
			if it != game.NoItem && band(it) == b {
				want = append(want, planetTile{it: it, slot: k, band: b})
			}
		}
	}
	ts := planetTiles(home.Planet)
	if len(ts) != len(want) || len(ts) != len(homeDefences) {
		t.Fatalf("%d tiles, want %d", len(ts), len(want))
	}
	for i, pt := range ts {
		if pt.it != want[i].it || pt.slot != want[i].slot || pt.band != want[i].band {
			t.Errorf("tile %d: %v slot %d band %d, want %v slot %d band %d", i, pt.it, pt.slot, pt.band, want[i].it, want[i].slot, want[i].band)
		}
		r := pt.r
		if r.X0 < sysInfo.X0 || r.X1 > sysInfo.X1 || r.Y0 < planetPicTop+planetPicD || r.Y1 > panelBottom {
			t.Errorf("tile %d at %v: outside the box under the globe", i, r)
		}
		for j, q := range ts[:i] {
			if r.X0 < q.r.X1 && q.r.X0 < r.X1 && r.Y0 < q.r.Y1 && q.r.Y0 < r.Y1 {
				t.Errorf("tiles %d and %d overlap", j, i)
			}
		}
	}

	text := func(v *systemScreen, slot int) string {
		for _, pt := range planetTiles(v.psel) {
			if pt.slot == slot {
				return v.planetTileText(pt)
			}
		}
		t.Fatalf("no tile for slot %d", slot)
		return ""
	}
	lrw := game.OrbitalDefences[game.LongRangeOrbitalWhopper]
	left0, _ := home.Planet.DefenceLeft(a.e, 0)
	left5, ok := home.Planet.DefenceLeft(a.e, 5)
	if !ok || left5 != game.OrbitalDefences[game.OrbitalShield].Strength-4 {
		t.Fatalf("DefenceLeft(5) = %d %v: the fixture's damage did not land", left5, ok)
	}
	if got, w := text(v, 0), fmt.Sprintf("%d shots left, %d/%d pts", lrw.Shots-1, left0, lrw.Strength); !strings.Contains(got, w) {
		t.Errorf("our weapon: %q, want %q in it", got, w)
	}
	if got, w := text(v, 5), fmt.Sprintf("%d/%d pts", left5, game.OrbitalDefences[game.OrbitalShield].Strength); !strings.Contains(got, w) {
		t.Errorf("our shield: %q, want %q in it", got, w)
	}
	if got := text(v, 7); got != game.OrbitalDocks.String() {
		t.Errorf("our docks: %q", got)
	}
	if !slices.Contains(v.planetLines(a.e), "our colony") {
		t.Errorf("our lines %q", v.planetLines(a.e))
	}

	// Empire 1's home with the same orbit, damaged the same.
	f := a.g.Empires[1].Colonies[0]
	defences(t, f.Planet, homeDefences)
	f.Planet.OrbitDamage[5] = 4
	a.e.Visit(f.Star)
	w := newSystemScreen(f.Star)
	w.Enter(a)
	w.clickBody(a, w.planetBody(f.PlanetIdx))
	if w.psel != f.Planet || len(planetTiles(f.Planet)) != len(homeDefences) || w.pselColony() != nil {
		t.Fatalf("foreign planet: psel %v, %d tiles", w.psel, len(planetTiles(f.Planet)))
	}
	for k := range homeDefences {
		if got := text(w, k); strings.Contains(got, "pts") || strings.Contains(got, "left") {
			t.Errorf("foreign slot %d shows what is hidden: %q", k, got)
		}
	}
	if !slices.Contains(w.planetLines(a.e), fmt.Sprintf("%s colony, %s", a.g.Empires[1].Species, game.ColourName(colourOf(a.g, 1)))) {
		t.Errorf("foreign lines %q", w.planetLines(a.e))
	}
	if err := pressSlot(t, w, a, 0); err == nil || w.aim {
		t.Errorf("a foreign weapon's press: %v, aim %v", err, w.aim)
	}
}

// The planet's fire mode: a weapon tile, then a hostile ship, arms it
// (Empire.ArmPlanet); its refusals (armed already, no shot left) come
// back; a friendly ship fires nothing. A ship's fire mode on a planet
// arms ArmAtPlanet when CanHitPlanet accepts it (the label says "fire at
// <planet>"), else shows its refusal (label and click alike).
func TestPlanetFire(t *testing.T) {
	a, v, _, gun, raider := panelFixture(t)
	home := a.e.Colonies[0]
	defences(t, home.Planet, homeDefences)
	v.clickBody(a, v.planetBody(home.PlanetIdx))
	if err := pressSlot(t, v, a, 0); err != nil || !v.aim || v.pslot != 0 {
		t.Fatalf("weapon press: %v aim %v slot %d", err, v.aim, v.pslot)
	}
	if !strings.HasPrefix(v.planetFoot(), "fire "+game.LongRangeOrbitalWhopper.String()) {
		t.Errorf("foot %q", v.planetFoot())
	}
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("aim at the Raider: %v %v", b, live)
	}
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || v.aim || len(home.Armed) != 1 || home.Armed[0].Part != 0 {
		t.Fatalf("fire at the Raider: %v, aim %v, armed %+v", err, v.aim, home.Armed)
	}
	pressSlot(t, v, a, 0)
	if err := v.aimClick(a, shipOf(v, raider)); err == nil || !strings.Contains(err.Error(), "already armed") {
		t.Errorf("twice: %v", err)
	}
	pressSlot(t, v, a, 2)
	home.OrbitShots[2] = game.OrbitalDefences[game.OrbitalMissileBase].Shots
	if _, live := v.aimTarget(); live {
		t.Error("no shot left: the line is live")
	}
	if err := v.aimClick(a, shipOf(v, raider)); err == nil || !strings.Contains(err.Error(), "no shot left") || len(home.Armed) != 1 {
		t.Errorf("no shot left: %v, armed %+v", err, home.Armed)
	}
	pressSlot(t, v, a, 0)
	if err := v.aimClick(a, shipOf(v, gun)); err != nil || len(home.Armed) != 1 {
		t.Errorf("at our own ship: %v, armed %+v", err, home.Armed)
	}

	// A ship at a planet: empire 1's colony ship guards an uninhabited one.
	pl := slices.IndexFunc(v.sys.Planets, func(p *game.Planet) bool { return p.Owner < 0 })
	if pl < 0 {
		t.Fatal("no uninhabited planet at the home star")
	}
	a.g.Empires[1].Spawn(game.FixtureColonyShip, v.star, pl)
	v.rebuild()
	v.clickBody(a, shipOf(v, gun))
	aimGun := func() {
		t.Helper()
		if err := pressTile(t, v, a, gun, game.MassBarrageGun); err != nil || !v.aim {
			t.Fatalf("gun tile: %v aim %v", err, v.aim)
		}
	}
	aimGun()
	v.hover = v.planetBody(pl)
	// Out of the gun's range: refused at arm time, the label red, the
	// line dead (not "fire at" then nothing fired).
	pp := v.sys.Planets[pl].Pos
	gun.Pos = game.Vec3{X: pp.X + 2*gun.WeaponRange(&game.Gizmos[game.MassBarrageGun]), Y: pp.Y, Z: pp.Z}
	far := gun.PlanetInRange(a.g.Cluster, gun.Weapon, pl)
	if far == nil {
		t.Fatal("fixture: the planet in range")
	}
	if _, label, live, ok := v.planetAimLabel(); !ok || live || label != far.Error() {
		t.Errorf("out of range: %q live %v, want %q", label, live, far)
	}
	if _, live := v.aimTarget(); live {
		t.Error("out of range: the line is live")
	}
	if err := v.aimClick(a, v.planetBody(pl)); err == nil || err.Error() != far.Error() || len(gun.Armed) != 0 {
		t.Fatalf("fire out of range: %v, armed %+v", err, gun.Armed)
	}
	gun.Pos = game.Vec3{X: pp.X + 0.5*gun.WeaponRange(&game.Gizmos[game.MassBarrageGun]), Y: pp.Y, Z: pp.Z}
	aimGun() // a click ends the fire mode
	v.hover = v.planetBody(pl)
	if b, label, live, ok := v.planetAimLabel(); !ok || !live || b != v.hover || label != "fire at "+v.sys.Planets[pl].Name {
		t.Errorf("guarded planet: %q live %v ok %v", label, live, ok)
	}
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("line to the guarded planet: %v %v", b, live)
	}
	if err := v.aimClick(a, v.planetBody(pl)); err != nil || !slices.ContainsFunc(gun.Armed, func(s game.Shot) bool { return s.AtPlanet && s.Planet == pl }) {
		t.Fatalf("fire at the guarded planet: %v, armed %+v", err, gun.Armed)
	}
	// A second Mass Barrage Gun: the tile's second shot goes through it.
	gun.Design.Parts = append(slices.Clone(gun.Design.Parts), game.MassBarrageGun)
	gun.Shots = append(gun.Shots, 0)
	gun.Power = 10
	aimGun()
	if err := v.aimClick(a, v.planetBody(pl)); err != nil || len(gun.Armed) != 2 || gun.Armed[1].Part != len(gun.Design.Parts)-1 {
		t.Fatalf("the twin gun at the planet: %v, armed %+v", err, gun.Armed)
	}
	a.e.Disarm(gun)
	aimGun()
	v.hover = v.planetBody(home.PlanetIdx)
	want := a.g.CanHitPlanet(a.e, v.star, home.PlanetIdx)
	if want == nil {
		t.Fatal("our home is a target")
	}
	if _, label, live, ok := v.planetAimLabel(); !ok || live || label != want.Error() {
		t.Errorf("our home's label %q live %v, want %q", label, live, want)
	}
	if err := v.aimClick(a, v.planetBody(home.PlanetIdx)); err == nil || err.Error() != want.Error() || len(gun.Armed) != 0 {
		t.Errorf("fire at our home: %v, armed %+v", err, gun.Armed)
	}
}

// The planet's fire mode on a planet (Empire.ArmPlanetAt): the label
// and the line follow CanArmPlanetAt ("fire at <planet>" and live, else
// its refusal, red, dead); a click arms the
// AtPlanet shot, or comes back refused with nothing armed.
func TestPlanetFirePlanet(t *testing.T) {
	a, v, _, _, _ := panelFixture(t)
	home := a.e.Colonies[0]
	defences(t, home.Planet, homeDefences)
	// An uninhabited planet guarded by empire 1's colony ship, between
	// the Missile Base's range and the Long Range Whopper's.
	pl := slices.IndexFunc(v.sys.Planets, func(p *game.Planet) bool { return p.Owner < 0 })
	if pl < 0 {
		t.Fatal("no uninhabited planet at the home star")
	}
	dl, dm := game.OrbitalDefences[game.LongRangeOrbitalWhopper], game.OrbitalDefences[game.OrbitalMissileBase]
	hp := home.Planet.Pos
	v.sys.Planets[pl].Pos = game.Vec3{X: hp.X + float64(dl.Range+dm.Range)/2*game.RangeUnit, Y: hp.Y, Z: hp.Z}
	a.g.Empires[1].Spawn(game.FixtureColonyShip, v.star, pl)
	v.rebuild()
	v.clickBody(a, v.planetBody(home.PlanetIdx))
	aim := func(slot int) {
		t.Helper()
		if err := pressSlot(t, v, a, slot); err != nil || !v.aim || v.pslot != slot {
			t.Fatalf("weapon press %d: %v aim %v slot %d", slot, err, v.aim, v.pslot)
		}
	}
	// The base: out of range.
	aim(2)
	v.hover = v.planetBody(pl)
	far := a.e.CanArmPlanetAt(home, 2, pl)
	if far == nil || !strings.Contains(far.Error(), "out of range") {
		t.Fatalf("fixture: the base %v", far)
	}
	if b, label, live, ok := v.planetAimLabel(); !ok || live || b != v.hover || label != far.Error() {
		t.Errorf("out of range: %q live %v ok %v, want %q", label, live, ok, far)
	}
	if _, live := v.aimTarget(); live {
		t.Error("out of range: the line is live")
	}
	if err := v.aimClick(a, v.planetBody(pl)); err == nil || err.Error() != far.Error() || len(home.Armed) != 0 {
		t.Fatalf("the base out of range: %v, armed %+v", err, home.Armed)
	}
	// Its own planet: refused.
	aim(0)
	v.hover = v.planetBody(home.PlanetIdx)
	self := a.e.CanArmPlanetAt(home, 0, home.PlanetIdx)
	if _, label, live, ok := v.planetAimLabel(); self == nil || !ok || live || label != self.Error() {
		t.Errorf("itself: %q live %v, want %v", label, live, self)
	}
	// The Whopper: in range.
	v.hover = v.planetBody(pl)
	if b, label, live, ok := v.planetAimLabel(); !ok || !live || b != v.hover || label != "fire at "+v.sys.Planets[pl].Name {
		t.Errorf("in range: %q live %v ok %v", label, live, ok)
	}
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("the line: %v %v", b, live)
	}
	if err := v.aimClick(a, v.planetBody(pl)); err != nil || v.aim ||
		!slices.Equal(home.Armed, []game.Shot{{Part: 0, AtPlanet: true, Planet: pl}}) {
		t.Fatalf("fire at the planet: %v, aim %v, armed %+v", err, v.aim, home.Armed)
	}
	// The Tractor Beam's aim takes no planet label.
	v.aim, v.ptract = true, true
	if _, _, _, ok := v.planetAimLabel(); ok {
		t.Error("a label in the beam's aim")
	}
}

// A planet's shot at a planet, played by a real Round: the replay's
// bullet flies from our planet to the other one with planetGun's bank,
// its impact on the target planet; the log line reads "<planet>
// (orbital)  <structure> → <planet>  -N <structure>", the target's
// owner's flag; the toast counts it dealt; its sounds are the gun's.
func TestReplayPlanetAtPlanet(t *testing.T) {
	a, v, _, _, _ := panelFixture(t)
	home := a.e.Colonies[0]
	p := home.Planet
	defences(t, p, homeDefences)
	pl := slices.IndexFunc(v.sys.Planets, func(q *game.Planet) bool { return q.Owner < 0 })
	if pl < 0 {
		t.Fatal("no uninhabited planet at the home star")
	}
	q := v.sys.Planets[pl]
	q.Pos = game.Vec3{X: p.Pos.X + 0.5, Y: p.Pos.Y, Z: p.Pos.Z}
	e1 := a.g.Empires[1]
	q.Owner = e1.Index
	e1.Colonies = append(e1.Colonies, &game.Colony{Planet: q, Star: v.star, PlanetIdx: pl})
	if err := q.BuildOrbital(0, game.OrbitalShield); err != nil {
		t.Fatal(err)
	}
	v.rebuild()
	cast := v.replayCast()
	if err := a.e.ArmPlanetAt(home, 0, pl); err != nil { // homeDefences' Long Range Whopper
		t.Fatal(err)
	}
	_, _, lg := a.g.Round(v.star, []bool{false, true})
	if len(lg.Shots) != 1 || !lg.Shots[0].FromPlanet || !lg.Shots[0].AtPlanet {
		t.Fatalf("round: %+v", lg.Shots)
	}
	f := lg.Shots[0]
	v.startReplay(a.e, cast, []game.RoundLog{lg})
	if v.replay == nil || v.rlog == nil {
		t.Fatal("no replay")
	}
	r := &v.replay.rounds[0]
	gun := planetGun(game.LongRangeOrbitalWhopper)
	at := func(tick int) []effect {
		v.tick = v.replay.t0 + r.start + tick
		v.rebuild()
		return v.effects()
	}
	var bs []effect
	for _, e := range at(shotAt(0)) {
		if e.kind == fxBullet {
			bs = append(bs, e)
		}
	}
	if len(bs) != 1 || bs[0].from == nil || bs[0].from.planet != p || bs[0].ship == nil || bs[0].ship.planet != q ||
		bs[0].fromPos != p.Pos || bs[0].toPos != q.Pos || bs[0].bank != bulletBank(gun) {
		t.Errorf("the bullet: %+v", bs)
	}
	bank, _ := impactOf(f)
	if !slices.ContainsFunc(at(landsAt(0)), func(e effect) bool {
		return e.kind == fxImpact && e.bank == bank && e.ship != nil && e.ship.planet == q
	}) {
		t.Errorf("no %s impact on %s", bank, q.Name)
	}
	d := game.OrbitalDefences[game.LongRangeOrbitalWhopper]
	line, flags := lineText(v.roundLines(v.replay, r, 0, nil)[0])
	want := fmt.Sprintf("%s (orbital)  %s → %s  -%d %s", p.Name, game.LongRangeOrbitalWhopper, q.Name, d.Damage, game.OrbitalShield)
	if line != want || !slices.Equal(flags, []int{0, 1}) {
		t.Errorf("log line %q %v, want %q [0 1]", line, flags, want)
	}
	if got, want := roundNews(a.g, 0, 1, cast, lg), fmt.Sprintf("Round 1: dealt 1 hit (%d hull)", d.Damage); got != want {
		t.Errorf("toast %q, want %q", got, want)
	}
	if hitCue(f) != hitSound(gun) {
		t.Errorf("hit cue %q, want %q", hitCue(f), hitSound(gun))
	}
}

// The replay of a round with a planet's shot (from the planet, planetGun's
// bullet and explosion), a shot at a structure (over the planet's sprite),
// a shot at a ship in orbit (on its ring) and a razing (a blast over the
// planet, the kill phase without a destroyed ship): every effect placed,
// none panics drawn across the replay.
func TestReplayPlanets(t *testing.T) {
	a, v, colony, _, raider := panelFixture(t)
	home := a.e.Colonies[0]
	p := home.Planet
	defences(t, p, homeDefences)
	if colony.Orbit != home.PlanetIdx {
		t.Fatalf("the colony ship is not in orbit of the home (%d)", colony.Orbit)
	}
	cast := v.replayCast()
	ring := drawnBody(t, cast, colony).pos
	logs := []game.RoundLog{{Star: v.star,
		Shots: []game.Fired{
			{FromPlanet: true, ShooterPlanet: home.PlanetIdx, ShooterSlot: 2, ShooterItem: game.OrbitalMissileBase,
				Target: raider, ShooterEmpire: 0, TargetEmpire: 1, From: p.Pos, To: raider.Pos, Damage: 5},
			{Shooter: raider, Gizmo: game.MassBarrageGun, ShooterEmpire: 1, TargetEmpire: 0, From: raider.Pos, To: p.Pos,
				AtPlanet: true, TargetPlanet: home.PlanetIdx, HitSlot: 5, HitItem: game.OrbitalShield, Damage: 1},
			{Shooter: raider, Target: colony, Gizmo: game.MassBarrageGun, ShooterEmpire: 1, TargetEmpire: 0, From: raider.Pos, To: colony.Pos,
				AtPlanet: true, TargetPlanet: home.PlanetIdx, HitSlot: -1, HitItem: game.NoItem, Damage: 1},
		},
		Razed: []game.Razing{{Planet: home.PlanetIdx, Slot: 5, Item: game.OrbitalShield}},
	}}
	v.startReplay(a.e, cast, logs)
	if v.replay == nil {
		t.Fatal("no replay")
	}
	r := &v.replay.rounds[0]
	if r.kill == 0 {
		t.Fatal("a razing alone has no kill phase")
	}
	pb := v.planetBody(home.PlanetIdx)
	at := func(tick int) []effect {
		v.tick = v.replay.t0 + r.start + tick
		v.rebuild()
		return v.effects()
	}
	// The shots one by one: shot k's bullet alone of its kind at shotAt(k),
	// its impact among those playing as it lands.
	gun := planetGun(game.OrbitalMissileBase)
	bullet := func(k int) effect {
		t.Helper()
		var bs []effect
		for _, e := range at(shotAt(k)) {
			if e.kind == fxBullet {
				bs = append(bs, e)
			}
		}
		if len(bs) != 1 {
			t.Fatalf("shot %d: %d bullets", k, len(bs))
		}
		return bs[0]
	}
	if b := bullet(0); b.from == nil || b.from.planet != p || b.fromPos != p.Pos || b.bank != bulletBank(gun) || b.ship == nil || b.ship.ship != raider {
		t.Errorf("the planet's bullet: from %v at %v, bank %s, to %v", b.from, b.fromPos, b.bank, b.ship)
	}
	if b := bullet(1); b.ship == nil || b.ship.planet != p || b.toPos != p.Pos || b.bank != bulletBank(game.MassBarrageGun) {
		t.Errorf("the bullet at the shield: to %v at %v", b.ship, b.toPos)
	}
	if b := bullet(2); b.ship == nil || b.ship.ship != colony || b.toPos != ring {
		t.Errorf("the bullet at the ship in orbit: to %v at %v, want its ring slot %v", b.ship, b.toPos, ring)
	}
	landed := func(k int) effect {
		t.Helper()
		fx := at(landsAt(k))
		bank, _ := impactOf(r.log.Shots[k])
		for _, e := range fx {
			if e.kind == fxImpact && e.bank == bank && e.step == 0 {
				return e
			}
		}
		t.Fatalf("shot %d: no %s starting among %+v", k, bank, fx)
		return effect{}
	}
	if e := landed(0); e.bank != explBank(gun) || e.ship.ship != raider {
		t.Errorf("the planet's impact %+v", e)
	}
	if e := landed(2); e.ship.ship != colony {
		t.Errorf("the impact in orbit %+v", e)
	}
	if e := landed(1); e.ship.planet != p {
		t.Errorf("the impact on the shield %+v", e)
	}
	after := func(name string, kind fxKind) {
		t.Helper()
		list := v.drawList()
		i := slices.IndexFunc(list, func(it drawItem) bool { return it.body != nil && it.body.kind == planetBody && it.body.planet == p })
		if i < 0 || i+1 >= len(list) || list[i+1].fx == nil || list[i+1].fx.kind != kind || list[i+1].fx.ship.planet != p {
			t.Errorf("%s: not drawn right after the planet", name)
		}
	}
	after("the impact on the shield", fxImpact)
	fx := at(r.blastStart())
	if len(fx) != 1 || fx[0].kind != fxRaze || fx[0].bank != destroyBank || fx[0].step != 0 || fx[0].ship == nil || fx[0].ship.planet != p || fx[0].pos != pb.pos {
		t.Fatalf("the razing: %+v", fx)
	}
	after("the razing's blast", fxRaze)

	// Drawn with the game's art, every tick: no nil shooter or target
	// dereferenced.
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a.a = as
	sc := ebiten.NewImage(screenW, screenH)
	v.psel = p
	for tk := range v.replay.length() {
		v.tick = v.replay.t0 + tk
		v.rebuild()
		v.Render(a, sc)
		v.Text(a, sc)
	}
}

// The Razed event reads in the log as a Destroyed does (its Text) and
// opens its planet, as any event with a planet.
func TestLogRazed(t *testing.T) {
	a, v, _, _, _ := panelFixture(t)
	home := a.e.Colonies[0]
	ev := game.Event{Kind: game.Razed, Item: game.OrbitalShield, Tech: game.NoTech, Star: v.star, Planet: home.PlanetIdx, Text: "Orbital Shield lost on X"}
	a.log = append(a.log, logEntry{Day: a.e.Day, Ev: ev})
	if !linked(ev) {
		t.Error("a razing is not linked")
	}
	ss := newLogScreen().open(a, len(a.log)-1)
	if len(ss) != 2 {
		t.Fatalf("opens %d screens", len(ss))
	}
	sys, ok1 := ss[0].(*systemScreen)
	pl, ok2 := ss[1].(*planetScreen)
	if !ok1 || !ok2 || sys.star != v.star || pl.star != v.star || pl.idx != home.PlanetIdx {
		t.Errorf("opens %#v %#v", ss[0], ss[1])
	}
}
