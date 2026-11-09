package game_test

import (
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// oracle is e's totals today, recomputed here from the model's own
// functions (Colony.Shown, Design.Cost, Knows, StanceWith...).
func oracle(g *game.Game, e *game.Empire) game.Sample {
	s := game.Sample{Day: g.Day, Colonies: len(e.Colonies), Ships: len(e.Ships), Systems: e.ControlledSystems()}
	stars := map[int]bool{}
	for _, c := range e.Colonies {
		ind, res, pro := c.Shown()
		s.Pop, s.Industry, s.Research, s.Prosperity = s.Pop+c.Pop, s.Industry+ind, s.Research+res, s.Prosperity+pro
		stars[c.Star] = true
	}
	s.Stars = len(stars)
	for _, sh := range e.Ships {
		s.Fleet += sh.Design.Cost()
	}
	for t := range game.NumTechs {
		if e.Research.Known[t] {
			s.Techs++
		}
	}
	for star := range g.Cluster.Stars {
		if e.Knows(star) {
			s.Explored++
		}
	}
	for j := range g.Empires {
		if j != e.Index && e.StanceWith(j) == game.War {
			s.Wars++
		}
	}
	return s
}

// Three bot empires on seed 1 for 420 days (colonies founded from ~335):
// no sample before the first EndDay, then one at day 0 and at every
// HistoryEvery-th day's close, each equal to the oracle that day; an
// empire defeated (Lost set) stops sampling, the others go on.
func TestHistory(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	bots := bot.NewAll(g, bot.Default())
	for _, e := range g.Empires {
		if len(e.History) != 0 {
			t.Fatalf("empire %d: %d samples before any day", e.Index, len(e.History))
		}
	}
	want := make([]game.Sample, len(g.Empires)) // day 0's, before the day runs
	for i, e := range g.Empires {
		want[i] = oracle(g, e)
	}
	grew := false
	for g.Day < 420 {
		before := g.Day
		bot.Day(g, bots)
		for i, e := range g.Empires {
			n := g.Day/game.HistoryEvery + 1 // day 0's, then one per period closed
			if len(e.History) != n {
				t.Fatalf("day %d empire %d: %d samples, want %d", g.Day, i, len(e.History), n)
			}
			if before == 0 && e.History[0] != want[i] {
				t.Errorf("empire %d day 0: %+v, want %+v", i, e.History[0], want[i])
			}
			if g.Day%game.HistoryEvery != 0 {
				continue
			}
			if got, w := e.History[n-1], oracle(g, e); got != w {
				t.Errorf("day %d empire %d: %+v, want %+v", g.Day, i, got, w)
			}
			grew = grew || len(e.Colonies) > 1
		}
	}
	if !grew {
		t.Fatal("no colony founded by day 420: the samples walked nothing but homes")
	}
	dead := g.Empires[2]
	dead.Lost = g.Day
	keep := len(dead.History)
	for range game.HistoryEvery {
		g.EndDay()
	}
	if len(dead.History) != keep {
		t.Errorf("a defeated empire sampled: %d samples, want %d", len(dead.History), keep)
	}
	if n := g.Day/game.HistoryEvery + 1; len(g.Empires[0].History) != n {
		t.Errorf("a live empire: %d samples, want %d", len(g.Empires[0].History), n)
	}
}
