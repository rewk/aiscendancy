package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
	"github.com/rewk/aiscendancy/ui"
)

// slotsApp is seed 1 (two empires) on its starmap, saving beside a fresh
// quick save path (none written yet).
func slotsApp(t *testing.T) *app {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	a.savePath = filepath.Join(t.TempDir(), "quick.json")
	a.open([]screen{newStarmapScreen()})
	return a
}

// openSlotsOn pushes the slots screen in mode saving (Enter peeks).
func openSlotsOn(a *app, saving bool) *slotsScreen {
	s := newSlotsScreen(saving)
	a.push(s)
	return s
}

// peekDay is the day path's summary says, −1 when it does not peek.
func peekDay(t *testing.T, path string) int {
	t.Helper()
	v, sum, err := save.Peek(path)
	if err != nil || v != save.Version || sum == nil {
		return -1
	}
	return sum.Day
}

// Every part of a row lies in the big area (the right margin left to the
// scroll cue) and in its row's band, no two overlapping; the rows fit;
// the longest texts fit their columns; a row holds every other empire.
func TestSlotsLayout(t *testing.T) {
	if n := svRowH * slotsRows(); n > mapArea.Y1-mapArea.Y0-2*padY || slotsRows() < 2 {
		t.Fatalf("%d rows of %d px in %d", slotsRows(), svRowH, mapArea.Y1-mapArea.Y0)
	}
	lh := int(bigFace.Size * 1.25)
	for r := range slotsRows() {
		y0 := svY0(r)
		parts := map[string]rect{
			"num":  {svNumX(), y0 + svPad, svFlagX(), y0 + svPad + lh},
			"flag": {svFlagX(), y0 + svPad, svFlagX() + flagW*zoom, y0 + svPad + flagH*zoom},
			"text": {svTextX(), y0 + svPad, svTextX() + svTextW, y0 + svPad + 4*lh},
			"ring": {svRingX(), y0 + (svRowH-svRingW)/2, svRingX() + svRingW, y0 + (svRowH+svRingW)/2},
		}
		for k := range 3 {
			parts[fmt.Sprintf("block %d", k)] = rect{svBlockX(k), y0 + svPad, svBlockX(k) + svBlockW,
				y0 + svIconDY() + svIconLines()*svPitch}
		}
		for k := range svMetMax {
			parts[fmt.Sprintf("met %d", k)] = svMetCell(y0, k)
		}
		for n, p := range parts {
			if p.X0 < mapArea.X0+padX || p.X1 > mapArea.X1-padX || p.Y0 < y0 || p.Y1 > y0+svRowH || p.Y1 > mapArea.Y1 {
				t.Errorf("row %d %s %+v outside the row [%d,%d) or the area %+v", r, n, p, y0, y0+svRowH, mapArea)
			}
			for m, q := range parts {
				if n < m && p.X0 < q.X1 && q.X0 < p.X1 && p.Y0 < q.Y1 && q.Y0 < p.Y1 {
					t.Errorf("row %d: %s %+v overlaps %s %+v", r, n, p, m, q)
				}
			}
		}
		if got := svRowAt(float64(y0 + svRowH/2)); got != r {
			t.Errorf("row %d's middle is row %d", r, got)
		}
	}
	if svMetMax < maxEmpires-1 {
		t.Errorf("a row shows %d met empires, a game has up to %d others", svMetMax, maxEmpires-1)
	}
	if svIconLines()*svPerLine < 100 {
		t.Errorf("%d icons a block", svIconLines()*svPerLine)
	}
	if w := ui.Width("Quick save  F5 / F9", smallFace); w > svTextW {
		t.Errorf("the quick label: %.0f px in %d", w, svTextW)
	}
	texts := []string{"2026-12-31 23:59", "Day 99999"}
	for sp := range game.NumSpecies {
		texts = append(texts, sp.String())
	}
	for _, s := range texts {
		if w := ui.Width(s, bigFace); w > svTextW {
			t.Errorf("%q: %.0f px in %d", s, w, svTextW)
		}
	}
	for k, n := range []int{999, 999, int(game.NumTechs)} { // ships, colonies, techs
		if w := ui.Width(fmt.Sprintf("%s %d", svLabels[k], n), smallFace); w > svBlockW {
			t.Errorf("block %d label: %.0f px in %d", k, w, svBlockW)
		}
	}
	// The bursts and the laurel (origins at the flag's corner) stay in a
	// met cell, measured on the game's sprites.
	as, err := assets.Open(cob.DefaultDir())
	if err != nil {
		t.Skip(err)
	}
	defer as.Close()
	c := svMetCell(svY0(0), 0)
	fx, fy := c.X0+(svMetW-flagW*zoom)/2, c.Y0
	for _, f := range []int{intelWar, intelAlly} {
		sp, err := as.Sprite(intelBank, f)
		if err != nil {
			t.Fatal(err)
		}
		b := sp.Image.Bounds()
		x0, y0 := fx+zoom*sp.Offset.X, fy+zoom*sp.Offset.Y
		if x0 < c.X0 || x0+zoom*b.Dx() > c.X1 || y0 < c.Y0 || y0+zoom*b.Dy() > c.Y1 {
			t.Errorf("frame %d at (%d,%d) %dx%d: out of the cell %+v", f, x0, y0, zoom*b.Dx(), zoom*b.Dy(), c)
		}
	}
}

// Save mode: New save writes the next slot and closes; an occupied slot
// asks first (no keeps the file, yes overwrites and closes); the quick
// row writes at once; Esc (cancel) drops the question.
func TestSlotsSave(t *testing.T) {
	a := slotsApp(t)
	s := openSlotsOn(a, true)
	ns, _ := save.Slots(a.savePath)
	first := save.SlotPath(a.savePath, save.NextSlot(ns))
	if row, ok := s.row(1); !ok || !row.fresh || row.path != first || s.quick.exists {
		t.Fatalf("row 1: %+v, quick %+v", row, s.quick)
	}
	s.press(a, 1)
	if got := peekDay(t, first); got != a.e.Day || a.top() == screen(s) || a.toastErr {
		t.Fatalf("New save: day %d (want %d), top %T, toast %q", got, a.e.Day, a.top(), a.toast)
	}
	saved := a.e.Day
	a.g.EndDay()
	if a.e.Day == saved {
		t.Fatal("fixture: the day did not move")
	}

	s = openSlotsOn(a, true)
	ns, _ = save.Slots(a.savePath)
	if row, ok := s.row(2); len(s.slots) != 1 || !ok || !row.fresh || row.n != save.NextSlot(ns) {
		t.Fatalf("slots %+v, row 2 %+v", s.slots, row)
	}
	s.press(a, 1)
	if s.confirm == nil || s.confirm.lines[0] != fmt.Sprintf("Overwrite save %d?", s.slots[0].n) {
		t.Fatalf("no question: %+v", s.confirm)
	}
	if !s.cancel() || s.confirm != nil || s.cancel() {
		t.Error("cancel did not drop the question once")
	}
	s.press(a, 1)
	s.answer(a, false)
	if got := peekDay(t, first); got != saved || a.top() != screen(s) || s.confirm != nil {
		t.Fatalf("no: day %d (want %d), top %T", got, saved, a.top())
	}
	s.press(a, 1)
	s.answer(a, true)
	if got := peekDay(t, first); got != a.e.Day || a.top() == screen(s) {
		t.Fatalf("yes: day %d (want %d), top %T", got, a.e.Day, a.top())
	}

	s = openSlotsOn(a, true)
	s.press(a, 0)
	if got := peekDay(t, a.savePath); got != a.e.Day || s.confirm != nil || a.top() == screen(s) {
		t.Fatalf("quick: day %d, question %v, top %T", got, s.confirm, a.top())
	}
	s = openSlotsOn(a, true)
	if !s.quick.exists || s.quick.sum == nil {
		t.Errorf("quick row after a save: %+v", s.quick)
	}
	s.press(a, 0) // occupied, still at once
	if s.confirm != nil || a.top() == screen(s) {
		t.Errorf("quick over a save asked: %+v", s.confirm)
	}
}

// slotsFiles writes slots 1 and 2 (games on two seeds), 3 of the version
// before, 4 broken, beside a.savePath; it returns the two games.
func slotsFiles(t *testing.T, a *app) (g1, g2 *game.Game) {
	t.Helper()
	g1 = game.NewGame(2, []game.Species{game.Orfa, game.Mebes}, game.Average)
	g2 = game.NewGame(3, []game.Species{game.Minions, game.Orfa, game.Mebes}, game.Average)
	g2.EndDay()
	for n, g := range map[int]*game.Game{1: g1, 2: g2} {
		if err := save.Write(save.SlotPath(a.savePath, n), &save.File{Seed: g.Seed, Game: g}); err != nil {
			t.Fatal(err)
		}
	}
	b := gunzipped(t, save.SlotPath(a.savePath, 1))
	old := bytes.Replace(b, fmt.Appendf(nil, `"Version":%d,`, save.Version), fmt.Appendf(nil, `"Version":%d,`, save.Version-1), 1)
	gzipTo(t, save.SlotPath(a.savePath, 3), old)
	gzipTo(t, save.SlotPath(a.savePath, 4), []byte(`{"Vers`))
	return g1, g2
}

// Load mode: a loadable slot replaces the game on a fresh starmap (the
// screen gone with the stack); the refused rows, the broken one and the
// empty quick row do nothing; the rows read as Peek says, slot k at row
// k (the quick row pinned at 0).
func TestSlotsLoad(t *testing.T) {
	a := slotsApp(t)
	_, g2 := slotsFiles(t, a)
	old := a.g
	s := openSlotsOn(a, false)
	if len(s.slots) != 4 || s.entries() != 4 {
		t.Fatalf("%d slots, %d entries", len(s.slots), s.entries())
	}
	for r := 1; r <= 4; r++ {
		row, ok := s.row(r)
		wantOK := r <= 2
		if !ok || row.n != r || (row.sum != nil) != wantOK || (row.why == "") != wantOK {
			t.Errorf("row %d: %+v", r, row)
		}
	}
	if row, _ := s.row(3); row.why != fmt.Sprintf("version %d, not loadable", save.Version-1) {
		t.Errorf("row 3: %q", row.why)
	}
	for _, r := range []int{0, 3, 4, -1, slotsRows()} {
		s.press(a, r)
		if a.g != old || a.top() != screen(s) || s.confirm != nil || a.toast != "" {
			t.Fatalf("row %d: loaded, tried (%q) or asked (top %T)", r, a.toast, a.top())
		}
	}
	s.press(a, 2)
	if a.g == old || a.g.Seed != g2.Seed || a.e.Day != g2.Day || len(a.g.Empires) != len(g2.Empires) {
		t.Fatalf("loaded seed %d day %d, want %d day %d", a.g.Seed, a.e.Day, g2.Seed, g2.Day)
	}
	if len(a.stack) != 1 || fmt.Sprintf("%T", a.top()) != "*main.starmapScreen" {
		t.Errorf("stack after the load: %d, top %T", len(a.stack), a.top())
	}
	// Save mode: a refused row is no load but may be overwritten.
	s = openSlotsOn(a, true)
	s.press(a, 3)
	if s.confirm == nil {
		t.Fatal("refused row: no overwrite question")
	}
	s.answer(a, true)
	if peekDay(t, save.SlotPath(a.savePath, 3)) != a.e.Day {
		t.Error("refused row not overwritten")
	}
}

// Before a game (the New Game screen): the quit prompt's Load... opens the
// screen, Save... does not; a load starts the game on its starmap.
func TestSlotsNoGame(t *testing.T) {
	dir := t.TempDir()
	a := &app{savePath: filepath.Join(dir, "quick.json")}
	_, g2 := slotsFiles(t, a)
	a.open([]screen{newNewGameScreen()})
	sv := openSlotsOn(a, true) // unreachable by the mouse: no game, nothing written
	sv.press(a, 0)
	sv.press(a, 1)
	if _, err := os.Stat(a.savePath); err == nil || sv.confirm != nil || a.top() != screen(sv) {
		t.Fatalf("saved without a game (question %v)", sv.confirm)
	}
	a.pop()
	a.quitting = true
	if !a.openSlots(true, false) || !a.quitting || len(a.stack) != 1 {
		t.Fatalf("Save... without a game: quitting %v, %d screens", a.quitting, len(a.stack))
	}
	if a.openSlots(false, false) {
		t.Error("nothing pressed: answered")
	}
	if !a.openSlots(false, true) || a.quitting {
		t.Fatal("Load... did not close the prompt")
	}
	s := top[*slotsScreen](t, a)
	if s.saving || s.entries() != 4 {
		t.Fatalf("saving %v, %d entries", s.saving, s.entries())
	}
	s.press(a, 0) // no quick save, and no game to write
	if a.inGame() {
		t.Fatal("the empty quick row loaded")
	}
	s.press(a, 2)
	if !a.inGame() || a.g.Seed != g2.Seed || len(a.stack) != 1 {
		t.Fatalf("in game %v, %d screens", a.inGame(), len(a.stack))
	}
	top[*starmapScreen](t, a)
	if !a.openSlots(true, false) || !top[*slotsScreen](t, a).saving {
		t.Error("Save... in a game")
	}
}

// The scroll: the quick row stays at row 0, row 1 shows entry off; the
// offset stops at the last page and clamps back when the list shrinks.
func TestSlotsScroll(t *testing.T) {
	a := slotsApp(t)
	for n := 1; n <= 7; n++ {
		if err := save.Write(save.SlotPath(a.savePath, n), &save.File{Game: a.g}); err != nil {
			t.Fatal(err)
		}
	}
	s := openSlotsOn(a, true)
	page := slotsRows() - 1
	if s.entries() != 8 || page >= s.entries() {
		t.Fatalf("%d entries, %d a page: nothing scrolls", s.entries(), page)
	}
	s.scroll(100)
	if s.off != s.entries()-page {
		t.Errorf("off %d, want %d", s.off, s.entries()-page)
	}
	if row, _ := s.row(0); row.n != 0 {
		t.Errorf("row 0 scrolled: %+v", row)
	}
	if row, _ := s.row(1); row.n != s.slots[s.off].n {
		t.Errorf("row 1 is slot %d, want %d", row.n, s.slots[s.off].n)
	}
	if row, _ := s.row(page); !row.fresh {
		t.Errorf("last row %+v, want New save", row)
	}
	s.slots = s.slots[:2]
	s.scroll(0)
	if s.off != 0 {
		t.Errorf("shrunk to %d entries: off %d", s.entries(), s.off)
	}
	s.scroll(-5)
	if s.off != 0 {
		t.Errorf("off %d above the top", s.off)
	}
}

// The quit prompt's six buttons: in the box, under the title, over the
// answers, none overlapping; the key help reads the slots screen as a
// list, its question as a popup.
func TestQuitSlotsButtons(t *testing.T) {
	title := float64(quitBox.Y0+dlgTitleY) + bigFace.Size*1.25
	bs := map[string]rect{"save": quitSave, "load": quitLoad, "saveAs": quitSaveAs, "loadAs": quitLoadAs, "shot": quitShot, "settings": quitSet}
	for n, r := range bs {
		if r.X0 < quitBox.X0 || r.X1 > quitBox.X1 || float64(r.Y0) < title || r.Y1 > dlgBar.Y0 {
			t.Errorf("%s %+v: box %+v, title ends %.0f, bar at %d", n, r, quitBox, title, dlgBar.Y0)
		}
		for m, q := range bs {
			if n < m && r.X0 < q.X1 && q.X0 < r.X1 && r.Y0 < q.Y1 && q.Y0 < r.Y1 {
				t.Errorf("%s %+v overlaps %s %+v", n, r, m, q)
			}
		}
	}
	a := slotsApp(t)
	s := openSlotsOn(a, true)
	if a.keysSection() != keysLists {
		t.Errorf("section %q", a.keysSection())
	}
	s.confirm = &prompt{lines: []string{"x"}}
	if a.keysSection() != keysPopups {
		t.Errorf("question: section %q", a.keysSection())
	}
}

// The right column: the title on top, the map's square under it, the
// small graphs under the map in the panel and clear of the "?" button,
// none overlapping; each graph's label with a long value fits.
func TestSlotsPreviewLayout(t *testing.T) {
	parts := map[string]rect{"title": svTitle, "map": svMap()}
	for k := range statSeries {
		parts[statSeries[k].name] = svGraph(k)
		if p := svPlot(k); p.Y1-p.Y0 < 40 {
			t.Errorf("graph %d: a %d px plot", k, p.Y1-p.Y0)
		}
		if w := ui.Width(fmt.Sprintf("%s %d", statSeries[k].name, 999999), smallFace); w > float64(svGraph(k).X1-svGraph(k).X0) {
			t.Errorf("graph %d label: %.0f px in %d", k, w, svGraph(k).X1-svGraph(k).X0)
		}
	}
	for n, p := range parts {
		if p.X0 < svPanel.X0 || p.X1 > svPanel.X1 || p.Y0 < svPanel.Y0 || p.Y1 > svPanel.Y1 {
			t.Errorf("%s %+v outside the panel %+v", n, p, svPanel)
		}
		if p.X0 < keysButton.X1 && keysButton.X0 < p.X1 && p.Y0 < keysButton.Y1 && keysButton.Y0 < p.Y1 {
			t.Errorf("%s %+v under the ? button", n, p)
		}
		for m, q := range parts {
			if n < m && p.X0 < q.X1 && q.X0 < p.X1 && p.Y0 < q.Y1 && q.Y0 < p.Y1 {
				t.Errorf("%s %+v overlaps %s %+v", n, p, m, q)
			}
		}
	}
	if svPanel.X0 < mapArea.X1 {
		t.Errorf("the panel %+v over the big area %+v", svPanel, mapArea)
	}
}

// countReads counts readSave's calls for the test.
func countReads(t *testing.T) *int {
	t.Helper()
	n, saved := new(int), readSave
	t.Cleanup(func() { readSave = saved })
	readSave = func(path string) (*save.File, error) {
		*n++
		return saved(path)
	}
	return n
}

// hoverRow hovers screen row r, as Update does.
func hoverRow(s *slotsScreen, r int) { s.hoverAt(float64(mapArea.X0+50), float64(svY0(r)+10)) }

// A hovered loadable row is read once whatever the frames, its preview
// the player's History; a refused, broken or empty row and no row read
// nothing and preview nothing; Enter drops the previews (read again).
func TestSlotsPreviewCache(t *testing.T) {
	a := slotsApp(t)
	_, g2 := slotsFiles(t, a)
	reads := countReads(t)
	s := openSlotsOn(a, false)
	for range 3 {
		hoverRow(s, 2)
	}
	row, p, ok := s.hovered()
	if *reads != 1 || !ok || row.n != 2 || p == nil || p.why != "" {
		t.Fatalf("%d reads, row %+v, preview %+v", *reads, row, p)
	}
	if h := g2.Empires[0].History; len(h) == 0 || len(p.hist) != len(h) || p.hist[0] != h[0] || p.colour != g2.Empires[0].Colour {
		t.Errorf("preview History %v colour %d, want %v %d", p.hist, p.colour, h, g2.Empires[0].Colour)
	}
	for _, r := range []int{0, 3, 4} { // the empty quick row, the old version, the broken file
		hoverRow(s, r)
		if row, p, ok := s.hovered(); !ok || row.n != r || p != nil || *reads != 1 {
			t.Errorf("row %d: %+v, preview %+v, %d reads", r, row, p, *reads)
		}
	}
	s.hoverAt(float64(mapArea.X1+10), float64(svY0(2)+10)) // right of the area
	if _, p, ok := s.hovered(); ok || p != nil || s.hover != -1 {
		t.Errorf("outside: hover %d, preview %+v", s.hover, p)
	}
	hoverRow(s, 1)
	hoverRow(s, 2)
	if *reads != 2 || len(s.previews) != 2 {
		t.Errorf("rows 1 then 2 again: %d reads, %d previews", *reads, len(s.previews))
	}
	s.Enter(a)
	if len(s.previews) != 0 {
		t.Fatalf("Enter kept %d previews", len(s.previews))
	}
	hoverRow(s, 2)
	if *reads != 3 {
		t.Errorf("after Enter: %d reads", *reads)
	}
}

// The preview's map is the saved player's knowledge: exactly the stars it
// knows (Empire.Knows), the lanes joining two of them, a settled star in
// its colonist's colour; the cluster holds unknown stars, and they are
// not there.
func TestSlotsPreviewFog(t *testing.T) {
	a := slotsApp(t)
	e := a.e
	home := e.Colonies[0].Star
	for _, li := range a.cl.Stars[home].Lanes[:1] { // one neighbour learnt
		e.Visit(a.cl.Other(li, home))
	}
	for range 2 * game.HistoryEvery {
		a.g.EndDay()
	}
	path := save.SlotPath(a.savePath, 1)
	if err := save.Write(path, &save.File{Game: a.g}); err != nil {
		t.Fatal(err)
	}
	s := openSlotsOn(a, false)
	hoverRow(s, 1)
	_, p, _ := s.hovered()
	if p == nil || p.why != "" {
		t.Fatalf("preview %+v", p)
	}
	var known []int
	for i := range a.cl.Stars {
		if e.Knows(i) {
			known = append(known, i)
		}
	}
	if len(known) < 2 || len(known) == len(a.cl.Stars) {
		t.Fatalf("fixture: %d of %d stars known", len(known), len(a.cl.Stars))
	}
	var shown []int
	for _, st := range p.stars {
		shown = append(shown, st.star)
		if col := a.cl.Stars[st.star].System.Colonist(); st.settled != (col >= 0) || st.settled && st.colour != empireColour(colourOf(a.g, col)) {
			t.Errorf("star %d: settled %v colour %v, colonist %d", st.star, st.settled, st.colour, col)
		}
		for _, tick := range []int{0, svTurnTicks / 8, svTurnTicks / 3} {
			c := svCamera(svMap(), tick)
			if x, y := svAt(&c, st); !svMap().has(float64(x), float64(y)) {
				t.Errorf("star %d at (%v, %v) on tick %d, off the map", st.star, x, y, tick)
			}
		}
	}
	if !slices.Equal(shown, known) {
		t.Errorf("stars shown %v, known %v", shown, known)
	}
	var joined [][2]int // the cluster's lanes between two known stars
	for _, l := range a.cl.Lanes {
		if e.Knows(l.A) && e.Knows(l.B) {
			joined = append(joined, [2]int{l.A, l.B})
		}
	}
	var drawn [][2]int
	for _, l := range p.lanes {
		drawn = append(drawn, [2]int{p.stars[l.a].star, p.stars[l.b].star})
	}
	if len(joined) == 0 || !slices.Equal(drawn, joined) {
		t.Errorf("lanes drawn %v, between known stars %v", drawn, joined)
	}
}

// F5 under the slots screen rewrites the quick save: its row shows the
// new summary and its preview is read again, not the file Enter peeked.
func TestSlotsQuickSavedUnder(t *testing.T) {
	a := slotsApp(t)
	a.save() // day 0
	s := openSlotsOn(a, false)
	hoverRow(s, 0)
	if s.quick.sum == nil || s.quick.sum.Day != 0 || s.previews[s.quick.path] == nil {
		t.Fatalf("quick row %+v, previews %d", s.quick, len(s.previews))
	}
	for range 12 {
		a.g.EndDay()
	}
	pressing(t, ebiten.KeyF5)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if peekDay(t, a.savePath) != a.g.Day {
		t.Fatalf("F5 did not save: file day %d, game day %d", peekDay(t, a.savePath), a.g.Day)
	}
	if s.quick.sum == nil || s.quick.sum.Day != a.g.Day {
		t.Errorf("quick row still day %d after F5 on day %d", s.quick.sum.Day, a.g.Day)
	}
	if p := s.previews[s.quick.path]; p != nil && len(p.hist) != len(a.e.History) {
		t.Errorf("stale preview: %d samples, the game has %d", len(p.hist), len(a.e.History))
	}
}

// While the overwrite question is up it owns the keys: Space ends no
// day, F5 writes no quick save, H opens no Stats.
func TestSlotsQuestionKeys(t *testing.T) {
	a := slotsApp(t)
	s := openSlotsOn(a, true)
	s.press(a, 1) // New save: slot 1 exists
	s = openSlotsOn(a, true)
	s.press(a, 1)
	if s.confirm == nil {
		t.Fatal("fixture: no overwrite question")
	}
	day := a.g.Day
	for _, k := range []ebiten.Key{ebiten.KeySpace, ebiten.KeyF5, ebiten.KeyH} {
		pressing(t, k)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if a.g.Day != day || a.top() != screen(s) || s.confirm == nil {
		t.Errorf("under the question: day %d (was %d), top %T, question %v", a.g.Day, day, a.top(), s.confirm != nil)
	}
	if _, err := os.Stat(a.savePath); err == nil {
		t.Error("F5 wrote the quick save under the question")
	}
}

// A summary naming a species or colour past the banks is refused, not
// drawn (a hand-edited file).
func TestSlotsBadSummary(t *testing.T) {
	a := slotsApp(t)
	a.save()
	b := gunzipped(t, a.savePath)
	bad := strings.Replace(string(b), `"Colour":`+strconv.Itoa(a.e.Colour), `"Colour":99`, 1)
	if bad == string(b) {
		t.Fatal("fixture: no colour to break")
	}
	gzipTo(t, a.savePath, []byte(bad))
	if r := peekRow(0, a.savePath); r.sum != nil || r.why == "" {
		t.Errorf("bad summary peeked as %+v", r)
	}
	if r := peekRow(0, save.SlotPath(a.savePath, 1)); r.exists {
		t.Errorf("missing file: %+v", r)
	}
}

// gunzipped is the save at path's JSON.
func gunzipped(t *testing.T, path string) []byte {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zr, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// gzipTo writes b gzipped to path, as save.Write does.
func gzipTo(t *testing.T, path string, b []byte) {
	t.Helper()
	var z bytes.Buffer
	zw := gzip.NewWriter(&z)
	zw.Write(b)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, z.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
