package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The discovery popup: a box per tech the player came to know, a row
// per unlock with its help. Fed by diffing Research.Known around every
// grant site (learnt); never saved. While up it takes every input but
// F3, F5, F9 and Tab.

// discovery is a queued tech and its source line.
type discovery struct {
	tech game.Tech
	from string
}

// The box is the prompt's, help.shp frame 0 at dlgBox, Research and OK
// the button bar's halves. The content sits discInset inside the frame,
// above the bar: the head (the restree picture at discPicK, 50x50 origin
// the centre, beside its title and source, the pair centred), then the
// rows sharing what is left.
const (
	discInset = 10 * zoom
	discPicK  = 1.5 // the restree picture's scale, under zoom
	discPicW  = 75  // its side, native px
	discGap   = 6 * zoom
	discOKW   = 120 * zoom // the quit prompt's buttons
	discOKH   = 22 * zoom
	discIconW = 40 * zoom // a row's icon box, the icon scaled down to it
)

// The popup's faces: the title and names bigger than bigFace, the
// help at 14, the largest at which the longest help (Energy
// Redirection's three) fits uncut (discovery_test; 15 cuts it).
var (
	discTitleFace = ui.Face(22)
	discNameFace  = ui.Face(18)
	discTextFace  = ui.Face(14)
	discNameH     = discNameFace.Size * 1.25
	discTextH     = discTextFace.Size * 1.25
)

// learnt queues, in tech order, every tech known now and not in before,
// its source read in evs (the granting batch). The batch count and the
// story restart on an empty queue; a batch taking the tree from
// incomplete (before) to Complete sets discStory.
func (a *app) learnt(before [game.NumTechs]bool, evs []game.Event) {
	if len(a.discoveries) == 0 {
		a.discTotal, a.discStory = 0, false
	}
	n := len(a.discoveries)
	for t := range game.NumTechs {
		if !before[t] && a.e.Research.Known[t] {
			a.discoveries = append(a.discoveries, discovery{t, a.discoverySource(t, evs)})
			a.discTotal++
		}
	}
	if len(a.discoveries) > n && slices.Contains(before[:], false) && a.e.Research.Complete() {
		a.discStory = true
	}
}

// storyShown reports the head's popup tells the singularity
// (game.SingularityStory): the last queued of the batch that completed
// the tree (nothing is learnt after it, so the queue's last).
func (a *app) storyShown() bool {
	return a.discStory && len(a.discoveries) == 1 && a.e.Research.Complete()
}

// discoverySource is t's source line from evs: a Discovered at a planet
// is the Xeno dig there; without a planet, the AbilityUsed opening its
// run of Discovered (the Dubtaks' theft) or "Our scientists"; no
// Discovered is an exchange, "Learnt from the <species>" of the
// TradeMade naming t (else the batch's first), or "Learnt".
func (a *app) discoverySource(t game.Tech, evs []game.Event) string {
	ability := ""
	for _, x := range evs {
		switch x.Kind {
		case game.AbilityUsed:
			ability = x.Text
			continue
		case game.Discovered:
			if x.Tech != t {
				continue
			}
			switch {
			case x.Star >= 0:
				return fmt.Sprintf("%s on %s", game.XenoArcheologicalDig, a.e.Planet(x.Star, x.Planet).Name)
			case ability != "":
				return ability
			}
			return "Our scientists"
		}
		ability = "" // the run is broken
	}
	from := -1 // the TradeMade naming t, else the batch's first
	for _, x := range evs {
		if x.Kind == game.TradeMade && x.Other >= 0 && x.Other < len(a.g.Empires) && (from < 0 || x.Tech == t) {
			if from = x.Other; x.Tech == t {
				break
			}
		}
	}
	if from >= 0 {
		return "Learnt from the " + a.g.Empires[from].Species.String()
	}
	return "Learnt"
}

// discovering is whether the popup is up: a queue, the game not over,
// no combat replay on top (it waits, as the end prompt).
func (a *app) discovering() bool {
	if len(a.discoveries) == 0 || !a.inGame() || a.over() {
		return false
	}
	if v, ok := a.top().(*systemScreen); ok && v.replay != nil {
		return false
	}
	return true
}

func (a *app) discoveryUpdate() error {
	// Not Space: the day's key does nothing under a popup.
	keys := keyJustPressed(ebiten.KeyEnter) || keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3)
	x, y := a.mouse()
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if keyJustPressed(ebiten.KeyR) || click && a.onResearchButton(x, y) {
		a.discoveryResearch()
		return nil
	}
	a.discoveryPress(keys, click, x, y)
	return nil
}

// onResearchButton reports native (x,y) on the head's Research button.
func (a *app) onResearchButton(x, y float64) bool {
	return len(a.discoveries) > 0 && discLayout(a, a.discoveries[0].tech).research.has(x, y)
}

// discoveryResearch is the Research button or R: the head closes, the
// research screen opens unless on top; the queue goes on over it.
func (a *app) discoveryResearch() {
	if len(a.discoveries) == 0 {
		return
	}
	a.discoveries = a.discoveries[1:]
	if _, ok := a.top().(*researchScreen); !ok {
		a.push(newResearchScreen())
	}
}

// discoveryPress closes the head on a closing key or a click on OK or
// outside the box; nothing else.
func (a *app) discoveryPress(key, click bool, x, y float64) {
	if len(a.discoveries) == 0 {
		return
	}
	l := discLayout(a, a.discoveries[0].tech)
	if key || click && (l.ok.has(x, y) || !l.box.has(x, y)) {
		a.discoveries = a.discoveries[1:]
	}
}

// discRow is one unlock: its icon scaled by k to w x iconH native px,
// the row's top y and height, its lines.
type discRow struct {
	unlock
	y, h, w, iconH int
	k              float64
	lines          []string
}

// unlockText is u's help paragraph ("" without assets), a hull's slot
// count.
func unlockText(a *app, u unlock) string {
	switch u.bank {
	case "planitem":
		return a.itemHelp(game.Item(u.frame))
	case gizmoBank:
		return a.gizmoHelp(game.Gizmo(u.frame))
	}
	return fmt.Sprintf("%d slots", game.Hull(u.frame).Slots())
}

// discContent is the room inside the frame, above the button bar.
func discContent() rect {
	return rect{dlgBox.X0 + discInset, dlgBox.Y0 + discInset, dlgBox.X1 - discInset, dlgBar.Y0 - discInset}
}

// discTextX/discTextW are the rows' text column.
func discTextX() int { return discContent().X0 + discIconW + discGap }

func discTextW() float64 { return float64(discContent().X1 - discTextX()) }

// discLines is text reflowed to w native px, each line cut to w.
func discLines(text string, w float64) []string {
	ls := reflow(paragraph{false, text}, int(w/ui.Width("m", discTextFace)))
	for i := range ls {
		ls[i] = fitWidth(ls[i], w, discTextFace)
	}
	return ls
}

// discLayoutT is a popup laid out: foot is the y of the "unlocks
// nothing new" line, story the singularity's lines at storyY (none when
// not storyShown).
type discLayoutT struct {
	box, ok  rect
	research rect
	rows     []discRow
	foot     int
	story    []string
	storyY   int
}

// discLayout lays t's popup out in the fixed box: the rows under the
// head share the room left evenly, each as tall as its icon or its name
// over as many lines as its share holds ("..." when cut); a row using
// less than its share leaves the rest to the next ones. The story, when
// shown, takes the content's foot over the full width first.
func discLayout(a *app, t game.Tech) discLayoutT {
	us := unlocks(a.e.Research, a.e.Species, t)
	c := discContent()
	l := discLayoutT{box: dlgBox}
	if a.storyShown() {
		l.story = discLines(game.SingularityStory, float64(c.X1-c.X0))
		l.storyY = c.Y1 - int(float64(len(l.story))*discTextH)
		c.Y1 = l.storyY - discGap
	}
	mid := (dlgBar.X0 + dlgBar.X1) / 2
	l.research = rect{dlgBar.X0, dlgBar.Y0, mid, dlgBar.Y1}
	l.ok = rect{mid, dlgBar.Y0, dlgBar.X1, dlgBar.Y1}
	y := c.Y0 + discPicW + discGap
	l.foot = y
	w := discTextW()
	for i, u := range us {
		share := (c.Y1 - y - discGap*(len(us)-1-i)) / (len(us) - i)
		r := discRow{unlock: u, y: y, k: zoom}
		n := max(0, int((float64(share)-discNameH)/discTextH))
		if txt := unlockText(a, u); txt != "" && n > 0 {
			all := discLines(txt, w)
			r.lines = all[:min(len(all), n)]
			if len(all) > n {
				ln := &r.lines[n-1]
				*ln = fitWidth(*ln, w-ui.Width("...", discTextFace), discTextFace) + "..."
			}
		}
		r.h = int(discNameH + float64(len(r.lines))*discTextH)
		if a.a != nil {
			if sp := a.spr(u.bank, u.frame); sp.Image != nil {
				b := sp.Image.Bounds()
				r.k = min(r.k, float64(discIconW)/float64(b.Dx()), float64(min(share, discIconW))/float64(b.Dy()))
				r.w, r.iconH = int(r.k*float64(b.Dx())), int(r.k*float64(b.Dy()))
				r.h = max(r.h, r.iconH)
			}
		}
		l.rows = append(l.rows, r)
		y += r.h + discGap
	}
	return l
}

// drawDiscovery paints the queue's head, "k of n" for a bigger batch.
func (a *app) drawDiscovery(frame *ebiten.Image) {
	d := a.discoveries[0]
	l := discLayout(a, d.tech)
	c := discContent()
	fillRectWith(frame, l.box, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(l.box.X0), float64(l.box.Y0))

	// The head: "k of n" at the top right, the picture and its two lines
	// centred as one block in what is left between two such margins.
	free := float64(c.X1 - c.X0)
	if a.discTotal > 1 {
		n := fmt.Sprintf("%d of %d", a.discTotal-len(a.discoveries)+1, a.discTotal)
		nw := ui.Width(n, discTextFace)
		ui.Print(frame, n, discTextFace, float64(c.X1)-nw, float64(c.Y0), textColour)
		free -= 2 * (nw + float64(discGap))
	}
	tw := free - discPicW - discGap
	title := fitWidth(d.tech.String(), tw, discTitleFace)
	from := fitWidth(d.from, tw, discTextFace)
	bw := discPicW + discGap + max(ui.Width(title, discTitleFace), ui.Width(from, discTextFace))
	x0 := float64(c.X0+c.X1)/2 - bw/2
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(discPicK, discPicK)
	op.GeoM.Translate(x0+discPicW/2, float64(c.Y0)+discPicW/2)
	op.Filter = ebiten.FilterLinear
	a.spr(resBank, int(d.tech)).Draw(frame, &op)
	tx := x0 + discPicW + discGap
	titleH := discTitleFace.Size * 1.25
	ty := float64(c.Y0) + discPicW/2 - (titleH+discTextH)/2
	ui.Print(frame, title, discTitleFace, tx, ty, titleColour)
	ui.Print(frame, from, discTextFace, tx, ty+titleH, textColour)

	nx, nw := float64(discTextX()), discTextW()
	for _, r := range l.rows {
		if a.a != nil {
			sp := a.spr(r.bank, r.frame)
			var op ebiten.DrawImageOptions
			op.GeoM.Scale(r.k, r.k)
			op.GeoM.Translate(float64(c.X0+(discIconW-r.w)/2)-r.k*float64(sp.Offset.X), float64(r.y+(r.h-r.iconH)/2)-r.k*float64(sp.Offset.Y))
			if r.k != zoom {
				op.Filter = ebiten.FilterLinear
			}
			sp.Draw(frame, &op)
		}
		ui.Print(frame, fitWidth(r.name, nw, discNameFace), discNameFace, nx, float64(r.y), textColour)
		ui.Print(frame, strings.Join(r.lines, "\n"), discTextFace, nx, float64(r.y)+discNameH, textColour)
	}
	if len(l.rows) == 0 {
		ui.Print(frame, "unlocks nothing new", discTextFace, float64(c.X0), float64(l.foot), textColour)
	}
	if l.story != nil {
		ui.Print(frame, strings.Join(l.story, "\n"), discTextFace, float64(c.X0), float64(l.storyY), titleColour)
	}
	mx, my := a.mouse()
	for _, b := range []struct {
		r   rect
		txt string
	}{{l.research, "Research (R)"}, {l.ok, "OK (Enter)"}} {
		col := textColour
		if b.r.has(mx, my) {
			col = yellow
		}
		ui.Print(frame, b.txt, bigFace, float64(b.r.X0+b.r.X1)/2-ui.Width(b.txt, bigFace)/2, float64(b.r.Y0+b.r.Y1)/2-bigFace.Size*0.6, col)
	}
}

// The ability popup: the original's notice when the player's timed
// ability recharges (an AbilityReady in endDay's events sets a.charged),
// a prompt with OK alone; after the discoveries, never saved.

// chargedText is the original's notice.
const chargedText = "Your species has amassed enough energy to use its special ability."

// charging is whether the ability popup is up: as discovering, after
// the discovery, exploration, colony report, contact and proposal queues.
func (a *app) charging() bool {
	if !a.charged || len(a.discoveries) > 0 || len(a.explored) > 0 || len(a.colReports) > 0 || len(a.contacts) > 0 || len(a.proposals) > 0 || !a.inGame() || a.over() {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// chargedPrompt is the popup: chargedText reflowed to the box.
func (a *app) chargedPrompt() prompt {
	box, _, _ := quitPrompt.rects()
	cols := int(float64(box.X1-box.X0-4*padX) / ui.Width("m", bigFace))
	return prompt{reflow(paragraph{false, chargedText}, cols), "OK (Enter)", ""}
}

// chargedUpdate closes the popup on any answer (answerCharged).
func (a *app) chargedUpdate() error {
	if ok, _ := a.chargedPrompt().update(a); ok {
		a.answerCharged()
	}
	return nil
}

// answerCharged closes the popup, nothing else.
func (a *app) answerCharged() { a.charged = false }
