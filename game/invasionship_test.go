package game

import (
	"slices"
	"testing"
)

// InvasionShip: the best engine, the generators its draw needs, a lane
// drive when allowed, the modules; the smallest allowed hull holding
// them, none when no allowed hull does.
func TestInvasionShip(t *testing.T) {
	r := knowing(append(earlyShipTechs, Gizmos[InvasionModule].ResearchReq)...)
	if _, ok := InvasionShip(knowing(earlyShipTechs...), true, 2); ok {
		t.Error("an invasion ship without the module's tech")
	}
	if _, ok := InvasionShip(r, true, 0); ok {
		t.Error("an invasion ship of no module")
	}
	gens := func(engine, gen Gizmo) int {
		return max(1, (Gizmos[engine].Power+Gizmos[gen].Power-1)/Gizmos[gen].Power)
	}
	engine, _ := best(r, Drive)
	gen, _ := best(r, Generator)
	head := []Gizmo{engine}
	for range gens(engine, gen) {
		head = append(head, gen)
	}
	mods := func(n int) []Gizmo {
		out := []Gizmo{}
		for range n {
			out = append(out, InvasionModule)
		}
		return out
	}
	// No lane drive yet (asked for, not allowed): the head and 2 modules.
	d, ok := InvasionShip(r, true, 2)
	if want := append(slices.Clone(head), mods(2)...); !ok || d.Hull != Small || !slices.Equal(d.Parts, want) || d.Valid(r) != nil {
		t.Errorf("no lanes: %v %v, want Small %v (%v)", ok, d, want, d.Valid(r))
	}
	r.Grant(SpacetimeSurfing)
	lane := append(slices.Clone(head), StarLaneDrive)
	for _, tc := range []struct {
		modules int
		hull    Hull
		ok      bool
	}{
		{Small.Slots() - len(lane), Small, true},
		{Small.Slots() - len(lane) + 1, Medium, true},
		{Medium.Slots() - len(lane), Medium, true},
		{Medium.Slots() - len(lane) + 1, 0, false}, // Large not known
	} {
		d, ok := InvasionShip(r, true, tc.modules)
		want := append(slices.Clone(lane), mods(tc.modules)...)
		if ok != tc.ok || ok && (d.Hull != tc.hull || !slices.Equal(d.Parts, want) || d.Valid(r) != nil || d.InvasionModules() != tc.modules) {
			t.Errorf("%d modules: %v %v, want %v %s %v", tc.modules, ok, d, tc.ok, tc.hull, want)
		}
	}
	if d, ok := InvasionShip(r, false, 1); !ok || d.LaneDrives() != 0 || d.Name != "Invader" {
		t.Errorf("no lane drive wanted: %v %v", ok, d)
	}
}
