package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
)

// On the home world a start (the picker's choice) sets the colony's
// project (game.Colony's rules: the square stays empty, a second start is
// refused) and the hover says whether the picker opens; an uninhabited
// planet is read-only.
func TestPlanetBuild(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	home := e.Cluster.Home
	s := newPlanetScreen(home, 0)
	s.Enter(a)
	col := e.Colonies[0]
	if s.col != col {
		t.Fatalf("the home world's screen found colony %p, want %p", s.col, col)
	}

	cell, ok := s.planet.BestCell(game.Agriplot, game.Base)
	if !ok {
		t.Fatal("no square for an Agriplot")
	}
	s.cell, s.cellOK = cell, true
	if !s.canOpen(a) {
		t.Fatalf("hover refuses: %v", col.CanStart(e.Research, game.Agriplot, cell))
	}
	if err := s.start(a, game.Agriplot); err != nil {
		t.Fatal(err)
	}
	if col.Project == nil || col.Project.Item != game.Agriplot || col.Project.Cell != cell {
		t.Fatalf("project %+v", col.Project)
	}
	if it := s.planet.Surface[cell].Item; it != game.NoItem {
		t.Fatalf("the square holds %s at once: the sandbox built", it)
	}

	// The planet would take a second Agriplot there; the colony would not.
	if !s.planet.CanBuild(cell, game.Agriplot, game.Base) {
		t.Fatal("the planet refuses the square: the next check proves nothing")
	}
	if s.canOpen(a) {
		t.Error("hover accepts a second project")
	}
	// Enqueuing now: the planned square holds the Agriplot.
	if err := s.start(a, game.Agriplot); err == nil || !strings.Contains(err.Error(), "holds Agriplot") || len(col.Queue) != 0 {
		t.Errorf("second start: %v, queue %v, want the plan's refusal", err, col.Queue)
	}

	// An uninhabited planet is read-only: no verdict, the click refused
	// and nothing built, on a square or an orbit slot.
	pl := -1
	for i := range e.Cluster.Stars[home].System.Planets {
		if !e.Inhabited(home, i) {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Fatal("no uninhabited planet at home")
	}
	u := newPlanetScreen(home, pl)
	u.Enter(a)
	if u.col != nil || u.other != nil {
		t.Fatal("a colony on an uninhabited planet")
	}
	c, ok := u.planet.BestCell(game.Agriplot, game.Base)
	if !ok {
		t.Skip("no Agriplot square on the uninhabited planet")
	}
	if !u.planet.CanBuild(c, game.Agriplot, e.Species.Traits()) {
		t.Fatal("the planet refuses the square: the next checks prove nothing")
	}
	u.cell, u.cellOK = c, true
	if u.canOpen(a) || u.hoverColour(a) == yellow || u.hoverColour(a) == red {
		t.Errorf("read-only hover: verdict %v, colour %v", u.canOpen(a), u.hoverColour(a))
	}
	if _, err := u.clickSlot(a); err == nil || err.Error() != "uninhabited" || u.pick != nil {
		t.Errorf("uninhabited click: %v, picker %v", err, u.pick)
	}
	if err := u.start(a, game.Agriplot); err == nil || err.Error() != "uninhabited" || u.planet.Surface[c].Item != game.NoItem {
		t.Errorf("uninhabited build: %v, square holds %s", err, u.planet.Surface[c].Item)
	}
	u.cellOK, u.slot, u.slotOK = false, 0, true
	if err := u.start(a, game.Shipyard); err == nil || err.Error() != "uninhabited" || u.planet.Orbit[0] != game.NoItem {
		t.Errorf("uninhabited orbital build: %v, slot holds %s", err, u.planet.Orbit[0])
	}
	// A project button hovered on it: the status line names the project,
	// no verdict (never the nil colony's CanStartProject).
	u.slotOK, u.projOn = false, game.FirstProject
	u.Text(a, ebiten.NewImage(screenW, screenH))
}

// Another empire's colony is read-only: its species, Pop and project in
// the panels, the clicks refused ("not your planet"), its Shipyard opens
// nothing, and neither its project nor its surface changes.
func TestPlanetForeignColony(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	orfa := g.Empires[1].Colonies[0]
	if err := orfa.Planet.BuildOrbital(2, game.Shipyard); err != nil {
		t.Fatal(err)
	}
	s := newPlanetScreen(orfa.Star, orfa.PlanetIdx)
	s.Enter(a)
	if s.col != nil || s.other != orfa {
		t.Fatalf("the Orfa home: colony %p, other %p, want nil, %p", s.col, s.other, orfa)
	}
	pop, pp := s.panelTexts()
	if want := "POP " + strconv.Itoa(orfa.Pop) + "/" + strconv.Itoa(orfa.Capacity()) + "\n" + game.Orfa.String(); pop != want {
		t.Errorf("POP panel %q, want %q", pop, want)
	}
	if want := "No Project"; orfa.Project == nil && pp.name != want || orfa.Project != nil && pp.name != projectName(orfa.Project) || pp.col != orfa {
		t.Errorf("project panel %+v", pp)
	}
	c, ok := s.planet.BestCell(game.Agriplot, game.Orfa.Traits())
	if !ok {
		t.Fatal("no Agriplot square on the Orfa home")
	}
	if !s.planet.CanBuild(c, game.Agriplot, game.Orfa.Traits()) {
		t.Fatal("the planet refuses the square: the next checks prove nothing")
	}
	s.cell, s.cellOK = c, true
	if s.canOpen(a) {
		t.Error("hover accepts on another empire's colony")
	}
	if _, err := s.clickSlot(a); err == nil || err.Error() != "not your planet" || s.pick != nil {
		t.Errorf("foreign click: %v, picker %v", err, s.pick)
	}
	if err := s.start(a, game.Agriplot); err == nil || err.Error() != "not your planet" || orfa.Project != nil || s.planet.Surface[c].Item != game.NoItem {
		t.Errorf("foreign build: %v, project %+v, square holds %s", err, orfa.Project, s.planet.Surface[c].Item)
	}
	s.cellOK, s.slot, s.slotOK = false, 2, true
	if push, err := s.clickSlot(a); push != nil || err != nil || orfa.Project != nil {
		t.Errorf("foreign Shipyard click: %#v, %v, project %+v", push, err, orfa.Project)
	}
}

// A colony founded while the planet's screen is open (Space or the bot
// ends the day under it): the next start is the colony's project, never a
// free instant build.
func TestPlanetColonisedUnderTheScreen(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	home := e.Cluster.Home
	pl := -1
	for i := range e.Cluster.Stars[home].System.Planets {
		if _, ok := e.Planet(home, i).BestCell(game.ColonyBase, game.Base); ok && !e.Inhabited(home, i) {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Skip("no colonisable planet at home")
	}
	s := newPlanetScreen(home, pl)
	s.Enter(a)
	if s.col != nil {
		t.Fatal("a colony before the founding")
	}
	sh := e.Spawn(fixtureShip, home, pl)
	if err := e.Found(sh); err != nil {
		t.Fatal(err)
	}
	col := e.Colony(home, pl)
	c, ok := s.planet.BestCell(game.Agriplot, game.Base)
	if !ok {
		t.Skip("no Agriplot square")
	}
	s.cell, s.cellOK = c, true
	err := s.start(a, game.Agriplot)
	if it := s.planet.Surface[c].Item; it != game.NoItem {
		t.Fatalf("the click built %s at once on a colony (err %v): an instant build", it, err)
	}
	if err != nil || col.Project == nil || col.Project.Cell != c {
		t.Fatalf("no colony project: %v, %+v", err, col.Project)
	}
	if s.col != col {
		t.Errorf("screen colony %p, want %p", s.col, col)
	}
}

// A left click on an orbit slot holding a Shipyard opens its dialog,
// whose Build Ship opens a blank design screen carrying the player's
// colony; on an uninhabited planet it opens nothing. On another slot the
// click opens the picker or says why not (refused uninhabited).
func TestPlanetShipyardClick(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	home := e.Cluster.Home
	pl := -1
	for i := range e.Cluster.Stars[home].System.Planets {
		if !e.Inhabited(home, i) {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Fatal("no uninhabited planet at home")
	}
	if err := e.Colonies[0].Planet.BuildOrbital(2, game.Shipyard); err != nil {
		t.Fatal(err)
	}
	for _, idx := range []int{0, pl} {
		s := newPlanetScreen(home, idx)
		s.Enter(a)
		if idx == pl {
			if err := s.planet.BuildOrbital(2, game.Shipyard); err != nil {
				t.Fatal(err)
			}
		}
		s.slot, s.slotOK, s.cellOK = 2, true, false
		push, err := s.clickSlot(a)
		if !s.onShipyard() || err != nil || push != nil || (s.dlg != nil) != (idx == 0) {
			t.Fatalf("planet %d: Shipyard slot click: %#v %v, dialog %v", idx, push, err, s.dlg)
		}
		if idx == 0 {
			push, err = s.press(a, dlgBuildShip)
		}
		switch {
		case idx == 0 && !blankDesign(push, e.Colonies[0]):
			t.Errorf("home Shipyard click pushes %#v, want a blank design for %p", push, e.Colonies[0])
		case idx == pl && push != nil:
			t.Errorf("uninhabited Shipyard click pushes %#v", push)
		}
		if s.col != nil && s.col.Project != nil {
			t.Errorf("planet %d: the click started %+v", idx, s.col.Project)
		}
		// Slot 3 is empty: the build path (a second Shipyard there).
		s.slot = 3
		if s.onShipyard() {
			t.Fatalf("planet %d: slot 3 holds a Shipyard already", idx)
		}
		var verdict error // the colony's own answer: research refuses it
		if s.col != nil {
			verdict = s.col.CanStartOrbital(e.Research, game.Shipyard, 3)
			if err := s.start(a, game.Shipyard); verdict == nil || err == nil || err.Error() != verdict.Error() || s.col.Project != nil {
				t.Errorf("colony start on slot 3: %v, want %v, project %+v", err, verdict, s.col.Project)
			}
		}
		cs := s.choices(a)
		push, err = s.clickSlot(a)
		if push != nil {
			t.Errorf("planet %d: empty slot pushes %#v", idx, push)
		}
		switch {
		case s.col != nil && buildable(cs) && (err != nil || s.pick == nil):
			t.Errorf("colony click on slot 3: %v, picker %v, want it open", err, s.pick)
		case s.col != nil && !buildable(cs) && (err == nil || s.pick != nil || s.col.Project != nil):
			t.Errorf("colony click on slot 3: %v, picker %v, project %+v", err, s.pick, s.col.Project)
		case s.col == nil && (err == nil || err.Error() != "uninhabited" || s.planet.Orbit[3] != game.NoItem):
			t.Errorf("uninhabited build on slot 3: %v, holds %s", err, s.planet.Orbit[3])
		}
	}
}

// A square answers the mouse when it holds a structure or the planet says
// one can reach it; the others are markers. Expectations from the model.
func TestPlanetOpenSquares(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	s := newPlanetScreen(g.Empires[0].Cluster.Home, 0)
	s.Enter(a)
	var built, reach, marker int
	for _, c := range s.planet.Cells() {
		sq := s.planet.Surface[c]
		want := sq.Item != game.NoItem || s.planet.Reachable(c)
		if got := s.open(c); got != want {
			t.Errorf("%v (%s, %s): open %v, want %v", c, sq.Kind, sq.Item, got, want)
		}
		switch {
		case sq.Item != game.NoItem:
			built++
		case want:
			reach++
		default:
			marker++
		}
	}
	if built == 0 || reach == 0 || marker == 0 {
		t.Errorf("home world: %d built, %d reachable, %d markers: a case is not exercised", built, reach, marker)
	}
}

// popLayout: busy, then free, then room icons, popCols a row from the
// panel's corner at popPitch; more than popRows rows switch to the dense
// grid at popDense; past its rows the rest is counted, not drawn.
func TestPopLayout(t *testing.T) {
	icons, more, k := popLayout(3, 9, 5)
	if len(icons) != 17 || more != 0 || k != zoom {
		t.Fatalf("%d icons, %d more", len(icons), more)
	}
	for i, ic := range icons {
		want := popRoom
		switch {
		case i < 3:
			want = popBusy
		case i < 12:
			want = popFree
		}
		if ic.kind != want || ic.x != i%popCols*popPitch || ic.y != i/popCols*popPitch {
			t.Errorf("icon %d: %+v, want kind %d at (%d,%d)", i, ic, want, i%popCols*popPitch, i/popCols*popPitch)
		}
	}
	if icons[popCols].x != 0 || icons[popCols].y != popPitch {
		t.Errorf("row 2 starts at (%d,%d)", icons[popCols].x, icons[popCols].y)
	}
	if icons, more, k := popLayout(50, 5, 15); len(icons) != popCols*popRows || more != 0 || k != zoom {
		t.Errorf("full: %d icons, %d more, scale %v", len(icons), more, k)
	}
	icons, more, k = popLayout(50, 5, 40)
	if len(icons) != 95 || more != 0 || k != popDense || icons[54].kind != popFree || icons[94].kind != popRoom ||
		icons[popDenseCols].x != 0 || icons[popDenseCols].y != popDensePitch {
		t.Errorf("dense: %d icons, %d more, scale %v, row 2 %+v", len(icons), more, k, icons[popDenseCols])
	}
	icons, more, _ = popLayout(150, 50, 0)
	if n := popDenseCols * popDenseRows; len(icons) != n || more != 200-n || icons[n-1].y != (popDenseRows-1)*popDensePitch {
		t.Errorf("overflow: %d icons, %d more, last %+v", len(icons), more, icons[len(icons)-1])
	}
	if icons, more, _ := popLayout(0, 0, 0); len(icons) != 0 || more != 0 {
		t.Errorf("empty: %v, %d", icons, more)
	}
	// The dense grid stays inside the normal one's area, its pitch the
	// icon at popDense.
	if popDensePitch != 10*popDense || popDenseCols*popDensePitch > popCols*popPitch || popDenseRows*popDensePitch > popRows*popPitch {
		t.Error("the dense grid overflows the icons' area")
	}
	// The panel's icons fill it left to right in the original's pitch.
	if popCols*popPitch != panels[3].X1-panels[3].X0 || float64(popRows*popPitch)+3*bigFace.Size*1.25 > float64(panels[3].Y1-panels[3].Y0) {
		t.Error("the icons and three bigFace lines do not fit the POP panel")
	}
}

// The POP panel's counts come from the colonies: the player's busy and
// free people and room to its capacity, another empire's people all
// free, an uninhabited planet's room alone; the text says the totals and
// "+N" for the icons that do not fit.
func TestPopPanel(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	e := g.Empires[0]
	col := e.Colonies[0]
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	s.Enter(a)
	col.Pop = col.Busy() + 2
	busy, free, room := s.popCounts()
	if busy != col.Busy() || free != col.Free() || room != col.Capacity()-col.Pop || free != 2 || room <= 0 {
		t.Errorf("home: %d %d %d, want %d %d %d", busy, free, room, col.Busy(), col.Free(), col.Capacity()-col.Pop)
	}
	pop, _ := s.panelTexts()
	d, ok := col.GrowthDays()
	if !ok {
		t.Fatal("the home does not grow: the days case is not walked")
	}
	if want := "POP " + strconv.Itoa(col.Pop) + "/" + strconv.Itoa(col.Capacity()) + "\nfree 2\ngrowth in " + dayCount(d); pop != want {
		t.Errorf("home text %q, want %q", pop, want)
	}
	// Over capacity and past the panel: no room, "+N".
	col.Pop = col.Capacity() + popDenseCols*popDenseRows
	if _, _, room := s.popCounts(); room != 0 {
		t.Errorf("over capacity: room %d", room)
	}
	pop, _ = s.panelTexts()
	if want := "+" + strconv.Itoa(col.Pop-popDenseCols*popDenseRows); !strings.HasSuffix(pop, "  "+want+"\nfull") {
		t.Errorf("overflow text %q, want it ending %q then full", pop, want)
	}

	orfa := g.Empires[1].Colonies[0]
	o := newPlanetScreen(orfa.Star, orfa.PlanetIdx)
	o.Enter(a)
	if busy, free, room := o.popCounts(); busy != 0 || free != orfa.Pop || room != orfa.Capacity()-orfa.Pop {
		t.Errorf("foreign: %d %d %d, want 0 %d %d", busy, free, room, orfa.Pop, orfa.Capacity()-orfa.Pop)
	}
	if pop, _ := o.panelTexts(); strings.Contains(pop, "growth") || strings.Count(pop, "\n") != 1 {
		t.Errorf("foreign text %q: its growth is not ours to see", pop)
	}

	for i, p := range a.cl.Stars[col.Star].System.Planets {
		if p.Owner >= 0 {
			continue
		}
		u := newPlanetScreen(col.Star, i)
		u.Enter(a)
		bare := p.Capacity()
		if busy, free, room := u.popCounts(); busy != 0 || free != 0 || room != bare || bare <= 0 {
			t.Errorf("uninhabited %s: %d %d %d, want 0 0 %d", p.Name, busy, free, room, bare)
		}
		if pop, _ := u.panelTexts(); pop != "POP 0/"+strconv.Itoa(bare)+"\n" {
			t.Errorf("uninhabited text %q", pop)
		}
		return
	}
	t.Fatal("no uninhabited planet at home")
}

// growthText: the days GrowthDays says, "full" at capacity, "no growth"
// with room but no prosperity (a colony with no structure).
func TestGrowthText(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	col := g.Empires[0].Colonies[0]
	d, ok := col.GrowthDays()
	if !ok || growthText(col) != "growth in "+dayCount(d) {
		t.Errorf("home: %q, GrowthDays %d %v", growthText(col), d, ok)
	}
	col.Pop = col.Capacity()
	if got := growthText(col); got != "full" {
		t.Errorf("at capacity: %q", got)
	}
	var bare *game.Planet
	for _, p := range g.Cluster.Stars[col.Star].System.Planets {
		if p.Owner < 0 && p.Capacity() > 1 {
			bare = p
		}
	}
	if bare == nil {
		t.Fatal("no uninhabited planet at home")
	}
	c := &game.Colony{Planet: bare, Star: col.Star, Pop: 1}
	if _, _, pros := c.Output(); pros != 0 {
		t.Fatalf("a bare colony makes %d prosperity", pros)
	}
	if got := growthText(c); got != "no growth" {
		t.Errorf("no prosperity: %q", got)
	}
}

// The Lush Growth Bomb's button starts it (Colony.StartProject); while it
// runs a click asks to cancel it; once the planet is bombed the button
// is tinted and a click is refused with the rule's error.
func TestPlanetLushBomb(t *testing.T) {
	a, s, col := pickerHome(t)
	it := game.LushGrowthBomb
	a.e.Research.Grant(game.Tech(it.Info().ResearchReq))
	lush := func() projButton {
		for _, b := range s.projButtons(a) {
			if b.item == it {
				return b
			}
		}
		t.Fatal("no Lush Growth Bomb button")
		return projButton{}
	}
	if b := lush(); b.err != nil || b.bombed {
		t.Fatalf("before: %v, bombed %v", b.err, b.bombed)
	}
	if err := s.clickProject(a, it); err != nil || col.Project == nil || col.Project.Item != it {
		t.Fatalf("click: %v, project %+v", err, col.Project)
	}
	if !s.running(it) || s.planet.Lush {
		t.Fatalf("running %v, lush %v", s.running(it), s.planet.Lush)
	}
	if err := s.clickProject(a, it); err != nil || s.confirm == nil || s.doomed != col.Project {
		t.Fatalf("click while it runs: %v, question %v", err, s.confirm)
	}
	s.answerCancel(false)
	for day := 0; day < 2000 && !s.planet.Lush; day++ {
		col.EndDay(a.e.Research, 0)
	}
	if !s.planet.Lush || col.Project != nil {
		t.Fatalf("lush %v, project %+v", s.planet.Lush, col.Project)
	}
	want := col.CanStartProject(a.e.Research, it)
	if b := lush(); !b.bombed || want == nil || b.err.Error() != want.Error() {
		t.Errorf("after: bombed %v, %v, want %v", b.bombed, b.err, want)
	}
	if err := s.clickProject(a, it); err == nil || err.Error() != want.Error() || col.Project != nil {
		t.Errorf("second click: %v, project %+v", err, col.Project)
	}
	for _, b := range s.projButtons(a) {
		if b.item != it && b.bombed {
			t.Errorf("%s tinted", b.item)
		}
	}
}

// A ruins square (hand-laid on the home world's reachable white square)
// says "Xeno ruins"; once research allows the dig, its picker offers it
// there with game.Colony.CanStart's verdict, the status line saying it,
// and nowhere else; started, the project is the dig, drawn in place; done,
// the ruins are gone and the discovery's log line opens the planet.
func TestPlanetRuins(t *testing.T) {
	a, s, col := pickerHome(t)
	dig := game.XenoArcheologicalDig
	ruin := s.cell
	s.planet.Surface[ruin].Ruins = true
	kind := s.planet.Surface[ruin].Kind
	plain := fmt.Sprintf("(%d,%d) %s - Xeno ruins", ruin.I, ruin.J, kind)
	if a.e.Research.Allows(dig) {
		t.Fatal("the fixture: the dig allowed at day 0")
	}
	if got := s.squareText(a); got != plain {
		t.Errorf("dig not researched: %q, want %q", got, plain)
	}
	a.e.Research.Grant(game.Tech(dig.Info().ResearchReq))
	offered := func() (error, bool) {
		for _, c := range s.choices(a) {
			if c.item == dig {
				return c.err, true
			}
		}
		return nil, false
	}
	err, ok := offered()
	if want := col.CanStart(a.e.Research, dig, ruin); !ok || want != nil || err != nil {
		t.Fatalf("on the ruins: offered %v, err %v, CanStart %v", ok, err, want)
	}
	if got, want := s.squareText(a), plain+" (the dig can start)"; got != want {
		t.Errorf("status %q, want %q", got, want)
	}
	pop := col.Pop
	col.Pop = col.Busy() // no free worker: refused, and said why
	want := col.CanStart(a.e.Research, dig, ruin)
	if err, ok := offered(); !ok || want == nil || err == nil || err.Error() != want.Error() {
		t.Errorf("no free worker: offered %v, err %v, want %v", ok, err, want)
	}
	if got := s.squareText(a); want == nil || got != plain+" - "+want.Error() {
		t.Errorf("no free worker: status %q, verdict %v", got, want)
	}
	col.Pop = pop
	for _, c := range s.planet.Cells() {
		if c == ruin || !s.open(c) {
			continue
		}
		s.cell = c
		if _, ok := offered(); ok {
			t.Errorf("the dig offered at %v, off the ruins", c)
		}
		if strings.Contains(s.squareText(a), "Xeno") {
			t.Errorf("%v: %q", c, s.squareText(a))
		}
	}

	s.cell = ruin
	if err := s.start(a, dig); err != nil || col.Project == nil || col.Project.Item != dig {
		t.Fatalf("start: %v, project %+v", err, col.Project)
	}
	pj := col.Project
	if projectName(pj) != dig.String() || !buildsInPlace(pj) || !s.digging(ruin) || pj.Cell != ruin {
		t.Errorf("running: name %q, in place %v, digging %v at %v", projectName(pj), buildsInPlace(pj), s.digging(ruin), pj.Cell)
	}
	if got := s.squareText(a); got != plain { // a project runs: no verdict
		t.Errorf("while digging: %q, want %q", got, plain)
	}
	s.askCancel()
	if w := fmt.Sprintf("Building %s at (%d,%d)", dig, ruin.I, ruin.J); s.confirm == nil || s.confirm.lines[0] != w {
		t.Errorf("cancel question %+v, want %q", s.confirm, w)
	}
	s.answerCancel(false)

	var evs []game.Event
	for day := 0; day < 2000 && col.Project != nil; day++ {
		evs = append(evs, col.EndDay(a.e.Research, 0)...)
	}
	if col.Project != nil || s.planet.Surface[ruin].Ruins || s.digging(ruin) {
		t.Fatalf("after: project %+v, ruins %v", col.Project, s.planet.Surface[ruin].Ruins)
	}
	if got := s.squareText(a); strings.Contains(got, "Xeno") {
		t.Errorf("dug: %q", got)
	}
	disc := slices.IndexFunc(evs, func(e game.Event) bool { return e.Kind == game.Discovered })
	if disc < 0 {
		t.Fatalf("no discovery in %v", evs)
	}
	a.log = append(a.log, logEntry{Day: 0, Ev: evs[disc]})
	ss := newLogScreen().open(a, len(a.log)-1)
	if len(ss) != 2 {
		t.Fatalf("the dig's discovery opens %v", ss)
	}
	if sys, ok := ss[0].(*systemScreen); !ok || sys.star != s.star {
		t.Errorf("first screen %#v, want system %d", ss[0], s.star)
	}
	if pl, ok := ss[1].(*planetScreen); !ok || pl.star != s.star || pl.idx != s.idx {
		t.Errorf("second screen %#v, want planet %d of %d", ss[1], s.idx, s.star)
	}
}

// The project panel (the original's): idle "No Project" and how to
// build; a running project its name and the days left at today's
// industry, never its cost; "no industry" without any; an uninhabited
// planet says so. The title names the planet and its system.
func TestProjectPanel(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	col := a.e.Colonies[0]
	s := newPlanetScreen(col.Star, col.PlanetIdx)
	s.Enter(a)
	col.Project = nil
	if _, pp := s.panelTexts(); pp.name != "No Project" || pp.foot != "click a square or slot to build" || pp.title != col.Planet.Name+" - "+s.sysName {
		t.Errorf("idle: %+v", pp)
	}
	at, ok := col.Planet.BestCell(game.Factory, col.Traits())
	if !ok {
		t.Fatal("no Factory square")
	}
	if err := col.Start(a.e.Research, game.Factory, at); err != nil {
		t.Fatal(err)
	}
	n, ok := col.DaysLeft()
	if !ok {
		t.Fatal("the fixture makes no industry")
	}
	_, pp := s.panelTexts()
	if want := dayCount(max(1, n)) + " until completion"; pp.name != "Factory" || pp.foot != want || strings.Contains(pp.foot, "/") {
		t.Errorf("running: %+v, want Factory / %q", pp, want)
	}
	other := -1
	for i, p := range a.cl.Stars[col.Star].System.Planets {
		if p.Owner < 0 {
			other = i
			break
		}
	}
	if other < 0 {
		t.Skip("no uninhabited planet in the home system")
	}
	u := newPlanetScreen(col.Star, other)
	u.Enter(a)
	if _, pp := u.panelTexts(); pp.name != "uninhabited" || pp.col != nil {
		t.Errorf("uninhabited: %+v", pp)
	}
}

// A bare planet draws every square as a marker, the original's (nothing
// is reachable without a colony), until a settler is picking its
// square: then all are open.
func TestBarePlanetMarkers(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	home := a.e.Colonies[0]
	sys := a.e.Cluster.Stars[a.e.Cluster.Home].System
	idx := (home.PlanetIdx + 1) % len(sys.Planets)
	if sys.Planets[idx].HasStructure() {
		t.Skip("no bare planet next to the home")
	}
	s := newPlanetScreen(a.e.Cluster.Home, idx)
	s.Enter(a)
	for _, c := range s.planet.Cells() {
		if s.open(c) {
			t.Fatalf("%v open on a bare planet", c)
		}
	}
	s.settler = &game.Starship{}
	for _, c := range s.planet.Cells() {
		if !s.open(c) {
			t.Fatalf("%v closed while the settler picks", c)
		}
	}
}
