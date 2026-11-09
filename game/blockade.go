package game

import "fmt"

// The blockade: a ship of an empire at War with a
// colony's owner, in its planet's orbit, stops the colony's work in orbit
// (an orbital structure, a ship, a refit). The project stays, its
// progress kept, but the day's industry is lost (nothing carried) for as
// long as a blockader stays. A blockader must see the orbit: an Orbital
// Cloaker over the planet (OrbitCloaked) keeps its yard working.

// InOrbit reports pj built in orbit: an orbital structure, a ship, a
// refit.
func (pj *Project) InOrbit() bool { return pj.Orbital || pj.Item == Ship || pj.Refit != "" }

// Blockader is the first empire (game order) at War with c's owner with
// a ship in c's planet's orbit and SeesOrbit there, −1 none (and outside
// a game).
func (c *Colony) Blockader() int {
	if c.owner == nil || c.Star < 0 {
		return -1
	}
	for _, o := range c.owner.peerList() {
		if o == c.owner || c.owner.StanceWith(o.Index) != War || !o.SeesOrbit(c.Planet) {
			continue
		}
		for _, s := range o.Ships {
			if s.Lane < 0 && s.Star == c.Star && s.Orbit == c.PlanetIdx {
				return o.Index
			}
		}
	}
	return -1
}

// Blockaded reports c's running project in orbit and a Blockader there
// now: it makes no progress today.
func (c *Colony) Blockaded() bool {
	return c.Running() && c.Project.InOrbit() && c.Blockader() >= 0
}

// blockades marks, at the day's start (Game.EndDay, before any ship
// moves), every colony Blockaded then: Colony.EndDay reads the mark.
func (g *Game) blockades() {
	for _, e := range g.Empires {
		for _, c := range e.Colonies {
			c.blockaded = c.Blockaded()
		}
	}
}

// blockadeNews compares, at the close (ships moved, invasions done, the
// defeated's and the scrapped ships gone: enforceCaps), every colony's
// Blockader with its BlockedBy and reports a change to both sides, the
// lift first: "Blockade of X lifted" (not to a defeated blockader);
// "X blockaded by the Y" to the owner, "X blockaded" to the blockader
// (it sees the orbit). BlockedBy takes today's.
func (g *Game) blockadeNews() []Report {
	var reps []Report
	for _, e := range g.Empires {
		for _, c := range e.Colonies {
			now := c.Blockader() + 1
			if now == c.BlockedBy {
				continue
			}
			ev := Event{Kind: Blockade, Item: NoItem, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx}
			if was := c.BlockedBy - 1; was >= 0 {
				ev.Text, ev.Other = "Blockade of "+c.Planet.Name+" lifted", was
				reps = append(reps, Report{e.Index, ev})
				if g.Empires[was].Alive() {
					ev.Other = e.Index
					reps = append(reps, Report{was, ev})
				}
			}
			if by := now - 1; by >= 0 {
				ev.Text, ev.Other = fmt.Sprintf("%s blockaded by the %s", c.Planet.Name, g.Empires[by].Species), by
				reps = append(reps, Report{e.Index, ev})
				ev.Text, ev.Other = c.Planet.Name+" blockaded", e.Index
				reps = append(reps, Report{by, ev})
			}
			c.BlockedBy = now
		}
	}
	return reps
}
