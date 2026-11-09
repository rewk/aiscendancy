// Package game is the rules model: planets, their squares and structures.
// The view packages read it; nothing here knows about pixels or Ebiten.
package game

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// SquareKind is the colour of a surface square. Black squares take only an
// outpost or tubes, white ones any structure; the bonus kinds favour one
// output. Each bonus's colour is the view's business.
type SquareKind uint8

const (
	Black SquareKind = iota
	White
	BonusIndustry
	BonusProsperity
	BonusResearch
	NumSquareKinds
)

func (k SquareKind) String() string {
	return [...]string{"black", "white", "industry", "prosperity", "research"}[k]
}

// PlanetType orders the types as planitem.txt and planets.shp do.
type PlanetType uint8

const (
	Husk PlanetType = iota
	Primordial
	Congenial
	Eden
	Mineral
	Supermineral
	Chapel
	Cathedral
	SpecialPlanet // "Special": the name Special is a GizmoCat
	Tycoon
	Cornucopia
	NumPlanetTypes
)

func (t PlanetType) String() string {
	return [...]string{"Husk", "Primordial", "Congenial", "Eden", "Mineral", "Supermineral",
		"Chapel", "Cathedral", "Special", "Tycoon", "Cornucopia"}[t]
}

// Mix is a type's square percentages (planitem.txt's column order):
// black, industry, prosperity, research, the rest white.
type Mix [4]int

// roll draws a square's kind from m: one IntN(100) of r, White past the
// four percentages.
func (m Mix) roll(r *rand.Rand) SquareKind {
	roll := r.IntN(100)
	for k, pct := range m {
		if roll < pct {
			return [...]SquareKind{Black, BonusIndustry, BonusProsperity, BonusResearch}[k]
		}
		roll -= pct
	}
	return White
}

// Mixes is planitem.txt's table, checked against the game by the tests.
var Mixes = [NumPlanetTypes]Mix{
	{100, 0, 0, 0},
	{50, 2, 2, 2},
	{20, 3, 5, 3},
	{0, 3, 20, 3},
	{40, 10, 2, 2},
	{20, 20, 2, 2},
	{40, 2, 2, 10},
	{20, 3, 3, 20},
	{40, 10, 10, 10},
	{20, 15, 15, 15},
	{0, 33, 34, 33},
}

// Sizes run 1 (7 squares) to 5 (73 squares).
const (
	MinSize = 1
	MaxSize = 5
	// GridSide is the frame the layouts sit in: cells (i,j) in 0..10, the
	// centre square at (Centre,Centre) for every size.
	GridSide = 11
	Centre   = 5
	// OrbitSlots is the number of orbital locations of any planet.
	OrbitSlots = 10
)

// Cell addresses a surface square in grid coordinates. Screen position in
// an isometric view: x grows with i-j, y with i+j.
type Cell struct{ I, J int }

// MarshalText is "i,j": a Cell keys Planet.Surface, and JSON map keys must
// be text.
func (c Cell) MarshalText() ([]byte, error) { return fmt.Appendf(nil, "%d,%d", c.I, c.J), nil }

// String is "(i,j)", as the messages print a square.
func (c Cell) String() string { return fmt.Sprintf("(%d,%d)", c.I, c.J) }

// UnmarshalText reads MarshalText's "i,j".
func (c *Cell) UnmarshalText(b []byte) error {
	si, sj, ok := strings.Cut(string(b), ",")
	i, err1 := strconv.Atoi(si)
	j, err2 := strconv.Atoi(sj)
	if !ok || err1 != nil || err2 != nil {
		return fmt.Errorf("cell %q: want i,j", b)
	}
	*c = Cell{i, j}
	return nil
}

// layoutFirst and layoutCount are planitem.txt's two 5x15 tables: for
// size s (row s-1) and grid row j (column j), the first i and the number
// of squares. Only columns 5-s..5+s are used.
var layoutFirst = [MaxSize][GridSide]int{
	{0, 0, 0, 0, 4, 4, 5, 0, 0, 0, 0},
	{0, 0, 0, 4, 3, 4, 5, 6, 0, 0, 0},
	{0, 0, 3, 2, 3, 3, 4, 5, 7, 0, 0},
	{0, 2, 1, 1, 2, 2, 3, 4, 5, 7, 0},
	{1, 0, 0, 0, 1, 1, 2, 3, 4, 5, 7},
}

var layoutCount = [MaxSize][GridSide]int{
	{0, 0, 0, 0, 2, 3, 2, 0, 0, 0, 0},
	{0, 0, 0, 1, 3, 3, 3, 1, 0, 0, 0},
	{0, 0, 1, 4, 4, 5, 4, 4, 1, 0, 0},
	{0, 2, 5, 6, 6, 7, 6, 6, 5, 2, 0},
	{3, 6, 7, 8, 8, 9, 8, 8, 7, 6, 3},
}

// Layout lists the cells of a planet of the given size, row by row (by j,
// then i), which is back-to-front for an isometric painter.
func Layout(size int) []Cell {
	var cells []Cell
	for j := 0; j < GridSide; j++ {
		for k := 0; k < layoutCount[size-1][j]; k++ {
			cells = append(cells, Cell{layoutFirst[size-1][j] + k, j})
		}
	}
	return cells
}

// Square is one surface location.
type Square struct {
	Kind SquareKind
	Item Item // NoItem when empty
	// Ruins marks Xeno ruins, at most one per planet (layRuins), whatever
	// the Kind (still counted); the dig clears it.
	Ruins bool
	// Automated marks a structure whose Automation completed: it employs
	// nobody, output and MaxPop unchanged; cleared with the item.
	Automated bool
}

// Planet is one world and what stands on it.
type Planet struct {
	Name    string
	Type    PlanetType
	Size    int
	Surface map[Cell]*Square
	Orbit   [OrbitSlots]Item
	// OrbitDamage is the hidden damage per slot's orbital defence, reset
	// when the slot is emptied or built (views read DefenceLeft).
	OrbitDamage [OrbitSlots]int
	Pos         Vec3 // where it sits in its star system; zero outside one
	// Owner is the colony's empire Index, −1 none (a literal's 0 is
	// empire 0: set it).
	Owner int
	// Lush is set by a completed Lush Growth Bomb (Capacity).
	Lush bool
	// PopBonus is the Capacity the Mebes' SA_MoreMaxPop added.
	PopBonus int
}

// NewPlanet rolls a planet's squares from its type's mix.
func NewPlanet(seed uint64, name string, typ PlanetType, size int) *Planet {
	if size < MinSize || size > MaxSize {
		panic(fmt.Sprintf("planet size %d", size))
	}
	r := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	p := &Planet{Name: name, Type: typ, Size: size, Surface: map[Cell]*Square{}, Owner: -1}
	for i := range p.Orbit {
		p.Orbit[i] = NoItem
	}
	mix := Mixes[typ]
	for _, c := range Layout(size) {
		p.Surface[c] = &Square{Kind: mix.roll(r), Item: NoItem}
	}
	return p
}

// NewHomePlanet is the generic home: a congenial size 3 (Species.HomePlanet
// makes the real ones).
func NewHomePlanet(seed uint64, name string) *Planet {
	return homePlanet(seed, name, Congenial, 3, Base)
}

// homePlanet rolls a world with the colony base on its centre square,
// forced white, and its four edge neighbours whitened unless
// tr.BuildsOnBlack (ours: a base hemmed in by black strangles the start;
// the original's homes start open).
func homePlanet(seed uint64, name string, typ PlanetType, size int, tr Traits) *Planet {
	p := NewPlanet(seed, name, typ, size)
	c := Cell{Centre, Centre}
	p.Surface[c].Kind = White
	p.Surface[c].Item = ColonyBase
	if tr.BuildsOnBlack() {
		return p
	}
	for _, n := range c.edges() {
		if sq, ok := p.Surface[n]; ok && sq.Kind == Black {
			sq.Kind = White
		}
	}
	return p
}

// edges are the four cells sharing an edge with c.
func (c Cell) edges() [4]Cell {
	return [4]Cell{{c.I - 1, c.J}, {c.I + 1, c.J}, {c.I, c.J - 1}, {c.I, c.J + 1}}
}

// RuinsAt is the planet's Xeno ruins square, false when it has none.
func (p *Planet) RuinsAt() (Cell, bool) {
	for _, c := range p.Cells() {
		if p.Surface[c].Ruins {
			return c, true
		}
	}
	return Cell{}, false
}

// Cells lists the planet's squares back to front.
func (p *Planet) Cells() []Cell { return Layout(p.Size) }

// CanBuild reports whether a surface structure may go on a cell: an
// empty Reachable square without ruins, not black but for the Transport
// Tubes or tr (the Orfa). The data flags the Outpost too (FlagOnBlack),
// the original refuses it there.
func (p *Planet) CanBuild(c Cell, it Item, tr Traits) bool {
	sq, ok := p.Surface[c]
	if !ok || !it.Surface() || sq.Item != NoItem || sq.Ruins || !p.Reachable(c) {
		return false
	}
	return sq.Kind != Black || it.Has(FlagOnBlack) && it != Outpost || tr.BuildsOnBlack()
}

// CanSettle reports a founded colony's Colony Base may go on c: empty, no
// ruins, Reachable, black included.
func (p *Planet) CanSettle(c Cell) bool {
	sq, ok := p.Surface[c]
	return ok && sq.Item == NoItem && !sq.Ruins && p.Reachable(c)
}

// Reachable reports a square shares an edge with a structure (as in the
// original: the colony grows outward, Tubes cross black); every square on
// a planet without structure.
func (p *Planet) Reachable(c Cell) bool {
	if !p.HasStructure() {
		return true
	}
	for _, n := range c.edges() {
		if sq, ok := p.Surface[n]; ok && sq.Item != NoItem {
			return true
		}
	}
	return false
}

// Count is how many of an item the planet holds, surface and orbit.
func (p *Planet) Count(it Item) int {
	n := 0
	for _, sq := range p.Surface {
		if sq.Item == it {
			n++
		}
	}
	for _, o := range p.Orbit {
		if o == it {
			n++
		}
	}
	return n
}

// HasStructure reports anything built on the surface.
func (p *Planet) HasStructure() bool {
	for _, sq := range p.Surface {
		if sq.Item != NoItem {
			return true
		}
	}
	return false
}

// Build is BuildWith under no species' traits (Base), for fixtures.
func (p *Planet) Build(c Cell, it Item) error { return p.BuildWith(c, it, Base) }

// BuildWith places a surface structure where CanBuild under tr allows it.
func (p *Planet) BuildWith(c Cell, it Item, tr Traits) error {
	if !p.CanBuild(c, it, tr) {
		return fmt.Errorf("cannot build %s at %v", it, c)
	}
	p.Surface[c].Item = it
	return nil
}

// CanTerraform reports Terraforming may start at the black square c: empty
// and reachable, or under Tubes (the guides: "change your tubes into
// useable white squares"); never on ruins.
func (p *Planet) CanTerraform(c Cell) bool {
	sq, ok := p.Surface[c]
	if !ok || sq.Kind != Black || sq.Ruins {
		return false
	}
	return sq.Item == TransportTubes || sq.Item == NoItem && p.Reachable(c)
}

// Offers reports whether a square's picker lists the OnSquare item it at
// c (CanStart then judges): only the dig on the ruins square, Terraforming
// on black only, the rest anywhere.
func (p *Planet) Offers(it Item, c Cell) bool {
	sq, ok := p.Surface[c]
	switch {
	case !it.OnSquare() || !ok:
		return false
	case it == XenoArcheologicalDig || sq.Ruins:
		return it == XenoArcheologicalDig && sq.Ruins
	case it == Terraforming:
		return sq.Kind == Black
	}
	return true
}

// Terraform turns the black square c white, as CanTerraform allows; its
// Tubes are removed, the square left empty.
func (p *Planet) Terraform(c Cell) error {
	if !p.CanTerraform(c) {
		return fmt.Errorf("cannot terraform %v", c)
	}
	p.whiten(c)
	return nil
}

// whiten turns square c White, removing Transport Tubes; any other
// structure stays (also the Govorom's SA_Cornucopia).
func (p *Planet) whiten(c Cell) {
	sq := p.Surface[c]
	sq.Kind = White
	if sq.Item == TransportTubes {
		sq.Item, sq.Automated = NoItem, false
	}
}

// Demolish empties a square, its Automated mark with it; demolishing
// nothing is fine.
func (p *Planet) Demolish(c Cell) {
	if sq, ok := p.Surface[c]; ok {
		sq.Item, sq.Automated = NoItem, false
	}
}

// Automate marks the structure it at c Automated, refused unless c holds
// it not yet automated.
func (p *Planet) Automate(c Cell, it Item) error {
	sq, ok := p.Surface[c]
	if !ok || it == NoItem || sq.Item != it || sq.Automated {
		return fmt.Errorf("cannot automate %s at %v", it, c)
	}
	sq.Automated = true
	return nil
}

// HasShipyard reports whether ships can be built here.
func (p *Planet) HasShipyard() bool { return slices.Contains(p.Orbit[:], Shipyard) }

// HasDocks reports whether the ships in orbit here can be refitted.
func (p *Planet) HasDocks() bool { return slices.Contains(p.Orbit[:], OrbitalDocks) }

// CanBuildOrbital reports whether an orbital item may go in a slot: it is
// empty, and ships need a shipyard.
func (p *Planet) CanBuildOrbital(slot int, it Item) bool {
	if slot < 0 || slot >= OrbitSlots || !it.Orbital() || p.Orbit[slot] != NoItem {
		return false
	}
	return it != Ship || p.HasShipyard()
}

// BuildOrbital places an orbital item.
func (p *Planet) BuildOrbital(slot int, it Item) error {
	if !p.CanBuildOrbital(slot, it) {
		return fmt.Errorf("cannot build %s in orbit slot %d", it, slot)
	}
	p.Orbit[slot], p.OrbitDamage[slot] = it, 0
	return nil
}

// DemolishOrbital empties a slot, its OrbitDamage reset.
func (p *Planet) DemolishOrbital(slot int) {
	if slot >= 0 && slot < OrbitSlots {
		p.Orbit[slot], p.OrbitDamage[slot] = NoItem, 0
	}
}

// Output sums the three productions. A structure on a bonus square gets
// +1 of that output when it makes at least 1 by itself (the original, as
// observed).
func (p *Planet) Output() (industry, research, prosperity int) {
	for _, sq := range p.Surface {
		if sq.Item == NoItem {
			continue
		}
		in := sq.Item.Info()
		industry += in.Industry + bonus(sq.Kind, BonusIndustry, in.Industry)
		research += in.Research + bonus(sq.Kind, BonusResearch, in.Research)
		prosperity += in.Prosp + bonus(sq.Kind, BonusProsperity, in.Prosp)
	}
	for _, it := range p.Orbit {
		if it != NoItem {
			in := it.Info()
			industry += in.Industry
			research += in.Research
			prosperity += in.Prosp
		}
	}
	return
}

func bonus(have, want SquareKind, base int) int {
	if have == want && base > 0 {
		return 1
	}
	return 0
}

// Population returns the people employed (Item.Staff, none when
// Automated) and the room the structures make.
func (p *Planet) Population() (used, max int) {
	for _, sq := range p.Surface {
		if sq.Item != NoItem {
			if !sq.Automated {
				used += sq.Item.Staff()
			}
			max += sq.Item.Info().MaxPop
		}
	}
	for _, it := range p.Orbit {
		if it != NoItem {
			used += it.Staff()
			max += it.Info().MaxPop
		}
	}
	return
}

// BestCell picks where to put a surface structure among the cells CanBuild
// allows: a bonus square of the output it makes most of (ties industry,
// research, prosperity), then white, then any; within a rank the nearest
// the centre by |i-5|+|j-5|, then Layout order.
func (p *Planet) BestCell(it Item, tr Traits) (Cell, bool) {
	want := Black // no bonus kind wanted: Black never ranks first below
	if it.Surface() {
		in := it.Info()
		switch m := max(in.Industry, in.Research, in.Prosp); {
		case m <= 0:
		case in.Industry == m:
			want = BonusIndustry
		case in.Research == m:
			want = BonusResearch
		default:
			want = BonusProsperity
		}
	}
	var best Cell
	bestRank, bestDist := 3, 0
	for _, c := range p.Cells() {
		if !p.CanBuild(c, it, tr) {
			continue
		}
		rank := 2
		switch k := p.Surface[c].Kind; {
		case want != Black && k == want:
			rank = 0
		case k == White:
			rank = 1
		}
		d := abs(c.I-Centre) + abs(c.J-Centre)
		if rank < bestRank || rank == bestRank && d < bestDist {
			best, bestRank, bestDist = c, rank, d
		}
	}
	return best, bestRank < 3
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
