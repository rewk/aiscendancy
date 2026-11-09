package game

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rewk/aiscendancy/journal"
)

// Ship specials (docs/rules.md "Ship specials"). A special is used at once
// (the original's), not armed for the round: Empire.UseSpecial checks,
// pays and applies it in one call. Its uses today are its Starship.Shots
// entry, its energy Gizmos[g].Power; the numbers are gizmos.txt's
// (Special1, Special2), each effect quotes gizhelp.txt. The passives act
// inside the rules they bend (LaneDrives, payMove, ShipsSeen). Here the
// self-targeted actives; targeted.go the ship- and planet-targeted and
// area ones; positional.go those that move ships or aim at a lane.

// TargetKind is what a special is aimed at: nothing (itself), a ship, a
// star lane, a planet.
type TargetKind uint8

const (
	NoTarget TargetKind = iota
	ShipTarget
	LaneTarget
	PlanetTarget
)

var targetKindNames = [...]string{"no", "ship", "lane", "planet"}

func (k TargetKind) String() string {
	if int(k) >= len(targetKindNames) {
		return "unknown"
	}
	return targetKindNames[k]
}

// Target is a special's aim: for a ShipTarget exactly one of Ship (a
// Sighting of another empire's ship) and Mine (one of the user's own);
// Index in the star's System.Lanes or System.Planets.
type Target struct {
	Kind  TargetKind
	Ship  Sighting
	Mine  *Starship
	Index int
}

// aim is what an effect is aimed at, for targetCheck: itself, a hostile
// ship, an allied or own ship, either, a hostile planet, a lane.
type aim uint8

const (
	aimSelf aim = iota
	aimHostile
	aimAlly
	aimAny
	aimPlanet
	aimLane
)

// specialEffect is one active special: its aim, its own refusal (checked
// before anything is paid; nil none), apply (after paying: the user's
// report line, its outcome for SpecialUse and the victims' Reports),
// once (lost after apply).
type specialEffect struct {
	aim    aim
	refuse func(e *Empire, s *Starship, part int, t Target) error
	apply  func(e *Empire, s *Starship, part int, t Target) (text, outcome string, more []Report)
	once   bool
}

// SpecialUse is a use of a special as data, for a view (Event.Use, never
// saved): the user, the gadget, the star, the target (a ship and its
// owner, −1 none; Index the planet or lane), and Outcome, a short result
// naming no ship nor planet ("tied down", "-3", "Ueberlaser destroyed");
// what it did to the ships out of the lanes at the star, for a replay:
// those it destroyed and those it moved (From where they stood, To where
// they are, at the star still: a lane's endpoint, a Recaller's none),
// in Empires then Ships order.
type SpecialUse struct {
	User         *Starship
	UserEmpire   int
	Gizmo        Gizmo
	Star         int
	Kind         TargetKind
	Target       *Starship
	TargetEmpire int
	Index        int
	Outcome      string
	Destroyed    []*Starship
	Moves        []Move
}

// Consumable reports a part its use spends: a Colonizer (its colonists
// land), an Invasion Module (its troops), a once special (LosePart).
func (g Gizmo) Consumable() bool {
	return g == Colonizer || g == InvasionModule || specialEffects[g].once
}

// Consumables is Counts less the parts not Consumable: on a ship's
// Aboard, what it has left to spend.
func (d Design) Consumables() []PartCount {
	return slices.DeleteFunc(d.Counts(), func(pc PartCount) bool { return !pc.Gizmo.Consumable() })
}

// specialEffects is the modelled actives, by gizmo; every other
// non-passive special is "not modelled yet".
var specialEffects = map[Gizmo]specialEffect{
	Hyperfuel:            {apply: useHyperfuel, once: true},
	ToroidalBlaster:      {refuse: refuseToroidal, apply: useToroidal},
	Cannibalizer:         {refuse: refuseCannibalizer, apply: useCannibalizer},
	Replenisher:          {refuse: refuseReplenisher, apply: useReplenisher},
	RemoteRepairFacility: {refuse: refuseRepair, apply: useRepair},
	Accutron:             {refuse: refuseAccutron, apply: useAccutron},
	Invulnerablizer:      {refuse: refuseInvulnerablizer, apply: useInvulnerablizer, once: true},
	Recaller:             {refuse: refuseRecaller, apply: useRecaller},

	MolecularTieDown:     {aim: aimHostile, apply: useTieDown},
	IntellectScrambler:   {aim: aimHostile, apply: useScrambler},
	BrunswikDissipator:   {aim: aimHostile, apply: useBrunswik},
	Disarmer:             {aim: aimHostile, refuse: refuseDisarmer, apply: useDisarmer},
	ContainmentDevice:    {aim: aimHostile, refuse: refuseContainment, apply: useContainment},
	ShieldBlaster:        {aim: aimHostile, apply: useShieldBlaster},
	Backfirer:            {aim: aimHostile, refuse: refuseDisarmer, apply: useBackfirer},
	MovingPartExploiter:  {aim: aimHostile, apply: useExploiter},
	Gizmogrifier:         {aim: aimHostile, refuse: refuseGizmogrifier, apply: useGizmogrifier},
	SpecialtyBlaster:     {aim: aimHostile, refuse: refuseSpecialty, apply: useSpecialty},
	Disintegrator:        {aim: aimHostile, apply: useDisintegrator, once: true},
	MyrmidonicCarbonizer: {aim: aimHostile, apply: useCarbonizer},
	SacrificialOrb:       {aim: aimAlly, refuse: refuseOrb, apply: useOrb},
	PlasmaCoupler:        {aim: aimAlly, refuse: refuseCoupler, apply: useCoupler},
	SmartBomb:            {refuse: refuseSmartBomb, apply: useSmartBomb, once: true},
	SelfDestructotron:    {apply: useDestructotron},
	PhaseBomb:            {aim: aimPlanet, refuse: refusePhaseBomb, apply: usePhaseBomb, once: true},

	PositronBouncer:      {aim: aimHostile, apply: useBouncer},
	GizmoTractorBeam:     {aim: aimAny, refuse: refuseTractor, apply: useTractor},
	Hyperswapper:         {aim: aimAny, apply: useSwapper},
	FleetDisperser:       {aim: aimHostile, refuse: refuseDisperser, apply: useDisperser},
	GravityDistorter:     {refuse: refuseDistorter, apply: useDistorter},
	MassCondensor:        {aim: aimHostile, refuse: refuseMass, apply: useMass},
	GravimetricCondensor: {refuse: refuseCondensor, apply: useCondensor},
	GravimetricCatapult:  {refuse: refuseCatapult, apply: useCatapult},
	LaneBlocker:          {aim: aimLane, refuse: refuseLaneBlocker, apply: useLaneBlocker, once: true},
	LaneDestabilizer:     {aim: aimLane, refuse: refuseDestabilizer, apply: useDestabilizer, once: true},
	LaneMagnetron:        {aim: aimLane, refuse: refuseMagnetron, apply: useMagnetron, once: true},
	LaneEndoscope:        {aim: aimLane, refuse: refuseEndoscope, apply: useEndoscope},
}

// targetKinds is the target kinds g's flags take (ship, lane, planet),
// NoTarget alone for none.
func targetKinds(g Gizmo) []TargetKind {
	f := Gizmos[g].Flags
	var ks []TargetKind
	if f&GzTargetShip != 0 {
		ks = append(ks, ShipTarget)
	}
	if f&GzTargetLane != 0 {
		ks = append(ks, LaneTarget)
	}
	if f&GzTargetSurface != 0 {
		ks = append(ks, PlanetTarget)
	}
	if ks == nil {
		ks = []TargetKind{NoTarget}
	}
	return ks
}

// specialCheck is the ship-side refusals of a use of part, in order: no
// such part, not a Special, used by an order (Colonizer, Invasion
// Module), passive, a target kind not taken (nil t: unchecked), not
// modelled, in a lane, in orbit (there a ship only leaves,
// colonises or invades), no use left (Uses 0: no limit), short of energy.
func (s *Starship) specialCheck(part int, t *Target) error {
	if part < 0 || part >= len(s.Design.Parts) {
		return fmt.Errorf("no part %d", part)
	}
	g := s.Design.Parts[part]
	in := &Gizmos[g]
	switch {
	case in.Cat != Special:
		return fmt.Errorf("%s is not a special", in.Name)
	case g == Colonizer || g == InvasionModule:
		return fmt.Errorf("%s is used by an order", in.Name)
	case in.Flags&GzPassive != 0:
		return fmt.Errorf("%s is always on", in.Name)
	}
	if t != nil {
		if ks := targetKinds(g); !slices.Contains(ks, t.Kind) {
			if t.Kind == NoTarget {
				return fmt.Errorf("%s needs a %s target", in.Name, ks[0])
			}
			return fmt.Errorf("%s takes no %s target", in.Name, t.Kind)
		}
	}
	if _, ok := specialEffects[g]; !ok {
		return errors.New("not modelled yet")
	}
	switch {
	case s.Lane >= 0:
		return errors.New("in a star lane")
	case s.Orbit >= 0:
		return errors.New("in orbit: leave it first")
	case in.Uses > 0 && part < len(s.Shots) && s.Shots[part] >= in.Uses:
		return fmt.Errorf("%s: no use left today", in.Name)
	case s.Power < in.Power:
		return fmt.Errorf("%s: not enough energy (%d of %d)", in.Name, s.Power, in.Power)
	}
	return nil
}

// specialReady is specialCheck without a target (Usable).
func (s *Starship) specialReady(part int) error { return s.specialCheck(part, nil) }

// UseSpecial uses special part of s at t, at once: reps[0] the user's
// SpecialUsed, then the victims'. Refused (changing nothing) as command,
// specialCheck, targetCheck and the special's own refusal say; else one
// use and its Power paid, the effect, a one-time device lost. Energy is
// not reserved for armed shots (Round drops the unpaid ones). The other
// empires are e's peerList: no Game needed.
func (e *Empire) UseSpecial(s *Starship, part int, t Target) (reps []Report, err error) {
	if err := e.command(s); err != nil {
		return nil, err
	}
	if err := s.specialCheck(part, &t); err != nil {
		return nil, err
	}
	g := s.Design.Parts[part]
	fx := specialEffects[g]
	if err := e.targetCheck(s, g, fx.aim, t); err != nil {
		return nil, err
	}
	if fx.refuse != nil {
		if err := fx.refuse(e, s, part, t); err != nil {
			return nil, err
		}
	}
	if len(s.Shots) != len(s.Design.Parts) {
		s.Shots = make([]int, len(s.Design.Parts))
	}
	s.Shots[part]++
	s.Power -= Gizmos[g].Power
	star := s.Star
	var en journal.Entry
	if e.journal != nil { // before the effect moves or destroys anything
		en = e.shipEntry("special", s)
		en.Part, en.Item, en.Star, en.Target = part, Gizmos[g].Name, star, e.targetName(s, t)
	}
	tg := t.ship() // before the effect: a ship it destroys is gone after
	before := standing(e.peerList(), star)
	text, outcome, more := fx.apply(e, s, part, t)
	if fx.once {
		s.LosePart(part)
	}
	e.journal.Record(en)
	use := &SpecialUse{User: s, UserEmpire: e.Index, Gizmo: g, Star: star, Kind: t.Kind, TargetEmpire: -1, Index: t.Index, Outcome: outcome}
	use.Destroyed, use.Moves = before.diff(e.peerList(), star)
	foreign := t.Kind == ShipTarget && t.Mine == nil && t.Ship.Empire != e.Index
	switch {
	case t.Kind != ShipTarget:
	case foreign:
		use.Target, use.TargetEmpire = tg, t.Ship.Empire
	default:
		use.Target, use.TargetEmpire = tg, e.Index
	}
	ev := Event{Kind: SpecialUsed, Item: NoItem, Tech: NoTech, Star: star, Planet: -1, Other: -1, Text: text, Use: use}
	if foreign { // the user's line never names a foreign ship: "a <Species> ship"
		ev.Text = hideName(text, s.Name, tg.Name, aShip(e.peerList(), t.Ship.Empire))
	}
	reps = append([]Report{{e.Index, ev}}, more...)
	// A foreign ship aimed at: its owner reads the same, the user named
	// by its species alone (a Disarmer must not take a gun
	// unannounced), unless the effect reported to it already.
	if foreign && !slices.ContainsFunc(more, func(r Report) bool { return r.Empire == t.Ship.Empire }) {
		v := ev
		v.Text = strings.Replace(text, s.Name, "a "+s.Species.String()+" ship", 1)
		reps = append(reps, Report{t.Ship.Empire, v})
	}
	return reps, nil
}

// shipsHere is the ships out of the lanes at a star as they stood: each
// one's empire, Pos (Empires then Ships order).
type shipsHere []Move

// standing is the ships out of the lanes at star, From their Pos.
func standing(es []*Empire, star int) shipsHere {
	var out shipsHere
	for _, o := range es {
		for _, x := range o.Ships {
			if x.Star == star && x.Lane < 0 {
				out = append(out, Move{Ship: x, Empire: o.Index, From: x.Pos, Orbit: -1})
			}
		}
	}
	return out
}

// diff is what happened to h since: the ships gone from their empire
// (destroyed), those still at star whose Pos changed (moved: To their
// Pos, Orbit theirs out of a lane, else −1).
func (h shipsHere) diff(es []*Empire, star int) (gone []*Starship, moved []Move) {
	for _, m := range h {
		x := m.Ship
		switch {
		case !slices.Contains(es[m.Empire].Ships, x):
			gone = append(gone, x)
		case x.Star == star && x.Pos != m.From:
			m.To = x.Pos
			if x.Lane < 0 {
				m.Orbit = x.Orbit
			}
			moved = append(moved, m)
		}
	}
	return gone, moved
}

// aShip is how another empire's reader names a ship of empire o: "a
// <Species> ship", "an" before a vowel.
func aShip(es []*Empire, o int) string {
	if o < 0 || o >= len(es) {
		return "a ship"
	}
	sp := es[o].Species.String()
	if strings.ContainsRune("AEIOU", rune(sp[0])) {
		return "an " + sp + " ship"
	}
	return "a " + sp + " ship"
}

// hideName is text with every name after the user's own (its first
// occurrence: every special's line opens on it) replaced by as: the
// user's empire never reads a foreign ship's name.
func hideName(text, user, name, as string) string {
	i := strings.Index(text, user)
	if i < 0 {
		return strings.ReplaceAll(text, name, as)
	}
	i += len(user)
	return text[:i] + strings.ReplaceAll(text[i:], name, as)
}

// targetCheck is the target's refusals of a use of g aimed a at t (a kind
// the effect does not aim at is ErrNotModelled):
//   - aimHostile: outToFire, a Mine target, then fireTarget's.
//   - aimAlly: allyTarget's (orbit allowed).
//   - aimAny (ships moved): outToFire, then a Mine or allied target as
//     aimAlly, any other as aimHostile; a target in orbit refused.
//   - aimPlanet: a planet of s's star owned by an empire at War with e.
//   - aimLane: a lane endpoint of s's System.Lanes.
//
// Then the range: g's Range × RangeUnit to the target (never Boosted).
func (e *Empire) targetCheck(s *Starship, g Gizmo, a aim, t Target) error {
	want := [...]TargetKind{aimSelf: NoTarget, aimHostile: ShipTarget, aimAlly: ShipTarget, aimAny: ShipTarget,
		aimPlanet: PlanetTarget, aimLane: LaneTarget}[a]
	if t.Kind != want {
		return ErrNotModelled
	}
	var at Vec3
	switch a {
	case aimSelf:
		return nil
	case aimHostile:
		if err := e.outToFire(s); err != nil {
			return err
		}
		if t.Mine != nil {
			if t.Ship.ship != nil {
				return errNoTarget
			}
			return errors.New("not hostile")
		}
		if err := e.fireTarget(s, t.Ship); err != nil {
			return err
		}
		at = t.Ship.ship.Pos
	case aimAlly:
		tg, err := e.allyTarget(s, t)
		if err != nil {
			return err
		}
		at = tg.Pos
	case aimAny:
		if err := e.outToFire(s); err != nil {
			return err
		}
		var tg *Starship
		if t.Mine != nil || t.Ship.ship != nil && t.Ship.Empire != e.Index && e.StanceWith(t.Ship.Empire) == Alliance {
			var err error
			if tg, err = e.allyTarget(s, t); err != nil {
				return err
			}
			if tg.Orbit >= 0 {
				return fmt.Errorf("%s is in orbit", tg.Name)
			}
		} else {
			if err := e.fireTarget(s, t.Ship); err != nil {
				return err
			}
			tg = t.Ship.ship
		}
		at = tg.Pos
	case aimLane:
		ls := e.Cluster.Stars[s.Star].System.Lanes
		if t.Index < 0 || t.Index >= len(ls) {
			return fmt.Errorf("no lane %d", t.Index)
		}
		at = ls[t.Index].Pos
	case aimPlanet:
		ps := e.Cluster.Stars[s.Star].System.Planets
		if t.Index < 0 || t.Index >= len(ps) {
			return fmt.Errorf("no planet %d", t.Index)
		}
		p := ps[t.Index]
		if p.Owner < 0 || p.Owner == e.Index || e.StanceWith(p.Owner) != War {
			return errors.New("not hostile")
		}
		at = p.Pos
	}
	if in := &Gizmos[g]; dist(s.Pos, at) > float64(in.Range)*RangeUnit+1e-9 {
		return fmt.Errorf("%s: out of range", in.Name)
	}
	return nil
}

var errNoTarget = errors.New("no such target")

// allyTarget is the ship t names for an ally-targeted special of s, or
// why not: exactly one of Mine (another of e's Ships) and Ship (alive, of
// an ally), out of the lanes at s's star.
func (e *Empire) allyTarget(s *Starship, t Target) (*Starship, error) {
	es := e.peerList()
	tg := t.ship()
	switch {
	case (t.Mine == nil) == (t.Ship.ship == nil) || tg.Hull <= 0:
		return nil, errNoTarget
	case t.Mine == s:
		return nil, errors.New("not on itself")
	case t.Mine != nil && !slices.Contains(e.Ships, tg):
		return nil, errNoTarget
	case t.Mine == nil && (t.Ship.Empire < 0 || t.Ship.Empire >= len(es) || !slices.Contains(es[t.Ship.Empire].Ships, tg)):
		return nil, errNoTarget
	case t.Mine == nil && (t.Ship.Empire == e.Index || e.StanceWith(t.Ship.Empire) != Alliance):
		return nil, errors.New("not an ally")
	case tg.Lane >= 0:
		return nil, errors.New("the target is in a star lane")
	case tg.Star != s.Star:
		return nil, errors.New("not in the same system")
	}
	return tg, nil
}

// ship is the ship t names: Mine, else the Sighting's (nil: none).
func (t Target) ship() *Starship {
	if t.Mine != nil {
		return t.Mine
	}
	return t.Ship.ship
}

// LosePart removes part k from the ship: a NEW Parts slice (the old may
// be shared), Shots and the armed shots re-indexed, Weapon reset when k or
// after, Cargo, Invaders and Pop capped at what is left aboard (the
// colonists over the berths lost, unreported), a shield
// lost while on refunding its Power (its share of the reservation). A refit
// restores the full design, paying the lost parts again.
func (s *Starship) LosePart(k int) {
	if k < 0 || k >= len(s.Design.Parts) {
		return
	}
	if g := s.Design.Parts[k]; s.Shields && Gizmos[g].Cat == Shield {
		s.Power += Gizmos[g].Power // its share of the reservation back
	}
	s.Design.Parts = slices.Delete(slices.Clone(s.Design.Parts), k, k+1)
	if s.Design.Slots != nil {
		s.Design.Slots = slices.Delete(slices.Clone(s.Design.Slots), k, k+1)
	}
	if k < len(s.Shots) {
		s.Shots = slices.Delete(slices.Clone(s.Shots), k, k+1)
	}
	if s.Weapon >= k {
		s.Weapon = -1
	}
	var armed []Shot
	for _, a := range s.Armed {
		if a.Part != k {
			if a.Part > k {
				a.Part--
			}
			armed = append(armed, a)
		}
	}
	s.Armed = armed
	s.Cargo = min(s.Cargo, s.Design.Colonizers())
	s.Invaders = min(s.Invaders, s.Design.InvasionModules())
	s.Pop = min(s.Pop, s.Design.Berths())
}

// WeaponRange is Range × RangeUnit, doubled while Boosted (the 2005
// guide's "Range w/ Accutron" column): every ship weapon's reach.
func (s *Starship) WeaponRange(w *GizmoInfo) float64 {
	r := float64(w.Range) * RangeUnit
	if s.Boosted {
		r *= 2
	}
	return r
}

// Hyperfuel: "a one-time-use power reserve of great capacity": Power +=
// Special1, the part lost.
func useHyperfuel(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	n := Gizmos[Hyperfuel].Special1
	s.Power += n
	return fmt.Sprintf("%s burns its Hyperfuel: %d energy", s.Name, n), fmt.Sprintf("+%d energy", n), nil
}

// firstDrive is the index of the first Drive part aboard, −1 none.
func firstDrive(s *Starship) int {
	return slices.IndexFunc(s.Design.Parts, func(g Gizmo) bool { return Gizmos[g].Cat == Drive })
}

// refuseToroidal: the blaster burns an engine, none aboard is a refusal.
func refuseToroidal(e *Empire, s *Starship, part int, _ Target) error {
	if firstDrive(s) < 0 {
		return errors.New("no engine aboard")
	}
	return nil
}

// Toroidal Blaster: "a huge boost in engine performance, but ... usually
// damages some of them": Power += Special1, the FIRST Drive lost
// (ASSUMPTION: always one).
func useToroidal(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	n := Gizmos[ToroidalBlaster].Special1
	s.Power += n
	k := firstDrive(s)
	lost := s.Design.Parts[k]
	s.LosePart(k)
	return fmt.Sprintf("%s's Toroidal Blaster: %d energy, %s burnt out", s.Name, n, lost), fmt.Sprintf("+%d energy, %s burnt out", n, lost), nil
}

// refuseCannibalizer: the hull's Special1 points must leave the ship
// alive.
func refuseCannibalizer(e *Empire, s *Starship, part int, _ Target) error {
	if s.Hull <= Gizmos[Cannibalizer].Special1 {
		return errors.New("would scuttle the ship")
	}
	return nil
}

// Cannibalizer: "convert some of the mass of its own hull into an energy
// reserve": Hull −= Special1, Power += Special2 (ASSUMPTION: that way).
func useCannibalizer(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	in := &Gizmos[Cannibalizer]
	s.Hull -= in.Special1
	s.Power += in.Special2
	return fmt.Sprintf("%s cannibalizes %d hull for %d energy", s.Name, in.Special1, in.Special2),
		fmt.Sprintf("-%d hull, +%d energy", in.Special1, in.Special2), nil
}

// refuseReplenisher: nothing to recharge when no weapon fired today.
func refuseReplenisher(e *Empire, s *Starship, part int, _ Target) error {
	for k, g := range s.Design.Parts {
		if Gizmos[g].Cat == Weapon && k < len(s.Shots) && s.Shots[k] > 0 {
			return nil
		}
	}
	return errors.New("nothing to replenish")
}

// Replenisher: "fully recharges all of the weapons aboard a ship": every
// Weapon part's Shots back to 0.
func useReplenisher(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	for k, g := range s.Design.Parts {
		if Gizmos[g].Cat == Weapon {
			s.Shots[k] = 0
		}
	}
	return fmt.Sprintf("%s replenishes its weapons", s.Name), "weapons recharged", nil
}

// refuseRepair: a full hull has nothing to repair.
func refuseRepair(e *Empire, s *Starship, part int, _ Target) error {
	if s.Hull >= s.MaxHull() {
		return errors.New("hull is full")
	}
	return nil
}

// Remote Repair Facility: "repair damage without having to enter Orbital
// Docks": Hull += Special1 up to MaxHull, as often as the energy lasts.
func useRepair(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	before := s.Hull
	s.Hull = min(s.MaxHull(), s.Hull+Gizmos[RemoteRepairFacility].Special1)
	s.healed()
	return fmt.Sprintf("%s repairs %d hull points", s.Name, s.Hull-before), fmt.Sprintf("+%d hull", s.Hull-before), nil
}

// refuseAccutron: once a day is enough, it lasts until the refill.
func refuseAccutron(e *Empire, s *Starship, part int, _ Target) error {
	if s.Boosted {
		return errors.New("already active today")
	}
	return nil
}

// Accutron: "increases the effective range of all the weapons": Boosted.
func useAccutron(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	s.Boosted = true
	return fmt.Sprintf("%s's Accutron: range doubled today", s.Name), "range doubled", nil
}

// refuseInvulnerablizer: a second one would add nothing today.
func refuseInvulnerablizer(e *Empire, s *Starship, part int, _ Target) error {
	if s.Invulnerable {
		return errors.New("already invulnerable today")
	}
	return nil
}

// Invulnerablizer: "will not allow the ship to come to harm ... can be
// used only once": Invulnerable, the part lost.
func useInvulnerablizer(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	s.Invulnerable = true
	return fmt.Sprintf("%s is invulnerable today", s.Name), "invulnerable", nil
}

// HomeStar is the star NewGame put e's home world on (planet 0; pickHomes,
// no PRNG), whoever holds it today.
func (e *Empire) HomeStar() int {
	return e.Cluster.pickHomes(len(e.peerList()))[e.Index]
}

// refuseRecaller: already home.
func refuseRecaller(e *Empire, s *Starship, part int, _ Target) error {
	if s.Star == e.HomeStar() {
		return errors.New("already home")
	}
	return nil
}

// Recaller: "arrives immediately at its home system": at HomeStar's home
// world LeavePoint (its FreeSpot), its order and armed shots dropped.
func useRecaller(e *Empire, s *Starship, part int, _ Target) (string, string, []Report) {
	home := e.HomeStar()
	s.Pos = FreeSpot(LeavePoint(e.Cluster, home, 0), spaceAt(e.peerList(), home, s))
	s.Star, s.Orbit, s.Lane = home, -1, -1
	s.Order, s.Armed = Order{}, nil
	e.Visit(home)
	return fmt.Sprintf("%s is recalled to %s", s.Name, e.StarLabel(home)), "recalled home", nil
}
