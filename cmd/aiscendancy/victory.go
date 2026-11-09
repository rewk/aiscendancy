package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/help"
	"github.com/rewk/aiscendancy/ui"
)

// The victory popup: the player's Won at the close (game/victory.go)
// sets a.victory; after every other popup it shows the victory's end text
// (end-<v.Ending()>.txt's enddesc) and a rank, Continue alone: the game
// goes on. Never saved: the Won event is told once.

// victoryFallback is the popup's text without the game's archives.
var victoryFallback = []string{"Our species has won."}

// rankDays is the days a rank costs: a victory on day d takes rank
// 9 − d/rankDays of the end text's ten, clamped (OUR ASSUMPTION: the
// original's rule is in the exe; the sooner the higher).
const rankDays = 200

// victoryRank is the rank of a victory on day among ranks (lowest first).
func victoryRank(ranks []string, day int) string {
	if len(ranks) == 0 {
		return ""
	}
	k := min(max(len(ranks)-1-day/rankDays, 0), len(ranks)-1)
	return ranks[k]
}

// winning is whether the victory popup is up: after the ability popup's
// conditions and the ability popup itself.
func (a *app) winning() bool {
	if a.victory == game.NoVictory || a.charged || len(a.discoveries) > 0 || len(a.explored) > 0 || len(a.colReports) > 0 || len(a.contacts) > 0 || len(a.proposals) > 0 || !a.inGame() || a.over() {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// victoryPrompt is the popup: the end text reflowed to the box, a blank
// line, the rank and the day; read from the archives once per victory
// (winTxt), not every frame.
func (a *app) victoryPrompt() prompt {
	if a.winFor != a.victory || a.winTxt == nil {
		a.winTxt, a.winFor = a.victoryLines(), a.victory
	}
	return prompt{a.winTxt, "Continue (Enter)", ""}
}

// victoryLines are victoryPrompt's lines.
func (a *app) victoryLines() []string {
	box, _, _ := quitPrompt.rects()
	cols := int(float64(box.X1-box.X0-4*padX) / ui.Width("m", bigFace))
	desc, ranks := victoryFallback, []string(nil)
	if a.a != nil {
		name := "end-" + a.victory.Ending() + ".txt"
		b, err := a.a.Game.Read(name)
		if err == nil {
			desc, err = help.ParseEnding(b)
		}
		if err == nil {
			ranks, err = help.ParseRanks(b)
		}
		if err != nil {
			log.Printf("%s: %v", name, err)
			desc, ranks = victoryFallback, nil
		}
	}
	ls := reflow(paragraph{false, strings.Join(desc, " ")}, cols)
	ls = append(ls, "", fmt.Sprintf("Victory by %s, day %d", a.victory, a.e.WonDay))
	if r := victoryRank(ranks, a.e.WonDay); r != "" {
		ls = append(ls, "Rank: "+r)
	}
	return ls
}

// victoryUpdate closes the popup on any answer.
func (a *app) victoryUpdate() error {
	if ok, _ := a.victoryPrompt().update(a); ok {
		a.victory = game.NoVictory
	}
	return nil
}
