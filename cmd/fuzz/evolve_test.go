package main

import (
	"bytes"
	"io"
	"math"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// evolveCfg is a small, fast GA: Vacuous, 2 seeds, a short cap.
func evolveCfg(t *testing.T, species string, jobs int, rng uint64) config {
	t.Helper()
	cfg, err := parseConfig("evolve", "1-2", species, "vacuous", 300, 0, jobs)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.setEvolve(5, 3, rng, ""); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSetEvolve(t *testing.T) {
	var cfg config
	for _, c := range [][2]int{{3, 1}, {4, 0}} {
		if err := cfg.setEvolve(c[0], c[1], 1, ""); err == nil {
			t.Errorf("pop %d gens %d accepted", c[0], c[1])
		}
	}
	if err := cfg.setEvolve(4, 1, 9, "d"); err != nil || cfg.pop != 4 || cfg.gens != 1 || cfg.rng != 9 || cfg.out != "d" {
		t.Errorf("setEvolve: %+v %v", cfg, err)
	}
	if cfg, err := parseConfig("evolve", "1", "orfa,1", "vacuous", 10, 0, 1); err != nil || !cfg.evolve || len(cfg.species) != 2 {
		t.Errorf("parseConfig evolve: %+v %v", cfg, err)
	}
}

// Output byte-identical whatever -j and across runs; another -rng draws
// other gen-0 genomes.
func TestEvolveDeterministic(t *testing.T) {
	run := func(cfg config) string {
		var b bytes.Buffer
		if err := evolveAll(&b, io.Discard, cfg); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	a := run(evolveCfg(t, "0,orfa", 1, 7))
	if b := run(evolveCfg(t, "0,orfa", 3, 7)); a != b {
		t.Errorf("-j 1 vs -j 3:\n%s\n%s", a, b)
	}
	if b := run(evolveCfg(t, "0,orfa", 3, 7)); a != b {
		t.Errorf("two runs, same -rng:\n%s\n%s", a, b)
	}
	if !strings.HasPrefix(a, "evolve: Minions, Vacuous, seeds 1-2, pop 5, gens 3, cap 300 days, rng 7, fitness solo\n") ||
		!strings.Contains(a, "\nevolve: Orfa, ") {
		t.Errorf("headers:\n%s", a)
	}

	pop := func(rng uint64, sp game.Species) []member {
		return initPop(6, speciesRand(rng, sp))
	}
	p7, p8, q7 := pop(7, 0), pop(7, 0), pop(8, 0)
	if !reflect.DeepEqual(p7, p8) {
		t.Error("gen 0 differs on the same rng")
	}
	if !reflect.DeepEqual(p7[0].g, bot.Neutral()) || !reflect.DeepEqual(q7[0].g, bot.Neutral()) {
		t.Error("gen 0's first genome is not Neutral")
	}
	for i := 1; i < 6; i++ {
		if reflect.DeepEqual(p7[i].g, q7[i].g) {
			t.Errorf("rng 7 and 8 draw the same mutant %d", i)
		}
		if reflect.DeepEqual(p7[i].g, bot.Neutral()) {
			t.Errorf("mutant %d is Neutral", i)
		}
	}
	if reflect.DeepEqual(p7[1].g, pop(7, 1)[1].g) {
		t.Error("two species share a stream")
	}
}

// The best fitness never drops: the elites keep theirs.
func TestEvolveElitism(t *testing.T) {
	cfg := evolveCfg(t, "orfa", 4, 3)
	res := evolve(cfg, game.Orfa, io.Discard)
	if len(res.gens) != cfg.gens {
		t.Fatalf("%d generations, want %d", len(res.gens), cfg.gens)
	}
	for k := 1; k < len(res.gens); k++ {
		if res.gens[k].best < res.gens[k-1].best {
			t.Errorf("best fitness %v drops at gen %d", res.gens, k)
		}
	}
	if res.bestFit != res.gens[len(res.gens)-1].best {
		t.Errorf("bestFit %v, last gen %v", res.bestFit, res.gens[len(res.gens)-1])
	}

	// breed on a handcrafted sorted population: the top 2 carried with
	// their scores, the rest unscored children.
	r := rand.New(rand.NewPCG(1, 2))
	ms := initPop(6, r)
	for i := range ms {
		ms[i].fit, ms[i].wins, ms[i].scored = float64(10-i), 2, true
	}
	next := breed(ms, 1, 3, r)
	if len(next) != len(ms) {
		t.Fatalf("next population %d, want %d", len(next), len(ms))
	}
	for i, m := range next {
		elite := i < elites
		if m.scored != elite || elite && (m.g != ms[i].g || m.fit != ms[i].fit) {
			t.Errorf("next[%d] = %+v (elite %v)", i, m, elite)
		}
	}
}

func TestCrossover(t *testing.T) {
	var a, b bot.Genome
	for i := range a.Build {
		a.Build[i], b.Build[i] = 1, -1
	}
	for i := range a.Tech {
		a.Tech[i], b.Tech[i] = 2, -2
	}
	for i := range a.HullThreat {
		a.HullThreat[i], b.HullThreat[i] = 3, -3
	}
	for i := range a.HullReach {
		a.HullReach[i], b.HullReach[i] = 4, -4
	}
	a.Balance, a.CostW, a.ResCostW, a.ShipyardFactories, a.FrontierWorth = 1, 2, 3, 4, 5
	b.Balance, b.CostW, b.ResCostW, b.ShipyardFactories, b.FrontierWorth = 6, 7, 8, 9, 10
	a.FleeRatio, a.AttackRatio, a.ThreatAvoid, a.WarshipShare, a.DefendW, a.Standoff, a.Rounds = 11, 12, 13, 14, 15, 16, 17
	b.FleeRatio, b.AttackRatio, b.ThreatAvoid, b.WarshipShare, b.DefendW, b.Standoff, b.Rounds = 21, 22, 23, 24, 25, 26, 27
	a.RefitGain, a.RepairAt, b.RefitGain, b.RepairAt = 18, 19, 28, 29
	a.RuinWorth, b.RuinWorth = 20, 30
	a.IdleParty, b.IdleParty = 0.25, 0.75
	a.AutomateCostW, b.AutomateCostW = -0.5, 0.5
	a.FoundFloor, a.InFlight, b.FoundFloor, b.InFlight = 2, 10, 4, 20
	a.AbandonIdle, b.AbandonIdle = 30, 90
	a.InvadeMargin, a.InvadeShare, b.InvadeMargin, b.InvadeShare = 31, 32, 41, 42
	a.ShieldWorth, a.ShieldPop, a.OrbitalShields, a.OrbitalGuns = 1, 3, 1, 2
	b.ShieldWorth, b.ShieldPop, b.OrbitalShields, b.OrbitalGuns = 6, 9, 3, 4
	a.OrbitalCloakers, a.SurfaceCloakers, b.OrbitalCloakers, b.SurfaceCloakers = 0.2, 0.3, 0.7, 0.8
	a.Temper, a.EnvyW, a.GrudgeShip, a.GrudgePlanet, a.GrudgeWar = 51, 52, 53, 54, 55
	b.Temper, b.EnvyW, b.GrudgeShip, b.GrudgePlanet, b.GrudgeWar = 61, 62, 63, 64, 65
	a.GrudgeRefusal, a.IntrudeW, a.Forgive, a.WarCaution, a.WearyW = 56, 57, 0.05, 58, 59
	b.GrudgeRefusal, b.IntrudeW, b.Forgive, b.WarCaution, b.WearyW = 66, 67, 0.15, 68, 69
	a.TradeW, a.HospW, a.GrudgeBreak, a.TradeFair, a.TradeZeal, a.Hospitable = 71, 0.5, 72, 1, 73, 0.25
	b.TradeW, b.HospW, b.GrudgeBreak, b.TradeFair, b.TradeZeal, b.Hospitable = 81, 1.5, 82, 2, 83, 0.75
	a.FavourW, a.AskZeal, a.DefameW, a.FreeResearchDays = 74, 75, 76, 77
	b.FavourW, b.AskZeal, b.DefameW, b.FreeResearchDays = 84, 85, 86, 87
	a.CannibalAt, a.ShieldAt, a.RecallAt = 0.1, 0.2, 0.3
	b.CannibalAt, b.ShieldAt, b.RecallAt = 0.6, 0.7, 0.8
	a.SmartBombAt, b.SmartBombAt = 3, 4
	a.ReplaceDays, a.ReplaceFull, b.ReplaceDays, b.ReplaceFull = 100, 0.25, 200, 0.75
	a.ShieldFirst, b.ShieldFirst = 0.2, 0.8
	a.Upgrade, a.IndShare, a.ResShare, b.Upgrade, b.IndShare, b.ResShare = 1, 0.2, 0.3, 5, 0.4, 0.5
	a.IndFloor, b.IndFloor = 2, 6
	a.GrudgeEncroach, b.GrudgeEncroach = 50, 200
	a.Rage, b.Rage = 30, 300
	a.HabitatShare, a.HabitatFill, b.HabitatShare, b.HabitatFill = 0.5, 0.25, 2, 0.75
	a.HelpW, b.HelpW = 10, 90
	a0, b0 := a, b
	r := rand.New(rand.NewPCG(5, 6))
	// Build, Tech, HullThreat, HullReach, then every scalar of both groups.
	var fromB [4 + scalarGenes + combatGenes]int
	const n = 64
	for range n {
		c := crossover(&a, &b, r)
		switch c.Build {
		case a.Build:
		case b.Build:
			fromB[0]++
		default:
			t.Fatalf("Build mixed: %v", c.Build)
		}
		switch c.Tech {
		case a.Tech:
		case b.Tech:
			fromB[1]++
		default:
			t.Fatalf("Tech mixed: %v", c.Tech)
		}
		switch c.HullThreat {
		case a.HullThreat:
		case b.HullThreat:
			fromB[2]++
		default:
			t.Fatalf("HullThreat mixed: %v", c.HullThreat)
		}
		switch c.HullReach {
		case a.HullReach:
		case b.HullReach:
			fromB[3]++
		default:
			t.Fatalf("HullReach mixed: %v", c.HullReach)
		}
		sa := append(scalars(&a), combatScalars(&a)...)
		sb := append(scalars(&b), combatScalars(&b)...)
		sc := append(scalars(c), combatScalars(c)...)
		for k := range sc {
			switch *sc[k] {
			case *sa[k]:
			case *sb[k]:
				fromB[4+k]++
			default:
				t.Fatalf("scalar %d = %v", k, *sc[k])
			}
		}
	}
	if a != a0 || b != b0 {
		t.Error("crossover touched a parent")
	}
	// Each block's coin is drawn: both parents give it some of the time.
	for k, x := range fromB {
		if x == 0 || x == n {
			t.Errorf("block %d from b %d of %d times", k, x, n)
		}
	}
}

func TestMutate(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 1))
	g := bot.Neutral()
	mutate(g, r, 0, 5)
	if *g != *bot.Neutral() {
		t.Error("pm 0 changed the genome")
	}
	// A gene Neutral holds at a bound (Rounds at RoundCap) stays put when
	// its draw clamps back onto it: each gene must move in some trial.
	moved := make([]bool, len(genes(bot.Neutral())))
	for range 20 {
		g := bot.Neutral()
		mutate(g, r, 1, 1e6)
		c := *g
		c.Clamp()
		if c != *g {
			t.Fatalf("out of bounds: %+v", *g)
		}
		n := bot.Neutral()
		for k, p := range genes(n) {
			moved[k] = moved[k] || *p != *genes(g)[k]
		}
	}
	for k, m := range moved {
		if !m {
			t.Errorf("pm 1: gene %d never changed", k)
		}
	}
}

func TestTournament(t *testing.T) {
	fit := []float64{0.1, 0.9, 0.5, 0.9, 0.2}
	r := rand.New(rand.NewPCG(3, 4))
	twin := rand.New(rand.NewPCG(3, 4))
	for range 200 {
		got := tournament(fit, r)
		var drawn [tourSize]int
		for k := range drawn {
			drawn[k] = twin.IntN(len(fit))
		}
		// The oracle: the highest fitness drawn, then its lowest index.
		top := math.Inf(-1)
		for _, i := range drawn {
			top = max(top, fit[i])
		}
		want := len(fit)
		for _, i := range drawn {
			if fit[i] == top {
				want = min(want, i)
			}
		}
		if got != want {
			t.Fatalf("drawn %v: got %d, want %d", drawn, got, want)
		}
	}
}

func TestFitness(t *testing.T) {
	const days = 1000
	won := soloResult{stars: 15, mile: [3]int{100, 200, 500}, systems: 11}
	late := soloResult{stars: 15, mile: [3]int{100, 200, days}, systems: 11}
	lost := soloResult{stars: 15, mile: [3]int{100, 200, 0}, systems: 10, day: days}
	if f := runFitness(won, days); f != 1.5 {
		t.Errorf("win on day 500 of 1000: %v, want 1.5", f)
	}
	// need(3, 15) = 11.
	if f := runFitness(lost, days); math.Abs(f-10.0/11) > 1e-12 {
		t.Errorf("10 of 15 lost: %v, want 10/11", f)
	}
	if runFitness(late, days) <= runFitness(lost, days) {
		t.Error("a win on the cap day does not beat a loss")
	}
	f, w := score([]float64{runFitness(won, days), runFitness(lost, days)}, []bool{true, false})
	if math.Abs(f-(1.5+10.0/11)/2) > 1e-12 || w != 1 {
		t.Errorf("mean of a win and a loss: %v, %d wins", f, w)
	}
}

func TestSeedList(t *testing.T) {
	for in, want := range map[string]string{"1-4": "1-4", "1-3,9": "1-3,9", "5,2-2": "5,2", "1,3,4": "1,3-4"} {
		s, _ := parseSeeds(in)
		if got := seedList(s); got != want {
			t.Errorf("seedList(%q) = %q, want %q", in, got, want)
		}
	}
}

// -o writes each species' best genome, readable back.
func TestEvolveWrites(t *testing.T) {
	cfg := evolveCfg(t, "orfa", 4, 1)
	cfg.gens = 1
	cfg.out = filepath.Join(t.TempDir(), "g")
	if err := evolveAll(io.Discard, io.Discard, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := bot.ReadGenome(filepath.Join(cfg.out, "orfa.json")); err != nil {
		t.Error(err)
	}
}

// genes covers every float64 of Genome once: its length is the type's
// count of float64s (an array counts its length), each pointer distinct
// and inside the genome.
func TestGenes(t *testing.T) {
	want := 0
	typ := reflect.TypeOf(bot.Genome{})
	for i := range typ.NumField() {
		switch f := typ.Field(i).Type; f.Kind() {
		case reflect.Float64:
			want++
		case reflect.Array:
			if f.Elem().Kind() != reflect.Float64 {
				t.Fatalf("field %s: array of %s", typ.Field(i).Name, f.Elem())
			}
			want += f.Len()
		default:
			t.Fatalf("field %s: %s", typ.Field(i).Name, f)
		}
	}
	var g bot.Genome
	gs := genes(&g)
	if len(gs) != want {
		t.Fatalf("genes: %d, the Genome holds %d", len(gs), want)
	}
	if n := len(scalars(&g)) + len(combatScalars(&g)) + len(g.Build) + len(g.Tech) + len(g.HullThreat) + len(g.HullReach); n != want {
		t.Errorf("blocks hold %d genes, the Genome %d", n, want)
	}
	// Writing k through the k-th pointer leaves every gene distinct: no
	// pointer twice.
	for k, p := range gs {
		*p = float64(k + 1)
	}
	seen := map[float64]bool{}
	for _, p := range genes(&g) {
		if seen[*p] {
			t.Fatalf("gene %v twice", *p)
		}
		seen[*p] = true
	}
	if g.SurfaceCloakers != float64(want) || g.HullReach[0] != float64(want-combatGenes-len(g.HullReach)+1) ||
		g.HullThreat[0] != float64(want-combatGenes-len(g.HullReach)-len(g.HullThreat)+1) {
		t.Errorf("order: HullThreat[0] %v, HullReach[0] %v, SurfaceCloakers %v", g.HullThreat[0], g.HullReach[0], g.SurfaceCloakers)
	}
}

func TestVersusRunFitness(t *testing.T) {
	const days = 1000
	// need(3, 15) = 11; the planets' share is colonies / planets.
	for _, c := range []struct {
		r    versusFitResult
		want float64
	}{
		{versusFitResult{stars: 15, winner: 0, day: 250, systems: 11, colonies: 12}, 1.75},
		{versusFitResult{stars: 15, winner: 0, day: days, systems: 11, colonies: 12}, 1},
		{versusFitResult{stars: 15, planets: 40, winner: 2, day: 400, systems: 3, colonies: 4}, (3 + 4.0/40) / 11},
		{versusFitResult{stars: 15, planets: 40, winner: -1, day: days, systems: 10, colonies: 10}, (10 + 10.0/40) / 11},
		{versusFitResult{stars: 15, planets: 40, winner: 1, day: 600, systems: 2, colonies: 0}, 0},
		// The planets' share ranks two losers holding as many systems.
		{versusFitResult{stars: 15, planets: 40, winner: 1, day: 600, systems: 1, colonies: 8}, (1 + 8.0/40) / 11},
	} {
		if f := versusRunFitness(c.r, days); math.Abs(f-c.want) > 1e-12 {
			t.Errorf("%+v: %v, want %v", c.r, f, c.want)
		}
	}
}

func TestSetFitness(t *testing.T) {
	cfg := evolveCfg(t, "orfa", 1, 1)
	if err := cfg.setFitness("solo"); err != nil || cfg.versusFit {
		t.Errorf("solo: %v %v", cfg.versusFit, err)
	}
	cfg.empires = 2
	if err := cfg.setFitness("versus"); err != nil || !cfg.versusFit {
		t.Errorf("versus: %v %v", cfg.versusFit, err)
	}
	for _, e := range []int{1, int(game.NumSpecies) + 1, game.Vacuous.Stars() + 1} {
		cfg.empires = e
		if err := cfg.setFitness("versus"); err == nil {
			t.Errorf("versus with %d empires accepted", e)
		}
	}
	if err := cfg.setFitness("duel"); err == nil {
		t.Error("-fitness duel accepted")
	}
	solo, _ := parseConfig("solo", "1", "all", "vacuous", 10, 2, 1)
	if err := solo.setFitness("versus"); err == nil {
		t.Error("-fitness versus accepted in solo mode")
	}
}

// versusCfg is evolveCfg on the versus fitness: 3 empires.
func versusCfg(t *testing.T, species string, jobs int) config {
	t.Helper()
	cfg := evolveCfg(t, species, jobs, 7)
	cfg.empires, cfg.gens = 3, 2
	if err := cfg.setFitness("versus"); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestEvolveVersusDeterministic(t *testing.T) {
	run := func(cfg config) string {
		var b bytes.Buffer
		if err := evolveAll(&b, io.Discard, cfg); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	a := run(versusCfg(t, "snovemdomas", 1))
	if b := run(versusCfg(t, "snovemdomas", 3)); a != b {
		t.Errorf("-j 1 vs -j 3:\n%s\n%s", a, b)
	}
	if !strings.HasPrefix(a, "evolve: Snovemdomas, Vacuous, seeds 1-2, pop 5, gens 2, cap 300 days, rng 7, fitness versus 3 empires\n") {
		t.Errorf("header:\n%s", a)
	}
	if !strings.Contains(a, "\nHullThreat ") || !strings.Contains(a, "  DefendW ") {
		t.Errorf("no combat genes:\n%s", a)
	}
}

// The versus fitness game: empire 0 the asked species, empire 0 played
// by the genome, the stop at the first winner.
func TestVersusFit(t *testing.T) {
	cfg := versusCfg(t, "orfa", 1)
	if g := versusGame(1, game.Orfa, cfg.density, cfg.empires); g.Empires[0].Species != game.Orfa || len(g.Empires) != 3 {
		t.Errorf("versusGame: empire 0 %s, %d empires", g.Empires[0].Species, len(g.Empires))
	}
	// The stop: the first of seeds 15-17 (2 empires, Vacuous, 3000 days)
	// where some empire wins ends on that day, before the cap (a fixture
	// coupled to the bot's strength: none winning is a Fatal). Of seeds
	// 1-60 only 15 (day 1484) and 57 win (measured with the scouts;
	// without them 38 alone of 1-40, on day 1649).
	cfg = versusCfg(t, "orfa", 1)
	cfg.empires, cfg.days = 2, 3000
	var def versusFitResult
	for seed := uint64(15); seed <= 17; seed++ {
		if def = versusFit(cfg, game.Orfa, seed, bot.Default()); def.winner >= 0 {
			cfg.seeds = []uint64{seed}
			break
		}
		if def.day != cfg.days {
			t.Fatalf("seed %d: no winner, stopped on day %d", seed, def.day)
		}
	}
	if def.winner < 0 {
		t.Fatalf("no winner on seeds 15-17 in %d days", cfg.days)
	}
	if def.day >= cfg.days || def.winner == 0 && def.systems < need(3, def.stars) {
		t.Errorf("seed %d: %+v, want the stop on the winning day", cfg.seeds[0], def)
	}
	// evaluate scores the same game under -fitness versus.
	if f, w := evaluate(cfg, game.Orfa, []*bot.Genome{nil}); f[0] != versusRunFitness(def, cfg.days) || (w[0] == 1) != (def.winner == 0) {
		t.Errorf("evaluate: %v %v, the game %+v", f, w, def)
	}
	// Empire 0 is the genome's: one that never builds scores below the
	// default bot on the same game.
	idle := bot.Neutral()
	for i := range idle.Build {
		idle.Build[i] = -10
	}
	if low := versusFit(cfg, game.Orfa, cfg.seeds[0], genomeConfig(idle)); versusRunFitness(low, cfg.days) >= versusRunFitness(def, cfg.days) {
		t.Errorf("an idle genome %+v scores no less than the default %+v", low, def)
	}
}

// scaleOf is 1 everywhere but on the diplomacy and the specials' genes,
// which carry their Neutral magnitude (dipScales): Temper 100, Forgive
// 0.01, RecallAt 0.25, SmartBombAt 1, the thirty-one in step with scalars().
func TestScaleOf(t *testing.T) {
	g := bot.Neutral()
	gs := genes(g)
	at := func(p *float64) int {
		for i, q := range gs {
			if q == p {
				return i
			}
		}
		t.Fatal("gene not listed")
		return -1
	}
	for _, c := range []struct {
		p    *float64
		want float64
	}{{&g.Build[0], 1}, {&g.Balance, 1}, {&g.AutomateCostW, 1}, {&g.FoundFloor, 1}, {&g.InFlight, 1}, {&g.AbandonIdle, 60}, {&g.Temper, 100}, {&g.Forgive, 0.01}, {&g.WearyW, 1},
		{&g.TradeW, 30}, {&g.HospW, 0.2}, {&g.GrudgeBreak, 150}, {&g.TradeFair, 0.5}, {&g.TradeZeal, 60}, {&g.Hospitable, 0.1},
		{&g.FavourW, 100}, {&g.AskZeal, 90}, {&g.DefameW, 100}, {&g.FreeResearchDays, 30},
		{&g.CannibalAt, 0.5}, {&g.ShieldAt, 0.5}, {&g.RecallAt, 0.25}, {&g.SmartBombAt, 1}, {&g.ReplaceDays, 100}, {&g.ReplaceFull, 0.5}, {&g.ShieldFirst, 0.5},
		{&g.Upgrade, 2}, {&g.IndShare, 0.1}, {&g.ResShare, 0.1}, {&g.IndFloor, 2}, {&g.GrudgeEncroach, 100}, {&g.Rage, 100},
		{&g.HabitatShare, 0.5}, {&g.HabitatFill, 0.5}, {&g.HelpW, 30},
		{&g.HullThreat[0], 1}, {&g.FleeRatio, 1}} {
		if s := scaleOf(g, at(c.p)); s != c.want {
			t.Errorf("gene %d: scale %v, want %v", at(c.p), s, c.want)
		}
	}
	if n := len(dipScales); scalarGenes-10 != n {
		t.Errorf("%d scales for %d scalars past the first ten", n, scalarGenes-10)
	}
}

// -fresh: generation k plays -seeds + 1000·(k+1), the final comparison
// the hold-out -seeds; refused outside evolve and on a wide -seeds.
func TestEvolveFresh(t *testing.T) {
	cfg := evolveCfg(t, "orfa", 3, 7)
	if got := genSeeds(cfg, 1); !reflect.DeepEqual(got, cfg.seeds) {
		t.Errorf("genSeeds without -fresh: %v", got)
	}
	if err := cfg.setFresh(true); err != nil {
		t.Fatal(err)
	}
	if got := genSeeds(cfg, 0); !reflect.DeepEqual(got, []uint64{1001, 1002}) {
		t.Errorf("gen 0 seeds %v", got)
	}
	if got := genSeeds(cfg, 2); !reflect.DeepEqual(got, []uint64{3001, 3002}) {
		t.Errorf("gen 2 seeds %v", got)
	}
	wide, _ := parseConfig("evolve", "1,1001", "orfa", "vacuous", 300, 0, 1)
	if err := wide.setFresh(true); err == nil {
		t.Error("-seeds 1,1001 accepted")
	}
	solo, _ := parseConfig("solo", "1", "orfa", "vacuous", 300, 0, 1)
	if err := solo.setFresh(true); err == nil {
		t.Error("-fresh accepted in solo mode")
	}

	var a, b bytes.Buffer
	if err := evolveAll(&a, io.Discard, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.jobs = 1
	if err := evolveAll(&b, io.Discard, cfg); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("-j 3 vs -j 1:\n%s\n%s", &a, &b)
	}
	if !strings.HasPrefix(a.String(), "evolve: Orfa, Vacuous, seeds 1-2, fresh clusters +1000 per gen, pop 5,") ||
		!strings.Contains(a.String(), "\nbest vs neutral vs default on the hold-out seeds 1-2: ") {
		t.Errorf("headers:\n%s", &a)
	}
	res := evolve(cfg, game.Orfa, io.Discard)
	f, _ := evaluate(cfg, game.Orfa, []*bot.Genome{res.best, bot.Neutral(), nil})
	if res.bestFit != f[0] || res.neutral != f[1] || res.dflt != f[2] {
		t.Errorf("comparison %v %v %v, on -seeds %v", res.bestFit, res.neutral, res.dflt, f)
	}
}
