package main

import (
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The planet's Tractor Beam on the system display (sysplanet.go): its
// tile, the tractor mode, its pulls, in and out of combat mode.

// beams puts n Tractor Beams on p's empty surface squares.
func beams(t *testing.T, p *game.Planet, n int) {
	t.Helper()
	for _, sq := range p.Surface {
		if n > 0 && sq.Item == game.NoItem {
			sq.Item, n = game.TractorBeam, n-1
		}
	}
	if n > 0 {
		t.Fatal("no room for the beams")
	}
}

// offPlanet is the point d from p's Pos toward the system's centre, then
// lifted by dy.
func offPlanet(p *game.Planet, d, dy float64) game.Vec3 {
	n := vecDist(p.Pos, game.Vec3{})
	return game.Vec3{X: p.Pos.X * (1 - d/n), Y: p.Pos.Y*(1-d/n) + dy, Z: p.Pos.Z * (1 - d/n)}
}

// beamTile is the selected planet's Tractor Beam tile, ok false none.
func beamTile(v *systemScreen) (planetTile, bool) {
	for _, pt := range v.ptiles() {
		if pt.slot < 0 {
			return pt, true
		}
	}
	return planetTile{}, false
}

// pressBeam presses the beam's tile.
func pressBeam(t *testing.T, v *systemScreen, a *app) {
	t.Helper()
	pt, ok := beamTile(v)
	if !ok {
		t.Fatal("no beam tile")
	}
	if err := v.planetClick(a, float64(pt.r.X0+pt.r.X1)/2, float64(pt.r.Y0+pt.r.Y1)/2); err != nil || !v.aim || !v.ptract {
		t.Fatalf("beam press: %v, aim %v, tractor %v", err, v.aim, v.ptract)
	}
}

// The tile shows on the player's colony with a beam (last of the
// structures, pulls left in its text), never in the orbit's tiles; its
// press enters the tractor mode, a click on a ship pulls it (where
// TractorEnd said), the player's own included, then refuses once the
// pulls are spent; Esc, a right click and a weapon's press end the mode.
func TestPlanetTractor(t *testing.T) {
	a, v, _, gun, raider := panelFixture(t)
	home := a.e.Colonies[0]
	p := home.Planet
	defences(t, p, homeDefences)
	v.clickBody(a, v.planetBody(home.PlanetIdx))
	if _, ok := beamTile(v); ok {
		t.Fatal("a beam tile without a beam")
	}
	beams(t, p, 2)
	pt, ok := beamTile(v)
	if !ok || pt.band != 2 || pt.it != game.TractorBeam || pt.r.Y1 > panelBottom {
		t.Fatalf("beam tile %+v, %v", pt, ok)
	}
	for _, o := range planetTiles(p) {
		if o.slot < 0 {
			t.Error("the orbit's tiles hold the beam")
		}
		if o.band == 2 && (o.r.Y0 > pt.r.Y0 || o.r.Y0 == pt.r.Y0 && o.r.X0 >= pt.r.X0) {
			t.Errorf("the beam %v not after %s %v", pt.r, o.it, o.r)
		}
	}
	if txt := v.planetTileText(pt); !strings.Contains(txt, "2 pulls left") {
		t.Errorf("tile text %q", txt)
	}

	pressBeam(t, v, a)
	if !strings.HasPrefix(v.planetFoot(), "Tractor Beam") {
		t.Errorf("foot %q", v.planetFoot())
	}
	raider.Orbit, raider.Pos = -1, offPlanet(p, 1, 0)
	gun.Orbit, gun.Pos = -1, offPlanet(p, 1, 0.3)
	v.rebuild()
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("aim at the Raider: %v %v", b, live)
	}
	sg, _ := a.g.SightingOf(a.e, raider)
	want, err := a.e.TractorEnd(home, game.Target{Kind: game.ShipTarget, Ship: sg})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || v.aim || raider.Pos != want || home.Tractors != 1 {
		t.Fatalf("pull the Raider: %v, aim %v, at %v want %v, tractors %d", err, v.aim, raider.Pos, want, home.Tractors)
	}
	if !v.msgOK || !strings.Contains(v.msg, p.Name+"'s Tractor Beam") {
		t.Errorf("message %q (ok %v)", v.msg, v.msgOK)
	}
	if txt := v.planetTileText(pt); !strings.Contains(txt, "1 pull left") {
		t.Errorf("tile text after a pull %q", txt)
	}

	pressBeam(t, v, a)
	want, err = a.e.TractorEnd(home, game.Target{Kind: game.ShipTarget, Mine: gun})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.aimClick(a, shipOf(v, gun)); err != nil || gun.Pos != want || home.Tractors != 2 {
		t.Fatalf("pull our Gunboat: %v, at %v want %v", err, gun.Pos, want)
	}
	pressBeam(t, v, a)
	v.hover = shipOf(v, raider)
	if _, live := v.aimTarget(); live {
		t.Error("spent: the line is live")
	}
	if err := v.aimClick(a, shipOf(v, raider)); err == nil || !strings.Contains(err.Error(), "no pull left") {
		t.Errorf("spent: %v", err)
	}

	pressBeam(t, v, a)
	if v.cancel(); v.aim || v.psel != p {
		t.Errorf("Esc: aim %v, psel %v", v.aim, v.psel)
	}
	pressBeam(t, v, a)
	if v.rightClick(); v.aim {
		t.Error("right click: aim kept")
	}
	pressBeam(t, v, a)
	if err := pressSlot(t, v, a, 0); err != nil || !v.aim || v.ptract {
		t.Errorf("a weapon's press: %v, aim %v, tractor %v", err, v.aim, v.ptract)
	}
	// The beam spent the round: the weapon's shot is refused.
	v.hover = shipOf(v, raider)
	if err := v.aimClick(a, shipOf(v, raider)); err == nil || !strings.Contains(err.Error(), "Tractor Beam used") {
		t.Errorf("fire after a pull: %v", err)
	}
}

// In combat mode: a pull, then the next round gives the pulls back.
func TestPlanetTractorRounds(t *testing.T) {
	a, v, _ := roundApp(t)
	home := a.e.Colonies[0]
	beams(t, home.Planet, 1)
	if !v.inCombat(a) {
		t.Fatal("no combat mode")
	}
	foe := a.g.Empires[1].Ships[0]
	foe.Orbit, foe.Pos = -1, offPlanet(home.Planet, 1, 0)
	v.sel = nil
	v.rebuild()
	v.clickBody(a, v.planetBody(home.PlanetIdx))
	pressBeam(t, v, a)
	if err := v.aimClick(a, shipOf(v, foe)); err != nil || home.Tractors != 1 {
		t.Fatalf("pull: %v, tractors %d", err, home.Tractors)
	}
	a.stepRound(v)
	if home.Tractors != 0 || !strings.Contains(v.planetTileText(mustBeam(t, v)), "1 pull left") {
		t.Errorf("after the round: tractors %d", home.Tractors)
	}
}

func mustBeam(t *testing.T, v *systemScreen) planetTile {
	t.Helper()
	pt, ok := beamTile(v)
	if !ok {
		t.Fatal("no beam tile")
	}
	return pt
}
