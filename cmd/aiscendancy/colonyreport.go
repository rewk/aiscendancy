package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/rewk/aiscendancy/game"
	"github.com/rewk/aiscendancy/ui"
)

// The colony report: the original's popup each time the player founds a
// colony (help.txt colonize / maxcolonize), as it is: the planet's name
// at the left, its globe big at the right in the box's upper half, the
// text below, OK the whole bar. Queued by the planet screen's Create
// Colony (foundHere) and endDay's Built ColonyBase; after the
// exploration queue, before the ability popup, under the same rules;
// never saved.

// colonyReport is a queued report: the planet and its text, written at
// the founding (the counts then).
type colonyReport struct {
	star, planet int
	text         string
}

// help.txt's colonize and maxcolonize, their double spaces kept.
const (
	colonizeText    = "%s Colony Report: Our empire now contains %d colon%s.  The largest known empire belongs to the %s, who have %d."
	maxColonizeText = "%s Colony Report: Our empire now contains %d colon%s.  It is the largest known empire in the galaxy."
)

// The box's parts, native px: the name at the original's (50,45) off
// the box, the globe at the scale fitting an Enormous one to
// reportGlobeW, centred in the upper half's right half, the text from
// the lower half's top.
var (
	reportNameAt = orig(50, 45, 50, 45)
	reportFace   = discTitleFace
)

const reportGlobeW = 200

// reportText is the report for an empire of n colonies: the largest
// known empire (in contact, StanceWith not Unknown; the most colonies,
// the lowest index on a tie) when it has more than n, else ours
// (ASSUMPTION: a tie is ours).
func (a *app) reportText(n int) string {
	sp := a.e.Species.String()
	best := -1
	for i, e := range a.g.Empires {
		if i != a.player && a.e.StanceWith(i) != game.Unknown && (best < 0 || len(e.Colonies) > len(a.g.Empires[best].Colonies)) {
			best = i
		}
	}
	ies := "ies"
	if n == 1 {
		ies = "y"
	}
	if best >= 0 && len(a.g.Empires[best].Colonies) > n {
		return fmt.Sprintf(colonizeText, sp, n, ies, a.g.Empires[best].Species.String(), len(a.g.Empires[best].Colonies))
	}
	return fmt.Sprintf(maxColonizeText, sp, n, ies)
}

// founded queues a report per player colony founded in evs (a Built
// ColonyBase), the k-th counting before + k + 1 colonies, before the
// count ahead of the day: the foundings run in the empire's day, the
// invasions' gains and losses after it.
func (a *app) founded(evs []game.Event, before int) {
	var at []game.Event
	for _, x := range evs {
		if x.Kind == game.Built && x.Item == game.ColonyBase {
			at = append(at, x)
		}
	}
	for k, x := range at {
		a.reported(x.Star, x.Planet, before+k+1)
	}
}

// reported queues the report of the colony on (star, planet), our
// empire then of n colonies.
func (a *app) reported(star, planet, n int) {
	a.colReports = append(a.colReports, colonyReport{star, planet, a.reportText(n)})
}

// reporting is whether the popup is up: as discovering, after the
// discovery and exploration queues.
func (a *app) reporting() bool {
	if len(a.colReports) == 0 || len(a.discoveries) > 0 || len(a.explored) > 0 || !a.inGame() || a.over() {
		return false
	}
	v, ok := a.top().(*systemScreen)
	return !ok || v.replay == nil
}

// reportLayoutT is a report laid out: the box, OK, the globe's box, the
// name's place, the text lines and their top.
type reportLayoutT struct {
	box, ok, globe rect
	nameX, nameY   int
	textY          int
	lines          []string
}

func reportLayout(r colonyReport) reportLayoutT {
	c := discContent()
	b := dlgBox
	midY := (b.Y0 + b.Y1) / 2
	gx, gy := (b.X0+b.X1)/2+(b.X1-b.X0)/4, (b.Y0+midY)/2
	l := reportLayoutT{box: b, ok: dlgBar,
		globe: rect{gx - reportGlobeW/2, gy - reportGlobeW/2, gx + reportGlobeW/2, gy + reportGlobeW/2},
		nameX: b.X0 + reportNameAt.X0, nameY: b.Y0 + reportNameAt.Y0, textY: midY + discGap}
	w := c.X1 - c.X0
	for _, s := range reflow(paragraph{false, r.text}, int(float64(w)/ui.Width("m", bigFace))) {
		l.lines = append(l.lines, fitWidth(s, float64(w), bigFace))
	}
	return l
}

func (a *app) reportUpdate() error {
	// Not Space: the day's key does nothing under a popup.
	keys := keyJustPressed(ebiten.KeyEnter) || keyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButton3)
	x, y := a.mouse()
	a.reportPress(keys, inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), x, y)
	return nil
}

// reportPress closes the head on a closing key or a click on OK or
// outside the box; nothing else.
func (a *app) reportPress(key, click bool, x, y float64) {
	if len(a.colReports) == 0 {
		return
	}
	l := reportLayout(a.colReports[0])
	if key || click && (l.ok.has(x, y) || !l.box.has(x, y)) {
		a.colReports = a.colReports[1:]
	}
}

// drawReport paints the queue's head.
func (a *app) drawReport(frame *ebiten.Image) {
	r := a.colReports[0]
	l := reportLayout(r)
	fillRectWith(frame, l.box, dlgFill)
	drawSprite(frame, a.spr(dlgBank, 0), float64(l.box.X0), float64(l.box.Y0))
	p := a.e.Planet(r.star, r.planet)
	if a.a != nil {
		fitScaled(frame, a.spr("planets", int(p.Type)*game.MaxSize+p.Size-1), l.globe, a.globeScale(l.globe))
	}
	name := fitWidth(p.Name, float64(l.globe.X0-l.nameX-discGap), reportFace)
	ui.Print(frame, name, reportFace, float64(l.nameX), float64(l.nameY), titleColour)
	for k, s := range l.lines {
		ui.Print(frame, s, bigFace, float64(l.box.X0+l.box.X1)/2-ui.Width(s, bigFace)/2, float64(l.textY+k*exploreLineH), textColour)
	}
	mx, my := a.mouse()
	col := textColour
	if l.ok.has(mx, my) {
		col = yellow
	}
	const ok = "OK (Enter)"
	ui.Print(frame, ok, bigFace, float64(l.ok.X0+l.ok.X1)/2-ui.Width(ok, bigFace)/2, float64(l.ok.Y0+l.ok.Y1)/2-bigFace.Size*0.6, col)
}
