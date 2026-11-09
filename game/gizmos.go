package game

// gizmos.txt (cob0) lists the ship parts: a documented header, a "#" line,
// then per gizmo a name line (^ for spaces) and a number line
// "Cat Power Range Level NumUses Industry Special1 Special2 ResReq flags... 255",
// the flags being bit positions ended by 255. CRLF, ends with a ^Z byte.

// GizmoCat is a ship part's category, gizmos.txt's Cat column.
type GizmoCat uint8

const (
	Weapon GizmoCat = iota
	Shield
	Drive
	Scanner
	Generator
	Special
	NumGizmoCats
)

var gizmoCatNames = [NumGizmoCats]string{"Weapon", "Shield", "Drive", "Scanner", "Generator", "Special"}

func (c GizmoCat) String() string {
	if c >= NumGizmoCats {
		return "none"
	}
	return gizmoCatNames[c]
}

// Gizmo is a ship part, indexed as gizmos.txt lists them (the order of
// gizmos.flc). The gizmo Tractor Beam is GizmoTractorBeam: the item
// TractorBeam has the name.
type Gizmo uint8

const (
	MassBarrageGun Gizmo = iota
	FourierMissiles
	QuantumSingularityLauncher
	MolecularDisassociator
	ElectromagneticPulser
	Plasmatron
	Ueberlaser
	FergnatzLens
	HypersphereDriver
	Nanomanipulator
	IonWrap
	ConcussionShield
	WaveScatterer
	Deactotron
	HyperwaveNullifier
	Nanoshell
	TonklinMotor
	IonBanger
	GravitonProjector
	InertiaNegator
	NanowaveSpaceBender
	TonklinFrequencyAnalyzer
	SubspacePhaseArray
	AuralCloudConstructor
	HyperwaveTympanum
	MurgatroydsKnower
	NanowaveDecouplingNet
	ProtonShaver
	SubatomicScoop
	QuarkExpress
	VanCreegHypersplicer
	Nanotwirler
	LaneBlocker
	MolecularTieDown
	IntellectScrambler
	BrunswikDissipator
	Recaller
	Disarmer
	SmartBomb
	GravityDistorter
	FleetDisperser
	XRayMegaglasses
	Cloaker
	StarLaneDrive
	StarLaneHyperdrive
	PositronBouncer
	GravimetricCatapult
	MyrmidonicCarbonizer
	ContainmentDevice
	ShieldBlaster
	Backfirer
	LaneDestabilizer
	GizmoTractorBeam
	Cannibalizer
	MovingPartExploiter
	Hyperswapper
	GravimetricCondensor
	Accutron
	RemoteRepairFacility
	SacrificialOrb
	LaneMagnetron
	Disintegrator
	LaneEndoscope
	ToroidalBlaster
	Gizmogrifier
	Replenisher
	SpecialtyBlaster
	GyroInductor
	PlasmaCoupler
	Invulnerablizer
	PhaseBomb
	Colonizer
	SelfDestructotron
	InvasionModule
	MassCondensor
	Hyperfuel
	// SpaceHabitat is ours, not the game's: it carries Special1
	// population units between the owner's colonies (Empire.Load).
	SpaceHabitat
	NumGizmos // 77
)

// NumGameGizmos is the parts gizmos.txt lists; ours start there.
const NumGameGizmos = SpaceHabitat

// Gizmo flags, the header's GZ_ bit positions as masks.
const (
	GzTargetShip uint8 = 1 << iota
	GzTargetLane
	GzTargetOrbit
	GzTargetSurface
	GzTargetOwnShips
	GzToggleable
	GzNoTarget
	GzPassive
)

// GizmoInfo is one entry of gizmos.txt. The column meanings are the
// header's; "specials" use them as their own code sees fit.
type GizmoInfo struct {
	Name string
	Cat  GizmoCat
	// Power: for weapons, shields, drives, scanners and most specials the
	// cost of operating the device; for generators the power supplied.
	Power int
	// Range: for weapons and scanners, the maximum range.
	Range int
	// Level: for weapons damage, for shields strength, for scanners range
	// per turn, for drives the maximum distance travelled (sugar cube units).
	Level int
	// Uses per turn, 0 for no limit.
	Uses int
	// Industry added to the cost of building a ship. The header warns the
	// original altered these as read in (Ship::init_global_gizmos()).
	Industry int
	// Special1, Special2: context dependent, per gizmo.
	Special1, Special2 int
	ResearchReq        Tech  // the research node allowing it, NoTech for none
	Flags              uint8 // Gz* bits
}

// Gizmos is gizmos.txt, typed, then ours (from NumGameGizmos);
// TestGizmoTableMatchesGame checks the game's against the installed game.
var Gizmos = [NumGizmos]GizmoInfo{
	MassBarrageGun:             {"Mass Barrage Gun", Weapon, 1, 25, 1, 1, 15, 0, 0, Superconductivity, GzTargetShip | GzTargetLane | GzTargetOrbit},
	FourierMissiles:            {"Fourier Missiles", Weapon, 1, 40, 2, 1, 25, 0, 0, SpectralAnalysis, GzTargetShip | GzTargetLane | GzTargetOrbit},
	QuantumSingularityLauncher: {"Quantum Singularity Launcher", Weapon, 2, 25, 4, 1, 40, 0, 0, GravityControl, GzTargetShip | GzTargetLane | GzTargetOrbit},
	MolecularDisassociator:     {"Molecular Disassociator", Weapon, 2, 50, 4, 1, 50, 0, 0, StrongForceWeakening, GzTargetShip | GzTargetLane | GzTargetOrbit},
	ElectromagneticPulser:      {"Electromagnetic Pulser", Weapon, 1, 50, 1, 5, 60, 0, 0, EMFieldCoupling, GzTargetShip | GzTargetLane | GzTargetOrbit},
	Plasmatron:                 {"Plasmatron", Weapon, 2, 100, 4, 1, 70, 0, 0, Plasmatics, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	Ueberlaser:                 {"Ueberlaser", Weapon, 3, 50, 6, 2, 70, 0, 0, CoherentPhotonics, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	FergnatzLens:               {"Fergnatz Lens", Weapon, 0, 35, 4, 2, 70, 0, 0, FergnatzsLastTheorem, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	HypersphereDriver:          {"Hypersphere Driver", Weapon, 6, 75, 10, 2, 100, 0, 0, Hypergeometry, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	Nanomanipulator:            {"Nanomanipulator", Weapon, 6, 50, 13, 3, 100, 0, 0, Nanofocusing, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	IonWrap:                    {"Ion Wrap", Shield, 1, 0, 1, 0, 10, 0, 0, EnvironmentalEncapsulation, GzToggleable},
	ConcussionShield:           {"Concussion Shield", Shield, 2, 0, 2, 0, 30, 0, 0, MomentumDeconservation, GzToggleable},
	WaveScatterer:              {"Wave Scatterer", Shield, 0, 0, 1, 0, 50, 0, 0, LightBending, GzToggleable},
	Deactotron:                 {"Deactotron", Shield, 2, 0, 3, 0, 60, 0, 0, EnergyRedirection, GzToggleable},
	HyperwaveNullifier:         {"Hyperwave Nullifier", Shield, 4, 0, 4, 0, 100, 0, 0, HyperwaveTechnology, GzToggleable},
	Nanoshell:                  {"Nanoshell", Shield, 3, 0, 5, 0, 200, 0, 0, Nanodeflection, GzToggleable},
	TonklinMotor:               {"Tonklin Motor", Drive, 1, 0, 2, 0, 10, 0, 0, TonklinDiary, GzPassive},
	IonBanger:                  {"Ion Banger", Drive, 1, 0, 4, 0, 30, 0, 0, AdvancedChemistry, GzPassive},
	GravitonProjector:          {"Graviton Projector", Drive, 3, 0, 6, 0, 40, 0, 0, GravimetricCombustion, GzPassive},
	InertiaNegator:             {"Inertia Negator", Drive, 1, 0, 6, 0, 20, 0, 0, InertialControl, GzPassive},
	NanowaveSpaceBender:        {"Nanowave Space Bender", Drive, 5, 0, 10, 0, 80, 0, 0, Nanopropulsion, GzPassive},
	TonklinFrequencyAnalyzer:   {"Tonklin Frequency Analyzer", Scanner, 0, 25, 1, 0, 20, 0, 0, TonklinDiary, GzPassive},
	SubspacePhaseArray:         {"Subspace Phase Array", Scanner, 0, 50, 1, 0, 40, 0, 0, AdvancedInterferometry, GzPassive},
	AuralCloudConstructor:      {"Aural Cloud Constructor", Scanner, 0, 75, 2, 0, 60, 0, 0, EMFieldCoupling, GzPassive},
	HyperwaveTympanum:          {"Hyperwave Tympanum", Scanner, 0, 100, 3, 0, 80, 0, 0, Hyperradiation, GzPassive},
	MurgatroydsKnower:          {"Murgatroyd's Knower", Scanner, 0, 200, 4, 0, 100, 0, 0, MurgatroydHypothesis, GzPassive},
	NanowaveDecouplingNet:      {"Nanowave Decoupling Net", Scanner, 0, 1000, 5, 0, 200, 0, 0, Nanoenergons, GzPassive},
	ProtonShaver:               {"Proton Shaver", Generator, 2, 0, 1, 0, 20, 0, 0, InterplanetaryExploration, GzPassive},
	SubatomicScoop:             {"Subatomic Scoop", Generator, 4, 0, 2, 0, 35, 0, 0, PowerConversion, GzPassive},
	QuarkExpress:               {"Quark Express", Generator, 6, 0, 3, 0, 60, 0, 0, Subatomics, GzPassive},
	VanCreegHypersplicer:       {"Van Creeg Hypersplicer", Generator, 8, 0, 4, 0, 80, 0, 0, Hyperradiation, GzPassive},
	Nanotwirler:                {"Nanotwirler", Generator, 10, 0, 5, 0, 100, 0, 0, Nanoenergons, GzPassive},
	LaneBlocker:                {"Lane Blocker", Special, 3, 40, 0, 1, 30, 0, 0, StarLaneAnatomy, GzTargetLane},
	MolecularTieDown:           {"Molecular Tie Down", Special, 7, 50, 0, 1, 20, 0, 0, Gravimetrics, GzTargetShip},
	IntellectScrambler:         {"Intellect Scrambler", Special, 7, 50, 0, 1, 20, 0, 0, Hyperlogic, GzTargetShip},
	BrunswikDissipator:         {"Brunswik Dissipator", Special, 15, 50, 0, 1, 100, 0, 0, StasisFieldScience, GzTargetShip},
	Recaller:                   {"Recaller", Special, 7, 0, 0, 0, 40, 0, 0, EnergyRedirection, GzNoTarget},
	Disarmer:                   {"Disarmer", Special, 5, 60, 0, 1, 30, 0, 0, MatterDuplication, GzTargetShip},
	SmartBomb:                  {"Smart Bomb", Special, 7, 0, 7, 0, 30, 15, 0, ScientificSorcery, GzNoTarget},
	GravityDistorter:           {"Gravity Distorter", Special, 4, 0, 0, 1, 20, 65, 0, MomentumReflection, GzNoTarget},
	FleetDisperser:             {"Fleet Disperser", Special, 5, 120, 0, 1, 30, 45, 0, RepulsionBeamTech, GzTargetShip},
	XRayMegaglasses:            {"X Ray Megaglasses", Special, 3, 120, 0, 0, 30, 0, 0, Hyperlogic, GzPassive},
	Cloaker:                    {"Cloaker", Special, 3, 0, 0, 0, 30, 0, 0, Cloaking, GzPassive},
	StarLaneDrive:              {"Star Lane Drive", Special, 0, 0, 1, 0, 25, 0, 0, SpacetimeSurfing, GzPassive},
	StarLaneHyperdrive:         {"Star Lane Hyperdrive", Special, 0, 0, 2, 0, 50, 0, 0, AdvancedExploration, GzPassive},
	PositronBouncer:            {"Positron Bouncer", Special, 2, 60, 0, 1, 10, 100, 0, PositronGuidance, GzTargetShip},
	GravimetricCatapult:        {"Gravimetric Catapult", Special, 3, 0, 0, 1, 15, 0, 0, MassPhasing, GzNoTarget},
	MyrmidonicCarbonizer:       {"Myrmidonic Carbonizer", Special, 4, 40, 20, 1, 70, 20, 0, EnergyFocusing, GzTargetShip | GzTargetLane | GzTargetOrbit | GzTargetSurface},
	ContainmentDevice:          {"Containment Device", Special, 5, 50, 0, 1, 50, 0, 0, ScientificSorcery, GzTargetShip},
	ShieldBlaster:              {"Shield Blaster", Special, 7, 50, 0, 1, 30, 0, 0, Teleinfiltration, GzTargetShip},
	Backfirer:                  {"Backfirer", Special, 10, 50, 0, 1, 60, 0, 0, HyperwaveEmissionControl, GzTargetShip},
	LaneDestabilizer:           {"Lane Destabilizer", Special, 7, 40, 0, 1, 40, 0, 0, StarLaneAnatomy, GzTargetLane},
	GizmoTractorBeam:           {"Tractor Beam", Special, 3, 120, 0, 1, 30, 65, 0, StasisFieldScience, GzTargetShip},
	Cannibalizer:               {"Cannibalizer", Special, 0, 0, 0, 1, 20, 5, 20, CoherentPhotonics, GzNoTarget},
	MovingPartExploiter:        {"Moving Part Exploiter", Special, 10, 50, 0, 1, 60, 0, 0, ActionAtADistance, GzTargetShip},
	Hyperswapper:               {"Hyperswapper", Special, 3, 120, 0, 1, 20, 0, 0, Hypergeometry, GzTargetShip},
	GravimetricCondensor:       {"Gravimetric Condensor", Special, 5, 0, 0, 1, 30, 65, 10, GravityFlowControl, GzNoTarget},
	Accutron:                   {"Accutron", Special, 7, 0, 0, 1, 60, 0, 0, EnergyFocusing, GzNoTarget},
	RemoteRepairFacility:       {"Remote Repair Facility", Special, 30, 0, 0, 0, 70, 5, 0, SelfModifyingStructures, GzNoTarget},
	SacrificialOrb:             {"Sacrificial Orb", Special, 3, 70, 0, 0, 20, 6, 0, EnergyRedirection, GzTargetShip},
	LaneMagnetron:              {"Lane Magnetron", Special, 10, 50, 0, 1, 50, 0, 0, HyperdriveTechnology, GzTargetLane},
	Disintegrator:              {"Disintegrator", Special, 25, 25, 0, 1, 150, 0, 0, DoomMechanization, GzTargetShip},
	LaneEndoscope:              {"Lane Endoscope", Special, 10, 50, 0, 0, 20, 0, 0, Snooping, GzTargetLane},
	ToroidalBlaster:            {"Toroidal Blaster", Special, 0, 0, 0, 1, 20, 40, 0, GravimetricCombustion, GzNoTarget},
	Gizmogrifier:               {"Gizmogrifier", Special, 4, 60, 0, 1, 30, 0, 0, FergnatzsLastTheorem, GzTargetShip},
	Replenisher:                {"Replenisher", Special, 8, 0, 0, 1, 60, 0, 0, LightBending, GzNoTarget},
	SpecialtyBlaster:           {"Specialty Blaster", Special, 5, 60, 0, 1, 30, 0, 0, Teleinfiltration, GzTargetShip},
	GyroInductor:               {"Gyro-Inductor", Special, 0, 0, 0, 1, 20, 10, 0, MurgatroydHypothesis, GzPassive},
	PlasmaCoupler:              {"Plasma Coupler", Special, 4, 70, 0, 1, 20, 0, 0, Plasmatics, GzTargetShip},
	Invulnerablizer:            {"Invulnerablizer", Special, 20, 0, 0, 1, 60, 0, 0, IllusoryMachinations, GzNoTarget},
	PhaseBomb:                  {"Phase Bomb", Special, 4, 25, 0, 1, 40, 0, 0, MolecularExplosives, GzTargetSurface},
	Colonizer:                  {"Colonizer", Special, 5, 0, 0, 0, 35, 0, 0, EnvironmentalEncapsulation, GzPassive},
	SelfDestructotron:          {"Self Destructotron", Special, 0, 40, 20, 0, 50, 20, 0, DoomMechanization, GzNoTarget},
	InvasionModule:             {"Invasion Module", Special, 10, 0, 0, 0, 70, 0, 0, AdvancedInterferometry, GzPassive},
	MassCondensor:              {"Mass Condensor", Special, 8, 120, 0, 1, 50, 50, 5, MolecularExplosives, GzTargetShip},
	Hyperfuel:                  {"Hyperfuel", Special, 0, 0, 0, 0, 20, 40, 0, SuperstringCompression, GzNoTarget},
	SpaceHabitat:               {"Space Habitat", Special, 0, 0, 0, 0, 50, 3, 0, PlanetaryReplenishment, GzPassive},
}

// String is the gizmo's display name.
func (g Gizmo) String() string {
	if g >= NumGizmos {
		return "none"
	}
	return Gizmos[g].Name
}
