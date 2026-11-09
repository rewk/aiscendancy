package bot

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/rewk/aiscendancy/game"
)

// Genome is a bot's tunable weights (Config.Genome; nil plays the fixed
// policy). It draws nothing: the same genome on the same game plays the
// same days; only cmd/fuzz draws genomes. The bounds are Clamp's.
type Genome struct {
	// Build is each item's priority for the greedy structure pick; an
	// item at Build <= 0 is never built by it.
	Build [game.NumItems]float64
	// Tech is each tech's priority for the research pick; a tech at
	// Tech <= 0 is studied only when nothing better is available.
	Tech [game.NumTechs]float64

	Balance  float64 // structure pick: bonus for producing the output the mix wants (wanted)
	CostW    float64 // structure pick: penalty per 100 industry of cost
	ResCostW float64 // research pick: penalty per natural log of the tech's cost

	ShipyardFactories float64 // Factories standing before a Shipyard (rounded)
	FrontierWorth     float64 // open places an unexplored frontier star counts for (rounded)
	FoundFloor        float64 // Size under which a planet is not founded while a bigger settleable one is known (rounded)
	InFlight          float64 // Colonizers in flight (aboard, or in colony ships being built) past which no colony ship starts, the first excepted (rounded)
	AbandonIdle       float64 // days a colony or invasion ship stands idle before it is abandoned, 0 never (rounded)

	// Combat. HullThreat is the damage per day assumed of an unscanned
	// foreign ship, by hull size.
	HullThreat [game.NumHulls]float64
	// Above pressure FleeRatio my ships flee; an armed ship attacks a
	// star of pressure below AttackRatio (pressure: combat.go).
	FleeRatio    float64
	AttackRatio  float64
	ThreatAvoid  float64 // route: extra lane days per point of threat (damage per day) seen at a star
	WarshipShare float64 // warships wanted per colony
	DefendW      float64 // a warship is built when the threat next to a colony exceeds DefendW × its defence

	// Round tactics. HullReach is the move plus weapon range assumed of
	// an unscanned foreign ship, in Pos units, by hull size.
	HullReach [game.NumHulls]float64
	Standoff  float64 // a fighting ship stands at Standoff × its longest affordable range from its target
	Rounds    float64 // rounds played per combat star per day (rounded), at most RoundCap

	// Refit (wantRefit): when today's warship on the hull deals at least
	// (1 + RefitGain) × the ship's damage; a repair below RepairAt × MaxHull.
	RefitGain float64
	RepairAt  float64

	RuinWorth float64 // lane days nearer a star with a foundable ruins planet counts in route (a dig is a free tech)
	IdleParty float64 // room left, (Capacity − Pop) / Capacity, above which an idle colony parties, else Scientist Takeover
	// AutomateCostW ranks the structures to automate by AutomateCostW ×
	// Cost, the least first: 0 is Cells order, positive the cheapest
	// first, negative the dearest (cheapest-first measured worse: the GA
	// decides, docs/rules.md).
	AutomateCostW float64

	InvadeMargin float64 // Invasion Modules built past the fewest that beat the shields (rounded)
	InvadeShare  float64 // invasion ships wanted per armed warship
	// Defences (fortify), only at War: a colony of Pop ≥ ShieldPop raises
	// its surface shields' worth (game.ShieldWorth's units) to
	// ShieldWorth, then OrbitalShields orbital shields, OrbitalGuns
	// orbital weapons, OrbitalCloakers Orbital Cloakers, SurfaceCloakers
	// Surface Cloakers; all rounded, 0 skips a step.
	ShieldWorth     float64
	ShieldPop       float64
	OrbitalShields  float64
	OrbitalGuns     float64
	OrbitalCloakers float64
	SurfaceCloakers float64

	// Diplomacy: attitude = Start − grudge − envy + weariness + goodwill,
	// read against the species' thresholds.
	Temper        float64 // added to the six thresholds (Personality; not Start, not ShipPatience)
	EnvyW         float64 // attitude lost per system the other holds beyond mine
	GrudgeShip    float64 // grudge per ship of mine they destroyed
	GrudgePlanet  float64 // grudge per colony of mine they invaded
	GrudgeWar     float64 // grudge per war they declared on me
	GrudgeRefusal float64 // grudge per offer of mine they refused
	IntrudeW      float64 // grudge per armed-ship-day of theirs in my systems, × 50 / ShipPatience
	Forgive       float64 // the share of the grudge that fades each day
	WarCaution    float64 // war only when my armed ships ≥ WarCaution × theirs seen (and at least one)
	WearyW        float64 // war weariness: attitude gained per day at war, fading by Forgive ("war has no value to us")
	// Goodwill (fading by Forgive), exchanges, Alien Hospitality.
	TradeW      float64 // goodwill per knowledge exchange concluded with them
	HelpW       float64 // goodwill per request of mine they granted (Relation.Helped)
	HospW       float64 // goodwill per Alien Hospitality point they poured toward me
	GrudgeBreak float64 // grudge per alliance they broke with me
	TradeFair   float64 // their exchange accepted when my worth ≥ TradeFair × theirs (worth: stars + lanes + 1 for a tech)
	TradeZeal   float64 // days between my exchange proposals to one empire (rounded)
	Hospitable  float64 // the share of my colonies an idle one may run Alien Hospitality within (idle)
	// Third-party requests, defamation.
	FavourW float64 // an ally's request: the thresholds toward the third party bent by FavourW its way (Decide)
	AskZeal float64 // days between my requests to one ally (rounded)
	DefameW float64 // grudge per defamation of them I heard (Relation.Defamed)
	// GrudgeEncroach is the grudge per colony they founded in a system
	// where I hold one (Relation.Encroached).
	GrudgeEncroach float64
	// Rage: under DeclareWar − Rage the war waits for no parity (warReady).
	Rage float64
	// Timed abilities.
	FreeResearchDays float64 // the Chamachies finish a topic when it has at least this many days left (rounded)
	// Ship specials: shares of the ship's MaxHull but SmartBombAt.
	CannibalAt  float64 // a Cannibalizer is used only while Hull > CannibalAt × MaxHull
	ShieldAt    float64 // an Invulnerablizer is used under fire at Hull ≤ ShieldAt × MaxHull (the Sacrificial Orb's too)
	RecallAt    float64 // a Recaller is used under fire at Hull ≤ RecallAt × MaxHull
	SmartBombAt float64 // a Smart Bomb is used with at least SmartBombAt hostile ships within its blast
	// Replacement (replacePick): a structure is replaced by a better one
	// when gain × ReplaceDays ≥ loss; ≤ 0 never. ReplaceFull ≥ 0.5 waits
	// for the planet to be full (no buildable cell), below replaces
	// whenever idle.
	ReplaceDays float64
	ReplaceFull float64
	// ShieldFirst (Round's shieldSwitch): a fighting ship whose energy
	// with its shields' reservation held cannot pay the shots it wants
	// keeps the shields at ≥ 0.5, turns them off (the draw refunded)
	// below.
	ShieldFirst float64
	// Economy (Economy, wanted, upgradeOpen): a family's upper tier
	// waits for Upgrade structures of its family (rounded); the
	// structure pick steers the outputs toward IndShare industry,
	// ResShare research, the rest prosperity, industry first while under
	// IndFloor.
	Upgrade  float64
	IndShare float64
	ResShare float64
	IndFloor float64
	// Population transport (bot/habitat.go): HabitatShare Habitat ships
	// wanted per ship-load of min(the donors' free people, the
	// recipients' room), 0 builds none; a colony at Pop ≤ HabitatFill ×
	// Capacity is a recipient.
	HabitatShare float64
	HabitatFill  float64
}

// Neutral is the genome near the fixed policy, where a search starts:
// Build 1 for the GreedyItems, Tech by rank in GuideResearch (the head
// highest), 0 elsewhere; the combat, diplomacy and economy genes a nil
// genome plays (the Habitat's too).
func Neutral() *Genome {
	g := &Genome{Balance: 1, CostW: 0.5, ResCostW: 1, ShipyardFactories: 2, FrontierWorth: frontierWorth,
		FoundFloor: foundFloor, InFlight: inFlightCap, AbandonIdle: abandonIdle, RuinWorth: ruinWorth,
		HabitatShare: habitatShare, HabitatFill: habitatFill}
	g.setCombat(NeutralCombat())
	g.setDiplomacy(NeutralDiplomacy())
	g.setEconomy(NeutralEconomy())
	for _, it := range GreedyItems {
		g.Build[it] = 1
	}
	for i, t := range GuideResearch {
		g.Tech[t] = float64(len(GuideResearch) - i)
	}
	return g
}

// Combat is the combat genes alone: a nil genome plays NeutralCombat.
type Combat struct {
	HullThreat                                                 [game.NumHulls]float64
	FleeRatio, AttackRatio, ThreatAvoid, WarshipShare, DefendW float64
	HullReach                                                  [game.NumHulls]float64
	Standoff, Rounds                                           float64
	RefitGain, RepairAt                                        float64
	InvadeMargin, InvadeShare                                  float64
	CannibalAt, ShieldAt, RecallAt, SmartBombAt                float64
	ShieldFirst                                                float64
	ShieldWorth, ShieldPop, OrbitalShields, OrbitalGuns        float64
	OrbitalCloakers, SurfaceCloakers                           float64
}

// NeutralCombat is the combat genes a nil genome plays. One module to
// spare: a Surface Shield built during the trip still falls; an invasion
// ship per two warships: they clear the guard first. Defences from
// Pop 6: two Surface Shields or one Mega (worth 4, a two-module
// invasion falls), one orbital shield (the guard: no orbit, no
// invasion), one orbital weapon, one cloaker of each kind.
func NeutralCombat() Combat {
	c := Combat{HullThreat: [game.NumHulls]float64{2, 4, 6, 8}, FleeRatio: 1, AttackRatio: 0.5, ThreatAvoid: 2, WarshipShare: 0.1, DefendW: 1,
		Standoff: 0.9, Rounds: RoundCap, RefitGain: 0.25, RepairAt: 0.5, InvadeMargin: 1, InvadeShare: 0.5,
		CannibalAt: 0.5, ShieldAt: 0.5, RecallAt: 0.25, SmartBombAt: 2, ShieldFirst: 1,
		ShieldWorth: 4, ShieldPop: 6, OrbitalShields: 1, OrbitalGuns: 1, OrbitalCloakers: 1, SurfaceCloakers: 1}
	for i := range c.HullReach {
		c.HullReach[i] = neutralHullReach
	}
	return c
}

// neutralHullReach is the first parts research gives (the fixture
// Raider's): a Tonklin Motor's move plus the Mass Barrage Gun's range,
// 0.2 + 0.25; every hull alike (more slots buy guns, not range).
var neutralHullReach = game.Design{Parts: []game.Gizmo{game.TonklinMotor}}.Reach() +
	float64(game.Gizmos[game.MassBarrageGun].Range)*game.RangeUnit

// Combat is g's combat genes.
func (g *Genome) Combat() Combat {
	return Combat{g.HullThreat, g.FleeRatio, g.AttackRatio, g.ThreatAvoid, g.WarshipShare, g.DefendW, g.HullReach, g.Standoff, g.Rounds,
		g.RefitGain, g.RepairAt, g.InvadeMargin, g.InvadeShare, g.CannibalAt, g.ShieldAt, g.RecallAt, g.SmartBombAt, g.ShieldFirst,
		g.ShieldWorth, g.ShieldPop, g.OrbitalShields, g.OrbitalGuns, g.OrbitalCloakers, g.SurfaceCloakers}
}

func (g *Genome) setCombat(c Combat) {
	g.HullThreat, g.FleeRatio, g.AttackRatio, g.ThreatAvoid, g.WarshipShare, g.DefendW =
		c.HullThreat, c.FleeRatio, c.AttackRatio, c.ThreatAvoid, c.WarshipShare, c.DefendW
	g.HullReach, g.Standoff, g.Rounds = c.HullReach, c.Standoff, c.Rounds
	g.RefitGain, g.RepairAt = c.RefitGain, c.RepairAt
	g.InvadeMargin, g.InvadeShare = c.InvadeMargin, c.InvadeShare
	g.CannibalAt, g.ShieldAt, g.RecallAt, g.SmartBombAt = c.CannibalAt, c.ShieldAt, c.RecallAt, c.SmartBombAt
	g.ShieldFirst = c.ShieldFirst
	g.ShieldWorth, g.ShieldPop, g.OrbitalShields, g.OrbitalGuns = c.ShieldWorth, c.ShieldPop, c.OrbitalShields, c.OrbitalGuns
	g.OrbitalCloakers, g.SurfaceCloakers = c.OrbitalCloakers, c.SurfaceCloakers
}

// Diplomacy is the diplomacy genes alone: a nil genome plays
// NeutralDiplomacy.
type Diplomacy struct {
	Temper, EnvyW                                      float64
	GrudgeShip, GrudgePlanet, GrudgeWar, GrudgeRefusal float64
	IntrudeW, Forgive, WarCaution, WearyW              float64
	TradeW, HospW, GrudgeBreak, TradeFair              float64
	TradeZeal, Hospitable                              float64
	FavourW, AskZeal, DefameW                          float64
	FreeResearchDays                                   float64
	GrudgeEncroach, Rage                               float64
	HelpW                                              float64
}

// NeutralDiplomacy is the diplomacy genes a nil genome plays. Weariness
// 1 a day fades to 100 at Forgive 1%: a species whose ProposePeace sits
// within 100 of Start makes peace by itself; a steady trader's goodwill
// reaches the 1000 clamp.
func NeutralDiplomacy() Diplomacy {
	return Diplomacy{Temper: 0, EnvyW: 10, GrudgeShip: 40, GrudgePlanet: 150, GrudgeWar: 100, GrudgeRefusal: 20,
		IntrudeW: 2, Forgive: 0.01, WarCaution: 1, WearyW: 1,
		TradeW: 30, HospW: 0.2, GrudgeBreak: 150, TradeFair: 0.5, TradeZeal: 60, Hospitable: 0.1,
		FavourW: 100, AskZeal: 90, DefameW: 100, FreeResearchDays: 30, GrudgeEncroach: 100, Rage: 100,
		HelpW: 30}
}

// Diplomacy is g's diplomacy genes.
func (g *Genome) Diplomacy() Diplomacy {
	return Diplomacy{g.Temper, g.EnvyW, g.GrudgeShip, g.GrudgePlanet, g.GrudgeWar, g.GrudgeRefusal, g.IntrudeW, g.Forgive, g.WarCaution, g.WearyW,
		g.TradeW, g.HospW, g.GrudgeBreak, g.TradeFair, g.TradeZeal, g.Hospitable, g.FavourW, g.AskZeal, g.DefameW, g.FreeResearchDays, g.GrudgeEncroach, g.Rage, g.HelpW}
}

func (g *Genome) setDiplomacy(d Diplomacy) {
	g.Temper, g.EnvyW = d.Temper, d.EnvyW
	g.GrudgeShip, g.GrudgePlanet, g.GrudgeWar, g.GrudgeRefusal = d.GrudgeShip, d.GrudgePlanet, d.GrudgeWar, d.GrudgeRefusal
	g.IntrudeW, g.Forgive, g.WarCaution, g.WearyW = d.IntrudeW, d.Forgive, d.WarCaution, d.WearyW
	g.TradeW, g.HospW, g.GrudgeBreak, g.TradeFair, g.TradeZeal, g.Hospitable = d.TradeW, d.HospW, d.GrudgeBreak, d.TradeFair, d.TradeZeal, d.Hospitable
	g.FavourW, g.AskZeal, g.DefameW = d.FavourW, d.AskZeal, d.DefameW
	g.FreeResearchDays, g.GrudgeEncroach, g.Rage = d.FreeResearchDays, d.GrudgeEncroach, d.Rage
	g.HelpW = d.HelpW
}

// Clamp brings every gene into its bounds, a NaN to the lower one. Then
// AttackRatio is at most FleeRatio: a star attack picks is never one the
// ship would flee on arrival.
func (g *Genome) Clamp() {
	for i := range g.Build {
		g.Build[i] = clamp(g.Build[i], -10, 10)
	}
	for i := range g.Tech {
		g.Tech[i] = clamp(g.Tech[i], -10, 10)
	}
	g.Balance = clamp(g.Balance, 0, 10)
	g.CostW = clamp(g.CostW, 0, 10)
	g.ResCostW = clamp(g.ResCostW, 0, 10)
	g.FrontierWorth = clamp(g.FrontierWorth, 0, 10)
	g.ShipyardFactories = clamp(g.ShipyardFactories, 0, 6)
	g.FoundFloor = clamp(g.FoundFloor, 1, game.MaxSize)
	g.InFlight = clamp(g.InFlight, 1, 50)
	g.AbandonIdle = clamp(g.AbandonIdle, 0, 1000)
	g.RuinWorth = clamp(g.RuinWorth, 0, 63)
	g.IdleParty = clamp(g.IdleParty, 0, 1)
	g.AutomateCostW = clamp(g.AutomateCostW, -1, 1)
	for i := range g.HullThreat {
		g.HullThreat[i] = clamp(g.HullThreat[i], 0, 50)
	}
	g.FleeRatio = clamp(g.FleeRatio, 0.05, 10)
	g.AttackRatio = clamp(g.AttackRatio, 0.05, 10)
	g.ThreatAvoid = clamp(g.ThreatAvoid, 0, 21)
	g.DefendW = clamp(g.DefendW, 0, 10)
	g.WarshipShare = clamp(g.WarshipShare, 0, 2)
	for i := range g.HullReach {
		g.HullReach[i] = clamp(g.HullReach[i], 0, 5)
	}
	g.Standoff = clamp(g.Standoff, 0.1, 1)
	g.Rounds = clamp(g.Rounds, 1, RoundCap)
	g.RefitGain = clamp(g.RefitGain, 0, 4)
	g.RepairAt = clamp(g.RepairAt, 0, 1)
	g.InvadeMargin = clamp(g.InvadeMargin, 0, 10)
	g.InvadeShare = clamp(g.InvadeShare, 0, 4)
	g.ShieldWorth = clamp(g.ShieldWorth, 0, 10)
	g.ShieldPop = clamp(g.ShieldPop, 2, 20)
	g.OrbitalShields = clamp(g.OrbitalShields, 0, 4)
	g.OrbitalGuns = clamp(g.OrbitalGuns, 0, 4)
	g.OrbitalCloakers = clamp(g.OrbitalCloakers, 0, 1)
	g.SurfaceCloakers = clamp(g.SurfaceCloakers, 0, 1)
	g.Temper = clamp(g.Temper, -300, 300)
	g.EnvyW = clamp(g.EnvyW, 0, 100)
	g.GrudgeShip = clamp(g.GrudgeShip, 0, 400)
	g.GrudgePlanet = clamp(g.GrudgePlanet, 0, 1000)
	g.GrudgeWar = clamp(g.GrudgeWar, 0, 1000)
	g.GrudgeRefusal = clamp(g.GrudgeRefusal, 0, 200)
	g.IntrudeW = clamp(g.IntrudeW, 0, 50)
	g.Forgive = clamp(g.Forgive, 0, 0.2)
	g.WarCaution = clamp(g.WarCaution, 0, 5)
	g.WearyW = clamp(g.WearyW, 0, 20)
	g.TradeW = clamp(g.TradeW, 0, 200)
	g.HelpW = clamp(g.HelpW, 0, 200)
	g.HospW = clamp(g.HospW, 0, 2)
	g.GrudgeBreak = clamp(g.GrudgeBreak, 0, 1000)
	g.TradeFair = clamp(g.TradeFair, 0, 3)
	g.TradeZeal = clamp(g.TradeZeal, 10, 400)
	g.Hospitable = clamp(g.Hospitable, 0, 1)
	g.FavourW = clamp(g.FavourW, 0, 500)
	g.AskZeal = clamp(g.AskZeal, 10, 400)
	g.DefameW = clamp(g.DefameW, 0, 1000)
	g.GrudgeEncroach = clamp(g.GrudgeEncroach, 0, 1000)
	g.Rage = clamp(g.Rage, 0, 1000)
	g.FreeResearchDays = clamp(g.FreeResearchDays, 1, 400)
	g.CannibalAt = clamp(g.CannibalAt, 0, 1)
	g.ShieldAt = clamp(g.ShieldAt, 0, 1)
	g.RecallAt = clamp(g.RecallAt, 0, 1)
	g.SmartBombAt = clamp(g.SmartBombAt, 1, 10)
	g.ReplaceDays = clamp(g.ReplaceDays, 0, 400)
	g.ReplaceFull = clamp(g.ReplaceFull, 0, 1)
	g.ShieldFirst = clamp(g.ShieldFirst, 0, 1)
	g.Upgrade = clamp(g.Upgrade, 0, 20)
	g.IndShare = clamp(g.IndShare, 0, 1)
	g.ResShare = clamp(g.ResShare, 0, 1)
	g.IndFloor = clamp(g.IndFloor, 0, 30)
	g.HabitatShare = clamp(g.HabitatShare, 0, 4)
	g.HabitatFill = clamp(g.HabitatFill, 0, 1)
	if s := g.IndShare + g.ResShare; s > 1 { // prosperity's share never below 0
		g.IndShare, g.ResShare = g.IndShare/s, g.ResShare/s
	}
	g.AttackRatio = min(g.AttackRatio, g.FleeRatio)
}

func clamp(x, lo, hi float64) float64 {
	if math.IsNaN(x) {
		return lo
	}
	return min(hi, max(lo, x))
}

// ReadGenome reads a genome written by WriteGenome, clamped; a gene the
// file lacks plays Neutral, but HabitatShare and HabitatFill read 0 (a
// genome from before the Habitat builds none).
func ReadGenome(path string) (*Genome, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	g := Neutral()
	g.HabitatShare, g.HabitatFill = 0, 0
	if err := json.Unmarshal(b, g); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	g.Clamp()
	return g, nil
}

// WriteGenome writes g as indented JSON.
func WriteGenome(path string, g *Genome) error {
	b, err := json.MarshalIndent(g, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
