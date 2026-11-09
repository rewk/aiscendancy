package bot

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// specialShip puts a Small of mine with parts at X 0 of home, an Orfa
// Raider of empire 1 at X 0.4 (in range of a Mass Barrage Gun only when
// boosted: 0.25 × 2), energy 20 (every special's Power within it).
func specialShip(t *testing.T, gn *Genome, parts ...game.Gizmo) (*game.Game, *Bot, *game.Starship, int) {
	t.Helper()
	g, b, home := roundFixture(t, gn)
	s := put(b.E, game.Design{Name: "Spec", Hull: game.Small, Parts: parts}, home, game.Vec3{})
	put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.4})
	s.Power = 20
	if len(g.Hostiles(b.E, home)) != 1 {
		t.Fatal("fixture: the raider is not seen")
	}
	return g, b, s, home
}

// partOf is the index of g in s's parts, −1 none.
func partOf(s *game.Starship, g game.Gizmo) int { return slices.Index(s.Design.Parts, g) }

// An Accutron is used before the shots are armed: the raider at 0.4,
// out of the gun's 0.25 but within its boosted 0.5, is shot at at once
// (shots first, no move); without the Accutron the ship closes in first
// and arms nothing (its one generator kept for the move).
func TestSpecialAccutron(t *testing.T) {
	_, b, s, home := specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Accutron)
	b.Round(home)
	if !s.Boosted || s.Shots[partOf(s, game.Accutron)] != 1 || len(s.Armed) != 1 || s.Order.Kind == game.GoToPoint {
		t.Errorf("accutron: boosted %v, shots %v, armed %d, order %+v", s.Boosted, s.Shots, len(s.Armed), s.Order)
	}
	_, b, s, home = specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun)
	if b.Round(home); s.Boosted || len(s.Armed) != 0 || s.Order.Kind != game.GoToPoint {
		t.Errorf("no accutron: boosted %v, armed %d, order %+v", s.Boosted, len(s.Armed), s.Order)
	}
	// No weapon aboard: no use.
	_, b, s, home = specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.Accutron)
	if b.Round(home); s.Boosted || s.Shots[partOf(s, game.Accutron)] != 0 {
		t.Errorf("unarmed: boosted %v, shots %v", s.Boosted, s.Shots)
	}
}

// A Replenisher is used when every limited weapon is spent (the gun's
// shot back, armed again this round), not while one has a shot left
// (which the game would accept: another weapon fired).
func TestSpecialReplenisher(t *testing.T) {
	_, b, s, home := specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Replenisher)
	gun := partOf(s, game.MassBarrageGun)
	s.Shots[gun] = 1
	s.Pos = game.Vec3{X: 0.3} // the raider in range
	if b.Round(home); s.Shots[partOf(s, game.Replenisher)] != 1 || s.Shots[gun] != 0 || len(s.Armed) != 1 {
		t.Errorf("spent: shots %v, armed %d", s.Shots, len(s.Armed))
	}
	d := game.Design{Name: "Two", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.MassBarrageGun, game.Replenisher}}
	_, b, _, home = specialShip(t, noFlight(), game.ProtonShaver)
	two := put(b.E, d, home, game.Vec3{X: 0.3})
	two.Power, two.Shots[2] = 20, 1
	if b.Round(home); two.Shots[4] != 0 || two.Shots[2] != 1 {
		t.Errorf("one gun left: shots %v", two.Shots)
	}
}

// Hyperfuel is burnt when the energy left is short of a move and the
// cheapest shot (1 + 1), the part lost; not at that energy.
func TestSpecialHyperfuel(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Hyperfuel}
	_, b, s, home := specialShip(t, noFlight(), parts...)
	s.Power = 1
	if b.Round(home); partOf(s, game.Hyperfuel) >= 0 || s.Power < 1+game.Gizmos[game.Hyperfuel].Special1-2 {
		t.Errorf("low: parts %v, power %d", s.Design.Parts, s.Power)
	}
	_, b, s, home = specialShip(t, noFlight(), parts...)
	need := s.Design.MoveCost() + game.Gizmos[game.MassBarrageGun].Power
	s.Power = need
	if b.Round(home); partOf(s, game.Hyperfuel) < 0 {
		t.Errorf("energy %d: burnt", need)
	}
}

// The Cannibalizer: low energy and Hull above CannibalAt × MaxHull (20
// on a Snovemdomas Small): 5 hull for 20 energy; not at Hull 10 (half),
// which the game would accept (10 > 5).
func TestSpecialCannibalizer(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Cannibalizer}
	_, b, s, home := specialShip(t, noFlight(), parts...)
	s.Power = 0
	in := &game.Gizmos[game.Cannibalizer]
	if b.Round(home); s.Hull != s.MaxHull()-in.Special1 || s.Shots[partOf(s, game.Cannibalizer)] != 1 {
		t.Errorf("full hull: hull %d, shots %v", s.Hull, s.Shots)
	}
	_, b, s, home = specialShip(t, noFlight(), parts...)
	s.Power, s.Hull = 0, s.MaxHull()/2
	if b.Round(home); s.Hull != s.MaxHull()/2 || s.Shots[partOf(s, game.Cannibalizer)] != 0 {
		t.Errorf("half hull: hull %d, shots %v", s.Hull, s.Shots)
	}
	// Fuel first: the Hyperfuel's energy leaves no need for the hull.
	_, b, s, home = specialShip(t, noFlight(), append(parts, game.Hyperfuel)...)
	s.Power = 0
	if b.Round(home); s.Hull != s.MaxHull() || partOf(s, game.Hyperfuel) >= 0 {
		t.Errorf("fuel and cannibal: hull %d, parts %v", s.Hull, s.Design.Parts)
	}
}

// The Toroidal Blaster: energy short of a move, two drives aboard: one
// burnt for 40 energy; with a single drive (the game would accept) no
// use.
func TestSpecialToroidal(t *testing.T) {
	_, b, s, home := specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.TonklinMotor, game.MassBarrageGun, game.ToroidalBlaster)
	s.Power = 1 // MoveCost 2
	if b.Round(home); drives(s) != 1 || s.Shots[partOf(s, game.ToroidalBlaster)] != 1 {
		t.Errorf("two drives: parts %v, shots %v", s.Design.Parts, s.Shots)
	}
	_, b, s, home = specialShip(t, noFlight(), game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.ToroidalBlaster)
	s.Power = 0
	if b.Round(home); drives(s) != 1 || s.Shots[partOf(s, game.ToroidalBlaster)] != 0 {
		t.Errorf("one drive: parts %v, shots %v", s.Design.Parts, s.Shots)
	}
}

// The Invulnerablizer: under fire at Hull ≤ ShieldAt × MaxHull (10 of
// 20); not at 11, nor beside a harmless hostile (HullThreat 0).
func TestSpecialInvulnerablizer(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Invulnerablizer}
	for _, tc := range []struct {
		hull   int
		threat float64
		used   bool
	}{{10, 2, true}, {11, 2, false}, {10, 0, false}} {
		_, b, s, home := specialShip(t, genomeWith(func(g *Genome) { g.FleeRatio, g.HullThreat[game.Small] = 1e9, tc.threat }), parts...)
		s.Hull = tc.hull
		b.Round(home)
		if s.Invulnerable != tc.used || (partOf(s, game.Invulnerablizer) < 0) != tc.used {
			t.Errorf("hull %d threat %v: invulnerable %v, parts %v", tc.hull, tc.threat, s.Invulnerable, s.Design.Parts)
		}
	}
}

// The Recaller: under fire away from home at Hull ≤ RecallAt × MaxHull
// (5 of 20): the ship is home, and Round leaves it alone after (no shot
// armed, no order at the star it left); at Hull 6 it stays and fights.
func TestSpecialRecaller(t *testing.T) {
	for _, tc := range []struct {
		hull   int
		recall bool
	}{{5, true}, {6, false}} {
		g, b, home, adj := fleeFixture(t, noFlight())
		v := adj[0]
		s := put(b.E, game.Design{Name: "Spec", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Recaller}}, v, game.Vec3{})
		put(g.Empires[1], game.FixtureRaider, v, game.Vec3{X: 0.2})
		s.Power, s.Hull = 20, tc.hull
		if len(g.Hostiles(b.E, v)) != 1 || v == b.E.HomeStar() {
			t.Fatal("fixture: no raider seen away from home")
		}
		b.Round(v)
		switch {
		case tc.recall && (s.Star != home || len(s.Armed) != 0 || s.Order.Kind != game.NoOrder):
			t.Errorf("hull %d: star %d (home %d), armed %d, order %+v", tc.hull, s.Star, home, len(s.Armed), s.Order)
		case !tc.recall && (s.Star != v || s.Shots[partOf(s, game.Recaller)] != 0 || len(s.Armed) != 1):
			t.Errorf("hull %d: star %d, shots %v, armed %d", tc.hull, s.Star, s.Shots, len(s.Armed))
		}
	}
}

// On a quiet star (no hostile seen) the Remote Repair Facility is used
// while it is usable and the hull short: 100 energy = 3 uses of 30, the
// hull +5 +5 then capped (12 short: full); beside a hostile, no repair.
// Act calls it.
func TestSpecialRepairs(t *testing.T) {
	d := game.Design{Name: "Fix", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.RemoteRepairFacility}}
	_, b, home := roundFixture(t, noFlight())
	s := put(b.E, d, home, game.Vec3{})
	s.Power, s.Hull = 100, s.MaxHull()-12
	b.repairs()
	if s.Hull != s.MaxHull() || s.Shots[2] != 3 || s.Power != 10 {
		t.Errorf("quiet: hull %d of %d, shots %v, power %d", s.Hull, s.MaxHull(), s.Shots, s.Power)
	}
	// Energy for one use only: one use, the hull still short.
	s.Shots[2], s.Power, s.Hull = 0, 30, s.MaxHull()-12
	if b.repairs(); s.Hull != s.MaxHull()-7 || s.Shots[2] != 1 || s.Power != 0 {
		t.Errorf("one use: hull %d, shots %v, power %d", s.Hull, s.Shots, s.Power)
	}
	// Through Act.
	s.Shots[2], s.Power, s.Hull = 0, 30, s.MaxHull()-12
	if b.Act(); s.Hull != s.MaxHull()-7 {
		t.Errorf("Act: hull %d", s.Hull)
	}
	// Being refitted at its Docks: the refit's fixed cost pays the hull,
	// the pass leaves it alone (refitFixture's Docks and research).
	col := b.E.Colonies[0]
	b.E.Research = allKnown()
	col.Planet.Orbit[1] = game.OrbitalDocks
	col.Pop = col.Busy() + 1
	s.Shots[2], s.Power, s.Hull, s.Orbit = 0, 100, s.MaxHull()-12, col.PlanetIdx
	col.Cancel()
	if err := b.E.Refit(col, s, s.Design); err != nil {
		t.Fatal(err)
	}
	if b.repairs(); s.Shots[2] != 0 {
		t.Errorf("refitting: shots %v, want none", s.Shots)
	}
	// A hostile seen there: no repair.
	g, b, home := roundFixture(t, noFlight())
	s = put(b.E, d, home, game.Vec3{})
	put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.4})
	s.Power, s.Hull = 100, s.MaxHull()-12
	if b.repairs(); s.Hull != s.MaxHull()-12 || s.Shots[2] != 0 {
		t.Errorf("hostile: hull %d, shots %v", s.Hull, s.Shots)
	}
}

// targetFixture puts a Small of mine with parts at the origin of home
// (energy 100), and empire 1's foe design at at; an Eye of mine beside
// (Subspace Phase Array, X Ray Megaglasses when xray: the foe's parts
// known) when scan, else the foe unscanned.
func targetFixture(t *testing.T, gn *Genome, foe game.Design, at game.Vec3, scan, xray bool, parts ...game.Gizmo) (*game.Game, *Bot, *game.Starship, *game.Starship, int) {
	t.Helper()
	g, b, home := roundFixture(t, gn)
	s := put(b.E, game.Design{Name: "Spec", Hull: game.Small, Parts: parts}, home, game.Vec3{})
	s.Power = 100
	f := put(g.Empires[1], foe, home, at)
	eyes(t, g, b, home, f, scan, xray)
	return g, b, s, f, home
}

// eyes spawns my scanner Eye at the origin of star when scan (X Ray too
// when xray) and checks f is seen as asked.
func eyes(t *testing.T, g *game.Game, b *Bot, star int, f *game.Starship, scan, xray bool) {
	t.Helper()
	if scan {
		d := game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.SubspacePhaseArray}}
		if xray {
			d.Parts = append(d.Parts, game.XRayMegaglasses)
		}
		put(b.E, d, star, game.Vec3{Y: 0.01})
	}
	i := slices.IndexFunc(g.Hostiles(b.E, star), func(h game.Sighting) bool { return h.Pos == f.Pos })
	if i < 0 {
		t.Fatal("fixture: the foe is not seen")
	}
	h := g.Hostiles(b.E, star)[i]
	if (h.Scan != nil) != scan || scan && h.Scan.Parts != xray {
		t.Fatalf("fixture: scan %+v, want scan %v xray %v", h.Scan, scan, xray)
	}
}

// useAt runs the bot's specials at star as Round does.
func useAt(b *Bot, star int) { b.specials(star, b.G.Hostiles(b.E, star), b.genes()) }

// alive: f is still in its empire's Ships.
func alive(g *game.Game, f *game.Starship) bool { return g.OwnerOf(f) != nil }

// usedNews: b's news holds a SpecialUsed report of b's empire.
func usedNews(b *Bot) bool {
	return slices.ContainsFunc(b.News(), func(r game.Report) bool {
		return r.Empire == b.E.Index && r.Event.Kind == game.SpecialUsed
	})
}

var (
	foeScout  = game.Design{Name: "Eye", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}
	foeColony = game.Design{Name: "Seed", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.Colonizer}}
)

// The Disintegrator kills the strongest armed hostile in range (the
// Lancer over a nearer Raider), its news the SpecialUsed line and the
// Destroyed pair, through Round; a Disarmer after it, and Round's
// gunboat, aim at the Raider, read again after the kill. Unknown parts
// with HullThreat 0: no use.
func TestSpecialDisintegrator(t *testing.T) {
	g, b, s, raider, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.1}, true, true,
		game.ProtonShaver, game.TonklinMotor, game.Disintegrator, game.Disarmer)
	lancer := put(g.Empires[1], game.FixtureLancer, home, game.Vec3{X: 0.2})
	// A gunboat nearer the Lancer (roundFight's tie on hull points): it
	// must fire at the Raider, not the Lancer's stale sighting.
	gun := put(b.E, game.FixtureGunboat, home, game.Vec3{X: 0.25})
	b.Round(home)
	if len(gun.Armed) != 1 || gun.Armed[0].Target.Pos != raider.Pos {
		t.Errorf("gunboat after the kill: armed %+v", gun.Armed)
	}
	news := b.News()
	destroyed := slices.ContainsFunc(news, func(r game.Report) bool { return r.Event.Kind == game.Destroyed })
	if alive(g, lancer) || !alive(g, raider) || partOf(s, game.Disintegrator) >= 0 || !destroyed ||
		news[0].Event.Kind != game.SpecialUsed || news[0].Empire != b.E.Index {
		t.Errorf("lancer alive %v, raider alive %v, parts %v, news %+v", alive(g, lancer), alive(g, raider), s.Design.Parts, news)
	}
	if raider.Design.Weapons() != 0 {
		t.Errorf("the Disarmer after the kill: raider parts %v", raider.Design.Parts)
	}
	g, b, s, raider, home = targetFixture(t, genomeWith(func(g *Genome) { g.FleeRatio, g.HullThreat[game.Small] = 1e9, 0 }),
		game.FixtureRaider, game.Vec3{X: 0.1}, false, false, game.ProtonShaver, game.TonklinMotor, game.Disintegrator)
	if useAt(b, home); !alive(g, raider) || partOf(s, game.Disintegrator) < 0 || len(b.News()) != 0 {
		t.Errorf("harmless unknown: raider alive %v, parts %v", alive(g, raider), s.Design.Parts)
	}
}

// The Brunswik Dissipator drains an armed hostile with energy left; not
// one scanned at energy 0 (the game would accept).
func TestSpecialBrunswik(t *testing.T) {
	for _, power := range []int{3, 0} {
		_, b, s, f, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.BrunswikDissipator)
		f.Power = power
		eyes(t, b.G, b, home, f, true, true)
		useAt(b, home)
		if used := s.Shots[partOf(s, game.BrunswikDissipator)] == 1; used != (power > 0) || f.Power != 0 || used != usedNews(b) {
			t.Errorf("power %d: used %v, foe power %d", power, used, f.Power)
		}
	}
}

// The Intellect Scrambler: the armed Raider scrambled; a known unarmed
// scout not (the game would accept).
func TestSpecialScrambler(t *testing.T) {
	for _, foe := range []game.Design{game.FixtureRaider, foeScout} {
		_, b, _, f, home := targetFixture(t, noFlight(), foe, game.Vec3{X: 0.3}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.IntellectScrambler)
		if useAt(b, home); f.Scrambled != (foe.Name == "Raider") {
			t.Errorf("%s: scrambled %v", foe.Name, f.Scrambled)
		}
	}
}

// The Shield Blaster: shields on, blasted down; scanned with them off,
// no use (the game would accept).
func TestSpecialShieldBlaster(t *testing.T) {
	for _, on := range []bool{true, false} {
		_, b, s, f, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.ShieldBlaster)
		f.Shields = on
		useAt(b, home)
		if used := s.Shots[partOf(s, game.ShieldBlaster)] == 1; used != on || f.ShieldsDown != on {
			t.Errorf("shields %v: used %v, down %v", on, used, f.ShieldsDown)
		}
	}
}

// The Disarmer: an unknown Raider (HullThreat 2) loses its gun; unknown
// with HullThreat 0 it keeps it (the game would accept).
func TestSpecialDisarmer(t *testing.T) {
	for _, threat := range []float64{2, 0} {
		_, b, _, f, home := targetFixture(t, genomeWith(func(g *Genome) { g.FleeRatio, g.HullThreat[game.Small] = 1e9, threat }),
			game.FixtureRaider, game.Vec3{X: 0.3}, false, false, game.ProtonShaver, game.TonklinMotor, game.Disarmer)
		if useAt(b, home); (f.Design.Weapons() == 0) != (threat > 0) {
			t.Errorf("threat %v: foe parts %v", threat, f.Design.Parts)
		}
	}
}

// The Backfirer: the known one-gun Raider hurt; scanned without X Ray
// (its weapons unknown), no use (the game would accept).
func TestSpecialBackfirer(t *testing.T) {
	for _, xray := range []bool{true, false} {
		_, b, _, f, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, true, xray,
			game.ProtonShaver, game.TonklinMotor, game.Backfirer)
		if useAt(b, home); (f.Hull < f.MaxHull()) != xray {
			t.Errorf("xray %v: foe hull %d of %d", xray, f.Hull, f.MaxHull())
		}
	}
}

// The Moving Part Exploiter: the Raider's five known parts, five hull
// points; a four-part ship, no use (the game would accept).
func TestSpecialExploiter(t *testing.T) {
	four := game.Design{Name: "Four", Hull: game.Small, Parts: game.FixtureRaider.Parts[:4]}
	for _, foe := range []game.Design{game.FixtureRaider, four} {
		_, b, _, f, home := targetFixture(t, noFlight(), foe, game.Vec3{X: 0.3}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.MovingPartExploiter)
		want := f.MaxHull()
		if len(foe.Parts) == 5 {
			want -= 5
		}
		if useAt(b, home); f.Hull != want {
			t.Errorf("%d parts: foe hull %d, want %d", len(foe.Parts), f.Hull, want)
		}
	}
}

// The Gizmogrifier: the armed Raider loses Parts[0]; a known unarmed
// scout, no use (the game would accept).
func TestSpecialGizmogrifier(t *testing.T) {
	for _, foe := range []game.Design{game.FixtureRaider, foeScout} {
		_, b, _, f, home := targetFixture(t, noFlight(), foe, game.Vec3{X: 0.3}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.Gizmogrifier)
		if useAt(b, home); (len(f.Design.Parts) < len(foe.Parts)) != (foe.Name == "Raider") {
			t.Errorf("%s: parts %v", foe.Name, f.Design.Parts)
		}
	}
}

// The Specialty Blaster: a Raider with a Hyperfuel known loses it; its
// parts unknown (no X Ray), no use (the game would accept).
func TestSpecialSpecialtyBlaster(t *testing.T) {
	foe := game.Design{Name: "Fuel", Hull: game.Small, Parts: append(slices.Clone(game.FixtureRaider.Parts), game.Hyperfuel)}
	for _, xray := range []bool{true, false} {
		_, b, _, f, home := targetFixture(t, noFlight(), foe, game.Vec3{X: 0.3}, true, xray,
			game.ProtonShaver, game.TonklinMotor, game.SpecialtyBlaster)
		if useAt(b, home); slices.Contains(f.Design.Parts, game.Hyperfuel) == xray {
			t.Errorf("xray %v: parts %v", xray, f.Design.Parts)
		}
	}
}

// awayFixture is targetFixture at a star next to home (no colony of
// mine there).
func awayFixture(t *testing.T, foe game.Design, scan, xray bool, parts ...game.Gizmo) (*Bot, *game.Starship, *game.Starship, int) {
	t.Helper()
	g, b, _, adj := fleeFixture(t, noFlight())
	v := adj[0]
	if b.colonyAt(v) {
		t.Fatal("fixture: a colony next door")
	}
	s := put(b.E, game.Design{Name: "Spec", Hull: game.Small, Parts: parts}, v, game.Vec3{})
	s.Power = 100
	f := put(g.Empires[1], foe, v, game.Vec3{X: 0.3})
	eyes(t, g, b, v, f, scan, xray)
	return b, s, f, v
}

// The Molecular Tie Down: a colony ship (unscanned: cargo unknown) by
// my home, tied down; the same next door (no colony of mine), no use.
func TestSpecialTieDown(t *testing.T) {
	_, b, _, f, home := targetFixture(t, noFlight(), foeColony, game.Vec3{X: 0.3}, false, false,
		game.ProtonShaver, game.TonklinMotor, game.MolecularTieDown)
	if useAt(b, home); !f.TiedDown || !usedNews(b) {
		t.Errorf("at home: tied %v", f.TiedDown)
	}
	b, s, f, v := awayFixture(t, foeColony, false, false, game.ProtonShaver, game.TonklinMotor, game.MolecularTieDown)
	if useAt(b, v); f.TiedDown || s.Shots[2] != 0 {
		t.Errorf("away: tied %v, shots %v", f.TiedDown, s.Shots)
	}
}

// The Containment Device, no colony needed: the known unarmed colony
// ship loses its Colonizer; an armed one (a gun known aboard), no use
// (the game would accept).
func TestSpecialContainment(t *testing.T) {
	gunned := game.Design{Name: "Gun", Hull: game.Small, Parts: append(slices.Clone(foeColony.Parts), game.MassBarrageGun)}
	for _, foe := range []game.Design{foeColony, gunned} {
		b, _, f, v := awayFixture(t, foe, true, true, game.ProtonShaver, game.TonklinMotor, game.ContainmentDevice)
		if useAt(b, v); (f.Cargo == 0) != (foe.Name == "Seed") {
			t.Errorf("%s: cargo %d, parts %v", foe.Name, f.Cargo, f.Design.Parts)
		}
	}
}

// The Myrmidonic Carbonizer at its ideal range hurts the armed Raider; a
// known unarmed scout there, no use (the game would accept).
func TestSpecialCarbonizer(t *testing.T) {
	ideal := float64(game.Gizmos[game.MyrmidonicCarbonizer].Special1) * game.RangeUnit
	for _, foe := range []game.Design{game.FixtureRaider, foeScout} {
		_, b, _, f, home := targetFixture(t, noFlight(), foe, game.Vec3{X: ideal}, true, true,
			game.ProtonShaver, game.TonklinMotor, game.MyrmidonicCarbonizer)
		if useAt(b, home); (f.Hull < f.MaxHull()) != (foe.Name == "Raider") {
			t.Errorf("%s: hull %d of %d", foe.Name, f.Hull, f.MaxHull())
		}
	}
}

// The Smart Bomb: two Raiders within its blast (SmartBombAt 2), both
// hit; one alone, no use (the game would accept), unless SmartBombAt 1.
func TestSpecialSmartBomb(t *testing.T) {
	for _, tc := range []struct {
		foes int
		at   float64
		used bool
	}{{2, 2, true}, {1, 2, false}, {1, 1, true}} {
		g, b, s, f, home := targetFixture(t, genomeWith(func(g *Genome) { g.FleeRatio, g.SmartBombAt = 1e9, tc.at }),
			game.FixtureRaider, game.Vec3{X: 0.1}, false, false, game.ProtonShaver, game.TonklinMotor, game.SmartBomb)
		fs := []*game.Starship{f}
		if tc.foes == 2 {
			fs = append(fs, put(g.Empires[1], game.FixtureRaider, home, game.Vec3{Y: 0.1}))
		}
		useAt(b, home)
		if (partOf(s, game.SmartBomb) < 0) != tc.used {
			t.Errorf("%d foes, at %v: parts %v", tc.foes, tc.at, s.Design.Parts)
		}
		for _, x := range fs {
			if hit := !alive(g, x) || x.Hull < x.MaxHull(); hit != tc.used {
				t.Errorf("%d foes, at %v: a foe hit %v", tc.foes, tc.at, hit)
			}
		}
	}
}

// The Self Destructotron: about to die (their dmg 4 ≥ Hull 1), no
// escape (HullReach 5: every point within reach), a Raider within its
// blast: it goes, the Raider hurt, and the next ship of mine still uses
// its Accutron (Ships shrank under the loop); with the default reach (an
// escape), or at Hull 20 (above their dmg), no use.
func TestSpecialSelfDestructotron(t *testing.T) {
	for _, tc := range []struct {
		reach float64
		hull  int
		gone  bool
	}{{5, 1, true}, {0, 1, false}, {5, 20, false}} {
		gn := genomeWith(func(g *Genome) {
			g.FleeRatio = 1e9
			if tc.reach > 0 {
				g.HullReach[game.Small] = tc.reach
			}
		})
		g, b, s, f, home := targetFixture(t, gn, game.FixtureRaider, game.Vec3{X: 0.2}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.SelfDestructotron)
		s.Hull = tc.hull
		next := put(b.E, game.Design{Name: "Next", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.Accutron}}, home, game.Vec3{X: -0.6})
		next.Power = 20
		put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 1}) // out of the blast: hostiles stay seen
		useAt(b, home)
		gone := g.OwnerOf(s) == nil
		if gone != tc.gone || (f.Hull < f.MaxHull()) != gone || !next.Boosted {
			t.Errorf("reach %v hull %d: gone %v, foe hull %d, next boosted %v", tc.reach, tc.hull, gone, f.Hull, next.Boosted)
		}
	}
}

// The Sacrificial Orb: a full user (20) takes 6 off a mate at 5; at 15
// (15 − 6 ≤ ShieldAt × 20), or the mate at 15 (above ShieldAt × 20), no
// use (the game would accept).
func TestSpecialOrb(t *testing.T) {
	for _, tc := range []struct {
		user, mate int
		used       bool
	}{{20, 5, true}, {15, 5, false}, {20, 15, false}} {
		_, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 1}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.SacrificialOrb)
		s.Hull = tc.user
		mate := put(b.E, foeScout, home, game.Vec3{X: -0.3})
		mate.Hull = tc.mate
		useAt(b, home)
		n := 0
		if tc.used {
			n = game.Gizmos[game.SacrificialOrb].Special1
		}
		if mate.Hull != tc.mate+n || s.Hull != tc.user-n {
			t.Errorf("user %d mate %d: mate %d, user %d", tc.user, tc.mate, mate.Hull, s.Hull)
		}
	}
}

// The Plasma Coupler (two motors: MoveCost 2): energy at 2 × MoveCost + 4,
// half to the mate short of a move; one less, or the mate's energy at its
// MoveCost, no use (the game would accept: 1 to beam).
func TestSpecialCoupler(t *testing.T) {
	for _, tc := range []struct {
		less, mate int
		used       bool
	}{{0, 0, true}, {1, 0, false}, {0, -1, false}} {
		_, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 1}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.TonklinMotor, game.PlasmaCoupler)
		need := 2*s.Design.MoveCost() + game.Gizmos[game.PlasmaCoupler].Power
		s.Power = need - tc.less
		if (need-1-game.Gizmos[game.PlasmaCoupler].Power)/2 <= 0 {
			t.Fatalf("fixture: %d short of the need beams nothing", need-1)
		}
		mate := put(b.E, foeScout, home, game.Vec3{X: -0.3})
		mate.Power = 0
		if tc.mate < 0 {
			mate.Power = mate.Design.MoveCost()
		}
		before := mate.Power
		useAt(b, home)
		if (mate.Power > before) != tc.used {
			t.Errorf("less %d mate %d: mate %d, user %d", tc.less, tc.mate, mate.Power, s.Power)
		}
	}
}

// The Phase Bomb at empire 1's home planet (guarded: a Raider in its
// orbit, a planetTargets planet): a structure razed, the Razed report in
// the news (a Factory built: the home holds its base alone); out of its
// range, no use.
func TestSpecialPhaseBomb(t *testing.T) {
	for _, off := range []float64{0.2, 0.3} {
		g, b, _ := roundFixture(t, noFlight())
		col := g.Empires[1].Colonies[0]
		star, p := col.Star, col.Planet
		b.E.Visit(star)
		if !slices.ContainsFunc(p.Cells(), func(c game.Cell) bool { return p.Build(c, game.Factory) == nil }) {
			t.Fatal("fixture: no Factory built")
		}
		guard := g.Empires[1].Spawn(game.FixtureRaider, star, slices.Index(g.Cluster.Stars[star].System.Planets, p))
		s := put(b.E, game.Design{Name: "Spec", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.PhaseBomb}},
			star, game.Vec3{X: p.Pos.X + off, Y: p.Pos.Y, Z: p.Pos.Z})
		s.Power = 20
		if len(b.planetTargets(star)) != 1 || len(g.Hostiles(b.E, star)) != 1 || guard.Orbit < 0 {
			t.Fatalf("fixture: targets %v", b.planetTargets(star))
		}
		useAt(b, star)
		razed := slices.ContainsFunc(b.News(), func(r game.Report) bool { return r.Event.Kind == game.Razed && r.Empire == 1 })
		if razed != (off == 0.2) || (partOf(s, game.PhaseBomb) < 0) != razed {
			t.Errorf("off %v: razed %v, parts %v", off, razed, s.Design.Parts)
		}
	}
}

// A repair's report goes to the news.
func TestSpecialRepairsNews(t *testing.T) {
	_, b, home := roundFixture(t, noFlight())
	s := put(b.E, game.Design{Name: "Fix", Hull: game.Small, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.RemoteRepairFacility}}, home, game.Vec3{})
	s.Power, s.Hull = 30, s.MaxHull()-5
	if b.repairs(); !usedNews(b) {
		t.Error("no SpecialUsed in the news")
	}
}

// shotsOf is the uses of s's g part today, −1 when the part is gone (a
// one-time device spent).
func shotsOf(s *game.Starship, g game.Gizmo) int {
	if k := partOf(s, g); k >= 0 {
		return s.Shots[k]
	}
	return -1
}

// The Positron Bouncer: an unscanned Raider at 0.4, within its
// HullReach 0.45 of me (it can hit me), pushed away; at 0.5, out of its
// reach but in the Bouncer's 0.6, no use (the game would accept).
func TestSpecialBouncer(t *testing.T) {
	for _, x := range []float64{0.4, 0.5} {
		_, b, s, f, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: x}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.PositronBouncer)
		useAt(b, home)
		if used := shotsOf(s, game.PositronBouncer) == 1; used != (x == 0.4) || used != (dist(f.Pos, s.Pos) > x+0.01) || used != usedNews(b) {
			t.Errorf("raider at %v: used %v, raider at %+v", x, used, f.Pos)
		}
	}
}

// The Tractor Beam: an unscanned colony ship at 0.45, beyond my gun's
// 0.25, pulled in; a known armed Raider there, or the colony ship at 0.2
// (within the gun's range), no use (the game would accept both).
func TestSpecialTractor(t *testing.T) {
	for _, tc := range []struct {
		foe  game.Design
		x    float64
		scan bool
		used bool
	}{{foeColony, 0.45, false, true}, {game.FixtureRaider, 0.45, true, false}, {foeColony, 0.2, false, false}} {
		_, b, s, f, home := targetFixture(t, noFlight(), tc.foe, game.Vec3{X: tc.x}, tc.scan, tc.scan,
			game.ProtonShaver, game.TonklinMotor, game.MassBarrageGun, game.GizmoTractorBeam)
		useAt(b, home)
		if used := shotsOf(s, game.GizmoTractorBeam) == 1; used != tc.used || used != (dist(f.Pos, s.Pos) < tc.x-0.01) {
			t.Errorf("%s at %v: used %v, foe at %+v", tc.foe.Name, tc.x, used, f.Pos)
		}
	}
	// No gun aboard: nothing to pull it into.
	_, b, s, _, home := targetFixture(t, noFlight(), foeColony, game.Vec3{X: 0.45}, false, false,
		game.ProtonShaver, game.TonklinMotor, game.GizmoTractorBeam)
	if useAt(b, home); shotsOf(s, game.GizmoTractorBeam) != 0 {
		t.Errorf("no gun: shots %v", s.Shots)
	}
}

// cornered is a genome where a Medium hull reaches the whole system (no
// escape anywhere) and a Small one 0.1 (a Small covers only its
// neighbourhood); FleeRatio out of reach.
func cornered() *Genome {
	return genomeWith(func(g *Genome) {
		g.FleeRatio, g.HullReach[game.Medium], g.HullReach[game.Small] = 1e9, 5, 0.1
	})
}

var foeMedium = game.Design{Name: "Big", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor}}

// The Hyperswapper, cornered (a Medium covering everything, Hull 1):
// a Small beside me at −0.05, the Small at X 1 swapped with (its spot,
// once swapped, under the Medium alone); with the Small beside IT
// instead (its spot hotter), or far from us both (every spot as hot),
// no use (the game would accept).
func TestSpecialHyperswapper(t *testing.T) {
	for _, side := range []game.Vec3{{X: -0.05}, {X: 1.05}, {Z: -1}} {
		g, b, s, f, home := targetFixture(t, cornered(), foeScout, game.Vec3{X: 1}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.Hyperswapper)
		put(g.Empires[1], foeMedium, home, game.Vec3{Y: -1.3}) // out of the swapper's 1.2
		put(g.Empires[1], foeScout, home, side)
		s.Hull = 1
		useAt(b, home)
		swapped := s.Pos == game.Vec3{X: 1} && f.Pos == game.Vec3{}
		if swapped != (side.X < 0) || swapped != (shotsOf(s, game.Hyperswapper) == 1) {
			t.Errorf("small at %+v: me at %+v, target at %+v", side, s.Pos, f.Pos)
		}
	}
	// Not about to die (Hull 20 above their 8): no use.
	g, b, s, _, home := targetFixture(t, cornered(), foeScout, game.Vec3{X: 1}, false, false,
		game.ProtonShaver, game.TonklinMotor, game.Hyperswapper)
	put(g.Empires[1], foeMedium, home, game.Vec3{Y: -1.3})
	put(g.Empires[1], foeScout, home, game.Vec3{X: -0.05})
	if useAt(b, home); shotsOf(s, game.Hyperswapper) != 0 {
		t.Errorf("full hull: shots %v", s.Shots)
	}
}

// The Fleet Disperser: the nearest Raider at 0.3 with two more within
// its 0.45, used; with one more only, no use (the game would accept).
func TestSpecialDisperser(t *testing.T) {
	for _, n := range []int{2, 1} {
		g, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.FleetDisperser)
		for _, y := range []float64{0.2, -0.2}[:n] {
			put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.3, Y: y})
		}
		if useAt(b, home); (shotsOf(s, game.FleetDisperser) == 1) != (n == 2) {
			t.Errorf("%d more: shots %v", n, s.Shots)
		}
	}
}

// The Gravity Distorter: two Raiders within its 0.65, used; one, or a
// mate of mine in space within it too, no use (the game would accept).
func TestSpecialDistorter(t *testing.T) {
	for _, tc := range []struct {
		foes int
		mate bool
		used bool
	}{{2, false, true}, {1, false, false}, {2, true, false}} {
		g, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.GravityDistorter)
		if tc.foes == 2 {
			put(g.Empires[1], game.FixtureRaider, home, game.Vec3{Y: 0.3})
		}
		if tc.mate {
			put(b.E, foeScout, home, game.Vec3{X: -0.3})
		}
		if useAt(b, home); (shotsOf(s, game.GravityDistorter) == 1) != tc.used {
			t.Errorf("%d foes, mate %v: shots %v", tc.foes, tc.mate, s.Shots)
		}
	}
}

// The Mass Condensor at the Raider at X 1 with two more within its 0.5,
// used; one more, or a mate of mine within 0.5 of the Raider, no use
// (the game would accept).
func TestSpecialMassCondensor(t *testing.T) {
	for _, tc := range []struct {
		more int
		mate bool
		used bool
	}{{2, false, true}, {1, false, false}, {2, true, false}} {
		g, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 1}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.MassCondensor)
		for _, y := range []float64{0.3, -0.3}[:tc.more] {
			put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 1, Y: y})
		}
		if tc.mate {
			put(b.E, foeScout, home, game.Vec3{X: 0.8})
		}
		if useAt(b, home); (shotsOf(s, game.MassCondensor) == 1) != tc.used {
			t.Errorf("%d more, mate %v: shots %v", tc.more, tc.mate, s.Shots)
		}
	}
}

// The Gravimetric Condensor from X −0.8 (out of its 0.65): two Raiders
// within it, used; one (the other at 0.9, beyond), or the user at the
// star's centre (pulled too), no use (the game would accept).
func TestSpecialGravimetricCondensor(t *testing.T) {
	for _, tc := range []struct {
		second float64
		at     game.Vec3
		used   bool
	}{{0.4, game.Vec3{X: -0.8}, true}, {0.9, game.Vec3{X: -0.8}, false}, {0.4, game.Vec3{Z: 0.2}, false}} {
		g, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.4}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.GravimetricCondensor)
		s.Pos = tc.at
		put(g.Empires[1], game.FixtureRaider, home, game.Vec3{Y: tc.second})
		if useAt(b, home); (shotsOf(s, game.GravimetricCondensor) == 1) != tc.used {
			t.Errorf("second at %v, me at %+v: shots %v", tc.second, tc.at, s.Shots)
		}
	}
}

// The Gravimetric Catapult, cornered at X 0.5 under a Small at 0.55: it
// whips to the far side (only the Medium there); with the Small at −0.55
// instead (the mirror hotter), no use (the game would accept).
func TestSpecialCatapult(t *testing.T) {
	for _, x := range []float64{0.55, -0.55} {
		g, b, s, _, home := targetFixture(t, cornered(), foeScout, game.Vec3{X: x}, false, false,
			game.ProtonShaver, game.TonklinMotor, game.GravimetricCatapult)
		put(g.Empires[1], foeMedium, home, game.Vec3{Y: -1.3})
		s.Pos = game.Vec3{X: 0.5}
		useAt(b, home)
		if used := shotsOf(s, game.GravimetricCatapult) == 1; used != (x > 0) || used != (s.Pos.X < 0) {
			t.Errorf("small at %v: used %v, me at %+v", x, used, s.Pos)
		}
	}
}

// endpointAlone is an endpoint k of star in System.Lanes (the last one
// whose Pos has no other endpoint within g's reach) for which ok holds
// of its far star, −1 none.
func endpointAlone(c *game.Cluster, star int, g game.Gizmo, ok func(far int) bool) int {
	ls := c.Stars[star].System.Lanes
	for k := len(ls) - 1; k >= 0; k-- {
		probe := &game.Starship{Star: star, Pos: ls[k].Pos}
		if es := laneEndpoints(c, probe, g); len(es) == 1 && ok(es[0].far) {
			return k
		}
	}
	return -1
}

// The Lane Magnetron: an unarmed scout under a Raider (fleeing) at the
// endpoint of its refuge's lane slips into it; one with a GoToStar
// order to another neighbour, at that lane's endpoint, takes that lane;
// beside a harmless Raider (HullThreat 0: not fleeing), no use (the game
// would accept).
func TestSpecialMagnetron(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.StarLaneDrive, game.LaneMagnetron}
	for _, tc := range []struct {
		threat float64
		order  bool
	}{{2, false}, {2, true}, {0, false}} {
		g, b, home, adj := fleeFixture(t, genomeWith(func(g *Genome) { g.HullThreat[game.Small] = tc.threat }))
		c := g.Cluster
		put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.3})
		s := put(b.E, game.Design{Name: "Run", Hull: game.Small, Parts: parts}, home, game.Vec3{})
		s.Power = 100
		v := b.refuge(s)
		if tc.order {
			v = adj[0]
			if v == b.refuge(s) {
				v = adj[1]
			}
			if err := b.E.Order(s, game.Order{Kind: game.GoToStar, Star: v}); err != nil {
				t.Fatal(err)
			}
		}
		k := slices.IndexFunc(c.Stars[home].Lanes, func(li int) bool { return c.Other(li, home) == v })
		s.Pos = c.Stars[home].System.Lanes[k].Pos
		useAt(b, home)
		in := s.Lane == c.Stars[home].Lanes[k] && s.Order == game.Order{Kind: game.GoToStar, Star: v}
		if in != (tc.threat > 0) || in != (partOf(s, game.LaneMagnetron) < 0) {
			t.Errorf("threat %v order %v: lane %d (want %d), order %+v, parts %v", tc.threat, tc.order, s.Lane, c.Stars[home].Lanes[k], s.Order, s.Design.Parts)
		}
	}
}

// The Lane Destabilizer is never used: a Raider of the enemy's inside
// the lane at hand (the game would accept), another at the star.
func TestSpecialDestabilizerNever(t *testing.T) {
	g, b, s, _, home := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.3}, false, false,
		game.ProtonShaver, game.TonklinMotor, game.LaneDestabilizer)
	c := g.Cluster
	s.Pos = c.Stars[home].System.Lanes[0].Pos
	in := put(g.Empires[1], game.FixtureRaider, home, game.Vec3{})
	in.Lane, in.Arrive = c.Stars[home].Lanes[0], b.E.Day+3
	if useAt(b, home); partOf(s, game.LaneDestabilizer) < 0 {
		t.Error("used")
	}
}

// The Lane Blocker in the quiet pass, at war: at my home, the endpoint
// at hand blocked (LaneBlocks keyed by its CLUSTER lane, not its
// endpoint index); its far star holding a colony of mine, or at a star
// with none of mine, no use (the game would accept).
func TestSpecialLaneBlocker(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.LaneBlocker}
	g, b, home, adj := fleeFixture(t, noFlight())
	c := g.Cluster
	k := endpointAlone(c, home, game.LaneBlocker, func(int) bool { return true })
	if k < 0 {
		t.Fatal("fixture: no lone endpoint")
	}
	li := c.Stars[home].Lanes[k]
	if li == k {
		t.Fatalf("fixture: endpoint %d is cluster lane %d (an index mix-up unseen)", k, li)
	}
	s := put(b.E, game.Design{Name: "Bar", Hull: game.Small, Parts: parts}, home, c.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	if b.lanes(); b.E.LaneBlocks[li] != b.E.Day+game.BlockDays || len(b.E.LaneBlocks) != 1 || !usedNews(b) {
		t.Errorf("home: blocks %v, want lane %d", b.E.LaneBlocks, li)
	}
	// The far star mine.
	g, b, home, _ = fleeFixture(t, noFlight())
	s = put(b.E, game.Design{Name: "Bar", Hull: game.Small, Parts: parts}, home, c.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	b.E.Colonies = append(b.E.Colonies, &game.Colony{Star: c.Other(li, home), PlanetIdx: 0})
	if b.lanes(); len(b.E.LaneBlocks) != 0 {
		t.Errorf("far star mine: blocks %v", b.E.LaneBlocks)
	}
	// Next door, no colony of mine there, the far star not home.
	v := adj[0]
	kv := endpointAlone(c, v, game.LaneBlocker, func(far int) bool { return far != home })
	if kv < 0 || b.colonyAt(v) {
		t.Fatal("fixture: no endpoint next door")
	}
	_, b, _, _ = fleeFixture(t, noFlight())
	s = put(b.E, game.Design{Name: "Bar", Hull: game.Small, Parts: parts}, v, c.Stars[v].System.Lanes[kv].Pos)
	s.Power = 100
	if b.lanes(); len(b.E.LaneBlocks) != 0 {
		t.Errorf("next door: blocks %v", b.E.LaneBlocks)
	}
	// At home, at peace (met, no war).
	g = game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	g.Contact(0, 1)
	b = NewAll(g, Config{NoResearch: true, Genome: noFlight()})[0]
	s = put(b.E, game.Design{Name: "Bar", Hull: game.Small, Parts: parts}, home, g.Cluster.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	if b.lanes(); len(b.E.LaneBlocks) != 0 {
		t.Errorf("at peace: blocks %v", b.E.LaneBlocks)
	}
}

// The Lane Endoscope through Act (the quiet pass wired): the endpoint
// at hand's unknown far star explored; known already, no use (the
// game's own refusal too).
func TestSpecialEndoscope(t *testing.T) {
	parts := []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.LaneEndoscope}
	g, b, home := roundFixture(t, noFlight())
	c := g.Cluster
	k := endpointAlone(c, home, game.LaneEndoscope, func(far int) bool { return !b.E.Knows(far) })
	if k < 0 {
		t.Fatal("fixture: no lone endpoint to an unknown star")
	}
	far := c.Other(c.Stars[home].Lanes[k], home)
	s := put(b.E, game.Design{Name: "Look", Hull: game.Small, Parts: parts}, home, c.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	if b.Act(); !b.E.Knows(far) || !usedNews(b) {
		t.Errorf("unknown: knows %v", b.E.Knows(far))
	}
	// Another lone endpoint: its far star unknown, then visited.
	_, b, _ = roundFixture(t, noFlight())
	b.E.Visit(far)
	s = put(b.E, game.Design{Name: "Look", Hull: game.Small, Parts: parts}, home, c.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	if b.lanes(); usedNews(b) || s.Power != 100 {
		t.Errorf("known: news %+v, power %d", b.News(), s.Power)
	}
	// Unknown, but a hostile at the star: not a quiet day.
	g, b, _ = roundFixture(t, noFlight())
	put(g.Empires[1], game.FixtureRaider, home, game.Vec3{X: 0.3})
	s = put(b.E, game.Design{Name: "Look", Hull: game.Small, Parts: parts}, home, c.Stars[home].System.Lanes[k].Pos)
	s.Power = 100
	if b.lanes(); b.E.Knows(far) || usedNews(b) {
		t.Errorf("hostile there: knows %v", b.E.Knows(far))
	}
}

// A special used inside the rounds (Round's specials) reports the same
// day: Day gathers the bots' News after their Round too, not only after
// Act (else a Disintegrator's kill of the player's ship
// reaches the log a day late, in the next Day's News).
func TestSpecialRoundNewsSameDay(t *testing.T) {
	g, b, _, raider, _ := targetFixture(t, noFlight(), game.FixtureRaider, game.Vec3{X: 0.1}, true, true,
		game.ProtonShaver, game.TonklinMotor, game.Disintegrator)
	_, reps, _ := Day(g, []*Bot{b}) // empire 1 the window's player
	if alive(g, raider) {
		t.Fatal("fixture: the raider survived the day")
	}
	if !slices.ContainsFunc(reps, func(r game.Report) bool { return r.Empire == 1 && r.Event.Kind == game.Destroyed }) {
		t.Errorf("the player's loss is not reported the day it happened: %+v (left in the news: %+v)", reps, b.News())
	}
}
