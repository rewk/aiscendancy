package main

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The planets screen (the original's, capture image0040, denser): a row
// per colony: the orbit in miniature, the globe, the text, the output
// pictures and POP pips at x1. Row r shows the colony colonyOrder puts at
// off+r, looked up at every use: no index is kept.
type planetsScreen struct {
	off   int // rows scrolled: the top row shows colonyOrder's off-th
	hover int // row under the cursor, −1 none
	btn   int // sort button under the cursor (idleButton, projectsButton, sortButton), noButton none
	news  newsView
}

// The sort keys, in the cycle's order; the choice lives on the app
// (planetsIdleFirst, planetsProjectsFirst, planetsSort), so a reopened
// screen keeps it.
const (
	sortFounded = iota // the Colonies order
	sortPop
	sortIndustry
	sortResearch
	sortProsperity
	sortQueue // the construction queue's length
	sortKeys
)

var (
	sortNames = [sortKeys]string{"founded", "pop", "industry", "research", "prosperity", "queue"}
	sortOrder = [sortKeys]string{"oldest first", "most first", "most first", "most first", "most first", "longest first"}
)

// The sort buttons: the two toggles in box 3 (toggleRect), the key in 4.
const (
	idleButton = iota + 100 // clear of the turn box's and the starmap's ids
	sortButton
	projectsButton
)

// colonyKey is c's value under sort key k (not founded).
func colonyKey(c *game.Colony, k int) int {
	ind, res, pro := c.Shown()
	switch k {
	case sortPop:
		return c.Pop
	case sortIndustry:
		return ind
	case sortResearch:
		return res
	case sortQueue:
		return len(c.Queue)
	}
	return pro
}

// colonyOrder lists the indices into e.Colonies as the rows show them:
// by key (founded keeps the Colonies order, the others go descending,
// ties keeping it); when idleFirst, by idleOrder first. Each colony's
// group and key are computed once, before the sort (Shown is the cost).
func colonyOrder(e *game.Empire, idleFirst, projectsFirst bool, key int) []int {
	n := len(e.Colonies)
	ord, group, val := make([]int, n), make([]int, n), make([]int, n)
	for i, c := range e.Colonies {
		ord[i] = i
		if idleFirst {
			group[i] = idleOrder(c, projectsFirst)
		}
		if key != sortFounded {
			val[i] = colonyKey(c, key)
		}
	}
	slices.SortStableFunc(ord, func(i, j int) int {
		if group[i] != group[j] {
			return group[i] - group[j]
		}
		return val[j] - val[i]
	})
	return ord
}

// planetsButtonAt is the sort button under (x, y), noButton none.
func planetsButtonAt(x, y float64) int {
	switch {
	case toggleRect(0).has(x, y):
		return idleButton
	case toggleRect(1).has(x, y):
		return projectsButton
	case mapStack[4].has(x, y):
		return sortButton
	}
	return noButton
}

// toggleRect is toggle k's frame in box 3, 0 Idle first over 1
// Planetary projects first, padX in, toggleGap apart.
func toggleRect(k int) rect {
	r := mapStack[3]
	h := (r.Y1 - r.Y0 - 2*toggleGap - toggleGap) / 2
	y := r.Y0 + toggleGap + k*(h+toggleGap)
	return rect{r.X0 + padX, y, r.X1 - padX, y + h}
}

const toggleGap = 2 * zoom

// toggleOn is a toggle's frame and label when on.
var toggleOn = color.RGBA{110, 220, 110, 255}

// click presses sort button b: a toggle flips, the key moves on (back on
// a right click, both wrapping). The order changes: back to the top.
func (s *planetsScreen) click(a *app, b int, right bool) {
	switch b {
	case idleButton:
		a.planetsIdleFirst = !a.planetsIdleFirst
	case projectsButton:
		a.planetsProjectsFirst = !a.planetsProjectsFirst
	case sortButton:
		d := 1
		if right {
			d = sortKeys - 1
		}
		a.planetsSort = (a.planetsSort + d) % sortKeys
	default:
		return
	}
	s.off = 0
}

func newPlanetsScreen() *planetsScreen {
	return &planetsScreen{hover: -1, btn: noButton, news: newNewsView()}
}

func (s *planetsScreen) Enter(*app) {}

const planetsHelp = "wheel/arrows: scroll  S: species\nclick: go to the planet\nEsc: back"

// Row geometry, native px: the output pictures are their banks' 108x100
// canvases at x1, a row one picture plus rowGap. Left to right, colGap
// apart: the miniature (miniCols columns of five), the globe, the text,
// the pictures, the pips (the asset's 10 px pitch) at the right margin.
const (
	outPicW, outPicH = 108, 100
	rowGap           = 2
	planetRowH       = outPicH + rowGap
	rowGlobe         = outPicH
	outGap, colGap   = 4, 8
	popListPitch     = 10
	popListCols      = 10
	popListRows      = 8
	miniCols         = 2
	miniPitch        = outPicH / (game.OrbitSlots / miniCols)
	miniCell         = miniPitch - 2
	miniW            = miniCols * miniPitch
)

// planetsX0 is the row's left edge, globeX the globe's, planetsTop row
// 0's top.
func planetsX0() int  { return mapArea.X0 + padX }
func globeX() int     { return planetsX0() + miniW + colGap }
func planetsTop() int { return mapArea.Y0 + padY }

// miniRect is orbit slot k's cell in row r: column k%2, row k/2, as on
// the planet screen.
func miniRect(r, k int) rect {
	x := planetsX0() + (k%miniCols)*miniPitch
	y := rowY0(r) + (k/miniCols)*miniPitch
	return rect{x, y, x + miniCell, y + miniCell}
}

// miniTile is what an orbit slot shows: a structure's planitem tile,
// else a ship's hull (orbitShipsAt), else nothing (ok false).
type miniTile struct {
	bank  string
	frame int
	ok    bool
}

func orbitMini(a *app, c *game.Colony) [game.OrbitSlots]miniTile {
	var out [game.OrbitSlots]miniTile
	for k, it := range c.Planet.Orbit {
		if it != game.NoItem {
			out[k] = miniTile{"planitem", int(it), true}
		}
	}
	for _, o := range orbitShipsAt(a, c.Star, c.PlanetIdx, c.Planet, c) {
		out[o.slot] = miniTile{shipBank(o.species), int(o.hull), true}
	}
	return out
}

// drawMini paints sp shrunk to fit r (linear filter), its drawn box
// centred: Sprite.Draw puts the box at the position plus Offset.
func drawMini(dst *ebiten.Image, sp *assets.Sprite, r rect) {
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	k := min(1, float64(r.X1-r.X0)/w, float64(r.Y1-r.Y0)/h)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(r.X0+r.X1)/2-k*(float64(sp.Offset.X)+w/2), float64(r.Y0+r.Y1)/2-k*(float64(sp.Offset.Y)+h/2))
	op.Filter = ebiten.FilterLinear
	sp.Draw(dst, &op)
}

// popListX is the pips' block's left edge, outPicX(i) picture i's.
func popListX() int { return mapArea.X1 - padX - popListCols*popListPitch }
func outPicX(i int) int {
	return popListX() - colGap - (3-i)*outPicW - (2-i)*outGap
}

// planetTextX is the text block's left edge, planetTextW its width.
func planetTextX() int { return globeX() + rowGlobe + colGap }
func planetTextW() int { return outPicX(0) - colGap - planetTextX() }

func planetsRows() int { return (mapArea.Y1 - mapArea.Y0 - 2*padY) / planetRowH }

func rowY0(r int) int { return planetsTop() + r*planetRowH }

// row is the row under native y, −1 outside the rows.
func (s *planetsScreen) row(y float64) int {
	if y < float64(mapArea.Y0) || y >= float64(mapArea.Y1) {
		return -1
	}
	r := y - float64(planetsTop())
	if r < 0 {
		return -1
	}
	if k := int(r) / planetRowH; k < planetsRows() {
		return k
	}
	return -1
}

// colony is the Colonies index row r shows (through the sort order,
// computed now: no index outlives a call), −1 when none.
func (s *planetsScreen) colony(a *app, r int) int {
	if i := listIndex(s.off, r, len(a.e.Colonies)); i >= 0 {
		return colonyOrder(a.e, a.planetsIdleFirst, a.planetsProjectsFirst, a.planetsSort)[i]
	}
	return -1
}

// shown is the Colonies index of each row on screen, top first: one
// sort for the whole frame (colony sorts per call).
func (s *planetsScreen) shown(a *app) []int {
	ord := colonyOrder(a.e, a.planetsIdleFirst, a.planetsProjectsFirst, a.planetsSort)
	var out []int
	for r := range planetsRows() {
		i := listIndex(s.off, r, len(ord))
		if i < 0 {
			break
		}
		out = append(out, ord[i])
	}
	return out
}

func (s *planetsScreen) lineAt(a *app, y float64) int { return s.colony(a, s.row(y)) }

func (s *planetsScreen) scroll(a *app, d int) {
	s.off = min(max(s.off+d, 0), max(0, len(a.e.Colonies)-planetsRows()))
}

// open is what a click on Colonies[i] pushes: its system, its planet.
func (s *planetsScreen) open(a *app, i int) []screen {
	c := a.e.Colonies[i]
	return []screen{newSystemScreen(c.Star), newPlanetScreen(c.Star, c.PlanetIdx)}
}

// planetDesc is "(<Size> <Type> Planet)", static.txt's "(%s %s Planet)".
func planetDesc(p *game.Planet) string {
	return fmt.Sprintf("(%s %s Planet)", sizeWords[p.Size-game.MinSize], p.Type)
}

// colonyLines are a row's lines: name, description, star, project and
// foot (projPanel's). The original pairs name+description and
// project+days on one line each: neither fits beside x1 pictures.
func colonyLines(e *game.Empire, cl *game.Cluster, c *game.Colony) (name, desc, star, proj, foot string) {
	var pp projPanel
	pp.fill(c, "")
	proj = pp.name
	switch _, free, _ := colonyPop(c); {
	case c.Project != nil:
		proj = "Building " + proj
	case len(c.Queue) > 0: // "Waiting: <head>"
	case free == 0: // idle for want of people, not of a choice
		proj = "No Project/No Pop."
	}
	return c.Planet.Name, planetDesc(c.Planet), starLabel(e, cl, c.Star), proj, pp.foot
}

// queueCount is c's queue length as its row shows it, "queue N"; ""
// when empty.
func queueCount(c *game.Colony) string {
	if len(c.Queue) == 0 {
		return ""
	}
	return fmt.Sprintf("queue %d", len(c.Queue))
}

// idleRank is 0 for a colony idle with free people, 1 on an ongoing
// project (it gives way to any start: parked until people are free),
// 2 idle without free people, 3 busy: the project line's
// colour and idleOrder's groups.
func idleRank(c *game.Colony) int {
	switch _, free, _ := colonyPop(c); {
	case c.Project != nil && c.Project.Item.Ongoing():
		return 1
	case c.Project != nil:
		return 3
	case free == 0:
		return 2
	}
	return 0
}

// ongoingColour is an ongoing project's line, light blue.
var ongoingColour = color.RGBA{130, 200, 255, 255}

// idleOrder is c's group in the idle first order: the idle with free
// people, the idle without, then the busy last; the ongoing (Scientist
// Takeover, Endless Party) before all the idle when projectsFirst, else
// after them.
func idleOrder(c *game.Colony, projectsFirst bool) int {
	switch idleRank(c) {
	case 0:
		return 1
	case 1:
		if projectsFirst {
			return 0
		}
		return 3
	case 2:
		return 2
	}
	return 4
}

// projColour is the project line's colour by idleRank: red, light blue,
// yellow, textColour.
func projColour(c *game.Colony) color.RGBA {
	return [4]color.RGBA{errColour, ongoingColour, yellow, textColour}[idleRank(c)]
}

func planetsCounts(e *game.Empire) string {
	n, m := len(e.Colonies), e.ControlledSystems()
	return fmt.Sprintf("%d %s\n%d %s", n, countWord(n, "planet"), m, countWord(m, "system"))
}

// countWord is word, with an s unless n is 1.
func countWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// clusterText is box 2's text: the home, the cluster, the empires (none
// without g).
func clusterText(e *game.Empire, cl *game.Cluster, g *game.Game) string {
	str := fmt.Sprintf("Home: %s\n%s %d stars, %d lanes",
		starLabel(e, cl, cl.Home), cl.Density, len(cl.Stars), len(knownLanes(e, cl)))
	if g != nil {
		str += fmt.Sprintf("\n%d empires", len(g.Empires))
	}
	return str
}

func (s *planetsScreen) Update(a *app) error {
	x, y := a.mouse()
	_, wy := ebiten.Wheel()
	press := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	wheeled, pressed := s.news.input(a, x, y, wy, press)
	if wy != 0 && !wheeled {
		s.scroll(a, notch(wy, 1))
	}
	s.off = keyScroll(s.off, planetsRows())
	s.scroll(a, 0) // colonies may have gone since
	if onSpeciesKey(a, inpututil.IsKeyJustPressed(ebiten.KeyS)) {
		return nil
	}
	if pressed {
		return nil
	}
	s.hover = -1
	if mapArea.has(x, y) {
		s.hover = s.row(y)
	}
	s.btn = planetsButtonAt(x, y)
	right := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if s.btn != noButton {
		if press || right {
			s.click(a, s.btn, right)
		}
		return nil
	}
	if press || right {
		if i := s.colony(a, s.hover); i >= 0 {
			for _, sc := range s.open(a, i) {
				a.push(sc)
			}
		}
	}
	if press {
		a.pressTurn(turnButton(x, y)) // the turn box, as on the log
	}
	return nil
}

// rowStarW is the width the star's suns.shp frame is drawn at behind a
// row's globe: bigger than the globe's box, so only a corner of it shows.
const rowStarW = 110

// drawRowStar draws the star centred on (cx, cy), the globe box's
// top-right corner, into box (the globe's box, clipped): the original
// shows each colony's star big beside its planet; ours peeks out behind
// it, its colour and its size.
func drawRowStar(box *ebiten.Image, sp *assets.Sprite, cx, cy float64) {
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	k := rowStarW / float64(b.Dx())
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(cx-k*(float64(sp.Offset.X)+float64(b.Dx())/2), cy-k*(float64(sp.Offset.Y)+float64(b.Dy())/2))
	op.Filter = ebiten.FilterLinear
	sp.Draw(box, &op)
}

// Render paints the chrome, then each row's pictures.
func (s *planetsScreen) Render(a *app, sc *ebiten.Image) {
	drawMapChrome(a, sc, 5)
	drawNewsBox(sc, newsBox)
	dst := clipped(sc, mapArea)
	at1 := func(bank string, f, x, y int) {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(float64(x), float64(y))
		a.spr(bank, f).Draw(dst, &op)
	}
	for r, i := range s.shown(a) {
		c := a.e.Colonies[i]
		y0 := rowY0(r)
		p := c.Planet
		for k, m := range orbitMini(a, c) {
			if m.ok {
				drawMini(dst, a.spr(m.bank, m.frame), miniRect(r, k))
			}
		}
		drawRowStar(clipped(dst, rect{globeX(), y0, globeX() + rowGlobe, y0 + outPicH}),
			a.spr("suns", int(a.cl.Stars[c.Star].System.Star)), float64(globeX()+rowGlobe), float64(y0))
		fitSprite(dst, a.spr("planets", int(p.Type)*game.MaxSize+p.Size-1),
			float64(globeX()+rowGlobe/2), float64(y0+outPicH/2), rowGlobe, outPicH)
		for k, v := range outputValues(c) {
			at1(outputBanks[k], outputFrame(v), outPicX(k), y0)
		}
		if d := shiftCue(c); d != 0 { // in the gap from industry's picture (1) to the other's
			g := outPicX(1) - outGap/2
			if d > 0 {
				g = outPicX(2) - outGap/2
			}
			drawShift(dst, float64(g), float64(y0+outPicH/2), shiftW*2/3, shiftH*2/3, d)
		}
		if c.Singular() { // research's share to industry
			drawShift(dst, float64(outPicX(1)-outGap/2), float64(y0+outPicH/2), shiftW*2/3, shiftH*2/3, 1)
		}
		busy, free, room := colonyPop(c)
		icons, _ := popGrid(busy, free, room, popListCols, popListRows, popListPitch)
		for _, ic := range icons {
			at1("17np", int(ic.kind), popListX()+ic.x, y0+ic.y)
		}
	}
	drawScrollCues(a, sc, mapArea, s.off, planetsRows(), len(a.e.Colonies))
}

// Text writes each row's lines, output values and POP line, the boxes.
func (s *planetsScreen) Text(a *app, frame *ebiten.Image) {
	if len(a.e.Colonies) == 0 {
		ui.Print(frame, "no colonies", bigFace, float64(planetTextX()), float64(planetsTop()), textColour)
	}
	tw := float64(planetTextW())
	for r, i := range s.shown(a) {
		c := a.e.Colonies[i]
		y0 := float64(rowY0(r))
		name, desc, star, proj, foot := colonyLines(a.e, a.cl, c)
		nc, pc := textColour, projColour(c)
		if r == s.hover {
			nc = yellow
		}
		y := y0
		for k, l := range []struct {
			s    string
			face *text.GoTextFace
			c    color.RGBA
		}{
			{name, bigFace, nc}, {desc, bigFace, textColour}, {star, smallFace, dim(textColour, 0.6)},
			{proj, bigFace, pc}, {foot, smallFace, textColour},
		} {
			w := tw
			if q := queueCount(c); k == 3 && q != "" { // the project line ends on the queue's length
				qw := ui.Width(q, smallFace)
				ui.Print(frame, q, smallFace, float64(planetTextX())+tw-qw, y+(bigFace.Size-smallFace.Size)/2, textColour)
				w -= qw + 8
			}
			ui.Print(frame, fitWidth(l.s, w, l.face), l.face, float64(planetTextX()), y, l.c)
			y += l.face.Size * 1.25
		}
		for k, v := range outputValues(c) {
			outputBadge(frame, v, outPicX(k)+outPicW, int(y0)+outPicH)
		}
		busy, free, room := colonyPop(c)
		_, more := popGrid(busy, free, room, popListCols, popListRows, popListPitch)
		pop := fmt.Sprintf("POP %d/%d", busy+free, busy+free+room)
		if more > 0 {
			pop += fmt.Sprintf(" +%d", more)
		}
		ui.Print(frame, pop, smallFace, float64(popListX()), y0+float64(popListRows*popListPitch)+2, textColour)
	}
	putBox(frame, mapStack[0], planetsHelp, smallFace, textColour)
	putBox(frame, mapStack[1], planetsCounts(a.e), bigFace, textColour)
	putBox(frame, mapStack[2], clusterText(a.e, a.cl, a.g), smallFace, textColour)
	// The toggles: a frame and its label, green when on, yellow hovered;
	// projects first dim while idle first is off (no effect then).
	for k, b := range []struct {
		button int
		label  string
		on     bool
	}{
		{idleButton, "Idle first", a.planetsIdleFirst},
		{projectsButton, "Planetary projects first", a.planetsProjectsFirst},
	} {
		c := dim(textColour, 0.6)
		if b.on {
			c = toggleOn
		}
		if k == 1 && !a.planetsIdleFirst {
			c = dim(c, 0.5)
		}
		r := toggleRect(k)
		strokeRect(frame, r, c)
		if s.btn == b.button {
			c = yellow
		}
		t := fitWidth(b.label, float64(r.X1-r.X0-2*padX), smallFace)
		ui.Print(frame, t, smallFace, float64(r.X0+r.X1)/2-ui.Width(t, smallFace)/2, float64(r.Y0+r.Y1)/2-smallFace.Size*0.6, c)
	}
	// The key: a label (yellow hovered) over its state, as the starmap's boxes.
	c := textColour
	if s.btn == sortButton {
		c = yellow
	}
	r := mapStack[4]
	putBox(frame, r, "Sort: "+sortNames[a.planetsSort], bigFace, c)
	ui.Print(frame, fitWidth(sortOrder[a.planetsSort], float64(r.X1-r.X0-2*padX), smallFace), smallFace,
		float64(r.X0+padX), float64(r.Y0+padY)+bigFace.Size*1.25, c)
	s.news.text(a, frame)
}
