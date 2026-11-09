package main

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// The planetary display's chrome, measured on 17np frame 0: each rect is
// the largest solid run of one colour (black for panels 1-3, (12,32,48)
// for 4-5, transparent for the big area) with a clean 1 px ring. Panels
// 1-3 match planind/planres/planpro's 108x100 canvas, the big area 17np
// frame 4 (626x358).
var (
	panels = [5]rect{
		orig(7, 7, 115, 107),
		orig(123, 7, 231, 107),
		orig(239, 7, 347, 107),
		orig(355, 7, 455, 107),
		orig(463, 7, 633, 107),
	}
	planetArea = orig(7, 115, 633, 473)
)

// A planitem tile is 68x52 original px, the square's diamond in its
// bottom 34 rows, origin at the diamond's top vertex. Native px.
const (
	halfW = 34 * zoom // diamond half-width
	halfH = 17 * zoom // diamond half-height
)

// gridCentre is G, cell (5,5)'s diamond centre: orig (410,297), measured
// on DOSBox captures (34x17 steps centred there). Native px.
func gridCentre() (float64, float64) { return zoom * 410, zoom * 297 }

// grid maps a planet's cells to the screen and back.
type grid struct {
	gx, gy float64 // G
	cells  map[game.Cell]bool
}

func newGrid(size int) *grid {
	g := &grid{cells: map[game.Cell]bool{}}
	g.gx, g.gy = gridCentre()
	for _, c := range game.Layout(size) {
		g.cells[c] = true
	}
	return g
}

// Centre is the screen centre of cell c's diamond: x grows with i-j, y with
// i+j, cell (5,5) at G.
func (g *grid) Centre(c game.Cell) (float64, float64) {
	return g.gx + float64((c.I-c.J)*halfW), g.gy + float64((c.I+c.J-2*game.Centre)*halfH)
}

// CellAt inverts Centre: with u = dx/halfW, v = dy/halfH from G, i =
// 5+(u+v)/2, j = 5+(v-u)/2; the diamond |u|+|v| <= 1 is the unit square
// around the cell, so rounding both picks it.
func (g *grid) CellAt(x, y float64) (game.Cell, bool) {
	u, v := (x-g.gx)/halfW, (y-g.gy)/halfH
	c := game.Cell{
		I: int(math.Floor(game.Centre + (u+v)/2 + 0.5)),
		J: int(math.Floor(game.Centre + (v-u)/2 + 0.5)),
	}
	return c, g.cells[c]
}

// The ten orbit slots: a flat 2x5 grid of 70x70 cells at the big area's
// left, origins orig (14+70c, 119+70r), measured on a full colony's
// capture (planitem frames 22-30 are 70x70 centred at (35,35)). Slot k is
// column k%2, row k/2 (row-major assumed). Native px.
const (
	slotSize = 70 * zoom
	slotHalf = slotSize / 2
	slotX0   = 14 * zoom
	slotY0   = 119 * zoom
)

// slotCentre is orbit slot k's cell centre.
func slotCentre(k int) (float64, float64) {
	return float64(slotX0 + slotHalf + slotSize*(k%2)), float64(slotY0 + slotHalf + slotSize*(k/2))
}

// inSlotCell reports whether (x,y) lies in the slotSize cell centred on (cx,cy).
func inSlotCell(x, y, cx, cy float64) bool {
	return math.Abs(x-cx) < slotHalf && math.Abs(y-cy) < slotHalf
}

// inDiamond reports whether (x,y) lies in the halfW x halfH diamond centred on (cx,cy).
func inDiamond(x, y, cx, cy float64) bool {
	return math.Abs(x-cx)/halfW+math.Abs(y-cy)/halfH <= 1
}

// SlotAt finds the orbit slot whose cell holds (x,y).
func SlotAt(x, y float64) (int, bool) {
	for k := 0; k < game.OrbitSlots; k++ {
		if cx, cy := slotCentre(k); inSlotCell(x, y, cx, cy) {
			return k, true
		}
	}
	return 0, false
}
