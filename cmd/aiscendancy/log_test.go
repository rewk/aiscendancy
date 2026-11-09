package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// logApp is seed 1 played days days by the bot, the starmap open: its
// colony ship founds on days 335-340, the log holds 40 entries on day 400.
func logApp(t *testing.T, days int) *app {
	t.Helper()
	g := game.NewGame(2, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	a.bots = []*bot.Bot{bot.New(e, bot.Default())}
	a.push(newStarmapScreen())
	for range days {
		a.endDay()
	}
	return a
}

// rowY is the native y at the middle of row r, from where Text draws it;
// rowX the box's middle.
func rowY(r int) float64 { return logRowsTop() + (float64(r)+0.5)*logLineH }

func rowX() float64 { b := logBox(); return float64(b.X0+b.X1) / 2 }

// The log is stored oldest first; the screen shows it newest first from
// the top, row r the (off+r)-th newest, nothing outside the rows.
func TestLogNewestFirst(t *testing.T) {
	a := logApp(t, 700) // more entries than rows: the scroll moves
	n := len(a.log)
	if !slices.IsSortedFunc(a.log, func(x, y logEntry) int { return x.Day - y.Day }) {
		t.Fatal("log not in day order")
	}
	s := newLogScreen()
	for r := range 3 {
		if i := s.lineAt(a, rowX(), rowY(r)); i != n-1-r {
			t.Errorf("row %d: entry %d, want %d", r, i, n-1-r)
		}
	}
	s.scroll(a, 4)
	if i := s.lineAt(a, rowX(), rowY(0)); i != n-5 {
		t.Errorf("scrolled 4: top entry %d, want %d", i, n-5)
	}
	b := logBox()
	for _, p := range [][2]float64{{rowX(), logRowsTop() - 1}, {rowX(), logRowsTop() + float64(logRows())*logLineH + 1},
		{float64(b.X0 - 1), rowY(0)}, {float64(b.X1 + 1), rowY(0)}} {
		if i := s.lineAt(a, p[0], p[1]); i != -1 {
			t.Errorf("at %v: entry %d, want none", p, i)
		}
	}
	// The box sits on the screen; the title above the rows, the help
	// line under them, inside it.
	if b.X0 < 0 || b.Y0 < 0 || b.X1 > screenW || b.Y1 > screenH || logRowsTop() < float64(b.Y0)+logLineH ||
		logRowsTop()+float64(logRows())*logLineH+smallFace.Size*1.25 > float64(b.Y1) {
		t.Errorf("box %+v, rows from %.0f", b, logRowsTop())
	}
}

// A placed entry opens its system then its planet, pushed over the log;
// a discovery opens the research screen.
func TestLogOpen(t *testing.T) {
	a := logApp(t, 400)
	home := a.cl.Home
	s := newLogScreen()
	built, found, disc := -1, -1, -1
	for i, l := range a.log {
		switch {
		case l.Ev.Kind == game.Built && l.Ev.Item == game.ColonyBase:
			found = i
		case l.Ev.Kind == game.Built && built < 0:
			built = i
		case l.Ev.Kind == game.Discovered:
			disc = i
		}
	}
	if built < 0 || found < 0 || disc < 0 {
		t.Fatalf("built %d, founded %d, discovered %d", built, found, disc)
	}
	if ss := s.open(a, disc); len(ss) != 1 {
		t.Errorf("a discovery opens %v", ss)
	} else if _, ok := ss[0].(*researchScreen); !ok || !linked(a.log[disc].Ev) || a.log[disc].Ev.Star >= 0 {
		t.Errorf("a discovery opens %#v", ss[0])
	}
	check := func(i, star, planet int) {
		t.Helper()
		ss := s.open(a, i)
		if len(ss) != 2 {
			t.Fatalf("%q opens %d screens", a.log[i].Ev.Text, len(ss))
		}
		sys, ok1 := ss[0].(*systemScreen)
		pl, ok2 := ss[1].(*planetScreen)
		if !ok1 || !ok2 || sys.star != star || pl.star != star || pl.idx != planet {
			t.Errorf("%q opens %#v, %#v; want star %d planet %d", a.log[i].Ev.Text, ss[0], ss[1], star, planet)
		}
	}
	check(built, home, 0)
	ev := a.log[found].Ev
	if ev.Star < 0 || ev.Planet < 0 || a.e.Colony(ev.Star, ev.Planet) == nil {
		t.Fatalf("founding %q at (%d, %d): no colony there", ev.Text, ev.Star, ev.Planet)
	}
	check(found, ev.Star, ev.Planet)

	// A click on its line closes the log and opens them over the starmap.
	a.push(s)
	s.off = len(a.log) - 1 - built // built's line is row 0
	if err := s.click(a, rowX(), rowY(0)); err != nil || len(a.stack) != 3 {
		t.Fatalf("click: %v, stack %v", err, a.stack)
	}
	if _, ok := a.stack[1].(*systemScreen); !ok {
		t.Errorf("stack %v: want starmap, system, planet", a.stack)
	}
	if p := a.top().(*planetScreen); p.planet != a.e.Planet(home, 0) {
		t.Error("the planet screen was not entered on the home world")
	}
	// A click in the box off a line does nothing; outside, it closes.
	a.stack = a.stack[:1]
	a.push(s)
	if err := s.click(a, rowX(), float64(logBox().Y0+1)); err != nil || a.top() != screen(s) {
		t.Fatalf("a click on the title: %v, top %T", err, a.top())
	}
	if err := s.click(a, float64(logBox().X0-1), rowY(0)); err != nil || len(a.stack) != 1 {
		t.Fatalf("a click outside: %v, stack %v", err, a.stack)
	}
}

func TestAgeColour(t *testing.T) {
	const today = 340
	for _, c := range []struct {
		day  int
		want float64
	}{{today, 1}, {today - 1, 0.7}, {today - 5, 0.7}, {today - 6, 0.45}, {0, 0.45}} {
		want := dim(textColour, c.want)
		if got := ageColour(c.day, today); got != want {
			t.Errorf("day %d: %v, want %v", c.day, got, want)
		}
	}
}

// The offset stays in [0, max(0, n − visible)].
func TestLogScroll(t *testing.T) {
	a := logApp(t, 700)
	n := len(a.log)
	if n <= logRows() {
		t.Fatalf("only %d entries for %d rows", n, logRows())
	}
	s := newLogScreen()
	for _, c := range []struct{ d, want int }{{-5, 0}, {3, 3}, {1 << 20, n - logRows()}, {-1, n - logRows() - 1}, {-(1 << 20), 0}} {
		s.scroll(a, c.d)
		if s.off != c.want {
			t.Errorf("scroll %d: off %d, want %d", c.d, s.off, c.want)
		}
	}
	a.log = a.log[:logRows()-2]
	s.scroll(a, 10)
	if s.off != 0 {
		t.Errorf("short log: off %d", s.off)
	}
}

// report never pushes the log: on the starmap,
// which covers every event, a day's news (a freeing growth's included)
// leaves the stack as it is; no event reports nothing anywhere.
func TestReport(t *testing.T) {
	a := logApp(t, 1)
	grew := []game.Event{{Kind: game.Grew, Star: a.cl.Home, Planet: 0, Other: -1}}
	built := []game.Event{{Kind: game.Built, Item: game.Factory, Star: a.cl.Home, Planet: 0, Other: -1}}
	a.report(nil)
	a.report(grew)
	a.report(built)
	if len(a.stack) != 1 {
		t.Fatalf("starmap: stack %v", a.stack)
	}
	a.push(newResearchScreen())
	a.report(nil)
	if _, ok := a.top().(*researchScreen); !ok || len(a.stack) != 2 {
		t.Errorf("no news: top %T, stack %d", a.top(), len(a.stack))
	}
}

func TestToggleLog(t *testing.T) {
	a := logApp(t, 1)
	if err := a.toggleLog(); err != nil || len(a.stack) != 2 {
		t.Fatalf("open: %v, stack %v", err, a.stack)
	}
	if _, ok := a.top().(*logScreen); !ok {
		t.Fatalf("top %T", a.top())
	}
	if err := a.toggleLog(); err != nil || len(a.stack) != 1 {
		t.Fatalf("close: %v, stack %v", err, a.stack)
	}
	if _, ok := a.top().(*starmapScreen); !ok {
		t.Errorf("top %T", a.top())
	}
}

// Past logKeep the oldest go: a full log takes a day's events at its end.
func TestLogCap(t *testing.T) {
	a := logApp(t, 1)
	a.log = a.log[:0]
	for k := range logKeep {
		a.log = append(a.log, logEntry{Day: -k, Ev: game.Event{Text: "filler", Star: -1, Planet: -1}})
	}
	var ev []game.Event
	for len(ev) == 0 {
		ev = a.endDay()
	}
	if len(a.log) != logKeep {
		t.Fatalf("%d kept", len(a.log))
	}
	tail := a.log[logKeep-len(ev):]
	for k, x := range ev {
		got := tail[k].Ev
		got.Text = strings.TrimSuffix(got.Text, ", no free pop") // record's note
		if got != x || tail[k].Day != a.e.Day {
			t.Errorf("entry %d: %+v, want day %d %+v", k, tail[k], a.e.Day, x)
		}
	}
	if a.log[0].Day != -len(ev) {
		t.Errorf("oldest kept is filler day %d, want %d", a.log[0].Day, -len(ev))
	}
}

// A report on a screen opened from the log, its event elsewhere: the
// climb drops that screen and the log too, down to the starmap (no log
// comes back by itself); no news changes nothing.
func TestReportClimbsPastTheLog(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	a := testApp(g)
	a.bots = []*bot.Bot{bot.New(e, bot.Default())}
	a.open([]screen{newStarmapScreen()})
	a.push(newLogScreen())
	a.push(newSystemScreen(e.Cluster.Home))
	a.report(nil)
	if len(a.stack) != 3 {
		t.Fatalf("no news reported: stack %d deep", len(a.stack))
	}
	other := (e.Cluster.Home + 1) % len(a.cl.Stars)
	a.report([]game.Event{{Kind: game.Built, Item: game.Factory, Tech: game.NoTech, Star: other, Planet: 0, Other: -1}})
	if _, ok := a.top().(*starmapScreen); len(a.stack) != 1 || !ok {
		t.Fatalf("stack %d deep, top %T: want the starmap alone", len(a.stack), a.top())
	}
}

// The log is a popup over the screen it came on: the starmap's turn
// buttons run the day and leave the starmap on top (no log pops); the
// counter opens it and, over it, its press is a click outside, which
// closes it.
func TestLogTurnButtons(t *testing.T) {
	a := logApp(t, 1)
	day := a.e.Day
	if turnButton(float64(mapArea.X0+1), float64(mapArea.Y0+1)) != noButton || a.pressTurn(noButton) {
		t.Fatal("a press in the area is a turn")
	}
	if !a.pressTurn(turnButton(float64(mapSkip.X0+1), float64(mapSkip.Y0+1))) || a.e.Day <= day || a.e.Day > day+maxSkip {
		t.Fatalf("skip: day %d from %d", a.e.Day, day)
	}
	if len(a.stack) != 1 {
		t.Fatalf("stack %v: the skip's news must leave the starmap alone", a.stack)
	}
	cx, cy := float64(mapTop.X0+1), float64(mapTop.Y0+1)
	if logBox().has(cx, cy) {
		t.Fatal("the counter is under the box")
	}
	if counter := turnButton(cx, cy); counter != logButton || !a.pressTurn(counter) || len(a.stack) != 2 {
		t.Fatalf("counter on the starmap: stack %d", len(a.stack))
	}
	l := top[*logScreen](t, a)
	if l.off != 0 || l.below(a) != a.stack[0] {
		t.Fatalf("the log: off %d, over %T", l.off, l.below(a))
	}
	if err := l.click(a, cx, cy); err != nil || len(a.stack) != 1 {
		t.Fatalf("counter over the log: %v, stack %d", err, len(a.stack))
	}
}
