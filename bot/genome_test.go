package bot

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// soloGenome plays seed 1 Average solo with cfg for days, calling each
// after every Day.
func soloGenome(cfg Config, days int, each func(g *game.Game, b *Bot)) *game.Game {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	bots := NewAll(g, cfg)
	for range days {
		Day(g, bots)
		if each != nil {
			each(g, bots[0])
		}
	}
	return g
}

func neutralCfg() Config {
	cfg := Default()
	cfg.Genome = Neutral()
	return cfg
}

// The same genome on the same game plays the same days: systems and
// every colony's project, day by day.
func TestGenomeDeterministic(t *testing.T) {
	trace := func() []string {
		var out []string
		soloGenome(neutralCfg(), 600, func(g *game.Game, b *Bot) {
			s := fmt.Sprint(b.E.ControlledSystems())
			for _, col := range b.E.Colonies {
				it := game.NoItem
				if col.Project != nil {
					it = col.Project.Item
				}
				s += fmt.Sprint(" ", it)
			}
			out = append(out, s)
		})
		return out
	}
	a, b := trace(), trace()
	if !reflect.DeepEqual(a, b) {
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("day %d: %q then %q", i+1, a[i], b[i])
			}
		}
	}
}

// Neutral expands: on day 1000 of seed 1 it holds 15 systems and 42
// colonies (measured; the fixed policy the same). Loosely:
// more than one system, several colonies.
func TestNeutralSpreads(t *testing.T) {
	g := soloGenome(neutralCfg(), 1000, nil)
	e := g.Empires[0]
	if e.ControlledSystems() < 2 || len(e.Colonies) < 4 {
		t.Errorf("day 1000: %d systems, %d colonies", e.ControlledSystems(), len(e.Colonies))
	}
}

// Build[Factory] 0: the pick never starts a Factory (no script, so any
// Factory project is the pick's), while it keeps picking others
// (Agriplots, measured 464 colony-days of 600).
func TestGenomeNoFactory(t *testing.T) {
	cfg := neutralCfg()
	cfg.Script = nil
	cfg.Genome.Build[game.Factory] = 0
	n, others := 0, 0
	soloGenome(cfg, 600, func(g *game.Game, b *Bot) {
		for _, col := range b.E.Colonies {
			switch {
			case col.Project == nil:
			case col.Project.Item == game.Factory:
				n++
			default:
				others++
			}
		}
	})
	if n > 0 || others == 0 {
		t.Errorf("%d colony-days on a Factory project, %d on other picks", n, others)
	}
}

// PickTopicBy: on a fresh Research, the Available tech of the highest
// score; a tech's weight makes it win; a heavy ResCostW the cheapest.
func TestPickTopicBy(t *testing.T) {
	av := game.NewResearch().Available()
	if len(av) < 2 {
		t.Fatalf("fresh Available %v", av)
	}
	pick := func(g *Genome) game.Tech {
		r := game.NewResearch()
		PickTopicBy(r, g)
		return r.Topic
	}
	score := func(g *Genome, t game.Tech) float64 {
		return g.Tech[t] - g.ResCostW*math.Log(float64(game.NewResearch().Cost(t)))
	}
	n := Neutral()
	want := av[0]
	for _, t := range av {
		if score(n, t) > score(n, want) {
			want = t
		}
	}
	if got := pick(n); got != want {
		t.Errorf("Neutral picks %v, the best score is %v", got, want)
	}
	last := av[len(av)-1]
	n.Tech[last] = 10
	if got := pick(n); got != last {
		t.Errorf("Tech[%v] 10 picks %v", last, got)
	}
	cheap := &Genome{ResCostW: 10}
	want = av[0]
	r := game.NewResearch()
	for _, t := range av {
		if r.Cost(t) < r.Cost(want) {
			want = t
		}
	}
	if got := pick(cheap); got != want {
		t.Errorf("ResCostW 10 picks %v (cost %d), the cheapest is %v (%d)", got, r.Cost(got), want, r.Cost(want))
	}
	// A topic already set stays.
	r.Study(last)
	PickTopicBy(r, cheap)
	if r.Topic != last {
		t.Errorf("topic %v replaced by %v", last, r.Topic)
	}
}

// candidate is the oracle's filter: GenomePick's rule read off the brief.
func candidate(col *game.Colony, r *game.Research, g *Genome, it game.Item) bool {
	if it == game.ColonyBase || it == game.Shipyard || it == game.Ship || g.Build[it] <= 0 || !upgradeOpen(col, it, g.Upgrade) {
		return false
	}
	if it.Orbital() {
		for sl := range game.OrbitSlots {
			if col.CanStartOrbital(r, it, sl) == nil {
				return true
			}
		}
		return false
	}
	at, ok := col.Planet.BestCell(it, col.Traits())
	return ok && col.CanStart(r, it, at) == nil
}

// GenomePick on every idle colony of a 600-day game, with a genome that
// weighs ColonyBase, Shipyard and Ship highest and every third item
// negative: never an item of Build <= 0 nor those three; a start that
// succeeds where it says; no candidate of a higher score (ties lower
// index).
func TestGenomePick(t *testing.T) {
	gn := &Genome{Balance: 2, CostW: 0.5}
	for it := range gn.Build {
		gn.Build[it] = float64(it%5) - 1 // -1, 0, 1, 2, 3, ...
	}
	gn.Build[game.ColonyBase], gn.Build[game.Shipyard], gn.Build[game.Ship] = 10, 10, 10
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	bots := NewAll(g, Default())
	picks := map[game.Item]int{}
	for range 600 {
		e := g.Empires[0]
		for _, col := range e.Colonies {
			if col.Project != nil {
				continue
			}
			it, at, slot := GenomePick(col, e.Research, gn)
			if it == game.NoItem {
				for c := game.Item(0); c < game.FirstProject; c++ {
					if candidate(col, e.Research, gn, c) {
						t.Fatalf("day %d %s: NoItem, %v could start", g.Day, col.Planet.Name, c)
					}
				}
				continue
			}
			picks[it]++
			if !candidate(col, e.Research, gn, it) {
				t.Fatalf("day %d %s: picked %v (Build %v)", g.Day, col.Planet.Name, it, gn.Build[it])
			}
			makes := wants(col, gn.Economy())
			score := func(c game.Item) float64 {
				s := gn.Build[c] - gn.CostW*float64(c.Info().Cost)/100
				if makes(c.Info()) {
					s += gn.Balance
				}
				return s
			}
			for c := game.Item(0); c < game.FirstProject; c++ {
				if candidate(col, e.Research, gn, c) && (score(c) > score(it) || score(c) == score(it) && c < it) {
					t.Fatalf("day %d %s: picked %v (%.2f), %v scores %.2f", g.Day, col.Planet.Name, it, score(it), c, score(c))
				}
			}
			cp := *col
			cp.Project = nil
			var err error
			if it.Orbital() {
				err = cp.StartOrbital(e.Research, it, slot)
			} else if slot != -1 {
				err = fmt.Errorf("surface %v with slot %d", it, slot)
			} else {
				err = cp.Start(e.Research, it, at)
			}
			if err != nil {
				t.Fatalf("day %d %s: %v at %v/%d: %v", g.Day, col.Planet.Name, it, at, slot, err)
			}
		}
		Day(g, bots)
	}
	if len(picks) < 2 {
		t.Errorf("picks %v: too few items walked", picks)
	}
	t.Logf("picks %v", picks)
}

// Act researches by PickTopicBy with a genome (the queue unused), and
// not at all under NoResearch.
func TestActResearchesByGenome(t *testing.T) {
	av := game.NewResearch().Available()
	last := av[len(av)-1]
	for _, none := range []bool{false, true} {
		e := game.NewEmpire(1, game.Snovemdomas, game.Average)
		cfg := Default()
		cfg.Genome, cfg.NoResearch = &Genome{}, none
		cfg.Genome.Tech[last] = 10
		New(e, cfg).Act()
		if want := map[bool]game.Tech{false: last, true: game.NoTech}[none]; e.Research.Topic != want {
			t.Errorf("NoResearch %v: topic %v, want %v", none, e.Research.Topic, want)
		}
	}
}

// claimed counts FrontierWorth (rounded) per frontier star with a
// genome, frontierWorth without. The genomes carry the fixed policy's
// FoundFloor so the foundable planets, hence spare, differ only by the
// frontier.
func TestGenomeFrontierWorth(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	spare := func(gn *Genome) int {
		cfg := Default()
		cfg.Genome = gn
		b := New(e, cfg)
		b.claimed()
		return b.spare
	}
	base, s0 := spare(nil), spare(&Genome{FoundFloor: foundFloor})
	k := (base - s0) / frontierWorth
	if k < 1 || base-s0 != k*frontierWorth {
		t.Fatalf("spare %d fixed, %d at FrontierWorth 0", base, s0)
	}
	if got := spare(&Genome{FoundFloor: foundFloor, FrontierWorth: 5.6}); got != s0+6*k {
		t.Errorf("FrontierWorth 5.6: spare %d, want %d", got, s0+6*k)
	}
}

// The Shipyard waits for ShipyardFactories (rounded) Factories with a
// genome, two without.
func TestGenomeShipyardFactories(t *testing.T) {
	for _, tc := range []struct {
		gn   *Genome
		yard bool
	}{{nil, false}, {&Genome{ShipyardFactories: 0.4}, true}, {&Genome{ShipyardFactories: 2.6}, false}} {
		e := game.NewEmpire(1, game.Snovemdomas, game.Average)
		e.Research.Known[game.OrbitalStructures] = true
		home := e.Colonies[0]
		if n := home.Planet.Count(game.Factory); n >= 2 {
			t.Fatalf("the home stands %d Factories already", n)
		}
		b := New(e, Config{Genome: tc.gn})
		b.planColony(0, home, b.claimed())
		if got := home.Project != nil && home.Project.Item == game.Shipyard; got != tc.yard {
			t.Errorf("genome %+v: shipyard started %v, want %v (project %+v)", tc.gn, got, tc.yard, home.Project)
		}
	}
}

// A genome file older than a gene plays that gene Neutral, not 0
// (else genes added later read as 0 from an older file).
func TestReadGenomeFillsNeutral(t *testing.T) {
	path := t.TempDir() + "/old.json"
	if err := os.WriteFile(path, []byte(`{"Temper": 5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := ReadGenome(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := NeutralDiplomacy(); g.Temper != 5 || g.TradeW != n.TradeW || g.TradeFair != n.TradeFair || g.Hospitable != n.Hospitable ||
		g.FavourW != n.FavourW || g.AskZeal != n.AskZeal || g.DefameW != n.DefameW || g.FreeResearchDays != n.FreeResearchDays {
		t.Errorf("read %+v, want Temper 5 and the rest Neutral", g.Diplomacy())
	}
	if n := NeutralCombat(); g.CannibalAt != n.CannibalAt || g.ShieldAt != n.ShieldAt || g.RecallAt != n.RecallAt || n.RecallAt == 0 ||
		g.SmartBombAt != n.SmartBombAt || n.SmartBombAt == 0 {
		t.Errorf("specials' genes %v %v %v %v, want Neutral %+v", g.CannibalAt, g.ShieldAt, g.RecallAt, g.SmartBombAt, n)
	}
	// Written and read back: the four genes survive the file.
	g.CannibalAt, g.ShieldAt, g.RecallAt, g.SmartBombAt = 0.7, 0.3, 0.1, 5
	if err := WriteGenome(path, g); err != nil {
		t.Fatal(err)
	}
	if h, err := ReadGenome(path); err != nil || h.CannibalAt != 0.7 || h.ShieldAt != 0.3 || h.RecallAt != 0.1 || h.SmartBombAt != 5 {
		t.Errorf("round trip: %+v, %v", h.Combat(), err)
	}
}
