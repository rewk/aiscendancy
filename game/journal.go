package game

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/rewk/aiscendancy/journal"
)

// The action journal (docs/formats.md "The action journal"): every
// decision entry point records one journal.Entry once it has ACCEPTED a
// call; a refusal records nothing, nor does the day's own resolution,
// which goes through unexported twins (foundAt, cancel, answer,
// declareWar...). The Writer hangs off the Game, each Empire and each
// Colony, never saved, nil until SetJournal. A Colony points at its owner
// (adopt) wherever it is made, loaded (Empire.Relink) or changes hands.

// SetJournal makes g record its actions to w (nil: none). Call it again on
// a loaded game.
func (g *Game) SetJournal(w *journal.Writer) {
	g.journal = w
	for _, e := range g.Empires {
		e.journal = w
		for _, c := range e.Colonies {
			c.adopt(e)
		}
	}
}

// adopt makes e c's owner, its journal c's.
func (c *Colony) adopt(e *Empire) { c.journal, c.owner = e.journal, e }

// entry is e's act today, every index −1.
func (e *Empire) entry(act string) journal.Entry { return journal.NewEntry(e.Day, e.Index, act) }

// entry is the act of c's owner on its day (0 for a colony never
// adopted), the colony named, every index −1.
func (c *Colony) entry(act string) journal.Entry {
	day, owner := 0, 0
	if c.owner != nil {
		day, owner = c.owner.Day, c.owner.Index
	}
	en := journal.NewEntry(day, owner, act)
	en.Colony = c.Planet.Name
	return en
}

// shipEntry is e's act on s today.
func (e *Empire) shipEntry(act string, s *Starship) journal.Entry {
	en := e.entry(act)
	en.Ship = s.Name
	return en
}

// pairEntry is a's act toward b today (the diplomacy).
func (g *Game) pairEntry(act string, a, b int) journal.Entry {
	en := journal.NewEntry(g.Day, a, act)
	en.Other = b
	return en
}

// Study makes t the research topic (Research.Study) and records it.
func (e *Empire) Study(t Tech) error {
	same := e.Research.Topic == t
	if err := e.Research.Study(t); err != nil {
		return err
	}
	if e.journal != nil && !same { // picking the topic again decides nothing
		en := e.entry("study")
		en.Tech = t.String()
		e.journal.Record(en)
	}
	return nil
}

// SetShields turns s's shields on or off (Starship.SetShields: on
// reserves ShieldDraw, off refunds it), recorded when it changes; a ship
// not of e's is refused.
func (e *Empire) SetShields(s *Starship, on bool) error {
	if !slices.Contains(e.Ships, s) {
		return errNotOurs
	}
	if s.Shields == on {
		return nil
	}
	if err := s.SetShields(on); err != nil {
		return err
	}
	if e.journal != nil {
		en := e.shipEntry("shields", s)
		en.On = on
		e.journal.Record(en)
	}
	return nil
}

// recordOrder is Order's entry: the kind and what it points at.
func (e *Empire) recordOrder(s *Starship, o Order) {
	en := e.shipEntry("order", s)
	en.Kind = o.Kind.String()
	switch o.Kind {
	case GoToStar, GoToSystem:
		en.Star = o.Star
	case GoToPlanet:
		en.Star, en.Planet = s.Star, o.Planet
	case Colonise, Invade:
		en.Star, en.Planet = s.Star, s.Orbit
	case GoToPoint:
		en.Star = s.Star
		en.Target = fmt.Sprintf("(%.3f,%.3f,%.3f)", o.Pos.X, o.Pos.Y, o.Pos.Z)
	}
	e.journal.Record(en)
}

// targetName names a special's target t from s's star: the ship's name,
// "lane N" (N in the system's Lanes) or the planet's name; "" for none.
func (e *Empire) targetName(s *Starship, t Target) string {
	switch t.Kind {
	case ShipTarget:
		if tg := t.ship(); tg != nil {
			return tg.Name
		}
	case LaneTarget:
		return "lane " + strconv.Itoa(t.Index)
	case PlanetTarget:
		if pls := e.Cluster.Stars[s.Star].System.Planets; t.Index >= 0 && t.Index < len(pls) {
			return pls[t.Index].Name
		}
	}
	return ""
}
