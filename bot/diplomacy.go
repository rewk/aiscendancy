package bot

import (
	"math"
	"slices"

	"github.com/rewk/aiscendancy/game"
)

// The bot's diplomacy (docs/rules.md "Diplomacy"): an attitude toward
// each empire met, on the original's 0..1000 scale, recomputed every Act
// from the game's ledger (game.Relation) weighed by the genome, then read
// against the species' thresholds.

// Personality is a species' thresholds on the attitude scale, typed from
// the game's race*.dip: the six in a fixed order (DeclareWar <
// AcceptPeace < ProposePeace < BreakAlliance < AcceptAlliance <
// ProposeAlliance), the attitude a contact starts at, and the patience
// with foreign armed ships in one's systems (the larger, the more
// tolerant).
type Personality struct {
	DeclareWar, AcceptPeace, ProposePeace, BreakAlliance, AcceptAlliance, ProposeAlliance int
	Start                                                                                 int
	ShipPatience                                                                          int
}

// Personalities holds every species' DIPVALUES, in Species order.
var Personalities = [game.NumSpecies]Personality{
	game.Minions:     {290, 360, 450, 510, 600, 750, 340, 95},
	game.Snovemdomas: {250, 350, 400, 450, 600, 760, 340, 35},
	game.Orfa:        {210, 310, 410, 470, 570, 750, 420, 60},
	game.Kambuchka:   {250, 330, 450, 470, 600, 800, 350, 95},
	game.Hanshaks:    {150, 200, 350, 470, 600, 750, 440, 50},
	game.Fludentri:   {300, 400, 500, 550, 700, 750, 380, 40},
	game.Baliflids:   {220, 240, 300, 440, 500, 680, 320, 72},
	game.Swaparamans: {270, 310, 400, 475, 575, 700, 390, 80},
	game.Frutmaka:    {300, 400, 500, 600, 700, 800, 310, 100},
	game.Shevar:      {450, 530, 670, 750, 800, 900, 455, 110},
	game.Govorom:     {200, 300, 400, 430, 600, 700, 350, 45},
	game.Ungooma:     {200, 300, 400, 450, 600, 750, 410, 140},
	game.Dubtaks:     {250, 300, 450, 580, 600, 700, 620, 145},
	game.Capelons:    {250, 350, 450, 570, 650, 790, 310, 80},
	game.Mebes:       {170, 290, 360, 450, 550, 690, 370, 110},
	game.Oculons:     {300, 360, 450, 480, 600, 750, 440, 80},
	game.Arbryls:     {210, 300, 370, 420, 620, 780, 400, 95},
	game.Marmosians:  {320, 450, 560, 570, 700, 850, 360, 90},
	game.Chronomyst:  {150, 250, 450, 540, 670, 690, 430, 40},
	game.Chamachies:  {210, 300, 450, 550, 590, 700, 320, 80},
	game.Nimbuloids:  {130, 310, 440, 490, 590, 760, 320, 90},
}

// AtmosphereShift is what the game's Atmosphere adds to the six
// thresholds per step from Neutral (Hostile +100 puts most species at war
// on contact). Not a gene: the fuzzer plays Neutral games only.
const AtmosphereShift = 100

// Thresholds is b's species' Personality, the genome's Temper (rounded)
// and the game's atmosphere added to the six thresholds.
func (b *Bot) Thresholds() Personality {
	p := Personalities[b.E.Species]
	t := int(math.Round(b.dipGenes().Temper))
	if b.G != nil {
		t += int(b.G.Atmosphere) * AtmosphereShift
	}
	p.DeclareWar += t
	p.AcceptPeace += t
	p.ProposePeace += t
	p.BreakAlliance += t
	p.AcceptAlliance += t
	p.ProposeAlliance += t
	return p
}

// Attitude is b's attitude toward empire j as of its last Act, false
// before any or for an empire never met.
func (b *Bot) Attitude(j int) (int, bool) {
	if j >= 0 && j < len(b.attOK) && b.attOK[j] {
		return b.att[j], true
	}
	// Not read today yet (att is not saved: a game just loaded): the
	// formula on the saved grudge, weary and goodwill, for an empire
	// diplomacy would read. A query: nothing is kept.
	n := len(b.E.Relations)
	if j < 0 || j >= n || j == b.E.Index || !b.E.Alive() || b.E.StanceWith(j) == game.Unknown ||
		len(b.grudge) != n || len(b.weary) != n || len(b.goodwill) != n {
		return 0, false
	}
	return b.attitude(j, b.knownSystems(n), len(b.ownStars())), true
}

// ownStars is the stars my colonies are at.
func (b *Bot) ownStars() map[int]bool {
	mine := map[int]bool{}
	for _, c := range b.E.Colonies {
		mine[c.Star] = true
	}
	return mine
}

// attitude is toward j from grudge, weary and goodwill as they stand:
// Start − grudge − envy + weary + goodwill in [0, 1000], envy EnvyW per
// system j holds (theirs) beyond mine.
func (b *Bot) attitude(j int, theirs []int, mine int) int {
	dg, pers := b.dipGenes(), Personalities[b.E.Species]
	envy := dg.EnvyW * float64(max(0, theirs[j]-mine))
	return int(min(1000, max(0, math.Round(float64(pers.Start)-b.grudge[j]-envy+b.weary[j]+b.goodwill[j]))))
}

// News drains the Reports of b's calls since the last News.
func (b *Bot) News() []game.Report {
	n := b.news
	b.news = nil
	return n
}

// dipGenes is the Genome's diplomacy genes, else NeutralDiplomacy.
func (b *Bot) dipGenes() Diplomacy {
	if g := b.Cfg.Genome; g != nil {
		return g.Diplomacy()
	}
	return NeutralDiplomacy()
}

// freshDiplomacy starts the diplomacy over, one entry per empire: seen
// is the ledger as it stands (past facts are held against no one), all
// else zero, lastTrade and lastAsk −1.
func (b *Bot) freshDiplomacy() {
	n := len(b.E.Relations)
	b.grudge, b.weary, b.goodwill, b.seen, b.lastTrade, b.lastAsk, b.att, b.attOK = nil, nil, nil, nil, nil, nil, nil, nil
	if n == 0 {
		return
	}
	b.grudge, b.weary, b.goodwill, b.seen = make([]float64, n), make([]float64, n), make([]float64, n), slices.Clone(b.E.Relations)
	b.lastTrade, b.lastAsk = slices.Repeat([]int{-1}, n), slices.Repeat([]int{-1}, n)
	b.att, b.attOK = make([]int, n), make([]bool, n)
}

// diplomacy is Act's daily pass over every empire j met, in index order.
// grudge, weary and goodwill fade by Forgive and grow by the ledger's
// facts since seen, each weighed by its gene, into attitude (envy over
// the stars I Know). Then j's pending offers are answered (Decide) and,
// by the stance re-read: at War, peace offered at ProposePeace; at
// Peace, war under DeclareWar when warReady (a war the game bars,
// WarBarred, is not wanted), else an alliance at
// ProposeAlliance, else proposeTrade; allied, a break under
// BreakAlliance, else askWar, else proposeTrade. Without G, nothing.
func (b *Bot) diplomacy() {
	if b.G == nil {
		return
	}
	n := len(b.E.Relations)
	if len(b.seen) != n || len(b.grudge) != n || len(b.weary) != n || len(b.goodwill) != n || len(b.lastTrade) != n || len(b.lastAsk) != n || len(b.att) != n {
		b.freshDiplomacy()
	}
	if n == 0 {
		return
	}
	me, dg, pers, th := b.E.Index, b.dipGenes(), Personalities[b.E.Species], b.Thresholds()
	theirs := b.knownSystems(n)
	mine := b.ownStars()
	for j := range n {
		if j == me || b.E.StanceWith(j) == game.Unknown {
			continue
		}
		r, s := *b.E.Rel(j), b.seen[j]
		b.seen[j] = r
		b.grudge[j] = b.grudge[j]*(1-dg.Forgive) +
			float64(r.ShipsLost-s.ShipsLost)*dg.GrudgeShip +
			float64(r.PlanetsLost-s.PlanetsLost)*dg.GrudgePlanet +
			float64(r.WarsOn-s.WarsOn)*dg.GrudgeWar +
			float64(r.Refused-s.Refused)*dg.GrudgeRefusal +
			float64(r.Intrusions-s.Intrusions)*dg.IntrudeW*50/float64(pers.ShipPatience) +
			float64(r.Broken-s.Broken)*dg.GrudgeBreak +
			float64(r.Defamed-s.Defamed)*dg.DefameW +
			float64(r.Encroached-s.Encroached)*dg.GrudgeEncroach
		b.weary[j] = b.weary[j]*(1-dg.Forgive) + float64(r.WarDays-s.WarDays)*dg.WearyW
		b.goodwill[j] = b.goodwill[j]*(1-dg.Forgive) + float64(r.Trades-s.Trades)*dg.TradeW +
			float64(r.Helped-s.Helped)*dg.HelpW + float64(r.Hospitality-s.Hospitality)*dg.HospW
		att := b.attitude(j, theirs, len(mine))
		b.att[j], b.attOK[j] = att, true

		for _, o := range b.G.PendingOffers(me) {
			if o.From != j {
				continue
			}
			b.say(b.G.Answer(o, b.Decide(o)))
		}
		switch b.E.StanceWith(j) {
		case game.War:
			if att >= th.ProposePeace {
				b.say(b.G.OfferPeace(me, j))
			}
		case game.Peace:
			if att < th.DeclareWar && b.G.WarBarred(me, j) == 0 { // barred: as if no war wanted
				if b.warReady(j) {
					b.say(b.G.DeclareWar(me, j))
				}
				break // war wanted: no alliance, no exchange
			}
			if att >= th.ProposeAlliance {
				b.say(b.G.OfferAlliance(me, j))
				break
			}
			b.proposeTrade(j, dg)
		case game.Alliance:
			if att < th.BreakAlliance {
				b.say(b.G.BreakAlliance(me, j))
				break
			}
			if b.askWar(j, dg) {
				break // one pending offer per pair: the request first
			}
			b.proposeTrade(j, dg)
		}
	}
}

// worth is an exchange's value to the side learning gn: a star or a
// lane 1 each, a tech 1.
func worth(gn game.Gain) int {
	w := gn.Stars + gn.Lanes
	if gn.Tech != game.NoTech {
		w++
	}
	return w
}

// Decide is b's answer to the offer o, from its last attitude toward
// o.From (none yet: Start) against Thresholds: a peace at AcceptPeace or
// more, an alliance at AcceptAlliance or more, an exchange when
// tradeFair. An ally's request is judged on the attitude toward o.Third,
// against the act's proposing threshold (mine toward Third) bent by
// FavourW the ally's way; a war the game bars (WarBarred) refused. Pure,
// so the window answers the player at once as Act would. A
// defeated empire refuses.
func (b *Bot) Decide(o game.Offer) bool {
	if !b.E.Alive() {
		return false
	}
	about := o.From
	if o.Kind.Request() {
		about = o.Third
	}
	att, ok := b.Attitude(about)
	if !ok {
		att = Personalities[b.E.Species].Start
	}
	th, fav := b.Thresholds(), int(math.Round(b.dipGenes().FavourW))
	switch {
	case o.Kind == game.PeaceOffer:
		return att >= th.AcceptPeace
	case o.Kind == game.AllianceOffer:
		return att >= th.AcceptAlliance
	case o.Kind.Trade():
		return b.G != nil && b.tradeFair(o.From, o.Kind, b.dipGenes())
	case o.Kind == game.AskWarOn:
		return att < th.DeclareWar+fav && b.G != nil && b.G.WarBarred(b.E.Index, o.Third) == 0 && b.warReady(o.Third)
	case o.Kind == game.AskPeaceWith:
		return att >= th.ProposePeace-fav
	case o.Kind == game.AskBreakWith:
		return att < th.BreakAlliance+fav
	case o.Kind == game.AskAllianceWith:
		return att >= th.ProposeAlliance-fav
	}
	return false
}

// warReady reports my arms enough for a war on t: my armed ships, one at
// least, number at least WarCaution × t's armed ships seen near my
// systems (armedSeen: a war is fought at the border);
// or, in a rage (my attitude toward t under DeclareWar − Rage), any armed
// ship will do.
func (b *Bot) warReady(t int) bool {
	armed := b.armedShips()
	if armed == 0 {
		return false
	}
	if att, ok := b.Attitude(t); ok && att < b.Thresholds().DeclareWar-int(math.Round(b.dipGenes().Rage)) {
		return true
	}
	seen := b.armedSeen()
	return t >= 0 && t < len(seen) && float64(armed) >= b.dipGenes().WarCaution*float64(seen[t])
}

// warTarget is the armed ships I want for the wars I wage or want (else
// the bots never arm for the wars their attitude asks for): over every
// living empire met at War with me, at Peace under my DeclareWar and not
// WarBarred, or at Peace under my ProposePeace (wary) with an armed ship
// seen near me, ⌈WarCaution × its armed ships seen near me⌉ + 1, at
// least 2; 0 with none.
func (b *Bot) warTarget() int {
	if b.G == nil {
		return 0
	}
	seen, wc, th := b.armedSeen(), b.dipGenes().WarCaution, b.Thresholds()
	want := 0
	for j, e := range b.G.Empires {
		if j == b.E.Index || j >= len(seen) || !e.Alive() {
			continue
		}
		switch b.E.StanceWith(j) {
		case game.War:
		case game.Peace:
			att, ok := b.Attitude(j)
			wanted := att < th.DeclareWar && b.G.WarBarred(b.E.Index, j) == 0 // barred: wary at most
			if !ok || att >= th.ProposePeace || !wanted && seen[j] == 0 {
				continue
			}
		default:
			continue
		}
		want = max(want, int(math.Ceil(wc*float64(seen[j])))+1, 2)
	}
	return want
}

// askWar asks my ally j, every round(AskZeal) days, for war on my first
// living enemy j is at Peace with: true when asked, lastAsk[j] set
// whatever the game answers. The other requests are the player's only.
func (b *Bot) askWar(j int, dg Diplomacy) bool {
	if last := b.lastAsk[j]; last >= 0 && b.G.Day-last < int(math.Round(dg.AskZeal)) {
		return false
	}
	me := b.E.Index
	for t := range b.G.Empires {
		if t == me || t == j || !b.G.Empires[t].Alive() || b.E.StanceWith(t) != game.War || b.G.Empires[j].StanceWith(t) != game.Peace {
			continue
		}
		b.say(b.G.Request(me, j, game.AskWarOn, t))
		b.lastAsk[j] = b.G.Day
		return true
	}
	return false
}

// tradeFair reports j's exchange of kind k worth accepting: what I learn
// is worth at least TradeFair × what j learns (Gain as it stands).
func (b *Bot) tradeFair(j int, k game.OfferKind, dg Diplomacy) bool {
	me := b.E.Index
	return float64(worth(b.G.Gain(me, j, k))) >= dg.TradeFair*float64(worth(b.G.Gain(j, me, k)))
}

// proposeTrade offers j, every round(TradeZeal) days, the exchange kind
// worth the most to me (ties the first; none worth 0), lastTrade[j] set
// whatever the game answers: a refusal is not retried daily.
func (b *Bot) proposeTrade(j int, dg Diplomacy) {
	if last := b.lastTrade[j]; last >= 0 && b.G.Day-last < int(math.Round(dg.TradeZeal)) {
		return
	}
	me, best, bestW := b.E.Index, game.TradeStars, 0
	for _, k := range []game.OfferKind{game.TradeStars, game.TradeLanes, game.TradeResearch} {
		if w := worth(b.G.Gain(me, j, k)); w > bestW {
			best, bestW = k, w
		}
	}
	if bestW == 0 {
		return
	}
	b.say(b.G.OfferTrade(me, j, best))
	b.lastTrade[j] = b.G.Day
}

// knownSystems is, per empire, the stars I Know holding at least one of
// its colonies (Planet.Owner).
func (b *Bot) knownSystems(n int) []int {
	out := make([]int, n)
	for st, star := range b.E.Cluster.Stars {
		if !b.E.Knows(st) {
			continue
		}
		var at []int
		for _, p := range star.System.Planets {
			if p.Owner >= 0 && p.Owner < n && !slices.Contains(at, p.Owner) {
				at = append(at, p.Owner)
				out[p.Owner]++
			}
		}
	}
	return out
}

// armedSeen is, per empire, its ships Game.ShipsSeenAll shows me near my
// systems (at a star of mine or one lane from it) but those a scan of
// their parts shows unarmed (unscanned, a ship may be armed), counted
// once a day (seenDay).
func (b *Bot) armedSeen() []int {
	n := len(b.E.Relations)
	if b.seenArmed != nil && len(b.seenArmed) == n && b.seenDay == b.G.Day {
		return b.seenArmed
	}
	c := b.E.Cluster
	near := make([]bool, len(c.Stars))
	for _, col := range b.E.Colonies {
		near[col.Star] = true
		for _, li := range c.Stars[col.Star].Lanes {
			near[c.Other(li, col.Star)] = true
		}
	}
	out := make([]int, n)
	for st := range c.Stars {
		if !near[st] {
			continue
		}
		for _, sg := range b.G.ShipsSeenAll(b.E, st) {
			if sg.Empire < 0 || sg.Empire >= n || sg.Scan != nil && sg.Scan.Parts && sg.Scan.Design.Weapons() == 0 {
				continue
			}
			out[sg.Empire]++
		}
	}
	b.seenArmed, b.seenDay = out, b.G.Day
	return out
}

// armedShips is my ships carrying a weapon part, anywhere.
func (b *Bot) armedShips() int {
	k := 0
	for _, s := range b.E.Ships {
		if s.Design.Weapons() > 0 {
			k++
		}
	}
	return k
}
