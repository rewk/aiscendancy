package main

import (
	"os"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
)

// laneFixture is shipScreen's first ship just short of endpoint k (the
// home's last lane, a non-trivial index), ordered to the far star but not
// moved yet; k is the lane's index in the star's cluster Lanes.
func laneFixture(t *testing.T) (e *game.Empire, v *systemScreen, sh, other *game.Starship, k int) {
	t.Helper()
	e, v, ships := shipScreen(t)
	c := e.Cluster
	k = len(c.Stars[c.Home].Lanes) - 1
	sh, other = ships[0], ships[1]
	p := v.sys.Lanes[k].Pos
	sh.Orbit, sh.Pos = -1, game.Vec3{X: p.X * 0.97, Y: p.Y * 0.97, Z: p.Z * 0.97}
	sh.Order = game.Order{Kind: game.GoToStar, Star: c.Other(c.Stars[c.Home].Lanes[k], c.Home)}
	return e, v, sh, other, k
}

// A hop that reaches the endpoint lights it as the anim ends, not a tick
// before; the flare is the lane's (its index in the star's Lanes); a hop
// that enters no lane lights nothing.
func TestHopFlares(t *testing.T) {
	e, v, sh, other, k := laneFixture(t)
	v.sel = sh
	if err := v.order(e, sh.Order); err != nil {
		t.Fatal(err)
	}
	if sh.Lane != e.Cluster.Stars[v.star].Lanes[k] || v.anim.ship != sh {
		t.Fatalf("fixture: lane %d (want %d), anim %v", sh.Lane, e.Cluster.Stars[v.star].Lanes[k], v.anim.ship)
	}
	t0 := v.anim.t0
	v.tick = t0 + hopTicks - 1
	v.advance(e)
	if len(v.flares) != 0 {
		t.Errorf("anim running: flares %+v", v.flares)
	}
	v.tick++
	v.advance(e)
	if want := []flare{{k, v.tick}}; !slices.Equal(v.flares, want) {
		t.Errorf("anim over: flares %+v, want %+v", v.flares, want)
	}
	v.flares = nil
	other.Orbit = -1
	v.sel = other
	if err := v.order(e, game.Order{Kind: game.GoToPoint, Pos: game.Vec3{X: other.Pos.X + 0.05, Y: other.Pos.Y, Z: other.Pos.Z}}); err != nil || v.anim.ship != other {
		t.Fatalf("point order: %v, anim %v", err, v.anim.ship)
	}
	v.tick = v.anim.t0 + hopTicks
	v.advance(e)
	if len(v.flares) != 0 {
		t.Errorf("a hop into no lane lit %+v", v.flares)
	}
}

// A day's close that takes a drawn ship into a lane lights its endpoint
// at the end of the close's moves (closeReplay, no round); the idle ship
// lights nothing, nor a quiet day.
func TestEndDayFlares(t *testing.T) {
	e, v, sh, _, k := laneFixture(t)
	a := testApp(v.g)
	a.push(v)
	v.tick = 100
	a.endDay()
	if sh.Lane != e.Cluster.Stars[v.star].Lanes[k] || sh.Star != v.star {
		t.Fatalf("fixture: the close left the ship at lane %d star %d", sh.Lane, sh.Star)
	}
	if want := []flare{{k, 100 + closeTicks}}; !slices.Equal(v.flares, want) {
		t.Errorf("flares %+v, want %+v", v.flares, want)
	}
	v.flares = nil
	v.rebuild()
	a.endDay() // sh is in its lane already, the other idle
	if len(v.flares) != 0 {
		t.Errorf("a quiet day lit %+v", v.flares)
	}
}

// A hop whose anim the close cuts short (its model already in the lane):
// one flare, from the close; advance then drops the stale anim unlit.
func TestCutHopFlaresOnce(t *testing.T) {
	e, v, sh, _, k := laneFixture(t)
	a := testApp(v.g)
	a.push(v)
	v.sel = sh
	if err := v.order(e, sh.Order); err != nil || sh.Lane < 0 || v.anim.ship != sh {
		t.Fatalf("fixture: %v lane %d anim %v", err, sh.Lane, v.anim.ship)
	}
	v.tick = v.anim.t0 + 5
	v.rebuild()
	a.endDay() // the flare waits for the planets' glide (closeTicks)
	if want := []flare{{k, v.tick + closeTicks}}; !slices.Equal(v.flares, want) {
		t.Errorf("after the close: flares %+v, want %+v", v.flares, want)
	}
	v.tick = v.anim.t0 + hopTicks
	v.advance(e)
	if want := []flare{{k, v.anim.t0 + 5 + closeTicks}}; v.anim.ship != nil || !slices.Equal(v.flares, want) {
		t.Errorf("stale anim %v, flares %+v, want %+v", v.anim.ship, v.flares, want)
	}
}

// With a replay playing the flare waits for its end; a skip brings it
// forward to the skip's tick.
func TestFlareAfterReplay(t *testing.T) {
	e, v, sh, _, k := laneFixture(t)
	sh.Lane = e.Cluster.Stars[v.star].Lanes[k]
	v.tick = 50
	v.replay = &replay{t0: 40, rounds: []replayRound{{start: 0, move: moveTicks, flight: flightTicks}}}
	end := v.replay.t0 + v.replay.length()
	v.startFlares([]*game.Starship{sh})
	if want := []flare{{k, end}}; !slices.Equal(v.flares, want) {
		t.Fatalf("flares %+v, want %+v (the replay's end)", v.flares, want)
	}
	if _, on := v.flareFrame(k); on {
		t.Error("lit while the replay plays")
	}
	v.tick = end
	if f, on := v.flareFrame(k); !on || f != 0 {
		t.Errorf("at the replay's end: frame %d %v, want 0", f, on)
	}
	v.tick = 60
	v.skipReplay(testApp(v.g))
	if want := []flare{{k, 60}}; !slices.Equal(v.flares, want) {
		t.Errorf("after a skip: flares %+v, want %+v", v.flares, want)
	}
}

// The frame runs tick/fxTicks up the exitFrames frames and back down,
// then the flare is gone and Update's drop forgets it; one endpoint holds
// one flare.
func TestFlareFrames(t *testing.T) {
	e, v, sh, _, k := laneFixture(t)
	sh.Lane = e.Cluster.Stars[v.star].Lanes[k]
	v.startFlare(e.Cluster, sh.Lane, 10)
	v.startFlare(e.Cluster, sh.Lane, 20) // restarts
	if want := []flare{{k, 20}}; !slices.Equal(v.flares, want) {
		t.Fatalf("flares %+v, want %+v", v.flares, want)
	}
	for _, c := range []struct {
		tick, frame int
		on          bool
	}{
		{19, 0, false}, {20, 0, true}, {20 + fxTicks - 1, 0, true}, {20 + fxTicks, 1, true},
		{20 + 5*fxTicks + 1, 5, true}, {20 + exitFrames*fxTicks - 1, exitFrames - 1, true}, {20 + exitFrames*fxTicks, exitFrames - 2, true},
		{20 + flareTicks - 1, 0, true}, {20 + flareTicks, 0, false},
	} {
		v.tick = c.tick
		if f, on := v.flareFrame(k); f != c.frame || on != c.on {
			t.Errorf("tick %d: frame %d %v, want %d %v", c.tick, f, on, c.frame, c.on)
		}
		if _, on := v.flareFrame(k - 1); on {
			t.Errorf("tick %d: endpoint %d lit", c.tick, k-1)
		}
	}
	v.tick = 20 + flareTicks - 1
	v.dropFlares()
	if len(v.flares) != 1 {
		t.Error("dropped before its end")
	}
	v.tick++
	v.dropFlares()
	if len(v.flares) != 0 {
		t.Errorf("expired, kept %+v", v.flares)
	}
}

// A lit endpoint draws its exit frame in place of the sphere, unlit the
// sphere. Every exit frame shares the sphere's canvas and origin (so
// Render's scale, taken on the sphere's Box, centres it alike); the
// frames grow past the sphere's Box (why not each its own). Render runs
// lit (ReadPixels needs the game loop: no pixel check).
func TestFlareDrawn(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	e, v, sh, _, k := laneFixture(t)
	a := testApp(v.g)
	a.a = as
	sh.Lane = e.Cluster.Stars[v.star].Lanes[k]
	v.rebuild()
	i := slices.IndexFunc(v.bodies, func(b body) bool { return b.kind == laneBody && b.idx == k })
	if i < 0 || v.bodies[i].lane != &v.sys.Lanes[k] {
		t.Fatalf("no lane body %d with its lane", k)
	}
	lb := &v.bodies[i]
	sphere := a.spr(lb.sprite())
	if got := v.laneSprite(a, lb, sphere); got != sphere {
		t.Error("unlit: not the sphere")
	}
	for _, red := range []bool{false, true} {
		l := game.Lane{Red: red}
		sph := a.spr((&body{kind: laneBody, lane: &l}).sprite())
		grows := false
		for f := range exitFrames {
			fr := a.spr(exitBank(&l), f).Frame
			if fr.Center != sph.Frame.Center || fr.Width != sph.Frame.Width || fr.Height != sph.Frame.Height {
				t.Errorf("%s %d: origin %v canvas %dx%d, the sphere's %v %dx%d", exitBank(&l), f, fr.Center, fr.Width, fr.Height,
					sph.Frame.Center, sph.Frame.Width, sph.Frame.Height)
			}
			grows = grows || !fr.Box.In(sph.Frame.Box)
		}
		if !grows {
			t.Errorf("%s never grows past the sphere", exitBank(&l))
		}
	}
	v.tick = 100
	v.flares = []flare{{k, v.tick - (exitFrames-1)*fxTicks}}
	if got := v.laneSprite(a, lb, sphere); got != a.spr(exitBank(lb.lane), exitFrames-1) {
		t.Error("lit: not the exit bank's last frame")
	}
	v.Render(a, ebiten.NewImage(screenW, screenH))
}
