package game

import (
	"slices"
	"strings"
	"testing"
)

// restree.txt: "66", then "Name flag cost prereq... 255" lines; the older
// table after them is not read.
func TestResearchTableMatchesGame(t *testing.T) {
	lines := gameTxt(t, "restree.txt")
	if n := numbers(t, lines[0]); len(n) != 1 || n[0] != int(NumTechs) {
		t.Fatalf("count line %q, want %d", lines[0], NumTechs)
	}
	for i := Tech(0); i < NumTechs; i++ {
		f := strings.Fields(lines[1+int(i)])
		name := strings.ReplaceAll(f[0], "^", " ")
		n := numbers(t, strings.Join(f[1:], " "))
		if len(n) < 3 || n[0] != 0 || n[len(n)-1] != 255 {
			t.Fatalf("tech %d: %q", i, lines[1+int(i)])
		}
		var req []Tech
		for _, p := range n[2 : len(n)-1] {
			req = append(req, Tech(p))
		}
		got := Techs[i]
		if got.Name != name || got.Cost != n[1] || !slices.Equal(got.Requires, req) {
			t.Errorf("tech %d: game %q %d %v, typed %q %d %v", i, name, n[1], req, got.Name, got.Cost, got.Requires)
		}
	}
}

// The prerequisites are in range and acyclic: three point FORWARD in the
// file (5->6, 19->21, 55->56), so file order is not a topological order.
func TestResearchDAG(t *testing.T) {
	const (
		white = iota
		grey
		black
	)
	var color [NumTechs]int
	var visit func(t0 Tech) bool
	visit = func(t0 Tech) bool {
		switch color[t0] {
		case grey:
			return false
		case black:
			return true
		}
		color[t0] = grey
		for _, p := range Techs[t0].Requires {
			if p >= NumTechs {
				t.Fatalf("%s requires tech %d", t0, p)
			}
			if p == t0 || !visit(p) {
				return false
			}
		}
		color[t0] = black
		return true
	}
	for i := Tech(0); i < NumTechs; i++ {
		if !visit(i) {
			t.Fatalf("cycle through %s", i)
		}
	}
}

func TestTechNamesAreConstants(t *testing.T) {
	for _, c := range []struct {
		t    Tech
		name string
	}{
		{OrbitalStructures, "Orbital Structures"},
		{Hyperlogic, "Hyperlogic"},
		{Cloaking, "Cloaking"},
		{EMFieldCoupling, "EM Field Coupling"},
		{FergnatzsLastTheorem, "Fergnatz's Last Theorem"},
		{IllusoryMachinations, "Illusory Machinations"},
	} {
		if c.t.String() != c.name {
			t.Errorf("tech %d is %q, want %q", c.t, c.t, c.name)
		}
	}
	if Cloaking != 15 || IllusoryMachinations != NumTechs-1 || NoTech.String() != "none" {
		t.Errorf("Cloaking %d, IllusoryMachinations %d, NoTech %q", Cloaking, IllusoryMachinations, NoTech)
	}
}

func TestResearchStart(t *testing.T) {
	r := NewResearch()
	var want []Tech
	for i := Tech(0); i < NumTechs; i++ {
		if len(Techs[i].Requires) == 0 {
			want = append(want, i)
		}
	}
	if got := r.Available(); !slices.Equal(got, want) {
		t.Fatalf("available %v, want %v", got, want)
	}
	if r.Topic != NoTech {
		t.Errorf("topic %v", r.Topic)
	}
	if d, ok := r.Advance(100); ok || d != NoTech {
		t.Errorf("no topic discovered %v", d)
	}
	locked := Hyperlogic // needs 6 and 8
	if r.CanStudy(locked) || r.Study(locked) == nil {
		t.Errorf("%s studiable from scratch", locked)
	}
	if r.Study(NoTech) == nil {
		t.Error("NoTech studiable")
	}
}

func TestResearchAdvance(t *testing.T) {
	r := NewResearch()
	// Interplanetary Exploration needs only Orbital Structures.
	if err := r.Study(OrbitalStructures); err != nil {
		t.Fatal(err)
	}
	cost := r.Cost(OrbitalStructures)
	if _, ok := r.Advance(cost - 1); ok || r.Topic != OrbitalStructures || r.Progress[OrbitalStructures] != cost-1 {
		t.Fatalf("below cost: topic %v progress %d", r.Topic, r.Progress[OrbitalStructures])
	}
	if r.CanStudy(InterplanetaryExploration) {
		t.Fatal("dependant available early")
	}
	d, ok := r.Advance(10)
	if !ok || d != OrbitalStructures || !r.Known[d] || r.Topic != NoTech {
		t.Fatalf("across cost: %v %v known %v topic %v", d, ok, r.Known[d], r.Topic)
	}
	if r.Progress[d] != cost {
		t.Errorf("surplus kept: progress %d, cost %d", r.Progress[d], cost)
	}
	if !r.CanStudy(InterplanetaryExploration) || r.CanStudy(OrbitalStructures) {
		t.Error("availability after discovery")
	}
	// Spacetime Surfing needs 1 and 2: not yet.
	if r.CanStudy(SpacetimeSurfing) {
		t.Error("Spacetime Surfing with one of two prerequisites")
	}
	defer func() {
		if recover() == nil {
			t.Error("negative points did not panic")
		}
	}()
	r.Advance(-1)
}

func TestResearchSwitchKeepsProgress(t *testing.T) {
	r := NewResearch()
	r.Study(OrbitalStructures)
	r.Advance(20)
	if err := r.Study(TonklinDiary); err != nil {
		t.Fatal(err)
	}
	r.Advance(5)
	if r.Progress[OrbitalStructures] != 20 || r.Progress[TonklinDiary] != 5 {
		t.Fatalf("progress %d %d", r.Progress[OrbitalStructures], r.Progress[TonklinDiary])
	}
	r.Study(OrbitalStructures)
	if d, ok := r.Advance(r.Cost(OrbitalStructures) - 20); !ok || d != OrbitalStructures {
		t.Errorf("resumed study: %v %v", d, ok)
	}
}

func TestResearchGrant(t *testing.T) {
	r := NewResearch()
	r.Grant(Hyperlogic) // prerequisites 6 and 8 unknown
	if !r.Known[Hyperlogic] {
		t.Fatal("not granted")
	}
	for i := Tech(0); i < NumTechs; i++ {
		if !slices.Contains(Techs[i].Requires, Hyperlogic) {
			continue
		}
		all := true
		for _, p := range Techs[i].Requires {
			all = all && r.Known[p]
		}
		if r.CanStudy(i) != all {
			t.Errorf("%s: CanStudy %v, prerequisites known %v", i, r.CanStudy(i), all)
		}
	}
	if !r.CanStudy(Diplomatics) || !r.CanStudy(LevelLogic) { // need Hyperlogic alone
		t.Error("dependants of a granted tech not available")
	}
}

func TestResearchAllows(t *testing.T) {
	r := NewResearch()
	for it := Item(0); it < NumItems; it++ {
		if Items[it].ResearchReq == NoResearch && !r.Allows(it) {
			t.Errorf("%s needs nothing, refused", it)
		}
	}
	if Items[Factory].ResearchReq != NoResearch || !r.Allows(Factory) {
		t.Error("Factory")
	}
	campus := Tech(Items[ResearchCampus].ResearchReq)
	if campus != Hyperlogic || r.Allows(ResearchCampus) {
		t.Fatalf("Research Campus needs %v, allowed %v", campus, r.Allows(ResearchCampus))
	}
	r.Grant(campus)
	if !r.Allows(ResearchCampus) {
		t.Error("Research Campus refused after its tech")
	}
	if items, _ := r.Unlocks(campus); !slices.Contains(items, ResearchCampus) {
		t.Errorf("Unlocks(%s) = %v", campus, items)
	}
	if r.AllowsGizmo(MassBarrageGun) {
		t.Error("gizmo allowed without its tech")
	}
	r.Grant(Gizmos[MassBarrageGun].ResearchReq)
	if !r.AllowsGizmo(MassBarrageGun) {
		t.Error("gizmo refused after its tech")
	}
	// Interstellar Trucking's 254 is no tech: never allowed, even knowing all.
	for i := Tech(0); i < NumTechs; i++ {
		r.Grant(i)
	}
	if r.Allows(InterstellarTrucking) {
		t.Error("Interstellar Trucking allowed")
	}
}

func TestUnlocksMatchRequirements(t *testing.T) {
	r := NewResearch()
	nItems, nGizmos := 0, 0
	for i := Tech(0); i < NumTechs; i++ {
		items, gizmos := r.Unlocks(i)
		for _, it := range items {
			if Items[it].ResearchReq != int(i) {
				t.Errorf("%s unlocks %s, which needs %d", i, it, Items[it].ResearchReq)
			}
		}
		for _, g := range gizmos {
			if Gizmos[g].ResearchReq != i {
				t.Errorf("%s unlocks %s, which needs %v", i, g, Gizmos[g].ResearchReq)
			}
		}
		nItems += len(items)
		nGizmos += len(gizmos)
	}
	// Every requirement that names a tech is someone's unlock.
	wantItems, wantGizmos := 0, 0
	for it := Item(0); it < NumItems; it++ {
		if Items[it].ResearchReq < int(NumTechs) {
			wantItems++
		}
	}
	for g := Gizmo(0); g < NumGizmos; g++ {
		if Gizmos[g].ResearchReq != NoTech {
			wantGizmos++
		}
	}
	if nItems != wantItems || nGizmos != wantGizmos {
		t.Errorf("unlocks %d items %d gizmos, want %d %d", nItems, nGizmos, wantItems, wantGizmos)
	}
}

// Studying Available()[0] to completion, over and over, learns the whole tree.
func TestResearchWholeTree(t *testing.T) {
	r := NewResearch()
	for n := 0; n < int(NumTechs); n++ {
		av := r.Available()
		if len(av) == 0 {
			t.Fatalf("stuck after %d techs", n)
		}
		if err := r.Study(av[0]); err != nil {
			t.Fatal(err)
		}
		if d, ok := r.Advance(r.Cost(av[0])); !ok || d != av[0] {
			t.Fatalf("%s not discovered", av[0])
		}
	}
	for i := Tech(0); i < NumTechs; i++ {
		if !r.Known[i] {
			t.Errorf("%s unknown", i)
		}
	}
	if len(r.Available()) != 0 {
		t.Error("techs left available")
	}
}

// manualDays is the fan manual's research list (b-sting.nl, "days" at one
// research point per turn): the oracle for Cost.
var manualDays = map[Tech]int{
	OrbitalStructures: 50, InterplanetaryExploration: 50, TonklinDiary: 50,
	Xenobiology: 90, EnvironmentalEncapsulation: 50, SpectralAnalysis: 120,
	Superconductivity: 100, SpacetimeSurfing: 90, AdvancedChemistry: 100,
	AdvancedInterferometry: 90, Cloaking: 1800, PowerConversion: 100,
	GravityControl: 560, Hyperlogic: 1120, MomentumDeconservation: 280,
	Diplomatics: 2000, MolecularExplosives: 720, StrongForceWeakening: 980,
	LevelLogic: 3300, LightBending: 2560, Gravimetrics: 1320,
	EMFieldCoupling: 4200, MassPhasing: 2200, ThoughtAnalysis: 7700,
	AdvancedFunTechniques: 5580, PositronGuidance: 2400, Subatomics: 3300,
	AdvancedExploration: 4140, GravimetricCombustion: 2600, Plasmatics: 5700,
	PlanetaryReplenishment: 2560, MomentumReflection: 4420,
	LargeScaleConstruction: 4200, Hyperradiation: 5040,
	SuperstringCompression: 3640, MurgatroydHypothesis: 5940,
	EnergyRedirection: 4400, StasisFieldScience: 3600, MatterDuplication: 5040,
	ScientificSorcery: 5400, StarLaneAnatomy: 3450, CoherentPhotonics: 7000,
	Microbotics: 9020, EnergyFocusing: 5220, InertialControl: 10080,
	AdvancedPlanetaryArmaments: 12240, FergnatzsLastTheorem: 10880,
	Hypergeometry: 12920, RepulsionBeamTech: 7040, HyperwaveTechnology: 7260,
	MegagraphTheory: 11760, Nanoenergons: 12580, EcospherePhaseControl: 9240,
	Teleinfiltration: 8580, HyperwaveEmissionControl: 9460,
	HyperdriveTechnology: 8800, ActionAtADistance: 12960, DoomMechanization: 37600,
	Nanofocusing: 22080, Snooping: 11520, SelfModifyingStructures: 12000,
	Nanopropulsion: 17600, Nanodeflection: 21600, AccelEnergyReplenishment: 12480,
	GravityFlowControl: 12720, IllusoryMachinations: 33000,
}

func TestCostMatchesManual(t *testing.T) {
	if len(manualDays) != int(NumTechs) {
		t.Fatalf("%d manual entries, want %d", len(manualDays), int(NumTechs))
	}
	r := NewResearch()
	for tech, days := range manualDays {
		if got := r.Cost(tech); got != days {
			t.Errorf("%v: cost %d, manual says %d days", tech, got, days)
		}
	}
}

// Days rounds the remaining cost up at the rate: 0 once known, -1 at no
// research; ResearchRate sums the colonies' Output research.
func TestResearchDays(t *testing.T) {
	g := NewGame(1, []Species{Snovemdomas}, Average)
	e := g.Empires[0]
	r := e.Research
	tc := r.Available()[0]
	c := r.Cost(tc)
	if r.Days(tc, c) != 1 || r.Days(tc, c+1) != 1 || r.Days(tc, 0) != -1 {
		t.Errorf("days at %d: %d, at 0: %d", c, r.Days(tc, c), r.Days(tc, 0))
	}
	r.Study(tc)
	r.Advance(c - 1)
	if r.Days(tc, 1) != 1 {
		t.Errorf("one point left: %d days", r.Days(tc, 1))
	}
	r.Grant(tc)
	if r.Days(tc, 0) != 0 {
		t.Errorf("known: %d days", r.Days(tc, 0))
	}
	if got := e.ResearchRate(); got != 0 {
		t.Errorf("a new home researches %d a day", got)
	}
	c0 := e.Colonies[0]
	at, ok := c0.Planet.BestCell(Laboratory, c0.Traits())
	if !ok {
		t.Fatal("no Laboratory square at home")
	}
	if err := c0.Planet.Build(at, Laboratory); err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, col := range e.Colonies {
		_, res, _ := col.Output()
		want += res
	}
	if got := e.ResearchRate(); got != want || want <= 0 {
		t.Errorf("rate %d, want %d (> 0)", got, want)
	}
}

// A dense cluster scales the costs from DenseFrom by Stars / DenseStars,
// none at or under DenseStars; the empires of a game carry its count.
func TestDenseCost(t *testing.T) {
	plain := NewResearch()
	for _, c := range []struct {
		stars int
		num   int // the scale's numerator over 2
	}{{0, 2}, {15, 2}, {50, 2}, {75, 3}, {100, 4}} {
		r := &Research{Stars: c.stars}
		for tc := Tech(0); tc < NumTechs; tc++ {
			want := plain.Cost(tc)
			if tc >= DenseFrom {
				want = want * c.num / 2
			}
			if got := r.Cost(tc); got != want {
				t.Errorf("%d stars, %v: cost %d, want %d", c.stars, tc, got, want)
			}
		}
	}
	for _, d := range []Density{Average, VeryDense} {
		g := NewGame(1, []Species{Orfa, Mebes}, d)
		for _, e := range g.Empires {
			if e.Research.Stars != len(g.Cluster.Stars) {
				t.Errorf("%v: empire %d's Stars %d of %d", d, e.Index, e.Research.Stars, len(g.Cluster.Stars))
			}
		}
	}
}
