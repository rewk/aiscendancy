package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/dip"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The negotiation (Talk: a Species screen row clicked): the .dip lines over
// our layout: their portrait (smraces) and facts, a speech box (ours
// dimmed, then theirs), our lines as a menu (the player's DA_ texts),
// rebuilt from the game at every use. A line the game would refuse is
// dimmed but still said: the refusal becomes their line.
//
// An offer to a bot is answered at once (its Decide, Game.Answer), their
// line ACCEPT_<Key> or REJECT_<Key>; without a bot it waits. The opening
// line is RESPONSE_Discover on the session's first talk (a.greeted, not
// saved), NoDipResponse after. A missing script or key falls back to the
// Report texts, or our plain words.
//
// Their pending offers to the player are answered here, one at a time,
// oldest first (incoming): announced as their line (their race's DA_
// text, as the original's proposals), the menu topped by Accept and
// Refuse (player.dip's DR_ACCEPT_/DR_REJECT_ texts), alone with the leave
// line, until it is gone.
type negotiateScreen struct {
	j       int            // the empire talked to
	picking bool           // the request sub-menu is up
	ask     game.OfferKind // the request picked, while picking
	ours    string         // our last line, "" before any
	theirs  string         // their last line
	refusal bool           // theirs is the game's refusal
	told    game.Offer     // their offer last announced, while toldOK
	toldOK  bool
}

func newNegotiateScreen(j int) *negotiateScreen { return &negotiateScreen{j: j} }

// Enter says their opening line: their pending offer, else the greeting.
func (s *negotiateScreen) Enter(a *app) {
	if a.greeted == nil {
		a.greeted = map[*game.Empire]bool{}
	}
	e := a.g.Empires[s.j]
	key, plain := "NoDipResponse", fmt.Sprintf("The %s are listening.", e.Species)
	if !a.greeted[e] {
		key, plain = "RESPONSE_Discover", fmt.Sprintf("The %s greet the %s.", e.Species, a.e.Species)
		a.greeted[e] = true
	}
	s.reply(a, key, "", plain)
	s.announce(a)
}

// announce makes their oldest pending offer their line, once: a new one
// (made under the screen, or next after an answer) is said when it shows;
// one gone unanswered here (lapsed, or answered in a popup over the
// screen) leaves "Offer withdrawn." (answer clears toldOK first).
func (s *negotiateScreen) announce(a *app) {
	o, ok := incoming(a, s.j)
	if !ok && s.toldOK {
		s.theirs, s.refusal, s.toldOK = "Offer withdrawn.", false, false
		return
	}
	if !ok || s.toldOK && o == s.told {
		return
	}
	s.told, s.toldOK = o, true
	s.theirs, s.refusal = offerSpeech(a, o), false
}

// offerSpeech is offer o to the player in its maker's words: their race
// script's DA_ text (%y us, %m them, %t the third), else plain.
func offerSpeech(a *app, o game.Offer) string {
	them, us := a.g.Empires[o.From].Species.String(), a.e.Species.String()
	third, plain := "", fmt.Sprintf("The %s propose %s.", them, offerWords(o.Kind))
	if o.Kind.Request() {
		third = a.g.Empires[o.Third].Species.String()
		plain = fmt.Sprintf("The %s ask for %s.", them, requestWords(a, o))
	}
	if t, ok := a.scripts().races[a.g.Empires[o.From].Species].Say(offerKeys[o.Kind], us, them, third, ""); ok {
		return t
	}
	return plain
}

// dipScripts are the .dip files parsed, nil where the archive lacks one.
type dipScripts struct {
	player *dip.Script
	races  [game.NumSpecies]*dip.Script
}

// scripts reads player.dip and raceNN.dip once; a failed file stays nil.
func (a *app) scripts() *dipScripts {
	if a.dips != nil {
		return a.dips
	}
	a.dips = &dipScripts{}
	if a.a == nil {
		return a.dips
	}
	read := func(name string) *dip.Script {
		b, err := a.a.Game.Read(name)
		if err == nil {
			var s *dip.Script
			if s, err = dip.Parse(b); err == nil {
				return s
			}
		}
		log.Printf("%s: %v", name, err)
		return nil
	}
	a.dips.player = read("player.dip")
	for sp := range game.NumSpecies {
		a.dips.races[sp] = read(fmt.Sprintf("race%02d.dip", sp))
	}
	return a.dips
}

// negAct is what a menu line does.
type negAct uint8

const (
	negDeclare negAct = iota // DeclareWar
	negBreak                 // BreakAlliance
	negOffer                 // an offer of kind: OfferPeace, OfferAlliance, OfferTrade
	negAsk                   // a request of kind: opens the sub-menu
	negThird                 // the sub-menu's species: Request(kind, third)
	negBack                  // the sub-menu's way back
	negLeave                 // DA_NoDipAction: back to the Species screen
	negAccept                // their pending offer kind accepted: Answer
	negRefuse                // and refused
)

// offerKeys are each OfferKind's player DA_ key.
var offerKeys = [game.AskAllianceWith + 1]string{
	game.PeaceOffer: "OfferPeace", game.AllianceOffer: "ProposeAlliance",
	game.TradeStars: "TradeStars", game.TradeLanes: "TradeStarlanes", game.TradeResearch: "TradeResearch",
	game.AskPeaceWith: "OfferPeaceToOtherRace", game.AskWarOn: "DeclareWarOnOtherRace",
	game.AskBreakWith: "BreakAllianceWithOtherRace", game.AskAllianceWith: "ProposeAllianceToOtherRace",
}

// offerPlain is our line for each kind without player.dip (%t the third).
var offerPlain = [game.AskAllianceWith + 1]string{
	game.PeaceOffer: "We offer peace.", game.AllianceOffer: "We propose an alliance.",
	game.TradeStars: "Let us exchange our stars.", game.TradeLanes: "Let us exchange our lanes.",
	game.TradeResearch: "Let us exchange research.",
	game.AskPeaceWith:  "Offer peace to the %t.", game.AskWarOn: "Declare war on the %t.",
	game.AskBreakWith: "Break your alliance with the %t.", game.AskAllianceWith: "Offer an alliance to the %t.",
}

// replyKey is kind k's suffix in the race's ACCEPT_/REJECT_ keys.
func replyKey(k game.OfferKind) string {
	if k == game.AllianceOffer {
		return "Alliance"
	}
	return offerKeys[k]
}

// negLine is one menu line (third −1 none).
type negLine struct {
	act   negAct
	kind  game.OfferKind
	third int
	key   string
	text  string
	dim   bool
	r     rect
}

// The layout in mapArea: portrait, box under it, menu under the box.
var (
	negX0, negX1 = mapArea.X0 + padX, mapArea.X1 - padX
	negPortY     = mapArea.Y0 + padY
	negPortSide  = 100 * zoom // smraces frames are 100x100
	negBox       = rect{negX0, negPortY + negPortSide + 16, negX1, negPortY + negPortSide + 16 + 2*padY + int(6*logLineH)}
	negMenuTop   = float64(negBox.Y1) + logLineH
)

// negBoxLines is the speech box's lines, negOursMax at most ours.
const negBoxLines, negOursMax = 6, 2

// cols is how many bigFace characters fit in w native px.
func cols(w int) int { return int(float64(w) / ui.Width("0", bigFace)) }

// names are (them, us): the species' names.
func (s *negotiateScreen) names(a *app) (string, string) {
	return a.g.Empires[s.j].Species.String(), a.e.Species.String()
}

// our is the player's line for key from player.dip (%y them, %m us, %t
// third), else plain with %t filled.
func (s *negotiateScreen) our(a *app, key, plain, third string) string {
	them, us := s.names(a)
	if t, ok := a.scripts().player.Say(key, them, us, third, ""); ok {
		return t
	}
	return strings.ReplaceAll(plain, "%t", third)
}

// reply sets their line from their race script's response key (%y us,
// %m them, %t third), else plain.
func (s *negotiateScreen) reply(a *app, key, third, plain string) {
	them, us := s.names(a)
	s.refusal = false
	if t, ok := a.scripts().races[a.g.Empires[s.j].Species].Reply(key, us, them, third, ""); ok {
		s.theirs = t
		return
	}
	s.theirs = plain
}

// answerLines are Accept and Refuse for their offer o, our player.dip
// DR_ACCEPT_/DR_REJECT_ texts (%y them, %m us, %t the third), else plain.
func (s *negotiateScreen) answerLines(a *app, o game.Offer) []negLine {
	them, us := s.names(a)
	third := ""
	if o.Kind.Request() {
		third = a.g.Empires[o.Third].Species.String()
	}
	var ls []negLine
	for _, w := range []struct {
		act          negAct
		prefix, text string
	}{{negAccept, "ACCEPT_", "Accept"}, {negRefuse, "REJECT_", "Refuse"}} {
		key := w.prefix + replyKey(o.Kind)
		if t, ok := a.scripts().player.Reply(key, them, us, third, ""); ok {
			w.text = t
		}
		ls = append(ls, negLine{act: w.act, kind: o.Kind, third: o.Third, key: key, text: w.text})
	}
	return ls
}

// thirds are the empires the player has met, j aside: whom j met stays
// unknown (one it never met refuses as any wrong stance).
func (s *negotiateScreen) thirds(a *app) []int {
	var out []int
	for t := range a.g.Empires {
		if t != a.player && t != s.j && a.e.StanceWith(t) != game.Unknown {
			out = append(out, t)
		}
	}
	return out
}

// lines are the menu now: the sub-menu while picking a third (allied
// still), else Accept, Refuse and the leave line while an offer of
// theirs is pending, else the stance's lines and the leave line.
func (s *negotiateScreen) lines(a *app) []negLine {
	var ls []negLine
	st := a.e.StanceWith(s.j)
	if s.picking && st == game.Alliance {
		for _, t := range s.thirds(a) {
			ls = append(ls, negLine{act: negThird, kind: s.ask, third: t, text: "the " + a.g.Empires[t].Species.String()})
		}
		ls = append(ls, negLine{act: negBack, third: -1, text: "back"})
	} else if o, ok := incoming(a, s.j); ok {
		ls = append(s.answerLines(a, o), negLine{act: negLeave, third: -1, key: "NoDipAction", text: s.our(a, "NoDipAction", "Goodbye.", "...")})
	} else {
		busy := offerDim(a, s.j)
		add := func(act negAct, k game.OfferKind, key, plain string, dim bool) {
			ls = append(ls, negLine{act: act, kind: k, third: -1, key: key, text: s.our(a, key, plain, "..."), dim: dim})
		}
		offer := func(k game.OfferKind) {
			dim := busy // an exchange is never dimmed: what they hold stays unknown
			act := negOffer
			if k.Request() {
				act, dim = negAsk, dim || len(s.thirds(a)) == 0
			}
			add(act, k, offerKeys[k], offerPlain[k], dim)
		}
		trades := func() {
			for _, k := range []game.OfferKind{game.TradeLanes, game.TradeStars, game.TradeResearch} {
				offer(k)
			}
		}
		switch st {
		case game.War:
			offer(game.PeaceOffer)
		case game.Peace:
			add(negDeclare, 0, "DeclareWar", "We declare war on you.", false)
			offer(game.AllianceOffer)
			trades()
		case game.Alliance:
			add(negBreak, 0, "BreakAlliance", "Our alliance is over.", false)
			trades()
			for k := game.AskPeaceWith; k <= game.AskAllianceWith; k++ {
				offer(k)
			}
		}
		add(negLeave, 0, "NoDipAction", "Goodbye.", false)
	}
	y := negMenuTop
	n := cols(negX1 - negX0)
	for i := range ls {
		ls[i].text = wrap(ls[i].text, n)
		h := float64(strings.Count(ls[i].text, "\n")+1)*logLineH + logLineH/2
		ls[i].r = rect{negX0, int(y), negX1, int(y + h)}
		y += h
	}
	return ls
}

// say is line l said: the game's call, the answer, their line.
func (s *negotiateScreen) say(a *app, l negLine) error {
	switch l.act {
	case negLeave:
		return a.pop()
	case negBack:
		s.picking = false
		return nil
	case negAsk:
		s.picking, s.ask = true, l.kind
		return nil
	case negAccept, negRefuse:
		s.answer(a, l)
		return nil
	}
	third := ""
	if l.act == negThird {
		third = a.g.Empires[l.third].Species.String()
		s.ours = s.our(a, offerKeys[l.kind], offerPlain[l.kind], third)
		s.picking = false
	} else {
		s.ours = strings.ReplaceAll(l.text, "\n", " ")
	}
	var reps []game.Report
	var err error
	switch l.act {
	case negDeclare:
		reps, err = a.g.DeclareWar(a.player, s.j)
	case negBreak:
		reps, err = a.g.BreakAlliance(a.player, s.j)
	case negThird:
		reps, err = a.g.Request(a.player, s.j, l.kind, l.third)
	case negOffer:
		switch {
		case l.kind == game.PeaceOffer:
			reps, err = a.g.OfferPeace(a.player, s.j)
		case l.kind == game.AllianceOffer:
			reps, err = a.g.OfferAlliance(a.player, s.j)
		default:
			reps, err = a.g.OfferTrade(a.player, s.j, l.kind)
		}
	}
	evs := a.record(reps)
	if err != nil {
		s.theirs, s.refusal = err.Error(), true
		return nil
	}
	switch l.act {
	case negDeclare:
		s.reply(a, "RESPONSE_DeclareWar", third, texts(evs))
		return nil
	case negBreak:
		s.reply(a, "RESPONSE_BreakAlliance", third, texts(evs))
		return nil
	}
	o, ok := s.pending(a, l.kind, l.third)
	b := botOf(a, s.j)
	if !ok || b == nil {
		s.theirs, s.refusal = "They will answer later.", false
		return nil
	}
	accept := b.Decide(o)
	before := a.e.Research.Known // an exchange's discoveries (learnt)
	reps, err = a.g.Answer(o, accept)
	evs = a.record(reps)
	a.learnt(before, evs)
	if err != nil {
		s.theirs, s.refusal = err.Error(), true
		return nil
	}
	key := "REJECT_"
	if accept {
		key = "ACCEPT_"
	}
	s.reply(a, key+replyKey(l.kind), third, texts(evs))
	return nil
}

// answer is their oldest pending offer answered by line l (Accept or
// Refuse): Game.Answer, its reports logged (an exchange's discoveries
// learnt); their line the next offer, else the reports' texts.
func (s *negotiateScreen) answer(a *app, l negLine) {
	s.ours = strings.ReplaceAll(l.text, "\n", " ")
	o, ok := incoming(a, s.j)
	if !ok {
		s.theirs, s.refusal = fmt.Sprintf("no offer from the %s", a.g.Empires[s.j].Species), true
		return
	}
	before := a.e.Research.Known
	reps, err := a.g.Answer(o, l.act == negAccept)
	evs := a.record(reps)
	a.learnt(before, evs)
	if err != nil {
		s.theirs, s.refusal = err.Error(), true
		return
	}
	s.theirs, s.refusal, s.toldOK = texts(evs), false, false
	s.announce(a)
}

// pending is the player's offer to j of kind k about third (−1 none),
// matched by From/To/Kind/Third: the last pending one may be another's.
func (s *negotiateScreen) pending(a *app, k game.OfferKind, third int) (game.Offer, bool) {
	for _, o := range a.g.PendingOffers(s.j) {
		if o.From == a.player && o.To == s.j && o.Kind == k && o.Third == third {
			return o, true
		}
	}
	return game.Offer{}, false
}

// texts is the events' texts on one line: a reply's fallback.
func texts(evs []game.Event) string {
	var ts []string
	for _, e := range evs {
		ts = append(ts, e.Text)
	}
	return strings.Join(ts, ". ")
}

// click is a left press at native (x,y): the line under it said.
func (s *negotiateScreen) click(a *app, x, y float64) error {
	for _, l := range s.lines(a) {
		if l.r.has(x, y) {
			return s.say(a, l)
		}
	}
	return nil
}

// boxLines are the speech box's lines: ours (counted by ours) then
// theirs.
func (s *negotiateScreen) boxLines() (ls []string, ours int) {
	n := cols(negBox.X1 - negBox.X0 - 2*padX)
	if s.ours != "" {
		ls = strings.Split(wrap(s.ours, n), "\n")
		ls = ls[:min(len(ls), negOursMax)]
	}
	ours = len(ls)
	ls = append(ls, strings.Split(wrap(s.theirs, n), "\n")...)
	return ls[:min(len(ls), negBoxLines)], ours
}

const negotiateHelp = "click: say a line\nEsc: back to the species"

func (s *negotiateScreen) Update(a *app) error {
	s.announce(a)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.mouse()
		if err := s.click(a, x, y); err != nil {
			return err
		}
		a.pressTurn(turnButton(x, y))
	}
	return nil
}

// Render paints the chrome, their portrait and their flag right of it.
func (s *negotiateScreen) Render(a *app, sc *ebiten.Image) {
	drawMapChrome(a, sc, 2)
	half := float64(negPortSide) / 2
	drawSprite(sc, a.spr(portraitBank, int(a.g.Empires[s.j].Species)), float64(negX0)+half, float64(negPortY)+half)
	drawFlag(a, sc, a.g.Empires[s.j], negFlagX, int(float64(negPortY)+logLineH))
}

// negFlagX is the flag's left, right of the portrait; the facts follow it.
var negFlagX = negX0 + negPortSide + 24

// negFacts are the lines beside j's portrait: what the player knows, and
// our friendship toward them (rounded), its days of barred war after.
func negFacts(a *app, j int) []string {
	o := a.g.Empires[j]
	f := fmt.Sprintf("friendship: %.0f", a.e.Rel(j).Friendship)
	if n := barredDays(a, j); n > 0 {
		f += fmt.Sprintf(", no war for %d day%s", n, plural(n))
	}
	return []string{o.Species.String(), "colour: " + game.ColourName(o.Colour), "stance: " + a.e.StanceWith(j).String(), f}
}

// Text writes their facts, the speech box, the menu, the two boxes.
func (s *negotiateScreen) Text(a *app, frame *ebiten.Image) {
	o := a.g.Empires[s.j]
	x := float64(negFlagX + flagW*zoom + 12)
	y := float64(negPortY) + logLineH
	for i, l := range negFacts(a, s.j) {
		c := dim(textColour, 0.8)
		switch {
		case i == 0:
			c = textColour
		case i == 3 && barredDays(a, s.j) > 0:
			c = knownColour
		}
		ui.Print(frame, fitWidth(l, float64(negX1)-x, bigFace), bigFace, x, y+float64(i)*logLineH*1.5, c)
	}
	strokeRect(frame, negBox, dim(textColour, 0.6))
	ls, ours := s.boxLines()
	for i, l := range ls {
		c := textColour
		switch {
		case i < ours:
			c = dim(textColour, 0.6)
		case s.refusal:
			c = errColour
		}
		ui.Print(frame, l, bigFace, float64(negBox.X0+padX), float64(negBox.Y0+padY)+float64(i)*logLineH, c)
	}
	mx, my := a.mouse()
	for _, l := range s.lines(a) {
		c := textColour
		switch {
		case l.r.has(mx, my):
			c = yellow
		case l.dim:
			c = dim(textColour, 0.45)
		}
		ui.Print(frame, l.text, bigFace, float64(l.r.X0), float64(l.r.Y0), c)
	}
	putBox(frame, mapStack[0], negotiateHelp, smallFace, textColour)
	putBox(frame, mapStack[1], "talking to the\n"+o.Species.String(), smallFace, textColour)
}
