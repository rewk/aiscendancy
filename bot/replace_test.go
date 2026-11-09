package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// replaceBot is a fresh greedy home with the Industrial Megafacility's
// tech, a Factory on each of n white non-bonus squares (Cells order) and
// one free worker; its genome builds Factories and Megafacilities only,
// replaces at ReplaceDays days, at Upgrade 0 (no threshold:
// TestUpgradeThreshold's).
func replaceBot(t *testing.T, n int, days float64) (*Bot, *game.Colony, []game.Cell) {
	t.Helper()
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	e.Research.Grant(game.Tech(game.IndustrialMegafacility.Info().ResearchReq))
	col := e.Colonies[0]
	p, tr := col.Planet, col.Traits()
	var fs []game.Cell
	for _, c := range p.Cells() {
		if len(fs) < n && p.Surface[c].Kind == game.White && p.CanBuild(c, game.Factory, tr) {
			if err := p.Build(c, game.Factory); err != nil {
				t.Fatal(err)
			}
			fs = append(fs, c)
		}
	}
	if len(fs) != n {
		t.Fatalf("%d white squares, want %d", len(fs), n)
	}
	col.Pop = col.Busy() + 1
	g := Neutral()
	g.Build = [game.NumItems]float64{}
	g.Build[game.Factory], g.Build[game.IndustrialMegafacility] = 1, 1
	g.ReplaceDays, g.Upgrade = days, 0
	return New(e, Config{Greedy: true, Genome: g}), col, fs
}

// blacken turns every empty square of col's planet black: no cell left.
func blacken(col *game.Colony) {
	for _, sq := range col.Planet.Surface {
		if sq.Item == game.NoItem {
			sq.Kind = game.Black
		}
	}
}

// The Factory → Megafacility trade, derived from the model: gain and
// loss as replacePick's doc states; ReplaceDays at the break-even
// replaces, a day less does not, 0 never.
func TestReplaceFactory(t *testing.T) {
	_, col, fs := replaceBot(t, 1, 0)
	f := fs[0]
	bi, br, bp := col.Output()
	ai, ar, ap := col.OutputWith(f, game.IndustrialMegafacility)
	ei, er, ep := col.OutputWith(f, game.NoItem)
	gain := float64(ai + ar + ap - bi - br - bp)
	days := (game.IndustrialMegafacility.Info().Cost + ei - 1) / ei
	loss := float64(days * (bi + br + bp - ei - er - ep))
	if gain <= 0 || loss <= 0 {
		t.Fatalf("fixture: gain %v loss %v", gain, loss)
	}
	breakEven := loss / gain
	for _, tc := range []struct {
		days float64
		want bool
	}{{0, false}, {breakEven - 1, false}, {breakEven, true}, {400, true}} {
		b, col, _ := replaceBot(t, 1, tc.days)
		b.planColony(0, col, b.claimed())
		pj := col.Project
		got := pj != nil && pj.Item == game.IndustrialMegafacility && pj.Cell == f && col.Planet.Surface[f].Item == game.NoItem
		if got != tc.want {
			t.Errorf("ReplaceDays %v (break-even %v): project %+v, square %s, want replaced %v",
				tc.days, breakEven, pj, col.Planet.Surface[f].Item, tc.want)
		}
		if !tc.want && col.Planet.Surface[f].Item != game.Factory {
			t.Errorf("ReplaceDays %v: the Factory is gone", tc.days)
		}
	}
}

// ReplaceFull ≥ 0.5 waits for the planet to be full: with a buildable
// cell the greedy pick runs instead, with none the Factory is replaced.
func TestReplaceFull(t *testing.T) {
	b, col, fs := replaceBot(t, 1, 400)
	b.Cfg.Genome.ReplaceFull = 1
	if full(col, b.E.Research, b.Cfg.Genome) {
		t.Fatal("fixture full")
	}
	b.planColony(0, col, b.claimed())
	if pj := col.Project; pj == nil || pj.Cell == fs[0] || col.Planet.Surface[fs[0]].Item != game.Factory {
		t.Errorf("room left: project %+v, square %s", pj, col.Planet.Surface[fs[0]].Item)
	}
	b, col, fs = replaceBot(t, 1, 400)
	b.Cfg.Genome.ReplaceFull = 1
	blacken(col)
	if !full(col, b.E.Research, b.Cfg.Genome) {
		t.Fatal("blackened planet not full")
	}
	b.planColony(0, col, b.claimed())
	if pj := col.Project; pj == nil || pj.Item != game.IndustrialMegafacility || pj.Cell != fs[0] {
		t.Errorf("full: project %+v", pj)
	}
	// Below 0.5 the room left does not matter.
	b, col, fs = replaceBot(t, 1, 400)
	b.Cfg.Genome.ReplaceFull = 0.49
	b.planColony(0, col, b.claimed())
	if pj := col.Project; pj == nil || pj.Item != game.IndustrialMegafacility || pj.Cell != fs[0] {
		t.Errorf("ReplaceFull 0.49: project %+v", pj)
	}
}

// A Habitat (MaxPop), a Surface Shield (no output) and the Colony Base
// are never replaced; a Habitat never replaces, whatever its Build and
// its prosperity's worth.
func TestReplaceNeverHousingOrDefence(t *testing.T) {
	b, col, fs := replaceBot(t, 1, 400)
	p, r, g := col.Planet, b.E.Research, b.Cfg.Genome
	r.Grant(game.Tech(game.Habitat.Info().ResearchReq))
	at, ok := p.BestCell(game.Factory, col.Traits())
	if !ok || p.Build(at, game.Habitat) != nil {
		t.Fatal("no square for a Habitat")
	}
	if at, ok = p.BestCell(game.Factory, col.Traits()); !ok || p.Build(at, game.SurfaceShield) != nil {
		t.Fatal("no square for a Surface Shield")
	}
	var labs []game.Cell
	for range 3 { // research off the bottom: prosperity is scarcest
		if at, ok = p.BestCell(game.Laboratory, col.Traits()); !ok || p.Build(at, game.Laboratory) != nil {
			t.Fatal("no square for a Laboratory")
		}
		labs = append(labs, at)
	}
	col.Pop = col.Busy() + 1
	// A Habitat on the Factory's square would pay (the prosperity
	// wanted), but it holds people: never a candidate.
	g.Build = [game.NumItems]float64{}
	g.Build[game.Habitat] = 10
	g.Balance = 10
	g.IndShare, g.ResShare, g.IndFloor = 0, 0, 0 // prosperity's share 1: wanted
	ai, ar, ap := col.OutputWith(fs[0], game.Habitat)
	bi, br, bp := col.Output()
	if ap-bp <= 0 || wanted(col, g.Economy()) != 2 {
		t.Fatalf("fixture: Habitat gain %d %d %d, wanted %d (out %d %d %d pop %d)", ai-bi, ar-br, ap-bp, wanted(col, g.Economy()), bi, br, bp, col.Pop)
	}
	if at, it, ok := replacePick(col, r, g); ok {
		t.Errorf("replaced %s at %v by %s", p.Surface[at].Item, at, it)
	}
	// Nothing but the Habitat, the Shield and the base: nothing to replace
	// by a Megafacility.
	for _, c := range append(labs, fs[0]) {
		p.Demolish(c)
	}
	g.Build[game.IndustrialMegafacility] = 10
	if at, it, ok := replacePick(col, r, g); ok {
		t.Errorf("replaced %s at %v by %s", p.Surface[at].Item, at, it)
	}
}

// Of two Factories alike, the first in Cells order is replaced, every
// time.
func TestReplacePickDeterministic(t *testing.T) {
	for range 5 {
		b, col, fs := replaceBot(t, 2, 400)
		at, it, ok := replacePick(col, b.E.Research, b.Cfg.Genome)
		if !ok || at != fs[0] || it != game.IndustrialMegafacility {
			t.Fatalf("pick %v %s %v, want %v", at, it, ok, fs[0])
		}
	}
}

// The genes: 0 for Neutral, clamped to [0, 400] and [0, 1].
func TestReplaceGenes(t *testing.T) {
	if g := Neutral(); g.ReplaceDays != 0 || g.ReplaceFull != 0 {
		t.Errorf("neutral %v %v", g.ReplaceDays, g.ReplaceFull)
	}
	g := Neutral()
	g.ReplaceDays, g.ReplaceFull = 1e6, 2
	if g.Clamp(); g.ReplaceDays != 400 || g.ReplaceFull != 1 {
		t.Errorf("clamped high %v %v", g.ReplaceDays, g.ReplaceFull)
	}
	g.ReplaceDays, g.ReplaceFull = -1, -1
	if g.Clamp(); g.ReplaceDays != 0 || g.ReplaceFull != 0 {
		t.Errorf("clamped low %v %v", g.ReplaceDays, g.ReplaceFull)
	}
}

// No churn: after a replacement, the colony as it stands never picks a
// trade back (the gain is Σ output, which only grows).
func TestReplaceNoChurn(t *testing.T) {
	b, col, fs := replaceBot(t, 1, 400)
	g := b.Cfg.Genome
	g.Build[game.Laboratory], g.Balance = 1, 5
	at, it, ok := replacePick(col, b.E.Research, g)
	if !ok || at != fs[0] {
		t.Fatalf("fixture: pick %v %s %v", at, it, ok)
	}
	old := col.Planet.Surface[at].Item
	if err := col.Replace(b.E.Research, at, it); err != nil {
		t.Fatal(err)
	}
	for col.Project != nil {
		b.E.EndDay()
	}
	if at2, it2, ok := replacePick(col, b.E.Research, g); ok && at2 == at && it2 == old {
		t.Errorf("%s at %v traded back to %s", it, at, old)
	}
}

// ReplaceDays 0 never replaces, even a trade losing nothing: with three
// Factories (industry sum 4 → 3 rounds to 3 either way) a Megafacility
// is free of loss.
func TestReplaceDaysZero(t *testing.T) {
	b, col, fs := replaceBot(t, 3, 0)
	ei, er, ep := col.OutputWith(fs[0], game.NoItem)
	if bi, br, bp := col.Output(); bi+br+bp != ei+er+ep {
		t.Fatalf("fixture: the Factory's loss %d", bi+br+bp-ei-er-ep)
	}
	if at, it, ok := replacePick(col, b.E.Research, b.Cfg.Genome); !ok || it != game.IndustrialMegafacility {
		t.Fatalf("fixture: pick %v %s %v", at, it, ok)
	}
	b.planColony(0, col, b.claimed())
	for _, f := range fs {
		if col.Planet.Surface[f].Item != game.Factory {
			t.Errorf("%v: %s", f, col.Planet.Surface[f].Item)
		}
	}
}

// The Upgrade gene holds a replacement back too: two Factories, the
// Megafacility past break-even, replaced at Upgrade 2, not at 3.
func TestReplaceUpgradeGene(t *testing.T) {
	for _, tc := range []struct {
		upgrade float64
		want    bool
	}{{2, true}, {3, false}} {
		b, col, _ := replaceBot(t, 2, 400)
		b.Cfg.Genome.Upgrade = tc.upgrade
		_, it, ok := replacePick(col, b.E.Research, b.Cfg.Genome)
		if got := ok && it == game.IndustrialMegafacility; got != tc.want {
			t.Errorf("Upgrade %v: replaced %v (%s), want %v", tc.upgrade, got, it, tc.want)
		}
	}
}
