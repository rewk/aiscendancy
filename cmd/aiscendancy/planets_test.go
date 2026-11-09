package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// planetsApp is logApp's seed 1 at day 340: the home and the colonies
// the bot's colony ship founds on days 335-340.
func planetsApp(t *testing.T) *app {
	t.Helper()
	a := logApp(t, 340)
	if len(a.e.Colonies) < 2 {
		t.Fatalf("%d colonies at day 340: the fixture needs several", len(a.e.Colonies))
	}
	return a
}

// planetsRowY is the native y at the middle of row r.
func planetsRowY(r int) float64 { return float64(rowY0(r)) + planetRowH/2 }

// Row r shows Colonies[r]; nothing past the last colony, above row 0 or
// below the rows; the rows and the blocks fit the area.
func TestPlanetsRows(t *testing.T) {
	a := planetsApp(t)
	s := newPlanetsScreen()
	a.push(s)
	n := len(a.e.Colonies)
	for r := range n {
		if i := s.lineAt(a, planetsRowY(r)); i != r {
			t.Errorf("row %d: colony %d", r, i)
		}
	}
	for _, y := range []float64{float64(mapArea.Y0), float64(planetsTop()) - 0.5, planetsRowY(n),
		float64(rowY0(planetsRows())) + 1, float64(mapArea.Y1)} {
		if i := s.lineAt(a, y); i != -1 {
			t.Errorf("y %.1f: colony %d, want none", y, i)
		}
	}
	bottom := mapArea.Y1 - padY
	if end := rowY0(planetsRows()); end > bottom || end+planetRowH <= bottom {
		t.Errorf("%d rows end at %d, area bottom %d", planetsRows(), end, bottom)
	}
	// Left to right: the orbit miniature, globe, text, three pictures,
	// the pips, inside the margins.
	edges := []int{planetsX0(), planetsX0() + miniW, globeX(), globeX() + rowGlobe, planetTextX(), planetTextX() + planetTextW()}
	for k := range 3 {
		edges = append(edges, outPicX(k), outPicX(k)+outPicW)
	}
	edges = append(edges, popListX(), popListX()+popListCols*popListPitch, mapArea.X1-padX)
	for k := 1; k < len(edges); k++ {
		if edges[k] < edges[k-1] {
			t.Errorf("row blocks overlap: edge %d at %d < %d", k, edges[k], edges[k-1])
		}
	}
	if planetTextW() < 280 { // "(Enormous Primordial Planet)" in bigFace is 269 px
		t.Errorf("text block %d px", planetTextW())
	}
	if popListRows*popListPitch+int(smallFace.Size*1.25) > planetRowH {
		t.Error("the POP line falls out of the row")
	}
	// The miniature: slot k in column k%2, row k/2, five rows in the row's
	// height, the cells apart, the last one above the next row.
	for k := range game.OrbitSlots {
		r := miniRect(1, k)
		if r.X0 != planetsX0()+(k%2)*miniPitch || r.Y0 != rowY0(1)+(k/2)*miniPitch || r.X1 > globeX() || r.Y1 > rowY0(2) {
			t.Errorf("slot %d cell %+v", k, r)
		}
	}
	if miniRect(1, 9).Y1 > rowY0(1)+outPicH {
		t.Errorf("the last cell %+v ends under the globe's bottom %d", miniRect(1, 9), rowY0(1)+outPicH)
	}
}

// The miniature's tiles: a structure's planitem frame, a ship in a free
// slot its hull (the planet screen's mapping), nothing elsewhere.
func TestPlanetsOrbitMini(t *testing.T) {
	a := planetsApp(t)
	c := a.e.Colonies[0]
	p := c.Planet
	for k := range p.Orbit {
		p.Orbit[k] = game.NoItem
	}
	p.Orbit[3] = game.Shipyard
	sh := a.e.Spawn(warship, c.Star, c.PlanetIdx)
	m := orbitMini(a, c)
	want := [game.OrbitSlots]miniTile{0: {shipBank(a.e.Species), int(sh.Design.Hull), true}, 3: {"planitem", int(game.Shipyard), true}}
	if m != want {
		t.Errorf("mini %+v, want %+v", m, want)
	}
}

// A click on a row opens the colony's system, then its planet screen
// above it; Esc (pop) comes back to the list through the system.
func TestPlanetsOpen(t *testing.T) {
	a := planetsApp(t)
	s := newPlanetsScreen()
	a.push(s)
	for i, c := range a.e.Colonies {
		scs := s.open(a, i)
		if len(scs) != 2 {
			t.Fatalf("colony %d opens %#v", i, scs)
		}
		sys, ok1 := scs[0].(*systemScreen)
		ps, ok2 := scs[1].(*planetScreen)
		if !ok1 || !ok2 || sys.star != c.Star || ps.star != c.Star || ps.idx != c.PlanetIdx {
			t.Fatalf("colony %d (%s) opens %#v", i, c.Planet.Name, scs)
		}
		for _, sc := range scs {
			a.push(sc)
		}
		if ps.planet != c.Planet || ps.col != c {
			t.Errorf("colony %d: planet screen on %v, colony %v", i, ps.planet, ps.col)
		}
		a.pop()
		if a.top() != screen(sys) {
			t.Errorf("colony %d: Esc gives %T", i, a.top())
		}
		a.pop()
		if a.top() != screen(s) {
			t.Fatalf("colony %d: back to %T", i, a.top())
		}
	}
}

// The text lines: name, description, star, project and its foot, from
// the model; an idle colony says No Project with no foot.
func TestPlanetsLines(t *testing.T) {
	a := planetsApp(t)
	e := a.e
	busy := 0
	for _, c := range e.Colonies {
		name, desc, star, proj, foot := colonyLines(e, a.cl, c)
		wantDesc := "(" + sizeWords[c.Planet.Size-game.MinSize] + " " + c.Planet.Type.String() + " Planet)"
		if name != c.Planet.Name || desc != wantDesc || star != e.StarLabel(c.Star) {
			t.Errorf("%s: %q %q %q", c.Planet.Name, name, desc, star)
		}
		pj := c.Project
		if pj == nil {
			continue
		}
		busy++
		wantFoot := "no industry"
		if d, ok := c.DaysLeft(); ok {
			wantFoot = dayCount(max(1, d)) + " until completion"
		}
		if proj != "Building "+projectName(pj) || foot != wantFoot {
			t.Errorf("%s: %q / %q, want Building %s / %q", c.Planet.Name, proj, foot, projectName(pj), wantFoot)
		}
		// Idle: "No Project" in red with free people, "No Project/No Pop."
		// in yellow without; busy: textColour.
		if projColour(c) != textColour {
			t.Errorf("%s busy: colour %v", c.Planet.Name, projColour(c))
		}
		c.Project = nil
		pop := c.Pop
		c.Pop = c.Busy() + 1
		if _, _, _, proj, foot := colonyLines(e, a.cl, c); proj != "No Project" || foot != "" || projColour(c) != errColour {
			t.Errorf("%s idle with a free one: %q / %q, colour %v", c.Planet.Name, proj, foot, projColour(c))
		}
		c.Pop = c.Busy()
		if _, _, _, proj, foot := colonyLines(e, a.cl, c); proj != "No Project/No Pop." || foot != "" || projColour(c) != yellow {
			t.Errorf("%s idle with none free: %q / %q, colour %v", c.Planet.Name, proj, foot, projColour(c))
		}
		c.Pop, c.Project = pop, pj
	}
	if busy == 0 {
		t.Fatal("no colony has a project: the Building branch is not walked")
	}
}

// The game's own size words (static.txt 41-45) name the sizes.
func TestSizeWords(t *testing.T) {
	g, err := cob.OpenGame(cob.DefaultDir())
	if err != nil {
		t.Skip(err)
	}
	b, err := g.Read("static.txt")
	if err != nil {
		t.Fatal(err)
	}
	var ls []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.Trim(l, "\r\x1a "); strings.HasPrefix(l, "\"") {
			ls = append(ls, strings.Trim(l, "\""))
		}
	}
	at := -1
	for i := range ls {
		if ls[i] == "Tiny" {
			at = i
			break
		}
	}
	if at < 0 || at+len(sizeWords) > len(ls) {
		t.Fatal("static.txt: no size words")
	}
	for k, w := range sizeWords {
		if ls[at+k] != w {
			t.Errorf("size %d: %q, static.txt %q", k+game.MinSize, w, ls[at+k])
		}
	}
	if len(sizeWords) != game.MaxSize-game.MinSize+1 {
		t.Errorf("%d size words for sizes %d-%d", len(sizeWords), game.MinSize, game.MaxSize)
	}
}

// The offset stays in [0, max(0, n − visible)]; the counts follow the
// colonies and the systems, singular at one.
func TestPlanetsScroll(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	e, c := a.e, a.cl
	if got, want := planetsCounts(e), "1 planet\n1 system"; got != want || len(e.Colonies) != 1 || e.ControlledSystems() != 1 {
		t.Errorf("counts %q, want %q", got, want)
	}
	other := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	e.Planet(other, 0).Owner = e.Index // control is the planets' owners
	for len(e.Colonies) < planetsRows()+5 {
		e.Colonies = append(e.Colonies, &game.Colony{Planet: e.Planet(other, 0), Star: other, PlanetIdx: 0})
	}
	n := len(e.Colonies)
	if got, want := planetsCounts(e), fmt.Sprintf("%d planets\n%d systems", n, e.ControlledSystems()); got != want || e.ControlledSystems() != 2 {
		t.Errorf("counts %q, want %q", got, want)
	}
	s := newPlanetsScreen()
	for _, k := range []struct{ d, want int }{{-5, 0}, {3, 3}, {1 << 20, n - planetsRows()}, {-1, n - planetsRows() - 1}, {-(1 << 20), 0}} {
		s.scroll(a, k.d)
		if s.off != k.want {
			t.Errorf("scroll %d: off %d, want %d", k.d, s.off, k.want)
		}
	}
	s.scroll(a, 4)
	if i := s.lineAt(a, planetsRowY(0)); i != 4 {
		t.Errorf("scrolled 4: top colony %d", i)
	}
	e.Colonies = e.Colonies[:2]
	s.scroll(a, 0)
	if s.off != 0 {
		t.Errorf("colonies gone: off %d", s.off)
	}
}

// The cluster box: the home, the cluster, the empires; three smallFace
// lines, the box's three.
func TestClusterText(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	str := clusterText(a.e, a.cl, a.g)
	ls := strings.Split(str, "\n")
	h := float64(mapStack[2].Y1 - mapStack[2].Y0)
	if len(ls) != linesFit(h, smallFace) || ls[0] != "Home: "+a.e.StarLabel(a.cl.Home) ||
		!strings.HasPrefix(ls[1], a.cl.Density.String()) || ls[2] != fmt.Sprintf("%d empires", len(g.Empires)) {
		t.Errorf("cluster text %q", str)
	}
	if n := strings.Count(clusterText(a.e, a.cl, nil), "\n"); n != 1 {
		t.Errorf("no game: %d line breaks", n)
	}
}

// Render and Text run over every row with the game's sprites, and the
// sprites fit the row as its geometry assumes.
func TestPlanetsDraws(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a := planetsApp(t)
	a.a = as
	for _, b := range outputBanks {
		for f := 0; f <= 20; f++ {
			if r := a.spr(b, f).Image.Bounds(); r.Dx()+a.spr(b, f).Offset.X > outPicW || r.Dy()+a.spr(b, f).Offset.Y > outPicH {
				t.Errorf("%s %d: %v + %v past %dx%d", b, f, r, a.spr(b, f).Offset, outPicW, outPicH)
			}
		}
	}
	s := newPlanetsScreen()
	a.push(s)
	s.hover = 0
	sc := ebiten.NewImage(screenW, screenH)
	s.Render(a, sc)
	s.Text(a, sc)
}

// sortApp is an app whose empire holds colonies of bare planets with
// varied Pop, Project and orbit items (so every key has ties and
// distinct values), as many as the rows plus a few.
func sortApp(t *testing.T) *app {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	a.e.Colonies = nil
	for k := range planetsRows() + 4 {
		p := &game.Planet{Surface: map[game.Cell]*game.Square{}}
		for i := range p.Orbit {
			p.Orbit[i] = game.NoItem
		}
		for i := range k * 7 % 4 {
			p.Orbit[i] = game.Factory
		}
		for i := range k * 3 % 3 {
			p.Orbit[game.OrbitSlots-1-i] = game.Laboratory
		}
		c := &game.Colony{Planet: p, Pop: 1 + k*5%4, Star: -1, PlanetIdx: -1}
		if k%3 != 0 {
			c.Project = &game.Project{}
			if k%4 == 1 {
				c.Project.Item = game.EndlessParty
			}
		}
		a.e.Colonies = append(a.e.Colonies, c)
	}
	return a
}

// Each key sorts by the model's value (descending, ties in Colonies
// order), founded keeps the order; idle first splits the groups (the
// idle with free people, the ongoing, the idle without, the busy), each
// in key order.
func TestColonyOrder(t *testing.T) {
	a := sortApp(t)
	e := a.e
	// The groups by the project line's colour: red (idle, free people),
	// light blue (ongoing), yellow (idle), the rest.
	rank := func(c *game.Colony) int {
		switch projColour(c) {
		case errColour:
			return 0
		case ongoingColour:
			return 1
		case yellow:
			return 2
		}
		return 3
	}
	seen := map[int]bool{}
	for _, c := range e.Colonies {
		seen[rank(c)] = true
	}
	if len(seen) != 4 {
		t.Fatalf("fixture: ranks %v, want all four", seen)
	}
	val := func(c *game.Colony, key int) int {
		ind, res, pro := c.Shown()
		return [sortKeys]int{0, c.Pop, ind, res, pro}[key]
	}
	// The idle first groups: the ongoing before the idle when projects
	// first, after them else; the busy last.
	group := func(c *game.Colony, pf bool) int {
		return map[bool][4]int{false: {0, 3, 1, 4}, true: {1, 0, 2, 4}}[pf][rank(c)]
	}
	for key := range sortKeys {
		for _, idle := range []bool{false, true} {
			for _, pf := range []bool{false, true} {
				want := make([]int, len(e.Colonies))
				for i := range want {
					want[i] = i
				}
				slices.SortStableFunc(want, func(i, j int) int {
					ci, cj := e.Colonies[i], e.Colonies[j]
					if idle && group(ci, pf) != group(cj, pf) {
						return group(ci, pf) - group(cj, pf)
					}
					return val(cj, key) - val(ci, key)
				})
				if got := colonyOrder(e, idle, pf, key); !slices.Equal(got, want) {
					t.Errorf("key %s idle %v projects %v: %v, want %v", sortNames[key], idle, pf, got, want)
				}
			}
		}
	}
	// the fixture sorts differently by key, else the test proves nothing
	if slices.Equal(colonyOrder(e, false, false, sortPop), colonyOrder(e, false, false, sortIndustry)) ||
		slices.Equal(colonyOrder(e, false, false, sortFounded), colonyOrder(e, false, false, sortPop)) {
		t.Error("fixture: keys give the same order")
	}
}

// A click on a toggle of box 3 flips it, in box 4 advances, a right
// click goes back, both ways wrapping; the state is on the app, the
// scroll resets; the hovered button is found by position.
func TestPlanetsSortButtons(t *testing.T) {
	a := sortApp(t)
	s := newPlanetsScreen()
	a.push(s)
	for r, want := range map[rect]int{toggleRect(0): idleButton, toggleRect(1): projectsButton, mapStack[4]: sortButton} {
		x, y := centre(r)
		if got := planetsButtonAt(x, y); got != want {
			t.Errorf("%v: button %d, want %d", r, got, want)
		}
		if !inside(r, mapStack[3]) && r != mapStack[4] {
			t.Errorf("toggle %v out of box 3", r)
		}
	}
	if toggleRect(0).Y1 > toggleRect(1).Y0 {
		t.Errorf("toggles overlap: %v %v", toggleRect(0), toggleRect(1))
	}
	for _, r := range []rect{mapStack[2], newsBox, mapArea} {
		if x, y := centre(r); planetsButtonAt(x, y) != noButton {
			t.Errorf("%v is a sort button", r)
		}
	}
	s.off = 3
	s.click(a, idleButton, false)
	if !a.planetsIdleFirst || s.off != 0 {
		t.Errorf("toggle: idle %v off %d", a.planetsIdleFirst, s.off)
	}
	s.click(a, idleButton, true)
	if a.planetsIdleFirst {
		t.Error("second toggle left idle first on")
	}
	s.off = 3
	if s.click(a, projectsButton, false); !a.planetsProjectsFirst || a.planetsIdleFirst || s.off != 0 {
		t.Errorf("projects toggle: %v, idle %v, off %d", a.planetsProjectsFirst, a.planetsIdleFirst, s.off)
	}
	if s.click(a, projectsButton, false); a.planetsProjectsFirst {
		t.Error("second projects toggle left it on")
	}
	for k := 1; k <= sortKeys; k++ {
		s.off = 2
		s.click(a, sortButton, false)
		if a.planetsSort != k%sortKeys || s.off != 0 {
			t.Fatalf("advance %d: key %d off %d", k, a.planetsSort, s.off)
		}
	}
	s.click(a, sortButton, true)
	if a.planetsSort != sortKeys-1 {
		t.Errorf("back from founded: key %d", a.planetsSort)
	}
	s.click(a, sortButton, true)
	if a.planetsSort != sortKeys-2 {
		t.Errorf("back: key %d", a.planetsSort)
	}
	s.click(a, noButton, false)
	if a.planetsSort != sortKeys-2 {
		t.Error("noButton moved the key")
	}
	// a fresh screen keeps the state; rows go through the order, and so
	// does the colony a row opens; a colony gone leaves no stale row
	a.planetsIdleFirst = true
	n := newPlanetsScreen()
	ord := colonyOrder(a.e, true, a.planetsProjectsFirst, a.planetsSort)
	for r := range planetsRows() {
		if i := n.lineAt(a, planetsRowY(r)); i != ord[r] {
			t.Errorf("row %d: colony %d, want %d", r, i, ord[r])
		}
	}
	n.scroll(a, 2)
	if i := n.colony(a, 0); i != ord[2] {
		t.Errorf("scrolled 2: colony %d, want %d", i, ord[2])
	}
	a.e.Colonies = a.e.Colonies[:2]
	n.scroll(a, 0)
	if n.off != 0 {
		t.Errorf("colonies gone: off %d", n.off)
	}
	for r := range 3 {
		if i := n.colony(a, r); i >= len(a.e.Colonies) || (r >= 2 && i != -1) {
			t.Errorf("row %d: colony %d after the cut", r, i)
		}
	}
}
