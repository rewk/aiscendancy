package game

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestNewEmpire(t *testing.T) {
	a, b := NewEmpire(1, Snovemdomas, Average), NewEmpire(1, Snovemdomas, Average)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two empires from one seed differ")
	}
	c, plain := a.Cluster, NewCluster(1, Average)
	if c.Home != plain.Home || !reflect.DeepEqual(c.Lanes, plain.Lanes) {
		t.Fatal("the empire's cluster is not NewCluster(seed)")
	}
	if len(a.Colonies) != 1 {
		t.Fatalf("%d colonies", len(a.Colonies))
	}
	home := a.Colonies[0]
	sp := Snovemdomas.Info()
	p := c.Stars[c.Home].System.Planets[0]
	if home.Star != c.Home || home.PlanetIdx != 0 || home.Planet != p || p.Type != sp.HomeType || p.Size != sp.HomeSize ||
		p.Name != "Snovemdomas Prime" || home.Pop != StartPop || p.Count(ColonyBase) != 1 {
		t.Errorf("home: star %d planet %d %s %v %d pop %d", home.Star, home.PlanetIdx, p.Name, p.Type, p.Size, home.Pop)
	}
	if p.Pos != plain.Stars[c.Home].System.Planets[0].Pos {
		t.Error("the home world did not keep the replaced planet's Pos")
	}
	if a.Colony(c.Home, 0) != home || !a.Inhabited(c.Home, 0) || a.Inhabited(c.Home, 1) || a.ControlledSystems() != 1 {
		t.Error("Colony/Inhabited/ControlledSystems")
	}
}

// shipEmpire is seed 1 Average with the colony-ship techs granted and the
// home given a Shipyard, two Factories and the people to build.
func shipEmpire(t *testing.T) (*Empire, *Colony) {
	t.Helper()
	e := NewEmpire(1, Snovemdomas, Average)
	for _, tc := range earlyShipTechs {
		e.Research.Grant(tc)
	}
	e.Research.Grant(SpacetimeSurfing)
	home := e.Colonies[0]
	for range 2 {
		at, ok := home.Planet.BestCell(Factory, Base)
		if !ok || home.Planet.Build(at, Factory) != nil {
			t.Fatal("no square for a Factory")
		}
	}
	if err := home.Planet.BuildOrbital(0, Shipyard); err != nil {
		t.Fatal(err)
	}
	home.Pop = 8
	return e, home
}

func TestBuildShipRefusals(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	for _, tc := range earlyShipTechs {
		e.Research.Grant(tc)
	}
	home := e.Colonies[0]
	home.Pop = 6
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	if err := e.BuildShip(home, d); err == nil || !strings.Contains(err.Error(), "shipyard") {
		t.Errorf("without a shipyard: %v", err)
	}
	e, home = shipEmpire(t)
	bad := Design{Name: "Rock", Hull: Small, Parts: []Gizmo{ProtonShaver, Colonizer}}
	if err := e.BuildShip(home, bad); err == nil || !strings.Contains(err.Error(), "no engine") {
		t.Errorf("invalid design: %v", err)
	}
	for range ShipsPerSystem {
		e.Ships = append(e.Ships, &Starship{Name: "x", Orbit: -1, Lane: -1})
	}
	if err := e.BuildShip(home, d); !errors.Is(err, ErrShipCap) {
		t.Errorf("over the cap: %v", err)
	}
	if err := e.CanStartShip(home, 0); !errors.Is(err, ErrShipCap) {
		t.Errorf("CanStartShip over the cap: %v", err)
	}
	// A running Ship project counts toward the cap.
	e.Ships = e.Ships[:ShipsPerSystem-1]
	if err := e.BuildShip(home, d); err != nil {
		t.Fatal(err)
	}
	if err := e.CanBuildShip(e.Colonies[0], d); !errors.Is(err, ErrShipCap) {
		t.Errorf("ship project not counted: %v", err)
	}
	if home.Project.Design == nil || home.Project.Cost() != d.Cost() || home.Project.Design.Name != d.Name {
		t.Errorf("project %+v", home.Project)
	}
}

// BuildShip refuses another empire's colony (CanRefit's, ArmPlanet's
// rule): a view keeping a colony captured under it (the design screen
// opened from a Shipyard, an invasion at the day's close) must not start
// a ship there, which would launch for the new owner.
func TestBuildShipNotOurs(t *testing.T) {
	e, home := shipEmpire(t)
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	o := NewEmpire(1, Orfa, Average) // the same techs: only the owner differs
	for _, tc := range earlyShipTechs {
		o.Research.Grant(tc)
	}
	o.Research.Grant(SpacetimeSurfing)
	if err := o.CanBuildShip(home, d); err == nil || !strings.Contains(err.Error(), "not our colony") {
		t.Errorf("another's colony: %v", err)
	}
	if err := o.BuildShip(home, d); err == nil || home.Project != nil {
		t.Errorf("another's colony: %v, project %+v", err, home.Project)
	}
	if err := e.BuildShip(home, d); err != nil {
		t.Errorf("our own: %v", err)
	}
}

// runEmpire runs days until stop holds, returning the day; fails after max.
func runEmpire(t *testing.T, e *Empire, max int, stop func([]Event) bool) int {
	t.Helper()
	for range max {
		if stop(e.EndDay()) {
			return e.Day
		}
	}
	t.Fatalf("not done in %d days", max)
	return 0
}

func TestColonyShipScenario(t *testing.T) {
	e, home := shipEmpire(t)
	d, ok := ColonyShip(e.Research, true)
	if !ok {
		t.Fatal("no colony ship")
	}
	if err := e.BuildShip(home, d); err != nil {
		t.Fatal(err)
	}
	busy, pop := home.Busy(), home.Pop
	start := e.Day
	ind, _, _ := home.Output()
	grew := 0
	built := runEmpire(t, e, 1000, func(ev []Event) bool {
		for _, x := range ev {
			if x.Kind == Grew && strings.Contains(x.Text, home.Planet.Name) {
				grew++
			}
			if x.Kind == Built && x.Item == Ship {
				return true
			}
		}
		return false
	})
	t.Logf("%s (cost %d) built in %d days at industry %d", d, d.Cost(), built-start, ind)
	if len(e.Ships) != 1 || home.Project != nil {
		t.Fatalf("ships %d, project %v", len(e.Ships), home.Project)
	}
	s := e.Ships[0]
	if s.Name != "Colonizer I" || s.Star != home.Star || s.Orbit != home.PlanetIdx || s.Pos != home.Planet.Pos ||
		s.Cargo != d.Colonizers() || s.Lane != -1 {
		t.Errorf("ship %+v", s)
	}
	// The crew left: Pop is one down from where growth took it, and the
	// Busy count is back to the structures' (the reserve released).
	if home.Busy() != busy-1 || home.Pop != pop+grew-1 {
		t.Errorf("busy %d (was %d with the project), pop %d (was %d, grew %d)", home.Busy(), busy, home.Pop, pop, grew)
	}
	if home.Planet.Count(Ship) != 0 {
		t.Error("a Ship item was placed in orbit")
	}

	// To the nearest uninhabited planet of the home system that has a
	// square for a base (a Husk has none), then found.
	target := -1
	for i := 1; i < len(e.Cluster.Stars[s.Star].System.Planets); i++ {
		if _, ok := e.Planet(s.Star, i).BestCell(ColonyBase, Base); ok {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("seed 1's home system has no colonisable planet")
	}
	s.Order = Order{Kind: GoToPlanet, Planet: target}
	runEmpire(t, e, 100, func([]Event) bool { return s.Orbit == target })
	p := e.Planet(s.Star, target)
	want, _ := p.BestCell(ColonyBase, Base)
	cargo := s.Cargo
	if err := e.Found(s); err != nil {
		t.Fatal(err)
	}
	col := e.Colony(s.Star, target)
	if col == nil || len(e.Colonies) != 2 || col.Pop != StartPop || p.Surface[want].Item != ColonyBase || s.Cargo != cargo-1 ||
		!e.Inhabited(s.Star, target) || e.ControlledSystems() != 1 {
		t.Fatalf("founding: colony %v, cargo %d", col, s.Cargo)
	}
	if err := e.Found(s); err == nil {
		t.Error("founded twice on one planet")
	}

	// Through a lane to an adjacent star.
	c := e.Cluster
	li := c.Stars[s.Star].Lanes[0]
	far := c.Other(li, s.Star)
	s.Order = Order{Kind: GoToStar, Star: far}
	entered := runEmpire(t, e, 100, func([]Event) bool { return s.Lane >= 0 })
	if s.Arrive != entered+LaneDays(c, li, 1, Base) {
		t.Errorf("arrive %d, entered %d + %d", s.Arrive, entered, LaneDays(c, li, 1, Base))
	}
	out := runEmpire(t, e, 100, func([]Event) bool { return s.Lane < 0 })
	if out != s.Arrive || s.Star != far {
		t.Errorf("out on day %d at star %d, want %d at %d", out, s.Star, s.Arrive, far)
	}
	_, end, _ := laneEnd(c, far, home.Star)
	if s.Pos != c.Stars[far].System.Lanes[end].Pos {
		t.Errorf("pos %v", s.Pos)
	}

	// A Colonise order founds in EndDay.
	if len(c.Stars[far].System.Planets) == 0 {
		t.Fatal("the far star has no planet")
	}
	s.Order = Order{Kind: GoToPlanet, Planet: 0}
	runEmpire(t, e, 100, func([]Event) bool { return s.Orbit == 0 })
	s.Order = Order{Kind: Colonise}
	var founded bool
	for _, x := range e.EndDay() {
		founded = founded || x.Kind == Built && x.Item == ColonyBase && x.Star == far && x.Planet == 0
	}
	if !founded || !e.Inhabited(far, 0) || e.ControlledSystems() != 2 || s.Order.Kind != NoOrder {
		t.Errorf("Colonise: event %v, inhabited %v, systems %d", founded, e.Inhabited(far, 0), e.ControlledSystems())
	}
}

func TestOrder(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	s := e.Spawn(laneShip, c.Home, 0)
	if err := e.Order(s, Order{Kind: Colonise}); err == nil || !strings.Contains(err.Error(), "already inhabited") {
		t.Errorf("colonise the home world: %v", err)
	}
	if s.Order.Kind != NoOrder {
		t.Errorf("a refused order was set: %+v", s.Order)
	}
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: -1}); err == nil || s.Order.Kind != NoOrder {
		t.Errorf("Check's refusal not passed on: %v, %+v", err, s.Order)
	}
	far := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	for _, o := range []Order{{Kind: GoToPlanet, Planet: 1}, {Kind: GoToStar, Star: far}, {}} {
		if err := e.Order(s, o); err != nil || s.Order != o {
			t.Errorf("%+v: %v, order %+v", o, err, s.Order)
		}
	}
	other := e.Spawn(laneShip, c.Home, 1)
	if err := e.Order(other, Order{Kind: Colonise}); err != nil || other.Order.Kind != Colonise {
		t.Errorf("colonise planet 1: %v", err)
	}
}

// Every event says where it happened: a colony's on its planet (Built,
// Grew), a discovery nowhere (−1), a founding on the founded planet.
func TestEventPlaces(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	home := e.Colonies[0]
	// A lab and a plot so that research and growth run beside the build.
	for _, it := range []Item{Laboratory, Agriplot} {
		at, ok := home.Planet.BestCell(it, Base)
		if !ok || home.Planet.Build(at, it) != nil {
			t.Fatalf("no square for %v", it)
		}
	}
	home.Pop = 4 // the crews and one for the build, under Capacity
	at, ok := home.Planet.BestCell(Factory, Base)
	if !ok {
		t.Fatal("no square for a Factory")
	}
	if err := home.Start(e.Research, Factory, at); err != nil {
		t.Fatal(err)
	}
	if err := e.Research.Study(e.Research.Available()[0]); err != nil {
		t.Fatal(err)
	}
	seen := map[EventKind]bool{}
	runEmpire(t, e, 1000, func(ev []Event) bool {
		for _, x := range ev {
			seen[x.Kind] = true
			want := [2]int{e.Cluster.Home, 0}
			if x.Kind == Discovered {
				want = [2]int{-1, -1}
			}
			if got := [2]int{x.Star, x.Planet}; got != want {
				t.Errorf("%v %q at %v, want %v", x.Kind, x.Text, got, want)
			}
		}
		return seen[Built] && seen[Discovered] && seen[Grew]
	})

	pl := -1
	for i := 1; i < len(e.Cluster.Stars[e.Cluster.Home].System.Planets); i++ {
		if _, ok := e.Planet(e.Cluster.Home, i).BestCell(ColonyBase, Base); ok {
			pl = i
			break
		}
	}
	if pl < 0 {
		t.Skip("seed 1's home system has no colonisable planet")
	}
	s := e.Spawn(laneShip, e.Cluster.Home, pl)
	if err := e.Order(s, Order{Kind: Colonise}); err != nil {
		t.Fatal(err)
	}
	founded := false
	for _, x := range e.EndDay() {
		if x.Kind == Built && x.Item == ColonyBase {
			founded = true
			if x.Star != e.Cluster.Home || x.Planet != pl {
				t.Errorf("founding %q at (%d, %d), want (%d, %d)", x.Text, x.Star, x.Planet, e.Cluster.Home, pl)
			}
		}
	}
	if !founded {
		t.Error("no founding event")
	}
}

// shipResearch learns the Small hull's tech, the Tonklin Motor's and
// the Proton Shaver's, with their prerequisites.
func shipResearch(r *Research) {
	learn(r, Small.Tech())
	learn(r, Gizmos[TonklinMotor].ResearchReq)
	learn(r, Gizmos[ProtonShaver].ResearchReq)
}

// AddTemplate keeps named templates that fit their hull and hold only
// discovered material, valid or not yet (no engine is Valid's business):
// a same name replaces in place, an unnamed or overfull one is refused,
// and the stored parts do not alias the caller's slice.
func TestAddTemplate(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	shipResearch(e.Research)
	a := Template{Name: "Scout", Hull: Small, Parts: []Part{{Best: true, Cat: Generator}, {Best: true, Cat: Drive}}}
	b := Template{Name: "Brick", Hull: Small, Parts: []Part{{Gizmo: ProtonShaver, Cat: Generator}}} // no engine
	for _, tp := range []Template{a, b} {
		if err := e.AddTemplate(tp); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.Templates) != 2 || e.Templates[0].Name != "Scout" || e.Templates[1].Name != "Brick" {
		t.Fatalf("templates %v", e.Templates)
	}
	a.Parts[1] = Part{Gizmo: Colonizer} // the caller edits its slice after saving
	if !e.Templates[0].Parts[1].Best {
		t.Error("the saved parts alias the caller's slice")
	}

	learn(e.Research, Medium.Tech())
	scout2 := Template{Name: "Scout", Hull: Medium, Parts: []Part{{Gizmo: TonklinMotor, Cat: Drive}}}
	if err := e.AddTemplate(scout2); err != nil {
		t.Fatal(err)
	}
	if len(e.Templates) != 2 || e.Templates[0].Hull != Medium || len(e.Templates[0].Parts) != 1 {
		t.Errorf("replace: %v", e.Templates)
	}

	unnamed := b
	unnamed.Name = ""
	over := Template{Name: "Over", Hull: Small, Parts: make([]Part, Small.Slots()+1)}
	for _, bad := range []Template{unnamed, over} {
		if err := e.AddTemplate(bad); err == nil {
			t.Errorf("%q saved", bad.Name)
		}
	}
	if len(e.Templates) != 2 {
		t.Errorf("refusals changed the list: %v", e.Templates)
	}
}

// Undiscovered material is refused, the list unchanged: a hull, a fixed
// part research does not allow, a Best part whose category has no
// allowed gizmo (Realise's ok false); the same template is accepted once
// research allows it.
func TestAddTemplateDiscovered(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	r := e.Research
	shipResearch(r)
	drive := Part{Best: true, Cat: Drive}
	cases := []struct {
		name  string
		tp    Template
		allow func() // what makes it acceptable
		known func() bool
	}{
		{"hull", Template{Hull: Enormous, Parts: []Part{drive}},
			func() { learn(r, Enormous.Tech()) }, func() bool { return r.Known[Enormous.Tech()] }},
		{"fixed", Template{Hull: Small, Parts: []Part{drive, {Gizmo: Nanoshell, Cat: Shield}}},
			func() { learn(r, Gizmos[Nanoshell].ResearchReq) }, func() bool { return r.AllowsGizmo(Nanoshell) }},
		{"best", Template{Hull: Small, Parts: []Part{drive, {Best: true, Cat: Weapon}}},
			func() { learn(r, Gizmos[MassBarrageGun].ResearchReq) }, func() bool { _, ok := best(r, Weapon); return ok }},
	}
	for _, c := range cases {
		c.tp.Name = c.name
		if c.known() {
			t.Fatalf("%s: fixture research already allows it", c.name)
		}
		if _, ok := c.tp.Realise(r); c.name == "best" && ok {
			t.Fatalf("best: Realise ok")
		}
		if err := e.AddTemplate(c.tp); err == nil || len(e.Templates) != 0 {
			t.Errorf("%s: saved (%v), templates %v", c.name, err, e.Templates)
		}
	}
	for _, c := range cases {
		c.tp.Name = c.name
		c.allow()
		if !c.known() {
			t.Fatalf("%s: allow() did not", c.name)
		}
		if err := e.AddTemplate(c.tp); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	if len(e.Templates) != len(cases) {
		t.Errorf("templates %v", e.Templates)
	}
}

// An empire starts with no template: they are the player's own.
func TestNewEmpireTemplates(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	for _, e := range g.Empires {
		if e.Templates != nil {
			t.Errorf("empire %d: templates %v", e.Index, e.Templates)
		}
	}
}

// Relink points each colony at its planet in the cluster and refuses an
// index outside it.
func TestRelink(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Colonies[0]
	c.Planet = nil
	if err := e.Relink(); err != nil || c.Planet != e.Planet(c.Star, c.PlanetIdx) {
		t.Fatalf("relink: %v", err)
	}
	c.PlanetIdx = len(e.Cluster.Stars[c.Star].System.Planets)
	if e.Relink() == nil {
		t.Error("a planet index past the system relinked")
	}
	c.PlanetIdx, c.Star = 0, len(e.Cluster.Stars)
	if e.Relink() == nil {
		t.Error("a star index past the cluster relinked")
	}
}

// A system another empire has a colony in is controlled by nobody: it
// leaves both fleet caps (help.txt: "the only colonies there").
func TestControlledSystemsShared(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	a, b := g.Empires[0], g.Empires[1]
	home := a.Colonies[0].Star
	if len(g.Cluster.Stars[home].System.Planets) < 2 {
		t.Skip("home system too small to share")
	}
	if a.ControlledSystems() != 1 || b.ControlledSystems() != 1 {
		t.Fatalf("at start: %d and %d systems", a.ControlledSystems(), b.ControlledSystems())
	}
	g.Cluster.Stars[home].System.Planets[(a.Colonies[0].PlanetIdx+1)%2].Owner = b.Index
	if a.ControlledSystems() != 0 {
		t.Errorf("shared home: %d systems, want 0", a.ControlledSystems())
	}
}

// The close reports an Arrived per ship reaching an orbit or leaving a
// lane at its star (offline arrivals are news); a
// Hop on the day's display reports nothing (no close), a point reached
// neither.
func TestArrivedEvents(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	c := e.Cluster
	home := e.Colonies[0]
	other := (home.PlanetIdx + 1) % len(c.Stars[home.Star].System.Planets)
	arrivals := func(evs []Event) []Event {
		var out []Event
		for _, x := range evs {
			if x.Kind == Arrived {
				out = append(out, x)
			}
		}
		return out
	}
	// Orbit, offline: ordered, the close moves it there.
	s := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	if err := e.Order(s, Order{Kind: GoToPlanet, Planet: other}); err != nil {
		t.Fatal(err)
	}
	var got []Event
	for d := 0; s.Orbit != other; d++ {
		if d > 30 {
			t.Fatalf("not in orbit of %d after %d days: %+v", other, d, s)
		}
		got = append(got, arrivals(e.EndDay())...)
	}
	want := []Event{{Kind: Arrived, Item: NoItem, Tech: NoTech, Star: home.Star, Planet: other, Other: -1,
		Text: s.Name + " in orbit of " + e.Planet(home.Star, other).Name}}
	if !slices.Equal(got, want) {
		t.Errorf("orbit: %+v, want %+v", got, want)
	}
	// Out of a lane at the far star.
	far := c.Other(c.Stars[home.Star].Lanes[0], home.Star)
	if err := e.Order(s, Order{Kind: GoToStar, Star: far}); err != nil {
		t.Fatal(err)
	}
	got = nil
	for d := 0; s.Star != far || s.Lane >= 0; d++ {
		if d > 200 {
			t.Fatalf("not at %d after %d days", far, d)
		}
		got = append(got, arrivals(e.EndDay())...)
	}
	want = []Event{{Kind: Arrived, Item: NoItem, Tech: NoTech, Star: far, Planet: -1, Other: -1, Text: s.Name + " arrived at " + c.Stars[far].Name}}
	if !slices.Equal(got, want) {
		t.Errorf("lane: %+v, want %+v", got, want)
	}
	// On the display: Hops into an orbit, then the close says nothing.
	s2 := e.Spawn(laneShip, home.Star, home.PlanetIdx)
	pp := e.Planet(home.Star, other).Pos
	s2.Orbit, s2.Pos = -1, Vec3{X: pp.X + 0.1, Y: pp.Y, Z: pp.Z} // a hop from it
	if err := e.Order(s2, Order{Kind: GoToPlanet, Planet: other}); err != nil {
		t.Fatal(err)
	}
	for s2.Orbit != other {
		moved, _ := s2.Hop(c, e.Day, nil, nil)
		if !moved {
			t.Fatalf("the display's hops stop short: %+v", s2)
		}
	}
	if x := arrivals(e.EndDay()); len(x) != 0 {
		t.Errorf("an orbit reached on the display reported: %+v", x)
	}
}

// A ship named in the build (BuildShipNamed) bears that name as it is at
// launch, no numeral added; a name another ship or ship project bears is
// refused; ShipNameFor proposes the name Spawn would give, and Spawn
// skips a name the player took.
func TestBuildShipNamed(t *testing.T) {
	e, home := shipEmpire(t)
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	want := e.ShipNameFor(d.Name)
	if got := shipName(d.Name, 1); got != want {
		t.Errorf("proposal %q, want %q", got, want)
	}
	if err := e.BuildShipNamed(home, d, " \t "); err == nil || home.Project != nil {
		t.Errorf("a blank name: %v, project %v", err, home.Project)
	}
	if err := e.AddTemplate(Template{Name: "  ", Hull: d.Hull}); err == nil {
		t.Error("a blank template name accepted")
	}
	if err := e.BuildShipNamed(home, d, want); err != nil {
		t.Fatal(err)
	}
	if got := e.ShipNameFor(d.Name); got != shipName(d.Name, 2) {
		t.Errorf("proposal with a ship project: %q", got)
	}
	if err := e.BuildShipNamed(home, d, want); err == nil || !strings.Contains(err.Error(), "named") {
		t.Errorf("a name the project bears: %v", err)
	}
	for home.Project != nil {
		e.EndDay()
	}
	s := e.Ships[len(e.Ships)-1]
	if s.Name != want || s.Design.Name != d.Name {
		t.Fatalf("launched %q of %q, want %q of %q", s.Name, s.Design.Name, want, d.Name)
	}
	// A plain name, then the auto name it would collide with skipped.
	if err := e.BuildShipNamed(home, d, "Seeder"); err != nil {
		t.Fatal(err)
	}
	for home.Project != nil {
		e.EndDay()
	}
	if s := e.Ships[len(e.Ships)-1]; s.Name != "Seeder" {
		t.Errorf("named %q, want Seeder", s.Name)
	}
	taken := e.ShipNameFor(d.Name) // the player took the next numeral
	e.Ships[len(e.Ships)-1].Name = taken
	if e.ShipNameFor(d.Name) == taken {
		t.Error("proposed a name taken")
	}
	if s := e.Spawn(d, home.Star, home.PlanetIdx); s.Name == taken {
		t.Errorf("Spawn reused %q", s.Name)
	}
	if err := e.BuildShipNamed(home, d, "Seeder"); err == nil {
		t.Error("a ship's name accepted again")
	}
}

// A ship's default name is its class (design name) with the first
// numeral no ship or ship project bears, per class: a gap left by a
// loss or a custom name is taken first; past
// XXXIX the numeral is decimal.
func TestShipNumbers(t *testing.T) {
	e, home := shipEmpire(t)
	d, ok := ColonyShip(e.Research, false)
	if !ok {
		t.Fatal("no colony ship")
	}
	other := d
	other.Name = "Other"
	e.Ships = nil
	if got := e.ShipNameFor(d.Name); got != shipName(d.Name, 1) {
		t.Errorf("first proposal %q", got)
	}
	e.ShipNameFor(d.Name) // proposed only: no ship takes it
	one := e.Spawn(d, home.Star, home.PlanetIdx)
	two := e.Spawn(d, home.Star, home.PlanetIdx)
	if one.Name != shipName(d.Name, 1) || two.Name != shipName(d.Name, 2) {
		t.Errorf("first ships %q, %q", one.Name, two.Name)
	}
	if got := e.Spawn(other, home.Star, home.PlanetIdx).Name; got != shipName("Other", 1) {
		t.Errorf("another class's first ship %q", got)
	}
	if got := e.ShipNameFor(d.Name); got != shipName(d.Name, 3) {
		t.Errorf("after I and II: %q, want III", got)
	}
	e.Ships = slices.DeleteFunc(e.Ships, func(s *Starship) bool { return s == one }) // I lost
	if got := e.ShipNameFor(d.Name); got != shipName(d.Name, 1) {
		t.Errorf("I lost: %q, want I again", got)
	}
	e.Ships = nil
	if err := e.BuildShipNamed(home, d, shipName(d.Name, 1)); err != nil { // a project bears I
		t.Fatal(err)
	}
	if got := e.ShipNameFor(d.Name); got != shipName(d.Name, 2) {
		t.Errorf("I in the yard: %q, want II", got)
	}
	if got := shipName(d.Name, 40); got != d.Name+" 40" {
		t.Errorf("decimal numeral: %q", got)
	}
}
