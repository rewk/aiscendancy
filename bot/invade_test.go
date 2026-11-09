package bot

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The bots' invasions (invade.go) and the colony bookkeeping across a
// capture (Note, Save, Resume).

// invadeFixture: two empires on seed 1 (Snovemdomas, Orfa), a bot each
// (gn), every tech and every star known to empire 0; target is empire
// 1's home colony, its surface shields cleared.
func invadeFixture(t *testing.T, gn *Genome) (g *game.Game, bots []*Bot, target *game.Colony) {
	t.Helper()
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	atWar(t, g, 0, 1)
	bots = NewAll(g, Config{NoResearch: true, Genome: gn})
	bots[0].E.Research = allKnown()
	target = g.Empires[1].Colonies[0]
	for _, sq := range target.Planet.Surface {
		if _, ok := game.ShieldWorth[sq.Item]; ok {
			sq.Item = game.NoItem
		}
	}
	for st := range g.Cluster.Stars {
		bots[0].E.Visit(st)
	}
	return g, bots, target
}

// invaderOf is the bot's invasion design for n modules, all techs known.
func invaderOf(t *testing.T, n int) game.Design {
	t.Helper()
	d, ok := game.InvasionShip(allKnown(), true, n)
	if !ok {
		t.Fatalf("no invasion ship of %d modules", n)
	}
	return d
}

// shield puts it on the first empty square of p.
func shield(t *testing.T, p *game.Planet, it game.Item) {
	t.Helper()
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq.Item == game.NoItem {
			sq.Item = it
			return
		}
	}
	t.Fatalf("%s: no empty square", p.Name)
}

// A capture: the winner's bot logs the colony as captured that day, not
// founded; the loser's drops it, no panic (Note once sliced Colonies
// from the count it had seen); Save/Resume keep both maps by colony.
func TestNoteCapture(t *testing.T) {
	g, bots, target := invadeFixture(t, nil)
	e0, e1 := g.Empires[0], g.Empires[1]
	Day(g, bots) // day 1: a founding is never on the day 0 of the homes
	// A second colony of the loser first, founded and noted, in a star
	// of nobody's (its own home has no free planet; a shared one would
	// leave it no system once its home is captured: defeated,
	// game/defeat.go): the capture takes its home, the first of its
	// Colonies.
	at, free := -1, -1
	for st, sr := range g.Cluster.Stars {
		if slices.ContainsFunc(sr.System.Planets, func(p *game.Planet) bool { return p.Owner >= 0 }) {
			continue
		}
		for pl, p := range sr.System.Planets {
			if _, ok := p.BestCell(game.ColonyBase, e1.Species.Traits()); ok {
				at, free = st, pl
				break
			}
		}
		if free >= 0 {
			break
		}
	}
	if free < 0 {
		t.Fatal("no free planet in a star of nobody's")
	}
	if err := e1.Found(e1.Spawn(game.FixtureColonyShip, at, free)); err != nil {
		t.Fatal(err)
	}
	e1.Ships = nil
	bots[1].Note()
	if len(e1.Colonies) != 2 || bots[1].Founded[e1.Colonies[1]] != e1.Day {
		t.Fatalf("fixture: %d colonies, founded %v", len(e1.Colonies), bots[1].Founded)
	}
	s := e0.Spawn(invaderOf(t, 2), target.Star, target.PlanetIdx)
	Day(g, bots)
	if target.Planet.Owner != 0 || s.Invaders != 1 { // no shields: one module lands
		t.Fatalf("no capture: owner %d, invaders %d", target.Planet.Owner, s.Invaders)
	}
	b0, b1 := bots[0], bots[1]
	if day, ok := b0.Captured[target]; !ok || day != g.Day {
		t.Errorf("winner: captured %v (day %d), want day %d", b0.Captured, day, g.Day)
	}
	if _, ok := b0.Founded[target]; ok {
		t.Error("winner: the captured colony is logged as founded")
	}
	if _, ok := b1.Founded[target]; ok || len(b1.Founded) != 1 || b1.Founded[e1.Colonies[0]] == 0 {
		t.Errorf("loser: founded %v, want only its second colony", b1.Founded)
	}
	// The loser's script no longer plays: its first colony was not one it
	// started with.
	if day, ok := b1.Founded[e1.Colonies[0]]; !ok || day == 0 {
		t.Errorf("loser's first colony founded %d %v", day, ok)
	}
	col := e1.Colonies[0]
	col.Project, col.Pop = nil, col.Busy()+1
	b1.Cfg.Script, b1.Cfg.Greedy, b1.next = []game.Item{game.Agriplot}, false, 0
	if b1.planColony(0, col, b1.claimed()); b1.next != 0 || col.Project != nil {
		t.Errorf("the home's script played on the loser's first colony: next %d, project %+v", b1.next, col.Project)
	}
	for i, b := range bots {
		r := Resume(b.E, b.Save())
		if !reflect.DeepEqual(r.Founded, b.Founded) || !reflect.DeepEqual(r.Captured, b.Captured) {
			t.Errorf("empire %d resumed: founded %v captured %v, want %v %v", i, r.Founded, r.Captured, b.Founded, b.Captured)
		}
		r.Note()
		if !reflect.DeepEqual(r.Founded, b.Founded) || !reflect.DeepEqual(r.Captured, b.Captured) {
			t.Errorf("empire %d: a Note after Resume changed the maps", i)
		}
	}
	// An empire left without colonies: a fresh bot, Note, nothing.
	e1.Colonies = nil
	nb := New(e1, Config{})
	nb.Note()
	if len(nb.Founded) != 0 || len(nb.Captured) != 0 {
		t.Errorf("no colony: %v %v", nb.Founded, nb.Captured)
	}
}

// The invasion ship's modules: the fewest beating the target's surface
// shields plus InvadeMargin; built only with contact, a target, and
// fewer loaded invasion ships than ⌈InvadeShare × warships⌉.
func TestInvasionDesign(t *testing.T) {
	for _, tc := range []struct {
		name             string
		contact          bool
		margin, share    float64
		warships, others int // armed ships of mine, loaded invasion ships
		shields          []game.Item
		want             bool
	}{
		{"no contact", false, 1, 0.5, 2, 0, nil, false},
		{"no warship", true, 1, 0.5, 0, 0, nil, false},
		{"bare", true, 1, 0.5, 1, 0, nil, true},
		{"share met", true, 1, 0.5, 2, 1, nil, false},
		{"share 4", true, 1, 4, 1, 3, nil, true},
		{"margin 0", true, 0, 0.5, 1, 0, nil, true},
		{"a shield", true, 1, 0.5, 1, 0, []game.Item{game.SurfaceShield}, true},
		{"shield and mega", true, 2, 0.5, 1, 0, []game.Item{game.SurfaceShield, game.SurfaceMegaShield}, true},
	} {
		gn := genomeWith(func(g *Genome) { g.InvadeMargin, g.InvadeShare = tc.margin, tc.share })
		g, bots, target := invadeFixture(t, gn)
		b := bots[0]
		if !tc.contact {
			b.E.Visited = make([]bool, len(g.Cluster.Stars))
			b.E.Visit(b.E.Colonies[0].Star)
		}
		for range tc.warships {
			b.E.Spawn(game.FixtureGunboat, b.E.Colonies[0].Star, 0)
		}
		for range tc.others {
			b.E.Spawn(invaderOf(t, 1), b.E.Colonies[0].Star, 0)
		}
		worth := 0
		for _, it := range tc.shields {
			shield(t, target.Planet, it)
			worth += game.ShieldWorth[it]
		}
		b.assess()
		d, ok := b.invasionDesign(b.E.Colonies[0])
		if ok != tc.want {
			t.Errorf("%s: design %v, want %v", tc.name, ok, tc.want)
			continue
		}
		if !ok {
			continue
		}
		need := int(math.Ceil(float64(worth+1) / game.ModuleWorth))
		if n := d.InvasionModules(); n != need+int(tc.margin) || n*game.ModuleWorth <= worth || d.LaneDrives() != 1 || d.Valid(b.E.Research) != nil {
			t.Errorf("%s: %v (%d modules), want %d + %v", tc.name, d, n, need, tc.margin)
		}
	}
}

// invadable: another empire's colony at a known star, guarded by no
// Orbital Shield and no other empire's ship in its orbit (mine do not
// guard it against me).
func TestInvadable(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(g *game.Game, target *game.Colony)
		want bool
	}{
		{"bare", func(*game.Game, *game.Colony) {}, true},
		{"orbital shield", func(g *game.Game, target *game.Colony) {
			target.Planet.Orbit[game.OrbitSlots-1] = game.OrbitalShield
		}, false},
		{"its ship in orbit", func(g *game.Game, target *game.Colony) {
			g.Empires[1].Spawn(unarmedScout, target.Star, target.PlanetIdx)
		}, false},
		{"my ship in orbit", func(g *game.Game, target *game.Colony) {
			g.Empires[0].Spawn(unarmedScout, target.Star, target.PlanetIdx)
		}, true},
		{"its ship elsewhere", func(g *game.Game, target *game.Colony) {
			put(g.Empires[1], unarmedScout, target.Star, game.Vec3{X: 1})
		}, true},
		{"unknown", func(g *game.Game, target *game.Colony) {
			g.Empires[0].Visited[target.Star] = false
		}, false},
	} {
		g, bots, target := invadeFixture(t, nil)
		tc.set(g, target)
		if got := bots[0].invadable(target.Star, target.PlanetIdx); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	g, bots, _ := invadeFixture(t, nil)
	home := g.Empires[0].Colonies[0]
	if bots[0].invadable(home.Star, home.PlanetIdx) {
		t.Error("my own colony invadable")
	}
	for pl := range g.Cluster.Stars[home.Star].System.Planets {
		if g.Empires[0].Planet(home.Star, pl).Owner < 0 && bots[0].invadable(home.Star, pl) {
			t.Errorf("unowned planet %d invadable", pl)
		}
	}
	bots[0].G = nil
	if bots[0].invadable(g.Empires[1].Colonies[0].Star, 0) {
		t.Error("invadable without G")
	}
}

// The run: from next door GoToStar the target's star (dest), there
// GoToPlanet the target, in orbit Invade; emptied by the capture, it is
// idle. Guarded, it is given nothing, day after day.
func TestInvadeRun(t *testing.T) {
	g, bots, target := invadeFixture(t, nil)
	b := bots[0]
	c := g.Cluster
	adj := neighbours(c, target.Star)
	b.E.Visit(adj[0])
	s := put(b.E, invaderOf(t, 2), adj[0], game.Vec3{})
	b.Act()
	if s.Order.Kind != game.GoToStar || s.Order.Star != target.Star || b.dest[s] != target.Star {
		t.Fatalf("next door: order %+v dest %d, want GoToStar %d", s.Order, b.dest[s], target.Star)
	}
	s.Star, s.Order, s.Pos = target.Star, game.Order{}, game.Vec3{X: 1}
	b.Act()
	if s.Order.Kind != game.GoToPlanet || s.Order.Planet != target.PlanetIdx {
		t.Fatalf("at the star: order %+v, want GoToPlanet %d", s.Order, target.PlanetIdx)
	}
	// Shielded past its modules on the way (2 Surface Shields, worth 4
	// against 2 modules' 4): dropped, no other target.
	shield(t, target.Planet, game.SurfaceShield)
	shield(t, target.Planet, game.SurfaceShield)
	if b.Act(); s.Order.Kind != game.NoOrder {
		t.Fatalf("shielded past its modules: order %+v", s.Order)
	}
	for _, sq := range target.Planet.Surface {
		if sq.Item == game.SurfaceShield {
			sq.Item = game.NoItem
		}
	}
	if b.Act(); s.Order.Kind != game.GoToPlanet {
		t.Fatalf("unshielded: order %+v", s.Order)
	}
	// Guarded on the way (an Orbital Shield: a guard dealing no damage,
	// a ship would be a threat to flee): the GoToPlanet is dropped,
	// nothing ordered.
	last := &target.Planet.Orbit[game.OrbitSlots-1]
	*last = game.OrbitalShield
	for range 2 {
		b.Act()
		if s.Order.Kind != game.NoOrder {
			t.Fatalf("guarded: order %+v", s.Order)
		}
	}
	// In orbit, still guarded: no Invade; the guard gone, Invade.
	s.Orbit, s.Pos = target.PlanetIdx, target.Planet.Pos
	if b.Act(); s.Order.Kind != game.NoOrder {
		t.Fatalf("guarded in orbit: order %+v", s.Order)
	}
	*last = game.NoItem
	b.Act()
	if s.Order.Kind != game.Invade {
		t.Fatalf("in orbit: order %+v, want Invade", s.Order)
	}
	g.EndDay()
	b.Note()
	if target.Planet.Owner != 0 || s.Invaders != 1 { // no shields: one module lands
		t.Fatalf("no capture: owner %d invaders %d", target.Planet.Owner, s.Invaders)
	}
	b.Act()
	if s.Order.Kind != game.NoOrder {
		t.Errorf("emptied: order %+v", s.Order)
	}
}

// Shields past its modules: the target is not one for this ship.
func TestInvadeBeats(t *testing.T) {
	g, bots, target := invadeFixture(t, nil)
	b := bots[0]
	shield(t, target.Planet, game.SurfaceShield)
	s := b.E.Spawn(invaderOf(t, 1), target.Star, target.PlanetIdx) // 1 module vs worth 2
	if b.Act(); s.Order.Kind != game.NoOrder {
		t.Errorf("1 module vs a Surface Shield: order %+v", s.Order)
	}
	s.Invaders = 2
	if b.Act(); s.Order.Kind != game.Invade {
		t.Errorf("2 modules: order %+v", s.Order)
	}
	_ = g
}

// Colonising and invading in orbit, a ship never flees: it acts today.
func TestInvaderStaysInOrbit(t *testing.T) {
	g, bots, target := invadeFixture(t, nil)
	b := bots[0]
	s := b.E.Spawn(invaderOf(t, 2), target.Star, target.PlanetIdx)
	put(g.Empires[1], game.FixtureRaider, target.Star, game.Vec3{X: 1})
	if err := b.E.Order(s, game.Order{Kind: game.Invade}); err != nil {
		t.Fatal(err)
	}
	b.assess()
	if b.flee(s) || s.Order.Kind != game.Invade {
		t.Errorf("flee: order %+v", s.Order)
	}
	b.Round(target.Star)
	if s.Order.Kind != game.Invade {
		t.Errorf("round: order %+v", s.Order)
	}
}

// A GoToPlanet to a planet another empire guards (Barred) is cleared in
// Act: the ship plans again instead of bouncing for ever.
func TestActClearsBarredGoToPlanet(t *testing.T) {
	g, bots, _ := invadeFixture(t, nil)
	b := bots[0]
	home := b.E.Colonies[0].Star
	pl := -1
	for k := range g.Cluster.Stars[home].System.Planets {
		if !b.E.Inhabited(home, k) {
			pl = k
			break
		}
	}
	if pl < 0 {
		t.Fatal("no free planet at home")
	}
	guard := g.Empires[1].Spawn(unarmedScout, home, pl)
	if !b.E.Barred(home, pl) {
		t.Fatal("not barred: the fixture proves nothing")
	}
	// Scanned (an eye beside it): a hulk dealing nothing, no reason to
	// flee; the ship's order is Act's alone.
	put(b.E, game.Design{Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.TonklinFrequencyAnalyzer}}, home, guard.Pos)
	s := put(b.E, unarmedScout, home, game.Vec3{X: 1})
	s.Order = game.Order{Kind: game.GoToPlanet, Planet: pl}
	if b.Act(); s.Order.Kind == game.GoToPlanet && s.Order.Planet == pl {
		t.Errorf("barred GoToPlanet kept: %+v", s.Order)
	}
}

// The invasion genes: neutral 1 module to spare, an invasion ship per two
// warships (a nil genome plays the same), clamped to [0, 10] and [0, 4].
func TestInvadeGenes(t *testing.T) {
	n, c := Neutral(), NeutralCombat()
	if n.InvadeMargin != 1 || n.InvadeShare != 0.5 || c.InvadeMargin != 1 || c.InvadeShare != 0.5 || n.Combat() != c {
		t.Errorf("neutral: %v %v / %+v", n.InvadeMargin, n.InvadeShare, c)
	}
	for _, tc := range []struct{ in, margin, share float64 }{
		{-1, 0, 0}, {99, 10, 4}, {math.NaN(), 0, 0}, {2, 2, 2},
	} {
		g := Neutral()
		g.InvadeMargin, g.InvadeShare = tc.in, tc.in
		g.Clamp()
		if g.InvadeMargin != tc.margin || g.InvadeShare != tc.share {
			t.Errorf("clamp %v: %v %v, want %v %v", tc.in, g.InvadeMargin, g.InvadeShare, tc.margin, tc.share)
		}
	}
}
