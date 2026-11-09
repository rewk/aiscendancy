package game

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/rewk/aiscendancy/journal"
)

// Diplomacy (docs/rules.md "Diplomacy"): a stance per pair, kept equal
// both ways; contact by meeting at a star or telepathy; war declared,
// peace and alliances offered and answered; knowledge exchanges; Alien
// Hospitality; third-party requests between allies; the Baliflids' and
// Marmosians' abilities; a ledger of facts per ordered pair for the bots.
// Only War is Hostile.

// Stance is an empire's standing with another.
type Stance uint8

const (
	Unknown  Stance = iota // never met
	Peace                  // met, not fighting
	War                    // Hostile both ways
	Alliance               // at peace, and neither colonises the other's systems (Empire.AllySystem)
)

func (s Stance) String() string {
	return [...]string{"unknown", "peace", "war", "alliance"}[s]
}

// Relation is what an empire holds about another (Empire.Relations[j]):
// the stance, the contact day and a LEDGER of counts, never judgements.
type Relation struct {
	Stance      Stance
	Met         int // day of contact, -1 never
	WarDays     int // days spent at war with j
	WarsOn      int // wars j declared on me
	ShipsLost   int // my ships j destroyed (its ships or its planets)
	PlanetsLost int // my colonies j invaded
	Intrusions  int // armed-ship-days of j at stars where I hold a colony and j none, at Peace
	Refused     int // my offers j refused or let expire
	LastOffer   int // day I last offered j anything, -1 never
	AllyDays    int // days spent allied with j
	Broken      int // alliances j broke with me
	Trades      int // knowledge exchanges concluded with j
	Hospitality int // Alien Hospitality points j poured toward me
	Helped      int // my requests j granted
	Defamed     int // times j was defamed to me (causeWar)
	Encroached  int // colonies j founded at stars where I held one, met (Empire.encroach)
	// Friendship is what j gave me, fading as a bot's goodwill does:
	// FriendTrade per exchange, FriendHelp per request of mine j granted,
	// FriendHosp per Alien Hospitality point; each close keeps 1 −
	// FriendFade of it. War on j is barred while it holds FriendBar or
	// more (WarBarred).
	Friendship float64 `json:",omitempty"`
}

// newRelations is n Relations, Unknown, Met and LastOffer −1.
func newRelations(n int) []Relation {
	rs := make([]Relation, n)
	for i := range rs {
		rs[i].Met, rs[i].LastOffer = -1, -1
	}
	return rs
}

// OfferKind is what an Offer proposes.
type OfferKind uint8

const (
	PeaceOffer    OfferKind = iota // at War
	AllianceOffer                  // at Peace
	TradeStars                     // the stars each Knows (Gain), at Peace or Alliance
	TradeLanes                     // the lanes each KnowsLane
	TradeResearch                  // one tech each, the cheapest
	// The third-party requests between allies: From asks To to act toward
	// Offer.Third.
	AskPeaceWith    // To offers peace to Third (To at War with it)
	AskWarOn        // To declares war on Third (To at Peace with it)
	AskBreakWith    // To breaks its alliance with Third
	AskAllianceWith // To offers an alliance to Third (To at Peace with it)
)

func (k OfferKind) String() string {
	return [...]string{"peace", "alliance", "stars", "lanes", "research",
		"peace request", "war request", "break request", "alliance request"}[k]
}

// Trade reports a knowledge exchange (OfferTrade).
func (k OfferKind) Trade() bool { return k >= TradeStars && k <= TradeResearch }

// Request reports a third-party request (Request).
func (k OfferKind) Request() bool { return k >= AskPeaceWith && k <= AskAllianceWith }

// Gain is what an empire would learn from another in an exchange.
type Gain struct {
	Stars, Lanes int
	Tech         Tech // NoTech for none
}

// Zero reports nothing to learn.
func (gn Gain) Zero() bool { return gn.Stars == 0 && gn.Lanes == 0 && gn.Tech == NoTech }

// Gain is what to would learn from from in an exchange of kind k: the
// stars or lanes from knows and to does not, or the cheapest such tech
// for to, ties by index (one tech each side).
func (g *Game) Gain(to, from int, k OfferKind) Gain {
	gn, _, _ := g.gain(to, from, k)
	return gn
}

// gain is Gain with the stars and lanes it counts, in index order.
func (g *Game) gain(to, from int, k OfferKind) (gn Gain, stars, lanes []int) {
	et, ef := g.Empires[to], g.Empires[from]
	gn.Tech = NoTech
	switch k {
	case TradeStars:
		for st := range g.Cluster.Stars {
			if ef.Knows(st) && !et.Knows(st) {
				stars = append(stars, st)
			}
		}
	case TradeLanes:
		for li := range g.Cluster.Lanes {
			if ef.KnowsLane(li) && !et.KnowsLane(li) {
				lanes = append(lanes, li)
			}
		}
	case TradeResearch:
		for t := Tech(0); t < NumTechs; t++ {
			if ef.Research.Known[t] && !et.Research.Known[t] &&
				(gn.Tech == NoTech || et.Research.Cost(t) < et.Research.Cost(gn.Tech)) {
				gn.Tech = t
			}
		}
	}
	gn.Stars, gn.Lanes = len(stars), len(lanes)
	return gn, stars, lanes
}

// learn applies Gain(to, from, k) to to (Visit, LearnLane, Grant),
// returning it.
func (g *Game) learn(to, from int, k OfferKind) Gain {
	gn, stars, lanes := g.gain(to, from, k)
	et := g.Empires[to]
	for _, st := range stars {
		et.Visit(st)
	}
	for _, li := range lanes {
		et.LearnLane(li)
	}
	if gn.Tech != NoTech {
		et.Research.Grant(gn.Tech)
	}
	return gn
}

// Offer is a pending proposal (Game.Offers) from From to To, made on Day;
// Third is the empire a request is about, else −1.
type Offer struct {
	From, To int
	Kind     OfferKind
	Day      int
	Third    int
}

// OfferCooldown is the days before From may offer To again.
const OfferCooldown = 5

// OfferExpiry: an offer unanswered this many days is refused by EndDay.
const OfferExpiry = 10

// StanceWith is e's stance with empire j, Unknown out of range.
func (e *Empire) StanceWith(j int) Stance {
	if r := e.Rel(j); r != nil {
		return r.Stance
	}
	return Unknown
}

// Rel is e's Relation with empire j, nil when j is out of range.
func (e *Empire) Rel(j int) *Relation {
	if j < 0 || j >= len(e.Relations) {
		return nil
	}
	return &e.Relations[j]
}

// AllyStances is empire j's wars and alliances with the living empires
// other than i, in empire order, told to i while i and j are allied and
// j lives; ok false otherwise.
func (g *Game) AllyStances(i, j int) (wars, allies []int, ok bool) {
	if i == j || g.Empires[i].StanceWith(j) != Alliance || !g.Empires[j].Alive() {
		return nil, nil, false
	}
	for k, o := range g.Empires {
		if k == i || k == j || !o.Alive() {
			continue
		}
		switch g.Empires[j].StanceWith(k) {
		case War:
			wars = append(wars, k)
		case Alliance:
			allies = append(allies, k)
		}
	}
	return wars, allies, true
}

// PendingOffers is a copy of the Offers made to empire to, in order.
func (g *Game) PendingOffers(to int) []Offer {
	var out []Offer
	for _, o := range g.Offers {
		if o.To == to {
			out = append(out, o)
		}
	}
	return out
}

// dipEvent is a diplomatic Event about other: no star, no planet.
func dipEvent(k EventKind, other int, text string) Event {
	return Event{Kind: k, Item: NoItem, Tech: NoTech, Star: -1, Planet: -1, Other: other, Text: text}
}

// setStance sets the stance a↔b both ways.
func (g *Game) setStance(a, b int, s Stance) {
	g.Empires[a].Relations[b].Stance = s
	g.Empires[b].Relations[a].Stance = s
}

// Contact makes a and b meet when Unknown: Peace, Met today, a Contacted
// Report each.
func (g *Game) Contact(a, b int) []Report {
	if a == b || g.Empires[a].StanceWith(b) != Unknown {
		return nil
	}
	g.setStance(a, b, Peace)
	g.Empires[a].Relations[b].Met = g.Day
	g.Empires[b].Relations[a].Met = g.Day
	ea, eb := g.Empires[a], g.Empires[b]
	return []Report{
		{a, dipEvent(Contacted, b, "Met the "+eb.Species.String())},
		{b, dipEvent(Contacted, a, "Met the "+ea.Species.String())},
	}
}

// DeclareWar puts a and b at War, refused when either is defeated (as
// every diplomatic act) or unless at Peace: an ally breaks first (the
// original's BREAK_ALLIANCE). b counts WarsOn; the Offers between them
// are dropped; WarDeclared for both.
func (g *Game) DeclareWar(a, b int) ([]Report, error) {
	reps, err := g.declareWar(a, b)
	if err == nil && g.journal != nil {
		g.journal.Record(g.pairEntry("war", a, b))
	}
	return reps, err
}

// declareWar is DeclareWar unrecorded (a granted request's act, a fixture's war).
func (g *Game) declareWar(a, b int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	ea, eb := g.Empires[a], g.Empires[b]
	switch st := ea.StanceWith(b); {
	case a == b || st == Unknown:
		return nil, fmt.Errorf("never met the %s", eb.Species)
	case st != Peace:
		return nil, fmt.Errorf("not at peace with the %s", eb.Species)
	}
	if n := g.WarBarred(a, b); n > 0 {
		return nil, fmt.Errorf("the %s are your friends: no war on them for %s", eb.Species, countOf(n, "day"))
	}
	g.setStance(a, b, War)
	eb.Relations[a].WarsOn++
	eb.Relations[a].Friendship = 0 // a broke it: b owes a nothing
	g.dropOffers(a, b)
	return []Report{
		{a, dipEvent(WarDeclared, b, "War declared on "+eb.Species.String())},
		{b, dipEvent(WarDeclared, a, "War declared by "+ea.Species.String())},
	}, nil
}

// Friendship's weights, a neutral bot's goodwill genes (bot.NeutralDiplomacy:
// TradeW, HelpW, HospW, Forgive), and its brake on war:
// Relation.Friendship.
const (
	FriendTrade = 30
	FriendHelp  = 30
	FriendHosp  = 0.2
	FriendFade  = 0.01
	FriendBar   = 50
)

// WarBarred is the closes before a may declare war on b if b gave a
// nothing more: a's Friendship toward b fading under FriendBar; 0 when
// free (or out of range).
func (g *Game) WarBarred(a, b int) int {
	if a < 0 || a >= len(g.Empires) || b < 0 || b >= len(g.Empires) || a == b {
		return 0
	}
	n := 0
	for f := g.Empires[a].Relations[b].Friendship; f >= FriendBar; f *= 1 - FriendFade {
		n++
	}
	return n
}

// dropOffers removes the pending Offers between a and b, either way.
func (g *Game) dropOffers(a, b int) {
	g.Offers = slices.DeleteFunc(g.Offers, func(o Offer) bool {
		return o.From == a && o.To == b || o.From == b && o.To == a
	})
}

// BreakAlliance puts allies a and b back at Peace: b counts Broken, the
// Offers between them are dropped, AllianceBroken for both.
func (g *Game) BreakAlliance(a, b int) ([]Report, error) {
	reps, err := g.breakAlliance(a, b)
	if err == nil && g.journal != nil {
		g.journal.Record(g.pairEntry("break", a, b))
	}
	return reps, err
}

// breakAlliance is BreakAlliance unrecorded (a granted request's act).
func (g *Game) breakAlliance(a, b int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	ea, eb := g.Empires[a], g.Empires[b]
	switch st := ea.StanceWith(b); {
	case a == b || st == Unknown:
		return nil, fmt.Errorf("never met the %s", eb.Species)
	case st != Alliance:
		return nil, fmt.Errorf("not allied with the %s", eb.Species)
	}
	g.setStance(a, b, Peace)
	eb.Relations[a].Broken++
	g.dropOffers(a, b)
	return []Report{
		{a, dipEvent(AllianceBroken, b, "Alliance with "+eb.Species.String()+" broken")},
		{b, dipEvent(AllianceBroken, a, ea.Species.String()+" broke our alliance")},
	}, nil
}

// offerEntry is a's "offer" of kind k to b, a request's third named in
// Target ("empire N").
func (g *Game) offerEntry(a, b int, k OfferKind, third int) journal.Entry {
	en := g.pairEntry("offer", a, b)
	en.Kind = k.String()
	if third >= 0 {
		en.Target = "empire " + strconv.Itoa(third)
	}
	return en
}

// offer makes a's Offer of kind k to b (about third), the stance checked
// by the caller: refused while one from a to b pends or within
// OfferCooldown of a's last. Sets LastOffer; Reports for b then a.
func (g *Game) offer(a, b int, k OfferKind, third int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	ea, eb := g.Empires[a], g.Empires[b]
	if slices.ContainsFunc(g.Offers, func(o Offer) bool { return o.From == a && o.To == b }) {
		return nil, fmt.Errorf("already offered to the %s", eb.Species)
	}
	r := ea.Rel(b)
	if r.LastOffer >= 0 && g.Day-r.LastOffer < OfferCooldown {
		return nil, fmt.Errorf("offered %d days ago", g.Day-r.LastOffer)
	}
	g.Offers = append(g.Offers, Offer{From: a, To: b, Kind: k, Day: g.Day, Third: third})
	r.LastOffer = g.Day
	from, to := ea.Species.String(), eb.Species.String()
	kind, theirs, ours := PeaceOffered, from+" offers peace", "Peace offered to "+to
	switch {
	case k == AllianceOffer:
		kind, theirs, ours = AllianceOffered, from+" offers alliance", "Alliance offered to "+to
	case k.Trade():
		kind, theirs, ours = TradeOffered, from+" offers trade of "+k.String(), "Trade of "+k.String()+" offered to "+to
	case k.Request():
		act := requestNames[k-AskPeaceWith] + g.Empires[third].Species.String()
		kind, theirs, ours = HelpAsked, from+" asks: "+act, "Asked "+to+": "+act
	}
	return []Report{{b, dipEvent(kind, a, theirs)}, {a, dipEvent(kind, b, ours)}}, nil
}

// OfferPeace makes a's peace Offer to b, refused unless at War.
func (g *Game) OfferPeace(a, b int) ([]Report, error) {
	reps, err := g.offerPeace(a, b)
	if err == nil && g.journal != nil {
		g.journal.Record(g.offerEntry(a, b, PeaceOffer, -1))
	}
	return reps, err
}

// offerPeace is OfferPeace unrecorded (a granted request's act).
func (g *Game) offerPeace(a, b int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	if g.Empires[a].StanceWith(b) != War {
		return nil, fmt.Errorf("not at war with the %s", g.Empires[b].Species)
	}
	return g.offer(a, b, PeaceOffer, -1)
}

// OfferAlliance makes a's alliance Offer to b, refused unless at Peace.
func (g *Game) OfferAlliance(a, b int) ([]Report, error) {
	reps, err := g.offerAlliance(a, b)
	if err == nil && g.journal != nil {
		g.journal.Record(g.offerEntry(a, b, AllianceOffer, -1))
	}
	return reps, err
}

// offerAlliance is OfferAlliance unrecorded (a granted request's act).
func (g *Game) offerAlliance(a, b int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	eb := g.Empires[b]
	switch st := g.Empires[a].StanceWith(b); {
	case a == b || st == Unknown:
		return nil, fmt.Errorf("never met the %s", eb.Species)
	case st != Peace:
		return nil, fmt.Errorf("not at peace with the %s", eb.Species)
	}
	return g.offer(a, b, AllianceOffer, -1)
}

// OfferTrade makes a's Offer of the exchange k to b, refused unless at
// Peace or Alliance and when neither would learn anything.
func (g *Game) OfferTrade(a, b int, k OfferKind) ([]Report, error) {
	reps, err := g.offerTrade(a, b, k)
	if err == nil && g.journal != nil {
		g.journal.Record(g.offerEntry(a, b, k, -1))
	}
	return reps, err
}

// offerTrade is OfferTrade's body, unrecorded.
func (g *Game) offerTrade(a, b int, k OfferKind) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	eb := g.Empires[b]
	if !k.Trade() {
		return nil, fmt.Errorf("%s is not an exchange", k)
	}
	switch st := g.Empires[a].StanceWith(b); {
	case a == b || st == Unknown:
		return nil, fmt.Errorf("never met the %s", eb.Species)
	case st == War:
		return nil, fmt.Errorf("at war with the %s: only peace can be offered", eb.Species)
	}
	if g.Gain(a, b, k).Zero() && g.Gain(b, a, k).Zero() {
		return nil, fmt.Errorf("nothing to exchange with the %s", eb.Species)
	}
	return g.offer(a, b, k, -1)
}

// The requests' phrases by kind − AskPeaceWith, followed by the third's
// species (the act asked, the act in an answer), and the stance To must
// hold with Third (requestValid).
var (
	requestNames = [...]string{"peace with ", "war on ", "break with ", "alliance with "}
	requestNeeds = [...]Stance{War, Peace, Alliance, Peace}
)

// requestValid reports a request o still makes sense: From and To allied,
// To's stance with Third requestNeeds'.
func (g *Game) requestValid(o Offer) bool {
	if !o.Kind.Request() || o.Third < 0 || o.Third >= len(g.Empires) || o.Third == o.From || o.Third == o.To {
		return false
	}
	return g.Empires[o.From].StanceWith(o.To) == Alliance &&
		g.Empires[o.To].StanceWith(o.Third) == requestNeeds[o.Kind-AskPeaceWith]
}

// Request makes a's request of kind k to its ally b to act toward t (an
// Offer, Third t): refused unless allied, t a third a has met and not
// defeated, and requestValid.
func (g *Game) Request(a, b int, k OfferKind, t int) ([]Report, error) {
	reps, err := g.request(a, b, k, t)
	if err == nil && g.journal != nil {
		g.journal.Record(g.offerEntry(a, b, k, t))
	}
	return reps, err
}

// request is Request's body, unrecorded.
func (g *Game) request(a, b int, k OfferKind, t int) ([]Report, error) {
	if err := g.defeatedErr(a, b); err != nil {
		return nil, err
	}
	if !k.Request() {
		return nil, fmt.Errorf("%s is not a request", k)
	}
	ea, eb := g.Empires[a], g.Empires[b]
	if a == b || ea.StanceWith(b) != Alliance {
		return nil, fmt.Errorf("not allied with the %s", eb.Species)
	}
	if t < 0 || t >= len(g.Empires) || t == a || t == b || ea.StanceWith(t) == Unknown {
		name := "them"
		if t >= 0 && t < len(g.Empires) {
			name = "the " + g.Empires[t].Species.String()
		}
		return nil, fmt.Errorf("never met %s", name)
	}
	// A defeated Third keeps its stances: requestValid alone would pass.
	if err := g.defeatedErr(t); err != nil {
		return nil, err
	}
	o := Offer{From: a, To: b, Kind: k, Third: t}
	if !g.requestValid(o) {
		what := [...]string{"at war with", "at peace with", "allied with", "at peace with"}[k-AskPeaceWith]
		return nil, fmt.Errorf("the %s are not %s the %s", eb.Species, what, g.Empires[t].Species)
	}
	return g.offer(a, b, k, t)
}

// Answer is o.To's answer to the pending Offer o, which is removed (a moot
// request is dropped, "moot", whatever the answer). Refused: From counts
// Refused, OfferRefused for both. Accepted: a peace or an alliance sets
// the stance (PeaceMade, AllianceMade), an exchange (still at Peace or
// Alliance) applies both Gains as they stand now (learn, TradeMade with
// the tech learnt, the singularity's Discovered after it when that tech
// completes the tree, Trades counted), a request is granted (grant). A
// crossed offer the acceptance makes moot is dropped, not left to expire
// as a refusal.
func (g *Game) Answer(o Offer, accept bool) ([]Report, error) {
	if accept && g.barredRequest(o) {
		accept = false // a refusal, its reason in To's Report
	}
	reps, err := g.answer(o, accept)
	if err == nil && g.journal != nil {
		en := g.pairEntry("answer", o.To, o.From)
		en.Kind, en.Accept = o.Kind.String(), accept
		if o.Kind.Request() {
			en.Target = "empire " + strconv.Itoa(o.Third)
		}
		g.journal.Record(en)
	}
	return reps, err
}

// barredRequest reports o an ally's request for a war To may not declare
// (WarBarred): accepted, it is refused.
func (g *Game) barredRequest(o Offer) bool {
	return o.Kind == AskWarOn && g.WarBarred(o.To, o.Third) > 0
}

// answer is Answer unrecorded (an offer expiring: diplomacy).
func (g *Game) answer(o Offer, accept bool) ([]Report, error) {
	if err := g.defeatedErr(o.From, o.To); err != nil {
		return nil, err
	}
	i := slices.Index(g.Offers, o)
	if i < 0 {
		return nil, fmt.Errorf("no such pending offer")
	}
	ef, et := g.Empires[o.From], g.Empires[o.To]
	if st := ef.StanceWith(o.To); accept && o.Kind.Trade() && st != Peace && st != Alliance {
		return nil, fmt.Errorf("not at peace with the %s", et.Species)
	}
	g.Offers = slices.Delete(g.Offers, i, i+1)
	// No Refused for a premise gone.
	if o.Kind.Request() && !g.requestValid(o) {
		return nil, fmt.Errorf("the request is moot")
	}
	from, to := ef.Species.String(), et.Species.String()
	if !accept {
		ef.Relations[o.To].Refused++
		what := "peace"
		switch {
		case o.Kind == AllianceOffer:
			what = "alliance"
		case o.Kind.Trade():
			what = "trade of " + o.Kind.String()
		case o.Kind.Request():
			what = requestNames[o.Kind-AskPeaceWith] + g.Empires[o.Third].Species.String()
		}
		why := ""
		if g.barredRequest(o) {
			why = fmt.Sprintf(" (the %s are our friends)", g.Empires[o.Third].Species)
		}
		return []Report{
			{o.From, dipEvent(OfferRefused, o.To, to+" refuses "+what)},
			{o.To, dipEvent(OfferRefused, o.From, "Refused "+from+": "+what+why)},
		}, nil
	}
	switch {
	case o.Kind.Request():
		return g.grant(o)
	case o.Kind == PeaceOffer:
		g.setStance(o.From, o.To, Peace)
		g.Offers = slices.DeleteFunc(g.Offers, func(x Offer) bool { return x.From == o.To && x.To == o.From })
		return []Report{
			{o.From, dipEvent(PeaceMade, o.To, "Peace with "+to)},
			{o.To, dipEvent(PeaceMade, o.From, "Peace with "+from)},
		}, nil
	case o.Kind == AllianceOffer:
		g.setStance(o.From, o.To, Alliance)
		g.Offers = slices.DeleteFunc(g.Offers, func(x Offer) bool {
			return x.From == o.To && x.To == o.From && x.Kind == AllianceOffer
		})
		return []Report{
			{o.From, dipEvent(AllianceMade, o.To, "Alliance with "+to)},
			{o.To, dipEvent(AllianceMade, o.From, "Alliance with "+from)},
		}, nil
	}
	// Order-free: what From learns, To knew already.
	gf, gt := g.learn(o.From, o.To, o.Kind), g.learn(o.To, o.From, o.Kind)
	ef.stopTakeovers()
	et.stopTakeovers()
	g.Offers = slices.DeleteFunc(g.Offers, func(x Offer) bool {
		return x.From == o.To && x.To == o.From && x.Kind == o.Kind
	})
	ef.Relations[o.To].Trades++
	et.Relations[o.From].Trades++
	ef.Relations[o.To].Friendship += FriendTrade
	et.Relations[o.From].Friendship += FriendTrade
	reps := []Report{{o.From, tradeEvent(o.Kind, o.To, to, gf, gt)}}
	if x, ok := singularity(ef.Research, gf.Tech); ok {
		reps = append(reps, Report{o.From, x})
	}
	reps = append(reps, Report{o.To, tradeEvent(o.Kind, o.From, from, gt, gf)})
	if x, ok := singularity(et.Research, gt.Tech); ok {
		reps = append(reps, Report{o.To, x})
	}
	return reps, nil
}

// tradeEvent is a TradeMade with other (named name), its Tech gn's (what
// we learnt), its text also naming gave (what other learnt from us).
func tradeEvent(k OfferKind, other int, name string, gn, gave Gain) Event {
	x := dipEvent(TradeMade, other, tradeText(k, name, gn, gave))
	x.Tech = gn.Tech
	return x
}

// grant is Answer's accept of the request o, already removed from Offers:
// To acts toward Third unrecorded (its error returned, nothing counted),
// From counts Helped; HelpGiven for From then To, then the act's Reports.
func (g *Game) grant(o Offer) ([]Report, error) {
	if !g.requestValid(o) {
		return nil, fmt.Errorf("the request is moot")
	}
	var act []Report
	var err error
	// An offer To already made Third of the kind asked IS the act.
	pending := func(k OfferKind) bool {
		return slices.ContainsFunc(g.Offers, func(x Offer) bool { return x.From == o.To && x.To == o.Third && x.Kind == k })
	}
	switch o.Kind {
	case AskPeaceWith:
		if !pending(PeaceOffer) {
			act, err = g.offerPeace(o.To, o.Third)
		}
	case AskWarOn:
		act, err = g.declareWar(o.To, o.Third)
	case AskBreakWith:
		act, err = g.breakAlliance(o.To, o.Third)
	case AskAllianceWith:
		if !pending(AllianceOffer) {
			act, err = g.offerAlliance(o.To, o.Third)
		}
	}
	if err != nil {
		return nil, err
	}
	ef, et := g.Empires[o.From], g.Empires[o.To]
	ef.Relations[o.To].Helped++
	ef.Relations[o.To].Friendship += FriendHelp
	what := requestNames[o.Kind-AskPeaceWith] + g.Empires[o.Third].Species.String()
	return append([]Report{
		{o.From, dipEvent(HelpGiven, o.To, et.Species.String()+" grants: "+what)},
		{o.To, dipEvent(HelpGiven, o.From, "Granted "+ef.Species.String()+": "+what)},
	}, act...), nil
}

// tradeText is "4 stars from Orfa" (or the tech), else "Nothing learnt
// from Orfa", then "; taught them 2 stars" (or "nothing"): gave is what
// Orfa learnt.
func tradeText(k OfferKind, other string, gn, gave Gain) string {
	what := gainText(k, gn)
	if what == "" {
		what = "Nothing learnt"
	}
	taught := gainText(k, gave)
	if taught == "" {
		taught = "nothing"
	}
	return what + " from " + other + "; taught them " + taught
}

// gainText is gn as "4 stars", "1 lane" or the tech, "" when none.
func gainText(k OfferKind, gn Gain) string {
	switch {
	case k == TradeStars && gn.Stars > 0:
		return countOf(gn.Stars, "star")
	case k == TradeLanes && gn.Lanes > 0:
		return countOf(gn.Lanes, "lane")
	case k == TradeResearch && gn.Tech != NoTech:
		return gn.Tech.String()
	}
	return ""
}

// countOf is "1 star", "4 stars".
func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// diplomacy is EndDay's close, after the invasions: (a) the empires
// Present together at a star meet, stars then pairs in index order; (b)
// the ledger: per ordered pair a WarDay, an AllyDay, or at Peace one
// Intrusion per armed ship of b out of the lanes where a holds a colony
// and b none; (c) in Offers order, a moot request dropped silently, an
// offer OfferExpiry days old refused; (d) every Friendship fades by
// FriendFade, then each empire's Alien Hospitality q summed goes into the
// Hospitality of those at Peace or Alliance with it, FriendHosp a point
// into their Friendship.
func (g *Game) diplomacy() []Report {
	n := len(g.Empires)
	if n < 2 {
		return nil
	}
	var reps []Report
	here := make([]int, 0, n)
	for st := range g.Cluster.Stars {
		here = here[:0]
		for i, e := range g.Empires {
			if e.Present(st) {
				here = append(here, i)
			}
		}
		for i, a := range here {
			for _, b := range here[i+1:] {
				reps = append(reps, g.Contact(a, b)...)
			}
		}
	}
	colonised := make([]map[int]bool, n)
	for i, e := range g.Empires {
		colonised[i] = map[int]bool{}
		for _, c := range e.Colonies {
			colonised[i][c.Star] = true
		}
	}
	for _, ea := range g.Empires {
		for _, eb := range g.Empires {
			if ea == eb {
				continue
			}
			r := &ea.Relations[eb.Index]
			switch r.Stance {
			case War:
				r.WarDays++
			case Alliance:
				r.AllyDays++
			case Peace:
				for _, s := range eb.Ships {
					if s.Lane < 0 && colonised[ea.Index][s.Star] && !colonised[eb.Index][s.Star] && s.Design.Weapons() > 0 {
						r.Intrusions++
					}
				}
			}
		}
	}
	for _, o := range slices.Clone(g.Offers) {
		if o.Kind.Request() && !g.requestValid(o) {
			if i := slices.Index(g.Offers, o); i >= 0 {
				g.Offers = slices.Delete(g.Offers, i, i+1)
			}
			continue
		}
		if o.Day+OfferExpiry <= g.Day {
			r, _ := g.answer(o, false)
			reps = append(reps, r...)
		}
	}
	for _, ea := range g.Empires {
		for j := range ea.Relations {
			ea.Relations[j].Friendship *= 1 - FriendFade
		}
	}
	for _, ea := range g.Empires {
		hosp := 0
		for _, c := range ea.Colonies {
			if it, q := c.Diverted(); it == AlienHospitality {
				hosp += q
			}
		}
		if hosp == 0 {
			continue
		}
		ea.HospitalityGiven += hosp
		for _, eb := range g.Empires {
			if st := eb.StanceWith(ea.Index); eb != ea && (st == Peace || st == Alliance) {
				eb.Relations[ea.Index].Hospitality += hosp
				eb.Relations[ea.Index].Friendship += FriendHosp * float64(hosp)
			}
		}
	}
	return reps
}

// AbilityState is empire i's timed ability: never ready when passive
// (Recharge 0) or asleep (AbilityAsleep), else Recharge − (Day −
// AbilityUsed) days remain (ready at 0; it starts empty, AbilityUsed 0).
func (g *Game) AbilityState(i int) (ready bool, days int) {
	e := g.Empires[i]
	rc := e.Species.Info().Recharge
	if rc == 0 || g.AbilityAsleep(i) {
		return false, max(0, rc-(g.Day-e.AbilityUsed))
	}
	if days = rc - (g.Day - e.AbilityUsed); days <= 0 {
		return true, 0
	}
	return false, days
}

// enemies is the empires at War with i, in index order.
func (g *Game) enemies(i int) []int {
	var out []int
	for j := range g.Empires {
		if j != i && g.Empires[i].StanceWith(j) == War {
			out = append(out, j)
		}
	}
	return out
}

// forcePeace is SA_ForcePeace ("Force all aliens to make peace with
// you"): every enemy of i at Peace, their offers dropped, PeaceMade for i
// then j; the ledgers untouched. Refused with no enemy.
func (g *Game) forcePeace(i int) ([]Report, error) {
	foes := g.enemies(i)
	if len(foes) == 0 {
		return nil, fmt.Errorf("at war with nobody")
	}
	var reps []Report
	sp := g.Empires[i].Species.String()
	for _, j := range foes {
		g.setStance(i, j, Peace)
		g.dropOffers(i, j)
		reps = append(reps,
			Report{i, dipEvent(PeaceMade, j, "Peace forced on "+g.Empires[j].Species.String())},
			Report{j, dipEvent(PeaceMade, i, "Peace forced by "+sp)})
	}
	return reps, nil
}

// causeWar is SA_CauseWar ("Make aliens dislike anyone at war with
// you"): every third empire that has met an enemy x of i counts Defamed
// about x. No Report (ASSUMPTION: not announced). Refused with no enemy.
func (g *Game) causeWar(i int) ([]Report, error) {
	foes := g.enemies(i)
	if len(foes) == 0 {
		return nil, fmt.Errorf("at war with nobody")
	}
	for _, x := range foes {
		for y, ey := range g.Empires {
			if y != i && y != x && ey.StanceWith(x) != Unknown {
				ey.Relations[x].Defamed++
			}
		}
	}
	return nil, nil
}
