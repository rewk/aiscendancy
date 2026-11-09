package main

import (
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/help"
	"github.com/rewk/aiscendancy/ui"
)

// The end of the game for the player: once the player's empire is
// defeated no day runs and the end prompt comes up (end-xtnc.txt's text,
// New game or Quit, F9 the quick save), taking every input.

// over reports a game whose player is defeated: no more days.
func (a *app) over() bool { return a.inGame() && !a.e.Alive() }

// ending reports the end prompt up: the player out, the top screen
// neither the log nor a combat replay (both come first).
func (a *app) ending() bool {
	if !a.over() {
		return false
	}
	switch s := a.top().(type) {
	case *logScreen:
		return false
	case *systemScreen:
		return s.replay == nil
	}
	return true
}

// endFallback is the end prompt's text without the game's archives.
var endFallback = []string{"Our species is defeated."}

// endPrompt is the end prompt, its lines read once (endLines).
func (a *app) endPrompt() prompt {
	if a.endTxt == nil {
		a.endTxt = a.endLines()
	}
	return prompt{a.endTxt, "New game (Enter)", "Quit (Esc)"}
}

// endLines is end-xtnc.txt's description reflowed to the prompt, at
// most the lines above its buttons; endFallback without it.
func (a *app) endLines() []string {
	if a.a == nil {
		return endFallback
	}
	b, err := a.a.Game.Read("end-xtnc.txt")
	var ls []string
	if err == nil {
		ls, err = help.ParseEnding(b)
	}
	if err != nil {
		log.Printf("end-xtnc.txt: %v", err)
		return endFallback
	}
	box, _, _ := quitPrompt.rects()
	cols := int(float64(box.X1-box.X0-4*padX) / ui.Width("m", bigFace))
	out := reflow(paragraph{false, strings.Join(ls, " ")}, cols)
	n := (dlgBar.Y0 - box.Y0 - dlgTitleY - int(bigFace.Size)) / (zoom * promptLineH)
	return out[:min(len(out), n+1)]
}

// endUpdate reads the end prompt's keys and clicks: F9 loads the quick
// save, else the answers.
func (a *app) endUpdate() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		a.load()
		return nil
	}
	if ok, yes := a.endPrompt().update(a); ok {
		return a.answerEnd(yes)
	}
	return nil
}

// answerEnd closes the end prompt: Quit brings up the quit prompt, New
// game drops the whole game state and replaces the stack (its screens
// point into the old game) with the New Game screen.
func (a *app) answerEnd(newGame bool) error {
	if !newGame {
		a.quitting = true
		return nil
	}
	a.warn = nil
	a.g, a.player, a.e, a.cl = nil, 0, nil, nil
	a.bots, a.log, a.greeted = nil, nil, nil
	a.discoveries, a.discTotal, a.charged, a.victory = nil, 0, false, game.NoVictory
	a.explored, a.colReports, a.contacts, a.proposals = nil, nil, nil, nil
	clear(a.stack)
	a.stack = nil
	a.open([]screen{newNewGameScreen()})
	return nil
}
