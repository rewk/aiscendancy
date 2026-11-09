package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// contactApp is seed 1 with Snovemdomas, Orfa and Govorom, none met,
// the starmap open.
func contactApp(t *testing.T) *app {
	t.Helper()
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Govorom}, game.Average))
	a.push(newStarmapScreen())
	for j := range a.g.Empires {
		if j != a.player && a.e.StanceWith(j) != game.Unknown {
			t.Fatalf("empire %d met at the start", j)
		}
	}
	return a
}

// A ship of ours at each foreign home and one of empire 2's at empire
// 1's: the day's close queues one popup per empire we met, in the
// events' order, none for the bots' own contact; a quiet day none more.
func TestContactQueue(t *testing.T) {
	a := contactApp(t)
	h1, h2 := a.g.Empires[1].Colonies[0].Star, a.g.Empires[2].Colonies[0].Star
	a.e.Spawn(fixtureShip, h1, 0)
	a.e.Spawn(fixtureShip, h2, 0)
	a.g.Empires[2].Spawn(fixtureShip, h1, 0)
	evs := a.endDay()
	var want []contactNote
	for _, x := range evs {
		if x.Kind == game.Contacted {
			want = append(want, met1(x.Other))
		}
	}
	if a.g.Empires[1].StanceWith(2) == game.Unknown {
		t.Fatal("the bots did not meet")
	}
	if len(want) != 2 || !slices.Contains(want, met1(1)) || !slices.Contains(want, met1(2)) {
		t.Fatalf("our contacts %v, want 1 and 2", want)
	}
	if !slices.Equal(a.contacts, want) || !a.contacting() {
		t.Fatalf("queue %v, want %v (up %v)", a.contacts, want, a.contacting())
	}
	a.endDay()
	if !slices.Equal(a.contacts, want) {
		t.Errorf("a quiet day queued %v", a.contacts)
	}
}

// Talk opens the negotiation with the head's empire, the queue going on
// over it; T over it opens the next's.
func TestContactTalk(t *testing.T) {
	a := contactApp(t)
	a.g.Contact(0, 1)
	a.g.Contact(0, 2)
	a.contacts = metAll(2, 1)
	l := contactLayout(a, 2)
	a.contactPress(false, true, float64(l.talk.X0+1), float64(l.talk.Y0+1)) // the plain press: Talk is not OK
	if !slices.Equal(a.contacts, metAll(2, 1)) {
		t.Fatalf("a click on Talk closed as OK: %v", a.contacts)
	}
	n := len(a.stack)
	a.contactTalk()
	s := top[*negotiateScreen](t, a)
	if s.j != 2 || len(a.stack) != n+1 || !slices.Equal(a.contacts, metAll(1)) || !a.contacting() {
		t.Fatalf("talking to %d, stack %d -> %d, queue %v, up %v", s.j, n, len(a.stack), a.contacts, a.contacting())
	}
	pressing(t, ebiten.KeyT)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if s := top[*negotiateScreen](t, a); s.j != 1 || len(a.stack) != n+2 || len(a.contacts) != 0 {
		t.Errorf("T: talking to %d, stack %d -> %d, queue %v", s.j, n, len(a.stack), a.contacts)
	}
	a.contacts = metAll(1)
	a.contactTalk()
	if len(a.stack) != n+2 {
		t.Errorf("Talk with its negotiation on top pushed another: stack %d", len(a.stack))
	}
}

// OK, a closing key and a click outside pop the head, a click inside
// does nothing; it waits for the colony reports, the ability popup waits
// for it; Space runs no day; F9's restart and New game drop it.
func TestContactClose(t *testing.T) {
	a := contactApp(t)
	a.g.Contact(0, 1)
	a.contacts = metAll(1, 1, 1, 1)
	l := contactLayout(a, 1)
	if l.box != dlgBox || l.ok.X1 != dlgBar.X1 || l.talk.X0 != dlgBar.X0 || l.talk.X1 != l.ok.X0 {
		t.Errorf("box %+v, Talk %+v, OK %+v", l.box, l.talk, l.ok)
	}
	a.contactPress(false, true, float64(l.ringX), float64(l.ringY))
	if len(a.contacts) != 4 {
		t.Fatal("a click inside closed it")
	}
	a.contactPress(false, true, float64(l.ok.X0+1), float64(l.ok.Y0+1))
	a.contactPress(true, false, 0, 0)
	a.contactPress(false, true, float64(l.box.X0-1), float64(l.box.Y0))
	if !slices.Equal(a.contacts, metAll(1)) {
		t.Fatalf("after three closes: %v", a.contacts)
	}
	a.colReports = []colonyReport{{a.cl.Home, 0, "x"}}
	a.charged = true
	if a.contacting() || !a.reporting() {
		t.Fatal("up over a colony report")
	}
	a.colReports = nil
	if !a.contacting() || a.charging() || a.keysSection() != keysPopups {
		t.Fatalf("contacting %v, charging %v", a.contacting(), a.charging())
	}
	day, n := a.g.Day, len(a.stack)
	pressing(t, ebiten.KeySpace)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	pressing(t, ebiten.KeyEnter)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if a.g.Day != day || len(a.contacts) != 0 || len(a.stack) != n || !a.charging() {
		t.Fatalf("day %d -> %d, queue %v, stack %d -> %d, charging %v", day, a.g.Day, a.contacts, n, len(a.stack), a.charging())
	}
	a.contacts = metAll(1)
	a.restart()
	if a.contacts != nil {
		t.Errorf("restart kept %v", a.contacts)
	}
	a.contacts = metAll(1)
	if err := a.answerEnd(true); err != nil || a.contacts != nil || a.contacting() {
		t.Errorf("New game kept %v (%v)", a.contacts, err)
	}
}

// The text is "The <species> <description>" from the species table,
// nothing else of the empire; every species' fits the column above the
// bar, the ring inside the content.
func TestContactText(t *testing.T) {
	a := contactApp(t)
	c := discContent()
	o := a.g.Empires[1]
	for sp := range game.NumSpecies {
		o.Species = sp
		in := game.SpeciesTable[sp]
		if got, want := a.contactText(1), "The "+in.Name+" "+in.Description; got != want {
			t.Errorf("%s: %q, want %q", sp, got, want)
		}
		l := contactLayout(a, 1)
		ring := rect{l.ringX - contactRingW/2, l.ringY - contactRingW/2, l.ringX + contactRingW/2, l.ringY + contactRingW/2}
		if !inside(ring, c) || l.textX < ring.X1 || l.titleY < c.Y0 || l.nameY < l.titleY {
			t.Errorf("%s: ring %+v, text x %d, content %+v", sp, ring, l.textX, c)
		}
		if bottom := l.textY + len(l.lines)*exploreLineH; bottom > c.Y1 {
			t.Errorf("%s: %d lines end at %d, past %d", sp, len(l.lines), bottom, c.Y1)
		}
		if got := strings.Join(l.lines, " "); strings.Join(strings.Fields(got), " ") != strings.Join(strings.Fields(a.contactText(1)), " ") {
			t.Errorf("%s: lines %q cut the text", sp, l.lines)
		}
	}
}

// The Species box's flags: ours, then each empire we met in empire
// order (a bots' contact adds none), the defeated kept; built with no
// allocation into a kept buffer.
func TestSpeciesFlags(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Govorom, game.Mebes}, game.Average))
	buf := speciesFlags(a, nil)
	if !slices.Equal(buf, []int{0}) {
		t.Fatalf("none met: %v", buf)
	}
	a.g.Contact(0, 2)
	a.g.Contact(1, 3)
	a.g.Contact(1, 0)
	var want []int
	for j := range a.g.Empires {
		if j == a.player || a.e.Relations[j].Met >= 0 {
			want = append(want, j)
		}
	}
	if buf = speciesFlags(a, buf[:0]); !slices.Equal(buf, want) || !slices.Equal(want, []int{0, 1, 2}) {
		t.Fatalf("flags %v, want %v", buf, want)
	}
	a.g.Empires[2].Lost = 1
	if buf = speciesFlags(a, buf[:0]); !slices.Equal(buf, want) {
		t.Errorf("a defeated empire dropped: %v", buf)
	}
	if n := testing.AllocsPerRun(10, func() { buf = speciesFlags(a, buf[:0]) }); n != 0 {
		t.Errorf("%v allocations a frame", n)
	}
}

// The flags sit in the picture flush right, evenly spaced, side by side
// while they fit (all seven, right of the label), overlapping past that;
// a lone flag where ours always was.
func TestSpeciesFlagX(t *testing.T) {
	pic := speciesPic()
	r := mapStack[speciesBox]
	if pic.X1 > r.X1 || pic.X0 < r.X0 || pic.Y1 > r.Y1 {
		t.Fatalf("picture %+v outside the box %+v", pic, r)
	}
	if x := speciesFlagX(pic, 0, 1); x != float64(r.X1-zoom*(15+6)) {
		t.Errorf("lone flag at %v", x)
	}
	w := flagW * zoom
	if s := speciesFlagX(pic, 1, maxEmpires) - speciesFlagX(pic, 0, maxEmpires); s != float64(w+zoom) ||
		float64(pic.X0) < float64(r.X0+padX)+ui.Width("Species", bigFace) {
		t.Errorf("%d flags: step %v, picture from %d over the label", maxEmpires, s, pic.X0)
	}
	for n := 2; n <= maxEmpires; n++ {
		step := speciesFlagX(pic, 1, n) - speciesFlagX(pic, 0, n)
		for k := range n {
			x := speciesFlagX(pic, k, n)
			if x < float64(pic.X0) || x+float64(w) > float64(pic.X1) {
				t.Errorf("%d flags: flag %d at %v out of %+v", n, k, x, pic)
			}
			if k > 0 && x-speciesFlagX(pic, k-1, n) != step {
				t.Errorf("%d flags: uneven at %d", n, k)
			}
		}
		if last := speciesFlagX(pic, n-1, n) + float64(w); last != float64(pic.X1) {
			t.Errorf("%d flags end at %v, want %d", n, last, pic.X1)
		}
		if fit := n*w+(n-1)*zoom <= pic.X1-pic.X0; fit != (step == float64(w+zoom)) || step <= 0 {
			t.Errorf("%d flags: step %v (fit %v)", n, step, fit)
		}
	}
}

// The popup draws every species in every colour, and the starmap its
// Species box with every empire met, one defeated, with the game's
// sprites (headless; no panic, no frame out of its bank).
func TestContactDraw(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { as.Close() })
	a := contactApp(t)
	a.a = as
	a.g.Contact(0, 1)
	a.g.Contact(0, 2)
	a.g.Empires[2].Lost = 1
	frame := ebiten.NewImage(screenW, screenH)
	o := a.g.Empires[1]
	for sp := range game.NumSpecies {
		for c := range game.NumColours {
			o.Species, o.Colour = sp, c
			a.contacts = metAll(1)
			a.drawContact(frame)
			a.contacts = []contactNote{{1, game.WarDeclared}} // their DA_DeclareWar line
			a.drawContact(frame)
		}
	}
	top[*starmapScreen](t, a).Render(a, frame)
}

// met1 is empire j's first-contact note.
func met1(j int) contactNote { return contactNote{j, game.Contacted} }

// metAll is the first-contact notes of js, in order.
func metAll(js ...int) []contactNote {
	var out []contactNote
	for _, j := range js {
		out = append(out, met1(j))
	}
	return out
}

// A war declared on us and an alliance broken with us queue the same
// popup, titled and worded for the act (the race's DA_ line, else a
// plain sentence); the day's other events queue nothing.
func TestContactWarBreak(t *testing.T) {
	a := contactApp(t)
	a.record(a.g.Contact(0, 1))
	a.record(a.g.Contact(0, 2))
	a.contacts = nil
	ally(t, a.g) // 0 and 1 allied (negotiate_test.go)
	reps, err := a.g.BreakAlliance(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := a.g.DeclareWar(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	a.contacted(a.record(append(reps, r2...)))
	want := []contactNote{{1, game.AllianceBroken}, {2, game.WarDeclared}}
	if !slices.Equal(a.contacts, want) || !a.contacting() {
		t.Fatalf("queue %v, want %v", a.contacts, want)
	}
	for _, n := range want {
		txt := a.noteText(n)
		if txt == "" || txt == a.contactText(n.J) || noteTitles[n.Kind] == "" {
			t.Errorf("%v: title %q, text %q", n, noteTitles[n.Kind], txt)
		}
	}
	if got := a.noteText(want[1]); a.a == nil && got != "The Govorom declare war on us." {
		t.Errorf("plain war text %q", got)
	}
	a.contactPress(true, false, 0, 0)
	if !slices.Equal(a.contacts, want[1:]) {
		t.Errorf("after OK: %v", a.contacts)
	}
	a.contacts = nil
	a.g.Empires[2].Lost = 1
	a.contacted([]game.Event{{Kind: game.WarDeclared, Other: 2}})
	if len(a.contacts) != 0 {
		t.Errorf("a defeated empire's war queued: %v", a.contacts)
	}
}
