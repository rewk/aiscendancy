package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The system display: one star, its planets and lane endpoints in 3D,
// the original's sprites projected over a perspective grid disc.
//
// Mouse: hover a ship to see it in the info box, another body on the
// view's bottom line (hoverLine); a planet click selects it, a second
// opens it (sysplanet.go); a lane endpoint click pushes the far star's
// system; left-drag rotates, the wheel zooms. A ship click selects it:
// its parts fill the info box (panelTiles), a weapon tile enters the
// fire mode (aim), a shield tile toggles the shields. A selected ship is
// sent by a click on a planet (its orbit), a lane endpoint, or nothing:
// a point whose height a second click fixes (moveMode); a right click
// cancels one layer at a time (rightClick). Ships in orbit are hidden
// here (orbitHidden: the planet screen's), kept in the lists. An order
// moves one hop (one MoveCost); refusals show in the info box. Stop
// (stopRect) is X: it cancels the order; Navigate… (navRect) is N: a
// starmap picks any system (navigate.go). Keys: X cancels, F fires, D
// shields, M repeats, A auto, arrows pan, G grid, E elevation lines, R
// resets, S the species screen; F1 lists them (keys.go). At a fight the
// screen is in combat mode (inCombat, roundmode.go).
//
// The chrome is 16battle.tmp frame 0 (640x480), measured as the largest
// transparent run whose 1-px ring holds none. The big area opens above
// y 142 into the top-right region, transparent and borderless (the
// original's panel: drawSysPanel). The info box is filled black over the
// chrome (trimming the 3D spill); the chrome's box ends at y 395 over six
// unused button holes (rows 403-433: x 479-510, 518-549, 557-588,
// 596-627; rows 442-472: 479-549, 596-627): ours runs over them, a
// half-black 1-px rim hiding the cut (drawInfoRim). Original px.
var (
	sysArea    = orig(7, 7, 466, 473)
	sysTop     = orig(466, 0, 640, 142) // the panel's place: the star's name, the buttons
	sysInfo    = orig(474, 142, 633, 473)
	sysInfoBox = orig(474, 142, 633, 395) // the chrome's own box: the extension is below it
)

// sysTrim is everything right of the big area, painted black before the
// chrome (see mapTrim).
var sysTrim = rect{sysArea.X1, 0, screenW, screenH}

// sysCamera puts the origin 95 original px below the big area's centre,
// as the original's disc (y 335 of 480): the bodies float above the plane.
func sysCamera() cam.Camera {
	return cam.Default(float64(sysArea.X0+sysArea.X1)/2, float64(sysArea.Y0+sysArea.Y1)/2+zoom*95)
}

var (
	gridColour = color.RGBA{40, 90, 230, 255}
	elevColour = color.RGBA{255, 255, 0, 255}
	footColour = color.RGBA{255, 255, 255, 255}
	hoverColor = color.RGBA{0, 255, 0, 255}
)

// gridLines are the grid's segments x = k/10 and z = k/10, k = −9..9,
// clipped to the unit disc: both ends on the unit circle, projected and
// joined (a 3D segment projects straight).
func gridLines() [][2]game.Vec3 {
	var out [][2]game.Vec3
	for k := -9; k <= 9; k++ {
		a := float64(k) / 10
		h := math.Sqrt(1 - a*a)
		out = append(out,
			[2]game.Vec3{{X: a, Z: -h}, {X: a, Z: h}},
			[2]game.Vec3{{X: -h, Z: a}, {X: h, Z: a}})
	}
	return out
}

type bodyKind uint8

const (
	starBody bodyKind = iota
	planetBody
	laneBody
	shipBody
)

// body is one thing the display draws and hovers: the star, a planet, a
// lane endpoint or a ship.
type body struct {
	kind    bodyKind
	pos     game.Vec3
	star    game.StarType  // starBody
	planet  *game.Planet   // planetBody
	idx     int            // planetBody: the planet's index in its system; laneBody: the lane's in System.Lanes
	lane    *game.Lane     // laneBody: its To is a STAR index
	ship    *game.Starship // shipBody: a model pointer, stable across rebuilds
	species game.Species   // shipBody: the owner's, for the sprite bank
	owner   int            // shipBody: the owner's index in Game.Empires
	// seen is a foreign ship as the player sees it: sprite and text read
	// it, never ship's design (ship is only the identity). nil on ours.
	seen *game.Sighting
	// orbiting: in orbit slot orbit, hidden from the view, in the lists.
	orbiting bool
	orbit    int
	// castSeen is a replay cast's foreign ship as the player saw it when
	// the cast was taken (replayCast), for the round log's names.
	castSeen *game.Sighting
}

// shipIconBank's frame shieldIcon is the shield sphere of the 13x13 part
// icons (frames 6-11: drive, weapon, shield, special, scanner, generator;
// formats.md), drawn at its pixel size, shieldIconW wide (shieldRow).
const (
	shipIconBank = "shipicon"
	shieldIcon   = 8
	shieldIconW  = 13
)

// shipBank is a species' ship sprites: smship00..20 in Species order
// (ASSUMPTION, as raceNN.dip), a frame per hull, 158x112, origin centred.
func shipBank(sp game.Species) string { return fmt.Sprintf("smship%02d", int(sp)) }

// sprite is the body's bank and frame. planets.shp is type-major, five
// sizes per type; lanes.shp frame 0 is the blue sphere, 1 the red.
func (b body) sprite() (string, int) {
	switch b.kind {
	case planetBody:
		return "planets", int(b.planet.Type)*game.MaxSize + b.planet.Size - 1
	case laneBody:
		if b.lane.Red {
			return "lanes", 1
		}
		return "lanes", 0
	case shipBody:
		if b.seen != nil {
			return shipBank(b.species), int(b.seen.Hull)
		}
		return shipBank(b.species), int(b.ship.Design.Hull)
	}
	return "suns", int(b.star)
}

// radius is the body's size in world units (the grid disc's radius is 1).
func (b body) radius() float64 {
	switch b.kind {
	case planetBody:
		return game.PlanetRadius(b.planet.Size)
	case laneBody:
		return 0.07
	case shipBody:
		return 0.05
	}
	return 0.10
}

// placed is a body projected for this frame.
type placed struct {
	*body
	pt cam.Point
	r  float64 // screen radius
}

// systemBodies lists the star above the origin, the planets, then the lanes.
func systemBodies(s *game.StarSystem) []body {
	bs := []body{{kind: starBody, pos: game.Vec3{Y: game.StarHeight}, star: s.Star}}
	for i, p := range s.Planets {
		bs = append(bs, body{kind: planetBody, pos: p.Pos, planet: p, idx: i})
	}
	for i := range s.Lanes {
		bs = append(bs, body{kind: laneBody, pos: s.Lanes[i].Pos, lane: &s.Lanes[i], idx: i})
	}
	return bs
}

// seenBodies is systemBodies under e's fog: planets when e knows the
// star, the endpoints of the lanes it knows. A nil e sees everything.
func seenBodies(e *game.Empire, star int, sys *game.StarSystem) []body {
	if e == nil {
		return systemBodies(sys)
	}
	bs := []body{{kind: starBody, pos: game.Vec3{Y: game.StarHeight}, star: sys.Star}}
	if e.Knows(star) {
		for i, p := range sys.Planets {
			bs = append(bs, body{kind: planetBody, pos: p.Pos, planet: p, idx: i})
		}
	}
	for k, li := range e.Cluster.Stars[star].Lanes { // li is sys.Lanes[k]'s cluster lane
		if e.KnowsLane(li) {
			bs = append(bs, body{kind: laneBody, pos: sys.Lanes[k].Pos, lane: &sys.Lanes[k], idx: k})
		}
	}
	return bs
}

// shipBodies lists the ships whose Star is star (a lane ship stays until
// arrival, at its endpoint), Empires then Ships order; one in orbit is
// marked orbiting. The moving ship (nil none) is at at, never orbiting.
func shipBodies(g *game.Game, star int, moving *game.Starship, at game.Vec3) []body {
	var bs []body
	for i, e := range g.Empires {
		for _, sh := range e.Ships {
			if sh.Star != star {
				continue
			}
			b := body{kind: shipBody, pos: sh.Pos, ship: sh, species: e.Species, owner: i}
			switch {
			case sh == moving:
				b.pos = at
			case sh.Orbit >= 0 && sh.Lane < 0:
				b.orbiting, b.orbit = true, sh.Orbit
			}
			bs = append(bs, b)
		}
	}
	return bs
}

// sighted keeps of bs the player's ships and the foreign ones in seen,
// each carrying its Sighting, at its Pos out of orbit. Paired by order
// (ShipsSeen walks Empires then Ships, skipping lane ships, as
// shipBodies); a body that is not seen's next (Sighting.Of: a cloaked
// orbit's) is dropped.
func sighted(bs []body, seen []game.Sighting, player int) []body {
	var out []body
	for _, b := range bs {
		switch {
		case b.owner == player:
			out = append(out, b)
		case b.ship.Lane >= 0 || len(seen) == 0: // in a lane out: LaneTraffic's, not drawn
		default:
			sg := seen[0]
			if sg.Empire != b.owner || !sg.Of(b.ship) {
				continue
			}
			seen = seen[1:]
			b.seen = &sg
			if sg.Orbit < 0 {
				b.pos = sg.Pos
			}
			out = append(out, b)
		}
	}
	return out
}

// drawnAt is where sh is drawn now: the anim's point for the moving ship,
// else its Pos.
func (s *systemScreen) drawnAt(sh *game.Starship) game.Vec3 {
	if sh == s.anim.ship {
		return s.animPos()
	}
	return sh.Pos
}

// orbitHidden reports b is neither drawn nor picked: a ship in orbit,
// shown by its planet's guardRing and on the planet screen.
func orbitHidden(b *body) bool { return b.kind == shipBody && b.orbiting }

// pickBody is the body whose projected disc holds (x,y), the nearest one
// on an overlap: the last such in back-to-front order.
func pickBody(ps []placed, x, y float64) *body {
	var found *body
	for _, p := range ps {
		if math.Hypot(x-p.pt.X, y-p.pt.Y) <= p.r {
			found = p.body
		}
	}
	return found
}

type systemScreen struct {
	star   int // index in the cluster
	sys    *game.StarSystem
	g      *game.Game   // set by Enter
	e      *game.Empire // nil in bare tests: its fog applies (seenBodies)
	player int          // only its ships are selected and ordered
	bodies []body

	cam        cam.Camera
	grid, elev bool
	drag       drag
	hover      *body              // into bodies, valid within the tick; nil none
	sel        *game.Starship     // the selected ship (the player's), nil none
	gunTicks   map[game.Gizmo]int // app.gunTicks: the replayed shots' flights (flyTicks)
	sndTicks   map[string]int     // app.sndTicks: the impacts' and blasts' lengths (impactTicks, blastTicks)
	// last is the ship order dropped (a destination), Repeat Move's
	// without a selection.
	last *game.Starship
	// fsel is the foreign ship inspected (its read-only panel, foreignText),
	// nil none; never set with sel or psel. back is our ship selected when
	// fsel was taken, selected again when fsel drops (dropForeign).
	fsel, back *game.Starship
	// psel is the selected planet, nil none; never set with sel.
	psel     *game.Planet
	msg      string // last order refusal (or special's report: msgOK), shown until msgUntil
	msgUntil int
	msgOK    bool // msg is a success report: textColour, not errColour
	tick     int
	move     moveMode
	ghost    moveMode // the first click's preview (ghostAt), off when none
	// placing is the first click pending: set by a ship's selection,
	// cleared by any click in the view (an order, a shot, a foot).
	placing bool
	anim    anim
	// auto is the Auto run: each anim's end hops the same ship again, else
	// the next ordered one (autoNext); cleared when none moves, by a new
	// order or a day's close.
	auto bool
	// pending is the left button's run (hopAll): the ships still to hop
	// once each, one per anim's end; cleared as auto.
	pending []*game.Starship
	btn     int // the panel button hovered (sysButton), noButton for none
	// aim is the fire mode (the ship's part sel.Weapon, or the planet's
	// slot pslot): a click on a hostile ship (or, a ship's, a planet) fires
	// there (aimClick); any other click, a right click or Esc ends it.
	// With psel, ptract makes it the planet's tractor mode: a click on any
	// ship pulls it (planetTractor).
	aim    bool
	pslot  int
	ptract bool
	// aimSpecial is the targeted special part aimed, −1 a weapon's aim;
	// aimG its gizmo: a part lost meanwhile shifts the indices, so an aim
	// whose part no longer holds aimG is dropped (specialAim).
	aimSpecial int
	aimG       game.Gizmo
	stopHot    bool // the mouse is over a shown Stop button
	navHot     bool // the mouse is over a shown, enabled Navigate… button
	tile       int  // the panel tile hovered (index into panelTiles(sel), or foreignTiles), −1 none
	// replay is the day's combat rounds being redrawn, nil none: its cast
	// replaces the ship bodies.
	replay *replay
	rlog   *roundLog // the replay's round log overlay (roundlog.go), nil none
	// rings are the planets' guard rings before the rounds being
	// replayed (snapRings), drawn while the replay runs: the model is
	// already past them, a shield shot down mid-series.
	rings  map[int]color.RGBA
	flares []flare // lane endpoints lit by a ship entering (flare.go), one each
	cues   []cue   // sound effects waiting for their tick (sfx.go)
	// The info box's lists (syslists.go): listHover the hover came from a
	// row, listTab the tab, listOff the scrolls.
	listHover bool
	listTab   int
	listOff   [2]int
	news      newsView // in newsBox, over the info box's foot, with nothing selected (newsOn)
}

// anim is the ship hop being drawn (ship nil: none): the model already
// stands at to, the body slides from from over hopTicks ticks from t0;
// a day's close (Day != day) drops it.
type anim struct {
	ship     *game.Starship // a model pointer, never a body
	from, to game.Vec3
	t0, day  int
	bounce   bool      // off a barred orbit: from, touch, to (bounce.go)
	touch    game.Vec3 // the bounce's turn
}

// ticks is the anim's length: a bounce flies half as long again.
func (m anim) ticks() int {
	if m.bounce {
		return hopTicks * 3 / 2
	}
	return hopTicks
}

// hopTicks is a hop's animation: half a second at 60 ticks.
const hopTicks = 30

// moveMode is the height step of a free move: a click on nothing, a ship
// selected, plants the foot on the plane at the ship's height (plant),
// the cross follows the cursor's y along the vertical (moveTrack), a
// second click orders GoToPoint. A right click or Esc ends it; the cross
// is clamped within game.SystemReach.
type moveMode struct {
	on   bool
	foot game.Vec3 // (x, 0, z), planted by the first click
	pt   game.Vec3 // the cross's world point this tick
	ok   bool      // the cursor unprojects to something usable this tick
}

// moveReach is a hair inside game.SystemReach, so a point scaled onto
// the sphere is not refused by a rounding ulp.
const moveReach = game.SystemReach - 1e-9

const moveHelp = "height: click to go\nright click or Esc: cancel"

func newSystemScreen(star int) *systemScreen { return newSystemScreenAt(star, nil) }

// newSystemScreenAt opens star's system with sel selected (nil: none),
// its first click previewed (placing) as a selection's; Enter's rebuild
// drops it in orbit, in a lane or elsewhere.
func newSystemScreenAt(star int, sel *game.Starship) *systemScreen {
	return &systemScreen{star: star, cam: sysCamera(), grid: true, elev: true, sel: sel, placing: sel != nil, tile: -1,
		aimSpecial: -1, news: newNewsView()}
}

func (s *systemScreen) Enter(a *app) {
	s.sys, s.g, s.e, s.player = a.cl.Stars[s.star].System, a.g, a.e, a.player
	s.gunTicks, s.sndTicks = a.gunTicks, a.sndTicks
	s.rebuild()
}

// rebuild lists the bodies afresh: seenBodies, then the ships (sighted,
// the moving one where its hop has got to) or a replay's cast. Pointers
// into the old slice (hover) go stale. Enter runs it: -shot never Updates.
func (s *systemScreen) rebuild() {
	s.bodies = seenBodies(s.e, s.star, s.sys)
	switch {
	case s.replay != nil:
		s.glidePlanets()
		s.bodies = append(s.bodies, s.replayBodies()...)
	case s.g != nil:
		var seen []game.Sighting
		if s.e != nil {
			seen = s.g.ShipsSeen(s.e, s.star)
		}
		// A ship in a lane is gone from the system once its hop's anim,
		// ending at the lane's sphere, is over.
		bs := sighted(shipBodies(s.g, s.star, s.anim.ship, s.animPos()), seen, s.player)
		s.bodies = append(s.bodies, slices.DeleteFunc(bs, func(b body) bool { return b.ship.Lane >= 0 && b.ship != s.anim.ship })...)
	}
	s.checkForeign()
	// A replay's cast omits lane ships: a selected one stays while the
	// model holds it here.
	kept := s.replay != nil && s.sel != nil && s.sel.Star == s.star && s.g.OwnerOf(s.sel) != nil
	inOrbit := s.sel != nil && s.sel != s.anim.ship && s.sel.Lane < 0 && s.sel.Orbit >= 0 // the planet screen's
	inLane := s.sel != nil && s.sel.Lane >= 0                                             // the day's close took it: no interaction in a lane
	if s.sel != nil && (inOrbit || inLane || !kept && !slices.ContainsFunc(s.bodies, func(b body) bool { return b.ship == s.sel })) {
		s.sel = nil // it left the system
		s.move = moveMode{}
	}
	s.clampLists()
}

// checkForeign drops fsel once one of ours or a planet is selected (back
// too), or once the player no longer sees it here (gone to a lane or
// another star, destroyed): back is then selected again (dropForeign).
func (s *systemScreen) checkForeign() {
	switch {
	case s.fsel == nil:
	case s.sel != nil || s.psel != nil:
		s.fsel, s.back = nil, nil
	default:
		if _, ok := s.foreignSeen(); !ok {
			s.dropForeign()
		}
	}
}

// foreignSeen is fsel as the player sees it at this star, false when not.
func (s *systemScreen) foreignSeen() (game.Sighting, bool) {
	if s.fsel == nil || s.g == nil || s.e == nil || s.fsel.Star != s.star {
		return game.Sighting{}, false
	}
	return s.g.SightingOf(s.e, s.fsel)
}

// inspectable reports b a foreign ship the player sees out of the lanes:
// a click on it outside the fire mode inspects it (inspect).
func (s *systemScreen) inspectable(b *body) bool {
	return b.kind == shipBody && b.owner != s.player && b.seen != nil && b.ship.Lane < 0
}

// inspect takes b's ship as fsel, our selected ship kept as back (a
// foreign ship inspected already keeps the first back); the selection,
// move and fire modes dropped.
func (s *systemScreen) inspect(b *body) {
	if s.fsel == nil {
		s.back = s.sel
	}
	s.fsel, s.sel, s.psel, s.placing = b.ship, nil, nil, false
	s.move, s.aim, s.aimSpecial = moveMode{}, false, -1
}

// dropForeign drops fsel and selects back again while it is still ours,
// here and out of a lane (no move preview: placing stays false).
func (s *systemScreen) dropForeign() {
	if b := s.back; b != nil && s.e != nil && slices.Contains(s.e.Ships, b) && b.Star == s.star && b.Lane < 0 {
		s.sel = b
	}
	s.fsel, s.back = nil, nil
}

// selectable reports b is the player's ship out of the lanes: a lane
// ship is drawn only while its hop's anim runs.
func (s *systemScreen) selectable(b *body) bool {
	return b.owner == s.player && b.ship.Lane < 0
}

// animPos is where the animated ship is drawn this tick.
func (s *systemScreen) animPos() game.Vec3 {
	m := s.anim
	f := min(1, float64(s.tick-m.t0)/float64(m.ticks()))
	if m.bounce {
		return bouncePos(m.from, m.touch, m.to, f)
	}
	return lerp(m.from, m.to, f)
}

// hop moves sh one Hop on its order (Barred bounces it off guarded
// orbits) and, when it moved, animates it from where it was drawn to where
// it is, replacing any other anim (whose model is already at its end).
func (s *systemScreen) hop(e *game.Empire, sh *game.Starship) bool {
	from := s.drawnAt(sh)
	var obs []game.Vec3 // ShipGap: nil without a game (view tests)
	if s.g != nil {
		obs = s.g.Obstacles(sh)
	}
	if err := e.BlockedLane(sh); err != nil { // the Arbryls' block: the hand hop waits too
		s.fail(err)
		return false
	}
	moved, _ := sh.Hop(e.Cluster, e.Day, obs, e.Barred)
	if moved {
		s.anim = anim{} // drawnAt must not read the old anim
		to := s.drawnAt(sh)
		s.anim = anim{ship: sh, from: from, to: to, t0: s.tick, day: e.Day}
		s.cues = append(s.cues, cue{name: thrustSound(sh.Design.Hull), t: s.tick})
		if touch, pl, ok := bounceAt(s.g, sh, sh.Pos); ok {
			s.anim.bounce, s.anim.touch = true, touch
			s.cues = append(s.cues, cue{name: bounceSound, t: s.tick + int(bounceIn*float64(s.anim.ticks()))})
			s.fail(errors.New(bounceLine(e, s.star, pl)))
		}
	}
	return moved
}

// advance is the anim's tick: dropped (and Auto, the pending hops) when
// a day closed since (laneWatch's flare then, not this one), ended after
// hopTicks, its endpoint lit when the hop entered a lane, then on Auto
// the next hop (autoNext), else the next pending ship's (hopPending).
func (s *systemScreen) advance(e *game.Empire) {
	m := s.anim
	switch {
	case m.ship == nil:
		s.auto, s.pending = false, nil // nothing runs: a run without an anim is over
	case m.day != e.Day:
		s.anim, s.auto, s.pending = anim{}, false, nil
	case s.tick-m.t0 >= m.ticks():
		s.anim = anim{}
		if m.ship.Lane >= 0 {
			s.startFlare(e.Cluster, m.ship.Lane, s.tick)
		}
		switch {
		case s.auto:
			s.auto = s.autoNext(e, m.ship)
		case len(s.pending) > 0:
			s.hopPending(e)
		}
	}
}

// movable reports sh one of the ships a run here hops: at s's star, out
// of a lane, ordered.
func (s *systemScreen) movable(sh *game.Starship) bool {
	return sh != nil && sh.Star == s.star && sh.Lane < 0 && sh.Order.Kind != game.NoOrder
}

// autoNext hops first (nil: none) if it can, else the first of e's ships
// here, ordered and out of a lane, that moves; it reports whether one did.
func (s *systemScreen) autoNext(e *game.Empire, first *game.Starship) bool {
	if s.movable(first) && s.hop(e, first) {
		return true
	}
	for _, sh := range e.Ships {
		if sh != first && s.movable(sh) && s.hop(e, sh) {
			return true
		}
	}
	return false
}

// startAuto is Auto (A): the run starts with the selected ship, else the
// first that moves; none: refused.
func (s *systemScreen) startAuto(e *game.Empire) error {
	s.pending = nil
	s.auto = s.autoNext(e, s.sel)
	if !s.auto {
		return errors.New("nothing to move")
	}
	return nil
}

// hopAll is the left button out of combat mode: every one of e's ships
// here that a run hops (movable) makes its next hop, once each, one anim
// after another (pending, as Auto's chain); none moves: refused.
func (s *systemScreen) hopAll(e *game.Empire) error {
	s.auto, s.pending = false, nil
	for _, sh := range e.Ships {
		if s.movable(sh) {
			s.pending = append(s.pending, sh)
		}
	}
	if !s.hopPending(e) {
		return errors.New("nothing to move")
	}
	return nil
}

// hopPending hops the first pending ship that moves, dropping it and
// those before it that did not; false when none moved (pending empty).
func (s *systemScreen) hopPending(e *game.Empire) bool {
	for len(s.pending) > 0 {
		sh := s.pending[0]
		s.pending = s.pending[1:]
		if s.movable(sh) && s.hop(e, sh) {
			return true
		}
	}
	return false
}

// repeat is Repeat Move (M): the selected ship, else the last one
// ordered while still here and ordered, hops once more on its order.
func (s *systemScreen) repeat(e *game.Empire) error {
	sh := s.sel
	if sh == nil && slices.Contains(e.Ships, s.last) && s.movable(s.last) {
		sh = s.last
	}
	switch {
	case sh == nil:
		return errors.New("no ship selected")
	case sh.Order.Kind != game.GoToPlanet && sh.Order.Kind != game.GoToStar && sh.Order.Kind != game.GoToPoint:
		return errors.New("no order to repeat")
	case sh.Lane >= 0:
		return errors.New("in a lane")
	case sh.Power < sh.Design.MoveCost():
		return errors.New("out of power")
	}
	s.hop(e, sh)
	return nil
}

// press acts on panel button b. The left one (End Turn) never closes the
// day: it skips a running replay, else plays the next round in combat
// mode (stepRound), else hops every ordered ship here once (hopAll, a
// hand move: refused where the rounds move), a refusal toasted. Repeat
// Move and Auto as M and A.
func (s *systemScreen) press(a *app, b int) {
	switch b {
	case endTurnButton:
		switch {
		case s.skipReplay(a):
		case s.inCombat(a):
			a.stepRound(s)
		case !s.handHops() && s.roundsDone(a):
			a.toastFor(noRoundLeft, true)
		case !s.handHops():
			a.toastFor(roundsMove, true)
		default:
			if err := s.hopAll(a.e); err != nil {
				a.toastFor(err.Error(), true)
			}
		}
	case repeatButton:
		s.handMove(a, s.repeat)
	case autoButton:
		s.handMove(a, s.startAuto)
	case planetsFilter, shipsFilter:
		s.showTab(b - planetsFilter)
	}
}

// placeAll projects every body but orbitHidden ones, back to front.
func (s *systemScreen) placeAll() []placed {
	out := make([]placed, 0, len(s.bodies))
	for i := range s.bodies {
		if orbitHidden(&s.bodies[i]) {
			continue
		}
		pt := s.cam.Project(s.bodies[i].pos)
		out = append(out, placed{&s.bodies[i], pt, s.bodies[i].radius() * pt.Scale})
	}
	slices.SortStableFunc(out, func(p, q placed) int { return -cmpFloat(p.pt.Depth, q.pt.Depth) })
	return out
}

func (s *systemScreen) Update(a *app) error {
	s.tick++
	s.dropFlares()
	s.advance(a.e)
	s.playCues(a.sfx)
	if s.replayTick(a) {
		return nil // the climb ran: this screen may be gone
	}
	if s.replay == nil {
		s.rings = nil
	}
	s.rebuild()
	s.hover = nil
	if s.sel == nil && s.psel == nil {
		s.aim = false
	}
	switch k, on := s.specialAim(); {
	case on && k < 0: // the aimed part is gone: the mode ends
		s.aim, s.aimSpecial = false, -1
	case !on: // no special aimed: none is left over for a later aim
		s.aimSpecial = -1
	}
	if keyJustPressed(ebiten.KeyX) {
		s.fail(s.order(a.e, game.Order{}))
	}
	if keyJustPressed(ebiten.KeyN) && s.panelOn() {
		s.fail(s.navigate(a))
		if a.top() != s {
			return nil
		}
	}
	if keyJustPressed(ebiten.KeyG) {
		s.grid = !s.grid
	}
	if keyJustPressed(ebiten.KeyE) {
		s.elev = !s.elev
	}
	if keyJustPressed(ebiten.KeyR) {
		s.cam = sysCamera()
	}
	if onSpeciesKey(a, keyJustPressed(ebiten.KeyS)) {
		return nil
	}
	focal := s.cam.Focal
	x, y, click := steer(a, &s.cam, &s.drag, sysArea)
	_, wy := ebiten.Wheel()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	wheeled, newsPress := false, false
	s.news.hover = -1
	if s.newsOn() {
		wheeled, newsPress = s.news.input(a, x, y, wy, press)
	}
	if wheeled {
		s.cam.Focal = focal // over the news it scrolls, not the camera
	} else {
		s.wheel(x, y, wy, focal) // over the lists: they scroll, not the camera
	}
	s.cam.Clamp()
	if sysArea.has(x, y) {
		s.hover = pickBody(s.placeAll(), x, y)
	}
	s.listHoverAt(x, y)
	s.tile, s.stopHot, s.navHot = -1, false, false
	switch {
	case s.panelOn():
		sh, _ := s.stopState(a.e)
		s.stopHot = sh && stopRect().has(x, y)
		s.navHot = sh && navRect().has(x, y) && a.e.CanNavigate(s.sel) == nil
		s.tile = tileAt(panelTiles(s.sel), x, y)
	case s.planetOn():
		s.tile = planetTileAt(s.ptiles(), x, y)
	case s.fsel != nil:
		s.tile = tileAt(s.foreignTiles(), x, y)
	}
	if keyJustPressed(ebiten.KeyF) {
		s.fail(s.fire(a))
	}
	if keyJustPressed(ebiten.KeyD) {
		s.fail(s.toggleShields())
	}
	if keyJustPressed(ebiten.KeyM) {
		s.handMove(a, s.repeat)
	}
	if keyJustPressed(ebiten.KeyA) {
		s.handMove(a, s.startAuto)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		s.rightClick()
	}
	s.moveTrack(x, y)
	s.ghostAt(x, y)
	switch {
	case click && s.skipReplay(a): // a click in the view only skips the replay
	case s.aim && click:
		s.fail(s.aimClick(a, s.hover))
	case click:
		s.click(a, x, y)
	}
	// steer never reports a click outside sysArea: the info box and the
	// panel act on the press, last (End Turn may climb off this screen),
	// unless the news took it.
	s.btn = sysButton(x, y)
	if press {
		switch {
		case newsPress:
		case s.panelOn() && sysInfo.has(x, y):
			s.fail(s.panelClick(a, x, y))
		case s.planetOn() && sysInfo.has(x, y):
			s.fail(s.planetClick(a, x, y))
		case s.fsel != nil && sysInfo.has(x, y): // read-only
		case s.listsOn() && sysInfo.has(x, y):
			s.aim = false
			s.listClick(a, x, y)
		case !sysArea.has(x, y):
			s.aim = false
			s.press(a, s.btn)
		}
	}
	return nil
}

// fire is F: the selected ship arms ReadyWeapon's part at the hovered
// ship; that part armed already this round, the next weapon Arm accepts
// (Weapon order: so each F arms one more), else ReadyWeapon's refusal.
func (s *systemScreen) fire(a *app) error {
	if s.sel == nil {
		return errors.New("no ship selected")
	}
	if s.hover == nil || s.hover.kind != shipBody {
		return errors.New("hover the ship to fire at")
	}
	t := s.hover.ship
	part, err := s.sel.ReadyWeapon(a.cl, t)
	if err != nil {
		return err
	}
	first := s.shoot(a, part, t)
	if first == nil || !s.sel.ArmedPart(part) {
		return first
	}
	for p, g := range s.sel.Design.Parts {
		if p != part && game.Gizmos[g].Cat == game.Weapon && !s.sel.ArmedPart(p) && s.sel.CanFire(a.cl, p, t) == nil && s.shoot(a, p, t) == nil {
			return nil
		}
	}
	return first
}

// shoot arms the selected ship's part at t: it fires in the next round
// played at this star (Next round, or the day's close).
func (s *systemScreen) shoot(a *app, part int, t *game.Starship) error {
	if a.g == nil || a.e == nil {
		return errors.New("no game")
	}
	sg, err := hostileSighting(a, t)
	if err != nil {
		return err
	}
	return a.e.Arm(s.sel, part, sg)
}

// hostileSighting is the player's sighting of t, or why t is no target.
func hostileSighting(a *app, t *game.Starship) (game.Sighting, error) {
	sg, ok := a.g.SightingOf(a.e, t)
	switch {
	case !ok && a.g.OwnerOf(t) == nil:
		return sg, errors.New("no such target")
	case !ok:
		return sg, errors.New("not hostile")
	}
	return sg, nil
}

// aimPart is the part the fire mode shoots at t: the first holding the
// chosen weapon's gizmo not armed this round that CanFire, else
// sel.Weapon (its refusal tells).
func (s *systemScreen) aimPart(c *game.Cluster, t *game.Starship) int {
	sh := s.sel
	if sh.Weapon < 0 || sh.Weapon >= len(sh.Design.Parts) {
		return sh.Weapon
	}
	g := sh.Design.Parts[sh.Weapon]
	for k, p := range sh.Design.Parts {
		if p == g && !sh.ArmedPart(k) && sh.CanFire(c, k, t) == nil {
			return k
		}
	}
	return sh.Weapon
}

// aimClick is a left click on b (nil: nothing) in the fire mode, which
// ends: a special is used at b, the planet's Tractor Beam pulls b, a
// hostile ship is fired at (by the planet or the ship), a planet is the
// selected planet's target (planetShootPlanet) or a ship's (shootPlanet).
func (s *systemScreen) aimClick(a *app, b *body) error {
	k, on := s.specialAim()
	s.aim, s.aimSpecial, s.placing = false, -1, false
	switch {
	case on && k < 0:
		return nil
	case on:
		return s.useAt(a, k, b)
	case s.psel != nil && s.ptract:
		return s.planetTractor(a, b)
	case s.psel != nil && s.hostile(b):
		return s.planetShoot(a, b.ship)
	case s.psel != nil && b != nil && b.kind == planetBody:
		return s.planetShootPlanet(a, b.idx)
	case s.sel == nil:
		return nil
	case b != nil && b.kind == planetBody:
		return s.shootPlanet(a, b.idx)
	case !s.hostile(b):
		return nil
	}
	return s.shoot(a, s.aimPart(a.cl, b.ship), b.ship)
}

// specialAim is the special part aimed (k −1: stale, the part no longer
// holds aimG) and whether a special's aim is on at all.
func (s *systemScreen) specialAim() (k int, on bool) {
	if !s.aim || s.sel == nil || s.psel != nil || s.aimSpecial < 0 {
		return -1, false
	}
	if k := s.aimSpecial; k < len(s.sel.Design.Parts) && s.sel.Design.Parts[k] == s.aimG {
		return k, true
	}
	return -1, true
}

// aimsSpecial is whether a special's aim is on.
func (s *systemScreen) aimsSpecial() bool {
	_, on := s.specialAim()
	return on
}

// vecDist is the distance from p to q, as the model measures range.
func vecDist(p, q game.Vec3) float64 {
	return math.Sqrt((p.X-q.X)*(p.X-q.X) + (p.Y-q.Y)*(p.Y-q.Y) + (p.Z-q.Z)*(p.Z-q.Z))
}

// specialAimKind is what g is aimed at, read off its flags (the model's
// table is unexported): a ship first (any, allied or hostile), else a
// lane opening, else a planet, else nothing (used at once).
func specialAimKind(g game.Gizmo) specAim {
	f := game.Gizmos[g].Flags
	switch {
	case f&game.GzTargetShip != 0 && slices.Contains(anyAimed, g):
		return aimAnyShip
	case f&game.GzTargetShip != 0 && slices.Contains(allyAimed, g):
		return aimAllyShip
	case f&game.GzTargetShip != 0: // the Carbonizer's lane and planet flags: not modelled
		return aimHostileShip
	case f&game.GzTargetLane != 0:
		return aimLaneOpening
	case f&game.GzTargetSurface != 0:
		return aimPlanetBody
	}
	return aimNone
}

// specAim is a special's aim kind.
type specAim uint8

const (
	aimNone specAim = iota
	aimHostileShip
	aimAllyShip
	aimPlanetBody
	aimAnyShip
	aimLaneOpening
)

// anyAimed take any ship but the user's (the model's aimAny).
var anyAimed = []game.Gizmo{game.GizmoTractorBeam, game.Hyperswapper}

// allyAimed take an own or allied ship (the model's aimAlly).
var allyAimed = []game.Gizmo{game.SacrificialOrb, game.PlasmaCoupler}

// useAt uses the selected ship's special part k at b: a ship (sighted,
// or Mine), a planet or a lane opening. Only b's kind is read: a wrong
// target is UseSpecial's refusal.
func (s *systemScreen) useAt(a *app, k int, b *body) error {
	if a.g == nil || a.e == nil || b == nil {
		return nil
	}
	var t game.Target
	switch {
	case b.kind == shipBody && b.owner == s.player:
		t = game.Target{Kind: game.ShipTarget, Mine: b.ship}
	case b.kind == shipBody:
		sg, ok := a.g.SightingOf(a.e, b.ship)
		if !ok {
			return errors.New("no such target")
		}
		t = game.Target{Kind: game.ShipTarget, Ship: sg}
	case b.kind == planetBody:
		t = game.Target{Kind: game.PlanetTarget, Index: b.idx}
	case b.kind == laneBody:
		i := s.laneIndex(b)
		if i < 0 {
			return errors.New("no such target")
		}
		t = game.Target{Kind: game.LaneTarget, Index: i}
	default:
		return nil
	}
	reps, err := a.e.UseSpecial(s.sel, k, t)
	if err != nil {
		return err
	}
	s.afterUse(a, reps)
	return nil
}

// laneIndex is b's lane's index in System.Lanes (LaneTarget's Index, not
// the cluster lane's), -1 none.
func (s *systemScreen) laneIndex(b *body) int {
	for i := range s.sys.Lanes {
		if &s.sys.Lanes[i] == b.lane {
			return i
		}
	}
	return -1
}

// aimCandidate is whether b is what the aimed special g takes.
func (s *systemScreen) aimCandidate(g game.Gizmo, b *body) bool {
	if b == nil {
		return false
	}
	switch specialAimKind(g) {
	case aimHostileShip:
		return s.hostile(b)
	case aimAllyShip:
		return b.kind == shipBody && (b.owner == s.player && b.ship != s.sel ||
			b.owner != s.player && s.g != nil && s.g.Empires[s.player].StanceWith(b.owner) == game.Alliance)
	case aimAnyShip:
		return b.kind == shipBody && b.ship != s.sel
	case aimLaneOpening:
		return b.kind == laneBody
	case aimPlanetBody:
		return b.kind == planetBody
	}
	return false
}

// hostile is whether b is a ship of an empire Hostile to the player's.
func (s *systemScreen) hostile(b *body) bool {
	return b != nil && b.kind == shipBody && s.g != nil && s.g.Hostile(s.g.Empires[s.player], s.g.Empires[b.owner])
}

// aimTarget is the fire mode's line end (nil none) and whether the shot
// would go: a ship's weapon by CanFire, a ship's or a planet's at a planet
// by planetAimLabel; a planet's at a ship while it has a shot left (no
// game helper tells its range); the
// planet's Tractor Beam by CanTractor on any ship.
func (s *systemScreen) aimTarget() (*body, bool) {
	if k, on := s.specialAim(); on {
		// Live within Range × RangeUnit of the user (never Boosted); the
		// other refusals show on the click.
		if k < 0 || !s.aimCandidate(s.aimG, s.hover) {
			return nil, false
		}
		at := s.hover.pos
		switch s.hover.kind {
		case shipBody:
			at = s.hover.ship.Pos
		case planetBody:
			at = s.hover.planet.Pos
		}
		r := float64(game.Gizmos[s.aimG].Range) * game.RangeUnit
		return s.hover, vecDist(s.sel.Pos, at) <= r+1e-9
	}
	if b, _, live, ok := s.planetAimLabel(); ok {
		return b, live
	}
	if s.aim && s.psel != nil && s.ptract {
		col := s.pselColony()
		if s.hover == nil || s.hover.kind != shipBody || col == nil || s.e == nil || s.g == nil {
			return nil, false
		}
		t, err := tractorTarget(s.g, s.e, s.hover)
		return s.hover, err == nil && s.e.CanTractor(col, t) == nil
	}
	if !s.aim || s.sel == nil && s.psel == nil || !s.hostile(s.hover) {
		return nil, false
	}
	t := s.hover.ship
	if s.psel != nil {
		col := s.pselColony()
		return s.hover, col != nil && shotsLeft(col, s.pslot) > 0
	}
	return s.hover, s.sel.CanFire(s.g.Cluster, s.aimPart(s.g.Cluster, t), t) == nil
}

// aimFrom is the fire mode line's start: the selected ship's or planet's
// body, nil when not drawn.
func (s *systemScreen) aimFrom() *body {
	for i := range s.bodies {
		if b := &s.bodies[i]; b.kind == shipBody && s.sel != nil && b.ship == s.sel || b.kind == planetBody && s.psel != nil && b.planet == s.psel {
			return b
		}
	}
	return nil
}

// drawArmed dashes a line from each of the player's armed shots here,
// ships' and colonies', to its target (the Sighting's Pos: nothing moves
// before the round), or the planet shot at; Round clears Armed.
func (s *systemScreen) drawArmed(sc *ebiten.Image) {
	planets := s.sys.Planets
	line := func(from game.Vec3, sh game.Shot) {
		to := sh.Target.Pos
		if sh.AtPlanet {
			if sh.Planet < 0 || sh.Planet >= len(planets) {
				return
			}
			to = planets[sh.Planet].Pos
		}
		p, q := s.cam.Project(from), s.cam.Project(to)
		dashLine(sc, p.X, p.Y, q.X, q.Y, armedColour)
	}
	for i := range s.bodies {
		b := &s.bodies[i]
		switch {
		case b.kind == shipBody && b.owner == s.player && !orbitHidden(b):
			for _, sh := range b.ship.Armed {
				line(b.pos, sh)
			}
		case b.kind == planetBody && s.e != nil:
			if c := s.e.Colony(s.star, b.idx); c != nil {
				for _, sh := range c.Armed {
					line(b.pos, sh)
				}
			}
		}
	}
}

// dashLine strokes (x0,y0)-(x1,y1) in dashes of 4 zoomed px, 3 apart.
func dashLine(sc *ebiten.Image, x0, y0, x1, y1 float64, c color.RGBA) {
	const dash, gap = 4 * zoom, 3 * zoom
	d := math.Hypot(x1-x0, y1-y0)
	if d == 0 {
		return
	}
	ux, uy := (x1-x0)/d, (y1-y0)/d
	for t := 0.0; t < d; t += dash + gap {
		e := min(t+dash, d)
		vector.StrokeLine(sc, float32(x0+ux*t), float32(y0+uy*t), float32(x0+ux*e), float32(y0+uy*e), zoom, c, true)
	}
}

// toggleShields is D: on reserves the shields' draw off the energy at
// once, refused when short; off refunds it (Empire.SetShields).
func (s *systemScreen) toggleShields() error {
	if s.sel == nil {
		return errors.New("no ship selected")
	}
	return s.e.SetShields(s.sel, !s.sel.Shields)
}

// rightClick cancels one layer: the height step, else the aim, else the
// inspected foreign ship (back to ours, dropForeign), else the selected
// ship, else the selected planet. It never leaves the screen.
func (s *systemScreen) rightClick() {
	switch {
	case s.move.on:
		s.move = moveMode{}
	case s.aim:
		s.aim, s.aimSpecial = false, -1
	case s.fsel != nil:
		s.dropForeign()
	case s.sel != nil:
		s.sel = nil
	default:
		s.psel = nil
	}
}

// plant enters the height step at footAt.
func (s *systemScreen) plant(x, y float64) {
	if m, ok := s.footAt(x, y); ok {
		s.move = m
	}
}

// footAt is the height step the first click at (x, y) would enter: the
// foot on the plane at the ship's height under the cursor, pulled within
// moveReach.
func (s *systemScreen) footAt(x, y float64) (moveMode, bool) {
	y0 := s.sel.Pos.Y
	lim := moveReach*moveReach - y0*y0
	pt, ok := s.cam.UnprojectPlane(x, y, y0)
	if !ok || lim <= 0 {
		return moveMode{}, false
	}
	if r := math.Hypot(pt.X, pt.Z); r > math.Sqrt(lim) {
		k := math.Sqrt(lim) / r
		pt.X, pt.Z = pt.X*k, pt.Z*k
	}
	return moveMode{on: true, foot: game.Vec3{X: pt.X, Z: pt.Z}, pt: game.Vec3{X: pt.X, Y: y0, Z: pt.Z}, ok: true}, true
}

// ghostAt is the preview of the first click at (x, y), a ship just
// selected: footAt where a click would plant (in
// the view, no body under the cursor, off the lists, out of the fire
// mode and the height step); off elsewhere, a body hovered included, and
// once the first click is spent (placing).
func (s *systemScreen) ghostAt(x, y float64) {
	s.ghost = moveMode{}
	if s.sel == nil || !s.placing || s.move.on || s.aim || s.listHover || s.hover != nil || !sysArea.has(x, y) {
		return
	}
	s.ghost, _ = s.footAt(x, y)
}

// moveTrack is a tick of the height step: the cross at the cursor's y on
// the foot's vertical, clamped; the step ends with the ship gone.
func (s *systemScreen) moveTrack(x, y float64) {
	m := &s.move
	if !m.on {
		return
	}
	if s.sel == nil {
		*m = moveMode{}
		return
	}
	var yy float64
	yy, m.ok = s.cam.UnprojectVertical(y, m.foot.X, m.foot.Z)
	if !m.ok {
		return
	}
	lim := math.Sqrt(max(0, moveReach*moveReach-m.foot.X*m.foot.X-m.foot.Z*m.foot.Z))
	m.pt = game.Vec3{X: m.foot.X, Y: min(max(yy, -lim), lim), Z: m.foot.Z}
}

// moveTarget is the hovered planet or lane endpoint a click in the view
// would send the selected ship to, out of the fire and height modes and
// off the lists' rows; nil none.
func (s *systemScreen) moveTarget() *body {
	b := s.hover
	if s.sel == nil || s.aim || s.move.on || s.listHover || b == nil {
		return nil
	}
	if b.kind == planetBody || b.kind == laneBody && b.lane.To >= 0 {
		return b
	}
	return nil
}

// targetText labels a click on b: targetLabel, then the ETA of a
// copy of the ship ordered there, or Check's refusal.
func (s *systemScreen) targetText(e *game.Empire, b *body) string {
	t := targetLabel(e, b)
	if err := s.sel.Check(e.Cluster, targetOrder(b)); err != nil {
		return t + ", refused: " + err.Error()
	}
	tmp := *s.sel
	tmp.Order = targetOrder(b)
	if days, ok := tmp.ETA(e.Cluster, e.Day); ok {
		t += ", " + etaText(days)
	}
	return t
}

// targetOrder is the order a click on b gives.
func targetOrder(b *body) game.Order {
	if b.kind == planetBody {
		return game.Order{Kind: game.GoToPlanet, Planet: b.idx}
	}
	return game.Order{Kind: game.GoToStar, Star: b.lane.To}
}

// targetLabel names what a click on b does.
func targetLabel(e *game.Empire, b *body) string {
	if b.kind == planetBody {
		return "orbit " + b.planet.Name
	}
	return "lane to " + e.StarLabel(b.lane.To)
}

// cancel is Esc before the screen's pop: it ends the move and fire
// modes, else drops the inspected foreign ship (back to ours), else the
// selection; it reports whether it did any.
func (s *systemScreen) cancel() bool {
	on := s.move.on || s.aim
	s.move, s.aim, s.aimSpecial = moveMode{}, false, -1
	if !on && s.fsel != nil {
		s.dropForeign()
		return true
	}
	if !on && (s.psel != nil || s.sel != nil) {
		s.psel, s.sel, on = nil, nil, true
	}
	return on
}

// moveText is the info box in the height step: the help, the cross's
// point, the ETA of a copy of the ship ordered there.
func (s *systemScreen) moveText(e *game.Empire) string {
	m := s.move
	t := moveHelp + fmt.Sprintf("\nx %.2f z %.2f y %.2f", m.pt.X, m.pt.Z, m.pt.Y)
	if m.ok && s.sel != nil {
		tmp := *s.sel
		tmp.Order = game.Order{Kind: game.GoToPoint, Pos: m.pt}
		if days, ok := tmp.ETA(e.Cluster, e.Day); ok {
			t += "\n" + etaText(days)
		}
	}
	return t
}

// orbitColony is e's colony sh orbits, nil none.
func orbitColony(e *game.Empire, sh *game.Starship) *game.Colony {
	if sh.Lane >= 0 || sh.Orbit < 0 {
		return nil
	}
	for _, c := range e.Colonies {
		if c.Star == sh.Star && c.PlanetIdx == sh.Orbit {
			return c
		}
	}
	return nil
}

// refitFoot is the ship dialog Refit's hover at e's Docks: the
// template's refit (RefitTarget, CanRefit accepting it) with its cost
// and days at today's industry (at least 1: it completes at a close),
// else the hand refit's pointer; "" away from docks or while refitted
// (atDocks).
func refitFoot(e *game.Empire, sh *game.Starship) string {
	c := atDocks(e, sh)
	if c == nil {
		return ""
	}
	d, err := e.RefitTarget(sh)
	if err == nil {
		err = e.CanRefit(c, sh, d)
	}
	if err != nil {
		return fmt.Sprintf("refit: edit the parts at %s's Orbital Docks", c.Planet.Name)
	}
	cost := game.RefitCost(sh, d)
	ind, _, _ := c.Output()
	if ind <= 0 {
		return fmt.Sprintf("refit to %s: +%d industry, no industry", sh.Design.Name, cost)
	}
	return fmt.Sprintf("refit to %s: +%d industry, %s", sh.Design.Name, cost, dayCount(max(1, (cost+ind-1)/ind)))
}

// dayCount is "1 day" or "N days".
func dayCount(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// fail shows err in the info box for two seconds (60 ticks a second).
func (s *systemScreen) fail(err error) {
	if err != nil {
		s.msg, s.msgUntil, s.msgOK = err.Error(), s.tick+120, false
	}
}

// note shows a success report where fail shows a refusal.
func (s *systemScreen) note(text string) {
	s.msg, s.msgUntil, s.msgOK = text, s.tick+120, true
}

// msgColour is the message's colour: a refusal's red, a report's text.
func (s *systemScreen) msgColour() color.RGBA {
	if s.msgOK {
		return textColour
	}
	return errColour
}

// order gives the selected ship o: accepted, it ends Auto and hops once
// where handHops, a destination then dropping the ship (in combat it
// stays selected for the rounds); the cancel (X, Stop) disarms its
// shots too.
func (s *systemScreen) order(e *game.Empire, o game.Order) error {
	if s.sel == nil {
		return nil
	}
	if err := e.Order(s.sel, o); err != nil {
		return err
	}
	s.auto, s.pending = false, nil
	if o.Kind == game.NoOrder {
		e.Disarm(s.sel)
	}
	if s.handHops() {
		s.hop(e, s.sel)
		if o.Kind != game.NoOrder {
			s.sel, s.last = nil, s.sel
		}
	}
	return nil
}

// handHops reports the screen hopping ships by hand (order, Repeat Move,
// Auto, hopAll): not where the player is engaged (game.Engaged, combat
// mode or its rounds used up): the rounds move them there, a hop each,
// as the bots'.
func (s *systemScreen) handHops() bool {
	return s.g == nil || s.e == nil || !s.g.Engaged(s.e, s.star)
}

// roundsMove is the refusal of a hand move where the rounds move;
// noRoundLeft the left button's there once RoundCap rounds are played.
const (
	roundsMove  = "moves play in the rounds"
	noRoundLeft = "no round left here today"
)

// handMove is Repeat Move or Auto (f), refused with a toast where the
// rounds move the ships.
func (s *systemScreen) handMove(a *app, f func(*game.Empire) error) {
	if !s.handHops() {
		a.toastFor(roundsMove, true)
		return
	}
	s.fail(f(a.e))
}

// click is a left click at (x, y) in the view. In the height step it
// orders GoToPoint at the cross, whatever is under the cursor (nothing
// while the cross is unusable); a ship selected, nothing under it plants
// the step's foot; else clickBody on the body there.
func (s *systemScreen) click(a *app, x, y float64) {
	if !sysArea.has(x, y) {
		return
	}
	s.placing = false // the first click spent; a selection sets it again
	b := pickBody(s.placeAll(), x, y)
	switch {
	case s.move.on:
		if !s.move.ok {
			return
		}
		s.fail(s.order(a.e, game.Order{Kind: game.GoToPoint, Pos: s.move.pt}))
		s.move = moveMode{}
	case s.sel != nil && b == nil:
		s.plant(x, y)
	default:
		s.clickBody(a, b)
	}
}

// clickBody is a click in the view on b (nil: nothing): act, then the
// screen it asks for pushed.
func (s *systemScreen) clickBody(a *app, b *body) {
	push, err := s.act(a.e, b)
	s.pushed(a, push, err)
}

// clickList is a click on a list row's body: actList.
func (s *systemScreen) clickList(a *app, b *body) {
	push, err := s.actList(a.e, b)
	s.pushed(a, push, err)
}

// pushed reports err and pushes the screen asked for, if any.
func (s *systemScreen) pushed(a *app, push screen, err error) {
	s.fail(err)
	if push != nil {
		s.move = moveMode{}
		a.push(push)
	}
}

// act is a click in the view on b (nil: nothing): with a ship selected
// a planet or lane endpoint orders it there (targetOrder) and the step
// ends; else actList.
func (s *systemScreen) act(e *game.Empire, b *body) (push screen, err error) {
	if s.sel != nil && b != nil && (b.kind == planetBody || b.kind == laneBody && b.lane.To >= 0) {
		s.move = moveMode{}
		return nil, s.order(e, targetOrder(b))
	}
	return s.actList(e, b)
}

// actList is a click on a list row's body: it never orders, it inspects.
// A ship is selected (a foreign one seen out of the lanes inspected:
// inspect), a planet selected or opened when already (the ship dropped),
// nothing drops the planet, a lane endpoint opens the far star's system
// when known and no ship is selected.
func (s *systemScreen) actList(e *game.Empire, b *body) (push screen, err error) {
	switch {
	case b != nil && b.kind == shipBody:
		switch {
		case s.selectable(b):
			s.sel, s.psel, s.fsel, s.back, s.move, s.placing = b.ship, nil, nil, nil, moveMode{}, true
		case s.inspectable(b):
			s.inspect(b)
		}
	case b != nil && b.kind == planetBody:
		if s.psel == b.planet {
			return newPlanetScreen(s.star, b.idx), nil
		}
		s.selectPlanet(b)
	case b == nil && s.fsel != nil: // as the right click (a left-click way out)
		s.dropForeign()
	case b == nil:
		if s.sel == nil {
			s.psel = nil
		}
	case s.sel == nil && b.kind == laneBody && b.lane.To >= 0 && knows(e, b.lane.To):
		return newSystemScreen(b.lane.To), nil
	}
	return nil, nil
}

// Render paints the grid, the bodies back to front with their elevation
// lines and rings, the mode marks, then the chrome (trimming the spill),
// the top-right panel and the info box.
func (s *systemScreen) Render(a *app, sc *ebiten.Image) {
	sc.Fill(color.Black)
	if s.grid {
		for _, l := range gridLines() {
			p, q := s.cam.Project(l[0]), s.cam.Project(l[1])
			vector.StrokeLine(sc, float32(p.X), float32(p.Y), float32(q.X), float32(q.Y), zoom, gridColour, true)
		}
	}
	newsAt := s.newsBody(a) // ringed as the hover
	// Each body's elevation line just before its sprite: a nearer line
	// crosses a farther sprite, never the reverse. The star stays full
	// (the light source), the rest dims with depth; the replay's effects
	// sort in (drawList).
	for _, it := range s.drawList() {
		if it.fx != nil {
			s.drawEffect(a, sc, *it.fx)
			continue
		}
		p := *it.body
		f := 1.0
		if p.kind != starBody {
			f = fade(p.pt.Depth, s.cam.Dist)
		}
		if s.elev && !p.orbiting {
			foot := s.cam.Project(game.Vec3{X: p.pos.X, Z: p.pos.Z})
			vector.StrokeLine(sc, float32(foot.X), float32(foot.Y), float32(p.pt.X), float32(p.pt.Y), zoom, dim(elevColour, f), true)
			footMark(sc, foot, dim(footColour, f))
		}
		sp := a.spr(p.sprite())
		if sp.Image == nil {
			continue
		}
		// The drawn box spans the body's diameter (p.r is native px: no
		// zoom), scaled about the frame's origin, the sphere's centre.
		k := p.r / (float64(sp.Frame.Box.Dx()) / 2)
		if p.kind == laneBody {
			sp = s.laneSprite(a, p.body, sp) // a flare at the sphere's scale: its frames grow
		}
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(p.pt.X, p.pt.Y)
		op.Filter = ebiten.FilterLinear
		op.ColorScale.Scale(float32(f), float32(f), float32(f), 1)
		sp.Draw(sc, &op)
		ring := float32(p.r + 3*zoom)
		if c, ok := s.ringOf(p.body); ok {
			vector.StrokeCircle(sc, float32(p.pt.X), float32(p.pt.Y), ring, zoom, c, true)
			ring += 3 * zoom
		}
		if p.kind == shipBody && p.ship == s.sel {
			vector.StrokeCircle(sc, float32(p.pt.X), float32(p.pt.Y), ring, zoom, yellow, true)
			ring += 3 * zoom
		}
		if p.kind == planetBody && p.planet == s.psel { // the original's white frame
			vector.StrokeRect(sc, float32(p.pt.X)-ring, float32(p.pt.Y)-ring, 2*ring, 2*ring, zoom, color.White, true)
			ring += 3 * zoom
		}
		if p.body == s.hover || p.body == newsAt {
			vector.StrokeCircle(sc, float32(p.pt.X), float32(p.pt.Y), ring, zoom, hoverColor, true)
		}
	}
	// A green ring on each of the player's point orders' destinations.
	for i := range s.bodies {
		if b := &s.bodies[i]; b.kind == shipBody && b.owner == s.player && b.ship.Order.Kind == game.GoToPoint {
			q := s.cam.Project(b.ship.Order.Pos)
			vector.StrokeCircle(sc, float32(q.X), float32(q.Y), 6*zoom, zoom, hoverColor, true)
		}
	}
	if b := s.moveTarget(); b != nil {
		// Two yellow rings on the click's target; Text labels it.
		q := s.cam.Project(b.pos)
		r := float32(b.radius() * q.Scale)
		vector.StrokeCircle(sc, float32(q.X), float32(q.Y), r+3*zoom, zoom, yellow, true)
		vector.StrokeCircle(sc, float32(q.X), float32(q.Y), r+6*zoom, zoom, yellow, true)
	}
	m := s.move
	if !m.on {
		m = s.ghost
	}
	if m.on && m.ok {
		foot, q := s.cam.Project(m.foot), s.cam.Project(m.pt)
		vector.StrokeLine(sc, float32(foot.X), float32(foot.Y), float32(q.X), float32(q.Y), zoom, footColour, true)
		footMark(sc, foot, footColour)
		const h = 5 * zoom // an 11 original px X
		vector.StrokeLine(sc, float32(q.X-h), float32(q.Y-h), float32(q.X+h), float32(q.Y+h), zoom, footColour, true)
		vector.StrokeLine(sc, float32(q.X-h), float32(q.Y+h), float32(q.X+h), float32(q.Y-h), zoom, footColour, true)
	}
	s.drawArmed(sc)
	if t, live := s.aimTarget(); t != nil {
		// The fire mode's line: white when the shot would go, else grey.
		c := color.RGBA{110, 110, 110, 255}
		if live {
			c = footColour
		}
		if b := s.aimFrom(); b != nil {
			p, q := s.cam.Project(b.pos), s.cam.Project(t.pos)
			vector.StrokeLine(sc, float32(p.X), float32(p.Y), float32(q.X), float32(q.Y), zoom, c, true)
		}
	}

	fillRect(sc, sysTrim)
	drawSprite(sc, a.spr("16battle.tmp", 0), 0, 0)
	drawSysPanel(a, sc, s.btn)
	s.drawStrip(a, sc)
	s.drawFilters(a, sc)
	fillRect(sc, sysInfo)
	if s.newsOn() { // its soft border covers the rim's place
		drawNewsBox(sc, newsBox)
	} else {
		drawInfoRim(sc)
	}
	switch s.box() {
	case boxPanel:
		s.drawPanel(a, sc)
	case boxPlanet:
		s.drawPlanetPanel(a, sc)
	case boxForeign:
		s.drawForeign(a, sc)
	case boxLists:
		s.drawLists(a, sc)
	}
}

// drawIcon draws a small icon at its pixel size (never scaled), its
// frame's box top-left at (x,y), dimmed by f.
func drawIcon(dst *ebiten.Image, ic *assets.Sprite, x, y, f float64) {
	if ic.Frame == nil {
		return
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(x-float64(ic.Frame.Box.Min.X-ic.Frame.Center.X), y-float64(ic.Frame.Box.Min.Y-ic.Frame.Center.Y))
	op.ColorScale.Scale(float32(f), float32(f), float32(f), 1)
	ic.Draw(dst, &op)
}

// drawInfoRim paints the extension's rim: half-alpha black, one original
// px wide, down both sides from the chrome's box bottom and along the
// bottom.
func drawInfoRim(sc *ebiten.Image) {
	rim := color.RGBA{0, 0, 0, 128}
	y0, y1 := float32(sysInfoBox.Y1), float32(sysInfo.Y1)
	vector.FillRect(sc, float32(sysInfo.X0)-zoom, y0, zoom, y1-y0+zoom, rim, false)
	vector.FillRect(sc, float32(sysInfo.X1), y0, zoom, y1-y0+zoom, rim, false)
	vector.FillRect(sc, float32(sysInfo.X0), y1, float32(sysInfo.X1-sysInfo.X0), zoom, rim, false)
}

// snapRings keeps every planet's guard ring before the model plays the
// rounds to replay (bot.Begin, bot.Step); a replay already running keeps
// its own, taken before its first round.
func (s *systemScreen) snapRings() {
	if s.replay != nil && s.rings != nil {
		return
	}
	s.rings = map[int]color.RGBA{}
	for i := range s.bodies {
		if b := &s.bodies[i]; b.kind == planetBody {
			if c, ok := guardRing(s.g, s.e, s.star, b); ok {
				s.rings[b.idx] = c
			}
		}
	}
}

// ringOf is planet body b's guard ring: the snapped one while a replay
// runs, else the model's.
func (s *systemScreen) ringOf(b *body) (color.RGBA, bool) {
	if s.replay != nil && s.rings != nil {
		c, ok := s.rings[b.idx]
		return c, ok && b.kind == planetBody
	}
	return guardRing(s.g, s.e, s.star, b)
}

// guardRing is the orbit marker on planet body b (battle.txt: "a colored
// circle surrounding the planet"), the view's one sign of its orbit: the
// guard's colour (Game.Guard), else the first orbiting ship's; none with
// an empty orbit or one e does not see (SeesOrbit: a foreign cloaker).
func guardRing(g *game.Game, e *game.Empire, star int, b *body) (color.RGBA, bool) {
	if g == nil || b.kind != planetBody || !e.SeesOrbit(b.planet) {
		return color.RGBA{}, false
	}
	if i, ok := g.Guard(star, b.idx); ok {
		return empireColour(colourOf(g, i)), true
	}
	for _, e := range g.Empires {
		for _, sh := range e.Ships {
			if sh.Star == star && sh.Lane < 0 && sh.Orbit == b.idx {
				return empireColour(colourOf(g, e.Index)), true
			}
		}
	}
	return color.RGBA{}, false
}

// footMark is an elevation line's foot: a 3x3 original px square on p.
func footMark(sc *ebiten.Image, p cam.Point, c color.RGBA) {
	vector.FillRect(sc, float32(math.Round(p.X)-3*zoom/2), float32(math.Round(p.Y)-3*zoom/2), 3*zoom, 3*zoom, c, false)
}

// lighten moves c f of the way to white.
func lighten(c color.RGBA, f float64) color.RGBA {
	up := func(v uint8) uint8 { return v + uint8(float64(255-v)*f) }
	return color.RGBA{up(c.R), up(c.G), up(c.B), c.A}
}

// dim scales a colour's channels by f, alpha kept.
func dim(c color.RGBA, f float64) color.RGBA {
	return color.RGBA{uint8(float64(c.R) * f), uint8(float64(c.G) * f), uint8(float64(c.B) * f), c.A}
}

// Text writes the title strip, the view's labels and the info box: the
// ship's, planet's or inspected foreign ship's panel, else the lists or
// the move mode's text, and with nothing selected the news (newsOn)
// under the F1 hint.
func (s *systemScreen) Text(a *app, frame *ebiten.Image) {
	at := func(r rect) (float64, float64) { return float64(r.X0 + 6*zoom), float64(r.Y0 + 6*zoom) }
	_, y := at(sysStrip)
	// The title right of the star's picture, cut to the strip.
	tx, tw := stripTextX(), float64(sysStrip.X1-6*zoom)-stripTextX()
	for i, l := range strings.Split(s.title(a.e), "\n") {
		ui.Print(frame, fitWidth(l, tw, bigFace), bigFace, tx, y+float64(i)*bigFace.Size*1.25, textColour)
	}
	s.filtersText(frame)
	s.floatsText(a, frame)
	s.logText(a, frame)
	if s.inCombat(a) {
		s.combatText(a, frame)
	}
	if b, label, live, ok := s.planetAimLabel(); ok && !s.aimsSpecial() {
		// A ship's fire mode on a planet: planetAimLabel's verdict under it.
		c := yellow
		if !live {
			c = errColour
		}
		q := s.cam.Project(b.pos)
		ui.Print(frame, label, smallFace, q.X-ui.Width(label, smallFace)/2, q.Y+b.radius()*q.Scale+8*zoom, c)
	}
	// The hover line: one white line at the view's bottom, as the
	// starmap's hovered star.
	if l := s.hoverLine(a.e); l != "" {
		ui.Print(frame, l, bigFace, float64(sysArea.X0+padX),
			float64(sysArea.Y1-padY)-bigFace.Size*1.25, color.RGBA{255, 255, 255, 255})
	}
	if s.panelOn() {
		s.panelText(a, frame)
		return
	}
	if s.planetOn() {
		s.planetText(a.e, frame)
		return
	}
	if s.fsel != nil {
		s.foreignText(a, frame)
		return
	}
	x, y := at(sysInfo)
	w := float64(sysInfo.X1 - sysInfo.X0 - 12*zoom)
	if s.newsOn() {
		s.news.text(a, frame)
	}
	if s.sel == nil && !s.move.on {
		hintText(frame, listHint)
	}
	if s.box() == boxLists {
		s.listsText(a.e, frame)
		if s.tick < s.msgUntil { // a refused row click: above the hint
			ui.Print(frame, fitWidth(s.msg, w, smallFace), smallFace, x, listHintY()-smallFace.Size*1.25, s.msgColour())
		}
		return
	}
	desc := s.moveText(a.e) // boxMove
	ui.Print(frame, desc, bigFace, x, y, textColour)
	if s.tick < s.msgUntil {
		lines := strings.Count(desc, "\n") + 1
		ui.Print(frame, wrap(s.msg, infoCols), bigFace, x, y+float64(lines)*bigFace.Size*1.25, s.msgColour())
	}
}

// combatHelp is combat mode's hint under the banner.
const combatHelp = "End Turn, Space: next round"

// combatText is combat mode's banner at the view's top, "Combat – round
// N" (N the round the next step plays: the day's rounds there, plus one)
// and its hint.
func (s *systemScreen) combatText(a *app, frame *ebiten.Image) {
	x, y := float64(sysArea.X0+padX), float64(sysArea.Y0+padY)
	ui.Print(frame, fmt.Sprintf("Combat – round %d", a.g.RoundsAt(s.star)+1), bigFace, x, y, yellow)
	ui.Print(frame, combatHelp, smallFace, x, y+bigFace.Size*1.25, footColour)
}

// hoverLine is the view's bottom line: the move target's (where a click
// sends the selected ship, its ETA or refusal), else the hovered
// star, planet or lane described on one line; "" for none or a ship
// (a hovered ship has no text).
func (s *systemScreen) hoverLine(e *game.Empire) string {
	if b := s.moveTarget(); b != nil {
		return s.targetText(e, b)
	}
	if b := s.hover; b != nil && b.kind != shipBody {
		return strings.ReplaceAll(s.describe(e, b), "\n", ", ")
	}
	return ""
}

// newsOn reports the news shown: nothing selected.
func (s *systemScreen) newsOn() bool {
	return s.sel == nil && s.fsel == nil && !s.panelOn() && !s.planetOn()
}

// newsBody is the body of this system the hovered news line names, the
// one its click opens: its planet, else the star; nil for another
// system's line, none hovered or the news hidden.
func (s *systemScreen) newsBody(a *app) *body {
	i := s.news.hovered
	if !s.newsOn() || i < 0 || i >= len(a.log) || a.log[i].Ev.Star != s.star {
		return nil
	}
	ev := a.log[i].Ev
	for k := range s.bodies {
		b := &s.bodies[k]
		if ev.Planet >= 0 && b.kind == planetBody && b.idx == ev.Planet || ev.Planet < 0 && b.kind == starBody {
			return b
		}
	}
	return nil
}

// title is the strip's three lines: name (or type), type (or
// "unexplored"), "Day N  M lanes" (the lanes e knows).
func (s *systemScreen) title(e *game.Empire) string {
	m := starLanes(e, e.Cluster, s.star)
	if !e.Knows(s.star) {
		return fmt.Sprintf("%s\nunexplored\nDay %d  %d lanes", s.sys.Star, e.Day, m)
	}
	return fmt.Sprintf("%s\n%s\nDay %d  %d lanes", s.sys.Name, s.sys.Star, e.Day, m)
}

// describe is a body's text, "" for none: hoverLine's (not a ship's).
func (s *systemScreen) describe(e *game.Empire, b *body) string {
	if b == nil {
		return ""
	}
	if b.kind == shipBody && b.owner != s.player {
		return "" // foreignLines reads its Sighting: the inspected panel
	}
	if b.kind == shipBody {
		return s.describeShip(e, b.ship)
	}
	switch b.kind {
	case planetBody:
		d := fmt.Sprintf("%s\n%s size %d", b.planet.Name, b.planet.Type, b.planet.Size)
		if ruinsSeen(e, s.star, b.planet) {
			d += "\n" + ruinsLine
		}
		return d
	case laneBody:
		kind := "Star lane"
		if b.lane.Red {
			kind = "Red star lane"
		}
		if b.lane.To >= 0 {
			kind += " to " + e.StarLabel(b.lane.To)
		}
		return kind
	}
	return e.StarLabel(s.star)
}

// describeShip is the player's ship's text, the keys when selected.
func (s *systemScreen) describeShip(e *game.Empire, sh *game.Starship) string {
	lines := []string{sh.Name}
	lines = append(lines, strings.Split(sh.Design.String(), ", ")...)
	lines = append(lines, fmt.Sprintf("cargo %d", sh.Cargo), fmt.Sprintf("power %d/%d", sh.Power, sh.Design.PowerPerDay()),
		hullText(sh), fmt.Sprintf("shields %s (score %d)", onOff(sh.Shields), sh.Design.ShieldScore()))
	lines = append(lines, shipWhere(e, sh))
	if ol := shipOrder(e, sh); ol != "" {
		lines = append(lines, ol)
	}
	if o := sh.Order; o.Kind == game.GoToPoint {
		lines = append(lines, fmt.Sprintf("x %.2f z %.2f y %.2f", o.Pos.X, o.Pos.Z, o.Pos.Y))
	}
	if sh == s.sel {
		lines = append(lines, "X cancel  F fire", "D shields")
	}
	return strings.Join(lines, "\n")
}

// hullText is "hull H/MaxHull".
func hullText(sh *game.Starship) string { return fmt.Sprintf("hull %d/%d", sh.Hull, sh.MaxHull()) }

// bar is a panel gauge: label, fraction filled, colour, the numbers after.
type bar struct {
	label string
	frac  float64
	c     color.RGBA
	num   string
	tick  float64 // graduations every tick of the gauge (a frac), 0 none
}

// Bar colours: the hull's green, yellow under a half, red under a
// quarter; the energy's blue.
var (
	barGreen  = color.RGBA{90, 200, 90, 255}
	barRed    = color.RGBA{220, 70, 50, 255}
	barEnergy = color.RGBA{80, 170, 230, 255}
)

// shipBars is the ship's hull and energy gauges (0 of 0 is empty).
func shipBars(sh *game.Starship) [2]bar {
	return [2]bar{
		hullBar(sh.Hull, sh.MaxHull()),
		{"energy", gaugeFrac(sh.Power, sh.Design.PowerPerDay()), barEnergy, fmt.Sprintf("%d/%d", sh.Power, sh.Design.PowerPerDay()), 0},
	}
}

// hullBar is the hull's gauge, n of m points: green, yellow under a
// half, red under a quarter.
func hullBar(n, m int) bar {
	h := gaugeFrac(n, m)
	c := barGreen
	switch {
	case h < 0.25:
		c = barRed
	case h < 0.5:
		c = yellow
	}
	return bar{"hull", h, c, fmt.Sprintf("%d/%d", n, m), 0}
}

// gaugeFrac is n/d within [0, 1], 0 for d ≤ 0.
func gaugeFrac(n, d int) float64 {
	if d <= 0 {
		return 0
	}
	return min(1, max(0, float64(n)/float64(d)))
}

// Gauge geometry, native px: the label column, the numbers', the height.
const (
	barLabelW = 7 * 7 * zoom / 2 // "energy " in Go Mono 12
	barNumW   = 8 * 7 * zoom / 2 // "999/999"
	barH      = 5 * zoom
)

// barBreak is an overflowing bar's spill past its gauge, native px.
const barBreak = 10

// drawBar draws b in a text line's slot at (x, y), w wide. A frac over 1
// breaks the gauge's right edge: the outline open there, the fill run
// on barBreak px fading to black, the number after it.
func drawBar(frame *ebiten.Image, b bar, x, y, w float64) {
	ui.Print(frame, b.label, smallFace, x, y, textColour)
	gx, gw, gy := x+barLabelW, w-barLabelW-barNumW, y+(8*zoom-barH)/2
	vector.FillRect(frame, float32(gx), float32(gy), float32(gw), float32(barH), color.RGBA{0, 0, 0, 200}, false)
	in := gw - 2*zoom
	if b.frac > 0 && in > 0 {
		vector.FillRect(frame, float32(gx+zoom), float32(gy+zoom), float32(in*min(b.frac, 1)), float32(barH-2*zoom), b.c, false)
	}
	// The graduations: a 1 px line a little lighter than what it crosses,
	// the fill or the empty gauge.
	for f := b.tick; b.tick > 0 && f < 1; f += b.tick {
		tx, c := gx+zoom+in*f, color.RGBA{60, 60, 60, 255}
		if f < b.frac {
			c = lighten(b.c, 0.35)
		}
		vector.FillRect(frame, float32(math.Round(tx)), float32(gy+zoom), 1, float32(barH-2*zoom), c, false)
	}
	edge, nx := dim(textColour, 0.5), gx+gw+2*zoom
	if b.frac <= 1 {
		strokeRect(frame, rect{int(gx), int(gy), int(gx + gw), int(gy + barH)}, edge)
		ui.Print(frame, b.num, smallFace, nx, y, textColour)
		return
	}
	x0, x1, y0, y1 := float32(gx), float32(gx+gw), float32(gy), float32(gy+barH)
	vector.FillRect(frame, x0, y0, x1-x0, zoom, edge, false)
	vector.FillRect(frame, x0, y1-zoom, x1-x0, zoom, edge, false)
	vector.FillRect(frame, x0, y0, zoom, y1-y0, edge, false)
	for i := range barBreak {
		vector.FillRect(frame, x1-zoom+float32(i), y0+zoom, 1, y1-y0-2*zoom, dim(b.c, 1-float64(i+1)/(barBreak+1)), false)
	}
	ui.Print(frame, b.num, smallFace, nx+barBreak, y, textColour)
}

// scanShields is a Scan's shields as tileState says ours: "down" when
// on but dark, else on or off.
func scanShields(sc *game.Scan) string {
	if sc.Shields && sc.ShieldsDown {
		return "down"
	}
	return onOff(sc.Shields)
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// shipWhere is where sh is: lane, orbit or space.
func shipWhere(e *game.Empire, sh *game.Starship) string {
	cl := e.Cluster
	switch {
	case sh.Lane >= 0:
		return "lane to " + e.StarLabel(cl.Other(sh.Lane, sh.Star))
	case sh.Orbit >= 0:
		return "orbit of " + e.Planet(sh.Star, sh.Orbit).Name
	}
	return "in space"
}

// shipOrder is sh's order with its ETA ("-> Vernunft II, 3 days"), ""
// when idle or unreachable; a refit's hold says where and the days left.
func shipOrder(e *game.Empire, sh *game.Starship) string {
	if c := e.Refitting(sh); c != nil { // held: no order runs
		d, ok := c.DaysLeft()
		switch {
		case c.Blockaded():
			return "being refitted: blockaded, at " + c.Planet.Name
		case !ok:
			return "being refitted: no industry, at " + c.Planet.Name
		}
		// The days first: the panel's line cuts a long planet name.
		return fmt.Sprintf("being refitted: %s at %s", dayCount(max(1, d)), c.Planet.Name)
	}
	days, ok := sh.ETA(e.Cluster, e.Day)
	switch o := sh.Order; {
	case o.Kind == game.GoToSystem && sh.Lane < 0:
		return navOrder(e, sh)
	case o.Kind == game.Colonise:
		return "colonise"
	case o.Kind == game.Invade:
		return invadeText(e, sh)
	case !ok:
		return ""
	case o.Kind == game.GoToPlanet:
		return fmt.Sprintf("-> %s, %s", e.Planet(sh.Star, o.Planet).Name, etaText(days))
	case o.Kind == game.GoToStar:
		return fmt.Sprintf("-> %s, %s", e.StarLabel(o.Star), etaText(days))
	case o.Kind == game.GoToPoint:
		return "-> point, " + etaText(days)
	}
	return ""
}

// invadeText is an Invade order's text, "invade <planet>": Game.EndDay
// resolves it, as the Invade dialog says (cut short for the Ships
// screen's column).
func invadeText(e *game.Empire, sh *game.Starship) string {
	name := "the planet"
	if sh.Orbit >= 0 && sh.Lane < 0 {
		name = e.Planet(sh.Star, sh.Orbit).Name
	}
	return "invade " + name
}

// etaText is "today" for 0 (today's power gets there), else "N days".
func etaText(days int) string {
	if days == 0 {
		return "today"
	}
	return fmt.Sprintf("%d days", days)
}

// infoCols is the bigFace characters a sysInfo line holds (318 native px
// less margins, Go Mono's 0.6 em at 16 px).
const infoCols = 30

// wrap breaks s at spaces into lines of at most n characters (a longer
// word stays whole).
func wrap(s string, n int) string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		switch {
		case line == "":
			line = w
		case len(line)+1+len(w) <= n:
			line += " " + w
		default:
			lines = append(lines, line)
			line = w
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// The ship panel (a ship selected, out of the move mode), native px: name
// (bigFace), five smallFace lines (panelText), then the parts as
// gizmos.shp tiles at scale 1 in four bands (weapons, shields, specials,
// core: the drives, scanners and generators, for the original's
// "passive"), identical parts sharing a tile (Design.Counts
// order, the clickable ones first) with "xN" (tileCount; a weapon's its
// parts, "used U/M" under it: usedText)
// and the state at its bottom-left. The foot holds the hovered tile's
// text or the fire prompt. Empty bands are skipped, tiles past
// panelBottom dropped from the end, their count after the last label.
const (
	panelX0       = 960 // sysInfo.X0 + 12
	panelBandsTop = 428 // sysInfo.Y0 + 12, a bigFace line (20), five smallFace lines (16 each): panelText's design, two bars, where, order; the buttons' row (stopRowH)
	panelLabelH   = 18
	panelCols     = 4
	tileW, tileH  = 70, 54 // gizmos.shp's 70x53 tile, at scale 1
	tilePitch     = 74
	tileRowH      = 58
	panelBottom   = 889 // sysInfo.Y1 − 12 − 3 smallH: the foot's three lines free
	// tileOX, tileOY is gizmos.shp's origin (34,36) in a tile's rect.
	tileOX, tileOY = 34, 36
)

// The bands, top to bottom.
const (
	weaponBand = iota
	shieldBand
	specialBand
	coreBand
	numBands
)

var bandNames = [numBands]string{"weapons", "shields", "specials", "core"}

// gizmoBand is the band a gizmo's tile goes in.
func gizmoBand(g game.Gizmo) int {
	switch game.Gizmos[g].Cat {
	case game.Weapon:
		return weaponBand
	case game.Shield:
		return shieldBand
	case game.Special:
		return specialBand
	}
	return coreBand
}

// panelTile is n parts of gizmo g, the first Design.Parts[part], in r.
type panelTile struct {
	g    game.Gizmo
	n    int
	part int
	band int
	r    rect
}

// usedH is the line under a weapon tile: its shots used today (usedText).
const usedH = 14

// bandRowH is a band's row pitch: the weapons' holds their used line.
func bandRowH(band int) int {
	if band == weaponBand {
		return tileRowH + usedH
	}
	return tileRowH
}

// panelTiles lays sh's parts out, a band's label panelLabelH above its
// first tile, the clickable groups first within a band.
func panelTiles(sh *game.Starship) []panelTile {
	var out []panelTile
	cs := sh.Design.Counts()
	slices.SortStableFunc(cs, func(a, b game.PartCount) int {
		switch ca, cb := tileClickable(a.Gizmo), tileClickable(b.Gizmo); {
		case ca && !cb:
			return -1
		case cb && !ca:
			return 1
		}
		return 0
	})
	y := panelBandsTop
	for band := range numBands {
		i := 0
		for _, c := range cs {
			if gizmoBand(c.Gizmo) != band {
				continue
			}
			x0 := panelX0 + (i%panelCols)*tilePitch
			y0 := y + panelLabelH + (i/panelCols)*bandRowH(band)
			if y0+tileH+bandRowH(band)-tileRowH > panelBottom {
				break
			}
			out = append(out, panelTile{g: c.Gizmo, n: c.N, part: slices.Index(sh.Design.Parts, c.Gizmo), band: band, r: rect{x0, y0, x0 + tileW, y0 + tileH}})
			i++
		}
		if i > 0 {
			y += panelLabelH + (i+panelCols-1)/panelCols*bandRowH(band)
		}
	}
	return out
}

// tileClickable is whether useTile acts on g's tile: a weapon, a shield,
// a special neither passive nor used from the planet screen.
func tileClickable(g game.Gizmo) bool {
	switch gizmoBand(g) {
	case weaponBand, shieldBand:
		return true
	case specialBand:
		return game.Gizmos[g].Flags&game.GzPassive == 0 && planetUse(g) == ""
	}
	return false
}

// panelHidden is how many of sh's groups panelTiles drops.
func panelHidden(sh *game.Starship) int { return len(sh.Design.Counts()) - len(panelTiles(sh)) }

// tileAt is the index of the tile holding (x,y), −1 none.
func tileAt(ts []panelTile, x, y float64) int {
	for i, t := range ts {
		if t.r.has(x, y) {
			return i
		}
	}
	return -1
}

// panelOn is whether the info box shows the selected ship's panel.
func (s *systemScreen) panelOn() bool { return s.sel != nil && !s.move.on }

// panelClick is a left press in the info box with the panel on: Stop
// (as X), Navigate… (as N), a tile's useTile; elsewhere the fire mode
// ends.
func (s *systemScreen) panelClick(a *app, x, y float64) error {
	if sh, on := s.stopState(a.e); sh && stopRect().has(x, y) {
		if !on {
			return nil
		}
		s.aim = false
		return s.order(a.e, game.Order{}) // as X; order ends the auto mode
	}
	if sh, _ := s.stopState(a.e); sh && navRect().has(x, y) {
		if a.e.CanNavigate(s.sel) != nil {
			return nil // greyed
		}
		return s.navigate(a) // as N
	}
	ts := panelTiles(s.sel)
	k := tileAt(ts, x, y)
	if k < 0 {
		s.aim = false
		return nil
	}
	return s.useTile(a, ts[k])
}

// stopRect is the panel's Stop button, native px: right-aligned in the
// info box, on its own row (stopRowH) under panelText's last line
// (order). The lines' y is panelText's: the name (bigFace line), the
// design line, two bars, where and order, 8*zoom each.
func stopRect() rect {
	y := int(float64(sysInfo.Y0+6*zoom)+bigFace.Size*1.25) + 5*8*zoom
	return rect{sysInfo.X1 - 12*zoom - stopW, y + 2, sysInfo.X1 - 12*zoom, y + stopRowH - 2}
}

// stopW is the Stop button's width, stopGap the gap to Navigate…,
// stopRowH their row's height (panelBandsTop leaves it).
const stopW, stopGap, stopRowH = 64, 8, 32

// stopState: the Stop button is shown for e's ships only, lit while the
// ship has an order, armed shots or Auto runs.
func (s *systemScreen) stopState(e *game.Empire) (shown, on bool) {
	if s.sel == nil || !slices.Contains(e.Ships, s.sel) {
		return false, false
	}
	return true, s.sel.Order.Kind != game.NoOrder || len(s.sel.Armed) > 0 || s.auto
}

// useTile acts on tile t: a weapon starts the fire mode, a shield toggles
// the shields, a Colonizer or Invasion Module does nothing (planet
// screen), another special is refused (Usable), aimed (aimSpecial) or
// used at once. All but an aim end the fire mode.
func (s *systemScreen) useTile(a *app, t panelTile) error {
	if t.band == weaponBand {
		s.sel.Weapon, s.aim, s.aimSpecial, s.move = t.part, true, -1, moveMode{}
		return nil
	}
	s.aim, s.aimSpecial = false, -1
	switch {
	case t.band == shieldBand:
		return s.toggleShields()
	case planetUse(t.g) != "":
		return nil
	case t.band == specialBand:
		k := groupPart(s.sel, t)
		if ok, why := s.sel.Usable(k); !ok {
			return errors.New(why)
		}
		if specialAimKind(t.g) != aimNone {
			s.aim, s.aimSpecial, s.aimG, s.move = true, k, t.g, moveMode{}
			return nil
		}
		reps, err := a.e.UseSpecial(s.sel, k, game.Target{})
		if err != nil {
			return err
		}
		s.afterUse(a, reps)
	}
	return nil
}

// groupPart is the first part holding t.g that Usable accepts (a spent
// one gives way), else t.part (its refusal tells why).
func groupPart(sh *game.Starship, t panelTile) int {
	for k, p := range sh.Design.Parts {
		if p != t.g {
			continue
		}
		if ok, _ := sh.Usable(k); ok {
			return k
		}
	}
	return t.part
}

// afterUse logs a special's reports, the first as the message. The use
// may have moved or destroyed ships: the bodies are rebuilt, a ship gone
// from here deselected.
func (s *systemScreen) afterUse(a *app, reps []game.Report) {
	a.record(reps)
	s.note(reps[0].Event.Text)
	s.noteUse(reps[0].Event.Use)
	if s.sel.Star != s.star || !slices.Contains(a.e.Ships, s.sel) {
		s.sel, s.move = nil, moveMode{}
	}
	s.rebuild()
}

// noPower is a weapon's or special's state short of energy, noGen a
// weapon's with every generator taken this round; shieldOn frames a
// switched-on shield.
const (
	noPower = "no power"
	noGen   = "no gen"
)

// actionsText is the weapons band's count in combat: the round's actions
// left, one per generator, each armed shot taking one (Arm; a move takes
// one too in the round).
func actionsText(sh *game.Starship) string {
	return fmt.Sprintf("actions %d/%d", max(sh.Design.Generators()-len(sh.Armed), 0), sh.Design.Generators())
}

var shieldOn = color.RGBA{90, 200, 255, 255}

// armedColour marks the shots armed for the next round: the panel's
// tile frame, the view's dashed line.
var armedColour = color.RGBA{255, 70, 50, 255}

// tileArmed is how many of tile t's parts are armed this round.
func tileArmed(sh *game.Starship, t panelTile) int {
	n := 0
	for p, g := range sh.Design.Parts {
		if g == t.g && sh.ArmedPart(p) {
			n++
		}
	}
	return n
}

// tileState is a tile's state, "" none: the shields' on/off, "down"
// when on but dark, "boosted", "invulnerable", a Space Habitat's
// "aboard/berths"; "no power" on a weapon or active special with a use
// left whose Power exceeds the energy (the original's "insufficient
// power"); "no gen" on a weapon with a part left to arm once the
// armed shots take every generator.
func tileState(sh *game.Starship, t panelTile) string {
	switch {
	case game.Gizmos[t.g].Cat == game.Shield && sh.Shields && sh.ShieldsDown:
		return "down" // a Shield Blaster, or a hit short of the draw (takeShot)
	case game.Gizmos[t.g].Cat == game.Shield:
		return onOff(sh.Shields)
	case t.g == game.Accutron && sh.Boosted:
		return "boosted"
	case t.g == game.Invulnerablizer && sh.Invulnerable:
		return "invulnerable"
	case t.g == game.SpaceHabitat: // our ship's panel only: a foreign one's cargo stays unseen
		return fmt.Sprintf("%d/%d", sh.Pop, sh.Berths())
	case (game.Gizmos[t.g].Cat == game.Weapon || game.Gizmos[t.g].Cat == game.Special) && game.Gizmos[t.g].Flags&game.GzPassive == 0 &&
		sh.Power < game.Gizmos[t.g].Power && (game.Gizmos[t.g].Uses == 0 || usesLeft(sh, t) > 0):
		return noPower
	case game.Gizmos[t.g].Cat == game.Weapon && len(sh.Armed) >= sh.Design.Generators() && tileArmable(sh, t) > 0:
		return noGen
	}
	return ""
}

// tileArmable is how many of tile t's parts could still be armed this
// round but for the generators: not armed (one arm a part a round), a
// shot left today.
func tileArmable(sh *game.Starship, t panelTile) int {
	n, uses := 0, game.Gizmos[t.g].Uses
	for p, g := range sh.Design.Parts {
		if g == t.g && !sh.ArmedPart(p) && (uses == 0 || p >= len(sh.Shots) || sh.Shots[p] < uses) {
			n++
		}
	}
	return n
}

// tileCount is the tile's "xN": a special's uses left today when
// limited, the cargo, the Invaders, else the parts aboard (a weapon's
// too: its shots in usedText).
func tileCount(sh *game.Starship, t panelTile) int {
	in := &game.Gizmos[t.g]
	switch {
	case in.Cat == game.Special && in.Uses > 0:
		return usesLeft(sh, t)
	case t.g == game.Colonizer:
		return sh.Cargo
	case t.g == game.InvasionModule:
		return sh.Invaders
	}
	return t.n
}

// usesLeft is the uses left today of tile t's parts: Uses each, less
// their Shots.
func usesLeft(sh *game.Starship, t panelTile) int {
	in := &game.Gizmos[t.g]
	left := 0
	for k, g := range sh.Design.Parts {
		if g == t.g {
			left += in.Uses
			if k < len(sh.Shots) {
				left -= sh.Shots[k]
			}
		}
	}
	return left
}

// usedText is the line under a weapon tile (the
// "xN" stays the parts aboard, a spent shot no longer looked like a lost
// part): "used U/M", M the parts × Uses a day.
func usedText(sh *game.Starship, t panelTile) string {
	m := t.n * game.Gizmos[t.g].Uses
	return fmt.Sprintf("used %d/%d", m-usesLeft(sh, t), m)
}

// tileText is the hover line for t: "<name> xN[: state, facts]", a
// special's facts being its Usable verdict (or planetUse).
func tileText(sh *game.Starship, t panelTile) string {
	l := fmt.Sprintf("%s x%d", game.Gizmos[t.g].Name, tileCount(sh, t))
	var st []string
	if v := tileState(sh, t); v != "" {
		st = append(st, v)
	}
	if k := tileArmed(sh, t); k > 0 {
		st = append(st, fmt.Sprintf("armed %d for the next round", k))
	}
	if t.band == specialBand {
		ok, why := sh.Usable(groupPart(sh, t))
		if u := planetUse(t.g); u != "" && (ok || t.g == game.SpaceHabitat) { // a habitat is never Usable: always its use
			why = u
		}
		st = append(st, why)
	} else if f := partFacts(t.g); len(f) > 1 {
		st = append(st, f[0])
	}
	if len(st) > 0 {
		l += ": " + strings.Join(st, ", ")
	}
	return l
}

// planetUse says g is used from the planet screen, "" for the others.
func planetUse(g game.Gizmo) string {
	switch g {
	case game.Colonizer:
		return "colonise from the planet screen"
	case game.InvasionModule:
		return "invade from the planet screen"
	case game.SpaceHabitat:
		return "load and unload from the planet screen"
	}
	return ""
}

// panelFoot is the fire mode's prompt, else the hovered tile's text.
func (s *systemScreen) panelFoot(e *game.Empire) string {
	if k, on := s.specialAim(); on && k >= 0 {
		what := map[specAim]string{
			aimHostileShip: "click a hostile ship",
			aimAllyShip:    "click a ship of yours or an ally's",
			aimPlanetBody:  "click a planet",
			aimAnyShip:     "click any ship",
			aimLaneOpening: "click a star lane opening",
		}[specialAimKind(s.aimG)]
		return "use " + game.Gizmos[s.aimG].Name + ": " + what + " (right click cancels)"
	}
	if s.aim && s.sel.Weapon >= 0 && s.sel.Weapon < len(s.sel.Design.Parts) {
		return "fire " + game.Gizmos[s.sel.Design.Parts[s.sel.Weapon]].Name + ": click the target (right click cancels)"
	}
	if s.stopHot {
		return "stop: cancel the order and the armed shots (X)"
	}
	if s.navHot {
		return "navigate: pick any system on the starmap (N)"
	}
	if ts := panelTiles(s.sel); s.tile >= 0 && s.tile < len(ts) {
		return tileText(s.sel, ts[s.tile])
	}
	return ""
}

// drawPanel draws the tiles at scale 1, clipped to the info box, the
// chosen weapon's framed yellow, the hovered one white.
func (s *systemScreen) drawPanel(a *app, sc *ebiten.Image) {
	dst := clipped(sc, sysInfo)
	sh := s.sel
	s.drawPanelFlag(a, dst, s.player)
	drawShieldRow(a, dst, s.panelShields())
	for i, t := range panelTiles(sh) {
		a.spr(gizmoBank, int(t.g)).DrawAt(dst, float64(t.r.X0+tileOX), float64(t.r.Y0+tileOY))
		if t.band == weaponBand && sh.Weapon >= 0 && sh.Weapon < len(sh.Design.Parts) && sh.Design.Parts[sh.Weapon] == t.g {
			frameTile(dst, t.r, 0, yellow)
		}
		if t.band == weaponBand && tileArmed(sh, t) > 0 {
			frameTile(dst, t.r, -1, armedColour) // inside the chosen weapon's frame
		}
		if t.band == shieldBand && sh.Shields { // the original frames a switched-on shield
			frameTile(dst, t.r, 0, shieldOn)
		}
		if i == s.tile {
			frameTile(dst, t.r, 1, color.RGBA{255, 255, 255, 255})
		}
	}
}

// panelWidth is a ship panel's text width.
func panelWidth() float64 { return float64(sysInfo.X1 - sysInfo.X0 - 12*zoom) }

// panelShields is our selected ship's design line laid out.
func (s *systemScreen) panelShields() shieldLine {
	return layShields(s.sel.Design.Name, shipShields(s.sel), float64(panelX0), panelWidth())
}

// foreignShields is the inspected ship's design line laid out: empty
// unscanned (its hull is in the title).
func (s *systemScreen) foreignShields(sg game.Sighting) shieldLine {
	if sg.Scan == nil {
		return shieldLine{}
	}
	return layShields(sg.Scan.Design.Name, scanShieldState(sg.Scan), float64(panelX0), panelWidth())
}

// panelText writes the panel's text on the frame, native px.
func (s *systemScreen) panelText(a *app, frame *ebiten.Image) {
	e, sh := a.e, s.sel
	x := float64(panelX0)
	w := panelWidth()
	y := float64(sysInfo.Y0 + 6*zoom)
	fw := s.panelTitleDX(a, s.player)
	ui.Print(frame, fitWidth(sh.Name, w-fw, bigFace), bigFace, x+fw, y, textColour)
	y = designLineY()
	shieldRowText(frame, s.panelShields(), x)
	y += 8 * zoom
	for _, b := range shipBars(sh) {
		drawBar(frame, b, x, y, w)
		y += 8 * zoom
	}
	if shown, on := s.stopState(e); shown {
		if on {
			drawButton(frame, stopRect(), "Stop", s.stopHot)
		} else {
			drawButtonOff(frame, stopRect(), "Stop")
		}
		if e.CanNavigate(sh) == nil {
			drawButton(frame, navRect(), navLabel, s.navHot)
		} else {
			drawButtonOff(frame, navRect(), navLabel)
		}
	}
	for _, l := range []string{shipWhere(e, sh), shipOrder(e, sh)} {
		ui.Print(frame, fitWidth(l, w, smallFace), smallFace, x, y, textColour)
		y += 8 * zoom
	}

	band, ts := -1, panelTiles(sh)
	hidden := panelHidden(sh)
	for i, t := range ts {
		if t.band != band {
			band = t.band
			label := bandNames[band]
			if band == weaponBand && !s.handHops() {
				label += "  " + actionsText(sh)
			}
			if last := slices.IndexFunc(ts[i:], func(u panelTile) bool { return u.band != band }) < 0; last && hidden > 0 {
				label += fmt.Sprintf(" +%d", hidden)
			}
			ui.Print(frame, label, smallFace, x, float64(t.r.Y0-panelLabelH)+3, dim(textColour, 0.7))
		}
		ly := float64(t.r.Y1) - smallFace.Size - 1
		if t.band == weaponBand {
			ui.Print(frame, usedText(sh, t), smallFace, float64(t.r.X0), float64(t.r.Y1)+1, dim(textColour, 0.8))
		}
		n := fmt.Sprintf("x%d", tileCount(sh, t))
		cw := ui.Width(n, smallFace) + 4
		drawBadge(frame, n, float64(t.r.X1)-cw+2, ly, textColour)
		// The shields' switch shares the foot with the count.
		if st := tileState(sh, t); st != "" && ui.Width(st, smallFace)+4+cw+4 <= tileW {
			c := yellow
			if st == noPower || st == noGen {
				c = red
			}
			drawBadge(frame, st, float64(t.r.X0)+2, ly, c)
		}
	}

	lh := smallH
	y = printFoot(frame, s.panelFoot(e), x, w)
	if s.tick < s.msgUntil {
		ml := strings.Split(wrap(s.msg, footCols), "\n") // a report is a sentence
		for i, l := range ml {
			ui.Print(frame, fitWidth(l, w, smallFace), smallFace, x, y-lh*float64(len(ml)-i), s.msgColour())
		}
	}
}

// The foreign panel (fsel): our ship panel's layout,
// read-only, masked by what the player's scanners read (game.Sighting):
// unscanned only the owner, hull size, where and our hits; scanned the
// design's name, shields, the hull and energy bars and the cargo, the
// energy's maximum printed only with the parts (X Ray Megaglasses), which
// alone show the tiles: "xN" the parts aboard, the shields' on/off the
// only state. No buttons, no clicks, no keys.

// foreignView is the foreign panel's text: title (bigFace, the hull's
// size in it), the bars (scanned only), the where lines; note stands in
// for the tiles, "" when they show (scan.Parts). The design line is
// foreignShields'.
type foreignView struct {
	title string
	bars  []bar
	where []string
	note  string
	scan  *game.Scan
}

// foreignLines is sg's panel text as e reads it.
func foreignLines(e *game.Empire, g *game.Game, sg game.Sighting) foreignView {
	v := foreignView{title: g.Empires[sg.Empire].Species.String() + " " + strings.ToLower(sg.Hull.String()) + " ship", scan: sg.Scan}
	where := "in space"
	if sg.Orbit >= 0 {
		where = "orbit of " + e.Planet(sg.Star, sg.Orbit).Name
	}
	switch sc := sg.Scan; {
	case sc == nil:
		v.note = "not scanned"
	default:
		en := bar{"energy", gaugeFrac(sc.Energy, sc.PowerPerDay), barEnergy, fmt.Sprint(sc.Energy), 0}
		if sc.Parts {
			en.num = fmt.Sprintf("%d/%d", sc.Energy, sc.PowerPerDay)
		} else {
			v.note = "parts unknown (X Ray Megaglasses)"
		}
		v.bars = []bar{hullBar(sc.HullPts, sc.MaxHull), en}
		if sc.Cargo > 0 {
			where += fmt.Sprintf(", cargo %d", sc.Cargo)
		}
	}
	v.where = []string{where}
	return v
}

// foreignTiles is fsel's parts laid out as our panel's (panelTiles on a
// ship carrying the scanned design only), nil unless Scan.Parts.
func (s *systemScreen) foreignTiles() []panelTile {
	sg, ok := s.foreignSeen()
	if !ok || sg.Scan == nil || !sg.Scan.Parts {
		return nil
	}
	return panelTiles(&game.Starship{Design: sg.Scan.Design})
}

// foreignTileText is a foreign tile's hover line, "<name> xN[: shields
// on|off, facts]": the parts aboard, none of our Usable verdicts.
func foreignTileText(t panelTile, shields bool) string {
	l := fmt.Sprintf("%s x%d", game.Gizmos[t.g].Name, t.n)
	var st []string
	if t.band == shieldBand {
		st = append(st, onOff(shields))
	}
	if f := partFacts(t.g); len(f) > 1 {
		st = append(st, f[0])
	}
	if len(st) > 0 {
		l += ": " + strings.Join(st, ", ")
	}
	return l
}

// drawForeign draws the foreign tiles at scale 1, clipped to the info
// box, a switched-on shield framed, the hovered tile white.
func (s *systemScreen) drawForeign(a *app, sc *ebiten.Image) {
	sg, ok := s.foreignSeen()
	if !ok {
		return
	}
	dst := clipped(sc, sysInfo)
	s.drawPanelFlag(a, dst, sg.Empire)
	drawShieldRow(a, dst, s.foreignShields(sg))
	for i, t := range s.foreignTiles() {
		a.spr(gizmoBank, int(t.g)).DrawAt(dst, float64(t.r.X0+tileOX), float64(t.r.Y0+tileOY))
		if t.band == shieldBand && sg.Scan.Shields {
			frameTile(dst, t.r, 0, shieldOn)
		}
		if i == s.tile {
			frameTile(dst, t.r, 1, color.RGBA{255, 255, 255, 255})
		}
	}
}

// panelFlag is empire o's flag, drawn before a ship panel's title, ours
// or a foreign one (for the colour's name).
func (s *systemScreen) panelFlag(a *app, o int) *assets.Sprite {
	return a.flag(s.g.Empires[o].Species, colourOf(s.g, o))
}

// drawPanelFlag draws panelFlag at the panel's left, its top 3 original
// px above the title's so its 25 px clear the design line.
func (s *systemScreen) drawPanelFlag(a *app, dst *ebiten.Image, o int) {
	fl := s.panelFlag(a, o)
	fl.DrawAt(dst, float64(panelX0-fl.Offset.X), float64(sysInfo.Y0+3*zoom-fl.Offset.Y))
}

// panelTitleDX is how far a ship panel's title starts right of panelX0:
// past the flag.
func (s *systemScreen) panelTitleDX(a *app, o int) float64 {
	return float64(s.panelFlag(a, o).Image.Bounds().Dx()) + 8
}

// foreignText writes the foreign panel's text, panelText's geometry, the
// title past the owner's flag.
func (s *systemScreen) foreignText(a *app, frame *ebiten.Image) {
	sg, ok := s.foreignSeen()
	if !ok {
		return
	}
	v := foreignLines(s.e, s.g, sg)
	x := float64(panelX0)
	w := panelWidth()
	y := float64(sysInfo.Y0 + 6*zoom)
	fw := s.panelTitleDX(a, sg.Empire)
	ui.Print(frame, fitWidth(v.title, w-fw, bigFace), bigFace, x+fw, y, textColour)
	y = designLineY()
	shieldRowText(frame, s.foreignShields(sg), x)
	y += 8 * zoom
	for _, b := range v.bars {
		drawBar(frame, b, x, y, w)
		y += 8 * zoom
	}
	for _, l := range v.where {
		ui.Print(frame, fitWidth(l, w, smallFace), smallFace, x, y, textColour)
		y += 8 * zoom
	}
	if v.note != "" {
		ui.Print(frame, fitWidth(v.note, w, smallFace), smallFace, x, y, textColour)
		return
	}
	ts := s.foreignTiles()
	band, hidden := -1, len(v.scan.Design.Counts())-len(ts)
	for i, t := range ts {
		if t.band != band {
			band = t.band
			label := bandNames[band]
			if last := slices.IndexFunc(ts[i:], func(u panelTile) bool { return u.band != band }) < 0; last && hidden > 0 {
				label += fmt.Sprintf(" +%d", hidden)
			}
			ui.Print(frame, label, smallFace, x, float64(t.r.Y0-panelLabelH)+3, dim(textColour, 0.7))
		}
		ly := float64(t.r.Y1) - smallFace.Size - 1
		n := fmt.Sprintf("x%d", t.n)
		cw := ui.Width(n, smallFace) + 4
		drawBadge(frame, n, float64(t.r.X1)-cw+2, ly, textColour)
		if t.band == shieldBand {
			drawBadge(frame, scanShields(v.scan), float64(t.r.X0)+2, ly, yellow)
		}
	}
	if s.tile >= 0 && s.tile < len(ts) {
		printFoot(frame, foreignTileText(ts[s.tile], v.scan.Shields), x, w)
	}
}

// footCols is the smallFace characters a panel line holds (294 native px,
// ~7.3 px advance).
const footCols = 40

// badgeBack is drawBadge's plate.
var badgeBack = color.RGBA{0, 0, 0, 200}

// drawBadge prints str in smallFace at (x, y) over a translucent plate:
// a tile's count, points or state.
func drawBadge(frame *ebiten.Image, str string, x, y float64, c color.Color) {
	vector.FillRect(frame, float32(x-2), float32(y-1), float32(ui.Width(str, smallFace)+4), float32(smallFace.Size+2), badgeBack, false)
	ui.Print(frame, str, smallFace, x, y, c)
}

// frameTile strokes tile r grown by d original px in c.
func frameTile(dst *ebiten.Image, r rect, d int, c color.RGBA) {
	d *= zoom
	strokeRect(dst, rect{r.X0 - d, r.Y0 - d, r.X1 + d, r.Y1 + d}, c)
}

// printFoot writes foot in yellow at the info box's bottom, wrapped at
// footCols; it returns its first line's y.
func printFoot(frame *ebiten.Image, foot string, x, w float64) float64 {
	ls := strings.Split(wrap(foot, footCols), "\n")
	if ls[0] == "" {
		ls = nil
	}
	y := float64(sysInfo.Y1-6*zoom) - float64(len(ls))*smallH
	for i, l := range ls {
		ui.Print(frame, fitWidth(l, w, smallFace), smallFace, x, y+float64(i)*smallH, yellow)
	}
	return y
}
