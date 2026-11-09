package main

import (
	"fmt"
	"log"
	"slices"

	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/sound"
)

// The sound effects (docs/screens.md "Sound effects"): the original's
// soundfx.txt files, on the music's card at the config's sfx volume (0: none). Only
// these play (never button.voc, the click's ting):
// blueexit when a ship the system screen draws enters or leaves a lane,
// red or blue (redexit is shield.voc's bytes, a placeholder); trust01
// when ships move, trust02 when a Large or Enormous hull is among them;
// a replayed shot's weapon (gbNN) as it takes off, its hit (ghNN) or the
// shield's as it lands (shotCues); bounceSound off a barred orbit
// (bounce.go); destroy as a round's blasts and razings start.
const (
	laneSound   = "blueexit"
	smallThrust = "trust01"
	bigThrust   = "trust02"
	shieldSound = "shield"
	blastSound  = "destroy" // a round's blasts and razings (destroy.shp's)
)

// gunSound and hitSound are weapon g's shot and hit: batfx.txt's sfx1
// sfx2, gbNN and ghNN with NN its Gizmo index, as its banks (Mass
// Barrage Gun: gb00 gh00).
func gunSound(g game.Gizmo) string { return fmt.Sprintf("gb%02d", int(g)) }
func hitSound(g game.Gizmo) string { return fmt.Sprintf("gh%02d", int(g)) }

// thrustSound is the engine of a move whose biggest hull is h.
func thrustSound(h game.Hull) string {
	if h >= game.Large {
		return bigThrust
	}
	return smallThrust
}

// effectDevice starts an effect by name at a volume (oto, a fake in
// tests) and reports the card's failure.
type effectDevice interface {
	effect(name string, volume float64) (voice, error)
	err() error
}

// sfx plays the effects. Nil-safe (none under -shot and in tests). A name
// still playing does not start again: an Auto run's hops make one engine
// noise, a day's flares one whoosh. A failure is logged once and mutes
// it for good.
type sfx struct {
	dev     effectDevice
	volume  float64
	dead    bool
	playing map[string]voice
}

func (x *sfx) play(name string) {
	if x == nil || x.dead || x.volume <= 0 {
		return
	}
	if v := x.playing[name]; v != nil && v.IsPlaying() {
		return
	}
	v, err := x.dev.effect(name, x.volume)
	if err == nil {
		err = x.dev.err()
	}
	if err != nil {
		log.Printf("sound effects: %v; muted", err)
		x.dead = true
		return
	}
	if x.playing == nil {
		x.playing = map[string]voice{}
	}
	if old := x.playing[name]; old != nil {
		old.Close()
	}
	x.playing[name] = v
}

func (o *otoDevice) effect(name string, volume float64) (voice, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	t, err := sound.Effect(o.g, name)
	if err != nil {
		return nil, err
	}
	return stopOnClose(o.d.PlayAt(t, volume)), nil
}

// cue is effect name due at s's tick t; lane marks a flare's, which a
// skipped replay brings forward as the flare (the others are dropped).
type cue struct {
	name string
	t    int
	lane bool
}

// playCues plays the cues due at s.tick and forgets them.
func (s *systemScreen) playCues(x *sfx) {
	s.cues = slices.DeleteFunc(s.cues, func(c cue) bool {
		if c.t > s.tick {
			return false
		}
		x.play(c.name)
		return true
	})
}

// moveCue cues the engine of a move phase at t over the ships of ms that
// s draws (sighted: no fog leak); none drawn, none.
func (s *systemScreen) moveCue(ms []game.Move, t int) {
	h, any := game.Small, false
	for _, m := range ms {
		if slices.ContainsFunc(s.bodies, func(b body) bool { return b.kind == shipBody && b.ship == m.Ship }) {
			h, any = max(h, m.Ship.Design.Hull), true
		}
	}
	if any {
		s.cues = append(s.cues, cue{name: thrustSound(h), t: t})
	}
}

// shotCues cues replayed shot f taking off at t: its weapon then, its hit
// (hull or structure) or the shield's (absorbed, knocked down) as it
// lands fly ticks later. A name still playing is not started again
// (sfx.play).
func (s *systemScreen) shotCues(f game.Fired, t, fly int) {
	s.cues = append(s.cues, cue{name: gunSound(shotGizmo(f)), t: t})
	if h := hitCue(f); h != "" {
		s.cues = append(s.cues, cue{name: h, t: t + fly})
	}
}

// hitCue is shot f's landing sound: its weapon's ghNN on a hull or
// structure, the shield's when absorbed or knocked down; "" none.
func hitCue(f game.Fired) string {
	switch kindOf(f) {
	case hullHit:
		return hitSound(shotGizmo(f))
	case absorbed, knockDown:
		return shieldSound
	}
	return ""
}

// fxSoundTicks is the length in ticks of every impact's and blast's
// sound (each weapon's ghNN, the shield's, the blast's), read as
// gunTicks: an impact or a blast plays that long, ending with its sound.
// A sound missing is left out.
func fxSoundTicks(g *cob.Game) map[string]int {
	names := []string{shieldSound, blastSound}
	for w := range game.Gizmo(len(game.Gizmos)) {
		if game.Gizmos[w].Cat == game.Weapon {
			names = append(names, hitSound(w))
		}
	}
	out := map[string]int{}
	for _, n := range names {
		if t, err := sound.Effect(g, n); err == nil {
			out[n] = max(1, int(t.Duration().Seconds()*60+0.5))
		}
	}
	return out
}

// impactTicks is replayed shot f's impact: its landing sound's length,
// else its frames at fxTicks (no archives: the tests); 0 none.
func (s *systemScreen) impactTicks(f game.Fired) int {
	_, n := impactOf(f)
	if n == 0 {
		return 0
	}
	if t, ok := s.sndTicks[hitCue(f)]; ok {
		return t
	}
	return n * fxTicks
}

// blastTicks is a round's blasts' length: the blast sound's, else
// destroy.shp's frames at fxTicks.
func (s *systemScreen) blastTicks() int {
	if t, ok := s.sndTicks[blastSound]; ok {
		return t
	}
	return destroyFrames * fxTicks
}

// fxStep is the frame of an n-frame effect u ticks into its d.
func fxStep(u, n, d int) int { return min(n-1, u*n/max(d, 1)) }

// gunTicks is each weapon's gbNN length in ticks at 60 TPS, read from
// the archives whether the sound plays or not: a replayed shot flies that
// long, landing as its sound ends (the bullets were otherwise too quick
// between close ships). A sound missing is left out.
func gunTicks(g *cob.Game) map[game.Gizmo]int {
	out := map[game.Gizmo]int{}
	for w := range game.Gizmo(len(game.Gizmos)) {
		if game.Gizmos[w].Cat != game.Weapon {
			continue
		}
		if t, err := sound.Effect(g, gunSound(w)); err == nil {
			out[w] = max(1, int(t.Duration().Seconds()*60+0.5))
		}
	}
	return out
}

// flyTicks is replayed shot f's flight: its weapon's gunTicks, else
// flightTicks (no archives: the tests).
func (s *systemScreen) flyTicks(f game.Fired) int {
	if n, ok := s.gunTicks[shotGizmo(f)]; ok {
		return n
	}
	return flightTicks
}
