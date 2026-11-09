package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/help"
	"github.com/rewk/aiscendancy/ui"
)

// helpApp is a test app with the game's assets (the Habitat grafted)
// and the help files parsed apart from the app (help.Parse on the cob
// bytes), our parts' paragraphs after gizhelp.txt's; it skips without
// the game.
func helpApp(t *testing.T) (a *app, items, gizmos []string) {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { as.Close() })
	a = testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	a.a = as
	parse := func(name string) []string {
		b, err := as.Game.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		ps, err := help.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		return ps
	}
	graftHabitat(as)
	gizmos = parse("gizhelp.txt")
	for g := game.NumGameGizmos; g < game.NumGizmos; g++ { // ours: gizmoHelp's own
		gizmos = append(gizmos, ourGizmoHelp[g])
	}
	return a, parse("planhelp.txt"), gizmos
}

// wantLines is text as the spots wrap it: reflow to w's columns.
func wantLines(text string, w float64) []string {
	return reflow(paragraph{false, text}, helpCols(w))
}

// wantLinesIn is wantLines in face's columns.
func wantLinesIn(face *text.GoTextFace, s string, w float64) []string {
	return reflow(paragraph{false, s}, int(w/ui.Width("m", face)))
}

// checkHelp: got is want's head, as many lines as fit from top to
// bottom (and at most max), at least one, each within w; the lines end
// above bottom.
func checkHelp(t *testing.T, what string, got, want []string, top, bottom, w float64, max int) {
	t.Helper()
	n := min(len(want), int((bottom-top)/smallH), max)
	if n < 1 || len(got) != n {
		t.Errorf("%s: %d lines, want %d of %d (top %.0f bottom %.0f)", what, len(got), n, len(want), top, bottom)
		return
	}
	for i, l := range got {
		if l != want[i] {
			t.Errorf("%s line %d: %q, want %q", what, i, l, want[i])
		}
		if ui.Width(l, smallFace) > w {
			t.Errorf("%s line %d: %q wider than %.0f", what, i, l, w)
		}
	}
	if end := top + float64(len(got))*smallH; end > bottom {
		t.Errorf("%s: lines end at %.0f, past %.0f", what, end, bottom)
	}
}

// startsFile: line is the head of the file's paragraph for name.
func startsFile(t *testing.T, what, line, para, name string) {
	t.Helper()
	if f := help.Fill(para, name); f == "" || !strings.HasPrefix(f, line) {
		t.Errorf("%s: %q does not start %q", what, line, f)
	}
}

// Without the assets every help is "" and every spot has no lines.
func TestHelpsNoAssets(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	if h := a.itemHelp(game.Factory); h != "" {
		t.Errorf("itemHelp: %q", h)
	}
	if h := a.gizmoHelp(0); h != "" {
		t.Errorf("gizmoHelp: %q", h)
	}
	p := &picker{cs: []choice{{item: game.Factory}}, hover: 0}
	u := unlock{game.Factory.String(), "planitem", int(game.Factory)}
	for what, ls := range map[string][]string{
		"part": partHelpLines(a, 0), "dlg": dlgHelpLines(a, game.Factory), "ship dlg": shipDlgHelpLines(a, 0),
		"picker": p.helpLines(a), "unlock": unlockHelpLines(a, u),
	} {
		if len(ls) != 0 {
			t.Errorf("%s: %q", what, ls)
		}
	}
}

// Out of range, NoItem included: "".
func TestHelpsUnknown(t *testing.T) {
	a, _, _ := helpApp(t)
	if a.itemHelp(game.NoItem) != "" || a.itemHelp(game.NumItems) != "" || a.gizmoHelp(game.NumGizmos) != "" {
		t.Error("an unknown index has a help")
	}
	if h := a.itemHelp(game.ColonyBase); h != "" {
		t.Errorf("the Colony Base's designers' note shown: %q", h)
	}
	if h := a.itemHelp(game.Factory); !strings.HasPrefix(h, "The Factory is") {
		t.Errorf("Factory: %q", h)
	}
}

// The design screen's part box: every part's paragraph under its facts
// and a blank line, whole: the box shows partHelpRows of it, the wheel
// scrolls the rest (helpScroll).
func TestPartHelp(t *testing.T) {
	a, _, gizmos := helpApp(t)
	w := float64(desPart.X1 - desPart.X0 - 2*padX)
	for g := range game.NumGizmos {
		top := partHelpTop(g)
		if facts := float64(desPart.Y0+padY) + bigFace.Size*1.25 + float64(len(partFacts(g)))*smallH; top < facts+smallH {
			t.Errorf("%v: help at %.0f over the facts' end %.0f and a blank line", g, top, facts)
		}
		got, want := partHelpLines(a, g), wantLines(a.gizmoHelp(g), w)
		n := fitLines(top, float64(desPart.Y1-padY))
		checkHelp(t, g.String(), got[:min(n, len(got))], want, top, float64(desPart.Y1-padY), w, 1<<30)
		if !slices.Equal(got, want) {
			t.Errorf("%v: %d lines, want the whole paragraph's %d", g, len(got), len(want))
		}
		if n != partHelpRows(g) {
			t.Errorf("%v: %d rows, want %d", g, partHelpRows(g), n)
		}
		if len(got) > 0 {
			startsFile(t, g.String(), got[0], gizmos[g], g.String())
		}
	}
}

// The structure dialog: every item's paragraph under its picture, in the
// nebula's inner width less dlgGap a side, above the bar.
func TestDlgHelp(t *testing.T) {
	a, items, _ := helpApp(t)
	w := float64(dlgInnerX1 - dlgInnerX0 - 2*dlgGap)
	for it := range game.NumItems {
		top := dlgHelpTop(a, it)
		if sp := a.spr("planitem", int(it)); sp.Image != nil {
			if bot := float64(dlgBox.Y0+dlgPicY) + zoom*float64(sp.Image.Bounds().Dy())/2; top < bot {
				t.Errorf("%v: help at %.0f over the picture's bottom %.0f", it, top, bot)
			}
		}
		got := dlgHelpLines(a, it)
		if it == game.ColonyBase { // no paragraph (itemHelp)
			if got != nil {
				t.Errorf("Colony Base: %q", got)
			}
			continue
		}
		checkHelp(t, it.String(), got, wantLines(a.itemHelp(it), w), top, float64(dlgBox.Y0+dlgBarTop-padY), w, 1<<30)
		if len(got) > 0 {
			startsFile(t, it.String(), got[0], items[it], it.String())
		}
	}
	if x0, x1 := dlgBox.X0+dlgInnerX0+dlgGap, dlgBox.X0+dlgInnerX0+dlgGap+int(w); x0 < dlgBox.X0+dlgInnerX0 || x1 > dlgBox.X0+dlgInnerX1 {
		t.Errorf("help band [%d,%d) outside the nebula", x0, x1)
	}
}

// The ship dialog's left band: the part's name, then its paragraph, the
// lines that fit between the title and the bar.
func TestShipDlgHelp(t *testing.T) {
	a, _, gizmos := helpApp(t)
	w := float64(dlgPicX1 - dlgPicX0)
	for g := range game.NumGizmos {
		got := shipDlgHelpLines(a, g)
		name := wantLines(g.String(), w)
		want := append(name, wantLines(a.gizmoHelp(g), w)...)
		checkHelp(t, g.String(), got, want, dlgHelpBandY0, dlgHelpBandY1, w, 1<<30)
		if len(got) > len(name) {
			startsFile(t, g.String(), got[len(name)], gizmos[g], g.String())
		}
		if len(got) > 0 && got[0] != name[0] {
			t.Errorf("%v: first line %q, want the name", g, got[0])
		}
	}
	if dlgHelpBandY0 < dlgTitleBottom || dlgHelpBandY1 > dlgBarTop {
		t.Errorf("band [%d,%d) outside title %d .. bar %d", dlgHelpBandY0, dlgHelpBandY1, dlgTitleBottom, dlgBarTop)
	}
}

// The picker's footer: the hovered item's paragraph under the verdict,
// up to pickHelpLines (four fit in both boxes), inside the box.
func TestPickerHelp(t *testing.T) {
	a, items, _ := helpApp(t)
	for _, orbit := range []bool{false, true} {
		p := &picker{hover: -1, orbit: orbit}
		b := p.box()
		w := float64(b.X1 - b.X0 - 2*padX)
		if n := int((float64(b.Y1-padY) - p.helpTop()) / smallH); n < pickHelpLines {
			t.Errorf("orbit %v: %d help lines fit, want %d", orbit, n, pickHelpLines)
		}
		if p.helpTop() < float64(p.footY()+6)+smallH {
			t.Errorf("orbit %v: help at %.0f over the verdict line", orbit, p.helpTop())
		}
		if ls := p.helpLines(a); ls != nil {
			t.Errorf("orbit %v: no hover, %q", orbit, ls)
		}
		for it := range game.NumItems {
			p.cs, p.hover = []choice{{item: it}}, 0
			got := p.helpLines(a)
			if it == game.ColonyBase { // no paragraph (itemHelp)
				if got != nil {
					t.Errorf("Colony Base: %q", got)
				}
				continue
			}
			checkHelp(t, it.String(), got, wantLines(a.itemHelp(it), w), p.helpTop(), float64(b.Y1-padY), w, pickHelpLines)
			if len(got) > 0 {
				startsFile(t, it.String(), got[0], items[it], it.String())
			}
		}
	}
}

// The footer grew into the rows: every cell still holds its tile area
// and its name line under it, for the longest list of each box (22
// square choices, every orbital item); TestPickerAt keeps the cells in
// the body.
func TestPickerRowsHoldNames(t *testing.T) {
	for _, tc := range []struct {
		orbit bool
		n     int
	}{{false, 22}, {true, int(game.FirstProject - game.FirstOrbital)}} {
		p := &picker{cs: make([]choice, tc.n), hover: -1, orbit: tc.orbit}
		for i := range p.cs {
			c := p.cell(i)
			if name := float64(p.tileArea(i).Y1+8) + smallH; name > float64(c.Y1) {
				t.Errorf("orbit %v cell %d: name ends at %.0f past the cell's %d", tc.orbit, i, name, c.Y1)
			}
			if c.Y1 > p.footY() {
				t.Errorf("orbit %v cell %d: %v past the footer %d", tc.orbit, i, c, p.footY())
			}
		}
	}
}

// The research screen: each unlock row of every tech, hovered, puts the
// unlock's name then its paragraph in resHelpFace, at most the area's
// lower half (a hull's: nothing); hovering one leaves the shown tech
// alone.
func TestUnlockHelp(t *testing.T) {
	a, items, gizmos := helpApp(t)
	w := float64(resArea.X1 - resArea.X0 - 2*padX)
	half := int(float64(resArea.Y1-resArea.Y0) / 2 / (resHelpFace.Size * 1.25))
	s := newResearchScreen()
	r := a.e.Research
	hulls := 0
	for tech := range game.NumTechs {
		r.Topic = tech // shown with nothing hovered
		rows := unlockRows(a, tech, unlockTop(len(detail(r, tech, a.e.ResearchRate()))-1), unlockBottom())
		for i, u := range rows {
			x, y := float64(u.x+u.w/2), float64(u.y+u.h/2)
			if _, ok := s.hoverAt(a, x, y); ok || s.unlock == nil || *s.unlock != u.unlock || s.shown(r) != tech {
				t.Fatalf("%v row %d: hover %v unlock %v shown %v", tech, i, ok, s.unlock, s.shown(r))
			}
			if s.hoverAt(a, float64(resBox.X1), y); s.unlock != nil {
				t.Errorf("%v row %d: hovered past the box's right edge", tech, i)
			}
			got := unlockHelpLines(a, u.unlock)
			var para, file string
			switch u.bank {
			case "planitem":
				para, file = a.itemHelp(game.Item(u.frame)), items[u.frame]
				if game.Item(u.frame) == game.ColonyBase { // no paragraph: no help at all
					if got != nil {
						t.Errorf("%v Colony Base: %q", tech, got)
					}
					continue
				}
			case gizmoBank:
				para, file = a.gizmoHelp(game.Gizmo(u.frame)), gizmos[u.frame]
			default:
				hulls++
				if got != nil {
					t.Errorf("%v hull row %q: %q", tech, u.name, got)
				}
				continue
			}
			name := wantLinesIn(resHelpFace, u.name, w)
			want := append(name, wantLinesIn(resHelpFace, para, w)...)
			if want = want[:min(len(want), half)]; !slices.Equal(got, want) {
				t.Errorf("%v: %q, want %q", u.name, got, want)
			}
			if len(got) > len(name) {
				startsFile(t, u.name, got[len(name)], file, u.name)
			}
		}
	}
	if hulls == 0 {
		t.Error("no hull row walked")
	}
	// Every line within the area's width, the band within its lower half.
	lh, tall := 0, 0
	for tech := range game.NumTechs {
		for _, u := range unlocks(r, a.e.Species, tech) {
			got := unlockHelpLines(a, u)
			if len(got) == 0 {
				continue
			}
			if got[0] != wantLinesIn(resHelpFace, u.name, w)[0] {
				t.Errorf("%v: first line %q", u.name, got[0])
			}
			for _, l := range got {
				if ui.Width(l, resHelpFace) > w {
					t.Errorf("%v: %q past the area", u.name, l)
				}
			}
			lh, tall = max(lh, len(got)), max(tall, int(float64(len(got))*resHelpFace.Size*1.25))
		}
	}
	if tall == 0 || tall > (resArea.Y1-resArea.Y0)/2 {
		t.Errorf("the tree's band: %d lines, %d px of the area's %d", lh, tall, resArea.Y1-resArea.Y0)
	}
	t.Logf("the tree's band: at most %d lines, %d px", lh, tall)
}
