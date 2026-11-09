package main

import "github.com/rewk/aiscendancy/game"

// The climb: after a day the view climbs a tree of places (starmap root,
// systems, planets) when the screen on top does not cover the day's
// events. A Discovered with no place is the discovery popup's.

// evPlace is a node of the tree: star −1 the root, planet −1 a system.
type evPlace struct{ star, planet int }

// rootKind reports the kinds placed at the root whatever their Star (a
// Defeated carries a star for the log's link).
func rootKind(k game.EventKind) bool {
	return k >= game.Contacted && k <= game.HelpGiven || k == game.AbilityUsed || k == game.AbilityReady || k == game.Defeated
}

// placeOf is ev's node: the root for rootKind or no star, else its
// planet, else its system.
func placeOf(ev game.Event) evPlace {
	switch {
	case rootKind(ev.Kind) || ev.Star < 0:
		return evPlace{-1, -1}
	case ev.Planet >= 0:
		return evPlace{ev.Star, ev.Planet}
	}
	return evPlace{ev.Star, -1}
}

// navigable reports an event the climb considers.
func navigable(ev game.Event) bool { return ev.Kind != game.Discovered || ev.Star >= 0 }

// queuedOn reports a Built or Unqueued of e's colony that runs a project
// again after the close: its queue took over (game.Colony.Next), so the
// news stays a log line and does not climb.
func queuedOn(e *game.Empire, ev game.Event) bool {
	if e == nil || ev.Kind != game.Built && ev.Kind != game.Unqueued || ev.Planet < 0 {
		return false
	}
	c := e.Colony(ev.Star, ev.Planet)
	return c != nil && c.Running()
}

// idleAfter reports a Built, Unqueued or Grew of e's colony that runs no
// project and queues nothing after the close, people free: it waits for
// an order (a queue waiting for a free worker starts on its own, and
// with none free no order would start). The Grew logged is the one that
// frees a worker on a colony idle for want of people (finishDay's
// freed): the order it waited for is due now.
func idleAfter(e *game.Empire, ev game.Event) bool {
	if e == nil || ev.Kind != game.Built && ev.Kind != game.Unqueued && ev.Kind != game.Grew || ev.Planet < 0 {
		return false
	}
	c := e.Colony(ev.Star, ev.Planet)
	if c == nil || c.Running() || len(c.Queue) > 0 {
		return false
	}
	_, free, _ := colonyPop(c)
	return free > 0
}

// noPopAfter reports a Built or Unqueued of e's colony that runs no
// project after the close with no free people: the log line says so.
func noPopAfter(e *game.Empire, ev game.Event) bool {
	if e == nil || ev.Kind != game.Built && ev.Kind != game.Unqueued || ev.Planet < 0 {
		return false
	}
	c := e.Colony(ev.Star, ev.Planet)
	if c == nil || c.Running() {
		return false
	}
	_, free, _ := colonyPop(c)
	return free == 0
}

// colonyKind, shipKind and diplomatic are the kinds the planets list,
// the fleet list and the species screens cover.
func colonyKind(ev game.Event) bool {
	switch ev.Kind {
	case game.Built, game.Grew, game.Razed, game.Invaded, game.Repelled, game.Unqueued:
		return true
	}
	return false
}

func shipKind(ev game.Event) bool {
	switch ev.Kind {
	case game.Destroyed, game.Refitted, game.SpecialUsed, game.Scrapped, game.Arrived, game.NoRoute:
		return true
	}
	return ev.Kind == game.Built && ev.Item == game.Ship
}

func diplomatic(ev game.Event) bool {
	if ev.Other < 0 {
		return false
	}
	return ev.Star < 0 && ev.Kind >= game.Contacted && ev.Kind <= game.HelpGiven || ev.Kind == game.Defeated
}

// covers reports screen sc shows ev already: the starmap and the log
// all; a system or planet screen those placed there; the planets list
// colony kinds and events on our colonies; the fleet ship kinds; the
// species and negotiation screens diplomatic ones.
func covers(a *app, sc screen, ev game.Event) bool {
	p := placeOf(ev)
	switch s := sc.(type) {
	case *starmapScreen, *logScreen:
		return true
	case *systemScreen:
		return p.star >= 0 && p.star == s.star
	case *planetScreen:
		return p.star >= 0 && p == evPlace{s.star, s.idx}
	case *planetsScreen:
		return colonyKind(ev) || p.planet >= 0 && a.e != nil && a.e.Colony(p.star, p.planet) != nil
	case *fleetScreen:
		return shipKind(ev)
	case *speciesScreen, *negotiateScreen:
		return diplomatic(ev)
	}
	return false
}

// navigate is the climb: nothing when the top covers every navigable
// event (those queuedOn left out); a combat replay defers it (navDue).
// Else the target is the nearest covering ancestor (a planet's system,
// else the starmap): a stacked screen of that place is popped down to,
// kept as is; else the stack drops to its starmap and a fresh system
// screen goes over it.
func (a *app) navigate(evs []game.Event) {
	var nav []game.Event
	for _, ev := range evs {
		if navigable(ev) && !queuedOn(a.e, ev) {
			nav = append(nav, ev)
		}
	}
	top := a.top()
	all := true
	for _, ev := range nav {
		if !covers(a, top, ev) {
			all = false
			break
		}
	}
	if all {
		return
	}
	if v, ok := top.(*systemScreen); ok && v.replay != nil {
		v.replay.navDue = append(v.replay.navDue, nav...)
		return
	}
	star := -1
	if ps, ok := top.(*planetScreen); ok {
		star = ps.star
		for _, ev := range nav {
			if placeOf(ev).star != ps.star {
				star = -1
				break
			}
		}
	}
	if star >= 0 {
		for i, sc := range a.stack {
			if v, ok := sc.(*systemScreen); ok && v.star == star {
				a.popTo(i)
				return
			}
		}
	}
	for i, sc := range a.stack {
		if _, ok := sc.(*starmapScreen); ok {
			a.popTo(i)
			if star >= 0 {
				a.push(newSystemScreen(star))
			}
			return
		}
	}
}

// popTo drops every screen above stack index i (i not re-entered).
func (a *app) popTo(i int) {
	for j := i + 1; j < len(a.stack); j++ {
		a.stack[j] = nil
	}
	a.stack = a.stack[:i+1]
}
