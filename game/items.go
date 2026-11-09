package game

import "slices"

// Item is a planetary structure or project, numbered as in the game's
// planitem.txt (and as the frames of planitem.shp): 0-21 surface
// structures, 22-30 orbital structures, 31-38 projects.
type Item int8

// NoItem marks an empty square or orbital slot.
const NoItem Item = -1

const (
	Factory Item = iota
	Agriplot
	Laboratory
	Habitat
	Metroplex
	ColonyBase
	IndustrialMegafacility
	ArtificialHydroponifer
	ResearchCampus
	LogicFactory
	EngineeringRetreat
	SurfaceCloaker
	HyperpowerPlant
	FertilizationPlant
	Internet
	CloningPlant
	ObservationInstallation
	TractorBeam
	SurfaceShield
	SurfaceMegaShield
	Outpost
	TransportTubes
	Shipyard
	Ship
	OrbitalDocks
	OrbitalCloaker
	OrbitalShield
	OrbitalMegaShield
	OrbitalMissileBase
	ShortRangeOrbitalWhopper
	LongRangeOrbitalWhopper
	AlienHospitality
	InterstellarTrucking
	EndlessParty
	ScientistTakeover
	Automation
	Terraforming
	LushGrowthBomb
	XenoArcheologicalDig
	NumItems
)

const (
	FirstOrbital = Shipyard
	FirstProject = AlienHospitality
)

// Item flags, as planitem.txt lists them after the research requirement.
// The meanings are read off which items carry them; 0 and 5 are guesses.
const (
	FlagSpecial = 0 // placed on something that exists already (base, tubes, automation, terraforming, dig)
	FlagProject = 1 // a project, not a structure
	FlagWeapon  = 2
	FlagShield  = 3
	FlagOngoing = 5 // a project that never completes
	FlagOnBlack = 6 // the data's black-square flag (outpost, tubes); CanBuild keeps the tubes
	NoResearch  = 255
)

// ItemInfo is one row of planitem.txt.
type ItemInfo struct {
	Name                      string
	Industry, Research, Prosp int // output
	MaxPop                    int // room it adds for population
	UsePop                    int // population it employs
	Cost                      int // industry to build
	ResearchReq               int // technology index, NoResearch for none
	Flags                     []int
}

// Items is planitem.txt, typed; TestItemsMatchGame checks it against the
// installed game.
var Items = [NumItems]ItemInfo{
	{"Factory", 1, 0, 0, 0, 1, 30, 255, nil},
	{"Agriplot", 0, 0, 2, 0, 1, 30, 255, nil},
	{"Laboratory", 0, 1, 0, 0, 1, 50, 255, nil},
	{"Habitat", 0, 0, 2, 3, 1, 160, 26, nil},
	{"Metroplex", 1, 1, 1, 2, 1, 200, 31, nil},
	{"Colony Base", 1, 0, 1, 2, 1, 120, 5, []int{0}},
	{"Industrial Megafacility", 3, 0, 0, 0, 1, 110, 22, nil},
	{"Artificial Hydroponifer", 0, 0, 3, 0, 1, 100, 10, nil},
	{"Research Campus", 0, 3, 0, 0, 1, 160, 14, nil},
	{"Logic Factory", 0, 1, 1, 0, 1, 80, 41, nil},
	{"Engineering Retreat", 1, 1, 0, 0, 1, 80, 45, nil},
	{"Surface Cloaker", 0, 0, 0, 0, 1, 40, 15, nil},
	{"Hyperpower Plant", 0, 0, 0, 0, 1, 200, 25, nil},
	{"Fertilization Plant", 0, 0, 0, 0, 1, 200, 52, nil},
	{"Internet", 0, 0, 0, 0, 1, 250, 59, nil},
	{"Cloning Plant", 0, 0, 0, 0, 1, 250, 38, nil},
	{"Observation Installation", 0, 0, 0, 0, 1, 40, 20, nil},
	{"Tractor Beam", 0, 0, 0, 0, 1, 50, 34, []int{2}},
	{"Surface Shield", 0, 0, 0, 0, 1, 100, 4, []int{3}},
	{"Surface Mega Shield", 0, 0, 0, 0, 1, 180, 61, []int{3}},
	{"Outpost", 0, 0, 0, 1, 1, 120, 255, []int{6}},
	{"Transport Tubes", 0, 0, 0, 0, 1, 10, 255, []int{0, 6}},
	{"Shipyard", 1, 0, 0, 0, 1, 240, 0, nil},
	{"Ship", 0, 0, 0, 0, 1, 20, 1, nil},
	{"Orbital Docks", 1, 0, 0, 0, 1, 170, 16, nil},
	{"Orbital Cloaker", 0, 0, 0, 0, 1, 40, 15, nil},
	{"Orbital Shield", 0, 0, 0, 0, 1, 60, 0, []int{3}},
	{"Orbital Mega Shield", 0, 0, 0, 0, 1, 120, 43, []int{3}},
	{"Orbital Missile Base", 0, 0, 0, 0, 1, 60, 9, []int{2}},
	{"Short Range Orbital Whopper", 0, 0, 0, 0, 1, 90, 37, []int{2}},
	{"Long Range Orbital Whopper", 0, 0, 0, 0, 1, 180, 61, []int{2}},
	{"Alien Hospitality", 0, 0, 0, 0, 0, 1, 20, []int{1, 5}},
	{"Interstellar Trucking", 0, 0, 0, 0, 0, 1, 254, []int{1, 5}},
	{"Endless Party", 0, 0, 0, 0, 0, 1, 41, []int{1, 5}},
	{"Scientist Takeover", 0, 0, 0, 0, 0, 1, 32, []int{1, 5}},
	{"Automation", 0, 0, 0, 0, 1, 1, 51, []int{0}},
	{"Terraforming", 0, 0, 0, 0, 1, 50, 26, []int{0}},
	{"Lush Growth Bomb", 0, 0, 0, 0, 1, 200, 62, []int{1}},
	{"Xeno Archeological Dig", 0, 0, 0, 0, 1, 50, 6, []int{0}},
}

// Info returns the item's row.
func (it Item) Info() *ItemInfo { return &Items[it] }

// Staff is the population a built it employs: its UsePop, none for the
// FlagOnBlack items (Outpost, Tubes: checked in the original; Items keeps
// planitem.txt's 1; docs/rules.md).
func (it Item) Staff() int {
	if it.Has(FlagOnBlack) {
		return 0
	}
	return it.Info().UsePop
}

// Unique reports a structure of which one per planet has the whole
// effect (the guide's "no need to build more than 1 per planet"); a
// second adds nothing.
func (it Item) Unique() bool {
	switch it {
	case HyperpowerPlant, FertilizationPlant, Internet, CloningPlant, ObservationInstallation:
		return true
	}
	return false
}

// String is the item's display name.
func (it Item) String() string {
	if it < 0 || it >= NumItems {
		return "none"
	}
	return Items[it].Name
}

// Surface reports a structure built on a surface square.
func (it Item) Surface() bool { return it >= 0 && it < FirstOrbital }

// Orbital reports a structure built on an orbital slot.
func (it Item) Orbital() bool { return it >= FirstOrbital && it < FirstProject }

// Project reports a project, not a structure: a planet-wide one, or a
// Targeted one.
func (it Item) Project() bool { return it >= FirstProject && it < NumItems }

// Ongoing reports a modelled project that never completes, running until
// replaced or cancelled (Interstellar Trucking's flag 5 is not modelled).
func (it Item) Ongoing() bool {
	return it == EndlessParty || it == ScientistTakeover || it == AlienHospitality
}

// Targeted reports a project started at a cell (Colony.CanStart), not on
// the planet: Terraforming, Automation, the dig.
func (it Item) Targeted() bool {
	return it == Terraforming || it == Automation || it == XenoArcheologicalDig
}

// PlanetWide reports a project not Targeted.
func (it Item) PlanetWide() bool { return it.Project() && !it.Targeted() }

// OnSquare reports an item started on an empty surface square: the
// surface structures, Terraforming and the dig.
func (it Item) OnSquare() bool {
	return it.Surface() || it == Terraforming || it == XenoArcheologicalDig
}

// Has reports whether the item carries a flag.
func (it Item) Has(flag int) bool { return slices.Contains(Items[it].Flags, flag) }
