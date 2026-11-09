package main

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// pressing fakes keys as just pressed this tick (keyJustPressed), none
// for no key; the real one is back after the test.
func pressing(t *testing.T, keys ...ebiten.Key) {
	t.Helper()
	saved := keyJustPressed
	t.Cleanup(func() { keyJustPressed = saved })
	keyJustPressed = func(k ebiten.Key) bool {
		for _, d := range keys {
			if d == k {
				return true
			}
		}
		return false
	}
}

// F1 opens the key help and F1 or Esc closes it; while up Space runs no
// day, Tab opens no log, Esc goes no back (no quit prompt on the last
// screen). Without it Space ends the day: the fake reaches Update.
func TestKeysOverlay(t *testing.T) {
	a := discApp(t)
	day, n := a.g.Day, len(a.stack)
	step := func(keys ...ebiten.Key) {
		t.Helper()
		pressing(t, keys...)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	step(ebiten.KeyF1)
	if !a.keysUp || a.keysSection() != keysStarmap {
		t.Fatalf("F1: up %v, section %q", a.keysUp, a.keysSection())
	}
	step(ebiten.KeySpace)
	step(ebiten.KeyTab)
	if !a.keysUp || a.g.Day != day || len(a.stack) != n {
		t.Errorf("under the help: up %v, day %d -> %d, stack %d -> %d", a.keysUp, day, a.g.Day, n, len(a.stack))
	}
	step(ebiten.KeyF1)
	if a.keysUp {
		t.Error("F1 again left it up")
	}
	step(ebiten.KeyF1)
	step(ebiten.KeyEscape)
	if a.keysUp || a.quitting || len(a.stack) != n {
		t.Errorf("Esc: up %v, quitting %v, stack %d -> %d", a.keysUp, a.quitting, n, len(a.stack))
	}
	if !a.keysInput(true, false, false) || !a.keysInput(false, true, false) || a.keysUp || a.keysInput(false, false, false) {
		t.Error("a click (close) did not close it, or a quiet tick was taken")
	}
	step(ebiten.KeySpace)
	if a.g.Day != day+1 {
		t.Errorf("Space without the help: day %d, want %d", a.g.Day, day+1)
	}
}

// Space neither closes the discovery nor the ability popup nor runs a
// day under them; Enter closes each.
func TestSpaceUnderPopups(t *testing.T) {
	a := discApp(t)
	day := a.g.Day
	a.discoveries = []discovery{{1, "x"}}
	a.charged = true
	for _, up := range []func() bool{a.discovering, a.charging} {
		pressing(t, ebiten.KeySpace)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
		if !up() || a.g.Day != day {
			t.Errorf("Space: up %v, day %d -> %d", up(), day, a.g.Day)
		}
		if a.keysSection() != keysPopups {
			t.Errorf("section %q under a popup", a.keysSection())
		}
		pressing(t, ebiten.KeyEnter)
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
		if up() {
			t.Error("Enter left the popup up")
		}
	}
}

// On the system screen D flips the selected ship's shields and S opens
// the species screen, as everywhere.
func TestSystemKeys(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	if err := spawnFixtures(a, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	gun := a.e.Ships[0]
	a.push(newStarmapScreen())
	v := newSystemScreen(gun.Star)
	a.push(v)
	v.sel = gun
	was := gun.Shields
	pressing(t, ebiten.KeyD)
	if err := v.Update(a); err != nil {
		t.Fatal(err)
	}
	if gun.Shields == was || a.top() != v {
		t.Errorf("D: shields %v -> %v, top %T", was, gun.Shields, a.top())
	}
	pressing(t, ebiten.KeyS)
	if err := v.Update(a); err != nil {
		t.Fatal(err)
	}
	if gun.Shields == was {
		t.Error("S flipped the shields back")
	}
	top[*speciesScreen](t, a)
}

// The "?" button opens the help like F1 and, being a left click, closes
// it; a quiet tick opens nothing. It sits in the frame, 24x24, clear of
// the bottom-right buttons we can list.
func TestKeysButton(t *testing.T) {
	a := discApp(t)
	if a.keysInput(false, false, false) || a.keysUp {
		t.Fatal("a quiet tick opened it")
	}
	if !a.keysInput(false, true, true) || !a.keysUp {
		t.Fatal("the button did not open it (and take the press)")
	}
	if !a.keysInput(false, true, true) || a.keysUp {
		t.Error("the button while up did not close it")
	}
	if !a.keysInput(true, false, false) || !a.keysUp {
		t.Error("F1 no longer opens it")
	}
	r := keysButton
	if r.X0 < 0 || r.Y0 < 0 || r.X1 > screenW || r.Y1 > screenH || r.X1-r.X0 != 24 || r.Y1-r.Y0 != 24 {
		t.Errorf("button %v: not 24x24 in the frame", r)
	}
	overlap := func(o rect) bool { return o.X0 < r.X1 && r.X0 < o.X1 && o.Y0 < r.Y1 && r.Y0 < o.Y1 }
	named := map[string]rect{"quitSave": quitSave, "quitLoad": quitLoad, "quitShot": quitShot, "quitSet": quitSet,
		"sysEndTurn": sysEndTurn, "sysRepeat": sysRepeat, "sysAuto": sysAuto,
		"sysFilters.planets": sysFilters[planetsTab], "sysFilters.ships": sysFilters[shipsTab]}
	for i, m := range mapStack {
		named[fmt.Sprintf("mapStack[%d]", i)] = m
	}
	for n, o := range named {
		if overlap(o) {
			t.Errorf("keysButton overlaps %s %v", n, o)
		}
	}
	// turn.go's end-day button, by probing its hit test over the button.
	for y := r.Y0; y < r.Y1; y++ {
		for x := r.X0; x < r.X1; x++ {
			if turnButton(float64(x), float64(y)) != noButton {
				t.Fatalf("turnButton answers at (%d,%d), under keysButton", x, y)
			}
		}
	}
}

// keyTable fits the overlay's two columns as drawKeys flows it: no
// section past keysBot, none past a third column; the keys and the what
// within their widths. The research screen names its section.
func TestKeysFit(t *testing.T) {
	lh := bigFace.Size * 1.25
	x, y := float64(keysX0), keysTop+2*lh
	for _, sec := range keyTable {
		h := float64(len(sec.rows)+1) * lh
		if y+h > keysBot && x == keysX0 {
			x, y = keysX1, keysTop+2*lh
		}
		if y+h > keysBot {
			t.Errorf("section %q ends at %.0f past %d", sec.title, y+h, keysBot)
		}
		for _, r := range sec.rows {
			if w := ui.Width(r.keys, bigFace); 12+w > keysWhat {
				t.Errorf("%q: keys %q %.0f px wide", sec.title, r.keys, w)
			}
			if w := ui.Width(r.what, bigFace); x+keysWhat+w > screenW {
				t.Errorf("%q: %q past the frame", sec.title, r.what)
			}
		}
		y += h + lh/2
	}
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	a.push(newResearchScreen())
	if a.keysSection() != keysResearch {
		t.Errorf("research screen: section %q", a.keysSection())
	}
}
