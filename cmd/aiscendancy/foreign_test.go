package main

import (
	"os"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// foreignFixture is panelFixture with the Raider moved clear of the
// planets, where a click in the view picks it.
func foreignFixture(t *testing.T) (a *app, v *systemScreen, gun, raider *game.Starship, x, y float64) {
	t.Helper()
	a, v, _, gun, raider = panelFixture(t)
	a.push(v)
	v.cam = sysCamera()
	raider.Pos = game.Vec3{X: gun.Pos.X + 0.2, Y: gun.Pos.Y, Z: gun.Pos.Z + 0.2}
	v.rebuild()
	q := v.cam.Project(raider.Pos)
	if b := pickBody(v.placeAll(), q.X, q.Y); b == nil || b.ship != raider || !sysArea.has(q.X, q.Y) {
		t.Fatalf("fixture: the Raider's point picks %v", b)
	}
	return a, v, gun, raider, q.X, q.Y
}

// A click on a foreign ship in the view inspects it (fsel), our ship
// kept as back; a right click, Esc, or the ship's leaving selects back
// again, unless back left the star; one of ours or a planet selected
// drops both; a list row inspects too; a ship in a lane is not
// inspected; the fire mode's click still arms.
func TestForeignSelect(t *testing.T) {
	a, v, gun, raider, x, y := foreignFixture(t)
	inspect := func(sel *game.Starship) {
		t.Helper()
		v.sel = sel
		if v.click(a, x, y); v.fsel != raider || v.back != sel || v.sel != nil || v.psel != nil || v.placing || v.move.on || v.aim {
			t.Fatalf("click on the Raider: fsel %v back %v sel %v psel %v placing %v", v.fsel, v.back, v.sel, v.psel, v.placing)
		}
		if v.box() != boxForeign || v.listsOn() || v.newsOn() || v.panelOn() {
			t.Fatalf("box %d, lists %v, news %v", v.box(), v.listsOn(), v.newsOn())
		}
	}
	inspect(gun)
	if v.rightClick(); v.fsel != nil || v.back != nil || v.sel != gun || v.placing {
		t.Errorf("right click: fsel %v back %v sel %v placing %v", v.fsel, v.back, v.sel, v.placing)
	}
	inspect(gun)
	if !v.cancel() || v.fsel != nil || v.sel != gun {
		t.Errorf("Esc: fsel %v sel %v", v.fsel, v.sel)
	}
	// A left click on nothing too, not the step's
	// foot: the ship back is selected, no move started.
	inspect(gun)
	if v.actList(a.e, nil); v.fsel != nil || v.back != nil || v.sel != gun || v.move.on {
		t.Errorf("left click on nothing: fsel %v back %v sel %v move %v", v.fsel, v.back, v.sel, v.move.on)
	}
	// A second foreign ship inspected keeps the first back.
	inspect(gun)
	other := a.g.Empires[1].Spawn(foeShip, gun.Star, 0)
	other.Orbit, other.Pos = -1, game.Vec3{X: raider.Pos.X, Y: raider.Pos.Y, Z: raider.Pos.Z - 0.2}
	v.rebuild()
	if v.actList(a.e, shipOf(v, other)); v.fsel != other || v.back != gun {
		t.Errorf("a second foreign ship: fsel %v back %v", v.fsel, v.back)
	}
	if v.rightClick(); v.fsel != nil || v.sel != gun {
		t.Errorf("right click after two: fsel %v sel %v", v.fsel, v.sel)
	}
	inspect(nil)
	if v.rightClick(); v.fsel != nil || v.sel != nil {
		t.Errorf("right click, no back: fsel %v sel %v", v.fsel, v.sel)
	}

	// back gone from the star (here a lane, then another star): not selected.
	inspect(gun)
	gun.Lane = 0
	if v.rightClick(); v.fsel != nil || v.sel != nil {
		t.Errorf("right click, back in a lane: sel %v", v.sel)
	}
	gun.Lane = -1
	inspect(gun)
	home := gun.Star
	gun.Star = (home + 1) % len(a.cl.Stars)
	if v.cancel(); v.sel != nil {
		t.Errorf("Esc, back at another star: sel %v", v.sel)
	}
	gun.Star = home
	v.rebuild()

	// The Raider's leaving drops it, back selected again: a lane, then
	// destroyed.
	inspect(gun)
	raider.Lane = 0
	if v.rebuild(); v.fsel != nil || v.sel != gun {
		t.Errorf("the Raider in a lane: fsel %v sel %v", v.fsel, v.sel)
	}
	raider.Lane = -1
	v.rebuild()
	inspect(gun)
	o := a.g.Empires[1]
	kept := o.Ships
	o.Ships = slices.DeleteFunc(slices.Clone(o.Ships), func(s *game.Starship) bool { return s == raider })
	if v.rebuild(); v.fsel != nil || v.sel != gun {
		t.Errorf("the Raider destroyed: fsel %v sel %v", v.fsel, v.sel)
	}
	o.Ships = kept
	v.rebuild()

	// One of ours or a planet selected drops fsel and back.
	inspect(gun)
	if v.actList(a.e, shipOf(v, gun)); v.sel != gun || v.fsel != nil || v.back != nil {
		t.Errorf("our ship selected: sel %v fsel %v back %v", v.sel, v.fsel, v.back)
	}
	inspect(gun)
	if v.actList(a.e, v.planetBody(0)); v.psel == nil || v.fsel != nil || v.back != nil || v.sel != nil {
		t.Errorf("a planet selected: psel %v fsel %v back %v", v.psel, v.fsel, v.back)
	}
	v.psel = nil
	inspect(gun)
	v.sel = gun
	if v.rebuild(); v.fsel != nil || v.back != nil {
		t.Errorf("ours selected from elsewhere: fsel %v back %v", v.fsel, v.back)
	}

	// A list row inspects; a ship in a lane is not inspected.
	v.sel = nil
	rx, ry, ok := rowPoint(v, shipsTab, bodyOf(v, raider))
	if !ok {
		t.Fatal("no row for the Raider")
	}
	if v.listClick(a, rx, ry); v.fsel != raider || v.back != nil {
		t.Errorf("row click: fsel %v", v.fsel)
	}
	v.rightClick()
	lane := *shipOf(v, raider)
	raider.Lane = 0
	if v.actList(a.e, &lane); v.fsel != nil {
		t.Errorf("a ship in a lane inspected")
	}
	raider.Lane = -1
	v.rebuild()

	// The fire mode: the click on the hostile fires, nothing inspected.
	v.sel = gun
	if err := pressTile(t, v, a, gun, game.MassBarrageGun); err != nil || !v.aim {
		t.Fatalf("weapon tile: %v, aim %v", err, v.aim)
	}
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || len(gun.Armed) != 1 || v.fsel != nil || v.sel != gun {
		t.Errorf("fire: %v, armed %+v, fsel %v, sel %v", err, gun.Armed, v.fsel, v.sel)
	}

	// The real Update with fsel on: no panel of ours read (nil sel).
	v.sel, v.aim = nil, false
	inspect(gun)
	if err := v.Update(a); err != nil || v.fsel != raider {
		t.Errorf("Update: %v, fsel %v", err, v.fsel)
	}
}

// The foreign panel renders through the game's assets at each scan
// level, a tile hovered (a headless Render + Text smoke).
func TestForeignRenders(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a, v, _, raider, x, y := foreignFixture(t)
	a.a = as
	a.preloadGame()
	v.click(a, x, y)
	frame := ebiten.NewImage(screenW, screenH)
	draw := func(level string) {
		t.Helper()
		if v.fsel != raider || v.box() != boxForeign {
			t.Fatalf("%s: fsel %v", level, v.fsel)
		}
		v.Render(a, frame)
		v.Text(a, frame)
	}
	draw("unscanned")
	a.e.ScanAll = true
	v.rebuild()
	if ts := v.foreignTiles(); len(ts) == 0 {
		t.Fatal("ScanAll: no tiles")
	} else {
		v.tile = len(ts) - 1
	}
	draw("x-rayed")
}
