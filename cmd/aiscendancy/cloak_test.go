package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// The cloakers on the views (game.Empire.SeesOrbit, SeesSurface): the
// planet panel, the guard ring, the system's bodies, the planet screen.

// cloakFixture is empire 1's home seen by the player (empire 0): the
// panel's homeDefences plus an Orbital Cloaker in slot 9, a ship of
// empire 1 in its orbit (hidden) then one in space (seen).
func cloakFixture(t *testing.T) (a *app, f *game.Colony, hidden, seen *game.Starship) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a = testApp(g)
	f = g.Empires[1].Colonies[0]
	defences(t, f.Planet, homeDefences)
	defences(t, f.Planet, map[int]game.Item{9: game.OrbitalCloaker})
	a.e.Visit(f.Star)
	d := game.Design{Name: "Picket", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver}}
	hidden = g.Empires[1].Spawn(d, f.Star, f.PlanetIdx)
	seen = g.Empires[1].Spawn(d, f.Star, f.PlanetIdx)
	seen.Orbit = -1
	return a, f, hidden, seen
}

// The planet panel on a foreign cloaked orbit: no tile, "orbit: cloaked"
// for the guard line, no ring; the ship in orbit not a body, the one in
// space paired with its own sighting. ScanAll, then the cloaker razed,
// show it all again.
func TestCloakedOrbitPanel(t *testing.T) {
	a, f, hidden, seen := cloakFixture(t)
	w := newSystemScreen(f.Star)
	w.Enter(a)
	w.clickBody(a, w.planetBody(f.PlanetIdx))
	if w.psel != f.Planet {
		t.Fatalf("psel %v", w.psel)
	}
	guarded := func() bool {
		return slices.ContainsFunc(w.planetLines(a.e), func(l string) bool { return strings.HasPrefix(l, "guarded:") })
	}
	body := func(sh *game.Starship) *body {
		for i := range w.bodies {
			if w.bodies[i].ship == sh {
				return &w.bodies[i]
			}
		}
		return nil
	}
	if n := len(w.ptiles()); n != 0 {
		t.Errorf("%d tiles on a cloaked orbit", n)
	}
	if !slices.Contains(w.planetLines(a.e), orbitCloakedLine) || guarded() {
		t.Errorf("lines %q", w.planetLines(a.e))
	}
	if _, ok := guardRing(a.g, a.e, f.Star, w.planetBody(f.PlanetIdx)); ok {
		t.Error("a guard ring on a cloaked orbit")
	}
	if _, ok := guardRing(a.g, nil, f.Star, w.planetBody(f.PlanetIdx)); !ok {
		t.Error("fixture: no guard ring without the fog")
	}
	if body(hidden) != nil {
		t.Error("the ship in the cloaked orbit is a body")
	}
	if b := body(seen); b == nil || b.seen == nil || !b.seen.Of(seen) {
		t.Errorf("the ship in space: %+v", b)
	}

	a.e.ScanAll = true
	w.rebuild()
	if n := len(w.ptiles()); n != len(homeDefences)+1 || !guarded() || body(hidden) == nil {
		t.Errorf("ScanAll: %d tiles, lines %q", n, w.planetLines(a.e))
	}
	a.e.ScanAll = false

	f.Planet.DemolishOrbital(9)
	w.rebuild()
	if n := len(w.ptiles()); n != len(homeDefences) || !guarded() || slices.Contains(w.planetLines(a.e), orbitCloakedLine) {
		t.Errorf("razed: %d tiles, lines %q", n, w.planetLines(a.e))
	}
	if b := body(hidden); b == nil || b.seen == nil || !b.seen.Of(hidden) {
		t.Errorf("razed: the ship in orbit %+v", b)
	}
}

// The planet screen: a cloaked orbit draws no structure nor foreign ship
// and says so in the POP panel; a cloaked surface leaves the species and
// the cloaks, no people, project, outputs nor square hover. The owner's
// screen shows everything.
func TestCloakedPlanetScreen(t *testing.T) {
	a, f, hidden, _ := cloakFixture(t)
	own := a.e.Spawn(hidden.Design, f.Star, f.PlanetIdx)
	s := newPlanetScreen(f.Star, f.PlanetIdx)
	s.Enter(a)
	if s.veiled || !s.orbitVeiled {
		t.Fatalf("veiled %v orbitVeiled %v", s.veiled, s.orbitVeiled)
	}
	if pop, _ := s.panelTexts(); !strings.HasPrefix(pop, "POP ") || !strings.HasSuffix(pop, "\n"+orbitCloakedLine) {
		t.Errorf("POP panel %q", pop)
	}
	if slices.ContainsFunc(s.orbitShips(a), func(o orbitShip) bool { return o.own == nil }) {
		t.Error("a foreign ship drawn in a cloaked orbit")
	}
	// The player's ship takes the first slot: the structures' are unseen.
	if os := s.orbitShips(a); len(os) != 1 || os[0].own != own || os[0].slot != 0 || f.Planet.Orbit[0] == game.NoItem {
		t.Errorf("our ship in the cloaked orbit: %+v", os)
	}

	var c game.Cell // a structure's square: hovered before the veil
	for _, cc := range f.Planet.Cells() {
		if f.Planet.Surface[cc].Item != game.NoItem {
			c = cc
			break
		}
	}
	x, y := s.grid.Centre(c)
	if s.hoverAt(a, x, y); !s.cellOK || s.cell != c {
		t.Fatalf("fixture: no hover on %v", c)
	}
	var free []game.Cell
	for _, cc := range f.Planet.Cells() {
		if f.Planet.Surface[cc].Item == game.NoItem {
			free = append(free, cc)
		}
	}
	f.Planet.Surface[free[0]].Ruins = true
	if !ruinsSeen(a.e, f.Star, f.Planet) {
		t.Fatal("fixture: the ruins unseen")
	}
	f.Planet.Surface[free[1]].Item = game.SurfaceCloaker
	if ruinsSeen(a.e, f.Star, f.Planet) {
		t.Error("ruins seen on a cloaked surface")
	}
	s.refresh(a)
	pop, pp := s.panelTexts()
	if want := game.Orfa.String() + "\n" + surfaceCloakedLine + "\n" + orbitCloakedLine; !s.veiled || pop != want {
		t.Errorf("POP panel %q, want %q", pop, want)
	}
	if pp.col != nil || pp.name != "" || s.colony() != nil || outputValues(s.colony()) != [3]int{} {
		t.Errorf("veiled: project panel %+v, colony %p", pp, s.colony())
	}
	if b, fr, r := s.popCounts(); b+fr+r != 0 {
		t.Errorf("veiled people %d %d %d", b, fr, r)
	}
	if s.hoverAt(a, x, y); s.cellOK {
		t.Error("a veiled square hovered")
	}
	s.Text(a, ebiten.NewImage(screenW, screenH))

	// The owner sees through its cloakers.
	b := testApp(a.g)
	b.player, b.e = 1, a.g.Empires[1]
	o := newPlanetScreen(f.Star, f.PlanetIdx)
	o.Enter(b)
	if o.veiled || o.orbitVeiled || o.colony() != f {
		t.Errorf("the owner: veiled %v %v", o.veiled, o.orbitVeiled)
	}

	// Drawn with the game's art: no structure looked up on a veiled screen.
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a.a = as
	sc := ebiten.NewImage(screenW, screenH)
	s.Render(a, sc)
	s.Text(a, sc)
}

// The round log on a cloaked orbit, the cloaker
// razed in the round (gone when the replay is laid): our shot through it
// onto the ship there and the one on the cloaker, its orbital weapon's
// shot at us, the ship destroyed, the cloaker razed, a special aimed at
// the ship there: each "something in <planet>'s orbit", no ship's flag,
// no name, hull nor structure; ours named. With ScanAll, the old names.
func TestCloakedRoundLog(t *testing.T) {
	a, f, hidden, seen := cloakFixture(t)
	pl := f.PlanetIdx
	gun := a.e.Spawn(hidden.Design, f.Star, pl)
	hidden.Design.Hull = game.Large // its name differs from the seen one's
	gun.Orbit = -1
	logs := []game.RoundLog{{Star: f.Star,
		Shots: []game.Fired{
			{Shooter: gun, Gizmo: game.ProtonShaver, AtPlanet: true, TargetPlanet: pl, Target: hidden, TargetEmpire: 1, Damage: 3},
			{Shooter: gun, Gizmo: game.ProtonShaver, AtPlanet: true, TargetPlanet: pl, HitSlot: 9, HitItem: game.OrbitalCloaker, TargetEmpire: 1, Damage: 5},
			{FromPlanet: true, ShooterPlanet: pl, ShooterItem: f.Planet.Orbit[0], ShooterEmpire: 1, Target: gun, Damage: 2},
		},
		Destroyed: []*game.Starship{hidden},
		Razed:     []game.Razing{{Planet: pl, Slot: 9, Item: game.OrbitalCloaker}},
	}}
	use := &game.SpecialUse{User: seen, UserEmpire: 1, Gizmo: game.GizmoTractorBeam, Star: f.Star, Kind: game.ShipTarget,
		Target: hidden, TargetEmpire: 1, Outcome: "pulled"}
	lay := func() (*systemScreen, []string, [][]int) {
		w := newSystemScreen(f.Star)
		w.Enter(a)
		cast := w.replayCast() // before the round, the cloaker standing
		f.Planet.Orbit[9] = game.NoItem
		defer func() { f.Planet.Orbit[9] = game.OrbitalCloaker }()
		w.startReplay(a.e, cast, logs, []*game.SpecialUse{use})
		var txt []string
		var fl [][]int
		for _, l := range w.rlog.lines {
			s, f := lineText(l)
			txt, fl = append(txt, s), append(fl, f)
		}
		return w, txt, fl
	}
	w, txt, fl := lay()
	hid := w.hiddenIn(pl)
	named := []string{w.logName(w.replayCast(), hidden, 1), game.OrbitalCloaker.String(), f.Planet.Orbit[0].String()}
	n := 0
	for i, l := range txt {
		for _, x := range named {
			if strings.Contains(l, x) {
				t.Errorf("line %q names %q", l, x)
			}
		}
		if strings.Contains(l, "in "+f.Planet.Name+"'s orbit") { // hiddenIn, or the razing's
			n++
			if strings.Contains(l, " → "+hid) && fl[i][len(fl[i])-1] != -1 || strings.HasSuffix(l, hid+" destroyed") && fl[i][0] != -1 {
				t.Errorf("line %q flags %v", l, fl[i])
			}
		}
	}
	if n != 6 || !slices.ContainsFunc(txt, func(l string) bool { return strings.Contains(l, gun.Name) }) {
		t.Errorf("%d hidden lines, ours named: %q", n, txt)
	}

	a.e.ScanAll = true
	_, txt, _ = lay()
	a.e.ScanAll = false
	if all := strings.Join(txt, "\n"); strings.Contains(all, hid) || !strings.Contains(all, named[1]) || !strings.Contains(all, named[2]) {
		t.Errorf("ScanAll: %q", txt)
	}
}
