package main

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// newsLog replaces a's log by n entries per day of days (oldest first),
// a placeless Built each, and sets today.
func newsLog(a *app, today int, days ...[2]int) {
	a.log = nil
	for _, d := range days {
		for range d[1] {
			a.log = append(a.log, logEntry{Day: d[0], Ev: game.Event{Kind: game.Built, Item: game.NoItem, Tech: game.NoTech,
				Star: -1, Planet: -1, Other: -1, Text: "filler"}})
		}
	}
	a.e.Day = today
}

// The news lists the log newest first: the last newsDays days, but
// newsMin entries at least while the log holds them.
func TestNewsEntries(t *testing.T) {
	a, _, _ := scopeApp(t)
	newest := func(n int) []int {
		out := make([]int, n)
		for k := range out {
			out[k] = len(a.log) - 1 - k
		}
		return out
	}
	for _, c := range []struct {
		name string
		days [][2]int
		want int
	}{
		{"few recent: newsMin", [][2]int{{60, 10}, {85, 5}, {95, 8}, {100, 4}}, newsMin},
		{"many recent: all of the last 10 days", [][2]int{{60, 10}, {90, 3}, {91, 30}, {100, 4}}, 34},
		{"short log: all", [][2]int{{10, 4}, {20, 3}}, 7},
		{"empty", nil, 0},
	} {
		newsLog(a, 100, c.days...)
		if got := newsEntries(a); !slices.Equal(got, newest(c.want)) {
			t.Errorf("%s: %v, want the %d newest", c.name, got, c.want)
		}
	}
}

// The offset stays in [0, max(0, entries − rows)]; the rows map to the
// entries from it, nothing outside the lines.
func TestNewsScroll(t *testing.T) {
	a, _, _ := scopeApp(t)
	newsLog(a, 100, [2]int{100, 30})
	r := newsBox
	n := newsView{hover: -1}
	rows := n.rows(r)
	if rows != linesFit(float64(r.Y1-r.Y0), smallFace) || rows < 6 {
		t.Fatalf("%d rows in newsBox", rows)
	}
	for _, c := range []struct{ d, want int }{{-5, 0}, {3, 3}, {1 << 20, 30 - rows}, {-1, 29 - rows}, {-(1 << 20), 0}} {
		n.scroll(a, r, c.d)
		if n.off != c.want {
			t.Errorf("scroll %d: off %d, want %d", c.d, n.off, c.want)
		}
	}
	x := float64(r.X0+r.X1) / 2
	rowY := func(k int) float64 { return float64(r.Y0+padY) + (float64(k)+0.5)*newsLineH }
	n.off = 4
	for k := range rows {
		if got := n.entry(a, r, n.row(r, x, rowY(k))); got != 29-4-k {
			t.Errorf("row %d: entry %d, want %d", k, got, 29-4-k)
		}
	}
	for _, p := range [][2]float64{{x, float64(r.Y0) + 1}, {x, float64(r.Y1) + 1}, {float64(r.X0 - 1), rowY(0)}, {x, rowY(rows)}} {
		if k := n.row(r, p[0], p[1]); k != -1 || n.entry(a, r, k) != -1 {
			t.Errorf("at %v: row %d, want none", p, k)
		}
	}
	// The wheel: a notch over the box, nothing elsewhere.
	n.off = 0
	if !n.wheel(a, r, x, rowY(0), -1) || n.off != newsNotch {
		t.Errorf("wheel down: off %d", n.off)
	}
	if !n.wheel(a, r, x, rowY(0), 1) || n.off != 0 {
		t.Errorf("wheel up: off %d", n.off)
	}
	if n.wheel(a, r, float64(r.X0-1), rowY(0), -1) || n.wheel(a, r, x, rowY(0), 0) || n.off != 0 {
		t.Errorf("wheel outside: off %d", n.off)
	}
	// A shorter log re-clamps (update), the hover follows the cursor.
	n.off = 20
	a.log = a.log[:5]
	if n.update(a, r, x, rowY(2)); n.off != 0 || n.hover != 2 {
		t.Errorf("update: off %d hover %d", n.off, n.hover)
	}
}

// A press in the box is the news' (true) whatever is under it; on a
// linked line it pushes that event's screens (eventScreens); outside it
// is not the news' and pushes nothing.
func TestNewsClick(t *testing.T) {
	a, home, _ := scopeApp(t)
	newsLog(a, 100, [2]int{100, 2})
	a.log = append(a.log,
		logEntry{Day: 100, Ev: ev(game.Discovered, -1, -1)},
		logEntry{Day: 100, Ev: ev(game.Built, home, 0)}) // row 0
	r := newsBox
	n := newsView{hover: -1}
	x := float64(r.X0+r.X1) / 2
	rowY := func(k int) float64 { return float64(r.Y0+padY) + (float64(k)+0.5)*newsLineH }
	if !n.click(a, r, x, rowY(0)) || len(a.stack) != 3 {
		t.Fatalf("a placed line: stack %v", a.stack)
	}
	if v, ok := a.stack[1].(*systemScreen); !ok || v.star != home {
		t.Errorf("stack %v: want the home system then its planet", a.stack)
	}
	if p := top[*planetScreen](t, a); p.star != home || p.idx != 0 {
		t.Errorf("planet screen %d,%d", p.star, p.idx)
	}
	a.stack = a.stack[:1]
	if !n.click(a, r, x, rowY(1)) || len(a.stack) != 2 {
		t.Fatalf("a discovery: stack %v", a.stack)
	}
	top[*researchScreen](t, a)
	a.stack = a.stack[:1]
	for _, p := range [][2]float64{{x, rowY(2)}, {x, rowY(6)}, {x, float64(r.Y0) + 1}} {
		if !n.click(a, r, p[0], p[1]) || len(a.stack) != 1 {
			t.Errorf("at %v (no link): stack %v", p, a.stack)
		}
	}
	if n.click(a, r, float64(r.X0-1), rowY(0)) || n.click(a, r, x, float64(r.Y1)+1) || len(a.stack) != 1 {
		t.Errorf("outside: stack %v", a.stack)
	}
}

// The news' box: on the starmap (and the lists) newsBox holds no button
// and meets neither the stack nor the big area; its border fits the
// screen; a press in it is the news' (input), on a linked line its
// screens pushed, never reaching a button; the wheel over it is the
// news', elsewhere not.
func TestNewsBoxes(t *testing.T) {
	a, home, _ := scopeApp(t)
	s := a.stack[0].(*starmapScreen)
	newsLog(a, 100, [2]int{100, 3})
	for y := newsBox.Y0; y < newsBox.Y1; y += 2 {
		for x := newsBox.X0; x < newsBox.X1; x += 2 {
			if b := s.button(float64(x), float64(y)); b != noButton {
				t.Fatalf("button %d at (%d,%d) in newsBox", b, x, y)
			}
		}
	}
	rim := len(newsRim)
	if newsBox != orig(474, 364, 633, 473) || newsBox.Y0-rim < mapStack[len(mapStack)-1].Y1 || newsBox.X0-rim < mapArea.X1 ||
		newsBox.X1+rim > screenW || newsBox.Y1+rim > screenH {
		t.Errorf("newsBox %+v", newsBox)
	}
	if n := (&newsView{}).rows(newsBox); n != 13 {
		t.Errorf("%d rows in newsBox", n)
	}
	n := &s.news
	at := func(k int) (float64, float64) {
		return float64(newsBox.X0 + padX + 1), float64(newsBox.Y0+padY) + (float64(k)+0.5)*newsLineH
	}
	x, y := at(5) // past the last line
	if _, pressed := n.input(a, x, y, 0, true); !pressed || len(a.stack) != 1 {
		t.Errorf("a press in the news: pressed %v, stack %v", pressed, a.stack)
	}
	if _, pressed := n.input(a, x, y, 0, false); pressed {
		t.Error("no press, yet pressed")
	}
	a.log = append(a.log, logEntry{Day: 100, Ev: ev(game.Built, home, 1)})
	x, y = at(0)
	if wheeled, pressed := n.input(a, x, y, 0, true); wheeled || !pressed {
		t.Errorf("a press on a linked line: wheeled %v pressed %v", wheeled, pressed)
	}
	if p := top[*planetScreen](t, a); p.star != home || p.idx != 1 || len(a.stack) != 3 || n.hover != 0 {
		t.Errorf("a press on a linked line: stack %v, hover %d", a.stack, n.hover)
	}
	a.stack = a.stack[:1]
	if wheeled, _ := n.input(a, x, y, -1, false); !wheeled {
		t.Error("the wheel over the news is not the news'")
	}
	x, y = float64(mapStack[shipButton].X0+5), float64(mapStack[shipButton].Y0+5)
	if wheeled, pressed := n.input(a, x, y, -1, true); wheeled || pressed || len(a.stack) != 1 || n.hover != -1 {
		t.Errorf("outside the news: wheeled %v pressed %v, stack %v, hover %d", wheeled, pressed, a.stack, n.hover)
	}
}

// The system screen's news is newsBox, shown with nothing selected; the
// lists of both tabs, their hint line and the refusal above it end above
// the box and its border, and still show rows.
func TestSystemNewsBox(t *testing.T) {
	a, home, _ := scopeApp(t)
	top := float64(newsBox.Y0 - len(newsRim))
	if listHintY()+smallFace.Size*1.25 > top {
		t.Errorf("hint at %.0f, news border at %.0f", listHintY(), top)
	}
	for _, tab := range []int{planetsTab, shipsTab} {
		n := listRowsN(tab)
		if n < 4 || float64(listRect(tab, n-1).Y1) > listHintY() {
			t.Errorf("tab %d: %d rows, the last ending at %d, hint at %.0f", tab, n, listRect(tab, n-1).Y1, listHintY())
		}
	}
	if !sysInfo.has(float64(newsBox.X0), float64(newsBox.Y0)) || !sysInfo.has(float64(newsBox.X1-1), float64(newsBox.Y1-1)) {
		t.Errorf("newsBox %+v out of the info box %+v", newsBox, sysInfo)
	}
	v := newSystemScreen(home)
	a.push(v)
	if !v.newsOn() {
		t.Error("no news with nothing selected")
	}
	v.psel = a.e.Planet(home, 0)
	if v.newsOn() {
		t.Error("news under the planet's panel")
	}
	v.psel, v.sel = nil, &game.Starship{}
	if v.newsOn() {
		t.Error("news under the ship's panel")
	}
}

// The fleet screen's stacked box 2 is the Designs button: its press pushes
// a blank design screen (building nowhere), the day unchanged; the news
// box and the big area are not it.
func TestFleetDesigns(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	s := newFleetScreen()
	a.push(s)
	r := mapStack[2]
	for _, p := range [][2]float64{{float64(r.X0), float64(r.Y0)}, {float64(r.X1) - 0.5, float64(r.Y1) - 0.5}} {
		if b := fleetButton(p[0], p[1]); b != designsButton {
			t.Errorf("fleetButton(%v) = %d", p, b)
		}
	}
	for _, p := range [][2]float64{{float64(r.X0) - 0.5, float64(r.Y0)}, {float64(r.X0), float64(r.Y1)},
		{float64(newsBox.X0 + 5), float64(newsBox.Y0 + 5)}, {float64(mapArea.X0 + 5), float64(mapArea.Y0 + 5)}} {
		if b := fleetButton(p[0], p[1]); b != noButton {
			t.Errorf("fleetButton(%v) = %d, want none", p, b)
		}
	}
	s.press(a, fleetButton(float64(r.X0+5), float64(r.Y0+5)))
	if !blankDesign(a.top(), nil) || len(a.stack) != 2 || g.Empires[0].Day != 0 {
		t.Errorf("Designs: stack %v, day %d", a.stack, g.Empires[0].Day)
	}
	a.stack = a.stack[:1]
	s.press(a, noButton)
	if len(a.stack) != 1 {
		t.Errorf("no button: stack %v", a.stack)
	}
}

// The species screen's news: a press on a linked line is the news' and
// pushes its screens; the box meets none of the screen's own press
// targets (the big area's rows and buttons, the ability button, the turn
// box), so no second layer could act on it.
func TestSpeciesNews(t *testing.T) {
	a, home, _ := scopeApp(t)
	s := newSpeciesScreen(-1)
	a.push(s)
	newsLog(a, 100, [2]int{100, 3})
	a.log = append(a.log, logEntry{Day: 100, Ev: ev(game.Built, home, 0)})
	_, ar, _, _ := s.ability(a)
	for y := newsBox.Y0; y < newsBox.Y1; y += 2 {
		for x := newsBox.X0; x < newsBox.X1; x += 2 {
			fx, fy := float64(x), float64(y)
			if mapArea.has(fx, fy) || ar.has(fx, fy) || turnButton(fx, fy) != noButton {
				t.Fatalf("(%d,%d) in newsBox is the screen's too", x, y)
			}
		}
	}
	x, y := float64(newsBox.X0+padX+1), float64(newsBox.Y0+padY+1)
	if _, pressed := s.news.input(a, x, y, 0, true); !pressed {
		t.Error("the press is not the news'")
	}
	if p := top[*planetScreen](t, a); p.star != home || p.idx != 0 || len(a.stack) != 4 {
		t.Errorf("stack %v", a.stack)
	}
	if v, ok := a.stack[2].(*systemScreen); !ok || v.star != home {
		t.Errorf("stack %v: want the home system over the species screen", a.stack)
	}
}

// A hovered line too wide slides left: 0 while it fits; else a rest at
// 0, newsPace px a tick, a rest at textW − boxW, never beyond, and the
// cycle again.
func TestNewsSlide(t *testing.T) {
	if got := newsSlide(500, 100, 100); got != 0 {
		t.Errorf("a fitting line slides %v", got)
	}
	const tw, bw = 130.0, 100.0
	over := tw - bw
	for _, c := range []struct {
		t    int
		want float64
	}{
		{0, 0}, {newsPause, 0}, {newsPause + 10, 10 * newsPace}, {newsPause + int(over/newsPace), over},
		{newsPause + int(over/newsPace) + newsPause, over},
		{newsPause + int(over/newsPace) + 1 + newsPause, 0}, // the next cycle
		{-5, 0},
	} {
		if got := newsSlide(c.t, tw, bw); got != c.want {
			t.Errorf("tick %d: %v, want %v", c.t, got, c.want)
		}
	}
	for k := range 1000 {
		if got := newsSlide(k, tw, bw); got < 0 || got > over {
			t.Fatalf("tick %d: %v out of [0, %v]", k, got, over)
		}
	}
}

// An ability's line opens the species screen (its button), unless the
// use has a place: the Govorom's rich world opens its system and planet.
func TestAbilityNewsPlace(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Govorom, game.Orfa}, game.Average)
	g.Empires[0].AbilityUsed = g.Day - game.Govorom.Info().Recharge
	g.Empires[0].Colonies[0].Planet.Type = game.Tycoon // not rich yet: the use takes it
	a := testApp(g)
	reps, err := g.UseAbility(0)
	if err != nil {
		t.Fatal(err)
	}
	ev, c := reps[0].Event, g.Empires[0].Colonies[0]
	sc := eventScreens(a, ev)
	if ev.Star != c.Star || ev.Planet != c.PlanetIdx || len(sc) != 2 {
		t.Fatalf("rich world event %+v: screens %d, want its system and planet", ev, len(sc))
	}
	if v, ok := sc[1].(*planetScreen); !ok || v.star != c.Star {
		t.Errorf("second screen %T, want the planet", sc[1])
	}
	ev.Star, ev.Planet = -1, -1
	if sc := eventScreens(a, ev); len(sc) != 1 {
		t.Errorf("an unplaced ability: %d screens, want the species one", len(sc))
	} else if _, ok := sc[0].(*speciesScreen); !ok {
		t.Errorf("an unplaced ability opens %T", sc[0])
	}
}

// The starmap's hover ring marks the hovered star, else the star of the
// hovered news line (the system its click opens); none over a line
// without a star.
func TestNewsRing(t *testing.T) {
	a, home, other := scopeApp(t)
	s := top[*starmapScreen](t, a)
	a.log = append(a.log,
		logEntry{Day: 100, Ev: ev(game.Discovered, -1, -1)},
		logEntry{Day: 100, Ev: ev(game.Built, home, 0)}) // row 0
	x := float64(newsBox.X0+newsBox.X1) / 2
	rowY := func(k int) float64 { return float64(newsBox.Y0+padY) + (float64(k)+0.5)*newsLineH }
	if r := s.ringed(a); r != -1 {
		t.Errorf("nothing hovered: ring at %d", r)
	}
	s.news.update(a, newsBox, x, rowY(0))
	if r := s.ringed(a); r != home {
		t.Errorf("the Built line: ring at %d, want %d", r, home)
	}
	s.hover = other // a star hovered wins
	if r := s.ringed(a); r != other {
		t.Errorf("a star hovered: ring at %d, want %d", r, other)
	}
	s.hover = -1
	s.nav = &navPick{} // Navigate… picking: the line rings nothing
	if r := s.ringed(a); r != -1 {
		t.Errorf("Navigate… picking: ring at %d", r)
	}
	s.nav = nil
	s.news.update(a, newsBox, x, rowY(1))
	if r := s.ringed(a); r != -1 {
		t.Errorf("the discovery line: ring at %d", r)
	}
}

// An offer's or request's news line rings the other empire's home star
// once the map shows it theirs (known), nowhere before; another kind
// naming them (a refusal) rings nothing.
func TestNewsRingOffer(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	s := top[*starmapScreen](t, a)
	h := g.Empires[1].HomeStar()
	if a.e.Knows(h) {
		t.Fatal("fixture: their home known at the start")
	}
	offer := func(k game.EventKind) game.Event {
		e := ev(k, -1, -1)
		e.Other = 1
		return e
	}
	for _, k := range []game.EventKind{game.PeaceOffered, game.AllianceOffered, game.TradeOffered, game.HelpAsked} {
		if r := newsStar(a, offer(k)); r != -1 {
			t.Errorf("%v, home unknown: ring at %d", k, r)
		}
	}
	a.e.Visited[h] = true
	for _, k := range []game.EventKind{game.PeaceOffered, game.AllianceOffered, game.TradeOffered, game.HelpAsked} {
		if r := newsStar(a, offer(k)); r != h {
			t.Errorf("%v, home known: ring at %d, want %d", k, r, h)
		}
	}
	if r := newsStar(a, offer(game.OfferRefused)); r != -1 {
		t.Errorf("a refusal: ring at %d", r)
	}
	a.log = append(a.log, logEntry{Day: 100, Ev: offer(game.TradeOffered)})
	s.news.update(a, newsBox, float64(newsBox.X0+newsBox.X1)/2, float64(newsBox.Y0+padY)+0.5*newsLineH)
	if r := s.ringed(a); r != h {
		t.Errorf("the offer line hovered: ring at %d, want %d", r, h)
	}
}

// The system screen rings the body its hovered news line names: the
// planet, the star for a line without one; nothing for another system's
// line, nor with the news hidden.
func TestSystemNewsRing(t *testing.T) {
	a, home, other := scopeApp(t)
	v := newSystemScreen(home)
	a.push(v)
	v.Enter(a)
	a.log = append(a.log,
		logEntry{Day: 100, Ev: ev(game.Built, other, 0)},
		logEntry{Day: 100, Ev: ev(game.Arrived, home, -1)},
		logEntry{Day: 100, Ev: ev(game.Built, home, 1)}) // row 0
	x := float64(newsBox.X0+newsBox.X1) / 2
	rowY := func(k int) float64 { return float64(newsBox.Y0+padY) + (float64(k)+0.5)*newsLineH }
	hover := func(k int) *body { v.news.update(a, newsBox, x, rowY(k)); return v.newsBody(a) }
	if b := hover(0); b == nil || b.kind != planetBody || b.idx != 1 {
		t.Errorf("planet 1's line: %+v", b)
	}
	if b := hover(1); b == nil || b.kind != starBody {
		t.Errorf("the system's line: %+v", b)
	}
	if b := hover(2); b != nil {
		t.Errorf("another system's line: %+v", b)
	}
	hover(0)
	v.psel = a.e.Planet(home, 0) // the planet's panel hides the news
	if b := v.newsBody(a); b != nil {
		t.Errorf("news hidden: %+v", b)
	}
}
