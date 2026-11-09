package bot

import (
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// family reads the item table: the three output families and their
// tiers, the mixed producers (Logic Factory, Engineering Retreat) and the
// MaxPop items in none; bestTier climbs with research.
func TestFamilies(t *testing.T) {
	want := [3][]game.Item{
		{game.Factory, game.IndustrialMegafacility},
		{game.Laboratory, game.ResearchCampus},
		{game.Agriplot, game.ArtificialHydroponifer},
	}
	var got [3][]game.Item
	for it := game.Item(0); it < game.NumItems; it++ {
		if f := family(it); f >= 0 {
			got[f] = append(got[f], it)
		}
	}
	for f := range want {
		if !slices.Equal(got[f], want[f]) {
			t.Errorf("family %d: %v, want %v", f, got[f], want[f])
		}
	}
	r := game.NewResearch()
	for f, tiers := range want {
		if b := bestTier(r, f, nil); b != tiers[0] {
			t.Errorf("family %d, nothing known: %s, want %s", f, b, tiers[0])
		}
		r.Grant(game.Tech(tiers[1].Info().ResearchReq))
		if b := bestTier(r, f, nil); b != tiers[1] {
			t.Errorf("family %d, %s known: %s", f, tiers[1], b)
		}
	}
}

// AimItems holds every upper tier (an item of a family or housing that
// needs research) and nothing else.
func TestAimItems(t *testing.T) {
	var want []game.Item
	for it := game.Item(0); it < game.FirstOrbital; it++ {
		if (family(it) >= 0 || housing(it)) && it.Info().ResearchReq != game.NoResearch {
			want = append(want, it)
		}
	}
	got := slices.Clone(AimItems)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("AimItems %v, want the upper tiers %v", AimItems, want)
	}
}

// upgrade is the fixed policy's Upgrade threshold, rounded, at least 1.
func upgrade() int { return max(1, int(math.Round(NeutralEconomy().Upgrade))) }

// Base before upper tier: with the Industrial Megafacility known and
// industry the output wanted (IndShare 1, two Agriplots against the
// upkeep), the fixed pick and a genome's (Build 1 and 2, Upgrade 2)
// build Factories while fewer than Upgrade stand, the Megafacility from
// there on.
func TestUpgradeThreshold(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	e.Research.Grant(game.Tech(game.IndustrialMegafacility.Info().ResearchReq))
	col := e.Colonies[0]
	p, tr := col.Planet, col.Traits()
	put := func(it game.Item) {
		t.Helper()
		at, ok := p.BestCell(it, tr)
		if !ok || p.Build(at, it) != nil {
			t.Fatalf("fixture: no square for %s", it)
		}
	}
	put(game.Agriplot)
	put(game.Agriplot)
	ec := NeutralEconomy()
	ec.IndShare, ec.ResShare = 1, 0
	g := Neutral()
	g.Build = [game.NumItems]float64{}
	g.Build[game.Factory], g.Build[game.IndustrialMegafacility] = 1, 2
	g.Upgrade, g.IndShare, g.ResShare = 2, 1, 0
	for n := 0; n <= upgrade()+1; n++ {
		col.Pop = col.Busy() + 1
		if wanted(col, ec) != 0 {
			t.Fatalf("%d Factories: wanted %d, want industry", n, wanted(col, ec))
		}
		want := game.Factory
		if n >= upgrade() {
			want = game.IndustrialMegafacility
		}
		if it := GreedyPick(col, e.Research, ec); it != want {
			t.Errorf("fixed, %d Factories (Upgrade %d): %s, want %s", n, upgrade(), it, want)
		}
		want = game.Factory
		if n >= 2 {
			want = game.IndustrialMegafacility
		}
		if it, _, _ := GenomePick(col, e.Research, g); it != want {
			t.Errorf("genome, %d Factories (Upgrade 2): %s, want %s", n, it, want)
		}
		put(game.Factory)
	}
	// Upper tiers count toward the threshold: the Factories swapped for
	// Megafacilities keep it open.
	for _, c := range p.Cells() {
		if p.Surface[c].Item == game.Factory {
			p.Surface[c].Item = game.IndustrialMegafacility
		}
	}
	if !upgradeOpen(col, game.IndustrialMegafacility, float64(upgrade())) {
		t.Error("Megafacilities alone: the threshold shut")
	}
}

// The balance. Prosperity first: three Laboratories (research 3) and an
// industry share of 1 would want industry, yet a colony whose upkeep eats
// its prosperity wants prosperity and builds an Agriplot, below 0 and at
// 0 with room to grow. Fed by Agriplots it wants the output furthest
// under its share, so the shares steer the pick; at Capacity prosperity
// grows nothing and is never wanted, even at a prosperity share of 1.
func TestBalancePick(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	col := e.Colonies[0]
	p, tr := col.Planet, col.Traits()
	r := e.Research
	put := func(it game.Item) {
		t.Helper()
		at, ok := p.BestCell(it, tr)
		if !ok || p.Build(at, it) != nil {
			t.Fatalf("fixture: no square for %s", it)
		}
	}
	for range 3 {
		put(game.Laboratory)
	}
	ind := Economy{Upgrade: 3, IndShare: 1}
	res := Economy{Upgrade: 3, ResShare: 1}
	for _, want := range []int{-1, 0} {
		for col.Pop = col.Busy() + 1; ; col.Pop++ { // the upkeep pop/4 grows
			if _, _, pros := col.Output(); pros <= want {
				break
			}
		}
		if _, _, pros := col.Output(); pros != want || col.Pop >= col.Capacity() {
			t.Fatalf("fixture: prosperity %d, pop %d of %d", pros, col.Pop, col.Capacity())
		}
		for _, ec := range []Economy{ind, res, NeutralEconomy()} {
			if k := wanted(col, ec); k != 2 {
				t.Errorf("prosperity %d, %+v: wanted %d, want prosperity", want, ec, k)
			}
			if it := GreedyPick(col, r, ec); it != game.Agriplot {
				t.Errorf("prosperity %d, %+v: %s, want Agriplot", want, ec, it)
			}
		}
		col.Pop = col.Busy() + 1
	}
	for {
		col.Pop = col.Busy() + 1
		if _, _, pros := col.Output(); pros > 0 {
			break
		}
		put(game.Agriplot)
	}
	for _, tc := range []struct {
		ec   Economy
		k    int
		want game.Item
	}{{ind, 0, game.Factory}, {res, 1, game.Laboratory}} {
		if k := wanted(col, tc.ec); k != tc.k {
			t.Errorf("fed, %+v: wanted %d, want %d", tc.ec, k, tc.k)
		}
		if it := GreedyPick(col, r, tc.ec); it != tc.want {
			t.Errorf("fed, %+v: %s, want %s", tc.ec, it, tc.want)
		}
	}
	// The gap: industry 1 against research 3 at shares 0.5 / 0.5:
	// industry is further under its share.
	half := Economy{Upgrade: 3, IndShare: 0.5, ResShare: 0.5}
	if i, rs, _ := col.Output(); i >= rs {
		t.Fatalf("fixture: industry %d, research %d", i, rs)
	}
	if k := wanted(col, half); k != 0 {
		t.Errorf("shares 0.5/0.5, less industry than research: wanted %d, want industry", k)
	}
	// Capped, a prosperity share of 1: industry or research, the lower
	// (targets 0 and 0).
	for col.Pop < col.Capacity() {
		col.Pop++
	}
	if _, _, pros := col.Output(); pros < 0 {
		for {
			put(game.Agriplot)
			if _, _, pros := col.Output(); pros >= 0 {
				break
			}
		}
	}
	if k := wanted(col, Economy{Upgrade: 3}); k != 0 {
		t.Errorf("capped, prosperity share 1: wanted %d, want industry", k)
	}
}

// Clamp: Upgrade in [0, 20], each share in [0, 1], the two scaled down
// to a sum of 1 past it (prosperity's never below 0), IndFloor in
// [0, 30]; Neutral carries NeutralEconomy.
func TestEconomyGenesClamp(t *testing.T) {
	if g := Neutral(); g.Economy() != NeutralEconomy() {
		t.Errorf("Neutral: %+v, want %+v", g.Economy(), NeutralEconomy())
	}
	for _, c := range []struct{ in, want Economy }{
		{Economy{-1, -0.5, 2, -3}, Economy{0, 0, 1, 0}},
		{Economy{30, 0.6, 0.6, 40}, Economy{20, 0.5, 0.5, 30}},
		{Economy{4, 0.3, 0.2, 5}, Economy{4, 0.3, 0.2, 5}},
	} {
		g := Neutral()
		g.setEconomy(c.in)
		if g.Clamp(); g.Economy() != c.want {
			t.Errorf("Clamp(%+v) = %+v, want %+v", c.in, g.Economy(), c.want)
		}
	}
}

// At Capacity a known Habitat goes first, a worker short of it the
// scarcest output's structure.
func TestGreedyPickHousingWhenCapped(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	e.Research.Grant(game.Tech(game.Habitat.Info().ResearchReq))
	col := e.Colonies[0]
	col.Pop = col.Capacity() - 1
	if col.Free() < 1 {
		t.Fatalf("fixture: free %d", col.Free())
	}
	if it := GreedyPick(col, e.Research, NeutralEconomy()); it == game.Habitat {
		t.Errorf("below Capacity: Habitat")
	}
	col.Pop = col.Capacity()
	if it := GreedyPick(col, e.Research, NeutralEconomy()); it != game.Habitat {
		t.Errorf("at Capacity: %s, want Habitat", it)
	}
}

// occupy marks every empty square of col's planet as ruins: no square
// left for any structure, black ones included (Outpost).
func occupy(col *game.Colony) {
	for _, sq := range col.Planet.Surface {
		if sq.Item == game.NoItem {
			sq.Ruins = true
		}
	}
}

// The fixed policy with nothing to build: no square left, a Factory is
// replaced by the known Megafacility; with a square left and the
// population short of Capacity it waits; capped, it replaces; a family's
// structure is never replaced by another family's (Agriplot by a
// Megafacility).
func TestTierReplace(t *testing.T) {
	fixed := func(b *Bot) *Bot { b.Cfg.Genome = nil; return b }
	b, col, fs := replaceBot(t, upgrade(), 0)
	b = fixed(b)
	occupy(col)
	if it := GreedyPick(col, b.E.Research, NeutralEconomy()); it != game.NoItem {
		t.Fatalf("fixture: %s can start", it)
	}
	b.planColony(0, col, b.claimed())
	if pj := col.Project; pj == nil || pj.Item != game.IndustrialMegafacility || !slices.Contains(fs, pj.Cell) ||
		col.Planet.Surface[pj.Cell].Item != game.NoItem {
		t.Errorf("no square left: project %+v", pj)
	}

	// Room left, no free worker, Pop under Capacity: nothing.
	b, col, fs = replaceBot(t, upgrade(), 0)
	b = fixed(b)
	col.Pop = col.Busy()
	if col.Pop >= col.Capacity() || !roomFor(col, greedyItems(col, b.E.Research, NeutralEconomy())) {
		t.Fatalf("fixture: pop %d of %d", col.Pop, col.Capacity())
	}
	if b.planColony(0, col, b.claimed()); col.Project != nil {
		t.Errorf("growing, room left: project %+v", col.Project)
	}
	// Capped (empty squares blackened down to Capacity = Busy), every
	// worker employed, one empty square left: replaced.
	last := game.Cell{I: -1}
	for {
		at, ok := col.Planet.BestCell(game.Factory, col.Traits())
		if !ok {
			break
		}
		if err := col.Planet.Build(at, game.Factory); err != nil {
			t.Fatal(err)
		}
		last = at
	}
	if last.I < 0 {
		t.Fatal("fixture: no Factory built")
	}
	col.Planet.Demolish(last) // the room left
	for _, sq := range col.Planet.Surface {
		if sq.Item == game.NoItem && sq.Kind != game.Black && sq != col.Planet.Surface[last] {
			sq.Kind = game.Black
		}
	}
	col.Planet.PopBonus -= col.Capacity() - col.Busy() // capped
	col.Pop = col.Busy()
	if col.Pop != col.Capacity() || !roomFor(col, greedyItems(col, b.E.Research, NeutralEconomy())) {
		t.Fatalf("fixture: pop %d of %d", col.Pop, col.Capacity())
	}
	if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != game.IndustrialMegafacility {
		t.Errorf("capped: project %+v", col.Project)
	}

	// Full, Factories short of the Upgrade threshold: swapped all the
	// same (a small planet never reaches it).
	b, col, fs = replaceBot(t, upgrade()-1, 0)
	b = fixed(b)
	occupy(col)
	if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != game.IndustrialMegafacility || !slices.Contains(fs, col.Project.Cell) {
		t.Errorf("%d Factories, Upgrade %d: project %+v", upgrade()-1, upgrade(), col.Project)
	}

	// Agriplots alone, the Megafacility known: no swap across families.
	b, col, fs = replaceBot(t, upgrade(), 0)
	b = fixed(b)
	for _, c := range fs {
		col.Planet.Surface[c].Item = game.Agriplot
	}
	occupy(col)
	if b.planColony(0, col, b.claimed()); col.Project != nil {
		t.Errorf("Agriplots: project %+v", col.Project)
	}
}

// The ongoing project is the last resort: a full colony with two
// Factories that can upgrade one does, one that cannot parties.
func TestIdleAfterReplace(t *testing.T) {
	for _, tc := range []struct {
		mega bool
		want game.Item
	}{{true, game.IndustrialMegafacility}, {false, game.EndlessParty}} {
		b, col, _ := replaceBot(t, upgrade(), 0)
		b.Cfg = Config{Greedy: true, NoResearch: true}
		r := b.E.Research
		r.Grant(game.Tech(game.EndlessParty.Info().ResearchReq))
		if !tc.mega {
			r.Known[game.IndustrialMegafacility.Info().ResearchReq] = false
		}
		occupy(col)
		if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != tc.want {
			t.Errorf("Megafacility known %v: project %+v, want %s", tc.mega, col.Project, tc.want)
		}
	}
}

// After GuideResearch the fixed policy studies toward each AimItems tech
// in turn, the cheapest studiable of its prerequisites each time, and
// reaches every one; then Available()[0].
func TestAimTopic(t *testing.T) {
	r := game.NewResearch()
	q := slices.Clone(GuideResearch)
	study := func() game.Tech {
		t.Helper()
		tp := nextTopic(r, &q, false)
		if tp == game.NoTech {
			t.Fatal("no topic")
		}
		r.Grant(tp)
		return tp
	}
	for len(q) > 0 {
		study()
	}
	// closure is t and its unknown prerequisites, transitively.
	var closure func(t game.Tech, in map[game.Tech]bool)
	closure = func(t game.Tech, in map[game.Tech]bool) {
		if r.Known[t] || in[t] {
			return
		}
		in[t] = true
		for _, p := range game.Techs[t].Requires {
			closure(p, in)
		}
	}
	for _, it := range AimItems {
		aim := game.Tech(it.Info().ResearchReq)
		in := map[game.Tech]bool{}
		closure(aim, in)
		for !r.Known[aim] {
			cheapest := 0
			for c := range in {
				if r.CanStudy(c) && (cheapest == 0 || r.Cost(c) < cheapest) {
					cheapest = r.Cost(c)
				}
			}
			if tp := study(); !in[tp] || r.Cost(tp) != cheapest {
				t.Errorf("toward %s: studied %s (cost %d), not its cheapest studiable prerequisite (%d)", aim, tp, r.Cost(tp), cheapest)
			}
		}
	}
	if !r.Known[game.PlanetaryReplenishment] || !r.Known[game.LargeScaleConstruction] || !r.Known[game.PositronGuidance] {
		t.Error("an aim not reached")
	}
	want := r.Available()[0]
	if tp := nextTopic(r, &q, false); tp != want {
		t.Errorf("every aim known: %s, want Available()[0] %s", tp, want)
	}
}

// Terraforming known, a black square left: the bot whitens it first, an
// empty one before one under Tubes; unknown, it does not.
func TestTerraform(t *testing.T) {
	for _, tc := range []struct {
		known, tubes bool
	}{{true, false}, {true, true}, {false, false}} {
		b, col, _ := replaceBot(t, 1, 0)
		b.Cfg.Genome = nil
		if tc.known {
			b.E.Research.Grant(game.Tech(game.Terraforming.Info().ResearchReq))
		}
		blacken(col)
		p := col.Planet
		var tube game.Cell
		if tc.tubes {
			for _, c := range p.Cells() {
				if p.CanTerraform(c) {
					if err := p.Build(c, game.TransportTubes); err != nil {
						t.Fatal(err)
					}
					tube = c
					break
				}
			}
			occupy(col)
		}
		b.planColony(0, col, b.claimed())
		pj := col.Project
		switch {
		case !tc.known:
			if pj != nil && pj.Item == game.Terraforming {
				t.Errorf("unknown: project %+v", pj)
			}
		case pj == nil || pj.Item != game.Terraforming:
			t.Errorf("tubes %v: project %+v", tc.tubes, pj)
		case tc.tubes && pj.Cell != tube:
			t.Errorf("tubes: at %v, want %v", pj.Cell, tube)
		case !tc.tubes && p.Surface[pj.Cell].Item != game.NoItem:
			t.Errorf("empty: at %v under %s", pj.Cell, p.Surface[pj.Cell].Item)
		}
	}
}

// IndFloor: under it industry comes first, whatever the shares; at it
// the mix decides. A fresh colony (its Colony Base alone: industry 1,
// prosperity 1) wanted research at the neutral shares before the floor
// (the Colony Base's industry met its share) and industry now.
func TestIndFloor(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	col := e.Colonies[0]
	col.Pop = col.Busy() + 1
	ind, _, _ := col.Planet.Output() // the structures' sum, IndFloor's
	if _, _, pros := col.Output(); pros <= 0 {
		t.Fatal("fixture: no prosperity")
	}
	res := Economy{Upgrade: 3, ResShare: 1, IndFloor: float64(ind + 1)}
	if k := wanted(col, res); k != 0 {
		t.Errorf("industry %d under the floor %v: wanted %d, want industry", ind, res.IndFloor, k)
	}
	if res.IndFloor = float64(ind); wanted(col, res) != 1 {
		t.Errorf("industry %d at the floor: wanted %d, want research", ind, wanted(col, res))
	}

	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	home := g.Empires[0].Colonies[0]
	p := g.Cluster.Stars[home.Star].System.Planets[1]
	at, ok := p.BestCell(game.ColonyBase, home.Traits())
	if !ok {
		t.Fatal("fixture: no square for a Colony Base")
	}
	p.Surface[at].Item, p.Owner = game.ColonyBase, 0
	fresh := game.NewColony(p)
	fresh.Star, fresh.PlanetIdx = home.Star, 1
	before := NeutralEconomy()
	before.IndFloor = 0
	if k := wanted(fresh, before); k != 1 {
		t.Fatalf("fixture: a fresh colony without the floor wanted %d, want research", k)
	}
	fi, _, _ := fresh.Planet.Output()
	if after := NeutralEconomy(); float64(fi) >= after.IndFloor {
		t.Errorf("a fresh colony's industry %d at the neutral floor %v", fi, after.IndFloor)
	} else if k := wanted(fresh, after); k != 0 {
		t.Errorf("a fresh colony: wanted %d, want industry", k)
	}
}

// The stall: the last free worker taken while the next unit's upkeep
// would bring prosperity to nil leaves no one to build the Agriplot, the
// colony frozen. wanted keeps that worker for prosperity, under IndFloor
// too.
func TestWantedNoStall(t *testing.T) {
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	col := e.Colonies[0]
	p, tr := col.Planet, col.Traits()
	for {
		col.Pop = col.Busy() + 1
		next := (col.Pop+1)/4 - col.Pop/4
		_, _, pros := col.Output()
		if col.Pop >= col.Capacity() || pros < 0 {
			t.Fatalf("fixture: pop %d of %d, prosperity %d", col.Pop, col.Capacity(), pros)
		}
		if next == 1 && pros == 1 {
			break
		}
		it := game.Factory
		if pros < 1 {
			it = game.Agriplot
		}
		at, ok := p.BestCell(it, tr)
		if !ok || p.Build(at, it) != nil {
			t.Fatalf("fixture: no square for %s", it)
		}
	}
	ec := NeutralEconomy()
	ec.IndFloor = 100
	if k := wanted(col, ec); k != 2 {
		t.Errorf("last worker, prosperity %d to be eaten: wanted %d, want prosperity", 1, k)
	}
}
