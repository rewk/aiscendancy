package main

import (
	"slices"
	"testing"

	"github.com/rewk/aiscendancy/game"
)

func (d *fakeDeck) effect(name string, volume float64) (voice, error) { return d.play(name) }

// An effect still playing is not started again, one ended is; volume 0
// plays none; a failure mutes for good; nil plays nothing.
func TestSfxPlay(t *testing.T) {
	var none *sfx
	none.play(laneSound)
	d := &fakeDeck{}
	x := &sfx{dev: d, volume: 0.5}
	x.play(laneSound)
	x.play(laneSound)
	x.play(smallThrust)
	if want := []string{laneSound, smallThrust}; !slices.Equal(d.names(), want) {
		t.Fatalf("started %v, want %v", d.names(), want)
	}
	d.started[0].playing = false
	x.play(laneSound)
	if len(d.started) != 3 {
		t.Errorf("an ended effect not restarted: %v", d.names())
	}
	d.fail = true
	x.play(bigThrust)
	d.fail = false
	x.play(bigThrust)
	if !x.dead || len(d.started) != 3 {
		t.Errorf("after a failure: dead %v, started %v", x.dead, d.names())
	}
	q := &fakeDeck{}
	(&sfx{dev: q}).play(laneSound)
	if len(q.started) != 0 {
		t.Errorf("volume 0 played %v", q.names())
	}
}

func TestThrustSound(t *testing.T) {
	for h, want := range map[game.Hull]string{game.Small: smallThrust, game.Medium: smallThrust, game.Large: bigThrust, game.Enormous: bigThrust} {
		if got := thrustSound(h); got != want {
			t.Errorf("%v: %s, want %s", h, got, want)
		}
	}
}

// A hand hop cues its engine at once; the hop into the lane cues the
// whoosh with the flare; playCues plays each when due, once.
func TestHopCues(t *testing.T) {
	e, v, sh, _, k := laneFixture(t)
	v.sel = sh
	if err := v.order(e, sh.Order); err != nil || v.anim.ship != sh {
		t.Fatalf("fixture: %v, anim %v", err, v.anim.ship)
	}
	t0 := v.anim.t0
	if want := []cue{{name: thrustSound(sh.Design.Hull), t: t0}}; !slices.Equal(v.cues, want) {
		t.Fatalf("cues %+v, want %+v", v.cues, want)
	}
	d := &fakeDeck{}
	x := &sfx{dev: d, volume: 1}
	v.tick = t0
	v.playCues(x)
	v.tick = t0 + hopTicks
	v.advance(e)
	if want := []cue{{name: laneSound, t: v.tick, lane: true}}; !slices.Equal(v.cues, want) || len(v.flares) != 1 || v.flares[0].k != k {
		t.Fatalf("anim over: cues %+v, flares %+v", v.cues, v.flares)
	}
	v.playCues(x)
	v.playCues(x)
	if want := []string{thrustSound(sh.Design.Hull), laneSound}; !slices.Equal(d.names(), want) || len(v.cues) != 0 {
		t.Errorf("played %v, want %v; cues left %+v", d.names(), want, v.cues)
	}
}

// The close taking a drawn ship into a lane cues the engine as its moves
// start and the whoosh at their end, with the flare.
func TestEndDayCues(t *testing.T) {
	_, v, sh, _, _ := laneFixture(t)
	a := testApp(v.g)
	a.push(v)
	v.tick = 100
	a.endDay()
	want := []cue{{name: thrustSound(sh.Design.Hull), t: 100}, {name: laneSound, t: 100 + closeTicks, lane: true}}
	if !slices.Equal(v.cues, want) {
		t.Errorf("cues %+v, want %+v", v.cues, want)
	}
}

// A ship out of a lane at the close, drawn here, lights the endpoint it
// came out of and cues the whoosh; one still short of its day does not.
func TestExitFlares(t *testing.T) {
	e, v, sh, other, k := laneFixture(t)
	c := e.Cluster
	lane := c.Stars[v.star].Lanes[k]
	sh.Order, sh.Orbit = game.Order{}, -1
	sh.Star, sh.Lane, sh.Arrive = c.Other(lane, v.star), lane, e.Day
	other.Star, other.Lane, other.Orbit, other.Arrive = sh.Star, lane, -1, e.Day+50
	a := testApp(v.g)
	a.push(v)
	v.rebuild()
	v.tick = 100
	a.endDay()
	if sh.Lane >= 0 || sh.Star != v.star || other.Lane != lane {
		t.Fatalf("fixture: sh lane %d star %d, other lane %d", sh.Lane, sh.Star, other.Lane)
	}
	// After the close's replay: the planets glide closeTicks.
	if want := []flare{{k, 100 + closeTicks}}; !slices.Equal(v.flares, want) {
		t.Errorf("flares %+v, want %+v", v.flares, want)
	}
	if want := []cue{{name: laneSound, t: 100 + closeTicks, lane: true}}; !slices.Equal(v.cues, want) {
		t.Errorf("cues %+v, want %+v", v.cues, want)
	}
}

// A skipped replay drops the cues of the phases skipped and brings the
// flares' forward.
func TestSkipCues(t *testing.T) {
	_, v, _, _, _ := laneFixture(t)
	v.tick = 60
	v.replay = &replay{t0: 40, rounds: []replayRound{{start: 0, move: moveTicks, flight: flightTicks}}}
	v.cues = []cue{{name: smallThrust, t: 50}, {name: bigThrust, t: 70}, {name: laneSound, t: 73, lane: true}}
	v.skipReplay(testApp(v.g))
	if want := []cue{{name: smallThrust, t: 50}, {name: laneSound, t: 60, lane: true}}; !slices.Equal(v.cues, want) {
		t.Errorf("cues %+v, want %+v", v.cues, want)
	}
}

// A ship out of a lane at a close whose moves replay here (another ship
// moving) still lights its endpoint, at the replay's end: the replay's
// cast, taken before the close, holds no lane ship.
func TestExitFlaresUnderReplay(t *testing.T) {
	e, v, sh, other, k := laneFixture(t)
	c := e.Cluster
	lane := c.Stars[v.star].Lanes[k]
	sh.Order, sh.Orbit = game.Order{}, -1
	sh.Star, sh.Lane, sh.Arrive = c.Other(lane, v.star), lane, e.Day
	other.Orbit = -1
	other.Order = game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: other.Pos.X + 0.05, Y: other.Pos.Y, Z: other.Pos.Z}}
	a := testApp(v.g)
	a.push(v)
	v.rebuild()
	v.tick = 100
	a.endDay()
	if sh.Lane >= 0 || sh.Star != v.star || v.replay == nil {
		t.Fatalf("fixture: sh lane %d star %d, replay %v", sh.Lane, sh.Star, v.replay != nil)
	}
	end := v.replay.t0 + v.replay.length()
	if want := []flare{{k, end}}; !slices.Equal(v.flares, want) {
		t.Errorf("flares %+v, want %+v", v.flares, want)
	}
}
