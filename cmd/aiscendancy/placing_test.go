package main

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
)

// hoverPanel hovers the project panel's centre, clear of the cancel cross.
func hoverPanel(a *app, s *planetScreen) {
	p := panels[4]
	s.hoverAt(a, float64(p.X0+p.X1)/2, float64(p.Y0+p.Y1)/2)
}

// placeHome is pickerHome with people to spare, the surface picker
// opened from the project panel and item chosen in it: the mode on.
func placeHome(t *testing.T, item game.Item) (*app, *planetScreen, *game.Colony) {
	t.Helper()
	a, s, col := pickerHome(t)
	col.Pop += 4
	hoverPanel(a, s)
	if _, err := s.leftClick(a); err != nil || s.pick == nil || !s.pick.surface {
		t.Fatalf("panel click: %v, picker %+v", err, s.pick)
	}
	for i, c := range s.pick.cs {
		if c.item == item {
			if err := s.pick.choose(a, s, i); err != nil || s.pick != nil || !s.placing() || s.placeItem != item {
				t.Fatalf("choose %s: %v, picker %v, placing %s", item, err, s.pick, s.placeItem)
			}
			return a, s, col
		}
	}
	t.Fatalf("%s not in the surface picker", item)
	return nil, nil, nil
}

// accepting is the first open empty square of the plan the colony
// accepts it on now (the per-square picker's verdict), else fails.
func accepting(t *testing.T, a *app, s *planetScreen, it game.Item) game.Cell {
	t.Helper()
	pl := s.plan()
	for _, c := range pl.Cells() {
		if pl.Surface[c].Item != game.NoItem || !s.openIn(pl, c) || !pl.Offers(it, c) {
			continue
		}
		if s.cellVerdict(a, it, c) == nil {
			return c
		}
	}
	t.Fatalf("no square accepts %s", it)
	return game.Cell{}
}

// The project panel opens the surface picker: every Surface item
// research allows but the Colony Base, in Item order, each buildable
// when some free square takes it (CanStart there); its title says so.
// The cancel cross still asks to cancel; a foreign or bare planet's
// panel does nothing.
func TestPlacePanel(t *testing.T) {
	a, s, col := pickerHome(t)
	home := s.cell
	a.e.Research.Known[game.ColonyBase.Info().ResearchReq] = true // known, yet never offered
	hoverPanel(a, s)
	if !s.panelOn || s.cellOK || s.slotOK {
		t.Fatalf("panel hover: on %v, cell %v, slot %v", s.panelOn, s.cellOK, s.slotOK)
	}
	if _, err := s.leftClick(a); err != nil || s.pick == nil || !s.pick.surface || s.pick.orbit {
		t.Fatalf("click: %v, picker %+v", err, s.pick)
	}
	if want := "Build on the surface  free "; !strings.HasPrefix(s.pick.title, want) || s.pick.box() != pickFull {
		t.Errorf("title %q, box %v", s.pick.title, s.pick.box())
	}
	var want []choice
	for it := game.Item(0); it < game.NumItems; it++ {
		if it.Project() || it.Orbital() || it == game.ColonyBase || !a.e.Research.Allows(it) {
			continue
		}
		ok := false
		for _, c := range s.planet.Cells() {
			sq := s.planet.Surface[c]
			if sq.Item == game.NoItem && s.planet.Reachable(c) && s.planet.Offers(it, c) && col.CanStart(a.e.Research, it, c) == nil {
				ok = true
				break
			}
		}
		want = append(want, choice{item: it, err: map[bool]error{false: errNoFreeSquare}[ok]})
	}
	if len(s.pick.cs) != len(want) {
		t.Fatalf("%d choices, want %d", len(s.pick.cs), len(want))
	}
	for i, c := range s.pick.cs {
		if c.item != want[i].item || (c.err == nil) != (want[i].err == nil) {
			t.Errorf("choice %d: %s err %v, want %s buildable %v", i, c.item, c.err, want[i].item, want[i].err == nil)
		}
	}
	// No free people: every choice refused, the panel flashes why.
	s.pick = nil
	pop := col.Pop
	col.Pop = col.Busy()
	if err := s.clickPanel(a); err == nil || s.pick != nil {
		t.Errorf("no free people: %v, picker %v", err, s.pick)
	}
	col.Pop = pop

	// The cancel cross is not the panel.
	if err := col.Start(a.e.Research, game.Factory, home); err != nil {
		t.Fatal(err)
	}
	s.hoverAt(a, float64(cancelRect.X0+cancelRect.X1)/2, float64(cancelRect.Y0+cancelRect.Y1)/2)
	if _, err := s.leftClick(a); err != nil || !s.xOn || s.panelOn || s.confirm == nil || s.pick != nil {
		t.Fatalf("cross: %v, x %v, panel %v, question %v, picker %v", err, s.xOn, s.panelOn, s.confirm, s.pick)
	}
	s.answerCancel(false)
	// Enqueuing, the panel still opens it, judged by CanEnqueue.
	hoverPanel(a, s)
	if _, err := s.leftClick(a); err != nil || s.pick == nil || !strings.HasPrefix(s.pick.title, "Queue on the surface") {
		t.Fatalf("enqueuing: %v, picker %+v", err, s.pick)
	}
	s.pick = nil

	others := 0
	for i, p := range a.cl.Stars[col.Star].System.Planets {
		if i == col.PlanetIdx {
			continue
		}
		others++
		u := newPlanetScreen(col.Star, i)
		u.Enter(a)
		hoverPanel(a, u)
		if _, err := u.leftClick(a); u.panelOn || u.pick != nil || u.placing() || err != nil {
			t.Errorf("planet %d (owner %d): panel %v, picker %v, placing %v, err %v", i, p.Owner, u.panelOn, u.pick, u.placing(), err)
		}
	}
	if others == 0 {
		t.Error("fixture: no other planet in the home system")
	}
}

// The mode: three clicks on accepting squares start one and queue two,
// the mode staying on; a structure, a refused square, an orbit slot do
// nothing; a queued ghost's square dequeues it. With a project running
// all three queue.
func TestPlaceMode(t *testing.T) {
	a, s, col := placeHome(t, game.Factory)
	var at []game.Cell
	for range 3 {
		c := accepting(t, a, s, game.Factory)
		s.cell, s.cellOK, s.slotOK = c, true, false
		if s.hoverColour(a) != yellow || !strings.HasSuffix(s.placeText(a), "Factory") {
			t.Errorf("%v: hover %v, line %q", c, s.hoverColour(a), s.placeText(a))
		}
		if _, err := s.leftClick(a); err != nil || !s.placing() || s.pick != nil || s.dlg != nil {
			t.Fatalf("click %v: %v, placing %v, picker %v, dialog %v", c, err, s.placing(), s.pick, s.dlg)
		}
		at = append(at, c)
	}
	if col.Project == nil || col.Project.Item != game.Factory || col.Project.Cell != at[0] || len(col.Queue) != 2 ||
		col.Queue[0].Cell != at[1] || col.Queue[1].Cell != at[2] {
		t.Fatalf("project %+v, queue %+v, want at %v", col.Project, col.Queue, at)
	}
	if _, pp := s.panelTexts(); pp.foot != placingFoot(game.Factory) || pp.name != "Factory" {
		t.Errorf("panel %+v", pp)
	}
	q := len(col.Queue)
	nothing := func(what string) {
		t.Helper()
		if _, err := s.leftClick(a); err != nil || len(col.Queue) != q || s.dlg != nil || s.pick != nil || s.confirm != nil || !s.placing() {
			t.Errorf("%s: %v, queue %d, dialog %v, picker %v, question %v, placing %v", what, err, len(col.Queue), s.dlg, s.pick, s.confirm, s.placing())
		}
	}
	// The running project's target: refused (red), no cancel question.
	s.cell = at[0]
	if s.hoverColour(a) != red || s.placeErr(a) == nil {
		t.Errorf("target: hover %v, verdict %v", s.hoverColour(a), s.placeErr(a))
	}
	nothing("the project's target")
	for _, c := range s.planet.Cells() {
		if s.planet.Surface[c].Item != game.NoItem {
			s.cell = c
			break
		}
	}
	nothing("a structure")
	s.cellOK, s.slot, s.slotOK = false, 0, true
	nothing("an orbit slot")
	// A queued ghost's square dequeues it, the mode on.
	s.cell, s.cellOK, s.slotOK = at[2], true, false
	if _, err := s.leftClick(a); err != nil || len(col.Queue) != 1 || !s.placing() {
		t.Errorf("ghost: %v, queue %+v, placing %v", err, col.Queue, s.placing())
	}

	// A project running from before: the three clicks queue.
	a, s, col = placeHome(t, game.Laboratory)
	if err := col.Start(a.e.Research, game.Agriplot, accepting(t, a, s, game.Agriplot)); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		s.cell, s.cellOK = accepting(t, a, s, game.Laboratory), true
		if _, err := s.leftClick(a); err != nil {
			t.Fatal(err)
		}
	}
	if col.Project.Item != game.Agriplot || len(col.Queue) != 3 || !s.placing() {
		t.Errorf("project %+v, queue %+v, placing %v", col.Project, col.Queue, s.placing())
	}
	for _, e := range col.Queue {
		if e.Item != game.Laboratory {
			t.Errorf("queued %+v", e)
		}
	}
}

// The mode ends on a right click, on Esc (the screen staying), on the
// panel's click (no picker), on the colony changing hands; a second Esc
// then goes back.
func TestPlaceEnd(t *testing.T) {
	a, s, _ := placeHome(t, game.Factory)
	s.rightClick()
	if s.placing() {
		t.Error("right click: still placing")
	}

	a, s, _ = placeHome(t, game.Factory)
	a.push(newStarmapScreen())
	a.push(s)
	n := len(a.stack)
	pressing(t, ebiten.KeyEscape)
	if err := a.Update(); err != nil || s.placing() || len(a.stack) != n || a.top() != s || a.quitting {
		t.Fatalf("Esc: %v, placing %v, stack %d -> %d", err, s.placing(), n, len(a.stack))
	}
	if err := a.Update(); err != nil || len(a.stack) != n-1 {
		t.Errorf("second Esc: %v, stack %d -> %d", err, n, len(a.stack))
	}

	a, s, _ = placeHome(t, game.Factory)
	hoverPanel(a, s)
	if _, err := s.leftClick(a); err != nil || s.placing() || s.pick != nil {
		t.Errorf("panel: %v, placing %v, picker %v", err, s.placing(), s.pick)
	}

	a, s, _ = placeHome(t, game.Factory)
	s.col = &game.Colony{} // as if the screen last saw another colony
	s.refresh(a)
	if s.placing() {
		t.Error("colony changed hands: still placing")
	}
}
