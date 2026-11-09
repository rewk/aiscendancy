package sound

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"

	"github.com/rewk/aiscendancy/cob"
)

func openTestLibrary(t *testing.T) *Library {
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
	l, err := OpenLibrary(g)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// music.txt lists 29 tunes: the five themes, victory, the two intros,
// one stinger per species; every one is in an archive.
func TestMusicList(t *testing.T) {
	l := openTestLibrary(t)
	want := []string{"theme00", "theme01", "theme02", "theme03", "theme04", "victory", "intro00", "intro01"}
	for i := range 21 {
		want = append(want, "race"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	got := l.Names()
	if len(got) != len(want) {
		t.Fatalf("%d tunes %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tune %d = %q, want %q", i, got[i], want[i])
		}
		if _, err := l.g.Read(l.paths[got[i]]); err != nil {
			t.Error(err)
		}
	}
}

func TestParseMusicCount(t *testing.T) {
	if p, err := parseMusic("2\r\ndata\\A.raw\r\ndata\\b.raw\r\n\r\n\x1a"); err != nil || len(p) != 2 || p[0] != "data/a.raw" {
		t.Errorf("got %v, %v", p, err)
	}
	if _, err := parseMusic("3\r\ndata\\a.raw\r\n"); err == nil {
		t.Error("a count over the paths accepted")
	}
}

// frames decodes s16le stereo into (left, right) pairs.
func frames(t *testing.T, b []byte) [][2]int16 {
	t.Helper()
	if len(b)%FrameBytes != 0 {
		t.Fatalf("%d bytes, not whole frames", len(b))
	}
	out := make([][2]int16, len(b)/FrameBytes)
	for i := range out {
		out[i][0] = int16(binary.LittleEndian.Uint16(b[4*i:]))
		out[i][1] = int16(binary.LittleEndian.Uint16(b[4*i+2:]))
	}
	return out
}

// u8 mono becomes (b−128)<<8 on both channels.
func TestConvert(t *testing.T) {
	raw := []byte{0, 1, 127, 128, 129, 200, 255}
	b, err := io.ReadAll(NewTrack(raw))
	if err != nil {
		t.Fatal(err)
	}
	f := frames(t, b)
	if len(f) != len(raw) {
		t.Fatalf("%d frames for %d samples", len(f), len(raw))
	}
	for i, r := range raw {
		want := int16(int(r)-128) * 256
		if f[i][0] != want || f[i][1] != want {
			t.Errorf("sample %d (%d): %v, want %d both", i, r, f[i], want)
		}
	}
	if f[0][0] != -32768 || f[6][0] != 32512 {
		t.Errorf("extremes %d %d", f[0][0], f[6][0])
	}
}

// synth is loud samples, a near-silence of n samples (values wobbling
// about 128), loud samples; the loud ones count up so each is
// recognisable.
func synth(head, n, tail int) []byte {
	var b []byte
	for i := range head {
		b = append(b, byte(10+i%50))
	}
	for i := range n {
		b = append(b, byte(126+i%5))
	}
	for i := range tail {
		b = append(b, byte(200+i%50))
	}
	return b
}

// A 10 s silence plays whole (no squeezing), every
// sample in place.
func TestSilencePlays(t *testing.T) {
	const head, tail = 100, 100
	raw := synth(head, 10*SampleRate, tail)
	tr := NewTrack(raw)
	if tr.Len() != int64(len(raw))*FrameBytes {
		t.Fatalf("Len %d for %d samples", tr.Len(), len(raw))
	}
	b, _ := io.ReadAll(tr)
	f := frames(t, b)
	if len(f) != len(raw) {
		t.Fatalf("%d frames, want %d", len(f), len(raw))
	}
	for i, r := range raw {
		if want := int16(int(r)-128) * 256; f[i][0] != want {
			t.Fatalf("frame %d = %d, want raw's %d", i, f[i][0], want)
		}
	}
}

// Seek then read is reading from the start and dropping the bytes
// before, at every offset class (mid-frame, the end) and through Read
// calls of awkward sizes.
func TestSeek(t *testing.T) {
	raw := append(synth(50, 4*SampleRate, 30), synth(20, 5*SampleRate, 10)...)
	all, _ := io.ReadAll(NewTrack(raw))
	tr := NewTrack(raw)
	if int64(len(all)) != tr.Len() {
		t.Fatalf("read %d bytes, Len %d", len(all), tr.Len())
	}
	for _, off := range []int64{0, 1, 3, 4, 199, 200, 201, FrameBytes * 49, FrameBytes*50 + 2, tr.Len() - 5, tr.Len() - 1} {
		if _, err := tr.Seek(off, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		var got []byte
		buf := make([]byte, 7) // odd: ends mid-frame
		for {
			n, err := tr.Read(buf)
			got = append(got, buf[:n]...)
			if err == io.EOF {
				break
			}
		}
		if !bytes.Equal(got, all[off:]) {
			t.Errorf("seek %d: %d bytes differ from the tail of a full read (%d)", off, len(got), len(all)-int(off))
		}
	}
	if p, _ := tr.Seek(-4, io.SeekEnd); p != tr.Len()-4 {
		t.Errorf("SeekEnd -4 = %d", p)
	}
	if _, err := tr.Seek(-1, io.SeekStart); err == nil {
		t.Error("negative seek accepted")
	}
}

// theme00: four pieces ~30 s apart and a silent tail (|b−128| ≤ 2), all
// played: at least three runs over 20 s (~32 s measured), the last run
// ending the file, and Len the whole file.
func TestTheme00(t *testing.T) {
	l := openTestLibrary(t)
	tr, err := l.Track("theme00")
	if err != nil {
		t.Fatal(err)
	}
	silent := func(b byte) bool { return b >= 126 && b <= 130 }
	var runs []int // silent runs over 3 s, their lengths
	end := 0       // where the last such run ends
	for i := 0; i < len(tr.raw); i++ {
		j := i
		for j < len(tr.raw) && silent(tr.raw[j]) {
			j++
		}
		if j-i > 3*SampleRate {
			runs, end = append(runs, j-i), j
		}
		i = j
	}
	long := 0
	for _, r := range runs {
		if r > 20*SampleRate {
			long++
		}
	}
	if tr.Len() != int64(len(tr.raw))*FrameBytes {
		t.Errorf("Len %d for %d raw samples", tr.Len(), len(tr.raw))
	}
	if long < 3 || len(runs) < 4 || end != len(tr.raw) {
		t.Errorf("%d runs, %d over 20 s, the last ending at %d of %d", len(runs), long, end, len(tr.raw))
	}
	t.Logf("theme00: %d silent runs over 3 s %v, %v", len(runs), runs, tr.Duration())
}
