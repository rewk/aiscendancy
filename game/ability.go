package game

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/rewk/aiscendancy/journal"
)

// The species' timed abilities (docs/rules.md "Species abilities"; source
// help.txt's abilNN / abresNN texts).

// BlockDays is how long the Arbryls' SA_BlockStarLanes closes the lanes.
// ASSUMPTION: the help gives no duration.
const BlockDays = 20

// MebesPopBonus is the Capacity a SA_MoreMaxPop use adds to each colony:
// measured in DOSBox.
const MebesPopBonus = 2

// abilities is each timed species' use: refused (nothing changed), or the
// AbilityUsed text and the others' Reports; a use with a place returns
// i's own AbilityUsed first, placed, which UseAbility keeps as its own.
var abilities = map[Species]func(g *Game, i int) (string, []Report, error){
	Baliflids: func(g *Game, i int) (string, []Report, error) {
		reps, err := g.forcePeace(i)
		return "Peace forced on all", reps, err
	},
	Marmosians: func(g *Game, i int) (string, []Report, error) {
		reps, err := g.causeWar(i)
		return "Aliens turned on our enemies", reps, err
	},
	Fludentri:   (*Game).healShips,
	Swaparamans: (*Game).doublePower,
	Shevar:      (*Game).sapAlienPower,
	Dubtaks:     (*Game).stealResearch,
	Chamachies:  (*Game).freeResearch,
	Nimbuloids:  (*Game).industryBoost,
	Mebes:       (*Game).moreMaxPop,
	Capelons:    (*Game).shieldPlanets,
	Ungooma:     (*Game).bumpShipsInLanes,
	Frutmaka:    (*Game).repelAlienShips,
	Arbryls:     (*Game).blockStarLanes,
	Govorom:     (*Game).cornucopia,
}

// ErrAbilityAsleep refuses a timed ability while its empire controls no
// system (AbilityAsleep).
var ErrAbilityAsleep = errors.New("no system controlled: the ability sleeps")

// AbilityAsleep reports empire i's timed ability off: the empire
// controls no system (the Ungooma bumping ships
// with nothing left; it wakes with a system controlled again).
func (g *Game) AbilityAsleep(i int) bool {
	e := g.Empires[i]
	return e.Species.Info().Recharge > 0 && e.ControlledSystems() == 0
}

// UseAbility uses empire i's timed ability, refused when defeated,
// passive, asleep, not ready, or by the species' own function. A use sets
// AbilityUsed; its Reports start with i's AbilityUsed.
func (g *Game) UseAbility(i int) ([]Report, error) {
	e := g.Empires[i]
	if err := g.defeatedErr(i); err != nil {
		return nil, err
	}
	info := e.Species.Info()
	if info.Recharge == 0 {
		return nil, fmt.Errorf("the %s's ability is passive", e.Species)
	}
	if g.AbilityAsleep(i) {
		return nil, ErrAbilityAsleep
	}
	if ready, days := g.AbilityState(i); !ready {
		return nil, fmt.Errorf("ready in %d days", days)
	}
	use, ok := abilities[e.Species]
	if !ok {
		return nil, fmt.Errorf("%s: %w", info.AbilityText, ErrNotModelled)
	}
	text, reps, err := use(g, i)
	if err != nil {
		return nil, err
	}
	e.AbilityUsed = g.Day
	if g.journal != nil {
		g.journal.Record(journal.NewEntry(g.Day, i, "ability"))
	}
	if len(reps) > 0 && reps[0].Empire == i && reps[0].Event.Kind == AbilityUsed {
		return reps, nil
	}
	return append([]Report{{i, dipEvent(AbilityUsed, -1, text)}}, reps...), nil
}

// abilitiesReady is the close's AbilityReady Reports: one to each living
// timed empire whose ability recharges today (Day − AbilityUsed ==
// Recharge exactly, so once a charge). An asleep empire's recharge
// pauses instead: AbilityUsed moves a day on, and
// up to Recharge − 1 days behind, so a charge full before the sleep is
// due again the close after the waking, with its Report.
func (g *Game) abilitiesReady() []Report {
	var reps []Report
	for i, e := range g.Empires {
		rc := e.Species.Info().Recharge
		switch {
		case rc == 0 || !e.Alive():
		case g.AbilityAsleep(i):
			e.AbilityUsed = max(e.AbilityUsed+1, g.Day-rc+1)
		case g.Day-e.AbilityUsed == rc:
			reps = append(reps, Report{i, dipEvent(AbilityReady, -1, "Special ability ready")})
		}
	}
	return reps
}

// healShips is the Fludentri's SA_HealShips ("Repair all ships'
// damage"): every ship of i at MaxHull; refused when none is damaged.
func (g *Game) healShips(i int) (string, []Report, error) {
	ships := g.Empires[i].Ships
	if !slices.ContainsFunc(ships, func(s *Starship) bool { return s.Hull < s.MaxHull() }) {
		return "", nil, errors.New("nothing to repair")
	}
	for _, s := range ships {
		s.Hull = s.MaxHull()
		s.healed()
	}
	return "All ships repaired", nil, nil
}

// doublePower is the Swaparamans' SA_DoublePower ("Double all ships'
// power"): every ship's Power left doubled until the Refill.
func (g *Game) doublePower(i int) (string, []Report, error) {
	ships := g.Empires[i].Ships
	if len(ships) == 0 {
		return "", nil, errors.New("no ship")
	}
	for _, s := range ships {
		s.Power *= 2
	}
	return "Ship power doubled today", nil, nil
}

// sapAlienPower is the Shevar's SA_SapAlienPower ("Wipe out the power in
// alien ships in any system you occupy"): every other empire's ship out
// of the lanes where i is Present has Power 0, unarmed, its shields'
// reservation taken too: on ones go dark and Sapped, off
// refunding nothing until the Refill; a Report per empire hit.
func (g *Game) sapAlienPower(i int) (string, []Report, error) {
	me := g.Empires[i]
	var reps []Report
	for _, o := range g.Empires {
		if o == me {
			continue
		}
		hit := false
		for _, s := range o.Ships {
			if s.Lane < 0 && me.Present(s.Star) {
				s.Power, s.Armed = 0, nil
				if s.Shields {
					s.ShieldsDown, s.Sapped = true, true
				}
				hit = true
			}
		}
		if hit {
			reps = append(reps, Report{o.Index, dipEvent(AbilityUsed, i, "Ships drained by "+me.Species.String())})
		}
	}
	if reps == nil {
		return "", nil, errors.New("no alien ship in our systems")
	}
	return "Alien ships drained", reps, nil
}

// stealResearch is the Dubtaks' SA_StealResearch ("Steal any technology
// owned by two or more races"): Granted in tech order, met or not
// (ASSUMPTION), a Discovered Report each (the singularity's after the
// last when it completes the tree).
func (g *Game) stealResearch(i int) (string, []Report, error) {
	r := g.Empires[i].Research
	var reps []Report
	for t := range NumTechs {
		if r.Known[t] {
			continue
		}
		n := 0
		for _, o := range g.Empires {
			if o.Index != i && o.Research.Known[t] {
				n++
			}
		}
		if n >= 2 {
			r.Grant(t)
			reps = append(reps, Report{i, Event{Kind: Discovered, Item: NoItem, Tech: t, Star: -1, Planet: -1, Other: -1,
				Text: t.String() + " stolen"}})
			if x, ok := singularity(r, t); ok {
				reps = append(reps, Report{i, x})
			}
		}
	}
	if reps == nil {
		return "", nil, errors.New("nothing to steal")
	}
	g.Empires[i].stopTakeovers()
	return "Techs stolen", reps, nil
}

// freeResearch is the Chamachies' SA_FreeResearch ("Achieve any
// discovery in one day"): the topic's Progress at its Cost, discovered at
// the close.
func (g *Game) freeResearch(i int) (string, []Report, error) {
	r := g.Empires[i].Research
	if r.Topic == NoTech {
		return "", nil, errors.New("no topic under study")
	}
	r.Progress[r.Topic] = r.Cost(r.Topic)
	return r.Topic.String() + " due tomorrow", nil, nil
}

// industryBoost is the Nimbuloids' SA_IndustryBoost ("Double industry on
// every planet for one day"): every completing project gains a day's
// industry now, capped at its Cost.
func (g *Game) industryBoost(i int) (string, []Report, error) {
	n := 0
	for _, c := range g.Empires[i].Colonies {
		if pj := c.Project; pj != nil && !pj.Item.Ongoing() {
			ind, _, _ := c.Output()
			pj.Progress = min(pj.Progress+ind, pj.Cost())
			n++
		}
	}
	if n == 0 {
		return "", nil, errors.New("no project running")
	}
	return "Projects gain a day's industry", nil, nil
}

// moreMaxPop is the Mebes' SA_MoreMaxPop ("Increase the maximum
// population of all colonies"): every colony's planet PopBonus
// +MebesPopBonus.
func (g *Game) moreMaxPop(i int) (string, []Report, error) {
	cols := g.Empires[i].Colonies
	if len(cols) == 0 {
		return "", nil, errors.New("no colony")
	}
	for _, c := range cols {
		c.Planet.PopBonus += MebesPopBonus
	}
	return fmt.Sprintf("Colony capacity +%d", MebesPopBonus), nil, nil
}

// shieldPlanets is the Capelons' SA_ShieldPlanets ("Make all colonies
// invincible for one day"): Invincible = Day (IsInvincible).
func (g *Game) shieldPlanets(i int) (string, []Report, error) {
	g.Empires[i].Invincible = g.Day
	return "Colonies invincible today", nil, nil
}

// IsInvincible reports e's colonies invincible on day: the use's day D and
// D+1, as the close increments Day before the invasions.
func (e *Empire) IsInvincible(day int) bool {
	return e.Invincible >= 0 && (day == e.Invincible || day == e.Invincible+1)
}

// bumpShipsInLanes is the Ungooma's SA_BumpShipsInLanes ("Bump all ships
// backward down star lanes"): every ship in a lane back at its entry
// (FreeSpot, one by one); a lane order (GoToStar, GoToSystem) kept, so
// the next close sends it down the lane again (a route recomputed), any
// other dropped; a Report per other empire hit.
func (g *Game) bumpShipsInLanes(i int) (string, []Report, error) {
	var reps []Report
	bumped := false
	for _, o := range g.Empires {
		hit := false
		for _, s := range o.Ships {
			if s.Lane < 0 {
				continue
			}
			s.Lane, s.Arrive, s.Orbit = -1, 0, -1
			if k := s.Order.Kind; k != GoToStar && k != GoToSystem {
				s.Order = Order{}
			}
			s.Pos = FreeSpot(s.Pos, spaceAt(g.Empires, s.Star, s))
			hit = true
		}
		bumped = bumped || hit
		if hit && o.Index != i {
			reps = append(reps, Report{o.Index, dipEvent(AbilityUsed, i, "Ships bumped by "+g.Empires[i].Species.String())})
		}
	}
	if !bumped {
		return "", nil, errors.New("no ship in a lane")
	}
	return "Lane ships bumped", reps, nil
}

// repelAlienShips is the Frutmaka's SA_RepelAlienShips ("Warp alien ships
// out of your colonies' stars"): every other empire's ship out of the
// lanes (and not refitting) at i's colony stars, all taken first, goes to
// the far end of the star's lowest-index lane (FreeSpot); a lane order
// (GoToStar, GoToSystem) goes on as a GoToSystem to its star from there
// (a route recomputed at the close, the bump's rule), dropped when that
// is where it landed, any other dropped; a star with no lane keeps them
// (ASSUMPTION). A Report per empire hit.
func (g *Game) repelAlienShips(i int) (string, []Report, error) {
	me, c := g.Empires[i], g.Cluster
	var stars []int
	for _, col := range me.Colonies {
		if len(c.Stars[col.Star].Lanes) > 0 && !slices.Contains(stars, col.Star) {
			stars = append(stars, col.Star)
		}
	}
	type move struct {
		s    *Starship
		o    *Empire
		from int
	}
	var moves []move
	for _, st := range stars {
		for _, o := range g.Empires {
			if o == me {
				continue
			}
			for _, s := range o.Ships {
				if s.Star == st && s.Lane < 0 && o.Refitting(s) == nil {
					moves = append(moves, move{s, o, st})
				}
			}
		}
	}
	if len(moves) == 0 {
		return "", nil, errors.New("no alien ship at our colonies")
	}
	from := make([][]string, len(g.Empires))
	for _, m := range moves {
		lane := slices.Min(c.Stars[m.from].Lanes)
		far := c.Other(lane, m.from)
		_, end, _ := laneEnd(c, far, m.from)
		s := m.s
		s.Star, s.Orbit, s.Lane = far, -1, -1
		if k := s.Order.Kind; (k == GoToStar || k == GoToSystem) && s.Order.Star != far {
			s.Order = Order{Kind: GoToSystem, Star: s.Order.Star}
		} else {
			s.Order = Order{}
		}
		s.Pos = FreeSpot(c.Stars[far].System.Lanes[end].Pos, spaceAt(g.Empires, far, s))
		if n := c.Stars[m.from].Name; !slices.Contains(from[m.o.Index], n) {
			from[m.o.Index] = append(from[m.o.Index], n)
		}
	}
	var reps []Report
	for j, names := range from {
		if names != nil {
			reps = append(reps, Report{j, dipEvent(AbilityUsed, i,
				me.Species.String()+" warp us out of "+strings.Join(names, ", "))})
		}
	}
	return "Alien ships warped away", reps, nil
}

// blockStarLanes is the Arbryls' SA_BlockStarLanes ("Block all star
// lanes entering your colonised systems"): BlockedUntil = Day +
// BlockDays (LaneBlocker).
func (g *Game) blockStarLanes(i int) (string, []Report, error) {
	g.Empires[i].BlockedUntil = g.Day + BlockDays
	return fmt.Sprintf("Lanes blocked for %d days", BlockDays), nil, nil
}

// LaneBlocker is the first other empire closing cluster lane lane,
// entered from star, to e on day: by a Lane Blocker (LaneBlocks), or by
// SA_BlockStarLanes with a colony at the far end; nil when open. Only the
// entry is closed (Order refuses it, holdAtBlock waits).
func (e *Empire) LaneBlocker(star, lane, day int) *Empire {
	far := -1
	for _, o := range e.peerList() {
		if o == e {
			continue
		}
		if day < o.LaneBlocks[lane] {
			return o
		}
		if day >= o.BlockedUntil {
			continue
		}
		if far < 0 {
			far = e.Cluster.Other(lane, star)
		}
		for _, c := range o.Colonies {
			if c.Star == far {
				return o
			}
		}
	}
	return nil
}

// blockedOrder is Order's refusal of a GoToStar o into a lane closed to
// e today (LaneBlocker), nil otherwise; o passed s.Check.
func (e *Empire) blockedOrder(s *Starship, o Order) error {
	if o.Kind != GoToStar {
		return nil
	}
	lane, _, _ := laneEnd(e.Cluster, s.Star, o.Star)
	if b := e.LaneBlocker(s.Star, lane, e.Day); b != nil {
		return fmt.Errorf("the lane is blocked by the %s", b.Species)
	}
	return nil
}

// BlockedLane is blockedOrder on s's own order (the window's hop).
func (e *Empire) BlockedLane(s *Starship) error { return e.blockedOrder(s, s.Order) }

// holdAtBlock swaps the GoToStar order of s, while its lane is closed on
// day, for a GoToPoint to the entry, or NoOrder when s stands at its
// FreeSpot already (waiting spends nothing). held reports the swap: the
// caller moves s and puts o back.
func (e *Empire) holdAtBlock(s *Starship, day int, near func(star int) []Vec3) (o Order, held bool) {
	if s.Lane >= 0 || s.Order.Kind != GoToStar {
		return Order{}, false
	}
	lane, end, ok := laneEnd(e.Cluster, s.Star, s.Order.Star)
	if !ok || e.LaneBlocker(s.Star, lane, day) == nil {
		return Order{}, false
	}
	o = s.Order
	entry := e.Cluster.Stars[s.Star].System.Lanes[end].Pos
	var obs []Vec3
	if near != nil {
		obs = near(s.Star)
	}
	if s.Orbit < 0 && dist(s.Pos, FreeSpot(entry, obs)) < 1e-9 {
		s.Order = Order{}
	} else {
		s.Order = Order{Kind: GoToPoint, Pos: entry}
	}
	return o, true
}

// cornucopia is the Govorom's SA_Cornucopia ("Turn your least populated
// world into a Cornucopia"): i's least populated colony not yet rich
// (first on a tie) becomes Type Cornucopia, every black or white square
// but the Colony Base's a bonus one, drawn by cornucopiaKey;
// structures and ruins stay, tubes on black go (whiten).
func (g *Game) cornucopia(i int) (string, []Report, error) {
	cols := g.Empires[i].Colonies
	if len(cols) == 0 {
		return "", nil, errors.New("no colony")
	}
	var least *Colony
	for _, c := range cols {
		if !rich(c.Planet) && (least == nil || c.Pop < least.Pop) {
			least = c
		}
	}
	if least == nil {
		return "", nil, errors.New("every colony is a rich world already")
	}
	p := least.Planet
	p.Type = Cornucopia
	r := rand.New(rand.NewPCG(cornucopiaKey(g.Seed, i, g.Day, least), 0x6a09e667f3bcc908))
	// In a shuffled order, each bare square takes the bonus kind fewest so
	// far, ties drawn: as near a third each as the kept ones allow.
	bonus := [...]SquareKind{BonusIndustry, BonusProsperity, BonusResearch}
	var n [3]int
	var bare []Cell
	for _, at := range p.Cells() {
		switch sq := p.Surface[at]; {
		case sq.Kind >= BonusIndustry:
			n[sq.Kind-BonusIndustry]++
		case sq.Item != ColonyBase:
			bare = append(bare, at)
		}
	}
	r.Shuffle(len(bare), func(x, y int) { bare[x], bare[y] = bare[y], bare[x] })
	for _, at := range bare {
		if p.Surface[at].Kind == Black {
			p.whiten(at)
		}
		low := min(n[0], n[1], n[2])
		var ks []int
		for k := range n {
			if n[k] == low {
				ks = append(ks, k)
			}
		}
		k := ks[r.IntN(len(ks))]
		p.Surface[at].Kind = bonus[k]
		n[k]++
	}
	ev := dipEvent(AbilityUsed, -1, p.Name+" made rich")
	ev.Star, ev.Planet = least.Star, least.PlanetIdx // the news opens it
	return ev.Text, []Report{{i, ev}}, nil
}

// rich reports p a Cornucopia whose every square but the Colony Base's
// is a bonus one.
func rich(p *Planet) bool {
	return p.Type == Cornucopia && !slices.ContainsFunc(p.Cells(), func(at Cell) bool {
		sq := p.Surface[at]
		return sq.Item != ColonyBase && (sq.Kind == Black || sq.Kind == White)
	})
}

// cornucopiaKey is SA_Cornucopia's draw key, the dig's.
func cornucopiaKey(seed uint64, empire, day int, c *Colony) uint64 {
	return c.drawKey(DigKey(seed, empire, day))
}
