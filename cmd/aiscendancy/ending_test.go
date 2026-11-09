package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
)

// The player defeated (its last colony captured by the Orfa): the
// day that defeats it is logged, the end prompt is up at once over the
// starmap (no log pops), held back while a log
// opened over it is up; no day runs after it (turn buttons, skip,
// Space); New game opens the New Game screen with no game, Quit quits.
func TestDefeatEndsTheGame(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	// The player's home captured by the Orfa, as an invasion leaves it.
	b := g.Empires[1]
	home := a.e.Colonies[0]
	home.Planet.Owner = b.Index
	a.e.Colonies = nil
	b.Colonies = append(b.Colonies, home)
	if a.ending() {
		t.Fatal("the end prompt before the defeat")
	}

	a.pressTurn(nextButton)
	if a.e.Alive() || !slices.ContainsFunc(a.log, func(l logEntry) bool { return l.Ev.Kind == game.Defeated }) {
		t.Fatalf("alive %v, log %v", a.e.Alive(), a.log)
	}
	top[*starmapScreen](t, a)
	if !a.ending() {
		t.Error("no end prompt after the defeat")
	}
	a.push(newLogScreen())
	if a.ending() {
		t.Error("the end prompt over the log")
	}
	day := a.e.Day
	a.pressTurn(nextButton)
	a.pressTurn(skipButton)
	a.space()
	if a.e.Day != day || g.Day != day {
		t.Fatalf("days ran after the defeat: %d, game %d, want %d", a.e.Day, g.Day, day)
	}
	if err := a.pop(); err != nil {
		t.Fatal(err)
	}
	if !a.ending() {
		t.Fatal("no end prompt once the log is closed")
	}
	if pr := a.endPrompt(); !slices.Equal(pr.lines, endFallback) || pr.yes != "New game (Enter)" || pr.no != "Quit (Esc)" {
		t.Errorf("prompt %+v", pr)
	}
	// Quit asks first (the quit prompt); No comes back to the end prompt.
	if err := a.answerEnd(false); err != nil || !a.quitting || !a.ending() {
		t.Errorf("Quit: %v, quit prompt %v", err, a.quitting)
	}
	if err := a.answerQuit(false); err != nil || a.quitting || !a.ending() {
		t.Errorf("No: %v, quit prompt %v, end prompt %v", err, a.quitting, a.ending())
	}
	if err := a.answerEnd(false); err != nil {
		t.Fatal(err)
	}
	if err := a.answerQuit(true); !errors.Is(err, ebiten.Termination) {
		t.Errorf("Yes: %v", err)
	}
	if err := a.answerEnd(true); err != nil || a.inGame() || a.e != nil || a.bots != nil || a.log != nil || len(a.stack) != 1 {
		t.Fatalf("New game: %v, in game %v, stack %d", err, a.inGame(), len(a.stack))
	}
	top[*newGameScreen](t, a)
	if a.ending() {
		t.Error("the end prompt on the New Game screen")
	}
}
