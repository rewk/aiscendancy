package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// shipTechs are what a first colony ship needs: the hulls Small and
// Medium, the Tonklin Motor, the Proton Shaver, the Colonizer.
var shipTechs = []game.Tech{game.OrbitalStructures, game.InterplanetaryExploration, game.TonklinDiary,
	game.Xenobiology, game.EnvironmentalEncapsulation}

// designApp is seed 1 Average with shipTechs granted and a design screen
// pushed.
func designApp(t *testing.T) (*app, *designScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	for _, tc := range shipTechs {
		e.Research.Grant(tc)
	}
	a := testApp(g)
	s := newDesignScreen()
	a.push(s)
	return a, s
}

// yardApp is seed 1 Average with shipTechs granted, the home colony
// holding two Factories, a Shipyard on orbit slot 0 and 8 people: it can
// build a ship. The planet screen is pushed on the home world.
func yardApp(t *testing.T) (*app, *planetScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	for _, tc := range shipTechs {
		e.Research.Grant(tc)
	}
	home := e.Colonies[0]
	for range 2 {
		at, ok := home.Planet.BestCell(game.Factory, game.Base)
		if !ok || home.Planet.Build(at, game.Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	if err := home.Planet.BuildOrbital(0, game.Shipyard); err != nil {
		t.Fatal(err)
	}
	home.Pop = 8
	a := testApp(g)
	s := newPlanetScreen(e.Cluster.Home, 0)
	a.push(s)
	return a, s
}

// yardDesign is a blank design screen building at the yardApp's colony,
// pushed as the Shipyard's Build Ship pushes it.
func yardDesign(t *testing.T) (*app, *planetScreen, *designScreen) {
	t.Helper()
	a, p := yardApp(t)
	s := newDesignScreenFor(p.col, freshTemplate())
	a.push(s)
	return a, p, s
}

// saveTemplates saves the player's templates the tests load: the
// colony template (ColonyTemplate, "Colonizer") and a "Scout" of a Best
// generator and drive, both of shipTechs' material.
func saveTemplates(t *testing.T, e *game.Empire) {
	t.Helper()
	scout := game.Template{Name: "Scout", Hull: game.Small, Parts: []game.Part{{Best: true, Cat: game.Generator}, {Best: true, Cat: game.Drive}}}
	for _, tp := range []game.Template{game.ColonyTemplate(false), scout} {
		if err := e.AddTemplate(tp); err != nil {
			t.Fatal(err)
		}
	}
}

// blankDesign tells whether s is a design screen building at col (nil:
// nowhere) on a fresh template: no part, freshTemplate's hull, nothing
// loaded, no box up.
func blankDesign(s any, col *game.Colony) bool {
	d, ok := s.(*designScreen)
	return ok && d.col == col && d.ship == nil && len(d.t.Parts) == 0 && d.t.Hull == freshTemplate().Hull && d.loaded == "" && d.box == nil
}

// fixed is gs as fixed template parts.
func fixed(gs ...game.Gizmo) []game.Part {
	out := make([]game.Part, len(gs))
	for i, g := range gs {
		out[i] = game.Part{Gizmo: g, Cat: game.Gizmos[g].Cat}
	}
	return out
}

// clickR left-clicks the middle of r on s; hoverR hovers it.
func clickR(a *app, s *designScreen, r rect) bool { x, y := centre(r); return s.leftClick(a, x, y) }
func hoverR(a *app, s *designScreen, r rect)      { x, y := centre(r); s.hoverAt(a, x, y) }

// centre is the middle of r in native px.
func centre(r rect) (float64, float64) {
	return float64(r.X0+r.X1) / 2, float64(r.Y0+r.Y1) / 2
}

// Each hull's slot table: Hull.Slots() entries, distinct, on the lattice
// (u+v even), sorted by v then u (the paint order), every diamond and
// tile floor inside the big area, clear of the picture box.
func TestHullSlots(t *testing.T) {
	for h := range game.NumHulls {
		sl := hullSlots[h]
		if len(sl) != h.Slots() {
			t.Errorf("%v: %d slots, the hull has %d", h, len(sl), h.Slots())
		}
		for k, p := range sl {
			if (p.u+p.v)%2 != 0 {
				t.Errorf("%v slot %d %v off the lattice", h, k, p)
			}
			if k > 0 {
				if q := sl[k-1]; q.v > p.v || q.v == p.v && q.u >= p.u {
					t.Errorf("%v slot %d %v not after %v", h, k, p, q)
				}
			}
			x, y := desSlotCentre(h, k)
			if x-slotHW < float64(desSlots.X0) || x+slotHW > float64(desSlots.X1) ||
				y-slotHH < float64(desSlots.Y0) || y+slotHH > float64(desSlots.Y1) {
				t.Errorf("%v slot %d at (%v,%v) leaves the area %+v", h, k, x, y, desSlots)
			}
			// The tile's floor ends 17 original px under its origin.
			if _, ay := slotAnchor(h, k); ay+zoom*17 > float64(desSlots.Y1) {
				t.Errorf("%v slot %d tile floor ends at %v", h, k, ay+zoom*17)
			}
			if x+slotHW > float64(desPicture.X0) && x-slotHW < float64(desPicture.X1) &&
				y+slotHH > float64(desPicture.Y0) && y-slotHH < float64(desPicture.Y1) {
				t.Errorf("%v slot %d at (%v,%v) under the picture", h, k, x, y)
			}
		}
	}
}

// slotAt: a diamond's centre and a point on its edge are its slot, its
// bounding box's corner is no slot (nor the neighbour's); the picture box
// and the chrome hold none.
func TestDesignSlotAt(t *testing.T) {
	for h := range game.NumHulls {
		for k := range hullSlots[h] {
			cx, cy := desSlotCentre(h, k)
			for _, d := range [][2]float64{{0, 0}, {slotHW, 0}, {-slotHW / 2, slotHH / 2}, {0, -slotHH}} {
				if got, ok := slotAt(h, cx+d[0], cy+d[1]); !ok || got != k {
					t.Errorf("%v slot %d %+v: slotAt %d %v", h, k, d, got, ok)
				}
			}
			for _, d := range [][2]float64{{slotHW - 1, slotHH - 1}, {-slotHW + 1, -slotHH + 1}} {
				if got, ok := slotAt(h, cx+d[0], cy+d[1]); ok && got == k {
					t.Errorf("%v slot %d: its box corner %+v hits it", h, k, d)
				}
			}
		}
		for _, p := range [][2]float64{{zoom * 10, zoom * 10}, {zoom * 164, zoom * 225}, {zoom * 559, zoom * 279}, {float64(desSlots.X1) - 1, float64(desSlots.Y0)}} {
			if k, ok := slotAt(h, p[0], p[1]); ok {
				t.Errorf("%v: slotAt(%v) = %d outside the slots", h, p, k)
			}
		}
	}
}

// add fills the slots in order up to the hull's count; remove closes the
// gap; the realisation follows every change.
func TestAddRemove(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	n := s.t.Hull.Slots()
	parts := append(fixed(game.ProtonShaver), game.Part{Best: true, Cat: game.Drive})
	parts = append(parts, fixed(game.Colonizer, game.IonWrap, game.Colonizer)...)
	if len(parts) != n {
		t.Fatalf("the fixture needs %d parts", n)
	}
	for _, p := range parts {
		if err := s.add(r, p); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(s.t.Parts, parts) {
		t.Fatalf("parts %v", s.t.Parts)
	}
	if d, ok := s.t.Realise(r); !ok || !slices.Equal(s.d.Parts, d.Parts) || s.d.Parts[1] != game.TonklinMotor {
		t.Errorf("realised %v, want %v", s.d.Parts, d.Parts)
	}
	if err := s.add(r, fixed(game.TonklinMotor)[0]); err == nil || len(s.t.Parts) != n {
		t.Errorf("past the hull: err %v, %d parts", err, len(s.t.Parts))
	}
	s.remove(r, 1)
	want := slices.Concat(parts[:1], parts[2:])
	if !slices.Equal(s.t.Parts, want) || len(s.d.Parts) != 4 || s.d.Parts[1] != game.Colonizer {
		t.Errorf("after remove: %v (realised %v), want %v", s.t.Parts, s.d.Parts, want)
	}
	s.remove(r, 7) // an empty slot
	if len(s.t.Parts) != 4 {
		t.Error("removing an empty slot changed the parts")
	}
	// A right click on slot 0's diamond takes its part out.
	x, y := desSlotCentre(s.t.Hull, 0)
	s.rightClick(r, x, y)
	if !slices.Equal(s.t.Parts, want[1:]) || len(s.d.Parts) != 3 {
		t.Errorf("right click: %v", s.t.Parts)
	}
}

// The hull button steps to the next hull research allows, skipping the
// others, and drops the parts past the new hull's slots.
func TestCycleHull(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	r.Grant(game.LargeScaleConstruction) // Enormous, not Large
	if r.Known[game.Large.Tech()] || !r.Known[game.Enormous.Tech()] {
		t.Fatal("fixture: Large known or Enormous not")
	}
	var seen []game.Hull
	for range 4 {
		cx, cy := centre(desHull)
		s.leftClick(a, cx, cy)
		seen = append(seen, s.t.Hull)
	}
	if want := []game.Hull{game.Medium, game.Enormous, game.Small, game.Medium}; !slices.Equal(seen, want) {
		t.Errorf("hulls %v, want %v", seen, want)
	}
	s.t.Hull = game.Enormous
	s.t.Parts, s.t.Slots = fixed(0, 1, 2, 3, 4, 5, 6, 7), nil
	s.cycleHull(r)
	if s.t.Hull != game.Small || !slices.Equal(s.t.Parts, fixed(0, 1, 2, 3, 4)) || s.d.Hull != game.Small || len(s.d.Parts) != 5 {
		t.Errorf("Enormous -> %v with %v, realised %v", s.t.Hull, s.t.Parts, s.d)
	}
}

// The list at start is "Best Drive" then the drives research allows, in
// table order; a click on an entry (at its grid cell) adds that part, the
// Best one as Best (the template keeps the category). The Specials have
// no Best entry.
func TestDesignList(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	click := func(x, y float64) { s.leftClick(a, x, y) }
	want := []game.Part{{Best: true, Cat: game.Drive}}
	for g := range game.NumGizmos {
		if game.Gizmos[g].Cat == game.Drive && r.AllowsGizmo(g) {
			want = append(want, game.Part{Gizmo: g, Cat: game.Drive})
		}
	}
	if len(want) < 2 {
		t.Fatal("fixture: no drive allowed")
	}
	if got := s.list(r); s.cat != game.Drive || !slices.Equal(got, want) {
		t.Errorf("list %v, want %v", got, want)
	}
	click(cellCentre(0))
	click(desSlotCentre(s.t.Hull, 0))
	click(cellCentre(1))
	click(desSlotCentre(s.t.Hull, 1))
	if !slices.Equal(s.t.Parts, want[:2]) {
		t.Errorf("clicks on entries 0, 1: parts %v", s.t.Parts)
	}
	// The Best entry realises to what game's Realise picks, the hover
	// shows it.
	best, _ := game.Template{Parts: want[:1]}.Realise(r)
	if s.d.Parts[0] != best.Parts[0] {
		t.Errorf("Best Drive realised %v, want %v", s.d.Parts[0], best.Parts[0])
	}
	s.hover, s.index = hoverList, 0
	if g, ok := s.shown(r); !ok || g != best.Parts[0] {
		t.Errorf("shown %v %v", g, ok)
	}
	if entryName(want[0]) != "Best Drive" || entryName(want[1]) != want[1].Gizmo.String() {
		t.Errorf("names %q %q", entryName(want[0]), entryName(want[1]))
	}
	s.cat = game.Special
	if l := s.list(r); len(l) == 0 || l[0].Best {
		t.Errorf("specials %v", l)
	}
}

// cellCentre is the middle of list cell i.
func cellCentre(i int) (float64, float64) { return centre(listCell(i)) }

// The grid: the cells tile the list 4 a row inside its box; listAt maps a
// cell's centre to its entry (the 5th opens the second row), the gap
// between two cells and a cell past the list to none. With every tech
// known each category's list holds the parts the table files under it,
// the Specials the most, and all fit the cells: nothing scrolls.
func TestDesignListGrid(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	at := func(x, y float64) (int, bool) { return s.listAt(r, x, y) }
	for i := range listCells {
		c := listCell(i)
		if c.X0 < desList.X0 || c.X1 > desList.X1 || c.Y0 < desList.Y0 || c.Y1 > desList.Y1 {
			t.Errorf("cell %d %+v leaves the list %+v", i, c, desList)
		}
	}
	for tc := range game.NumTechs {
		r.Grant(tc)
	}
	most := 0
	for c := range game.NumGizmoCats {
		s.cat = c
		var want []game.Part
		if c != game.Special {
			want = append(want, game.Part{Best: true, Cat: c})
		}
		for g := range game.NumGizmos {
			if game.Gizmos[g].Cat == c && r.AllowsGizmo(g) {
				want = append(want, game.Part{Gizmo: g, Cat: c})
			}
		}
		l := s.list(r)
		if !slices.Equal(l, want) || len(l) > listCells {
			t.Errorf("%v: %d entries %v, want %v (%d cells)", c, len(l), l, want, listCells)
		}
		if c == game.Special {
			most = len(l)
		}
		// Every entry's cell answers it.
		for i := range l {
			if k, ok := at(cellCentre(i)); !ok || k != i {
				t.Errorf("%v entry %d: listAt %d %v", c, i, k, ok)
			}
		}
		if _, ok := at(cellCentre(len(l))); ok {
			t.Errorf("%v: the cell past %d entries answers", c, len(l))
		}
	}
	if most != 45 {
		t.Errorf("%d specials with every tech, the brief counted 44 and the Space Habitat", most)
	}
	s.cat = game.Special
	if k, ok := at(cellCentre(0)); !ok || k != 0 {
		t.Errorf("cell 0: %d %v", k, ok)
	}
	x, y := cellCentre(4)
	if k, ok := s.listAt(r, x, y); !ok || k != 4 || y < float64(listCell(0).Y1) || x != float64(listCell(0).X0+listCell(0).X1)/2 {
		t.Errorf("cell 4 at (%v,%v): %d %v", x, y, k, ok)
	}
	_, y = cellCentre(0)
	gap := float64(listCell(0).X1) + 0.5 // between cells 0 and 1
	if k, ok := s.listAt(r, gap, y); ok {
		t.Errorf("the gap answers %d", k)
	}
	x, _ = cellCentre(0)
	if k, ok := s.listAt(r, x, float64(listCell(0).Y1)+1); ok {
		t.Errorf("the row gap answers %d", k)
	}
}

// Hovering an entry shows its part, a click picks it (a second drops
// it), a click on a slot then places it there, a hole before it kept; a
// category with no allowed gizmo lists nothing, not even its Best entry.
func TestDesignListHoverClick(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	click := func(x, y float64) { s.leftClick(a, x, y) }
	hover := func(x, y float64) { s.hoverAt(a, x, y) }
	l := s.list(r)
	i := len(l) - 1
	hover(cellCentre(i))
	if s.hover != hoverList || s.index != i {
		t.Fatalf("hover %v %d, want entry %d", s.hover, s.index, i)
	}
	if g, ok := s.shown(r); !ok || g != l[i].Gizmo {
		t.Errorf("shown %v %v, want %v", g, ok, l[i].Gizmo)
	}
	click(cellCentre(i))
	if !s.holding || s.held != l[i] || len(s.t.Parts) != 0 {
		t.Fatalf("pick: holding %v %v, parts %v", s.holding, s.held, s.t.Parts)
	}
	click(cellCentre(i))
	if s.holding {
		t.Error("a second click kept the part")
	}
	click(desSlotCentre(s.t.Hull, 2))
	if len(s.t.Parts) != 0 {
		t.Errorf("a slot click with nothing held placed %v", s.t.Parts)
	}
	click(cellCentre(i))
	click(desSlotCentre(s.t.Hull, 2))
	if !slices.Equal(s.t.Parts, l[i:]) || !slices.Equal(s.t.Slots, []int{2}) || !s.holding {
		t.Errorf("parts %v slots %v holding %v, want %v in slot 2, still held", s.t.Parts, s.t.Slots, s.holding, l[i:])
	}
	s.cat = game.Weapon
	if _, ok := realisePart(r, game.Part{Best: true, Cat: game.Weapon}); ok {
		t.Fatal("fixture: a weapon is allowed")
	}
	hover(cellCentre(0))
	if l := s.list(r); len(l) != 0 || s.hover == hoverList {
		t.Errorf("no weapon allowed: list %v, hover %v", l, s.hover)
	}
	r.Grant(game.Gizmos[game.MassBarrageGun].ResearchReq)
	if l := s.list(r); len(l) < 2 || !l[0].Best || l[0].Cat != game.Weapon {
		t.Errorf("a weapon allowed: list %v", l)
	}
}

// The load button sits under the title lines and holds its label; a click
// on it enters the pick mode, a click on template line 1 loads a copy of
// Templates[1] (its parts not aliased) and ends the mode; in the mode a
// category button or a list cell does nothing; Esc (cancel) and the button
// again leave the mode without loading.
func TestDesignLoadTemplate(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	saveTemplates(t, a.e)
	click := func(x, y float64) { s.leftClick(a, x, y) }
	hover := func(x, y float64) { s.hoverAt(a, x, y) }
	if title := float64(desTitle.Y0+padY) + 5*bigFace.Size*1.25; float64(desLoad.Y0) < title ||
		desLoad.Y1 > desTitle.Y1 || float64(desLoad.X1-desLoad.X0) < ui.Width("Load template", bigFace) {
		t.Errorf("button %+v: title lines end at %v native", desLoad, title)
	}
	if len(a.e.Templates) != 2 {
		t.Fatalf("fixture templates %v", a.e.Templates)
	}
	before := s.t
	hover(centre(desLoad))
	if s.hover != hoverLoad {
		t.Errorf("button hover %v", s.hover)
	}
	s.editing = true
	click(centre(desLoad))
	if !s.picking || s.editing {
		t.Fatalf("button: picking %v, editing %v", s.picking, s.editing)
	}
	// The list's clicks are the templates' now; a category button is dead.
	click(centre(catHoles[0].r))
	if s.cat != game.Drive || !s.picking || len(s.t.Parts) != 0 {
		t.Errorf("in the pick mode a category click: cat %v, picking %v", s.cat, s.picking)
	}
	x := float64(desList.X0+desList.X1) / 2
	y := float64(pickTop(1) + pickPitch/2)
	s.hoverAt(a, x, y)
	if s.hover != hoverPick || s.index != 1 {
		t.Errorf("line hover %v %d", s.hover, s.index)
	}
	if _, ok := s.pickAt(a, x, float64(pickTop(len(a.e.Templates))+1)); ok {
		t.Error("a line past the templates answers")
	}
	s.leftClick(a, x, y)
	src := a.e.Templates[1]
	if s.picking || s.t.Name != src.Name || s.t.Hull != src.Hull || !slices.Equal(s.t.Parts, src.Parts) || s.loaded != src.Name {
		t.Fatalf("loaded %+v (picking %v, loaded %q), want %+v", s.t, s.picking, s.loaded, src)
	}
	if d, ok := src.Realise(r); !ok || !slices.Equal(s.d.Parts, d.Parts) || s.d.Name != src.Name {
		t.Errorf("realised %+v, want %+v %v", s.d, d, ok)
	}
	keep := src.Parts[0]
	s.t.Parts[0] = game.Part{Best: true, Cat: game.Weapon}
	if a.e.Templates[1].Parts[0] != keep {
		t.Errorf("the editor's parts alias the empire's: %v", a.e.Templates[1].Parts)
	}

	// Esc leaves the mode without loading, then (a second Esc) nothing
	// to cancel: the app pops.
	s.t = before
	click(centre(desLoad))
	if !s.cancel() || s.picking || s.t.Name != before.Name || s.cancel() {
		t.Errorf("cancel: picking %v, name %q", s.picking, s.t.Name)
	}
	// The button again leaves it too.
	click(centre(desLoad))
	click(centre(desLoad))
	if s.picking || s.t.Name != before.Name {
		t.Errorf("button twice: picking %v, name %q", s.picking, s.t.Name)
	}
}

// Each category hole lists its category; the hull hole none.
func TestCategoryHoles(t *testing.T) {
	a, s := designApp(t)
	want := map[game.GizmoCat]rect{
		game.Weapon:    orig(485, 7, 516, 38),
		game.Shield:    orig(602, 7, 633, 38),
		game.Drive:     orig(485, 46, 516, 77),
		game.Scanner:   orig(524, 46, 555, 77),
		game.Generator: orig(563, 46, 594, 77),
		game.Special:   orig(602, 46, 633, 77),
	}
	if len(catHoles) != int(game.NumGizmoCats) {
		t.Fatalf("%d holes", len(catHoles))
	}
	for c, r := range want {
		s.cat = game.Drive
		cx, cy := centre(r)
		s.leftClick(a, cx, cy)
		if s.cat != c {
			t.Errorf("hole %+v: cat %v, want %v", r, s.cat, c)
		}
	}
	s.cat = game.Generator
	cx, cy := centre(desHull)
	s.leftClick(a, cx, cy)
	if s.cat != game.Generator {
		t.Error("the hull button changed the category")
	}
}

// The save button sits under the load button, same x and height, 8 px
// down, inside the title box, wide enough for its label; it lights on
// hover, it and S open the save box on the design's name, which takes
// every input: the typed runes edit the field (S is a character there),
// Enter saves a copy under the typed name (AddTemplate) and the design
// takes it, the box closes; a second save under that name replaces in
// place; the screen's later edits stay off the saved parts.
func TestDesignSaveBox(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	if hoverR(a, s, desSave); s.hover != hoverSave {
		t.Errorf("hover %v over the save button", s.hover)
	}
	if want := defaultName(a); s.t.Name != want {
		t.Errorf("fresh name %q, default %q", s.t.Name, want)
	}
	s.add(r, fixed(game.ProtonShaver)[0])
	s.picking = true
	if clickR(a, s, desSave); s.box == nil || s.box.mode != nameSave || s.box.name != s.t.Name || s.picking || !s.typing() {
		t.Fatalf("button: box %+v, picking %v", s.box, s.picking)
	}
	want := s.t.Name[:len(s.t.Name)-1] + "Sx"
	s.handleKeys(a, nil, desKeys{backspace: true})
	s.handleKeys(a, []rune("Sx\u0007"), desKeys{s: true})
	if s.box == nil || s.box.name != want || len(a.e.Templates) != 0 {
		t.Fatalf("typed: box %+v, want %q, %d templates", s.box, want, len(a.e.Templates))
	}
	s.handleKeys(a, []rune(strings.Repeat("é", 30)), desKeys{})
	if n := len([]rune(s.box.name)); n != nameCap {
		t.Errorf("field of %d runes", n)
	}
	for range nameCap {
		s.handleKeys(a, nil, desKeys{backspace: true})
	}
	s.handleKeys(a, []rune("Probe"), desKeys{})
	if built := s.handleKeys(a, nil, desKeys{enter: true}); built || s.box != nil || s.msgErr || s.msg != "saved" {
		t.Fatalf("Enter: built %v, box %+v, msg %q", built, s.box, s.msg)
	}
	if len(a.e.Templates) != 1 || a.e.Templates[0].Name != "Probe" || s.t.Name != "Probe" || s.d.Name != "Probe" ||
		!slices.Equal(a.e.Templates[0].Parts, s.t.Parts) {
		t.Fatalf("saved %v, design %q (realised %q)", a.e.Templates, s.t.Name, s.d.Name)
	}
	// S opens it again on the new name; Save (the click) replaces in place.
	s.add(r, game.Part{Best: true, Cat: game.Drive})
	if s.handleKeys(a, []rune("s"), desKeys{s: true}); s.box == nil || s.box.name != "Probe" {
		t.Fatalf("S: box %+v", s.box)
	}
	_, yes, _ := s.box.prompt().rects()
	clickR(a, s, yes)
	if saved := a.e.Templates[0]; len(a.e.Templates) != 1 || s.box != nil || len(saved.Parts) != 2 || !saved.Parts[1].Best || saved.Parts[1].Cat != game.Drive {
		t.Errorf("same name: %v, box %+v", a.e.Templates, s.box)
	}
	s.remove(r, 1) // the screen keeps editing: the saved parts stay
	if len(a.e.Templates[0].Parts) != 2 {
		t.Errorf("the screen's edit reached the saved template: %v", a.e.Templates[0].Parts)
	}
}

// Cancel, Esc and the back button (app.back: the screen's cancel first)
// close the box, nothing saved, the name kept, the screen still there: a
// second cancel has nothing to close. Under the box the screen's own
// clicks and keys are dead.
func TestDesignSaveBoxCancel(t *testing.T) {
	a, s := designApp(t)
	name := s.t.Name
	s.handleKeys(a, nil, desKeys{s: true})
	s.handleKeys(a, []rune("Other"), desKeys{})
	_, _, no := s.box.prompt().rects()
	clickR(a, s, no)
	if s.box != nil || len(a.e.Templates) != 0 || s.t.Name != name {
		t.Errorf("Cancel: box %+v, %v, name %q", s.box, a.e.Templates, s.t.Name)
	}
	s.add(a.e.Research, fixed(game.ProtonShaver)[0])
	s.handleKeys(a, nil, desKeys{s: true})
	parts := len(s.t.Parts)
	clickR(a, s, listCell(0)) // the list is under the box: dead
	sx, sy := desSlotCentre(s.t.Hull, 0)
	s.rightClick(a.e.Research, sx, sy)
	s.handleKeys(a, nil, desKeys{f2: true})
	if len(s.t.Parts) != parts || s.editing || s.box == nil {
		t.Errorf("under the box: parts %d, editing %v, box %+v", len(s.t.Parts), s.editing, s.box)
	}
	if err := a.back(); err != nil || s.box != nil || a.top() != screen(s) || len(a.e.Templates) != 0 || s.t.Name != name {
		t.Errorf("Esc: %v, box %+v, top %T", err, s.box, a.top())
	}
	if s.cancel() {
		t.Error("a second cancel found something to close")
	}
}

// The box refuses an empty name and undiscovered material, staying open
// with the refusal (red: err), nothing saved, the design's name kept.
func TestDesignSaveBoxRefused(t *testing.T) {
	a, s := designApp(t)
	name := s.t.Name
	s.handleKeys(a, nil, desKeys{s: true})
	for range nameCap {
		s.handleKeys(a, nil, desKeys{backspace: true})
	}
	s.handleKeys(a, nil, desKeys{enter: true})
	if s.box == nil || s.box.err == "" || len(a.e.Templates) != 0 || s.t.Name != name {
		t.Errorf("empty: box %+v, %v", s.box, a.e.Templates)
	}
	// A part research does not allow (put in by hand: the lists hold none).
	if a.e.Research.AllowsGizmo(game.Nanoshell) {
		t.Fatal("fixture: Nanoshell allowed")
	}
	s.box = nil
	s.t.Parts = append(s.t.Parts, fixed(game.Nanoshell)...)
	s.handleKeys(a, nil, desKeys{s: true})
	s.handleKeys(a, nil, desKeys{enter: true})
	want := a.e.AddTemplate(game.Template{Name: s.box.name, Hull: s.t.Hull, Parts: s.t.Parts})
	if want == nil || s.box == nil || s.box.err != want.Error() || len(a.e.Templates) != 0 || s.t.Name != name {
		t.Errorf("undiscovered: box %+v, want %v, %v", s.box, want, a.e.Templates)
	}
}

// The title box's three buttons: one row on its bottom (padY up), inside
// it, under its five text lines, left to right Load, Save, Build, apart,
// each wide enough for its label.
func TestDesignButtons(t *testing.T) {
	bs := []struct {
		r     rect
		label string
	}{{desLoad, "Load template"}, {desSave, "Save as template"}, {desBuild, "Build"}}
	text := float64(desTitle.Y0+padY) + 4*bigFace.Size*1.25
	for i, b := range bs {
		if b.r.Y1 != desTitle.Y1-padY || float64(b.r.Y0) < text || b.r.X0 < desTitle.X0 || b.r.X1 > desTitle.X1 ||
			float64(b.r.X1-b.r.X0) < ui.Width(b.label, bigFace)+2*padX {
			t.Errorf("%s at %+v, title %+v, text ends at %.0f", b.label, b.r, desTitle, text)
		}
		if i > 0 && b.r.X0 < bs[i-1].r.X1+btnGap {
			t.Errorf("%s at %+v overlaps %s at %+v", b.label, b.r, bs[i-1].label, bs[i-1].r)
		}
	}
}

// The title's time to build: days at the colony's industry today
// (Colony.DaysFor), the cost without a colony.
func TestBuildTime(t *testing.T) {
	a, _, s := yardDesign(t)
	r := a.e.Research
	s.add(r, game.Part{Best: true, Cat: game.Generator})
	s.add(r, game.Part{Best: true, Cat: game.Drive})
	n, ok := s.col.DaysFor(s.d.Cost())
	if !ok || n < 2 {
		t.Fatalf("fixture: %d days %v", n, ok)
	}
	if l, _, _ := s.titleLines(a.e); l[1] != itoa(n)+" days" {
		t.Errorf("at a colony: %q, want %d days", l[1], n)
	}
	at, ok := s.col.Planet.BestCell(game.Factory, s.col.Traits())
	if !ok || s.col.Start(r, game.Factory, at) != nil {
		t.Fatal("no Factory to start")
	}
	if n, _ := s.col.DaysFor(s.d.Cost()); s.buildTime() != "busy: "+itoa(n)+" days" {
		t.Errorf("busy: %q", s.buildTime())
	}
	s.col = nil
	if got := s.buildTime(); got != "cost "+itoa(s.d.Cost()) {
		t.Errorf("no colony: %q", got)
	}
}

// The title box: the name with its cursor only in the text mode, F2 or a
// click entering it (S a character there, not the box), Enter ending it;
// Esc ends the mode before it leaves the screen.
func TestDesignTitleEdit(t *testing.T) {
	a, s := designApp(t)
	name := s.t.Name
	s.handleKeys(a, []rune("x"), desKeys{})
	if s.t.Name != name || s.typing() {
		t.Errorf("typed outside the mode: %q", s.t.Name)
	}
	s.handleKeys(a, nil, desKeys{f2: true})
	if !s.editing || !s.typing() {
		t.Fatal("F2 did not enter the text mode")
	}
	s.handleKeys(a, nil, desKeys{backspace: true})
	s.handleKeys(a, []rune("x\u0007S"), desKeys{s: true})
	if want := name[:len(name)-1] + "xS"; s.t.Name != want || s.box != nil || s.d.Name != s.t.Name {
		t.Errorf("typed: %q (realised %q), want %q, box %+v", s.t.Name, s.d.Name, want, s.box)
	}
	s.handleKeys(a, []rune(strings.Repeat("é", 30)), desKeys{})
	if n := len([]rune(s.t.Name)); n != nameCap {
		t.Errorf("name of %d runes", n)
	}
	if built := s.handleKeys(a, nil, desKeys{enter: true}); built || s.editing || s.box != nil {
		t.Errorf("Enter in the mode: built %v, editing %v, box %+v", built, s.editing, s.box)
	}
	s.editing = true
	if !s.cancel() || s.editing || s.cancel() {
		t.Error("cancel")
	}
	clickR(a, s, desTitle)
	if !s.editing {
		t.Error("title click")
	}
}

// Templates are optional: on a blank screen from the Shipyard, parts
// added, Build in the box builds the ship (BuildShip) and closes the
// screen, the templates untouched.
func TestShipyardNoTemplate(t *testing.T) {
	a, p, s := yardDesign(t)
	r := a.e.Research
	for _, pt := range []game.Part{{Best: true, Cat: game.Generator}, {Best: true, Cat: game.Drive}} {
		s.add(r, pt)
	}
	s.add(r, fixed(game.Colonizer)[0])
	if s.handleKeys(a, nil, desKeys{enter: true}) || s.box == nil || s.box.mode != nameBuild {
		t.Fatalf("Enter: box %+v", s.box)
	}
	if !s.handleKeys(a, nil, desKeys{enter: true}) {
		t.Fatalf("not built: %q", s.box.err)
	}
	pj := p.col.Project
	if pj == nil || pj.Item != game.Ship || pj.Design == nil || !slices.Equal(pj.Design.Parts, s.d.Parts) || pj.Cost() != s.d.Cost() {
		t.Errorf("project %+v", pj)
	}
	if len(a.e.Templates) != 0 {
		t.Errorf("templates changed: %v", a.e.Templates)
	}
}

// The build box's name: the hull's without a load, following a hull
// switch; the last loaded template's after a load (not the title's, even
// renamed): that is the class; the field proposes the ship's name
// (ShipNameFor), what is typed there names the ship as it is.
func TestDesignBuildName(t *testing.T) {
	a, p, s := yardDesign(t)
	r := a.e.Research
	saveTemplates(t, a.e)
	s.handleKeys(a, nil, desKeys{enter: true})
	if s.box == nil || s.box.class != s.t.Hull.String() || s.box.name != a.e.ShipNameFor(s.box.class) || s.box.at != p.col.Planet.Name {
		t.Fatalf("no load: box %+v, hull %v", s.box, s.t.Hull)
	}
	s.cancel()
	s.cycleHull(r)
	if s.t.Hull == freshTemplate().Hull {
		t.Fatal("fixture: one hull only")
	}
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box.class != s.t.Hull.String() || s.box.name != a.e.ShipNameFor(s.t.Hull.String()) {
		t.Errorf("after a hull switch: %q (%q), hull %v", s.box.name, s.box.class, s.t.Hull)
	}
	s.cancel()
	s.load(r, a.e.Templates[0])
	s.t.Name = "Renamed"
	if clickR(a, s, desBuild); s.box == nil || s.box.mode != nameBuild || s.box.class != a.e.Templates[0].Name || s.box.name != a.e.ShipNameFor(a.e.Templates[0].Name) {
		t.Fatalf("after a load: box %+v", s.box)
	}
	for range nameCap {
		s.handleKeys(a, nil, desKeys{backspace: true})
	}
	s.handleKeys(a, []rune("Seeder"), desKeys{})
	_, yes, _ := s.box.prompt().rects()
	if !clickR(a, s, yes) {
		t.Fatalf("not built: %q", s.box.err)
	}
	class := a.e.Templates[0].Name
	if pj := p.col.Project; pj == nil || pj.Design == nil || pj.Design.Name != class || pj.ShipName != "Seeder" || s.t.Name != class {
		t.Errorf("project %+v, design %q", pj, s.t.Name)
	}
}

// A design saved as a template is that class: the build box then offers
// the saved name (the ship named after it, its refit finding the
// template), not the hull's nor an earlier loaded template's.
func TestDesignBuildNameAfterSave(t *testing.T) {
	a, _, s := yardDesign(t)
	r := a.e.Research
	saveTemplates(t, a.e)
	s.load(r, a.e.Templates[1])
	s.add(r, game.Part{Best: true, Cat: game.Generator})
	s.handleKeys(a, nil, desKeys{s: true})
	for range nameCap {
		s.handleKeys(a, nil, desKeys{backspace: true})
	}
	s.handleKeys(a, []rune("Scout II"), desKeys{})
	if s.handleKeys(a, nil, desKeys{enter: true}) || s.box != nil {
		t.Fatalf("not saved: %+v", s.box)
	}
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box == nil || s.box.class != "Scout II" || s.box.name != a.e.ShipNameFor("Scout II") {
		t.Errorf("build box after a save: %+v", s.box)
	}
}

// Cancel and Esc build nothing and keep the screen; an empty name and a
// refusal (the colony busy) stay in the box; without a colony Enter
// opens nothing and there is no Build button (the click falls on the
// title box, which edits the name).
func TestDesignBuildBox(t *testing.T) {
	a, p, s := yardDesign(t)
	saveTemplates(t, a.e)
	s.load(a.e.Research, a.e.Templates[0])
	if hoverR(a, s, desBuild); s.hover != hoverBuild {
		t.Fatalf("hover %v over the button", s.hover)
	}
	clickR(a, s, desBuild)
	_, yes, no := s.box.prompt().rects()
	if clickR(a, s, no) || s.box != nil || p.col.Project != nil {
		t.Errorf("Cancel: box %+v, project %+v", s.box, p.col.Project)
	}
	s.handleKeys(a, nil, desKeys{enter: true})
	if err := a.back(); err != nil || s.box != nil || a.top() != screen(s) || p.col.Project != nil {
		t.Errorf("Esc: %v, box %+v, top %T", err, s.box, a.top())
	}
	s.handleKeys(a, nil, desKeys{enter: true})
	for range nameCap {
		s.handleKeys(a, nil, desKeys{backspace: true})
	}
	if s.handleKeys(a, nil, desKeys{enter: true}) || s.box == nil || s.box.err == "" || p.col.Project != nil {
		t.Errorf("empty name: box %+v", s.box)
	}
	s.handleKeys(a, []rune("A"), desKeys{})
	if !clickR(a, s, yes) || p.col.Project == nil {
		t.Fatalf("not built: %+v", s.box)
	}
	// A second one: the colony is busy, BuildShip refuses in the box.
	s.handleKeys(a, nil, desKeys{enter: true})
	d, _ := s.t.Realise(a.e.Research)
	d.Name = s.box.name
	want := a.e.BuildShip(p.col, d)
	if s.handleKeys(a, nil, desKeys{enter: true}) || s.box == nil || want == nil || s.box.err != want.Error() {
		t.Errorf("busy: box %+v, want %v", s.box, want)
	}
	s.box, s.col = nil, nil
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box != nil {
		t.Error("a build box without a colony")
	}
	if hoverR(a, s, desBuild); s.hover == hoverBuild || clickR(a, s, desBuild) || s.box != nil {
		t.Error("a Build button without a colony")
	}
}

// The screen edits a clone: its edits never reach the template it was
// opened on, even with spare capacity in that slice.
func TestDesignScreenCopies(t *testing.T) {
	a, _ := designApp(t)
	src := game.Template{Name: "T", Hull: game.Small, Parts: make([]game.Part, 2, 5)}
	src.Parts[0] = game.Part{Best: true, Cat: game.Generator}
	src.Parts[1] = game.Part{Best: true, Cat: game.Drive}
	s := newDesignScreenFor(nil, src)
	a.push(s)
	s.add(a.e.Research, fixed(game.Colonizer)[0])
	s.remove(a.e.Research, 0)
	if !src.Parts[0].Best || src.Parts[0].Cat != game.Generator || src.Parts[:3][2] != (game.Part{}) {
		t.Errorf("source trashed: %v", src.Parts[:3])
	}
	if s.t.Name != "T" {
		t.Errorf("Enter renamed a named template: %q", s.t.Name)
	}
}

// The title box: the name with its cursor only in the text mode and the
// hull, the realised design's cost, power and reach, the slots, the keys;
// the verdict: Valid's.
func TestTitleLines(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	s.add(r, game.Part{Best: true, Cat: game.Generator})
	s.add(r, game.Part{Best: true, Cat: game.Drive})
	l, v, ok := s.titleLines(a.e)
	sup, dem := s.d.Power()
	if l[0] != s.t.Name+` "Small"` || s.t.Name != defaultName(a) || !ok || v != "valid" ||
		l[1] != "cost "+itoa(s.d.Cost()) || l[2] != "power "+itoa(sup)+", demand "+itoa(dem) ||
		l[3] != "slots 2/"+itoa(game.Small.Slots())+", reach "+itoa(game.Gizmos[game.TonklinMotor].Level)+"/move" {
		t.Errorf("%q %q %v", l, v, ok)
	}
	s.editing = true
	if l, _, _ := s.titleLines(a.e); l[0] != s.t.Name+`_ "Small"` {
		t.Errorf("editing: %q", l[0])
	}
	s.editing = false
	s.remove(r, 0) // no generator: Valid's refusal
	if _, v, ok := s.titleLines(a.e); ok || v != s.d.Valid(r).Error() {
		t.Errorf("unpowered: %q %v", v, ok)
	}
}

// The part box's facts by category, industry last.
func TestPartFacts(t *testing.T) {
	m := game.Gizmos[game.TonklinMotor]
	if f := partFacts(game.TonklinMotor); len(f) != 2 || f[0] != "reach "+itoa(m.Level)+" cubes  power "+itoa(m.Power) ||
		f[1] != "industry "+itoa(m.Industry) {
		t.Errorf("drive %q", f)
	}
	if f := partFacts(game.StarLaneDrive); len(f) == 0 || !strings.HasPrefix(f[len(f)-1], "industry ") {
		t.Errorf("special %q", f)
	}
	w := float64(desPart.X1 - desPart.X0 - 2*padX)
	for g := range game.NumGizmos {
		for _, l := range partFacts(g) {
			if fitWidth(l, w, smallFace) != l {
				t.Errorf("%v: %q is wider than the part box", g, l)
			}
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// A hull switch after a load is another class: the build box goes back
// to the hull's name; a load after it restores the template's.
func TestDesignBuildNameHullSwitch(t *testing.T) {
	a, _, s := yardDesign(t)
	r := a.e.Research
	saveTemplates(t, a.e)
	s.load(r, a.e.Templates[1])
	s.cycleHull(r)
	if s.t.Hull == a.e.Templates[1].Hull {
		t.Fatal("fixture: one hull only")
	}
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box == nil || s.box.class != s.t.Hull.String() {
		t.Errorf("after load then hull switch: %+v, hull %v", s.box, s.t.Hull)
	}
	s.cancel()
	s.load(r, a.e.Templates[1])
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box == nil || s.box.class != a.e.Templates[1].Name {
		t.Errorf("after a reload: %+v", s.box)
	}
}

// The save box warns while its name is a saved template's, and saving
// still replaces it; the build box never warns.
func TestDesignSaveBoxClash(t *testing.T) {
	a, _, s := yardDesign(t)
	r := a.e.Research
	saveTemplates(t, a.e)
	n := len(a.e.Templates)
	s.load(r, a.e.Templates[1])
	s.handleKeys(a, nil, desKeys{s: true})
	if s.box == nil || !s.box.clash(a.e) {
		t.Fatalf("no clash on a loaded template's name: %+v", s.box)
	}
	s.handleKeys(a, []rune("x"), desKeys{})
	if s.box.clash(a.e) {
		t.Errorf("clash on %q", s.box.name)
	}
	s.handleKeys(a, nil, desKeys{backspace: true})
	s.add(r, game.Part{Best: true, Cat: game.Generator})
	if s.handleKeys(a, nil, desKeys{enter: true}); s.box != nil || len(a.e.Templates) != n || len(a.e.Templates[1].Parts) != len(s.t.Parts) {
		t.Errorf("not replaced: box %+v, templates %v", s.box, a.e.Templates)
	}
	s.handleKeys(a, nil, desKeys{enter: true})
	if s.box == nil || s.box.mode != nameBuild || s.box.clash(a.e) {
		t.Errorf("build box: %+v", s.box)
	}
}

// The five gauges sum the design's parts per category (damage, strength,
// Level, supply) on the fixed desGaugeFull scale graduated every 4 (the
// original's icon), past 1 beyond it (five of the best generators); they
// sit in the title box's right column, above the buttons.
func TestDesignBars(t *testing.T) {
	cats := []game.GizmoCat{game.Weapon, game.Shield, game.Drive, game.Scanner, game.Generator}
	val := func(gz game.Gizmo) int {
		if game.Gizmos[gz].Cat == game.Generator {
			return game.Gizmos[gz].Power
		}
		return game.Gizmos[gz].Level
	}
	for _, d := range []game.Design{game.FixtureRaider, game.FixtureCruiser,
		{Hull: game.Enormous, Parts: slices.Repeat([]game.Gizmo{game.Nanotwirler}, 5)}, {Hull: game.Small}} {
		for k, b := range designBars(d) {
			want := 0
			for _, gz := range d.Parts {
				if game.Gizmos[gz].Cat == cats[k] {
					want += val(gz)
				}
			}
			if b.num != fmt.Sprint(want) || b.frac != float64(want)/desGaugeFull || b.tick != 4.0/desGaugeFull {
				t.Errorf("%s gauge %s: %s %.3f, want %d", d.Name, b.label, b.num, b.frac, want)
			}
		}
	}
	if four := 4 * game.Gizmos[game.Nanotwirler].Power; four != desGaugeFull {
		t.Errorf("four Nanotwirlers supply %d, the scale is %d", four, desGaugeFull)
	}
	if b := designBars(game.Design{Hull: game.Enormous, Parts: slices.Repeat([]game.Gizmo{game.Nanotwirler}, 5)})[4]; b.frac <= 1 {
		t.Errorf("five Nanotwirlers: %.3f, want past 1", b.frac)
	}
	// Layout: inside the box, right of the text column, above the buttons;
	// a broken gauge's number still inside the box.
	if desGaugeX() < float64(desTitle.X0+padX) || desGaugeX()+desGaugeW+barBreak > float64(desTitle.X1) ||
		desGaugeY(0) < float64(desTitle.Y0) || desGaugeY(4)+8*zoom > float64(desLoad.Y0) {
		t.Errorf("gauges at x %.0f, y %.0f..%.0f; title %+v, buttons at %d", desGaugeX(), desGaugeY(0), desGaugeY(4), desTitle, desLoad.Y0)
	}
}

// The view toggles: Detailed lays one entry a row, its whole width
// answering, detRows shown from listOff; the next design screen opens
// detailed too; a click on a row adds its part; a category switch goes
// back to the top; Compact brings the grid back.
func TestDesignDetailedList(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	for tc := range game.NumTechs {
		r.Grant(tc)
	}
	s.cat = game.Special
	n := len(s.list(r))
	if n <= detRows {
		t.Fatalf("fixture: %d Specials, want more than %d", n, detRows)
	}
	if clickR(a, s, desDetailed); !s.detailed || !a.desDetailed {
		t.Fatalf("Detailed: detailed %v, app %v", s.detailed, a.desDetailed)
	}
	b := newDesignScreen()
	a.push(b)
	if !b.detailed {
		t.Error("the next screen opens compact")
	}
	a.pop()
	row := func(k int) rect { _, hit, _ := s.entryCell(k + s.listOff); return hit }
	for k := range detRows {
		hit := row(k)
		if hit.Y1 > desList.Y1 {
			t.Errorf("row %d %+v leaves the list", k, hit)
		}
		for _, x := range []float64{float64(hit.X0 + 1), float64(hit.X1 - 1)} {
			if i, ok := s.listAt(r, x, float64(hit.Y0+hit.Y1)/2); !ok || i != k {
				t.Errorf("row %d at x %v: entry %d %v", k, x, i, ok)
			}
		}
	}
	past := row(detRows)
	if _, ok := s.listAt(r, float64(past.X0+1), float64(past.Y0+1)); ok {
		t.Error("the row past the view answers")
	}
	s.listOff = n // clamped to the last screenful
	s.listScroll(a, r)
	if s.listOff != n-detRows {
		t.Errorf("listOff %d, want %d", s.listOff, n-detRows)
	}
	if i, ok := s.listAt(r, float64(row(0).X0+1), float64(row(0).Y0+1)); !ok || i != n-detRows {
		t.Errorf("scrolled: row 0 is entry %d %v, want %d", i, ok, n-detRows)
	}
	clickR(a, s, row(detRows-1))
	sx, sy := desSlotCentre(s.t.Hull, 0)
	s.leftClick(a, sx, sy)
	if len(s.t.Parts) != 1 || s.t.Parts[0] != s.list(r)[n-1] {
		t.Errorf("a row then a slot placed %v", s.t.Parts)
	}
	clickR(a, s, catHoles[0].r)
	if s.cat != catHoles[0].cat || s.listOff != 0 {
		t.Errorf("category switch: cat %v, listOff %d", s.cat, s.listOff)
	}
	if clickR(a, s, desCompact); s.detailed || a.desDetailed {
		t.Error("Compact left the detailed view")
	}
}

// Placed by hand, a part keeps its slot: emptying one leaves a hole, a
// part dropped on a full slot replaces it, a smaller hull drops the parts
// past its slots only, the saved template and the built ship keep the
// layout; Esc and a right click off the slots drop the held part.
func TestDesignHoles(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	r.Grant(game.LargeScaleConstruction) // Enormous
	drives := s.list(r)
	click := func(x, y float64) { s.leftClick(a, x, y) }
	right := func(x, y float64) { s.rightClick(r, x, y) }
	s.t.Hull = game.Enormous
	put := func(p game.Part, k int) {
		click(cellCentre(slices.Index(drives, p)))
		click(desSlotCentre(s.t.Hull, k))
		s.holding = false
	}
	put(drives[1], 6)
	put(drives[0], 1)
	put(drives[1], 3)
	if want := []int{1, 3, 6}; !slices.Equal(s.t.Slots, want) || s.t.Parts[0] != drives[0] {
		t.Fatalf("slots %v parts %v, want %v", s.t.Slots, s.t.Parts, want)
	}
	right(desSlotCentre(s.t.Hull, 3))
	if want := []int{1, 6}; !slices.Equal(s.t.Slots, want) {
		t.Errorf("after emptying 3: slots %v, want %v", s.t.Slots, want)
	}
	put(drives[1], 1)
	if s.t.Parts[0] != drives[1] || len(s.t.Parts) != 2 {
		t.Errorf("a drop on slot 1 left %v", s.t.Parts)
	}
	click(cellCentre(0))
	right(centre(desPicture))
	if s.holding {
		t.Error("a right click off the slots kept the part")
	}
	click(cellCentre(0))
	if !s.cancel() || s.holding {
		t.Error("Esc kept the part")
	}
	if err := a.e.AddTemplate(s.t); err != nil {
		t.Fatal(err)
	}
	if got := a.e.Templates[len(a.e.Templates)-1]; !slices.Equal(got.Slots, []int{1, 6}) {
		t.Errorf("saved slots %v", got.Slots)
	}
	if d, _ := s.t.Realise(r); !slices.Equal(d.Slots, []int{1, 6}) || !slices.Equal(d.Fixed().Slots, d.Slots) {
		t.Errorf("realised slots %v", d.Slots)
	}
	s.t = s.t.FitHull(game.Small) // 5 slots: slot 6 goes
	if !slices.Equal(s.t.Slots, []int{1}) || len(s.t.Parts) != 1 {
		t.Errorf("on a Small hull: slots %v parts %v", s.t.Slots, s.t.Parts)
	}
}

// The part box describes the held part, a hovered one changing nothing;
// dropped, the box is empty again.
func TestDesignShowsHeld(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	l := s.list(r)
	click := func(x, y float64) { s.leftClick(a, x, y) }
	hover := func(x, y float64) { s.hoverAt(a, x, y) }
	click(cellCentre(1))
	hover(centre(desPicture))
	if g, ok := s.shown(r); !ok || g != l[1].Gizmo {
		t.Errorf("held: shown %v %v, want %v", g, ok, l[1].Gizmo)
	}
	s.cat = game.Generator // another list, the drive still held
	gen := s.list(r)[len(s.list(r))-1].Gizmo
	hover(cellCentre(len(s.list(r)) - 1))
	if g, ok := s.shown(r); !ok || g != l[1].Gizmo || g == gen {
		t.Errorf("hovered: shown %v %v, want the held %v", g, ok, l[1].Gizmo)
	}
	s.cancel()
	hover(centre(desPicture))
	if _, ok := s.shown(r); ok {
		t.Error("dropped: the box still describes a part")
	}
}

// Best is a template's: the template screen lists "Best <cat>", a screen
// building at a colony does not, and a template loaded there comes in
// with today's gizmos in the Best parts' slots, its name kept.
func TestBestTemplatesOnly(t *testing.T) {
	a, s := designApp(t)
	r := a.e.Research
	if l := s.list(r); len(l) == 0 || !l[0].Best {
		t.Fatalf("template screen: %v", l)
	}
	b := newDesignScreenFor(a.e.Colonies[0], freshTemplate())
	a.push(b)
	if l := b.list(r); slices.ContainsFunc(l, func(p game.Part) bool { return p.Best }) {
		t.Errorf("build screen lists %v", l)
	}
	tp := game.Template{Name: "Scout", Hull: game.Small, Parts: []game.Part{{Best: true, Cat: game.Drive}}, Slots: []int{3}}
	b.load(r, tp)
	d, _ := tp.Realise(r)
	if b.t.Name != "Scout" || b.loaded != "Scout" || !slices.Equal(b.t.Parts, d.Fixed().Parts) || !slices.Equal(b.t.Slots, []int{3}) || b.t.Parts[0].Best {
		t.Errorf("loaded on the build screen: %+v", b.t)
	}
}
