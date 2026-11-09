package bot

import (
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// The bot's ship specials (docs/rules.md "The bot's specials"), used
// through Empire.UseSpecial: a ship's own state is known, the hostiles'
// only through their Sightings. A refusal costs nothing and is dropped.

// specialRule is one self-targeted special's use rule on ship s under
// fire: hs the hostiles seen at its star (non-empty), dmg their
// theirSide damage.
type specialRule struct {
	g    game.Gizmo
	want func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) bool
}

// specialRules is the round's self-targeted specials, in the order they
// are tried: the Recaller first (away from home, it leaves: nothing else
// is used), the Accutron before any shot (the range doubles), the
// Toroidal Blaster last (a burst when out of energy). Every rule reads
// the state after the uses before it (Hyperfuel's energy stops the
// Cannibalizer).
var specialRules = []specialRule{
	{game.Recaller, func(b *Bot, s *game.Starship, _ []game.Sighting, dmg float64, cg Combat) bool {
		return dmg > 0 && hullAtMost(s, cg.RecallAt) && s.Star != b.E.HomeStar()
	}},
	{game.Invulnerablizer, func(_ *Bot, s *game.Starship, _ []game.Sighting, dmg float64, cg Combat) bool {
		return dmg > 0 && hullAtMost(s, cg.ShieldAt)
	}},
	{game.Accutron, func(_ *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) bool {
		return s.Design.Weapons() > 0
	}},
	{game.Replenisher, func(_ *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) bool {
		return weaponsSpent(s)
	}},
	{game.Hyperfuel, func(_ *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) bool {
		return lowEnergy(s)
	}},
	{game.Cannibalizer, func(_ *Bot, s *game.Starship, _ []game.Sighting, _ float64, cg Combat) bool {
		return lowEnergy(s) && float64(s.Hull) > cg.CannibalAt*float64(s.MaxHull())
	}},
	{game.ToroidalBlaster, func(_ *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) bool {
		return s.Power < s.Design.MoveCost() && drives(s) >= 2
	}},
}

// targetRule is one targeted or area special's rule: the Target to use it
// at, false none.
type targetRule struct {
	g    game.Gizmo
	pick func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool)
}

// partsExploitAt is the fewest parts known worth a Moving Part Exploiter
// (one hull point a part).
const partsExploitAt = 5

// targetRules is the round's targeted and area specials, tried in this
// order after specialRules. Most aim at the strongest mayBeArmed hostile
// afloat within the gizmo's range (strongestArmed), some with a further
// test: Brunswik its energy unknown or > 0, Shield Blaster its shields
// unknown or on, Specialty Blaster a Special known, Positron Bouncer its
// threatReach covering s, Fleet Disperser and Mass Condensor disperseAt
// others near it (the Condensor pulls s too: no ship of mine there). The
// others:
//
//	Backfirer, Part Exploiter  the most weapon (any) parts known, at least 1 (partsExploitAt)
//	Tie Down, Containment      the nearest carrier (a colony ship, an invader); the Tie Down at a colony star of mine
//	Tractor Beam               the nearest carrier beyond my longestRange (pulled into my guns)
//	Smart Bomb                 at least SmartBombAt hostiles within its blast
//	Self Destructotron         their dmg ≥ my Hull, no escape, a hostile in range
//	Hyperswapper               their dmg ≥ my Hull, no escape: the swap leaving me the least threatAt, below today's
//	Gravimetric Catapult       their dmg > 0, no escape, threatAt −Pos below threatAt Pos
//	Sacrificial Orb            hull to spare: the most damaged mate in range at Hull ≤ ShieldAt × MaxHull
//	Plasma Coupler             energy to spare: the mate in range of least energy, short of its MoveCost
//	Phase Bomb                 the nearest planet in range of an empire at War
//	Gravity Distorter          disperseAt hostiles near s, no other ship of mine there
//	Gravimetric Condensor      disperseAt hostiles in the shell it pulls, none of mine within it
var targetRules = []targetRule{
	{game.Disintegrator, strongestArmed(game.Disintegrator, nil)},
	{game.BrunswikDissipator, strongestArmed(game.BrunswikDissipator, func(h game.Sighting) bool { return h.Scan == nil || h.Scan.Energy > 0 })},
	{game.IntellectScrambler, strongestArmed(game.IntellectScrambler, nil)},
	{game.ShieldBlaster, strongestArmed(game.ShieldBlaster, func(h game.Sighting) bool { return h.Scan == nil || h.Scan.Shields })},
	{game.Disarmer, strongestArmed(game.Disarmer, nil)},
	{game.Backfirer, mostKnown(game.Backfirer, 1, func(g game.Gizmo) bool { return game.Gizmos[g].Cat == game.Weapon })},
	{game.MovingPartExploiter, mostKnown(game.MovingPartExploiter, partsExploitAt, func(game.Gizmo) bool { return true })},
	{game.Gizmogrifier, strongestArmed(game.Gizmogrifier, nil)},
	{game.SpecialtyBlaster, strongestArmed(game.SpecialtyBlaster, func(h game.Sighting) bool { return knownParts(h, isSpecial) > 0 })},
	{game.MolecularTieDown, func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		if !b.colonyAt(s.Star) {
			return game.Target{}, false
		}
		return nearestCarrier(s, hs, game.MolecularTieDown, -1)
	}},
	{game.ContainmentDevice, func(_ *Bot, s *game.Starship, hs []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		return nearestCarrier(s, hs, game.ContainmentDevice, -1)
	}},
	{game.MyrmidonicCarbonizer, strongestArmed(game.MyrmidonicCarbonizer, nil)},
	{game.SmartBomb, func(_ *Bot, s *game.Starship, hs []game.Sighting, _ float64, cg Combat) (game.Target, bool) {
		r := float64(game.Gizmos[game.SmartBomb].Special1) * game.RangeUnit
		return game.Target{}, float64(len(within(s, hs, r))) >= cg.SmartBombAt
	}},
	{game.SelfDestructotron, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		ok := dmg >= float64(s.Hull) && len(within(s, hs, gizmoReach(game.SelfDestructotron))) > 0 && !b.canEscape(s, hs, cg)
		return game.Target{}, ok
	}},
	{game.SacrificialOrb, func(b *Bot, s *game.Starship, _ []game.Sighting, _ float64, cg Combat) (game.Target, bool) {
		if float64(s.Hull-game.Gizmos[game.SacrificialOrb].Special1) <= cg.ShieldAt*float64(s.MaxHull()) {
			return game.Target{}, false
		}
		var t *game.Starship
		for _, o := range b.mates(s, game.SacrificialOrb) {
			if hullAtMost(o, cg.ShieldAt) && (t == nil || o.MaxHull()-o.Hull > t.MaxHull()-t.Hull) {
				t = o
			}
		}
		return game.Target{Kind: game.ShipTarget, Mine: t}, t != nil
	}},
	{game.PlasmaCoupler, func(b *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		if s.Power < 2*s.Design.MoveCost()+game.Gizmos[game.PlasmaCoupler].Power {
			return game.Target{}, false
		}
		var t *game.Starship
		for _, o := range b.mates(s, game.PlasmaCoupler) {
			if o.Power < o.Design.MoveCost() && (t == nil || o.Power < t.Power) {
				t = o
			}
		}
		return game.Target{Kind: game.ShipTarget, Mine: t}, t != nil
	}},
	{game.PhaseBomb, func(b *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		ps := b.E.Cluster.Stars[s.Star].System.Planets
		r, pl, pd := gizmoReach(game.PhaseBomb), -1, 0.0
		for k, p := range ps { // every planet of an empire at War (the game's rule), defended or not
			if p.Owner < 0 || p.Owner == b.E.Index || b.E.StanceWith(p.Owner) != game.War {
				continue
			}
			if d := dist(s.Pos, p.Pos); d <= r+1e-9 && (pl < 0 || d < pd) {
				pl, pd = k, d
			}
		}
		return game.Target{Kind: game.PlanetTarget, Index: pl}, pl >= 0
	}},
	{game.PositronBouncer, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		hits := func(h game.Sighting) bool { return threatReach(h, cg) >= dist(h.Pos, s.Pos)-1e-9 }
		return strongestArmed(game.PositronBouncer, hits)(b, s, hs, dmg, cg)
	}},
	{game.GizmoTractorBeam, func(_ *Bot, s *game.Starship, hs []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		lr := longestRange(s)
		if lr <= 0 {
			return game.Target{}, false
		}
		return nearestCarrier(s, hs, game.GizmoTractorBeam, lr)
	}},
	{game.Hyperswapper, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		if dmg < float64(s.Hull) || b.canEscape(s, hs, cg) {
			return game.Target{}, false
		}
		best, bestT := -1, threatAt(b, cg, s.Pos, hs)
		if bestT <= 0 {
			return game.Target{}, false
		}
		r := gizmoReach(game.Hyperswapper)
		for i, h := range hs {
			if !inRangeOf(s, h, r) {
				continue
			}
			after := slices.Clone(hs) // the swap: h at my Pos, me at its
			after[i].Pos = s.Pos
			if t := threatAt(b, cg, h.Pos, after); t < bestT {
				best, bestT = i, t
			}
		}
		if best < 0 {
			return game.Target{}, false
		}
		return game.Target{Kind: game.ShipTarget, Ship: hs[best]}, true
	}},
	{game.FleetDisperser, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		r := specialReach(game.FleetDisperser, 1)
		crowded := func(h game.Sighting) bool { return others(hs, h, r) >= disperseAt }
		return strongestArmed(game.FleetDisperser, crowded)(b, s, hs, dmg, cg)
	}},
	{game.GravityDistorter, func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		r := specialReach(game.GravityDistorter, 1)
		return game.Target{}, len(within(s, hs, r)) >= disperseAt && !b.mineInSpace(s.Star, s.Pos, r, s)
	}},
	{game.MassCondensor, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		r := specialReach(game.MassCondensor, 1)
		bunched := func(h game.Sighting) bool {
			return others(hs, h, r) >= disperseAt && !b.mineInSpace(s.Star, h.Pos, r, nil)
		}
		return strongestArmed(game.MassCondensor, bunched)(b, s, hs, dmg, cg)
	}},
	{game.GravimetricCondensor, func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		r1, r2 := specialReach(game.GravimetricCondensor, 1), specialReach(game.GravimetricCondensor, 2)
		n := 0
		for _, h := range hs {
			if d := dist(h.Pos, game.Vec3{}); afloat(h) && d <= r1+1e-9 && d > r2+1e-9 {
				n++
			}
		}
		return game.Target{}, n >= disperseAt && !b.mineInSpace(s.Star, game.Vec3{}, r1, nil)
	}},
	{game.GravimetricCatapult, func(b *Bot, s *game.Starship, hs []game.Sighting, dmg float64, cg Combat) (game.Target, bool) {
		if dmg <= 0 || b.canEscape(s, hs, cg) {
			return game.Target{}, false
		}
		mirror := game.Vec3{X: -s.Pos.X, Y: -s.Pos.Y, Z: -s.Pos.Z}
		return game.Target{}, threatAt(b, cg, mirror, hs) < threatAt(b, cg, s.Pos, hs)
	}},
}

// disperseAt is the fewest hostiles an area special must move.
const disperseAt = 2

// laneRules is the round's lane specials, tried after targetRules, Target
// a LaneTarget (Index the ENDPOINT in System.Lanes):
//
//	Lane Magnetron     fleeing with a lane drive: the lane of the GoToStar kept, else of the refuge, in range
//	Lane Destabilizer  never: LaneTraffic tells neither side nor way (it could hurry a foe in)
var laneRules = []targetRule{
	{game.LaneMagnetron, func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, cg Combat) (game.Target, bool) {
		if s.Design.LaneDrives() == 0 || !b.fleeing(s, hs, cg) {
			return game.Target{}, false
		}
		v := b.refuge(s)
		if s.Order.Kind == game.GoToStar {
			v = s.Order.Star
		}
		for _, l := range laneEndpoints(b.E.Cluster, s, game.LaneMagnetron) {
			if l.far == v {
				return game.Target{Kind: game.LaneTarget, Index: l.k}, true
			}
		}
		return game.Target{}, false
	}},
}

// quietLaneRules is the quiet-day lane pass (hs nil, dmg 0), the first
// endpoint in range:
//
//	Lane Blocker   at my colony star, atWar: toward no colony of mine, not blocked by me today
//	Lane Endoscope toward a star I do not Know
var quietLaneRules = []targetRule{
	{game.LaneBlocker, func(b *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		if !b.colonyAt(s.Star) || !b.atWar() {
			return game.Target{}, false
		}
		for _, l := range laneEndpoints(b.E.Cluster, s, game.LaneBlocker) {
			if !b.colonyAt(l.far) && b.E.Day >= b.E.LaneBlocks[l.lane] {
				return game.Target{Kind: game.LaneTarget, Index: l.k}, true
			}
		}
		return game.Target{}, false
	}},
	{game.LaneEndoscope, func(b *Bot, s *game.Starship, _ []game.Sighting, _ float64, _ Combat) (game.Target, bool) {
		for _, l := range laneEndpoints(b.E.Cluster, s, game.LaneEndoscope) {
			if !b.E.Knows(l.far) {
				return game.Target{Kind: game.LaneTarget, Index: l.k}, true
			}
		}
		return game.Target{}, false
	}},
}

// laneEnd is one lane endpoint of a star: k its index in System.Lanes (a
// LaneTarget's Index), lane the cluster lane (LaneBlocks' key), far the
// star it leads to.
type laneEnd struct{ k, lane, far int }

// laneEndpoints is the endpoints of s's star within g's reach of s, in
// System.Lanes order.
func laneEndpoints(c *game.Cluster, s *game.Starship, g game.Gizmo) []laneEnd {
	var out []laneEnd
	r := gizmoReach(g)
	for k, l := range c.Stars[s.Star].System.Lanes {
		if dist(s.Pos, l.Pos) <= r+1e-9 {
			li := c.Stars[s.Star].Lanes[k]
			out = append(out, laneEnd{k, li, c.Other(li, s.Star)})
		}
	}
	return out
}

// specialReach is g's Special1 (n 1) or Special2 (n 2) × RangeUnit.
func specialReach(g game.Gizmo, n int) float64 {
	if n == 2 {
		return float64(game.Gizmos[g].Special2) * game.RangeUnit
	}
	return float64(game.Gizmos[g].Special1) * game.RangeUnit
}

// threatAt is the summed damage of the hostiles of hs whose threatReach
// covers p.
func threatAt(b *Bot, cg Combat, p game.Vec3, hs []game.Sighting) float64 {
	t := 0.0
	for _, h := range hs {
		if dist(p, h.Pos) <= threatReach(h, cg)+1e-9 {
			t += sightingSide(b.G, cg, h).dmg
		}
	}
	return t
}

// others counts the afloat hostiles of hs within r of h, h excluded.
func others(hs []game.Sighting, h game.Sighting, r float64) int {
	n := 0
	for _, x := range hs {
		if x != h && afloat(x) && dist(h.Pos, x.Pos) <= r+1e-9 {
			n++
		}
	}
	return n
}

// mineInSpace: a ship of mine but not in space at star within r of c.
func (b *Bot) mineInSpace(star int, c game.Vec3, r float64, not *game.Starship) bool {
	return slices.ContainsFunc(b.E.Ships, func(o *game.Starship) bool {
		return o != not && o.Star == star && o.InSpace() && dist(c, o.Pos) <= r+1e-9
	})
}

// fleeing is Round's flee test for s at its star.
func (b *Bot) fleeing(s *game.Starship, hs []game.Sighting, cg Combat) bool {
	their := b.theirSide(s.Star, hs, cg)
	if !armed(s.Design) {
		return their.dmg > 0
	}
	return pressure(their, b.mine(s.Star)) > cg.FleeRatio && b.canEscape(s, hs, cg)
}

// gizmoReach is g's Range × RangeUnit, in Pos units.
func gizmoReach(g game.Gizmo) float64 { return float64(game.Gizmos[g].Range) * game.RangeUnit }

// within is the hostiles of hs inRangeOf s, in hs order.
func within(s *game.Starship, hs []game.Sighting, r float64) []game.Sighting {
	var out []game.Sighting
	for _, h := range hs {
		if inRangeOf(s, h, r) {
			out = append(out, h)
		}
	}
	return out
}

// inRangeOf: h is afloat within r of s.
func inRangeOf(s *game.Starship, h game.Sighting, r float64) bool {
	return afloat(h) && dist(s.Pos, h.Pos) <= r+1e-9
}

// afloat: h is in space, its scanned hull points (when scanned) above 0.
func afloat(h game.Sighting) bool { return inSpace(h) && (h.Scan == nil || h.Scan.HullPts > 0) }

// knownArmed: h's parts are known and a weapon is among them.
func knownArmed(h game.Sighting) bool {
	return h.Scan != nil && h.Scan.Parts && h.Scan.Design.Weapons() > 0
}

// mayBeArmed: knownArmed, or its parts unknown and its hull's HullThreat
// above 0.
func mayBeArmed(h game.Sighting, cg Combat) bool {
	if h.Scan != nil && h.Scan.Parts {
		return knownArmed(h)
	}
	return cg.HullThreat[h.Hull] > 0
}

// knownParts counts the parts known aboard h for which is holds.
func knownParts(h game.Sighting, is func(game.Gizmo) bool) int {
	if h.Scan == nil || !h.Scan.Parts {
		return 0
	}
	n := 0
	for _, g := range h.Scan.Design.Parts {
		if is(g) {
			n++
		}
	}
	return n
}

func isSpecial(g game.Gizmo) bool { return game.Gizmos[g].Cat == game.Special }

// stronger orders x before y: the greater sightingSide damage, ties the
// fewer hull points, then the nearer to s (false on a full tie).
func stronger(b *Bot, s *game.Starship, cg Combat, x, y game.Sighting) bool {
	sx, sy := sightingSide(b.G, cg, x), sightingSide(b.G, cg, y)
	switch {
	case sx.dmg != sy.dmg:
		return sx.dmg > sy.dmg
	case sx.hp != sy.hp:
		return sx.hp < sy.hp
	}
	return dist(s.Pos, x.Pos) < dist(s.Pos, y.Pos)
}

// strongestArmed is the rule "the strongest mayBeArmed hostile within g's
// reach of s for which also holds (nil: any)".
func strongestArmed(g game.Gizmo, also func(game.Sighting) bool) func(*Bot, *game.Starship, []game.Sighting, float64, Combat) (game.Target, bool) {
	return func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, cg Combat) (game.Target, bool) {
		var t *game.Sighting
		for _, h := range within(s, hs, gizmoReach(g)) {
			if mayBeArmed(h, cg) && (also == nil || also(h)) && (t == nil || stronger(b, s, cg, h, *t)) {
				t = &h
			}
		}
		return shipTarget(t)
	}
}

// shipTarget is the ShipTarget at *t, false when t is nil.
func shipTarget(t *game.Sighting) (game.Target, bool) {
	if t == nil {
		return game.Target{}, false
	}
	return game.Target{Kind: game.ShipTarget, Ship: *t}, true
}

// mostKnown is the rule "the hostile within g's reach of s with the most
// known parts is holds for, at least least", ties as stronger.
func mostKnown(g game.Gizmo, least int, is func(game.Gizmo) bool) func(*Bot, *game.Starship, []game.Sighting, float64, Combat) (game.Target, bool) {
	return func(b *Bot, s *game.Starship, hs []game.Sighting, _ float64, cg Combat) (game.Target, bool) {
		var t *game.Sighting
		tn := 0
		for _, h := range within(s, hs, gizmoReach(g)) {
			n := knownParts(h, is)
			if n >= least && (t == nil || n > tn || n == tn && stronger(b, s, cg, h, *t)) {
				t, tn = &h, n
			}
		}
		return shipTarget(t)
	}
}

// nearestCarrier is the nearest hostile within g's reach of s, farther
// than beyond (−1: any), not knownArmed, its cargo unknown or above 0,
// ties hs order.
func nearestCarrier(s *game.Starship, hs []game.Sighting, g game.Gizmo, beyond float64) (game.Target, bool) {
	var t *game.Sighting
	td := 0.0
	for _, h := range within(s, hs, gizmoReach(g)) {
		if d := dist(s.Pos, h.Pos); d > beyond+1e-9 && !knownArmed(h) && (h.Scan == nil || h.Scan.Cargo > 0) && (t == nil || d < td) {
			t, td = &h, d
		}
	}
	return shipTarget(t)
}

// mates is my other ships at s's star out of the lanes (orbit allowed)
// within g's reach of s, in Ships order.
func (b *Bot) mates(s *game.Starship, g game.Gizmo) []*game.Starship {
	var out []*game.Starship
	r := gizmoReach(g)
	for _, o := range b.E.Ships {
		if o != s && o.Star == s.Star && o.Lane < 0 && dist(s.Pos, o.Pos) <= r+1e-9 {
			out = append(out, o)
		}
	}
	return out
}

// specials uses the specials of my ships at star (out of the lanes, not
// being refitted), in Ships order over a snapshot (a self-destruct or a
// blast shrinks Ships): specialRules, targetRules, then laneRules, each
// once, with the first usablePart of the gizmo. After a use the hostiles
// and their damage are read again and the ship's turn ends once it is
// no longer here; no hostile left, nothing more is used. It returns the
// hostiles as last read, so Round never aims at a ship killed here.
func (b *Bot) specials(star int, hs []game.Sighting, cg Combat) []game.Sighting {
	if len(hs) == 0 {
		return hs
	}
	dmg := b.theirSide(star, hs, cg).dmg
	used := func(s *game.Starship, reps []game.Report) bool {
		b.news = append(b.news, reps...)
		hs = b.G.HostilesAll(b.E, star)
		if len(hs) > 0 {
			dmg = b.theirSide(star, hs, cg).dmg
		}
		return here(b.E, s, star)
	}
	for _, s := range slices.Clone(b.E.Ships) {
		if len(hs) == 0 {
			return hs
		}
		if !here(b.E, s, star) || b.E.Refitting(s) != nil {
			continue
		}
		for _, r := range specialRules {
			if k := usablePart(s, r.g); k >= 0 && r.want(b, s, hs, dmg, cg) {
				if reps, err := b.E.UseSpecial(s, k, game.Target{}); err == nil && !used(s, reps) {
					break
				}
			}
		}
		// false: s's turn ended or no hostile is left.
		try := func(rs []targetRule) bool {
			for _, r := range rs {
				if len(hs) == 0 || !here(b.E, s, star) {
					return false
				}
				k := usablePart(s, r.g)
				if k < 0 {
					continue
				}
				if t, ok := r.pick(b, s, hs, dmg, cg); ok {
					if reps, err := b.E.UseSpecial(s, k, t); err == nil && !used(s, reps) {
						return false
					}
				}
			}
			return true
		}
		if here(b.E, s, star) && try(targetRules) {
			try(laneRules)
		}
	}
	return hs
}

// lanes is the quiet-day lane pass: each quiet ship tries quietLaneRules
// once each. Without G, nothing.
func (b *Bot) lanes() {
	if b.G == nil {
		return
	}
	for _, s := range b.E.Ships {
		if !b.quiet(s) {
			continue
		}
		for _, r := range quietLaneRules {
			k := usablePart(s, r.g)
			if k < 0 {
				continue
			}
			if t, ok := r.pick(b, s, nil, 0, b.genes()); ok {
				b.say(b.E.UseSpecial(s, k, t))
			}
		}
	}
}

// quiet: s is out of the lanes, not being refitted, at a star with no
// hostile seen.
func (b *Bot) quiet(s *game.Starship) bool {
	return s.Lane < 0 && b.E.Refitting(s) == nil && len(b.G.HostilesAll(b.E, s.Star)) == 0
}

// here: s is one of e's ships, at star out of the lanes.
func here(e *game.Empire, s *game.Starship, star int) bool {
	return s.Star == star && s.Lane < 0 && slices.Contains(e.Ships, s)
}

// repairs is the quiet-day pass: each quiet damaged ship uses its Remote
// Repair Facilities while one is usable and the use raises the hull and
// spends energy (else it would loop). Without G, nothing.
func (b *Bot) repairs() {
	if b.G == nil {
		return
	}
	for _, s := range b.E.Ships {
		if s.Hull >= s.MaxHull() || !b.quiet(s) {
			continue
		}
		for s.Hull < s.MaxHull() {
			k := usablePart(s, game.RemoteRepairFacility)
			if k < 0 {
				break
			}
			hull, power := s.Hull, s.Power
			reps, err := b.E.UseSpecial(s, k, game.Target{})
			b.news = append(b.news, reps...)
			if err != nil || s.Hull <= hull || s.Power >= power {
				break
			}
		}
	}
}

// theirSide is the hostiles' side at star as Round reads it: hs's
// estimates plus planetSide.
func (b *Bot) theirSide(star int, hs []game.Sighting, cg Combat) side {
	var their side
	for _, sg := range hs {
		their = their.plus(sightingSide(b.G, cg, sg))
	}
	return their.plus(b.planetSide(star))
}

// usablePart is the first part of s that is g and Usable says "use", −1
// none.
func usablePart(s *game.Starship, g game.Gizmo) int {
	for k, p := range s.Design.Parts {
		if p != g {
			continue
		}
		if ok, act := s.Usable(k); ok && act == "use" {
			return k
		}
	}
	return -1
}

// drives is the number of Drive parts aboard s.
func drives(s *game.Starship) int {
	n := 0
	for _, g := range s.Design.Parts {
		if game.Gizmos[g].Cat == game.Drive {
			n++
		}
	}
	return n
}

// hullAtMost: Hull ≤ share × MaxHull.
func hullAtMost(s *game.Starship, share float64) bool {
	return float64(s.Hull) <= share*float64(s.MaxHull())
}

// weaponsSpent: s has weapons of limited Uses, none with a shot left.
func weaponsSpent(s *game.Starship) bool {
	n := 0
	for k, g := range s.Design.Parts {
		if w := &game.Gizmos[g]; w.Cat == game.Weapon && w.Uses > 0 {
			if shotLeft(s, k, w) {
				return false
			}
			n++
		}
	}
	return n > 0
}

// lowEnergy: s's energy is short of a move plus its cheapest shot.
func lowEnergy(s *game.Starship) bool {
	cheap := -1
	for _, g := range s.Design.Parts {
		if w := &game.Gizmos[g]; w.Cat == game.Weapon && (cheap < 0 || w.Power < cheap) {
			cheap = w.Power
		}
	}
	return s.Power < s.Design.MoveCost()+max(cheap, 0)
}
