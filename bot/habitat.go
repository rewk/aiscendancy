package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// The bot's population transport (docs/rules.md "Population transport"):
// a full colony with free people is a donor, a colony at Pop ≤
// HabitatFill × Capacity a recipient; a shipyard builds Habitat ships
// (habitatDesign) while the donors' surplus and the recipients' room
// call for more, and each one ferries (ferry): it loads at a donor,
// unloads at a recipient, and flies between them.

// habitatShare and habitatFill are the fixed policy's HabitatShare and
// HabitatFill (Genome's): a Habitat ship per two ship-loads of the
// people that can move, to colonies at half their Capacity or less.
const (
	habitatShare = 0.5
	habitatFill  = 0.5
)

// habitatGenes is the Genome's HabitatShare and HabitatFill, else
// habitatShare and habitatFill.
func (b *Bot) habitatGenes() (share, fill float64) {
	if g := b.Cfg.Genome; g != nil {
		return g.HabitatShare, g.HabitatFill
	}
	return habitatShare, habitatFill
}

// donor reports col full (Pop at Capacity or more) with a free unit
// (Colony.Free): a colony with room never gives its people away.
func donor(col *game.Colony) bool {
	return col.Pop >= col.Capacity() && col.Free() >= 1
}

// recipient reports col below its Capacity at Pop ≤ fill × Capacity.
func recipient(col *game.Colony, fill float64) bool {
	return col.Pop < col.Capacity() && float64(col.Pop) <= fill*float64(col.Capacity())
}

// habitatShip is col's Habitat ship: game.HabitatShipOn every known hull
// (a lane drive when allowed), of those col builds within ColonyDays
// (Colony.DaysFor) the one with the most berths, the smallest hull on a
// tie; none within, the smallest hull's; ok false when none.
func (b *Bot) habitatShip(col *game.Colony) (game.Design, bool) {
	r := b.E.Research
	lane := r.AllowsGizmo(game.StarLaneDrive)
	var best, first game.Design
	okBest, okFirst := false, false
	for h := game.Hull(0); h < game.NumHulls; h++ {
		d, ok := game.HabitatShipOn(r, h, lane)
		if !ok {
			continue
		}
		if !okFirst {
			first, okFirst = d, true
		}
		if days, ok := col.DaysFor(d.Cost()); ok && days <= ColonyDays && (!okBest || d.Berths() > best.Berths()) {
			best, okBest = d, true
		}
	}
	if okBest {
		return best, true
	}
	return first, okFirst
}

// habTally is what habitatDesign weighs, counted once an Act (a colony's
// build reads it, scanning every colony per shipyard was a third of a
// bot's day): the donors' Free, the recipients' room, the Habitat ships
// (Berths > 0) flying and being built, bumped by build's own starts.
type habTally struct{ surplus, room, ships int }

// tally is the Act's habTally, counted on first use.
func (b *Bot) tally(fill float64) *habTally {
	if b.hab != nil {
		return b.hab
	}
	t := &habTally{}
	for _, c := range b.E.Colonies {
		switch {
		case donor(c):
			t.surplus += c.Free()
		case recipient(c, fill):
			t.room += c.Capacity() - c.Pop
		}
		if pd := building(c); pd != nil && pd.Berths() > 0 {
			t.ships++
		}
	}
	for _, s := range b.E.Ships {
		if s.Design.Berths() > 0 {
			t.ships++
		}
	}
	b.hab = t
	return t
}

// habitatDesign is the Habitat ship col's shipyard should start, ok false
// when none: HabitatShare > 0 and the Space Habitat allowed; a fleet slot
// left free after it (as wantScout); and fewer Habitat ships than
// ⌈HabitatShare × min(surplus, room) / its berths⌉, as the Act's tally
// counts them.
func (b *Bot) habitatDesign(col *game.Colony) (game.Design, bool) {
	share, fill := b.habitatGenes()
	if share <= 0 || !b.E.Research.AllowsGizmo(game.SpaceHabitat) {
		return game.Design{}, false
	}
	if n, capa := b.E.Fleet(); n+1 >= capa {
		return game.Design{}, false
	}
	t := b.tally(fill)
	if t.surplus == 0 || t.room == 0 {
		return game.Design{}, false
	}
	d, ok := b.habitatShip(col)
	if !ok {
		return game.Design{}, false
	}
	wanted := math.Ceil(share * float64(min(t.surplus, t.room)) / float64(d.Berths()))
	return d, float64(t.ships) < wanted
}

// ferrying is the population aboard my Habitat ships already sent
// somewhere, by place: a GoToPlanet's planet, a GoToStar's or a lane's
// destination (dest, else the next star) as place{star, -1}. Rebuilt
// each Act from the orders, nothing saved; ferry adds what it sends.
func (b *Bot) ferrying() map[place]int {
	in := map[place]int{}
	for _, s := range b.E.Ships {
		if s.Design.Berths() == 0 || s.Pop == 0 {
			continue
		}
		switch to, ok := b.dest[s]; {
		case s.Lane >= 0 || s.Order.Kind == game.GoToStar:
			if !ok {
				to = s.Order.Star
				if s.Lane >= 0 {
					to = b.E.Cluster.Other(s.Lane, s.Star)
				}
			}
			in[place{to, -1}] += s.Pop
		case s.Order.Kind == game.GoToPlanet:
			in[place{s.Star, s.Order.Planet}] += s.Pop
		}
	}
	return in
}

// ferry moves a Habitat ship of mine, out of the lanes. In orbit of my
// colony, as it stands before either: a recipient takes the people
// aboard (Unload until refused), else a donor gives (Load until refused)
// when a recipient elsewhere would take them: one waiting where it
// loaded would land them back the next Act, the donor drained into a
// recipient, and load them again the one after. An order under way
// stands while its target still qualifies (a GoToPlanet's colony, a
// GoToStar's dest star holding one), else it is dropped. Then, NoOrder:
// loaded, toward the nearest recipient whose room is not taken by the
// people already sent there (in, by ferrying); empty, toward the nearest
// donor; nearest as invasionTarget finds it (its system first, then by
// lane days avoiding danger), the far star in dest. Nowhere: it waits.
func (b *Bot) ferry(s *game.Starship, in map[place]int) {
	if s.Lane >= 0 {
		return
	}
	_, fill := b.habitatGenes()
	loaded := s.Pop > 0
	// qualifies is the target test of a ship already sent: no room check,
	// its own people are counted in in.
	qualifies := func(star, pl int) bool {
		c := b.E.Colony(star, pl)
		if c == nil {
			return false
		}
		if loaded {
			return recipient(c, fill)
		}
		return donor(c)
	}
	ok := func(star, pl int) bool {
		if s.Orbit >= 0 && star == s.Star && pl == s.Orbit || !qualifies(star, pl) {
			return false
		}
		if !loaded {
			return true
		}
		c := b.E.Colony(star, pl)
		return c.Capacity()-c.Pop-in[place{star, pl}]-in[place{star, -1}] > 0
	}
	target := func() (st, pl, first int) {
		return b.invasionTarget(s.Star, s.Pos, s.Design.LaneStrength(), s.Species.Traits(), b.danger(s), ok)
	}
	if s.Orbit >= 0 {
		// One or the other, as the colony stands: filled to Capacity, a
		// recipient is a donor, and would take its people back.
		switch c := b.E.Colony(s.Star, s.Orbit); {
		case c == nil:
		case loaded && recipient(c, fill):
			for b.E.Unload(s) == nil {
			}
		case donor(c):
			loaded = true // the target a load would seek
			if st, _, _ := target(); st >= 0 {
				for b.E.Load(s) == nil {
				}
			}
		}
		loaded = s.Pop > 0
	}
	anyAt := func(star int) bool {
		for pl := range b.E.Cluster.Stars[star].System.Planets {
			if qualifies(star, pl) {
				return true
			}
		}
		return false
	}
	switch s.Order.Kind {
	case game.NoOrder:
	case game.GoToPlanet:
		if qualifies(s.Star, s.Order.Planet) {
			return
		}
		s.Order = game.Order{}
	case game.GoToStar:
		if to, ok := b.dest[s]; !ok || anyAt(to) {
			return
		}
		s.Order = game.Order{}
		delete(b.dest, s)
	default:
		return
	}
	st, pl, first := target()
	switch {
	case st < 0:
	case first < 0:
		if b.E.Order(s, game.Order{Kind: game.GoToPlanet, Planet: pl}) == nil {
			in[place{st, pl}] += s.Pop
		}
	case b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first}) == nil:
		b.dest[s] = st
		in[place{st, -1}] += s.Pop
	}
}
