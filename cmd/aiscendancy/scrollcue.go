package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The scroll cue: a blinking yellow triangle in an overflowing list's
// right margin (padX, kept free by every list), up at the top when rows
// are hidden above, down at the bottom when hidden below.
const (
	cueW, cueH = 10, 8 // the triangle's base and height, native px
	cueInset   = 2     // from the box's edges
	cueBlink   = 30    // ticks on, then as many off
)

// cueBox is the cue's box in r: right-aligned, cueInset from the top
// edge, or the bottom when down.
func cueBox(r rect, down bool) rect {
	x1 := r.X1 - cueInset
	y0 := r.Y0 + cueInset
	if down {
		y0 = r.Y1 - cueInset - cueH
	}
	return rect{x1 - cueW, y0, x1, y0 + cueH}
}

// cueOn is the blink's phase at tick t.
func cueOn(t int) bool { return t/cueBlink%2 == 0 }

// cueShown is whether each cue shows for n rows with [off, off+shown)
// in view.
func cueShown(off, shown, n int) (up, down bool) {
	return off > 0, off+shown < n
}

// drawScrollCues paints such a list's cues in r, in the on phase.
func drawScrollCues(a *app, dst *ebiten.Image, r rect, off, shown, n int) {
	if !cueOn(a.tick) {
		return
	}
	up, down := cueShown(off, shown, n)
	if up {
		fillTriangle(dst, cueBox(r, false), false)
	}
	if down {
		fillTriangle(dst, cueBox(r, true), true)
	}
}

// fillTriangle fills b with a yellow triangle, apex up (down flips it).
func fillTriangle(dst *ebiten.Image, b rect, down bool) {
	x0, x1, xm := float32(b.X0), float32(b.X1), float32(b.X0+b.X1)/2
	top, bot := float32(b.Y0), float32(b.Y1)
	var p vector.Path
	if down {
		p.MoveTo(x0, top)
		p.LineTo(x1, top)
		p.LineTo(xm, bot)
	} else {
		p.MoveTo(xm, top)
		p.LineTo(x1, bot)
		p.LineTo(x0, bot)
	}
	p.Close()
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(yellow)
	vector.FillPath(dst, &p, &vector.FillOptions{}, op)
}
