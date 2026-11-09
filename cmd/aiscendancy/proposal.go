package main

import (
	"fmt"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The proposal popup: each offer or request another empire makes the
// player during a day (pending to us at the close, not at beginDay's
// snapshot) brings up its maker's portrait in its colour ring, the
// stance, their race's DA_ line (offerSpeech, Talk's) and the days left
// before it lapses as a refusal. Accept and Refuse answer it at once
// (Game.Answer); Talk opens the negotiation with them; Postpone closes
// it, the offer still pending. Queued after the contacts, before the
// ability popup, under the same rules; never saved.

const proposalTitle = "Proposal"

// proposalAct is a popup button's act.
type proposalAct uint8

const (
	propAccept proposalAct = iota
	propRefuse
	propTalk
	propPostpone
)

// proposalButtons are the bar's quarters, left to right.
var proposalButtons = [...]string{"Accept (A)", "Refuse (R)", "Talk (T)", "Postpone (Esc)"}

// proposed queues, in Game.Offers' order, the offers pending to the
// player that before (beginDay's snapshot) lacks.
func (a *app) proposed(before []game.Offer) {
	for _, o := range a.g.PendingOffers(a.player) {
		if o.To == a.player && !slices.Contains(before, o) && !slices.Contains(a.proposals, o) {
			a.proposals = append(a.proposals, o)
		}
	}
}

// proposing is whether the popup is up: as contacting, after the
// contact queue; a head no longer pending is dropped first.
func (a *app) proposing() bool {
	if len(a.proposals) == 0 || len(a.contacts) > 0 || len(a.discoveries) > 0 || len(a.explored) > 0 || len(a.colReports) > 0 || !a.inGame() || a.over() {
		return false
	}
	for len(a.proposals) > 0 && !slices.Contains(a.g.PendingOffers(a.player), a.proposals[0]) {
		a.proposals = a.proposals[1:]
	}
	if len(a.proposals) == 0 {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// proposalLayoutT is a popup laid out: contact's, its bar in four.
type proposalLayoutT struct {
	contactLayoutT
	buttons [len(proposalButtons)]rect
}

// proposalLayout lays offer o's popup out as the contact popup, the
// text column holding their line then the lapse line.
func proposalLayout(a *app, o game.Offer) proposalLayoutT {
	l := proposalLayoutT{contactLayoutT: contactLayout(a, o.From)}
	c := discContent()
	w := c.X1 - l.textX
	l.lines = nil
	for _, s := range reflow(paragraph{false, proposalText(a, o)}, int(float64(w)/ui.Width("m", bigFace))) {
		l.lines = append(l.lines, fitWidth(s, float64(w), bigFace))
	}
	bw := (dlgBar.X1 - dlgBar.X0) / len(l.buttons)
	for k := range l.buttons {
		l.buttons[k] = rect{dlgBar.X0 + k*bw, dlgBar.Y0, dlgBar.X0 + (k+1)*bw, dlgBar.Y1}
	}
	l.buttons[len(l.buttons)-1].X1 = dlgBar.X1
	return l
}

// proposalText is their line, then when it lapses.
func proposalText(a *app, o game.Offer) string {
	left := o.Day + game.OfferExpiry - a.g.Day
	return fmt.Sprintf("%s Unanswered, it lapses as a refusal in %d day%s.", offerSpeech(a, o), left, plural(left))
}

func (a *app) proposalUpdate() error {
	// Not Space: the day's key does nothing under a popup.
	x, y := a.mouse()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	switch {
	case keyJustPressed(ebiten.KeyA):
		a.proposalDo(propAccept)
	case keyJustPressed(ebiten.KeyR):
		a.proposalDo(propRefuse)
	case keyJustPressed(ebiten.KeyT):
		a.proposalDo(propTalk)
	case keyJustPressed(ebiten.KeyEnter) || keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
		a.proposalDo(propPostpone)
	case click:
		a.proposalClick(x, y)
	}
	return nil
}

// proposalClick is a left press at (x,y): a button's act; outside the
// box, Postpone; inside, nothing.
func (a *app) proposalClick(x, y float64) {
	if len(a.proposals) == 0 {
		return
	}
	l := proposalLayout(a, a.proposals[0])
	for k, r := range l.buttons {
		if r.has(x, y) {
			a.proposalDo(proposalAct(k))
			return
		}
	}
	if !l.box.has(x, y) {
		a.proposalDo(propPostpone)
	}
}

// proposalDo pops the head and acts: Accept and Refuse answer it
// (Game.Answer, its reports logged, an exchange's discoveries queued; a
// refusal of the game toasted); Talk pushes the negotiation with its
// maker (Talk's own Accept and Refuse then show it).
func (a *app) proposalDo(act proposalAct) {
	if len(a.proposals) == 0 {
		return
	}
	o := a.proposals[0]
	a.proposals = a.proposals[1:]
	switch act {
	case propAccept, propRefuse:
		before := a.e.Research.Known
		reps, err := a.g.Answer(o, act == propAccept)
		a.learnt(before, a.record(reps))
		if err != nil {
			a.toastFor(err.Error(), true)
		}
	case propTalk:
		if s, ok := a.top().(*negotiateScreen); ok && s.j == o.From {
			return
		}
		a.push(newNegotiateScreen(o.From))
	}
}

// drawProposal paints the queue's head.
func (a *app) drawProposal(frame *ebiten.Image) {
	o := a.proposals[0]
	e := a.g.Empires[o.From]
	l := proposalLayout(a, o)
	fillRectWith(frame, l.box, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(l.box.X0), float64(l.box.Y0))
	ui.Print(frame, proposalTitle, discTitleFace, float64(l.box.X0+l.box.X1)/2-ui.Width(proposalTitle, discTitleFace)/2, float64(l.titleY), titleColour)
	if a.a != nil {
		drawSprite(frame, a.spr(ringBank, e.Colour), float64(l.ringX), float64(l.ringY))
		drawSprite(frame, a.spr(portraitBank, int(e.Species)), float64(l.ringX), float64(l.ringY))
		drawFlag(a, frame, e, l.flagX, l.nameY)
	}
	ui.Print(frame, e.Species.String(), discNameFace, float64(l.textX), float64(l.nameY), titleColour)
	ui.Print(frame, "Stance: "+a.e.StanceWith(o.From).String(), bigFace, float64(l.textX), float64(l.stanceY), textColour)
	for k, s := range l.lines {
		ui.Print(frame, s, bigFace, float64(l.textX), float64(l.textY+k*exploreLineH), textColour)
	}
	mx, my := a.mouse()
	for k, r := range l.buttons {
		col := textColour
		if r.has(mx, my) {
			col = yellow
		}
		txt := proposalButtons[k]
		ui.Print(frame, txt, bigFace, float64(r.X0+r.X1)/2-ui.Width(txt, bigFace)/2, float64(r.Y0+r.Y1)/2-bigFace.Size*0.6, col)
	}
}
