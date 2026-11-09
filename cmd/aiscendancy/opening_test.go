package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// The opening's labels: Resume is dead without a loadable quick save and
// its key does nothing; Load opens the slots screen over the opening, Esc
// back to it; Exit (X) terminates; the two unplayed labels never play;
// the introduction opens over it and, without the movie (no assets),
// leaves at once.
func TestOpening(t *testing.T) {
	a := &app{lastPath: filepath.Join(t.TempDir(), "none.json")}
	a.open([]screen{newOpeningScreen()})
	s := top[*openingScreen](t, a)
	for _, k := range []int{openTutorial, openResume} {
		if s.live(k) {
			t.Errorf("label %d live", k)
		}
	}
	pressing(t, ebiten.KeyR)
	if err := s.Update(a); err != nil || a.inGame() || len(a.stack) != 1 {
		t.Fatalf("R without a save: %v, game %v, %d screens", err, a.inGame(), len(a.stack))
	}
	pressing(t, ebiten.KeyI)
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	mv := top[*movieScreen](t, a)
	pressing(t)
	if err := mv.Update(a); err != nil {
		t.Fatal(err)
	}
	top[*openingScreen](t, a)
	pressing(t, ebiten.KeyL)
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	top[*slotsScreen](t, a)
	if err := a.back(); err != nil || a.quitting {
		t.Fatalf("Esc on the slots: %v, quitting %v", err, a.quitting)
	}
	top[*openingScreen](t, a)
	pressing(t, ebiten.KeyX)
	if err := s.Update(a); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("X: %v", err)
	}
}

// A day's end writes lastPath (saveLast), not the quick save; Resume
// Last Game loads it onto a fresh starmap, at that day.
func TestOpeningResume(t *testing.T) {
	dir := t.TempDir()
	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	w := testApp(g)
	w.bots = bot.NewAll(g, bot.Default())
	w.bots[0] = nil
	w.savePath, w.lastPath = filepath.Join(dir, "quick.json"), filepath.Join(dir, lastFile)
	w.nextDay()
	if w.toastErr {
		t.Fatal(w.toast)
	}
	if _, err := os.Stat(w.savePath); !os.IsNotExist(err) {
		t.Fatalf("the quick save written: %v", err)
	}
	a := &app{lastPath: w.lastPath}
	a.open([]screen{newOpeningScreen()})
	s := top[*openingScreen](t, a)
	if !s.live(openResume) {
		t.Fatal("Resume dead with a last save")
	}
	pressing(t, ebiten.KeyR)
	if err := s.Update(a); err != nil || !a.inGame() || len(a.stack) != 1 {
		t.Fatalf("R: %v, game %v, %d screens (toast %q)", err, a.inGame(), len(a.stack), a.toast)
	}
	top[*starmapScreen](t, a)
	if a.e.Day != w.e.Day || a.e.Day == 0 {
		t.Errorf("resumed at day %d, want %d", a.e.Day, w.e.Day)
	}
}
