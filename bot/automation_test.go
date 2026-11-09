package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// automationBot is a fresh greedy home with Automation researched, full
// with one free worker: ten Factories built, every empty square but three
// reachable ones blackened (the capacity comes down under the people),
// Pop = Busy+1.
func automationBot(t *testing.T) (*Bot, *game.Colony) {
	t.Helper()
	e := game.NewEmpire(1, game.Snovemdomas, game.Average)
	e.Research.Grant(game.Tech(game.Automation.Info().ResearchReq))
	col := e.Colonies[0]
	p, tr := col.Planet, col.Traits()
	for range 10 {
		at, ok := p.BestCell(game.Factory, tr)
		if !ok {
			t.Fatal("no Factory square left")
		}
		if err := p.Build(at, game.Factory); err != nil {
			t.Fatal(err)
		}
	}
	kept := 0
	for _, c := range p.Cells() {
		switch sq := p.Surface[c]; {
		case sq.Item != game.NoItem:
		case kept < 3 && p.CanBuild(c, game.Factory, tr):
			kept++
		default:
			sq.Kind = game.Black
		}
	}
	col.Pop = col.Busy() + 1
	if kept != 3 || col.Pop < col.Capacity() {
		t.Fatalf("fixture: %d squares kept, pop %d of %d", kept, col.Pop, col.Capacity())
	}
	return New(e, Config{Greedy: true}), col
}

// squares counts col's empty squares a Factory would take.
func squares(col *game.Colony) int {
	n := 0
	for _, c := range col.Planet.Cells() {
		if col.Planet.CanBuild(c, game.Factory, col.Traits()) {
			n++
		}
	}
	return n
}

// A full colony down to its last free worker with squares left automates
// its first structure in Cells order not automated yet (the nil
// genome's AutomateCostW 0; automateAt ranks by costW × Cost otherwise);
// with workers to spare, room to grow, one square left, or not greedy,
// it does not.
func TestAutomateShortOfWorkers(t *testing.T) {
	b, col := automationBot(t)
	if n := squares(col); n < 2 || col.Free() != 1 || col.Pop < col.Capacity() {
		t.Fatalf("fixture: %d squares, free %d, pop %d of %d", n, col.Free(), col.Pop, col.Capacity())
	}
	p := col.Planet
	// The structures by costW × Cost, ties in Cells order: ranked(0) is
	// Cells order, ranked(1) the cheapest first, ranked(-1) the dearest.
	ranked := func(costW float64) game.Cell {
		var best game.Cell
		bestScore, found := 0.0, false
		for _, c := range p.Cells() {
			sq := p.Surface[c]
			if sq.Item == game.NoItem || sq.Automated {
				continue
			}
			if score := costW * float64(sq.Item.Info().Cost); !found || score < bestScore {
				best, bestScore, found = c, score, true
			}
		}
		if !found {
			t.Fatal("no structure left")
		}
		return best
	}
	first := ranked(0)
	if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != game.Automation || col.Project.Cell != first {
		t.Fatalf("project %+v, want the Automation of %v", col.Project, first)
	}
	col.Cancel()
	// A genome's AutomateCostW ranks by cost: the Colony Base (120) is
	// last at 1 and first at -1, a Factory (30) first at 1.
	base := game.Cell{I: game.Centre, J: game.Centre}
	if cheap, dear := ranked(1), ranked(-1); p.Surface[cheap].Item != game.Factory || dear != base {
		t.Fatalf("ranked: cheapest %s at %v, dearest %v (base %v)", p.Surface[cheap].Item, cheap, dear, base)
	}
	for _, w := range []float64{1, -1} {
		if at, ok := automateAt(col, b.E.Research, w); !ok || at != ranked(w) {
			t.Errorf("costW %v: %v %v, want %v", w, at, ok, ranked(w))
		}
	}
	// The first automated: the next one is the second in Cells order.
	p.Surface[first].Automated = true
	second := ranked(0)
	col.Pop = col.Busy() + 1
	if b.planColony(0, col, b.claimed()); col.Project == nil || col.Project.Item != game.Automation || col.Project.Cell != second {
		t.Fatalf("project %+v, want the Automation of %v", col.Project, second)
	}
	notAutomation := func(name string) {
		t.Helper()
		pj := col.Project
		if b.planColony(0, col, b.claimed()); col.Project != nil && col.Project.Item == game.Automation && col.Project != pj {
			t.Errorf("%s: automated %v", name, col.Project.Cell)
		}
		col.Cancel()
	}
	col.Cancel()
	col.Pop++ // two free, short of the three squares still
	notAutomation("two free")
	col.Pop--
	cfg := b.Cfg
	b.Cfg.Greedy = false
	notAutomation("not greedy")
	b.Cfg = cfg
	// Room to grow: the planet bombed Lush.
	p.Lush = true
	if col.Pop >= col.Capacity() {
		t.Fatal("lush: still full")
	}
	notAutomation("room to grow")
	p.Lush = false
	// One square left: the others blackened (capacity falls, Pop stays).
	left := 0
	for _, c := range p.Cells() {
		if p.CanBuild(c, game.Factory, col.Traits()) {
			if left++; left > 1 {
				p.Surface[c].Kind = game.Black
			}
		}
	}
	if n := squares(col); n != 1 || col.Pop < col.Capacity() || col.Free() != 1 {
		t.Fatalf("one square fixture: %d squares, pop %d of %d, free %d", n, col.Pop, col.Capacity(), col.Free())
	}
	notAutomation("one square")
}

// Transport Tubes employ nobody (Item.Staff, as an Outpost): automating
// them hands back no worker, so automateAt never picks them, though they
// are the cheapest (10) at AutomateCostW 1.
func TestAutomateSkipsUnstaffed(t *testing.T) {
	b, col := automationBot(t)
	p := col.Planet
	placed := false
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; !placed && sq.Kind == game.Black && sq.Item == game.NoItem {
			sq.Item, placed = game.TransportTubes, true
		}
	}
	if !placed || col.Free() != 1 || col.Pop < col.Capacity() {
		t.Fatalf("fixture: tubes %v, free %d, pop %d of %d", placed, col.Free(), col.Pop, col.Capacity())
	}
	for _, w := range []float64{0, 1, -1} {
		at, ok := automateAt(col, b.E.Research, w)
		if !ok {
			t.Fatalf("costW %v: nothing to automate", w)
		}
		if it := p.Surface[at].Item; it.Staff() == 0 {
			t.Errorf("costW %v: automates the %s at %v, which employs nobody", w, it, at)
		}
	}
}
