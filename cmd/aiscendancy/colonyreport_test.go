package main

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
)

// reportApp is seed 1 with Snovemdomas and Orfa, the starmap open, a
// colony ship in orbit of a free colonisable planet at home.
func reportApp(t *testing.T) (a *app, sh *game.Starship, pl int) {
	t.Helper()
	a = testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average))
	a.push(newStarmapScreen())
	home := a.cl.Home
	pl = -1
	for i := range a.cl.Stars[home].System.Planets {
		if _, ok := a.e.Planet(home, i).BestCell(game.ColonyBase, a.e.Species.Traits()); ok && !a.e.Inhabited(home, i) {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Fatal("no colonisable planet at home")
	}
	return a, a.e.Spawn(fixtureShip, home, pl), pl
}

// A Colonise order resolved at the day's close queues one report for
// that planet, counting the colonies after the founding.
func TestReportColonise(t *testing.T) {
	a, sh, pl := reportApp(t)
	n := len(a.e.Colonies)
	if err := a.e.Order(sh, game.Order{Kind: game.Colonise}); err != nil {
		t.Fatal(err)
	}
	a.endDay()
	if a.e.Colony(a.cl.Home, pl) == nil || len(a.e.Colonies) != n+1 {
		t.Fatalf("not founded: %d colonies", len(a.e.Colonies))
	}
	want := []colonyReport{{a.cl.Home, pl, a.reportText(n + 1)}}
	if !slices.Equal(a.colReports, want) || !a.reporting() {
		t.Fatalf("queue %+v, want %+v", a.colReports, want)
	}
	a.endDay()
	if len(a.colReports) != 1 {
		t.Errorf("a quiet day queued %+v", a.colReports)
	}
}

// The planet screen's Create Colony queues one at once.
func TestReportCreateColony(t *testing.T) {
	a, sh, pl := reportApp(t)
	s := newPlanetScreen(a.cl.Home, pl)
	a.push(s)
	c, _ := s.planet.BestCell(game.ColonyBase, a.e.Species.Traits())
	s.settler, s.cell, s.cellOK = sh, c, true
	if err := s.foundHere(a); err != nil {
		t.Fatal(err)
	}
	want := []colonyReport{{a.cl.Home, pl, a.reportText(len(a.e.Colonies))}}
	if !slices.Equal(a.colReports, want) || !a.reporting() {
		t.Errorf("queue %+v, want %+v", a.colReports, want)
	}
}

// The two texts: the largest known empire when bigger than ours (an
// unknown one never counts), else ours; a tie is ours; "colony" for one.
func TestReportText(t *testing.T) {
	a, _, _ := reportApp(t)
	us := a.e.Species.String()
	orfa := a.g.Empires[1]
	for len(orfa.Colonies) < 3 {
		orfa.Colonies = append(orfa.Colonies, &game.Colony{Star: -1, PlanetIdx: -1})
	}
	max1 := us + " Colony Report: Our empire now contains 1 colony.  It is the largest known empire in the galaxy."
	if got := a.reportText(1); got != max1 {
		t.Errorf("unmet: %q, want %q", got, max1)
	}
	a.g.Contact(0, 1)
	want := us + " Colony Report: Our empire now contains 2 colonies.  The largest known empire belongs to the " +
		orfa.Species.String() + ", who have 3."
	if got := a.reportText(2); got != want {
		t.Errorf("met: %q, want %q", got, want)
	}
	tie := us + " Colony Report: Our empire now contains 3 colonies.  It is the largest known empire in the galaxy."
	if got := a.reportText(len(orfa.Colonies)); got != tie {
		t.Errorf("tie: %q, want %q", got, tie)
	}
}

// OK, a closing key and a click outside pop the head, a click inside
// does nothing; it waits for the explorations, the ability popup waits
// for it; Space runs no day; F9's restart and New game drop it.
func TestReportClose(t *testing.T) {
	a, _, pl := reportApp(t)
	home := a.cl.Home
	a.colReports = []colonyReport{{home, pl, "a"}, {home, pl, "b"}, {home, pl, "c"}, {home, pl, "d"}}
	l := reportLayout(a.colReports[0])
	if l.box != dlgBox || l.ok != dlgBar || !inside(l.globe, l.box) || l.globe.Y1 > (l.box.Y0+l.box.Y1)/2 || l.textY < l.globe.Y1 {
		t.Errorf("box %+v, OK %+v, globe %+v, text %d", l.box, l.ok, l.globe, l.textY)
	}
	a.reportPress(false, true, float64(l.globe.X0+1), float64(l.globe.Y0+1))
	if len(a.colReports) != 4 {
		t.Fatal("a click inside closed it")
	}
	a.reportPress(false, true, float64(l.ok.X0+1), float64(l.ok.Y0+1))
	a.reportPress(true, false, 0, 0)
	a.reportPress(false, true, float64(l.box.X0-1), float64(l.box.Y0))
	if !slices.Equal(a.colReports, []colonyReport{{home, pl, "d"}}) {
		t.Fatalf("after three closes: %+v", a.colReports)
	}
	a.explored = []exploration{{home, "x"}}
	a.charged = true
	if a.reporting() || !a.exploring() {
		t.Fatal("up over an exploration")
	}
	a.explored = nil
	if !a.reporting() || a.charging() || a.keysSection() != keysPopups {
		t.Fatalf("reporting %v, charging %v", a.reporting(), a.charging())
	}
	day := a.g.Day
	pressing(t, ebiten.KeySpace)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	pressing(t, ebiten.KeyEnter)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if a.g.Day != day || len(a.colReports) != 0 || !a.charging() {
		t.Fatalf("day %d -> %d, queue %+v, charging %v", day, a.g.Day, a.colReports, a.charging())
	}
	a.colReports = []colonyReport{{home, pl, "a"}}
	a.restart()
	if a.colReports != nil {
		t.Errorf("restart kept %+v", a.colReports)
	}
	a.colReports = []colonyReport{{home, pl, "a"}}
	if err := a.answerEnd(true); err != nil || a.colReports != nil || a.reporting() {
		t.Errorf("New game kept %+v (%v)", a.colReports, err)
	}
}

// A founding and a capture the same day: the report counts the colonies
// as after the founding (the invasions run after every empire's day),
// not the capture's one more.
func TestReportCountBeforeCapture(t *testing.T) {
	a, _, inv, _ := invadeApp(t)
	if err := a.e.Order(inv, game.Order{Kind: game.Invade}); err != nil {
		t.Fatal(err)
	}
	home := a.e.Colonies[0].Star
	pl := -1
	for i := range a.cl.Stars[home].System.Planets {
		if _, ok := a.e.Planet(home, i).BestCell(game.ColonyBase, a.e.Species.Traits()); ok && !a.e.Inhabited(home, i) {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Fatal("no colonisable planet at home")
	}
	sh := a.e.Spawn(fixtureShip, home, pl)
	if err := a.e.Order(sh, game.Order{Kind: game.Colonise}); err != nil {
		t.Fatal(err)
	}
	n := len(a.e.Colonies)
	a.endDay()
	if len(a.e.Colonies) != n+2 {
		t.Fatalf("%d colonies after, want %d (founded and captured)", len(a.e.Colonies), n+2)
	}
	if want := a.reportText(n + 1); len(a.colReports) != 1 || a.colReports[0].text != want {
		t.Errorf("reports %+v, want one %q", a.colReports, want)
	}
}
