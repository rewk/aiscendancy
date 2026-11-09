package game

import (
	"errors"
	"strings"
	"testing"
)

// automationHome is a home colony with Automation researched, a Factory
// built next to its Colony Base and the people to staff a project, at
// capacity (no growth moves Pop); f is the Factory's cell.
func automationHome(t *testing.T) (c *Colony, r *Research, f Cell) {
	t.Helper()
	e := NewEmpire(1, Snovemdomas, Average)
	c, r = e.Colonies[0], e.Research
	r.Grant(Tech(Automation.Info().ResearchReq))
	at, ok := c.Planet.BestCell(Factory, c.Traits())
	if !ok {
		t.Fatal("no Factory square at home")
	}
	if err := c.Planet.Build(at, Factory); err != nil {
		t.Fatal(err)
	}
	c.Pop = c.Capacity()
	if c.Free() < 2 {
		t.Fatalf("home free %d: the fixture needs two", c.Free())
	}
	return c, r, at
}

// An Automation costs AutomationExtra plus its target structure's own
// Cost, whatever the structure; planitem.txt's Cost 1 plays no part.
func TestAutomationCost(t *testing.T) {
	for it := Item(0); it < FirstOrbital; it++ {
		pj := &Project{Item: Automation, Target: it}
		if got, want := pj.Cost(), AutomationExtra+it.Info().Cost; got != want {
			t.Errorf("Automation of %s costs %d, want %d", it, got, want)
		}
	}
	c, r, f := automationHome(t)
	if err := c.Start(r, Automation, f); err != nil {
		t.Fatal(err)
	}
	if got, want := c.Project.Cost(), AutomationExtra+Factory.Info().Cost; c.Project.Target != Factory || got != want {
		t.Errorf("started on the Factory: target %s, cost %d, want %d", c.Project.Target, got, want)
	}
	// The Colony Base: a Target that is not the zero Item.
	c.Cancel()
	if err := c.Start(r, Automation, Cell{Centre, Centre}); err != nil {
		t.Fatal(err)
	}
	if got, want := c.Project.Cost(), AutomationExtra+ColonyBase.Info().Cost; c.Project.Target != ColonyBase || got != want {
		t.Errorf("started on the Colony Base: target %s, cost %d, want %d", c.Project.Target, got, want)
	}
}

// While it builds an Automation takes one free worker (its item's
// UsePop); once complete the structure employs nobody: two more free
// than during the build, capacity and outputs unchanged. It completes on
// the day its industry reaches the cost, reported Built.
func TestAutomationWorkers(t *testing.T) {
	c, r, f := automationHome(t)
	p := c.Planet
	busy, capa := c.Busy(), c.Capacity()
	ind, res, pros := p.Output()
	out, _, _ := c.Output()
	if err := c.Start(r, Automation, f); err != nil {
		t.Fatal(err)
	}
	if got, want := c.Busy(), busy+Automation.Info().UsePop; got != want {
		t.Errorf("during: busy %d, want %d", got, want)
	}
	cost := c.Project.Cost()
	days := (cost + out - 1) / out
	d, ev := runUntil(t, c, r, Built, 2*days)
	if d != days {
		t.Errorf("completed on day %d, want %d (cost %d at %d a day)", d, days, cost, out)
	}
	if want := "Factory automated on " + p.Name; ev.Item != Automation || ev.Text != want {
		t.Errorf("event %+v, want %q", ev, want)
	}
	if !p.Surface[f].Automated || p.Surface[f].Item != Factory || c.Project != nil {
		t.Fatalf("after: square %+v, project %+v", *p.Surface[f], c.Project)
	}
	if got, want := c.Busy(), busy-Factory.Info().UsePop; got != want {
		t.Errorf("after: busy %d, want %d", got, want)
	}
	if got, want := c.Busy(), busy+Automation.Info().UsePop-2; got != want {
		t.Errorf("after: busy %d, want two under the build's %d", got, want+2)
	}
	if c.Capacity() != capa {
		t.Errorf("capacity %d, want %d", c.Capacity(), capa)
	}
	if i, rs, ps := p.Output(); i != ind || rs != res || ps != pros {
		t.Errorf("outputs %d/%d/%d, want %d/%d/%d", i, rs, ps, ind, res, pros)
	}
}

// The refusals: not researched, no structure, automated already, a
// project running, no free worker; never planet-wide nor orbital. The
// Colony Base is automatable; an ongoing project gives way.
func TestAutomationRefusals(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c, r := e.Colonies[0], e.Research
	base := Cell{Centre, Centre}
	c.Pop = c.Capacity()
	if err := c.CanStart(r, Automation, base); err == nil || !strings.Contains(err.Error(), Tech(Automation.Info().ResearchReq).String()) {
		t.Errorf("unresearched: %v", err)
	}
	c, r, f := automationHome(t)
	p := c.Planet
	if err := c.CanStart(r, Automation, base); err != nil || p.Surface[base].Item != ColonyBase {
		t.Errorf("the Colony Base (%s): %v", p.Surface[base].Item, err)
	}
	empty, ok := p.BestCell(Agriplot, c.Traits())
	if !ok {
		t.Fatal("no empty square")
	}
	if err := c.CanStart(r, Automation, empty); err == nil || !strings.Contains(err.Error(), "no structure") {
		t.Errorf("empty square: %v", err)
	}
	if err := c.CanStart(r, Automation, Cell{-1, -1}); err == nil {
		t.Error("off the grid: allowed")
	}
	if err := c.CanStartOrbital(r, Automation, 0); err == nil {
		t.Error("in orbit: allowed")
	}
	if err := c.CanStartProject(r, Automation); err == nil || errors.Is(err, ErrNotModelled) {
		t.Errorf("planet-wide: %v", err)
	}
	p.Surface[f].Automated = true
	if err := c.CanStart(r, Automation, f); err == nil || !strings.Contains(err.Error(), "automated already") {
		t.Errorf("automated already: %v", err)
	}
	p.Surface[f].Automated = false
	if err := c.Start(r, Agriplot, empty); err != nil {
		t.Fatal(err)
	}
	if err := c.CanStart(r, Automation, f); !errors.Is(err, ErrProjectRunning) {
		t.Errorf("a project running: %v, want %v", err, ErrProjectRunning)
	}
	c.Cancel()
	c.Pop = c.Busy() + Automation.Info().UsePop - 1
	if err := c.CanStart(r, Automation, f); !errors.Is(err, ErrNoFreePop) {
		t.Errorf("no free worker: %v, want %v", err, ErrNoFreePop)
	}
	c.Pop = c.Capacity()
	r.Grant(Tech(EndlessParty.Info().ResearchReq))
	if err := c.StartProject(r, EndlessParty); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(r, Automation, f); err != nil || c.Project.Item != Automation {
		t.Errorf("over an ongoing project: %v, project %+v", err, c.Project)
	}
}

// Demolish and Terraform clear Automated with the structure: a new one on
// the square employs its people. An Automation whose target was
// demolished, or replaced by another structure, completes as a dropped
// build: no event, nothing automated.
func TestAutomationDemolish(t *testing.T) {
	c, r, f := automationHome(t)
	p := c.Planet
	p.Surface[f].Automated = true
	busy := c.Busy()
	p.Demolish(f)
	if p.Surface[f].Automated {
		t.Error("demolished: still automated")
	}
	if err := p.Build(f, Factory); err != nil {
		t.Fatal(err)
	}
	if p.Surface[f].Automated || c.Busy() != busy+Factory.Info().UsePop {
		t.Errorf("rebuilt: automated %v, busy %d, want %d", p.Surface[f].Automated, c.Busy(), busy+Factory.Info().UsePop)
	}
	sq := p.Surface[f]
	sq.Kind, sq.Item, sq.Automated = Black, TransportTubes, true
	if err := p.Terraform(f); err != nil || sq.Automated {
		t.Errorf("terraformed tubes: %v, square %+v", err, *sq)
	}
	sq.Kind = White

	for _, replace := range []Item{NoItem, Agriplot} {
		if err := p.Build(f, Factory); err != nil {
			t.Fatal(err)
		}
		if err := c.Start(r, Automation, f); err != nil {
			t.Fatal(err)
		}
		p.Demolish(f)
		if replace != NoItem {
			if err := p.Build(f, replace); err != nil {
				t.Fatal(err)
			}
		}
		for day := 0; day < 500 && c.Project != nil; day++ {
			for _, ev := range c.EndDay(r, 0) {
				if ev.Kind == Built {
					t.Errorf("replaced by %s: %+v", replace, ev)
				}
			}
		}
		if c.Project != nil || sq.Automated || sq.Item != replace {
			t.Errorf("replaced by %s: project %+v, square %+v", replace, c.Project, *sq)
		}
		p.Demolish(f)
	}
}
