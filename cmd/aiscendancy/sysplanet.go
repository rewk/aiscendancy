package main

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The selected planet (capture image0039): a first click selects it, a
// second (or on the panel's globe) opens its planet screen; one selection
// at a time, ship or planet.
//
// The planet panel, in the info box, native px: name over the globe
// (planalNN fitted in a planetPicD disc, top right), smallFace lines left
// of it, then the orbital structures as planitem tiles at scale 1 in
// three bands (weapons, shields, other). The player's colony shows shots
// left ("xN") and hit points left; another's are bare (the original hides
// their damage). A press on one of the player's weapons enters its fire
// mode (pslot). The player's colony with a Tractor Beam (a surface
// structure) shows it last among the structures, slot −1, "xN" pulls left
// this round; its press enters the tractor mode (ptract): a click on any
// ship in space pulls it at once (Empire.UseTractor).
const (
	planetPicD     = 128                   // the globe's drawn diameter
	planetPicTop   = 296                   // sysInfo.Y0 + 12
	planetPicCX    = listX1 - planetPicD/2 // the globe's centre
	planetPicCY    = planetPicTop + planetPicD/2
	planetTextX1   = listX1 - planetPicD - 8       // the lines' right edge
	planetBandsTop = planetPicTop + planetPicD + 8 // the first band's label
	ptileW, ptileH = 70, 70                        // planitem's orbital canvas, scale 1
	ptileRowH      = 74                            // a tile row's pitch
	ptileO         = 35                            // the canvas's origin, its centre
	planetLabelH   = 20                            // a band's label line
	numPlanetBands = 3                             // weapons, shields, other
)

var planetBandNames = [numPlanetBands]string{"weapons", "shields", "structures"}

// The lines a foreign cloaker leaves (game.Empire.SeesOrbit, SeesSurface).
const (
	orbitCloakedLine   = "orbit: cloaked"
	surfaceCloakedLine = "surface cloaked"
)

// planetTile is the structure it in orbit slot, in band, drawn in r.
type planetTile struct {
	it   game.Item
	slot int
	band int
	r    rect
}

// planetBand is the band of orbital structure it: 0 a weapon, 1 a
// shield (game.OrbitalDefences), 2 anything else, the Orbital Cloaker
// too (a cascade layer, not a shield).
func planetBand(it game.Item) int {
	d, ok := it.Defence()
	switch {
	case ok && d.Weapon():
		return 0
	case ok && it != game.OrbitalCloaker:
		return 1
	}
	return 2
}

// planetTiles lays p's orbit out: a band's label planetLabelH above its
// first tile, rows of panelCols at tilePitch, tiles past panelBottom
// dropped.
func planetTiles(p *game.Planet) []planetTile { return planetLayout(p, false) }

// planetLayout is planetTiles, with beam the Tractor Beam's tile (slot
// −1) after the other structures.
func planetLayout(p *game.Planet, beam bool) []planetTile {
	var out []planetTile
	y := planetBandsTop
	for band := range numPlanetBands {
		items := p.Orbit[:]
		if beam && band == 2 {
			items = append(slices.Clone(items), game.TractorBeam)
		}
		i := 0
		for slot, it := range items {
			if it == game.NoItem || planetBand(it) != band {
				continue
			}
			if slot >= game.OrbitSlots {
				slot = -1
			}
			x0 := panelX0 + (i%panelCols)*tilePitch
			y0 := y + planetLabelH + (i/panelCols)*ptileRowH
			if y0+ptileH > panelBottom {
				break
			}
			out = append(out, planetTile{it: it, slot: slot, band: band, r: rect{x0, y0, x0 + ptileW, y0 + ptileH}})
			i++
		}
		if i > 0 {
			y += planetLabelH + (i+panelCols-1)/panelCols*ptileRowH
		}
	}
	return out
}

// planetTileAt is the index of the tile holding (x,y), −1 none.
func planetTileAt(ts []planetTile, x, y float64) int {
	for i, t := range ts {
		if t.r.has(x, y) {
			return i
		}
	}
	return -1
}

// ptiles is the selected planet's tiles, the Tractor Beam's on the
// player's colony holding one; none on an orbit the player does not see
// (SeesOrbit).
func (s *systemScreen) ptiles() []planetTile {
	if !s.e.SeesOrbit(s.psel) {
		return nil
	}
	col := s.pselColony()
	return planetLayout(s.psel, col != nil && col.Planet.Count(game.TractorBeam) > 0)
}

// pullsLeft is col's Tractor Beam pulls left this round.
func pullsLeft(col *game.Colony) int {
	return max(0, col.Planet.Count(game.TractorBeam)-col.Tractors)
}

// planetOn is whether the info box shows the selected planet's panel.
func (s *systemScreen) planetOn() bool { return s.psel != nil && !s.move.on }

// pselIdx is the selected planet's index in the system, −1 none.
func (s *systemScreen) pselIdx() int { return slices.Index(s.sys.Planets, s.psel) }

// pselColony is the player's colony on the selected planet, nil none.
func (s *systemScreen) pselColony() *game.Colony {
	if s.e == nil || s.psel == nil || s.psel.Owner != s.player {
		return nil
	}
	return s.e.Colony(s.star, s.pselIdx())
}

// selectPlanet selects b's planet, dropping the ship (ours or the
// inspected foreign one), fire and move modes.
func (s *systemScreen) selectPlanet(b *body) {
	s.sel, s.psel, s.fsel, s.back, s.aim, s.move = nil, b.planet, nil, nil, false, moveMode{}
}

// shotsLeft is the shots slot's orbital weapon has left today.
func shotsLeft(col *game.Colony, slot int) int {
	d, _ := col.Planet.Orbit[slot].Defence()
	return max(0, d.Shots-col.OrbitShots[slot])
}

// planetLines is the panel's lines left of the globe: type and size,
// owner, guard (or "orbit: cloaked" when e does not see it), ruins.
func (s *systemScreen) planetLines(e *game.Empire) []string {
	p := s.psel
	out := []string{fmt.Sprintf("%s size %d", p.Type, p.Size)}
	switch o := p.Owner; {
	case o < 0:
		out = append(out, "no colony")
	case o == s.player:
		out = append(out, "our colony")
	case s.g != nil && o < len(s.g.Empires):
		out = append(out, fmt.Sprintf("%s colony, %s", s.g.Empires[o].Species, game.ColourName(colourOf(s.g, o))))
	default:
		out = append(out, "a colony")
	}
	switch {
	case !e.SeesOrbit(p):
		out = append(out, orbitCloakedLine)
	case s.g != nil:
		if gd, ok := s.g.Guard(s.star, s.pselIdx()); ok {
			out = append(out, "guarded: "+game.ColourName(colourOf(s.g, gd)))
		}
	}
	if ruinsSeen(e, s.star, p) {
		out = append(out, ruinsLine)
	}
	return out
}

// planetTileText is the hover line for t: the structure, a weapon's
// damage, shots and range, and on the player's colony shots and points
// left.
func (s *systemScreen) planetTileText(t planetTile) string {
	l := t.it.String()
	if col := s.pselColony(); t.slot < 0 && col != nil {
		g := game.Gizmos[game.GizmoTractorBeam]
		n := pullsLeft(col)
		return fmt.Sprintf("%s: pulls a ship %d toward the planet, range %d, %d pull%s left this round (no shots then)",
			l, g.Special1, g.Range, n, map[bool]string{true: "s"}[n != 1])
	}
	var st []string
	d, ok := t.it.Defence()
	if ok && d.Weapon() {
		st = append(st, fmt.Sprintf("%d dmg x%d a day, range %d", d.Damage, d.Shots, d.Range))
	}
	if col := s.pselColony(); col != nil && ok {
		if d.Weapon() {
			n := shotsLeft(col, t.slot)
			st = append(st, fmt.Sprintf("%d shot%s left", n, map[bool]string{true: "s"}[n != 1]))
		}
		if left, ok := s.psel.DefenceLeft(s.e, t.slot); ok {
			st = append(st, fmt.Sprintf("%d/%d pts", left, d.Strength))
		}
	}
	if len(st) > 0 {
		l += ": " + strings.Join(st, ", ")
	}
	return l
}

// planetFoot is the fire or tractor mode's prompt, else the hovered
// tile's text.
func (s *systemScreen) planetFoot() string {
	if s.aim && s.ptract {
		return "Tractor Beam: click a ship in space (right click cancels)"
	}
	if s.aim {
		return "fire " + s.psel.Orbit[s.pslot].String() + ": click the target (right click cancels)"
	}
	if ts := s.ptiles(); s.tile >= 0 && s.tile < len(ts) {
		return s.planetTileText(ts[s.tile])
	}
	return ""
}

// onGlobe is whether (x,y) is on the panel's globe disc.
func onGlobe(x, y float64) bool {
	return math.Hypot(x-planetPicCX, y-planetPicCY) <= planetPicD/2
}

// planetClick is a left press in the info box with the planet panel on:
// the globe opens the planet screen, a player's weapon starts its fire
// mode, its Tractor Beam the tractor mode, anything else ends either.
func (s *systemScreen) planetClick(a *app, x, y float64) error {
	s.aim = false
	if onGlobe(x, y) {
		s.move = moveMode{}
		a.push(newPlanetScreen(s.star, s.pselIdx()))
		return nil
	}
	ts := s.ptiles()
	k := planetTileAt(ts, x, y)
	switch {
	case k < 0:
		return nil
	case ts[k].slot < 0: // only the player's colony shows the beam
		s.ptract, s.aim = true, true
		return nil
	case ts[k].band != 0:
		return nil
	case s.pselColony() == nil:
		return errors.New("not our colony")
	}
	s.pslot, s.ptract, s.aim = ts[k].slot, false, true
	return nil
}

// planetTractor pulls b's ship (the player's own or a sighted one) with
// the selected colony's Tractor Beam, at once; b no ship does nothing.
func (s *systemScreen) planetTractor(a *app, b *body) error {
	if a.g == nil || a.e == nil {
		return errors.New("no game")
	}
	if b == nil || b.kind != shipBody {
		return nil
	}
	col := s.pselColony()
	if col == nil {
		return errors.New("not our colony")
	}
	t, err := tractorTarget(a.g, a.e, b)
	if err != nil {
		return err
	}
	reps, err := a.e.UseTractor(col, t)
	if err != nil {
		return err
	}
	a.record(reps)
	s.note(reps[0].Event.Text)
	s.rebuild()
	return nil
}

// tractorTarget is the beam's Target on ship body b: Mine for e's, else
// e's sighting.
func tractorTarget(g *game.Game, e *game.Empire, b *body) (game.Target, error) {
	if b.owner == e.Index {
		return game.Target{Kind: game.ShipTarget, Mine: b.ship}, nil
	}
	sg, ok := g.SightingOf(e, b.ship)
	if !ok {
		return game.Target{}, errors.New("no such target")
	}
	return game.Target{Kind: game.ShipTarget, Ship: sg}, nil
}

// planetShoot arms the selected planet's weapon (pslot) at t: it fires
// in the next round played here (a step, or the close's first).
func (s *systemScreen) planetShoot(a *app, t *game.Starship) error {
	if a.g == nil || a.e == nil {
		return errors.New("no game")
	}
	col := s.pselColony()
	if col == nil {
		return errors.New("not our colony")
	}
	sg, err := hostileSighting(a, t)
	if err != nil {
		return err
	}
	return a.e.ArmPlanet(col, s.pslot, sg)
}

// planetShootPlanet arms the selected planet's weapon (pslot) at planet
// pl's cascade (ArmPlanetAt): it fires in the next round played here.
func (s *systemScreen) planetShootPlanet(a *app, pl int) error {
	if a.e == nil {
		return errors.New("no game")
	}
	col := s.pselColony()
	if col == nil {
		return errors.New("not our colony")
	}
	return a.e.ArmPlanetAt(col, s.pslot, pl)
}

// shootPlanet arms the selected ship's chosen weapon at planet pl (its
// orbit cascade): the first part holding sel.Weapon's gizmo accepted,
// else sel.Weapon's refusal.
func (s *systemScreen) shootPlanet(a *app, pl int) error {
	if a.e == nil {
		return errors.New("no game")
	}
	sh := s.sel
	if sh.Weapon < 0 || sh.Weapon >= len(sh.Design.Parts) {
		return a.e.ArmAtPlanet(sh, sh.Weapon, pl)
	}
	err := a.e.ArmAtPlanet(sh, sh.Weapon, pl)
	if err == nil {
		return nil
	}
	for k, g := range sh.Design.Parts {
		if k != sh.Weapon && g == sh.Design.Parts[sh.Weapon] && a.e.ArmAtPlanet(sh, k, pl) == nil {
			return nil
		}
	}
	return err
}

// planetAimLabel is the fire mode's label on a hovered planet: "fire at
// <planet>", or the refusal: a planet's weapon CanArmPlanetAt's; a
// ship's CanHitPlanet's, then the chosen weapon's PlanetInRange's (the
// same gizmo's other parts reach as far: shootPlanet); ok false when
// none hovered (or the Tractor Beam is aimed).
func (s *systemScreen) planetAimLabel() (b *body, label string, live, ok bool) {
	if !s.aim || s.hover == nil || s.hover.kind != planetBody || s.g == nil || s.e == nil {
		return nil, "", false, false
	}
	if s.psel != nil {
		col := s.pselColony()
		if s.ptract || col == nil {
			return nil, "", false, false
		}
		if err := s.e.CanArmPlanetAt(col, s.pslot, s.hover.idx); err != nil {
			return s.hover, err.Error(), false, true
		}
		return s.hover, "fire at " + s.hover.planet.Name, true, true
	}
	if s.sel == nil {
		return nil, "", false, false
	}
	if err := s.g.CanHitPlanet(s.e, s.star, s.hover.idx); err != nil {
		return s.hover, err.Error(), false, true
	}
	if err := s.sel.PlanetInRange(s.g.Cluster, s.sel.Weapon, s.hover.idx); err != nil {
		return s.hover, err.Error(), false, true
	}
	return s.hover, "fire at " + s.hover.planet.Name, true, true
}

// drawPlanetPanel draws the globe and the tiles, clipped to the info box,
// the fire or tractor mode's tile framed yellow, the hovered tile white;
// the beam's surface picture shrunk into its tile.
func (s *systemScreen) drawPlanetPanel(a *app, sc *ebiten.Image) {
	dst := clipped(sc, sysInfo)
	p := s.psel
	if sp := a.spr(globeBank(p.Type), p.Size-1); sp.Image != nil {
		k := planetPicD / float64(max(sp.Frame.Box.Dx(), sp.Frame.Box.Dy()))
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(planetPicCX, planetPicCY)
		op.Filter = ebiten.FilterLinear
		sp.Draw(dst, &op)
	}
	for i, t := range s.ptiles() {
		if t.slot < 0 {
			drawMini(dst, a.spr("planitem", int(t.it)), t.r)
		} else {
			a.spr("planitem", int(t.it)).DrawAt(dst, float64(t.r.X0+ptileO), float64(t.r.Y0+ptileO))
		}
		if s.aim && (s.ptract && t.slot < 0 || !s.ptract && t.slot == s.pslot) {
			frameTile(dst, t.r, 0, yellow)
		}
		if i == s.tile {
			frameTile(dst, t.r, 1, color.RGBA{255, 255, 255, 255})
		}
	}
}

// planetText writes the planet panel's text on the frame, native px.
func (s *systemScreen) planetText(e *game.Empire, frame *ebiten.Image) {
	p := s.psel
	x := float64(panelX0)
	w := float64(sysInfo.X1 - sysInfo.X0 - 12*zoom)
	y := float64(sysInfo.Y0 + 6*zoom)
	ui.Print(frame, fitWidth(p.Name, w, bigFace), bigFace, x, y, textColour)
	y += bigFace.Size * 1.25
	for _, l := range s.planetLines(e) {
		ui.Print(frame, fitWidth(l, planetTextX1-x, smallFace), smallFace, x, y, textColour)
		y += 8 * zoom
	}

	col := s.pselColony()
	band := -1
	for _, t := range s.ptiles() {
		if t.band != band {
			band = t.band
			ui.Print(frame, planetBandNames[band], smallFace, x, float64(t.r.Y0-planetLabelH)+2, dim(textColour, 0.7))
		}
		if col == nil {
			continue // another's: bare
		}
		ly := float64(t.r.Y1) - smallFace.Size - 1
		cw := 0.0
		if t.slot < 0 {
			n := fmt.Sprintf("x%d", pullsLeft(col))
			drawBadge(frame, n, float64(t.r.X1)-ui.Width(n, smallFace)-2, ly, textColour)
			continue
		}
		if t.band == 0 {
			n := fmt.Sprintf("x%d", shotsLeft(col, t.slot))
			cw = ui.Width(n, smallFace) + 4
			drawBadge(frame, n, float64(t.r.X1)-cw+2, ly, textColour)
		}
		if left, ok := p.DefenceLeft(e, t.slot); ok {
			d, _ := t.it.Defence()
			pts := fmt.Sprintf("%d/%d", left, d.Strength)
			if t.band == 0 {
				drawBadge(frame, pts, float64(t.r.X0)+2, ly, yellow)
			} else {
				drawBadge(frame, pts, float64(t.r.X1)-ui.Width(pts, smallFace)-2, ly, yellow)
			}
		}
	}

	y = printFoot(frame, s.planetFoot(), x, w)
	if s.tick < s.msgUntil {
		ui.Print(frame, fitWidth(s.msg, w, smallFace), smallFace, x, y-smallH, s.msgColour())
	}
}
