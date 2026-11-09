// Package bot is the playing policy of an Empire: what to build, research,
// send where. It imports only game/ (no pixels, no printing); the sim and
// the window both play it.
package bot

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// Config is what the bot plays: the home's opening script (items started
// in order, waiting on a refusal), the research queue (then AimItems'
// techs, then the first available topic), Greedy after the script.
type Config struct {
	Script     []game.Item
	Research   []game.Tech
	Greedy     bool
	NoResearch bool // never pick a topic (the sim's -research none)
	// NoDiplomacy skips Act's diplomacy pass: no offer made or answered,
	// the stances left as set (fuzz's duel: an arena at War stays so).
	NoDiplomacy bool `json:",omitempty"`
	// Genome, when set, replaces the fixed choices by its weights; nil
	// plays the fixed policy.
	Genome *Genome `json:",omitempty"`
	// Difficulty raises the policy above the original's AI (0, Original:
	// as tuned on its saves).
	Difficulty Difficulty `json:",omitempty"`
}

// Difficulty is the bots' level, picked on the new-game screen (a
// level per species may follow, say an invader).
type Difficulty int

const (
	Original Difficulty = iota // the pace of the original's saves
	Hard                       // HardGenome (AtLevel); inFlightMax at least hardInFlight
	NumDifficulties
)

func (d Difficulty) String() string {
	switch d {
	case Original:
		return "Original"
	case Hard:
		return "Hard"
	}
	return fmt.Sprintf("Difficulty(%d)", int(d))
}

// Default is the simulator's default: agriplot, factory (IndFloor then
// keeps industry first); the 2005 guide's research order; greedy.
func Default() Config {
	return Config{
		Script:   []game.Item{game.Agriplot, game.Factory},
		Research: append([]game.Tech(nil), GuideResearch...),
		Greedy:   true,
	}
}

// GuideResearch is the 2005 walkthrough's research order, the galaxy
// mode's default queue (then auto).
var GuideResearch = []game.Tech{
	game.OrbitalStructures, game.Xenobiology, game.TonklinDiary, game.InterplanetaryExploration,
	game.EnvironmentalEncapsulation, game.SpacetimeSurfing, game.AdvancedChemistry, game.PowerConversion,
	game.Hyperlogic,
}

// GreedyItems are the Neutral genome's Build list, in index order: the
// fixed policy's first tiers (greedyItems weighs the higher ones).
var GreedyItems = []game.Item{game.Factory, game.Agriplot, game.Laboratory, game.Habitat, game.Outpost}

// AimItems are the structures whose techs (ResearchReq) the fixed policy
// studies once GuideResearch is done, in this order (aimTopic): the
// population ones, then the output families' upper tiers (family);
// TestAimItems checks every such tier of the item table is here.
var AimItems = []game.Item{game.Habitat, game.Metroplex, game.IndustrialMegafacility, game.ResearchCampus, game.ArtificialHydroponifer}

// replaceDays is the fixed policy's ReplaceDays (tierReplace): a swap
// pays the output its building loses back within 100 days.
const replaceDays = 100

// frontierWorth is the places an unexplored star counts for in spare:
// about the planets a system holds (1-5, uniform).
const frontierWorth = 3

// foundFloor is the fixed policy's planet-worth floor (Genome's
// FoundFloor): sizes 4-5 first, as the original's AI.
const foundFloor = 4

// inFlightCap is the fixed policy's cap on Colonizers in flight
// (Genome's InFlight): one two-Colonizer ship at a time, the cap nearest
// the saves' pace (docs/rules.md "Colonisation pace").
const inFlightCap = 2

// hardInFlight is Hard's floor under inFlightMax: past it colonies stop
// growing (docs/rules.md "Colonisation pace", Difficulty).
const hardInFlight = 6

// abandonIdle is the fixed policy's AbandonIdle (Genome's): a colony or
// invasion ship idle 60 days is scrapped, freeing its fleet-cap slot.
const abandonIdle = 60

// ruinWorth is the lane days Xeno ruins count for in route: one blue lane
// at one drive (the mean LaneDays of an Average cluster's blue lanes at
// LaneStrength 1, seeds 1-10: 15.2), so a ruins world wins over a
// comparable one a lane nearer.
const ruinWorth = 13

// Bot plays one Empire. Act before each EndDay, Note after it.
type Bot struct {
	E       *game.Empire
	G       *game.Game // for combat and diplomacy; nil: the bot never fires
	Cfg     Config
	Founded map[*game.Colony]int // founding day per colony, 0 for those the bot started with
	// Captured is the capture day of each colony taken by an invasion
	// (never in Founded).
	Captured map[*game.Colony]int

	// dest is the final star each ship was sent to (its order names only
	// the first hop); claimed prunes it.
	dest map[*game.Starship]int
	// inbound is the Colonizers under way to each dest star (claimed).
	inbound map[int]int
	// hab is the day's Habitat tally (habitatDesign), nil until first
	// needed in an Act.
	hab *habTally
	// spare is the open places less the Colonizers not yet sent
	// (claimed): colony ships are built only while it is positive.
	spare int
	// floor is the Size under which foundable refuses a planet, set by
	// claimed (setFloor); 0 or 1 refuses none.
	floor int
	// inFlight is the Colonizers aboard my ships under orders, in a lane or
	// idle and able to go, plus those of the colony ships being built
	// (claimed); build starts none past inFlightMax.
	inFlight int
	// idleDays is the Acts in a row each colony or invasion ship stood idle
	// (abandonStranded); saved (State.Idle), or a loaded bot would keep a
	// stranded ship longer than the game that never stopped.
	idleDays map[*game.Starship]int

	// cg, threat and contact are assess's, redone every Act.
	cg      Combat
	threat  []side
	contact bool

	next  int         // the home's script index
	queue []game.Tech // research still queued
	// invading is the planets my ships are ordered to Invade at Act's
	// end: Note tells a capture from a founding by it.
	invading map[place]bool

	// The diplomacy, by empire index: seen is the ledger as last read,
	// lastTrade and lastAsk my last proposal's and request's day (-1
	// never); att, attOK and news are not saved.
	grudge    []float64
	weary     []float64
	goodwill  []float64
	seen      []game.Relation
	lastTrade []int
	lastAsk   []int
	att       []int
	attOK     []bool
	news      []game.Report
	// seenArmed is armedSeen's count, as of seenDay (not saved).
	seenArmed []int
	seenDay   int
}

// New returns a bot for e, every colony e holds founded on day 0. G is
// nil: set it (NewAll does) for the bot to fight.
func New(e *game.Empire, cfg Config) *Bot {
	b := &Bot{E: e, Cfg: cfg, queue: cfg.Research, Founded: map[*game.Colony]int{}, Captured: map[*game.Colony]int{},
		dest: map[*game.Starship]int{}, inbound: map[int]int{}}
	for _, col := range e.Colonies {
		b.Founded[col] = 0
	}
	b.freshDiplomacy()
	return b
}

// State is a Bot's progress as plain data, for a saved game. Founded and
// Captured hold a day per colony in Empire.Colonies order, Dest a star and
// Idle the idle Acts per ship in Empire.Ships order, -1 for none; the
// diplomacy slices are by empire index.
type State struct {
	Cfg       Config
	Next      int
	Queue     []game.Tech
	Founded   []int
	Captured  []int
	Dest      []int
	Idle      []int
	Grudge    []float64
	Weary     []float64
	Goodwill  []float64
	Seen      []game.Relation
	LastTrade []int
	LastAsk   []int
}

// Save is b's State.
func (b *Bot) Save() *State {
	return &State{Cfg: b.Cfg, Next: b.next, Queue: append([]game.Tech(nil), b.queue...),
		Founded: toList(b.E.Colonies, b.Founded), Captured: toList(b.E.Colonies, b.Captured), Dest: toList(b.E.Ships, b.dest),
		Idle:   toList(b.E.Ships, b.idleDays),
		Grudge: slices.Clone(b.grudge), Weary: slices.Clone(b.weary), Goodwill: slices.Clone(b.goodwill), Seen: slices.Clone(b.seen),
		LastTrade: slices.Clone(b.lastTrade), LastAsk: slices.Clone(b.lastAsk)}
}

// toList is m's value for each of keys, -1 for a key not in m.
func toList[K comparable](keys []K, m map[K]int) []int {
	var out []int
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			v = -1
		}
		out = append(out, v)
	}
	return out
}

// fromList is toList's inverse into m: negative values and those past
// keys skipped.
func fromList[K comparable](keys []K, vs []int, m map[K]int) {
	for i, v := range vs {
		if i < len(keys) && v >= 0 {
			m[keys[i]] = v
		}
	}
}

// Resume is the Bot that Saved st, playing e (decoded from the same
// save). The diplomacy is taken as saved only when every slice holds one
// entry per empire, else it starts fresh.
func Resume(e *game.Empire, st *State) *Bot {
	b := &Bot{E: e, Cfg: st.Cfg, next: st.Next, queue: st.Queue, Founded: map[*game.Colony]int{}, Captured: map[*game.Colony]int{},
		dest: map[*game.Starship]int{}, inbound: map[int]int{}}
	fromList(e.Colonies, st.Founded, b.Founded)
	fromList(e.Colonies, st.Captured, b.Captured)
	fromList(e.Ships, st.Dest, b.dest)
	if len(st.Idle) > 0 {
		b.idleDays = map[*game.Starship]int{}
		fromList(e.Ships, st.Idle, b.idleDays)
	}
	b.freshDiplomacy()
	if n := len(e.Relations); n > 0 && len(st.Grudge) == n && len(st.Weary) == n && len(st.Goodwill) == n && len(st.Seen) == n &&
		len(st.LastTrade) == n && len(st.LastAsk) == n {
		b.grudge, b.weary, b.goodwill = slices.Clone(st.Grudge), slices.Clone(st.Weary), slices.Clone(st.Goodwill)
		b.seen, b.lastTrade, b.lastAsk = slices.Clone(st.Seen), slices.Clone(st.LastTrade), slices.Clone(st.LastAsk)
	}
	return b
}

// Act plays one day's decisions before the rounds and EndDay, in its
// body's order. Orders to a Barred planet are cleared (Hop would bounce
// them for ever), round moves away from combat stars; the Invade orders
// are kept for Note. A defeated empire does nothing.
func (b *Bot) Act() {
	if !b.E.Alive() {
		return
	}
	t := game.NoTech // studied through Empire.Study: journalled
	if g := b.Cfg.Genome; g == nil {
		t = nextTopic(b.E.Research, &b.queue, b.Cfg.NoResearch)
	} else if !b.Cfg.NoResearch {
		t = topicBy(b.E.Research, g)
	}
	if t != game.NoTech {
		b.E.Study(t)
	}
	b.assess()
	if !b.Cfg.NoDiplomacy {
		b.diplomacy()
	}
	b.ability()
	b.repairs()
	b.lanes()
	b.startRefits()
	for _, s := range b.E.Ships {
		b.shieldSwitch(s, b.cg, false) // back on, the draw reserved from the day's energy
		if s.Lane < 0 && s.Order.Kind == game.GoToPlanet && s.Orbit != s.Order.Planet && b.E.Barred(s.Star, s.Order.Planet) {
			s.Order = game.Order{}
		}
	}
	claimed := b.claimed()
	b.hab = nil
	for i, col := range b.E.Colonies {
		if !col.Running() { // idle, or an ongoing project: it gives way to any start
			b.planColony(i, col, claimed)
		}
	}
	if b.G != nil {
		fight := b.combatStars()
		for _, s := range b.E.Ships {
			if s.Lane < 0 && !fight[s.Star] {
				stopPoint(s)
			}
		}
	}
	in := b.ferrying()
	for _, s := range b.E.Ships {
		switch {
		case b.flee(s):
		case b.refitRoute(s):
		case armed(s.Design):
			b.attack(s)
		case s.Invaders > 0:
			b.invade(s, claimed)
		case s.Design.Berths() > 0:
			b.ferry(s, in)
		case explorer(s.Design):
			b.explore(s, claimed)
		default:
			b.planShip(s, claimed)
		}
	}
	b.invading = map[place]bool{}
	for _, s := range b.E.Ships {
		if s.Order.Kind == game.Invade && s.Lane < 0 && s.Orbit >= 0 {
			b.invading[place{s.Star, s.Orbit}] = true
		}
	}
	b.abandonStranded()
	b.retireScouts()
}

// abandonStranded counts the Acts in a row each ship holding a Colonizer
// or an Invasion Module, and no Space Habitat (a Habitat ship waits for
// work, never stranded), stands idle: out of a lane, under no order, not
// being refitted. It runs after every planner, so an idle ship is one
// none could use. A busy or gone ship restarts at 0; one reaching
// abandonAfter is Abandoned (an error, ruled out by the idle test, only
// restarts its count); abandonAfter 0 abandons none.
func (b *Bot) abandonStranded() {
	n := b.abandonAfter()
	if n <= 0 {
		b.idleDays = nil
		return
	}
	if b.idleDays == nil {
		b.idleDays = map[*game.Starship]int{}
	}
	still := map[*game.Starship]bool{}
	var drop []*game.Starship
	for _, s := range b.E.Ships {
		if s.Design.Colonizers()+s.Design.InvasionModules() == 0 || s.Design.Berths() > 0 || s.Lane >= 0 || s.Order.Kind != game.NoOrder || b.E.Refitting(s) != nil {
			continue
		}
		still[s] = true
		if b.idleDays[s]++; b.idleDays[s] >= n {
			drop = append(drop, s)
		}
	}
	for s := range b.idleDays {
		if !still[s] {
			delete(b.idleDays, s)
		}
	}
	for _, s := range drop { // Ships order; Abandon deletes from Ships
		b.E.Abandon(s)
		delete(b.idleDays, s)
	}
}

// abandonAfter is the Genome's AbandonIdle (rounded), else abandonIdle.
func (b *Bot) abandonAfter() int {
	if g := b.Cfg.Genome; g != nil {
		return int(math.Round(g.AbandonIdle))
	}
	return abandonIdle
}

// combatStars marks, per star, the Game's CombatStars today.
func (b *Bot) combatStars() []bool {
	fight := make([]bool, len(b.E.Cluster.Stars))
	for _, st := range b.G.CombatStars() {
		fight[st] = true
	}
	return fight
}

// stopPoint clears a GoToPoint order (s holds instead of drifting on).
func stopPoint(s *game.Starship) {
	if s.Order.Kind == game.GoToPoint {
		s.Order = game.Order{}
	}
}

// say appends a game call's Reports to the news, its refusal dropped.
func (b *Bot) say(reps []game.Report, _ error) { b.news = append(b.news, reps...) }

// Note records the colonies gained by the last EndDay, by identity: a new
// one is Captured when Act left an Invade order on its planet, else
// Founded; colonies e no longer holds leave both maps.
func (b *Bot) Note() {
	ours := make(map[*game.Colony]bool, len(b.E.Colonies))
	for _, col := range b.E.Colonies {
		ours[col] = true
	}
	for col := range b.Founded {
		if !ours[col] {
			delete(b.Founded, col)
		}
	}
	for col := range b.Captured {
		if !ours[col] {
			delete(b.Captured, col)
		}
	}
	for _, col := range b.E.Colonies {
		_, f := b.Founded[col]
		_, c := b.Captured[col]
		switch {
		case f || c:
		case b.invading[place{col.Star, col.PlanetIdx}]:
			b.Captured[col] = b.E.Day
		default:
			b.Founded[col] = b.E.Day
		}
	}
	b.invading = nil
}

// Try starts it on col at its BestCell (orbital items: the first slot that
// takes it), or says why not.
func Try(col *game.Colony, r *game.Research, it game.Item) error {
	if it.Orbital() {
		err := fmt.Errorf("%s: no free orbit slot", it)
		for slot := range game.OrbitSlots {
			if err = col.StartOrbital(r, it, slot); err == nil {
				return nil
			}
		}
		return err
	}
	at, ok := col.Planet.BestCell(it, col.Traits())
	if !ok {
		return fmt.Errorf("%s: no square", it)
	}
	return col.Start(r, it, at)
}

// GreedyPick returns the item to start after the script: among
// greedyItems that can start at their BestCell, at Capacity the most
// MaxPop first (room for the next worker); then those making the output
// the mix wants (wanted; ResShare 0 past the singularity), then the
// cheapest, then the lowest index. NoItem when none can.
func GreedyPick(col *game.Colony, r *game.Research, ec Economy) game.Item {
	makes := wants(col, ec.singular(r))
	capped := col.Pop >= col.Capacity()
	best := game.NoItem
	bestRoom, bestMakes, bestCost := 0, false, 0
	for _, it := range greedyItems(col, r, ec) {
		at, ok := col.Planet.BestCell(it, col.Traits())
		if !ok || col.CanStart(r, it, at) != nil {
			continue
		}
		in := it.Info()
		room, m := 0, makes(in)
		if capped {
			room = in.MaxPop
		}
		switch {
		case best == game.NoItem, room > bestRoom,
			room == bestRoom && m && !bestMakes,
			room == bestRoom && m == bestMakes && in.Cost < bestCost:
			best, bestRoom, bestMakes, bestCost = it, room, m, in.Cost
		}
	}
	return best
}

// greedyItems is what the fixed policy builds on col, in index order:
// the best known tier of each output family the Upgrade threshold lets
// through (bestTier, upgradeOpen) and every known surface item with
// MaxPop but the Colony Base (Habitat, Metroplex, Outpost).
func greedyItems(col *game.Colony, r *game.Research, ec Economy) []game.Item {
	open := func(it game.Item) bool { return upgradeOpen(col, it, ec.Upgrade) }
	var out []game.Item
	for it := game.Item(0); it < game.FirstOrbital; it++ {
		if f := family(it); f >= 0 && it == bestTier(r, f, open) || housing(it) && r.Allows(it) {
			out = append(out, it)
		}
	}
	return out
}

// family is the one output a surface item makes, 0 industry, 1 research,
// 2 prosperity (wanted's order), read from the item table: an item of
// no MaxPop making exactly one of the three (Factory, Industrial
// Megafacility; Laboratory, Research Campus; Agriplot, Artificial
// Hydroponifer). -1 for any other.
func family(it game.Item) int {
	in := it.Info()
	if !it.Surface() || in.MaxPop > 0 {
		return -1
	}
	f := -1
	for k, v := range [3]int{in.Industry, in.Research, in.Prosp} {
		switch {
		case v <= 0:
		case f >= 0:
			return -1
		default:
			f = k
		}
	}
	return f
}

// output is it's output of family f.
func output(it game.Item, f int) int {
	in := it.Info()
	return [3]int{in.Industry, in.Research, in.Prosp}[f]
}

// bestTier is family f's item of most output r allows and ok takes
// (nil: any), ties the cheaper then the lower index; NoItem when none.
func bestTier(r *game.Research, f int, ok func(game.Item) bool) game.Item {
	best := game.NoItem
	for it := game.Item(0); it < game.FirstOrbital; it++ {
		if family(it) != f || !r.Allows(it) || ok != nil && !ok(it) {
			continue
		}
		if best == game.NoItem || output(it, f) > output(best, f) ||
			output(it, f) == output(best, f) && it.Info().Cost < best.Info().Cost {
			best = it
		}
	}
	return best
}

// roomFor reports a BestCell for one of items on col's planet.
func roomFor(col *game.Colony, items []game.Item) bool {
	for _, it := range items {
		if _, ok := col.Planet.BestCell(it, col.Traits()); ok {
			return true
		}
	}
	return false
}

// housing reports a surface structure adding population room, the Colony
// Base aside (a colony's founding, not a build).
func housing(it game.Item) bool {
	return it.Surface() && it != game.ColonyBase && it.Info().MaxPop > 0
}

// GenomePick is the genome's structure pick among the items of g.Build >
// 0 that can start (ColonyBase, Shipyard and Ship are build's) and
// upgradeOpen lets through: the highest g.Build + g.Balance·wants
// (ResShare 0 past the singularity) − g.CostW·Cost/100, ties the lower
// index. It starts nothing: at is the cell, slot the orbit slot (-1 for
// a surface item); NoItem when none can.
func GenomePick(col *game.Colony, r *game.Research, g *Genome) (it game.Item, at game.Cell, slot int) {
	makes := wants(col, g.Economy().singular(r))
	it, slot = game.NoItem, -1
	bestScore := 0.0
	for c := game.Item(0); c < game.FirstProject; c++ {
		if c == game.ColonyBase || c == game.Shipyard || c == game.Ship || g.Build[c] <= 0 || !upgradeOpen(col, c, g.Upgrade) {
			continue
		}
		cAt, cSlot := game.Cell{}, -1
		if c.Orbital() {
			for sl := range game.OrbitSlots {
				if col.CanStartOrbital(r, c, sl) == nil {
					cSlot = sl
					break
				}
			}
			if cSlot < 0 {
				continue
			}
		} else {
			var ok bool
			if cAt, ok = col.Planet.BestCell(c, col.Traits()); !ok || col.CanStart(r, c, cAt) != nil {
				continue
			}
		}
		in := c.Info()
		score := g.Build[c] - g.CostW*float64(in.Cost)/100
		if makes(in) {
			score += g.Balance
		}
		if it == game.NoItem || score > bestScore {
			it, at, slot, bestScore = c, cAt, cSlot, score
		}
	}
	return it, at, slot
}

// PickTopicBy studies, when r has no topic, the Available tech of the
// highest g.Tech[t] − g.ResCostW·ln(Cost(t)), ties the lower index.
func PickTopicBy(r *game.Research, g *Genome) {
	if best := topicBy(r, g); best != game.NoTech {
		r.Study(best)
	}
}

// topicBy is PickTopicBy's choice, NoTech for none.
func topicBy(r *game.Research, g *Genome) game.Tech {
	if r.Topic != game.NoTech {
		return game.NoTech
	}
	best, bestScore := game.NoTech, 0.0
	for _, t := range r.Available() {
		score := g.Tech[t] - g.ResCostW*math.Log(float64(max(1, r.Cost(t))))
		if best == game.NoTech || score > bestScore {
			best, bestScore = t, score
		}
	}
	return best
}

// PickTopic studies, when r has no topic and none is false, the queue's
// head (known heads dropped), the queue done the way to AimItems' techs
// (aimTopic), else Available()[0]; a head not yet studiable stays queued
// (Available()[0] meanwhile).
func PickTopic(r *game.Research, queue *[]game.Tech, none bool) {
	if t := nextTopic(r, queue, none); t != game.NoTech {
		r.Study(t)
	}
}

// nextTopic is PickTopic's choice, NoTech for none, the queue updated.
func nextTopic(r *game.Research, queue *[]game.Tech, none bool) game.Tech {
	if r.Topic != game.NoTech || none {
		return game.NoTech
	}
	q := *queue
	for len(q) > 0 && r.Known[q[0]] {
		q = q[1:]
	}
	t := game.NoTech
	if len(q) > 0 && r.CanStudy(q[0]) {
		t = q[0]
		q = q[1:]
	} else if len(q) == 0 {
		t = aimTopic(r)
	}
	if av := r.Available(); t == game.NoTech && len(av) > 0 {
		t = av[0]
	}
	*queue = q
	return t
}

// aimTopic is the topic toward the first AimItems tech not known: that
// tech when Available, else the cheapest Available one among its unknown
// prerequisites, transitively (ties the lower index; one always is).
// NoTech once every aim is known.
func aimTopic(r *game.Research) game.Tech {
	for _, it := range AimItems {
		aim := game.Tech(it.Info().ResearchReq)
		if r.Known[aim] {
			continue
		}
		best := game.NoTech
		var walk func(t game.Tech)
		walk = func(t game.Tech) {
			switch {
			case r.Known[t]:
			case r.CanStudy(t):
				if best == game.NoTech || r.Cost(t) < r.Cost(best) || r.Cost(t) == r.Cost(best) && t < best {
					best = t
				}
			default:
				for _, p := range game.Techs[t].Requires {
					walk(p)
				}
			}
		}
		walk(aim)
		return best
	}
	return game.NoTech
}

// foundable reports a planet a Colonizer may settle (settleable) of Size
// at least floor.
func (b *Bot) foundable(star, pl int) bool {
	return b.E.Planet(star, pl).Size >= b.floor && b.settleable(star, pl)
}

// settleable reports a planet at a star the empire Knows, uninhabited, not
// Barred (its orbit refused), in no ally's system (CanFound refuses it)
// nor a courted one's, with a square for a Colony Base.
func (b *Bot) settleable(star, pl int) bool {
	if !b.E.Knows(star) || b.E.Inhabited(star, pl) || b.E.Barred(star, pl) || b.E.AllySystem(star) || b.courted(star) {
		return false
	}
	_, ok := b.E.Planet(star, pl).BestCell(game.ColonyBase, b.E.Species.Traits())
	return ok
}

// courted reports star holds a colony of an empire at Peace with me that
// I mean to keep at peace (my attitude at or over my war threshold, or
// none read yet): founding beside it is an affront its ledger keeps
// (Relation.Encroached, GrudgeEncroach). Wanting war, I settle anyway.
func (b *Bot) courted(star int) bool {
	if b.Cfg.NoDiplomacy { // no attitude read: no affront weighed either
		return false
	}
	var th Personality
	read := false
	for _, p := range b.E.Cluster.Stars[star].System.Planets {
		j := p.Owner
		if j < 0 || j == b.E.Index || b.E.StanceWith(j) != game.Peace {
			continue
		}
		if !read {
			th, read = b.Thresholds(), true
		}
		if att, ok := b.Attitude(j); !ok || att >= th.DeclareWar {
			return true
		}
	}
	return false
}

type place struct{ star, planet int }

// claimed lists the planets ships are heading to or about to settle, and,
// as place{star, -1}, the stars ships are heading to (lane, GoToStar or
// dest): one explorer each. It sets floor first (setFloor), prunes dest of
// the ships gone or idle, rebuilds inbound, and sets spare and inFlight.
func (b *Bot) claimed() map[place]bool {
	b.setFloor()
	m := map[place]bool{}
	b.inbound = map[int]int{}
	live := map[*game.Starship]bool{}
	for _, s := range b.E.Ships {
		live[s] = true
	}
	for s, to := range b.dest {
		if !live[s] || s.Lane < 0 && s.Order.Kind == game.NoOrder {
			delete(b.dest, s)
		} else {
			m[place{to, -1}] = true
			b.inbound[to] += s.Cargo
		}
	}
	for _, s := range b.E.Ships {
		switch {
		case s.Lane >= 0:
			m[place{b.E.Cluster.Other(s.Lane, s.Star), -1}] = true
		case s.Order.Kind == game.GoToStar:
			m[place{s.Order.Star, -1}] = true
		case s.Order.Kind == game.GoToPlanet:
			m[place{s.Star, s.Order.Planet}] = true
		case s.Order.Kind == game.Colonise || s.Order.Kind == game.Invade:
			m[place{s.Star, s.Orbit}] = true
		}
	}
	// spare: targets plus frontier() per unexplored star next to a known
	// one, less the Colonizers of idle ships able to go and of ships built.
	b.spare = 0
	c := b.E.Cluster
	for st := range c.Stars {
		if b.E.Knows(st) {
			b.spare += b.targets(st, m)
			continue
		}
		if !b.unexplored(st, m) {
			continue
		}
		for _, li := range c.Stars[st].Lanes {
			if b.E.Knows(c.Other(li, st)) {
				b.spare += b.frontier()
				break
			}
		}
	}
	// inFlight: the ships under orders or in a lane, and the idle ones
	// able to go (planShip sends them after build: last night's colony
	// ship); an idle ship's leftover with no drive and nothing in reach
	// blocks nothing.
	b.inFlight = 0
	for _, s := range b.E.Ships {
		switch {
		case s.Lane >= 0 || s.Order.Kind != game.NoOrder:
			b.inFlight += s.Cargo
		case s.Design.LaneDrives() > 0 || b.targets(s.Star, m) > 0:
			b.spare -= s.Cargo
			b.inFlight += s.Cargo
		}
	}
	for _, col := range b.E.Colonies {
		if d := building(col); d != nil {
			b.spare -= d.Colonizers()
			b.inFlight += d.Colonizers()
		}
	}
	return m
}

// setFloor sets floor to the Genome's FoundFloor (rounded), else
// foundFloor, while the empire knows a settleable planet of that Size or
// more (claimed or not: a big planet under way holds the floor up until
// settled); else to 1, every size.
func (b *Bot) setFloor() {
	want := foundFloor
	if g := b.Cfg.Genome; g != nil {
		want = int(math.Round(g.FoundFloor))
	}
	b.floor = 1
	if want <= 1 {
		return
	}
	for st, star := range b.E.Cluster.Stars {
		if !b.E.Knows(st) {
			continue
		}
		for pl, p := range star.System.Planets {
			if p.Size >= want && b.settleable(st, pl) {
				b.floor = want
				return
			}
		}
	}
}

// inFlightMax is the Genome's InFlight (rounded), else inFlightCap; at
// least hardInFlight when Hard.
func (b *Bot) inFlightMax() int {
	n := inFlightCap
	if g := b.Cfg.Genome; g != nil {
		n = int(math.Round(g.InFlight))
	}
	if b.Cfg.Difficulty == Hard {
		n = max(n, hardInFlight)
	}
	return n
}

// building is the design of the new ship col builds, nil when none (a
// refit is a Ship project too, of no new ship).
func building(col *game.Colony) *game.Design {
	if pj := col.Project; pj != nil && pj.Item == game.Ship && pj.Design != nil {
		return pj.Design
	}
	return nil
}

// frontier is the places an unexplored frontier star counts for: the
// Genome's FrontierWorth (rounded), else frontierWorth.
func (b *Bot) frontier() int {
	if g := b.Cfg.Genome; g != nil {
		return int(math.Round(g.FrontierWorth))
	}
	return frontierWorth
}

// idleParty is the Genome's IdleParty, else 0.
func (b *Bot) idleParty() float64 {
	if g := b.Cfg.Genome; g != nil {
		return g.IdleParty
	}
	return 0
}

// automateCostW is the Genome's AutomateCostW, else 0 (Cells order).
func (b *Bot) automateCostW() float64 {
	if g := b.Cfg.Genome; g != nil {
		return g.AutomateCostW
	}
	return 0
}

// ruinW is the Genome's RuinWorth, else ruinWorth.
func (b *Bot) ruinW() float64 {
	if g := b.Cfg.Genome; g != nil {
		return g.RuinWorth
	}
	return ruinWorth
}

// ruins reports planet pl of star foundable with Xeno ruins.
func (b *Bot) ruins(star, pl int) bool {
	if !b.foundable(star, pl) {
		return false
	}
	_, ok := b.E.Planet(star, pl).RuinsAt()
	return ok
}

// ruinsTarget reports an unclaimed foundable planet with ruins at star.
func (b *Bot) ruinsTarget(star int, claimed map[place]bool) bool {
	for pl := range b.E.Cluster.Stars[star].System.Planets {
		if !claimed[place{star, pl}] && b.ruins(star, pl) {
			return true
		}
	}
	return false
}

// unexplored reports star unknown and no ship on its way there.
func (b *Bot) unexplored(star int, claimed map[place]bool) bool {
	return !b.E.Knows(star) && !claimed[place{star, -1}]
}

// targets counts the unclaimed foundable planets of a known star less the
// Colonizers inbound to it, never below 0.
func (b *Bot) targets(star int, claimed map[place]bool) int {
	if !b.E.Knows(star) {
		return 0
	}
	n := 0
	for pl := range b.E.Cluster.Stars[star].System.Planets {
		if !claimed[place{star, pl}] && b.foundable(star, pl) {
			n++
		}
	}
	return max(0, n-b.inbound[star])
}

// reachable reports a target for a ship of design d built at col: in its
// system, or, with a lane drive, any star route finds (targets or
// unexplored).
func (b *Bot) reachable(col *game.Colony, d game.Design, claimed map[place]bool) bool {
	if b.targets(col.Star, claimed) > 0 {
		return true
	}
	if d.LaneDrives() == 0 {
		return false
	}
	to, _ := b.route(col.Star, d.LaneStrength(), col.Traits(), claimed, nil)
	return to >= 0
}

// route picks a far destination for a lane ship at star from, by search
// (ThreatAvoid): the star with targets and no hostile damage of least
// cost, less ruinW with a ruinsTarget, ties the first settled; else the
// first unexplored one; to -1 when none.
func (b *Bot) route(from int, strength float64, tr game.Traits, claimed map[place]bool, blocked func(v int) bool) (to, first int) {
	return b.routeOr(from, strength, tr, claimed, blocked, math.Inf(1))
}

// routeOr is route against a standing offer of cost here (planShip: a
// planet of from's own system, 0): only a cheaper star wins, and no
// unexplored one when here is finite. Stars settle by cost, so the search
// stops once d − ruinW cannot beat the best.
func (b *Bot) routeOr(from int, strength float64, tr game.Traits, claimed map[place]bool, blocked func(v int) bool, here float64) (to, first int) {
	to, first = -1, -1
	w := b.ruinW()
	best, bestFirst, bestCost := -1, -1, here
	b.searchDays(from, strength, tr, b.cg.ThreatAvoid, blocked, func(u, f int, d float64) bool {
		if d-w >= bestCost {
			return true // u and every star after it cost at least bestCost
		}
		if b.targets(u, claimed) > 0 && b.their(u).dmg <= 0 {
			cost := d
			if w > 0 && b.ruinsTarget(u, claimed) {
				cost -= w
			}
			if cost < bestCost {
				best, bestFirst, bestCost = u, f, cost
			}
			return d-w >= bestCost
		}
		if to < 0 && math.IsInf(here, 1) && b.unexplored(u, claimed) {
			to, first = u, f
		}
		return false
	})
	if best >= 0 {
		return best, bestFirst
	}
	return to, first
}

// ColonyDays is the build time past which a colony ship steps down a
// hull (colonyShip).
const ColonyDays = 60

// colonyShip is col's colony ship, none while spare < 1. With Colonizers
// in flight, game.ColonyShip when it keeps them within inFlightMax. With
// none (a larger hull settles several planets on one
// trip), game.ColonyShipOn every known hull with at most spare
// Colonizers: of those col builds within ColonyDays (Colony.DaysFor) the
// one carrying the most, the smallest hull on a tie; none within, the
// smallest hull's.
func (b *Bot) colonyShip(col *game.Colony) (game.Design, bool) {
	want := b.spare
	if want < 1 {
		return game.Design{}, false
	}
	r := b.E.Research
	lane := r.AllowsGizmo(game.StarLaneDrive)
	if b.inFlight > 0 {
		d, ok := game.ColonyShip(r, lane)
		return d, ok && b.inFlight+d.Colonizers() <= b.inFlightMax()
	}
	var best, first game.Design
	okBest, okFirst := false, false
	for h := game.Hull(0); h < game.NumHulls; h++ {
		d, ok := game.ColonyShipOn(r, h, lane, want)
		if !ok {
			continue
		}
		if !okFirst {
			first, okFirst = d, true
		}
		if days, ok := col.DaysFor(d.Cost()); ok && days <= ColonyDays && (!okBest || d.Colonizers() > best.Colonizers()) {
			best, okBest = d, true
		}
	}
	if okBest {
		return best, true
	}
	return first, okFirst
}

// scouts is the explorers an empire keeps flying or building while a
// frontier star is left (else the bots explore only the
// stars they settle, so they never meet).
const scouts = 2

// explorer reports d a scout: ScoutShip's name (a warship disarmed by
// LosePart stays a warship), no Colonizer, no Invasion Module.
func explorer(d game.Design) bool {
	return d.Name == game.ScoutName && d.Colonizers() == 0 && d.InvasionModules() == 0
}

// wantScout reports a scout should start: fewer than scouts, and than
// the systems I control, flying and building; a fleet slot left free
// after it (the cap, ShipsPerSystem a system, would else hold no colony
// ship: one system never grows); and a frontier star (unexplored,
// unclaimed, next to a known one) left.
func (b *Bot) wantScout(claimed map[place]bool) bool {
	if n, capa := b.E.Fleet(); n+1 >= capa {
		return false
	}
	n := 0
	for _, s := range b.E.Ships {
		if explorer(s.Design) {
			n++
		}
	}
	for _, col := range b.E.Colonies {
		if d := building(col); d != nil && explorer(*d) {
			n++
		}
	}
	if n >= min(scouts, b.E.ControlledSystems()-1) {
		return false
	}
	c := b.E.Cluster
	for st := range c.Stars {
		if !b.unexplored(st, claimed) {
			continue
		}
		for _, li := range c.Stars[st].Lanes {
			if b.E.Knows(c.Other(li, st)) {
				return true
			}
		}
	}
	return false
}

// explore sends an idle scout s, out of a lane, to the nearest unexplored
// star unclaimed (search, ThreatAvoid, the hostiles' stars blocked),
// claiming it; with none, to my nearest colony's star to wait there (an
// unscanned ship parked by a neighbour's colony looks armed to it:
// armedSeen).
func (b *Bot) explore(s *game.Starship, claimed map[place]bool) {
	if s.Order.Kind != game.NoOrder || s.Lane >= 0 {
		return
	}
	strength, tr, danger := s.Design.LaneStrength(), s.Species.Traits(), b.danger(s)
	to, first := b.search(s.Star, strength, tr, b.cg.ThreatAvoid, danger, func(u, _ int) bool {
		return b.unexplored(u, claimed)
	})
	if to >= 0 {
		if b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first}) == nil {
			claimed[place{to, -1}] = true
			b.dest[s] = to
		}
		return
	}
	if b.colonyAt(s.Star) {
		return
	}
	if to, first = b.search(s.Star, strength, tr, b.cg.ThreatAvoid, danger, func(u, _ int) bool { return b.colonyAt(u) }); to >= 0 {
		b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first})
	}
}

// retireScouts abandons my idle scouts, out of a lane, once I know every
// star: their fleet slots go to other ships.
func (b *Bot) retireScouts() {
	for st := range b.E.Cluster.Stars {
		if !b.E.Knows(st) {
			return
		}
	}
	var drop []*game.Starship
	for _, s := range b.E.Ships {
		if explorer(s.Design) && s.Lane < 0 && s.Order.Kind == game.NoOrder {
			drop = append(drop, s)
		}
	}
	for _, s := range drop { // Abandon deletes from Ships
		b.E.Abandon(s)
	}
}

// planColony starts a project on col (build); when nothing starts, idle
// picks its ongoing project.
func (b *Bot) planColony(i int, col *game.Colony, claimed map[place]bool) {
	pj := col.Project
	if b.build(i, col, claimed); col.Project == pj {
		b.idle(col)
	}
}

// idle runs an ongoing project on col: Alien Hospitality when hospitable;
// else an Endless Party while the room left is above idleParty, else a
// Scientist Takeover (whichever is researched; never past the
// singularity, CanStartProject refuses it). The right one running is
// kept.
func (b *Bot) idle(col *game.Colony) {
	r := b.E.Research
	party, sci := r.Allows(game.EndlessParty), r.Allows(game.ScientistTakeover) && !r.Complete()
	var it game.Item
	switch capa := col.Capacity(); {
	case r.Allows(game.AlienHospitality) && b.hospitable(col):
		it = game.AlienHospitality
	case !party && !sci:
		return
	case !sci || party && float64(capa-col.Pop) > b.idleParty()*float64(capa):
		it = game.EndlessParty
	default:
		it = game.ScientistTakeover
	}
	if pj := col.Project; pj == nil || pj.Item != it {
		col.StartProject(r, it) // a refusal (none known): it stays as it is
	}
}

// hospitable reports col may run Alien Hospitality: some empire is at
// Peace with me (the hospitality courts the undecided), and fewer of my
// other colonies run it than ⌈Hospitable × my colonies⌉, so the first in
// Colonies order take the share, the same ones day after day.
func (b *Bot) hospitable(col *game.Colony) bool {
	if !b.stanced(game.Peace) {
		return false
	}
	share := int(math.Ceil(b.dipGenes().Hospitable * float64(len(b.E.Colonies))))
	n := 0
	for _, c := range b.E.Colonies {
		if c != col && c.Project != nil && c.Project.Item == game.AlienHospitality {
			n++
		}
	}
	return n < share
}

// yielded is col without its ongoing project (a read-only shallow copy):
// what the picks weigh, an ongoing project switching industry off.
func yielded(col *game.Colony) *game.Colony {
	if pj := col.Project; pj != nil && pj.Item.Ongoing() {
		cp := *col
		cp.Project = nil
		return &cp
	}
	return col
}

// build starts col's first project that can start, in this order: the
// Xeno Archeological Dig on its ruins; the home's script (waiting on a
// refusal); Terraforming, once known, while a black square is left
// (terraformAt); a Shipyard once Orbital Structures is known and
// ShipyardFactories (2) Factories stand; with contact, the empire's one
// Orbital Docks (both wait for free people rather than fall through);
// at a shipyard, a warship (wantWarship, its hull warship's); at War, a
// defence (fortify: shields, orbital shields, orbital weapons, cloakers;
// it waits for free people while the colony can grow); at a shipyard, an
// invasion ship (invasionDesign; after a scout, wantScout), a colony ship
// (colonyShip, reachable), a Habitat ship (habitatDesign); under Greedy, an
// Automation (automateAt), else with a Genome a replacement
// (replacePick, when ReplaceDays > 0 and, at ReplaceFull ≥ 0.5, the
// planet is full) or GenomePick, else GreedyPick, and when that finds
// nothing with the population capped or no square left (roomFor) an
// upgrade (tierReplace).
func (b *Bot) build(i int, col *game.Colony, claimed map[place]bool) {
	r := b.E.Research
	if r.Allows(game.XenoArcheologicalDig) {
		if at, ok := col.Planet.RuinsAt(); ok && col.Start(r, game.XenoArcheologicalDig, at) == nil {
			return
		}
	}
	if day, ok := b.Founded[col]; i == 0 && ok && day == 0 && b.next < len(b.Cfg.Script) {
		if Try(col, r, b.Cfg.Script[b.next]) == nil {
			b.next++
		}
		return
	}
	if r.Allows(game.Terraforming) {
		if at, ok := terraformAt(col.Planet); ok && col.Start(r, game.Terraforming, at) == nil {
			return
		}
	}
	p := col.Planet
	factories := 2
	if g := b.Cfg.Genome; g != nil {
		factories = int(math.Round(g.ShipyardFactories))
	}
	if r.Known[game.OrbitalStructures] && !p.HasShipyard() && p.Count(game.Factory) >= factories {
		err := Try(col, r, game.Shipyard)
		if err == nil || errors.Is(err, game.ErrNoFreePop) {
			return
		}
	}
	if b.contact && p.HasShipyard() && r.Allows(game.OrbitalDocks) && !b.docksPlanned() {
		err := Try(col, r, game.OrbitalDocks)
		if err == nil || errors.Is(err, game.ErrNoFreePop) {
			return
		}
	}
	shipyard := p.HasShipyard()
	if shipyard && b.wantWarship(col) {
		if d, ok := b.warship(col); ok && b.E.BuildShip(col, d) == nil {
			return
		}
	}
	if b.fortify(col) {
		return
	}
	if shipyard {
		if b.wantScout(claimed) {
			if d, ok := game.ScoutShip(r); ok && b.E.BuildShip(col, d) == nil {
				return
			}
		}
		if d, ok := b.invasionDesign(col); ok && b.E.BuildShip(col, d) == nil {
			return
		}
		if d, ok := b.colonyShip(col); ok && b.reachable(col, d, claimed) {
			if b.E.BuildShip(col, d) == nil {
				b.spare -= d.Colonizers()
				b.inFlight += d.Colonizers()
				return
			}
		}
		if d, ok := b.habitatDesign(col); ok && b.E.BuildShip(col, d) == nil {
			b.hab.ships++
			return
		}
	}
	if b.Cfg.Greedy {
		if at, ok := automateAt(col, r, b.automateCostW()); ok && col.Start(r, game.Automation, at) == nil {
			return
		}
	}
	if g := b.Cfg.Genome; g != nil && b.Cfg.Greedy {
		if g.ReplaceDays > 0 && (g.ReplaceFull < 0.5 || full(col, r, g)) {
			if at, it, ok := replacePick(col, r, g); ok {
				if err := col.Replace(r, at, it); err != nil {
					panic(err) // replacePick checked CanReplace
				}
				return
			}
		}
		if it, at, slot := GenomePick(col, r, g); it != game.NoItem {
			var err error
			if slot >= 0 {
				err = col.StartOrbital(r, it, slot)
			} else {
				err = col.Start(r, it, at)
			}
			if err != nil {
				panic(err) // GenomePick checked the same cell or slot
			}
		}
	} else if b.Cfg.Greedy {
		ec := b.econ()
		if it := GreedyPick(col, r, ec); it != game.NoItem {
			if err := Try(col, r, it); err != nil {
				panic(err) // GreedyPick checked CanStart at the same cell
			}
		} else if col.Pop >= col.Capacity() || !roomFor(col, greedyItems(col, r, ec)) {
			if at, it, ok := tierReplace(col, r); ok {
				if err := col.Replace(r, at, it); err != nil {
					panic(err) // tierReplace checked CanReplace
				}
			}
		}
	}
}

// terraformAt is the black square p whitens first (CanTerraform): an
// empty one, else one under Tubes, in Cells order.
func terraformAt(p *game.Planet) (game.Cell, bool) {
	var tubes []game.Cell
	for _, c := range p.Cells() {
		if !p.CanTerraform(c) {
			continue
		}
		if p.Surface[c].Item == game.NoItem {
			return c, true
		}
		tubes = append(tubes, c)
	}
	if len(tubes) > 0 {
		return tubes[0], true
	}
	return game.Cell{}, false
}

// automateAt is the structure col automates rather than stay short of
// workers: research allows Automation, Pop is at Capacity, the last free
// worker is the one an Automation takes, and more than that many empty
// squares would take a Factory. Automating hands back two workers, so the
// bot alternates automating and building instead of stalling. The pick is
// the non-automated structure employing someone (Item.Staff) of least
// costW × Cost, ties in Cells order; false when none.
func automateAt(col *game.Colony, r *game.Research, costW float64) (game.Cell, bool) {
	if !r.Allows(game.Automation) || col.Pop < col.Capacity() || col.Free() != game.Automation.Info().UsePop {
		return game.Cell{}, false
	}
	p, tr := col.Planet, col.Traits()
	squares := 0
	for _, c := range p.Cells() {
		if p.CanBuild(c, game.Factory, tr) {
			squares++
		}
	}
	if squares <= col.Free() {
		return game.Cell{}, false
	}
	var best game.Cell
	bestScore, found := 0.0, false
	for _, c := range p.Cells() {
		sq := p.Surface[c]
		if sq.Item == game.NoItem || sq.Automated || sq.Item.Staff() == 0 {
			continue
		}
		if score := costW * float64(sq.Item.Info().Cost); !found || score < bestScore {
			best, bestScore, found = c, score, true
		}
	}
	return best, found
}

// planShip gives an idle ship with Colonizers an order: Colonise the
// foundable planet it orbits; else GoToPlanet the nearest unclaimed
// foundable planet of its system (ruins first when ruinW > 0), unless
// routeOr finds a ruins star within ruinW lane days; else, with a lane
// drive, GoToStar route's first hop (never into nor through a star in
// danger for it), the destination in dest; else it idles.
func (b *Bot) planShip(s *game.Starship, claimed map[place]bool) {
	if s.Order.Kind != game.NoOrder || s.Lane >= 0 || s.Cargo < 1 {
		return
	}
	if s.Orbit >= 0 && !claimed[place{s.Star, s.Orbit}] && b.foundable(s.Star, s.Orbit) {
		b.E.Order(s, game.Order{Kind: game.Colonise})
		claimed[place{s.Star, s.Orbit}] = true
		return
	}
	c := b.E.Cluster
	w, strength := b.ruinW(), s.Design.LaneStrength()
	best, bestD, bestRuins := -1, 0.0, false
	for pl, p := range c.Stars[s.Star].System.Planets {
		if claimed[place{s.Star, pl}] || !b.foundable(s.Star, pl) {
			continue
		}
		r := w > 0 && b.ruins(s.Star, pl)
		if d := dist(s.Pos, p.Pos); best < 0 || r && !bestRuins || r == bestRuins && d < bestD {
			best, bestD, bestRuins = pl, d, r
		}
	}
	goStar := func(to, first int) {
		b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first})
		claimed[place{to, -1}] = true
		b.dest[s] = to
		b.inbound[to] += s.Cargo
	}
	if best >= 0 {
		if !bestRuins && w > 0 && strength > 0 {
			if to, first := b.routeOr(s.Star, strength, s.Species.Traits(), claimed, b.danger(s), 0); to >= 0 {
				goStar(to, first)
				return
			}
		}
		b.E.Order(s, game.Order{Kind: game.GoToPlanet, Planet: best})
		claimed[place{s.Star, best}] = true
		return
	}
	if strength == 0 {
		return
	}
	if to, first := b.route(s.Star, strength, s.Species.Traits(), claimed, b.danger(s)); to >= 0 {
		goStar(to, first)
	}
}

func dist(a, b game.Vec3) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
}

// NewAll is one bot per empire of g, in order, all playing cfg, G set.
func NewAll(g *game.Game, cfg Config) []*Bot {
	bots := make([]*Bot, len(g.Empires))
	for i, e := range g.Empires {
		bots[i] = New(e, cfg)
		bots[i].G = g
	}
	return bots
}

// RoundCap is the most rounds a day plays at one combat star.
const RoundCap = 30

// rounds is b's round cap: its Rounds gene rounded.
func (b *Bot) rounds() int { return int(math.Round(b.genes().Rounds)) }

// Day plays one day of g, bots[i] playing g.Empires[i] (nil or past the
// end: the window's player): Begin, then Finish (every combat star played
// as DayRun.Auto plays it). Reports and RoundLogs come in play order.
func Day(g *game.Game, bots []*Bot) (evs [][]game.Event, reps []game.Report, rounds []game.RoundLog) {
	return Begin(g, bots).Finish()
}
