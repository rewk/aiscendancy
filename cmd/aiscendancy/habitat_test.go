package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// habDesign carries two Space Habitats.
var habDesign = game.Design{Name: "Hab", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.SpaceHabitat, game.SpaceHabitat}}

// habApp is seed 1 alone, the home's people raised, a habitat ship in
// its orbit, the home's planet screen entered.
func habApp(t *testing.T) (a *app, s *planetScreen, hab *game.Starship, col *game.Colony) {
	t.Helper()
	a = testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average))
	col = a.e.Colonies[0]
	col.Pop = col.Busy() + 3
	if col.Pop >= col.Capacity() {
		t.Fatalf("fixture: pop %d of %d", col.Pop, col.Capacity())
	}
	hab = a.e.Spawn(habDesign, col.Star, col.PlanetIdx)
	s = newPlanetScreen(col.Star, col.PlanetIdx)
	a.push(s)
	return a, s, hab, col
}

// fullDots counts the full berths of berthDots.
func fullDots(sh *game.Starship) int {
	ds, _ := berthDots(sh)
	n := 0
	for _, d := range ds {
		if d.full {
			n++
		}
	}
	return n
}

// Load and Unload: offered for a habitat over the player's colony, greyed
// with the rule's refusal as hover; a press moves one unit and keeps the
// dialog up, the POP panel's counts and the Aboard row following.
func TestOrbitShipLoad(t *testing.T) {
	a, s, hab, col := habApp(t)
	s.openShipDlg(hab)
	if got := s.dlgActs(a); !slices.Equal(got, shipDlgWant(a, hab)) || !slices.Contains(got, dlgLoad) || !slices.Contains(got, dlgUnload) {
		t.Fatalf("bar %v, want %v", got, shipDlgWant(a, hab))
	}
	err := a.e.CanUnload(hab)
	if err == nil || !s.dlgOff(a, dlgUnload) || s.dlgOff(a, dlgLoad) || s.shipDlgText(a, hab, dlgUnload) != "Unload: "+err.Error() {
		t.Errorf("empty: unload off %v (%v), load off %v, %q", s.dlgOff(a, dlgUnload), err, s.dlgOff(a, dlgLoad), s.shipDlgText(a, hab, dlgUnload))
	}
	if got := s.shipDlgText(a, hab, dlgLoad); got != "Load: one colonist boards "+hab.Name {
		t.Errorf("load text %q", got)
	}
	if _, err := s.press(a, dlgUnload); err != nil || hab.Pop != 0 || s.dlg == nil {
		t.Errorf("greyed unload: %v, pop %d", err, hab.Pop)
	}
	busy, free, room := s.popCounts()
	pop := col.Pop
	if _, err := s.press(a, dlgLoad); err != nil {
		t.Fatal(err)
	}
	if s.dlg == nil || s.dlg.ship != hab || col.Pop != pop-1 || hab.Pop != 1 {
		t.Fatalf("load: dialog %+v, colony %d ship %d", s.dlg, col.Pop, hab.Pop)
	}
	if b, f, r := s.popCounts(); b != busy || f != free-1 || r != room+1 {
		t.Errorf("POP after load %d/%d/%d, before %d/%d/%d", b, f, r, busy, free, room)
	}
	if ds, _ := berthDots(hab); len(ds) != hab.Berths() || fullDots(hab) != 1 {
		t.Errorf("aboard row: %d dots, %d full", len(ds), fullDots(hab))
	}
	if got := s.shipDlgText(a, hab, dlgUnload); got != "Unload: one colonist lands on "+col.Planet.Name {
		t.Errorf("unload text %q", got)
	}
	if _, err := s.press(a, dlgUnload); err != nil || s.dlg == nil || col.Pop != pop || hab.Pop != 0 {
		t.Errorf("unload: %v, colony %d ship %d", err, col.Pop, hab.Pop)
	}
	// Full: Load greyed with the rule's sentence.
	hab.Pop = hab.Berths()
	if err := a.e.CanLoad(hab); err == nil || !s.dlgOff(a, dlgLoad) || s.shipDlgText(a, hab, dlgLoad) != "Load: "+err.Error() {
		t.Errorf("full: %v, %q", err, s.shipDlgText(a, hab, dlgLoad))
	}
	if fullDots(hab) != hab.Berths() {
		t.Errorf("full row: %d full", fullDots(hab))
	}
	// A ship with no habitat: no Load.
	plain := a.e.Spawn(game.Design{Name: "Plain", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}, col.Star, col.PlanetIdx)
	s.openShipDlg(plain)
	if got := s.dlgActs(a); slices.Contains(got, dlgLoad) || slices.Contains(got, dlgUnload) {
		t.Errorf("no habitat: %v", got)
	}
	if ds, _ := berthDots(plain); ds != nil {
		t.Errorf("no habitat: %d dots", len(ds))
	}
}

// Over another empire's colony: no Load nor Unload.
func TestOrbitShipLoadForeign(t *testing.T) {
	a, s, _, col := invadeApp(t)
	hab := a.e.Spawn(habDesign, col.Star, col.PlanetIdx)
	s.openShipDlg(hab)
	if got := s.dlgActs(a); slices.Contains(got, dlgLoad) || slices.Contains(got, dlgUnload) || !slices.Equal(got, shipDlgWant(a, hab)) {
		t.Errorf("foreign colony: %v", got)
	}
}

// The berths' circles: a module's together, inside the left band, under
// the label, wrapped by whole modules.
func TestBerthDots(t *testing.T) {
	per := game.Gizmos[game.SpaceHabitat].Special1
	for n := 1; n <= 12; n++ {
		parts := []game.Gizmo{game.ProtonShaver}
		for range n {
			parts = append(parts, game.SpaceHabitat)
		}
		sh := &game.Starship{Design: game.Design{Hull: game.Enormous, Parts: parts}, Pop: 1}
		ds, top := berthDots(sh)
		if len(ds) != n*per || top < dlgHelpBandY0 {
			t.Fatalf("%d modules: %d dots, top %d", n, len(ds), top)
		}
		for i, d := range ds {
			if d.x-berthR < dlgPicX0 || d.x+berthR > dlgPicX1 || d.y-berthR < float32(top)+float32(smallH) || d.y+berthR > berthBottom {
				t.Errorf("%d modules, dot %d at (%v,%v) out of the band", n, i, d.x, d.y)
			}
			if d.full != (i < sh.Pop) {
				t.Errorf("%d modules, dot %d full %v", n, i, d.full)
			}
			if i%per > 0 && (d.y != ds[i-1].y || d.x-ds[i-1].x != berthPitch) {
				t.Errorf("%d modules: dot %d not beside %d", n, i, i-1)
			}
			if i%per == 0 && i > 0 && d.y == ds[i-1].y && d.x-ds[i-1].x != berthPitch+berthGroupGap {
				t.Errorf("%d modules: module gap at %d %v", n, i, d.x-ds[i-1].x)
			}
		}
	}
}

// The system panel: our habitat's tile badges aboard/berths, its hover
// names the planet screen.
func TestHabitatTile(t *testing.T) {
	sh := &game.Starship{Design: habDesign, Pop: 2, Orbit: -1, Lane: -1}
	i := slices.IndexFunc(panelTiles(sh), func(t panelTile) bool { return t.g == game.SpaceHabitat })
	if i < 0 {
		t.Fatal("no habitat tile")
	}
	tl := panelTiles(sh)[i]
	if got := tileState(sh, tl); got != "2/6" {
		t.Errorf("badge %q", got)
	}
	if got := tileText(sh, tl); !strings.Contains(got, planetUse(game.SpaceHabitat)) || planetUse(game.SpaceHabitat) == "" {
		t.Errorf("hover %q", got)
	}
	if tileClickable(game.SpaceHabitat) {
		t.Error("a habitat tile is clickable")
	}
}
