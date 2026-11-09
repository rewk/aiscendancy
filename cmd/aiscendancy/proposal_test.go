package main

import (
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/rewk/aiscendancy/assets"
	"github.com/rewk/aiscendancy/cob"
	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// proposalApp is contactApp with empires 1 and 2 met (at peace).
func proposalApp(t *testing.T) *app {
	t.Helper()
	a := contactApp(t)
	a.record(a.g.Contact(0, 1))
	a.record(a.g.Contact(0, 2))
	return a
}

// An offer made to us during the day (between beginDay and finishDay,
// where the bots act) is queued at the close, once; ours to another is
// not, nor one already pending at the start.
func TestProposalQueue(t *testing.T) {
	a := proposalApp(t)
	if _, err := a.g.OfferAlliance(2, 0); err != nil {
		t.Fatal(err)
	}
	old := a.g.Offers[0]
	d := a.beginDay()
	if _, err := a.g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := a.g.OfferTrade(0, 2, game.TradeStars); err != nil {
		t.Fatal(err)
	}
	a.finishDay(d)
	if len(a.proposals) != 1 || a.proposals[0].From != 1 || a.proposals[0].Kind != game.AllianceOffer || !a.proposing() {
		t.Fatalf("queue %+v (up %v), old %+v", a.proposals, a.proposing(), old)
	}
	a.proposed(nil)
	if len(a.proposals) != 2 || a.proposals[1] != old {
		t.Errorf("proposed(nil): %+v, want the alliance then the old one once", a.proposals)
	}
}

// Accept answers at once (the alliance made, logged); Refuse refuses
// (their Refused counted); Postpone, Esc and a click outside keep the
// offer pending; a click inside does nothing; Talk opens the
// negotiation with its maker, its Accept on top.
func TestProposalButtons(t *testing.T) {
	a := proposalApp(t)
	offer := func(j int) {
		t.Helper()
		before := a.g.PendingOffers(0)
		a.g.Empires[j].Rel(0).LastOffer = -1
		if _, err := a.g.OfferAlliance(j, 0); err != nil {
			t.Fatal(err)
		}
		a.proposed(before)
	}
	press := func(k proposalAct) {
		t.Helper()
		l := proposalLayout(a, a.proposals[0])
		a.proposalClick(float64(l.buttons[k].X0+2), float64(l.buttons[k].Y0+2))
	}
	offer(1)
	l := proposalLayout(a, a.proposals[0])
	a.proposalClick(float64(l.ringX), float64(l.ringY))
	if len(a.proposals) != 1 {
		t.Fatal("a click inside closed it")
	}
	press(propAccept)
	if a.e.StanceWith(1) != game.Alliance || len(a.proposals) != 0 || len(a.g.Offers) != 0 {
		t.Fatalf("Accept: %v, queue %v, offers %v", a.e.StanceWith(1), a.proposals, a.g.Offers)
	}
	if last := a.log[len(a.log)-1].Ev; last.Kind != game.AllianceMade {
		t.Errorf("Accept logged %v", last.Kind)
	}
	offer(2)
	press(propRefuse)
	if a.e.StanceWith(2) != game.Peace || len(a.g.Offers) != 0 || a.g.Empires[2].Rel(0).Refused != 1 {
		t.Fatalf("Refuse: %v, offers %v, refused %d", a.e.StanceWith(2), a.g.Offers, a.g.Empires[2].Rel(0).Refused)
	}
	offer(2)
	press(propPostpone)
	a.proposed(nil)
	pressing(t, ebiten.KeyEscape)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	a.proposed(nil)
	l = proposalLayout(a, a.proposals[0])
	a.proposalClick(float64(l.box.X0-1), float64(l.box.Y0))
	if len(a.proposals) != 0 || len(a.g.Offers) != 1 {
		t.Fatalf("Postpone: queue %v, offers %v", a.proposals, a.g.Offers)
	}
	a.proposed(nil)
	press(propTalk)
	s := top[*negotiateScreen](t, a)
	if s.j != 2 || len(a.proposals) != 0 || negKeysOf(a, s)[0] != "ACCEPT_Alliance" {
		t.Errorf("Talk: empire %d, queue %v, menu %v", s.j, a.proposals, negKeysOf(a, s))
	}
}

// It waits for the contacts; the ability popup waits for it; a head no
// longer pending (answered in Talk) is dropped; F9's restart drops the
// queue; Space runs no day under it.
func TestProposalOrder(t *testing.T) {
	a := proposalApp(t)
	if _, err := a.g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	a.proposed(nil)
	a.contacts, a.charged = metAll(1), true
	if a.proposing() || !a.contacting() {
		t.Fatal("up over a contact")
	}
	a.contacts = nil
	if !a.proposing() || a.charging() || a.keysSection() != keysPopups {
		t.Fatalf("proposing %v, charging %v", a.proposing(), a.charging())
	}
	day := a.g.Day
	pressing(t, ebiten.KeySpace)
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
	if a.g.Day != day || !a.proposing() {
		t.Fatalf("Space: day %d -> %d", day, a.g.Day)
	}
	if _, err := a.g.Answer(a.proposals[0], false); err != nil {
		t.Fatal(err)
	}
	if a.proposing() || len(a.proposals) != 0 || !a.charging() {
		t.Errorf("answered elsewhere: up %v, queue %v, charging %v", a.proposing(), a.proposals, a.charging())
	}
	a.charged = false
	if _, err := a.g.OfferPeace(0, 1); err == nil {
		t.Fatal("peace offered at peace")
	}
	a.g.Empires[1].Rel(0).LastOffer = -1
	if _, err := a.g.OfferAlliance(1, 0); err != nil {
		t.Fatal(err)
	}
	a.proposed(nil)
	a.restart()
	if len(a.proposals) != 0 {
		t.Errorf("restart kept %v", a.proposals)
	}
}

// The layout: four equal buttons across the bar, each label inside; the
// text column's lines (a request's, the longest) above the bar; the
// lapse line counts the days left.
func TestProposalLayout(t *testing.T) {
	a := proposalApp(t)
	ally(t, a.g)
	a.g.Contact(1, 2)
	a.g.Empires[1].Rel(0).LastOffer = -1
	if _, err := a.g.Request(1, 0, game.AskAllianceWith, 2); err != nil {
		t.Fatal(err)
	}
	a.proposed(nil)
	o := a.proposals[0]
	l := proposalLayout(a, o)
	for k, r := range l.buttons {
		if r.Y0 != dlgBar.Y0 || k > 0 && r.X0 != l.buttons[k-1].X1 || ui.Width(proposalButtons[k], bigFace) > float64(r.X1-r.X0) {
			t.Errorf("button %d %+v", k, r)
		}
	}
	if l.buttons[0].X0 != dlgBar.X0 || l.buttons[3].X1 != dlgBar.X1 {
		t.Errorf("bar %+v, buttons %+v", dlgBar, l.buttons)
	}
	if bottom := l.textY + len(l.lines)*exploreLineH; bottom > discContent().Y1 {
		t.Errorf("text ends at %d past %d: %q", bottom, discContent().Y1, l.lines)
	}
	a.g.Day += 3
	if got := proposalText(a, o); !strings.HasSuffix(got, "lapses as a refusal in 7 days.") {
		t.Errorf("lapse: %q", got)
	}
}

// With the game: every species' DA_ line of the longest kind (a request
// naming the Snovemdomas) fits the text column above the bar; drawn.
func TestProposalDraw(t *testing.T) {
	dir := cob.DefaultDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("no game at %s", dir)
	}
	as, err := assets.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { as.Close() })
	a := testApp(game.NewGame(1, []game.Species{game.Orfa, game.Mebes, game.Snovemdomas}, game.Average))
	a.push(newStarmapScreen())
	a.a = as
	for _, p := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
		a.g.Contact(p[0], p[1])
	}
	ally(t, a.g)
	frame := ebiten.NewImage(screenW, screenH)
	o := a.g.Empires[1]
	for sp := range game.NumSpecies {
		o.Species = sp
		for k := game.PeaceOffer; k <= game.AskAllianceWith; k++ {
			of := game.Offer{From: 1, To: 0, Kind: k, Day: a.g.Day, Third: 2}
			l := proposalLayout(a, of)
			if bottom := l.textY + len(l.lines)*exploreLineH; bottom > discContent().Y1 {
				t.Errorf("%s %s: text ends at %d past %d: %q", sp, k, bottom, discContent().Y1, l.lines)
			}
			a.proposals = []game.Offer{of}
			a.drawProposal(frame)
		}
	}
}
