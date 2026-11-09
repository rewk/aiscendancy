package main

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
)

func TestParse(t *testing.T) {
	cfg, err := parseConfig("orfa", 3, 10, "Lab, agri_plot,Transport Tubes,shipyard", "none", "orbital_structures,tonklin diary", false)
	if err != nil {
		t.Fatal(err)
	}
	want := []game.Item{game.Laboratory, game.Agriplot, game.TransportTubes, game.Shipyard}
	if cfg.species != game.Orfa || cfg.greedy || len(cfg.script) != len(want) {
		t.Fatalf("%+v", cfg)
	}
	for i, it := range want {
		if cfg.script[i] != it {
			t.Errorf("script[%d] = %v, want %v", i, cfg.script[i], it)
		}
	}
	if cfg.research != "queue" || len(cfg.queue) != 2 || cfg.queue[0] != game.OrbitalStructures || cfg.queue[1] != game.TonklinDiary {
		t.Errorf("research %q %v", cfg.research, cfg.queue)
	}
	if cfg, err := parseConfig("1", 1, 1, "", "greedy", "auto", false); err != nil || cfg.species != game.Snovemdomas {
		t.Errorf("species by index: %v %v", cfg.species, err)
	}
	for _, bad := range [][3]string{{"nobody", "factory", "auto"}, {"orfa", "factory,warp", "auto"}, {"orfa", "endless party", "auto"}, {"orfa", "", "no such tech"}} {
		if _, err := parseConfig(bad[0], 1, 1, bad[1], "greedy", bad[2], false); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// Same flags, same output: the core run twice.
func TestRunDeterministic(t *testing.T) {
	cfg, err := parseConfig("Snovemdomas", 1, 200, "agriplot,laboratory,factory", "greedy", "auto", false)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	run(&a, cfg)
	run(&b, cfg)
	if a.String() != b.String() {
		t.Fatal("two runs differ")
	}
	for _, s := range []string{"Agriplot built", "Laboratory built", "Factory built", "discovered"} {
		if !strings.Contains(a.String(), s) {
			t.Errorf("no %q in the log", s)
		}
	}
}

// The galaxy mode on seed 1: deterministic, no panic, colonies founded.
// ColonyShip takes the smallest hull, so the first Small colony ship is
// complete on day 333 and the first colony founded on day 338 (seed 1);
// the run is 600 days and the day is logged.
func TestGalaxyRun(t *testing.T) {
	cfg, err := parseConfig("snovemdomas", 1, 600, "agriplot,laboratory,factory", "greedy", "auto", false)
	if err == nil {
		cfg, err = galaxyConfig(cfg, "average", false)
	}
	if err != nil {
		t.Fatal(err)
	}
	if cfg.research != "queue" || len(cfg.queue) != len(bot.GuideResearch) {
		t.Errorf("galaxy research %q %v", cfg.research, cfg.queue)
	}
	var a, b bytes.Buffer
	run(&a, cfg)
	run(&b, cfg)
	if a.String() != b.String() {
		t.Fatal("two runs differ")
	}
	first := 0
	for _, line := range strings.Split(a.String(), "\n") {
		if strings.Contains(line, "Colony founded") {
			fmt.Sscanf(line, "day %d:", &first)
			break
		}
	}
	if first == 0 {
		t.Fatalf("no colony founded in %d days", cfg.days)
	}
	t.Logf("first colony on day %d", first)
	g := runGalaxy(io.Discard, cfg)
	if len(g.g.Empires[0].Colonies) < 2 || len(g.g.Empires[0].Ships) < 1 {
		t.Errorf("%d colonies, %d ships", len(g.g.Empires[0].Colonies), len(g.g.Empires[0].Ships))
	}
	for _, s := range []string{"Colonizer I launched", "after 600 days", "known ("} {
		if !strings.Contains(a.String(), s) {
			t.Errorf("no %q in the log", s)
		}
	}
	if _, err := galaxyConfig(cfg, "crowded", false); err == nil {
		t.Error("bad density accepted")
	}
}

// -empires 3 on seed 1: three species from PickSpecies, every line
// tagged, three summaries, at least two empires spread; a save of day 400
// loaded (-load) plays days 401-500 into the same summaries as a straight
// 500-day run.
func TestGalaxyEmpires(t *testing.T) {
	cfg, err := parseConfig("snovemdomas", 1, 400, "agriplot,laboratory,factory", "greedy", "auto", false)
	if err == nil {
		cfg, err = galaxyConfig(cfg, "average", false)
	}
	if err != nil {
		t.Fatal(err)
	}
	cfg.empires = 3
	var out bytes.Buffer
	g := runGalaxy(&out, cfg)
	sps := game.PickSpecies(1, game.Snovemdomas, 3)
	spread := 0
	for i, e := range g.g.Empires {
		if e.Species != sps[i] || g.bots[i].E != e {
			t.Fatalf("empire %d: %v, want %v", i, e.Species, sps[i])
		}
		if !strings.Contains(out.String(), "\n== "+e.Species.String()+" ==\n") {
			t.Errorf("no summary for %v", e.Species)
		}
		if len(e.Colonies) > 1 {
			spread++
		}
	}
	if spread < 2 || strings.Count(out.String(), "\nafter 400 days") != 3 {
		t.Errorf("%d empires spread, %d summaries", spread, strings.Count(out.String(), "\nafter 400 days"))
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "day ") && !strings.Contains(line, ": [") {
			t.Errorf("untagged event %q", line)
		}
	}

	f := &save.File{Seed: 1, Game: g.g}
	for _, b := range g.bots {
		f.Bots = append(f.Bots, b.Save())
	}
	p := filepath.Join(t.TempDir(), "g.json")
	if err := save.Write(p, f); err != nil {
		t.Fatal(err)
	}
	loaded := cfg
	loaded.days = 100
	if loaded.from, err = save.Read(p); err != nil {
		t.Fatal(err)
	}
	var on, straight bytes.Buffer
	runGalaxy(&on, loaded)
	cfg.days = 500
	runGalaxy(&straight, cfg)
	tail := func(s string) string { return s[strings.Index(s, "\n== "):] }
	if !strings.Contains(on.String(), "loaded on day 400") || tail(on.String()) != tail(straight.String()) {
		t.Error("the loaded game's day 500 is not the straight run's")
	}
}

// -genome: refused without -galaxy, read with bot.ReadGenome, played by
// every fresh bot.
func TestGenomeFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "g.json")
	gn := bot.Neutral()
	gn.FrontierWorth = 4
	if err := bot.WriteGenome(path, gn); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig("snovemdomas", 1, 5, "agriplot,laboratory,factory", "greedy", "auto", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := genomeConfig(cfg, path); err == nil {
		t.Error("-genome accepted without -galaxy")
	}
	if c, err := genomeConfig(cfg, ""); err != nil || c.genome != nil {
		t.Errorf("no -genome: %v, %v", c.genome, err)
	}
	if cfg, err = galaxyConfig(cfg, "average", false); err != nil {
		t.Fatal(err)
	}
	if _, err := genomeConfig(cfg, path+".none"); err == nil {
		t.Error("a missing genome file accepted")
	}
	if cfg, err = genomeConfig(cfg, path); err != nil {
		t.Fatal(err)
	}
	cfg.empires = 2
	g := runGalaxy(io.Discard, cfg)
	for i, b := range g.bots {
		if b.Cfg.Genome == nil || *b.Cfg.Genome != *gn {
			t.Errorf("bot %d: genome %v", i, b.Cfg.Genome)
		}
	}
}
