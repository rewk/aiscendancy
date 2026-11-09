package game

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/rewk/aiscendancy/journal"
)

// Game is one cluster and its empires, in a fixed order (index 0 the
// window's player by default) that EndDay runs them in, as the original
// takes the species' turns.
type Game struct {
	Seed    uint64
	Cluster *Cluster
	Empires []*Empire
	Day     int // days run so far, equal to every Empire.Day
	// Atmosphere is the New Game screen's choice: the bots read it as a
	// bias (bot.AtmosphereShift), the rules ignore it.
	Atmosphere Atmosphere
	// Offers are the pending diplomatic proposals, in append order.
	Offers []Offer
	// RoundsToday is the rounds played today per star (Round counts
	// them; nil before the first, then len(Cluster.Stars)): the fought
	// stars' ships do not Step at the close (those still CombatStars
	// there: a fight ended mid-day lets them Step), the caps (bot.RoundCap, a
	// bot's Rounds) count the mid-day rounds with the close's. Saved, so
	// a load mid-day still refills them; EndDay sets it to nil.
	RoundsToday []int `json:",omitempty"`
	// closeFrom is the engagements at the close's start (BeginClose,
	// before the bots act), Game.opened's before; nil: EndDay's own. Not
	// saved: BeginClose and EndDay run in one call (bot.Begin, Finish).
	closeFrom map[[2]int]bool
	// journal records the accepted actions (SetJournal); nil none.
	journal *journal.Writer
}

// Atmosphere is the original's "general aggressiveness of species": a
// step either side of Neutral, so a bias is int(a) steps.
type Atmosphere int

const (
	Peaceful Atmosphere = -1
	Neutral  Atmosphere = 0
	Hostile  Atmosphere = 1
)

func (a Atmosphere) String() string {
	switch a {
	case Peaceful:
		return "Peaceful"
	case Hostile:
		return "Hostile"
	}
	return "Neutral"
}

// NewGame rolls the cluster from seed and settles one empire per species,
// empire i at pickHomes(n)[i]: the star's first planet replaced by the
// species' home world "<Species> Prime" (seedFor), colonised. The
// Hanshaks (KnowsRaces) meet everyone at day 0, the Kambuchka
// (KnowsHomeStars) know every home star. Panics on no species or one
// twice.
func NewGame(seed uint64, species []Species, d Density) *Game {
	if len(species) == 0 {
		panic("a game of no empire")
	}
	seen := map[Species]bool{}
	for _, sp := range species {
		if seen[sp] {
			panic(fmt.Sprintf("species %s twice", sp))
		}
		seen[sp] = true
	}
	c := NewCluster(seed, d)
	homes := c.pickHomes(len(species))
	colours := PickColours(seed, 0, len(species))
	g := &Game{Seed: seed, Cluster: c}
	for i, sp := range species {
		sys := c.Stars[homes[i]].System
		hp := sp.HomePlanet(seedFor(seed, i), sp.String()+" Prime")
		hp.Pos = sys.Planets[0].Pos
		hp.Owner = i
		sys.Planets[0] = hp
		spreadOrbits(sys.Planets) // the home's size
		col := NewColony(hp)
		col.Star, col.PlanetIdx = homes[i], 0
		e := &Empire{Species: sp, Index: i, Colour: colours[i], Research: &Research{Topic: NoTech, Stars: len(c.Stars)}, Cluster: c,
			Colonies: []*Colony{col}, Invincible: -1}
		col.adopt(e)
		e.Visit(homes[i])
		g.Empires = append(g.Empires, e)
	}
	for _, e := range g.Empires {
		e.peers = g.Empires
		e.Relations = newRelations(len(species))
	}
	for _, e := range g.Empires {
		if e.Species.Traits().KnowsHomeStars() {
			for _, h := range homes {
				e.Visit(h)
			}
		}
	}
	for a, e := range g.Empires {
		if e.Species.Traits().KnowsRaces() {
			for b := range g.Empires {
				g.Contact(a, b)
			}
		}
	}
	return g
}

// seedFor is empire i's home world seed: seed itself for empire 0, then
// steps of the golden-ratio constant.
func seedFor(seed uint64, i int) uint64 { return seed + uint64(i)*0x9e3779b97f4a7c15 }

// EndDay runs one day: Day incremented, the planets turned on their
// orbits (turnPlanets), the blockades marked (blockades), every empire's endDay in order
// (the fought stars' ships only refilled; every empire's ships are
// obstacles), then the invasions, the close of control (enforceCaps),
// the blockades begun or lifted (blockadeNews, the ships lost at the
// close gone), the victories, the diplomacy's close, the recharged abilities and the
// fights opened (opened, against the engagements at the start), their
// Reports appended to each empire's events; the History sampled
// (record) at day 0's start and every HistoryEvery days' close.
func (g *Game) EndDay() [][]Event {
	if g.Day == 0 {
		g.record()
	}
	g.Day++
	before := g.closeFrom
	if before == nil {
		before = g.engagements()
	}
	g.closeFrom = nil
	fought := slices.Clone(g.RoundsToday) // only where the fight goes on
	cs := g.CombatStars()
	for st := range fought {
		if !slices.Contains(cs, st) {
			fought[st] = 0
		}
	}
	g.turnPlanets() // the orbits, before the ships move
	g.blockades()   // as the day starts: today's moves change nothing
	evs := make([][]Event, len(g.Empires))
	for i, e := range g.Empires {
		evs[i] = e.endDay(g.Seed, fought, g.Empires)
	}
	reps := g.invasions()
	reps = append(reps, g.enforceCaps()...)
	reps = append(reps, g.blockadeNews()...)
	reps = append(reps, g.victories()...)
	reps = append(reps, g.diplomacy()...)
	reps = append(reps, g.abilitiesReady()...)
	reps = append(reps, g.opened(before)...)
	for _, r := range reps {
		evs[r.Empire] = append(evs[r.Empire], r.Event)
	}
	g.RoundsToday = nil
	if g.Day%HistoryEvery == 0 {
		g.record()
	}
	return evs
}

// Relink points every empire at the game's cluster (saved once) and its
// peers, and relinks its colonies; errors on a bad Index or Relations.
func (g *Game) Relink() error {
	for i, e := range g.Empires {
		if e == nil || e.Index != i {
			return fmt.Errorf("empire %d: missing or out of order", i)
		}
		if len(e.Relations) != len(g.Empires) {
			return fmt.Errorf("empire %d: %d relations for %d empires", i, len(e.Relations), len(g.Empires))
		}
		e.Cluster, e.peers = g.Cluster, g.Empires
		if err := e.Relink(); err != nil {
			return fmt.Errorf("empire %d: %w", i, err)
		}
	}
	return nil
}

// The empires' colours, in the original's order: its Player Color
// choices, racering.shp's frames and game.pal's 4-entry ramps from 16
// (the flags are drawn in the first, green).
const (
	Green = iota
	Purple
	Yellow
	Pink
	Brown
	Blue
	Orange
	NumColours
)

var colourNames = [NumColours]string{"Green", "Purple", "Yellow", "Pink", "Brown", "Blue", "Orange"}

// ColourName is colour c's name, "Green".."Orange".
func ColourName(c int) string {
	if c < 0 || c >= NumColours {
		return fmt.Sprintf("Colour(%d)", c)
	}
	return colourNames[c]
}

// ParseColour reads a colour by name (any case) or index.
func ParseColour(s string) (int, error) {
	for c, n := range colourNames {
		if strings.EqualFold(s, n) || s == strconv.Itoa(c) {
			return c, nil
		}
	}
	return 0, fmt.Errorf("no colour %q (green, purple, yellow, pink, brown, blue, orange)", s)
}

// PickColours is first then a shuffle of the rest (own PRNG), repeating
// past NumColours. Panics on first out of range.
func PickColours(seed uint64, first, n int) []int {
	if first < 0 || first >= NumColours {
		panic(fmt.Sprintf("colour %d of %d", first, NumColours))
	}
	var rest []int
	for c := range NumColours {
		if c != first {
			rest = append(rest, c)
		}
	}
	r := rand.New(rand.NewPCG(seed, 0xc2b2ae35))
	r.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	order := append([]int{first}, rest...)
	out := make([]int, n)
	for i := range out {
		out[i] = order[i%NumColours]
	}
	return out
}

// SetColour gives empire i colour c, the empire holding c taking i's old
// one.
func (g *Game) SetColour(i, c int) {
	if c < 0 || c >= NumColours {
		panic(fmt.Sprintf("colour %d of %d", c, NumColours))
	}
	for _, e := range g.Empires {
		if e.Colour == c {
			e.Colour = g.Empires[i].Colour
		}
	}
	g.Empires[i].Colour = c
}

// PickSpecies is first then the first n−1 of a shuffle of the others (own
// PRNG). Panics on n outside 1..NumSpecies.
func PickSpecies(seed uint64, first Species, n int) []Species {
	if n < 1 || n > int(NumSpecies) {
		panic(fmt.Sprintf("%d species of %d", n, NumSpecies))
	}
	var rest []Species
	for sp := range NumSpecies {
		if sp != first {
			rest = append(rest, sp)
		}
	}
	r := rand.New(rand.NewPCG(seed, 0x5bd1e995))
	r.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	return append([]Species{first}, rest[:n-1]...)
}
