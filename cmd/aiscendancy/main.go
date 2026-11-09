// aiscendancy is the game's one window: the opening, the New Game
// screen, then the
// starmap, a star's system, a planet, stacked as the player goes in; Esc
// goes back one screen.
//
//	aiscendancy                              the opening (the main menu)
//	aiscendancy -load game.json              start from a save instead
//	aiscendancy -load game.json -star 16     open on star 16's system
//	aiscendancy -load game.json -star 16 -planet 0   on its first planet
//	aiscendancy -shot out.png                render one frame to a PNG and exit
//	aiscendancy -save game.json.gz           where F5 saves and F9 loads (default ~/.local/share/aiscendancy/quick.json.gz, %LOCALAPPDATA%\aiscendancy\quick.json.gz on Windows)
//	aiscendancy -journal play.jsonl          append every action taken (player and bots) to a JSON Lines file
//
// A game set up by flags is made offline by cmd/mksave (a save to
// -load). Standing flag values go in aiscendancy.json (-config, config.go), the
// sound settings there only (the settings panel, F2, writes them).
// F5 saves (package save), F9 loads the -save file back. The window plays
// empire 0 by hand, package bot every other one (and the player's when
// saved with one). Space ends the day on any screen, Tab toggles the log.
// The skeleton is app.go (the stack, the 1280x960 frame, -shot).
package main

import (
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/save"
)

// initialStack is the screens to open, not entered yet: the opening
// screen alone without a game, else the starmap, the system of star
// (star ≥ 0), its planet (planet ≥ 0).
func initialStack(cl *game.Cluster, star, planet int) ([]screen, error) {
	if cl == nil {
		if star >= 0 || planet >= 0 {
			return nil, fmt.Errorf("-star and -planet need -load")
		}
		return []screen{newOpeningScreen()}, nil
	}
	ss := []screen{newStarmapScreen()}
	if star < 0 {
		if planet >= 0 {
			return nil, fmt.Errorf("-planet needs -star")
		}
		return ss, nil
	}
	if star >= len(cl.Stars) {
		return nil, fmt.Errorf("star %d out of 0..%d", star, len(cl.Stars)-1)
	}
	ss = append(ss, newSystemScreen(star))
	if planet < 0 {
		return ss, nil
	}
	if n := len(cl.Stars[star].System.Planets); planet >= n {
		return nil, fmt.Errorf("planet %d: %s has %d planets", planet, cl.Stars[star].Name, n)
	}
	return append(ss, newPlanetScreen(star, planet)), nil
}

// topScreens are what -screen opens over the stack, the starmap's boxes
// and keys reach them in play (a dev aid: -shot of a list screen).
var topScreens = map[string]func() screen{
	"planets":  func() screen { return newPlanetsScreen() },
	"fleet":    func() screen { return newFleetScreen() },
	"log":      func() screen { return newLogScreen() },
	"research": func() screen { return newResearchScreen() },
	"species":  func() screen { return newSpeciesScreen(-1) },
	"stats":    func() screen { return newStatsScreen() },
	"designs":  func() screen { return newDesignScreen() },
	"save":     func() screen { return newSlotsScreen(true) },
	"load":     func() screen { return newSlotsScreen(false) },
}

// topScreen is -screen's screen by name; "" none. It needs a game.
func topScreen(cl *game.Cluster, name string) (screen, error) {
	if name == "" {
		return nil, nil
	}
	mk, ok := topScreens[name]
	if !ok {
		names := slices.Sorted(maps.Keys(topScreens))
		return nil, fmt.Errorf("-screen %q: one of %s", name, strings.Join(names, ", "))
	}
	if cl == nil {
		return nil, fmt.Errorf("-screen needs -load")
	}
	return mk(), nil
}

// where names the top screen for the -shot line.
func where(a *app) string {
	switch s := a.top().(type) {
	case *openingScreen:
		return fmt.Sprintf("opening: resume %v", s.resume)
	case *newGameScreen:
		return s.summary()
	case *introScreen:
		return fmt.Sprintf("introduction: %s", s.title)
	case *starmapScreen:
		return fmt.Sprintf("starmap: %s, %d empires, home %s", strings.ReplaceAll(mapSummary(a.cl), "\n", ", "), len(a.g.Empires), a.cl.Stars[a.cl.Home].Name)
	case *systemScreen:
		return fmt.Sprintf("system %d: %s, star type %d, %d planets, %d lanes",
			s.star, s.sys.Name, s.sys.Star, len(s.sys.Planets), len(s.sys.Lanes))
	case *planetScreen:
		return fmt.Sprintf("planet %d of star %d: %s, %s size %d", s.idx, s.star, s.planet.Name, s.planet.Type, s.planet.Size)
	case *researchScreen:
		return fmt.Sprintf("research: topic %v, %d known", a.e.Research.Topic, known(a.e.Research))
	case *designScreen:
		return fmt.Sprintf("design %q: %s, %d parts, %d templates", s.t.Name, s.t.Hull, len(s.t.Parts), len(a.e.Templates))
	case *logScreen:
		return fmt.Sprintf("log: %d entries, day %d", len(a.log), a.e.Day)
	case *fleetScreen:
		return fmt.Sprintf("fleet: %d ships", len(a.e.Ships))
	case *planetsScreen:
		return fmt.Sprintf("planets: %d colonies, %d systems", len(a.e.Colonies), a.e.ControlledSystems())
	case *slotsScreen:
		return fmt.Sprintf("saves: %d slots, saving %v", len(s.slots), s.saving)
	}
	return "?"
}

func main() {
	dir := flag.String("game", cob.DefaultDir(), "game directory holding ascend*.cob")
	star := flag.Int("star", -1, "with -load, open on this star's system (index in the cluster)")
	planet := flag.Int("planet", -1, "with -star, open on this planet of the system (index from 0)")
	shot := flag.String("shot", "", "render one frame to this PNG and exit")
	top := flag.String("screen", "", "with -load, open this screen over the stack: planets, fleet, log, research, species, stats, designs, save, load")
	savePath := flag.String("save", save.DefaultPath(), "where F5 saves and F9 loads")
	load := flag.String("load", "", "start from this save (cmd/mksave makes one) instead of the opening; the player's bot plays iff it was saved with one")
	reveal := flag.Bool("reveal", false, "cheat: the player's empire explores every star (planets, lanes, the other empires' ships and colonies; saved with it) and scans every ship (designs, hull, energy)")
	genome := flag.String("genome", "", "every fresh bot (a new game's, a save's empire without one) plays this genome (cmd/fuzz -o); a loaded save's bots keep theirs")
	journalPath := flag.String("journal", "", "append every action taken in the game, the player's and the bots', to this JSON Lines file (package journal); none when empty")
	config := flag.String("config", configDefault, "standing flag values and the sound settings (music, volume, music-gap, sfx: there only), a JSON object keyed by flag name (config.go, aiscendancy.json.example); the command line wins")
	// The settings panel's keys: in the config file only (config.go).
	sound := flag.NewFlagSet("settings", flag.ContinueOnError)
	music := sound.Bool("music", true, "play the original's music (F3 toggles it); false starts muted")
	volume := sound.Float64("volume", 0.6, "music volume, 0..1")
	musicGap := sound.Duration("music-gap", 3*time.Minute, "silence between two tunes")
	sfxVolume := sound.Float64("sfx", 0.6, "sound effects volume, 0..1 (0: none)")
	flag.Parse()
	named := false
	flag.Visit(func(f *flag.Flag) { named = named || f.Name == "config" })
	if err := applyConfig(flag.CommandLine, sound, *config, named); err != nil {
		log.Fatal(err)
	}

	as, err := assets.Open(*dir)
	if err != nil {
		log.Fatal(err)
	}
	defer as.Close()
	a := newApp(as, nil, 0, *shot)
	a.savePath, a.reveal = *savePath, *reveal
	a.lastPath = filepath.Join(filepath.Dir(*savePath), lastFile)
	a.configPath = *config
	a.set = settings{music: *music, volume: min(max(*volume, 0), 1), sfx: min(max(*sfxVolume, 0), 1), gap: max(*musicGap, 0), game: *dir, save: *savePath}
	if *genome != "" {
		if a.genome, err = bot.ReadGenome(*genome); err != nil {
			log.Fatal(err)
		}
	}
	if *journalPath != "" {
		jf, err := os.OpenFile(*journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		defer jf.Close()
		a.journal = journal.New(jf)
	}
	if *load != "" {
		if err := a.loadFrom(*load); err != nil {
			log.Fatal(err)
		}
	}
	ss, err := initialStack(a.cl, *star, *planet)
	if err != nil {
		log.Fatal(err)
	}
	if sc, err := topScreen(a.cl, *top); err != nil {
		log.Fatal(err)
	} else if sc != nil {
		ss = append(ss, sc)
	}
	a.preload()
	if *load != "" {
		a.toastFor(fmt.Sprintf("loaded %s, day %d", *load, a.e.Day), false)
	}
	a.open(ss)
	if *shot == "" {
		a.music, a.sfx = newJukebox(as.Game, *music, *volume, *musicGap, *sfxVolume)
	}

	ebiten.SetWindowSize(screenW, screenH)
	// A bigger window scales the fixed layout (Layout).
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("AIscendancy")
	if icon := windowIcon(as); icon != nil {
		ebiten.SetWindowIcon(icon)
	}
	if err := ebiten.RunGame(a); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	switch {
	case a.shot == "":
	case a.inGame():
		fmt.Printf("seed %d, day %d, %s -> %s\n", a.g.Seed, a.e.Day, where(a), a.shot)
	default:
		fmt.Printf("%s -> %s\n", where(a), a.shot)
	}
}
