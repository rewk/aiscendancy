package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// dlgHome is the home world's planet screen with a Factory built next to
// the Colony Base, a Shipyard and an Orbital Shield in orbit, people to
// staff a project; f is the Factory's cell.
func dlgHome(t *testing.T) (a *app, s *planetScreen, f game.Cell) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a = testApp(g)
	col := a.e.Colonies[0]
	p := col.Planet
	f, ok := p.BestCell(game.Factory, col.Traits())
	if !ok {
		t.Fatal("no Factory square")
	}
	if err := p.Build(f, game.Factory); err != nil {
		t.Fatal(err)
	}
	for k, it := range []game.Item{game.Shipyard, game.OrbitalShield} {
		p.Orbit[k] = it
	}
	col.Pop = col.Busy() + 2
	s = newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	return a, s, f
}

// dlgHoverCell and dlgHoverSlot set the hover as hoverAt would.
func dlgHoverCell(s *planetScreen, c game.Cell) { s.cell, s.cellOK, s.slotOK = c, true, false }
func dlgHoverSlot(s *planetScreen, k int)       { s.slot, s.slotOK, s.cellOK = k, true, false }

// The dialog's title and buttons per structure, from the model: a
// Factory's Abandon, Automate (once researched), Cancel; an automated
// one's Abandon, Cancel; the Colony Base's no Abandon; a Shipyard's Build
// Ship first; an orbital's no Automate.
func TestStructDlgButtons(t *testing.T) {
	a, s, f := dlgHome(t)
	p, r := s.planet, a.e.Research
	base := game.Cell{I: game.Centre, J: game.Centre}
	open := func(slot int, c game.Cell) []dlgAct {
		t.Helper()
		if slot >= 0 {
			dlgHoverSlot(s, slot)
		} else {
			dlgHoverCell(s, c)
		}
		if push, err := s.clickSlot(a); push != nil || err != nil || s.dlg == nil || s.pick != nil || s.confirm != nil {
			t.Fatalf("click at %v/%d: %#v %v, dialog %v", c, slot, push, err, s.dlg)
		}
		return s.dlgActs(a)
	}
	kind := squareKinds[p.Surface[f].Kind]
	for _, tc := range []struct {
		name    string
		slot    int
		cell    game.Cell
		prep    func()
		title   string
		buttons []dlgAct
	}{
		{"factory unresearched", -1, f, func() {}, "Factory on " + kind + " Square", []dlgAct{dlgAbandon, dlgCancel}},
		{"factory", -1, f, func() { r.Grant(game.Tech(game.Automation.Info().ResearchReq)) },
			"Factory on " + kind + " Square", []dlgAct{dlgAbandon, dlgAutomate, dlgCancel}},
		{"automated", -1, f, func() { p.Surface[f].Automated = true }, "Automated Factory on " + kind + " Square", []dlgAct{dlgAbandon, dlgCancel}},
		{"colony base", -1, base, func() {}, "Colony Base on White Square", []dlgAct{dlgAutomate, dlgCancel}},
		{"shipyard", 0, base, func() {}, "Shipyard on Orbital Square", []dlgAct{dlgBuildShip, dlgAbandon, dlgCancel}},
		{"orbital", 1, base, func() {}, "Orbital Shield on Orbital Square", []dlgAct{dlgAbandon, dlgCancel}},
	} {
		tc.prep()
		if got := open(tc.slot, tc.cell); !slices.Equal(got, tc.buttons) {
			t.Errorf("%s: buttons %v, want %v", tc.name, got, tc.buttons)
		}
		if got := s.dlg.title(p); got != tc.title {
			t.Errorf("%s: title %q, want %q", tc.name, got, tc.title)
		}
		bs := s.dlgButtons(a)
		for i, b := range bs {
			x, y := float64(b.r.X0+b.r.X1)/2, float64(b.r.Y0+b.r.Y1)/2
			if k, ok := s.dlgAt(a, x, y); !ok || k != b.act || !dlgBox.has(x, y) || i > 0 && b.r.X0 < bs[i-1].r.X1 {
				t.Errorf("%s: button %d %v at (%v,%v): %v %v", tc.name, i, b.act, x, y, k, ok)
			}
		}
		if !s.cancel() || s.dlg != nil {
			t.Errorf("%s: Esc left the dialog %v", tc.name, s.dlg)
		}
	}
	// The status line names an automated structure as the dialog does.
	dlgHoverCell(s, f)
	if got, want := s.squareText(a), "Automated Factory"; !strings.Contains(got, want) {
		t.Errorf("status %q, want %q in it", got, want)
	}
}

// The presses: Automate starts the colony's Automation (a refusal is the
// colony's, the dialog kept), Abandon demolishes and closes, Build Ship
// opens the templates, Cancel and a click outside close; a click on the
// running project's target asks to cancel it instead, while another
// structure's still opens.
func TestStructDlgPress(t *testing.T) {
	a, s, f := dlgHome(t)
	p, r, col := s.planet, a.e.Research, s.col
	r.Grant(game.Tech(game.Automation.Info().ResearchReq))
	empty, ok := p.BestCell(game.Agriplot, col.Traits())
	if !ok {
		t.Fatal("no empty square")
	}
	if err := col.Start(r, game.Agriplot, empty); err != nil {
		t.Fatal(err)
	}
	dlgHoverCell(s, f)
	if _, err := s.clickSlot(a); err != nil || s.dlg == nil {
		t.Fatalf("click on the Factory while a project runs: %v, dialog %v", err, s.dlg)
	}
	// A project runs: Automate queues (CanEnqueue), the dialog closes.
	if push, err := s.press(a, dlgAutomate); push != nil || err != nil || s.dlg != nil ||
		!slices.Equal(col.Queue, []game.Queued{{Item: game.Automation, Cell: f, Slot: -1}}) {
		t.Errorf("Automate queued: %#v %v, dialog %v, queue %+v", push, err, s.dlg, col.Queue)
	}
	col.Cancel()
	col.Next(r)
	if pj := col.Project; pj == nil || pj.Item != game.Automation || pj.Cell != f || pj.Target != game.Factory {
		t.Fatalf("project %+v", pj)
	}
	// The target asks to cancel; the Colony Base opens its dialog.
	if _, err := s.clickSlot(a); err != nil || s.dlg != nil || s.confirm == nil {
		t.Errorf("click on the target: %v, dialog %v, question %v", err, s.dlg, s.confirm)
	}
	s.answerCancel(false)
	if s.hoverColour(a) != orange {
		t.Errorf("the target's hover %v, want orange", s.hoverColour(a))
	}
	dlgHoverCell(s, game.Cell{I: game.Centre, J: game.Centre})
	if _, err := s.clickSlot(a); err != nil || s.dlg == nil || s.confirm != nil || s.hoverColour(a) != hoverColor {
		t.Errorf("click on the base: %v, dialog %v, question %v, hover %v", err, s.dlg, s.confirm, s.hoverColour(a))
	}
	// Cancel and a click outside the box close, nothing changed; a click in
	// the box off the buttons keeps it.
	if _, err := s.press(a, dlgCancel); err != nil || s.dlg != nil || col.Project == nil {
		t.Errorf("Cancel: %v, dialog %v, project %+v", err, s.dlg, col.Project)
	}
	s.openDlg()
	if s.dlgClick(a, float64(dlgBox.X0+8), float64(dlgBox.Y0+8)); s.dlg == nil {
		t.Error("a click in the box closed it")
	}
	if s.dlgClick(a, float64(dlgBox.X0-8), float64(dlgBox.Y0-8)); s.dlg != nil {
		t.Error("a click outside kept it")
	}
	// Abandon: the Factory demolished (its running Automation then dropped
	// by the model), an orbital slot emptied.
	col.Cancel()
	dlgHoverCell(s, f)
	s.openDlg()
	if _, err := s.press(a, dlgAbandon); err != nil || s.dlg != nil || p.Surface[f].Item != game.NoItem {
		t.Errorf("Abandon: %v, dialog %v, square %+v", err, s.dlg, *p.Surface[f])
	}
	dlgHoverSlot(s, 1)
	s.openDlg()
	if _, err := s.press(a, dlgAbandon); err != nil || s.dlg != nil || p.Orbit[1] != game.NoItem {
		t.Errorf("orbital Abandon: %v, dialog %v, slot %s", err, s.dlg, p.Orbit[1])
	}
	// Abandon refused by the model (the last Shipyard under a Ship
	// project): the dialog stays, the slot is kept.
	col.Project = &game.Project{Item: game.Ship, Slot: 1, Orbital: true}
	dlgHoverSlot(s, 0)
	s.openDlg()
	if _, err := s.press(a, dlgAbandon); err == nil || s.dlg == nil || p.Orbit[0] != game.Shipyard {
		t.Errorf("the Shipyard's Abandon under a ship: %v, dialog %v, slot %s", err, s.dlg, p.Orbit[0])
	}
	col.Cancel()
	s.dlg = nil
	// Build Ship: a blank design screen for this colony.
	dlgHoverSlot(s, 0)
	s.openDlg()
	if push, err := s.press(a, dlgBuildShip); err != nil || s.dlg != nil {
		t.Errorf("Build Ship: %v, dialog %v", err, s.dlg)
	} else if !blankDesign(push, col) {
		t.Errorf("Build Ship pushes %#v", push)
	}
	// The structure gone under the dialog closes it.
	dlgHoverSlot(s, 0)
	s.openDlg()
	p.DemolishOrbital(0)
	if s.refresh(a); s.dlg != nil {
		t.Error("the dialog outlived its Shipyard")
	}
}

// Another empire's ship drawn in a free orbit slot takes no click: the
// slot's own click goes through (here the picker opens); the player's
// own ship there is selected.
func TestForeignShipSlotClick(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	col := a.e.Colonies[0]
	a.e.Research.Grant(game.Tech(game.Shipyard.Info().ResearchReq))
	col.Pop = col.Busy() + 1
	g.Empires[1].Spawn(foeShip, col.Star, col.PlanetIdx)
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	os := s.orbitShips(a)
	if len(os) != 1 || os[0].own != nil {
		t.Fatalf("orbit %+v: want the one foreign ship", os)
	}
	x, y := slotCentre(os[0].slot)
	s.hoverAt(a, x, y)
	if !s.slotOK || s.slot != os[0].slot {
		t.Fatalf("hover slot %d %v, want %d", s.slot, s.slotOK, os[0].slot)
	}
	if push, err := s.leftClick(a); push != nil || err != nil || s.pick == nil || s.dlg != nil {
		t.Errorf("click on the foreign ship's slot: %#v %v, picker %v, dialog %+v", push, err, s.pick, s.dlg)
	}
	s.pick = nil
	mine := a.e.Spawn(fixtureShip, col.Star, col.PlanetIdx)
	os = s.orbitShips(a)
	x, y = slotCentre(os[0].slot)
	s.hoverAt(a, x, y)
	if push, err := s.leftClick(a); push != nil || err != nil || s.pick != nil || s.dlg == nil || s.dlg.ship != mine {
		t.Errorf("click on my ship: %#v %v, picker %v, dialog %+v", push, err, s.pick, s.dlg)
	}
}

// An Outpost (employs nobody: Item.Staff) is not offered Automate; its
// Abandon is greyed, a press doing nothing, while the population needs
// its room (Colony.CanAbandon), pressable once the room is spare.
func TestStructDlgOutpost(t *testing.T) {
	a, s, _ := dlgHome(t)
	col, p, r := a.e.Colonies[0], s.planet, a.e.Research
	r.Grant(game.Tech(game.Automation.Info().ResearchReq))
	at, ok := p.BestCell(game.Outpost, col.Traits())
	if !ok || p.Build(at, game.Outpost) != nil {
		t.Fatal("no Outpost square")
	}
	dlgHoverCell(s, at)
	if _, err := s.clickSlot(a); err != nil || s.dlg == nil {
		t.Fatalf("no dialog: %v", err)
	}
	if got := s.dlgActs(a); slices.Contains(got, dlgAutomate) || !slices.Contains(got, dlgAbandon) {
		t.Errorf("buttons %v: want Abandon, no Automate", got)
	}
	col.Pop = col.Capacity() // the Outpost's room is used
	if !s.dlgOff(a, dlgAbandon) || s.dlgOff(a, dlgCancel) {
		t.Fatal("Abandon not greyed with the room used")
	}
	if push, err := s.press(a, dlgAbandon); push != nil || err != nil || p.Surface[at].Item != game.Outpost || s.dlg == nil {
		t.Fatalf("greyed press: %v %v, item %v, dialog %v", push, err, p.Surface[at].Item, s.dlg)
	}
	col.Pop = col.Capacity() - game.Outpost.Info().MaxPop
	if s.dlgOff(a, dlgAbandon) {
		t.Fatal("Abandon greyed with the room spare")
	}
	if _, err := s.press(a, dlgAbandon); err != nil || p.Surface[at].Item != game.NoItem {
		t.Errorf("press: %v, item %v", err, p.Surface[at].Item)
	}
}
