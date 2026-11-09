package main

import (
	"fmt"
	"image/color"
	"log"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The system's lists (docs/screens.md "The lists"): with nothing
// selected the info box lists the view's planets or ships, one tab at a
// time, the tabs in the panel's two filter holes. Rows are indices into
// s.bodies (rebuilt every tick), so a hovered row sets s.hover to that
// very body and a press on it is a click on it in the view.
//
// Sprites at scale 1 (a row would not hold them at x2), shrunk only to
// fit: a planet up to 71 px (size 5 shrinks), a ship (up to 157x101) into
// 104x56. A ship's first line leads with cos_ship.shp frame 3 (the 5x5
// pennant) in its owner's colour: the sprites are the species', not the
// empire's.

// boxMode is what the info box shows.
type boxMode uint8

const (
	boxLists   boxMode = iota
	boxMove            // the move mode's text
	boxPanel           // the selected ship's panel
	boxPlanet          // the selected planet's panel (sysplanet.go)
	boxForeign         // the inspected foreign ship's read-only panel (fsel)
)

// box is what the info box shows now.
func (s *systemScreen) box() boxMode {
	switch {
	case s.panelOn():
		return boxPanel
	case s.planetOn():
		return boxPlanet
	case s.fsel != nil:
		return boxForeign
	case s.move.on:
		return boxMove
	}
	return boxLists
}

// listsOn is whether the lists take the mouse: no selection, no move mode.
func (s *systemScreen) listsOn() bool {
	return s.sel == nil && s.psel == nil && s.fsel == nil && !s.move.on
}

// The lists' geometry, native px: rows of listRowH(tab) from listRowsTop
// to the hint line, x from panelX0 to listX1, the sprite centred in a
// listIconW(tab) cell, the lines right of it.
const (
	listRowsTop = 296  // sysInfo.Y0 + 12
	listX1      = 1254 // sysInfo.X1 - 12
	filterMax   = 5
	listHint    = "F1, ?: keys"
	shipMarkBnk = "cos_ship"
	shipMarkFr  = 3 // the bright 5x5 pennant of the green ramp
)

// rowFlagAt is the top-left of a planet row's owner flag (raceflag.shp at
// the asset's size) in row rr: the globe cell's corner, where the round
// globe leaves room. No ring: a circle is the view's orbit marker.
func rowFlagAt(rr rect) (x, y float64) { return float64(rr.X0 + 2), float64(rr.Y0 + 2) }

// rowTextX is where row rr's lines start in tab t, past the sprite.
func rowTextX(t int, rr rect) float64 { return float64(rr.X0 + listIconW(t) + 4) }

// The tabs, listTab's values.
const (
	planetsTab = iota
	shipsTab
)

// listRowH is a row's height in tab t: five planet rows (a system's
// most) fill the strip.
func listRowH(t int) int {
	if t == planetsTab {
		return 75
	}
	return 60
}

// listIconW is the sprite cell's width in tab t.
func listIconW(t int) int {
	if t == planetsTab {
		return 76
	}
	return 108
}

// The project column: a row of the player's colony ends on its project's
// tile (a ship's hull) in projIconH over the days left, in projColW.
const (
	projColW  = 56
	projIconH = 44
)

// rowProject is b's project column: the tile (bank "" when idle), the
// foot and its colour; ok false unless b is the player's colony.
func (s *systemScreen) rowProject(b *body) (bank string, frame int, foot string, c color.RGBA, ok bool) {
	if b.kind != planetBody || b.planet.Owner != s.player {
		return "", 0, "", c, false
	}
	var col *game.Colony
	for _, cc := range s.e.Colonies {
		if cc.Star == s.star && cc.PlanetIdx == b.idx {
			col = cc
		}
	}
	if col == nil {
		return "", 0, "", c, false
	}
	c = projColour(col)
	pj := col.Project
	if pj == nil {
		foot = "No Project"
		switch _, free, _ := colonyPop(col); {
		case len(col.Queue) > 0:
			foot = "Waiting"
		case free == 0:
			foot = "No Pop."
		}
		return "", 0, foot, c, true
	}
	bank, frame = "planitem", int(pj.Item)
	if pj.Design != nil {
		bank, frame = colonyShips(s.g, col), int(pj.Design.Hull)
	}
	if _, on := ongoingFoot(col); on {
		foot = "ongoing"
	} else if col.Blockaded() {
		foot = "blockaded"
	} else if n, left := col.DaysLeft(); !left {
		foot = "no industry"
	} else {
		foot = dayCount(max(1, n))
	}
	return bank, frame, foot, c, true
}

// projIconRect is the project tile's cell in row rr; projFootY the
// foot line's top.
func projIconRect(rr rect) rect {
	return rect{rr.X1 - 4 - projColW, rr.Y0 + 2, rr.X1 - 4, rr.Y0 + 2 + projIconH}
}
func projFootY(rr rect) float64 { return float64(rr.Y1-2) - smallFace.Size*1.25 }

// listHintY is the hint line's top, above the news box's border.
func listHintY() float64 { return float64(newsBox.Y0-len(newsRim)) - smallFace.Size*1.25 - 4 }

// listRowsN is the rows tab t shows.
func listRowsN(t int) int { return int((listHintY() - 4 - listRowsTop) / float64(listRowH(t))) }

// listRect is visible row r of tab t.
func listRect(t, r int) rect {
	h := listRowH(t)
	y0 := listRowsTop + r*h
	return rect{panelX0, y0, listX1, y0 + h - 2}
}

// listRows is the two tabs' rows as indices into s.bodies: the planets
// in order; the ships out of lanes (all during a replay), the player's
// first, then by owner.
func (s *systemScreen) listRows() [2][]int {
	var out [2][]int
	for i, b := range s.bodies {
		switch {
		case b.kind == planetBody:
			out[planetsTab] = append(out[planetsTab], i)
		case b.kind == shipBody && (s.replay != nil || b.ship.Lane < 0):
			out[shipsTab] = append(out[shipsTab], i)
		}
	}
	rank := func(i int) int {
		if o := s.bodies[i].owner; o != s.player {
			return 1 + o
		}
		return 0
	}
	slices.SortStableFunc(out[shipsTab], func(i, j int) int { return rank(i) - rank(j) })
	return out
}

// clampLists keeps each tab's scroll within its rows.
func (s *systemScreen) clampLists() {
	rows := s.listRows()
	for t := range rows {
		s.listOff[t] = max(0, min(s.listOff[t], len(rows[t])-listRowsN(t)))
	}
}

// listAt is the body under (x,y) in the tab shown, nil for none.
func (s *systemScreen) listAt(x, y float64) *body {
	t, rows := s.listTab, s.listRows()
	for r := range listRowsN(t) {
		if k := s.listOff[t] + r; k < len(rows[t]) && listRect(t, r).has(x, y) {
			return &s.bodies[rows[t][k]]
		}
	}
	return nil
}

// listHoverAt sets s.hover to the row's body under (x,y) when the lists
// are on (listHover set).
func (s *systemScreen) listHoverAt(x, y float64) {
	s.listHover = false
	if !s.listsOn() {
		return
	}
	if b := s.listAt(x, y); b != nil {
		s.hover, s.listHover = b, true
	}
}

// showTab is a press on filter t: its list shown, the selection and the
// move mode dropped.
func (s *systemScreen) showTab(t int) {
	s.listTab, s.sel, s.psel, s.fsel, s.back, s.aim = t, nil, nil, nil, nil, false
	s.move = moveMode{}
}

// listClick is a left press on the lists: a row's body clicked in the
// view, or for a ship in orbit (hidden from the view) its planet's screen.
func (s *systemScreen) listClick(a *app, x, y float64) {
	b := s.listAt(x, y)
	switch {
	case b == nil:
	case orbitHidden(b):
		a.push(s.orbitScreen(b))
	default:
		s.clickList(a, b)
	}
}

// orbitScreen is the planet screen b orbits, b's dialog open when it is
// the player's.
func (s *systemScreen) orbitScreen(b *body) *planetScreen {
	ps := newPlanetScreen(s.star, b.orbit)
	if b.owner == s.player {
		ps.openShipDlg(b.ship)
	}
	return ps
}

// wheel scrolls the tab shown a row a notch over the box with the lists
// on, putting back focal (steer has already zoomed); true when it took
// the wheel.
func (s *systemScreen) wheel(x, y, wy, focal float64) bool {
	if wy == 0 || !s.listsOn() || !sysInfo.has(x, y) {
		return false
	}
	s.cam.Focal = focal
	if wy > 0 {
		s.listOff[s.listTab]--
	} else {
		s.listOff[s.listTab]++
	}
	s.clampLists()
	return true
}

// rowText is a row's lines: a planet's name, size and type words, owner's
// species, "Xeno ruins" when seen; the player's ship's name and design;
// another's hull ("scanned" or "hull", "ship" unseen) and species; a
// ship in orbit adds "in orbit of <planet>" (the view hides it), the
// player's then its order (orderText, the fleet screen's: 4 lines of
// 15 px fill the 60 px row).
func (s *systemScreen) rowText(b *body) []string {
	out := s.rowLines(b)
	if b.kind == shipBody && b.orbiting && b.orbit < len(s.sys.Planets) {
		out = append(out, "in orbit of "+s.sys.Planets[b.orbit].Name)
	}
	if b.kind == shipBody && b.owner == s.player && s.e != nil {
		out = append(out, orderText(s.e, b.ship))
	}
	return out
}

// rowLines is rowText before the orbit line.
func (s *systemScreen) rowLines(b *body) []string {
	switch {
	case b.kind == planetBody:
		ts := sizeWords[b.planet.Size-game.MinSize] + " " + b.planet.Type.String()
		var out []string
		if b.planet.Name != "" {
			out = append(out, b.planet.Name)
		}
		out = append(out, ts)
		if o := b.planet.Owner; o >= 0 {
			out = append(out, s.g.Empires[o].Species.String())
		}
		if ruinsSeen(s.e, s.star, b.planet) {
			out = append(out, ruinsLine)
		}
		return out
	case b.owner == s.player:
		return []string{b.ship.Name, b.ship.Design.Name}
	case b.seen == nil:
		return []string{"ship", b.species.String()}
	case b.seen.Scan != nil:
		return []string{b.seen.Hull.String() + " scanned", b.species.String()}
	}
	return []string{b.seen.Hull.String() + " hull", b.species.String()}
}

// ruinsLine is the lists' and info box's line for Xeno ruins.
const ruinsLine = "Xeno ruins"

// ruinsSeen reports p of star holds Xeno ruins, e knows the star and
// SeesSurface (always with no empire).
func ruinsSeen(e *game.Empire, star int, p *game.Planet) bool {
	_, ok := p.RuinsAt()
	return ok && knows(e, star) && e.SeesSurface(p)
}

// shipMark is cos_ship's pennant in colour c.
func (a *app) shipMark(c int) *assets.Sprite { return a.pennant(shipMarkFr, c) }

// pennant is cos_ship's frame f (0-3, the starmap's depth levels) in
// colour c.
func (a *app) pennant(f, c int) *assets.Sprite {
	s, err := a.a.SpriteIn(shipMarkBnk, f, c)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// filterBg is the filters' plate: the original's filter blue (16battle
// frames 7-9).
var filterBg = color.RGBA{8, 16, 40, 255}

// fitSprite draws sp scaled to fit w x h centred at (cx, cy), never
// enlarged. Its drawn box is centred, not its origin: smship hulls sit
// anywhere on their canvas.
func fitSprite(dst *ebiten.Image, sp *assets.Sprite, cx, cy, w, h float64) {
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	k, x, y := fitPlace(float64(b.Dx()), float64(b.Dy()), float64(sp.Offset.X), float64(sp.Offset.Y), cx, cy, w, h)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x, y)
	sp.Draw(dst, &op)
}

// fitPlace is fitSprite's geometry: the scale k ≤ 1 fitting an sw x sh
// box (its top-left off from the origin) into w x h, and the origin's
// place centring the box on (cx, cy).
func fitPlace(sw, sh, offX, offY, cx, cy, w, h float64) (k, x, y float64) {
	k = min(1, w/sw, h/sh)
	return k, cx - k*(offX+sw/2), cy - k*(offY+sh/2)
}

// drawFilters paints the two filters: the plate, the first filterMax
// rows' sprites in equal cells, the tab shown framed, the hovered yellow.
func (s *systemScreen) drawFilters(a *app, sc *ebiten.Image) {
	rows := s.listRows()
	for t, r := range sysFilters {
		fillRectWith(sc, r, filterBg)
		dst := clipped(sc, r)
		n := min(len(rows[t]), filterMax)
		cw := float64(r.X1-r.X0-4-14) / float64(max(n, 1)) // 14 px: the count
		for i := range n {
			b := &s.bodies[rows[t][i]]
			cx := float64(r.X0+2) + cw*(float64(i)+0.5)
			fitSprite(dst, a.spr(b.sprite()), cx, float64(r.Y0+r.Y1)/2, cw-2, float64(r.Y1-r.Y0-6))
		}
		switch {
		case s.btn == planetsFilter+t:
			strokeRect(sc, r, yellow)
		case s.listTab == t:
			strokeRect(sc, r, textColour)
		}
	}
}

// filtersText writes each filter's count at its bottom-right corner.
func (s *systemScreen) filtersText(frame *ebiten.Image) {
	rows := s.listRows()
	for t, r := range sysFilters {
		n := fmt.Sprint(len(rows[t]))
		ui.Print(frame, n, smallFace, float64(r.X1-3*zoom)-ui.Width(n, smallFace), float64(r.Y1-2*zoom)-smallFace.Size*1.2, textColour)
	}
}

// drawLists paints the visible rows' sprites, owner marks, project
// tiles and the hovered row's frame, clipped to the box.
func (s *systemScreen) drawLists(a *app, sc *ebiten.Image) {
	dst := clipped(sc, sysInfo)
	rows := s.listRows()
	t := s.listTab
	for r := range listRowsN(t) {
		k := s.listOff[t] + r
		if k >= len(rows[t]) {
			break
		}
		b := &s.bodies[rows[t][k]]
		rr := listRect(t, r)
		cell := listIconW(t)
		cx, cy := float64(rr.X0+cell/2), float64(rr.Y0+rr.Y1)/2
		sp := a.spr(b.sprite())
		fw, fh := float64(cell-4), float64(rr.Y1-rr.Y0-4)
		fitSprite(dst, sp, cx, cy, fw, fh)
		switch b.kind {
		case planetBody:
			if o := b.planet.Owner; o >= 0 && s.g != nil {
				fx, fy := rowFlagAt(rr)
				a.flag(s.g.Empires[o].Species, colourOf(s.g, o)).DrawAt(dst, fx, fy)
			}
		case shipBody:
			y := rowTextY(rr, len(s.rowText(b))) + (smallFace.Size*1.25-5*zoom)/2
			drawSprite(dst, a.shipMark(colourOf(s.g, b.owner)), float64(rr.X0+cell+4), y)
		}
		if bank, frame, _, _, ok := s.rowProject(b); ok && bank != "" {
			drawMini(dst, a.spr(bank, frame), projIconRect(rr))
		}
		if b == s.hover {
			strokeRect(dst, rr, hoverColor)
		}
	}
}

// rowTextY is the top of n smallFace lines centred in row rr.
func rowTextY(rr rect, n int) float64 {
	return float64(rr.Y0+rr.Y1)/2 - float64(n)*smallFace.Size*1.25/2
}

// listsText writes the rows' lines, "a-b/N" on the hint line when the
// tab overflows, "unexplored" for an unknown star's planets.
func (s *systemScreen) listsText(e *game.Empire, frame *ebiten.Image) {
	rows := s.listRows()
	t := s.listTab
	n := listRowsN(t)
	head := dim(textColour, 0.7)
	if len(rows[t]) > n {
		h := fmt.Sprintf("%d-%d/%d", s.listOff[t]+1, min(len(rows[t]), s.listOff[t]+n), len(rows[t]))
		ui.Print(frame, h, smallFace, float64(panelX0), listHintY(), head)
	}
	for r := range n {
		k := s.listOff[t] + r
		if k >= len(rows[t]) {
			break
		}
		b := &s.bodies[rows[t][k]]
		rr := listRect(t, r)
		x := rowTextX(t, rr)
		lines := s.rowText(b)
		lh, y := smallFace.Size*1.25, rowTextY(rr, len(lines))
		x1 := float64(rr.X1)
		if _, _, foot, c, ok := s.rowProject(b); ok {
			x1 = float64(projIconRect(rr).X0 - 2) // "Enormous Supermineral" fits
			ui.Print(frame, foot, smallFace, float64(rr.X1-4)-ui.Width(foot, smallFace), projFootY(rr), c)
		}
		for i, l := range lines {
			lx, c := x, head
			if i == 0 {
				c = textColour
				if b.kind == shipBody {
					lx += 5*zoom + 4 // after the pennant
				}
			}
			ui.Print(frame, fitWidth(l, x1-lx, smallFace), smallFace, lx, y+float64(i)*lh, c)
		}
	}
	if t == planetsTab && !knows(e, s.star) {
		ui.Print(frame, "unexplored", smallFace, float64(panelX0), listRowsTop+4, head)
	}
}

// hintText writes the F1 hint above the news, right-aligned.
func hintText(frame *ebiten.Image, hint string) {
	ui.Print(frame, hint, smallFace, float64(sysInfo.X1-6*zoom)-ui.Width(hint, smallFace), listHintY(), dim(textColour, 0.7))
}
