package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/sound"
)

// themes are the jukebox's tunes, in order, looped (victory has no
// screen; intro00/01 are the introduction's, movie.go).
var themes = [...]string{"theme00", "theme01", "theme02", "theme03", "theme04"}

func nextTheme(i int) int { return (i + 1) % len(themes) }

// stingerName is the species' tune raceNN, NN its game.Species index:
// OUR ASSUMPTION that race00-20.raw follow that order too.
func stingerName(sp game.Species) string { return fmt.Sprintf("race%02d", int(sp)) }

// voice is one playing tune (oto, a fake in tests): IsPlaying false at
// its end, Pause keeps its place, Close silences it for good.
type voice interface {
	Play()
	Pause()
	IsPlaying() bool
	SetVolume(v float64)
	Close() error
}

// musicDevice starts tunes by name at its volume (setVolume: the next
// ones) and reports the device's failure.
type musicDevice interface {
	play(name string) (voice, error)
	setVolume(v float64)
	err() error
}

// jukebox is the window's music: the themes in turn, stopped by a
// stinger, after which a theme drawn by pick starts afresh (not
// resumed). Nil-safe (none under -shot and in tests). A failure is
// logged once and mutes it for good.
type jukebox struct {
	dev   musicDevice
	on    bool
	dead  bool
	theme int
	cur   voice // the theme, playing or paused; nil before the first
	sting voice
	pick  func(n int) int // the theme after a stinger, in [0,n); nil: rand.IntN
	gap   int             // frames of silence between two tunes
	wait  int             // frames of silence left before the next tune
	held  bool            // a screen plays its own tunes (hold): no theme
}

func (j *jukebox) fail(err error) {
	log.Printf("music: %v; muted", err)
	j.dead = true
	for _, v := range []voice{j.cur, j.sting} {
		if v != nil {
			v.Close()
		}
	}
	j.cur, j.sting = nil, nil
}

func (j *jukebox) start(name string) voice {
	v, err := j.dev.play(name)
	if err != nil {
		j.fail(err)
		return nil
	}
	return v
}

// update runs every frame: an ended tune is closed, the next (drawn
// after a stinger, else in turn) waits gap frames; the first frame
// starts theme00 at once.
func (j *jukebox) update() {
	if j == nil || j.dead || !j.on {
		return
	}
	if err := j.dev.err(); err != nil {
		j.fail(err)
		return
	}
	if j.held {
		return
	}
	switch {
	case j.sting != nil && j.sting.IsPlaying(), j.cur != nil && j.cur.IsPlaying():
		return
	case j.sting != nil:
		j.sting.Close()
		j.sting = nil
		j.theme = rand.IntN(len(themes))
		if j.pick != nil {
			j.theme = j.pick(len(themes))
		}
		j.wait = j.gap
	case j.cur != nil:
		j.cur.Close()
		j.cur = nil
		j.theme = nextTheme(j.theme)
		j.wait = j.gap
	}
	if j.wait > 0 {
		j.wait--
		return
	}
	j.cur = j.start(themes[j.theme])
}

// toggle turns the music off (everything paused in place) or back on;
// it reports the new state, false once dead.
func (j *jukebox) toggle() bool {
	if j == nil || j.dead {
		return false
	}
	j.on = !j.on
	switch {
	case !j.on:
		for _, v := range []voice{j.cur, j.sting} {
			if v != nil {
				v.Pause()
			}
		}
	case j.sting != nil:
		j.sting.Play()
	case j.cur != nil:
		j.cur.Play()
	}
	return j.on
}

// setOn turns the music on or off (toggle).
func (j *jukebox) setOn(on bool) {
	if j != nil && j.on != on {
		j.toggle()
	}
}

// setVolume sets the music's volume, the playing tunes' too.
func (j *jukebox) setVolume(v float64) {
	if j == nil {
		return
	}
	j.dev.setVolume(v)
	for _, t := range []voice{j.cur, j.sting} {
		if t != nil {
			t.SetVolume(v)
		}
	}
}

// setGap sets the silence between two tunes; a wait under way is cut to
// it.
func (j *jukebox) setGap(d time.Duration) {
	if j != nil {
		j.gap = int(d.Seconds() * 60)
		j.wait = min(j.wait, j.gap)
	}
}

// stinger plays sp's tune in place of the theme or a playing stinger
// (closed); nothing when off.
func (j *jukebox) stinger(sp game.Species) { j.tune(stingerName(sp)) }

// hold silences the jukebox for a screen playing its own tunes (tune):
// on, every tune stops and no theme starts; off, the screen's tune stops
// and a theme drawn by pick starts at the next update.
func (j *jukebox) hold(on bool) {
	if j == nil || j.dead {
		return
	}
	for _, v := range []voice{j.cur, j.sting} {
		if v != nil {
			v.Close()
		}
	}
	j.cur, j.sting, j.held = nil, nil, on
	if !on {
		j.theme = rand.IntN(len(themes))
		if j.pick != nil {
			j.theme = j.pick(len(themes))
		}
		j.wait = 0
	}
}

// tune plays the named tune as a stinger: in place of the theme or a
// playing stinger (closed); nothing when off.
func (j *jukebox) tune(name string) {
	if j == nil || j.dead || !j.on {
		return
	}
	if j.sting != nil {
		j.sting.Close()
	}
	if j.cur != nil {
		j.cur.Close()
		j.cur = nil
	}
	j.sting = j.start(name)
}

// closer wraps a voice whose Close does not silence it (oto v3.4's
// Player.Close is a no-op): Close pauses first.
type closer struct{ voice }

func (c closer) Close() error {
	c.Pause()
	return c.voice.Close()
}

// stopOnClose is v with a Close that silences it.
func stopOnClose(v voice) voice { return closer{v} }

// otoDevice is the real musicDevice, the card opened on the first tune
// (music false in the config opens nothing until F3).
type otoDevice struct {
	lib    *sound.Library
	g      *cob.Game
	volume float64
	d      *sound.Device
}

// open opens the card on first use.
func (o *otoDevice) open() error {
	if o.d == nil {
		d, err := sound.Open(o.volume)
		if err != nil {
			return err
		}
		o.d = d
	}
	return nil
}

func (o *otoDevice) play(name string) (voice, error) {
	if o.lib == nil {
		lib, err := sound.OpenLibrary(o.g)
		if err != nil {
			return nil, err
		}
		o.lib = lib
	}
	if err := o.open(); err != nil {
		return nil, err
	}
	t, err := o.lib.Track(name)
	if err != nil {
		return nil, err
	}
	return stopOnClose(o.d.Play(t)), nil
}

func (o *otoDevice) setVolume(v float64) {
	o.volume = v
	if o.d != nil {
		o.d.SetVolume(v)
	}
}

func (o *otoDevice) err() error {
	if o.d == nil {
		return nil
	}
	return o.d.Err()
}

// newJukebox is the window's music from game g, volume clamped to 0..1,
// gap of silence between tunes, and its sound effects on the same card
// at sfxVolume.
func newJukebox(g *cob.Game, on bool, volume float64, gap time.Duration, sfxVolume float64) (*jukebox, *sfx) {
	d := &otoDevice{g: g, volume: min(max(volume, 0), 1)}
	return &jukebox{dev: d, on: on, gap: int(max(gap, 0).Seconds() * 60)}, &sfx{dev: d, volume: min(max(sfxVolume, 0), 1)}
}
