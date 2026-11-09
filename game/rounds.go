package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Combat rounds (docs/rules.md "Combat rounds"). A ship's ENERGY
// is the day's budget (Starship.Power): a move pays MoveCost, a shot its
// weapon's Power; shields that are on hold their ShieldDraw off it, a
// reservation taken at the switch and at Refill, never per round. Its
// POWER is what runs at once, Design.Generators per round: a move or an
// armed shot takes one, shields none. Where hostile ships meet out of
// the lanes (CombatStars) the day is played in rounds: each side arms
// blind (Empire.Arm), then Round fires every armed shot on the round's
// starting positions, the damage landing together, and then moves every
// surviving ordered ship one Hop, one after another (shots
// before moves).

// Shot is one armed shot: the weapon part (a colony's: its orbit slot)
// and its target, a ship as the shooter's empire sees it, or with
// AtPlanet the orbit cascade of planet Planet (orbit.go).
type Shot struct {
	Part     int
	Target   Sighting // the ship shot at, unless AtPlanet
	AtPlanet bool
	Planet   int // AtPlanet's planet index in the star's System.Planets
}

// armedEnergy is the energy s's armed shots will pay.
func (s *Starship) armedEnergy() int {
	n := 0
	for _, a := range s.Armed {
		n += Gizmos[s.Design.Parts[a.Part]].Power
	}
	return n
}

// ArmedPart reports s's part is armed this round already.
func (s *Starship) ArmedPart(part int) bool {
	return slices.ContainsFunc(s.Armed, func(a Shot) bool { return a.Part == part })
}

// Arm arms s's weapon part at t for this round, nil when accepted, as
// armShooter and fireTarget allow. Range is NOT checked (Round does, on
// the round's starting positions); nothing is spent before Round.
func (e *Empire) Arm(s *Starship, part int, t Sighting) error {
	if _, err := e.armShooter(s, part); err != nil {
		return err
	}
	if err := e.fireTarget(s, t); err != nil {
		return err
	}
	s.Armed = append(s.Armed, Shot{Part: part, Target: t})
	if e.journal != nil {
		en := e.shipEntry("arm", s)
		en.Part, en.Star, en.Other = part, s.Star, t.Empire
		en.Target = t.ship.Name // fireTarget refused a nil one
		e.journal.Record(en)
	}
	return nil
}

// fireTarget is the refusals on the target t of s (Arm, targetCheck):
// gone (or a stale Sighting), not hostile, in a lane, in another system,
// in orbit (hit through its planet only).
func (e *Empire) fireTarget(s *Starship, t Sighting) error {
	tg := t.ship
	es := e.peerList()
	switch {
	case tg == nil || tg.Hull <= 0 || t.Empire < 0 || t.Empire >= len(es) || !slices.Contains(es[t.Empire].Ships, tg):
		return errors.New("no such target")
	case t.Empire == e.Index || e.StanceWith(t.Empire) != War: // Game.Hostile, without the Game
		return errors.New("not hostile")
	case tg.Lane >= 0:
		return errors.New("the target is in a star lane")
	case tg.Star != s.Star:
		return errors.New("not in the same system")
	case tg.Orbit >= 0:
		return fmt.Errorf("the target is in orbit of %s: fire at the planet", e.Planet(tg.Star, tg.Orbit).Name) // unnamed: a foreign ship never is
	}
	return nil
}

// armShooter is the refusals on the shooter (not e's, no such part, not
// a weapon, Scrambled, armed already, no shot left, no generator free,
// short of energy for every armed shot, outToFire), and the weapon's row.
func (e *Empire) armShooter(s *Starship, part int) (*GizmoInfo, error) {
	if !slices.Contains(e.Ships, s) {
		return nil, errNotOurs
	}
	if part < 0 || part >= len(s.Design.Parts) {
		return nil, fmt.Errorf("no part %d", part)
	}
	w := &Gizmos[s.Design.Parts[part]]
	var err error
	switch {
	case w.Cat != Weapon:
		err = fmt.Errorf("%s is not a weapon", w.Name)
	case s.Scrambled:
		err = fmt.Errorf("%s: crew scrambled", s.Name)
	case slices.ContainsFunc(s.Armed, func(a Shot) bool { return a.Part == part }):
		err = fmt.Errorf("%s: already armed this round", w.Name)
	case w.Uses > 0 && part < len(s.Shots) && s.Shots[part] >= w.Uses:
		err = fmt.Errorf("%s: no shot left today", w.Name)
	case len(s.Armed) >= s.Design.Generators():
		err = fmt.Errorf("%s: no generator free (%d armed of %d)", w.Name, len(s.Armed), s.Design.Generators())
	case s.Power < s.armedEnergy()+w.Power:
		err = fmt.Errorf("%s: not enough energy (%d left, %d armed, %d more)", w.Name, s.Power, s.armedEnergy(), w.Power)
	default:
		err = e.outToFire(s)
	}
	return w, err
}

// outToFire refuses a shooter in a lane or in orbit (the original's: a
// ship leaves orbit to fight).
func (e *Empire) outToFire(s *Starship) error {
	switch {
	case s.Lane >= 0:
		return errors.New("in a star lane")
	case s.Orbit >= 0:
		return fmt.Errorf("%s is in orbit of %s: leave orbit to fire", s.Name, e.Planet(s.Star, s.Orbit).Name)
	}
	return nil
}

// Disarm drops every shot s armed this round.
func (e *Empire) Disarm(s *Starship) {
	if len(s.Armed) > 0 && e.journal != nil {
		e.journal.Record(e.shipEntry("disarm", s))
	}
	s.Armed = nil
}

// SightingOf is t as e sees it (the ShipsSeen entry holding t), false
// when e does not see it (unknown star, a lane, e's own, gone).
func (g *Game) SightingOf(e *Empire, t *Starship) (Sighting, bool) {
	for _, sg := range g.ShipsSeen(e, t.Star) {
		if sg.ship == t {
			return sg, true
		}
	}
	return Sighting{}, false
}

// CombatStars is the stars, ascending, where Round is played: ships of
// two mutually Hostile empires out of the lanes, or a colony with an
// orbital defence that facesFoe, or whose gun reaches a planet it may
// fire at (planetFoe: the original's bots fire at
// every valid target).
func (g *Game) CombatStars() []int {
	at := make([][]int, len(g.Cluster.Stars)) // empire indices present, ascending
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if s.Lane >= 0 {
				continue
			}
			if l := at[s.Star]; len(l) == 0 || l[len(l)-1] != e.Index {
				at[s.Star] = append(l, e.Index)
			}
		}
	}
	fight := make([]bool, len(g.Cluster.Stars))
	for st, l := range at {
	pairs:
		for i, a := range l {
			for _, b := range l[i+1:] {
				if g.Hostile(g.Empires[a], g.Empires[b]) && g.Hostile(g.Empires[b], g.Empires[a]) {
					fight[st] = true
					break pairs
				}
			}
		}
	}
	for _, f := range g.Empires {
		for _, c := range f.Colonies {
			if c.Star < 0 || c.Star >= len(fight) || fight[c.Star] {
				continue
			}
			def, gun := false, false
			for _, it := range c.Planet.Orbit {
				if d, ok := it.Defence(); ok {
					def, gun = true, gun || d.Weapon()
				}
			}
			if def && g.facesFoe(f, c.Star, gun) || gun && g.planetFoe(f, c) {
				fight[c.Star] = true
			}
		}
	}
	var out []int
	for st, ok := range fight {
		if ok {
			out = append(out, st)
		}
	}
	return out
}

// facesFoe reports a ship out of the lanes at star of an empire mutually
// Hostile to f that is armed, or, with gun, in space (f can shoot it).
func (g *Game) facesFoe(f *Empire, star int, gun bool) bool {
	for _, e := range g.Empires {
		if !g.Hostile(e, f) || !g.Hostile(f, e) {
			continue
		}
		for _, s := range e.Ships {
			if s.Star == star && s.Lane < 0 && (gun && s.Orbit < 0 || s.Design.Weapons() > 0) {
				return true
			}
		}
	}
	return false
}

// planetFoe reports one of c's orbital weapons reaching another planet of
// its star that f may fire at (gunHits).
func (g *Game) planetFoe(f *Empire, c *Colony) bool {
	for pl := range g.Cluster.Stars[c.Star].System.Planets {
		if g.gunHits(f, c, pl) {
			return true
		}
	}
	return false
}

// gunHits reports one of c's orbital weapons reaching planet pl of its
// star, not c's own, that f may fire at (CanHitPlanet, the weapon's Range
// from c's planet): ArmPlanetAt's grounds but the round's state (shots,
// arming).
func (g *Game) gunHits(f *Empire, c *Colony, pl int) bool {
	ps := g.Cluster.Stars[c.Star].System.Planets
	if pl == c.PlanetIdx || pl < 0 || pl >= len(ps) {
		return false
	}
	for _, it := range c.Planet.Orbit {
		if d, ok := it.Defence(); ok && d.Weapon() && dist(c.Planet.Pos, ps[pl].Pos) <= float64(d.Range)*RangeUnit+1e-9 {
			return g.CanHitPlanet(f, c.Star, pl) == nil
		}
	}
	return false
}

// Move is one ship's move in a round: its Pos before and after its Hop.
type Move struct {
	Ship     *Starship
	Empire   int
	From, To Vec3
	Orbit    int // the planet whose orbit the move entered, −1 none
}

// Fired is one shot fired in a round. The shooter is a ship (Shooter, its
// weapon Gizmo) or, FromPlanet, an orbital weapon (Shooter nil). The
// target is a ship, or AtPlanet the cascade of TargetPlanet: the ship it
// reached, or nil when the structure HitItem in HitSlot took the full
// Damage (views show a foreign one's left only via DefenceLeft).
type Fired struct {
	Shooter, Target             *Starship
	ShooterEmpire, TargetEmpire int
	Gizmo                       Gizmo // the weapon, a ship's (0, meaningless, FromPlanet)
	From, To                    Vec3  // at the round's start, before its moves (a planet's Pos)
	Damage                      int   // hull points the shot took (shotDamage), or a structure's damage
	Absorbed                    int   // what the target's shields took off: Level − Damage
	// ShieldsDown: the shot knocked the target's shields down (takeShot:
	// Power short of the draw), no hull damage.
	ShieldsDown bool

	FromPlanet                 bool
	ShooterPlanet, ShooterSlot int
	ShooterItem                Item

	AtPlanet     bool
	TargetPlanet int
	HitSlot      int  // AtPlanet, Target nil: the slot struck
	HitItem      Item // and its structure
}

// Razing is an orbital structure destroyed in a round: planet, slot and
// what stood there.
type Razing struct {
	Planet, Slot int
	Item         Item
}

// RoundLog is what one Round did at Star, for a view's replay, each list
// in Round's order.
type RoundLog struct {
	Star      int
	Moves     []Move
	Shots     []Fired
	Destroyed []*Starship
	Razed     []Razing
}

// Round plays one round at star (passed[i]: empire i neither moves nor
// shoots) over the ships out of the lanes there, empire then Ships order
// (shields draw nothing here: their reservation, above), shots before
// moves:
//  1. Shots on the round's starting positions: a shot is dropped at no
//     cost when its shooter is in an orbit or Scrambled, its target left
//     the round or space or is out of WeaponRange, or no shot or energy
//     is left (part order); else paid, taking a generator. Every shot
//     paid, they land in that order, those AtPlanet first (one finding
//     its cascade fallen given back before any draw reads its energy,
//     and its generator), a ship hit through
//     takeShot (raised shields pay their draw per hit, or go dark short
//     of it). A shot AtPlanet strikes the cascade's first layer still
//     standing under the pending damage (orbitTarget): the full Level on
//     a structure, nothing when its owner IsInvincible. Then the
//     colonies' orbital weapons, at a ship or at a planet's cascade (as
//     a ship's; every layer fallen: dropped, no shot spent). Then the
//     damage lands together: ships at 0 hull destroyed (the first shooter
//     credited), structures at Strength razed, planet then slot order.
//  2. Moves, the survivors only: each ship Hops once if a generator and
//     MoveCost are left after the shots it paid (and its shields' draws
//     per hit), against the others where they stand by its turn
//     (sequential: no two end within ShipGap; the destroyed
//     block nothing), guards and lane blocks as at the close.
//  3. Every Armed at star is cleared, the colonies' Tractors too.
//
// acted reports a move or a shot. RoundsToday[star] counts it.
func (g *Game) Round(star int, passed []bool) (acted bool, reps []Report, log RoundLog) {
	log.Star = star
	if g.RoundsToday == nil {
		g.RoundsToday = make([]int, len(g.Cluster.Stars))
	}
	g.RoundsToday[star]++
	plays := func(e *Empire) bool { return e.Index >= len(passed) || !passed[e.Index] }
	type seat struct {
		e *Empire
		s *Starship
	}
	var here []seat
	for _, e := range g.Empires {
		for _, s := range e.Ships {
			if s.Star == star && s.Lane < 0 {
				here = append(here, seat{e, s})
			}
		}
	}
	pending := make([]int, len(here))
	type credit struct {
		e    *Empire
		name string
		ship bool // name is a ship's (hidden from the other side), else a planet's
	}
	by := make([]credit, len(here)) // the credited shooter, e nil none
	// land adds a shot of level on seat j to its pending damage, the first
	// shooter credited, and books it in the target's Dealt for the
	// shooter's empire (Hull follows below, before anyone reads either);
	// an Invulnerable target takes 0, no credit, its shields untouched;
	// else takeShot, which charges the shields' draw; down: this shot
	// knocked them down (dark before it: false).
	land := func(j, level int, who credit) (dm int, down bool) {
		t := here[j].s
		if t.Invulnerable {
			return 0, false
		}
		was := t.ShieldsDown
		d := t.takeShot(level)
		pending[j] += d
		t.dealt(who.e.Index, d)
		if by[j].e == nil {
			by[j] = who
		}
		return d, !was && t.ShieldsDown
	}
	seatOf := func(t *Starship) int { return slices.IndexFunc(here, func(x seat) bool { return x.s == t }) }
	shipFell := func(t *Starship) bool { j := seatOf(t); return j < 0 || t.Hull-pending[j] <= 0 }
	sys := g.Cluster.Stars[star].System
	var slotPend []int // per planet × OrbitSlots: the round's damage on each structure
	var slotBy []credit
	fell := func(pl int) func(int) bool {
		p := sys.Planets[pl]
		return func(k int) bool {
			d, _ := p.Orbit[k].Defence()
			return slotPend != nil && p.OrbitDamage[k]+slotPend[pl*OrbitSlots+k] >= d.Strength
		}
	}
	// strike lands a shot of level by who (of e) on the first layer of
	// planet pl's cascade still standing under the pending damage, into f
	// (AtPlanet, the layer struck): the ship in orbit through land, else
	// the full level on the structure (nothing when its owner
	// IsInvincible); false when every layer fell this round.
	strike := func(e *Empire, pl, level int, who credit, f *Fired) bool {
		p := sys.Planets[pl]
		hit := g.orbitTarget(e, star, pl, fell(pl), shipFell)
		if !hit.ok {
			return false
		}
		f.To, f.AtPlanet, f.TargetPlanet, f.HitSlot, f.HitItem = p.Pos, true, pl, -1, NoItem
		if t := hit.Ship; t != nil {
			j := seatOf(t)
			d, down := land(j, level, who)
			f.Target, f.TargetEmpire, f.To, f.Damage, f.Absorbed, f.ShieldsDown = t, here[j].e.Index, t.Pos, d, level-d, down
			return true
		}
		if slotPend == nil {
			slotPend = make([]int, len(sys.Planets)*OrbitSlots)
			slotBy = make([]credit, len(slotPend))
		}
		x := pl*OrbitSlots + hit.Slot
		f.TargetEmpire, f.HitSlot, f.HitItem, f.Damage = p.Owner, hit.Slot, p.Orbit[hit.Slot], level
		if o := g.empireAt(p.Owner); o != nil && o.IsInvincible(g.Day) {
			f.Damage, f.Absorbed = 0, level // SA_ShieldPlanets: the shot spent, nothing lands
			return true
		}
		slotPend[x] += level
		if slotBy[x].e == nil {
			slotBy[x] = who
		}
		return true
	}
	// Every shot is paid first, then they land in the same order: a hit's
	// shield draw (takeShot) comes off the target's energy after its own
	// shots of the round are paid, whatever the seat order.
	type paid struct {
		h seat
		a Shot
		w *GizmoInfo
		j int // the target's seat, a ship shot
		i int // the shooter's seat
	}
	var shots []paid
	gens := make([]int, len(here)) // per seat, the generators its paid shots took
	for i, h := range here {
		s := h.s
		if !plays(h.e) || s.Orbit >= 0 || s.Scrambled {
			continue
		}
		for _, a := range s.Armed {
			w, j := &Gizmos[s.Design.Parts[a.Part]], -1
			var to Vec3
			if a.AtPlanet {
				to = sys.Planets[a.Planet].Pos
			} else {
				j = seatOf(a.Target.ship)
				if j < 0 || here[j].s.Lane >= 0 || here[j].s.Hull <= 0 || here[j].s.Orbit >= 0 {
					continue
				}
				to = here[j].s.Pos
			}
			if dist(s.Pos, to) > s.WeaponRange(w)+1e-9 ||
				w.Uses > 0 && s.Shots[a.Part] >= w.Uses || s.Power < w.Power {
				continue
			}
			s.Shots[a.Part]++
			s.Power -= w.Power
			gens[i]++
			shots = append(shots, paid{h, a, w, j, i})
		}
	}
	// The shots at planets land first: a cascade's fall shows only then
	// (pending), and a shot given back must be back before a hit's draw
	// reads its shooter's energy. The ships in orbit they strike take no
	// other hit, so credits and pending are the same in either order; the
	// log keeps the paid order.
	fired := make([]Fired, len(shots))
	landed := make([]bool, len(shots))
	for i, sh := range shots {
		h, a, w, s := sh.h, sh.a, sh.w, sh.h.s
		if !a.AtPlanet {
			continue
		}
		f := Fired{Shooter: s, ShooterEmpire: h.e.Index, Gizmo: s.Design.Parts[a.Part], From: s.Pos}
		if !strike(h.e, a.Planet, w.Level, credit{h.e, s.Name, true}, &f) { // every layer fell this round: the shot given back
			s.Shots[a.Part]--
			s.Power += w.Power
			gens[sh.i]--
			continue
		}
		fired[i], landed[i] = f, true
	}
	for i, sh := range shots {
		h, a, w, s := sh.h, sh.a, sh.w, sh.h.s
		if a.AtPlanet {
			continue
		}
		j := sh.j
		t := here[j].s
		d, down := land(j, w.Level, credit{h.e, s.Name, true})
		fired[i] = Fired{Shooter: s, Target: t, ShooterEmpire: h.e.Index, TargetEmpire: here[j].e.Index,
			Gizmo: s.Design.Parts[a.Part], From: s.Pos, To: t.Pos, Damage: d, Absorbed: w.Level - d, ShieldsDown: down}
		landed[i] = true
	}
	for i, f := range fired {
		if landed[i] {
			log.Shots = append(log.Shots, f)
			acted = true
		}
	}
	type gunSeat struct {
		e *Empire
		c *Colony
	}
	var guns []gunSeat
	for _, e := range g.Empires {
		for _, c := range e.Colonies {
			if c.Star == star {
				guns = append(guns, gunSeat{e, c})
			}
		}
	}
	for _, gn := range guns {
		if !plays(gn.e) {
			continue
		}
		p := gn.c.Planet
		for _, a := range gn.c.Armed {
			d, ok := p.Orbit[a.Part].Defence()
			if !ok || !d.Weapon() || gn.c.OrbitShots[a.Part] >= d.Shots {
				continue
			}
			reach := float64(d.Range)*RangeUnit + 1e-9
			who := credit{gn.e, gn.c.Planet.Name, false}
			f := Fired{ShooterEmpire: gn.e.Index, From: p.Pos,
				FromPlanet: true, ShooterPlanet: gn.c.PlanetIdx, ShooterSlot: a.Part, ShooterItem: p.Orbit[a.Part]}
			if a.AtPlanet { // a planet's cascade, as a ship's shot at it; all fallen: dropped, no shot spent
				if dist(p.Pos, sys.Planets[a.Planet].Pos) > reach || !strike(gn.e, a.Planet, d.Damage, who, &f) {
					continue
				}
			} else {
				j := seatOf(a.Target.ship)
				if j < 0 {
					continue
				}
				t := here[j].s
				if t.Lane >= 0 || t.Hull <= 0 || t.Orbit >= 0 || dist(p.Pos, t.Pos) > reach {
					continue
				}
				dm, down := land(j, d.Damage, who)
				f.Target, f.TargetEmpire, f.To, f.Damage, f.Absorbed, f.ShieldsDown = t, here[j].e.Index, t.Pos, dm, d.Damage-dm, down
			}
			gn.c.OrbitShots[a.Part]++
			log.Shots = append(log.Shots, f)
			acted = true
		}
	}
	for j, h := range here {
		h.s.Hull -= pending[j]
	}
	for j, h := range here {
		if by[j].e != nil && h.s.Hull <= 0 {
			reps = append(reps, g.destroyedBy(by[j].e, by[j].name, by[j].ship, h.e, h.s)...)
			log.Destroyed = append(log.Destroyed, h.s)
		}
	}
	for x, dm := range slotPend {
		if dm == 0 {
			continue
		}
		pl, k := x/OrbitSlots, x%OrbitSlots
		p := sys.Planets[pl]
		p.OrbitDamage[k] += dm
		if d, _ := p.Orbit[k].Defence(); p.OrbitDamage[k] >= d.Strength {
			log.Razed = append(log.Razed, Razing{Planet: pl, Slot: k, Item: p.Orbit[k]})
			reps = append(reps, g.raze(slotBy[x].e, p.Owner, star, pl, k)...)
		}
	}
	// The survivors move: the shots above paid their energy and took a
	// generator each, a destroyed ship (Hull <= 0, out of its Ships) neither
	// moves nor stands in the way.
	var obs []Vec3
	for i, h := range here {
		s := h.s
		if !plays(h.e) || s.Hull <= 0 || s.Design.Generators()-gens[i] < 1 || s.Power < s.Design.MoveCost() {
			continue
		}
		obs = obs[:0] // read live: the earlier seats have moved already
		for k, o := range here {
			if k != i && o.s.Hull > 0 && o.s.InSpace() {
				obs = append(obs, o.s.Pos)
			}
		}
		from := s.Pos
		nav, hop, routed, _ := h.e.steer(s) // no route: the close drops it, with its news
		if !routed && nav.Kind == GoToSystem {
			s.Order = nav
		}
		o, held := h.e.holdAtBlock(s, h.e.Day, func(int) []Vec3 { return obs })
		moved, _ := s.Hop(g.Cluster, h.e.Day, obs, h.e.barredIn(g.Empires))
		if held {
			s.Order = o
		}
		if routed {
			unsteer(s, nav, hop, s.Star)
		}
		if moved {
			acted = true
			log.Moves = append(log.Moves, Move{Ship: s, Empire: h.e.Index, From: from, To: s.Pos, Orbit: s.Orbit})
		}
	}
	for _, gn := range guns {
		gn.c.Armed, gn.c.Tractors = nil, 0
	}
	for _, h := range here {
		h.s.Armed = nil
	}
	return acted, reps, log
}

// BeginClose marks the close's start, before the bots act (bot.Begin):
// a fight a bot opens then (a war declared at the close) is news to
// EndDay's opened.
func (g *Game) BeginClose() { g.closeFrom = g.engagements() }

// engagements is every (empire, star) Engaged now, over CombatStars.
func (g *Game) engagements() map[[2]int]bool {
	in := map[[2]int]bool{}
	for _, st := range g.CombatStars() {
		for _, e := range g.Empires {
			if g.Engaged(e, st) {
				in[[2]int{e.Index, st}] = true
			}
		}
	}
	return in
}

// opened is an Engaged Report per (empire, star) engaged now but not in
// before: the hostile empires seen there named, ascending, Other the
// first. A fight going on is no news.
func (g *Game) opened(before map[[2]int]bool) []Report {
	var reps []Report
	for _, st := range g.CombatStars() {
		for _, e := range g.Empires {
			if before[[2]int{e.Index, st}] || !g.Engaged(e, st) {
				continue
			}
			var foes []int
			for _, sg := range g.Hostiles(e, st) {
				if !slices.Contains(foes, sg.Empire) {
					foes = append(foes, sg.Empire)
				}
			}
			what := "ships"
			if len(foes) == 0 { // only defended colonies
				foes, what = g.defendedFoes(e, st), "planets"
			}
			slices.Sort(foes)
			names := make([]string, len(foes))
			for i, o := range foes {
				names[i] = g.Empires[o].Species.String()
			}
			reps = append(reps, Report{e.Index, Event{Kind: Engaged, Item: NoItem, Tech: NoTech, Star: st, Planet: -1,
				Other: foes[0], Text: fmt.Sprintf("Combat at %s: %s %s", g.Cluster.Stars[st].Name, strings.Join(names, ", "), what)}})
		}
	}
	return reps
}

// RoundsAt is the rounds played today at star (RoundsToday), 0 when none
// or out of range.
func (g *Game) RoundsAt(star int) int {
	if star < 0 || star >= len(g.RoundsToday) {
		return 0
	}
	return g.RoundsToday[star]
}

// Engaged reports e in the fight at star: e knows it, it is a combat
// star (CombatStars), e sees a hostile ship or defended colony there
// (defendedFoes; not a fight between others), and e has a colony there
// or a ship out of the lanes.
func (g *Game) Engaged(e *Empire, star int) bool {
	if !e.Knows(star) || !slices.Contains(g.CombatStars(), star) {
		return false
	}
	if len(g.Hostiles(e, star)) == 0 && len(g.defendedFoes(e, star)) == 0 {
		return false
	}
	return slices.ContainsFunc(e.Colonies, func(c *Colony) bool { return c.Star == star }) ||
		slices.ContainsFunc(e.Ships, func(s *Starship) bool { return s.Star == star && s.Lane < 0 })
}

// defendedFoes is the empires, ascending, mutually Hostile with e whose
// colony at star has an orbital defence e sees (SeesOrbit), or one of e's
// guns there may hit (gunHits, blind at a cloaked
// orbit): a shielded planet with no ship around is still a fight.
func (g *Game) defendedFoes(e *Empire, star int) []int {
	var out []int
	for _, o := range g.Empires {
		if o == e || !g.Hostile(e, o) || !g.Hostile(o, e) {
			continue
		}
		if slices.ContainsFunc(o.Colonies, func(c *Colony) bool {
			return c.Star == star && (e.SeesOrbit(c.Planet) && slices.ContainsFunc(c.Planet.Orbit[:], func(it Item) bool {
				_, ok := it.Defence()
				return ok
			}) || slices.ContainsFunc(e.Colonies, func(m *Colony) bool { return m.Star == star && g.gunHits(e, m, c.PlanetIdx) }))
		}) {
			out = append(out, o.Index)
		}
	}
	return out
}

// UsableEnergy reports s's energy left enough for a hop or a shot: its
// MoveCost when it has a drive, or the Power of one of its weapons with
// a shot left today (Uses), unless Scrambled.
func (s *Starship) UsableEnergy() bool {
	if s.Design.Reach() > 0 && s.Power >= s.Design.MoveCost() {
		return true
	}
	if s.Scrambled {
		return false
	}
	for p, g := range s.Design.Parts {
		w := &Gizmos[g]
		if w.Cat == Weapon && s.Power >= w.Power && (w.Uses == 0 || p >= len(s.Shots) || s.Shots[p] < w.Uses) {
			return true
		}
	}
	return false
}

// StepRound is e's mid-day round at star (docs/rules.md "Combat rounds",
// mid-day): refused unless e is Engaged there; else Round(star, passed),
// recorded ("round", the star). The caps (bot.RoundCap) are the caller's.
func (g *Game) StepRound(e *Empire, star int, passed []bool) (acted bool, reps []Report, log RoundLog, err error) {
	if !g.Engaged(e, star) {
		name := fmt.Sprint(star)
		if star >= 0 && star < len(g.Cluster.Stars) {
			name = g.Cluster.Stars[star].Name
		}
		return false, nil, RoundLog{}, fmt.Errorf("not engaged at %s", name)
	}
	acted, reps, log = g.Round(star, passed)
	if e.journal != nil {
		en := e.entry("round")
		en.Star = star
		e.journal.Record(en)
	}
	return acted, reps, log, nil
}
