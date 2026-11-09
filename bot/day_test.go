package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// DayRun: the player (no bot) at a combat star hops once a round when the
// rounds are stepped (manual), once a day under Auto, which also ends the
// star; Finish closes the day with every round's log.
func TestDayRunManual(t *testing.T) {
	setup := func() (*game.Game, *game.Starship, int) {
		g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
		atWar(t, g, 0, 1)
		star := g.Empires[0].Colonies[0].Star
		s := g.Empires[0].Spawn(game.FixtureGunboat, star, 0)
		g.Empires[1].Spawn(game.FixtureRaider, star, 0)
		if err := g.Empires[0].Order(s, game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 1, Y: 0.5}}); err != nil {
			t.Fatal(err)
		}
		return g, s, star
	}
	hops := func(s *game.Starship, step func()) int {
		n, at := 0, s.Pos
		for range 3 {
			step()
			if s.Pos != at {
				n++
			}
			at = s.Pos
		}
		return n
	}

	g, s, star := setup()
	d := Begin(g, []*Bot{nil, New(g.Empires[1], Default())})
	if st, r, ok := d.Star(); !ok || st != star || r != 0 {
		t.Fatalf("Star() = %d, %d, %v, want %d, 0, true", st, r, ok, star)
	}
	if n := hops(s, func() { d.Round(true) }); n < 2 {
		t.Errorf("stepped: %d hops in 3 rounds, want ≥ 2 (energy %d)", n, s.Power)
	}
	if st, _, ok := d.Star(); ok && st == star {
		d.Auto()
	}
	if st, _, ok := d.Star(); ok && st == star {
		t.Error("Auto left the star unfinished")
	}
	day := g.Day
	if _, _, rounds := d.Finish(); len(rounds) < 3 || g.Day != day+1 {
		t.Errorf("Finish: %d rounds, day %d → %d", len(rounds), day, g.Day)
	}

	g, s, _ = setup()
	d = Begin(g, []*Bot{nil, New(g.Empires[1], Default())})
	if n := hops(s, func() { d.Round(false) }); n != 1 {
		t.Errorf("unstepped: %d hops, want round 0's only", n)
	}
}
