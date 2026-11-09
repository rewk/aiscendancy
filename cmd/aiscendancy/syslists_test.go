package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// listShips is the ships' tab, in row order.
func listShips(v *systemScreen) []*game.Starship {
	var out []*game.Starship
	for _, i := range v.listRows()[shipsTab] {
		out = append(out, v.bodies[i].ship)
	}
	return out
}

// rowPoint is a point inside the row of tab t showing body i (an index
// into v.bodies), scrolled as v is; false when not shown. It shows tab t.
func rowPoint(v *systemScreen, t, i int) (float64, float64, bool) {
	v.listTab = t
	k := slices.Index(v.listRows()[t], i)
	r := k - v.listOff[t]
	if k < 0 || r < 0 || r >= listRowsN(t) {
		return 0, 0, false
	}
	rr := listRect(t, r)
	return float64(rr.X0 + 5), float64(rr.Y0 + 5), true
}

// bodyOf is the index in v.bodies of the body of ship sh, −1 none.
func bodyOf(v *systemScreen, sh *game.Starship) int {
	return slices.IndexFunc(v.bodies, func(b body) bool { return b.kind == shipBody && b.ship == sh })
}

// The lists are the view's bodies: no planet and no foreign ship at a
// star the player does not know; there, the planets in system order, the
// player's ships out of the lanes first, then the Sightings; a ship in a
// lane out is not listed, though the view draws the player's.
func TestSystemLists(t *testing.T) {
	g, e, o, star := fogGame(t)
	theirs := o.Spawn(foeShip, star, 0)
	away := o.Spawn(fixtureShip, star, 0)
	away.Lane = g.Cluster.Stars[star].Lanes[0]
	a := testApp(g)
	v := newSystemScreen(star)
	v.Enter(a)
	if rows := v.listRows(); len(rows[0])+len(rows[1]) != 0 {
		t.Errorf("unknown star: rows %v, want none", rows)
	}

	e.Visit(star)
	mine := e.Spawn(fixtureShip, star, 0)
	gone := e.Spawn(fixtureShip, star, 0)
	gone.Lane = g.Cluster.Stars[star].Lanes[0]
	v.rebuild()
	if bodyOf(v, gone) >= 0 {
		t.Fatal("the player's ship in a lane out is still drawn")
	}
	rows := v.listRows()
	sys := g.Cluster.Stars[star].System
	if len(rows[0]) != len(sys.Planets) {
		t.Fatalf("%d planet rows, want %d", len(rows[0]), len(sys.Planets))
	}
	for k, i := range rows[0] {
		if b := v.bodies[i]; b.kind != planetBody || b.planet != sys.Planets[k] {
			t.Errorf("planet row %d: %+v, want planet %d", k, b, k)
		}
	}
	if got := listShips(v); !slices.Equal(got, []*game.Starship{mine, theirs}) {
		t.Errorf("ship rows %v, want [%s %s]", got, mine.Name, theirs.Name)
	}
	// Both in orbit of planet 0: the view hides them, the rows say where;
	// the player's adds its order.
	orbit := "in orbit of " + sys.Planets[0].Name
	b := v.bodies[rows[1][1]]
	if l := v.rowText(&b); b.seen == nil || !slices.Equal(l, []string{b.seen.Hull.String() + " hull", o.Species.String(), orbit}) {
		t.Errorf("foreign row %q (sighting %v)", l, b.seen)
	}
	b = v.bodies[rows[1][0]]
	if l := v.rowText(&b); !slices.Equal(l, []string{mine.Name, mine.Design.Name, orbit, "idle"}) {
		t.Errorf("own row %q", l)
	}
	mine.Orbit = -1 // in space: no orbit line
	mine.Order = game.Order{Kind: game.GoToStar, Star: g.Empires[0].Colonies[0].Star}
	v.rebuild()
	b = v.bodies[v.listRows()[1][0]]
	if l := v.rowText(&b); !slices.Equal(l, []string{mine.Name, mine.Design.Name, orderText(v.e, mine)}) || !strings.HasPrefix(l[2], "-> ") {
		t.Errorf("own row in space %q", l)
	}
	for _, i := range rows[0] {
		b := v.bodies[i]
		l := v.rowText(&b)
		if o := b.planet.Owner; (o >= 0) != (l[len(l)-1] == g.Empires[max(o, 0)].Species.String()) {
			t.Errorf("planet row %q, owner %d", l, o)
		}
	}

	// The player first, whatever the Game order: seen from empire 1.
	o.Visit(star)
	a.player, a.e = 1, o
	w := newSystemScreen(star)
	w.Enter(a)
	if got := listShips(w); !slices.Equal(got, []*game.Starship{theirs, mine}) {
		t.Errorf("empire 1's ship rows %v, want [%s %s]", got, theirs.Name, mine.Name)
	}
}

// Hovering a row hovers its very body (the view rings it) and keeps the
// lists; a press on a ship row selects the ship as a click in the view,
// a ship in orbit's (hidden from the view) opens its planet's screen with
// the ship selected there; on a planet row selects the planet; a press
// on a tab shows it; nothing off the rows and tabs.
func TestListClicks(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	home := e.Colonies[0].Star
	ships := []*game.Starship{e.Spawn(fixtureShip, home, 0), e.Spawn(fixtureShip, home, 0)}
	ships[1].Orbit, ships[1].Pos = -1, game.Vec3{X: 0.3, Z: -0.4} // in space
	a := testApp(g)
	v := newSystemScreen(home)
	a.push(v)

	if v.listTab != planetsTab {
		t.Errorf("tab %d shown first, want the planets", v.listTab)
	}
	v.press(a, sysButton(float64(sysFilters[shipsTab].X0+3), float64(sysFilters[shipsTab].Y0+3)))
	if v.listTab != shipsTab {
		t.Fatalf("a press on the ships' filter shows tab %d", v.listTab)
	}
	x, y, ok := rowPoint(v, shipsTab, bodyOf(v, ships[1]))
	if !ok {
		t.Fatal("no row for the second ship")
	}
	v.hover = nil
	v.listHoverAt(x, y)
	if v.hover != &v.bodies[bodyOf(v, ships[1])] || !v.listHover || v.box() != boxLists {
		t.Errorf("row hover: %v (listHover %v), box %d", v.hover, v.listHover, v.box())
	}
	below := float64(listRect(shipsTab, len(ships)).Y0 + 5) // under the last row
	v.hover = nil
	v.listHoverAt(x, below)
	if v.hover != nil || v.listHover {
		t.Errorf("under the rows hovers %v", v.hover)
	}
	v.listClick(a, x, below)
	if v.sel != nil || len(a.stack) != 1 || v.listTab != shipsTab {
		t.Errorf("press off the rows: sel %v, %d screens, tab %d", v.sel, len(a.stack), v.listTab)
	}
	v.listClick(a, x, y)
	if v.sel != ships[1] || v.box() != boxPanel || v.listsOn() {
		t.Fatalf("ship row press: sel %v, box %d", v.sel, v.box())
	}
	v.sel = nil
	if x, y, ok = rowPoint(v, shipsTab, bodyOf(v, ships[0])); !ok {
		t.Fatal("no row for the ship in orbit")
	}
	v.listClick(a, x, y)
	if ps, ok := a.top().(*planetScreen); !ok || ps.star != home || ps.idx != 0 || ps.dlg == nil || ps.dlg.ship != ships[0] || v.sel != nil {
		t.Fatalf("orbit row press: top %#v, sel %v", a.top(), v.sel)
	}
	a.pop()

	v.sel = nil
	p := slices.IndexFunc(v.bodies, func(b body) bool { return b.kind == planetBody && b.idx == 2 })
	v.press(a, sysButton(float64(sysFilters[planetsTab].X1-3), float64(sysFilters[planetsTab].Y1-3)))
	if v.listTab != planetsTab {
		t.Fatalf("a press on the planets' filter shows tab %d", v.listTab)
	}
	if x, y, ok = rowPoint(v, planetsTab, p); !ok {
		t.Fatal("no row for planet 2")
	}
	// A planet row selects it (its panel takes the lists' place); its
	// globe opens it.
	v.listClick(a, x, y)
	if v.psel != v.sys.Planets[2] || v.box() != boxPlanet || v.listsOn() || len(a.stack) != 1 {
		t.Fatalf("planet row press: psel %v, box %d, %d screens", v.psel, v.box(), len(a.stack))
	}
	if err := v.planetClick(a, planetPicCX, planetPicCY); err != nil {
		t.Fatal(err)
	}
	if ps := top[*planetScreen](t, a); ps.star != home || ps.idx != 2 {
		t.Errorf("planet globe press opened star %d planet %d, want %d 2", ps.star, ps.idx, home)
	}
}

// A row hovers its body with the lists shown; a view hover leaves the
// lists, a ship with no text, any other body on the hover line.
func TestListsHover(t *testing.T) {
	_, v, ships := shipScreen(t)
	x, y, _ := rowPoint(v, shipsTab, bodyOf(v, ships[0]))
	if v.listHoverAt(x, y); v.box() != boxLists || v.hover == nil || v.hover.ship != ships[0] {
		t.Errorf("row hover: box %d, hover %v", v.box(), v.hover)
	}
	v.hover, v.listHover = &v.bodies[0], false // the star, hovered in the view: the lists stay, the line says it
	if v.box() != boxLists || v.hoverLine(v.e) != v.e.StarLabel(v.star) {
		t.Errorf("star hover: box %d, line %q", v.box(), v.hoverLine(v.e))
	}
	v.hover = &v.bodies[bodyOf(v, ships[0])]
	if v.box() != boxLists || v.hoverLine(v.e) != "" {
		t.Errorf("ship hover: box %d, line %q", v.box(), v.hoverLine(v.e))
	}
}

// The wheel over the box scrolls the tab shown, clamped, and leaves the
// camera as it was before steer zoomed it; over the view it is the
// camera's. Rows follow the scroll; a tab that shrinks clamps back.
func TestListScroll(t *testing.T) {
	e, v, ships := shipScreen(t)
	for range 2 * listRowsN(shipsTab) {
		ships = append(ships, e.Spawn(fixtureShip, v.star, 1))
	}
	v.rebuild()
	n, rows := len(v.listRows()[shipsTab]), listRowsN(shipsTab)
	sx, sy := float64(panelX0+20), float64(listRowsTop+5)
	focal := v.cam.Focal
	v.listTab = planetsTab
	v.cam.Focal = focal * 1.1 // steer's zoom this tick
	if !v.wheel(sx, sy, -1, focal) || v.cam.Focal != focal || v.listOff != [2]int{0, 0} {
		t.Errorf("wheel down over the planets (all shown): focal %v (want %v), offsets %v", v.cam.Focal, focal, v.listOff)
	}
	v.listTab = shipsTab
	if !v.wheel(sx, sy, -1, focal) || v.listOff != [2]int{0, 1} {
		t.Errorf("wheel down over the ships: offsets %v", v.listOff)
	}
	if b := v.listAt(sx, sy); b == nil || b.ship != listShips(v)[1] {
		t.Errorf("top row after a notch: %v, want %s", b, listShips(v)[1].Name)
	}
	for range n + 5 {
		v.wheel(sx, sy, -1, focal)
	}
	if v.listOff[shipsTab] != n-rows {
		t.Errorf("scrolled to %d, want the last page %d", v.listOff[shipsTab], n-rows)
	}
	v.cam.Focal = focal * 1.1
	if v.wheel(float64(sysArea.X0+50), float64(sysArea.Y0+50), -1, focal) || v.cam.Focal == focal {
		t.Error("the wheel over the view was taken from the camera")
	}
	v.sel = ships[0] // its panel hides the lists
	if v.wheel(sx, sy, 1, focal) {
		t.Error("the wheel scrolled the hidden lists")
	}
	v.sel = nil
	for _, sh := range ships[2:] {
		sh.Lane = e.Cluster.Stars[v.star].Lanes[0]
	}
	v.rebuild()
	if v.listOff[shipsTab] != 0 {
		t.Errorf("after the ships left: offset %d, want 0", v.listOff[shipsTab])
	}
}

// A filter's press shows its list whatever the box held: the selected
// ship's panel.
func TestFilterShowsLists(t *testing.T) {
	_, v, ships := shipScreen(t)
	v.sel = ships[0]
	v.press(nil, shipsFilter) // a filter's press needs no app
	if v.box() != boxLists || v.listTab != shipsTab || v.sel != nil {
		t.Errorf("after the ships' filter: box %d, tab %d, sel %v", v.box(), v.listTab, v.sel)
	}
}

// A full system's five planets fit the planets' tab without a scroll.
func TestListPlanetsFit(t *testing.T) {
	if n := listRowsN(planetsTab); n < 5 {
		t.Errorf("the planets' tab shows %d rows, want 5", n)
	}
}

// During the replay the ships' tab is its cast as drawn: the Gunboat
// destroyed in round 0 stays listed until its blast's first tick.
func TestListsReplay(t *testing.T) {
	a, v, _ := combatApp(t)
	gun := a.e.Ships[0]
	a.endDay()
	rp := v.replay
	if rp == nil {
		t.Fatal("no replay")
	}
	r0 := &rp.rounds[0]
	v.tick = rp.t0 + r0.start + r0.blastStart() - 1
	v.rebuild()
	if !slices.Contains(listShips(v), gun) {
		t.Errorf("a tick before its blast the Gunboat is not listed: %v", listShips(v))
	}
	v.tick++
	v.rebuild()
	if slices.Contains(listShips(v), gun) {
		t.Error("the Gunboat is still listed at its blast")
	}
}

// A planet with Xeno ruins ends its row and its info box with "Xeno
// ruins" at a star the player knows, not at one it does not; dug (no
// ruins any more), it says nothing.
func TestRuinsRow(t *testing.T) {
	g, e, _, star := fogGame(t)
	sys := g.Cluster.Stars[star].System
	with := sys.Planets[0]
	at, ok := with.RuinsAt()
	if !ok { // lay some by hand: the fixture star may have none
		at = with.Cells()[0]
		with.Surface[at].Ruins = true
	}
	a := testApp(g)
	v := newSystemScreen(star)
	v.Enter(a)
	check := func(p *game.Planet, want bool) {
		t.Helper()
		b := body{kind: planetBody, planet: p}
		l := v.rowText(&b)
		if got := l[len(l)-1] == ruinsLine; got != want {
			t.Errorf("%s (knows %v): row %q, want ruins %v", p.Name, e.Knows(star), l, want)
		}
		if got := strings.HasSuffix(v.describe(e, &b), "\n"+ruinsLine); got != want {
			t.Errorf("%s (knows %v): info %q, want ruins %v", p.Name, e.Knows(star), v.describe(e, &b), want)
		}
	}
	check(with, false)
	e.Visit(star)
	check(with, true)
	with.Surface[at].Ruins = false
	check(with, false)
}

// The project column (rowProject): the player's colony row carries its
// project's tile (planitem by item, the hull of a ship) and the days
// left; idle, "No Project" in red with free people, "No Pop." in yellow
// without, no tile; another planet none. The cell sits at the row's
// right, inside it, the foot line under it.
func TestRowProject(t *testing.T) {
	e, v, _ := shipScreen(t)
	home := e.Colonies[0]
	pb := v.planetBody(home.PlanetIdx)
	other := v.planetBody((home.PlanetIdx + 1) % len(v.sys.Planets))
	if _, _, _, _, ok := v.rowProject(other); ok {
		t.Error("a project column on an unowned planet")
	}
	if home.Project == nil {
		t.Skip("the home starts with no project")
	}
	bank, frame, foot, c, ok := v.rowProject(pb)
	wantFoot := "no industry"
	if d, has := home.DaysLeft(); has {
		wantFoot = dayCount(max(1, d))
	}
	if !ok || bank != "planitem" || frame != int(home.Project.Item) || foot != wantFoot || c != textColour {
		t.Errorf("busy: %q %d %q %v %v, want planitem %d %q", bank, frame, foot, c, ok, home.Project.Item, wantFoot)
	}
	pj := home.Project
	home.Project = &game.Project{Design: &game.Design{Hull: game.Small}}
	if bank, frame, _, _, _ := v.rowProject(pb); bank != shipBank(e.Species) || frame != int(game.Small) {
		t.Errorf("ship: %q %d", bank, frame)
	}
	home.Project = nil
	if bank, _, foot, c, ok := v.rowProject(pb); !ok || bank != "" || foot != "No Project" || c != errColour {
		t.Errorf("idle: %q %q %v %v", bank, foot, c, ok)
	}
	pop := home.Pop
	_, free, _ := colonyPop(home)
	home.Pop -= free
	if _, _, foot, c, _ := v.rowProject(pb); foot != "No Pop." || c != yellow {
		t.Errorf("idle, no free people: %q %v", foot, c)
	}
	home.Pop, home.Project = pop, pj

	rr := listRect(planetsTab, 0)
	ic := projIconRect(rr)
	if ic.X1 != rr.X1-4 || ic.X0 < rr.X0+listIconW(planetsTab) || ic.Y0 < rr.Y0 || float64(ic.Y1) > projFootY(rr) || projFootY(rr)+smallFace.Size*1.25 > float64(rr.Y1) {
		t.Errorf("cell %v, foot %v in row %v", ic, projFootY(rr), rr)
	}
}

// fitSprite centres the sprite's drawn box, not its origin: a hull whose
// art sits off its canvas middle (the Orfa small hull, measured on its
// bank) lands with its box's centre on the cell's, shrunk only when the
// box exceeds the cell. A planet row's flag sits in the globe's cell,
// its lines start past the cell as a ship's.
func TestFitPlaceCentresTheBox(t *testing.T) {
	a, _, _ := helpApp(t)
	bank, err := a.a.Bank(shipBank(game.Orfa))
	if err != nil {
		t.Fatal(err)
	}
	f := bank.Frames[int(game.Small)]
	sw, sh := float64(f.Box.Dx()), float64(f.Box.Dy())
	offX, offY := float64(f.Box.Min.X-f.Center.X), float64(f.Box.Min.Y-f.Center.Y)
	if offX+sw/2 == 0 && offY+sh/2 == 0 {
		t.Fatal("the Orfa small hull's box is centred on its origin: the case is not exercised")
	}
	for _, c := range []struct{ w, h float64 }{{200, 200}, {sw / 2, 200}} {
		k, x, y := fitPlace(sw, sh, offX, offY, 100, 100, c.w, c.h)
		if cx, cy := x+k*(offX+sw/2), y+k*(offY+sh/2); cx != 100 || cy != 100 {
			t.Errorf("cell %vx%v: box centre (%v,%v), want (100,100)", c.w, c.h, cx, cy)
		}
		if want := min(1, c.w/sw, c.h/sh); k != want {
			t.Errorf("cell %vx%v: scale %v, want %v", c.w, c.h, k, want)
		}
	}
	rr := listRect(planetsTab, 0)
	for _, tab := range []int{planetsTab, shipsTab} {
		if rr := listRect(tab, 0); rowTextX(tab, rr) != float64(rr.X0+listIconW(tab)+4) {
			t.Errorf("tab %d: lines at %v", tab, rowTextX(tab, rr))
		}
	}
	if room := float64(projIconRect(rr).X0-4) - rowTextX(planetsTab, rr); room < ui.Width("Snovemdomas Prime", smallFace) {
		t.Errorf("a planet row's %v px of lines cut the longest name", room)
	}
	fx, fy := rowFlagAt(rr)
	if fx < float64(rr.X0) || fy < float64(rr.Y0) || fx+flagW > float64(rr.X0+listIconW(planetsTab)) || fy+flagH > float64(rr.Y1) {
		t.Errorf("the flag at (%v,%v) leaves the globe cell of row %v", fx, fy, rr)
	}
}
