package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/sound"
)

// subtitle.txt read: the rates and tunes at their FRAMENUMBER, tunes 6
// and 7 being intro00 and intro01, eighteen subtitles from frame 55, none
// before; the test skips without the game.
func TestMovieScript(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	g, err := cob.OpenGame(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	b, err := g.Read(movieScriptF)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := parseMovieScript(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if want := []movieCue{{1, 12}, {474, 10}, {544, 10}, {604, 12}, {669, 12}}; !slices.Equal(ms.rates, want) {
		t.Errorf("rates %v, want %v", ms.rates, want)
	}
	if want := []movieCue{{20, 6}, {988, 7}}; !slices.Equal(ms.tunes, want) {
		t.Errorf("tunes %v, want %v", ms.tunes, want)
	}
	lib, err := sound.OpenLibrary(g)
	if err != nil {
		t.Fatal(err)
	}
	if n := lib.Names(); n[6] != "intro00" || n[7] != "intro01" {
		t.Errorf("tunes 6, 7: %s, %s", n[6], n[7])
	}
	if len(ms.subs) != 18 {
		t.Errorf("%d subtitles", len(ms.subs))
	}
	for f, want := range map[int]string{54: "", 55: "THIS GALAXY CONSISTS", 119: "THIS GALAXY CONSISTS",
		120: "OF THREE HUNDRED BILLION STARS.", 1143: "WOULD GAIN"} {
		if got := ms.subAt(f); got != want {
			t.Errorf("frame %d: %q, want %q", f, got, want)
		}
	}
}

// Held, the jukebox stops its theme and starts none; a tune plays; the
// release closes it and a drawn theme starts at the next update.
func TestJukeboxHold(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d, on: true, pick: func(n int) int { return 2 }}
	j.update()
	th := d.last()
	j.hold(true)
	j.update()
	if !th.closed || len(d.started) != 1 {
		t.Fatalf("held: theme %+v, %d started", th, len(d.started))
	}
	j.tune("intro00")
	in := d.last()
	if in.name != "intro00" || !in.playing {
		t.Fatalf("tune %+v", in)
	}
	in.playing = false // its end: still nothing
	j.update()
	if len(d.started) != 2 {
		t.Fatalf("%d started while held", len(d.started))
	}
	j.hold(false)
	j.update()
	if !in.closed || d.last().name != themes[2] {
		t.Fatalf("released: tune %+v, last %s", in, d.last().name)
	}
}

// F9 under the introduction loads the quick save (restart drops the
// stack, the movie with it, no leave): the jukebox's hold goes too, a
// theme plays on the loaded game.
func TestMovieLeftByLoad(t *testing.T) {
	dir := t.TempDir()
	g := game.NewGame(1, game.PickSpecies(1, game.Snovemdomas, 3), game.Average)
	w := testApp(g)
	w.savePath = filepath.Join(dir, "quick.json.gz")
	if !w.saveTo(w.savePath) {
		t.Fatal(w.toast)
	}
	a := &app{savePath: w.savePath, music: &jukebox{dev: &fakeDeck{}, on: true}}
	a.open([]screen{newOpeningScreen(), newMovieScreen()})
	a.music.hold(true) // the movie's Enter, which needs the assets
	pressing(t, ebiten.KeyF9)
	if err := a.Update(); err != nil || !a.inGame() {
		t.Fatalf("F9: %v, game %v (toast %q)", err, a.inGame(), a.toast)
	}
	if a.music.held {
		t.Error("the jukebox still held after the movie was dropped")
	}
}

// The app-level keys pass under the introduction: F3 (the music) and
// F12 (the screenshot, of the movie's frame) do not end it; others do.
func TestMovieKeys(t *testing.T) {
	for _, k := range []ebiten.Key{ebiten.KeyF3, ebiten.KeyF12} {
		if movieLeaves([]ebiten.Key{k}) {
			t.Errorf("%v ends the movie", k)
		}
	}
	if !movieLeaves([]ebiten.Key{ebiten.KeyF12, ebiten.KeyA}) || movieLeaves(nil) {
		t.Error("A kept it, or no key ended it")
	}
}
