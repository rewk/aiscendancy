package game

import (
	"errors"
	"fmt"
	"slices"
)

// Ship combat. Sources: the 2005 GameFAQs guide (hull strengths, damage =
// the weapon's Level, Uses = shots a day), the fan manual (shields take
// their score off every shot), gizhelp.txt (shields draw only while on).
// A shot spends the weapon's Power and one Use (0: no limit) and does
// takeShot off the target's Hull: shotDamage while its raised shields
// pay their draw, nothing for the hit that finds them short (dark after
// it), the full Level on dark or no shields; at 0 hull the ship is
// destroyed. Fire is armed (Empire.Arm) and resolved by Game.Round
// (rounds.go), the one combat rule for the window, the bots and the
// fuzzer.

// CanFire reports why the ship cannot fire part at t, nil when it can: a
// Weapon part with a shot left, the energy for it (the shields' draw is
// already off Power while on), both ships in the same system out of the
// lanes, t within WeaponRange and not itself.
func (s *Starship) CanFire(c *Cluster, part int, t *Starship) error {
	if part < 0 || part >= len(s.Design.Parts) {
		return fmt.Errorf("no part %d", part)
	}
	w := &Gizmos[s.Design.Parts[part]]
	switch {
	case w.Cat != Weapon:
		return fmt.Errorf("%s is not a weapon", w.Name)
	case t == nil:
		return errors.New("no target")
	case w.Uses > 0 && s.Shots[part] >= w.Uses:
		return fmt.Errorf("%s: no shot left today", w.Name)
	case s.Power < w.Power:
		return fmt.Errorf("%s: not enough power (%d of %d)", w.Name, s.Power, w.Power)
	case s.Lane >= 0 || t.Lane >= 0:
		return errors.New("in a star lane")
	case t.Star != s.Star:
		return errors.New("not in the same system")
	case dist(s.Pos, t.Pos) > s.WeaponRange(w)+1e-9:
		return fmt.Errorf("%s: out of range", w.Name)
	case t == s:
		return errors.New("cannot fire on itself")
	}
	return nil
}

// ReadyWeapon is the part s fires at t: its chosen Weapon when that
// CanFire, else the first that can; when none can, the first weapon's
// refusal, or "no weapon".
func (s *Starship) ReadyWeapon(c *Cluster, t *Starship) (part int, err error) {
	if s.Weapon >= 0 && s.Weapon < len(s.Design.Parts) && s.CanFire(c, s.Weapon, t) == nil {
		return s.Weapon, nil
	}
	for p, g := range s.Design.Parts {
		if Gizmos[g].Cat != Weapon {
			continue
		}
		e := s.CanFire(c, p, t)
		if e == nil {
			return p, nil
		}
		if err == nil {
			err = e
		}
	}
	if err == nil {
		err = errors.New("no weapon")
	}
	return -1, err
}

// shotDamage is the weapon's Level less the target's Absorb, never below 0.
func shotDamage(level, absorb int) int { return max(0, level-absorb) }

// DamagePerDay is what s inflicts on t in a full day of firing without
// moving (PowerPerDay spent as weaponsDamage spends it against t's
// shields as they stand: Absorb, ShieldDraw, its energy left now, all of
// it kept for the draws; the shields' reservation off s's supply while
// s.Shields), range and s's Power, Shots and day flags ignored: the
// tooltip's estimate.
func (s *Starship) DamagePerDay(t *Starship) int {
	return weaponsDamage(s.Design, s.Design.PowerPerDay(), targetShields{t.Absorb(), t.Design.ShieldDraw(), t.Power}, s.Shields)
}

// DesignDamage is DamagePerDay for a fresh ship of design d (shields on)
// against a shield score whose energy is unknown: the shields assumed to
// pay their draw for every shot, never knocked down (the knock-down hit
// does nothing, the later ones the full Level: either side of the truth).
// The bot's estimate (against 0: no shield, exact).
func DesignDamage(d Design, shield int) int {
	return weaponsDamage(d, d.PowerPerDay(), targetShields{shield, 0, 0}, true)
}

// DesignDamageOn is DesignDamage against a fresh ship of design t: its
// shields on when its supply carries their draw (Refill), the energy
// PowerPerDay less that reservation, all of it kept for the draws.
func DesignDamageOn(d, t Design) int {
	sh, draw := targetShields{}, t.ShieldDraw()
	if draw <= t.PowerPerDay() {
		sh = targetShields{t.ShieldScore(), draw, t.PowerPerDay() - draw}
	}
	return weaponsDamage(d, d.PowerPerDay(), sh, true)
}

// targetShields is a target's shields as weaponsDamage spends them:
// absorb while up, each shot taking draw off energy, the shot the energy
// cannot pay knocking them down (takeShot).
type targetShields struct{ absorb, draw, energy int }

// hit is takeShot on the estimate: the damage of a shot of level.
func (t *targetShields) hit(level int) int {
	switch {
	case t.absorb == 0:
		return level
	case t.energy < t.draw:
		t.absorb = 0
		return 0
	}
	t.energy -= t.draw
	return shotDamage(level, t.absorb)
}

// weaponsDamage is a day of rounds without a move from power: ShieldDraw
// off it once while shields (the reservation, as Refill: a draw power
// cannot carry leaves them off), then each round at most Generators weapons fire
// once, in part order, while they have Uses (0 counted as one) and the
// energy covers them (a dear one skipped, a cheaper later one fires),
// until a round fires nothing; each shot lands on t as takeShot does.
func weaponsDamage(d Design, power int, t targetShields, shields bool) int {
	draw := d.ShieldDraw()
	gens := d.Generators()
	var buf [32]int
	left := buf[:0]
	if len(d.Parts) > len(buf) {
		left = make([]int, 0, len(d.Parts))
	}
	for _, g := range d.Parts {
		left = append(left, max(1, Gizmos[g].Uses))
	}
	dmg := 0
	if shields && draw <= power {
		power -= draw
	}
	for {
		fired := 0
		for i, g := range d.Parts {
			if fired == gens {
				break
			}
			w := &Gizmos[g]
			if w.Cat != Weapon || left[i] == 0 || power < w.Power {
				continue
			}
			left[i]--
			power -= w.Power
			fired++
			dmg += t.hit(w.Level)
		}
		if fired == 0 {
			return dmg
		}
	}
}

// Report is an event and the empire (index in Game.Empires) whose news it
// is: one attack concerns two empires.
type Report struct {
	Empire int
	Event  Event
}

// Hostile reports whether a's ships may fire on b's: at War. The one hook
// of combat, orbit fire, the fog and the invasions.
func (g *Game) Hostile(a, b *Empire) bool { return a != b && b != nil && a.StanceWith(b.Index) == War }

// OwnerOf is the empire whose Ships hold s (pointer identity), nil when
// none does (a destroyed ship).
func (g *Game) OwnerOf(s *Starship) *Empire { return ownerIn(g.Empires, s) }

// EnemiesAt lists the ships of every empire Hostile to e at star, out of
// the lanes, in empire then Ships order.
func (g *Game) EnemiesAt(e *Empire, star int) []*Starship {
	var out []*Starship
	for _, o := range g.Empires {
		if !g.Hostile(e, o) {
			continue
		}
		for _, s := range o.Ships {
			if s.Star == star && s.Lane < 0 {
				out = append(out, s)
			}
		}
	}
	return out
}

// destroyedBy is destroyedIn over the game's empires.
func (g *Game) destroyedBy(se *Empire, shooter string, byShip bool, te *Empire, target *Starship) []Report {
	return destroyedIn(g.Empires, se, shooter, byShip, te, target)
}

// destroyedIn destroys target (of te) among es, shot down by shooter (of
// se: a ship's Name, byShip, or a planet's): te counts ShipsLost about se
// (not its own), and the two Destroyed Reports, se's then te's, each
// naming the other side's ship by its species alone and the star
// (a foreign ship is never named; the star to go
// there), se's not even that when the target stood in an orbit se does
// not SeesOrbit, then te's lostAboard.
func destroyedIn(es []*Empire, se *Empire, shooter string, byShip bool, te *Empire, target *Starship) []Report {
	destroyIn(es, target)
	if r := te.Rel(se.Index); r != nil && se != te {
		r.ShipsLost++
	}
	ev := Event{Kind: Destroyed, Item: NoItem, Tech: NoTech, Star: target.Star, Planet: -1, Other: -1}
	mine, theirs := ev, ev
	at := se.Cluster.Stars[target.Star].Name
	victim, killer := target.Name, shooter
	if se != te {
		victim = aShip(es, te.Index)
		if p := orbitOf(se.Cluster, target); p != nil && !se.SeesOrbit(p) {
			victim = fmt.Sprintf("something in %s's orbit", p.Name)
		}
		if byShip {
			killer = aShip(es, se.Index)
		}
	}
	mine.Text = fmt.Sprintf("%s destroys %s at %s", shooter, victim, at)
	theirs.Text = fmt.Sprintf("%s lost to %s at %s", target.Name, killer, at)
	return append([]Report{{se.Index, mine}, {te.Index, theirs}}, lostAboard(te, target)...)
}

// orbitOf is the planet whose orbit s stands in, nil none (in space, in
// a lane).
func orbitOf(c *Cluster, s *Starship) *Planet {
	if s.Lane >= 0 || s.Orbit < 0 {
		return nil
	}
	return c.Stars[s.Star].System.Planets[s.Orbit]
}

// lostAboard is owner's report of the colonists s carried (Starship.Pop)
// when destroyed, a Destroyed event after the ship's own; nil when none.
// The owner's news alone: another empire never learns a ship's cargo.
func lostAboard(owner *Empire, s *Starship) []Report {
	if s.Pop < 1 {
		return nil
	}
	return []Report{{owner.Index, Event{Kind: Destroyed, Item: NoItem, Tech: NoTech, Star: s.Star, Planet: -1, Other: -1,
		Text: fmt.Sprintf("%d %s lost aboard %s", s.Pop, plural(s.Pop, "colonist", "colonists"), s.Name)}}}
}

// CanFireAt is CanFire on a sighted ship: the bots aim through the fog.
func (s *Starship) CanFireAt(c *Cluster, part int, t Sighting) error {
	return s.CanFire(c, part, t.ship)
}

// Destroy removes s from its empire's Ships, order kept; false when no
// empire holds it. Another holder of the pointer must check OwnerOf.
func (g *Game) Destroy(s *Starship) bool { return destroyIn(g.Empires, s) }

// destroyIn is Destroy among es.
func destroyIn(es []*Empire, s *Starship) bool {
	for _, e := range es {
		if i := slices.Index(e.Ships, s); i >= 0 {
			e.Ships = slices.Delete(e.Ships, i, i+1)
			return true
		}
	}
	return false
}
