package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// fogGame is two empires on seed 1, the player's home, empire 1's home,
// and a third star neither empire knows yet.
func fogGame(t *testing.T) (g *game.Game, e, o *game.Empire, other int) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e, o = g.Empires[0], g.Empires[1]
	other = -1
	for i := range g.Cluster.Stars {
		if !e.Knows(i) && !o.Knows(i) && len(g.Cluster.Stars[i].System.Planets) > 0 {
			other = i
			break
		}
	}
	if other < 0 {
		t.Fatal("no star unknown to both empires with a planet")
	}
	return g, e, o, other
}

// A foreign ship is drawn where the player Knows the star, present or
// not, from its Sighting (hull, place); at a star the player does not
// know it is not; nor while it travels a lane out of a known one.
func TestSightedBodies(t *testing.T) {
	g, e, o, star := fogGame(t)
	away := o.Spawn(fixtureShip, star, 0)      // first: the pairing must skip it
	away.Lane = g.Cluster.Stars[star].Lanes[0] // left by its first lane
	theirs := o.Spawn(foeShip, star, 0)
	a := testApp(g)
	v := newSystemScreen(star)
	v.Enter(a)
	if e.Knows(star) || shipOf(v, theirs) != nil || shipOf(v, away) != nil {
		t.Errorf("unknown star %d: theirs listed %v, the one in a lane %v", star, shipOf(v, theirs), shipOf(v, away))
	}
	e.Visit(star)
	v.rebuild()
	if e.Present(star) {
		t.Fatal("the fixture made the player present")
	}
	seen := g.ShipsSeen(e, star)
	b := shipOf(v, theirs)
	if len(seen) != 1 || b == nil || b.seen == nil {
		t.Fatalf("known star: %d sightings, body %v", len(seen), b)
	}
	if _, f := b.sprite(); *b.seen != seen[0] || f != int(seen[0].Hull) || b.pos != v.sys.Planets[0].Pos || !orbitHidden(b) || b.owner != seen[0].Empire {
		t.Errorf("sighted body %+v (frame %d), want sighting %+v hidden in planet 0's orbit", b, f, seen[0])
	}
	if shipOf(v, away) != nil {
		t.Error("a foreign ship in a lane is drawn: it is the starmap's pennant")
	}
	// In space: drawn at the Sighting's Pos.
	theirs.Orbit, theirs.Pos = -1, game.Vec3{X: 0.3, Y: 0.4, Z: -0.5}
	v.rebuild()
	if b := shipOf(v, theirs); b == nil || b.pos != theirs.Pos || b.pos != g.ShipsSeen(e, star)[0].Pos {
		t.Errorf("in space: body %v, want at %v", b, theirs.Pos)
	}
}

// foreignOf is sh's foreign panel as v's player reads it now.
func foreignOf(t *testing.T, v *systemScreen, sh *game.Starship) foreignView {
	t.Helper()
	sg, ok := v.g.SightingOf(v.e, sh)
	if !ok {
		t.Fatalf("%s not sighted", sh.Name)
	}
	return foreignLines(v.e, v.g, sg)
}

// The foreign panel at the three scan levels: unscanned the owner, hull
// size and where (never our hits), no bars, no tiles; a scanner the design's
// name and shields, the hull and energy bars (the energy's maximum
// hidden) and the cargo; X Ray Megaglasses the energy's maximum and the
// parts' tiles, each "xN" the parts aboard whatever the ship spent.
// Never the ship's name.
func TestForeignPanel(t *testing.T) {
	g, e, o, _ := fogGame(t)
	home := e.Colonies[0].Star
	theirs := o.Spawn(foeShip, home, 0)
	theirs.Hull--
	theirs.Power = theirs.Design.PowerPerDay() / 2
	theirs.Cargo = 2
	theirs.Dealt = []int{3}
	a := testApp(g)
	v := newSystemScreen(home)
	v.Enter(a)
	if v.act(e, shipOf(v, theirs)); v.fsel != theirs || v.box() != boxForeign {
		t.Fatalf("click: fsel %v, box %d", v.fsel, v.box())
	}
	at := "orbit of " + v.sys.Planets[0].Name
	title := o.Species.String() + " " + strings.ToLower(theirs.Design.Hull.String()) + " ship" // the colour: the flag drawn before it
	lines := func(fv foreignView) []string {
		return append(slices.Clone(fv.where), fv.note)
	}
	// row is the design line: its text and the shields' state shown.
	row := func() (string, shieldState) {
		sg, _ := v.foreignSeen()
		l := v.foreignShields(sg)
		if sg.Scan == nil {
			return l.head + l.tail, shieldState{}
		}
		return l.head + l.tail, scanShieldState(sg.Scan)
	}
	check := func(level string, fv foreignView) {
		t.Helper()
		if fv.title != title || len(fv.title) > infoCols {
			t.Errorf("%s: title %q, want %q (at most %d)", level, fv.title, title, infoCols)
		}
		for _, l := range append(lines(fv), fv.title) {
			if strings.Contains(l, theirs.Name) || len(l) > footCols {
				t.Errorf("%s: line %q names the ship or runs over %d", level, l, footCols)
			}
		}
	}

	fv := foreignOf(t, v, theirs)
	check("unscanned", fv)
	if want := []string{at, "not scanned"}; !slices.Equal(lines(fv), want) || fv.bars != nil || v.foreignTiles() != nil {
		t.Errorf("unscanned: %q, %d bars, %d tiles; want %q", lines(fv), len(fv.bars), len(v.foreignTiles()), want)
	}
	if l, st := row(); l != "" || st.shown {
		t.Errorf("unscanned design line %q %+v, want none (the hull is in the title)", l, st)
	}

	// A scanner without X Ray Megaglasses reads the hull, not the parts.
	scanner := game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.TonklinFrequencyAnalyzer}}
	e.Spawn(scanner, home, 0)
	v.rebuild()
	if sc := g.ShipsSeen(e, home)[0].Scan; sc == nil || sc.Parts {
		t.Fatalf("the plain scanner reads %+v, want a Scan without Parts", sc)
	}
	fv = foreignOf(t, v, theirs)
	check("scanned", fv)
	want := []string{fmt.Sprintf("%s, cargo %d", at, theirs.Cargo), "parts unknown (X Ray Megaglasses)"}
	if !theirs.Shields || theirs.ShieldsDown || theirs.ShieldHits() < 2 {
		t.Fatalf("fixture: shields %v down %v, %d hits", theirs.Shields, theirs.ShieldsDown, theirs.ShieldHits())
	}
	if l, st := row(); l != theirs.Design.Name+"  " || st != (shieldState{shown: true, lit: true, hits: 1, absorb: -1}) {
		t.Errorf("scanned design line %q %+v: want the design and one lit icon, no number", l, st)
	}
	ours := shipBars(theirs) // the same gauges as our panel's
	en := ours[1]
	en.num = fmt.Sprint(theirs.Power)
	if !slices.Equal(lines(fv), want) || !slices.Equal(fv.bars, []bar{ours[0], en}) || v.foreignTiles() != nil {
		t.Errorf("scanned: %q, bars %+v, %d tiles; want %q, %+v", lines(fv), fv.bars, len(v.foreignTiles()), want, []bar{ours[0], en})
	}
	if strings.Contains(fv.bars[1].num, "/") {
		t.Errorf("energy %q shows the maximum without the parts", fv.bars[1].num)
	}

	// X Ray Megaglasses: the energy's maximum and the tiles.
	scanner.Parts = append(slices.Clone(scanner.Parts), game.XRayMegaglasses)
	e.Spawn(scanner, home, 0)
	v.rebuild()
	if sc := g.ShipsSeen(e, home)[0].Scan; sc == nil || !sc.Parts {
		t.Fatalf("the X Ray scanner reads %+v", sc)
	}
	fv = foreignOf(t, v, theirs)
	check("x-rayed", fv)
	if l, st := row(); st != shipShields(theirs) || l != fmt.Sprintf("%s   %d", theirs.Design.Name, theirs.Absorb()) {
		t.Errorf("x-rayed design line %q %+v, want ours' row %+v", l, st, shipShields(theirs))
	}
	if want := want[:1]; !slices.Equal(fv.where, want) || fv.note != "" || !slices.Equal(fv.bars, ours[:]) {
		t.Errorf("x-rayed: %q, note %q, bars %+v; want %q, %+v", lines(fv), fv.note, fv.bars, want, ours)
	}
	ts := v.foreignTiles()
	if !slices.Equal(ts, panelTiles(theirs)) || len(ts) != len(theirs.Design.Counts()) {
		t.Fatalf("tiles %+v, want our layout of its design %+v", ts, panelTiles(theirs))
	}
	theirs.Shots = make([]int, len(theirs.Design.Parts)) // every use spent: the counts stay the parts aboard
	for k, p := range theirs.Design.Parts {
		theirs.Shots[k] = game.Gizmos[p].Uses
	}
	for _, tl := range v.foreignTiles() {
		n := 0
		for _, p := range theirs.Design.Parts {
			if p == tl.g {
				n++
			}
		}
		l := foreignTileText(tl, theirs.Shields)
		if tl.n != n || !strings.HasPrefix(l, fmt.Sprintf("%s x%d", game.Gizmos[tl.g].Name, n)) {
			t.Errorf("tile %s: n %d, text %q; want x%d", game.Gizmos[tl.g].Name, tl.n, l, n)
		}
		if on := strings.Contains(l, ": "+onOff(theirs.Shields)); on != (tl.band == shieldBand) {
			t.Errorf("tile %s: text %q, the shields' state %v", game.Gizmos[tl.g].Name, l, on)
		}
	}
}

// The starmap's pennants: in a lane, the player's own ships wherever,
// the others' on the lanes it knows, empire by empire; at a star, the
// player first then each empire it sees there, once each.
func TestPennants(t *testing.T) {
	g, e, o, star := fogGame(t)
	cl := g.Cluster
	home := e.Colonies[0].Star
	known := cl.Stars[home].Lanes[0]
	unknown := -1
	for _, li := range cl.Stars[star].Lanes {
		if !e.KnowsLane(li) {
			unknown = li
		}
	}
	if unknown < 0 {
		t.Fatal("no lane unknown to the player")
	}
	for range 2 {
		o.Spawn(foeShip, home, 0).Lane = known
	}
	o.Spawn(foeShip, star, 0).Lane = unknown
	e.Spawn(fixtureShip, home, 0).Lane = known
	want := []game.LaneSighting{{Empire: e.Index, Lane: known, From: home}, {Empire: o.Index, Lane: known, From: home}, {Empire: o.Index, Lane: known, From: home}}
	got := laneShips(g, e)
	if len(got) != len(want) {
		t.Fatalf("lane ships %+v, want %+v", got, want)
	}
	for k, ls := range got {
		w := want[k]
		if ls.Empire != w.Empire || ls.Lane != w.Lane || ls.From != w.From || ls.Frac < 0 || ls.Frac > 1 {
			t.Errorf("lane ship %d: %+v, want %+v", k, ls, w)
		}
	}
	for range 2 {
		o.Spawn(foeShip, home, 0)
	}
	docked := e.Spawn(fixtureShip, home, 0)
	ps := starPennants(g, e)
	if fmt.Sprint(ps[home]) != fmt.Sprint([]pennant{{e.Index, false}, {o.Index, false}}) || len(ps[star]) != 0 {
		t.Errorf("home's pennants %v, want [%d %d]; the unknown star's %v", ps[home], e.Index, o.Index, ps[star])
	}
	// Being refitted, as being built: a ghost where no other ship stands.
	for _, sh := range e.Ships {
		if sh != docked && sh.Star == home && sh.Lane < 0 {
			t.Fatalf("another ship of the player's at home: %s", sh.Name)
		}
	}
	col := e.Colonies[0]
	col.Project = &game.Project{Item: game.OrbitalDocks, Refit: docked.Name}
	ghost := fmt.Sprint([]pennant{{e.Index, true}, {o.Index, false}})
	if ps := starPennants(g, e); fmt.Sprint(ps[home]) != ghost {
		t.Errorf("home's pennants with the ship refitted %v, want %s", ps[home], ghost)
	}
	col.Project = &game.Project{Item: game.Ship}
	e.Ships = slices.DeleteFunc(e.Ships, func(sh *game.Starship) bool { return sh == docked })
	if ps := starPennants(g, e); fmt.Sprint(ps[home]) != ghost {
		t.Errorf("home's pennants with a ship being built %v, want %s", ps[home], ghost)
	}
	e.Spawn(fixtureShip, home, 0)
	if ps := starPennants(g, e); fmt.Sprint(ps[home]) != fmt.Sprint([]pennant{{e.Index, false}, {o.Index, false}}) {
		t.Errorf("home's pennants with a ship being built and one standing %v, want solid", ps[home])
	}
	if laneShips(nil, nil) != nil || laneShips(g, nil) != nil || starPennants(nil, nil) != nil {
		t.Error("pennants without a game or an empire")
	}
}
