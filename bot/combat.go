package bot

import (
	"math"

	"github.com/rewk/aiscendancy/game"
)

// The bot's combat policy (docs/rules.md "The bot's combat policy"). It
// reads foreign ships through Game.HostilesAll only (the cloakers do not
// blind it): an unscanned ship is judged by its hull size, a scanned one by its Scan. assess redoes every
// estimate each Act: no memory, nothing saved.

// side is one side's strength at a star: damage per day, hull points,
// count.
type side struct {
	dmg, hp float64
	n       int
}

func (a side) plus(b side) side { return side{a.dmg + b.dmg, a.hp + b.hp, a.n + b.n} }

// pressure is (their damage / my hull) / (my damage / their hull): how
// much faster they kill my side than I kill theirs. 0 when they deal no
// damage; +Inf when they do and I deal none or have no hull.
func pressure(their, mine side) float64 {
	switch {
	case their.dmg <= 0:
		return 0
	case mine.dmg <= 0 || mine.hp <= 0:
		return math.Inf(1)
	}
	return (their.dmg / mine.hp) / (mine.dmg / their.hp)
}

// sightingSide is a hostile ship's estimate: damage from its design when
// its parts are scanned, else HullThreat of its hull; hull points from
// its Scan, else its hull's full strength for its owner's species less
// what my own shots took off it (sg.Dealt), at least 1.
func sightingSide(g *game.Game, cg Combat, sg game.Sighting) side {
	sc := sg.Scan
	if sc != nil && sc.Parts {
		return side{float64(game.DesignDamage(sc.Design, 0)), float64(sc.HullPts), 1}
	}
	hp := (&game.Starship{Design: game.Design{Hull: sg.Hull}, Species: g.Empires[sg.Empire].Species}).MaxHull()
	if sc != nil {
		hp = sc.HullPts
	} else {
		hp = max(1, hp-sg.Dealt)
	}
	return side{cg.HullThreat[sg.Hull], float64(hp), 1}
}

// shipSide is one of my ships: its design's damage, its hull points left.
func shipSide(s *game.Starship) side {
	return side{float64(game.DesignDamage(s.Design, 0)), float64(s.Hull), 1}
}

// armed reports a warship: a design that deals damage and carries no
// Colonizer (a colony ship with a gun stays a colony ship).
func armed(d game.Design) bool { return d.Colonizers() == 0 && game.DesignDamage(d, 0) > 0 }

// genes is the Genome's combat genes, else NeutralCombat.
func (b *Bot) genes() Combat {
	if g := b.Cfg.Genome; g != nil {
		return g.Combat()
	}
	return NeutralCombat()
}

// assess sets cg, threat (the hostiles' side at every known star: ships
// seen plus planetSide) and contact (a known star holds a hostile or a
// planet of another empire). Without G: no threat, no contact.
func (b *Bot) assess() {
	b.cg, b.threat, b.contact = b.genes(), nil, false
	if b.G == nil {
		return
	}
	c := b.E.Cluster
	b.threat = make([]side, len(c.Stars))
	for st := range c.Stars {
		if !b.E.Knows(st) {
			continue
		}
		for _, sg := range b.G.HostilesAll(b.E, st) {
			b.threat[st] = b.threat[st].plus(sightingSide(b.G, b.cg, sg))
		}
		b.threat[st] = b.threat[st].plus(b.planetSide(st))
		if b.threat[st].n > 0 {
			b.contact = true
		}
		for _, p := range c.Stars[st].System.Planets {
			if p.Owner >= 0 && p.Owner != b.E.Index {
				b.contact = true
			}
		}
	}
}

// planetSide is the orbital defences of the hostile colonies at star, one
// per colony holding one: Σ Damage × Shots, Σ Strength at full (a
// foreign planet's damage taken is hidden).
func (b *Bot) planetSide(star int) side {
	var t side
	for _, p := range b.E.Cluster.Stars[star].System.Planets {
		if p.Owner < 0 || p.Owner == b.E.Index || !b.G.Hostile(b.E, b.G.Empires[p.Owner]) {
			continue
		}
		var c side
		for _, it := range p.Orbit {
			if d, ok := it.Defence(); ok {
				c.dmg += float64(d.Damage * d.Shots)
				c.hp += float64(d.Strength)
				c.n = 1
			}
		}
		t = t.plus(c)
	}
	return t
}

// their is the hostiles' side at star (assess), zero when none is seen.
func (b *Bot) their(star int) side {
	if star < 0 || star >= len(b.threat) {
		return side{}
	}
	return b.threat[star]
}

// mine is my side at star: my ships there out of the lanes.
func (b *Bot) mine(star int) side {
	var m side
	for _, s := range b.E.Ships {
		if s.Star == star && s.Lane < 0 {
			m = m.plus(shipSide(s))
		}
	}
	return m
}

// danger is the test "s would flee at star v": armed, the pressure on my
// ships at v with s counted once exceeds FleeRatio; unarmed, the
// hostiles at v deal damage. The routes pass it as search's blocked: a
// ship is never sent into, nor through, a star it would flee.
func (b *Bot) danger(s *game.Starship) func(v int) bool {
	if !armed(s.Design) {
		return b.unarmedDanger
	}
	me := shipSide(s)
	return func(v int) bool {
		t := b.their(v)
		if t.dmg <= 0 {
			return false
		}
		m := me
		for _, o := range b.E.Ships {
			if o != s && o.Star == v && o.Lane < 0 {
				m = m.plus(shipSide(o))
			}
		}
		return pressure(t, m) > b.cg.FleeRatio
	}
}

// colonyAt reports one of my colonies at star.
func (b *Bot) colonyAt(star int) bool {
	for _, col := range b.E.Colonies {
		if col.Star == star {
			return true
		}
	}
	return false
}

// flee sends s toRefuge from a star where the pressure on my side exceeds
// FleeRatio, reporting it gone; a landing ship stays.
func (b *Bot) flee(s *game.Starship) bool {
	if s.Lane >= 0 || landing(s) {
		return false
	}
	t := b.their(s.Star)
	if t.dmg <= 0 || pressure(t, b.mine(s.Star)) <= b.cg.FleeRatio {
		return false
	}
	return b.toRefuge(s)
}

// landing reports s in orbit with a Colonise or Invade order: it founds
// (invades) today, so it never flees.
func landing(s *game.Starship) bool {
	return (s.Order.Kind == game.Colonise || s.Order.Kind == game.Invade) && s.Orbit >= 0
}

// toRefuge orders s GoToStar its refuge, its dest dropped; false when it
// has none or the order is refused.
func (b *Bot) toRefuge(s *game.Starship) bool {
	v := b.refuge(s)
	if v < 0 || b.E.Order(s, game.Order{Kind: game.GoToStar, Star: v}) != nil {
		return false
	}
	delete(b.dest, s)
	return true
}

// refuge is where s flees by lane: the adjacent star not in danger for it
// (any when all are), of least hostile damage, ties one with a colony of
// mine, then fewer lane days, then lane order; -1 without a lane drive.
func (b *Bot) refuge(s *game.Starship) int {
	strength := s.Design.LaneStrength()
	if strength == 0 {
		return -1
	}
	c := b.E.Cluster
	danger := b.danger(s)
	best, bestDanger, bestDmg, bestCol, bestDays := -1, false, 0.0, false, 0
	for _, li := range c.Stars[s.Star].Lanes {
		v := c.Other(li, s.Star)
		dg, d, col, days := danger(v), b.their(v).dmg, b.colonyAt(v), game.LaneDays(c, li, strength, s.Species.Traits())
		if best < 0 || !dg && bestDanger || dg == bestDanger && (d < bestDmg || d == bestDmg && (col && !bestCol || col == bestCol && days < bestDays)) {
			best, bestDanger, bestDmg, bestCol, bestDays = v, dg, d, col, days
		}
	}
	return best
}

// attack moves an idle warship: it stays where hostiles are seen at a
// pressure up to FleeRatio (the rounds do the rest); else, with a lane
// drive, it heads for the nearest star whose hostiles, this ship added to
// my side there, press below AttackRatio, else for the nearest star with
// a colony of mine (staying at one). Both searches avoid danger; the
// target goes in dest.
func (b *Bot) attack(s *game.Starship) {
	if s.Order.Kind != game.NoOrder || s.Lane >= 0 || !armed(s.Design) {
		return
	}
	if here := b.their(s.Star); here.n > 0 && pressure(here, b.mine(s.Star)) <= b.cg.FleeRatio {
		return
	}
	strength := s.Design.LaneStrength()
	if strength == 0 {
		return
	}
	me, tr := shipSide(s), s.Species.Traits()
	danger := b.danger(s)
	to, first := b.search(s.Star, strength, tr, 0, danger, func(v, _ int) bool {
		t := b.their(v)
		return t.n > 0 && pressure(t, b.mine(v).plus(me)) < b.cg.AttackRatio
	})
	if to < 0 {
		if b.colonyAt(s.Star) {
			return
		}
		to, first = b.search(s.Star, strength, tr, 0, danger, func(v, _ int) bool { return b.colonyAt(v) })
	}
	if to >= 0 && b.E.Order(s, game.Order{Kind: game.GoToStar, Star: first}) == nil {
		b.dest[s] = to
	}
}

// search is the bots' Dijkstra over the lanes from star from: the cost
// into v is LaneDays plus avoid × the hostiles' damage at v; stars settle
// by cost, then fewer hops, then lower index. Only from and known stars
// are expanded (an unknown star is a leaf). A star blocked accepts (nil:
// none; asked once per star, never from) is never reached: no path
// crosses it. stop sees each settled star but from with the adjacent star
// its path starts with; the first it accepts is (to, first), else (-1,
// -1).
func (b *Bot) search(from int, strength float64, tr game.Traits, avoid float64, blocked func(v int) bool, stop func(u, first int) bool) (to, first int) {
	return b.searchDays(from, strength, tr, avoid, blocked, func(u, first int, _ float64) bool { return stop(u, first) })
}

// searchDays is search whose stop is also told u's settled cost.
func (b *Bot) searchDays(from int, strength float64, tr game.Traits, avoid float64, blocked func(v int) bool, stop func(u, first int, days float64) bool) (to, first int) {
	c := b.E.Cluster
	n := len(c.Stars)
	days, hops, hop := make([]float64, n), make([]int, n), make([]int, n)
	reached, done := make([]bool, n), make([]bool, n)
	reached[from] = true
	var asked, block []bool // blocked's answers, asked once per star
	if blocked != nil {
		asked, block = make([]bool, n), make([]bool, n)
	}
	for {
		u := -1
		for v := range n {
			if !done[v] && reached[v] && (u < 0 || days[v] < days[u] || days[v] == days[u] && hops[v] < hops[u]) {
				u = v // ascending v: equal (days, hops) keep the lower index
			}
		}
		if u < 0 {
			return -1, -1
		}
		done[u] = true
		if u != from {
			if stop(u, hop[u], days[u]) {
				return u, hop[u]
			}
			if !b.E.Knows(u) {
				continue
			}
		}
		for _, li := range c.Stars[u].Lanes {
			v := c.Other(li, u)
			d, h := days[u]+float64(game.LaneDays(c, li, strength, tr))+avoid*b.their(v).dmg, hops[u]+1
			if done[v] || reached[v] && (days[v] < d || days[v] == d && hops[v] <= h) {
				continue
			}
			if blocked != nil {
				if !asked[v] {
					asked[v], block[v] = true, blocked(v)
				}
				if block[v] {
					continue
				}
			}
			days[v], hops[v], reached[v] = d, h, true
			if hop[v] = hop[u]; u == from {
				hop[v] = v
			}
		}
	}
}

// WarshipDays is the build time past which a warship steps down a hull
// (an Enormous took one colony 100 days and more).
const WarshipDays = 60

// warship is col's warship: game.WarshipWith col's specials (fitted: the most
// of them that still leave a weapon) on the largest known hull col
// builds within WarshipDays (Colony.DaysFor), stepping down to Medium,
// kept whatever its days; ok false when none is armed.
func (b *Bot) warship(col *game.Colony) (game.Design, bool) {
	r := b.E.Research
	lane := r.AllowsGizmo(game.StarLaneDrive)
	extra := b.fitted(col)
	var last game.Design
	found := false
	for h := game.NumHulls - 1; h >= game.Medium; h-- {
		if !r.Known[h.Tech()] {
			continue
		}
		var d game.Design
		ok := false
		for k := len(extra); k >= 0 && !ok; k-- {
			d, ok = game.WarshipWith(r, h, lane, extra[:k])
			ok = ok && armed(d)
		}
		if !ok {
			continue
		}
		last, found = d, true
		if days, ok := col.DaysFor(d.Cost()); ok && days <= WarshipDays {
			return d, true
		}
	}
	return last, found
}

// carried reports a warship of mine may carry special g: every Special
// but the Colonizer, the Invasion Module, the lane drives (the design's
// own), the Lane Destabilizer (no rule uses it) and the Space Habitat
// (the Habitat ships', bot/habitat.go).
func carried(g game.Gizmo) bool {
	switch g {
	case game.Colonizer, game.InvasionModule, game.StarLaneDrive, game.StarLaneHyperdrive, game.LaneDestabilizer, game.SpaceHabitat:
		return false
	}
	return game.Gizmos[g].Cat == game.Special
}

// fitted is the one or two specials col's next warship carries, drawn
// among those research allows and carried: a hash of the game's seed,
// my index, the day and col's place, so a load draws what the run would
// have and no PRNG state is kept (random-looking,
// reproducible). The second is the first again when the draw repeats.
func (b *Bot) fitted(col *game.Colony) []game.Gizmo {
	var pool []game.Gizmo
	for g := range game.NumGizmos {
		if carried(g) && b.E.Research.AllowsGizmo(g) {
			pool = append(pool, g)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	var seed uint64
	if b.G != nil {
		seed = b.G.Seed
	}
	h := splitmix(splitmix(splitmix(splitmix(seed)^uint64(b.E.Index))^uint64(b.E.Day)) ^ uint64(col.Star)<<8 ^ uint64(col.PlanetIdx))
	out := []game.Gizmo{pool[(h>>1)%uint64(len(pool))]}
	if h&1 == 1 {
		out = append(out, pool[(h>>33)%uint64(len(pool))])
	}
	return out
}

// splitmix is SplitMix64's finaliser.
func splitmix(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ x>>31
}

// wantWarship reports col should start a warship, only with contact:
// when the armed ships flying and being built number fewer than
// ⌈WarshipShare × colonies⌉ or than the wars want (warTarget), or when
// the hostile damage at col's star and its neighbours exceeds DefendW ×
// mine at col's star.
func (b *Bot) wantWarship(col *game.Colony) bool {
	if !b.contact {
		return false
	}
	n := 0
	for _, s := range b.E.Ships {
		if armed(s.Design) {
			n++
		}
	}
	for _, o := range b.E.Colonies {
		if d := building(o); d != nil && armed(*d) {
			n++
		}
	}
	if float64(n) < math.Ceil(b.cg.WarshipShare*float64(len(b.E.Colonies))) || n < b.warTarget() {
		return true
	}
	c := b.E.Cluster
	threat := b.their(col.Star).dmg
	for _, li := range c.Stars[col.Star].Lanes {
		threat += b.their(c.Other(li, col.Star)).dmg
	}
	return threat > b.cg.DefendW*b.mine(col.Star).dmg
}
