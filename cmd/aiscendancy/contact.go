package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The first-contact popup: each empire the player meets (our Contacted
// event in endDay's) brings up its portrait in its colour ring, its
// species, the original's description and the stance: only what contact
// reveals. The same popup tells a war declared on the player or an
// alliance broken with it, titled so, their
// race's DA_DeclareWar / DA_BreakAlliance line as the text: endDay's
// events hold only the others' acts, the player's are recorded where
// made (negotiate.go).
// Queued after the colony reports, before the ability popup, under the
// same rules; never saved.

const contactTitle = "First contact"

// contactNote is a queued popup: empire J and what it did, Contacted,
// WarDeclared or AllianceBroken.
type contactNote struct {
	J    int
	Kind game.EventKind
}

// noteTitles are the popup's titles by kind.
var noteTitles = map[game.EventKind]string{game.Contacted: contactTitle, game.WarDeclared: "War declared", game.AllianceBroken: "Alliance broken"}

// contactRingW is racering's frame at zoom, native px (109 px wide).
const contactRingW = 109 * zoom

// contacted queues, in event order, the empire each Contacted,
// WarDeclared or AllianceBroken of the player's evs names; not a war or
// a break by an empire defeated at the same close.
func (a *app) contacted(evs []game.Event) {
	for _, x := range evs {
		if _, ok := noteTitles[x.Kind]; ok && x.Other >= 0 && x.Other < len(a.g.Empires) &&
			(x.Kind == game.Contacted || a.g.Empires[x.Other].Alive()) {
			a.contacts = append(a.contacts, contactNote{x.Other, x.Kind})
		}
	}
}

// contacting is whether the popup is up: as discovering, after the
// discovery, exploration and colony report queues.
func (a *app) contacting() bool {
	if len(a.contacts) == 0 || len(a.discoveries) > 0 || len(a.explored) > 0 || len(a.colReports) > 0 || !a.inGame() || a.over() {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// contactText is j's description as the original phrases it: "The
// <species> <description>".
func (a *app) contactText(j int) string {
	in := a.g.Empires[j].Species.Info()
	return "The " + in.Name + " " + in.Description
}

// noteText is n's text: the description at contact, else their race's
// DA_ line for the act, a plain sentence without the archive's.
func (a *app) noteText(n contactNote) string {
	them, us := a.g.Empires[n.J].Species.String(), a.e.Species.String()
	key, plain := "DeclareWar", "The "+them+" declare war on us."
	switch n.Kind {
	case game.Contacted:
		return a.contactText(n.J)
	case game.AllianceBroken:
		key, plain = "BreakAlliance", "The "+them+" break our alliance."
	}
	if t, ok := a.scripts().races[a.g.Empires[n.J].Species].Say(key, us, them, "", ""); ok {
		return t
	}
	return plain
}

// contactLayoutT is a popup laid out: the box, its two buttons, the
// title's top, the ring's centre, the text column's left, the name's,
// the stance's and the description's tops, the description's lines.
type contactLayoutT struct {
	box, talk, ok         rect
	titleY                int
	ringX, ringY          int
	flagX, textX          int // the flag's left at nameY, then the text column's
	nameY, stanceY, textY int
	lines                 []string
}

// contactLayout is j's first-contact popup laid out (noteLayout).
func contactLayout(a *app, j int) contactLayoutT {
	return noteLayout(a, contactNote{j, game.Contacted})
}

// noteLayout lays n's popup out in the fixed box: the title centred
// over a row, the row (the ring at the left, the empire's flag, the
// name, the stance and the text in the column right of it) centred
// down; Talk and OK the bar's halves.
func noteLayout(a *app, n contactNote) contactLayoutT {
	c := discContent()
	mid := (dlgBar.X0 + dlgBar.X1) / 2
	l := contactLayoutT{box: dlgBox, talk: rect{dlgBar.X0, dlgBar.Y0, mid, dlgBar.Y1}, ok: rect{mid, dlgBar.Y0, dlgBar.X1, dlgBar.Y1}}
	titleH := int(discTitleFace.Size * 1.25)
	l.titleY = c.Y0 + (c.Y1-c.Y0-(titleH+discGap+contactRingW))/2
	top := l.titleY + titleH + discGap
	l.ringX, l.ringY = c.X0+contactRingW/2, top+contactRingW/2
	l.flagX = c.X0 + contactRingW + 2*discGap
	l.textX = l.flagX + flagW*zoom + discGap
	w := c.X1 - l.textX
	l.nameY = top
	l.stanceY = l.nameY + int(discNameH)
	l.textY = l.stanceY + exploreLineH + discGap
	for _, s := range reflow(paragraph{false, a.noteText(n)}, int(float64(w)/ui.Width("m", bigFace))) {
		l.lines = append(l.lines, fitWidth(s, float64(w), bigFace))
	}
	return l
}

func (a *app) contactUpdate() error {
	// Not Space: the day's key does nothing under a popup.
	keys := keyJustPressed(ebiten.KeyEnter) || keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	x, y := a.mouse()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if keyJustPressed(ebiten.KeyT) || click && len(a.contacts) > 0 && noteLayout(a, a.contacts[0]).talk.has(x, y) {
		a.contactTalk()
		return nil
	}
	a.contactPress(keys, click, x, y)
	return nil
}

// contactTalk is Talk or T: the head closes, the negotiation with its
// empire opens (unless already on top); the queue goes on over it.
func (a *app) contactTalk() {
	if len(a.contacts) == 0 {
		return
	}
	j := a.contacts[0].J
	a.contacts = a.contacts[1:]
	if s, ok := a.top().(*negotiateScreen); ok && s.j == j {
		return
	}
	a.push(newNegotiateScreen(j))
}

// contactPress closes the head on a closing key or a click on OK or
// outside the box; nothing else.
func (a *app) contactPress(key, click bool, x, y float64) {
	if len(a.contacts) == 0 {
		return
	}
	l := noteLayout(a, a.contacts[0])
	if key || click && (l.ok.has(x, y) || !l.box.has(x, y)) {
		a.contacts = a.contacts[1:]
	}
}

// drawContact paints the queue's head.
func (a *app) drawContact(frame *ebiten.Image) {
	n := a.contacts[0]
	j, title := n.J, noteTitles[n.Kind]
	o := a.g.Empires[j]
	l := noteLayout(a, n)
	fillRectWith(frame, l.box, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(l.box.X0), float64(l.box.Y0))
	ui.Print(frame, title, discTitleFace, float64(l.box.X0+l.box.X1)/2-ui.Width(title, discTitleFace)/2, float64(l.titleY), titleColour)
	if a.a != nil {
		drawSprite(frame, a.spr(ringBank, o.Colour), float64(l.ringX), float64(l.ringY))
		drawSprite(frame, a.spr(portraitBank, int(o.Species)), float64(l.ringX), float64(l.ringY))
		drawFlag(a, frame, o, l.flagX, l.nameY)
	}
	ui.Print(frame, o.Species.String(), discNameFace, float64(l.textX), float64(l.nameY), titleColour)
	ui.Print(frame, "Stance: "+a.e.StanceWith(j).String(), bigFace, float64(l.textX), float64(l.stanceY), textColour)
	for k, s := range l.lines {
		ui.Print(frame, s, bigFace, float64(l.textX), float64(l.textY+k*exploreLineH), textColour)
	}
	mx, my := a.mouse()
	for _, b := range []struct {
		r   rect
		txt string
	}{{l.talk, "Talk (T)"}, {l.ok, "OK (Enter)"}} {
		col := textColour
		if b.r.has(mx, my) {
			col = yellow
		}
		ui.Print(frame, b.txt, bigFace, float64(b.r.X0+b.r.X1)/2-ui.Width(b.txt, bigFace)/2, float64(b.r.Y0+b.r.Y1)/2-bigFace.Size*0.6, col)
	}
}

// drawFlag paints e's flag in its colour at zoom, top-left (x,y).
func drawFlag(a *app, dst *ebiten.Image, e *game.Empire, x, y int) {
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x), float64(y))
	a.flag(e.Species, e.Colour).Draw(dst, &op)
}
