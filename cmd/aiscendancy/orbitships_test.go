package main

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The planet screen's orbit: the ships in the free slots, their
// selection and actions; Invade's texts; a capture under open screens.

// invaderDesign carries n Invasion Modules (Spawn loads them as Invaders).
func invaderDesign(n int) game.Design {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor}
	for range n {
		parts = append(parts, game.InvasionModule)
	}
	return game.Design{Name: "Invader", Hull: game.Medium, Parts: parts}
}

// invadeApp is seed 1 with two empires (Snovemdomas, Orfa), the player's
// invader (2 modules) in orbit of empire 1's home colony, its planet's
// surface shields cleared (the capture must not depend on the roll), the
// player knowing that star; the planet screen of that colony entered.
func invadeApp(t *testing.T) (a *app, s *planetScreen, inv *game.Starship, col *game.Colony) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a = testApp(g)
	col = g.Empires[1].Colonies[0]
	for _, sq := range col.Planet.Surface {
		if _, ok := game.ShieldWorth[sq.Item]; ok {
			sq.Item = game.NoItem
		}
	}
	a.e.Visit(col.Star)
	atWar(t, g, 0, 1)
	inv = a.e.Spawn(invaderDesign(2), col.Star, col.PlanetIdx)
	s = newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	return a, s, inv, col
}

// atWar makes empires a and b meet then puts them at war (invasions and
// fights need War: game/diplomacy.go), the reports dropped.
func atWar(t *testing.T, g *game.Game, a, b int) {
	t.Helper()
	g.Contact(a, b)
	if _, err := g.DeclareWar(a, b); err != nil {
		t.Fatal(err)
	}
}

// orbitShips: the player's ships in this orbit first (Ships order), then
// the Sightings, in the free slots in slot order (a structure, the
// rising project and a queued one keep their slot, moving only the ship
// that sat there: the others keep theirs); none of another orbit or in a
// lane.
func TestOrbitShipsSlots(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	e, o := g.Empires[0], g.Empires[1]
	col := e.Colonies[0]
	p := col.Planet
	p.Orbit[1] = game.Shipyard                          // a structure between free slots
	theirs := o.Spawn(foeShip, col.Star, col.PlanetIdx) // first in Game order: still after the player's
	a1 := e.Spawn(fixtureShip, col.Star, col.PlanetIdx)
	a2 := e.Spawn(warship, col.Star, col.PlanetIdx)
	e.Spawn(fixtureShip, col.Star, col.PlanetIdx).Lane = g.Cluster.Stars[col.Star].Lanes[0]
	other := -1
	for i := range g.Cluster.Stars[col.Star].System.Planets {
		if i != col.PlanetIdx {
			other = i
		}
	}
	e.Spawn(fixtureShip, col.Star, other)
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	s.Enter(a)
	var free []int
	for k, it := range p.Orbit {
		if it == game.NoItem {
			free = append(free, k)
		}
	}
	if len(free) == game.OrbitSlots || len(free) < 3 {
		t.Fatalf("free slots %v: the structure case is not walked", free)
	}
	got := s.orbitShips(a)
	if len(got) != 3 || got[0].own != a1 || got[1].own != a2 || got[2].own != nil || got[2].empire != 1 || got[2].species != o.Species || got[2].hull != theirs.Design.Hull {
		t.Fatalf("orbit ships %+v", got)
	}
	for i, os := range got {
		if os.slot != free[i] || p.Orbit[os.slot] != game.NoItem {
			t.Errorf("ship %d in slot %d, want free slot %d", i, os.slot, free[i])
		}
	}
	// The colony's orbital project rising in ship 0's slot moves it to the
	// first slot left, the others stay.
	col.Project = &game.Project{Item: game.Shipyard, Orbital: true, Slot: free[0]}
	if got := s.orbitShips(a); got[0].slot != free[3] || got[1].slot != free[1] || got[2].slot != free[2] {
		t.Errorf("with a project in slot %d: slots %d %d %d", free[0], got[0].slot, got[1].slot, got[2].slot)
	}
	col.Project = nil
	// A queued orbital structure in ship 1's slot: the same.
	col.Queue = []game.Queued{{Item: game.OrbitalDocks, Orbital: true, Slot: free[1]}}
	if got := s.orbitShips(a); got[0].slot != free[3] || got[1].slot != free[0] || got[2].slot != free[2] {
		t.Errorf("with docks queued in slot %d: slots %d %d %d", free[1], got[0].slot, got[1].slot, got[2].slot)
	}
	col.Queue = nil
	// A ship leaving moves no other.
	before := s.orbitShips(a)
	a1.Orbit = -1
	if got := s.orbitShips(a); len(got) != 2 || got[0].own != a2 || got[0].slot != before[1].slot || got[1].slot != before[2].slot {
		t.Errorf("after a leave: %+v, before %+v", got, before)
	}
	a1.Orbit = col.PlanetIdx
	// More ships than free slots: the rest is not drawn.
	for range game.OrbitSlots {
		e.Spawn(fixtureShip, col.Star, col.PlanetIdx)
	}
	if n := len(s.orbitShips(a)); n != len(free) {
		t.Errorf("%d drawn for %d free slots", n, len(free))
	}
	if (&planetScreen{star: col.Star, idx: col.PlanetIdx, planet: p}).orbitShips(&app{cl: g.Cluster}) != nil {
		t.Error("ships without a game")
	}
}

// shipPress opens sh's dialog and presses k there, the refusal returned.
func shipPress(a *app, s *planetScreen, sh *game.Starship, k dlgAct) error {
	s.openShipDlg(sh)
	_, err := s.press(a, k)
	return err
}

// shipDlgWant is the ship dialog's bar from the model's verdicts: Leave
// Orbit unless refitted, the first of Refit (orbiting the player's
// docks, not refitted: the predicate restated), Create Colony,
// Invade, Abandon (not refitted), Load and Unload (berths over a
// colony of the player's), then Show and Cancel.
func shipDlgWant(a *app, sh *game.Starship) []dlgAct {
	refitted := a.e.Refitting(sh) != nil
	var out []dlgAct
	if !refitted {
		out = append(out, dlgLeaveOrbit)
	}
	switch {
	case orbitColony(a.e, sh) != nil && orbitColony(a.e, sh).Planet.HasDocks() && !refitted:
		out = append(out, dlgRefit)
	case a.e.CanFound(sh) == nil:
		out = append(out, dlgCreateColony)
	case sh.Invaders > 0 && a.e.Planet(sh.Star, sh.Orbit).Owner >= 0 && a.e.Planet(sh.Star, sh.Orbit).Owner != a.e.Index:
		out = append(out, dlgInvade)
	case !refitted:
		out = append(out, dlgAbandon)
	}
	if sh.Berths() > 0 && a.e.Colony(sh.Star, sh.Orbit) != nil {
		out = append(out, dlgLoad, dlgUnload)
	}
	return append(out, dlgShow, dlgCancel)
}

// pickShip hovers o's slot and left-clicks it as Update would.
func pickShip(t *testing.T, a *app, s *planetScreen, o orbitShip) (screen, error) {
	t.Helper()
	x, y := slotCentre(o.slot)
	s.hoverAt(a, x, y)
	if h, ok := s.hoveredShip(a); !ok || h != o {
		t.Fatalf("hover at slot %d: %+v %v", o.slot, h, ok)
	}
	return s.leftClick(a)
}

// A click on the player's ship in its slot selects it and opens its
// dialog, every time; another empire's does nothing. The bar follows the
// ship (Invade for the invader, Abandon for the colony ship over the
// foe's colony and for the Gunboat); a click outside the box or Esc
// closes it, the selection kept; the dialog stays up with no colony of
// the player's here, and closes with the selection once the ship left.
func TestOrbitShipSelect(t *testing.T) {
	a, s, inv, col := invadeApp(t)
	g := a.g
	foe := g.Empires[1].Spawn(foeShip, col.Star, col.PlanetIdx)
	colony := a.e.Spawn(fixtureShip, col.Star, col.PlanetIdx)
	gun := a.e.Spawn(warship, col.Star, col.PlanetIdx)
	os := s.orbitShips(a)
	if len(os) != 4 || os[3].own != nil || os[3].empire != 1 || os[3].hull != foe.Design.Hull {
		t.Fatalf("orbit %+v", os)
	}
	if pickShip(t, a, s, os[3]); s.dlg != nil {
		t.Errorf("the foreign ship: dialog %+v", s.dlg)
	}
	if !strings.Contains(s.slotShipText(a, os[3]), "Orfa ship") {
		t.Errorf("foreign slot text %q", s.slotShipText(a, os[3]))
	}
	foe.Orbit = -1 // a hostile ship in orbit guards the planet: Invade would not be offered
	if a.e.CanInvade(inv) != nil || a.e.CanFound(colony) == nil || a.e.CanInvade(colony) == nil || a.e.CanInvade(gun) == nil {
		t.Fatalf("verdicts: invade %v, found %v", a.e.CanInvade(inv), a.e.CanFound(colony))
	}
	acts := map[*game.Starship][]dlgAct{
		inv:    {dlgLeaveOrbit, dlgInvade, dlgShow, dlgCancel},
		colony: {dlgLeaveOrbit, dlgAbandon, dlgShow, dlgCancel},
		gun:    {dlgLeaveOrbit, dlgAbandon, dlgShow, dlgCancel},
	}
	for i, sh := range []*game.Starship{inv, colony, gun} {
		for again := range 2 { // selected already: the dialog opens all the same
			if push, err := pickShip(t, a, s, os[i]); push != nil || err != nil {
				t.Fatalf("%s: push %T, err %v", sh.Name, push, err)
			}
			if s.dlg == nil || s.dlg.ship != sh || !slices.Equal(s.dlgActs(a), acts[sh]) || !slices.Equal(acts[sh], shipDlgWant(a, sh)) {
				t.Errorf("%s (%d): dialog %+v, acts %v", sh.Name, again, s.dlg, s.dlgActs(a))
			}
			if got := s.dlg.title(s.planet); got != `Ship "`+sh.Name+`"` {
				t.Errorf("title %q", got)
			}
			s.refresh(a) // no colony of the player's here: the ship's dialog stays
			if s.col != nil || s.dlg == nil {
				t.Fatalf("refresh: col %v closed the ship's dialog", s.col)
			}
			if s.dlgClick(a, 1, 1); s.dlg != nil { // outside the box
				t.Errorf("click outside: dialog %+v", s.dlg)
			}
		}
		if !strings.Contains(s.slotShipText(a, os[i]), "(click: its dialog)") {
			t.Errorf("slot text %q", s.slotShipText(a, os[i]))
		}
	}
	s.openShipDlg(gun)
	if !s.cancel() || s.dlg != nil {
		t.Errorf("Esc: dialog %+v", s.dlg)
	}
	// The order line under the picture: shipLine's, "in orbit" without an
	// order, the Invade order's text with one.
	s.openShipDlg(gun)
	if got := s.dlgOrderLine(a); got != shipLine(a, gun) || got != "in orbit" {
		t.Errorf("gunboat's order line %q, shipLine %q", got, shipLine(a, gun))
	}
	if err := a.e.Order(inv, game.Order{Kind: game.Invade}); err != nil {
		t.Fatal(err)
	}
	s.openShipDlg(inv)
	if got := s.dlgOrderLine(a); got != shipLine(a, inv) || got != shipOrder(a.e, inv) || got == "in orbit" {
		t.Errorf("invader's order line %q, shipLine %q", got, shipLine(a, inv))
	}
	inv.Order = game.Order{}
	s.dlg = nil
	// The hover lines.
	for k, want := range map[dlgAct]string{
		dlgInvade:     "Invade: " + inv.Name + " lands now, the fewest of its 2 invasion modules that beat the shields",
		dlgLeaveOrbit: "Leave Orbit: " + inv.Name + " leaves the orbit now, to be moved on the system screen",
		dlgAbandon:    "Abandon: " + inv.Name + " is scrapped now, nothing back",
		dlgShow:       "Show: " + inv.Name + "'s parts, read-only",
		dlgCancel:     "",
	} {
		if got := s.shipDlgText(a, inv, k); got != want {
			t.Errorf("%s hover %q, want %q", dlgLabels[k], got, want)
		}
	}
	// Show, its button clicked: its parts, read-only, the dialog closed,
	// the selection kept.
	pickShip(t, a, s, os[2])
	bs := s.dlgButtons(a)
	k := slices.IndexFunc(bs, func(b dlgButton) bool { return b.act == dlgShow })
	if k < 0 {
		t.Fatalf("no Show button: %+v", bs)
	}
	push, err := s.dlgClick(a, float64(bs[k].r.X0+2), float64(bs[k].r.Y0+2))
	v, ok := push.(*designScreen)
	if err != nil || !ok || v.ship != gun || s.dlg != nil {
		t.Fatalf("Show: push %T, err %v, dialog %+v", push, err, s.dlg)
	}
	a.push(v)
	if v.t.Name != gun.Name || len(v.t.Parts) != len(gun.Design.Parts) || v.d.Cost() != gun.Design.Cost() {
		t.Errorf("view: %q %d parts, cost %d", v.t.Name, len(v.t.Parts), v.d.Cost())
	}
	_, verdict, ok := v.titleLines(a.e)
	if !ok || verdict != fmt.Sprintf("hull %d/%d  energy %d/%d", gun.Hull, gun.MaxHull(), gun.Power, gun.Design.PowerPerDay()) {
		t.Errorf("view verdict %q %v", verdict, ok)
	}
	v.cat = game.Weapon
	if l := v.list(a.e.Research); len(l) != 1 || l[0].Best || game.Gizmos[l[0].Gizmo].Cat != game.Weapon {
		t.Errorf("view list of weapons: %+v", l)
	}
	// A spent Colonizer is not aboard any more:
	// the view shows Cargo of them, not the Design's list.
	colony.Cargo--
	cv := newShipViewScreen(colony)
	var aboard int
	for _, p := range cv.t.Parts {
		if p.Gizmo == game.Colonizer {
			aboard++
		}
	}
	if aboard != colony.Cargo || len(cv.t.Parts) != len(colony.Design.Parts)-1 || colony.Design.Colonizers() != colony.Cargo+1 {
		t.Errorf("view of a ship with a spent Colonizer: %d aboard, %d parts, cargo %d", aboard, len(cv.t.Parts), colony.Cargo)
	}
	// Read-only: no add, no remove, no save, no name edit, no Load/Build.
	n := len(v.t.Parts)
	v.cat = game.Drive
	lc := listCell(0)
	v.leftClick(a, float64(lc.X0+1), float64(lc.Y0+1))
	sx, sy := slotAnchor(v.t.Hull, 0)
	v.rightClick(a.e.Research, sx+1, sy+1)
	v.handleKeys(a, []rune("x"), desKeys{f2: true})
	v.handleKeys(a, nil, desKeys{s: true})
	v.hoverAt(a, float64(desLoad.X0+1), float64(desLoad.Y0+1))
	if len(v.t.Parts) != n || v.editing || v.t.Name != gun.Name || v.hover == hoverLoad || slices.ContainsFunc(a.e.Templates, func(t game.Template) bool { return t.Name == gun.Name }) {
		t.Errorf("read-only broken: %d parts (was %d), editing %v, name %q, hover %v", len(v.t.Parts), n, v.editing, v.t.Name, v.hover)
	}
	if err := a.pop(); err != nil {
		t.Fatal(err)
	}
	// The ship leaves the orbit under its dialog: both close.
	s.openShipDlg(colony)
	colony.Orbit = -1
	s.refresh(a)
	if s.dlg != nil {
		t.Errorf("a ship out of the orbit: dialog %+v", s.dlg)
	}
}

// Abandon asks first: the box closes on the question, No (or Esc) keeps
// the ship, Yes scraps it now (Empire.Abandon): gone from Ships and
// drawn no more; refused under a refit (the rule's error before any
// question, the box and the ship kept).
func TestOrbitShipAbandon(t *testing.T) {
	a, s, _, col := invadeApp(t)
	gun := a.e.Spawn(warship, col.Star, col.PlanetIdx)
	if want := shipDlgWant(a, gun); !slices.Contains(want, dlgAbandon) {
		t.Fatalf("no Abandon for the gunboat: %v", want)
	}
	ask := func(what string) {
		t.Helper()
		if err := shipPress(a, s, gun, dlgAbandon); err != nil || !slices.Contains(a.e.Ships, gun) || s.dlg != nil ||
			s.confirm == nil || s.scrapAsk != gun || !strings.Contains(s.confirm.lines[0], gun.Name) {
			t.Fatalf("%s: %v, in Ships %v, dialog %+v, question %+v", what, err, slices.Contains(a.e.Ships, gun), s.dlg, s.confirm)
		}
	}
	ask("press")
	if s.answerScrap(a, false); s.confirm != nil || s.scrapAsk != nil || !slices.Contains(a.e.Ships, gun) {
		t.Fatalf("no: question %v, in Ships %v", s.confirm, slices.Contains(a.e.Ships, gun))
	}
	ask("press again")
	if !s.cancel() || s.confirm != nil || s.scrapAsk != nil || !slices.Contains(a.e.Ships, gun) {
		t.Fatalf("Esc: question %v, in Ships %v", s.confirm, slices.Contains(a.e.Ships, gun))
	}
	ask("press a third time")
	if s.answerScrap(a, true); s.confirm != nil || s.scrapAsk != nil || slices.Contains(a.e.Ships, gun) {
		t.Fatalf("yes: question %v, in Ships %v", s.confirm, slices.Contains(a.e.Ships, gun))
	}
	s.refresh(a)
	for _, o := range s.orbitShips(a) {
		if o.own == gun {
			t.Error("the abandoned ship still drawn")
		}
	}

	a, _, dc, worn, _ := docksApp(t)
	ds := newPlanetScreen(dc.Star, dc.PlanetIdx)
	a.push(ds)
	if d, err := a.e.RefitTarget(worn); err != nil || a.e.Refit(dc, worn, d) != nil {
		t.Fatal(err)
	}
	want := a.e.Abandon(worn)
	if want == nil {
		t.Fatal("the rule scraps a ship under refit")
	}
	if err := shipPress(a, ds, worn, dlgAbandon); err == nil || err.Error() != want.Error() || ds.dlg == nil || !slices.Contains(a.e.Ships, worn) || ds.dlg.ship != worn || ds.confirm != nil {
		t.Errorf("abandon under refit: %v, want %v, dialog %+v", err, want, ds.dlg)
	}
	if got := ds.dlgActs(a); !slices.Equal(got, []dlgAct{dlgShow, dlgCancel}) || !slices.Equal(got, shipDlgWant(a, worn)) {
		t.Errorf("bar under refit %v", got)
	}
}

// Invade from the planet screen resolves at once (Game.InvadeNow): the
// colony is the player's, the modules spent, no order left, the
// capture logged and flashed; greyed and inert over the player's own
// colony. Create Colony likewise: refused on an inhabited planet, accepted
// on a free one.
func TestOrbitShipOrders(t *testing.T) {
	a, s, inv, col := invadeApp(t)
	if got := s.shipDlgText(a, inv, dlgInvade); !strings.Contains(got, "2 invasion modules") {
		t.Errorf("invade hover %q", got)
	}
	day := a.g.Day
	if err := shipPress(a, s, inv, dlgInvade); err != nil || inv.Order.Kind != game.NoOrder || s.dlg != nil {
		t.Fatalf("invade: %v, order %+v, dialog %+v", err, inv.Order, s.dlg)
	}
	if col.Planet.Owner != a.player || s.col != col || inv.Invaders != 1 || a.g.Day != day { // no shields: one lands
		t.Fatalf("not captured at once: owner %d, screen colony %v, %d modules, day %d", col.Planet.Owner, s.col, inv.Invaders, a.g.Day)
	}
	if n := len(a.log); n == 0 || a.log[n-1].Ev.Kind != game.Invaded || s.msg != a.log[n-1].Ev.Text {
		t.Errorf("log %v, message %q", a.log, s.msg)
	}

	home := a.e.Colonies[0]
	mine := a.e.Spawn(invaderDesign(1), home.Star, home.PlanetIdx)
	hs := newPlanetScreen(home.Star, home.PlanetIdx)
	a.push(hs)
	wantErr := a.e.Order(mine, game.Order{Kind: game.Invade})
	if wantErr == nil {
		t.Fatal("the rule accepts invading the player's own colony")
	}
	if err := shipPress(a, hs, mine, dlgInvade); err != nil || mine.Order.Kind != game.NoOrder || hs.dlg == nil {
		t.Errorf("invade our own: %v (greyed: inert), order %+v, dialog %+v", err, mine.Order, hs.dlg)
	}
	if slices.Contains(hs.dlgActs(a), dlgInvade) { // and the button is not even offered
		t.Errorf("Invade offered over our own colony: %v", hs.dlgActs(a))
	}
	settler := a.e.Spawn(fixtureShip, home.Star, home.PlanetIdx)
	if err := shipPress(a, hs, settler, dlgCreateColony); err == nil || err.Error() != a.e.CanFound(settler).Error() || hs.settler != nil || hs.dlg == nil {
		t.Errorf("colonise the home: %v, settler %v", err, hs.settler)
	}
	if slices.Contains(hs.dlgActs(a), dlgCreateColony) {
		t.Errorf("Create Colony offered over our own colony: %v", hs.dlgActs(a))
	}
	free := -1
	for i, p := range a.cl.Stars[home.Star].System.Planets {
		if p.Owner < 0 {
			free = i
			break
		}
	}
	fs := newPlanetScreen(home.Star, free)
	a.push(fs)
	settler.Orbit = free
	fs.refresh(a)
	if err := shipPress(a, fs, settler, dlgCreateColony); err != nil || fs.settler != settler || fs.dlg != nil || settler.Order.Kind != game.NoOrder {
		t.Fatalf("colonise %d: %v, settler %v, dialog %+v, order %+v", free, err, fs.settler, fs.dlg, settler.Order)
	}
	if got, want := fs.shipDlgText(a, settler, dlgCreateColony), "Create Colony: "+settler.Name+" founds a colony now, on the square you click next"; got != want {
		t.Errorf("hover %q", got)
	}
	if !fs.cancel() || fs.settler != nil {
		t.Error("Esc does not give up Create Colony")
	}
	if err := shipPress(a, fs, settler, dlgCreateColony); err != nil || fs.settler != settler {
		t.Fatalf("colonise again: %v", err)
	}
	// The next click on a square founds there, now, a black square too
	// (Planet.CanSettle); the colony can start a project the same day.
	p := fs.planet
	at, black := game.Cell{}, false
	for _, c := range p.Cells() {
		if p.CanSettle(c) && (!black || p.Surface[c].Kind == game.Black) {
			at, black = c, p.Surface[c].Kind == game.Black
		}
	}
	if !p.CanSettle(at) {
		t.Fatalf("%s: no square to settle", p.Name)
	}
	fs.cell, fs.cellOK, fs.slotOK = at, true, false
	cargo := settler.Cargo
	if _, err := fs.leftClick(a); err != nil {
		t.Fatalf("found at %v: %v", at, err)
	}
	if fs.col == nil || fs.settler != nil || p.Surface[at].Item != game.ColonyBase || settler.Cargo != cargo-1 {
		t.Fatalf("after the click: col %v, settler %v, square %v, cargo %d", fs.col, fs.settler, p.Surface[at], settler.Cargo)
	}
	startable := false
	for _, c := range p.Cells() {
		startable = startable || fs.col.CanStart(a.e.Research, game.Laboratory, c) == nil || fs.col.CanStart(a.e.Research, game.Factory, c) == nil
	}
	if !startable {
		t.Errorf("the new colony can start nothing today (free %d)", fs.col.Free())
	}
}

// At the colony's Orbital Docks the dialog's middle button is Refit for
// every ship of the player's there, the template's own ship too (what a
// refit changes is the design screen's verdict; Abandon is not offered
// there); its hover is refitFoot's; the press closes the dialog and gives
// the refit mode's design screen on the ship; refitted, only Show and
// Cancel are left.
func TestOrbitShipRefit(t *testing.T) {
	a, _, col, worn, done := docksApp(t)
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	for _, sh := range []*game.Starship{done, worn} {
		s.openShipDlg(sh)
		if got := s.dlgActs(a); !slices.Equal(got, []dlgAct{dlgLeaveOrbit, dlgRefit, dlgShow, dlgCancel}) || !slices.Equal(got, shipDlgWant(a, sh)) {
			t.Fatalf("%s: acts %v, no Refit in the middle at the docks", sh.Name, got)
		}
		if got, foot := s.shipDlgText(a, sh, dlgRefit), refitFoot(a.e, sh); got == "" || got != foot {
			t.Errorf("refit hover %q, foot %q", got, foot)
		}
	}
	next, err := s.press(a, dlgRefit)
	ds, ok := next.(*designScreen)
	if err != nil || !ok || ds.refit != worn || ds.col != col || s.dlg != nil || a.e.Refitting(worn) != nil {
		t.Fatalf("refit: %v, screen %T, dialog %+v", err, next, s.dlg)
	}
	d, err := a.e.RefitTarget(worn)
	if err != nil || a.e.Refit(col, worn, d) != nil {
		t.Fatal(err)
	}
	if s.openShipDlg(worn); !slices.Equal(s.dlgActs(a), []dlgAct{dlgShow, dlgCancel}) {
		t.Errorf("refitted: acts %v, want Show and Cancel only", s.dlgActs(a))
	}
	// Pressed anyway (the dialog was up when the refit started): refused.
	if next, err := s.press(a, dlgRefit); err == nil || next != nil || s.dlg == nil {
		t.Errorf("Refit under refit: %v, screen %T", err, next)
	}
}

// The day's close resolves the invasion: the log gets the Invaded event
// (linked: it opens the system then the planet), the open planet screen
// now shows the player's colony (picker and question closed), the system
// screen keeps its selected planet; the loser's home flag moves off the
// lost home (to its oldest colony left: none here, so no flag).
func TestCaptureUnderScreens(t *testing.T) {
	a, s, inv, col := invadeApp(t)
	loser := a.g.Empires[1]
	v := newSystemScreen(col.Star)
	a.stack = nil
	a.push(v)
	v.psel = col.Planet
	a.push(s)
	if s.other != col || s.col != nil {
		t.Fatalf("before: col %v other %v", s.col, s.other)
	}
	s.pick = &picker{} // a foreign planet opens none; it must close all the same
	if fl := homeFlags(a.e, a.g); len(fl) != 2 || fl[1].star != col.Star {
		t.Fatalf("flags before %+v", fl)
	}
	if err := shipPress(a, s, inv, dlgInvade); err != nil {
		t.Fatal(err)
	}
	s.openShipDlg(inv) // the ship's dialog survives the colony changing hands
	a.endDay()         // the loser's defeat is the close's
	k := slices.IndexFunc(a.log, func(x logEntry) bool { return x.Ev.Kind == game.Invaded })
	if k < 0 || col.Planet.Owner != a.player || len(loser.Colonies) != 0 {
		t.Fatalf("no capture: log %v, owner %d", a.log, col.Planet.Owner)
	}
	ev := a.log[k].Ev
	l := &logScreen{}
	// The loser's last colony gone, it is defeated too: its Defeated
	// follows the Invaded in the log.
	last := slices.IndexFunc(a.log, func(x logEntry) bool { return x.Ev.Kind == game.Invaded })
	if last < 0 || a.log[last].Ev != ev || !linked(ev) || loser.Alive() {
		t.Fatalf("log %v, loser alive %v", a.log, loser.Alive())
	}
	if scs := l.open(a, last); len(scs) != 2 || scs[0].(*systemScreen).star != col.Star || scs[1].(*planetScreen).idx != col.PlanetIdx {
		t.Errorf("the Invaded line opens %#v", scs)
	}
	s.refresh(a)
	if s.col != col || s.other != nil || s.readOnly() != "" || s.pick != nil || s.dlg == nil || s.dlg.ship != inv {
		t.Errorf("after: col %v other %v pick %v dialog %+v", s.col, s.other, s.pick, s.dlg)
	}
	v.rebuild()
	if v.psel != col.Planet || v.pselColony() != col {
		t.Errorf("system screen: psel %v, colony %v", v.psel, v.pselColony())
	}
	if fl := homeFlags(a.e, a.g); len(fl) != 1 || fl[0].star != a.e.Colonies[0].Star {
		t.Errorf("flags after %+v: the empire without a colony has none", fl)
	}
	// Empire 1 founds again elsewhere: its flag is that colony's.
	far := -1
	for i, p := range a.cl.Stars[a.e.Colonies[0].Star].System.Planets {
		if p.Owner < 0 {
			far = i
		}
	}
	if err := loser.Found(loser.Spawn(fixtureShip, a.e.Colonies[0].Star, far)); err != nil {
		t.Fatal(err)
	}
	if fl := homeFlags(a.e, a.g); len(fl) != 2 || fl[1].star != loser.Colonies[0].Star {
		t.Errorf("flags after a new colony %+v", fl)
	}
}

// The player's colony captured under its open planet screen: the
// project's cancel question and the structure dialog close, the screen
// turns read-only on the new owner's colony; an automated structure
// stays automated (the colony is taken as it is, its species the
// captor's).
func TestColonyLostUnderScreen(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	col := a.e.Colonies[0]
	for _, sq := range col.Planet.Surface {
		if _, ok := game.ShieldWorth[sq.Item]; ok {
			sq.Item = game.NoItem
		}
	}
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	s.confirm = &prompt{}
	base := game.Cell{I: game.Centre, J: game.Centre}
	s.dlg = &structDlg{cell: base}
	col.Planet.Surface[base].Automated = true
	atWar(t, g, 0, 1)
	foe := g.Empires[1].Spawn(invaderDesign(1), col.Star, col.PlanetIdx)
	if _, err := g.Invade(g.Empires[1], foe); err != nil || col.Planet.Owner != 1 {
		t.Fatalf("capture: %v, owner %d", err, col.Planet.Owner)
	}
	s.refresh(a)
	if s.col != nil || s.other != col || s.confirm != nil || s.dlg != nil || s.readOnly() != "not your planet" {
		t.Errorf("after: col %v other %v confirm %v dialog %v", s.col, s.other, s.confirm, s.dlg)
	}
	if sq := col.Planet.Surface[base]; sq.Item != game.ColonyBase || !sq.Automated {
		t.Errorf("captured base %+v, want the Colony Base automated", *sq)
	}
	if pop, _ := s.panelTexts(); !strings.Contains(pop, game.Orfa.String()) {
		t.Errorf("POP text %q: the colony takes its captor's species", pop)
	}
}

// Rendering through a capture, with the game's art: the planet screen
// with a ship selected in its slot (its dialog up) and the system screen
// with the planet selected, before and after the day; no panic.
func TestCaptureRenders(t *testing.T) {
	a, s, inv, col := invadeApp(t)
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
	v := newSystemScreen(col.Star)
	v.Enter(a)
	v.psel = col.Planet
	s.openShipDlg(inv)
	sc := ebiten.NewImage(screenW, screenH)
	draw := func() {
		s.refresh(a)
		s.Render(a, sc)
		s.Text(a, sc)
		v.rebuild()
		v.Render(a, sc)
		v.Text(a, sc)
	}
	draw()
	if err := shipPress(a, s, inv, dlgInvade); err != nil {
		t.Fatal(err)
	}
	draw()
	s.openShipDlg(inv)
	a.endDay()
	draw()
	if col.Planet.Owner != a.player {
		t.Fatalf("no capture: owner %d", col.Planet.Owner)
	}
}

// The fullest ship dialog bar fits: Leave Orbit, the widest middle
// button, Load, Unload, Show, Cancel, each label in its own cell
// (dlgButtons: the label's width plus a share of the rest, 4*zoom inset
// a side).
func TestShipDlgLabelsFit(t *testing.T) {
	mid := 0.0
	for _, k := range []dlgAct{dlgCreateColony, dlgInvade, dlgRefit, dlgAbandon} {
		mid = max(mid, ui.Width(dlgLabels[k], bigFace))
	}
	w := mid
	acts := []dlgAct{dlgLeaveOrbit, dlgLoad, dlgUnload, dlgShow, dlgCancel}
	for _, k := range acts {
		w += ui.Width(dlgLabels[k], bigFace)
	}
	if room := float64(dlgBar.X1 - dlgBar.X0 - (len(acts)+1)*8*zoom); w > room {
		t.Errorf("labels %.0f px, the bar %.0f", w, room)
	}
}

// Leave Orbit: offered first for an own ship in orbit (not while refitted),
// it puts the ship in space at once and for free, at game.LeavePoint (or
// FreeSpot near it), gone from the planet screen's orbit, a body in space
// of the system view under it, selected there.
func TestOrbitShipLeave(t *testing.T) {
	a, v, col, worn, _ := docksApp(t)
	v.sel = nil
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(v)
	a.push(s)
	s.openShipDlg(worn)
	if acts := s.dlgActs(a); len(acts) == 0 || acts[0] != dlgLeaveOrbit {
		t.Fatalf("acts %v: Leave Orbit not first", acts)
	}
	if got := s.shipDlgText(a, worn, dlgLeaveOrbit); got != "Leave Orbit: "+worn.Name+" leaves the orbit now, to be moved on the system screen" {
		t.Errorf("hover %q", got)
	}
	// Held by a refit: no Leave Orbit.
	if d, err := a.e.RefitTarget(worn); err != nil || a.e.Refit(col, worn, d) != nil {
		t.Fatal(err)
	}
	if s.openShipDlg(worn); slices.Contains(s.dlgActs(a), dlgLeaveOrbit) {
		t.Error("Leave Orbit offered while refitted")
	}
	col.Project = nil
	if a.e.Refitting(worn) != nil {
		t.Fatal("refit not cancelled")
	}

	want := game.Order{Kind: game.GoToPoint, Pos: game.LeavePoint(a.cl, col.Star, col.PlanetIdx)}
	power := worn.Power
	if _, err := s.press(a, dlgLeaveOrbit); err != nil || s.dlg != nil || !worn.InSpace() || worn.Order != (game.Order{}) || worn.Power != power {
		t.Fatalf("leave: %v, orbit %d lane %d, order %+v, power %v of %v", err, worn.Orbit, worn.Lane, worn.Order, worn.Power, power)
	}
	if v.sel != worn {
		t.Errorf("the system screen selects %v", v.sel)
	}
	dx, dy, dz := worn.Pos.X-want.Pos.X, worn.Pos.Y-want.Pos.Y, worn.Pos.Z-want.Pos.Z
	if d := math.Sqrt(dx*dx + dy*dy + dz*dz); d > 4*game.ShipGap {
		t.Errorf("stands %.3f from the leave point", d)
	}
	s.openShipDlg(worn) // a dialog on a ship out of the orbit closes
	if s.refresh(a); s.dlg != nil {
		t.Error("the ship's dialog is still up out of the orbit")
	}
	for _, o := range s.orbitShips(a) {
		if o.own == worn {
			t.Error("still drawn in an orbit slot")
		}
	}
	v.rebuild()
	if !slices.ContainsFunc(v.bodies, func(b body) bool { return b.ship == worn && !b.orbiting }) {
		t.Error("the ship is not a body in space in the system view")
	}
}

// Over an uninhabited planet (no colony on the screen) the click opens
// the ship's dialog, Create Colony in the middle; its press sets
// settling. Create Colony is the pressed ship's: another ship's dialog
// gives it up, so the next square click never founds with a ship the
// player did not press it for.
func TestSettlingFollowsTheShip(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	home := a.e.Colonies[0]
	free := -1
	for i, p := range a.cl.Stars[home.Star].System.Planets {
		if p.Owner < 0 {
			free = i
			break
		}
	}
	first := a.e.Spawn(fixtureShip, home.Star, free)
	second := a.e.Spawn(fixtureShip, home.Star, free)
	s := newPlanetScreen(home.Star, free)
	a.push(s)
	os := s.orbitShips(a)
	if len(os) != 2 || os[0].own != first || os[1].own != second {
		t.Fatalf("orbit %+v", os)
	}
	if push, err := pickShip(t, a, s, os[0]); push != nil || err != nil || s.col != nil || s.dlg == nil || s.dlg.ship != first {
		t.Fatalf("click: %T %v, col %v, dialog %+v", push, err, s.col, s.dlg)
	}
	if a.e.CanFound(first) != nil || !slices.Equal(s.dlgActs(a), []dlgAct{dlgLeaveOrbit, dlgCreateColony, dlgShow, dlgCancel}) || !slices.Equal(s.dlgActs(a), shipDlgWant(a, first)) {
		t.Fatalf("acts %v (CanFound %v)", s.dlgActs(a), a.e.CanFound(first))
	}
	if _, err := s.press(a, dlgCreateColony); err != nil || s.settler != first || s.dlg != nil {
		t.Fatalf("colonise: %v, settler %v, dialog %+v", err, s.settler, s.dlg)
	}
	pickShip(t, a, s, os[1])
	s.refresh(a)
	if s.dlg == nil || s.dlg.ship != second || s.settler != nil {
		t.Fatalf("another ship's dialog: %+v, settler %v", s.dlg, s.settler)
	}
	p := s.planet
	for _, c := range p.Cells() {
		if p.CanSettle(c) {
			s.cell, s.cellOK, s.slotOK = c, true, false
			break
		}
	}
	cargo := second.Cargo
	s.leftClick(a)
	if second.Cargo != cargo || p.Owner >= 0 {
		t.Errorf("a square click founded with %s, never pressed", second.Name)
	}
	// The settler leaving the orbit gives Create Colony up.
	if err := shipPress(a, s, second, dlgCreateColony); err != nil || s.settler != second {
		t.Fatalf("colonise with %s: %v", second.Name, err)
	}
	second.Orbit = -1
	if s.refresh(a); s.settler != nil {
		t.Error("settling kept for a ship out of the orbit")
	}
}

// The ship dialog's parts grid: what is aboard (Aboard: the Show screen's
// list) in cell order, then the hull's free slots as diamonds, nothing
// past Slots; a spent Colonizer is one tile fewer. The grid and the
// picture band (with three order lines) lie in the box above the bar,
// side by side; the hit test finds a part at its cell's centre, nothing
// on a free slot, past the hull, or in a structure's dialog.
func TestShipDlgGrid(t *testing.T) {
	a, s, _, col := invadeApp(t)
	for h := range game.NumHulls {
		if n := game.Hull(h).Slots(); n > dlgGridCols*dlgGridCols {
			t.Errorf("%s: %d slots, the grid holds %d", game.Hull(h), n, dlgGridCols*dlgGridCols)
		}
	}
	gun := a.e.Spawn(warship, col.Star, col.PlanetIdx)
	colony := a.e.Spawn(fixtureShip, col.Star, col.PlanetIdx)
	colony.Cargo--
	for _, sh := range []*game.Starship{gun, colony} {
		parts := sh.Aboard().Parts
		cells := shipDlgCells(sh)
		if len(cells) != sh.Design.Hull.Slots() {
			t.Errorf("%s: %d cells for %d slots", sh.Name, len(cells), sh.Design.Hull.Slots())
		}
		for k, c := range cells {
			if c.r != dlgGridCell(k) || c.part != (k < len(parts)) || c.part && c.g != parts[k] {
				t.Errorf("%s cell %d: %+v, aboard %v", sh.Name, k, c, parts)
			}
		}
	}
	var tiles int
	for _, c := range shipDlgCells(colony) {
		if c.part && c.g == game.Colonizer {
			tiles++
		}
	}
	if tiles != colony.Cargo || tiles != colony.Design.Colonizers()-1 {
		t.Errorf("%d Colonizer tiles, cargo %d of %d", tiles, colony.Cargo, colony.Design.Colonizers())
	}

	// Geometry: the constants agree with the box's bar, everything inside.
	if dlgBox.X0+dlgInnerX1 != dlgBar.X1 || dlgBox.X0+dlgInnerX0 != dlgBar.X0 || dlgBox.Y0+dlgBarTop != dlgBar.Y0 {
		t.Fatalf("inner edges %d,%d bar top %d vs bar %+v", dlgInnerX0, dlgInnerX1, dlgBarTop, dlgBar)
	}
	inside := func(what string, r rect) {
		t.Helper()
		if r.X0 < dlgBox.X0 || r.X1 > dlgBox.X1 || r.Y0 < dlgBox.Y0+dlgTitleBottom || r.Y1 > dlgBar.Y0 {
			t.Errorf("%s %+v out of the box %+v / under the title / over the bar %+v", what, r, dlgBox, dlgBar)
		}
	}
	for k := range dlgGridCols * dlgGridCols {
		inside(fmt.Sprintf("cell %d", k), dlgGridCell(k))
	}
	// The same gap left of the picture, between it and the grid, right of
	// the grid (within the division's remainder).
	left, mid, right := dlgPicX0-dlgInnerX0, dlgGridCell(0).X0-dlgBox.X0-dlgPicX1, dlgBar.X1-dlgGridCell(dlgGridCols-1).X1
	if left != dlgGap || mid != dlgGap || right < dlgGap || right > dlgGap+2 {
		t.Errorf("gaps %d %d %d, want %d", left, mid, right, dlgGap)
	}
	lines := 3 * int(smallFace.Size*1.25)
	pic := rect{dlgBox.X0 + dlgPicX0, dlgBox.Y0 + dlgGridY, dlgBox.X0 + dlgPicX1, dlgBox.Y0 + dlgGridY + dlgShipPicH + dlgOrderGap + lines}
	inside("picture and order lines", pic)
	if pic.X1 > dlgGridCell(0).X0 {
		t.Errorf("picture band %+v overlaps the grid at x %d", pic, dlgGridCell(0).X0)
	}

	// The hit test.
	parts := colony.Aboard().Parts
	if len(parts) >= colony.Design.Hull.Slots() || colony.Design.Hull.Slots() >= dlgGridCols*dlgGridCols {
		t.Fatalf("%s: %d aboard, %d slots: no free slot or no cell past the hull", colony.Name, len(parts), colony.Design.Hull.Slots())
	}
	at := func(d *structDlg, k int) (game.Gizmo, bool) { // cell k's centre
		r := dlgGridCell(k)
		return dlgGridAt(d, float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2)
	}
	s.openShipDlg(colony)
	for k, g := range parts {
		if got, ok := at(s.dlg, k); !ok || got != g {
			t.Errorf("cell %d: %v %v, want %v", k, got, ok, g)
		}
	}
	if _, ok := at(s.dlg, len(parts)); ok {
		t.Error("a part on a free slot")
	}
	if _, ok := at(s.dlg, colony.Design.Hull.Slots()); ok {
		t.Error("a part past the hull")
	}
	if _, ok := at(&structDlg{}, 0); ok {
		t.Error("a part in a structure's dialog")
	}
}

// At peace with the planet's owner Invade is shown greyed over the
// foreign colony, its hover the rule's refusal, a press inert.
func TestOrbitShipInvadeGreyed(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	col := g.Empires[1].Colonies[0]
	a.e.Visit(col.Star)
	g.Contact(0, 1) // Peace
	inv := a.e.Spawn(invaderDesign(2), col.Star, col.PlanetIdx)
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	refusal := a.e.CanInvade(inv)
	if refusal == nil {
		t.Fatal("fixture: invading at peace accepted")
	}
	s.openShipDlg(inv)
	if !slices.Contains(s.dlgActs(a), dlgInvade) || !s.dlgOff(a, dlgInvade) {
		t.Fatalf("bar %v, off %v", s.dlgActs(a), s.dlgOff(a, dlgInvade))
	}
	if got := s.shipDlgText(a, inv, dlgInvade); got != "Invade: "+refusal.Error() {
		t.Errorf("hover %q, want the refusal %q", got, refusal)
	}
	if err := shipPress(a, s, inv, dlgInvade); err != nil || inv.Order.Kind != game.NoOrder || s.dlg == nil {
		t.Errorf("greyed press: %v, order %+v, dialog %v", err, inv.Order, s.dlg)
	}
}

// Leave Orbit from the planet screen selects the ship on the system
// screen under it and drops the planet's selection: the fire mode's
// shooter is the ship, not the planet.
func TestLeaveOrbitSelectsShip(t *testing.T) {
	a, s, inv, col := invadeApp(t)
	a.stack = nil // the system screen under the planet's
	v := newSystemScreen(col.Star)
	a.push(v)
	v.psel = col.Planet
	a.push(s)
	if err := s.leave(a, inv); err != nil {
		t.Fatal(err)
	}
	if v.sel != inv || v.psel != nil {
		t.Errorf("after Leave Orbit: the ship selected %v, a planet still selected %v", v.sel == inv, v.psel != nil)
	}
}
