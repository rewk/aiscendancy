package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// exploreApp is a lone Snovemdomas empire on seed 1, a colony ship at
// home, the starmap open, and an unknown neighbour of home.
func exploreApp(t *testing.T) (a *app, sh *game.Starship, far int) {
	t.Helper()
	a = testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	a.push(newStarmapScreen())
	home := a.cl.Home
	far = -1
	for _, li := range a.cl.Stars[home].Lanes {
		if o := a.cl.Other(li, home); !a.e.Knows(o) {
			far = o
			break
		}
	}
	if far < 0 {
		t.Fatal("every neighbour of home known")
	}
	return a, a.e.Spawn(fixtureShip, home, 0), far
}

// sail orders sh to star and runs endDay until it sits there out of a
// lane, reporting whether an exploration was queued before that day.
func sail(t *testing.T, a *app, sh *game.Starship, star int) (early bool) {
	t.Helper()
	if err := a.e.Order(sh, game.Order{Kind: game.GoToStar, Star: star}); err != nil {
		t.Fatal(err)
	}
	for d := 0; sh.Star != star || sh.Lane >= 0; d++ {
		if d > 100 {
			t.Fatalf("not at %d after %d days: %+v", star, d, sh)
		}
		early = early || len(a.explored) > 0
		a.endDay()
	}
	return early
}

// A ship reaching an unknown star queues one exploration, named after
// it, on its arrival day only; a later visit queues nothing.
func TestExploreArrival(t *testing.T) {
	a, sh, far := exploreApp(t)
	home := a.cl.Home
	if sail(t, a, sh, far) {
		t.Fatal("queued before the arrival")
	}
	want := []exploration{{far, sh.Name}}
	if !slices.Equal(a.explored, want) || !a.exploring() {
		t.Fatalf("queue %+v, want %+v", a.explored, want)
	}
	a.endDay()
	if !slices.Equal(a.explored, want) {
		t.Fatalf("a day at the star: %+v", a.explored)
	}
	a.explored = nil
	sail(t, a, sh, home)
	sail(t, a, sh, far)
	if len(a.explored) != 0 {
		t.Errorf("a later visit queued %+v", a.explored)
	}
}

// A star made known with no ship of ours there queues nothing; with one
// it does; a star past visited's length counts as unvisited; stars come
// in index order, the first ship there naming them.
func TestExploreDiff(t *testing.T) {
	a, sh, far := exploreApp(t)
	visited := slices.Clone(a.e.Visited)
	a.e.Visit(far)
	a.explore(visited)
	if len(a.explored) != 0 {
		t.Fatalf("no ship there: %+v", a.explored)
	}
	two := a.e.Spawn(fixtureShip, far, 0)
	a.e.Spawn(fixtureShip, far, 0)
	a.explore(visited)
	if want := []exploration{{far, two.Name}}; !slices.Equal(a.explored, want) {
		t.Fatalf("queue %+v, want %+v", a.explored, want)
	}
	a.explored = nil
	a.explore(nil)
	var want []exploration
	for star := range a.cl.Stars {
		switch star {
		case sh.Star:
			want = append(want, exploration{star, sh.Name})
		case far:
			want = append(want, exploration{star, two.Name})
		}
	}
	if !slices.Equal(a.explored, want) {
		t.Errorf("past the length: %+v, want %+v", a.explored, want)
	}
}

// The text is help.txt's event20 with the star's planets and lanes, in
// the singular for one.
func TestExploreText(t *testing.T) {
	a, _, far := exploreApp(t)
	st := &a.cl.Stars[far]
	np, nl := len(st.System.Planets), len(st.Lanes)
	got := a.exploreMessage(exploration{far, "Colonizer I"})
	if !strings.HasPrefix(got, "Colonizer I: We have discovered the "+st.Name+" System.  It contains ") ||
		!strings.HasSuffix(got, ".  We're awaiting further orders.") {
		t.Errorf("text %q", got)
	}
	for _, c := range []struct {
		n    int
		word string
	}{{np, "planet"}, {nl, "star lane"}} {
		w := fmt.Sprintf(" %d %s", c.n, c.word)
		if c.n != 1 {
			w += "s"
		}
		if !strings.Contains(got, w+" ") && !strings.Contains(got, w+".") {
			t.Errorf("%q lacks %q", got, w)
		}
	}
	if np < 2 || nl < 2 {
		t.Fatalf("star %d: %d planets, %d lanes: pick a bigger one", far, np, nl)
	}
	st.System.Planets, st.Lanes = st.System.Planets[:1], st.Lanes[:1]
	want := "Colonizer I: We have discovered the " + st.Name + " System.  It contains 1 planet and 1 star lane.  We're awaiting further orders."
	if got := a.exploreMessage(exploration{far, "Colonizer I"}); got != want {
		t.Errorf("singular %q, want %q", got, want)
	}
}

// OK, a closing key and a click outside pop the head; a click inside
// elsewhere does nothing; Go to System drops to the starmap and opens
// the star's system, the next head then up over it.
func TestExploreClose(t *testing.T) {
	a, sh, far := exploreApp(t)
	home := a.cl.Home
	a.explored = []exploration{{far, sh.Name}, {home, sh.Name}, {far, "x"}, {home, "y"}}
	l := exploreLayout(a, a.explored[0])
	if l.box != dlgBox || l.goTo.X1 != l.ok.X0 || l.goTo.X0 != dlgBar.X0 || l.ok.X1 != dlgBar.X1 || l.ok.Y0 != dlgBar.Y0 {
		t.Errorf("box %+v, Go %+v, OK %+v", l.box, l.goTo, l.ok)
	}
	a.explorePress(false, true, float64(l.star.X0+1), float64(l.star.Y0+1))
	a.explorePress(false, true, float64(l.goTo.X0+l.goTo.X1)/2, float64(l.goTo.Y0+l.goTo.Y1)/2) // Go is not a press
	if len(a.explored) != 4 {
		t.Fatalf("a click inside closed it: %+v", a.explored)
	}
	a.explorePress(false, true, float64(l.ok.X0+l.ok.X1)/2, float64(l.ok.Y0+l.ok.Y1)/2)
	a.explorePress(true, false, 0, 0)
	if len(a.explored) != 2 {
		t.Fatalf("OK and a key: %+v", a.explored)
	}
	a.push(newLogScreen())
	a.push(newSystemScreen(home))
	a.exploreGo()
	v := top[*systemScreen](t, a)
	if v.star != far || len(a.stack) != 2 {
		t.Fatalf("Go: star %d, stack %d", v.star, len(a.stack))
	}
	top := a.stack[0]
	if _, ok := top.(*starmapScreen); !ok || !a.exploring() || a.explored[0].star != home {
		t.Fatalf("under %T, up %v, queue %+v", top, a.exploring(), a.explored)
	}
	a.explorePress(false, true, float64(l.box.X0-1), float64(l.box.Y0))
	if len(a.explored) != 0 || a.exploring() {
		t.Errorf("a click outside: %+v", a.explored)
	}
}

// Under the popup Space runs no day and G goes to the system; it waits
// for the discoveries, the ability popup waits for it; F9's restart and
// New game drop it.
func TestExploreOrder(t *testing.T) {
	a, sh, far := exploreApp(t)
	a.explored = []exploration{{far, sh.Name}}
	a.discoveries = []discovery{{1, "x"}}
	a.charged = true
	if a.exploring() || !a.discovering() {
		t.Fatal("up over a discovery")
	}
	a.discoveries = nil
	if !a.exploring() || a.charging() {
		t.Fatalf("exploring %v, charging %v", a.exploring(), a.charging())
	}
	if a.keysSection() != keysPopups {
		t.Errorf("section %q", a.keysSection())
	}
	day := a.g.Day
	for _, k := range []ebiten.Key{ebiten.KeySpace, ebiten.KeyG} {
		pressing(t, k)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if a.g.Day != day || len(a.explored) != 0 || !a.charging() {
		t.Fatalf("day %d -> %d, queue %+v, charging %v", day, a.g.Day, a.explored, a.charging())
	}
	if v := top[*systemScreen](t, a); v.star != far {
		t.Errorf("G opened star %d", v.star)
	}
	v := a.top().(*systemScreen)
	a.explored = []exploration{{far, "x"}}
	v.replay = &replay{}
	if a.exploring() {
		t.Error("up over a replay")
	}
	v.replay = nil
	a.restart()
	if a.explored != nil {
		t.Errorf("restart kept %+v", a.explored)
	}
	a.explored = []exploration{{far, "x"}}
	if err := a.answerEnd(true); err != nil || a.explored != nil || a.exploring() {
		t.Errorf("New game kept %+v (%v)", a.explored, err)
	}
}

// Every star of a few clusters fits the box, and one padded to
// MaxSystemPlanets (the generator rolls fewer): the row inside the room,
// cells at least exploreMinCell wide, each globe and name within its
// cell, the whole text under the pictures and above the bar.
func TestExploreFits(t *testing.T) {
	const exploreMinCell = 48
	c := discContent()
	check := func(a *app, star int, what string) {
		t.Helper()
		x := exploration{star, "Lane Colonizer 12"}
		l := exploreLayout(a, x)
		if len(l.cells) != len(a.cl.Stars[star].System.Planets) {
			t.Errorf("%s: %d cells", what, len(l.cells))
		}
		if l.star.X0 < c.X0 || l.star.Y0 < c.Y0 || l.textY < l.star.Y1 || l.textY+len(l.lines)*exploreLineH > c.Y1 {
			t.Errorf("%s: star %+v, text [%d,+%d lines)", what, l.star, l.textY, len(l.lines))
		}
		for i, r := range l.cells {
			if r.X1-r.X0 < exploreMinCell || r.X1 > c.X1 || r.X0 < l.star.X1 || !inside(l.globes[i], r) ||
				ui.Width(l.names[i], smallFace) > float64(r.X1-r.X0) || l.nameY+exploreNameH > r.Y1 {
				t.Errorf("%s cell %d: %+v, globe %+v, name %q", what, i, r, l.globes[i], l.names[i])
			}
		}
		for _, s := range l.lines {
			if ui.Width(s, bigFace) > float64(c.X1-c.X0) {
				t.Errorf("%s: %q too wide", what, s)
			}
		}
		if !slices.Equal(strings.Fields(strings.Join(l.lines, " ")), strings.Fields(a.exploreMessage(x))) {
			t.Errorf("%s: lines %q cut", what, l.lines)
		}
	}
	for _, d := range []game.Density{game.Average, game.VeryDense} {
		for seed := uint64(1); seed <= 4; seed++ {
			cl := game.NewCluster(seed, d)
			for star := range cl.Stars {
				check(&app{cl: cl}, star, fmt.Sprintf("%v %d star %d", d, seed, star))
			}
		}
	}
	cl := game.NewCluster(1, game.Average)
	sys := cl.Stars[cl.Home].System
	for len(sys.Planets) < game.MaxSystemPlanets {
		sys.Planets = append(sys.Planets, sys.Planets[0])
	}
	check(&app{cl: cl}, cl.Home, "padded")
}

// inside reports q within r.
func inside(q, r rect) bool { return q.X0 >= r.X0 && q.Y0 >= r.Y0 && q.X1 <= r.X1 && q.Y1 <= r.Y1 }

// Both popups draw every star of the cluster with the game's sprites
// (headless; no panic, no frame out of its bank).
func TestExploreDraw(t *testing.T) {
	a, _, _ := helpApp(t)
	frame := ebiten.NewImage(screenW, screenH)
	for star, st := range a.cl.Stars {
		a.explored = []exploration{{star, "Colonizer I"}}
		a.drawExplore(frame)
		for i := range st.System.Planets {
			a.colReports = []colonyReport{{star, i, a.reportText(1)}}
			a.drawReport(frame)
		}
	}
}

// The notice names the other empires holding planets of the star, in
// empire order, as "The A, B and C have colonies here.", before the
// orders' sentence; the player's own planets name nobody.
func TestExploreOwners(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Govorom}, game.Average))
	star := -1
	for i, st := range a.cl.Stars {
		if len(st.System.Planets) >= 3 && slices.IndexFunc(st.System.Planets, func(p *game.Planet) bool { return p.Owner >= 0 }) < 0 {
			star = i
			break
		}
	}
	if star < 0 {
		t.Fatal("no free star of 3 planets")
	}
	ps := a.cl.Stars[star].System.Planets
	msg := func() string { return a.exploreMessage(exploration{star, "S"}) }
	if strings.Contains(msg(), "colonies") {
		t.Errorf("nobody's: %q", msg())
	}
	ps[0].Owner = 0
	if strings.Contains(msg(), "colonies") || len(a.exploreOwners(star)) != 0 {
		t.Errorf("the player's own: %q", msg())
	}
	ps[1].Owner, ps[2].Owner = 3, 1
	if want := "The Orfa and Govorom have colonies here.  We're awaiting further orders."; !strings.HasSuffix(msg(), want) {
		t.Errorf("two: %q, want the suffix %q", msg(), want)
	}
	ps[0].Owner = 2
	if want := "  The Orfa, Mebes and Govorom have colonies here.  We're"; !strings.Contains(msg(), want) {
		t.Errorf("three: %q, want %q", msg(), want)
	}
	if got := andList([]string{"A"}); got != "A" {
		t.Errorf("one: %q", got)
	}
}
