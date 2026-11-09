package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// The bot's invasions (docs/rules.md "The bot's combat policy", its
// Invasions): with contact, a shipyard builds an invasion ship sized for
// the nearest hostile colony (invasionDesign); the ship flies to the
// nearest one its modules beat and Invades (invade). Emptied, it idles
// like any unarmed ship.

// worth is the surface shields' worth of p against Invasion Modules, 0
// for a species whose invasions ignore shields (InvadesAlways).
func (b *Bot) worth(p *game.Planet) int {
	if b.E.Species.Traits().InvadesAlways() {
		return 0
	}
	w := 0
	for _, sq := range p.Surface {
		w += game.ShieldWorth[sq.Item]
	}
	return w
}

// modulesFor is the fewest Invasion Modules whose worth exceeds worth.
func modulesFor(worth int) int {
	return (worth + 1 + game.ModuleWorth - 1) / game.ModuleWorth
}

// invadable reports planet pl of star a colony I may invade as I see it:
// known, Hostile, no Orbital (Mega) Shield and no other empire's ship
// seen in its orbit. False without G.
func (b *Bot) invadable(star, pl int) bool {
	if b.G == nil || !b.E.Knows(star) {
		return false
	}
	p := b.E.Planet(star, pl)
	if p.Owner < 0 || p.Owner == b.E.Index || !b.G.Hostile(b.E, b.G.Empires[p.Owner]) {
		return false
	}
	for _, it := range p.Orbit {
		if it == game.OrbitalShield || it == game.OrbitalMegaShield {
			return false
		}
	}
	for _, sg := range b.G.ShipsSeenAll(b.E, star) {
		if sg.Orbit == pl {
			return false
		}
	}
	return true
}

// invasionTarget is the nearest planet ok accepts: at from first
// (nearest pos, ties planet order), else, with strength > 0
// (LaneStrength), at the nearest star search finds, its first in planet
// order; star -1 when none, first the path's first hop (-1 at from).
func (b *Bot) invasionTarget(from int, pos game.Vec3, strength float64, tr game.Traits, blocked func(v int) bool, ok func(star, pl int) bool) (star, pl, first int) {
	planets := func(st int) []*game.Planet { return b.E.Cluster.Stars[st].System.Planets }
	pl, best := -1, 0.0
	for k, p := range planets(from) {
		if d := dist(pos, p.Pos); ok(from, k) && (pl < 0 || d < best) {
			pl, best = k, d
		}
	}
	if pl >= 0 {
		return from, pl, -1
	}
	if strength == 0 {
		return -1, -1, -1
	}
	to, first := b.search(from, strength, tr, 0, blocked, func(v, _ int) bool {
		for k := range planets(v) {
			if ok(v, k) {
				pl = k
				return true
			}
		}
		return false
	})
	if to < 0 {
		return -1, -1, -1
	}
	return to, pl, first
}

// unarmedDanger is danger for an unarmed ship: the hostiles seen at v
// deal damage.
func (b *Bot) unarmedDanger(v int) bool { return b.their(v).dmg > 0 }

// invasionDesign is the invasion ship col's shipyard should start, ok
// false when none: only with contact and Invasion Modules allowed, while
// the loaded invasion ships flying and being built number fewer than
// ⌈InvadeShare × warships⌉ (counted as wantWarship), and col's star is
// out of danger, toward the nearest invadable colony (with a lane drive
// when allowed). It carries modulesFor + InvadeMargin (rounded) modules,
// fewer when the hull lacks slots, never fewer than modulesFor.
func (b *Bot) invasionDesign(col *game.Colony) (game.Design, bool) {
	r := b.E.Research
	if !b.contact || !r.AllowsGizmo(game.InvasionModule) {
		return game.Design{}, false
	}
	inv, war := 0, 0
	for _, s := range b.E.Ships {
		switch {
		case armed(s.Design):
			war++
		case s.Invaders > 0:
			inv++
		}
	}
	for _, o := range b.E.Colonies {
		if d := building(o); d != nil {
			switch {
			case armed(*d):
				war++
			case d.InvasionModules() > 0:
				inv++
			}
		}
	}
	if float64(inv) >= math.Ceil(b.cg.InvadeShare*float64(war)) || b.unarmedDanger(col.Star) {
		return game.Design{}, false
	}
	lane := r.AllowsGizmo(game.StarLaneDrive)
	strength := 0.0
	if lane {
		strength = 1
	}
	st, pl, _ := b.invasionTarget(col.Star, col.Planet.Pos, strength, col.Traits(), b.unarmedDanger, b.invadable)
	if st < 0 {
		return game.Design{}, false
	}
	need := modulesFor(b.worth(b.E.Planet(st, pl)))
	for n := need + int(math.Round(b.cg.InvadeMargin)); n >= need; n-- {
		if d, ok := game.InvasionShip(r, lane, n); ok {
			return d, true
		}
	}
	return game.Design{}, false
}

// invade moves a ship of mine carrying Invasion Modules, out of the
// lanes. A target is an invadable colony whose shields its modules beat,
// unclaimed. A GoToPlanet is cleared once its planet is no longer
// invadable or beaten: it never waits on a guard. Then, NoOrder: in orbit
// of a target, Invade; else GoToPlanet the nearest target of its system;
// else GoToStar the first hop toward the nearest star with one (avoiding
// danger), the star in dest. Any other order stands.
func (b *Bot) invade(s *game.Starship, claimed map[place]bool) {
	if s.Lane >= 0 || s.Invaders < 1 {
		return
	}
	beats := func(star, pl int) bool {
		return b.invadable(star, pl) && b.worth(b.E.Planet(star, pl)) < s.Invaders*game.ModuleWorth
	}
	if s.Order.Kind == game.GoToPlanet {
		if beats(s.Star, s.Order.Planet) {
			return
		}
		s.Order = game.Order{}
	}
	if s.Order.Kind != game.NoOrder {
		return
	}
	ok := func(star, pl int) bool { return !claimed[place{star, pl}] && beats(star, pl) }
	if s.Orbit >= 0 && ok(s.Star, s.Orbit) {
		if b.E.Order(s, game.Order{Kind: game.Invade}) == nil {
			claimed[place{s.Star, s.Orbit}] = true
		}
		return
	}
	st, pl, first := b.invasionTarget(s.Star, s.Pos, s.Design.LaneStrength(), s.Species.Traits(), b.danger(s), ok)
	switch {
	case st < 0:
	case first < 0:
		if b.E.Order(s, game.Order{Kind: game.GoToPlanet, Planet: pl}) == nil {
			claimed[place{st, pl}] = true
		}
	case b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first}) == nil:
		b.dest[s] = st
	}
}
