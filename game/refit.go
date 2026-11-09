package game

import (
	"errors"
	"fmt"
	"slices"
)

// Refit at Orbital Docks (planhelp.txt: "its contents may be changed, and
// its hull is repaired"): a colony project, Item OrbitalDocks,
// Project.Refit naming the ship, Project.Design the target, the hull kept.
// The ship is held meanwhile but fights as it is; destroyed, the refit is
// cancelled at the next close; completed, applyRefit.

// RefitCost is the industry of refitting s to d. ASSUMPTION: the parts
// ADDED (the multiset d − s.Aboard(): a spent Colonizer or Invasion
// Module is gone, re-adding it is paid; a removed part refunds nothing)
// at their Industry, plus ⌈(MaxHull − Hull) × HullCost / MaxHull⌉ of
// repair.
func RefitCost(s *Starship, d Design) int {
	have := map[Gizmo]int{}
	for _, g := range s.Aboard().Parts {
		have[g]++
	}
	n := 0
	for _, g := range d.Parts {
		if have[g] > 0 {
			have[g]--
			continue
		}
		n += Gizmos[g].Industry
	}
	if full := s.MaxHull(); full > 0 && s.Hull < full {
		n += ((full-s.Hull)*s.Design.Hull.Cost() + full - 1) / full
	}
	return n
}

// sameParts reports a and b hold the same parts, whatever their order.
func sameParts(a, b []Gizmo) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

// RefitTarget is the player's refit of s: e's template named as its
// design, realised; errors when none, not realisable, or another hull.
func (e *Empire) RefitTarget(s *Starship) (Design, error) {
	i := slices.IndexFunc(e.Templates, func(t Template) bool { return t.Name == s.Design.Name })
	if i < 0 {
		return Design{}, fmt.Errorf("no template %q", s.Design.Name)
	}
	t := e.Templates[i]
	d, ok := t.Realise(e.Research)
	if !ok {
		return Design{}, fmt.Errorf("template %q is not realisable yet", t.Name)
	}
	if d.Hull != s.Design.Hull {
		return Design{}, fmt.Errorf("template %q is a %s hull, %s is %s", t.Name, d.Hull, s.Name, s.Design.Hull)
	}
	return d, nil
}

// CanRefit reports why c cannot refit s to d, nil when it can: both e's,
// Docks there, s in that orbit, d Valid on s's hull with a berth for each
// colonist aboard (Starship.Pop), something to refit,
// no refit of s already, then the colony's common checks. Nothing to
// refit: d holds the parts aboard (Aboard) and the hull is full.
func (e *Empire) CanRefit(c *Colony, s *Starship, d Design) error {
	switch {
	case !slices.Contains(e.Ships, s):
		return fmt.Errorf("%s is not ours", s.Name)
	case !slices.Contains(e.Colonies, c):
		return fmt.Errorf("%s is not our colony", c.Planet.Name)
	case !c.Planet.HasDocks():
		return fmt.Errorf("%s has no Orbital Docks", c.Planet.Name)
	case s.Lane >= 0 || s.Star != c.Star || s.Orbit != c.PlanetIdx:
		return fmt.Errorf("%s is not in orbit of %s", s.Name, c.Planet.Name)
	case d.Hull != s.Design.Hull:
		return fmt.Errorf("a refit keeps the hull: %s is %s, the design %s", s.Name, s.Design.Hull, d.Hull)
	case d.Berths() < s.Pop:
		return fmt.Errorf("%s carries %d %s, the design berths %d: unload first", s.Name, s.Pop, plural(s.Pop, "colonist", "colonists"), d.Berths())
	}
	if err := d.Valid(e.Research); err != nil {
		return err
	}
	if sameParts(d.Parts, s.Aboard().Parts) && s.Hull >= s.MaxHull() {
		return errors.New("nothing to refit")
	}
	if at := e.Refitting(s); at != nil {
		return fmt.Errorf("%s is being refitted at %s", s.Name, at.Planet.Name)
	}
	return c.common(e.Research, OrbitalDocks)
}

// Refit starts c's project refitting s to d, as CanRefit allows: its cost
// fixed now, s's order cleared.
func (e *Empire) Refit(c *Colony, s *Starship, d Design) error {
	if err := e.CanRefit(c, s, d); err != nil {
		return err
	}
	d.Parts, d.Slots = slices.Clone(d.Parts), slices.Clone(d.Slots)
	c.Project = &Project{Item: OrbitalDocks, Slot: -1, Orbital: true, Design: &d, Refit: s.Name, RefitCost: RefitCost(s, d)}
	c.credit()
	s.Order = Order{}
	if e.journal != nil {
		en := e.shipEntry("refit", s)
		en.Colony, en.Design = c.Planet.Name, d.Name
		e.journal.Record(en)
	}
	return nil
}

// Refitting is the colony refitting s, nil when none.
func (e *Empire) Refitting(s *Starship) *Colony {
	for _, c := range e.Colonies {
		if c.Project != nil && c.Project.Refit != "" && c.Project.Refit == s.Name {
			return c
		}
	}
	return nil
}

// ship is e's ship named name, nil when none (destroyed).
func (e *Empire) ship(name string) *Starship {
	for _, s := range e.Ships {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// cancelLostRefits cancels every refit whose ship is no longer in Ships.
func (e *Empire) cancelLostRefits() {
	for _, c := range e.Colonies {
		if pj := c.Project; pj != nil && pj.Refit != "" && e.ship(pj.Refit) == nil {
			c.cancel()
		}
	}
}

// applyRefit gives s the design d: hull full, cargo loaded, the per-part
// state reset, the power left capped at d's day. Shields on are switched
// off before (the old draw refunded) and on again after (d's draw taken;
// short of it today but carried by d's day, they stay on at Power 0 for
// the Refill that follows in the same endDay to reserve; off only when
// d's day cannot carry them, as Refill would).
func applyRefit(s *Starship, d Design) {
	on := s.Shields
	_ = s.SetShields(false) // off is never refused
	s.Design = d
	s.Hull = s.MaxHull()
	s.healed()
	s.Cargo = d.Colonizers()
	s.Invaders = d.InvasionModules()
	s.Shots = make([]int, len(d.Parts))
	s.Weapon = -1
	s.Armed = nil
	s.Power = min(s.Power, d.PowerPerDay())
	if on && s.SetShields(true) != nil && d.ShieldDraw() <= d.PowerPerDay() {
		s.Shields, s.Power = true, 0
	}
}
