package bot

import (
	"cmp"
	"math"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The colonies' defences at war (fortify.go).

// fortifyFixture: seed 1, two empires, empire 0 (bot b, genome gn,
// research r) at War with empire 1 unless peace; col its home, idle,
// with free people and Pop at least ShieldPop; b.cg assessed.
func fortifyFixture(t *testing.T, r *game.Research, gn *Genome, peace bool) (*Bot, *game.Colony) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	if peace {
		g.Contact(0, 1)
	} else {
		atWar(t, g, 0, 1)
	}
	b := NewAll(g, Config{NoResearch: true, Genome: gn})[0]
	b.E.Research = r
	col := b.E.Colonies[0]
	col.Project, col.Queue = nil, nil
	b.assess()
	fill(col, b.cg)
	return b, col
}

// fill gives col a free worker and Pop at least ShieldPop.
func fill(col *game.Colony, cg Combat) {
	col.Pop = max(col.Busy()+1, int(math.Round(cg.ShieldPop)))
}

// knowing is a Research knowing only the techs items need.
func knowing(items ...game.Item) *game.Research {
	r := game.NewResearch()
	for _, it := range items {
		if req := it.Info().ResearchReq; req >= 0 && req < int(game.NumTechs) {
			r.Known[req] = true
		}
	}
	return r
}

// complete places col's project as the close would and clears it.
func complete(t *testing.T, col *game.Colony) {
	t.Helper()
	pj, p := col.Project, col.Planet
	var err error
	if pj.Orbital {
		err = p.BuildOrbital(pj.Slot, pj.Item)
	} else {
		err = p.BuildWith(pj.Cell, pj.Item, col.Traits())
	}
	if err != nil {
		t.Fatal(err)
	}
	col.Project = nil
}

// started runs fortify until it starts nothing, completing each project,
// and returns the items in order.
func started(t *testing.T, b *Bot, col *game.Colony) []game.Item {
	t.Helper()
	var out []game.Item
	for range 20 {
		fill(col, b.cg)
		if !b.fortify(col) {
			if col.Project != nil {
				t.Fatalf("fortify false but started %s", col.Project.Item)
			}
			return out
		}
		if col.Project == nil {
			t.Fatalf("fortify true, nothing started (free %d)", col.Free())
		}
		out = append(out, col.Project.Item)
		complete(t, col)
	}
	t.Fatalf("no end: %v", out)
	return nil
}

// gunOracle is the orbital weapon of r of most Damage × Shots, then
// Range, read off game.OrbitalDefences by a sort.
func gunOracle(r *game.Research) game.Item {
	var guns []game.Item
	for it := range game.Item(game.NumItems) {
		if d := game.OrbitalDefences[it]; d.Damage > 0 && r.Allows(it) {
			guns = append(guns, it)
		}
	}
	if len(guns) == 0 {
		return game.NoItem
	}
	slices.SortStableFunc(guns, func(a, b game.Item) int {
		da, db := game.OrbitalDefences[a], game.OrbitalDefences[b]
		return cmp.Or(cmp.Compare(db.Damage*db.Shots, da.Damage*da.Shots), cmp.Compare(db.Range, da.Range))
	})
	return guns[0]
}

// times is it repeated n times.
func times(it game.Item, n int) []game.Item { return slices.Repeat([]game.Item{it}, n) }

// At war, the order is surface shields to ShieldWorth, orbital shields to
// OrbitalShields, guns to OrbitalGuns, Orbital then Surface Cloakers to
// their genes, then nothing: the Megas and the best gun when known, else
// the plain ones, a cloaker only when known; so many as the genes and
// game.ShieldWorth make it.
func TestFortifyOrder(t *testing.T) {
	plain := []game.Item{game.SurfaceShield, game.OrbitalShield, game.OrbitalMissileBase}
	for _, tc := range []struct {
		name  string
		r     *game.Research
		genes [5]float64 // ShieldWorth, OrbitalShields, OrbitalGuns, OrbitalCloakers, SurfaceCloakers
	}{
		{"all known, neutral", allKnown(), [5]float64{4, 1, 1, 1, 1}},
		{"all known, more", allKnown(), [5]float64{10, 3, 2, 1, 1}},
		{"plain, neutral", knowing(plain...), [5]float64{4, 1, 1, 1, 1}},
		{"plain, more", knowing(plain...), [5]float64{7, 2, 3, 1, 1}},
		{"plain and the surface cloaker", knowing(append(plain, game.SurfaceCloaker)...), [5]float64{4, 1, 1, 1, 1}},
	} {
		b, col := fortifyFixture(t, tc.r, defenceGenes(tc.genes), false)
		sh, osh := game.SurfaceShield, game.OrbitalShield
		if tc.r.Allows(game.SurfaceMegaShield) {
			sh = game.SurfaceMegaShield
		}
		if tc.r.Allows(game.OrbitalMegaShield) {
			osh = game.OrbitalMegaShield
		}
		w := game.ShieldWorth[sh]
		want := times(sh, (int(tc.genes[0])+w-1)/w)
		want = append(want, times(osh, int(tc.genes[1]))...)
		want = append(want, times(gunOracle(tc.r), int(tc.genes[2]))...)
		for k, it := range []game.Item{game.OrbitalCloaker, game.SurfaceCloaker} {
			if tc.r.Allows(it) {
				want = append(want, times(it, int(tc.genes[3+k]))...)
			}
		}
		if got := started(t, b, col); !slices.Equal(got, want) {
			t.Errorf("%s: started %v, want %v", tc.name, got, want)
		}
	}
}

// defenceGenes is Neutral with the five defence targets of g (fortify's
// order).
func defenceGenes(g [5]float64) *Genome {
	return genomeWith(func(n *Genome) {
		n.ShieldWorth, n.OrbitalShields, n.OrbitalGuns, n.OrbitalCloakers, n.SurfaceCloakers = g[0], g[1], g[2], g[3], g[4]
	})
}

// Each target at 0 skips its step; all five at 0 start nothing.
func TestFortifyZero(t *testing.T) {
	gun := gunOracle(allKnown())
	ms, mo, oc, sc := game.SurfaceMegaShield, game.OrbitalMegaShield, game.OrbitalCloaker, game.SurfaceCloaker
	for _, tc := range []struct {
		genes [5]float64
		want  []game.Item
	}{
		{[5]float64{0, 1, 1, 1, 1}, []game.Item{mo, gun, oc, sc}},
		{[5]float64{4, 0, 1, 1, 1}, []game.Item{ms, gun, oc, sc}},
		{[5]float64{4, 1, 0, 1, 1}, []game.Item{ms, mo, oc, sc}},
		{[5]float64{4, 1, 1, 0, 1}, []game.Item{ms, mo, gun, sc}},
		{[5]float64{4, 1, 1, 1, 0}, []game.Item{ms, mo, gun, oc}},
		{[5]float64{0, 0, 0, 0, 0}, nil},
	} {
		b, col := fortifyFixture(t, allKnown(), defenceGenes(tc.genes), false)
		if got := started(t, b, col); !slices.Equal(got, tc.want) {
			t.Errorf("genes %v: started %v, want %v", tc.genes, got, tc.want)
		}
	}
}

// At peace nothing; under ShieldPop nothing; at ShieldPop the shield.
// A nil genome plays NeutralCombat's ShieldPop.
func TestFortifyGates(t *testing.T) {
	b, col := fortifyFixture(t, allKnown(), nil, true)
	if b.fortify(col) || col.Project != nil {
		t.Errorf("at peace: started %+v", col.Project)
	}
	for _, gn := range []*Genome{nil, genomeWith(func(g *Genome) { g.ShieldPop = 9 })} {
		b, col = fortifyFixture(t, allKnown(), gn, false)
		pop := int(math.Round(b.cg.ShieldPop))
		if col.Busy()+1 > pop-1 {
			t.Fatalf("fixture: busy %d, ShieldPop %d", col.Busy(), pop)
		}
		col.Pop = pop - 1
		if b.fortify(col) || col.Project != nil {
			t.Errorf("Pop %d < ShieldPop %d: started %+v", col.Pop, pop, col.Project)
		}
		col.Pop = pop
		if !b.fortify(col) || col.Project == nil || col.Project.Item != game.SurfaceMegaShield {
			t.Errorf("Pop %d = ShieldPop: project %+v", col.Pop, col.Project)
		}
	}
}

// A shield queued (Colony.Queue) counts toward the target: the orbital
// shield comes next. Through Act, the running shield is never doubled:
// the next Act keeps the same project.
func TestFortifyCountsPlanned(t *testing.T) {
	b, col := fortifyFixture(t, allKnown(), nil, false)
	at, ok := col.Planet.BestCell(game.SurfaceMegaShield, col.Traits())
	if !ok {
		t.Fatal("no square for the shield")
	}
	col.Queue = []game.Queued{{Item: game.SurfaceMegaShield, Cell: at}}
	if !b.fortify(col) || col.Project == nil || col.Project.Item != game.OrbitalMegaShield {
		t.Errorf("shield queued: project %+v, want %s", col.Project, game.OrbitalMegaShield)
	}

	b, col = fortifyFixture(t, allKnown(), nil, false)
	b.Cfg.Greedy = true
	if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != game.SurfaceMegaShield {
		t.Fatalf("planColony: project %+v", col.Project)
	}
	pj := col.Project
	b.Act()
	if col.Project != pj {
		t.Errorf("Act replaced the running shield by %+v", col.Project)
	}
}

// Short of free people the step waits (true, nothing started) while the
// colony can grow; at Capacity it falls through (false). A full orbit
// never waits: the orbital steps fall through (no surface step after).
func TestFortifyWaits(t *testing.T) {
	gn := genomeWith(func(g *Genome) { g.ShieldPop = 2 })
	b, col := fortifyFixture(t, allKnown(), gn, false)
	staff(t, col, 2)
	if col.Pop = col.Busy(); col.Pop >= col.Capacity() {
		t.Fatalf("fixture: pop %d, capacity %d", col.Pop, col.Capacity())
	}
	if !b.fortify(col) || col.Project != nil {
		t.Errorf("no free people, room to grow: waits false or started %+v", col.Project)
	}
	col.Planet.PopBonus -= col.Capacity() - col.Busy() // capped: Capacity down to the people employed
	col.Pop = col.Busy()
	if _, ok := col.Planet.BestCell(game.SurfaceMegaShield, col.Traits()); !ok || col.Pop < col.Capacity() {
		t.Fatalf("fixture: pop %d, capacity %d, square for the shield %v", col.Pop, col.Capacity(), ok)
	}
	if b.fortify(col) || col.Project != nil {
		t.Errorf("no free people at Capacity: waits or started %+v", col.Project)
	}

	b, col = fortifyFixture(t, allKnown(), genomeWith(func(g *Genome) { g.ShieldPop, g.ShieldWorth, g.SurfaceCloakers = 2, 0, 0 }), false)
	staff(t, col, 2)
	for slot := range col.Planet.Orbit {
		if col.Planet.Orbit[slot] == game.NoItem {
			col.Planet.Orbit[slot] = game.OrbitalDocks
		}
	}
	col.Pop = col.Busy()
	if b.fortify(col) || col.Project != nil {
		t.Errorf("full orbit, no free people: waits or started %+v", col.Project)
	}
}

// staff builds Factories on col until its structures employ n people.
func staff(t *testing.T, col *game.Colony, n int) {
	t.Helper()
	p, tr := col.Planet, col.Traits()
	for _, c := range p.Cells() {
		if col.Busy() >= n {
			return
		}
		if p.CanBuild(c, game.Factory, tr) {
			if err := p.BuildWith(c, game.Factory, tr); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Fatalf("staff %d: busy %d, no square left", n, col.Busy())
}

// bestGun is gunOracle for every subset of the orbital weapons known.
func TestBestGun(t *testing.T) {
	var guns []game.Item
	for it := range game.Item(game.NumItems) {
		if game.OrbitalDefences[it].Damage > 0 {
			guns = append(guns, it)
		}
	}
	if len(guns) < 2 {
		t.Fatalf("weapons %v", guns)
	}
	for mask := range 1 << len(guns) {
		var known []game.Item
		for k, it := range guns {
			if mask>>k&1 == 1 {
				known = append(known, it)
			}
		}
		r := knowing(known...)
		if got, want := bestGun(r), gunOracle(r); got != want {
			t.Errorf("known %v: %s, want %s", known, got, want)
		}
	}
}

// The genes: NeutralCombat's in Neutral, clamped to their bounds.
func TestFortifyGenes(t *testing.T) {
	n, c := Neutral(), NeutralCombat()
	if n.ShieldWorth != c.ShieldWorth || n.ShieldPop != c.ShieldPop || n.OrbitalShields != c.OrbitalShields || n.OrbitalGuns != c.OrbitalGuns ||
		n.OrbitalCloakers != c.OrbitalCloakers || n.SurfaceCloakers != c.SurfaceCloakers {
		t.Errorf("Neutral %v %v %v %v %v %v, NeutralCombat %+v", n.ShieldWorth, n.ShieldPop, n.OrbitalShields, n.OrbitalGuns, n.OrbitalCloakers, n.SurfaceCloakers, c)
	}
	if n.Combat() != c {
		t.Errorf("Neutral().Combat() differs from NeutralCombat")
	}
	g := Neutral()
	g.ShieldWorth, g.ShieldPop, g.OrbitalShields, g.OrbitalGuns = 11, 1, -1, math.NaN()
	g.OrbitalCloakers, g.SurfaceCloakers = 2, -1
	g.Clamp()
	if g.ShieldWorth != 10 || g.ShieldPop != 2 || g.OrbitalShields != 0 || g.OrbitalGuns != 0 || g.OrbitalCloakers != 1 || g.SurfaceCloakers != 0 {
		t.Errorf("clamped %v %v %v %v %v %v", g.ShieldWorth, g.ShieldPop, g.OrbitalShields, g.OrbitalGuns, g.OrbitalCloakers, g.SurfaceCloakers)
	}
}
