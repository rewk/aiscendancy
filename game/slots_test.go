package game

import (
	"slices"
	"testing"
)

func fixedParts(gs ...Gizmo) []Part {
	out := make([]Part, len(gs))
	for i, g := range gs {
		out[i] = Part{Gizmo: g, Cat: Gizmos[g].Cat}
	}
	return out
}

// Place inserts in slot order or replaces, Unplace leaves a hole, FitHull
// drops the parts past the hull's slots; the input is never mutated; a
// packed template (Slots nil) reads as slots 0..n-1.
func TestTemplateSlots(t *testing.T) {
	t0 := Template{Hull: Medium, Parts: fixedParts(TonklinMotor, ProtonShaver)}
	if i, ok := t0.PartAt(1); !ok || i != 1 || t0.SlotOf(0) != 0 {
		t.Fatalf("packed: PartAt(1) %d %v", i, ok)
	}
	if _, ok := t0.PartAt(2); ok {
		t.Error("packed: slot 2 full")
	}
	t1 := t0.Place(fixedParts(MassBarrageGun)[0], 6)
	t1 = t1.Place(fixedParts(IonWrap)[0], 4)
	if !slices.Equal(t1.Slots, []int{0, 1, 4, 6}) || t1.Parts[2].Gizmo != IonWrap || t1.Parts[3].Gizmo != MassBarrageGun {
		t.Fatalf("placed: slots %v parts %v", t1.Slots, t1.Parts)
	}
	if t0.Slots != nil || len(t0.Parts) != 2 {
		t.Errorf("Place mutated its input: %v %v", t0.Slots, t0.Parts)
	}
	t2 := t1.Place(fixedParts(FourierMissiles)[0], 4)
	if len(t2.Parts) != 4 || t2.Parts[2].Gizmo != FourierMissiles || t1.Parts[2].Gizmo != IonWrap {
		t.Errorf("replace: %v (input %v)", t2.Parts, t1.Parts)
	}
	t3 := t2.Unplace(1)
	if !slices.Equal(t3.Slots, []int{0, 4, 6}) || len(t2.Slots) != 4 {
		t.Errorf("unplace: slots %v (input %v)", t3.Slots, t2.Slots)
	}
	if t4 := t3.Unplace(2); !slices.Equal(t4.Slots, t3.Slots) {
		t.Errorf("unplace of a hole: %v", t4.Slots)
	}
	t5 := t3.FitHull(Small) // 5 slots
	if t5.Hull != Small || !slices.Equal(t5.Slots, []int{0, 4}) || len(t5.Parts) != 2 {
		t.Errorf("fit Small: %v %v", t5.Slots, t5.Parts)
	}
}

// Valid and AddTemplate refuse slots out of step with the parts; Realise
// keeps a slot with its part and drops it with an unrealised Best part.
func TestSlotsChecked(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	r := e.Research
	for tc := range NumTechs {
		r.Grant(tc)
	}
	d := Design{Name: "x", Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver}}
	for _, bad := range [][]int{{0}, {1, 1}, {2, 1}, {0, 5}, {-1, 0}} {
		d.Slots = bad
		if err := d.Valid(r); err == nil {
			t.Errorf("design slots %v accepted", bad)
		}
		tp := d.Fixed()
		if err := e.AddTemplate(tp); err == nil {
			t.Errorf("template slots %v accepted", bad)
		}
	}
	d.Slots = []int{1, 4}
	if err := d.Valid(r); err != nil {
		t.Errorf("slots 1, 4: %v", err)
	}
	tp := Template{Hull: Small, Parts: []Part{{Best: true, Cat: Weapon}, {Gizmo: TonklinMotor, Cat: Drive}}, Slots: []int{0, 3}}
	if got, ok := tp.Realise(NewGame(1, []Species{Snovemdomas}, Average).Empires[0].Research); ok || !slices.Equal(got.Slots, []int{3}) || !slices.Equal(got.Parts, []Gizmo{TonklinMotor}) {
		t.Errorf("unrealised Best: %v %v %v", got.Parts, got.Slots, ok)
	}
}

// A ship keeps its design's slots; Aboard drops a spent Colonizer's slot
// with it; LosePart drops the lost part's, the design untouched.
func TestShipSlots(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Colonizer, MassBarrageGun}, Slots: []int{0, 1, 3, 4}}
	_, e, s := specialShip(t, d)
	if !slices.Equal(s.Design.Slots, d.Slots) {
		t.Fatalf("spawned slots %v", s.Design.Slots)
	}
	s.Cargo = 0
	if ab := s.Aboard(); !slices.Equal(ab.Slots, []int{0, 1, 4}) || len(ab.Parts) != 3 {
		t.Errorf("aboard: %v %v", ab.Parts, ab.Slots)
	}
	sister := e.Spawn(d, s.Star, 0)
	s.LosePart(1)
	if !slices.Equal(s.Design.Slots, []int{0, 3, 4}) || !slices.Equal(sister.Design.Slots, d.Slots) || !slices.Equal(d.Slots, []int{0, 1, 3, 4}) {
		t.Errorf("lost part: %v, sister %v, design %v", s.Design.Slots, sister.Design.Slots, d.Slots)
	}
}
