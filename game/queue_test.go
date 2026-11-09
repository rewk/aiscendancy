package game

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// queueWorld is baseWorld's colony with the people to spare (Pop 6),
// Terraforming, Automation and the orbital structures researched, and
// square (6,5), east of the base, black.
func queueWorld(t *testing.T) (*Colony, *Research) {
	t.Helper()
	p := baseWorld()
	p.Surface[Cell{6, 5}].Kind = Black
	c := NewColony(p)
	c.Pop = 6
	r := NewResearch()
	for _, it := range []Item{Terraforming, Automation} {
		r.Grant(Tech(it.Info().ResearchReq))
	}
	r.Grant(OrbitalStructures)
	return c, r
}

// An entry is judged on the planet as planned: (4,4) and (7,5) are
// reachable only through the running Factory and a queued Agriplot; the
// real planet is untouched, and CanStart still refuses them.
func TestEnqueueReachesThroughThePlan(t *testing.T) {
	c, r := queueWorld(t)
	p := c.Planet
	p.Surface[Cell{6, 5}].Kind = White
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	if err := c.CanEnqueue(r, Agriplot, Cell{7, 5}); err == nil || !strings.Contains(err.Error(), "reachable") {
		t.Errorf("(7,5) before (6,5) is queued: %v", err)
	}
	for _, at := range []Cell{{4, 4}, {6, 5}, {7, 5}} {
		if err := c.Enqueue(r, Agriplot, at); err != nil {
			t.Errorf("Agriplot at %v: %v", at, err)
		}
	}
	for _, at := range []Cell{{4, 4}, {7, 5}} {
		if p.Surface[at].Item != NoItem {
			t.Errorf("%v holds %s: the plan touched the planet", at, p.Surface[at].Item)
		}
		if p.Reachable(at) {
			t.Errorf("%v reachable on the real planet", at)
		}
	}
	if pl := c.Planned(); pl.Surface[Cell{7, 5}].Item != Agriplot || pl.Surface[Cell{4, 5}].Item != Factory {
		t.Errorf("planned (7,5) %s, (4,5) %s", pl.Surface[Cell{7, 5}].Item, pl.Surface[Cell{4, 5}].Item)
	}
}

// Terraforming then a Factory on the same black square; an Automation
// on a queued structure; a target the project or an entry takes, twice,
// refused by the plan.
func TestEnqueueOnTheSameSquare(t *testing.T) {
	c, r := queueWorld(t)
	black := Cell{6, 5}
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	if err := c.CanEnqueue(r, Factory, black); err == nil || !strings.Contains(err.Error(), "black") {
		t.Errorf("Factory on black: %v", err)
	}
	for _, it := range []Item{Terraforming, Factory, Automation} {
		if err := c.Enqueue(r, it, black); err != nil {
			t.Fatalf("%s at %v: %v", it, black, err)
		}
	}
	for _, it := range []Item{Terraforming, Factory, Automation} {
		if err := c.CanEnqueue(r, it, black); err == nil {
			t.Errorf("%s at %v twice: accepted", it, black)
		}
	}
	if err := c.CanEnqueue(r, Agriplot, Cell{4, 5}); err == nil {
		t.Error("the running project's square: accepted")
	}
	if err := c.CanEnqueue(r, Automation, Cell{4, 5}); err != nil {
		t.Errorf("Automation on the running Factory: %v", err)
	}
	if err := c.EnqueueOrbital(r, Shipyard, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.CanEnqueueOrbital(r, OrbitalDocks, 0); err == nil {
		t.Error("orbit slot 0 twice: accepted")
	}
	if len(c.Queue) != 4 || c.Planet.Surface[black].Kind != Black || c.Planet.Orbit[0] != NoItem {
		t.Errorf("queue %v, square %v, slot 0 %s", c.Queue, c.Planet.Surface[black].Kind, c.Planet.Orbit[0])
	}
}

// Ships, refits' Docks aside, planet-wide projects, a Colony Base and an
// unknown item are never queued; free workers are not checked.
func TestEnqueueRefuses(t *testing.T) {
	c, r := queueWorld(t)
	r.Grant(Tech(LushGrowthBomb.Info().ResearchReq))
	r.Grant(Tech(EndlessParty.Info().ResearchReq))
	c.Planet.Orbit[1] = Shipyard
	if err := c.CanEnqueueOrbital(r, Ship, 2); err == nil {
		t.Error("a Ship queued")
	}
	for _, it := range []Item{LushGrowthBomb, EndlessParty, Shipyard} {
		if err := c.CanEnqueue(r, it, Cell{6, 5}); err == nil {
			t.Errorf("%s queued on a square", it)
		}
	}
	if err := c.CanEnqueue(r, ColonyBase, Cell{4, 5}); !errors.Is(err, ErrNotBuildable) {
		t.Errorf("Colony Base: %v", err)
	}
	if err := c.CanEnqueue(r, ResearchCampus, Cell{4, 5}); err == nil || !strings.Contains(err.Error(), "needs Hyperlogic") {
		t.Errorf("Research Campus: %v", err)
	}
	c.Pop = c.Busy()
	if err := c.CanEnqueue(r, Factory, Cell{4, 5}); err != nil {
		t.Errorf("no free people: %v", err)
	}
	if len(c.Queue) != 0 {
		t.Errorf("queue %v after checks", c.Queue)
	}
}

// The head starts at the close the project completes on; the next waits.
func TestNextAtTheClose(t *testing.T) {
	c, r := queueWorld(t)
	if err := c.Start(r, Factory, Cell{4, 5}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []Cell{{5, 4}, {5, 6}} {
		if err := c.Enqueue(r, Agriplot, at); err != nil {
			t.Fatal(err)
		}
	}
	for d := 1; ; d++ {
		evs := c.EndDay(r, 0)
		if !slices.ContainsFunc(evs, func(e Event) bool { return e.Kind == Built }) {
			if d > 100 {
				t.Fatal("no Built in 100 days")
			}
			continue
		}
		if slices.ContainsFunc(evs, func(e Event) bool { return e.Kind == Unqueued }) {
			t.Errorf("day %d: %v", d, evs)
		}
		break
	}
	if pj := c.Project; pj == nil || pj.Item != Agriplot || pj.Cell != (Cell{5, 4}) || pj.Progress != 0 {
		t.Fatalf("project after the close %+v, want the Agriplot at (5,4)", pj)
	}
	if len(c.Queue) != 1 || c.Queue[0].Cell != (Cell{5, 6}) {
		t.Errorf("queue %v, want the (5,6) Agriplot", c.Queue)
	}
}

// Short of people the head waits, Next changing nothing; a worker later
// it starts. An invalid head is dropped (Unqueued) and the next starts.
func TestNextWaitsAndDrops(t *testing.T) {
	c, r := queueWorld(t)
	black := Cell{6, 5}
	for _, q := range []struct {
		it Item
		at Cell
	}{{Terraforming, black}, {Factory, black}, {Agriplot, Cell{5, 4}}} {
		if err := c.Enqueue(r, q.it, q.at); err != nil {
			t.Fatal(err)
		}
	}
	c.Pop = c.Busy()
	if evs := c.Next(r); evs != nil || c.Project != nil || len(c.Queue) != 3 {
		t.Fatalf("no free people: events %v, project %v, queue %d", evs, c.Project, len(c.Queue))
	}
	if err := c.Dequeue(0); err != nil {
		t.Fatal(err)
	}
	c.Pop++
	evs := c.Next(r)
	if len(evs) != 1 || evs[0].Kind != Unqueued || evs[0].Item != Factory || evs[0].Star != c.Star || evs[0].Planet != c.PlanetIdx ||
		evs[0].Other != -1 || evs[0].Tech != NoTech || !strings.HasPrefix(evs[0].Text, "Factory dropped from the queue on Hand: ") {
		t.Errorf("events %+v, want the Factory's Unqueued", evs)
	}
	if pj := c.Project; pj == nil || pj.Item != Agriplot || len(c.Queue) != 0 {
		t.Errorf("project %+v, queue %v, want the Agriplot started", pj, c.Queue)
	}
	if evs := c.Next(r); evs != nil {
		t.Errorf("Next while running: %v", evs)
	}
}

// An ongoing project gives way to the head, at the close.
func TestNextReplacesOngoing(t *testing.T) {
	c, r := queueWorld(t)
	r.Grant(Tech(EndlessParty.Info().ResearchReq))
	if err := c.Enqueue(r, Agriplot, Cell{5, 4}); err != nil {
		t.Fatal(err)
	}
	if err := c.StartProject(r, EndlessParty); err != nil {
		t.Fatal(err)
	}
	c.EndDay(r, 0)
	if pj := c.Project; pj == nil || pj.Item != Agriplot || len(c.Queue) != 0 {
		t.Errorf("project %+v, queue %v, want the Agriplot", pj, c.Queue)
	}
}

// QueueUp swaps with the entry before, Dequeue removes; bad indices are
// refused; each accepted call is journalled with its entry.
func TestQueueUpDequeue(t *testing.T) {
	c, r := queueWorld(t)
	var buf bytes.Buffer
	c.journal = journal.New(&buf)
	ats := []Cell{{5, 4}, {5, 6}, {4, 5}}
	for _, at := range ats {
		if err := c.Enqueue(r, Agriplot, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.EnqueueOrbital(r, Shipyard, 3); err != nil {
		t.Fatal(err)
	}
	ens := drain(t, &buf)
	if len(ens) != 4 || ens[0].Act != "enqueue" || ens[0].Cell != "(5,4)" || ens[0].Slot != -1 || ens[3].Item != "Shipyard" || ens[3].Slot != 3 || ens[3].Cell != "" {
		t.Errorf("enqueue entries %+v", ens)
	}
	for _, i := range []int{0, -1, 4} {
		if err := c.QueueUp(i); err == nil {
			t.Errorf("QueueUp(%d) accepted", i)
		}
	}
	for _, i := range []int{-1, 4} {
		if err := c.Dequeue(i); err == nil {
			t.Errorf("Dequeue(%d) accepted", i)
		}
	}
	en := one(t, &buf, "QueueUp(2)", "queue-up", 0, c.QueueUp(2))
	if en.Cell != "(4,5)" || en.Colony != "Hand" {
		t.Errorf("queue-up entry %+v", en)
	}
	if got := []Cell{c.Queue[0].Cell, c.Queue[1].Cell, c.Queue[2].Cell}; !slices.Equal(got, []Cell{{5, 4}, {4, 5}, {5, 6}}) {
		t.Errorf("after QueueUp(2): %v", got)
	}
	en = one(t, &buf, "Dequeue(0)", "dequeue", 0, c.Dequeue(0))
	if en.Cell != "(5,4)" || en.Item != "Agriplot" {
		t.Errorf("dequeue entry %+v", en)
	}
	if len(c.Queue) != 3 || c.Queue[0].Cell != (Cell{4, 5}) || !c.Queue[2].Orbital {
		t.Errorf("after Dequeue(0): %v", c.Queue)
	}
	c.Next(r)
	if en := one(t, &buf, "Next", "start", 0, nil); en.Cell != "(4,5)" {
		t.Errorf("Next's entry %+v", en)
	}
}

// A captured colony loses its queue with its project.
func TestInvadeClearsQueue(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	e0 := g.Empires[0]
	col.Project = &Project{Item: Factory, Cell: col.Planet.Cells()[0], Slot: -1}
	col.Queue = []Queued{{Item: Agriplot, Cell: col.Planet.Cells()[1], Slot: -1}, {Item: Shipyard, Slot: 4, Orbital: true}}
	s := e0.Spawn(invader(3), col.Star, col.PlanetIdx)
	if err := e0.Order(s, Order{Kind: Invade}); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	if e0.Colony(col.Star, col.PlanetIdx) != col {
		t.Fatal("not captured")
	}
	if col.Project != nil || col.Queue != nil {
		t.Errorf("after the capture: project %+v, queue %v", col.Project, col.Queue)
	}
}

// A Ship completing: the head is judged once the crew has left the Pop
// (Empire.endDay's Next): short of the worker the crew took it waits,
// with one to spare it starts the same close.
func TestNextAfterLaunch(t *testing.T) {
	for _, spare := range []int{0, 1} {
		e, home := shipEmpire(t)
		d, ok := ColonyShip(e.Research, false)
		if !ok {
			t.Fatal("no colony ship")
		}
		at, ok := home.Planet.BestCell(Agriplot, home.Traits())
		if !ok {
			t.Fatal("no Agriplot square")
		}
		if err := e.BuildShip(home, d); err != nil {
			t.Fatal(err)
		}
		if err := home.Enqueue(e.Research, Agriplot, at); err != nil {
			t.Fatal(err)
		}
		home.Project.Progress = home.Project.Cost() - 1
		home.Pop, home.Growth = home.Busy()+spare, 0
		ships := len(e.Ships)
		e.EndDay()
		if len(e.Ships) != ships+1 {
			t.Fatalf("spare %d: no ship launched", spare)
		}
		started := home.Project != nil && home.Project.Item == Agriplot
		if started != (spare == 1) || home.Free() < 0 || len(home.Queue) != 1-spare {
			t.Errorf("spare %d: project %+v, free %d, queue %v", spare, home.Project, home.Free(), home.Queue)
		}
	}
}
