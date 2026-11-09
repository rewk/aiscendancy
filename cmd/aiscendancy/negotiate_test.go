package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/bot"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/dip"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// negKeys are the reply keys the race sample answers.
var negKeys = []string{"Alliance", "OfferPeace", "TradeStarlanes", "TradeStars", "TradeResearch",
	"OfferPeaceToOtherRace", "DeclareWarOnOtherRace", "BreakAllianceWithOtherRace", "ProposeAllianceToOtherRace"}

// negScripts are a player and a race script in the files' grammar, each
// text "<key> %y %m %t": the player's DA_ lines and DR_ answers, the
// race's DA_ proposals and DR_ responses.
func negScripts(t *testing.T) *dipScripts {
	t.Helper()
	var p, r strings.Builder
	p.WriteString("header\r\nDIPVALUES\r\nACTIONS\r\n")
	for _, k := range []string{"DeclareWar", "OfferPeace", "ProposeAlliance", "BreakAlliance", "TradeStarlanes",
		"TradeStars", "TradeResearch", "OfferPeaceToOtherRace", "DeclareWarOnOtherRace",
		"BreakAllianceWithOtherRace", "ProposeAllianceToOtherRace", "NoDipAction"} {
		fmt.Fprintf(&p, "DA_%s\r\n%s %%y %%m %%t\r\n\r\n", k, k)
	}
	r.WriteString("ACTIONS\r\n")
	for k := game.PeaceOffer; k <= game.AskAllianceWith; k++ { // their proposals
		fmt.Fprintf(&r, "DA_%s\r\n%s %%y %%m %%t\r\n\r\n", offerKeys[k], offerKeys[k])
	}
	r.WriteString("RESPONSES\r\n")
	keys := []string{"RESPONSE_Discover", "RESPONSE_DeclareWar", "RESPONSE_BreakAlliance", "NoDipResponse"}
	for _, k := range negKeys {
		keys = append(keys, "ACCEPT_"+k, "REJECT_"+k)
	}
	p.WriteString("RESPONSES\r\n") // our answers to theirs
	for _, k := range keys {
		fmt.Fprintf(&r, "DR_%s\r\n%s %%y %%m %%t\r\n\r\n", k, k)
		fmt.Fprintf(&p, "DR_%s\r\n%s %%y %%m %%t\r\n\r\n", k, k)
	}
	r.WriteString("END DIPLOMACY\r\n")
	ps, err := dip.Parse([]byte(p.String()))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := dip.Parse([]byte(r.String()))
	if err != nil {
		t.Fatal(err)
	}
	d := &dipScripts{player: ps}
	for i := range d.races {
		d.races[i] = rs
	}
	return d
}

// negApp is seed 1 with four empires (Snovemdomas the player, Orfa,
// Mebes, Govorom): the player has met all three, the Orfa the Mebes
// only; a bot for each other empire (G set: they judge exchanges), the
// test scripts injected; the species screen, then a click on the Orfa's
// row: Talk.
func negApp(t *testing.T) (*app, *negotiateScreen) {
	t.Helper()
	g := game.NewGame(1, []game.Species{game.Snovemdomas, game.Orfa, game.Mebes, game.Govorom}, game.Average)
	a := testApp(g)
	a.push(newStarmapScreen())
	a.record(g.Contact(0, 1))
	a.record(g.Contact(0, 2))
	a.record(g.Contact(0, 3))
	g.Contact(1, 2)
	a.bots = make([]*bot.Bot, len(g.Empires))
	for i := 1; i < len(g.Empires); i++ {
		a.bots[i] = bot.New(g.Empires[i], bot.Default())
		a.bots[i].G = g
	}
	a.dips = negScripts(t)
	sp := newSpeciesScreen(-1)
	a.push(sp)
	rowClick(a, sp, 0)
	return a, top[*negotiateScreen](t, a)
}

// negKeysOf is the menu's DA_ keys, a third's line as "third:<empire>",
// back as "back".
func negKeysOf(a *app, s *negotiateScreen) []string {
	var out []string
	for _, l := range s.lines(a) {
		switch l.act {
		case negThird:
			out = append(out, fmt.Sprintf("third:%d", l.third))
		case negBack:
			out = append(out, "back")
		default:
			out = append(out, l.key)
		}
	}
	return out
}

// sayKey clicks the centre of the line of key (or "third:N", "back").
func sayKey(t *testing.T, a *app, s *negotiateScreen, key string) {
	t.Helper()
	ks := negKeysOf(a, s)
	i := slices.Index(ks, key)
	if i < 0 {
		t.Fatalf("no %s line among %v", key, ks)
	}
	r := s.lines(a)[i].r
	if err := s.click(a, float64(r.X0+r.X1)/2, float64(r.Y0+r.Y1)/2); err != nil {
		t.Fatal(err)
	}
}

// ally makes empires 0 and 1 allies through the game.
func ally(t *testing.T, g *game.Game) {
	t.Helper()
	if _, err := g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Answer(g.Offers[len(g.Offers)-1], true); err != nil {
		t.Fatal(err)
	}
}

// The menu per stance, its texts the player's script with %y them and
// %m us, every line inside the big area, none overlapping.
func TestNegotiateMenu(t *testing.T) {
	a, s := negApp(t)
	check := func(stance string, want []string) {
		t.Helper()
		if got := negKeysOf(a, s); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", stance, got, want)
		}
		ls := s.lines(a)
		for i, l := range ls {
			if l.r.X0 < mapArea.X0 || l.r.X1 > mapArea.X1 || l.r.Y0 < negBox.Y1 || l.r.Y1 > mapArea.Y1 {
				t.Errorf("%s: %q at %v outside", stance, l.text, l.r)
			}
			if i > 0 && l.r.Y0 < ls[i-1].r.Y1 {
				t.Errorf("%s: %q overlaps the line above", stance, l.text)
			}
		}
	}
	check("peace", []string{"DeclareWar", "ProposeAlliance", "TradeStarlanes", "TradeStars", "TradeResearch", "NoDipAction"})
	if l := s.lines(a)[0]; l.text != "DeclareWar Orfa Snovemdomas ..." {
		t.Errorf("our line %q", l.text)
	}
	ally(t, a.g)
	check("alliance", []string{"BreakAlliance", "TradeStarlanes", "TradeStars", "TradeResearch", "OfferPeaceToOtherRace",
		"DeclareWarOnOtherRace", "BreakAllianceWithOtherRace", "ProposeAllianceToOtherRace", "NoDipAction"})
	if _, err := a.g.BreakAlliance(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	check("war", []string{"OfferPeace", "NoDipAction"})
	sc := ebiten.NewImage(screenW, screenH)
	s.Text(a, sc)
}

// The first talk with an empire greets (RESPONSE_Discover, %y us, %m
// them), the next says NoDipResponse; another empire is greeted anew.
// The leave line uncovers the species screen.
func TestNegotiateGreeting(t *testing.T) {
	a, s := negApp(t)
	if s.j != 1 || s.theirs != "RESPONSE_Discover Snovemdomas Orfa " {
		t.Fatalf("empire %d, first line %q", s.j, s.theirs)
	}
	sayKey(t, a, s, "NoDipAction")
	sp := top[*speciesScreen](t, a)
	rowClick(a, sp, 0)
	if s = top[*negotiateScreen](t, a); s.theirs != "NoDipResponse Snovemdomas Orfa " {
		t.Errorf("second talk %q", s.theirs)
	}
	sayKey(t, a, s, "NoDipAction")
	sp = top[*speciesScreen](t, a)
	rowClick(a, sp, 1)
	if s = top[*negotiateScreen](t, a); s.j != 2 || s.theirs != "RESPONSE_Discover Snovemdomas Mebes " {
		t.Errorf("the Mebes: empire %d, %q", s.j, s.theirs)
	}
}

// An exchange is answered at once by the bot's Decide: the stars (they
// learn every star we know, we nothing: fair to them) accepted, the
// research (we learn a tech, they nothing) refused; their line the
// ACCEPT_/REJECT_ response; the Mebes' pending offer to the Orfa never
// answered in place of ours.
func TestNegotiateTrade(t *testing.T) {
	a, s := negApp(t)
	theirs, err := a.g.OfferAlliance(2, 1)
	if err != nil || len(theirs) == 0 {
		t.Fatal(err)
	}
	mebes := a.g.Offers[0]
	for st := range a.cl.Stars {
		a.e.Visit(st)
	}
	a.g.Empires[1].Research.Grant(0)
	b := a.bots[1]
	for _, w := range []struct {
		key  string
		kind game.OfferKind
	}{{"TradeStars", game.TradeStars}, {"TradeResearch", game.TradeResearch}} {
		accept := b.Decide(game.Offer{From: 0, To: 1, Kind: w.kind, Third: -1})
		trades, refused := a.e.Rel(1).Trades, a.e.Rel(1).Refused
		sayKey(t, a, s, w.key)
		if !slices.Equal(a.g.Offers, []game.Offer{mebes}) {
			t.Fatalf("%s: offers %v, want the Mebes' alone", w.key, a.g.Offers)
		}
		verdict, kind := "REJECT_", game.OfferRefused
		if accept {
			verdict, kind = "ACCEPT_", game.TradeMade
			trades++
		} else {
			refused++
		}
		if r := a.e.Rel(1); r.Trades != trades || r.Refused != refused {
			t.Errorf("%s: trades %d refused %d, want %d %d", w.key, r.Trades, r.Refused, trades, refused)
		}
		if want := verdict + replyKey(w.kind) + " Snovemdomas Orfa "; s.theirs != want || s.refusal {
			t.Errorf("%s: their line %q (refusal %v), want %q", w.key, s.theirs, s.refusal, want)
		}
		if want := w.key + " Orfa Snovemdomas ..."; s.ours != want {
			t.Errorf("our line %q, want %q", s.ours, want)
		}
		if last := a.log[len(a.log)-1].Ev; last.Kind != kind || last.Other != 1 {
			t.Errorf("%s: log %+v, want %v", w.key, last, kind)
		}
		a.g.Day += game.OfferCooldown
	}
	if a.e.Rel(1).Trades != 1 || a.e.Rel(1).Refused != 1 {
		t.Errorf("fixture: want one exchange accepted, one refused; trades %d refused %d", a.e.Rel(1).Trades, a.e.Rel(1).Refused)
	}
	// A second offer within the cooldown: the game's refusal is their line.
	a.g.Day -= game.OfferCooldown
	sayKey(t, a, s, "TradeResearch")
	if !s.refusal || s.theirs != "offered 0 days ago" {
		t.Errorf("refusal %v %q", s.refusal, s.theirs)
	}
}

// A request opens the third species' sub-menu: every species the player
// met, the Govorom too though the Orfa never met them (whom they met
// stays unknown: asked about the Govorom, they refuse as on a wrong
// stance, never "never met"); back closes it; the Mebes picked makes the
// request with Third 2. With no bot it stays pending ("They will answer
// later."); with the bot its Decide answers, the line ACCEPT_/REJECT_
// with %t the Mebes.
func TestNegotiateRequest(t *testing.T) {
	a, s := negApp(t)
	ally(t, a.g)
	bt := a.bots[1]
	a.bots[1] = nil
	sayKey(t, a, s, "DeclareWarOnOtherRace")
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"third:2", "third:3", "back"}) {
		t.Fatalf("sub-menu %v", got)
	}
	if l := s.lines(a)[0]; l.text != "the Mebes" {
		t.Errorf("third line %q", l.text)
	}
	if a.g.Empires[1].StanceWith(3) != game.Unknown {
		t.Fatal("fixture: the Orfa know the third empire")
	}
	sayKey(t, a, s, "third:3")
	if !s.refusal || strings.Contains(s.theirs, "met") || len(a.g.Offers) != 0 {
		t.Errorf("asked about an empire they never met: refusal %v %q, offers %v", s.refusal, s.theirs, a.g.Offers)
	}
	s.picking = false
	sayKey(t, a, s, "DeclareWarOnOtherRace")
	sayKey(t, a, s, "back")
	sayKey(t, a, s, "DeclareWarOnOtherRace")
	sayKey(t, a, s, "third:2")
	if want := []game.Offer{{From: 0, To: 1, Kind: game.AskWarOn, Day: a.g.Day, Third: 2}}; !slices.Equal(a.g.Offers, want) {
		t.Fatalf("offers %v, want %v", a.g.Offers, want)
	}
	if s.theirs != "They will answer later." || s.ours != "DeclareWarOnOtherRace Orfa Snovemdomas Mebes" || s.picking {
		t.Errorf("ours %q, theirs %q, picking %v", s.ours, s.theirs, s.picking)
	}
	if last := a.log[len(a.log)-1].Ev; last.Kind != game.HelpAsked || last.Other != 1 {
		t.Errorf("log %+v", last)
	}
	// The bot answers at once.
	a.g.Offers = nil
	a.g.Day += game.OfferCooldown
	a.bots[1] = bt
	accept := bt.Decide(game.Offer{From: 0, To: 1, Kind: game.AskWarOn, Third: 2})
	sayKey(t, a, s, "DeclareWarOnOtherRace")
	sayKey(t, a, s, "third:2")
	verdict, stance := "REJECT_", game.Peace
	if accept {
		verdict, stance = "ACCEPT_", game.War
	}
	if want := verdict + "DeclareWarOnOtherRace Snovemdomas Orfa Mebes"; s.theirs != want || len(a.g.Offers) != 0 {
		t.Errorf("their line %q, want %q; offers %v", s.theirs, want, a.g.Offers)
	}
	if st := a.g.Empires[1].StanceWith(2); st != stance {
		t.Errorf("Orfa-Mebes %v, want %v", st, stance)
	}
}

// Declare war: RESPONSE_DeclareWar, the menu rebuilt for War; from an
// alliance, Break: RESPONSE_BreakAlliance and the Peace menu. A sub-menu
// left open when the alliance ends shows the main menu.
func TestNegotiateUnilateral(t *testing.T) {
	a, s := negApp(t)
	ally(t, a.g)
	sayKey(t, a, s, "OfferPeaceToOtherRace")
	if !s.picking {
		t.Fatal("no sub-menu")
	}
	if _, err := a.g.BreakAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if got := negKeysOf(a, s); got[0] != "DeclareWar" {
		t.Errorf("after the break under the sub-menu: %v", got)
	}
	a.g.Day += game.OfferCooldown
	ally(t, a.g)
	s.picking = false
	sayKey(t, a, s, "BreakAlliance")
	if a.e.StanceWith(1) != game.Peace || s.theirs != "RESPONSE_BreakAlliance Snovemdomas Orfa " {
		t.Errorf("break: %v, %q", a.e.StanceWith(1), s.theirs)
	}
	sayKey(t, a, s, "DeclareWar")
	if a.e.StanceWith(1) != game.War || s.theirs != "RESPONSE_DeclareWar Snovemdomas Orfa " {
		t.Errorf("war: %v, %q", a.e.StanceWith(1), s.theirs)
	}
	if last := a.log[len(a.log)-1].Ev; last.Kind != game.WarDeclared || last.Other != 1 {
		t.Errorf("log %+v", last)
	}
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"OfferPeace", "NoDipAction"}) {
		t.Errorf("war menu %v", got)
	}
}

// Without scripts: our plain lines, their line the game's Report texts.
func TestNegotiateFallback(t *testing.T) {
	a, s := negApp(t)
	a.dips = &dipScripts{}
	if l := s.lines(a)[0]; l.text != "We declare war on you." {
		t.Errorf("plain line %q", l.text)
	}
	sayKey(t, a, s, "DeclareWar")
	if want := a.log[len(a.log)-1].Ev.Text; s.theirs != want {
		t.Errorf("their line %q, want the report %q", s.theirs, want)
	}
	n := len(a.log)
	sayKey(t, a, s, "OfferPeace")
	// The answer's reports, after our offer's.
	var ts []string
	for _, l := range a.log[n:] {
		if l.Ev.Kind != game.PeaceOffered {
			ts = append(ts, l.Ev.Text)
		}
	}
	if want := strings.Join(ts, ". "); s.theirs != want || len(ts) == 0 {
		t.Errorf("their line %q, want %q", s.theirs, want)
	}
	// No archive at all: the scripts are read as none.
	b := &app{}
	if d := b.scripts(); d.player != nil || d.races[0] != nil {
		t.Error("scripts without an archive")
	}
}

// The speech box wraps a long line to its width, our line at most
// negOursMax lines, negBoxLines in all.
func TestNegotiateBox(t *testing.T) {
	_, s := negApp(t)
	s.ours = strings.Repeat("ours words ", 40)
	s.theirs = strings.Repeat("their words ", 60)
	ls, ours := s.boxLines()
	if ours != negOursMax || len(ls) != negBoxLines {
		t.Fatalf("%d lines, %d ours", len(ls), ours)
	}
	for _, l := range ls {
		if w := ui.Width(l, bigFace); w > float64(negBox.X1-negBox.X0-2*padX) {
			t.Errorf("%q: %.0f px past the box", l, w)
		}
	}
}

// The game's own scripts: read once from the archives, the greeting
// the race file's RESPONSE_Discover; Render and Text draw.
func TestNegotiateGameScripts(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer as.Close()
	a, _ := negApp(t)
	a.a, a.dips, a.greeted = as, nil, nil
	d := a.scripts()
	if d.player == nil || slices.Contains(d.races[:], nil) {
		t.Fatalf("scripts %+v", d)
	}
	if a.scripts() != d {
		t.Error("read twice")
	}
	raw, err := as.Game.Read(fmt.Sprintf("race%02d.dip", int(game.Orfa)))
	if err != nil {
		t.Fatal(err)
	}
	orfa, err := dip.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := newNegotiateScreen(1)
	a.push(s)
	if want, _ := orfa.Reply("RESPONSE_Discover", "Snovemdomas", "Orfa", "", ""); s.theirs != want || want == "" {
		t.Errorf("greeting %q, want %q", s.theirs, want)
	}
	sc := ebiten.NewImage(screenW, screenH)
	s.Render(a, sc)
	s.Text(a, sc)
}

// Every offer kind has its script key and our plain line; the
// alliance's reply key is "Alliance", every other kind's its DA_ key.
func TestNegotiateKeys(t *testing.T) {
	for k := game.PeaceOffer; k <= game.AskAllianceWith; k++ {
		if offerKeys[k] == "" || offerPlain[k] == "" {
			t.Errorf("%s: key %q, plain %q", k, offerKeys[k], offerPlain[k])
		}
		if want := offerKeys[k]; k != game.AllianceOffer && replyKey(k) != want || !slices.Contains(negKeys, replyKey(k)) {
			t.Errorf("%s: reply key %q", k, replyKey(k))
		}
		if k.Request() != strings.Contains(offerPlain[k], "%t") {
			t.Errorf("%s: plain %q and %%t", k, offerPlain[k])
		}
	}
}

// talkAgain leaves Talk and clicks the Orfa's row anew: Enter again.
func talkAgain(t *testing.T, a *app) *negotiateScreen {
	t.Helper()
	if err := a.pop(); err != nil {
		t.Fatal(err)
	}
	rowClick(a, top[*speciesScreen](t, a), 0)
	return top[*negotiateScreen](t, a)
}

// warWithPeaceOffer is negApp at war with the Orfa, their peace offer
// pending, Talk entered on it.
func warWithPeaceOffer(t *testing.T) (*app, *negotiateScreen) {
	t.Helper()
	a, _ := negApp(t)
	if _, err := a.g.DeclareWar(0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.g.OfferPeace(1, 0); err != nil {
		t.Fatal(err)
	}
	return a, talkAgain(t, a)
}

// Their pending peace offer is their opening line (their DA_ text, %y us,
// %m them), the menu our DR_ACCEPT_/DR_REJECT_ answers and Goodbye; Accept
// makes peace both ways, the offer gone, PeaceMade logged, the menu the
// Peace one.
func TestNegotiateAcceptPeace(t *testing.T) {
	a, s := warWithPeaceOffer(t)
	if s.theirs != "OfferPeace Snovemdomas Orfa " || s.refusal {
		t.Errorf("opening %q (refusal %v)", s.theirs, s.refusal)
	}
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"ACCEPT_OfferPeace", "REJECT_OfferPeace", "NoDipAction"}) {
		t.Fatalf("menu %v", got)
	}
	if l := s.lines(a)[0]; l.text != "ACCEPT_OfferPeace Orfa Snovemdomas" { // wrap trims
		t.Errorf("accept line %q", l.text)
	}
	sayKey(t, a, s, "ACCEPT_OfferPeace")
	if a.e.StanceWith(1) != game.Peace || a.g.Empires[1].StanceWith(0) != game.Peace || len(a.g.Offers) != 0 {
		t.Errorf("after Accept: %v / %v, offers %v", a.e.StanceWith(1), a.g.Empires[1].StanceWith(0), a.g.Offers)
	}
	if last := a.log[len(a.log)-1].Ev; last.Kind != game.PeaceMade || last.Other != 1 {
		t.Errorf("log %+v, want PeaceMade with 1", last)
	}
	if s.ours != "ACCEPT_OfferPeace Orfa Snovemdomas" || s.theirs != a.log[len(a.log)-1].Ev.Text {
		t.Errorf("ours %q, theirs %q", s.ours, s.theirs)
	}
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"DeclareWar", "ProposeAlliance", "TradeStarlanes", "TradeStars", "TradeResearch", "NoDipAction"}) {
		t.Errorf("menu after %v", got)
	}
}

// Refuse: still at war, the offer gone, their Refused counted,
// OfferRefused logged, the War menu.
func TestNegotiateRefusePeace(t *testing.T) {
	a, s := warWithPeaceOffer(t)
	sayKey(t, a, s, "REJECT_OfferPeace")
	if a.e.StanceWith(1) != game.War || len(a.g.Offers) != 0 || a.g.Empires[1].Rel(0).Refused != 1 {
		t.Errorf("after Refuse: %v, offers %v, their refused %d", a.e.StanceWith(1), a.g.Offers, a.g.Empires[1].Rel(0).Refused)
	}
	if last := a.log[len(a.log)-1].Ev; last.Kind != game.OfferRefused || last.Other != 1 {
		t.Errorf("log %+v", last)
	}
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"OfferPeace", "NoDipAction"}) {
		t.Errorf("menu after %v", got)
	}
}

// Two offers of theirs pending: the oldest first, the next announced as
// soon as it is answered; one made under the screen is announced by the
// next Update.
func TestNegotiateOffersInTurn(t *testing.T) {
	a, s := negApp(t)
	a.g.Offers = append(a.g.Offers, game.Offer{From: 1, To: 0, Kind: game.TradeStars, Day: a.g.Day, Third: -1},
		game.Offer{From: 1, To: 0, Kind: game.TradeLanes, Day: a.g.Day, Third: -1})
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
	if s.theirs != "TradeStars Snovemdomas Orfa " || negKeysOf(a, s)[0] != "ACCEPT_TradeStars" {
		t.Fatalf("first %q, menu %v", s.theirs, negKeysOf(a, s))
	}
	sayKey(t, a, s, "REJECT_TradeStars")
	if s.theirs != "TradeStarlanes Snovemdomas Orfa " || negKeysOf(a, s)[0] != "ACCEPT_TradeStarlanes" || len(a.g.Offers) != 1 {
		t.Errorf("next %q, menu %v, offers %v", s.theirs, negKeysOf(a, s), a.g.Offers)
	}
}

// Without scripts their offer is plain words, our answers Accept and
// Refuse; a request names its third.
func TestNegotiateOfferPlain(t *testing.T) {
	a, _ := negApp(t)
	a.dips = &dipScripts{}
	ally(t, a.g)
	a.g.Day += game.OfferCooldown
	if _, err := a.g.Request(1, 0, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	s := talkAgain(t, a)
	if s.theirs != "The Orfa ask for war on the Mebes." {
		t.Errorf("opening %q", s.theirs)
	}
	if ls := s.lines(a); ls[0].text != "Accept" || ls[1].text != "Refuse" {
		t.Errorf("answers %q %q", ls[0].text, ls[1].text)
	}
	a.g.Offers = []game.Offer{{From: 1, To: 0, Kind: game.PeaceOffer, Day: a.g.Day, Third: -1}}
	s.announce(a)
	if s.theirs != "The Orfa propose peace." {
		t.Errorf("peace %q", s.theirs)
	}
}

// An ally's request accepted in Talk: their line its DA_ text with %t the
// Mebes, the player at war with them, HelpGiven then WarDeclared logged.
func TestNegotiateAcceptRequest(t *testing.T) {
	a, _ := negApp(t)
	ally(t, a.g)
	a.g.Day += game.OfferCooldown
	if _, err := a.g.Request(1, 0, game.AskWarOn, 2); err != nil {
		t.Fatal(err)
	}
	s := talkAgain(t, a)
	if s.theirs != "DeclareWarOnOtherRace Snovemdomas Orfa Mebes" {
		t.Errorf("opening %q", s.theirs)
	}
	sayKey(t, a, s, "ACCEPT_DeclareWarOnOtherRace")
	if a.e.StanceWith(2) != game.War || len(a.g.Offers) != 0 || a.g.Empires[1].Rel(0).Helped != 1 {
		t.Errorf("after Accept: %v with 2, offers %v, Helped %d", a.e.StanceWith(2), a.g.Offers, a.g.Empires[1].Rel(0).Helped)
	}
	n := len(a.log)
	if n < 2 || a.log[n-2].Ev.Kind != game.HelpGiven || a.log[n-1].Ev.Kind != game.WarDeclared {
		t.Errorf("log tail %+v", a.log[max(0, n-2):])
	}
}

// Their facts are species, colour, stance and our friendship: no
// attitude, a bot's mood; the barred war's days after a friendship at
// FriendBar or more.
func TestNegotiateFacts(t *testing.T) {
	a, _ := negApp(t)
	o := a.g.Empires[1]
	want := []string{"Orfa", "colour: " + game.ColourName(o.Colour), "stance: peace", "friendship: 0"}
	if got := negFacts(a, 1); !slices.Equal(got, want) {
		t.Errorf("facts %q, want %q", got, want)
	}
	a.e.Rel(1).Friendship = 55
	want[3] = "friendship: 55, no war for 10 days"
	if got := negFacts(a, 1); !slices.Equal(got, want) {
		t.Errorf("friends: facts %q, want %q", got, want)
	}
	// At war (they declared it): no war left to bar.
	a.e.Rel(1).Stance, o.Rel(0).Stance = game.War, game.War
	want[2], want[3] = "stance: war", "friendship: 55"
	if got := negFacts(a, 1); !slices.Equal(got, want) {
		t.Errorf("at war: facts %q, want %q", got, want)
	}
}

// An exchange with nothing to learn either way is not dimmed (what they
// know stays hidden): said, the game's refusal is their line.
func TestNegotiateTradeBlind(t *testing.T) {
	a, s := negApp(t)
	a.e.Research.Known = a.g.Empires[1].Research.Known
	if !a.g.Gain(0, 1, game.TradeResearch).Zero() || !a.g.Gain(1, 0, game.TradeResearch).Zero() {
		t.Fatal("fixture: research still to exchange")
	}
	for _, l := range s.lines(a) {
		if l.key == "TradeResearch" && l.dim {
			t.Fatal("TradeResearch dimmed")
		}
	}
	sayKey(t, a, s, "TradeResearch")
	if !s.refusal || !strings.HasPrefix(s.theirs, "nothing to exchange") || len(a.g.Offers) != 0 {
		t.Errorf("their line %q (refusal %v), offers %v", s.theirs, s.refusal, a.g.Offers)
	}
}

// Their announced offer gone unanswered here (lapsed, or answered in a
// popup over the screen): their line becomes "Offer withdrawn.", once,
// and the menu the stance's again.
func TestNegotiateWithdrawn(t *testing.T) {
	a, s := warWithPeaceOffer(t)
	a.g.Offers = nil
	s.announce(a)
	if s.theirs != "Offer withdrawn." || s.refusal || s.toldOK {
		t.Errorf("after the offer went: %q (refusal %v, told %v)", s.theirs, s.refusal, s.toldOK)
	}
	s.theirs = "next"
	s.announce(a)
	if s.theirs != "next" {
		t.Errorf("withdrawn said twice: %q", s.theirs)
	}
	if got := negKeysOf(a, s); !slices.Equal(got, []string{"OfferPeace", "NoDipAction"}) {
		t.Errorf("menu %v", got)
	}
}
