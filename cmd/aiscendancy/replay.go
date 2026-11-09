package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/cam"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/shp"
	"github.com/rewk/aiscendancy/ui"
)

// The combat replay (docs/screens.md "Combat replay"): the system screen
// redraws the day's rounds at its star from game.RoundLog alone, over the
// cast snapshot before them, when the player knows the star, then the
// close's moves (closeReplay). A round plays in phases, each only when
// it has something, in the rule's order (shots before moves): its shots
// one by one in shotOrder's order from the round's starting positions, shot k's
// bullet flying from shotAt(k) as long as its weapon's sound (flyTicks),
// then its impact over its
// target (impactOf) as long as its landing sound (impactTicks) while
// shot k+1 flies, a floating number rising over the target (floats);
// the destroyed and razed blow up after the last impact (destroy.shp,
// replacing the ship's sprite) as long as the blast's sound
// (blastTicks), then the survivors'
// moves tween (moveTicks). Space, End Turn or a click in the view skips
// it.

// The replay's timing, ticks at 60 TPS; preload checks the frame counts.
const (
	moveTicks     = 18
	closeTicks    = 30 // the close's moves (closeReplay): slower than a round's
	flightTicks   = 15 // a shot's bullet without its sound's length (flyTicks); the next shot flies as it lands
	floatTicks    = 45 // a floating number's life (floats)
	floatRise     = 30 // native px it rises over it
	fxTicks       = 3  // ticks per frame of every effect
	explFrames    = 10 // explNN.shp
	shieldFrames  = 8  // shield.shp
	destroyFrames = 15 // destroy.shp
	exitFrames    = 10 // blueexit.shp, redexit.shp: the lane flare (flare.go)
)

// fxPixel is an effect's world size per art pixel when not sized on a
// body: a ship sprite's (a Small hull ~100 px is drawn 2·0.05 wide).
const fxPixel = 0.001

// bulletBank and explBank are weapon g's projectile and impact banks:
// NN = the weapon's Gizmo index (docs/formats.md, checked on 00 and 05).
func bulletBank(g game.Gizmo) string { return fmt.Sprintf("bullet%02d", int(g)) }
func explBank(g game.Gizmo) string   { return fmt.Sprintf("expl%02d", int(g)) }

const (
	shieldBank   = "shield"
	destroyBank  = "destroy"
	blueExitBank = "blueexit" // lanes.shp's spheres flaring, same canvas and origin
	redExitBank  = "redexit"
)

// planetGun is the ship weapon an orbital weapon's shot is drawn with
// (no bank of their own): Missile Base = Fourier Missiles, else Mass
// Barrage Gun.
func planetGun(it game.Item) game.Gizmo {
	if it == game.OrbitalMissileBase {
		return game.FourierMissiles
	}
	return game.MassBarrageGun
}

// shotGizmo is the weapon f is drawn with.
func shotGizmo(f game.Fired) game.Gizmo {
	if f.FromPlanet {
		return planetGun(f.ShooterItem)
	}
	return f.Gizmo
}

// destroy.shp is every hull's blast; destroy2.shp is unused (formats.md).

// replay is one day's rounds at the screen's star, from t0 (the screen's
// tick at endDay).
type replay struct {
	cast   []body      // the ships at the day's start, pos where drawn
	orbits []game.Vec3 // the planets' Pos before the close (planetsNow), nil when it lays none
	rounds []replayRound
	t0     int
	navDue []game.Event // the day's events whose climb waits for the replay's end (app.navigate)
	fight  bool         // a round is laid (startReplay), not the close's moves only
}

// replayRound is one RoundLog laid out: cast[i]'s position at the
// round's start (at), where its shots fly from and to, and after its
// moves (to), its orbit then (orbAt, orbTo, −1 in space), destroyed
// earlier (gone) or now (boom); the phases' lengths, 0 when empty, in play order flight (the first
// shot's), impact (the other shots' up to the last impact's end), kill,
// move.
type replayRound struct {
	log                        game.RoundLog
	start                      int // ticks from t0
	at, to                     []game.Vec3
	orbAt, orbTo               []int
	gone, boom                 []bool
	move, flight, impact, kill int
	fly                        []int             // shot k's flight in ticks (flyTicks); past it flightTicks
	num                        int               // the day's round number at the star (the round log's header), 0 the close's
	glide                      bool              // the close's: the planets glide from replay.orbits to their Pos over its moves
	touch                      map[int]game.Vec3 // cast index -> a bounce's turn (bounceAt), its move drawn bouncePos
}

// flightOf is shot k's flight in ticks.
func (r *replayRound) flightOf(k int) int {
	if k < len(r.fly) {
		return r.fly[k]
	}
	return flightTicks
}

// shotOrder is the order the replay plays a round's shots in, a copy:
// the sides alternate (cosmetic: Round resolves
// them all before any ship goes), each side's own order kept and each
// target's too (an outcome hangs on the shots before it: absorbed, then
// down, then hull), the shot that destroyed a ship last (its last hull
// hit; a shot after it at the wreck did nothing).
func shotOrder(shots []game.Fired, destroyed []*game.Starship) []game.Fired {
	type key struct {
		ship   *game.Starship
		planet int
	}
	target := func(f game.Fired) key {
		if f.Target != nil {
			return key{f.Target, -1}
		}
		return key{nil, f.TargetPlanet}
	}
	lastAt := map[*game.Starship]int{}
	for i, f := range shots {
		if f.Target != nil && f.Damage > 0 && slices.Contains(destroyed, f.Target) {
			lastAt[f.Target] = i
		}
	}
	var kills, rest []int
	for i, f := range shots {
		if k, ok := lastAt[f.Target]; ok && k == i {
			kills = append(kills, i)
		} else {
			rest = append(rest, i)
		}
	}
	// Round robin over the sides in first-shot order: a side's next shot
	// goes when no earlier shot at its target waits.
	var sides []int
	queue := map[int][]int{}
	for _, i := range rest {
		e := shots[i].ShooterEmpire
		if _, ok := queue[e]; !ok {
			sides = append(sides, e)
		}
		queue[e] = append(queue[e], i)
	}
	waiting := func(i int) bool {
		for _, q := range queue {
			for _, j := range q {
				if j < i && target(shots[j]) == target(shots[i]) {
					return true
				}
			}
		}
		return false
	}
	out := make([]game.Fired, 0, len(shots))
	for n := 0; n < len(rest); {
		for _, e := range sides {
			if q := queue[e]; len(q) > 0 && !waiting(q[0]) {
				out, queue[e] = append(out, shots[q[0]]), q[1:]
				n++
			}
		}
	}
	for _, i := range kills {
		out = append(out, shots[i])
	}
	return out
}

// shotAt is a round's tick shot k's bullet takes off at: the shots fly
// one by one, each landing (its impact) as the next takes off.
func (r *replayRound) shotAt(k int) int {
	t := 0
	for i := range k {
		t += r.flightOf(i)
	}
	return t
}

// landsAt is a round's tick shot k's impact starts at.
func (r *replayRound) landsAt(k int) int { return r.shotAt(k) + r.flightOf(k) }

func (r *replayRound) length() int { return r.flight + r.impact + r.kill + r.move }

// moveStart is the round's tick its moves start at, after the shots.
func (r *replayRound) moveStart() int { return r.flight + r.impact + r.kill }

// impactKind is what a shot did at its target.
type impactKind uint8

const (
	noImpact  impactKind = iota
	hullHit              // Damage on a hull or a structure
	absorbed             // all absorbed by the shields
	knockDown            // knocked the shields down (Fired.ShieldsDown), no damage
)

// kindOf is f's impactKind.
func kindOf(f game.Fired) impactKind {
	switch {
	case f.Damage > 0:
		return hullHit
	case f.ShieldsDown:
		return knockDown
	case f.Absorbed > 0:
		return absorbed
	}
	return noImpact
}

// impactOf is a shot's effect at its target: the weapon's explosion on
// hull, the shield's when absorbed or knocked down, else "".
func impactOf(f game.Fired) (bank string, frames int) {
	switch kindOf(f) {
	case hullHit:
		return explBank(shotGizmo(f)), explFrames
	case absorbed, knockDown:
		return shieldBank, shieldFrames
	}
	return "", 0
}

// replayCast is the ships out of the lanes at s's star now, as
// shipBodies places them, a foreign one with the player's Sighting of it
// now (castSeen: the round log's name for it).
func (s *systemScreen) replayCast() []body {
	if s.g == nil {
		return nil
	}
	var out []body
	for _, b := range shipBodies(s.g, s.star, nil, game.Vec3{}) {
		if b.ship.Lane < 0 {
			if b.owner != s.player && s.e != nil {
				if sg, ok := s.g.SightingOf(s.e, b.ship); ok {
					b.castSeen = &sg
				}
			}
			out = append(out, b)
		}
	}
	return out
}

// startReplay lays out the rounds of logs at s's star over cast, when e
// knows the star; empty rounds add nothing. A replay still running goes
// on (combat mode's steps, the close's moves): the new rounds start at
// its end, from its last positions, over its own cast, a ship not in
// cast (destroyed, in a lane) gone from then on.
//
// uses[i], when given, is the specials used before logs[i]'s round
// (bot.Step's, DayRun.Round's reps: specialUses), shown at its start.
func (s *systemScreen) startReplay(e *game.Empire, cast []body, logs []game.RoundLog, uses ...[]*game.SpecialUse) {
	n := 0
	if s.replay != nil {
		n = len(s.replay.rounds)
	}
	num0 := 0 // the day's rounds at the star before logs' (their numbers follow)
	if s.g != nil {
		num0 = max(0, s.g.RoundsAt(s.star)-len(slices.DeleteFunc(slices.Clone(logs), func(l game.RoundLog) bool { return l.Star != s.star })))
	}
	s.layReplay(e, cast, logs, moveTicks, num0, uses, nil)
	if s.replay != nil && len(s.replay.rounds) > n {
		s.replay.fight = true
	}
}

// closeReplay adds the day's close at s's star after any rounds: one
// move-only phase over closeTicks, each ship of cast (taken before the
// close) moving to where the close took it (closeMoves), the planets
// gliding from orbits (planetsNow before the close) to their Pos; none
// when nothing moved.
func (s *systemScreen) closeReplay(e *game.Empire, cast []body, orbits []game.Vec3) {
	ms := s.closeMoves(cast)
	if slices.Equal(orbits, s.planetsNow()) {
		orbits = nil
	}
	if len(ms) > 0 || orbits != nil {
		s.layReplay(e, cast, []game.RoundLog{{Star: s.star, Moves: ms}}, closeTicks, -1, nil, orbits)
	}
}

// planetsNow is the model's Pos of s's planets, by index.
func (s *systemScreen) planetsNow() []game.Vec3 {
	var out []game.Vec3
	if s.sys != nil {
		for _, p := range s.sys.Planets {
			out = append(out, p.Pos)
		}
	}
	return out
}

// glidePlanets places the planet bodies of a replay laying the close's
// orbits: where they stood before the close (replay.orbits) until its
// round, gliding to their Pos over its moves (a chord: the model's step
// is a few hundredths), at it after.
func (s *systemScreen) glidePlanets() {
	rp := s.replay
	if rp == nil || rp.orbits == nil {
		return
	}
	r, t := rp.now(s.tick)
	f := 1.0
	for i := range rp.rounds {
		if g := &rp.rounds[i]; g.glide {
			switch {
			case r.start < g.start:
				f = 0
			case r == g && r.move > 0:
				f = min(1, max(0, float64(t-r.moveStart())/float64(r.move)))
			}
		}
	}
	for i := range s.bodies {
		if b := &s.bodies[i]; b.kind == planetBody && b.idx < len(rp.orbits) {
			b.pos = lerp(rp.orbits[b.idx], b.planet.Pos, f)
		}
	}
}

// closeMoves is the close as Moves: each ship of cast still at s's star,
// out of the lanes or in a lane out of it (at its endpoint sphere, as a
// round's lane entry), whose position or orbit changed; one in the same
// orbit before and after only rode its planet (no move, no sound).
func (s *systemScreen) closeMoves(cast []body) []game.Move {
	var out []game.Move
	for _, b := range cast {
		sh := b.ship
		if sh.Star != s.star || s.g.OwnerOf(sh) == nil {
			continue
		}
		orb, was := -1, -1
		if sh.Lane < 0 {
			orb = sh.Orbit
		}
		if b.orbiting {
			was = b.orbit
		}
		if orb >= 0 && orb == was { // rode its planet's orbit: no move of its own
			continue
		}
		if sh.Pos != b.pos || orb != was {
			out = append(out, game.Move{Ship: sh, Empire: b.owner, From: b.pos, To: sh.Pos, Orbit: orb})
		}
	}
	return out
}

// layReplay is startReplay with mt ticks for a round's moves, the
// star's logs numbered num0+1 on (the round log's headers; −1: the
// close's, unnumbered), each round's lines added to the round log.
//
// orbits, non-nil for the close only (closeReplay), lays its round even
// with no move, marked glide.
func (s *systemScreen) layReplay(e *game.Empire, cast []body, logs []game.RoundLog, mt, num0 int, uses [][]*game.SpecialUse, orbits []game.Vec3) {
	if !knows(e, s.star) {
		return
	}
	rp := s.replay
	var at []game.Vec3
	var orb []int
	var gone []bool
	start := 0
	if rp == nil {
		rp = &replay{cast: cast, t0: s.tick}
		at, orb, gone = make([]game.Vec3, len(cast)), make([]int, len(cast)), make([]bool, len(cast))
		for i, b := range cast {
			at[i], orb[i] = b.pos, -1
			if b.orbiting {
				orb[i] = b.orbit
			}
		}
	} else {
		last := &rp.rounds[len(rp.rounds)-1]
		at, orb, start = last.to, last.orbTo, rp.length()
		gone = make([]bool, len(rp.cast))
		for i, b := range rp.cast {
			gone[i] = last.gone[i] || last.boom[i] || !slices.ContainsFunc(cast, func(c body) bool { return c.ship == b.ship })
		}
	}
	idx := func(sh *game.Starship) int {
		return slices.IndexFunc(rp.cast, func(b body) bool { return b.ship == sh })
	}
	n0, j := len(rp.rounds), 0
	if s.replay == nil {
		s.rlog.cut(s.tick) // a replay dropped unended (a new day): its lines all out
	}
	if s.rlog == nil {
		s.rlog = &roundLog{}
	}
	// lay adds l as a round from start, its moves over mt ticks.
	lay := func(l game.RoundLog, mt int) *replayRound {
		l.Shots = shotOrder(l.Shots, l.Destroyed)
		r := replayRound{log: l, start: start, at: at, to: slices.Clone(at), orbAt: orb, orbTo: slices.Clone(orb),
			gone: gone, boom: make([]bool, len(rp.cast))}
		bounced := false
		for _, m := range l.Moves {
			if i := idx(m.Ship); i >= 0 {
				r.to[i], r.orbTo[i] = m.To, m.Orbit
				if touch, _, ok := bounceAt(s.g, m.Ship, m.To); ok && m.From != m.To {
					if r.touch == nil {
						r.touch = map[int]game.Vec3{}
					}
					r.touch[i] = touch
					bounced = bounced || slices.ContainsFunc(s.bodies, func(b body) bool { return b.kind == shipBody && b.ship == m.Ship })
				}
			}
		}
		for _, d := range l.Destroyed {
			if i := idx(d); i >= 0 {
				r.boom[i] = true
			}
		}
		end := 0 // the last impact's end
		for _, f := range l.Shots {
			r.fly = append(r.fly, s.flyTicks(f))
		}
		for k, f := range l.Shots {
			end = max(end, r.landsAt(k)+s.impactTicks(f))
			s.shotCues(f, rp.t0+start+r.shotAt(k), r.flightOf(k))
		}
		if len(l.Shots) > 0 {
			r.flight, r.impact = r.flightOf(0), end-r.flightOf(0)
		}
		if len(l.Destroyed)+len(l.Razed) > 0 {
			r.kill = s.blastTicks()
			s.cues = append(s.cues, cue{name: blastSound, t: rp.t0 + start + r.blastStart()})
		}
		if len(l.Moves) > 0 || orbits != nil {
			r.move, r.glide = mt, orbits != nil
		}
		if len(l.Moves) > 0 {
			s.moveCue(l.Moves, rp.t0+start+r.moveStart())
			if bounced { // a drawn ship's: no fog leak
				s.cues = append(s.cues, cue{name: bounceSound, t: rp.t0 + start + r.moveStart() + int(bounceIn*float64(mt))})
			}
		}
		rp.rounds = append(rp.rounds, r)
		start += r.length()
		at, orb = r.to, r.orbTo
		gone = slices.Clone(gone)
		for i, b := range r.boom {
			gone[i] = gone[i] || b
		}
		return &rp.rounds[len(rp.rounds)-1]
	}
	for li, l := range logs {
		if l.Star != s.star {
			continue
		}
		j++
		var us []*game.SpecialUse
		if li < len(uses) {
			us = uses[li]
		}
		// The specials used before the round, one after the other at its
		// start, each a round of its own effects (its kills' blasts, then
		// its moves' tween) when the player sees its user (useLine); the
		// round's shots then from where they left the ships.
		var lines []logLine
		veil := s.orbitVeil(e, logs[li:]) // as the player saw the orbits then
		for _, u := range us {
			ul, ok := s.useLine(rp.cast, u, rp.t0+start, veil)
			if !ok {
				continue
			}
			lines = append(lines, ul)
			if len(u.Moves)+len(u.Destroyed) > 0 {
				lay(game.RoundLog{Star: s.star, Moves: u.Moves, Destroyed: u.Destroyed}, moveTicks)
			}
		}
		if len(l.Moves)+len(l.Shots)+len(l.Destroyed) > 0 || orbits != nil {
			t0 := rp.t0 + start
			r := lay(l, mt)
			if num0 >= 0 {
				r.num = num0 + j
			}
			lines = append(lines, s.roundLines(rp, r, t0, veil)...)
		}
		if len(lines) > 0 && num0 >= 0 {
			lines = append([]logLine{roundHeader(num0+j, lines[0].t)}, lines...)
		}
		s.rlog.add(lines...)
	}
	if len(rp.rounds) > n0 {
		s.replay = rp
		if orbits != nil {
			rp.orbits = orbits
		}
	}
}

// length is the whole replay's ticks.
func (rp *replay) length() int {
	r := &rp.rounds[len(rp.rounds)-1]
	return r.start + r.length()
}

// now is the round playing at tick and the ticks into it (clamped to
// the last round's end).
func (rp *replay) now(tick int) (*replayRound, int) {
	t := tick - rp.t0
	for i := range rp.rounds {
		if r := &rp.rounds[i]; t < r.start+r.length() {
			return r, max(0, t-r.start)
		}
	}
	r := &rp.rounds[len(rp.rounds)-1]
	return r, r.length()
}

// replayTick ends the replay once over: true when a climb waited.
func (s *systemScreen) replayTick(a *app) bool {
	if s.replay != nil && s.tick-s.replay.t0 >= s.replay.length() {
		return s.endReplay(a)
	}
	return false
}

// skipReplay drops a running replay: true when the press is spent on it,
// a fight's or one a climb waits for (navDue); the close's moves alone
// are cut and false, the press going on (no second press a day).
func (s *systemScreen) skipReplay(a *app) bool {
	if s.replay == nil {
		return false
	}
	spent := s.replay.fight || len(s.replay.navDue) > 0
	s.endReplay(a)
	return spent
}

// endReplay drops the replay and runs the climb that waited for it:
// true then (this screen may be gone).
func (s *systemScreen) endReplay(a *app) bool {
	due := s.replay.navDue
	s.replay = nil
	s.rlog.cut(s.tick)        // a skip shows the whole log at once
	for i := range s.flares { // a skip brings forward the flares waiting for the end
		s.flares[i].t0 = min(s.flares[i].t0, s.tick)
	}
	s.cues = slices.DeleteFunc(s.cues, func(c cue) bool { return c.t > s.tick && !c.lane }) // a skipped phase's
	for i := range s.cues {
		s.cues[i].t = min(s.cues[i].t, s.tick)
	}
	if len(due) > 0 {
		a.navigate(due)
	}
	return len(due) > 0
}

// lerp is a + (b − a)·f.
func lerp(a, b game.Vec3, f float64) game.Vec3 {
	return game.Vec3{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f, Z: a.Z + (b.Z-a.Z)*f}
}

// replayBodies is the cast at the screen's tick: each ship not yet
// blown up at its tweened position, those in orbit hidden (orbiting).
func (s *systemScreen) replayBodies() []body {
	r, t := s.replay.now(s.tick)
	f := 1.0
	if r.move > 0 {
		f = min(1, max(0, float64(t-r.moveStart())/float64(r.move)))
	}
	var out []body
	for i := range s.replay.cast {
		if r.gone[i] || r.boom[i] && t >= r.blastStart() {
			continue
		}
		o := r.orbTo[i] // a ship flying into or out of an orbit is in space
		switch {
		case t < r.moveStart():
			o = r.orbAt[i] // the shots' phases: where the round started
		case r.orbAt[i] != o && f < 1:
			o = -1
		}
		pos := lerp(r.at[i], r.to[i], f)
		if touch, ok := r.touch[i]; ok && t >= r.moveStart() {
			pos = bouncePos(r.at[i], touch, r.to[i], f)
		}
		out = append(out, s.replayBody(i, pos, o))
	}
	return out
}

// blastStart is the round's tick its explosions start at.
func (r *replayRound) blastStart() int { return r.flight + r.impact }

// replayBody is cast[i] at pos in orbit (−1 in space); a foreign one
// carries its Sighting while the player sees it.
func (s *systemScreen) replayBody(i int, pos game.Vec3, orbit int) body {
	b := s.replay.cast[i]
	b.pos = pos
	b.orbiting, b.orbit = orbit >= 0, orbit
	if b.owner != s.player && s.e != nil {
		if sg, ok := s.g.SightingOf(s.e, b.ship); ok {
			b.seen = &sg
		}
	}
	return b
}

// fxKind is how an effect is placed and sized.
type fxKind uint8

const (
	fxBullet fxKind = iota // flies shooter → target, fxPixel per art pixel
	fxImpact               // over its target's sprite, sized on it (impactAt)
	fxBlast                // in place of its ship's sprite, sized on it (blastAt)
	fxRaze                 // over a planet, sized on its orbit ring (razeAt)
)

// effect is one replay sprite this tick: bank's frame step (a bullet's
// looped). pos is where it depth-sorts, and is drawn when ship (the body
// it sits on, a planet's for one struck or razed) is nil; a bullet flies
// from (from, fromPos) to (ship, toPos), fraction f of the way.
type effect struct {
	kind           fxKind
	bank           string
	step           int
	pos            game.Vec3
	ship, from     *body
	fromPos, toPos game.Vec3
	f              float64
}

// shotEnds is shot f's ends in round r: its shooter's and target's
// bodies at the round's start, before the moves (a planet's body for a
// planet's end), the log's From/To when not drawn.
func (s *systemScreen) shotEnds(r *replayRound, f game.Fired) (from *body, p game.Vec3, to *body, q game.Vec3) {
	if f.FromPlanet {
		from, p = s.planetEnd(f.ShooterPlanet, f.From)
	} else {
		from, p = s.castEnd(r, f.Shooter, f.From)
	}
	if f.AtPlanet && f.Target == nil {
		to, q = s.planetEnd(f.TargetPlanet, f.To)
	} else {
		to, q = s.castEnd(r, f.Target, f.To)
	}
	return
}

// castEnd is sh's body at round r's start, logged when not in the cast.
func (s *systemScreen) castEnd(r *replayRound, sh *game.Starship, logged game.Vec3) (*body, game.Vec3) {
	if i := slices.IndexFunc(s.replay.cast, func(b body) bool { return sh != nil && b.ship == sh }); i >= 0 {
		b := s.replayBody(i, r.at[i], r.orbAt[i])
		return &b, b.pos
	}
	return nil, logged
}

// planetEnd is planet pl's body, logged when not drawn.
func (s *systemScreen) planetEnd(pl int, logged game.Vec3) (*body, game.Vec3) {
	if b := s.planetBody(pl); b != nil {
		return b, b.pos
	}
	return nil, logged
}

// effects is what the replay draws this tick besides the ships: in the
// shots' phases each shot's bullet in flight and impact playing (several
// at once: an impact outlasts the next shot's flight), then the blasts.
func (s *systemScreen) effects() []effect {
	if s.replay == nil {
		return nil
	}
	r, t := s.replay.now(s.tick)
	var out []effect
	switch {
	case t < r.blastStart():
		for k, sh := range r.log.Shots {
			if u := t - r.shotAt(k); u >= 0 && u < r.flightOf(k) {
				from, p, to, q := s.shotEnds(r, sh)
				f := float64(u) / float64(r.flightOf(k))
				out = append(out, effect{kind: fxBullet, bank: bulletBank(shotGizmo(sh)), step: u / fxTicks, pos: lerp(p, q, f),
					ship: to, from: from, fromPos: p, toPos: q, f: f})
			}
		}
		for k, sh := range r.log.Shots {
			if bank, n := impactOf(sh); bank != "" && t >= r.landsAt(k) && t < r.landsAt(k)+s.impactTicks(sh) {
				_, _, to, q := s.shotEnds(r, sh)
				out = append(out, effect{kind: fxImpact, bank: bank, step: fxStep(t-r.landsAt(k), n, s.impactTicks(sh)), pos: q, ship: to})
			}
		}
	case t < r.blastStart()+r.kill:
		t -= r.blastStart()
		for _, d := range r.log.Destroyed {
			b, p := s.castEnd(r, d, d.Pos)
			out = append(out, effect{kind: fxBlast, bank: destroyBank, step: fxStep(t, destroyFrames, r.kill), pos: p, ship: b})
		}
		for _, z := range r.log.Razed {
			b, p := s.planetEnd(z.Planet, s.sys.Planets[z.Planet].Pos)
			out = append(out, effect{kind: fxRaze, bank: destroyBank, step: fxStep(t, destroyFrames, r.kill), pos: p, ship: b})
		}
	}
	return out
}

// floatText is a floating number over a shot's target: what it says, its
// colour, the target's body (nil: at q) and its age in ticks.
type floatText struct {
	text string
	c    color.RGBA
	to   *body
	q    game.Vec3
	age  int
}

// floatColour, by impactKind: hull red, absorbed light blue, knock-down yellow.
var floatColour = [...]color.RGBA{
	hullHit:   {255, 70, 50, 255},
	absorbed:  {140, 200, 255, 255},
	knockDown: yellow,
}

// floatOf is shot f's floating text, false for none: "-N" for hull or
// structure damage, "0" all absorbed, "shields down" a knock-down.
func floatOf(f game.Fired) (string, color.RGBA, bool) {
	switch k := kindOf(f); k {
	case hullHit:
		return fmt.Sprintf("-%d", f.Damage), floatColour[k], true
	case absorbed:
		return "0", floatColour[k], true
	case knockDown:
		return "shields down", floatColour[k], true
	}
	return "", color.RGBA{}, false
}

// floats is the floating texts this tick: each shot's from its impact's
// start for floatTicks, across the phases and rounds (cut at the
// replay's end), in log order.
func (s *systemScreen) floats() []floatText {
	if s.replay == nil {
		return nil
	}
	t := s.tick - s.replay.t0
	var out []floatText
	for i := range s.replay.rounds {
		r := &s.replay.rounds[i]
		for k, f := range r.log.Shots {
			age := t - r.start - r.landsAt(k)
			if age < 0 || age >= floatTicks {
				continue
			}
			if txt, c, ok := floatOf(f); ok {
				_, _, to, q := s.shotEnds(r, f)
				out = append(out, floatText{txt, c, to, q, age})
			}
		}
	}
	return out
}

// floatsText draws the floating texts in the view, bigFace with a black
// shadow, centred over their target's drawn box (its point when not
// drawn), rising floatRise px and fading out over floatTicks.
func (s *systemScreen) floatsText(a *app, frame *ebiten.Image) {
	dst := clipped(frame, sysArea)
	for _, ft := range s.floats() {
		pt := s.cam.Project(ft.q)
		x, y := pt.X, pt.Y
		if ft.to != nil {
			if box, ok := s.drawnBox(a, ft.to); ok {
				x, y = (box.x0+box.x1)/2, box.y0
			}
		}
		f := float64(ft.age) / floatTicks
		y -= bigFace.Size*1.25 + floatRise*f
		x -= ui.Width(ft.text, bigFace) / 2
		al := uint8(255 * (1 - f))
		ui.Print(dst, ft.text, bigFace, x+1, y+1, color.NRGBA{0, 0, 0, al})
		ui.Print(dst, ft.text, bigFace, x, y, color.NRGBA{ft.c.R, ft.c.G, ft.c.B, al})
	}
}

// planetBody is planet pl's body in s.bodies, nil when not drawn.
func (s *systemScreen) planetBody(pl int) *body {
	for i := range s.bodies {
		if b := &s.bodies[i]; b.kind == planetBody && b.idx == pl {
			return b
		}
	}
	return nil
}

// placedEffect is an effect projected for this frame.
type placedEffect struct {
	effect
	pt cam.Point
}

// placeEffects projects effects back to front (stable: log order kept).
func (s *systemScreen) placeEffects() []placedEffect {
	var out []placedEffect
	for _, e := range s.effects() {
		out = append(out, placedEffect{e, s.cam.Project(e.pos)})
	}
	slices.SortStableFunc(out, func(p, q placedEffect) int { return -cmpFloat(p.pt.Depth, q.pt.Depth) })
	return out
}

// drawItem is one sprite of the system view in draw order: a body or an
// effect.
type drawItem struct {
	body *placed
	fx   *placedEffect
}

// drawList is the view's sprites back to front: the bodies, each one's
// impacts and razings right after it (over it whatever their depth),
// every other effect before the first body nearer than it.
func (s *systemScreen) drawList() []drawItem {
	ps, fx := s.placeAll(), s.placeEffects()
	type key struct { // a ship's body or a planet's: the other nil
		sh *game.Starship
		pl *game.Planet
	}
	keyOf := func(b *body) key { return key{b.ship, b.planet} }
	shown := map[key]bool{}
	for _, p := range ps {
		if p.kind == shipBody || p.kind == planetBody {
			shown[keyOf(p.body)] = true
		}
	}
	over := map[key][]*placedEffect{}
	var free []*placedEffect
	for i := range fx {
		if e := &fx[i]; (e.kind == fxImpact || e.kind == fxRaze) && e.ship != nil && shown[keyOf(e.ship)] {
			over[keyOf(e.ship)] = append(over[keyOf(e.ship)], e)
		} else {
			free = append(free, e)
		}
	}
	var out []drawItem
	for i := range ps {
		p := &ps[i]
		for len(free) > 0 && free[0].pt.Depth >= p.pt.Depth {
			out = append(out, drawItem{fx: free[0]})
			free = free[1:]
		}
		out = append(out, drawItem{body: p})
		if p.kind == shipBody || p.kind == planetBody {
			for _, e := range over[keyOf(p.body)] {
				out = append(out, drawItem{fx: e})
			}
		}
	}
	for _, e := range free {
		out = append(out, drawItem{fx: e})
	}
	return out
}

// rectF is a box on the screen, native px, half-open as image.Rectangle.
type rectF struct{ x0, y0, x1, y1 float64 }

func (r rectF) w() float64                 { return r.x1 - r.x0 }
func (r rectF) h() float64                 { return r.y1 - r.y0 }
func (r rectF) centre() (float64, float64) { return (r.x0 + r.x1) / 2, (r.y0 + r.y1) / 2 }

// scaled is frame f's Box drawn k native px per art pixel with its origin
// at (x,y).
func scaled(f *shp.Frame, k, x, y float64) rectF {
	return rectF{x + k*float64(f.Box.Min.X-f.Center.X), y + k*float64(f.Box.Min.Y-f.Center.Y),
		x + k*float64(f.Box.Max.X-f.Center.X), y + k*float64(f.Box.Max.Y-f.Center.Y)}
}

// shipBox is frame f's drawn box at origin (x,y) for screen radius r,
// as Render draws it: 2r wide.
func shipBox(f *shp.Frame, x, y, r float64) rectF {
	return scaled(f, r/(float64(f.Box.Dx())/2), x, y)
}

// blastAt is a blast's scale and origin over a ship drawn in box ship:
// frame 0's Box, centred on it, scaled so its inscribed ellipse holds the
// ship box's corners, k = hypot(w/Dx, h/Dy) (≤ 4% of a hull showing; a
// box fit leaves up to 18%). Later frames bloom to 4.6x frame 0's width.
func blastAt(ship rectF, frames []*shp.Frame) (k, x, y float64) {
	f0 := frames[0]
	k = math.Hypot(ship.w()/float64(f0.Box.Dx()), ship.h()/float64(f0.Box.Dy()))
	cx, cy := ship.centre()
	bx := float64(f0.Box.Min.X+f0.Box.Max.X)/2 - float64(f0.Center.X)
	by := float64(f0.Box.Min.Y+f0.Box.Max.Y)/2 - float64(f0.Center.Y)
	return k, cx - k*bx, cy - k*by
}

// impactAt is an impact's scale and origin over box ship: its frames'
// largest extent spans the ship's larger side, centred.
func impactAt(ship rectF, frames []*shp.Frame) (k, x, y float64) {
	ext := 0
	for _, f := range frames {
		ext = max(ext, f.Box.Dx(), f.Box.Dy())
	}
	x, y = ship.centre()
	if ext == 0 {
		return 0, x, y
	}
	return max(ship.w(), ship.h()) / float64(ext), x, y
}

// razeAt is a razing's scale and origin at a planet projected at pt,
// world radius r: impactAt over its orbit ring's box.
func razeAt(pt cam.Point, r float64, frames []*shp.Frame) (k, x, y float64) {
	rr := (r + 0.06) * pt.Scale
	return impactAt(rectF{pt.X - rr, pt.Y - rr, pt.X + rr, pt.Y + rr}, frames)
}

// drawnBox is body b's sprite box this frame; false for an empty frame.
func (s *systemScreen) drawnBox(a *app, b *body) (rectF, bool) {
	sp := a.spr(b.sprite())
	if sp.Frame == nil || sp.Frame.Empty() {
		return rectF{}, false
	}
	pt := s.cam.Project(b.pos)
	return shipBox(sp.Frame, pt.X, pt.Y, b.radius()*pt.Scale), true
}

// drawnCentre is a bullet's end: b's drawn box centre, else w projected.
func (s *systemScreen) drawnCentre(a *app, b *body, w game.Vec3) (float64, float64) {
	if b != nil {
		if box, ok := s.drawnBox(a, b); ok {
			return box.centre()
		}
	}
	pt := s.cam.Project(w)
	return pt.X, pt.Y
}

// drawEffect draws e's frame, dimmed with depth: a bullet at fxPixel
// (pt.Scale is already native: no zoom) between its ends' centres, an
// impact, blast or razing sized on its body, else fxPixel at its point.
func (s *systemScreen) drawEffect(a *app, sc *ebiten.Image, e placedEffect) {
	frame := e.step
	if e.kind == fxBullet {
		frame %= a.frames(e.bank)
	}
	sp := a.spr(e.bank, frame)
	if sp.Image == nil {
		return
	}
	k, x, y := fxPixel*e.pt.Scale, e.pt.X, e.pt.Y
	if e.kind == fxBullet {
		px, py := s.drawnCentre(a, e.from, e.fromPos)
		qx, qy := s.drawnCentre(a, e.ship, e.toPos)
		x, y = px+(qx-px)*e.f, py+(qy-py)*e.f
	} else if e.ship != nil {
		b, err := a.a.Bank(e.bank)
		if err != nil {
			log.Fatal(err)
		}
		if e.kind == fxRaze {
			k, x, y = razeAt(s.cam.Project(e.ship.pos), e.ship.radius(), b.Frames)
		} else if box, ok := s.drawnBox(a, e.ship); ok {
			if e.kind == fxImpact {
				k, x, y = impactAt(box, b.Frames)
			} else {
				k, x, y = blastAt(box, b.Frames)
			}
		}
	}
	f := float32(fade(e.pt.Depth, s.cam.Dist))
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	op.ColorScale.Scale(f, f, f, 1)
	sp.Draw(sc, &op)
}

// fxBanks is every effect bank the system view draws (the replay's, the
// lane flare's) and the frame count its timing assumes (0: a bullet,
// looped).
func fxBanks() map[string]int {
	m := map[string]int{shieldBank: shieldFrames, destroyBank: destroyFrames, blueExitBank: exitFrames, redExitBank: exitFrames}
	for g := range game.NumGizmos {
		if game.Gizmos[g].Cat == game.Weapon {
			m[bulletBank(g)], m[explBank(g)] = 0, explFrames
		}
	}
	return m
}
