package bot

import "github.com/rewk/aiscendancy/game"

// tractor uses my colonies' Tractor Beams at star for this round (Round
// calls it before defend: a pull takes the colony's shots). Each colony
// there, in Colonies order, pulls while it has a pull left:
//  1. my first ship (Ships order) below RepairAt × MaxHull the beam
//     accepts (CanTractor: in space, in range, not alongside) and no
//     other colony of mine pulled this round: home (two beams would
//     tug it back and forth);
//  2. else the first hostile in space (hs order) that the pull
//     (TractorEnd) brings within range of an orbital weapon with a shot
//     left not reaching it now: another colony's of mine at star that has
//     not pulled this round, or this colony's own only when none of its
//     weapons reaches any hostile now (the pull costs its shots this
//     round: they fire the next).
//
// Else no pull. The hostiles come back read again after a pull (the
// sightings carry positions).
func (b *Bot) tractor(star int, hs []game.Sighting, cg Combat) []game.Sighting {
	var pulled map[*game.Starship]*game.Colony // my ships pulled, by whom
	for _, col := range b.E.Colonies {
		if col.Star != star {
			continue
		}
		for col.Tractors < col.Planet.Count(game.TractorBeam) {
			t, ok := b.tractorTarget(col, star, hs, cg, pulled)
			if !ok {
				break
			}
			if _, err := b.E.UseTractor(col, t); err != nil {
				break
			}
			if t.Mine != nil {
				if pulled == nil {
					pulled = map[*game.Starship]*game.Colony{}
				}
				pulled[t.Mine] = col
			}
			hs = b.G.HostilesAll(b.E, star)
		}
	}
	return hs
}

// tractorTarget is tractor's pick for col, ok false none.
func (b *Bot) tractorTarget(col *game.Colony, star int, hs []game.Sighting, cg Combat, pulled map[*game.Starship]*game.Colony) (game.Target, bool) {
	for _, s := range b.E.Ships {
		t := game.Target{Kind: game.ShipTarget, Mine: s}
		if by := pulled[s]; (by == nil || by == col) && float64(s.Hull) < cg.RepairAt*float64(s.MaxHull()) && b.E.CanTractor(col, t) == nil {
			return t, true
		}
	}
	own := true // this colony's weapons have no hostile in range
	for _, h := range hs {
		if inSpace(h) && gunsReach(col, h.Pos) {
			own = false
			break
		}
	}
	for _, h := range hs {
		t := game.Target{Kind: game.ShipTarget, Ship: h}
		if !inSpace(h) {
			continue
		}
		end, err := b.E.TractorEnd(col, t)
		if err != nil {
			continue
		}
		for _, c := range b.E.Colonies {
			if c.Star == star && (c != col && c.Tractors == 0 || c == col && own) && !gunsReach(c, h.Pos) && gunsReach(c, end) {
				return t, true
			}
		}
	}
	return game.Target{}, false
}

// gunsReach reports an orbital weapon of col with a shot left today
// within range of pos.
func gunsReach(col *game.Colony, pos game.Vec3) bool {
	p := col.Planet
	for slot, it := range p.Orbit {
		d, ok := it.Defence()
		if ok && d.Weapon() && col.OrbitShots[slot] < d.Shots && dist(p.Pos, pos) <= float64(d.Range)*game.RangeUnit+1e-9 {
			return true
		}
	}
	return false
}
