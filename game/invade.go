package game

import (
	"fmt"
	"slices"
)

// Invasions (gizhelp, Invasion Module: "It can be destroyed by a
// planetary Surface Shield unless multiple Invasion Modules are launched
// to overload the Surface Shield's tracking ability"; like
// colonising, on a hostile empire's planet). A ship in orbit invades
// (the window's InvadeNow, the bots' Invade order at the close): success
// iff its modules' worth exceeds the shields' total STRICTLY, and then
// only the fewest that do are launched (one for the Minions); else all
// are, and lost. The surface shields fall in cell order while the
// launched worth left covers their ShieldWorth (a later cheaper one may
// still fall), demolished.

// ModuleWorth is one module's worth, in half shields (2.5 stays exact).
const ModuleWorth = 2

// ShieldWorth is what each surface shield stops, in ModuleWorth's units
// (fixed, deterministic).
var ShieldWorth = map[Item]int{SurfaceShield: 2, SurfaceMegaShield: 5}

// CanInvade reports why s cannot invade the planet it orbits (not e's,
// Check, invadeErr), nil when it can.
func (e *Empire) CanInvade(s *Starship) error {
	if !slices.Contains(e.Ships, s) {
		return errNotOurs
	}
	if err := s.Check(e.Cluster, Order{Kind: Invade}); err != nil {
		return err
	}
	return e.invadeErr(s)
}

// invadeErr refuses an Invade by s, in orbit, unless the planet is the
// colony of an empire at War with e, not IsInvincible, and guarded by
// nobody but e (the guard named only when e SeesOrbit).
func (e *Empire) invadeErr(s *Starship) error {
	p := e.Planet(s.Star, s.Orbit)
	switch {
	case p.Owner < 0:
		return fmt.Errorf("%s is not inhabited", p.Name)
	case p.Owner == e.Index:
		return fmt.Errorf("%s is ours", p.Name)
	case e.StanceWith(p.Owner) != War:
		return fmt.Errorf("we are not at war with %s", whoIn(e.peerList(), p.Owner))
	}
	es := e.peerList()
	for _, o := range es {
		if o.Index == p.Owner && o.IsInvincible(e.Day) {
			return fmt.Errorf("the %s's colonies are invincible today", o.Species)
		}
	}
	if g, ok := invasionGuard(es, p, s.Star, s.Orbit, e.Index); ok {
		if !e.SeesOrbit(p) { // the guard unseen (a foreign cloaker): not named
			return fmt.Errorf("%s's orbit is guarded", p.Name)
		}
		return fmt.Errorf("%s is guarded by %s", p.Name, whoIn(es, g))
	}
	return nil
}

// invasionGuard is who protects planet p (pl of star) from inv (battle.txt:
// "Planets which have ships or defenses in orbit are protected from
// invasion"): its owner with an orbital shield, else the first ship in
// orbit that is not inv's.
func invasionGuard(es []*Empire, p *Planet, star, pl, inv int) (empire int, ok bool) {
	if p.Owner >= 0 && p.Owner != inv && orbitShielded(p) {
		return p.Owner, true
	}
	return inOrbit(es, star, pl, func(e *Empire) bool { return e.Index == inv })
}

// whoIn names empire g of es: "the <species>", else "empire g".
func whoIn(es []*Empire, g int) string {
	for _, o := range es {
		if o.Index == g {
			return "the " + o.Species.String()
		}
	}
	return fmt.Sprintf("empire %d", g)
}

// Invade resolves s's invasion of the planet it orbits, refused (nothing
// spent) as Order refuses it. Else the modules launched are spent and
// the shields fall as above; the Minions (InvadesAlways) always win, on
// one module. On success the colony changes hands as it is, appended to
// e's Colonies, its project and queue lost, its orbital weapons reset,
// the loser counting PlanetsLost. Reports: Invaded (help.txt's
// invadesuccess, EP_COLONYINVADED) or Repelled (invadefailure, ours),
// e's then the defender's.
func (g *Game) Invade(e *Empire, s *Starship) ([]Report, error) {
	if err := s.Check(g.Cluster, Order{Kind: Invade}); err != nil {
		return nil, err
	}
	if err := e.invadeErr(s); err != nil {
		return nil, err
	}
	p := e.Planet(s.Star, s.Orbit)
	te := g.Empires[p.Owner]
	col := te.Colony(s.Star, s.Orbit)
	if col == nil {
		return nil, fmt.Errorf("%s: no colony of the %s", p.Name, te.Species)
	}
	n := s.Invaders
	always := s.Species.Traits().InvadesAlways()
	total := 0
	for _, sq := range p.Surface {
		total += ShieldWorth[sq.Item]
	}
	// The minimum launched: enough to beat the
	// shields strictly, one for the Minions; short of that, all of them.
	spent := n
	switch {
	case always:
		spent = 1
	case n*ModuleWorth > total:
		spent = total/ModuleWorth + 1
	}
	s.Invaders -= spent
	left, fell := spent*ModuleWorth, 0
	for _, c := range p.Cells() {
		sq, ok := p.Surface[c]
		if !ok {
			continue
		}
		w, ok := ShieldWorth[sq.Item]
		if !ok {
			continue
		}
		if always || w <= left {
			left -= w
			p.Demolish(c)
			fell++
		}
	}
	ev := Event{Item: NoItem, Tech: NoTech, Star: s.Star, Planet: s.Orbit, Other: -1}
	mine, theirs := ev, ev
	if !always && n*ModuleWorth <= total {
		mine.Kind, theirs.Kind = Repelled, Repelled
		mine.Text = "Invasion of " + p.Name + " failed"
		theirs.Text = fmt.Sprintf("%s repelled from %s", e.Species, p.Name)
		return []Report{{e.Index, mine}, {te.Index, theirs}}, nil
	}
	col.cancel()
	col.Queue = nil // the loser's plans
	// The capture says it all: no blockade news after it.
	col.Armed, col.OrbitShots, col.Tractors, col.BlockedBy = nil, [OrbitSlots]int{}, 0, 0
	i := slices.Index(te.Colonies, col)
	te.Colonies = slices.Delete(te.Colonies, i, i+1)
	col.adopt(e)
	e.Colonies = append(e.Colonies, col)
	p.Owner = e.Index
	if r := te.Rel(e.Index); r != nil {
		r.PlanetsLost++
	}
	mine.Kind, theirs.Kind = Invaded, Invaded
	mine.Text = p.Name + " captured"
	theirs.Text = fmt.Sprintf("%s invaded by %s", p.Name, e.Species)
	return []Report{{e.Index, mine}, {te.Index, theirs}}, nil
}

// InvadeNow is Invade resolved at once, the window's button (no
// wait for the close): command's refusals first, then
// Invade's; journalled ("invade"), s's order cleared. The bots still
// order Invade, resolved at the close (invasions).
func (g *Game) InvadeNow(e *Empire, s *Starship) ([]Report, error) {
	if err := e.command(s); err != nil {
		return nil, err
	}
	reps, err := g.Invade(e, s)
	if err != nil {
		return nil, err
	}
	s.Order = Order{}
	if e.journal != nil {
		en := e.shipEntry("invade", s)
		en.Star, en.Planet = s.Star, s.Orbit
		e.journal.Record(en)
	}
	return reps, nil
}

// plural is one for n == 1, many otherwise (0 invaders).
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// invasions resolves the day's Invade orders in Empires then Ships order,
// each cleared; a refused one is dropped silently, modules kept. A
// capture is live for the later invaders.
func (g *Game) invasions() []Report {
	var reps []Report
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if s.Order.Kind != Invade {
				continue
			}
			s.Order = Order{}
			r, _ := g.Invade(e, s)
			reps = append(reps, r...)
		}
	}
	return reps
}
