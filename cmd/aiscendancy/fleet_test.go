package main

import (
	"fmt"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// fleetApp is seed 1 with two fixture ships at home: Ships[0] idle in
// orbit, Ships[1] ordered to the first lane's far star and run day by
// day until it is in the lane.
func fleetApp(t *testing.T) *app {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	c := e.Cluster
	e.Spawn(fixtureShip, c.Home, 0)
	sh := e.Spawn(fixtureShip, c.Home, 0)
	far := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	if err := e.Order(sh, game.Order{Kind: game.GoToStar, Star: far}); err != nil {
		t.Fatal(err)
	}
	for d := 0; sh.Lane < 0; d++ {
		if d > 50 {
			t.Fatalf("not in a lane after %d days: %+v", d, sh)
		}
		e.EndDay()
	}
	return testApp(g)
}

// fleetMidY is the native y at the middle of row r.
func fleetMidY(r int) float64 { return fleetRowY0(r) + fleetRowH/2 }

// fleetRowY is the middle of a logLineH row r under fleetTop: the
// species screen's rows (species_test borrows it), the old fleet rows'.
func fleetRowY(r int) float64 { return fleetTop() + (float64(r)+0.5)*logLineH }

func TestFleetLines(t *testing.T) {
	a := fleetApp(t)
	e, c := a.e, a.cl
	idle, lane := e.Ships[0], e.Ships[1]
	days, ok := lane.ETA(c, e.Day)
	if !ok || lane.Order.Kind != game.GoToStar {
		t.Fatalf("lane ship: ETA %d %v, order %+v", days, ok, lane.Order)
	}
	for _, w := range []struct {
		sh          *game.Starship
		where, ordr string
	}{
		{idle, "orbit " + e.Planet(c.Home, 0).Name, "idle"},
		{lane, "lane to " + e.StarLabel(c.Other(lane.Lane, lane.Star)),
			"arrival " + etaText(days)},
	} {
		if got := whereText(e, w.sh); got != w.where {
			t.Errorf("%s: where %q, want %q", w.sh.Name, got, w.where)
		}
		if got := orderText(e, w.sh); got != w.ordr {
			t.Errorf("%s: order %q, want %q", w.sh.Name, got, w.ordr)
		}
		for k, str := range shipLines(e, w.sh) {
			if ui.Width(str, fleetFace(k)) > fleetTextW() {
				t.Errorf("%s: %q cut at %.0f px", w.sh.Name, str, fleetTextW())
			}
		}
	}
	// The worst cases the cluster allows: its longest star name, a planet
	// "VIII" of it, two-digit days.
	long := ""
	for _, st := range c.Stars {
		if len(st.Name) > len(long) {
			long = st.Name
		}
	}
	for _, str := range []string{
		"orbit " + long + " VIII", "lane to " + long, "space at " + long,
		"-> " + long + " VIII, " + etaText(99), "being refitted, " + dayCount(99),
	} {
		if ui.Width(str, fleetSubFace) > fleetTextW() {
			t.Errorf("%q cut at %.0f px", str, fleetTextW())
		}
	}
	// The name's bigger line and the two under it stay in the row's band.
	if end := fleetTextY(1, fleetLines-1) + fleetSubFace.Size*1.25; end > fleetRowY0(2)-fleetRowGap/2 {
		t.Errorf("row 1's text ends at %.1f, past its band (%.1f)", end, fleetRowY0(2)-fleetRowGap/2)
	}
	// Left to right inside the area: icon, text, mini gauges, gauges.
	x0 := float64(mapArea.X0 + padX)
	edges := []float64{x0, fleetIconX(), fleetIconX() + fleetIconW, fleetTextX(), fleetTextX() + fleetTextW(),
		fleetMiniX(), fleetMiniX() + fleetMiniW, fleetGaugeX(), fleetGaugeX() + fleetGaugeW, float64(mapArea.X1 - padX)}
	for k := 1; k < len(edges); k++ {
		if edges[k] < edges[k-1] {
			t.Errorf("edge %d at %.1f left of edge %d at %.1f: %v", k, edges[k], k-1, edges[k-1], edges)
		}
	}
	// The five mini gauges fit the three lines.
	if 5*gaugePitch > fleetLines*logLineH || gaugeBarH > gaugePitch {
		t.Errorf("mini gauges: pitch %.1f, bar %d, lines %.0f", gaugePitch, gaugeBarH, fleetLines*logLineH)
	}
}

// The rows: inside the area, apart, the icon and both text lines (the
// gauges' slots too) inside their own row; lineAt maps a row's whole band
// to its ship, the header room and past the ships to none.
func TestFleetRows(t *testing.T) {
	a := fleetApp(t)
	s := newFleetScreen()
	a.push(s)
	if fleetShipH > fleetRowH || fleetLines*logLineH > fleetRowH {
		t.Fatalf("row %.0f px: ship %d, lines %.0f", fleetRowH, fleetShipH, fleetLines*logLineH)
	}
	for r := range fleetRows() {
		y0, y1 := fleetRowY0(r), fleetRowY0(r)+fleetRowH
		if r > 0 && y0 < fleetRowY0(r-1)+fleetRowH {
			t.Errorf("row %d overlaps row %d", r, r-1)
		}
		if y0 < float64(mapArea.Y0) || y1 > float64(mapArea.Y1-padY) {
			t.Errorf("row %d [%.0f, %.0f) outside the area %v", r, y0, y1, mapArea)
		}
		if l0, l1 := fleetLineY(r, 0), fleetLineY(r, fleetLines-1)+logLineH; l0 < y0 || l1 > y1 {
			t.Errorf("row %d: lines [%.0f, %.0f) outside [%.0f, %.0f)", r, l0, l1, y0, y1)
		}
		// The cell inside its row band and the area, the ship's box in it,
		// apart from the text column.
		cr := fleetCell(r)
		if float64(cr.Y0) < y0 || float64(cr.Y1) > y1 || cr.Y1-cr.Y0 < fleetShipH || cr.Y0 < mapArea.Y0 || cr.Y1 > mapArea.Y1 {
			t.Errorf("row %d [%.0f, %.0f): cell %v, ship %d high", r, y0, y1, cr, fleetShipH)
		}
		if float64(cr.X0) < fleetIconX() || float64(cr.X1)+fleetGap > fleetTextX() || cr.X1-cr.X0 < 2*fleetShipW-4 {
			t.Errorf("row %d: cell %v, text at %.0f", r, cr, fleetTextX())
		}
	}
	// One more row would not fit.
	if end := fleetRowY0(fleetRows()); end+fleetRowH <= float64(mapArea.Y1-padY) {
		t.Errorf("%d rows end at %.0f: room for one more", fleetRows(), end)
	}
	for r := range 2 {
		for _, y := range []float64{fleetRowY0(r), fleetLineY(r, 0) + 1, fleetMidY(r), fleetLineY(r, fleetLines-1) + logLineH - 1, fleetRowY0(r) + fleetRowH - 0.5} {
			if i := s.lineAt(a, y); i != r {
				t.Errorf("row %d, y %.1f: ship %d", r, y, i)
			}
		}
	}
	for _, y := range []float64{fleetTop() - 0.5, fleetMidY(2), fleetRowY0(fleetRows()) + 1, float64(mapArea.Y1)} {
		if i := s.lineAt(a, y); i != -1 {
			t.Errorf("y %.1f: ship %d, want none", y, i)
		}
	}
}

// A click opens the ship's system with it selected; the one in a lane
// opens the system it left, unselected (no interaction in a lane); a
// ship elsewhere is not kept. A right click does the same (Update: both
// buttons push open).
func TestFleetOpen(t *testing.T) {
	a := fleetApp(t)
	e := a.e
	s := newFleetScreen()
	a.push(s)
	orbits := 0
	for i, sh := range e.Ships {
		scs := s.open(a, i)
		sys, ok := scs[0].(*systemScreen)
		if !ok || sys.star != sh.Star {
			t.Fatalf("ship %d opens %#v", i, scs)
		}
		for _, sc := range scs {
			a.push(sc)
		}
		if sh.Lane < 0 && sh.Orbit >= 0 { // the planet screen, the ship in its slot
			orbits++
			ps, ok := a.top().(*planetScreen)
			if len(scs) != 2 || !ok || ps.star != sh.Star || ps.idx != sh.Orbit || ps.dlg == nil || ps.dlg.ship != sh || sys.sel != nil {
				t.Errorf("ship %d (%s) in orbit: opens %#v, top %#v", i, whereText(e, sh), scs, a.top())
			}
		} else {
			want := sh
			if sh.Lane >= 0 {
				want = nil
			}
			if len(scs) != 1 || sys.sel != want {
				t.Errorf("ship %d (%s): selected %v after Enter", i, whereText(e, sh), sys.sel)
			}
		}
		a.stack = a.stack[:len(a.stack)-len(scs)]
	}
	if orbits == 0 || orbits == len(e.Ships) {
		t.Fatalf("%d of %d ships in orbit: the fixture walks only one branch", orbits, len(e.Ships))
	}
	if len(a.stack) != 1 || a.top() != screen(s) {
		t.Fatalf("stack %v", a.stack)
	}
	other := e.Cluster.Other(e.Cluster.Stars[e.Cluster.Home].Lanes[0], e.Cluster.Home)
	v := newSystemScreenAt(other, e.Ships[0])
	a.push(v)
	if v.sel != nil {
		t.Error("a ship of another system stays selected")
	}
	if newSystemScreen(other).sel != nil {
		t.Error("newSystemScreen preselects")
	}
}

// The offset stays in [0, max(0, n − visible)]; the count and the cap
// are the rule's.
func TestFleetScroll(t *testing.T) {
	a := fleetApp(t)
	e := a.e
	for len(e.Ships) < fleetRows()+5 {
		e.Spawn(fixtureShip, e.Cluster.Home, 0)
	}
	n := len(e.Ships)
	s := newFleetScreen()
	for _, c := range []struct{ d, want int }{{-5, 0}, {3, 3}, {1 << 20, n - fleetRows()}, {-1, n - fleetRows() - 1}, {-(1 << 20), 0}} {
		s.scroll(a, c.d)
		if s.off != c.want {
			t.Errorf("scroll %d: off %d, want %d", c.d, s.off, c.want)
		}
	}
	s.scroll(a, 4)
	if i := s.lineAt(a, fleetMidY(0)); i != 4 {
		t.Errorf("scrolled 4: top ship %d", i)
	}
	e.Ships = e.Ships[:2]
	s.scroll(a, 0)
	if s.off != 0 {
		t.Errorf("ships gone: off %d", s.off)
	}

	want := fmt.Sprintf("2 ships\ncap %d", game.ShipsPerSystem*e.ControlledSystems())
	if got := fleetCounts(e); got != want || e.ControlledSystems() != 1 {
		t.Errorf("counts %q, want %q (%d systems)", got, want, e.ControlledSystems())
	}
	// A colony in a second system (its planet owned: control is the
	// planets' owners) raises the cap.
	c := e.Cluster
	other := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	e.Planet(other, 0).Owner = e.Index
	e.Colonies = append(e.Colonies, &game.Colony{Planet: e.Planet(other, 0), Star: other, PlanetIdx: 0})
	want = fmt.Sprintf("2 ships\ncap %d", game.ShipsPerSystem*2)
	if got := fleetCounts(e); got != want || e.ControlledSystems() != 2 {
		t.Errorf("two systems: %q, want %q", got, want)
	}
}

// A ship being built is a row after the ships: its class (Spawn names
// it) or its given name, its yard, the days left, what the design will
// carry; a click opens its colony's planet; box 1 counts it.
func TestFleetBuilds(t *testing.T) {
	a := fleetApp(t)
	e := a.e
	col := e.Colonies[0]
	d := game.FixtureColonyShip
	col.Project = &game.Project{Item: game.Ship, Slot: -1, Orbital: true, Design: &d}
	if n := fleetEntries(e); n != len(e.Ships)+1 || build(e, n-1) != col || build(e, 0) != nil {
		t.Fatalf("%d entries, the last %v", n, build(e, n-1))
	}
	s := newFleetScreen()
	a.push(s)
	i := len(e.Ships)
	if got := s.lineAt(a, fleetMidY(i)); got != i {
		t.Errorf("row %d: entry %d", i, got)
	}
	want := [fleetLines]string{d.Name, "yard " + col.Planet.Name, "being built, " + projectDays(col)}
	if got := buildLines(col); got != want || d.Colonizers() == 0 {
		t.Errorf("build lines %q, want %q", got, want)
	}
	col.Project.ShipName = "Ark"
	if got := buildLines(col); got[0] != "Ark" {
		t.Errorf("a named build: %q", got[0])
	}
	scs := s.open(a, i)
	if len(scs) != 2 {
		t.Fatalf("a build opens %#v", scs)
	}
	sys, ok := scs[0].(*systemScreen)
	ps, ok2 := scs[1].(*planetScreen)
	if !ok || !ok2 || sys.star != col.Star || ps.star != col.Star || ps.idx != col.PlanetIdx {
		t.Errorf("a build opens %#v", scs)
	}
	wantC := fmt.Sprintf("%d ships, 1 building\ncap %d", len(e.Ships), game.ShipsPerSystem*e.ControlledSystems())
	if got := fleetCounts(e); got != wantC {
		t.Errorf("counts %q, want %q", got, wantC)
	}
}

// A ship under refit says so instead of its order.
func TestFleetRefitOrder(t *testing.T) {
	a, _, col, worn, _ := docksApp(t)
	e := a.e
	d, err := e.RefitTarget(worn)
	if err == nil {
		err = e.Refit(col, worn, d)
	}
	if err != nil {
		t.Fatal(err)
	}
	if got, want := orderText(e, worn), "being refitted, "+projectDays(col); got != want || worn.Order.Kind != game.NoOrder {
		t.Errorf("order %q, want %q", got, want)
	}
}

// Each entry's cell scene follows the model: a ship in a lane (blue and
// red: a second ship sent down a red lane) its sphere by the lane's
// colour, in orbit its planet's globe, in space its star's sun, a build
// the Shipyard; every kind walked.
func TestFleetScenes(t *testing.T) {
	a := fleetApp(t)
	g, e, c := a.g, a.e, a.cl
	if err := game.SpawnFixtures(g, 0, 1, 0); err != nil { // a Gunboat in the home's space
		t.Fatal(err)
	}
	red := -1
	for l, sl := range c.Lanes {
		if sl.Red && len(c.Stars[sl.A].System.Planets) > 0 {
			red = l
			break
		}
	}
	if red < 0 {
		t.Fatal("seed 1: no red lane from a star with planets")
	}
	from := c.Lanes[red].A
	sh := e.Spawn(fixtureShip, from, 0)
	if err := e.Order(sh, game.Order{Kind: game.GoToStar, Star: c.Other(red, from)}); err != nil {
		t.Fatal(err)
	}
	for d := 0; sh.Lane < 0; d++ {
		if d > 80 {
			t.Fatalf("not in the red lane after %d days: %+v", d, sh)
		}
		e.EndDay()
	}
	col := e.Colonies[0]
	d := game.FixtureColonyShip
	col.Project = &game.Project{Item: game.Ship, Slot: -1, Orbital: true, Design: &d}
	seen := map[string]bool{}
	for i := range fleetEntries(e) {
		got := sceneOf(e, i)
		var want fleetScene
		var key string
		if build(e, i) != nil {
			want, key = fleetScene{sceneBuild, "planitem", int(game.Shipyard)}, "build"
		} else {
			s := e.Ships[i]
			switch {
			case s.Lane >= 0 && c.Lanes[s.Lane].Red:
				want, key = fleetScene{sceneLane, redExitBank, 0}, "red"
			case s.Lane >= 0:
				want, key = fleetScene{sceneLane, blueExitBank, 0}, "blue"
			case s.Orbit >= 0:
				p := e.Planet(s.Star, s.Orbit)
				want, key = fleetScene{sceneOrbit, "planets", int(p.Type)*game.MaxSize + p.Size - 1}, "orbit"
			case s.InSpace():
				want, key = fleetScene{sceneSpace, "suns", int(c.Stars[s.Star].System.Star)}, "space"
			}
		}
		seen[key] = true
		if got != want {
			t.Errorf("entry %d (%s): scene %+v, want %+v", i, key, got, want)
		}
	}
	for _, k := range []string{"build", "red", "blue", "orbit", "space"} {
		if !seen[k] {
			t.Errorf("no %s entry walked: %v", k, seen)
		}
	}
}
