package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/save"
)

// newCfg is main's defaults: seed 1, average, Snovemdomas, 3 empires,
// the player's bot on.
func newCfg(out string) config {
	return config{seed: 1, density: game.Average, species: game.Snovemdomas, empires: 3, useBot: true, out: out}
}

func mustRun(t *testing.T, cfg config) *save.File {
	t.Helper()
	var sb strings.Builder
	if err := run(&sb, cfg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "-> "+cfg.out) {
		t.Errorf("line %q", sb.String())
	}
	f, err := save.Read(cfg.out)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// 100 days written are the state the window's loop makes: the same Game
// and bots, JSON byte for byte, and a log of the player's news.
func TestDays(t *testing.T) {
	dir := t.TempDir()
	x := filepath.Join(dir, "x.json")
	cfg := newCfg(x)
	cfg.days = 100
	f := mustRun(t, cfg)
	if f.Game.Day != 100 || len(f.Game.Empires) != 3 || f.Seed != 1 {
		t.Fatalf("day %d, %d empires, seed %d", f.Game.Day, len(f.Game.Empires), f.Seed)
	}
	if len(f.Log) == 0 || f.Log[len(f.Log)-1].Day > 100 {
		t.Fatalf("log %d entries", len(f.Log))
	}

	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	bots := bot.NewAll(g, bot.Default())
	var lg []save.Entry
	for range 100 {
		evs, reps, _ := bot.Day(g, bots)
		for _, r := range reps {
			if r.Empire == 0 {
				lg = append(lg, save.Entry{Day: g.Day, Event: r.Event})
			}
		}
		for _, x := range evs[0] {
			lg = append(lg, save.Entry{Day: g.Day, Event: x})
		}
	}
	var sts []*bot.State
	for _, b := range bots {
		sts = append(sts, b.Save())
	}
	for _, c := range []struct {
		name      string
		got, want any
	}{{"game", f.Game, g}, {"bots", f.Bots, sts}, {"log", f.Log, lg}} {
		gb, err := json.Marshal(c.got)
		if err != nil {
			t.Fatal(err)
		}
		wb, err := json.Marshal(c.want)
		if err != nil {
			t.Fatal(err)
		}
		if string(gb) != string(wb) {
			t.Errorf("%s: the save differs from the window's loop", c.name)
		}
	}

	// -load x -days 50: day 150, x's log first.
	y := filepath.Join(dir, "y.json")
	cfg = newCfg(y)
	cfg.load, cfg.days = x, 50
	f2 := mustRun(t, cfg)
	if f2.Game.Day != 150 {
		t.Errorf("loaded +50: day %d", f2.Game.Day)
	}
	if len(f2.Log) < len(f.Log) || !reflect.DeepEqual(f2.Log[:len(f.Log)], f.Log) {
		t.Errorf("the loaded log does not start with x's (%d then %d entries)", len(f.Log), len(f2.Log))
	}
	if f2.Bots[0] == nil {
		t.Error("the player's bot lost on load")
	}
}

// The fixtures: -foes needs a second empire; -ships puts colony ships in
// orbit of the player's home.
func TestFixtures(t *testing.T) {
	dir := t.TempDir()
	cfg := newCfg(filepath.Join(dir, "foes.json"))
	cfg.empires, cfg.foes = 1, 1
	if err := run(io.Discard, cfg); err == nil {
		t.Error("-foes 1 -empires 1 accepted")
	}
	cfg = newCfg(filepath.Join(dir, "ships.json"))
	cfg.ships = 2
	f := mustRun(t, cfg)
	e := f.Game.Empires[0]
	home := e.Colonies[0]
	if len(e.Ships) != 2 {
		t.Fatalf("%d ships", len(e.Ships))
	}
	for _, s := range e.Ships {
		if !reflect.DeepEqual(s.Design, game.FixtureColonyShip) || s.Star != home.Star || s.Orbit != home.PlanetIdx {
			t.Errorf("%s: %s at %d/%d, home %d/%d", s.Name, s.Design.Name, s.Star, s.Orbit, home.Star, home.PlanetIdx)
		}
	}
	for i := 1; i < len(f.Game.Empires); i++ {
		if n := len(f.Game.Empires[i].Ships); n != 0 {
			t.Errorf("empire %d: %d ships", i, n)
		}
	}
}

// -o is required; -bot=false saves no bot for the player; -colour is the
// player's.
func TestFlags(t *testing.T) {
	if err := run(io.Discard, newCfg("")); err == nil {
		t.Error("no -o accepted")
	}
	cfg := newCfg(filepath.Join(t.TempDir(), "nobot.json"))
	cfg.useBot, cfg.colour = false, game.Orange
	f := mustRun(t, cfg)
	if f.Bots[0] != nil || f.Bots[1] == nil {
		t.Errorf("bots %v", f.Bots)
	}
	if c := f.Game.Empires[0].Colour; c != game.Orange {
		t.Errorf("the player's colour %d, want %d", c, game.Orange)
	}
	cfg.empires = 22
	if err := run(io.Discard, cfg); err == nil {
		t.Error("-empires 22 accepted")
	}
}

// -genome: every fresh bot plays it, and the save keeps it.
func TestGenome(t *testing.T) {
	cfg := newCfg(filepath.Join(t.TempDir(), "genome.json"))
	cfg.genome = bot.Neutral()
	cfg.genome.FrontierWorth = 4
	cfg.days = 5
	f := mustRun(t, cfg)
	for i, st := range f.Bots {
		if st == nil || st.Cfg.Genome == nil || *st.Cfg.Genome != *cfg.genome {
			t.Errorf("bot %d: %+v", i, st)
		}
	}
}

// -arena: spawned after the fixtures, as game.SpawnArena places it on a
// fresh game; refused with one empire, with -load, on a bad spec.
func TestArena(t *testing.T) {
	dir := t.TempDir()
	cfg := newCfg(filepath.Join(dir, "arena.json"))
	cfg.ships, cfg.arena = 1, "raider:raider,colony"
	f := mustRun(t, cfg)

	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	if err := game.SpawnFixtures(g, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	a, b, err := game.ParseArena(cfg.arena)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.SpawnArena(g, a, b); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		got, _ := json.Marshal(f.Game.Empires[i].Ships)
		want, _ := json.Marshal(g.Empires[i].Ships)
		if string(got) != string(want) {
			t.Errorf("empire %d ships:\n%s\nwant\n%s", i, got, want)
		}
	}
	if n := len(f.Game.Empires[0].Ships); n != 2 {
		t.Errorf("empire 0: %d ships", n)
	}

	for _, bad := range []func(*config){
		func(c *config) { c.empires = 1 },
		func(c *config) { c.load = cfg.out },
		func(c *config) { c.arena = "raider:" },
		func(c *config) { c.arena = "raider:gunboat" },
	} {
		c := newCfg(filepath.Join(dir, "bad.json"))
		c.arena = "raider:raider"
		bad(&c)
		if err := run(io.Discard, c); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
}

// -scenario: staged on the new game as game.ApplyScenario stages it,
// written and read back with its colonies and ships; refused with -load,
// with -arena, on a bad file.
func TestScenario(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "scen.json")
	const js = `{"planets": [{"planet": 1, "empire": 2, "pop": 3, "orbit": ["Orbital Shield"]}],
		"ships": [{"empire": 0, "design": "cruiser", "at": "orbit 0"}, {"empire": 2, "design": "frigate", "count": 2, "at": "lane 0"}]}`
	if err := os.WriteFile(spec, []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := newCfg(filepath.Join(dir, "out.json"))
	cfg.scenario = spec
	f := mustRun(t, cfg)

	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	s, err := game.ParseScenario(strings.NewReader(js))
	if err != nil {
		t.Fatal(err)
	}
	if err := game.ApplyScenario(g, s); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(f.Game)
	want, _ := json.Marshal(g)
	if string(got) != string(want) {
		t.Error("the save is not the staged game")
	}
	e2 := f.Game.Empires[2]
	if len(e2.Colonies) != 2 || e2.Colonies[1].Pop != 3 || len(e2.Ships) != 2 || len(f.Game.Empires[0].Ships) != 1 ||
		f.Game.Empires[0].StanceWith(2) != game.War {
		t.Errorf("empire 2: %d colonies, %d ships; empire 0: %d ships, stance %v", len(e2.Colonies), len(e2.Ships),
			len(f.Game.Empires[0].Ships), f.Game.Empires[0].StanceWith(2))
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"ships": [], "fleet": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, mod := range []func(*config){
		func(c *config) { c.load = cfg.out },
		func(c *config) { c.arena = "raider:raider" },
		func(c *config) { c.scenario = bad },
		func(c *config) { c.scenario = filepath.Join(dir, "none.json") },
		func(c *config) { c.empires = 2 }, // empire 2 named
	} {
		c := newCfg(filepath.Join(dir, "refused.json"))
		c.scenario = spec
		mod(&c)
		if err := run(io.Discard, c); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "refused.json")); err == nil {
		t.Error("a refused scenario wrote a save")
	}
}

// Every file in scenarios/ applies to mksave's default new game (seed 1,
// average, 3 empires): each empire's ships as many as the file asks.
func TestScenarioFiles(t *testing.T) {
	files, err := filepath.Glob("../../scenarios/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenario files: %v", err)
	}
	for _, path := range files {
		fh, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		s, err := game.ParseScenario(fh)
		fh.Close()
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		cfg := newCfg(filepath.Join(t.TempDir(), "x.json"))
		cfg.scenario = path
		var sb strings.Builder
		if err := run(&sb, cfg); err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		f, err := save.Read(cfg.out)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]int, len(f.Game.Empires))
		for _, sh := range s.Ships {
			want[sh.Empire] += max(sh.Count, 1)
		}
		for i, e := range f.Game.Empires {
			if len(e.Ships) != want[i] {
				t.Errorf("%s: empire %d has %d ships, want %d", path, i, len(e.Ships), want[i])
			}
		}
	}
}
