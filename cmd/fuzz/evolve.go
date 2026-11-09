package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// The GA's fixed parameters (the flags give pop, gens and the seed).
const (
	elites      = 2   // copied unchanged into the next generation
	tourSize    = 3   // drawn per tournament, with replacement
	pmGen0      = 0.3 // gen 0's mutants of Neutral
	sigmaGen0   = 2.0
	pmBreed     = 0.1 // a child's mutation
	sigmaFirst  = 1.0 // sigma at gen 0 ...
	sigmaLast   = 0.2 // ... and at the last gen, linear between
	scalarGenes = 47  // Balance, CostW, ResCostW, ShipyardFactories, FrontierWorth, RuinWorth, IdleParty, AutomateCostW, FoundFloor, InFlight, AbandonIdle, the twenty diplomacy genes, the four specials' (CannibalAt, ShieldAt, RecallAt, SmartBombAt), then ReplaceDays, ReplaceFull, ShieldFirst, Upgrade, IndShare, ResShare, IndFloor, GrudgeEncroach, Rage, HabitatShare, HabitatFill, HelpW
	combatGenes = 17  // FleeRatio, AttackRatio, ThreatAvoid, WarshipShare, DefendW, Standoff, Rounds, RefitGain, RepairAt, InvadeMargin, InvadeShare, ShieldWorth, ShieldPop, OrbitalShields, OrbitalGuns, OrbitalCloakers, SurfaceCloakers
)

// setEvolve checks and stores evolve's flags.
func (cfg *config) setEvolve(pop, gens int, rng uint64, out string) error {
	if pop < 4 {
		return fmt.Errorf("-pop %d: at least 4", pop)
	}
	if gens < 1 {
		return fmt.Errorf("-gens %d: at least 1", gens)
	}
	cfg.pop, cfg.gens, cfg.rng, cfg.out = pop, gens, rng, out
	return nil
}

// freshStride parts generation k's clusters, -seeds + freshStride·(k+1),
// from the hold-out -seeds and from each other.
const freshStride = 1000

// setFresh checks and stores -fresh: -seeds must span less than
// freshStride, or a generation would replay a hold-out cluster.
func (cfg *config) setFresh(fresh bool) error {
	if !fresh {
		return nil
	}
	if !cfg.evolve {
		return errors.New("-fresh applies to evolve")
	}
	if lo, hi := slices.Min(cfg.seeds), slices.Max(cfg.seeds); hi-lo >= freshStride {
		return fmt.Errorf("-fresh: -seeds %d-%d span %d or more", lo, hi, freshStride)
	}
	cfg.fresh = true
	return nil
}

// genSeeds is generation k's clusters: -seeds, or under -fresh as many
// new ones, -seeds shifted by freshStride·(k+1).
func genSeeds(cfg config, k int) []uint64 {
	if !cfg.fresh {
		return cfg.seeds
	}
	out := make([]uint64, len(cfg.seeds))
	for i, s := range cfg.seeds {
		out[i] = s + freshStride*uint64(k+1)
	}
	return out
}

// setFitness checks and stores -fitness: solo, or versus (evolve only,
// -empires checked as in versus mode).
func (cfg *config) setFitness(kind string) error {
	switch kind {
	case "solo":
		cfg.versusFit = false
		return nil
	case "versus":
		if !cfg.evolve {
			return fmt.Errorf("-fitness %s applies to evolve", kind)
		}
		if err := checkEmpires(cfg.empires, cfg.density); err != nil {
			return err
		}
		cfg.versusFit = true
		return nil
	}
	return fmt.Errorf("-fitness %q: solo or versus", kind)
}

// genes lists pointers to every gene of g in a fixed order: Build, Tech,
// the scalars, HullThreat, HullReach, the combat scalars. Mutation and
// spread walk it.
func genes(g *bot.Genome) []*float64 {
	out := make([]*float64, 0, len(g.Build)+len(g.Tech)+scalarGenes+len(g.HullThreat)+len(g.HullReach)+combatGenes)
	for i := range g.Build {
		out = append(out, &g.Build[i])
	}
	for i := range g.Tech {
		out = append(out, &g.Tech[i])
	}
	out = append(out, scalars(g)...)
	for i := range g.HullThreat {
		out = append(out, &g.HullThreat[i])
	}
	for i := range g.HullReach {
		out = append(out, &g.HullReach[i])
	}
	return append(out, combatScalars(g)...)
}

func scalars(g *bot.Genome) []*float64 {
	return []*float64{&g.Balance, &g.CostW, &g.ResCostW, &g.ShipyardFactories, &g.FrontierWorth, &g.RuinWorth, &g.IdleParty, &g.AutomateCostW, &g.FoundFloor, &g.InFlight, &g.AbandonIdle,
		&g.Temper, &g.EnvyW, &g.GrudgeShip, &g.GrudgePlanet, &g.GrudgeWar, &g.GrudgeRefusal, &g.IntrudeW, &g.Forgive, &g.WarCaution, &g.WearyW,
		&g.TradeW, &g.HospW, &g.GrudgeBreak, &g.TradeFair, &g.TradeZeal, &g.Hospitable, &g.FavourW, &g.AskZeal, &g.DefameW, &g.FreeResearchDays,
		&g.CannibalAt, &g.ShieldAt, &g.RecallAt, &g.SmartBombAt, &g.ReplaceDays, &g.ReplaceFull, &g.ShieldFirst, &g.Upgrade, &g.IndShare, &g.ResShare, &g.IndFloor, &g.GrudgeEncroach, &g.Rage,
		&g.HabitatShare, &g.HabitatFill, &g.HelpW}
}

func combatScalars(g *bot.Genome) []*float64 {
	return []*float64{&g.FleeRatio, &g.AttackRatio, &g.ThreatAvoid, &g.WarshipShare, &g.DefendW, &g.Standoff, &g.Rounds, &g.RefitGain, &g.RepairAt,
		&g.InvadeMargin, &g.InvadeShare, &g.ShieldWorth, &g.ShieldPop, &g.OrbitalShields, &g.OrbitalGuns, &g.OrbitalCloakers, &g.SurfaceCloakers}
}

// dipScales are the mutation scales of the scalars after the first ten,
// in scalars() order (AbandonIdle, Temper to FreeResearchDays, then
// CannibalAt, ShieldAt, RecallAt, SmartBombAt, ReplaceDays, ReplaceFull,
// ShieldFirst, Upgrade, IndShare, ResShare, IndFloor, GrudgeEncroach,
// Rage, HabitatShare, HabitatFill, HelpW): their Neutral magnitude
// (AbandonIdle's 60 days; Temper's 100: the thresholds are 50+ apart;
// the replacement genes, Neutral 0, a quarter and half of their range;
// ShieldFirst, a switch at 0.5, half its range; Upgrade 2 structures,
// the shares 0.1, IndFloor 2 industry, GrudgeEncroach 100 as the
// grudges, Rage 100, the Habitat's 0.5, HelpW 30 as TradeW). The diplomacy genes live on the
// attitude scale (hundreds) or a daily share (hundredths), the specials'
// on a share of the hull, where a unit step is a crawl or a leap: sigma
// is multiplied by the scale.
var dipScales = [...]float64{60, 100, 10, 40, 150, 100, 20, 2, 0.01, 1, 1, 30, 0.2, 150, 0.5, 60, 0.1, 100, 90, 100, 30, 0.5, 0.5, 0.25, 1, 100, 0.5, 0.5, 2, 0.1, 0.1, 2, 100, 100, 0.5, 0.5, 30}

// scaleOf is gene i's mutation scale in genes() order: 1 but for
// AbandonIdle, the diplomacy and the specials' genes (dipScales).
func scaleOf(g *bot.Genome, i int) float64 {
	first := len(g.Build) + len(g.Tech) + scalarGenes - len(dipScales)
	if i >= first && i < first+len(dipScales) {
		return dipScales[i-first]
	}
	return 1
}

// mutate adds N(0, sigma × scaleOf) to each gene with probability pm,
// then clamps. It draws one Float64 per gene whatever pm, plus a normal
// per hit.
func mutate(g *bot.Genome, r *rand.Rand, pm, sigma float64) {
	for i, p := range genes(g) {
		if r.Float64() < pm {
			*p += r.NormFloat64() * sigma * scaleOf(g, i)
		}
	}
	g.Clamp()
}

// crossover is a block crossover: Build whole from a or b (one coin),
// Tech likewise, each scalar its own coin, HullThreat whole (one coin),
// HullReach likewise, each combat scalar its own coin; drawn in that
// order. Neither parent is touched.
func crossover(a, b *bot.Genome, r *rand.Rand) *bot.Genome {
	c := *a // arrays only: a deep copy
	if r.IntN(2) == 1 {
		c.Build = b.Build
	}
	if r.IntN(2) == 1 {
		c.Tech = b.Tech
	}
	coins := func(ca, cb []*float64) {
		for k := range ca {
			if r.IntN(2) == 1 {
				*ca[k] = *cb[k]
			}
		}
	}
	coins(scalars(&c), scalars(b))
	if r.IntN(2) == 1 {
		c.HullThreat = b.HullThreat
	}
	if r.IntN(2) == 1 {
		c.HullReach = b.HullReach
	}
	coins(combatScalars(&c), combatScalars(b))
	return &c
}

// tournament draws tourSize indices of fit uniformly with replacement and
// returns the fittest, ties to the lower index.
func tournament(fit []float64, r *rand.Rand) int {
	best := r.IntN(len(fit))
	for range tourSize - 1 {
		i := r.IntN(len(fit))
		if fit[i] > fit[best] || fit[i] == fit[best] && i < best {
			best = i
		}
	}
	return best
}

// sigmaAt is the breeding sigma of the population of generation k:
// sigmaFirst at 0, sigmaLast at gens-1, linear between.
func sigmaAt(k, gens int) float64 {
	if gens <= 1 {
		return sigmaFirst
	}
	return sigmaFirst + (sigmaLast-sigmaFirst)*float64(k)/float64(gens-1)
}

// runFitness scores one solo run: 2 − winDay/cap when won (in [1, 2)),
// else systems / the victory's need (below 1): any win beats any loss.
func runFitness(r soloResult, days int) float64 {
	if r.mile[2] != 0 {
		return 2 - float64(r.mile[2])/float64(days)
	}
	return float64(r.systems) / float64(need(3, r.stars))
}

// versusFitResult is one versus-fitness game, seen from empire 0 (the
// genome's): winner is the first empire past two-thirds, -1 none by the
// cap; day the day it stopped.
type versusFitResult struct {
	stars    int
	planets  int // the galaxy's
	winner   int
	day      int
	systems  int // empire 0's at the stop
	colonies int
}

// versusRunFitness scores one versus-fitness game as runFitness does a
// solo one: 2 − day/cap when empire 0 won, else (its systems + its share
// of the galaxy's planets) / the victory's need, 0 when it has no colony
// left (a defeated empire 0: game/defeat.go). The planets' share (below
// 1) ranks two losers holding as many systems: control takes the only
// colonies of a system (StarSystem.Controller), a bot spread over shared
// systems may control none but its home.
func versusRunFitness(r versusFitResult, days int) float64 {
	switch {
	case r.winner == 0:
		return 2 - float64(r.day)/float64(days)
	case r.colonies == 0:
		return 0
	}
	return (float64(r.systems) + float64(r.colonies)/float64(r.planets)) / float64(need(3, r.stars))
}

// versusFit plays one versus-fitness game on seed: cfg.empires empires,
// sp as empire 0 played by bc, the others bot.Default(), until an empire
// controls more than two-thirds of the stars or cfg.days.
func versusFit(cfg config, sp game.Species, seed uint64, bc bot.Config) versusFitResult {
	g := versusGame(seed, sp, cfg.density, cfg.empires)
	bots := bot.NewAll(g, bot.Default())
	bots[0] = bot.New(g.Empires[0], bc)
	bots[0].G = g
	r := versusFitResult{stars: len(g.Cluster.Stars), winner: -1}
	for _, st := range g.Cluster.Stars {
		r.planets += len(st.System.Planets)
	}
	n := need(3, r.stars)
	for range cfg.days {
		bot.Day(g, bots)
		for i, e := range g.Empires {
			if e.ControlledSystems() >= n {
				r.winner = i
				break
			}
		}
		if r.winner >= 0 {
			break
		}
	}
	e := g.Empires[0]
	r.day, r.systems, r.colonies = g.Day, e.ControlledSystems(), len(e.Colonies)
	return r
}

// score is a genome's mean run fitness over its runs, and its wins.
func score(fits []float64, won []bool) (fit float64, wins int) {
	for i, f := range fits {
		fit += f
		if won[i] {
			wins++
		}
	}
	return fit / float64(len(fits)), wins
}

// genomeConfig is the bot a genome plays: the default script, greedy,
// the genome (a private copy per game); nil is bot.Default() exactly.
func genomeConfig(g *bot.Genome) bot.Config {
	if g == nil {
		return bot.Default()
	}
	cp := *g
	return bot.Config{Script: bot.Default().Script, Greedy: true, Genome: &cp}
}

// evaluate plays every genome on every seed (one parallel job per pair,
// each writing its slot), solo or versus by cfg.versusFit, and scores
// them in order.
func evaluate(cfg config, sp game.Species, gs []*bot.Genome) (fit []float64, wins []int) {
	n := len(cfg.seeds)
	fits, won := make([]float64, len(gs)*n), make([]bool, len(gs)*n)
	parallel(len(fits), cfg.jobs, func(i int) {
		bc, seed := genomeConfig(gs[i/n]), cfg.seeds[i%n]
		if cfg.versusFit {
			r := versusFit(cfg, sp, seed, bc)
			fits[i], won[i] = versusRunFitness(r, cfg.days), r.winner == 0
			return
		}
		r := solo(sp, seed, cfg.density, cfg.days, bc)
		fits[i], won[i] = runFitness(r, cfg.days), r.mile[2] != 0
	})
	fit, wins = make([]float64, len(gs)), make([]int, len(gs))
	for k := range gs {
		fit[k], wins[k] = score(fits[k*n:(k+1)*n], won[k*n:(k+1)*n])
	}
	return fit, wins
}

// fitnessName is the evolve header's word for the fitness.
func fitnessName(cfg config) string {
	if cfg.versusFit {
		return fmt.Sprintf("fitness versus %d empires", cfg.empires)
	}
	return "fitness solo"
}

// member is one genome of a population; scored once evaluated (an elite
// keeps its score: the policy is deterministic).
type member struct {
	g      *bot.Genome
	fit    float64
	wins   int
	scored bool
}

// genStat is one generation's line.
type genStat struct {
	best, median, spread float64
	wins                 int // the best genome's runs won
}

// evolveResult is one species' GA.
type evolveResult struct {
	species       game.Species
	gens          []genStat
	best          *bot.Genome
	bestFit       float64
	neutral, dflt float64 // Neutral's and bot.Default()'s fitness
}

// speciesRand is a species' GA stream: PCG(seed, species+1).
func speciesRand(seed uint64, sp game.Species) *rand.Rand {
	return rand.New(rand.NewPCG(seed, uint64(sp)+1))
}

// initPop is gen 0: Neutral, then pop−1 mutants of it.
func initPop(pop int, r *rand.Rand) []member {
	ms := []member{{g: bot.Neutral()}}
	for len(ms) < pop {
		g := bot.Neutral()
		mutate(g, r, pmGen0, sigmaGen0)
		ms = append(ms, member{g: g})
	}
	return ms
}

// breed makes the next population from ms sorted best first: the elites
// copied with their scores, then tournament children of generation k.
func breed(ms []member, k, gens int, r *rand.Rand) []member {
	fit := make([]float64, len(ms))
	for i, m := range ms {
		fit[i] = m.fit
	}
	next := slices.Clone(ms[:elites])
	sigma := sigmaAt(k, gens)
	for len(next) < len(ms) {
		a, b := ms[tournament(fit, r)].g, ms[tournament(fit, r)].g
		c := crossover(a, b, r)
		mutate(c, r, pmBreed, sigma)
		next = append(next, member{g: c})
	}
	return next
}

// spread is the mean over genes of the population's standard deviation.
func spread(ms []member) float64 {
	cols := make([][]*float64, len(ms))
	for i, m := range ms {
		cols[i] = genes(m.g)
	}
	n := float64(len(ms))
	total := 0.0
	for k := range cols[0] {
		mean := 0.0
		for _, c := range cols {
			mean += *c[k]
		}
		mean /= n
		v := 0.0
		for _, c := range cols {
			d := *c[k] - mean
			v += d * d
		}
		total += math.Sqrt(v / n)
	}
	return total / float64(len(cols[0]))
}

// evolve runs one species' GA. Every PRNG draw is here, in the calling
// goroutine and a fixed order; only the games run in parallel. progress
// gets a line per generation (stderr: it carries times).
func evolve(cfg config, sp game.Species, progress io.Writer) evolveResult {
	r := speciesRand(cfg.rng, sp)
	res := evolveResult{species: sp}
	ms := initPop(cfg.pop, r)
	t0 := time.Now()
	for k := range cfg.gens {
		gc := cfg
		gc.seeds = genSeeds(cfg, k)
		if cfg.fresh {
			for i := range ms {
				ms[i].scored = false // new clusters: the elites play again
			}
		}
		var todo []*bot.Genome
		var at []int
		for i, m := range ms {
			if !m.scored {
				todo, at = append(todo, m.g), append(at, i)
			}
		}
		fit, wins := evaluate(gc, sp, todo)
		for j, i := range at {
			ms[i].fit, ms[i].wins, ms[i].scored = fit[j], wins[j], true
		}
		if k == 0 {
			res.neutral = ms[0].fit // Neutral is gen 0's first member
		}
		slices.SortStableFunc(ms, func(a, b member) int { return cmpDesc(a.fit, b.fit) })
		fs := make([]float64, len(ms))
		for i, m := range ms {
			fs[i] = m.fit
		}
		// ms is sorted desc: the lower middle of the ascending order.
		res.gens = append(res.gens, genStat{best: ms[0].fit, median: fs[len(fs)/2], spread: spread(ms), wins: ms[0].wins})
		fmt.Fprintf(progress, "evolve: %s gen %d/%d best %.3f (%s)\n", sp, k+1, cfg.gens, ms[0].fit, time.Since(t0).Round(time.Second))
		if k < cfg.gens-1 {
			ms = breed(ms, k+1, cfg.gens, r)
		}
	}
	res.best, res.bestFit = ms[0].g, ms[0].fit
	if cfg.fresh {
		// The comparison on the hold-out -seeds, which no generation played.
		f, _ := evaluate(cfg, sp, []*bot.Genome{res.best, bot.Neutral(), nil})
		res.bestFit, res.neutral, res.dflt = f[0], f[1], f[2]
		return res
	}
	d, _ := evaluate(cfg, sp, []*bot.Genome{nil})
	res.dflt = d[0]
	return res
}

// evolveAll runs one GA per species in list order, printing each and
// writing its best genome to cfg.out when set.
func evolveAll(w, progress io.Writer, cfg config) error {
	if cfg.out != "" {
		if err := os.MkdirAll(cfg.out, 0o755); err != nil {
			return err
		}
	}
	for i, sp := range cfg.species {
		if i > 0 {
			fmt.Fprintln(w)
		}
		res := evolve(cfg, sp, progress)
		printEvolve(w, cfg, res)
		if cfg.out != "" {
			if err := bot.WriteGenome(filepath.Join(cfg.out, strings.ToLower(sp.String())+".json"), res.best); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedList prints seeds as written runs: 1-4,9.
func seedList(seeds []uint64) string {
	var parts []string
	for i := 0; i < len(seeds); {
		j := i
		for j+1 < len(seeds) && seeds[j+1] == seeds[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d-%d", seeds[i], seeds[j]))
		} else {
			parts = append(parts, fmt.Sprint(seeds[i]))
		}
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// topTechs is how many Tech weights printEvolve lists.
const topTechs = 10

// printEvolve writes one species' GA: the generations, the comparison,
// then the best genome (structures, top techs, scalars).
func printEvolve(w io.Writer, cfg config, res evolveResult) {
	fresh := ""
	if cfg.fresh {
		fresh = fmt.Sprintf(", fresh clusters +%d per gen", freshStride)
	}
	fmt.Fprintf(w, "evolve: %s, %s, seeds %s%s, pop %d, gens %d, cap %d days, rng %d, %s\n\n",
		res.species, cfg.density, seedList(cfg.seeds), fresh, cfg.pop, cfg.gens, cfg.days, cfg.rng, fitnessName(cfg))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "gen\tbest\tmedian\tspread\twins")
	for k, s := range res.gens {
		fmt.Fprintf(tw, "%d\t%.3f\t%.3f\t%.3f\t%d/%d\n", k, s.best, s.median, s.spread, s.wins, len(cfg.seeds))
	}
	tw.Flush()
	held := ""
	if cfg.fresh {
		held = fmt.Sprintf(" on the hold-out seeds %s", seedList(cfg.seeds))
	}
	fmt.Fprintf(w, "\nbest vs neutral vs default%s: %.3f %.3f %.3f\n\n", held, res.bestFit, res.neutral, res.dflt)

	g := res.best
	var items []game.Item
	for it := game.Item(0); it < game.FirstProject; it++ {
		// bot.GenomePick never weighs these (founding, the Shipyard rule,
		// BuildShip): their genes drift and mean nothing.
		if it != game.ColonyBase && it != game.Shipyard && it != game.Ship {
			items = append(items, it)
		}
	}
	slices.SortStableFunc(items, func(a, b game.Item) int { return cmpDesc(g.Build[a], g.Build[b]) })
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "structure\tweight")
	for _, it := range items {
		if g.Build[it] <= 0 {
			fmt.Fprintf(tw, "%s\tnever\n", it)
		} else {
			fmt.Fprintf(tw, "%s\t%.2f\n", it, g.Build[it])
		}
	}
	tw.Flush()

	techs := make([]game.Tech, game.NumTechs)
	for t := range techs {
		techs[t] = game.Tech(t)
	}
	slices.SortStableFunc(techs, func(a, b game.Tech) int { return cmpDesc(g.Tech[a], g.Tech[b]) })
	fmt.Fprintln(w)
	tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "tech\tweight")
	for _, t := range techs[:min(topTechs, len(techs))] {
		fmt.Fprintf(tw, "%s\t%.2f\n", t, g.Tech[t])
	}
	tw.Flush()

	fmt.Fprintf(w, "\nBalance %.2f  CostW %.2f  ResCostW %.2f  ShipyardFactories %.2f  FrontierWorth %.2f  RuinWorth %.2f  IdleParty %.2f  AutomateCostW %.2f  FoundFloor %.2f  InFlight %.2f  AbandonIdle %.2f\n",
		g.Balance, g.CostW, g.ResCostW, g.ShipyardFactories, g.FrontierWorth, g.RuinWorth, g.IdleParty, g.AutomateCostW, g.FoundFloor, g.InFlight, g.AbandonIdle)
	ht := make([]string, len(g.HullThreat))
	for i, x := range g.HullThreat {
		ht[i] = fmt.Sprintf("%.2f", x)
	}
	fmt.Fprintf(w, "HullThreat %s  FleeRatio %.2f  AttackRatio %.2f  ThreatAvoid %.2f  WarshipShare %.2f  DefendW %.2f\n",
		strings.Join(ht, "/"), g.FleeRatio, g.AttackRatio, g.ThreatAvoid, g.WarshipShare, g.DefendW)
	hr := make([]string, len(g.HullReach))
	for i, x := range g.HullReach {
		hr[i] = fmt.Sprintf("%.2f", x)
	}
	fmt.Fprintf(w, "HullReach %s  Standoff %.2f  Rounds %.2f  RefitGain %.2f  RepairAt %.2f  InvadeMargin %.2f  InvadeShare %.2f\n",
		strings.Join(hr, "/"), g.Standoff, g.Rounds, g.RefitGain, g.RepairAt, g.InvadeMargin, g.InvadeShare)
	fmt.Fprintf(w, "ShieldWorth %.2f  ShieldPop %.2f  OrbitalShields %.2f  OrbitalGuns %.2f  OrbitalCloakers %.2f  SurfaceCloakers %.2f\n",
		g.ShieldWorth, g.ShieldPop, g.OrbitalShields, g.OrbitalGuns, g.OrbitalCloakers, g.SurfaceCloakers)
	fmt.Fprintf(w, "Temper %.2f  EnvyW %.2f  GrudgeShip %.2f  GrudgePlanet %.2f  GrudgeWar %.2f  GrudgeRefusal %.2f  IntrudeW %.2f  Forgive %.3f  WarCaution %.2f  WearyW %.2f\n",
		g.Temper, g.EnvyW, g.GrudgeShip, g.GrudgePlanet, g.GrudgeWar, g.GrudgeRefusal, g.IntrudeW, g.Forgive, g.WarCaution, g.WearyW)
	fmt.Fprintf(w, "TradeW %.2f  HospW %.2f  GrudgeBreak %.2f  TradeFair %.2f  TradeZeal %.2f  Hospitable %.2f  FavourW %.2f  AskZeal %.2f  DefameW %.2f  FreeResearchDays %.2f  GrudgeEncroach %.2f  Rage %.2f  HelpW %.2f\n",
		g.TradeW, g.HospW, g.GrudgeBreak, g.TradeFair, g.TradeZeal, g.Hospitable, g.FavourW, g.AskZeal, g.DefameW, g.FreeResearchDays, g.GrudgeEncroach, g.Rage, g.HelpW)
	fmt.Fprintf(w, "CannibalAt %.2f  ShieldAt %.2f  RecallAt %.2f  SmartBombAt %.2f  ReplaceDays %.2f  ReplaceFull %.2f  ShieldFirst %.2f\n",
		g.CannibalAt, g.ShieldAt, g.RecallAt, g.SmartBombAt, g.ReplaceDays, g.ReplaceFull, g.ShieldFirst)
	fmt.Fprintf(w, "HabitatShare %.2f  HabitatFill %.2f\n", g.HabitatShare, g.HabitatFill)
}

// cmpDesc orders floats high first (a stable sort keeps index order on ties).
func cmpDesc(a, b float64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	}
	return 0
}
