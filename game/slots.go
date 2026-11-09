package game

import (
	"fmt"
	"slices"
)

// A design's parts sit in hull slots. Slots (Template's and Design's),
// when set, is each part's slot, parallel to Parts and strictly
// increasing: a part placed by hand keeps its slot and a removed one
// leaves a hole. nil: the parts fill the slots
// from 0 in order, the bots' and the fixtures'. No rule reads a slot;
// Parts stays in slot order, so everything reading Parts is unchanged.

// slotOf is part i's slot under slots (nil: packed).
func slotOf(slots []int, i int) int {
	if slots == nil {
		return i
	}
	return slots[i]
}

// checkSlots reports slots out of step with n parts on hull h.
func checkSlots(slots []int, n int, h Hull) error {
	if slots == nil {
		return nil
	}
	if len(slots) != n {
		return fmt.Errorf("%d slots for %d parts", len(slots), n)
	}
	for i, k := range slots {
		if k < 0 || k >= h.Slots() || i > 0 && k <= slots[i-1] {
			return fmt.Errorf("slot %d at part %d: not increasing within the %s hull's %d", k, i, h, h.Slots())
		}
	}
	return nil
}

// SlotOf is part i's slot.
func (t Template) SlotOf(i int) int { return slotOf(t.Slots, i) }

// PartAt is the index in Parts of the part in slot k; false: empty.
func (t Template) PartAt(k int) (int, bool) {
	for i := range t.Parts {
		if s := t.SlotOf(i); s == k {
			return i, true
		} else if s > k {
			break
		}
	}
	return 0, false
}

// explicit is t with Slots set (packed ones spelled out), a fresh slice.
func (t Template) explicit() Template {
	s := make([]int, len(t.Parts))
	for i := range s {
		s[i] = t.SlotOf(i)
	}
	t.Slots = s
	return t
}

// Place puts p in slot k (in the hull), replacing the part there; t's
// slices are fresh.
func (t Template) Place(p Part, k int) Template {
	t = t.explicit()
	t.Parts = slices.Clone(t.Parts)
	if i, ok := t.PartAt(k); ok {
		t.Parts[i] = p
		return t
	}
	i, _ := slices.BinarySearch(t.Slots, k)
	t.Parts, t.Slots = slices.Insert(t.Parts, i, p), slices.Insert(t.Slots, i, k)
	return t
}

// Unplace empties slot k, the others staying; t's slices are fresh.
func (t Template) Unplace(k int) Template {
	i, ok := t.PartAt(k)
	if !ok {
		return t
	}
	t = t.explicit()
	t.Parts, t.Slots = slices.Delete(slices.Clone(t.Parts), i, i+1), slices.Delete(t.Slots, i, i+1)
	return t
}

// FitHull puts t on hull h, dropping the parts in slots past h's; t's
// slices are fresh.
func (t Template) FitHull(h Hull) Template {
	t = t.explicit()
	n := 0
	for n < len(t.Slots) && t.Slots[n] < h.Slots() {
		n++
	}
	t.Hull, t.Parts, t.Slots = h, slices.Clone(t.Parts[:n]), t.Slots[:n]
	return t
}
