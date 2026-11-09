package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// fakeVoice is a tune on fakeDeck: it plays until the test ends it.
// With deaf, Close does not stop it (oto v3.4's Player.Close).
type fakeVoice struct {
	name    string
	playing bool
	closed  bool
	deaf    bool
}

func (v *fakeVoice) Play()             { v.playing = !v.closed }
func (v *fakeVoice) Pause()            { v.playing = false }
func (v *fakeVoice) IsPlaying() bool   { return v.playing }
func (v *fakeVoice) SetVolume(float64) {}
func (v *fakeVoice) Close() error {
	v.playing, v.closed = v.playing && v.deaf, true
	return nil
}

// fakeDeck records every tune started; fail makes the next play fail,
// devErr is the device's error; oto deals deaf voices through
// stopOnClose, as otoDevice.
type fakeDeck struct {
	started []*fakeVoice
	fail    bool
	devErr  error
	oto     bool
}

func (d *fakeDeck) play(name string) (voice, error) {
	if d.fail {
		return nil, errors.New("no such tune")
	}
	v := &fakeVoice{name: name, playing: true, deaf: d.oto}
	d.started = append(d.started, v)
	if d.oto {
		return stopOnClose(v), nil
	}
	return v, nil
}

func (d *fakeDeck) err() error { return d.devErr }

func (d *fakeDeck) setVolume(float64) {}

func (d *fakeDeck) last() *fakeVoice { return d.started[len(d.started)-1] }

func (d *fakeDeck) names() []string {
	var out []string
	for _, v := range d.started {
		out = append(out, v.name)
	}
	return out
}

func TestNextTheme(t *testing.T) {
	var got []string
	for i, n := 0, 0; n < 7; i, n = nextTheme(i), n+1 {
		got = append(got, themes[i])
	}
	want := []string{"theme00", "theme01", "theme02", "theme03", "theme04", "theme00", "theme01"}
	if !slices.Equal(got, want) {
		t.Errorf("%v, want %v", got, want)
	}
	if stingerName(game.Minions) != "race00" || stingerName(game.Species(game.NumSpecies-1)) != "race20" {
		t.Errorf("stingers %s..%s", stingerName(game.Minions), stingerName(game.Species(game.NumSpecies-1)))
	}
}

// The first frame starts theme00; a theme that ends gives way to the
// next, theme04 to theme00; a playing one is left alone.
func TestJukeboxThemes(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d, on: true}
	j.update()
	j.update()
	if !slices.Equal(d.names(), []string{"theme00"}) {
		t.Fatalf("started %v", d.names())
	}
	for range len(themes) {
		d.last().playing = false // the tune ends
		j.update()
	}
	want := append(themes[:], "theme00")
	if !slices.Equal(d.names(), want) {
		t.Fatalf("started %v, want %v", d.names(), want)
	}
	for _, v := range d.started[:len(d.started)-1] {
		if !v.closed {
			t.Errorf("%s not closed", v.name)
		}
	}
}

// A pick pauses the theme in place and plays raceNN; a second pick
// replaces the stinger; its end resumes the SAME theme voice (no new
// start) without advancing.
func TestJukeboxStinger(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d, on: true, pick: func(n int) int { return n - 2 }}
	j.update()
	j.update()
	d.last().playing = false
	j.update() // theme01 now
	th := d.last()
	j.stinger(game.Orfa)
	if th.playing || !th.closed || j.cur != nil || d.last().name != stingerName(game.Orfa) {
		t.Fatalf("theme %+v, last %s", th, d.last().name)
	}
	first := d.last()
	j.update() // the stinger plays: nothing moves
	j.stinger(game.Mebes)
	if !first.closed || d.last().name != stingerName(game.Mebes) {
		t.Fatalf("second pick: first %+v, last %s", first, d.last().name)
	}
	n := len(d.started)
	d.last().playing = false // the stinger ends: a drawn theme starts afresh
	j.update()
	want := themes[len(themes)-2]
	if len(d.started) != n+1 || d.last().name != want || j.sting != nil || j.theme != len(themes)-2 || th.playing {
		t.Fatalf("after the stinger: %v (was %d starts), want %s; theme index %d, old theme playing %v", d.names(), n, want, j.theme, th.playing)
	}
}

// Off pauses everything in place and starts nothing (no stinger, no
// next theme); on resumes what was paused, the stinger first.
func TestJukeboxToggle(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d}
	j.update()
	j.stinger(game.Orfa)
	if len(d.started) != 0 {
		t.Fatalf("off started %v", d.names())
	}
	if !j.toggle() {
		t.Fatal("toggle did not turn it on")
	}
	j.update()
	th := d.last()
	j.stinger(game.Orfa)
	st := d.last()
	if j.toggle() || st.playing || th.playing {
		t.Fatalf("off: stinger %+v theme %+v", st, th)
	}
	j.update()
	if len(d.started) != 2 || st.closed {
		t.Fatalf("off: update moved on (%v)", d.names())
	}
	j.toggle()
	if !st.playing || th.playing {
		t.Fatalf("on: stinger %+v theme %+v", st, th)
	}
}

// A device error or a missing tune mutes the jukebox for good, its
// voices closed; a nil jukebox (-shot) is inert.
func TestJukeboxFail(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d, on: true}
	j.update()
	th := d.last()
	d.devErr = errors.New("no ALSA")
	j.update()
	if !j.dead || !th.closed || j.cur != nil {
		t.Fatalf("after the device error: dead %v, theme %+v", j.dead, th)
	}
	d.devErr = nil
	j.update()
	j.stinger(game.Orfa)
	if j.toggle() || len(d.started) != 1 {
		t.Fatalf("dead jukebox started %v", d.names())
	}

	d = &fakeDeck{}
	j = &jukebox{dev: d, on: true}
	j.update()
	d.fail = true
	j.stinger(game.Orfa)
	if !j.dead {
		t.Fatal("a missing stinger did not mute")
	}

	var nj *jukebox
	nj.update()
	nj.stinger(game.Orfa)
	if nj.toggle() {
		t.Fatal("nil jukebox toggled on")
	}
}

// The New Game screen's pick plays nothing: the stinger is the
// introduction's (TestIntroFlow).
func TestNewGamePickSilent(t *testing.T) {
	d := &fakeDeck{}
	a := &app{music: &jukebox{dev: d, on: true}}
	a.music.update()
	s := newNewGameScreen()
	s.click(a, float64(ngList.X0+ngList.X1)/2, ngLineY(int(game.Orfa)))
	if s.species != game.Orfa || !slices.Equal(d.names(), []string{"theme00"}) || !d.started[0].playing {
		t.Fatalf("picked %s, started %v", s.species, d.names())
	}
}

// A stinger started while one plays silences it, also on oto's players,
// whose Close is a no-op (the voices of an oto deck): the device's voices
// go through stopOnClose.
func TestStingerReplaces(t *testing.T) {
	for _, oto := range []bool{false, true} {
		d := &fakeDeck{oto: oto}
		j := &jukebox{dev: d, on: true}
		j.update()
		th := d.last()
		j.stinger(game.Orfa)
		first := d.last()
		j.stinger(game.Snovemdomas)
		if first.playing || !first.closed || !d.last().playing || d.last().name != stingerName(game.Snovemdomas) || th.playing {
			t.Fatalf("oto %v: first %+v, last %+v, theme %+v", oto, first, d.last(), th)
		}
	}
}

// Between two tunes the jukebox keeps gap frames of silence: after a
// theme the next in turn, after a stinger the drawn theme; the first
// tune starts at once, and off the silence does not run down.
func TestJukeboxGap(t *testing.T) {
	d := &fakeDeck{}
	j := &jukebox{dev: d, on: true, gap: 3, pick: func(int) int { return 2 }}
	j.update()
	if len(d.started) != 1 {
		t.Fatalf("first frame: %v", d.names())
	}
	d.last().playing = false
	for i := range 3 {
		j.update()
		if len(d.started) != 1 {
			t.Fatalf("silent frame %d: %v", i, d.names())
		}
	}
	j.toggle()
	j.update() // off: the silence does not run down
	j.toggle()
	j.update()
	if len(d.started) != 2 || d.last().name != themes[1] {
		t.Fatalf("after the gap: %v", d.names())
	}
	j.stinger(game.Orfa)
	d.last().playing = false
	for range 3 {
		j.update()
	}
	if len(d.started) != 3 {
		t.Fatalf("stinger's gap: %v", d.names())
	}
	j.update()
	if len(d.started) != 4 || d.last().name != themes[2] {
		t.Fatalf("after the stinger's gap: %v", d.names())
	}
}
