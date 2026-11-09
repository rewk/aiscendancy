package game

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// Invasions (invade.go): the refusals, the shields' arithmetic, the
// capture, the same-day order.

// invader is a design carrying n Invasion Modules (Spawn loads them).
func invader(n int) Design {
	parts := []Gizmo{ProtonShaver, TonklinMotor}
	for range n {
		parts = append(parts, InvasionModule)
	}
	return Design{Name: "Invader", Hull: Enormous, Parts: parts}
}

// invadeFixture is a game of species (empire 0 the invader, empire 1
// the defender) and empire 1's home colony, its planet's shields
// cleared (a home world rolls none, but the fixture must not depend on
// it); every pair at War.
func invadeFixture(t *testing.T, species ...Species) (*Game, *Colony) {
	t.Helper()
	g := NewGame(1, species, Average)
	allAtWar(t, g)
	col := g.Empires[1].Colonies[0]
	for _, sq := range col.Planet.Surface {
		if _, ok := ShieldWorth[sq.Item]; ok {
			sq.Item = NoItem
		}
	}
	return g, col
}

// placeShields puts its on the first empty squares of p in cell order,
// returning their cells.
func placeShields(t *testing.T, p *Planet, its ...Item) []Cell {
	t.Helper()
	var at []Cell
	for _, c := range p.Cells() {
		if len(at) == len(its) {
			break
		}
		if sq := p.Surface[c]; sq.Item == NoItem {
			sq.Item = its[len(at)]
			at = append(at, c)
		}
	}
	if len(at) < len(its) {
		t.Fatalf("%s: room for %d shields of %d", p.Name, len(at), len(its))
	}
	return at
}

func TestInvadeRefusals(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa, Mebes)
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	star := col.Star
	inv := func() *Starship { return e0.Spawn(invader(2), star, col.PlanetIdx) }
	home0, free := e0.Colonies[0].Star, -1 // empire 1's home star has no free planet (seed 1)
	for i, p := range g.Cluster.Stars[home0].System.Planets {
		if p.Owner < 0 {
			free = i
			break
		}
	}
	if free < 0 {
		t.Fatal("no free planet at empire 0's home: the fixture proves nothing")
	}
	for _, c := range []struct {
		name  string
		setup func() *Starship
		want  string
	}{
		{"in space", func() *Starship { s := inv(); inSpaceAt(s, s.Pos); return s }, "not in orbit"},
		{"no module", func() *Starship { s := inv(); s.Invaders = 0; return s }, "no invasion module"},
		{"no module aboard", func() *Starship { return e0.Spawn(laneShip, star, col.PlanetIdx) }, "no invasion module"},
		{"unowned", func() *Starship { return e0.Spawn(invader(2), home0, free) }, "is not inhabited"},
		{"own planet", func() *Starship { return e1.Spawn(invader(2), star, col.PlanetIdx) }, "is ours"},
		{"owner's ship in orbit", func() *Starship { e1.Spawn(laneShip, star, col.PlanetIdx); return inv() }, "guarded by the Orfa"},
		{"a third empire's ship in orbit", func() *Starship { e2.Spawn(laneShip, star, col.PlanetIdx); return inv() }, "guarded by the Mebes"},
		{"orbital shield", func() *Starship {
			if err := col.Planet.BuildOrbital(OrbitSlots-1, OrbitalShield); err != nil {
				t.Fatal(err)
			}
			return inv()
		}, "guarded by the Orfa"},
		{"orbital mega shield", func() *Starship {
			if err := col.Planet.BuildOrbital(OrbitSlots-1, OrbitalMegaShield); err != nil {
				t.Fatal(err)
			}
			return inv()
		}, "guarded by the Orfa"},
	} {
		for _, e := range g.Empires {
			e.Ships = nil
		}
		col.Planet.DemolishOrbital(OrbitSlots - 1)
		s := c.setup()
		err := g.Empires[slices.IndexFunc(g.Empires, func(e *Empire) bool { return slices.Contains(e.Ships, s) })].Order(s, Order{Kind: Invade})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		if s.Order.Kind != NoOrder {
			t.Errorf("%s: order %v kept", c.name, s.Order)
		}
	}
	// The invader's own ships in orbit are no guard, nor a non-shield orbital.
	for _, e := range g.Empires {
		e.Ships = nil
	}
	col.Planet.DemolishOrbital(OrbitSlots - 1)
	if err := col.Planet.BuildOrbital(OrbitSlots-1, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	e0.Spawn(laneShip, star, col.PlanetIdx)
	if s := inv(); e0.Order(s, Order{Kind: Invade}) != nil || s.Order.Kind != Invade {
		t.Errorf("own ship in orbit, a Whopper: refused (%v)", s.Order)
	}
	// No engine needed: the order moves nothing.
	pod := e0.Spawn(Design{Hull: Small, Parts: []Gizmo{InvasionModule}}, star, col.PlanetIdx)
	if err := e0.Order(pod, Order{Kind: Invade}); err != nil {
		t.Errorf("an engineless pod: %v", err)
	}
}

func TestUsableInvade(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	s := g.Empires[0].Spawn(invader(2), col.Star, col.PlanetIdx)
	part := slices.Index(s.Design.Parts, InvasionModule)
	if ok, say := s.Usable(part); !ok || say != "invade" {
		t.Errorf("in orbit: %v %q", ok, say)
	}
	s.Invaders = 0
	if ok, say := s.Usable(part); ok || say != "no invasion module left" {
		t.Errorf("none left: %v %q", ok, say)
	}
	s.Invaders = 1
	inSpaceAt(s, s.Pos)
	if ok, say := s.Usable(part); ok || say != "not in orbit" {
		t.Errorf("in space: %v %q", ok, say)
	}
}

// The rule's table (the worked examples first): n modules against the
// shields laid in cell order; win, and which shields survive (in cell
// order). A Surface Shield is worth one module, a Mega 2.5; a win
// spends the fewest modules that beat the shields (one for the Minions),
// a defeat all of them.
func TestInvadeShields(t *testing.T) {
	S, M := SurfaceShield, SurfaceMegaShield
	for _, c := range []struct {
		name     string
		sp       Species
		n        int
		shields  []Item
		win      bool
		survives []Item
		kept     int // modules left aboard
	}{
		{"3 vs 2", Snovemdomas, 3, []Item{S, S}, true, nil, 0},
		{"3 vs 5", Snovemdomas, 3, []Item{S, S, S, S, S}, false, []Item{S, S}, 0},
		{"2 vs 2", Snovemdomas, 2, []Item{S, S}, false, nil, 0},
		{"2 vs a mega", Snovemdomas, 2, []Item{M}, false, []Item{M}, 0},
		{"3 vs a mega", Snovemdomas, 3, []Item{M}, true, nil, 0},
		{"4 vs a mega: 3 launched", Snovemdomas, 4, []Item{M}, true, nil, 1},
		{"1 vs none", Snovemdomas, 1, nil, true, nil, 0},
		{"6 vs none: 1 launched", Snovemdomas, 6, nil, true, nil, 5},
		{"6 vs 1: 2 launched", Snovemdomas, 6, []Item{S}, true, nil, 4},
		{"2 vs mega, shield: the shield after the mega falls", Snovemdomas, 2, []Item{M, S}, false, []Item{M}, 0},
		{"3 vs shield, mega", Snovemdomas, 3, []Item{S, M}, false, []Item{M}, 0},
		{"4 vs shield, mega", Snovemdomas, 4, []Item{S, M}, true, nil, 0},
		{"5 vs 2 megas", Snovemdomas, 5, []Item{M, M}, false, nil, 0},
		{"6 vs 2 megas", Snovemdomas, 6, []Item{M, M}, true, nil, 0},
		{"Minions, 1 vs 2 megas and a shield", Minions, 1, []Item{M, M, S}, true, nil, 0},
		{"Minions, 1 vs 1", Minions, 1, []Item{S}, true, nil, 0},
		{"Minions, 3 vs 2 megas: 1 launched", Minions, 3, []Item{M, M}, true, nil, 2},
	} {
		g, col := invadeFixture(t, c.sp, Orfa)
		e0, e1 := g.Empires[0], g.Empires[1]
		p := col.Planet
		cells := placeShields(t, p, c.shields...)
		total := 0
		for _, it := range c.shields {
			total += ShieldWorth[it]
		}
		if want := c.sp.Traits().InvadesAlways() || c.n*ModuleWorth > total; want != c.win {
			t.Fatalf("%s: the table says win %v, the rule %v", c.name, c.win, want)
		}
		s := e0.Spawn(invader(c.n), col.Star, col.PlanetIdx)
		if s.Invaders != c.n {
			t.Fatalf("%s: spawned with %d modules", c.name, s.Invaders)
		}
		reps, err := g.Invade(e0, s)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if s.Invaders != c.kept {
			t.Errorf("%s: %d modules left, want %d", c.name, s.Invaders, c.kept)
		}
		var left []Item
		for _, at := range cells {
			if it := p.Surface[at].Item; it != NoItem {
				left = append(left, it)
			}
		}
		if !slices.Equal(left, c.survives) {
			t.Errorf("%s: shields left %v, want %v", c.name, left, c.survives)
		}
		won := p.Owner == 0 && slices.Contains(e0.Colonies, col) && !slices.Contains(e1.Colonies, col)
		if won != c.win || len(reps) != 2 {
			t.Errorf("%s: won %v (owner %d), want %v; %d reports", c.name, won, p.Owner, c.win, len(reps))
			continue
		}
		kind := Repelled
		if c.win {
			kind = Invaded
		}
		if reps[0].Empire != 0 || reps[1].Empire != 1 || reps[0].Event.Kind != kind || reps[1].Event.Kind != kind {
			t.Errorf("%s: reports %+v, want %v for 0 then 1", c.name, reps, kind)
		}
		if !c.win {
			want := fmt.Sprintf("%s repelled from %s", c.sp, p.Name)
			if reps[1].Event.Text != want {
				t.Errorf("%s: defender %q, want %q", c.name, reps[1].Event.Text, want)
			}
		}
	}
}

// A capture moves the colony as it is: owner, Colonies of both sides,
// population, growth and structures unchanged, the captor its owner, the
// project lost, the orbital weapons' day cleared; both sides told with
// the original's texts. The loser's last colony goes: the game plays on.
func TestInvadeCapture(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	e0, e1 := g.Empires[0], g.Empires[1]
	p := col.Planet
	if err := p.BuildOrbital(1, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	col.OrbitShots[1] = 2
	col.Project = &Project{Item: Factory, Cell: p.Cells()[0], Slot: -1}
	col.Pop, col.Growth = 7, 3
	shields := placeShields(t, p, SurfaceShield, SurfaceShield)
	surface := map[Cell]Item{}
	for c, sq := range p.Surface {
		surface[c] = sq.Item
	}
	for _, c := range shields {
		surface[c] = NoItem
	}
	orbit := p.Orbit
	s := e0.Spawn(invader(3), col.Star, col.PlanetIdx)
	if err := e0.Order(s, Order{Kind: Invade}); err != nil {
		t.Fatal(err)
	}
	evs := g.EndDay()
	if p.Owner != 0 || e0.Colony(col.Star, col.PlanetIdx) != col || len(e1.Colonies) != 0 || len(e0.Colonies) != 2 {
		t.Fatalf("owner %d, e0 colonies %d, e1 %d", p.Owner, len(e0.Colonies), len(e1.Colonies))
	}
	if col.owner != e0 || col.Pop != 7 || col.Project != nil || col.OrbitShots != ([OrbitSlots]int{}) || col.Armed != nil {
		t.Errorf("colony after: owner %v pop %d project %v shots %v", col.owner.Index, col.Pop, col.Project, col.OrbitShots)
	}
	got := map[Cell]Item{}
	for c, sq := range p.Surface {
		got[c] = sq.Item
	}
	if !maps.Equal(got, surface) || p.Orbit != orbit {
		t.Errorf("structures changed: surface %v orbit %v", got, p.Orbit)
	}
	if s.Order.Kind != NoOrder || s.Invaders != 0 {
		t.Errorf("ship after: order %v, %d modules", s.Order, s.Invaders)
	}
	at := func(es []Event, k EventKind) []Event {
		return slices.DeleteFunc(slices.Clone(es), func(e Event) bool { return e.Kind != k })
	}
	mine, theirs := at(evs[0], Invaded), at(evs[1], Invaded)
	wantMine := p.Name + " captured"
	wantTheirs := p.Name + " invaded by Snovemdomas"
	if len(mine) != 1 || mine[0].Text != wantMine || mine[0].Star != col.Star || mine[0].Planet != col.PlanetIdx {
		t.Errorf("invader's news %+v, want %q", mine, wantMine)
	}
	if len(theirs) != 1 || theirs[0].Text != wantTheirs || theirs[0].Star != col.Star || theirs[0].Planet != col.PlanetIdx {
		t.Errorf("defender's news %+v, want %q", theirs, wantTheirs)
	}
	// The colony runs in its new empire's day, the colonyless one plays on.
	day := e0.Day
	for range 5 {
		g.CombatStars()
		g.EndDay()
	}
	if e0.Day != day+5 || e1.Day != day+5 || e1.ControlledSystems() != 0 {
		t.Errorf("days %d %d, e1 systems %d", e0.Day, e1.Day, e1.ControlledSystems())
	}
	if err := e1.CanBuildShip(col, laneShip); err == nil {
		t.Error("the colonyless empire may build a ship")
	}
}

// Orfas who capture a planet build on its black squares: the colony's
// Traits are its owner's.
func TestInvadeBuildsOnBlack(t *testing.T) {
	g, col := invadeFixture(t, Orfa, Snovemdomas)
	p := col.Planet
	black, ok := Cell{}, false
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq.Item == NoItem && !sq.Ruins && p.Reachable(c) {
			sq.Kind, black, ok = Black, c, true
			break
		}
	}
	if !ok {
		t.Fatal("no empty reachable square")
	}
	if col.squareErr(p, Factory, black) == nil {
		t.Fatal("the Snovemdomas build on black")
	}
	s := g.Empires[0].Spawn(invader(3), col.Star, col.PlanetIdx)
	if _, err := g.InvadeNow(g.Empires[0], s); err != nil {
		t.Fatal(err)
	}
	if err := col.squareErr(p, Factory, black); err != nil {
		t.Errorf("captured by the Orfa: a Factory on black: %v", err)
	}
}

// One invader alone, singular texts: 1 invader, 1 colony (the invader
// had none: a fixture taking its home away).
func TestInvadeTexts(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	e0 := g.Empires[0]
	s := e0.Spawn(invader(1), col.Star, col.PlanetIdx)
	e0.Colonies = nil
	col.OrbitShots[3], col.Armed = 1, []Shot{{Part: 3}} // mid-day: Invade clears them itself
	reps, err := g.Invade(e0, s)
	if err != nil {
		t.Fatal(err)
	}
	if col.OrbitShots != ([OrbitSlots]int{}) || col.Armed != nil {
		t.Errorf("captured mid-day: shots %v armed %v", col.OrbitShots, col.Armed)
	}
	want := col.Planet.Name + " captured"
	if reps[0].Event.Text != want {
		t.Errorf("%q, want %q", reps[0].Event.Text, want)
	}
	g2, col2 := invadeFixture(t, Snovemdomas, Orfa)
	placeShields(t, col2.Planet, SurfaceShield)
	r2, err := g2.Invade(g2.Empires[0], g2.Empires[0].Spawn(invader(1), col2.Star, col2.PlanetIdx))
	if err != nil {
		t.Fatal(err)
	}
	if want := "Invasion of " + col2.Planet.Name + " failed"; r2[0].Event.Text != want {
		t.Errorf("%q, want %q", r2[0].Event.Text, want)
	}
	if want := "Snovemdomas repelled from " + col2.Planet.Name; r2[1].Event.Text != want {
		t.Errorf("%q, want %q", r2[1].Event.Text, want)
	}
}

// Several invaders the same day, in Empires then Ships order: the first
// captures; the same empire's next is refused (its own planet now), its
// modules kept, its order cleared. Ships of two foreign empires in orbit
// guard the planet against each other: neither invades.
func TestInvadeSameDay(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa, Mebes)
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	a := e0.Spawn(invader(2), col.Star, col.PlanetIdx)
	b := e0.Spawn(invader(2), col.Star, col.PlanetIdx)
	for _, s := range []*Starship{a, b} {
		if err := e0.Order(s, Order{Kind: Invade}); err != nil {
			t.Fatal(err)
		}
	}
	evs := g.EndDay()
	if col.Planet.Owner != 0 || a.Invaders != 1 || b.Invaders != 2 || b.Order.Kind != NoOrder {
		t.Errorf("owner %d, modules %d %d, b's order %v", col.Planet.Owner, a.Invaders, b.Invaders, b.Order)
	}
	if n := len(slices.DeleteFunc(slices.Clone(evs[0]), func(e Event) bool { return e.Kind != Invaded })); n != 1 {
		t.Errorf("%d captures reported", n)
	}

	g, col = invadeFixture(t, Snovemdomas, Orfa, Mebes)
	e0, e1, e2 = g.Empires[0], g.Empires[1], g.Empires[2]
	a = e0.Spawn(invader(2), col.Star, col.PlanetIdx)
	c := e2.Spawn(invader(2), col.Star, col.PlanetIdx)
	a.Order, c.Order = Order{Kind: Invade}, Order{Kind: Invade} // an old state: Order refuses both
	g.EndDay()
	if col.Planet.Owner != 1 || !slices.Contains(e1.Colonies, col) || a.Invaders != 2 || c.Invaders != 2 ||
		a.Order.Kind != NoOrder || c.Order.Kind != NoOrder {
		t.Errorf("owner %d, modules %d %d, orders %v %v", col.Planet.Owner, a.Invaders, c.Invaders, a.Order, c.Order)
	}
}

// An Invade order neither moves the ship nor waits (ETA), and the lone
// Empire.EndDay leaves it to the Game.
func TestInvadeOrderStill(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	s := g.Empires[0].Spawn(invader(2), col.Star, col.PlanetIdx)
	s.Order = Order{Kind: Invade}
	if moved, arrived := s.Hop(g.Cluster, 0, nil, nil); moved || arrived || s.Order.Kind != Invade {
		t.Errorf("Hop: %v %v, order %v", moved, arrived, s.Order)
	}
	if _, ok := s.ETA(g.Cluster, 0); ok {
		t.Error("an Invade order has an ETA")
	}
	pos := s.Pos
	g.Empires[0].EndDay()
	if s.Pos != pos || s.Orbit != col.PlanetIdx || s.Order.Kind != Invade || col.Planet.Owner != 1 {
		t.Errorf("lone close: pos %v orbit %d order %v owner %d", s.Pos, s.Orbit, s.Order, col.Planet.Owner)
	}
}

// InvadeNow resolves at once, before any close: refused for another's
// ship or at peace (nothing spent), else captured with the day unmoved,
// the order cleared and an "invade" journal line.
func TestInvadeNow(t *testing.T) {
	g, col := invadeFixture(t, Snovemdomas, Orfa)
	e0, e1 := g.Empires[0], g.Empires[1]
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	s := e0.Spawn(invader(2), col.Star, col.PlanetIdx)
	if _, err := g.InvadeNow(e1, s); err == nil || s.Invaders != 2 {
		t.Errorf("another empire's ship: %v, %d modules", err, s.Invaders)
	}
	s.Order = Order{Kind: Invade}
	day := g.Day
	reps, err := g.InvadeNow(e0, s)
	if err != nil || len(reps) != 2 || reps[0].Event.Kind != Invaded {
		t.Fatalf("invade: %v, %v", reps, err)
	}
	if col.Planet.Owner != 0 || e0.Colony(col.Star, col.PlanetIdx) != col || g.Day != day || s.Order.Kind != NoOrder || s.Invaders != 1 {
		t.Errorf("owner %d, day %d (was %d), order %v, %d modules", col.Planet.Owner, g.Day, day, s.Order.Kind, s.Invaders)
	}
	if !strings.Contains(buf.String(), `"invade"`) {
		t.Errorf("journal %q", buf.String())
	}
}
