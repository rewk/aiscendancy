package game

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// knowing returns a Research knowing ts and nothing else.
func knowing(ts ...Tech) *Research {
	r := NewResearch()
	for _, t := range ts {
		r.Grant(t)
	}
	return r
}

// allKnown knows the whole tree.
func allKnown() *Research {
	r := NewResearch()
	for t := range NumTechs {
		r.Grant(t)
	}
	return r
}

// The early techs a colony ship needs: hull and generator, engine, Colonizer.
var earlyShipTechs = []Tech{OrbitalStructures, InterplanetaryExploration, TonklinDiary, Xenobiology, EnvironmentalEncapsulation}

func TestHulls(t *testing.T) {
	want := []struct {
		slots, cost int
		tech        Tech
		name        string
	}{
		{5, 70, InterplanetaryExploration, "Small"},
		{10, 140, InterplanetaryExploration, "Medium"},
		{15, 280, AdvancedExploration, "Large"},
		{25, 500, LargeScaleConstruction, "Enormous"},
	}
	for h := range NumHulls {
		w := want[h]
		if h.Slots() != w.slots || h.Cost() != w.cost || h.Tech() != w.tech || h.String() != w.name {
			t.Errorf("%v: %d slots, cost %d, %v; want %+v", h, h.Slots(), h.Cost(), h.Tech(), w)
		}
	}
}

// The 2005 walkthrough's first colony ship. Its measured cost is 207; ours
// is the HullCost ASSUMPTION plus gizmos.txt's parts, 205: close.
func TestDesignCostGuideShip(t *testing.T) {
	d := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Colonizer, Colonizer, Colonizer}}
	want := HullCost[Small] + Gizmos[ProtonShaver].Industry + Gizmos[TonklinMotor].Industry + 3*Gizmos[Colonizer].Industry
	if d.Cost() != want || want != 205 {
		t.Errorf("cost %d, want %d (=205)", d.Cost(), want)
	}
	if err := d.Valid(knowing(earlyShipTechs...)); err != nil {
		t.Errorf("the guide's ship refused: %v", err)
	}
	if d.Colonizers() != 3 || d.LaneDrives() != 0 {
		t.Errorf("colonizers %d, lane drives %d", d.Colonizers(), d.LaneDrives())
	}
}

func TestDesignValid(t *testing.T) {
	all := allKnown()
	ok := Design{Name: "ok", Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver}}
	if err := ok.Valid(all); err != nil {
		t.Fatalf("baseline refused: %v", err)
	}
	for _, c := range []struct {
		d    Design
		r    *Research
		want string
	}{
		{Design{Name: "crowded", Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver, Colonizer, Colonizer, Colonizer, Colonizer}}, all, "slots"},
		{Design{Name: "forbidden", Hull: Small, Parts: []Gizmo{IonBanger, ProtonShaver}}, knowing(earlyShipTechs...), "Ion Banger needs Advanced Chemistry"},
		{Design{Name: "hull", Hull: Large, Parts: []Gizmo{TonklinMotor, ProtonShaver}}, knowing(earlyShipTechs...), "Large hull needs"},
		{Design{Name: "deficit", Hull: Small, Parts: []Gizmo{TonklinMotor, TonklinMotor, TonklinMotor, ProtonShaver}}, all, "power 2 for a demand of 3"},
		{Design{Name: "unpowered", Hull: Small, Parts: []Gizmo{TonklinMotor}}, all, "power 0 for a demand of 1"},
		{Design{Name: "still", Hull: Small, Parts: []Gizmo{ProtonShaver, Colonizer}}, all, "no engine"},
	} {
		err := c.d.Valid(c.r)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.d.Name, err, c.want)
		}
	}
}

func TestColonyShip(t *testing.T) {
	if _, ok := ColonyShip(knowing(OrbitalStructures, InterplanetaryExploration, TonklinDiary), true); ok {
		t.Error("a colony ship without Environmental Encapsulation")
	}
	r := knowing(earlyShipTechs...)
	// Interplanetary Exploration allows Small and Medium: the smaller.
	d, ok := ColonyShip(r, true) // lane drive wanted but not allowed yet
	want := []Gizmo{TonklinMotor, ProtonShaver}
	for range 3 {
		want = append(want, Colonizer)
	}
	if !ok || d.Hull != Small || !slices.Equal(d.Parts, want) {
		t.Errorf("early: %v %v, want Small %v", ok, d, want)
	}
	r.Grant(SpacetimeSurfing)
	d, ok = ColonyShip(r, true)
	want = append([]Gizmo{TonklinMotor, ProtonShaver, StarLaneDrive}, want[3:]...)
	if !ok || !slices.Equal(d.Parts, want) {
		t.Errorf("with lanes: %v", d)
	}
	if err := d.Valid(r); err != nil {
		t.Errorf("ColonyShip invalid: %v", err)
	}
	if got := d.String(); got != "Small: Tonklin Motor, Proton Shaver, Star Lane Drive, Colonizer x2" {
		t.Errorf("String %q", got)
	}
	r.Grant(AdvancedChemistry)
	if d, _ = ColonyShip(r, false); d.Parts[0] != IonBanger || d.Colonizers() != 3 {
		t.Errorf("with Advanced Chemistry: %v", d)
	}
	// Every hull: the design is valid and fills its slots.
	for _, extra := range [][]Tech{nil, {Hyperlogic, AdvancedExploration}, {LargeScaleConstruction}} {
		for _, x := range extra {
			r.Grant(x)
		}
		d, ok := ColonyShip(r, true)
		if !ok || len(d.Parts) != d.Hull.Slots() || d.Valid(r) != nil {
			t.Errorf("%v: ok %v, %d parts, %v", d, ok, len(d.Parts), d.Valid(r))
		}
	}
	// The smallest hull is still taken: bigger ones are for a later policy.
	if d, _ := ColonyShip(r, true); d.Hull != Small {
		t.Errorf("hull %v, want Small", d.Hull)
	}
}

// Reach, MoveCost and PowerPerDay sum the drives' Level and Power and the
// generators' Power (gizmos.txt's table).
func TestReachMoveCostPower(t *testing.T) {
	g := func(x Gizmo) *GizmoInfo { return &Gizmos[x] }
	d := Design{Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver}}
	if v := d.Reach(); math.Abs(v-float64(g(TonklinMotor).Level)*SugarCube) > 1e-12 || math.Abs(v-0.2) > 1e-12 {
		t.Errorf("Tonklin reach %v", v)
	}
	if d.MoveCost() != g(TonklinMotor).Power || d.PowerPerDay() != g(ProtonShaver).Power {
		t.Errorf("Tonklin: cost %d, power %d", d.MoveCost(), d.PowerPerDay())
	}
	d.Parts = append(d.Parts, TonklinMotor, IonBanger)
	lv := 2*g(TonklinMotor).Level + g(IonBanger).Level
	if v := d.Reach(); math.Abs(v-float64(lv)*SugarCube) > 1e-12 || math.Abs(v-0.8) > 1e-12 {
		t.Errorf("2 Tonklin + Ion Banger reach %v", v)
	}
	if c := d.MoveCost(); c != 2*g(TonklinMotor).Power+g(IonBanger).Power {
		t.Errorf("2 Tonklin + Ion Banger cost %d", c)
	}
	if p := d.PowerPerDay(); p != g(ProtonShaver).Power {
		t.Errorf("power %d", p)
	}
	still := Design{Hull: Small, Parts: []Gizmo{ProtonShaver, StarLaneDrive, Colonizer}}
	if still.Reach() != 0 || still.MoveCost() != 0 {
		t.Errorf("no engine: reach %v cost %d", still.Reach(), still.MoveCost())
	}
}

// moveDays is the days Step takes over dist (each Step refills first):
// the moves it needs (at least one), PowerPerDay/MoveCost of them a day.
func moveDays(d Design, dist float64) int {
	hops := max(1, int(math.Ceil((dist-1e-9)/d.Reach())))
	perDay := d.PowerPerDay() / d.MoveCost()
	return (hops + perDay - 1) / perDay
}

// twoStars: stars 0 and 1, 0.3 apart, one lane; system 0 has a planet at
// radius 0.5 on +X, system 1 none. Each system's lane endpoint is listed.
func twoStars() *Cluster {
	c := &Cluster{
		Stars: []Star{{Name: "A", Pos: Vec3{}}, {Name: "B", Pos: Vec3{0.3, 0, 0}}},
		Lanes: []StarLane{{A: 0, B: 1}},
	}
	c.Stars[0].Lanes, c.Stars[1].Lanes = []int{0}, []int{0}
	pa := NewPlanet(1, "A I", Congenial, 3)
	pa.Pos = Vec3{0.5, 0, 0}
	c.Stars[0].System = &StarSystem{Name: "A", Planets: []*Planet{pa}, Lanes: []Lane{{Pos: Vec3{-1, 0, 0}, To: 1}}}
	c.Stars[1].System = &StarSystem{Name: "B", Lanes: []Lane{{Pos: Vec3{0, 0.5, 1.1}, To: 0}}}
	return c
}

func TestStepToPlanet(t *testing.T) {
	c := twoStars()
	s := &Starship{Design: Design{Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver}}, Orbit: -1, Lane: -1,
		Order: Order{Kind: GoToPlanet, Planet: 0}} // Power 0: Step refills first
	n := moveDays(s.Design, 0.5)
	for day := 1; day <= n; day++ {
		arrived := s.Step(c, day, nil, nil)
		if arrived != (day == n) {
			t.Fatalf("day %d: arrived %v at %v", day, arrived, s.Pos)
		}
	}
	if s.Orbit != 0 || s.Pos != c.Stars[0].System.Planets[0].Pos || s.Order.Kind != NoOrder {
		t.Errorf("after: orbit %d pos %v order %v", s.Orbit, s.Pos, s.Order)
	}
}

func TestStepThroughLane(t *testing.T) {
	c := twoStars()
	d := Design{Hull: Small, Parts: []Gizmo{TonklinMotor, ProtonShaver, StarLaneDrive}}
	// At 0.9 from the endpoint (-1,0,0).
	s := &Starship{Design: d, Pos: Vec3{-0.1, 0, 0}, Orbit: -1, Lane: -1, Order: Order{Kind: GoToStar, Star: 1}}
	n := moveDays(d, 0.9)
	days := LaneDays(c, 0, 1, Base)
	if want := int(math.Ceil(0.3/LaneSpeed - 1e-9)); days != want { // a blue lane 0.3 long
		t.Fatalf("lane days %d, want %d", days, want)
	}
	day := 0
	for s.Lane < 0 {
		day++
		if s.Step(c, day, nil, nil) || day > 10 {
			t.Fatalf("day %d: arrived before the lane, or never entered", day)
		}
	}
	if day != n || s.Arrive != n+days || s.Pos != c.Stars[0].System.Lanes[0].Pos {
		t.Fatalf("entered on day %d, arrive %d, pos %v", day, s.Arrive, s.Pos)
	}
	for day++; day < s.Arrive; day++ {
		s.Power = 0
		if s.Step(c, day, nil, nil) || s.Power != d.PowerPerDay() {
			t.Fatalf("day %d: out of the lane, or not refilled (power %d)", day, s.Power)
		}
	}
	if !s.Step(c, day, nil, nil) || s.Star != 1 || s.Lane != -1 || s.Orbit != -1 || s.Pos != c.Stars[1].System.Lanes[0].Pos {
		t.Errorf("day %d: star %d lane %d orbit %d pos %v", day, s.Star, s.Lane, s.Orbit, s.Pos)
	}
	if s.Order.Kind != NoOrder {
		t.Errorf("order %v kept", s.Order)
	}
}

func TestLaneDaysScaleWithDrives(t *testing.T) {
	c := twoStars()
	if a, b := LaneDays(c, 0, 1, Base), LaneDays(c, 0, 2, Base); a != 2*b {
		t.Errorf("strength 1: %d days, strength 2: %d", a, b)
	}
	// A ship without a lane drive drops a GoToStar order where it is.
	s := &Starship{Design: Design{Hull: Small, Parts: []Gizmo{TonklinMotor}}, Orbit: -1, Lane: -1, Order: Order{Kind: GoToStar, Star: 1}}
	if s.Step(c, 1, nil, nil) || s.Order.Kind != NoOrder || s.Lane != -1 || s.Pos != (Vec3{}) {
		t.Errorf("no drive: %+v", s)
	}
}

// laneShip is the view's -ships fixture: a Small hull with a Proton
// Shaver, a Tonklin Motor, a Star Lane Drive and two Colonizers.
var laneShip = Design{Name: "Colony Ship", Hull: Small,
	Parts: []Gizmo{ProtonShaver, TonklinMotor, StarLaneDrive, Colonizer, Colonizer}}

// notAdjacent is a star other than star and not one lane from it.
func notAdjacent(t *testing.T, c *Cluster, star int) int {
	t.Helper()
	for k := range c.Stars {
		if _, _, ok := laneEnd(c, star, k); !ok && k != star {
			return k
		}
	}
	t.Fatal("every star is adjacent")
	return -1
}

func TestCheck(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	s := e.Spawn(laneShip, c.Home, 0)
	far := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	n := len(c.Stars[c.Home].System.Planets)
	ok := []Order{{}, {Kind: GoToPlanet, Planet: 0}, {Kind: GoToPlanet, Planet: n - 1}, {Kind: GoToStar, Star: far}, {Kind: Colonise}}
	for _, o := range ok {
		if err := s.Check(c, o); err != nil {
			t.Errorf("%+v refused: %v", o, err)
		}
	}
	stranger := notAdjacent(t, c, c.Home)
	noDrive := *s
	noDrive.Design = Design{Hull: Small, Parts: []Gizmo{ProtonShaver, TonklinMotor, Colonizer}}
	inSpace, empty, inLane := *s, *s, *s
	inSpace.Orbit = -1
	empty.Cargo = 0
	inLane.Lane, inLane.Arrive = c.Stars[c.Home].Lanes[0], 5
	still, unpowered, heavy := *s, *s, *s
	still.Design = Design{Hull: Small, Parts: []Gizmo{ProtonShaver, StarLaneDrive, Colonizer}}
	unpowered.Design = Design{Hull: Small, Parts: []Gizmo{TonklinMotor, StarLaneDrive, Colonizer}}
	heavy.Design = Design{Hull: Small, Parts: []Gizmo{GravitonProjector, ProtonShaver}} // cost 3 > power 2
	if heavy.Design.MoveCost() <= heavy.Design.PowerPerDay() {
		t.Fatalf("heavy: cost %d, power %d", heavy.Design.MoveCost(), heavy.Design.PowerPerDay())
	}
	spent := *s
	spent.Power = 0 // nothing left today is not a refusal
	if err := spent.Check(c, Order{Kind: GoToPlanet, Planet: n - 1}); err != nil {
		t.Errorf("no power left today: %v", err)
	}
	for _, x := range []struct {
		s    *Starship
		o    Order
		want string
	}{
		{s, Order{Kind: GoToPlanet, Planet: -1}, "no such planet"},
		{s, Order{Kind: GoToPlanet, Planet: n}, "no such planet"},
		{s, Order{Kind: GoToStar, Star: stranger}, c.Stars[stranger].Name + " is not adjacent"},
		{s, Order{Kind: GoToStar, Star: c.Home}, " is not adjacent"},
		{s, Order{Kind: GoToStar, Star: len(c.Stars)}, "no star"},
		{&noDrive, Order{Kind: GoToStar, Star: far}, "no star lane drive"},
		{&inSpace, Order{Kind: Colonise}, "not in orbit"},
		{&empty, Order{Kind: Colonise}, "no colonizer"},
		{&inLane, Order{Kind: GoToPlanet, Planet: 0}, "in a star lane"},
		{&inLane, Order{Kind: GoToStar, Star: far}, "in a star lane"},
		{&inLane, Order{Kind: Colonise}, "in a star lane"},
		{&still, Order{Kind: GoToPlanet, Planet: 0}, "no engine"},
		{&still, Order{Kind: GoToPoint}, "no engine"},
		{&unpowered, Order{Kind: GoToStar, Star: far}, "not enough power to move"},
		{&heavy, Order{Kind: GoToPoint}, "not enough power to move"},
	} {
		if err := x.s.Check(c, x.o); err == nil || !strings.Contains(err.Error(), x.want) {
			t.Errorf("%+v: %v, want %q", x.o, err, x.want)
		}
	}
	if err := inLane.Check(c, Order{}); err != nil {
		t.Errorf("cancel in a lane: %v", err)
	}
}

// hopAll is the system display after an order on day0: Hop after Hop on
// day0 while the ship moves and has not arrived (see systemScreen.hop's
// chain). It reports an arrival in space
// (a lane entry is not one).
func hopAll(c *Cluster, s *Starship, day0 int) bool {
	for {
		moved, arrived := s.Hop(c, day0, nil, nil)
		if arrived {
			return s.Lane < 0
		}
		if !moved {
			return false
		}
	}
}

// stepUntil steps s from day0+1 until it reports an arrival, checking on
// the way that ETA counts down by one a day, in space as in a lane. It
// returns the arrival day.
func stepUntil(t *testing.T, c *Cluster, s *Starship, day0, eta int) int {
	t.Helper()
	for day := day0 + 1; day <= day0+1000; day++ {
		if s.Step(c, day, nil, nil) {
			return day
		}
		want := day0 + eta - day
		if got, ok := s.ETA(c, day); !ok || got != want {
			t.Fatalf("day %d: ETA %d %v, want %d", day, got, ok, want)
		}
	}
	t.Fatal("never arrived")
	return -1
}

// ETA is Step's count: from orbit, from deep space and from the lane
// endpoint itself, for every planet and every lane of the home system.
func TestETAMatchesStep(t *testing.T) {
	const day0 = 10
	for _, seed := range []uint64{1, 2, 3} {
		e := NewEmpire(seed, Snovemdomas, Average)
		c := e.Cluster
		sys := c.Stars[c.Home].System
		var starts []func(s *Starship, target Vec3)
		starts = append(starts,
			func(s *Starship, target Vec3) {}, // in orbit of planet 0
			func(s *Starship, target Vec3) { // the far side of the disc
				s.Orbit, s.Pos = -1, Vec3{X: -target.X, Y: -target.Y, Z: -target.Z}
			},
			func(s *Starship, target Vec3) { s.Orbit, s.Pos = -1, target }, // on the target
		)
		var orders []Order
		for k := range sys.Planets {
			orders = append(orders, Order{Kind: GoToPlanet, Planet: k})
		}
		for _, li := range c.Stars[c.Home].Lanes {
			orders = append(orders, Order{Kind: GoToStar, Star: c.Other(li, c.Home)})
		}
		orders = append(orders,
			Order{Kind: GoToPoint, Pos: Vec3{X: 0.7, Y: 0.3, Z: -0.4}},
			Order{Kind: GoToPoint, Pos: Vec3{X: -1, Y: -0.5, Z: 0.6}},
			Order{Kind: GoToPoint, Pos: sys.Planets[0].Pos}) // where the orbiting ship stands
		for i, start := range starts {
			for _, o := range orders {
				s := e.Spawn(laneShip, c.Home, 0)
				target := sys.Planets[0].Pos
				switch o.Kind {
				case GoToPlanet:
					target = sys.Planets[o.Planet].Pos
				case GoToPoint:
					target = o.Pos
				default:
					_, end, _ := laneEnd(c, c.Home, o.Star)
					target = sys.Lanes[end].Pos
				}
				start(s, target)
				if err := e.Order(s, o); err != nil {
					t.Fatalf("seed %d start %d %+v: %v", seed, i, o, err)
				}
				eta0, ok := s.ETA(c, day0)
				if !ok {
					t.Fatalf("seed %d start %d %+v: no ETA", seed, i, o)
				}
				// The view hops at once; ETA 0 is exactly "arrived in them".
				if hopAll(c, s, day0) != (eta0 == 0) {
					t.Errorf("seed %d start %d %+v: ETA %d, arrived in the hops %v", seed, i, o, eta0, eta0 != 0)
				}
				if eta0 == 0 {
					continue
				}
				eta, ok := s.ETA(c, day0)
				if !ok || eta != eta0 {
					t.Errorf("seed %d start %d %+v: ETA %d after the hops, %d before", seed, i, o, eta, eta0)
				}
				if got := stepUntil(t, c, s, day0, eta); got != day0+eta {
					t.Errorf("seed %d start %d %+v: arrived day %d, ETA said %d", seed, i, o, got, day0+eta)
				}
				if o.Kind == GoToStar && s.Star != o.Star || o.Kind == GoToPlanet && s.Orbit != o.Planet ||
					o.Kind == GoToPoint && (s.Pos != o.Pos || s.Orbit != -1 || s.Star != c.Home) {
					t.Errorf("seed %d start %d %+v: ended at star %d orbit %d", seed, i, o, s.Star, s.Orbit)
				}
			}
		}
	}
}

func TestETANothingToWait(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	s := e.Spawn(laneShip, e.Cluster.Home, 0)
	for _, o := range []Order{{}, {Kind: Colonise}, {Kind: GoToPlanet, Planet: 99}} {
		s.Order = o // set raw: Order would refuse the last one
		if d, ok := s.ETA(e.Cluster, 0); ok {
			t.Errorf("%+v: ETA %d", o, d)
		}
	}
	still := e.Spawn(Design{Hull: Small, Parts: []Gizmo{ProtonShaver}}, e.Cluster.Home, 0)
	still.Orbit = -1
	still.Order = Order{Kind: GoToPlanet, Planet: 0}
	if d, ok := still.ETA(e.Cluster, 0); ok {
		t.Errorf("no engine: ETA %d", d)
	}
}

// GoToPoint leaves the orbit on its first day, stops on the point in
// space, takes a day even from the point itself, and is refused past
// SystemReach (by Check, hence by Empire.Order).
func TestGoToPoint(t *testing.T) {
	e := NewEmpire(6, Snovemdomas, Average)
	c := e.Cluster
	s := e.Spawn(laneShip, c.Home, 0)
	p := Vec3{X: 0.5, Y: 0.2, Z: -0.6}
	if err := e.Order(s, Order{Kind: GoToPoint, Pos: p}); err != nil {
		t.Fatal(err)
	}
	if s.Step(c, 1, nil, nil) || s.Orbit != -1 {
		t.Fatalf("day 1: orbit %d, pos %v", s.Orbit, s.Pos)
	}
	for day := 2; !s.Step(c, day, nil, nil); day++ {
		if day > 100 {
			t.Fatal("never arrived")
		}
	}
	if s.Pos != p || s.Order.Kind != NoOrder || s.Orbit != -1 {
		t.Errorf("after: pos %v order %+v orbit %d", s.Pos, s.Order, s.Orbit)
	}
	// Ordered to where it stands: one move, today.
	if err := e.Order(s, Order{Kind: GoToPoint, Pos: p}); err != nil {
		t.Fatal(err)
	}
	if d, ok := s.ETA(c, 0); !ok || d != 0 {
		t.Errorf("ETA on the point: %d %v, want 0", d, ok)
	}
	if !s.Step(c, 1, nil, nil) || s.Pos != p {
		t.Errorf("no arrival on the point itself: %v", s.Pos)
	}
	// Reach: on the sphere is fine, past it refused, and the order kept.
	edge := Vec3{X: SystemReach * 0.6, Y: SystemReach * 0.8}
	if err := s.Check(c, Order{Kind: GoToPoint, Pos: edge}); err != nil {
		t.Errorf("on the reach: %v", err)
	}
	s.Order = Order{Kind: GoToPlanet, Planet: 0}
	out := Vec3{X: edge.X * 1.001, Y: edge.Y * 1.001}
	if err := e.Order(s, Order{Kind: GoToPoint, Pos: out}); err == nil || !strings.Contains(err.Error(), "out of the system") || s.Order.Kind != GoToPlanet {
		t.Errorf("past the reach: %v, order %+v", err, s.Order)
	}
	inLane := *s
	inLane.Lane = c.Stars[c.Home].Lanes[0]
	if err := inLane.Check(c, Order{Kind: GoToPoint, Pos: p}); err == nil || !strings.Contains(err.Error(), "in a star lane") {
		t.Errorf("in a lane: %v", err)
	}
}

// Hop is one move: Reach towards the target for MoveCost, until the day's
// power is out (order kept), then Step refills.
func TestHop(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	s := e.Spawn(laneShip, c.Home, 0)
	d := s.Design
	if s.Power != d.PowerPerDay() {
		t.Fatalf("spawned with power %d, want %d", s.Power, d.PowerPerDay())
	}
	// A point 2.5 moves away, towards the origin.
	from := s.Pos
	r := dist(from, Vec3{})
	k := (r - 2.5*d.Reach()) / r
	p := Vec3{X: from.X * k, Y: from.Y * k, Z: from.Z * k}
	if err := e.Order(s, Order{Kind: GoToPoint, Pos: p}); err != nil {
		t.Fatal(err)
	}
	perDay := d.PowerPerDay() / d.MoveCost()
	if perDay != 2 {
		t.Fatalf("%d moves a day, the fixture needs 2", perDay)
	}
	for i := 1; i <= perDay; i++ {
		moved, arrived := s.Hop(c, 1, nil, nil)
		if !moved || arrived || s.Orbit != -1 || s.Power != d.PowerPerDay()-i*d.MoveCost() {
			t.Fatalf("hop %d: moved %v arrived %v orbit %d power %d", i, moved, arrived, s.Orbit, s.Power)
		}
		if got := dist(from, s.Pos); math.Abs(got-float64(i)*d.Reach()) > 1e-9 {
			t.Errorf("hop %d: %v from the start, want %v", i, got, float64(i)*d.Reach())
		}
	}
	at := s.Pos
	if moved, arrived := s.Hop(c, 1, nil, nil); moved || arrived || s.Pos != at || s.Order.Kind != GoToPoint || s.Power != 0 {
		t.Fatalf("out of power: moved %v arrived %v pos %v order %+v power %d", moved, arrived, s.Pos, s.Order, s.Power)
	}
	if eta, ok := s.ETA(c, 0); !ok || eta != 1 {
		t.Errorf("ETA out of power %d %v, want 1: the close's refill gets there", eta, ok)
	}
	// The close refills, then spends on the order: the moves left, the
	// rest of the power kept for the screen.
	left := int(math.Ceil((dist(at, p) - 1e-9) / d.Reach()))
	if !s.Step(c, 1, nil, nil) || s.Pos != p || s.Order.Kind != NoOrder || s.Power != d.PowerPerDay()-left*d.MoveCost() {
		t.Fatalf("Step: pos %v order %+v power %d, want %d", s.Pos, s.Order, s.Power, d.PowerPerDay()-left*d.MoveCost())
	}
	if s.Power == d.PowerPerDay() || s.Power == 0 {
		t.Fatalf("power %d: the case wants some spent and some left", s.Power)
	}

	// In the ordered orbit: arrived, no power spent.
	s.Orbit, s.Pos, s.Order = 0, c.Stars[c.Home].System.Planets[0].Pos, Order{Kind: GoToPlanet, Planet: 0}
	pw := s.Power
	if moved, arrived := s.Hop(c, 2, nil, nil); moved || !arrived || s.Order.Kind != NoOrder || s.Power != pw {
		t.Errorf("in that orbit: moved %v arrived %v order %+v power %d", moved, arrived, s.Order, s.Power)
	}
	// A refused order is dropped, nothing spent.
	s.Order = Order{Kind: GoToPlanet, Planet: 99}
	if moved, arrived := s.Hop(c, 2, nil, nil); moved || arrived || s.Order.Kind != NoOrder || s.Power != pw {
		t.Errorf("refused: moved %v arrived %v order %+v power %d", moved, arrived, s.Order, s.Power)
	}
	// NoOrder and Colonise do not move.
	for _, o := range []Order{{}, {Kind: Colonise}} {
		s.Order = o
		if moved, arrived := s.Hop(c, 2, nil, nil); moved || arrived || s.Power != pw {
			t.Errorf("%+v: moved %v arrived %v", o, moved, arrived)
		}
	}
}

// An order given without the display (the simulator, an AI) is not a day
// late: Step spends the closing day's power on it before the refill, so
// the ship arrives on day0+ETA like one hopped on the display, in space
// and through a lane.
func TestStepSpendsTheClosingDay(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	c := e.Cluster
	day0 := 10
	s := e.Spawn(laneShip, c.Home, 0)
	s.Order = Order{Kind: GoToPoint, Pos: Vec3{X: 0.5, Y: 0.1, Z: 0.5}}
	eta, ok := s.ETA(c, day0)
	if !ok || eta < 1 {
		t.Fatalf("ETA %d %v: want at least a day for a Refill to matter", eta, ok)
	}
	if got := stepUntil(t, c, s, day0, eta); got != day0+eta || s.Order.Kind != NoOrder {
		t.Fatalf("arrived day %d, want %d; order %+v", got, day0+eta, s.Order)
	}
	s = e.Spawn(laneShip, c.Home, 0)
	to := c.Other(c.Stars[c.Home].Lanes[0], c.Home)
	s.Order = Order{Kind: GoToStar, Star: to}
	eta, ok = s.ETA(c, day0)
	if !ok {
		t.Fatal("no ETA to the star")
	}
	if got := stepUntil(t, c, s, day0, eta); got != day0+eta || s.Star != to || s.Lane >= 0 {
		t.Fatalf("out of the lane day %d, want %d; star %d lane %d", got, day0+eta, s.Star, s.Lane)
	}
}

// Chronomyst lanes: the days at twice the LaneStrength, at least 1, on
// every lane of a real cluster; a 1-day lane stays 1 day; and a Chronomyst
// ship entering a lane is due that many days later.
func TestChronomystLanes(t *testing.T) {
	e := NewEmpire(1, Chronomyst, Average)
	c, tr := e.Cluster, Chronomyst.Traits()
	faster := 0
	for li := range c.Lanes {
		for _, st := range []float64{1, math.Sqrt2, 2} {
			want := LaneDays(c, li, 2*st, Base)
			got, base := LaneDays(c, li, st, tr), LaneDays(c, li, st, Base)
			if got != want {
				t.Errorf("lane %d, strength %g: %d days, want %d", li, st, got, want)
			}
			if base == 1 && got != 1 {
				t.Errorf("lane %d: base 1 day, Chronomyst %d", li, got)
			}
			if got < base {
				faster++
			}
		}
	}
	if faster == 0 {
		t.Error("no lane is faster for the Chronomyst")
	}

	home := e.Colonies[0].Star
	s := e.Spawn(laneShip, home, 0)
	if s.Species != Chronomyst {
		t.Fatalf("ship species %v", s.Species)
	}
	li := c.Stars[home].Lanes[0]
	s.Order = Order{Kind: GoToStar, Star: c.Other(li, home)}
	entered := runEmpire(t, e, 100, func([]Event) bool { return s.Lane >= 0 })
	if want := LaneDays(c, li, laneShip.LaneStrength(), tr); s.Lane != li || s.Arrive != entered+want {
		t.Errorf("lane %d (want %d), arrive %d, entered %d + %d", s.Lane, li, s.Arrive, entered, want)
	}
}

// rank: generators by Power, scanners by Range, drives by Level then least
// Power, the rest by Level; a tie goes to the later gizmo of the table.
func TestBestRank(t *testing.T) {
	cases := []struct {
		techs []Tech
		cat   GizmoCat
		want  Gizmo
	}{
		{[]Tech{InterplanetaryExploration, PowerConversion}, Generator, SubatomicScoop},
		{[]Tech{TonklinDiary, AdvancedInterferometry}, Scanner, SubspacePhaseArray},
		{[]Tech{GravimetricCombustion, InertialControl}, Drive, InertiaNegator},   // Level 6 both, Power 1 < 3
		{[]Tech{EnvironmentalEncapsulation, LightBending}, Shield, WaveScatterer}, // Level 1 both: the later
		{[]Tech{Superconductivity, SpectralAnalysis}, Weapon, FourierMissiles},
	}
	for _, c := range cases {
		if g, ok := best(knowing(c.techs...), c.cat); !ok || g != c.want {
			t.Errorf("%v %v: %v %v, want %v", c.techs, c.cat, g, ok, c.want)
		}
	}
	if _, ok := best(NewResearch(), Generator); ok {
		t.Error("a generator with no research")
	}
}

// Realise: no Best part finds a gizmo on fresh research; with the colony
// ship's techs ColonyTemplate realises to ColonyShip's design; the Best
// parts stay Best in the template; fixed parts pass through unjudged.
func TestTemplateRealise(t *testing.T) {
	if _, ok := ColonyTemplate(false).Realise(NewResearch()); ok {
		t.Error("realised with no generator allowed")
	}
	r := knowing(append(earlyShipTechs, SpacetimeSurfing, AdvancedChemistry)...)
	for _, lane := range []bool{false, true} {
		tp := ColonyTemplate(lane)
		d, ok := tp.Realise(r)
		want, wok := ColonyShip(r, lane)
		if !ok || !wok || d.Name != want.Name || d.Hull != want.Hull || !slices.Equal(d.Parts, want.Parts) {
			t.Errorf("lane %v: %v %v, want ColonyShip's %v", lane, ok, d, want)
		}
		if !tp.Parts[0].Best || tp.Parts[0].Cat != Drive || !tp.Parts[1].Best || tp.Parts[1].Cat != Generator {
			t.Errorf("template parts %v", tp.Parts)
		}
	}
	odd := Template{Name: "Odd", Hull: Small, Parts: []Part{{Gizmo: Nanoshell}, {Best: true, Cat: Drive}}}
	d, ok := odd.Realise(r)
	if !ok || !slices.Equal(d.Parts, []Gizmo{Nanoshell, IonBanger}) || d.Valid(r) == nil {
		t.Errorf("fixed part not allowed yet: %v %v, Valid %v", ok, d, d.Valid(r))
	}
}

// Fixed round-trips through Realise whatever the research.
func TestDesignFixed(t *testing.T) {
	d := Design{Name: "Scout", Hull: Medium, Parts: []Gizmo{ProtonShaver, Nanotwirler, StarLaneDrive}}
	tp := d.Fixed()
	if tp.Name != "Scout" || tp.Hull != Medium || len(tp.Parts) != 3 || tp.Parts[1].Best || tp.Parts[1].Cat != Generator {
		t.Errorf("Fixed %v", tp)
	}
	back, ok := tp.Realise(NewResearch())
	if !ok || back.Name != d.Name || back.Hull != d.Hull || !slices.Equal(back.Parts, d.Parts) {
		t.Errorf("round trip %v %v", ok, back)
	}
}

// A Best part with no allowed gizmo is left out, the rest realised.
func TestTemplateRealisePartial(t *testing.T) {
	r := knowing(OrbitalStructures, InterplanetaryExploration, TonklinDiary) // no lane, a drive and a generator
	tp := Template{Name: "P", Hull: Small, Parts: []Part{{Best: true, Cat: Weapon}, {Best: true, Cat: Drive}, {Gizmo: Colonizer}}}
	d, ok := tp.Realise(r)
	if ok || !slices.Equal(d.Parts, []Gizmo{TonklinMotor, Colonizer}) {
		t.Errorf("%v %v", ok, d)
	}
}

// countsOracle groups d's parts by walking the screen's order: the
// Colonizers, the Invasion Modules, the other Specials by index, then per
// category from Weapon the parts in Parts order, each gizmo once, N its
// count in Parts.
func countsOracle(d Design) []PartCount {
	n := map[Gizmo]int{}
	for _, g := range d.Parts {
		n[g]++
	}
	var out []PartCount
	take := func(g Gizmo) {
		if n[g] > 0 {
			out = append(out, PartCount{g, n[g]})
			n[g] = 0
		}
	}
	take(Colonizer)
	take(InvasionModule)
	for g := range NumGizmos {
		if Gizmos[g].Cat == Special {
			take(g)
		}
	}
	for c := Weapon; c < Special; c++ {
		for _, g := range d.Parts {
			if Gizmos[g].Cat == c {
				take(g)
			}
		}
	}
	return out
}

func TestCounts(t *testing.T) {
	mixed := Design{Hull: Enormous, Parts: []Gizmo{IonWrap, Cloaker, Ueberlaser, InvasionModule, Colonizer, MassBarrageGun,
		Ueberlaser, TonklinMotor, Colonizer, SmartBomb, ProtonShaver, Colonizer, IonWrap}}
	for _, d := range []Design{laneShip, fightFixture, mixed, {Hull: Small}} {
		got := d.Counts()
		if want := countsOracle(d); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", d.Name, got, want)
		}
		sum := 0
		for _, c := range got {
			sum += c.N
		}
		if sum != len(d.Parts) {
			t.Errorf("%s: %d parts counted of %d", d.Name, sum, len(d.Parts))
		}
	}
	// The colony ship read out: the Colonizers lead, the lane drive is a Special.
	want := []PartCount{{Colonizer, 2}, {StarLaneDrive, 1}, {TonklinMotor, 1}, {ProtonShaver, 1}}
	if got := laneShip.Counts(); !slices.Equal(got, want) {
		t.Errorf("colony ship %v, want %v", got, want)
	}
	if got := mixed.Counts()[:2]; got[0] != (PartCount{Colonizer, 3}) || got[1] != (PartCount{InvasionModule, 1}) {
		t.Errorf("mixed leads with %v", got)
	}
}

// Usable: the Colonizer and the Invasion Module, in orbit with one left
// (TestUsableInvade); the other Specials are UseSpecial's
// (TestUsableAgrees); a non-special or a missing part is no use.
func TestUsable(t *testing.T) {
	e := NewEmpire(1, Snovemdomas, Average)
	d := Design{Hull: Medium, Parts: []Gizmo{ProtonShaver, TonklinMotor, Colonizer, InvasionModule, StarLaneDrive}}
	s := e.Spawn(d, e.Cluster.Home, 0)
	col := slices.Index(d.Parts, Colonizer)
	for _, c := range []struct {
		name  string
		part  int
		setup func()
		ok    bool
		say   string
	}{
		{"in orbit", col, func() {}, true, "colonise"},
		{"no cargo", col, func() { s.Cargo = 0 }, false, "no colonizer left"},
		{"in space", col, func() { s.Cargo, s.Orbit = 1, -1 }, false, "not in orbit"},
		{"invasion module", slices.Index(d.Parts, InvasionModule), func() { s.Orbit = 0 }, true, "invade"},
		{"lane drive", slices.Index(d.Parts, StarLaneDrive), func() {}, false, "Star Lane Drive is always on"},
		{"a motor", 1, func() {}, false, "Tonklin Motor is not a special"},
		{"no part", len(d.Parts), func() {}, false, "no part 5"},
	} {
		c.setup()
		if ok, say := s.Usable(c.part); ok != c.ok || say != c.say {
			t.Errorf("%s: %v %q, want %v %q", c.name, ok, say, c.ok, c.say)
		}
	}
}

// Warship: the largest hull research allows, the best engine once, a
// lane drive when asked and allowed, weapons and shields alternating
// weapon first with the generators they need; a shield too dear gives
// way to a weapon. Nothing known: no warship.
func TestWarship(t *testing.T) {
	if _, ok := Warship(NewResearch(), true); ok {
		t.Error("a warship with nothing known")
	}
	early := Design{Hull: Medium, Parts: []Gizmo{TonklinMotor, ProtonShaver, MassBarrageGun}}
	if _, ok := Warship(knowing(early.Hull.Tech(), Gizmos[TonklinMotor].ResearchReq, Gizmos[ProtonShaver].ResearchReq), false); ok {
		t.Error("a warship without a weapon")
	}
	// Medium (10 slots), Tonklin Motor 1, Proton Shaver 2, Mass Barrage
	// Gun 1, Hyperwave Nullifier 4, the Tonklin Diary's scanner (no
	// power, one slot): gun (demand 2, 1 PS, 4 parts), shield (6, 3 PS,
	// 7), gun (7, 4 PS, 9), the shield would need 6 PS (12 parts): a gun
	// instead (8, 4 PS, 10 parts); neither fits after.
	fixed := Design{Hull: Medium, Parts: []Gizmo{TonklinMotor, ProtonShaver, MassBarrageGun, HyperwaveNullifier}}
	r := partTechs(fixed)
	sc, okSc := best(r, Scanner)
	if !okSc || Gizmos[sc].Power != 0 {
		t.Fatalf("fixture: scanner %v %v, want a powerless one allowed", sc, okSc)
	}
	d, ok := Warship(r, true) // lane drive not allowed
	want := []Gizmo{TonklinMotor, ProtonShaver, ProtonShaver, ProtonShaver, ProtonShaver, sc, MassBarrageGun, HyperwaveNullifier, MassBarrageGun, MassBarrageGun}
	if !ok || d.Hull != Medium || !slices.Equal(d.Parts, want) || d.Valid(r) != nil {
		t.Errorf("Medium: %v %v, want %v (%v)", ok, d, want, d.Valid(r))
	}

	all := allKnown()
	largest := Hull(0)
	for h := Hull(0); h < NumHulls; h++ {
		if all.Known[h.Tech()] {
			largest = h
		}
	}
	bestW, _ := best(all, Weapon)
	for _, lane := range []bool{false, true} {
		d, ok := Warship(all, lane)
		if !ok || d.Hull != largest || d.Valid(all) != nil || len(d.Parts) > d.Hull.Slots() {
			t.Errorf("lane %v: %v %v (%v)", lane, ok, d, d.Valid(all))
		}
		if d.LaneDrives() != map[bool]int{false: 0, true: 1}[lane] || !slices.Contains(d.Parts, bestW) {
			t.Errorf("lane %v: %v", lane, d)
		}
		if DesignDamage(d, 0) <= 0 {
			t.Errorf("lane %v: %v deals nothing", lane, d)
		}
	}
}

// WarshipOn carries exactly one Scanner, the best allowed, on a Medium
// or larger hull, none on a Small one or when no scanner is allowed.
func TestWarshipScanner(t *testing.T) {
	scanners := func(d Design) []Gizmo {
		var out []Gizmo
		for _, g := range d.Parts {
			if Gizmos[g].Cat == Scanner {
				out = append(out, g)
			}
		}
		return out
	}
	all := allKnown()
	bestSc, _ := best(all, Scanner)
	for h := Hull(0); h < NumHulls; h++ {
		for _, lane := range []bool{false, true} {
			d, ok := WarshipOn(all, h, lane)
			got := scanners(d)
			want := []Gizmo{bestSc}
			if h == Small {
				want = nil
			}
			if !ok || !slices.Equal(got, want) || d.Valid(all) != nil {
				t.Errorf("%v lane %v: %v %v, scanners %v want %v", h, lane, ok, d, got, want)
			}
		}
	}
	// No scanner allowed: Ion Banger, Proton Shaver, Mass Barrage Gun.
	r := partTechs(Design{Hull: Medium, Parts: []Gizmo{IonBanger, ProtonShaver, MassBarrageGun}})
	if _, ok := best(r, Scanner); ok {
		t.Fatal("fixture: a scanner allowed")
	}
	if d, ok := WarshipOn(r, Medium, true); !ok || len(scanners(d)) != 0 {
		t.Errorf("no scanner known: %v %v", ok, d)
	}
}

// LeavePoint is PlanetClear from the planet, on the planet-star line
// (the star at the system's origin), for every planet of a system.
func TestLeavePoint(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas, Orfa}, Average)
	star := g.Empires[0].Colonies[0].Star
	for i, p := range g.Cluster.Stars[star].System.Planets {
		lp := LeavePoint(g.Cluster, star, i)
		if d := dist(lp, p.Pos); math.Abs(d-PlanetClear) > 1e-9 {
			t.Errorf("planet %d: %.6f from the planet, want %v", i, d, PlanetClear)
		}
		if math.Abs(dist(Vec3{}, lp)+PlanetClear-dist(Vec3{}, p.Pos)) > 1e-9 {
			t.Errorf("planet %d: %v not between the star and %v", i, lp, p.Pos)
		}
	}
}

// Ships leaving one orbit together, each ordered GoToPoint to the same
// LeavePoint, all end in space ShipGap apart, off the orbit ring, with
// their order cleared (the occupied point is replaced by a FreeSpot); a
// free point is still reached exactly.
func TestLeaveOrbitTogether(t *testing.T) {
	for _, n := range []int{3, 5} {
		e := NewEmpire(1, Snovemdomas, Average)
		c := e.Cluster
		sys := c.Stars[c.Home].System
		var ships []*Starship
		for i := 0; i < n; i++ {
			ships = append(ships, e.Spawn(laneShip, c.Home, 0))
		}
		lp := LeavePoint(c, c.Home, 0)
		for _, s := range ships {
			if err := e.Order(s, Order{Kind: GoToPoint, Pos: lp}); err != nil {
				t.Fatal(err)
			}
		}
		for day := 0; day < 3; day++ {
			e.EndDay()
		}
		for i, s := range ships {
			if !s.InSpace() || s.Order.Kind != NoOrder {
				t.Errorf("n=%d ship %d: orbit %d order %+v", n, i, s.Orbit, s.Order)
			}
			if d := dist(s.Pos, sys.Planets[0].Pos); d < PlanetClear-ShipGap-1e-9 {
				t.Errorf("n=%d ship %d: %.3f from the planet", n, i, d)
			}
			for j := i + 1; j < n; j++ {
				if d := dist(s.Pos, ships[j].Pos); d < ShipGap-1e-9 {
					t.Errorf("n=%d ships %d,%d: %.3f apart", n, i, j, d)
				}
			}
		}
		if ships[0].Pos != lp && ships[1].Pos != lp && ships[2].Pos != lp {
			t.Errorf("n=%d: nobody on the point itself", n)
		}
	}
	e := NewEmpire(1, Snovemdomas, Average)
	s := e.Spawn(laneShip, e.Cluster.Home, 0)
	p := Vec3{X: 0.5, Y: 0.2, Z: -0.6}
	e.Order(s, Order{Kind: GoToPoint, Pos: p})
	for day := 0; day < 100 && s.Order.Kind != NoOrder; day++ {
		e.EndDay()
	}
	if s.Pos != p {
		t.Errorf("free point: %v, want %v", s.Pos, p)
	}
}

// Abandon scraps a ship in orbit or in space, refuses one in a lane,
// another empire's, and one being refitted (the refit's project keeps
// the ship).
func TestAbandonShip(t *testing.T) {
	e, col := shipEmpire(t)
	c := e.Cluster
	s := e.Spawn(laneShip, c.Home, 0)
	o := NewEmpire(2, Orfa, Average)
	if err := o.Abandon(s); err != errNotOurs {
		t.Errorf("another empire's: %v", err)
	}
	s.Lane, s.Arrive = 0, 5
	if err := e.Abandon(s); err == nil {
		t.Error("abandoned in a lane")
	}
	s.Lane = -1
	if err := e.Abandon(s); err != nil || slices.Contains(e.Ships, s) {
		t.Errorf("abandon: %v, still ours %v", err, slices.Contains(e.Ships, s))
	}
	if err := e.Abandon(s); err != errNotOurs {
		t.Errorf("abandoned twice: %v", err)
	}
	// Being refitted: refused.
	e.Research.Grant(Gravimetrics)
	col.Planet.Orbit[0] = OrbitalDocks
	s2 := e.Spawn(laneShip, col.Star, col.PlanetIdx)
	s2.Hull--
	if err := e.Refit(col, s2, s2.Design); err != nil {
		t.Fatal(err)
	}
	if err := e.Abandon(s2); err == nil || !slices.Contains(e.Ships, s2) {
		t.Errorf("abandoned under refit: %v", err)
	}
}
