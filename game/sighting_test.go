package game

import (
	"slices"
	"testing"
)

// The partial fog's view of foreign ships: ShipsSeen shows another
// empire's ships out of the lanes at a known star (owner, hull, where),
// never the empire's own, nothing at an unknown star; a ship in a lane is
// not seen at its star but counted by LaneTraffic on a known lane, 0 on
// an unknown one, except for the Oculons (every lane known).
func TestShipsSeen(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa, Oculons}, Average)
	allAtWar(t, g)
	c := g.Cluster
	e0, e1, e2 := g.Empires[0], g.Empires[1], g.Empires[2]
	home := e0.Colonies[0].Star
	far := e1.Colonies[0].Star
	if e0.Knows(far) {
		t.Fatalf("empire 0 knows empire 1's home %d", far)
	}
	mine := e0.Spawn(FixtureGunboat, home, 0)
	raider := e1.Spawn(FixtureRaider, home, 0)
	colony := e1.Spawn(FixtureColonyShip, home, 1)
	away := e1.Spawn(FixtureRaider, far, 0)

	want := []Sighting{
		{Empire: 1, Hull: raider.Design.Hull, Star: home, Pos: raider.Pos, Orbit: raider.Orbit, ship: raider},
		{Empire: 1, Hull: colony.Design.Hull, Star: home, Pos: colony.Pos, Orbit: colony.Orbit, ship: colony},
	}
	if got := g.ShipsSeen(e0, home); !slices.Equal(got, want) {
		t.Errorf("empire 0 at home sees %+v, want %+v", got, want)
	}
	if got := g.Hostiles(e0, home); !slices.Equal(got, want) {
		t.Errorf("hostiles %+v", got)
	}
	if got := g.ShipsSeen(e0, far); got != nil {
		t.Errorf("unknown star %d: %+v", far, got)
	}
	// Empire 1 knows home (its ships spawned there): it sees empire 0's
	// Gunboat and none of its own.
	if got := g.ShipsSeen(e1, home); len(got) != 1 || got[0].ship != mine || got[0].Empire != 0 {
		t.Errorf("empire 1 at home sees %+v", got)
	}
	if got := g.ShipsSeen(e1, far); len(got) != 0 {
		t.Errorf("empire 1 sees its own ship at its home: %+v", got)
	}

	known := c.Stars[home].Lanes[0]
	unknown := -1
	for li := range c.Lanes {
		if !e0.KnowsLane(li) && !e1.KnowsLane(li) {
			unknown = li
			break
		}
	}
	if unknown < 0 {
		t.Fatal("no lane unknown to empires 0 and 1")
	}
	raider.Lane, away.Lane, mine.Lane = known, unknown, known
	if got := g.ShipsSeen(e0, home); len(got) != 1 || got[0].ship != colony {
		t.Errorf("a ship in a lane is seen at its star: %+v", got)
	}
	if n := g.LaneTraffic(e0, known); n != 1 {
		t.Errorf("empire 0, known lane %d: %d ships, want 1 (its own not counted)", known, n)
	}
	if n := g.LaneTraffic(e1, known); n != 1 {
		t.Errorf("empire 1, known lane %d: %d ships, want empire 0's", known, n)
	}
	if n := g.LaneTraffic(e0, unknown); n != 0 {
		t.Errorf("empire 0, unknown lane %d: %d ships", unknown, n)
	}
	if !e2.KnowsLane(unknown) || e2.Knows(c.Lanes[unknown].A) || e2.Knows(c.Lanes[unknown].B) {
		t.Fatalf("the Oculons' lane %d: knows lane %v", unknown, e2.KnowsLane(unknown))
	}
	if n := g.LaneTraffic(e2, unknown); n != 1 {
		t.Errorf("Oculons, lane %d: %d ships, want 1", unknown, n)
	}
}

// The bot's aim through a Sighting is CanFire's on the ship, and a shot
// armed at the Sighting hits the ship.
func TestFireAtSighting(t *testing.T) {
	g, sh, tg := duel(fightFixture, fightFixture)
	gun := part(fightFixture, MassBarrageGun)
	tg.Shields = false
	seen := g.Hostiles(g.Empires[0], sh.Star)
	if len(seen) != 1 || seen[0].ship != tg {
		t.Fatalf("hostiles %+v", seen)
	}
	if err := sh.CanFireAt(g.Cluster, gun, seen[0]); err != nil {
		t.Fatal(err)
	}
	hull := tg.Hull
	if err := g.Empires[0].Arm(sh, gun, seen[0]); err != nil {
		t.Fatal(err)
	}
	if acted, _, _ := g.Round(sh.Star, nil); !acted || tg.Hull != hull-Gizmos[MassBarrageGun].Level {
		t.Errorf("armed at the Sighting: acted %v, hull %d of %d", acted, tg.Hull, hull)
	}
	if err, want := sh.CanFireAt(g.Cluster, gun, seen[0]), sh.CanFire(g.Cluster, gun, tg); err == nil || err.Error() != want.Error() {
		t.Errorf("after the shot: %v, want %v", err, want)
	}
}

// Scanners (docs/rules.md "Scanners"): a foreign ship within the range
// of one of e's scanners at its star carries a Scan with its design and
// status; out of range, behind a Cloaker, or with the scanner in a lane
// it does not.
func TestScan(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e0, e1 := g.Empires[0], g.Empires[1]
	home := e0.Colonies[0].Star
	eye := e0.Spawn(Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, TonklinFrequencyAnalyzer, XRayMegaglasses}}, home, 0) // the parts: TestXRay
	raider := e1.Spawn(FixtureRaider, home, 0)
	r := float64(Gizmos[TonklinFrequencyAnalyzer].Range) * RangeUnit
	if eye.ScanRange() != r {
		t.Fatalf("ScanRange %v, want %v", eye.ScanRange(), r)
	}
	if (&Starship{Design: FixtureRaider}).ScanRange() != 0 {
		t.Error("a ship without a scanner scans")
	}
	seen := func() *Scan {
		for _, sg := range g.ShipsSeen(e0, home) {
			if sg.ship == raider {
				return sg.Scan
			}
		}
		t.Fatal("raider not seen")
		return nil
	}
	raider.Pos = eye.Pos
	raider.Pos.X += r // at the edge: in range
	sc := seen()
	if sc == nil || !slices.Equal(sc.Design.Parts, FixtureRaider.Parts) || sc.HullPts != raider.Hull || sc.Shields != raider.Shields ||
		sc.Energy != raider.Power {
		t.Fatalf("in range: %+v", sc)
	}
	sc.Design.Parts[0] = Cloaker
	if raider.Design.Parts[0] == Cloaker {
		t.Error("the Scan's design aliases the ship's")
	}
	raider.Power--
	if sc := seen(); sc == nil || sc.Energy != raider.Power {
		t.Errorf("energy spent: %+v, want %d", sc, raider.Power)
	}
	if raider.ShieldsDown = true; !seen().ShieldsDown || sc.ShieldsDown {
		t.Errorf("knocked down: the Scan's ShieldsDown %v (before %v)", seen().ShieldsDown, sc.ShieldsDown)
	}
	raider.ShieldsDown = false
	if sc := seen(); sc.Absorb != raider.Absorb() || sc.ShieldHits != raider.ShieldHits() || sc.ShieldHits == 0 {
		t.Errorf("the Scan's absorb %d hits %d, the ship's %d %d", sc.Absorb, sc.ShieldHits, raider.Absorb(), raider.ShieldHits())
	}
	raider.Pos.X += 1
	if sc := seen(); sc != nil {
		t.Errorf("out of range: %+v", sc)
	}
	raider.Pos.X -= 1
	eye.Lane = 0
	if sc := seen(); sc != nil {
		t.Errorf("scanner in a lane: %+v", sc)
	}
	eye.Lane = -1
	raider.Design.Parts = append(slices.Clone(raider.Design.Parts), Cloaker)
	if sc := seen(); sc != nil {
		t.Errorf("cloaked: %+v", sc)
	}
}

// An Observation Installation on one of e's colonies scans the foreign
// ships at its star within ObservationRange of the planet, a Cloaker
// aboard still hiding the ship; beyond the range, or from a lane, not.
func TestObservationInstallation(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e0, e1 := g.Empires[0], g.Empires[1]
	col := e0.Colonies[0]
	raider := e1.Spawn(FixtureRaider, col.Star, 0)
	raider.Orbit = -1
	seen := func() *Scan {
		for _, sg := range g.ShipsSeen(e0, col.Star) {
			if sg.ship == raider {
				return sg.Scan
			}
		}
		t.Fatal("raider not seen")
		return nil
	}
	r := ObservationRange * RangeUnit
	raider.Pos = col.Planet.Pos
	raider.Pos.X += r
	if seen() != nil {
		t.Fatal("scanned without an installation")
	}
	at, ok := col.Planet.BestCell(ObservationInstallation, Base)
	if !ok {
		t.Fatal("no cell")
	}
	if err := col.Planet.Build(at, ObservationInstallation); err != nil {
		t.Fatal(err)
	}
	if sc := seen(); sc == nil || sc.HullPts != raider.Hull {
		t.Errorf("at the edge of the range: %+v", sc)
	}
	raider.Pos.X += 1
	if sc := seen(); sc != nil {
		t.Errorf("out of range: %+v", sc)
	}
	raider.Pos.X -= 1
	raider.Design.Parts = append(slices.Clone(raider.Design.Parts), Cloaker)
	if sc := seen(); sc != nil {
		t.Errorf("cloaked: %+v", sc)
	}
}
