package game

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSpecies reads a species from its index (0..20) or its name, case,
// spaces, underscores, hyphens and apostrophes ignored.
func ParseSpecies(s string) (Species, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 || n >= int(NumSpecies) {
			return 0, fmt.Errorf("species %d out of 0..%d", n, NumSpecies-1)
		}
		return Species(n), nil
	}
	for sp := range NumSpecies {
		if foldName(sp.String()) == foldName(s) {
			return sp, nil
		}
	}
	return 0, fmt.Errorf("unknown species %q", s)
}

// foldName lowers a name and drops its separators, for a loose match.
func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '_', '-', '\'':
			return -1
		}
		return r
	}, strings.ToLower(s))
}

// Species is one of the 21 playable species, indexed as race00-20.dip and
// help.txt's abil00-20 (the fan manual's order). Checked against the game
// by TestSpeciesMatchGame: raceNN.dip's first line (race02 has no SA_,
// race03 spells KAMBUCHA), help.txt's "<Name> Special Ability" (an
// abresNN section only for the timed ones), newgame.txt's descriptions.
// AbilityText, Recharge and the home world are the fan manual's
// (b-sting.nl species.html).
type Species uint8

const (
	Minions Species = iota
	Snovemdomas
	Orfa
	Kambuchka
	Hanshaks
	Fludentri
	Baliflids
	Swaparamans
	Frutmaka
	Shevar
	Govorom
	Ungooma
	Dubtaks
	Capelons
	Mebes
	Oculons
	Arbryls
	Marmosians
	Chronomyst
	Chamachies
	Nimbuloids
	NumSpecies
)

// SpeciesInfo is one species' line of the table.
type SpeciesInfo struct {
	Name        string
	Ability     string // the SA_ identifier, e.g. "SA_SuperMarines"
	AbilityText string // the manual's one-line description
	Recharge    int    // days between uses, 0 for a passive ability
	HomeSize    int    // MinSize..MaxSize
	HomeType    PlanetType
	Description string // newgame.txt's line, verbatim ("are a mysterious group...")
}

// SpeciesTable is indexed by Species.
var SpeciesTable = [NumSpecies]SpeciesInfo{
	Minions: {Name: "Minions", Ability: "SA_SuperMarines", AbilityText: "Planetary invasions always succeed", Recharge: 0, HomeSize: 5, HomeType: Supermineral,
		Description: "are a mysterious group of robot creatures controlled by an unknown extra-galactic super force. They are hideously strong and quick."},
	Snovemdomas: {Name: "Snovemdomas", Ability: "SA_DoubleHull", AbilityText: "All ships have double strength hulls", Recharge: 0, HomeSize: 5, HomeType: Primordial,
		Description: "are incredibly tough, and guard their territories with alarming ferocity. They are generally uninterested in diplomacy with other species."},
	Orfa: {Name: "Orfa", Ability: "SA_BuildOnBlack", AbilityText: "Build on black planet squares", Recharge: 0, HomeSize: 4, HomeType: Primordial,
		Description: "are from a seething, volcanic world, and they are able to subsist on raw minerals.  They are slothful but highly intelligent."},
	Kambuchka: {Name: "Kambuchka", Ability: "SA_KnowHomeWorlds", AbilityText: "See all home stars", Recharge: 0, HomeSize: 5, HomeType: Chapel,
		Description: "subsist by filtering the rich, soupy atmosphere of their world. A Kambuchka's entire body is a convoluted sensory membrane."},
	Hanshaks: {Name: "Hanshaks", Ability: "SA_KnowRaces", AbilityText: "Talk to all races at the start of the game", Recharge: 0, HomeSize: 4, HomeType: Eden,
		Description: "are an ancient race of toroidal mind-bodies. They are in tune with life and nature, and are masters of communication."},
	Fludentri: {Name: "Fludentri", Ability: "SA_HealShips", AbilityText: "Repair all ships' damage", Recharge: 60, HomeSize: 5, HomeType: Congenial,
		Description: "are composed entirely of liquid, and a severely injured Fludentri heals in seconds. They are ambitious and unintellectual."},
	Baliflids: {Name: "Baliflids", Ability: "SA_ForcePeace", AbilityText: "Force all aliens to make peace with you", Recharge: 100, HomeSize: 4, HomeType: SpecialPlanet,
		Description: "are cute little rodents. Their disarming appearance belies their ruthless diplomatic skill."},
	Swaparamans: {Name: "Swaparamans", Ability: "SA_DoublePower", AbilityText: "Double all ships' power", Recharge: 62, HomeSize: 4, HomeType: Chapel,
		Description: "are cosmic exiles.  They were driven from their home world by the dread Frutmaka, and survived only by finding great strength in adversity."},
	Frutmaka: {Name: "Frutmaka", Ability: "SA_RepelAlienShips", AbilityText: "Warp alien ships out of your colonies' stars", Recharge: 77, HomeSize: 5, HomeType: Mineral,
		Description: "share a common home world and ancestry with the Swaparamans.  Their black-hole god, Graveesha, has given them the ability to warp space."},
	Shevar: {Name: "Shevar", Ability: "SA_SapAlienPower", AbilityText: "Wipe out the power in alien ships in any system you occupy", Recharge: 90, HomeSize: 4, HomeType: Mineral,
		Description: "are a non-organic lifeform, and masters of the dark forces of the universe.  They harness arcane powers that sap and oppose all life."},
	Govorom: {Name: "Govorom", Ability: "SA_Cornucopia", AbilityText: "Turn your least populated world into a Cornucopia", Recharge: 150, HomeSize: 4, HomeType: Cornucopia,
		Description: "are spirits of nature.  They transformed their once-barren home world into a lush paradise.  They are guarded but trustworthy."},
	Ungooma: {Name: "Ungooma", Ability: "SA_BumpShipsInLanes", AbilityText: "Bump all ships backward down star lanes", Recharge: 70, HomeSize: 4, HomeType: Congenial,
		Description: "are brain parasites.  Their mind-control can leave a victim uncertain of which memories are real and which were implanted."},
	Dubtaks: {Name: "Dubtaks", Ability: "SA_StealResearch", AbilityText: "Steal any technology owned by two or more races", Recharge: 63, HomeSize: 5, HomeType: Cathedral,
		Description: "practice science in a most unsportsmanlike fashion. Nearly all of their knowledge is stolen."},
	Capelons: {Name: "Capelons", Ability: "SA_ShieldPlanets", AbilityText: "Make all colonies invincible for one day", Recharge: 66, HomeSize: 5, HomeType: SpecialPlanet,
		Description: "are adept at camouflage, self-protection, and survival.  They keep their motives hidden and are cunning diplomats."},
	Mebes: {Name: "Mebes", Ability: "SA_MoreMaxPop", AbilityText: "Increase the maximum population of all colonies", Recharge: 72, HomeSize: 5, HomeType: Tycoon,
		Description: "are large single-celled creatures that reproduce at will.  They are extremely expansionistic and communication with them is difficult."},
	Oculons: {Name: "Oculons", Ability: "SA_KnowStarLanes", AbilityText: "Can see all star lanes", Recharge: 0, HomeSize: 4, HomeType: Cathedral,
		Description: "are a chivalrous race of astronomers and mystics. They are fierce enemies and loyal friends."},
	Arbryls: {Name: "Arbryls", Ability: "SA_BlockStarLanes", AbilityText: "Block all star lanes entering your colonised systems", Recharge: 100, HomeSize: 5, HomeType: Eden,
		Description: "are a gangly tree-people. They are respectful of other life, but want more than anything to be left alone."},
	Marmosians: {Name: "Marmosians", Ability: "SA_CauseWar", AbilityText: "Make aliens dislike anyone at war with you", Recharge: 100, HomeSize: 4, HomeType: Supermineral,
		Description: "are territorial insects.  They use complex pheromone signals to manipulate other creatures into protecting their territory for them."},
	Chronomyst: {Name: "Chronomyst", Ability: "SA_FastLaneTravel", AbilityText: "Travel twice as fast through star lanes", Recharge: 0, HomeSize: 3, HomeType: Eden,
		Description: "are jewel-like water creatures.  Their brains are organic optical computers, making them lightning-quick."},
	Chamachies: {Name: "Chamachies", Ability: "SA_FreeResearch", AbilityText: "Achieve any discovery in one day", Recharge: 89, HomeSize: 4, HomeType: Cathedral,
		Description: "are six-legged reptilians.  They are fascinated with technology and gadgets, and produce brilliant engineers and fleet captains."},
	Nimbuloids: {Name: "Nimbuloids", Ability: "SA_IndustryBoost", AbilityText: "Double industry on every planet for one day", Recharge: 68, HomeSize: 4, HomeType: Tycoon,
		Description: "are composed of dense, cohesive gases.  Their abilities to form tools of any shape and fit into small spaces make them marvelous builders."},
}

func (s Species) String() string { return SpeciesTable[s].Name }

// Info returns the species' line of SpeciesTable.
func (s Species) Info() SpeciesInfo { return SpeciesTable[s] }

// HomePlanet rolls the species' home world (homePlanet).
func (s Species) HomePlanet(seed uint64, name string) *Planet {
	in := s.Info()
	return homePlanet(seed, name, in.HomeType, in.HomeSize, s.Traits())
}

// Traits are a species' passive rules (Recharge 0), one method per rule
// bent; the timed abilities go through Game.UseAbility.
type Traits interface {
	// BuildsOnBlack allows any surface structure on black (Orfa); the
	// home world is not forced open either.
	BuildsOnBlack() bool
	// LaneFactor multiplies the lane speed (Chronomyst: 2).
	LaneFactor() float64
	// HullFactor multiplies every ship's hull strength (Snovemdomas: 2).
	HullFactor() int
	// KnowsStarLanes shows every lane, not the stars behind (Oculons).
	KnowsStarLanes() bool
	// InvadesAlways makes every invasion succeed (Minions).
	InvadesAlways() bool
	// KnowsRaces meets every other empire at day 0 (Hanshaks).
	KnowsRaces() bool
	// KnowsHomeStars explores every home star at day 0 (Kambuchka).
	KnowsHomeStars() bool
}

// baseTraits is everyone's default; a species type embeds it and
// overrides what its ability changes.
type baseTraits struct{}

func (baseTraits) BuildsOnBlack() bool  { return false }
func (baseTraits) LaneFactor() float64  { return 1 }
func (baseTraits) HullFactor() int      { return 1 }
func (baseTraits) KnowsStarLanes() bool { return false }
func (baseTraits) InvadesAlways() bool  { return false }
func (baseTraits) KnowsRaces() bool     { return false }
func (baseTraits) KnowsHomeStars() bool { return false }

// kambuchkaTraits: SA_KnowHomeWorlds, "see all home stars".
type kambuchkaTraits struct{ baseTraits }

func (kambuchkaTraits) KnowsHomeStars() bool { return true }

// hanshaksTraits: SA_KnowRaces.
type hanshaksTraits struct{ baseTraits }

func (hanshaksTraits) KnowsRaces() bool { return true }

// minionsTraits: SA_SuperMarines, "planetary invasions always succeed".
type minionsTraits struct{ baseTraits }

func (minionsTraits) InvadesAlways() bool { return true }

// oculonsTraits: SA_KnowStarLanes, "can see all star lanes".
type oculonsTraits struct{ baseTraits }

func (oculonsTraits) KnowsStarLanes() bool { return true }

// orfaTraits: SA_BuildOnBlack.
type orfaTraits struct{ baseTraits }

func (orfaTraits) BuildsOnBlack() bool { return true }

// chronomystTraits: SA_FastLaneTravel, "twice as fast" (fan manual).
type chronomystTraits struct{ baseTraits }

func (chronomystTraits) LaneFactor() float64 { return 2 }

// snovemdomasTraits: SA_DoubleHull.
type snovemdomasTraits struct{ baseTraits }

func (snovemdomasTraits) HullFactor() int { return 2 }

// speciesTraits holds the species whose passive ability bends a rule.
var speciesTraits = map[Species]Traits{Orfa: orfaTraits{}, Chronomyst: chronomystTraits{}, Snovemdomas: snovemdomasTraits{}, Oculons: oculonsTraits{},
	Minions: minionsTraits{}, Hanshaks: hanshaksTraits{}, Kambuchka: kambuchkaTraits{}}

// Base is the traits of no species: sandboxes and placeholders.
var Base Traits = baseTraits{}

// Traits returns the species' passive rules, Base when its ability bends
// none of them.
func (s Species) Traits() Traits {
	if t, ok := speciesTraits[s]; ok {
		return t
	}
	return Base
}
