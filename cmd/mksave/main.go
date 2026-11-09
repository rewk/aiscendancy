// mksave makes a save game offline, headless (no Ebiten: no display
// needed), for cmd/aiscendancy -load: a new game (or a loaded one), the
// fixture ships spawned, days played by the bots, written with package
// save. The window's fixtures and fast-forward live here, not in the
// window.
//
//	mksave -o game.json                          a new game, day 0: seed 1, average density, 3 empires
//	mksave -seed 7 -density dense -o game.json   or "very dense", or 0..4
//	mksave -species orfa -empires 5 -o game.json the player's species, five empires on the cluster
//	mksave -colour blue -o game.json             the player's colour, the others' drawn by the seed
//	mksave -days 340 -o game.json                on day 340, the bots having played
//	mksave -bot=false -o game.json               no bot for the player's empire (the others always have one)
//	mksave -ships 2 -o game.json                 two free colony ships in orbit of the player's home
//	mksave -warships 1 -foes 2 -o game.json      a free Gunboat at home, two of empire 1's Raiders in its orbit
//	mksave -arena raider,raider:raider -o a.json a fight at the player's home: side A (empire 0) in orbit, side B (empire 1) at the first lane's endpoint
//	mksave -scenario scenarios/fleets.json -o f.json  one system staged (game.ParseScenario): fleets of archetypes at the player's home
//	mksave -scenario scenarios/siege.json -o s.json   empire 1's armed planet at star 8 (seed 1, average), empire 0's fleet by it
//	mksave -scenario scenarios/threeway.json -o t.json  star 33 (seed 1, average, -empires >= 3): 1 and 2 allied, at war with 0
//	mksave -load game.json -days 50 -o later.json  50 more days of a save (-seed -density -species -colour -empires -bot ignored)
//	mksave -genome g.json -days 340 -o game.json every fresh bot plays this genome (cmd/fuzz -o); a loaded save's bots keep theirs
//	mksave -days 340 -journal a.jsonl -o game.json  every action of the -days appended to a.jsonl (package journal)
//
// -arena takes game.ParseArena's spec (designs raider, lancer, colony and
// the archetypes corvette, frigate, cruiser, battleship) and
// game.SpawnArena's placement; it needs -empires >= 2 and is refused with
// -load (a save's home may hold anything).
//
// -scenario takes game.ParseScenario's JSON (one star, its planets'
// owners, orbit and surface structures, pops, arena ships placed in
// orbit, by a lane or by a planet, the stances) and game.ApplyScenario's
// staging; refused with -load and with -arena. Its planet and star
// indices are the cluster's: the files in scenarios/ are seed 1's,
// average density.
//
// The order is: the game (new or loaded) and its journal (-journal: the
// fixtures are setup, not journalled actions: they record nothing), the
// fixtures spawned, the arena spawned or the scenario staged, -days
// days of bot.Day, each day's player reports (empire 0's) then its events
// appended to the log stamped with the day that closes, as the window
// does (the last save.LogKeep kept), then save.Write.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/save"
)

// config is the flags, parsed.
type config struct {
	seed                   uint64
	density                game.Density
	species                game.Species
	colour                 int
	empires                int
	useBot                 bool
	days                   int
	ships, warships, foes  int
	load, out              string
	arena                  string          // game.ParseArena's spec, "" none
	scenario               string          // game.ParseScenario's file, "" none
	genome                 *bot.Genome     // every fresh bot's Config.Genome (-genome), nil: bot.Default's fixed policy
	journal                *journal.Writer // the action journal (-journal), nil none
	setSeed, setDensity    bool            // given on the command line: said ignored under -load
	setSpecies, setEmpires bool
	setBot, setColour      bool
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("mksave: ")
	seed := flag.Uint64("seed", 1, "cluster seed")
	density := flag.String("density", "average", `vacuous, sparse, average, dense, "very dense", or 0..4`)
	species := flag.String("species", "Snovemdomas", "the player's species, by name or index")
	colour := flag.String("colour", "green", "the player's colour (green, purple, yellow, pink, brown, blue, orange, or 0..6): the others' are drawn by the seed")
	empires := flag.Int("empires", 3, "empires on the cluster, 1..21: the player's species then others drawn by the seed")
	useBot := flag.Bool("bot", true, "a bot plays the player's empire 0 too (the other empires always have one)")
	days := flag.Int("days", 0, "play N days, the bots playing")
	ships := flag.Int("ships", 0, "fixture: this many colony ships (lane drive, two Colonizers) in orbit of the player's home world")
	warships := flag.Int("warships", 0, "fixture: this many Gunboats (a gun, a shield) in orbit of the player's home world")
	foes := flag.Int("foes", 0, "fixture: this many Raiders of empire 1 in orbit of the player's home world; needs -empires >= 2")
	load := flag.String("load", "", "start from this save: -seed, -density, -species, -colour, -empires, -bot are ignored")
	out := flag.String("o", "", "where to write the save (required)")
	arena := flag.String("arena", "", "fixture: a fight at the player's home, side A:side B (raider,colony:raider; designs raider, lancer, colony, corvette, frigate, cruiser, battleship); needs -empires >= 2, refused with -load")
	scenario := flag.String("scenario", "", "fixture: stage one system from this JSON file (game.ParseScenario: planets, ships, stances); refused with -load and -arena")
	genome := flag.String("genome", "", "every fresh bot plays this genome (cmd/fuzz -o); a loaded save's bots keep their own")
	journalPath := flag.String("journal", "", "append every action the bots take over -days to this JSON Lines file (package journal)")
	flag.Parse()

	cfg := config{seed: *seed, empires: *empires, useBot: *useBot, days: *days,
		ships: *ships, warships: *warships, foes: *foes, load: *load, out: *out, arena: *arena, scenario: *scenario}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "seed":
			cfg.setSeed = true
		case "density":
			cfg.setDensity = true
		case "species":
			cfg.setSpecies = true
		case "empires":
			cfg.setEmpires = true
		case "bot":
			cfg.setBot = true
		case "colour":
			cfg.setColour = true
		}
	})
	var err error
	if cfg.density, err = game.ParseDensity(*density); err != nil {
		log.Fatal(err)
	}
	if cfg.species, err = game.ParseSpecies(*species); err != nil {
		log.Fatal(err)
	}
	if cfg.colour, err = game.ParseColour(*colour); err != nil {
		log.Fatal(err)
	}
	if *genome != "" {
		if cfg.genome, err = bot.ReadGenome(*genome); err != nil {
			log.Fatal(err)
		}
	}
	if *journalPath != "" {
		jf, err := os.OpenFile(*journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		defer jf.Close()
		cfg.journal = journal.New(jf)
	}
	if err := run(os.Stdout, cfg); err != nil {
		log.Fatal(err)
	}
}

// run makes the save cfg describes and prints its one line to w.
func run(w io.Writer, cfg config) error {
	switch {
	case cfg.out == "":
		return errors.New("-o: no output path")
	case cfg.days < 0 || cfg.ships < 0 || cfg.warships < 0 || cfg.foes < 0:
		return errors.New("-days, -ships, -warships, -foes: negative")
	}
	var arenaA, arenaB []game.Design
	if cfg.arena != "" {
		if cfg.load != "" {
			return errors.New("-arena with -load: the arena is a new game's")
		}
		if cfg.empires < 2 {
			return fmt.Errorf("-arena needs -empires >= 2 (%d)", cfg.empires)
		}
		var err error
		if arenaA, arenaB, err = game.ParseArena(cfg.arena); err != nil {
			return err
		}
	}
	var scen *game.Scenario
	if cfg.scenario != "" {
		switch {
		case cfg.load != "":
			return errors.New("-scenario with -load: the scenario is a new game's")
		case cfg.arena != "":
			return errors.New("-scenario with -arena: one staging at a time")
		}
		fh, err := os.Open(cfg.scenario)
		if err != nil {
			return err
		}
		scen, err = game.ParseScenario(fh)
		fh.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", cfg.scenario, err)
		}
	}
	var f *save.File
	var bots []*bot.Bot
	fresh := bot.Default()
	fresh.Genome = cfg.genome
	if cfg.load != "" {
		if cfg.setSeed || cfg.setDensity || cfg.setSpecies || cfg.setColour || cfg.setEmpires || cfg.setBot {
			fmt.Fprintf(w, "-load %s: -seed -density -species -colour -empires -bot ignored\n", cfg.load)
		}
		var err error
		if f, err = save.Read(cfg.load); err != nil {
			return err
		}
		bots = resume(f, fresh)
	} else {
		if cfg.empires < 1 || cfg.empires > int(game.NumSpecies) {
			return fmt.Errorf("-empires %d out of 1..%d", cfg.empires, game.NumSpecies)
		}
		if cfg.empires > cfg.density.Stars() { // game.NewGame panics past one home per star
			return fmt.Errorf("-empires %d: a %s cluster has %d stars", cfg.empires, cfg.density, cfg.density.Stars())
		}
		g := game.NewGame(cfg.seed, game.PickSpecies(cfg.seed, cfg.species, cfg.empires), cfg.density)
		g.SetColour(0, cfg.colour)
		f = &save.File{Seed: cfg.seed, Game: g}
		bots = bot.NewAll(g, fresh)
		if !cfg.useBot {
			bots[0] = nil
		}
	}
	g := f.Game
	g.SetJournal(cfg.journal)
	if err := game.SpawnFixtures(g, cfg.ships, cfg.warships, cfg.foes); err != nil {
		return err
	}
	if arenaA != nil {
		if err := game.SpawnArena(g, arenaA, arenaB); err != nil {
			return err
		}
	}
	if scen != nil {
		if err := game.ApplyScenario(g, scen); err != nil {
			return fmt.Errorf("%s: %w", cfg.scenario, err)
		}
	}
	for range cfg.days {
		evs, reps, _ := bot.Day(g, bots)
		f.Log = record(f.Log, g.Day, reps, evs[0])
	}
	f.Bots = make([]*bot.State, len(g.Empires))
	for i, b := range bots {
		if b != nil {
			f.Bots[i] = b.Save()
		}
	}
	if err := save.Write(cfg.out, f); err != nil {
		return err
	}
	fmt.Fprintf(w, "seed %d, day %d, %d empires, %d log entries -> %s\n", f.Seed, g.Day, len(g.Empires), len(f.Log), cfg.out)
	return nil
}

// resume is the bots of a loaded save as the window's loadFrom makes them:
// the save's own, a fresh one playing cfg for another empire it has none
// for, none for the player's (empire 0) then; G set on every one.
func resume(f *save.File, cfg bot.Config) []*bot.Bot {
	bots := make([]*bot.Bot, len(f.Game.Empires))
	for i, e := range f.Game.Empires {
		switch {
		case i < len(f.Bots) && f.Bots[i] != nil:
			bots[i] = bot.Resume(e, f.Bots[i])
		case i != 0:
			bots[i] = bot.New(e, cfg)
		}
		if bots[i] != nil {
			bots[i].G = f.Game // the bots fight
		}
	}
	return bots
}

// record appends the player's day to lg as the window's endDay does: its
// combat reports (Empire 0's of reps), then its events, all stamped day,
// the last save.LogKeep kept.
func record(lg []save.Entry, day int, reps []game.Report, evs []game.Event) []save.Entry {
	for _, r := range reps {
		if r.Empire == 0 {
			lg = append(lg, save.Entry{Day: day, Event: r.Event})
		}
	}
	for _, x := range evs {
		lg = append(lg, save.Entry{Day: day, Event: x})
	}
	if n := len(lg); n > save.LogKeep {
		lg = lg[n-save.LogKeep:]
	}
	return lg
}
