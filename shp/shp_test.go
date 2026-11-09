package shp

import (
	"image"
	"image/color"
	"os"
	"strings"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

func openTestGame(t *testing.T) *cob.Game {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	return g
}

// Every .shp and .tmp of the game decodes, and every non-empty frame's RLE
// stream yields exactly its box height in rows and no more: the grammar in
// the package comment is the whole format.
func TestEveryBankDecodes(t *testing.T) {
	g := openTestGame(t)
	banks, frames, empty := 0, 0, 0
	for _, name := range g.Names("") {
		if !strings.HasSuffix(name, ".shp") && !strings.HasSuffix(name, ".tmp") {
			continue
		}
		b, err := g.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		bank, err := Decode(b)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		banks++
		for i, f := range bank.Frames {
			frames++
			if f.Empty() {
				empty++
				continue
			}
			if !f.Box.In(image.Rect(0, 0, f.Width, f.Height)) {
				t.Errorf("%s[%d]: box %v outside canvas %dx%d", name, i, f.Box, f.Width, f.Height)
			}
		}
	}
	t.Logf("%d banks, %d frames, %d empty", banks, frames, empty)
	if banks < 200 || frames < 1500 {
		t.Errorf("too few: %d banks, %d frames", banks, frames)
	}
}

func TestKnownFrames(t *testing.T) {
	g := openTestGame(t)
	b, err := g.Read("data/bullet04.shp")
	if err != nil {
		t.Fatal(err)
	}
	bank, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(bank.Frames) != 4 {
		t.Fatalf("bullet04: %d frames, want 4", len(bank.Frames))
	}
	f := bank.Frames[0]
	if f.Width != 40 || f.Height != 40 || f.Center != image.Pt(20, 20) {
		t.Errorf("bullet04[0]: canvas %dx%d center %v", f.Width, f.Height, f.Center)
	}
	// Header box (-10,-6)-(10,11) inclusive, around (20,20).
	if want := image.Rect(10, 14, 31, 32); f.Box != want {
		t.Errorf("bullet04[0]: box %v, want %v", f.Box, want)
	}
	// First row of the stream: 01 05 | 03 b1 | 01 01 | 0b c6 c3 03 30 05 | 01 01 | 03 c6 | 00
	wantRow := []int{-1, -1, -1, -1, -1, 0xb1, -1, 0xc6, 0xc3, 0x03, 0x30, 0x05, -1, 0xc6}
	for x, v := range wantRow {
		if v < 0 {
			if f.Opaque[x] {
				t.Errorf("row 0 x=%d: want transparent", x)
			}
		} else if !f.Opaque[x] || int(f.Index[x]) != v {
			t.Errorf("row 0 x=%d: got %v/%#x, want %#x", x, f.Opaque[x], f.Index[x], v)
		}
	}
	for x := len(wantRow); x < f.Box.Dx(); x++ {
		if f.Opaque[x] {
			t.Errorf("row 0 x=%d: want transparent tail", x)
		}
	}

	b, _ = g.Read("data/lgrace00.shp")
	bank, err = Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	f = bank.Frames[0]
	if f.Width != 297 || f.Height != 200 || f.Box != image.Rect(0, 0, 297, 200) {
		t.Errorf("lgrace00: canvas %dx%d box %v", f.Width, f.Height, f.Box)
	}
	pal, err := Palette(must(g.Read("data/game.pal")))
	if err != nil {
		t.Fatal(err)
	}
	img := f.RGBA(pal)
	if img.Bounds() != image.Rect(0, 0, 297, 200) {
		t.Errorf("rgba bounds %v", img.Bounds())
	}
}

func TestOriginOrder(t *testing.T) {
	g := openTestGame(t)
	bank, err := Decode(must(g.Read("data/9endgame.shp")))
	if err != nil {
		t.Fatal(err)
	}
	f := bank.Frames[9]
	if f.Width != 286 || f.Height != 80 || f.Center != image.Pt(143, 40) {
		t.Errorf("9endgame[9]: canvas %dx%d origin %v", f.Width, f.Height, f.Center)
	}
	if want := image.Rect(0, 0, 286, 80); f.Box != want {
		t.Errorf("9endgame[9]: box %v, want %v", f.Box, want)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

func rgba(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

func TestPaletteScale(t *testing.T) {
	b := make([]byte, 768)
	b[0], b[1], b[2] = 63, 32, 0
	pal, err := Palette(b)
	if err != nil {
		t.Fatal(err)
	}
	if c := pal[0]; c != rgba(255, 130, 0) {
		t.Errorf("got %v", c)
	}
}

// A frame whose slice (up to the next frame's offset) is shorter than its
// 24-byte header is an error, not a panic: here two frames share offset 24
// and the first one's slice is empty.
func TestShortFrameIsAnError(t *testing.T) {
	b := make([]byte, 24+24)
	copy(b, "1.10")
	b[4] = 2  // two frames
	b[8] = 24 // frame 0 at 24
	b[16] = 24
	if _, err := Decode(b); err == nil {
		t.Error("no error")
	}
}
