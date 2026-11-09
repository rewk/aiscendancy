package game

import (
	"math"
	"strings"
	"testing"
)

// singularFixture is ongoingFixture with Laboratories until the
// research is odd and at least 3 (the halving floors), the tree still
// incomplete.
func singularFixture(t *testing.T) (*Empire, *Colony) {
	t.Helper()
	e, c := ongoingFixture(t)
	for _, res, _ := c.Output(); res < 3 || res%2 == 0; _, res, _ = c.Output() {
		at, ok := c.Planet.BestCell(Laboratory, Base)
		if !ok || c.Planet.Build(at, Laboratory) != nil {
			t.Fatalf("no square for a Laboratory (research %d)", res)
		}
	}
	c.Pop = c.Busy() + 2
	if e.Research.Complete() || c.Singular() {
		t.Fatal("fixture: tree complete")
	}
	return e, c
}

// grantAll learns every tech r does not know.
func grantAll(r *Research) {
	for tc := range NumTechs {
		r.Grant(tc)
	}
}

// Past the singularity res / SingularityShare of the final research
// moves to the industry; an ongoing project switches all of it off, its
// q still the planet's industry's; Scientist Takeover is refused.
func TestSingularityOutput(t *testing.T) {
	e, c := singularFixture(t)
	ind0, res0, pros0 := c.Output()
	if err := c.CanStartProject(e.Research, ScientistTakeover); err != nil {
		t.Fatalf("takeover before: %v", err)
	}
	sumInd, _, _ := c.Planet.Output()
	q0 := int(math.Floor(float64(sumInd)*plant(c.Planet, HyperpowerPlant))) / OngoingShare
	grantAll(e.Research)
	if !e.Research.Complete() || !c.Singular() {
		t.Fatal("all granted: not Complete")
	}
	conv := res0 / SingularityShare
	if ind, res, pros := c.Output(); ind != ind0+conv || res != res0-conv || pros != pros0 {
		t.Errorf("output %d/%d/%d, want %d/%d/%d", ind, res, pros, ind0+conv, res0-conv, pros0)
	}
	if i, r, p := c.Shown(); i != ind0+conv || r != res0-conv || p != pros0 {
		t.Errorf("Shown %d/%d/%d", i, r, p)
	}
	if n, ok := c.DaysFor(ind0 + conv); !ok || n != 1 {
		t.Errorf("DaysFor one day's industry: %d %v", n, ok)
	}
	err := c.CanStartProject(e.Research, ScientistTakeover)
	if err == nil || !strings.Contains(err.Error(), "research is complete") {
		t.Errorf("takeover past the singularity: %v", err)
	}
	if err := c.StartProject(e.Research, EndlessParty); err != nil {
		t.Fatal(err)
	}
	if it, q := c.Diverted(); it != EndlessParty || q != q0 {
		t.Errorf("party: Diverted %s %d, want %d", it, q, q0)
	}
	if ind, res, _ := c.Output(); ind != 0 || res != res0-conv {
		t.Errorf("party: output %d/%d, want 0/%d", ind, res, res0-conv)
	}
	if i, _, _ := c.Shown(); i != ind0+conv {
		t.Errorf("party: Shown industry %d, want %d", i, ind0+conv)
	}
}

// singularities counts the singularity's events in ev, checking each.
func singularities(t *testing.T, ev []Event) int {
	t.Helper()
	n := 0
	for _, x := range ev {
		if x.Text != SingularityText {
			continue
		}
		n++
		if x != (Event{Kind: Discovered, Item: NoItem, Tech: NoTech, Star: -1, Planet: -1, Other: -1, Text: SingularityText}) {
			t.Errorf("singularity event %+v", x)
		}
	}
	return n
}

// The last tech studied completes the tree: its Discovered, then the
// singularity's, once; the next day none. A running Scientist Takeover
// is dropped at the next EndDay.
func TestSingularityAdvance(t *testing.T) {
	e, c := singularFixture(t)
	r := e.Research
	if err := c.StartProject(r, ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	last := NumTechs - 1
	for tc := range NumTechs {
		if tc != last {
			r.Grant(tc)
		}
	}
	if err := r.Study(last); err != nil {
		t.Fatal(err)
	}
	r.Progress[last] = r.Cost(last) - 1
	ev := c.EndDay(r, 0)
	var at int
	for at = range ev {
		if ev[at].Kind == Discovered && ev[at].Tech == last {
			break
		}
	}
	if at+1 >= len(ev) || ev[at].Tech != last || ev[at+1].Text != SingularityText || singularities(t, ev) != 1 {
		t.Fatalf("events %+v", ev)
	}
	if c.Project == nil || c.Project.Item != ScientistTakeover {
		t.Fatal("takeover dropped on the completing day (before its EndDay)")
	}
	ev = c.EndDay(r, 0)
	if n := singularities(t, ev); n != 0 {
		t.Errorf("next day: %d singularities", n)
	}
	if c.Project != nil {
		t.Errorf("takeover kept: %+v", c.Project)
	}
}

// The empire's day completing the tree stops every running Scientist
// Takeover that day, the completing colony's own included.
func TestSingularityStopsTakeovers(t *testing.T) {
	e, c := singularFixture(t)
	r := e.Research
	if err := c.StartProject(r, ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	last := NumTechs - 1
	for tc := range NumTechs {
		if tc != last {
			r.Grant(tc)
		}
	}
	if err := r.Study(last); err != nil {
		t.Fatal(err)
	}
	r.Progress[last] = r.Cost(last) - 1
	if singularities(t, e.EndDay()) != 1 {
		t.Fatal("no singularity")
	}
	if c.Project != nil {
		t.Errorf("takeover kept past the singularity's day: %+v", c.Project)
	}
}

// takeover starts a Scientist Takeover at e's home.
func takeover(t *testing.T, e *Empire) *Colony {
	t.Helper()
	e.Research.Grant(Tech(ScientistTakeover.Info().ResearchReq))
	c := e.Colonies[0]
	if err := c.StartProject(e.Research, ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	return c
}

// The dig's grant of the last tech: Discovered at the colony, then the
// singularity's.
func TestSingularityDig(t *testing.T) {
	e, c, at := digHome(t, 1)
	r := e.Research
	last := NumTechs - 1
	for tc := range NumTechs {
		if tc != last {
			r.Grant(tc)
		}
	}
	if err := c.Start(r, XenoArcheologicalDig, at); err != nil {
		t.Fatal(err)
	}
	ev := runDig(t, c, r, 7)
	n := len(ev)
	for k, x := range ev {
		if x.Kind == Discovered && x.Tech == last {
			n = k
		}
	}
	if n+1 >= len(ev) || ev[n+1].Text != SingularityText || singularities(t, ev) != 1 {
		t.Fatalf("events %+v", ev)
	}
	if n := singularities(t, c.EndDay(r, 7)); n != 0 {
		t.Errorf("next day: %d singularities", n)
	}
}

// The Dubtaks' theft of the last techs: the singularity's Report after
// the last stolen, once.
func TestSingularitySteal(t *testing.T) {
	g := NewGame(1, []Species{Dubtaks, Orfa, Mebes}, Average)
	grantAll(g.Empires[1].Research)
	grantAll(g.Empires[2].Research)
	c := takeover(t, g.Empires[0])
	reps := used(t, g, 0)
	if len(reps) < 3 || reps[len(reps)-1] != (Report{0, Event{Kind: Discovered, Item: NoItem, Tech: NoTech, Star: -1, Planet: -1, Other: -1, Text: SingularityText}}) ||
		reps[len(reps)-2].Event.Tech != NumTechs-1 {
		t.Fatalf("reports %+v", reps)
	}
	var ev []Event
	for _, x := range reps {
		ev = append(ev, x.Event)
	}
	if n := singularities(t, ev); n != 1 {
		t.Errorf("%d singularities", n)
	}
	if c.Project != nil {
		t.Errorf("takeover kept after the theft: %+v", c.Project)
	}
}

// A research exchange learning the last tech: the singularity's Report
// after the learner's TradeMade; nothing for the side that learnt
// nothing, its tree complete already.
func TestSingularityTrade(t *testing.T) {
	g := dipGame(t)
	g.Contact(0, 1)
	e0, e1 := g.Empires[0], g.Empires[1]
	grantAll(e1.Research)
	last := NumTechs - 1
	for tc := range NumTechs {
		if tc != last {
			e0.Research.Grant(tc)
		}
	}
	c := takeover(t, e0)
	if _, err := g.OfferTrade(0, 1, TradeResearch); err != nil {
		t.Fatal(err)
	}
	reps := accept(t, g, 0, 1)
	if len(reps) != 3 || reps[0].Empire != 0 || reps[0].Event.Kind != TradeMade || reps[0].Event.Tech != last ||
		reps[1] != (Report{0, Event{Kind: Discovered, Item: NoItem, Tech: NoTech, Star: -1, Planet: -1, Other: -1, Text: SingularityText}}) ||
		reps[2].Empire != 1 || reps[2].Event.Kind != TradeMade {
		t.Fatalf("reports %+v", reps)
	}
	if !e0.Research.Complete() {
		t.Error("not Complete after the trade")
	}
	if c.Project != nil {
		t.Errorf("takeover kept after the trade: %+v", c.Project)
	}
}

// A Scientist Takeover still running past the singularity (the state
// Colony.EndDay alone leaves; Empire.stopTakeovers clears it): DaysFor forecasts at the output the start leaves, the
// takeover's research share, and so its industry share, out.
func TestSingularityDaysForTakeover(t *testing.T) {
	e, c := singularFixture(t)
	if err := c.StartProject(e.Research, ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	grantAll(e.Research)
	pj := c.Project
	c.Project = nil // what a start leaves
	ind, _, _ := c.Output()
	c.Project = pj
	const k = 50
	if n, ok := c.DaysFor(k * ind); !ok || n != k {
		t.Errorf("DaysFor %d: %d %v, want %d (industry %d once started)", k*ind, n, ok, k, ind)
	}
	if c.Project != pj {
		t.Error("DaysFor changed the project")
	}
}
