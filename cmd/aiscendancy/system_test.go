package main

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/journal"
	"github.com/rewk/aiscendancy/ui"
)

func TestGridLines(t *testing.T) {
	ls := gridLines()
	if len(ls) != 38 {
		t.Fatalf("%d segments, want 38 (19 per direction)", len(ls))
	}
	for i, l := range ls {
		for _, p := range l {
			if r := math.Hypot(p.X, p.Z); math.Abs(r-1) > 1e-12 || p.Y != 0 {
				t.Errorf("segment %d: end %v at radius %v", i, p, r)
			}
		}
	}
}

func TestBodySprite(t *testing.T) {
	cases := []struct {
		b     body
		bank  string
		frame int
	}{
		{body{kind: planetBody, planet: &game.Planet{Type: game.Cornucopia, Size: 5, Owner: -1}}, "planets", 54},
		{body{kind: planetBody, planet: &game.Planet{Type: game.Husk, Size: 1, Owner: -1}}, "planets", 0},
		{body{kind: planetBody, planet: &game.Planet{Type: game.Eden, Size: 2, Owner: -1}}, "planets", 16},
		{body{kind: laneBody, lane: &game.Lane{}}, "lanes", 0},
		{body{kind: laneBody, lane: &game.Lane{Red: true}}, "lanes", 1},
		{body{kind: starBody, star: 12}, "suns", 12},
	}
	for _, c := range cases {
		if bank, frame := c.b.sprite(); bank != c.bank || frame != c.frame {
			t.Errorf("%+v: %s %d, want %s %d", c.b, bank, frame, c.bank, c.frame)
		}
	}
}

// placeAll paints back to front, and pickBody picks the nearest of overlapping
// discs: a body at +Z (near side) beats the star it covers.
func TestPlaceAllAndHit(t *testing.T) {
	s := &game.StarSystem{Name: "x", Planets: []*game.Planet{
		{Type: game.Eden, Size: 5, Pos: game.Vec3{Z: 0.05}, Owner: -1},
		{Type: game.Eden, Size: 5, Pos: game.Vec3{Z: -0.9}, Owner: -1},
	}, Lanes: []game.Lane{{Pos: game.Vec3{X: 1}}}}
	v := &systemScreen{sys: s, cam: cam.Default(200, 200)}
	v.bodies = systemBodies(s)
	ps := v.placeAll()
	for i := 1; i < len(ps); i++ {
		if ps[i].pt.Depth > ps[i-1].pt.Depth {
			t.Fatalf("not back to front at %d: %v after %v", i, ps[i].pt.Depth, ps[i-1].pt.Depth)
		}
	}
	o := v.cam.Project(game.Vec3{})
	if b := pickBody(ps, o.X, o.Y); b == nil || b.planet != s.Planets[0] {
		t.Errorf("hit at the origin: %+v, want the near planet", b)
	}
	if b := pickBody(ps, 0, 0); b != nil {
		t.Errorf("hit in the void: %+v", b)
	}
}

// shipScreen is seed 1 Average's home system with two fixture ships in
// orbit of the home world, the system screen entered on it.
func shipScreen(t *testing.T) (*game.Empire, *systemScreen, []*game.Starship) {
	t.Helper()
	g := game.NewGame(2, []game.Species{game.Snovemdomas}, game.Average)
	e := g.Empires[0]
	home := e.Cluster.Home
	ships := []*game.Starship{e.Spawn(fixtureShip, home, 0), e.Spawn(fixtureShip, home, 0)}
	a := testApp(g)
	v := newSystemScreen(home)
	v.Enter(a)
	return e, v, ships
}

// clickAt is a left click at (x, y) in the view, as Update gives it.
func clickAt(v *systemScreen, e *game.Empire, x, y float64) {
	v.click(&app{e: e}, x, y)
}

// shipOf is the body of ship sh in v.bodies, nil when not listed.
func shipOf(v *systemScreen, sh *game.Starship) *body {
	for i := range v.bodies {
		if v.bodies[i].kind == shipBody && v.bodies[i].ship == sh {
			return &v.bodies[i]
		}
	}
	return nil
}

func TestShipBodies(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	p := v.sys.Planets[0]
	for _, sh := range ships {
		b := shipOf(v, sh)
		if b == nil {
			t.Fatalf("%s not listed", sh.Name)
		}
		if b.pos != p.Pos || !orbitHidden(b) || b.orbit != 0 {
			t.Errorf("%s at %v, hidden %v: want hidden in planet 0's orbit at %v", sh.Name, b.pos, orbitHidden(b), p.Pos)
		}
		if sh.Pos != p.Pos {
			t.Errorf("%s: the model's Pos moved to %v", sh.Name, sh.Pos)
		}
		if bank, frame := b.sprite(); bank != shipBank(e.Species) || frame != int(game.Small) {
			t.Errorf("sprite %s %d", bank, frame)
		}
	}
	if shipBank(0) != "smship00" || shipBank(game.NumSpecies-1) != "smship20" {
		t.Errorf("banks %s..%s, want smship00..smship20", shipBank(0), shipBank(game.NumSpecies-1))
	}
	// A ship in a lane out of this system is gone from it, unless its
	// hop's anim still runs; a ship in another system is not listed.
	ships[1].Orbit, ships[1].Lane, ships[1].Pos = -1, c.Stars[c.Home].Lanes[0], v.sys.Lanes[0].Pos
	far := e.Spawn(fixtureShip, c.Other(c.Stars[c.Home].Lanes[0], c.Home), 0)
	v.rebuild()
	if b := shipOf(v, ships[1]); b != nil {
		t.Errorf("ship in the lane: %+v", b)
	}
	v.anim.ship = ships[1]
	v.rebuild()
	if shipOf(v, ships[1]) == nil {
		t.Error("ship hopping into the lane: not drawn")
	}
	v.anim.ship = nil
	v.rebuild()
	if shipOf(v, far) != nil {
		t.Error("a ship of another system is listed")
	}
}

func TestShipClicks(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh := ships[0]
	// In space (a ship in orbit is the planet screen's: hidden here).
	ships[0].Orbit, ships[0].Pos = -1, game.Vec3{X: 0.2, Z: -0.5}
	ships[1].Orbit, ships[1].Pos = -1, game.Vec3{X: -0.3, Z: -0.5}
	v.rebuild()
	// The ships can be picked where they are drawn.
	v.cam = sysCamera()
	ps := v.placeAll()
	picked := false
	for _, p := range ps {
		if b := pickBody(ps, p.pt.X, p.pt.Y); p.kind == shipBody && b != nil && b.kind == shipBody {
			picked = true
		}
	}
	if !picked {
		t.Error("no ship can be picked at its own position")
	}

	if push, err := v.act(e, shipOf(v, sh)); push != nil || err != nil || v.sel != sh {
		t.Fatalf("select: %v %v sel %v", push, err, v.sel)
	}
	if v.act(e, shipOf(v, sh)); v.sel != sh {
		t.Fatal("a click on the selected ship keeps it selected (no toggle)")
	}
	v.act(e, shipOf(v, ships[1]))
	v.act(e, shipOf(v, sh))
	if v.sel != sh {
		t.Fatal("a click on another ship does not move the selection")
	}

	var planet1 *body
	for i := range v.bodies {
		if v.bodies[i].kind == planetBody && v.bodies[i].idx == 1 {
			planet1 = &v.bodies[i]
		}
	}
	// A planet click with a ship selected orders its orbit (targetOrder),
	// the ship dropped once ordered (out of combat); a list row inspects
	// instead (actList).
	if push, err := v.actList(e, planet1); push != nil || err != nil || sh.Order.Kind != game.NoOrder || v.sel != nil || v.psel != planet1.planet {
		t.Errorf("planet 1 row with a ship selected: %v %v order %+v, sel %v psel %v", push, err, sh.Order, v.sel, v.psel)
	}
	if v.act(e, shipOf(v, sh)); v.sel != sh || v.psel != nil {
		t.Fatalf("a ship click after the planet: sel %v psel %v", v.sel, v.psel)
	}
	tmp := *sh
	wantPlanet := game.Order{Kind: game.GoToPlanet, Planet: planet1.idx}
	wantErr := tmp.Check(c, wantPlanet)
	wantSel := sh
	if wantErr == nil {
		wantSel = nil
	}
	if push, err := v.act(e, planet1); push != nil || (err == nil) != (wantErr == nil) || v.sel != wantSel || v.psel != nil {
		t.Errorf("planet 1 click with a ship selected: %v %v (Check: %v) sel %v psel %v", push, err, wantErr, v.sel, v.psel)
	} else if wantErr == nil && sh.Order != wantPlanet {
		t.Errorf("planet click order %+v, want %+v", sh.Order, wantPlanet)
	}
	v.sel = sh
	if err := v.order(e, game.Order{}); err != nil || v.sel != sh {
		t.Fatal(err, "cancel drops the ship")
	}
	lanes := 0
	for i := range v.bodies {
		b := &v.bodies[i]
		if b.kind != laneBody || b.lane.To < 0 {
			continue
		}
		lanes++
		v.sel = sh
		if push, err := v.act(e, b); push != nil || err != nil || sh.Order != (game.Order{Kind: game.GoToStar, Star: b.lane.To}) || v.sel != nil {
			t.Errorf("order through the lane to %d: %v %v %+v", b.lane.To, push, err, sh.Order)
		}
	}
	if lanes != len(c.Stars[c.Home].Lanes) {
		t.Errorf("%d lane endpoints ordered, %d lanes", lanes, len(c.Stars[c.Home].Lanes))
	}
	v.fail(errors.New("x"))
	if v.msg != "x" || v.msgUntil <= v.tick {
		t.Errorf("fail: msg %q until %d at tick %d", v.msg, v.msgUntil, v.tick)
	}
	v.sel = sh
	if err := v.order(e, game.Order{}); err != nil || sh.Order.Kind != game.NoOrder || v.sel != sh {
		t.Errorf("cancel: %v %+v", err, sh.Order)
	}
	if push, _ := v.act(e, nil); push != nil || v.sel != sh {
		t.Error("act on nothing leaves the ship selected (the view's click plants the height step)")
	}
	v.sel = nil

	// No selection: a planet's first click selects it, the second opens
	// its screen; the lane screens; no order given.
	if push, _ := v.act(e, planet1); push != nil || v.psel != planet1.planet || v.box() != boxPlanet {
		t.Errorf("planet click without a selection: %#v, psel %v", push, v.psel)
	}
	if push, _ := v.act(e, planet1); push == nil || push.(*planetScreen).idx != 1 || push.(*planetScreen).star != c.Home {
		t.Errorf("click on the selected planet: %#v", push)
	}
	if v.act(e, nil); v.psel != nil {
		t.Error("a click on nothing keeps the planet selected")
	}
	for i := range v.bodies {
		if b := &v.bodies[i]; b.kind == laneBody && b.lane.To >= 0 {
			e.Visit(b.lane.To) // an unexplored far star opens nothing (TestSystemLaneToUnexplored)
			if push, _ := v.act(e, b); push == nil || push.(*systemScreen).star != b.lane.To {
				t.Errorf("lane click without a selection: %#v", push)
			}
		}
	}
	if sh.Order.Kind != game.NoOrder {
		t.Errorf("an order without a selection: %+v", sh.Order)
	}
	if v.order(e, game.Order{Kind: game.GoToPoint}) != nil || sh.Order.Kind != game.NoOrder {
		t.Error("an order without a selection")
	}
}

// The selection drops once the ship enters its lane (no interaction in a
// lane), the model still holding it at this star.
func TestSelectionLeaves(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh := ships[0]
	v.act(e, shipOf(v, sh))
	to := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	// The order hops, the anims chain while the power lasts (the ETA
	// counts today's power as spent).
	if err := v.order(e, game.Order{Kind: game.GoToStar, Star: to}); err != nil || v.sel != nil {
		t.Fatal(err, "the order keeps the ship selected")
	}
	chain(v, e)
	v.sel = sh // picked again
	eta, _ := sh.ETA(c, e.Day)
	entered := false
	for range eta - 1 {
		e.EndDay()
		v.rebuild()
		switch {
		case sh.Lane < 0 && v.sel != sh:
			t.Fatalf("day %d: deselected while still in space", e.Day)
		case sh.Lane >= 0 && (v.sel != nil || v.move.on):
			t.Fatalf("day %d: selected in lane %d", e.Day, sh.Lane)
		case sh.Lane >= 0:
			entered = true
			if v.act(e, shipOf(v, sh)); v.sel != nil {
				t.Fatalf("day %d: selected again in the lane", e.Day)
			}
		}
	}
	e.EndDay()
	v.rebuild()
	if !entered || sh.Star != to || v.sel != nil || shipOf(v, sh) != nil {
		t.Errorf("day %d: entered %v, star %d, sel %v", e.Day, entered, sh.Star, v.sel)
	}
}

func TestDescribeShip(t *testing.T) {
	e, v, ships := shipScreen(t)
	sh := ships[0]
	b := shipOf(v, sh)
	got := v.describe(e, b)
	pw := fmt.Sprintf("cargo 2\npower %d/%d\n", sh.Design.PowerPerDay(), sh.Design.PowerPerDay())
	for _, w := range []string{sh.Name, "Small: Proton Shaver", "Colonizer x2", pw, "orbit of " + v.sys.Planets[0].Name} {
		if !strings.Contains(got, w) {
			t.Errorf("%q lacks %q", got, w)
		}
	}
	if strings.Contains(got, "X cancel") {
		t.Error("key help on an unselected ship")
	}
	v.sel = sh
	e.Order(sh, game.Order{Kind: game.GoToPlanet, Planet: 1})
	days, _ := sh.ETA(e.Cluster, e.Day)
	got = v.describe(e, b)
	if w := fmt.Sprintf("-> %s, %s", v.sys.Planets[1].Name, etaText(days)); !strings.Contains(got, w) || !strings.Contains(got, "X cancel  F fire") || strings.Contains(got, "colonise") {
		t.Errorf("%q lacks %q or the keys", got, w)
	}
	for i, l := range strings.Split(got, "\n") {
		if len(l) > infoCols {
			t.Errorf("line %d %q: too wide for the info box", i, l)
		}
	}
}

func TestWrap(t *testing.T) {
	for _, c := range []struct {
		s    string
		n    int
		want string
	}{
		{"Snovemdomas Prime is already inhabited", infoCols, "Snovemdomas Prime is already\ninhabited"},
		{"short", 30, "short"},
		{"abcdefgh ij", 4, "abcdefgh\nij"},
	} {
		if got := wrap(c.s, c.n); got != c.want {
			t.Errorf("wrap(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

// The free move, fed screen positions projected through the screen's
// camera: a click on nothing, a ship selected, plants the foot, the
// cross tracks the cursor up the vertical, a second click orders
// GoToPoint.
func TestMoveMode(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh := ships[0]
	v.cam = sysCamera()
	nearV := func(a, b game.Vec3) bool {
		return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6 && math.Abs(a.Z-b.Z) < 1e-6
	}
	at := func(p game.Vec3) (float64, float64) { q := v.cam.Project(p); return q.X, q.Y }

	// A click on nothing with no selection plants nothing; on the ship it
	// selects it, no step yet.
	if clickAt(v, e, 100, 100); v.sel != nil || v.move.on {
		t.Fatalf("click, nothing selected: sel %v mode %+v", v.sel, v.move)
	}
	if v.act(e, shipOf(v, sh)); v.sel != sh || v.move.on {
		t.Fatalf("click on the ship: sel %v mode %+v", v.sel, v.move)
	}
	y0 := sh.Pos.Y
	want := game.Vec3{X: 0.3, Y: y0, Z: -0.4}
	x, y := at(want)
	// Before the click the preview is the foot it would plant; off on a
	// body, off the area.
	if v.ghostAt(x, y); !v.ghost.on || !nearV(v.ghost.foot, game.Vec3{X: 0.3, Z: -0.4}) || !nearV(v.ghost.pt, want) || v.move.on {
		t.Fatalf("preview: %+v, step %+v", v.ghost, v.move)
	}
	v.hover = shipOf(v, sh)
	if v.ghostAt(x, y); v.ghost.on {
		t.Errorf("preview on a body: %+v", v.ghost)
	}
	v.hover = nil
	if v.ghostAt(float64(sysArea.X1)+20, y); v.ghost.on {
		t.Errorf("preview off the area: %+v", v.ghost)
	}
	// A click outside the area is not one.
	if clickAt(v, e, float64(sysArea.X1)+20, y); v.move.on {
		t.Fatalf("a click outside the area: %+v", v.move)
	}
	if clickAt(v, e, x, y); v.sel != sh || !v.move.on || !v.move.ok || !nearV(v.move.foot, game.Vec3{X: 0.3, Z: -0.4}) || !nearV(v.move.pt, want) {
		t.Fatalf("planted: %+v, want foot (0.3, 0, -0.4)", v.move)
	}
	if v.panelOn() || v.box() != boxMove {
		t.Error("the panel shows in the height step")
	}
	if v.ghostAt(x, y); v.ghost.on {
		t.Error("a preview in the height step")
	}
	// The step reads only the screen y: any x lands on the foot's vertical.
	want = game.Vec3{X: 0.3, Y: 0.7, Z: -0.4}
	_, y = at(want)
	if v.moveTrack(x+40, y); !v.move.ok || !nearV(v.move.pt, want) {
		t.Fatalf("height: %+v, want pt %v", v.move, want)
	}
	before := *sh
	if got := v.moveText(e); !strings.Contains(got, "x 0.30 z -0.40 y 0.70") {
		t.Errorf("move text %q", got)
	} else {
		tmp := *sh
		tmp.Order = game.Order{Kind: game.GoToPoint, Pos: v.move.pt}
		days, _ := tmp.ETA(c, e.Day)
		if w := "\n" + etaText(days); !strings.HasSuffix(got, w) {
			t.Errorf("move text %q lacks %q", got, w)
		}
	}
	if sh.Order != before.Order {
		t.Fatalf("the text set the order: %+v", sh.Order)
	}
	if clickAt(v, e, x+40, y); v.move.on || sh.Order.Kind != game.GoToPoint || !nearV(sh.Order.Pos, want) || v.sel != nil {
		t.Fatalf("second click: mode %+v order %+v sel %v", v.move, sh.Order, v.sel)
	}
	v.sel = sh // picked again
	days, _ := sh.ETA(c, e.Day)
	if got, w := v.describe(e, shipOf(v, sh)), "-> point, "+etaText(days)+"\nx 0.30 z -0.40 y 0.70"; !strings.Contains(got, w) {
		t.Errorf("describe %q lacks %q", got, w)
	}

	// Clamped onto SystemReach: far out in the plane, then high up the
	// vertical; the clamped point is accepted by the rules.
	y0 = sh.Pos.Y                                // the ship hopped on the order above
	x, y = at(game.Vec3{X: 0.9, Y: y0, Z: -1.3}) // inside the area, outside the reach
	clickAt(v, e, x, y)
	if d := math.Hypot(math.Hypot(v.move.foot.X, y0), v.move.foot.Z); !v.move.on || math.Abs(d-game.SystemReach) > 1e-6 {
		t.Fatalf("plane clamp: %+v at %v", v.move, d)
	}
	f := v.move.foot
	_, y = at(game.Vec3{X: f.X, Y: 1, Z: f.Z}) // the foot is on the reach: lim = y0
	v.moveTrack(x, y)
	if clickAt(v, e, x, y); v.move.on || math.Abs(math.Hypot(math.Hypot(sh.Order.Pos.X, sh.Order.Pos.Y), sh.Order.Pos.Z)-game.SystemReach) > 1e-6 {
		t.Fatalf("height clamp: %+v order %+v", v.move, sh.Order)
	}
	v.sel = sh // picked again

	// Esc cancels the step, the selection kept; a second drops the ship,
	// a third has nothing left (the screen's then).
	clickAt(v, e, x, y)
	if !v.move.on || !v.cancel() || v.move.on || v.sel != sh {
		t.Error("cancel does not report the step")
	}
	if !v.cancel() || v.sel != nil || v.cancel() {
		t.Error("the second Esc does not drop the ship")
	}
	v.sel = sh
	// A right click ends the step, the ship kept selected.
	clickAt(v, e, x, y)
	if v.rightClick(); v.move.on || v.sel != sh {
		t.Errorf("right click in the step: sel %v mode %+v", v.sel, v.move)
	}

	// A ship in a lane cannot be selected (no
	// interaction in a lane), and one selected when the day's close took
	// it into a lane is dropped by rebuild.
	ships[1].Orbit, ships[1].Lane, ships[1].Pos = -1, c.Stars[c.Home].Lanes[0], v.sys.Lanes[0].Pos
	v.rebuild()
	v.sel = nil
	if push, err := v.act(e, shipOf(v, ships[1])); push != nil || err != nil || v.sel != nil {
		t.Errorf("lane ship clicked: %v sel %v", err, v.sel)
	}
	v.sel = ships[1]
	v.rebuild()
	if v.sel != nil || v.move.on {
		t.Errorf("lane ship kept selected: %v mode %+v", v.sel, v.move)
	}

	// The step ends when the ship leaves the system.
	v.sel = sh
	clickAt(v, e, x, y)
	sh.Star = c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	v.rebuild()
	if v.moveTrack(x, y); v.move.on || v.sel != nil {
		t.Errorf("ship gone: %+v sel %v", v.move, v.sel)
	}
}

// The first click spends the preview (combat: it does not come back
// after an orbit order or a shot): a click on a body, the fire
// mode's click, none after them until the ship is selected again.
func TestMovePreviewSpent(t *testing.T) {
	e, v, ships := shipScreen(t)
	sh := ships[0]
	v.cam = sysCamera()
	q := v.cam.Project(game.Vec3{X: 0.3, Y: sh.Pos.Y, Z: -0.4})
	x, y := q.X, q.Y
	v.act(e, shipOf(v, sh))
	if v.ghostAt(x, y); !v.ghost.on {
		t.Fatalf("no preview once selected: %+v", v.ghost)
	}
	pl := -1
	for i := range v.bodies {
		if v.bodies[i].kind == planetBody {
			pl = i
		}
	}
	p := v.cam.Project(v.bodies[pl].pos)
	clickAt(v, e, p.X, p.Y)
	if v.ghostAt(x, y); v.ghost.on || v.sel != nil || sh.Order.Kind != game.GoToPlanet {
		t.Errorf("preview after an orbit order: %+v, sel %v, order %+v", v.ghost, v.sel, sh.Order)
	}
	v.act(e, shipOf(v, sh))
	v.aim = true
	v.aimClick(&app{e: e}, nil)
	if v.ghostAt(x, y); v.ghost.on {
		t.Errorf("preview after the fire mode's click: %+v", v.ghost)
	}
	v.act(e, shipOf(v, sh))
	if v.ghostAt(x, y); !v.ghost.on {
		t.Errorf("no preview after a new selection: %+v", v.ghost)
	}
}

// With a ship selected, a hovered planet or lane endpoint is the click's
// target: two rings and a label with the days (moveTarget,
// targetText), the click gives the orbit or the lane order
// through Empire.Order; a refusal keeps the selection and says why.
func TestMoveTarget(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh := ships[0]
	v.cam = sysCamera()
	bodyOf := func(kind bodyKind, n int) *body {
		for i := range v.bodies {
			if v.bodies[i].kind != kind {
				continue
			}
			if n == 0 {
				return &v.bodies[i]
			}
			n--
		}
		t.Fatalf("no body of kind %d", kind)
		return nil
	}
	pl := bodyOf(planetBody, 1)
	v.hover = pl
	if v.moveTarget() != nil {
		t.Fatal("a target with no ship selected")
	}
	v.sel = sh
	if v.moveTarget() != pl {
		t.Fatalf("hover planet: target %v", v.moveTarget())
	}
	tmp := *sh
	tmp.Order = game.Order{Kind: game.GoToPlanet, Planet: pl.idx}
	days, _ := tmp.ETA(c, e.Day)
	if got, want := v.targetText(e, pl), fmt.Sprintf("orbit %s, %s", pl.planet.Name, etaText(days)); got != want {
		t.Errorf("text %q, want %q", got, want)
	}
	if _, err := v.act(e, pl); err != nil || v.move.on || sh.Order != tmp.Order || v.sel != nil {
		t.Fatalf("click planet: %v %+v order %+v sel %v", err, v.move, sh.Order, v.sel)
	}
	v.sel = sh // picked again
	// A list row under the cursor is no target.
	v.hover, v.listHover = pl, true
	if v.moveTarget() != nil {
		t.Fatal("a target on a list row")
	}
	v.listHover = false
	// Off a body, no target; in the height step neither.
	v.hover = nil
	if v.moveTarget() != nil {
		t.Fatal("target kept off a body")
	}
	v.hover, v.move.on = pl, true
	if v.moveTarget() != nil {
		t.Fatal("a target in the height step")
	}
	v.move = moveMode{}
	// A lane endpoint, from the height step too (the step ends).
	ln := bodyOf(laneBody, 0)
	v.move.on = true
	if _, err := v.act(e, ln); err != nil || v.move.on || sh.Order != (game.Order{Kind: game.GoToStar, Star: ln.lane.To}) || v.sel != nil {
		t.Fatalf("click lane: %v %+v order %+v sel %v", err, v.move, sh.Order, v.sel)
	}
	v.sel = sh
	// A refused target (no lane drive) returns the reason, the ship stays
	// selected; the label says it.
	sh.Order = game.Order{}
	sh.Design.Parts = []game.Gizmo{game.ProtonShaver, game.TonklinMotor}
	if _, err := v.act(e, ln); err == nil || v.sel != sh || sh.Order.Kind != game.NoOrder {
		t.Fatalf("lane without a drive: %v sel %v order %+v", err, v.sel, sh.Order)
	}
	if got := v.targetText(e, ln); !strings.HasSuffix(got, ", refused: no star lane drive") {
		t.Errorf("text %q", got)
	}
}

func TestETAText(t *testing.T) {
	if etaText(0) != "today" || etaText(1) != "1 days" || etaText(3) != "3 days" {
		t.Errorf("%q %q %q", etaText(0), etaText(1), etaText(3))
	}
}

// chain runs the screen's anims to their end (on Auto, each end hops the
// next).
func chain(v *systemScreen, e *game.Empire) {
	for v.anim.ship != nil {
		v.tick += hopTicks
		v.advance(e)
	}
}

// An accepted order hops the ship at once and animates it, ONCE: the
// anim's end stops it, the order kept; Repeat Move hops again; EndDay
// finishes on the ETA's day and refills; a day's close drops a stale
// anim; another ship's order replaces the anim.
func TestHopAnim(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh := ships[0]
	d := sh.Design
	near := func(a, b game.Vec3) bool {
		return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.Z-b.Z) < 1e-9
	}
	dist := func(a, b game.Vec3) float64 { return math.Hypot(math.Hypot(a.X-b.X, a.Y-b.Y), a.Z-b.Z) }
	// A point 0.5 from the ship, towards the origin.
	p0 := sh.Pos
	r := dist(p0, game.Vec3{})
	k := (r - 0.5) / r
	target := game.Vec3{X: p0.X * k, Y: p0.Y * k, Z: p0.Z * k}
	if d.PowerPerDay()/d.MoveCost() != 2 || 2*d.Reach() >= 0.5 {
		t.Fatalf("the fixture must need more than a day's two moves: reach %v", d.Reach())
	}
	v.tick = 100
	v.act(e, shipOf(v, sh))
	if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: target}); err != nil {
		t.Fatal(err)
	}
	if got := dist(p0, sh.Pos); math.Abs(got-d.Reach()) > 1e-9 || sh.Power != d.PowerPerDay()-d.MoveCost() {
		t.Fatalf("after the order: moved %v, power %d", got, sh.Power)
	}
	hop1 := sh.Pos
	// From where it was: the planet (a ship in orbit is hidden there).
	ring := p0
	if a := v.anim; a.ship != sh || a.from != ring || a.to != hop1 || a.t0 != 100 || a.day != e.Day || ring != v.sys.Planets[0].Pos {
		t.Fatalf("anim %+v, want from the planet at %v", a, ring)
	}
	// Halfway through, the body is halfway; the model is at the hop's end.
	v.tick = 100 + hopTicks/2
	v.advance(e)
	v.rebuild()
	mid := game.Vec3{X: (ring.X + hop1.X) / 2, Y: (ring.Y + hop1.Y) / 2, Z: (ring.Z + hop1.Z) / 2}
	if b := shipOf(v, sh); b == nil || !near(b.pos, mid) || v.anim.ship != sh {
		t.Fatalf("halfway: body %+v, want %v", b, mid)
	}
	// The end: the ship stops after its one hop, power and order kept.
	v.tick = 100 + hopTicks
	v.advance(e)
	if v.anim.ship != nil || sh.Pos != hop1 || sh.Power != d.PowerPerDay()-d.MoveCost() || sh.Order.Kind != game.GoToPoint {
		t.Fatalf("after one hop's anim: anim %+v pos %v power %d order %+v", v.anim, sh.Pos, sh.Power, sh.Order)
	}
	// Repeat Move: a second hop, animated from the first one's end.
	if err := v.repeat(e); err != nil {
		t.Fatal(err)
	}
	hop2 := sh.Pos
	if a := v.anim; a.ship != sh || a.from != hop1 || a.to != hop2 || a.t0 != v.tick || sh.Power != d.PowerPerDay()-2*d.MoveCost() || math.Abs(dist(hop1, hop2)-d.Reach()) > 1e-9 {
		t.Fatalf("second hop: anim %+v power %d", a, sh.Power)
	}
	if sh.Power >= d.MoveCost() {
		t.Fatalf("power %d left after two moves", sh.Power)
	}
	// The next end: no hop, the order kept; Repeat is refused, no power.
	v.tick += hopTicks
	v.advance(e)
	v.rebuild()
	if err := v.repeat(e); err == nil || err.Error() != "out of power" || v.anim.ship != nil {
		t.Fatalf("repeat out of power: %v anim %+v", err, v.anim)
	}
	if v.anim.ship != nil || sh.Pos != hop2 || sh.Order.Kind != game.GoToPoint || !near(shipOf(v, sh).pos, hop2) {
		t.Fatalf("out of power: anim %+v pos %v order %+v", v.anim, sh.Pos, sh.Order)
	}
	if got := v.describe(e, shipOf(v, sh)); !strings.Contains(got, fmt.Sprintf("power %d/%d", sh.Power, d.PowerPerDay())) {
		t.Errorf("describe %q lacks the power", got)
	}
	// EndDay arrives on the day the ETA says: it refills, then spends the
	// moves left; the rest of the power stays for the screen.
	eta, ok := sh.ETA(c, e.Day)
	if !ok || eta != 1 {
		t.Fatalf("ETA %d %v out of power, want 1", eta, ok)
	}
	left := int(math.Ceil((dist(hop2, target) - 1e-9) / d.Reach()))
	e.EndDay()
	if sh.Pos != target || sh.Order.Kind != game.NoOrder || sh.Power != d.PowerPerDay()-left*d.MoveCost() {
		t.Fatalf("after %d days: pos %v order %+v power %d, want %d", eta, sh.Pos, sh.Order, sh.Power, d.PowerPerDay()-left*d.MoveCost())
	}

	// Another ship's order replaces the anim (the second ship is now the
	// first of the home orbit's ring: it starts from that slot); a ship in
	// space starts from its Pos.
	other := ships[1]
	v.act(e, shipOf(v, other))
	if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: target}); err != nil || v.anim.ship != other || v.anim.from != ring {
		t.Fatalf("second ship: %v anim %+v", err, v.anim)
	}
	v.sel = sh
	if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: p0}); err != nil || v.anim.ship != sh || v.anim.from != target {
		t.Fatalf("back to the first: %v anim %+v", err, v.anim)
	}
	// A day's close drops the anim at once, mid-hop: Step moved the model.
	v.tick++
	e.EndDay()
	at := sh.Pos
	v.advance(e)
	if v.anim.ship != nil || sh.Pos != at {
		t.Errorf("after the day's close: anim %+v pos %v", v.anim, sh.Pos)
	}
}

// The screen hops on Empire.Day: a lane entered on the screen exits
// LaneDays EndDays later, as ETA says. And a ship flying into an orbit is
// off the ring: the next ship of that orbit starts from the first slot.
func TestHopEdges(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	sh, other := ships[0], ships[1]
	lane := c.Stars[c.Home].Lanes[0]
	to := c.Other(lane, c.Home)
	sh.Orbit, sh.Pos = -1, v.sys.Lanes[0].Pos // on the endpoint (Star.Lanes is in endpoint order)
	v.sel = sh
	if err := v.order(e, game.Order{Kind: game.GoToStar, Star: to}); err != nil {
		t.Fatal(err)
	}
	days := game.LaneDays(c, lane, sh.Design.LaneStrength(), sh.Species.Traits())
	if eta, ok := sh.ETA(c, e.Day); sh.Lane != lane || sh.Arrive != e.Day+days || !ok || eta != days {
		t.Fatalf("lane %d arrive %d (day %d), ETA %d %v, want %d", sh.Lane, sh.Arrive, e.Day, eta, ok, days)
	}
	for range days {
		e.EndDay()
	}
	if sh.Star != to || sh.Lane != -1 {
		t.Errorf("after %d days: star %d lane %d", days, sh.Star, sh.Lane)
	}

	// A third ship flies into the home orbit (anim running, Orbit set):
	// drawn flying, not hidden, until its anim ends at the planet.
	third := e.Spawn(fixtureShip, c.Home, 0)
	p := v.sys.Planets[0]
	third.Orbit, third.Pos = -1, game.Vec3{X: p.Pos.X, Y: p.Pos.Y + third.Design.Reach()/2, Z: p.Pos.Z}
	// Ships order: sh (gone), other, third; put third before other.
	e.Ships[1], e.Ships[2] = third, other
	v.sel = third
	if err := v.order(e, game.Order{Kind: game.GoToPlanet, Planet: 0}); err != nil || third.Orbit != 0 || v.anim.ship != third || v.anim.to != p.Pos {
		t.Fatalf("into orbit: %v orbit %d anim %+v", err, third.Orbit, v.anim)
	}
	v.rebuild()
	if b := shipOf(v, third); b == nil || orbitHidden(b) || v.sel != nil {
		t.Errorf("flying into orbit: body %+v, sel %v: drawn until the anim ends, dropped", b, v.sel)
	}
	v.tick = v.anim.t0 + hopTicks
	v.advance(e)
	v.rebuild()
	if b := shipOf(v, third); b == nil || !orbitHidden(b) || v.sel != nil {
		t.Errorf("in orbit: body %+v, sel %v: hidden, deselected", b, v.sel)
	}
	v.sel = other
	if err := v.order(e, game.Order{Kind: game.GoToPoint}); err != nil || v.anim.ship != other || v.anim.from != p.Pos {
		t.Errorf("out of the orbit: %v anim %+v, want from the planet %v", err, v.anim, p.Pos)
	}
}

// Two empires' ships at one star: the screen lists both, each with its
// owner and species, the rings counting every empire's ships; the
// foreign ship is described but never selected nor ordered.
func TestForeignShips(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e, o := g.Empires[0], g.Empires[1]
	home := e.Colonies[0].Star
	mine := e.Spawn(fixtureShip, home, 0)
	theirs := o.Spawn(fixtureShip, home, 1)
	ringed := o.Spawn(fixtureShip, home, 0) // second on planet 0's ring
	a := testApp(g)
	v := newSystemScreen(home)
	v.Enter(a)
	for _, c := range []struct {
		sh    *game.Starship
		owner int
		sp    game.Species
	}{{mine, 0, e.Species}, {theirs, 1, game.Orfa}, {ringed, 1, game.Orfa}} {
		b := shipOf(v, c.sh)
		if b == nil {
			t.Fatalf("%s of empire %d not listed", c.sh.Name, c.owner)
		}
		if bank, _ := b.sprite(); b.owner != c.owner || b.species != c.sp || bank != shipBank(c.sp) {
			t.Errorf("%s: owner %d, species %s, bank %s", c.sh.Name, b.owner, b.species, bank)
		}
	}
	p0 := v.sys.Planets[0]
	if b := shipOf(v, ringed); b.pos != p0.Pos || b.pos != v.drawnAt(ringed) || !orbitHidden(b) {
		t.Errorf("the Orfa ship on planet 0: at %v (drawnAt %v), hidden %v, want hidden at the planet %v", b.pos, v.drawnAt(ringed), orbitHidden(b), p0.Pos)
	}

	// A click on the foreign ship inspects it (fsel), our selection kept
	// as back, nothing ordered.
	foreign := shipOf(v, theirs)
	if push, err := v.act(e, foreign); push != nil || err != nil || v.sel != nil || v.fsel != theirs || v.back != nil {
		t.Errorf("act on the foreign ship: push %#v, err %v, sel %v, fsel %v", push, err, v.sel, v.fsel)
	}
	v.act(e, shipOf(v, mine))
	if v.sel != mine || v.fsel != nil {
		t.Errorf("act on ours after: sel %v, fsel %v", v.sel, v.fsel)
	}
	if push, err := v.act(e, shipOf(v, theirs)); push != nil || err != nil || v.sel != nil || v.fsel != theirs || v.back != mine || mine.Order.Kind != game.NoOrder {
		t.Errorf("act on the foreign ship with a selection: push %#v, err %v, sel %v, back %v, order %+v", push, err, v.sel, v.back, mine.Order)
	}
	got := foreignOf(t, v, theirs)
	if want := game.Orfa.String() + " " + strings.ToLower(fixtureShip.Hull.String()) + " ship"; got.title != want ||
		!slices.Equal(got.where, []string{"orbit of " + v.sys.Planets[1].Name}) || got.note != "not scanned" {
		t.Errorf("foreign ship's panel %+v, want %q", got, want)
	}
	if got := v.describe(e, shipOf(v, mine)); !strings.HasPrefix(got, mine.Name+"\n") {
		t.Errorf("own ship described %q", got)
	}
}

// The -warships/-foes fixtures, F and S: F fires the selected ship's
// first able weapon at the hovered ship, the damage the table's less the
// target's shields; a destruction leaves the bodies, the empire and the
// hover, and logs the player's report; D flips the shields.
func TestCombatKeys(t *testing.T) {
	if err := spawnFixtures(testApp(game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)), 0, 0, 1); err == nil {
		t.Error("-foes with one empire accepted")
	}
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	e, o := g.Empires[0], g.Empires[1]
	a := testApp(g)
	if err := spawnFixtures(a, 1, 1, 2); err != nil {
		t.Fatal(err)
	}
	if len(e.Ships) != 2 || len(o.Ships) != 2 {
		t.Fatalf("%d ships, %d foes", len(e.Ships), len(o.Ships))
	}
	colony, gun, foe, foe2 := e.Ships[0], e.Ships[1], o.Ships[0], o.Ships[1]
	if !strings.HasPrefix(gun.Name, "Gunboat") || !strings.HasPrefix(foe.Name, "Raider") || foe.Star != colony.Star || foe.Orbit != -1 || gun.Orbit != -1 {
		t.Errorf("fixtures %s %s at %d/%d", gun.Name, foe.Name, foe.Star, foe.Orbit)
	}
	if err := foeShip.Valid(allTechs()); err != nil {
		t.Error(err)
	}
	v := newSystemScreen(colony.Star)
	v.Enter(a)
	w := game.Gizmos[game.MassBarrageGun]

	v.hover = shipOf(v, foe)
	if err := v.fire(a); err == nil {
		t.Error("F with no selection")
	}
	v.sel = gun
	hull := foe.Hull
	if err := v.fire(a); err != nil || len(gun.Armed) != 1 || foe.Hull != hull {
		t.Errorf("F on a shielded Raider: %v, armed %v, hull %d of %d (nothing before the close)", err, gun.Armed, foe.Hull, hull)
	}
	if err := v.fire(a); err == nil || !strings.Contains(err.Error(), "already armed this round") {
		t.Errorf("F twice in a round: %v", err)
	}
	twin := gun.Design // a second weapon: the next F arms it
	twin.Parts = append(slices.Clone(twin.Parts), game.MassBarrageGun)
	gun2 := e.Spawn(twin, gun.Star, 0)
	gun2.Orbit, gun2.Pos = -1, gun.Pos
	v.sel = gun2
	if err := v.fire(a); err != nil || len(gun2.Armed) != 1 {
		t.Errorf("F on a twin: %v, armed %v", err, gun2.Armed)
	}
	if err := v.fire(a); err != nil || len(gun2.Armed) != 2 || gun2.Armed[0].Part == gun2.Armed[1].Part {
		t.Errorf("F again on a twin: %v, armed %v (the second weapon)", err, gun2.Armed)
	}
	e.Ships = slices.DeleteFunc(e.Ships, func(x *game.Starship) bool { return x == gun2 })
	v.sel = gun
	a.endDay() // the first round fires the armed shot
	if foe.Hull != hull-max(0, w.Level-foe.Absorb()) || len(gun.Armed) != 0 {
		t.Errorf("the close on a shielded Raider: hull %d of %d, armed %v", foe.Hull, hull, gun.Armed)
	}
	mbg := slices.Index(gun.Design.Parts, game.MassBarrageGun)
	gun.Shots[mbg] = w.Uses
	if err := v.fire(a); err == nil || err.Error() != gun.CanFire(g.Cluster, mbg, foe).Error() || !strings.Contains(err.Error(), "no shot left today") {
		t.Errorf("F with the shot spent: %v", err)
	}
	gun.Refill()
	foe.Shields = false
	if err := v.fire(a); err != nil {
		t.Errorf("F on an unshielded Raider: %v", err)
	}
	a.endDay()
	if foe.Hull != hull-w.Level {
		t.Errorf("the close on an unshielded Raider: hull %d of %d", foe.Hull, hull)
	}
	if fv := foreignOf(t, v, foe); fv.note != "not scanned" || fv.bars != nil {
		t.Errorf("foreign ship's panel %+v: no scanner aboard the Gunboat, nothing read", fv)
	}

	gun.Refill()
	foe.Hull = w.Level
	if err := v.fire(a); err != nil {
		t.Fatal(err)
	}
	a.endDay()
	v.rebuild() // Update's, every frame
	if shipOf(v, foe) != nil || g.OwnerOf(foe) != nil || len(o.Ships) != 1 {
		t.Errorf("destroyed: still a body %v, owner %v", shipOf(v, foe) != nil, g.OwnerOf(foe))
	}
	logged := false
	for _, x := range a.log {
		logged = logged || x.Ev.Text == gun.Name+" destroys an "+game.Orfa.String()+" ship at "+e.Cluster.Stars[gun.Star].Name && x.Day == e.Day // never the foe's name
	}
	if !logged {
		t.Errorf("log %+v", a.log)
	}
	v.hover = &body{kind: shipBody, ship: foe, owner: 1}
	if err := v.fire(a); err == nil || err.Error() != "no such target" {
		t.Errorf("F on a destroyed ship: %v", err)
	}

	gun.Refill()
	gun.Pos = colony.Pos // the planet turned three days, pushing it off: back in range, the stance refuses
	v.hover = shipOf(v, colony)
	if err := v.fire(a); err == nil || !strings.Contains(err.Error(), "not hostile") {
		t.Errorf("F on our own ship: %v", err)
	}
	v.sel, v.hover = colony, shipOf(v, foe2)
	if err := v.fire(a); err == nil || err.Error() != "no weapon" {
		t.Errorf("F with no weapon: %v", err)
	}

	v.sel = gun
	var buf bytes.Buffer
	g.SetJournal(journal.New(&buf)) // D goes through Empire.SetShields: journalled
	for _, want := range []bool{false, true} {
		if err := v.toggleShields(); err != nil || gun.Shields != want {
			t.Errorf("D: %v, shields %v", err, gun.Shields)
		}
	}
	if n := strings.Count(buf.String(), `"act":"shields"`); n != 2 {
		t.Errorf("D journalled %d times: %q", n, buf.String())
	}
	g.SetJournal(nil)
	if desc := v.describe(e, shipOf(v, gun)); !strings.Contains(desc, fmt.Sprintf("hull %d/%d\nshields on (score %d)", gun.Hull, gun.MaxHull(), gun.Design.ShieldScore())) {
		t.Errorf("own ship described %q", desc)
	}
	// D off refunds the draw; on short of it is refused, the switch kept.
	draw, power := gun.Design.ShieldDraw(), gun.Power
	if err := v.toggleShields(); err != nil || gun.Shields || gun.Power != power+draw || draw < 1 {
		t.Fatalf("D off: %v, shields %v, power %d, want %d", err, gun.Shields, gun.Power, power+draw)
	}
	gun.Power = draw - 1
	if err := v.toggleShields(); err == nil || !strings.Contains(err.Error(), "not enough energy") || gun.Shields || gun.Power != draw-1 {
		t.Errorf("D on short of the draw: %v, shields %v, power %d", err, gun.Shields, gun.Power)
	}
	gun.Power = power + draw
	if err := v.toggleShields(); err != nil || !gun.Shields || gun.Power != power {
		t.Errorf("D on: %v, shields %v, power %d, want %d", err, gun.Shields, gun.Power, power)
	}
	v.sel = nil
	if v.toggleShields() == nil {
		t.Error("S with no selection")
	}
}

// F fires the weapon ReadyWeapon gives: the one picked on the panel
// (a click on its tile) while it can, the first that can once it is
// spent.
func TestFireReadyWeapon(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	if err := spawnFixtures(a, 0, 0, 1); err != nil {
		t.Fatal(err)
	}
	foe := g.Empires[1].Ships[0]
	foe.Hull = 1000 // survives the volley
	d := game.Design{Name: "Twin", Hull: game.Small, Parts: []game.Gizmo{game.QuarkExpress, game.MassBarrageGun, game.Ueberlaser}}
	sh := a.e.Spawn(d, foe.Star, a.e.Colonies[0].PlanetIdx)
	sh.Orbit = -1 // in space, on the planet: the foe is in range
	sh.Power = 100
	gun, laser := 1, 2
	v := newSystemScreen(sh.Star)
	v.Enter(a)
	v.sel, v.hover = sh, shipOf(v, foe)
	if err := pressTile(t, v, a, sh, game.Ueberlaser); err != nil || sh.Weapon != laser {
		t.Fatalf("laser tile: %v, Weapon %d", err, sh.Weapon)
	}
	v.aim = false
	for range game.Gizmos[game.Ueberlaser].Uses {
		if err := v.fire(a); err != nil {
			t.Fatal(err)
		}
		a.g.Round(sh.Star, nil)
	}
	if sh.Shots[laser] != game.Gizmos[game.Ueberlaser].Uses || sh.Shots[gun] != 0 {
		t.Errorf("the laser picked: shots %v", sh.Shots)
	}
	if sh.CanFire(a.cl, laser, foe) == nil {
		t.Fatal("the laser still fires")
	}
	if err := v.fire(a); err != nil || len(sh.Armed) != 1 || sh.Armed[0].Part != gun {
		t.Errorf("the laser spent: %v, armed %+v", err, sh.Armed)
	}
	if a.g.Round(sh.Star, nil); sh.Shots[gun] != 1 {
		t.Errorf("the gun's round: shots %v", sh.Shots)
	}
}

// allTechs knows the whole tree.
func allTechs() *game.Research {
	r := game.NewResearch()
	for t := range game.NumTechs {
		r.Grant(t)
	}
	return r
}

// endDay logs the bots' combat reports that are the player's: a Gunboat
// lost to empire 1's bot, its Raider's report left out.
func TestEndDayLogsCombat(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a := testApp(g)
	if err := spawnFixtures(a, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	cfg := bot.Default()
	cfg.Genome = bot.Neutral()
	cfg.Genome.FleeRatio = 1e9 // the Raider overrates the unscanned Gunboat: no flight
	cfg.Genome.Temper = 300    // the Orfa's Start sits over its ProposePeace: no peace offer ahead of the combat
	a.bots = bot.NewAll(g, cfg)
	a.bots[0] = nil
	gun := a.e.Ships[0]
	gun.Shields, gun.Hull = false, game.Gizmos[game.MassBarrageGun].Level
	evs := a.endDay()
	want := gun.Name + " lost to an " + game.Orfa.String() + " ship at " + g.Cluster.Stars[gun.Star].Name // never the foe's name
	if len(evs) == 0 || evs[0].Text != want || evs[0].Kind != game.Destroyed || len(a.e.Ships) != 0 {
		t.Fatalf("events %+v, %d ships left", evs, len(a.e.Ships))
	}
	for _, l := range a.log {
		if strings.Contains(l.Ev.Text, " destroys ") {
			t.Errorf("the Raider's report logged: %q", l.Ev.Text)
		}
	}
	if a.log[0].Ev.Text != want || a.log[0].Day != a.e.Day {
		t.Errorf("log %+v", a.log[0])
	}
}

// The fog on the system screen (game/fog.go): an unknown star shows the
// star and its known lane endpoints, no planet, its type in the title
// and on the lane labels; a known one all its planets and lanes. Another
// empire's ship shows only while the player is present at its star.
func TestSystemFog(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	e, o, cl := g.Empires[0], g.Empires[1], g.Cluster
	home := e.Colonies[0].Star
	a := testApp(g)
	count := func(v *systemScreen, k bodyKind) int {
		n := 0
		for _, b := range v.bodies {
			if b.kind == k {
				n++
			}
		}
		return n
	}

	v := newSystemScreen(home)
	v.Enter(a)
	if np, nl := count(v, planetBody), count(v, laneBody); np != len(v.sys.Planets) || nl != len(v.sys.Lanes) {
		t.Errorf("home: %d planets %d lanes, want %d %d", np, nl, len(v.sys.Planets), len(v.sys.Lanes))
	}
	if got := v.title(e); !strings.HasPrefix(got, v.sys.Name+"\n") {
		t.Errorf("home title %q", got)
	}

	far := cl.Other(cl.Stars[home].Lanes[0], home)
	if e.Knows(far) {
		t.Fatalf("star %d known on day 0", far)
	}
	u := newSystemScreen(far)
	u.Enter(a)
	if np, nl, ns := count(u, planetBody), count(u, laneBody), count(u, starBody); np != 0 || nl != 1 || ns != 1 {
		t.Errorf("unknown star %d (%d lanes): %d planets %d lanes %d stars, want 0 1 1", far, len(u.sys.Lanes), np, nl, ns)
	}
	for _, b := range u.bodies {
		if b.kind == laneBody && b.lane.To != home {
			t.Errorf("unknown star shows the lane to %d", b.lane.To)
		}
	}
	if got, w := u.title(e), fmt.Sprintf("%s\nunexplored\nDay %d  1 lanes", u.sys.Star, e.Day); got != w {
		t.Errorf("unknown title %q, want %q", got, w)
	}
	if got := u.describe(e, &u.bodies[0]); got != u.sys.Star.String() {
		t.Errorf("unknown star described %q", got)
	}
	// From the home, the lane to it says its type, not its name.
	for _, b := range v.bodies {
		if b.kind == laneBody && b.lane.To == far {
			if got := v.describe(e, &b); !strings.HasSuffix(got, " to "+cl.Stars[far].Type.String()) || strings.Contains(got, cl.Stars[far].Name) {
				t.Errorf("lane to the unknown star: %q", got)
			}
		}
	}

	// Empire 1's ship at its own home: not seen until a ship of ours is
	// there (Spawn visits: the star becomes known too).
	oh := o.Colonies[0].Star
	theirs := o.Spawn(fixtureShip, oh, 0)
	w := newSystemScreen(oh)
	w.Enter(a)
	if e.Present(oh) || shipOf(w, theirs) != nil || count(w, planetBody) != 0 {
		t.Errorf("foreign home %d: present %v, their ship %v, %d planets", oh, e.Present(oh), shipOf(w, theirs), count(w, planetBody))
	}
	mine := e.Spawn(fixtureShip, oh, 0)
	w.rebuild()
	if shipOf(w, theirs) == nil || shipOf(w, mine) == nil || count(w, planetBody) != len(w.sys.Planets) || count(w, laneBody) != len(w.sys.Lanes) {
		t.Errorf("foreign home with our ship: theirs %v mine %v, %d planets %d lanes", shipOf(w, theirs), shipOf(w, mine), count(w, planetBody), count(w, laneBody))
	}
	// Our ship leaves into a lane (Star kept, out of Present): the known
	// star keeps theirs in sight (the partial fog); ours is gone.
	mine.Lane = cl.Stars[oh].Lanes[0]
	w.rebuild()
	if e.Present(oh) || shipOf(w, theirs) == nil || shipOf(w, mine) != nil {
		t.Errorf("our ship in a lane: theirs %v mine %v", shipOf(w, theirs), shipOf(w, mine))
	}
	x := cl.Other(mine.Lane, oh)
	wantTo := cl.Stars[x].Type.String()
	if e.Knows(x) {
		wantTo = cl.Stars[x].Name
	}
	if got := shipWhere(e, mine); got != "lane to "+wantTo {
		t.Errorf("shipWhere %q, want lane to %s", got, wantTo)
	}
}

// toward is the point dist from p towards the origin.
func toward(p game.Vec3, dist float64) game.Vec3 {
	r := math.Hypot(math.Hypot(p.X, p.Y), p.Z)
	k := (r - dist) / r
	return game.Vec3{X: p.X * k, Y: p.Y * k, Z: p.Z * k}
}

// sideways is the point dist from p along the horizontal perpendicular
// to p, sign ±1: with toward, three rays from one orbit at right angles,
// ships flying them staying far beyond game.ShipGap of each other.
func sideways(p game.Vec3, dist, sign float64) game.Vec3 {
	u := game.Vec3{X: -p.Z, Z: p.X}
	k := sign * dist / math.Hypot(u.X, u.Z)
	return game.Vec3{X: p.X + u.X*k, Y: p.Y, Z: p.Z + u.Z*k}
}

// Repeat Move's refusals: no selection, no order, a ship in a lane, no
// power left; none of them hops.
func TestRepeatRefusals(t *testing.T) {
	e, v, ships := shipScreen(t)
	sh := ships[0]
	want := func(what string) {
		t.Helper()
		pos := sh.Pos
		if err := v.repeat(e); err == nil || err.Error() != what || v.anim.ship != nil || sh.Pos != pos {
			t.Errorf("%s: got %v anim %+v", what, err, v.anim)
		}
	}
	want("no ship selected")
	v.sel = sh
	want("no order to repeat")
	sh.Order = game.Order{Kind: game.GoToPoint, Pos: toward(sh.Pos, 0.5)}
	sh.Lane = 0
	want("in a lane")
	sh.Lane = -1
	sh.Power = 0
	want("out of power")
	sh.Power = sh.Design.MoveCost()
	if err := v.repeat(e); err != nil || v.anim.ship != sh || sh.Power != 0 {
		t.Errorf("with one move's power: %v anim %+v power %d", err, v.anim, sh.Power)
	}
}

// Auto: the selected ship first, then each anim's end hops the same ship
// while it moves, else the next ordered one in Ships order, until none
// moves (out of power, or arrived: order done); then Auto is off. A new
// order, or a day's close, ends it; with nothing to move it is refused.
func TestAuto(t *testing.T) {
	e, v, ships := shipScreen(t)
	c := e.Cluster
	a, b := ships[0], ships[1]
	cc := e.Spawn(fixtureShip, c.Home, 0)
	v.rebuild()
	d := fixtureShip
	if d.PowerPerDay()/d.MoveCost() != 2 || 2*d.Reach() >= 0.45 {
		t.Fatalf("the fixture must need more than a day's two moves: reach %v", d.Reach())
	}
	if err := v.startAuto(e); err == nil || err.Error() != "nothing to move" || v.auto {
		t.Fatalf("no order: %v auto %v", err, v.auto)
	}
	// Three rays at right angles from the one orbit: once out of it, two
	// ships are at least one Reach apart, never within game.ShipGap.
	if d.Reach() < game.ShipGap {
		t.Fatalf("reach %v: the rays' ships would crowd", d.Reach())
	}
	ta, tb, tc := toward(a.Pos, 0.5), sideways(b.Pos, 0.45, 1), sideways(cc.Pos, 1.5*d.Reach(), -1)
	for _, o := range []struct {
		sh *game.Starship
		to game.Vec3
	}{{a, ta}, {cc, tc}, {b, tb}} {
		v.sel = o.sh
		if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: o.to}); err != nil {
			t.Fatal(err)
		}
		chain(v, e) // one hop each: Auto is off
	}
	if a.Power != d.MoveCost() || b.Power != d.MoveCost() || cc.Power != d.MoveCost() {
		t.Fatalf("after the orders: power %d %d %d", a.Power, b.Power, cc.Power)
	}
	v.sel = b
	if err := v.startAuto(e); err != nil || !v.auto || v.anim.ship != b {
		t.Fatalf("start: %v auto %v anim %v, want the selected ship", err, v.auto, v.anim.ship)
	}
	var seq []*game.Starship
	for v.anim.ship != nil {
		seq = append(seq, v.anim.ship)
		v.tick += hopTicks
		v.advance(e)
	}
	if want := []*game.Starship{b, a, cc}; !slices.Equal(seq, want) {
		t.Errorf("hops %v, want b, a, c", seq)
	}
	if v.auto {
		t.Error("Auto still on with nothing to move")
	}
	for _, sh := range []*game.Starship{a, b} {
		if sh.Power >= d.MoveCost() || sh.Order.Kind != game.GoToPoint {
			t.Errorf("%s: power %d order %+v, want out of power, order kept", sh.Name, sh.Power, sh.Order)
		}
	}
	if cc.Pos != tc || cc.Order.Kind != game.NoOrder {
		t.Errorf("c: pos %v order %+v, want arrived at %v", cc.Pos, cc.Order, tc)
	}

	// A day's close ends Auto; so does a new order (X's cancel).
	restart := func() {
		t.Helper()
		e.EndDay()
		a.Power = d.PowerPerDay() // whatever Step spent on the old orders
		v.sel = a
		if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: toward(a.Pos, 0.5)}); err != nil {
			t.Fatal(err)
		}
		if err := v.startAuto(e); err != nil || !v.auto {
			t.Fatalf("restart: %v", err)
		}
	}
	restart()
	e.EndDay()
	v.advance(e)
	if v.auto || v.anim.ship != nil {
		t.Errorf("after the day's close: auto %v anim %+v", v.auto, v.anim)
	}
	restart()
	v.sel = a
	if err := v.order(e, game.Order{}); err != nil || v.auto {
		t.Errorf("X: %v auto %v, want off", err, v.auto)
	}
}

// Each hole's top-left corner is its button; the area's is none; the
// panel's strip holds the title's three bigFace lines.
func TestSysButton(t *testing.T) {
	for _, c := range []struct {
		r rect
		b int
	}{{sysEndTurn, endTurnButton}, {sysRepeat, repeatButton}, {sysAuto, autoButton}, {sysArea, noButton}, {sysFilters[0], planetsFilter}, {sysFilters[1], shipsFilter}} {
		if got := sysButton(float64(c.r.X0), float64(c.r.Y0)); got != c.b {
			t.Errorf("corner of %v: button %d, want %d", c.r, got, c.b)
		}
	}
	if sysButton(float64(sysAuto.X1), float64(sysAuto.Y0)) != noButton {
		t.Error("the Auto hole's right edge is a button")
	}
	if h := 6*zoom + 3*bigFace.Size*1.25; h > float64(sysStrip.Y1-sysStrip.Y0) {
		t.Errorf("the title's %v native px overflow the strip", h)
	}
}

// End Turn never closes the day: with no ordered ship here it is
// refused, toasted.
func TestEndTurnButton(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas}, game.Average)
	a := testApp(g)
	v := newSystemScreen(a.e.Cluster.Home)
	a.push(v)
	day := a.e.Day
	v.press(a, sysButton(float64(sysEndTurn.X0+1), float64(sysEndTurn.Y0+1)))
	if a.e.Day != day || a.toast != "nothing to move" {
		t.Errorf("day %d of %d, toast %q", a.e.Day, day, a.toast)
	}
}

// tileCentre is a panel tile's centre, native px.
func tileCentre(p panelTile) (float64, float64) {
	return float64(p.r.X0+p.r.X1) / 2, float64(p.r.Y0+p.r.Y1) / 2
}

// pressTile presses the centre of sh's tile of gizmo g on v's panel.
func pressTile(t *testing.T, v *systemScreen, a *app, sh *game.Starship, g game.Gizmo) error {
	t.Helper()
	x, y := tileCentre(panelTile0(t, sh, g))
	return v.panelClick(a, x, y)
}

// panelTile0 is sh's panel tile of gizmo g.
func panelTile0(t *testing.T, sh *game.Starship, g game.Gizmo) panelTile {
	t.Helper()
	for _, p := range panelTiles(sh) {
		if p.g == g {
			return p
		}
	}
	t.Fatalf("%s: no %s tile", sh.Name, g)
	return panelTile{}
}

// panelFixture is seed 1 with two empires and the -ships, -warships and
// -foes fixtures at the player's home planet (a colony ship, a Gunboat,
// empire 1's Raider), its system entered.
func panelFixture(t *testing.T) (a *app, v *systemScreen, colony, gun, raider *game.Starship) {
	t.Helper()
	g := game.NewGame(2, []game.Species{game.Snovemdomas, game.Orfa}, game.Average)
	a = testApp(g)
	if err := spawnFixtures(a, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	colony, gun, raider = a.e.Ships[0], a.e.Ships[1], g.Empires[1].Ships[0]
	v = newSystemScreen(gun.Star)
	v.Enter(a)
	return a, v, colony, gun, raider
}

// The panel lists Design.Counts' groups band by band (weapons, shields,
// specials, passive), the clickable ones first within a band, else
// Counts' order (the Colonizers lead the passive specials), a band with
// no part skipped; rows of panelCols at
// tilePitch, each band panelLabelH under the last; every tile inside the
// info box above its foot, none overlapping.
func TestPanelTiles(t *testing.T) {
	_, _, colony, gun, _ := panelFixture(t)
	many := &game.Starship{Name: "Scanners", Design: game.Design{Hull: game.Enormous, Parts: []game.Gizmo{
		game.MassBarrageGun, game.TonklinFrequencyAnalyzer, game.SubspacePhaseArray, game.AuralCloudConstructor,
		game.HyperwaveTympanum, game.MurgatroydsKnower, game.ProtonShaver}}}
	for _, sh := range []*game.Starship{colony, gun, many} {
		var want []game.PartCount
		for band := range numBands {
			for _, click := range []bool{true, false} {
				for _, c := range sh.Design.Counts() {
					if gizmoBand(c.Gizmo) == band && tileClickable(c.Gizmo) == click {
						want = append(want, c)
					}
				}
			}
		}
		ts := panelTiles(sh)
		if len(ts) != len(want) {
			t.Fatalf("%s: %d tiles for %d groups", sh.Name, len(ts), len(want))
		}
		y, inBand, band := panelBandsTop, 0, -1
		for i, p := range ts {
			if p.g != want[i].Gizmo || p.n != want[i].N || p.band != gizmoBand(p.g) || sh.Design.Parts[p.part] != p.g || slices.Index(sh.Design.Parts, p.g) != p.part {
				t.Errorf("%s tile %d: %+v, want %+v", sh.Name, i, p, want[i])
			}
			if p.band != band {
				if band >= 0 {
					y += panelLabelH + (inBand+panelCols-1)/panelCols*bandRowH(band)
				}
				band, inBand = p.band, 0
			}
			x0, y0 := panelX0+inBand%panelCols*tilePitch, y+panelLabelH+inBand/panelCols*bandRowH(band)
			if p.r != (rect{x0, y0, x0 + tileW, y0 + tileH}) {
				t.Errorf("%s tile %d (%s) at %v, want (%d,%d)", sh.Name, i, p.g, p.r, x0, y0)
			}
			inBand++
			if p.r.X0 < sysInfo.X0 || p.r.X1 > sysInfo.X1 || p.r.Y0 < sysInfo.Y0 || p.r.Y1 > panelBottom || panelBottom > sysInfo.Y1 {
				t.Errorf("%s tile %d at %v: out of the box", sh.Name, i, p.r)
			}
			for _, q := range ts[:i] {
				if p.r.X0 < q.r.X1 && q.r.X0 < p.r.X1 && p.r.Y0 < q.r.Y1 && q.r.Y0 < p.r.Y1 {
					t.Errorf("%s: tiles %v and %v overlap", sh.Name, p.r, q.r)
				}
			}
			if cx, cy := tileCentre(p); tileAt(ts, cx, cy) != i {
				t.Errorf("%s tile %d's centre misses it", sh.Name, i)
			}
		}
	}
	if ts := panelTiles(colony); ts[0].band != specialBand || ts[0].g != game.Colonizer || ts[0].n != 2 {
		t.Errorf("the colony ship's first tile %+v, want the Colonizers x2", ts[0])
	}
	if ts := panelTiles(many); ts[5].r.Y0 != ts[1].r.Y0+tileRowH || ts[5].r.X0 != ts[1].r.X0 {
		t.Errorf("the fifth passive tile at %v, not under the first %v", ts[5].r, ts[1].r)
	}
	if panelBottom != sysInfo.Y1-6*zoom-int(3*smallH) {
		t.Errorf("panelBottom %d, not above the foot's three lines", panelBottom)
	}
	if tileAt(panelTiles(gun), float64(panelX0+tileW+1), float64(panelTiles(gun)[0].r.Y0+1)) != -1 {
		t.Error("the gap between two tiles hits one")
	}
}

// A warship with all four bands shows
// every group, the Replenisher and Hyperfuel ahead of the passive
// specials; a ship past the box drops its last groups, panelHidden
// counting them.
func TestPanelTilesFull(t *testing.T) {
	g := game.Gizmo.String
	killer := &game.Starship{Design: game.Design{Hull: game.Enormous, Parts: []game.Gizmo{
		game.InertiaNegator, game.VanCreegHypersplicer, game.VanCreegHypersplicer, game.Nanoshell,
		game.InvasionModule, game.StarLaneHyperdrive, game.Plasmatron, game.MurgatroydsKnower,
		game.Hyperfuel, game.Replenisher}}}
	ts := panelTiles(killer)
	if n := panelHidden(killer); n != 0 || len(ts) != 9 {
		t.Fatalf("KILLER: %d tiles, %d hidden", len(ts), n)
	}
	var specials []string
	for _, p := range ts {
		if p.band == specialBand {
			specials = append(specials, g(p.g))
		}
	}
	if got, want := strings.Join(specials, ","), "Replenisher,Hyperfuel,Invasion Module,Star Lane Hyperdrive"; got != want {
		t.Errorf("specials %s, want %s", got, want)
	}
	var parts []game.Gizmo
	for i := range game.Gizmo(len(game.Gizmos)) {
		if gizmoBand(i) != weaponBand && gizmoBand(i) != shieldBand {
			parts = append(parts, i)
		}
	}
	all := &game.Starship{Design: game.Design{Hull: game.Enormous, Parts: append([]game.Gizmo{game.Plasmatron, game.Nanoshell}, parts...)}}
	ts = panelTiles(all)
	if n := panelHidden(all); n == 0 || n+len(ts) != len(all.Design.Counts()) || ts[len(ts)-1].r.Y1 > panelBottom {
		t.Errorf("an overfull ship: %d tiles, %d hidden, the last at %v", len(ts), n, ts[len(ts)-1].r)
	}
}

// The tiles' counts and the hover line: a weapon's parts, its shots used
// on the line under it, the Colonizers' cargo, the
// parts aboard for the rest; the
// shields' switch as the only state; the part's facts, or a special's
// Usable verdict, in the hover line.
func TestPanelStates(t *testing.T) {
	a, _, colony, gun, raider := panelFixture(t)
	w := slices.Index(gun.Design.Parts, game.MassBarrageGun)
	uses := game.Gizmos[game.MassBarrageGun].Uses
	if uses == 0 {
		t.Fatal("the fixture gun has no shot limit")
	}
	gt := panelTile0(t, gun, game.MassBarrageGun)
	if got, u := tileCount(gun, gt), usedText(gun, gt); got != gt.n || u != fmt.Sprintf("used 0/%d", gt.n*uses) || tileState(gun, gt) != "" {
		t.Errorf("fresh gun x%d %q %q, want x%d", got, u, tileState(gun, gt), gt.n)
	}
	sg, _ := a.g.SightingOf(a.e, raider)
	if err := a.e.Arm(gun, w, sg); err != nil {
		t.Fatal(err)
	}
	a.g.Round(gun.Star, nil)
	if got, u := tileCount(gun, gt), usedText(gun, gt); got != gt.n || u != fmt.Sprintf("used 1/%d", gt.n*uses) {
		t.Errorf("after a shot x%d %q, want x%d, used 1", got, u, gt.n)
	}
	if got := tileText(gun, gt); got != fmt.Sprintf("Mass Barrage Gun x%d: %s", gt.n, partFacts(game.MassBarrageGun)[0]) {
		t.Errorf("gun hover %q", got)
	}
	if got := tileState(gun, panelTile0(t, gun, game.IonWrap)); got != onOff(gun.Shields) {
		t.Errorf("shield %q", got)
	}
	// On but dark (a Shield Blaster): "down", whatever the energy (the
	// draw is reserved, never short); off stays "off" whatever the dark
	// flag.
	st := panelTile0(t, gun, game.IonWrap)
	shields, down, power := gun.Shields, gun.ShieldsDown, gun.Power
	gun.Shields, gun.ShieldsDown = true, true
	for _, p := range []int{0, gun.Design.ShieldDraw()} {
		if gun.Power = p; tileState(gun, st) != "down" {
			t.Errorf("dark at energy %d: %q", p, tileState(gun, st))
		}
	}
	gun.Shields = false
	if got := tileState(gun, st); got != onOff(false) {
		t.Errorf("off %q", got)
	}
	gun.Shields, gun.ShieldsDown, gun.Power = shields, down, power
	// A weapon with a shot left but short of its Power: "no power"; out
	// of shots, the count says it alone.
	shots := slices.Clone(gun.Shots)
	clear(gun.Shots)
	gun.Power = game.Gizmos[game.MassBarrageGun].Power - 1
	if got := tileState(gun, gt); got != noPower {
		t.Errorf("gun short of its power %q", got)
	}
	for k := range gun.Shots {
		gun.Shots[k] = uses
	}
	if got := tileState(gun, gt); got != "" {
		t.Errorf("gun out of shots %q", got)
	}
	copy(gun.Shots, shots)
	gun.Power = power
	ps := panelTile0(t, gun, game.ProtonShaver)
	if tileState(gun, ps) != "" || tileCount(gun, ps) != 2 || tileText(gun, ps) != "Proton Shaver x2: "+partFacts(game.ProtonShaver)[0] {
		t.Errorf("generators: %q / %q", tileState(gun, ps), tileText(gun, ps))
	}
	ct := panelTile0(t, colony, game.Colonizer)
	if got := tileText(colony, ct); tileCount(colony, ct) != colony.Cargo || got != fmt.Sprintf("Colonizer x%d: colonise from the planet screen", colony.Cargo) {
		t.Errorf("colonizer hover %q", got)
	}
	// The Invasion Modules: the live Invaders as the count, the planet
	// screen as the use in orbit, Usable's refusal out of it.
	inv := a.e.Spawn(game.Design{Name: "Invader", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.InvasionModule, game.InvasionModule}}, colony.Star, colony.Orbit)
	inv.Invaders = 1 // one spent: the count is what is left, not the parts
	it := panelTile0(t, inv, game.InvasionModule)
	if got := tileText(inv, it); tileCount(inv, it) != inv.Invaders || it.n != 2 || got != "Invasion Module x1: invade from the planet screen" {
		t.Errorf("invasion module x%d hover %q", tileCount(inv, it), got)
	}
	inv.Orbit = -1
	_, why := inv.Usable(it.part)
	if got := tileText(inv, it); got != "Invasion Module x1: "+why || why == "invade" {
		t.Errorf("out of orbit hover %q, Usable %q", got, why)
	}
	if got := tileText(colony, panelTile0(t, colony, game.StarLaneDrive)); got != "Star Lane Drive x1: Star Lane Drive is always on" {
		t.Errorf("lane drive hover %q", got)
	}
}

// A weapon tile picks the weapon and enters the fire mode; a click on the
// unshielded Raider fires it (the Level off its hull) and ends the mode;
// a right click, Esc, a click on nothing or on the player's own ship end
// it without firing; the fire line is live in range, not out of it.
func TestPanelFire(t *testing.T) {
	a, v, colony, gun, raider := panelFixture(t)
	v.sel = gun
	w := slices.Index(gun.Design.Parts, game.MassBarrageGun)
	if gun.Weapon != -1 {
		t.Fatalf("Spawn's Weapon %d", gun.Weapon)
	}
	arm := func() {
		t.Helper()
		if err := pressTile(t, v, a, gun, game.MassBarrageGun); err != nil || gun.Weapon != w || !v.aim {
			t.Fatalf("weapon tile: %v, Weapon %d, aim %v", err, gun.Weapon, v.aim)
		}
	}
	arm()
	if f := v.panelFoot(a.e); f != "fire Mass Barrage Gun: click the target (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	raider.Shields = false
	hull := raider.Hull
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b != shipOf(v, raider) || !live {
		t.Errorf("aim at the Raider in range: %v, live %v", b, live)
	}
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || len(gun.Armed) != 1 || v.aim {
		t.Errorf("fire: %v, armed %+v, aim %v", err, gun.Armed, v.aim)
	}
	if a.g.Round(gun.Star, nil); raider.Hull != hull-game.Gizmos[game.MassBarrageGun].Level {
		t.Errorf("the round: hull %d of %d", raider.Hull, hull)
	}
	gun.Refill()
	hull = raider.Hull

	arm()
	if v.rightClick(); v.aim || v.sel != gun || raider.Hull != hull {
		t.Errorf("right click: aim %v, sel %v, hull %d", v.aim, v.sel, raider.Hull)
	}
	arm()
	if !v.cancel() || v.aim || v.sel != gun {
		t.Error("Esc does not end the fire mode, the ship kept")
	}
	for _, b := range []*body{nil, shipOf(v, colony), shipOf(v, gun)} {
		arm()
		if err := v.aimClick(a, b); err != nil || v.aim || raider.Hull != hull || gun.Shots[w] != 0 {
			t.Errorf("click on %v: %v, aim %v, hull %d", b, err, v.aim, raider.Hull)
		}
	}
	arm()
	if err := v.panelClick(a, float64(sysInfo.X1-2), float64(sysInfo.Y1-2)); err != nil || v.aim {
		t.Errorf("a press in the box off the tiles: %v, aim %v", err, v.aim)
	}
	arm()
	if v.rightClick(); v.aim || v.move.on || v.sel != gun {
		t.Errorf("right click: aim %v, mode %+v, sel %v", v.aim, v.move, v.sel)
	}
	v.move = moveMode{}

	// Out of range: the Raider moved 0.1 past the gun's Range × RangeUnit
	// off its orbit (a fixture point); the line goes grey and the
	// click is refused by the rule.
	d := float64(game.Gizmos[game.MassBarrageGun].Range)*game.RangeUnit + 0.1
	raider.Orbit, raider.Pos = -1, game.Vec3{X: gun.Pos.X + d, Y: gun.Pos.Y, Z: gun.Pos.Z}
	v.rebuild()
	if gun.CanFire(a.cl, w, raider) == nil {
		t.Fatal("the far Raider is in range")
	}
	arm()
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b == nil || live {
		t.Errorf("aim at the far Raider: %v, live %v", b, live)
	}
	// Range is checked by the round on its starting positions, not at
	// arming: the shot is armed, then dropped.
	if err := v.aimClick(a, v.hover); err != nil || len(gun.Armed) != 1 {
		t.Errorf("fire out of range: %v, armed %+v", err, gun.Armed)
	}
	if a.g.Round(gun.Star, nil); raider.Hull != hull || gun.Shots[w] != 0 {
		t.Errorf("the round out of range: hull %d, shots %v", raider.Hull, gun.Shots)
	}
	v.hover = shipOf(v, colony)
	if b, _ := v.aimTarget(); b != nil {
		t.Error("the own colony ship is an aim target")
	}
}

// A shield tile toggles the shields; a Colonizer tile orders nothing (its
// order is the planet screen's), nor does an Invasion Module's; another
// special says why; a passive tile does nothing.
func TestPanelUse(t *testing.T) {
	a, v, home, gun, _ := panelFixture(t)
	e := a.e
	v.sel = gun
	for _, want := range []bool{false, true} {
		if err := pressTile(t, v, a, gun, game.IonWrap); err != nil || gun.Shields != want {
			t.Errorf("shield tile: %v, Shields %v", err, gun.Shields)
		}
	}
	before := *gun
	if err := pressTile(t, v, a, gun, game.TonklinMotor); err != nil || gun.Shields != before.Shields || gun.Weapon != before.Weapon || gun.Order != before.Order {
		t.Errorf("a drive tile changed the ship: %v %+v", err, gun)
	}

	v.sel = home
	if err := pressTile(t, v, a, home, game.Colonizer); err != nil || home.Order.Kind != game.NoOrder {
		t.Errorf("colonizer tile at home: %v, order %v", err, home.Order.Kind)
	}
	if err := pressTile(t, v, a, home, game.StarLaneDrive); err == nil || err.Error() != "Star Lane Drive is always on" {
		t.Errorf("lane drive tile: %v", err)
	}

	free := -1
	for p := range e.Cluster.Stars[home.Star].System.Planets {
		if !e.Inhabited(home.Star, p) {
			free = p
			break
		}
	}
	if free < 0 {
		t.Fatal("no uninhabited planet at home")
	}
	away := e.Spawn(fixtureShip, home.Star, free)
	if e.Order(away, game.Order{Kind: game.Colonise}) != nil {
		t.Fatal("the rule refuses Colonise at the free planet: the next check proves nothing")
	}
	away.Order = game.Order{}
	v.sel = away
	if err := pressTile(t, v, a, away, game.Colonizer); err != nil || away.Order.Kind != game.NoOrder {
		t.Errorf("colonizer tile over %s: %v, order %v", e.Planet(away.Star, free).Name, err, away.Order.Kind)
	}
	inv := e.Spawn(game.Design{Name: "Invader", Hull: game.Medium, Parts: []game.Gizmo{game.ProtonShaver, game.TonklinMotor, game.InvasionModule}}, home.Star, free)
	v.sel = inv
	if err := pressTile(t, v, a, inv, game.InvasionModule); err != nil || inv.Order.Kind != game.NoOrder {
		t.Errorf("invasion module tile: %v, order %v", err, inv.Order.Kind)
	}
}

// A lane endpoint click without a selection opens the far star's system,
// as the starmap's click: nothing while the star is unexplored.
func TestSystemLaneToUnexplored(t *testing.T) {
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes}, game.Average)
	e, cl := g.Empires[0], g.Cluster
	home := e.Colonies[0].Star
	far := cl.Other(cl.Stars[home].Lanes[0], home)
	a := testApp(g)
	v := newSystemScreen(home)
	v.Enter(a)
	var end *body
	for i := range v.bodies {
		if b := &v.bodies[i]; b.kind == laneBody && b.lane.To == far {
			end = b
		}
	}
	if end == nil {
		t.Fatalf("no endpoint to %d at home", far)
	}
	if push, _ := v.act(e, end); push != nil {
		t.Errorf("unexplored star %d: the endpoint click pushed %T", far, push)
	}
	e.Visit(far)
	if push, _ := v.act(e, end); push == nil {
		t.Errorf("explored star %d: the endpoint click pushed nothing", far)
	}
}

// The panel's gauges: the hull's fraction and colour by what is left,
// the energy's of the day's supply, 0 of 0 empty, never over 1.
func TestShipBars(t *testing.T) {
	_, _, _, gun, _ := panelFixture(t)
	gun.Refill() // the full supply: the shields draw per round, not here
	full := shipBars(gun)
	if full[0].frac != 1 || full[0].c != barGreen || full[0].num != fmt.Sprintf("%d/%d", gun.Hull, gun.MaxHull()) ||
		full[1].frac != float64(gun.Power)/float64(gun.Design.PowerPerDay()) || full[1].num != fmt.Sprintf("%d/%d", gun.Power, gun.Design.PowerPerDay()) {
		t.Errorf("full: %+v", full)
	}
	gun.Hull = gun.MaxHull() / 3
	gun.Power = 0
	hit := shipBars(gun)
	if want := float64(gun.Hull) / float64(gun.MaxHull()); hit[0].frac != want || hit[0].c != yellow || hit[1].frac != 0 {
		t.Errorf("a third left, no energy: %+v", hit)
	}
	gun.Hull = 1
	if b := shipBars(gun)[0]; b.c != barRed {
		t.Errorf("one point left: %+v", b)
	}
	bare := &game.Starship{Design: game.Design{Hull: game.Small}}
	if b := shipBars(bare); b[1].frac != 0 || b[0].frac > 1 {
		t.Errorf("no generator: %+v", b)
	}
}

// A self-targeted special's tile acts: the Accutron boosts the ship (the
// report is the message in the normal colour, the tile says "boosted"),
// a second click is refused by the model, Hyperfuel adds its energy and
// leaves the panel, the Remote Repair Facility mends, the Recaller sends
// the ship home and deselects it.
func TestPanelSpecialUse(t *testing.T) {
	a, v, _, _, raider := panelFixture(t)
	e := a.e
	home := a.e.Colonies[0].Star
	mk := func(parts ...game.Gizmo) *game.Starship {
		sh := e.Spawn(game.Design{Name: "Special", Hull: game.Enormous, Parts: append([]game.Gizmo{game.MassBarrageGun, game.TonklinMotor}, parts...)}, home, 0)
		sh.Orbit, sh.Pos = -1, game.FreeSpot(game.LeavePoint(e.Cluster, home, 0), nil) // in space: the orbit is the planet screen's
		sh.Power = 1000
		v.rebuild()
		v.sel = sh
		return sh
	}
	_ = raider

	acc := mk(game.Accutron)
	tile := panelTile0(t, acc, game.Accutron)
	if got := tileText(acc, tile); got != "Accutron x1: use" {
		t.Errorf("hover before %q", got)
	}
	pw := acc.Power
	if err := pressTile(t, v, a, acc, game.Accutron); err != nil || !acc.Boosted || acc.Power != pw-game.Gizmos[game.Accutron].Power {
		t.Fatalf("accutron click: %v, Boosted %v, power %d of %d", err, acc.Boosted, acc.Power, pw)
	}
	if !v.msgOK || !strings.Contains(v.msg, "Accutron") || v.msgColour() != textColour || v.tick >= v.msgUntil {
		t.Errorf("report %q ok %v", v.msg, v.msgOK)
	}
	if got := tileText(acc, tile); !strings.Contains(got, "boosted") {
		t.Errorf("hover after %q", got)
	}
	err := pressTile(t, v, a, acc, game.Accutron)
	if err == nil {
		t.Error("a second Accutron click was accepted")
	} else if _, why := acc.Usable(tile.part); err.Error() != why {
		t.Errorf("second click %q, Usable says %q", err, why)
	}
	v.fail(err)
	if v.msgOK || v.msgColour() != errColour {
		t.Error("a refusal shows as a report")
	}

	hf := mk(game.Hyperfuel)
	before, n := hf.Power, len(hf.Design.Parts)
	if err := pressTile(t, v, a, hf, game.Hyperfuel); err != nil || hf.Power != before-game.Gizmos[game.Hyperfuel].Power+game.Gizmos[game.Hyperfuel].Special1 {
		t.Errorf("hyperfuel: %v, power %d from %d", err, hf.Power, before)
	}
	if len(hf.Design.Parts) != n-1 || slices.ContainsFunc(panelTiles(hf), func(p panelTile) bool { return p.g == game.Hyperfuel }) {
		t.Error("the Hyperfuel tile is still there")
	}
	for _, p := range panelTiles(hf) {
		if hf.Design.Parts[p.part] != p.g {
			t.Errorf("tile %+v indexes the wrong part", p)
		}
	}

	rr := mk(game.RemoteRepairFacility)
	rr.Hull = 1
	if err := pressTile(t, v, a, rr, game.RemoteRepairFacility); err != nil || rr.Hull <= 1 {
		t.Errorf("repair: %v, hull %d", err, rr.Hull)
	}

	// Away from home, the Recaller brings the ship back and drops it.
	var far int
	for _, l := range a.g.Cluster.Stars[home].Lanes {
		far = a.g.Cluster.Other(l, home)
		break
	}
	rc := mk(game.Recaller)
	e.Visit(far)
	rc.Star, rc.Orbit = far, -1
	v = newSystemScreen(far)
	v.Enter(a)
	v.sel = rc
	if err := v.useTile(a, panelTile0(t, rc, game.Recaller)); err != nil {
		t.Fatalf("recaller: %v", err)
	}
	if rc.Star != e.HomeStar() || v.sel != nil {
		t.Errorf("recalled ship at %d (home %d), selected %v", rc.Star, e.HomeStar(), v.sel)
	}
}

// A targeted special's tile enters the fire mode with aimSpecial (no use
// yet), the foot naming the target its aim takes; the line is live on a
// candidate within the special's Range × RangeUnit; a click uses it at
// the clicked body (the Disarmer on a hostile ship, the Sacrificial Orb on
// an own one, the Phase Bomb on a hostile planet), the mode ending on a
// use, a refusal (the model's words), Esc, a right click, or a part lost
// meanwhile (a stale aim does nothing).
func TestPanelSpecialAim(t *testing.T) {
	a, v, _, gun, raider := panelFixture(t)
	e := a.e
	home := e.Colonies[0].Star
	mk := func(parts ...game.Gizmo) *game.Starship {
		sh := e.Spawn(game.Design{Name: "Aimer", Hull: game.Enormous, Parts: append([]game.Gizmo{game.MassBarrageGun, game.TonklinMotor}, parts...)}, home, 0)
		sh.Orbit, sh.Pos = -1, game.FreeSpot(game.LeavePoint(e.Cluster, home, 0), nil)
		sh.Power = 1000
		v.rebuild()
		v.sel = sh
		return sh
	}
	lastLog := func() game.Event {
		t.Helper()
		if len(a.log) == 0 {
			t.Fatal("empty log")
		}
		return a.log[len(a.log)-1].Ev
	}

	dis := mk(game.Disarmer, game.Accutron) // the Accutron: what a stale index lands on
	k := slices.Index(dis.Design.Parts, game.Disarmer)
	raider.Orbit, raider.Pos = -1, game.Vec3{X: dis.Pos.X + 0.1, Y: dis.Pos.Y, Z: dis.Pos.Z}
	v.rebuild()
	arm := func(sh *game.Starship, g game.Gizmo) {
		t.Helper()
		pw := sh.Power
		if err := pressTile(t, v, a, sh, g); err != nil || !v.aim || v.aimSpecial != slices.Index(sh.Design.Parts, g) || sh.Power != pw {
			t.Fatalf("%s tile: %v, aim %v, aimSpecial %d, power %d of %d", g, err, v.aim, v.aimSpecial, sh.Power, pw)
		}
	}
	arm(dis, game.Disarmer)
	if f := v.panelFoot(e); f != "use Disarmer: click a hostile ship (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("aim at the near Raider: %v, live %v", b, live)
	}
	v.hover = shipOf(v, gun)
	if b, _ := v.aimTarget(); b != nil {
		t.Error("an own ship is a Disarmer candidate")
	}

	// Refused on an own ship: the rule's words, nothing changed, the mode over.
	parts := len(gun.Design.Parts)
	err := v.aimClick(a, shipOf(v, gun))
	if err == nil || v.aim || v.aimSpecial != -1 || len(gun.Design.Parts) != parts || dis.Shots[k] != 0 {
		t.Fatalf("Disarmer on an own ship: %v, aim %v, parts %d of %d", err, v.aim, len(gun.Design.Parts), parts)
	}
	v.fail(err)
	if v.msgOK || v.msg != err.Error() {
		t.Errorf("refusal shown as %q ok %v", v.msg, v.msgOK)
	}

	// Esc and the right click end the mode, nothing used.
	arm(dis, game.Disarmer)
	if !v.cancel() || v.aim || v.aimSpecial != -1 || v.sel == nil {
		t.Error("Esc does not end the special's aim, the ship kept")
	}
	arm(dis, game.Disarmer)
	if v.rightClick(); v.aim || v.aimSpecial != -1 {
		t.Errorf("right click: aim %v", v.aim)
	}

	// Out of range: the line goes grey.
	far := raider.Pos
	raider.Pos.X = dis.Pos.X + float64(game.Gizmos[game.Disarmer].Range)*game.RangeUnit + 0.1
	v.rebuild()
	arm(dis, game.Disarmer)
	v.hover = shipOf(v, raider)
	if b, live := v.aimTarget(); b == nil || live {
		t.Errorf("aim at the far Raider: %v, live %v", b, live)
	}
	raider.Pos = far
	v.rebuild()
	v.hover = shipOf(v, raider)

	// The use: the Raider's first weapon lost, the report logged and shown.
	w := raider.Design.Weapons()
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || v.aim || v.aimSpecial != -1 {
		t.Fatalf("Disarmer on the Raider: %v, aim %v", err, v.aim)
	}
	if raider.Design.Weapons() != w-1 {
		t.Errorf("Raider weapons %d of %d", raider.Design.Weapons(), w)
	}
	if ev := lastLog(); ev.Kind != game.SpecialUsed || !v.msgOK || v.msg != ev.Text {
		t.Errorf("log %+v, msg %q ok %v", ev, v.msg, v.msgOK)
	}
	// The round log shows it at once in violet, the Raider never named
	// (nor in the line shown).
	if ls := v.rlog.shown(v.tick); len(ls) == 0 || ls[len(ls)-1].c != useColour || ls[len(ls)-1].t != v.tick {
		t.Errorf("round log %+v", ls)
	} else if txt, _ := lineText(ls[len(ls)-1]); !strings.HasPrefix(txt, dis.Name+"  Disarmer → ") || strings.Contains(txt, raider.Name) || strings.Contains(v.msg, raider.Name) {
		t.Errorf("use line %q, msg %q", txt, v.msg)
	}

	// A stale aim (the part shifted by a loss meanwhile) does nothing.
	dis.Shots[k] = 0 // a new day's use
	arm(dis, game.Disarmer)
	dis.LosePart(0)
	w = raider.Design.Weapons()
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || v.aim || raider.Design.Weapons() != w {
		t.Errorf("stale aim: %v, aim %v, weapons %d of %d", err, v.aim, raider.Design.Weapons(), w)
	}

	// The Sacrificial Orb on an own damaged ship heals it.
	mate := mk()
	orb := mk(game.SacrificialOrb)
	mate.Hull -= 3
	v.rebuild()
	arm(orb, game.SacrificialOrb)
	if f := v.panelFoot(e); f != "use Sacrificial Orb: click a ship of yours or an ally's (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	v.hover = shipOf(v, mate)
	if b, live := v.aimTarget(); b != v.hover || !live {
		t.Errorf("aim at the mate: %v, live %v", b, live)
	}
	for _, b := range []*body{shipOf(v, orb), shipOf(v, raider)} {
		if v.hover = b; func() bool { b, _ := v.aimTarget(); return b != nil }() {
			t.Errorf("%s is an Orb candidate", b.ship.Name)
		}
	}
	hull, own := mate.Hull, orb.Hull
	if err := v.aimClick(a, shipOf(v, mate)); err != nil || mate.Hull != hull+3 || orb.Hull != own-3 || v.aim {
		t.Errorf("orb: %v, mate %d from %d, orb %d from %d", err, mate.Hull, hull, orb.Hull, own)
	}

	// The Phase Bomb at empire 1's home razes a structure, the device lost
	// and its tile gone (the owner's Razed report is empire 1's log).
	e1 := a.g.Empires[1]
	home1 := e1.Colonies[0]
	p := home1.Planet
	e.Visit(home1.Star)
	bomb := e.Spawn(game.Design{Name: "Bomber", Hull: game.Enormous, Parts: []game.Gizmo{game.MassBarrageGun, game.TonklinMotor, game.PhaseBomb}}, home1.Star, home1.PlanetIdx)
	bomb.Orbit, bomb.Pos, bomb.Power = -1, p.Pos, 1000
	var cell game.Cell
	found := false
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item == game.NoItem {
			cell, found = c, true
			break
		}
	}
	if !found {
		t.Fatal("no free cell on empire 1's home")
	}
	p.Surface[cell].Item = game.Factory
	v = newSystemScreen(home1.Star)
	v.Enter(a)
	v.sel = bomb
	arm(bomb, game.PhaseBomb)
	if f := v.panelFoot(e); f != "use Phase Bomb: click a planet (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	var pb *body
	for i := range v.bodies {
		if b := &v.bodies[i]; b.kind == planetBody && b.idx == home1.PlanetIdx {
			pb = b
		}
	}
	if pb == nil {
		t.Fatal("no body for empire 1's home planet")
	}
	v.hover = pb
	if b, live := v.aimTarget(); b != pb || !live {
		t.Errorf("aim at the planet: %v, live %v", b, live)
	}
	if err := v.aimClick(a, pb); err != nil || v.aim {
		t.Fatalf("phase bomb: %v, aim %v", err, v.aim)
	}
	if p.Surface[cell].Item == game.Factory {
		t.Error("the Factory stands")
	}
	if slices.Contains(bomb.Design.Parts, game.PhaseBomb) || slices.ContainsFunc(panelTiles(bomb), func(pt panelTile) bool { return pt.g == game.PhaseBomb }) {
		t.Error("the Phase Bomb is still aboard")
	}
	if ev := lastLog(); ev.Kind != game.SpecialUsed || !strings.Contains(ev.Text, "Phase Bomb") {
		t.Errorf("log %+v", ev)
	}
}

// The positional and lane specials' aim: any ship but the user (the
// Tractor Beam pulls a hostile in, an own ship is a candidate too), a
// lane opening for the Lane Blocker and Endoscope (Index into
// System.Lanes, live within Range × RangeUnit of the endpoint), and the
// self-targeted Catapult used by its tile at once, its ship kept selected.
func TestPanelPositionalAim(t *testing.T) {
	a, v, _, gun, raider := panelFixture(t)
	e := a.e
	home := e.Colonies[0].Star
	mk := func(parts ...game.Gizmo) *game.Starship {
		sh := e.Spawn(game.Design{Name: "Aimer", Hull: game.Enormous, Parts: append([]game.Gizmo{game.MassBarrageGun, game.TonklinMotor}, parts...)}, home, 0)
		sh.Orbit, sh.Pos = -1, game.FreeSpot(game.LeavePoint(e.Cluster, home, 0), nil)
		sh.Power = 1000
		v.rebuild()
		v.sel = sh
		return sh
	}
	arm := func(sh *game.Starship, g game.Gizmo) {
		t.Helper()
		if err := pressTile(t, v, a, sh, g); err != nil || !v.aim || v.aimSpecial != slices.Index(sh.Design.Parts, g) {
			t.Fatalf("%s tile: %v, aim %v, aimSpecial %d", g, err, v.aim, v.aimSpecial)
		}
	}
	candidate := func(b *body) (*body, bool) {
		v.hover = b
		return v.aimTarget()
	}

	// The Tractor Beam: a hostile in range live, an own ship a candidate.
	tb := mk(game.GizmoTractorBeam)
	raider.Orbit, raider.Pos = -1, game.Vec3{X: tb.Pos.X + 0.3, Y: tb.Pos.Y, Z: tb.Pos.Z}
	gun.Orbit, gun.Pos = -1, game.Vec3{X: tb.Pos.X, Y: tb.Pos.Y, Z: tb.Pos.Z + 0.3}
	v.rebuild()
	arm(tb, game.GizmoTractorBeam)
	if f := v.panelFoot(e); f != "use Tractor Beam: click any ship (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	if b, live := candidate(shipOf(v, raider)); b == nil || !live {
		t.Errorf("hostile: %v, live %v", b, live)
	}
	if b, _ := candidate(shipOf(v, gun)); b == nil {
		t.Error("an own ship is no Tractor Beam candidate")
	}
	if b, _ := candidate(shipOf(v, tb)); b != nil {
		t.Error("the user is a candidate")
	}
	d0 := vecDist(tb.Pos, raider.Pos)
	if err := v.aimClick(a, shipOf(v, raider)); err != nil || v.aim {
		t.Fatalf("tractor: %v, aim %v", err, v.aim)
	}
	if d := vecDist(tb.Pos, raider.Pos); d >= d0 {
		t.Errorf("Raider distance %v from %v", d, d0)
	}

	// The lane devices: a lane opening, Index into System.Lanes.
	lb := mk(game.LaneBlocker, game.LaneEndoscope)
	sys := v.sys
	if len(sys.Lanes) == 0 {
		t.Fatal("no lane at home")
	}
	li := 0
	lane := &sys.Lanes[li]
	r := float64(game.Gizmos[game.LaneBlocker].Range) * game.RangeUnit
	lb.Pos = game.Vec3{X: lane.Pos.X, Y: lane.Pos.Y, Z: lane.Pos.Z}
	v.rebuild()
	v.sel = lb
	var lbody *body
	for i := range v.bodies {
		if b := &v.bodies[i]; b.kind == laneBody && b.lane == lane {
			lbody = b
		}
	}
	if lbody == nil {
		t.Fatal("no body for the lane")
	}
	arm(lb, game.LaneBlocker)
	if f := v.panelFoot(e); f != "use Lane Blocker: click a star lane opening (right click cancels)" {
		t.Errorf("prompt %q", f)
	}
	if b, live := candidate(lbody); b != lbody || !live {
		t.Errorf("lane in range: %v, live %v", b, live)
	}
	if b, _ := candidate(shipOf(v, raider)); b != nil {
		t.Error("a ship is a lane candidate")
	}
	lb.Pos.X += r + 0.1
	if b, live := candidate(lbody); b != lbody || live {
		t.Errorf("lane out of range: %v, live %v", b, live)
	}
	lb.Pos.X -= r + 0.1

	// The Endoscope first (not one-time): the far star becomes known.
	far := e.Cluster.Other(e.Cluster.Stars[home].Lanes[li], home)
	if e.Knows(far) {
		t.Fatal("fixture: far star known")
	}
	v.aim, v.aimSpecial = false, -1
	arm(lb, game.LaneEndoscope)
	if err := v.aimClick(a, lbody); err != nil || !e.Knows(far) {
		t.Fatalf("endoscope: %v, knows %v", err, e.Knows(far))
	}

	arm(lb, game.LaneBlocker)
	if err := v.aimClick(a, lbody); err != nil || e.LaneBlocks[e.Cluster.Stars[home].Lanes[li]] <= e.Day {
		t.Fatalf("blocker: %v, blocks %v", err, e.LaneBlocks)
	}
	if ev := a.log[len(a.log)-1].Ev; ev.Kind != game.SpecialUsed {
		t.Errorf("log %+v", ev)
	}
	if slices.ContainsFunc(panelTiles(lb), func(pt panelTile) bool { return pt.g == game.LaneBlocker }) {
		t.Error("the Lane Blocker tile stays")
	}

	// The Catapult: the tile alone, the ship mirrored and still selected.
	if v.replay != nil { // an earlier use's effects still playing
		v.skipReplay(a)
	}
	cat := mk(game.GravimetricCatapult)
	from := cat.Pos
	if err := pressTile(t, v, a, cat, game.GravimetricCatapult); err != nil || v.aim {
		t.Fatalf("catapult: %v, aim %v", err, v.aim)
	}
	if v.sel != cat || vecDist(cat.Pos, from) < 1e-6 || vecDist(cat.Pos, game.Vec3{X: -from.X, Y: -from.Y, Z: -from.Z}) > 0.2 {
		t.Errorf("catapult: sel %v, from %v to %v", v.sel == cat, from, cat.Pos)
	}
	// Its move plays at once (noteUse's replay): drawn where it stood,
	// then at the new position once the replay ends.
	if b := shipOf(v, cat); v.replay == nil || b == nil || b.pos != from {
		t.Fatalf("the catapult's move not replayed from %v: replay %v", from, v.replay != nil)
	}
	v.tick = v.replay.t0 + v.replay.length()
	v.replayTick(a)
	v.rebuild()
	if b := shipOf(v, cat); v.replay != nil || b == nil || b.pos != cat.Pos {
		t.Error("bodies not rebuilt at the new position")
	}
}

// Every ship-targeted special is aimed at a ship on the screen, as the
// model aims it (the Myrmidonic Carbonizer, its flags naming lanes and
// planets too, must not take a lane opening).
func TestSpecialAimKindShips(t *testing.T) {
	for g := range game.Gizmo(len(game.Gizmos)) {
		in := &game.Gizmos[g]
		if in.Cat != game.Special || in.Flags&game.GzTargetShip == 0 {
			continue
		}
		if k := specialAimKind(g); k != aimHostileShip && k != aimAllyShip && k != aimAnyShip {
			t.Errorf("%s: aim kind %d, want a ship", in.Name, k)
		}
	}
	if k := specialAimKind(game.MyrmidonicCarbonizer); k != aimHostileShip {
		t.Errorf("Carbonizer: aim kind %d, want a hostile ship", k)
	}
}

// A tile groups the parts of one gizmo: once the first is spent today,
// the tile uses the next one with a use left (as the weapons' aimPart),
// and its text tells the group's verdict (not "no use
// left today" under an "x1").
func TestPanelSpecialGroup(t *testing.T) {
	a, v, _, _, _ := panelFixture(t)
	e := a.e
	home := e.Colonies[0].Star
	sh := e.Spawn(game.Design{Name: "Two", Hull: game.Enormous, Parts: []game.Gizmo{game.MassBarrageGun, game.TonklinMotor, game.Cannibalizer, game.Cannibalizer}}, home, 0)
	sh.Orbit, sh.Pos = -1, game.FreeSpot(game.LeavePoint(e.Cluster, home, 0), nil)
	v.rebuild()
	v.sel = sh
	for n := range 2 {
		if err := pressTile(t, v, a, sh, game.Cannibalizer); err != nil {
			t.Fatalf("use %d: %v", n+1, err)
		}
		if n == 0 {
			if l := tileText(sh, panelTile0(t, sh, game.Cannibalizer)); l != "Cannibalizer x1: use" {
				t.Errorf("after one use: %q", l)
			}
		}
	}
	if sh.Shots[2] != 1 || sh.Shots[3] != 1 {
		t.Errorf("shots %v, want both Cannibalizers used", sh.Shots)
	}
}

// The strip's star icon, with the flag on its corner, and the title
// share the strip: the icon inside the strip's opaque hole (orig [4,44)
// high, measured on the panel frame) and big enough for the flag, the
// title's lines right of it.
func TestStripLayout(t *testing.T) {
	r := stripIcon
	if r.X0 < sysStrip.X0 || r.X1 > sysStrip.X1 || r.Y0 < 4*zoom || r.Y1 > 44*zoom {
		t.Errorf("%v leaves the strip's hole", r)
	}
	if r.X1-r.X0 < flagW || r.Y1-r.Y0 < flagH {
		t.Errorf("the icon %v cannot carry a %dx%d flag", r, flagW, flagH)
	}
	tw := float64(sysStrip.X1-6*zoom) - stripTextX()
	for _, l := range []string{"Light Yellow Medium", "Day 1234  12 lanes"} {
		if tw < ui.Width(l, bigFace) {
			t.Errorf("the title's room %v px cuts %q", tw, l)
		}
	}
}

// The panel's Stop button is X for the mouse: shown for the empire's own
// selected ship only, lit while it has an order or the auto mode runs;
// a click ends them, a dim one does nothing; the rect sits in the info
// box above the tiles.
func TestPanelStop(t *testing.T) {
	a, v, _, gun, raider := panelFixture(t)
	r := stopRect()
	if r.Y1 > panelBandsTop || r.X0 < sysInfo.X0 || r.X1 > sysInfo.X1 || r.Y0 < sysInfo.Y0 {
		t.Errorf("stopRect %+v not in the info box %+v above the tiles (%d)", r, sysInfo, panelBandsTop)
	}
	cx, cy := float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2
	v.sel = gun
	gun.Order = game.Order{Kind: game.GoToStar, Star: gun.Star}
	if sh, on := v.stopState(a.e); !sh || !on {
		t.Fatalf("ship with an order: shown %v, on %v", sh, on)
	}
	v.aim = true
	if err := v.panelClick(a, cx, cy); err != nil || gun.Order.Kind != game.NoOrder || v.aim {
		t.Errorf("click with an order: %v, order %v, aim %v", err, gun.Order.Kind, v.aim)
	}
	if sh, on := v.stopState(a.e); !sh || on {
		t.Errorf("ship without order: shown %v, on %v", sh, on)
	}
	v.aim = true
	before := *gun
	if err := v.panelClick(a, cx, cy); err != nil || gun.Order != before.Order || gun.Shields != before.Shields || !v.aim {
		t.Errorf("dim click: %v, order %+v, aim %v", err, gun.Order, v.aim)
	}
	v.aim = false
	v.auto = true
	if sh, on := v.stopState(a.e); !sh || !on {
		t.Errorf("auto mode: shown %v, on %v", sh, on)
	}
	if err := v.panelClick(a, cx, cy); err != nil || v.auto {
		t.Errorf("click in auto mode: %v, auto %v", err, v.auto)
	}

	v.sel = raider
	raider.Order = game.Order{Kind: game.GoToStar, Star: raider.Star}
	if sh, on := v.stopState(a.e); sh || on {
		t.Errorf("foreign ship: shown %v, on %v", sh, on)
	}
	if err := v.panelClick(a, cx, cy); err != nil || raider.Order.Kind != game.GoToStar {
		t.Errorf("click on a foreign ship's panel: %v, order %v", err, raider.Order.Kind)
	}
}

// Right click cancels one layer per click: the height step, the aim, the
// selected ship, the selected planet, then nothing; it never leaves the
// screen and never orders or selects. Left click in the step orders
// GoToPoint whatever is under the cursor, a planet's body included.
func TestRightClickLayers(t *testing.T) {
	a, v, _, gun, _ := panelFixture(t)
	a.push(v)
	depth := len(a.stack)
	home := a.e.Colonies[0]
	pb := v.planetBody(home.PlanetIdx)
	v.cam = sysCamera()
	v.sel, v.psel = gun, home.Planet
	v.move = moveMode{on: true, ok: true}
	v.aim = true
	steps := []struct {
		name string
		gone func() bool
	}{
		{"step", func() bool { return !v.move.on && v.aim && v.sel == gun && v.psel != nil }},
		{"aim", func() bool { return !v.aim && v.sel == gun && v.psel != nil }},
		{"ship", func() bool { return v.sel == nil && v.psel != nil }},
		{"planet", func() bool { return v.sel == nil && v.psel == nil }},
		{"nothing", func() bool { return v.sel == nil && v.psel == nil }},
	}
	order := gun.Order
	for _, st := range steps {
		v.rightClick()
		if !st.gone() || len(a.stack) != depth || a.top() != screen(v) || gun.Order != order {
			t.Fatalf("right click, layer %s: move %v aim %v sel %v psel %v, %d screens", st.name, v.move.on, v.aim, v.sel, v.psel, len(a.stack))
		}
	}

	// The step's click orders over a planet body, the ship kept selected.
	v.sel = gun
	q := v.cam.Project(pb.pos)
	v.plant(q.X, q.Y)
	if !v.move.on {
		t.Fatal("no step planted")
	}
	v.moveTrack(q.X, q.Y)
	pt := v.move.pt
	v.click(a, q.X, q.Y)
	if v.move.on || v.sel != gun || gun.Order != (game.Order{Kind: game.GoToPoint, Pos: pt}) {
		t.Errorf("click in the step: mode %+v sel %v order %+v, want point %v", v.move, v.sel, gun.Order, pt)
	}
}

// A screen opened with a ship selected (the fleet's row, newSystemScreenAt)
// previews its first click as a selection in the view does.
func TestMovePreviewOpenedSelected(t *testing.T) {
	_, v0, ships := shipScreen(t)
	sh := ships[0]
	sh.Orbit = -1 // in space: one in orbit is dropped at Enter
	v := newSystemScreenAt(sh.Star, sh)
	v.Enter(testApp(v0.g))
	if v.sel != sh {
		t.Fatalf("sel %v, want the ship", v.sel)
	}
	v.cam = sysCamera()
	q := v.cam.Project(game.Vec3{X: 0.3, Y: sh.Pos.Y, Z: -0.4})
	if v.ghostAt(q.X, q.Y); !v.ghost.on {
		t.Errorf("no preview on a screen opened with the ship selected: %+v", v.ghost)
	}
}
