package game

import (
	"slices"
	"testing"
)

// HabitatShipOn: the best engine, the generators its draw needs, a lane
// drive when wanted and allowed, Space Habitats in every slot left; none
// without the Habitat's tech, on an unknown hull, or with no slot left.
func TestHabitatShipOn(t *testing.T) {
	if _, ok := HabitatShipOn(knowing(earlyShipTechs...), Small, true); ok {
		t.Error("a habitat ship without Planetary Replenishment")
	}
	r := knowing(append(earlyShipTechs, PlanetaryReplenishment)...)
	if _, ok := HabitatShipOn(r, Large, false); ok {
		t.Error("a habitat ship on an unknown hull")
	}
	engine, _ := best(r, Drive)
	gen, _ := best(r, Generator)
	head := powered(engine, gen, Gizmos[engine].Power, nil)
	fill := func(head []Gizmo, h Hull) []Gizmo {
		out := slices.Clone(head)
		for len(out) < h.Slots() {
			out = append(out, SpaceHabitat)
		}
		return out
	}
	check := func(what string, lane bool, h Hull, head []Gizmo) {
		t.Helper()
		d, ok := HabitatShipOn(r, h, lane)
		want := fill(head, h)
		if !ok || d.Name != HabitatName || d.Hull != h || !slices.Equal(d.Parts, want) || d.Valid(r) != nil {
			t.Errorf("%s: %v %v, want %s %v (%v)", what, ok, d, h, want, d.Valid(r))
		}
		if n := len(want) - len(head); d.Berths() != n*Gizmos[SpaceHabitat].Special1 {
			t.Errorf("%s: %d berths, want %d habitats' worth", what, d.Berths(), n)
		}
	}
	check("no lanes yet", true, Small, head)
	check("Medium", false, Medium, head)
	r.Grant(SpacetimeSurfing)
	check("with lanes", true, Small, append(slices.Clone(head), StarLaneDrive))
	check("lane drive not wanted", false, Small, head)
	if d, ok := HabitatShipOn(r, Small, true); !ok || d.LaneDrives() != 1 {
		t.Errorf("lane drive: %v %v", ok, d)
	}
}
