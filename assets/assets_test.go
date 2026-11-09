package assets

import (
	"image"
	"os"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

func openTestAssets(t *testing.T) *Assets {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// The planitem tile's canvas is 68x52 with its origin at (34,18) (the
// diamond's top vertex) and a box from (0,9): the offset is box − origin.
func TestSpriteOffset(t *testing.T) {
	a := openTestAssets(t)
	s, err := a.Sprite("planitem", 0)
	if err != nil {
		t.Fatal(err)
	}
	f := s.Frame
	if want := f.Box.Min.Sub(f.Center); s.Offset != want {
		t.Errorf("offset %v, box-origin %v", s.Offset, want)
	}
	if s.Offset != image.Pt(-34, -9) {
		t.Errorf("planitem[0] offset %v, want (-34,-9)", s.Offset)
	}
	if s.Image == nil {
		t.Error("planitem[0] has no image")
	}
	if again, _ := a.Sprite("data/PLANITEM.SHP", 0); again != s {
		t.Error("the same frame under another spelling is not cached")
	}
}

func TestBankNames(t *testing.T) {
	a := openTestAssets(t)
	for _, n := range []string{"17np", "17np.shp", "data/17np.shp"} {
		b, err := a.Bank(n)
		if err != nil {
			t.Fatal(err)
		}
		if len(b.Frames) != 9 {
			t.Errorf("%s: %d frames, want 9", n, len(b.Frames))
		}
	}
	if _, err := a.Sprite("17np", 9); err == nil {
		t.Error("frame 9 of 9 gave no error")
	}
}

// Graft places the source frame's pixels so its origin lands at at,
// remapped, on the canvas given; Sprite then serves it; a bank's own
// frame is refused.
func TestGraft(t *testing.T) {
	a := openTestAssets(t)
	gb, err := a.Bank("gizmos")
	if err != nil {
		t.Fatal(err)
	}
	n := len(gb.Frames)
	if a.Graft("gizmos", n-1, "planitem", 3, 70, 53, image.Pt(34, 36), image.Pt(35, 18), nil) == nil {
		t.Error("grafted over the bank's own frame")
	}
	sb, err := a.Bank("planitem")
	if err != nil {
		t.Fatal(err)
	}
	src := sb.Frames[3]
	from, to := src.Index[0], src.Index[0]+1 // a remapped index
	for k, op := range src.Opaque {
		if op {
			from, to = src.Index[k], src.Index[k]+1
			break
		}
	}
	at := image.Pt(35, 18)
	if err := a.Graft("gizmos", n, "planitem", 3, 70, 53, image.Pt(34, 36), at, map[uint8]uint8{from: to}); err != nil {
		t.Fatal(err)
	}
	s, err := a.Sprite("gizmos", n)
	if err != nil {
		t.Fatal(err)
	}
	f := s.Frame
	if f.Width != 70 || f.Height != 53 || f.Center != image.Pt(34, 36) || s.Offset != f.Box.Min.Sub(f.Center) || s.Image == nil {
		t.Fatalf("grafted frame %dx%d origin %v box %v offset %v", f.Width, f.Height, f.Center, f.Box, s.Offset)
	}
	shift := at.Sub(src.Center)
	if f.Box != src.Box.Add(shift).Intersect(image.Rect(0, 0, 70, 53)) {
		t.Errorf("box %v, source %v shifted %v", f.Box, src.Box, shift)
	}
	seen := 0
	for y := f.Box.Min.Y; y < f.Box.Max.Y; y++ {
		for x := f.Box.Min.X; x < f.Box.Max.X; x++ {
			d := (y-f.Box.Min.Y)*f.Box.Dx() + x - f.Box.Min.X
			p := image.Pt(x, y).Sub(shift).Sub(src.Box.Min)
			k := p.Y*src.Box.Dx() + p.X
			want := src.Index[k]
			if want == from {
				want = to
			}
			if f.Opaque[d] != src.Opaque[k] || f.Opaque[d] && f.Index[d] != want {
				t.Fatalf("(%d,%d): %v %d, source %v %d", x, y, f.Opaque[d], f.Index[d], src.Opaque[k], src.Index[k])
			}
			if f.Opaque[d] {
				seen++
			}
		}
	}
	if seen == 0 {
		t.Error("nothing grafted")
	}
}
