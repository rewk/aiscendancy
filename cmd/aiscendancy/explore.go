package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The exploration popup: the original's notice when one of the
// player's ships reaches a star never visited (help.txt event20), our
// picture the star and its planets instead of the ship. Fed by diffing
// Empire.Visited around endDay (explore); after the discovery queue,
// before the ability popup, under the same rules; never saved.

// exploration is a queued star and the ship naming the notice.
type exploration struct {
	star int
	ship string
}

// exploreText is help.txt's event20, its double spaces kept: the ship,
// the star, the planets and the lanes with their plurals, then ours:
// exploreOwners' sentence, empty without one.
const exploreText = "%s: We have discovered the %s System.  It contains %d planet%s and %d star lane%s.  %sWe're awaiting further orders."

// The pictures' row in discContent, native px: the star's suns.shp
// frame fitted to an exploreStarW box, then a cell per planet, its globe
// fitted to the cell's width (at most exploreGlobeW) over its name. Every
// globe takes the scale fitting an Enormous one to the globe box of a
// system at its most, exploreMaxPlanets, so sizes read across systems.
const (
	exploreMaxPlanets = 5
	exploreStarW      = 120
	exploreGlobeW     = 80
	exploreCellMax    = 2 * exploreGlobeW // a lone planet's name room
	exploreCellGap    = discGap / 2
	exploreFlagW      = 15 * zoom / 2 // a holder's flag (raceflag, 15 wide at zoom) half over the globe
)

var (
	exploreLineH = int(bigFace.Size * 1.25)
	exploreNameH = int(smallFace.Size * 1.25)
)

// explore queues, in star index order, every star known now and not in
// visited (Visited before the day; a star past its length was not)
// where one of the player's ships sits out of a lane, the first in
// Ships order naming it. Stars learnt otherwise give nothing.
func (a *app) explore(visited []bool) {
	for star := range a.cl.Stars {
		if !a.e.Knows(star) || star < len(visited) && visited[star] {
			continue
		}
		if i := slices.IndexFunc(a.e.Ships, func(sh *game.Starship) bool { return sh.Star == star && sh.Lane < 0 }); i >= 0 {
			a.explored = append(a.explored, exploration{star, a.e.Ships[i].Name})
		}
	}
}

// exploring is whether the popup is up: as discovering, after the
// discovery queue.
func (a *app) exploring() bool {
	if len(a.explored) == 0 || len(a.discoveries) > 0 || !a.inGame() || a.over() {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// exploreMessage is x's notice, exploreText filled.
func (a *app) exploreMessage(x exploration) string {
	st := a.cl.Stars[x.star]
	np, nl := len(st.System.Planets), len(st.Lanes)
	owners := ""
	if os := a.exploreOwners(x.star); len(os) > 0 {
		names := make([]string, len(os))
		for i, o := range os {
			names[i] = a.g.Empires[o].Species.String()
		}
		owners = "The " + andList(names) + " have colonies here.  "
	}
	return fmt.Sprintf(exploreText, x.ship, st.Name, np, plural(np), nl, plural(nl), owners)
}

// exploreOwners are the other empires holding a planet of star, in
// empire order (the ship there sees them); none without a game.
func (a *app) exploreOwners(star int) []int {
	if a.g == nil || a.e == nil {
		return nil
	}
	var out []int
	for o := range a.g.Empires {
		if o == a.e.Index {
			continue
		}
		if slices.ContainsFunc(a.cl.Stars[star].System.Planets, func(p *game.Planet) bool { return p.Owner == o }) {
			out = append(out, o)
		}
	}
	return out
}

// andList joins "A", "A and B", "A, B and C".
func andList(s []string) string {
	if len(s) < 2 {
		return strings.Join(s, "")
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}

// exploreCell is a cell's width for n planets: the row's room left of
// the star shared, at most exploreCellMax.
func exploreCell(n int) int {
	c := discContent()
	return min(exploreCellMax, (c.X1-c.X0-exploreStarW-discGap-(n-1)*exploreCellGap)/n)
}

// exploreLayoutT is a popup laid out: the box, its two buttons, the
// star's box, a cell per planet (the globe's box inside, its name and
// the name's y), the text lines and their top.
type exploreLayoutT struct {
	box, goTo, ok rect
	star          rect
	cells, globes []rect
	names         []string
	nameY, textY  int
	lines         []string
}

// exploreLayout lays x's popup out in the fixed box: the row (the star,
// then the cells sharing the width left, at most exploreCellMax each)
// centred across, the text under it, the pair centred down; the globes
// and their names centred on the star's middle.
func exploreLayout(a *app, x exploration) exploreLayoutT {
	c := discContent()
	mid := (dlgBar.X0 + dlgBar.X1) / 2
	l := exploreLayoutT{box: dlgBox, goTo: rect{dlgBar.X0, dlgBar.Y0, mid, dlgBar.Y1}, ok: rect{mid, dlgBar.Y0, dlgBar.X1, dlgBar.Y1}}
	w := c.X1 - c.X0
	cols := int(float64(w) / ui.Width("m", bigFace))
	for _, s := range reflow(paragraph{false, a.exploreMessage(x)}, cols) {
		l.lines = append(l.lines, fitWidth(s, float64(w), bigFace))
	}
	ps := a.cl.Stars[x.star].System.Planets
	n := len(ps)
	roww, cell := exploreStarW, 0
	if n > 0 {
		cell = exploreCell(n)
		roww += discGap + n*cell + (n-1)*exploreCellGap
	}
	x0 := (c.X0 + c.X1 - roww) / 2
	y0 := c.Y0 + (c.Y1-c.Y0-(exploreStarW+discGap+len(l.lines)*exploreLineH))/2
	l.star = rect{x0, y0, x0 + exploreStarW, y0 + exploreStarW}
	l.textY = l.star.Y1 + discGap
	g := min(cell, exploreGlobeW)
	gy := y0 + (exploreStarW-g-exploreNameH)/2
	l.nameY = gy + g
	for i, p := range ps {
		cx := l.star.X1 + discGap + i*(cell+exploreCellGap)
		l.cells = append(l.cells, rect{cx, y0, cx + cell, l.star.Y1})
		l.globes = append(l.globes, rect{cx + (cell-g)/2, gy, cx + (cell-g)/2 + g, gy + g})
		l.names = append(l.names, fitWidth(p.Name, float64(cell), smallFace))
	}
	return l
}

func (a *app) exploreUpdate() error {
	// Not Space: the day's key does nothing under a popup.
	keys := keyJustPressed(ebiten.KeyEnter) || keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3)
	x, y := a.mouse()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if keyJustPressed(ebiten.KeyG) || click && len(a.explored) > 0 && exploreLayout(a, a.explored[0]).goTo.has(x, y) {
		a.exploreGo()
		return nil
	}
	a.explorePress(keys, click, x, y)
	return nil
}

// exploreGo is Go to System or G: the head closes, the stack drops to
// the starmap and the star's system opens on it, the queue going on over
// it.
func (a *app) exploreGo() {
	if len(a.explored) == 0 {
		return
	}
	star := a.explored[0].star
	a.explored = a.explored[1:]
	for i, sc := range a.stack {
		if _, ok := sc.(*starmapScreen); ok {
			a.popTo(i)
			break
		}
	}
	a.push(newSystemScreen(star))
}

// explorePress closes the head on a closing key or a click on OK or
// outside the box; nothing else.
func (a *app) explorePress(key, click bool, x, y float64) {
	if len(a.explored) == 0 {
		return
	}
	l := exploreLayout(a, a.explored[0])
	if key || click && (l.ok.has(x, y) || !l.box.has(x, y)) {
		a.explored = a.explored[1:]
	}
}

// drawExplore paints the queue's head.
func (a *app) drawExplore(frame *ebiten.Image) {
	x := a.explored[0]
	l := exploreLayout(a, x)
	fillRectWith(frame, l.box, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(l.box.X0), float64(l.box.Y0))
	if a.a != nil {
		fitScaled(frame, a.spr("suns", int(a.cl.Stars[x.star].System.Star)), l.star, zoom)
	}
	g := min(exploreCell(exploreMaxPlanets), exploreGlobeW)
	k := min(a.globeScale(rect{0, 0, g, g}), zoom)
	owners := a.exploreOwners(x.star)
	for i, p := range a.cl.Stars[x.star].System.Planets {
		if a.a != nil {
			fitScaled(frame, a.spr("planets", int(p.Type)*game.MaxSize+p.Size-1), l.globes[i], k)
		}
		if o := p.Owner; a.a != nil && slices.Contains(owners, o) { // its holder's flag on the globe's top right
			e := a.g.Empires[o]
			drawSprite(frame, a.flag(e.Species, e.Colour), float64(l.globes[i].X1-exploreFlagW), float64(l.globes[i].Y0))
		}
		c := l.cells[i]
		ui.Print(frame, l.names[i], smallFace, float64(c.X0+c.X1)/2-ui.Width(l.names[i], smallFace)/2, float64(l.nameY), textColour)
	}
	for k, s := range l.lines {
		ui.Print(frame, s, bigFace, float64(l.box.X0+l.box.X1)/2-ui.Width(s, bigFace)/2, float64(l.textY+k*exploreLineH), textColour)
	}
	mx, my := a.mouse()
	for _, b := range []struct {
		r   rect
		txt string
	}{{l.goTo, "Go to System (G)"}, {l.ok, "OK (Enter)"}} {
		col := textColour
		if b.r.has(mx, my) {
			col = yellow
		}
		ui.Print(frame, b.txt, bigFace, float64(b.r.X0+b.r.X1)/2-ui.Width(b.txt, bigFace)/2, float64(b.r.Y0+b.r.Y1)/2-bigFace.Size*0.6, col)
	}
}

// globeScale is the scale fitting every type's MaxSize globe to r: a
// globe drawn at it shows its planet's size.
func (a *app) globeScale(r rect) float64 {
	k := 0.0
	for t := range int(game.NumPlanetTypes) {
		im := a.spr("planets", t*game.MaxSize+game.MaxSize-1).Image
		if im == nil {
			continue
		}
		f := min(float64(r.X1-r.X0)/float64(im.Bounds().Dx()), float64(r.Y1-r.Y0)/float64(im.Bounds().Dy()))
		if k == 0 || f < k {
			k = f
		}
	}
	return k
}

// fitScaled draws sp fitted to r, its drawn box centred, at most kmax
// times (0: no cap), Linear when not at zoom.
func fitScaled(dst *ebiten.Image, sp *assets.Sprite, r rect, kmax float64) {
	if sp.Image == nil {
		return
	}
	b := sp.Image.Bounds()
	bw, bh := float64(b.Dx()), float64(b.Dy())
	k := min(float64(r.X1-r.X0)/bw, float64(r.Y1-r.Y0)/bh)
	if kmax > 0 {
		k = min(k, kmax)
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(r.X0+r.X1)/2-k*(float64(sp.Offset.X)+bw/2), float64(r.Y0+r.Y1)/2-k*(float64(sp.Offset.Y)+bh/2))
	if k != zoom {
		op.Filter = ebiten.FilterLinear
	}
	sp.Draw(dst, &op)
}
