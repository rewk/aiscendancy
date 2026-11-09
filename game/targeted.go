package game

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

// The targeted specials and the two area ones (docs/rules.md "Ship
// specials"): aimed at a hostile ship, an allied or own ship, a hostile
// planet, or every ship around. targetCheck has refused a bad target: an
// effect reads t.ship() (or the planet) as valid. Numbers from gizmos.txt,
// quotes gizhelp.txt.

// ownerIn is the empire of es whose Ships hold s, nil none.
func ownerIn(es []*Empire, s *Starship) *Empire {
	for _, e := range es {
		if slices.Contains(e.Ships, s) {
			return e
		}
	}
	return nil
}

// hurt lands d hull points on t (nothing when Invulnerable); at 0 hull t
// is destroyed, credited to e's s.
func hurt(e *Empire, s, t *Starship, d int) []Report {
	if t.Invulnerable || d <= 0 {
		return nil
	}
	t.Hull -= d
	t.dealt(e.Index, d)
	if t.Hull > 0 {
		return nil
	}
	es := e.peerList()
	return destroyedIn(es, e, s.Name, true, ownerIn(es, t), t)
}

// firstOf is the index of the first part of t for which is holds, −1
// none.
func firstOf(t *Starship, is func(Gizmo) bool) int { return slices.IndexFunc(t.Design.Parts, is) }

func isCat(c GizmoCat) func(Gizmo) bool { return func(g Gizmo) bool { return Gizmos[g].Cat == c } }

// destroyPart is dev's report of tg losing part k, and its outcome.
func destroyPart(s *Starship, dev string, tg *Starship, k int) (string, string) {
	lost := tg.Design.Parts[k]
	tg.LosePart(k)
	return fmt.Sprintf("%s's %s destroys %s's %s", s.Name, dev, tg.Name, lost), fmt.Sprintf("%s destroyed", lost)
}

// Molecular Tie Down: "temporarily immobilizes it": TiedDown.
func useTieDown(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	tg.TiedDown = true
	return fmt.Sprintf("%s ties down %s", s.Name, tg.Name), "tied down", nil
}

// Intellect Scrambler: "turn an experienced crew into a group of bumbling
// rookies": Scrambled, its armed shots dropped (ASSUMPTION: no crew
// experience modelled, the rookies cannot fire today).
func useScrambler(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	tg.Scrambled, tg.Armed = true, nil
	return fmt.Sprintf("%s scrambles the crew of %s", s.Name, tg.Name), "scrambled", nil
}

// Brunswik Dissipator: "drains the target ship's generators": its energy
// 0, its armed shots dropped, until the Refill.
func useBrunswik(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	tg.Power, tg.Armed = 0, nil
	return fmt.Sprintf("%s drains the generators of %s", s.Name, tg.Name), "drained", nil
}

// refuseDisarmer: the Disarmer and the Backfirer need a weapon aboard
// the target.
func refuseDisarmer(e *Empire, s *Starship, part int, t Target) error {
	if firstOf(t.ship(), isCat(Weapon)) < 0 {
		return errors.New("no weapon aboard")
	}
	return nil
}

// Disarmer: "selectively destroys weapons": the target's first Weapon
// lost.
func useDisarmer(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	text, out := destroyPart(s, "Disarmer", tg, firstOf(tg, isCat(Weapon)))
	return text, out, nil
}

// carried is the index of the target's first Colonizer or Invasion Module
// with a live count left, −1 none.
func carried(t *Starship) int {
	return firstOf(t, func(g Gizmo) bool {
		return g == Colonizer && t.Cargo > 0 || g == InvasionModule && t.Invaders > 0
	})
}

// refuseContainment: a live Colonizer or module is needed.
func refuseContainment(e *Empire, s *Starship, part int, t Target) error {
	if carried(t.ship()) < 0 {
		return errors.New("no colonizer nor invasion module aboard")
	}
	return nil
}

// Containment Device: "destroys one colonizer or invasion module": the
// first live one lost, its count one down.
func useContainment(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	k := carried(tg)
	if tg.Design.Parts[k] == Colonizer {
		tg.Cargo--
	} else {
		tg.Invaders--
	}
	text, out := destroyPart(s, "Containment Device", tg, k)
	return text, out, nil
}

// Shield Blaster: "temporarily shuts down the defensive systems":
// ShieldsDown (Shields kept, back tomorrow).
func useShieldBlaster(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	tg.ShieldsDown = true
	return fmt.Sprintf("%s blasts the shields of %s down", s.Name, tg.Name), "shields down", nil
}

// Backfirer: "The more weapons the target ship carries, the more it is
// damaged by its own weapons": Σ Level of its Weapons, not absorbed
// (ASSUMPTION: internal).
func useBackfirer(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	d, _ := tg.Design.sum(Weapon)
	text, out := damaged(s, "Backfirer", tg, d)
	return text, out, hurt(e, s, tg, d)
}

// damaged is a damaging effect's report line, before hurt lands it.
func damaged(s *Starship, dev string, tg *Starship, d int) (string, string) {
	if tg.Invulnerable {
		d = 0
	}
	return fmt.Sprintf("%s's %s: %d damage to %s", s.Name, dev, d, tg.Name), fmt.Sprintf("-%d", d)
}

// Moving Part Exploiter: "The more intricate and advanced the equipment
// ..., the more internal damage": one hull point per part (ASSUMPTION),
// not absorbed.
func useExploiter(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	d := len(tg.Design.Parts)
	text, out := damaged(s, "Moving Part Exploiter", tg, d)
	return text, out, hurt(e, s, tg, d)
}

// refuseGizmogrifier: a target with no part has nothing to lock onto.
func refuseGizmogrifier(e *Empire, s *Starship, part int, t Target) error {
	if len(t.ship().Design.Parts) == 0 {
		return errors.New("no part aboard")
	}
	return nil
}

// Gizmogrifier: "destroy the first major device it locks onto": Parts[0]
// lost (ASSUMPTION).
func useGizmogrifier(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	text, out := destroyPart(s, "Gizmogrifier", t.ship(), 0)
	return text, out, nil
}

// refuseSpecialty: a Special part is needed.
func refuseSpecialty(e *Empire, s *Starship, part int, t Target) error {
	if firstOf(t.ship(), isCat(Special)) < 0 {
		return errors.New("no special aboard")
	}
	return nil
}

// Specialty Blaster: "destroys one specialized device": the first Special
// lost.
func useSpecialty(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	text, out := destroyPart(s, "Specialty Blaster", tg, firstOf(tg, isCat(Special)))
	return text, out, nil
}

// Disintegrator: "cannot be blocked by normal defenses ... can be used
// only once": the target destroyed outright; an Invulnerable one shrugs
// it off (ASSUMPTION: "will not allow the ship to come to harm" wins).
func useDisintegrator(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	if tg.Invulnerable {
		return fmt.Sprintf("%s's Disintegrator is shrugged off by %s", s.Name, tg.Name), "shrugged off", nil
	}
	es := e.peerList()
	text := fmt.Sprintf("%s's Disintegrator: %s disintegrates", s.Name, tg.Name)
	return text, "destroyed", destroyedIn(es, e, s.Name, true, ownerIn(es, tg), tg)
}

// Myrmidonic Carbonizer: "There is an ideal range for the Carbonizer, at
// which it is the most powerful weapon known": carbonizerLevel less the
// target's Absorb. Ship targets only for now.
func useCarbonizer(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	d := shotDamage(carbonizerLevel(dist(s.Pos, tg.Pos)), tg.Absorb())
	text, out := damaged(s, "Myrmidonic Carbonizer", tg, d)
	return text, out, hurt(e, s, tg, d)
}

// carbonizerLevel is round(Level × (1 − |d − ideal| / ideal)) ≥ 0, ideal
// = Special1 × RangeUnit (ASSUMPTION: a triangle, 0 at the user and at
// 2 × ideal = Range).
func carbonizerLevel(d float64) int {
	in := &Gizmos[MyrmidonicCarbonizer]
	ideal := float64(in.Special1) * RangeUnit
	return max(0, int(math.Round(float64(in.Level)*(1-math.Abs(d-ideal)/ideal))))
}

// orbAmount is what the Sacrificial Orb moves: Special1, at most the
// target's damage.
func orbAmount(t *Starship) int { return min(Gizmos[SacrificialOrb].Special1, t.MaxHull()-t.Hull) }

// refuseOrb: nothing to repair on a full hull; the user must survive.
func refuseOrb(e *Empire, s *Starship, part int, t Target) error {
	n := orbAmount(t.ship())
	switch {
	case n <= 0:
		return errors.New("nothing to repair")
	case s.Hull <= n:
		return errors.New("would scuttle the ship")
	}
	return nil
}

// Sacrificial Orb: "repair damage to another ship by absorbing that
// damage itself": orbAmount moves from the user's Hull to the target's.
func useOrb(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	n := orbAmount(tg)
	tg.Hull += n
	tg.healed()
	s.Hull -= n
	return fmt.Sprintf("%s takes %d hull off %s", s.Name, n, tg.Name), fmt.Sprintf("+%d hull", n), nil
}

// refuseCoupler: half the energy left after the cost must be something.
func refuseCoupler(e *Empire, s *Starship, part int, t Target) error {
	if (s.Power-Gizmos[PlasmaCoupler].Power)/2 <= 0 {
		return errors.New("no energy to beam")
	}
	return nil
}

// Plasma Coupler: "beam some of its power to another ship": half the
// energy left after the cost (ASSUMPTION: no number in the table).
func useCoupler(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	tg := t.ship()
	n := s.Power / 2
	s.Power -= n
	tg.Power += n
	return fmt.Sprintf("%s beams %d energy to %s", s.Name, n, tg.Name), fmt.Sprintf("+%d energy", n), nil
}

// around is spaceNear(s.Star, s.Pos, r, s), of empires at War with e
// only when hostile: collected before any damage lands.
func around(e *Empire, s *Starship, r float64, hostile bool) []*Starship {
	var out []*Starship
	for _, o := range e.peerList() {
		if hostile && (o == e || e.StanceWith(o.Index) != War) {
			continue
		}
		for _, x := range o.Ships {
			if x != s && x.Star == s.Star && x.InSpace() && dist(s.Pos, x.Pos) <= r+1e-9 {
				out = append(out, x)
			}
		}
	}
	return out
}

// smartBombReach is the Smart Bomb's blast radius: Special1 × RangeUnit.
func smartBombReach() float64 { return float64(Gizmos[SmartBomb].Special1) * RangeUnit }

// refuseSmartBomb: a hostile ship must be within reach.
func refuseSmartBomb(e *Empire, s *Starship, part int, t Target) error {
	if len(around(e, s, smartBombReach(), true)) == 0 {
		return errors.New("no hostile ship in range")
	}
	return nil
}

// Smart Bomb: "fires in multiple directions at every ship it has
// determined to be hostile ... One use expends it": blast on every
// hostile ship within smartBombReach.
func useSmartBomb(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	vs := around(e, s, smartBombReach(), true)
	n := fmt.Sprintf("%d %s", len(vs), plural(len(vs), "ship", "ships"))
	return fmt.Sprintf("%s's Smart Bomb strikes %s", s.Name, n), n + " struck", blast(e, s, vs, Gizmos[SmartBomb].Level)
}

// blast lands shotDamage(level, Absorb) on each of vs in order, the kills
// credited to s.
func blast(e *Empire, s *Starship, vs []*Starship, level int) []Report {
	var reps []Report
	for _, v := range vs {
		reps = append(reps, hurt(e, s, v, shotDamage(level, v.Absorb()))...)
	}
	return reps
}

// Self Destructotron: "destroys the ship using it ... greatly damages
// other ships nearby": blast on every other ship (its own empire's too)
// within Range, then the user destroyed (no Destroyed report of its own
// but its lostAboard).
func useDestructotron(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	in := &Gizmos[SelfDestructotron]
	vs := around(e, s, float64(in.Range)*RangeUnit, false)
	reps := blast(e, s, vs, in.Level)
	destroyIn(e.peerList(), s)
	reps = append(reps, lostAboard(e, s)...)
	return fmt.Sprintf("%s self-destructs", s.Name), "self-destructed", reps
}

// phaseCell is the first surface cell (Planet.Cells order) of p holding a
// structure other than the Colony Base, false none.
func phaseCell(p *Planet) (Cell, bool) {
	for _, c := range p.Cells() {
		if sq := p.Surface[c]; sq != nil && sq.Item != NoItem && sq.Item != ColonyBase {
			return c, true
		}
	}
	return Cell{}, false
}

// refusePhaseBomb: a structure besides the base is needed.
func refusePhaseBomb(e *Empire, s *Starship, part int, t Target) error {
	if _, ok := phaseCell(e.Planet(s.Star, t.Index)); !ok {
		return errors.New("nothing to destroy")
	}
	return nil
}

// Phase Bomb: "destroys structures on the planet's surface. It can be
// used only once": phaseCell demolished (the workers freed), the owner's
// Razed report; an owner IsInvincible shrugs it off.
func usePhaseBomb(e *Empire, s *Starship, part int, t Target) (string, string, []Report) {
	p := e.Planet(s.Star, t.Index)
	if o := e.peerList(); p.Owner < len(o) && o[p.Owner].IsInvincible(e.Day) {
		return fmt.Sprintf("%s's Phase Bomb is shrugged off by %s", s.Name, p.Name), "shrugged off", nil
	}
	c, _ := phaseCell(p)
	it := p.Surface[c].Item
	what := it.String() // e's texts: unnamed under a Surface Cloaker e does not see through
	if !e.SeesSurface(p) {
		what = "something"
	}
	p.Demolish(c)
	ev := Event{Kind: Razed, Item: it, Tech: NoTech, Star: s.Star, Planet: t.Index, Other: -1,
		Text: fmt.Sprintf("%s lost on %s", it, p.Name)}
	return fmt.Sprintf("%s's Phase Bomb: %s razed on %s", s.Name, what, p.Name), what + " razed", []Report{{p.Owner, ev}}
}
