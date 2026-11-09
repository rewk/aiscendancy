package main

import (
	"slices"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/game"
)

// The lane flare (docs/screens.md, the system screen's hops): a ship
// entering a lane, or coming out of one at the close, lights its
// endpoint, blueexit.shp or redexit.shp (lanes.shp's sphere flaring up)
// played forward then back (no abrupt stop), fxTicks a frame,
// in place of the sphere. Starts: a screen hop's anim ending on the
// endpoint (advance); a day's close taking a drawn ship into a lane
// (laneWatch before bot.Day, startFlares after), from the replay's end
// when one plays; a ship out of a lane at the close, at the endpoint it
// came out of (inbound before, startExits after). Each flare sounds
// blueexit (sfx.go). A view effect only: nothing in game/ knows of it.

// flare is lane endpoint k (an index into s.sys.Lanes) lit from tick t0;
// a t0 still ahead (a replay playing first) draws nothing yet.
type flare struct{ k, t0 int }

// flareTicks is a flare's whole run: frames 0 to exitFrames−1 and back
// to 0, the brightest shown once.
const flareTicks = (2*exitFrames - 1) * fxTicks

// exitBank is lane l's flare bank, by its sphere's colour.
func exitBank(l *game.Lane) string {
	if l.Red {
		return redExitBank
	}
	return blueExitBank
}

// startFlare lights the endpoint of lane (a cluster lane) from t0,
// restarting one lit there already, and cues its sound: the lane's index
// in the star's Lanes is the endpoint's in System.Lanes (seenBodies). A
// lane not at s's star lights nothing.
func (s *systemScreen) startFlare(c *game.Cluster, lane, t0 int) {
	k := slices.Index(c.Stars[s.star].Lanes, lane)
	if k < 0 {
		return
	}
	s.flares = slices.DeleteFunc(s.flares, func(f flare) bool { return f.k == k })
	s.flares = append(s.flares, flare{k, t0})
	if c := (cue{name: laneSound, t: t0, lane: true}); !slices.Contains(s.cues, c) {
		s.cues = append(s.cues, c) // several flares at once: one cue
	}
}

// flareFrame is endpoint k's exit frame at s.tick; false when unlit.
func (s *systemScreen) flareFrame(k int) (int, bool) {
	for _, f := range s.flares {
		if t := s.tick - f.t0; f.k == k && t >= 0 && t < flareTicks {
			i := t / fxTicks
			if i >= exitFrames {
				i = 2*exitFrames - 2 - i
			}
			return i, true
		}
	}
	return 0, false
}

// dropFlares forgets the flares run out at s.tick.
func (s *systemScreen) dropFlares() {
	s.flares = slices.DeleteFunc(s.flares, func(f flare) bool { return s.tick-f.t0 >= flareTicks })
}

// laneSprite is lane body b's sprite this tick: its flare's frame when
// lit, else sphere. The frames share the sphere's canvas and origin, so
// the caller's scale, taken on the sphere's Box, centres them alike.
func (s *systemScreen) laneSprite(a *app, b *body, sphere *assets.Sprite) *assets.Sprite {
	if f, ok := s.flareFrame(b.idx); ok {
		return a.spr(exitBank(b.lane), f)
	}
	return sphere
}

// laneWatch is the ships s draws out of the lanes (sighted only: no fog
// leak; those in orbit too) and the hop in flight, whose model may be in
// its lane already (advance drops that anim unlit at the close). endDay
// takes it before bot.Day for startFlares.
func (s *systemScreen) laneWatch() []*game.Starship {
	var out []*game.Starship
	for _, b := range s.bodies {
		if b.kind == shipBody && (b.ship.Lane < 0 || b.ship == s.anim.ship) {
			out = append(out, b.ship)
		}
	}
	return out
}

// flareStart is when a flare the close brings starts: the replay's end
// when one plays, else now.
func (s *systemScreen) flareStart() int {
	if s.replay != nil {
		return s.replay.t0 + s.replay.length()
	}
	return s.tick
}

// startFlares lights the endpoint of each watched ship now in a lane out
// of s's star (flareStart).
func (s *systemScreen) startFlares(watch []*game.Starship) {
	t0 := s.flareStart()
	for _, sh := range watch {
		if sh.Lane >= 0 && sh.Star == s.star {
			s.startFlare(s.g.Cluster, sh.Lane, t0)
		}
	}
}

// laneShip is a ship in a lane and that lane.
type laneShip struct {
	ship *game.Starship
	lane int
}

// inbound is every ship in a lane toward s's star; finishDay takes it
// before the close for startExits.
func (s *systemScreen) inbound() []laneShip {
	var out []laneShip
	if s.g == nil {
		return nil
	}
	for _, e := range s.g.Empires {
		for _, sh := range e.Ships {
			if sh.Lane >= 0 && s.g.Cluster.Other(sh.Lane, sh.Star) == s.star {
				out = append(out, laneShip{sh, sh.Lane})
			}
		}
	}
	return out
}

// startExits lights (flareStart) the endpoint each ship of in came out
// of at the close, when the player sees it now (sighted: no fog leak).
// The live ships, not s.bodies: a replay playing draws its cast, taken
// before the close, which holds no lane ship.
func (s *systemScreen) startExits(in []laneShip) {
	if len(in) == 0 {
		return
	}
	var seen []game.Sighting
	if s.e != nil {
		seen = s.g.ShipsSeen(s.e, s.star)
	}
	now := sighted(shipBodies(s.g, s.star, nil, game.Vec3{}), seen, s.player)
	t0 := s.flareStart()
	for _, l := range in {
		if l.ship.Lane < 0 && l.ship.Star == s.star &&
			slices.ContainsFunc(now, func(b body) bool { return b.ship == l.ship }) {
			s.startFlare(s.g.Cluster, l.lane, t0)
		}
	}
}
