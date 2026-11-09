package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The close that unites the galaxy under the player's alliance brings
// the victory popup up (after the ability popup), Continue alone, the
// day and the victory in its text; it is told once; New game drops it.
func TestVictoryPopup(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average))
	a.push(newStarmapScreen())
	a.e.Relations[1].Stance, a.g.Empires[1].Relations[0].Stance = game.Alliance, game.Alliance
	evs := a.endDay()
	if !slices.ContainsFunc(evs, func(x game.Event) bool { return x.Kind == game.Won && x.Other < 0 }) || a.victory != game.UniteVictory || !a.winning() {
		t.Fatalf("events %+v, victory %v, up %v", evs, a.victory, a.winning())
	}
	p := a.victoryPrompt()
	if _, _, no := p.rects(); no != (rect{}) || !strings.Contains(strings.Join(p.lines, " "), "Victory by unification, day") {
		t.Errorf("no %v, lines %q", no, p.lines)
	}
	a.charged = true
	if a.winning() {
		t.Error("up over the ability popup")
	}
	a.charged = false
	a.victory = game.NoVictory // Continue
	a.endDay()
	if a.victory != game.NoVictory {
		t.Error("told again")
	}
	a.victory = game.UniteVictory
	if err := a.answerEnd(true); err != nil || a.victory != game.NoVictory {
		t.Errorf("New game kept it: %v", err)
	}
}

// The rank: the sooner the higher, clamped to the ten.
func TestVictoryRank(t *testing.T) {
	r := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	for _, c := range []struct {
		day  int
		want string
	}{{0, "9"}, {rankDays - 1, "9"}, {rankDays, "8"}, {5 * rankDays, "4"}, {100 * rankDays, "0"}} {
		if got := victoryRank(r, c.day); got != c.want {
			t.Errorf("day %d: %q, want %q", c.day, got, c.want)
		}
	}
	if victoryRank(nil, 3) != "" {
		t.Error("a rank without ranks")
	}
}

// The popup's text cache follows the victory, not its kind alone: a
// victory, then F9 back to before it and the same victory later, shows
// the later day.
func TestVictoryPopupAfterReload(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average))
	a.savePath = filepath.Join(t.TempDir(), "quick.json")
	a.push(newStarmapScreen())
	if !a.saveTo(a.savePath) {
		t.Fatal("fixture: save")
	}
	ally := func() {
		a.e.Relations[1].Stance, a.g.Empires[1].Relations[0].Stance = game.Alliance, game.Alliance
	}
	ally()
	a.endDay()
	first := strings.Join(a.victoryPrompt().lines, " ")
	if a.victory != game.UniteVictory {
		t.Fatalf("fixture: victory %v", a.victory)
	}
	a.victory = game.NoVictory // Continue
	if !a.load() {
		t.Fatal("fixture: load")
	}
	a.bots = nil // as testApp: no bot breaks the alliance
	for range 3 {
		a.endDay()
	}
	ally()
	a.endDay()
	if a.victory != game.UniteVictory || a.e.WonDay == 1 {
		t.Fatalf("fixture: victory %v on day %d", a.victory, a.e.WonDay)
	}
	got := strings.Join(a.victoryPrompt().lines, " ")
	if want := fmt.Sprintf("day %d", a.e.WonDay); !strings.Contains(got, want) {
		t.Errorf("after the reload %q, want %q (first %q)", got, want, first)
	}
}
