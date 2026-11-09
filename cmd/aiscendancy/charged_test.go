package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// charge makes empire i's ability ready today: last used Recharge days
// ago (a timed ability starts empty).
func charge(g *game.Game, i int) {
	g.Empires[i].AbilityUsed = g.Day - g.Empires[i].Species.Info().Recharge
}

// The closing day holding our AbilityReady brings the ability popup up
// over the starmap (none before it), the event logged at the root; the
// popup has OK alone, the whole bar; OK closes it, no day run; F9's
// restart and New game drop it.
func TestChargedPopup(t *testing.T) {
	a := testApp(game.NewGame(1, []game.Species{game.Baliflids, game.Orfa}, game.Average))
	a.push(newStarmapScreen())
	rc := a.e.Species.Info().Recharge
	for a.g.Day < rc-1 {
		a.endDay()
		if a.charged {
			t.Fatalf("up on day %d, recharge %d", a.g.Day, rc)
		}
	}
	evs := a.endDay()
	i := slices.IndexFunc(evs, func(x game.Event) bool { return x.Kind == game.AbilityReady })
	if i < 0 || !a.charged || !a.charging() {
		t.Fatalf("day %d: events %+v, charged %v", a.g.Day, evs, a.charged)
	}
	top[*starmapScreen](t, a)
	if l := a.log[len(a.log)-len(evs)+i]; l.Ev != evs[i] || l.Day != rc || !rootKind(l.Ev.Kind) || len(eventScreens(a, l.Ev)) != 1 {
		t.Errorf("log %+v", l)
	}
	p := a.chargedPrompt()
	if _, yes, no := p.rects(); yes != dlgBar || no != (rect{}) || strings.Join(p.lines, " ") != chargedText {
		t.Errorf("buttons %v %v, lines %q", yes, no, p.lines)
	}
	a.discoveries = []discovery{{1, "x"}} // the discoveries first
	if a.charging() {
		t.Error("up over a discovery")
	}
	a.discoveries = nil
	day := a.g.Day
	a.answerCharged()
	if a.charging() || a.g.Day != day {
		t.Errorf("after OK: up %v, day %d -> %d", a.charging(), day, a.g.Day)
	}
	a.charged = true
	a.restart()
	if a.charged {
		t.Error("restart kept it")
	}
	a.charged = true
	if err := a.answerEnd(true); err != nil || a.charged {
		t.Errorf("New game kept it: %v", err)
	}
}
