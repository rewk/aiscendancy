package game

import "slices"

// The orbital guard (battle.txt info24: "Planets which have ships or
// defenses in orbit are protected from invasion ... Foreign ships are
// prevented from orbiting a planet that has such orbital protection").

// guardOf is who guards planet pl of star among es: an owned planet its
// owner alone, with an orbital shield or one of its ships in orbit (a
// foreign ship there guards nothing); an unowned one the first
// ship in its orbit. ok false when nobody does.
func guardOf(c *Cluster, es []*Empire, star, pl int) (empire int, ok bool) {
	p := c.Stars[star].System.Planets[pl]
	if p.Owner >= 0 {
		if orbitShielded(p) {
			return p.Owner, true
		}
		return inOrbit(es, star, pl, func(e *Empire) bool { return e.Index != p.Owner })
	}
	return inOrbit(es, star, pl, nil)
}

// orbitShielded reports an Orbital Shield or Mega Shield in p's orbit.
func orbitShielded(p *Planet) bool {
	return slices.ContainsFunc(p.Orbit[:], func(it Item) bool { return it == OrbitalShield || it == OrbitalMegaShield })
}

// inOrbit is the empire of the first ship in orbit of planet pl of star,
// es then Ships order, out of the lanes, the empires skip reports left
// out (nil: none); ok false when none.
func inOrbit(es []*Empire, star, pl int, skip func(*Empire) bool) (empire int, ok bool) {
	for _, e := range es {
		if skip != nil && skip(e) {
			continue
		}
		for _, s := range e.Ships {
			if s.Star == star && s.Lane < 0 && s.Orbit == pl {
				return e.Index, true
			}
		}
	}
	return -1, false
}

// Guard is who guards planet pl of star (guardOf over the game's
// empires): every other empire is refused its orbit, whatever the stance.
func (g *Game) Guard(star, pl int) (empire int, ok bool) {
	return guardOf(g.Cluster, g.Empires, star, pl)
}

// peerList is the empires e shares its cluster with, e alone outside a
// Game.
func (e *Empire) peerList() []*Empire {
	if e.peers == nil {
		return []*Empire{e}
	}
	return e.peers
}

// Barred reports planet pl of star guarded by another empire than e: its
// ships bounce off the orbit (Hop), the order kept, as in the original.
func (e *Empire) Barred(star, pl int) bool { return e.barredIn(e.peerList())(star, pl) }

// barredIn is Barred among es, read live: Hop's and Step's barred.
func (e *Empire) barredIn(es []*Empire) func(star, pl int) bool {
	return func(star, pl int) bool {
		g, ok := guardOf(e.Cluster, es, star, pl)
		return ok && g != e.Index
	}
}
