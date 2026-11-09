package bot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// Hard lifts inFlightMax to hardInFlight (a larger genome's kept), saves
// with the bot (Original omitted), and colonises faster than Original
// on the same galaxy.
func TestDifficulty(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Orfa}, game.Average)
	big := Neutral()
	big.InFlight = 9
	for _, c := range []struct {
		cfg  Config
		want int
	}{
		{Config{}, inFlightCap},
		{Config{Difficulty: Hard}, hardInFlight},
		{Config{Genome: Neutral(), Difficulty: Hard}, hardInFlight},
		{Config{Genome: big, Difficulty: Hard}, 9},
		{Config{Genome: big}, 9},
	} {
		if n := New(g.Empires[0], c.cfg).inFlightMax(); n != c.want {
			t.Errorf("%v, genome %v: inFlightMax %d, want %d", c.cfg.Difficulty, c.cfg.Genome != nil, n, c.want)
		}
	}

	raw, err := json.Marshal(New(g.Empires[0], Config{Difficulty: Hard}).Save())
	if err != nil {
		t.Fatal(err)
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil || st.Cfg.Difficulty != Hard {
		t.Errorf("saved %v, %v", st.Cfg.Difficulty, err)
	}
	if raw, _ := json.Marshal(Config{}); strings.Contains(string(raw), "Difficulty") {
		t.Errorf("Original written: %s", raw)
	}
	if Hard.String() != "Hard" || Original.String() != "Original" {
		t.Errorf("names %v %v", Original, Hard)
	}

	// mksave -empires 5, seed 1 at day 710: 53 colonies Original, 65 with
	// an InFlight of 6 (measured).
	colonies := func(d Difficulty) int {
		g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 5), game.Average)
		cfg := Default()
		cfg.Difficulty = d
		bots := NewAll(g, cfg)
		for range 710 {
			Day(g, bots)
		}
		n := 0
		for _, e := range g.Empires {
			n += len(e.Colonies)
		}
		return n
	}
	if o, h := colonies(Original), colonies(Hard); h <= o {
		t.Errorf("day 710: %d colonies Hard, %d Original", h, o)
	}
}
