package main

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
)

// The research screen's tree view, the original's look: the techs as
// bubbles on a vertical cylinder of radius 1, a row per level down it,
// seen from a little above by a cam.Camera; view only, the rules stay in
// game.Research.
//
// The layout is computed once from game.Techs. A tech's level is 0
// without prerequisites, else 1 + its prerequisites' highest (memoized:
// restree.txt points forward in places). Its angle θ around the axis
// (0 the front, +Z at Yaw 0) puts it at (sin θ, −level·resLevelH,
// cos θ). The roots sit evenly spaced, the first at −(n−1)/2 of 2π/n
// (−120°, 0°, +120° for the three). A deeper tech wants the circular mean
// of its prerequisites' angles (atan2 of the summed sines and cosines; a
// sum ~0 takes the first prerequisite's); its level sorted by that (ties
// by index), resPasses relaxation passes push circular neighbours closer
// than g = min(2π/n, resMinGap) apart, half the deficit each.
//
// The view turns the cylinder by yaw and looks at height yc: a tech is
// projected at pos − (0,yc,0). Shown: the known techs, the studiable ones
// and the topic; a locked tech is hidden (no spoiler). Lines join a shown
// tech to its shown prerequisites (so a tech known out of order, by a
// dig or a trade, links only to a prerequisite known or studiable), and
// lines and bubbles paint back to front, a line at its middle's depth,
// each bubble resring.shp's ring (frame 0 orange-red the topic, 2 light blue
// hovered, 3 pink the selected, frame 2 tinted sea green (resKnownTint)
// a known tech, 1 blue the rest) over restree.shp's
// picture: whole when known, grown with progress/cost while unknown (a
// dropped topic keeps its points, so its picture stays), a dot for the
// topic at none, else nothing. Every sprite of a bubble scales by the projection
// (zoom at resRef, the default scale at the axis' depth) and dims with
// depth.
//
// The controls are the starmap's on a cylinder: a left drag turns it (dx)
// and moves along its axis (dy), the near side following the cursor; the
// wheel zooms (Focal within cam's limits, about the area's centre); the
// arrows turn (left, right) and move along the axis (up, down); R
// re-centres as Enter does. The chrome's two button holes (the original's
// 20res.shp arrows) are left as the chrome draws them.

// The cylinder and its camera (farther, levels
// 0.85 apart, between the first try's 0.55 and 1.2). Focal/Dist = 292
// native px per unit at the axis' depth: the cylinder's ±1 spans 585 of
// the area's 940 px. A bubble draws at zoom where the scale is resRef, so
// a 60 px ring is 90 px at the axis. Dist 8 keeps the perspective weak
// (the far side 7/9 of the near); a level's ellipse (~150 px at Pitch
// 15°) sits apart from the next (0.85·cos 15°·292 = 240 px), about 3
// levels in the area's 854.
const (
	resLevelH = 0.85
	resPitch  = 15 * math.Pi / 180
	resDist   = 8
	resFocal  = 2340
	resRef    = 390 // px per unit where a bubble draws at zoom
	resMinGap = 50 * math.Pi / 180
	resPasses = 500

	resRingR   = 30             // the ring's outer radius, art px (60x60, origin the centre)
	resRingIn  = 26.0 / 30      // the ring's inner edge over its outer radius (5 px thick, a pixel in)
	resGrowMin = 0.06           // the topic's picture at no progress: a dot
	resSpinV   = 0.025          // radians per tick an arrow is held
	resScrollV = resLevelH / 30 // per tick held: a level in half a second
)

// treeNode is a tech's place on the cylinder.
type treeNode struct {
	level int
	theta float64 // radians, 0 the front, in [−π, π]
}

var resTree, resLevels = layoutTree()

// techLevels is each tech's level, memoized over the prerequisites.
func techLevels() [game.NumTechs]int {
	var lv [game.NumTechs]int
	for i := range lv {
		lv[i] = -1
	}
	var level func(t game.Tech) int
	level = func(t game.Tech) int {
		if lv[t] < 0 {
			l := 0
			for _, p := range game.Techs[t].Requires {
				l = max(l, level(p)+1)
			}
			lv[t] = l
		}
		return lv[t]
	}
	for t := range game.NumTechs {
		level(t)
	}
	return lv
}

// resGap is the least angle between neighbours of a level of n.
func resGap(n int) float64 { return min(2*math.Pi/float64(n), resMinGap) }

// layoutTree places every tech (see the file comment) and counts the levels.
func layoutTree() (out [game.NumTechs]treeNode, levels int) {
	lv := techLevels()
	for t, l := range lv {
		out[t].level = l
		levels = max(levels, l+1)
	}
	byLevel := make([][]game.Tech, levels)
	for t := range game.NumTechs {
		byLevel[lv[t]] = append(byLevel[lv[t]], t)
	}
	roots := byLevel[0]
	for i, t := range roots {
		out[t].theta = (float64(i) - float64(len(roots)-1)/2) * 2 * math.Pi / float64(len(roots))
	}
	for _, ts := range byLevel[1:] {
		want := make(map[game.Tech]float64, len(ts))
		for _, t := range ts {
			var sx, cz float64
			req := game.Techs[t].Requires
			for _, p := range req {
				s, c := math.Sincos(out[p].theta)
				sx, cz = sx+s, cz+c
			}
			want[t] = out[req[0]].theta
			if math.Hypot(sx, cz) > 1e-9 {
				want[t] = math.Atan2(sx, cz)
			}
		}
		order := slices.Clone(ts) // index order already: a stable sort keeps ties so
		slices.SortStableFunc(order, func(a, b game.Tech) int { return cmpFloat(want[a], want[b]) })
		n := len(order)
		th := make([]float64, n) // ascending, unwrapped: the last gap wraps by 2π
		for i, t := range order {
			th[i] = want[t]
		}
		g := resGap(n)
		for range resPasses {
			for i := range n {
				j := (i + 1) % n
				gap := th[j] - th[i]
				if j == 0 {
					gap += 2 * math.Pi
				}
				if d := g - gap; d > 0 {
					th[i] -= d / 2
					th[j] += d / 2
				}
			}
		}
		for i, t := range order {
			out[t].theta = math.Remainder(th[i], 2*math.Pi)
		}
	}
	return out, levels
}

// treePos is t's world position on the cylinder.
func treePos(t game.Tech) game.Vec3 {
	s, c := math.Sincos(resTree[t].theta)
	return game.Vec3{X: s, Y: -float64(resTree[t].level) * resLevelH, Z: c}
}

// treeCamera is the view's camera, the axis at resArea's centre.
func (s *researchScreen) treeCamera() cam.Camera {
	return cam.Camera{Yaw: s.yaw, Pitch: resPitch, Dist: resDist, Focal: s.treeFocal(),
		Cx: float64(resArea.X0+resArea.X1) / 2, Cy: float64(resArea.Y0+resArea.Y1) / 2}
}

// treeFocal is the view's Focal: s.focal, resFocal while 0.
func (s *researchScreen) treeFocal() float64 {
	if s.focal == 0 {
		return resFocal
	}
	return s.focal
}

// treeProject is t's projection in the current view.
func (s *researchScreen) treeProject(t game.Tech) cam.Point {
	c := s.treeCamera()
	p := treePos(t)
	p.Y -= s.yc
	return c.Project(p)
}

// treeShown reports t drawn: known, studiable or the topic.
func treeShown(r *game.Research, t game.Tech) bool {
	return r.Known[t] || r.CanStudy(t) || r.Topic == t
}

// bubble is a shown tech projected: its centre, k native px per art
// pixel, its ring's outer radius in native px, z its paint depth: the
// view depth less the height's share (Depth + y·sin Pitch), so only the
// cylinder's near and far sides order the paint; with the height in, a
// bubble above the axis would count nearer and paint over a front one.
type bubble struct {
	t       game.Tech
	p       cam.Point
	k, r, z float64
}

// bubbles are the shown techs, back to front.
func (s *researchScreen) bubbles(r *game.Research) []bubble {
	var out []bubble
	for t := range game.NumTechs {
		if treeShown(r, t) {
			p := s.treeProject(t)
			k := zoom * p.Scale / resRef
			z := p.Depth + (treePos(t).Y-s.yc)*math.Sin(resPitch)
			out = append(out, bubble{t, p, k, resRingR * k, z})
		}
	}
	slices.SortStableFunc(out, func(a, b bubble) int { return cmpFloat(b.z, a.z) })
	return out
}

// treeAt is the front-most shown tech whose ring holds (x,y) in resArea.
func (s *researchScreen) treeAt(r *game.Research, x, y float64) (game.Tech, bool) {
	if !resArea.has(x, y) {
		return game.NoTech, false
	}
	bs := s.bubbles(r)
	for i := len(bs) - 1; i >= 0; i-- {
		if math.Hypot(x-bs[i].p.X, y-bs[i].p.Y) <= bs[i].r {
			return bs[i].t, true
		}
	}
	return game.NoTech, false
}

// centre aims the view at the topic: its level's height, its angle to the
// front; no topic: the first studiable tech; none: level 0, yaw 0.
func (s *researchScreen) centre(r *game.Research) {
	t := r.Topic
	if t >= game.NumTechs {
		if av := r.Available(); len(av) > 0 {
			t = av[0]
		}
	}
	s.yaw, s.yc, s.focal = 0, 0, 0
	if t < game.NumTechs {
		s.yaw, s.yc = resTree[t].theta, -float64(resTree[t].level)*resLevelH
	}
}

// clampView holds yc within the levels' heights, Focal within cam's limits.
func (s *researchScreen) clampView() {
	s.yc = min(max(s.yc, -float64(resLevels-1)*resLevelH), 0)
	if s.focal != 0 {
		s.focal = min(max(s.focal, cam.MinFocal), cam.MaxFocal)
	}
}

// treeInput is the tree's input for a tick, without Ebiten: the left
// button's press, release and hold, the wheel's wy, the arrows held as
// turn (right − left) and along (down − up). A press in resArea starts a
// drag (dx turns, dy moves along the axis, the near side following the
// cursor), its release without straying a click (researchScreen.click) on the bubble
// under it; the wheel zooms; the arrows move the tree as the starmap's
// pan does. The hover is set first.
func (s *researchScreen) treeInput(a *app, x, y float64, press, release, held bool, wy, turn, along float64) {
	t, ok := s.hoverAt(a, x, y)
	switch {
	case press:
		s.drag.press(x, y, resArea.has(x, y))
	case release:
		if s.drag.release(x, y) {
			s.click(a.e, t, ok)
		}
	case held:
		dx, dy := s.drag.move(x, y)
		f := s.treeFocal() / resDist // px per unit at the axis
		s.yaw -= dx / f
		s.yc += dy / (f * math.Cos(resPitch))
	}
	s.yaw -= turn * resSpinV
	s.yc += along * resScrollV
	if wy != 0 {
		s.focal = s.treeFocal() * math.Pow(1.1, wy)
	}
	s.clampView()
}

// Colours: the lines' light blue, from the captures.
var resLineColour = color.RGBA{110, 160, 240, 255}

// depthShade dims a bubble by its depth: 1 at the cylinder's near edge,
// 0.45 at its far edge.
func depthShade(depth float64) float32 {
	t := min(max((depth-(resDist-1))/2, 0), 1)
	return float32(1 - 0.55*t)
}

// drawTree paints the lines and bubbles back to front into sc clipped to
// resArea.
func (s *researchScreen) drawTree(a *app, sc *ebiten.Image) {
	r := a.e.Research
	dst := sc.SubImage(image.Rect(resArea.X0, resArea.Y0, resArea.X1, resArea.Y1)).(*ebiten.Image)
	bs := s.bubbles(r)
	at := make(map[game.Tech]bubble, len(bs))
	for _, b := range bs {
		at[b.t] = b
	}
	// The paint order by z: bubbles (q NoTech) and lines from b to q, a
	// line at its middle's z, so it passes over the farther end's bubble
	// and under the nearer's.
	type item struct {
		b     bubble
		q     game.Tech
		depth float64
	}
	var items []item
	for _, b := range bs {
		items = append(items, item{b, game.NoTech, b.z})
		for _, q := range game.Techs[b.t].Requires {
			if p, ok := at[q]; ok {
				items = append(items, item{b, q, (b.z + p.z) / 2})
			}
		}
	}
	slices.SortStableFunc(items, func(i, j item) int { return cmpFloat(j.depth, i.depth) })
	for _, it := range items {
		b := it.b
		if it.q != game.NoTech {
			// Between the rings' inner edges (an empty ring shows no line
			// inside); none between overlapping bubbles.
			p := at[it.q]
			dx, dy := p.p.X-b.p.X, p.p.Y-b.p.Y
			n := math.Hypot(dx, dy)
			r0, r1 := b.r*resRingIn, p.r*resRingIn
			if n <= r0+r1 {
				continue
			}
			ux, uy := dx/n, dy/n
			c := shade(resLineColour, fade((b.p.Depth+p.p.Depth)/2, resDist))
			vector.StrokeLine(dst, float32(b.p.X+ux*r0), float32(b.p.Y+uy*r0), float32(p.p.X-ux*r1), float32(p.p.Y-uy*r1), zoom, c, true)
			continue
		}
		f := depthShade(b.p.Depth)
		draw := func(sp *assets.Sprite, k float64) {
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(b.p.X, b.p.Y)
			op.ColorScale.Scale(f, f, f, 1)
			op.Filter = ebiten.FilterLinear
			sp.Draw(dst, &op)
		}
		switch {
		case r.Known[b.t]:
			draw(a.spr(resBank, int(b.t)), b.k)
		case r.Topic == b.t || r.Progress[b.t] > 0:
			grow := min(max(float64(r.Progress[b.t])/float64(r.Cost(b.t)), resGrowMin), 1)
			draw(a.spr(resBank, int(b.t)), b.k*grow)
		}
		ring := 1
		switch {
		case r.Topic == b.t:
			ring = 0
		case s.hover == b.t:
			ring = 2
		case s.sel == b.t:
			ring = 3
		case r.Known[b.t]: // the light blue ring greyed and tinted
			sp := a.spr("resring", 2)
			if sp.Image == nil {
				continue
			}
			var op colorm.DrawImageOptions
			op.GeoM.Translate(float64(sp.Offset.X), float64(sp.Offset.Y))
			op.GeoM.Scale(b.k, b.k)
			op.GeoM.Translate(b.p.X, b.p.Y)
			op.Filter = ebiten.FilterLinear
			colorm.DrawImage(dst, sp.Image, resKnownM(f), &op)
			continue
		}
		draw(a.spr("resring", ring), b.k)
	}
}

// resKnownTint is a known tech's ring colour (sea green, near
// knownColour): the ring's luma ×1.3 times these.
var resKnownTint = [3]float64{0.25, 1, 0.6}

// resKnownM greys a pixel (luma 0.3/0.59/0.11, ×1.3), tints it by
// resKnownTint and dims it by the depth's f.
func resKnownM(f float32) colorm.ColorM {
	var m colorm.ColorM
	for i, t := range resKnownTint {
		for j, w := range [3]float64{0.3, 0.59, 0.11} {
			m.SetElement(i, j, 1.3*w*t*float64(f))
		}
	}
	return m
}
