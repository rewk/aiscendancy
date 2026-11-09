package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/game"
)

// The day and combat mode (docs/screens.md "Combat mode"). endDay runs
// a day in one go (bot.DayRun): beginDay has the bots Act; finishDay
// plays every combat star by Auto (the player passing past the close's
// first round at each) and closes the day. During the day a system
// screen at a star where the player is engaged is in combat mode
// (inCombat, derived each frame): its left button, Space and the mouse's
// forward button step one round there (stepRound, bot.Step), logged at
// once. Ending the day warns first when ships of the player's with
// usable energy stand in such a fight (restless, warning.go).

// dayRun is the window's side of the running day: the snapshots
// finishDay diffs, the topmost system screen at the start (view) with
// its cast and lane watch, and the bot.DayRun.
type dayRun struct {
	known    [game.NumTechs]bool  // the discoveries' diff (learnt)
	visited  []bool               // the explorations' (explore)
	colonies int                  // the colony reports' count (founded)
	free     map[*game.Colony]int // the Grew filter's (freed)
	view     *systemScreen
	cast     []body
	watch    []*game.Starship
	run      *bot.DayRun
	offers   []game.Offer // the offers pending to the player (proposed)
}

// topSystem is the topmost system screen in the stack, nil none.
func (a *app) topSystem() *systemScreen {
	for _, s := range slices.Backward(a.stack) {
		if v, ok := s.(*systemScreen); ok {
			return v
		}
	}
	return nil
}

// beginDay snapshots today and has the bots Act (bot.Begin); the topmost
// system screen's replay is dropped (a new day replaces it), its cast and
// lane watch taken.
func (a *app) beginDay() dayRun {
	t := dayRun{known: a.e.Research.Known, visited: slices.Clone(a.e.Visited), colonies: len(a.e.Colonies),
		free: map[*game.Colony]int{}, offers: a.g.PendingOffers(a.player)}
	for _, c := range a.e.Colonies {
		t.free[c] = c.Free()
	}
	if v := a.topSystem(); v != nil {
		v.replay = nil // its shots' and moves' sounds to come with it, as endReplay's
		v.cues = slices.DeleteFunc(v.cues, func(c cue) bool { return c.t > v.tick && !c.lane })
		v.snapRings()
		t.view, t.cast, t.watch = v, v.replayCast(), v.laneWatch()
	}
	t.run = bot.Begin(a.g, a.bots)
	return t
}

// fightStars is the stars, ascending, of a fight open to the player: it
// is Engaged there and the day's rounds there are below RoundCap. A
// system screen at one is in combat mode; the starmap marks them.
func fightStars(a *app) []int {
	if a.g == nil || a.e == nil {
		return nil
	}
	var out []int
	for _, st := range a.g.CombatStars() {
		if a.g.RoundsAt(st) < bot.RoundCap && a.g.Engaged(a.e, st) {
			out = append(out, st)
		}
	}
	return out
}

// restless is the fight stars (fightStars) where one of the player's
// ships out of a lane, not refitting, has UsableEnergy: the day's end
// warns of them.
func (a *app) restless() []int {
	var out []int
	for _, st := range fightStars(a) {
		if slices.ContainsFunc(a.e.Ships, func(s *game.Starship) bool {
			return s.Star == st && s.Lane < 0 && a.e.Refitting(s) == nil && s.UsableEnergy()
		}) {
			out = append(out, st)
		}
	}
	return out
}

// inCombat is s's combat mode, derived (never stored, so a screen opened
// at a fight, or a fight starting under it, is in it at once): the
// player Engaged at s's star with rounds left there today.
func (s *systemScreen) inCombat(a *app) bool {
	return a.g != nil && a.e != nil && a.g.Engaged(a.e, s.star) && a.g.RoundsAt(s.star) < bot.RoundCap
}

// roundsDone reports s's star at RoundCap today: no round left there.
func (s *systemScreen) roundsDone(a *app) bool {
	return a.g != nil && a.g.RoundsAt(s.star) >= bot.RoundCap
}

// stepRound is one mid-day round at v's star (bot.Step), the player's
// armed shots and orders played: replayed after any replay running, its
// lanes entered flaring, its news logged at once (stamped today: no
// close is pending) and toasted with the round's number (RoundsAt after
// the step: the round just played). A refusal is toasted.
func (a *app) stepRound(v *systemScreen) {
	v.snapRings()
	cast, watch := v.replayCast(), v.laneWatch()
	_, reps, lg, err := bot.Step(a.g, a.bots, v.star)
	if err != nil {
		a.toastFor(err.Error(), true)
		return
	}
	v.startReplay(a.e, cast, []game.RoundLog{lg}, specialUses(reps))
	v.startFlares(watch)
	a.record(reps)
	a.toastFor(roundNews(a.g, a.player, a.g.RoundsAt(v.star), cast, lg), false)
}

// finishDay plays the combat stars left round by round (Auto's order,
// each round's specials kept with its log for the round log), closes the day
// (DayRun.Finish) and returns the player's events: the bots' news and
// the rounds' first (Begin's, then in play order), then the day's events
// (a Grew only when freed), all logged. The topmost system screen, when
// still today's view, replays the rounds played at its star, then the
// close's moves (its cast taken after the rounds: Auto per star before
// Finish, as Finish's own loop) and its planets' glide (their Pos
// snapped with that cast), the lanes entered flaring at the end.
func (a *app) finishDay(t dayRun) []game.Event {
	d := t.run
	var rounds []game.RoundLog
	var uses [][]*game.SpecialUse
	for _, _, ok := d.Star(); ok; _, _, ok = d.Star() {
		_, reps, lg := d.Round(false)
		rounds, uses = append(rounds, lg), append(uses, specialUses(reps))
	}
	v := a.topSystem()
	var cast []body
	var watch []*game.Starship
	var in []laneShip
	var orbits []game.Vec3
	if v != nil {
		cast, watch, in, orbits = v.replayCast(), v.laneWatch(), v.inbound(), v.planetsNow()
		if v == t.view {
			v.startReplay(a.e, t.cast, rounds, uses...)
			watch = append(watch, t.watch...)
		}
	}
	evs, reps, _ := d.Finish() // every star played: EndDay alone
	if v != nil {
		v.closeReplay(a.e, cast, orbits)
		v.startFlares(watch)
		v.startExits(in)
	}
	for _, x := range popNews(a.e, t.free, evs[a.player]) {
		reps = append(reps, game.Report{Empire: a.player, Event: x})
	}
	ev := a.record(reps)
	a.learnt(t.known, ev)
	a.explore(t.visited)
	a.founded(ev, t.colonies)
	a.contacted(ev)
	a.proposed(t.offers)
	if slices.ContainsFunc(ev, func(x game.Event) bool { return x.Kind == game.AbilityReady }) {
		a.charged = true
	}
	if slices.ContainsFunc(ev, func(x game.Event) bool { return x.Kind == game.Won && x.Other < 0 }) {
		a.victory, a.winTxt = a.e.Won, nil // the popup (victory.go), its text read anew: same kind, another day after a reload
	}
	return ev
}

// roundNews is round n's toast for player: the hits its side dealt and
// took (hull points, then what the shields absorbed) and the ships
// destroyed, theirs and ours (owners read off cast); a zero is left
// out, a quiet round says so.
func roundNews(g *game.Game, player, n int, cast []body, l game.RoundLog) string {
	var gave, took [3]int // hits, hull, absorbed
	for _, f := range l.Shots {
		if f.Damage+f.Absorbed == 0 {
			continue
		}
		target := f.TargetEmpire
		if f.AtPlanet && f.Target == nil {
			target = g.Cluster.Stars[l.Star].System.Planets[f.TargetPlanet].Owner
		}
		switch {
		case f.ShooterEmpire == player:
			gave = [3]int{gave[0] + 1, gave[1] + f.Damage, gave[2] + f.Absorbed}
		case target == player:
			took = [3]int{took[0] + 1, took[1] + f.Damage, took[2] + f.Absorbed}
		}
	}
	killed, lost := 0, 0
	for _, d := range l.Destroyed {
		if i := slices.IndexFunc(cast, func(b body) bool { return b.ship == d }); i >= 0 && cast[i].owner == player {
			lost++
		} else {
			killed++
		}
	}
	var out []string
	for _, h := range []struct {
		verb string
		n    [3]int
	}{{"dealt", gave}, {"took", took}} {
		if h.n[0] == 0 {
			continue
		}
		s := fmt.Sprintf("%s %d hit%s", h.verb, h.n[0], plural(h.n[0]))
		var what []string
		if h.n[1] > 0 {
			what = append(what, fmt.Sprintf("%d hull", h.n[1]))
		}
		if h.n[2] > 0 {
			what = append(what, fmt.Sprintf("%d shields", h.n[2]))
		}
		if len(what) > 0 {
			s += " (" + strings.Join(what, ", ") + ")"
		}
		out = append(out, s)
	}
	if killed > 0 {
		out = append(out, fmt.Sprintf("destroyed %d", killed))
	}
	if lost > 0 {
		out = append(out, fmt.Sprintf("lost %d", lost))
	}
	if len(out) == 0 {
		return fmt.Sprintf("Round %d: no hits", n)
	}
	return fmt.Sprintf("Round %d: %s", n, strings.Join(out, ", "))
}
