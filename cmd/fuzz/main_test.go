package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

func TestParseSeeds(t *testing.T) {
	for in, want := range map[string][]uint64{
		"1-10":     {1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		"1,4,7":    {1, 4, 7},
		"1-3,9":    {1, 2, 3, 9},
		" 5 , 2-2": {5, 2},
		"0":        {0},
	} {
		if got, err := parseSeeds(in); err != nil || !slices.Equal(got, want) {
			t.Errorf("parseSeeds(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", ",", "x", "1-", "-3", "3-1", "1-x", "1,,y", "1.5", "-1"} {
		if got, err := parseSeeds(bad); err == nil {
			t.Errorf("parseSeeds(%q) = %v accepted", bad, got)
		}
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig("solo", "1-2", "all", "average", 10, 4, 1)
	if err != nil || len(cfg.species) != int(game.NumSpecies) || cfg.species[3] != game.Species(3) {
		t.Fatalf("all: %v %v", cfg.species, err)
	}
	if cfg, err = parseConfig("solo", "1", "orfa,1", "dense", 10, 4, 1); err != nil || !slices.Equal(cfg.species, []game.Species{game.Orfa, game.Snovemdomas}) || cfg.density != game.Dense {
		t.Fatalf("list: %+v %v", cfg, err)
	}
	for _, bad := range [][3]string{{"brawl", "1", "all"}, {"solo", "x", "all"}, {"solo", "1", "nobody"}, {"solo", "1", ","}, {"versus", "1", "orfa"}} {
		if _, err := parseConfig(bad[0], bad[1], bad[2], "average", 10, 4, 1); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, e := range []int{1, int(game.NumSpecies) + 1} {
		if _, err := parseConfig("versus", "1", "all", "average", 10, e, 1); err == nil {
			t.Errorf("-empires %d accepted", e)
		}
	}
	if _, err := parseConfig("versus", "1", "all", "vacuous", 10, game.Vacuous.Stars()+1, 1); err == nil {
		t.Error("more empires than stars accepted")
	}
}

// The output is byte-identical whatever the worker count.
func TestDeterministicAcrossWorkers(t *testing.T) {
	cfg, err := parseConfig("solo", "1-2", "0,orfa", "vacuous", 400, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	printSolo(&a, cfg, runSolo(cfg))
	cfg.jobs = 3
	printSolo(&b, cfg, runSolo(cfg))
	if a.String() != b.String() {
		t.Errorf("solo -j 1 vs -j 3:\n%s\n%s", a.String(), b.String())
	}

	if cfg, err = parseConfig("versus", "1,2", "all", "vacuous", 200, 2, 1); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	b.Reset()
	printVersus(&a, cfg, runVersus(cfg))
	cfg.jobs = 3
	printVersus(&b, cfg, runVersus(cfg))
	if a.String() != b.String() {
		t.Errorf("versus -j 1 vs -j 3:\n%s\n%s", a.String(), b.String())
	}
	if !strings.HasPrefix(a.String(), "versus: Vacuous, 2 games of 2 empires, 200 days\n") {
		t.Errorf("versus header:\n%s", a.String())
	}
	sps := map[game.Species]bool{}
	for _, r := range runVersus(cfg) {
		for _, e := range r.ranked {
			sps[e.species] = true
		}
	}
	// header, blank, table head + one row per game, blank, head + species,
	// blank, the conflict, alliance and victory lines.
	if n := strings.Count(a.String(), "\n"); n != 2+1+len(cfg.seeds)+1+1+len(sps)+4 {
		t.Errorf("versus output %d lines:\n%s", n, a.String())
	}
}

// Milestones against a replay of the same game: the day each milestone
// is first met, the stop on the win, the final systems. The cap is set
// between the two seeds' win days (from an uncapped first pass), so one
// run wins and the other hits the cap whatever the bot's strength.
// Seeds 3-4 win on days 2440 and 1794 (measured with the pace
// FoundFloor 4 InFlight 2); seed 1 never leaves its home system there.
func TestMilestones(t *testing.T) {
	cfg, err := parseConfig("solo", "3-4", "0", "vacuous", 3000, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	first := runSolo(cfg)
	a, b := first[0].mile[2], first[1].mile[2]
	if a == 0 || b == 0 || a == b {
		t.Fatalf("uncapped win days %d, %d: need two distinct wins", a, b)
	}
	cfg.days = (a + b) / 2
	res := runSolo(cfg)
	reached := 0
	for _, r := range res {
		g := game.NewGame(r.seed, []game.Species{r.species}, cfg.density)
		if r.stars != len(g.Cluster.Stars) {
			t.Fatalf("seed %d: %d stars, the game has %d", r.seed, r.stars, len(g.Cluster.Stars))
		}
		prev := 0
		for k, d := range r.mile {
			if d != 0 && d < prev {
				t.Errorf("seed %d: milestones %v decrease", r.seed, r.mile)
			}
			if d != 0 {
				prev = d
			}
			if d == 0 && k < 2 && r.mile[k+1] != 0 {
				t.Errorf("seed %d: milestone %d missed, %d met: %v", r.seed, k+1, k+2, r.mile)
			}
		}
		if r.mile[2] != 0 {
			reached++
			if r.day != r.mile[2] || r.systems < need(3, r.stars) {
				t.Errorf("seed %d: won on day %d but stopped on %d with %d/%d", r.seed, r.mile[2], r.day, r.systems, r.stars)
			}
		} else if r.day != cfg.days {
			t.Errorf("seed %d: no win but stopped on day %d", r.seed, r.day)
		}

		// Replay: each milestone's day is the first day the count meets it.
		bots := bot.NewAll(g, bot.Default())
		e := g.Empires[0]
		var mile [3]int
		last := 0
		for g.Day < r.day {
			before := e.ControlledSystems()
			bot.Day(g, bots)
			s := e.ControlledSystems()
			if s > before {
				last = g.Day
			}
			for k := range mile {
				if mile[k] == 0 && s >= need(k+1, len(g.Cluster.Stars)) {
					mile[k] = g.Day
				}
			}
		}
		if mile != r.mile || last != r.lastGrowth || e.ControlledSystems() != r.systems || len(e.Colonies) != r.colonies {
			t.Errorf("seed %d: run %+v, replay mile %v last %d systems %d colonies %d", r.seed, r, mile, last, e.ControlledSystems(), len(e.Colonies))
		}
	}
	if reached == 0 || reached == len(res) {
		t.Errorf("%d of %d runs won: both ends of the stop rule must be walked", reached, len(res))
	}
}

func TestNeed(t *testing.T) {
	for _, c := range []struct{ k, n, want int }{{1, 15, 4}, {2, 15, 8}, {3, 15, 11}, {1, 100, 25}, {3, 50, 34}, {3, 51, 35}, {2, 25, 13}} {
		if got := need(c.k, c.n); got != c.want {
			t.Errorf("need(%d, %d) = %d, want %d", c.k, c.n, got, c.want)
		}
	}
}

func TestMedian(t *testing.T) {
	for _, c := range []struct {
		xs   []int
		want int
	}{{nil, 0}, {[]int{7}, 7}, {[]int{9, 3}, 3}, {[]int{5, 1, 3}, 3}, {[]int{40, 10, 30, 20}, 20}} {
		if got := median(c.xs); got != c.want {
			t.Errorf("median(%v) = %d, want %d", c.xs, got, c.want)
		}
	}
}

// The summaries' arithmetic on handcrafted results.
func TestSummaries(t *testing.T) {
	cfg := config{density: game.Average, days: 3000}
	sp0, sp1 := game.Species(0), game.Species(1)
	res := []soloResult{
		{species: sp1, seed: 1, stars: 50, mile: [3]int{100, 200, 900}, systems: 38},
		{species: sp1, seed: 2, stars: 50, mile: [3]int{100, 200, 300}, systems: 40},
		{species: sp1, seed: 3, stars: 50, mile: [3]int{100, 200, 0}, systems: 10, lastGrowth: 777, day: 3000},
		{species: sp1, seed: 4, stars: 50, mile: [3]int{100, 200, 600}, systems: 40},
		{species: sp0, seed: 1, stars: 50, systems: 1, day: 3000},
	}
	var b bytes.Buffer
	printSolo(&b, cfg, res)
	out := b.String()
	if !strings.HasPrefix(out, "solo: Average, 5 runs, cap 3000 days\n") {
		t.Errorf("header:\n%s", out)
	}
	// The stalled run prints its lastGrowth, the reached ones "-".
	if rows := strings.Split(out, "\n"); strings.Fields(rows[5])[8] != "777" || strings.Fields(rows[3])[8] != "-" {
		t.Errorf("stalled column:\n%s", out)
	}
	// Summary in first-run order. sp1: reached 900, 300, 600 -> sorted,
	// odd count: 600; share (38+40+10+40)/50/4 = 64%. sp0: 0/1, -, 1/50 = 2%.
	sum := out[strings.LastIndex(out, "species"):]
	lines := strings.Split(strings.TrimSpace(sum), "\n")
	if len(lines) != 3 {
		t.Fatalf("summary rows:\n%s", sum)
	}
	for i, want := range [][]string{
		{sp1.String(), "4", "3/4", "600", "64%"},
		{sp0.String(), "1", "0/1", "-", "2%"},
	} {
		if got := strings.Fields(lines[i+1]); !slices.Equal(got, want) {
			t.Errorf("solo row %d = %v, want %v", i+1, got, want)
		}
	}

	vcfg := config{versus: true, density: game.Vacuous, days: 100, empires: 2}
	vres := []versusResult{
		{seed: 1, stars: 10, ranked: []versusEmpire{{species: sp0, systems: 5}, {species: sp1, systems: 1}}},
		{seed: 2, stars: 10, ranked: []versusEmpire{{species: sp1, systems: 4}, {species: sp0, systems: 2}}},
		{seed: 3, stars: 10, ranked: []versusEmpire{{species: sp1, systems: 3}, {species: game.Species(5), systems: 3}}},
	}
	b.Reset()
	printVersus(&b, vcfg, vres)
	out = b.String()
	sum = out[strings.LastIndex(out, "species"):strings.LastIndex(out, "\nconflict")]
	if c := out[strings.LastIndex(out, "conflict"):]; !strings.HasPrefix(c, "conflict per game: wars 0.00") {
		t.Errorf("conflict line %q", c)
	}
	// Species order. sp0: 2 games, 1 win, ranks 1,2 -> 1.5, share (5+2)/20 = 35%.
	// sp1: 3 games, 2 wins, ranks 2,1,1 -> 1.3, share (1+4+3)/30 = 27%.
	lines = strings.Split(strings.TrimSpace(sum), "\n")
	if len(lines) != 4 {
		t.Fatalf("summary rows:\n%s", sum)
	}
	for i, want := range [][]string{
		{sp0.String(), "2", "1", "1.5", "35%"},
		{sp1.String(), "3", "2", "1.3", "27%"},
		{game.Species(5).String(), "1", "0", "2.0", "30%"},
	} {
		if got := strings.Fields(lines[i+1]); !slices.Equal(got, want) {
			t.Errorf("row %d = %v, want %v", i+1, got, want)
		}
	}
}

func TestRank(t *testing.T) {
	es := []versusEmpire{
		{index: 0, systems: 2, pop: 5},
		{index: 1, systems: 3, pop: 1},
		{index: 2, systems: 2, pop: 9},
		{index: 3, systems: 2, pop: 5},
	}
	rank(es)
	var got []int
	for _, e := range es {
		got = append(got, e.index)
	}
	if want := []int{1, 2, 0, 3}; !slices.Equal(got, want) {
		t.Errorf("rank order %v, want %v", got, want)
	}
}
