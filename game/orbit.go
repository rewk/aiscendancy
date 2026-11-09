package game

import (
	"errors"
	"fmt"
	"slices"
)

// Planets in combat: a planet as a TARGET (its orbit cascade)
// and as a SHOOTER (its orbital weapons).
//
// A shot at a planet hits its Orbital Cloakers, then its orbital shields
// (slot order), then its orbital weapons, then the ships in its orbit
// hostile to the shooter;
// at a planet not PlanetHostile (the shooter's own, a neutral's), only
// those ships (an enemy in orbit of your colony). A
// structure absorbs nothing: the full damage goes on its hidden
// OrbitDamage; at its Strength it is destroyed, the excess lost. A ship in
// orbit takes the shot as from a ship. Sources: b-sting.nl "Shields vs.
// Weapons" (strengths 15 and 35, the damage never recovers); the cascade
// order and the weapons' and the cloaker's 15 are ours. A cloaker is
// a layer, not a shield: it guards nothing (orbitShielded).
//
// Orbital weapons fire like ship parts (battle.txt): armed per round
// (Empire.ArmPlanet), Shots a day, no energy (ASSUMPTION), ranged from the
// planet's Pos, at hostile ships in space, or at another planet's cascade
// as a ship's shot (Empire.ArmPlanetAt).

// OrbitalDefence is one orbital structure's combat row. Range is in
// RangeUnit, as a ship weapon's.
type OrbitalDefence struct {
	Strength int // hit points: a shot's full damage comes off them
	Damage   int // a weapon's damage per shot, 0 for a shield
	Shots    int // a weapon's shots a day
	Range    int // a weapon's reach, × RangeUnit
}

// Weapon reports a structure that fires (Damage > 0).
func (d OrbitalDefence) Weapon() bool { return d.Damage > 0 }

// OrbitalDefences is the table, by Item; a zero row is no defence. The
// weapons' damage and shots are the fan manual's; their Strength 15 and
// ranges 40/40/100 are ASSUMPTIONS, the Orbital Cloaker's 15
// too.
var OrbitalDefences = [NumItems]OrbitalDefence{
	OrbitalCloaker:           {Strength: 15},
	OrbitalShield:            {Strength: 15},
	OrbitalMegaShield:        {Strength: 35},
	OrbitalMissileBase:       {Strength: 15, Damage: 7, Shots: 1, Range: 40},
	ShortRangeOrbitalWhopper: {Strength: 15, Damage: 7, Shots: 3, Range: 40},
	LongRangeOrbitalWhopper:  {Strength: 15, Damage: 8, Shots: 3, Range: 100},
}

// Defence is it's row, false when it is no orbital defence.
func (it Item) Defence() (OrbitalDefence, bool) {
	if it < 0 || it >= NumItems || OrbitalDefences[it].Strength == 0 {
		return OrbitalDefence{}, false
	}
	return OrbitalDefences[it], true
}

// DefenceLeft is the hit points left to the defence in slot, known to the
// owner only (as the original; views read this, never OrbitDamage).
func (p *Planet) DefenceLeft(viewer *Empire, slot int) (left int, ok bool) {
	if viewer == nil || viewer.Index != p.Owner || slot < 0 || slot >= OrbitSlots {
		return 0, false
	}
	d, ok := p.Orbit[slot].Defence()
	if !ok {
		return 0, false
	}
	return d.Strength - p.OrbitDamage[slot], true
}

// PlanetHostile reports planet pl of star a target for e: not its own,
// owned or guarded by an empire Hostile to e.
func (g *Game) PlanetHostile(e *Empire, star, pl int) bool {
	p := g.Cluster.Stars[star].System.Planets[pl]
	switch {
	case p.Owner == e.Index:
		return false
	case p.Owner >= 0 && g.Hostile(e, g.empireAt(p.Owner)):
		return true
	}
	gd, ok := g.Guard(star, pl)
	return ok && g.Hostile(e, g.empireAt(gd))
}

// empireAt is Empires[i], nil out of range (Hostile is false on nil).
func (g *Game) empireAt(i int) *Empire {
	if i < 0 || i >= len(g.Empires) {
		return nil
	}
	return g.Empires[i]
}

// orbitHit is what a shot at a planet strikes: the structure in Slot
// (Ship nil), or Ship in orbit; ok false when nothing is left.
type orbitHit struct {
	Slot int
	Ship *Starship
	ok   bool
}

// orbitLayer is structure it's rank in the cascade: 0 an Orbital
// Cloaker, 1 a shield, 2 a weapon, −1 no defence.
func orbitLayer(it Item) int {
	d, ok := it.Defence()
	switch {
	case !ok:
		return -1
	case it == OrbitalCloaker:
		return 0
	case d.Weapon():
		return 2
	}
	return 1
}

// orbitTarget is the cascade's next layer at planet pl of star for a
// shot of by: the first cloaker, the first shield, the first weapon (all
// only at a planet PlanetHostile to by), the first ship in orbit Hostile
// to by, skipping those fell and shipFell report fallen (nil: none).
func (g *Game) orbitTarget(by *Empire, star, pl int, fell func(slot int) bool, shipFell func(*Starship) bool) orbitHit {
	p := g.Cluster.Stars[star].System.Planets[pl]
	if g.PlanetHostile(by, star, pl) {
		for layer := range 3 {
			for k, it := range p.Orbit {
				if orbitLayer(it) == layer && (fell == nil || !fell(k)) {
					return orbitHit{Slot: k, ok: true}
				}
			}
		}
	}
	for _, o := range g.Empires {
		if !g.Hostile(by, o) {
			continue
		}
		for _, s := range o.Ships {
			if s.Star == star && s.Lane < 0 && s.Orbit == pl && (shipFell == nil || !shipFell(s)) {
				return orbitHit{Slot: -1, Ship: s, ok: true}
			}
		}
	}
	return orbitHit{Slot: -1}
}

// CanHitPlanet reports why e may not arm at planet pl of star (neither
// PlanetHostile nor a hostile ship in its orbit, nothing to hit in
// orbit), nil when it may. An orbit e does not SeesOrbit (a foreign
// cloaker, fog.go) is judged from what e sees, never from what it hides:
// hostile through its owner alone, then a target whatever it holds
// (Round strikes the real cascade, or gives the shot back).
func (g *Game) CanHitPlanet(e *Empire, star, pl int) error {
	if star < 0 || star >= len(g.Cluster.Stars) || pl < 0 || pl >= len(g.Cluster.Stars[star].System.Planets) {
		return errors.New("no such planet")
	}
	p := g.Cluster.Stars[star].System.Planets[pl]
	if !e.SeesOrbit(p) {
		if !g.Hostile(e, g.empireAt(p.Owner)) {
			return fmt.Errorf("%s: not hostile", p.Name)
		}
		return nil
	}
	hit := g.orbitTarget(e, star, pl, nil, nil)
	if !g.PlanetHostile(e, star, pl) && !hit.ok {
		return fmt.Errorf("%s: not hostile", p.Name)
	}
	if !hit.ok {
		return fmt.Errorf("%s: nothing to hit in orbit", p.Name)
	}
	return nil
}

// ArmAtPlanet arms s's weapon part at planet pl's cascade for this round,
// as armShooter, CanHitPlanet and PlanetInRange allow (Round checks the
// range again, its safety net).
func (e *Empire) ArmAtPlanet(s *Starship, part int, pl int) error {
	if _, err := e.armShooter(s, part); err != nil {
		return err
	}
	g := &Game{Cluster: e.Cluster, Empires: e.peerList()}
	if err := g.CanHitPlanet(e, s.Star, pl); err != nil {
		return err
	}
	if err := s.PlanetInRange(e.Cluster, part, pl); err != nil {
		return err
	}
	s.Armed = append(s.Armed, Shot{Part: part, AtPlanet: true, Planet: pl})
	if e.journal != nil {
		en := e.shipEntry("arm", s)
		en.Part, en.Star, en.Planet, en.Other = part, s.Star, pl, e.Planet(s.Star, pl).Owner
		e.journal.Record(en)
	}
	return nil
}

// PlanetInRange refuses weapon part of s out of its WeaponRange of planet
// pl of its star, CanFire's distance rule (nil for a part or planet that
// is none: armShooter and CanHitPlanet say so).
func (s *Starship) PlanetInRange(c *Cluster, part, pl int) error {
	if part < 0 || part >= len(s.Design.Parts) || s.Star < 0 || s.Star >= len(c.Stars) {
		return nil
	}
	ps := c.Stars[s.Star].System.Planets
	if pl < 0 || pl >= len(ps) {
		return nil
	}
	if w := &Gizmos[s.Design.Parts[part]]; dist(s.Pos, ps[pl].Pos) > s.WeaponRange(w)+1e-9 {
		return fmt.Errorf("%s: out of range", w.Name)
	}
	return nil
}

// planetGun is ArmPlanet's and ArmPlanetAt's common refusals for the
// orbital weapon in slot of col: col not e's, no weapon there, armed
// already, no shot left, its Tractor Beam used this round.
func (e *Empire) planetGun(col *Colony, slot int) error {
	if !slices.Contains(e.Colonies, col) {
		return errors.New("not a colony of this empire")
	}
	if slot < 0 || slot >= OrbitSlots {
		return fmt.Errorf("no orbit slot %d", slot)
	}
	it := col.Planet.Orbit[slot]
	d, ok := it.Defence()
	switch {
	case !ok || !d.Weapon():
		return fmt.Errorf("orbit slot %d holds no orbital weapon", slot)
	case slices.ContainsFunc(col.Armed, func(a Shot) bool { return a.Part == slot }):
		return fmt.Errorf("%s: already armed this round", it)
	case col.OrbitShots[slot] >= d.Shots:
		return fmt.Errorf("%s: no shot left today", it)
	case col.Tractors > 0: // the round went to the beam (tractor.go)
		return errors.New("Tractor Beam used this round")
	}
	return nil
}

// ArmPlanet arms the orbital weapon in slot of col at t for this round:
// refused on planetGun's grounds and fireTarget's (no Ships check); a
// ship in orbit is reached through its planet (ArmPlanetAt). No energy,
// no generator; Round checks the range.
func (e *Empire) ArmPlanet(col *Colony, slot int, t Sighting) error {
	if err := e.planetGun(col, slot); err != nil {
		return err
	}
	tg := t.ship
	switch {
	case tg == nil || tg.Hull <= 0:
		return errors.New("no such target")
	case t.Empire == e.Index || e.StanceWith(t.Empire) != War: // Game.Hostile, without the Game
		return errors.New("not hostile")
	case tg.Lane >= 0:
		return errors.New("the target is in a star lane")
	case tg.Star != col.Star:
		return errors.New("not in the same system")
	case tg.Orbit >= 0:
		return fmt.Errorf("the target is in orbit of %s: fire at the planet", e.Planet(tg.Star, tg.Orbit).Name) // unnamed: a foreign ship never is
	}
	col.Armed = append(col.Armed, Shot{Part: slot, Target: t})
	if e.journal != nil {
		en := e.entry("arm")
		en.Colony, en.Slot, en.Star, en.Planet = col.Planet.Name, slot, col.Star, col.PlanetIdx
		en.Target, en.Other = tg.Name, t.Empire
		e.journal.Record(en)
	}
	return nil
}

// CanArmPlanetAt reports why ArmPlanetAt(col, slot, pl) would be refused,
// nil when it would arm: planetGun's grounds, then pl no planet of col's
// system or col's own, CanHitPlanet's verdict (judged from what e sees),
// then the weapon's Range × RangeUnit from col's planet to pl's.
func (e *Empire) CanArmPlanetAt(col *Colony, slot, pl int) error {
	if err := e.planetGun(col, slot); err != nil {
		return err
	}
	ps := e.Cluster.Stars[col.Star].System.Planets
	switch {
	case pl < 0 || pl >= len(ps):
		return errors.New("no such planet")
	case pl == col.PlanetIdx:
		return fmt.Errorf("%s cannot fire at itself", col.Planet.Name)
	}
	g := &Game{Cluster: e.Cluster, Empires: e.peerList()}
	if err := g.CanHitPlanet(e, col.Star, pl); err != nil {
		return err
	}
	it := col.Planet.Orbit[slot]
	if d, _ := it.Defence(); dist(col.Planet.Pos, ps[pl].Pos) > float64(d.Range)*RangeUnit+1e-9 {
		return fmt.Errorf("%s: out of range", it)
	}
	return nil
}

// ArmPlanetAt arms the orbital weapon in slot of col at planet pl's
// cascade for this round (the original's planets
// fire at planets), as CanArmPlanetAt allows (Round checks the range
// again).
func (e *Empire) ArmPlanetAt(col *Colony, slot, pl int) error {
	if err := e.CanArmPlanetAt(col, slot, pl); err != nil {
		return err
	}
	col.Armed = append(col.Armed, Shot{Part: slot, AtPlanet: true, Planet: pl})
	if e.journal != nil {
		p := e.Planet(col.Star, pl)
		en := e.entry("arm")
		en.Colony, en.Slot, en.Star, en.Planet = col.Planet.Name, slot, col.Star, pl
		en.Target, en.Other = p.Name, p.Owner
		e.journal.Record(en)
	}
	return nil
}

// DisarmPlanet drops every shot col armed this round.
func (e *Empire) DisarmPlanet(col *Colony) {
	if len(col.Armed) > 0 && e.journal != nil {
		e.journal.Record(col.entry("disarm"))
	}
	col.Armed = nil
}

// raze destroys the structure in slot of planet pl of star, shot down by
// se: the Razed Reports, se's then owner's (none for −1); se's names
// neither the structure nor its Item when se did not SeesOrbit p before.
func (g *Game) raze(se *Empire, owner, star, pl, slot int) []Report {
	p := g.Cluster.Stars[star].System.Planets[pl]
	it := p.Orbit[slot]
	veiled := !se.SeesOrbit(p)
	p.DemolishOrbital(slot)
	ev := Event{Kind: Razed, Item: it, Tech: NoTech, Star: star, Planet: pl, Other: -1}
	mine, theirs := ev, ev
	mine.Text = fmt.Sprintf("%s razed on %s", it, p.Name)
	if veiled {
		mine.Item, mine.Text = NoItem, fmt.Sprintf("something razed in %s's orbit", p.Name)
	}
	theirs.Text = fmt.Sprintf("%s lost on %s", it, p.Name)
	if owner < 0 {
		return []Report{{se.Index, mine}}
	}
	return []Report{{se.Index, mine}, {owner, theirs}}
}
