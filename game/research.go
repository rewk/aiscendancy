package game

import "fmt"

// Tech is a technology, indexed 0-65 as restree.txt lists them (the fan
// manual counts from 1). restree.txt: a count line, a blank, then "Name
// flag cost prereq... 255" per tech (^ for spaces, flag always 0). The
// prerequisites are NOT all earlier in the file, but the graph has no
// cycle. An abandoned table in capitals follows: ignored.
type Tech uint8

const (
	OrbitalStructures Tech = iota
	InterplanetaryExploration
	TonklinDiary
	SpacetimeSurfing
	Superconductivity
	EnvironmentalEncapsulation
	Xenobiology
	SpectralAnalysis
	AdvancedInterferometry
	PowerConversion
	AdvancedChemistry
	MomentumDeconservation
	GravityControl
	MolecularExplosives
	Hyperlogic
	Cloaking
	Gravimetrics
	StrongForceWeakening
	LightBending
	AdvancedExploration
	Diplomatics
	MassPhasing
	PositronGuidance
	GravimetricCombustion
	EMFieldCoupling
	Subatomics
	PlanetaryReplenishment
	MomentumReflection
	Hyperradiation
	Plasmatics
	EnergyRedirection
	LargeScaleConstruction
	LevelLogic
	StarLaneAnatomy
	StasisFieldScience
	CoherentPhotonics
	SuperstringCompression
	MurgatroydHypothesis
	MatterDuplication
	EnergyFocusing
	ScientificSorcery
	AdvancedFunTechniques
	RepulsionBeamTech
	HyperwaveTechnology
	FergnatzsLastTheorem
	ThoughtAnalysis
	InertialControl
	Nanoenergons
	Hypergeometry
	Teleinfiltration
	HyperdriveTechnology
	Microbotics
	EcospherePhaseControl
	HyperwaveEmissionControl
	Nanopropulsion
	Nanodeflection
	Nanofocusing
	DoomMechanization
	Snooping
	MegagraphTheory
	SelfModifyingStructures
	AdvancedPlanetaryArmaments
	AccelEnergyReplenishment
	GravityFlowControl
	ActionAtADistance
	IllusoryMachinations
	NumTechs // 66
)

// NoTech is the tables' 255: nothing required, or no current study.
const NoTech Tech = NoResearch

// TechInfo is one line of restree.txt.
type TechInfo struct {
	Name     string
	Cost     int    // research points, restree.txt's cost column
	Requires []Tech // prerequisites, nil for none
}

// Techs is restree.txt, typed; TestResearchTableMatchesGame checks it
// against the installed game.
var Techs = [NumTechs]TechInfo{
	OrbitalStructures:          {"Orbital Structures", 50, nil},
	InterplanetaryExploration:  {"Interplanetary Exploration", 50, []Tech{OrbitalStructures}},
	TonklinDiary:               {"Tonklin Diary", 50, nil},
	SpacetimeSurfing:           {"Spacetime Surfing", 90, []Tech{InterplanetaryExploration, TonklinDiary}},
	Superconductivity:          {"Superconductivity", 100, []Tech{TonklinDiary}},
	EnvironmentalEncapsulation: {"Environmental Encapsulation", 50, []Tech{Xenobiology}},
	Xenobiology:                {"Xenobiology", 90, nil},
	SpectralAnalysis:           {"Spectral Analysis", 120, []Tech{TonklinDiary}},
	AdvancedInterferometry:     {"Advanced Interferometry", 90, []Tech{SpectralAnalysis}},
	PowerConversion:            {"Power Conversion", 100, []Tech{SpacetimeSurfing}},
	AdvancedChemistry:          {"Advanced Chemistry", 100, []Tech{EnvironmentalEncapsulation}},
	MomentumDeconservation:     {"Momentum Deconservation", 140, []Tech{SpacetimeSurfing}},
	GravityControl:             {"Gravity Control", 140, []Tech{SpacetimeSurfing}},
	MolecularExplosives:        {"Molecular Explosives", 120, []Tech{SpectralAnalysis, PowerConversion}},
	Hyperlogic:                 {"Hyperlogic", 140, []Tech{Xenobiology, AdvancedInterferometry}},
	Cloaking:                   {"Cloaking", 180, []Tech{AdvancedInterferometry}},
	Gravimetrics:               {"Gravimetrics", 110, []Tech{GravityControl}},
	StrongForceWeakening:       {"Strong Force Weakening", 70, []Tech{PowerConversion}},
	LightBending:               {"Light Bending", 160, []Tech{GravityControl}},
	AdvancedExploration:        {"Advanced Exploration", 230, []Tech{Hyperlogic, MassPhasing}},
	Diplomatics:                {"Diplomatics", 100, []Tech{Hyperlogic}},
	MassPhasing:                {"Mass Phasing", 100, []Tech{Gravimetrics}},
	PositronGuidance:           {"Positron Guidance", 100, []Tech{MolecularExplosives}},
	GravimetricCombustion:      {"Gravimetric Combustion", 100, []Tech{Gravimetrics, PositronGuidance}},
	EMFieldCoupling:            {"EM Field Coupling", 150, []Tech{LightBending}},
	Subatomics:                 {"Subatomics", 110, []Tech{PositronGuidance}},
	PlanetaryReplenishment:     {"Planetary Replenishment", 80, []Tech{EnvironmentalEncapsulation, GravimetricCombustion}},
	MomentumReflection:         {"Momentum Reflection", 130, []Tech{MomentumDeconservation, Subatomics}},
	Hyperradiation:             {"Hyperradiation", 140, []Tech{EMFieldCoupling, MomentumReflection}},
	Plasmatics:                 {"Plasmatics", 150, []Tech{Subatomics}},
	EnergyRedirection:          {"Energy Redirection", 110, []Tech{Hyperradiation}},
	LargeScaleConstruction:     {"Large Scale Construction", 100, []Tech{AdvancedExploration, GravimetricCombustion}},
	LevelLogic:                 {"Level Logic", 75, []Tech{Hyperlogic}},
	StarLaneAnatomy:            {"Star Lane Anatomy", 75, []Tech{Plasmatics, EnergyRedirection}},
	StasisFieldScience:         {"Stasis Field Science", 75, []Tech{Hyperradiation}},
	CoherentPhotonics:          {"Coherent Photonics", 140, []Tech{EnergyRedirection}},
	SuperstringCompression:     {"Superstring Compression", 70, []Tech{Plasmatics}},
	MurgatroydHypothesis:       {"Murgatroyd Hypothesis", 110, []Tech{LevelLogic, SuperstringCompression}},
	MatterDuplication:          {"Matter Duplication", 90, []Tech{MomentumReflection, SuperstringCompression}},
	EnergyFocusing:             {"Energy Focusing", 90, []Tech{CoherentPhotonics, MurgatroydHypothesis}},
	ScientificSorcery:          {"Scientific Sorcery", 90, []Tech{MurgatroydHypothesis}},
	AdvancedFunTechniques:      {"Advanced Fun Techniques", 90, []Tech{Diplomatics}},
	RepulsionBeamTech:          {"Repulsion Beam Tech", 110, []Tech{EnergyFocusing}},
	HyperwaveTechnology:        {"Hyperwave Technology", 110, []Tech{Hyperradiation, EnergyFocusing}},
	FergnatzsLastTheorem:       {"Fergnatz's Last Theorem", 160, []Tech{ScientificSorcery}},
	ThoughtAnalysis:            {"Thought Analysis", 110, []Tech{LevelLogic}},
	InertialControl:            {"Inertial Control", 140, []Tech{StarLaneAnatomy, MatterDuplication}},
	Nanoenergons:               {"Nanoenergons", 170, []Tech{HyperwaveTechnology, FergnatzsLastTheorem}},
	Hypergeometry:              {"Hypergeometry", 170, []Tech{FergnatzsLastTheorem}},
	Teleinfiltration:           {"Teleinfiltration", 110, []Tech{HyperwaveTechnology, ThoughtAnalysis}},
	HyperdriveTechnology:       {"Hyperdrive Technology", 110, []Tech{StarLaneAnatomy, Teleinfiltration}},
	Microbotics:                {"Microbotics", 110, []Tech{MatterDuplication}},
	EcospherePhaseControl:      {"Ecosphere Phase Control", 110, []Tech{PlanetaryReplenishment, Hypergeometry}},
	HyperwaveEmissionControl:   {"Hyperwave Emission Control", 110, []Tech{HyperwaveTechnology}},
	Nanopropulsion:             {"Nanopropulsion", 200, []Tech{Nanoenergons, HyperdriveTechnology}},
	Nanodeflection:             {"Nanodeflection", 240, []Tech{InertialControl, Nanofocusing}},
	Nanofocusing:               {"Nanofocusing", 240, []Tech{Nanoenergons}},
	DoomMechanization:          {"Doom Mechanization", 400, []Tech{Teleinfiltration, HyperwaveEmissionControl}},
	Snooping:                   {"Snooping", 120, []Tech{ThoughtAnalysis, HyperwaveEmissionControl}},
	MegagraphTheory:            {"Megagraph Theory", 120, []Tech{FergnatzsLastTheorem}},
	SelfModifyingStructures:    {"Self Modifying Structures", 120, []Tech{Microbotics, Nanofocusing}},
	AdvancedPlanetaryArmaments: {"Advanced Planetary Armaments", 120, []Tech{LargeScaleConstruction, CoherentPhotonics}},
	AccelEnergyReplenishment:   {"Accel Energy Replenishment", 120, []Tech{Nanopropulsion, MegagraphTheory}},
	GravityFlowControl:         {"Gravity Flow Control", 120, []Tech{Nanopropulsion}},
	ActionAtADistance:          {"Action At A Distance", 120, []Tech{Teleinfiltration, MegagraphTheory}},
	IllusoryMachinations:       {"Illusory Machinations", 300, []Tech{Nanofocusing, AccelEnergyReplenishment}},
}

// String is the tech's display name, "none" for NoTech or out of range.
func (t Tech) String() string {
	if t >= NumTechs {
		return "none"
	}
	return Techs[t].Name
}

// costScale multiplies restree.txt's cost into the points a tech takes: 1
// for index 0-10, then 2·(index−10). Derived from the fan manual's days at
// one point a day (b-sting.nl), checked in the game; TestCostMatchesManual
// holds all 66.
func costScale(t Tech) int {
	if t <= 10 {
		return 1
	}
	return 2 * (int(t) - 10)
}

// Research is one species' state of the tree.
type Research struct {
	Known    [NumTechs]bool
	Progress [NumTechs]int // points sunk into each tech; kept when the topic changes
	Topic    Tech          // the current study, NoTech for none
	// Stars is the cluster's star count: a dense one scales the costs
	// from DenseFrom (Cost); 0, a lone colony, none.
	Stars int
}

// NewResearch is a species that knows nothing and studies nothing.
func NewResearch() *Research { return &Research{Topic: NoTech} }

// Available lists the techs not known whose prerequisites all are, in index
// order: the empty disks of the research screen.
func (r *Research) Available() []Tech {
	var out []Tech
	for t := Tech(0); t < NumTechs; t++ {
		if r.CanStudy(t) {
			out = append(out, t)
		}
	}
	return out
}

// CanStudy reports t is unknown and every prerequisite is known.
func (r *Research) CanStudy(t Tech) bool {
	if t >= NumTechs || r.Known[t] {
		return false
	}
	for _, p := range Techs[t].Requires {
		if !r.Known[p] {
			return false
		}
	}
	return true
}

// Study makes t the current topic. Progress on the previous topic stays.
func (r *Research) Study(t Tech) error {
	if !r.CanStudy(t) {
		return fmt.Errorf("research: %v cannot be studied", t)
	}
	r.Topic = t
	return nil
}

// DenseFrom is the first tech index whose cost a dense cluster scales,
// and DenseStars the star count past which it does
// (with the QoL a dense cluster's many colonies ran the tree out early).
const (
	DenseFrom  = 15
	DenseStars = 50
)

// Cost is the points t takes to discover: the table's cost scaled by its
// place in the tree, then from DenseFrom by max(1, Stars / DenseStars)
// (×1.5 at 75 stars, ×2 at 100), multiplied before the division.
func (r *Research) Cost(t Tech) int {
	c := Techs[t].Cost * costScale(t)
	if t >= DenseFrom && r.Stars > DenseStars {
		c = c * r.Stars / DenseStars
	}
	return c
}

// Days is ⌈(Cost − Progress) / rate⌉: 0 for a known tech, -1 for rate ≤ 0.
func (r *Research) Days(t Tech, rate int) int {
	switch {
	case r.Known[t]:
		return 0
	case rate <= 0:
		return -1
	}
	return (r.Cost(t) - r.Progress[t] + rate - 1) / rate
}

// Advance spends a day's points on the topic; at the cost the tech is
// known, the topic cleared and the tech returned. The surplus is lost, as
// in the original, and so are the points with no topic.
func (r *Research) Advance(points int) (discovered Tech, ok bool) {
	if points < 0 {
		panic(fmt.Sprintf("research: negative points %d", points))
	}
	t := r.Topic
	if t == NoTech {
		return NoTech, false
	}
	r.Progress[t] += points
	if r.Progress[t] < r.Cost(t) {
		return NoTech, false
	}
	r.Progress[t] = r.Cost(t)
	r.Known[t] = true
	r.Topic = NoTech
	return t, true
}

// Grant learns t outright, prerequisites or not (a xeno dig, diplomacy).
// Granting the current topic clears it.
func (r *Research) Grant(t Tech) {
	r.Known[t] = true
	if r.Topic == t {
		r.Topic = NoTech
	}
}

// Complete reports every tech known: the scientific singularity
// (Colony.Output, SingularityShare). Derived, nothing stored.
func (r *Research) Complete() bool {
	for _, k := range r.Known {
		if !k {
			return false
		}
	}
	return true
}

// SingularityText is the news of the tree's completion (singularity),
// SingularityStory its long telling (the window's discovery popup).
const (
	SingularityText  = "Scientific singularity reached"
	SingularityStory = "Scientific singularity. There is nothing left to discover: even matter now bends to science. From today, half of every colony's research goes to its industry."
)

// singularity is the event following the learning of t when t completed
// r's tree: t was unknown before (every grant path learns only an
// unknown tech), so the tree was not complete then and the event fires
// on the transition alone. false for NoTech (nothing learnt) or a tree
// still incomplete.
func singularity(r *Research, t Tech) (Event, bool) {
	if t == NoTech || !r.Complete() {
		return Event{}, false
	}
	return Event{Kind: Discovered, Item: NoItem, Tech: NoTech, Star: -1, Planet: -1, Other: -1, Text: SingularityText}, true
}

// known reports a table's research requirement is met. A requirement past
// the tree other than NoResearch (Interstellar Trucking's 254) never is.
func (r *Research) known(req int) bool {
	if req == NoResearch {
		return true
	}
	return req >= 0 && req < int(NumTechs) && r.Known[req]
}

// Allows reports the item's research requirement is met.
func (r *Research) Allows(it Item) bool { return r.known(Items[it].ResearchReq) }

// AllowsGizmo reports the ship part's research requirement is met.
func (r *Research) AllowsGizmo(g Gizmo) bool { return r.known(int(Gizmos[g].ResearchReq)) }

// Unlocks lists the items and ship parts that require t, in index order;
// Unlocks(NoTech) what needs no research.
func (r *Research) Unlocks(t Tech) (items []Item, gizmos []Gizmo) {
	for it := Item(0); it < NumItems; it++ {
		if Items[it].ResearchReq == int(t) {
			items = append(items, it)
		}
	}
	for g := Gizmo(0); g < NumGizmos; g++ {
		if Gizmos[g].ResearchReq == t {
			gizmos = append(gizmos, g)
		}
	}
	return items, gizmos
}
