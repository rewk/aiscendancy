package game

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

// refitFixture is shipEmpire with Orbital Docks in slot 1 (their tech
// granted) and a fresh colony ship s in orbit of the home.
func refitFixture(t *testing.T) (*Empire, *Colony, *Starship) {
	t.Helper()
	e, home := shipEmpire(t)
	e.Research.Grant(Tech(Items[OrbitalDocks].ResearchReq))
	if err := home.Planet.BuildOrbital(1, OrbitalDocks); err != nil {
		t.Fatal(err)
	}
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	return e, home, e.Spawn(d, home.Star, home.PlanetIdx)
}

// withGen is s's design with its last part (a Colonizer) swapped for a
// Proton Shaver: one part added, one removed.
func withGen(s *Starship) Design {
	d := s.Design
	d.Parts = slices.Clone(d.Parts)
	d.Parts[len(d.Parts)-1] = ProtonShaver
	return d
}

func TestRefitCost(t *testing.T) {
	ind := func(g Gizmo) int { return Gizmos[g].Industry }
	ship := func(hull int, parts ...Gizmo) *Starship {
		s := &Starship{Design: Design{Hull: Small, Parts: parts}, Species: Snovemdomas}
		s.Hull = s.MaxHull() - hull
		s.Cargo, s.Invaders = s.Design.Colonizers(), s.Design.InvasionModules() // loaded, as Spawn does
		return s
	}
	spent := func(s *Starship) *Starship { s.Cargo, s.Invaders = 0, 0; return s }
	full := ship(0).MaxHull()
	repair := func(lost int) int { return int(math.Ceil(float64(lost*Small.Cost()) / float64(full))) }
	base := []Gizmo{TonklinMotor, ProtonShaver, Colonizer, Colonizer, Colonizer}
	for _, tc := range []struct {
		name   string
		s      *Starship
		target []Gizmo
		want   int
	}{
		{"one added", ship(0, base...), append(slices.Clone(base), ProtonShaver), ind(ProtonShaver)},
		{"multiset: a second copy is added", ship(0, ProtonShaver), []Gizmo{ProtonShaver, ProtonShaver}, ind(ProtonShaver)},
		{"swap", ship(0, base...), []Gizmo{TonklinMotor, ProtonShaver, Colonizer, Colonizer, StarLaneDrive}, ind(StarLaneDrive)},
		{"order is free", ship(0, base...), []Gizmo{Colonizer, Colonizer, ProtonShaver, Colonizer, TonklinMotor}, 0},
		{"removal refunds nothing", ship(0, base...), base[:3], 0},
		{"pure repair, ceil", ship(3, base...), base, repair(3)},
		{"repair of a wreck at 1", ship(full-1, base...), base, repair(full - 1)},
		{"repair plus a part", ship(1, base...), append(slices.Clone(base), MassBarrageGun), repair(1) + ind(MassBarrageGun)},
		// The parts counted are the ship's aboard: a spent module re-added is paid.
		{"spent invasion module re-added", spent(ship(0, TonklinMotor, InvasionModule)), []Gizmo{TonklinMotor, InvasionModule}, ind(InvasionModule)},
		{"spent colonizers re-added", spent(ship(0, base...)), base, 3 * ind(Colonizer)},
		{"a loaded module is kept", ship(0, TonklinMotor, InvasionModule), []Gizmo{TonklinMotor, InvasionModule}, 0},
	} {
		d := Design{Hull: Small, Parts: tc.target}
		if got := RefitCost(tc.s, d); got != tc.want {
			t.Errorf("%s: cost %d, want %d", tc.name, got, tc.want)
		}
	}
	if full*Small.Cost()%3 == 0 {
		t.Fatal("the ceil case divides exactly: pick another loss")
	}
}

func TestCanRefit(t *testing.T) {
	if _, _, s := refitFixture(t); RefitCost(s, withGen(s)) <= 0 {
		t.Fatal("fixture refit costs nothing")
	}
	for _, tc := range []struct {
		name string
		set  func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship)
		want string // "" for the happy path
		is   error
	}{
		{"happy", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) { return c, s }, "", nil},
		{"foreign ship", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			cp := *s
			return c, &cp
		}, "not ours", nil},
		{"foreign colony", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			return &Colony{Planet: c.Planet, Star: c.Star, PlanetIdx: c.PlanetIdx}, s
		}, "not our colony", nil},
		{"no docks", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			c.Planet.Orbit[1] = NoItem
			return c, s
		}, "no Orbital Docks", nil},
		{"in space", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			s.Orbit = -1
			return c, s
		}, "not in orbit", nil},
		{"another planet", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			s.Orbit = c.PlanetIdx + 1
			return c, s
		}, "not in orbit", nil},
		{"in a lane", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			s.Lane = 0
			return c, s
		}, "not in orbit", nil},
		{"another hull", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			d.Hull = Medium
			return c, s
		}, "keeps the hull", nil},
		{"invalid", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			d.Parts = []Gizmo{ProtonShaver, Colonizer}
			return c, s
		}, "no engine", nil},
		{"nothing to refit", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			d.Parts = slices.Clone(s.Design.Parts)
			slices.Reverse(d.Parts) // the same multiset in another order
			return c, s
		}, "nothing to refit", nil},
		{"already refitting", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			if err := e.Refit(c, s, *d); err != nil {
				t.Fatal(err)
			}
			return c, s
		}, "being refitted", nil},
		{"project running", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			c.Project = &Project{Item: Factory}
			return c, s
		}, "", ErrProjectRunning},
		{"no free worker", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			c.Pop = c.Busy()
			return c, s
		}, "", ErrNoFreePop},
		{"no research", func(e *Empire, c *Colony, s *Starship, d *Design) (*Colony, *Starship) {
			e.Research.Known[Items[OrbitalDocks].ResearchReq] = false
			return c, s
		}, "needs", nil},
	} {
		e, c, s := refitFixture(t)
		d := withGen(s)
		c, s = tc.set(e, c, s, &d)
		err := e.CanRefit(c, s, d)
		switch {
		case tc.want == "" && tc.is == nil:
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		case tc.is != nil:
			if !errors.Is(err, tc.is) {
				t.Errorf("%s: %v, want %v", tc.name, err, tc.is)
			}
		case err == nil || !strings.Contains(err.Error(), tc.want):
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	// A spent Colonizer is something to refit: the design re-adds it.
	e, c, s := refitFixture(t)
	s.Cargo--
	if err := e.CanRefit(c, s, s.Design); err != nil {
		t.Errorf("a spent Colonizer re-added: %v", err)
	}
	if err := e.CanRefit(c, s, s.Aboard()); err == nil || err.Error() != "nothing to refit" {
		t.Errorf("the parts aboard, full hull: %v", err)
	}
	// A damaged hull is something to refit with the same parts.
	e, c, s = refitFixture(t)
	s.Hull--
	if err := e.CanRefit(c, s, s.Design); err != nil {
		t.Errorf("pure repair: %v", err)
	}
	// The running refit names its ship in the other projects' refusal.
	if err := e.Refit(c, s, s.Design); err != nil {
		t.Fatal(err)
	}
	if err := c.CanStartOrbital(e.Research, Ship, 2); !errors.Is(err, ErrProjectRunning) || !strings.Contains(err.Error(), "refit of "+s.Name) {
		t.Errorf("running refit: %v", err)
	}
}

func TestRefitStartsAndHolds(t *testing.T) {
	e, c, s := refitFixture(t)
	d := withGen(s)
	busy := c.Busy()
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: c.PlanetIdx + 1}); err != nil {
		t.Fatal(err)
	}
	if err := e.Refit(c, s, d); err != nil {
		t.Fatal(err)
	}
	pj := c.Project
	if pj.Refit != s.Name || pj.Item != OrbitalDocks || !pj.Orbital || pj.Slot != -1 || pj.Cost() != RefitCost(s, d) ||
		pj.Design == nil || !slices.Equal(pj.Design.Parts, d.Parts) {
		t.Errorf("project %+v", pj)
	}
	d.Parts[0] = Colonizer // the project holds its own copy
	if pj.Design.Parts[0] == Colonizer {
		t.Error("the project shares the caller's parts")
	}
	if c.Busy() != busy+Items[OrbitalDocks].UsePop {
		t.Errorf("busy %d, want %d", c.Busy(), busy+Items[OrbitalDocks].UsePop)
	}
	if s.Order.Kind != NoOrder {
		t.Errorf("order %v kept", s.Order)
	}
	if e.Refitting(s) != c {
		t.Error("Refitting")
	}
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: c.PlanetIdx + 1}); err == nil || !strings.Contains(err.Error(), "being refitted at "+c.Planet.Name) {
		t.Errorf("order while refitting: %v", err)
	}
	// Not a ship: the fleet counts the ships only.
	if e.fleet() != len(e.Ships) {
		t.Errorf("fleet %d with %d ships", e.fleet(), len(e.Ships))
	}
}

func TestRefitCompletes(t *testing.T) {
	e, c, s := refitFixture(t)
	s.Hull -= 2
	s.Cargo = 0
	s.Invaders = 3 // the design carries none: the refit unloads them
	s.Weapon = 0
	d := withGen(s)
	orbit, pop := c.Planet.Orbit, c.Pop
	if err := e.Refit(c, s, d); err != nil {
		t.Fatal(err)
	}
	days, ok := c.DaysLeft()
	if !ok || days < 2 {
		t.Fatalf("days left %d %v: the refit should take several days", days, ok)
	}
	var got []Event
	day := runEmpire(t, e, 100, func(evs []Event) bool {
		for _, x := range evs {
			if x.Kind == Refitted {
				got = append(got, x)
			}
		}
		return len(got) > 0
	})
	if day != days {
		t.Errorf("refitted on day %d, DaysLeft said %d", day, days)
	}
	want := Event{Kind: Refitted, Item: OrbitalDocks, Tech: NoTech, Star: c.Star, Planet: c.PlanetIdx, Other: -1,
		Text: s.Name + " refitted at " + c.Planet.Name}
	if len(got) != 1 || got[0] != want {
		t.Errorf("events %+v, want %+v", got, want)
	}
	if !slices.Equal(s.Design.Parts, d.Parts) || s.Design.Hull != d.Hull || s.Hull != s.MaxHull() || s.Cargo != d.Colonizers() ||
		s.Invaders != d.InvasionModules() || len(s.Shots) != len(d.Parts) || s.Weapon != -1 {
		t.Errorf("ship after refit: %+v", s)
	}
	if c.Planet.Orbit != orbit || c.Pop != pop || c.Project != nil {
		t.Errorf("orbit %v (was %v), pop %d (was %d), project %+v", c.Planet.Orbit, orbit, c.Pop, pop, c.Project)
	}
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: c.PlanetIdx + 1}); err != nil {
		t.Errorf("order after the refit: %v", err)
	}
	if Refitted.String() != "refitted" {
		t.Errorf("EventKind String %q", Refitted.String())
	}
}

// A destroyed ship's refit is cancelled at the next close, before any
// industry goes to it: no event, the worker free.
func TestRefitCancelledWhenLost(t *testing.T) {
	e, c, s := refitFixture(t)
	busy := c.Busy()
	if err := e.Refit(c, s, withGen(s)); err != nil {
		t.Fatal(err)
	}
	e.Ships = slices.DeleteFunc(e.Ships, func(x *Starship) bool { return x == s })
	for _, x := range e.EndDay() {
		if x.Kind == Refitted || x.Kind == Built {
			t.Errorf("event %+v", x)
		}
	}
	if c.Project != nil || c.Busy() != busy {
		t.Errorf("project %+v, busy %d (was %d)", c.Project, c.Busy(), busy)
	}
}

// A refit costing nothing (a part removed, the hull full) completes at
// the next close and still places no Docks.
func TestRefitFree(t *testing.T) {
	e, c, s := refitFixture(t)
	d := s.Design
	d.Parts = d.Parts[:len(d.Parts)-1]
	orbit := c.Planet.Orbit
	if err := e.Refit(c, s, d); err != nil {
		t.Fatal(err)
	}
	if c.Project.Cost() != 0 {
		t.Fatalf("cost %d", c.Project.Cost())
	}
	n := 0
	for _, x := range e.EndDay() {
		if x.Kind == Refitted {
			n++
		}
		if x.Item == OrbitalDocks && x.Kind == Built {
			t.Errorf("built %+v", x)
		}
	}
	if n != 1 || c.Planet.Orbit != orbit || len(s.Design.Parts) != len(d.Parts) {
		t.Errorf("%d refits, orbit %v (was %v), parts %v", n, c.Planet.Orbit, orbit, s.Design.Parts)
	}
}

func TestRefitTarget(t *testing.T) {
	e, _, s := refitFixture(t)
	if err := e.AddTemplate(ColonyTemplate(false)); err != nil || s.Design.Name != e.Templates[0].Name {
		t.Fatalf("fixture template: %v, ship %q", err, s.Design.Name)
	}
	d, err := e.RefitTarget(s)
	want, _ := e.Templates[0].Realise(e.Research)
	if err != nil || d.Name != s.Design.Name || !slices.Equal(d.Parts, want.Parts) {
		t.Errorf("target %v %v, want %v", d, err, want)
	}
	s.Design.Name = "Nobody"
	if _, err := e.RefitTarget(s); err == nil || !strings.Contains(err.Error(), "no template") {
		t.Errorf("no template: %v", err)
	}
	// AddTemplate refuses a Best part with nothing allowed: appended
	// directly, to walk RefitTarget's own guard.
	e.Templates = append(e.Templates, Template{Name: "Gunner", Hull: Small, Parts: []Part{{Best: true, Cat: Drive}, {Best: true, Cat: Weapon}}})
	s.Design.Name = "Gunner"
	if _, err := e.RefitTarget(s); err == nil || !strings.Contains(err.Error(), "not realisable") {
		t.Errorf("no weapon known: %v", err)
	}
	if err := e.AddTemplate(Template{Name: "Big", Hull: Medium, Parts: []Part{{Best: true, Cat: Drive}}}); err != nil {
		t.Fatal(err)
	}
	s.Design.Name = "Big"
	if _, err := e.RefitTarget(s); err == nil || !strings.Contains(err.Error(), "hull") {
		t.Errorf("another hull: %v", err)
	}
}

// WarshipOn fills the hull it is given, and refuses a hull research does
// not allow; Warship is WarshipOn on the largest allowed hull.
func TestWarshipOn(t *testing.T) {
	r := NewResearch()
	for _, tc := range append(slices.Clone(earlyShipTechs), Superconductivity) {
		r.Grant(tc)
	}
	big, ok := knownHull(r, true)
	if !ok || big == Small {
		t.Fatalf("largest hull %v %v", big, ok)
	}
	w, okW := Warship(r, false)
	on, okOn := WarshipOn(r, big, false)
	if !okW || !okOn || w.Hull != big || !slices.Equal(w.Parts, on.Parts) {
		t.Errorf("Warship %v %v, WarshipOn largest %v %v", w, okW, on, okOn)
	}
	if small, ok := WarshipOn(r, Small, false); !ok || small.Hull != Small || len(small.Parts) > Small.Slots() || small.Valid(r) != nil {
		t.Errorf("small %v %v", small, ok)
	}
	if _, ok := WarshipOn(r, Enormous, false); ok {
		t.Error("Enormous allowed without its tech")
	}
	if _, ok := WarshipOn(r, NumHulls, false); ok {
		t.Error("no hull allowed")
	}
}

// A refit while the shields are on refunds the old draw and takes the
// new one (the reservation re-applied); a draw the power left cannot pay
// leaves them off; shields off stay off.
func TestRefitShieldReservation(t *testing.T) {
	_, s, _ := duel(fightFixture, fightFixture)
	two := Design{Name: "Two", Hull: Small, Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, IonWrap, IonWrap}}
	heavy := Design{Name: "Heavy", Hull: Small, Parts: []Gizmo{ProtonShaver, ProtonShaver, TonklinMotor, HyperwaveNullifier}}
	old := fightFixture.ShieldDraw()
	if two.ShieldDraw() <= old || two.ShieldDraw() > two.PowerPerDay() || heavy.ShieldDraw() <= old || heavy.ShieldDraw() > heavy.PowerPerDay() {
		t.Fatalf("fixture: draws %d, %d (supply %d), %d (supply %d)", old, two.ShieldDraw(), two.PowerPerDay(), heavy.ShieldDraw(), heavy.PowerPerDay())
	}
	power := s.Power
	applyRefit(s, two)
	if want := min(power+old, two.PowerPerDay()) - two.ShieldDraw(); !s.Shields || s.Power != want {
		t.Errorf("refit on: shields %v, power %d, want %d", s.Shields, s.Power, want)
	}
	if err := s.SetShields(false); err != nil || s.Power != min(power+old, two.PowerPerDay()) {
		t.Errorf("off after the refit: %v, power %d (the new draw refunded)", err, s.Power)
	}

	_, s, _ = duel(fightFixture, fightFixture)
	s.Power = 0 // spent: the refund alone is short of heavy's draw
	applyRefit(s, heavy)
	if !s.Shields || s.Power != 0 {
		t.Errorf("refit short of the new draw: shields %v, power %d, want on at 0 (heavy's day carries them)", s.Shields, s.Power)
	}
	s.Refill()
	if !s.Shields || s.Power != heavy.PowerPerDay()-heavy.ShieldDraw() {
		t.Errorf("the Refill after: shields %v, power %d, want %d", s.Shields, s.Power, heavy.PowerPerDay()-heavy.ShieldDraw())
	}

	_, s, _ = duel(fightFixture, fightFixture)
	_ = s.SetShields(false)
	power = s.Power
	applyRefit(s, two)
	if s.Shields || s.Power != min(power, two.PowerPerDay()) {
		t.Errorf("refit off: shields %v, power %d", s.Shields, s.Power)
	}
}
