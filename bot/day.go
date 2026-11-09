package bot

import (
	"errors"
	"fmt"

	"github.com/rewk/aiscendancy/game"
)

// DayRun is one day of g in steps, for the window's round mode (the
// player plays a combat's rounds one by one): Begin has
// every bot Act; the combat stars (CombatStars, computed once,
// ascending) are played in turn, a star by Round as the player steps it
// or by Auto; Finish plays what is left and closes the day. At a star,
// round r of this close: each empire with a bot plays it while the
// day's rounds there (g.RoundsAt, the mid-day Steps counted) are below
// its rounds cap (arming by Round before g.Round), one without when the
// round is stepped (Round's manual) or r is 0; the star is done after a
// round where nothing moved or fired, or once the day's rounds there
// reach RoundCap. A star at RoundCap already is dropped unplayed.
type DayRun struct {
	g      *game.Game
	bots   []*Bot
	caps   []int
	passed []bool
	stars  []int // the combat stars left, the current first
	r      int   // rounds this close played at stars[0]: the player's round 0
	reps   []game.Report
	rounds []game.RoundLog
}

// Begin starts a day of g: every bot Acts (orders only), its News kept.
func Begin(g *game.Game, bots []*Bot) *DayRun {
	d := &DayRun{g: g, bots: bots, caps: make([]int, len(g.Empires)), passed: make([]bool, len(g.Empires))}
	g.BeginClose()
	for _, b := range bots {
		if b != nil {
			b.Act()
		}
	}
	for _, b := range bots {
		if b != nil {
			d.reps = append(d.reps, b.News()...)
		}
	}
	for i := range d.caps {
		if b := d.bot(i); b != nil {
			d.caps[i] = b.rounds()
		}
	}
	d.stars = g.CombatStars()
	return d
}

// bot is empire i's bot, nil for the player.
func (d *DayRun) bot(i int) *Bot {
	if i < len(d.bots) {
		return d.bots[i]
	}
	return nil
}

// skip drops the leading stars whose day's rounds reached RoundCap
// already (mid-day Steps): none is left to play there.
func (d *DayRun) skip() {
	for len(d.stars) > 0 && d.g.RoundsAt(d.stars[0]) >= RoundCap {
		d.stars, d.r = d.stars[1:], 0
	}
}

// Star is the combat star playing now and the rounds this close played
// there; false when none is left.
func (d *DayRun) Star() (star, rounds int, ok bool) {
	if d.skip(); len(d.stars) == 0 {
		return -1, 0, false
	}
	return d.stars[0], d.r, true
}

// Round plays one round at the current star, the empires without a bot
// playing it when manual (else at round 0 only), and moves on to the next
// star when this one is done. Its Reports and log are also kept for
// Finish's returns; acted as game.Round's. None left: nothing.
func (d *DayRun) Round(manual bool) (acted bool, reps []game.Report, log game.RoundLog) {
	if d.skip(); len(d.stars) == 0 {
		return false, nil, game.RoundLog{}
	}
	st := d.stars[0]
	n := d.g.RoundsAt(st)
	for i := range d.passed {
		b := d.bot(i)
		d.passed[i] = b == nil && !manual && d.r > 0 || b != nil && n >= d.caps[i]
		if b != nil && !d.passed[i] {
			b.Round(st)
			reps = append(reps, b.News()...) // its specials' uses
		}
	}
	acted, rr, log := d.g.Round(st, d.passed)
	reps = append(reps, rr...)
	d.reps = append(d.reps, reps...)
	d.rounds = append(d.rounds, log)
	if d.r++; !acted || d.g.RoundsAt(st) >= RoundCap {
		d.stars, d.r = d.stars[1:], 0
	}
	return acted, reps, log
}

// Auto plays the current star to its end, unstepped (the player passing
// past round 0, or past the rounds stepped already), returning its
// Reports and logs; none left: nothing.
func (d *DayRun) Auto() (reps []game.Report, logs []game.RoundLog) {
	if d.skip(); len(d.stars) == 0 {
		return nil, nil
	}
	for st := d.stars[0]; len(d.stars) > 0 && d.stars[0] == st; {
		_, rr, lg := d.Round(false)
		reps, logs = append(reps, rr...), append(logs, lg)
	}
	return reps, logs
}

// Finish plays every star left by Auto, then g.EndDay, and every bot
// Notes. Reports and RoundLogs are the whole day's, in play order.
func (d *DayRun) Finish() (evs [][]game.Event, reps []game.Report, rounds []game.RoundLog) {
	for len(d.stars) > 0 {
		d.Auto()
	}
	evs = d.g.EndDay()
	for _, b := range d.bots {
		if b != nil {
			b.Note()
		}
	}
	return evs, d.reps, d.rounds
}

// Step plays one mid-day round at star, stepped by the window's player
// (the empire without a bot, bots indexed as in Day): refused at RoundCap
// rounds there today, or by g.StepRound (not engaged) before any bot
// arms; else each bot below its rounds cap there plays Round (its News
// first in reps), the others passing, then g.StepRound for the player.
func Step(g *game.Game, bots []*Bot, star int) (acted bool, reps []game.Report, log game.RoundLog, err error) {
	d := &DayRun{g: g, bots: bots}
	var player *game.Empire
	for i, e := range g.Empires {
		if d.bot(i) == nil {
			player = e
			break
		}
	}
	switch {
	case player == nil:
		return false, nil, game.RoundLog{}, errors.New("no player: every empire has a bot")
	case g.RoundsAt(star) >= RoundCap:
		return false, nil, game.RoundLog{}, fmt.Errorf("no round left today at %s", g.Cluster.Stars[star].Name)
	case !g.Engaged(player, star):
		return g.StepRound(player, star, nil) // its refusal, nothing armed
	}
	passed := make([]bool, len(g.Empires))
	n := g.RoundsAt(star)
	for i := range g.Empires {
		if b := d.bot(i); b != nil {
			if passed[i] = n >= b.rounds(); !passed[i] {
				b.Round(star)
				reps = append(reps, b.News()...) // its specials' uses
			}
		}
	}
	acted, rr, log, err := g.StepRound(player, star, passed)
	return acted, append(reps, rr...), log, err
}
