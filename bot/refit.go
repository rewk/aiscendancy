package bot

import (
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// The bot's refit policy (docs/rules.md "The bot's combat policy"): one
// Orbital Docks per empire once in contact (build), an armed ship wanting
// a refit sails to it (refitRoute) and is refitted there before any
// colony plans (startRefits).

// docksPlanned reports a colony of mine holding Orbital Docks or building
// them (a refit is an OrbitalDocks project too, with a Refit name).
func (b *Bot) docksPlanned() bool {
	for _, col := range b.E.Colonies {
		if col.Planet.HasDocks() {
			return true
		}
		if pj := col.Project; pj != nil && pj.Item == game.OrbitalDocks && pj.Refit == "" {
			return true
		}
	}
	return false
}

// refitMaxDays bounds what a ship waits for and spends at the Docks (the
// running project's days left plus its refit's): past it the ship is
// worth more fighting. A policy choice, not a rule.
var refitMaxDays = 60 // a var: the tests' fixtures make little industry

// docksFor is my first colony with Orbital Docks at star, on planet pl
// (pl < 0: any), dockable for s to d, whose orbit s is in or may enter;
// nil when none.
func (b *Bot) docksFor(star, pl int, s *game.Starship, d game.Design) *game.Colony {
	for _, col := range b.E.Colonies {
		if col.Star == star && (pl < 0 || col.PlanetIdx == pl) && col.Planet.HasDocks() && dockable(col, s, d) &&
			(s.Orbit == col.PlanetIdx && s.Star == star || !b.E.Barred(star, col.PlanetIdx)) {
			return col
		}
	}
	return nil
}

// dockable reports col able to refit s to d within refitMaxDays: it
// makes industry, a worker is free once its running project ends, and
// that project's days left plus ⌈RefitCost / industry⌉ fit.
func dockable(col *game.Colony, s *game.Starship, d game.Design) bool {
	col = yielded(col) // an ongoing project gives way to the refit
	ind, _, _ := col.Output()
	if ind <= 0 {
		return false
	}
	wait, free := 0, col.Free()
	if col.Project != nil {
		days, ok := col.DaysLeft()
		if !ok {
			return false
		}
		wait, free = days, col.FreeAtCompletion()
	}
	if free < game.OrbitalDocks.Info().UsePop {
		return false
	}
	return wait+(game.RefitCost(s, d)+ind-1)/ind <= refitMaxDays
}

// anyDocks reports a colony of mine with Orbital Docks standing.
func (b *Bot) anyDocks() bool {
	for _, col := range b.E.Colonies {
		if col.Planet.HasDocks() {
			return true
		}
	}
	return false
}

// wantRefit is the refit an armed ship s wants (b.cg's genes): today's
// warship on its hull when it differs and deals at least (1 + RefitGain)
// × s's damage (RefitGain 0: strictly more); else, below RepairAt ×
// MaxHull, a repair (to that warship when it deals more, else s's own
// design). ok false otherwise.
func (b *Bot) wantRefit(s *game.Starship) (target game.Design, ok bool) {
	if !armed(s.Design) {
		return game.Design{}, false
	}
	cur := game.DesignDamage(s.Design, 0)
	var extra []game.Gizmo // the specials it carries, kept
	for _, g := range s.Design.Parts {
		if carried(g) {
			extra = append(extra, g)
		}
	}
	d, okD := game.WarshipWith(b.E.Research, s.Design.Hull, s.Design.LaneDrives() > 0, extra)
	other := okD && armed(d) && !sameParts(d.Parts, s.Design.Parts)
	if other {
		if nd := game.DesignDamage(d, 0); nd > cur && float64(nd) >= (1+b.cg.RefitGain)*float64(cur) {
			return d, true
		}
	}
	if float64(s.Hull) < b.cg.RepairAt*float64(s.MaxHull()) {
		if other && game.DesignDamage(d, 0) > cur {
			return d, true
		}
		return s.Design, true
	}
	return game.Design{}, false
}

// sameParts reports a and b hold the same parts in any order: the
// warship's design against the ship's (CanRefit compares Aboard instead).
func sameParts(a, b []game.Gizmo) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

// startRefits refits, in Ships order, each ship of mine in orbit of a
// dockable Docks colony free of a real project, not refitted already,
// NoOrder (or the GoToPlanet that brought it), no hostile at its star,
// that wantRefit: the first such ship takes the colony. Run before the
// colonies plan, so a waiting ship's docks get no other project.
func (b *Bot) startRefits() {
	if !b.anyDocks() {
		return
	}
	for _, s := range b.E.Ships {
		if s.Lane >= 0 || s.Orbit < 0 || b.their(s.Star).n > 0 {
			continue
		}
		if k := s.Order.Kind; k != game.NoOrder && (k != game.GoToPlanet || s.Order.Planet != s.Orbit) {
			continue
		}
		if b.E.Refitting(s) != nil {
			continue
		}
		if d, ok := b.wantRefit(s); ok {
			if col := b.docksFor(s.Star, s.Orbit, s, d); col != nil && yielded(col).Project == nil {
				b.E.Refit(col, s, d) // a refusal: it waits in orbit
			}
		}
	}
}

// refitRoute moves a ship of mine toward a refit, reporting it handled.
// Being refitted, it holds. Out of the lanes, NoOrder, no hostile at its
// star, wanting a refit: in orbit of a docksFor colony it holds (waiting
// for startRefits); at a star with one it goes GoToPlanet; else GoToStar
// the first hop toward the nearest star with one (avoiding danger), the
// star in dest. Otherwise false: attack or planShip decide.
func (b *Bot) refitRoute(s *game.Starship) bool {
	if !b.anyDocks() {
		return false
	}
	if b.E.Refitting(s) != nil {
		return true
	}
	if s.Lane >= 0 || s.Order.Kind != game.NoOrder || b.their(s.Star).n > 0 {
		return false
	}
	d, ok := b.wantRefit(s)
	if !ok {
		return false
	}
	if s.Orbit >= 0 && b.docksFor(s.Star, s.Orbit, s, d) != nil {
		return true
	}
	if col := b.docksFor(s.Star, -1, s, d); col != nil {
		return b.E.Order(s, game.Order{Kind: game.GoToPlanet, Planet: col.PlanetIdx}) == nil
	}
	strength := s.Design.LaneStrength()
	if strength == 0 {
		return false
	}
	// The dockable stars, judged once: search asks every star it settles.
	at := map[int]bool{}
	for _, col := range b.E.Colonies {
		if !at[col.Star] && col.Planet.HasDocks() && dockable(col, s, d) && !b.E.Barred(col.Star, col.PlanetIdx) {
			at[col.Star] = true
		}
	}
	if len(at) == 0 {
		return false
	}
	to, first := b.search(s.Star, strength, s.Species.Traits(), 0, b.danger(s), func(v, _ int) bool { return at[v] })
	if to < 0 || b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first}) != nil {
		return false
	}
	b.dest[s] = to
	return true
}
