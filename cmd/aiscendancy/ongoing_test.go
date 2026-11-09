package main

import (
	"fmt"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// ongoingHome is pickerHome with both ongoing projects researched and
// Scientist Takeover running.
func ongoingHome(t *testing.T) (*app, *planetScreen, *game.Colony) {
	t.Helper()
	a, s, col := pickerHome(t)
	for _, it := range []game.Item{game.EndlessParty, game.ScientistTakeover} {
		a.e.Research.Grant(game.Tech(it.Info().ResearchReq))
	}
	if err := col.StartProject(a.e.Research, game.ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	return a, s, col
}

// The project panel and the planets list say "ongoing: +N research" or
// "+N prosperity", N game's Diverted share.
func TestOngoingFoot(t *testing.T) {
	a, _, col := ongoingHome(t)
	for _, tc := range []struct {
		it   game.Item
		word string
	}{{game.EndlessParty, "prosperity"}, {game.ScientistTakeover, "research"}} {
		if err := col.StartProject(a.e.Research, tc.it); err != nil {
			t.Fatal(err)
		}
		it, n := col.Diverted()
		want := fmt.Sprintf("ongoing: +%d %s", n, tc.word)
		var pp projPanel
		pp.fill(col, "idle")
		if it != tc.it || pp.name != tc.it.String() || pp.foot != want {
			t.Errorf("%s: panel %q / %q, want %q", tc.it, pp.name, pp.foot, want)
		}
		if _, _, _, proj, foot := colonyLines(a.e, a.cl, col); proj != "Building "+tc.it.String() || foot != want {
			t.Errorf("%s: planets list %q / %q", tc.it, proj, foot)
		}
	}
}

// The output pictures follow the project the moment it starts: under
// Scientist Takeover the industry stays (curved, as idle), its share
// added to research,
// the cue pointing left (research's picture); Endless Party's right;
// none idle.
func TestOngoingPanels(t *testing.T) {
	a, s, col := ongoingHome(t)
	for range 3 { // industry enough for a share
		at, ok := col.Planet.BestCell(game.Factory, game.Base)
		if !ok || col.Planet.Build(at, game.Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	col.Pop = col.Busy() + 2
	pj := col.Project // the idle output: the industry shown curved, kept under the project
	col.Project = nil
	ind, res, _ := col.Output()
	col.Project = pj
	if sum, _, _ := col.Planet.Output(); ind == sum {
		t.Fatalf("fixture: industry %d not curved", sum)
	}
	net := func() int { _, _, p := col.Output(); return p } // prosperity shown net
	_, q := col.Diverted()
	if got, want := outputValues(s.colony()), [3]int{res + q, ind, net()}; q == 0 || got != want {
		t.Errorf("takeover: pictures %v, want %v (share %d)", got, want, q)
	}
	if d := shiftCue(col); d != -1 {
		t.Errorf("takeover: cue %d, want -1", d)
	}
	if err := col.StartProject(a.e.Research, game.EndlessParty); err != nil {
		t.Fatal(err)
	}
	if got, want := outputValues(s.colony()), [3]int{res, ind, net()}; got != want || shiftCue(col) != 1 || net() <= 0 {
		t.Errorf("party: pictures %v, want %v; cue %d, want 1", got, want, shiftCue(col))
	}
	col.Project = nil
	if shiftCue(col) != 0 || shiftCue(nil) != 0 {
		t.Error("a cue without an ongoing project")
	}
	if got, want := outputValues(s.colony()), [3]int{res, ind, net()}; got != want {
		t.Errorf("idle: pictures %v, want %v", got, want)
	}
}

// An ongoing project's picture is whole, never night-tinted as unbuilt;
// another project's follows its Progress.
func TestProjDone(t *testing.T) {
	_, _, col := ongoingHome(t)
	if d := projDone(col.Project); d != 1 {
		t.Errorf("ongoing: done %v, want 1", d)
	}
	pj := &game.Project{Item: game.Factory}
	pj.Progress = pj.Cost() / 2
	if d := projDone(pj); d != float64(pj.Progress)/float64(pj.Cost()) {
		t.Errorf("factory half built: done %v", d)
	}
}

// With an ongoing project running a square opens the picker (no cancel
// question) and a pick replaces the project; the running ongoing
// project's own button asks to cancel it, the other one's replaces it.
func TestOngoingClicks(t *testing.T) {
	a, s, col := ongoingHome(t)
	if s.hoverColour(a) == orange {
		t.Error("hover orange: an ongoing project gives way")
	}
	if _, err := s.clickSlot(a); err != nil || s.pick == nil || s.confirm != nil {
		t.Fatalf("square click: %v, picker %v, question %v", err, s.pick != nil, s.confirm != nil)
	}
	s.pick = nil
	if err := s.start(a, game.Factory); err != nil || col.Project == nil || col.Project.Item != game.Factory {
		t.Fatalf("pick: %v, project %v", err, col.Project)
	}
	col.Cancel()

	if err := col.StartProject(a.e.Research, game.ScientistTakeover); err != nil {
		t.Fatal(err)
	}
	if err := s.clickProject(a, game.EndlessParty); err != nil || s.confirm != nil || col.Project.Item != game.EndlessParty {
		t.Errorf("the other button: %v, question %v, project %v", err, s.confirm != nil, col.Project)
	}
	if err := s.clickProject(a, game.EndlessParty); err != nil || s.confirm == nil || s.doomed != col.Project {
		t.Errorf("its own button: %v, question %v", err, s.confirm != nil)
	}
	s.answerCancel(true)
	if col.Project != nil {
		t.Errorf("cancelled: project %v", col.Project)
	}
}

// The industry picture carries Alien Hospitality's icon while it runs,
// not under another ongoing project, an idle colony or none.
func TestHospitable(t *testing.T) {
	_, _, col := ongoingHome(t) // Scientist Takeover running
	if hospitable(col) || hospitable(nil) {
		t.Error("an icon under Scientist Takeover or without a colony")
	}
	col.Project = &game.Project{Item: game.AlienHospitality}
	if !hospitable(col) {
		t.Error("no icon under Alien Hospitality")
	}
	col.Project = nil
	if hospitable(col) {
		t.Error("an icon on an idle colony")
	}
}
