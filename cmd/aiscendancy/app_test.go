package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
	"github.com/rewk/aiscendancy/ui"
)

// Every pixel the chrome leaves transparent is in the big area or in the
// trim, so nothing the 3D view spills (a zoom, a pan) shows through the
// chrome's button holes right of the area. The starmap's and the system's
// chromes, both of the 1starmap/16battle family.
func TestChromeTrimsTheScene(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	a, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, c := range []struct {
		bank       string
		area, trim rect
	}{
		{"1starmap.tmp", mapArea, mapTrim},
		{"16battle.tmp", sysArea, sysTrim},
	} {
		b, err := a.Bank(c.bank)
		if err != nil {
			t.Fatal(err)
		}
		// The frame's pixels are original px; one covers the zoom x zoom
		// native block from (zoom·x, zoom·y), and every rect edge is a
		// multiple of zoom, so its corner decides.
		f, n := b.Frames[0], 0
		for y := range screenH / zoom {
			for x := range screenW / zoom {
				p := image.Pt(x, y)
				if p.In(f.Box) && f.Opaque[(y-f.Box.Min.Y)*f.Box.Dx()+x-f.Box.Min.X] {
					continue
				}
				if fx, fy := float64(zoom*x), float64(zoom*y); !c.area.has(fx, fy) && !c.trim.has(fx, fy) {
					n++
				}
			}
		}
		if n > 0 {
			t.Errorf("%s: %d transparent chrome pixels outside the area and the trim", c.bank, n)
		}
	}
}

// A press and release within clickSlop is a click, wherever the cursor
// strayed in between; a press outside the area is never one.
func TestDrag(t *testing.T) {
	var d drag
	d.press(100, 100, true)
	if dx, dy := d.move(100+zoom, 100+2*zoom); dx != zoom || dy != 2*zoom {
		t.Errorf("move: %v %v", dx, dy)
	}
	if !d.release(100+zoom, 100+zoom) {
		t.Error("a 2 original px wobble is a click")
	}
	d.press(100, 100, true)
	d.move(110, 100)
	if d.release(100, 100) {
		t.Error("a drag back to the start is not a click")
	}
	d.press(100, 100, true)
	if d.release(100+clickSlop, 100) {
		t.Error("a clickSlop move is a drag")
	}
	d.press(100, 100, false)
	if dx, dy := d.move(150, 100); dx != 0 || dy != 0 {
		t.Errorf("no drag is on, yet the move is %v %v", dx, dy)
	}
	if d.release(100, 100) {
		t.Error("a press outside the area is not a click")
	}
	if d.release(100, 100) {
		t.Error("a release with no press is not a click")
	}
}

// The tests' names for game's fixtures and save's log bound.
var fixtureShip, warship, foeShip = game.FixtureColonyShip, game.FixtureGunboat, game.FixtureRaider

const logKeep = save.LogKeep

// spawnFixtures is game.SpawnFixtures on a's game (the player is empire 0).
func spawnFixtures(a *app, ships, warships, foes int) error {
	return game.SpawnFixtures(a.g, ships, warships, foes)
}

// testApp is a window on g playing empire 0, with no assets, no images
// and no bots: Enter draws nothing.
func testApp(g *game.Game) *app {
	return &app{g: g, player: 0, e: g.Empires[0], cl: g.Cluster}
}

func top[S screen](t *testing.T, a *app) S {
	t.Helper()
	s, ok := a.top().(S)
	if !ok {
		t.Fatalf("top screen is %T", a.top())
	}
	return s
}

// The clicks push the screen of what was clicked, with the right star and
// planet indices; Esc (pop) walks back and never empties the stack.
func TestNavigation(t *testing.T) {
	cl := game.NewCluster(1, game.Average)
	a := &app{cl: cl} // no assets, no images: Enter draws nothing
	a.open([]screen{newStarmapScreen()})
	m := top[*starmapScreen](t, a)

	// A click in the void opens nothing.
	if pickStar(m.project(a), float64(mapArea.X0), float64(mapArea.Y0)) >= 0 {
		t.Fatal("a star in the area's corner")
	}
	m.click(a, float64(mapArea.X0), float64(mapArea.Y0))
	if len(a.stack) != 1 {
		t.Fatalf("void click pushed %T", a.top())
	}

	// Click the home star.
	home := cl.Home
	p := m.project(a)[home]
	if !mapArea.has(p.X, p.Y) || pickStar(m.project(a), p.X, p.Y) != home {
		t.Fatalf("home star at %+v not pickable", p)
	}
	m.click(a, p.X+1, p.Y)
	s := top[*systemScreen](t, a)
	if s.star != home || s.sys != cl.Stars[home].System {
		t.Fatalf("clicked star %d, opened star %d", home, s.star)
	}

	// Travel each pickable lane endpoint: it leads to the far end of the
	// cluster lane Star.Lanes[k], and Esc comes back.
	travelled := 0
	for k := range s.sys.Lanes {
		pt := s.cam.Project(s.sys.Lanes[k].Pos)
		if b := pickBody(s.placeAll(), pt.X, pt.Y); !sysArea.has(pt.X, pt.Y) || b == nil || b.lane != &s.sys.Lanes[k] {
			continue
		}
		s.click(a, pt.X, pt.Y)
		far := top[*systemScreen](t, a)
		if want := cl.Other(cl.Stars[home].Lanes[k], home); far.star != want || far.sys != cl.Stars[want].System {
			t.Errorf("lane %d: opened star %d, the lane leads to %d", k, far.star, want)
		}
		if err := a.pop(); err != nil || top[*systemScreen](t, a) != s {
			t.Fatalf("Esc from the far system: %v, top %T", err, a.top())
		}
		travelled++
	}
	if travelled == 0 {
		t.Error("no lane endpoint could be clicked")
	}

	// Click each pickable planet.
	opened := 0
	for i, pl := range s.sys.Planets {
		pt := s.cam.Project(pl.Pos)
		if b := pickBody(s.placeAll(), pt.X, pt.Y); !sysArea.has(pt.X, pt.Y) || b == nil || b.planet != pl {
			continue
		}
		s.click(a, pt.X, pt.Y) // selects it
		if s.psel != pl || len(a.stack) != 2 {
			t.Errorf("planet %d: first click selected %v, %d screens", i, s.psel, len(a.stack))
		}
		s.click(a, pt.X, pt.Y) // opens it
		ps := top[*planetScreen](t, a)
		if ps.star != home || ps.idx != i || ps.planet != pl || ps.grid == nil || ps.sysName != s.sys.Name {
			t.Errorf("planet %d: opened %+v", i, ps)
		}
		if err := a.pop(); err != nil {
			t.Fatal(err)
		}
		opened++
	}
	if opened == 0 {
		t.Error("no planet could be clicked")
	}

	if err := a.pop(); err != nil || len(a.stack) != 1 {
		t.Fatalf("Esc from the system: %v, %d screens", err, len(a.stack))
	}
	for range 2 {
		if err := a.pop(); !errors.Is(err, ebiten.Termination) || len(a.stack) != 1 || a.top() != m {
			t.Fatalf("Esc on the starmap: %v, %d screens", err, len(a.stack))
		}
	}
}

// -star and -planet build the stack under the screen they open, and are
// checked against the cluster.
func TestShotFlags(t *testing.T) {
	cl := game.NewCluster(1, game.Average)
	star := -1
	for i, s := range cl.Stars {
		if len(s.System.Planets) >= 2 {
			star = i
			break
		}
	}
	if star < 0 {
		t.Fatal("no star with 2 planets")
	}
	n := len(cl.Stars[star].System.Planets)

	ss, err := initialStack(cl, -1, -1)
	if err != nil || len(ss) != 1 {
		t.Fatalf("no flags: %d screens, %v", len(ss), err)
	}
	if _, ok := ss[0].(*starmapScreen); !ok {
		t.Fatalf("no flags: %T", ss[0])
	}

	ss, err = initialStack(cl, star, -1)
	if err != nil || len(ss) != 2 {
		t.Fatalf("-star: %d screens, %v", len(ss), err)
	}
	if s, ok := ss[1].(*systemScreen); !ok || s.star != star {
		t.Fatalf("-star: %+v", ss[1])
	}

	ss, err = initialStack(cl, star, n-1)
	if err != nil || len(ss) != 3 {
		t.Fatalf("-star -planet: %d screens, %v", len(ss), err)
	}
	a := &app{cl: cl}
	a.open(ss)
	p := top[*planetScreen](t, a)
	if p.star != star || p.idx != n-1 || p.planet != cl.Stars[star].System.Planets[n-1] {
		t.Fatalf("-star -planet: %+v", p)
	}
	if s := a.stack[1].(*systemScreen); s.sys != cl.Stars[star].System {
		t.Fatal("the system under the planet was not entered")
	}

	for _, c := range [][2]int{{-1, 0}, {len(cl.Stars), -1}, {star, n}} {
		if _, err := initialStack(cl, c[0], c[1]); err == nil {
			t.Errorf("-star %d -planet %d accepted", c[0], c[1])
		}
	}
}

// Without -load (no cluster) the program opens on the opening screen
// alone, with no game, and -star or -planet is an error; its New Game
// leaves the New Game screen alone.
func TestInitialStackNewGame(t *testing.T) {
	ss, err := initialStack(nil, -1, -1)
	if err != nil || len(ss) != 1 {
		t.Fatalf("%d screens, %v", len(ss), err)
	}
	if _, ok := ss[0].(*openingScreen); !ok {
		t.Fatalf("%T", ss[0])
	}
	for _, c := range [][2]int{{3, -1}, {-1, 0}, {3, 0}} {
		if _, err := initialStack(nil, c[0], c[1]); err == nil {
			t.Errorf("-star %d -planet %d accepted without -load", c[0], c[1])
		}
	}
	a := &app{}
	a.open(ss)
	if err := top[*openingScreen](t, a).choose(a, openNew); err != nil || len(a.stack) != 1 {
		t.Fatalf("New Game: %v, %d screens", err, len(a.stack))
	}
	if a.inGame() {
		t.Fatal("a game on the New Game screen")
	}
	if w := where(a); w != "new game: Snovemdomas, Green, Average, 3 empires, Neutral, Original bots, seed 1" {
		t.Fatalf("where: %q", w)
	}
}

// endDay lets the bot act, runs the day and returns its events, appended
// to the log. runUntilEvent stops on the first day with any event,
// growth included: a twin empire replayed day by day had none before.
func TestEndDayBot(t *testing.T) {
	newBotApp := func() *app {
		a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
		a.bots = []*bot.Bot{bot.New(a.e, bot.Default())}
		return a
	}
	a := newBotApp()
	a.endDay()
	if a.e.Day != 1 || a.e.Colonies[0].Project == nil || a.e.Research.Topic == game.NoTech {
		t.Fatalf("day %d: the bot did not act (project %v, topic %v)", a.e.Day, a.e.Colonies[0].Project, a.e.Research.Topic)
	}

	twin := newBotApp()
	twin.endDay()
	for range 3 {
		n, last := a.runUntilEvent()
		if n < 1 || n > maxSkip {
			t.Fatalf("ran %d days", n)
		}
		for d := 1; d <= n; d++ {
			ev := twin.endDay()
			if d < n && len(ev) > 0 {
				t.Fatalf("day %d had events %v, runUntilEvent ran on to day %d", twin.e.Day, ev, a.e.Day)
			}
			if d == n && len(ev) == 0 && n < maxSkip {
				t.Fatalf("stopped on day %d with no event", a.e.Day)
			}
			if d == n && !slices.Equal(ev, last) {
				t.Fatalf("day %d: returned %v, the twin's %v", a.e.Day, last, ev)
			}
		}
		if twin.e.Day != a.e.Day || !slices.Equal(twin.log, a.log) {
			t.Fatalf("twin at day %d with %v, app at %d with %v", twin.e.Day, twin.log, a.e.Day, a.log)
		}
	}
}

// With no bot and nothing running, the skip stops on the growth that
// gives a colony with no free people a worker (GrowthDays), not on one
// adding to free people; full, it runs to maxSkip. Growth never pops the
// log, and only the freeing one is logged, Idle (red: an order is due).
func TestRunUntilEventCap(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	c.Project = nil
	e.Research.Topic = game.NoTech
	a := testApp(g)
	c.Pop = c.Busy() // nobody free: the next unit is a worker
	n0, ok := c.GrowthDays()
	if !ok {
		t.Fatal("the home does not grow: the fixture proves nothing")
	}
	if n, ev := a.runUntilEvent(); n != n0 || len(ev) != 1 || ev[0].Kind != game.Grew {
		t.Errorf("freeing growth: ran %d days (want %d), last %v", n, n0, ev)
	}
	if c.Free() != 1 || len(a.log) != 1 || !a.log[0].Idle {
		t.Fatalf("free %d, log %v (the freeing growth on an idle colony is red)", c.Free(), a.log)
	}
	c.Growth = game.GrowthCost - 1 // the next unit tomorrow, one free already: no stop
	n1, ok := c.GrowthDays()
	if !ok || n1 >= maxSkip {
		t.Fatalf("the next growth in %d days %v: the fixture proves nothing", n1, ok)
	}
	pop := c.Pop
	if n, ev := a.runUntilEvent(); n != maxSkip || len(ev) != 0 || len(a.log) != 1 || c.Pop <= pop {
		t.Errorf("growth with people free: ran %d days, last %v, log %v, pop %d (was %d)", n, ev, a.log, c.Pop, pop)
	}
	c.Pop = c.Capacity()
	day := e.Day
	if n, ev := a.runUntilEvent(); n != maxSkip || e.Day != day+maxSkip || len(ev) != 0 {
		t.Errorf("full: ran %d days, day %d, last %v", n, e.Day, ev)
	}
}

// endDay runs every empire of the game, each bot acting: the player's
// none when it has no bot (its home starts nothing), the others'
// always. The log takes the player's events only: those a twin game,
// played by the same bots through Game.EndDay, gives empire 0.
func TestEndDayEmpires(t *testing.T) {
	sp := game.PickSpecies(1, game.Snovemdomas, 3)
	newGameApp := func() *app {
		a := testApp(game.NewGame(1, sp, game.Average))
		a.bots = bot.NewAll(a.g, bot.Default())
		a.bots[a.player] = nil
		return a
	}
	a, twin := newGameApp(), newGameApp()
	others := 0 // the other empires' events: the log must not hold them
	var want []logEntry
	for range 60 {
		a.endDay()
		for _, b := range twin.bots[1:] {
			b.Act()
		}
		free := map[*game.Colony]int{}
		for _, c := range twin.e.Colonies {
			free[c] = c.Free()
		}
		evs := twin.g.EndDay()
		for _, b := range twin.bots[1:] {
			b.Note()
		}
		for _, x := range popNews(twin.e, free, evs[0]) { // growth only when it frees a worker
			want = append(want, logEntry{Day: twin.g.Day, Ev: x})
		}
		others += len(evs[1]) + len(evs[2])
	}
	for i, e := range a.g.Empires {
		if e.Day != 60 || a.g.Day != 60 {
			t.Fatalf("empire %d at day %d, game at %d", i, e.Day, a.g.Day)
		}
		if acted := e.Colonies[0].Project != nil || e.Research.Topic != game.NoTech; acted != (i != a.player) {
			t.Errorf("empire %d: played %v (project %v, topic %v)", i, acted, e.Colonies[0].Project, e.Research.Topic)
		}
	}
	if others == 0 {
		t.Fatal("the other empires had no event in 60 days: a weak fixture")
	}
	if !slices.Equal(a.log, want) {
		t.Errorf("log %v, want empire 0's %v", a.log, want)
	}
}

// F5's save and F9's load: a 3-empire game written by one app comes
// back into another (on another game, deep in a system screen) with the
// same day, empires, ships, log and every bot resumed, on a fresh stack
// of one starmap screen over the loaded cluster. A save with no bot at
// all leaves the player's empire none and every other a fresh bot; a
// failed load changes nothing.
func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "q", "quick.json")
	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	a := testApp(g)
	a.bots, a.savePath = bot.NewAll(g, bot.Default()), path
	for range 350 {
		a.endDay()
	}
	e := a.e
	if len(e.Ships)+len(e.Colonies) < 3 || len(a.log) == 0 {
		t.Fatalf("day 350: %d ships, %d colonies, %d logged: a weak fixture", len(e.Ships), len(e.Colonies), len(a.log))
	}
	a.planetsIdleFirst, a.planetsSort = true, sortKeys-1
	a.save()
	if a.toastErr || a.tick >= a.toastUntil {
		t.Fatalf("save: toast %q (err %v)", a.toast, a.toastErr)
	}

	other := game.NewGame(2, []game.Species{game.Snovemdomas, game.Orfa}, game.Dense)
	b := testApp(other)
	b.savePath = path
	b.open([]screen{newStarmapScreen(), newSystemScreen(other.Cluster.Home)})
	b.load()
	if b.toastErr {
		t.Fatalf("load: %s", b.toast)
	}
	if b.g == other || b.e != b.g.Empires[0] || b.player != 0 || b.cl != b.g.Cluster || b.cl != b.e.Cluster || b.g.Seed != 1 {
		t.Fatalf("load kept the old game or cluster (seed %d)", b.g.Seed)
	}
	if len(b.g.Empires) != 3 || b.g.Day != 350 {
		t.Fatalf("loaded %d empires on day %d", len(b.g.Empires), b.g.Day)
	}
	for i, le := range b.g.Empires {
		ae := g.Empires[i]
		if le.Species != ae.Species || le.Day != 350 || len(le.Ships) != len(ae.Ships) || len(le.Colonies) != len(ae.Colonies) {
			t.Errorf("empire %d: %s day %d, %d ships, %d colonies; want %s 350, %d, %d", i, le.Species, le.Day,
				len(le.Ships), len(le.Colonies), ae.Species, len(ae.Ships), len(ae.Colonies))
		}
		if bb := b.bots[i]; bb == nil || bb.E != le || len(bb.Founded) != len(a.bots[i].Founded) {
			t.Errorf("bot %d not resumed on the loaded empire: %v", i, bb)
		}
	}
	if !b.planetsIdleFirst || b.planetsSort != sortKeys-1 {
		t.Errorf("planets order: idle first %v, sort %d; want true, %d", b.planetsIdleFirst, b.planetsSort, sortKeys-1)
	}
	if !slices.Equal(b.log, a.log) {
		t.Errorf("log: %d entries, want %d", len(b.log), len(a.log))
	}
	if len(b.stack) != 1 {
		t.Fatalf("stack of %d screens after load", len(b.stack))
	}
	if _, ok := b.stack[0].(*starmapScreen); !ok {
		t.Fatalf("stack holds %T", b.stack[0])
	}
	// Both play on alike.
	for range 30 {
		if ea, eb := a.endDay(), b.endDay(); !slices.Equal(ea, eb) {
			t.Fatalf("day %d: %v, loaded %v", a.e.Day, ea, eb)
		}
	}

	// A save with no bot: the player's empire gets none, the others a
	// fresh one each.
	c := testApp(other)
	c.savePath = path
	if err := save.Write(path, c.file()); err != nil {
		t.Fatal(err)
	}
	if err := b.loadFrom(path); err != nil {
		t.Fatal(err)
	}
	if b.e.Day != 0 || b.g.Seed != 2 || len(b.log) != 0 || len(b.bots) != 2 {
		t.Fatalf("no-bot save: day %d, seed %d, %d logged, %d bots", b.e.Day, b.g.Seed, len(b.log), len(b.bots))
	}
	if b.bots[0] != nil || b.bots[1] == nil || b.bots[1].E != b.g.Empires[1] ||
		!reflect.DeepEqual(b.bots[1].Save(), bot.New(b.g.Empires[1], bot.Default()).Save()) {
		t.Errorf("no-bot save: bots %v", b.bots)
	}

	// A failed load keeps the game and the stack.
	keep, top := b.e, b.top()
	b.savePath = filepath.Join(t.TempDir(), "none.json")
	b.load()
	if !b.toastErr || b.e != keep || b.top() != top || len(b.stack) != 1 {
		t.Errorf("failed load: toast %q err %v, game kept %v", b.toast, b.toastErr, b.e == keep)
	}
}

// Esc on the last screen asks before quitting: the prompt comes up, No
// keeps the game, Yes ends it; above the last screen Esc pops as before.
func TestQuitPrompt(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	a.open([]screen{newStarmapScreen(), newSystemScreen(g.Cluster.Home)})
	if err := a.back(); err != nil || a.quitting || len(a.stack) != 1 {
		t.Fatalf("back above the starmap: err %v, quitting %v, stack %d", err, a.quitting, len(a.stack))
	}
	if err := a.back(); err != nil || !a.quitting || len(a.stack) != 1 {
		t.Fatalf("back on the starmap: err %v, quitting %v, stack %d", err, a.quitting, len(a.stack))
	}
	if err := a.answerQuit(false); err != nil || a.quitting {
		t.Fatalf("no: err %v, quitting %v", err, a.quitting)
	}
	a.back()
	if err := a.answerQuit(true); err != ebiten.Termination || a.quitting {
		t.Fatalf("yes: err %v, quitting %v", err, a.quitting)
	}
	if !quitYes.has(float64(quitYes.X0+1), float64(quitYes.Y0+1)) || quitYes.has(float64(quitNo.X0+1), float64(quitNo.Y0+1)) {
		t.Error("the buttons overlap or miss their own corner")
	}
}

// A freed worker is news only where nothing is being built: idle or on
// an ongoing project, not while a structure runs (its worker waits for
// the build's end).
func TestFreedOnlyIdle(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	x := game.Event{Kind: game.Grew, Star: c.Star, Planet: c.PlanetIdx}
	for _, tc := range []struct {
		pj   *game.Project
		want bool
	}{
		{nil, true},
		{&game.Project{Item: game.ScientistTakeover, Slot: -1}, true},
		{&game.Project{Item: game.EndlessParty, Slot: -1}, true},
		{&game.Project{Item: game.Factory}, false},
	} {
		c.Project = tc.pj
		c.Pop = c.Busy() + 1
		if got := freed(e, map[*game.Colony]int{c: 0}, x); got != tc.want {
			t.Errorf("project %+v: freed %v, want %v", tc.pj, got, tc.want)
		}
		if freed(e, map[*game.Colony]int{c: 1}, x) {
			t.Errorf("project %+v: one free before: freed", tc.pj)
		}
	}
}

// The freeing growth the log keeps reads "Available pop on <planet>";
// game's own Grew text ("<planet> grows to N") stays for sim and mksave.
func TestFreedGrowthText(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	c := a.e.Colonies[0]
	c.Pop, c.Growth = c.Busy(), game.GrowthCost
	if c.Free() != 0 || c.Pop >= c.Capacity() || c.Running() {
		t.Fatalf("fixture: free %d, pop %d of %d, running %v", c.Free(), c.Pop, c.Capacity(), c.Running())
	}
	a.endDay()
	want := "Available pop on " + c.Planet.Name
	if !slices.ContainsFunc(a.log, func(l logEntry) bool { return l.Ev.Kind == game.Grew && l.Ev.Text == want }) {
		t.Errorf("log %v, want a Grew %q", a.log, want)
	}
}

// Two growths of one colony in a day (a Cloning Plant's doubled bank)
// make one "Available pop" line.
func TestPopNewsOnce(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	c := e.Colonies[0]
	free := map[*game.Colony]int{c: 0}
	c.Pop = c.Busy() + 2
	grew := game.Event{Kind: game.Grew, Item: game.NoItem, Tech: game.NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1}
	other := game.Event{Kind: game.Built, Item: game.NoItem, Tech: game.NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1, Text: "x"}
	got := popNews(e, free, []game.Event{grew, other, grew})
	if c.Running() || len(got) != 2 || got[0].Text != "Available pop on "+c.Planet.Name || got[1] != other {
		t.Errorf("popNews %+v (running %v)", got, c.Running())
	}
}

// TestPushCollapsesLoops: a placed screen pushed over the stack drops back
// to its earlier instance's position; a screen without a place stacks.
func TestPushCollapsesLoops(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	a := testApp(g)
	home := g.Empires[0].Colonies[0].Star
	a.push(newStarmapScreen())
	a.push(newSystemScreen(home))
	a.push(newLogScreen())
	again := newSystemScreen(home)
	a.push(again)
	if len(a.stack) != 2 || a.top() != screen(again) {
		t.Fatalf("after the log's system: %d screens, top %T", len(a.stack), a.top())
	}
	a.push(newPlanetScreen(home, 0))
	a.push(newLogScreen())
	a.push(newLogScreen())
	if len(a.stack) != 4 {
		t.Fatalf("a second log: %d screens", len(a.stack))
	}
	a.push(newPlanetScreen(home, 0))
	if len(a.stack) != 3 {
		t.Fatalf("the planet again: %d screens", len(a.stack))
	}
	a.push(newDesignScreen())
	a.push(newDesignScreen())
	if len(a.stack) != 5 {
		t.Fatalf("two design screens: %d screens", len(a.stack))
	}
}

// The quit prompt's quick save and load (all by mouse): two buttons in
// the box, apart, under the title, above the answers; save writes the
// quick save and keeps the prompt, load closes it on the loaded game, a
// failed load keeps it; no save without a game.
func TestQuitSaveLoad(t *testing.T) {
	title := float64(quitBox.Y0+dlgTitleY) + bigFace.Size*1.25
	for _, r := range []rect{quitSave, quitLoad} {
		if r.X0 < quitBox.X0 || r.X1 > quitBox.X1 || float64(r.Y0) < title || r.Y1 > dlgBar.Y0 ||
			float64(r.X1-r.X0) < ui.Width("Quick save (F5)", bigFace)+2*padX {
			t.Errorf("button %+v: box %+v, title ends %.0f, bar at %d", r, quitBox, title, dlgBar.Y0)
		}
	}
	if quitSave.X1 > quitLoad.X0 {
		t.Errorf("save %+v overlaps load %+v", quitSave, quitLoad)
	}
	path := filepath.Join(t.TempDir(), "quick.json")
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	a.savePath = path
	a.quitting = true
	if !a.quitSaveLoad(true, false) || !a.quitting || a.toastErr {
		t.Fatalf("save: quitting %v, toast %q", a.quitting, a.toast)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no quick save: %v", err)
	}
	if a.quitSaveLoad(false, false) || !a.quitting {
		t.Error("nothing pressed: answered")
	}
	if !a.quitSaveLoad(false, true) || a.quitting || a.toastErr || a.g == g {
		t.Errorf("load: quitting %v, toast %q, same game %v", a.quitting, a.toast, a.g == g)
	}
	a.savePath, a.quitting = filepath.Join(t.TempDir(), "none.json"), true
	if !a.quitSaveLoad(false, true) || !a.quitting || !a.toastErr {
		t.Errorf("failed load: quitting %v, toast %q", a.quitting, a.toast)
	}
	none := &app{}
	none.savePath, none.quitting = filepath.Join(t.TempDir(), "x.json"), true
	if !none.quitSaveLoad(true, false) || !none.quitting {
		t.Error("save without a game")
	}
	if _, err := os.Stat(none.savePath); err == nil {
		t.Error("saved without a game")
	}
}

// The quit prompt's Screenshot button sits under the two others, inside
// the box above the answers; screenshots go to shots/ beside the quick
// save, writeSnap making the directory and writing a PNG that decodes.
func TestSnap(t *testing.T) {
	if quitShot.Y0 < quitSave.Y1 || quitShot.Y1 > dlgBar.Y0 || quitShot.X0 < quitBox.X0 || quitShot.X1 > quitBox.X1 ||
		float64(quitShot.X1-quitShot.X0) < ui.Width("Screenshot (F12)", bigFace)+2*padX {
		t.Errorf("button %+v: save %+v, box %+v, bar at %d", quitShot, quitSave, quitBox, dlgBar.Y0)
	}
	dir := t.TempDir()
	a := &app{savePath: filepath.Join(dir, "quick.json")}
	if a.shotsDir() != filepath.Join(dir, "shots") {
		t.Errorf("shots in %s", a.shotsDir())
	}
	path := filepath.Join(a.shotsDir(), "x.png")
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	if _, err := writeSnap(path, img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got, err := png.Decode(f); err != nil || got.Bounds() != img.Bounds() {
		t.Errorf("decoded %v, %v", got, err)
	}
}

// -screen: each name opens its screen, "" none; an unknown name or no
// game is refused.
func TestTopScreen(t *testing.T) {
	cl := game.NewCluster(1, game.Average)
	for name, want := range map[string]screen{"planets": &planetsScreen{}, "fleet": &fleetScreen{}, "log": &logScreen{},
		"research": &researchScreen{}, "species": &speciesScreen{}, "designs": &designScreen{},
		"save": &slotsScreen{}, "load": &slotsScreen{}} {
		sc, err := topScreen(cl, name)
		if err != nil || fmt.Sprintf("%T", sc) != fmt.Sprintf("%T", want) {
			t.Errorf("%s: %T %v", name, sc, err)
		}
	}
	if sc, err := topScreen(cl, ""); sc != nil || err != nil {
		t.Errorf("none: %T %v", sc, err)
	}
	if _, err := topScreen(cl, "nope"); err == nil || !strings.Contains(err.Error(), "planets") {
		t.Errorf("unknown: %v", err)
	}
	if _, err := topScreen(nil, "fleet"); err == nil {
		t.Error("no game: accepted")
	}
}
