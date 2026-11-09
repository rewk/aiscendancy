package main

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// duelCfg is parseConfig then setDuel, failing the test on an error.
func duelCfg(t *testing.T, seeds, species, arena string, days int, swap bool) config {
	t.Helper()
	cfg, err := parseConfig("duel", seeds, species, "vacuous", days, 4, 1)
	if err == nil {
		err = cfg.setDuel(arena, "", "", swap, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestDuelConfig(t *testing.T) {
	cfg := duelCfg(t, "1", "all", defaultArena, 10, false)
	if !cfg.duel || !slices.Equal(cfg.species, []game.Species{0, 1}) || len(cfg.arenaA) != 1 || len(cfg.arenaB) != 1 || cfg.nameA != "default" || cfg.genomeA != nil {
		t.Errorf("defaults: %+v", cfg)
	}
	if cfg = duelCfg(t, "1", "orfa,mebes", "raider:colony", 10, true); !slices.Equal(cfg.species, []game.Species{game.Orfa, game.Mebes}) || !cfg.swap {
		t.Errorf("orfa,mebes: %+v", cfg)
	}
	for _, sp := range []string{"orfa", "orfa,orfa", "orfa,mebes,1", "nobody,orfa"} {
		if _, err := parseConfig("duel", "1", sp, "average", 10, 4, 1); err == nil {
			t.Errorf("-species %q accepted", sp)
		}
	}
	cfg, _ = parseConfig("duel", "1", "all", "average", 10, 4, 1)
	for _, bad := range []string{"raider", "raider:", "x:raider"} {
		if err := cfg.setDuel(bad, "", "", false, nil); err == nil {
			t.Errorf("-arena %q accepted", bad)
		}
	}
	if err := cfg.setDuel(defaultArena, filepath.Join(t.TempDir(), "none.json"), "", false, nil); err == nil {
		t.Error("a missing -genome accepted")
	}
	solo, _ := parseConfig("solo", "1", "all", "average", 10, 4, 1)
	for _, f := range []string{"arena", "genome-b", "swap"} {
		if err := solo.setDuel(defaultArena, "", "", false, map[string]bool{f: true}); err == nil {
			t.Errorf("-%s accepted in solo", f)
		}
	}
	vs, _ := parseConfig("versus", "1", "all", "average", 10, 4, 1)
	if err := vs.setDuel(defaultArena, "", "", false, map[string]bool{"genome": true}); err == nil {
		t.Error("-genome accepted in versus")
	}
	if err := solo.setDuel(defaultArena, "", "", false, map[string]bool{"days": true}); err != nil {
		t.Errorf("solo: %v", err)
	}

	// A genome file: read, named by its path; solo's -genome too.
	p := filepath.Join(t.TempDir(), "g.json")
	gen := bot.Neutral()
	gen.FleeRatio = 1e9
	if err := bot.WriteGenome(p, gen); err != nil {
		t.Fatal(err)
	}
	gen.Clamp() // as ReadGenome
	if err := solo.setDuel(defaultArena, p, "", false, map[string]bool{"genome": true}); err != nil || solo.genomeA == nil || solo.genomeA.FleeRatio != gen.FleeRatio {
		t.Errorf("solo -genome: %v %+v", err, solo)
	}
	if err := cfg.setDuel(defaultArena, "", p, false, nil); err != nil || cfg.genomeB == nil || cfg.genomeB.FleeRatio != gen.FleeRatio || cfg.nameB != p || cfg.genomeA != nil {
		t.Errorf("-genome-b: %v %+v", err, cfg)
	}
}

// Every game is consistent: a winner keeps ships at the star and the
// loser has none there; a draw is both gone, or the cap with both there;
// the arena's ships are alive or destroyed. The swapped game is duel with
// the genomes exchanged; the output is identical whatever -j.
func TestDuel(t *testing.T) {
	for _, arena := range []string{defaultArena, "raider:colony", "colony:raider,raider"} {
		cfg := duelCfg(t, "1-3", "all", arena, 12, true)
		p := filepath.Join(t.TempDir(), "g.json")
		gen := bot.Neutral()
		gen.FleeRatio = 1e9
		if err := bot.WriteGenome(p, gen); err != nil {
			t.Fatal(err)
		}
		if err := cfg.setDuel(arena, p, "", true, nil); err != nil {
			t.Fatal(err)
		}
		res := runDuel(cfg)
		if len(res) != 6 {
			t.Fatalf("%s: %d games", arena, len(res))
		}
		n := [2]int{len(cfg.arenaA), len(cfg.arenaB)}
		for i, r := range res {
			if r.seed != cfg.seeds[i/2] || r.swapped != (i%2 == 1) {
				t.Errorf("%s game %d: seed %d swapped %v", arena, i, r.seed, r.swapped)
			}
			a, b := r.side[0], r.side[1]
			switch {
			case r.day == 0:
				if r.winner != -1 || a.atStar == 0 || b.atStar == 0 {
					t.Errorf("%s %+v: timeout", arena, r)
				}
			case r.winner == -1:
				if a.atStar != 0 || b.atStar != 0 {
					t.Errorf("%s %+v: draw before the cap", arena, r)
				}
			default:
				if w, l := r.side[r.winner], r.side[1-r.winner]; w.atStar == 0 || l.atStar != 0 || l.hull != 0 {
					t.Errorf("%s %+v: winner %d", arena, r, r.winner)
				}
			}
			if r.day > cfg.days {
				t.Errorf("%s %+v: past the cap", arena, r)
			}
			for k, s := range r.side {
				if s.alive+s.lost != n[k] || s.atStar > s.alive || s.hull < 0 {
					t.Errorf("%s %+v: side %d of %d ships", arena, r, k, n[k])
				}
			}
			want := duel(r.seed, cfg, cfg.genomeA, cfg.genomeB)
			if r.swapped {
				want = duel(r.seed, cfg, cfg.genomeB, cfg.genomeA)
				want.swapped = true
			}
			if r != want {
				t.Errorf("%s game %d: %+v, duel says %+v", arena, i, r, want)
			}
		}

		var x, y bytes.Buffer
		printDuel(&x, cfg, res)
		cfg.jobs = 4
		printDuel(&y, cfg, runDuel(cfg))
		if x.String() != y.String() {
			t.Errorf("%s: -j 1 vs -j 4:\n%s\n%s", arena, x.String(), y.String())
		}
		// header, blank, table head + 6 rows, blank, sides, genomes, hull.
		if out := x.String(); strings.Count(out, "\n") != 2+1+6+1+3 || !strings.Contains(out, "arena "+arena+", 6 games, cap 12 days") {
			t.Errorf("output:\n%s", out)
		}
	}
}

// The genome that won: the side's own, exchanged in a swapped game.
func TestGenomeWinner(t *testing.T) {
	for _, c := range []struct {
		winner  int
		swapped bool
		want    int
	}{{0, false, 0}, {1, false, 1}, {-1, false, -1}, {0, true, 1}, {1, true, 0}, {-1, true, -1}} {
		if got := (duelResult{winner: c.winner, swapped: c.swapped}).genomeWinner(); got != c.want {
			t.Errorf("%+v: %d", c, got)
		}
	}
}

// runDuel's swapped game gives -genome to empire 1, -genome-b to empire 0.
func TestDuelGenomes(t *testing.T) {
	cfg := config{genomeA: bot.Neutral(), genomeB: bot.Neutral()}
	if a, b := cfg.duelGenomes(false); a != cfg.genomeA || b != cfg.genomeB {
		t.Error("unswapped")
	}
	if a, b := cfg.duelGenomes(true); a != cfg.genomeB || b != cfg.genomeA {
		t.Error("swapped")
	}
}

// A kill is counted on the loser's side: B's Raider set to one hull point,
// shields off, on A's Raider, which never flees. That the bot fires on it
// is the bot's (a sitting target); the counting is play's.
func TestPlayKill(t *testing.T) {
	g := game.NewGame(1, []game.Species{0, game.Orfa}, game.Vacuous)
	a, b, err := game.ParseArena("raider:raider")
	if err != nil {
		t.Fatal(err)
	}
	if err := game.SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	foe := g.Empires[1].Ships[0]
	foe.Hull, foe.Shields, foe.Pos = 1, false, g.Empires[0].Ships[0].Pos
	gen := bot.Neutral()
	gen.FleeRatio = 1e9
	var bots []*bot.Bot
	for _, e := range g.Empires {
		bc := bot.Default()
		bc.Genome = gen
		bots = append(bots, bot.New(e, bc))
		bots[len(bots)-1].G = g
	}
	r := play(g, bots, 5)
	if r.side[1].lost != 1 || r.side[1].alive != 0 || r.side[0].lost != 0 || r.winner != 0 || r.day == 0 || r.side[1].hull != 0 {
		t.Errorf("%+v", r)
	}
}

// TestDuelWarHolds: the arena's War survives the bots' Act (NoDiplomacy;
// else a fresh war without grudge was offered peace and accepted on day
// 0): lancers, which outgun raiders, destroy one within the default 30
// days, default genomes.
func TestDuelWarHolds(t *testing.T) {
	cfg, err := parseConfig("duel", "1", "orfa,mebes", "average", duelDays, 4, 1)
	if err == nil {
		err = cfg.setDuel("lancer,lancer:raider,raider", "", "", false, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	if r := runDuel(cfg)[0]; r.side[1].lost == 0 {
		t.Errorf("no raider destroyed: %+v", r)
	}
}

// The corvette mirror fights: each side's damage ledger (Sighting.Dealt)
// lowers its estimate of the unscanned foe's hull after the first
// exchange (without it both overrated the foe's untouched hull and fled
// to the rim: 16 timeouts, nothing destroyed). A mirror still ends in
// draws (the two die the same round).
func TestDuelCorvetteMirror(t *testing.T) {
	cfg, err := parseConfig("duel", "1-8", "orfa,mebes", "average", duelDays, 4, 6)
	if err == nil {
		err = cfg.setDuel("corvette:corvette", "", "", true, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	timeouts, kills := 0, 0
	for _, r := range runDuel(cfg) {
		if r.day == 0 {
			timeouts++
		}
		kills += r.side[0].lost + r.side[1].lost
	}
	if timeouts > 0 || kills == 0 {
		t.Errorf("%d timeouts of 16, %d ships destroyed", timeouts, kills)
	}
}
