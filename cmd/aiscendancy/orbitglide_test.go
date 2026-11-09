package main

import (
	"testing"

	"github.com/rewk/aiscendancy/game"
)

// The close's replay glides the planets from where they stood before it
// to their Pos over its moves: at its start they are at the old Pos,
// halfway between the two, and the model's once the replay ends. The
// ships riding an orbit are no move (no thrust cue).
func TestCloseGlide(t *testing.T) {
	_, v, ships := shipScreen(t)
	a := testApp(v.g)
	a.push(v)
	old := v.planetsNow()
	a.endDay()
	rp := v.replay
	if rp == nil || len(rp.rounds) != 1 || !rp.rounds[0].glide {
		t.Fatalf("replay %v: want the close's glide", rp != nil)
	}
	r := rp.rounds[0]
	for _, m := range r.log.Moves {
		for _, sh := range ships {
			if m.Ship == sh && sh.Orbit >= 0 {
				t.Errorf("%s rode its orbit, replayed as a move", sh.Name)
			}
		}
	}
	at := func(tick int) []game.Vec3 {
		v.tick = rp.t0 + r.start + tick
		v.rebuild()
		var out []game.Vec3
		for _, b := range v.bodies {
			if b.kind == planetBody {
				out = append(out, b.pos)
			}
		}
		return out
	}
	now := v.planetsNow()
	if len(now) == 0 || len(now) != len(old) {
		t.Fatalf("%d planets, %d before", len(now), len(old))
	}
	start, mid := at(r.moveStart()), at(r.moveStart()+r.move/2)
	f := float64(r.move/2) / float64(r.move)
	for i := range now {
		if old[i] == now[i] {
			t.Errorf("planet %d did not turn", i)
		}
		if start[i] != old[i] {
			t.Errorf("planet %d at the start: %v, want %v", i, start[i], old[i])
		}
		if want := lerp(old[i], now[i], f); mid[i] != want {
			t.Errorf("planet %d mid-close: %v, want %v", i, mid[i], want)
		}
	}
	for n := 0; v.replay != nil && n < 1000; n++ {
		v.tick++
		v.replayTick(a)
	}
	v.rebuild()
	i := 0
	for _, b := range v.bodies {
		if b.kind == planetBody {
			if b.pos != now[i] {
				t.Errorf("planet %d after the replay: %v, want %v", i, b.pos, now[i])
			}
			i++
		}
	}
}

// On a fight day the planets hold where they stood through the rounds
// (the model turns them only at the close), then glide.
func TestFightThenGlide(t *testing.T) {
	a, v, _ := combatApp(t)
	old := v.planetsNow()
	a.endDay()
	rp := v.replay
	if rp == nil || len(rp.rounds) < 2 || !rp.rounds[len(rp.rounds)-1].glide || rp.rounds[0].glide {
		t.Fatalf("replay %v: want rounds then the close's glide", rp != nil)
	}
	r := rp.rounds[0]
	v.tick = rp.t0 + r.start + r.length() - 1 // the first round's last tick
	v.rebuild()
	i := 0
	for _, b := range v.bodies {
		if b.kind == planetBody {
			if b.pos != old[i] {
				t.Errorf("planet %d in round 0: %v, want %v", i, b.pos, old[i])
			}
			i++
		}
	}
	if i == 0 {
		t.Fatal("no planet drawn")
	}
}
