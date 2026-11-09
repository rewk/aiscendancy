package cob

import (
	"os"
	"testing"
)

// The tests need the installed game; they skip without it.
func openTestGame(t *testing.T) *Game {
	t.Helper()
	dir := DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	return g
}

func TestTableOfContents(t *testing.T) {
	g := openTestGame(t)
	if len(g.Archives) != 3 {
		t.Fatalf("want 3 archives, got %d", len(g.Archives))
	}
	total := 0
	for _, a := range g.Archives {
		var sum int64
		for _, e := range a.Entries() {
			if e.Size < 0 {
				t.Errorf("%s: %q: negative size", a.Path, e.Name)
			}
			sum += e.Size
		}
		st, _ := os.Stat(a.Path)
		n := len(a.Entries())
		// Every byte after the table of contents belongs to some file.
		if want := st.Size() - int64(4+n*(nameLen+4)); sum != want {
			t.Errorf("%s: sizes sum to %d, want %d", a.Path, sum, want)
		}
		total += n
	}
	if total < 400 {
		t.Errorf("only %d files in all", total)
	}
}

func TestReadByName(t *testing.T) {
	g := openTestGame(t)
	for _, name := range []string{"data/game.pal", `DATA\GAME.PAL`} {
		b, err := g.Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != 768 {
			t.Errorf("%s: %d bytes, want 768", name, len(b))
		}
	}
	if _, err := g.Read("data/nope.shp"); err == nil {
		t.Error("missing file: want an error")
	}
	txt, err := g.Read("gizmos.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(txt) == 0 || txt[0] == 0 {
		t.Error("gizmos.txt looks empty")
	}
}
