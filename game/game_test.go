package game_test

import (
	"reflect"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// owners maps every planet of the cluster to its Owner, by star then index.
func owners(c *game.Cluster) [][]int {
	m := make([][]int, len(c.Stars))
	for s, st := range c.Stars {
		for _, p := range st.System.Planets {
			m[s] = append(m[s], p.Owner)
		}
	}
	return m
}

// Three empires on seed 1: distinct homes holding their species' home
// worlds, owned by their Index, every other planet −1; then bots play
// them, and no planet ever changes Owner twice nor gets a colony of two
// empires, the colonies matching the Owners day by day. 400 days: by day
// 300 only Mebes has founded (Snovemdomas' first colony is day ~335).
func TestNewGameThreeEmpires(t *testing.T) {
	sps := []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}
	g := game.NewGame(1, sps, game.Average)
	c := g.Cluster
	if len(g.Empires) != 3 || g.Empires[0].Colonies[0].Star != c.Home {
		t.Fatalf("%d empires, empire 0 not at Home %d", len(g.Empires), c.Home)
	}
	homes := map[int]bool{}
	for i, e := range g.Empires {
		col := e.Colonies[0]
		p, in := col.Planet, sps[i].Info()
		if e.Index != i || e.Species != sps[i] || e.Cluster != c || len(e.Colonies) != 1 || col.PlanetIdx != 0 ||
			p != e.Planet(col.Star, 0) || p.Name != sps[i].String()+" Prime" || p.Type != in.HomeType ||
			p.Size != in.HomeSize || p.Owner != i || p.Count(game.ColonyBase) != 1 || !e.Owns(p) {
			t.Errorf("empire %d: index %d %v at star %d: %s %v size %d owner %d", i, e.Index, e.Species, col.Star,
				p.Name, p.Type, p.Size, p.Owner)
		}
		homes[col.Star] = true
	}
	if len(homes) != 3 {
		t.Fatal("two empires share a home star")
	}
	start := owners(c)
	for s, row := range start {
		for pl, o := range row {
			if (o >= 0) != (pl == 0 && homes[s]) {
				t.Errorf("star %d planet %d owned by %d", s, pl, o)
			}
		}
	}

	bots := bot.NewAll(g, bot.Default())
	changed := map[[2]int]int{} // Owner changes per planet
	prev := start
	for range 400 {
		for _, b := range bots {
			b.Act()
		}
		g.EndDay()
		for _, b := range bots {
			b.Note()
		}
		now := owners(c)
		for s := range now {
			for pl := range now[s] {
				if now[s][pl] != prev[s][pl] {
					changed[[2]int{s, pl}]++
					if prev[s][pl] >= 0 || changed[[2]int{s, pl}] > 1 {
						t.Fatalf("day %d: star %d planet %d owner %d -> %d", g.Day, s, pl, prev[s][pl], now[s][pl])
					}
				}
			}
		}
		prev = now
		colonised := 0
		for i, e := range g.Empires {
			if e.Day != g.Day {
				t.Fatalf("empire %d day %d, game day %d", i, e.Day, g.Day)
			}
			for _, col := range e.Colonies {
				if col.Planet.Owner != i {
					t.Fatalf("day %d: empire %d's colony on %s owned by %d", g.Day, i, col.Planet.Name, col.Planet.Owner)
				}
			}
			colonised += len(e.Colonies)
		}
		owned := 0
		for _, row := range now {
			for _, o := range row {
				if o >= 0 {
					owned++
				}
			}
		}
		if owned != colonised {
			t.Fatalf("day %d: %d planets owned, %d colonies", g.Day, owned, colonised)
		}
	}
	spread := 0
	for i, e := range g.Empires {
		t.Logf("%v (empire %d): %d colonies", e.Species, i, len(e.Colonies))
		if len(e.Colonies) > 1 {
			spread++
		}
	}
	if spread < 2 {
		t.Errorf("%d empires founded a colony in %d days", spread, g.Day)
	}
}

// A one-species game is NewEmpire's empire, field for field, but the
// Relations: one in the game, none alone.
func TestNewGameOne(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	if len(g.Empires[0].Relations) != 1 || e.Relations != nil {
		t.Fatalf("relations %d in the game, %v alone", len(g.Empires[0].Relations), e.Relations)
	}
	g.Empires[0].Relations = nil
	if !reflect.DeepEqual(g.Empires[0], e) || e.Index != 0 || g.Cluster != g.Empires[0].Cluster || g.Seed != 1 {
		t.Error("NewGame of one species is not NewEmpire")
	}
}

// Game.Relink points every empire at the game's cluster, and refuses an
// empire out of order.
func TestGameRelink(t *testing.T) {
	g := game.NewGame(2, []game.Species{game.Snovemdomas, game.Orfa}, game.Sparse)
	for _, e := range g.Empires {
		e.Cluster = nil
		e.Colonies[0].Planet = nil
	}
	if err := g.Relink(); err != nil {
		t.Fatal(err)
	}
	for i, e := range g.Empires {
		col := e.Colonies[0]
		if e.Cluster != g.Cluster || col.Planet != g.Cluster.Stars[col.Star].System.Planets[0] {
			t.Errorf("empire %d not relinked", i)
		}
	}
	g.Empires[1].Index = 0
	if g.Relink() == nil {
		t.Error("two empires of Index 0 relinked")
	}
}

// PickSpecies: deterministic, first kept, distinct, all 21 at most.
func TestPickSpecies(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		for _, n := range []int{1, 2, 5, int(game.NumSpecies)} {
			sp := game.PickSpecies(seed, game.Orfa, n)
			if !slices.Equal(sp, game.PickSpecies(seed, game.Orfa, n)) || len(sp) != n || sp[0] != game.Orfa {
				t.Fatalf("seed %d n %d: %v", seed, n, sp)
			}
			seen := map[game.Species]bool{}
			for _, s := range sp {
				if seen[s] || s >= game.NumSpecies {
					t.Fatalf("seed %d n %d: %v", seed, n, sp)
				}
				seen[s] = true
			}
		}
	}
	if slices.Equal(game.PickSpecies(1, game.Orfa, 5), game.PickSpecies(2, game.Orfa, 5)) {
		t.Error("seeds 1 and 2 pick the same species")
	}
}

// PickColours puts first first, the others distinct and seeded; NewGame
// draws them with the player green; SetColour swaps, the colours staying
// distinct.
func TestColours(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		for first := range game.NumColours {
			cs := game.PickColours(seed, first, game.NumColours)
			if !slices.Equal(cs, game.PickColours(seed, first, game.NumColours)) || cs[0] != first {
				t.Fatalf("seed %d first %d: %v", seed, first, cs)
			}
			if s := slices.Sorted(slices.Values(cs)); !slices.Equal(s, []int{0, 1, 2, 3, 4, 5, 6}) {
				t.Fatalf("seed %d first %d: %v not a permutation", seed, first, cs)
			}
		}
	}
	if slices.Equal(game.PickColours(1, 0, 7), game.PickColours(2, 0, 7)) {
		t.Error("seeds 1 and 2 draw the same colours")
	}
	if cs := game.PickColours(1, game.Blue, 9); cs[7] != cs[0] || cs[8] != cs[1] {
		t.Errorf("past NumColours: %v", cs)
	}

	g := game.NewGame(3, game.PickSpecies(3, game.Orfa, 5), game.Average)
	colours := func() []int {
		var cs []int
		for _, e := range g.Empires {
			cs = append(cs, e.Colour)
		}
		return cs
	}
	if cs := colours(); !slices.Equal(cs, game.PickColours(3, game.Green, 5)) {
		t.Fatalf("NewGame colours %v", cs)
	}
	for c := range game.NumColours {
		before := colours()
		g.SetColour(0, c)
		cs := colours()
		if cs[0] != c || len(slices.Compact(slices.Sorted(slices.Values(cs)))) != 5 {
			t.Fatalf("SetColour(0, %d): %v from %v", c, cs, before)
		}
		for i := 1; i < 5; i++ {
			if cs[i] != before[i] && (before[i] != c || cs[i] != before[0]) {
				t.Errorf("SetColour(0, %d): empire %d %d -> %d", c, i, before[i], cs[i])
			}
		}
	}
}

// A Colonizer of empire 0 orbiting empire 1's home can neither be ordered
// to Colonise nor Found there: Inhabited reads Planet.Owner, any empire's.
func TestFoundRefusesAnotherEmpire(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e, other := g.Empires[0], g.Empires[1].Colonies[0]
	d := game.Design{Name: "Colony Ship", Hull: game.Small,
		Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}
	s := e.Spawn(d, other.Star, 0)
	if !e.Inhabited(other.Star, 0) || e.Owns(other.Planet) || !g.Empires[1].Owns(other.Planet) {
		t.Error("Inhabited/Owns on the other empire's home")
	}
	if e.Order(s, game.Order{Kind: game.Colonise}) == nil || e.Found(s) == nil || other.Planet.Owner != 1 || len(e.Colonies) != 1 {
		t.Error("founded on another empire's home")
	}
}

// FoundAt settles on the square given, a black one too, and refuses a
// square CanSettle refuses; the colony may start a project at once.
func TestFoundAtBlack(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	d := game.Design{Name: "Colony Ship", Hull: game.Small,
		Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}
	for i, p := range g.Cluster.Stars[home.Star].System.Planets {
		if p.Owner >= 0 {
			continue
		}
		var black []game.Cell
		for _, c := range p.Cells() {
			if p.CanSettle(c) && p.Surface[c].Kind == game.Black {
				black = append(black, c)
			}
		}
		if len(black) == 0 {
			continue
		}
		s := e.Spawn(d, home.Star, i)
		if e.FoundAt(s, game.Cell{I: -1, J: -1}) == nil {
			t.Fatal("founded off the surface")
		}
		if err := e.FoundAt(s, black[0]); err != nil {
			t.Fatal(err)
		}
		col := e.Colonies[len(e.Colonies)-1]
		if p.Surface[black[0]].Item != game.ColonyBase || p.Owner != e.Index || col.Planet != p || s.Cargo != 0 {
			t.Fatalf("square %v, owner %d, cargo %d", p.Surface[black[0]], p.Owner, s.Cargo)
		}
		if e.FoundAt(s, black[0]) == nil {
			t.Error("founded twice")
		}
		return
	}
	t.Skip("no uninhabited planet with a black square at home")
}

// The Kambuchka know every empire's home star at day 0 (explored: its
// name, planets, lanes and ships show), the others only their own.
func TestKambuchkaKnowHomes(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Kambuchka, game.Orfa}, game.Average)
	for a, e := range g.Empires {
		for b, o := range g.Empires {
			home := o.Colonies[0].Star
			if want := a == b || a == 1; e.Knows(home) != want {
				t.Errorf("empire %d knows %d's home star %d: %v, want %v", a, b, home, e.Knows(home), want)
			}
		}
	}
}

// The news box is narrow: every event text of a bot-played 3-empire
// galaxy stays short, names included (the window scrolls a longer line
// under the cursor; this guards the texts' style, not a hard limit). 50:
// "Artificial Hydroponifer built on Snovemdomas Prime", the bots' upper
// tiers.
func TestEventTextsShort(t *testing.T) {
	const most = 50
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	bots := bot.NewAll(g, bot.Default())
	n := 0
	check := func(x game.Event) {
		n++
		if k := utf8.RuneCountInString(x.Text); k > most || k == 0 {
			t.Errorf("day %d, %v: %q is %d runes, want 1 to %d", g.Day, x.Kind, x.Text, k, most)
		}
	}
	for range 600 {
		evs, reps, _ := bot.Day(g, bots)
		for _, es := range evs {
			for _, x := range es {
				check(x)
			}
		}
		for _, r := range reps {
			check(r.Event)
		}
	}
	if n < 100 {
		t.Fatalf("%d events in 600 days: a weak fixture", n)
	}
}
