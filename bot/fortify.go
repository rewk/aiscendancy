package bot

import (
	"errors"
	"math"
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// fortify starts col's next defence, only while my empire is at War with
// some empire and col's Pop is at least ShieldPop, in this order, each
// step while its target is short: surface shields up to ShieldWorth
// (game.ShieldWorth summed; the Mega when known, else the plain one, at
// its BestCell), orbital shields up to OrbitalShields (the Mega when
// known), orbital weapons up to OrbitalGuns (bestGun), Orbital Cloakers
// up to OrbitalCloakers, Surface Cloakers up to SurfaceCloakers. All
// counted on col.Planned(): what stands, the running project and the
// queue (a shield razed or a slot destroyed is gone, rebuilt); the
// shields and cloakers by item, the guns by OrbitalDefence.Weapon. A step that cannot
// start (unknown, no square or slot) falls through to the next; short
// of free people it waits (true) while col can still grow, else falls
// through (at Capacity the wait would last for ever). True when it
// started one or waits.
func (b *Bot) fortify(col *game.Colony) bool {
	cg := b.cg
	if !b.stanced(game.War) || col.Pop < int(math.Round(cg.ShieldPop)) {
		return false
	}
	r, p := b.E.Research, col.Planned()
	worth, shields, guns, ocloaks, scloaks := 0, 0, 0, 0, 0
	for _, sq := range p.Surface {
		worth += game.ShieldWorth[sq.Item]
		if sq.Item == game.SurfaceCloaker {
			scloaks++
		}
	}
	for _, it := range p.Orbit {
		switch d, _ := it.Defence(); {
		case it == game.OrbitalShield, it == game.OrbitalMegaShield:
			shields++
		case it == game.OrbitalCloaker:
			ocloaks++
		case d.Weapon():
			guns++
		}
	}
	try := func(it game.Item) (done bool) {
		if it == game.NoItem || it.Orbital() && !slices.ContainsFunc(col.Planet.Orbit[:], func(o game.Item) bool { return o == game.NoItem }) {
			return false // no slot: never wait for people
		}
		err := Try(col, r, it)
		return err == nil || errors.Is(err, game.ErrNoFreePop) && col.Pop < col.Capacity()
	}
	if worth < int(math.Round(cg.ShieldWorth)) && try(knownOf(r, game.SurfaceMegaShield, game.SurfaceShield)) {
		return true
	}
	if shields < int(math.Round(cg.OrbitalShields)) && try(knownOf(r, game.OrbitalMegaShield, game.OrbitalShield)) {
		return true
	}
	if guns < int(math.Round(cg.OrbitalGuns)) && try(bestGun(r)) {
		return true
	}
	if ocloaks < int(math.Round(cg.OrbitalCloakers)) && try(knownOf(r, game.OrbitalCloaker)) {
		return true
	}
	return scloaks < int(math.Round(cg.SurfaceCloakers)) && try(knownOf(r, game.SurfaceCloaker))
}

// knownOf is the first of items r allows, NoItem when none.
func knownOf(r *game.Research, items ...game.Item) game.Item {
	for _, it := range items {
		if r.Allows(it) {
			return it
		}
	}
	return game.NoItem
}

// bestGun is the orbital weapon r allows of most Damage × Shots, then
// Range, then the lower index (game.OrbitalDefences); NoItem when none.
func bestGun(r *game.Research) game.Item {
	best := game.NoItem
	var bd game.OrbitalDefence
	for it := game.Item(0); it < game.NumItems; it++ {
		d, ok := it.Defence()
		if !ok || !d.Weapon() || !r.Allows(it) {
			continue
		}
		if best == game.NoItem || d.Damage*d.Shots > bd.Damage*bd.Shots ||
			d.Damage*d.Shots == bd.Damage*bd.Shots && d.Range > bd.Range {
			best, bd = it, d
		}
	}
	return best
}
