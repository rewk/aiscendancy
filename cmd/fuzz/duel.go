package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"text/tabwriter"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

const (
	duelDays     = 30                 // duel's -days when not given: side B starts at the lane end, a short range makes the approach take days
	defaultArena = "frigate:corvette" // duel's -arena: a fight with a winner (mirrors draw, raiders do no damage)
)

// duelSpecies reads duel's -species: "all" is the first two species in
// order (solo's "all" list cut to two), else exactly two distinct names.
// The cluster must hold two homes.
func (cfg *config) duelSpecies(species string) error {
	if species == "all" {
		cfg.species = []game.Species{0, 1}
	} else {
		for _, f := range list(species) {
			sp, err := game.ParseSpecies(f)
			if err != nil {
				return err
			}
			cfg.species = append(cfg.species, sp)
		}
		if len(cfg.species) != 2 || cfg.species[0] == cfg.species[1] {
			return fmt.Errorf("-species %q: duel takes two distinct species", species)
		}
	}
	return checkEmpires(2, cfg.density)
}

// setDuel checks and stores duel's flags (set: the flags given); outside
// duel any of them given is an error, but -genome in solo (every game's
// bot, genomeConfig).
func (cfg *config) setDuel(arena, genomeA, genomeB string, swap bool, set map[string]bool) error {
	read := func(path string) (*bot.Genome, string, error) {
		if path == "" {
			return nil, "default", nil
		}
		g, err := bot.ReadGenome(path)
		return g, path, err
	}
	if !cfg.duel {
		solo := !cfg.versus && !cfg.evolve
		for _, f := range []string{"arena", "genome", "genome-b", "swap"} {
			if set[f] && !(solo && f == "genome") {
				return fmt.Errorf("-%s applies to duel", f)
			}
		}
		if solo {
			var err error
			cfg.genomeA, cfg.nameA, err = read(genomeA)
			return err
		}
		return nil
	}
	var err error
	if cfg.arenaA, cfg.arenaB, err = game.ParseArena(arena); err != nil {
		return err
	}
	cfg.arena, cfg.swap = arena, swap
	if cfg.genomeA, cfg.nameA, err = read(genomeA); err != nil {
		return err
	}
	cfg.genomeB, cfg.nameB, err = read(genomeB)
	return err
}

// duelSide is one side's end of a duel.
type duelSide struct {
	atStar int // the empire's ships at the arena star, out of the lanes
	alive  int // the arena's ships still in the empire's Ships, wherever they are
	hull   int // the Hull of the ships at the star, summed
	lost   int // the empire's ships destroyed (the Destroyed Reports)
}

// duelResult is one duel; side A is empire 0, B empire 1.
type duelResult struct {
	seed    uint64
	swapped bool // empire 0 played -genome-b, empire 1 -genome
	day     int  // the day it ended, 0 at the cap (timeout)
	winner  int  // 0 side A, 1 side B, -1 draw
	side    [2]duelSide
}

// runDuel plays one duel per seed, each followed by its swapped game
// under -swap.
func runDuel(cfg config) []duelResult {
	per := 1
	if cfg.swap {
		per = 2
	}
	res := make([]duelResult, per*len(cfg.seeds))
	parallel(len(res), cfg.jobs, func(i int) {
		swapped := i%per == 1
		gA, gB := cfg.duelGenomes(swapped)
		res[i] = duel(cfg.seeds[i/per], cfg, gA, gB)
		res[i].swapped = swapped
	})
	return res
}

// duelGenomes is empire 0's and empire 1's genomes: -genome and
// -genome-b, exchanged when swapped.
func (cfg config) duelGenomes(swapped bool) (gA, gB *bot.Genome) {
	if swapped {
		return cfg.genomeB, cfg.genomeA
	}
	return cfg.genomeA, cfg.genomeB
}

// duel plays one: game.NewGame(seed, the two species, density),
// game.SpawnArena at empire 0's home star, empire 0's bot bot.Default()
// with genome gA and NoDiplomacy, empire 1's with gB, then play.
func duel(seed uint64, cfg config, gA, gB *bot.Genome) duelResult {
	g := game.NewGame(seed, cfg.species, cfg.density)
	if err := game.SpawnArena(g, cfg.arenaA, cfg.arenaB); err != nil {
		panic(err) // two empires, a home with lanes: NewGame's
	}
	bots := make([]*bot.Bot, 2)
	for k, gen := range []*bot.Genome{gA, gB} {
		bc := bot.Default()
		bc.Genome = gen
		bc.NoDiplomacy = true // the arena's War holds: a fresh war without grudge would end in peace on day 0
		bots[k] = bot.New(g.Empires[k], bc)
		bots[k].G = g
	}
	r := play(g, bots, cfg.days)
	r.seed = seed
	return r
}

// play runs a duel set on g (two empires, the arena at empire 0's home
// star, their Ships the arena's): bot.Day until the day after which one
// side, or both, has no ship at the star out of the lanes (the winner
// the side left; both gone, a draw), else days days (a draw: both
// remain). Destroyed Reports come in pairs (game.Game.destroyed), the
// shooter's then the target's: the second's Empire is the loser.
func play(g *game.Game, bots []*bot.Bot, days int) duelResult {
	star := g.Empires[0].Colonies[0].Star // read before any day: the arena's home, never captured yet
	var arena [2][]*game.Starship
	for k := range arena {
		arena[k] = slices.Clone(g.Empires[k].Ships)
	}
	r := duelResult{winner: -1}
	at := func(k int) (n, hull int) {
		for _, s := range g.Empires[k].Ships {
			if s.Star == star && s.Lane < 0 {
				n++
				hull += s.Hull
			}
		}
		return n, hull
	}
	for range days {
		_, reps, _ := bot.Day(g, bots)
		for j := 0; j+1 < len(reps); j += 2 {
			if reps[j+1].Event.Kind == game.Destroyed {
				r.side[reps[j+1].Empire].lost++
			}
		}
		nA, _ := at(0)
		nB, _ := at(1)
		if nA == 0 || nB == 0 {
			r.day = g.Day
			switch {
			case nA > 0:
				r.winner = 0
			case nB > 0:
				r.winner = 1
			}
			break
		}
	}
	for k := range r.side {
		r.side[k].atStar, r.side[k].hull = at(k)
		for _, s := range arena[k] {
			if slices.Contains(g.Empires[k].Ships, s) {
				r.side[k].alive++
			}
		}
	}
	return r
}

// genomeWinner is the winning genome: 0 -genome, 1 -genome-b, -1 draw.
func (r duelResult) genomeWinner() int {
	if r.winner < 0 || !r.swapped {
		return r.winner
	}
	return 1 - r.winner
}

// printDuel writes one row per game, in runDuel's order, then the wins
// by side (and by genome under -swap) and the mean hull left at the star.
func printDuel(w io.Writer, cfg config, res []duelResult) {
	fmt.Fprintf(w, "duel: %s, %s (A) vs %s (B), arena %s, %d games, cap %d days; genomes A %s, B %s\n\n",
		cfg.density, cfg.species[0], cfg.species[1], cfg.arena, len(res), cfg.days, cfg.nameA, cfg.nameB)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "seed\tgenomes\tend\twinner\tA star/alive\tB star/alive\tA hull\tB hull\tA lost\tB lost")
	var wins [2]int
	var gwins [2]int
	var hull [2]int
	draws := 0
	for _, r := range res {
		genomes, end, winner := "A/B", "timeout", "draw"
		if r.swapped {
			genomes = "B/A"
		}
		if r.day != 0 {
			end = strconv.Itoa(r.day)
		}
		if r.winner >= 0 {
			winner = string(rune('A' + r.winner))
			wins[r.winner]++
			gwins[r.genomeWinner()]++
		} else {
			draws++
		}
		a, b := r.side[0], r.side[1]
		hull[0] += a.hull
		hull[1] += b.hull
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%d/%d\t%d/%d\t%d\t%d\t%d\t%d\n", r.seed, genomes, end, winner,
			a.atStar, a.alive, b.atStar, b.alive, a.hull, b.hull, a.lost, b.lost)
	}
	tw.Flush()
	fmt.Fprintf(w, "\nsides: A %d, B %d, draws %d\n", wins[0], wins[1], draws)
	if cfg.swap {
		fmt.Fprintf(w, "genomes: A (%s) %d, B (%s) %d, draws %d\n", cfg.nameA, gwins[0], cfg.nameB, gwins[1], draws)
	}
	if n := float64(len(res)); n > 0 {
		fmt.Fprintf(w, "mean hull at the star: A %.1f, B %.1f\n", float64(hull[0])/n, float64(hull[1])/n)
	}
}
