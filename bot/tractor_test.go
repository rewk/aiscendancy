package bot

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The bot's Tractor Beams (tractor.go).

// beamColony gives col's planet one Tractor Beam on an empty square.
func beamColony(t *testing.T, col *game.Colony) {
	t.Helper()
	for _, sq := range col.Planet.Surface {
		if sq.Item == game.NoItem {
			sq.Item = game.TractorBeam
			return
		}
	}
	t.Fatal("no empty square")
}

// toward is the point d from p toward the system's centre.
func toward(p game.Vec3, d float64) game.Vec3 {
	n := dist(p, game.Vec3{})
	return game.Vec3{X: p.X - d*p.X/n, Y: p.Y - d*p.Y/n, Z: p.Z - d*p.Z/n}
}

// A ship of mine below RepairAt × MaxHull in the beam's range is pulled
// home (where TractorEnd said); a sound one is left alone.
func TestTractorDamagedShip(t *testing.T) {
	g, b, home := roundFixture(t, noFlight())
	col := b.E.Colonies[0]
	beamColony(t, col)
	p := col.Planet
	put(g.Empires[1], unarmedScout, home, game.Vec3{X: -1.4}) // a hostile: the round runs
	at := toward(p.Pos, game.TractorRange()-0.01)
	sound := put(b.E, unarmedScout, home, at)
	if b.Round(home); sound.Pos != at || col.Tractors != 0 {
		t.Fatalf("a sound ship pulled: %v, tractors %d", sound.Pos, col.Tractors)
	}
	sound.Hull = int(b.genes().RepairAt*float64(sound.MaxHull())) - 1
	if sound.Hull <= 0 {
		t.Fatal("fixture: MaxHull too small")
	}
	want, err := b.E.TractorEnd(col, game.Target{Kind: game.ShipTarget, Mine: sound})
	if err != nil {
		t.Fatal(err)
	}
	if b.Round(home); sound.Pos != want || col.Tractors != 1 {
		t.Errorf("damaged: at %v, want %v; tractors %d", sound.Pos, want, col.Tractors)
	}
}

// twoColonies is roundFixture with a second colony of empire 0 at its
// home star (c1) holding an Orbital Missile Base in its last slot, a
// Tractor Beam on the home's planet (c0), and empire 1's unarmed foe.
func twoColonies(t *testing.T) (g *game.Game, b *Bot, home int, c0, c1 *game.Colony, foe *game.Starship) {
	t.Helper()
	g, b, home = roundFixture(t, noFlight())
	e := b.E
	c0 = e.Colonies[0]
	beamColony(t, c0)
	for pl, p := range g.Cluster.Stars[home].System.Planets {
		if p.Owner < 0 && e.Found(e.Spawn(game.FixtureColonyShip, home, pl)) == nil {
			break
		}
	}
	if len(e.Colonies) < 2 {
		t.Fatal("fixture: no second colony")
	}
	c1 = e.Colonies[1]
	if err := c1.Planet.BuildOrbital(game.OrbitSlots-1, game.OrbitalMissileBase); err != nil {
		t.Fatal(err)
	}
	return g, b, home, c0, c1, put(g.Empires[1], unarmedScout, home, game.Vec3{X: 9})
}

// baseRange is the Orbital Missile Base's reach.
var baseRange = float64(game.OrbitalDefences[game.OrbitalMissileBase].Range) * game.RangeUnit

// foeTarget is e's Target on foe.
func foeTarget(g *game.Game, e *game.Empire, foe *game.Starship) game.Target {
	sg, _ := g.SightingOf(e, foe)
	return game.Target{Kind: game.ShipTarget, Ship: sg}
}

// intoRange puts foe on a point (a 0.05 grid around beam's planet) in
// beam's range and keep-free, from which beam's pull ends within gun's
// base range (TractorEnd the oracle): out of it now (in false) or in it
// already (in true); false none.
func intoRange(g *game.Game, e *game.Empire, beam, gun *game.Colony, foe *game.Starship, keep game.Vec3, in bool) bool {
	const step = 0.05
	b, gp := beam.Planet.Pos, gun.Planet.Pos
	n := int(game.TractorRange() / step)
	for i := -n; i <= n; i++ {
		for j := -n; j <= n; j++ {
			for k := -n; k <= n; k++ {
				at := game.Vec3{X: b.X + float64(i)*step, Y: b.Y + float64(j)*step, Z: b.Z + float64(k)*step}
				d := dist(at, gp)
				if dist(at, game.Vec3{}) > game.SystemReach || dist(at, keep) < 0.2 || (d <= baseRange-0.01) != in || d > baseRange-0.01 && d <= baseRange+0.01 ||
					d > baseRange+game.TractorRange() || dist(at, b) > game.TractorRange()-0.01 {
					continue
				}
				foe.Pos = at
				if end, err := e.TractorEnd(beam, foeTarget(g, e, foe)); err == nil && dist(end, gp) <= baseRange-0.01 {
					return true
				}
			}
		}
	}
	return false
}

// A hostile the pull brings within another colony's weapon range (out of
// it now) is pulled, then that weapon arms at it: the sightings were read
// again. One in range already (and kept in range by a pull) is left
// alone.
func TestTractorIntoGuns(t *testing.T) {
	g, b, home, c0, c1, foe := twoColonies(t)
	e := b.E
	if !intoRange(g, e, c0, c1, foe, game.Vec3{X: 9}, false) {
		t.Fatal("fixture: no spot")
	}
	start := foe.Pos
	want, _ := e.TractorEnd(c0, foeTarget(g, e, foe))
	b.Round(home)
	if foe.Pos != want || c0.Tractors != 1 {
		t.Fatalf("foe at %v (from %v), want %v; tractors %d", foe.Pos, start, want, c0.Tractors)
	}
	if len(c1.Armed) != 1 || c1.Armed[0].Part != game.OrbitSlots-1 {
		t.Errorf("c1 armed %+v, want its base at the foe", c1.Armed)
	}

	// The next round, the foe in range, and kept in range by a pull.
	e.DisarmPlanet(c1)
	c0.Tractors = 0
	if !intoRange(g, e, c0, c1, foe, game.Vec3{X: 9}, true) {
		t.Fatal("fixture: no spot in range")
	}
	before := foe.Pos
	if b.Round(home); foe.Pos != before || c0.Tractors != 0 {
		t.Errorf("a foe in range pulled to %v, tractors %d", foe.Pos, c0.Tractors)
	}
}

// A colony that pulled this round has no shots: another colony's beam
// does not pull a hostile into its range. Here c0's beam brings a damaged
// ship home first, then c1's beam passes on the foe.
func TestTractorSpentGuns(t *testing.T) {
	g, b, home, c0, c1, foe := twoColonies(t)
	e := b.E
	beamColony(t, c1)
	if err := c0.Planet.BuildOrbital(game.OrbitSlots-1, game.OrbitalMissileBase); err != nil {
		t.Fatal(err)
	}
	hurt := put(e, unarmedScout, home, toward(c0.Planet.Pos, game.TractorRange()-0.01))
	hurt.Hull = 1
	if !intoRange(g, e, c1, c0, foe, hurt.Pos, false) {
		t.Fatal("fixture: no spot")
	}
	before := foe.Pos
	if b.Round(home); c0.Tractors != 1 || c1.Tractors != 0 || foe.Pos != before {
		t.Errorf("tractors %d/%d, foe at %v from %v", c0.Tractors, c1.Tractors, foe.Pos, before)
	}
}
