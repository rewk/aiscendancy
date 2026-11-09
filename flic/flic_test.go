package flic

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

const intro = "data/intro65a.flc"

func readIntro(t *testing.T) []byte {
	t.Helper()
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	b, err := g.Read(intro)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The introduction decodes whole: 1143 frames of 640x480, then io.EOF;
// every frame matches ffmpeg's own FLIC decoder (rgb24), when ffmpeg is
// installed.
func TestIntro(t *testing.T) {
	b := readIntro(t)
	m, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.W != 640 || m.H != 480 || m.Frames != 1143 {
		t.Fatalf("%dx%d, %d frames", m.W, m.H, m.Frames)
	}
	var oracle io.Reader
	if ff, err := exec.LookPath("ffmpeg"); err == nil {
		cmd := exec.Command(ff, "-v", "error", "-f", "flic", "-i", "pipe:0", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
		cmd.Stdin = bytes.NewReader(b)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("ffmpeg: %v", err)
		}
		oracle = bytes.NewReader(out)
	} else {
		t.Log("no ffmpeg: decoding unchecked")
	}
	want := make([]byte, 3*m.W*m.H)
	for f := range m.Frames {
		if err := m.Next(); err != nil {
			t.Fatalf("frame %d: %v", f, err)
		}
		if oracle == nil {
			continue
		}
		if _, err := io.ReadFull(oracle, want); err != nil {
			t.Fatalf("ffmpeg frame %d: %v", f, err)
		}
		for i, ix := range m.Pix {
			r, g, b, _ := m.Palette[ix].RGBA()
			if uint8(r>>8) != want[3*i] || uint8(g>>8) != want[3*i+1] || uint8(b>>8) != want[3*i+2] {
				t.Fatalf("frame %d pixel (%d,%d): index %d %v, ffmpeg %v", f, i%m.W, i/m.W, ix, m.Palette[ix], want[3*i:3*i+3])
			}
		}
	}
	if err := m.Next(); err != io.EOF {
		t.Errorf("past the last frame: %v", err)
	}
}

// A header of no area or past maxSide is refused before any allocation.
func TestDecodeSize(t *testing.T) {
	for _, wh := range [][2]uint16{{0, 480}, {640, 0}, {maxSide + 1, 480}, {640, 65535}} {
		b := make([]byte, headerLen)
		le := binary.LittleEndian
		le.PutUint16(b[4:], magicFLC)
		le.PutUint16(b[8:], wh[0])
		le.PutUint16(b[10:], wh[1])
		le.PutUint16(b[12:], 8)
		if _, err := Decode(b); err == nil {
			t.Errorf("%dx%d accepted", wh[0], wh[1])
		}
	}
}
