// fuzz plays many bot-only games headless (no Ebiten) and prints
// measurements: how fast a species spreads alone, who wins a crowd.
//
//	fuzz -seeds 1-3 -species all -days 3000           solo: every species alone, 3 clusters each
//	fuzz -species orfa,mebes -seeds 1-10,20 -density dense
//	fuzz -mode versus -seeds 1-6 -empires 4 -days 1000
//	fuzz -mode evolve -species orfa -seeds 1-4 -pop 32 -gens 20 -o genomes
//	fuzz -mode evolve -fitness versus -species orfa -empires 4
//	fuzz -mode duel -seeds 1-8 -swap -genome genomes/orfa.json   small fights, see Duel
//	fuzz -mode duel -arena raider:colony -species orfa,mebes -days 20
//
// Every game is bot.NewAll(g, bot.Default()) advanced by bot.Day: the
// sim -galaxy default (its script, the GuideResearch queue, greedy). No
// rule is computed here: the tool only reads the Game.
//
// Solo: one game per (species, seed), species-major, of that species
// alone (game.NewGame), its bot the -genome file's (none: bot.Default). With n stars the milestones are a quarter, a
// half and the control victory, more than two-thirds (the game's
// end-ctrl.txt; docs/rules.md "Winning"): need = ceil(n/4), ceil(n/2),
// floor(2n/3)+1. Each day records the first day ControlledSystems
// reaches each, and the last day it grew (the stall marker of a run
// that never wins). A run stops on the day it wins, else at -days.
//
// Versus: one game per seed of -empires empires, game.PickSpecies(seed,
// seed mod NumSpecies, empires): every species leads some seed. Exactly
// -days days; the empires ranked by systems, then total pop, then index.
// The empires fight by the combat rounds (game.Round, played by
// bot.Day); solo games have one empire, no combat.
//
// Duel: per seed, game.NewGame(seed, the two -species, density): "all"
// (the default) is the first two species, Minions (A) and Snovemdomas
// (B, double hulls); -empires is ignored. game.SpawnArena with -arena
// (game.ParseArena: side A, empire 0, in orbit of its home planet; side
// B, empire 1, at the home's first lane endpoint, as just arrived).
// Empire 0's bot is bot.Default() with the -genome file (nil: the fixed
// policy), empire 1's with -genome-b; -swap plays each seed again with
// the two genomes exchanged, the arena and species unchanged, so the
// home side's advantage cancels in the genomes' tally. bot.Day until the
// day after which a side has no ship at the arena star out of the lanes
// (the other wins; neither: a draw), else -days (30 unless given; a
// timeout, a draw). A row per game: the end day, the winner, per side
// the ships at the star / the arena's ships still alive anywhere (fled
// ones count), the hull at the star, the ships destroyed (bot.Day's
// Destroyed Reports, by pair); then the wins by side (and by genome
// under -swap) and the mean hull at the star. No round counter: bot.Day
// exposes none.
//
// Evolve: a genetic algorithm over bot.Genome, one independent run per
// -species, in list order. A genome plays bot.Config{the default Script,
// Greedy, Genome} in a solo game per seed; a run scores 2 − winDay/cap
// when won, else systems/need(3), so any win beats any loss; the
// genome's fitness is the mean over the seeds. -fitness versus replaces
// the solo game: per seed, a game of -empires empires (checked as in
// versus mode), game.PickSpecies(seed, species, empires), empire 0 the
// genome's and the others bot.Default(), played until some empire
// controls more than two-thirds of the stars (need(3)) or -days; a run
// scores as versusRunFitness does. The final comparison uses the same
// fitness.
// Gen 0 is Neutral and -pop−1 mutants of it (each gene with prob 0.3
// gets += N(0, 2)). Each generation evaluates its unscored genomes (the
// policy is deterministic: an elite keeps its score), sorts by fitness
// desc (stable on population index), then breeds: the top 2 copied, the
// rest children of two tournament parents (3 drawn uniformly with
// replacement, the fittest wins, ties to the lower index), by block
// crossover (crossover), mutated (each gene with prob 0.1 gets += N(0,
// sigma), sigma linear from 1.0 at gen 0 to 0.2 at the last), clamped.
// The only PRNG is per species, PCG(-rng, species+1), drawn in the main
// goroutine in a fixed order: the games run in parallel but the output
// depends on -rng and -seeds only. The best genome is compared with
// Neutral and the nil-genome bot.Default() on the same seeds, printed,
// and written with bot.WriteGenome to <-o>/<species>.json when -o is set.
// -fresh fights the fit to a few clusters: generation k plays as many
// new ones, each seed + 1000·(k+1), every genome scored again (the
// elites too), and the final comparison (the last best, Neutral, the
// default) runs on -seeds, which no generation played.
//
// The jobs run on -j goroutines (games share no mutable state); each
// writes its own slot of a preallocated slice and printing waits for all
// of them, in job order: stdout is byte-identical whatever -j. The wall
// time goes to stderr.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

type config struct {
	versus  bool
	evolve  bool
	duel    bool
	seeds   []uint64
	species []game.Species // solo, evolve; duel: exactly two, sides A and B
	density game.Density
	days    int
	empires int // versus, evolve's versus fitness
	jobs    int // worker goroutines

	pop, gens int    // evolve: population, generations
	versusFit bool   // evolve: -fitness versus
	rng       uint64 // evolve: the GA's PRNG seed
	out       string // evolve: genome directory, "" none
	fresh     bool   // evolve: each generation on new clusters, -seeds held out

	arena            string        // duel: the -arena spec, as given
	arenaA, arenaB   []game.Design // duel: game.ParseArena(arena)
	genomeA, genomeB *bot.Genome   // duel: -genome, -genome-b; nil: bot.Default's fixed policy
	nameA, nameB     string        // duel: their file names, "default" for none
	swap             bool          // duel: each seed played again with the genomes exchanged
}

func main() {
	mode := flag.String("mode", "solo", "solo (one species alone per game), versus (several empires), evolve (a GA over bot genomes) or duel (a small fight, game.SpawnArena)")
	seeds := flag.String("seeds", "1-10", "seeds: a range a-b, a comma list, or both (1-3,9)")
	species := flag.String("species", "all", "solo: all, or a comma list of species by name or index; duel: two, sides A and B (all: the first two species)")
	density := flag.String("density", "average", "cluster density")
	days := flag.Int("days", 3000, "the day cap (versus: the days played; duel: 30 unless given)")
	empires := flag.Int("empires", 4, "versus, evolve -fitness versus: empires per game")
	fitness := flag.String("fitness", "solo", "evolve: solo (the genome's species alone) or versus (against -empires-1 default bots)")
	jobs := flag.Int("j", max(1, runtime.NumCPU()-2), "worker goroutines (default: all CPUs but two, the machine stays usable)")
	pop := flag.Int("pop", 32, "evolve: genomes per generation")
	gens := flag.Int("gens", 20, "evolve: generations")
	rngSeed := flag.Uint64("rng", 1, "evolve: the GA's PRNG seed (-seeds stays the clusters)")
	out := flag.String("o", "", "evolve: a directory for <species>.json, the best genomes")
	fresh := flag.Bool("fresh", false, "evolve: each generation plays new clusters (-seeds + 1000·(gen+1)); -seeds is held out for the final comparison")
	arena := flag.String("arena", defaultArena, "duel: side A (empire 0, in orbit of its home):side B (empire 1, at the home's first lane), designs raider, lancer, colony, corvette, frigate, cruiser, battleship")
	genomeA := flag.String("genome", "", "duel: empire 0's genome (bot.ReadGenome); solo: every game's; none: bot.Default")
	genomeB := flag.String("genome-b", "", "duel: empire 1's genome, none: bot.Default")
	swap := flag.Bool("swap", false, "duel: each seed played again with the two genomes exchanged")
	flag.Parse()

	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if *mode == "duel" && !set["days"] {
		*days = duelDays
	}

	cfg, err := parseConfig(*mode, *seeds, *species, *density, *days, *empires, *jobs)
	if err == nil && cfg.evolve {
		err = cfg.setEvolve(*pop, *gens, *rngSeed, *out)
	}
	if err == nil {
		err = cfg.setFitness(*fitness)
	}
	if err == nil {
		err = cfg.setFresh(*fresh)
	}
	if err == nil {
		err = cfg.setDuel(*arena, *genomeA, *genomeB, *swap, set)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fuzz:", err)
		os.Exit(2)
	}
	t0 := time.Now()
	switch {
	case cfg.evolve:
		if err := evolveAll(os.Stdout, os.Stderr, cfg); err != nil {
			fmt.Fprintln(os.Stderr, "fuzz:", err)
			os.Exit(1)
		}
	case cfg.duel:
		printDuel(os.Stdout, cfg, runDuel(cfg))
	case cfg.versus:
		printVersus(os.Stdout, cfg, runVersus(cfg))
	default:
		printSolo(os.Stdout, cfg, runSolo(cfg))
	}
	fmt.Fprintf(os.Stderr, "fuzz: %s\n", time.Since(t0).Round(time.Millisecond))
}

func parseConfig(mode, seeds, species, density string, days, empires, jobs int) (config, error) {
	cfg := config{days: days, empires: empires, jobs: jobs}
	var err error
	switch mode {
	case "solo":
	case "versus":
		cfg.versus = true
	case "evolve":
		cfg.evolve = true
	case "duel":
		cfg.duel = true
	default:
		return cfg, fmt.Errorf("-mode %q: solo, versus, evolve or duel", mode)
	}
	if cfg.seeds, err = parseSeeds(seeds); err != nil {
		return cfg, err
	}
	if cfg.density, err = game.ParseDensity(density); err != nil {
		return cfg, err
	}
	if days < 1 {
		return cfg, fmt.Errorf("-days %d: at least 1", days)
	}
	if jobs < 1 {
		return cfg, fmt.Errorf("-j %d: at least 1", jobs)
	}
	if cfg.versus {
		if species != "all" {
			return cfg, errors.New("-species applies to solo")
		}
		return cfg, checkEmpires(empires, cfg.density)
	}
	if cfg.duel {
		return cfg, cfg.duelSpecies(species)
	}
	if species == "all" {
		for sp := range game.NumSpecies {
			cfg.species = append(cfg.species, sp)
		}
		return cfg, nil
	}
	for _, f := range list(species) {
		sp, err := game.ParseSpecies(f)
		if err != nil {
			return cfg, err
		}
		cfg.species = append(cfg.species, sp)
	}
	if len(cfg.species) == 0 {
		return cfg, errors.New("-species: none")
	}
	return cfg, nil
}

// checkEmpires validates a versus game's empire count on d.
func checkEmpires(empires int, d game.Density) error {
	switch {
	case empires < 2 || empires > int(game.NumSpecies):
		return fmt.Errorf("-empires %d: 2..%d", empires, game.NumSpecies)
	case empires > d.Stars():
		// game.NewGame panics past one home per star.
		return fmt.Errorf("-empires %d: a %s cluster has %d stars", empires, d, d.Stars())
	}
	return nil
}

// list splits a comma list, dropping empty fields.
func list(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// parseSeeds reads "1-10", "1,4,7" or "1-3,9": ranges inclusive, in the
// order written (no dedupe).
func parseSeeds(s string) ([]uint64, error) {
	var out []uint64
	for _, f := range list(s) {
		a, b, isRange := strings.Cut(f, "-")
		lo, err := strconv.ParseUint(strings.TrimSpace(a), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("-seeds %q: bad seed %q", s, a)
		}
		hi := lo
		if isRange {
			if hi, err = strconv.ParseUint(strings.TrimSpace(b), 10, 64); err != nil {
				return nil, fmt.Errorf("-seeds %q: bad seed %q", s, b)
			}
			if hi < lo {
				return nil, fmt.Errorf("-seeds %q: range %d-%d backwards", s, lo, hi)
			}
		}
		for x := lo; ; x++ {
			out = append(out, x)
			if x == hi {
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-seeds %q: no seed", s)
	}
	return out, nil
}

// parallel calls f(0..n-1) on j goroutines.
func parallel(n, j int, f func(i int)) {
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(j, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				f(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}

// need is milestone k's system count on n stars: ceil(k·n/4) for k = 1,
// 2; for k = 3 the victory, more than two-thirds: floor(2n/3)+1.
func need(k, n int) int {
	if k == 3 {
		return 2*n/3 + 1
	}
	return (k*n + 3) / 4
}

// soloResult is one solo run. mile[k-1] is the day milestone k was
// reached, 0 never; lastGrowth the last day the systems grew, 0 never.
type soloResult struct {
	species    game.Species
	seed       uint64
	stars      int
	mile       [3]int
	systems    int
	colonies   int
	day        int // the day played to
	lastGrowth int
}

// runSolo plays one game per (species, seed), species-major.
func runSolo(cfg config) []soloResult {
	res := make([]soloResult, len(cfg.species)*len(cfg.seeds))
	parallel(len(res), cfg.jobs, func(i int) {
		res[i] = solo(cfg.species[i/len(cfg.seeds)], cfg.seeds[i%len(cfg.seeds)], cfg.density, cfg.days, genomeConfig(cfg.genomeA))
	})
	return res
}

// solo plays one game of sp alone, every empire's bot on bc.
func solo(sp game.Species, seed uint64, d game.Density, days int, bc bot.Config) soloResult {
	g := game.NewGame(seed, []game.Species{sp}, d)
	bots := bot.NewAll(g, bc)
	e := g.Empires[0]
	r := soloResult{species: sp, seed: seed, stars: len(g.Cluster.Stars)}
	prev := e.ControlledSystems()
	for range days {
		bot.Day(g, bots)
		s := e.ControlledSystems()
		if s > prev {
			r.lastGrowth = g.Day
		}
		prev = s
		for k := range r.mile {
			if r.mile[k] == 0 && s >= need(k+1, r.stars) {
				r.mile[k] = g.Day
			}
		}
		if r.mile[2] != 0 {
			break
		}
	}
	r.systems, r.colonies, r.day = e.ControlledSystems(), len(e.Colonies), g.Day
	return r
}

func dayCell(d int) string {
	if d == 0 {
		return "-"
	}
	return strconv.Itoa(d)
}

// median is the lower middle of xs (sorted in place), 0 for none.
func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	slices.Sort(xs)
	return xs[(len(xs)-1)/2]
}

// printSolo writes the runs' table, then one summary row per species in
// first-run order.
func printSolo(w io.Writer, cfg config, res []soloResult) {
	fmt.Fprintf(w, "solo: %s, %d runs, cap %d days\n\n", cfg.density, len(res), cfg.days)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "species\tseed\tstars\t1/4\t1/2\twin\tsystems\tcolonies\tstalled")
	for _, r := range res {
		stalled := "-"
		if r.mile[2] == 0 {
			stalled = strconv.Itoa(r.lastGrowth)
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\t%s\t%d/%d\t%d\t%s\n", r.species, r.seed, r.stars,
			dayCell(r.mile[0]), dayCell(r.mile[1]), dayCell(r.mile[2]), r.systems, r.stars, r.colonies, stalled)
	}
	tw.Flush()

	fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "species\truns\twins\tmedian-win\tmean-share")
	var order []game.Species
	for _, r := range res {
		if !slices.Contains(order, r.species) {
			order = append(order, r.species)
		}
	}
	for _, sp := range order {
		var runs int
		var reached []int
		share := 0.0
		for _, r := range res {
			if r.species != sp {
				continue
			}
			runs++
			share += float64(r.systems) / float64(r.stars)
			if r.mile[2] != 0 {
				reached = append(reached, r.mile[2])
			}
		}
		med := "-"
		if len(reached) > 0 {
			med = strconv.Itoa(median(reached))
		}
		fmt.Fprintf(tw, "%s\t%d\t%d/%d\t%s\t%.0f%%\n", sp, runs, len(reached), runs, med, 100*share/float64(runs))
	}
	tw.Flush()
}

// versusEmpire is one empire's end of a versus game.
type versusEmpire struct {
	index    int
	species  game.Species
	systems  int
	colonies int
	pop      int
}

// versusResult is one versus game, its empires in rank order, and its
// conflicts summed from the ledgers (game.Relation, each fact counted
// once, by its victim): wars declared, empire-days at war, ships
// destroyed, colonies invaded; the empires defeated and the armed ships
// flying at the end; the victories won by kind (game.Victory) with their
// days summed, the living pairs allied at the end.
type versusResult struct {
	seed   uint64
	stars  int
	ranked []versusEmpire

	wars, warDays, shipsLost, invaded, defeated, armed int
	won, wonDays                                       [game.UniteVictory + 1]int
	allied                                             int
}

// runVersus plays one game per seed.
func runVersus(cfg config) []versusResult {
	res := make([]versusResult, len(cfg.seeds))
	parallel(len(res), cfg.jobs, func(i int) {
		res[i] = versus(cfg.seeds[i], cfg.density, cfg.days, cfg.empires)
	})
	return res
}

// versusGame is a versus game on seed: empires species picked by
// game.PickSpecies with lead as empire 0.
func versusGame(seed uint64, lead game.Species, d game.Density, empires int) *game.Game {
	return game.NewGame(seed, game.PickSpecies(seed, lead, empires), d)
}

func versus(seed uint64, d game.Density, days, empires int) versusResult {
	g := versusGame(seed, game.Species(seed%uint64(game.NumSpecies)), d, empires)
	bots := bot.NewAll(g, bot.Default())
	for range days {
		bot.Day(g, bots)
	}
	r := versusResult{seed: seed, stars: len(g.Cluster.Stars)}
	for i, e := range g.Empires {
		ve := versusEmpire{index: i, species: e.Species, systems: e.ControlledSystems(), colonies: len(e.Colonies)}
		for _, col := range e.Colonies {
			ve.pop += col.Pop
		}
		r.ranked = append(r.ranked, ve)
		r.won[e.Won]++
		r.wonDays[e.Won] += e.WonDay
		for j, rel := range e.Relations {
			if j > i && rel.Stance == game.Alliance && e.Alive() && g.Empires[j].Alive() {
				r.allied++
			}
			r.wars += rel.WarsOn
			r.warDays += rel.WarDays
			r.shipsLost += rel.ShipsLost
			r.invaded += rel.PlanetsLost
		}
		if !e.Alive() {
			r.defeated++
		}
		for _, sh := range e.Ships {
			if sh.Design.Weapons() > 0 {
				r.armed++
			}
		}
	}
	rank(r.ranked)
	return r
}

// rank sorts by systems desc, then pop desc, then index asc.
func rank(es []versusEmpire) {
	slices.SortFunc(es, func(a, b versusEmpire) int {
		switch {
		case a.systems != b.systems:
			return b.systems - a.systems
		case a.pop != b.pop:
			return b.pop - a.pop
		}
		return a.index - b.index
	})
}

// printVersus writes one row per game, empires in rank order, then a
// summary per species that played, in species order.
func printVersus(w io.Writer, cfg config, res []versusResult) {
	fmt.Fprintf(w, "versus: %s, %d games of %d empires, %d days\n\n", cfg.density, len(res), cfg.empires, cfg.days)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	head := "seed"
	for k := range cfg.empires {
		head += "\t#" + strconv.Itoa(k+1)
	}
	fmt.Fprintln(tw, head)
	for _, r := range res {
		fmt.Fprintf(tw, "%d", r.seed)
		for _, e := range r.ranked {
			fmt.Fprintf(tw, "\t%s %d/%d", e.species, e.systems, r.stars)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()

	type agg struct {
		games, wins, rankSum int
		share                float64
	}
	var by [game.NumSpecies]agg
	for _, r := range res {
		for k, e := range r.ranked {
			a := &by[e.species]
			a.games++
			a.rankSum += k + 1
			a.share += float64(e.systems) / float64(r.stars)
			if k == 0 {
				a.wins++
			}
		}
	}
	fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "species\tgames\twins\tmean-rank\tmean-share")
	for sp := range game.NumSpecies {
		a := by[sp]
		if a.games == 0 {
			continue
		}
		n := float64(a.games)
		fmt.Fprintf(tw, "%s\t%d\t%d\t%.1f\t%.0f%%\n", sp, a.games, a.wins, float64(a.rankSum)/n, 100*a.share/n)
	}
	tw.Flush()

	var c versusResult
	for _, r := range res {
		c.wars += r.wars
		c.warDays += r.warDays
		c.shipsLost += r.shipsLost
		c.invaded += r.invaded
		c.defeated += r.defeated
		c.armed += r.armed
		c.allied += r.allied
		for v := range c.won {
			c.won[v] += r.won[v]
			c.wonDays[v] += r.wonDays[v]
		}
	}
	n := float64(max(1, len(res)))
	fmt.Fprintf(w, "\nconflict per game: wars %.2f  empire-days at war %.0f  ships destroyed %.1f  colonies invaded %.1f  defeated %.2f  armed ships at the end %.1f\n",
		float64(c.wars)/n, float64(c.warDays)/n, float64(c.shipsLost)/n, float64(c.invaded)/n, float64(c.defeated)/n, float64(c.armed)/n)
	fmt.Fprintf(w, "alliances at the end per game: %.2f\nvictories (empires, mean day):", float64(c.allied)/n)
	for v := game.ControlVictory; v <= game.UniteVictory; v++ {
		fmt.Fprintf(w, "  %s %d", v, c.won[v])
		if c.won[v] > 0 {
			fmt.Fprintf(w, " (%d)", c.wonDays[v]/c.won[v])
		}
	}
	fmt.Fprintln(w)
}
