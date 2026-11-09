package game

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/journal"
)

// The planetary Tractor Beam (tractor.go).

// addBeams puts n Tractor Beams on p's empty surface squares.
func addBeams(t *testing.T, p *Planet, n int) {
	t.Helper()
	for _, sq := range p.Surface {
		if n == 0 {
			return
		}
		if sq.Item == NoItem {
			sq.Item = TractorBeam
			n--
		}
	}
	if n > 0 {
		t.Fatalf("no room for %d more beams", n)
	}
}

// beamSpot is the point d from p's Pos toward the system's centre (away
// from the other planets of the seed-1 home, checked by the pulls' ends).
func beamSpot(p *Planet, d float64) Vec3 { return along(p.Pos, unit(p.Pos, Vec3{}), d) }

// beamSiege is siege with n beams on the home planet and the foe d off it.
func beamSiege(t *testing.T, n int, d float64) (g *Game, foe *Starship, home *Colony) {
	t.Helper()
	g, foe, home = siege(t)
	addBeams(t, home.Planet, n)
	inSpaceAt(foe, beamSpot(home.Planet, d))
	return g, foe, home
}

func sighted(t *testing.T, g *Game, e *Empire, s *Starship) Target {
	t.Helper()
	sg, ok := g.SightingOf(e, s)
	if !ok {
		t.Fatalf("%s not sighted", s.Name)
	}
	return Target{Kind: ShipTarget, Ship: sg}
}

// A pull brings the ship Special1 nearer the planet along the line to its
// Pos; one starting nearer than Special1 + PlanetClear stops PlanetClear
// off it. Reports: the user's, and the same to a foreign target's owner.
func TestTractorPull(t *testing.T) {
	reach := specialReach(Gizmos[GizmoTractorBeam].Special1)
	d0 := TractorRange() - 1e-6
	g, foe, home := beamSiege(t, 2, d0)
	e0, p := g.Empires[0], home.Planet
	reps, err := e0.UseTractor(home, sighted(t, g, e0, foe))
	if err != nil {
		t.Fatal(err)
	}
	if d := dist(foe.Pos, p.Pos); math.Abs(d-(d0-reach)) > 1e-9 {
		t.Errorf("pulled to %.6f off the planet, want %.6f", d, d0-reach)
	}
	if u := unit(p.Pos, foe.Pos); dist(u, unit(p.Pos, Vec3{})) > 1e-9 {
		t.Errorf("off the line: direction %v", u)
	}
	want := p.Name + "'s Tractor Beam: " + foe.Name + " is moved"
	if len(reps) != 2 || reps[0].Empire != 0 || reps[1].Empire != 1 || reps[0].Event.Text != want ||
		reps[1].Event != reps[0].Event || reps[0].Event.Kind != SpecialUsed || reps[0].Event.Star != home.Star {
		t.Errorf("reports %+v, want %q to 0 and 1", reps, want)
	}
	// The second beam: the foe now nearer than reach + PlanetClear.
	if dist(foe.Pos, p.Pos) >= reach+PlanetClear {
		t.Fatalf("fixture: %.3f off", dist(foe.Pos, p.Pos))
	}
	if _, err := e0.UseTractor(home, sighted(t, g, e0, foe)); err != nil {
		t.Fatal(err)
	}
	if d := dist(foe.Pos, p.Pos); math.Abs(d-PlanetClear) > 1e-9 {
		t.Errorf("stopped %.6f off the planet, want PlanetClear %v", d, PlanetClear)
	}
	if foe.Orbit != -1 || home.Tractors != 2 {
		t.Errorf("orbit %d, tractors %d", foe.Orbit, home.Tractors)
	}
}

// Any ship: an own ship (Mine) and a foreign one at peace are pulled too;
// an own ship's pull reports to its user alone.
func TestTractorFriendly(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e0, e1 := g.Empires[0], g.Empires[1]
	home := e0.Colonies[0]
	addBeams(t, home.Planet, 2)
	mine := e0.Spawn(orbitShip, home.Star, home.PlanetIdx)
	inSpaceAt(mine, beamSpot(home.Planet, 1))
	reps, err := e0.UseTractor(home, Target{Kind: ShipTarget, Mine: mine})
	if err != nil || len(reps) != 1 || reps[0].Empire != 0 {
		t.Fatalf("own ship: %v, %+v", err, reps)
	}
	other := e1.Spawn(orbitShip, home.Star, home.PlanetIdx)
	inSpaceAt(other, Vec3{X: home.Planet.Pos.X, Y: home.Planet.Pos.Y + 1, Z: home.Planet.Pos.Z})
	if g.Hostile(e0, e1) {
		t.Fatal("fixture: at war")
	}
	if _, err := e0.UseTractor(home, sighted(t, g, e0, other)); err != nil {
		t.Errorf("a ship at peace: %v", err)
	}
}

// Each refusal, nothing changed.
func TestTractorRefusals(t *testing.T) {
	g, foe, home := beamSiege(t, 1, 0.5)
	e0, e1 := g.Empires[0], g.Empires[1]
	p := home.Planet
	tg := sighted(t, g, e0, foe)
	refuse := func(name string, e *Empire, c *Colony, x Target, want string) {
		t.Helper()
		pos, n := foe.Pos, c.Tractors
		if _, err := e.UseTractor(c, x); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", name, err, want)
		}
		if err := e.CanTractor(c, x); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s (CanTractor): %v, want %q", name, err, want)
		}
		if foe.Pos != pos || c.Tractors != n {
			t.Errorf("%s: changed things", name)
		}
	}
	refuse("another empire's colony", e1, home, tg, "not a colony of this empire")
	bare := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	refuse("no beam", bare.Empires[0], bare.Empires[0].Colonies[0], tg, "has no Tractor Beam")
	refuse("no kind", e0, home, Target{Ship: tg.Ship}, "no such target")
	refuse("both", e0, home, Target{Kind: ShipTarget, Ship: tg.Ship, Mine: e0.Spawn(orbitShip, home.Star, home.PlanetIdx)}, "no such target")
	refuse("another's ship as Mine", e0, home, Target{Kind: ShipTarget, Mine: foe}, "no such target")
	foe.Hull = 0
	refuse("a wreck", e0, home, tg, "no such target")
	foe.Hull = foe.MaxHull()
	foe.Lane = 0
	refuse("in a lane", e0, home, tg, "star lane")
	foe.Lane = -1
	foe.Star++
	refuse("another star", e0, home, tg, "not in the same system")
	foe.Star--
	foe.Orbit = home.PlanetIdx
	refuse("in orbit", e0, home, tg, "in orbit of "+p.Name)
	foe.Orbit = -1
	inSpaceAt(foe, beamSpot(p, TractorRange()+1e-6))
	refuse("out of range", e0, home, tg, "out of range")
	inSpaceAt(foe, beamSpot(p, PlanetClear))
	refuse("alongside", e0, home, tg, "already alongside")
	inSpaceAt(foe, beamSpot(p, PlanetClear/2))
	refuse("within PlanetClear", e0, home, tg, "already alongside")
	inSpaceAt(foe, beamSpot(p, 0.5))
	if _, err := e0.UseTractor(home, tg); err != nil {
		t.Fatal(err)
	}
	refuse("no pull left", e0, home, tg, "no pull left this round")
	// A destroyed ship (gone from its empire's Ships) is stale.
	g2, foe2, home2 := beamSiege(t, 1, 0.5)
	tg2 := sighted(t, g2, g2.Empires[0], foe2)
	g2.Empires[1].Ships = nil
	if err := g2.Empires[0].CanTractor(home2, tg2); err == nil || !strings.Contains(err.Error(), "no such target") {
		t.Errorf("a destroyed ship: %v", err)
	}
}

// The round's cost: a pull drops the colony's armed shots and ArmPlanet
// refuses until the round ends; Round and the close reset the pulls, a
// gunless colony's too.
func TestTractorRound(t *testing.T) {
	g, foe, home := beamSiege(t, 1, 0.5)
	e0, p := g.Empires[0], home.Planet
	const slot = 4
	if err := p.BuildOrbital(slot, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	sg, _ := g.SightingOf(e0, foe)
	if err := e0.ArmPlanet(home, slot, sg); err != nil {
		t.Fatal(err)
	}
	if _, err := e0.UseTractor(home, Target{Kind: ShipTarget, Ship: sg}); err != nil {
		t.Fatal(err)
	}
	sg, _ = g.SightingOf(e0, foe)
	if len(home.Armed) != 0 {
		t.Errorf("armed %v after a pull", home.Armed)
	}
	if err := e0.ArmPlanet(home, slot, sg); err == nil || !strings.Contains(err.Error(), "Tractor Beam used this round") {
		t.Errorf("ArmPlanet after a pull: %v", err)
	}
	hull := foe.Hull
	_, _, log := g.Round(home.Star, nil)
	if foe.Hull != hull || len(log.Shots) != 0 && log.Shots[0].FromPlanet {
		t.Errorf("the planet fired: %+v", log.Shots)
	}
	if home.Tractors != 0 {
		t.Errorf("tractors %d after the round", home.Tractors)
	}
	if err := e0.ArmPlanet(home, slot, sg); err != nil {
		t.Errorf("ArmPlanet the next round: %v", err)
	}

	// Gunless: Round's reset still reaches it; the close too.
	g, foe, home = beamSiege(t, 1, 0.5)
	if _, err := g.Empires[0].UseTractor(home, sighted(t, g, g.Empires[0], foe)); err != nil {
		t.Fatal(err)
	}
	g.Round(home.Star, nil)
	if home.Tractors != 0 {
		t.Errorf("gunless: tractors %d after the round", home.Tractors)
	}
	inSpaceAt(foe, beamSpot(home.Planet, 0.5))
	if _, err := g.Empires[0].UseTractor(home, sighted(t, g, g.Empires[0], foe)); err != nil {
		t.Fatal(err)
	}
	g.EndDay()
	if home.Tractors != 0 {
		t.Errorf("tractors %d after the close", home.Tractors)
	}
}

// A pull records one "tractor" (no "disarm"); a refusal nothing.
func TestJournalTractor(t *testing.T) {
	g, foe, home := beamSiege(t, 1, 0.5)
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf))
	e0 := g.Empires[0]
	if err := home.Planet.BuildOrbital(4, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	sg, _ := g.SightingOf(e0, foe)
	if err := e0.ArmPlanet(home, 4, sg); err != nil {
		t.Fatal(err)
	}
	drain(t, &buf)
	_, err := e0.UseTractor(home, Target{Kind: ShipTarget, Ship: sg})
	if en := one(t, &buf, "UseTractor", "tractor", 0, err); en.Colony != home.Planet.Name || en.Star != home.Star ||
		en.Planet != home.PlanetIdx || en.Target != foe.Name || en.Other != 1 {
		t.Errorf("UseTractor: %+v", en)
	}
	if _, err := e0.UseTractor(home, Target{Kind: ShipTarget, Ship: sg}); err == nil {
		t.Fatal("a second pull")
	}
	none(t, &buf, "UseTractor refused")
}

// The round's pulls are saved: a pull moves its ship at once, a reload
// must not give it back (unlike Armed, which fires only at the round).
func TestTractorsSaved(t *testing.T) {
	c := &Colony{Tractors: 2}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Colony
	if err := json.Unmarshal(b, &back); err != nil || back.Tractors != 2 {
		t.Errorf("reloaded Tractors %d (%v) from %s", back.Tractors, err, b)
	}
}
