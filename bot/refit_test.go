package bot

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// refitFixture: fleeFixture (two empires on seed 1, every neighbour of
// empire 0's home known) with every tech known and the home holding a
// Shipyard (orbit slot 0) and, when docks, Orbital Docks (slot 1), a
// worker free for a project.
func refitFixture(t *testing.T, gn *Genome, docks bool) (g *game.Game, b *Bot, home *game.Colony, adj []int) {
	t.Helper()
	g, b, _, adj = fleeFixture(t, gn)
	b.E.Research = allKnown()
	home = b.E.Colonies[0]
	home.Planet.Orbit[0] = game.Shipyard
	if docks {
		home.Planet.Orbit[1] = game.OrbitalDocks
	}
	home.Pop = home.Busy() + 1
	return g, b, home, adj
}

// secondColony founds a colony of b's empire on the first foundable
// planet of its home star, a Shipyard in its orbit and a worker free.
func secondColony(t *testing.T, b *Bot, home *game.Colony) *game.Colony {
	t.Helper()
	for pl := range b.E.Cluster.Stars[home.Star].System.Planets {
		if !b.foundable(home.Star, pl) {
			continue
		}
		cs := b.E.Spawn(game.FixtureColonyShip, home.Star, pl)
		if err := b.E.Found(cs); err != nil {
			t.Fatal(err)
		}
		b.E.Ships = b.E.Ships[:len(b.E.Ships)-1]
		col := b.E.Colonies[len(b.E.Colonies)-1]
		col.Planet.Orbit[0] = game.Shipyard
		col.Pop = col.Busy() + 1
		return col
	}
	t.Fatal("no foundable planet at home")
	return nil
}

// refitProject is a refit project of ship name to d, as Empire.Refit
// sets it.
func refitProject(name string, d game.Design) *game.Project {
	return &game.Project{Item: game.OrbitalDocks, Slot: -1, Orbital: true, Design: &d, Refit: name}
}

// A refit project is no ship under construction: wantWarship does not
// count its armed target, spare does not take its Colonizers off; a
// Ship project of the same design does both.
func TestRefitProjectNotAShip(t *testing.T) {
	g, b, home, _ := refitFixture(t, nil, true)
	b.E.Visit(g.Empires[1].Colonies[0].Star) // contact
	d := laneGunboat(t)
	b.E.Spawn(d, home.Star, home.PlanetIdx) // at war the war wants 2 (warTarget): one flies, one more is wanted
	b.assess()
	if !b.wantWarship(home) {
		t.Fatal("one warship of the two the war wants: none wanted, the fixture proves nothing")
	}
	home.Project = refitProject("Ghost 1", d)
	if !b.wantWarship(home) {
		t.Error("a refit project counted as a warship under construction")
	}
	home.Project = &game.Project{Item: game.Ship, Slot: 2, Orbital: true, Design: &d}
	if b.wantWarship(home) {
		t.Error("a Ship project of an armed design not counted")
	}
	cs := game.FixtureColonyShip
	home.Project = nil
	b.claimed()
	idle := b.spare
	home.Project = refitProject("Ghost 1", cs)
	if b.claimed(); b.spare != idle {
		t.Errorf("spare %d with a refit project, %d without", b.spare, idle)
	}
	home.Project = &game.Project{Item: game.Ship, Slot: 2, Orbital: true, Design: &cs}
	if b.claimed(); b.spare != idle-cs.Colonizers() {
		t.Errorf("spare %d with a colony ship building, want %d", b.spare, idle-cs.Colonizers())
	}
}

// The bot's one Orbital Docks: started at a shipyard colony with
// contact, never without, never while a colony holds Docks or builds
// them; a refit project elsewhere is no Docks.
func TestDocksBuilt(t *testing.T) {
	isDocks := func(col *game.Colony) bool {
		pj := col.Project
		return pj != nil && pj.Item == game.OrbitalDocks && pj.Refit == ""
	}
	for _, tc := range []struct {
		name    string
		contact bool
		other   func(o *game.Colony) // the second colony's state
		want    bool
	}{
		{"contact", true, func(o *game.Colony) {}, true},
		{"no contact", false, func(o *game.Colony) {}, false},
		{"docks standing elsewhere", true, func(o *game.Colony) { o.Planet.Orbit[1] = game.OrbitalDocks }, false},
		{"docks building elsewhere", true, func(o *game.Colony) {
			o.Project = &game.Project{Item: game.OrbitalDocks, Slot: 1, Orbital: true}
		}, false},
		{"a refit elsewhere", true, func(o *game.Colony) { o.Project = refitProject("Ghost 1", game.FixtureGunboat) }, true},
	} {
		g, b, home, _ := refitFixture(t, nil, false)
		other := secondColony(t, b, home)
		tc.other(other)
		if tc.contact {
			b.E.Visit(g.Empires[1].Colonies[0].Star)
		}
		b.assess()
		if b.contact != tc.contact {
			t.Fatalf("%s: contact %v", tc.name, b.contact)
		}
		b.planColony(1, home, b.claimed()) // i 1: past the home's script
		if got := isDocks(home); got != tc.want {
			t.Errorf("%s: docks %v, want %v (project %+v)", tc.name, got, tc.want, home.Project)
		}
	}
}

// wantRefit: an outdated ship wants today's warship on its hull once it
// deals (1 + RefitGain) × its damage, an up-to-date one wants nothing;
// below RepairAt × its full hull either wants a repair, the outdated one
// to today's design, the up-to-date one to its own.
func TestWantRefit(t *testing.T) {
	_, b, home, _ := refitFixture(t, nil, true)
	old := b.E.Spawn(game.FixtureGunboat, home.Star, home.PlanetIdx)
	now, ok := game.WarshipOn(b.E.Research, old.Design.Hull, false)
	if !ok || !armed(now) || sameParts(now.Parts, old.Design.Parts) {
		t.Fatalf("WarshipOn: %v ok %v", now, ok)
	}
	gain := float64(game.DesignDamage(now, 0))/float64(game.DesignDamage(old.Design, 0)) - 1
	if gain <= 0 {
		t.Fatalf("today's warship deals no more (gain %v): the fixture proves nothing", gain)
	}
	up := b.E.Spawn(now, home.Star, home.PlanetIdx)
	for _, tc := range []struct {
		name     string
		s        *game.Starship
		gain     float64
		repairAt float64
		hull     int // hull points; 0: full
		want     bool
		target   game.Design
	}{
		{"outdated", old, gain * 0.99, 0.5, 0, true, now},
		{"outdated, gain short", old, gain * 1.01, 0.5, 0, false, game.Design{}},
		{"up to date", up, 0, 0.5, 0, false, game.Design{}},
		{"up to date, at RepairAt", up, 0, 0.5, (up.MaxHull() + 1) / 2, false, game.Design{}},
		{"up to date, below RepairAt", up, 0, 0.5, (up.MaxHull()+1)/2 - 1, true, now},
		{"up to date, below RepairAt 1", up, 0, 1, up.MaxHull() - 1, true, now},
		{"outdated, gain short, below RepairAt", old, gain * 1.01, 0.5, old.MaxHull()/2 - 1, true, now},
		{"outdated, below RepairAt 0", old, gain * 1.01, 0, 1, false, game.Design{}},
	} {
		b.Cfg.Genome = genomeWith(func(g *Genome) { g.RefitGain, g.RepairAt = tc.gain, tc.repairAt })
		b.assess()
		tc.s.Hull = tc.s.MaxHull()
		if tc.hull > 0 {
			tc.s.Hull = tc.hull
		}
		d, ok := b.wantRefit(tc.s)
		if ok != tc.want || ok && !sameParts(d.Parts, tc.target.Parts) {
			t.Errorf("%s: want %v target %v, got %v %v", tc.name, tc.want, tc.target.Parts, ok, d.Parts)
		}
		if ok {
			if err := b.E.CanRefit(home, tc.s, d); err != nil {
				t.Errorf("%s: CanRefit refuses the wanted refit: %v", tc.name, err)
			}
		}
	}
	// A pure repair: the up-to-date ship's own design.
	b.Cfg.Genome = nil
	b.assess()
	up.Hull = 1
	if d, ok := b.wantRefit(up); !ok || d.Name != up.Design.Name || !sameParts(d.Parts, up.Design.Parts) {
		t.Errorf("pure repair: %v %v, want its own design", ok, d)
	}
	// Unarmed: never.
	cs := b.E.Spawn(game.FixtureColonyShip, home.Star, home.PlanetIdx)
	cs.Hull = 1
	if _, ok := b.wantRefit(cs); ok {
		t.Error("a colony ship wants a refit")
	}
}

// At RefitGain 0 (its clamp's floor) a design differing from today's
// warship by one non-weapon part of its category, dealing equal damage,
// wants no refit: the gain must be strict too.
func TestWantRefitEqualDamage(t *testing.T) {
	_, b, home, _ := refitFixture(t, nil, true)
	now, ok := game.WarshipOn(b.E.Research, game.FixtureGunboat.Hull, false)
	if !ok {
		t.Fatal("no warship")
	}
	var lesser game.Design
find:
	for i, p := range now.Parts {
		for g := range game.Gizmo(game.NumGizmos) {
			if c := game.Gizmos[p].Cat; c == game.Weapon || game.Gizmos[g].Cat != c || g == p {
				continue
			}
			d := now
			d.Parts = slices.Clone(now.Parts)
			d.Parts[i] = g
			if d.Valid(b.E.Research) == nil && armed(d) && !sameParts(d.Parts, now.Parts) &&
				game.DesignDamage(d, 0) == game.DesignDamage(now, 0) {
				lesser = d
				break find
			}
		}
	}
	if lesser.Parts == nil {
		t.Fatalf("no equal-damage variant of %v", now.Parts)
	}
	s := b.E.Spawn(lesser, home.Star, home.PlanetIdx)
	b.Cfg.Genome = genomeWith(func(g *Genome) { g.RefitGain, g.RepairAt = 0, 0.5 })
	b.assess()
	if d, ok := b.wantRefit(s); ok {
		t.Errorf("equal damage at RefitGain 0: refit to %v", d.Parts)
	}
}

// damaged spawns d for b's empire in orbit of home's planet, its hull
// one point: below any RepairAt but 0.
func damaged(b *Bot, d game.Design, home *game.Colony) *game.Starship {
	s := b.E.Spawn(d, home.Star, home.PlanetIdx)
	s.Hull = 1
	return s
}

// longRefits lifts refitMaxDays for the test: the fixtures' colonies
// make 3 industry a day, a refit there takes months.
func longRefits(t *testing.T) {
	old := refitMaxDays
	refitMaxDays = 1 << 20
	t.Cleanup(func() { refitMaxDays = old })
}

// startRefits runs before the colonies plan: a damaged ship in orbit of
// the idle Docks colony is refitted there, where planColony would have
// started a warship; a second waiting ship gets nothing (the first in
// Ships order wins the colony).
func TestStartRefitsBeforePlanning(t *testing.T) {
	longRefits(t)
	g, b, home, _ := refitFixture(t, genomeWith(func(g *Genome) { g.WarshipShare = 2 }), true)
	b.E.Visit(g.Empires[1].Colonies[0].Star) // contact: a warship wanted
	b.assess()
	if !b.wantWarship(home) {
		t.Fatal("no warship wanted: the fixture proves nothing")
	}
	s1 := damaged(b, laneGunboat(t), home)
	s2 := damaged(b, laneGunboat(t), home)
	b.Act()
	if pj := home.Project; pj == nil || pj.Refit != s1.Name {
		t.Fatalf("project %+v, want the refit of %s", pj, s1.Name)
	}
	if b.E.Refitting(s2) != nil || s2.Order.Kind != game.NoOrder || s2.Orbit != home.PlanetIdx {
		t.Errorf("second ship: refitting %v, order %+v, orbit %d: want holding in orbit", b.E.Refitting(s2), s2.Order, s2.Orbit)
	}
}

// A ship wanting a refit in orbit of a busy Docks colony holds there
// (attack would send it at the weak hostile next door, as it does the
// same ship at full hull); one being refitted holds too.
func TestRefitHolds(t *testing.T) {
	longRefits(t)
	g, b, home, adj := refitFixture(t, nil, true)
	home.Pop++ // the Factory keeps its worker: one more for the Docks
	if home.Pop > home.Capacity() {
		t.Fatalf("pop %d over room %d", home.Pop, home.Capacity())
	}
	e1 := g.Empires[1]
	foe := adj[1]
	e1.Spawn(game.FixtureColonyShip, foe, 0) // unarmed: an easy target
	if err := home.Start(b.E.Research, game.Factory, mustCell(t, home, game.Factory)); err != nil {
		t.Fatal(err)
	}
	s := damaged(b, laneGunboat(t), home)
	b.Act()
	if s.Order.Kind != game.NoOrder || s.Orbit != home.PlanetIdx || b.E.Refitting(s) != nil {
		t.Errorf("waiting: order %+v orbit %d, want holding in orbit", s.Order, s.Orbit)
	}
	s.Hull = s.MaxHull() // no refit wanted: attack decides
	b.Act()
	if s.Order.Kind != game.GoToStar || s.Order.Star != foe {
		t.Fatalf("at full hull: order %+v, want GoToStar %d (the fixture proves nothing)", s.Order, foe)
	}
	// Being refitted: it holds, no order, no dest.
	s.Order, s.Hull = game.Order{}, 1
	delete(b.dest, s)
	home.Cancel()
	if err := b.E.Refit(home, s, s.Design); err != nil {
		t.Fatal(err)
	}
	s.Hull = s.MaxHull() // whatever it wants now, it is held
	b.Act()
	if _, ok := b.dest[s]; s.Order.Kind != game.NoOrder || ok {
		t.Errorf("refitting: order %+v, dest %v", s.Order, ok)
	}
}

// mustCell is it's BestCell on col's planet.
func mustCell(t *testing.T, col *game.Colony, it game.Item) game.Cell {
	t.Helper()
	at, ok := col.Planet.BestCell(it, col.Traits())
	if !ok {
		t.Fatalf("no cell for %s", it)
	}
	return at
}

// A damaged lane warship next door heads for the Docks star (GoToStar,
// the star in dest); there, out of orbit, GoToPlanet the Docks planet;
// with no Docks at all it does what attack says (home: the nearest
// colony star).
func TestRefitRoute(t *testing.T) {
	longRefits(t)
	_, b, home, adj := refitFixture(t, nil, true)
	home.Pop++ // the project the first Act starts keeps its worker: one more for the Docks
	if home.Pop > home.Capacity() {
		t.Fatalf("pop %d over room %d", home.Pop, home.Capacity())
	}
	s := damaged(b, laneGunboat(t), home)
	s.Star, s.Orbit, s.Pos = adj[0], -1, game.Vec3{}
	b.Act()
	if s.Order.Kind != game.GoToStar || s.Order.Star != home.Star || b.dest[s] != home.Star {
		t.Errorf("next door: order %+v dest %d, want GoToStar %d", s.Order, b.dest[s], home.Star)
	}
	s.Star, s.Order = home.Star, game.Order{}
	b.Act()
	if s.Order.Kind != game.GoToPlanet || s.Order.Planet != home.PlanetIdx {
		t.Errorf("at the Docks star: order %+v, want GoToPlanet %d", s.Order, home.PlanetIdx)
	}
	// No Docks: refitRoute declines, the same ship plans as before.
	home.Planet.Orbit[1] = game.NoItem
	s.Order = game.Order{}
	if b.assess(); b.refitRoute(s) {
		t.Error("no Docks: refitRoute handled the ship")
	}
}

// A ship being refitted cannot get away (its orders are refused) nor
// fire (it is in orbit: Arm refuses): in a round it
// is held, left as it is, unarmed, unordered, under pressure above
// FleeRatio or not; Round does not touch it (a GoToPoint, which no
// refit start leaves, stays).
func TestRoundRefittingFights(t *testing.T) {
	g, b, home, _ := refitFixture(t, genomeWith(func(g *Genome) { g.FleeRatio = 0.05; g.AttackRatio = 0.05 }), true)
	s := damaged(b, laneGunboat(t), home)
	if err := b.E.Refit(home, s, s.Design); err != nil {
		t.Fatal(err)
	}
	foe := g.Empires[1].Spawn(game.FixtureRaider, home.Star, home.PlanetIdx)
	foe.Orbit = -1
	b.Round(home.Star)
	if len(s.Armed) != 0 || s.Order.Kind != game.NoOrder || s.Orbit != home.PlanetIdx {
		t.Errorf("refitting under pressure: armed %v, order %+v, orbit %d: want it held in orbit, unarmed", s.Armed, s.Order, s.Orbit)
	}
	b.Cfg.Genome = noFlight()
	foe.Pos = home.Planet.Pos // within range: a fight would clear a GoToPoint
	s.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: 0.5}}
	if b.Round(home.Star); len(s.Armed) != 0 || s.Order.Kind != game.GoToPoint {
		t.Errorf("refitting, no flight: armed %v, order %+v: want it untouched", s.Armed, s.Order)
	}
}

// The refit genes: Neutral and a nil genome play RefitGain 0.25,
// RepairAt 0.5; Clamp keeps RefitGain in [0, 4], RepairAt in [0, 1].
func TestRefitGenes(t *testing.T) {
	if n, c := Neutral(), NeutralCombat(); n.RefitGain != 0.25 || n.RepairAt != 0.5 || c.RefitGain != 0.25 || c.RepairAt != 0.5 {
		t.Errorf("Neutral %v/%v, NeutralCombat %v/%v", n.RefitGain, n.RepairAt, c.RefitGain, c.RepairAt)
	}
	for _, tc := range []struct{ in, gain, repair float64 }{
		{-1, 0, 0}, {0.3, 0.3, 0.3}, {2, 2, 1}, {9, 4, 1},
	} {
		g := Neutral()
		g.RefitGain, g.RepairAt = tc.in, tc.in
		g.Clamp()
		if g.RefitGain != tc.gain || g.RepairAt != tc.repair {
			t.Errorf("Clamp(%v): RefitGain %v, RepairAt %v", tc.in, g.RefitGain, g.RepairAt)
		}
	}
}

// A Docks colony that cannot refit within refitMaxDays (no free worker,
// too little industry for the refit, a long project running) neither
// holds a waiting ship in orbit nor draws one: attack decides (else
// warships park for hundreds of days).
func TestRefitNotDockable(t *testing.T) {
	g, b, home, adj := refitFixture(t, nil, true)
	g.Empires[1].Spawn(game.FixtureColonyShip, adj[1], 0) // an easy target
	s := damaged(b, laneGunboat(t), home)
	b.assess()
	d, ok := b.wantRefit(s)
	if !ok {
		t.Fatal("the fixture: no refit wanted")
	}
	ind, _, _ := home.Output()
	if days := (game.RefitCost(s, d) + ind - 1) / ind; days <= refitMaxDays {
		t.Fatalf("the fixture: the refit takes %d days, within %d", days, refitMaxDays)
	}
	b.Act()
	if b.E.Refitting(s) != nil || s.Order.Kind != game.GoToStar || s.Order.Star != adj[1] {
		t.Errorf("too long a refit: refitting %v, order %+v, want attack's GoToStar %d", b.E.Refitting(s) != nil, s.Order, adj[1])
	}
	longRefits(t)
	s.Order, s.Star, s.Orbit, s.Pos = game.Order{}, home.Star, home.PlanetIdx, home.Planet.Pos
	delete(b.dest, s)
	home.Cancel()           // the first Act's project: its worker would come back
	home.Pop = home.Busy()  // no free worker, none to come back
	b.Cfg.NoResearch = true // nothing else changes today
	b.Act()
	if b.E.Refitting(s) != nil || s.Order.Kind != game.GoToStar {
		t.Errorf("no free worker: refitting %v, order %+v, want attack's GoToStar", b.E.Refitting(s) != nil, s.Order)
	}
}

// A foreign ship in orbit of my Docks colony guards nothing (a colony is
// guarded by its owner alone): the Docks are not
// Barred to me and a ship wanting a refit next door still heads there.
func TestRefitRouteGuarded(t *testing.T) {
	longRefits(t)
	g, b, home, adj := refitFixture(t, nil, true)
	s := damaged(b, laneGunboat(t), home)
	s.Star, s.Orbit, s.Pos = adj[0], -1, game.Vec3{}
	g.Empires[1].Spawn(game.Design{Name: "Hulk", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver}}, home.Star, home.PlanetIdx)
	if b.E.Barred(home.Star, home.PlanetIdx) {
		t.Fatal("a foreign hulk bars my own colony")
	}
	if b.assess(); !b.refitRoute(s) || s.Order.Kind != game.GoToStar || s.Order.Star != home.Star {
		t.Errorf("Docks with a foreign hulk in orbit: order %+v, want GoToStar %d", s.Order, home.Star)
	}
}
