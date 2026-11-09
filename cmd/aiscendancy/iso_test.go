package main

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// Every cell's diamond centre and the four points just inside its vertices
// map back to that cell, for every size; a point far away maps to none.
func TestCellAtRoundTrip(t *testing.T) {
	for size := game.MinSize; size <= game.MaxSize; size++ {
		g := newGrid(size)
		for _, c := range game.Layout(size) {
			cx, cy := g.Centre(c)
			for _, d := range [][2]float64{{0, 0}, {halfW - 1, 0}, {-halfW + 1, 0}, {0, halfH - 1}, {0, -halfH + 1}} {
				x, y := cx+d[0], cy+d[1]
				if !inDiamond(x, y, cx, cy) {
					t.Fatalf("(%v,%v) not in its own diamond", x, y)
				}
				got, ok := g.CellAt(x, y)
				if !ok || got != c {
					t.Errorf("size %d: %v%+v -> %v %v", size, c, d, got, ok)
				}
			}
		}
		if c, ok := g.CellAt(-1000, -1000); ok {
			t.Errorf("size %d: far point hit %v", size, c)
		}
	}
}

// Slots round-trip the same way (the centre and the four corners just
// inside the cell) and do not overlap one another.
func TestSlotAt(t *testing.T) {
	for k := 0; k < game.OrbitSlots; k++ {
		cx, cy := slotCentre(k)
		for _, d := range [][2]float64{{0, 0}, {slotHalf - 1, slotHalf - 1}, {-slotHalf + 1, slotHalf - 1}, {slotHalf - 1, -slotHalf + 1}, {-slotHalf + 1, -slotHalf + 1}} {
			if got, ok := SlotAt(cx+d[0], cy+d[1]); !ok || got != k {
				t.Errorf("slot %d%+v -> %d %v", k, d, got, ok)
			}
		}
	}
	if k, ok := SlotAt(zoom*600, zoom*300); ok {
		t.Errorf("grid side hit slot %d", k)
	}
}

// The grid and the orbit column stay inside the big area: every diamond of
// the largest planet and every slot's cell.
func TestInsideArea(t *testing.T) {
	g := newGrid(game.MaxSize)
	for _, c := range game.Layout(game.MaxSize) {
		x, y := g.Centre(c)
		if x-halfW < float64(planetArea.X0) || x+halfW > float64(planetArea.X1) || y-halfH < float64(planetArea.Y0) || y+halfH > float64(planetArea.Y1) {
			t.Errorf("cell %v at (%v,%v) leaves the planetArea", c, x, y)
		}
	}
	for k := 0; k < game.OrbitSlots; k++ {
		if x, _ := slotCentre(k); x+slotHalf > float64(planetArea.X0+(planetArea.X1-planetArea.X0)/3) {
			t.Errorf("slot %d reaches past the orbit column", k)
		}
	}
}

// Every orbit slot's cell lies inside the big area, so SlotAt never
// answers over the chrome's bottom border.
func TestSlotsInsideArea(t *testing.T) {
	for k := 0; k < game.OrbitSlots; k++ {
		x, y := slotCentre(k)
		if x-slotHalf < float64(planetArea.X0) || y-slotHalf < float64(planetArea.Y0) || y+slotHalf > float64(planetArea.Y1) {
			t.Errorf("slot %d at (%v,%v) leaves the planetArea", k, x, y)
		}
	}
	if k, ok := SlotAt(float64(planetArea.X0+zoom*115), float64(planetArea.Y1+zoom*2)); ok {
		t.Errorf("the chrome's bottom border hits slot %d", k)
	}
}
