package main

import "testing"

// The cues sit inside their box, in the right margin (no content there:
// the lists end at X1 − padX), up at the top and down at the bottom.
func TestCueBox(t *testing.T) {
	for _, r := range []rect{mapArea, fleetBox(), speciesRowsBox()} {
		for _, down := range []bool{false, true} {
			b := cueBox(r, down)
			if b.X0 < r.X1-padX || b.X1 > r.X1 || b.Y0 < r.Y0 || b.Y1 > r.Y1 {
				t.Errorf("cue %v in %v: %v", down, r, b)
			}
			if b.X1-b.X0 != cueW || b.Y1-b.Y0 != cueH {
				t.Errorf("cue %v: size %v", down, b)
			}
			if down && b.Y1 != r.Y1-cueInset || !down && b.Y0 != r.Y0+cueInset {
				t.Errorf("cue %v: not at its edge: %v in %v", down, b, r)
			}
		}
	}
}

// cueShown: up only with rows above, down only with rows below; cueOn
// alternates every cueBlink ticks.
func TestCueShown(t *testing.T) {
	for _, c := range []struct {
		off, shown, n int
		up, down      bool
	}{
		{0, 5, 3, false, false}, {0, 5, 5, false, false}, {0, 5, 6, false, true},
		{1, 5, 6, true, false}, {1, 5, 7, true, true},
	} {
		if up, down := cueShown(c.off, c.shown, c.n); up != c.up || down != c.down {
			t.Errorf("cueShown(%d, %d, %d) = %v, %v; want %v, %v", c.off, c.shown, c.n, up, down, c.up, c.down)
		}
	}
	if !cueOn(0) || !cueOn(cueBlink-1) || cueOn(cueBlink) || cueOn(2*cueBlink-1) || !cueOn(2*cueBlink) {
		t.Error("cueOn: the blink is not cueBlink on, cueBlink off")
	}
}
