package game

import (
	"errors"
	"fmt"
	"slices"
)

// The day's close of control (docs/rules.md), after the empires' close and
// the invasions (the only moves of control), before the diplomacy:
//
//  1. The defeat (the original's end-dest): every
//     living empire with no colony left is out (its ships go too: with no
//     system controlled the scrap would take them). An empire whose
//     colonies all stand in shared systems lives on, its fleet cap 0,
//     until they are invaded.
//  2. The scrap (help.txt EP_SCRAPNEWESTSHIP): an empire over its fleet
//     cap loses its newest ships.

// ErrDefeated refuses a diplomatic act or an ability of, or toward, an
// empire out of the game.
var ErrDefeated = errors.New("defeated")

// Alive reports e still in the game: Lost is 0.
func (e *Empire) Alive() bool { return e.Lost == 0 }

// defeatedErr is ErrDefeated for the first of the empires named not
// Alive ("the Orfa are defeated"), nil when all are.
func (g *Game) defeatedErr(idx ...int) error {
	for _, i := range idx {
		if e := g.Empires[i]; !e.Alive() {
			return fmt.Errorf("the %s are %w", e.Species, ErrDefeated)
		}
	}
	return nil
}

// enforceCaps is the close of control: the defeats' Reports by loser,
// then the scraps' by empire.
func (g *Game) enforceCaps() []Report {
	var losers []*Empire
	for _, e := range g.Empires {
		if e.Alive() && len(e.Colonies) == 0 {
			losers = append(losers, e)
		}
	}
	for _, e := range losers {
		e.Lost = g.Day
	}
	var reps []Report
	for _, e := range losers {
		reps = append(reps, g.defeat(e)...)
	}
	for _, e := range g.Empires {
		if e.Alive() {
			reps = append(reps, e.scrap()...)
		}
	}
	return reps
}

// defeat takes e (Lost set, no colony) out of the game: its ships
// destroyed, the Offers from, to or about it dropped, its lane blocks
// lifted. A Defeated Report at its HomeStar for e, then for
// every other living empire (a same-day loser hears of no other).
func (g *Game) defeat(e *Empire) []Report {
	home := e.HomeStar()
	for len(e.Ships) > 0 {
		destroyIn([]*Empire{e}, e.Ships[len(e.Ships)-1])
	}
	g.Offers = slices.DeleteFunc(g.Offers, func(o Offer) bool {
		return o.From == e.Index || o.To == e.Index || o.Kind.Request() && o.Third == e.Index
	})
	e.BlockedUntil, e.LaneBlocks = 0, nil
	ev := Event{Kind: Defeated, Item: NoItem, Tech: NoTech, Star: home, Planet: -1, Other: -1,
		Text: "We are defeated"}
	reps := []Report{{e.Index, ev}}
	for _, o := range g.Empires {
		if o != e && o.Alive() {
			ev.Other = e.Index
			ev.Text = e.Species.String() + " defeated"
			reps = append(reps, Report{o.Index, ev})
		}
	}
	return reps
}

// scrap destroys e's newest ships (the last of Ships, in birth order)
// while it flies more than its cap, wherever they are; a Ship project is
// never cancelled (its ship goes at its launch day's close). One
// Scrapped event per ship (then its lostAboard), not journalled.
func (e *Empire) scrap() []Report {
	var reps []Report
	for capa := ShipsPerSystem * e.ControlledSystems(); len(e.Ships) > capa; {
		s := e.Ships[len(e.Ships)-1]
		destroyIn([]*Empire{e}, s)
		pl := s.Orbit
		if s.Lane >= 0 {
			pl = -1
		}
		reps = append(reps, Report{e.Index, Event{Kind: Scrapped, Item: NoItem, Tech: NoTech, Star: s.Star, Planet: pl, Other: -1,
			Text: s.Name + " scrapped: system lost"}})
		reps = append(reps, lostAboard(e, s)...)
	}
	return reps
}
