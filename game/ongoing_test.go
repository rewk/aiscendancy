package game

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// ongoingFixture is shipEmpire (a Shipyard) with five Factories, two
// free workers and both ongoing projects researched.
func ongoingFixture(t *testing.T) (*Empire, *Colony) {
	t.Helper()
	e, c := shipEmpire(t)
	for _, it := range []Item{EndlessParty, ScientistTakeover} {
		e.Research.Grant(Tech(it.Info().ResearchReq))
	}
	for range 3 { // industry enough that the share's divisor shows
		at, ok := c.Planet.BestCell(Factory, Base)
		if !ok || c.Planet.Build(at, Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	c.Pop = c.Busy() + 2
	if sum, _, _ := c.Planet.Output(); sum/4 == sum/3 {
		t.Fatalf("industry sum %d: a quarter is a third", sum)
	}
	return e, c
}

func TestOngoing(t *testing.T) {
	for it := Item(0); it < NumItems; it++ {
		if want := it == EndlessParty || it == ScientistTakeover || it == AlienHospitality; it.Ongoing() != want {
			t.Errorf("%s: Ongoing %v", it, it.Ongoing())
		}
	}
}

// While an ongoing project runs the industry is 0 and its quarter,
// floor(Σ industry × hyperpower) / 4 off Planet.Output, goes to
// prosperity before the curve (party) or to research (takeover); Shown
// is Output's, its industry the idle one.
func TestOngoingOutput(t *testing.T) {
	e, c := ongoingFixture(t)
	sumInd, sumRes, sumPros := c.Planet.Output()
	q := int(math.Floor(float64(sumInd)*plant(c.Planet, HyperpowerPlant))) / 4
	if q == 0 {
		t.Fatalf("fixture diverts nothing (industry sum %d)", sumInd)
	}
	ind0, res0, pros0 := c.Output()
	if ind0 == 0 {
		t.Fatal("idle fixture makes no industry")
	}
	if it, n := c.Diverted(); it != NoItem || n != 0 {
		t.Errorf("idle: Diverted %s %d", it, n)
	}
	if i, r, p := c.Shown(); i != ind0 || r != res0 || p != pros0 {
		t.Errorf("idle: Shown %d/%d/%d, want Output's %d/%d/%d", i, r, p, ind0, res0, pros0)
	}
	for _, tc := range []struct {
		it        Item
		res, pros int
	}{
		{EndlessParty, res0, power(math.Floor(float64(sumPros)*plant(c.Planet, FertilizationPlant))+float64(q)) - c.Pop/4},
		{ScientistTakeover, int(math.Floor(float64(sumRes)*plant(c.Planet, Internet))) + q, pros0},
	} {
		if err := c.StartProject(e.Research, tc.it); err != nil {
			t.Fatal(err)
		}
		ind, res, pros := c.Output()
		if ind != 0 || res != tc.res || pros != tc.pros {
			t.Errorf("%s: output %d/%d/%d, want 0/%d/%d", tc.it, ind, res, pros, tc.res, tc.pros)
		}
		if it, n := c.Diverted(); it != tc.it || n != q {
			t.Errorf("%s: Diverted %s %d, want %d", tc.it, it, n, q)
		}
		if i, r, p := c.Shown(); i != ind0 || r != res || p != pros { // the industry kept, curved
			t.Errorf("%s: Shown %d/%d/%d, want %d/%d/%d", tc.it, i, r, p, ind0, res, pros)
		}
		if _, ok := c.DaysLeft(); ok {
			t.Errorf("%s: DaysLeft reports a completion", tc.it)
		}
	}
	// Alien Hospitality diverts the curved industry's quarter (ours), the
	// colony's output else as idle bar the industry.
	e.Research.Grant(Tech(AlienHospitality.Info().ResearchReq))
	if err := c.StartProject(e.Research, AlienHospitality); err != nil {
		t.Fatal(err)
	}
	if it, n := c.Diverted(); it != AlienHospitality || n != ind0/4 {
		t.Errorf("hospitality: Diverted %s %d, want %d", it, n, ind0/4)
	}
	if _, n := c.divert(400); n != c.industry(400)/4 || n >= 100 { // the raw quarter would be 100
		t.Errorf("hospitality on a sum of 400: %d, want %d", n, c.industry(400)/4)
	}
	if ind, res, pros := c.Output(); ind != 0 || res != res0 || pros != pros0 {
		t.Errorf("hospitality: output %d/%d/%d, want 0/%d/%d", ind, res, pros, res0, pros0)
	}
}

// An ongoing project never completes, takes no industry, starts with no
// event; the research it makes reaches the empire.
func TestOngoingNeverCompletes(t *testing.T) {
	for _, it := range []Item{EndlessParty, ScientistTakeover} {
		e, c := ongoingFixture(t)
		if err := c.StartProject(e.Research, it); err != nil {
			t.Fatal(err)
		}
		pj := c.Project
		for day := range 500 {
			for _, x := range c.EndDay(e.Research, 0) {
				if x.Kind == Built {
					t.Fatalf("%s: day %d: %+v", it, day, x)
				}
			}
			if c.Project != pj || pj.Progress != 0 || c.Carry != 0 {
				t.Fatalf("%s: day %d: project %v, progress %d, carry %d", it, day, c.Project, pj.Progress, c.Carry)
			}
		}
	}
}

// Any start replaces a running ongoing project: a structure, an
// orbital, a ship, a refit, the other ongoing project; the same one again
// is refused; not researched, refused.
func TestOngoingGivesWay(t *testing.T) {
	start := func(t *testing.T, it Item) (*Empire, *Colony) {
		t.Helper()
		e, c, _ := refitFixture(t)
		for _, x := range []Item{EndlessParty, ScientistTakeover} {
			e.Research.Grant(Tech(x.Info().ResearchReq))
		}
		if err := c.StartProject(e.Research, it); err != nil {
			t.Fatal(err)
		}
		return e, c
	}
	for _, tc := range []struct {
		name string
		do   func(e *Empire, c *Colony) error
		want Item
	}{
		{"structure", func(e *Empire, c *Colony) error {
			at, _ := c.Planet.BestCell(Factory, Base)
			return c.Start(e.Research, Factory, at)
		}, Factory},
		{"orbital", func(e *Empire, c *Colony) error {
			e.Research.Grant(Tech(OrbitalShield.Info().ResearchReq))
			return c.StartOrbital(e.Research, OrbitalShield, OrbitSlots-1)
		}, OrbitalShield},
		{"ship", func(e *Empire, c *Colony) error {
			d, _ := ColonyShip(e.Research, false)
			return e.BuildShip(c, d)
		}, Ship},
		{"refit", func(e *Empire, c *Colony) error {
			s := e.Ships[0]
			return e.Refit(c, s, withGen(s))
		}, OrbitalDocks},
		{"the other one", func(e *Empire, c *Colony) error {
			return c.StartProject(e.Research, ScientistTakeover)
		}, ScientistTakeover},
	} {
		e, c := start(t, EndlessParty)
		if err := tc.do(e, c); err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if c.Project == nil || c.Project.Item != tc.want {
			t.Errorf("%s: project %v, want %s", tc.name, c.Project, tc.want)
		}
	}
	e, c := start(t, ScientistTakeover)
	if err := c.StartProject(e.Research, ScientistTakeover); !errors.Is(err, ErrProjectRunning) || !strings.Contains(err.Error(), "already running") {
		t.Errorf("same one again: %v", err)
	}
	if err := c.CanStartProject(e.Research, EndlessParty); err != nil {
		t.Errorf("the other one: %v", err)
	}
	// A project that completes holds the colony against an ongoing start.
	at, _ := c.Planet.BestCell(Factory, Base)
	if err := c.Start(e.Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	if err := c.CanStartProject(e.Research, EndlessParty); !errors.Is(err, ErrProjectRunning) {
		t.Errorf("ongoing over a Factory: %v", err)
	}
	if _, ok := c.DaysLeft(); !ok {
		t.Error("the Factory reports no completion")
	}
	// Not researched.
	f := NewEmpire(1, Snovemdomas, Average)
	for _, it := range []Item{EndlessParty, ScientistTakeover} {
		if err := f.Colonies[0].CanStartProject(f.Research, it); err == nil || errors.Is(err, ErrNotModelled) {
			t.Errorf("%s unresearched: %v", it, err)
		}
	}
}

// DaysFor under an ongoing project counts the industry the start brings
// back (any start replaces the ongoing one): the days it says are the
// days a structure started then takes, not "no industry".
func TestDaysForOngoing(t *testing.T) {
	e, c := ongoingFixture(t)
	cost := Factory.Info().Cost
	idle, ok := c.DaysFor(cost)
	if !ok {
		t.Fatal("idle fixture makes no industry")
	}
	if err := c.StartProject(e.Research, EndlessParty); err != nil {
		t.Fatal(err)
	}
	n, ok := c.DaysFor(cost)
	if !ok || n != idle {
		t.Fatalf("under the party: DaysFor %d %v, want %d (idle)", n, ok, idle)
	}
	at, found := c.Planet.BestCell(Factory, Base)
	if !found {
		t.Fatal("no square for a Factory")
	}
	if err := c.Start(e.Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	days := 0
	for ; c.Project != nil && days <= 1000; days++ {
		c.EndDay(e.Research, 0)
	}
	if days != n {
		t.Errorf("the Factory took %d days, DaysFor said %d", days, n)
	}
}
