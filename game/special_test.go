package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// Ship specials (docs/rules.md "Ship specials"): the use frame, the eight
// self-targeted actives, the three passives.

// specialShip is a fresh game's empire 0 and a ship of d in space on its
// home planet's Pos, its day refilled with the shields off (the full
// supply as energy).
func specialShip(t *testing.T, d Design) (*Game, *Empire, *Starship) {
	t.Helper()
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e := g.Empires[0]
	home := e.Colonies[0]
	s := e.Spawn(d, home.Star, home.PlanetIdx)
	s.Shields = false
	inSpaceAt(s, s.Pos)
	return g, e, s
}

// useOK is UseSpecial expected to succeed.
func useOK(t *testing.T, e *Empire, s *Starship, part int) string {
	t.Helper()
	return useAt(t, e, s, part, Target{})[0].Event.Text
}

// useAt is UseSpecial at tg expected to succeed, its Reports: the user's
// first, Kind SpecialUsed at its star, its text naming the ship.
func useAt(t *testing.T, e *Empire, s *Starship, part int, tg Target) []Report {
	t.Helper()
	name, star, g := s.Name, s.Star, s.Design.Parts[part]
	var names []string // every ship's and planet's here: none in an Outcome
	for _, o := range e.peerList() {
		for _, x := range o.Ships {
			names = append(names, x.Name)
		}
	}
	for _, p := range e.Cluster.Stars[star].System.Planets {
		names = append(names, p.Name)
	}
	victim := tg.ship()
	foreign := tg.Kind == ShipTarget && tg.Mine == nil && tg.Ship.Empire != e.Index
	reps, err := e.UseSpecial(s, part, tg)
	if err != nil {
		t.Fatalf("use %s: %v", Gizmos[s.Design.Parts[part]].Name, err)
	}
	if len(reps) == 0 {
		t.Fatal("no report")
	}
	r := reps[0]
	if r.Empire != e.Index || r.Event.Kind != SpecialUsed || r.Event.Star != star || r.Event.Planet != -1 ||
		r.Event.Other != -1 || r.Event.Item != NoItem || r.Event.Tech != NoTech || !strings.HasPrefix(r.Event.Text, name) {
		t.Errorf("user's report %+v", r)
	}
	u := r.Event.Use
	if u == nil || u.User != s || u.UserEmpire != e.Index || u.Gizmo != g || u.Star != star || u.Kind != tg.Kind || u.Index != tg.Index || u.Outcome == "" {
		t.Fatalf("%s: use %+v", Gizmos[g].Name, u)
	}
	if tg.Kind == ShipTarget && (u.Target != victim || foreign && u.TargetEmpire != tg.Ship.Empire || !foreign && u.TargetEmpire != e.Index) ||
		tg.Kind != ShipTarget && (u.Target != nil || u.TargetEmpire != -1) {
		t.Errorf("%s: use's target %v of %d", Gizmos[g].Name, u.Target, u.TargetEmpire)
	}
	for _, n := range names {
		if strings.Contains(u.Outcome, n) {
			t.Errorf("%s: outcome %q names %q", Gizmos[g].Name, u.Outcome, n)
		}
	}
	if foreign && strings.Contains(r.Event.Text, victim.Name) {
		t.Errorf("%s: the user's line %q names the foreign target", Gizmos[g].Name, r.Event.Text)
	}
	return reps
}

// useRefused is UseSpecial expected to refuse with want (a substring),
// changing nothing.
func useRefused(t *testing.T, e *Empire, s *Starship, part int, tg Target, want string) {
	t.Helper()
	power, hull, shots, parts := s.Power, s.Hull, slices.Clone(s.Shots), slices.Clone(s.Design.Parts)
	_, err := e.UseSpecial(s, part, tg)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("part %d: %v, want %q", part, err, want)
	}
	if s.Power != power || s.Hull != hull || !slices.Equal(s.Shots, shots) || !slices.Equal(s.Design.Parts, parts) {
		t.Errorf("refusal %q changed the ship", want)
	}
}

// The refusal order on one ship: not ours, no part, not a special, used
// by an order, passive, target kinds, lane, uses, energy.
func TestSpecialRefusalOrder(t *testing.T) {
	d := Design{Hull: Medium, Parts: []Gizmo{Nanotwirler, TonklinMotor, XRayMegaglasses, Hyperswapper, Accutron, Colonizer}}
	g, e, s := specialShip(t, d)
	acc, tie := part(d, Accutron), part(d, Hyperswapper)
	if _, err := g.Empires[1].UseSpecial(s, acc, Target{}); err != errNotOurs {
		t.Errorf("another empire's ship: %v", err)
	}
	useRefused(t, e, s, len(d.Parts), Target{}, "no part")
	useRefused(t, e, s, part(d, TonklinMotor), Target{}, "Tonklin Motor is not a special")
	useRefused(t, e, s, part(d, Colonizer), Target{}, "Colonizer is used by an order")
	useRefused(t, e, s, part(d, XRayMegaglasses), Target{}, "X Ray Megaglasses is always on")
	useRefused(t, e, s, tie, Target{}, "Hyperswapper needs a ship target")
	useRefused(t, e, s, acc, Target{Kind: ShipTarget}, "Accutron takes no ship target")
	// "not modelled yet" (specialCheck) has no gizmo left: every
	// non-passive special has an effect (targetCheck's, the Carbonizer's
	// lane and planet targets, is TestTargetCheck's).
	s.Lane = 0
	useRefused(t, e, s, acc, Target{}, "in a star lane")
	s.Lane = -1
	s.Power = Gizmos[Accutron].Power - 1
	useRefused(t, e, s, acc, Target{}, "not enough energy")
	s.Shots[acc] = Gizmos[Accutron].Uses
	useRefused(t, e, s, acc, Target{}, "no use left today") // before the energy
	s.Refill()
	useOK(t, e, s, acc)
	if s.Shots[acc] != 1 || s.Power != d.PowerPerDay()-Gizmos[Accutron].Power {
		t.Errorf("paid: shots %v power %d", s.Shots, s.Power)
	}
}

// Usable and UseSpecial never disagree on what Usable can see: every
// Special but the two order-used ones, fresh, in a lane, short of energy
// and used up.
func TestUsableAgrees(t *testing.T) {
	for g := Gizmo(0); g < NumGizmos; g++ {
		in := &Gizmos[g]
		if in.Cat != Special || g == Colonizer || g == InvasionModule {
			continue
		}
		d := Design{Hull: Medium, Parts: []Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, TonklinMotor, g}}
		k := part(d, g)
		ks := targetKinds(g)
		_, own := specialEffects[g]
		// check builds a fresh ship, bends it (a use that succeeds changes
		// it) and compares the two verdicts.
		check := func(state string, bend func(s *Starship)) *Starship {
			t.Helper()
			_, e, s := specialShip(t, d)
			bend(s)
			ok, why := s.Usable(k)
			_, err := e.UseSpecial(s, k, Target{Kind: ks[0]})
			switch {
			case !ok && (err == nil || err.Error() != why):
				t.Errorf("%s %s: Usable %q, UseSpecial %v", in.Name, state, why, err)
			case ok && why != "use":
				t.Errorf("%s %s: Usable %v %q", in.Name, state, ok, why)
			case ok && !own:
				t.Errorf("%s %s: usable but not modelled", in.Name, state)
			}
			return s
		}
		s := check("fresh", func(*Starship) {})
		check("in a lane", func(s *Starship) { s.Lane = 0 })
		if in.Power > 0 {
			check("short of energy", func(s *Starship) { s.Power = in.Power - 1 })
		}
		if in.Uses > 0 {
			check("used up", func(s *Starship) { s.Shots[k] = in.Uses })
		}
		_, _, s = specialShip(t, d)
		switch {
		case in.Flags&GzPassive != 0:
			if ok, why := s.Usable(k); ok || why != in.Name+" is always on" {
				t.Errorf("%s passive: %v %q", in.Name, ok, why)
			}
		case specialEffects[g].apply == nil:
			if ok, why := s.Usable(k); ok || why != "not modelled yet" {
				t.Errorf("%s: %v %q", in.Name, ok, why)
			}
		}
	}
}

// Hyperfuel: Special1 energy, the part lost (Shots and Weapon follow).
func TestHyperfuel(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Hyperfuel, MassBarrageGun}}
	_, e, s := specialShip(t, d)
	s.Weapon = part(d, MassBarrageGun)
	before := s.Power
	useOK(t, e, s, part(d, Hyperfuel))
	if s.Power != before-Gizmos[Hyperfuel].Power+Gizmos[Hyperfuel].Special1 {
		t.Errorf("power %d from %d", s.Power, before)
	}
	if slices.Contains(s.Design.Parts, Hyperfuel) || len(s.Shots) != len(s.Design.Parts) || s.Weapon != -1 {
		t.Errorf("after: parts %v shots %v weapon %d", s.Design.Parts, s.Shots, s.Weapon)
	}
	if !slices.Contains(d.Parts, Hyperfuel) {
		t.Error("the design's parts were mutated")
	}
}

// Toroidal Blaster: Special1 energy, the first drive lost; no drive, a
// refusal; a ship with no drive left does not move (Hop: nothing, the
// order dropped by Check) and its close ends.
func TestToroidalBlaster(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, IonBanger, ToroidalBlaster}}
	g, e, s := specialShip(t, d)
	in := &Gizmos[ToroidalBlaster]
	before := s.Power
	useOK(t, e, s, part(d, ToroidalBlaster))
	if s.Power != before-in.Power+in.Special1 || !slices.Equal(s.Design.Parts, []Gizmo{ProtonShaver, IonBanger, ToroidalBlaster}) {
		t.Fatalf("first use: power %d parts %v", s.Power, s.Design.Parts)
	}
	k := part(s.Design, ToroidalBlaster)
	if s.Shots[k] != 1 {
		t.Errorf("the blaster's use moved with it: %v", s.Shots)
	}
	useRefused(t, e, s, k, Target{}, "no use left today")
	s.Refill()
	useOK(t, e, s, k)
	k = part(s.Design, ToroidalBlaster)
	if s.Design.Reach() != 0 {
		t.Fatalf("drives left: %v", s.Design.Parts)
	}
	s.Refill()
	useRefused(t, e, s, k, Target{}, "no engine aboard")

	s.Order = Order{Kind: GoToPoint, Pos: Vec3{X: 1}}
	if moved, arrived := s.Hop(g.Cluster, e.Day, nil, nil); moved || arrived || s.Order.Kind != NoOrder {
		t.Errorf("no drive: hop %v %v, order %+v", moved, arrived, s.Order)
	}
	s.Order = Order{Kind: GoToPoint, Pos: Vec3{X: 1}}
	if s.Step(g.Cluster, e.Day+1, nil, nil) {
		t.Error("a ship with no drive arrived")
	}
}

// Cannibalizer: Special1 hull points for Special2 energy; refused when
// that would leave no hull.
func TestCannibalizer(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Cannibalizer}}
	_, e, s := specialShip(t, d)
	in := &Gizmos[Cannibalizer]
	k := part(d, Cannibalizer)
	hull, power := s.Hull, s.Power
	useOK(t, e, s, k)
	if s.Hull != hull-in.Special1 || s.Power != power-in.Power+in.Special2 {
		t.Errorf("hull %d power %d, from %d %d", s.Hull, s.Power, hull, power)
	}
	s.Refill()
	s.Hull = in.Special1
	useRefused(t, e, s, k, Target{}, "would scuttle the ship")
}

// Replenisher: every weapon's shots back to 0; refused when none fired.
func TestReplenisher(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, MassBarrageGun, ElectromagneticPulser, Replenisher}}
	_, e, s := specialShip(t, d)
	k := part(d, Replenisher)
	useRefused(t, e, s, k, Target{}, "nothing to replenish")
	gun, emp := part(d, MassBarrageGun), part(d, ElectromagneticPulser)
	s.Shots[gun], s.Shots[emp] = Gizmos[MassBarrageGun].Uses, Gizmos[ElectromagneticPulser].Uses
	power := s.Power
	useOK(t, e, s, k)
	if s.Shots[gun] != 0 || s.Shots[emp] != 0 || s.Shots[k] != 1 || s.Power != power-Gizmos[Replenisher].Power {
		t.Errorf("shots %v power %d", s.Shots, s.Power)
	}
}

// Remote Repair Facility: Special1 hull points a use, capped at MaxHull,
// as often as the energy lasts (Uses 0); refused on a full hull.
func TestRemoteRepair(t *testing.T) {
	d := Design{Hull: Medium, Parts: []Gizmo{Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, Nanotwirler, TonklinMotor, RemoteRepairFacility}}
	_, e, s := specialShip(t, d)
	in := &Gizmos[RemoteRepairFacility]
	k := part(d, RemoteRepairFacility)
	useRefused(t, e, s, k, Target{}, "hull is full")
	s.Power = 2 * in.Power
	full := s.MaxHull()
	s.Hull = full - in.Special1 - 2
	useOK(t, e, s, k)
	if s.Hull != full-2 {
		t.Errorf("hull %d, want %d", s.Hull, full-2)
	}
	useOK(t, e, s, k)
	if s.Hull != full || s.Power != 0 || s.Shots[k] != 2 {
		t.Errorf("capped: hull %d of %d, power %d, uses %d", s.Hull, full, s.Power, s.Shots[k])
	}
	s.Hull--
	useRefused(t, e, s, k, Target{}, "not enough energy")
}

// Accutron: Boosted, the weapons' range doubled until the Refill; a
// target just out of Range is fired at in a Round only when boosted.
func TestAccutron(t *testing.T) {
	d := Design{Name: "Spotter", Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, MassBarrageGun, Accutron}}
	g, sh, tg := roundDuel(d, hulk)
	e := g.Empires[0]
	gun, k := part(d, MassBarrageGun), part(d, Accutron)
	w := &Gizmos[MassBarrageGun]
	r := float64(w.Range) * RangeUnit
	if sh.WeaponRange(w) != r {
		t.Fatalf("range %v, want %v", sh.WeaponRange(w), r)
	}
	tg.Pos = sh.Pos
	tg.Pos.X += 1.5 * r
	if sh.CanFire(g.Cluster, gun, tg) == nil {
		t.Error("CanFire beyond Range")
	}
	if err := arm(t, g, sh, gun, tg); err != nil {
		t.Fatal(err)
	}
	if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != 0 || sh.Shots[gun] != 0 {
		t.Fatalf("out of range fired: %+v", log.Shots)
	}
	useOK(t, e, sh, k)
	if !sh.Boosted || sh.WeaponRange(w) != 2*r || sh.CanFire(g.Cluster, gun, tg) != nil {
		t.Fatalf("boosted %v range %v", sh.Boosted, sh.WeaponRange(w))
	}
	if err := arm(t, g, sh, gun, tg); err != nil {
		t.Fatal(err)
	}
	hull := tg.Hull
	if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != 1 || tg.Hull != hull-w.Level {
		t.Fatalf("boosted: shots %+v hull %d of %d", log.Shots, tg.Hull, hull)
	}
	sh.Shots[k], sh.Power = 0, Gizmos[Accutron].Power
	useRefused(t, e, sh, k, Target{}, "already active today")
	sh.Refill()
	if sh.Boosted || sh.WeaponRange(w) != r {
		t.Error("the Refill kept the boost")
	}
}

// Invulnerablizer: Invulnerable until the Refill, the part lost; a
// second one refused the same day; in a Round a shot at it is paid and
// lands nothing (Damage 0, the Level Absorbed, no destruction).
func TestInvulnerablizer(t *testing.T) {
	d := Design{Name: "Bubble", Hull: Small, Parts: []Gizmo{Nanotwirler, Nanotwirler, TonklinMotor, Invulnerablizer, Invulnerablizer}}
	g, sh, tg := roundDuel(oneGen, d)
	e1 := g.Empires[1]
	power := tg.Power
	useOK(t, e1, tg, part(d, Invulnerablizer))
	if !tg.Invulnerable || tg.Power != power-Gizmos[Invulnerablizer].Power || len(tg.Design.Parts) != len(d.Parts)-1 {
		t.Fatalf("after: %v power %d parts %v", tg.Invulnerable, tg.Power, tg.Design.Parts)
	}
	tg.Power = Gizmos[Invulnerablizer].Power
	useRefused(t, e1, tg, part(tg.Design, Invulnerablizer), Target{}, "already invulnerable today")

	gun := part(oneGen, MassBarrageGun)
	w := &Gizmos[MassBarrageGun]
	tg.Hull = w.Level // one shot would kill it
	energy := sh.Power
	if err := arm(t, g, sh, gun, tg); err != nil {
		t.Fatal(err)
	}
	_, reps, log := g.Round(sh.Star, nil)
	if len(log.Shots) != 1 || log.Shots[0].Damage != 0 || log.Shots[0].Absorbed != w.Level {
		t.Fatalf("shots %+v", log.Shots)
	}
	if tg.Hull != w.Level || len(log.Destroyed) != 0 || len(reps) != 0 || g.OwnerOf(tg) != e1 {
		t.Errorf("harmed: hull %d, destroyed %v", tg.Hull, log.Destroyed)
	}
	if sh.Shots[gun] != 1 || sh.Power != energy-w.Power {
		t.Errorf("the shot unpaid: shots %v energy %d", sh.Shots, sh.Power)
	}
	tg.Refill()
	if tg.Invulnerable {
		t.Error("the Refill kept it")
	}
}

// An invulnerable ship in orbit, hit through its planet's cascade, and
// one in space shot by a planet's gun: spent, nothing lands.
func TestInvulnerableAgainstPlanets(t *testing.T) {
	g, sh, home := siege(t)
	e0 := g.Empires[0]
	mate := e0.Spawn(orbitShip, home.Star, home.PlanetIdx)
	mate.Invulnerable = true
	hull := mate.Hull
	_, log := volley(t, g, sh, home.PlanetIdx, 1)
	if len(log.Shots) != 1 || log.Shots[0].Target != mate || log.Shots[0].Damage != 0 ||
		log.Shots[0].Absorbed != Gizmos[log.Shots[0].Gizmo].Level || mate.Hull != hull {
		t.Fatalf("cascade: %+v, hull %d of %d", log.Shots, mate.Hull, hull)
	}

	p := home.Planet
	slot := 0
	if err := p.BuildOrbital(slot, LongRangeOrbitalWhopper); err != nil {
		t.Fatal(err)
	}
	def, _ := LongRangeOrbitalWhopper.Defence()
	sh.Refill()
	sh.Invulnerable = true
	sg, ok := g.SightingOf(e0, sh)
	if !ok {
		t.Fatal("the siege gun unseen")
	}
	if err := e0.ArmPlanet(home, slot, sg); err != nil {
		t.Fatal(err)
	}
	hull = sh.Hull
	_, _, log = g.Round(home.Star, nil)
	if len(log.Shots) != 1 || !log.Shots[0].FromPlanet || log.Shots[0].Damage != 0 || log.Shots[0].Absorbed != def.Damage ||
		sh.Hull != hull || home.OrbitShots[slot] != 1 {
		t.Errorf("planet gun: %+v, hull %d of %d, shots %v", log.Shots, sh.Hull, hull, home.OrbitShots)
	}
}

// Recaller: the ship at once at its home star, in space at the FreeSpot
// of the home world's LeavePoint against every empire's ships there, its
// order and armed shots dropped; refused at home and in a lane.
func TestRecaller(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{Nanotwirler, TonklinMotor, Recaller}}
	g, e, s := specialShip(t, d)
	for _, x := range g.Empires {
		if x.HomeStar() != x.Colonies[0].Star {
			t.Errorf("%s: HomeStar %d, home colony at %d", x.Species, x.HomeStar(), x.Colonies[0].Star)
		}
	}
	if lone := NewEmpire(1, Orfa, Average); lone.HomeStar() != lone.Cluster.Home {
		t.Errorf("a lone empire's home %d, want %d", lone.HomeStar(), lone.Cluster.Home)
	}
	home := e.HomeStar()
	k := part(d, Recaller)
	useRefused(t, e, s, k, Target{}, "already home")

	away := g.Cluster.Other(g.Cluster.Stars[home].Lanes[0], home)
	s.Star, s.Pos, s.Orbit = away, Vec3{X: 0.5}, -1
	s.Order = Order{Kind: GoToPoint, Pos: Vec3{X: 0.7}}
	s.Armed = []Shot{{Part: 0}}
	s.Lane = 0
	useRefused(t, e, s, k, Target{}, "in a star lane")
	s.Lane = -1
	col := e.Colonies[0]
	col.Project = &Project{Item: OrbitalDocks, Slot: -1, Orbital: true, Refit: s.Name}
	useRefused(t, e, s, k, Target{}, "being refitted")
	col.Project = nil

	leave := LeavePoint(g.Cluster, home, 0)
	blocker := g.Empires[1].Spawn(orbitShip, home, 0)
	inSpaceAt(blocker, leave)
	power := s.Power
	useOK(t, e, s, k)
	want := FreeSpot(leave, []Vec3{blocker.Pos})
	if s.Star != home || s.Pos != want || want == leave || s.Orbit != -1 || s.Lane != -1 ||
		s.Order != (Order{}) || s.Armed != nil {
		t.Errorf("recalled: star %d pos %v (want %v) orbit %d lane %d order %+v armed %v", s.Star, s.Pos, want, s.Orbit, s.Lane, s.Order, s.Armed)
	}
	if s.Power != power-Gizmos[Recaller].Power || s.Shots[k] != 1 || !slices.Contains(s.Design.Parts, Recaller) {
		t.Errorf("paid: power %d shots %v parts %v", s.Power, s.Shots, s.Design.Parts)
	}
}

// LosePart: a new Parts slice (the design and a sister ship untouched),
// Shots shifted with it, Weapon reset from the lost index on.
func TestLosePart(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun, MassBarrageGun}}
	_, e, s := specialShip(t, d)
	sister := e.Spawn(d, s.Star, 0)
	copy(s.Shots, []int{0, 1, 2, 3})
	s.Weapon = 0
	s.LosePart(1)
	if !slices.Equal(s.Design.Parts, []Gizmo{ProtonShaver, MassBarrageGun, MassBarrageGun}) || !slices.Equal(s.Shots, []int{0, 2, 3}) || s.Weapon != 0 {
		t.Errorf("parts %v shots %v weapon %d", s.Design.Parts, s.Shots, s.Weapon)
	}
	if !slices.Equal(d.Parts, []Gizmo{ProtonShaver, TonklinMotor, MassBarrageGun, MassBarrageGun}) || !slices.Equal(sister.Design.Parts, d.Parts) {
		t.Errorf("shared parts mutated: %v / %v", d.Parts, sister.Design.Parts)
	}
	s.Weapon = 2
	s.LosePart(1)
	if s.Weapon != -1 || len(s.Shots) != 2 {
		t.Errorf("weapon %d shots %v", s.Weapon, s.Shots)
	}
	// A refit to the full design pays the lost parts again.
	if got, want := RefitCost(s, d), Gizmos[TonklinMotor].Industry+Gizmos[MassBarrageGun].Industry; got != want {
		t.Errorf("RefitCost %d, want %d", got, want)
	}
}

// Star Lane Hyperdrive: its Level in LaneDrives and LaneStrength (twice
// one Star Lane Drive's, measured), a lane entered with it lasts its
// LaneDays; Check takes it as a lane drive.
func TestHyperdrive(t *testing.T) {
	hyper := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, StarLaneHyperdrive}}
	one := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, StarLaneDrive}}
	if hyper.LaneDrives() != Gizmos[StarLaneHyperdrive].Level || hyper.LaneStrength() != float64(Gizmos[StarLaneHyperdrive].Level) || hyper.LaneStrength() != 2*one.LaneStrength() {
		t.Fatalf("LaneDrives %d, LaneStrength %g / %g", hyper.LaneDrives(), hyper.LaneStrength(), one.LaneStrength())
	}
	g, e, s := specialShip(t, hyper)
	c := g.Cluster
	li := c.Stars[s.Star].Lanes[0]
	to := c.Other(li, s.Star)
	s.Pos = c.Stars[s.Star].System.Lanes[0].Pos
	if err := e.Order(s, Order{Kind: GoToStar, Star: to}); err != nil {
		t.Fatal(err)
	}
	s.Power = hyper.PowerPerDay()
	if _, arrived := s.Hop(c, e.Day, nil, nil); !arrived || s.Lane != li {
		t.Fatalf("lane not entered: lane %d", s.Lane)
	}
	if want := e.Day + LaneDays(c, li, hyper.LaneStrength(), s.Species.Traits()); s.Arrive != want {
		t.Errorf("arrive %d, want %d", s.Arrive, want)
	}
}

// Gyro-Inductor: a move leaving an orbit gets its MoveCost back once,
// whatever the number aboard (never a gain), the bounce off a guarded
// orbit included; a move in space pays.
func TestGyroInductor(t *testing.T) {
	gyro := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, GyroInductor}}
	plain := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor}}
	cost := gyro.MoveCost()
	for _, c := range []struct {
		d     Design
		orbit bool
		paid  int
	}{{gyro, true, 0}, {plain, true, cost}, {gyro, false, cost},
		{Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, GyroInductor, GyroInductor}}, true, 0}} {
		g, e, s := specialShip(t, c.d)
		if c.orbit {
			s.Orbit = e.Colonies[0].PlanetIdx
		}
		s.Order = Order{Kind: GoToPoint, Pos: LeavePoint(g.Cluster, s.Star, 0)}
		power := s.Power
		if moved, _ := s.Hop(g.Cluster, e.Day, nil, nil); !moved || power-s.Power != c.paid || s.Orbit != -1 {
			t.Errorf("%v in orbit %v: moved %v, paid %d, want %d", c.d.Parts, c.orbit, moved, power-s.Power, c.paid)
		}
	}
	// The bounce: in orbit of planet 0 (its Pos set beside planet 1), sent
	// to planet 1, barred.
	g, e, s := specialShip(t, gyro)
	sys := g.Cluster.Stars[s.Star].System
	if len(sys.Planets) < 2 {
		t.Fatal("the home system has one planet")
	}
	s.Orbit, s.Pos = 0, sys.Planets[1].Pos
	s.Pos.X += PlanetClear + ShipGap + 0.05
	s.Order = Order{Kind: GoToPlanet, Planet: 1}
	power := s.Power
	moved, arrived := s.Hop(g.Cluster, e.Day, nil, func(int, int) bool { return true })
	if !moved || arrived || s.Orbit != -1 || s.Power != power {
		t.Errorf("bounce: moved %v arrived %v orbit %d power %d of %d", moved, arrived, s.Orbit, s.Power, power)
	}
}

// X Ray Megaglasses: a Scan carries the parts only when a ship of the
// scanning empire at that star, out of the lanes, has them, or under
// ScanAll; the status is read either way.
func TestXRay(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	e0, e1 := g.Empires[0], g.Empires[1]
	home := e0.Colonies[0].Star
	e0.Spawn(Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, TonklinFrequencyAnalyzer}}, home, 0)
	raider := e1.Spawn(FixtureRaider, home, 0)
	raider.Orbit = -1
	seen := func() *Scan {
		t.Helper()
		sg, ok := g.SightingOf(e0, raider)
		if !ok || sg.Scan == nil {
			t.Fatal("raider not scanned")
		}
		return sg.Scan
	}
	if sc := seen(); sc.Parts || sc.Design.Parts != nil || sc.Design.Hull != raider.Design.Hull || sc.Design.Name != raider.Design.Name ||
		sc.HullPts != raider.Hull || sc.Energy != raider.Power || sc.Shields != raider.Shields {
		t.Errorf("without X Ray: %+v", sc)
	}
	glasses := e0.Spawn(Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, XRayMegaglasses}}, home, 0)
	if sc := seen(); !sc.Parts || !slices.Equal(sc.Design.Parts, raider.Design.Parts) {
		t.Errorf("with X Ray: %+v", sc)
	}
	glasses.Lane = 0
	if sc := seen(); sc.Parts {
		t.Error("X Ray from a lane")
	}
	e0.ScanAll = true
	if sc := seen(); !sc.Parts || !slices.Equal(sc.Design.Parts, raider.Design.Parts) {
		t.Errorf("ScanAll: %+v", sc)
	}
}

// The Accutron's range at a planet: a shot at an orbit just out of
// Range is refused unboosted (ArmAtPlanet), fires boosted.
func TestAccutronAtPlanet(t *testing.T) {
	g, sh, home := siege(t)
	e1 := g.Empires[1]
	g.Empires[0].Spawn(orbitShip, home.Star, home.PlanetIdx) // something to hit
	gun := slices.IndexFunc(sh.Design.Parts, func(x Gizmo) bool { return Gizmos[x].Cat == Weapon })
	w := &Gizmos[sh.Design.Parts[gun]]
	sh.Pos = home.Planet.Pos
	sh.Pos.X += 1.5 * float64(w.Range) * RangeUnit
	for _, boosted := range []bool{false, true} {
		sh.Refill()
		sh.Boosted = boosted
		if err := e1.ArmAtPlanet(sh, gun, home.PlanetIdx); err != nil {
			if boosted || err.Error() != w.Name+": out of range" {
				t.Fatalf("boosted %v: %v", boosted, err)
			}
		}
		want := 0
		if boosted {
			want = 1
		}
		if _, _, log := g.Round(sh.Star, nil); len(log.Shots) != want {
			t.Errorf("boosted %v: shots %+v", boosted, log.Shots)
		}
	}
}

// A shield lost while on gives back its share of the reservation: the
// invariant (ShieldDraw off Power while on) holds on the smaller design.
func TestLosePartShieldRefund(t *testing.T) {
	_, s, _ := duel(fightFixture, fightFixture)
	power, k := s.Power, part(fightFixture, IonWrap)
	s.LosePart(k)
	if want := power + Gizmos[IonWrap].Power; !s.Shields || s.Power != want || s.Design.ShieldDraw() != fightFixture.ShieldDraw()-Gizmos[IonWrap].Power {
		t.Errorf("shield lost: shields %v, power %d, want %d, draw %d", s.Shields, s.Power, want, s.Design.ShieldDraw())
	}
	_, s, _ = duel(fightFixture, fightFixture)
	_ = s.SetShields(false)
	power = s.Power
	s.LosePart(k)
	if s.Power != power {
		t.Errorf("shield lost while off: power %d, want %d", s.Power, power)
	}
}

// The consumables are the cargo parts and the once specials; a ship's
// Aboard leaves those spent out, in Counts order.
func TestConsumables(t *testing.T) {
	for g, want := range map[Gizmo]bool{Colonizer: true, InvasionModule: true, Hyperfuel: true, SmartBomb: true,
		StarLaneDrive: false, TonklinMotor: false, Gizmo(0): false} {
		if g.Consumable() != want {
			t.Errorf("%s: consumable %v", Gizmos[g].Name, !want)
		}
	}
	d := Design{Hull: Medium, Parts: []Gizmo{TonklinMotor, Colonizer, Colonizer, InvasionModule, Hyperfuel, StarLaneDrive}}
	if got, want := d.Consumables(), []PartCount{{Colonizer, 2}, {InvasionModule, 1}, {Hyperfuel, 1}}; !slices.Equal(got, want) {
		t.Errorf("design: %v, want %v", got, want)
	}
	s := &Starship{Design: d, Cargo: 1}
	if got, want := s.Aboard().Consumables(), []PartCount{{Colonizer, 1}, {Hyperfuel, 1}}; !slices.Equal(got, want) {
		t.Errorf("one colonist left, no troops: %v, want %v", got, want)
	}
}

// A SpecialUsed Event's Use is never written: the JSON of an Event with
// one is the JSON without (save.Entry holds Events).
func TestSpecialUseNotSaved(t *testing.T) {
	ev := Event{Kind: SpecialUsed, Item: NoItem, Tech: NoTech, Star: 3, Planet: -1, Other: -1, Text: "x"}
	bare, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	ev.Use = &SpecialUse{Gizmo: Disarmer, Outcome: "destroyed", TargetEmpire: 1}
	with, err := json.Marshal(ev)
	if err != nil || string(with) != string(bare) {
		t.Errorf("with a Use %s, without %s (%v)", with, bare, err)
	}
}

// hideName replaces a name only after the user's own, so a target whose
// name is a prefix of the user's is caught without touching the user.
func TestHideName(t *testing.T) {
	got := hideName("Raider II's Disarmer destroys Raider I's Ueberlaser", "Raider II", "Raider I", "a Mebes ship")
	if want := "Raider II's Disarmer destroys a Mebes ship's Ueberlaser"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}
